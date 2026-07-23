package postgres

import (
	"context"
	"testing"
	"time"

	"automatiza-go/internal/models"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/zap"
)

// couponRuleSchema is a minimal coupon_rules table (no FKs to
// profiles/coupons — the matcher query never joins them) mirroring the
// columns from migrations 046/047/048 that FindMatchingCoupon touches.
const couponRuleSchema = `
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS coupon_rules (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL,
    coupon_id UUID,
    platform VARCHAR(50) NOT NULL,
    period_start TIMESTAMPTZ,
    period_end TIMESTAMPTZ,
    min_price NUMERIC(10,2) NOT NULL DEFAULT 0,
    priority INTEGER NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    official_only BOOLEAN NOT NULL DEFAULT FALSE,
    code VARCHAR(100),
    message TEXT,
    discount_type VARCHAR(50),
    discount_value DECIMAL(10,2),
    max_discount DECIMAL(10,2),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- 075: segmentação por categoria oficial Shopee. NULL/'{}' = genérica.
    product_category_ids BIGINT[],
    CONSTRAINT coupon_rules_product_categories_shopee_only CHECK (
        COALESCE(cardinality(product_category_ids), 0) = 0
        OR platform = 'shopee'
    )
);
`

func setupCouponRuleTestDB(t *testing.T) (*DB, func()) {
	t.Helper()
	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx,
		"postgres:15-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	require.NoError(t, err, "failed to start postgres container")

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err, "failed to get connection string")

	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err, "failed to create pool")
	require.NoError(t, pool.Ping(ctx), "failed to ping database")

	_, err = pool.Exec(ctx, couponRuleSchema)
	require.NoError(t, err, "failed to create schema")

	db := &DB{pool: pool, logger: zap.NewNop()}
	cleanup := func() {
		pool.Close()
		require.NoError(t, pgContainer.Terminate(ctx), "failed to terminate container")
	}
	return db, cleanup
}

// TestCouponRuleRepository_FindMatchingCoupon_OfficialTieBreak reproduces
// the production incident: a "Loja Oficial" rule (official_only=true) with
// min_price=0 must win over a generic price-tier rule on an official-store
// product, even though the generic rule has a higher min_price. Before the
// fix (ORDER BY official_only DESC first) min_price DESC made the generic
// rule win and the official-store coupon never fired.
func TestCouponRuleRepository_FindMatchingCoupon_OfficialTieBreak(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	db, cleanup := setupCouponRuleTestDB(t)
	defer cleanup()

	ctx := context.Background()
	repo := NewCouponRuleRepository(db, zap.NewNop())
	userID := "11111111-1111-1111-1111-111111111111"

	// Rule A: official-only, any price. Rule B: generic, min R$100.
	insert := func(code string, minPrice float64, officialOnly bool) {
		_, err := db.pool.Exec(ctx, `
			INSERT INTO coupon_rules (user_id, platform, min_price, official_only, is_active, code, discount_type, discount_value)
			VALUES ($1, 'shopee', $2, $3, TRUE, $4, 'fixed', 20)
		`, userID, minPrice, officialOnly, code)
		require.NoError(t, err)
	}
	insert("OFICIAL20", 0, true)
	insert("GENERIC100", 100, false)

	t.Run("gated on: official product picks the official-only rule", func(t *testing.T) {
		coupon, err := repo.FindMatchingCoupon(ctx, userID, "shopee", 150, true, true, nil)
		require.NoError(t, err)
		require.NotNil(t, coupon)
		assert.Equal(t, "OFICIAL20", coupon.Code,
			"with officialFirst=true the official-only rule must win the tie on a Mall product")
	})

	t.Run("gated off: official product keeps the old min_price winner", func(t *testing.T) {
		coupon, err := repo.FindMatchingCoupon(ctx, userID, "shopee", 150, true, false, nil)
		require.NoError(t, err)
		require.NotNil(t, coupon)
		assert.Equal(t, "GENERIC100", coupon.Code,
			"with officialFirst=false the historical min_price-first ordering must win")
	})

	t.Run("non-official product picks the generic rule regardless of gate", func(t *testing.T) {
		coupon, err := repo.FindMatchingCoupon(ctx, userID, "shopee", 150, false, true, nil)
		require.NoError(t, err)
		require.NotNil(t, coupon)
		assert.Equal(t, "GENERIC100", coupon.Code,
			"official-only rule must be filtered out for a non-Mall product")
	})
}

// IDs de categoria de fixture — valores arbitrários dentro do range Int32
// da Shopee, não os IDs reais do catálogo.
const (
	catA int64 = 100001
	catB int64 = 100002
	catC int64 = 100003
)

// ruleFixture descreve uma regra do cenário. Os defaults (zero value)
// correspondem a: regra genérica, ativa, sem período, min_price 0,
// priority 0, official_only false, plataforma shopee.
type ruleFixture struct {
	code         string
	platform     string // "" → "shopee"
	minPrice     float64
	priority     int
	officialOnly bool
	inactive     bool
	categories   []int64 // nil = genérica; usar emptyCats para gravar '{}'
	// createdAgo controla a ordem de created_at (maior = mais antiga).
	createdAgo time.Duration
	// periodStart/periodEnd em offsets relativos a NOW; zero = NULL.
	periodStart time.Duration
	periodEnd   time.Duration
	userID      string // "" → usuário principal do cenário
}

// emptyCats grava literalmente '{}' na coluna (regra legada/importada que
// nasceu com array vazio em vez de NULL). Deve se comportar como genérica.
var emptyCats = []int64{}

// TestCouponRuleRepository_FindMatchingCoupon_CategorySpecificity prova a
// ordem TOTAL do matcher com a coluna product_category_ids (migração 075):
// especificidade de categoria vence official_only, que vence min_price,
// priority e created_at — e que regra legada (categoria NULL) mantém
// exatamente o conjunto elegível e a ordenação de antes.
func TestCouponRuleRepository_FindMatchingCoupon_CategorySpecificity(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	db, cleanup := setupCouponRuleTestDB(t)
	defer cleanup()

	ctx := context.Background()
	repo := NewCouponRuleRepository(db, zap.NewNop())

	const (
		userID  = "22222222-2222-2222-2222-222222222222"
		otherID = "33333333-3333-3333-3333-333333333333"
		price   = 1000.0
	)

	seed := func(t *testing.T, rules ...ruleFixture) {
		t.Helper()
		_, err := db.pool.Exec(ctx, "TRUNCATE coupon_rules")
		require.NoError(t, err)
		now := time.Now().UTC()
		for _, r := range rules {
			platform := r.platform
			if platform == "" {
				platform = "shopee"
			}
			owner := r.userID
			if owner == "" {
				owner = userID
			}
			var start, end interface{}
			if r.periodStart != 0 {
				start = now.Add(r.periodStart)
			}
			if r.periodEnd != 0 {
				end = now.Add(r.periodEnd)
			}
			var cats interface{}
			if r.categories != nil {
				cats = r.categories
			}
			_, err := db.pool.Exec(ctx, `
				INSERT INTO coupon_rules (user_id, platform, min_price, priority, official_only,
				                          is_active, period_start, period_end, code,
				                          discount_type, discount_value,
				                          product_category_ids, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'fixed', 20, $10, $11, $11)
			`, owner, platform, r.minPrice, r.priority, r.officialOnly, !r.inactive,
				start, end, r.code, cats, now.Add(-r.createdAgo))
			require.NoError(t, err)
		}
	}

	// match roda o matcher com os defaults do cenário: produto oficial,
	// officialFirst desligado (o mais próximo do comportamento histórico).
	match := func(t *testing.T, cats []int64) *models.Coupon {
		t.Helper()
		c, err := repo.FindMatchingCoupon(ctx, userID, "shopee", price, true, false, cats)
		require.NoError(t, err)
		return c
	}

	// --- 1. Desempate legado intacto (todas as regras genéricas) ---------

	t.Run("1a legado: maior min_price vence", func(t *testing.T) {
		seed(t, ruleFixture{code: "MIN100", minPrice: 100}, ruleFixture{code: "MIN500", minPrice: 500})
		c := match(t, []int64{catA})
		require.NotNil(t, c)
		assert.Equal(t, "MIN500", c.Code)
	})

	t.Run("1b legado: maior priority vence com mesmo min_price", func(t *testing.T) {
		seed(t, ruleFixture{code: "PRIO1", minPrice: 100, priority: 1},
			ruleFixture{code: "PRIO9", minPrice: 100, priority: 9})
		c := match(t, []int64{catA})
		require.NotNil(t, c)
		assert.Equal(t, "PRIO9", c.Code)
	})

	t.Run("1c legado: created_at mais recente vence o resto empatado", func(t *testing.T) {
		seed(t, ruleFixture{code: "OLD", createdAgo: 48 * time.Hour},
			ruleFixture{code: "NEW", createdAgo: time.Hour})
		c := match(t, []int64{catA})
		require.NotNil(t, c)
		assert.Equal(t, "NEW", c.Code)
	})

	// --- 2..4. Elegibilidade e precedência por categoria -----------------

	t.Run("2 específica da categoria do produto vence a genérica", func(t *testing.T) {
		seed(t, ruleFixture{code: "CAT_A", categories: []int64{catA}}, ruleFixture{code: "GENERICA"})
		c := match(t, []int64{catA})
		require.NotNil(t, c)
		assert.Equal(t, "CAT_A", c.Code)
	})

	t.Run("3 específica de OUTRA categoria é inelegível", func(t *testing.T) {
		seed(t, ruleFixture{code: "CAT_B", categories: []int64{catB}}, ruleFixture{code: "GENERICA"})
		c := match(t, []int64{catA})
		require.NotNil(t, c)
		assert.Equal(t, "GENERICA", c.Code)
	})

	t.Run("4 casa por INTERSEÇÃO (produto multi-categoria)", func(t *testing.T) {
		seed(t, ruleFixture{code: "CAT_AB", categories: []int64{catA, catB}}, ruleFixture{code: "GENERICA"})
		c := match(t, []int64{catB, catC})
		require.NotNil(t, c)
		assert.Equal(t, "CAT_AB", c.Code)
	})

	// --- 5. Produto sem categoria (inclui cache antigo) ------------------

	t.Run("5a produto com categorias nil só alcança a genérica", func(t *testing.T) {
		seed(t, ruleFixture{code: "CAT_A", categories: []int64{catA}}, ruleFixture{code: "GENERICA"})
		c := match(t, nil)
		require.NotNil(t, c)
		assert.Equal(t, "GENERICA", c.Code)
	})

	t.Run("5b produto com categorias vazias só alcança a genérica", func(t *testing.T) {
		seed(t, ruleFixture{code: "CAT_A", categories: []int64{catA}}, ruleFixture{code: "GENERICA"})
		c := match(t, []int64{})
		require.NotNil(t, c)
		assert.Equal(t, "GENERICA", c.Code)
	})

	// --- 6..9. Precedência contra min_price/priority/official -----------

	t.Run("6 específica vence genérica com min_price e priority maiores", func(t *testing.T) {
		seed(t, ruleFixture{code: "CAT_A", categories: []int64{catA}},
			ruleFixture{code: "GEN_FORTE", minPrice: 900, priority: 99})
		c := match(t, []int64{catA})
		require.NotNil(t, c)
		assert.Equal(t, "CAT_A", c.Code)
	})

	t.Run("7 específica NÃO oficial vence genérica oficial com officialFirst=true", func(t *testing.T) {
		seed(t, ruleFixture{code: "CAT_A", categories: []int64{catA}},
			ruleFixture{code: "GEN_OFICIAL", officialOnly: true, minPrice: 900})
		c, err := repo.FindMatchingCoupon(ctx, userID, "shopee", price, true, true, []int64{catA})
		require.NoError(t, err)
		require.NotNil(t, c)
		assert.Equal(t, "CAT_A", c.Code,
			"especificidade de categoria tem precedência ABSOLUTA sobre official_only")
	})

	t.Run("8 duas específicas com officialFirst=false: desempate legado", func(t *testing.T) {
		seed(t, ruleFixture{code: "CAT_OFICIAL", categories: []int64{catA}, officialOnly: true},
			ruleFixture{code: "CAT_NORMAL", categories: []int64{catA}, minPrice: 100})
		c := match(t, []int64{catA})
		require.NotNil(t, c)
		assert.Equal(t, "CAT_NORMAL", c.Code, "sem officialFirst, min_price DESC decide")
	})

	t.Run("9 duas específicas com officialFirst=true: oficial primeiro", func(t *testing.T) {
		seed(t, ruleFixture{code: "CAT_OFICIAL", categories: []int64{catA}, officialOnly: true},
			ruleFixture{code: "CAT_NORMAL", categories: []int64{catA}, minPrice: 100})
		c, err := repo.FindMatchingCoupon(ctx, userID, "shopee", price, true, true, []int64{catA})
		require.NoError(t, err)
		require.NotNil(t, c)
		assert.Equal(t, "CAT_OFICIAL", c.Code)
	})

	t.Run("10 específica official_only não casa produto não-oficial", func(t *testing.T) {
		seed(t, ruleFixture{code: "CAT_OFICIAL", categories: []int64{catA}, officialOnly: true},
			ruleFixture{code: "GENERICA"})
		c, err := repo.FindMatchingCoupon(ctx, userID, "shopee", price, false, true, []int64{catA})
		require.NoError(t, err)
		require.NotNil(t, c)
		assert.Equal(t, "GENERICA", c.Code)
	})

	// --- 11..15. Array vazio, tenant, filtros e ausência de match --------

	t.Run("11 array vazio persistido é genérico", func(t *testing.T) {
		seed(t, ruleFixture{code: "VAZIA", categories: emptyCats},
			ruleFixture{code: "CAT_B", categories: []int64{catB}})
		c := match(t, []int64{catA})
		require.NotNil(t, c)
		assert.Equal(t, "VAZIA", c.Code, "'{}' tem a mesma semântica de NULL")
	})

	t.Run("12 regra específica de OUTRO tenant nunca é candidata", func(t *testing.T) {
		seed(t, ruleFixture{code: "OUTRO_CAT_A", categories: []int64{catA}, userID: otherID},
			ruleFixture{code: "GENERICA"})
		c := match(t, []int64{catA})
		require.NotNil(t, c)
		assert.Equal(t, "GENERICA", c.Code)
	})

	t.Run("13 específicas inativa/fora do período/acima do preço são descartadas", func(t *testing.T) {
		seed(t,
			ruleFixture{code: "CAT_INATIVA", categories: []int64{catA}, inactive: true},
			ruleFixture{code: "CAT_FUTURA", categories: []int64{catA}, periodStart: 24 * time.Hour, periodEnd: 48 * time.Hour},
			ruleFixture{code: "CAT_CARA", categories: []int64{catA}, minPrice: price + 1},
			ruleFixture{code: "GENERICA"},
		)
		c := match(t, []int64{catA})
		require.NotNil(t, c)
		assert.Equal(t, "GENERICA", c.Code)
	})

	t.Run("14 Mercado Livre só enxerga a genérica da própria plataforma", func(t *testing.T) {
		seed(t, ruleFixture{code: "SHOPEE_CAT_A", categories: []int64{catA}},
			ruleFixture{code: "ML_GENERICA", platform: "mercado_livre"})
		c, err := repo.FindMatchingCoupon(ctx, userID, "mercado_livre", price, false, false, nil)
		require.NoError(t, err)
		require.NotNil(t, c)
		assert.Equal(t, "ML_GENERICA", c.Code)
	})

	t.Run("15 nenhuma regra elegível devolve nil sem erro", func(t *testing.T) {
		seed(t, ruleFixture{code: "CAT_A", categories: []int64{catA}})
		c, err := repo.FindMatchingCoupon(ctx, userID, "shopee", price, true, false, []int64{catB})
		require.NoError(t, err)
		assert.Nil(t, c)
	})
}
