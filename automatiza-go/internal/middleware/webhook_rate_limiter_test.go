package middleware

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// ---------------------------------------------------------------------------
// DefaultWebhookRateLimitConfig
// ---------------------------------------------------------------------------

func TestDefaultWebhookRateLimitConfig(t *testing.T) {
	t.Parallel()

	cfg := DefaultWebhookRateLimitConfig()

	if cfg.MaxRequests != 2000 {
		t.Errorf("expected MaxRequests=2000, got %d", cfg.MaxRequests)
	}
	if cfg.WindowSize != time.Minute {
		t.Errorf("expected WindowSize=1m, got %s", cfg.WindowSize)
	}
	if cfg.UseRedis {
		t.Error("expected UseRedis=false")
	}
	if cfg.RedisClient != nil {
		t.Error("expected RedisClient=nil")
	}
	if cfg.KeyPrefix != "webhook_rl" {
		t.Errorf("expected KeyPrefix=%q, got %q", "webhook_rl", cfg.KeyPrefix)
	}
	if cfg.Logger != nil {
		t.Error("expected Logger=nil")
	}
}

// ---------------------------------------------------------------------------
// newWebhookRateLimiter
// ---------------------------------------------------------------------------

func TestNewWebhookRateLimiter_NilConfigUsesDefaults(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rl := newWebhookRateLimiter(nil, ctx)

	if rl.config == nil {
		t.Fatal("expected config to be non-nil when nil is passed")
	}
	if rl.config.MaxRequests != 2000 {
		t.Errorf("expected default MaxRequests=2000, got %d", rl.config.MaxRequests)
	}
	if rl.config.WindowSize != time.Minute {
		t.Errorf("expected default WindowSize=1m, got %s", rl.config.WindowSize)
	}
	if rl.config.KeyPrefix != "webhook_rl" {
		t.Errorf("expected default KeyPrefix=%q, got %q", "webhook_rl", rl.config.KeyPrefix)
	}
	if rl.memStore == nil {
		t.Error("expected memStore to be initialized")
	}
}

func TestNewWebhookRateLimiter_CustomConfig(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := zaptest.NewLogger(t)
	cfg := &WebhookRateLimiterConfig{
		MaxRequests: 100,
		WindowSize:  30 * time.Second,
		UseRedis:    false,
		KeyPrefix:   "custom_prefix",
		Logger:      logger,
	}

	rl := newWebhookRateLimiter(cfg, ctx)

	if rl.config.MaxRequests != 100 {
		t.Errorf("expected MaxRequests=100, got %d", rl.config.MaxRequests)
	}
	if rl.config.WindowSize != 30*time.Second {
		t.Errorf("expected WindowSize=30s, got %s", rl.config.WindowSize)
	}
	if rl.config.KeyPrefix != "custom_prefix" {
		t.Errorf("expected KeyPrefix=%q, got %q", "custom_prefix", rl.config.KeyPrefix)
	}
	if rl.config.Logger != logger {
		t.Error("expected Logger to match the provided logger")
	}
	if rl.memStore == nil {
		t.Error("expected memStore to be initialized")
	}
}

// ---------------------------------------------------------------------------
// checkMemory
// ---------------------------------------------------------------------------

func TestCheckMemory(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		maxRequests     int
		windowSize      time.Duration
		requestCount    int
		expectAllowed   bool
		expectRemaining int
		expectRetry     bool // retryAfter > 0
	}{
		{
			name:            "first request is allowed",
			maxRequests:     5,
			windowSize:      time.Minute,
			requestCount:    1,
			expectAllowed:   true,
			expectRemaining: 4,
			expectRetry:     false,
		},
		{
			name:            "requests within limit are allowed",
			maxRequests:     5,
			windowSize:      time.Minute,
			requestCount:    3,
			expectAllowed:   true,
			expectRemaining: 2,
			expectRetry:     false,
		},
		{
			name:            "last allowed request",
			maxRequests:     5,
			windowSize:      time.Minute,
			requestCount:    5,
			expectAllowed:   true,
			expectRemaining: 0,
			expectRetry:     false,
		},
		{
			name:            "first request over limit is rejected",
			maxRequests:     5,
			windowSize:      time.Minute,
			requestCount:    6,
			expectAllowed:   false,
			expectRemaining: 0,
			expectRetry:     true,
		},
		{
			name:            "well over limit still rejected",
			maxRequests:     3,
			windowSize:      time.Minute,
			requestCount:    10,
			expectAllowed:   false,
			expectRemaining: 0,
			expectRetry:     true,
		},
		{
			name:            "single max request - first allowed",
			maxRequests:     1,
			windowSize:      time.Minute,
			requestCount:    1,
			expectAllowed:   true,
			expectRemaining: 0,
			expectRetry:     false,
		},
		{
			name:            "single max request - second rejected",
			maxRequests:     1,
			windowSize:      time.Minute,
			requestCount:    2,
			expectAllowed:   false,
			expectRemaining: 0,
			expectRetry:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			cfg := &WebhookRateLimiterConfig{
				MaxRequests: tt.maxRequests,
				WindowSize:  tt.windowSize,
				UseRedis:    false,
				KeyPrefix:   "test_rl",
				Logger:      zaptest.NewLogger(t),
			}
			rl := newWebhookRateLimiter(cfg, ctx)

			var allowed bool
			var remaining int
			var retryAfter time.Duration

			for i := 0; i < tt.requestCount; i++ {
				allowed, remaining, retryAfter = rl.checkMemory("192.168.1.1")
			}

			if allowed != tt.expectAllowed {
				t.Errorf("expected allowed=%v after %d requests, got %v",
					tt.expectAllowed, tt.requestCount, allowed)
			}
			if remaining != tt.expectRemaining {
				t.Errorf("expected remaining=%d, got %d", tt.expectRemaining, remaining)
			}
			if tt.expectRetry && retryAfter <= 0 {
				t.Error("expected retryAfter > 0, got <= 0")
			}
			if !tt.expectRetry && retryAfter != 0 {
				t.Errorf("expected retryAfter=0, got %s", retryAfter)
			}
		})
	}
}

func TestCheckMemory_WindowReset(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := &WebhookRateLimiterConfig{
		MaxRequests: 3,
		WindowSize:  50 * time.Millisecond, // short window for test
		UseRedis:    false,
		KeyPrefix:   "test_rl",
		Logger:      zaptest.NewLogger(t),
	}
	rl := newWebhookRateLimiter(cfg, ctx)

	// Exhaust the limit
	for i := 0; i < 3; i++ {
		rl.checkMemory("10.0.0.1")
	}
	allowed, _, _ := rl.checkMemory("10.0.0.1")
	if allowed {
		t.Fatal("expected request to be rejected after exhausting limit")
	}

	// Wait for the window to expire
	time.Sleep(60 * time.Millisecond)

	// After window reset, requests should be allowed again
	allowed, remaining, retryAfter := rl.checkMemory("10.0.0.1")
	if !allowed {
		t.Error("expected request to be allowed after window reset")
	}
	if remaining != 2 {
		t.Errorf("expected remaining=2 after reset (first new request), got %d", remaining)
	}
	if retryAfter != 0 {
		t.Errorf("expected retryAfter=0 after reset, got %s", retryAfter)
	}
}

// ---------------------------------------------------------------------------
// check (delegation)
// ---------------------------------------------------------------------------

func TestCheck_DelegatesToCheckMemory_WhenUseRedisFalse(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := &WebhookRateLimiterConfig{
		MaxRequests: 5,
		WindowSize:  time.Minute,
		UseRedis:    false,
		RedisClient: nil,
		KeyPrefix:   "test_rl",
		Logger:      zaptest.NewLogger(t),
	}
	rl := newWebhookRateLimiter(cfg, ctx)

	allowed, remaining, retryAfter := rl.check("10.0.0.1")
	if !allowed {
		t.Error("expected first request to be allowed")
	}
	if remaining != 4 {
		t.Errorf("expected remaining=4, got %d", remaining)
	}
	if retryAfter != 0 {
		t.Errorf("expected retryAfter=0, got %s", retryAfter)
	}
}

func TestCheck_FallsBackToMemory_WhenUseRedisTrueButClientNil(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := &WebhookRateLimiterConfig{
		MaxRequests: 10,
		WindowSize:  time.Minute,
		UseRedis:    true,
		RedisClient: nil, // nil client forces fallback
		KeyPrefix:   "test_rl",
		Logger:      zaptest.NewLogger(t),
	}
	rl := newWebhookRateLimiter(cfg, ctx)

	// check() should not panic and should use memory
	allowed, remaining, _ := rl.check("10.0.0.1")
	if !allowed {
		t.Error("expected request to be allowed via memory fallback")
	}
	if remaining != 9 {
		t.Errorf("expected remaining=9, got %d", remaining)
	}
}

// ---------------------------------------------------------------------------
// WebhookRateLimitMiddleware (Gin integration)
// ---------------------------------------------------------------------------

func TestWebhookRateLimitMiddleware_AllowedWithinLimit(t *testing.T) {
	t.Parallel()

	cfg := &WebhookRateLimiterConfig{
		MaxRequests: 5,
		WindowSize:  time.Minute,
		UseRedis:    false,
		KeyPrefix:   "test_rl",
		Logger:      zaptest.NewLogger(t),
	}

	r := gin.New()
	r.Use(WebhookRateLimitMiddleware(cfg))
	r.GET("/webhook", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	for i := 0; i < 5; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/webhook", nil)
		req.RemoteAddr = "192.168.1.100:12345"
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("request %d: expected status 200, got %d", i+1, w.Code)
		}

		limitHeader := w.Header().Get("X-RateLimit-Limit")
		if limitHeader != "5" {
			t.Errorf("request %d: expected X-RateLimit-Limit=5, got %q", i+1, limitHeader)
		}

		remainingHeader := w.Header().Get("X-RateLimit-Remaining")
		expectedRemaining := strconv.Itoa(5 - (i + 1))
		if remainingHeader != expectedRemaining {
			t.Errorf("request %d: expected X-RateLimit-Remaining=%s, got %q",
				i+1, expectedRemaining, remainingHeader)
		}

		retryAfterHeader := w.Header().Get("Retry-After")
		if retryAfterHeader != "" {
			t.Errorf("request %d: expected no Retry-After header, got %q", i+1, retryAfterHeader)
		}
	}
}

func TestWebhookRateLimitMiddleware_Returns429WhenExceeded(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	cfg := &WebhookRateLimiterConfig{
		MaxRequests: 3,
		WindowSize:  time.Minute,
		UseRedis:    false,
		KeyPrefix:   "test_rl",
		Logger:      logger,
	}

	r := gin.New()
	r.Use(WebhookRateLimitMiddleware(cfg))
	r.GET("/webhook", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// First 3 requests should succeed
	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/webhook", nil)
		req.RemoteAddr = "10.0.0.50:9999"
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i+1, w.Code)
		}
	}

	// 4th request should be rate limited
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/webhook", nil)
	req.RemoteAddr = "10.0.0.50:9999"
	r.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected status 429, got %d", w.Code)
	}

	limitHeader := w.Header().Get("X-RateLimit-Limit")
	if limitHeader != "3" {
		t.Errorf("expected X-RateLimit-Limit=3, got %q", limitHeader)
	}

	remainingHeader := w.Header().Get("X-RateLimit-Remaining")
	if remainingHeader != "0" {
		t.Errorf("expected X-RateLimit-Remaining=0, got %q", remainingHeader)
	}

	retryAfterHeader := w.Header().Get("Retry-After")
	if retryAfterHeader == "" {
		t.Error("expected Retry-After header to be set on 429 response")
	}
}

func TestWebhookRateLimitMiddleware_HeadersPresent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		maxRequests       int
		requestIndex      int // 0-based, how many requests to send before checking
		expectStatus      int
		expectLimit       string
		expectRemaining   string
		expectRetryAfter  bool
	}{
		{
			name:             "first request - all remaining",
			maxRequests:      10,
			requestIndex:     0,
			expectStatus:     http.StatusOK,
			expectLimit:      "10",
			expectRemaining:  "9",
			expectRetryAfter: false,
		},
		{
			name:             "mid window - partial remaining",
			maxRequests:      10,
			requestIndex:     5,
			expectStatus:     http.StatusOK,
			expectLimit:      "10",
			expectRemaining:  "4",
			expectRetryAfter: false,
		},
		{
			name:             "at limit - zero remaining, still allowed",
			maxRequests:      10,
			requestIndex:     9,
			expectStatus:     http.StatusOK,
			expectLimit:      "10",
			expectRemaining:  "0",
			expectRetryAfter: false,
		},
		{
			name:             "over limit - rejected with retry-after",
			maxRequests:      10,
			requestIndex:     10,
			expectStatus:     http.StatusTooManyRequests,
			expectLimit:      "10",
			expectRemaining:  "0",
			expectRetryAfter: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := &WebhookRateLimiterConfig{
				MaxRequests: tt.maxRequests,
				WindowSize:  time.Minute,
				UseRedis:    false,
				KeyPrefix:   "test_rl",
				Logger:      zaptest.NewLogger(t),
			}

			r := gin.New()
			r.Use(WebhookRateLimitMiddleware(cfg))
			r.GET("/webhook", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"status": "ok"})
			})

			var w *httptest.ResponseRecorder
			for i := 0; i <= tt.requestIndex; i++ {
				w = httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, "/webhook", nil)
				req.RemoteAddr = "172.16.0.1:8080"
				r.ServeHTTP(w, req)
			}

			if w.Code != tt.expectStatus {
				t.Errorf("expected status %d, got %d", tt.expectStatus, w.Code)
			}
			if got := w.Header().Get("X-RateLimit-Limit"); got != tt.expectLimit {
				t.Errorf("expected X-RateLimit-Limit=%q, got %q", tt.expectLimit, got)
			}
			if got := w.Header().Get("X-RateLimit-Remaining"); got != tt.expectRemaining {
				t.Errorf("expected X-RateLimit-Remaining=%q, got %q", tt.expectRemaining, got)
			}
			retryAfter := w.Header().Get("Retry-After")
			if tt.expectRetryAfter && retryAfter == "" {
				t.Error("expected Retry-After header to be set")
			}
			if !tt.expectRetryAfter && retryAfter != "" {
				t.Errorf("expected no Retry-After header, got %q", retryAfter)
			}
		})
	}
}

func TestWebhookRateLimitMiddleware_AbortsPipelineOn429(t *testing.T) {
	t.Parallel()

	cfg := &WebhookRateLimiterConfig{
		MaxRequests: 1,
		WindowSize:  time.Minute,
		UseRedis:    false,
		KeyPrefix:   "test_rl",
		Logger:      zaptest.NewLogger(t),
	}

	handlerCalled := 0
	r := gin.New()
	r.Use(WebhookRateLimitMiddleware(cfg))
	r.GET("/webhook", func(c *gin.Context) {
		handlerCalled++
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// First request goes through
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/webhook", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	r.ServeHTTP(w, req)
	if handlerCalled != 1 {
		t.Fatalf("expected handler to be called once, got %d", handlerCalled)
	}

	// Second request should be rate limited and handler should NOT be called
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/webhook", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	r.ServeHTTP(w, req)
	if handlerCalled != 1 {
		t.Errorf("expected handler NOT to be called on 429, but it was called %d times total", handlerCalled)
	}
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", w.Code)
	}
}

func TestWebhookRateLimitMiddleware_NilConfig(t *testing.T) {
	t.Parallel()

	r := gin.New()
	r.Use(WebhookRateLimitMiddleware(nil))
	r.GET("/webhook", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/webhook", nil)
	req.RemoteAddr = "10.0.0.1:5000"
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 with nil config (defaults), got %d", w.Code)
	}

	// Should use default MaxRequests=2000
	limitHeader := w.Header().Get("X-RateLimit-Limit")
	if limitHeader != "2000" {
		t.Errorf("expected X-RateLimit-Limit=2000 (default), got %q", limitHeader)
	}
}

func TestWebhookRateLimitMiddleware_NilLogger(t *testing.T) {
	t.Parallel()

	cfg := &WebhookRateLimiterConfig{
		MaxRequests: 1,
		WindowSize:  time.Minute,
		UseRedis:    false,
		KeyPrefix:   "test_rl",
		Logger:      nil, // nil logger should not panic
	}

	r := gin.New()
	r.Use(WebhookRateLimitMiddleware(cfg))
	r.GET("/webhook", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// First request
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/webhook", nil)
	req.RemoteAddr = "10.0.0.1:5000"
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Second request triggers 429 -- should not panic with nil logger
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/webhook", nil)
	req.RemoteAddr = "10.0.0.1:5000"
	r.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429 with nil logger, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// WebhookRateLimitMiddlewareWithContext
// ---------------------------------------------------------------------------

func TestWebhookRateLimitMiddlewareWithContext_ContextCancellationStopsCleanup(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())

	cfg := &WebhookRateLimiterConfig{
		MaxRequests: 100,
		WindowSize:  time.Minute,
		UseRedis:    false,
		KeyPrefix:   "test_rl",
		Logger:      zaptest.NewLogger(t),
	}

	r := gin.New()
	r.Use(WebhookRateLimitMiddlewareWithContext(cfg, ctx))
	r.GET("/webhook", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Verify middleware works before cancellation
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/webhook", nil)
	req.RemoteAddr = "10.0.0.1:5000"
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Cancel context -- this should stop the cleanup goroutine
	cancel()

	// Give goroutine a moment to receive the cancellation
	time.Sleep(10 * time.Millisecond)

	// Middleware should still function after context cancellation
	// (only the cleanup loop is affected, not the rate limiting itself)
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/webhook", nil)
	req.RemoteAddr = "10.0.0.1:5000"
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 after context cancellation, got %d", w.Code)
	}
}

func TestWebhookRateLimitMiddlewareWithContext_UsesProvidedContext(t *testing.T) {
	t.Parallel()

	// Create a context that is already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cfg := &WebhookRateLimiterConfig{
		MaxRequests: 5,
		WindowSize:  time.Minute,
		UseRedis:    false,
		KeyPrefix:   "test_rl",
		Logger:      zaptest.NewLogger(t),
	}

	// Should not hang or panic even with pre-cancelled context
	r := gin.New()
	r.Use(WebhookRateLimitMiddlewareWithContext(cfg, ctx))
	r.GET("/webhook", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/webhook", nil)
	req.RemoteAddr = "10.0.0.1:5000"
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 with pre-cancelled context, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// cleanupLoop
// ---------------------------------------------------------------------------

func TestCleanupLoop_ExpiredEntriesRemoved(t *testing.T) {
	t.Parallel()

	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := &WebhookRateLimiterConfig{
		MaxRequests: 100,
		WindowSize:  10 * time.Millisecond, // very short window
		UseRedis:    false,
		KeyPrefix:   "test_rl",
		Logger:      zaptest.NewLogger(t),
	}

	rl := &webhookRateLimiter{
		config:   cfg,
		memStore: make(map[string]*rateLimitEntry),
	}

	// Add entries that will expire very quickly
	now := time.Now()
	rl.memStore["expired-ip-1"] = &rateLimitEntry{count: 5, resetAt: now.Add(-10 * time.Millisecond)}
	rl.memStore["expired-ip-2"] = &rateLimitEntry{count: 3, resetAt: now.Add(-5 * time.Millisecond)}
	rl.memStore["active-ip"] = &rateLimitEntry{count: 1, resetAt: now.Add(10 * time.Minute)}

	// Run cleanup directly (not via goroutine) by simulating what cleanupLoop does
	rl.memStoreMu.Lock()
	cleanupNow := time.Now()
	for k, v := range rl.memStore {
		if cleanupNow.After(v.resetAt) {
			delete(rl.memStore, k)
		}
	}
	rl.memStoreMu.Unlock()

	rl.memStoreMu.RLock()
	defer rl.memStoreMu.RUnlock()

	if _, exists := rl.memStore["expired-ip-1"]; exists {
		t.Error("expected expired-ip-1 to be cleaned up")
	}
	if _, exists := rl.memStore["expired-ip-2"]; exists {
		t.Error("expected expired-ip-2 to be cleaned up")
	}
	if _, exists := rl.memStore["active-ip"]; !exists {
		t.Error("expected active-ip to remain after cleanup")
	}
}

func TestCleanupLoop_StopsOnContextCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())

	cfg := &WebhookRateLimiterConfig{
		MaxRequests: 100,
		WindowSize:  time.Minute,
		UseRedis:    false,
		KeyPrefix:   "test_rl",
		Logger:      zaptest.NewLogger(t),
	}

	rl := &webhookRateLimiter{
		config:   cfg,
		memStore: make(map[string]*rateLimitEntry),
	}

	logger := zaptest.NewLogger(t)

	done := make(chan struct{})
	go func() {
		defer func() {
			// Capture panics from SafeGo or other unexpected issues
			if r := recover(); r != nil {
				logger.Error("cleanupLoop panicked in test", zap.Any("recover", r))
				t.Errorf("cleanupLoop panicked: %v", r)
			}
			close(done)
		}()
		rl.cleanupLoop(ctx)
	}()

	// Cancel context and wait for cleanup loop to exit
	cancel()

	select {
	case <-done:
		// cleanupLoop exited as expected
	case <-time.After(2 * time.Second):
		t.Fatal("cleanupLoop did not exit after context cancellation within timeout")
	}
}

// ---------------------------------------------------------------------------
// Concurrent access
// ---------------------------------------------------------------------------

func TestConcurrentAccess_SameIP(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := &WebhookRateLimiterConfig{
		MaxRequests: 100,
		WindowSize:  time.Minute,
		UseRedis:    false,
		KeyPrefix:   "test_rl",
		Logger:      zaptest.NewLogger(t),
	}
	rl := newWebhookRateLimiter(cfg, ctx)

	const goroutines = 50
	const requestsPerGoroutine = 4

	var wg sync.WaitGroup
	allowedCount := make(chan int, goroutines)
	rejectedCount := make(chan int, goroutines)

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			allowed := 0
			rejected := 0
			for r := 0; r < requestsPerGoroutine; r++ {
				ok, _, _ := rl.check("concurrent-ip")
				if ok {
					allowed++
				} else {
					rejected++
				}
			}
			allowedCount <- allowed
			rejectedCount <- rejected
		}()
	}

	wg.Wait()
	close(allowedCount)
	close(rejectedCount)

	totalAllowed := 0
	totalRejected := 0
	for a := range allowedCount {
		totalAllowed += a
	}
	for r := range rejectedCount {
		totalRejected += r
	}

	totalRequests := goroutines * requestsPerGoroutine
	if totalAllowed+totalRejected != totalRequests {
		t.Errorf("expected total=%d, got allowed=%d + rejected=%d = %d",
			totalRequests, totalAllowed, totalRejected, totalAllowed+totalRejected)
	}

	// With MaxRequests=100 and 200 total requests, exactly 100 should be allowed
	if totalAllowed != cfg.MaxRequests {
		t.Errorf("expected exactly %d allowed requests, got %d", cfg.MaxRequests, totalAllowed)
	}
	expectedRejected := totalRequests - cfg.MaxRequests
	if totalRejected != expectedRejected {
		t.Errorf("expected %d rejected requests, got %d", expectedRejected, totalRejected)
	}
}

func TestConcurrentAccess_MiddlewareHTTP(t *testing.T) {
	t.Parallel()

	cfg := &WebhookRateLimiterConfig{
		MaxRequests: 20,
		WindowSize:  time.Minute,
		UseRedis:    false,
		KeyPrefix:   "test_rl",
		Logger:      zaptest.NewLogger(t),
	}

	r := gin.New()
	r.Use(WebhookRateLimitMiddleware(cfg))
	r.GET("/webhook", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	const goroutines = 10
	const requestsPerGoroutine = 5
	// Total: 50 requests, limit: 20

	var wg sync.WaitGroup
	statusCodes := make(chan int, goroutines*requestsPerGoroutine)

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < requestsPerGoroutine; i++ {
				w := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, "/webhook", nil)
				req.RemoteAddr = "10.0.0.99:1234"
				r.ServeHTTP(w, req)
				statusCodes <- w.Code
			}
		}()
	}

	wg.Wait()
	close(statusCodes)

	okCount := 0
	tooManyCount := 0
	for code := range statusCodes {
		switch code {
		case http.StatusOK:
			okCount++
		case http.StatusTooManyRequests:
			tooManyCount++
		default:
			t.Errorf("unexpected status code: %d", code)
		}
	}

	if okCount != 20 {
		t.Errorf("expected exactly 20 OK responses, got %d", okCount)
	}
	if tooManyCount != 30 {
		t.Errorf("expected exactly 30 rate-limited responses, got %d", tooManyCount)
	}
}

// ---------------------------------------------------------------------------
// Different IPs get independent limits
// ---------------------------------------------------------------------------

func TestDifferentIPs_IndependentLimits(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := &WebhookRateLimiterConfig{
		MaxRequests: 3,
		WindowSize:  time.Minute,
		UseRedis:    false,
		KeyPrefix:   "test_rl",
		Logger:      zaptest.NewLogger(t),
	}
	rl := newWebhookRateLimiter(cfg, ctx)

	// Exhaust limit for IP-A
	for i := 0; i < 3; i++ {
		rl.check("ip-a")
	}
	allowed, _, _ := rl.check("ip-a")
	if allowed {
		t.Error("expected ip-a to be rate limited after 3 requests")
	}

	// IP-B should still have full allowance
	allowed, remaining, retryAfter := rl.check("ip-b")
	if !allowed {
		t.Error("expected ip-b to be allowed (independent from ip-a)")
	}
	if remaining != 2 {
		t.Errorf("expected remaining=2 for ip-b, got %d", remaining)
	}
	if retryAfter != 0 {
		t.Errorf("expected retryAfter=0 for ip-b, got %s", retryAfter)
	}
}

func TestDifferentIPs_IndependentLimits_Middleware(t *testing.T) {
	t.Parallel()

	cfg := &WebhookRateLimiterConfig{
		MaxRequests: 2,
		WindowSize:  time.Minute,
		UseRedis:    false,
		KeyPrefix:   "test_rl",
		Logger:      zaptest.NewLogger(t),
	}

	r := gin.New()
	r.Use(WebhookRateLimitMiddleware(cfg))
	r.GET("/webhook", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	ips := []string{"10.0.0.1:1000", "10.0.0.2:1000", "10.0.0.3:1000"}
	for _, ip := range ips {
		for i := 0; i < 2; i++ {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/webhook", nil)
			req.RemoteAddr = ip
			r.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				t.Errorf("ip=%s request %d: expected 200, got %d", ip, i+1, w.Code)
			}
		}
	}

	// Now each IP should be at its limit; next request from each should be 429
	for _, ip := range ips {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/webhook", nil)
		req.RemoteAddr = ip
		r.ServeHTTP(w, req)
		if w.Code != http.StatusTooManyRequests {
			t.Errorf("ip=%s: expected 429 after exhausting limit, got %d", ip, w.Code)
		}
	}
}

// ---------------------------------------------------------------------------
// Edge cases
// ---------------------------------------------------------------------------

func TestWebhookRateLimitMiddleware_ResponseBodyContainsError(t *testing.T) {
	t.Parallel()

	cfg := &WebhookRateLimiterConfig{
		MaxRequests: 1,
		WindowSize:  time.Minute,
		UseRedis:    false,
		KeyPrefix:   "test_rl",
		Logger:      zaptest.NewLogger(t),
	}

	r := gin.New()
	r.Use(WebhookRateLimitMiddleware(cfg))
	r.GET("/webhook", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Consume the single allowed request
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/webhook", nil)
	req.RemoteAddr = "10.0.0.1:5000"
	r.ServeHTTP(w, req)

	// Rate limited request
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/webhook", nil)
	req.RemoteAddr = "10.0.0.1:5000"
	r.ServeHTTP(w, req)

	body := w.Body.String()
	if body == "" {
		t.Error("expected non-empty response body on 429")
	}
	// The response JSON should contain "rate limit exceeded"
	if !containsSubstring(body, "rate limit exceeded") {
		t.Errorf("expected response body to contain 'rate limit exceeded', got %q", body)
	}
}

func TestCheckMemory_ManyIPs(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := &WebhookRateLimiterConfig{
		MaxRequests: 5,
		WindowSize:  time.Minute,
		UseRedis:    false,
		KeyPrefix:   "test_rl",
		Logger:      zaptest.NewLogger(t),
	}
	rl := newWebhookRateLimiter(cfg, ctx)

	// Create entries for many different IPs
	for i := 0; i < 1000; i++ {
		ip := fmt.Sprintf("192.168.%d.%d", i/256, i%256)
		allowed, _, _ := rl.check(ip)
		if !allowed {
			t.Errorf("expected first request from %s to be allowed", ip)
		}
	}

	// Verify store has all IPs
	rl.memStoreMu.RLock()
	storeSize := len(rl.memStore)
	rl.memStoreMu.RUnlock()

	if storeSize != 1000 {
		t.Errorf("expected 1000 entries in memStore, got %d", storeSize)
	}
}

func TestWebhookRateLimitMiddleware_DifferentHTTPMethods(t *testing.T) {
	t.Parallel()

	cfg := &WebhookRateLimiterConfig{
		MaxRequests: 3,
		WindowSize:  time.Minute,
		UseRedis:    false,
		KeyPrefix:   "test_rl",
		Logger:      zaptest.NewLogger(t),
	}

	r := gin.New()
	r.Use(WebhookRateLimitMiddleware(cfg))

	handler := func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
	r.GET("/webhook", handler)
	r.POST("/webhook", handler)
	r.PUT("/webhook", handler)
	r.DELETE("/webhook", handler)

	methods := []string{http.MethodGet, http.MethodPost, http.MethodPut}

	// All methods from the same IP share the same rate limit
	for _, method := range methods {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(method, "/webhook", nil)
		req.RemoteAddr = "10.0.0.5:8080"
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("method=%s: expected 200, got %d", method, w.Code)
		}
	}

	// 4th request (DELETE) should be rate limited since IP already has 3 requests
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/webhook", nil)
	req.RemoteAddr = "10.0.0.5:8080"
	r.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429 on 4th request (different method, same IP), got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// cleanupLoop — actual goroutine tests exercising the real ticker path
// ---------------------------------------------------------------------------

func TestCleanupLoop_ActualTicker_RemovesExpiredEntries(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Use a fast CleanupInterval so the real cleanupLoop ticks quickly in tests.
	cfg := &WebhookRateLimiterConfig{
		MaxRequests:     100,
		WindowSize:      10 * time.Millisecond,
		UseRedis:        false,
		KeyPrefix:       "test_cleanup_tick",
		Logger:          zaptest.NewLogger(t),
		CleanupInterval: 20 * time.Millisecond,
	}

	rl := &webhookRateLimiter{
		config:   cfg,
		memStore: make(map[string]*rateLimitEntry),
	}

	// Pre-populate with entries: some already expired, some still active.
	now := time.Now()
	rl.memStore["expired-a"] = &rateLimitEntry{count: 5, resetAt: now.Add(-50 * time.Millisecond)}
	rl.memStore["expired-b"] = &rateLimitEntry{count: 2, resetAt: now.Add(-10 * time.Millisecond)}
	rl.memStore["active-a"] = &rateLimitEntry{count: 1, resetAt: now.Add(10 * time.Minute)}

	logger := zaptest.NewLogger(t)

	// Run the REAL cleanupLoop in a goroutine.
	done := make(chan struct{})
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Info("cleanup goroutine recovered", zap.Any("r", r))
			}
			close(done)
		}()
		rl.cleanupLoop(ctx)
	}()

	// Wait enough for at least one tick cycle of 20ms to execute.
	time.Sleep(80 * time.Millisecond)

	// Cancel context to stop the goroutine.
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("cleanup goroutine did not exit in time")
	}

	rl.memStoreMu.RLock()
	defer rl.memStoreMu.RUnlock()

	if _, exists := rl.memStore["expired-a"]; exists {
		t.Error("expected expired-a to be removed by cleanup")
	}
	if _, exists := rl.memStore["expired-b"]; exists {
		t.Error("expected expired-b to be removed by cleanup")
	}
	if _, exists := rl.memStore["active-a"]; !exists {
		t.Error("expected active-a to survive cleanup")
	}
}

func TestCleanupLoop_ContextAlreadyCancelled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Already cancelled

	cfg := &WebhookRateLimiterConfig{
		MaxRequests:     10,
		WindowSize:      time.Minute,
		UseRedis:        false,
		KeyPrefix:       "test_cleanup_cancelled",
		Logger:          zaptest.NewLogger(t),
		CleanupInterval: 10 * time.Millisecond,
	}

	rl := &webhookRateLimiter{
		config:   cfg,
		memStore: make(map[string]*rateLimitEntry),
	}

	logger := zaptest.NewLogger(t)
	done := make(chan struct{})
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Info("cleanupLoop recovered from panic", zap.Any("r", r))
			}
			close(done)
		}()
		rl.cleanupLoop(ctx)
	}()

	select {
	case <-done:
		// cleanupLoop should exit almost immediately since context is already done.
	case <-time.After(2 * time.Second):
		t.Fatal("cleanupLoop did not exit promptly for already-cancelled context")
	}
}

func TestCleanupLoop_EmptyStore(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())

	cfg := &WebhookRateLimiterConfig{
		MaxRequests:     10,
		WindowSize:      time.Minute,
		UseRedis:        false,
		KeyPrefix:       "test_cleanup_empty",
		Logger:          zaptest.NewLogger(t),
		CleanupInterval: 15 * time.Millisecond,
	}

	rl := &webhookRateLimiter{
		config:   cfg,
		memStore: make(map[string]*rateLimitEntry),
	}

	logger := zaptest.NewLogger(t)
	done := make(chan struct{})
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Info("cleanupLoop panic in empty store test", zap.Any("r", r))
			}
			close(done)
		}()
		rl.cleanupLoop(ctx)
	}()

	// Let it run long enough for at least one tick to fire (15ms).
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("cleanupLoop did not exit after cancel")
	}

	// Store should still be empty (no panic).
	rl.memStoreMu.RLock()
	defer rl.memStoreMu.RUnlock()
	if len(rl.memStore) != 0 {
		t.Errorf("expected empty memStore, got %d entries", len(rl.memStore))
	}
}

// ---------------------------------------------------------------------------
// check — additional memStore branch coverage
// ---------------------------------------------------------------------------

func TestCheck_MemStore_NewEntry(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := &WebhookRateLimiterConfig{
		MaxRequests: 10,
		WindowSize:  time.Minute,
		UseRedis:    false,
		KeyPrefix:   "test_check_new",
		Logger:      zaptest.NewLogger(t),
	}
	rl := newWebhookRateLimiter(cfg, ctx)

	// First check for a new IP creates a new entry.
	allowed, remaining, retryAfter := rl.check("new-ip")
	if !allowed {
		t.Error("expected first request to be allowed")
	}
	if remaining != 9 {
		t.Errorf("remaining = %d, want 9", remaining)
	}
	if retryAfter != 0 {
		t.Errorf("retryAfter = %s, want 0", retryAfter)
	}

	// Verify entry was created.
	rl.memStoreMu.RLock()
	entry, exists := rl.memStore["new-ip"]
	rl.memStoreMu.RUnlock()
	if !exists {
		t.Fatal("expected entry to be created for new-ip")
	}
	if entry.count != 1 {
		t.Errorf("entry.count = %d, want 1", entry.count)
	}
}

func TestCheck_MemStore_ExactLimit(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := &WebhookRateLimiterConfig{
		MaxRequests: 3,
		WindowSize:  time.Minute,
		UseRedis:    false,
		KeyPrefix:   "test_check_exact",
		Logger:      zaptest.NewLogger(t),
	}
	rl := newWebhookRateLimiter(cfg, ctx)

	ip := "exact-limit-ip"

	// Send exactly MaxRequests requests.
	for i := 0; i < 3; i++ {
		allowed, _, _ := rl.check(ip)
		if !allowed {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}

	// The (MaxRequests+1)-th request should be rejected.
	allowed, remaining, retryAfter := rl.check(ip)
	if allowed {
		t.Error("request over limit should be rejected")
	}
	if remaining != 0 {
		t.Errorf("remaining = %d, want 0", remaining)
	}
	if retryAfter <= 0 {
		t.Error("expected retryAfter > 0 when over limit")
	}
}

func TestCheck_MemStore_ExpiredEntry_ResetsCounter(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := &WebhookRateLimiterConfig{
		MaxRequests: 2,
		WindowSize:  20 * time.Millisecond,
		UseRedis:    false,
		KeyPrefix:   "test_check_expired",
		Logger:      zaptest.NewLogger(t),
	}
	rl := newWebhookRateLimiter(cfg, ctx)

	ip := "expired-entry-ip"

	// Exhaust the limit.
	rl.check(ip)
	rl.check(ip)
	allowed, _, _ := rl.check(ip)
	if allowed {
		t.Fatal("expected request to be rejected after exhausting limit")
	}

	// Wait for window to expire.
	time.Sleep(30 * time.Millisecond)

	// After reset, should be allowed again.
	allowed, remaining, retryAfter := rl.check(ip)
	if !allowed {
		t.Error("expected request to be allowed after window expiry")
	}
	if remaining != 1 {
		t.Errorf("remaining = %d, want 1", remaining)
	}
	if retryAfter != 0 {
		t.Errorf("retryAfter = %s, want 0", retryAfter)
	}
}

func TestCheck_UseRedisTrueWithClient_FallsBackOnRedisError(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a Redis client pointing to an unreachable address so pipeline.Exec
	// fails, triggering the checkRedis -> checkMemory fallback path.
	fakeRedis := redis.NewClient(&redis.Options{
		Addr:        "127.0.0.1:1", // unreachable port
		DialTimeout: 10 * time.Millisecond,
		ReadTimeout: 10 * time.Millisecond,
	})
	defer fakeRedis.Close()

	cfg := &WebhookRateLimiterConfig{
		MaxRequests:     5,
		WindowSize:      time.Minute,
		UseRedis:        true,
		RedisClient:     fakeRedis,
		KeyPrefix:       "test_redis_err",
		Logger:          zaptest.NewLogger(t),
		CleanupInterval: time.Hour, // no cleanup needed
	}
	rl := newWebhookRateLimiter(cfg, ctx)

	// check() will enter checkRedis(), pipeline.Exec will fail,
	// then checkRedis falls back to checkMemory.
	allowed, remaining, _ := rl.check("redis-err-ip")
	if !allowed {
		t.Error("expected request to be allowed after Redis fallback to memory")
	}
	if remaining != 4 {
		t.Errorf("remaining = %d, want 4", remaining)
	}
}

func TestCheck_UseRedisTrueButClientNil_MemStoreFallback(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := &WebhookRateLimiterConfig{
		MaxRequests: 5,
		WindowSize:  time.Minute,
		UseRedis:    true,
		RedisClient: nil,
		KeyPrefix:   "test_redis_nil",
		Logger:      zaptest.NewLogger(t),
	}
	rl := newWebhookRateLimiter(cfg, ctx)

	// UseRedis=true but RedisClient=nil => falls back to memory via check().
	allowed, remaining, _ := rl.check("fallback-ip")
	if !allowed {
		t.Error("expected request to be allowed via memory fallback")
	}
	if remaining != 4 {
		t.Errorf("remaining = %d, want 4", remaining)
	}

	// Verify it persisted in memStore.
	rl.memStoreMu.RLock()
	_, exists := rl.memStore["fallback-ip"]
	rl.memStoreMu.RUnlock()
	if !exists {
		t.Error("expected entry in memStore after fallback check")
	}
}

// ---------------------------------------------------------------------------
// WebhookRateLimitMiddleware — no-logger 429 edge case
// ---------------------------------------------------------------------------

func TestWebhookRateLimitMiddleware_NoLogger_429DoesNotPanic(t *testing.T) {
	t.Parallel()

	cfg := &WebhookRateLimiterConfig{
		MaxRequests: 1,
		WindowSize:  time.Minute,
		UseRedis:    false,
		KeyPrefix:   "test_no_logger",
		Logger:      nil,
	}

	r := gin.New()
	r.Use(WebhookRateLimitMiddleware(cfg))
	r.GET("/webhook", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// First request: OK.
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/webhook", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Second request: 429, with nil logger -- must not panic.
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/webhook", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	r.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// checkRedis — successful path using miniredis
// ---------------------------------------------------------------------------

func TestCheckRedis_FirstRequestAllowed(t *testing.T) {
	t.Parallel()

	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rc.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := &WebhookRateLimiterConfig{
		MaxRequests:     10,
		WindowSize:      time.Minute,
		UseRedis:        true,
		RedisClient:     rc,
		KeyPrefix:       "test_redis",
		Logger:          zaptest.NewLogger(t),
		CleanupInterval: time.Hour,
	}
	rl := newWebhookRateLimiter(cfg, ctx)

	allowed, remaining, retryAfter := rl.checkRedis("192.168.1.1")

	if !allowed {
		t.Error("expected first request to be allowed")
	}
	if remaining != 9 {
		t.Errorf("expected remaining=9, got %d", remaining)
	}
	if retryAfter != 0 {
		t.Errorf("expected retryAfter=0, got %s", retryAfter)
	}
}

func TestCheckRedis_MultipleRequestsWithinLimit(t *testing.T) {
	t.Parallel()

	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rc.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := &WebhookRateLimiterConfig{
		MaxRequests:     5,
		WindowSize:      time.Minute,
		UseRedis:        true,
		RedisClient:     rc,
		KeyPrefix:       "test_redis",
		Logger:          zaptest.NewLogger(t),
		CleanupInterval: time.Hour,
	}
	rl := newWebhookRateLimiter(cfg, ctx)

	for i := 1; i <= 4; i++ {
		allowed, remaining, retryAfter := rl.checkRedis("10.0.0.1")
		if !allowed {
			t.Errorf("request %d: expected allowed=true", i)
		}
		expectedRemaining := 5 - i
		if remaining != expectedRemaining {
			t.Errorf("request %d: expected remaining=%d, got %d", i, expectedRemaining, remaining)
		}
		if retryAfter != 0 {
			t.Errorf("request %d: expected retryAfter=0, got %s", i, retryAfter)
		}
	}
}

func TestCheckRedis_ExactLimitStillAllowed(t *testing.T) {
	t.Parallel()

	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rc.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := &WebhookRateLimiterConfig{
		MaxRequests:     3,
		WindowSize:      time.Minute,
		UseRedis:        true,
		RedisClient:     rc,
		KeyPrefix:       "test_redis",
		Logger:          zaptest.NewLogger(t),
		CleanupInterval: time.Hour,
	}
	rl := newWebhookRateLimiter(cfg, ctx)

	var allowed bool
	var remaining int
	var retryAfter time.Duration

	// Send exactly MaxRequests requests.
	for i := 0; i < 3; i++ {
		allowed, remaining, retryAfter = rl.checkRedis("10.0.0.2")
	}

	// The last (3rd) request should still be allowed with remaining=0.
	if !allowed {
		t.Error("expected request at exact limit to be allowed")
	}
	if remaining != 0 {
		t.Errorf("expected remaining=0 at exact limit, got %d", remaining)
	}
	if retryAfter != 0 {
		t.Errorf("expected retryAfter=0 at exact limit, got %s", retryAfter)
	}
}

func TestCheckRedis_OverLimitRejected(t *testing.T) {
	t.Parallel()

	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rc.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := &WebhookRateLimiterConfig{
		MaxRequests:     3,
		WindowSize:      time.Minute,
		UseRedis:        true,
		RedisClient:     rc,
		KeyPrefix:       "test_redis",
		Logger:          zaptest.NewLogger(t),
		CleanupInterval: time.Hour,
	}
	rl := newWebhookRateLimiter(cfg, ctx)

	// Exhaust the limit.
	for i := 0; i < 3; i++ {
		rl.checkRedis("10.0.0.3")
	}

	// 4th request should be rejected.
	allowed, remaining, retryAfter := rl.checkRedis("10.0.0.3")

	if allowed {
		t.Error("expected request over limit to be rejected")
	}
	if remaining != 0 {
		t.Errorf("expected remaining=0 when over limit, got %d", remaining)
	}
	if retryAfter <= 0 {
		t.Error("expected retryAfter > 0 when over limit")
	}
}

func TestCheckRedis_TTLNegativeFallsBackToWindowSize(t *testing.T) {
	t.Parallel()

	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rc.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	windowSize := 30 * time.Second
	cfg := &WebhookRateLimiterConfig{
		MaxRequests:     2,
		WindowSize:      windowSize,
		UseRedis:        true,
		RedisClient:     rc,
		KeyPrefix:       "test_redis_ttl",
		Logger:          zaptest.NewLogger(t),
		CleanupInterval: time.Hour,
	}
	rl := newWebhookRateLimiter(cfg, ctx)

	// Make requests to exceed the limit.
	rl.checkRedis("10.0.0.4")
	rl.checkRedis("10.0.0.4")

	// Manually remove the TTL from the key to force TTL to return -1.
	key := "test_redis_ttl:ip:10.0.0.4"
	mr.SetTTL(key, 0) // removes TTL, making PERSIST

	// Next request should be over limit. TTL will be -1, so the code
	// should fall back to using WindowSize.
	allowed, remaining, retryAfter := rl.checkRedis("10.0.0.4")

	if allowed {
		t.Error("expected request over limit to be rejected")
	}
	if remaining != 0 {
		t.Errorf("expected remaining=0, got %d", remaining)
	}
	// When TTL is negative, retryAfter should fall back to WindowSize (30s).
	if retryAfter != windowSize {
		t.Errorf("expected retryAfter=%s (WindowSize fallback), got %s", windowSize, retryAfter)
	}
}

func TestCheckRedis_IndependentIPs(t *testing.T) {
	t.Parallel()

	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rc.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := &WebhookRateLimiterConfig{
		MaxRequests:     2,
		WindowSize:      time.Minute,
		UseRedis:        true,
		RedisClient:     rc,
		KeyPrefix:       "test_redis",
		Logger:          zaptest.NewLogger(t),
		CleanupInterval: time.Hour,
	}
	rl := newWebhookRateLimiter(cfg, ctx)

	// Exhaust limit for IP-A.
	rl.checkRedis("ip-a")
	rl.checkRedis("ip-a")
	allowed, _, _ := rl.checkRedis("ip-a")
	if allowed {
		t.Error("expected ip-a to be rate limited")
	}

	// IP-B should still have its full allowance.
	allowed, remaining, retryAfter := rl.checkRedis("ip-b")
	if !allowed {
		t.Error("expected ip-b to be allowed (independent from ip-a)")
	}
	if remaining != 1 {
		t.Errorf("expected remaining=1 for ip-b, got %d", remaining)
	}
	if retryAfter != 0 {
		t.Errorf("expected retryAfter=0 for ip-b, got %s", retryAfter)
	}
}

func TestCheckRedis_ViaCheckDelegation(t *testing.T) {
	t.Parallel()

	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rc.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := &WebhookRateLimiterConfig{
		MaxRequests:     5,
		WindowSize:      time.Minute,
		UseRedis:        true,
		RedisClient:     rc,
		KeyPrefix:       "test_redis_delegation",
		Logger:          zaptest.NewLogger(t),
		CleanupInterval: time.Hour,
	}
	rl := newWebhookRateLimiter(cfg, ctx)

	// Use check() which should delegate to checkRedis since UseRedis=true
	// and RedisClient is non-nil.
	allowed, remaining, retryAfter := rl.check("delegation-ip")
	if !allowed {
		t.Error("expected first request to be allowed via Redis delegation")
	}
	if remaining != 4 {
		t.Errorf("expected remaining=4, got %d", remaining)
	}
	if retryAfter != 0 {
		t.Errorf("expected retryAfter=0, got %s", retryAfter)
	}

	// Verify the key was created in Redis (not in memStore).
	key := "test_redis_delegation:ip:delegation-ip"
	if !mr.Exists(key) {
		t.Error("expected Redis key to exist after checkRedis")
	}

	// Verify memStore is empty (Redis path was used, not memory).
	rl.memStoreMu.RLock()
	memEntries := len(rl.memStore)
	rl.memStoreMu.RUnlock()
	if memEntries != 0 {
		t.Errorf("expected empty memStore when using Redis, got %d entries", memEntries)
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
