// Package middleware provides HTTP middleware for the API.
// This file implements rate limiting for webhook endpoints.
package middleware

import (
	"automatiza-go/internal/observability"
	"context"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// WebhookRateLimiterConfig contains configuration for webhook rate limiting.
type WebhookRateLimiterConfig struct {
	// MaxRequests is the maximum number of requests per IP per window.
	MaxRequests int

	// WindowSize is the duration of the rate limit window.
	WindowSize time.Duration

	// UseRedis enables Redis-backed rate limiting for distributed deployments.
	// If false, uses in-memory rate limiting (per-instance).
	UseRedis bool

	// RedisClient is required if UseRedis is true.
	RedisClient *redis.Client

	// KeyPrefix is the Redis key prefix for rate limiting.
	KeyPrefix string

	// Logger for logging rate limit events.
	Logger *zap.Logger

	// CleanupInterval controls how often expired in-memory entries are purged.
	// Defaults to 1 minute when zero.
	CleanupInterval time.Duration
}

// DefaultWebhookRateLimitConfig returns default configuration.
// 2000 requests per minute per IP — WAHA sends webhooks from a single IP in bursts.
func DefaultWebhookRateLimitConfig() *WebhookRateLimiterConfig {
	return &WebhookRateLimiterConfig{
		MaxRequests: 2000,
		WindowSize:  time.Minute,
		UseRedis:    false,
		KeyPrefix:   "webhook_rl",
	}
}

// webhookRateLimiter implements rate limiting for webhooks.
type webhookRateLimiter struct {
	config *WebhookRateLimiterConfig

	// In-memory rate limiting (fallback when Redis unavailable)
	memStore   map[string]*rateLimitEntry
	memStoreMu sync.RWMutex
}

type rateLimitEntry struct {
	count   int
	resetAt time.Time
}

// newWebhookRateLimiter creates a new webhook rate limiter.
// The provided context controls the lifetime of the cleanup goroutine.
func newWebhookRateLimiter(config *WebhookRateLimiterConfig, ctx context.Context) *webhookRateLimiter {
	if config == nil {
		config = DefaultWebhookRateLimitConfig()
	}

	rl := &webhookRateLimiter{
		config:   config,
		memStore: make(map[string]*rateLimitEntry),
	}

	// Start cleanup goroutine for in-memory store with context cancellation (T3.9)
	observability.SafeGo("webhook-rate-limiter-cleanup", func() {
		rl.cleanupLoop(ctx)
	})

	return rl
}

// cleanupLoop periodically removes expired entries from the in-memory store.
// Respects context cancellation for graceful shutdown (T3.9).
func (rl *webhookRateLimiter) cleanupLoop(ctx context.Context) {
	interval := rl.config.CleanupInterval
	if interval <= 0 {
		interval = time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			rl.memStoreMu.Lock()
			now := time.Now()
			for k, v := range rl.memStore {
				if now.After(v.resetAt) {
					delete(rl.memStore, k)
				}
			}
			rl.memStoreMu.Unlock()
		}
	}
}

// check performs rate limit check and returns true if allowed.
func (rl *webhookRateLimiter) check(ip string) (allowed bool, remaining int, retryAfter time.Duration) {
	// Try Redis first if enabled
	if rl.config.UseRedis && rl.config.RedisClient != nil {
		return rl.checkRedis(ip)
	}

	// Fall back to in-memory
	return rl.checkMemory(ip)
}

// checkRedis performs rate limiting using Redis.
// Uses context.WithTimeout (T3.9) to prevent indefinite blocking on Redis.
func (rl *webhookRateLimiter) checkRedis(ip string) (bool, int, time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	key := rl.config.KeyPrefix + ":ip:" + ip

	pipe := rl.config.RedisClient.Pipeline()
	incrCmd := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, rl.config.WindowSize)
	ttlCmd := pipe.TTL(ctx, key)

	_, err := pipe.Exec(ctx)
	if err != nil {
		// On Redis error, fall back to memory
		if rl.config.Logger != nil {
			rl.config.Logger.Warn("Redis rate limit error, falling back to memory",
				zap.String("ip", ip),
				zap.Error(err))
		}
		return rl.checkMemory(ip)
	}

	count := int(incrCmd.Val())
	ttl := ttlCmd.Val()
	if ttl < 0 {
		ttl = rl.config.WindowSize
	}

	remaining := rl.config.MaxRequests - count
	if remaining < 0 {
		remaining = 0
	}

	if count > rl.config.MaxRequests {
		return false, remaining, ttl
	}

	return true, remaining, 0
}

// checkMemory performs rate limiting using in-memory store.
func (rl *webhookRateLimiter) checkMemory(ip string) (bool, int, time.Duration) {
	now := time.Now()

	rl.memStoreMu.Lock()
	defer rl.memStoreMu.Unlock()

	entry, exists := rl.memStore[ip]
	if !exists || now.After(entry.resetAt) {
		// New entry or expired
		rl.memStore[ip] = &rateLimitEntry{
			count:   1,
			resetAt: now.Add(rl.config.WindowSize),
		}
		return true, rl.config.MaxRequests - 1, 0
	}

	// Increment existing entry
	entry.count++

	remaining := rl.config.MaxRequests - entry.count
	if remaining < 0 {
		remaining = 0
	}

	if entry.count > rl.config.MaxRequests {
		retryAfter := entry.resetAt.Sub(now)
		return false, remaining, retryAfter
	}

	return true, remaining, 0
}

// WebhookRateLimitMiddleware creates a Gin middleware for webhook rate limiting.
// The cleanup goroutine uses a background context; for server-lifetime control,
// use WebhookRateLimitMiddlewareWithContext.
func WebhookRateLimitMiddleware(config *WebhookRateLimiterConfig) gin.HandlerFunc {
	return WebhookRateLimitMiddlewareWithContext(config, context.Background())
}

// WebhookRateLimitMiddlewareWithContext creates a Gin middleware for webhook rate limiting.
// The ctx parameter controls the lifetime of the background cleanup goroutine (T3.9).
func WebhookRateLimitMiddlewareWithContext(config *WebhookRateLimiterConfig, ctx context.Context) gin.HandlerFunc {
	limiter := newWebhookRateLimiter(config, ctx)

	return func(c *gin.Context) {
		ip := c.ClientIP()

		allowed, remaining, retryAfter := limiter.check(ip)

		// Set rate limit headers
		c.Header("X-RateLimit-Limit", strconv.Itoa(limiter.config.MaxRequests))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(remaining))

		if !allowed {
			if limiter.config.Logger != nil {
				limiter.config.Logger.Warn("webhook rate limit exceeded",
					zap.String("ip", ip),
					zap.Duration("retry_after", retryAfter))
			}

			c.Header("Retry-After", retryAfter.String())
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "rate limit exceeded",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
