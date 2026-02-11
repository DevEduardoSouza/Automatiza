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

	"github.com/getsentry/sentry-go"
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
}

// DefaultWebhookRateLimitConfig returns default configuration.
// 500 requests per minute per IP - generous for legitimate WAHA webhooks.
func DefaultWebhookRateLimitConfig() *WebhookRateLimiterConfig {
	return &WebhookRateLimiterConfig{
		MaxRequests: 500,
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
func newWebhookRateLimiter(config *WebhookRateLimiterConfig) *webhookRateLimiter {
	if config == nil {
		config = DefaultWebhookRateLimitConfig()
	}

	rl := &webhookRateLimiter{
		config:   config,
		memStore: make(map[string]*rateLimitEntry),
	}

	// Start cleanup goroutine for in-memory store
	go rl.cleanupLoop()

	return rl
}

// cleanupLoop periodically removes expired entries from the in-memory store.
func (rl *webhookRateLimiter) cleanupLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
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
func (rl *webhookRateLimiter) checkRedis(ip string) (bool, int, time.Duration) {
	ctx := context.Background()
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
func WebhookRateLimitMiddleware(config *WebhookRateLimiterConfig) gin.HandlerFunc {
	limiter := newWebhookRateLimiter(config)

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
			// N16: Capture rate limit violations in Sentry for observability
			observability.CaptureMessage(sentry.LevelWarning, "webhook rate limit exceeded",
				map[string]string{"module": "webhook-rate-limiter", "ip": ip})

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
