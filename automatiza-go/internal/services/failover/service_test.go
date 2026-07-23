package failover

import (
	"context"
	"errors"
	"testing"
	"time"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"

	"go.uber.org/zap"
)

// =============================================================================
// Harness do fluxo de confirmação (seções 7.4/7.5 e testes 13.5 do plano).
//
// O fake registra a ORDEM das chamadas. É isso que prova as garantias que
// importam aqui — "o gate fechou ANTES de aplicar", "abortou SEM aplicar" — e
// que nenhuma asserção sobre estado final consegue provar sozinha.
// =============================================================================

type fakeServiceRepo struct {
	repository.InstanceFailoverRepository

	policy   *models.InstanceFailoverPolicy
	dryRun   *models.InstanceFailoverDryRun
	rows     []repository.FailoverAutomationRouting
	lastOp   *models.InstanceFailoverOperation
	opItems  []models.InstanceFailoverOperationItem
	replayOp bool

	// drain
	leases     int
	dispatches int

	// registro de chamadas, em ordem
	calls []string

	applyErr error
}

func (f *fakeServiceRepo) record(name string) { f.calls = append(f.calls, name) }

func (f *fakeServiceRepo) called(name string) bool {
	for _, c := range f.calls {
		if c == name {
			return true
		}
	}
	return false
}

func (f *fakeServiceRepo) GetPolicy(context.Context, int64, string) (*models.InstanceFailoverPolicy, error) {
	return f.policy, nil
}

func (f *fakeServiceRepo) GetDryRun(context.Context, string, string) (*models.InstanceFailoverDryRun, error) {
	return f.dryRun, nil
}

func (f *fakeServiceRepo) ListUserAutomationsRouting(context.Context, string) ([]repository.FailoverAutomationRouting, error) {
	return f.rows, nil
}

func (f *fakeServiceRepo) InsertOperation(_ context.Context, op *models.InstanceFailoverOperation) (*models.InstanceFailoverOperation, bool, error) {
	f.record("InsertOperation")
	op.ID = "op-1"
	op.Status = models.FailoverOpPreparing
	f.lastOp = op
	return op, f.replayOp, nil
}

func (f *fakeServiceRepo) CloseGate(_ context.Context, in repository.FailoverCloseGateInput) (*models.InstanceFailoverOperation, error) {
	f.record("CloseGate")
	f.opItems = in.Items
	now := time.Now().UTC()
	f.lastOp.Status = models.FailoverOpDraining
	f.lastOp.GateClosedAt = &now
	f.lastOp.DrainDeadlineAt = &in.DrainDeadlineAt
	return f.lastOp, nil
}

func (f *fakeServiceRepo) CountActiveLeases(context.Context, []string, time.Time) (int, error) {
	f.record("CountActiveLeases")
	return f.leases, nil
}

func (f *fakeServiceRepo) CountNonTerminalDispatches(context.Context, []string, time.Time) (int, error) {
	f.record("CountNonTerminalDispatches")
	return f.dispatches, nil
}

func (f *fakeServiceRepo) ApplyProjection(context.Context, repository.FailoverApplyInput) error {
	f.record("ApplyProjection")
	return f.applyErr
}

func (f *fakeServiceRepo) TerminateOperation(_ context.Context, in repository.FailoverTerminateInput) (*models.InstanceFailoverPolicy, error) {
	f.record("TerminateOperation:" + in.OperationStatus)
	f.lastOp.Status = in.OperationStatus
	if in.PolicyState != "" {
		f.policy.State = in.PolicyState
	}
	return f.policy, nil
}

func (f *fakeServiceRepo) GetOperation(context.Context, string, string) (*models.InstanceFailoverOperation, error) {
	return f.lastOp, nil
}

func (f *fakeServiceRepo) ListOperationItems(context.Context, string) ([]models.InstanceFailoverOperationItem, error) {
	return f.opItems, nil
}

func (f *fakeServiceRepo) InsertEvent(_ context.Context, ev *models.InstanceFailoverEvent) (*models.InstanceFailoverEvent, error) {
	f.record("InsertEvent:" + ev.EventType)
	return ev, nil
}

func (f *fakeServiceRepo) GetLastCompletedOperation(context.Context, int64, string, string) (*models.InstanceFailoverOperation, error) {
	return nil, nil
}

type fakeServiceInstanceRepo struct {
	repository.InstanceRepository
}

func (f *fakeServiceInstanceRepo) GetUserInstances(context.Context, string) ([]models.Instance, error) {
	return []models.Instance{
		{ID: 10, UserID: "u1", InstanceID: "sess-primary", Status: models.InstanceStatusConnected},
		{ID: 20, UserID: "u1", InstanceID: "sess-backup", Status: models.InstanceStatusConnected},
	}, nil
}

// buildHarness monta um cenário confirmável: 1 automação usando a primária (10),
// reserva (20) WORKING e participante do grupo de destino.
func buildHarness(t *testing.T) (*Service, *fakeServiceRepo, *fakeProvider) {
	t.Helper()

	row := row("11111111-1111-1111-1111-111111111111", []string{"10"}, []string{"g1@g.us"})
	set := BuildSwapSet([]repository.FailoverAutomationRouting{row}, 10, 20)
	if set.Blocked() {
		t.Fatalf("fixture must be clean: %v", set.BlockingReasons)
	}

	policy := &models.InstanceFailoverPolicy{
		ID: 1, UserID: "u1",
		PrimaryInstanceID: 10, BackupInstanceID: 20,
		Mode: models.FailoverModeManual, State: models.FailoverStateActive,
		Version: 3, DrainDeadlineSeconds: 120,
	}

	// A ref precisa bater exatamente com o que o serviço recalcula.
	refItems := []models.DryRunRefItem{{
		AutomationID: set.Items[0].AutomationID,
		Before:       set.Items[0].Before.Fingerprint(),
		After:        set.Items[0].After.Fingerprint(),
	}}
	cov := map[string]ItemCoverage{
		set.Items[0].AutomationID: {
			RequiredTargets: []string{"g1@g.us"},
			RequiredSources: []string{},
			Result:          models.FailoverCoverageCovered,
		},
	}
	ref := models.DryRunRef(policy.ID, policy.Version, models.FailoverDirectionFailover, refItems, CoverageHash(cov))

	repo := &fakeServiceRepo{
		policy: policy,
		rows:   []repository.FailoverAutomationRouting{row},
		dryRun: &models.InstanceFailoverDryRun{
			ID: "dr-1", PolicyIDSnapshot: 1, UserID: "u1",
			Direction: models.FailoverDirectionFailover,
			Status:    models.FailoverDryRunPass,
			DryRunRef: ref, PolicyVersion: policy.Version,
			ExpiresAt: time.Now().UTC().Add(10 * time.Minute),
		},
	}

	provider := &fakeProvider{
		status: workingStatus(),
		groups: []whatsapp.GroupInfo{{ID: "g1@g.us"}},
	}

	svc := NewService(repo, &fakeServiceInstanceRepo{}, provider, func() bool { return true }, zap.NewNop())
	svc.coverage.gap = 0
	svc.sleep = func(context.Context, time.Duration) bool { return true }
	return svc, repo, provider
}

func confirmInput() ConfirmInput {
	return ConfirmInput{
		UserID: "u1", PolicyID: 1, ExpectedVersion: 3,
		DryRunID: "dr-1", IdempotencyKey: "chave-idempotente-valida",
	}
}

// 13.5.3 — caminho feliz: postcheck aprovado finaliza em active_on_backup.
func TestConfirm_HappyPath(t *testing.T) {
	svc, repo, _ := buildHarness(t)
	in := confirmInput()
	in.DryRunRef = repo.dryRun.DryRunRef

	res, err := svc.ConfirmFailover(context.Background(), in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Policy.State != models.FailoverStateActiveOnBackup {
		t.Fatalf("policy should end on the backup chip, got %s", res.Policy.State)
	}
	if !repo.called("TerminateOperation:completed") {
		t.Fatalf("operation should complete, calls: %v", repo.calls)
	}

	// A ORDEM é a garantia: gate fecha, drena, só então aplica.
	gate, drain, apply := indexOf(repo.calls, "CloseGate"),
		indexOf(repo.calls, "CountActiveLeases"),
		indexOf(repo.calls, "ApplyProjection")
	if !(gate < drain && drain < apply) {
		t.Fatalf("expected CloseGate < drain < ApplyProjection, got %v", repo.calls)
	}
}

// 13.4.7 — trabalho em voo no deadline aborta e NENHUMA automação muda.
//
// É o invariante mais caro da feature: prosseguir aqui deixaria um job Asynq
// com o chip antigo congelado enviando depois da troca — mensagem duplicada no
// grupo do cliente.
func TestConfirm_DrainTimeoutAbortsWithoutApplying(t *testing.T) {
	svc, repo, _ := buildHarness(t)
	repo.leases = 1 // nunca drena

	// Relógio que salta o deadline na segunda leitura.
	calls := 0
	base := time.Now().UTC()
	svc.now = func() time.Time {
		calls++
		if calls > 3 {
			return base.Add(10 * time.Minute)
		}
		return base
	}

	in := confirmInput()
	in.DryRunRef = repo.dryRun.DryRunRef

	_, err := svc.ConfirmFailover(context.Background(), in)
	if !errors.Is(err, ErrDrainTimeout) {
		t.Fatalf("expected ErrDrainTimeout, got %v", err)
	}
	if repo.called("ApplyProjection") {
		t.Fatal("drain timeout must NEVER apply the projection")
	}
	if !repo.called("TerminateOperation:aborted") {
		t.Fatalf("operation should abort, calls: %v", repo.calls)
	}
	if repo.policy.State != models.FailoverStateActive {
		t.Fatalf("policy must return to active, got %s", repo.policy.State)
	}
	if !repo.called("InsertEvent:" + models.FailoverEventSwitchDrainTimeout) {
		t.Fatalf("drain timeout must be recorded in the ledger, calls: %v", repo.calls)
	}
}

// Dispatch pendente também segura o drain (não só a lease).
func TestConfirm_PendingDispatchBlocksDrain(t *testing.T) {
	svc, repo, _ := buildHarness(t)
	repo.leases = 0
	repo.dispatches = 2

	calls := 0
	base := time.Now().UTC()
	svc.now = func() time.Time {
		calls++
		if calls > 3 {
			return base.Add(10 * time.Minute)
		}
		return base
	}

	in := confirmInput()
	in.DryRunRef = repo.dryRun.DryRunRef

	if _, err := svc.ConfirmFailover(context.Background(), in); !errors.Is(err, ErrDrainTimeout) {
		t.Fatalf("expected ErrDrainTimeout, got %v", err)
	}
	if repo.called("ApplyProjection") {
		t.Fatal("in-flight dispatch must block the switch")
	}
}

// 13.5.4 — reserva deixa de cobrir entre o gate e a aplicação: aborta sem tocar
// em automação nenhuma.
func TestConfirm_CoverageLostAfterGateAborts(t *testing.T) {
	svc, repo, provider := buildHarness(t)
	// Depois do gate, a reserva "sai" do grupo.
	provider.groups = nil

	in := confirmInput()
	in.DryRunRef = repo.dryRun.DryRunRef

	_, err := svc.ConfirmFailover(context.Background(), in)
	if !errors.Is(err, ErrCoverageBlocked) {
		t.Fatalf("expected ErrCoverageBlocked, got %v", err)
	}
	if repo.called("ApplyProjection") {
		t.Fatal("coverage lost after the gate must abort before applying")
	}
	if repo.policy.State != models.FailoverStateActive {
		t.Fatalf("policy must return to active, got %s", repo.policy.State)
	}
}

// 13.5.1 — confirmar em estado errado é FailedPrecondition, sem abrir operação.
func TestConfirm_IllegalStateIsRejectedBeforeAnyWrite(t *testing.T) {
	svc, repo, _ := buildHarness(t)
	repo.policy.State = models.FailoverStateActiveOnBackup

	in := confirmInput()
	in.DryRunRef = repo.dryRun.DryRunRef

	if _, err := svc.ConfirmFailover(context.Background(), in); !errors.Is(err, ErrStateConflict) {
		t.Fatalf("expected ErrStateConflict, got %v", err)
	}
	if len(repo.calls) != 0 {
		t.Fatalf("illegal transition must not touch the repository, calls: %v", repo.calls)
	}
}

// 13.5.2 — ABA: versão antiga é Aborted, sem operação.
func TestConfirm_StaleVersionIsAborted(t *testing.T) {
	svc, repo, _ := buildHarness(t)
	in := confirmInput()
	in.DryRunRef = repo.dryRun.DryRunRef
	in.ExpectedVersion = 2 // política está na 3

	if _, err := svc.ConfirmFailover(context.Background(), in); !errors.Is(err, repository.ErrFailoverVersionConflict) {
		t.Fatalf("expected version conflict, got %v", err)
	}
	if len(repo.calls) != 0 {
		t.Fatalf("stale version must not open an operation, calls: %v", repo.calls)
	}
}

// Um ensaio BLOQUEADO nunca pode ser confirmado — nem com a ref certa.
func TestConfirm_BlockedDryRunIsNeverConfirmable(t *testing.T) {
	svc, repo, _ := buildHarness(t)
	repo.dryRun.Status = models.FailoverDryRunBlocked

	in := confirmInput()
	in.DryRunRef = repo.dryRun.DryRunRef

	if _, err := svc.ConfirmFailover(context.Background(), in); !errors.Is(err, ErrDryRunMismatch) {
		t.Fatalf("expected ErrDryRunMismatch, got %v", err)
	}
	if len(repo.calls) != 0 {
		t.Fatal("a blocked dry run must not open an operation")
	}
}

// Ensaio vencido (TTL de 15min) é recusado.
func TestConfirm_ExpiredDryRunIsRejected(t *testing.T) {
	svc, repo, _ := buildHarness(t)
	repo.dryRun.ExpiresAt = time.Now().UTC().Add(-time.Minute)

	in := confirmInput()
	in.DryRunRef = repo.dryRun.DryRunRef

	if _, err := svc.ConfirmFailover(context.Background(), in); !errors.Is(err, ErrDryRunExpired) {
		t.Fatalf("expected ErrDryRunExpired, got %v", err)
	}
}

// Ref que não corresponde ao ensaio é recusada: é o que impede confirmar um
// ensaio que já não descreve a realidade.
func TestConfirm_WrongRefIsRejected(t *testing.T) {
	svc, repo, _ := buildHarness(t)
	in := confirmInput()
	in.DryRunRef = "ref-de-outro-ensaio"

	if _, err := svc.ConfirmFailover(context.Background(), in); !errors.Is(err, ErrDryRunMismatch) {
		t.Fatalf("expected ErrDryRunMismatch, got %v", err)
	}
	if len(repo.calls) != 0 {
		t.Fatal("wrong ref must not open an operation")
	}
}

// 13.5.8 — replay da mesma key devolve o estado gravado, sem segunda execução.
func TestConfirm_ReplayDoesNotRunTwice(t *testing.T) {
	svc, repo, provider := buildHarness(t)
	repo.replayOp = true

	in := confirmInput()
	in.DryRunRef = repo.dryRun.DryRunRef

	if _, err := svc.ConfirmFailover(context.Background(), in); err != nil {
		t.Fatalf("replay should not error: %v", err)
	}
	if repo.called("CloseGate") || repo.called("ApplyProjection") {
		t.Fatalf("replay must not re-execute, calls: %v", repo.calls)
	}
	// E nenhuma chamada nova ao WAHA.
	for _, c := range provider.calls {
		t.Fatalf("replay must not call the provider, got %q", c)
	}
}

// 13.3.7 — automação nova entre o ensaio e o gate muda a ref: aborta tudo.
func TestConfirm_SetChangedSinceDryRunAborts(t *testing.T) {
	svc, repo, _ := buildHarness(t)
	// Nasce outra automação usando a primária depois do ensaio.
	repo.rows = append(repo.rows,
		row("22222222-2222-2222-2222-222222222222", []string{"10"}, []string{"g9@g.us"}))

	in := confirmInput()
	in.DryRunRef = repo.dryRun.DryRunRef

	_, err := svc.ConfirmFailover(context.Background(), in)
	if err == nil {
		t.Fatal("a changed eligible set must not be confirmable with the old ref")
	}
	if repo.called("ApplyProjection") {
		t.Fatalf("must not apply with a stale set, calls: %v", repo.calls)
	}
}

// 13.3.6 — automação implícita que apareceu depois bloqueia a conta inteira.
func TestConfirm_ImplicitAutomationBlocksBeforeGate(t *testing.T) {
	svc, repo, _ := buildHarness(t)
	repo.rows = append(repo.rows,
		row("33333333-3333-3333-3333-333333333333", nil, []string{"g9@g.us"}))

	in := confirmInput()
	in.DryRunRef = repo.dryRun.DryRunRef

	if _, err := svc.ConfirmFailover(context.Background(), in); err == nil {
		t.Fatal("implicit routing must block the confirmation")
	}
	if repo.called("CloseGate") {
		t.Fatal("blocking reasons must be caught BEFORE closing the gate")
	}
}

// stale_configuration na transação de aplicação aborta sem estado parcial.
func TestConfirm_StaleConfigurationDuringApplyAborts(t *testing.T) {
	svc, repo, _ := buildHarness(t)
	repo.applyErr = repository.ErrFailoverStaleConfiguration

	in := confirmInput()
	in.DryRunRef = repo.dryRun.DryRunRef

	if _, err := svc.ConfirmFailover(context.Background(), in); err == nil {
		t.Fatal("stale configuration must fail the confirmation")
	}
	if !repo.called("TerminateOperation:aborted") {
		t.Fatalf("must abort, calls: %v", repo.calls)
	}
	if repo.policy.State != models.FailoverStateActive {
		t.Fatalf("policy must return to active, got %s", repo.policy.State)
	}
}

// 13.5.12 — DisablePolicy é proibido enquanto o roteamento está na reserva.
func TestDisablePolicy_BlockedOnBackup(t *testing.T) {
	svc, repo, _ := buildHarness(t)
	repo.policy.State = models.FailoverStateActiveOnBackup

	_, err := svc.DisablePolicy(context.Background(), "u1", 1, 3, "chave-idempotente-valida")
	if !errors.Is(err, ErrStateConflict) {
		t.Fatalf("expected ErrStateConflict, got %v", err)
	}
}

// 13.5.9 — reverter sem failover concluído é recusado.
func TestRevert_RequiresCompletedFailover(t *testing.T) {
	svc, repo, _ := buildHarness(t)
	repo.policy.State = models.FailoverStateActiveOnBackup
	repo.dryRun.Direction = models.FailoverDirectionRevert

	in := confirmInput()
	in.DryRunRef = repo.dryRun.DryRunRef

	if _, err := svc.RevertFailover(context.Background(), in); !errors.Is(err, ErrStateConflict) {
		t.Fatalf("expected ErrStateConflict, got %v", err)
	}
}

// Feature desligada: nenhuma RPC toca o repositório.
func TestConfirm_DisabledTouchesNothing(t *testing.T) {
	svc, repo, _ := buildHarness(t)
	svc.enabled = func() bool { return false }

	in := confirmInput()
	in.DryRunRef = repo.dryRun.DryRunRef

	if _, err := svc.ConfirmFailover(context.Background(), in); !errors.Is(err, ErrDisabled) {
		t.Fatalf("expected ErrDisabled, got %v", err)
	}
	if len(repo.calls) != 0 {
		t.Fatalf("disabled feature must not touch the repository, calls: %v", repo.calls)
	}
}

func indexOf(list []string, want string) int {
	for i, v := range list {
		if v == want {
			return i
		}
	}
	return -1
}
