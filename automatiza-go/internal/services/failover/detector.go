package failover

import (
	"context"
	"time"

	"automatiza-go/internal/models"
	"automatiza-go/internal/observability"
	"automatiza-go/internal/repository"

	"go.uber.org/zap"
)

// Detector observa a primária das políticas em modo assisted e, quando a
// indisponibilidade é PERSISTENTE, cria uma proposta.
//
// O que ele nunca faz, e por quê:
//
//   - nunca troca nada. Só o humano confirma. Uma proposta é evidência, não
//     decisão.
//   - nunca lê dispatch_logs. Nos fan-outs síncronos um timeout com desfecho
//     incerto é gravado como `failed` (o slot de dedupe fica retido de
//     propósito), então contar 'failed' confundiria "não sei se entregou" com
//     "o chip morreu".
//   - nunca olha texto de erro. Nenhuma string de provider é evidência de nada.
//   - nunca conclui banimento. O enum da WAHA (waha-plus-openapi.json) não tem
//     BANNED; o que existe é STOPPED/STARTING/SCAN_QR_CODE/PASSKEY_*/WORKING/
//     FAILED. "Indisponível" é o teto do que se pode afirmar.
type Detector struct {
	repo         repository.InstanceFailoverRepository
	instanceRepo repository.InstanceRepository
	provider     ProviderReader
	logger       *zap.Logger
	enabled      func() bool
	now          func() time.Time
}

// NewDetector cria o detector assisted.
func NewDetector(
	repo repository.InstanceFailoverRepository,
	instanceRepo repository.InstanceRepository,
	provider ProviderReader,
	enabled func() bool,
	logger *zap.Logger,
) *Detector {
	return &Detector{
		repo:         repo,
		instanceRepo: instanceRepo,
		provider:     provider,
		logger:       logger,
		enabled:      enabled,
		now:          time.Now,
	}
}

// Run é o tick do cron (60s). No-op com a feature desligada.
func (d *Detector) Run(ctx context.Context) error {
	if d.enabled == nil || !d.enabled() {
		return nil
	}

	now := d.now().UTC()
	if expired, err := d.repo.ExpireProposals(ctx, now); err != nil {
		d.logger.Warn("failover detector: expiring proposals failed", zap.Error(err))
	} else {
		for i := 0; i < expired; i++ {
			observability.RecordFailoverProposal("expired")
		}
	}

	policies, err := d.repo.ListAssistedPolicies(ctx)
	if err != nil {
		return err
	}

	for i := range policies {
		policy := &policies[i]
		if err := d.probe(ctx, policy); err != nil {
			// Uma política com problema não pode parar as outras.
			d.logger.Warn("failover detector: probe failed",
				zap.Int64("policyID", policy.ID), zap.Error(err))
		}
	}
	return nil
}

// probe faz UMA observação da primária de uma política.
func (d *Detector) probe(ctx context.Context, policy *models.InstanceFailoverPolicy) error {
	instance, err := d.instanceRepo.GetInstanceByID(ctx, policy.PrimaryInstanceID)
	if err != nil {
		return err
	}
	// Sem linha local ou sem sessão não há observação VÁLIDA: indeterminado.
	if instance == nil || instance.UserID != policy.UserID || instance.InstanceID == "" {
		_, _, err := d.repo.RecordProbe(ctx, repository.FailoverProbeInput{
			PolicyID: policy.ID, UserID: policy.UserID,
			Healthy: false, Valid: false, Status: "unknown", Now: d.now().UTC(), RequireDetectorLock: true,
		})
		return err
	}

	dbHealthy := instance.IsConnected()

	fresh, err := d.provider.GetSessionStatusFresh(ctx, instance.InstanceID)
	if err != nil {
		// Timeout/erro/sessão ausente ZERA a acumulação. Um shard WAHA
		// reiniciando não pode virar "a primária caiu" para toda a base.
		_, _, rerr := d.repo.RecordProbe(ctx, repository.FailoverProbeInput{
			PolicyID: policy.ID, UserID: policy.UserID,
			Healthy: false, Valid: false, Status: "provider_error", Now: d.now().UTC(), RequireDetectorLock: true,
		})
		return rerr
	}

	in := repository.FailoverProbeInput{
		PolicyID:            policy.ID,
		UserID:              policy.UserID,
		Now:                 d.now().UTC(),
		Status:              fresh.RawStatus,
		RequireDetectorLock: true,
	}

	switch {
	case fresh.IsWorking() && dbHealthy:
		in.Healthy, in.Valid = true, true
	case !fresh.Known:
		// Status que não reconhecemos: indeterminado, nunca evidência.
		in.Healthy, in.Valid = false, false
	case fresh.IsWorking() != dbHealthy:
		// Divergência banco × WAHA: um dos dois está velho. Não conta.
		in.Healthy, in.Valid = false, false
	default:
		// Estado CONHECIDO e diferente de WORKING, com o banco concordando.
		// Este é o único caso que acumula.
		in.Healthy, in.Valid = false, true
	}

	_, event, err := d.repo.RecordProbe(ctx, in)
	if err != nil {
		return err
	}
	if event != nil {
		switch event.EventType {
		case models.FailoverEventProposalCreated:
			observability.RecordFailoverProposal("created")
			d.logger.Info("failover detector: proposal created",
				zap.Int64("policyID", policy.ID),
				zap.Int("primaryInstanceID", policy.PrimaryInstanceID))
		case models.FailoverEventProposalRecovered:
			observability.RecordFailoverProposal("recovered")
		}
	}
	return nil
}
