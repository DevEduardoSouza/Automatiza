package config

import (
	"automatiza-go/internal/lib/humanize"
	"automatiza-go/internal/lib/pacing"
	"automatiza-go/internal/lib/reputation"
)

// AntiBanRuntime holds the process-wide anti-ban infrastructure (presence
// choreographer + Redis-backed reputation and pacing limiter). It is built once
// in main() and read by the dispatch services and the send worker via
// GetAntiBanRuntime(), so those don't each thread the deps through their (large)
// constructors. Everything inside is nil-safe / fail-open.
type AntiBanRuntime struct {
	// Humanizer may be nil when WhatsApp is not configured; its methods are
	// nil-receiver safe (BeforeSend on a nil *Humanizer is a no-op).
	Humanizer *humanize.Humanizer
	// Reputation and Pacing are always non-nil (nil-backed no-op when Redis absent).
	Reputation *reputation.Store
	Pacing     *pacing.Limiter
}

var antiBanRuntime *AntiBanRuntime

// SetAntiBanRuntime installs the runtime. Call once at startup.
func SetAntiBanRuntime(r *AntiBanRuntime) {
	antiBanRuntime = r
}

// GetAntiBanRuntime returns the runtime, lazily falling back to a fully nil-safe
// no-op runtime if SetAntiBanRuntime was never called (e.g. in tests).
func GetAntiBanRuntime() *AntiBanRuntime {
	if antiBanRuntime == nil {
		antiBanRuntime = &AntiBanRuntime{
			Humanizer:  nil,
			Reputation: reputation.NewStore(nil, 0, 0, nil),
			Pacing:     pacing.NewLimiter(nil, nil),
		}
	}
	return antiBanRuntime
}
