package postgres

import (
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
)

type webhookEventsRepository struct {
	db     *DB
	logger *zap.Logger
}

// NewWebhookEventsRepository creates a new PostgreSQL webhook events repository
func NewWebhookEventsRepository(db *DB, logger *zap.Logger) repository.WebhookEventsRepository {
	return &webhookEventsRepository{
		db:     db,
		logger: logger,
	}
}

// InsertIfNotExists attempts to insert an event, returns true if inserted, false if duplicate.
// Uses ON CONFLICT DO NOTHING for idempotency.
func (r *webhookEventsRepository) InsertIfNotExists(ctx context.Context, eventID, sessionName, eventType string) (bool, error) {
	query := `
		INSERT INTO webhook_events (event_id, session_name, event_type, received_at, status, retry_count)
		VALUES ($1, $2, $3, $4, $5, 0)
		ON CONFLICT (event_id) DO NOTHING
	`

	now := time.Now().UTC()
	result, err := r.db.pool.Exec(ctx, query, eventID, sessionName, eventType, now, models.WebhookEventRecordStatusReceived)
	if err != nil {
		r.logger.Error("Failed to insert webhook event",
			zap.String("eventID", eventID),
			zap.String("sessionName", sessionName),
			zap.String("eventType", eventType),
			zap.Error(err))
		return false, fmt.Errorf("insert webhook event: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		r.logger.Debug("Webhook event already exists (duplicate)",
			zap.String("eventID", eventID))
		return false, nil
	}

	r.logger.Debug("Webhook event inserted",
		zap.String("eventID", eventID),
		zap.String("sessionName", sessionName),
		zap.String("eventType", eventType))

	return true, nil
}

// Exists checks if an event exists by its event_id
func (r *webhookEventsRepository) Exists(ctx context.Context, eventID string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM webhook_events WHERE event_id = $1)`

	var exists bool
	err := r.db.QueryRow(ctx, query, eventID).Scan(&exists)
	if err != nil {
		r.logger.Error("Failed to check webhook event existence",
			zap.String("eventID", eventID),
			zap.Error(err))
		return false, fmt.Errorf("check webhook event existence: %w", err)
	}

	return exists, nil
}

// MarkProcessing marks an event as processing
func (r *webhookEventsRepository) MarkProcessing(ctx context.Context, eventID string) error {
	query := `UPDATE webhook_events SET status = $1 WHERE event_id = $2`

	result, err := r.db.pool.Exec(ctx, query, models.WebhookEventRecordStatusProcessing, eventID)
	if err != nil {
		r.logger.Error("Failed to mark webhook event as processing",
			zap.String("eventID", eventID),
			zap.Error(err))
		return fmt.Errorf("mark webhook event processing: %w", err)
	}

	if result.RowsAffected() == 0 {
		r.logger.Warn("No webhook event found to mark as processing",
			zap.String("eventID", eventID))
	}

	return nil
}

// MarkProcessed marks an event as processed with current timestamp
func (r *webhookEventsRepository) MarkProcessed(ctx context.Context, eventID string) error {
	query := `UPDATE webhook_events SET status = $1, processed_at = $2 WHERE event_id = $3`

	now := time.Now().UTC()
	result, err := r.db.pool.Exec(ctx, query, models.WebhookEventRecordStatusProcessed, now, eventID)
	if err != nil {
		r.logger.Error("Failed to mark webhook event as processed",
			zap.String("eventID", eventID),
			zap.Error(err))
		return fmt.Errorf("mark webhook event processed: %w", err)
	}

	if result.RowsAffected() == 0 {
		r.logger.Warn("No webhook event found to mark as processed",
			zap.String("eventID", eventID))
	}

	return nil
}

// MarkProcessedEnsure finaliza o evento em UM statement, cobrindo também o
// caso de linha ausente (handler falhou ao inserir e forçou o enqueue): upsert
// que insere já como 'processed' ou atualiza a linha existente. É o único
// roundtrip do consumer no caminho feliz do modo lean-persist
// (WEBHOOK_LEAN_PERSIST) — substitui Exists + InsertIfNotExists +
// MarkProcessing + MarkProcessed. Roda com contexto DESACOPLADO
// (WithoutCancel): sucesso do processamento não pode virar linha órfã porque
// o ctx da task morreu na hora de persistir (regra go-backend.md).
func (r *webhookEventsRepository) MarkProcessedEnsure(ctx context.Context, eventID, sessionName, eventType string) error {
	query := `
		INSERT INTO webhook_events (event_id, session_name, event_type, received_at, status, retry_count, processed_at)
		VALUES ($1, $2, $3, NOW(), $4, 0, $5)
		ON CONFLICT (event_id) DO UPDATE SET status = $4, processed_at = EXCLUDED.processed_at`

	opCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()

	now := time.Now().UTC()
	if _, err := r.db.pool.Exec(opCtx, query, eventID, sessionName, eventType,
		models.WebhookEventRecordStatusProcessed, now); err != nil {
		r.logger.Error("Failed to mark webhook event processed (ensure)",
			zap.String("eventID", eventID),
			zap.Error(err))
		return fmt.Errorf("mark webhook event processed (ensure): %w", err)
	}
	return nil
}

// MarkFailed marks an event as failed with error message and increments retry count
func (r *webhookEventsRepository) MarkFailed(ctx context.Context, eventID, errorMessage string) error {
	query := `
		UPDATE webhook_events
		SET status = $1, error_message = $2, retry_count = retry_count + 1
		WHERE event_id = $3
	`

	result, err := r.db.pool.Exec(ctx, query, models.WebhookEventRecordStatusFailed, errorMessage, eventID)
	if err != nil {
		r.logger.Error("Failed to mark webhook event as failed",
			zap.String("eventID", eventID),
			zap.String("errorMessage", errorMessage),
			zap.Error(err))
		return fmt.Errorf("mark webhook event failed: %w", err)
	}

	if result.RowsAffected() == 0 {
		r.logger.Warn("No webhook event found to mark as failed",
			zap.String("eventID", eventID))
	}

	return nil
}

// MarkDLQ moves an event to DLQ (dead letter queue) status
func (r *webhookEventsRepository) MarkDLQ(ctx context.Context, eventID, errorMessage string) error {
	query := `UPDATE webhook_events SET status = $1, error_message = $2 WHERE event_id = $3`

	result, err := r.db.pool.Exec(ctx, query, models.WebhookEventRecordStatusDLQ, errorMessage, eventID)
	if err != nil {
		r.logger.Error("Failed to mark webhook event as DLQ",
			zap.String("eventID", eventID),
			zap.String("errorMessage", errorMessage),
			zap.Error(err))
		return fmt.Errorf("mark webhook event DLQ: %w", err)
	}

	if result.RowsAffected() == 0 {
		r.logger.Warn("No webhook event found to mark as DLQ",
			zap.String("eventID", eventID))
	}

	return nil
}

// IncrementRetryCount increments the retry count for an event
func (r *webhookEventsRepository) IncrementRetryCount(ctx context.Context, eventID string) error {
	query := `UPDATE webhook_events SET retry_count = retry_count + 1 WHERE event_id = $1`

	result, err := r.db.pool.Exec(ctx, query, eventID)
	if err != nil {
		r.logger.Error("Failed to increment webhook event retry count",
			zap.String("eventID", eventID),
			zap.Error(err))
		return fmt.Errorf("increment webhook event retry count: %w", err)
	}

	if result.RowsAffected() == 0 {
		r.logger.Warn("No webhook event found to increment retry count",
			zap.String("eventID", eventID))
	}

	return nil
}

// GetPendingEvents retrieves events that need processing (received or failed with retries remaining).
// T3.9: Hard limit of 1000 results to prevent unbounded memory allocation.
func (r *webhookEventsRepository) GetPendingEvents(ctx context.Context, limit int) ([]models.WebhookEventRecord, error) {
	const maxLimit = 1000
	if limit <= 0 {
		limit = 100
	}
	if limit > maxLimit {
		limit = maxLimit
	}

	query := `
		SELECT id, event_id, session_name, event_type, received_at, processed_at, status, error_message, retry_count
		FROM webhook_events
		WHERE status IN ($1, $2)
		ORDER BY received_at ASC
		LIMIT $3
	`

	rows, err := r.db.Query(ctx, query, models.WebhookEventRecordStatusReceived, models.WebhookEventRecordStatusFailed, limit)
	if err != nil {
		r.logger.Error("Failed to get pending webhook events",
			zap.Int("limit", limit),
			zap.Error(err))
		return nil, fmt.Errorf("get pending webhook events: %w", err)
	}
	defer rows.Close()

	return r.scanWebhookEvents(rows)
}

// GetFailedEvents retrieves failed events for retry/DLQ analysis.
// T3.9: Hard limit of 1000 results to prevent unbounded memory allocation.
func (r *webhookEventsRepository) GetFailedEvents(ctx context.Context, limit int) ([]models.WebhookEventRecord, error) {
	const maxLimit = 1000
	if limit <= 0 {
		limit = 100
	}
	if limit > maxLimit {
		limit = maxLimit
	}

	query := `
		SELECT id, event_id, session_name, event_type, received_at, processed_at, status, error_message, retry_count
		FROM webhook_events
		WHERE status = $1
		ORDER BY received_at ASC
		LIMIT $2
	`

	rows, err := r.db.Query(ctx, query, models.WebhookEventRecordStatusFailed, limit)
	if err != nil {
		r.logger.Error("Failed to get failed webhook events",
			zap.Int("limit", limit),
			zap.Error(err))
		return nil, fmt.Errorf("get failed webhook events: %w", err)
	}
	defer rows.Close()

	return r.scanWebhookEvents(rows)
}

// Cleanup deletes old processed events for maintenance
func (r *webhookEventsRepository) Cleanup(ctx context.Context, olderThan time.Duration) (int64, error) {
	query := `DELETE FROM webhook_events WHERE status = $1 AND processed_at < $2`
	cutoff := time.Now().Add(-olderThan)

	result, err := r.db.pool.Exec(ctx, query, models.WebhookEventRecordStatusProcessed, cutoff)
	if err != nil {
		r.logger.Error("Failed to cleanup old webhook events",
			zap.Duration("olderThan", olderThan),
			zap.Error(err))
		return 0, fmt.Errorf("cleanup webhook events: %w", err)
	}

	rowsDeleted := result.RowsAffected()
	if rowsDeleted > 0 {
		r.logger.Info("Cleaned up old webhook events",
			zap.Int64("rowsDeleted", rowsDeleted),
			zap.Duration("olderThan", olderThan))
	}

	return rowsDeleted, nil
}

// CleanupUnrecoverable apaga eventos em estados terminais que a retenção de
// 'processed' nunca alcançava e que acumulavam para sempre (465k 'received' +
// 58k 'processing' + 169k 'failed' medidos em produção em 14/07): o payload
// desses eventos só existia no task do Redis e já se foi — a linha é uma
// lápide sem utilidade após a janela. Apaga em lotes (ctid + LIMIT) para não
// segurar lock/IO num DELETE gigante na tabela de ~9M linhas.
func (r *webhookEventsRepository) CleanupUnrecoverable(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)
	const batch = 50000
	query := `
		DELETE FROM webhook_events WHERE ctid IN (
			SELECT ctid FROM webhook_events
			WHERE status IN ($1, $2, $3, $4) AND received_at < $5
			LIMIT $6
		)`

	var total int64
	for i := 0; i < 40; i++ { // teto de 2M linhas por execução (roda diariamente)
		result, err := r.db.pool.Exec(ctx, query,
			models.WebhookEventRecordStatusReceived,
			models.WebhookEventRecordStatusProcessing,
			models.WebhookEventRecordStatusFailed,
			models.WebhookEventRecordStatusDLQ,
			cutoff, batch)
		if err != nil {
			r.logger.Error("Failed to cleanup unrecoverable webhook events",
				zap.Duration("olderThan", olderThan),
				zap.Error(err))
			return total, fmt.Errorf("cleanup unrecoverable webhook events: %w", err)
		}
		n := result.RowsAffected()
		total += n
		if n < batch {
			break
		}
	}

	if total > 0 {
		r.logger.Info("Cleaned up unrecoverable webhook events",
			zap.Int64("rowsDeleted", total),
			zap.Duration("olderThan", olderThan))
	}

	return total, nil
}

// scanWebhookEvents is a helper to scan rows into WebhookEvent structs
func (r *webhookEventsRepository) scanWebhookEvents(rows interface {
	Next() bool
	Scan(...interface{}) error
	Err() error
}) ([]models.WebhookEventRecord, error) {
	var events []models.WebhookEventRecord
	for rows.Next() {
		var e models.WebhookEventRecord
		err := rows.Scan(
			&e.ID,
			&e.EventID,
			&e.SessionName,
			&e.EventType,
			&e.ReceivedAt,
			&e.ProcessedAt,
			&e.Status,
			&e.ErrorMessage,
			&e.RetryCount,
		)
		if err != nil {
			return nil, fmt.Errorf("scan webhook event: %w", err)
		}
		events = append(events, e)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate webhook events: %w", err)
	}

	return events, nil
}
