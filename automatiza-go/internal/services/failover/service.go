package failover

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"time"

	"automatiza-go/internal/models"
	"automatiza-go/internal/observability"
	"automatiza-go/internal/repository"

	"github.com/getsentry/sentry-go"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

const (
	// dryRunTTL é exato: 15 minutos. Um ensaio mais velho descreve uma
	// fotografia do WAHA que já não se pode assumir válida.
	dryRunTTL = 15 * time.Minute
	// drainPoll é o intervalo entre checagens de trabalho em voo.
	drainPoll = 500 * time.Millisecond
)

// Service implementa as regras da Melhoria 5.
type Service struct {
	repo         repository.InstanceFailoverRepository
	instanceRepo repository.InstanceRepository
	coverage     *CoverageChecker
	logger       *zap.Logger
	enabled      func() bool

	// now e sleep são injetáveis para o teste conseguir controlar drain e TTL
	// sem relógio de parede.
	now   func() time.Time
	sleep func(ctx context.Context, d time.Duration) bool
}

// NewService cria o serviço.
func NewService(
	repo repository.InstanceFailoverRepository,
	instanceRepo repository.InstanceRepository,
	provider ProviderReader,
	enabled func() bool,
	logger *zap.Logger,
) *Service {
	return &Service{
		repo:         repo,
		instanceRepo: instanceRepo,
		coverage:     NewCoverageChecker(provider),
		logger:       logger,
		enabled:      enabled,
		now:          time.Now,
		sleep:        sleepCtx,
	}
}

func sleepCtx(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// ensureEnabled é a primeira linha de TODA RPC, inclusive as de leitura.
func (s *Service) ensureEnabled() error {
	if s.enabled == nil || !s.enabled() {
		return ErrDisabled
	}
	return nil
}

// =============================================================================
// Política
// =============================================================================

// SetPolicyInput é a entrada de criação/atualização.
type SetPolicyInput struct {
	UserID                    string
	PolicyID                  int64
	PrimaryInstanceID         int
	BackupInstanceID          int
	Mode                      string
	ExpectedVersion           int64
	IdempotencyKey            string
	UnhealthyAfterSeconds     *int
	RequiredConsecutiveProbes *int
	DrainDeadlineSeconds      *int
	ProposalTTLSeconds        *int
}

// SetPolicy cria ou atualiza a política. A primária é IMUTÁVEL: trocar de
// primária exige DisablePolicy e uma política nova, porque a política antiga
// carrega o histórico de qual chip servia o quê.
func (s *Service) SetPolicy(ctx context.Context, in SetPolicyInput) (*repository.FailoverPolicyMutation, error) {
	if err := s.ensureEnabled(); err != nil {
		return nil, err
	}
	if !models.ValidIdempotencyKey(in.IdempotencyKey) {
		return nil, fmt.Errorf("%w: idempotency_key must be 16..128 printable ASCII bytes", ErrInvalidArgument)
	}
	switch in.Mode {
	case models.FailoverModeManual, models.FailoverModeAssisted:
	default:
		return nil, fmt.Errorf("%w: unknown mode", ErrInvalidArgument)
	}
	if in.PrimaryInstanceID <= 0 || in.BackupInstanceID <= 0 {
		return nil, fmt.Errorf("%w: instance ids must be positive", ErrInvalidArgument)
	}
	if in.PrimaryInstanceID == in.BackupInstanceID {
		return nil, fmt.Errorf("%w: primary and backup must be different chips", ErrInvalidArgument)
	}
	if err := s.assertOwnership(ctx, in.UserID, in.PrimaryInstanceID, in.BackupInstanceID); err != nil {
		return nil, err
	}

	if in.PolicyID != 0 {
		current, err := s.repo.GetPolicy(ctx, in.PolicyID, in.UserID)
		if err != nil {
			return nil, err
		}
		if current == nil {
			return nil, ErrNotFound
		}
		if current.PrimaryInstanceID != in.PrimaryInstanceID {
			return nil, fmt.Errorf("%w: primary chip is immutable; disable this policy and create a new one", ErrInvalidArgument)
		}
		if !current.IsStable() || current.State == models.FailoverStateActiveOnBackup {
			return nil, fmt.Errorf("%w: policy can only be edited while active or proposed", ErrStateConflict)
		}
	} else if in.ExpectedVersion != 0 {
		return nil, fmt.Errorf("%w: create requires expected_version=0", ErrInvalidArgument)
	}

	hash := models.HashRequest("set_policy",
		strconv.FormatInt(in.PolicyID, 10),
		strconv.Itoa(in.PrimaryInstanceID),
		strconv.Itoa(in.BackupInstanceID),
		in.Mode,
		strconv.FormatInt(in.ExpectedVersion, 10),
		optIntStr(in.UnhealthyAfterSeconds),
		optIntStr(in.RequiredConsecutiveProbes),
		optIntStr(in.DrainDeadlineSeconds),
		optIntStr(in.ProposalTTLSeconds),
	)

	out, err := s.repo.SetPolicy(ctx, repository.SetFailoverPolicyInput{
		PolicyID:                  in.PolicyID,
		UserID:                    in.UserID,
		ActorUserID:               in.UserID,
		PrimaryInstanceID:         in.PrimaryInstanceID,
		BackupInstanceID:          in.BackupInstanceID,
		Mode:                      in.Mode,
		ExpectedVersion:           in.ExpectedVersion,
		IdempotencyKey:            in.IdempotencyKey,
		RequestHash:               hash,
		UnhealthyAfterSeconds:     in.UnhealthyAfterSeconds,
		RequiredConsecutiveProbes: in.RequiredConsecutiveProbes,
		DrainDeadlineSeconds:      in.DrainDeadlineSeconds,
		ProposalTTLSeconds:        in.ProposalTTLSeconds,
	})
	return out, translateRepoError(err)
}

// GetPolicy devolve a política e o contexto de leitura da UI.
func (s *Service) GetPolicy(ctx context.Context, userID string, policyID int64) (*models.InstanceFailoverPolicy, *models.InstanceFailoverEvent, *models.InstanceFailoverOperation, error) {
	if err := s.ensureEnabled(); err != nil {
		return nil, nil, nil, err
	}
	policy, err := s.repo.GetPolicy(ctx, policyID, userID)
	if err != nil {
		return nil, nil, nil, err
	}
	if policy == nil {
		return nil, nil, nil, ErrNotFound
	}
	last, err := s.repo.GetLastEvent(ctx, policyID, userID)
	if err != nil {
		return nil, nil, nil, err
	}
	var op *models.InstanceFailoverOperation
	if policy.CurrentOperationID != nil {
		op, err = s.repo.GetOperation(ctx, *policy.CurrentOperationID, userID)
		if err != nil {
			return nil, nil, nil, err
		}
	}
	return policy, last, op, nil
}

// ListPolicies lista as políticas do usuário.
func (s *Service) ListPolicies(ctx context.Context, userID string) ([]models.InstanceFailoverPolicy, error) {
	if err := s.ensureEnabled(); err != nil {
		return nil, err
	}
	return s.repo.ListPolicies(ctx, userID)
}

// DisablePolicy remove a política. Proibido em active_on_backup: apagar ali
// jogaria fora o único registro de qual era o roteamento original.
func (s *Service) DisablePolicy(ctx context.Context, userID string, policyID, expectedVersion int64, key string) (*repository.FailoverPolicyMutation, error) {
	if err := s.ensureEnabled(); err != nil {
		return nil, err
	}
	if !models.ValidIdempotencyKey(key) {
		return nil, fmt.Errorf("%w: idempotency_key must be 16..128 printable ASCII bytes", ErrInvalidArgument)
	}

	current, err := s.repo.GetPolicy(ctx, policyID, userID)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, ErrNotFound
	}
	if current.State != models.FailoverStateActive && current.State != models.FailoverStateProposed {
		return nil, fmt.Errorf("%w: revert to the primary chip before disabling this policy", ErrStateConflict)
	}

	hash := models.HashRequest("disable_policy",
		strconv.FormatInt(policyID, 10), strconv.FormatInt(expectedVersion, 10))
	out, err := s.repo.DisablePolicy(ctx, repository.DisableFailoverPolicyInput{
		PolicyID:        policyID,
		UserID:          userID,
		ActorUserID:     userID,
		ExpectedVersion: expectedVersion,
		IdempotencyKey:  key,
		RequestHash:     hash,
	})
	return out, translateRepoError(err)
}

// ListEvents pagina o ledger.
func (s *Service) ListEvents(ctx context.Context, userID string, policyID int64, cursor string, limit int) ([]models.InstanceFailoverEvent, string, error) {
	if err := s.ensureEnabled(); err != nil {
		return nil, "", err
	}
	return s.repo.ListEvents(ctx, repository.ListFailoverEventsInput{
		PolicyID: policyID, UserID: userID, Cursor: cursor, Limit: limit,
	})
}

// =============================================================================
// Dry-run
// =============================================================================

// DryRunResult é o relatório completo devolvido ao cliente.
type DryRunResult struct {
	DryRun *models.InstanceFailoverDryRun
	Items  []models.InstanceFailoverDryRunItem
}

// RunDryRun ensaia a troca. PUREZA OPERACIONAL: não altera policy state nem
// version, não pega lease, não escreve automação/instância/grupo/fila/dispatch
// e no WAHA só faz GET. As únicas escritas são o recibo append-only e o evento
// de auditoria, numa transação curta depois que todas as leituras terminaram.
func (s *Service) RunDryRun(ctx context.Context, userID string, policyID int64, direction, key string) (*DryRunResult, error) {
	if err := s.ensureEnabled(); err != nil {
		return nil, err
	}
	if !models.ValidIdempotencyKey(key) {
		return nil, fmt.Errorf("%w: idempotency_key must be 16..128 printable ASCII bytes", ErrInvalidArgument)
	}
	if direction != models.FailoverDirectionFailover && direction != models.FailoverDirectionRevert {
		return nil, fmt.Errorf("%w: unknown direction", ErrInvalidArgument)
	}

	policy, err := s.repo.GetPolicy(ctx, policyID, userID)
	if err != nil {
		return nil, err
	}
	if policy == nil {
		return nil, ErrNotFound
	}

	switch direction {
	case models.FailoverDirectionFailover:
		if policy.State != models.FailoverStateActive && policy.State != models.FailoverStateProposed {
			return nil, fmt.Errorf("%w: failover dry run requires an active or proposed policy", ErrStateConflict)
		}
	case models.FailoverDirectionRevert:
		if policy.State != models.FailoverStateActiveOnBackup {
			return nil, fmt.Errorf("%w: revert dry run requires the policy to be running on the backup chip", ErrStateConflict)
		}
	}

	hash := models.HashRequest("dry_run", strconv.FormatInt(policyID, 10), direction)

	// Replay com a MESMA chave e o MESMO request devolve o recibo gravado e faz
	// zero chamadas ao WAHA.
	if existing, err := s.repo.FindDryRunByKey(ctx, userID, direction, key); err != nil {
		return nil, err
	} else if existing != nil {
		if existing.RequestHash != hash {
			return nil, repository.ErrFailoverIdempotencyMismatch
		}
		items, err := s.repo.ListDryRunItems(ctx, existing.ID)
		if err != nil {
			return nil, err
		}
		return &DryRunResult{DryRun: existing, Items: items}, nil
	}

	set, targetInstanceID, sourceOp, err := s.buildSet(ctx, policy, direction)
	if err != nil {
		return nil, err
	}
	_ = sourceOp

	session, err := s.sessionNameFor(ctx, userID, targetInstanceID)
	if err != nil {
		return nil, err
	}

	cov, items, blocking := s.evaluateSet(ctx, session, set)
	status := models.FailoverDryRunPass
	if len(blocking) > 0 {
		status = models.FailoverDryRunBlocked
	}

	refItems := make([]models.DryRunRefItem, 0, len(set.Items))
	for i := range set.Items {
		it := &set.Items[i]
		refItems = append(refItems, models.DryRunRefItem{
			AutomationID: it.AutomationID,
			Before:       it.Before.Fingerprint(),
			After:        it.After.Fingerprint(),
		})
	}
	ref := models.DryRunRef(policy.ID, policy.Version, direction, refItems, CoverageHash(cov))

	now := s.now().UTC()
	report, _ := json.Marshal(map[string]any{
		"direction":       direction,
		"target_instance": targetInstanceID,
		"items":           len(set.Items),
		"blocking":        blocking,
	})

	dr := &models.InstanceFailoverDryRun{
		PolicyID:         &policy.ID,
		PolicyIDSnapshot: policy.ID,
		UserID:           userID,
		Direction:        direction,
		IdempotencyKey:   key,
		RequestHash:      hash,
		PolicyVersion:    policy.Version,
		DryRunRef:        ref,
		Status:           status,
		Report:           report,
		ExpiresAt:        now.Add(dryRunTTL),
	}

	evType := models.FailoverEventDryRunPassed
	if status == models.FailoverDryRunBlocked {
		evType = models.FailoverEventDryRunBlocked
	}
	ev := &models.InstanceFailoverEvent{
		PolicyID:         &policy.ID,
		PolicyIDSnapshot: policy.ID,
		UserID:           userID,
		EventType:        evType,
		ActorType:        models.FailoverActorUser,
		ActorUserID:      &userID,
		PolicyVersion:    &policy.Version,
		Report:           report,
	}

	if err := s.repo.SaveDryRun(ctx, dr, items, ev); err != nil {
		return nil, translateRepoError(err)
	}
	for i := range items {
		items[i].DryRunID = dr.ID
	}
	return &DryRunResult{DryRun: dr, Items: items}, nil
}

// buildSet resolve o conjunto elegível conforme a direção.
func (s *Service) buildSet(ctx context.Context, policy *models.InstanceFailoverPolicy, direction string) (*EligibleSet, int, *models.InstanceFailoverOperation, error) {
	rows, err := s.repo.ListUserAutomationsRouting(ctx, policy.UserID)
	if err != nil {
		return nil, 0, nil, err
	}

	if direction == models.FailoverDirectionFailover {
		set := BuildSwapSet(rows, policy.PrimaryInstanceID, policy.BackupInstanceID)
		return set, policy.BackupInstanceID, nil, nil
	}

	// Reversão: o alvo é o snapshot original da última operação de failover
	// concluída, não uma troca textual inversa.
	sourceOp, err := s.repo.GetLastCompletedOperation(ctx, policy.ID, policy.UserID, models.FailoverDirectionFailover)
	if err != nil {
		return nil, 0, nil, err
	}
	if sourceOp == nil {
		return nil, 0, nil, fmt.Errorf("%w: no completed failover to revert", ErrStateConflict)
	}
	opItems, err := s.repo.ListOperationItems(ctx, sourceOp.ID)
	if err != nil {
		return nil, 0, nil, err
	}
	byID := make(map[string]*repository.FailoverAutomationRouting, len(rows))
	for i := range rows {
		byID[rows[i].Automation.ID] = &rows[i]
	}
	set, err := BuildRevertSet(opItems, byID)
	if err != nil {
		return nil, 0, nil, err
	}
	return set, policy.PrimaryInstanceID, sourceOp, nil
}

// evaluateSet faz a coleta ao vivo e monta os itens do relatório.
func (s *Service) evaluateSet(ctx context.Context, session string, set *EligibleSet) (map[string]ItemCoverage, []models.InstanceFailoverDryRunItem, []string) {
	allGroups := map[string]struct{}{}
	targetGroups := map[string]struct{}{}
	for i := range set.Items {
		it := &set.Items[i]
		for _, g := range it.After.TargetGroups() {
			allGroups[g] = struct{}{}
			targetGroups[g] = struct{}{}
		}
		for _, g := range it.After.SourceGroups() {
			allGroups[g] = struct{}{}
		}
	}

	cov := s.coverage.Inspect(ctx, session, keysSorted(allGroups), keysSorted(targetGroups))

	blocking := append([]string(nil), set.BlockingReasons...)
	blocking = append(blocking, cov.Reasons...)

	byID := make(map[string]ItemCoverage, len(set.Items))
	items := make([]models.InstanceFailoverDryRunItem, 0, len(set.Items))

	for i := range set.Items {
		it := &set.Items[i]
		ic := EvaluateItem(it, cov)
		byID[it.AutomationID] = ic
		if ic.Result != models.FailoverCoverageCovered {
			blocking = append(blocking, ic.Reasons...)
		}

		before, _ := it.Before.MarshalSnapshot()
		after, _ := it.After.MarshalSnapshot()
		id := it.AutomationID
		items = append(items, models.InstanceFailoverDryRunItem{
			AutomationID:         &id,
			AutomationIDSnapshot: id,
			AutomationName:       it.Name,
			ConfigFingerprint:    it.Before.Fingerprint(),
			SnapshotBefore:       before,
			SnapshotAfter:        after,
			RequiredSourceGroups: ic.RequiredSources,
			RequiredTargetGroups: ic.RequiredTargets,
			AdminRequiredGroups:  ic.AdminRequired,
			MissingGroups:        ic.MissingGroups,
			MissingAdminGroups:   ic.MissingAdmin,
			Result:               ic.Result,
			ReasonCodes:          ic.Reasons,
		})
	}

	return byID, items, dedupeSorted(blocking)
}

// =============================================================================
// Confirmação (failover e reversão compartilham o algoritmo)
// =============================================================================

// ConfirmInput é a entrada de ConfirmFailover/RevertFailover.
type ConfirmInput struct {
	UserID          string
	PolicyID        int64
	ExpectedVersion int64
	DryRunID        string
	DryRunRef       string
	IdempotencyKey  string
}

// ConfirmResult é o desfecho de uma operação.
type ConfirmResult struct {
	Operation *models.InstanceFailoverOperation
	Policy    *models.InstanceFailoverPolicy
	Reasons   []string
}

// ConfirmFailover troca o roteamento da primária para a reserva.
func (s *Service) ConfirmFailover(ctx context.Context, in ConfirmInput) (*ConfirmResult, error) {
	return s.confirm(ctx, in, models.FailoverDirectionFailover)
}

// RevertFailover devolve o roteamento à primária a partir do snapshot original.
func (s *Service) RevertFailover(ctx context.Context, in ConfirmInput) (*ConfirmResult, error) {
	return s.confirm(ctx, in, models.FailoverDirectionRevert)
}

func (s *Service) confirm(ctx context.Context, in ConfirmInput, kind string) (*ConfirmResult, error) {
	if err := s.ensureEnabled(); err != nil {
		return nil, err
	}
	if !models.ValidIdempotencyKey(in.IdempotencyKey) {
		return nil, fmt.Errorf("%w: idempotency_key must be 16..128 printable ASCII bytes", ErrInvalidArgument)
	}

	policy, err := s.repo.GetPolicy(ctx, in.PolicyID, in.UserID)
	if err != nil {
		return nil, err
	}
	if policy == nil {
		return nil, ErrNotFound
	}

	var requiredState, operationState, successState string
	switch kind {
	case models.FailoverDirectionFailover:
		requiredState = models.FailoverStateActive
		if policy.State == models.FailoverStateProposed {
			requiredState = models.FailoverStateProposed
		}
		operationState = models.FailoverStateSwitching
		successState = models.FailoverStateActiveOnBackup
	default:
		requiredState = models.FailoverStateActiveOnBackup
		operationState = models.FailoverStateReverting
		successState = models.FailoverStateActive
	}
	if policy.State != requiredState {
		return nil, fmt.Errorf("%w: policy is %s", ErrStateConflict, policy.State)
	}
	if policy.Version != in.ExpectedVersion {
		return nil, repository.ErrFailoverVersionConflict
	}

	// O ensaio precisa existir, ter passado, ser desta direção, não estar
	// vencido e bater ref + versão. Um ensaio bloqueado NUNCA é confirmável.
	dryRun, err := s.repo.GetDryRun(ctx, in.DryRunID, in.UserID)
	if err != nil {
		return nil, err
	}
	if dryRun == nil {
		return nil, ErrNotFound
	}
	if dryRun.Direction != kind || dryRun.Status != models.FailoverDryRunPass {
		return nil, fmt.Errorf("%w: dry run is not a passing %s run", ErrDryRunMismatch, kind)
	}
	if dryRun.DryRunRef != in.DryRunRef || dryRun.PolicyVersion != policy.Version {
		return nil, ErrDryRunMismatch
	}
	if s.now().UTC().After(dryRun.ExpiresAt) {
		return nil, ErrDryRunExpired
	}

	hash := models.HashRequest("confirm", kind,
		strconv.FormatInt(in.PolicyID, 10),
		strconv.FormatInt(in.ExpectedVersion, 10),
		in.DryRunID, in.DryRunRef)

	var sourceOpID *string
	if kind == models.FailoverDirectionRevert {
		sourceOp, err := s.repo.GetLastCompletedOperation(ctx, policy.ID, in.UserID, models.FailoverDirectionFailover)
		if err != nil {
			return nil, err
		}
		if sourceOp == nil {
			return nil, fmt.Errorf("%w: no completed failover to revert", ErrStateConflict)
		}
		sourceOpID = &sourceOp.ID
	}

	op, replay, err := s.repo.InsertOperation(ctx, &models.InstanceFailoverOperation{
		PolicyID:           &policy.ID,
		PolicyIDSnapshot:   policy.ID,
		UserID:             in.UserID,
		Kind:               kind,
		ReturnPolicyState:  policy.State,
		IdempotencyKey:     in.IdempotencyKey,
		RequestHash:        hash,
		DryRunID:           dryRun.ID,
		SourceOperationID:  sourceOpID,
		StartPolicyVersion: policy.Version,
	})
	if err != nil {
		return nil, translateRepoError(err)
	}
	if replay {
		// Replay: terminal devolve o report gravado; não terminal devolve o
		// estado atual sem iniciar uma segunda execução.
		fresh, ferr := s.repo.GetPolicy(ctx, in.PolicyID, in.UserID)
		if ferr != nil {
			return nil, ferr
		}
		return &ConfirmResult{Operation: op, Policy: fresh}, nil
	}

	return s.execute(ctx, policy, op, operationState, successState, kind)
}

// execute roda o algoritmo completo: gate → drain → revalidação → apply →
// pós-verificação → finalização (ou restauração).
func (s *Service) execute(
	ctx context.Context,
	policy *models.InstanceFailoverPolicy,
	op *models.InstanceFailoverOperation,
	operationState, successState, kind string,
) (*ConfirmResult, error) {
	startedAt := s.now()
	defer func() {
		observability.RecordFailoverOperationDuration(kind, s.now().Sub(startedAt).Seconds())
	}()

	// Recalcula o conjunto SOB a transação de gate: entre o ensaio e agora, uma
	// automação pode ter nascido, morrido ou mudado de forma.
	set, targetInstanceID, _, err := s.buildSet(ctx, policy, kind)
	if err != nil {
		return nil, err
	}
	if set.Blocked() {
		// Bloqueio detectado ANTES do gate: nada foi tocado, a operação morre
		// aqui e a política volta ao estado de retorno.
		return s.abort(ctx, policy, op, set.BlockingReasons, kind, ErrCoverageBlocked)
	}

	items, err := set.ToOperationItems(op.ID)
	if err != nil {
		return nil, err
	}

	requestedEv, gateEv, appliedEv, completedEv, abortedEv, timeoutEv := eventNames(kind)

	deadline := s.now().UTC().Add(time.Duration(policy.DrainDeadlineSeconds) * time.Second)
	closed, err := s.repo.CloseGate(ctx, repository.FailoverCloseGateInput{
		OperationID:     op.ID,
		PolicyID:        policy.ID,
		UserID:          policy.UserID,
		ActorUserID:     policy.UserID,
		ExpectedVersion: policy.Version,
		ExpectedState:   policy.State,
		NewState:        operationState,
		Items:           items,
		DrainDeadlineAt: deadline,
		RequestedEvent:  requestedEv,
		GateClosedEvent: gateEv,
	})
	if err != nil {
		return nil, translateRepoError(err)
	}
	op = closed

	// --- drain -------------------------------------------------------------
	// Depois daqui o gate está FECHADO: nenhum produtor consegue lease para
	// estas automações. Só falta o trabalho que já estava em voo terminar.
	gateClosedAt := s.now().UTC()
	if op.GateClosedAt != nil {
		gateClosedAt = *op.GateClosedAt
	}
	ids := set.AutomationIDs()

	drainStartedAt := s.now()
	drainErr := s.waitForDrain(ctx, ids, gateClosedAt, deadline)
	observability.RecordFailoverDrainDuration(kind, s.now().Sub(drainStartedAt).Seconds())
	if drainErr != nil {
		// Deadline: ABORTA. Nunca prossegue — um job Asynq com InstanceName
		// congelado enviaria pelo chip antigo depois da troca, e isso é
		// split-brain com mensagem duplicada no grupo do cliente.
		return s.abort(ctx, policy, op, []string{models.ReasonDrainTimeout}, kind, ErrDrainTimeout,
			withTimeoutEvent(timeoutEv))
	}

	// --- revalidação do chip alvo -----------------------------------------
	//
	// A partir daqui o gate está fechado, então TODO caminho de saída antes da
	// aplicação precisa abortar explicitamente. Só devolver o erro deixaria a
	// política presa em switching/reverting até o recovery passar (até 60s) —
	// tempo em que nenhuma automação da conta envia nada.
	session, err := s.sessionNameFor(ctx, policy.UserID, targetInstanceID)
	if err != nil {
		return s.abort(ctx, policy, op, []string{models.ReasonTargetNotWorking}, kind, err)
	}
	if reasons := s.verifyCoverage(ctx, session, set); len(reasons) > 0 {
		return s.abort(ctx, policy, op, reasons, kind, ErrCoverageBlocked)
	}

	// --- apply -------------------------------------------------------------
	if err := s.repo.ApplyProjection(ctx, repository.FailoverApplyInput{
		OperationID:   op.ID,
		PolicyID:      policy.ID,
		UserID:        policy.UserID,
		ActorUserID:   policy.UserID,
		ExpectedState: operationState,
		Items:         items,
		ItemResult:    models.FailoverItemApplied,
		Event:         appliedEv,
	}); err != nil {
		if errors.Is(err, repository.ErrFailoverStaleConfiguration) {
			return s.abort(ctx, policy, op, []string{models.ReasonStaleConfiguration}, kind, err)
		}
		// ApplyProjection é all-or-nothing: se falhou, NENHUMA automação foi
		// alterada, então abortar é seguro. (Se tivesse aplicado e falhado
		// depois, a transação teria feito rollback.)
		return s.abort(ctx, policy, op, []string{models.ReasonIndeterminate}, kind, translateRepoError(err))
	}

	// --- pós-verificação ---------------------------------------------------
	if reasons := s.verifyCoverage(ctx, session, set); len(reasons) > 0 {
		return s.restore(ctx, policy, op, items, reasons, kind, abortedEv)
	}

	finalPolicy, err := s.repo.TerminateOperation(ctx, repository.FailoverTerminateInput{
		OperationID:     op.ID,
		PolicyID:        policy.ID,
		UserID:          policy.UserID,
		ActorUserID:     policy.UserID,
		OperationStatus: models.FailoverOpCompleted,
		PolicyState:     successState,
		Event:           completedEv,
		Report:          mustJSON(map[string]any{"items": len(items), "target_instance": targetInstanceID}),
	})
	if err != nil {
		return nil, err
	}

	observability.RecordFailoverOperation(kind, models.FailoverOpCompleted)

	final, err := s.repo.GetOperation(ctx, op.ID, policy.UserID)
	if err != nil {
		return nil, err
	}
	return &ConfirmResult{Operation: final, Policy: finalPolicy}, nil
}

// waitForDrain espera as leases ativas e depois os dispatches pending/queued
// criados até o fechamento do gate.
func (s *Service) waitForDrain(ctx context.Context, automationIDs []string, gateClosedAt, deadline time.Time) error {
	for {
		leases, err := s.repo.CountActiveLeases(ctx, automationIDs, s.now().UTC())
		if err != nil {
			return err
		}
		pending := 0
		if leases == 0 {
			// Só depois das leases: um produtor com lease viva ainda pode
			// CRIAR dispatch, então contar antes daria um zero enganoso.
			pending, err = s.repo.CountNonTerminalDispatches(ctx, automationIDs, gateClosedAt)
			if err != nil {
				return err
			}
			if pending == 0 {
				return nil
			}
		}
		if !s.now().UTC().Before(deadline) {
			return fmt.Errorf("%w: %d leases, %d in-flight dispatches", ErrDrainTimeout, leases, pending)
		}
		if !s.sleep(ctx, drainPoll) {
			return ctx.Err()
		}
	}
}

// verifyCoverage repete a coleta ao vivo e devolve os reason codes que impedem
// seguir. Lista vazia = tudo certo.
func (s *Service) verifyCoverage(ctx context.Context, session string, set *EligibleSet) []string {
	cov, _, blocking := s.evaluateSet(ctx, session, set)
	_ = cov
	return blocking
}

type abortOption func(*abortConfig)

type abortConfig struct{ timeoutEvent string }

func withTimeoutEvent(name string) abortOption {
	return func(c *abortConfig) { c.timeoutEvent = name }
}

// abort desfaz a operação SEM ter tocado em automação nenhuma.
func (s *Service) abort(
	ctx context.Context,
	policy *models.InstanceFailoverPolicy,
	op *models.InstanceFailoverOperation,
	reasons []string,
	kind string,
	cause error,
	opts ...abortOption,
) (*ConfirmResult, error) {
	cfg := abortConfig{}
	for _, o := range opts {
		o(&cfg)
	}
	observability.RecordFailoverOperation(kind, models.FailoverOpAborted)
	for _, r := range reasons {
		observability.RecordFailoverCoverageBlocked(r)
	}
	_, _, _, _, abortedEv, _ := eventNames(kind)

	// Estado de retorno: a proposta só é preservada se ainda não expirou.
	returnState := op.ReturnPolicyState
	if returnState == models.FailoverStateProposed {
		fresh, err := s.repo.GetPolicy(ctx, policy.ID, policy.UserID)
		if err == nil && fresh != nil {
			if fresh.ProposalExpiresAt == nil || s.now().UTC().After(*fresh.ProposalExpiresAt) {
				returnState = models.FailoverStateActive
			}
		}
	}

	if cfg.timeoutEvent != "" {
		if _, err := s.repo.InsertEvent(ctx, &models.InstanceFailoverEvent{
			PolicyID:         &policy.ID,
			PolicyIDSnapshot: policy.ID,
			OperationID:      &op.ID,
			UserID:           policy.UserID,
			EventType:        cfg.timeoutEvent,
			ActorType:        models.FailoverActorSystem,
			Report:           mustJSON(map[string]any{"reason_codes": reasons}),
		}); err != nil {
			s.logger.Warn("failover: could not write drain timeout event", zap.Error(err))
		}
	}

	finalPolicy, err := s.repo.TerminateOperation(ctx, repository.FailoverTerminateInput{
		OperationID:     op.ID,
		PolicyID:        policy.ID,
		UserID:          policy.UserID,
		ActorUserID:     policy.UserID,
		OperationStatus: models.FailoverOpAborted,
		PolicyState:     returnState,
		Event:           abortedEv,
		ReasonCodes:     reasons,
		Report:          mustJSON(map[string]any{"reason_codes": reasons}),
	})
	if err != nil {
		return nil, err
	}

	final, ferr := s.repo.GetOperation(ctx, op.ID, policy.UserID)
	if ferr != nil {
		return nil, ferr
	}
	return &ConfirmResult{Operation: final, Policy: finalPolicy, Reasons: reasons}, cause
}

// restore desfaz uma aplicação que não passou na pós-verificação.
//
// Exige que CADA automação esteja exatamente em fingerprint_after. Se alguma
// não casa nem com after nem com before, o estado é indeterminado: política e
// operação viram `failed`, o gate CONTINUA fechado e um humano decide. Nunca
// "melhor esforço".
func (s *Service) restore(
	ctx context.Context,
	policy *models.InstanceFailoverPolicy,
	op *models.InstanceFailoverOperation,
	items []models.InstanceFailoverOperationItem,
	reasons []string,
	kind string,
	abortedEv string,
) (*ConfirmResult, error) {
	operationState := models.FailoverStateSwitching
	if kind == models.FailoverDirectionRevert {
		operationState = models.FailoverStateReverting
	}

	err := s.repo.ApplyProjection(ctx, repository.FailoverApplyInput{
		OperationID:            op.ID,
		PolicyID:               policy.ID,
		UserID:                 policy.UserID,
		ActorUserID:            policy.UserID,
		ExpectedState:          operationState,
		Items:                  items,
		ExpectFingerprintAfter: true,
		ItemResult:             models.FailoverItemRestored,
		Event:                  abortedEv,
	})
	if err != nil {
		// Estado indeterminado: alguém precisa olhar. O Sentry leva só IDs,
		// contagens e reason codes — nunca snapshot, grupo ou telefone.
		observability.RecordFailoverOperation(kind, models.FailoverOpFailed)
		observability.LogAndCapture(s.logger, sentry.LevelError,
			"failover: restore failed, policy marked failed and gate kept closed",
			err, map[string]string{
				"module":       "failover",
				"flow":         "restore",
				"kind":         kind,
				"policy_id":    strconv.FormatInt(policy.ID, 10),
				"operation_id": op.ID,
				"items":        strconv.Itoa(len(items)),
			})

		finalPolicy, termErr := s.repo.TerminateOperation(ctx, repository.FailoverTerminateInput{
			OperationID:     op.ID,
			PolicyID:        policy.ID,
			UserID:          policy.UserID,
			ActorUserID:     policy.UserID,
			OperationStatus: models.FailoverOpFailed,
			PolicyState:     models.FailoverStateFailed,
			Event:           failedEventName(kind),
			ReasonCodes:     append(reasons, models.ReasonIndeterminate),
			Report:          mustJSON(map[string]any{"reason_codes": append(reasons, models.ReasonIndeterminate)}),
		})
		if termErr != nil {
			return nil, termErr
		}
		final, _ := s.repo.GetOperation(ctx, op.ID, policy.UserID)
		return &ConfirmResult{Operation: final, Policy: finalPolicy, Reasons: reasons}, ErrIndeterminate
	}

	return s.abort(ctx, policy, op, append(reasons, models.ReasonPostcheckFailed), kind, ErrCoverageBlocked)
}

// =============================================================================
// Helpers
// =============================================================================

// assertOwnership prova, ANTES do banco reafirmar pela FK composta, que os dois
// chips são do usuário autenticado.
func (s *Service) assertOwnership(ctx context.Context, userID string, instanceIDs ...int) error {
	instances, err := s.instanceRepo.GetUserInstances(ctx, userID)
	if err != nil {
		return err
	}
	owned := make(map[int]struct{}, len(instances))
	for _, inst := range instances {
		owned[inst.ID] = struct{}{}
	}
	for _, id := range instanceIDs {
		if _, ok := owned[id]; !ok {
			// NotFound, não PermissionDenied: revelar "existe mas não é seu"
			// já é vazamento.
			return ErrNotFound
		}
	}
	return nil
}

func (s *Service) sessionNameFor(ctx context.Context, userID string, instanceID int) (string, error) {
	instances, err := s.instanceRepo.GetUserInstances(ctx, userID)
	if err != nil {
		return "", err
	}
	for _, inst := range instances {
		if inst.ID == instanceID {
			if inst.InstanceID == "" {
				return "", fmt.Errorf("%w: chip has no WAHA session", ErrStateConflict)
			}
			return inst.InstanceID, nil
		}
	}
	return "", ErrNotFound
}

func eventNames(kind string) (requested, gateClosed, applied, completed, aborted, timeout string) {
	if kind == models.FailoverDirectionRevert {
		return models.FailoverEventRevertRequested, models.FailoverEventRevertGateClosed,
			models.FailoverEventRevertApplied, models.FailoverEventRevertCompleted,
			models.FailoverEventRevertAborted, models.FailoverEventRevertDrainTimeout
	}
	return models.FailoverEventSwitchRequested, models.FailoverEventSwitchGateClosed,
		models.FailoverEventSwitchApplied, models.FailoverEventSwitchCompleted,
		models.FailoverEventSwitchAborted, models.FailoverEventSwitchDrainTimeout
}

func failedEventName(kind string) string {
	if kind == models.FailoverDirectionRevert {
		return models.FailoverEventRevertFailed
	}
	return models.FailoverEventSwitchFailed
}

// translateRepoError converte pgx.ErrNoRows em ErrNotFound; os erros sentinela
// do repositório passam intactos para a borda mapear.
func translateRepoError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func optIntStr(v *int) string {
	if v == nil {
		return ""
	}
	return strconv.Itoa(*v)
}

func keysSorted(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		return []byte(`{}`)
	}
	return b
}

// GetPolicyRaw devolve a política sem o contexto de leitura, para a borda
// resolver defaults (ex.: FailoverMode UNSPECIFIED num update deve preservar o
// modo atual, não silenciosamente virar MANUAL).
func (s *Service) GetPolicyRaw(ctx context.Context, userID string, policyID int64) (*models.InstanceFailoverPolicy, error) {
	if err := s.ensureEnabled(); err != nil {
		return nil, err
	}
	return s.repo.GetPolicy(ctx, policyID, userID)
}

// OperationItems devolve os itens de uma operação, para o relatório da API.
func (s *Service) OperationItems(ctx context.Context, operationID string) ([]models.InstanceFailoverOperationItem, error) {
	if err := s.ensureEnabled(); err != nil {
		return nil, err
	}
	return s.repo.ListOperationItems(ctx, operationID)
}
