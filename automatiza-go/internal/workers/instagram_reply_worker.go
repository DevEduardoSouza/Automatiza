package workers

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"automatiza-go/internal/lib/cache"
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"
	"automatiza-go/internal/services/instagram"

	"github.com/hibiken/asynq"
	"go.uber.org/zap"
)

const (
	// dmReplyWindow é a regra DURA da Meta: 24h desde a mensagem do usuário.
	// Fora dela o envio é recusado por eles — gastar a chamada só produz erro.
	dmReplyWindow = 24 * time.Hour
	// replyLockTTL serializa as respostas de uma MESMA conta. Duas respostas
	// simultâneas competiriam pela cota e poderiam sair fora de ordem na thread.
	replyLockTTL = 90 * time.Second
	// defaultDailyReplyCap vale quando o plano não define. A chave de IA é
	// nossa: sem teto, um post viral vira custo ilimitado.
	defaultDailyReplyCap = 50
	// replyMaxChars: comentário do Instagram tem teto; cortar do nosso lado sai
	// melhor que ser cortado pela Meta no meio de uma frase.
	replyMaxChars = 480
)

// InstagramReplier é a fatia do service que o worker usa. Nenhum método devolve
// token: o worker manda userID e o service resolve a credencial.
type InstagramReplier interface {
	FeatureEnabled() bool
	AIReplyEnabled() bool
	ReplyToEvent(ctx context.Context, userID string, ev *models.InstagramInboundEvent) (*instagram.ReplyOutcome, error)
	DailyReplyCap(ctx context.Context, userID string) int
}

// InstagramReplyWorker consome instagram:ai_reply.
//
// Contrato de retorno igual ao do worker de Story: erro ao asynq SOMENTE em
// falha transitória antes da última tentativa. Todo desfecho terminal é gravado
// e retorna nil — o payload carrega texto de terceiro e não deve ser arquivado.
type InstagramReplyWorker struct {
	svc         InstagramReplier
	inboundRepo repository.InstagramInboundRepository
	replyRepo   repository.InstagramReplyRepository
	cache       *cache.Cache
	logger      *zap.Logger

	retryState func(ctx context.Context) (retried, maxRetry int, ok bool)
}

// NewInstagramReplyWorker builds the reply worker.
func NewInstagramReplyWorker(
	svc InstagramReplier,
	inboundRepo repository.InstagramInboundRepository,
	replyRepo repository.InstagramReplyRepository,
	c *cache.Cache,
	logger *zap.Logger,
) *InstagramReplyWorker {
	return &InstagramReplyWorker{
		svc: svc, inboundRepo: inboundRepo, replyRepo: replyRepo,
		cache: c, logger: logger, retryState: asynqRetryState,
	}
}

// ProcessTask implements the asynq handler.
func (w *InstagramReplyWorker) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var data InstagramReplyJobData
	if err := json.Unmarshal(t.Payload(), &data); err != nil {
		w.logger.Error("instagram reply: undecodable payload")
		return nil
	}
	if data.UserID == "" || data.ProviderEventID == "" {
		w.logger.Error("instagram reply: payload missing user or event")
		return nil
	}

	// Kill switches relidos AQUI, não no enfileiramento: o job pode ter ficado
	// na fila enquanto alguém desligava a feature.
	if !w.svc.FeatureEnabled() || !w.svc.AIReplyEnabled() {
		w.skip(ctx, &data, models.InstagramErrAIDisabled)
		return nil
	}

	// Trava pública contra resposta duplicada. Só quem cria o claim responde;
	// os demais param aqui — inclusive um redelivery da Meta.
	reply, created, err := w.replyRepo.Claim(ctx, data.UserID, data.ProviderEventID, data.EventType)
	if err != nil {
		return fmt.Errorf("instagram reply: claim: %w", err) // transitório
	}
	if !created {
		return nil // outro executor já atendeu este evento
	}

	// Lock por conta: duas respostas simultâneas da mesma conta competiriam
	// pela cota e poderiam sair fora de ordem.
	release, err := w.acquireAccountLock(ctx, data.UserID)
	if err != nil {
		return w.finishTransient(ctx, reply.ID, err)
	}
	defer release()

	outcome, err := w.run(ctx, &data, reply.ID)
	if err != nil {
		return w.finishTransient(ctx, reply.ID, err)
	}
	_ = outcome
	return nil
}

// run executa o caminho principal. Devolve erro apenas para o que é transitório.
func (w *InstagramReplyWorker) run(ctx context.Context, data *InstagramReplyJobData, replyID string) (*instagram.ReplyOutcome, error) {
	event := &models.InstagramInboundEvent{
		ID:              data.InboundID,
		UserID:          data.UserID,
		ProviderEventID: data.ProviderEventID,
		EventType:       data.EventType,
		SenderID:        data.SenderID,
		ThreadID:        data.ThreadID,
		Content:         data.Content,
		OccurredAt:      data.OccurredAt,
	}

	if event.Content == "" {
		w.finish(ctx, replyID, models.InstagramReplySkipped, "", models.InstagramErrEmptyContent, 0, 0)
		w.markInbound(ctx, data.InboundID, models.InstagramInboundSkipped, models.InstagramErrEmptyContent)
		return nil, nil
	}

	// Janela de 24h: regra dura da Meta para Direct. Comentário e menção não
	// têm prazo, por isso o gate é só para message.
	if event.EventType == models.InstagramEventMessage && !withinDMWindow(event.OccurredAt, time.Now()) {
		w.finish(ctx, replyID, models.InstagramReplySkipped, "", models.InstagramErrWindowExpired, 0, 0)
		w.markInbound(ctx, data.InboundID, models.InstagramInboundSkipped, models.InstagramErrWindowExpired)
		w.logger.Info("instagram reply: fora da janela de 24h",
			zap.String("userID", data.UserID), zap.String("type", event.EventType))
		return nil, nil
	}

	// Cota diária — a chave de IA é NOSSA.
	cap := w.svc.DailyReplyCap(ctx, data.UserID)
	if cap <= 0 {
		cap = defaultDailyReplyCap
	}
	used, err := w.replyRepo.CountSince(ctx, data.UserID, time.Now().Add(-24*time.Hour))
	if err != nil {
		return nil, err // transitório: sem saber a cota, não gasta IA
	}
	if used >= cap {
		w.finish(ctx, replyID, models.InstagramReplySkipped, "", models.InstagramErrQuotaDaily, 0, 0)
		w.markInbound(ctx, data.InboundID, models.InstagramInboundSkipped, models.InstagramErrQuotaDaily)
		w.logger.Warn("instagram reply: cota diaria atingida",
			zap.String("userID", data.UserID), zap.Int("usadas", used), zap.Int("cota", cap))
		return nil, nil
	}

	outcome, err := w.svc.ReplyToEvent(ctx, data.UserID, event)
	if err != nil {
		if code, terminal := replyTerminalCode(err); terminal {
			w.finish(ctx, replyID, models.InstagramReplyFailed, "", code, 0, 0)
			w.markInbound(ctx, data.InboundID, models.InstagramInboundFailed, code)
			return nil, nil
		}
		return nil, err
	}
	if outcome == nil || outcome.Skipped {
		code := models.InstagramErrChannelOff
		if outcome != nil && outcome.SkipCode != "" {
			code = outcome.SkipCode
		}
		w.finish(ctx, replyID, models.InstagramReplySkipped, "", code, 0, 0)
		w.markInbound(ctx, data.InboundID, models.InstagramInboundSkipped, code)
		return outcome, nil
	}

	w.finish(ctx, replyID, models.InstagramReplySent, outcome.Text, "",
		outcome.PromptTokens, outcome.CompletionTokens)
	w.markInbound(ctx, data.InboundID, models.InstagramInboundReplied, "")
	// Só contagens e classe: o texto respondido fica no banco, não no log.
	w.logger.Info("instagram reply enviada",
		zap.String("userID", data.UserID), zap.String("type", event.EventType),
		zap.Int("tokens", outcome.PromptTokens+outcome.CompletionTokens))
	return outcome, nil
}

// withinDMWindow decide se ainda dá para responder um Direct.
//
// Timestamp ausente é tratado como FORA da janela: sem saber quando a mensagem
// chegou, responder pode violar a regra da Meta, e a recusa cai na conta do
// cliente. Fail-closed é o certo aqui.
func withinDMWindow(occurredAt *time.Time, now time.Time) bool {
	if occurredAt == nil {
		return false
	}
	return now.Sub(*occurredAt) < dmReplyWindow
}

// replyTerminalCode mapeia erro definitivo para código seguro. Só errors.Is —
// nunca texto, que muda ao sabor da Meta.
func replyTerminalCode(err error) (string, bool) {
	switch {
	case errors.Is(err, instagram.ErrInstagramFeatureDisabled):
		return models.InstagramErrFeatureDisabled, true
	case errors.Is(err, instagram.ErrInstagramTokenExpired):
		return models.InstagramErrTokenExpired, true
	case errors.Is(err, instagram.ErrInstagramAuth):
		return models.InstagramErrAuthRequired, true
	case errors.Is(err, instagram.ErrInstagramRateLimit):
		return models.InstagramErrQuotaMeta, true
	case errors.Is(err, instagram.ErrInstagramAIUnavailable):
		return models.InstagramErrAIFailed, true
	case errors.Is(err, instagram.ErrInstagramPermanent):
		return models.InstagramErrSendRejected, true
	default:
		return "", false
	}
}

func (w *InstagramReplyWorker) finish(ctx context.Context, replyID, status, text, code string, pt, ct int) {
	if err := w.replyRepo.MarkResult(context.WithoutCancel(ctx), replyID, status, text, code, pt, ct); err != nil {
		w.logger.Error("instagram reply: persist result failed",
			zap.String("replyID", replyID), zap.String("code", code), zap.Error(err))
	}
}

func (w *InstagramReplyWorker) markInbound(ctx context.Context, inboundID, status, code string) {
	if inboundID == "" {
		return
	}
	if err := w.inboundRepo.MarkStatus(context.WithoutCancel(ctx), inboundID, status, code); err != nil {
		w.logger.Warn("instagram reply: mark inbound failed", zap.String("inboundID", inboundID))
	}
}

func (w *InstagramReplyWorker) skip(ctx context.Context, data *InstagramReplyJobData, code string) {
	w.markInbound(ctx, data.InboundID, models.InstagramInboundSkipped, code)
}

// finishTransient devolve erro enquanto houver retry; na última tentativa grava
// terminal e retorna nil, para o asynq não arquivar um payload com texto de
// terceiro.
func (w *InstagramReplyWorker) finishTransient(ctx context.Context, replyID string, err error) error {
	if retried, maxRetry, ok := w.retryState(ctx); ok && retried >= maxRetry {
		w.finish(ctx, replyID, models.InstagramReplyFailed, "", models.InstagramErrTransientExhausted, 0, 0)
		w.logger.Warn("instagram reply: retries esgotados", zap.String("replyID", replyID))
		return nil
	}
	return err
}

// acquireAccountLock serializa por conta. Fail-closed: sem Redis operacional
// não há lock, e responder sem serializar arrisca duas respostas na mesma
// thread e estouro de cota.
func (w *InstagramReplyWorker) acquireAccountLock(ctx context.Context, userID string) (func(), error) {
	if !w.cache.Operational() {
		return nil, fmt.Errorf("instagram reply: lock indisponivel: %w", instagram.ErrInstagramTransient)
	}
	sum := sha256.Sum256([]byte(userID))
	key := "ig_reply_account_lock:" + hex.EncodeToString(sum[:])

	tokenBytes := make([]byte, 16)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, err
	}
	token := hex.EncodeToString(tokenBytes)

	ok, err := w.cache.SetNX(ctx, key, token, replyLockTTL)
	if err != nil {
		return nil, fmt.Errorf("instagram reply: lock erro: %w", instagram.ErrInstagramTransient)
	}
	if !ok {
		return nil, fmt.Errorf("instagram reply: conta ocupada: %w", instagram.ErrInstagramTransient)
	}
	return func() {
		// compare-and-delete: DEL cru apagaria o lock que outro worker já
		// readquiriu depois de o nosso expirar.
		if _, err := w.cache.CompareAndDelete(context.WithoutCancel(ctx), key, token); err != nil {
			w.logger.Warn("instagram reply: liberar lock falhou", zap.Error(err))
		}
	}, nil
}
