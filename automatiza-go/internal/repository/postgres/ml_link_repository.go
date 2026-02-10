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

type mlLinkRepository struct {
	db     *DB
	logger *zap.Logger
}

// NewMLLinkRepository creates a new PostgreSQL ML link repository
func NewMLLinkRepository(db *DB, logger *zap.Logger) repository.MLLinkRepository {
	return &mlLinkRepository{
		db:     db,
		logger: logger,
	}
}

func (r *mlLinkRepository) GetByOriginalURL(ctx context.Context, userID, originalURL string) (*models.MLLinkEntry, error) {
	query := `
		SELECT id, user_id, original_url, short_url, product_id, created_at
		FROM ml_link_entries
		WHERE user_id = $1 AND original_url = $2
	`

	var link models.MLLinkEntry
	var createdAt time.Time

	err := r.db.QueryRow(ctx, query, userID, originalURL).Scan(
		&link.ID, &link.UserID, &link.OriginalURL, &link.ShortURL, &link.ProductID, &createdAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	link.CreatedAt = createdAt.Format(time.RFC3339)
	return &link, nil
}

func (r *mlLinkRepository) Create(ctx context.Context, link *models.MLLinkEntry) error {
	now := time.Now().UTC()

	query := `
		INSERT INTO ml_link_entries (user_id, original_url, short_url, product_id, created_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id, original_url) DO UPDATE SET
			short_url = EXCLUDED.short_url,
			product_id = EXCLUDED.product_id
		RETURNING id, created_at
	`

	var id string
	var createdAt time.Time

	err := r.db.QueryRow(ctx, query, link.UserID, link.OriginalURL, link.ShortURL, link.ProductID, now).Scan(&id, &createdAt)
	if err != nil {
		r.logger.Error("Failed to create ML link entry",
			zap.String("userID", link.UserID),
			zap.Error(err))
		return err
	}

	link.ID = id
	link.CreatedAt = createdAt.Format(time.RFC3339)
	return nil
}

func (r *mlLinkRepository) GetRecentByUser(ctx context.Context, userID string, limit int) ([]models.MLLinkEntry, error) {
	if limit <= 0 {
		limit = 50
	}

	query := `
		SELECT id, user_id, original_url, short_url, product_id, created_at
		FROM ml_link_entries
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := r.db.Query(ctx, query, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var links []models.MLLinkEntry
	for rows.Next() {
		var link models.MLLinkEntry
		var createdAt time.Time

		err := rows.Scan(&link.ID, &link.UserID, &link.OriginalURL, &link.ShortURL, &link.ProductID, &createdAt)
		if err != nil {
			return nil, err
		}

		link.CreatedAt = createdAt.Format(time.RFC3339)
		links = append(links, link)
	}

	return links, rows.Err()
}

// Delete removes an ML link entry
func (r *mlLinkRepository) Delete(ctx context.Context, id, userID string) error {
	query := `DELETE FROM ml_link_entries WHERE id = $1 AND user_id = $2`

	_, err := r.db.pool.Exec(ctx, query, id, userID)
	return err
}

// DeleteByUser removes all ML link entries for a user
func (r *mlLinkRepository) DeleteByUser(ctx context.Context, userID string) error {
	query := `DELETE FROM ml_link_entries WHERE user_id = $1`

	_, err := r.db.pool.Exec(ctx, query, userID)
	return err
}

// Count returns the number of ML link entries for a user
func (r *mlLinkRepository) Count(ctx context.Context, userID string) (int, error) {
	query := `SELECT COUNT(*) FROM ml_link_entries WHERE user_id = $1`

	var count int
	err := r.db.QueryRow(ctx, query, userID).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}
