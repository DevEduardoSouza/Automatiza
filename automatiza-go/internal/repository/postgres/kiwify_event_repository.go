package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"automatiza-go/internal/observability"
	"automatiza-go/internal/repository"

	"github.com/getsentry/sentry-go"
	"go.uber.org/zap"
)

type kiwifyEventRepository struct {
	db     *DB
	logger *zap.Logger
}

// NewKiwifyEventRepository creates a PostgreSQL-backed Kiwify webhook
// idempotency/audit repository.
func NewKiwifyEventRepository(db *DB, logger *zap.Logger) repository.KiwifyEventRepository {
	return &kiwifyEventRepository{db: db, logger: logger}
}

// Claim atomically reserves an idempotency key. The unique PRIMARY KEY makes the
// INSERT the concurrency control: only the first delivery inserts a row
// (claimed=true); concurrent or repeated deliveries conflict (claimed=false) and
// receive the prior status so the caller can decide to skip.
func (r *kiwifyEventRepository) Claim(ctx context.Context, key, email, event string) (bool, string, error) {
	now := time.Now().UTC()
	// Try to claim.
	res, err := r.db.pool.Exec(ctx, `
		INSERT INTO kiwify_webhook_events (idempotency_key, email, event, status, created_at, updated_at)
		VALUES ($1, $2, $3, 'processing', $4, $4)
		ON CONFLICT (idempotency_key) DO NOTHING
	`, key, email, event, now)
	if err != nil {
		return false, "", fmt.Errorf("claim kiwify event: %w", err)
	}
	if res.RowsAffected() == 1 {
		return true, "", nil
	}

	// Conflict: someone already claimed it. Fetch the prior status.
	var priorStatus string
	if scanErr := r.db.QueryRow(ctx,
		`SELECT status FROM kiwify_webhook_events WHERE idempotency_key = $1`, key,
	).Scan(&priorStatus); scanErr != nil {
		// Row vanished between INSERT-conflict and SELECT (rare: a concurrent
		// Release). Treat as not-claimed with unknown status; caller skips.
		return false, "", nil
	}
	return false, priorStatus, nil
}

// Finish marks a claimed key as done, recording the user and result.
func (r *kiwifyEventRepository) Finish(ctx context.Context, key, userID, result string) error {
	now := time.Now().UTC()
	var userIDArg interface{}
	if userID != "" {
		userIDArg = userID
	}
	_, err := r.db.pool.Exec(ctx, `
		UPDATE kiwify_webhook_events
		SET status = 'done', user_id = $2, result = $3, updated_at = $4
		WHERE idempotency_key = $1
	`, key, userIDArg, result, now)
	if err != nil {
		r.logger.Warn("Failed to mark kiwify event done", zap.String("key", key), zap.Error(err))
		return fmt.Errorf("finish kiwify event: %w", err)
	}
	return nil
}

// Release deletes a claimed key after a processing failure so the retry can
// reprocess it instead of being deduplicated.
func (r *kiwifyEventRepository) Release(ctx context.Context, key string) error {
	_, err := r.db.pool.Exec(ctx,
		`DELETE FROM kiwify_webhook_events WHERE idempotency_key = $1 AND status = 'processing'`, key)
	if err != nil {
		r.logger.Warn("Failed to release kiwify event key", zap.String("key", key), zap.Error(err))
		return fmt.Errorf("release kiwify event: %w", err)
	}
	return nil
}

// RecordBillingAudit appends a billing state-transition audit event to
// system_events (the established audit sink), mirroring the admin handler.
func (r *kiwifyEventRepository) RecordBillingAudit(ctx context.Context, payload map[string]interface{}) error {
	now := time.Now().UTC()
	var payloadJSON []byte
	if payload != nil {
		var marshalErr error
		payloadJSON, marshalErr = json.Marshal(payload)
		if marshalErr != nil {
			observability.LogAndCapture(r.logger, sentry.LevelWarning,
				"Failed to marshal billing audit payload", marshalErr, map[string]string{
					"module": "kiwify", "operation": "billing_audit_marshal",
				})
			payloadJSON = []byte("null")
		}
	}
	_, err := r.db.pool.Exec(ctx, `
		INSERT INTO system_events (event_type, payload, created_at)
		VALUES ('billing_transition', $1, $2)
	`, payloadJSON, now)
	if err != nil {
		r.logger.Warn("Failed to record billing audit", zap.Error(err))
		return fmt.Errorf("record billing audit: %w", err)
	}
	return nil
}
