package workers

import (
	"sync"
	"time"
)

// deadGroupTTL é por quanto tempo um par (sessão, grupo) fica marcado como
// "morto" depois que o WAHA recusa o envio com "you're not participating in
// that group". O TTL é curto: se o bot for readicionado ao grupo, o ciclo
// seguinte (apos o TTL) volta a tentar normalmente — sem intervenção e sem
// mutar a config de grupos da automação do usuário.
const deadGroupTTL = 30 * time.Minute

// deadGroupCache guarda, em memória do processo do worker, os grupos dos quais
// a sessão não participa mais. Evita martelar o WAHA (que responde 500 e
// dispara o fallback texto + retries do asynq) a cada produto/ciclo para um
// grupo do qual o bot foi removido.
//
// Escopo in-process é suficiente: roda um único container worker. Se um dia
// houver réplicas, cada uma reaprende o grupo morto na primeira falha — no pior
// caso uma tentativa extra por réplica, não um problema de correção.
type deadGroupCache struct {
	mu      sync.Mutex
	entries map[string]time.Time // key -> expiry
}

func newDeadGroupCache() *deadGroupCache {
	return &deadGroupCache{entries: make(map[string]time.Time)}
}

func deadGroupKey(session, groupID string) string {
	return session + "|" + groupID
}

// markDead registra o par (session, groupID) como morto por deadGroupTTL.
func (c *deadGroupCache) markDead(session, groupID string, now time.Time) {
	if c == nil || session == "" || groupID == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[deadGroupKey(session, groupID)] = now.Add(deadGroupTTL)
}

// isDead informa se o par está marcado como morto e ainda dentro do TTL.
// Entradas expiradas são removidas de forma preguiçosa no acesso.
func (c *deadGroupCache) isDead(session, groupID string, now time.Time) bool {
	if c == nil || session == "" || groupID == "" {
		return false
	}
	key := deadGroupKey(session, groupID)
	c.mu.Lock()
	defer c.mu.Unlock()
	expiry, ok := c.entries[key]
	if !ok {
		return false
	}
	if now.After(expiry) {
		delete(c.entries, key)
		return false
	}
	return true
}
