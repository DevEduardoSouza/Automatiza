// metrics.go — activity accounting, the health snapshot, and the router's
// observability + metric-ingest HTTP endpoints (ETAPA 8, 9).
//
//	GET  /router/metrics        -> JSON array of per-shard ShardHealth
//	POST /router/shard-metrics  -> ingest pushed mem/CPU from the sentinel
//
// The router measures activity itself: it proxies every WhatsApp API call, so
// counting requests per upstream yields a real per-shard throughput signal with
// no external dependency.
package main

import (
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

var (
	cfg    Config        // loaded once in main()
	mstore *metricsStore // pushed mem/CPU, keyed by shard name

	// shardStats is the latest HTTP-measured snapshot per shard, written by
	// refresh() and read by snapshot(). Guarded by statsMu.
	statsMu    sync.RWMutex
	shardStats = map[string]ShardHealth{}

	// Per-shard cumulative proxied-request counters (atomic), plus the previous
	// reading, so refresh() can derive a request-per-second rate.
	reqTotal  = map[string]*int64{}
	reqPrev   = map[string]int64{}
	reqPrevAt time.Time
)

// initHealth wires the health subsystem for the configured upstreams. Must be
// called after loadUpstreams() so the upstream list is populated.
func initHealth() {
	cfg = loadConfig()
	mstore = newMetricsStore()
	for _, up := range upstreams {
		var z int64
		reqTotal[up.Name] = &z
	}
	reqPrevAt = time.Now()
}

// countRequest bumps a shard's proxied-request counter. Unknown names are
// ignored (defensive; all upstream names are pre-registered in initHealth).
func countRequest(name string) {
	if c := reqTotal[name]; c != nil {
		atomic.AddInt64(c, 1)
	}
}

// serveVia proxies to an upstream while accounting the request for the activity
// signal. Every request that reaches an upstream goes through here.
func serveVia(up *Upstream, w http.ResponseWriter, r *http.Request) {
	countRequest(up.Name)
	up.Proxy.ServeHTTP(w, r)
}

// requestRates returns req/s per shard since the previous call, and resets the
// window. Called once per refresh under the refresh goroutine (single writer).
func requestRates(now time.Time) map[string]float64 {
	elapsed := now.Sub(reqPrevAt).Seconds()
	if elapsed <= 0 {
		elapsed = 1
	}
	out := map[string]float64{}
	for name, c := range reqTotal {
		cur := atomic.LoadInt64(c)
		prev := reqPrev[name]
		d := cur - prev
		if d < 0 {
			d = 0
		}
		out[name] = float64(d) / elapsed
		reqPrev[name] = cur
	}
	reqPrevAt = now
	return out
}

// snapshot assembles the scored per-shard view: the HTTP-measured stats overlaid
// with fresh pushed mem/CPU, then scored. Order follows the configured upstream
// list so output is stable. Used by both pickForCreate and /router/metrics.
func snapshot(now time.Time) []ShardHealth {
	statsMu.RLock()
	base := make(map[string]ShardHealth, len(shardStats))
	for k, v := range shardStats {
		base[k] = v
	}
	statsMu.RUnlock()

	out := make([]ShardHealth, 0, len(upstreams))
	for _, up := range upstreams {
		h := base[up.Name]
		h.Name = up.Name
		h.Cap = capN
		if p, fresh := mstore.get(up.Name, cfg.MetricTTL, now); fresh {
			h.MemPct, h.CPUPct, h.MetricFresh = p.MemPct, p.CPUPct, true
		} else {
			h.MemPct, h.CPUPct, h.MetricFresh = -1, -1, false
		}
		h.computeScore(cfg)
		out = append(out, h)
	}
	return out
}

// handleMetrics serves GET /router/metrics — the full per-shard health view for
// dashboards, the sentinel's alerting, and debugging.
func handleMetrics(w http.ResponseWriter, r *http.Request) {
	snap := snapshot(time.Now())
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"generatedAt": time.Now().UTC().Format(time.RFC3339),
		"cap":         capN,
		"shards":      snap,
	})
}

// shardMetricInput is the body of POST /router/shard-metrics.
type shardMetricInput struct {
	Shard  string  `json:"shard"`
	MemPct float64 `json:"memPct"`
	CPUPct float64 `json:"cpuPct"`
}

// handleIngest serves POST /router/shard-metrics — the sentinel pushes each
// shard's memory/CPU percentage here. Accepts a single object or an array.
func handleIngest(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	r.Body.Close()
	var items []shardMetricInput
	if err := json.Unmarshal(body, &items); err != nil {
		var one shardMetricInput
		if err2 := json.Unmarshal(body, &one); err2 != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
		items = []shardMetricInput{one}
	}
	now := time.Now()
	n := 0
	for _, it := range items {
		if it.Shard == "" || byName[it.Shard] == nil {
			continue // ignore unknown shards; never let a bad push corrupt state
		}
		mstore.set(it.Shard, it.MemPct, it.CPUPct, now)
		n++
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]int{"accepted": n})
}
