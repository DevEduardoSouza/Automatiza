package postgres

import (
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"
	"context"
	"encoding/json"
	"time"

	"go.uber.org/zap"
)

type systemEventRepository struct {
	db     *DB
	logger *zap.Logger
}

// NewSystemEventRepository creates a new PostgreSQL system event repository
func NewSystemEventRepository(db *DB, logger *zap.Logger) repository.SystemEventRepository {
	return &systemEventRepository{
		db:     db,
		logger: logger,
	}
}

func (r *systemEventRepository) Create(ctx context.Context, eventType string, payload map[string]interface{}) error {
	now := time.Now().UTC()

	var payloadJSON []byte
	if payload != nil {
		payloadJSON, _ = json.Marshal(payload)
	}

	query := `
		INSERT INTO system_events (event_type, payload, created_at)
		VALUES ($1, $2, $3)
	`

	_, err := r.db.pool.Exec(ctx, query, eventType, payloadJSON, now)
	if err != nil {
		r.logger.Error("Failed to create system event",
			zap.String("eventType", eventType),
			zap.Error(err))
		return err
	}

	return nil
}

func (r *systemEventRepository) GetRecent(ctx context.Context, limit int) ([]models.SystemEvent, error) {
	if limit <= 0 {
		limit = 100
	}

	query := `
		SELECT id, event_type, payload, created_at
		FROM system_events
		ORDER BY created_at DESC
		LIMIT $1
	`

	rows, err := r.db.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []models.SystemEvent
	for rows.Next() {
		var e models.SystemEvent
		var payloadJSON []byte
		var createdAt time.Time

		err := rows.Scan(&e.ID, &e.EventType, &payloadJSON, &createdAt)
		if err != nil {
			return nil, err
		}

		if payloadJSON != nil {
			_ = json.Unmarshal(payloadJSON, &e.Payload)
		}

		e.CreatedAt = createdAt.Format(time.RFC3339)
		events = append(events, e)
	}

	return events, rows.Err()
}

// GetByType retrieves events by type
func (r *systemEventRepository) GetByType(ctx context.Context, eventType string, limit int) ([]models.SystemEvent, error) {
	if limit <= 0 {
		limit = 50
	}

	query := `
		SELECT id, event_type, payload, created_at
		FROM system_events
		WHERE event_type = $1
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := r.db.Query(ctx, query, eventType, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []models.SystemEvent
	for rows.Next() {
		var e models.SystemEvent
		var payloadJSON []byte
		var createdAt time.Time

		err := rows.Scan(&e.ID, &e.EventType, &payloadJSON, &createdAt)
		if err != nil {
			return nil, err
		}

		if payloadJSON != nil {
			_ = json.Unmarshal(payloadJSON, &e.Payload)
		}

		e.CreatedAt = createdAt.Format(time.RFC3339)
		events = append(events, e)
	}

	return events, rows.Err()
}

// DeleteOld deletes events older than the specified duration
func (r *systemEventRepository) DeleteOld(ctx context.Context, olderThan time.Duration) (int64, error) {
	query := `DELETE FROM system_events WHERE created_at < $1`
	cutoff := time.Now().Add(-olderThan)

	result, err := r.db.pool.Exec(ctx, query, cutoff)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected(), nil
}

// Count returns the total number of events
func (r *systemEventRepository) Count(ctx context.Context) (int, error) {
	query := `SELECT COUNT(*) FROM system_events`

	var count int
	err := r.db.QueryRow(ctx, query).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}
