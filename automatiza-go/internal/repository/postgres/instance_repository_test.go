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
// Test Database Setup for Instance Repository
// =============================================================================

// instanceTestDB holds the test database and container for instance tests.
type instanceTestDB struct {
	container testcontainers.Container
	pool      *pgxpool.Pool
	db        *DB
}

// setupInstanceTestDB creates a PostgreSQL container and initializes the schema.
func setupInstanceTestDB(t *testing.T) *instanceTestDB {
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
	_, err = pool.Exec(ctx, instanceSchema)
	require.NoError(t, err, "failed to create schema")

	logger := zap.NewNop()

	return &instanceTestDB{
		container: pgContainer,
		pool:      pool,
		db:        &DB{pool: pool, logger: logger},
	}
}

// cleanup closes the database connection and terminates the container.
func (tdb *instanceTestDB) cleanup(t *testing.T) {
	t.Helper()
	ctx := context.Background()

	if tdb.pool != nil {
		tdb.pool.Close()
	}
	if tdb.container != nil {
		require.NoError(t, tdb.container.Terminate(ctx), "failed to terminate container")
	}
}

// Schema for instances table
const instanceSchema = `
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS instances (
    id SERIAL PRIMARY KEY,
    user_id VARCHAR(36) NOT NULL,
    instance_id VARCHAR(255) NOT NULL,
    friendly_name VARCHAR(255),
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    qr_code TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, instance_id)
);

CREATE INDEX IF NOT EXISTS idx_instances_user_id ON instances(user_id);
CREATE INDEX IF NOT EXISTS idx_instances_status ON instances(status);
`

// =============================================================================
// UpsertInstance Tests
// =============================================================================

func TestInstanceRepository_UpsertInstance(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupInstanceTestDB(t)
	defer tdb.cleanup(t)

	logger := zap.NewNop()
	repo := NewInstanceRepository(tdb.db, logger)
	ctx := context.Background()

	t.Run("creates new instance successfully", func(t *testing.T) {
		friendlyName := "My WhatsApp"
		instance := &models.Instance{
			InstanceID:   "instance-1",
			FriendlyName: &friendlyName,
			Status:       models.InstanceStatusPending,
		}

		created, err := repo.UpsertInstance(ctx, "user-1", instance)

		require.NoError(t, err)
		assert.NotZero(t, created.ID)
		assert.Equal(t, "user-1", created.UserID)
		assert.Equal(t, "instance-1", created.InstanceID)
		assert.Equal(t, "My WhatsApp", *created.FriendlyName)
		assert.Equal(t, models.InstanceStatusPending, created.Status)
		assert.NotEmpty(t, created.CreatedAt)
		assert.NotEmpty(t, created.UpdatedAt)
	})

	t.Run("updates existing instance on conflict", func(t *testing.T) {
		friendlyName := "Original Name"
		instance := &models.Instance{
			InstanceID:   "instance-upsert",
			FriendlyName: &friendlyName,
			Status:       models.InstanceStatusPending,
		}

		created, err := repo.UpsertInstance(ctx, "user-upsert", instance)
		require.NoError(t, err)
		originalID := created.ID

		// Update with new status
		newName := "Updated Name"
		updated := &models.Instance{
			InstanceID:   "instance-upsert",
			FriendlyName: &newName,
			Status:       models.InstanceStatusConnected,
		}

		result, err := repo.UpsertInstance(ctx, "user-upsert", updated)

		require.NoError(t, err)
		assert.Equal(t, originalID, result.ID) // Same ID - was updated, not inserted
		assert.Equal(t, models.InstanceStatusConnected, result.Status)
		assert.Equal(t, "Updated Name", *result.FriendlyName)
	})

	t.Run("creates instance without friendly name", func(t *testing.T) {
		instance := &models.Instance{
			InstanceID: "instance-no-name",
			Status:     models.InstanceStatusPending,
		}

		created, err := repo.UpsertInstance(ctx, "user-2", instance)

		require.NoError(t, err)
		assert.NotZero(t, created.ID)
		assert.Nil(t, created.FriendlyName)
	})

	t.Run("creates instance with QR code", func(t *testing.T) {
		qrCode := "data:image/png;base64,..."
		instance := &models.Instance{
			InstanceID: "instance-qr",
			Status:     models.InstanceStatusPending,
			QRCode:     &qrCode,
		}

		created, err := repo.UpsertInstance(ctx, "user-3", instance)

		require.NoError(t, err)
		require.NotNil(t, created.QRCode)
		assert.Equal(t, qrCode, *created.QRCode)
	})
}

// =============================================================================
// GetUserInstances Tests
// =============================================================================

func TestInstanceRepository_GetUserInstances(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupInstanceTestDB(t)
	defer tdb.cleanup(t)

	logger := zap.NewNop()
	repo := NewInstanceRepository(tdb.db, logger)
	ctx := context.Background()

	userID := "user-list-test"

	// Create test instances
	for i := 1; i <= 3; i++ {
		name := "Instance " + string(rune('0'+i))
		instance := &models.Instance{
			InstanceID:   "instance-" + string(rune('0'+i)),
			FriendlyName: &name,
			Status:       models.InstanceStatusPending,
		}
		_, err := repo.UpsertInstance(ctx, userID, instance)
		require.NoError(t, err)
		time.Sleep(10 * time.Millisecond) // Ensure different timestamps
	}

	// Create instance for another user
	otherInstance := &models.Instance{
		InstanceID: "other-instance",
		Status:     models.InstanceStatusPending,
	}
	_, err := repo.UpsertInstance(ctx, "other-user", otherInstance)
	require.NoError(t, err)

	t.Run("returns all instances for user ordered by created_at DESC", func(t *testing.T) {
		instances, err := repo.GetUserInstances(ctx, userID)

		require.NoError(t, err)
		assert.Len(t, instances, 3)

		// Verify order (most recent first)
		for i := 0; i < len(instances)-1; i++ {
			assert.True(t, instances[i].CreatedAt >= instances[i+1].CreatedAt,
				"expected instances to be ordered by created_at DESC")
		}
	})

	t.Run("does not return other users instances", func(t *testing.T) {
		instances, err := repo.GetUserInstances(ctx, userID)

		require.NoError(t, err)
		for _, inst := range instances {
			assert.Equal(t, userID, inst.UserID)
		}
	})

	t.Run("returns empty slice for unknown user", func(t *testing.T) {
		instances, err := repo.GetUserInstances(ctx, "unknown-user")

		require.NoError(t, err)
		assert.Len(t, instances, 0)
	})
}

// =============================================================================
// GetInstanceByID Tests
// =============================================================================

func TestInstanceRepository_GetInstanceByID(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupInstanceTestDB(t)
	defer tdb.cleanup(t)

	logger := zap.NewNop()
	repo := NewInstanceRepository(tdb.db, logger)
	ctx := context.Background()

	t.Run("returns instance by ID", func(t *testing.T) {
		friendlyName := "Test Instance"
		instance := &models.Instance{
			InstanceID:   "instance-get-id",
			FriendlyName: &friendlyName,
			Status:       models.InstanceStatusConnected,
		}
		created, err := repo.UpsertInstance(ctx, "user-get", instance)
		require.NoError(t, err)

		found, err := repo.GetInstanceByID(ctx, created.ID)

		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, created.ID, found.ID)
		assert.Equal(t, "user-get", found.UserID)
		assert.Equal(t, "instance-get-id", found.InstanceID)
		assert.Equal(t, "Test Instance", *found.FriendlyName)
		assert.Equal(t, models.InstanceStatusConnected, found.Status)
	})

	t.Run("returns nil for non-existent ID", func(t *testing.T) {
		found, err := repo.GetInstanceByID(ctx, 999999)

		require.NoError(t, err)
		assert.Nil(t, found)
	})
}

// =============================================================================
// GetInstanceByName Tests
// =============================================================================

func TestInstanceRepository_GetInstanceByName(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupInstanceTestDB(t)
	defer tdb.cleanup(t)

	logger := zap.NewNop()
	repo := NewInstanceRepository(tdb.db, logger)
	ctx := context.Background()

	t.Run("returns instance by instance_id (name)", func(t *testing.T) {
		instance := &models.Instance{
			InstanceID: "unique-instance-name",
			Status:     models.InstanceStatusOpen,
		}
		_, err := repo.UpsertInstance(ctx, "user-name-test", instance)
		require.NoError(t, err)

		found, err := repo.GetInstanceByName(ctx, "unique-instance-name")

		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, "unique-instance-name", found.InstanceID)
		assert.Equal(t, models.InstanceStatusOpen, found.Status)
	})

	t.Run("returns nil for non-existent name", func(t *testing.T) {
		found, err := repo.GetInstanceByName(ctx, "non-existent-name")

		require.NoError(t, err)
		assert.Nil(t, found)
	})
}

// =============================================================================
// GetInstanceByUserAndName Tests
// =============================================================================

func TestInstanceRepository_GetInstanceByUserAndName(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupInstanceTestDB(t)
	defer tdb.cleanup(t)

	logger := zap.NewNop()
	repo := NewInstanceRepository(tdb.db, logger)
	ctx := context.Background()

	userID := "user-and-name-test"
	instanceName := "shared-name"

	// Create instance
	instance := &models.Instance{
		InstanceID: instanceName,
		Status:     models.InstanceStatusConnected,
	}
	_, err := repo.UpsertInstance(ctx, userID, instance)
	require.NoError(t, err)

	t.Run("returns instance for matching user and name", func(t *testing.T) {
		found, err := repo.GetInstanceByUserAndName(ctx, userID, instanceName)

		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, userID, found.UserID)
		assert.Equal(t, instanceName, found.InstanceID)
	})

	t.Run("returns nil for wrong user", func(t *testing.T) {
		found, err := repo.GetInstanceByUserAndName(ctx, "different-user", instanceName)

		require.NoError(t, err)
		assert.Nil(t, found)
	})

	t.Run("returns nil for wrong name", func(t *testing.T) {
		found, err := repo.GetInstanceByUserAndName(ctx, userID, "different-name")

		require.NoError(t, err)
		assert.Nil(t, found)
	})
}

// =============================================================================
// GetAllConnectedInstances Tests
// =============================================================================

func TestInstanceRepository_GetAllConnectedInstances(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupInstanceTestDB(t)
	defer tdb.cleanup(t)

	logger := zap.NewNop()
	repo := NewInstanceRepository(tdb.db, logger)
	ctx := context.Background()

	// Create instances with different statuses
	statuses := []struct {
		instanceID string
		status     string
		shouldFind bool
	}{
		{"connected-1", models.InstanceStatusConnected, true},
		{"connected-2", models.InstanceStatusOpen, true},
		{"disconnected-1", models.InstanceStatusDisconnected, false},
		{"pending-1", models.InstanceStatusPending, false},
	}

	for _, s := range statuses {
		instance := &models.Instance{
			InstanceID: s.instanceID,
			Status:     s.status,
		}
		_, err := repo.UpsertInstance(ctx, "user-connected-test", instance)
		require.NoError(t, err)
	}

	t.Run("returns only connected and open instances", func(t *testing.T) {
		instances, err := repo.GetAllConnectedInstances(ctx)

		require.NoError(t, err)
		assert.Len(t, instances, 2)

		for _, inst := range instances {
			assert.True(t,
				inst.Status == models.InstanceStatusConnected || inst.Status == models.InstanceStatusOpen,
				"expected only connected or open instances, got %s", inst.Status)
		}
	})

	t.Run("returns instances ordered by created_at DESC", func(t *testing.T) {
		instances, err := repo.GetAllConnectedInstances(ctx)

		require.NoError(t, err)
		for i := 0; i < len(instances)-1; i++ {
			assert.True(t, instances[i].CreatedAt >= instances[i+1].CreatedAt,
				"expected instances to be ordered by created_at DESC")
		}
	})
}

// =============================================================================
// UpdateInstanceStatus Tests
// =============================================================================

func TestInstanceRepository_UpdateInstanceStatus(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupInstanceTestDB(t)
	defer tdb.cleanup(t)

	logger := zap.NewNop()
	repo := NewInstanceRepository(tdb.db, logger)
	ctx := context.Background()

	t.Run("updates status successfully", func(t *testing.T) {
		instance := &models.Instance{
			InstanceID: "status-update-test",
			Status:     models.InstanceStatusPending,
		}
		created, err := repo.UpsertInstance(ctx, "user-status", instance)
		require.NoError(t, err)

		err = repo.UpdateInstanceStatus(ctx, created.ID, models.InstanceStatusConnected)
		require.NoError(t, err)

		// Verify
		found, err := repo.GetInstanceByID(ctx, created.ID)
		require.NoError(t, err)
		assert.Equal(t, models.InstanceStatusConnected, found.Status)
	})

	t.Run("updates status to disconnected", func(t *testing.T) {
		instance := &models.Instance{
			InstanceID: "status-disconnect-test",
			Status:     models.InstanceStatusConnected,
		}
		created, err := repo.UpsertInstance(ctx, "user-status-2", instance)
		require.NoError(t, err)

		err = repo.UpdateInstanceStatus(ctx, created.ID, models.InstanceStatusDisconnected)
		require.NoError(t, err)

		// Verify
		found, err := repo.GetInstanceByID(ctx, created.ID)
		require.NoError(t, err)
		assert.Equal(t, models.InstanceStatusDisconnected, found.Status)
	})

	t.Run("updates non-existent instance without error", func(t *testing.T) {
		err := repo.UpdateInstanceStatus(ctx, 999999, models.InstanceStatusConnected)
		require.NoError(t, err)
	})
}

// =============================================================================
// DeleteInstance Tests
// =============================================================================

func TestInstanceRepository_DeleteInstance(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupInstanceTestDB(t)
	defer tdb.cleanup(t)

	logger := zap.NewNop()
	repo := NewInstanceRepository(tdb.db, logger)
	ctx := context.Background()

	t.Run("deletes instance successfully", func(t *testing.T) {
		instance := &models.Instance{
			InstanceID: "to-delete",
			Status:     models.InstanceStatusPending,
		}
		created, err := repo.UpsertInstance(ctx, "user-delete", instance)
		require.NoError(t, err)

		err = repo.DeleteInstance(ctx, created.ID)
		require.NoError(t, err)

		// Verify deletion
		found, err := repo.GetInstanceByID(ctx, created.ID)
		require.NoError(t, err)
		assert.Nil(t, found)
	})

	t.Run("deletes non-existent instance without error", func(t *testing.T) {
		err := repo.DeleteInstance(ctx, 999999)
		require.NoError(t, err)
	})

	t.Run("only deletes specified instance", func(t *testing.T) {
		userID := "user-multi-delete"

		// Create multiple instances
		inst1 := &models.Instance{InstanceID: "keep-1", Status: models.InstanceStatusPending}
		inst2 := &models.Instance{InstanceID: "delete-me", Status: models.InstanceStatusPending}
		inst3 := &models.Instance{InstanceID: "keep-2", Status: models.InstanceStatusPending}

		created1, err := repo.UpsertInstance(ctx, userID, inst1)
		require.NoError(t, err)
		created2, err := repo.UpsertInstance(ctx, userID, inst2)
		require.NoError(t, err)
		created3, err := repo.UpsertInstance(ctx, userID, inst3)
		require.NoError(t, err)

		// Delete only the second
		err = repo.DeleteInstance(ctx, created2.ID)
		require.NoError(t, err)

		// Verify only two remain
		instances, err := repo.GetUserInstances(ctx, userID)
		require.NoError(t, err)
		assert.Len(t, instances, 2)

		ids := make([]int, len(instances))
		for i, inst := range instances {
			ids[i] = inst.ID
		}
		assert.Contains(t, ids, created1.ID)
		assert.Contains(t, ids, created3.ID)
		assert.NotContains(t, ids, created2.ID)
	})
}

// =============================================================================
// Edge Cases Tests
// =============================================================================

func TestInstanceRepository_EdgeCases(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupInstanceTestDB(t)
	defer tdb.cleanup(t)

	logger := zap.NewNop()
	repo := NewInstanceRepository(tdb.db, logger)
	ctx := context.Background()

	t.Run("handles special characters in friendly name", func(t *testing.T) {
		friendlyName := "Test 'quotes' \"double\" & special <chars>"
		instance := &models.Instance{
			InstanceID:   "special-chars-instance",
			FriendlyName: &friendlyName,
			Status:       models.InstanceStatusPending,
		}

		created, err := repo.UpsertInstance(ctx, "user-special", instance)
		require.NoError(t, err)

		found, err := repo.GetInstanceByID(ctx, created.ID)
		require.NoError(t, err)
		assert.Equal(t, friendlyName, *found.FriendlyName)
	})

	t.Run("handles unicode in friendly name", func(t *testing.T) {
		friendlyName := "WhatsApp Promocoes Brasil"
		instance := &models.Instance{
			InstanceID:   "unicode-instance",
			FriendlyName: &friendlyName,
			Status:       models.InstanceStatusPending,
		}

		created, err := repo.UpsertInstance(ctx, "user-unicode", instance)
		require.NoError(t, err)

		found, err := repo.GetInstanceByID(ctx, created.ID)
		require.NoError(t, err)
		assert.Equal(t, friendlyName, *found.FriendlyName)
	})

	t.Run("handles very long QR code", func(t *testing.T) {
		longQR := make([]byte, 10000)
		for i := range longQR {
			longQR[i] = 'a'
		}
		qrCode := string(longQR)

		instance := &models.Instance{
			InstanceID: "long-qr-instance",
			Status:     models.InstanceStatusPending,
			QRCode:     &qrCode,
		}

		created, err := repo.UpsertInstance(ctx, "user-long-qr", instance)
		require.NoError(t, err)

		found, err := repo.GetInstanceByID(ctx, created.ID)
		require.NoError(t, err)
		assert.Equal(t, 10000, len(*found.QRCode))
	})
}

// =============================================================================
// Concurrent Access Tests
// =============================================================================

func TestInstanceRepository_ConcurrentAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupInstanceTestDB(t)
	defer tdb.cleanup(t)

	logger := zap.NewNop()
	repo := NewInstanceRepository(tdb.db, logger)
	ctx := context.Background()

	t.Run("handles concurrent status updates", func(t *testing.T) {
		instance := &models.Instance{
			InstanceID: "concurrent-test",
			Status:     models.InstanceStatusPending,
		}
		created, err := repo.UpsertInstance(ctx, "user-concurrent", instance)
		require.NoError(t, err)

		const numGoroutines = 10
		done := make(chan bool, numGoroutines)
		errors := make(chan error, numGoroutines)

		// Launch concurrent updates
		for i := 0; i < numGoroutines; i++ {
			go func() {
				err := repo.UpdateInstanceStatus(ctx, created.ID, models.InstanceStatusConnected)
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
		found, err := repo.GetInstanceByID(ctx, created.ID)
		require.NoError(t, err)
		assert.Equal(t, models.InstanceStatusConnected, found.Status)
	})
}

// =============================================================================
// Timestamp Tests
// =============================================================================

func TestInstanceRepository_Timestamps(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupInstanceTestDB(t)
	defer tdb.cleanup(t)

	logger := zap.NewNop()
	repo := NewInstanceRepository(tdb.db, logger)
	ctx := context.Background()

	t.Run("created_at and updated_at are set on create", func(t *testing.T) {
		beforeCreate := time.Now().UTC()

		instance := &models.Instance{
			InstanceID: "timestamp-test",
			Status:     models.InstanceStatusPending,
		}
		created, err := repo.UpsertInstance(ctx, "user-timestamp", instance)
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
	})

	t.Run("updated_at changes on status update", func(t *testing.T) {
		instance := &models.Instance{
			InstanceID: "timestamp-update-test",
			Status:     models.InstanceStatusPending,
		}
		created, err := repo.UpsertInstance(ctx, "user-timestamp-2", instance)
		require.NoError(t, err)

		originalUpdatedAt, err := time.Parse(time.RFC3339, created.UpdatedAt)
		require.NoError(t, err)

		// Wait to ensure timestamp difference
		time.Sleep(50 * time.Millisecond)

		err = repo.UpdateInstanceStatus(ctx, created.ID, models.InstanceStatusConnected)
		require.NoError(t, err)

		updated, err := repo.GetInstanceByID(ctx, created.ID)
		require.NoError(t, err)

		newUpdatedAt, err := time.Parse(time.RFC3339, updated.UpdatedAt)
		require.NoError(t, err)

		// updated_at should be later than original
		assert.True(t, newUpdatedAt.After(originalUpdatedAt))

		// created_at should remain unchanged
		assert.Equal(t, created.CreatedAt, updated.CreatedAt)
	})
}
