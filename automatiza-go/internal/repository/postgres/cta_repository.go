package postgres

import (
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

type ctaRepository struct {
	db     *DB
	logger *zap.Logger
}

// NewCTARepository creates a new PostgreSQL CTA repository
func NewCTARepository(db *DB, logger *zap.Logger) repository.CTARepository {
	return &ctaRepository{
		db:     db,
		logger: logger,
	}
}

func (r *ctaRepository) GetUserCTAs(ctx context.Context, userID string, filters map[string]interface{}) ([]models.CTA, error) {
	query := `
		SELECT id, user_id, message, platform, cta_type, is_active, created_at, updated_at
		FROM ctas
		WHERE user_id = $1
	`
	args := []interface{}{userID}
	argIndex := 2

	if platform, ok := filters["platform"].(string); ok && platform != "" {
		query += fmt.Sprintf(" AND platform = $%d", argIndex)
		args = append(args, platform)
		argIndex++
	}
	if ctaType, ok := filters["cta_type"].(string); ok && ctaType != "" {
		query += fmt.Sprintf(" AND cta_type = $%d", argIndex)
		args = append(args, ctaType)
		argIndex++
	}
	if isActive, ok := filters["is_active"].(bool); ok {
		query += fmt.Sprintf(" AND is_active = $%d", argIndex)
		args = append(args, isActive)
	}

	query += " ORDER BY created_at DESC"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanCTAs(rows)
}

func (r *ctaRepository) GetCTAByID(ctx context.Context, ctaID, userID string) (*models.CTA, error) {
	query := `
		SELECT id, user_id, message, platform, cta_type, is_active, created_at, updated_at
		FROM ctas
		WHERE id = $1 AND user_id = $2
	`

	return r.scanCTA(r.db.QueryRow(ctx, query, ctaID, userID))
}

func (r *ctaRepository) GetCTAsByIDs(ctx context.Context, ctaIDs []string, userID string) ([]models.CTA, error) {
	if len(ctaIDs) == 0 {
		return nil, nil
	}

	query := `
		SELECT id, user_id, message, platform, cta_type, is_active, created_at, updated_at
		FROM ctas
		WHERE id = ANY($1) AND user_id = $2 AND is_active = true
	`

	rows, err := r.db.Query(ctx, query, ctaIDs, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanCTAs(rows)
}

func (r *ctaRepository) CreateCTA(ctx context.Context, userID string, data *models.CTA) (*models.CTA, error) {
	now := time.Now().UTC()

	query := `
		INSERT INTO ctas (user_id, message, platform, cta_type, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at
	`

	var id string
	var createdAt, updatedAt time.Time

	err := r.db.QueryRow(ctx, query,
		userID, data.Message, data.Platform, data.CTAType, data.IsActive, now, now,
	).Scan(&id, &createdAt, &updatedAt)

	if err != nil {
		r.logger.Error("Failed to create CTA",
			zap.String("userID", userID),
			zap.Error(err))
		return nil, err
	}

	data.ID = id
	data.UserID = userID
	data.CreatedAt = createdAt.Format(time.RFC3339)
	data.UpdatedAt = updatedAt.Format(time.RFC3339)

	return data, nil
}

func (r *ctaRepository) UpdateCTA(ctx context.Context, ctaID, userID string, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return nil
	}

	setClauses := ""
	args := make([]interface{}, 0, len(updates)+2)
	argIndex := 1

	for key, value := range updates {
		if setClauses != "" {
			setClauses += ", "
		}
		setClauses += fmt.Sprintf("%s = $%d", key, argIndex)
		args = append(args, value)
		argIndex++
	}

	// Always update updated_at
	setClauses += fmt.Sprintf(", updated_at = $%d", argIndex)
	args = append(args, time.Now().UTC())
	argIndex++

	args = append(args, ctaID, userID)

	query := fmt.Sprintf("UPDATE ctas SET %s WHERE id = $%d AND user_id = $%d", setClauses, argIndex, argIndex+1)

	_, err := r.db.pool.Exec(ctx, query, args...)
	if err != nil {
		r.logger.Error("Failed to update CTA",
			zap.String("ctaID", ctaID),
			zap.Error(err))
		return err
	}

	return nil
}

func (r *ctaRepository) DeleteCTA(ctx context.Context, ctaID, userID string) error {
	query := `DELETE FROM ctas WHERE id = $1 AND user_id = $2`

	_, err := r.db.pool.Exec(ctx, query, ctaID, userID)
	if err != nil {
		r.logger.Error("Failed to delete CTA",
			zap.String("ctaID", ctaID),
			zap.Error(err))
		return err
	}

	return nil
}

// Helper functions

func (r *ctaRepository) scanCTA(row pgx.Row) (*models.CTA, error) {
	var c models.CTA
	var createdAt, updatedAt time.Time

	err := row.Scan(
		&c.ID, &c.UserID, &c.Message, &c.Platform, &c.CTAType, &c.IsActive,
		&createdAt, &updatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	c.CreatedAt = createdAt.Format(time.RFC3339)
	c.UpdatedAt = updatedAt.Format(time.RFC3339)

	return &c, nil
}

func (r *ctaRepository) scanCTAs(rows pgx.Rows) ([]models.CTA, error) {
	var ctas []models.CTA
	for rows.Next() {
		var c models.CTA
		var createdAt, updatedAt time.Time

		err := rows.Scan(
			&c.ID, &c.UserID, &c.Message, &c.Platform, &c.CTAType, &c.IsActive,
			&createdAt, &updatedAt,
		)
		if err != nil {
			return nil, err
		}

		c.CreatedAt = createdAt.Format(time.RFC3339)
		c.UpdatedAt = updatedAt.Format(time.RFC3339)

		ctas = append(ctas, c)
	}

	return ctas, rows.Err()
}
