package mercadolivre

import (
	"sync"
	"time"
)

// circuitBreaker é a cópia do breaker da Shopee (shopee/circuit_breaker.go):
// contador de falhas consecutivas que abre por `backoffTime` e half-abre
// depois. Nil-safe de propósito — o breaker é colaborador opcional, então um
// serviço construído sem ele (testes) se comporta exatamente como antes.
//
// O ML usa DOIS breakers, não um, porque os dois modos de falha pedem tempos
// muito diferentes:
//
//   - infra (rede/timeout/5xx): 5 falhas, 30s — igual à Shopee.
//   - bloqueio (ErrMLBlocked): 3 falhas, 15min. Sondar um WAF a cada 30s piora
//     o bloqueio; o backoff curto da Shopee está calibrado para outra coisa.
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

// canRequest reports whether a request is allowed. Nil breaker always allows.
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

// recordFailure counts a failure and stamps the time. No-op on nil.
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
