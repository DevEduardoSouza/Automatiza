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

type mlCredentialsRepository struct {
	db     *DB
	logger *zap.Logger
}

// NewMLCredentialsRepository creates a new PostgreSQL ML credentials repository
func NewMLCredentialsRepository(db *DB, logger *zap.Logger) repository.MLCredentialsRepository {
	return &mlCredentialsRepository{
		db:     db,
		logger: logger,
	}
}

func (r *mlCredentialsRepository) Get(ctx context.Context, userID string) (*models.MLCredentialsDB, error) {
	query := `
		SELECT id, user_id, encrypted_data, created_at, updated_at
		FROM ml_credentials
		WHERE user_id = $1
	`

	var creds models.MLCredentialsDB
	var createdAt, updatedAt time.Time

	err := r.db.QueryRow(ctx, query, userID).Scan(
		&creds.ID, &creds.UserID, &creds.EncryptedData, &createdAt, &updatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	creds.CreatedAt = createdAt.Format(time.RFC3339)
	creds.UpdatedAt = updatedAt.Format(time.RFC3339)

	return &creds, nil
}

func (r *mlCredentialsRepository) Save(ctx context.Context, userID, encryptedData string) error {
	now := time.Now().UTC()

	query := `
		INSERT INTO ml_credentials (user_id, encrypted_data, created_at, updated_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id) DO UPDATE SET
			encrypted_data = EXCLUDED.encrypted_data,
			updated_at = EXCLUDED.updated_at
	`

	_, err := r.db.pool.Exec(ctx, query, userID, encryptedData, now, now)
	if err != nil {
		r.logger.Error("Failed to save ML credentials",
			zap.String("userID", userID),
			zap.Error(err))
		return err
	}

	return nil
}

// Delete removes ML credentials for a user
func (r *mlCredentialsRepository) Delete(ctx context.Context, userID string) error {
	query := `DELETE FROM ml_credentials WHERE user_id = $1`

	_, err := r.db.pool.Exec(ctx, query, userID)
	if err != nil {
		r.logger.Error("Failed to delete ML credentials",
			zap.String("userID", userID),
			zap.Error(err))
		return err
	}

	return nil
}

// Exists checks if ML credentials exist for a user
func (r *mlCredentialsRepository) Exists(ctx context.Context, userID string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM ml_credentials WHERE user_id = $1)`

	var exists bool
	err := r.db.QueryRow(ctx, query, userID).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}
