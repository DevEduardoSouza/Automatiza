package postgres

import (
	"automatiza-go/internal/models"
	"automatiza-go/internal/observability"
	"automatiza-go/internal/repository"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

type couponRepository struct {
	db     *DB
	logger *zap.Logger
}

// NewCouponRepository creates a new PostgreSQL coupon repository
func NewCouponRepository(db *DB, logger *zap.Logger) repository.CouponRepository {
	return &couponRepository{
		db:     db,
		logger: logger,
	}
}

func (r *couponRepository) GetUserCoupons(ctx context.Context, userID string, filters map[string]interface{}) ([]models.Coupon, error) {
	query := `
		SELECT id, user_id, code, title, store, affiliate_link, expires_at, is_active,
		       category, discount_type, discount_value, max_discount, min_product_price,
		       coupon_text, created_at, updated_at
		FROM coupons
		WHERE user_id = $1
	`
	args := []interface{}{userID}
	argIndex := 2

	// Apply filters
	if store, ok := filters["store"].(string); ok && store != "" {
		query += fmt.Sprintf(" AND store = $%d", argIndex)
		args = append(args, store)
		argIndex++
	}
	if isActive, ok := filters["is_active"].(bool); ok {
		query += fmt.Sprintf(" AND is_active = $%d", argIndex)
		args = append(args, isActive)
		argIndex++
	}
	if category, ok := filters["category"].(string); ok && category != "" {
		query += fmt.Sprintf(" AND category = $%d", argIndex)
		args = append(args, category)
		argIndex++
	}

	query += " ORDER BY created_at DESC"

	// Apply pagination
	if limit, ok := filters["limit"].(int); ok && limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIndex)
		args = append(args, limit)
		argIndex++
	}
	if offset, ok := filters["offset"].(int); ok && offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argIndex)
		args = append(args, offset)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanCoupons(rows)
}

func (r *couponRepository) GetCouponByID(ctx context.Context, couponID, userID string) (*models.Coupon, error) {
	query := `
		SELECT id, user_id, code, title, store, affiliate_link, expires_at, is_active,
		       category, discount_type, discount_value, max_discount, min_product_price,
		       coupon_text, created_at, updated_at
		FROM coupons
		WHERE id = $1 AND user_id = $2
	`

	return r.scanCoupon(r.db.QueryRow(ctx, query, couponID, userID))
}

func (r *couponRepository) GetCouponByCode(ctx context.Context, userID, code string) (*models.Coupon, error) {
	query := `
		SELECT id, user_id, code, title, store, affiliate_link, expires_at, is_active,
		       category, discount_type, discount_value, max_discount, min_product_price,
		       coupon_text, created_at, updated_at
		FROM coupons
		WHERE user_id = $1 AND UPPER(code) = UPPER($2)
		LIMIT 1
	`

	coupon, err := r.scanCoupon(r.db.QueryRow(ctx, query, userID, code))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // Not found = nil, nil
		}
		return nil, fmt.Errorf("failed to get coupon by code: %w", err)
	}
	return coupon, nil
}

func (r *couponRepository) CreateCoupon(ctx context.Context, userID string, data *models.Coupon) (*models.Coupon, error) {
	now := time.Now().UTC()

	var expiresAt *time.Time
	if data.ExpiresAt != nil {
		t, parseErr := time.Parse(time.RFC3339, *data.ExpiresAt)
		if parseErr != nil {
			observability.LogAndCapture(r.logger, sentry.LevelWarning,
				"Failed to parse coupon expires_at timestamp",
				parseErr, map[string]string{
					"module": "coupon", "operation": "create_parse_expires_at",
					"raw_value": *data.ExpiresAt,
				})
		} else {
			expiresAt = &t
		}
	}

	query := `
		INSERT INTO coupons (
			user_id, code, title, store, affiliate_link, expires_at, is_active,
			category, discount_type, discount_value, max_discount, min_product_price,
			coupon_text, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		RETURNING id, created_at, updated_at
	`

	var id string
	var createdAt, updatedAt time.Time

	err := r.db.QueryRow(ctx, query,
		userID, data.Code, data.Title, data.Store, data.AffiliateLink, expiresAt, data.IsActive,
		data.Category, data.DiscountType, data.DiscountValue, data.MaxDiscount, data.MinProductPrice,
		data.CouponText, now, now,
	).Scan(&id, &createdAt, &updatedAt)

	if err != nil {
		r.logger.Error("Failed to create coupon",
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

func (r *couponRepository) UpdateCoupon(ctx context.Context, couponID, userID string, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return nil
	}

	// Build dynamic UPDATE query
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

	// Add couponID and userID as the last arguments
	args = append(args, couponID, userID)

	query := fmt.Sprintf("UPDATE coupons SET %s WHERE id = $%d AND user_id = $%d", setClauses, argIndex, argIndex+1)

	_, err := r.db.pool.Exec(ctx, query, args...)
	if err != nil {
		r.logger.Error("Failed to update coupon",
			zap.String("couponID", couponID),
			zap.Error(err))
		return err
	}

	return nil
}

func (r *couponRepository) DeleteCoupon(ctx context.Context, couponID, userID string) error {
	query := `DELETE FROM coupons WHERE id = $1 AND user_id = $2`

	_, err := r.db.pool.Exec(ctx, query, couponID, userID)
	if err != nil {
		r.logger.Error("Failed to delete coupon",
			zap.String("couponID", couponID),
			zap.Error(err))
		return err
	}

	return nil
}

func (r *couponRepository) GetActiveCouponsByStore(ctx context.Context, userID, store string) ([]models.Coupon, error) {
	query := `
		SELECT id, user_id, code, title, store, affiliate_link, expires_at, is_active,
		       category, discount_type, discount_value, max_discount, min_product_price,
		       coupon_text, created_at, updated_at
		FROM coupons
		WHERE user_id = $1 AND store = $2 AND is_active = true
		  AND (expires_at IS NULL OR expires_at > NOW())
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(ctx, query, userID, store)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanCoupons(rows)
}

// Helper functions

func (r *couponRepository) scanCoupon(row pgx.Row) (*models.Coupon, error) {
	var c models.Coupon
	var expiresAt *time.Time
	var createdAt, updatedAt time.Time

	err := row.Scan(
		&c.ID, &c.UserID, &c.Code, &c.Title, &c.Store, &c.AffiliateLink, &expiresAt, &c.IsActive,
		&c.Category, &c.DiscountType, &c.DiscountValue, &c.MaxDiscount, &c.MinProductPrice,
		&c.CouponText, &createdAt, &updatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	if expiresAt != nil {
		t := expiresAt.Format(time.RFC3339)
		c.ExpiresAt = &t
	}
	c.CreatedAt = createdAt.Format(time.RFC3339)
	c.UpdatedAt = updatedAt.Format(time.RFC3339)

	return &c, nil
}

func (r *couponRepository) scanCoupons(rows pgx.Rows) ([]models.Coupon, error) {
	var coupons []models.Coupon
	for rows.Next() {
		var c models.Coupon
		var expiresAt *time.Time
		var createdAt, updatedAt time.Time

		err := rows.Scan(
			&c.ID, &c.UserID, &c.Code, &c.Title, &c.Store, &c.AffiliateLink, &expiresAt, &c.IsActive,
			&c.Category, &c.DiscountType, &c.DiscountValue, &c.MaxDiscount, &c.MinProductPrice,
			&c.CouponText, &createdAt, &updatedAt,
		)
		if err != nil {
			return nil, err
		}

		if expiresAt != nil {
			t := expiresAt.Format(time.RFC3339)
			c.ExpiresAt = &t
		}
		c.CreatedAt = createdAt.Format(time.RFC3339)
		c.UpdatedAt = updatedAt.Format(time.RFC3339)

		coupons = append(coupons, c)
	}

	return coupons, rows.Err()
}

// GetBestCouponForProduct finds the best applicable coupon for a product
func (r *couponRepository) GetBestCouponForProduct(ctx context.Context, userID, store string, productPrice float64, category *string) (*models.Coupon, error) {
	query := `
		SELECT id, user_id, code, title, store, affiliate_link, expires_at, is_active,
		       category, discount_type, discount_value, max_discount, min_product_price,
		       coupon_text, created_at, updated_at
		FROM coupons
		WHERE user_id = $1 AND store = $2 AND is_active = true
		  AND (expires_at IS NULL OR expires_at > NOW())
		  AND (min_product_price IS NULL OR min_product_price <= $3)
		  AND (category IS NULL OR category = $4)
		ORDER BY discount_value DESC NULLS LAST
		LIMIT 1
	`

	return r.scanCoupon(r.db.QueryRow(ctx, query, userID, store, productPrice, category))
}
