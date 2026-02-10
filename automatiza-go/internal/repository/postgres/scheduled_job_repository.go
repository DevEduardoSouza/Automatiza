package postgres

import (
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

type scheduledJobRepository struct {
	db     *DB
	logger *zap.Logger
}

// NewScheduledJobRepository creates a new PostgreSQL scheduled job repository
func NewScheduledJobRepository(db *DB, logger *zap.Logger) repository.ScheduledJobRepository {
	return &scheduledJobRepository{
		db:     db,
		logger: logger,
	}
}

func (r *scheduledJobRepository) GetPendingJobs(ctx context.Context) ([]models.ScheduledJob, error) {
	query := `
		SELECT id, automation_id, scheduled_for, status, error_message, created_at, updated_at
		FROM scheduled_jobs
		WHERE status = 'pending' AND scheduled_for <= NOW()
		ORDER BY scheduled_for ASC
		LIMIT 100
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []models.ScheduledJob
	for rows.Next() {
		var j models.ScheduledJob
		var scheduledFor, createdAt, updatedAt time.Time

		err := rows.Scan(&j.ID, &j.AutomationID, &scheduledFor, &j.Status, &j.ErrorMessage, &createdAt, &updatedAt)
		if err != nil {
			return nil, err
		}

		j.ScheduledFor = scheduledFor.Format(time.RFC3339)
		j.CreatedAt = createdAt.Format(time.RFC3339)
		j.UpdatedAt = updatedAt.Format(time.RFC3339)
		jobs = append(jobs, j)
	}

	return jobs, rows.Err()
}

func (r *scheduledJobRepository) UpdateJobStatus(ctx context.Context, jobID, status string, errorMsg *string) error {
	query := `
		UPDATE scheduled_jobs
		SET status = $1, error_message = $2, updated_at = $3
		WHERE id = $4
	`

	_, err := r.db.pool.Exec(ctx, query, status, errorMsg, time.Now().UTC(), jobID)
	if err != nil {
		r.logger.Error("Failed to update job status",
			zap.String("jobID", jobID),
			zap.String("status", status),
			zap.Error(err))
		return err
	}

	return nil
}

func (r *scheduledJobRepository) CreateJob(ctx context.Context, job *models.ScheduledJob) (*models.ScheduledJob, error) {
	now := time.Now().UTC()

	// Parse scheduled_for
	scheduledFor, err := time.Parse(time.RFC3339, job.ScheduledFor)
	if err != nil {
		scheduledFor = now
	}

	query := `
		INSERT INTO scheduled_jobs (automation_id, scheduled_for, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at
	`

	var id string
	var createdAt, updatedAt time.Time

	err = r.db.QueryRow(ctx, query, job.AutomationID, scheduledFor, models.JobStatusPending, now, now).Scan(&id, &createdAt, &updatedAt)
	if err != nil {
		r.logger.Error("Failed to create scheduled job",
			zap.String("automationID", job.AutomationID),
			zap.Error(err))
		return nil, err
	}

	job.ID = id
	job.Status = models.JobStatusPending
	job.CreatedAt = createdAt.Format(time.RFC3339)
	job.UpdatedAt = updatedAt.Format(time.RFC3339)

	return job, nil
}

func (r *scheduledJobRepository) GetJobByID(ctx context.Context, jobID string) (*models.ScheduledJob, error) {
	query := `
		SELECT id, automation_id, scheduled_for, status, error_message, created_at, updated_at
		FROM scheduled_jobs
		WHERE id = $1
	`

	var j models.ScheduledJob
	var scheduledFor, createdAt, updatedAt time.Time

	err := r.db.QueryRow(ctx, query, jobID).Scan(&j.ID, &j.AutomationID, &scheduledFor, &j.Status, &j.ErrorMessage, &createdAt, &updatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	j.ScheduledFor = scheduledFor.Format(time.RFC3339)
	j.CreatedAt = createdAt.Format(time.RFC3339)
	j.UpdatedAt = updatedAt.Format(time.RFC3339)

	return &j, nil
}

// GetJobsByAutomation retrieves all jobs for a specific automation
func (r *scheduledJobRepository) GetJobsByAutomation(ctx context.Context, automationID string, limit int) ([]models.ScheduledJob, error) {
	if limit <= 0 {
		limit = 50
	}

	query := `
		SELECT id, automation_id, scheduled_for, status, error_message, created_at, updated_at
		FROM scheduled_jobs
		WHERE automation_id = $1
		ORDER BY scheduled_for DESC
		LIMIT $2
	`

	rows, err := r.db.Query(ctx, query, automationID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []models.ScheduledJob
	for rows.Next() {
		var j models.ScheduledJob
		var scheduledFor, createdAt, updatedAt time.Time

		err := rows.Scan(&j.ID, &j.AutomationID, &scheduledFor, &j.Status, &j.ErrorMessage, &createdAt, &updatedAt)
		if err != nil {
			return nil, err
		}

		j.ScheduledFor = scheduledFor.Format(time.RFC3339)
		j.CreatedAt = createdAt.Format(time.RFC3339)
		j.UpdatedAt = updatedAt.Format(time.RFC3339)
		jobs = append(jobs, j)
	}

	return jobs, rows.Err()
}

// DeleteOldJobs deletes completed or failed jobs older than the specified duration
func (r *scheduledJobRepository) DeleteOldJobs(ctx context.Context, olderThan time.Duration) (int64, error) {
	query := `
		DELETE FROM scheduled_jobs
		WHERE status IN ('completed', 'failed') AND updated_at < $1
	`
	cutoff := time.Now().Add(-olderThan)

	result, err := r.db.pool.Exec(ctx, query, cutoff)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected(), nil
}

// GetQueueStatus returns the current status of the job queue
func (r *scheduledJobRepository) GetQueueStatus(ctx context.Context) (*models.QueueStatus, error) {
	query := `
		SELECT
			COUNT(*) FILTER (WHERE status = 'pending') as pending,
			COUNT(*) FILTER (WHERE status = 'processing') as processing,
			COUNT(*) FILTER (WHERE status = 'completed') as completed,
			COUNT(*) FILTER (WHERE status = 'failed') as failed
		FROM scheduled_jobs
		WHERE created_at > NOW() - INTERVAL '24 hours'
	`

	var status models.QueueStatus
	err := r.db.QueryRow(ctx, query).Scan(&status.Pending, &status.Processing, &status.Completed, &status.Failed)
	if err != nil {
		return nil, err
	}

	status.TotalQueued = status.Pending + status.Processing
	return &status, nil
}

// ClaimJob atomically claims a job for processing (prevents double processing)
func (r *scheduledJobRepository) ClaimJob(ctx context.Context, jobID string) (bool, error) {
	query := `
		UPDATE scheduled_jobs
		SET status = 'processing', updated_at = $1
		WHERE id = $2 AND status = 'pending'
	`

	result, err := r.db.pool.Exec(ctx, query, time.Now().UTC(), jobID)
	if err != nil {
		return false, err
	}

	return result.RowsAffected() > 0, nil
}
