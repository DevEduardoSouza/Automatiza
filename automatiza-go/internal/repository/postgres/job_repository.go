package postgres

import (
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

type jobRepository struct {
	db     *DB
	logger *zap.Logger
}

// NewJobRepository creates a new PostgreSQL job repository for standalone jobs
func NewJobRepository(db *DB, logger *zap.Logger) repository.JobRepository {
	return &jobRepository{
		db:     db,
		logger: logger,
	}
}

const getJobByIDQuery = `
	SELECT id, user_id, action_type, scheduled_for, status, payload, error_message, retry_count, max_retries, created_at, updated_at
	FROM jobs
	WHERE id = $1
`

func (r *jobRepository) GetByID(ctx context.Context, id string) (*models.StandaloneJob, error) {
	var job models.StandaloneJob

	err := r.db.QueryRow(ctx, getJobByIDQuery, id).Scan(
		&job.ID,
		&job.UserID,
		&job.ActionType,
		&job.ScheduledFor,
		&job.Status,
		&job.Payload,
		&job.ErrorMessage,
		&job.RetryCount,
		&job.MaxRetries,
		&job.CreatedAt,
		&job.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		r.logger.Error("Failed to get job by ID",
			zap.String("jobID", id),
			zap.Error(err))
		return nil, err
	}

	return &job, nil
}

func (r *jobRepository) GetByUserID(ctx context.Context, userID string, status *models.StandaloneJobStatus, limit int) ([]models.StandaloneJob, error) {
	if limit <= 0 {
		limit = 50
	}

	var query string
	var args []interface{}

	if status != nil {
		query = `
			SELECT id, user_id, action_type, scheduled_for, status, payload, error_message, retry_count, max_retries, created_at, updated_at
			FROM jobs
			WHERE user_id = $1 AND status = $2
			ORDER BY scheduled_for DESC
			LIMIT $3
		`
		args = []interface{}{userID, *status, limit}
	} else {
		query = `
			SELECT id, user_id, action_type, scheduled_for, status, payload, error_message, retry_count, max_retries, created_at, updated_at
			FROM jobs
			WHERE user_id = $1
			ORDER BY scheduled_for DESC
			LIMIT $2
		`
		args = []interface{}{userID, limit}
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		r.logger.Error("Failed to get jobs by user ID",
			zap.String("userID", userID),
			zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	var jobs []models.StandaloneJob
	for rows.Next() {
		var job models.StandaloneJob
		err := rows.Scan(
			&job.ID,
			&job.UserID,
			&job.ActionType,
			&job.ScheduledFor,
			&job.Status,
			&job.Payload,
			&job.ErrorMessage,
			&job.RetryCount,
			&job.MaxRetries,
			&job.CreatedAt,
			&job.UpdatedAt,
		)
		if err != nil {
			r.logger.Error("Failed to scan job row",
				zap.Error(err))
			return nil, err
		}
		jobs = append(jobs, job)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return jobs, nil
}

// encodeJobCursor encodes a (scheduled_for, id) pair into an opaque cursor token.
func encodeJobCursor(scheduledFor time.Time, id string) string {
	raw := fmt.Sprintf("%d|%s", scheduledFor.UnixMicro(), id)
	return base64.URLEncoding.EncodeToString([]byte(raw))
}

// decodeJobCursor decodes a cursor token into (scheduled_for, id).
func decodeJobCursor(cursor string) (time.Time, string, error) {
	decoded, err := base64.URLEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, "", fmt.Errorf("invalid cursor: %w", err)
	}
	parts := strings.SplitN(string(decoded), "|", 2)
	if len(parts) != 2 {
		return time.Time{}, "", fmt.Errorf("invalid cursor format")
	}
	var micros int64
	_, err = fmt.Sscanf(parts[0], "%d", &micros)
	if err != nil {
		return time.Time{}, "", fmt.Errorf("invalid cursor timestamp: %w", err)
	}
	return time.UnixMicro(micros), parts[1], nil
}

func (r *jobRepository) ListByUserID(ctx context.Context, params repository.JobListParams) (*repository.JobListResult, error) {
	pageSize := params.PageSize
	if pageSize <= 0 {
		pageSize = 50
	}
	if pageSize > 100 {
		pageSize = 100
	}

	// Count total
	var totalCount int
	countErr := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM jobs WHERE user_id = $1`, params.UserID).Scan(&totalCount)
	if countErr != nil {
		r.logger.Error("Failed to count jobs", zap.String("userID", params.UserID), zap.Error(countErr))
		return nil, countErr
	}

	// Build cursor-based query (ORDER BY scheduled_for DESC, id DESC)
	var query string
	var args []interface{}

	// Fetch one extra to know if there's a next page
	fetchLimit := pageSize + 1

	if params.Cursor != "" {
		cursorTime, cursorID, err := decodeJobCursor(params.Cursor)
		if err != nil {
			return nil, fmt.Errorf("invalid page_token: %w", err)
		}
		query = `
			SELECT id, user_id, action_type, scheduled_for, status, payload, error_message, retry_count, max_retries, created_at, updated_at
			FROM jobs
			WHERE user_id = $1 AND (scheduled_for, id) < ($2, $3)
			ORDER BY scheduled_for DESC, id DESC
			LIMIT $4
		`
		args = []interface{}{params.UserID, cursorTime, cursorID, fetchLimit}
	} else {
		query = `
			SELECT id, user_id, action_type, scheduled_for, status, payload, error_message, retry_count, max_retries, created_at, updated_at
			FROM jobs
			WHERE user_id = $1
			ORDER BY scheduled_for DESC, id DESC
			LIMIT $2
		`
		args = []interface{}{params.UserID, fetchLimit}
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		r.logger.Error("Failed to list jobs by user ID", zap.String("userID", params.UserID), zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	var jobs []models.StandaloneJob
	for rows.Next() {
		var job models.StandaloneJob
		if err := rows.Scan(
			&job.ID, &job.UserID, &job.ActionType, &job.ScheduledFor,
			&job.Status, &job.Payload, &job.ErrorMessage, &job.RetryCount, &job.MaxRetries, &job.CreatedAt, &job.UpdatedAt,
		); err != nil {
			r.logger.Error("Failed to scan job row", zap.Error(err))
			return nil, err
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	result := &repository.JobListResult{
		TotalCount: totalCount,
	}

	if len(jobs) > pageSize {
		// There's a next page
		result.Jobs = jobs[:pageSize]
		lastJob := result.Jobs[pageSize-1]
		result.NextCursor = encodeJobCursor(lastJob.ScheduledFor, lastJob.ID)
	} else {
		result.Jobs = jobs
	}

	return result, nil
}

func (r *jobRepository) GetPendingDueJobs(ctx context.Context, limit int) ([]models.StandaloneJob, error) {
	if limit <= 0 {
		limit = 100
	}

	// Use FOR UPDATE SKIP LOCKED for safe concurrent processing
	query := `
		SELECT id, user_id, action_type, scheduled_for, status, payload, error_message, retry_count, max_retries, created_at, updated_at
		FROM jobs
		WHERE status = $1 AND scheduled_for <= NOW()
		ORDER BY scheduled_for ASC
		LIMIT $2
		FOR UPDATE SKIP LOCKED
	`

	rows, err := r.db.Query(ctx, query, models.StandaloneJobStatusPending, limit)
	if err != nil {
		r.logger.Error("Failed to get pending due jobs",
			zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	var jobs []models.StandaloneJob
	for rows.Next() {
		var job models.StandaloneJob
		err := rows.Scan(
			&job.ID,
			&job.UserID,
			&job.ActionType,
			&job.ScheduledFor,
			&job.Status,
			&job.Payload,
			&job.ErrorMessage,
			&job.RetryCount,
			&job.MaxRetries,
			&job.CreatedAt,
			&job.UpdatedAt,
		)
		if err != nil {
			r.logger.Error("Failed to scan pending job row",
				zap.Error(err))
			return nil, err
		}
		jobs = append(jobs, job)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return jobs, nil
}

func (r *jobRepository) Create(ctx context.Context, job *models.StandaloneJob) (*models.StandaloneJob, error) {
	now := time.Now().UTC()

	// Generate UUID if not provided
	if job.ID == "" {
		job.ID = uuid.New().String()
	}

	// Set default status to pending
	if job.Status == "" {
		job.Status = models.StandaloneJobStatusPending
	}

	// Use ScheduledFor if provided, otherwise default to now
	scheduledFor := job.ScheduledFor
	if scheduledFor.IsZero() {
		scheduledFor = now
	}

	query := `
		INSERT INTO jobs (id, user_id, type, action_type, scheduled_for, status, payload, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at, updated_at
	`

	var createdAt, updatedAt time.Time

	err := r.db.QueryRow(ctx, query,
		job.ID,
		job.UserID,
		job.ActionType,
		job.ActionType,
		scheduledFor,
		job.Status,
		job.Payload,
		now,
		now,
	).Scan(&job.ID, &createdAt, &updatedAt)
	if err != nil {
		r.logger.Error("Failed to create job",
			zap.String("userID", job.UserID),
			zap.String("actionType", string(job.ActionType)),
			zap.Error(err))
		return nil, err
	}

	job.CreatedAt = createdAt
	job.UpdatedAt = updatedAt
	job.ScheduledFor = scheduledFor

	return job, nil
}

func (r *jobRepository) Update(ctx context.Context, id string, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return nil
	}

	// Build dynamic UPDATE query
	setClauses := make([]string, 0, len(updates)+1)
	args := make([]interface{}, 0, len(updates)+2)
	argIndex := 1

	for key, value := range updates {
		// Handle JSONB payload specially - JobPayload implements driver.Valuer
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", key, argIndex))
		args = append(args, value)
		argIndex++
	}

	// Always update updated_at
	setClauses = append(setClauses, fmt.Sprintf("updated_at = $%d", argIndex))
	args = append(args, time.Now().UTC())
	argIndex++

	// Add id as the last argument
	args = append(args, id)

	query := fmt.Sprintf("UPDATE jobs SET %s WHERE id = $%d", strings.Join(setClauses, ", "), argIndex)

	result, err := r.db.pool.Exec(ctx, query, args...)
	if err != nil {
		r.logger.Error("Failed to update job",
			zap.String("jobID", id),
			zap.Error(err))
		return err
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("job not found: %s", id)
	}

	return nil
}

func (r *jobRepository) UpdateStatus(ctx context.Context, id string, status models.StandaloneJobStatus, errorMsg *string) error {
	query := `
		UPDATE jobs
		SET status = $1, error_message = $2, updated_at = $3
		WHERE id = $4
	`

	result, err := r.db.pool.Exec(ctx, query, status, errorMsg, time.Now().UTC(), id)
	if err != nil {
		r.logger.Error("Failed to update job status",
			zap.String("jobID", id),
			zap.String("status", string(status)),
			zap.Error(err))
		return err
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("job not found: %s", id)
	}

	return nil
}

// FailStaleRunning marca como 'failed' jobs presos em 'running' há mais de
// olderThan (crash/deploy no meio da execução, ou o ctx do cron — cap 5min —
// morreu antes do desfecho terminal). Sem isso o job sumia do pipeline em
// 'running' eterno. Vai para 'failed' (e NÃO de volta para 'pending') de
// propósito: re-executar um job de envio parcialmente concluído re-enviaria
// aos grupos já atendidos — duplicar é pior que perder o restante.
func (r *jobRepository) FailStaleRunning(ctx context.Context, olderThan time.Duration) (int64, error) {
	query := `
		UPDATE jobs
		SET status = $1,
		    error_message = 'job interrompido (crash/timeout) - marcado como failed para nao duplicar envios',
		    updated_at = NOW()
		WHERE status = $2 AND updated_at < $3
	`

	result, err := r.db.pool.Exec(ctx, query,
		models.StandaloneJobStatusFailed,
		models.StandaloneJobStatusRunning,
		time.Now().UTC().Add(-olderThan),
	)
	if err != nil {
		r.logger.Error("Failed to fail stale running jobs", zap.Error(err))
		return 0, fmt.Errorf("fail stale running jobs: %w", err)
	}

	n := result.RowsAffected()
	if n > 0 {
		r.logger.Warn("Marked stale 'running' jobs as failed", zap.Int64("count", n))
	}
	return n, nil
}

func (r *jobRepository) ScheduleRetry(ctx context.Context, id string, nextRunAt time.Time) error {
	query := `
		UPDATE jobs
		SET status = $1, retry_count = retry_count + 1, scheduled_for = $2, error_message = NULL, updated_at = $3
		WHERE id = $4
	`

	result, err := r.db.pool.Exec(ctx, query, models.StandaloneJobStatusPending, nextRunAt, time.Now().UTC(), id)
	if err != nil {
		r.logger.Error("Failed to schedule retry",
			zap.String("jobID", id),
			zap.Time("nextRunAt", nextRunAt),
			zap.Error(err))
		return err
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("job not found: %s", id)
	}

	return nil
}

func (r *jobRepository) Delete(ctx context.Context, id, userID string) error {
	query := `
		DELETE FROM jobs
		WHERE id = $1 AND user_id = $2
	`

	result, err := r.db.pool.Exec(ctx, query, id, userID)
	if err != nil {
		r.logger.Error("Failed to delete job",
			zap.String("jobID", id),
			zap.String("userID", userID),
			zap.Error(err))
		return err
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("job not found or not owned by user: %s", id)
	}

	return nil
}

func (r *jobRepository) CountByStatus(ctx context.Context, userID string) (map[models.StandaloneJobStatus]int, error) {
	query := `
		SELECT status, COUNT(*) as count
		FROM jobs
		WHERE user_id = $1
		GROUP BY status
	`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		r.logger.Error("Failed to count jobs by status",
			zap.String("userID", userID),
			zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	result := make(map[models.StandaloneJobStatus]int)
	for rows.Next() {
		var status models.StandaloneJobStatus
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			r.logger.Error("Failed to scan job count row",
				zap.Error(err))
			return nil, err
		}
		result[status] = count
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}
