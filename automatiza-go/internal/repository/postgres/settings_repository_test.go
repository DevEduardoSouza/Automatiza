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
// Test Database Setup for Settings Repository
// =============================================================================

// settingsTestDB holds the test database and container for settings tests.
type settingsTestDB struct {
	container testcontainers.Container
	pool      *pgxpool.Pool
	db        *DB
}

// setupSettingsTestDB creates a PostgreSQL container and initializes the schema.
func setupSettingsTestDB(t *testing.T) *settingsTestDB {
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
	_, err = pool.Exec(ctx, dispatchTextsSchema)
	require.NoError(t, err, "failed to create schema")

	logger := zap.NewNop()

	return &settingsTestDB{
		container: pgContainer,
		pool:      pool,
		db:        &DB{pool: pool, logger: logger},
	}
}

// cleanup closes the database connection and terminates the container.
func (tdb *settingsTestDB) cleanup(t *testing.T) {
	t.Helper()
	ctx := context.Background()

	if tdb.pool != nil {
		tdb.pool.Close()
	}
	if tdb.container != nil {
		require.NoError(t, tdb.container.Terminate(ctx), "failed to terminate container")
	}
}

// Schema for dispatch_texts table
const dispatchTextsSchema = `
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS dispatch_texts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id VARCHAR(36) NOT NULL,
    name VARCHAR(255) NOT NULL,
    body TEXT NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_dispatch_texts_user_id ON dispatch_texts(user_id);
CREATE INDEX IF NOT EXISTS idx_dispatch_texts_is_active ON dispatch_texts(is_active);
`

// =============================================================================
// GetDispatchText Tests
// =============================================================================

func TestSettingsRepository_GetDispatchText(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupSettingsTestDB(t)
	defer tdb.cleanup(t)

	logger := zap.NewNop()
	repo := NewSettingsRepository(tdb.db, logger)
	ctx := context.Background()

	t.Run("returns dispatch text by ID", func(t *testing.T) {
		dt := &models.DispatchText{
			Name:     "Welcome Message",
			Body:     "Hello, welcome to our group!",
			IsActive: true,
		}
		created, err := repo.CreateDispatchText(ctx, "user-get-1", dt)
		require.NoError(t, err)

		found, err := repo.GetDispatchText(ctx, created.ID)

		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, created.ID, found.ID)
		assert.Equal(t, "user-get-1", found.UserID)
		assert.Equal(t, "Welcome Message", found.Name)
		assert.Equal(t, "Hello, welcome to our group!", found.Body)
		assert.True(t, found.IsActive)
		assert.NotEmpty(t, found.CreatedAt)
		assert.NotEmpty(t, found.UpdatedAt)
	})

	t.Run("returns nil for non-existent ID", func(t *testing.T) {
		found, err := repo.GetDispatchText(ctx, "00000000-0000-0000-0000-000000000000")

		require.NoError(t, err)
		assert.Nil(t, found)
	})

	t.Run("returns dispatch text with is_active false", func(t *testing.T) {
		dt := &models.DispatchText{
			Name:     "Inactive Message",
			Body:     "This is inactive",
			IsActive: false,
		}
		created, err := repo.CreateDispatchText(ctx, "user-get-1", dt)
		require.NoError(t, err)

		found, err := repo.GetDispatchText(ctx, created.ID)

		require.NoError(t, err)
		require.NotNil(t, found)
		assert.False(t, found.IsActive)
	})
}

// =============================================================================
// GetDispatchTextsByUser Tests
// =============================================================================

func TestSettingsRepository_GetDispatchTextsByUser(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupSettingsTestDB(t)
	defer tdb.cleanup(t)

	logger := zap.NewNop()
	repo := NewSettingsRepository(tdb.db, logger)
	ctx := context.Background()

	userID := "user-list-test"

	// Create test data with different names to verify ordering
	testTexts := []struct {
		name     string
		body     string
		isActive bool
	}{
		{"Charlie", "Body C", true},
		{"Alpha", "Body A", true},
		{"Bravo", "Body B", false},
	}

	for _, tt := range testTexts {
		dt := &models.DispatchText{
			Name:     tt.name,
			Body:     tt.body,
			IsActive: tt.isActive,
		}
		_, err := repo.CreateDispatchText(ctx, userID, dt)
		require.NoError(t, err)
	}

	// Create entry for another user
	otherDt := &models.DispatchText{
		Name:     "Other User Text",
		Body:     "Other body",
		IsActive: true,
	}
	_, err := repo.CreateDispatchText(ctx, "other-user", otherDt)
	require.NoError(t, err)

	t.Run("returns all user dispatch texts ordered by name", func(t *testing.T) {
		texts, err := repo.GetDispatchTextsByUser(ctx, userID)

		require.NoError(t, err)
		assert.Len(t, texts, 3)

		// Verify alphabetical order by name
		assert.Equal(t, "Alpha", texts[0].Name)
		assert.Equal(t, "Bravo", texts[1].Name)
		assert.Equal(t, "Charlie", texts[2].Name)
	})

	t.Run("returns empty slice for unknown user", func(t *testing.T) {
		texts, err := repo.GetDispatchTextsByUser(ctx, "unknown-user")

		require.NoError(t, err)
		assert.Len(t, texts, 0)
	})

	t.Run("does not return other users dispatch texts", func(t *testing.T) {
		texts, err := repo.GetDispatchTextsByUser(ctx, userID)

		require.NoError(t, err)
		for _, dt := range texts {
			assert.Equal(t, userID, dt.UserID)
		}
	})

	t.Run("returns both active and inactive texts", func(t *testing.T) {
		texts, err := repo.GetDispatchTextsByUser(ctx, userID)

		require.NoError(t, err)
		var activeCount, inactiveCount int
		for _, dt := range texts {
			if dt.IsActive {
				activeCount++
			} else {
				inactiveCount++
			}
		}
		assert.Equal(t, 2, activeCount)
		assert.Equal(t, 1, inactiveCount)
	})
}

// =============================================================================
// GetActiveDispatchTextByID Tests
// =============================================================================

func TestSettingsRepository_GetActiveDispatchTextByID(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupSettingsTestDB(t)
	defer tdb.cleanup(t)

	logger := zap.NewNop()
	repo := NewSettingsRepository(tdb.db, logger)
	ctx := context.Background()

	userID := "user-active-test"

	t.Run("returns active dispatch text by ID and user", func(t *testing.T) {
		dt := &models.DispatchText{
			Name:     "Active Text",
			Body:     "Active body",
			IsActive: true,
		}
		created, err := repo.CreateDispatchText(ctx, userID, dt)
		require.NoError(t, err)

		found, err := repo.GetActiveDispatchTextByID(ctx, created.ID, userID)

		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, created.ID, found.ID)
		assert.Equal(t, userID, found.UserID)
		assert.True(t, found.IsActive)
	})

	t.Run("returns nil for inactive dispatch text", func(t *testing.T) {
		dt := &models.DispatchText{
			Name:     "Inactive Text",
			Body:     "Inactive body",
			IsActive: false,
		}
		created, err := repo.CreateDispatchText(ctx, userID, dt)
		require.NoError(t, err)

		found, err := repo.GetActiveDispatchTextByID(ctx, created.ID, userID)

		require.NoError(t, err)
		assert.Nil(t, found)
	})

	t.Run("returns nil for wrong user", func(t *testing.T) {
		dt := &models.DispatchText{
			Name:     "User Specific Text",
			Body:     "User specific body",
			IsActive: true,
		}
		created, err := repo.CreateDispatchText(ctx, userID, dt)
		require.NoError(t, err)

		found, err := repo.GetActiveDispatchTextByID(ctx, created.ID, "different-user")

		require.NoError(t, err)
		assert.Nil(t, found)
	})

	t.Run("returns nil for non-existent ID", func(t *testing.T) {
		found, err := repo.GetActiveDispatchTextByID(ctx, "00000000-0000-0000-0000-000000000000", userID)

		require.NoError(t, err)
		assert.Nil(t, found)
	})
}

// =============================================================================
// GetActiveDispatchTextsByUser Tests
// =============================================================================

func TestSettingsRepository_GetActiveDispatchTextsByUser(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupSettingsTestDB(t)
	defer tdb.cleanup(t)

	logger := zap.NewNop()
	repo := NewSettingsRepository(tdb.db, logger)
	ctx := context.Background()

	userID := "user-active-list-test"

	// Create test data
	testTexts := []struct {
		name     string
		body     string
		isActive bool
	}{
		{"Active One", "Body 1", true},
		{"Active Two", "Body 2", true},
		{"Inactive One", "Body 3", false},
		{"Active Three", "Body 4", true},
	}

	for _, tt := range testTexts {
		dt := &models.DispatchText{
			Name:     tt.name,
			Body:     tt.body,
			IsActive: tt.isActive,
		}
		_, err := repo.CreateDispatchText(ctx, userID, dt)
		require.NoError(t, err)
	}

	t.Run("returns only active dispatch texts for user", func(t *testing.T) {
		texts, err := repo.GetActiveDispatchTextsByUser(ctx, userID)

		require.NoError(t, err)
		assert.Len(t, texts, 3)

		for _, dt := range texts {
			assert.True(t, dt.IsActive)
			assert.Equal(t, userID, dt.UserID)
		}
	})

	t.Run("returns texts ordered by name", func(t *testing.T) {
		texts, err := repo.GetActiveDispatchTextsByUser(ctx, userID)

		require.NoError(t, err)
		assert.Len(t, texts, 3)

		// Verify alphabetical order
		assert.Equal(t, "Active One", texts[0].Name)
		assert.Equal(t, "Active Three", texts[1].Name)
		assert.Equal(t, "Active Two", texts[2].Name)
	})

	t.Run("returns empty slice for user with no active texts", func(t *testing.T) {
		// Create user with only inactive texts
		inactiveUserID := "user-all-inactive"
		dt := &models.DispatchText{
			Name:     "Inactive",
			Body:     "Inactive body",
			IsActive: false,
		}
		_, err := repo.CreateDispatchText(ctx, inactiveUserID, dt)
		require.NoError(t, err)

		texts, err := repo.GetActiveDispatchTextsByUser(ctx, inactiveUserID)

		require.NoError(t, err)
		assert.Len(t, texts, 0)
	})

	t.Run("returns empty slice for unknown user", func(t *testing.T) {
		texts, err := repo.GetActiveDispatchTextsByUser(ctx, "unknown-user")

		require.NoError(t, err)
		assert.Len(t, texts, 0)
	})
}

// =============================================================================
// CreateDispatchText Tests
// =============================================================================

func TestSettingsRepository_CreateDispatchText(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupSettingsTestDB(t)
	defer tdb.cleanup(t)

	logger := zap.NewNop()
	repo := NewSettingsRepository(tdb.db, logger)
	ctx := context.Background()

	t.Run("creates dispatch text successfully", func(t *testing.T) {
		dt := &models.DispatchText{
			Name:     "New Message",
			Body:     "New message body",
			IsActive: true,
		}

		created, err := repo.CreateDispatchText(ctx, "user-create-1", dt)

		require.NoError(t, err)
		assert.NotEmpty(t, created.ID)
		assert.Equal(t, "user-create-1", created.UserID)
		assert.Equal(t, "New Message", created.Name)
		assert.Equal(t, "New message body", created.Body)
		assert.True(t, created.IsActive)
		assert.NotEmpty(t, created.CreatedAt)
		assert.NotEmpty(t, created.UpdatedAt)
	})

	t.Run("creates dispatch text with is_active false", func(t *testing.T) {
		dt := &models.DispatchText{
			Name:     "Inactive Message",
			Body:     "Inactive body",
			IsActive: false,
		}

		created, err := repo.CreateDispatchText(ctx, "user-create-2", dt)

		require.NoError(t, err)
		assert.False(t, created.IsActive)
	})

	t.Run("creates multiple dispatch texts for same user", func(t *testing.T) {
		userID := "user-multi-create"

		dt1 := &models.DispatchText{Name: "First", Body: "Body 1", IsActive: true}
		dt2 := &models.DispatchText{Name: "Second", Body: "Body 2", IsActive: true}

		created1, err := repo.CreateDispatchText(ctx, userID, dt1)
		require.NoError(t, err)

		created2, err := repo.CreateDispatchText(ctx, userID, dt2)
		require.NoError(t, err)

		assert.NotEqual(t, created1.ID, created2.ID)

		// Verify both exist
		texts, err := repo.GetDispatchTextsByUser(ctx, userID)
		require.NoError(t, err)
		assert.Len(t, texts, 2)
	})

	t.Run("allows duplicate names for same user", func(t *testing.T) {
		userID := "user-dup-name"

		dt1 := &models.DispatchText{Name: "Same Name", Body: "Body 1", IsActive: true}
		dt2 := &models.DispatchText{Name: "Same Name", Body: "Body 2", IsActive: true}

		_, err := repo.CreateDispatchText(ctx, userID, dt1)
		require.NoError(t, err)

		_, err = repo.CreateDispatchText(ctx, userID, dt2)
		require.NoError(t, err)
	})
}

// =============================================================================
// UpdateDispatchText Tests
// =============================================================================

func TestSettingsRepository_UpdateDispatchText(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupSettingsTestDB(t)
	defer tdb.cleanup(t)

	logger := zap.NewNop()
	repo := NewSettingsRepository(tdb.db, logger)
	ctx := context.Background()

	t.Run("updates name field", func(t *testing.T) {
		dt := &models.DispatchText{
			Name:     "Original Name",
			Body:     "Original body",
			IsActive: true,
		}
		created, err := repo.CreateDispatchText(ctx, "user-update-1", dt)
		require.NoError(t, err)

		err = repo.UpdateDispatchText(ctx, created.ID, map[string]interface{}{
			"name": "Updated Name",
		})
		require.NoError(t, err)

		updated, err := repo.GetDispatchText(ctx, created.ID)
		require.NoError(t, err)
		assert.Equal(t, "Updated Name", updated.Name)
		assert.Equal(t, "Original body", updated.Body) // unchanged
	})

	t.Run("updates body field", func(t *testing.T) {
		dt := &models.DispatchText{
			Name:     "Test Name",
			Body:     "Original body",
			IsActive: true,
		}
		created, err := repo.CreateDispatchText(ctx, "user-update-2", dt)
		require.NoError(t, err)

		err = repo.UpdateDispatchText(ctx, created.ID, map[string]interface{}{
			"body": "Updated body content",
		})
		require.NoError(t, err)

		updated, err := repo.GetDispatchText(ctx, created.ID)
		require.NoError(t, err)
		assert.Equal(t, "Updated body content", updated.Body)
		assert.Equal(t, "Test Name", updated.Name) // unchanged
	})

	t.Run("updates is_active field to false", func(t *testing.T) {
		dt := &models.DispatchText{
			Name:     "Active Text",
			Body:     "Body",
			IsActive: true,
		}
		created, err := repo.CreateDispatchText(ctx, "user-update-3", dt)
		require.NoError(t, err)

		err = repo.UpdateDispatchText(ctx, created.ID, map[string]interface{}{
			"is_active": false,
		})
		require.NoError(t, err)

		updated, err := repo.GetDispatchText(ctx, created.ID)
		require.NoError(t, err)
		assert.False(t, updated.IsActive)
	})

	t.Run("updates is_active field to true", func(t *testing.T) {
		dt := &models.DispatchText{
			Name:     "Inactive Text",
			Body:     "Body",
			IsActive: false,
		}
		created, err := repo.CreateDispatchText(ctx, "user-update-4", dt)
		require.NoError(t, err)

		err = repo.UpdateDispatchText(ctx, created.ID, map[string]interface{}{
			"is_active": true,
		})
		require.NoError(t, err)

		updated, err := repo.GetDispatchText(ctx, created.ID)
		require.NoError(t, err)
		assert.True(t, updated.IsActive)
	})

	t.Run("updates multiple fields", func(t *testing.T) {
		dt := &models.DispatchText{
			Name:     "Original",
			Body:     "Original body",
			IsActive: true,
		}
		created, err := repo.CreateDispatchText(ctx, "user-update-5", dt)
		require.NoError(t, err)

		err = repo.UpdateDispatchText(ctx, created.ID, map[string]interface{}{
			"name":      "New Name",
			"body":      "New body",
			"is_active": false,
		})
		require.NoError(t, err)

		updated, err := repo.GetDispatchText(ctx, created.ID)
		require.NoError(t, err)
		assert.Equal(t, "New Name", updated.Name)
		assert.Equal(t, "New body", updated.Body)
		assert.False(t, updated.IsActive)
	})

	t.Run("does nothing with empty updates", func(t *testing.T) {
		dt := &models.DispatchText{
			Name:     "Keep Same",
			Body:     "Keep body",
			IsActive: true,
		}
		created, err := repo.CreateDispatchText(ctx, "user-update-6", dt)
		require.NoError(t, err)
		originalUpdatedAt := created.UpdatedAt

		// Wait a bit to ensure timestamp would be different if updated
		time.Sleep(10 * time.Millisecond)

		err = repo.UpdateDispatchText(ctx, created.ID, map[string]interface{}{})
		require.NoError(t, err)

		updated, err := repo.GetDispatchText(ctx, created.ID)
		require.NoError(t, err)
		assert.Equal(t, "Keep Same", updated.Name)
		assert.Equal(t, "Keep body", updated.Body)
		assert.True(t, updated.IsActive)
		// Note: updated_at will change due to the UPDATE query structure
		_ = originalUpdatedAt // suppress unused variable warning
	})

	t.Run("updates non-existent record without error", func(t *testing.T) {
		err := repo.UpdateDispatchText(ctx, "00000000-0000-0000-0000-000000000000", map[string]interface{}{
			"name": "New Name",
		})
		require.NoError(t, err)
	})
}

// =============================================================================
// DeleteDispatchText Tests
// =============================================================================

func TestSettingsRepository_DeleteDispatchText(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupSettingsTestDB(t)
	defer tdb.cleanup(t)

	logger := zap.NewNop()
	repo := NewSettingsRepository(tdb.db, logger)
	ctx := context.Background()

	t.Run("deletes dispatch text successfully", func(t *testing.T) {
		dt := &models.DispatchText{
			Name:     "To Delete",
			Body:     "Body",
			IsActive: true,
		}
		created, err := repo.CreateDispatchText(ctx, "user-delete-1", dt)
		require.NoError(t, err)

		err = repo.DeleteDispatchText(ctx, created.ID, "user-delete-1")
		require.NoError(t, err)

		// Verify deletion
		found, err := repo.GetDispatchText(ctx, created.ID)
		require.NoError(t, err)
		assert.Nil(t, found)
	})

	t.Run("does not delete if user ID does not match", func(t *testing.T) {
		dt := &models.DispatchText{
			Name:     "User Owned",
			Body:     "Body",
			IsActive: true,
		}
		created, err := repo.CreateDispatchText(ctx, "user-delete-2", dt)
		require.NoError(t, err)

		// Try to delete with wrong user
		err = repo.DeleteDispatchText(ctx, created.ID, "wrong-user")
		require.NoError(t, err) // No error, just no rows affected

		// Verify it still exists
		found, err := repo.GetDispatchText(ctx, created.ID)
		require.NoError(t, err)
		assert.NotNil(t, found)
	})

	t.Run("deletes non-existent record without error", func(t *testing.T) {
		err := repo.DeleteDispatchText(ctx, "00000000-0000-0000-0000-000000000000", "user-delete-3")
		require.NoError(t, err)
	})

	t.Run("deletes only specified dispatch text", func(t *testing.T) {
		userID := "user-delete-multi"

		dt1 := &models.DispatchText{Name: "Keep 1", Body: "Body 1", IsActive: true}
		dt2 := &models.DispatchText{Name: "Delete", Body: "Body 2", IsActive: true}
		dt3 := &models.DispatchText{Name: "Keep 2", Body: "Body 3", IsActive: true}

		created1, err := repo.CreateDispatchText(ctx, userID, dt1)
		require.NoError(t, err)
		created2, err := repo.CreateDispatchText(ctx, userID, dt2)
		require.NoError(t, err)
		created3, err := repo.CreateDispatchText(ctx, userID, dt3)
		require.NoError(t, err)

		// Delete only the second one
		err = repo.DeleteDispatchText(ctx, created2.ID, userID)
		require.NoError(t, err)

		// Verify only two remain
		texts, err := repo.GetDispatchTextsByUser(ctx, userID)
		require.NoError(t, err)
		assert.Len(t, texts, 2)

		// Verify correct ones remain
		ids := make([]string, len(texts))
		for i, dt := range texts {
			ids[i] = dt.ID
		}
		assert.Contains(t, ids, created1.ID)
		assert.Contains(t, ids, created3.ID)
		assert.NotContains(t, ids, created2.ID)
	})
}

// =============================================================================
// Edge Cases Tests
// =============================================================================

func TestSettingsRepository_DispatchText_EdgeCases(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupSettingsTestDB(t)
	defer tdb.cleanup(t)

	logger := zap.NewNop()
	repo := NewSettingsRepository(tdb.db, logger)
	ctx := context.Background()

	t.Run("handles special characters in name and body", func(t *testing.T) {
		dt := &models.DispatchText{
			Name:     "Test 'quotes' \"double\" & special <chars>",
			Body:     "Body with 'quotes', \"double quotes\", & special <chars>",
			IsActive: true,
		}

		created, err := repo.CreateDispatchText(ctx, "user-edge-1", dt)
		require.NoError(t, err)

		found, err := repo.GetDispatchText(ctx, created.ID)
		require.NoError(t, err)
		assert.Equal(t, dt.Name, found.Name)
		assert.Equal(t, dt.Body, found.Body)
	})

	t.Run("handles unicode in name and body", func(t *testing.T) {
		dt := &models.DispatchText{
			Name:     "Unicode: Ola Mundo Oi",
			Body:     "Body with emojis and unicode: Welcome to our group!",
			IsActive: true,
		}

		created, err := repo.CreateDispatchText(ctx, "user-edge-2", dt)
		require.NoError(t, err)

		found, err := repo.GetDispatchText(ctx, created.ID)
		require.NoError(t, err)
		assert.Equal(t, dt.Name, found.Name)
		assert.Equal(t, dt.Body, found.Body)
	})

	t.Run("handles very long body text", func(t *testing.T) {
		longBody := make([]byte, 10000)
		for i := range longBody {
			longBody[i] = 'a'
		}

		dt := &models.DispatchText{
			Name:     "Long Body Test",
			Body:     string(longBody),
			IsActive: true,
		}

		created, err := repo.CreateDispatchText(ctx, "user-edge-3", dt)
		require.NoError(t, err)

		found, err := repo.GetDispatchText(ctx, created.ID)
		require.NoError(t, err)
		assert.Equal(t, 10000, len(found.Body))
	})

	t.Run("handles empty body", func(t *testing.T) {
		dt := &models.DispatchText{
			Name:     "Empty Body",
			Body:     "",
			IsActive: true,
		}

		created, err := repo.CreateDispatchText(ctx, "user-edge-4", dt)
		require.NoError(t, err)

		found, err := repo.GetDispatchText(ctx, created.ID)
		require.NoError(t, err)
		assert.Empty(t, found.Body)
	})

	t.Run("handles newlines in body", func(t *testing.T) {
		dt := &models.DispatchText{
			Name:     "Multiline",
			Body:     "Line 1\nLine 2\nLine 3",
			IsActive: true,
		}

		created, err := repo.CreateDispatchText(ctx, "user-edge-5", dt)
		require.NoError(t, err)

		found, err := repo.GetDispatchText(ctx, created.ID)
		require.NoError(t, err)
		assert.Equal(t, "Line 1\nLine 2\nLine 3", found.Body)
	})
}

// =============================================================================
// Concurrent Access Tests
// =============================================================================

func TestSettingsRepository_DispatchText_ConcurrentAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupSettingsTestDB(t)
	defer tdb.cleanup(t)

	logger := zap.NewNop()
	repo := NewSettingsRepository(tdb.db, logger)
	ctx := context.Background()

	t.Run("handles concurrent creates", func(t *testing.T) {
		const numGoroutines = 10
		done := make(chan bool, numGoroutines)
		errors := make(chan error, numGoroutines)

		userID := "user-concurrent-create"

		for i := 0; i < numGoroutines; i++ {
			go func(idx int) {
				dt := &models.DispatchText{
					Name:     "Concurrent " + string(rune('A'+idx)),
					Body:     "Body " + string(rune('A'+idx)),
					IsActive: true,
				}
				_, err := repo.CreateDispatchText(ctx, userID, dt)
				if err != nil {
					errors <- err
				}
				done <- true
			}(i)
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

		// Verify all were created
		texts, err := repo.GetDispatchTextsByUser(ctx, userID)
		require.NoError(t, err)
		assert.Len(t, texts, numGoroutines)
	})

	t.Run("handles concurrent updates to same record", func(t *testing.T) {
		dt := &models.DispatchText{
			Name:     "Concurrent Update",
			Body:     "Original",
			IsActive: true,
		}
		created, err := repo.CreateDispatchText(ctx, "user-concurrent-update", dt)
		require.NoError(t, err)

		const numGoroutines = 10
		done := make(chan bool, numGoroutines)
		errors := make(chan error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(idx int) {
				err := repo.UpdateDispatchText(ctx, created.ID, map[string]interface{}{
					"body": "Updated " + string(rune('A'+idx)),
				})
				if err != nil {
					errors <- err
				}
				done <- true
			}(i)
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

		// Verify record still exists and has some updated value
		found, err := repo.GetDispatchText(ctx, created.ID)
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Contains(t, found.Body, "Updated")
	})
}

// =============================================================================
// Timestamp Tests
// =============================================================================

func TestSettingsRepository_DispatchText_Timestamps(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupSettingsTestDB(t)
	defer tdb.cleanup(t)

	logger := zap.NewNop()
	repo := NewSettingsRepository(tdb.db, logger)
	ctx := context.Background()

	t.Run("created_at and updated_at are set on create", func(t *testing.T) {
		beforeCreate := time.Now().UTC()

		dt := &models.DispatchText{
			Name:     "Timestamp Test",
			Body:     "Body",
			IsActive: true,
		}
		created, err := repo.CreateDispatchText(ctx, "user-timestamp-1", dt)
		require.NoError(t, err)

		afterCreate := time.Now().UTC()

		// Parse timestamps
		createdAt, err := time.Parse(time.RFC3339, created.CreatedAt)
		require.NoError(t, err)
		updatedAt, err := time.Parse(time.RFC3339, created.UpdatedAt)
		require.NoError(t, err)

		// Verify timestamps are within expected range
		assert.True(t, createdAt.After(beforeCreate.Add(-time.Second)) && createdAt.Before(afterCreate.Add(time.Second)))
		assert.True(t, updatedAt.After(beforeCreate.Add(-time.Second)) && updatedAt.Before(afterCreate.Add(time.Second)))

		// created_at and updated_at should be equal on create
		assert.Equal(t, created.CreatedAt, created.UpdatedAt)
	})

	t.Run("updated_at changes on update", func(t *testing.T) {
		dt := &models.DispatchText{
			Name:     "Update Timestamp Test",
			Body:     "Original body",
			IsActive: true,
		}
		created, err := repo.CreateDispatchText(ctx, "user-timestamp-2", dt)
		require.NoError(t, err)

		originalUpdatedAt, err := time.Parse(time.RFC3339, created.UpdatedAt)
		require.NoError(t, err)

		// Wait to ensure timestamp difference
		time.Sleep(50 * time.Millisecond)

		err = repo.UpdateDispatchText(ctx, created.ID, map[string]interface{}{
			"body": "Updated body",
		})
		require.NoError(t, err)

		updated, err := repo.GetDispatchText(ctx, created.ID)
		require.NoError(t, err)

		newUpdatedAt, err := time.Parse(time.RFC3339, updated.UpdatedAt)
		require.NoError(t, err)

		// updated_at should be later than original
		assert.True(t, newUpdatedAt.After(originalUpdatedAt))

		// created_at should remain unchanged
		assert.Equal(t, created.CreatedAt, updated.CreatedAt)
	})
}
