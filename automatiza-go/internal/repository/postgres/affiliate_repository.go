package postgres

import (
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

type affiliateRepository struct {
	db     *DB
	logger *zap.Logger
}

// NewAffiliateCredentialsRepository creates a new PostgreSQL affiliate credentials repository
func NewAffiliateCredentialsRepository(db *DB, logger *zap.Logger) repository.AffiliateCredentialsRepository {
	return &affiliateRepository{
		db:     db,
		logger: logger,
	}
}

// scanCredentials scans a row into an AffiliateCredentials struct
func (r *affiliateRepository) scanCredentials(row pgx.Row) (*models.AffiliateCredentials, error) {
	var creds models.AffiliateCredentials
	var createdAt, updatedAt time.Time
	var credentialsJSON []byte
	var accessToken, refreshToken, tokenExpiresAt *string

	err := row.Scan(
		&creds.ID,
		&creds.UserID,
		&creds.Store,
		&creds.AppID,
		&creds.AppSecret,
		&credentialsJSON,
		&creds.AutoGenerate,
		&creds.IsActive,
		&accessToken,
		&refreshToken,
		&tokenExpiresAt,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	// Parse credentials JSONB
	if len(credentialsJSON) > 0 && string(credentialsJSON) != "null" {
		if err := json.Unmarshal(credentialsJSON, &creds.Credentials); err != nil {
			r.logger.Warn("Failed to parse credentials JSON, using legacy fields",
				zap.String("userID", creds.UserID),
				zap.String("store", creds.Store),
				zap.Error(err))
		}
	}

	// Set optional OAuth fields
	creds.AccessToken = accessToken
	creds.RefreshToken = refreshToken
	creds.TokenExpiresAt = tokenExpiresAt

	creds.CreatedAt = createdAt.Format(time.RFC3339)
	creds.UpdatedAt = updatedAt.Format(time.RFC3339)

	return &creds, nil
}

// baseSelectQuery returns the base SELECT query for credentials
func (r *affiliateRepository) baseSelectQuery() string {
	return `
		SELECT id, user_id, store, app_id, app_secret, credentials,
		       COALESCE(auto_generate, true) as auto_generate,
		       COALESCE(is_active, true) as is_active,
		       access_token, refresh_token, token_expires_at,
		       created_at, updated_at
		FROM affiliate_credentials
	`
}

func (r *affiliateRepository) GetByUserAndStore(ctx context.Context, userID, store string) (*models.AffiliateCredentials, error) {
	query := r.baseSelectQuery() + `
		WHERE user_id = $1 AND store = $2 AND COALESCE(is_active, true) = true
	`

	row := r.db.QueryRow(ctx, query, userID, store)
	return r.scanCredentials(row)
}

// GetAllByUser retrieves all active affiliate credentials for a user
func (r *affiliateRepository) GetAllByUser(ctx context.Context, userID string) ([]models.AffiliateCredentials, error) {
	query := r.baseSelectQuery() + `
		WHERE user_id = $1 AND COALESCE(is_active, true) = true
		ORDER BY store
	`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var creds []models.AffiliateCredentials
	for rows.Next() {
		var c models.AffiliateCredentials
		var createdAt, updatedAt time.Time
		var credentialsJSON []byte
		var accessToken, refreshToken, tokenExpiresAt *string

		err := rows.Scan(
			&c.ID,
			&c.UserID,
			&c.Store,
			&c.AppID,
			&c.AppSecret,
			&credentialsJSON,
			&c.AutoGenerate,
			&c.IsActive,
			&accessToken,
			&refreshToken,
			&tokenExpiresAt,
			&createdAt,
			&updatedAt,
		)
		if err != nil {
			return nil, err
		}

		// Parse credentials JSONB
		if len(credentialsJSON) > 0 && string(credentialsJSON) != "null" {
			if err := json.Unmarshal(credentialsJSON, &c.Credentials); err != nil {
				r.logger.Warn("Failed to parse credentials JSON",
					zap.String("userID", c.UserID),
					zap.String("store", c.Store),
					zap.Error(err))
			}
		}

		c.AccessToken = accessToken
		c.RefreshToken = refreshToken
		c.TokenExpiresAt = tokenExpiresAt
		c.CreatedAt = createdAt.Format(time.RFC3339)
		c.UpdatedAt = updatedAt.Format(time.RFC3339)
		creds = append(creds, c)
	}

	return creds, rows.Err()
}

func (r *affiliateRepository) Upsert(ctx context.Context, userID, store string, data *models.AffiliateCredentials) error {
	now := time.Now().UTC()

	// Serialize credentials map to JSON
	var credentialsJSON []byte
	var err error
	if data.Credentials != nil {
		credentialsJSON, err = json.Marshal(data.Credentials)
		if err != nil {
			return err
		}
	}

	query := `
		INSERT INTO affiliate_credentials (
			user_id, store, app_id, app_secret, credentials, auto_generate, is_active,
			access_token, refresh_token, token_expires_at, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (user_id, store) DO UPDATE SET
			app_id = EXCLUDED.app_id,
			app_secret = EXCLUDED.app_secret,
			credentials = EXCLUDED.credentials,
			auto_generate = EXCLUDED.auto_generate,
			is_active = EXCLUDED.is_active,
			access_token = EXCLUDED.access_token,
			refresh_token = EXCLUDED.refresh_token,
			token_expires_at = EXCLUDED.token_expires_at,
			updated_at = EXCLUDED.updated_at
	`

	// Parse token_expires_at if provided
	var tokenExpiresAt *time.Time
	if data.TokenExpiresAt != nil && *data.TokenExpiresAt != "" {
		parsed, err := time.Parse(time.RFC3339, *data.TokenExpiresAt)
		if err == nil {
			tokenExpiresAt = &parsed
		}
	}

	_, err = r.db.pool.Exec(ctx, query,
		userID,
		store,
		data.AppID,
		data.AppSecret,
		credentialsJSON,
		data.AutoGenerate,
		true, // is_active = true on upsert
		data.AccessToken,
		data.RefreshToken,
		tokenExpiresAt,
		now,
		now,
	)
	if err != nil {
		r.logger.Error("Failed to upsert affiliate credentials",
			zap.String("userID", userID),
			zap.String("store", store),
			zap.Error(err))
		return err
	}

	return nil
}

// Delete removes affiliate credentials for a user and store (hard delete)
func (r *affiliateRepository) Delete(ctx context.Context, userID, store string) error {
	query := `DELETE FROM affiliate_credentials WHERE user_id = $1 AND store = $2`

	result, err := r.db.pool.Exec(ctx, query, userID, store)
	if err != nil {
		r.logger.Error("Failed to delete affiliate credentials",
			zap.String("userID", userID),
			zap.String("store", store),
			zap.Error(err))
		return err
	}

	if result.RowsAffected() == 0 {
		r.logger.Debug("No credentials found to delete",
			zap.String("userID", userID),
			zap.String("store", store))
	}

	return nil
}

// Deactivate soft deletes credentials (sets is_active = false)
func (r *affiliateRepository) Deactivate(ctx context.Context, userID, store string) error {
	query := `
		UPDATE affiliate_credentials
		SET is_active = false, updated_at = $3
		WHERE user_id = $1 AND store = $2
	`

	result, err := r.db.pool.Exec(ctx, query, userID, store, time.Now().UTC())
	if err != nil {
		r.logger.Error("Failed to deactivate affiliate credentials",
			zap.String("userID", userID),
			zap.String("store", store),
			zap.Error(err))
		return err
	}

	if result.RowsAffected() == 0 {
		r.logger.Debug("No credentials found to deactivate",
			zap.String("userID", userID),
			zap.String("store", store))
	}

	return nil
}

// GetActiveStores returns a list of stores with active credentials for a user
func (r *affiliateRepository) GetActiveStores(ctx context.Context, userID string) ([]string, error) {
	query := `
		SELECT store
		FROM affiliate_credentials
		WHERE user_id = $1 AND COALESCE(is_active, true) = true
		ORDER BY store
	`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stores []string
	for rows.Next() {
		var store string
		if err := rows.Scan(&store); err != nil {
			return nil, err
		}
		stores = append(stores, store)
	}

	return stores, rows.Err()
}
