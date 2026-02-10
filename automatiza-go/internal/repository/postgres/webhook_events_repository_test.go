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

// testDB holds the test database and container
type testDB struct {
	container testcontainers.Container
	pool      *pgxpool.Pool
	db        *DB
}

// setupTestDB creates a PostgreSQL container and initializes the schema
func setupTestDB(t *testing.T) *testDB {
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
	_, err = pool.Exec(ctx, webhookEventsSchema)
	require.NoError(t, err, "failed to create schema")

	logger := zap.NewNop()

	return &testDB{
		container: pgContainer,
		pool:      pool,
		db:        &DB{pool: pool, logger: logger},
	}
}

// cleanup closes the database connection and terminates the container
func (tdb *testDB) cleanup(t *testing.T) {
	t.Helper()
	ctx := context.Background()

	if tdb.pool != nil {
		tdb.pool.Close()
	}
	if tdb.container != nil {
		require.NoError(t, tdb.container.Terminate(ctx), "failed to terminate container")
	}
}

// Schema for webhook_events table (matches migration 003)
const webhookEventsSchema = `
CREATE TABLE IF NOT EXISTS webhook_events (
    id SERIAL PRIMARY KEY,
    event_id VARCHAR(26) NOT NULL,
    session_name VARCHAR(128) NOT NULL,
    event_type VARCHAR(50) NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ,
    status VARCHAR(20) NOT NULL DEFAULT 'received'
        CHECK (status IN ('received', 'processing', 'processed', 'failed', 'dlq')),
    error_message TEXT,
    retry_count INTEGER NOT NULL DEFAULT 0,
    CONSTRAINT uq_webhook_events_event_id UNIQUE (event_id)
);
`

// =============================================================================
// Tests
// =============================================================================

func TestWebhookEventsRepository_InsertIfNotExists(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupTestDB(t)
	defer tdb.cleanup(t)

	logger := zap.NewNop()
	repo := NewWebhookEventsRepository(tdb.db, logger)
	ctx := context.Background()

	t.Run("inserts new event and returns true", func(t *testing.T) {
		inserted, err := repo.InsertIfNotExists(ctx, "event-001", "session-1", "message")
		require.NoError(t, err)
		assert.True(t, inserted, "expected event to be inserted")

		// Verify event exists
		exists, err := repo.Exists(ctx, "event-001")
		require.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("returns false for duplicate event", func(t *testing.T) {
		// Insert first
		inserted, err := repo.InsertIfNotExists(ctx, "event-002", "session-1", "message")
		require.NoError(t, err)
		assert.True(t, inserted)

		// Try to insert duplicate
		inserted, err = repo.InsertIfNotExists(ctx, "event-002", "session-1", "message")
		require.NoError(t, err)
		assert.False(t, inserted, "expected duplicate to not be inserted")
	})

	t.Run("sets initial status to received", func(t *testing.T) {
		_, err := repo.InsertIfNotExists(ctx, "event-003", "session-1", "message.ack")
		require.NoError(t, err)

		// Query the status directly
		var status string
		err = tdb.pool.QueryRow(ctx, "SELECT status FROM webhook_events WHERE event_id = $1", "event-003").Scan(&status)
		require.NoError(t, err)
		assert.Equal(t, "received", status)
	})
}

func TestWebhookEventsRepository_Exists(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupTestDB(t)
	defer tdb.cleanup(t)

	logger := zap.NewNop()
	repo := NewWebhookEventsRepository(tdb.db, logger)
	ctx := context.Background()

	t.Run("returns false for non-existent event", func(t *testing.T) {
		exists, err := repo.Exists(ctx, "non-existent")
		require.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("returns true for existing event", func(t *testing.T) {
		_, err := repo.InsertIfNotExists(ctx, "exists-test", "session-1", "message")
		require.NoError(t, err)

		exists, err := repo.Exists(ctx, "exists-test")
		require.NoError(t, err)
		assert.True(t, exists)
	})
}

func TestWebhookEventsRepository_StatusTransitions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupTestDB(t)
	defer tdb.cleanup(t)

	logger := zap.NewNop()
	repo := NewWebhookEventsRepository(tdb.db, logger)
	ctx := context.Background()

	t.Run("MarkProcessing transitions to processing", func(t *testing.T) {
		eventID := "status-processing-test"
		_, err := repo.InsertIfNotExists(ctx, eventID, "session-1", "message")
		require.NoError(t, err)

		err = repo.MarkProcessing(ctx, eventID)
		require.NoError(t, err)

		var status string
		err = tdb.pool.QueryRow(ctx, "SELECT status FROM webhook_events WHERE event_id = $1", eventID).Scan(&status)
		require.NoError(t, err)
		assert.Equal(t, "processing", status)
	})

	t.Run("MarkProcessed transitions to processed and sets timestamp", func(t *testing.T) {
		eventID := "status-processed-test"
		_, err := repo.InsertIfNotExists(ctx, eventID, "session-1", "message")
		require.NoError(t, err)

		beforeProcess := time.Now().UTC().Add(-time.Second) // Allow 1 second tolerance
		err = repo.MarkProcessed(ctx, eventID)
		require.NoError(t, err)

		var status string
		var processedAt time.Time
		err = tdb.pool.QueryRow(ctx, "SELECT status, processed_at FROM webhook_events WHERE event_id = $1", eventID).Scan(&status, &processedAt)
		require.NoError(t, err)
		assert.Equal(t, "processed", status)
		assert.True(t, processedAt.After(beforeProcess), "processed_at should be after beforeProcess")
		assert.False(t, processedAt.IsZero(), "processed_at should not be zero")
	})

	t.Run("MarkFailed transitions to failed and sets error message", func(t *testing.T) {
		eventID := "status-failed-test"
		_, err := repo.InsertIfNotExists(ctx, eventID, "session-1", "message")
		require.NoError(t, err)

		errorMsg := "some error occurred"
		err = repo.MarkFailed(ctx, eventID, errorMsg)
		require.NoError(t, err)

		var status, errMessage string
		var retryCount int
		err = tdb.pool.QueryRow(ctx, "SELECT status, error_message, retry_count FROM webhook_events WHERE event_id = $1", eventID).Scan(&status, &errMessage, &retryCount)
		require.NoError(t, err)
		assert.Equal(t, "failed", status)
		assert.Equal(t, errorMsg, errMessage)
		assert.Equal(t, 1, retryCount, "retry count should be incremented")
	})

	t.Run("MarkDLQ transitions to dlq status", func(t *testing.T) {
		eventID := "status-dlq-test"
		_, err := repo.InsertIfNotExists(ctx, eventID, "session-1", "message")
		require.NoError(t, err)

		dlqMsg := "max retries exceeded"
		err = repo.MarkDLQ(ctx, eventID, dlqMsg)
		require.NoError(t, err)

		var status, errMessage string
		err = tdb.pool.QueryRow(ctx, "SELECT status, error_message FROM webhook_events WHERE event_id = $1", eventID).Scan(&status, &errMessage)
		require.NoError(t, err)
		assert.Equal(t, "dlq", status)
		assert.Equal(t, dlqMsg, errMessage)
	})
}

func TestWebhookEventsRepository_IncrementRetryCount(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupTestDB(t)
	defer tdb.cleanup(t)

	logger := zap.NewNop()
	repo := NewWebhookEventsRepository(tdb.db, logger)
	ctx := context.Background()

	t.Run("increments retry count", func(t *testing.T) {
		eventID := "retry-test"
		_, err := repo.InsertIfNotExists(ctx, eventID, "session-1", "message")
		require.NoError(t, err)

		// Increment 3 times
		for i := 0; i < 3; i++ {
			err = repo.IncrementRetryCount(ctx, eventID)
			require.NoError(t, err)
		}

		var retryCount int
		err = tdb.pool.QueryRow(ctx, "SELECT retry_count FROM webhook_events WHERE event_id = $1", eventID).Scan(&retryCount)
		require.NoError(t, err)
		assert.Equal(t, 3, retryCount)
	})

	t.Run("does not error on non-existent event", func(t *testing.T) {
		err := repo.IncrementRetryCount(ctx, "non-existent")
		require.NoError(t, err)
	})
}

func TestWebhookEventsRepository_GetPendingEvents(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupTestDB(t)
	defer tdb.cleanup(t)

	logger := zap.NewNop()
	repo := NewWebhookEventsRepository(tdb.db, logger)
	ctx := context.Background()

	// Insert events with different statuses
	_, _ = repo.InsertIfNotExists(ctx, "pending-1", "session-1", "message")
	_, _ = repo.InsertIfNotExists(ctx, "pending-2", "session-1", "message")
	_, _ = repo.InsertIfNotExists(ctx, "pending-3", "session-2", "message.ack")

	// Mark one as processed (should NOT appear in pending)
	_ = repo.MarkProcessed(ctx, "pending-2")

	// Mark one as failed (should appear in pending - for retry)
	_ = repo.MarkFailed(ctx, "pending-3", "some error")

	t.Run("returns received and failed events", func(t *testing.T) {
		events, err := repo.GetPendingEvents(ctx, 10)
		require.NoError(t, err)
		assert.Len(t, events, 2, "expected 2 pending events")

		// Verify event IDs
		eventIDs := make(map[string]bool)
		for _, e := range events {
			eventIDs[e.EventID] = true
		}
		assert.True(t, eventIDs["pending-1"], "pending-1 should be in results")
		assert.True(t, eventIDs["pending-3"], "pending-3 (failed) should be in results")
		assert.False(t, eventIDs["pending-2"], "pending-2 (processed) should NOT be in results")
	})

	t.Run("respects limit", func(t *testing.T) {
		events, err := repo.GetPendingEvents(ctx, 1)
		require.NoError(t, err)
		assert.Len(t, events, 1)
	})

	t.Run("uses default limit when 0", func(t *testing.T) {
		events, err := repo.GetPendingEvents(ctx, 0)
		require.NoError(t, err)
		assert.LessOrEqual(t, len(events), 100)
	})
}

func TestWebhookEventsRepository_GetFailedEvents(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupTestDB(t)
	defer tdb.cleanup(t)

	logger := zap.NewNop()
	repo := NewWebhookEventsRepository(tdb.db, logger)
	ctx := context.Background()

	// Insert events with different statuses
	_, _ = repo.InsertIfNotExists(ctx, "failed-1", "session-1", "message")
	_, _ = repo.InsertIfNotExists(ctx, "failed-2", "session-1", "message")
	_, _ = repo.InsertIfNotExists(ctx, "failed-3", "session-2", "message.ack")

	// Mark one as failed
	_ = repo.MarkFailed(ctx, "failed-1", "error 1")
	_ = repo.MarkFailed(ctx, "failed-3", "error 3")

	t.Run("returns only failed events", func(t *testing.T) {
		events, err := repo.GetFailedEvents(ctx, 10)
		require.NoError(t, err)
		assert.Len(t, events, 2)

		for _, e := range events {
			assert.Equal(t, models.WebhookEventRecordStatusFailed, e.Status)
		}
	})
}

func TestWebhookEventsRepository_Cleanup(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupTestDB(t)
	defer tdb.cleanup(t)

	logger := zap.NewNop()
	repo := NewWebhookEventsRepository(tdb.db, logger)
	ctx := context.Background()

	// Insert and process events
	_, _ = repo.InsertIfNotExists(ctx, "cleanup-1", "session-1", "message")
	_, _ = repo.InsertIfNotExists(ctx, "cleanup-2", "session-1", "message")
	_, _ = repo.InsertIfNotExists(ctx, "cleanup-3", "session-1", "message")

	_ = repo.MarkProcessed(ctx, "cleanup-1")
	_ = repo.MarkProcessed(ctx, "cleanup-2")
	// cleanup-3 stays as "received"

	// Update one processed event to be old
	_, err := tdb.pool.Exec(ctx, "UPDATE webhook_events SET processed_at = NOW() - INTERVAL '2 days' WHERE event_id = $1", "cleanup-1")
	require.NoError(t, err)

	t.Run("deletes old processed events", func(t *testing.T) {
		deleted, err := repo.Cleanup(ctx, 24*time.Hour)
		require.NoError(t, err)
		assert.Equal(t, int64(1), deleted)

		// Verify cleanup-1 is gone
		exists, _ := repo.Exists(ctx, "cleanup-1")
		assert.False(t, exists)

		// Verify cleanup-2 still exists (not old enough)
		exists, _ = repo.Exists(ctx, "cleanup-2")
		assert.True(t, exists)

		// Verify cleanup-3 still exists (not processed)
		exists, _ = repo.Exists(ctx, "cleanup-3")
		assert.True(t, exists)
	})
}

func TestWebhookEventsRepository_ConcurrentInserts(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tdb := setupTestDB(t)
	defer tdb.cleanup(t)

	logger := zap.NewNop()
	repo := NewWebhookEventsRepository(tdb.db, logger)
	ctx := context.Background()

	t.Run("handles concurrent duplicate inserts correctly", func(t *testing.T) {
		eventID := "concurrent-test"
		const numGoroutines = 10

		results := make(chan bool, numGoroutines)
		errors := make(chan error, numGoroutines)

		// Launch concurrent inserts
		for i := 0; i < numGoroutines; i++ {
			go func() {
				inserted, err := repo.InsertIfNotExists(ctx, eventID, "session-1", "message")
				if err != nil {
					errors <- err
					return
				}
				results <- inserted
			}()
		}

		// Collect results
		insertedCount := 0
		for i := 0; i < numGoroutines; i++ {
			select {
			case err := <-errors:
				t.Fatalf("unexpected error: %v", err)
			case inserted := <-results:
				if inserted {
					insertedCount++
				}
			case <-time.After(10 * time.Second):
				t.Fatal("timeout waiting for results")
			}
		}

		// Exactly one insert should succeed
		assert.Equal(t, 1, insertedCount, "expected exactly one successful insert")

		// Verify only one row exists
		var count int
		err := tdb.pool.QueryRow(ctx, "SELECT COUNT(*) FROM webhook_events WHERE event_id = $1", eventID).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})
}
