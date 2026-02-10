package jobs

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const (
	// RateLimitDelay is the delay between actions to avoid WAHA rate limiting
	RateLimitDelay = 1 * time.Second
)

// ActionResult represents the result of an action on a single group
type ActionResult struct {
	GroupID string
	Success bool
	Error   string
}

// ExecuteWithRateLimit executes a function for each group with rate limiting between calls.
// It respects context cancellation and collects results for each group.
func ExecuteWithRateLimit(ctx context.Context, groups []string, fn func(groupID string) error) []ActionResult {
	results := make([]ActionResult, 0, len(groups))

	for i, groupID := range groups {
		// Add delay between groups (not before first)
		if i > 0 {
			select {
			case <-ctx.Done():
				// Context canceled, add remaining groups as canceled
				for _, g := range groups[i:] {
					results = append(results, ActionResult{GroupID: g, Success: false, Error: "canceled"})
				}
				return results
			case <-time.After(RateLimitDelay):
				// Delay completed
			}
		}

		// Check context before executing
		select {
		case <-ctx.Done():
			results = append(results, ActionResult{GroupID: groupID, Success: false, Error: "canceled"})
			continue
		default:
		}

		// Execute the action
		err := fn(groupID)
		if err != nil {
			results = append(results, ActionResult{GroupID: groupID, Success: false, Error: err.Error()})
		} else {
			results = append(results, ActionResult{GroupID: groupID, Success: true})
		}
	}

	return results
}

// AggregateResults aggregates action results and returns success count, failed count, and an error message.
// If there are failures, errorMsg will contain a semicolon-separated list of errors.
func AggregateResults(results []ActionResult) (success, failed int, errorMsg *string) {
	var errors []string

	for _, r := range results {
		if r.Success {
			success++
		} else {
			failed++
			errors = append(errors, fmt.Sprintf("%s: %s", r.GroupID, r.Error))
		}
	}

	if len(errors) > 0 {
		msg := strings.Join(errors, "; ")
		errorMsg = &msg
	}

	return success, failed, errorMsg
}

// DetermineJobStatus determines the final job status based on success/failure counts.
// Returns "done" if any success, "failed" only if all failed.
func DetermineJobStatus(successCount, failedCount int) string {
	if failedCount == 0 {
		return "done"
	}
	if successCount > 0 {
		return "done" // Partial success is still "done" with error message
	}
	return "failed"
}
