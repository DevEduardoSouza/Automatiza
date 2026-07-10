package humanize

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Throttle decides whether the online-presence call should fire for a session
// in the current window. Setting presence "online" on every single message is
// itself a robotic pattern (and wasteful), so we throttle it to at most once
// per window per session, shared across processes.
type Throttle interface {
	ShouldFire(ctx context.Context, session string) bool
}

// redisSetNX is the minimal surface we need from a redis client (both
// *redis.Client and redis.UniversalClient satisfy it).
type redisSetNX interface {
	SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.BoolCmd
}

// RedisThrottle refreshes presence at most once per window per session. Whoever
// wins the atomic SETNX in a window is the only caller that fires the presence
// call; everyone else skips. Fail-open: on any Redis error it ALLOWS the call
// (returns true) — presence is harmless/idempotent, so erring toward firing it
// occasionally more often is fine.
type RedisThrottle struct {
	rdb    redisSetNX
	window time.Duration
	logger *zap.Logger
}

// NewRedisThrottle builds a throttle. A nil rdb yields a nil throttle-behavior
// (always fire) via the ShouldFire guard.
func NewRedisThrottle(rdb redisSetNX, window time.Duration, logger *zap.Logger) *RedisThrottle {
	if logger == nil {
		logger = zap.NewNop()
	}
	if window <= 0 {
		window = 90 * time.Second
	}
	return &RedisThrottle{rdb: rdb, window: window, logger: logger}
}

// ShouldFire reports whether presence should be refreshed for this session now.
func (t *RedisThrottle) ShouldFire(ctx context.Context, session string) bool {
	if t == nil || t.rdb == nil || session == "" {
		return true // no throttle available → allow
	}
	ok, err := t.rdb.SetNX(ctx, "presence:online:"+session, 1, t.window).Result()
	if err != nil {
		t.logger.Warn("presence throttle failed, allowing", zap.String("session", session), zap.Error(err))
		return true // fail-open: allow
	}
	return ok
}
