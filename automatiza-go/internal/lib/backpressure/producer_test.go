package backpressure

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// MockQueueInspector implements QueueInspector for testing.
type MockQueueInspector struct {
	depth int
	err   error
}

func (m *MockQueueInspector) GetQueueDepth(ctx context.Context) (int, error) {
	return m.depth, m.err
}

// =============================================================================
// Global Capacity Tests
// =============================================================================

func TestProducerBackpressure_GlobalCapacity_AllowsWhenUnderLimit(t *testing.T) {
	logger := zap.NewNop()
	bp := NewProducerBackpressure(Config{
		Enabled:           true,
		GlobalMaxEnqueued: 1000,
		UserMaxEnqueued:   100,
	}, nil, logger)

	inspector := &MockQueueInspector{depth: 500}
	bp.SetQueueInspector(inspector)

	canEnqueue, reason, err := bp.CanEnqueue(context.Background(), "user-123")

	require.NoError(t, err)
	assert.True(t, canEnqueue)
	assert.Empty(t, reason)
}

func TestProducerBackpressure_GlobalCapacity_BlocksWhenAtLimit(t *testing.T) {
	logger := zap.NewNop()
	bp := NewProducerBackpressure(Config{
		Enabled:           true,
		GlobalMaxEnqueued: 1000,
		UserMaxEnqueued:   100,
	}, nil, logger)

	inspector := &MockQueueInspector{depth: 1000}
	bp.SetQueueInspector(inspector)

	canEnqueue, reason, err := bp.CanEnqueue(context.Background(), "user-123")

	require.NoError(t, err)
	assert.False(t, canEnqueue)
	assert.Contains(t, reason, "Sistema ocupado")
	assert.Contains(t, reason, "1000/1000")
}

func TestProducerBackpressure_GlobalCapacity_BlocksWhenOverLimit(t *testing.T) {
	logger := zap.NewNop()
	bp := NewProducerBackpressure(Config{
		Enabled:           true,
		GlobalMaxEnqueued: 1000,
		UserMaxEnqueued:   100,
	}, nil, logger)

	inspector := &MockQueueInspector{depth: 1500}
	bp.SetQueueInspector(inspector)

	canEnqueue, reason, err := bp.CanEnqueue(context.Background(), "user-123")

	require.NoError(t, err)
	assert.False(t, canEnqueue)
	assert.Contains(t, reason, "Sistema ocupado")
}

func TestProducerBackpressure_GlobalCapacity_AllowsWhenDisabled(t *testing.T) {
	logger := zap.NewNop()
	bp := NewProducerBackpressure(Config{
		Enabled:           true,
		GlobalMaxEnqueued: 0, // Disabled
		UserMaxEnqueued:   100,
	}, nil, logger)

	inspector := &MockQueueInspector{depth: 999999}
	bp.SetQueueInspector(inspector)

	canEnqueue, _, err := bp.CanEnqueue(context.Background(), "user-123")

	require.NoError(t, err)
	assert.True(t, canEnqueue)
}

func TestProducerBackpressure_GlobalCapacity_AllowsWhenNoInspector(t *testing.T) {
	logger := zap.NewNop()
	bp := NewProducerBackpressure(Config{
		Enabled:           true,
		GlobalMaxEnqueued: 1000,
		UserMaxEnqueued:   100,
	}, nil, logger)
	// No inspector set

	canEnqueue, _, err := bp.CanEnqueue(context.Background(), "user-123")

	require.NoError(t, err)
	assert.True(t, canEnqueue)
}

func TestProducerBackpressure_GlobalCapacity_AllowsOnInspectorError(t *testing.T) {
	logger := zap.NewNop()
	bp := NewProducerBackpressure(Config{
		Enabled:           true,
		GlobalMaxEnqueued: 1000,
		UserMaxEnqueued:   100,
	}, nil, logger)

	inspector := &MockQueueInspector{
		depth: 0,
		err:   errors.New("redis connection error"),
	}
	bp.SetQueueInspector(inspector)

	canEnqueue, _, err := bp.CanEnqueue(context.Background(), "user-123")

	require.NoError(t, err)
	assert.True(t, canEnqueue) // Fails open
}

func TestProducerBackpressure_GlobalCapacity_ChecksBeforeUserLimit(t *testing.T) {
	logger := zap.NewNop()
	bp := NewProducerBackpressure(Config{
		Enabled:           true,
		GlobalMaxEnqueued: 100,
		UserMaxEnqueued:   1000, // User has high limit
	}, nil, logger)

	inspector := &MockQueueInspector{depth: 150}
	bp.SetQueueInspector(inspector)

	canEnqueue, reason, err := bp.CanEnqueue(context.Background(), "user-123")

	require.NoError(t, err)
	assert.False(t, canEnqueue)
	assert.Contains(t, reason, "Sistema ocupado") // Global limit message, not user limit
}

// =============================================================================
// User Limit Tests (existing behavior preserved)
// =============================================================================

func TestProducerBackpressure_UserLimit_AllowsWhenUnderLimit(t *testing.T) {
	logger := zap.NewNop()
	bp := NewProducerBackpressure(Config{
		Enabled:           true,
		GlobalMaxEnqueued: 0, // Disabled
		UserMaxEnqueued:   100,
	}, nil, logger)

	canEnqueue, reason, err := bp.CanEnqueue(context.Background(), "user-123")

	require.NoError(t, err)
	assert.True(t, canEnqueue)
	assert.Empty(t, reason)
}

func TestProducerBackpressure_UserLimit_BlocksWhenAtLimit(t *testing.T) {
	bp, _ := newTestBP(t, Config{
		Enabled:           true,
		GlobalMaxEnqueued: 0,
		UserMaxEnqueued:   5,
		WindowMinutes:     10,
	})

	ctx := context.Background()

	// Record 5 enqueues (at limit)
	for i := 0; i < 5; i++ {
		err := bp.RecordEnqueue(ctx, "user-123")
		require.NoError(t, err)
	}

	canEnqueue, reason, err := bp.CanEnqueue(ctx, "user-123")

	require.NoError(t, err)
	assert.False(t, canEnqueue)
	assert.Contains(t, reason, "Limite de 5 mensagens")
}

func TestProducerBackpressure_Disabled_AlwaysAllows(t *testing.T) {
	logger := zap.NewNop()
	bp := NewProducerBackpressure(Config{
		Enabled:           false,
		GlobalMaxEnqueued: 1,
		UserMaxEnqueued:   1,
	}, nil, logger)

	inspector := &MockQueueInspector{depth: 999999}
	bp.SetQueueInspector(inspector)

	ctx := context.Background()

	// Even with limits set to 1, should allow when disabled
	canEnqueue, _, err := bp.CanEnqueue(ctx, "user-123")
	require.NoError(t, err)
	assert.True(t, canEnqueue)
}

// =============================================================================
// GetGlobalQueueDepth Tests
// =============================================================================

func TestProducerBackpressure_GetGlobalQueueDepth_ReturnsDepth(t *testing.T) {
	logger := zap.NewNop()
	bp := NewProducerBackpressure(Config{
		Enabled:           true,
		GlobalMaxEnqueued: 1000,
	}, nil, logger)

	inspector := &MockQueueInspector{depth: 456}
	bp.SetQueueInspector(inspector)

	depth, err := bp.GetGlobalQueueDepth(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 456, depth)
}

func TestProducerBackpressure_GetGlobalQueueDepth_ReturnsZeroWithoutInspector(t *testing.T) {
	logger := zap.NewNop()
	bp := NewProducerBackpressure(Config{
		Enabled: true,
	}, nil, logger)

	depth, err := bp.GetGlobalQueueDepth(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 0, depth)
}

func TestProducerBackpressure_GetGlobalQueueDepth_PropagatesError(t *testing.T) {
	logger := zap.NewNop()
	bp := NewProducerBackpressure(Config{
		Enabled: true,
	}, nil, logger)

	inspector := &MockQueueInspector{err: errors.New("connection error")}
	bp.SetQueueInspector(inspector)

	_, err := bp.GetGlobalQueueDepth(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "connection error")
}

// =============================================================================
// GetDelay Tests
// =============================================================================

func TestProducerBackpressure_GetDelay_ReturnsDelayForHighDepth(t *testing.T) {
	logger := zap.NewNop()
	bp := NewProducerBackpressure(Config{
		Enabled: true,
		DelayMS: 10000,
	}, nil, logger)

	tests := []struct {
		name       string
		depth      int
		wantDelay  bool
		minDelayMS int
	}{
		{"low depth", 10, false, 0},
		{"medium depth", 75, true, 5000},
		{"high depth", 150, true, 10000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			delay := bp.GetDelay(context.Background(), tt.depth)

			if tt.wantDelay {
				assert.GreaterOrEqual(t, delay.Milliseconds(), int64(tt.minDelayMS))
			} else {
				assert.Equal(t, int64(0), delay.Milliseconds())
			}
		})
	}
}

func TestProducerBackpressure_GetDelay_ReturnsZeroWhenDisabled(t *testing.T) {
	logger := zap.NewNop()
	bp := NewProducerBackpressure(Config{
		Enabled: false,
		DelayMS: 10000,
	}, nil, logger)

	delay := bp.GetDelay(context.Background(), 1000)

	assert.Equal(t, int64(0), delay.Milliseconds())
}

// =============================================================================
// Config Tests
// =============================================================================

func TestProducerBackpressure_DefaultConfig(t *testing.T) {
	logger := zap.NewNop()
	bp := NewProducerBackpressure(Config{
		Enabled: true,
		// All other values are zero
	}, nil, logger)

	// Check defaults were applied
	assert.Equal(t, 200, bp.userMaxEnqueued)
	assert.Equal(t, 10, bp.windowMinutes)
	assert.Equal(t, 15000, bp.delayMS)
	assert.Equal(t, 0, bp.globalMaxEnqueued) // Not defaulted
}

func TestProducerBackpressure_GetGlobalMaxEnqueued(t *testing.T) {
	logger := zap.NewNop()
	bp := NewProducerBackpressure(Config{
		Enabled:           true,
		GlobalMaxEnqueued: 5000,
	}, nil, logger)

	assert.Equal(t, 5000, bp.GetGlobalMaxEnqueued())
}
