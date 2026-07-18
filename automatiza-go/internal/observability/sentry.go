// Package observability provides Sentry integration for error tracking and
// structured logging with PII redaction and rate limiting.
package observability

import (
	"context"
	"errors"
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

// eventBudget bounds ALL events leaving this process, regardless of capture path
// (LogAndCapture, CaptureWithTags, SafeGo, sentrygin, asynq interceptor). The org
// quota is 5k errors/month and was burned within hours of each monthly reset,
// leaving Sentry deaf for the rest of the cycle. Enforced in BeforeSend — the one
// choke point every path crosses.
// ponytail: fixed 6/h ceiling (~4.3k/month worst case) with 10min per-signature
// dedup; genuinely distinct errors beyond the budget are dropped. Upgrade path:
// raise the org quota and delete this guard (or lift budgetMax).
type eventBudget struct {
	mu                sync.Mutex
	windowStart       time.Time
	used              int
	lastByFingerprint map[string]time.Time
}

const (
	budgetWindow     = time.Hour
	budgetMax        = 6
	fingerprintDedup = 10 * time.Minute
)

var globalEventBudget = &eventBudget{lastByFingerprint: make(map[string]time.Time)}

// allow reports whether an event with the given fingerprint fits the budget.
func (b *eventBudget) allow(fp string, now time.Time) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	if last, ok := b.lastByFingerprint[fp]; ok && now.Sub(last) < fingerprintDedup {
		return false
	}
	if now.Sub(b.windowStart) > budgetWindow {
		b.windowStart = now
		b.used = 0
	}
	if b.used >= budgetMax {
		return false
	}
	b.used++
	b.lastByFingerprint[fp] = now
	if len(b.lastByFingerprint) > 512 {
		for k, t := range b.lastByFingerprint {
			if now.Sub(t) > fingerprintDedup {
				delete(b.lastByFingerprint, k)
			}
		}
	}
	return true
}

// eventFingerprint derives a stable identity for dedup: first exception
// type+value when present, else the message.
func eventFingerprint(event *sentry.Event) string {
	if len(event.Exception) > 0 {
		return event.Exception[0].Type + ":" + event.Exception[0].Value
	}
	return event.Message
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
			// Drop events from development environments to avoid polluting production Sentry
			if cfg.Environment == "development" || cfg.Environment == "dev" {
				return nil
			}
			// Rede de segurança para capturas que não passam pelo LogAndCapture
			// (CaptureWithTags, CaptureMessage, interceptors): erro de estado do
			// usuário não entra no Sentry — ver userStateErrorMarkers.
			if isUserStateEvent(event) {
				return nil
			}
			// Orçamento global ANTES do envio: ruído já filtrado acima não consome
			// o teto; o que sobra respeita 6/h + dedup 10min por assinatura.
			if !globalEventBudget.allow(eventFingerprint(event), time.Now()) {
				return nil
			}
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

// userStateErrorMarkers identifica erros de ESTADO/CONFIG do próprio usuário
// (não são falhas da plataforma): credencial Shopee inválida/sem API, chip
// desconectado ou removido do grupo. Essas classes dominavam o volume do projeto
// (296k+ eventos de "error [10020]" até 02/07/2026) e mantinham a cota do Sentry
// permanentemente estourada — nenhum erro real era aceito (outcome rate_limited).
// Ficam fora do Sentry; o rastro operacional continua no zap e em dispatch_logs.
var userStateErrorMarkers = []string{
	"error [10020]",                     // Shopee: Invalid Credential
	"error [10035]",                     // Shopee: conta sem Open API
	"Invalid Signature",                 // Shopee: credencial errada
	"no Open API access",                // Shopee: acesso revogado
	"Session status is not as expected", // chip do usuário fora do ar
	"is not working: current status",    // idem (ProviderError)
	"instance not connected",            // idem (gate pré-envio)
	"not participating in that group",   // chip removido do grupo
}

func matchesUserStateMarker(s string) bool {
	if s == "" {
		return false
	}
	for _, m := range userStateErrorMarkers {
		if strings.Contains(s, m) {
			return true
		}
	}
	return false
}

// isUserStateError decide no ponto de captura (LogAndCapture).
func isUserStateError(msg string, err error) bool {
	if matchesUserStateMarker(msg) {
		return true
	}
	return err != nil && matchesUserStateMarker(err.Error())
}

// isUserStateEvent decide sobre o evento final (BeforeSend), cobrindo também
// capturas feitas fora do LogAndCapture.
func isUserStateEvent(event *sentry.Event) bool {
	if matchesUserStateMarker(event.Message) {
		return true
	}
	for i := range event.Exception {
		if matchesUserStateMarker(event.Exception[i].Value) {
			return true
		}
	}
	return false
}

// redactPII removes PII from Sentry events.
func redactPII(event *sentry.Event) *sentry.Event {
	// Redact message
	event.Message = redactString(event.Message)

	// Redact exception values (preserve type and stack frames)
	for i := range event.Exception {
		event.Exception[i].Value = redactString(event.Exception[i].Value)
	}

	// Redact breadcrumb messages and data WITHOUT mutating in place. sentry-go
	// shares breadcrumb pointers (and their Data maps) across concurrently-sent
	// events — they come from the same Hub/Scope. Writing them here from multiple
	// asynq goroutines crashed the worker with "fatal error: concurrent map
	// writes" (redactPII at sentry.go) under error storms. Build fresh copies and
	// point the event at those, leaving the shared originals untouched (we only
	// READ them, which is concurrency-safe).
	if len(event.Breadcrumbs) > 0 {
		cleaned := make([]*sentry.Breadcrumb, len(event.Breadcrumbs))
		for i, b := range event.Breadcrumbs {
			if b == nil {
				continue
			}
			cp := *b
			cp.Message = redactString(b.Message)
			if len(b.Data) > 0 {
				data := make(map[string]interface{}, len(b.Data))
				for k, v := range b.Data {
					if s, ok := v.(string); ok {
						data[k] = redactString(s)
					} else {
						data[k] = v
					}
				}
				cp.Data = data
			}
			cleaned[i] = &cp
		}
		event.Breadcrumbs = cleaned
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
	s = piiPatterns.phone.ReplaceAllStringFunc(s, maskPhonePartial)
	s = piiPatterns.bearer.ReplaceAllString(s, "[TOKEN_REDACTED]")
	s = piiPatterns.password.ReplaceAllString(s, "[SECRET_REDACTED]")
	return s
}

// maskPhonePartial preserves the first 5 characters (country + area code) and last 4,
// masking the middle portion. Example: "+5511999887766" -> "+5511*****7766".
// For short numbers (<= 9 chars) it falls back to full redaction.
func maskPhonePartial(phone string) string {
	if len(phone) <= 9 {
		return "[PHONE_REDACTED]"
	}
	return phone[:5] + strings.Repeat("*", len(phone)-9) + phone[len(phone)-4:]
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
	// Fallback to global logger if nil to prevent nil pointer panic
	if logger == nil {
		logger = zap.L()
	}

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

	// Erro de estado/config do usuário: já logado acima; não gasta cota do Sentry
	// (essa classe foi o flood que derrubou a ingestão até 02/07 — ver markers).
	if isUserStateError(msg, err) {
		return
	}

	// Rate limit key: file:line + error message
	_, file, line, _ := runtimeCaller(2)
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}
	rateLimitKey := fmt.Sprintf("%s:%d:%s", file, line, errMsg)

	shouldSend, _ := globalRateLimiter.shouldSend(rateLimitKey)
	if !shouldSend {
		return
	}

	hub := sentry.CurrentHub().Clone()
	hub.ConfigureScope(func(scope *sentry.Scope) {
		scope.SetLevel(level)
		for k, v := range tags {
			scope.SetTag(k, v)
		}
	})

	// Context cancellation is operational (client disconnect, shutdown) — log only, don't pollute Sentry
	if err != nil && (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)) {
		return
	}

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
