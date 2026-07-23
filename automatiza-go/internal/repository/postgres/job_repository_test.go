package postgres

import (
	"context"
	"sync"
	"testing"
	"time"

	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/zap"
)

// =============================================================================
// Setup
// =============================================================================

// jobsSchema reproduz as colunas de `jobs` que importam para a dedupe, mais o
// índice da migração 079. `type` NOT NULL e `action_type` nullable existem os
// dois em produção e são sempre gravados com o mesmo valor — o índice usa
// `type` justamente porque NULL não colide com NULL num índice único.
const jobsSchema = `
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS jobs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL,
    type VARCHAR(100) NOT NULL,
    action_type VARCHAR(100),
    status VARCHAR(50) DEFAULT 'pending',
    payload JSONB,
    result JSONB,
    error TEXT,
    error_message TEXT,
    last_error TEXT,
    scheduled_for TIMESTAMPTZ,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    attempts INTEGER DEFAULT 0,
    max_attempts INTEGER DEFAULT 3,
    retry_count INTEGER DEFAULT 0,
    max_retries INTEGER NOT NULL DEFAULT 3,
    retry_after TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_jobs_pending_logical
    ON jobs (user_id, type, scheduled_for, md5(payload::text))
    WHERE status = 'pending';
`

type jobsTestDB struct {
	container testcontainers.Container
	pool      *pgxpool.Pool
	db        *DB
}

func setupJobsTestDB(t *testing.T) *jobsTestDB {
	t.Helper()
	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	require.NoError(t, err, "failed to start postgres container")

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err, "failed to get connection string")

	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err, "failed to create pool")
	require.NoError(t, pool.Ping(ctx), "failed to ping database")

	_, err = pool.Exec(ctx, jobsSchema)
	require.NoError(t, err, "failed to create schema")

	return &jobsTestDB{
		container: pgContainer,
		pool:      pool,
		db:        &DB{pool: pool, logger: zap.NewNop()},
	}
}

func (tdb *jobsTestDB) cleanup(t *testing.T) {
	t.Helper()
	if tdb.pool != nil {
		tdb.pool.Close()
	}
	if tdb.container != nil {
		require.NoError(t, tdb.container.Terminate(context.Background()), "failed to terminate container")
	}
}

func (tdb *jobsTestDB) countJobs(t *testing.T) int {
	t.Helper()
	var n int
	require.NoError(t, tdb.pool.QueryRow(context.Background(), `SELECT count(*) FROM jobs`).Scan(&n))
	return n
}

const jobTestUserID = "11111111-1111-1111-1111-111111111111"

func newTestJob(scheduledFor time.Time, message string, groups ...string) *models.StandaloneJob {
	return &models.StandaloneJob{
		UserID:       jobTestUserID,
		ActionType:   models.JobActionSendMessage,
		Status:       models.StandaloneJobStatusPending,
		ScheduledFor: scheduledFor,
		MaxRetries:   3,
		Payload: models.JobPayload{
			Groups:  groups,
			Message: message,
		},
	}
}

// =============================================================================
// Create — idempotência
// =============================================================================

// O double-click no botão de agendar: o mesmo pedido chega duas vezes. A
// segunda tem que devolver o job original, não criar um segundo envio.
func TestJobRepository_Create_DoubleSubmitReturnsSameJob(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupJobsTestDB(t)
	defer tdb.cleanup(t)

	repo := NewJobRepository(tdb.db, zap.NewNop())
	ctx := context.Background()
	at := time.Now().UTC().Add(time.Hour).Truncate(time.Microsecond)

	first, deduped, err := repo.Create(ctx, newTestJob(at, "oferta do dia", "g1@g.us"))
	require.NoError(t, err)
	require.False(t, deduped, "primeira criação não pode ser marcada como duplicata")

	second, deduped, err := repo.Create(ctx, newTestJob(at, "oferta do dia", "g1@g.us"))
	require.NoError(t, err)

	assert.True(t, deduped, "segunda criação idêntica deve ser sinalizada como duplicata")
	assert.Equal(t, first.ID, second.ID, "duplicata deve devolver o job original")
	assert.Equal(t, 1, tdb.countJobs(t), "não pode existir uma segunda linha")
}

// A chave é o jsonb canônico, não o texto cru: dois clientes que serializam o
// payload com a ordem de chaves diferente descrevem o MESMO agendamento.
func TestJobRepository_Create_KeyOrderDoesNotDefeatDedupe(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupJobsTestDB(t)
	defer tdb.cleanup(t)

	repo := NewJobRepository(tdb.db, zap.NewNop())
	ctx := context.Background()
	at := time.Now().UTC().Add(time.Hour).Truncate(time.Microsecond)

	_, _, err := repo.Create(ctx, newTestJob(at, "promo", "g1@g.us"))
	require.NoError(t, err)

	// Inserção crua com as chaves em ordem diferente da que o Go produz.
	_, err = tdb.pool.Exec(ctx, `
		INSERT INTO jobs (user_id, type, action_type, scheduled_for, status, payload)
		VALUES ($1, 'send_message', 'send_message', $2, 'pending', $3::jsonb)
		ON CONFLICT (user_id, type, scheduled_for, md5(payload::text)) WHERE status = 'pending'
		DO NOTHING
	`, jobTestUserID, at, `{"message":"promo","groups":["g1@g.us"]}`)
	require.NoError(t, err)

	assert.Equal(t, 1, tdb.countJobs(t), "ordem das chaves do JSON não pode furar a dedupe")
}

// Horários distintos são recorrência legítima, não duplicata. Esta é a garantia
// que impede a correção de virar um bug pior que o original.
func TestJobRepository_Create_DistinctSchedulesAreNotDuplicates(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupJobsTestDB(t)
	defer tdb.cleanup(t)

	repo := NewJobRepository(tdb.db, zap.NewNop())
	ctx := context.Background()
	base := time.Now().UTC().Add(time.Hour).Truncate(time.Microsecond)

	for i := range 27 {
		_, deduped, err := repo.Create(ctx, newTestJob(base.AddDate(0, 0, i), "mesma mensagem", "g1@g.us"))
		require.NoError(t, err)
		assert.False(t, deduped, "ocorrência %d de uma recorrência não é duplicata", i)
	}

	assert.Equal(t, 27, tdb.countJobs(t), "as 27 ocorrências futuras devem sobreviver")
}

// Job com desfecho terminal sai do índice parcial: o mesmo conteúdo pode ser
// reagendado depois sem ser confundido com o que já rodou.
func TestJobRepository_Create_DoneJobDoesNotBlockRecreation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupJobsTestDB(t)
	defer tdb.cleanup(t)

	repo := NewJobRepository(tdb.db, zap.NewNop())
	ctx := context.Background()
	at := time.Now().UTC().Add(-time.Hour).Truncate(time.Microsecond)

	first, _, err := repo.Create(ctx, newTestJob(at, "ja enviada", "g1@g.us"))
	require.NoError(t, err)
	require.NoError(t, repo.UpdateStatus(ctx, first.ID, models.StandaloneJobStatusDone, nil))

	second, deduped, err := repo.Create(ctx, newTestJob(at, "ja enviada", "g1@g.us"))
	require.NoError(t, err)

	assert.False(t, deduped)
	assert.NotEqual(t, first.ID, second.ID)
	assert.Equal(t, 2, tdb.countJobs(t))
}

// =============================================================================
// CreateBatch — atomicidade
// =============================================================================

func TestJobRepository_CreateBatch_DedupesWithinBatch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupJobsTestDB(t)
	defer tdb.cleanup(t)

	repo := NewJobRepository(tdb.db, zap.NewNop())
	ctx := context.Background()
	at := time.Now().UTC().Add(time.Hour).Truncate(time.Microsecond)

	created, deduped, err := repo.CreateBatch(ctx, []*models.StandaloneJob{
		newTestJob(at, "oferta", "g1@g.us"),
		newTestJob(at, "oferta", "g1@g.us"), // repetido dentro do MESMO lote
		newTestJob(at, "oferta", "g2@g.us"), // grupo diferente = agendamento diferente
	})
	require.NoError(t, err)

	assert.Len(t, created, 3, "a resposta tem um job por item pedido")
	assert.Equal(t, 1, deduped)
	assert.Equal(t, created[0].ID, created[1].ID, "o item repetido aponta para o job original")
	assert.Equal(t, 2, tdb.countJobs(t))
}

func TestJobRepository_CreateBatch_RollsBackOnFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupJobsTestDB(t)
	defer tdb.cleanup(t)

	repo := NewJobRepository(tdb.db, zap.NewNop())
	ctx := context.Background()
	at := time.Now().UTC().Add(time.Hour).Truncate(time.Microsecond)

	broken := newTestJob(at, "quebrado", "g2@g.us")
	broken.UserID = "nao-e-um-uuid" // o banco recusa no INSERT

	_, _, err := repo.CreateBatch(ctx, []*models.StandaloneJob{
		newTestJob(at, "valido", "g1@g.us"),
		broken,
	})
	require.Error(t, err)

	assert.Equal(t, 0, tdb.countJobs(t), "falha no item 2 não pode deixar o item 1 gravado")
}

// =============================================================================
// Claim — corrida entre ticks/réplicas
// =============================================================================

// A corrida que o desenho antigo permitia: dois consumidores concorrentes não
// podem receber o mesmo job (senão a mensagem sai duas vezes).
func TestJobRepository_ClaimPendingDueJobs_ConcurrentClaimsAreDisjoint(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupJobsTestDB(t)
	defer tdb.cleanup(t)

	repo := NewJobRepository(tdb.db, zap.NewNop())
	ctx := context.Background()

	const total = 20
	past := time.Now().UTC().Add(-time.Hour).Truncate(time.Microsecond)
	for i := range total {
		_, _, err := repo.Create(ctx, newTestJob(past.Add(time.Duration(i)*time.Second), "vencido", "g1@g.us"))
		require.NoError(t, err)
	}

	var wg sync.WaitGroup
	results := make([][]models.StandaloneJob, 2)
	errs := make([]error, 2)
	for i := range 2 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx], errs[idx] = repo.ClaimPendingDueJobs(ctx, total)
		}(i)
	}
	wg.Wait()

	require.NoError(t, errs[0])
	require.NoError(t, errs[1])

	seen := map[string]bool{}
	for _, batch := range results {
		for _, job := range batch {
			assert.False(t, seen[job.ID], "job %s reivindicado duas vezes", job.ID)
			seen[job.ID] = true
			assert.Equal(t, models.StandaloneJobStatusRunning, job.Status, "job reivindicado já sai running")
		}
	}
	assert.Len(t, seen, total, "todo job vencido deve ser reivindicado por exatamente um consumidor")

	var stillPending int
	require.NoError(t, tdb.pool.QueryRow(ctx, `SELECT count(*) FROM jobs WHERE status = 'pending'`).Scan(&stillPending))
	assert.Zero(t, stillPending)
}

// Jobs futuros não são tocados pelo claim.
func TestJobRepository_ClaimPendingDueJobs_IgnoresFutureJobs(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupJobsTestDB(t)
	defer tdb.cleanup(t)

	repo := NewJobRepository(tdb.db, zap.NewNop())
	ctx := context.Background()

	_, _, err := repo.Create(ctx, newTestJob(time.Now().UTC().Add(-time.Hour), "vencido", "g1@g.us"))
	require.NoError(t, err)
	_, _, err = repo.Create(ctx, newTestJob(time.Now().UTC().Add(time.Hour), "futuro", "g1@g.us"))
	require.NoError(t, err)

	claimed, err := repo.ClaimPendingDueJobs(ctx, 50)
	require.NoError(t, err)

	require.Len(t, claimed, 1)
	assert.Equal(t, "vencido", claimed[0].Payload.Message)
}

// Devolução do lote cortado: o job volta para pending e é reivindicável de novo.
func TestJobRepository_ReleaseClaimedJobs_ReturnsJobsToQueue(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupJobsTestDB(t)
	defer tdb.cleanup(t)

	repo := NewJobRepository(tdb.db, zap.NewNop())
	ctx := context.Background()

	_, _, err := repo.Create(ctx, newTestJob(time.Now().UTC().Add(-time.Hour), "vencido", "g1@g.us"))
	require.NoError(t, err)

	claimed, err := repo.ClaimPendingDueJobs(ctx, 50)
	require.NoError(t, err)
	require.Len(t, claimed, 1)

	released, err := repo.ReleaseClaimedJobs(ctx, []string{claimed[0].ID})
	require.NoError(t, err)
	assert.Equal(t, int64(1), released)

	reclaimed, err := repo.ClaimPendingDueJobs(ctx, 50)
	require.NoError(t, err)
	assert.Len(t, reclaimed, 1, "job devolvido tem que voltar a ser reivindicável")
}

// Um release atrasado não pode ressuscitar job que já teve desfecho terminal.
func TestJobRepository_ReleaseClaimedJobs_IgnoresTerminalJobs(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupJobsTestDB(t)
	defer tdb.cleanup(t)

	repo := NewJobRepository(tdb.db, zap.NewNop())
	ctx := context.Background()

	created, _, err := repo.Create(ctx, newTestJob(time.Now().UTC().Add(-time.Hour), "vencido", "g1@g.us"))
	require.NoError(t, err)
	require.NoError(t, repo.UpdateStatus(ctx, created.ID, models.StandaloneJobStatusDone, nil))

	released, err := repo.ReleaseClaimedJobs(ctx, []string{created.ID})
	require.NoError(t, err)
	assert.Zero(t, released, "job done não pode voltar para a fila")
}

// =============================================================================
// Update — conflito legível
// =============================================================================

func TestJobRepository_Update_DuplicateScheduleIsTyped(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupJobsTestDB(t)
	defer tdb.cleanup(t)

	repo := NewJobRepository(tdb.db, zap.NewNop())
	ctx := context.Background()
	slotA := time.Now().UTC().Add(time.Hour).Truncate(time.Microsecond)
	slotB := slotA.Add(time.Hour)

	_, _, err := repo.Create(ctx, newTestJob(slotA, "igual", "g1@g.us"))
	require.NoError(t, err)
	moving, _, err := repo.Create(ctx, newTestJob(slotB, "igual", "g1@g.us"))
	require.NoError(t, err)

	// Mover o segundo para cima do primeiro: mesmo conteúdo, mesmo horário.
	err = repo.Update(ctx, moving.ID, map[string]interface{}{"scheduled_for": slotA})

	assert.ErrorIs(t, err, repository.ErrDuplicatePendingJob,
		"conflito de agendamento tem que virar erro tipado, não 23505 cru")
}
