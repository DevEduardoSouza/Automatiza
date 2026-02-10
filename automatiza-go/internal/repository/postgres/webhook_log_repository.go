package postgres

import (
	"context"
	"encoding/json"
	"time"

	"go.uber.org/zap"
)

// WebhookLog represents a webhook log entry
type WebhookLog struct {
	ID          int                    `json:"id"`
	Source      string                 `json:"source"`
	EventType   *string                `json:"event_type,omitempty"`
	Email       *string                `json:"email,omitempty"`
	Plan        *string                `json:"plan,omitempty"`
	Payload     map[string]interface{} `json:"payload,omitempty"`
	Status      string                 `json:"status"`
	Error       *string                `json:"error,omitempty"`
	ProcessedAt *string                `json:"processed_at,omitempty"`
	CreatedAt   string                 `json:"created_at"`
}

// WebhookLogRepository interface
type WebhookLogRepository interface {
	Create(ctx context.Context, source string, eventType *string, email *string, plan *string, payload map[string]interface{}) (*WebhookLog, error)
	UpdateStatus(ctx context.Context, id int, status string, err *string) error
	GetRecent(ctx context.Context, source string, limit int) ([]WebhookLog, error)
	DeleteOld(ctx context.Context, olderThan time.Duration) (int64, error)
}

type webhookLogRepository struct {
	db     *DB
	logger *zap.Logger
}

// NewWebhookLogRepository creates a new PostgreSQL webhook log repository
func NewWebhookLogRepository(db *DB, logger *zap.Logger) WebhookLogRepository {
	return &webhookLogRepository{
		db:     db,
		logger: logger,
	}
}

func (r *webhookLogRepository) Create(ctx context.Context, source string, eventType *string, email *string, plan *string, payload map[string]interface{}) (*WebhookLog, error) {
	now := time.Now().UTC()

	var payloadJSON []byte
	if payload != nil {
		payloadJSON, _ = json.Marshal(payload)
	}

	query := `
		INSERT INTO webhook_logs (source, event_type, email, plan, payload, status, created_at)
		VALUES ($1, $2, $3, $4, $5, 'received', $6)
		RETURNING id, created_at
	`

	var id int
	var createdAt time.Time

	err := r.db.QueryRow(ctx, query, source, eventType, email, plan, payloadJSON, now).Scan(&id, &createdAt)
	if err != nil {
		r.logger.Error("Failed to create webhook log",
			zap.String("source", source),
			zap.Error(err))
		return nil, err
	}

	return &WebhookLog{
		ID:        id,
		Source:    source,
		EventType: eventType,
		Email:     email,
		Plan:      plan,
		Payload:   payload,
		Status:    "received",
		CreatedAt: createdAt.Format(time.RFC3339),
	}, nil
}

func (r *webhookLogRepository) UpdateStatus(ctx context.Context, id int, status string, errMsg *string) error {
	now := time.Now().UTC()

	query := `
		UPDATE webhook_logs
		SET status = $1, error = $2, processed_at = $3
		WHERE id = $4
	`

	_, err := r.db.pool.Exec(ctx, query, status, errMsg, now, id)
	if err != nil {
		r.logger.Error("Failed to update webhook log status",
			zap.Int("id", id),
			zap.String("status", status),
			zap.Error(err))
		return err
	}

	return nil
}

func (r *webhookLogRepository) GetRecent(ctx context.Context, source string, limit int) ([]WebhookLog, error) {
	if limit <= 0 {
		limit = 50
	}

	query := `
		SELECT id, source, event_type, email, plan, payload, status, error, processed_at, created_at
		FROM webhook_logs
		WHERE source = $1
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := r.db.Query(ctx, query, source, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []WebhookLog
	for rows.Next() {
		var log WebhookLog
		var payloadJSON []byte
		var processedAt *time.Time
		var createdAt time.Time

		err := rows.Scan(
			&log.ID, &log.Source, &log.EventType, &log.Email, &log.Plan, &payloadJSON, &log.Status, &log.Error, &processedAt, &createdAt,
		)
		if err != nil {
			return nil, err
		}

		if payloadJSON != nil {
			_ = json.Unmarshal(payloadJSON, &log.Payload)
		}
		if processedAt != nil {
			t := processedAt.Format(time.RFC3339)
			log.ProcessedAt = &t
		}
		log.CreatedAt = createdAt.Format(time.RFC3339)

		logs = append(logs, log)
	}

	return logs, rows.Err()
}

func (r *webhookLogRepository) DeleteOld(ctx context.Context, olderThan time.Duration) (int64, error) {
	query := `DELETE FROM webhook_logs WHERE created_at < $1`
	cutoff := time.Now().Add(-olderThan)

	result, err := r.db.pool.Exec(ctx, query, cutoff)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected(), nil
}

// GetStats returns statistics about webhook logs
func (r *webhookLogRepository) GetStats(ctx context.Context) (map[string]interface{}, error) {
	query := `
		SELECT
			source,
			COUNT(*) as total,
			COUNT(*) FILTER (WHERE status = 'processed') as processed,
			COUNT(*) FILTER (WHERE status = 'failed') as failed
		FROM webhook_logs
		WHERE created_at > NOW() - INTERVAL '24 hours'
		GROUP BY source
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := make(map[string]interface{})
	for rows.Next() {
		var source string
		var total, processed, failed int
		if err := rows.Scan(&source, &total, &processed, &failed); err != nil {
			return nil, err
		}
		stats[source] = map[string]int{
			"total":     total,
			"processed": processed,
			"failed":    failed,
		}
	}

	return stats, rows.Err()
}
