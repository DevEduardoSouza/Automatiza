package workers

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"automatiza-go/internal/models"
	"automatiza-go/internal/services/monitoroffer"

	"github.com/hibiken/asynq"
	"go.uber.org/zap/zaptest"
)

const wkTestKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

// fakeRetryRepo implementa só o que o worker usa; o resto entra em pânico se
// alguém chamar por engano — é o que impede o teste de passar por acidente.
type fakeRetryRepo struct {
	att            *models.MonitorOfferAttempt
	claimErr       error
	claims         int
	attemptStarted int
	finished       []models.MonitorOfferItemOutcome
	rescheduled    []string
}

func (f *fakeRetryRepo) ClaimItemForAttempt(_ context.Context, _ string, _ int) (*models.MonitorOfferAttempt, error) {
	f.claims++
	return f.att, f.claimErr
}
func (f *fakeRetryRepo) MarkAttemptStarted(_ context.Context, _ string) error {
	f.attemptStarted++
	return nil
}
func (f *fakeRetryRepo) FinishItem(_ context.Context, o models.MonitorOfferItemOutcome) error {
	f.finished = append(f.finished, o)
	return nil
}
func (f *fakeRetryRepo) RescheduleItem(_ context.Context, id string, _ time.Time, _ string, _ bool) error {
	f.rescheduled = append(f.rescheduled, id)
	return nil
}

// --- métodos não usados pelo worker ---

func (f *fakeRetryRepo) CreateJobWithItems(context.Context, *models.MonitorOfferJob, []models.MonitorOfferItem) (bool, error) {
	panic("não deveria ser chamado")
}
func (f *fakeRetryRepo) GetJob(context.Context, string, string) (*models.MonitorOfferJob, error) {
	panic("não deveria ser chamado")
}
func (f *fakeRetryRepo) ListItems(context.Context, string) ([]models.MonitorOfferItem, error) {
	panic("não deveria ser chamado")
}
func (f *fakeRetryRepo) ClaimDueItems(context.Context, int) ([]models.MonitorOfferItemLease, error) {
	panic("não deveria ser chamado")
}
func (f *fakeRetryRepo) PublishCanonicalKey(context.Context, string, string) (models.CanonicalKeyVerdict, error) {
	panic("não deveria ser chamado")
}
func (f *fakeRetryRepo) UpsertDelivery(context.Context, models.MonitorOfferDelivery) error {
	panic("não deveria ser chamado")
}
func (f *fakeRetryRepo) MarkDeliverySending(context.Context, string, string, string, string) (bool, error) {
	panic("não deveria ser chamado")
}
func (f *fakeRetryRepo) FinishDelivery(context.Context, string, string, string, string, string, string, *string) error {
	panic("não deveria ser chamado")
}
func (f *fakeRetryRepo) ListDeliveries(context.Context, string, string) ([]models.MonitorOfferDelivery, error) {
	panic("não deveria ser chamado")
}
func (f *fakeRetryRepo) ReapStaleLeases(context.Context) (models.MonitorOfferReapStats, error) {
	panic("não deveria ser chamado")
}
func (f *fakeRetryRepo) ExpirePayloads(context.Context) (int, error) {
	panic("não deveria ser chamado")
}
func (f *fakeRetryRepo) DeleteTerminalOlderThan(context.Context, time.Duration, int) (int, error) {
	panic("não deveria ser chamado")
}
func (f *fakeRetryRepo) GetItemForReplay(context.Context, string, string) (*models.MonitorOfferItem, *models.MonitorOfferJob, error) {
	panic("não deveria ser chamado")
}
func (f *fakeRetryRepo) ResetItemForReplay(context.Context, string, string, bool) error {
	panic("não deveria ser chamado")
}

type fakeExecutor struct {
	calls   int
	outcome models.MonitorOfferItemOutcome
	err     error
	sawBody string
}

func (f *fakeExecutor) ExecuteRetry(_ context.Context, att models.MonitorOfferAttempt, p models.MonitorOfferPayload) (models.MonitorOfferItemOutcome, error) {
	f.calls++
	f.sawBody = p.BodyAfterStory
	out := f.outcome
	if out.ItemID == "" {
		out.ItemID = att.Item.ID
	}
	return out, f.err
}

func taskFor(t *testing.T, itemID string, gen int) *asynq.Task {
	t.Helper()
	b, _ := json.Marshal(MonitorOfferRetryJobData{ItemID: itemID, Generation: gen})
	return asynq.NewTask(TypeMonitorOfferRetry, b)
}

func attemptWithPayload(t *testing.T, body string) *models.MonitorOfferAttempt {
	t.Helper()
	ct, err := monitoroffer.EncodePayload(models.MonitorOfferPayload{BodyAfterStory: body}, wkTestKey)
	if err != nil {
		t.Fatalf("payload: %v", err)
	}
	return &models.MonitorOfferAttempt{
		Item: models.MonitorOfferItem{ID: "item-1", JobID: "job-1", SourceLinkHash: "abcdef0123456789abcdef", Host: "shopee.com.br"},
		Job:  models.MonitorOfferJob{ID: "job-1", UserID: "user-1", PayloadCiphertext: &ct},
	}
}

// TestGeracaoObsoletaEhNoOp é o contrato que torna seguro perder um ack de
// enqueue: a task antiga chega, o CAS não casa, e NADA acontece.
func TestGeracaoObsoletaEhNoOp(t *testing.T) {
	repo := &fakeRetryRepo{att: nil} // CAS não casou
	exec := &fakeExecutor{}
	w := NewMonitorOfferRetryWorker(repo, exec, wkTestKey, zaptest.NewLogger(t))

	if err := w.ProcessTask(context.Background(), taskFor(t, "item-1", 7)); err != nil {
		t.Fatalf("geração obsoleta não pode virar erro: %v", err)
	}
	if exec.calls != 0 {
		t.Error("executor não pode rodar com geração obsoleta")
	}
	if repo.attemptStarted != 0 {
		t.Error("tentativa não pode ser consumida com geração obsoleta")
	}
	if len(repo.finished)+len(repo.rescheduled) != 0 {
		t.Error("nenhuma transição deveria acontecer")
	}
}

// TestPayloadAusenteNaoEnvia: sem payload não dá para reclassificar, então o
// item encerra em vez de virar envio às cegas.
func TestPayloadAusenteNaoEnvia(t *testing.T) {
	repo := &fakeRetryRepo{att: &models.MonitorOfferAttempt{
		Item: models.MonitorOfferItem{ID: "item-1", JobID: "job-1"},
		Job:  models.MonitorOfferJob{ID: "job-1", UserID: "user-1"},
	}}
	exec := &fakeExecutor{}
	w := NewMonitorOfferRetryWorker(repo, exec, wkTestKey, zaptest.NewLogger(t))

	_ = w.ProcessTask(context.Background(), taskFor(t, "item-1", 1))

	if exec.calls != 0 {
		t.Error("executor não pode rodar sem payload")
	}
	if len(repo.finished) != 1 || repo.finished[0].ReasonCode != models.ReasonPayloadExpired {
		t.Errorf("desfecho = %+v, queria exhausted/payload_expired", repo.finished)
	}
	if repo.attemptStarted != 0 {
		t.Error("tentativa não pode ser consumida sem payload")
	}
}

// TestPayloadIndecifravelBloqueiaEnaoEnvia: chave trocada ou dado corrompido é
// permanente e alarmável — jamais "manda assim mesmo".
func TestPayloadIndecifravelBloqueiaEnaoEnvia(t *testing.T) {
	att := attemptWithPayload(t, "corpo")
	repo := &fakeRetryRepo{att: att}
	exec := &fakeExecutor{}
	// Chave DIFERENTE da que cifrou.
	outra := "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"
	w := NewMonitorOfferRetryWorker(repo, exec, outra, zaptest.NewLogger(t))

	_ = w.ProcessTask(context.Background(), taskFor(t, "item-1", 1))

	if exec.calls != 0 {
		t.Error("executor não pode rodar com payload indecifrável")
	}
	if len(repo.finished) != 1 {
		t.Fatalf("desfechos = %d, queria 1", len(repo.finished))
	}
	got := repo.finished[0]
	if got.Status != models.MonitorItemBlocked || got.ReasonCode != models.ReasonPayloadDecryptFail {
		t.Errorf("desfecho = %+v, queria blocked/payload_decrypt_failed", got)
	}
}

// TestTentativaSoContaDepoisDoPayloadDecifrado é a assimetria deliberada do
// worker: um crash antes disso devolveria o item sem queimar tentativa.
func TestTentativaSoContaDepoisDoPayloadDecifrado(t *testing.T) {
	repo := &fakeRetryRepo{att: attemptWithPayload(t, "corpo")}
	exec := &fakeExecutor{outcome: models.MonitorOfferItemOutcome{
		Status: models.MonitorItemSucceeded, ReasonCode: models.ReasonNone,
	}}
	w := NewMonitorOfferRetryWorker(repo, exec, wkTestKey, zaptest.NewLogger(t))

	_ = w.ProcessTask(context.Background(), taskFor(t, "item-1", 1))

	if repo.attemptStarted != 1 {
		t.Errorf("MarkAttemptStarted chamado %d vezes, queria 1", repo.attemptStarted)
	}
	if exec.calls != 1 {
		t.Errorf("executor chamado %d vezes, queria 1", exec.calls)
	}
	if exec.sawBody != "corpo" {
		t.Errorf("o executor recebeu %q, queria o corpo decifrado", exec.sawBody)
	}
	if len(repo.finished) != 1 || repo.finished[0].Status != models.MonitorItemSucceeded {
		t.Errorf("desfecho = %+v", repo.finished)
	}
}

// TestShutdownNaoQueimaTentativa: context.Canceled é o processo caindo, não a
// Shopee falhando.
func TestShutdownNaoConsomeReagendamento(t *testing.T) {
	repo := &fakeRetryRepo{att: attemptWithPayload(t, "corpo")}
	exec := &fakeExecutor{err: context.Canceled}
	w := NewMonitorOfferRetryWorker(repo, exec, wkTestKey, zaptest.NewLogger(t))

	if err := w.ProcessTask(context.Background(), taskFor(t, "item-1", 1)); err != nil {
		t.Fatalf("shutdown não pode virar erro de task: %v", err)
	}
	if len(repo.rescheduled) != 0 {
		t.Error("shutdown não deve reagendar: o lease vence e o sweep recupera")
	}
	if len(repo.finished) != 0 {
		t.Error("shutdown não pode encerrar o item")
	}
}

// TestFalhaDeExecucaoReagenda: erro real da rede volta para pending.
func TestFalhaDeExecucaoReagenda(t *testing.T) {
	repo := &fakeRetryRepo{att: attemptWithPayload(t, "corpo")}
	exec := &fakeExecutor{err: errors.New("shopee 429")}
	w := NewMonitorOfferRetryWorker(repo, exec, wkTestKey, zaptest.NewLogger(t))

	_ = w.ProcessTask(context.Background(), taskFor(t, "item-1", 1))

	if len(repo.rescheduled) != 1 {
		t.Errorf("%d reagendamentos, queria 1", len(repo.rescheduled))
	}
	if len(repo.finished) != 0 {
		t.Error("falha transitória não encerra o item")
	}
}

// TestPayloadDaTaskNuncaCarregaURLOuTexto: o Redis não é lugar para a mensagem
// do cliente.
func TestPayloadDaTaskNuncaCarregaURLOuTexto(t *testing.T) {
	b, err := json.Marshal(MonitorOfferRetryJobData{ItemID: "item-1", Generation: 3})
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if len(m) != 2 {
		t.Errorf("payload da task tem %d campos (%v); só item_id e generation são permitidos", len(m), m)
	}
	for _, proibido := range []string{"url", "body", "text", "link", "message"} {
		if _, ok := m[proibido]; ok {
			t.Errorf("campo proibido %q no payload da task", proibido)
		}
	}
}

func TestTaskComPayloadInvalidoNaoEntraEmPanico(t *testing.T) {
	repo := &fakeRetryRepo{}
	w := NewMonitorOfferRetryWorker(repo, &fakeExecutor{}, wkTestKey, zaptest.NewLogger(t))
	task := asynq.NewTask(TypeMonitorOfferRetry, []byte("{isso não é json"))

	if err := w.ProcessTask(context.Background(), task); err != nil {
		t.Fatalf("payload corrompido não melhora com retry: %v", err)
	}
	if repo.claims != 0 {
		t.Error("não deveria nem tentar reivindicar")
	}
}
