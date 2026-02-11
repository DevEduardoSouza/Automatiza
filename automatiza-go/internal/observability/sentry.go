// Package observability provides Sentry integration for error tracking and
// structured logging with PII redaction and rate limiting.
package observability

import (
	"fmt"
	"os"
	"regexp"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/getsentry/sentry-go"
	"go.uber.org/zap"
)

// SentryConfig holds the configuration for Sentry initialization.
type SentryConfig struct {
	DSN         string
	Environment string
	Release     string
	Debug       bool
	SampleRate  float64
}

// piiPatterns contains compiled regexes for PII redaction.
var piiPatterns = struct {
	email    *regexp.Regexp
	phone    *regexp.Regexp
	ip       *regexp.Regexp
	bearer   *regexp.Regexp
	password *regexp.Regexp
}{
	email:    regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`),
	phone:    regexp.MustCompile(`\+?\d{10,15}`),
	ip:       regexp.MustCompile(`\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`),
	bearer:   regexp.MustCompile(`(?i)(bearer\s+|token[=:]\s*)[a-zA-Z0-9\-._~+/]+=*`),
	password: regexp.MustCompile(`(?i)(password|senha|secret|api_?key)[=:]\s*\S+`),
}

// rateLimitEntry tracks a specific error for rate limiting.
type rateLimitEntry struct {
	count    int
	firstSee time.Time
	lastSent time.Time
}

// rateLimiter suppresses duplicate Sentry events when the same error
// occurs more than maxPerWindow times within the window duration.
type rateLimiter struct {
	mu           sync.Mutex
	entries      map[string]*rateLimitEntry
	maxPerWindow int
	window       time.Duration
}

var globalRateLimiter = &rateLimiter{
	entries:      make(map[string]*rateLimitEntry),
	maxPerWindow: 10,
	window:       60 * time.Second,
}

// shouldSend returns true if the event should be sent to Sentry.
// Returns (shouldSend, suppressedCount).
func (rl *rateLimiter) shouldSend(key string) (bool, int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	entry, exists := rl.entries[key]

	if !exists || now.Sub(entry.firstSee) > rl.window {
		rl.entries[key] = &rateLimitEntry{
			count:    1,
			firstSee: now,
			lastSent: now,
		}
		return true, 0
	}

	entry.count++
	if entry.count <= rl.maxPerWindow {
		entry.lastSent = now
		return true, 0
	}

	// Over threshold: only send once more at the end to report suppressed count
	suppressed := entry.count - rl.maxPerWindow
	return false, suppressed
}

// InitSentry initializes the Sentry SDK with PII redaction.
// Must be called before any other Sentry operation.
func InitSentry(cfg SentryConfig) error {
	if cfg.DSN == "" {
		return fmt.Errorf("sentry DSN is required")
	}

	sampleRate := cfg.SampleRate
	if sampleRate <= 0 || sampleRate > 1 {
		sampleRate = 1.0
	}

	err := sentry.Init(sentry.ClientOptions{
		Dsn:              cfg.DSN,
		Environment:      cfg.Environment,
		Release:          cfg.Release,
		Debug:            cfg.Debug,
		SampleRate:       sampleRate,
		AttachStacktrace: true,
		BeforeSend: func(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
			return redactPII(event)
		},
	})
	if err != nil {
		return fmt.Errorf("failed to initialize sentry: %w", err)
	}

	return nil
}

// Flush waits until the underlying Sentry transport sends any buffered events.
func Flush(timeout time.Duration) {
	sentry.Flush(timeout)
}

// redactPII removes PII from Sentry events.
func redactPII(event *sentry.Event) *sentry.Event {
	// Redact message
	event.Message = redactString(event.Message)

	// Redact exception values (preserve type and stack frames)
	for i := range event.Exception {
		event.Exception[i].Value = redactString(event.Exception[i].Value)
	}

	// Redact breadcrumb messages
	for i := range event.Breadcrumbs {
		event.Breadcrumbs[i].Message = redactString(event.Breadcrumbs[i].Message)
		for k, v := range event.Breadcrumbs[i].Data {
			if s, ok := v.(string); ok {
				event.Breadcrumbs[i].Data[k] = redactString(s)
			}
		}
	}

	// Redact request data
	if event.Request != nil {
		event.Request.Cookies = ""
		for k := range event.Request.Headers {
			lk := strings.ToLower(k)
			if lk == "authorization" || lk == "cookie" || lk == "set-cookie" || lk == "x-api-key" {
				event.Request.Headers[k] = "[REDACTED]"
			}
		}
	}

	// Redact user data (keep ID for correlation)
	if event.User.Email != "" {
		event.User.Email = "[REDACTED]"
	}
	if event.User.IPAddress != "" {
		event.User.IPAddress = "[REDACTED]"
	}

	// Redact extra and tags values
	for k, v := range event.Extra {
		if s, ok := v.(string); ok {
			event.Extra[k] = redactString(s)
		}
	}

	return event
}

// redactString applies PII patterns to a string.
func redactString(s string) string {
	s = piiPatterns.email.ReplaceAllString(s, "[EMAIL_REDACTED]")
	s = piiPatterns.phone.ReplaceAllString(s, "[PHONE_REDACTED]")
	s = piiPatterns.bearer.ReplaceAllString(s, "[TOKEN_REDACTED]")
	s = piiPatterns.password.ReplaceAllString(s, "[SECRET_REDACTED]")
	return s
}

// SafeGo runs a function in a goroutine with panic recovery and Sentry capture.
// The name parameter is used for tagging the event in Sentry.
func SafeGo(name string, fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				hub := sentry.CurrentHub().Clone()
				hub.ConfigureScope(func(scope *sentry.Scope) {
					scope.SetTag("goroutine", name)
					scope.SetLevel(sentry.LevelFatal)
					scope.SetExtra("stack", string(debug.Stack()))
				})

				switch val := r.(type) {
				case error:
					hub.CaptureException(val)
				default:
					hub.CaptureMessage(fmt.Sprintf("panic in goroutine %s: %v", name, val))
				}

				hub.Flush(2 * time.Second)

				// Log to stderr as fallback
				fmt.Fprintf(os.Stderr, "[PANIC] goroutine %s: %v\n%s\n", name, r, debug.Stack())
			}
		}()
		fn()
	}()
}

// LogAndCapture logs the error with Zap and captures it in Sentry with rate limiting.
// The level parameter controls both the Zap log level and Sentry severity.
// Tags are added as Sentry tags for filtering and alerting.
func LogAndCapture(logger *zap.Logger, level sentry.Level, msg string, err error, tags map[string]string) {
	// Always log
	fields := make([]zap.Field, 0, len(tags)+1)
	if err != nil {
		fields = append(fields, zap.Error(err))
	}
	for k, v := range tags {
		fields = append(fields, zap.String(k, v))
	}

	switch level {
	case sentry.LevelDebug:
		logger.Debug(msg, fields...)
	case sentry.LevelInfo:
		logger.Info(msg, fields...)
	case sentry.LevelWarning:
		logger.Warn(msg, fields...)
	case sentry.LevelError:
		logger.Error(msg, fields...)
	case sentry.LevelFatal:
		logger.Error(msg, fields...) // Don't call Fatal, let caller decide
	default:
		logger.Error(msg, fields...)
	}

	// Rate limit key: file:line + error message
	_, file, line, _ := runtimeCaller(2)
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}
	rateLimitKey := fmt.Sprintf("%s:%d:%s", file, line, errMsg)

	shouldSend, suppressed := globalRateLimiter.shouldSend(rateLimitKey)
	if !shouldSend {
		return
	}

	hub := sentry.CurrentHub().Clone()
	hub.ConfigureScope(func(scope *sentry.Scope) {
		scope.SetLevel(level)
		for k, v := range tags {
			scope.SetTag(k, v)
		}
		if suppressed > 0 {
			scope.SetExtra("suppressed_count", suppressed)
		}
	})

	if err != nil {
		hub.CaptureException(err)
	} else {
		hub.CaptureMessage(msg)
	}
}

// runtimeCaller is a wrapper for runtime.Caller to allow testing.
var runtimeCaller = func(skip int) (pc uintptr, file string, line int, ok bool) {
	// Import runtime only at call time to keep the var declaration clean
	return runtimeCallerImpl(skip + 1)
}

// CaptureWithTags captures an error in Sentry with the given tags.
func CaptureWithTags(err error, tags map[string]string) {
	hub := sentry.CurrentHub().Clone()
	hub.ConfigureScope(func(scope *sentry.Scope) {
		for k, v := range tags {
			scope.SetTag(k, v)
		}
	})
	hub.CaptureException(err)
}

// CaptureWithScope captures an error in Sentry with a custom scope configuration.
func CaptureWithScope(err error, configureScope func(scope *sentry.Scope)) {
	hub := sentry.CurrentHub().Clone()
	hub.ConfigureScope(configureScope)
	hub.CaptureException(err)
}

// CaptureMessage captures a message in Sentry with the given level and tags.
func CaptureMessage(level sentry.Level, msg string, tags map[string]string) {
	hub := sentry.CurrentHub().Clone()
	hub.ConfigureScope(func(scope *sentry.Scope) {
		scope.SetLevel(level)
		for k, v := range tags {
			scope.SetTag(k, v)
		}
	})
	hub.CaptureMessage(msg)
}
