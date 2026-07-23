package workers

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"automatiza-go/internal/lib/cache"
	"automatiza-go/internal/models"
	"automatiza-go/internal/services/instagram"

	"github.com/alicebob/miniredis/v2"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// --- fakes ---

type fakeReplier struct {
	mu sync.Mutex

	featureOff bool
	aiOff      bool
	cap        int
	err        error
	outcome    *instagram.ReplyOutcome
	calls      int
	gotEvents  []models.InstagramInboundEvent
}

func (f *fakeReplier) FeatureEnabled() bool { return !f.featureOff }
func (f *fakeReplier) AIReplyEnabled() bool { return !f.aiOff }
func (f *fakeReplier) DailyReplyCap(context.Context, string) int {
	return f.cap
}
func (f *fakeReplier) ReplyToEvent(_ context.Context, _ string, ev *models.InstagramInboundEvent) (*instagram.ReplyOutcome, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.gotEvents = append(f.gotEvents, *ev)
	if f.err != nil {
		return nil, f.err
	}
	if f.outcome != nil {
		return f.outcome, nil
	}
	return &instagram.ReplyOutcome{Text: "resposta", PromptTokens: 10, CompletionTokens: 5}, nil
}

type fakeReplyRepo struct {
	mu        sync.Mutex
	claimed   map[string]bool
	rows      map[string]*models.InstagramReply
	results   []string // "status:errorCode"
	sentToday int
	claimErr  error
	seq       int
}

func newFakeReplyRepo() *fakeReplyRepo {
	return &fakeReplyRepo{claimed: map[string]bool{}, rows: map[string]*models.InstagramReply{}}
}

func (f *fakeReplyRepo) Claim(_ context.Context, userID, eventID, eventType string) (*models.InstagramReply, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.claimErr != nil {
		return nil, false, f.claimErr
	}
	key := userID + "|" + eventID
	if f.claimed[key] {
		return f.rows[key], false, nil
	}
	f.seq++
	row := &models.InstagramReply{ID: "reply-" + strconv.Itoa(f.seq), UserID: userID,
		ProviderEventID: eventID, EventType: eventType, Status: models.InstagramReplyPending}
	f.claimed[key] = true
	f.rows[key] = row
	return row, true, nil
}

func (f *fakeReplyRepo) MarkResult(_ context.Context, id, status, text, code string, pt, ct int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.results = append(f.results, status+":"+code)
	return nil
}

func (f *fakeReplyRepo) CountSince(context.Context, string, time.Time) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.sentToday, nil
}

func (f *fakeReplyRepo) ListRecentByUser(context.Context, string, int) ([]models.InstagramReply, error) {
	return nil, nil
}

func (f *fakeReplyRepo) snapshot() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.results...)
}

type fakeInboundRepo struct {
	mu       sync.Mutex
	statuses []string // "id:status:code"
}

func (f *fakeInboundRepo) Create(context.Context, *models.InstagramInboundEvent) (*models.InstagramInboundEvent, bool, error) {
	return nil, false, nil
}
func (f *fakeInboundRepo) ListRecentByUser(context.Context, string, int) ([]models.InstagramInboundEvent, error) {
	return nil, nil
}
func (f *fakeInboundRepo) MarkStatus(_ context.Context, id, status, code string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.statuses = append(f.statuses, id+":"+status+":"+code)
	return nil
}
func (f *fakeInboundRepo) DeleteOlderThan(context.Context, time.Time, int) (int64, error) {
	return 0, nil
}

// --- harness ---

type replyHarness struct {
	w     *InstagramReplyWorker
	svc   *fakeReplier
	rep   *fakeReplyRepo
	inb   *fakeInboundRepo
	cache *cache.Cache
}

func newReplyWorkerTest(t *testing.T) *replyHarness {
	t.Helper()
	mr := miniredis.RunT(t)
	c := cache.NewCache(redis.NewClient(&redis.Options{Addr: mr.Addr()}), "test", false)
	svc := &fakeReplier{cap: 50}
	rep := newFakeReplyRepo()
	inb := &fakeInboundRepo{}
	w := NewInstagramReplyWorker(svc, inb, rep, c, zap.NewNop())
	return &replyHarness{w: w, svc: svc, rep: rep, inb: inb, cache: c}
}

func replyTask(t *testing.T, d InstagramReplyJobData) *asynq.Task {
	t.Helper()
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	return asynq.NewTask(TypeInstagramAIReply, b)
}

func commentJob(minutesAgo int) InstagramReplyJobData {
	at := time.Now().Add(-time.Duration(minutesAgo) * time.Minute)
	return InstagramReplyJobData{
		UserID: "user-A", InboundID: "inb-1", ProviderEventID: "COMENT-1",
		EventType: models.InstagramEventComment, SenderID: "VIS-9",
		Content: "quanto custa?", OccurredAt: &at,
	}
}

// --- testes dos trilhos ---

// Caminho feliz do comentário — o canal que a Fase 3 liga primeiro.
func TestReplyWorker_CommentHappyPath(t *testing.T) {
	h := newReplyWorkerTest(t)
	if err := h.w.ProcessTask(context.Background(), replyTask(t, commentJob(5))); err != nil {
		t.Fatalf("ProcessTask: %v", err)
	}
	if got := h.rep.snapshot(); len(got) != 1 || got[0] != "sent:" {
		t.Fatalf("esperado sent, got %v", got)
	}
	if h.svc.calls != 1 {
		t.Errorf("IA deveria ter sido chamada 1x, got %d", h.svc.calls)
	}
}

// A trava pública contra resposta duplicada: o MESMO evento chegando duas vezes
// (redelivery da Meta) só pode virar UMA resposta no perfil do cliente.
func TestReplyWorker_SameEventRepliesOnlyOnce(t *testing.T) {
	h := newReplyWorkerTest(t)
	ctx := context.Background()
	task := replyTask(t, commentJob(5))

	for i := 0; i < 3; i++ {
		if err := h.w.ProcessTask(ctx, task); err != nil {
			t.Fatalf("entrega %d: %v", i, err)
		}
	}
	if h.svc.calls != 1 {
		t.Fatalf("IA chamada %d vezes — o cliente veria respostas duplicadas", h.svc.calls)
	}
}

// Concorrência real: N executores no mesmo evento, uma resposta só.
func TestReplyWorker_ConcurrentDeliveriesReplyOnce(t *testing.T) {
	h := newReplyWorkerTest(t)
	ctx := context.Background()
	task := replyTask(t, commentJob(5))

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _ = h.w.ProcessTask(ctx, task) }()
	}
	wg.Wait()

	if h.svc.calls != 1 {
		t.Fatalf("sob concorrencia a IA foi chamada %d vezes", h.svc.calls)
	}
}

// Janela de 24h do Direct: regra dura da Meta. Fora dela, nem tenta.
func TestReplyWorker_DMOutsideWindowIsSkipped(t *testing.T) {
	h := newReplyWorkerTest(t)
	old := time.Now().Add(-25 * time.Hour)
	job := InstagramReplyJobData{
		UserID: "user-A", InboundID: "inb-1", ProviderEventID: "MSG-1",
		EventType: models.InstagramEventMessage, SenderID: "VIS-9",
		Content: "oi", OccurredAt: &old,
	}
	if err := h.w.ProcessTask(context.Background(), replyTask(t, job)); err != nil {
		t.Fatalf("ProcessTask: %v", err)
	}
	if got := h.rep.snapshot(); len(got) != 1 || got[0] != "skipped:"+models.InstagramErrWindowExpired {
		t.Fatalf("esperado skipped/window_expired, got %v", got)
	}
	if h.svc.calls != 0 {
		t.Error("nao deveria gastar IA fora da janela")
	}
}

// Dentro da janela, responde normalmente.
func TestReplyWorker_DMInsideWindowIsAnswered(t *testing.T) {
	h := newReplyWorkerTest(t)
	recent := time.Now().Add(-2 * time.Hour)
	job := InstagramReplyJobData{
		UserID: "user-A", InboundID: "inb-1", ProviderEventID: "MSG-2",
		EventType: models.InstagramEventMessage, SenderID: "VIS-9",
		Content: "oi", OccurredAt: &recent,
	}
	if err := h.w.ProcessTask(context.Background(), replyTask(t, job)); err != nil {
		t.Fatal(err)
	}
	if got := h.rep.snapshot(); len(got) != 1 || got[0] != "sent:" {
		t.Fatalf("esperado sent, got %v", got)
	}
}

// Timestamp AUSENTE num Direct = fora da janela. Sem saber quando chegou,
// responder pode violar a regra da Meta — e a punição cai na conta do cliente.
func TestReplyWorker_DMWithoutTimestampIsFailClosed(t *testing.T) {
	h := newReplyWorkerTest(t)
	job := InstagramReplyJobData{
		UserID: "user-A", InboundID: "inb-1", ProviderEventID: "MSG-3",
		EventType: models.InstagramEventMessage, SenderID: "VIS-9", Content: "oi",
	}
	if err := h.w.ProcessTask(context.Background(), replyTask(t, job)); err != nil {
		t.Fatal(err)
	}
	if got := h.rep.snapshot(); len(got) != 1 || got[0] != "skipped:"+models.InstagramErrWindowExpired {
		t.Fatalf("sem timestamp deveria ser fail-closed, got %v", got)
	}
	if h.svc.calls != 0 {
		t.Error("nao deveria chamar a IA sem saber a janela")
	}
}

// Comentário NÃO tem janela — é por isso que é o primeiro canal a ligar.
func TestReplyWorker_CommentHasNoWindow(t *testing.T) {
	h := newReplyWorkerTest(t)
	if err := h.w.ProcessTask(context.Background(), replyTask(t, commentJob(60*24*30))); err != nil {
		t.Fatal(err)
	}
	if got := h.rep.snapshot(); len(got) != 1 || got[0] != "sent:" {
		t.Fatalf("comentario antigo deveria ser respondido, got %v", got)
	}
}

// Cota diária: a chave de IA é NOSSA. Estourou, para de gastar.
func TestReplyWorker_DailyQuotaStopsSpending(t *testing.T) {
	h := newReplyWorkerTest(t)
	h.svc.cap = 3
	h.rep.sentToday = 3

	if err := h.w.ProcessTask(context.Background(), replyTask(t, commentJob(5))); err != nil {
		t.Fatal(err)
	}
	if got := h.rep.snapshot(); len(got) != 1 || got[0] != "skipped:"+models.InstagramErrQuotaDaily {
		t.Fatalf("esperado skipped/quota_daily, got %v", got)
	}
	if h.svc.calls != 0 {
		t.Error("cota estourada nao pode gastar IA")
	}
}

// Cota zero do plano cai no padrão conservador — nunca em "ilimitado".
func TestReplyWorker_ZeroPlanCapFallsBackToDefaultNotUnlimited(t *testing.T) {
	h := newReplyWorkerTest(t)
	h.svc.cap = 0
	h.rep.sentToday = defaultDailyReplyCap

	if err := h.w.ProcessTask(context.Background(), replyTask(t, commentJob(5))); err != nil {
		t.Fatal(err)
	}
	if got := h.rep.snapshot(); len(got) != 1 || got[0] != "skipped:"+models.InstagramErrQuotaDaily {
		t.Fatalf("cota 0 deveria usar o padrao, nao virar ilimitado; got %v", got)
	}
}

// Kill switch global da IA desligado: nem claim, nem IA, nem Meta.
func TestReplyWorker_AIKillSwitchStopsEverything(t *testing.T) {
	h := newReplyWorkerTest(t)
	h.svc.aiOff = true

	if err := h.w.ProcessTask(context.Background(), replyTask(t, commentJob(5))); err != nil {
		t.Fatal(err)
	}
	if len(h.rep.snapshot()) != 0 {
		t.Error("nao deveria nem reservar resposta com a IA desligada")
	}
	if h.svc.calls != 0 {
		t.Error("nao deveria chamar a IA")
	}
}

// Erro de autenticação é terminal com código seguro — e retorna nil, para o
// asynq não arquivar um payload que carrega texto de terceiro.
func TestReplyWorker_AuthErrorIsTerminalAndNotArchived(t *testing.T) {
	h := newReplyWorkerTest(t)
	h.svc.err = instagram.ErrInstagramAuth

	if err := h.w.ProcessTask(context.Background(), replyTask(t, commentJob(5))); err != nil {
		t.Fatalf("deveria retornar nil (sem arquivar), got %v", err)
	}
	if got := h.rep.snapshot(); len(got) != 1 || got[0] != "failed:"+models.InstagramErrAuthRequired {
		t.Fatalf("esperado failed/auth_required, got %v", got)
	}
}

// Falha transitória antes do limite volta como erro (asynq retenta).
func TestReplyWorker_TransientRetriesThenTerminates(t *testing.T) {
	h := newReplyWorkerTest(t)
	h.svc.err = errors.New("banco fora do ar")
	h.w.retryState = func(context.Context) (int, int, bool) { return 0, 3, true }

	if err := h.w.ProcessTask(context.Background(), replyTask(t, commentJob(5))); err == nil {
		t.Fatal("transitorio deveria voltar como erro para retry")
	}

	// Na última tentativa vira terminal e retorna nil.
	h2 := newReplyWorkerTest(t)
	h2.svc.err = errors.New("banco fora do ar")
	h2.w.retryState = func(context.Context) (int, int, bool) { return 3, 3, true }
	if err := h2.w.ProcessTask(context.Background(), replyTask(t, commentJob(5))); err != nil {
		t.Fatalf("ultima tentativa deveria retornar nil, got %v", err)
	}
	if got := h2.rep.snapshot(); len(got) != 1 || got[0] != "failed:"+models.InstagramErrTransientExhausted {
		t.Fatalf("esperado failed/transient_exhausted, got %v", got)
	}
}

// Conteúdo vazio não vira chamada de IA.
func TestReplyWorker_EmptyContentSkips(t *testing.T) {
	h := newReplyWorkerTest(t)
	job := commentJob(5)
	job.Content = ""
	if err := h.w.ProcessTask(context.Background(), replyTask(t, job)); err != nil {
		t.Fatal(err)
	}
	if got := h.rep.snapshot(); len(got) != 1 || got[0] != "skipped:"+models.InstagramErrEmptyContent {
		t.Fatalf("esperado skipped/empty_content, got %v", got)
	}
	if h.svc.calls != 0 {
		t.Error("nao deveria gastar IA com conteudo vazio")
	}
}

// Sem Redis operacional não há lock por conta — fail-closed.
func TestReplyWorker_NoRedisIsFailClosed(t *testing.T) {
	h := newReplyWorkerTest(t)
	h.w.cache = nil
	h.w.retryState = func(context.Context) (int, int, bool) { return 0, 3, true }

	if err := h.w.ProcessTask(context.Background(), replyTask(t, commentJob(5))); err == nil {
		t.Fatal("sem lock deveria falhar transitoriamente, nao responder")
	}
	if h.svc.calls != 0 {
		t.Error("nao deveria responder sem lock")
	}
}

// O TaskID deriva do evento e não carrega PII.
func TestInstagramReplyTaskID_DeterministicAndOpaque(t *testing.T) {
	a := InstagramReplyTaskID("user-A", "COMENT-1")
	if a != InstagramReplyTaskID("user-A", "COMENT-1") {
		t.Fatal("TaskID deveria ser determinístico")
	}
	if a == InstagramReplyTaskID("user-B", "COMENT-1") {
		t.Fatal("usuarios diferentes deveriam gerar ids diferentes")
	}
	for _, leak := range []string{"user-A", "COMENT-1"} {
		if contains(a, leak) {
			t.Errorf("TaskID vaza %q: %s", leak, a)
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
