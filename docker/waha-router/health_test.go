package main

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

// testConfig is a deterministic config independent of the environment.
func testConfig() Config {
	return Config{
		WMemory: 3, WCPU: 1.5, WCapacity: 2, WErrors: 1.5, WChurn: 1, WLatency: 1, WActivity: 1,
		SaturationMemPct: 90, SaturationUtilPct: 95,
		LatencyBudgetMs: 2000, ActivityRefRPS: 20,
		MetricTTL: 45 * time.Second, UnknownLoad: 0.5,
		RefreshInterval: 30 * time.Second, PollTimeout: 8 * time.Second,
	}
}

func scoreAll(c Config, shards []ShardHealth) []ShardHealth {
	for i := range shards {
		shards[i].computeScore(c)
	}
	return shards
}

// setupFleet installs a deterministic in-memory fleet for tests that exercise
// snapshot()/pickForCreate()/handleIngest() (which read package globals).
func setupFleet(names ...string) {
	upstreams = nil
	byName = map[string]*Upstream{}
	for _, n := range names {
		u, _ := url.Parse("http://" + n)
		up := &Upstream{Name: n, URL: u}
		upstreams = append(upstreams, up)
		byName[n] = up
	}
	capN = 100
	initHealth()
	cfg = testConfig() // override env-derived config for determinism
	statsMu.Lock()
	shardStats = map[string]ShardHealth{}
	statsMu.Unlock()
}

// The decisive test: a memory-stressed shard with FEW sessions is skipped —
// exactly the failure the old count-only router could not see.
func TestMemorySaturationBeatsRawCount(t *testing.T) {
	c := testConfig()
	// a: only 10 sessions but 92% memory -> saturated, must be excluded.
	a := ShardHealth{Name: "a", Available: true, Cap: 100, Total: 10, Connected: 10, MemPct: 92, CPUPct: 30}
	b := ShardHealth{Name: "b", Available: true, Cap: 100, Total: 70, Connected: 70, MemPct: 50, CPUPct: 30}
	a.computeScore(c)
	b.computeScore(c)
	if !a.Saturated {
		t.Fatalf("shard a at 92%% memory must be saturated")
	}
	best, emerg := selectBest([]ShardHealth{a, b})
	if best == nil || best.Name != "b" || emerg {
		t.Fatalf("expected b (a saturated by memory despite fewer sessions), got %+v emergency=%v", best, emerg)
	}
}

// With only count differing (no mem/cpu data), the score reduces to
// least-loaded — preserving sane behaviour when memory is unknown.
func TestScoreReducesToLeastLoadedWhenOnlyCountDiffers(t *testing.T) {
	c := testConfig()
	a := ShardHealth{Name: "a", Available: true, Cap: 100, Total: 80, Connected: 80, MemPct: -1, CPUPct: -1}
	b := ShardHealth{Name: "b", Available: true, Cap: 100, Total: 20, Connected: 20, MemPct: -1, CPUPct: -1}
	a.computeScore(c)
	b.computeScore(c)
	best, emerg := selectBest([]ShardHealth{a, b})
	if best == nil || best.Name != "b" || emerg {
		t.Fatalf("expected b (less loaded), got %+v", best)
	}
}

func TestHighLatencyLowersScore(t *testing.T) {
	c := testConfig()
	fast := ShardHealth{Name: "fast", Available: true, Cap: 100, Total: 50, Connected: 50, MemPct: 50, CPUPct: 30, LatencyMs: 50}
	slow := ShardHealth{Name: "slow", Available: true, Cap: 100, Total: 50, Connected: 50, MemPct: 50, CPUPct: 30, LatencyMs: 1800}
	fast.computeScore(c)
	slow.computeScore(c)
	if fast.Score <= slow.Score {
		t.Fatalf("fast shard must score higher: fast=%.3f slow=%.3f", fast.Score, slow.Score)
	}
	best, _ := selectBest([]ShardHealth{slow, fast})
	if best.Name != "fast" {
		t.Fatalf("expected fast, got %s", best.Name)
	}
}

func TestErrorsAndChurnLowerScore(t *testing.T) {
	c := testConfig()
	healthy := ShardHealth{Name: "h", Available: true, Cap: 100, Total: 50, Connected: 50, MemPct: 50, CPUPct: 30}
	bad := ShardHealth{Name: "bad", Available: true, Cap: 100, Total: 50, Connected: 20, Failed: 20, Starting: 10, MemPct: 50, CPUPct: 30}
	healthy.computeScore(c)
	bad.computeScore(c)
	if healthy.Score <= bad.Score {
		t.Fatalf("healthy must score higher than error/churn-heavy shard: %.3f vs %.3f", healthy.Score, bad.Score)
	}
}

func TestUnavailableShardExcluded(t *testing.T) {
	c := testConfig()
	down := ShardHealth{Name: "down", Available: false, Cap: 100, Total: 5}
	upS := ShardHealth{Name: "up", Available: true, Cap: 100, Total: 90, Connected: 90, MemPct: 60, CPUPct: 30}
	down.computeScore(c)
	upS.computeScore(c)
	best, emerg := selectBest([]ShardHealth{down, upS})
	if best == nil || best.Name != "up" || emerg {
		t.Fatalf("expected up (down excluded by failover), got %+v", best)
	}
}

func TestAllSaturatedEmergencyFallback(t *testing.T) {
	c := testConfig()
	a := ShardHealth{Name: "a", Available: true, Cap: 100, Total: 50, Connected: 50, MemPct: 95, CPUPct: 80}
	b := ShardHealth{Name: "b", Available: true, Cap: 100, Total: 50, Connected: 50, MemPct: 91, CPUPct: 80}
	a.computeScore(c)
	b.computeScore(c)
	best, emerg := selectBest([]ShardHealth{a, b})
	if best == nil || !emerg {
		t.Fatalf("all-saturated must fall back to least-loaded with emergency=true, got %+v emergency=%v", best, emerg)
	}
	if best.Name != "b" {
		t.Fatalf("emergency pick must be the less mem-loaded (b), got %s", best.Name)
	}
}

func TestAllAtCapReturnsNil(t *testing.T) {
	c := testConfig()
	a := ShardHealth{Name: "a", Available: true, Cap: 100, Total: 100, Connected: 100, MemPct: 50}
	b := ShardHealth{Name: "b", Available: true, Cap: 100, Total: 100, Connected: 100, MemPct: 50}
	a.computeScore(c)
	b.computeScore(c)
	best, _ := selectBest([]ShardHealth{a, b})
	if best != nil {
		t.Fatalf("all-at-cap must return nil (caller falls back to primary), got %s", best.Name)
	}
}

func TestNShardPicksHealthiest(t *testing.T) {
	c := testConfig()
	shards := scoreAll(c, []ShardHealth{
		{Name: "s1", Available: true, Cap: 100, Total: 90, Connected: 90, MemPct: 85, CPUPct: 60},
		{Name: "s2", Available: true, Cap: 100, Total: 40, Connected: 40, MemPct: 40, CPUPct: 30},
		{Name: "s3", Available: true, Cap: 100, Total: 70, Connected: 70, MemPct: 70, CPUPct: 50},
	})
	best, _ := selectBest(shards)
	if best == nil || best.Name != "s2" {
		t.Fatalf("expected s2 (healthiest of 3), got %+v", best)
	}
}

func TestMetricsStoreFreshness(t *testing.T) {
	s := newMetricsStore()
	now := time.Now()
	s.set("a", 80, 40, now.Add(-100*time.Second))
	if _, fresh := s.get("a", 45*time.Second, now); fresh {
		t.Fatalf("100s-old sample must be stale under a 45s TTL")
	}
	s.set("a", 80, 40, now.Add(-10*time.Second))
	if p, fresh := s.get("a", 45*time.Second, now); !fresh || p.MemPct != 80 {
		t.Fatalf("10s-old sample must be fresh with memPct=80")
	}
	if _, ok := s.get("missing", 45*time.Second, now); ok {
		t.Fatalf("missing shard must return not-ok")
	}
}

func TestSnapshotOverlaysFreshMetricsOnly(t *testing.T) {
	setupFleet("s1", "s2")
	statsMu.Lock()
	shardStats = map[string]ShardHealth{
		"s1": {Name: "s1", Available: true, Total: 10, Connected: 10},
		"s2": {Name: "s2", Available: true, Total: 20, Connected: 20},
	}
	statsMu.Unlock()
	now := time.Now()
	mstore.set("s1", 88, 40, now)                      // fresh
	mstore.set("s2", 30, 20, now.Add(-10*time.Minute)) // stale

	snap := snapshot(now)
	byN := map[string]ShardHealth{}
	for _, s := range snap {
		byN[s.Name] = s
	}
	if !byN["s1"].MetricFresh || byN["s1"].MemPct != 88 {
		t.Fatalf("s1 should carry fresh memPct=88, got %+v", byN["s1"])
	}
	if byN["s2"].MetricFresh || byN["s2"].MemPct != -1 {
		t.Fatalf("s2 stale metric must degrade to unknown (-1), got %+v", byN["s2"])
	}
}

func TestHandleIngestSingleAndArrayIgnoresUnknown(t *testing.T) {
	setupFleet("s1", "s2")
	body := `[{"shard":"s1","memPct":88,"cpuPct":40},{"shard":"ghost","memPct":10}]`
	req := httptest.NewRequest("POST", "/router/shard-metrics", strings.NewReader(body))
	w := httptest.NewRecorder()
	handleIngest(w, req)
	if w.Code != 200 {
		t.Fatalf("ingest status=%d", w.Code)
	}
	if p, fresh := mstore.get("s1", time.Minute, time.Now()); !fresh || p.MemPct != 88 {
		t.Fatalf("s1 must be ingested with memPct=88")
	}
	if _, ok := mstore.get("ghost", time.Minute, time.Now()); ok {
		t.Fatalf("unknown shard must be ignored, never stored")
	}
	// single-object form
	req2 := httptest.NewRequest("POST", "/router/shard-metrics", strings.NewReader(`{"shard":"s2","memPct":55,"cpuPct":22}`))
	w2 := httptest.NewRecorder()
	handleIngest(w2, req2)
	if p, fresh := mstore.get("s2", time.Minute, time.Now()); !fresh || p.MemPct != 55 {
		t.Fatalf("single-object ingest must store s2 memPct=55")
	}
}

func TestEnvHelpers(t *testing.T) {
	t.Setenv("ROUTER_TEST_F", "2.5")
	if envFloat("ROUTER_TEST_F", 1) != 2.5 {
		t.Fatal("envFloat should read env")
	}
	if envFloat("ROUTER_TEST_MISSING", 1.25) != 1.25 {
		t.Fatal("envFloat should default")
	}
	t.Setenv("ROUTER_TEST_I", "7")
	if envInt("ROUTER_TEST_I", 1) != 7 {
		t.Fatal("envInt should read env")
	}
	if clamp01(-1) != 0 || clamp01(2) != 1 || clamp01(0.5) != 0.5 {
		t.Fatal("clamp01 bounds wrong")
	}
}

// Degenerate config (all weights 0) must not divide by zero.
func TestZeroWeightsFallBackToCapacity(t *testing.T) {
	c := Config{SaturationMemPct: 90, SaturationUtilPct: 95}
	a := ShardHealth{Name: "a", Available: true, Cap: 100, Total: 80, MemPct: -1, CPUPct: -1}
	a.computeScore(c)
	if a.Score < 0 || a.Score > 1 {
		t.Fatalf("score must stay in [0,1] even with zero weights, got %.3f", a.Score)
	}
}

// ETAPA 13: concurrency — the shared routing state must be race-free under
// simultaneous request accounting, metric pushes, refresh-style writes and
// snapshot reads. Run with -race.
func TestConcurrentAccessNoRace(t *testing.T) {
	setupFleet("s1", "s2")
	var wg sync.WaitGroup
	stop := make(chan struct{})

	// Writer: simulates refresh() rewriting stats + consuming request rates.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			_ = requestRates(time.Now())
			statsMu.Lock()
			shardStats = map[string]ShardHealth{
				"s1": {Name: "s1", Available: true, Cap: 100, Total: 10, Connected: 10},
				"s2": {Name: "s2", Available: true, Cap: 100, Total: 20, Connected: 20},
			}
			statsMu.Unlock()
		}
	}()

	// Pushers: request accounting + metric ingestion.
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				countRequest("s1")
				countRequest("s2")
				mstore.set("s1", 50, 30, time.Now())
			}
		}()
	}

	// Readers: the hot path (snapshot + selection).
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				_, _ = selectBest(snapshot(time.Now()))
			}
		}()
	}

	time.Sleep(200 * time.Millisecond)
	close(stop)
	wg.Wait()
}
