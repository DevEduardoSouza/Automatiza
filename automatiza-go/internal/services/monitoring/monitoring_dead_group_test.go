package monitoring

import (
	"context"
	"testing"

	"automatiza-go/internal/lib/cache"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func newMiniredisCache(t *testing.T) *cache.Cache {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return cache.NewCache(rdb, "test", false)
}

func TestMonitoringDeadGroup_MarkAndCheck(t *testing.T) {
	s := &monitoringService{cache: newMiniredisCache(t), logger: zap.NewNop()}
	ctx := context.Background()

	if s.isDeadGroup(ctx, "sess-1", "grp-A@g.us") {
		t.Fatal("grupo nao deveria estar morto antes de marcar")
	}

	s.markDeadGroup(ctx, "sess-1", "grp-A@g.us")
	if !s.isDeadGroup(ctx, "sess-1", "grp-A@g.us") {
		t.Fatal("grupo deveria estar morto apos marcar")
	}

	// Sessao/grupo diferentes nao sao afetados.
	if s.isDeadGroup(ctx, "sess-1", "grp-B@g.us") {
		t.Fatal("grupo diferente nao deveria estar morto")
	}
	if s.isDeadGroup(ctx, "sess-2", "grp-A@g.us") {
		t.Fatal("sessao diferente nao deveria estar morta")
	}
}

func TestMonitoringDeadGroup_FailOpen(t *testing.T) {
	// Cache nil e chaves vazias: nunca reporta morto, nao panica ao marcar.
	s := &monitoringService{cache: nil, logger: zap.NewNop()}
	ctx := context.Background()
	s.markDeadGroup(ctx, "sess-1", "grp-A@g.us") // no-op, sem panic
	if s.isDeadGroup(ctx, "sess-1", "grp-A@g.us") {
		t.Fatal("cache nil deve ser fail-open (false)")
	}

	s2 := &monitoringService{cache: newMiniredisCache(t), logger: zap.NewNop()}
	if s2.isDeadGroup(ctx, "", "grp") || s2.isDeadGroup(ctx, "sess", "") {
		t.Fatal("chaves vazias devem retornar false")
	}
}
