package postgres

import (
	"context"
	"os"
	"path/filepath"
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

// groupMemberPredecessorSchema cria APENAS as tabelas predecessoras mínimas de
// que a migration 076 depende (FKs e colunas que ela referencia). O schema das
// tabelas novas NÃO é reproduzido aqui de propósito: o teste executa o arquivo
// REAL da migration, senão validaria uma aproximação em vez da migration.
const groupMemberPredecessorSchema = `
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS profiles (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4()
);

CREATE TABLE IF NOT EXISTS instances (
    id SERIAL PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    instance_id VARCHAR(255) NOT NULL,
    status VARCHAR(50) DEFAULT 'disconnected',
    UNIQUE(user_id, instance_id)
);

CREATE TABLE IF NOT EXISTS groups (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    instance_id INTEGER NOT NULL REFERENCES instances(id) ON DELETE CASCADE,
    wa_group_id VARCHAR(255) NOT NULL,
    status VARCHAR(50) DEFAULT 'active',
    participating_instance_ids INTEGER[] NOT NULL DEFAULT '{}',
    members_count INTEGER DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(user_id, wa_group_id)
);
`

// migration076 lê o arquivo real da migration.
func migration076(t *testing.T) string {
	t.Helper()
	path := filepath.Join("..", "..", "..", "scripts", "db", "migrations", "076_group_member_tracking.up.sql")
	data, err := os.ReadFile(path)
	require.NoError(t, err, "migration 076 deve existir em %s", path)
	return string(data)
}

func setupGroupMemberTestDB(t *testing.T) (*DB, repository.GroupMemberRepository) {
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
				WithStartupTimeout(90*time.Second),
		),
	)
	require.NoError(t, err, "failed to start postgres container")

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)
	require.NoError(t, pool.Ping(ctx))

	_, err = pool.Exec(ctx, groupMemberPredecessorSchema)
	require.NoError(t, err, "failed to create predecessor schema")

	_, err = pool.Exec(ctx, migration076(t))
	require.NoError(t, err, "a migration 076 real deve aplicar sobre o schema predecessor")

	t.Cleanup(func() {
		pool.Close()
		_ = pgContainer.Terminate(context.Background())
	})

	db := &DB{pool: pool, logger: zap.NewNop()}
	return db, NewGroupMemberRepository(db, zap.NewNop())
}

// =============================================================================
// Fixtures — identificadores sintéticos (hashes fake, nunca PII)
// =============================================================================

type tenantFixture struct {
	tenant    models.TrackingTenant
	groupID   string
	waGroupID string
}

// hash64 devolve um "hash" sintético de 64 chars a partir de um rótulo curto.
func hash64(label string) string {
	out := make([]byte, 64)
	for i := range out {
		out[i] = '0'
	}
	copy(out, label)
	return string(out)
}

func seedTenant(t *testing.T, db *DB, sessionName, waGroupID string) tenantFixture {
	t.Helper()
	ctx := context.Background()

	var userID string
	require.NoError(t, db.QueryRow(ctx,
		`INSERT INTO profiles DEFAULT VALUES RETURNING id::text`).Scan(&userID))

	var instancePK int
	require.NoError(t, db.QueryRow(ctx,
		`INSERT INTO instances (user_id, instance_id, status) VALUES ($1, $2, 'connected') RETURNING id`,
		userID, sessionName).Scan(&instancePK))

	var groupID string
	require.NoError(t, db.QueryRow(ctx,
		`INSERT INTO groups (user_id, instance_id, wa_group_id, participating_instance_ids)
		 VALUES ($1, $2, $3, ARRAY[$2::int]) RETURNING id::text`,
		userID, instancePK, waGroupID).Scan(&groupID))

	return tenantFixture{
		tenant:    models.TrackingTenant{UserID: userID, InstancePK: instancePK, SessionName: sessionName},
		groupID:   groupID,
		waGroupID: waGroupID,
	}
}

var baseTime = time.Date(2026, 7, 23, 15, 0, 0, 0, time.UTC)

type eventOpt func(*models.GroupMemberWebhookEvent)

func withOccurredAt(ts time.Time) eventOpt {
	return func(e *models.GroupMemberWebhookEvent) { e.OccurredAt = ts }
}
func withProviderCreatedAt(ts time.Time) eventOpt {
	return func(e *models.GroupMemberWebhookEvent) { e.ProviderCreatedAt = ts }
}
func withAppliedAt(ts time.Time) eventOpt {
	return func(e *models.GroupMemberWebhookEvent) { e.AppliedAt = ts }
}

func makeEvent(f tenantFixture, providerEventID, eventType string, participants []models.GroupMemberEventParticipant, opts ...eventOpt) models.GroupMemberWebhookEvent {
	e := models.GroupMemberWebhookEvent{
		ProviderEventID:   providerEventID,
		ProviderCreatedAt: baseTime,
		WAGroupID:         f.waGroupID,
		EventType:         eventType,
		OccurredAt:        baseTime,
		AppliedAt:         baseTime,
		Participants:      participants,
	}
	for _, o := range opts {
		o(&e)
	}
	return e
}

func member(hash, role string) models.GroupMemberEventParticipant {
	return models.GroupMemberEventParticipant{
		GroupMemberIdentity: models.GroupMemberIdentity{
			ParticipantHash: hash64(hash),
			IDType:          models.GroupMemberIDTypeCUs,
		},
		Role: role,
	}
}

// =============================================================================
// Leitura de estado
// =============================================================================

type projectionRow struct {
	isActive        bool
	role            *string
	joinedAt        *time.Time
	leftAt          *time.Time
	firstSeenSource string
	phoneHash       *string
	ddd             *string
	lastEventType   *string
	lastEventID     *string
}

func readProjection(t *testing.T, db *DB, f tenantFixture, hash string) (projectionRow, bool) {
	t.Helper()
	var r projectionRow
	err := db.QueryRow(context.Background(), `
		SELECT is_active, role, joined_at, left_at, first_seen_source,
		       phone_hash, ddd, last_event_type, last_provider_event_id
		FROM group_memberships
		WHERE user_id = $1 AND wa_group_id = $2 AND participant_hash = $3`,
		f.tenant.UserID, f.waGroupID, hash64(hash)).
		Scan(&r.isActive, &r.role, &r.joinedAt, &r.leftAt, &r.firstSeenSource,
			&r.phoneHash, &r.ddd, &r.lastEventType, &r.lastEventID)
	if err != nil {
		return projectionRow{}, false
	}
	return r, true
}

func countLedger(t *testing.T, db *DB, f tenantFixture) int {
	t.Helper()
	var n int
	require.NoError(t, db.QueryRow(context.Background(),
		`SELECT count(*) FROM group_member_events WHERE user_id = $1 AND wa_group_id = $2`,
		f.tenant.UserID, f.waGroupID).Scan(&n))
	return n
}

type snapshotRow struct {
	joins             int
	leaves            int
	membersCount      *int
	membersObservedAt *time.Time
}

func readSnapshot(t *testing.T, db *DB, f tenantFixture, date string) (snapshotRow, bool) {
	t.Helper()
	var s snapshotRow
	err := db.QueryRow(context.Background(), `
		SELECT joins_count, leaves_count, members_count, members_observed_at
		FROM group_member_daily_snapshots
		WHERE user_id = $1 AND wa_group_id = $2 AND snapshot_date = $3::date`,
		f.tenant.UserID, f.waGroupID, date).
		Scan(&s.joins, &s.leaves, &s.membersCount, &s.membersObservedAt)
	if err != nil {
		return snapshotRow{}, false
	}
	return s, true
}

// =============================================================================
// 1-2: a migration REAL aplica, é aditiva e idempotente
// =============================================================================

func TestGroupMemberRepository_MigrationIsIdempotent(t *testing.T) {
	db, _ := setupGroupMemberTestDB(t)

	// Reaplicar a migration inteira não pode falhar (o runner do projeto usa um
	// ledger, mas um replay manual acontece e não pode quebrar o deploy).
	_, err := db.pool.Exec(context.Background(), migration076(t))
	require.NoError(t, err, "a migration 076 deve ser idempotente")

	// Aditiva: nada foi removido do schema predecessor.
	for _, table := range []string{"profiles", "instances", "groups"} {
		var n int
		require.NoError(t, db.QueryRow(context.Background(),
			`SELECT count(*) FROM information_schema.tables WHERE table_name = $1`, table).Scan(&n))
		assert.Equal(t, 1, n, "tabela predecessora %s deve continuar existindo", table)
	}

	// As três colunas de controle entraram em groups.
	for _, col := range []string{"member_tracking_started_at", "members_reconciled_at", "members_reconcile_claimed_at"} {
		var n int
		require.NoError(t, db.QueryRow(context.Background(),
			`SELECT count(*) FROM information_schema.columns WHERE table_name = 'groups' AND column_name = $1`,
			col).Scan(&n))
		assert.Equal(t, 1, n, "coluna %s", col)
	}

	// Os índices declarados no contrato existem.
	for _, idx := range []string{
		"idx_group_member_events_user_group_occurred",
		"idx_group_member_events_user_phone_occurred",
		"idx_group_member_events_occurred_at",
		"idx_group_memberships_active",
		"idx_group_member_daily_snapshots_date",
		"idx_groups_member_reconcile_claim",
	} {
		var n int
		require.NoError(t, db.QueryRow(context.Background(),
			`SELECT count(*) FROM pg_indexes WHERE indexname = $1`, idx).Scan(&n))
		assert.Equal(t, 1, n, "índice %s", idx)
	}
}

func TestGroupMemberRepository_ConstraintsRejectInvalidRows(t *testing.T) {
	db, repo := setupGroupMemberTestDB(t)
	f := seedTenant(t, db, "session-a", "1200000001@g.us")
	ctx := context.Background()

	t.Run("usuário inexistente viola FK", func(t *testing.T) {
		bogus := models.TrackingTenant{
			UserID:      "00000000-0000-4000-8000-000000000000",
			InstancePK:  f.tenant.InstancePK,
			SessionName: "session-a",
		}
		_, err := repo.RecordWebhookBatch(ctx, bogus,
			makeEvent(f, "evt-fk", models.GroupMemberEventJoin, []models.GroupMemberEventParticipant{member("a", "participant")}))
		require.Error(t, err)
	})

	t.Run("event_type fora do enum", func(t *testing.T) {
		_, err := repo.RecordWebhookBatch(ctx, f.tenant,
			makeEvent(f, "evt-enum", "kick", []models.GroupMemberEventParticipant{member("a", "participant")}))
		require.Error(t, err)
	})

	t.Run("role fora do enum", func(t *testing.T) {
		_, err := repo.RecordWebhookBatch(ctx, f.tenant,
			makeEvent(f, "evt-role", models.GroupMemberEventJoin, []models.GroupMemberEventParticipant{member("a", "owner")}))
		require.Error(t, err)
	})

	t.Run("ddd malformado", func(t *testing.T) {
		bad := member("a", "participant")
		invalid := "XX"
		bad.DDD = &invalid
		_, err := repo.RecordWebhookBatch(ctx, f.tenant,
			makeEvent(f, "evt-ddd", models.GroupMemberEventJoin, []models.GroupMemberEventParticipant{bad}))
		require.Error(t, err)
	})

	t.Run("id_type fora do enum", func(t *testing.T) {
		bad := member("a", "participant")
		bad.IDType = "jid"
		_, err := repo.RecordWebhookBatch(ctx, f.tenant,
			makeEvent(f, "evt-idtype", models.GroupMemberEventJoin, []models.GroupMemberEventParticipant{bad}))
		require.Error(t, err)
	})

	assert.Zero(t, countLedger(t, db, f), "nenhuma linha inválida pode ter entrado")
}

// =============================================================================
// 3-5: idempotência de redelivery e ordem do array
// =============================================================================

func TestGroupMemberRepository_ConcurrentSameEnvelopeInsertsOnce(t *testing.T) {
	db, repo := setupGroupMemberTestDB(t)
	f := seedTenant(t, db, "session-a", "1200000001@g.us")

	event := makeEvent(f, "evt-concurrent", models.GroupMemberEventJoin,
		[]models.GroupMemberEventParticipant{member("a", "participant"), member("b", "participant")})

	const workers = 2
	start := make(chan struct{})
	var wg sync.WaitGroup
	results := make([]models.RecordBatchResult, workers)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start
			res, err := repo.RecordWebhookBatch(context.Background(), f.tenant, event)
			assert.NoError(t, err)
			results[idx] = res
		}(i)
	}
	close(start)
	wg.Wait()

	assert.Equal(t, 2, countLedger(t, db, f), "2 participantes = 2 linhas, nunca 4")

	total := 0
	for _, r := range results {
		total += r.Inserted
	}
	assert.Equal(t, 2, total, "só um dos workers pode ter inserido")

	snap, ok := readSnapshot(t, db, f, "2026-07-23")
	require.True(t, ok)
	assert.Equal(t, 2, snap.joins, "a contagem diária não pode dobrar com o replay")
}

func TestGroupMemberRepository_ReplayWithReorderedArrayIsDuplicate(t *testing.T) {
	db, repo := setupGroupMemberTestDB(t)
	f := seedTenant(t, db, "session-a", "1200000001@g.us")
	ctx := context.Background()

	a, b := member("a", "participant"), member("b", "participant")

	first, err := repo.RecordWebhookBatch(ctx, f.tenant,
		makeEvent(f, "evt-order", models.GroupMemberEventJoin, []models.GroupMemberEventParticipant{a, b}))
	require.NoError(t, err)
	assert.Equal(t, 2, first.Inserted)

	// Mesmo envelope, ordem invertida: a ordem do array não tem semântica.
	second, err := repo.RecordWebhookBatch(ctx, f.tenant,
		makeEvent(f, "evt-order", models.GroupMemberEventJoin, []models.GroupMemberEventParticipant{b, a}))
	require.NoError(t, err)
	assert.Zero(t, second.Inserted)
	assert.Equal(t, 2, second.Duplicates)

	assert.Equal(t, 2, countLedger(t, db, f))
	snap, _ := readSnapshot(t, db, f, "2026-07-23")
	assert.Equal(t, 2, snap.joins)
}

func TestGroupMemberRepository_SameSecondDistinctIDsAreTwoFacts(t *testing.T) {
	db, repo := setupGroupMemberTestDB(t)
	f := seedTenant(t, db, "session-a", "1200000001@g.us")
	ctx := context.Background()

	p := []models.GroupMemberEventParticipant{member("a", "participant")}

	// Dois fatos LEGÍTIMOS do mesmo tipo, mesmo participante, mesmo segundo:
	// só o ULID do envelope os distingue. Uma impressão por tipo+timestamp
	// teria colapsado os dois num só.
	_, err := repo.RecordWebhookBatch(ctx, f.tenant, makeEvent(f, "evt-1", models.GroupMemberEventJoin, p))
	require.NoError(t, err)
	_, err = repo.RecordWebhookBatch(ctx, f.tenant, makeEvent(f, "evt-2", models.GroupMemberEventJoin, p))
	require.NoError(t, err)

	assert.Equal(t, 2, countLedger(t, db, f))
}

// =============================================================================
// 6-10: ordem total, reentrada e promote/demote
// =============================================================================

func TestGroupMemberRepository_JoinThenLeave(t *testing.T) {
	db, repo := setupGroupMemberTestDB(t)
	f := seedTenant(t, db, "session-a", "1200000001@g.us")
	ctx := context.Background()

	p := []models.GroupMemberEventParticipant{member("a", "participant")}
	joinAt := baseTime
	leaveAt := baseTime.Add(time.Hour)

	_, err := repo.RecordWebhookBatch(ctx, f.tenant,
		makeEvent(f, "evt-join", models.GroupMemberEventJoin, p, withOccurredAt(joinAt)))
	require.NoError(t, err)

	_, err = repo.RecordWebhookBatch(ctx, f.tenant,
		makeEvent(f, "evt-leave", models.GroupMemberEventLeave, p,
			withOccurredAt(leaveAt), withProviderCreatedAt(leaveAt), withAppliedAt(leaveAt)))
	require.NoError(t, err)

	row, ok := readProjection(t, db, f, "a")
	require.True(t, ok)
	assert.False(t, row.isActive)
	require.NotNil(t, row.leftAt)
	assert.WithinDuration(t, leaveAt, *row.leftAt, time.Second)
	require.NotNil(t, row.joinedAt, "leave preserva o joined_at da permanência encerrada")
	assert.WithinDuration(t, joinAt, *row.joinedAt, time.Second)
	assert.Equal(t, models.GroupMemberSourceWebhook, row.firstSeenSource)
}

func TestGroupMemberRepository_LateEventGrowsLedgerButNotProjection(t *testing.T) {
	db, repo := setupGroupMemberTestDB(t)
	f := seedTenant(t, db, "session-a", "1200000001@g.us")
	ctx := context.Background()

	p := []models.GroupMemberEventParticipant{member("a", "participant")}
	recent := baseTime.Add(time.Hour)
	old := baseTime.Add(-time.Hour)

	_, err := repo.RecordWebhookBatch(ctx, f.tenant,
		makeEvent(f, "evt-recent", models.GroupMemberEventJoin, p,
			withOccurredAt(recent), withProviderCreatedAt(recent), withAppliedAt(recent)))
	require.NoError(t, err)

	// Leave ATRASADO (aconteceu antes, chegou depois): entra no ledger como
	// fato, mas não pode retroceder a projeção para "saiu".
	_, err = repo.RecordWebhookBatch(ctx, f.tenant,
		makeEvent(f, "evt-late", models.GroupMemberEventLeave, p,
			withOccurredAt(old), withProviderCreatedAt(old), withAppliedAt(recent.Add(time.Minute))))
	require.NoError(t, err)

	assert.Equal(t, 2, countLedger(t, db, f), "todo fato entra no ledger")

	row, ok := readProjection(t, db, f, "a")
	require.True(t, ok)
	assert.True(t, row.isActive, "evento atrasado não retrocede a projeção")
	require.NotNil(t, row.lastEventID)
	assert.Equal(t, "evt-recent", *row.lastEventID)
}

func TestGroupMemberRepository_TieBreakByProviderCreatedAt(t *testing.T) {
	db, repo := setupGroupMemberTestDB(t)
	f := seedTenant(t, db, "session-a", "1200000001@g.us")
	ctx := context.Background()

	p := []models.GroupMemberEventParticipant{member("a", "participant")}

	// occurred_at IDÊNTICO nos dois; vence o maior provider_created_at.
	_, err := repo.RecordWebhookBatch(ctx, f.tenant,
		makeEvent(f, "evt-b", models.GroupMemberEventLeave, p,
			withProviderCreatedAt(baseTime.Add(2*time.Second))))
	require.NoError(t, err)

	_, err = repo.RecordWebhookBatch(ctx, f.tenant,
		makeEvent(f, "evt-a", models.GroupMemberEventJoin, p,
			withProviderCreatedAt(baseTime.Add(time.Second))))
	require.NoError(t, err)

	row, ok := readProjection(t, db, f, "a")
	require.True(t, ok)
	assert.False(t, row.isActive, "o envelope com provider_created_at maior vence")
	require.NotNil(t, row.lastEventID)
	assert.Equal(t, "evt-b", *row.lastEventID)
}

func TestGroupMemberRepository_TieBreakByProviderEventID(t *testing.T) {
	db, repo := setupGroupMemberTestDB(t)
	f := seedTenant(t, db, "session-a", "1200000001@g.us")
	ctx := context.Background()

	p := []models.GroupMemberEventParticipant{member("a", "participant")}

	// Empate nos DOIS timestamps: só o ID desempata — e o resultado tem de ser
	// o mesmo independentemente da ordem de commit.
	_, err := repo.RecordWebhookBatch(ctx, f.tenant, makeEvent(f, "evt-zzz", models.GroupMemberEventLeave, p))
	require.NoError(t, err)
	_, err = repo.RecordWebhookBatch(ctx, f.tenant, makeEvent(f, "evt-aaa", models.GroupMemberEventJoin, p))
	require.NoError(t, err)

	row, ok := readProjection(t, db, f, "a")
	require.True(t, ok)
	require.NotNil(t, row.lastEventID)
	assert.Equal(t, "evt-zzz", *row.lastEventID, "ordem determinística pelo maior ID")
	assert.False(t, row.isActive)
}

func TestGroupMemberRepository_RejoinAfterLeave(t *testing.T) {
	db, repo := setupGroupMemberTestDB(t)
	f := seedTenant(t, db, "session-a", "1200000001@g.us")
	ctx := context.Background()

	p := []models.GroupMemberEventParticipant{member("a", "participant")}
	t1 := baseTime
	t2 := baseTime.Add(time.Hour)
	t3 := baseTime.Add(2 * time.Hour)

	for _, step := range []struct {
		id        string
		eventType string
		ts        time.Time
	}{
		{"evt-1", models.GroupMemberEventJoin, t1},
		{"evt-2", models.GroupMemberEventLeave, t2},
		{"evt-3", models.GroupMemberEventJoin, t3},
	} {
		_, err := repo.RecordWebhookBatch(ctx, f.tenant,
			makeEvent(f, step.id, step.eventType, p,
				withOccurredAt(step.ts), withProviderCreatedAt(step.ts), withAppliedAt(step.ts)))
		require.NoError(t, err)
	}

	row, ok := readProjection(t, db, f, "a")
	require.True(t, ok)
	assert.True(t, row.isActive)
	assert.Nil(t, row.leftAt, "reentrada limpa a saída anterior")
	require.NotNil(t, row.joinedAt)
	assert.WithinDuration(t, t3, *row.joinedAt, time.Second, "joined_at é o da NOVA permanência")

	snap, _ := readSnapshot(t, db, f, "2026-07-23")
	assert.Equal(t, 2, snap.joins)
	assert.Equal(t, 1, snap.leaves)
}

func TestGroupMemberRepository_PromoteOnlyChangesRole(t *testing.T) {
	db, repo := setupGroupMemberTestDB(t)
	f := seedTenant(t, db, "session-a", "1200000001@g.us")
	ctx := context.Background()

	joinAt := baseTime
	promoteAt := baseTime.Add(time.Hour)

	_, err := repo.RecordWebhookBatch(ctx, f.tenant,
		makeEvent(f, "evt-join", models.GroupMemberEventJoin,
			[]models.GroupMemberEventParticipant{member("a", "participant")}, withOccurredAt(joinAt)))
	require.NoError(t, err)

	_, err = repo.RecordWebhookBatch(ctx, f.tenant,
		makeEvent(f, "evt-promote", models.GroupMemberEventPromote,
			[]models.GroupMemberEventParticipant{member("a", "admin")},
			withOccurredAt(promoteAt), withProviderCreatedAt(promoteAt), withAppliedAt(promoteAt)))
	require.NoError(t, err)

	row, ok := readProjection(t, db, f, "a")
	require.True(t, ok)
	require.NotNil(t, row.role)
	assert.Equal(t, "admin", *row.role)
	assert.True(t, row.isActive, "promote não movimenta membro")
	require.NotNil(t, row.joinedAt)
	assert.WithinDuration(t, joinAt, *row.joinedAt, time.Second)
	assert.Nil(t, row.leftAt)
	require.NotNil(t, row.lastEventID)
	assert.Equal(t, "evt-promote", *row.lastEventID)

	// promote/demote não entram na contagem de entradas/saídas.
	snap, _ := readSnapshot(t, db, f, "2026-07-23")
	assert.Equal(t, 1, snap.joins)
	assert.Equal(t, 0, snap.leaves)
}

func TestGroupMemberRepository_PromoteWithoutProjectionDoesNotInventMember(t *testing.T) {
	db, repo := setupGroupMemberTestDB(t)
	f := seedTenant(t, db, "session-a", "1200000001@g.us")

	res, err := repo.RecordWebhookBatch(context.Background(), f.tenant,
		makeEvent(f, "evt-promote", models.GroupMemberEventPromote,
			[]models.GroupMemberEventParticipant{member("ghost", "admin")}))
	require.NoError(t, err)
	assert.Equal(t, 1, res.Inserted, "o fato é real e entra no ledger")

	_, ok := readProjection(t, db, f, "ghost")
	assert.False(t, ok, "promote não pode CRIAR um membro que nunca vimos entrar")
}

// =============================================================================
// 13: atomicidade
// =============================================================================

func TestGroupMemberRepository_FailedBatchLeavesNothingBehind(t *testing.T) {
	db, repo := setupGroupMemberTestDB(t)
	f := seedTenant(t, db, "session-a", "1200000001@g.us")

	good := member("a", "participant")
	bad := member("b", "participant")
	invalid := "ZZ" // viola o CHECK de ddd
	bad.DDD = &invalid

	_, err := repo.RecordWebhookBatch(context.Background(), f.tenant,
		makeEvent(f, "evt-atomic", models.GroupMemberEventJoin,
			[]models.GroupMemberEventParticipant{good, bad}))
	require.Error(t, err)

	// O participante VÁLIDO do mesmo envelope também não pode ter sobrado.
	assert.Zero(t, countLedger(t, db, f), "ledger")
	_, ok := readProjection(t, db, f, "a")
	assert.False(t, ok, "projeção")
	_, ok = readSnapshot(t, db, f, "2026-07-23")
	assert.False(t, ok, "snapshot")
}

// =============================================================================
// 14-17: reconciliação
// =============================================================================

func reconMember(hash, role string) models.GroupReconciliationMember {
	return models.GroupReconciliationMember{
		GroupMemberIdentity: models.GroupMemberIdentity{
			ParticipantHash: hash64(hash),
			IDType:          models.GroupMemberIDTypeCUs,
		},
		Role: role,
	}
}

func candidateOf(f tenantFixture) models.GroupReconciliationCandidate {
	return models.GroupReconciliationCandidate{
		GroupID:     f.groupID,
		UserID:      f.tenant.UserID,
		WAGroupID:   f.waGroupID,
		InstancePK:  f.tenant.InstancePK,
		SessionName: f.tenant.SessionName,
	}
}

func TestGroupMemberRepository_ReconciliationBaselineDoesNotInventJoin(t *testing.T) {
	db, repo := setupGroupMemberTestDB(t)
	f := seedTenant(t, db, "session-a", "1200000001@g.us")

	require.NoError(t, repo.ReconcileGroup(context.Background(), candidateOf(f),
		[]models.GroupReconciliationMember{reconMember("a", "participant")}, baseTime))

	row, ok := readProjection(t, db, f, "a")
	require.True(t, ok)
	assert.True(t, row.isActive)
	assert.Nil(t, row.joinedAt, "não observamos QUANDO entrou: joined_at fica NULL")
	assert.Equal(t, models.GroupMemberSourceReconciliation, row.firstSeenSource)

	assert.Zero(t, countLedger(t, db, f), "reconciliação NUNCA escreve no ledger")
}

func TestGroupMemberRepository_AbsentMemberIsDeactivatedWithoutLeftAt(t *testing.T) {
	db, repo := setupGroupMemberTestDB(t)
	f := seedTenant(t, db, "session-a", "1200000001@g.us")
	ctx := context.Background()

	require.NoError(t, repo.ReconcileGroup(ctx, candidateOf(f),
		[]models.GroupReconciliationMember{reconMember("a", "participant"), reconMember("b", "participant")}, baseTime))

	// Segunda foto sem "b".
	require.NoError(t, repo.ReconcileGroup(ctx, candidateOf(f),
		[]models.GroupReconciliationMember{reconMember("a", "participant")}, baseTime.Add(time.Hour)))

	row, ok := readProjection(t, db, f, "b")
	require.True(t, ok)
	assert.False(t, row.isActive)
	assert.Nil(t, row.leftAt, "não observamos QUANDO saiu: left_at fica NULL")
}

func TestGroupMemberRepository_EmptyResponseDeactivatesEveryone(t *testing.T) {
	db, repo := setupGroupMemberTestDB(t)
	f := seedTenant(t, db, "session-a", "1200000001@g.us")
	ctx := context.Background()

	require.NoError(t, repo.ReconcileGroup(ctx, candidateOf(f),
		[]models.GroupReconciliationMember{reconMember("a", "participant")}, baseTime))

	// Resposta VÁLIDA vazia é legítima: o grupo esvaziou.
	require.NoError(t, repo.ReconcileGroup(ctx, candidateOf(f),
		nil, baseTime.Add(time.Hour)))

	row, ok := readProjection(t, db, f, "a")
	require.True(t, ok)
	assert.False(t, row.isActive)

	snap, ok := readSnapshot(t, db, f, "2026-07-23")
	require.True(t, ok)
	require.NotNil(t, snap.membersCount)
	assert.Equal(t, 0, *snap.membersCount, "zero É válido depois do baseline")
}

func TestGroupMemberRepository_ConcurrentWebhookSurvivesStaleReconciliation(t *testing.T) {
	db, repo := setupGroupMemberTestDB(t)
	f := seedTenant(t, db, "session-a", "1200000001@g.us")
	ctx := context.Background()

	observationStartedAt := baseTime

	// O webhook é aplicado DURANTE a chamada WAHA (applied_at posterior ao
	// início da observação). A fotografia é potencialmente anterior a ele.
	_, err := repo.RecordWebhookBatch(ctx, f.tenant,
		makeEvent(f, "evt-race", models.GroupMemberEventJoin,
			[]models.GroupMemberEventParticipant{member("novato", "participant")},
			withAppliedAt(observationStartedAt.Add(2*time.Second))))
	require.NoError(t, err)

	// A foto (sem o novato) chega depois e NÃO pode desfazer a entrada.
	require.NoError(t, repo.ReconcileGroup(ctx, candidateOf(f),
		[]models.GroupReconciliationMember{reconMember("antigo", "participant")}, observationStartedAt))

	row, ok := readProjection(t, db, f, "novato")
	require.True(t, ok)
	assert.True(t, row.isActive, "webhook concorrente vence a foto mais velha")

	// O membro que a foto viu entrou normalmente.
	old, ok := readProjection(t, db, f, "antigo")
	require.True(t, ok)
	assert.True(t, old.isActive)
}

func TestGroupMemberRepository_ReconciliationEnrichesWithoutSplittingIdentity(t *testing.T) {
	db, repo := setupGroupMemberTestDB(t)
	f := seedTenant(t, db, "session-a", "1200000001@g.us")
	ctx := context.Background()

	// Baseline de um @lid SEM telefone.
	lid := models.GroupReconciliationMember{
		GroupMemberIdentity: models.GroupMemberIdentity{
			ParticipantHash: hash64("lid"),
			IDType:          models.GroupMemberIDTypeLID,
		},
		Role: "participant",
	}
	require.NoError(t, repo.ReconcileGroup(ctx, candidateOf(f),
		[]models.GroupReconciliationMember{lid}, baseTime))

	row, ok := readProjection(t, db, f, "lid")
	require.True(t, ok)
	assert.Nil(t, row.phoneHash)

	// Reconciliação posterior traz o pn do MESMO id: a identidade primária não
	// muda, então a projeção é enriquecida em vez de duplicada.
	phone := hash64("phone")
	ddd := "11"
	enriched := lid
	enriched.PhoneHash = &phone
	enriched.DDD = &ddd
	require.NoError(t, repo.ReconcileGroup(ctx, candidateOf(f),
		[]models.GroupReconciliationMember{enriched}, baseTime.Add(time.Hour)))

	var n int
	require.NoError(t, db.QueryRow(ctx,
		`SELECT count(*) FROM group_memberships WHERE user_id = $1 AND wa_group_id = $2`,
		f.tenant.UserID, f.waGroupID).Scan(&n))
	assert.Equal(t, 1, n, "uma projeção, não duas")

	row, _ = readProjection(t, db, f, "lid")
	require.NotNil(t, row.phoneHash)
	require.NotNil(t, row.ddd)
	assert.Equal(t, "11", *row.ddd)
}

// =============================================================================
// 18-20: timezone, evento atrasado e semântica de NULL/zero
// =============================================================================

func TestGroupMemberRepository_SnapshotBucketUsesSaoPaulo(t *testing.T) {
	db, repo := setupGroupMemberTestDB(t)
	f := seedTenant(t, db, "session-a", "1200000001@g.us")

	// 2026-07-24T02:30:00Z = 2026-07-23 23:30 em São Paulo (UTC-3).
	lateNight := time.Date(2026, 7, 24, 2, 30, 0, 0, time.UTC)

	_, err := repo.RecordWebhookBatch(context.Background(), f.tenant,
		makeEvent(f, "evt-tz", models.GroupMemberEventJoin,
			[]models.GroupMemberEventParticipant{member("a", "participant")},
			withOccurredAt(lateNight), withProviderCreatedAt(lateNight), withAppliedAt(lateNight)))
	require.NoError(t, err)

	snap, ok := readSnapshot(t, db, f, "2026-07-23")
	require.True(t, ok, "o fato pertence ao dia 23 do usuário, não ao 24 em UTC")
	assert.Equal(t, 1, snap.joins)

	_, ok = readSnapshot(t, db, f, "2026-07-24")
	assert.False(t, ok)
}

func TestGroupMemberRepository_LateEventCountsOnItsHistoricalDate(t *testing.T) {
	db, repo := setupGroupMemberTestDB(t)
	f := seedTenant(t, db, "session-a", "1200000001@g.us")
	ctx := context.Background()

	// Fato de 3 dias atrás, aplicado hoje: a contagem vai para a data correta.
	old := time.Date(2026, 7, 20, 15, 0, 0, 0, time.UTC)

	_, err := repo.RecordWebhookBatch(ctx, f.tenant,
		makeEvent(f, "evt-old", models.GroupMemberEventJoin,
			[]models.GroupMemberEventParticipant{member("a", "participant")},
			withOccurredAt(old), withProviderCreatedAt(old), withAppliedAt(baseTime)))
	require.NoError(t, err)

	snap, ok := readSnapshot(t, db, f, "2026-07-20")
	require.True(t, ok, "o dia histórico correto é atualizado mesmo depois de terminado")
	assert.Equal(t, 1, snap.joins)

	_, ok = readSnapshot(t, db, f, "2026-07-23")
	assert.False(t, ok, "o dia da aplicação não recebe a contagem")
}

func TestGroupMemberRepository_MembersCountNullBeforeBaseline(t *testing.T) {
	db, repo := setupGroupMemberTestDB(t)
	f := seedTenant(t, db, "session-a", "1200000001@g.us")
	ctx := context.Background()

	_, err := repo.RecordWebhookBatch(ctx, f.tenant,
		makeEvent(f, "evt-pre", models.GroupMemberEventJoin,
			[]models.GroupMemberEventParticipant{member("a", "participant")}))
	require.NoError(t, err)

	snap, ok := readSnapshot(t, db, f, "2026-07-23")
	require.True(t, ok)
	assert.Equal(t, 1, snap.joins)
	assert.Nil(t, snap.membersCount,
		"sem baseline a contagem seria um número falso: fica NULL, não zero")
	assert.Nil(t, snap.membersObservedAt)

	// Depois do baseline a observação passa a existir, com carimbo de tempo.
	observedAt := baseTime.Add(time.Hour)
	require.NoError(t, repo.ReconcileGroup(ctx, candidateOf(f),
		[]models.GroupReconciliationMember{reconMember("a", "participant"), reconMember("b", "participant")},
		observedAt))

	snap, ok = readSnapshot(t, db, f, "2026-07-23")
	require.True(t, ok)
	require.NotNil(t, snap.membersCount)
	assert.Equal(t, 2, *snap.membersCount)
	require.NotNil(t, snap.membersObservedAt)
	assert.WithinDuration(t, observedAt, *snap.membersObservedAt, time.Second)

	// E o baseline fica registrado no grupo.
	var started, reconciled *time.Time
	require.NoError(t, db.QueryRow(ctx,
		`SELECT member_tracking_started_at, members_reconciled_at FROM groups WHERE id = $1`,
		f.groupID).Scan(&started, &reconciled))
	require.NotNil(t, started)
	require.NotNil(t, reconciled)
}

// =============================================================================
// 21-22: fronteira de tenant
// =============================================================================

func TestGroupMemberRepository_TenantsNeverMix(t *testing.T) {
	db, repo := setupGroupMemberTestDB(t)
	ctx := context.Background()

	// MESMO wa_group_id e MESMO participant_hash em dois tenants distintos.
	const sharedGroup = "1200000001@g.us"
	a := seedTenant(t, db, "session-a", sharedGroup)
	b := seedTenant(t, db, "session-b", sharedGroup)

	p := []models.GroupMemberEventParticipant{member("shared", "participant")}

	_, err := repo.RecordWebhookBatch(ctx, a.tenant,
		makeEvent(a, "evt-a", models.GroupMemberEventJoin, p))
	require.NoError(t, err)
	_, err = repo.RecordWebhookBatch(ctx, b.tenant,
		makeEvent(b, "evt-b", models.GroupMemberEventLeave, p))
	require.NoError(t, err)

	rowA, ok := readProjection(t, db, a, "shared")
	require.True(t, ok)
	assert.True(t, rowA.isActive)

	rowB, ok := readProjection(t, db, b, "shared")
	require.True(t, ok)
	assert.False(t, rowB.isActive, "o estado de um tenant não pode vazar para o outro")

	assert.Equal(t, 1, countLedger(t, db, a))
	assert.Equal(t, 1, countLedger(t, db, b))
}

func TestGroupMemberRepository_ResolveWebhookTenant(t *testing.T) {
	db, repo := setupGroupMemberTestDB(t)
	ctx := context.Background()

	f := seedTenant(t, db, "session-unique", "1200000001@g.us")

	t.Run("uma instância resolve", func(t *testing.T) {
		tenant, err := repo.ResolveWebhookTenant(ctx, "session-unique")
		require.NoError(t, err)
		require.NotNil(t, tenant)
		assert.Equal(t, f.tenant.UserID, tenant.UserID)
		assert.Equal(t, f.tenant.InstancePK, tenant.InstancePK)
	})

	t.Run("zero instâncias é transitório", func(t *testing.T) {
		_, err := repo.ResolveWebhookTenant(ctx, "session-que-nao-existe")
		require.Error(t, err)
		// "unavailable" é a substring retryable do matcher do worker.
		assert.Contains(t, err.Error(), "tracking tenant unavailable")
		assert.NotErrorIs(t, err, repository.ErrAmbiguousTrackingTenant)
	})

	t.Run("nome repetido entre tenants é ambiguidade", func(t *testing.T) {
		// A tabela só garante UNIQUE(user_id, instance_id): dois usuários PODEM
		// ter o mesmo nome de sessão, e resolver "o primeiro" atribuiria o
		// webhook ao tenant errado.
		other := seedTenant(t, db, "session-duplicada", "1200000009@g.us")
		require.NotEmpty(t, other.tenant.UserID)
		dup := seedTenant(t, db, "session-duplicada", "1200000010@g.us")
		require.NotEmpty(t, dup.tenant.UserID)

		_, err := repo.ResolveWebhookTenant(ctx, "session-duplicada")
		require.Error(t, err)
		assert.ErrorIs(t, err, repository.ErrAmbiguousTrackingTenant)
	})
}

// =============================================================================
// 23-25: claim
// =============================================================================

func TestGroupMemberRepository_ConcurrentClaimersGetDisjointBatches(t *testing.T) {
	db, repo := setupGroupMemberTestDB(t)
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		seedTenant(t, db, "session-"+string(rune('a'+i)), "120000000"+string(rune('0'+i))+"@g.us")
	}

	const claimers = 2
	start := make(chan struct{})
	var wg sync.WaitGroup
	batches := make([][]models.GroupReconciliationCandidate, claimers)

	for i := 0; i < claimers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start
			got, err := repo.ClaimGroupsForReconciliation(ctx, 5, 15*time.Minute)
			assert.NoError(t, err)
			batches[idx] = got
		}(i)
	}
	close(start)
	wg.Wait()

	seen := map[string]bool{}
	for _, batch := range batches {
		for _, c := range batch {
			assert.False(t, seen[c.GroupID], "grupo %s reivindicado duas vezes", c.GroupID)
			seen[c.GroupID] = true
		}
	}
	assert.NotEmpty(t, seen)
}

func TestGroupMemberRepository_ClaimPicksConnectedParticipantWhenOwnerIsDown(t *testing.T) {
	db, repo := setupGroupMemberTestDB(t)
	ctx := context.Background()

	f := seedTenant(t, db, "session-owner", "1200000001@g.us")

	// Segundo chip do mesmo usuário, CONECTADO e participante do grupo.
	var secondPK int
	require.NoError(t, db.QueryRow(ctx,
		`INSERT INTO instances (user_id, instance_id, status) VALUES ($1, 'session-backup', 'connected') RETURNING id`,
		f.tenant.UserID).Scan(&secondPK))
	require.NoError(t, db.Exec(ctx,
		`UPDATE groups SET participating_instance_ids = ARRAY[instance_id, $2::int] WHERE id = $1`,
		f.groupID, secondPK))

	// A owner cai.
	require.NoError(t, db.Exec(ctx,
		`UPDATE instances SET status = 'disconnected' WHERE id = $1`, f.tenant.InstancePK))

	got, err := repo.ClaimGroupsForReconciliation(ctx, 10, 15*time.Minute)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, secondPK, got[0].InstancePK)
	assert.Equal(t, "session-backup", got[0].SessionName, "escolhe o chip conectado participante")

	// Sem NENHUM chip conectado o grupo simplesmente não é candidato.
	require.NoError(t, db.Exec(ctx, `UPDATE instances SET status = 'disconnected'`))
	require.NoError(t, repo.ReleaseReconciliationClaim(ctx, f.groupID))

	got, err = repo.ClaimGroupsForReconciliation(ctx, 10, 15*time.Minute)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestGroupMemberRepository_ExpiredClaimBecomesEligibleAgain(t *testing.T) {
	db, repo := setupGroupMemberTestDB(t)
	ctx := context.Background()

	f := seedTenant(t, db, "session-a", "1200000001@g.us")

	first, err := repo.ClaimGroupsForReconciliation(ctx, 10, 15*time.Minute)
	require.NoError(t, err)
	require.Len(t, first, 1)

	// Claim vivo: o grupo não é reivindicado de novo.
	second, err := repo.ClaimGroupsForReconciliation(ctx, 10, 15*time.Minute)
	require.NoError(t, err)
	assert.Empty(t, second)

	// Processo morto: o claim envelhece além do lease e volta para a fila.
	require.NoError(t, db.Exec(ctx,
		`UPDATE groups SET members_reconcile_claimed_at = now() - interval '20 minutes' WHERE id = $1`,
		f.groupID))

	third, err := repo.ClaimGroupsForReconciliation(ctx, 10, 15*time.Minute)
	require.NoError(t, err)
	require.Len(t, third, 1, "claim expirado volta a ser elegível")

	// E liberar explicitamente também devolve.
	require.NoError(t, repo.ReleaseReconciliationClaim(ctx, f.groupID))
	fourth, err := repo.ClaimGroupsForReconciliation(ctx, 10, 15*time.Minute)
	require.NoError(t, err)
	assert.Len(t, fourth, 1)
}

// =============================================================================
// 26: retenção
// =============================================================================

func TestGroupMemberRepository_CleanupRespectsWindowsAndKeepsProjection(t *testing.T) {
	db, repo := setupGroupMemberTestDB(t)
	f := seedTenant(t, db, "session-a", "1200000001@g.us")
	ctx := context.Background()

	now := time.Now().UTC()

	// Um fato antigo (200d) e um recente (10d).
	for _, step := range []struct {
		id  string
		age time.Duration
	}{
		{"evt-antigo", 200 * 24 * time.Hour},
		{"evt-recente", 10 * 24 * time.Hour},
	} {
		ts := now.Add(-step.age)
		_, err := repo.RecordWebhookBatch(ctx, f.tenant,
			makeEvent(f, step.id, models.GroupMemberEventJoin,
				[]models.GroupMemberEventParticipant{member("a", "participant")},
				withOccurredAt(ts), withProviderCreatedAt(ts), withAppliedAt(ts)))
		require.NoError(t, err)
	}

	// Um snapshot bem além dos 400 dias.
	require.NoError(t, db.Exec(ctx, `
		INSERT INTO group_member_daily_snapshots (user_id, wa_group_id, snapshot_date, joins_count)
		VALUES ($1, $2, $3::date, 1)`,
		f.tenant.UserID, f.waGroupID, now.Add(-500*24*time.Hour).Format("2006-01-02")))

	deleted, err := repo.CleanupOldGroupMemberEvents(ctx, now.Add(-180*24*time.Hour), 10_000, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(1), deleted, "só o fato fora da janela sai")
	assert.Equal(t, 1, countLedger(t, db, f), "o recente permanece")

	deletedSnaps, err := repo.CleanupOldGroupMemberSnapshots(ctx, now.Add(-400*24*time.Hour), 10_000, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(1), deletedSnaps)

	// A PROJEÇÃO é estado atual, não histórico: retenção nunca a apaga.
	_, ok := readProjection(t, db, f, "a")
	assert.True(t, ok, "a projeção sobrevive à retenção")

	// E o grupo/claim continuam intocados.
	var n int
	require.NoError(t, db.QueryRow(ctx, `SELECT count(*) FROM groups WHERE id = $1`, f.groupID).Scan(&n))
	assert.Equal(t, 1, n)
}

func TestGroupMemberRepository_CleanupIsBatchedAndBounded(t *testing.T) {
	db, repo := setupGroupMemberTestDB(t)
	f := seedTenant(t, db, "session-a", "1200000001@g.us")
	ctx := context.Background()

	now := time.Now().UTC()
	old := now.Add(-200 * 24 * time.Hour)

	participants := make([]models.GroupMemberEventParticipant, 0, 5)
	for i := 0; i < 5; i++ {
		participants = append(participants, member("p"+string(rune('a'+i)), "participant"))
	}
	_, err := repo.RecordWebhookBatch(ctx, f.tenant,
		makeEvent(f, "evt-bulk", models.GroupMemberEventJoin, participants,
			withOccurredAt(old), withProviderCreatedAt(old), withAppliedAt(old)))
	require.NoError(t, err)

	// batchSize 2, maxBatches 2 => teto de 4 linhas nesta execução: o cron não
	// pode monopolizar I/O do Postgres numa única passada.
	deleted, err := repo.CleanupOldGroupMemberEvents(ctx, now.Add(-180*24*time.Hour), 2, 2)
	require.NoError(t, err)
	assert.Equal(t, int64(4), deleted)
	assert.Equal(t, 1, countLedger(t, db, f), "o restante fica para a próxima execução")
}
