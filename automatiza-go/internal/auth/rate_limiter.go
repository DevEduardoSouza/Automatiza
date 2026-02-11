package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"automatiza-go/internal/observability"

	"github.com/getsentry/sentry-go"
	"github.com/redis/go-redis/v9"
)

// RateLimitConfig holds the configuration for rate limiting.
type RateLimitConfig struct {
	// MaxAttempts is the maximum number of attempts allowed in the window.
	MaxAttempts int

	// WindowSize is the duration of the sliding window.
	WindowSize time.Duration

	// BlockTime is how long to block after exceeding the limit.
	BlockTime time.Duration
}

// DefaultLoginRateLimit provides secure defaults for login rate limiting.
// IMPORTANT: These are production-safe defaults.
// - 10 attempts per 1 minute (secure default)
// - 5 minutes block after exceeding limit
// For development, override via RATE_LIMIT_MAX_ATTEMPTS=100 in .env
var DefaultLoginRateLimit = &RateLimitConfig{
	MaxAttempts: 10,
	WindowSize:  1 * time.Minute,
	BlockTime:   5 * time.Minute,
}

// DevelopmentLoginRateLimit provides relaxed limits for development.
// Use NewRateLimitConfig to get environment-appropriate settings.
var DevelopmentLoginRateLimit = &RateLimitConfig{
	MaxAttempts: 100,
	WindowSize:  1 * time.Minute,
	BlockTime:   1 * time.Minute,
}

// NewRateLimitConfig creates a rate limit config based on environment settings.
// If any value is <= 0, the corresponding default is used.
func NewRateLimitConfig(maxAttempts, windowSec, blockSec int) *RateLimitConfig {
	cfg := &RateLimitConfig{
		MaxAttempts: DefaultLoginRateLimit.MaxAttempts,
		WindowSize:  DefaultLoginRateLimit.WindowSize,
		BlockTime:   DefaultLoginRateLimit.BlockTime,
	}

	if maxAttempts > 0 {
		cfg.MaxAttempts = maxAttempts
	}
	if windowSec > 0 {
		cfg.WindowSize = time.Duration(windowSec) * time.Second
	}
	if blockSec > 0 {
		cfg.BlockTime = time.Duration(blockSec) * time.Second
	}

	return cfg
}

// RateLimiter provides rate limiting functionality using Redis.
type RateLimiter struct {
	client *redis.Client
	prefix string
	config *RateLimitConfig
}

// NewRateLimiter creates a new rate limiter with the given Redis client and configuration.
func NewRateLimiter(client *redis.Client, prefix string, config *RateLimitConfig) *RateLimiter {
	return &RateLimiter{
		client: client,
		prefix: prefix,
		config: config,
	}
}

// RateLimitResult contains the result of a rate limit check.
type RateLimitResult struct {
	// Allowed is true if the request is allowed.
	Allowed bool

	// Remaining is the number of remaining attempts.
	Remaining int

	// RetryAfter is the duration to wait before retrying (if blocked).
	RetryAfter time.Duration

	// ResetAt is when the rate limit window resets.
	ResetAt time.Time
}

// Check checks if the given key is rate limited.
// Returns the result indicating if the request is allowed.
func (r *RateLimiter) Check(ctx context.Context, key string) (*RateLimitResult, error) {
	fullKey := r.prefix + ":" + key
	blockKey := r.prefix + ":block:" + key

	// Check if blocked
	blocked, err := r.client.Exists(ctx, blockKey).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to check block status: %w", err)
	}

	if blocked > 0 {
		ttl, err := r.client.TTL(ctx, blockKey).Result()
		if err != nil {
			return nil, fmt.Errorf("failed to get block TTL: %w", err)
		}
		return &RateLimitResult{
			Allowed:    false,
			Remaining:  0,
			RetryAfter: ttl,
			ResetAt:    time.Now().Add(ttl),
		}, nil
	}

	// Get current count
	count, err := r.client.Get(ctx, fullKey).Int()
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, fmt.Errorf("failed to get rate limit count: %w", err)
	}

	remaining := r.config.MaxAttempts - count
	if remaining < 0 {
		remaining = 0
	}

	// Get TTL for reset time
	ttl, err := r.client.TTL(ctx, fullKey).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, fmt.Errorf("failed to get TTL: %w", err)
	}
	if ttl < 0 {
		ttl = r.config.WindowSize
	}

	return &RateLimitResult{
		Allowed:    remaining > 0,
		Remaining:  remaining,
		RetryAfter: 0,
		ResetAt:    time.Now().Add(ttl),
	}, nil
}

// Increment records an attempt for the given key.
// If the limit is exceeded, the key will be blocked.
// SECURITY: Uses fixed-window counter to prevent sliding window bypass attacks.
// The TTL is only set on the first request in the window, not reset on subsequent requests.
func (r *RateLimiter) Increment(ctx context.Context, key string) (*RateLimitResult, error) {
	fullKey := r.prefix + ":" + key
	blockKey := r.prefix + ":block:" + key

	// Check if already blocked
	blocked, err := r.client.Exists(ctx, blockKey).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to check block status: %w", err)
	}

	if blocked > 0 {
		ttl, err := r.client.TTL(ctx, blockKey).Result()
		if err != nil {
			return nil, fmt.Errorf("failed to get block TTL: %w", err)
		}
		return &RateLimitResult{
			Allowed:    false,
			Remaining:  0,
			RetryAfter: ttl,
			ResetAt:    time.Now().Add(ttl),
		}, nil
	}

	// Use Lua script for atomic increment with conditional TTL set.
	// This ensures the TTL is only set when the key is created (count == 1),
	// preventing window-slip attacks where attackers reset the window with each request.
	// Redis Lua script:
	// - INCR the counter
	// - Only set TTL if this is the first increment (counter == 1)
	// - Return the new count
	luaScript := `
		local count = redis.call('INCR', KEYS[1])
		if count == 1 then
			redis.call('EXPIRE', KEYS[1], ARGV[1])
		end
		return count
	`
	result, err := r.client.Eval(ctx, luaScript, []string{fullKey}, int(r.config.WindowSize.Seconds())).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to increment rate limit: %w", err)
	}

	countInt64, ok := result.(int64)
	if !ok {
		observability.CaptureMessage(sentry.LevelError, "rate limiter: unexpected type from Redis Eval result",
			map[string]string{"module": "rate-limiter", "expected": "int64", "got": fmt.Sprintf("%T", result)})
		// Fail open: allow the request rather than panic
		return &RateLimitResult{Allowed: true, Remaining: r.config.MaxAttempts}, nil
	}
	count := int(countInt64)
	remaining := r.config.MaxAttempts - count
	if remaining < 0 {
		remaining = 0
	}

	// Check if limit exceeded and block
	if count >= r.config.MaxAttempts {
		err = r.client.SetEx(ctx, blockKey, "1", r.config.BlockTime).Err()
		if err != nil {
			return nil, fmt.Errorf("failed to set block: %w", err)
		}
		return &RateLimitResult{
			Allowed:    false,
			Remaining:  0,
			RetryAfter: r.config.BlockTime,
			ResetAt:    time.Now().Add(r.config.BlockTime),
		}, nil
	}

	// Get TTL for reset time
	ttl, err := r.client.TTL(ctx, fullKey).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, fmt.Errorf("failed to get TTL: %w", err)
	}
	if ttl < 0 {
		ttl = r.config.WindowSize
	}

	return &RateLimitResult{
		Allowed:    remaining > 0,
		Remaining:  remaining,
		RetryAfter: 0,
		ResetAt:    time.Now().Add(ttl),
	}, nil
}

// Reset clears the rate limit for the given key.
// This should be called on successful authentication.
func (r *RateLimiter) Reset(ctx context.Context, key string) error {
	fullKey := r.prefix + ":" + key
	blockKey := r.prefix + ":block:" + key

	pipe := r.client.Pipeline()
	pipe.Del(ctx, fullKey)
	pipe.Del(ctx, blockKey)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to reset rate limit: %w", err)
	}

	return nil
}

// LoginRateLimitKey generates a rate limit key for login attempts.
// Uses both IP and email to prevent both credential stuffing and targeted attacks.
func LoginRateLimitKey(ip, email string) string {
	return fmt.Sprintf("login:%s:%s", ip, email)
}

// IPRateLimitKey generates a rate limit key for IP-based limiting.
func IPRateLimitKey(ip string) string {
	return fmt.Sprintf("ip:%s", ip)
}
