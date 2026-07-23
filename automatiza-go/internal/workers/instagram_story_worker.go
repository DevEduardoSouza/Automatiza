package workers

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"automatiza-go/internal/lib/cache"
	"automatiza-go/internal/lib/imagerender"
	"automatiza-go/internal/models"
	"automatiza-go/internal/observability"
	"automatiza-go/internal/repository"
	"automatiza-go/internal/services/instagram"

	"github.com/getsentry/sentry-go"
	"github.com/hibiken/asynq"
	"go.uber.org/zap"
)

const (
	// storyDailyCap is a local guard below Instagram's 100 posts/24h limit. A
	// fonte autoritativa é GetPublishingLimit; a contagem local só evita gastar
	// uma criação de container quando já publicamos muito para este usuário.
	storyDailyCap = 90
	// storyMetaQuotaCap é o limite oficial da janela móvel de 24h por conta.
	storyMetaQuotaCap = 100

	// accountLockTTL > timeout da task (90s), para o lock não expirar embaixo de
	// um job ainda em execução. staleLeaseAfter usa o mesmo valor: passado esse
	// tempo, o dono anterior comprovadamente não está mais vivo.
	accountLockTTL  = 2 * time.Minute
	staleLeaseAfter = 2 * time.Minute

	containerPollEvery  = 3 * time.Second
	containerPollBudget = 60 * time.Second
)

// errStoryLockBusy: outra task da MESMA conta está publicando. Transitório —
// o asynq tenta de novo depois.
var errStoryLockBusy = errors.New("instagram story: account busy")

// InstagramStoryPublisher is the slice of the Instagram service the worker
// needs. Repare que nenhum método devolve token: o worker manda userID e o
// service resolve a credencial internamente.
type InstagramStoryPublisher interface {
	FeatureEnabled() bool
	GetPublishingLimit(ctx context.Context, userID string) (int, error)
	EnsureStoryContainer(ctx context.Context, userID, publicImageURL, existingContainerID string) (string, error)
	GetStoryContainerStatus(ctx context.Context, userID, containerID string) (string, error)
	PublishStoryContainer(ctx context.Context, userID, containerID string) (string, error)
}

// InstagramStoryWorker consumes instagram:publish_story tasks: it materializes
// a public JPEG URL (mirrored image or text-rendered), publishes the story, and
// records the result in the ledger.
type InstagramStoryWorker struct {
	svc           InstagramStoryPublisher
	store         *instagram.MediaStore
	storyRepo     repository.InstagramStoryPostRepository
	cache         *cache.Cache
	publicBaseURL string
	logger        *zap.Logger

	// retryState existe como campo, e não como chamada direta ao asynq, porque
	// o contexto que carrega retry/max vem de um pacote INTERNO do asynq e não
	// pode ser construído de fora. Sem esta costura, a regra mais importante do
	// worker — virar terminal na última tentativa — ficaria sem teste.
	retryState func(ctx context.Context) (retried, maxRetry int, ok bool)
}

func asynqRetryState(ctx context.Context) (int, int, bool) {
	retried, okR := asynq.GetRetryCount(ctx)
	maxRetry, okM := asynq.GetMaxRetry(ctx)
	return retried, maxRetry, okR && okM
}

// NewInstagramStoryWorker builds the worker.
func NewInstagramStoryWorker(
	svc InstagramStoryPublisher,
	store *instagram.MediaStore,
	storyRepo repository.InstagramStoryPostRepository,
	c *cache.Cache,
	publicBaseURL string,
	logger *zap.Logger,
) *InstagramStoryWorker {
	return &InstagramStoryWorker{
		svc:           svc,
		store:         store,
		storyRepo:     storyRepo,
		cache:         c,
		publicBaseURL: strings.TrimRight(publicBaseURL, "/"),
		logger:        logger,
		retryState:    asynqRetryState,
	}
}

// ProcessTask implements the asynq handler.
//
// Contrato de retorno: erro devolvido ao asynq SOMENTE em falha transitória
// antes da última tentativa. Todo desfecho terminal é gravado no ledger e
// retorna nil — o job Instagram não deve ir para a fila de arquivados, porque
// seu payload pode conter o texto da mensagem do cliente.
func (w *InstagramStoryWorker) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var data InstagramStoryJobData
	if err := json.Unmarshal(t.Payload(), &data); err != nil {
		// Payload malformado nunca vira válido e não há ledger para marcar.
		w.logger.Error("instagram story: undecodable payload")
		return nil
	}
	if data.LedgerID == "" || data.UserID == "" {
		w.logger.Error("instagram story: payload missing ledger or user")
		return nil
	}

	ledger, err := w.storyRepo.GetByID(ctx, data.LedgerID)
	if err != nil {
		return fmt.Errorf("instagram story: load ledger: %w", err) // transitório
	}
	if ledger == nil {
		// Ledger sumiu (retenção) — nada a publicar nem a registrar.
		w.logger.Warn("instagram story: ledger row not found", zap.String("ledgerID", data.LedgerID))
		return nil
	}
	// O ledger é a autoridade; o payload é apenas transporte. Divergência
	// significa job adulterado ou id trocado — publicar aqui postaria o
	// conteúdo de um evento na conta de outro.
	if ledger.UserID != data.UserID ||
		(data.SourceGroup != "" && ledger.SourceGroup != data.SourceGroup) ||
		(data.MessageID != "" && ledger.MessageID != data.MessageID) {
		if err := w.finish(ctx, ledger.ID, models.InstagramStoryStatusFailed, "", models.InstagramErrInvalidPayload, false); err != nil {
			return w.persistFailure(ctx, ledger.ID, err)
		}
		return nil
	}
	if ledger.Status != models.InstagramStoryStatusPending {
		return nil // já terminal: idempotente
	}

	// Lock por conta ANTES do lease: dois Stories da mesma conta correndo juntos
	// competem pela mesma cota e podem gravar o ledger fora de ordem.
	release, err := w.acquireAccountLock(ctx, data.UserID)
	if err != nil {
		// claimed=false: NÃO temos lease. Chamar ReleaseProcessing aqui
		// apagaria o lease de quem está de fato publicando.
		return w.handleFailure(ctx, ledger.ID, err, false, time.Time{})
	}
	defer release()

	claimed, lease, err := w.storyRepo.TryStartProcessing(ctx, ledger.ID, time.Now().Add(-staleLeaseAfter))
	if err != nil {
		return w.handleFailure(ctx, ledger.ID, fmt.Errorf("instagram story: claim lease: %w", err), false, time.Time{})
	}
	if !claimed {
		// Lease fresco de outro executor, ou linha já terminal.
		return nil
	}

	if err := w.publish(ctx, &data, ledger); err != nil {
		return w.handleFailure(ctx, ledger.ID, err, true, lease)
	}
	return nil
}

// publish roda o caminho feliz. Qualquer erro devolvido é classificado por
// handleFailure — esta função não decide terminalidade.
func (w *InstagramStoryWorker) publish(ctx context.Context, data *InstagramStoryJobData, ledger *models.InstagramStoryPost) error {
	// Kill switch relido IMEDIATAMENTE antes da primeira chamada externa: o job
	// pode ter ficado na fila enquanto alguém desligava a feature.
	if !w.svc.FeatureEnabled() {
		return w.finish(ctx, ledger.ID, models.InstagramStoryStatusFailed, "", models.InstagramErrFeatureDisabled, false)
	}

	// Guard local barato antes de qualquer request.
	n, err := w.storyRepo.CountPublishedSince(ctx, data.UserID, time.Now().Add(-24*time.Hour))
	if err != nil {
		// Não é fatal (a cota da Meta logo abaixo é a fonte autoritativa e é
		// fail-closed), mas silenciar esconderia um Postgres degradado.
		w.logger.Warn("instagram story: local quota lookup failed", zap.String("ledgerID", ledger.ID), zap.Error(err))
	} else if n >= storyDailyCap {
		return w.finish(ctx, ledger.ID, models.InstagramStoryStatusFailed, "", models.InstagramErrQuotaLocal, false)
	}

	// Fonte oficial da cota. Falhar aqui é transitório de propósito: criar um
	// container sem saber a cota é o que gera erro caro do lado da Meta.
	used, err := w.svc.GetPublishingLimit(ctx, data.UserID)
	if err != nil {
		return err
	}
	if used >= storyMetaQuotaCap {
		return w.finish(ctx, ledger.ID, models.InstagramStoryStatusFailed, "", models.InstagramErrQuotaMeta, false)
	}

	containerID := ledger.ContainerID
	if containerID == "" {
		// A imagem só é preparada quando NÃO há container. Com container já
		// criado, a publicação não depende mais dos bytes — e a mídia efêmera
		// pode ter expirado, o que transformaria um retry recuperável em falha.
		servingKey, cleanup, err := w.resolveServingKey(ctx, data)
		if err != nil {
			return err
		}
		if cleanup != nil {
			defer cleanup()
		}

		publicURL := w.publicBaseURL + "/api/v1/public/story-media/" + servingKey
		containerID, err = w.svc.EnsureStoryContainer(ctx, data.UserID, publicURL, "")
		if err != nil {
			return err
		}
		// Persistir ANTES de qualquer polling/publicação: se o processo morrer
		// no próximo passo, o retry reaproveita este container em vez de criar
		// um segundo e publicar o Story duplicado.
		if err := w.storyRepo.SaveContainerID(ctx, ledger.ID, containerID); err != nil {
			return err
		}
	}

	status, err := w.waitForContainer(ctx, data.UserID, containerID)
	if err != nil {
		return err
	}

	switch status {
	case "PUBLISHED":
		// A Meta já publicou (aceite anterior cujo retorno se perdeu). Não
		// republicar: reconciliar. O media id não é conhecido neste caminho.
		if err := w.finish(ctx, ledger.ID, models.InstagramStoryStatusPublished, "", "", true); err != nil {
			return err
		}
		w.logger.Info("instagram story reconciled as published",
			zap.String("userID", data.UserID), zap.String("ledgerID", ledger.ID))
		return nil
	case "ERROR":
		return w.finish(ctx, ledger.ID, models.InstagramStoryStatusFailed, "", models.InstagramErrContainerError, false)
	case "EXPIRED":
		return w.finish(ctx, ledger.ID, models.InstagramStoryStatusFailed, "", models.InstagramErrContainerExpired, false)
	case "FINISHED":
		// segue para publicação
	default:
		// Status vazio/desconhecido: não decidir nada sobre ele.
		return fmt.Errorf("instagram story: unexpected container status: %w", instagram.ErrInstagramTransient)
	}

	mediaID, err := w.svc.PublishStoryContainer(ctx, data.UserID, containerID)
	if err != nil {
		// Timeout/transitório DEPOIS de a Meta possivelmente ter aceitado:
		// conferir o status uma vez antes de devolver erro, senão o retry
		// publicaria de novo.
		if st, stErr := w.svc.GetStoryContainerStatus(ctx, data.UserID, containerID); stErr == nil && st == "PUBLISHED" {
			if ferr := w.finish(ctx, ledger.ID, models.InstagramStoryStatusPublished, "", "", true); ferr != nil {
				return ferr
			}
			w.logger.Info("instagram story reconciled after publish timeout",
				zap.String("userID", data.UserID), zap.String("ledgerID", ledger.ID))
			return nil
		}
		return err
	}
	if mediaID == "" {
		return fmt.Errorf("instagram story: empty media id: %w", instagram.ErrInstagramTransient)
	}

	if err := w.finish(ctx, ledger.ID, models.InstagramStoryStatusPublished, mediaID, "", false); err != nil {
		return err
	}
	w.logger.Info("instagram story published",
		zap.String("userID", data.UserID),
		zap.String("ledgerID", ledger.ID),
		zap.String("mediaSource", sourceOf(data)))
	return nil
}

// waitForContainer consulta o status até sair de IN_PROGRESS, dentro de um
// orçamento fixo. Sem busy-loop: intervalo constante e teto bem abaixo do
// timeout da task, para o retry do asynq assumir em vez de estourar o job.
func (w *InstagramStoryWorker) waitForContainer(ctx context.Context, userID, containerID string) (string, error) {
	deadline := time.Now().Add(containerPollBudget)
	for {
		status, err := w.svc.GetStoryContainerStatus(ctx, userID, containerID)
		if err != nil {
			return "", err
		}
		if status != "IN_PROGRESS" {
			return status, nil
		}
		if time.Now().After(deadline) {
			return "", fmt.Errorf("instagram story: container still in progress: %w", instagram.ErrInstagramTransient)
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(containerPollEvery):
		}
	}
}

// resolveServingKey devolve a chave pública da imagem.
//
// Quando o payload traz MediaKey, os bytes já foram normalizados e guardados
// pelo trigger e são COMPARTILHADOS entre todos os donos do grupo-fonte: servir
// direto e NUNCA apagar (expira por TTL). Apagar ali fazia o job do segundo
// dono falhar com "mídia expirou". Só a chave que este worker cria é apagada.
func (w *InstagramStoryWorker) resolveServingKey(ctx context.Context, data *InstagramStoryJobData) (string, func(), error) {
	if data.MediaKey != "" {
		if _, err := w.store.Get(ctx, data.MediaKey); err != nil {
			return "", nil, err
		}
		return data.MediaKey, nil, nil
	}
	if strings.TrimSpace(data.Text) == "" {
		return "", nil, instagram.ErrMediaInvalid
	}
	rendered, err := imagerender.RenderStoryImage(data.Text, imagerender.Options{})
	if err != nil {
		return "", nil, instagram.ErrMediaInvalid
	}
	jpegBytes, err := instagram.NormalizeJPEG(rendered)
	if err != nil {
		return "", nil, err
	}
	key, err := w.store.Put(ctx, jpegBytes, storyMediaTTL)
	if err != nil {
		return "", nil, err
	}
	return key, func() { w.store.Delete(context.WithoutCancel(ctx), key) }, nil
}

// handleFailure classifica o erro e decide entre terminal e retry.
//
// claimed diz se ESTA execução detém o lease. Só quem o detém pode devolvê-lo:
// liberar um lease alheio soltaria a linha para um terceiro worker enquanto o
// dono legítimo ainda está publicando.
func (w *InstagramStoryWorker) handleFailure(ctx context.Context, ledgerID string, err error, claimed bool, lease time.Time) error {
	if code, terminal := terminalCodeFor(err); terminal {
		if ferr := w.finish(ctx, ledgerID, models.InstagramStoryStatusFailed, "", code, false); ferr != nil {
			return w.persistFailure(ctx, ledgerID, ferr)
		}
		w.logger.Warn("instagram story failed permanently",
			zap.String("ledgerID", ledgerID), zap.String("code", code))
		return nil
	}

	// Transitório. Na ÚLTIMA tentativa o ledger precisa virar terminal, senão a
	// linha fica presa em pending para sempre e some do radar. Retornar nil
	// junto evita que o asynq arquive o job — o payload pode conter o texto da
	// mensagem do cliente.
	retried, maxRetry, ok := w.retryState(ctx)
	if ok && retried >= maxRetry {
		if ferr := w.finish(ctx, ledgerID, models.InstagramStoryStatusFailed, "", models.InstagramErrTransientExhausted, false); ferr != nil {
			return w.persistFailure(ctx, ledgerID, ferr)
		}
		w.logger.Warn("instagram story: transient retries exhausted", zap.String("ledgerID", ledgerID))
		return nil
	}

	if claimed {
		// Libera o lease para o retry poder reivindicar a linha imediatamente —
		// e SOMENTE se ele ainda for o nosso (ver ReleaseProcessing).
		if relErr := w.storyRepo.ReleaseProcessing(context.WithoutCancel(ctx), ledgerID, lease); relErr != nil {
			w.logger.Warn("instagram story: release lease failed", zap.String("ledgerID", ledgerID), zap.Error(relErr))
		}
	}
	w.logger.Warn("instagram story transient failure",
		zap.String("ledgerID", ledgerID), zap.Int("retried", retried))
	return err
}

// terminalCodeFor mapeia o erro para um código seguro quando ele for definitivo.
// A decisão é só por errors.Is — nunca por texto, que muda ao sabor da Meta.
func terminalCodeFor(err error) (string, bool) {
	switch {
	case errors.Is(err, instagram.ErrInstagramFeatureDisabled):
		return models.InstagramErrFeatureDisabled, true
	case errors.Is(err, instagram.ErrInstagramTokenExpired):
		return models.InstagramErrTokenExpired, true
	case errors.Is(err, instagram.ErrInstagramAuth):
		return models.InstagramErrAuthRequired, true
	case errors.Is(err, instagram.ErrInstagramRateLimit):
		return models.InstagramErrQuotaMeta, true
	case errors.Is(err, instagram.ErrMediaNotFound):
		return models.InstagramErrMediaExpired, true
	case errors.Is(err, instagram.ErrMediaTooLarge):
		return models.InstagramErrMediaTooLarge, true
	case errors.Is(err, instagram.ErrMediaInvalid):
		return models.InstagramErrMediaInvalid, true
	case errors.Is(err, instagram.ErrInstagramPermanent):
		return models.InstagramErrPublishRejected, true
	default:
		return "", false
	}
}

// finish grava o desfecho terminal com contexto que sobrevive ao cancelamento
// da task — perder a gravação deixaria a linha pending sobre um Story publicado.
//
// DEVOLVE erro de propósito. Engolir a falha aqui era o pior desfecho possível:
// o Story sai publicado no Instagram, o job volta `nil` ao asynq e some, e a
// linha fica `pending` para sempre — a auditoria registrando o oposto do que
// aconteceu. Com o erro propagado, o retry reencontra o container em status
// PUBLISHED e reconcilia.
func (w *InstagramStoryWorker) finish(ctx context.Context, ledgerID, status, mediaID, code string, reconciled bool) error {
	if err := w.storyRepo.MarkResult(context.WithoutCancel(ctx), ledgerID, status, mediaID,
		code, models.InstagramSafeErrorMessage(code), reconciled); err != nil {
		w.logger.Error("instagram story: persist terminal state failed",
			zap.String("ledgerID", ledgerID), zap.String("code", code), zap.Error(err))
		return fmt.Errorf("instagram story: persist terminal state: %w", err)
	}
	return nil
}

// persistFailure decide o que fazer quando NEM o desfecho terminal conseguiu ser
// gravado: enquanto houver retry, devolve o erro; na última tentativa registra
// em observabilidade e desiste — devolver erro ali arquivaria o job, cujo
// payload pode conter o texto da mensagem do cliente.
func (w *InstagramStoryWorker) persistFailure(ctx context.Context, ledgerID string, err error) error {
	if retried, maxRetry, ok := w.retryState(ctx); ok && retried >= maxRetry {
		observability.LogAndCapture(w.logger, sentry.LevelError,
			"instagram story: ledger ficou pendente — persistência do estado terminal falhou até o fim", err,
			map[string]string{"module": "instagram", "operation": "story_worker", "ledgerID": ledgerID})
		return nil
	}
	return err
}

// acquireAccountLock serializa as publicações de uma MESMA conta profissional.
//
// Fail-closed: sem Redis não há lock, e sem lock duas tasks concorreriam pela
// cota e pela ordem de escrita do ledger. Melhor adiar do que publicar torto.
func (w *InstagramStoryWorker) acquireAccountLock(ctx context.Context, userID string) (func(), error) {
	// Operational, e não apenas "cache != nil": com APP_CACHE desligado o
	// SetNX devolve true SEM gravar nada, e o lock viraria um no-op que
	// deixaria duas tasks da mesma conta publicarem juntas.
	if !w.cache.Operational() {
		return nil, fmt.Errorf("instagram story: lock store unavailable: %w", instagram.ErrInstagramTransient)
	}
	// O UUID do cliente não vai em claro para o Redis.
	sum := sha256.Sum256([]byte(userID))
	key := "ig_story_account_lock:" + hex.EncodeToString(sum[:])

	tokenBytes := make([]byte, 16)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, err
	}
	token := hex.EncodeToString(tokenBytes)

	ok, err := w.cache.SetNX(ctx, key, token, accountLockTTL)
	if err != nil {
		return nil, fmt.Errorf("instagram story: lock error: %w", instagram.ErrInstagramTransient)
	}
	if !ok {
		return nil, errStoryLockBusy
	}
	return func() {
		// compare-and-delete: um DEL cru apagaria o lock que OUTRO worker já
		// readquiriu depois de o nosso ter expirado por TTL.
		if _, err := w.cache.CompareAndDelete(context.WithoutCancel(ctx), key, token); err != nil {
			w.logger.Warn("instagram story: lock release failed", zap.Error(err))
		}
	}, nil
}

func sourceOf(data *InstagramStoryJobData) string {
	if data.MediaKey != "" {
		return models.InstagramMediaSourceImage
	}
	return models.InstagramMediaSourceRenderedText
}
