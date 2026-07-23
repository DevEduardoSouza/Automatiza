package workers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"strconv"
	"strings"
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

// --- fixtures ---

// testJPEG produz um JPEG real e minúsculo. O media store agora VALIDA o
// conteúdo, então bytes falsos não servem mais como fixture.
func testJPEG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("encode fixture jpeg: %v", err)
	}
	return buf.Bytes()
}

// fakeStoryPublisher é thread-safe: alguns testes o exercitam de goroutines.
type fakeStoryPublisher struct {
	mu sync.Mutex

	disabled     bool
	quotaUsed    int
	quotaErr     error
	createErr    error
	publishErr   error
	statusSeq    []string // consumidos em ordem; o último se repete
	statusErr    error
	inFlightPeak int
	inFlight     int

	createCalls  []string // URLs públicas passadas ao EnsureStoryContainer
	publishCalls []string
	statusCalls  int
	quotaCalls   int
}

func (f *fakeStoryPublisher) FeatureEnabled() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return !f.disabled
}

func (f *fakeStoryPublisher) GetPublishingLimit(context.Context, string) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.quotaCalls++
	return f.quotaUsed, f.quotaErr
}

func (f *fakeStoryPublisher) EnsureStoryContainer(_ context.Context, _, publicURL, existing string) (string, error) {
	if existing != "" {
		return existing, nil
	}
	f.mu.Lock()
	f.inFlight++
	if f.inFlight > f.inFlightPeak {
		f.inFlightPeak = f.inFlight
	}
	f.createCalls = append(f.createCalls, publicURL)
	n := len(f.createCalls)
	err := f.createErr
	f.mu.Unlock()

	// Janela larga o bastante para duas tasks da mesma conta se sobreporem, se
	// o lock por conta não estiver funcionando.
	time.Sleep(20 * time.Millisecond)

	f.mu.Lock()
	f.inFlight--
	f.mu.Unlock()
	if err != nil {
		return "", err
	}
	return "container-" + strconv.Itoa(n), nil
}

func (f *fakeStoryPublisher) GetStoryContainerStatus(context.Context, string, string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.statusCalls++
	if f.statusErr != nil {
		return "", f.statusErr
	}
	if len(f.statusSeq) == 0 {
		return "FINISHED", nil
	}
	if f.statusCalls-1 < len(f.statusSeq) {
		return f.statusSeq[f.statusCalls-1], nil
	}
	return f.statusSeq[len(f.statusSeq)-1], nil
}

func (f *fakeStoryPublisher) PublishStoryContainer(_ context.Context, _, containerID string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.publishCalls = append(f.publishCalls, containerID)
	if f.publishErr != nil {
		return "", f.publishErr
	}
	return "media-" + strconv.Itoa(len(f.publishCalls)), nil
}

// fakeStoryRepo é um ledger em memória com o mesmo contrato de concorrência do
// PostgreSQL: TryStartProcessing só entrega o lease a UM chamador.
type fakeStoryRepo struct {
	mu        sync.Mutex
	rows      map[string]*models.InstagramStoryPost
	published int

	results   []string // "ledgerID:status:errorCode"
	releases  int
	saveCalls []string // "ledgerID:containerID"
	getErr    error
	markErr   error
}

func newFakeStoryRepo(rows ...*models.InstagramStoryPost) *fakeStoryRepo {
	r := &fakeStoryRepo{rows: map[string]*models.InstagramStoryPost{}}
	for _, row := range rows {
		r.rows[row.ID] = row
	}
	return r
}

func (f *fakeStoryRepo) GetOrCreate(_ context.Context, p *models.InstagramStoryPost) (*models.InstagramStoryPost, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, row := range f.rows {
		if row.UserID == p.UserID && row.SourceGroup == p.SourceGroup && row.MessageID == p.MessageID {
			return row, false, nil
		}
	}
	cp := *p
	cp.ID = "ledger-" + strconv.Itoa(len(f.rows)+1)
	f.rows[cp.ID] = &cp
	return &cp, true, nil
}

func (f *fakeStoryRepo) GetByID(_ context.Context, id string) (*models.InstagramStoryPost, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.getErr != nil {
		return nil, f.getErr
	}
	row, ok := f.rows[id]
	if !ok {
		return nil, nil
	}
	cp := *row
	return &cp, nil
}

func (f *fakeStoryRepo) TryStartProcessing(_ context.Context, id string, staleBefore time.Time) (bool, time.Time, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	row, ok := f.rows[id]
	if !ok || row.Status != models.InstagramStoryStatusPending {
		return false, time.Time{}, nil
	}
	if row.ProcessingStartedAt != nil && !row.ProcessingStartedAt.Before(staleBefore) {
		return false, time.Time{}, nil
	}
	// Carimbos distintos por reivindicação: é o token de posse do lease.
	now := time.Now().Add(time.Duration(row.AttemptCount) * time.Nanosecond)
	row.ProcessingStartedAt = &now
	row.AttemptCount++
	return true, now, nil
}

// Espelha o guard do PostgreSQL: só libera o lease que ainda for o informado.
func (f *fakeStoryRepo) ReleaseProcessing(_ context.Context, id string, lease time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.releases++
	row, ok := f.rows[id]
	if !ok || row.Status != models.InstagramStoryStatusPending {
		return nil
	}
	if row.ProcessingStartedAt == nil || !row.ProcessingStartedAt.Equal(lease) {
		return nil // lease já é de outro dono
	}
	row.ProcessingStartedAt = nil
	return nil
}

func (f *fakeStoryRepo) SaveContainerID(_ context.Context, id, containerID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.saveCalls = append(f.saveCalls, id+":"+containerID)
	if row, ok := f.rows[id]; ok {
		row.ContainerID = containerID
	}
	return nil
}

func (f *fakeStoryRepo) MarkResult(_ context.Context, id, status, igMediaID, errorCode, safeMessage string, reconciled bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.markErr != nil {
		return f.markErr
	}
	f.results = append(f.results, id+":"+status+":"+errorCode)
	// Espelha o `AND status = 'pending'` do PostgreSQL: desfecho não sobrescreve
	// desfecho.
	if row, ok := f.rows[id]; ok && row.Status == models.InstagramStoryStatusPending {
		row.Status = status
		row.IGMediaID = igMediaID
		row.ErrorCode = errorCode
		row.Error = safeMessage
		row.PublishedReconciled = reconciled
		row.ProcessingStartedAt = nil
	}
	return nil
}

func (f *fakeStoryRepo) ListRecentByUser(context.Context, string, int) ([]models.InstagramStoryPost, error) {
	return nil, nil
}

func (f *fakeStoryRepo) CountPublishedSince(context.Context, string, time.Time) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.published, nil
}

func (f *fakeStoryRepo) DeleteOlderThan(context.Context, time.Time, int) (int64, error) {
	return 0, nil
}

func (f *fakeStoryRepo) snapshotResults() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.results...)
}

// --- harness ---

type storyHarness struct {
	w     *InstagramStoryWorker
	store *instagram.MediaStore
	pub   *fakeStoryPublisher
	repo  *fakeStoryRepo
	mr    *miniredis.Miniredis
	cache *cache.Cache
}

func newStoryWorkerTest(t *testing.T, rows ...*models.InstagramStoryPost) *storyHarness {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	c := cache.NewCache(rdb, "test", false)
	store := instagram.NewMediaStore(rdb, zap.NewNop())
	pub := &fakeStoryPublisher{}
	repo := newFakeStoryRepo(rows...)
	w := NewInstagramStoryWorker(pub, store, repo, c, "https://test.example", zap.NewNop())
	return &storyHarness{w: w, store: store, pub: pub, repo: repo, mr: mr, cache: c}
}

func pendingRow(id, userID string) *models.InstagramStoryPost {
	return &models.InstagramStoryPost{
		ID: id, UserID: userID, SourceGroup: "g@g.us", MessageID: "m-" + id,
		Status: models.InstagramStoryStatusPending, MediaSource: models.InstagramMediaSourceImage,
	}
}

func storyTask(t *testing.T, data InstagramStoryJobData) *asynq.Task {
	t.Helper()
	b, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	return asynq.NewTask(TypeInstagramPublishStory, b)
}

// fixedRetryState força um ponto conhecido da escada de retries.
func fixedRetryState(retried, max int) func(context.Context) (int, int, bool) {
	return func(context.Context) (int, int, bool) { return retried, max, true }
}

// --- testes ---

// Payload legado (só LedgerID + UserID, sem grupo/mensagem) precisa continuar
// funcionando: jobs enfileirados antes desta mudança ainda estão na fila.
func TestInstagramStoryWorker_LegacyPayloadStillWorks(t *testing.T) {
	h := newStoryWorkerTest(t, pendingRow("l-1", "user-A"))
	ctx := context.Background()
	key, err := h.store.Put(ctx, testJPEG(t), time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	if err := h.w.ProcessTask(ctx, storyTask(t, InstagramStoryJobData{
		UserID: "user-A", MediaKey: key, LedgerID: "l-1",
	})); err != nil {
		t.Fatalf("ProcessTask: %v", err)
	}
	if got := h.repo.snapshotResults(); len(got) != 1 || got[0] != "l-1:published:" {
		t.Fatalf("esperado publicação, got %v", got)
	}
}

// Ledger e payload divergentes: o job pode ter sido adulterado ou apontar para
// outra conta. Nada pode sair para a Meta.
func TestInstagramStoryWorker_PayloadLedgerMismatchIsTerminal(t *testing.T) {
	h := newStoryWorkerTest(t, pendingRow("l-1", "user-A"))
	ctx := context.Background()

	if err := h.w.ProcessTask(ctx, storyTask(t, InstagramStoryJobData{
		UserID: "user-INTRUSO", LedgerID: "l-1",
	})); err != nil {
		t.Fatalf("ProcessTask deveria retornar nil, got %v", err)
	}
	got := h.repo.snapshotResults()
	if len(got) != 1 || got[0] != "l-1:failed:"+models.InstagramErrInvalidPayload {
		t.Fatalf("esperado failed/invalid_payload, got %v", got)
	}
	if h.pub.quotaCalls != 0 || len(h.pub.createCalls) != 0 {
		t.Error("não deveria haver NENHUMA chamada à Meta em payload divergente")
	}
}

// Ledger já terminal: reentrega é no-op idempotente.
func TestInstagramStoryWorker_TerminalLedgerIsNoop(t *testing.T) {
	row := pendingRow("l-1", "user-A")
	row.Status = models.InstagramStoryStatusPublished
	h := newStoryWorkerTest(t, row)

	if err := h.w.ProcessTask(context.Background(), storyTask(t, InstagramStoryJobData{
		UserID: "user-A", LedgerID: "l-1",
	})); err != nil {
		t.Fatalf("ProcessTask: %v", err)
	}
	if len(h.repo.snapshotResults()) != 0 || h.pub.quotaCalls != 0 {
		t.Error("ledger terminal não deveria ser reprocessado")
	}
}

// Duas tasks da MESMA conta não podem entrar juntas na Meta: o lock por conta
// serializa. Sem ele, as duas competiriam pela cota.
func TestInstagramStoryWorker_AccountLockSerializesSameAccount(t *testing.T) {
	h := newStoryWorkerTest(t, pendingRow("l-1", "user-A"), pendingRow("l-2", "user-A"))
	ctx := context.Background()
	key, err := h.store.Put(ctx, testJPEG(t), time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for _, id := range []string{"l-1", "l-2"} {
		wg.Add(1)
		go func(ledgerID string) {
			defer wg.Done()
			_ = h.w.ProcessTask(ctx, storyTask(t, InstagramStoryJobData{
				UserID: "user-A", MediaKey: key, LedgerID: ledgerID,
			}))
		}(id)
	}
	wg.Wait()

	h.pub.mu.Lock()
	peak := h.pub.inFlightPeak
	h.pub.mu.Unlock()
	if peak > 1 {
		t.Fatalf("duas publicações da mesma conta entraram juntas na Meta (pico=%d)", peak)
	}
}

// Guard local: acima de 90 publicações em 24h, nem a cota remota é consultada.
func TestInstagramStoryWorker_LocalQuotaIsTerminal(t *testing.T) {
	h := newStoryWorkerTest(t, pendingRow("l-1", "user-A"))
	h.repo.published = storyDailyCap

	if err := h.w.ProcessTask(context.Background(), storyTask(t, InstagramStoryJobData{
		UserID: "user-A", LedgerID: "l-1",
	})); err != nil {
		t.Fatalf("ProcessTask: %v", err)
	}
	got := h.repo.snapshotResults()
	if len(got) != 1 || got[0] != "l-1:failed:"+models.InstagramErrQuotaLocal {
		t.Fatalf("esperado failed/quota_local, got %v", got)
	}
	if h.pub.quotaCalls != 0 || len(h.pub.createCalls) != 0 {
		t.Error("guard local deveria evitar qualquer chamada à Meta")
	}
}

// Cota oficial estourada: terminal, sem criar container.
func TestInstagramStoryWorker_MetaQuotaIsTerminal(t *testing.T) {
	h := newStoryWorkerTest(t, pendingRow("l-1", "user-A"))
	h.pub.quotaUsed = storyMetaQuotaCap

	if err := h.w.ProcessTask(context.Background(), storyTask(t, InstagramStoryJobData{
		UserID: "user-A", LedgerID: "l-1",
	})); err != nil {
		t.Fatalf("ProcessTask: %v", err)
	}
	got := h.repo.snapshotResults()
	if len(got) != 1 || got[0] != "l-1:failed:"+models.InstagramErrQuotaMeta {
		t.Fatalf("esperado failed/quota_meta, got %v", got)
	}
	if len(h.pub.createCalls) != 0 {
		t.Error("não deveria criar container com cota estourada")
	}
}

// Falha AO CONSULTAR a cota é transitória: criar container sem saber a cota é
// justamente o que gera erro caro do lado da Meta.
func TestInstagramStoryWorker_QuotaLookupFailureRetries(t *testing.T) {
	h := newStoryWorkerTest(t, pendingRow("l-1", "user-A"))
	h.pub.quotaErr = instagram.ErrInstagramTransient
	h.w.retryState = fixedRetryState(0, 4)

	err := h.w.ProcessTask(context.Background(), storyTask(t, InstagramStoryJobData{
		UserID: "user-A", LedgerID: "l-1",
	}))
	if err == nil {
		t.Fatal("falha de cota deveria voltar como erro (retry do asynq)")
	}
	if len(h.pub.createCalls) != 0 {
		t.Error("não deveria criar container sem conhecer a cota")
	}
	if len(h.repo.snapshotResults()) != 0 {
		t.Error("falha transitória não deveria marcar o ledger")
	}
}

// O container precisa ser persistido ANTES do polling/publicação — é o que
// impede o retry de criar um segundo e publicar duplicado.
func TestInstagramStoryWorker_ContainerPersistedBeforePublish(t *testing.T) {
	h := newStoryWorkerTest(t, pendingRow("l-1", "user-A"))
	ctx := context.Background()
	key, err := h.store.Put(ctx, testJPEG(t), time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	if err := h.w.ProcessTask(ctx, storyTask(t, InstagramStoryJobData{
		UserID: "user-A", MediaKey: key, LedgerID: "l-1",
	})); err != nil {
		t.Fatalf("ProcessTask: %v", err)
	}
	if len(h.repo.saveCalls) != 1 || h.repo.saveCalls[0] != "l-1:container-1" {
		t.Fatalf("container deveria ter sido persistido, got %v", h.repo.saveCalls)
	}
	if len(h.pub.publishCalls) != 1 || h.pub.publishCalls[0] != "container-1" {
		t.Fatalf("deveria publicar o container persistido, got %v", h.pub.publishCalls)
	}
}

// Retry com container JÁ persistido não pode criar outro — nem depender da
// mídia efêmera, que pode ter expirado nesse meio-tempo.
func TestInstagramStoryWorker_RetryReusesPersistedContainer(t *testing.T) {
	row := pendingRow("l-1", "user-A")
	row.ContainerID = "container-antigo"
	h := newStoryWorkerTest(t, row)

	if err := h.w.ProcessTask(context.Background(), storyTask(t, InstagramStoryJobData{
		UserID: "user-A", MediaKey: "chave-ja-expirada", LedgerID: "l-1",
	})); err != nil {
		t.Fatalf("ProcessTask: %v", err)
	}
	if len(h.pub.createCalls) != 0 {
		t.Fatalf("não deveria criar novo container, got %v", h.pub.createCalls)
	}
	if len(h.pub.publishCalls) != 1 || h.pub.publishCalls[0] != "container-antigo" {
		t.Fatalf("deveria publicar o container existente, got %v", h.pub.publishCalls)
	}
}

// Status PUBLISHED antes de publicarmos = a Meta já publicou num aceite cujo
// retorno se perdeu. Reconciliar, nunca republicar.
func TestInstagramStoryWorker_AlreadyPublishedIsReconciled(t *testing.T) {
	row := pendingRow("l-1", "user-A")
	row.ContainerID = "container-1"
	h := newStoryWorkerTest(t, row)
	h.pub.statusSeq = []string{"PUBLISHED"}

	if err := h.w.ProcessTask(context.Background(), storyTask(t, InstagramStoryJobData{
		UserID: "user-A", LedgerID: "l-1",
	})); err != nil {
		t.Fatalf("ProcessTask: %v", err)
	}
	if len(h.pub.publishCalls) != 0 {
		t.Fatal("NÃO deveria chamar /media_publish para container já publicado")
	}
	got := h.repo.snapshotResults()
	if len(got) != 1 || got[0] != "l-1:published:" {
		t.Fatalf("esperado published, got %v", got)
	}
	if !h.repo.rows["l-1"].PublishedReconciled {
		t.Error("deveria marcar published_reconciled")
	}
}

// Timeout DEPOIS de a Meta aceitar o publish: conferir o status uma vez e
// reconciliar. Sem isso, o retry publicaria o Story de novo.
func TestInstagramStoryWorker_PublishTimeoutReconcilesInsteadOfDuplicating(t *testing.T) {
	row := pendingRow("l-1", "user-A")
	row.ContainerID = "container-1"
	h := newStoryWorkerTest(t, row)
	h.pub.statusSeq = []string{"FINISHED", "PUBLISHED"}
	h.pub.publishErr = instagram.ErrInstagramTransient

	if err := h.w.ProcessTask(context.Background(), storyTask(t, InstagramStoryJobData{
		UserID: "user-A", LedgerID: "l-1",
	})); err != nil {
		t.Fatalf("ProcessTask deveria reconciliar e retornar nil, got %v", err)
	}
	if len(h.pub.createCalls) != 0 {
		t.Error("não deveria criar um segundo container")
	}
	got := h.repo.snapshotResults()
	if len(got) != 1 || got[0] != "l-1:published:" {
		t.Fatalf("esperado published reconciliado, got %v", got)
	}
}

// ERROR e EXPIRED têm códigos terminais próprios.
func TestInstagramStoryWorker_TerminalContainerStatuses(t *testing.T) {
	for status, want := range map[string]string{
		"ERROR":   models.InstagramErrContainerError,
		"EXPIRED": models.InstagramErrContainerExpired,
	} {
		t.Run(status, func(t *testing.T) {
			row := pendingRow("l-1", "user-A")
			row.ContainerID = "container-1"
			h := newStoryWorkerTest(t, row)
			h.pub.statusSeq = []string{status}

			if err := h.w.ProcessTask(context.Background(), storyTask(t, InstagramStoryJobData{
				UserID: "user-A", LedgerID: "l-1",
			})); err != nil {
				t.Fatalf("ProcessTask: %v", err)
			}
			got := h.repo.snapshotResults()
			if len(got) != 1 || got[0] != "l-1:failed:"+want {
				t.Fatalf("esperado failed/%s, got %v", want, got)
			}
			if len(h.pub.publishCalls) != 0 {
				t.Error("não deveria publicar container em estado terminal de erro")
			}
		})
	}
}

// Erro de autenticação é terminal e retorna nil — o job NÃO pode ir para
// arquivados, porque o payload pode conter o texto da mensagem do cliente.
func TestInstagramStoryWorker_AuthErrorIsTerminalAndReturnsNil(t *testing.T) {
	h := newStoryWorkerTest(t, pendingRow("l-1", "user-A"))
	h.pub.quotaErr = instagram.ErrInstagramAuth

	if err := h.w.ProcessTask(context.Background(), storyTask(t, InstagramStoryJobData{
		UserID: "user-A", LedgerID: "l-1",
	})); err != nil {
		t.Fatalf("erro de auth deveria retornar nil (sem arquivar), got %v", err)
	}
	got := h.repo.snapshotResults()
	if len(got) != 1 || got[0] != "l-1:failed:"+models.InstagramErrAuthRequired {
		t.Fatalf("esperado failed/auth_required, got %v", got)
	}
}

// Transitório ANTES da última tentativa: devolve erro (asynq retenta) e LIBERA
// o lease, para o retry poder reivindicar a linha na hora.
func TestInstagramStoryWorker_TransientBeforeLastAttemptReleasesLease(t *testing.T) {
	h := newStoryWorkerTest(t, pendingRow("l-1", "user-A"))
	h.pub.quotaErr = instagram.ErrInstagramTransient
	h.w.retryState = fixedRetryState(1, 4)

	err := h.w.ProcessTask(context.Background(), storyTask(t, InstagramStoryJobData{
		UserID: "user-A", LedgerID: "l-1",
	}))
	if err == nil {
		t.Fatal("transitório antes do limite deveria devolver erro para retry")
	}
	if h.repo.releases != 1 {
		t.Errorf("lease deveria ter sido liberado exatamente 1x, got %d", h.repo.releases)
	}
	if h.repo.rows["l-1"].ProcessingStartedAt != nil {
		t.Error("lease continuou preso após falha transitória")
	}
}

// Última tentativa transitória: vira terminal e retorna nil. Sem isso a linha
// ficaria pending para sempre e sumiria do radar.
func TestInstagramStoryWorker_TransientExhaustedBecomesTerminal(t *testing.T) {
	h := newStoryWorkerTest(t, pendingRow("l-1", "user-A"))
	h.pub.quotaErr = instagram.ErrInstagramTransient
	h.w.retryState = fixedRetryState(4, 4)

	if err := h.w.ProcessTask(context.Background(), storyTask(t, InstagramStoryJobData{
		UserID: "user-A", LedgerID: "l-1",
	})); err != nil {
		t.Fatalf("última tentativa deveria retornar nil (sem arquivar), got %v", err)
	}
	got := h.repo.snapshotResults()
	if len(got) != 1 || got[0] != "l-1:failed:"+models.InstagramErrTransientExhausted {
		t.Fatalf("esperado failed/transient_exhausted, got %v", got)
	}
}

// Kill switch desligado com job já na fila: falha terminal ANTES de qualquer
// chamada à Meta.
func TestInstagramStoryWorker_FeatureDisabledStopsBeforeMeta(t *testing.T) {
	h := newStoryWorkerTest(t, pendingRow("l-1", "user-A"))
	h.pub.disabled = true

	if err := h.w.ProcessTask(context.Background(), storyTask(t, InstagramStoryJobData{
		UserID: "user-A", LedgerID: "l-1",
	})); err != nil {
		t.Fatalf("ProcessTask: %v", err)
	}
	got := h.repo.snapshotResults()
	if len(got) != 1 || got[0] != "l-1:failed:"+models.InstagramErrFeatureDisabled {
		t.Fatalf("esperado failed/feature_disabled, got %v", got)
	}
	if h.pub.quotaCalls != 0 || len(h.pub.createCalls) != 0 {
		t.Error("feature desligada não pode gerar chamada à Meta")
	}
}

// Regressão: quando dois donos DISTINTOS monitoram o mesmo grupo, o trigger
// guarda a imagem UMA vez (MediaKey compartilhada) e enfileira um job por dono
// com a mesma chave. A versão antiga deletava a chave compartilhada no fim do
// 1º job, e o 2º dono falhava com "mídia expirou".
func TestInstagramStoryWorker_SharedImageKeySurvivesMultipleOwners(t *testing.T) {
	h := newStoryWorkerTest(t, pendingRow("l-A", "user-A"), pendingRow("l-B", "user-B"))
	ctx := context.Background()
	sharedKey, err := h.store.Put(ctx, testJPEG(t), time.Hour)
	if err != nil {
		t.Fatalf("put shared media: %v", err)
	}

	for _, o := range []struct{ ledger, user string }{{"l-A", "user-A"}, {"l-B", "user-B"}} {
		if err := h.w.ProcessTask(ctx, storyTask(t, InstagramStoryJobData{
			UserID: o.user, MediaKey: sharedKey, LedgerID: o.ledger,
		})); err != nil {
			t.Fatalf("dono %s: ProcessTask erro: %v", o.user, err)
		}
	}

	if len(h.pub.createCalls) != 2 {
		t.Fatalf("esperado 2 publicações (1 por dono), obtido %d — a chave compartilhada foi deletada", len(h.pub.createCalls))
	}
	for _, u := range h.pub.createCalls {
		if !strings.Contains(u, sharedKey) {
			t.Errorf("URL deveria servir a chave compartilhada %q, got %q", sharedKey, u)
		}
	}
	if _, err := h.store.Get(ctx, sharedKey); err != nil {
		t.Errorf("chave compartilhada foi removida pelo worker (deveria sobreviver por TTL): %v", err)
	}
}

// A imagem gerada a partir do texto é criada pelo próprio worker e DEVE ser
// deletada após a publicação — não pode vazar no Redis.
func TestInstagramStoryWorker_RenderedTextKeyIsCleanedUp(t *testing.T) {
	h := newStoryWorkerTest(t, pendingRow("l-1", "user-A"))

	if err := h.w.ProcessTask(context.Background(), storyTask(t, InstagramStoryJobData{
		UserID: "user-A", Text: "Promoção imperdível hoje", LedgerID: "l-1",
	})); err != nil {
		t.Fatalf("ProcessTask erro: %v", err)
	}
	if len(h.pub.createCalls) != 1 {
		t.Fatalf("esperado 1 publicação, obtido %d", len(h.pub.createCalls))
	}
	for _, k := range h.mr.Keys() {
		if strings.HasPrefix(k, "ig_media:") {
			t.Errorf("chave de mídia própria vazou (deveria ser deletada após publish): %s", k)
		}
	}
}

// Mídia expirada antes de existir container: terminal com código seguro, sem
// publicar e sem derrubar o worker.
func TestInstagramStoryWorker_ExpiredMediaMarksFailed(t *testing.T) {
	h := newStoryWorkerTest(t, pendingRow("l-1", "user-A"))

	if err := h.w.ProcessTask(context.Background(), storyTask(t, InstagramStoryJobData{
		UserID: "user-A", MediaKey: "00000000000000000000000000000000", LedgerID: "l-1",
	})); err != nil {
		t.Fatalf("mídia expirada é terminal e deveria retornar nil, got %v", err)
	}
	if len(h.pub.createCalls) != 0 {
		t.Error("não deveria publicar com mídia expirada")
	}
	got := h.repo.snapshotResults()
	if len(got) != 1 || got[0] != "l-1:failed:"+models.InstagramErrMediaExpired {
		t.Fatalf("esperado failed/media_expired, got %v", got)
	}
}

// O lock precisa ser liberado por compare-and-delete: um DEL cru apagaria o
// lock que OUTRO worker já readquiriu depois de o nosso expirar.
func TestInstagramStoryWorker_LockReleaseDoesNotStealNewerLock(t *testing.T) {
	h := newStoryWorkerTest(t)
	ctx := context.Background()

	release, err := h.w.acquireAccountLock(ctx, "user-A")
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	// Simula: nosso lock expirou e outro executor pegou um novo.
	var lockKey string
	for _, k := range h.mr.Keys() {
		if strings.Contains(k, "ig_story_account_lock:") {
			lockKey = k
		}
	}
	if lockKey == "" {
		t.Fatal("chave de lock não encontrada")
	}
	h.mr.Set(lockKey, "token-de-outro-worker")

	release() // não pode apagar o lock alheio

	if got, err := h.mr.Get(lockKey); err != nil || got != "token-de-outro-worker" {
		t.Fatalf("release apagou o lock de outro worker (got %q, err %v)", got, err)
	}
}

// Sem Redis não há lock; publicar sem serialização é pior que adiar.
func TestInstagramStoryWorker_MissingRedisIsFailClosed(t *testing.T) {
	h := newStoryWorkerTest(t, pendingRow("l-1", "user-A"))
	h.w.cache = nil
	h.w.retryState = fixedRetryState(0, 4)

	err := h.w.ProcessTask(context.Background(), storyTask(t, InstagramStoryJobData{
		UserID: "user-A", LedgerID: "l-1",
	}))
	if err == nil {
		t.Fatal("sem Redis o worker deveria falhar transitoriamente, não publicar")
	}
	if !errors.Is(err, instagram.ErrInstagramTransient) {
		t.Errorf("falha de lock deveria ser transitória, got %v", err)
	}
	if h.pub.quotaCalls != 0 {
		t.Error("não deveria falar com a Meta sem lock")
	}
}

// Cache PRESENTE porém desabilitado (APP_CACHE off) é o caso traiçoeiro: o
// cliente Redis existe, mas SetNX devolve true SEM gravar nada. Se o worker
// olhasse só "cache != nil", o lock viraria um no-op silencioso e duas tasks
// da mesma conta publicariam juntas.
func TestInstagramStoryWorker_DisabledCacheIsFailClosed(t *testing.T) {
	h := newStoryWorkerTest(t, pendingRow("l-1", "user-A"))
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	h.w.cache = cache.NewCache(rdb, "test", true) // disabled=true, redis presente
	h.w.retryState = fixedRetryState(0, 4)

	err := h.w.ProcessTask(context.Background(), storyTask(t, InstagramStoryJobData{
		UserID: "user-A", LedgerID: "l-1",
	}))
	if err == nil {
		t.Fatal("cache desabilitado deveria falhar transitoriamente, não publicar sem lock")
	}
	if !errors.Is(err, instagram.ErrInstagramTransient) {
		t.Errorf("deveria ser transitório, got %v", err)
	}
	if h.pub.quotaCalls != 0 {
		t.Error("não deveria falar com a Meta sem lock efetivo")
	}
}

// Regressão do achado ALTO da revisão: se a gravação do desfecho terminal
// falhar, o worker NÃO pode responder "concluído" ao asynq.
//
// Era o pior desfecho possível: o Story sai publicado no Instagram, o job some
// da fila e a linha fica `pending` para sempre — a auditoria registrando o
// oposto do que aconteceu. O erro precisa voltar para o retry reencontrar o
// container em PUBLISHED e reconciliar.
func TestInstagramStoryWorker_PersistFailureIsRetriedNotSwallowed(t *testing.T) {
	h := newStoryWorkerTest(t, pendingRow("l-1", "user-A"))
	ctx := context.Background()
	key, err := h.store.Put(ctx, testJPEG(t), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	h.repo.markErr = errors.New("postgres fora do ar")
	h.w.retryState = fixedRetryState(0, 4)

	err = h.w.ProcessTask(ctx, storyTask(t, InstagramStoryJobData{
		UserID: "user-A", MediaKey: key, LedgerID: "l-1",
	}))
	if err == nil {
		t.Fatal("falha ao persistir o desfecho deveria voltar como erro (retry), não nil")
	}
	if h.repo.rows["l-1"].Status != models.InstagramStoryStatusPending {
		t.Error("a linha não deveria ter mudado de estado")
	}
}

// Na ÚLTIMA tentativa, porém, devolver erro arquivaria o job — cujo payload
// pode conter o texto da mensagem do cliente. Ali o worker desiste e registra.
func TestInstagramStoryWorker_PersistFailureOnLastAttemptDoesNotArchive(t *testing.T) {
	h := newStoryWorkerTest(t, pendingRow("l-1", "user-A"))
	ctx := context.Background()
	key, err := h.store.Put(ctx, testJPEG(t), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	h.repo.markErr = errors.New("postgres fora do ar")
	h.w.retryState = fixedRetryState(4, 4)

	if err := h.w.ProcessTask(ctx, storyTask(t, InstagramStoryJobData{
		UserID: "user-A", MediaKey: key, LedgerID: "l-1",
	})); err != nil {
		t.Fatalf("última tentativa deveria retornar nil (sem arquivar), got %v", err)
	}
}

// Regressão do achado MÉDIO 3: o lease tem dono. Uma execução zumbi — o asynq
// cancela o contexto no timeout de 90s mas NÃO mata a goroutine — não pode
// liberar o lease que outro worker já retomou legitimamente.
func TestInstagramStoryWorker_ZombieCannotReleaseAnotherLease(t *testing.T) {
	h := newStoryWorkerTest(t, pendingRow("l-1", "user-A"))
	ctx := context.Background()

	// A reivindica; depois B retoma o lease (simulando o lease vencido).
	okA, leaseA, err := h.repo.TryStartProcessing(ctx, "l-1", time.Now().Add(-time.Minute))
	if err != nil || !okA {
		t.Fatalf("A deveria reivindicar: ok=%v err=%v", okA, err)
	}
	okB, leaseB, err := h.repo.TryStartProcessing(ctx, "l-1", time.Now().Add(time.Minute))
	if err != nil || !okB {
		t.Fatalf("B deveria retomar o lease vencido: ok=%v err=%v", okB, err)
	}

	// A acorda e tenta liberar com o token ANTIGO.
	if err := h.repo.ReleaseProcessing(ctx, "l-1", leaseA); err != nil {
		t.Fatal(err)
	}
	got := h.repo.rows["l-1"].ProcessingStartedAt
	if got == nil || !got.Equal(leaseB) {
		t.Fatal("o zumbi liberou o lease de quem está publicando")
	}

	// B libera normalmente.
	if err := h.repo.ReleaseProcessing(ctx, "l-1", leaseB); err != nil {
		t.Fatal(err)
	}
	if h.repo.rows["l-1"].ProcessingStartedAt != nil {
		t.Error("o dono atual deveria conseguir liberar o próprio lease")
	}
}
