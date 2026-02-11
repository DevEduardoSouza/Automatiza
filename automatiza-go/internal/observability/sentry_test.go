package observability

import (
	"errors"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

func TestInitSentry_EmptyDSN_ReturnsError(t *testing.T) {
	err := InitSentry(SentryConfig{DSN: ""})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sentry DSN is required")
}

func TestInitSentry_InvalidDSN_ReturnsError(t *testing.T) {
	err := InitSentry(SentryConfig{DSN: "not-a-valid-dsn"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to initialize sentry")
}

func TestRedactPII_Email(t *testing.T) {
	event := &sentry.Event{
		Message: "user test@example.com failed login",
	}
	result := redactPII(event)
	assert.NotContains(t, result.Message, "test@example.com")
	assert.Contains(t, result.Message, "[EMAIL_REDACTED]")
}

func TestRedactPII_Phone(t *testing.T) {
	event := &sentry.Event{
		Message: "sent message to +5511999887766",
	}
	result := redactPII(event)
	assert.NotContains(t, result.Message, "5511999887766")
	assert.Contains(t, result.Message, "[PHONE_REDACTED]")
}

func TestRedactPII_Bearer(t *testing.T) {
	event := &sentry.Event{
		Message: "Authorization: Bearer eyJhbGciOiJIUzI1NiJ9.test",
	}
	result := redactPII(event)
	assert.NotContains(t, result.Message, "eyJhbGciOiJIUzI1NiJ9")
	assert.Contains(t, result.Message, "[TOKEN_REDACTED]")
}

func TestRedactPII_Password(t *testing.T) {
	event := &sentry.Event{
		Message: "password=secretpass123",
	}
	result := redactPII(event)
	assert.NotContains(t, result.Message, "secretpass123")
	assert.Contains(t, result.Message, "[SECRET_REDACTED]")
}

func TestRedactPII_PreservesExceptionType(t *testing.T) {
	event := &sentry.Event{
		Exception: []sentry.Exception{
			{
				Type:  "*errors.errorString",
				Value: "failed for user test@example.com",
			},
		},
	}
	result := redactPII(event)
	assert.Equal(t, "*errors.errorString", result.Exception[0].Type)
	assert.Contains(t, result.Exception[0].Value, "[EMAIL_REDACTED]")
}

func TestRedactPII_RequestHeaders(t *testing.T) {
	event := &sentry.Event{
		Request: &sentry.Request{
			Cookies: "session=abc123",
			Headers: map[string]string{
				"Authorization": "Bearer secret",
				"Content-Type":  "application/json",
			},
		},
	}
	result := redactPII(event)
	assert.Empty(t, result.Request.Cookies)
	assert.Equal(t, "[REDACTED]", result.Request.Headers["Authorization"])
	assert.Equal(t, "application/json", result.Request.Headers["Content-Type"])
}

func TestRedactPII_UserData(t *testing.T) {
	event := &sentry.Event{
		User: sentry.User{
			ID:        "user-123",
			Email:     "user@example.com",
			IPAddress: "192.168.1.1",
		},
	}
	result := redactPII(event)
	assert.Equal(t, "user-123", result.User.ID) // ID preserved
	assert.Equal(t, "[REDACTED]", result.User.Email)
	assert.Equal(t, "[REDACTED]", result.User.IPAddress)
}

func TestRateLimiter_AllowsUnderThreshold(t *testing.T) {
	rl := &rateLimiter{
		entries:      make(map[string]*rateLimitEntry),
		maxPerWindow: 3,
		window:       60 * time.Second,
	}

	for i := 0; i < 3; i++ {
		shouldSend, _ := rl.shouldSend("test-key")
		assert.True(t, shouldSend, "event %d should be sent", i)
	}
}

func TestRateLimiter_SuppressesOverThreshold(t *testing.T) {
	rl := &rateLimiter{
		entries:      make(map[string]*rateLimitEntry),
		maxPerWindow: 2,
		window:       60 * time.Second,
	}

	send1, _ := rl.shouldSend("test-key")
	assert.True(t, send1)

	send2, _ := rl.shouldSend("test-key")
	assert.True(t, send2)

	send3, suppressed := rl.shouldSend("test-key")
	assert.False(t, send3)
	assert.Equal(t, 1, suppressed)
}

func TestRateLimiter_ResetsAfterWindow(t *testing.T) {
	rl := &rateLimiter{
		entries:      make(map[string]*rateLimitEntry),
		maxPerWindow: 1,
		window:       1 * time.Millisecond,
	}

	send1, _ := rl.shouldSend("test-key")
	assert.True(t, send1)

	send2, _ := rl.shouldSend("test-key")
	assert.False(t, send2)

	// Wait for window to expire
	time.Sleep(5 * time.Millisecond)

	send3, _ := rl.shouldSend("test-key")
	assert.True(t, send3)
}

func TestLogAndCapture_Warning(t *testing.T) {
	logger := zaptest.NewLogger(t)
	testErr := errors.New("test error")
	tags := map[string]string{"module": "test"}

	// Override runtime caller for test
	origCaller := runtimeCaller
	runtimeCaller = func(skip int) (uintptr, string, int, bool) {
		return 0, "test_file.go", 42, true
	}
	defer func() { runtimeCaller = origCaller }()

	// Should not panic
	LogAndCapture(logger, sentry.LevelWarning, "test warning", testErr, tags)
}

func TestLogAndCapture_NilError(t *testing.T) {
	logger := zaptest.NewLogger(t)
	tags := map[string]string{"module": "test"}

	origCaller := runtimeCaller
	runtimeCaller = func(skip int) (uintptr, string, int, bool) {
		return 0, "test_file.go", 42, true
	}
	defer func() { runtimeCaller = origCaller }()

	// Should not panic with nil error
	LogAndCapture(logger, sentry.LevelInfo, "info message", nil, tags)
}

func TestSafeGo_PanicRecovery(t *testing.T) {
	recovered := make(chan bool, 1)

	// We can't easily test Sentry capture without a real DSN,
	// but we can verify the goroutine doesn't crash the test
	SafeGo("test-goroutine", func() {
		defer func() {
			recovered <- true
		}()
		panic("test panic")
	})

	select {
	case <-recovered:
		// SafeGo recovered the panic
	case <-time.After(2 * time.Second):
		t.Fatal("SafeGo did not recover panic within timeout")
	}
}

func TestCaptureWithTags_DoesNotPanic(t *testing.T) {
	testErr := errors.New("test error")
	tags := map[string]string{"module": "test", "flow": "unit-test"}
	// Should not panic even without initialized Sentry
	CaptureWithTags(testErr, tags)
}

func TestCaptureWithScope_DoesNotPanic(t *testing.T) {
	testErr := errors.New("test error")
	CaptureWithScope(testErr, func(scope *sentry.Scope) {
		scope.SetTag("test", "true")
		scope.SetLevel(sentry.LevelError)
	})
}

func TestCaptureMessage_DoesNotPanic(t *testing.T) {
	CaptureMessage(sentry.LevelWarning, "test message", map[string]string{"module": "test"})
}

func TestLogAndCapture_AllLevels(t *testing.T) {
	logger := zap.NewNop()

	origCaller := runtimeCaller
	runtimeCaller = func(skip int) (uintptr, string, int, bool) {
		return 0, "test.go", 1, true
	}
	defer func() { runtimeCaller = origCaller }()

	levels := []sentry.Level{
		sentry.LevelDebug,
		sentry.LevelInfo,
		sentry.LevelWarning,
		sentry.LevelError,
		sentry.LevelFatal,
	}

	for _, level := range levels {
		LogAndCapture(logger, level, "test "+string(level), errors.New("err"), nil)
	}
}
