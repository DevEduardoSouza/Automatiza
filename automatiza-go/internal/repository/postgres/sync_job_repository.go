package postgres

import (
	"automatiza-go/internal/repository"
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

type syncJobRepository struct {
	db     *DB
	logger *zap.Logger
}

// NewSyncJobRepository creates a new PostgreSQL sync job repository
func NewSyncJobRepository(db *DB, logger *zap.Logger) repository.SyncJobRepository {
	return &syncJobRepository{
		db:     db,
		logger: logger,
	}
}

func (r *syncJobRepository) Create(ctx context.Context, job *repository.SyncJob) (*repository.SyncJob, error) {
	query := `
		INSERT INTO sync_jobs (job_id, user_id, instance_id, instance_name, state, progress, total_groups, synced_groups)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at, updated_at
	`

	var createdAt, updatedAt time.Time
	err := r.db.QueryRow(ctx, query,
		job.JobID, job.UserID, job.InstanceID, job.InstanceName,
		job.State, job.Progress, job.TotalGroups, job.SyncedGroups,
	).Scan(&job.ID, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}

	job.CreatedAt = createdAt
	job.UpdatedAt = updatedAt
	return job, nil
}

func (r *syncJobRepository) GetByJobID(ctx context.Context, jobID string) (*repository.SyncJob, error) {
	query := `
		SELECT id, job_id, user_id, instance_id, instance_name, state, progress,
		       total_groups, synced_groups, error_message, started_at, completed_at,
		       created_at, updated_at
		FROM sync_jobs
		WHERE job_id = $1
	`

	var job repository.SyncJob
	var errorMessage *string
	err := r.db.QueryRow(ctx, query, jobID).Scan(
		&job.ID, &job.JobID, &job.UserID, &job.InstanceID, &job.InstanceName,
		&job.State, &job.Progress, &job.TotalGroups, &job.SyncedGroups,
		&errorMessage, &job.StartedAt, &job.CompletedAt,
		&job.CreatedAt, &job.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	if errorMessage != nil {
		job.ErrorMessage = *errorMessage
	}
	return &job, nil
}

func (r *syncJobRepository) UpdateState(ctx context.Context, jobID, state string) error {
	query := `UPDATE sync_jobs SET state = $2, updated_at = NOW() WHERE job_id = $1`
	return r.db.Exec(ctx, query, jobID, state)
}

func (r *syncJobRepository) UpdateProgress(ctx context.Context, jobID string, progress, syncedGroups, totalGroups int) error {
	query := `
		UPDATE sync_jobs
		SET progress = $2, synced_groups = $3, total_groups = $4, updated_at = NOW()
		WHERE job_id = $1
	`
	return r.db.Exec(ctx, query, jobID, progress, syncedGroups, totalGroups)
}

func (r *syncJobRepository) MarkStarted(ctx context.Context, jobID string) error {
	query := `
		UPDATE sync_jobs
		SET state = $2, started_at = NOW(), updated_at = NOW()
		WHERE job_id = $1
	`
	return r.db.Exec(ctx, query, jobID, repository.SyncJobStateInProgress)
}

func (r *syncJobRepository) MarkCompleted(ctx context.Context, jobID string, syncedGroups, totalGroups int) error {
	query := `
		UPDATE sync_jobs
		SET state = $2, progress = 100, synced_groups = $3, total_groups = $4,
		    completed_at = NOW(), updated_at = NOW()
		WHERE job_id = $1
	`
	return r.db.Exec(ctx, query, jobID, repository.SyncJobStateCompleted, syncedGroups, totalGroups)
}

func (r *syncJobRepository) MarkFailed(ctx context.Context, jobID, errorMessage string) error {
	query := `
		UPDATE sync_jobs
		SET state = $2, error_message = $3, completed_at = NOW(), updated_at = NOW()
		WHERE job_id = $1
	`
	return r.db.Exec(ctx, query, jobID, repository.SyncJobStateFailed, errorMessage)
}

func (r *syncJobRepository) GetActiveJobsByUser(ctx context.Context, userID string) ([]repository.SyncJob, error) {
	query := `
		SELECT id, job_id, user_id, instance_id, instance_name, state, progress,
		       total_groups, synced_groups, error_message, started_at, completed_at,
		       created_at, updated_at
		FROM sync_jobs
		WHERE user_id = $1 AND state IN ($2, $3)
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(ctx, query, userID, repository.SyncJobStatePending, repository.SyncJobStateInProgress)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []repository.SyncJob
	for rows.Next() {
		var job repository.SyncJob
		var errorMessage *string
		err := rows.Scan(
			&job.ID, &job.JobID, &job.UserID, &job.InstanceID, &job.InstanceName,
			&job.State, &job.Progress, &job.TotalGroups, &job.SyncedGroups,
			&errorMessage, &job.StartedAt, &job.CompletedAt,
			&job.CreatedAt, &job.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		if errorMessage != nil {
			job.ErrorMessage = *errorMessage
		}
		jobs = append(jobs, job)
	}

	return jobs, rows.Err()
}

func (r *syncJobRepository) Cleanup(ctx context.Context, olderThan time.Duration) (int64, error) {
	query := `
		DELETE FROM sync_jobs
		WHERE state IN ($1, $2) AND completed_at < $3
	`
	cutoff := time.Now().Add(-olderThan)
	err := r.db.Exec(ctx, query, repository.SyncJobStateCompleted, repository.SyncJobStateFailed, cutoff)
	if err != nil {
		return 0, err
	}
	// Note: This DB wrapper doesn't return affected rows count
	// For now, return 0 to indicate success without count
	return 0, nil
}
