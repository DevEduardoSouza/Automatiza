package backpressure

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// AsynqQueueInspector — Constructor
// =============================================================================

func TestNewAsynqQueueInspector_ReturnsNonNil(t *testing.T) {
	t.Parallel()
	mr := miniredis.RunT(t)

	inspector := asynq.NewInspector(asynq.RedisClientOpt{Addr: mr.Addr()})
	defer inspector.Close()

	aqi := NewAsynqQueueInspector(inspector, "default")
	assert.NotNil(t, aqi)
	assert.Equal(t, "default", aqi.queueName)
}

func TestNewAsynqQueueInspector_PreservesQueueName(t *testing.T) {
	t.Parallel()
	mr := miniredis.RunT(t)

	inspector := asynq.NewInspector(asynq.RedisClientOpt{Addr: mr.Addr()})
	defer inspector.Close()

	aqi := NewAsynqQueueInspector(inspector, "custom-queue")
	assert.Equal(t, "custom-queue", aqi.queueName)
}

// =============================================================================
// AsynqQueueInspector — GetQueueDepth
// =============================================================================

func TestAsynqQueueInspector_GetQueueDepth_EmptyQueue(t *testing.T) {
	t.Parallel()
	mr := miniredis.RunT(t)

	client := asynq.NewClient(asynq.RedisClientOpt{Addr: mr.Addr()})
	defer client.Close()

	// Enqueue one task to create queue metadata, then delete it
	task := asynq.NewTask("test:setup", nil)
	info, err := client.Enqueue(task, asynq.Queue("default"))
	require.NoError(t, err)

	inspector := asynq.NewInspector(asynq.RedisClientOpt{Addr: mr.Addr()})
	defer inspector.Close()

	// Delete the task to leave queue empty but metadata intact
	require.NoError(t, inspector.DeleteTask("default", info.ID))

	aqi := NewAsynqQueueInspector(inspector, "default")
	depth, err := aqi.GetQueueDepth(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, depth)
}

func TestAsynqQueueInspector_GetQueueDepth_WithPendingTasks(t *testing.T) {
	t.Parallel()
	mr := miniredis.RunT(t)

	client := asynq.NewClient(asynq.RedisClientOpt{Addr: mr.Addr()})
	defer client.Close()

	// Enqueue some tasks (they'll be pending since no server is processing)
	for i := 0; i < 5; i++ {
		task := asynq.NewTask("test:task", nil)
		_, err := client.Enqueue(task, asynq.Queue("messages"))
		require.NoError(t, err)
	}

	inspector := asynq.NewInspector(asynq.RedisClientOpt{Addr: mr.Addr()})
	defer inspector.Close()

	aqi := NewAsynqQueueInspector(inspector, "messages")
	depth, err := aqi.GetQueueDepth(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 5, depth)
}

func TestAsynqQueueInspector_GetQueueDepth_NonExistentQueue_ReturnsError(t *testing.T) {
	t.Parallel()
	mr := miniredis.RunT(t)

	inspector := asynq.NewInspector(asynq.RedisClientOpt{Addr: mr.Addr()})
	defer inspector.Close()

	aqi := NewAsynqQueueInspector(inspector, "nonexistent-queue")
	_, err := aqi.GetQueueDepth(context.Background())
	assert.Error(t, err)
}

func TestAsynqQueueInspector_GetQueueDepth_RedisDown_ReturnsError(t *testing.T) {
	t.Parallel()
	mr := miniredis.RunT(t)
	addr := mr.Addr()
	mr.Close()

	inspector := asynq.NewInspector(asynq.RedisClientOpt{Addr: addr})
	defer inspector.Close()

	aqi := NewAsynqQueueInspector(inspector, "default")
	_, err := aqi.GetQueueDepth(context.Background())
	assert.Error(t, err)
}

// =============================================================================
// AsynqQueueInspector — Interface Compliance
// =============================================================================

func TestAsynqQueueInspector_ImplementsQueueInspector(t *testing.T) {
	t.Parallel()
	// This is enforced at compile time via var _ QueueInspector = (*AsynqQueueInspector)(nil)
	// but we verify it in a test for documentation purposes
	var _ QueueInspector = (*AsynqQueueInspector)(nil)
}
