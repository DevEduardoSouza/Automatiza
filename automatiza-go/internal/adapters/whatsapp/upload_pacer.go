package whatsapp

import (
	"context"
	"os"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// UploadPacer spaces WhatsApp media uploads, per session, so they stay under
// WhatsApp's media-server rate limit. That limit surfaces as a WAHA HTTP 500
// wrapping "upload failed with status code 429/400" — which drops the offer to
// the text-only fallback ("promoção sem foto"). The retry in doRequest recovers
// those after the fact; pacing keeps them from happening in the first place.
//
// A nil UploadPacer (and a nil concrete implementation) is a no-op, so callers
// never need to nil-check before calling Reserve.
type UploadPacer interface {
	// Reserve returns how long the caller should wait before starting this
	// session's next media upload. Implementations MUST be fail-open: any
	// backend error (or an empty session) returns 0, never an error — pacing
	// must never block or break a send on its own.
	Reserve(ctx context.Context, session string) time.Duration
}

// uploadPaceScript reserves the next upload "slot" for a session and returns,
// in ms, how long to wait (slot - now). Atomic: each call advances the slot by
// `spacing`, so concurrent uploads to the SAME session — even from different
// processes (server relay + worker) — go out spaced instead of in a burst.
// It is not a rate limiter (it rejects nothing); it only pushes the schedule.
// Identical to the worker's session pacer script, but keyed per media upload.
var uploadPaceScript = redis.NewScript(`
local now = tonumber(ARGV[1])
local spacing = tonumber(ARGV[2])
local ttl = tonumber(ARGV[3])
local slot = now
local cur = redis.call('GET', KEYS[1])
if cur then
  local c = tonumber(cur)
  if c and c > now then slot = c end
end
redis.call('SET', KEYS[1], slot + spacing, 'PX', ttl)
return slot - now
`)

// RedisUploadPacer is the Redis-backed UploadPacer. Spacing is shared across
// processes via Redis, so the server (offer-generate relay) and the worker
// (queued sends) don't both upload media for the same session at once.
type RedisUploadPacer struct {
	rdb     redis.Scripter
	spacing time.Duration
	logger  *zap.Logger
}

// NewRedisUploadPacer returns nil (pacing disabled) when rdb is nil or spacing
// <= 0, mirroring the worker's session pacer.
func NewRedisUploadPacer(rdb redis.Scripter, spacing time.Duration, logger *zap.Logger) *RedisUploadPacer {
	if rdb == nil || spacing <= 0 {
		return nil
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &RedisUploadPacer{rdb: rdb, spacing: spacing, logger: logger}
}

// Reserve implements UploadPacer. Fail-open: nil pacer, empty session, or ANY
// Redis error returns 0 — the upload proceeds immediately rather than break.
func (p *RedisUploadPacer) Reserve(ctx context.Context, session string) time.Duration {
	if p == nil || session == "" {
		return 0
	}
	now := time.Now().UnixMilli()
	spacingMs := p.spacing.Milliseconds()
	// Generous TTL: the key expires on its own once the session goes idle, but
	// lives long enough to hold the slot through a burst.
	ttlMs := spacingMs*8 + 60_000
	key := "pace:media:" + session

	res, err := uploadPaceScript.Run(ctx, p.rdb, []string{key}, now, spacingMs, ttlMs).Int64()
	if err != nil {
		p.logger.Warn("media upload pacer failed, uploading without pacing",
			zap.String("session", session), zap.Error(err))
		return 0
	}
	if res <= 0 {
		return 0
	}
	return time.Duration(res) * time.Millisecond
}

// MediaPaceMsFromEnv reads MEDIA_PACE_MS. Absent/0/invalid => 0 (disabled).
func MediaPaceMsFromEnv() int {
	v := os.Getenv("MEDIA_PACE_MS")
	if v == "" {
		return 0
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return 0
	}
	return n
}
