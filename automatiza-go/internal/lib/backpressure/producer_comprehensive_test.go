package backpressure

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"automatiza-go/internal/lib/cache"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// =============================================================================
// Test Helpers
// =============================================================================

// newTestMiniRedis creates a miniredis server + go-redis client for testing.
func newTestMiniRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	t.Cleanup(func() { client.Close() })
	return mr, client
}

// newTestCache creates a Cache backed by miniredis for integration testing.
func newTestCache(t *testing.T) (*miniredis.Miniredis, *cache.Cache) {
	t.Helper()
	mr, client := newTestMiniRedis(t)
	c := cache.NewCache(client, "test", false)
	return mr, c
}

// newTestBP creates a ProducerBackpressure with miniredis-backed cache.
func newTestBP(t *testing.T, cfg Config) (*ProducerBackpressure, *miniredis.Miniredis) {
	t.Helper()
	mr, c := newTestCache(t)
	logger := zap.NewNop()
	bp := NewProducerBackpressure(cfg, c, logger)
	return bp, mr
}

// newLocalOnlyBP creates a ProducerBackpressure without cache (local-only mode).
func newLocalOnlyBP(cfg Config) *ProducerBackpressure {
	return NewProducerBackpressure(cfg, nil, zap.NewNop())
}

// =============================================================================
// NewProducerBackpressure — Constructor & Config
// =============================================================================

func TestNewProducerBackpressure_CustomConfigPreservesValues(t *testing.T) {
	t.Parallel()
	bp := newLocalOnlyBP(Config{
		Enabled:           true,
		DelayMS:           5000,
		UserMaxEnqueued:   50,
		GlobalMaxEnqueued: 2000,
		WindowMinutes:     30,
	})

	assert.Equal(t, 5000, bp.delayMS)
	assert.Equal(t, 50, bp.userMaxEnqueued)
	assert.Equal(t, 2000, bp.globalMaxEnqueued)
	assert.Equal(t, 30, bp.windowMinutes)
	assert.True(t, bp.enabled)
}

func TestNewProducerBackpressure_NegativeValuesGetDefaults(t *testing.T) {
	t.Parallel()
	bp := newLocalOnlyBP(Config{
		Enabled:       true,
		DelayMS:       -1,
		UserMaxEnqueued: -10,
		WindowMinutes: -5,
	})

	assert.Equal(t, 15000, bp.delayMS)
	assert.Equal(t, 200, bp.userMaxEnqueued)
	assert.Equal(t, 10, bp.windowMinutes)
}

func TestNewProducerBackpressure_NilLoggerAccepted(t *testing.T) {
	t.Parallel()
	// Should not panic with nil logger
	bp := NewProducerBackpressure(Config{Enabled: true}, nil, nil)
	assert.NotNil(t, bp)
	assert.Nil(t, bp.logger)
}

func TestNewProducerBackpressure_NoCacheAlwaysAllows(t *testing.T) {
	t.Parallel()
	bp := newLocalOnlyBP(Config{Enabled: true, UserMaxEnqueued: 1})
	ctx := context.Background()

	// Without cache, backpressure cannot track — always allows
	can, _, err := bp.CanEnqueue(ctx, "user-1")
	require.NoError(t, err)
	assert.True(t, can)
}

// =============================================================================
// SetQueueInspector
// =============================================================================

func TestSetQueueInspector_SetsInspector(t *testing.T) {
	t.Parallel()
	bp := newLocalOnlyBP(Config{Enabled: true})
	assert.Nil(t, bp.queueInspector)

	inspector := &MockQueueInspector{depth: 42}
	bp.SetQueueInspector(inspector)

	assert.NotNil(t, bp.queueInspector)
	depth, err := bp.queueInspector.GetQueueDepth(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 42, depth)
}

func TestSetQueueInspector_CanReplaceInspector(t *testing.T) {
	t.Parallel()
	bp := newLocalOnlyBP(Config{Enabled: true})

	bp.SetQueueInspector(&MockQueueInspector{depth: 10})
	bp.SetQueueInspector(&MockQueueInspector{depth: 99})

	depth, _ := bp.queueInspector.GetQueueDepth(context.Background())
	assert.Equal(t, 99, depth)
}

func TestSetQueueInspector_NilResetsInspector(t *testing.T) {
	t.Parallel()
	bp := newLocalOnlyBP(Config{Enabled: true, GlobalMaxEnqueued: 100})
	bp.SetQueueInspector(&MockQueueInspector{depth: 200})
	bp.SetQueueInspector(nil)

	// With nil inspector, global check should allow
	can, _, err := bp.CanEnqueue(context.Background(), "user-1")
	require.NoError(t, err)
	assert.True(t, can)
}

// =============================================================================
// RecordEnqueue — With Redis Cache
// =============================================================================

func TestRecordEnqueue_WithCache_IncrementsRedis(t *testing.T) {
	t.Parallel()
	bp, mr := newTestBP(t, Config{Enabled: true, UserMaxEnqueued: 100, WindowMinutes: 10})
	ctx := context.Background()

	require.NoError(t, bp.RecordEnqueue(ctx, "user-1"))
	require.NoError(t, bp.RecordEnqueue(ctx, "user-1"))

	// Verify Redis key exists with correct value
	val, err := mr.Get("test:backpressure:user:user-1")
	require.NoError(t, err)
	assert.Equal(t, "2", val)
}

func TestRecordEnqueue_WithCache_SetsExpirationOnFirstIncrement(t *testing.T) {
	t.Parallel()
	bp, mr := newTestBP(t, Config{Enabled: true, UserMaxEnqueued: 100, WindowMinutes: 5})
	ctx := context.Background()

	require.NoError(t, bp.RecordEnqueue(ctx, "user-1"))

	// Key should have TTL set
	ttl := mr.TTL("test:backpressure:user:user-1")
	assert.True(t, ttl > 0, "key should have TTL set")
	assert.True(t, ttl <= 5*time.Minute, "TTL should be <= 5 minutes")
}

func TestRecordEnqueue_WithCache_DoesNotResetExpirationOnSubsequent(t *testing.T) {
	t.Parallel()
	bp, mr := newTestBP(t, Config{Enabled: true, UserMaxEnqueued: 100, WindowMinutes: 10})
	ctx := context.Background()

	require.NoError(t, bp.RecordEnqueue(ctx, "user-1"))

	// Get TTL after first increment
	initialTTL := mr.TTL("test:backpressure:user:user-1")

	// Fast-forward a bit
	mr.FastForward(30 * time.Second)

	require.NoError(t, bp.RecordEnqueue(ctx, "user-1"))

	// TTL should have decreased (not reset)
	subsequentTTL := mr.TTL("test:backpressure:user:user-1")
	assert.True(t, subsequentTTL < initialTTL, "TTL should not be reset on subsequent increments")
}

func TestRecordEnqueue_Disabled_NoOp(t *testing.T) {
	t.Parallel()
	bp, mr := newTestBP(t, Config{Enabled: false, UserMaxEnqueued: 5})
	ctx := context.Background()

	err := bp.RecordEnqueue(ctx, "user-1")
	require.NoError(t, err)

	// Redis should not have the key
	assert.False(t, mr.Exists("test:backpressure:user:user-1"))
}

func TestRecordEnqueue_NilCache_NoOp(t *testing.T) {
	t.Parallel()
	bp := newLocalOnlyBP(Config{Enabled: true, UserMaxEnqueued: 5})
	ctx := context.Background()

	err := bp.RecordEnqueue(ctx, "user-1")
	require.NoError(t, err)
	// No panic, no error — silently no-op without cache
}

func TestRecordEnqueue_WithCache_ContinuesOnRedisError(t *testing.T) {
	t.Parallel()
	bp, mr := newTestBP(t, Config{Enabled: true, UserMaxEnqueued: 100})
	ctx := context.Background()

	// Close miniredis to simulate connection error
	mr.Close()

	// Should not return error (fail-open)
	err := bp.RecordEnqueue(ctx, "user-1")
	require.NoError(t, err)
}

func TestRecordEnqueue_WithCache_TracksMultipleUsersIndependently(t *testing.T) {
	t.Parallel()
	bp, _ := newTestBP(t, Config{Enabled: true, UserMaxEnqueued: 100})
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		require.NoError(t, bp.RecordEnqueue(ctx, "user-A"))
	}
	for i := 0; i < 3; i++ {
		require.NoError(t, bp.RecordEnqueue(ctx, "user-B"))
	}

	countA, _ := bp.GetUserCount(ctx, "user-A")
	countB, _ := bp.GetUserCount(ctx, "user-B")
	assert.Equal(t, 5, countA)
	assert.Equal(t, 3, countB)
}

// =============================================================================
// GetUserCount / getUserEnqueuedCount
// =============================================================================

func TestGetUserCount_UnknownUser_ReturnsZero(t *testing.T) {
	t.Parallel()
	bp, _ := newTestBP(t, Config{Enabled: true})
	ctx := context.Background()

	count, err := bp.GetUserCount(ctx, "nonexistent")
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestGetUserCount_WithCache_ReadsFromRedis(t *testing.T) {
	t.Parallel()
	bp, mr := newTestBP(t, Config{Enabled: true, UserMaxEnqueued: 100})
	ctx := context.Background()

	// Set value directly in Redis
	require.NoError(t, mr.Set("test:backpressure:user:user-1", "42"))

	count, err := bp.GetUserCount(ctx, "user-1")
	require.NoError(t, err)
	assert.Equal(t, 42, count)
}

func TestGetUserCount_WithCache_ReturnsZeroOnKeyNotFound(t *testing.T) {
	t.Parallel()
	bp, _ := newTestBP(t, Config{Enabled: true, UserMaxEnqueued: 100})
	ctx := context.Background()

	// Key does not exist in Redis — should return 0 (not fallback to stale local)
	count, err := bp.GetUserCount(ctx, "user-1")
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestGetUserCount_WithCache_ReturnsZeroOnRedisError(t *testing.T) {
	t.Parallel()
	bp, mr := newTestBP(t, Config{Enabled: true, UserMaxEnqueued: 100})
	ctx := context.Background()

	// Close Redis to force real error
	mr.Close()

	count, err := bp.GetUserCount(ctx, "user-1")
	require.NoError(t, err)
	assert.Equal(t, 0, count) // fail-open
}

func TestGetUserCount_WithCache_HandlesNonNumericValue(t *testing.T) {
	t.Parallel()
	bp, mr := newTestBP(t, Config{Enabled: true, UserMaxEnqueued: 100})
	ctx := context.Background()

	// Set non-numeric value in Redis
	require.NoError(t, mr.Set("test:backpressure:user:user-1", "not-a-number"))

	count, err := bp.GetUserCount(ctx, "user-1")
	require.NoError(t, err)
	assert.Equal(t, 0, count) // fmt.Sscanf fails silently, returns 0
}

func TestGetUserCount_WithCache_HandlesEmptyStringValue(t *testing.T) {
	t.Parallel()
	bp, mr := newTestBP(t, Config{Enabled: true, UserMaxEnqueued: 100})
	ctx := context.Background()

	require.NoError(t, mr.Set("test:backpressure:user:user-1", ""))

	count, err := bp.GetUserCount(ctx, "user-1")
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestGetUserCount_NilCache_ReturnsZero(t *testing.T) {
	t.Parallel()
	bp := newLocalOnlyBP(Config{Enabled: true, UserMaxEnqueued: 5})
	ctx := context.Background()

	count, err := bp.GetUserCount(ctx, "user-1")
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

// =============================================================================
// ResetUserCount
// =============================================================================

func TestResetUserCount_WithCache_RemovesFromRedis(t *testing.T) {
	t.Parallel()
	bp, mr := newTestBP(t, Config{Enabled: true, UserMaxEnqueued: 100})
	ctx := context.Background()

	require.NoError(t, bp.RecordEnqueue(ctx, "user-1"))
	require.NoError(t, bp.RecordEnqueue(ctx, "user-1"))

	// Verify key exists
	assert.True(t, mr.Exists("test:backpressure:user:user-1"))

	err := bp.ResetUserCount(ctx, "user-1")
	require.NoError(t, err)

	// Key should be gone
	assert.False(t, mr.Exists("test:backpressure:user:user-1"))
}

func TestResetUserCount_NilCache_NoError(t *testing.T) {
	t.Parallel()
	bp := newLocalOnlyBP(Config{Enabled: true})
	ctx := context.Background()

	err := bp.ResetUserCount(ctx, "nonexistent")
	require.NoError(t, err)
}

func TestResetUserCount_DoesNotAffectOtherUsers(t *testing.T) {
	t.Parallel()
	bp, _ := newTestBP(t, Config{Enabled: true, UserMaxEnqueued: 100})
	ctx := context.Background()

	require.NoError(t, bp.RecordEnqueue(ctx, "user-A"))
	require.NoError(t, bp.RecordEnqueue(ctx, "user-B"))

	require.NoError(t, bp.ResetUserCount(ctx, "user-A"))

	countA, _ := bp.GetUserCount(ctx, "user-A")
	countB, _ := bp.GetUserCount(ctx, "user-B")
	assert.Equal(t, 0, countA)
	assert.Equal(t, 1, countB)
}

func TestResetUserCount_AllowsReEnqueueAfterReset(t *testing.T) {
	t.Parallel()
	bp, _ := newTestBP(t, Config{Enabled: true, UserMaxEnqueued: 3})
	ctx := context.Background()

	// Fill up the limit
	for i := 0; i < 3; i++ {
		require.NoError(t, bp.RecordEnqueue(ctx, "user-1"))
	}
	can, _, _ := bp.CanEnqueue(ctx, "user-1")
	assert.False(t, can)

	// Reset and verify can enqueue again
	require.NoError(t, bp.ResetUserCount(ctx, "user-1"))

	can, _, err := bp.CanEnqueue(ctx, "user-1")
	require.NoError(t, err)
	assert.True(t, can)
}

// =============================================================================
// getUserKey
// =============================================================================

func TestGetUserKey_Format(t *testing.T) {
	t.Parallel()
	bp := newLocalOnlyBP(Config{Enabled: true})

	assert.Equal(t, "backpressure:user:user-123", bp.getUserKey("user-123"))
	assert.Equal(t, "backpressure:user:", bp.getUserKey(""))
	assert.Equal(t, "backpressure:user:abc-def-ghi", bp.getUserKey("abc-def-ghi"))
}

// =============================================================================
// CanEnqueue — Integration with Cache
// =============================================================================

func TestCanEnqueue_WithCache_ChecksRedisCount(t *testing.T) {
	t.Parallel()
	bp, mr := newTestBP(t, Config{
		Enabled:         true,
		UserMaxEnqueued: 5,
		WindowMinutes:   10,
	})
	ctx := context.Background()

	// Set user count to 5 directly in Redis
	require.NoError(t, mr.Set("test:backpressure:user:user-1", "5"))

	can, reason, err := bp.CanEnqueue(ctx, "user-1")
	require.NoError(t, err)
	assert.False(t, can)
	assert.Contains(t, reason, "Limite de 5 mensagens")
}

func TestCanEnqueue_WithCache_AllowsBelowLimit(t *testing.T) {
	t.Parallel()
	bp, mr := newTestBP(t, Config{
		Enabled:         true,
		UserMaxEnqueued: 10,
	})
	ctx := context.Background()

	require.NoError(t, mr.Set("test:backpressure:user:user-1", "9"))

	can, reason, err := bp.CanEnqueue(ctx, "user-1")
	require.NoError(t, err)
	assert.True(t, can)
	assert.Empty(t, reason)
}

func TestCanEnqueue_FailsOpenOnCacheError(t *testing.T) {
	t.Parallel()
	bp, mr := newTestBP(t, Config{
		Enabled:         true,
		UserMaxEnqueued: 5,
	})
	ctx := context.Background()

	// Close Redis
	mr.Close()

	// Should allow (fail-open)
	can, _, err := bp.CanEnqueue(ctx, "user-1")
	require.NoError(t, err)
	assert.True(t, can)
}

func TestCanEnqueue_UserLimitMessage_IncludesWindowMinutes(t *testing.T) {
	t.Parallel()
	bp, _ := newTestBP(t, Config{
		Enabled:         true,
		UserMaxEnqueued: 10,
		WindowMinutes:   15,
	})
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		require.NoError(t, bp.RecordEnqueue(ctx, "user-1"))
	}

	_, reason, _ := bp.CanEnqueue(ctx, "user-1")
	assert.Contains(t, reason, "10 mensagens")
	assert.Contains(t, reason, "15 minutos")
}

func TestCanEnqueue_ExactlyAtLimit_Blocks(t *testing.T) {
	t.Parallel()
	bp, _ := newTestBP(t, Config{
		Enabled:         true,
		UserMaxEnqueued: 1,
	})
	ctx := context.Background()

	require.NoError(t, bp.RecordEnqueue(ctx, "user-1"))

	can, _, err := bp.CanEnqueue(ctx, "user-1")
	require.NoError(t, err)
	assert.False(t, can)
}

func TestCanEnqueue_JustBelowLimit_Allows(t *testing.T) {
	t.Parallel()
	bp, _ := newTestBP(t, Config{
		Enabled:         true,
		UserMaxEnqueued: 3,
	})
	ctx := context.Background()

	require.NoError(t, bp.RecordEnqueue(ctx, "user-1"))
	require.NoError(t, bp.RecordEnqueue(ctx, "user-1"))

	can, _, err := bp.CanEnqueue(ctx, "user-1")
	require.NoError(t, err)
	assert.True(t, can)
}

func TestCanEnqueue_AfterKeyExpires_AllowsAgain(t *testing.T) {
	t.Parallel()
	bp, mr := newTestBP(t, Config{
		Enabled:         true,
		UserMaxEnqueued: 2,
		WindowMinutes:   10,
	})
	ctx := context.Background()

	// Fill to limit
	require.NoError(t, bp.RecordEnqueue(ctx, "user-1"))
	require.NoError(t, bp.RecordEnqueue(ctx, "user-1"))
	can, _, _ := bp.CanEnqueue(ctx, "user-1")
	assert.False(t, can)

	// Fast-forward past the window — key expires
	mr.FastForward(11 * time.Minute)

	// Should allow again (key not found = count 0)
	can, _, err := bp.CanEnqueue(ctx, "user-1")
	require.NoError(t, err)
	assert.True(t, can)
}

// =============================================================================
// GetDelay — Boundary Tests
// =============================================================================

func TestGetDelay_BoundaryValues(t *testing.T) {
	t.Parallel()
	bp := newLocalOnlyBP(Config{Enabled: true, DelayMS: 10000})
	ctx := context.Background()

	tests := []struct {
		name      string
		depth     int
		expectMS  int64
	}{
		{"depth 0", 0, 0},
		{"depth 1", 1, 0},
		{"depth 49", 49, 0},
		{"depth 50", 50, 0},
		{"depth 51", 51, 5000},     // Half delay
		{"depth 75", 75, 5000},     // Half delay
		{"depth 99", 99, 5000},     // Half delay
		{"depth 100", 100, 5000},   // Half delay
		{"depth 101", 101, 10000},  // Full delay
		{"depth 500", 500, 10000},  // Full delay
		{"depth 10000", 10000, 10000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			delay := bp.GetDelay(ctx, tt.depth)
			assert.Equal(t, tt.expectMS, delay.Milliseconds())
		})
	}
}

func TestGetDelay_NegativeDepth_ReturnsZero(t *testing.T) {
	t.Parallel()
	bp := newLocalOnlyBP(Config{Enabled: true, DelayMS: 10000})
	delay := bp.GetDelay(context.Background(), -1)
	assert.Equal(t, int64(0), delay.Milliseconds())
}

func TestGetDelay_ReturnsDuration_NotJustMillis(t *testing.T) {
	t.Parallel()
	bp := newLocalOnlyBP(Config{Enabled: true, DelayMS: 15000})
	delay := bp.GetDelay(context.Background(), 200)
	assert.Equal(t, 15*time.Second, delay)
}

// =============================================================================
// GetGlobalMaxEnqueued
// =============================================================================

func TestGetGlobalMaxEnqueued_ZeroWhenNotSet(t *testing.T) {
	t.Parallel()
	bp := newLocalOnlyBP(Config{Enabled: true})
	assert.Equal(t, 0, bp.GetGlobalMaxEnqueued())
}

func TestGetGlobalMaxEnqueued_ReturnsConfiguredValue(t *testing.T) {
	t.Parallel()
	bp := newLocalOnlyBP(Config{Enabled: true, GlobalMaxEnqueued: 999})
	assert.Equal(t, 999, bp.GetGlobalMaxEnqueued())
}

// =============================================================================
// checkGlobalCapacity — Detailed
// =============================================================================

func TestCheckGlobalCapacity_NegativeGlobalMax_AllowsAlways(t *testing.T) {
	t.Parallel()
	bp := newLocalOnlyBP(Config{
		Enabled:           true,
		GlobalMaxEnqueued: -1,
	})
	bp.SetQueueInspector(&MockQueueInspector{depth: 999999})

	can, _, err := bp.CanEnqueue(context.Background(), "user-1")
	require.NoError(t, err)
	assert.True(t, can)
}

func TestCheckGlobalCapacity_ExactlyAtLimit_Blocks(t *testing.T) {
	t.Parallel()
	bp := newLocalOnlyBP(Config{
		Enabled:           true,
		GlobalMaxEnqueued: 50,
		UserMaxEnqueued:   1000,
	})
	bp.SetQueueInspector(&MockQueueInspector{depth: 50})

	can, reason, err := bp.CanEnqueue(context.Background(), "user-1")
	require.NoError(t, err)
	assert.False(t, can)
	assert.Contains(t, reason, "50/50")
}

func TestCheckGlobalCapacity_OneUnderLimit_Allows(t *testing.T) {
	t.Parallel()
	bp := newLocalOnlyBP(Config{
		Enabled:           true,
		GlobalMaxEnqueued: 50,
		UserMaxEnqueued:   1000,
	})
	bp.SetQueueInspector(&MockQueueInspector{depth: 49})

	can, _, err := bp.CanEnqueue(context.Background(), "user-1")
	require.NoError(t, err)
	assert.True(t, can)
}

// =============================================================================
// GetGlobalQueueDepth — Additional Tests
// =============================================================================

func TestGetGlobalQueueDepth_WithInspector_ReturnsExactDepth(t *testing.T) {
	t.Parallel()
	bp := newLocalOnlyBP(Config{Enabled: true})
	bp.SetQueueInspector(&MockQueueInspector{depth: 12345})

	depth, err := bp.GetGlobalQueueDepth(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 12345, depth)
}

func TestGetGlobalQueueDepth_InspectorReturnsZero(t *testing.T) {
	t.Parallel()
	bp := newLocalOnlyBP(Config{Enabled: true})
	bp.SetQueueInspector(&MockQueueInspector{depth: 0})

	depth, err := bp.GetGlobalQueueDepth(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, depth)
}

// =============================================================================
// Concurrency Tests
// =============================================================================

func TestRecordEnqueue_ConcurrentAccess_IsThreadSafe(t *testing.T) {
	t.Parallel()
	bp, _ := newTestBP(t, Config{Enabled: true, UserMaxEnqueued: 10000})
	ctx := context.Background()

	const goroutines = 50
	const perGoroutine = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < perGoroutine; j++ {
				_ = bp.RecordEnqueue(ctx, "user-1")
			}
		}()
	}

	wg.Wait()

	count, err := bp.GetUserCount(ctx, "user-1")
	require.NoError(t, err)
	assert.Equal(t, goroutines*perGoroutine, count)
}

func TestRecordEnqueue_ConcurrentMultipleUsers_IsThreadSafe(t *testing.T) {
	t.Parallel()
	bp, _ := newTestBP(t, Config{Enabled: true, UserMaxEnqueued: 10000})
	ctx := context.Background()

	const users = 10
	const perUser = 100

	var wg sync.WaitGroup
	wg.Add(users)

	for i := 0; i < users; i++ {
		userID := fmt.Sprintf("user-%d", i)
		go func(uid string) {
			defer wg.Done()
			for j := 0; j < perUser; j++ {
				_ = bp.RecordEnqueue(ctx, uid)
			}
		}(userID)
	}

	wg.Wait()

	for i := 0; i < users; i++ {
		count, err := bp.GetUserCount(ctx, fmt.Sprintf("user-%d", i))
		require.NoError(t, err)
		assert.Equal(t, perUser, count)
	}
}

func TestCanEnqueue_ConcurrentReadsAndWrites(t *testing.T) {
	t.Parallel()
	bp, _ := newTestBP(t, Config{Enabled: true, UserMaxEnqueued: 1000})
	ctx := context.Background()

	var wg sync.WaitGroup
	const goroutines = 20

	// Writers
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_ = bp.RecordEnqueue(ctx, "user-1")
			}
		}()
	}

	// Readers (concurrent with writers)
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_, _, _ = bp.CanEnqueue(ctx, "user-1")
			}
		}()
	}

	// Should not panic or data race
	wg.Wait()
}

func TestResetUserCount_ConcurrentWithRecord(t *testing.T) {
	t.Parallel()
	bp, _ := newTestBP(t, Config{Enabled: true, UserMaxEnqueued: 10000})
	ctx := context.Background()

	var wg sync.WaitGroup

	// Writers
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 500; i++ {
			_ = bp.RecordEnqueue(ctx, "user-1")
		}
	}()

	// Resetters
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			_ = bp.ResetUserCount(ctx, "user-1")
			time.Sleep(time.Microsecond)
		}
	}()

	// Should not panic or data race
	wg.Wait()
}

// =============================================================================
// Integration: Full Flow with Redis
// =============================================================================

func TestFullFlow_RecordCheckResetCheck_WithCache(t *testing.T) {
	t.Parallel()
	bp, _ := newTestBP(t, Config{
		Enabled:         true,
		UserMaxEnqueued: 3,
		WindowMinutes:   10,
	})
	ctx := context.Background()

	// Record 3 messages
	for i := 0; i < 3; i++ {
		require.NoError(t, bp.RecordEnqueue(ctx, "user-1"))
	}

	// Should be blocked
	can, reason, err := bp.CanEnqueue(ctx, "user-1")
	require.NoError(t, err)
	assert.False(t, can)
	assert.Contains(t, reason, "Limite")

	// Reset
	require.NoError(t, bp.ResetUserCount(ctx, "user-1"))

	// Should be allowed again
	can, reason, err = bp.CanEnqueue(ctx, "user-1")
	require.NoError(t, err)
	assert.True(t, can)
	assert.Empty(t, reason)
}

func TestFullFlow_GlobalAndUserLimits_BothChecked(t *testing.T) {
	t.Parallel()
	bp := newLocalOnlyBP(Config{
		Enabled:           true,
		UserMaxEnqueued:   100,
		GlobalMaxEnqueued: 50,
	})
	ctx := context.Background()

	// User is under limit, but global is over
	bp.SetQueueInspector(&MockQueueInspector{depth: 60})

	can, reason, err := bp.CanEnqueue(ctx, "user-1")
	require.NoError(t, err)
	assert.False(t, can)
	assert.Contains(t, reason, "Sistema ocupado")

	// Fix global, now user should pass
	bp.SetQueueInspector(&MockQueueInspector{depth: 10})

	can, reason, err = bp.CanEnqueue(ctx, "user-1")
	require.NoError(t, err)
	assert.True(t, can)
	assert.Empty(t, reason)
}

func TestFullFlow_MultipleUsers_IndependentLimits(t *testing.T) {
	t.Parallel()
	bp, _ := newTestBP(t, Config{
		Enabled:         true,
		UserMaxEnqueued: 2,
		WindowMinutes:   10,
	})
	ctx := context.Background()

	// Fill user-A to limit
	require.NoError(t, bp.RecordEnqueue(ctx, "user-A"))
	require.NoError(t, bp.RecordEnqueue(ctx, "user-A"))

	// user-A blocked, user-B still allowed
	canA, _, _ := bp.CanEnqueue(ctx, "user-A")
	canB, _, _ := bp.CanEnqueue(ctx, "user-B")
	assert.False(t, canA)
	assert.True(t, canB)

	// user-B records and reaches limit too
	require.NoError(t, bp.RecordEnqueue(ctx, "user-B"))
	require.NoError(t, bp.RecordEnqueue(ctx, "user-B"))

	canB, _, _ = bp.CanEnqueue(ctx, "user-B")
	assert.False(t, canB)

	// Reset user-A only
	require.NoError(t, bp.ResetUserCount(ctx, "user-A"))
	canA, _, _ = bp.CanEnqueue(ctx, "user-A")
	canB, _, _ = bp.CanEnqueue(ctx, "user-B")
	assert.True(t, canA)
	assert.False(t, canB)
}

// =============================================================================
// Edge Cases
// =============================================================================

func TestCanEnqueue_EmptyUserID(t *testing.T) {
	t.Parallel()
	bp, _ := newTestBP(t, Config{Enabled: true, UserMaxEnqueued: 5})
	ctx := context.Background()

	can, _, err := bp.CanEnqueue(ctx, "")
	require.NoError(t, err)
	assert.True(t, can)

	// Record for empty user
	require.NoError(t, bp.RecordEnqueue(ctx, ""))
	count, _ := bp.GetUserCount(ctx, "")
	assert.Equal(t, 1, count)
}

func TestCanEnqueue_SpecialCharactersInUserID(t *testing.T) {
	t.Parallel()
	bp, _ := newTestBP(t, Config{Enabled: true, UserMaxEnqueued: 5})
	ctx := context.Background()

	specialIDs := []string{
		"user:with:colons",
		"user/with/slashes",
		"user with spaces",
		"user@email.com",
		"user-uuid-550e8400-e29b-41d4-a716-446655440000",
	}

	for _, uid := range specialIDs {
		can, _, err := bp.CanEnqueue(ctx, uid)
		require.NoError(t, err, "userID: %s", uid)
		assert.True(t, can, "userID: %s", uid)

		require.NoError(t, bp.RecordEnqueue(ctx, uid))
		count, _ := bp.GetUserCount(ctx, uid)
		assert.Equal(t, 1, count, "userID: %s", uid)
	}
}

func TestCanEnqueue_DisabledBP_SkipsEverything(t *testing.T) {
	t.Parallel()
	inspectorCalled := false
	inspector := &callTrackingInspector{called: &inspectorCalled}

	bp := newLocalOnlyBP(Config{
		Enabled:           false,
		GlobalMaxEnqueued: 1,
		UserMaxEnqueued:   1,
	})
	bp.SetQueueInspector(inspector)

	can, reason, err := bp.CanEnqueue(context.Background(), "user-1")
	require.NoError(t, err)
	assert.True(t, can)
	assert.Empty(t, reason)
	assert.False(t, inspectorCalled, "inspector should not be called when disabled")
}

// callTrackingInspector tracks whether GetQueueDepth was called.
type callTrackingInspector struct {
	called *bool
}

func (c *callTrackingInspector) GetQueueDepth(ctx context.Context) (int, error) {
	*c.called = true
	return 0, nil
}

func TestRecordEnqueue_ReturnValueAlwaysNil(t *testing.T) {
	t.Parallel()
	bp, _ := newTestBP(t, Config{Enabled: true, UserMaxEnqueued: 100})
	ctx := context.Background()

	// RecordEnqueue always returns nil (errors are logged, not returned)
	err := bp.RecordEnqueue(ctx, "user-1")
	assert.NoError(t, err)
}

func TestGetDelay_WithDifferentDelayConfigs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		delayMS int
		depth   int
		want    time.Duration
	}{
		{"1s delay, high depth", 1000, 101, 1 * time.Second},
		{"1s delay, medium depth", 1000, 75, 500 * time.Millisecond},
		{"500ms delay, high depth", 500, 101, 500 * time.Millisecond},
		{"500ms delay, medium depth", 500, 75, 250 * time.Millisecond},
		{"odd delay, medium depth", 333, 75, 166 * time.Millisecond}, // 333/2 = 166 (integer division)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			bp := newLocalOnlyBP(Config{Enabled: true, DelayMS: tt.delayMS})
			delay := bp.GetDelay(context.Background(), tt.depth)
			assert.Equal(t, tt.want, delay)
		})
	}
}

// =============================================================================
// MockQueueInspector — Dynamic (for multi-call scenarios)
// =============================================================================

type dynamicMockInspector struct {
	mu     sync.Mutex
	depths []int
	errs   []error
	calls  int
}

func (d *dynamicMockInspector) GetQueueDepth(ctx context.Context) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	idx := d.calls
	d.calls++

	depth := 0
	if idx < len(d.depths) {
		depth = d.depths[idx]
	}

	var err error
	if idx < len(d.errs) {
		err = d.errs[idx]
	}

	return depth, err
}

func TestCanEnqueue_InspectorRecoveryAfterError(t *testing.T) {
	t.Parallel()
	bp := newLocalOnlyBP(Config{
		Enabled:           true,
		GlobalMaxEnqueued: 100,
		UserMaxEnqueued:   1000,
	})

	inspector := &dynamicMockInspector{
		depths: []int{0, 50},
		errs:   []error{errors.New("temporary error"), nil},
	}
	bp.SetQueueInspector(inspector)
	ctx := context.Background()

	// First call: error → fail-open
	can, _, err := bp.CanEnqueue(ctx, "user-1")
	require.NoError(t, err)
	assert.True(t, can)

	// Second call: inspector recovered → normal check
	can, _, err = bp.CanEnqueue(ctx, "user-1")
	require.NoError(t, err)
	assert.True(t, can) // depth 50 < 100
}

// =============================================================================
// CanEnqueue — Global capacity message format
// =============================================================================

func TestCanEnqueue_GlobalBlockMessage_ContainsDepthAndMax(t *testing.T) {
	t.Parallel()
	bp := newLocalOnlyBP(Config{
		Enabled:           true,
		GlobalMaxEnqueued: 500,
		UserMaxEnqueued:   1000,
	})
	bp.SetQueueInspector(&MockQueueInspector{depth: 750})

	_, reason, _ := bp.CanEnqueue(context.Background(), "user-1")
	assert.Contains(t, reason, "750/500")
	assert.Contains(t, reason, "Sistema ocupado")
	assert.Contains(t, reason, "aguarde")
}
