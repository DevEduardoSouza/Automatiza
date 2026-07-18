package workers

import (
	"fmt"
	"testing"
)

// TestIsBackpressureError guards the mirror-DLQ fix: only the fan-out
// backpressure class gets the extended maxBackpressureRetries budget, matched on
// the stable "backpressure" marker — not on "temporary", which other transient
// errors also carry and which must keep the default MaxWebhookRetries.
func TestIsBackpressureError(t *testing.T) {
	// Exact string produced at monitoring_message_processor.go:298.
	busy := fmt.Errorf("mirror fan-out busy for chat %s (backpressure, temporary — will retry)", "5511@g.us")
	if !isBackpressureError(busy) {
		t.Fatal("the real 'mirror fan-out busy' error must be classified as backpressure")
	}

	// A generic transient error carries "temporary" but NOT "backpressure": it
	// must keep the default budget, not the extended one.
	if isBackpressureError(fmt.Errorf("temporary failure resolving host")) {
		t.Fatal("a plain 'temporary' error must NOT be treated as backpressure")
	}
	if isBackpressureError(nil) {
		t.Fatal("nil error is not backpressure")
	}
}

// TestBackpressureBudgetBelowAsynqCeiling guards the invariant that the app-level
// DLQ backstop fires BEFORE asynq archives the task: the busy budget must be
// strictly below the webhook task's asynq.MaxRetry(15) in asynq.go. If someone
// raises maxBackpressureRetries past the asynq ceiling, asynq would archive the
// task first and the offer would be lost outside our DLQ accounting.
func TestBackpressureBudgetBelowAsynqCeiling(t *testing.T) {
	const webhookAsynqMaxRetry = 15 // must match asynq.NewTask(TypeWAHAWebhook, ... MaxRetry) in asynq.go
	if maxBackpressureRetries >= webhookAsynqMaxRetry {
		t.Fatalf("maxBackpressureRetries(%d) must be < webhook asynq.MaxRetry(%d) so the app DLQ gate fires before asynq archives",
			maxBackpressureRetries, webhookAsynqMaxRetry)
	}
	if maxBackpressureRetries <= MaxWebhookRetries {
		t.Fatalf("maxBackpressureRetries(%d) must exceed MaxWebhookRetries(%d) or the fix is a no-op",
			maxBackpressureRetries, MaxWebhookRetries)
	}
}
