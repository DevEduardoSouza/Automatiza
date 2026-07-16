// Package postgres provides PostgreSQL implementations of repository interfaces.
// This file implements CheckpointRepository for reconciliation checkpoint storage.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"automatiza-go/internal/services/reconciliation"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

// Compile-time check that checkpointRepository implements reconciliation.CheckpointRepository
var _ reconciliation.CheckpointRepository = (*checkpointRepository)(nil)

// checkpointRepository implements reconciliation.CheckpointRepository using PostgreSQL.
type checkpointRepository struct {
	db     *DB
	logger *zap.Logger
}

// NewCheckpointRepository creates a new PostgreSQL checkpoint repository.
func NewCheckpointRepository(db *DB, logger *zap.Logger) reconciliation.CheckpointRepository {
	return &checkpointRepository{
		db:     db,
		logger: logger.Named("checkpoint-repo"),
	}
}

// GetOrCreate retrieves an existing checkpoint or creates a new one with default values.
func (r *checkpointRepository) GetOrCreate(ctx context.Context, sessionName, chatID string) (*reconciliation.Checkpoint, error) {
	// Try to get existing checkpoint first
	checkpoint, err := r.get(ctx, sessionName, chatID)
	if err == nil {
		return checkpoint, nil
	}

	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("failed to get checkpoint: %w", err)
	}

	// Create new checkpoint with default values
	now := time.Now().UTC()
	query := `
		INSERT INTO reconciliation_checkpoints (session_name, chat_id, last_message_timestamp, last_run_at, status)
		VALUES ($1, $2, 0, $3, 'idle')
		ON CONFLICT (session_name, chat_id) DO NOTHING
		RETURNING id, session_name, chat_id, last_message_timestamp, last_message_id, last_run_at, status
	`

	var cp reconciliation.Checkpoint
	var id int
	var lastMessageID *string
	var status string

	err = r.db.QueryRow(ctx, query, sessionName, chatID, now).Scan(
		&id, &cp.SessionName, &cp.ChatID, &cp.LastMessageTimestamp, &lastMessageID, &cp.LastRunAt, &status,
	)
	if err != nil {
		// If insert failed due to conflict, try to get the existing row
		if errors.Is(err, pgx.ErrNoRows) {
			return r.get(ctx, sessionName, chatID)
		}
		return nil, fmt.Errorf("failed to create checkpoint: %w", err)
	}

	if lastMessageID != nil {
		cp.LastMessageID = *lastMessageID
	}
	cp.Status = reconciliation.CheckpointStatus(status)

	r.logger.Debug("Created new checkpoint",
		zap.String("session", sessionName),
		zap.String("chat", chatID))

	return &cp, nil
}

// get retrieves a checkpoint by session and chat ID.
func (r *checkpointRepository) get(ctx context.Context, sessionName, chatID string) (*reconciliation.Checkpoint, error) {
	query := `
		SELECT session_name, chat_id, last_message_timestamp, last_message_id, last_run_at, status
		FROM reconciliation_checkpoints
		WHERE session_name = $1 AND chat_id = $2
	`

	var cp reconciliation.Checkpoint
	var lastMessageID *string
	var status string

	err := r.db.QueryRow(ctx, query, sessionName, chatID).Scan(
		&cp.SessionName, &cp.ChatID, &cp.LastMessageTimestamp, &lastMessageID, &cp.LastRunAt, &status,
	)
	if err != nil {
		return nil, err
	}

	if lastMessageID != nil {
		cp.LastMessageID = *lastMessageID
	}
	cp.Status = reconciliation.CheckpointStatus(status)

	return &cp, nil
}

// MarkRunning updates the checkpoint status to "running" if not already running.
func (r *checkpointRepository) MarkRunning(ctx context.Context, sessionName, chatID string) error {
	query := `
		UPDATE reconciliation_checkpoints
		SET status = 'running', last_run_at = $3
		WHERE session_name = $1 AND chat_id = $2 AND status != 'running'
	`

	result, err := r.db.pool.Exec(ctx, query, sessionName, chatID, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("failed to mark checkpoint as running: %w", err)
	}

	if result.RowsAffected() == 0 {
		// Check if it's because checkpoint doesn't exist or is already running
		existing, err := r.get(ctx, sessionName, chatID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return reconciliation.ErrNoCheckpoint
			}
			return err
		}
		if existing.Status == reconciliation.CheckpointStatusRunning {
			return reconciliation.ErrAlreadyRunning
		}
	}

	r.logger.Debug("Marked checkpoint as running",
		zap.String("session", sessionName),
		zap.String("chat", chatID))

	return nil
}

// MarkIdle updates the checkpoint status to "idle" after successful reconciliation.
func (r *checkpointRepository) MarkIdle(ctx context.Context, sessionName, chatID string) error {
	query := `
		UPDATE reconciliation_checkpoints
		SET status = 'idle', last_run_at = $3
		WHERE session_name = $1 AND chat_id = $2
	`

	_, err := r.db.pool.Exec(ctx, query, sessionName, chatID, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("failed to mark checkpoint as idle: %w", err)
	}

	r.logger.Debug("Marked checkpoint as idle",
		zap.String("session", sessionName),
		zap.String("chat", chatID))

	return nil
}

// MarkFailed updates the checkpoint status to "failed" after reconciliation error.
func (r *checkpointRepository) MarkFailed(ctx context.Context, sessionName, chatID string) error {
	query := `
		UPDATE reconciliation_checkpoints
		SET status = 'failed', last_run_at = $3
		WHERE session_name = $1 AND chat_id = $2
	`

	_, err := r.db.pool.Exec(ctx, query, sessionName, chatID, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("failed to mark checkpoint as failed: %w", err)
	}

	r.logger.Debug("Marked checkpoint as failed",
		zap.String("session", sessionName),
		zap.String("chat", chatID))

	return nil
}

// UpdateCheckpoint updates the last processed message timestamp and ID.
// MONOTÔNICO: nunca recua o watermark — o caminho vivo (AdvanceCheckpoint) e o
// backfill (este método) correm em paralelo, e um batch atrasado do backfill
// não pode sobrescrever um timestamp mais novo já registrado pelo caminho vivo.
func (r *checkpointRepository) UpdateCheckpoint(ctx context.Context, sessionName, chatID string, lastTimestamp int64, lastMessageID string) error {
	query := `
		UPDATE reconciliation_checkpoints
		SET last_message_id = CASE WHEN $3 > last_message_timestamp THEN $4 ELSE last_message_id END,
		    last_message_timestamp = GREATEST(last_message_timestamp, $3),
		    last_run_at = $5
		WHERE session_name = $1 AND chat_id = $2
	`

	_, err := r.db.pool.Exec(ctx, query, sessionName, chatID, lastTimestamp, lastMessageID, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("failed to update checkpoint: %w", err)
	}

	r.logger.Debug("Updated checkpoint",
		zap.String("session", sessionName),
		zap.String("chat", chatID),
		zap.Int64("timestamp", lastTimestamp),
		zap.String("messageID", lastMessageID))

	return nil
}

// AdvanceCheckpoint faz o upsert MONOTÔNICO do watermark a partir do caminho
// VIVO (fan-out do espelho concluído): cria o checkpoint na primeira mensagem
// do chat e só avança timestamps — nunca recua. É este método que dá vida à
// reconciliação: sem checkpoints criados pelo fluxo real, o watchdog de
// silêncio e o backfill de startup não tinham o que reconciliar (a tabela
// ficou 0 linhas desde sempre) e a "recuperação de gaps" era um no-op.
func (r *checkpointRepository) AdvanceCheckpoint(ctx context.Context, sessionName, chatID string, lastTimestamp int64, lastMessageID string) error {
	query := `
		INSERT INTO reconciliation_checkpoints
			(session_name, chat_id, last_message_timestamp, last_message_id, status, last_run_at)
		VALUES ($1, $2, $3, $4, 'idle', $5)
		ON CONFLICT (session_name, chat_id) DO UPDATE SET
			last_message_id = CASE
				WHEN EXCLUDED.last_message_timestamp > reconciliation_checkpoints.last_message_timestamp
				THEN EXCLUDED.last_message_id
				ELSE reconciliation_checkpoints.last_message_id END,
			last_message_timestamp = GREATEST(reconciliation_checkpoints.last_message_timestamp, EXCLUDED.last_message_timestamp),
			last_run_at = EXCLUDED.last_run_at
	`

	_, err := r.db.pool.Exec(ctx, query, sessionName, chatID, lastTimestamp, lastMessageID, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("failed to advance checkpoint: %w", err)
	}
	return nil
}

// ResetStuckCheckpoints devolve para 'idle' checkpoints presos em
// 'running'/'failed' há mais de olderThan. Sem isso, um crash no meio de um
// ReconcileChat deixava o checkpoint em 'running' e o MarkRunning recusava
// TODA reconciliação futura daquele chat — o safety-net morria em silêncio.
func (r *checkpointRepository) ResetStuckCheckpoints(ctx context.Context, olderThan time.Duration) (int64, error) {
	query := `
		UPDATE reconciliation_checkpoints
		SET status = 'idle'
		WHERE status IN ('running', 'failed') AND last_run_at < $1
	`

	result, err := r.db.pool.Exec(ctx, query, time.Now().UTC().Add(-olderThan))
	if err != nil {
		return 0, fmt.Errorf("failed to reset stuck checkpoints: %w", err)
	}
	return result.RowsAffected(), nil
}

// DeleteInactiveCheckpoints remove checkpoints sem atividade há mais de
// olderThan (grupo morreu, sessão despareada). Higiene: mantém a tabela e o
// custo do reconcileSession proporcionais aos chats realmente ativos.
func (r *checkpointRepository) DeleteInactiveCheckpoints(ctx context.Context, olderThan time.Duration) (int64, error) {
	query := `DELETE FROM reconciliation_checkpoints WHERE last_run_at < $1`

	result, err := r.db.pool.Exec(ctx, query, time.Now().UTC().Add(-olderThan))
	if err != nil {
		return 0, fmt.Errorf("failed to delete inactive checkpoints: %w", err)
	}
	return result.RowsAffected(), nil
}

// GetActiveSessionsChats retrieves all session/chat pairs that have checkpoints.
func (r *checkpointRepository) GetActiveSessionsChats(ctx context.Context) ([]reconciliation.SessionChat, error) {
	query := `
		SELECT session_name, chat_id
		FROM reconciliation_checkpoints
		ORDER BY last_run_at DESC
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query active sessions: %w", err)
	}
	defer rows.Close()

	var result []reconciliation.SessionChat
	for rows.Next() {
		var sc reconciliation.SessionChat
		if err := rows.Scan(&sc.SessionName, &sc.ChatID); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		result = append(result, sc)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return result, nil
}

// GetBySession retrieves all checkpoints for a specific session.
func (r *checkpointRepository) GetBySession(ctx context.Context, sessionName string) ([]reconciliation.Checkpoint, error) {
	query := `
		SELECT session_name, chat_id, last_message_timestamp, last_message_id, last_run_at, status
		FROM reconciliation_checkpoints
		WHERE session_name = $1
		ORDER BY last_run_at DESC
	`

	rows, err := r.db.Query(ctx, query, sessionName)
	if err != nil {
		return nil, fmt.Errorf("failed to query session checkpoints: %w", err)
	}
	defer rows.Close()

	var result []reconciliation.Checkpoint
	for rows.Next() {
		var cp reconciliation.Checkpoint
		var lastMessageID *string
		var status string

		if err := rows.Scan(
			&cp.SessionName, &cp.ChatID, &cp.LastMessageTimestamp, &lastMessageID, &cp.LastRunAt, &status,
		); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		if lastMessageID != nil {
			cp.LastMessageID = *lastMessageID
		}
		cp.Status = reconciliation.CheckpointStatus(status)

		result = append(result, cp)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return result, nil
}

// GetFailedCheckpoints retrieves all checkpoints with failed status.
// Useful for retry logic and monitoring.
func (r *checkpointRepository) GetFailedCheckpoints(ctx context.Context) ([]reconciliation.Checkpoint, error) {
	query := `
		SELECT session_name, chat_id, last_message_timestamp, last_message_id, last_run_at, status
		FROM reconciliation_checkpoints
		WHERE status = 'failed'
		ORDER BY last_run_at ASC
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query failed checkpoints: %w", err)
	}
	defer rows.Close()

	var result []reconciliation.Checkpoint
	for rows.Next() {
		var cp reconciliation.Checkpoint
		var lastMessageID *string
		var status string

		if err := rows.Scan(
			&cp.SessionName, &cp.ChatID, &cp.LastMessageTimestamp, &lastMessageID, &cp.LastRunAt, &status,
		); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		if lastMessageID != nil {
			cp.LastMessageID = *lastMessageID
		}
		cp.Status = reconciliation.CheckpointStatus(status)

		result = append(result, cp)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return result, nil
}

// DeleteBySession removes all checkpoints for a session.
// Used when a session is deleted.
func (r *checkpointRepository) DeleteBySession(ctx context.Context, sessionName string) error {
	query := `DELETE FROM reconciliation_checkpoints WHERE session_name = $1`

	result, err := r.db.pool.Exec(ctx, query, sessionName)
	if err != nil {
		return fmt.Errorf("failed to delete session checkpoints: %w", err)
	}

	r.logger.Info("Deleted session checkpoints",
		zap.String("session", sessionName),
		zap.Int64("count", result.RowsAffected()))

	return nil
}

// DeleteByChat removes a specific checkpoint.
func (r *checkpointRepository) DeleteByChat(ctx context.Context, sessionName, chatID string) error {
	query := `DELETE FROM reconciliation_checkpoints WHERE session_name = $1 AND chat_id = $2`

	_, err := r.db.pool.Exec(ctx, query, sessionName, chatID)
	if err != nil {
		return fmt.Errorf("failed to delete checkpoint: %w", err)
	}

	r.logger.Debug("Deleted checkpoint",
		zap.String("session", sessionName),
		zap.String("chat", chatID))

	return nil
}

// GetStats returns statistics about reconciliation checkpoints.
func (r *checkpointRepository) GetStats(ctx context.Context) (map[string]int, error) {
	query := `
		SELECT status, COUNT(*) as count
		FROM reconciliation_checkpoints
		GROUP BY status
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query checkpoint stats: %w", err)
	}
	defer rows.Close()

	stats := make(map[string]int)
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		stats[status] = count
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return stats, nil
}
