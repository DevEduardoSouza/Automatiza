package pacing

import (
	"context"
	"testing"
	"time"
)

func TestStepJitterBounds(t *testing.T) {
	c := DefaultConfig() // enabled, jitter [0.7,1.4]
	base := 10
	lo := time.Duration(float64(base) * c.JitterMinPct * float64(time.Second))
	hi := time.Duration(float64(base) * c.JitterMaxPct * float64(time.Second))
	for i := 0; i < 2000; i++ {
		d := c.Step(base)
		if d < lo || d > hi {
			t.Fatalf("Step out of bounds: %v not in [%v,%v]", d, lo, hi)
		}
	}
}

func TestStepDisabledNoJitter(t *testing.T) {
	c := DefaultConfig()
	c.Enabled = false
	if got := c.Step(5); got != 5*time.Second {
		t.Fatalf("disabled Step = %v, want 5s", got)
	}
}

func TestStepFallbackBase(t *testing.T) {
	c := DefaultConfig()
	c.Enabled = false
	c.BaseDelaySeconds = 3
	if got := c.Step(0); got != 3*time.Second {
		t.Fatalf("fallback base = %v, want 3s", got)
	}
}

func TestClampSpread(t *testing.T) {
	c := Config{MaxRunSpread: time.Minute}
	if got := c.ClampSpread(2 * time.Minute); got != time.Minute {
		t.Fatalf("clamp = %v, want 1m", got)
	}
	if got := c.ClampSpread(30 * time.Second); got != 30*time.Second {
		t.Fatalf("no-clamp = %v, want 30s", got)
	}
}

func TestShuffleNoopWhenDisabled(t *testing.T) {
	c := Config{ShuffleEnabled: false}
	xs := []string{"a", "b", "c"}
	c.Shuffle(xs)
	if xs[0] != "a" || xs[1] != "b" || xs[2] != "c" {
		t.Fatalf("disabled shuffle mutated order: %v", xs)
	}
}

func TestShufflePreservesMultiset(t *testing.T) {
	c := Config{ShuffleEnabled: true}
	xs := []string{"a", "b", "c", "d", "e", "f"}
	orig := append([]string{}, xs...)
	c.Shuffle(xs)
	if len(xs) != len(orig) {
		t.Fatalf("length changed: %d != %d", len(xs), len(orig))
	}
	seen := map[string]int{}
	for _, v := range xs {
		seen[v]++
	}
	for _, v := range orig {
		if seen[v] == 0 {
			t.Fatalf("element %q missing after shuffle", v)
		}
		seen[v]--
	}
}

func TestMicroBreakFailOpen(t *testing.T) {
	ctx := context.Background()
	var nilLimiter *Limiter
	if d := nilLimiter.MicroBreak(ctx, "chip", DefaultConfig()); d != 0 {
		t.Fatalf("nil *Limiter MicroBreak = %v, want 0", d)
	}
	nilRdb := NewLimiter(nil, nil)
	if d := nilRdb.MicroBreak(ctx, "chip", DefaultConfig()); d != 0 {
		t.Fatalf("nil-rdb MicroBreak = %v, want 0", d)
	}
	disabled := DefaultConfig()
	disabled.MicroBreakEnabled = false
	if d := NewLimiter(nil, nil).MicroBreak(ctx, "chip", disabled); d != 0 {
		t.Fatalf("disabled MicroBreak = %v, want 0", d)
	}
}

func TestReserveDailyDisabledCap(t *testing.T) {
	// cap<=0 disables the check → always allowed (fail-open), no Redis needed.
	l := NewLimiter(nil, nil)
	ok, used := l.ReserveDaily(context.Background(), "chip", "20260101", 0, 3600)
	if !ok || used != 0 {
		t.Fatalf("disabled daily cap: ok=%v used=%d, want true/0", ok, used)
	}
}
