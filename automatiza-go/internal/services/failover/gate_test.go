package failover

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"automatiza-go/internal/repository"

	"go.uber.org/zap"
)

// fakeGateRepo implementa só o que o gate usa. O embed da interface faz
// qualquer outro método explodir se for chamado — que é o alarme que queremos.
type fakeGateRepo struct {
	repository.InstanceFailoverRepository

	mu           sync.Mutex
	acquireCalls int
	renewCalls   int
	releaseCalls int

	acquireErr error
	renewOK    bool
	renewErr   error
}

func (f *fakeGateRepo) AcquireLease(_ context.Context, _ repository.FailoverLeaseInput) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.acquireCalls++
	if f.acquireErr != nil {
		return "", f.acquireErr
	}
	return "lease-1", nil
}

func (f *fakeGateRepo) RenewLease(_ context.Context, _ string, _ time.Duration) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.renewCalls++
	return f.renewOK, f.renewErr
}

func (f *fakeGateRepo) ReleaseLease(_ context.Context, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.releaseCalls++
	return nil
}

func (f *fakeGateRepo) counts() (int, int, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.acquireCalls, f.renewCalls, f.releaseCalls
}

// Com a feature desligada e nada pendente, o gate é no-op SEM I/O — o caminho
// quente de envio não paga nada por uma feature apagada.
func TestGate_InactiveIsNoopWithoutIO(t *testing.T) {
	repo := &fakeGateRepo{}
	g := NewGate(repo, func() bool { return false }, zap.NewNop())

	ctx := context.Background()
	lease, err := g.Acquire(ctx, "user", "auto")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lease.Release()

	if acquire, _, release := repo.counts(); acquire != 0 || release != 0 {
		t.Fatalf("inactive gate must not touch the database (acquire=%d release=%d)", acquire, release)
	}
	if lease.Ctx != ctx {
		t.Fatal("inactive gate must return the caller context untouched")
	}
	if lease.SendCtx == nil || lease.SendCtx.Err() != nil {
		t.Fatal("inactive gate must return a live send context (historical Background behaviour)")
	}
}

// Armado no boot, o gate volta a valer MESMO com a flag desligada: um rollback
// no meio de uma troca não pode abrir o gate.
func TestGate_ArmedOverridesDisabledFlag(t *testing.T) {
	repo := &fakeGateRepo{renewOK: true}
	g := NewGate(repo, func() bool { return false }, zap.NewNop())
	g.SetArmed(true)

	lease, err := g.Acquire(context.Background(), "user", "auto")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer lease.Release()

	if acquire, _, _ := repo.counts(); acquire != 1 {
		t.Fatal("armed gate must acquire a lease even with the flag off")
	}
}

// 13.4.2 — operação em curso impede o produtor de começar.
func TestGate_InProgressBlocksProducer(t *testing.T) {
	repo := &fakeGateRepo{acquireErr: repository.ErrFailoverInProgress}
	g := NewGate(repo, func() bool { return true }, zap.NewNop())

	_, err := g.Acquire(context.Background(), "user", "auto")
	if !errors.Is(err, repository.ErrFailoverInProgress) {
		t.Fatalf("expected ErrFailoverInProgress, got %v", err)
	}
}

// Erro de banco é FAIL-CLOSED: na dúvida sobre poder enviar, não envia.
func TestGate_DatabaseErrorFailsClosed(t *testing.T) {
	repo := &fakeGateRepo{acquireErr: errors.New("connection refused")}
	g := NewGate(repo, func() bool { return true }, zap.NewNop())

	if _, err := g.Acquire(context.Background(), "user", "auto"); err == nil {
		t.Fatal("database error must fail closed")
	}
}

func TestGate_MissingIdentityFailsClosed(t *testing.T) {
	g := NewGate(&fakeGateRepo{}, func() bool { return true }, zap.NewNop())
	if _, err := g.Acquire(context.Background(), "", "auto"); err == nil {
		t.Fatal("missing user id must fail closed")
	}
}

// 13.4.5 — heartbeat perdido cancela os DOIS contextos.
func TestGate_LostHeartbeatCancelsBothContexts(t *testing.T) {
	repo := &fakeGateRepo{renewOK: false}
	g := NewGate(repo, func() bool { return true }, zap.NewNop())
	g.heartbeat = 5 * time.Millisecond

	lease, err := g.Acquire(context.Background(), "user", "auto")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer lease.Release()

	select {
	case <-lease.Ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("lease context must be cancelled when the heartbeat is lost")
	}
	select {
	case <-lease.SendCtx.Done():
	case <-time.After(time.Second):
		t.Fatal("send context must be cancelled too — that is what stops the fan-out")
	}
}

// O SendCtx sobrevive ao cancelamento do CHAMADOR (comportamento histórico do
// espelho: cancelar o consumidor de webhook nunca abortou um envio em curso).
func TestGate_SendContextSurvivesCallerCancellation(t *testing.T) {
	repo := &fakeGateRepo{renewOK: true}
	g := NewGate(repo, func() bool { return true }, zap.NewNop())
	g.heartbeat = time.Hour

	ctx, cancel := context.WithCancel(context.Background())
	lease, err := g.Acquire(ctx, "user", "auto")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer lease.Release()

	cancel()

	select {
	case <-lease.Ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("lease context inherits the caller and should be cancelled")
	}
	if lease.SendCtx.Err() != nil {
		t.Fatal("send context must NOT die with the caller")
	}
}

func TestGate_ReleaseIsIdempotent(t *testing.T) {
	repo := &fakeGateRepo{renewOK: true}
	g := NewGate(repo, func() bool { return true }, zap.NewNop())
	g.heartbeat = time.Hour

	lease, err := g.Acquire(context.Background(), "user", "auto")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lease.Release()
	lease.Release()

	if _, _, release := repo.counts(); release != 1 {
		t.Fatalf("release must run once, ran %d times", release)
	}
}

func TestGate_HolderIDCarriesNoUserData(t *testing.T) {
	g := NewGate(&fakeGateRepo{}, func() bool { return true }, zap.NewNop())
	if g.HolderID() == "" {
		t.Fatal("holder id must identify the process")
	}
	// hostname:pid:uuid — três partes, nenhuma delas do usuário.
	parts := 1
	for _, c := range g.HolderID() {
		if c == ':' {
			parts++
		}
	}
	if parts < 3 {
		t.Fatalf("holder id should be <hostname>:<pid>:<uuid>, got %q", g.HolderID())
	}
}
