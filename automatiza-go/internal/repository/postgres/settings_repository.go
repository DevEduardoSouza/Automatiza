package postgres

import (
	"automatiza-go/internal/models"
	"automatiza-go/internal/observability"
	"automatiza-go/internal/repository"
	"context"
	"errors"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

type settingsRepository struct {
	db     *DB
	logger *zap.Logger
}

// NewSettingsRepository creates a new PostgreSQL settings repository
func NewSettingsRepository(db *DB, logger *zap.Logger) repository.SettingsRepository {
	return &settingsRepository{
		db:     db,
		logger: logger,
	}
}

func (r *settingsRepository) Get(ctx context.Context, keys ...string) (models.SettingsMap, error) {
	if len(keys) == 0 {
		return models.SettingsMap{}, nil
	}

	query := `SELECT key, value FROM settings WHERE key = ANY($1)`

	rows, err := r.db.Query(ctx, query, keys)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	settings := make(models.SettingsMap)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		settings[key] = value
	}

	return settings, rows.Err()
}

func (r *settingsRepository) GetAll(ctx context.Context) (models.SettingsMap, error) {
	query := `SELECT key, value FROM settings WHERE is_secret = false ORDER BY key`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	settings := make(models.SettingsMap)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		settings[key] = value
	}

	return settings, rows.Err()
}

func (r *settingsRepository) Update(ctx context.Context, key string, value interface{}) error {
	now := time.Now().UTC()

	query := `
		INSERT INTO settings (key, value, created_at, updated_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (key) DO UPDATE SET
			value = EXCLUDED.value,
			updated_at = EXCLUDED.updated_at
	`

	_, err := r.db.pool.Exec(ctx, query, key, value, now, now)
	if err != nil {
		r.logger.Error("Failed to update setting",
			zap.String("key", key),
			zap.Error(err))
		return err
	}

	return nil
}

func (r *settingsRepository) GetDispatchText(ctx context.Context, dispatchTextID string) (*models.DispatchText, error) {
	query := `
		SELECT id, user_id, name, body, is_active, created_at, updated_at
		FROM dispatch_texts
		WHERE id = $1
	`

	var dt models.DispatchText
	var createdAt, updatedAt time.Time

	err := r.db.QueryRow(ctx, query, dispatchTextID).Scan(
		&dt.ID, &dt.UserID, &dt.Name, &dt.Body, &dt.IsActive, &createdAt, &updatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	dt.CreatedAt = createdAt.Format(time.RFC3339)
	dt.UpdatedAt = updatedAt.Format(time.RFC3339)

	return &dt, nil
}

// Delete removes a setting by key
func (r *settingsRepository) Delete(ctx context.Context, key string) error {
	query := `DELETE FROM settings WHERE key = $1`

	_, err := r.db.pool.Exec(ctx, query, key)
	if err != nil {
		r.logger.Error("Failed to delete setting",
			zap.String("key", key),
			zap.Error(err))
		return err
	}

	return nil
}

// GetDispatchTextsByUser retrieves all dispatch texts for a user
func (r *settingsRepository) GetDispatchTextsByUser(ctx context.Context, userID string) ([]models.DispatchText, error) {
	query := `
		SELECT id, user_id, name, body, is_active, created_at, updated_at
		FROM dispatch_texts
		WHERE user_id = $1
		ORDER BY name
	`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var texts []models.DispatchText
	for rows.Next() {
		var dt models.DispatchText
		var createdAt, updatedAt time.Time

		err := rows.Scan(&dt.ID, &dt.UserID, &dt.Name, &dt.Body, &dt.IsActive, &createdAt, &updatedAt)
		if err != nil {
			return nil, err
		}

		dt.CreatedAt = createdAt.Format(time.RFC3339)
		dt.UpdatedAt = updatedAt.Format(time.RFC3339)
		texts = append(texts, dt)
	}

	return texts, rows.Err()
}

// CreateDispatchText creates a new dispatch text
func (r *settingsRepository) CreateDispatchText(ctx context.Context, userID string, dt *models.DispatchText) (*models.DispatchText, error) {
	now := time.Now().UTC()

	query := `
		INSERT INTO dispatch_texts (user_id, name, body, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at
	`

	var id string
	var createdAt, updatedAt time.Time

	err := r.db.QueryRow(ctx, query, userID, dt.Name, dt.Body, dt.IsActive, now, now).Scan(&id, &createdAt, &updatedAt)
	if err != nil {
		r.logger.Error("Failed to create dispatch text",
			zap.String("userID", userID),
			zap.Error(err))
		return nil, err
	}

	dt.ID = id
	dt.UserID = userID
	dt.CreatedAt = createdAt.Format(time.RFC3339)
	dt.UpdatedAt = updatedAt.Format(time.RFC3339)

	return dt, nil
}

// UpdateDispatchText updates a dispatch text
func (r *settingsRepository) UpdateDispatchText(ctx context.Context, dispatchTextID string, updates map[string]interface{}) error {
	query := `
		UPDATE dispatch_texts
		SET name = COALESCE($1, name), body = COALESCE($2, body), is_active = COALESCE($3, is_active), updated_at = $4
		WHERE id = $5
	`

	name, nameOk := updates["name"].(string)
	if !nameOk && updates["name"] != nil {
		observability.CaptureMessage(sentry.LevelWarning,
			"Unexpected type for dispatch text 'name' field in update",
			map[string]string{
				"module": "settings", "operation": "update_dispatch_text",
				"dispatch_text_id": dispatchTextID,
			})
	}
	body, bodyOk := updates["body"].(string)
	if !bodyOk && updates["body"] != nil {
		observability.CaptureMessage(sentry.LevelWarning,
			"Unexpected type for dispatch text 'body' field in update",
			map[string]string{
				"module": "settings", "operation": "update_dispatch_text",
				"dispatch_text_id": dispatchTextID,
			})
	}

	// Handle is_active - need to use pointer to allow COALESCE
	var isActive *bool
	if v, ok := updates["is_active"]; ok {
		if b, ok := v.(bool); ok {
			isActive = &b
		}
	}

	_, err := r.db.pool.Exec(ctx, query, nilIfEmpty(name), nilIfEmpty(body), isActive, time.Now().UTC(), dispatchTextID)
	return err
}

// DeleteDispatchText deletes a dispatch text
func (r *settingsRepository) DeleteDispatchText(ctx context.Context, dispatchTextID, userID string) error {
	query := `DELETE FROM dispatch_texts WHERE id = $1 AND user_id = $2`

	_, err := r.db.pool.Exec(ctx, query, dispatchTextID, userID)
	return err
}

// GetActiveDispatchTextByID retrieves an active dispatch text by ID and user
func (r *settingsRepository) GetActiveDispatchTextByID(ctx context.Context, dispatchTextID, userID string) (*models.DispatchText, error) {
	query := `
		SELECT id, user_id, name, body, is_active, created_at, updated_at
		FROM dispatch_texts
		WHERE id = $1 AND user_id = $2 AND is_active = true
	`

	var dt models.DispatchText
	var createdAt, updatedAt time.Time

	err := r.db.QueryRow(ctx, query, dispatchTextID, userID).Scan(
		&dt.ID, &dt.UserID, &dt.Name, &dt.Body, &dt.IsActive, &createdAt, &updatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	dt.CreatedAt = createdAt.Format(time.RFC3339)
	dt.UpdatedAt = updatedAt.Format(time.RFC3339)

	return &dt, nil
}

// GetActiveDispatchTextsByUser retrieves all active dispatch texts for a user
func (r *settingsRepository) GetActiveDispatchTextsByUser(ctx context.Context, userID string) ([]models.DispatchText, error) {
	query := `
		SELECT id, user_id, name, body, is_active, created_at, updated_at
		FROM dispatch_texts
		WHERE user_id = $1 AND is_active = true
		ORDER BY name
	`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var texts []models.DispatchText
	for rows.Next() {
		var dt models.DispatchText
		var createdAt, updatedAt time.Time

		err := rows.Scan(&dt.ID, &dt.UserID, &dt.Name, &dt.Body, &dt.IsActive, &createdAt, &updatedAt)
		if err != nil {
			return nil, err
		}

		dt.CreatedAt = createdAt.Format(time.RFC3339)
		dt.UpdatedAt = updatedAt.Format(time.RFC3339)
		texts = append(texts, dt)
	}

	return texts, rows.Err()
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
