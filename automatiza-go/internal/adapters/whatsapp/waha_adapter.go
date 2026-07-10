// Package whatsapp provides WhatsApp provider adapters.
// This file implements the WahaAdapter which translates between domain-neutral
// types and WAHA Plus API-specific formats.
//
// IMPORTANT: This adapter isolates all WAHA-specific details. The rest of the
// application only interacts with the WhatsAppProvider interface.
package whatsapp

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"automatiza-go/internal/lib/imagepad"
	"automatiza-go/internal/observability"

	"github.com/getsentry/sentry-go"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.uber.org/zap"
)

// =============================================================================
// Retry Configuration
// =============================================================================

// retryConfig holds configuration for HTTP request retries with exponential backoff.
type retryConfig struct {
	maxRetries int
	baseDelay  time.Duration
	maxDelay   time.Duration
}

// defaultRetryConfig returns the default retry configuration: 3 retries, 100ms-3s backoff.
func defaultRetryConfig() retryConfig {
	return retryConfig{
		maxRetries: 3,
		baseDelay:  100 * time.Millisecond,
		maxDelay:   3 * time.Second,
	}
}

// isRetryableError returns true if the error is a transient network error that can be retried.
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Timeout errors
	if errors.Is(err, context.DeadlineExceeded) {
		return false // don't retry context deadline exceeded from caller
	}

	// Network errors (DNS, connection refused, reset, etc.)
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout()
	}

	// Connection-level errors
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}

	// Check for common transient patterns in error strings
	errStr := err.Error()
	transientPatterns := []string{
		"connection refused",
		"connection reset",
		"broken pipe",
		"EOF",
		"no such host",
		"i/o timeout",
	}
	for _, pattern := range transientPatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}

	return false
}

// isRetryableStatusCode returns true if the HTTP status code indicates a transient server error.
func isRetryableStatusCode(statusCode int) bool {
	switch statusCode {
	case http.StatusTooManyRequests,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

// transientUploadCodes are the upstream WhatsApp media-upload status codes that
// WAHA wraps inside an HTTP 500 and that get the full retry budget: 429 is rate
// limiting, 5xx are upstream blips. A wrapped 400 is handled separately, with a
// capped retry — see isTransientUpload400.
var transientUploadCodes = []string{"429", "500", "502", "503", "504"}

// softUploadMaxRetries caps how many times a wrapped-400 media upload (see
// isTransientUpload400) is retried, independent of the full retry budget used
// for 429/5xx. Kept at 1 because 400 is by far the highest-volume upload
// failure, and retrying every one at full budget would pile upload load onto
// the already-saturated single WAHA container — risking the 429s the full
// retry path is meant to relieve.
const softUploadMaxRetries = 1

// isTransientWahaUploadError reports whether a WAHA HTTP 500 response wraps a
// transient WhatsApp media-upload failure, e.g.:
//
//	{"statusCode":500,...,"message":"2 UNKNOWN: upload failed with status code 429"}
//
// These surface as HTTP 500 from WAHA, so the plain status-code retry check
// (isRetryableStatusCode) misses them and the send fails immediately —
// dropping the image and forcing the message worker into its text-only
// fallback ("promoção sem foto"). Matching the wrapped upstream code lets the
// existing backoff retry recover the upload. "not participating in that group"
// 500s do not match this pattern, so dead-group sends are unaffected.
func isTransientWahaUploadError(statusCode int, body []byte) bool {
	if statusCode != http.StatusInternalServerError {
		return false
	}
	s := string(body)
	for _, code := range transientUploadCodes {
		if strings.Contains(s, "upload failed with status code "+code) {
			return true
		}
	}
	return false
}

// isTransientUpload400 reports whether a WAHA HTTP 500 wraps a WhatsApp
// media-upload 400, e.g.:
//
//	{"statusCode":500,...,"message":"2 UNKNOWN: upload failed with status code 400"}
//
// The original retry path deliberately excluded 400 on the assumption it meant
// malformed/rejected media. Production data contradicts that: the *same* valid
// JPEG uploads fine 100+ times and 400s only occasionally under load — i.e.
// WhatsApp's media server returns a transient 400, not a permanent rejection,
// and the send drops to the text-only fallback ("promoção sem foto") for no
// good reason. So these get a (capped, see softUploadMaxRetries) retry. A
// genuinely bad image still exhausts the retry and falls back to text, exactly
// as before — only a few wasted attempts later.
func isTransientUpload400(statusCode int, body []byte) bool {
	return statusCode == http.StatusInternalServerError &&
		strings.Contains(string(body), "upload failed with status code 400")
}

// backoffDelay calculates the delay for a given retry attempt using exponential backoff with jitter.
func backoffDelay(attempt int, cfg retryConfig) time.Duration {
	delay := float64(cfg.baseDelay) * math.Pow(2, float64(attempt))
	if delay > float64(cfg.maxDelay) {
		delay = float64(cfg.maxDelay)
	}
	// Add jitter: random value between 0 and delay
	jitter := rand.Float64() * delay
	return time.Duration(jitter)
}

// maxDownloadBodySize is the maximum allowed response body size for download operations (50 MB).
const maxDownloadBodySize int64 = 50 * 1024 * 1024

// =============================================================================
// Configuration
// =============================================================================

// WahaAdapterConfig contains configuration for the WAHA adapter.
type WahaAdapterConfig struct {
	// APIURL is the base URL for the WAHA API (e.g., "http://localhost:3000").
	APIURL string

	// APIKey is the API key for authenticating with WAHA.
	APIKey string

	// WebhookBaseURL is the base URL for webhook callbacks (e.g., "https://api.example.com").
	WebhookBaseURL string

	// WebhookSecret is the HMAC secret for webhook payload validation.
	WebhookSecret string

	// HTTPTimeout is the timeout for HTTP requests. Defaults to 30 seconds.
	HTTPTimeout time.Duration

	// StatusCacheTTL is how long to cache session status. Defaults to 5 seconds.
	StatusCacheTTL time.Duration
}

// validate checks if the configuration is valid.
func (c *WahaAdapterConfig) validate() error {
	if c.APIURL == "" {
		return errors.New("APIURL is required")
	}
	if c.APIKey == "" {
		return errors.New("APIKey is required")
	}
	return nil
}

// =============================================================================
// WAHA Adapter Implementation
// =============================================================================

// WahaAdapter implements WhatsAppProvider for WAHA Plus API.
type WahaAdapter struct {
	apiURL         string
	apiKey         string
	webhookBaseURL string
	webhookSecret  string
	httpClient     *http.Client
	logger         *zap.Logger
	mapper         *Mapper

	// Status cache to reduce API calls
	statusCache    map[string]*statusCacheEntry
	statusCacheMu  sync.RWMutex
	statusCacheTTL time.Duration

	// Per-session circuit breakers for status checks
	circuitBreakers map[string]*circuitBreaker
	cbMu            sync.RWMutex
	cbThreshold     int
	cbBackoffTime   time.Duration

	// retryEnabled controls whether network retries with backoff are enabled.
	// Controlled by WAHA_RETRY_ENABLED env var (default: true).
	retryEnabled bool
	retryCfg     retryConfig

	// uploadPacer, when set, spaces media uploads per session before they hit
	// WAHA, to keep them under WhatsApp's media-server rate limit. nil = off.
	uploadPacer UploadPacer

	// mediaJitter, when true, re-encodes URL-fetched product images with a tiny
	// per-chat quality delta so the same photo isn't broadcast with a byte-
	// identical hash to every group (anti-ban). Relayed/forwarded base64 media is
	// NEVER jittered (it must stay byte-for-byte identical to the source).
	// Controlled by HUMANIZE_MEDIA_JITTER (default: true).
	mediaJitter bool
}

// SetUploadPacer installs (or clears, with nil) the per-session media-upload
// pacer. Wired at startup from MEDIA_PACE_MS; off by default. Safe to call once
// before the adapter starts serving.
func (a *WahaAdapter) SetUploadPacer(p UploadPacer) {
	a.uploadPacer = p
}

// circuitBreaker implements circuit breaker pattern for status checks.
// After threshold consecutive failures, requests are blocked for backoffTime.
type circuitBreaker struct {
	mu          sync.Mutex
	failures    int
	lastFailure time.Time
	threshold   int           // Number of failures before opening circuit
	backoffTime time.Duration // How long to wait before allowing requests again
}

// newCircuitBreaker creates a new circuit breaker.
func newCircuitBreaker(threshold int, backoffTime time.Duration) *circuitBreaker {
	return &circuitBreaker{
		threshold:   threshold,
		backoffTime: backoffTime,
	}
}

// canRequest checks if a request is allowed.
func (cb *circuitBreaker) canRequest() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.failures >= cb.threshold {
		if time.Since(cb.lastFailure) < cb.backoffTime {
			return false // Circuit is open
		}
		cb.failures = 0 // Reset after backoff period
	}
	return true
}

// recordFailure records a failed request.
func (cb *circuitBreaker) recordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures++
	cb.lastFailure = time.Now()
}

// recordSuccess records a successful request and resets the counter.
func (cb *circuitBreaker) recordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures = 0
}

// getOrCreateCircuitBreaker returns the circuit breaker for the given session,
// creating one lazily if it doesn't exist (double-checked locking).
func (a *WahaAdapter) getOrCreateCircuitBreaker(sessionName string) *circuitBreaker {
	// Fast path: read lock
	a.cbMu.RLock()
	cb, ok := a.circuitBreakers[sessionName]
	a.cbMu.RUnlock()
	if ok {
		return cb
	}

	// Slow path: write lock + re-check
	a.cbMu.Lock()
	defer a.cbMu.Unlock()
	cb, ok = a.circuitBreakers[sessionName]
	if ok {
		return cb
	}
	cb = newCircuitBreaker(a.cbThreshold, a.cbBackoffTime)
	a.circuitBreakers[sessionName] = cb
	return cb
}

// ClearCircuitBreaker removes the circuit breaker for a specific session.
func (a *WahaAdapter) ClearCircuitBreaker(sessionName string) {
	a.cbMu.Lock()
	defer a.cbMu.Unlock()
	delete(a.circuitBreakers, sessionName)
}

// ClearAllCircuitBreakers removes all per-session circuit breakers.
func (a *WahaAdapter) ClearAllCircuitBreakers() {
	a.cbMu.Lock()
	defer a.cbMu.Unlock()
	a.circuitBreakers = make(map[string]*circuitBreaker)
}

// statusCacheEntry holds cached session status with expiration.
type statusCacheEntry struct {
	info      *SessionInfo
	expiresAt time.Time
}

// isDockerInternalURL returns true if the URL points to a Docker-internal or
// loopback address where HTTP is acceptable (no TLS needed).
func isDockerInternalURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := parsed.Hostname()
	// Docker service names have no dots (e.g. "waha", "redis")
	if !strings.Contains(host, ".") {
		return true
	}
	// Well-known internal hostnames
	switch host {
	case "localhost", "host.docker.internal", "gateway.docker.internal":
		return true
	}
	// Loopback IPs
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return true
	}
	return false
}

// NewWahaAdapter creates a new WAHA adapter with the given configuration.
func NewWahaAdapter(cfg WahaAdapterConfig, logger *zap.Logger) (*WahaAdapter, error) {
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	if logger == nil {
		logger = zap.NewNop()
	}

	namedLogger := logger.Named("waha-adapter")

	// W39: HTTPS enforcement in production — warn only for public-facing URLs.
	// Docker-internal URLs (http://waha:3000, http://host.docker.internal:...) are safe.
	if os.Getenv("NODE_ENV") == "production" && !strings.HasPrefix(cfg.APIURL, "https://") && !isDockerInternalURL(cfg.APIURL) {
		namedLogger.Warn("WAHA API URL is not HTTPS in production — communication is not encrypted",
			zap.String("url", cfg.APIURL))
		observability.CaptureMessage(sentry.LevelWarning, "WAHA API URL is not HTTPS in production — communication is not encrypted",
			map[string]string{"module": "waha-adapter", "security": "no-tls", "url": cfg.APIURL})
	}

	// W40: Alert Sentry Critical when HMAC is disabled in production
	if os.Getenv("NODE_ENV") == "production" && cfg.WebhookSecret == "" {
		observability.CaptureMessage(sentry.LevelFatal, "HMAC validation disabled in production — webhooks accept payloads from ANY source",
			map[string]string{"module": "waha-adapter", "security": "hmac-disabled"})
	}

	httpTimeout := cfg.HTTPTimeout
	if httpTimeout == 0 {
		httpTimeout = 30 * time.Second
	}

	statusCacheTTL := cfg.StatusCacheTTL
	if statusCacheTTL == 0 {
		statusCacheTTL = 30 * time.Second
	}

	// C6: Configure HTTP transport with connection pool limits
	// OTel: wrap with otelhttp for distributed tracing of external HTTP calls
	transport := otelhttp.NewTransport(&http.Transport{
		MaxIdleConns:    100,
		MaxConnsPerHost: 20,
		IdleConnTimeout: 90 * time.Second,
	})

	// N3: Retry enabled by default, controllable via env var for rollback
	retryEnabled := true
	if v := os.Getenv("WAHA_RETRY_ENABLED"); v == "false" || v == "0" {
		retryEnabled = false
		namedLogger.Info("WAHA retry with backoff is DISABLED via WAHA_RETRY_ENABLED env var")
	}

	// Anti-ban: per-recipient media-hash jitter on URL-fetched product images.
	// Default ON; disable with HUMANIZE_MEDIA_JITTER=false/0.
	mediaJitter := true
	if v := os.Getenv("HUMANIZE_MEDIA_JITTER"); v == "false" || v == "0" {
		mediaJitter = false
	}

	return &WahaAdapter{
		mediaJitter:    mediaJitter,
		apiURL:         cfg.APIURL,
		apiKey:         cfg.APIKey,
		webhookBaseURL: cfg.WebhookBaseURL,
		webhookSecret:  cfg.WebhookSecret,
		httpClient: &http.Client{
			Timeout:   httpTimeout,
			Transport: transport,
		},
		logger:          namedLogger,
		mapper:          newMapperWithLogger(namedLogger),
		statusCache:     make(map[string]*statusCacheEntry),
		statusCacheTTL:  statusCacheTTL,
		circuitBreakers: make(map[string]*circuitBreaker),
		cbThreshold:     3,                // 3 failures before opening circuit
		cbBackoffTime:   60 * time.Second, // 60s backoff (aligned with watchdog CheckInterval)
		retryEnabled:    retryEnabled,
		retryCfg:        defaultRetryConfig(),
	}, nil
}

// Compile-time check that WahaAdapter implements WhatsAppProvider.
var _ WhatsAppProvider = (*WahaAdapter)(nil)

// =============================================================================
// HTTP Helper Methods
// =============================================================================

// doRequest performs an HTTP request to the WAHA API with optional retry on transient errors.
func (a *WahaAdapter) doRequest(ctx context.Context, method, path string, body interface{}) ([]byte, error) {
	// Marshal body once (if present) so retries reuse the same bytes
	var jsonBody []byte
	if body != nil {
		var err error
		jsonBody, err = json.Marshal(body)
		if err != nil {
			return nil, NewProviderError(ErrCodeInvalidInput, "failed to marshal request body", err)
		}
	}

	reqURL := a.apiURL + path
	maxAttempts := 1
	if a.retryEnabled {
		maxAttempts = a.retryCfg.maxRetries + 1
	}

	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			// Wait with exponential backoff + jitter before retrying
			delay := backoffDelay(attempt-1, a.retryCfg)
			a.logger.Warn("retrying WAHA request after transient error",
				zap.String("method", method),
				zap.String("path", path),
				zap.Int("attempt", attempt+1),
				zap.Duration("delay", delay),
				zap.Error(lastErr))

			select {
			case <-ctx.Done():
				return nil, NewProviderError(ErrCodeTimeout, "request canceled during retry backoff", ctx.Err())
			case <-time.After(delay):
			}
		}

		var bodyReader io.Reader
		if jsonBody != nil {
			bodyReader = bytes.NewReader(jsonBody)
		}

		req, err := http.NewRequestWithContext(ctx, method, reqURL, bodyReader)
		if err != nil {
			return nil, NewProviderError(ErrCodeNetworkError, "failed to create request", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Api-Key", a.apiKey)

		resp, err := a.httpClient.Do(req)
		if err != nil {
			lastErr = err

			// Don't retry on context cancellation from the caller
			if errors.Is(err, context.Canceled) {
				return nil, NewProviderError(ErrCodeTimeout, "request canceled", err)
			}
			if errors.Is(err, context.DeadlineExceeded) {
				// Tell the per-request httpClient timeout apart from the
				// CALLER's deadline: when the caller's ctx is still alive, the
				// client timeout fired because WAHA/WhatsApp's media upload was
				// momentarily slow (>httpTimeout). That's transient, so retry
				// once (capped) instead of dropping the image straight to the
				// text-only fallback ("promoção sem foto") — the dominant cause
				// of "sem foto" once the 429/400 codes are retried. If the
				// caller's ctx is done, it really gave up, so return.
				if a.retryEnabled && ctx.Err() == nil && attempt < softUploadMaxRetries {
					lastErr = err
					continue
				}
				return nil, NewProviderError(ErrCodeTimeout, "request timed out", err)
			}

			// Retry on transient network errors
			if a.retryEnabled && attempt < maxAttempts-1 && isRetryableError(err) {
				continue
			}

			return nil, NewProviderError(ErrCodeNetworkError, "request failed", err)
		}

		respBody, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			if a.retryEnabled && attempt < maxAttempts-1 {
				continue
			}
			return nil, NewProviderError(ErrCodeNetworkError, "failed to read response", readErr)
		}

		// Retry on transient HTTP status codes (429, 502, 503, 504), and on
		// HTTP 500 responses that wrap a transient upstream media-upload
		// failure (e.g. "upload failed with status code 429"). These are the
		// dominant cause of images being dropped to the text-only fallback
		// under load.
		if a.retryEnabled && attempt < maxAttempts-1 {
			if isRetryableStatusCode(resp.StatusCode) || isTransientWahaUploadError(resp.StatusCode, respBody) {
				lastErr = fmt.Errorf("transient HTTP %d from WAHA", resp.StatusCode)
				continue
			}
			// Wrapped media-upload 400: transient on WhatsApp's side too, but
			// the highest-volume failure, so retried at most softUploadMaxRetries
			// times to recover without overloading WAHA.
			if attempt < softUploadMaxRetries && isTransientUpload400(resp.StatusCode, respBody) {
				lastErr = fmt.Errorf("transient wrapped-400 media upload from WAHA")
				continue
			}
		}

		// Log response for all send operations
		if strings.HasPrefix(path, "/api/send") {
			a.logger.Info("WAHA API response OK",
				zap.String("path", path),
				zap.Int("statusCode", resp.StatusCode))
		}

		if resp.StatusCode >= 400 {
			return nil, a.handleErrorResponse(resp.StatusCode, path, respBody)
		}

		return respBody, nil
	}

	// Exhausted all retries
	return nil, NewProviderError(ErrCodeNetworkError, fmt.Sprintf("request failed after %d attempts", maxAttempts), lastErr)
}

// handleErrorResponse converts HTTP error responses to ProviderError.
func (a *WahaAdapter) handleErrorResponse(statusCode int, path string, body []byte) *ProviderError {
	a.logger.Warn("WAHA API error",
		zap.Int("status", statusCode),
		zap.String("path", path),
		zap.String("response", string(body)))

	// Try to parse WAHA error response
	var wahaErr wahaErrorResponse
	if err := json.Unmarshal(body, &wahaErr); err == nil && (wahaErr.Message != "" || wahaErr.Error != "") {
		// Use HTTP status code as fallback when WAHA doesn't include code in response
		if wahaErr.Code == 0 {
			wahaErr.Code = statusCode
		}
		return a.mapper.ErrorFromWaha(&wahaErr, nil).WithDetails(map[string]any{
			"statusCode": statusCode,
			"path":       path,
		})
	}

	// Generic error mapping based on HTTP status
	switch statusCode {
	case http.StatusNotFound:
		return NewProviderError(ErrCodeSessionNotFound, "resource not found", nil)
	case http.StatusUnauthorized, http.StatusForbidden:
		return NewProviderError(ErrCodePermissionDenied, "authentication failed", nil)
	case http.StatusTooManyRequests:
		return NewProviderError(ErrCodeRateLimited, "rate limited", nil)
	case http.StatusBadRequest:
		return NewProviderError(ErrCodeInvalidInput, "bad request", nil)
	default:
		return NewProviderError(ErrCodeProviderError, fmt.Sprintf("WAHA error %d: %s", statusCode, string(body)), nil)
	}
}

// =============================================================================
// SessionManager Implementation
// =============================================================================

// CreateSession creates a new WhatsApp session with the specified configuration.
// If no webhook URL is available (neither from input nor from adapter config),
// the webhooks config is omitted so WAHA falls back to its container-level
// defaults (WHATSAPP_HOOK_URL / WHATSAPP_HOOK_EVENTS env vars).
func (a *WahaAdapter) CreateSession(ctx context.Context, input CreateSessionInput) (*SessionInfo, error) {
	webhookURL := input.WebhookURL
	if webhookURL == "" && a.webhookBaseURL != "" {
		webhookURL = a.webhookBaseURL + "/api/v1/webhooks/waha"
	}

	// Build GOWS engine storage config
	gowsConfig := map[string]interface{}{
		"storage": map[string]interface{}{
			"messages": input.Engine.StorageMessages,
			"groups":   input.Engine.StorageGroups,
			"chats":    input.Engine.StorageChats,
			"labels":   input.Engine.StorageLabels,
		},
	}

	// Build session config — only include webhooks when we have a valid URL.
	// Without a URL, WAHA uses its global defaults from env vars.
	sessionCfg := map[string]interface{}{
		"gows": gowsConfig,
	}

	if webhookURL != "" {
		events := input.WebhookEvents
		if len(events) == 0 {
			events = []string{"message", "message.ack", "session.status"}
		}

		wh := map[string]interface{}{
			"url":    webhookURL,
			"events": events,
		}

		hmacKey := input.HMACKey
		if hmacKey == "" {
			hmacKey = a.webhookSecret
		}
		if hmacKey != "" {
			wh["hmac"] = map[string]interface{}{"key": hmacKey}
		}

		sessionCfg["webhooks"] = []map[string]interface{}{wh}
	}

	payload := map[string]interface{}{
		"name":   input.SessionName,
		"start":  true,
		"config": sessionCfg,
	}

	resp, err := a.doRequest(ctx, http.MethodPost, "/api/sessions", payload)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	var wahaSession wahaSession
	if err := json.Unmarshal(resp, &wahaSession); err != nil {
		return nil, NewProviderError(ErrCodeProviderError, "failed to parse session response", err)
	}

	return a.mapper.SessionInfoFromWaha(&wahaSession), nil
}

// DeleteSession permanently removes a WhatsApp session.
func (a *WahaAdapter) DeleteSession(ctx context.Context, sessionName string) error {
	_, err := a.doRequest(ctx, http.MethodDelete, "/api/sessions/"+url.PathEscape(sessionName), nil)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	// Clear from cache
	a.ClearStatusCache(sessionName)
	a.ClearCircuitBreaker(sessionName)

	return nil
}

// GetSessionStatus retrieves the current status of a WhatsApp session.
func (a *WahaAdapter) GetSessionStatus(ctx context.Context, sessionName string) (*SessionInfo, error) {
	// Check cache first
	if info := a.getStatusFromCache(sessionName); info != nil {
		return info, nil
	}

	// Check per-session circuit breaker
	cb := a.getOrCreateCircuitBreaker(sessionName)
	if !cb.canRequest() {
		a.logger.Warn("circuit breaker open, blocking status check",
			zap.String("session", sessionName))
		return nil, NewProviderError(ErrCodeCircuitBreakerOpen, "service temporarily unavailable (circuit breaker open)", nil)
	}

	// N2: 10s timeout for status checks
	statusCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	resp, err := a.doRequest(statusCtx, http.MethodGet, "/api/sessions/"+url.PathEscape(sessionName), nil)
	if err != nil {
		cb.recordFailure()
		return nil, fmt.Errorf("failed to get session status: %w", err)
	}

	var wahaSession wahaSession
	if err := json.Unmarshal(resp, &wahaSession); err != nil {
		cb.recordFailure()
		return nil, NewProviderError(ErrCodeProviderError, "failed to parse session response", err)
	}

	// Success - reset circuit breaker
	cb.recordSuccess()

	info := a.mapper.SessionInfoFromWaha(&wahaSession)

	// Cache the status
	a.cacheStatus(sessionName, info)

	return info, nil
}

// GetQRCode retrieves the QR code for session authentication.
// Uses format=image to get binary PNG data from WAHA, then base64-encodes it
// into a data URI suitable for frontend <img src="..."> display.
func (a *WahaAdapter) GetQRCode(ctx context.Context, sessionName string) (*QRCodeData, error) {
	// Use direct HTTP call (not doRequest) because format=image returns binary PNG,
	// not JSON. We need to base64-encode the raw bytes ourselves.
	reqURL := a.apiURL + "/api/" + url.PathEscape(sessionName) + "/auth/qr?format=image"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, NewProviderError(ErrCodeNetworkError, "failed to create QR request", err)
	}
	req.Header.Set("X-Api-Key", a.apiKey)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get QR code: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, NewProviderError(ErrCodeNetworkError, "failed to read QR response", err)
	}

	if resp.StatusCode >= 400 {
		// QR not available is not an error — session might be already authenticated
		// or still starting up
		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusUnprocessableEntity {
			return nil, nil
		}
		return nil, a.handleErrorResponse(resp.StatusCode, reqURL, body)
	}

	// Determine content type — WAHA returns image/png for format=image
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/png"
	}

	// Base64-encode the binary PNG into a data URI for frontend display
	b64 := base64.StdEncoding.EncodeToString(body)
	dataURI := "data:" + contentType + ";base64," + b64

	return &QRCodeData{
		Value:       dataURI,
		Base64Image: dataURI,
		Mimetype:    contentType,
	}, nil
}

// RestartSession restarts an existing WhatsApp session.
// Uses the dedicated /restart endpoint per WAHA OpenAPI spec for atomic restart.
func (a *WahaAdapter) RestartSession(ctx context.Context, sessionName string) error {
	// Use the dedicated restart endpoint (POST /api/sessions/{session}/restart)
	// This is more reliable than stop+start as it's atomic
	_, err := a.doRequest(ctx, http.MethodPost, "/api/sessions/"+url.PathEscape(sessionName)+"/restart", nil)
	if err != nil {
		return fmt.Errorf("failed to restart session: %w", err)
	}

	// Clear cache so next status check gets fresh data
	a.ClearStatusCache(sessionName)
	a.ClearCircuitBreaker(sessionName)

	return nil
}

// LogoutSession logs out the WhatsApp session.
func (a *WahaAdapter) LogoutSession(ctx context.Context, sessionName string) error {
	_, err := a.doRequest(ctx, http.MethodPost, "/api/sessions/"+url.PathEscape(sessionName)+"/logout", nil)
	if err != nil {
		return fmt.Errorf("failed to logout session: %w", err)
	}

	// Clear cache
	a.ClearStatusCache(sessionName)
	a.ClearCircuitBreaker(sessionName)

	return nil
}

// =============================================================================
// GroupManager Implementation
// =============================================================================

// GetGroups retrieves all groups for a session.
func (a *WahaAdapter) GetGroups(ctx context.Context, sessionName string) ([]GroupInfo, error) {
	resp, err := a.doRequest(ctx, http.MethodGet, "/api/"+url.PathEscape(sessionName)+"/groups", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get groups: %w", err)
	}

	var wahaGroups []wahaGroup
	if err := json.Unmarshal(resp, &wahaGroups); err != nil {
		return nil, NewProviderError(ErrCodeProviderError, "failed to parse groups response", err)
	}

	return a.mapper.GroupInfosFromWaha(wahaGroups), nil
}

// GetGroupParticipants retrieves participants for a specific group.
// Uses the v2 endpoint which returns the pn (phone number) field alongside id.
// For participants with empty pn (common with LID-based IDs), resolves phone numbers
// via the WAHA LID mapping endpoint.
func (a *WahaAdapter) GetGroupParticipants(ctx context.Context, sessionName, groupID string) ([]ParticipantInfo, error) {
	path := "/api/" + url.PathEscape(sessionName) + "/groups/" + url.PathEscape(groupID) + "/participants/v2"
	resp, err := a.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get group participants: %w", err)
	}

	// Log raw response for debugging admin detection
	if len(resp) < 2000 {
		a.logger.Debug("participants/v2 raw response",
			zap.String("groupID", groupID),
			zap.String("body", string(resp)))
	} else {
		a.logger.Debug("participants/v2 raw response (truncated)",
			zap.String("groupID", groupID),
			zap.String("body", string(resp[:2000])))
	}

	var wahaParticipants []wahaParticipant
	if err := json.Unmarshal(resp, &wahaParticipants); err != nil {
		return nil, NewProviderError(ErrCodeProviderError, "failed to parse participants response", err)
	}

	// Check if any participants have unresolved phone numbers (empty pn with @lid ID)
	needsLidResolve := false
	for i := range wahaParticipants {
		if wahaParticipants[i].PhoneNumber == "" && strings.HasSuffix(wahaParticipants[i].ID, "@lid") {
			needsLidResolve = true
			break
		}
	}

	// Resolve LIDs to phone numbers in a single batch call
	if needsLidResolve {
		lidMap := a.fetchLidMappings(ctx, sessionName)
		for i := range wahaParticipants {
			if wahaParticipants[i].PhoneNumber == "" {
				if pn, ok := lidMap[wahaParticipants[i].ID]; ok {
					wahaParticipants[i].PhoneNumber = pn
				}
			}
		}
	}

	// Log LID resolution results
	resolved := 0
	for i := range wahaParticipants {
		if wahaParticipants[i].PhoneNumber != "" {
			resolved++
		}
	}
	a.logger.Debug("LID resolution complete",
		zap.String("groupID", groupID),
		zap.Int("total", len(wahaParticipants)),
		zap.Int("withPhone", resolved))

	return a.mapper.ParticipantInfosFromWaha(wahaParticipants), nil
}

// fetchLidMappings fetches all known LID-to-phone-number mappings for a session.
// Returns a map of LID → phone number (e.g. "56243167223861@lid" → "558191993610@c.us").
// On error, returns an empty map (non-fatal — participants will just show the LID).
func (a *WahaAdapter) fetchLidMappings(ctx context.Context, sessionName string) map[string]string {
	path := "/api/" + url.PathEscape(sessionName) + "/lids?limit=10000"
	resp, err := a.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		a.logger.Warn("failed to fetch LID mappings", zap.Error(err))
		return nil
	}

	var mappings []wahaLidMapping
	if err := json.Unmarshal(resp, &mappings); err != nil {
		a.logger.Warn("failed to parse LID mappings", zap.Error(err))
		return nil
	}

	a.logger.Debug("LID mappings fetched",
		zap.String("session", sessionName),
		zap.Int("count", len(mappings)))

	lidMap := make(map[string]string, len(mappings))
	for _, m := range mappings {
		if m.PN != "" {
			lidMap[m.LID] = m.PN
		}
	}
	return lidMap
}

// UpdateGroupSubject updates the group name/subject.
func (a *WahaAdapter) UpdateGroupSubject(ctx context.Context, input UpdateGroupSubjectInput) error {
	payload := a.mapper.UpdateGroupSubjectToWaha(&input)

	path := "/api/" + url.PathEscape(input.SessionName) + "/groups/" + url.PathEscape(input.GroupID) + "/subject"
	_, err := a.doRequest(ctx, http.MethodPut, path, payload)
	if err != nil {
		return fmt.Errorf("failed to update group subject: %w", err)
	}

	return nil
}

// UpdateGroupDescription updates the group description.
func (a *WahaAdapter) UpdateGroupDescription(ctx context.Context, input UpdateGroupDescriptionInput) error {
	payload := a.mapper.UpdateGroupDescriptionToWaha(&input)

	path := "/api/" + url.PathEscape(input.SessionName) + "/groups/" + url.PathEscape(input.GroupID) + "/description"
	_, err := a.doRequest(ctx, http.MethodPut, path, payload)
	if err != nil {
		return fmt.Errorf("failed to update group description: %w", err)
	}

	return nil
}

// UpdateGroupPicture updates the group profile picture.
func (a *WahaAdapter) UpdateGroupPicture(ctx context.Context, input UpdateGroupPictureInput) error {
	payload := a.mapper.UpdateGroupPictureToWaha(&input)

	path := "/api/" + url.PathEscape(input.SessionName) + "/groups/" + url.PathEscape(input.GroupID) + "/picture"
	_, err := a.doRequest(ctx, http.MethodPut, path, payload)
	if err != nil {
		return fmt.Errorf("failed to update group picture: %w", err)
	}

	return nil
}

// SetGroupAnnouncement toggles announcement mode (admin-only messages).
func (a *WahaAdapter) SetGroupAnnouncement(ctx context.Context, input SetGroupAnnouncementInput) error {
	payload := a.mapper.SetGroupAnnouncementToWaha(&input)

	path := "/api/" + url.PathEscape(input.SessionName) + "/groups/" + url.PathEscape(input.GroupID) + "/settings/security/messages-admin-only"
	_, err := a.doRequest(ctx, http.MethodPut, path, payload)
	if err != nil {
		return fmt.Errorf("failed to set group announcement: %w", err)
	}

	return nil
}

// GetGroupInviteCode retrieves the invite code for a specific group.
func (a *WahaAdapter) GetGroupInviteCode(ctx context.Context, sessionName, groupID string) (string, error) {
	path := "/api/" + url.PathEscape(sessionName) + "/groups/" + url.PathEscape(groupID) + "/invite-code"
	resp, err := a.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return "", fmt.Errorf("failed to get group invite code: %w", err)
	}

	// WAHA can return either JSON {"inviteCode": "..."} or plain text URL "https://chat.whatsapp.com/..."
	respStr := strings.TrimSpace(string(resp))

	// Try to parse as JSON first
	var wahaResp wahaInviteCodeResponse
	if err := json.Unmarshal(resp, &wahaResp); err == nil && wahaResp.InviteCode != "" {
		return wahaResp.InviteCode, nil
	}

	// If not JSON, try to extract invite code from URL
	// Format: https://chat.whatsapp.com/INVITE_CODE
	if strings.Contains(respStr, "chat.whatsapp.com/") {
		parts := strings.Split(respStr, "chat.whatsapp.com/")
		if len(parts) >= 2 {
			inviteCode := strings.TrimSpace(parts[1])
			// Remove any trailing characters like quotes or whitespace
			inviteCode = strings.Trim(inviteCode, "\"' \n\r\t")
			if inviteCode != "" {
				return inviteCode, nil
			}
		}
	}

	// If response looks like a plain invite code (alphanumeric)
	if len(respStr) > 0 && len(respStr) < 50 {
		// Clean up the response
		inviteCode := strings.Trim(respStr, "\"' \n\r\t")
		if inviteCode != "" {
			return inviteCode, nil
		}
	}

	return "", NewProviderError(ErrCodeProviderError, "failed to parse invite code response", nil)
}

// AddGroupMembers adds one or more members to a group.
func (a *WahaAdapter) AddGroupMembers(ctx context.Context, input AddGroupMembersInput) (*MemberOperationResult, error) {
	participants := make([]map[string]string, len(input.PhoneNumbers))
	for i, phone := range input.PhoneNumbers {
		participants[i] = map[string]string{"id": EnsureParticipantID(phone)}
	}

	payload := map[string]any{
		"participants": participants,
	}

	path := "/api/" + url.PathEscape(input.SessionName) + "/groups/" + url.PathEscape(input.GroupID) + "/participants/add"
	resp, err := a.doRequest(ctx, http.MethodPost, path, payload)
	if err != nil {
		return nil, fmt.Errorf("failed to add group members: %w", err)
	}

	return a.parseMemberOperationResult(resp, input.PhoneNumbers)
}

// RemoveGroupMembers removes one or more members from a group.
func (a *WahaAdapter) RemoveGroupMembers(ctx context.Context, input RemoveGroupMembersInput) (*MemberOperationResult, error) {
	participants := make([]map[string]string, len(input.PhoneNumbers))
	for i, phone := range input.PhoneNumbers {
		participants[i] = map[string]string{"id": EnsureParticipantID(phone)}
	}

	payload := map[string]any{
		"participants": participants,
	}

	path := "/api/" + url.PathEscape(input.SessionName) + "/groups/" + url.PathEscape(input.GroupID) + "/participants/remove"
	resp, err := a.doRequest(ctx, http.MethodPost, path, payload)
	if err != nil {
		return nil, fmt.Errorf("failed to remove group members: %w", err)
	}

	return a.parseMemberOperationResult(resp, input.PhoneNumbers)
}

// LeaveGroup makes the authenticated user leave a group.
func (a *WahaAdapter) LeaveGroup(ctx context.Context, input LeaveGroupInput) error {
	path := "/api/" + url.PathEscape(input.SessionName) + "/groups/" + url.PathEscape(input.GroupID) + "/leave"
	_, err := a.doRequest(ctx, http.MethodPost, path, nil)
	if err != nil {
		return fmt.Errorf("failed to leave group: %w", err)
	}

	return nil
}

// PromoteMember promotes a group member to admin.
func (a *WahaAdapter) PromoteMember(ctx context.Context, input PromoteMemberInput) error {
	participantID := EnsureParticipantID(input.PhoneNumber)

	payload := map[string]any{
		"participants": []map[string]string{{"id": participantID}},
	}

	path := "/api/" + url.PathEscape(input.SessionName) + "/groups/" + url.PathEscape(input.GroupID) + "/admin/promote"
	_, err := a.doRequest(ctx, http.MethodPost, path, payload)
	if err != nil {
		return fmt.Errorf("failed to promote member: %w", err)
	}

	return nil
}

// DemoteMember demotes a group admin to regular member.
func (a *WahaAdapter) DemoteMember(ctx context.Context, input DemoteMemberInput) error {
	participantID := EnsureParticipantID(input.PhoneNumber)

	payload := map[string]any{
		"participants": []map[string]string{{"id": participantID}},
	}

	path := "/api/" + url.PathEscape(input.SessionName) + "/groups/" + url.PathEscape(input.GroupID) + "/admin/demote"
	_, err := a.doRequest(ctx, http.MethodPost, path, payload)
	if err != nil {
		return fmt.Errorf("failed to demote member: %w", err)
	}

	return nil
}

// parseMemberOperationResult parses the WAHA response for member add/remove operations.
// W2: Detects empty body responses that silently assume success, which can mask
// promote/demote failures and lead to data corruption.
func (a *WahaAdapter) parseMemberOperationResult(resp []byte, requestedNumbers []string) (*MemberOperationResult, error) {
	// WAHA may return different response formats depending on version.
	// If the response is empty or just a success status, assume all succeeded
	// but log a warning — empty body means we cannot confirm individual results.
	if len(resp) == 0 {
		observability.LogAndCapture(
			a.logger,
			sentry.LevelWarning,
			"WAHA member operation returned empty body — assuming success without confirmation (potential data corruption)",
			fmt.Errorf("empty response body for member operation on %d numbers", len(requestedNumbers)),
			map[string]string{
				"module":          "waha-adapter",
				"flow":            "member-operation",
				"requested_count": strconv.Itoa(len(requestedNumbers)),
				"data_integrity":  "unverified",
			},
		)
		return &MemberOperationResult{
			SuccessCount:  len(requestedNumbers),
			FailedNumbers: []string{},
		}, nil
	}

	// Try to parse as a result with status per participant
	var participantResults []struct {
		Status string `json:"status"`
		JID    string `json:"jid"`
	}
	if err := json.Unmarshal(resp, &participantResults); err != nil {
		// If parsing fails, assume all succeeded (WAHA returned success HTTP status)
		a.logger.Warn("failed to parse member operation response — assuming success",
			zap.Int("requestedCount", len(requestedNumbers)),
			zap.Int("responseLen", len(resp)),
			zap.Error(err))
		return &MemberOperationResult{
			SuccessCount:  len(requestedNumbers),
			FailedNumbers: []string{},
		}, nil
	}

	result := &MemberOperationResult{
		FailedNumbers: []string{},
	}
	for _, pr := range participantResults {
		if pr.Status == "200" || pr.Status == "success" || pr.Status == "" {
			result.SuccessCount++
		} else {
			// Extract phone number from JID (remove @s.whatsapp.net)
			phone := pr.JID
			if idx := len(phone) - len("@s.whatsapp.net"); idx > 0 && phone[idx:] == "@s.whatsapp.net" {
				phone = phone[:idx]
			}
			result.FailedNumbers = append(result.FailedNumbers, phone)
		}
	}

	return result, nil
}

// =============================================================================
// MessageSender Implementation
// =============================================================================

// requireWorkingSession checks if the session is in WORKING/CONNECTED state before send operations.
// This prevents sending messages to sessions that are disconnected, starting, or in QR scan mode.
// Uses cached status to avoid excessive API calls (default 5s TTL, configurable via StatusCacheTTL).
func (a *WahaAdapter) requireWorkingSession(ctx context.Context, sessionName string) error {
	info, err := a.GetSessionStatus(ctx, sessionName)
	if err != nil {
		return fmt.Errorf("failed to check session status before sending: %w", err)
	}
	if info == nil {
		return NewProviderError(ErrCodeProviderError, "unexpected nil session status", nil)
	}

	if info.Status != SessionStatusConnected {
		a.logger.Warn("session not in working state",
			zap.String("session", sessionName),
			zap.String("status", string(info.Status)))
		return NewProviderError(
			ErrCodeSessionNotConnected,
			fmt.Sprintf("session '%s' is not working: current status is %s", sessionName, info.Status),
			nil,
		)
	}

	return nil
}

// SendText sends a text message to a chat.
func (a *WahaAdapter) SendText(ctx context.Context, input SendTextInput) (*MessageResult, error) {
	// Check session is working before sending
	if err := a.requireWorkingSession(ctx, input.SessionName); err != nil {
		return nil, err
	}

	a.logger.Info("SendText request",
		zap.String("session", input.SessionName),
		zap.String("chatId", input.ChatID),
		zap.Int("textLen", len(input.Text)),
		zap.Int("mentions", len(input.Mentions)))

	payload := a.mapper.SendTextToWaha(&input)

	resp, err := a.doRequest(ctx, http.MethodPost, "/api/sendText", payload)
	if err != nil {
		return nil, fmt.Errorf("failed to send text message: %w", err)
	}

	result, err := a.parseMessageResult(resp)
	if err != nil {
		return nil, err
	}

	a.logger.Info("SendText success",
		zap.String("session", input.SessionName),
		zap.String("chatId", input.ChatID),
		zap.String("messageId", result.ID))

	return result, nil
}

// SendImage sends an image message with optional caption.
func (a *WahaAdapter) SendImage(ctx context.Context, input SendImageInput) (*MessageResult, error) {
	// Check session is working before sending
	if err := a.requireWorkingSession(ctx, input.SessionName); err != nil {
		return nil, err
	}

	a.logger.Info("SendImage request",
		zap.String("session", input.SessionName),
		zap.String("chatId", input.ChatID),
		zap.Int("captionLen", len(input.Caption)),
		zap.String("imageURL", input.ImageURL),
		zap.Bool("hasBase64", input.ImageBase64 != ""))

	// Normalize ONLY images sent by URL (product photos from list/search/
	// generated offers) to WhatsApp's safe aspect window, so wide banners and
	// tall portraits aren't center-cropped ("zoomed") in the chat bubble.
	//
	// Relayed/forwarded media (offer-generate & monitor relay) arrives here as
	// base64 and is sent AS-IS — no padding, byte-for-byte the source image.
	// It used to be squared onto a white canvas, which added huge borders to
	// wide coupon banners and portrait coupon grids. Verified on a real device
	// that the group bubble does NOT crop these, and the requirement is that a
	// forwarded post look identical to the original, so the relay must not touch
	// the pixels. Fail-open on any URL download/decode error: keep the original.
	fetchedFromURL := false
	if input.ImageURL != "" && input.ImageBase64 == "" {
		if b64, perr := imagepad.FetchAndPadToSafeAspectJPEGBase64(ctx, input.ImageURL); perr == nil {
			input.ImageBase64 = b64
			input.Mimetype = "image/jpeg"
			input.ImageURL = ""
			fetchedFromURL = true
		} else {
			a.logger.Warn("Image aspect-normalize failed, sending original URL",
				zap.String("imageURL", input.ImageURL),
				zap.Error(perr))
		}
	}

	// Anti-ban: per-recipient media-hash jitter. Re-encode the product image with
	// a tiny, deterministic per-chat quality delta so the same photo doesn't go
	// out with a byte-identical hash to every group. ONLY on URL-fetched images —
	// relayed/forwarded base64 stays byte-for-byte identical (see note above).
	// Fail-safe: JitterJPEGBase64 returns the input unchanged on any error.
	if a.mediaJitter && fetchedFromURL && input.ChatID != "" {
		if jb, jerr := imagepad.JitterJPEGBase64(input.ImageBase64, input.ChatID); jerr == nil {
			input.ImageBase64 = jb
		}
	}

	// Pace the upload per session to stay under WhatsApp's media-server rate
	// limit (the source of the wrapped 429/400 that drop offers to text-only).
	// Fail-open: Reserve returns 0 on a nil pacer or any backend error.
	if a.uploadPacer != nil {
		if delay := a.uploadPacer.Reserve(ctx, input.SessionName); delay > 0 {
			a.logger.Debug("pacing media upload",
				zap.String("session", input.SessionName),
				zap.Duration("delay", delay))
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil, NewProviderError(ErrCodeTimeout, "canceled while pacing media upload", ctx.Err())
			}
		}
	}

	payload := a.mapper.SendImageToWaha(&input)

	resp, err := a.doRequest(ctx, http.MethodPost, "/api/sendImage", payload)
	if err != nil {
		return nil, fmt.Errorf("failed to send image message: %w", err)
	}

	result, err := a.parseMessageResult(resp)
	if err != nil {
		return nil, err
	}

	a.logger.Info("SendImage success",
		zap.String("session", input.SessionName),
		zap.String("chatId", input.ChatID),
		zap.String("messageId", result.ID))

	return result, nil
}

// SendVoice sends a voice message (PTT - Push to Talk).
func (a *WahaAdapter) SendVoice(ctx context.Context, input SendVoiceInput) (*MessageResult, error) {
	// Check session is working before sending
	if err := a.requireWorkingSession(ctx, input.SessionName); err != nil {
		a.logger.Warn("[DEBUG SendVoice] session check failed", zap.Error(err))
		return nil, err
	}

	payload := a.mapper.SendVoiceToWaha(&input)

	// Debug: log the payload being sent (truncate data field)
	dataPreview := payload.File.Data
	if len(dataPreview) > 80 {
		dataPreview = dataPreview[:40] + "..." + dataPreview[len(dataPreview)-40:]
	}
	a.logger.Info("[DEBUG SendVoice] WAHA request",
		zap.String("session", payload.Session),
		zap.String("chatId", payload.ChatID),
		zap.String("mimetype", payload.File.Mimetype),
		zap.String("filename", payload.File.Filename),
		zap.Int("dataLen", len(payload.File.Data)),
		zap.String("dataPreview", dataPreview),
		zap.Bool("convert", payload.Convert))

	resp, err := a.doRequest(ctx, http.MethodPost, "/api/sendVoice", payload)
	if err != nil {
		a.logger.Warn("[DEBUG SendVoice] WAHA request failed", zap.Error(err))
		return nil, fmt.Errorf("failed to send voice message: %w", err)
	}

	a.logger.Info("[DEBUG SendVoice] WAHA response", zap.String("body", string(resp)))

	return a.parseMessageResult(resp)
}

// =============================================================================
// PresenceController Implementation
// =============================================================================
//
// These are best-effort anti-ban choreography calls. They deliberately DO NOT
// gate on requireWorkingSession (the subsequent real send already does that; a
// presence call must not add a blocking status round-trip) and they FAIL-OPEN:
// any error is logged at debug and swallowed so it can never block the send.

// StartTyping shows the typing indicator in a chat (POST /api/startTyping).
func (a *WahaAdapter) StartTyping(ctx context.Context, input TypingInput) error {
	if input.SessionName == "" || input.ChatID == "" {
		return nil
	}
	req := &wahaChatRequest{Session: input.SessionName, ChatID: input.ChatID}
	if _, err := a.doRequest(ctx, http.MethodPost, "/api/startTyping", req); err != nil {
		a.logger.Debug("startTyping failed (ignored)",
			zap.String("session", input.SessionName),
			zap.String("chatId", input.ChatID), zap.Error(err))
	}
	return nil
}

// StopTyping clears the typing indicator (POST /api/stopTyping).
func (a *WahaAdapter) StopTyping(ctx context.Context, input TypingInput) error {
	if input.SessionName == "" || input.ChatID == "" {
		return nil
	}
	req := &wahaChatRequest{Session: input.SessionName, ChatID: input.ChatID}
	if _, err := a.doRequest(ctx, http.MethodPost, "/api/stopTyping", req); err != nil {
		a.logger.Debug("stopTyping failed (ignored)",
			zap.String("session", input.SessionName),
			zap.String("chatId", input.ChatID), zap.Error(err))
	}
	return nil
}

// SendSeen marks a chat/message as read (POST /api/sendSeen).
func (a *WahaAdapter) SendSeen(ctx context.Context, input SendSeenInput) error {
	if input.SessionName == "" || input.ChatID == "" {
		return nil
	}
	req := &wahaSendSeenRequest{
		Session:     input.SessionName,
		ChatID:      input.ChatID,
		MessageID:   input.MessageID,
		Participant: input.Participant,
	}
	if _, err := a.doRequest(ctx, http.MethodPost, "/api/sendSeen", req); err != nil {
		a.logger.Debug("sendSeen failed (ignored)",
			zap.String("session", input.SessionName),
			zap.String("chatId", input.ChatID), zap.Error(err))
	}
	return nil
}

// SetPresence sets session-level presence online/offline (POST /api/{session}/presence).
func (a *WahaAdapter) SetPresence(ctx context.Context, input SetPresenceInput) error {
	if input.SessionName == "" || input.Presence == "" {
		return nil
	}
	path := "/api/" + url.PathEscape(input.SessionName) + "/presence"
	req := &wahaSetPresenceRequest{Presence: string(input.Presence)}
	if _, err := a.doRequest(ctx, http.MethodPost, path, req); err != nil {
		a.logger.Debug("setPresence failed (ignored)",
			zap.String("session", input.SessionName),
			zap.String("presence", string(input.Presence)), zap.Error(err))
	}
	return nil
}

// SendPoll sends a poll message to a chat.
func (a *WahaAdapter) SendPoll(ctx context.Context, input SendPollInput) (*MessageResult, error) {
	// Check session is working before sending
	if err := a.requireWorkingSession(ctx, input.SessionName); err != nil {
		return nil, err
	}

	payload := a.mapper.SendPollToWaha(&input)

	resp, err := a.doRequest(ctx, http.MethodPost, "/api/sendPoll", payload)
	if err != nil {
		return nil, fmt.Errorf("failed to send poll message: %w", err)
	}

	return a.parseMessageResult(resp)
}

// SendLinkCustomPreview sends a message with a custom link preview card.
func (a *WahaAdapter) SendLinkCustomPreview(ctx context.Context, input SendLinkPreviewInput) (*MessageResult, error) {
	if err := a.requireWorkingSession(ctx, input.SessionName); err != nil {
		return nil, err
	}

	a.logger.Info("SendLinkCustomPreview request",
		zap.String("session", input.SessionName),
		zap.String("chatId", input.ChatID),
		zap.String("previewURL", input.URL),
		zap.String("title", input.Title))

	// WhatsApp center-crops PORTRAIT images in the rich preview card (a tall
	// product loses its top/bottom — measured via probe); square/landscape render
	// in full. So edge-extend ONLY portrait preview images to a square: the side
	// margins are filled with the photo's own edge columns, so the product shows
	// whole and the fill looks like a natural continuation of the photo (not a
	// white bar, blur, or obvious crop). Square/landscape are sent UNCHANGED.
	// Fail-open: keep the original on any download/decode error.
	switch {
	case input.ImageURL != "" && input.ImageBase64 == "":
		if b64, perr := imagepad.FetchEdgeExtendPortraitJPEGBase64(ctx, input.ImageURL); perr != nil {
			a.logger.Warn("Preview image edge-extend failed, sending original URL",
				zap.String("imageURL", input.ImageURL),
				zap.Error(perr))
		} else if b64 != "" {
			input.ImageBase64 = b64
			input.Mimetype = "image/jpeg"
			input.ImageURL = ""
		}
	case input.ImageBase64 != "":
		if b64, perr := imagepad.EdgeExtendPortraitJPEGBase64(input.ImageBase64); perr != nil {
			a.logger.Warn("Preview image edge-extend (base64) failed, sending original",
				zap.Error(perr))
		} else if b64 != "" {
			input.ImageBase64 = b64
			input.Mimetype = "image/jpeg"
		}
	}

	payload := a.mapper.SendLinkPreviewToWaha(&input)

	resp, err := a.doRequest(ctx, http.MethodPost, "/api/send/link-custom-preview", payload)
	if err != nil {
		return nil, fmt.Errorf("failed to send link custom preview: %w", err)
	}

	result, err := a.parseMessageResult(resp)
	if err != nil {
		return nil, err
	}

	a.logger.Info("SendLinkCustomPreview success",
		zap.String("session", input.SessionName),
		zap.String("chatId", input.ChatID),
		zap.String("messageId", result.ID))

	return result, nil
}

// SendVideo sends a video message to a chat.
func (a *WahaAdapter) SendVideo(ctx context.Context, input SendVideoInput) (*MessageResult, error) {
	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("invalid video input: %w", err)
	}

	if err := a.requireWorkingSession(ctx, input.SessionName); err != nil {
		return nil, err
	}

	payload := a.mapper.SendVideoToWaha(&input)

	resp, err := a.doRequest(ctx, http.MethodPost, "/api/sendVideo", payload)
	if err != nil {
		return nil, fmt.Errorf("failed to send video message: %w", err)
	}

	return a.parseMessageResult(resp)
}

// SendDocument sends a file/document message to a chat.
func (a *WahaAdapter) SendDocument(ctx context.Context, input SendDocumentInput) (*MessageResult, error) {
	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("invalid document input: %w", err)
	}

	if err := a.requireWorkingSession(ctx, input.SessionName); err != nil {
		return nil, err
	}

	payload := a.mapper.SendDocumentToWaha(&input)

	resp, err := a.doRequest(ctx, http.MethodPost, "/api/sendFile", payload)
	if err != nil {
		return nil, fmt.Errorf("failed to send document message: %w", err)
	}

	return a.parseMessageResult(resp)
}

// =============================================================================
// WebhookManager Implementation
// =============================================================================

// SetWebhook configures the webhook URL and events for a session.
// WAHA configures webhooks via session update (PUT /api/sessions/{session}),
// not via a separate webhook endpoint.
func (a *WahaAdapter) SetWebhook(ctx context.Context, input SetWebhookInput) error {
	webhookConfig := a.mapper.SetWebhookToWahaConfig(&input)

	updatePayload := &wahaUpdateSessionRequest{
		Config: &wahaSessionConfig{
			Webhooks: []wahaWebhookConfig{*webhookConfig},
		},
	}

	path := "/api/sessions/" + url.PathEscape(input.SessionName)
	_, err := a.doRequest(ctx, http.MethodPut, path, updatePayload)
	if err != nil {
		return fmt.Errorf("failed to set webhook: %w", err)
	}

	return nil
}

// =============================================================================
// MessageReconciler Implementation
// =============================================================================

// GetMessages retrieves messages from a chat for reconciliation purposes.
func (a *WahaAdapter) GetMessages(ctx context.Context, input GetMessagesInput) (*MessagesResult, error) {
	// Build query parameters
	params := url.Values{}
	params.Set("limit", strconv.Itoa(input.Limit))
	if input.Offset > 0 {
		params.Set("offset", strconv.Itoa(input.Offset))
	}
	if input.DownloadMedia {
		params.Set("downloadMedia", "true")
	}

	// Apply filters
	if input.Filter != nil {
		if input.Filter.FromMe != nil {
			params.Set("fromMe", strconv.FormatBool(*input.Filter.FromMe))
		}
		if input.Filter.FromTimestamp != nil {
			params.Set("fromTimestamp", strconv.FormatInt(input.Filter.FromTimestamp.Unix(), 10))
		}
		if input.Filter.ToTimestamp != nil {
			params.Set("toTimestamp", strconv.FormatInt(input.Filter.ToTimestamp.Unix(), 10))
		}
	}

	path := "/api/" + url.PathEscape(input.SessionName) + "/chats/" + url.PathEscape(input.ChatID) + "/messages?" + params.Encode()
	resp, err := a.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}

	var wahaResp wahaGetMessagesResponse
	if err := json.Unmarshal(resp, &wahaResp); err != nil {
		// Try parsing as array (WAHA sometimes returns just the messages array)
		var wahaMessages []wahaMessage
		if err2 := json.Unmarshal(resp, &wahaMessages); err2 != nil {
			return nil, NewProviderError(ErrCodeProviderError, "failed to parse messages response", err)
		}
		wahaResp.Messages = wahaMessages
		wahaResp.HasMore = len(wahaMessages) == input.Limit
	}

	return a.mapper.MessagesResultFromWaha(&wahaResp, input.Offset), nil
}

// =============================================================================
// Cache Management
// =============================================================================

// getStatusFromCache retrieves session status from cache if not expired.
func (a *WahaAdapter) getStatusFromCache(sessionName string) *SessionInfo {
	a.statusCacheMu.RLock()
	defer a.statusCacheMu.RUnlock()

	entry, ok := a.statusCache[sessionName]
	if !ok || time.Now().After(entry.expiresAt) {
		return nil
	}

	return entry.info
}

// cacheStatus stores session status in cache.
func (a *WahaAdapter) cacheStatus(sessionName string, info *SessionInfo) {
	a.statusCacheMu.Lock()
	defer a.statusCacheMu.Unlock()

	a.statusCache[sessionName] = &statusCacheEntry{
		info:      info,
		expiresAt: time.Now().Add(a.statusCacheTTL),
	}
}

// ClearStatusCache removes a session from the status cache.
// Exported so the Redis SSE bridge can invalidate stale cache on status events.
func (a *WahaAdapter) ClearStatusCache(sessionName string) {
	a.statusCacheMu.Lock()
	defer a.statusCacheMu.Unlock()

	delete(a.statusCache, sessionName)
}

// ClearAllStatusCache removes all entries from the status cache.
func (a *WahaAdapter) ClearAllStatusCache() {
	a.statusCacheMu.Lock()
	defer a.statusCacheMu.Unlock()

	a.statusCache = make(map[string]*statusCacheEntry)
}

// =============================================================================
// Session Maintenance (not part of WhatsAppProvider interface)
// =============================================================================

// ListAllSessions retrieves all WAHA sessions via GET /api/sessions.
// Returns raw session info including status for each session.
// This is used for maintenance operations like cleanup of FAILED sessions.
func (a *WahaAdapter) ListAllSessions(ctx context.Context) ([]SessionInfo, error) {
	resp, err := a.doRequest(ctx, http.MethodGet, "/api/sessions", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list all sessions: %w", err)
	}

	var wahaSessions []wahaSession
	if err := json.Unmarshal(resp, &wahaSessions); err != nil {
		return nil, NewProviderError(ErrCodeProviderError, "failed to parse sessions list response", err)
	}

	sessions := make([]SessionInfo, 0, len(wahaSessions))
	for i := range wahaSessions {
		sessions = append(sessions, *a.mapper.SessionInfoFromWaha(&wahaSessions[i]))
	}

	return sessions, nil
}

// SessionProtector reports whether a WAHA session must be protected from cleanup —
// i.e. it belongs to a real number that paired at least once. It is consulted before
// deleting any FAILED session. Returning a non-nil error is treated fail-safe (the
// session IS protected), so a transient lookup failure can never cause an irreversible
// unpair. A nil SessionProtector disables protection (delete every FAILED session).
type SessionProtector func(ctx context.Context, sessionName string) (protected bool, err error)

// CleanupFailedSessions lists all WAHA sessions and deletes those in FAILED status,
// EXCEPT sessions that isProtected reports as protected. Returns the number deleted.
// It runs periodically from a cron job to reclaim resources from zombie sessions
// (abandoned QR scans that never paired).
//
// CRITICAL — why isProtected exists: a FAILED status is frequently TRANSIENT for a
// real, paired number (a network blip, a WAHA restart, GOWS reconnecting). While a
// session is FAILED, WAHA reports it with me=null, so status + me alone cannot tell a
// dead never-paired zombie from a live number having a bad minute. Deleting a paired
// session is an IRREVERSIBLE unpair — the owner must physically re-scan the QR. So we
// consult the source of truth (the backend instance record: a number that paired at
// least once has a phone_number set) and never delete a protected session here; the
// waha-monitor cron restarts those instead. This is the backend-side fix that pairs
// with the structural block in waha-router.
func (a *WahaAdapter) CleanupFailedSessions(ctx context.Context, isProtected SessionProtector) (int, error) {
	sessions, err := a.ListAllSessions(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to list sessions for cleanup: %w", err)
	}

	var failed []string
	for _, s := range sessions {
		if s.Status != SessionStatusFailed {
			continue
		}
		// Defense-in-depth: never touch a session WAHA still reports as
		// authenticated (me != null), regardless of the FAILED flag.
		if s.AuthenticatedUser != nil {
			continue
		}
		failed = append(failed, s.Name)
	}

	if len(failed) == 0 {
		a.logger.Debug("no FAILED sessions to clean up",
			zap.Int("totalSessions", len(sessions)))
		return 0, nil
	}

	a.logger.Info("cleaning up FAILED WAHA sessions",
		zap.Int("failedCount", len(failed)),
		zap.Int("totalSessions", len(sessions)))

	deleted := 0
	protectedCount := 0
	for _, name := range failed {
		if isProtected != nil {
			protected, perr := isProtected(ctx, name)
			if perr != nil {
				// Fail-safe: on any uncertainty, protect the session (skip delete)
				// rather than risk unpairing a real number.
				a.logger.Warn("skipping cleanup of FAILED session: paired-check failed (protecting)",
					zap.String("session", name),
					zap.Error(perr))
				protectedCount++
				continue
			}
			if protected {
				a.logger.Info("protected paired session from cleanup (FAILED but paired at least once)",
					zap.String("session", name))
				protectedCount++
				continue
			}
		}

		if err := a.DeleteSession(ctx, name); err != nil {
			a.logger.Warn("failed to delete FAILED session",
				zap.String("session", name),
				zap.Error(err))
			continue
		}
		deleted++
		a.logger.Info("deleted FAILED session",
			zap.String("session", name))
	}

	if deleted > 0 || protectedCount > 0 {
		observability.CaptureMessage(sentry.LevelInfo,
			fmt.Sprintf("WAHA session cleanup: deleted %d, protected %d of %d FAILED sessions", deleted, protectedCount, len(failed)),
			map[string]string{
				"module":         "waha-adapter",
				"flow":           "session-cleanup",
				"deleted":        strconv.Itoa(deleted),
				"protected":      strconv.Itoa(protectedCount),
				"failed_total":   strconv.Itoa(len(failed)),
				"sessions_total": strconv.Itoa(len(sessions)),
			})
	}

	return deleted, nil
}

// =============================================================================
// Helper Methods
// =============================================================================

// parseMessageResult parses a WAHA message send response.
func (a *WahaAdapter) parseMessageResult(resp []byte) (*MessageResult, error) {
	var wahaResult wahaSendMessageResponse
	if err := json.Unmarshal(resp, &wahaResult); err != nil {
		return nil, NewProviderError(ErrCodeProviderError, "failed to parse message result", err)
	}

	return a.mapper.MessageResultFromWaha(&wahaResult), nil
}

// =============================================================================
// MediaDownloader Implementation
// =============================================================================

// DownloadMedia implements MediaDownloader.DownloadMedia.
// Downloads media content from a WhatsApp message using the WAHA media endpoint.
// Uses a direct HTTP call (instead of doRequest) to preserve Content-Type and
// Content-Disposition response headers for mimetype/filename extraction.
func (a *WahaAdapter) DownloadMedia(ctx context.Context, input DownloadMediaInput) (*DownloadMediaResult, error) {
	if input.SessionName == "" {
		return nil, NewProviderError(ErrCodeInvalidInput, "session name is required", nil)
	}
	if input.MediaID == "" {
		return nil, NewProviderError(ErrCodeInvalidInput, "media ID is required", nil)
	}

	a.logger.Debug("Downloading media",
		zap.String("session", input.SessionName),
		zap.String("mediaID", input.MediaID))

	// N2: 30s timeout for download operations
	dlCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// WAHA media download endpoint: GET /api/{session}/media/{mediaId}
	reqURL := fmt.Sprintf("%s/api/%s/media/%s", a.apiURL, url.PathEscape(input.SessionName), url.PathEscape(input.MediaID))
	req, err := http.NewRequestWithContext(dlCtx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, NewProviderError(ErrCodeNetworkError, "failed to create request", err)
	}
	req.Header.Set("X-Api-Key", a.apiKey)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, NewProviderError(ErrCodeNetworkError, "media download request failed", err)
	}
	defer resp.Body.Close()

	// N1: Limit download body size to 50 MB to prevent memory exhaustion
	limitedBody := http.MaxBytesReader(nil, resp.Body, maxDownloadBodySize)
	body, err := io.ReadAll(limitedBody)
	if err != nil {
		return nil, NewProviderError(ErrCodeNetworkError, "failed to read media response (may exceed 50MB limit)", err)
	}

	if resp.StatusCode >= 400 {
		return nil, a.handleErrorResponse(resp.StatusCode, reqURL, body)
	}

	// Extract mimetype from Content-Type header
	mimetype := resp.Header.Get("Content-Type")
	if mimetype == "" {
		mimetype = "application/octet-stream"
	}

	// Extract filename from Content-Disposition header if available
	var filename string
	if cd := resp.Header.Get("Content-Disposition"); cd != "" {
		// Parse "attachment; filename=\"example.jpg\"" or "inline; filename=example.jpg"
		if idx := strings.Index(cd, "filename="); idx >= 0 {
			filename = cd[idx+len("filename="):]
			filename = strings.Trim(filename, "\"' ")
			// Handle filename*= (RFC 5987) by taking only the part after ''
			if eqIdx := strings.Index(filename, "''"); eqIdx >= 0 {
				filename = filename[eqIdx+2:]
			}
		}
	}

	return &DownloadMediaResult{
		Data:     body,
		Mimetype: mimetype,
		Filename: filename,
	}, nil
}

// DownloadMediaByURL implements MediaDownloader.DownloadMediaByURL.
// Downloads media from an internal WAHA file URL (e.g. http://localhost:3000/api/files/...).
// The URL's host is replaced with the adapter's configured API URL so the
// request goes to the correct endpoint with authentication.
func (a *WahaAdapter) DownloadMediaByURL(ctx context.Context, mediaURL string) (*DownloadMediaResult, error) {
	if mediaURL == "" {
		return nil, NewProviderError(ErrCodeInvalidInput, "media URL is required", nil)
	}

	// Parse the media URL to extract the path
	parsed, err := url.Parse(mediaURL)
	if err != nil {
		return nil, NewProviderError(ErrCodeInvalidInput, "invalid media URL", err)
	}

	// Rewrite to use the adapter's configured API base URL
	reqURL := a.apiURL + parsed.Path

	a.logger.Debug("Downloading media by URL",
		zap.String("originalURL", mediaURL),
		zap.String("rewrittenURL", reqURL))

	// N2: 30s timeout for download operations
	dlCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(dlCtx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, NewProviderError(ErrCodeNetworkError, "failed to create request", err)
	}
	req.Header.Set("X-Api-Key", a.apiKey)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, NewProviderError(ErrCodeNetworkError, "media download request failed", err)
	}
	defer resp.Body.Close()

	// N1: Limit download body size to 50 MB to prevent memory exhaustion
	limitedBody := http.MaxBytesReader(nil, resp.Body, maxDownloadBodySize)
	body, err := io.ReadAll(limitedBody)
	if err != nil {
		return nil, NewProviderError(ErrCodeNetworkError, "failed to read media response (may exceed 50MB limit)", err)
	}

	if resp.StatusCode >= 400 {
		return nil, a.handleErrorResponse(resp.StatusCode, reqURL, body)
	}

	mimetype := resp.Header.Get("Content-Type")
	if mimetype == "" {
		mimetype = "application/octet-stream"
	}

	var filename string
	if cd := resp.Header.Get("Content-Disposition"); cd != "" {
		if idx := strings.Index(cd, "filename="); idx >= 0 {
			filename = cd[idx+len("filename="):]
			filename = strings.Trim(filename, "\"' ")
		}
	}

	return &DownloadMediaResult{
		Data:     body,
		Mimetype: mimetype,
		Filename: filename,
	}, nil
}

// RedownloadMedia implements MediaDownloader.RedownloadMedia.
// Re-downloads media from WhatsApp servers by fetching a specific message with downloadMedia=true.
// This bypasses the local WAHA file cache, which may have expired (WHATSAPP_FILES_LIFETIME).
// WhatsApp retains media on their servers for ~30 days.
func (a *WahaAdapter) RedownloadMedia(ctx context.Context, sessionName, chatID, messageID string) (*DownloadMediaResult, error) {
	if sessionName == "" {
		return nil, NewProviderError(ErrCodeInvalidInput, "session name is required", nil)
	}
	if chatID == "" {
		return nil, NewProviderError(ErrCodeInvalidInput, "chat ID is required", nil)
	}
	if messageID == "" {
		return nil, NewProviderError(ErrCodeInvalidInput, "message ID is required", nil)
	}

	a.logger.Debug("Re-downloading media from WhatsApp servers",
		zap.String("session", sessionName),
		zap.String("chatID", chatID),
		zap.String("messageID", messageID))

	// GET /api/{session}/chats/{chatId}/messages/{messageId}?downloadMedia=true
	path := fmt.Sprintf("/api/%s/chats/%s/messages/%s?downloadMedia=true",
		url.PathEscape(sessionName),
		url.PathEscape(chatID),
		url.PathEscape(messageID))

	resp, err := a.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to re-download media: %w", err)
	}

	// Parse the message response to extract fresh media URL
	var msgResp struct {
		Media struct {
			URL      string `json:"url"`
			Mimetype string `json:"mimetype"`
		} `json:"media"`
		MediaURL string `json:"mediaUrl"` // Legacy field
		Mimetype string `json:"mimetype"`
	}
	if err := json.Unmarshal(resp, &msgResp); err != nil {
		return nil, NewProviderError(ErrCodeProviderError, "failed to parse re-download response", err)
	}

	// Extract the fresh media URL (prefer media.url over legacy mediaUrl)
	freshMediaURL := msgResp.Media.URL
	if freshMediaURL == "" {
		freshMediaURL = msgResp.MediaURL
	}
	if freshMediaURL == "" {
		return nil, NewProviderError(ErrCodeProviderError, "no media URL in re-download response", nil)
	}

	// Extract mimetype from response
	mimetype := msgResp.Media.Mimetype
	if mimetype == "" {
		mimetype = msgResp.Mimetype
	}

	a.logger.Debug("Got fresh media URL from re-download",
		zap.String("freshURL", freshMediaURL),
		zap.String("mimetype", mimetype))

	// Download the fresh media using the new URL
	result, err := a.DownloadMediaByURL(ctx, freshMediaURL)
	if err != nil {
		return nil, fmt.Errorf("failed to download fresh media: %w", err)
	}

	// Use the mimetype from the message response if the download didn't provide one
	if result.Mimetype == "" || result.Mimetype == "application/octet-stream" {
		if mimetype != "" {
			result.Mimetype = mimetype
		}
	}

	return result, nil
}
