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

func TestPenalizeCrossesThresholdAndClear(t *testing.T) {
	ctx := context.Background()
	s, _, _ := newTestStore(t, 3, time.Hour)
	const session, group = "sess-a", "group-1"

	// Abaixo do limiar: não suprimido.
	s.Penalize(ctx, session, group, 1)
	s.Penalize(ctx, session, group, 1)
	if s.IsSuppressed(ctx, session, group) {
		t.Fatalf("score 2 < threshold 3 should not be suppressed")
	}

	// Cruza o limiar (2 + 1 = 3 >= 3): suprimido.
	s.Penalize(ctx, session, group, 1)
	if !s.IsSuppressed(ctx, session, group) {
		t.Fatalf("score 3 >= threshold 3 should be suppressed")
	}

	// Clear devolve o grupo à rotação.
	s.Clear(ctx, session, group)
	if s.IsSuppressed(ctx, session, group) {
		t.Fatalf("after Clear the group should not be suppressed")
	}
}

func TestWeightedPenalizeRemoval(t *testing.T) {
	ctx := context.Background()
	s, _, _ := newTestStore(t, 5, time.Hour)
	const session, group = "sess-b", "group-2"

	// Peso de remoção (bot expulso) = 3. Uma remoção sozinha não cruza 5.
	s.Penalize(ctx, session, group, 3)
	if s.IsSuppressed(ctx, session, group) {
		t.Fatalf("score 3 < threshold 5 should not be suppressed")
	}

	// Duas remoções (3 + 3 = 6 >= 5) cruzam o limiar.
	s.Penalize(ctx, session, group, 3)
	if !s.IsSuppressed(ctx, session, group) {
		t.Fatalf("score 6 >= threshold 5 should be suppressed")
	}
}

// R3: em modo strict, um ERRO de Redis suprime o envio (fail-closed).
func TestIsSuppressedStrictFailClosedOnRedisError(t *testing.T) {
	ctx := context.Background()
	s, mr, _ := newTestStore(t, 5, time.Hour)
	s.strict = true // fixa o strict (o default vem do env)
	const session, group = "sess-x", "group-x"

	// Redis saudável + grupo limpo: chave ausente devolve "0" -> não suprime.
	if s.IsSuppressed(ctx, session, group) {
		t.Fatalf("healthy Redis with a clean group must not be suppressed")
	}

	// Redis indisponível (miniredis fechado): a chamada erra.
	mr.Close()
	if !s.IsSuppressed(ctx, session, group) {
		t.Fatalf("strict mode must SUPPRESS on a Redis error (fail-closed)")
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

func TestFailOpenWhenRedisDown(t *testing.T) {
	ctx := context.Background()
	s, mr, _ := newTestStore(t, 1, time.Hour)
	s.strict = false // fail-open agora é o caminho do kill-switch (ANTIBAN_STRICT=false)
	const session, group = "sess-c", "group-3"

	// Suprimido enquanto o Redis está vivo.
	s.Penalize(ctx, session, group, 5)
	if !s.IsSuppressed(ctx, session, group) {
		t.Fatalf("expected suppressed before Redis goes down")
	}

	// Redis cai + strict off: fail-open — não suprime e não entra em pânico.
	mr.Close()
	if s.IsSuppressed(ctx, session, group) {
		t.Fatalf("with Redis down and strict off, IsSuppressed must fail open to false")
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
