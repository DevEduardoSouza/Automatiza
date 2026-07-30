package aliexpress

import (
	"sync"
	"time"
)

// breaker é um disjuntor mínimo: N falhas consecutivas abrem o circuito por
// uma janela, e a primeira tentativa depois da janela testa o serviço.
//
// ponytail: é uma cópia enxuta do circuitBreaker de internal/services/shopee
// (circuit_breaker.go). Duplicar ~40 linhas é deliberado no lugar de extrair
// para internal/lib/breaker AGORA: o tipo lá é privado do pacote shopee e
// mexer nele obrigaria a renomear 6 call sites no caminho quente de envio da
// Shopee, que serve 900+ contas, por conta de uma loja que ainda tem zero
// credencial cadastrada. Caminho de upgrade: quando um terceiro consumidor
// aparecer, extrair para internal/lib/breaker e trocar os três por um alias de
// tipo — a mecânica já é idêntica.
type breaker struct {
	mu           sync.Mutex
	failures     int
	maxFailures  int
	openUntil    time.Time
	openDuration time.Duration
}

func newBreaker(maxFailures int, openDuration time.Duration) *breaker {
	return &breaker{maxFailures: maxFailures, openDuration: openDuration}
}

// allow reporta se uma chamada pode sair agora.
func (b *breaker) allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return time.Now().After(b.openUntil)
}

func (b *breaker) recordFailure() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures++
	if b.failures >= b.maxFailures {
		b.openUntil = time.Now().Add(b.openDuration)
		b.failures = 0
	}
}

func (b *breaker) recordSuccess() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures = 0
}
