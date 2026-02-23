package observability

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/hibiken/asynq"
)

// testTransport captures Sentry events for test assertions.
type testTransport struct {
	mu     sync.Mutex
	events []*sentry.Event
}

func (t *testTransport) Configure(_ sentry.ClientOptions) {}
func (t *testTransport) SendEvent(event *sentry.Event) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.events = append(t.events, event)
}
func (t *testTransport) Flush(_ time.Duration) bool              { return true }
func (t *testTransport) FlushWithContext(_ context.Context) bool { return true }
func (t *testTransport) Close()                                  {}

// Events returns a snapshot of captured events (thread-safe).
func (t *testTransport) Events() []*sentry.Event {
	t.mu.Lock()
	defer t.mu.Unlock()
	cp := make([]*sentry.Event, len(t.events))
	copy(cp, t.events)
	return cp
}

func setupTestSentry(t *testing.T) *testTransport {
	t.Helper()
	transport := &testTransport{}
	err := sentry.Init(sentry.ClientOptions{
		Dsn:       "https://key@sentry.io/1",
		Transport: transport,
	})
	if err != nil {
		t.Fatalf("failed to init sentry: %v", err)
	}
	return transport
}

func TestSentryAsynqMiddleware_ErrorCapture(t *testing.T) {
	transport := setupTestSentry(t)

	handlerErr := errors.New("task processing failed")
	inner := asynq.HandlerFunc(func(_ context.Context, _ *asynq.Task) error {
		return handlerErr
	})

	wrapped := SentryAsynqMiddleware("test-worker", inner)

	task := asynq.NewTask("test:task", nil)
	err := wrapped.ProcessTask(context.Background(), task)

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	sentry.Flush(0)

	events := transport.Events()
	if len(events) == 0 {
		t.Fatal("expected sentry event, got none")
	}

	event := events[0]
	if event.Tags["worker"] != "test-worker" {
		t.Errorf("expected tag worker=test-worker, got %s", event.Tags["worker"])
	}
	if event.Tags["task_type"] != "test:task" {
		t.Errorf("expected tag task_type=test:task, got %s", event.Tags["task_type"])
	}
}

func TestSentryAsynqMiddleware_PanicRecovery(t *testing.T) {
	transport := setupTestSentry(t)

	inner := asynq.HandlerFunc(func(_ context.Context, _ *asynq.Task) error {
		panic("worker panic")
	})

	wrapped := SentryAsynqMiddleware("panic-worker", inner)

	task := asynq.NewTask("test:panic", nil)
	err := wrapped.ProcessTask(context.Background(), task)

	if err == nil {
		t.Fatal("expected error from panic recovery, got nil")
	}

	expectedMsg := "panic in worker panic-worker: worker panic"
	if err.Error() != expectedMsg {
		t.Errorf("expected error %q, got %q", expectedMsg, err.Error())
	}

	sentry.Flush(0)

	events := transport.Events()
	if len(events) == 0 {
		t.Fatal("expected sentry event from panic, got none")
	}

	found := false
	for _, ev := range events {
		if ev.Tags["panic"] == "true" && ev.Tags["worker"] == "panic-worker" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected sentry event with panic=true tag")
	}
}

func TestSentryAsynqMiddleware_MaxRetriesCritical(t *testing.T) {
	transport := setupTestSentry(t)

	handlerErr := errors.New("persistent failure")
	inner := asynq.HandlerFunc(func(_ context.Context, _ *asynq.Task) error {
		return handlerErr
	})

	wrapped := SentryAsynqMiddleware("retry-worker", inner)

	// Simulate max retries context: asynq sets these via context values.
	// Since we can't easily set retry/maxRetry via context in unit tests
	// (asynq doesn't export the context keys), we verify that the middleware
	// captures errors correctly. The retry_count/dlq logic is verified
	// through integration tests with a real Asynq server.
	task := asynq.NewTask("test:retry", nil)
	err := wrapped.ProcessTask(context.Background(), task)

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	sentry.Flush(0)

	events := transport.Events()
	if len(events) == 0 {
		t.Fatal("expected sentry event, got none")
	}

	event := events[0]
	if event.Tags["worker"] != "retry-worker" {
		t.Errorf("expected worker=retry-worker, got %s", event.Tags["worker"])
	}

	// When retryCount=0 and maxRetry=0 (defaults from context.Background()),
	// isLastRetry is true (0 >= 0-1), so it should be Critical with dlq=true.
	if event.Level != sentry.LevelFatal {
		t.Errorf("expected level Fatal (Critical) for max retries, got %s", event.Level)
	}
	if event.Tags["dlq"] != "true" {
		t.Errorf("expected dlq=true tag, got %s", event.Tags["dlq"])
	}

	_ = fmt.Sprintf("test helper") // Avoid unused import
}

func TestSentryAsynqMiddleware_Success(t *testing.T) {
	transport := setupTestSentry(t)

	inner := asynq.HandlerFunc(func(_ context.Context, _ *asynq.Task) error {
		return nil
	})

	wrapped := SentryAsynqMiddleware("ok-worker", inner)

	task := asynq.NewTask("test:ok", nil)
	err := wrapped.ProcessTask(context.Background(), task)

	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	sentry.Flush(0)

	events := transport.Events()
	if len(events) != 0 {
		t.Errorf("expected no sentry events for successful task, got %d", len(events))
	}
}
