package jobs

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// ExecuteWithRateLimit Tests
// =============================================================================

func TestExecuteWithRateLimit_Success(t *testing.T) {
	ctx := context.Background()
	groups := []string{"group1@g.us", "group2@g.us", "group3@g.us"}

	var callCount int32
	results := ExecuteWithRateLimit(ctx, groups, func(groupID string) error {
		atomic.AddInt32(&callCount, 1)
		return nil
	})

	assert.Len(t, results, 3)
	assert.Equal(t, int32(3), atomic.LoadInt32(&callCount))
	for _, r := range results {
		assert.True(t, r.Success)
		assert.Empty(t, r.Error)
	}
}

func TestExecuteWithRateLimit_PartialFailure(t *testing.T) {
	ctx := context.Background()
	groups := []string{"group1@g.us", "group2@g.us", "group3@g.us"}

	results := ExecuteWithRateLimit(ctx, groups, func(groupID string) error {
		if groupID == "group2@g.us" {
			return errors.New("failed for group2")
		}
		return nil
	})

	assert.Len(t, results, 3)

	successCount := 0
	failCount := 0
	for _, r := range results {
		if r.Success {
			successCount++
		} else {
			failCount++
			assert.Contains(t, r.Error, "failed for group2")
		}
	}

	assert.Equal(t, 2, successCount)
	assert.Equal(t, 1, failCount)
}

func TestExecuteWithRateLimit_AllFailure(t *testing.T) {
	ctx := context.Background()
	groups := []string{"group1@g.us", "group2@g.us"}

	results := ExecuteWithRateLimit(ctx, groups, func(groupID string) error {
		return errors.New("always fails")
	})

	assert.Len(t, results, 2)
	for _, r := range results {
		assert.False(t, r.Success)
		assert.Equal(t, "always fails", r.Error)
	}
}

func TestExecuteWithRateLimit_EmptyGroups(t *testing.T) {
	ctx := context.Background()
	groups := []string{}

	results := ExecuteWithRateLimit(ctx, groups, func(groupID string) error {
		t.Fatal("should not be called")
		return nil
	})

	assert.Len(t, results, 0)
}

func TestExecuteWithRateLimit_SingleGroup(t *testing.T) {
	ctx := context.Background()
	groups := []string{"group1@g.us"}

	var called bool
	results := ExecuteWithRateLimit(ctx, groups, func(groupID string) error {
		called = true
		assert.Equal(t, "group1@g.us", groupID)
		return nil
	})

	assert.True(t, called)
	assert.Len(t, results, 1)
	assert.True(t, results[0].Success)
	assert.Equal(t, "group1@g.us", results[0].GroupID)
}

func TestExecuteWithRateLimit_ContextCancelled_BeforeStart(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	groups := []string{"group1@g.us", "group2@g.us"}

	var callCount int32
	results := ExecuteWithRateLimit(ctx, groups, func(groupID string) error {
		atomic.AddInt32(&callCount, 1)
		return nil
	})

	// First group might execute, but context check should catch it quickly
	assert.Len(t, results, 2)

	// At least one should be canceled
	hasCancelled := false
	for _, r := range results {
		if !r.Success && r.Error == "canceled" {
			hasCancelled = true
			break
		}
	}
	assert.True(t, hasCancelled, "at least one result should be canceled")
}

func TestExecuteWithRateLimit_ContextCancelled_DuringExecution(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	groups := []string{"group1@g.us", "group2@g.us", "group3@g.us", "group4@g.us"}

	var callCount int32
	results := ExecuteWithRateLimit(ctx, groups, func(groupID string) error {
		count := atomic.AddInt32(&callCount, 1)
		if count == 2 {
			cancel() // Cancel after second group
		}
		return nil
	})

	assert.Len(t, results, 4)

	// Should have some successful and some canceled
	successCount := 0
	cancelledCount := 0
	for _, r := range results {
		if r.Success {
			successCount++
		} else if r.Error == "canceled" {
			cancelledCount++
		}
	}

	// At least the first 2 should succeed (before cancel), rest might be canceled
	assert.GreaterOrEqual(t, successCount, 1, "at least one should succeed")
}

func TestExecuteWithRateLimit_PreservesGroupOrder(t *testing.T) {
	ctx := context.Background()
	groups := []string{"group1@g.us", "group2@g.us", "group3@g.us"}

	var executionOrder []string
	results := ExecuteWithRateLimit(ctx, groups, func(groupID string) error {
		executionOrder = append(executionOrder, groupID)
		return nil
	})

	// Verify execution order matches input order
	require.Len(t, executionOrder, 3)
	assert.Equal(t, "group1@g.us", executionOrder[0])
	assert.Equal(t, "group2@g.us", executionOrder[1])
	assert.Equal(t, "group3@g.us", executionOrder[2])

	// Verify results preserve order
	require.Len(t, results, 3)
	assert.Equal(t, "group1@g.us", results[0].GroupID)
	assert.Equal(t, "group2@g.us", results[1].GroupID)
	assert.Equal(t, "group3@g.us", results[2].GroupID)
}

func TestExecuteWithRateLimit_ErrorMessages(t *testing.T) {
	ctx := context.Background()
	groups := []string{"group1@g.us", "group2@g.us"}

	results := ExecuteWithRateLimit(ctx, groups, func(groupID string) error {
		return errors.New("custom error for " + groupID)
	})

	require.Len(t, results, 2)
	assert.Equal(t, "custom error for group1@g.us", results[0].Error)
	assert.Equal(t, "custom error for group2@g.us", results[1].Error)
}

// =============================================================================
// AggregateResults Tests
// =============================================================================

func TestAggregateResults_AllSuccess(t *testing.T) {
	results := []ActionResult{
		{GroupID: "group1", Success: true},
		{GroupID: "group2", Success: true},
		{GroupID: "group3", Success: true},
	}

	success, failed, errorMsg := AggregateResults(results)

	assert.Equal(t, 3, success)
	assert.Equal(t, 0, failed)
	assert.Nil(t, errorMsg)
}

func TestAggregateResults_AllFailed(t *testing.T) {
	results := []ActionResult{
		{GroupID: "group1", Success: false, Error: "error1"},
		{GroupID: "group2", Success: false, Error: "error2"},
	}

	success, failed, errorMsg := AggregateResults(results)

	assert.Equal(t, 0, success)
	assert.Equal(t, 2, failed)
	assert.NotNil(t, errorMsg)
	assert.Contains(t, *errorMsg, "group1: error1")
	assert.Contains(t, *errorMsg, "group2: error2")
	assert.Contains(t, *errorMsg, "; ") // Semicolon separator
}

func TestAggregateResults_PartialSuccess(t *testing.T) {
	results := []ActionResult{
		{GroupID: "group1", Success: true},
		{GroupID: "group2", Success: false, Error: "failed"},
		{GroupID: "group3", Success: true},
	}

	success, failed, errorMsg := AggregateResults(results)

	assert.Equal(t, 2, success)
	assert.Equal(t, 1, failed)
	assert.NotNil(t, errorMsg)
	assert.Contains(t, *errorMsg, "group2: failed")
	assert.NotContains(t, *errorMsg, "group1")
	assert.NotContains(t, *errorMsg, "group3")
}

func TestAggregateResults_Empty(t *testing.T) {
	results := []ActionResult{}

	success, failed, errorMsg := AggregateResults(results)

	assert.Equal(t, 0, success)
	assert.Equal(t, 0, failed)
	assert.Nil(t, errorMsg)
}

func TestAggregateResults_SingleSuccess(t *testing.T) {
	results := []ActionResult{
		{GroupID: "group1", Success: true},
	}

	success, failed, errorMsg := AggregateResults(results)

	assert.Equal(t, 1, success)
	assert.Equal(t, 0, failed)
	assert.Nil(t, errorMsg)
}

func TestAggregateResults_SingleFailure(t *testing.T) {
	results := []ActionResult{
		{GroupID: "group1", Success: false, Error: "single error"},
	}

	success, failed, errorMsg := AggregateResults(results)

	assert.Equal(t, 0, success)
	assert.Equal(t, 1, failed)
	assert.NotNil(t, errorMsg)
	assert.Equal(t, "group1: single error", *errorMsg)
}

// =============================================================================
// DetermineJobStatus Tests
// =============================================================================

func TestDetermineJobStatus(t *testing.T) {
	tests := []struct {
		name         string
		successCount int
		failedCount  int
		wantStatus   string
	}{
		{
			name:         "all success",
			successCount: 5,
			failedCount:  0,
			wantStatus:   "done",
		},
		{
			name:         "all failed",
			successCount: 0,
			failedCount:  5,
			wantStatus:   "failed",
		},
		{
			name:         "partial success (more success)",
			successCount: 4,
			failedCount:  1,
			wantStatus:   "done",
		},
		{
			name:         "partial success (more failed)",
			successCount: 1,
			failedCount:  4,
			wantStatus:   "done", // Partial success is still "done"
		},
		{
			name:         "equal success and failed",
			successCount: 3,
			failedCount:  3,
			wantStatus:   "done", // Any success means "done"
		},
		{
			name:         "single success single failure",
			successCount: 1,
			failedCount:  1,
			wantStatus:   "done",
		},
		{
			name:         "zero both",
			successCount: 0,
			failedCount:  0,
			wantStatus:   "done", // No failures = done
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := DetermineJobStatus(tt.successCount, tt.failedCount)
			assert.Equal(t, tt.wantStatus, status)
		})
	}
}

// =============================================================================
// RateLimitDelay Constant Test
// =============================================================================

func TestRateLimitDelay_Value(t *testing.T) {
	// Ensure the rate limit delay is set to expected value
	assert.Equal(t, 1*time.Second, RateLimitDelay)
}

// =============================================================================
// ActionResult Structure Tests
// =============================================================================

func TestActionResult_Fields(t *testing.T) {
	// Test successful result
	successResult := ActionResult{
		GroupID: "group1@g.us",
		Success: true,
		Error:   "",
	}
	assert.Equal(t, "group1@g.us", successResult.GroupID)
	assert.True(t, successResult.Success)
	assert.Empty(t, successResult.Error)

	// Test failed result
	failedResult := ActionResult{
		GroupID: "group2@g.us",
		Success: false,
		Error:   "connection timeout",
	}
	assert.Equal(t, "group2@g.us", failedResult.GroupID)
	assert.False(t, failedResult.Success)
	assert.Equal(t, "connection timeout", failedResult.Error)
}

// =============================================================================
// Integration-Style Tests
// =============================================================================

func TestExecuteWithRateLimit_LongRunningFunction(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	ctx := context.Background()
	groups := []string{"group1@g.us", "group2@g.us"}

	start := time.Now()
	results := ExecuteWithRateLimit(ctx, groups, func(groupID string) error {
		// Simulate some work
		time.Sleep(10 * time.Millisecond)
		return nil
	})
	elapsed := time.Since(start)

	assert.Len(t, results, 2)
	// Should have at least RateLimitDelay between groups
	// First group: no delay, Second group: RateLimitDelay
	// Total minimum: RateLimitDelay + 2*10ms work
	assert.GreaterOrEqual(t, elapsed, RateLimitDelay, "should respect rate limit delay")
}

func TestExecuteWithRateLimit_ContextTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	// Create a context with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Many groups - should timeout before completing all
	groups := make([]string, 10)
	for i := range groups {
		groups[i] = "group@g.us"
	}

	results := ExecuteWithRateLimit(ctx, groups, func(groupID string) error {
		return nil
	})

	assert.Len(t, results, 10)

	// Some should be canceled due to timeout
	cancelledCount := 0
	for _, r := range results {
		if !r.Success && r.Error == "canceled" {
			cancelledCount++
		}
	}
	// With RateLimitDelay of 1s and timeout of 50ms, most should be canceled
	assert.Greater(t, cancelledCount, 0, "some results should be canceled due to timeout")
}

// =============================================================================
// Error Message Formatting Tests
// =============================================================================

func TestAggregateResults_ErrorFormatting(t *testing.T) {
	results := []ActionResult{
		{GroupID: "group1@g.us", Success: false, Error: "network error"},
		{GroupID: "group2@g.us", Success: false, Error: "permission denied"},
		{GroupID: "group3@g.us", Success: false, Error: "rate limited"},
	}

	_, _, errorMsg := AggregateResults(results)

	require.NotNil(t, errorMsg)

	// Verify format: "groupID: error; groupID: error; ..."
	assert.Contains(t, *errorMsg, "group1@g.us: network error")
	assert.Contains(t, *errorMsg, "group2@g.us: permission denied")
	assert.Contains(t, *errorMsg, "group3@g.us: rate limited")

	// Should have exactly 2 semicolons (3 errors - 1)
	semicolonCount := 0
	for _, c := range *errorMsg {
		if c == ';' {
			semicolonCount++
		}
	}
	assert.Equal(t, 2, semicolonCount)
}

// =============================================================================
// Concurrent Safety Tests
// =============================================================================

func TestAggregateResults_ConcurrentSafe(t *testing.T) {
	// AggregateResults should be safe to call from multiple goroutines
	// since it only reads from the slice

	results := []ActionResult{
		{GroupID: "group1", Success: true},
		{GroupID: "group2", Success: false, Error: "error"},
	}

	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			success, failed, _ := AggregateResults(results)
			assert.Equal(t, 1, success)
			assert.Equal(t, 1, failed)
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}
