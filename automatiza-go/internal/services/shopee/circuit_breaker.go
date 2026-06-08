package shopee

import (
	"errors"
	"sync"
	"time"
)

// ErrShopeeUnavailable is returned when the circuit breaker is open — the
// Shopee Affiliate API has failed repeatedly and we short-circuit further
// calls instead of hanging on the (likely dead) endpoint. Callers treat this
// as a transient external outage: search returns no offers, list/offer
// enrichment falls back to the raw URL. Crucially, returning instantly keeps
// the serial automation scheduler from burning its shared context budget on a
// dead provider, so Mercado Livre (and every other) automation keeps flowing.
var ErrShopeeUnavailable = errors.New("shopee API temporarily unavailable (circuit breaker open)")

// circuitBreaker implements a simple consecutive-failure circuit breaker,
// mirroring the per-session breaker in internal/adapters/whatsapp. After
// `threshold` consecutive infra failures (network/timeout/5xx) it opens for
// `backoffTime`; the next request after the backoff half-opens (counter reset)
// and probes the API, closing again on success. A single global breaker is
// correct here because a Shopee host outage is global, not per-user.
type circuitBreaker struct {
	mu          sync.Mutex
	failures    int
	lastFailure time.Time
	threshold   int
	backoffTime time.Duration
}

func newCircuitBreaker(threshold int, backoffTime time.Duration) *circuitBreaker {
	return &circuitBreaker{
		threshold:   threshold,
		backoffTime: backoffTime,
	}
}

// canRequest reports whether a request is allowed. When the breaker is open it
// blocks until backoffTime elapses, then resets to half-open (probe allowed).
// A nil breaker always allows — the breaker is an optional collaborator, so a
// service built without one (e.g. in tests) behaves exactly as before.
func (cb *circuitBreaker) canRequest() bool {
	if cb == nil {
		return true
	}
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.failures >= cb.threshold {
		if time.Since(cb.lastFailure) < cb.backoffTime {
			return false // open
		}
		cb.failures = 0 // backoff elapsed → half-open
	}
	return true
}

// recordFailure counts an infra failure and stamps the time. No-op on nil.
func (cb *circuitBreaker) recordFailure() {
	if cb == nil {
		return
	}
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures++
	cb.lastFailure = time.Now()
}

// recordSuccess closes the breaker. No-op on nil.
func (cb *circuitBreaker) recordSuccess() {
	if cb == nil {
		return
	}
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures = 0
}
