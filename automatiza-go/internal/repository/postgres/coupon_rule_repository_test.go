package postgres

import (
	"context"
	"testing"
	"time"

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
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
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
		coupon, err := repo.FindMatchingCoupon(ctx, userID, "shopee", 150, true, true)
		require.NoError(t, err)
		require.NotNil(t, coupon)
		assert.Equal(t, "OFICIAL20", coupon.Code,
			"with officialFirst=true the official-only rule must win the tie on a Mall product")
	})

	t.Run("gated off: official product keeps the old min_price winner", func(t *testing.T) {
		coupon, err := repo.FindMatchingCoupon(ctx, userID, "shopee", 150, true, false)
		require.NoError(t, err)
		require.NotNil(t, coupon)
		assert.Equal(t, "GENERIC100", coupon.Code,
			"with officialFirst=false the historical min_price-first ordering must win")
	})

	t.Run("non-official product picks the generic rule regardless of gate", func(t *testing.T) {
		coupon, err := repo.FindMatchingCoupon(ctx, userID, "shopee", 150, false, true)
		require.NoError(t, err)
		require.NotNil(t, coupon)
		assert.Equal(t, "GENERIC100", coupon.Code,
			"official-only rule must be filtered out for a non-Mall product")
	})
}
