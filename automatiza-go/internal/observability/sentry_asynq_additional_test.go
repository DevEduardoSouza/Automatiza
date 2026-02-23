package observability

import (
	"context"
	"errors"
	"testing"

	"github.com/getsentry/sentry-go"
	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSentryAsynqMiddleware_WithValidTraceIDInPayload_CreatesSpanLink(t *testing.T) {
	setupTestSentry(t)

	inner := asynq.HandlerFunc(func(_ context.Context, _ *asynq.Task) error {
		return nil
	})
	wrapped := SentryAsynqMiddleware("trace-link-worker", inner)

	// Valid 32-char hex trace ID triggers the span link creation path
	payload := []byte(`{"trace_id":"4bf92f3577b34da6a3ce929d0e0e4736"}`)
	task := asynq.NewTask("test:trace_link", payload)
	err := wrapped.ProcessTask(context.Background(), task)
	assert.NoError(t, err)
}

func TestSentryAsynqMiddleware_WithInvalidTraceIDInPayload_SkipsLink(t *testing.T) {
	setupTestSentry(t)

	inner := asynq.HandlerFunc(func(_ context.Context, _ *asynq.Task) error {
		return nil
	})
	wrapped := SentryAsynqMiddleware("invalid-trace-worker", inner)

	// Invalid trace ID: TraceIDFromHex fails → link is skipped
	payload := []byte(`{"trace_id":"not-a-valid-hex-trace-id"}`)
	task := asynq.NewTask("test:invalid_trace", payload)
	err := wrapped.ProcessTask(context.Background(), task)
	assert.NoError(t, err)
}

func TestSentryAsynqMiddleware_EmptyTraceIDInPayload_SkipsLink(t *testing.T) {
	setupTestSentry(t)

	inner := asynq.HandlerFunc(func(_ context.Context, _ *asynq.Task) error {
		return nil
	})
	wrapped := SentryAsynqMiddleware("empty-trace-worker", inner)

	// trace_id present but empty: condition `payload.TraceID != ""` is false
	payload := []byte(`{"trace_id":""}`)
	task := asynq.NewTask("test:empty_trace", payload)
	err := wrapped.ProcessTask(context.Background(), task)
	assert.NoError(t, err)
}

func TestSentryAsynqMiddleware_InvalidJSONPayload_SkipsLink(t *testing.T) {
	setupTestSentry(t)

	inner := asynq.HandlerFunc(func(_ context.Context, _ *asynq.Task) error {
		return nil
	})
	wrapped := SentryAsynqMiddleware("invalid-json-worker", inner)

	// Invalid JSON: Unmarshal fails → link is skipped
	payload := []byte(`not-json`)
	task := asynq.NewTask("test:invalid_json", payload)
	err := wrapped.ProcessTask(context.Background(), task)
	assert.NoError(t, err)
}

func TestSentryAsynqMiddleware_WithGlobalMetrics_RecordsDuration(t *testing.T) {
	setupTestSentry(t)

	origMetrics := GlobalMetrics
	defer func() { GlobalMetrics = origMetrics }()

	// Enable metrics so the WorkerTaskDuration recording branch is exercised
	require.NoError(t, InitMetrics(true))
	require.NotNil(t, GlobalMetrics)

	inner := asynq.HandlerFunc(func(_ context.Context, _ *asynq.Task) error {
		return nil
	})
	wrapped := SentryAsynqMiddleware("metrics-worker", inner)
	task := asynq.NewTask("test:with_metrics", nil)

	err := wrapped.ProcessTask(context.Background(), task)
	assert.NoError(t, err)
}

func TestSentryAsynqMiddleware_PanicWithErrorType_Captured(t *testing.T) {
	transport := setupTestSentry(t)

	inner := asynq.HandlerFunc(func(_ context.Context, _ *asynq.Task) error {
		panic(errors.New("error-type panic in worker"))
	})
	wrapped := SentryAsynqMiddleware("error-panic-worker", inner)

	task := asynq.NewTask("test:error_panic", nil)
	err := wrapped.ProcessTask(context.Background(), task)
	require.Error(t, err, "panic must be converted to an error")

	sentry.Flush(0)

	found := false
	for _, ev := range transport.Events() {
		if ev.Tags["panic"] == "true" && ev.Tags["worker"] == "error-panic-worker" {
			found = true
			break
		}
	}
	assert.True(t, found, "error-type panic must produce a Sentry event with panic=true tag")
}
