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
	created_at, updated_at, product_category_ids
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
		                          created_at, updated_at, product_category_ids)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
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
		nullableInt64Slice(data.ProductCategoryIDs),
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

// FindMatchingCoupon picks the winning rule for a product.
//
// Elegibilidade por categoria (075): uma regra com product_category_ids
// não vazio só entra quando INTERSECTA as categorias oficiais do produto
// (productCategoryIDs, vindo de ProductOfferV2.productCatIds). Regra com
// coluna NULL ou '{}' é genérica e continua elegível para tudo — inclusive
// para produto sem categoria, para entrada antiga de cache e para Mercado
// Livre, que passa nil.
//
// Tie-break (ordem TOTAL):
//  1. regra específica de categoria vence qualquer genérica;
//  2. quando officialFirst é true (gated por usuário), official_only sorta
//     à frente — uma regra de Shopee Mall bate uma faixa de preço genérica
//     num produto Mall, corrigindo o incidente em que a regra "Loja
//     Oficial" com min_price=0 perdia para qualquer faixa genérica;
//  3. min_price DESC; 4. priority DESC; 5. created_at DESC.
//
// Com TODAS as regras legadas (categoria NULL) a primeira chave é a
// constante FALSE para toda linha, e com officialFirst=false a segunda
// também — logo o conjunto elegível e a ordenação continuam idênticos ao
// histórico. O SQL faz o match inteiro num round-trip; o LIMIT 1 mais o
// índice parcial em (user_id, platform) WHERE is_active mantém rápido
// mesmo para usuários com muitas regras (o filtro por categoria roda sobre
// esse conjunto já reduzido — sem GIN nesta entrega).
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
func (r *couponRuleRepository) FindMatchingCoupon(
	ctx context.Context,
	userID, platform string,
	productPrice float64,
	productIsOfficial, officialFirst bool,
	productCategoryIDs []int64,
) (*models.Coupon, error) {
	// The CASE keeps the official tie-break rollout-gated: officialFirst=false
	// makes that sort key a constant FALSE for every row, so the ORDER BY
	// collapses to the historical `min_price DESC, priority DESC, created_at
	// DESC`. Only gated users get official_only rules sorted ahead.
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
		  AND (
		    cr.product_category_ids IS NULL
		    OR cardinality(cr.product_category_ids) = 0
		    OR cr.product_category_ids && $6::bigint[]
		  )
		ORDER BY
		  (
		    cr.product_category_ids IS NOT NULL
		    AND cardinality(cr.product_category_ids) > 0
		  ) DESC,
		  (CASE WHEN $5 THEN cr.official_only ELSE FALSE END) DESC,
		  cr.min_price DESC,
		  cr.priority DESC,
		  cr.created_at DESC
		LIMIT 1
	`

	// Slice nil/vazio vira '{}' no Postgres: `&&` com array vazio é FALSE,
	// então produto sem categoria (ou cache antigo, ou Mercado Livre) só
	// alcança as regras genéricas — nunca uma regra específica.
	cats := productCategoryIDs
	if cats == nil {
		cats = []int64{}
	}
	row := r.db.QueryRow(ctx, query, userID, platform, productPrice, productIsOfficial, officialFirst, cats)

	var (
		id, userIDOut, platformOut  string
		code, message, discountType *string
		discountValue, maxDiscount  *float64
		createdAt, updatedAt        time.Time
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
		&createdAt, &updatedAt, &rule.ProductCategoryIDs,
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
			&createdAt, &updatedAt, &rule.ProductCategoryIDs,
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

// nullableInt64Slice normaliza lista vazia para NULL na escrita. NULL e
// '{}' têm a MESMA semântica no matcher ("regra genérica"); gravar sempre
// NULL mantém uma representação só no banco e evita que um '{}' futuro
// pareça uma segmentação vazia intencional.
func nullableInt64Slice(v []int64) interface{} {
	if len(v) == 0 {
		return nil
	}
	return v
}
