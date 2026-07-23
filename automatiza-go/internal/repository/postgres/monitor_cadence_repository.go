package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

// Ledger da cadência do espelho (Melhoria 7, migração 078).
//
// Duas regras governam TODO este arquivo:
//
//  1. O relógio é o do PostgreSQL (transaction_timestamp()). O relógio do Go
//     nunca decide reserva, lease ou expiração — duas réplicas com skew de
//     segundos reordenariam a fila.
//  2. Toda transição é CAS (compare-and-swap) com token: um worker antigo,
//     acordando depois de um lease reemitido, faz zero linhas e vira no-op.
//     É isso que fecha o ABA de task reenfileirada.
type monitorCadenceRepository struct {
	db     *DB
	logger *zap.Logger
}

// NewMonitorCadenceRepository cria o repositório da cadência do espelho.
func NewMonitorCadenceRepository(db *DB, logger *zap.Logger) repository.MonitorCadenceRepository {
	return &monitorCadenceRepository{db: db, logger: logger}
}

// Leases operacionais, na sintaxe de interval do PostgreSQL (é o banco que as
// aplica; o relógio do Go não participa). Ficam aqui, junto do SQL que as usa,
// para não haver dois lugares dizendo por quanto tempo uma lease vale.
const (
	// Poller morreu entre reivindicar o head e enfileirar a task.
	monitorEnqueueLeaseTTL = "2 minutes"
	// Worker morreu com o item já em processing.
	monitorProcessingLeaseTTL = "6 minutes"
	// Destino reivindicado sem POST autorizado — seguro devolver a pending.
	monitorDestClaimLeaseTTL = "6 minutes"
	// Destino que JÁ autorizou o POST: curto de propósito, porque o desfecho
	// aqui é assumed_sent (terminal) e não pode demorar a ser registrado.
	monitorDestSendingLeaseTTL = "2 minutes"
)

func (r *monitorCadenceRepository) HasNonTerminalPending(ctx context.Context, automationIDs []string) (bool, error) {
	if len(automationIDs) == 0 {
		return false, nil
	}
	var exists bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM automation_monitor_pending
			WHERE automation_id_snapshot = ANY($1::uuid[])
			  AND status IN ('pending','enqueuing','enqueued','processing')
		)`, automationIDs).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

// Admit é a fronteira de durabilidade da feature: quando ela retorna sem erro,
// a mensagem EXISTE no banco e nenhuma falha posterior pode apagá-la.
//
// Ou todas as automações elegíveis são reservadas, ou nenhuma é — não existe
// "uma aceitou, a outra perdeu". Backlog cheio aborta a transação inteira, e o
// erro sobe até o consumidor de webhook, que já tem retry e DLQ de 7 dias.
func (r *monitorCadenceRepository) Admit(ctx context.Context, in models.MonitorAdmissionInput) (*models.MonitorAdmissionResult, error) {
	result := &models.MonitorAdmissionResult{
		Deferred:   make(map[string]int64, len(in.Automations)),
		Suppressed: make(map[string]struct{}),
	}
	if len(in.Automations) == 0 {
		return result, nil
	}

	ids := make([]string, 0, len(in.Automations))
	for _, a := range in.Automations {
		ids = append(ids, a.ID)
	}

	err := r.db.WithTx(ctx, func(tx pgx.Tx) error {
		var dbNow time.Time
		if err := tx.QueryRow(ctx, `SELECT transaction_timestamp()`).Scan(&dbNow); err != nil {
			return fmt.Errorf("read db clock: %w", err)
		}

		// Serializa admissões do MESMO (dono, grupo-fonte) entre réplicas. Sem
		// isto, o dedup de conteúdo abaixo é uma corrida: dois webhooks do
		// mesmo post entregues por chips diferentes leriam "não existe" ao
		// mesmo tempo e criariam dois payloads.
		if _, err := tx.Exec(ctx,
			`SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`,
			in.UserID+":"+in.SourceChatID); err != nil {
			return fmt.Errorf("admission lock: %w", err)
		}

		// 1. Redelivery exata: o payload já existe. Devolve o MESMO conjunto
		//    reservado na primeira passagem e não consome sequência.
		var payloadID int64
		err := tx.QueryRow(ctx, `
			SELECT id FROM automation_monitor_payloads
			WHERE user_id = $1 AND source_chat_id = $2 AND source_message_id = $3`,
			in.UserID, in.SourceChatID, in.SourceMessageID).Scan(&payloadID)
		switch {
		case err == nil:
			result.Redelivery = true
			rows, qErr := tx.Query(ctx, `
				SELECT automation_id_snapshot::text, id
				FROM automation_monitor_pending
				WHERE payload_id = $1 AND automation_id_snapshot = ANY($2::uuid[])`,
				payloadID, ids)
			if qErr != nil {
				return qErr
			}
			defer rows.Close()
			for rows.Next() {
				var aid string
				var pid int64
				if sErr := rows.Scan(&aid, &pid); sErr != nil {
					return sErr
				}
				result.Deferred[aid] = pid
			}
			return rows.Err()
		case errors.Is(err, pgx.ErrNoRows):
			// segue para a admissão nova
		default:
			return fmt.Errorf("lookup payload: %w", err)
		}

		// 2. Dedup de conteúdo (mesma janela do claim Redis do caminho vivo),
		//    resolvido no banco DEPOIS da persistência ser possível: reivindicar
		//    antes de gravar faria um erro de DB parecer duplicata no retry.
		var duplicate bool
		if err := tx.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM automation_monitor_payloads
				WHERE user_id = $1 AND source_chat_id = $2
				  AND content_sha256 = $3 AND accepted_at > $4
			)`, in.UserID, in.SourceChatID, in.ContentSHA256,
			dbNow.Add(-models.MonitorContentDedupWindow)).Scan(&duplicate); err != nil {
			return fmt.Errorf("content dedup: %w", err)
		}
		if duplicate {
			for _, id := range ids {
				result.Suppressed[id] = struct{}{}
			}
			return nil
		}

		// 3. Payload durável (inclui os bytes da mídia já adquiridos).
		snapshotJSON, mErr := json.Marshal(in.MessageSnapshot)
		if mErr != nil {
			return fmt.Errorf("marshal message snapshot: %w", mErr)
		}
		var mimetype, filename, sha *string
		var size *int
		var bytes []byte
		if in.Media != nil {
			mimetype, filename, sha = &in.Media.Mimetype, &in.Media.Filename, &in.Media.SHA256
			size, bytes = &in.Media.SizeBytes, in.Media.Bytes
		}
		if err := tx.QueryRow(ctx, `
			INSERT INTO automation_monitor_payloads (
				user_id, source_session_name, source_chat_id, source_message_id,
				content_sha256, received_at, message_snapshot,
				media_mimetype, media_filename, media_sha256, media_size_bytes, media_bytes
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
			RETURNING id`,
			in.UserID, in.SourceSessionName, in.SourceChatID, in.SourceMessageID,
			in.ContentSHA256, in.ReceivedAt, snapshotJSON,
			mimetype, filename, sha, size, bytes).Scan(&payloadID); err != nil {
			return fmt.Errorf("insert payload: %w", err)
		}

		// 4. Estado por automação (idempotente).
		if _, err := tx.Exec(ctx, `
			INSERT INTO automation_monitor_dispatch_state (automation_id, user_id)
			SELECT a, $2 FROM UNNEST($1::uuid[]) AS a
			ON CONFLICT (automation_id) DO NOTHING`, ids, in.UserID); err != nil {
			return fmt.Errorf("ensure state rows: %w", err)
		}

		// 5. Trava todos os estados em ordem determinística (UUID crescente):
		//    ordem fixa entre transações concorrentes = sem deadlock.
		states := make(map[string]struct {
			nextSequence int64
			nextAvailAt  time.Time
		}, len(ids))
		rows, err := tx.Query(ctx, `
			SELECT automation_id::text, next_sequence, next_available_at
			FROM automation_monitor_dispatch_state
			WHERE automation_id = ANY($1::uuid[])
			ORDER BY automation_id
			FOR UPDATE`, ids)
		if err != nil {
			return fmt.Errorf("lock states: %w", err)
		}
		for rows.Next() {
			var aid string
			var seq int64
			var avail time.Time
			if sErr := rows.Scan(&aid, &seq, &avail); sErr != nil {
				rows.Close()
				return sErr
			}
			states[aid] = struct {
				nextSequence int64
				nextAvailAt  time.Time
			}{seq, avail}
		}
		rows.Close()
		if rErr := rows.Err(); rErr != nil {
			return rErr
		}

		// 6. Reserva por automação, na ordem em que o chamador as passou.
		for _, auto := range in.Automations {
			st, ok := states[auto.ID]
			if !ok {
				// Automação apagada entre a listagem e agora: nada a reservar.
				continue
			}

			var backlog int
			if err := tx.QueryRow(ctx, `
				SELECT count(*) FROM automation_monitor_pending
				WHERE automation_id_snapshot = $1
				  AND status IN ('pending','enqueuing','enqueued','processing')`,
				auto.ID).Scan(&backlog); err != nil {
				return fmt.Errorf("count backlog: %w", err)
			}
			if backlog >= models.MonitorMaxBacklogPerAutomation {
				return models.ErrMonitorBacklogFull
			}

			base := st.nextAvailAt
			if dbNow.After(base) {
				base = dbNow
			}
			scheduled, wErr := in.Schedule(base, auto.Window)
			if wErr != nil {
				return wErr
			}

			windowJSON, jErr := json.Marshal(auto.Window)
			if jErr != nil {
				return fmt.Errorf("marshal window: %w", jErr)
			}

			var pendingID int64
			if err := tx.QueryRow(ctx, `
				INSERT INTO automation_monitor_pending (
					automation_id, automation_id_snapshot, user_id, payload_id,
					sequence_no, interval_minutes_snapshot, window_snapshot,
					delay_seconds_snapshot, scheduled_at, not_before_at
				) VALUES ($1,$1,$2,$3,$4,$5,$6,$7,$8,$8)
				RETURNING id`,
				auto.ID, in.UserID, payloadID, st.nextSequence,
				auto.IntervalMinutes, windowJSON, auto.DelaySeconds, scheduled).Scan(&pendingID); err != nil {
				return fmt.Errorf("insert pending: %w", err)
			}

			if _, err := tx.Exec(ctx, `
				UPDATE automation_monitor_dispatch_state
				SET next_sequence = $2,
				    next_available_at = $3,
				    updated_at = transaction_timestamp()
				WHERE automation_id = $1`,
				auto.ID, st.nextSequence+1,
				scheduled.Add(time.Duration(auto.IntervalMinutes)*time.Minute)); err != nil {
				return fmt.Errorf("advance state: %w", err)
			}

			result.Deferred[auto.ID] = pendingID
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// ClaimDueHeads devolve o head DEVIDO de até `limit` automações distintas, já
// em enqueuing. Só o menor sequence_no não terminal concorre — é o que garante
// que um item reagendado bloqueie seus sucessores em vez de ser ultrapassado.
func (r *monitorCadenceRepository) ClaimDueHeads(ctx context.Context, limit int) ([]models.MonitorPendingLease, error) {
	var leases []models.MonitorPendingLease

	err := r.db.WithTx(ctx, func(tx pgx.Tx) error {
		// Trava só os ESTADOS com trabalho devido. SKIP LOCKED: outra réplica
		// pollando ao mesmo tempo pega automações diferentes, nunca a mesma.
		rows, err := tx.Query(ctx, `
			SELECT s.automation_id::text
			FROM automation_monitor_dispatch_state s
			WHERE EXISTS (
				SELECT 1 FROM (
					SELECT p.status, p.not_before_at, p.first_started_at, p.interval_minutes_snapshot
					FROM automation_monitor_pending p
					WHERE p.automation_id_snapshot = s.automation_id
					  AND p.status IN ('pending','enqueuing','enqueued','processing')
					ORDER BY p.sequence_no
					LIMIT 1
				) head
				WHERE head.status = 'pending'
				  AND head.not_before_at <= transaction_timestamp()
				  AND (
				      head.first_started_at IS NOT NULL
				      OR s.last_dispatch_started_at IS NULL
				      OR transaction_timestamp() >=
				         s.last_dispatch_started_at + make_interval(mins => head.interval_minutes_snapshot)
				  )
			)
			ORDER BY s.automation_id
			LIMIT $1
			FOR UPDATE SKIP LOCKED`, limit)
		if err != nil {
			return err
		}
		var automationIDs []string
		for rows.Next() {
			var id string
			if sErr := rows.Scan(&id); sErr != nil {
				rows.Close()
				return sErr
			}
			automationIDs = append(automationIDs, id)
		}
		rows.Close()
		if rErr := rows.Err(); rErr != nil {
			return rErr
		}
		if len(automationIDs) == 0 {
			return nil
		}

		// CAS pending -> enqueuing no head de cada automação travada.
		claimRows, err := tx.Query(ctx, `
			UPDATE automation_monitor_pending t
			SET status = 'enqueuing',
			    enqueue_generation = t.enqueue_generation + 1,
			    enqueue_token = gen_random_uuid(),
			    enqueue_claimed_at = transaction_timestamp(),
			    updated_at = transaction_timestamp()
			FROM (
				SELECT DISTINCT ON (automation_id_snapshot) id
				FROM automation_monitor_pending
				WHERE automation_id_snapshot = ANY($1::uuid[])
				  AND status IN ('pending','enqueuing','enqueued','processing')
				ORDER BY automation_id_snapshot, sequence_no
			) head
			WHERE t.id = head.id AND t.status = 'pending'
			RETURNING t.id, t.automation_id_snapshot::text,
			          t.enqueue_generation, t.enqueue_token::text,
			          t.sequence_no, t.interval_minutes_snapshot`, automationIDs)
		if err != nil {
			return err
		}
		defer claimRows.Close()
		for claimRows.Next() {
			var l models.MonitorPendingLease
			if sErr := claimRows.Scan(&l.PendingID, &l.AutomationID, &l.EnqueueGen,
				&l.EnqueueToken, &l.SequenceNo, &l.IntervalMinutes); sErr != nil {
				return sErr
			}
			leases = append(leases, l)
		}
		return claimRows.Err()
	})
	if err != nil {
		return nil, err
	}
	return leases, nil
}

func (r *monitorCadenceRepository) MarkEnqueued(ctx context.Context, pendingID int64, generation int, token string) error {
	// Zero linhas é legítimo: a task pode já ter executado e movido o item
	// para processing antes do ack do poller.
	return r.db.Exec(ctx, `
		UPDATE automation_monitor_pending
		SET status = 'enqueued', enqueued_at = transaction_timestamp(),
		    updated_at = transaction_timestamp()
		WHERE id = $1 AND enqueue_generation = $2 AND enqueue_token = $3::uuid
		  AND status = 'enqueuing'`, pendingID, generation, token)
}

func (r *monitorCadenceRepository) ReleaseEnqueueLease(ctx context.Context, pendingID int64, generation int, token string, notBefore time.Time) error {
	return r.db.Exec(ctx, `
		UPDATE automation_monitor_pending
		SET status = 'pending', enqueue_token = NULL, enqueue_claimed_at = NULL,
		    not_before_at = $4, updated_at = transaction_timestamp()
		WHERE id = $1 AND enqueue_generation = $2 AND enqueue_token = $3::uuid
		  AND status IN ('enqueuing','enqueued')`, pendingID, generation, token, notBefore)
}

// ClaimPending faz o CAS enqueuing|enqueued -> processing e devolve o item com
// o payload durável já junto. nil,nil = task velha/replay (no-op no worker).
func (r *monitorCadenceRepository) ClaimPending(ctx context.Context, pendingID int64, generation int, token string) (*models.MonitorPendingWork, error) {
	var (
		w             models.MonitorPendingWork
		automationID  *string
		snapshotJSON  []byte
		windowJSON    []byte
		mediaMimetype *string
		mediaFilename *string
		mediaSHA      *string
		mediaSize     *int
		mediaBytes    []byte
	)

	err := r.db.QueryRow(ctx, `
		WITH claimed AS (
			UPDATE automation_monitor_pending
			SET status = 'processing',
			    processing_token = gen_random_uuid(),
			    processing_started_at = transaction_timestamp(),
			    processing_attempts = processing_attempts + 1,
			    updated_at = transaction_timestamp()
			WHERE id = $1 AND enqueue_generation = $2 AND enqueue_token = $3::uuid
			  AND status IN ('enqueuing','enqueued')
			RETURNING id, automation_id, automation_id_snapshot, user_id, payload_id,
			          sequence_no, interval_minutes_snapshot, delay_seconds_snapshot,
			          window_snapshot, processing_token, processing_attempts, first_started_at
		)
		SELECT c.id, c.automation_id::text, c.automation_id_snapshot::text, c.user_id::text,
		       c.payload_id, c.sequence_no, c.interval_minutes_snapshot,
		       c.delay_seconds_snapshot, c.window_snapshot, c.processing_token::text,
		       c.processing_attempts, (c.first_started_at IS NOT NULL),
		       p.message_snapshot, p.media_mimetype, p.media_filename,
		       p.media_sha256, p.media_size_bytes, p.media_bytes
		FROM claimed c
		JOIN automation_monitor_payloads p ON p.id = c.payload_id`,
		pendingID, generation, token).Scan(
		&w.PendingID, &automationID, &w.AutomationSnapshot, &w.UserID,
		&w.PayloadID, &w.SequenceNo, &w.IntervalMinutes,
		&w.DelaySeconds, &windowJSON, &w.ProcessingToken,
		&w.ProcessingAttempts, &w.FirstStarted,
		&snapshotJSON, &mediaMimetype, &mediaFilename,
		&mediaSHA, &mediaSize, &mediaBytes)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	if automationID != nil {
		w.AutomationID = *automationID
	}
	if err := json.Unmarshal(snapshotJSON, &w.Message); err != nil {
		return nil, fmt.Errorf("unmarshal message snapshot: %w", err)
	}
	if len(windowJSON) > 0 {
		if err := json.Unmarshal(windowJSON, &w.Window); err != nil {
			return nil, fmt.Errorf("unmarshal window snapshot: %w", err)
		}
	}
	if len(mediaBytes) > 0 {
		w.Media = &models.MonitorPayloadMedia{Bytes: mediaBytes}
		if mediaMimetype != nil {
			w.Media.Mimetype = *mediaMimetype
		}
		if mediaFilename != nil {
			w.Media.Filename = *mediaFilename
		}
		if mediaSHA != nil {
			w.Media.SHA256 = *mediaSHA
		}
		if mediaSize != nil {
			w.Media.SizeBytes = *mediaSize
		}
	}
	return &w, nil
}

// MarkPendingStarted grava o início REAL do item e, na mesma transação, o
// last_dispatch_started_at do estado. Os dois juntos são o anti-rajada: depois
// de downtime o sucessor conta o intervalo a partir daqui, não do horário
// reservado que já passou.
func (r *monitorCadenceRepository) MarkPendingStarted(ctx context.Context, pendingID int64, processingToken string) error {
	return r.db.Exec(ctx, `
		WITH started AS (
			UPDATE automation_monitor_pending
			SET first_started_at = transaction_timestamp(),
			    updated_at = transaction_timestamp()
			WHERE id = $1 AND processing_token = $2::uuid AND first_started_at IS NULL
			RETURNING automation_id_snapshot
		)
		UPDATE automation_monitor_dispatch_state s
		SET last_dispatch_started_at = transaction_timestamp(),
		    updated_at = transaction_timestamp()
		FROM started
		WHERE s.automation_id = started.automation_id_snapshot`, pendingID, processingToken)
}

func (r *monitorCadenceRepository) ReschedulePending(ctx context.Context, pendingID int64, processingToken string, notBefore time.Time, code string) error {
	return r.db.Exec(ctx, `
		UPDATE automation_monitor_pending
		SET status = 'pending', not_before_at = $3,
		    processing_token = NULL, processing_started_at = NULL,
		    last_error_code = NULLIF($4, ''), updated_at = transaction_timestamp()
		WHERE id = $1 AND processing_token = $2::uuid AND status = 'processing'`,
		pendingID, processingToken, notBefore, code)
}

func (r *monitorCadenceRepository) FinalizePending(ctx context.Context, pendingID int64, processingToken, status string, result models.MonitorPendingResult, code string) error {
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return r.db.Exec(ctx, `
		UPDATE automation_monitor_pending
		SET status = $3, result = $4, last_error_code = NULLIF($5, ''),
		    completed_at = transaction_timestamp(),
		    processing_token = NULL, processing_started_at = NULL,
		    updated_at = transaction_timestamp()
		WHERE id = $1 AND processing_token = $2::uuid AND status = 'processing'`,
		pendingID, processingToken, status, resultJSON, code)
}

// MaterializeDestinations congela a lista de destinos na PRIMEIRA execução.
// Retomada depois de crash relê as mesmas linhas: a ordem do fan-out (que é
// embaralhada por anti-ban) não pode mudar entre tentativas, senão um destino
// já atendido reapareceria em outra posição.
func (r *monitorCadenceRepository) MaterializeDestinations(ctx context.Context, pendingID int64, dests []models.MonitorPendingDestination) ([]models.MonitorPendingDestination, error) {
	var out []models.MonitorPendingDestination

	err := r.db.WithTx(ctx, func(tx pgx.Tx) error {
		existing, err := scanDestinations(ctx, tx, pendingID)
		if err != nil {
			return err
		}
		if len(existing) > 0 {
			out = existing
			return nil
		}
		if len(dests) == 0 {
			return nil
		}

		ordinals := make([]int32, len(dests))
		groups := make([]string, len(dests))
		keys := make([]string, len(dests))
		snaps := make([]string, len(dests))
		for i, d := range dests {
			ordinals[i] = int32(d.Ordinal)
			groups[i] = d.GroupID
			keys[i] = d.CanonicalKey
			if len(d.Snapshot) == 0 {
				snaps[i] = "{}"
			} else {
				snaps[i] = string(d.Snapshot)
			}
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO automation_monitor_pending_destinations (
				pending_id, ordinal, destination_group_id, canonical_key, dispatch_snapshot
			)
			SELECT $1, o.ordinal, o.gid, NULLIF(o.ck, ''), o.snap::jsonb
			FROM UNNEST($2::int[], $3::text[], $4::text[], $5::text[])
			     AS o(ordinal, gid, ck, snap)
			ON CONFLICT DO NOTHING`,
			pendingID, ordinals, groups, keys, snaps); err != nil {
			return fmt.Errorf("insert destinations: %w", err)
		}

		out, err = scanDestinations(ctx, tx, pendingID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *monitorCadenceRepository) ListDestinations(ctx context.Context, pendingID int64) ([]models.MonitorPendingDestination, error) {
	return scanDestinations(ctx, r.db.pool, pendingID)
}

// destinationQuerier abstrai pool e transação para a leitura de destinos.
type destinationQuerier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func scanDestinations(ctx context.Context, q destinationQuerier, pendingID int64) ([]models.MonitorPendingDestination, error) {
	rows, err := q.Query(ctx, `
		SELECT id, pending_id, ordinal, destination_group_id,
		       COALESCE(canonical_key, ''), dispatch_snapshot, status, attempts,
		       not_before_at, COALESCE(outcome_code, '')
		FROM automation_monitor_pending_destinations
		WHERE pending_id = $1
		ORDER BY ordinal`, pendingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.MonitorPendingDestination
	for rows.Next() {
		var d models.MonitorPendingDestination
		if err := rows.Scan(&d.ID, &d.PendingID, &d.Ordinal, &d.GroupID,
			&d.CanonicalKey, &d.Snapshot, &d.Status, &d.Attempts,
			&d.NotBeforeAt, &d.OutcomeCode); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (r *monitorCadenceRepository) ClaimDestination(ctx context.Context, destinationID int64, token string) (bool, error) {
	tag, err := r.db.pool.Exec(ctx, `
		UPDATE automation_monitor_pending_destinations
		SET status = 'claimed', attempts = attempts + 1,
		    claim_token = $2::uuid, claimed_at = transaction_timestamp(),
		    updated_at = transaction_timestamp()
		WHERE id = $1 AND status = 'pending'
		  AND not_before_at <= transaction_timestamp()`, destinationID, token)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// MarkDestinationSending cruza a fronteira do resultado incerto. Depois dela, o
// POST NUNCA é repetido sem prova de que não começou: na dúvida, assumed_sent.
func (r *monitorCadenceRepository) MarkDestinationSending(ctx context.Context, destinationID int64, token string) (bool, error) {
	tag, err := r.db.pool.Exec(ctx, `
		UPDATE automation_monitor_pending_destinations
		SET status = 'sending', sending_at = transaction_timestamp(),
		    updated_at = transaction_timestamp()
		WHERE id = $1 AND status = 'claimed' AND claim_token = $2::uuid`,
		destinationID, token)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (r *monitorCadenceRepository) FinishDestination(ctx context.Context, destinationID int64, status, outcomeCode string, notBefore *time.Time) error {
	return r.db.Exec(ctx, `
		UPDATE automation_monitor_pending_destinations
		SET status = $2,
		    outcome_code = NULLIF($3, ''),
		    not_before_at = COALESCE($4, not_before_at),
		    completed_at = CASE WHEN $2 = 'pending' THEN NULL ELSE transaction_timestamp() END,
		    claim_token = CASE WHEN $2 = 'pending' THEN NULL ELSE claim_token END,
		    claimed_at = CASE WHEN $2 = 'pending' THEN NULL ELSE claimed_at END,
		    updated_at = transaction_timestamp()
		WHERE id = $1`, destinationID, status, outcomeCode, notBefore)
}

// RecoverLeases é o que torna todo crash recuperável sem intervenção. Cada
// passo abaixo corresponde a uma linha da matriz de crash do plano (§14).
func (r *monitorCadenceRepository) RecoverLeases(ctx context.Context) (models.MonitorRecoveryStats, error) {
	var stats models.MonitorRecoveryStats

	// 1. Lease de enqueue expirada: o poller morreu entre o claim e o enqueue.
	//    Volta a pending; a geração é preservada e o próximo claim a incrementa,
	//    então uma task antiga que ressuscite não casa o CAS.
	tag, err := r.db.pool.Exec(ctx, `
		UPDATE automation_monitor_pending
		SET status = 'pending', enqueue_token = NULL, enqueue_claimed_at = NULL,
		    last_error_code = $2, updated_at = transaction_timestamp()
		WHERE status IN ('enqueuing','enqueued')
		  AND enqueue_claimed_at < transaction_timestamp() - $1::interval`,
		monitorEnqueueLeaseTTL, models.MonitorCodeLeaseExpired)
	if err != nil {
		return stats, err
	}
	stats.EnqueueLeasesReleased = int(tag.RowsAffected())

	// 2. Destino reivindicado mas sem POST autorizado: seguro voltar a pending.
	tag, err = r.db.pool.Exec(ctx, `
		UPDATE automation_monitor_pending_destinations
		SET status = 'pending', claim_token = NULL, claimed_at = NULL,
		    updated_at = transaction_timestamp()
		WHERE status = 'claimed'
		  AND claimed_at < transaction_timestamp() - $1::interval`,
		monitorDestClaimLeaseTTL)
	if err != nil {
		return stats, err
	}
	stats.DestinationsReleased = int(tag.RowsAffected())

	// 3. Destino que morreu DEPOIS de autorizar o POST: entrega incerta. Vira
	//    assumed_sent e nunca mais é enviado — duplicar uma promoção no grupo é
	//    pior que registrar um envio incerto (mesma política do caminho vivo).
	tag, err = r.db.pool.Exec(ctx, `
		UPDATE automation_monitor_pending_destinations
		SET status = 'assumed_sent', outcome_code = $2,
		    completed_at = transaction_timestamp(), updated_at = transaction_timestamp()
		WHERE status = 'sending'
		  AND sending_at < transaction_timestamp() - $1::interval`,
		monitorDestSendingLeaseTTL, models.MonitorCodeSendAmbiguous)
	if err != nil {
		return stats, err
	}
	stats.DestinationsAssumed = int(tag.RowsAffected())

	// 4a. Item travado em processing com destino recuperável (ou sem destino
	//     materializado): volta a pending e o poller o reemite.
	tag, err = r.db.pool.Exec(ctx, `
		UPDATE automation_monitor_pending p
		SET status = 'pending',
		    processing_token = NULL, processing_started_at = NULL,
		    not_before_at = GREATEST(p.not_before_at, transaction_timestamp()),
		    last_error_code = $2, updated_at = transaction_timestamp()
		WHERE p.status = 'processing'
		  AND p.processing_started_at < transaction_timestamp() - $1::interval
		  AND (
		      NOT EXISTS (SELECT 1 FROM automation_monitor_pending_destinations d WHERE d.pending_id = p.id)
		      OR EXISTS (
		          SELECT 1 FROM automation_monitor_pending_destinations d
		          WHERE d.pending_id = p.id AND d.status IN ('pending','claimed')
		      )
		  )`, monitorProcessingLeaseTTL, models.MonitorCodeLeaseExpired)
	if err != nil {
		return stats, err
	}
	stats.PendingsRecovered = int(tag.RowsAffected())

	// 4b. Item travado em processing cujos destinos já são TODOS terminais:
	//     fecha pelos contadores (o worker morreu depois do último envio).
	tag, err = r.db.pool.Exec(ctx, `
		UPDATE automation_monitor_pending p
		SET status = CASE WHEN agg.failed_count > 0 THEN 'failed' ELSE 'completed' END,
		    result = jsonb_build_object(
		        'sent_count', agg.sent_count,
		        'assumed_sent_count', agg.assumed_sent_count,
		        'skipped_count', agg.skipped_count,
		        'failed_count', agg.failed_count,
		        'canceled_count', agg.canceled_count),
		    completed_at = transaction_timestamp(),
		    processing_token = NULL, processing_started_at = NULL,
		    updated_at = transaction_timestamp()
		FROM (
		    SELECT d.pending_id,
		           count(*) FILTER (WHERE d.status = 'sent')::int AS sent_count,
		           count(*) FILTER (WHERE d.status = 'assumed_sent')::int AS assumed_sent_count,
		           count(*) FILTER (WHERE d.status = 'skipped')::int AS skipped_count,
		           count(*) FILTER (WHERE d.status = 'failed')::int AS failed_count,
		           count(*) FILTER (WHERE d.status = 'canceled')::int AS canceled_count,
		           count(*) FILTER (WHERE d.status IN ('pending','claimed','sending'))::int AS open_count
		    FROM automation_monitor_pending_destinations d
		    WHERE d.pending_id IN (
		        SELECT id FROM automation_monitor_pending
		        WHERE status = 'processing'
		          AND processing_started_at < transaction_timestamp() - $1::interval
		    )
		    GROUP BY d.pending_id
		) agg
		WHERE p.id = agg.pending_id AND p.status = 'processing' AND agg.open_count = 0`,
		monitorProcessingLeaseTTL)
	if err != nil {
		return stats, err
	}
	stats.PendingsRecovered += int(tag.RowsAffected())

	// 5. Órfão (automação apagada) ou automação que deixou de ser monitoring
	//    ativa: cancela de forma auditável antes de qualquer POST novo.
	var canceled int
	if err := r.db.QueryRow(ctx, `
		WITH gone AS (
			UPDATE automation_monitor_pending p
			SET status = 'canceled', completed_at = transaction_timestamp(),
			    last_error_code = $1, updated_at = transaction_timestamp()
			WHERE p.status IN ('pending','enqueuing','enqueued')
			  AND (
			      p.automation_id IS NULL
			      OR NOT EXISTS (
			          SELECT 1 FROM automations a
			          WHERE a.id = p.automation_id AND a.status = 'active' AND a.type = 'monitoring'
			      )
			  )
			RETURNING id
		), dcan AS (
			UPDATE automation_monitor_pending_destinations d
			SET status = 'canceled', outcome_code = $1,
			    completed_at = transaction_timestamp(), updated_at = transaction_timestamp()
			FROM gone WHERE d.pending_id = gone.id AND d.status IN ('pending','claimed')
			RETURNING d.id
		)
		SELECT (SELECT count(*) FROM gone)::int`, models.MonitorCodeAutomationGone).Scan(&canceled); err != nil {
		return stats, err
	}
	stats.PendingsCanceled = canceled

	// 6. Mídia cujo conjunto de pendings já é todo terminal: zera os bytes na
	//    hora (não espera a retenção de 30 dias). BYTEA é o item mais caro do
	//    ledger e o único que cresce com o tráfego de imagem.
	tag, err = r.db.pool.Exec(ctx, `
		UPDATE automation_monitor_payloads pay
		SET media_bytes = NULL, media_size_bytes = NULL, media_sha256 = NULL,
		    media_purged_at = transaction_timestamp()
		WHERE pay.media_bytes IS NOT NULL AND pay.media_purged_at IS NULL
		  AND NOT EXISTS (
		      SELECT 1 FROM automation_monitor_pending p
		      WHERE p.payload_id = pay.id
		        AND p.status IN ('pending','enqueuing','enqueued','processing')
		  )`)
	if err != nil {
		return stats, err
	}
	stats.MediaPurged = int(tag.RowsAffected())

	return stats, nil
}

func (r *monitorCadenceRepository) CancelForAutomation(ctx context.Context, automationID string) (int, error) {
	var canceled int
	err := r.db.QueryRow(ctx, `
		WITH gone AS (
			UPDATE automation_monitor_pending
			SET status = 'canceled', completed_at = transaction_timestamp(),
			    last_error_code = $2, updated_at = transaction_timestamp()
			WHERE automation_id_snapshot = $1
			  AND status IN ('pending','enqueuing','enqueued')
			RETURNING id
		), dcan AS (
			UPDATE automation_monitor_pending_destinations d
			SET status = 'canceled', outcome_code = $2,
			    completed_at = transaction_timestamp(), updated_at = transaction_timestamp()
			FROM gone WHERE d.pending_id = gone.id AND d.status IN ('pending','claimed')
			RETURNING d.id
		)
		SELECT (SELECT count(*) FROM gone)::int`,
		automationID, models.MonitorCodeAutomationGone).Scan(&canceled)
	return canceled, err
}

// Cleanup apaga itens terminais antigos em lotes. NUNCA toca não terminal: a
// fila viva é intocável mesmo que alguém chame isto com uma data errada.
func (r *monitorCadenceRepository) Cleanup(ctx context.Context, before time.Time, batch int) (int, error) {
	var deleted int
	if err := r.db.QueryRow(ctx, `
		WITH victims AS (
			SELECT id FROM automation_monitor_pending
			WHERE status IN ('completed','failed','canceled')
			  AND completed_at IS NOT NULL AND completed_at < $1
			ORDER BY completed_at
			LIMIT $2
		), gone AS (
			DELETE FROM automation_monitor_pending p
			USING victims v WHERE p.id = v.id
			RETURNING p.id
		)
		SELECT (SELECT count(*) FROM gone)::int`, before, batch).Scan(&deleted); err != nil {
		return 0, err
	}

	// Payload sem nenhum pending restante não tem mais dono: sai junto.
	if err := r.db.Exec(ctx, `
		WITH orphans AS (
			SELECT pay.id FROM automation_monitor_payloads pay
			WHERE pay.created_at < $1
			  AND NOT EXISTS (
			      SELECT 1 FROM automation_monitor_pending p WHERE p.payload_id = pay.id
			  )
			ORDER BY pay.created_at
			LIMIT $2
		)
		DELETE FROM automation_monitor_payloads pay
		USING orphans o WHERE pay.id = o.id`, before, batch); err != nil {
		return deleted, err
	}
	return deleted, nil
}

// GetMonitorQueueSummaries devolve contagem + próximo horário do head para as
// automações do usuário, em UMA query (a listagem chama isto uma vez por
// página, nunca por automação).
func (r *monitorCadenceRepository) GetMonitorQueueSummaries(ctx context.Context, userID string, automationIDs []string) (map[string]models.MonitorQueueSummary, error) {
	out := make(map[string]models.MonitorQueueSummary)
	if userID == "" || len(automationIDs) == 0 {
		return out, nil
	}

	rows, err := r.db.Query(ctx, `
		WITH q AS (
			SELECT p.automation_id_snapshot AS aid, p.sequence_no, p.not_before_at,
			       p.first_started_at, p.interval_minutes_snapshot
			FROM automation_monitor_pending p
			WHERE p.user_id = $1
			  AND p.automation_id_snapshot = ANY($2::uuid[])
			  AND p.status IN ('pending','enqueuing','enqueued','processing')
		), head AS (
			SELECT DISTINCT ON (aid) aid, not_before_at, first_started_at, interval_minutes_snapshot
			FROM q ORDER BY aid, sequence_no
		)
		SELECT q.aid::text, count(*)::int,
		       CASE
		           WHEN head.first_started_at IS NOT NULL THEN head.not_before_at
		           ELSE GREATEST(
		               head.not_before_at,
		               COALESCE(s.last_dispatch_started_at
		                        + make_interval(mins => head.interval_minutes_snapshot),
		                        head.not_before_at))
		       END
		FROM q
		JOIN head ON head.aid = q.aid
		LEFT JOIN automation_monitor_dispatch_state s ON s.automation_id = q.aid
		GROUP BY q.aid, head.first_started_at, head.not_before_at,
		         head.interval_minutes_snapshot, s.last_dispatch_started_at`,
		userID, automationIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var aid string
		var count int
		var next time.Time
		if err := rows.Scan(&aid, &count, &next); err != nil {
			return nil, err
		}
		nextCopy := next
		out[aid] = models.MonitorQueueSummary{PendingCount: count, NextScheduledAt: &nextCopy}
	}
	return out, rows.Err()
}
