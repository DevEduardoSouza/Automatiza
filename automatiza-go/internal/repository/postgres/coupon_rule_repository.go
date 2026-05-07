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

type couponRuleRepository struct {
	db     *DB
	logger *zap.Logger
}

// NewCouponRuleRepository creates a new PostgreSQL coupon_rule repository.
func NewCouponRuleRepository(db *DB, logger *zap.Logger) repository.CouponRuleRepository {
	return &couponRuleRepository{db: db, logger: logger}
}

const couponRuleSelectColumns = `
	id, user_id, coupon_id, platform, period_start, period_end,
	min_price, priority, is_active, created_at, updated_at
`

func (r *couponRuleRepository) GetUserCouponRules(ctx context.Context, userID string) ([]models.CouponRule, error) {
	query := fmt.Sprintf(`
		SELECT %s
		FROM coupon_rules
		WHERE user_id = $1
		ORDER BY is_active DESC, priority DESC, min_price DESC, created_at DESC
	`, couponRuleSelectColumns)

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return r.scanRules(rows)
}

func (r *couponRuleRepository) GetCouponRuleByID(ctx context.Context, ruleID, userID string) (*models.CouponRule, error) {
	query := fmt.Sprintf(`
		SELECT %s
		FROM coupon_rules
		WHERE id = $1 AND user_id = $2
	`, couponRuleSelectColumns)
	return r.scanRule(r.db.QueryRow(ctx, query, ruleID, userID))
}

func (r *couponRuleRepository) CreateCouponRule(ctx context.Context, userID string, data *models.CouponRule) (*models.CouponRule, error) {
	now := time.Now().UTC()
	query := `
		INSERT INTO coupon_rules (user_id, coupon_id, platform, period_start, period_end,
		                          min_price, priority, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id
	`
	var id string
	if err := r.db.QueryRow(ctx, query,
		userID, data.CouponID, data.Platform,
		nullableTimestamp(data.PeriodStart),
		nullableTimestamp(data.PeriodEnd),
		data.MinPrice, data.Priority, data.IsActive,
		now, now,
	).Scan(&id); err != nil {
		r.logger.Error("Failed to create coupon rule",
			zap.String("userID", userID),
			zap.Error(err))
		return nil, err
	}
	return r.GetCouponRuleByID(ctx, id, userID)
}

func (r *couponRuleRepository) UpdateCouponRule(ctx context.Context, ruleID, userID string, updates map[string]interface{}) error {
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

	setClauses += fmt.Sprintf(", updated_at = $%d", argIndex)
	args = append(args, time.Now().UTC())
	argIndex++

	args = append(args, ruleID, userID)

	query := fmt.Sprintf(
		"UPDATE coupon_rules SET %s WHERE id = $%d AND user_id = $%d",
		setClauses, argIndex, argIndex+1,
	)

	tag, err := r.db.pool.Exec(ctx, query, args...)
	if err != nil {
		r.logger.Error("Failed to update coupon rule",
			zap.String("ruleID", ruleID),
			zap.Error(err))
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("coupon rule not found")
	}
	return nil
}

func (r *couponRuleRepository) DeleteCouponRule(ctx context.Context, ruleID, userID string) error {
	tag, err := r.db.pool.Exec(ctx,
		"DELETE FROM coupon_rules WHERE id = $1 AND user_id = $2",
		ruleID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("coupon rule not found")
	}
	return nil
}

// FindMatchingCoupon picks the rule with the highest min_price that the
// product still clears, breaking ties by priority then created_at. The
// SQL does the entire match in one round-trip; the LIMIT 1 plus the
// partial index on (user_id, platform) WHERE is_active keeps it fast
// even for users with many rules.
//
// period_start / period_end are NULL = "always valid"; non-null bounds
// must contain NOW. min_price is the only product-side filter for now;
// store / category / official-only get added in the follow-up PR.
func (r *couponRuleRepository) FindMatchingCoupon(ctx context.Context, userID, platform string, productPrice float64) (*models.Coupon, error) {
	const query = `
		SELECT c.id, c.user_id, c.code, c.title, c.store, c.affiliate_link,
		       c.expires_at, c.is_active, c.category, c.discount_type,
		       c.discount_value, c.max_discount, c.min_product_price,
		       c.coupon_text, c.created_at, c.updated_at
		FROM coupon_rules cr
		JOIN coupons c ON c.id = cr.coupon_id
		WHERE cr.user_id = $1
		  AND cr.is_active = TRUE
		  AND cr.platform = $2
		  AND (cr.period_start IS NULL OR cr.period_start <= NOW())
		  AND (cr.period_end IS NULL OR cr.period_end >= NOW())
		  AND cr.min_price <= $3
		ORDER BY cr.min_price DESC, cr.priority DESC, cr.created_at DESC
		LIMIT 1
	`

	row := r.db.QueryRow(ctx, query, userID, platform, productPrice)

	var c models.Coupon
	var affiliateLink, category, discountType, couponText *string
	var discountValue, maxDiscount, minProductPrice *float64
	var expiresAt *time.Time
	var createdAt, updatedAt time.Time
	var isActive *bool

	err := row.Scan(
		&c.ID, &c.UserID, &c.Code, &c.Title, &c.Store, &affiliateLink,
		&expiresAt, &isActive, &category, &discountType,
		&discountValue, &maxDiscount, &minProductPrice,
		&couponText, &createdAt, &updatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	c.AffiliateLink = affiliateLink
	c.Category = category
	c.DiscountType = discountType
	c.DiscountValue = discountValue
	c.MaxDiscount = maxDiscount
	c.MinProductPrice = minProductPrice
	c.CouponText = couponText
	if isActive != nil {
		c.IsActive = *isActive
	}
	if expiresAt != nil {
		t := expiresAt.Format(time.RFC3339)
		c.ExpiresAt = &t
	}
	c.CreatedAt = createdAt.Format(time.RFC3339)
	c.UpdatedAt = updatedAt.Format(time.RFC3339)
	return &c, nil
}

func (r *couponRuleRepository) scanRule(row pgx.Row) (*models.CouponRule, error) {
	var rule models.CouponRule
	var periodStart, periodEnd *time.Time
	var createdAt, updatedAt time.Time

	err := row.Scan(
		&rule.ID, &rule.UserID, &rule.CouponID, &rule.Platform,
		&periodStart, &periodEnd,
		&rule.MinPrice, &rule.Priority, &rule.IsActive,
		&createdAt, &updatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if periodStart != nil {
		t := periodStart.Format(time.RFC3339)
		rule.PeriodStart = &t
	}
	if periodEnd != nil {
		t := periodEnd.Format(time.RFC3339)
		rule.PeriodEnd = &t
	}
	rule.CreatedAt = createdAt.Format(time.RFC3339)
	rule.UpdatedAt = updatedAt.Format(time.RFC3339)
	return &rule, nil
}

func (r *couponRuleRepository) scanRules(rows pgx.Rows) ([]models.CouponRule, error) {
	out := make([]models.CouponRule, 0)
	for rows.Next() {
		var rule models.CouponRule
		var periodStart, periodEnd *time.Time
		var createdAt, updatedAt time.Time

		if err := rows.Scan(
			&rule.ID, &rule.UserID, &rule.CouponID, &rule.Platform,
			&periodStart, &periodEnd,
			&rule.MinPrice, &rule.Priority, &rule.IsActive,
			&createdAt, &updatedAt,
		); err != nil {
			return nil, err
		}
		if periodStart != nil {
			t := periodStart.Format(time.RFC3339)
			rule.PeriodStart = &t
		}
		if periodEnd != nil {
			t := periodEnd.Format(time.RFC3339)
			rule.PeriodEnd = &t
		}
		rule.CreatedAt = createdAt.Format(time.RFC3339)
		rule.UpdatedAt = updatedAt.Format(time.RFC3339)
		out = append(out, rule)
	}
	return out, rows.Err()
}

// nullableTimestamp parses an RFC3339 string pointer into a value pgx can
// pass to a TIMESTAMPTZ column. Empty / nil / unparseable becomes NULL —
// the rule then behaves as "no lower / upper bound" on the period.
func nullableTimestamp(s *string) interface{} {
	if s == nil || *s == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, *s)
	if err != nil {
		return nil
	}
	return t
}
