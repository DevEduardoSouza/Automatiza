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
	min_price, priority, is_active, official_only,
	code, message, discount_type, discount_value, max_discount,
	created_at, updated_at
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
		INSERT INTO coupon_rules (user_id, platform, period_start, period_end,
		                          min_price, priority, is_active, official_only,
		                          code, message, discount_type, discount_value, max_discount,
		                          created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		RETURNING id
	`
	var id string
	if err := r.db.QueryRow(ctx, query,
		userID, data.Platform,
		nullableTimestamp(data.PeriodStart),
		nullableTimestamp(data.PeriodEnd),
		data.MinPrice, data.Priority, data.IsActive, data.OfficialOnly,
		nullableString(data.Code),
		nullableString(data.Message),
		nullableString(data.DiscountType),
		nullableFloat(data.DiscountValue),
		nullableFloat(data.MaxDiscount),
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
// As of 048 the rule itself owns code/message/discount fields — there's
// no JOIN to coupons anymore. We synthesize a *models.Coupon from the
// rule fields so the downstream coupon_handler / formatters keep working
// without signature churn. The synthesized Coupon's ID is set to the
// rule ID for ops logging — that's the entity that actually fired.
//
// period_start / period_end are NULL = "always valid"; non-null bounds
// must contain NOW. official_only=TRUE rules only match Shopee Mall
// items (productIsOfficial=true on the call); for non-Shopee platforms
// productIsOfficial is always false, so an official_only rule on
// Mercado Livre simply never matches — that's the intended behavior.
func (r *couponRuleRepository) FindMatchingCoupon(ctx context.Context, userID, platform string, productPrice float64, productIsOfficial bool) (*models.Coupon, error) {
	const query = `
		SELECT cr.id, cr.user_id, cr.platform,
		       cr.code, cr.message, cr.discount_type, cr.discount_value, cr.max_discount,
		       cr.created_at, cr.updated_at
		FROM coupon_rules cr
		WHERE cr.user_id = $1
		  AND cr.is_active = TRUE
		  AND cr.platform = $2
		  AND (cr.period_start IS NULL OR cr.period_start <= NOW())
		  AND (cr.period_end IS NULL OR cr.period_end >= NOW())
		  AND cr.min_price <= $3
		  AND (NOT cr.official_only OR $4)
		ORDER BY cr.min_price DESC, cr.priority DESC, cr.created_at DESC
		LIMIT 1
	`

	row := r.db.QueryRow(ctx, query, userID, platform, productPrice, productIsOfficial)

	var (
		id, userIDOut, platformOut       string
		code, message, discountType      *string
		discountValue, maxDiscount       *float64
		createdAt, updatedAt             time.Time
	)

	if err := row.Scan(
		&id, &userIDOut, &platformOut,
		&code, &message, &discountType, &discountValue, &maxDiscount,
		&createdAt, &updatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	c := &models.Coupon{
		ID:            id,
		UserID:        userIDOut,
		Store:         platformOut,
		IsActive:      true,
		DiscountType:  discountType,
		DiscountValue: discountValue,
		MaxDiscount:   maxDiscount,
		// Message goes into CouponText so the existing formatter (which
		// looks up CouponText) renders the rule's body verbatim.
		CouponText: message,
		CreatedAt:  createdAt.Format(time.RFC3339),
		UpdatedAt:  updatedAt.Format(time.RFC3339),
	}
	if code != nil {
		c.Code = *code
	}
	return c, nil
}

func (r *couponRuleRepository) scanRule(row pgx.Row) (*models.CouponRule, error) {
	var rule models.CouponRule
	var periodStart, periodEnd *time.Time
	var createdAt, updatedAt time.Time

	err := row.Scan(
		&rule.ID, &rule.UserID, &rule.CouponID, &rule.Platform,
		&periodStart, &periodEnd,
		&rule.MinPrice, &rule.Priority, &rule.IsActive, &rule.OfficialOnly,
		&rule.Code, &rule.Message, &rule.DiscountType, &rule.DiscountValue, &rule.MaxDiscount,
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
			&rule.MinPrice, &rule.Priority, &rule.IsActive, &rule.OfficialOnly,
			&rule.Code, &rule.Message, &rule.DiscountType, &rule.DiscountValue, &rule.MaxDiscount,
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

func nullableString(s *string) interface{} {
	if s == nil || *s == "" {
		return nil
	}
	return *s
}

func nullableFloat(f *float64) interface{} {
	if f == nil {
		return nil
	}
	return *f
}
