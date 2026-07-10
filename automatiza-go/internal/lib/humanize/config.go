// Package humanize orchestrates human-presence "choreography" around a real
// WhatsApp send — read receipt (sendSeen), online presence, and the typing
// indicator ("digitando…") with a human-like typing duration — to disguise the
// automated dispatch. Every step is best-effort and fail-open: a failure here
// must NEVER block or delay the actual message send beyond its own bounded sleep.
package humanize

import "time"

// Config holds the tuning knobs for the choreography. It is passed per-call
// (via Plan.Cfg) so the per-automation "humanize_mode" preset can scale it; the
// Humanizer itself is stateless with respect to tuning.
type Config struct {
	// Enabled is the master switch. When false, BeforeSend is a no-op.
	Enabled bool

	// Typing indicator.
	TypingEnabled     bool
	TypingProbability float64 // fraction of sends that show typing (0..1)
	CharsPerSecond    float64 // base typing speed
	SpeedJitterPct    float64 // ± fraction applied to CharsPerSecond per message
	MinTyping         time.Duration
	MaxTyping         time.Duration
	MediaTyping       time.Duration // "attaching" beat for media with little/no caption
	BurstThreshold    int           // rune count above which typing splits into bursts

	// Online presence.
	PresenceEnabled bool

	// Read receipt.
	SeenEnabled     bool
	SeenProbability float64 // fraction of sends preceded by a read receipt (0..1)
}

// DefaultConfig returns the built-in defaults (all ON, "medium"-ish baseline).
// The config singleton scales these per mode; these values also make the
// package usable/testable standalone.
func DefaultConfig() Config {
	return Config{
		Enabled:           true,
		TypingEnabled:     true,
		TypingProbability: 0.85,
		CharsPerSecond:    4.0,
		SpeedJitterPct:    0.30,
		MinTyping:         1500 * time.Millisecond,
		MaxTyping:         12 * time.Second,
		MediaTyping:       2500 * time.Millisecond,
		BurstThreshold:    140,
		PresenceEnabled:   true,
		SeenEnabled:       true,
		SeenProbability:   0.6,
	}
}
