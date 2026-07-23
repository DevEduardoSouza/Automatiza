package postgres

import (
	"context"
	"fmt"
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

// Testes da cadência do espelho contra PostgreSQL REAL (migração 078 executada
// do arquivo, não de uma aproximação escrita à mão).
//
// Por que integração e não mock: tudo que garante a correção desta feature vive
// no banco — advisory lock, FOR UPDATE, SKIP LOCKED, CAS por token, constraint
// de coerência de mídia. Um mock provaria apenas que o Go chama as funções na
// ordem que eu escrevi, que é justamente a parte que não pode falhar sozinha.

// monitorPredecessorSchema cria SÓ o que a 078 referencia (profiles e
// automations). O schema das tabelas novas não é replicado aqui de propósito.
const monitorPredecessorSchema = `
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS profiles (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4()
);

CREATE TABLE IF NOT EXISTS automations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    status VARCHAR(50) NOT NULL DEFAULT 'active',
    type VARCHAR(50) NOT NULL DEFAULT 'monitoring'
);
`

func migration078(t *testing.T) string {
	t.Helper()
	path := filepath.Join("..", "..", "..", "scripts", "db", "migrations", "078_monitor_dispatch_cadence.up.sql")
	data, err := os.ReadFile(path)
	require.NoError(t, err, "migração 078 deve existir em %s", path)
	return string(data)
}

func setupMonitorCadenceDB(t *testing.T) (*DB, repository.MonitorCadenceRepository) {
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

	_, err = pool.Exec(ctx, monitorPredecessorSchema)
	require.NoError(t, err, "predecessor schema")

	_, err = pool.Exec(ctx, migration078(t))
	require.NoError(t, err, "a migração 078 REAL deve aplicar sobre o schema predecessor")

	// Idempotência: reaplicar não pode falhar (o runner pode reexecutar).
	_, err = pool.Exec(ctx, migration078(t))
	require.NoError(t, err, "a 078 deve ser idempotente")

	t.Cleanup(func() {
		pool.Close()
		_ = pgContainer.Terminate(context.Background())
	})

	db := &DB{pool: pool, logger: zap.NewNop()}
	return db, NewMonitorCadenceRepository(db, zap.NewNop())
}

func seedAutomation(t *testing.T, db *DB, userID string) string {
	t.Helper()
	var id string
	require.NoError(t, db.QueryRow(context.Background(),
		`INSERT INTO automations (user_id) VALUES ($1) RETURNING id::text`, userID).Scan(&id))
	return id
}

func seedUser(t *testing.T, db *DB) string {
	t.Helper()
	var id string
	require.NoError(t, db.QueryRow(context.Background(),
		`INSERT INTO profiles DEFAULT VALUES RETURNING id::text`).Scan(&id))
	return id
}

// passthroughSchedule: o horário pedido é o horário concedido (sem janela).
func passthroughSchedule(base time.Time, _ models.MonitorWindowSnapshot) (time.Time, error) {
	return base, nil
}

func admission(userID, messageID string, autos ...models.MonitorAdmissionAutomation) models.MonitorAdmissionInput {
	return models.MonitorAdmissionInput{
		UserID:            userID,
		SourceSessionName: "session_test",
		SourceChatID:      "120363@g.us",
		SourceMessageID:   messageID,
		ContentSHA256:     fmt.Sprintf("%064s", messageID),
		ReceivedAt:        time.Now().UTC(),
		MessageSnapshot:   models.MonitorMessageSnapshot{MessageID: messageID, Body: "oferta " + messageID},
		Automations:       autos,
		Schedule:          passthroughSchedule,
	}
}

func autoWithInterval(id string, minutes int) models.MonitorAdmissionAutomation {
	return models.MonitorAdmissionAutomation{ID: id, IntervalMinutes: minutes, DelaySeconds: 10}
}

// =============================================================================
// Admissão
// =============================================================================

func TestMonitorCadence_AdmitReservesAndSpacesSequences(t *testing.T) {
	db, repo := setupMonitorCadenceDB(t)
	ctx := context.Background()
	user := seedUser(t, db)
	auto := seedAutomation(t, db, user)

	var scheduled []time.Time
	for i := 1; i <= 3; i++ {
		res, err := repo.Admit(ctx, admission(user, fmt.Sprintf("msg%d", i), autoWithInterval(auto, 30)))
		require.NoError(t, err)
		require.Len(t, res.Deferred, 1, "cada mensagem reserva exatamente um item")

		var seq int64
		var at time.Time
		require.NoError(t, db.QueryRow(ctx,
			`SELECT sequence_no, scheduled_at FROM automation_monitor_pending WHERE id = $1`,
			res.Deferred[auto]).Scan(&seq, &at))
		assert.Equal(t, int64(i), seq, "sequências são consecutivas")
		scheduled = append(scheduled, at)
	}

	// O 2º sai no mínimo 30min depois do 1º, o 3º 30min depois do 2º.
	assert.WithinDuration(t, scheduled[0].Add(30*time.Minute), scheduled[1], time.Second)
	assert.WithinDuration(t, scheduled[1].Add(30*time.Minute), scheduled[2], time.Second)
}

func TestMonitorCadence_ConcurrentAdmissionOfSameMessageReservesOnce(t *testing.T) {
	db, repo := setupMonitorCadenceDB(t)
	ctx := context.Background()
	user := seedUser(t, db)
	auto := seedAutomation(t, db, user)

	// A WAHA entrega o MESMO key.id por cada chip do dono. Todas as entregas
	// competem ao mesmo tempo; só uma pode consumir sequência.
	var wg sync.WaitGroup
	errs := make([]error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, err := repo.Admit(ctx, admission(user, "same-message", autoWithInterval(auto, 5)))
			errs[idx] = err
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		require.NoError(t, err, "admissão concorrente %d", i)
	}

	var payloads, pendings int
	var nextSeq int64
	require.NoError(t, db.QueryRow(ctx, `SELECT count(*) FROM automation_monitor_payloads`).Scan(&payloads))
	require.NoError(t, db.QueryRow(ctx, `SELECT count(*) FROM automation_monitor_pending`).Scan(&pendings))
	require.NoError(t, db.QueryRow(ctx,
		`SELECT next_sequence FROM automation_monitor_dispatch_state WHERE automation_id = $1`, auto).Scan(&nextSeq))

	assert.Equal(t, 1, payloads, "um único payload para a mesma mensagem")
	assert.Equal(t, 1, pendings, "um único item na fila")
	assert.Equal(t, int64(2), nextSeq, "a sequência avançou exatamente uma vez")
}

func TestMonitorCadence_ContentDuplicateIsSuppressedNotQueued(t *testing.T) {
	db, repo := setupMonitorCadenceDB(t)
	ctx := context.Background()
	user := seedUser(t, db)
	auto := seedAutomation(t, db, user)

	first := admission(user, "msg-a", autoWithInterval(auto, 10))
	_, err := repo.Admit(ctx, first)
	require.NoError(t, err)

	// Mesma mensagem repostada com outro message ID, dentro da janela de dedup.
	second := admission(user, "msg-b", autoWithInterval(auto, 10))
	second.ContentSHA256 = first.ContentSHA256
	res, err := repo.Admit(ctx, second)
	require.NoError(t, err)

	assert.Empty(t, res.Deferred, "conteúdo duplicado não entra na fila")
	assert.Contains(t, res.Suppressed, auto, "e também não sai pelo caminho imediato")

	var pendings int
	require.NoError(t, db.QueryRow(ctx, `SELECT count(*) FROM automation_monitor_pending`).Scan(&pendings))
	assert.Equal(t, 1, pendings)
}

func TestMonitorCadence_BacklogFullAbortsWholeAdmission(t *testing.T) {
	db, repo := setupMonitorCadenceDB(t)
	ctx := context.Background()
	user := seedUser(t, db)
	autoA := seedAutomation(t, db, user)
	autoB := seedAutomation(t, db, user)

	// Enche a fila de UMA das automações direto no banco (mais rápido que 200
	// admissões e prova o mesmo: o gate lê o estado real).
	for i := 1; i <= models.MonitorMaxBacklogPerAutomation; i++ {
		_, err := db.pool.Exec(ctx, `
			INSERT INTO automation_monitor_payloads
				(user_id, source_session_name, source_chat_id, source_message_id, content_sha256, received_at, message_snapshot)
			VALUES ($1,'s','c',$2,$3,now(),'{}'::jsonb)`,
			user, fmt.Sprintf("fill%d", i), fmt.Sprintf("%064d", i))
		require.NoError(t, err)
		_, err = db.pool.Exec(ctx, `
			INSERT INTO automation_monitor_pending
				(automation_id, automation_id_snapshot, user_id, payload_id, sequence_no,
				 interval_minutes_snapshot, window_snapshot, delay_seconds_snapshot, scheduled_at, not_before_at)
			SELECT $1, $1, $2, id, $3, 5, '{}'::jsonb, 0, now(), now()
			FROM automation_monitor_payloads WHERE source_message_id = $4`,
			autoA, user, i, fmt.Sprintf("fill%d", i))
		require.NoError(t, err)
	}

	_, err := repo.Admit(ctx, admission(user, "overflow",
		autoWithInterval(autoA, 5), autoWithInterval(autoB, 5)))
	require.ErrorIs(t, err, models.ErrMonitorBacklogFull)

	// Atomicidade: a automação SAUDÁVEL também não ficou com item nem payload.
	var pendingsB, payloads int
	require.NoError(t, db.QueryRow(ctx,
		`SELECT count(*) FROM automation_monitor_pending WHERE automation_id_snapshot = $1`, autoB).Scan(&pendingsB))
	require.NoError(t, db.QueryRow(ctx,
		`SELECT count(*) FROM automation_monitor_payloads WHERE source_message_id = 'overflow'`).Scan(&payloads))
	assert.Zero(t, pendingsB, "nenhuma automação é reservada quando outra estoura o teto")
	assert.Zero(t, payloads, "e o payload também sofre rollback")
}

func TestMonitorCadence_RedeliveryReturnsSameItemsWithoutConsumingSlot(t *testing.T) {
	db, repo := setupMonitorCadenceDB(t)
	ctx := context.Background()
	user := seedUser(t, db)
	auto := seedAutomation(t, db, user)

	first, err := repo.Admit(ctx, admission(user, "dup", autoWithInterval(auto, 15)))
	require.NoError(t, err)
	second, err := repo.Admit(ctx, admission(user, "dup", autoWithInterval(auto, 15)))
	require.NoError(t, err)

	assert.True(t, second.Redelivery)
	assert.Equal(t, first.Deferred[auto], second.Deferred[auto], "redelivery devolve o MESMO item")

	var nextSeq int64
	require.NoError(t, db.QueryRow(ctx,
		`SELECT next_sequence FROM automation_monitor_dispatch_state WHERE automation_id = $1`, auto).Scan(&nextSeq))
	assert.Equal(t, int64(2), nextSeq, "redelivery NÃO consome sequência")
}

func TestMonitorCadence_TwoAutomationsAreIndependent(t *testing.T) {
	db, repo := setupMonitorCadenceDB(t)
	ctx := context.Background()
	user := seedUser(t, db)
	autoA := seedAutomation(t, db, user)
	autoB := seedAutomation(t, db, user)

	res, err := repo.Admit(ctx, admission(user, "multi",
		autoWithInterval(autoA, 10), autoWithInterval(autoB, 60)))
	require.NoError(t, err)
	require.Len(t, res.Deferred, 2, "as duas automações são reservadas na MESMA transação")

	// Cada uma tem sua própria sequência e seu próprio horário livre.
	var availA, availB time.Time
	require.NoError(t, db.QueryRow(ctx,
		`SELECT next_available_at FROM automation_monitor_dispatch_state WHERE automation_id = $1`, autoA).Scan(&availA))
	require.NoError(t, db.QueryRow(ctx,
		`SELECT next_available_at FROM automation_monitor_dispatch_state WHERE automation_id = $1`, autoB).Scan(&availB))
	assert.WithinDuration(t, availA.Add(50*time.Minute), availB, time.Second)
}

// =============================================================================
// Outbox, ABA e execução
// =============================================================================

func TestMonitorCadence_ClaimDueHeadsIsSingleFlightPerAutomation(t *testing.T) {
	db, repo := setupMonitorCadenceDB(t)
	ctx := context.Background()
	user := seedUser(t, db)
	auto := seedAutomation(t, db, user)

	// Dois itens devidos; só o HEAD pode ser reivindicado.
	_, err := repo.Admit(ctx, admission(user, "m1", autoWithInterval(auto, 0)))
	require.NoError(t, err)
	_, err = repo.Admit(ctx, admission(user, "m2", autoWithInterval(auto, 0)))
	require.NoError(t, err)

	leases, err := repo.ClaimDueHeads(ctx, 10)
	require.NoError(t, err)
	require.Len(t, leases, 1, "só o menor sequence_no não terminal concorre")
	assert.Equal(t, int64(1), leases[0].SequenceNo)

	// Segunda passada não devolve nada: o head já está em enqueuing e o
	// sucessor não pode ultrapassá-lo.
	again, err := repo.ClaimDueHeads(ctx, 10)
	require.NoError(t, err)
	assert.Empty(t, again)
}

func TestMonitorCadence_StaleTaskCannotClaimAfterLeaseReissue(t *testing.T) {
	db, repo := setupMonitorCadenceDB(t)
	ctx := context.Background()
	user := seedUser(t, db)
	auto := seedAutomation(t, db, user)

	_, err := repo.Admit(ctx, admission(user, "aba", autoWithInterval(auto, 0)))
	require.NoError(t, err)

	first, err := repo.ClaimDueHeads(ctx, 1)
	require.NoError(t, err)
	require.Len(t, first, 1)

	// Envelhece a lease e roda a manutenção: o item volta a pending.
	_, err = db.pool.Exec(ctx,
		`UPDATE automation_monitor_pending SET enqueue_claimed_at = now() - interval '10 minutes'`)
	require.NoError(t, err)
	stats, err := repo.RecoverLeases(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, stats.EnqueueLeasesReleased)

	second, err := repo.ClaimDueHeads(ctx, 1)
	require.NoError(t, err)
	require.Len(t, second, 1)
	assert.Greater(t, second[0].EnqueueGen, first[0].EnqueueGen, "a geração avança na reemissão")

	// A task ANTIGA acorda agora: não pode mexer no item.
	stale, err := repo.ClaimPending(ctx, first[0].PendingID, first[0].EnqueueGen, first[0].EnqueueToken)
	require.NoError(t, err)
	assert.Nil(t, stale, "task de geração antiga é no-op")

	// A nova executa normalmente.
	work, err := repo.ClaimPending(ctx, second[0].PendingID, second[0].EnqueueGen, second[0].EnqueueToken)
	require.NoError(t, err)
	require.NotNil(t, work)
	assert.Equal(t, auto, work.AutomationID)
}

func TestMonitorCadence_DestinationLedgerLifecycle(t *testing.T) {
	db, repo := setupMonitorCadenceDB(t)
	ctx := context.Background()
	user := seedUser(t, db)
	auto := seedAutomation(t, db, user)

	res, err := repo.Admit(ctx, admission(user, "dests", autoWithInterval(auto, 0)))
	require.NoError(t, err)
	pendingID := res.Deferred[auto]

	leases, err := repo.ClaimDueHeads(ctx, 1)
	require.NoError(t, err)
	require.Len(t, leases, 1)
	work, err := repo.ClaimPending(ctx, leases[0].PendingID, leases[0].EnqueueGen, leases[0].EnqueueToken)
	require.NoError(t, err)
	require.NotNil(t, work)
	require.NoError(t, repo.MarkPendingStarted(ctx, pendingID, work.ProcessingToken))

	dests, err := repo.MaterializeDestinations(ctx, pendingID, []models.MonitorPendingDestination{
		{Ordinal: 0, GroupID: "g1@g.us", CanonicalKey: "shopee:1:2", Snapshot: []byte(`{"store":"shopee"}`)},
		{Ordinal: 1, GroupID: "g2@g.us", CanonicalKey: "shopee:1:2", Snapshot: []byte(`{"store":"shopee"}`)},
	})
	require.NoError(t, err)
	require.Len(t, dests, 2)

	// Rematerializar com OUTRA ordem não pode reescrever nada.
	again, err := repo.MaterializeDestinations(ctx, pendingID, []models.MonitorPendingDestination{
		{Ordinal: 0, GroupID: "g2@g.us", Snapshot: []byte(`{}`)},
	})
	require.NoError(t, err)
	require.Len(t, again, 2, "a ordem congelada na primeira execução é a que vale")
	assert.Equal(t, "g1@g.us", again[0].GroupID)

	// Ciclo feliz do primeiro destino.
	token := "11111111-1111-1111-1111-111111111111"
	claimed, err := repo.ClaimDestination(ctx, dests[0].ID, token)
	require.NoError(t, err)
	require.True(t, claimed)

	// Um segundo claim do MESMO destino falha (o CAS já não casa).
	dup, err := repo.ClaimDestination(ctx, dests[0].ID, "22222222-2222-2222-2222-222222222222")
	require.NoError(t, err)
	assert.False(t, dup, "destino reivindicado não pode ser reivindicado de novo")

	sending, err := repo.MarkDestinationSending(ctx, dests[0].ID, token)
	require.NoError(t, err)
	require.True(t, sending)
	require.NoError(t, repo.FinishDestination(ctx, dests[0].ID, models.MonitorDestinationSent, "", nil))

	// Destino já terminal jamais volta a ser reivindicável.
	after, err := repo.ClaimDestination(ctx, dests[0].ID, token)
	require.NoError(t, err)
	assert.False(t, after)

	// O segundo destino morre em 'sending': a manutenção o dá como
	// assumed_sent — nunca reenviado.
	_, err = repo.ClaimDestination(ctx, dests[1].ID, token)
	require.NoError(t, err)
	_, err = repo.MarkDestinationSending(ctx, dests[1].ID, token)
	require.NoError(t, err)
	_, err = db.pool.Exec(ctx,
		`UPDATE automation_monitor_pending_destinations SET sending_at = now() - interval '10 minutes' WHERE id = $1`,
		dests[1].ID)
	require.NoError(t, err)

	stats, err := repo.RecoverLeases(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, stats.DestinationsAssumed)

	final, err := repo.ListDestinations(ctx, pendingID)
	require.NoError(t, err)
	assert.Equal(t, models.MonitorDestinationSent, final[0].Status)
	assert.Equal(t, models.MonitorDestinationAssumedSent, final[1].Status)
}

func TestMonitorCadence_ClaimedDestinationLeaseReturnsToPending(t *testing.T) {
	db, repo := setupMonitorCadenceDB(t)
	ctx := context.Background()
	user := seedUser(t, db)
	auto := seedAutomation(t, db, user)

	res, err := repo.Admit(ctx, admission(user, "lease", autoWithInterval(auto, 0)))
	require.NoError(t, err)
	dests, err := repo.MaterializeDestinations(ctx, res.Deferred[auto], []models.MonitorPendingDestination{
		{Ordinal: 0, GroupID: "g1@g.us", Snapshot: []byte(`{}`)},
	})
	require.NoError(t, err)

	_, err = repo.ClaimDestination(ctx, dests[0].ID, "33333333-3333-3333-3333-333333333333")
	require.NoError(t, err)
	_, err = db.pool.Exec(ctx,
		`UPDATE automation_monitor_pending_destinations SET claimed_at = now() - interval '30 minutes'`)
	require.NoError(t, err)

	stats, err := repo.RecoverLeases(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, stats.DestinationsReleased)

	after, err := repo.ListDestinations(ctx, res.Deferred[auto])
	require.NoError(t, err)
	assert.Equal(t, models.MonitorDestinationPending, after[0].Status,
		"claim sem POST autorizado é seguro de retomar")
}

// =============================================================================
// Cancelamento, retenção e autorização
// =============================================================================

func TestMonitorCadence_DeletedAutomationOrphansAreCanceledNotLost(t *testing.T) {
	db, repo := setupMonitorCadenceDB(t)
	ctx := context.Background()
	user := seedUser(t, db)
	auto := seedAutomation(t, db, user)

	res, err := repo.Admit(ctx, admission(user, "orphan", autoWithInterval(auto, 30)))
	require.NoError(t, err)
	pendingID := res.Deferred[auto]

	_, err = db.pool.Exec(ctx, `DELETE FROM automations WHERE id = $1`, auto)
	require.NoError(t, err)

	// O FK zera o vínculo vivo, mas o item e sua auditoria continuam lá.
	var liveID *string
	var snapshot string
	require.NoError(t, db.QueryRow(ctx,
		`SELECT automation_id::text, automation_id_snapshot::text FROM automation_monitor_pending WHERE id = $1`,
		pendingID).Scan(&liveID, &snapshot))
	assert.Nil(t, liveID, "ON DELETE SET NULL preserva a linha")
	assert.Equal(t, auto, snapshot, "o snapshot mantém a auditoria")

	stats, err := repo.RecoverLeases(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, stats.PendingsCanceled)

	var status, code string
	require.NoError(t, db.QueryRow(ctx,
		`SELECT status, last_error_code FROM automation_monitor_pending WHERE id = $1`, pendingID).Scan(&status, &code))
	assert.Equal(t, models.MonitorPendingCanceled, status)
	assert.Equal(t, models.MonitorCodeAutomationGone, code)
}

func TestMonitorCadence_SummariesAreScopedToOwner(t *testing.T) {
	db, repo := setupMonitorCadenceDB(t)
	ctx := context.Background()
	owner := seedUser(t, db)
	intruder := seedUser(t, db)
	auto := seedAutomation(t, db, owner)

	_, err := repo.Admit(ctx, admission(owner, "s1", autoWithInterval(auto, 20)))
	require.NoError(t, err)

	mine, err := repo.GetMonitorQueueSummaries(ctx, owner, []string{auto})
	require.NoError(t, err)
	require.Contains(t, mine, auto)
	assert.Equal(t, 1, mine[auto].PendingCount)
	require.NotNil(t, mine[auto].NextScheduledAt)

	// Autorização horizontal: pedir a fila de outro dono devolve vazio, nunca
	// os dados dele.
	theirs, err := repo.GetMonitorQueueSummaries(ctx, intruder, []string{auto})
	require.NoError(t, err)
	assert.Empty(t, theirs)
}

func TestMonitorCadence_CleanupNeverTouchesLiveQueue(t *testing.T) {
	db, repo := setupMonitorCadenceDB(t)
	ctx := context.Background()
	user := seedUser(t, db)
	autoLive := seedAutomation(t, db, user)
	autoDone := seedAutomation(t, db, user)

	live, err := repo.Admit(ctx, admission(user, "live", autoWithInterval(autoLive, 30)))
	require.NoError(t, err)
	done, err := repo.Admit(ctx, admission(user, "done", autoWithInterval(autoDone, 30)))
	require.NoError(t, err)

	// Envelhece um item TERMINAL para muito além da retenção.
	_, err = db.pool.Exec(ctx, `
		UPDATE automation_monitor_pending
		SET status = 'completed', completed_at = now() - interval '90 days'
		WHERE id = $1`, done.Deferred[autoDone])
	require.NoError(t, err)

	deleted, err := repo.Cleanup(ctx, time.Now().Add(-30*24*time.Hour), 1000)
	require.NoError(t, err)
	assert.Equal(t, 1, deleted)

	var liveStill int
	require.NoError(t, db.QueryRow(ctx,
		`SELECT count(*) FROM automation_monitor_pending WHERE id = $1`, live.Deferred[autoLive]).Scan(&liveStill))
	assert.Equal(t, 1, liveStill, "item não terminal nunca é apagado")
}

func TestMonitorCadence_MediaIsPersistedAndPurgedWhenQueueDrains(t *testing.T) {
	db, repo := setupMonitorCadenceDB(t)
	ctx := context.Background()
	user := seedUser(t, db)
	auto := seedAutomation(t, db, user)

	in := admission(user, "with-media", autoWithInterval(auto, 0))
	in.Media = &models.MonitorPayloadMedia{
		Mimetype:  "image/jpeg",
		SHA256:    fmt.Sprintf("%064d", 7),
		SizeBytes: 4,
		Bytes:     []byte{0xFF, 0xD8, 0xFF, 0xD9},
	}
	res, err := repo.Admit(ctx, in)
	require.NoError(t, err)

	// Os BYTES ficam no banco: a URL da WAHA pode expirar antes do disparo.
	leases, err := repo.ClaimDueHeads(ctx, 1)
	require.NoError(t, err)
	work, err := repo.ClaimPending(ctx, leases[0].PendingID, leases[0].EnqueueGen, leases[0].EnqueueToken)
	require.NoError(t, err)
	require.NotNil(t, work.Media)
	assert.Equal(t, []byte{0xFF, 0xD8, 0xFF, 0xD9}, work.Media.Bytes)
	assert.Equal(t, "image/jpeg", work.Media.Mimetype)

	// Item terminal → a mídia é zerada na primeira manutenção (não espera 30d).
	require.NoError(t, repo.FinalizePending(ctx, res.Deferred[auto], work.ProcessingToken,
		models.MonitorPendingCompleted, models.MonitorPendingResult{SentCount: 1}, ""))
	_, err = repo.RecoverLeases(ctx)
	require.NoError(t, err)

	var bytes []byte
	var purgedAt *time.Time
	require.NoError(t, db.QueryRow(ctx,
		`SELECT media_bytes, media_purged_at FROM automation_monitor_payloads WHERE source_message_id = 'with-media'`).
		Scan(&bytes, &purgedAt))
	assert.Empty(t, bytes, "bytes zerados")
	assert.NotNil(t, purgedAt)
}

func TestMonitorCadence_RescheduleMovesOnlyNotBefore(t *testing.T) {
	db, repo := setupMonitorCadenceDB(t)
	ctx := context.Background()
	user := seedUser(t, db)
	auto := seedAutomation(t, db, user)

	res, err := repo.Admit(ctx, admission(user, "resched", autoWithInterval(auto, 0)))
	require.NoError(t, err)
	pendingID := res.Deferred[auto]

	var scheduledBefore time.Time
	var seqBefore int64
	require.NoError(t, db.QueryRow(ctx,
		`SELECT scheduled_at, sequence_no FROM automation_monitor_pending WHERE id = $1`, pendingID).
		Scan(&scheduledBefore, &seqBefore))

	leases, err := repo.ClaimDueHeads(ctx, 1)
	require.NoError(t, err)
	work, err := repo.ClaimPending(ctx, leases[0].PendingID, leases[0].EnqueueGen, leases[0].EnqueueToken)
	require.NoError(t, err)

	reopen := time.Now().Add(2 * time.Hour)
	require.NoError(t, repo.ReschedulePending(ctx, pendingID, work.ProcessingToken, reopen, models.MonitorCodeWindowClosed))

	var scheduledAfter, notBefore time.Time
	var seqAfter int64
	var status string
	require.NoError(t, db.QueryRow(ctx,
		`SELECT scheduled_at, not_before_at, sequence_no, status FROM automation_monitor_pending WHERE id = $1`, pendingID).
		Scan(&scheduledAfter, &notBefore, &seqAfter, &status))

	assert.Equal(t, models.MonitorPendingPending, status)
	assert.WithinDuration(t, scheduledBefore, scheduledAfter, time.Millisecond, "scheduled_at é imutável")
	assert.Equal(t, seqBefore, seqAfter, "sequence_no é imutável")
	assert.WithinDuration(t, reopen, notBefore, time.Second, "só o horário de liberação muda")
}

// A PROVA CENTRAL da feature: depois que um item começa a despachar, o próximo
// espera o intervalo contado a partir do INÍCIO REAL do anterior — não do
// horário reservado. É isso que impede a rajada de catch-up depois de um
// downtime, quando vários horários reservados já venceram.
func TestMonitorCadence_SuccessorWaitsIntervalFromRealStart(t *testing.T) {
	db, repo := setupMonitorCadenceDB(t)
	ctx := context.Background()
	user := seedUser(t, db)
	auto := seedAutomation(t, db, user)

	first, err := repo.Admit(ctx, admission(user, "m1", autoWithInterval(auto, 30)))
	require.NoError(t, err)
	second, err := repo.Admit(ctx, admission(user, "m2", autoWithInterval(auto, 30)))
	require.NoError(t, err)

	// Simula downtime: os dois horários reservados já venceram.
	_, err = db.pool.Exec(ctx,
		`UPDATE automation_monitor_pending SET not_before_at = now() - interval '1 hour'`)
	require.NoError(t, err)

	// Item 1 sai e marca o início real.
	leases, err := repo.ClaimDueHeads(ctx, 10)
	require.NoError(t, err)
	require.Len(t, leases, 1)
	work, err := repo.ClaimPending(ctx, leases[0].PendingID, leases[0].EnqueueGen, leases[0].EnqueueToken)
	require.NoError(t, err)
	require.NoError(t, repo.MarkPendingStarted(ctx, first.Deferred[auto], work.ProcessingToken))
	require.NoError(t, repo.FinalizePending(ctx, first.Deferred[auto], work.ProcessingToken,
		models.MonitorPendingCompleted, models.MonitorPendingResult{SentCount: 1}, ""))

	// Item 2 está devido pelo horário, mas o intervalo ainda não passou desde
	// o início real do item 1: NÃO pode sair agora.
	blocked, err := repo.ClaimDueHeads(ctx, 10)
	require.NoError(t, err)
	assert.Empty(t, blocked, "sucessor não pode sair antes do intervalo desde o início real do anterior")

	// Passados os 30 minutos, ele sai.
	_, err = db.pool.Exec(ctx,
		`UPDATE automation_monitor_dispatch_state SET last_dispatch_started_at = now() - interval '31 minutes'`)
	require.NoError(t, err)
	released, err := repo.ClaimDueHeads(ctx, 10)
	require.NoError(t, err)
	require.Len(t, released, 1, "vencido o intervalo, o sucessor é liberado")
	assert.Equal(t, second.Deferred[auto], released[0].PendingID)
	assert.Equal(t, int64(2), released[0].SequenceNo)
}

// Retry do MESMO item não espera o intervalo de novo: ele já é o head e já
// começou. Sem esta exceção, um destino que falhou ficaria refém da cadência.
func TestMonitorCadence_RetryOfStartedHeadIsNotIntervalGated(t *testing.T) {
	db, repo := setupMonitorCadenceDB(t)
	ctx := context.Background()
	user := seedUser(t, db)
	auto := seedAutomation(t, db, user)

	res, err := repo.Admit(ctx, admission(user, "retry", autoWithInterval(auto, 120)))
	require.NoError(t, err)

	leases, err := repo.ClaimDueHeads(ctx, 1)
	require.NoError(t, err)
	require.Len(t, leases, 1)
	work, err := repo.ClaimPending(ctx, leases[0].PendingID, leases[0].EnqueueGen, leases[0].EnqueueToken)
	require.NoError(t, err)
	require.NoError(t, repo.MarkPendingStarted(ctx, res.Deferred[auto], work.ProcessingToken))

	// Volta para a fila (destino falhou) com liberação imediata.
	require.NoError(t, repo.ReschedulePending(ctx, res.Deferred[auto], work.ProcessingToken,
		time.Now().Add(-time.Second), models.MonitorCodeSendFailed))

	again, err := repo.ClaimDueHeads(ctx, 1)
	require.NoError(t, err)
	require.Len(t, again, 1, "retry do item já iniciado não é barrado pelo intervalo")
	assert.Equal(t, res.Deferred[auto], again[0].PendingID)
}
