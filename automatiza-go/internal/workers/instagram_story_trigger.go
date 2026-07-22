package workers

import (
	"context"
	"strings"
	"time"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/lib/cache"
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
	cache          *cache.Cache
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
	c *cache.Cache,
	logger *zap.Logger,
) *StoryTrigger {
	return &StoryTrigger{
		automationRepo: automationRepo,
		igService:      igService,
		storyRepo:      storyRepo,
		waProvider:     waProvider,
		mediaStore:     mediaStore,
		queue:          queue,
		cache:          c,
		logger:         logger,
	}
}

// HandleStoryMessage resolves the owning accounts of the source group, prepares
// the media once, and enqueues one publish job per connected owner (deduped).
func (t *StoryTrigger) HandleStoryMessage(ctx context.Context, msg *models.IncomingMessage) {
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
	case isImage:
		data, mime := t.fetchImage(ctx, msg)
		if len(data) == 0 {
			t.logger.Warn("story trigger: could not fetch image, skipping story",
				zap.String("group", msg.RemoteJID), zap.String("messageID", msg.MessageID))
			return
		}
		key, err := t.mediaStore.Put(ctx, data, mime, storyMediaTTL)
		if err != nil {
			t.capture("story media put failed", err, msg)
			return
		}
		mediaKey = key
		mediaSource = models.InstagramMediaSourceImage
	case isVideo:
		// Vídeo fora do v1: o Story só publica imagem por enquanto.
		t.logger.Info("story trigger: video not supported yet, skipping",
			zap.String("group", msg.RemoteJID), zap.String("messageID", msg.MessageID))
		return
	default:
		text = strings.TrimSpace(msg.Body)
		if text == "" {
			return
		}
		mediaSource = models.InstagramMediaSourceRenderedText
	}

	for _, userID := range owners {
		if !t.claim(ctx, userID, msg) {
			continue // outro chip/automação do mesmo dono já disparou este story
		}
		ledgerID, err := t.storyRepo.Create(ctx, &models.InstagramStoryPost{
			UserID:      userID,
			SourceGroup: msg.RemoteJID,
			MessageID:   msg.MessageID,
			MediaSource: mediaSource,
			Status:      models.InstagramStoryStatusPending,
		})
		if err != nil {
			t.capture("story ledger create failed", err, msg)
			continue
		}
		if _, err := t.queue.EnqueueInstagramStory(ctx, &InstagramStoryJobData{
			UserID:      userID,
			MediaKey:    mediaKey,
			Text:        text,
			SourceGroup: msg.RemoteJID,
			MessageID:   msg.MessageID,
			LedgerID:    ledgerID,
		}); err != nil {
			t.capture("story enqueue failed", err, msg)
			_ = t.storyRepo.MarkResult(ctx, ledgerID, models.InstagramStoryStatusFailed, "", "enqueue failed: "+err.Error())
		}
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

// claim dedups one story per (owner, group, message) for 1h — the same message
// arrives via multiple chips and via multiple automations. Fail-open.
func (t *StoryTrigger) claim(ctx context.Context, userID string, msg *models.IncomingMessage) bool {
	if t.cache == nil {
		return true
	}
	key := "ig_story_claim:" + userID + ":" + msg.RemoteJID + ":" + msg.MessageID
	ok, err := t.cache.SetNX(ctx, key, "1", time.Hour)
	if err != nil {
		return true // fail-open: prefer a possible dup over dropping the story
	}
	return ok
}

func (t *StoryTrigger) capture(what string, err error, msg *models.IncomingMessage) {
	observability.LogAndCapture(t.logger, sentry.LevelWarning, "instagram story: "+what, err, map[string]string{
		"module":      "instagram",
		"operation":   "story_trigger",
		"sourceGroup": msg.RemoteJID,
	})
}
