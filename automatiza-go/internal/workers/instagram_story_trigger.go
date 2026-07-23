package workers

import (
	"context"
	"strings"
	"time"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/models"
	"automatiza-go/internal/observability"
	"automatiza-go/internal/repository"
	"automatiza-go/internal/services/instagram"

	"github.com/getsentry/sentry-go"
	"go.uber.org/zap"
)

// storyMediaTTL is how long the mirrored/rendered image lives in the ephemeral
// store waiting for the worker to publish it. Generous vs. the 90s task budget
// so a retried publish still finds the bytes.
const storyMediaTTL = 30 * time.Minute

// igStoryService is the slice of the Instagram service the trigger needs.
type igStoryService interface {
	FeatureEnabled() bool
	HasCredentials(ctx context.Context, userID string) bool
}

// StoryTrigger fans a {story}-tagged source message out to an Instagram story
// per owning account. It runs inside the monitoring fan-out (fail-open): every
// error only logs — it never touches the WhatsApp mirror.
type StoryTrigger struct {
	automationRepo repository.AutomationRepository
	igService      igStoryService
	storyRepo      repository.InstagramStoryPostRepository
	waProvider     whatsapp.WhatsAppProvider
	mediaStore     *instagram.MediaStore
	queue          *QueueClient
	logger         *zap.Logger
}

// NewStoryTrigger builds the trigger.
func NewStoryTrigger(
	automationRepo repository.AutomationRepository,
	igService igStoryService,
	storyRepo repository.InstagramStoryPostRepository,
	waProvider whatsapp.WhatsAppProvider,
	mediaStore *instagram.MediaStore,
	queue *QueueClient,
	logger *zap.Logger,
) *StoryTrigger {
	return &StoryTrigger{
		automationRepo: automationRepo,
		igService:      igService,
		storyRepo:      storyRepo,
		waProvider:     waProvider,
		mediaStore:     mediaStore,
		queue:          queue,
		logger:         logger,
	}
}

// HandleStoryMessage resolves the owning accounts of the source group, prepares
// the media once, and enqueues one publish job per connected owner.
//
// A dedupe NÃO mora mais no Redis: o índice único do ledger decide quem publica.
// O claim antigo tinha TTL de 1h e falhava aberto, então redelivery tardio,
// corrida entre chips ou Redis fora do ar publicavam o mesmo Story duas vezes.
func (t *StoryTrigger) HandleStoryMessage(ctx context.Context, msg *models.IncomingMessage) {
	if !t.igService.FeatureEnabled() {
		return
	}
	// MessageID vazio não participa do índice único parcial — sem ele não há
	// idempotência possível, então o evento simplesmente não é elegível.
	if msg == nil || msg.RemoteJID == "" || msg.MessageID == "" {
		return
	}

	owners := t.connectedOwners(ctx, msg.RemoteJID)
	if len(owners) == 0 {
		return
	}

	lower := strings.ToLower(msg.MediaType)
	isVideo := msg.HasMedia && strings.HasPrefix(lower, "video")
	isImage := msg.HasMedia && strings.HasPrefix(lower, "image")

	// Prepare the media ONCE — shared across every owner of this group.
	var mediaKey, text, mediaSource string
	switch {
	case isVideo:
		// Vídeo fora do v1: o Story só publica imagem por enquanto.
		// Identificadores de grupo/mensagem ficam FORA do log: a lista permitida
		// é userID, ledgerID, operação, classe/código e contagens.
		t.logger.Info("story trigger: video not supported yet, skipping",
			zap.Int("owners", len(owners)))
		return
	case isImage:
		data, _ := t.fetchImage(ctx, msg)
		if len(data) == 0 {
			t.logger.Warn("story trigger: could not fetch image, skipping story",
				zap.Int("owners", len(owners)))
			return
		}
		// Converte para JPEG AQUI, uma vez para todos os donos: o store só
		// aceita JPEG e a rota pública sempre serve image/jpeg.
		jpegBytes, err := instagram.NormalizeJPEG(data)
		if err != nil {
			t.logger.Warn("story trigger: image rejected", zap.String("class", "media_invalid"))
			return
		}
		key, err := t.mediaStore.Put(ctx, jpegBytes, storyMediaTTL)
		if err != nil {
			t.capture("story media put failed", err, msg)
			return
		}
		mediaKey = key
		mediaSource = models.InstagramMediaSourceImage
	default:
		text = strings.TrimSpace(msg.Body)
		if text == "" {
			return
		}
		mediaSource = models.InstagramMediaSourceRenderedText
	}

	for _, userID := range owners {
		t.dispatchForOwner(ctx, userID, mediaKey, text, mediaSource, msg)
	}
}

// dispatchForOwner grava o evento no ledger e enfileira o job — só quando ESTA
// chamada criou a linha. Linha preexistente significa que outro chip/automação
// do mesmo dono já atendeu o evento.
func (t *StoryTrigger) dispatchForOwner(ctx context.Context, userID, mediaKey, text, mediaSource string, msg *models.IncomingMessage) {
	row, created, err := t.storyRepo.GetOrCreate(ctx, &models.InstagramStoryPost{
		UserID:      userID,
		SourceGroup: msg.RemoteJID,
		MessageID:   msg.MessageID,
		MediaSource: mediaSource,
		Status:      models.InstagramStoryStatusPending,
	})
	if err != nil {
		t.capture("story ledger get-or-create failed", err, msg)
		return
	}
	if !created {
		return
	}

	if _, err := t.queue.EnqueueInstagramStory(ctx, &InstagramStoryJobData{
		UserID:      userID,
		MediaKey:    mediaKey,
		Text:        text,
		SourceGroup: msg.RemoteJID,
		MessageID:   msg.MessageID,
		LedgerID:    row.ID,
	}); err != nil {
		// Falha REAL de enqueue (conflito de id já volta como sucesso). O ledger
		// recebe um código enumerado; o erro cru fica só no log estruturado.
		t.capture("story enqueue failed", err, msg)
		_ = t.storyRepo.MarkResult(ctx, row.ID, models.InstagramStoryStatusFailed, "",
			models.InstagramErrEnqueueFailed,
			models.InstagramSafeErrorMessage(models.InstagramErrEnqueueFailed), false)
	}
}

// connectedOwners returns the distinct user ids of active monitoring +
// offer_generate automations watching this group that have Instagram connected.
func (t *StoryTrigger) connectedOwners(ctx context.Context, groupJID string) []string {
	seen := map[string]bool{}
	var ordered []string
	add := func(autos []models.Automation) {
		for i := range autos {
			uid := autos[i].UserID
			if uid == "" || seen[uid] {
				continue
			}
			seen[uid] = true
			ordered = append(ordered, uid)
		}
	}
	if mon, err := t.automationRepo.GetMonitoringAutomationsForGroup(ctx, groupJID); err == nil {
		add(mon)
	} else {
		t.logger.Warn("story trigger: monitoring automations lookup failed", zap.Error(err))
	}
	if og, err := t.automationRepo.GetOfferGenerateAutomationsForGroup(ctx, groupJID); err == nil {
		add(og)
	} else {
		t.logger.Warn("story trigger: offer_generate automations lookup failed", zap.Error(err))
	}

	connected := ordered[:0]
	for _, uid := range ordered {
		if t.igService.HasCredentials(ctx, uid) {
			connected = append(connected, uid)
		}
	}
	return connected
}

// fetchImage downloads the message image, preferring the internal WAHA URL and
// falling back to a re-download from WhatsApp servers.
func (t *StoryTrigger) fetchImage(ctx context.Context, msg *models.IncomingMessage) ([]byte, string) {
	if msg.MediaURL != "" {
		if res, err := t.waProvider.DownloadMediaByURL(ctx, msg.MediaURL); err == nil && res != nil && len(res.Data) > 0 {
			return res.Data, res.Mimetype
		}
	}
	if res, err := t.waProvider.RedownloadMedia(ctx, msg.InstanceName, msg.RemoteJID, msg.MessageID); err == nil && res != nil && len(res.Data) > 0 {
		return res.Data, res.Mimetype
	}
	return nil, ""
}

func (t *StoryTrigger) capture(what string, err error, msg *models.IncomingMessage) {
	// A tag do grupo saiu daqui: o JID identifica um grupo de cliente e o Sentry
	// é um destino externo.
	observability.LogAndCapture(t.logger, sentry.LevelWarning, "instagram story: "+what, err, map[string]string{
		"module":    "instagram",
		"operation": "story_trigger",
	})
}
