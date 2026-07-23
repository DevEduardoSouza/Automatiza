package groupmembers

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"automatiza-go/internal/adapters/whatsapp"
	appconfig "automatiza-go/internal/config"
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// =============================================================================
// Fakes — contam chamadas para provar "zero escrita" / "zero WAHA"
// =============================================================================

type fakeRepo struct {
	mu sync.Mutex

	tenant    *models.TrackingTenant
	tenantErr error

	batches   []models.GroupMemberWebhookEvent
	batchErr  error
	batchRes  models.RecordBatchResult
	batchCall int

	candidates    []models.GroupReconciliationCandidate
	candidatesErr error

	reconciled   []models.GroupReconciliationCandidate
	reconcileErr error

	released []string

	cleanupEvents    int64
	cleanupSnapshots int64
	cleanupCalls     int

	metrics     *models.GroupMemberMetrics
	metricsErr  error
	metricsArgs []metricsCall
}

// metricsCall registra os argumentos de leitura da tela de métricas: o escopo
// (userID) é o que impede uma conta de ler o histórico de outra, então o teste
// precisa poder afirmar o que chegou no repositório.
type metricsCall struct {
	userID  string
	days    int
	groupID string
}

func (f *fakeRepo) ResolveWebhookTenant(_ context.Context, _ string) (*models.TrackingTenant, error) {
	if f.tenantErr != nil {
		return nil, f.tenantErr
	}
	return f.tenant, nil
}

func (f *fakeRepo) RecordWebhookBatch(_ context.Context, _ models.TrackingTenant, e models.GroupMemberWebhookEvent) (models.RecordBatchResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.batchCall++
	if f.batchErr != nil {
		return models.RecordBatchResult{}, f.batchErr
	}
	f.batches = append(f.batches, e)
	res := f.batchRes
	if res.Inserted == 0 && res.Duplicates == 0 {
		res.Inserted = len(e.Participants)
	}
	return res, nil
}

func (f *fakeRepo) ClaimGroupsForReconciliation(_ context.Context, _ int, _ time.Duration) ([]models.GroupReconciliationCandidate, error) {
	return f.candidates, f.candidatesErr
}

func (f *fakeRepo) ReconcileGroup(_ context.Context, c models.GroupReconciliationCandidate, _ []models.GroupReconciliationMember, _ time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.reconcileErr != nil {
		return f.reconcileErr
	}
	f.reconciled = append(f.reconciled, c)
	return nil
}

func (f *fakeRepo) ReleaseReconciliationClaim(_ context.Context, groupID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.released = append(f.released, groupID)
	return nil
}

func (f *fakeRepo) CleanupOldGroupMemberEvents(_ context.Context, _ time.Time, _, _ int) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cleanupCalls++
	return f.cleanupEvents, nil
}

func (f *fakeRepo) CleanupOldGroupMemberSnapshots(_ context.Context, _ time.Time, _, _ int) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cleanupCalls++
	return f.cleanupSnapshots, nil
}

func (f *fakeRepo) GetMemberMetrics(_ context.Context, userID string, days int, groupID string) (*models.GroupMemberMetrics, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.metricsArgs = append(f.metricsArgs, metricsCall{userID: userID, days: days, groupID: groupID})
	if f.metricsErr != nil {
		return nil, f.metricsErr
	}
	return f.metrics, nil
}

func (f *fakeRepo) writeCalls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.batchCall + len(f.reconciled)
}

type fakeFetcher struct {
	mu sync.Mutex

	members []whatsapp.TrackingParticipant
	err     error
	calls   int32

	// gate bloqueia todas as chamadas até ser fechado (mede concorrência real).
	gate       chan struct{}
	inFlight   int32
	maxInFligh int32
}

func (f *fakeFetcher) GetGroupParticipantsForTracking(ctx context.Context, _, _ string) ([]whatsapp.TrackingParticipant, error) {
	atomic.AddInt32(&f.calls, 1)

	cur := atomic.AddInt32(&f.inFlight, 1)
	for {
		max := atomic.LoadInt32(&f.maxInFligh)
		if cur <= max || atomic.CompareAndSwapInt32(&f.maxInFligh, max, cur) {
			break
		}
	}
	defer atomic.AddInt32(&f.inFlight, -1)

	if f.gate != nil {
		select {
		case <-f.gate:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	return f.members, nil
}

// =============================================================================
// Helpers
// =============================================================================

func newTestService(t *testing.T, repo repository.GroupMemberRepository, fetcher ParticipantFetcher, secret string) *Service {
	t.Helper()
	t.Setenv("GROUP_MEMBER_TRACKING_ENABLED", "true")
	appconfig.ResetFeatureFlags()
	t.Cleanup(appconfig.ResetFeatureFlags)

	s := NewService(repo, fetcher, secret, zap.NewNop())
	s.SetClock(func() time.Time { return time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC) })
	return s
}

const validEventID = "evt_01aaaaaaaaaaaaaaaaaaaaaaaa"

const validEnvelopeMs int64 = 1774267200000 // 2026-03-23T00:00:00Z, magnitude ms

func validPayload(participants ...map[string]interface{}) map[string]interface{} {
	list := make([]interface{}, 0, len(participants))
	for _, p := range participants {
		list = append(list, p)
	}
	return map[string]interface{}{
		"type":         "join",
		"timestamp":    float64(1774267200), // segundos, conforme o contrato
		"group":        map[string]interface{}{"id": "120000000000000001@g.us"},
		"participants": list,
		"_data":        map[string]interface{}{},
	}
}

func participant(id, pn, role string) map[string]interface{} {
	p := map[string]interface{}{"id": id, "role": role}
	if pn != "" {
		p["pn"] = pn
	}
	return p
}

func defaultTenant() *models.TrackingTenant {
	return &models.TrackingTenant{UserID: synthUserA, InstancePK: 7, SessionName: "session-a"}
}

func process(t *testing.T, s *Service, payload map[string]interface{}) error {
	t.Helper()
	return s.ProcessGroupEvent(context.Background(), "session-a", EventTypeParticipants,
		validEventID, validEnvelopeMs, payload)
}

// =============================================================================
// Casos 1-2: inércia
// =============================================================================

func TestProcessGroupEvent_FlagOffWritesNothing(t *testing.T) {
	t.Setenv("GROUP_MEMBER_TRACKING_ENABLED", "false")
	appconfig.ResetFeatureFlags()
	t.Cleanup(appconfig.ResetFeatureFlags)

	repo := &fakeRepo{tenant: defaultTenant()}
	fetcher := &fakeFetcher{}
	s := NewService(repo, fetcher, strings.Repeat("s", minSecretLen), zap.NewNop())

	require.NoError(t, process(t, s, validPayload(participant(synthPhoneBR+"@c.us", "", "participant"))))
	require.NoError(t, s.ReconcileMemberships(context.Background()))
	require.NoError(t, s.CleanupTracking(context.Background()))

	assert.Zero(t, repo.writeCalls(), "flag off deve produzir ZERO escrita")
	assert.Zero(t, repo.cleanupCalls)
	assert.Zero(t, atomic.LoadInt32(&fetcher.calls), "flag off deve produzir ZERO chamada WAHA")
}

func TestProcessGroupEvent_ShortSecretIsNoop(t *testing.T) {
	repo := &fakeRepo{tenant: defaultTenant()}
	fetcher := &fakeFetcher{}
	s := newTestService(t, repo, fetcher, "curto-demais")

	require.NoError(t, process(t, s, validPayload(participant(synthPhoneBR+"@c.us", "", "participant"))))
	require.NoError(t, s.ReconcileMemberships(context.Background()))
	require.NoError(t, s.CleanupTracking(context.Background()))

	assert.Zero(t, repo.writeCalls())
	assert.Zero(t, atomic.LoadInt32(&fetcher.calls))
}

// =============================================================================
// Caso 3: eventos legados coexistem sem duplicar contagem
// =============================================================================

func TestProcessGroupEvent_LegacyEventsWriteNothing(t *testing.T) {
	repo := &fakeRepo{tenant: defaultTenant()}
	s := newTestService(t, repo, &fakeFetcher{}, strings.Repeat("s", minSecretLen))

	legacy := []string{"group.join", "group.leave", "group.v2.join", "group.v2.leave", "group.v2.update"}
	for _, evt := range legacy {
		err := s.ProcessGroupEvent(context.Background(), "session-a", evt,
			validEventID, validEnvelopeMs, validPayload(participant(synthPhoneBR+"@c.us", "", "participant")))
		require.NoError(t, err, "evento=%s", evt)
	}

	assert.Zero(t, repo.writeCalls(), "só group.v2.participants gera fato")
}

// =============================================================================
// Casos 4-6: lote, dedupe e participante inválido
// =============================================================================

func TestProcessGroupEvent_TwoParticipantsOneBatch(t *testing.T) {
	repo := &fakeRepo{tenant: defaultTenant()}
	s := newTestService(t, repo, &fakeFetcher{}, strings.Repeat("s", minSecretLen))

	require.NoError(t, process(t, s, validPayload(
		participant(synthPhoneBR+"@c.us", "", "participant"),
		participant(synthLID+"@lid", "", "admin"),
	)))

	require.Equal(t, 1, repo.batchCall, "um envelope = UMA chamada ao repositório")
	require.Len(t, repo.batches, 1)
	assert.Len(t, repo.batches[0].Participants, 2)
	assert.Equal(t, validEventID, repo.batches[0].ProviderEventID)
	assert.Equal(t, validEnvelopeMs, repo.batches[0].ProviderCreatedAt.UnixMilli())
	assert.Equal(t, int64(1774267200), repo.batches[0].OccurredAt.Unix())
	assert.Equal(t, "120000000000000001@g.us", repo.batches[0].WAGroupID)
}

// Guard de regressão do incidente 23/07/2026: o engine GOWS manda o timestamp
// do payload em MILISSEGUNDOS (o exemplo do OpenAPI é em segundos). O parser
// rejeitava essa magnitude e descartou 100% dos eventos reais de produção.
func TestProcessGroupEvent_GowsMillisecondPayloadTimestamp(t *testing.T) {
	repo := &fakeRepo{tenant: defaultTenant()}
	s := newTestService(t, repo, &fakeFetcher{}, strings.Repeat("s", minSecretLen))

	// Valores REAIS capturados da fila de produção (payload 1ms antes do envelope).
	payload := validPayload(participant(synthPhoneBR+"@c.us", "", "participant"))
	payload["type"] = "leave"
	payload["timestamp"] = float64(1784785154370)

	err := s.ProcessGroupEvent(context.Background(), "session-a", EventTypeParticipants,
		"evt_01ky6qrca26ybmdyhrmztq64cr", 1784785154371, payload)
	require.NoError(t, err)

	require.Len(t, repo.batches, 1, "o evento real do GOWS precisa ser ACEITO")
	got := repo.batches[0]
	assert.Equal(t, int64(1784785154370), got.OccurredAt.UnixMilli())
	assert.Equal(t, int64(1784785154371), got.ProviderCreatedAt.UnixMilli())
	assert.Equal(t, models.GroupMemberEventLeave, got.EventType)
	assert.Equal(t, "evt_01ky6qrca26ybmdyhrmztq64cr", got.ProviderEventID)
}

func TestProcessGroupEvent_DedupesRepeatedParticipant(t *testing.T) {
	repo := &fakeRepo{tenant: defaultTenant()}
	s := newTestService(t, repo, &fakeFetcher{}, strings.Repeat("s", minSecretLen))

	// Mesmo id duas vezes (uma com pn) — pn não muda a identidade primária.
	require.NoError(t, process(t, s, validPayload(
		participant(synthLID+"@lid", "", "participant"),
		participant(synthLID+"@lid", synthPhoneBR+"@c.us", "participant"),
	)))

	require.Len(t, repo.batches, 1)
	assert.Len(t, repo.batches[0].Participants, 1, "identidade repetida vira UMA linha")
}

func TestProcessGroupEvent_InvalidParticipantIsSkippedNotFatal(t *testing.T) {
	repo := &fakeRepo{tenant: defaultTenant()}
	s := newTestService(t, repo, &fakeFetcher{}, strings.Repeat("s", minSecretLen))

	require.NoError(t, process(t, s, validPayload(
		participant("lixo-sem-sufixo", "", "participant"),
		participant(synthPhoneBR+"@c.us", "", "participant"),
	)))

	require.Len(t, repo.batches, 1)
	assert.Len(t, repo.batches[0].Participants, 1,
		"participante inválido é pulado; os válidos do mesmo envelope continuam")
}

// =============================================================================
// Casos 7-8: envelope/payload malformado — nil, sem retry, sem escrita
// =============================================================================

func TestProcessGroupEvent_MalformedPayloadWritesNothing(t *testing.T) {
	valid := participant(synthPhoneBR+"@c.us", "", "participant")

	cases := map[string]map[string]interface{}{
		"_data ausente": func() map[string]interface{} {
			p := validPayload(valid)
			delete(p, "_data")
			return p
		}(),
		"_data não-objeto": func() map[string]interface{} {
			p := validPayload(valid)
			p["_data"] = "nope"
			return p
		}(),
		"grupo vazio": func() map[string]interface{} {
			p := validPayload(valid)
			p["group"] = map[string]interface{}{"id": ""}
			return p
		}(),
		"grupo ausente": func() map[string]interface{} {
			p := validPayload(valid)
			delete(p, "group")
			return p
		}(),
		"type fora do enum": func() map[string]interface{} {
			p := validPayload(valid)
			p["type"] = "kick"
			return p
		}(),
		"timestamp negativo": func() map[string]interface{} {
			p := validPayload(valid)
			p["timestamp"] = float64(-1)
			return p
		}(),
		"participants não-array": func() map[string]interface{} {
			p := validPayload(valid)
			p["participants"] = "nope"
			return p
		}(),
		"payload nil": nil,
	}

	for name, payload := range cases {
		t.Run(name, func(t *testing.T) {
			repo := &fakeRepo{tenant: defaultTenant()}
			s := newTestService(t, repo, &fakeFetcher{}, strings.Repeat("s", minSecretLen))

			require.NoError(t, process(t, s, payload), "malformado não deve retentar")
			assert.Zero(t, repo.writeCalls())
		})
	}
}

func TestProcessGroupEvent_InvalidEnvelopeIdentityWritesNothing(t *testing.T) {
	payload := validPayload(participant(synthPhoneBR+"@c.us", "", "participant"))

	cases := []struct {
		name      string
		eventID   string
		timestamp int64
	}{
		{"sem provider id", "", validEnvelopeMs},
		{"provider id de fallback interno", "a1b2:session-a:group.v2.participants:20260723T120000", validEnvelopeMs},
		{"timestamp de envelope em segundos", validEventID, 1774267200},
		{"timestamp de envelope zerado", validEventID, 0},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo := &fakeRepo{tenant: defaultTenant()}
			s := newTestService(t, repo, &fakeFetcher{}, strings.Repeat("s", minSecretLen))

			err := s.ProcessGroupEvent(context.Background(), "session-a", EventTypeParticipants,
				c.eventID, c.timestamp, payload)
			require.NoError(t, err)
			assert.Zero(t, repo.writeCalls())
		})
	}
}

// =============================================================================
// Casos 9-11: tenant e erro de banco
// =============================================================================

func TestProcessGroupEvent_MissingTenantIsRetryable(t *testing.T) {
	repo := &fakeRepo{tenantErr: errors.New("tracking tenant unavailable")}
	s := newTestService(t, repo, &fakeFetcher{}, strings.Repeat("s", minSecretLen))

	err := process(t, s, validPayload(participant(synthPhoneBR+"@c.us", "", "participant")))
	require.Error(t, err)
	// "unavailable" é a substring que o matcher do worker classifica como
	// retryable — a instância pode estar sendo criada agora.
	assert.Contains(t, err.Error(), "tracking tenant unavailable")
	assert.Zero(t, repo.writeCalls())
}

func TestProcessGroupEvent_AmbiguousTenantWritesNothing(t *testing.T) {
	repo := &fakeRepo{tenantErr: repository.ErrAmbiguousTrackingTenant}
	s := newTestService(t, repo, &fakeFetcher{}, strings.Repeat("s", minSecretLen))

	require.NoError(t, process(t, s, validPayload(participant(synthPhoneBR+"@c.us", "", "participant"))),
		"ambiguidade não é retentável: retentar escolheria o tenant errado de novo")
	assert.Zero(t, repo.writeCalls())
}

func TestProcessGroupEvent_DBErrorPropagates(t *testing.T) {
	repo := &fakeRepo{tenant: defaultTenant(), batchErr: errors.New("connection reset by peer")}
	s := newTestService(t, repo, &fakeFetcher{}, strings.Repeat("s", minSecretLen))

	err := process(t, s, validPayload(participant(synthPhoneBR+"@c.us", "", "participant")))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "connection reset")
}

// =============================================================================
// Casos 12-16: reconciliação
// =============================================================================

func candidates(n int) []models.GroupReconciliationCandidate {
	out := make([]models.GroupReconciliationCandidate, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, models.GroupReconciliationCandidate{
			GroupID:     "group-" + string(rune('a'+i)),
			UserID:      synthUserA,
			WAGroupID:   "12000000000000000" + string(rune('0'+i%10)) + "@g.us",
			InstancePK:  7,
			SessionName: "session-a",
		})
	}
	return out
}

func TestReconcile_LIDWithPNKeepsLIDIdentity(t *testing.T) {
	repo := &fakeRepo{candidates: candidates(1)}
	fetcher := &fakeFetcher{members: []whatsapp.TrackingParticipant{
		{ID: synthLID + "@lid", PN: synthPhoneBR + "@c.us", Role: "participant"},
	}}
	s := newTestService(t, repo, fetcher, strings.Repeat("s", minSecretLen))

	require.NoError(t, s.ReconcileMemberships(context.Background()))
	require.Len(t, repo.reconciled, 1)
	assert.Equal(t, int32(1), atomic.LoadInt32(&fetcher.calls), "UMA chamada WAHA por grupo")

	// A identidade tem de bater com a do LID sem pn — é o mesmo membro.
	expected, err := BuildIdentity([]byte(strings.Repeat("s", minSecretLen)), synthUserA, synthLID+"@lid", "")
	require.NoError(t, err)
	withPN, err := BuildIdentity([]byte(strings.Repeat("s", minSecretLen)), synthUserA, synthLID+"@lid", synthPhoneBR+"@c.us")
	require.NoError(t, err)
	assert.Equal(t, expected.ParticipantHash, withPN.ParticipantHash)
	require.NotNil(t, withPN.PhoneHash)
	require.NotNil(t, withPN.DDD)
	assert.Equal(t, "11", *withPN.DDD)
}

func TestReconcile_InvalidItemAbortsWholeGroup(t *testing.T) {
	repo := &fakeRepo{candidates: candidates(1)}
	fetcher := &fakeFetcher{members: []whatsapp.TrackingParticipant{
		{ID: synthPhoneBR + "@c.us", Role: "participant"},
		{ID: "lixo", Role: "participant"}, // inválido
	}}
	s := newTestService(t, repo, fetcher, strings.Repeat("s", minSecretLen))

	err := s.ReconcileMemberships(context.Background())
	require.Error(t, err)
	assert.Empty(t, repo.reconciled,
		"lista parcial desativaria membros legítimos: nenhuma projeção é alterada")
	assert.Equal(t, []string{"group-a"}, repo.released, "claim liberado")
}

func TestReconcile_OneFailureDoesNotCancelOthers(t *testing.T) {
	repo := &fakeRepo{candidates: candidates(3)}
	// O fetcher falha para TODOS; o que se prova aqui é que a rodada percorre
	// os 3 e agrega o erro em vez de abortar no primeiro.
	fetcher := &fakeFetcher{err: errors.New("waha timeout")}
	s := newTestService(t, repo, fetcher, strings.Repeat("s", minSecretLen))

	err := s.ReconcileMemberships(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "3 de 3 grupos falharam")
	assert.Equal(t, int32(3), atomic.LoadInt32(&fetcher.calls), "todos os grupos foram tentados")
	assert.Len(t, repo.released, 3, "todo claim que falhou volta para a fila")
}

func TestReconcile_ConcurrencyIsCapped(t *testing.T) {
	repo := &fakeRepo{candidates: candidates(reconcileConcurrency + 1)}
	gate := make(chan struct{})
	fetcher := &fakeFetcher{gate: gate}
	s := newTestService(t, repo, fetcher, strings.Repeat("s", minSecretLen))

	done := make(chan error, 1)
	go func() { done <- s.ReconcileMemberships(context.Background()) }()

	// Espera o teto encher sem time.Sleep como oráculo: o gate segura todas as
	// chamadas até liberarmos.
	require.Eventually(t, func() bool {
		return atomic.LoadInt32(&fetcher.inFlight) == int32(reconcileConcurrency)
	}, 5*time.Second, time.Millisecond)

	close(gate)
	require.NoError(t, <-done)

	assert.Equal(t, int32(reconcileConcurrency+1), atomic.LoadInt32(&fetcher.calls))
	assert.LessOrEqual(t, atomic.LoadInt32(&fetcher.maxInFligh), int32(reconcileConcurrency),
		"nunca mais que %d chamadas WAHA simultâneas", reconcileConcurrency)
}

func TestReconcile_ContextCancelReleasesClaimsAndLeaksNothing(t *testing.T) {
	repo := &fakeRepo{candidates: candidates(reconcileConcurrency + 5)}
	gate := make(chan struct{})
	fetcher := &fakeFetcher{gate: gate}
	s := newTestService(t, repo, fetcher, strings.Repeat("s", minSecretLen))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.ReconcileMemberships(ctx) }()

	require.Eventually(t, func() bool {
		return atomic.LoadInt32(&fetcher.inFlight) == int32(reconcileConcurrency)
	}, 5*time.Second, time.Millisecond)

	cancel()

	// A rodada TERMINA (nenhuma goroutine fica pendurada) e devolve erro
	// agregado; os claims em voo voltam para a fila com contexto desacoplado.
	select {
	case err := <-done:
		require.Error(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("ReconcileMemberships não retornou após cancelamento: goroutine vazada")
	}

	repo.mu.Lock()
	released := len(repo.released)
	repo.mu.Unlock()
	assert.NotZero(t, released, "claims em voo devem ser liberados no cancelamento")
}

func TestReconcile_NoFetcherIsNoop(t *testing.T) {
	repo := &fakeRepo{candidates: candidates(1)}
	s := newTestService(t, repo, nil, strings.Repeat("s", minSecretLen))

	require.NoError(t, s.ReconcileMemberships(context.Background()),
		"adapter nil não pode causar panic nem erro")
	assert.Empty(t, repo.reconciled)
}

func TestCleanupTracking_RunsBothTables(t *testing.T) {
	repo := &fakeRepo{cleanupEvents: 10, cleanupSnapshots: 3}
	s := newTestService(t, repo, &fakeFetcher{}, strings.Repeat("s", minSecretLen))

	require.NoError(t, s.CleanupTracking(context.Background()))
	assert.Equal(t, 2, repo.cleanupCalls, "ledger e snapshots; projeção nunca")
}
