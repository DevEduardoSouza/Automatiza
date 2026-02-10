package postgres

import (
	"automatiza-go/internal/repository"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

// CheckpointStatus constants
const (
	CheckpointStatusIdle    = "idle"
	CheckpointStatusRunning = "running"
	CheckpointStatusFailed  = "failed"
)

// ErrCheckpointAlreadyRunning is returned when trying to mark a checkpoint as running
// that is already in running status
var ErrCheckpointAlreadyRunning = errors.New("checkpoint is already running")

// reconciliationCheckpointRepository handles checkpoint persistence
type reconciliationCheckpointRepository struct {
	db     *DB
	logger *zap.Logger
}

// NewReconciliationCheckpointRepository creates a new repository
func NewReconciliationCheckpointRepository(db *DB, logger *zap.Logger) repository.ReconciliationCheckpointRepository {
	return &reconciliationCheckpointRepository{
		db:     db,
		logger: logger,
	}
}

// GetOrCreate gets an existing checkpoint or creates a new one with timestamp 0
func (r *reconciliationCheckpointRepository) GetOrCreate(ctx context.Context, sessionName, chatID string) (*repository.ReconciliationCheckpoint, error) {
	// Try to insert first with ON CONFLICT DO NOTHING
	insertQuery := `
		INSERT INTO reconciliation_checkpoints (session_name, chat_id, last_message_timestamp, status, last_run_at)
		VALUES ($1, $2, 0, $3, NOW())
		ON CONFLICT (session_name, chat_id) DO NOTHING
	`

	_, err := r.db.pool.Exec(ctx, insertQuery, sessionName, chatID, CheckpointStatusIdle)
	if err != nil {
		r.logger.Error("Failed to insert checkpoint",
			zap.String("sessionName", sessionName),
			zap.String("chatID", chatID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to insert checkpoint: %w", err)
	}

	// Now select the row (either newly created or existing)
	return r.Get(ctx, sessionName, chatID)
}

// Get gets a checkpoint, returns nil if not found
func (r *reconciliationCheckpointRepository) Get(ctx context.Context, sessionName, chatID string) (*repository.ReconciliationCheckpoint, error) {
	query := `
		SELECT id, session_name, chat_id, last_message_timestamp, last_message_id, last_run_at, status
		FROM reconciliation_checkpoints
		WHERE session_name = $1 AND chat_id = $2
	`

	var cp repository.ReconciliationCheckpoint
	err := r.db.QueryRow(ctx, query, sessionName, chatID).Scan(
		&cp.ID,
		&cp.SessionName,
		&cp.ChatID,
		&cp.LastMessageTimestamp,
		&cp.LastMessageID,
		&cp.LastRunAt,
		&cp.Status,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		r.logger.Error("Failed to get checkpoint",
			zap.String("sessionName", sessionName),
			zap.String("chatID", chatID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get checkpoint: %w", err)
	}

	return &cp, nil
}

// MarkRunning marks a checkpoint as running (for exclusive lock)
// Returns ErrCheckpointAlreadyRunning if already running
func (r *reconciliationCheckpointRepository) MarkRunning(ctx context.Context, sessionName, chatID string) error {
	// Atomic check: only update if status is idle
	query := `
		UPDATE reconciliation_checkpoints
		SET status = $1, last_run_at = NOW()
		WHERE session_name = $2 AND chat_id = $3 AND status = $4
		RETURNING id
	`

	var id int
	err := r.db.QueryRow(ctx, query, CheckpointStatusRunning, sessionName, chatID, CheckpointStatusIdle).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Either doesn't exist or is already running
			cp, getErr := r.Get(ctx, sessionName, chatID)
			if getErr != nil {
				return getErr
			}
			if cp == nil {
				return fmt.Errorf("checkpoint not found for session %s, chat %s", sessionName, chatID)
			}
			// Checkpoint exists but not idle, so it's already running or failed
			return ErrCheckpointAlreadyRunning
		}
		r.logger.Error("Failed to mark checkpoint as running",
			zap.String("sessionName", sessionName),
			zap.String("chatID", chatID),
			zap.Error(err))
		return fmt.Errorf("failed to mark checkpoint as running: %w", err)
	}

	return nil
}

// MarkIdle marks a checkpoint as idle
func (r *reconciliationCheckpointRepository) MarkIdle(ctx context.Context, sessionName, chatID string) error {
	return r.updateStatus(ctx, sessionName, chatID, CheckpointStatusIdle)
}

// MarkFailed marks a checkpoint as failed
func (r *reconciliationCheckpointRepository) MarkFailed(ctx context.Context, sessionName, chatID string) error {
	return r.updateStatus(ctx, sessionName, chatID, CheckpointStatusFailed)
}

// updateStatus is an internal helper to update checkpoint status
func (r *reconciliationCheckpointRepository) updateStatus(ctx context.Context, sessionName, chatID, status string) error {
	query := `
		UPDATE reconciliation_checkpoints
		SET status = $1, last_run_at = NOW()
		WHERE session_name = $2 AND chat_id = $3
	`

	result, err := r.db.pool.Exec(ctx, query, status, sessionName, chatID)
	if err != nil {
		r.logger.Error("Failed to update checkpoint status",
			zap.String("sessionName", sessionName),
			zap.String("chatID", chatID),
			zap.String("status", status),
			zap.Error(err))
		return fmt.Errorf("failed to update checkpoint status: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("checkpoint not found for session %s, chat %s", sessionName, chatID)
	}

	return nil
}

// UpdateCheckpoint updates the checkpoint with new timestamp and message ID
func (r *reconciliationCheckpointRepository) UpdateCheckpoint(ctx context.Context, sessionName, chatID string, lastTimestamp int64, lastMessageID string) error {
	query := `
		UPDATE reconciliation_checkpoints
		SET last_message_timestamp = $1, last_message_id = $2, last_run_at = NOW()
		WHERE session_name = $3 AND chat_id = $4
	`

	result, err := r.db.pool.Exec(ctx, query, lastTimestamp, lastMessageID, sessionName, chatID)
	if err != nil {
		r.logger.Error("Failed to update checkpoint",
			zap.String("sessionName", sessionName),
			zap.String("chatID", chatID),
			zap.Int64("lastTimestamp", lastTimestamp),
			zap.String("lastMessageID", lastMessageID),
			zap.Error(err))
		return fmt.Errorf("failed to update checkpoint: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("checkpoint not found for session %s, chat %s", sessionName, chatID)
	}

	return nil
}

// GetBySession gets all checkpoints for a session
func (r *reconciliationCheckpointRepository) GetBySession(ctx context.Context, sessionName string) ([]repository.ReconciliationCheckpoint, error) {
	query := `
		SELECT id, session_name, chat_id, last_message_timestamp, last_message_id, last_run_at, status
		FROM reconciliation_checkpoints
		WHERE session_name = $1
		ORDER BY last_run_at DESC
	`

	rows, err := r.db.Query(ctx, query, sessionName)
	if err != nil {
		r.logger.Error("Failed to get checkpoints by session",
			zap.String("sessionName", sessionName),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get checkpoints by session: %w", err)
	}
	defer rows.Close()

	return r.scanCheckpoints(rows)
}

// GetActive gets all checkpoints that are running (for recovery after restart)
func (r *reconciliationCheckpointRepository) GetActive(ctx context.Context) ([]repository.ReconciliationCheckpoint, error) {
	query := `
		SELECT id, session_name, chat_id, last_message_timestamp, last_message_id, last_run_at, status
		FROM reconciliation_checkpoints
		WHERE status = $1
		ORDER BY last_run_at ASC
	`

	rows, err := r.db.Query(ctx, query, CheckpointStatusRunning)
	if err != nil {
		r.logger.Error("Failed to get active checkpoints",
			zap.Error(err))
		return nil, fmt.Errorf("failed to get active checkpoints: %w", err)
	}
	defer rows.Close()

	return r.scanCheckpoints(rows)
}

// GetStale gets checkpoints that haven't been updated in a while
func (r *reconciliationCheckpointRepository) GetStale(ctx context.Context, olderThan time.Duration) ([]repository.ReconciliationCheckpoint, error) {
	cutoff := time.Now().Add(-olderThan)

	query := `
		SELECT id, session_name, chat_id, last_message_timestamp, last_message_id, last_run_at, status
		FROM reconciliation_checkpoints
		WHERE status = $1 AND last_run_at < $2
		ORDER BY last_run_at ASC
	`

	rows, err := r.db.Query(ctx, query, CheckpointStatusRunning, cutoff)
	if err != nil {
		r.logger.Error("Failed to get stale checkpoints",
			zap.Duration("olderThan", olderThan),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get stale checkpoints: %w", err)
	}
	defer rows.Close()

	return r.scanCheckpoints(rows)
}

// Delete deletes a checkpoint
func (r *reconciliationCheckpointRepository) Delete(ctx context.Context, sessionName, chatID string) error {
	query := `
		DELETE FROM reconciliation_checkpoints
		WHERE session_name = $1 AND chat_id = $2
	`

	result, err := r.db.pool.Exec(ctx, query, sessionName, chatID)
	if err != nil {
		r.logger.Error("Failed to delete checkpoint",
			zap.String("sessionName", sessionName),
			zap.String("chatID", chatID),
			zap.Error(err))
		return fmt.Errorf("failed to delete checkpoint: %w", err)
	}

	if result.RowsAffected() == 0 {
		r.logger.Debug("No checkpoint found to delete",
			zap.String("sessionName", sessionName),
			zap.String("chatID", chatID))
	}

	return nil
}

// DeleteBySession deletes all checkpoints for a session
func (r *reconciliationCheckpointRepository) DeleteBySession(ctx context.Context, sessionName string) (int64, error) {
	query := `
		DELETE FROM reconciliation_checkpoints
		WHERE session_name = $1
	`

	result, err := r.db.pool.Exec(ctx, query, sessionName)
	if err != nil {
		r.logger.Error("Failed to delete checkpoints by session",
			zap.String("sessionName", sessionName),
			zap.Error(err))
		return 0, fmt.Errorf("failed to delete checkpoints by session: %w", err)
	}

	return result.RowsAffected(), nil
}

// ResetStaleCheckpoints resets checkpoints stuck in running status back to idle
func (r *reconciliationCheckpointRepository) ResetStaleCheckpoints(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)

	query := `
		UPDATE reconciliation_checkpoints
		SET status = $1, last_run_at = NOW()
		WHERE status = $2 AND last_run_at < $3
	`

	result, err := r.db.pool.Exec(ctx, query, CheckpointStatusIdle, CheckpointStatusRunning, cutoff)
	if err != nil {
		r.logger.Error("Failed to reset stale checkpoints",
			zap.Duration("olderThan", olderThan),
			zap.Error(err))
		return 0, fmt.Errorf("failed to reset stale checkpoints: %w", err)
	}

	count := result.RowsAffected()
	if count > 0 {
		r.logger.Info("Reset stale checkpoints",
			zap.Int64("count", count),
			zap.Duration("olderThan", olderThan))
	}

	return count, nil
}

// scanCheckpoints is a helper to scan multiple checkpoint rows
func (r *reconciliationCheckpointRepository) scanCheckpoints(rows pgx.Rows) ([]repository.ReconciliationCheckpoint, error) {
	var checkpoints []repository.ReconciliationCheckpoint

	for rows.Next() {
		var cp repository.ReconciliationCheckpoint
		err := rows.Scan(
			&cp.ID,
			&cp.SessionName,
			&cp.ChatID,
			&cp.LastMessageTimestamp,
			&cp.LastMessageID,
			&cp.LastRunAt,
			&cp.Status,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan checkpoint: %w", err)
		}
		checkpoints = append(checkpoints, cp)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating checkpoints: %w", err)
	}

	return checkpoints, nil
}
