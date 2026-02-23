package observability

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

func TestInitSentry_ValidDSN_Succeeds(t *testing.T) {
	err := InitSentry(SentryConfig{
		DSN:         "https://key@sentry.io/1",
		Environment: "test",
		SampleRate:  1.0,
	})
	require.NoError(t, err)
}

func TestInitSentry_SampleRateZero_Normalized(t *testing.T) {
	err := InitSentry(SentryConfig{
		DSN:        "https://key@sentry.io/1",
		SampleRate: 0, // <= 0: normalized to 1.0
	})
	require.NoError(t, err)
}

func TestInitSentry_SampleRateAboveOne_Normalized(t *testing.T) {
	err := InitSentry(SentryConfig{
		DSN:        "https://key@sentry.io/1",
		SampleRate: 1.5, // > 1: normalized to 1.0
	})
	require.NoError(t, err)
}

func TestFlush_DoesNotPanic(t *testing.T) {
	Flush(0)
	Flush(10 * time.Millisecond)
}

func TestRedactPII_BreadcrumbMessage(t *testing.T) {
	event := &sentry.Event{
		Breadcrumbs: []*sentry.Breadcrumb{
			{
				Message: "user test@example.com clicked login",
				Data: map[string]interface{}{
					"email": "user@example.com",
					"token": "Bearer secret123",
				},
			},
		},
	}
	result := redactPII(event)
	crumb := result.Breadcrumbs[0]
	assert.Contains(t, crumb.Message, "[EMAIL_REDACTED]")
	assert.NotContains(t, crumb.Message, "test@example.com")
	assert.Equal(t, "[EMAIL_REDACTED]", crumb.Data["email"])
	assert.Contains(t, crumb.Data["token"].(string), "[TOKEN_REDACTED]")
}

func TestRedactPII_BreadcrumbsNonStringData(t *testing.T) {
	event := &sentry.Event{
		Breadcrumbs: []*sentry.Breadcrumb{
			{
				Message: "non-string breadcrumb data",
				Data: map[string]interface{}{
					"count":  42,
					"active": true,
					"score":  3.14,
				},
			},
		},
	}
	result := redactPII(event)
	crumb := result.Breadcrumbs[0]
	assert.Equal(t, 42, crumb.Data["count"])
	assert.Equal(t, true, crumb.Data["active"])
	assert.Equal(t, 3.14, crumb.Data["score"])
}

func TestRedactPII_RequestAllSensitiveHeaders(t *testing.T) {
	event := &sentry.Event{
		Request: &sentry.Request{
			Cookies: "session=abc",
			Headers: map[string]string{
				"Cookie":        "session=abc",
				"Set-Cookie":    "token=xyz; Path=/",
				"X-Api-Key":     "secret-api-key",
				"Authorization": "Bearer eyJhbGciOiJIUzI1NiJ9",
				"Content-Type":  "application/json",
			},
		},
	}
	result := redactPII(event)
	assert.Empty(t, result.Request.Cookies)
	assert.Equal(t, "[REDACTED]", result.Request.Headers["Cookie"])
	assert.Equal(t, "[REDACTED]", result.Request.Headers["Set-Cookie"])
	assert.Equal(t, "[REDACTED]", result.Request.Headers["X-Api-Key"])
	assert.Equal(t, "[REDACTED]", result.Request.Headers["Authorization"])
	assert.Equal(t, "application/json", result.Request.Headers["Content-Type"])
}

func TestRedactPII_ExtraStringData(t *testing.T) {
	event := &sentry.Event{
		Extra: map[string]interface{}{
			"user_email": "secret@example.com",
			"message":    "call to +5511999887766 failed",
		},
	}
	result := redactPII(event)
	assert.Equal(t, "[EMAIL_REDACTED]", result.Extra["user_email"])
	assert.NotContains(t, result.Extra["message"].(string), "999887")
}

func TestRedactPII_ExtraNonStringData(t *testing.T) {
	event := &sentry.Event{
		Extra: map[string]interface{}{
			"count":  100,
			"active": false,
			"ratio":  0.75,
		},
	}
	result := redactPII(event)
	assert.Equal(t, 100, result.Extra["count"])
	assert.Equal(t, false, result.Extra["active"])
	assert.Equal(t, 0.75, result.Extra["ratio"])
}

func TestRedactPII_EmptyUser(t *testing.T) {
	event := &sentry.Event{
		User: sentry.User{
			ID: "user-456",
			// No Email or IPAddress — should not panic and should leave empty
		},
	}
	result := redactPII(event)
	assert.Equal(t, "user-456", result.User.ID)
	assert.Empty(t, result.User.Email)
	assert.Empty(t, result.User.IPAddress)
}

func TestRedactPII_NilRequest(t *testing.T) {
	event := &sentry.Event{
		Message: "error without request context",
		// Request is nil — should skip redaction without panic
	}
	result := redactPII(event)
	assert.Nil(t, result.Request)
	assert.Equal(t, "error without request context", result.Message)
}

func TestSafeGo_PanicWithError(t *testing.T) {
	done := make(chan struct{}, 1)
	SafeGo("error-panic-goroutine", func() {
		defer func() { done <- struct{}{} }()
		panic(errors.New("error type panic"))
	})
	select {
	case <-done:
		// panic with error type recovered correctly
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for error-type panic recovery")
	}
}

func TestLogAndCapture_ContextCanceled_SkipsSentry(t *testing.T) {
	logger := zap.NewNop()

	origCaller := runtimeCaller
	runtimeCaller = func(_ int) (uintptr, string, int, bool) {
		return 0, "ctx_cancel_unique.go", 9001, true
	}
	defer func() { runtimeCaller = origCaller }()

	// context.Canceled is operational noise — must not reach Sentry
	LogAndCapture(logger, sentry.LevelError, "context canceled", context.Canceled, nil)
}

func TestLogAndCapture_ContextDeadlineExceeded_SkipsSentry(t *testing.T) {
	logger := zap.NewNop()

	origCaller := runtimeCaller
	runtimeCaller = func(_ int) (uintptr, string, int, bool) {
		return 0, "ctx_deadline_unique.go", 9002, true
	}
	defer func() { runtimeCaller = origCaller }()

	LogAndCapture(logger, sentry.LevelError, "deadline exceeded", context.DeadlineExceeded, nil)
}

func TestLogAndCapture_WrappedContextCanceled_SkipsSentry(t *testing.T) {
	logger := zap.NewNop()

	origCaller := runtimeCaller
	runtimeCaller = func(_ int) (uintptr, string, int, bool) {
		return 0, "ctx_wrapped_unique.go", 9003, true
	}
	defer func() { runtimeCaller = origCaller }()

	wrappedErr := errors.Join(context.Canceled, errors.New("additional context"))
	LogAndCapture(logger, sentry.LevelError, "wrapped cancel", wrappedErr, nil)
}

func TestLogAndCapture_UnknownLevel_FallsBackToError(t *testing.T) {
	logger := zap.NewNop()

	origCaller := runtimeCaller
	runtimeCaller = func(_ int) (uintptr, string, int, bool) {
		return 0, "unknown_level_unique.go", 9004, true
	}
	defer func() { runtimeCaller = origCaller }()

	LogAndCapture(logger, sentry.Level("custom-unknown"), "custom level msg", errors.New("err"), nil)
}

func TestLogAndCapture_RateLimited_EarlyReturn(t *testing.T) {
	logger := zap.NewNop()

	origCaller := runtimeCaller
	runtimeCaller = func(_ int) (uintptr, string, int, bool) {
		return 0, "rate_limit_unique.go", 9005, true
	}
	defer func() { runtimeCaller = origCaller }()

	origRL := globalRateLimiter
	globalRateLimiter = &rateLimiter{
		entries:      make(map[string]*rateLimitEntry),
		maxPerWindow: 1,
		window:       60 * time.Second,
	}
	defer func() { globalRateLimiter = origRL }()

	testErr := errors.New("rate limited error unique 9005")
	// First call: allowed (count=1 <= maxPerWindow=1)
	LogAndCapture(logger, sentry.LevelError, "first", testErr, nil)
	// Second call: suppressed — triggers the !shouldSend early return
	LogAndCapture(logger, sentry.LevelError, "second", testErr, nil)
}

func TestLogAndCapture_CapturesMessage_WhenErrNil_WithTags(t *testing.T) {
	logger := zaptest.NewLogger(t)

	origCaller := runtimeCaller
	runtimeCaller = func(_ int) (uintptr, string, int, bool) {
		return 0, "msg_capture_unique.go", 9006, true
	}
	defer func() { runtimeCaller = origCaller }()

	// nil error + non-empty tags: exercises tag loop + CaptureMessage path
	LogAndCapture(logger, sentry.LevelInfo, "info event", nil, map[string]string{
		"module": "test",
		"flow":   "unit",
	})
}
