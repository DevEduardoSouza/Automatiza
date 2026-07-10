// Package pacing computes anti-ban send rhythm: jittered inter-message delays,
// recipient-order shuffling, and (via Limiter) Redis-backed "micro-breaks" — short
// stops after N sends on a chip. It deliberately only SPACES and PAUSES sends; it
// never drops a recipient. All tuning comes from Config (populated centrally by
// internal/config/humanization.go and scaled per automation "humanize_mode").
package pacing

import (
	mathrand "math/rand/v2"
	"time"
)

// Config holds the pacing tuning knobs. It is a plain value passed per call so
// the per-automation preset can scale it.
type Config struct {
	// Enabled toggles jitter/micro-breaks. When false, Step returns the plain
	// base delay (still honoring the user's delay_seconds) with no jitter.
	Enabled bool

	// Inter-message delay.
	BaseDelaySeconds int     // fallback base when the caller's base <= 0
	JitterMinPct     float64 // e.g. 0.7
	JitterMaxPct     float64 // e.g. 1.4
	MaxRunSpread     time.Duration

	// Recipient-order shuffle.
	ShuffleEnabled bool

	// Micro-breaks (consumed by Limiter.MicroBreak).
	MicroBreakEnabled       bool
	MicroBreakEveryMin      int // sends between breaks (jittered lower bound)
	MicroBreakEveryMax      int
	MicroBreakCooldownMinMs int
	MicroBreakCooldownMaxMs int
}

// DefaultConfig returns the built-in defaults (used standalone/in tests).
func DefaultConfig() Config {
	return Config{
		Enabled:                 true,
		BaseDelaySeconds:        3,
		JitterMinPct:            0.7,
		JitterMaxPct:            1.4,
		MaxRunSpread:            20 * time.Minute,
		ShuffleEnabled:          true,
		MicroBreakEnabled:       true,
		MicroBreakEveryMin:      8,
		MicroBreakEveryMax:      15,
		MicroBreakCooldownMinMs: 30000,
		MicroBreakCooldownMaxMs: 90000,
	}
}

// Step returns one jittered inter-message gap. baseSeconds is typically the
// automation's delay_seconds; when <= 0 it falls back to BaseDelaySeconds (then 3).
// When Enabled is false it returns the plain base with no jitter. Uses the
// goroutine-safe math/rand/v2 top-level RNG (safe under the offer_generate
// goroutine fan-out).
func (c Config) Step(baseSeconds int) time.Duration {
	b := baseSeconds
	if b <= 0 {
		b = c.BaseDelaySeconds
	}
	if b <= 0 {
		b = 3
	}
	if !c.Enabled {
		return time.Duration(b) * time.Second
	}
	lo, hi := c.JitterMinPct, c.JitterMaxPct
	if hi < lo {
		lo, hi = hi, lo
	}
	if lo <= 0 && hi <= 0 {
		lo, hi = 0.7, 1.4
	}
	f := lo
	if hi > lo {
		f = lo + mathrand.Float64()*(hi-lo)
	}
	return time.Duration(float64(b) * f * float64(time.Second))
}

// ClampSpread caps an accumulated per-chip delay to MaxRunSpread so a big group
// list can't push the last send past the next interval.
func (c Config) ClampSpread(d time.Duration) time.Duration {
	if c.MaxRunSpread > 0 && d > c.MaxRunSpread {
		return c.MaxRunSpread
	}
	return d
}

// Shuffle reorders xs in place (Fisher–Yates). No-op when ShuffleEnabled is false.
func (c Config) Shuffle(xs []string) {
	if !c.ShuffleEnabled || len(xs) < 2 {
		return
	}
	mathrand.Shuffle(len(xs), func(i, j int) { xs[i], xs[j] = xs[j], xs[i] })
}
