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
// Test Database Setup
// =============================================================================

// actionHistoryTestDB holds the test database and container for action history tests.
type actionHistoryTestDB struct {
	container testcontainers.Container
	pool      *pgxpool.Pool
	db        *DB
}

// setupActionHistoryTestDB creates a PostgreSQL container and initializes the schema.
func setupActionHistoryTestDB(t *testing.T) *actionHistoryTestDB {
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
	_, err = pool.Exec(ctx, actionHistorySchema)
	require.NoError(t, err, "failed to create schema")

	logger := zap.NewNop()

	return &actionHistoryTestDB{
		container: pgContainer,
		pool:      pool,
		db:        &DB{pool: pool, logger: logger},
	}
}

// cleanup closes the database connection and terminates the container.
func (tdb *actionHistoryTestDB) cleanup(t *testing.T) {
	t.Helper()
	ctx := context.Background()

	if tdb.pool != nil {
		tdb.pool.Close()
	}
	if tdb.container != nil {
		require.NoError(t, tdb.container.Terminate(ctx), "failed to terminate container")
	}
}

// Schema for action_history table
const actionHistorySchema = `
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS action_history (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id VARCHAR(36) NOT NULL,
    action_type VARCHAR(50) NOT NULL,
    description TEXT NOT NULL,
    details JSONB,
    status VARCHAR(20) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'running', 'completed', 'failed')),
    error_message TEXT,
    target_count INTEGER DEFAULT 0,
    success_count INTEGER DEFAULT 0,
    error_count INTEGER DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_action_history_user_id ON action_history(user_id);
CREATE INDEX IF NOT EXISTS idx_action_history_status ON action_history(status);
CREATE INDEX IF NOT EXISTS idx_action_history_created_at ON action_history(created_at DESC);
`

// =============================================================================
// Create Tests
// =============================================================================

func TestActionHistoryRepository_Create(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupActionHistoryTestDB(t)
	defer tdb.cleanup(t)

	logger := zap.NewNop()
	repo := NewActionHistoryRepository(tdb.db, logger)
	ctx := context.Background()

	t.Run("creates action history successfully", func(t *testing.T) {
		details := map[string]interface{}{
			"group_id":      "group@g.us",
			"message_count": float64(10),
		}

		history, err := repo.Create(ctx, "user-123", models.ActionTypeSendMessage, "Sending messages to group", details)

		require.NoError(t, err)
		assert.NotEmpty(t, history.ID)
		assert.Equal(t, "user-123", history.UserID)
		assert.Equal(t, models.ActionTypeSendMessage, history.ActionType)
		assert.Equal(t, "Sending messages to group", history.Description)
		assert.Equal(t, models.ActionStatusPending, history.Status)
		assert.NotEmpty(t, history.CreatedAt)
		assert.Nil(t, history.ErrorMessage)
		assert.Nil(t, history.CompletedAt)

		// Verify details
		detailsMap := history.Details.(map[string]interface{})
		assert.Equal(t, "group@g.us", detailsMap["group_id"])
	})

	t.Run("creates action history without details", func(t *testing.T) {
		history, err := repo.Create(ctx, "user-456", models.ActionTypeSyncGroups, "Syncing groups", nil)

		require.NoError(t, err)
		assert.NotEmpty(t, history.ID)
		assert.Equal(t, "user-456", history.UserID)
		assert.Equal(t, models.ActionTypeSyncGroups, history.ActionType)
	})

	t.Run("creates multiple entries for same user", func(t *testing.T) {
		_, err := repo.Create(ctx, "user-multi", models.ActionTypeSendMessage, "First action", nil)
		require.NoError(t, err)

		_, err = repo.Create(ctx, "user-multi", models.ActionTypeSendMessage, "Second action", nil)
		require.NoError(t, err)

		// Both should be created
		histories, err := repo.GetUserHistory(ctx, "user-multi", 10, 0)
		require.NoError(t, err)
		assert.Len(t, histories, 2)
	})
}

// =============================================================================
// UpdateStatus Tests
// =============================================================================

func TestActionHistoryRepository_UpdateStatus(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupActionHistoryTestDB(t)
	defer tdb.cleanup(t)

	logger := zap.NewNop()
	repo := NewActionHistoryRepository(tdb.db, logger)
	ctx := context.Background()

	t.Run("updates status to running", func(t *testing.T) {
		history, err := repo.Create(ctx, "user-status", models.ActionTypeSendMessage, "Test action", nil)
		require.NoError(t, err)

		err = repo.UpdateStatus(ctx, history.ID, models.ActionStatusRunning, "")
		require.NoError(t, err)

		// Verify
		updated, err := repo.GetByID(ctx, history.ID)
		require.NoError(t, err)
		assert.Equal(t, models.ActionStatusRunning, updated.Status)
		assert.Nil(t, updated.CompletedAt) // Not completed yet
	})

	t.Run("updates status to completed", func(t *testing.T) {
		history, err := repo.Create(ctx, "user-status", models.ActionTypeSendMessage, "Test action", nil)
		require.NoError(t, err)

		err = repo.UpdateStatus(ctx, history.ID, models.ActionStatusCompleted, "")
		require.NoError(t, err)

		// Verify
		updated, err := repo.GetByID(ctx, history.ID)
		require.NoError(t, err)
		assert.Equal(t, models.ActionStatusCompleted, updated.Status)
		assert.NotNil(t, updated.CompletedAt) // Should have completed_at
	})

	t.Run("updates status to failed with error message", func(t *testing.T) {
		history, err := repo.Create(ctx, "user-status", models.ActionTypeSendMessage, "Test action", nil)
		require.NoError(t, err)

		errorMsg := "Connection timeout"
		err = repo.UpdateStatus(ctx, history.ID, models.ActionStatusFailed, errorMsg)
		require.NoError(t, err)

		// Verify
		updated, err := repo.GetByID(ctx, history.ID)
		require.NoError(t, err)
		assert.Equal(t, models.ActionStatusFailed, updated.Status)
		require.NotNil(t, updated.ErrorMessage)
		assert.Equal(t, "Connection timeout", *updated.ErrorMessage)
		assert.NotNil(t, updated.CompletedAt) // Failed also sets completed_at
	})

	t.Run("updates status without error message", func(t *testing.T) {
		history, err := repo.Create(ctx, "user-status", models.ActionTypeSendMessage, "Test action", nil)
		require.NoError(t, err)

		err = repo.UpdateStatus(ctx, history.ID, models.ActionStatusFailed, "")
		require.NoError(t, err)

		// Verify
		updated, err := repo.GetByID(ctx, history.ID)
		require.NoError(t, err)
		assert.Equal(t, models.ActionStatusFailed, updated.Status)
		assert.Nil(t, updated.ErrorMessage) // No error message
	})

	t.Run("updates non-existent record without error", func(t *testing.T) {
		// PostgreSQL UPDATE with non-matching WHERE doesn't error
		// Use valid UUID format that doesn't exist in database
		err := repo.UpdateStatus(ctx, "00000000-0000-0000-0000-000000000000", models.ActionStatusCompleted, "")
		require.NoError(t, err)
	})
}

// =============================================================================
// GetByID Tests
// =============================================================================

func TestActionHistoryRepository_GetByID(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupActionHistoryTestDB(t)
	defer tdb.cleanup(t)

	logger := zap.NewNop()
	repo := NewActionHistoryRepository(tdb.db, logger)
	ctx := context.Background()

	t.Run("returns history by ID", func(t *testing.T) {
		details := map[string]interface{}{
			"test_key": "test_value",
		}
		created, err := repo.Create(ctx, "user-get", models.ActionTypeSendMessage, "Get test", details)
		require.NoError(t, err)

		found, err := repo.GetByID(ctx, created.ID)

		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, created.ID, found.ID)
		assert.Equal(t, "user-get", found.UserID)
		assert.Equal(t, models.ActionTypeSendMessage, found.ActionType)
		assert.Equal(t, "Get test", found.Description)
		assert.Equal(t, models.ActionStatusPending, found.Status)

		// Verify details are parsed correctly
		detailsMap := found.Details.(map[string]interface{})
		assert.Equal(t, "test_value", detailsMap["test_key"])
	})

	t.Run("returns nil for non-existent ID", func(t *testing.T) {
		found, err := repo.GetByID(ctx, "00000000-0000-0000-0000-000000000000")

		require.NoError(t, err)
		assert.Nil(t, found)
	})

	t.Run("returns history with completed_at", func(t *testing.T) {
		created, err := repo.Create(ctx, "user-get", models.ActionTypeSendMessage, "Completed test", nil)
		require.NoError(t, err)

		err = repo.UpdateStatus(ctx, created.ID, models.ActionStatusCompleted, "")
		require.NoError(t, err)

		found, err := repo.GetByID(ctx, created.ID)

		require.NoError(t, err)
		require.NotNil(t, found)
		assert.NotNil(t, found.CompletedAt)
	})

	t.Run("returns history with error message", func(t *testing.T) {
		created, err := repo.Create(ctx, "user-get", models.ActionTypeSendMessage, "Failed test", nil)
		require.NoError(t, err)

		err = repo.UpdateStatus(ctx, created.ID, models.ActionStatusFailed, "Something went wrong")
		require.NoError(t, err)

		found, err := repo.GetByID(ctx, created.ID)

		require.NoError(t, err)
		require.NotNil(t, found)
		require.NotNil(t, found.ErrorMessage)
		assert.Equal(t, "Something went wrong", *found.ErrorMessage)
	})
}

// =============================================================================
// GetByUserID Tests
// =============================================================================

func TestActionHistoryRepository_GetByUserID(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupActionHistoryTestDB(t)
	defer tdb.cleanup(t)

	logger := zap.NewNop()
	repo := NewActionHistoryRepository(tdb.db, logger)
	ctx := context.Background()

	// Create test data
	userID := "user-list-test"
	for i := 0; i < 5; i++ {
		_, err := repo.Create(ctx, userID, models.ActionTypeSendMessage, "Action "+string(rune('A'+i)), nil)
		require.NoError(t, err)
		time.Sleep(10 * time.Millisecond) // Ensure different timestamps
	}

	// Create entries for another user
	_, err := repo.Create(ctx, "other-user", models.ActionTypeSendMessage, "Other user action", nil)
	require.NoError(t, err)

	t.Run("returns user history ordered by created_at DESC", func(t *testing.T) {
		histories, err := repo.GetByUserID(ctx, userID, 10, 0)

		require.NoError(t, err)
		assert.Len(t, histories, 5)

		// Verify order (most recent first)
		for i := 0; i < len(histories)-1; i++ {
			assert.True(t, histories[i].CreatedAt >= histories[i+1].CreatedAt,
				"expected histories to be ordered by created_at DESC")
		}
	})

	t.Run("respects limit", func(t *testing.T) {
		histories, err := repo.GetByUserID(ctx, userID, 2, 0)

		require.NoError(t, err)
		assert.Len(t, histories, 2)
	})

	t.Run("respects offset", func(t *testing.T) {
		// Get all to compare
		all, err := repo.GetByUserID(ctx, userID, 10, 0)
		require.NoError(t, err)

		// Get with offset
		offset, err := repo.GetByUserID(ctx, userID, 10, 2)
		require.NoError(t, err)

		assert.Len(t, offset, 3)
		assert.Equal(t, all[2].ID, offset[0].ID)
	})

	t.Run("uses default limit when zero", func(t *testing.T) {
		histories, err := repo.GetByUserID(ctx, userID, 0, 0)

		require.NoError(t, err)
		// Default limit is 50, but we only have 5
		assert.Len(t, histories, 5)
	})

	t.Run("returns empty slice for unknown user", func(t *testing.T) {
		histories, err := repo.GetByUserID(ctx, "unknown-user", 10, 0)

		require.NoError(t, err)
		assert.Len(t, histories, 0)
	})

	t.Run("does not return other users history", func(t *testing.T) {
		histories, err := repo.GetByUserID(ctx, userID, 10, 0)

		require.NoError(t, err)
		for _, h := range histories {
			assert.Equal(t, userID, h.UserID)
		}
	})
}

// =============================================================================
// GetUserHistory Tests (alias for GetByUserID)
// =============================================================================

func TestActionHistoryRepository_GetUserHistory(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupActionHistoryTestDB(t)
	defer tdb.cleanup(t)

	logger := zap.NewNop()
	repo := NewActionHistoryRepository(tdb.db, logger)
	ctx := context.Background()

	t.Run("returns same results as GetByUserID", func(t *testing.T) {
		userID := "user-history-test"
		_, err := repo.Create(ctx, userID, models.ActionTypeSendMessage, "Test action", nil)
		require.NoError(t, err)

		byUserID, err := repo.GetByUserID(ctx, userID, 10, 0)
		require.NoError(t, err)

		userHistory, err := repo.GetUserHistory(ctx, userID, 10, 0)
		require.NoError(t, err)

		assert.Equal(t, len(byUserID), len(userHistory))
		if len(byUserID) > 0 && len(userHistory) > 0 {
			assert.Equal(t, byUserID[0].ID, userHistory[0].ID)
		}
	})
}

// =============================================================================
// Update Tests
// =============================================================================

func TestActionHistoryRepository_Update(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupActionHistoryTestDB(t)
	defer tdb.cleanup(t)

	logger := zap.NewNop()
	repo := NewActionHistoryRepository(tdb.db, logger)
	ctx := context.Background()

	t.Run("updates single field", func(t *testing.T) {
		history, err := repo.Create(ctx, "user-update", models.ActionTypeSendMessage, "Original", nil)
		require.NoError(t, err)

		err = repo.Update(ctx, history.ID, map[string]interface{}{
			"description": "Updated description",
		})
		require.NoError(t, err)

		updated, err := repo.GetByID(ctx, history.ID)
		require.NoError(t, err)
		assert.Equal(t, "Updated description", updated.Description)
	})

	t.Run("updates multiple fields", func(t *testing.T) {
		history, err := repo.Create(ctx, "user-update", models.ActionTypeSendMessage, "Original", nil)
		require.NoError(t, err)

		err = repo.Update(ctx, history.ID, map[string]interface{}{
			"status":        models.ActionStatusCompleted,
			"error_message": "Some warning",
		})
		require.NoError(t, err)

		updated, err := repo.GetByID(ctx, history.ID)
		require.NoError(t, err)
		assert.Equal(t, models.ActionStatusCompleted, updated.Status)
		require.NotNil(t, updated.ErrorMessage)
		assert.Equal(t, "Some warning", *updated.ErrorMessage)
	})

	t.Run("updates details field", func(t *testing.T) {
		history, err := repo.Create(ctx, "user-update", models.ActionTypeSendMessage, "Original", nil)
		require.NoError(t, err)

		newDetails := map[string]interface{}{
			"new_key": "new_value",
		}

		err = repo.Update(ctx, history.ID, map[string]interface{}{
			"details": newDetails,
		})
		require.NoError(t, err)

		updated, err := repo.GetByID(ctx, history.ID)
		require.NoError(t, err)
		detailsMap := updated.Details.(map[string]interface{})
		assert.Equal(t, "new_value", detailsMap["new_key"])
	})

	t.Run("does nothing with empty updates", func(t *testing.T) {
		history, err := repo.Create(ctx, "user-update", models.ActionTypeSendMessage, "Original", nil)
		require.NoError(t, err)

		err = repo.Update(ctx, history.ID, map[string]interface{}{})
		require.NoError(t, err)

		// Verify nothing changed
		updated, err := repo.GetByID(ctx, history.ID)
		require.NoError(t, err)
		assert.Equal(t, "Original", updated.Description)
	})
}

// =============================================================================
// MarkCompleted Tests
// =============================================================================

func TestActionHistoryRepository_MarkCompleted(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupActionHistoryTestDB(t)
	defer tdb.cleanup(t)

	logger := zap.NewNop()
	repo := NewActionHistoryRepository(tdb.db, logger)
	ctx := context.Background()

	t.Run("marks action as completed via UpdateStatus", func(t *testing.T) {
		history, err := repo.Create(ctx, "user-mark", models.ActionTypeSendMessage, "To complete", nil)
		require.NoError(t, err)

		err = repo.UpdateStatus(ctx, history.ID, models.ActionStatusCompleted, "")
		require.NoError(t, err)

		updated, err := repo.GetByID(ctx, history.ID)
		require.NoError(t, err)
		assert.Equal(t, models.ActionStatusCompleted, updated.Status)
	})

	t.Run("marks action as failed via UpdateStatus", func(t *testing.T) {
		history, err := repo.Create(ctx, "user-mark", models.ActionTypeSendMessage, "To fail", nil)
		require.NoError(t, err)

		err = repo.UpdateStatus(ctx, history.ID, models.ActionStatusFailed, "Network error occurred")
		require.NoError(t, err)

		updated, err := repo.GetByID(ctx, history.ID)
		require.NoError(t, err)
		assert.Equal(t, models.ActionStatusFailed, updated.Status)
		require.NotNil(t, updated.ErrorMessage)
		assert.Equal(t, "Network error occurred", *updated.ErrorMessage)
	})

	t.Run("marks action as running via UpdateStatus", func(t *testing.T) {
		history, err := repo.Create(ctx, "user-mark", models.ActionTypeSendMessage, "To run", nil)
		require.NoError(t, err)

		err = repo.UpdateStatus(ctx, history.ID, models.ActionStatusRunning, "")
		require.NoError(t, err)

		updated, err := repo.GetByID(ctx, history.ID)
		require.NoError(t, err)
		assert.Equal(t, models.ActionStatusRunning, updated.Status)
	})
}

// =============================================================================
// Concurrent Access Tests
// =============================================================================

func TestActionHistoryRepository_ConcurrentUpdates(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupActionHistoryTestDB(t)
	defer tdb.cleanup(t)

	logger := zap.NewNop()
	repo := NewActionHistoryRepository(tdb.db, logger)
	ctx := context.Background()

	t.Run("handles concurrent status updates", func(t *testing.T) {
		history, err := repo.Create(ctx, "user-concurrent", models.ActionTypeSendMessage, "Concurrent test", nil)
		require.NoError(t, err)

		const numGoroutines = 10
		done := make(chan bool, numGoroutines)
		errors := make(chan error, numGoroutines)

		// Launch concurrent updates
		for i := 0; i < numGoroutines; i++ {
			go func() {
				err := repo.UpdateStatus(ctx, history.ID, models.ActionStatusRunning, "")
				if err != nil {
					errors <- err
				}
				done <- true
			}()
		}

		// Wait for all to complete
		for i := 0; i < numGoroutines; i++ {
			select {
			case <-done:
			case err := <-errors:
				t.Fatalf("unexpected error: %v", err)
			case <-time.After(10 * time.Second):
				t.Fatal("timeout waiting for goroutines")
			}
		}

		// Verify final state
		updated, err := repo.GetByID(ctx, history.ID)
		require.NoError(t, err)
		assert.Equal(t, models.ActionStatusRunning, updated.Status)
	})
}

// =============================================================================
// Edge Cases Tests
// =============================================================================

func TestActionHistoryRepository_EdgeCases(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupActionHistoryTestDB(t)
	defer tdb.cleanup(t)

	logger := zap.NewNop()
	repo := NewActionHistoryRepository(tdb.db, logger)
	ctx := context.Background()

	t.Run("handles large details JSON", func(t *testing.T) {
		largeDetails := make(map[string]interface{})
		for i := 0; i < 100; i++ {
			largeDetails["key_"+string(rune('0'+i))] = "value_" + string(rune('0'+i))
		}

		history, err := repo.Create(ctx, "user-edge", models.ActionTypeSendMessage, "Large details", largeDetails)
		require.NoError(t, err)

		found, err := repo.GetByID(ctx, history.ID)
		require.NoError(t, err)
		assert.NotNil(t, found.Details)
	})

	t.Run("handles special characters in description", func(t *testing.T) {
		desc := "Test with 'quotes', \"double quotes\", and special chars: <>&"

		history, err := repo.Create(ctx, "user-edge", models.ActionTypeSendMessage, desc, nil)
		require.NoError(t, err)

		found, err := repo.GetByID(ctx, history.ID)
		require.NoError(t, err)
		assert.Equal(t, desc, found.Description)
	})

	t.Run("handles empty string in error message", func(t *testing.T) {
		history, err := repo.Create(ctx, "user-edge", models.ActionTypeSendMessage, "Test", nil)
		require.NoError(t, err)

		// Update with empty error message via UpdateStatus
		err = repo.UpdateStatus(ctx, history.ID, models.ActionStatusFailed, "")
		require.NoError(t, err)

		found, err := repo.GetByID(ctx, history.ID)
		require.NoError(t, err)
		assert.Equal(t, models.ActionStatusFailed, found.Status)
	})

	t.Run("handles unicode in description", func(t *testing.T) {
		desc := "Test with unicode: 你好世界 Привет мир"

		history, err := repo.Create(ctx, "user-edge", models.ActionTypeSendMessage, desc, nil)
		require.NoError(t, err)

		found, err := repo.GetByID(ctx, history.ID)
		require.NoError(t, err)
		assert.Equal(t, desc, found.Description)
	})
}
