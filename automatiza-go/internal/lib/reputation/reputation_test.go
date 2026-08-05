package reputation

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// newTestStore sobe um miniredis e devolve um Store apontado para ele, junto do
// próprio miniredis (para inspecionar TTL / fechar) e uma função de limpeza.
func newTestStore(t *testing.T, threshold int, ttl time.Duration) (*Store, *miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return NewStore(rdb, threshold, ttl, zap.NewNop()), mr, rdb
}

// Supressão DESLIGADA (05/08/2026): Penalize continua acumulando a pontuação no
// Redis — o sinal segue sendo coletado — mas IsSuppressed nunca bloqueia envio,
// nem acima do limiar. Clear continua zerando a chave.
func TestPenalizeStillScoresButNeverSuppresses(t *testing.T) {
	ctx := context.Background()
	s, mr, _ := newTestStore(t, 3, time.Hour)
	const session, group = "sess-a", "group-1"

	s.Penalize(ctx, session, group, 1)
	s.Penalize(ctx, session, group, 1)
	if s.IsSuppressed(ctx, session, group) {
		t.Fatalf("suppression is disabled: score 2 must not suppress")
	}

	// Cruza o limiar (2 + 1 = 3 >= 3) e AINDA ASSIM não suprime.
	s.Penalize(ctx, session, group, 1)
	if got, err := mr.Get(key(session, group)); err != nil || got != "3" {
		t.Fatalf("expected the score to still be recorded as 3, got %q (err=%v)", got, err)
	}
	if s.IsSuppressed(ctx, session, group) {
		t.Fatalf("suppression is disabled: score 3 >= threshold 3 must NOT suppress")
	}

	// Clear segue zerando a pontuação acumulada.
	s.Clear(ctx, session, group)
	if _, err := mr.Get(key(session, group)); err == nil {
		t.Fatalf("Clear must delete the score key")
	}
}

func TestWeightedPenalizeRemovalNeverSuppresses(t *testing.T) {
	ctx := context.Background()
	s, mr, _ := newTestStore(t, 5, time.Hour)
	const session, group = "sess-b", "group-2"

	// Peso de remoção (bot expulso) = 3; duas remoções somam 6 >= 5.
	s.Penalize(ctx, session, group, 3)
	s.Penalize(ctx, session, group, 3)
	if got, err := mr.Get(key(session, group)); err != nil || got != "6" {
		t.Fatalf("expected accumulated score 6, got %q (err=%v)", got, err)
	}
	if s.IsSuppressed(ctx, session, group) {
		t.Fatalf("suppression is disabled: score 6 >= threshold 5 must NOT suppress")
	}
}

// Com a supressão desligada, nem o modo strict bloqueia envio num erro de Redis:
// o fail-closed do ANTIBAN_STRICT deixou de ter efeito.
func TestStrictNoLongerFailsClosed(t *testing.T) {
	ctx := context.Background()
	s, mr, _ := newTestStore(t, 5, time.Hour)
	s.strict = true
	const session, group = "sess-x", "group-x"

	if s.IsSuppressed(ctx, session, group) {
		t.Fatalf("healthy Redis with a clean group must not be suppressed")
	}

	mr.Close() // Redis indisponível
	if s.IsSuppressed(ctx, session, group) {
		t.Fatalf("suppression is disabled: strict mode must NOT suppress anymore")
	}
}

// R3: com o kill-switch desligado (strict=false), mantém o fail-open histórico.
func TestIsSuppressedFailOpenWhenNotStrict(t *testing.T) {
	ctx := context.Background()
	s, mr, _ := newTestStore(t, 5, time.Hour)
	s.strict = false
	const session, group = "sess-y", "group-y"

	mr.Close() // Redis indisponível
	if s.IsSuppressed(ctx, session, group) {
		t.Fatalf("non-strict mode must NOT suppress on a Redis error (fail-open)")
	}
}

func TestNilStoreIsNoOp(t *testing.T) {
	ctx := context.Background()
	var s *Store = NewStore(nil, 5, time.Hour, zap.NewNop())

	// Nenhuma dessas chamadas pode entrar em pânico.
	s.Penalize(ctx, "sess", "group", 3)
	s.Clear(ctx, "sess", "group")
	if s.IsSuppressed(ctx, "sess", "group") {
		t.Fatalf("nil-rdb store must never suppress")
	}

	// Um ponteiro *Store nil também é seguro.
	var np *Store
	np.Penalize(ctx, "sess", "group", 3)
	np.Clear(ctx, "sess", "group")
	if np.IsSuppressed(ctx, "sess", "group") {
		t.Fatalf("nil *Store must never suppress")
	}
}

func TestEmptyArgsAreNoOp(t *testing.T) {
	ctx := context.Background()
	s, mr, _ := newTestStore(t, 1, time.Hour)

	s.Penalize(ctx, "", "group", 5)
	s.Penalize(ctx, "sess", "", 5)
	if got := len(mr.Keys()); got != 0 {
		t.Fatalf("empty session/group must not write any key, got %d keys", got)
	}
	if s.IsSuppressed(ctx, "", "group") || s.IsSuppressed(ctx, "sess", "") {
		t.Fatalf("empty args must never be suppressed")
	}
}

func TestNeverSuppressesWithRedisDown(t *testing.T) {
	ctx := context.Background()
	s, mr, _ := newTestStore(t, 1, time.Hour)
	s.strict = false
	const session, group = "sess-c", "group-3"

	// Mesmo bem acima do limiar, com o Redis vivo: não suprime.
	s.Penalize(ctx, session, group, 5)
	if s.IsSuppressed(ctx, session, group) {
		t.Fatalf("suppression is disabled: must not suppress with Redis up")
	}

	// Redis cai: segue sem suprimir e sem entrar em pânico.
	mr.Close()
	if s.IsSuppressed(ctx, session, group) {
		t.Fatalf("suppression is disabled: must not suppress with Redis down")
	}
	// Penalize/Clear com Redis morto também não podem entrar em pânico.
	s.Penalize(ctx, session, group, 3)
	s.Clear(ctx, session, group)
}

func TestTTLApplied(t *testing.T) {
	ctx := context.Background()
	const ttl = 30 * time.Minute
	s, mr, _ := newTestStore(t, 5, ttl)
	const session, group = "sess-d", "group-4"

	s.Penalize(ctx, session, group, 1)

	k := key(session, group)
	got := mr.TTL(k)
	if got <= 0 {
		t.Fatalf("expected a positive TTL after Penalize, got %v", got)
	}
	if got > ttl {
		t.Fatalf("TTL %v should not exceed configured ttl %v", got, ttl)
	}
}

func TestDefaultsApplied(t *testing.T) {
	// threshold<=0 => 5, ttl<=0 => 24h.
	s := NewStore(nil, 0, 0, nil)
	if s.threshold != 5 {
		t.Fatalf("expected default threshold 5, got %d", s.threshold)
	}
	if s.ttl != 24*time.Hour {
		t.Fatalf("expected default ttl 24h, got %v", s.ttl)
	}
	if s.logger == nil {
		t.Fatalf("expected a non-nil (no-op) logger by default")
	}
}
