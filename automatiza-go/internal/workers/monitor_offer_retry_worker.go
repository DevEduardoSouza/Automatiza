package workers

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"
	"automatiza-go/internal/services/monitoroffer"

	"github.com/hibiken/asynq"
	"go.uber.org/zap"
)

// RetryExecutor é quem sabe reclassificar o link e entregá-lo aos destinos.
//
// Implementado pelo serviço (monitoring/offer_generate), que conhece chip,
// destino, template e cupom. O worker fica com a parte durável — claim,
// tentativa, desfecho — e não aprende a enviar.
type RetryExecutor interface {
	ExecuteRetry(ctx context.Context, att models.MonitorOfferAttempt, payload models.MonitorOfferPayload) (models.MonitorOfferItemOutcome, error)
}

// MonitorOfferRetryWorker processa a nova tentativa de UM link comercial.
//
// O ciclo tem uma assimetria deliberada: o claim (ClaimItemForAttempt) e a
// marcação da tentativa (MarkAttemptStarted) são passos SEPARADOS. Um crash
// entre os dois devolve o item a pending sem consumir tentativa — senão um
// processo morto no boot queimaria as 5 tentativas do item sem nunca ter falado
// com a Shopee.
type MonitorOfferRetryWorker struct {
	repo          repository.MonitorOfferRetryRepository
	executor      RetryExecutor
	encryptionKey string
	logger        *zap.Logger
}

// NewMonitorOfferRetryWorker cria o handler.
func NewMonitorOfferRetryWorker(
	repo repository.MonitorOfferRetryRepository,
	executor RetryExecutor,
	encryptionKey string,
	logger *zap.Logger,
) *MonitorOfferRetryWorker {
	return &MonitorOfferRetryWorker{repo: repo, executor: executor, encryptionKey: encryptionKey, logger: logger}
}

// ProcessTask executa uma tentativa.
//
// Devolve nil em quase todo desfecho de propósito: o Asynq está configurado com
// MaxRetry(0) e quem controla tentativa é a tabela. Devolver erro aqui só
// encheria o log — o item já carrega seu próprio backoff.
func (w *MonitorOfferRetryWorker) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var data MonitorOfferRetryJobData
	if err := json.Unmarshal(t.Payload(), &data); err != nil {
		// Payload corrompido não melhora com retry.
		w.logger.Error("monitor_offer_retry: payload da task inválido", zap.Error(err))
		return nil
	}

	att, err := w.repo.ClaimItemForAttempt(ctx, data.ItemID, data.Generation)
	if err != nil {
		w.logger.Warn("monitor_offer_retry: claim falhou",
			zap.String("item_id", data.ItemID), zap.Error(err))
		return nil
	}
	if att == nil {
		// Geração obsoleta ou item já terminal. É o desfecho que torna seguro
		// perder um ack de enqueue — não é erro.
		return nil
	}

	log := w.logger.With(
		zap.String("item_id", att.Item.ID),
		zap.String("job_id", att.Item.JobID),
		zap.String("user_id", att.Job.UserID),
		zap.String("store", att.Item.Store),
		// Nunca a URL: só o host e os 8 primeiros bytes do hash.
		zap.String("host", att.Item.Host),
		zap.String("link_hash", shortHash(att.Item.SourceLinkHash)),
	)

	if att.Job.PayloadCiphertext == nil || *att.Job.PayloadCiphertext == "" {
		w.finish(ctx, log, models.MonitorOfferItemOutcome{
			ItemID: att.Item.ID, Status: models.MonitorItemExhausted,
			URLClass: att.Item.URLClass, ErrorClass: models.ErrorClassTransient,
			ReasonCode: models.ReasonPayloadExpired, Outcome: models.MonitorOutcomeNone,
		})
		return nil
	}

	payload, err := monitoroffer.DecodePayload(*att.Job.PayloadCiphertext, w.encryptionKey)
	if err != nil {
		// Falha de decifra é permanente e ALARMÁVEL: significa chave trocada ou
		// dado corrompido. Nunca enviar às cegas.
		log.Error("monitor_offer_retry: payload não decifrou", zap.Error(err))
		w.finish(ctx, log, models.MonitorOfferItemOutcome{
			ItemID: att.Item.ID, Status: models.MonitorItemBlocked,
			URLClass: att.Item.URLClass, ErrorClass: models.ErrorClassPermanentConfig,
			ReasonCode: models.ReasonPayloadDecryptFail, Outcome: models.MonitorOutcomeNone,
		})
		return nil
	}

	// A partir daqui a tentativa CONTA: houve trabalho real.
	if err := w.repo.MarkAttemptStarted(ctx, att.Item.ID); err != nil {
		log.Warn("monitor_offer_retry: marcação da tentativa falhou", zap.Error(err))
		return nil
	}

	if w.executor == nil {
		// Sem executor injetado o componente está inerte (gate off no boot).
		// Devolve o item a pending sem gastar mais nada.
		w.reschedule(ctx, log, att.Item, models.ReasonNone)
		return nil
	}

	outcome, execErr := w.executor.ExecuteRetry(ctx, *att, payload)
	if execErr != nil {
		if errors.Is(execErr, context.Canceled) {
			// Shutdown não é falha de negócio: o lease vence e o sweep devolve
			// o item sem consumir mais uma tentativa.
			log.Info("monitor_offer_retry: cancelado no shutdown, o sweep recupera")
			return nil
		}
		log.Warn("monitor_offer_retry: execução falhou", zap.Error(execErr))
		w.reschedule(ctx, log, att.Item, models.ReasonTimeout)
		return nil
	}

	if outcome.Status == models.MonitorItemPending {
		w.reschedule(ctx, log, att.Item, outcome.ReasonCode)
		return nil
	}
	w.finish(ctx, log, outcome)
	return nil
}

// reschedule devolve o item a pending com o backoff determinístico.
func (w *MonitorOfferRetryWorker) reschedule(ctx context.Context, log *zap.Logger, item models.MonitorOfferItem, reason string) {
	if reason == "" {
		reason = models.ReasonNone
	}
	next := time.Now().UTC().Add(models.NextAttemptDelay(item.ID, int(item.Attempts)))
	if err := w.repo.RescheduleItem(ctx, item.ID, next, reason, true); err != nil {
		log.Warn("monitor_offer_retry: reagendamento falhou", zap.Error(err))
	}
}

func (w *MonitorOfferRetryWorker) finish(ctx context.Context, log *zap.Logger, out models.MonitorOfferItemOutcome) {
	if out.ReasonCode == "" {
		out.ReasonCode = models.ReasonNone
	}
	if out.Outcome == "" {
		out.Outcome = models.MonitorOutcomeNone
	}
	if err := w.repo.FinishItem(ctx, out); err != nil {
		log.Warn("monitor_offer_retry: gravação do desfecho falhou",
			zap.String("status", out.Status), zap.Error(err))
		return
	}
	log.Info("monitor_offer_retry: item encerrado",
		zap.String("status", out.Status), zap.String("reason", out.ReasonCode))
}

// shortHash devolve os 8 primeiros bytes (16 hex) do SHA-256 já armazenado.
func shortHash(full string) string {
	if len(full) >= 16 {
		return full[:16]
	}
	return full
}
