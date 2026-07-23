package failover

import (
	"context"
	"strconv"
	"time"

	"automatiza-go/internal/models"
	"automatiza-go/internal/observability"
	"automatiza-go/internal/repository"
	"automatiza-go/internal/repository/postgres"

	"github.com/getsentry/sentry-go"
	"go.uber.org/zap"
)

// leaseRetention: leases liberadas/expiradas há mais de 24h somem.
const leaseRetention = 24 * time.Hour

// stuckGrace: margem além do drain_deadline antes de considerar a operação
// travada. O plano pede alerta em deadline + 2min — tempo suficiente para a
// aplicação, a pós-verificação e a finalização, que somadas levam segundos.
const stuckGrace = 2 * time.Minute

// Recovery termina operações que ficaram não terminais (crash, deploy, resposta
// HTTP perdida).
//
// O princípio é único e não negociável: ele CLASSIFICA, nunca adivinha. Cada
// fase durável (preparing/draining sem applied_at; verifying com todos os itens
// em `after`; verifying com todos em `before`) tem um desfecho determinado. Um
// conjunto misto, um fingerprint que não é nem o de antes nem o de depois, ou
// uma referência incoerente entre política e operação viram `failed`, com o gate
// FECHADO e alerta — porque restaurar "no melhor esforço" com informação
// insuficiente é como se produz o estado que ninguém consegue depurar depois.
type Recovery struct {
	repo         repository.InstanceFailoverRepository
	instanceRepo repository.InstanceRepository
	service      *Service
	gate         *Gate
	logger       *zap.Logger
	enabled      func() bool
	now          func() time.Time
}

// NewRecovery cria o recuperador.
func NewRecovery(
	repo repository.InstanceFailoverRepository,
	instanceRepo repository.InstanceRepository,
	service *Service,
	gate *Gate,
	enabled func() bool,
	logger *zap.Logger,
) *Recovery {
	return &Recovery{
		repo:         repo,
		instanceRepo: instanceRepo,
		service:      service,
		gate:         gate,
		logger:       logger,
		enabled:      enabled,
		now:          time.Now,
	}
}

// Boot roda uma vez na subida do processo.
//
// Ele arma o gate quando encontra operação não terminal MESMO com a feature
// desligada. Sem isso, desligar a flag durante um rollback abriria o gate no
// meio de uma troca — exatamente o momento em que ele mais precisa estar
// fechado.
func (r *Recovery) Boot(ctx context.Context) error {
	pending, err := r.repo.ListNonTerminalOperations(ctx)
	if err != nil {
		return err
	}
	if len(pending) > 0 {
		r.setArmed(true)
		r.logger.Warn("failover: non-terminal operations found at boot; dispatch gate armed",
			zap.Int("count", len(pending)))
	}
	return r.Run(ctx)
}

// Run é o tick do cron (60s).
func (r *Recovery) Run(ctx context.Context) error {
	pending, err := r.repo.ListNonTerminalOperations(ctx)
	if err != nil {
		return err
	}

	// Com a flag desligada, o recovery só continua enquanto houver operação
	// pendente — depois disso fica inerte, como o resto da feature.
	if len(pending) == 0 {
		r.setArmed(false)
		if _, err := r.repo.CleanupLeases(ctx, r.now().UTC().Add(-leaseRetention)); err != nil {
			r.logger.Warn("failover: lease cleanup failed", zap.Error(err))
		}
		return nil
	}
	r.setArmed(true)

	for i := range pending {
		op := &pending[i]
		r.alertIfStuck(op)
		if err := r.recover(ctx, op); err != nil {
			r.logger.Error("failover recovery: operation could not be resolved",
				zap.String("operationID", op.ID), zap.Error(err))
		}
	}
	return nil
}

// alertIfStuck dispara alerta quando uma operação passa de
// drain_deadline + 2min sem chegar a um desfecho.
//
// É o sinal de que o gate está fechado há tempo demais: enquanto ele estiver,
// NENHUMA automação daquela conta envia. Um alerta aqui é a diferença entre
// "descobrimos em 2 minutos" e "o cliente ligou reclamando".
func (r *Recovery) alertIfStuck(op *models.InstanceFailoverOperation) {
	if op.DrainDeadlineAt == nil {
		return
	}
	if !r.now().UTC().After(op.DrainDeadlineAt.Add(stuckGrace)) {
		return
	}
	observability.LogAndCapture(r.logger, sentry.LevelError,
		"failover: operation still non-terminal well past its drain deadline (dispatch gate is closed)",
		nil, map[string]string{
			"module":       "failover",
			"flow":         "stuck_operation",
			"kind":         op.Kind,
			"status":       op.Status,
			"operation_id": op.ID,
			"policy_id":    strconv.FormatInt(op.PolicyIDSnapshot, 10),
		})
}

// setArmed liga/desliga os DOIS mecanismos de segurança que sobrevivem ao
// desligamento da feature flag: o gate de dispatch (impede novo envio) e o
// guard de mutação de automação (impede editar o conjunto congelado).
func (r *Recovery) setArmed(v bool) {
	r.gate.SetArmed(v)
	postgres.SetFailoverGuardArmed(v)
}

func (r *Recovery) recover(ctx context.Context, op *models.InstanceFailoverOperation) error {
	policy, err := r.repo.GetPolicy(ctx, op.PolicyIDSnapshot, op.UserID)
	if err != nil {
		return err
	}
	if policy == nil {
		// Política sumiu com operação viva: incoerência que não se resolve por
		// inferência. Fecha a operação como failed e deixa o rastro.
		_, err = r.repo.TerminateOperation(ctx, repository.FailoverTerminateInput{
			OperationID:     op.ID,
			PolicyID:        op.PolicyIDSnapshot,
			UserID:          op.UserID,
			OperationStatus: models.FailoverOpFailed,
			Event:           models.FailoverEventRecoveryFailed,
			ReasonCodes:     []string{models.ReasonIndeterminate},
			Report:          mustJSON(map[string]any{"reason_codes": []string{models.ReasonIndeterminate}}),
		})
		return err
	}

	// Fase 1 — nada foi aplicado ainda: abortar é sempre seguro, porque não há
	// alteração nenhuma a desfazer.
	if op.AppliedAt == nil {
		_, err := r.repo.TerminateOperation(ctx, repository.FailoverTerminateInput{
			OperationID:     op.ID,
			PolicyID:        policy.ID,
			UserID:          op.UserID,
			OperationStatus: models.FailoverOpAborted,
			PolicyState:     op.ReturnPolicyState,
			Event:           models.FailoverEventRecoveryAborted,
			ReasonCodes:     []string{models.ReasonIndeterminate},
			Report:          mustJSON(map[string]any{"phase": op.Status, "applied": false}),
		})
		return err
	}

	// Fase 2 — houve aplicação: classificar pelos fingerprints reais.
	items, err := r.repo.ListOperationItems(ctx, op.ID)
	if err != nil {
		return err
	}
	rows, err := r.repo.ListUserAutomationsRouting(ctx, op.UserID)
	if err != nil {
		return err
	}
	byID := make(map[string]*repository.FailoverAutomationRouting, len(rows))
	for i := range rows {
		byID[rows[i].Automation.ID] = &rows[i]
	}

	allAfter, allBefore := true, true
	for i := range items {
		it := &items[i]
		row, ok := byID[it.AutomationIDSnapshot]
		if !ok || row == nil {
			// Item apagado depois do gate: já é deleted_noop, não desempata.
			continue
		}
		snap, reasons := models.NewRoutingSnapshot(&row.Automation)
		if len(reasons) > 0 {
			allAfter, allBefore = false, false
			break
		}
		fp := snap.Fingerprint()
		if fp != it.FingerprintAfter {
			allAfter = false
		}
		if fp != it.FingerprintBefore {
			allBefore = false
		}
	}

	switch {
	case allAfter:
		observability.RecordFailoverRecovery("completed")
		// A aplicação chegou ao fim; falta só concluir. A verificação ao vivo
		// decide entre finalizar e restaurar.
		return r.finishApplied(ctx, policy, op, items)
	case allBefore:
		observability.RecordFailoverRecovery("aborted")
		// A restauração chegou ao fim (ou o apply nunca escreveu de fato).
		_, err := r.repo.TerminateOperation(ctx, repository.FailoverTerminateInput{
			OperationID:     op.ID,
			PolicyID:        policy.ID,
			UserID:          op.UserID,
			OperationStatus: models.FailoverOpAborted,
			PolicyState:     op.ReturnPolicyState,
			Event:           models.FailoverEventRecoveryAborted,
			Report:          mustJSON(map[string]any{"phase": op.Status, "restored": true}),
		})
		return err
	default:
		// Conjunto misto ou fingerprint de terceiro tipo. NÃO inferir.
		observability.RecordFailoverRecovery("failed")
		observability.LogAndCapture(r.logger, sentry.LevelError,
			"failover recovery: indeterminate state, policy marked failed and gate kept closed",
			nil, map[string]string{
				"module":       "failover",
				"flow":         "recovery_indeterminate",
				"operation_id": op.ID,
				"policy_id":    strconv.FormatInt(policy.ID, 10),
				"items":        strconv.Itoa(len(items)),
			})
		_, err := r.repo.TerminateOperation(ctx, repository.FailoverTerminateInput{
			OperationID:     op.ID,
			PolicyID:        policy.ID,
			UserID:          op.UserID,
			OperationStatus: models.FailoverOpFailed,
			PolicyState:     models.FailoverStateFailed,
			Event:           models.FailoverEventRecoveryFailed,
			ReasonCodes:     []string{models.ReasonIndeterminate},
			Report:          mustJSON(map[string]any{"reason_codes": []string{models.ReasonIndeterminate}}),
		})
		return err
	}
}

// finishApplied repete a verificação ao vivo de uma operação que já aplicou
// tudo, e conclui ou restaura conforme o resultado.
func (r *Recovery) finishApplied(
	ctx context.Context,
	policy *models.InstanceFailoverPolicy,
	op *models.InstanceFailoverOperation,
	items []models.InstanceFailoverOperationItem,
) error {
	targetInstanceID := policy.BackupInstanceID
	successState := models.FailoverStateActiveOnBackup
	operationState := models.FailoverStateSwitching
	if op.Kind == models.FailoverDirectionRevert {
		targetInstanceID = policy.PrimaryInstanceID
		successState = models.FailoverStateActive
		operationState = models.FailoverStateReverting
	}

	set := &EligibleSet{}
	for i := range items {
		it := &items[i]
		after, err := models.UnmarshalRoutingSnapshot(it.SnapshotAfter)
		if err != nil {
			return err
		}
		before, err := models.UnmarshalRoutingSnapshot(it.SnapshotBefore)
		if err != nil {
			return err
		}
		set.Items = append(set.Items, EligibleItem{
			AutomationID: it.AutomationIDSnapshot,
			Before:       before,
			After:        after,
		})
	}

	session, err := r.service.sessionNameFor(ctx, op.UserID, targetInstanceID)
	if err != nil {
		return err
	}

	if reasons := r.service.verifyCoverage(ctx, session, set); len(reasons) > 0 {
		// Restaurar exige que TUDO esteja em `after` — que é justamente a
		// condição que nos trouxe até aqui.
		applyErr := r.repo.ApplyProjection(ctx, repository.FailoverApplyInput{
			OperationID:            op.ID,
			PolicyID:               policy.ID,
			UserID:                 op.UserID,
			ExpectedState:          operationState,
			Items:                  items,
			ExpectFingerprintAfter: true,
			ItemResult:             models.FailoverItemRestored,
			Event:                  failedEventName(op.Kind),
		})
		if applyErr != nil {
			_, termErr := r.repo.TerminateOperation(ctx, repository.FailoverTerminateInput{
				OperationID:     op.ID,
				PolicyID:        policy.ID,
				UserID:          op.UserID,
				OperationStatus: models.FailoverOpFailed,
				PolicyState:     models.FailoverStateFailed,
				Event:           models.FailoverEventRecoveryFailed,
				ReasonCodes:     append(reasons, models.ReasonIndeterminate),
				Report:          mustJSON(map[string]any{"reason_codes": append(reasons, models.ReasonIndeterminate)}),
			})
			return termErr
		}
		_, err := r.repo.TerminateOperation(ctx, repository.FailoverTerminateInput{
			OperationID:     op.ID,
			PolicyID:        policy.ID,
			UserID:          op.UserID,
			OperationStatus: models.FailoverOpAborted,
			PolicyState:     op.ReturnPolicyState,
			Event:           models.FailoverEventRecoveryAborted,
			ReasonCodes:     reasons,
			Report:          mustJSON(map[string]any{"reason_codes": reasons}),
		})
		return err
	}

	_, err = r.repo.TerminateOperation(ctx, repository.FailoverTerminateInput{
		OperationID:     op.ID,
		PolicyID:        policy.ID,
		UserID:          op.UserID,
		OperationStatus: models.FailoverOpCompleted,
		PolicyState:     successState,
		Event:           models.FailoverEventRecoveryCompleted,
		Report:          mustJSON(map[string]any{"items": len(items), "target_instance": targetInstanceID}),
	})
	return err
}
