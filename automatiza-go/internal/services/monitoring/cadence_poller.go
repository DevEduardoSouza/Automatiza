package monitoring

import (
	"context"
	"time"

	"automatiza-go/internal/models"
	"automatiza-go/internal/observability"
	"automatiza-go/internal/repository"

	"go.uber.org/zap"
)

// Poller da cadência do espelho (Melhoria 7).
//
// Roda no processo SERVER (onde já vive o cron), nunca no worker: quem decide
// o que está devido precisa ser um lugar só por tick, e a coordenação entre
// réplicas é o SKIP LOCKED do próprio PostgreSQL — sem leader election, sem
// estado em memória.
//
// Repare no que o poller NÃO faz: ele não envia nada e não sabe o que a
// mensagem contém. Ele só transforma "item devido no banco" em "task na fila".

// DeferredEnqueuer é a fatia do cliente asynq usada aqui. Interface local para
// o serviço não depender do pacote workers (que depende deste).
type DeferredEnqueuer interface {
	EnqueueMonitorDeferredDispatch(ctx context.Context, pendingID int64, generation int, token string) error
}

// Cadências do poller (§6 do plano).
const (
	// CadencePollInterval: granularidade real da feature. scheduled_at é "não
	// antes de", nunca promessa de disparo no segundo exato.
	CadencePollInterval = 15 * time.Second
	// CadenceMaintenanceInterval: recuperação de leases e purga de mídia.
	CadenceMaintenanceInterval = time.Minute
	// CadenceCleanupInterval: retenção.
	CadenceCleanupInterval = 24 * time.Hour

	// cadencePollBatch: automações atendidas por tick.
	cadencePollBatch = 100
	// cadenceCleanupBatch/Rounds: apaga em lotes pequenos para nunca segurar
	// lock longo numa tabela que também está sendo escrita.
	cadenceCleanupBatch  = 1000
	cadenceCleanupRounds = 20
	// cadenceEnqueueRetryDelay: espera curta quando o enqueue falha (Redis
	// fora, por exemplo) antes de o item concorrer de novo.
	cadenceEnqueueRetryDelay = 15 * time.Second
)

// CadencePoller emite os gatilhos dos itens devidos e faz a manutenção do
// ledger.
type CadencePoller struct {
	repo     repository.MonitorCadenceRepository
	enqueuer DeferredEnqueuer
	logger   *zap.Logger
}

// NewCadencePoller cria o poller. repo/enqueuer nil = poller inerte.
func NewCadencePoller(repo repository.MonitorCadenceRepository, enqueuer DeferredEnqueuer, logger *zap.Logger) *CadencePoller {
	return &CadencePoller{repo: repo, enqueuer: enqueuer, logger: logger}
}

// Tick reivindica os heads devidos e enfileira o gatilho de cada um.
//
// Deliberadamente NÃO consulta a feature flag: com a flag desligada este é o
// mecanismo que DRENA, em ordem, a fila já admitida. Desligar a feature é
// rollback gracioso, não descarte de fila.
func (p *CadencePoller) Tick(ctx context.Context) error {
	if p.repo == nil || p.enqueuer == nil {
		return nil
	}

	leases, err := p.repo.ClaimDueHeads(ctx, cadencePollBatch)
	if err != nil {
		return err
	}
	if len(leases) == 0 {
		return nil
	}

	for _, lease := range leases {
		if err := p.enqueuer.EnqueueMonitorDeferredDispatch(ctx, lease.PendingID, lease.EnqueueGen, lease.EnqueueToken); err != nil {
			// Falha de enfileiramento devolve o item à fila com espera curta.
			// Se o processo morrer antes disso, a lease de 2 min faz o mesmo.
			p.logger.Warn("cadence: enqueue failed, releasing lease",
				zap.Int64("pendingID", lease.PendingID),
				zap.Int("generation", lease.EnqueueGen),
				zap.Error(err))
			observability.RecordMonitorCadenceEnqueue("error")
			if rErr := p.repo.ReleaseEnqueueLease(ctx, lease.PendingID, lease.EnqueueGen, lease.EnqueueToken,
				time.Now().Add(cadenceEnqueueRetryDelay)); rErr != nil {
				p.logger.Error("cadence: release enqueue lease failed",
					zap.Int64("pendingID", lease.PendingID), zap.Error(rErr))
			}
			continue
		}

		// Zero linhas aqui é legítimo: a task pode já ter executado e movido o
		// item para processing antes deste ack.
		if err := p.repo.MarkEnqueued(ctx, lease.PendingID, lease.EnqueueGen, lease.EnqueueToken); err != nil {
			p.logger.Warn("cadence: mark enqueued failed",
				zap.Int64("pendingID", lease.PendingID), zap.Error(err))
		}
		observability.RecordMonitorCadenceEnqueue("enqueued")
	}

	p.logger.Debug("cadence: heads enqueued", zap.Int("count", len(leases)))
	return nil
}

// Maintain recupera leases expiradas, cancela órfãos e zera mídia já entregue.
// É o que torna todo crash recuperável SEM intervenção humana.
func (p *CadencePoller) Maintain(ctx context.Context) error {
	if p.repo == nil {
		return nil
	}
	stats, err := p.repo.RecoverLeases(ctx)
	if err != nil {
		return err
	}
	observability.RecordMonitorCadenceRecovery("enqueue_lease", stats.EnqueueLeasesReleased)
	observability.RecordMonitorCadenceRecovery("destination_released", stats.DestinationsReleased)
	observability.RecordMonitorCadenceRecovery("destination_assumed_sent", stats.DestinationsAssumed)
	observability.RecordMonitorCadenceRecovery("pending_recovered", stats.PendingsRecovered)
	observability.RecordMonitorCadenceRecovery("pending_canceled", stats.PendingsCanceled)
	observability.RecordMonitorCadenceRecovery("media_purged", stats.MediaPurged)

	if stats.EnqueueLeasesReleased+stats.DestinationsReleased+stats.DestinationsAssumed+
		stats.PendingsRecovered+stats.PendingsCanceled+stats.MediaPurged > 0 {
		p.logger.Info("cadence: maintenance recovered work",
			zap.Int("enqueueLeases", stats.EnqueueLeasesReleased),
			zap.Int("destinationsReleased", stats.DestinationsReleased),
			zap.Int("destinationsAssumedSent", stats.DestinationsAssumed),
			zap.Int("pendingsRecovered", stats.PendingsRecovered),
			zap.Int("pendingsCanceled", stats.PendingsCanceled),
			zap.Int("mediaPurged", stats.MediaPurged))
	}
	return nil
}

// Cleanup apaga itens terminais além da retenção, em lotes.
func (p *CadencePoller) Cleanup(ctx context.Context) error {
	if p.repo == nil {
		return nil
	}
	before := time.Now().Add(-time.Duration(models.MonitorRetentionDays) * 24 * time.Hour)
	total := 0
	for i := 0; i < cadenceCleanupRounds; i++ {
		deleted, err := p.repo.Cleanup(ctx, before, cadenceCleanupBatch)
		if err != nil {
			return err
		}
		total += deleted
		if deleted < cadenceCleanupBatch {
			break
		}
	}
	if total > 0 {
		p.logger.Info("cadence: cleanup done", zap.Int("deleted", total))
	}
	return nil
}
