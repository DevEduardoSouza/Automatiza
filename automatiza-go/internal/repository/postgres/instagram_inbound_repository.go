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

type instagramInboundRepository struct {
	db     *DB
	logger *zap.Logger
}

// NewInstagramInboundRepository creates the PostgreSQL inbound-event repository.
func NewInstagramInboundRepository(db *DB, logger *zap.Logger) repository.InstagramInboundRepository {
	return &instagramInboundRepository{db: db, logger: logger}
}

const igInboundColumns = `id, user_id, provider_event_id, event_type, ig_account_id, sender_id,
	thread_id, content, occurred_at, received_at, status, error_code`

func scanIGInbound(row pgx.Row) (*models.InstagramInboundEvent, error) {
	var e models.InstagramInboundEvent
	var receivedAt time.Time
	var occurredAt *time.Time
	if err := row.Scan(&e.ID, &e.UserID, &e.ProviderEventID, &e.EventType, &e.IGAccountID,
		&e.SenderID, &e.ThreadID, &e.Content, &occurredAt, &receivedAt, &e.Status, &e.ErrorCode); err != nil {
		return nil, err
	}
	e.OccurredAt = occurredAt
	e.ReceivedAt = receivedAt.Format(time.RFC3339)
	return &e, nil
}

// Create é o ponto de idempotência da entrada. O ON CONFLICT DO NOTHING contra
// o índice único de provider_event_id garante que um reenvio da Meta — que
// acontece sempre que o ACK demora — não vire uma segunda resposta ao mesmo
// comentário.
func (r *instagramInboundRepository) Create(ctx context.Context, e *models.InstagramInboundEvent) (*models.InstagramInboundEvent, bool, error) {
	const insertQuery = `
		INSERT INTO instagram_inbound_events
			(user_id, provider_event_id, event_type, ig_account_id, sender_id, thread_id, content, occurred_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (provider_event_id) DO NOTHING
		RETURNING ` + igInboundColumns

	row, err := scanIGInbound(r.db.QueryRow(ctx, insertQuery,
		e.UserID, e.ProviderEventID, e.EventType, e.IGAccountID, e.SenderID, e.ThreadID, e.Content, e.OccurredAt))
	if err == nil {
		return row, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, false, err
	}

	const selectQuery = `SELECT ` + igInboundColumns + ` FROM instagram_inbound_events WHERE provider_event_id = $1`
	existing, err := scanIGInbound(r.db.QueryRow(ctx, selectQuery, e.ProviderEventID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, false, errors.New("instagram inbound: row vanished between insert and select")
		}
		return nil, false, err
	}
	return existing, false, nil
}

func (r *instagramInboundRepository) ListRecentByUser(ctx context.Context, userID string, limit int) ([]models.InstagramInboundEvent, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	const query = `
		SELECT ` + igInboundColumns + `
		FROM instagram_inbound_events
		WHERE user_id = $1
		ORDER BY received_at DESC
		LIMIT $2`
	rows, err := r.db.Query(ctx, query, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.InstagramInboundEvent
	for rows.Next() {
		e, err := scanIGInbound(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *e)
	}
	return out, rows.Err()
}

func (r *instagramInboundRepository) MarkStatus(ctx context.Context, id, status, errorCode string) error {
	const query = `UPDATE instagram_inbound_events SET status = $2, error_code = $3 WHERE id = $1`
	_, err := r.db.pool.Exec(ctx, query, id, status, errorCode)
	return err
}

// DeleteOlderThan aplica a retenção de 30 dias. Content é texto de terceiro:
// guardá-lo para sempre seria retenção sem finalidade.
func (r *instagramInboundRepository) DeleteOlderThan(ctx context.Context, before time.Time, limit int) (int64, error) {
	if limit <= 0 {
		return 0, nil
	}
	const query = `
		WITH doomed AS (
			SELECT id FROM instagram_inbound_events
			WHERE received_at < $1
			ORDER BY received_at, id
			LIMIT $2
		)
		DELETE FROM instagram_inbound_events e USING doomed d WHERE e.id = d.id`
	tag, err := r.db.pool.Exec(ctx, query, before, limit)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
