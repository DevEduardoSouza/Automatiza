package pacing

import (
	"context"
	mathrand "math/rand/v2"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// microBreakScript atomically counts sends per chip and, every N sends (where N
// is supplied fresh by Go each call), returns a cooldown to inject before the
// next send — a short "stop" that breaks the machine-gun cadence. State lives in
// Redis so it survives across runs and is shared by automations sharing a chip.
//
// KEYS[1]=burst:count:<chip>  KEYS[2]=burst:next:<chip>
// ARGV[1]=initialN  ARGV[2]=nextN  ARGV[3]=cooldownMs  ARGV[4]=ttlMs
var microBreakScript = redis.NewScript(`
local ttl = tonumber(ARGV[4])
local c = redis.call('INCR', KEYS[1])
redis.call('PEXPIRE', KEYS[1], ttl)
local nxt = redis.call('GET', KEYS[2])
if not nxt then
  nxt = tonumber(ARGV[1])
  redis.call('SET', KEYS[2], nxt, 'PX', ttl)
else
  nxt = tonumber(nxt)
end
if c >= nxt then
  redis.call('SET', KEYS[2], c + tonumber(ARGV[2]), 'PX', ttl)
  return tonumber(ARGV[3])
end
redis.call('PEXPIRE', KEYS[2], ttl)
return 0
`)

// dailyScript is the OPTIONAL per-chip daily counter (built but disabled by
// default — the dispatcher never calls it unless daily caps are turned on). It
// atomically checks a cap then increments; returns -1 when already at cap
// (denied, no increment), else the new count.
//
// KEYS[1]=sent:daily:<chip>:<date>  ARGV[1]=cap  ARGV[2]=ttlSeconds
var dailyScript = redis.NewScript(`
local cur = tonumber(redis.call('GET', KEYS[1]) or '0')
local cap = tonumber(ARGV[1])
if cap > 0 and cur >= cap then return -1 end
local n = redis.call('INCR', KEYS[1])
if n == 1 then redis.call('EXPIRE', KEYS[1], tonumber(ARGV[2])) end
return n
`)

// Limiter is the Redis-backed side of pacing (micro-breaks + optional daily
// counter). A nil Limiter (or nil rdb) is safe: every method fails open (returns
// 0 / allowed) so a missing or broken Redis never blocks a send.
type Limiter struct {
	rdb    redis.Scripter
	logger *zap.Logger
}

// NewLimiter builds a Limiter. nil rdb yields a nil-safe no-op limiter.
func NewLimiter(rdb redis.Scripter, logger *zap.Logger) *Limiter {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Limiter{rdb: rdb, logger: logger}
}

// MicroBreak returns an extra cooldown to add to this chip's next scheduled
// delay when a burst boundary is crossed, else 0. Fail-open: nil limiter,
// disabled config, empty chip, or any Redis error → 0.
func (l *Limiter) MicroBreak(ctx context.Context, chip string, cfg Config) time.Duration {
	if l == nil || l.rdb == nil || chip == "" || !cfg.MicroBreakEnabled {
		return 0
	}
	initialN := jitterInt(cfg.MicroBreakEveryMin, cfg.MicroBreakEveryMax, 8, 15)
	nextN := jitterInt(cfg.MicroBreakEveryMin, cfg.MicroBreakEveryMax, 8, 15)
	cooldownMs := jitterInt(cfg.MicroBreakCooldownMinMs, cfg.MicroBreakCooldownMaxMs, 30000, 90000)
	ttlMs := cfg.MicroBreakCooldownMaxMs*4 + 300000
	if ttlMs <= 0 {
		ttlMs = 660000
	}
	keys := []string{"pacing:burst:count:" + chip, "pacing:burst:next:" + chip}
	res, err := microBreakScript.Run(ctx, l.rdb, keys, initialN, nextN, cooldownMs, ttlMs).Int64()
	if err != nil {
		l.logger.Warn("micro-break check failed, no cooldown", zap.String("chip", chip), zap.Error(err))
		return 0
	}
	if res <= 0 {
		return 0
	}
	return time.Duration(res) * time.Millisecond
}

// ReserveDaily is the OPTIONAL daily-cap gate (disabled by default; not wired
// into the dispatch loops). allowed=false only when already at cap (no
// increment). Fail-open: allowed=true on any error. cap<=0 disables the check.
func (l *Limiter) ReserveDaily(ctx context.Context, chip, date string, cap int, ttlSeconds int) (allowed bool, used int) {
	if l == nil || l.rdb == nil || chip == "" || cap <= 0 {
		return true, 0
	}
	key := "pacing:sent:daily:" + chip + ":" + date
	res, err := dailyScript.Run(ctx, l.rdb, []string{key}, cap, ttlSeconds).Int64()
	if err != nil {
		l.logger.Warn("daily cap check failed, allowing", zap.String("chip", chip), zap.Error(err))
		return true, 0
	}
	if res < 0 {
		return false, cap
	}
	return true, int(res)
}

// jitterInt returns a uniform int in [min,max]. Falls back to [dmin,dmax] when
// the configured bounds are unset/invalid.
func jitterInt(min, max, dmin, dmax int) int {
	if min <= 0 && max <= 0 {
		min, max = dmin, dmax
	}
	if max < min {
		min, max = max, min
	}
	if max <= min {
		return min
	}
	return min + mathrand.IntN(max-min+1)
}
