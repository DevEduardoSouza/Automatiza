package postgres

import (
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

type webPushSubscriptionRepository struct {
	db     *DB
	logger *zap.Logger
}

// NewWebPushSubscriptionRepository creates a PostgreSQL Web Push subscription repository.
func NewWebPushSubscriptionRepository(db *DB, logger *zap.Logger) repository.WebPushSubscriptionRepository {
	return &webPushSubscriptionRepository{db: db, logger: logger}
}

func scanWebPushSub(row pgx.Row) (*models.WebPushSubscription, error) {
	var s models.WebPushSubscription
	if err := row.Scan(&s.ID, &s.UserID, &s.Endpoint, &s.P256dh, &s.Auth, &s.UserAgent, &s.CreatedAt, &s.LastSeenAt, &s.FailedAt); err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *webPushSubscriptionRepository) Upsert(ctx context.Context, sub *models.WebPushSubscription) error {
	const query = `
		INSERT INTO web_push_subscriptions (user_id, endpoint, p256dh, auth, user_agent, last_seen_at, failed_at)
		VALUES ($1, $2, $3, $4, $5, now(), NULL)
		ON CONFLICT (endpoint) DO UPDATE SET
			user_id      = EXCLUDED.user_id,
			p256dh       = EXCLUDED.p256dh,
			auth         = EXCLUDED.auth,
			user_agent   = EXCLUDED.user_agent,
			last_seen_at = now(),
			failed_at    = NULL`
	if _, err := r.db.pool.Exec(ctx, query, sub.UserID, sub.Endpoint, sub.P256dh, sub.Auth, sub.UserAgent); err != nil {
		r.logger.Error("Failed to upsert web push subscription", zap.String("userID", sub.UserID), zap.Error(err))
		return err
	}
	return nil
}

func (r *webPushSubscriptionRepository) ListByUser(ctx context.Context, userID string) ([]models.WebPushSubscription, error) {
	return r.query(ctx, `
		SELECT id, user_id, endpoint, p256dh, auth, user_agent, created_at, last_seen_at, failed_at
		FROM web_push_subscriptions WHERE user_id = $1`, userID)
}

func (r *webPushSubscriptionRepository) ListAll(ctx context.Context) ([]models.WebPushSubscription, error) {
	return r.query(ctx, `
		SELECT id, user_id, endpoint, p256dh, auth, user_agent, created_at, last_seen_at, failed_at
		FROM web_push_subscriptions WHERE failed_at IS NULL`)
}

func (r *webPushSubscriptionRepository) query(ctx context.Context, sql string, args ...interface{}) ([]models.WebPushSubscription, error) {
	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.WebPushSubscription
	for rows.Next() {
		s, err := scanWebPushSub(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *s)
	}
	return out, rows.Err()
}

func (r *webPushSubscriptionRepository) DeleteByEndpoint(ctx context.Context, endpoint string) error {
	if _, err := r.db.pool.Exec(ctx, `DELETE FROM web_push_subscriptions WHERE endpoint = $1`, endpoint); err != nil {
		r.logger.Error("Failed to delete web push subscription", zap.Error(err))
		return err
	}
	return nil
}

func (r *webPushSubscriptionRepository) DeleteFailedBefore(ctx context.Context, before time.Time) (int64, error) {
	tag, err := r.db.pool.Exec(ctx, `DELETE FROM web_push_subscriptions WHERE failed_at IS NOT NULL AND failed_at < $1`, before)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (r *webPushSubscriptionRepository) CountByUser(ctx context.Context, userID string) (int, error) {
	var n int
	err := r.db.QueryRow(ctx, `SELECT count(*) FROM web_push_subscriptions WHERE user_id = $1`, userID).Scan(&n)
	return n, err
}
