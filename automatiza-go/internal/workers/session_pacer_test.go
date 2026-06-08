package workers

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func newTestRedis(t *testing.T) *redis.Client {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	return redis.NewClient(&redis.Options{Addr: mr.Addr()})
}

func TestSessionPacer_Disabled(t *testing.T) {
	if newSessionPacer(nil, time.Second, zap.NewNop()) != nil {
		t.Fatal("sem redis o pacer deve ser nil (desligado)")
	}
	rdb := newTestRedis(t)
	if newSessionPacer(rdb, 0, zap.NewNop()) != nil {
		t.Fatal("spacing<=0 deve desligar o pacer")
	}

	var p *sessionPacer // nil
	if d := p.reserve(context.Background(), "s1"); d != 0 {
		t.Fatalf("pacer nil deve devolver 0, veio %v", d)
	}
}

func TestSessionPacer_SpacesConsecutiveSends(t *testing.T) {
	rdb := newTestRedis(t)
	p := newSessionPacer(rdb, time.Second, zap.NewNop())
	ctx := context.Background()

	d1 := p.reserve(ctx, "sessao-A")
	d2 := p.reserve(ctx, "sessao-A")
	d3 := p.reserve(ctx, "sessao-A")

	if d1 != 0 {
		t.Fatalf("1o envio nao deve atrasar, veio %v", d1)
	}
	// 2o e 3o sao empurrados em ~1 e ~2 spacings (margem para o relogio andar).
	if d2 < 800*time.Millisecond || d2 > time.Second {
		t.Fatalf("2o envio deveria ~1s, veio %v", d2)
	}
	if d3 < 1800*time.Millisecond || d3 > 2*time.Second {
		t.Fatalf("3o envio deveria ~2s, veio %v", d3)
	}
}

func TestSessionPacer_IndependentPerSession(t *testing.T) {
	rdb := newTestRedis(t)
	p := newSessionPacer(rdb, time.Second, zap.NewNop())
	ctx := context.Background()

	_ = p.reserve(ctx, "sessao-A")
	_ = p.reserve(ctx, "sessao-A")

	// Outra sessao começa do zero — não herda o slot da sessao-A.
	if d := p.reserve(ctx, "sessao-B"); d != 0 {
		t.Fatalf("outra sessao deve começar sem atraso, veio %v", d)
	}

	// Sessao vazia => 0.
	if d := p.reserve(ctx, ""); d != 0 {
		t.Fatalf("sessao vazia deve devolver 0, veio %v", d)
	}
}

func TestSessionPaceMsFromEnv(t *testing.T) {
	cases := map[string]int{"": 0, "0": 0, "-5": 0, "abc": 0, "1200": 1200}
	for in, want := range cases {
		t.Setenv("SESSION_PACE_MS", in)
		if got := sessionPaceMsFromEnv(); got != want {
			t.Fatalf("SESSION_PACE_MS=%q => %d, want %d", in, got, want)
		}
	}
}
