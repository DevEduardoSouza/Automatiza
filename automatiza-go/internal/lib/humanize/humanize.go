package humanize

import (
	"context"
	mathrand "math/rand/v2"
	"time"
	"unicode/utf8"

	"automatiza-go/internal/adapters/whatsapp"

	"go.uber.org/zap"
)

// presenceCtl is the narrow slice of the WhatsApp provider the humanizer needs.
// *whatsapp.WahaAdapter satisfies it, which keeps this package decoupled from
// the full provider and trivially fakeable in tests.
type presenceCtl interface {
	StartTyping(ctx context.Context, in whatsapp.TypingInput) error
	StopTyping(ctx context.Context, in whatsapp.TypingInput) error
	SendSeen(ctx context.Context, in whatsapp.SendSeenInput) error
	SetPresence(ctx context.Context, in whatsapp.SetPresenceInput) error
}

// Humanizer runs the pre-send choreography. It is stateless with respect to
// tuning (the per-call Plan.Cfg carries that) and holds only infra: the
// provider, the presence throttle, and a logger.
type Humanizer struct {
	ctl      presenceCtl
	throttle Throttle
	logger   *zap.Logger
}

// New builds a Humanizer. A nil ctl (WhatsApp not configured) makes BeforeSend a
// no-op. A nil throttle means presence fires unthrottled.
func New(ctl presenceCtl, throttle Throttle, logger *zap.Logger) *Humanizer {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Humanizer{ctl: ctl, throttle: throttle, logger: logger}
}

// Plan describes one send the choreography wraps.
type Plan struct {
	Session  string
	ChatID   string
	Text     string
	HasMedia bool
	Cfg      Config
}

// BeforeSend runs the pre-send choreography (seen → presence online → typing…)
// and returns a cleanup() that stops the typing indicator. It ALWAYS returns a
// non-nil cleanup (a no-op when disabled), so callers can `defer cleanup()`
// unconditionally. It is context-aware: a canceled ctx aborts the typing sleep
// early. Everything is best-effort and never returns an error.
func (h *Humanizer) BeforeSend(ctx context.Context, p Plan) func() {
	noop := func() {}
	if h == nil || h.ctl == nil || !p.Cfg.Enabled || p.Session == "" || p.ChatID == "" {
		return noop
	}
	cfg := p.Cfg

	// (a) Simulate opening/reading the chat — probabilistic, fire-and-forget.
	if cfg.SeenEnabled && chance(cfg.SeenProbability) {
		dctx, cancel := detach(ctx)
		go func() {
			defer cancel()
			_ = h.ctl.SendSeen(dctx, whatsapp.SendSeenInput{SessionName: p.Session, ChatID: p.ChatID})
		}()
	}

	// (b) Refresh online presence — throttled per session, fire-and-forget.
	if cfg.PresenceEnabled && h.shouldFirePresence(ctx, p.Session) {
		dctx, cancel := detach(ctx)
		go func() {
			defer cancel()
			_ = h.ctl.SetPresence(dctx, whatsapp.SetPresenceInput{SessionName: p.Session, Presence: whatsapp.PresenceOnline})
		}()
	}

	// (c) Typing decision — humans sometimes paste/forward with no typing.
	if !cfg.TypingEnabled || !chance(cfg.TypingProbability) {
		return noop
	}

	// (d) Human typing time, optionally split into bursts for long messages.
	total := typingDuration(cfg, p.Text, p.HasMedia)
	bursts := planBursts(cfg, p.Text, total)
	for i, seg := range bursts {
		_ = h.ctl.StartTyping(ctx, whatsapp.TypingInput{SessionName: p.Session, ChatID: p.ChatID})
		if !sleepCtx(ctx, seg.typeFor) {
			return h.stopFn(p.Session, p.ChatID)
		}
		if i < len(bursts)-1 {
			_ = h.ctl.StopTyping(ctx, whatsapp.TypingInput{SessionName: p.Session, ChatID: p.ChatID})
			if !sleepCtx(ctx, seg.pauseAfter) {
				return h.stopFn(p.Session, p.ChatID)
			}
		}
	}
	return h.stopFn(p.Session, p.ChatID)
}

func (h *Humanizer) shouldFirePresence(ctx context.Context, session string) bool {
	if h.throttle == nil {
		return true
	}
	return h.throttle.ShouldFire(ctx, session)
}

// stopFn returns a cleanup closure that clears the typing indicator on a
// detached, short-deadline context (so it still runs if the send ctx expired).
func (h *Humanizer) stopFn(session, chatID string) func() {
	return func() {
		dctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = h.ctl.StopTyping(dctx, whatsapp.TypingInput{SessionName: session, ChatID: chatID})
	}
}

type burst struct {
	typeFor    time.Duration
	pauseAfter time.Duration
}

// typingDuration models a human typing the message: time proportional to length
// at a jittered speed, plus small additive jitter, clamped to [Min, Max].
func typingDuration(cfg Config, text string, hasMedia bool) time.Duration {
	runes := utf8.RuneCountInString(text)
	cps := cfg.CharsPerSecond
	if cps <= 0 {
		cps = 4.0
	}
	speed := cps * (1 + (mathrand.Float64()*2-1)*cfg.SpeedJitterPct)
	if speed < 0.1 {
		speed = 0.1
	}
	raw := time.Duration(float64(runes) / speed * float64(time.Second))
	raw += time.Duration(mathrand.Float64() * 0.4 * float64(time.Second))
	if hasMedia && runes < 20 && raw < cfg.MediaTyping {
		raw = cfg.MediaTyping
	}
	if raw < cfg.MinTyping {
		raw = cfg.MinTyping
	}
	if cfg.MaxTyping > 0 && raw > cfg.MaxTyping {
		raw = cfg.MaxTyping
	}
	return raw
}

// planBursts splits a long message's typing time into 2–3 segments with brief
// think-pauses between them; short messages type in one burst.
func planBursts(cfg Config, text string, total time.Duration) []burst {
	runes := utf8.RuneCountInString(text)
	if cfg.BurstThreshold <= 0 || runes <= cfg.BurstThreshold {
		return []burst{{typeFor: total}}
	}
	n := 2
	if runes > 400 {
		n = 3
	}
	each := total / time.Duration(n)
	segs := make([]burst, n)
	for i := 0; i < n; i++ {
		segs[i].typeFor = each
		if i < n-1 {
			frac := 0.08 + mathrand.Float64()*0.10 // 0.08..0.18 of total
			segs[i].pauseAfter = time.Duration(float64(total) * frac)
		}
	}
	return segs
}

// chance reports true with probability p (goroutine-safe rand).
func chance(p float64) bool {
	if p <= 0 {
		return false
	}
	if p >= 1 {
		return true
	}
	return mathrand.Float64() < p
}

// detach returns a context that is decoupled from ctx's cancellation (so a
// fire-and-forget call isn't killed when the send ctx ends) but bounded by a
// short deadline.
func detach(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
}

// sleepCtx sleeps for d, returning false if ctx is canceled first.
func sleepCtx(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return true
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-ctx.Done():
		return false
	}
}
