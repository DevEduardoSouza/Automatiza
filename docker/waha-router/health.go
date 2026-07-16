// health.go — shard health scoring and intelligent new-session placement.
//
// The router owns the SINGLE routing decision for NEW sessions (pickForCreate).
// Existing sessions are never moved: they stay routed to their owning upstream
// by ownerOf(). This file replaces the old count-only choice with a multi-factor
// health score so that growth is spread by real load (memory, CPU, connected
// sessions, errors, churn, latency, activity, capacity) instead of raw count.
//
// Everything here is configuration-driven (see Config, loaded from env) and
// works for N upstreams with zero shard-specific code. It has no external
// dependencies (stdlib only), so the router stays a tiny, auditable binary.
package main

import (
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ---- Configuration (ETAPA 12: everything externalized, no magic numbers) ----

// Config holds every tunable. Defaults are sensible for the current fleet and
// every value is overridable via an environment variable, so behaviour can be
// tuned without a code change or rebuild.
type Config struct {
	// Weights for the load factors (relative; only their ratios matter). A
	// factor with weight 0 is ignored entirely.
	WMemory   float64
	WCPU      float64
	WCapacity float64
	WErrors   float64
	WChurn    float64 // starting + reconnecting sessions (instability)
	WLatency  float64
	WActivity float64 // proxied request throughput per shard

	// Saturation: a shard at/above EITHER threshold stops receiving new
	// sessions (ETAPA 7). Existing sessions are untouched.
	SaturationMemPct  float64 // e.g. 90 => mem >= 90% => saturated
	SaturationUtilPct float64 // e.g. 95 => sessions/cap >= 95% => saturated

	// Normalization references (what counts as "fully loaded" = 1.0).
	LatencyBudgetMs float64 // poll latency at/above this => latency load 1.0
	ActivityRefRPS  float64 // per-shard req/s at/above this => activity load 1.0

	// Freshness: pushed mem/CPU older than this is treated as unknown and the
	// score gracefully degrades to the HTTP-measurable signals.
	MetricTTL time.Duration

	// UnknownLoad is the neutral load [0,1] used for a factor whose input is
	// unavailable (e.g. mem/CPU not pushed yet). 0.5 = "assume average", which
	// neither favours nor punishes a shard we can't measure.
	UnknownLoad float64

	RefreshInterval time.Duration // how often refresh() polls the upstreams
	PollTimeout     time.Duration // per-upstream HTTP timeout during refresh
}

// loadConfig builds Config from the environment, applying defaults for anything
// unset or invalid.
func loadConfig() Config {
	return Config{
		WMemory:           envFloat("ROUTER_W_MEMORY", 3.0),
		WCPU:              envFloat("ROUTER_W_CPU", 1.5),
		WCapacity:         envFloat("ROUTER_W_CAPACITY", 2.0),
		WErrors:           envFloat("ROUTER_W_ERRORS", 1.5),
		WChurn:            envFloat("ROUTER_W_CHURN", 1.0),
		WLatency:          envFloat("ROUTER_W_LATENCY", 1.0),
		WActivity:         envFloat("ROUTER_W_ACTIVITY", 1.0),
		SaturationMemPct:  envFloat("ROUTER_SATURATION_MEM_PCT", 90),
		SaturationUtilPct: envFloat("ROUTER_SATURATION_UTIL_PCT", 95),
		LatencyBudgetMs:   envFloat("ROUTER_LATENCY_BUDGET_MS", 2000),
		ActivityRefRPS:    envFloat("ROUTER_ACTIVITY_REF_RPS", 20),
		MetricTTL:         time.Duration(envInt("ROUTER_METRIC_TTL_SEC", 45)) * time.Second,
		UnknownLoad:       clamp01(envFloat("ROUTER_UNKNOWN_LOAD", 0.5)),
		RefreshInterval:   time.Duration(envInt("ROUTER_REFRESH_SEC", 30)) * time.Second,
		PollTimeout:       time.Duration(envInt("ROUTER_POLL_TIMEOUT_SEC", 8)) * time.Second,
	}
}

func envFloat(key string, def float64) float64 {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}

func envInt(key string, def int) int {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func clamp01(f float64) float64 {
	if f < 0 {
		return 0
	}
	if f > 1 {
		return 1
	}
	return f
}

// ---- Pushed metrics (ETAPA 8: mem/CPU fed by the per-machine sentinel) ----

// pushedMetric is a mem/CPU sample POSTed to /router/shard-metrics. It carries
// its own receipt time so the router can treat stale samples as unknown.
type pushedMetric struct {
	MemPct     float64
	CPUPct     float64
	receivedAt time.Time
}

// metricsStore holds the latest pushed mem/CPU per shard. Safe for concurrent
// use: the ingest endpoint writes, the scorer reads.
type metricsStore struct {
	mu sync.RWMutex
	m  map[string]pushedMetric
}

func newMetricsStore() *metricsStore { return &metricsStore{m: map[string]pushedMetric{}} }

func (s *metricsStore) set(shard string, mem, cpu float64, now time.Time) {
	s.mu.Lock()
	s.m[shard] = pushedMetric{MemPct: mem, CPUPct: cpu, receivedAt: now}
	s.mu.Unlock()
}

// get returns the sample and whether it is still fresh relative to ttl at `now`.
func (s *metricsStore) get(shard string, ttl time.Duration, now time.Time) (pushedMetric, bool) {
	s.mu.RLock()
	p, ok := s.m[shard]
	s.mu.RUnlock()
	if !ok {
		return pushedMetric{}, false
	}
	fresh := now.Sub(p.receivedAt) <= ttl
	return p, fresh
}

// ---- Per-shard health snapshot (ETAPA 2,4,8) ----

// ShardHealth is a point-in-time view of one upstream, assembled by refresh()
// from what the router can measure over HTTP plus the pushed mem/CPU.
type ShardHealth struct {
	Name         string  `json:"name"`
	Available    bool    `json:"available"`
	LatencyMs    float64 `json:"latencyMs"`
	Total        int     `json:"totalSessions"`
	Connected    int     `json:"connected"`    // WORKING
	Failed       int     `json:"failed"`       // FAILED
	Starting     int     `json:"starting"`     // STARTING
	Reconnecting int     `json:"reconnecting"` // SCAN_QR_CODE (needs re-link)
	Stopped      int     `json:"stopped"`      // STOPPED
	UptimeMs     float64 `json:"uptimeMs"`
	RequestRPS   float64 `json:"requestRps"` // proxied API throughput to this shard
	MemPct       float64 `json:"memPct"`     // -1 when unknown/stale
	CPUPct       float64 `json:"cpuPct"`     // -1 when unknown/stale
	MetricFresh  bool    `json:"metricFresh"`
	Cap          int     `json:"cap"`
	Remaining    int     `json:"remaining"`
	Saturated    bool    `json:"saturated"`
	Score        float64 `json:"score"` // 0..1, higher = healthier for new placement
	Eligible     bool    `json:"eligible"`
}

// utilPct is sessions/cap as a percentage (0 when cap<=0).
func (h ShardHealth) utilPct() float64 {
	if h.Cap <= 0 {
		return 0
	}
	return float64(h.Total) / float64(h.Cap) * 100
}

// computeScore fills Saturated/Score/Eligible from the raw signals + config.
// Load model: every factor is normalized to [0,1] where 0 = idle/healthy and
// 1 = fully loaded/unhealthy. score = 1 - weightedMean(loads). The healthiest
// (highest score) eligible shard wins in pickForCreate.
func (h *ShardHealth) computeScore(c Config) {
	// Saturation & eligibility gates (ETAPA 7, 10).
	memSat := h.MemPct >= 0 && h.MemPct >= c.SaturationMemPct
	utilSat := h.utilPct() >= c.SaturationUtilPct
	h.Saturated = memSat || utilSat
	h.Remaining = h.Cap - h.Total
	if h.Remaining < 0 {
		h.Remaining = 0
	}
	h.Eligible = h.Available && !h.Saturated && h.Total < h.Cap

	// Factor loads (0 good .. 1 bad).
	memLoad := c.UnknownLoad
	if h.MemPct >= 0 {
		memLoad = clamp01(h.MemPct / 100)
	}
	cpuLoad := c.UnknownLoad
	if h.CPUPct >= 0 {
		cpuLoad = clamp01(h.CPUPct / 100)
	}
	capLoad := 0.0
	if h.Cap > 0 {
		capLoad = clamp01(float64(h.Total) / float64(h.Cap))
	}
	errLoad := 0.0
	churnLoad := 0.0
	if h.Total > 0 {
		errLoad = clamp01(float64(h.Failed) / float64(h.Total))
		churnLoad = clamp01(float64(h.Starting+h.Reconnecting) / float64(h.Total))
	}
	latLoad := 0.0
	if c.LatencyBudgetMs > 0 {
		latLoad = clamp01(h.LatencyMs / c.LatencyBudgetMs)
	}
	actLoad := 0.0
	if c.ActivityRefRPS > 0 {
		actLoad = clamp01(h.RequestRPS / c.ActivityRefRPS)
	}

	sumW := c.WMemory + c.WCPU + c.WCapacity + c.WErrors + c.WChurn + c.WLatency + c.WActivity
	if sumW <= 0 {
		// Degenerate config: fall back to pure capacity so we never divide by 0.
		h.Score = 1 - capLoad
		return
	}
	weighted := c.WMemory*memLoad + c.WCPU*cpuLoad + c.WCapacity*capLoad +
		c.WErrors*errLoad + c.WChurn*churnLoad + c.WLatency*latLoad + c.WActivity*actLoad
	h.Score = 1 - clamp01(weighted/sumW)
}

// selectBest returns the healthiest eligible shard for a NEW session.
// Preference order:
//  1. eligible (available, not saturated, under cap) with the highest score;
//  2. if none are eligible (capacity emergency), the least-loaded available
//     shard under cap, so a user's pairing is never refused — the caller logs
//     this loudly for alerting;
//  3. nil => caller falls back to primary().
//
// It never considers moving existing sessions; it only chooses placement.
func selectBest(shards []ShardHealth) (best *ShardHealth, emergency bool) {
	var chosen *ShardHealth
	for i := range shards {
		s := &shards[i]
		if !s.Eligible {
			continue
		}
		if chosen == nil || s.Score > chosen.Score {
			chosen = s
		}
	}
	if chosen != nil {
		return chosen, false
	}
	// Emergency fallback: best available shard still under cap, even if saturated.
	for i := range shards {
		s := &shards[i]
		if !s.Available || s.Total >= s.Cap {
			continue
		}
		if chosen == nil || s.Score > chosen.Score {
			chosen = s
		}
	}
	return chosen, chosen != nil
}
