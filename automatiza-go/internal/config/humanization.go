// Package config: anti-ban humanization configuration.
//
// This file is the single source of truth for the humanization knobs. It reads
// every HUMANIZE_*/PACING_* env var once (default ON), holds the "medium"
// baseline, and resolves a per-automation `humanize_mode` preset
// (off|low|medium|high, default high) into concrete tuning for the three
// subsystems: presence choreography (internal/lib/humanize), send pacing
// (internal/lib/pacing) and per-recipient content variation (internal/lib/text).
package config

import (
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"automatiza-go/internal/lib/humanize"
	"automatiza-go/internal/lib/pacing"
	"automatiza-go/internal/lib/text"
)

// Humanization holds the parsed baseline plus infra knobs read once at startup.
type Humanization struct {
	Enabled bool

	humanizeBase humanize.Config
	pacingBase   pacing.Config
	varyBase     text.VaryOptions

	// Infra knobs consumed at wiring time / by the scheduler.
	PresenceTTL        time.Duration
	SchedulerJitterSec int
}

// Resolved is the concrete tuning for one automation's mode, ready to hand to
// the humanizer, the content varier, and the pacer.
type Resolved struct {
	Humanize humanize.Config
	Vary     text.VaryOptions
	Pacing   pacing.Config
}

var (
	humanizationInst *Humanization
	humanizationOnce sync.Once
)

// GetHumanization returns the process-wide humanization config, loading it once.
func GetHumanization() *Humanization {
	humanizationOnce.Do(func() { humanizationInst = loadHumanization() })
	return humanizationInst
}

// ResetHumanization clears the singleton (test helper).
func ResetHumanization() {
	humanizationInst = nil
	humanizationOnce = sync.Once{}
}

func loadHumanization() *Humanization {
	h := &Humanization{}
	h.Enabled = getEnvBool("HUMANIZE_ENABLED", true)

	hb := humanize.DefaultConfig()
	hb.Enabled = h.Enabled
	hb.TypingEnabled = getEnvBool("HUMANIZE_TYPING", true)
	hb.TypingProbability = getEnvFloat("HUMANIZE_TYPING_PROBABILITY", 0.85)
	hb.CharsPerSecond = getEnvFloat("HUMANIZE_TYPING_CPS", 4.0)
	hb.SpeedJitterPct = getEnvFloat("HUMANIZE_TYPING_SPEED_JITTER", 0.30)
	hb.MinTyping = envMs("HUMANIZE_TYPING_MIN_MS", 1500)
	hb.MaxTyping = envMs("HUMANIZE_TYPING_MAX_MS", 12000)
	hb.MediaTyping = envMs("HUMANIZE_MEDIA_TYPING_MS", 2500)
	hb.BurstThreshold = getEnvInt("HUMANIZE_BURST_THRESHOLD", 140)
	hb.PresenceEnabled = getEnvBool("HUMANIZE_PRESENCE", true)
	hb.SeenEnabled = getEnvBool("HUMANIZE_SEND_SEEN", true)
	hb.SeenProbability = getEnvFloat("HUMANIZE_SEEN_PROBABILITY", 0.6)
	h.humanizeBase = hb

	pb := pacing.DefaultConfig()
	pb.Enabled = getEnvBool("PACING_ENABLED", true)
	pb.BaseDelaySeconds = getEnvInt("PACING_BASE_DELAY_SECONDS", 3)
	pb.JitterMinPct = getEnvFloat("PACING_JITTER_MIN_PCT", 0.7)
	pb.JitterMaxPct = getEnvFloat("PACING_JITTER_MAX_PCT", 1.4)
	pb.MaxRunSpread = envMs("PACING_MAX_RUN_SPREAD_MS", 1200000)
	pb.ShuffleEnabled = getEnvBool("PACING_SHUFFLE_ENABLED", true)
	pb.MicroBreakEnabled = getEnvBool("PACING_MICROBREAK_ENABLED", true)
	pb.MicroBreakEveryMin = getEnvInt("PACING_MICROBREAK_EVERY_MIN", 8)
	pb.MicroBreakEveryMax = getEnvInt("PACING_MICROBREAK_EVERY_MAX", 15)
	pb.MicroBreakCooldownMinMs = getEnvInt("PACING_MICROBREAK_COOLDOWN_MIN_MS", 30000)
	pb.MicroBreakCooldownMaxMs = getEnvInt("PACING_MICROBREAK_COOLDOWN_MAX_MS", 90000)
	// Teto do espalhamento de UM fan-out de espelho (monitoring/offer_generate):
	// requisito de produto — a oferta espelhada precisa chegar em todos os
	// grupos em menos de 1 minuto TOTAL. 15s de orçamento de sleep, porque o
	// tempo real de envio (digitação capada no espelho + HTTP, serial por
	// chip) também consome o minuto. 0 desliga o teto.
	pb.FanoutBudget = envMs("PACING_FANOUT_BUDGET_MS", 15000)
	h.pacingBase = pb

	h.varyBase = text.VaryOptions{
		Spintax:      getEnvBool("HUMANIZE_SPINTAX", true),
		ZeroWidth:    getEnvBool("HUMANIZE_ZERO_WIDTH", true),
		ZeroWidthMax: getEnvInt("HUMANIZE_ZERO_WIDTH_MAX", 3),
		EmojiJitter:  getEnvBool("HUMANIZE_EMOJI_JITTER", true),
		EmojiSet:     emojiSetFromEnv("HUMANIZE_EMOJI_SET"),
		PunctJitter:  getEnvBool("HUMANIZE_PUNCT_JITTER", true),
		LineBreak:    getEnvBool("HUMANIZE_LINEBREAK", true),
	}

	h.PresenceTTL = envMs("HUMANIZE_PRESENCE_TTL_MS", 90000)
	h.SchedulerJitterSec = getEnvInt("SCHEDULER_INTERVAL_JITTER_SEC", 45)
	return h
}

// ForMode resolves the per-automation preset (default "high") into concrete
// tuning. nil/empty/unknown => "high".
func (h *Humanization) ForMode(mode *string) Resolved {
	hum := h.humanizeBase
	vary := h.varyBase
	pac := h.pacingBase

	if !h.Enabled {
		return Resolved{Humanize: disabledHumanize(hum), Vary: text.VaryOptions{}, Pacing: noJitter(pac)}
	}

	m := "high"
	if mode != nil && strings.TrimSpace(*mode) != "" {
		m = strings.ToLower(strings.TrimSpace(*mode))
	}

	switch m {
	case "off":
		return Resolved{Humanize: disabledHumanize(hum), Vary: text.VaryOptions{}, Pacing: noJitter(pac)}

	case "low":
		hum.TypingProbability = 0.4
		vary.ZeroWidthMax = 1
		vary.EmojiJitter = false
		vary.LineBreak = false
		pac.JitterMinPct, pac.JitterMaxPct = scaleJitter(pac.JitterMinPct, pac.JitterMaxPct, 0.5)
		pac.MicroBreakEveryMin = scaleEvery(pac.MicroBreakEveryMin, 2.0)
		pac.MicroBreakEveryMax = scaleEvery(pac.MicroBreakEveryMax, 2.0)

	case "medium":
		// env baseline, unchanged.

	default: // "high" and any unknown value
		if hum.TypingProbability < 0.95 {
			hum.TypingProbability = 0.95
		}
		if vary.ZeroWidth && vary.ZeroWidthMax < 4 {
			vary.ZeroWidthMax = 4
		}
		pac.JitterMinPct, pac.JitterMaxPct = scaleJitter(pac.JitterMinPct, pac.JitterMaxPct, 1.5)
		pac.MicroBreakEveryMin = scaleEvery(pac.MicroBreakEveryMin, 0.6)
		pac.MicroBreakEveryMax = scaleEvery(pac.MicroBreakEveryMax, 0.6)
	}

	return Resolved{Humanize: hum, Vary: vary, Pacing: pac}
}

// PacingConfigForMode is a convenience for the enqueue/dispatch loops that only
// need the pacing slice.
func (h *Humanization) PacingConfigForMode(mode *string) pacing.Config {
	return h.ForMode(mode).Pacing
}

// ----- helpers -----

func disabledHumanize(c humanize.Config) humanize.Config {
	c.Enabled = false
	c.TypingEnabled = false
	c.PresenceEnabled = false
	c.SeenEnabled = false
	return c
}

func noJitter(c pacing.Config) pacing.Config {
	c.Enabled = false
	c.MicroBreakEnabled = false
	c.ShuffleEnabled = false
	return c
}

// scaleJitter scales the jitter band's deviation from 1.0 by k.
func scaleJitter(lo, hi, k float64) (float64, float64) {
	return 1 - (1-lo)*k, 1 + (hi-1)*k
}

// scaleEvery scales a "send every N" threshold (lower = more frequent breaks).
func scaleEvery(n int, factor float64) int {
	v := int(float64(n)*factor + 0.5)
	if v < 1 {
		v = 1
	}
	return v
}

func emojiSetFromEnv(key string) []string {
	v := os.Getenv(key)
	if v == "" {
		return []string{"", "", "🔥", "✅", "👍", "🛒", "⚡"}
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts)+1)
	out = append(out, "") // always allow "no emoji"
	for _, p := range parts {
		out = append(out, strings.TrimSpace(p))
	}
	return out
}

func envMs(key string, defMs int) time.Duration {
	return time.Duration(getEnvInt(key, defMs)) * time.Millisecond
}

func getEnvInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || n < 0 {
		return def
	}
	return n
}

func getEnvFloat(key string, def float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
	if err != nil || f < 0 {
		return def
	}
	return f
}
