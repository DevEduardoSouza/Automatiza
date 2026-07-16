package postgres

import (
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

type automationRunRepository struct {
	db     *DB
	logger *zap.Logger
}

// NewAutomationRunRepository creates a new PostgreSQL automation run repository
func NewAutomationRunRepository(db *DB, logger *zap.Logger) repository.AutomationRunRepository {
	return &automationRunRepository{
		db:     db,
		logger: logger,
	}
}

// GetLock retrieves the current lock status for an automation
// Returns nil if no active lock exists
func (r *automationRunRepository) GetLock(ctx context.Context, automationID string) (*time.Time, error) {
	query := `
		SELECT lock_until
		FROM automation_runs
		WHERE automation_id = $1 AND lock_until > NOW()
		ORDER BY lock_until DESC
		LIMIT 1
	`

	var lockUntil time.Time
	err := r.db.QueryRow(ctx, query, automationID).Scan(&lockUntil)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // No active lock
		}
		r.logger.Error("failed to get lock",
			zap.String("automationID", automationID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get lock for automation %s: %w", automationID, err)
	}

	return &lockUntil, nil
}

// AcquireLock attempts to acquire an execution lock for the specified duration
// Returns the lock expiry time or nil if lock cannot be acquired (already locked)
// Uses SELECT FOR UPDATE pattern to prevent race conditions
func (r *automationRunRepository) AcquireLock(ctx context.Context, automationID string, duration time.Duration) (*time.Time, error) {
	// Use a transaction with advisory lock to prevent race conditions
	var lockUntil *time.Time

	err := r.db.WithTx(ctx, func(tx pgx.Tx) error {
		// First, check if there's an active lock using SELECT FOR UPDATE
		checkQuery := `
			SELECT lock_until
			FROM automation_runs
			WHERE automation_id = $1 AND lock_until > NOW()
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		`

		var existingLock time.Time
		err := tx.QueryRow(ctx, checkQuery, automationID).Scan(&existingLock)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("failed to check existing lock: %w", err)
		}

		// If a lock exists, we cannot acquire
		if err == nil {
			r.logger.Debug("lock already held",
				zap.String("automationID", automationID),
				zap.Time("lockUntil", existingLock))
			return nil // Lock already held, lockUntil stays nil
		}

		// No active lock, create a new run with lock
		now := time.Now().UTC()
		lockExpiry := now.Add(duration)

		insertQuery := `
			INSERT INTO automation_runs (automation_id, status, lock_until, started_at)
			VALUES ($1, $2, $3, $4)
			RETURNING lock_until
		`

		err = tx.QueryRow(ctx, insertQuery, automationID, models.AutomationRunStatusRunning, lockExpiry, now).Scan(&lockExpiry)
		if err != nil {
			return fmt.Errorf("failed to insert automation run: %w", err)
		}

		lockUntil = &lockExpiry
		return nil
	})

	if err != nil {
		r.logger.Error("failed to acquire lock",
			zap.String("automationID", automationID),
			zap.Duration("duration", duration),
			zap.Error(err))
		return nil, fmt.Errorf("failed to acquire lock for automation %s: %w", automationID, err)
	}

	if lockUntil != nil {
		r.logger.Debug("lock acquired",
			zap.String("automationID", automationID),
			zap.Time("lockUntil", *lockUntil))
	}

	return lockUntil, nil
}

// ReleaseLock releases the execution lock for an automation
func (r *automationRunRepository) ReleaseLock(ctx context.Context, automationID string) error {
	query := `
		UPDATE automation_runs
		SET lock_until = NULL, completed_at = NOW(), status = $1
		WHERE automation_id = $2 AND lock_until > NOW()
	`

	result, err := r.db.pool.Exec(ctx, query, models.AutomationRunStatusCompleted, automationID)
	if err != nil {
		r.logger.Error("failed to release lock",
			zap.String("automationID", automationID),
			zap.Error(err))
		return fmt.Errorf("failed to release lock for automation %s: %w", automationID, err)
	}

	rowsAffected := result.RowsAffected()
	r.logger.Debug("lock released",
		zap.String("automationID", automationID),
		zap.Int64("rowsAffected", rowsAffected))

	return nil
}

// CreateRun creates a new automation run record
func (r *automationRunRepository) CreateRun(ctx context.Context, automationID string, status string) (string, error) {
	now := time.Now().UTC()

	query := `
		INSERT INTO automation_runs (automation_id, status, started_at)
		VALUES ($1, $2, $3)
		RETURNING id
	`

	var runID string
	err := r.db.QueryRow(ctx, query, automationID, status, now).Scan(&runID)
	if err != nil {
		r.logger.Error("failed to create automation run",
			zap.String("automationID", automationID),
			zap.String("status", status),
			zap.Error(err))
		return "", fmt.Errorf("failed to create automation run: %w", err)
	}

	r.logger.Debug("automation run created",
		zap.String("runID", runID),
		zap.String("automationID", automationID),
		zap.String("status", status))

	return runID, nil
}

// UpdateRunStatus updates the status of an automation run
func (r *automationRunRepository) UpdateRunStatus(ctx context.Context, runID string, status string, errorMsg *string) error {
	var query string
	var args []interface{}

	if status == models.AutomationRunStatusCompleted || status == models.AutomationRunStatusFailed {
		query = `
			UPDATE automation_runs
			SET status = $1, error_message = $2, completed_at = NOW(), lock_until = NULL
			WHERE id = $3
		`
		args = []interface{}{status, errorMsg, runID}
	} else {
		query = `
			UPDATE automation_runs
			SET status = $1, error_message = $2
			WHERE id = $3
		`
		args = []interface{}{status, errorMsg, runID}
	}

	result, err := r.db.pool.Exec(ctx, query, args...)
	if err != nil {
		r.logger.Error("failed to update run status",
			zap.String("runID", runID),
			zap.String("status", status),
			zap.Error(err))
		return fmt.Errorf("failed to update run status: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("automation run %s not found", runID)
	}

	return nil
}

// UpdateRunStats updates the statistics of an automation run
func (r *automationRunRepository) UpdateRunStats(ctx context.Context, runID string, offersFound, messagesSent, messagesFailed int) error {
	query := `
		UPDATE automation_runs
		SET offers_found = $1, messages_sent = $2, messages_failed = $3
		WHERE id = $4
	`

	result, err := r.db.pool.Exec(ctx, query, offersFound, messagesSent, messagesFailed, runID)
	if err != nil {
		r.logger.Error("failed to update run stats",
			zap.String("runID", runID),
			zap.Int("offersFound", offersFound),
			zap.Int("messagesSent", messagesSent),
			zap.Int("messagesFailed", messagesFailed),
			zap.Error(err))
		return fmt.Errorf("failed to update run stats: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("automation run %s not found", runID)
	}

	return nil
}

// GetRunByID retrieves an automation run by ID
func (r *automationRunRepository) GetRunByID(ctx context.Context, runID string) (*models.AutomationRun, error) {
	query := `
		SELECT id, automation_id, started_at, completed_at, status,
		       offers_found, messages_sent, messages_failed, error_message, lock_until
		FROM automation_runs
		WHERE id = $1
	`

	var run models.AutomationRun
	err := r.db.QueryRow(ctx, query, runID).Scan(
		&run.ID,
		&run.AutomationID,
		&run.StartedAt,
		&run.CompletedAt,
		&run.Status,
		&run.OffersFound,
		&run.MessagesSent,
		&run.MessagesFailed,
		&run.ErrorMessage,
		&run.LockUntil,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		r.logger.Error("failed to get automation run",
			zap.String("runID", runID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get automation run %s: %w", runID, err)
	}

	return &run, nil
}

// GetRunsByAutomationID retrieves recent runs for an automation
func (r *automationRunRepository) GetRunsByAutomationID(ctx context.Context, automationID string, limit int) ([]models.AutomationRun, error) {
	if limit <= 0 {
		limit = 10
	}

	query := `
		SELECT id, automation_id, started_at, completed_at, status,
		       offers_found, messages_sent, messages_failed, error_message, lock_until
		FROM automation_runs
		WHERE automation_id = $1
		ORDER BY started_at DESC
		LIMIT $2
	`

	rows, err := r.db.Query(ctx, query, automationID, limit)
	if err != nil {
		r.logger.Error("failed to get automation runs",
			zap.String("automationID", automationID),
			zap.Int("limit", limit),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get automation runs: %w", err)
	}
	defer rows.Close()

	var runs []models.AutomationRun
	for rows.Next() {
		var run models.AutomationRun
		err := rows.Scan(
			&run.ID,
			&run.AutomationID,
			&run.StartedAt,
			&run.CompletedAt,
			&run.Status,
			&run.OffersFound,
			&run.MessagesSent,
			&run.MessagesFailed,
			&run.ErrorMessage,
			&run.LockUntil,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan automation run: %w", err)
		}
		runs = append(runs, run)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating automation runs: %w", err)
	}

	return runs, nil
}

// CleanupOldRuns deletes old completed runs
func (r *automationRunRepository) CleanupOldRuns(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)

	query := `
		DELETE FROM automation_runs
		WHERE status IN ($1, $2) AND completed_at < $3
	`

	result, err := r.db.pool.Exec(ctx, query, models.AutomationRunStatusCompleted, models.AutomationRunStatusFailed, cutoff)
	if err != nil {
		r.logger.Error("failed to cleanup old runs",
			zap.Duration("olderThan", olderThan),
			zap.Error(err))
		return 0, fmt.Errorf("failed to cleanup old runs: %w", err)
	}

	deleted := result.RowsAffected()
	if deleted > 0 {
		r.logger.Info("cleaned up old automation runs",
			zap.Int64("deleted", deleted),
			zap.Duration("olderThan", olderThan))
	}

	return deleted, nil
}

// ReapStaleRunningLocks fails 'running' rows whose execution lock expired more
// than `grace` ago. A run is killed (per-automation/tick timeout, or a hard
// crash) before releaseExecutionLock can run; the lock lapses (~10min max) but
// the row stays 'running' with lock_until set, polluting the partial lock index
// (idx_automation_runs_active_lock WHERE lock_until IS NOT NULL) and escaping
// CleanupOldRuns (which only removes completed/failed). `grace` is a safety
// margin beyond the max lock duration so an in-flight run is never touched.
func (r *automationRunRepository) ReapStaleRunningLocks(ctx context.Context, grace time.Duration) (int64, error) {
	cutoff := time.Now().Add(-grace)

	query := `
		UPDATE automation_runs
		SET status = $1, lock_until = NULL, completed_at = NOW()
		WHERE status = $2 AND lock_until IS NOT NULL AND lock_until < $3
	`

	result, err := r.db.pool.Exec(ctx, query,
		models.AutomationRunStatusFailed,
		models.AutomationRunStatusRunning,
		cutoff,
	)
	if err != nil {
		r.logger.Error("failed to reap stale running locks",
			zap.Duration("grace", grace),
			zap.Error(err))
		return 0, fmt.Errorf("failed to reap stale running locks: %w", err)
	}

	reaped := result.RowsAffected()
	if reaped > 0 {
		r.logger.Warn("reaped stale 'running' automation locks",
			zap.Int64("reaped", reaped),
			zap.Duration("grace", grace))
	}

	return reaped, nil
}
