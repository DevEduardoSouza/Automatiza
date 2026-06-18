package offer_generate //nolint:revive // package name already has underscore

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"automatiza-go/internal/lib/cache"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func claimOnceTestService(t *testing.T) *offerGenerateService {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return &offerGenerateService{
		logger: zap.NewNop(),
		cache:  cache.NewCache(rdb, "test", false),
	}
}

func TestClaimOnce_SecondCallSkips(t *testing.T) {
	svc := claimOnceTestService(t)
	ctx := context.Background()
	if !svc.claimOnce(ctx, "k", time.Minute) {
		t.Fatal("first claim must win")
	}
	if svc.claimOnce(ctx, "k", time.Minute) {
		t.Fatal("second claim on the same key must lose")
	}
	if !svc.claimOnce(ctx, "other", time.Minute) {
		t.Fatal("a different key must win")
	}
}

// The whole point of the fix: N concurrent claims for the same key (the same
// message delivered by N chips at once) yield EXACTLY ONE winner — so the offer
// is processed/fanned out once, not N times.
func TestClaimOnce_ConcurrentSingleWinner(t *testing.T) {
	svc := claimOnceTestService(t)
	ctx := context.Background()

	const n = 50
	var wins int32
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start // line everyone up to maximize the race
			if svc.claimOnce(ctx, "burst", time.Minute) {
				atomic.AddInt32(&wins, 1)
			}
		}()
	}
	close(start)
	wg.Wait()

	if got := atomic.LoadInt32(&wins); got != 1 {
		t.Fatalf("exactly one concurrent claim must win, got %d", got)
	}
}

// Fail-open: with no cache configured, every call processes (never drops a real
// message); duplication is then backstopped by claimProductSlot.
func TestClaimOnce_NoCacheFailOpen(t *testing.T) {
	svc := &offerGenerateService{logger: zap.NewNop()} // cache nil
	if !svc.claimOnce(context.Background(), "k", time.Minute) {
		t.Fatal("with no cache, claim must fail-open to true")
	}
}
