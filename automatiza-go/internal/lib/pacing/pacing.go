// Package pacing computes anti-ban send rhythm: jittered inter-message delays,
// recipient-order shuffling, and (via Limiter) Redis-backed "micro-breaks" — short
// stops after N sends on a chip. It deliberately only SPACES and PAUSES sends; it
// never drops a recipient. All tuning comes from Config (populated centrally by
// internal/config/humanization.go and scaled per automation "humanize_mode").
package pacing

import (
	"context"
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

	// FanoutBudget caps the TOTAL sleep spread of one monitor-flow fan-out
	// (monitoring/offer_generate): the mirror must reach every destination
	// group well under a minute, so per-gap sleeps shrink as the deadline
	// approaches (see BudgetedGap). Zero disables the cap.
	FanoutBudget time.Duration
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

// FanoutDeadline returns the wall-clock instant by which a monitor-flow
// fan-out must have finished sleeping. Zero budget → zero time (no cap).
func (c Config) FanoutDeadline() time.Time {
	if c.FanoutBudget <= 0 {
		return time.Time{}
	}
	return time.Now().Add(c.FanoutBudget)
}

// BudgetedGap clamps one inter-message gap so the `remaining` sends of this
// fan-out (current included) still fit before the deadline. Send time itself
// (typing simulation, HTTP) eats budget naturally: the next call sees less
// time left and shrinks further, down to zero. A user-configured
// delay_seconds bigger than the budget is deliberately overridden here —
// the product guarantee "mirror lands in under a minute" wins for these
// flows; drip-feed pacing still applies to bulk dispatch types.
func BudgetedGap(gap time.Duration, deadline time.Time, remaining int) time.Duration {
	if deadline.IsZero() {
		return gap
	}
	if remaining < 1 {
		remaining = 1
	}
	maxGap := time.Until(deadline) / time.Duration(remaining)
	if maxGap < 0 {
		maxGap = 0
	}
	if gap > maxGap {
		return maxGap
	}
	return gap
}

// Sleep espera `d` respeitando o cancelamento do contexto.
//
// Devolve false quando o contexto morreu durante (ou antes) da espera — nesse
// caso o chamador DEVE parar o fan-out em vez de seguir para o próximo envio.
//
// Existe porque `time.Sleep` é surdo: com uma troca de chip em andamento, a
// lease de dispatch pode ser perdida no meio de um fan-out de dezenas de
// grupos, e um sleep não-cancelável continuaria acordando para enviar pelo chip
// que já não é o roteado. d <= 0 é no-op e devolve o estado atual do contexto.
func Sleep(ctx context.Context, d time.Duration) bool {
	if ctx == nil {
		time.Sleep(d)
		return true
	}
	if d <= 0 {
		return ctx.Err() == nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
