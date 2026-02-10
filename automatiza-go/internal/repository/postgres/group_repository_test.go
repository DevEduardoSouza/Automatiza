package postgres

import (
	"context"
	"testing"
	"time"

	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/zap"
)

// =============================================================================
// Test Database Setup for Group Repository
// =============================================================================

// groupTestDB holds the test database and container for group tests.
type groupTestDB struct {
	container testcontainers.Container
	pool      *pgxpool.Pool
	db        *DB
}

// setupGroupTestDB creates a PostgreSQL container and initializes the schema.
func setupGroupTestDB(t *testing.T) *groupTestDB {
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
	_, err = pool.Exec(ctx, groupSchema)
	require.NoError(t, err, "failed to create schema")

	logger := zap.NewNop()

	return &groupTestDB{
		container: pgContainer,
		pool:      pool,
		db:        &DB{pool: pool, logger: logger},
	}
}

// cleanup closes the database connection and terminates the container.
func (tdb *groupTestDB) cleanup(t *testing.T) {
	t.Helper()
	ctx := context.Background()

	if tdb.pool != nil {
		tdb.pool.Close()
	}
	if tdb.container != nil {
		require.NoError(t, tdb.container.Terminate(ctx), "failed to terminate container")
	}
}

// Schema for groups table
const groupSchema = `
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

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
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, wa_group_id)
);

CREATE INDEX IF NOT EXISTS idx_groups_user_id ON groups(user_id);
CREATE INDEX IF NOT EXISTS idx_groups_instance_id ON groups(instance_id);
CREATE INDEX IF NOT EXISTS idx_groups_wa_group_id ON groups(wa_group_id);
CREATE INDEX IF NOT EXISTS idx_groups_is_favorite ON groups(is_favorite);
`

// =============================================================================
// CreateGroup Tests
// =============================================================================

func TestGroupRepository_CreateGroup(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupGroupTestDB(t)
	defer tdb.cleanup(t)

	logger := zap.NewNop()
	repo := NewGroupRepository(tdb.db, logger)
	ctx := context.Background()

	t.Run("creates group successfully", func(t *testing.T) {
		description := "Test group description"
		group := &models.Group{
			WAGroupID:    "123456789@g.us",
			Name:         "Test Group",
			InstanceID:   "1",
			Description:  &description,
			MembersCount: 50,
			MemberLimit:  256,
			IsAdmin:      true,
		}

		created, err := repo.CreateGroup(ctx, "user-1", group)

		require.NoError(t, err)
		assert.NotEmpty(t, created.ID)
		assert.Equal(t, "user-1", created.UserID)
		assert.Equal(t, "123456789@g.us", created.WAGroupID)
		assert.Equal(t, "Test Group", created.Name)
		assert.Equal(t, "Test group description", *created.Description)
		assert.Equal(t, 50, created.MembersCount)
		assert.Equal(t, 256, created.MemberLimit)
		assert.True(t, created.IsAdmin)
		assert.NotEmpty(t, created.CreatedAt)
		assert.NotEmpty(t, created.UpdatedAt)
	})

	t.Run("creates group without description", func(t *testing.T) {
		group := &models.Group{
			WAGroupID:  "987654321@g.us",
			Name:       "No Description Group",
			InstanceID: "1",
		}

		created, err := repo.CreateGroup(ctx, "user-2", group)

		require.NoError(t, err)
		assert.NotEmpty(t, created.ID)
		assert.Equal(t, "No Description Group", created.Name)
	})

	t.Run("creates group with admin_only flag", func(t *testing.T) {
		group := &models.Group{
			WAGroupID:  "admin-only-group@g.us",
			Name:       "Admin Only Group",
			InstanceID: "1",
			AdminOnly:  true,
		}

		created, err := repo.CreateGroup(ctx, "user-3", group)

		require.NoError(t, err)
		assert.True(t, created.AdminOnly)
	})
}

// =============================================================================
// GetUserGroups Tests
// =============================================================================

func TestGroupRepository_GetUserGroups(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupGroupTestDB(t)
	defer tdb.cleanup(t)

	logger := zap.NewNop()
	repo := NewGroupRepository(tdb.db, logger)
	ctx := context.Background()

	userID := "user-list-test"

	// Create test groups with different admin status
	testGroups := []struct {
		waGroupID string
		name      string
		isAdmin   bool
	}{
		{"group-a@g.us", "Alpha Group", true},
		{"group-b@g.us", "Beta Group", false},
		{"group-c@g.us", "Charlie Group", true},
		{"group-d@g.us", "Delta Group", false},
	}

	for _, tg := range testGroups {
		group := &models.Group{
			WAGroupID:  tg.waGroupID,
			Name:       tg.name,
			InstanceID: "1",
			IsAdmin:    tg.isAdmin,
		}
		_, err := repo.CreateGroup(ctx, userID, group)
		require.NoError(t, err)
	}

	// Create group for another user
	otherGroup := &models.Group{
		WAGroupID:  "other-group@g.us",
		Name:       "Other User Group",
		InstanceID: "1",
	}
	_, err := repo.CreateGroup(ctx, "other-user", otherGroup)
	require.NoError(t, err)

	t.Run("returns all user groups ordered by name", func(t *testing.T) {
		groups, err := repo.GetUserGroups(ctx, userID, false)

		require.NoError(t, err)
		assert.Len(t, groups, 4)

		// Verify alphabetical order by name
		assert.Equal(t, "Alpha Group", groups[0].Name)
		assert.Equal(t, "Beta Group", groups[1].Name)
		assert.Equal(t, "Charlie Group", groups[2].Name)
		assert.Equal(t, "Delta Group", groups[3].Name)
	})

	t.Run("returns only admin groups when adminOnly is true", func(t *testing.T) {
		groups, err := repo.GetUserGroups(ctx, userID, true)

		require.NoError(t, err)
		assert.Len(t, groups, 2)

		for _, g := range groups {
			assert.True(t, g.IsAdmin)
		}
	})

	t.Run("does not return other users groups", func(t *testing.T) {
		groups, err := repo.GetUserGroups(ctx, userID, false)

		require.NoError(t, err)
		for _, g := range groups {
			assert.Equal(t, userID, g.UserID)
		}
	})

	t.Run("returns empty slice for unknown user", func(t *testing.T) {
		groups, err := repo.GetUserGroups(ctx, "unknown-user", false)

		require.NoError(t, err)
		assert.Len(t, groups, 0)
	})
}

// =============================================================================
// GetGroupByID Tests
// =============================================================================

func TestGroupRepository_GetGroupByID(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupGroupTestDB(t)
	defer tdb.cleanup(t)

	logger := zap.NewNop()
	repo := NewGroupRepository(tdb.db, logger)
	ctx := context.Background()

	t.Run("returns group by ID", func(t *testing.T) {
		description := "Test description"
		group := &models.Group{
			WAGroupID:    "get-by-id@g.us",
			Name:         "Get By ID Group",
			InstanceID:   "1",
			Description:  &description,
			MembersCount: 100,
		}
		created, err := repo.CreateGroup(ctx, "user-get-id", group)
		require.NoError(t, err)

		found, err := repo.GetGroupByID(ctx, created.ID)

		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, created.ID, found.ID)
		assert.Equal(t, "user-get-id", found.UserID)
		assert.Equal(t, "get-by-id@g.us", found.WAGroupID)
		assert.Equal(t, "Get By ID Group", found.Name)
		assert.Equal(t, "Test description", *found.Description)
		assert.Equal(t, 100, found.MembersCount)
	})

	t.Run("returns nil for non-existent ID", func(t *testing.T) {
		found, err := repo.GetGroupByID(ctx, "00000000-0000-0000-0000-000000000000")

		require.NoError(t, err)
		assert.Nil(t, found)
	})
}

// =============================================================================
// GetGroupByWAGroupID Tests
// =============================================================================

func TestGroupRepository_GetGroupByWAGroupID(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupGroupTestDB(t)
	defer tdb.cleanup(t)

	logger := zap.NewNop()
	repo := NewGroupRepository(tdb.db, logger)
	ctx := context.Background()

	userID := "user-wa-group-id"
	waGroupID := "unique-wa-group@g.us"

	// Create group
	group := &models.Group{
		WAGroupID:  waGroupID,
		Name:       "WA Group Test",
		InstanceID: "1",
	}
	_, err := repo.CreateGroup(ctx, userID, group)
	require.NoError(t, err)

	t.Run("returns group for matching user and waGroupID", func(t *testing.T) {
		found, err := repo.GetGroupByWAGroupID(ctx, userID, waGroupID)

		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, userID, found.UserID)
		assert.Equal(t, waGroupID, found.WAGroupID)
	})

	t.Run("returns nil for wrong user", func(t *testing.T) {
		found, err := repo.GetGroupByWAGroupID(ctx, "different-user", waGroupID)

		require.NoError(t, err)
		assert.Nil(t, found)
	})

	t.Run("returns nil for non-existent waGroupID", func(t *testing.T) {
		found, err := repo.GetGroupByWAGroupID(ctx, userID, "non-existent@g.us")

		require.NoError(t, err)
		assert.Nil(t, found)
	})
}

// =============================================================================
// GetGroupsByIDs Tests
// =============================================================================

func TestGroupRepository_GetGroupsByIDs(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupGroupTestDB(t)
	defer tdb.cleanup(t)

	logger := zap.NewNop()
	repo := NewGroupRepository(tdb.db, logger)
	ctx := context.Background()

	userID := "user-by-ids"

	// Create test groups
	var groupIDs []string
	for i := 1; i <= 5; i++ {
		group := &models.Group{
			WAGroupID:  "group-" + string(rune('0'+i)) + "@g.us",
			Name:       "Group " + string(rune('0'+i)),
			InstanceID: "1",
		}
		created, err := repo.CreateGroup(ctx, userID, group)
		require.NoError(t, err)
		groupIDs = append(groupIDs, created.ID)
	}

	t.Run("returns groups by IDs", func(t *testing.T) {
		requestedIDs := []string{groupIDs[0], groupIDs[2], groupIDs[4]}
		groups, err := repo.GetGroupsByIDs(ctx, userID, requestedIDs)

		require.NoError(t, err)
		assert.Len(t, groups, 3)
	})

	t.Run("returns empty slice for empty IDs", func(t *testing.T) {
		groups, err := repo.GetGroupsByIDs(ctx, userID, []string{})

		require.NoError(t, err)
		assert.Len(t, groups, 0)
	})

	t.Run("returns groups ordered by name", func(t *testing.T) {
		groups, err := repo.GetGroupsByIDs(ctx, userID, groupIDs)

		require.NoError(t, err)
		for i := 0; i < len(groups)-1; i++ {
			assert.True(t, groups[i].Name <= groups[i+1].Name,
				"expected groups to be ordered by name")
		}
	})

	t.Run("does not return groups for different user", func(t *testing.T) {
		groups, err := repo.GetGroupsByIDs(ctx, "different-user", groupIDs)

		require.NoError(t, err)
		assert.Len(t, groups, 0)
	})
}

// =============================================================================
// UpdateGroup Tests
// =============================================================================

func TestGroupRepository_UpdateGroup(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupGroupTestDB(t)
	defer tdb.cleanup(t)

	logger := zap.NewNop()
	repo := NewGroupRepository(tdb.db, logger)
	ctx := context.Background()

	t.Run("updates name field", func(t *testing.T) {
		group := &models.Group{
			WAGroupID:  "update-name@g.us",
			Name:       "Original Name",
			InstanceID: "1",
		}
		created, err := repo.CreateGroup(ctx, "user-update-1", group)
		require.NoError(t, err)

		err = repo.UpdateGroup(ctx, created.ID, map[string]interface{}{
			"name": "Updated Name",
		})
		require.NoError(t, err)

		updated, err := repo.GetGroupByID(ctx, created.ID)
		require.NoError(t, err)
		assert.Equal(t, "Updated Name", updated.Name)
	})

	t.Run("updates members_count", func(t *testing.T) {
		group := &models.Group{
			WAGroupID:    "update-members@g.us",
			Name:         "Members Update",
			InstanceID:   "1",
			MembersCount: 50,
		}
		created, err := repo.CreateGroup(ctx, "user-update-2", group)
		require.NoError(t, err)

		err = repo.UpdateGroup(ctx, created.ID, map[string]interface{}{
			"members_count": 100,
		})
		require.NoError(t, err)

		updated, err := repo.GetGroupByID(ctx, created.ID)
		require.NoError(t, err)
		assert.Equal(t, 100, updated.MembersCount)
	})

	t.Run("updates is_admin field", func(t *testing.T) {
		group := &models.Group{
			WAGroupID:  "update-admin@g.us",
			Name:       "Admin Update",
			InstanceID: "1",
			IsAdmin:    false,
		}
		created, err := repo.CreateGroup(ctx, "user-update-3", group)
		require.NoError(t, err)

		err = repo.UpdateGroup(ctx, created.ID, map[string]interface{}{
			"is_admin": true,
		})
		require.NoError(t, err)

		updated, err := repo.GetGroupByID(ctx, created.ID)
		require.NoError(t, err)
		assert.True(t, updated.IsAdmin)
	})

	t.Run("does nothing with empty updates", func(t *testing.T) {
		group := &models.Group{
			WAGroupID:  "empty-update@g.us",
			Name:       "No Change",
			InstanceID: "1",
		}
		created, err := repo.CreateGroup(ctx, "user-update-4", group)
		require.NoError(t, err)

		err = repo.UpdateGroup(ctx, created.ID, map[string]interface{}{})
		require.NoError(t, err)

		updated, err := repo.GetGroupByID(ctx, created.ID)
		require.NoError(t, err)
		assert.Equal(t, "No Change", updated.Name)
	})
}

// =============================================================================
// SyncGroups Tests
// =============================================================================

func TestGroupRepository_SyncGroups(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupGroupTestDB(t)
	defer tdb.cleanup(t)

	logger := zap.NewNop()
	repo := NewGroupRepository(tdb.db, logger)
	ctx := context.Background()

	userID := "user-sync"
	instanceID := 1

	t.Run("syncs groups - inserts new groups", func(t *testing.T) {
		groups := []models.Group{
			{WAGroupID: "sync-1@g.us", Name: "Sync Group 1", MembersCount: 10},
			{WAGroupID: "sync-2@g.us", Name: "Sync Group 2", MembersCount: 20},
		}

		err := repo.SyncGroups(ctx, userID, instanceID, groups, repository.SyncOptions{})
		require.NoError(t, err)

		// Verify groups were created
		userGroups, err := repo.GetUserGroups(ctx, userID, false)
		require.NoError(t, err)
		assert.Len(t, userGroups, 2)
	})

	t.Run("syncs groups - updates existing groups", func(t *testing.T) {
		// Sync again with updated data
		groups := []models.Group{
			{WAGroupID: "sync-1@g.us", Name: "Sync Group 1 Updated", MembersCount: 15},
			{WAGroupID: "sync-2@g.us", Name: "Sync Group 2 Updated", MembersCount: 25},
		}

		err := repo.SyncGroups(ctx, userID, instanceID, groups, repository.SyncOptions{})
		require.NoError(t, err)

		// Verify groups were updated
		group1, err := repo.GetGroupByWAGroupID(ctx, userID, "sync-1@g.us")
		require.NoError(t, err)
		assert.Equal(t, "Sync Group 1 Updated", group1.Name)
		assert.Equal(t, 15, group1.MembersCount)
	})

	t.Run("syncs groups - removes missing groups", func(t *testing.T) {
		// Sync with only one group - the other should be deleted
		groups := []models.Group{
			{WAGroupID: "sync-1@g.us", Name: "Only This One", MembersCount: 10},
		}

		err := repo.SyncGroups(ctx, userID, instanceID, groups, repository.SyncOptions{})
		require.NoError(t, err)

		// Verify only one group remains
		userGroups, err := repo.GetUserGroups(ctx, userID, false)
		require.NoError(t, err)
		assert.Len(t, userGroups, 1)
		assert.Equal(t, "sync-1@g.us", userGroups[0].WAGroupID)
	})

	t.Run("syncs empty groups - does nothing", func(t *testing.T) {
		err := repo.SyncGroups(ctx, userID, instanceID, []models.Group{}, repository.SyncOptions{})
		require.NoError(t, err)
	})
}

// =============================================================================
// DeleteGroup Tests
// =============================================================================

func TestGroupRepository_DeleteGroup(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupGroupTestDB(t)
	defer tdb.cleanup(t)

	logger := zap.NewNop()
	repo := NewGroupRepository(tdb.db, logger)
	ctx := context.Background()

	t.Run("deletes group successfully", func(t *testing.T) {
		group := &models.Group{
			WAGroupID:  "to-delete@g.us",
			Name:       "To Delete",
			InstanceID: "1",
		}
		created, err := repo.CreateGroup(ctx, "user-delete", group)
		require.NoError(t, err)

		err = repo.DeleteGroup(ctx, created.ID, "user-delete")
		require.NoError(t, err)

		// Verify deletion
		found, err := repo.GetGroupByID(ctx, created.ID)
		require.NoError(t, err)
		assert.Nil(t, found)
	})

	t.Run("does not delete if user ID does not match", func(t *testing.T) {
		group := &models.Group{
			WAGroupID:  "protected-group@g.us",
			Name:       "Protected",
			InstanceID: "1",
		}
		created, err := repo.CreateGroup(ctx, "user-owner", group)
		require.NoError(t, err)

		// Try to delete with wrong user
		err = repo.DeleteGroup(ctx, created.ID, "wrong-user")
		require.Error(t, err) // Should error because group not found for that user

		// Verify it still exists
		found, err := repo.GetGroupByID(ctx, created.ID)
		require.NoError(t, err)
		assert.NotNil(t, found)
	})

	t.Run("returns error for non-existent group", func(t *testing.T) {
		err := repo.DeleteGroup(ctx, "00000000-0000-0000-0000-000000000000", "user-delete")
		require.Error(t, err)
	})
}

// =============================================================================
// DeleteGroupsByInstance Tests
// =============================================================================

func TestGroupRepository_DeleteGroupsByInstance(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupGroupTestDB(t)
	defer tdb.cleanup(t)

	logger := zap.NewNop()
	repo := NewGroupRepository(tdb.db, logger)
	ctx := context.Background()

	userID := "user-delete-instance"
	instanceID := 42

	// Create groups for the instance
	for i := 1; i <= 3; i++ {
		group := &models.Group{
			WAGroupID:  "instance-group-" + string(rune('0'+i)) + "@g.us",
			Name:       "Instance Group " + string(rune('0'+i)),
			InstanceID: "42",
		}
		_, err := repo.CreateGroup(ctx, userID, group)
		require.NoError(t, err)
	}

	// Create group for different instance
	otherGroup := &models.Group{
		WAGroupID:  "other-instance@g.us",
		Name:       "Other Instance Group",
		InstanceID: "99",
	}
	_, err := repo.CreateGroup(ctx, userID, otherGroup)
	require.NoError(t, err)

	t.Run("deletes all groups for instance", func(t *testing.T) {
		err := repo.DeleteGroupsByInstance(ctx, userID, instanceID)
		require.NoError(t, err)

		// Verify instance groups are deleted
		groups, err := repo.GetUserGroups(ctx, userID, false)
		require.NoError(t, err)

		// Only the other instance group should remain
		assert.Len(t, groups, 1)
		assert.Equal(t, "99", groups[0].InstanceID)
	})
}

// =============================================================================
// ToggleFavorite Tests
// =============================================================================

func TestGroupRepository_ToggleFavorite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupGroupTestDB(t)
	defer tdb.cleanup(t)

	logger := zap.NewNop()
	repo := NewGroupRepository(tdb.db, logger)
	ctx := context.Background()

	userID := "user-favorite"

	t.Run("sets favorite to true", func(t *testing.T) {
		group := &models.Group{
			WAGroupID:  "favorite-test@g.us",
			Name:       "Favorite Test",
			InstanceID: "1",
			IsFavorite: false,
		}
		created, err := repo.CreateGroup(ctx, userID, group)
		require.NoError(t, err)

		err = repo.ToggleFavorite(ctx, created.ID, userID, true)
		require.NoError(t, err)

		updated, err := repo.GetGroupByID(ctx, created.ID)
		require.NoError(t, err)
		assert.True(t, updated.IsFavorite)
	})

	t.Run("sets favorite to false", func(t *testing.T) {
		group := &models.Group{
			WAGroupID:  "unfavorite-test@g.us",
			Name:       "Unfavorite Test",
			InstanceID: "1",
			IsFavorite: true,
		}
		created, err := repo.CreateGroup(ctx, userID, group)
		require.NoError(t, err)

		err = repo.ToggleFavorite(ctx, created.ID, userID, false)
		require.NoError(t, err)

		updated, err := repo.GetGroupByID(ctx, created.ID)
		require.NoError(t, err)
		assert.False(t, updated.IsFavorite)
	})

	t.Run("returns error for wrong user", func(t *testing.T) {
		group := &models.Group{
			WAGroupID:  "protected-favorite@g.us",
			Name:       "Protected Favorite",
			InstanceID: "1",
		}
		created, err := repo.CreateGroup(ctx, userID, group)
		require.NoError(t, err)

		err = repo.ToggleFavorite(ctx, created.ID, "wrong-user", true)
		require.Error(t, err)
	})
}

// =============================================================================
// GetFavoriteGroups Tests
// =============================================================================

func TestGroupRepository_GetFavoriteGroups(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupGroupTestDB(t)
	defer tdb.cleanup(t)

	logger := zap.NewNop()
	repo := NewGroupRepository(tdb.db, logger)
	ctx := context.Background()

	userID := "user-get-favorites"

	// Create groups with different favorite status
	testGroups := []struct {
		waGroupID  string
		name       string
		isFavorite bool
	}{
		{"fav-1@g.us", "Favorite 1", true},
		{"not-fav-1@g.us", "Not Favorite 1", false},
		{"fav-2@g.us", "Favorite 2", true},
		{"not-fav-2@g.us", "Not Favorite 2", false},
	}

	for _, tg := range testGroups {
		group := &models.Group{
			WAGroupID:  tg.waGroupID,
			Name:       tg.name,
			InstanceID: "1",
			IsFavorite: tg.isFavorite,
		}
		_, err := repo.CreateGroup(ctx, userID, group)
		require.NoError(t, err)
	}

	t.Run("returns only favorite groups", func(t *testing.T) {
		groups, err := repo.GetFavoriteGroups(ctx, userID)

		require.NoError(t, err)
		assert.Len(t, groups, 2)

		for _, g := range groups {
			assert.True(t, g.IsFavorite)
		}
	})

	t.Run("returns groups ordered by name", func(t *testing.T) {
		groups, err := repo.GetFavoriteGroups(ctx, userID)

		require.NoError(t, err)
		assert.Equal(t, "Favorite 1", groups[0].Name)
		assert.Equal(t, "Favorite 2", groups[1].Name)
	})

	t.Run("returns empty slice when no favorites", func(t *testing.T) {
		groups, err := repo.GetFavoriteGroups(ctx, "user-no-favorites")

		require.NoError(t, err)
		assert.Len(t, groups, 0)
	})
}

// =============================================================================
// IsUserAdminInGroup Tests
// =============================================================================

func TestGroupRepository_IsUserAdminInGroup(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupGroupTestDB(t)
	defer tdb.cleanup(t)

	logger := zap.NewNop()
	repo := NewGroupRepository(tdb.db, logger)
	ctx := context.Background()

	userID := "user-admin-check"

	// Create admin group
	adminGroup := &models.Group{
		WAGroupID:  "admin-group@g.us",
		Name:       "Admin Group",
		InstanceID: "1",
		IsAdmin:    true,
	}
	_, err := repo.CreateGroup(ctx, userID, adminGroup)
	require.NoError(t, err)

	// Create non-admin group
	nonAdminGroup := &models.Group{
		WAGroupID:  "non-admin-group@g.us",
		Name:       "Non Admin Group",
		InstanceID: "1",
		IsAdmin:    false,
	}
	_, err = repo.CreateGroup(ctx, userID, nonAdminGroup)
	require.NoError(t, err)

	t.Run("returns true for admin group", func(t *testing.T) {
		isAdmin, err := repo.IsUserAdminInGroup(ctx, userID, "admin-group@g.us")

		require.NoError(t, err)
		assert.True(t, isAdmin)
	})

	t.Run("returns false for non-admin group", func(t *testing.T) {
		isAdmin, err := repo.IsUserAdminInGroup(ctx, userID, "non-admin-group@g.us")

		require.NoError(t, err)
		assert.False(t, isAdmin)
	})

	t.Run("returns false for non-existent group", func(t *testing.T) {
		isAdmin, err := repo.IsUserAdminInGroup(ctx, userID, "non-existent@g.us")

		require.NoError(t, err)
		assert.False(t, isAdmin)
	})
}

// =============================================================================
// BulkUpdateGroups Tests
// =============================================================================

func TestGroupRepository_BulkUpdateGroups(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupGroupTestDB(t)
	defer tdb.cleanup(t)

	logger := zap.NewNop()
	repo := NewGroupRepository(tdb.db, logger)
	ctx := context.Background()

	userID := "user-bulk-update"

	// Create test groups
	var groupIDs []string
	for i := 1; i <= 3; i++ {
		group := &models.Group{
			WAGroupID:  "bulk-" + string(rune('0'+i)) + "@g.us",
			Name:       "Bulk Group " + string(rune('0'+i)),
			InstanceID: "1",
			IsFavorite: false,
		}
		created, err := repo.CreateGroup(ctx, userID, group)
		require.NoError(t, err)
		groupIDs = append(groupIDs, created.ID)
	}

	t.Run("updates multiple groups", func(t *testing.T) {
		affected, err := repo.BulkUpdateGroups(ctx, userID, groupIDs, map[string]interface{}{
			"is_favorite": true,
		})

		require.NoError(t, err)
		assert.Equal(t, int64(3), affected)

		// Verify all groups are updated
		for _, id := range groupIDs {
			group, err := repo.GetGroupByID(ctx, id)
			require.NoError(t, err)
			assert.True(t, group.IsFavorite)
		}
	})

	t.Run("returns 0 for empty group IDs", func(t *testing.T) {
		affected, err := repo.BulkUpdateGroups(ctx, userID, []string{}, map[string]interface{}{
			"is_favorite": true,
		})

		require.NoError(t, err)
		assert.Equal(t, int64(0), affected)
	})

	t.Run("returns 0 for empty updates", func(t *testing.T) {
		affected, err := repo.BulkUpdateGroups(ctx, userID, groupIDs, map[string]interface{}{})

		require.NoError(t, err)
		assert.Equal(t, int64(0), affected)
	})
}

// =============================================================================
// Edge Cases Tests
// =============================================================================

func TestGroupRepository_EdgeCases(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupGroupTestDB(t)
	defer tdb.cleanup(t)

	logger := zap.NewNop()
	repo := NewGroupRepository(tdb.db, logger)
	ctx := context.Background()

	t.Run("handles special characters in name", func(t *testing.T) {
		group := &models.Group{
			WAGroupID:  "special-name@g.us",
			Name:       "Test 'quotes' \"double\" & special <chars>",
			InstanceID: "1",
		}

		created, err := repo.CreateGroup(ctx, "user-special", group)
		require.NoError(t, err)

		found, err := repo.GetGroupByID(ctx, created.ID)
		require.NoError(t, err)
		assert.Equal(t, group.Name, found.Name)
	})

	t.Run("handles unicode in name and description", func(t *testing.T) {
		description := "Grupo de Promocoes e Ofertas"
		group := &models.Group{
			WAGroupID:   "unicode-group@g.us",
			Name:        "Promocoes Brasil",
			Description: &description,
			InstanceID:  "1",
		}

		created, err := repo.CreateGroup(ctx, "user-unicode", group)
		require.NoError(t, err)

		found, err := repo.GetGroupByID(ctx, created.ID)
		require.NoError(t, err)
		assert.Equal(t, group.Name, found.Name)
		assert.Equal(t, description, *found.Description)
	})

	t.Run("handles very long description", func(t *testing.T) {
		longDesc := make([]byte, 5000)
		for i := range longDesc {
			longDesc[i] = 'a'
		}
		description := string(longDesc)

		group := &models.Group{
			WAGroupID:   "long-desc@g.us",
			Name:        "Long Description Group",
			Description: &description,
			InstanceID:  "1",
		}

		created, err := repo.CreateGroup(ctx, "user-long-desc", group)
		require.NoError(t, err)

		found, err := repo.GetGroupByID(ctx, created.ID)
		require.NoError(t, err)
		assert.Equal(t, 5000, len(*found.Description))
	})

	t.Run("handles large member counts", func(t *testing.T) {
		group := &models.Group{
			WAGroupID:    "large-members@g.us",
			Name:         "Large Group",
			InstanceID:   "1",
			MembersCount: 100000,
			MemberLimit:  1000000,
		}

		created, err := repo.CreateGroup(ctx, "user-large", group)
		require.NoError(t, err)

		found, err := repo.GetGroupByID(ctx, created.ID)
		require.NoError(t, err)
		assert.Equal(t, 100000, found.MembersCount)
		assert.Equal(t, 1000000, found.MemberLimit)
	})
}
