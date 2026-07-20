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

// =============================================================================
// Test Database Setup for Dispatch Log Repository
// =============================================================================

// dispatchLogTestDB holds the test database and container for dispatch log tests.
type dispatchLogTestDB struct {
	container testcontainers.Container
	pool      *pgxpool.Pool
	db        *DB
}

// setupDispatchLogTestDB creates a PostgreSQL container and initializes the schema.
func setupDispatchLogTestDB(t *testing.T) *dispatchLogTestDB {
	t.Helper()
	ctx := context.Background()

	// Create PostgreSQL container
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

	// Get connection string
	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err, "failed to get connection string")

	// Create connection pool
	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err, "failed to create pool")

	// Ping to verify connection
	require.NoError(t, pool.Ping(ctx), "failed to ping database")

	// Create the schema
	_, err = pool.Exec(ctx, dispatchLogSchema)
	require.NoError(t, err, "failed to create schema")

	logger := zap.NewNop()

	return &dispatchLogTestDB{
		container: pgContainer,
		pool:      pool,
		db:        &DB{pool: pool, logger: logger},
	}
}

// cleanup closes the database connection and terminates the container.
func (tdb *dispatchLogTestDB) cleanup(t *testing.T) {
	t.Helper()
	ctx := context.Background()

	if tdb.pool != nil {
		tdb.pool.Close()
	}
	if tdb.container != nil {
		require.NoError(t, tdb.container.Terminate(ctx), "failed to terminate container")
	}
}

// Schema for dispatch_logs and automations tables
const dispatchLogSchema = `
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS automations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(50),
    status VARCHAR(50) NOT NULL DEFAULT 'active',
    mode VARCHAR(50),
    shopee_link TEXT,
    keyword VARCHAR(255),
    group_ids JSONB,
    send_groups JSONB,
    monitor_group_ids JSONB,
    monitoring_text_rules JSONB,
    categories JSONB,
    stores JSONB,
    min_price DECIMAL(10,2),
    max_price DECIMAL(10,2),
    min_discount DECIMAL(5,2),
    priority INTEGER DEFAULT 0,
    custom_message TEXT,
    include_image BOOLEAN DEFAULT false,
    dispatch_text_id UUID,
    filter_type VARCHAR(50),
    schedule_time TIME,
    schedule_days JSONB,
    start_time TIME,
    end_time TIME,
    interval_minutes INTEGER,
    delay_seconds INTEGER,
    dedupe_hours INTEGER,
    last_run_at TIMESTAMPTZ,
    next_run_at TIMESTAMPTZ,
    coupon_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS groups (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id VARCHAR(36) NOT NULL,
    instance_id INTEGER NOT NULL,
    wa_group_id VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    picture_url TEXT,
    members_count INTEGER NOT NULL DEFAULT 0,
    member_limit INTEGER NOT NULL DEFAULT 1024,
    status VARCHAR(50),
    is_admin BOOLEAN NOT NULL DEFAULT false,
    invite_code TEXT,
    admin_only BOOLEAN NOT NULL DEFAULT false,
    is_favorite BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS dispatch_logs (
    id SERIAL PRIMARY KEY,
    user_id VARCHAR(36) NOT NULL,
    automation_id UUID,
    automation_name VARCHAR(255),
    store VARCHAR(100),
    group_id VARCHAR(255),
    product_url TEXT,
    product_page_url TEXT,
    canonical_key VARCHAR(255),
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    type VARCHAR(50),
    error TEXT,
    product_price DECIMAL(10,2),
    product_category VARCHAR(255),
    product_discount DECIMAL(5,2),
    affiliate_link TEXT,
    coupon_id UUID,
    coupon_code VARCHAR(100),
    coupon_applied BOOLEAN,
    coupon_skip_reason TEXT,
    copy_index SMALLINT,
    list_item_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_dispatch_logs_user_id ON dispatch_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_dispatch_logs_automation_id ON dispatch_logs(automation_id);
CREATE INDEX IF NOT EXISTS idx_dispatch_logs_canonical_key ON dispatch_logs(canonical_key);
CREATE INDEX IF NOT EXISTS idx_dispatch_logs_created_at ON dispatch_logs(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_dispatch_logs_status ON dispatch_logs(status);
`

// Helper function to create a test automation
func createTestAutomation(t *testing.T, pool *pgxpool.Pool, userID string, name string) string {
	t.Helper()
	ctx := context.Background()

	var id string
	err := pool.QueryRow(ctx, `
		INSERT INTO automations (user_id, name, status)
		VALUES ($1, $2, 'active')
		RETURNING id
	`, userID, name).Scan(&id)
	require.NoError(t, err)
	return id
}

// Helper function to create a string pointer
func strPtr(s string) *string {
	return &s
}

// =============================================================================
// Create Tests
// =============================================================================

func TestDispatchLogRepository_Create(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupDispatchLogTestDB(t)
	defer tdb.cleanup(t)

	logger := zap.NewNop()
	repo := NewDispatchLogRepository(tdb.db, logger)
	ctx := context.Background()

	t.Run("creates dispatch log successfully", func(t *testing.T) {
		automationID := createTestAutomation(t, tdb.pool, "user-create-1", "Test Automation")
		automationName := "Test Automation"
		store := "shopee"
		groupID := "group-1"
		productURL := "https://shopee.com.br/product/123"
		canonicalKey := "shopee:123:group-1"
		logType := "search"

		log := &models.DispatchLog{
			UserID:         "user-create-1",
			AutomationID:   &automationID,
			AutomationName: &automationName,
			Store:          &store,
			GroupID:        &groupID,
			ProductURL:     &productURL,
			CanonicalKey:   &canonicalKey,
			Status:         models.DispatchLogStatusPending,
			Type:           &logType,
		}

		created, err := repo.Create(ctx, log)

		require.NoError(t, err)
		assert.NotZero(t, created.ID)
		assert.Equal(t, "user-create-1", created.UserID)
		assert.Equal(t, automationID, *created.AutomationID)
		assert.Equal(t, "Test Automation", *created.AutomationName)
		assert.Equal(t, "shopee", *created.Store)
		assert.Equal(t, "group-1", *created.GroupID)
		assert.Equal(t, models.DispatchLogStatusPending, created.Status)
		assert.NotEmpty(t, created.CreatedAt)
	})

	t.Run("creates dispatch log with minimal fields", func(t *testing.T) {
		log := &models.DispatchLog{
			UserID: "user-create-2",
			Status: models.DispatchLogStatusQueued,
		}

		created, err := repo.Create(ctx, log)

		require.NoError(t, err)
		assert.NotZero(t, created.ID)
		assert.Equal(t, models.DispatchLogStatusQueued, created.Status)
	})

	t.Run("persists coupon fields", func(t *testing.T) {
		couponID := "22222222-2222-2222-2222-222222222222"
		code := "OFICIAL20"
		applied := true
		log := &models.DispatchLog{
			UserID:        "user-coupon",
			Status:        models.DispatchLogStatusQueued,
			CouponID:      &couponID,
			CouponCode:    &code,
			CouponApplied: &applied,
		}
		created, err := repo.Create(ctx, log)
		require.NoError(t, err)

		var gotID, gotCode *string
		var gotApplied *bool
		var gotSkip *string
		err = tdb.pool.QueryRow(ctx,
			`SELECT coupon_id, coupon_code, coupon_applied, coupon_skip_reason FROM dispatch_logs WHERE id = $1`,
			created.ID,
		).Scan(&gotID, &gotCode, &gotApplied, &gotSkip)
		require.NoError(t, err)
		require.NotNil(t, gotID)
		assert.Equal(t, couponID, *gotID)
		require.NotNil(t, gotCode)
		assert.Equal(t, "OFICIAL20", *gotCode)
		require.NotNil(t, gotApplied)
		assert.True(t, *gotApplied)
		assert.Nil(t, gotSkip, "skip reason stays NULL when coupon applied")
	})

	t.Run("persists coupon skip reason when not applied", func(t *testing.T) {
		applied := false
		reason := "no coupon rule matched"
		log := &models.DispatchLog{
			UserID:           "user-coupon-skip",
			Status:           models.DispatchLogStatusQueued,
			CouponApplied:    &applied,
			CouponSkipReason: &reason,
		}
		created, err := repo.Create(ctx, log)
		require.NoError(t, err)

		var gotApplied *bool
		var gotSkip *string
		err = tdb.pool.QueryRow(ctx,
			`SELECT coupon_applied, coupon_skip_reason FROM dispatch_logs WHERE id = $1`,
			created.ID,
		).Scan(&gotApplied, &gotSkip)
		require.NoError(t, err)
		require.NotNil(t, gotApplied)
		assert.False(t, *gotApplied)
		require.NotNil(t, gotSkip)
		assert.Equal(t, "no coupon rule matched", *gotSkip)
	})

	t.Run("creates multiple logs for same user", func(t *testing.T) {
		userID := "user-multi-create"

		for i := 0; i < 3; i++ {
			log := &models.DispatchLog{
				UserID: userID,
				Status: models.DispatchLogStatusPending,
			}
			_, err := repo.Create(ctx, log)
			require.NoError(t, err)
		}

		// Verify all created
		logs, err := repo.GetUserLogs(ctx, userID, 10, 0)
		require.NoError(t, err)
		assert.Len(t, logs, 3)
	})
}

// =============================================================================
// GetUserLogs Tests
// =============================================================================

func TestDispatchLogRepository_GetUserLogs(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupDispatchLogTestDB(t)
	defer tdb.cleanup(t)

	logger := zap.NewNop()
	repo := NewDispatchLogRepository(tdb.db, logger)
	ctx := context.Background()

	userID := "user-list-test"

	// Create test logs
	for i := 0; i < 5; i++ {
		log := &models.DispatchLog{
			UserID: userID,
			Status: models.DispatchLogStatusSent,
		}
		_, err := repo.Create(ctx, log)
		require.NoError(t, err)
		time.Sleep(10 * time.Millisecond) // Ensure different timestamps
	}

	// Create logs for another user
	otherLog := &models.DispatchLog{
		UserID: "other-user",
		Status: models.DispatchLogStatusSent,
	}
	_, err := repo.Create(ctx, otherLog)
	require.NoError(t, err)

	t.Run("returns user logs ordered by created_at DESC", func(t *testing.T) {
		logs, err := repo.GetUserLogs(ctx, userID, 10, 0)

		require.NoError(t, err)
		assert.Len(t, logs, 5)

		// Verify order (most recent first)
		for i := 0; i < len(logs)-1; i++ {
			assert.True(t, logs[i].CreatedAt >= logs[i+1].CreatedAt,
				"expected logs to be ordered by created_at DESC")
		}
	})

	t.Run("respects limit", func(t *testing.T) {
		logs, err := repo.GetUserLogs(ctx, userID, 2, 0)

		require.NoError(t, err)
		assert.Len(t, logs, 2)
	})

	t.Run("respects offset", func(t *testing.T) {
		// Get all to compare
		all, err := repo.GetUserLogs(ctx, userID, 10, 0)
		require.NoError(t, err)

		// Get with offset
		offset, err := repo.GetUserLogs(ctx, userID, 10, 2)
		require.NoError(t, err)

		assert.Len(t, offset, 3)
		assert.Equal(t, all[2].ID, offset[0].ID)
	})

	t.Run("uses default limit when zero", func(t *testing.T) {
		logs, err := repo.GetUserLogs(ctx, userID, 0, 0)

		require.NoError(t, err)
		// Default limit is 50, but we only have 5
		assert.Len(t, logs, 5)
	})

	t.Run("returns empty slice for unknown user", func(t *testing.T) {
		logs, err := repo.GetUserLogs(ctx, "unknown-user", 10, 0)

		require.NoError(t, err)
		assert.Len(t, logs, 0)
	})

	t.Run("does not return other users logs", func(t *testing.T) {
		logs, err := repo.GetUserLogs(ctx, userID, 10, 0)

		require.NoError(t, err)
		for _, log := range logs {
			assert.Equal(t, userID, log.UserID)
		}
	})
}

// =============================================================================
// GetRecentByCanonicalKey Tests (Deduplication)
// =============================================================================

func TestDispatchLogRepository_GetRecentByCanonicalKey(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupDispatchLogTestDB(t)
	defer tdb.cleanup(t)

	logger := zap.NewNop()
	repo := NewDispatchLogRepository(tdb.db, logger)
	ctx := context.Background()

	userID := "user-dedup"
	groupID := "group-dedup"
	canonicalKey := "shopee:product123:group-dedup"

	t.Run("returns nil when no matching log exists", func(t *testing.T) {
		found, err := repo.GetRecentByCanonicalKey(ctx, userID, groupID, "non-existent-key", 24*time.Hour)

		require.NoError(t, err)
		assert.Nil(t, found)
	})

	t.Run("returns matching log within time window", func(t *testing.T) {
		// Create a log
		log := &models.DispatchLog{
			UserID:       userID,
			GroupID:      &groupID,
			CanonicalKey: &canonicalKey,
			Status:       models.DispatchLogStatusSent,
		}
		_, err := repo.Create(ctx, log)
		require.NoError(t, err)

		// Should find it within 24 hours
		found, err := repo.GetRecentByCanonicalKey(ctx, userID, groupID, canonicalKey, 24*time.Hour)

		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, canonicalKey, *found.CanonicalKey)
		assert.Equal(t, models.DispatchLogStatusSent, found.Status)
	})

	t.Run("returns nil for different user", func(t *testing.T) {
		found, err := repo.GetRecentByCanonicalKey(ctx, "different-user", groupID, canonicalKey, 24*time.Hour)

		require.NoError(t, err)
		assert.Nil(t, found)
	})

	t.Run("returns nil for different group", func(t *testing.T) {
		found, err := repo.GetRecentByCanonicalKey(ctx, userID, "different-group", canonicalKey, 24*time.Hour)

		require.NoError(t, err)
		assert.Nil(t, found)
	})

	t.Run("only returns logs with sent/queued/pending status", func(t *testing.T) {
		newCanonicalKey := "shopee:failed-product:group-dedup"

		// Create a failed log
		failedLog := &models.DispatchLog{
			UserID:       userID,
			GroupID:      &groupID,
			CanonicalKey: &newCanonicalKey,
			Status:       models.DispatchLogStatusFailed,
		}
		_, err := repo.Create(ctx, failedLog)
		require.NoError(t, err)

		// Should not find it (failed status)
		found, err := repo.GetRecentByCanonicalKey(ctx, userID, groupID, newCanonicalKey, 24*time.Hour)

		require.NoError(t, err)
		assert.Nil(t, found)
	})
}

// =============================================================================
// ExistsByCanonicalKey Tests
// =============================================================================

func TestDispatchLogRepository_ExistsByCanonicalKey(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupDispatchLogTestDB(t)
	defer tdb.cleanup(t)

	logger := zap.NewNop()
	repo := NewDispatchLogRepository(tdb.db, logger)
	ctx := context.Background()

	userID := "user-exists"
	groupID := "group-exists"
	canonicalKey := "shopee:exists123:group-exists"

	t.Run("returns false for empty canonical key", func(t *testing.T) {
		exists, err := repo.ExistsByCanonicalKey(ctx, userID, groupID, "", 24)

		require.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("returns false when key does not exist", func(t *testing.T) {
		exists, err := repo.ExistsByCanonicalKey(ctx, userID, groupID, "non-existent-key", 24)

		require.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("returns true when key exists within window", func(t *testing.T) {
		// Create a log (any status)
		log := &models.DispatchLog{
			UserID:       userID,
			GroupID:      &groupID,
			CanonicalKey: &canonicalKey,
			Status:       models.DispatchLogStatusFailed, // Even failed status
		}
		_, err := repo.Create(ctx, log)
		require.NoError(t, err)

		// Check existence
		exists, err := repo.ExistsByCanonicalKey(ctx, userID, groupID, canonicalKey, 24)

		require.NoError(t, err)
		assert.True(t, exists)
	})
}

// =============================================================================
// CreateIfNotExists Tests (Idempotent Creation)
// =============================================================================

func TestDispatchLogRepository_CreateIfNotExists(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupDispatchLogTestDB(t)
	defer tdb.cleanup(t)

	logger := zap.NewNop()
	repo := NewDispatchLogRepository(tdb.db, logger)
	ctx := context.Background()

	userID := "user-idempotent"
	groupID := "group-idempotent"
	canonicalKey := "shopee:idempotent123:group"

	t.Run("creates log when no canonical key", func(t *testing.T) {
		log := &models.DispatchLog{
			UserID: userID,
			Status: models.DispatchLogStatusPending,
		}

		created, wasCreated, err := repo.CreateIfNotExists(ctx, log, 24)

		require.NoError(t, err)
		assert.True(t, wasCreated)
		assert.NotNil(t, created)
		assert.NotZero(t, created.ID)
	})

	t.Run("creates log when canonical key does not exist", func(t *testing.T) {
		log := &models.DispatchLog{
			UserID:       userID,
			GroupID:      &groupID,
			CanonicalKey: &canonicalKey,
			Status:       models.DispatchLogStatusPending,
		}

		created, wasCreated, err := repo.CreateIfNotExists(ctx, log, 24)

		require.NoError(t, err)
		assert.True(t, wasCreated)
		assert.NotNil(t, created)
		assert.Equal(t, canonicalKey, *created.CanonicalKey)
	})

	t.Run("does not create log when canonical key already exists", func(t *testing.T) {
		log := &models.DispatchLog{
			UserID:       userID,
			GroupID:      &groupID,
			CanonicalKey: &canonicalKey,
			Status:       models.DispatchLogStatusPending,
		}

		created, wasCreated, err := repo.CreateIfNotExists(ctx, log, 24)

		require.NoError(t, err)
		assert.False(t, wasCreated)
		assert.Nil(t, created)
	})

	t.Run("creates log for different group even with same key", func(t *testing.T) {
		differentGroup := "different-group"
		log := &models.DispatchLog{
			UserID:       userID,
			GroupID:      &differentGroup,
			CanonicalKey: &canonicalKey,
			Status:       models.DispatchLogStatusPending,
		}

		created, wasCreated, err := repo.CreateIfNotExists(ctx, log, 24)

		require.NoError(t, err)
		assert.True(t, wasCreated)
		assert.NotNil(t, created)
	})
}

// NOTE: TestDispatchLogRepository_GetStats and TestDispatchLogRepository_DeleteOldLogs
// were removed because GetStats and DeleteOldLogs methods no longer exist in the
// DispatchLogRepository interface.

// =============================================================================
// DeleteRecentByAutomation Tests
// =============================================================================

func TestDispatchLogRepository_DeleteRecentByAutomation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupDispatchLogTestDB(t)
	defer tdb.cleanup(t)

	logger := zap.NewNop()
	repo := NewDispatchLogRepository(tdb.db, logger)
	ctx := context.Background()

	userID := "user-delete-by-auto"
	automationID := createTestAutomation(t, tdb.pool, userID, "Delete Test Automation")

	t.Run("deletes logs for automation within days", func(t *testing.T) {
		// Create logs for the automation
		for i := 0; i < 3; i++ {
			log := &models.DispatchLog{
				UserID:       userID,
				AutomationID: &automationID,
				Status:       models.DispatchLogStatusSent,
			}
			_, err := repo.Create(ctx, log)
			require.NoError(t, err)
		}

		// Delete logs from last 7 days
		deleted, err := repo.DeleteRecentByAutomation(ctx, automationID, 7)

		require.NoError(t, err)
		assert.Equal(t, int64(3), deleted)

		// Verify deletion
		logs, err := repo.GetUserLogs(ctx, userID, 10, 0)
		require.NoError(t, err)
		assert.Len(t, logs, 0)
	})

	t.Run("does not delete logs for other automations", func(t *testing.T) {
		otherAutomationID := createTestAutomation(t, tdb.pool, userID, "Other Automation")

		// Create log for other automation
		log := &models.DispatchLog{
			UserID:       userID,
			AutomationID: &otherAutomationID,
			Status:       models.DispatchLogStatusSent,
		}
		_, err := repo.Create(ctx, log)
		require.NoError(t, err)

		// Delete logs for first automation
		_, err = repo.DeleteRecentByAutomation(ctx, automationID, 7)
		require.NoError(t, err)

		// Verify other automation's logs remain
		logs, err := repo.GetUserLogs(ctx, userID, 10, 0)
		require.NoError(t, err)
		assert.Len(t, logs, 1)
		assert.Equal(t, otherAutomationID, *logs[0].AutomationID)
	})
}

// =============================================================================
// Update Tests
// =============================================================================

func TestDispatchLogRepository_Update(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupDispatchLogTestDB(t)
	defer tdb.cleanup(t)

	logger := zap.NewNop()
	repo := NewDispatchLogRepository(tdb.db, logger)
	ctx := context.Background()

	t.Run("updates status", func(t *testing.T) {
		log := &models.DispatchLog{
			UserID: "user-update",
			Status: models.DispatchLogStatusPending,
		}
		created, err := repo.Create(ctx, log)
		require.NoError(t, err)

		err = repo.Update(ctx, created.ID, map[string]interface{}{
			"status": models.DispatchLogStatusSent,
		})
		require.NoError(t, err)

		// Verify through GetUserLogs
		logs, err := repo.GetUserLogs(ctx, "user-update", 10, 0)
		require.NoError(t, err)
		require.Len(t, logs, 1)
		assert.Equal(t, models.DispatchLogStatusSent, logs[0].Status)
	})

	t.Run("updates status with error message", func(t *testing.T) {
		log := &models.DispatchLog{
			UserID: "user-update-error",
			Status: models.DispatchLogStatusPending,
		}
		created, err := repo.Create(ctx, log)
		require.NoError(t, err)

		err = repo.Update(ctx, created.ID, map[string]interface{}{
			"status": models.DispatchLogStatusFailed,
			"error":  "Connection timeout",
		})
		require.NoError(t, err)

		// Verify through GetUserLogs
		logs, err := repo.GetUserLogs(ctx, "user-update-error", 10, 0)
		require.NoError(t, err)
		require.Len(t, logs, 1)
		assert.Equal(t, models.DispatchLogStatusFailed, logs[0].Status)
		require.NotNil(t, logs[0].Error)
		assert.Equal(t, "Connection timeout", *logs[0].Error)
	})
}

// =============================================================================
// Edge Cases Tests
// =============================================================================

func TestDispatchLogRepository_EdgeCases(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupDispatchLogTestDB(t)
	defer tdb.cleanup(t)

	logger := zap.NewNop()
	repo := NewDispatchLogRepository(tdb.db, logger)
	ctx := context.Background()

	t.Run("handles special characters in product URL", func(t *testing.T) {
		productURL := "https://shopee.com.br/product?id=123&name=test%20product&special='quotes'"
		log := &models.DispatchLog{
			UserID:     "user-edge",
			ProductURL: &productURL,
			Status:     models.DispatchLogStatusPending,
		}

		_, err := repo.Create(ctx, log)
		require.NoError(t, err)

		logs, err := repo.GetUserLogs(ctx, "user-edge", 10, 0)
		require.NoError(t, err)
		require.Len(t, logs, 1)
		assert.Equal(t, productURL, *logs[0].ProductURL)
	})

	t.Run("handles unicode in automation name", func(t *testing.T) {
		automationName := "Promocoes e Ofertas"
		log := &models.DispatchLog{
			UserID:         "user-unicode",
			AutomationName: &automationName,
			Status:         models.DispatchLogStatusPending,
		}

		created, err := repo.Create(ctx, log)
		require.NoError(t, err)
		assert.NotZero(t, created.ID)
	})

	t.Run("handles very long product URL", func(t *testing.T) {
		longURL := make([]byte, 2000)
		for i := range longURL {
			longURL[i] = 'a'
		}
		productURL := "https://example.com/" + string(longURL)

		log := &models.DispatchLog{
			UserID:     "user-long-url",
			ProductURL: &productURL,
			Status:     models.DispatchLogStatusPending,
		}

		created, err := repo.Create(ctx, log)
		require.NoError(t, err)
		assert.NotZero(t, created.ID)
	})

	t.Run("handles all optional fields as nil", func(t *testing.T) {
		log := &models.DispatchLog{
			UserID: "user-nil-fields",
			Status: models.DispatchLogStatusPending,
		}

		created, err := repo.Create(ctx, log)
		require.NoError(t, err)
		assert.NotZero(t, created.ID)
		assert.Nil(t, created.AutomationID)
		assert.Nil(t, created.Store)
		assert.Nil(t, created.GroupID)
		assert.Nil(t, created.ProductURL)
		assert.Nil(t, created.CanonicalKey)
	})
}

// =============================================================================
// Concurrent Access Tests
// =============================================================================

func TestDispatchLogRepository_ConcurrentDedup(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupDispatchLogTestDB(t)
	defer tdb.cleanup(t)

	logger := zap.NewNop()
	repo := NewDispatchLogRepository(tdb.db, logger)
	ctx := context.Background()

	userID := "user-concurrent-dedup"
	groupID := "group-concurrent"
	canonicalKey := "shopee:concurrent123:group"

	t.Run("handles concurrent CreateIfNotExists", func(t *testing.T) {
		const numGoroutines = 10
		created := make(chan bool, numGoroutines)
		errors := make(chan error, numGoroutines)

		// Launch concurrent CreateIfNotExists
		for i := 0; i < numGoroutines; i++ {
			go func() {
				log := &models.DispatchLog{
					UserID:       userID,
					GroupID:      &groupID,
					CanonicalKey: &canonicalKey,
					Status:       models.DispatchLogStatusPending,
				}
				_, wasCreated, err := repo.CreateIfNotExists(ctx, log, 24)
				if err != nil {
					errors <- err
				}
				created <- wasCreated
			}()
		}

		// Collect results
		var createdCount int
		for i := 0; i < numGoroutines; i++ {
			select {
			case wasCreated := <-created:
				if wasCreated {
					createdCount++
				}
			case err := <-errors:
				t.Fatalf("unexpected error: %v", err)
			case <-time.After(10 * time.Second):
				t.Fatal("timeout waiting for goroutines")
			}
		}

		// Only one should have been created
		assert.Equal(t, 1, createdCount, "expected exactly one log to be created")
	})
}

// =============================================================================
// DeleteOldLogsByStatus (batched) Tests
// =============================================================================

// TestDispatchLogRepository_DeleteOldLogsByStatus_Batched prova que o delete em
// lotes (ctid + LIMIT 50000) atravessa mais de um lote, respeita o corte de
// idade e não toca em outros status — a checagem executável da lógica de loop.
func TestDispatchLogRepository_DeleteOldLogsByStatus_Batched(t *testing.T) {
	tdb := setupDispatchLogTestDB(t)
	defer tdb.cleanup(t)

	repo := NewDispatchLogRepository(tdb.db, zap.NewNop())
	ctx := context.Background()

	// 120.001 linhas 'skipped' velhas (3 lotes: 50k + 50k + 20.001), mais
	// linhas que DEVEM sobrar: 'skipped' recente e 'sent' velha.
	_, err := tdb.pool.Exec(ctx, `
		INSERT INTO dispatch_logs (user_id, group_id, status, error, created_at)
		SELECT 'user-batch', 'group-1', 'skipped', 'dedupe', NOW() - INTERVAL '10 days'
		FROM generate_series(1, 120001)`)
	require.NoError(t, err)
	_, err = tdb.pool.Exec(ctx, `
		INSERT INTO dispatch_logs (user_id, group_id, status, error, created_at) VALUES
		('user-batch', 'group-1', 'skipped', 'dedupe', NOW() - INTERVAL '1 hour'),
		('user-batch', 'group-1', 'sent', NULL, NOW() - INTERVAL '10 days')`)
	require.NoError(t, err)

	deleted, err := repo.DeleteOldLogsByStatus(ctx, "skipped", 3*24*time.Hour)
	require.NoError(t, err)
	assert.Equal(t, int64(120001), deleted, "deve deletar exatamente as skipped velhas, em 3 lotes")

	var remaining int
	require.NoError(t, tdb.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM dispatch_logs WHERE user_id = 'user-batch'`).Scan(&remaining))
	assert.Equal(t, 2, remaining, "skipped recente e sent velha devem sobrar")
}
