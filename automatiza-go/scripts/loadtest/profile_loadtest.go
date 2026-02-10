// Package main provides load testing for profile and password reset endpoints.
// It tests ConnectRPC endpoints using the Connect protocol with JSON encoding.
//
// Usage:
//
//	go run ./scripts/loadtest/profile_loadtest.go -url http://localhost:3000 -token <jwt_token>
//
// Scenarios:
//  1. GetProfile - 100 req/s for 30s (target: p99 < 100ms)
//  2. GetUserLimits - 100 req/s for 30s (target: p99 < 150ms)
//  3. RequestPasswordReset - 10 req/s for 60s (rate limiting test)
//  4. UpdateProfile - 20 req/s for 30s (target: p99 < 200ms)
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// Config holds load test configuration
type Config struct {
	BaseURL   string
	AuthToken string
	CSRFToken string
	Verbose   bool
}

// Metrics tracks request statistics
type Metrics struct {
	mu             sync.Mutex
	latencies      []time.Duration
	successCount   int64
	errorCount     int64
	rateLimitCount int64
	startTime      time.Time
}

// NewMetrics creates a new Metrics instance
func NewMetrics() *Metrics {
	return &Metrics{
		latencies: make([]time.Duration, 0, 10000),
		startTime: time.Now(),
	}
}

// Record records a request result
func (m *Metrics) Record(latency time.Duration, success bool, rateLimited bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.latencies = append(m.latencies, latency)
	if success {
		atomic.AddInt64(&m.successCount, 1)
	} else {
		atomic.AddInt64(&m.errorCount, 1)
	}
	if rateLimited {
		atomic.AddInt64(&m.rateLimitCount, 1)
	}
}

// Summary returns a summary of the metrics
func (m *Metrics) Summary() MetricsSummary {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.latencies) == 0 {
		return MetricsSummary{}
	}

	sorted := make([]time.Duration, len(m.latencies))
	copy(sorted, m.latencies)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	var total time.Duration
	for _, l := range sorted {
		total += l
	}

	n := len(sorted)
	elapsed := time.Since(m.startTime)

	return MetricsSummary{
		TotalRequests:  n,
		SuccessCount:   int(m.successCount),
		ErrorCount:     int(m.errorCount),
		RateLimitCount: int(m.rateLimitCount),
		P50:            sorted[n*50/100],
		P95:            sorted[n*95/100],
		P99:            sorted[n*99/100],
		Min:            sorted[0],
		Max:            sorted[n-1],
		Avg:            total / time.Duration(n),
		Elapsed:        elapsed,
		RPS:            float64(n) / elapsed.Seconds(),
	}
}

// MetricsSummary contains computed metrics
type MetricsSummary struct {
	TotalRequests  int
	SuccessCount   int
	ErrorCount     int
	RateLimitCount int
	P50            time.Duration
	P95            time.Duration
	P99            time.Duration
	Min            time.Duration
	Max            time.Duration
	Avg            time.Duration
	Elapsed        time.Duration
	RPS            float64
}

// Scenario defines a load test scenario
type Scenario struct {
	Name        string
	Endpoint    string
	Method      string
	Body        func(iteration int) []byte
	Headers     func(cfg Config, iteration int) map[string]string
	RPS         int
	Duration    time.Duration
	TargetP99   time.Duration
	CheckResult func(status int, body []byte) (success bool, rateLimited bool)
}

func main() {
	var (
		baseURL   = flag.String("url", "http://localhost:3000", "Base URL of the server")
		authToken = flag.String("token", "", "JWT auth token (required for authenticated endpoints)")
		csrfToken = flag.String("csrf", "", "CSRF token (optional)")
		scenario  = flag.String("scenario", "all", "Scenario to run: getprofile, getlimits, resetpassword, updateprofile, all")
		verbose   = flag.Bool("v", false, "Verbose output")
	)
	flag.Parse()

	cfg := Config{
		BaseURL:   *baseURL,
		AuthToken: *authToken,
		CSRFToken: *csrfToken,
		Verbose:   *verbose,
	}

	scenarios := buildScenarios()

	if *scenario == "all" {
		fmt.Println("========================================")
		fmt.Println("  Profile & Password Reset Load Test")
		fmt.Println("========================================")
		fmt.Printf("Base URL: %s\n", cfg.BaseURL)
		fmt.Printf("Auth Token: %s\n", maskToken(cfg.AuthToken))
		fmt.Println()

		for _, s := range scenarios {
			runScenario(cfg, s)
			fmt.Println()
		}
	} else {
		for _, s := range scenarios {
			if s.Name == *scenario {
				runScenario(cfg, s)
				return
			}
		}
		fmt.Printf("Unknown scenario: %s\n", *scenario)
		fmt.Println("Available scenarios: getprofile, getlimits, resetpassword, updateprofile, all")
		os.Exit(1)
	}
}

func buildScenarios() []Scenario {
	return []Scenario{
		{
			Name:      "getprofile",
			Endpoint:  "/profiles.v1.ProfilesService/GetProfile",
			Method:    "POST",
			Body:      func(_ int) []byte { return []byte(`{}`) },
			Headers:   authHeaders,
			RPS:       100,
			Duration:  30 * time.Second,
			TargetP99: 100 * time.Millisecond,
			CheckResult: func(status int, _ []byte) (bool, bool) {
				return status == 200, false
			},
		},
		{
			Name:      "getlimits",
			Endpoint:  "/profiles.v1.ProfilesService/GetUserLimits",
			Method:    "POST",
			Body:      func(_ int) []byte { return []byte(`{}`) },
			Headers:   authHeaders,
			RPS:       100,
			Duration:  30 * time.Second,
			TargetP99: 150 * time.Millisecond,
			CheckResult: func(status int, _ []byte) (bool, bool) {
				return status == 200, false
			},
		},
		{
			Name:     "resetpassword",
			Endpoint: "/profiles.v1.ProfilesService/RequestPasswordReset",
			Method:   "POST",
			Body: func(iteration int) []byte {
				// Use different emails to test rate limiting per email
				email := fmt.Sprintf("loadtest+%d@example.com", iteration%10)
				body, _ := json.Marshal(map[string]string{"email": email})
				return body
			},
			Headers:   publicHeaders,
			RPS:       10,
			Duration:  60 * time.Second,
			TargetP99: 500 * time.Millisecond, // Higher target due to email sending
			CheckResult: func(status int, body []byte) (bool, bool) {
				if status != 200 {
					return false, false
				}
				// Check if rate limited by response message
				var resp map[string]interface{}
				if err := json.Unmarshal(body, &resp); err != nil {
					return true, false
				}
				msg, _ := resp["message"].(string)
				success, _ := resp["success"].(bool)
				rateLimited := !success || (msg != "" && contains(msg, "tentativas"))
				return !rateLimited, rateLimited
			},
		},
		{
			Name:     "updateprofile",
			Endpoint: "/profiles.v1.ProfilesService/UpdateProfile",
			Method:   "POST",
			Body: func(iteration int) []byte {
				phone := fmt.Sprintf("5511999%06d", iteration%1000000)
				body, _ := json.Marshal(map[string]string{"phone_number": phone})
				return body
			},
			Headers:   authHeaders,
			RPS:       20,
			Duration:  30 * time.Second,
			TargetP99: 200 * time.Millisecond,
			CheckResult: func(status int, _ []byte) (bool, bool) {
				return status == 200, false
			},
		},
	}
}

func authHeaders(cfg Config, _ int) map[string]string {
	headers := map[string]string{
		"Content-Type": "application/json",
	}
	if cfg.AuthToken != "" {
		headers["Authorization"] = "Bearer " + cfg.AuthToken
	}
	if cfg.CSRFToken != "" {
		headers["X-CSRF-Token"] = cfg.CSRFToken
	}
	return headers
}

func publicHeaders(_ Config, _ int) map[string]string {
	return map[string]string{
		"Content-Type": "application/json",
	}
}

func runScenario(cfg Config, s Scenario) {
	fmt.Printf("----------------------------------------\n")
	fmt.Printf("Scenario: %s\n", s.Name)
	fmt.Printf("Endpoint: %s\n", s.Endpoint)
	fmt.Printf("Target: %d req/s for %s (p99 < %s)\n", s.RPS, s.Duration, s.TargetP99)
	fmt.Printf("----------------------------------------\n")

	// Check if auth token is required but missing
	if s.Headers == nil {
		s.Headers = publicHeaders
	}
	testHeaders := s.Headers(cfg, 0)
	if _, hasAuth := testHeaders["Authorization"]; hasAuth && cfg.AuthToken == "" {
		fmt.Println("SKIP: Requires auth token (use -token flag)")
		return
	}

	metrics := NewMetrics()
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        200,
			MaxIdleConnsPerHost: 200,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.Duration+5*time.Second)
	defer cancel()

	// Calculate interval between requests
	interval := time.Second / time.Duration(s.RPS)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var wg sync.WaitGroup
	var iteration atomic.Int64
	endTime := time.Now().Add(s.Duration)

	// Progress ticker
	progressTicker := time.NewTicker(5 * time.Second)
	defer progressTicker.Stop()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-progressTicker.C:
				summary := metrics.Summary()
				fmt.Printf("  Progress: %d requests, %.1f req/s, p99=%s\n",
					summary.TotalRequests, summary.RPS, summary.P99)
			}
		}
	}()

	fmt.Println("Running...")
	for time.Now().Before(endTime) {
		select {
		case <-ctx.Done():
			goto done
		case <-ticker.C:
			wg.Add(1)
			iter := int(iteration.Add(1))
			go func(i int) {
				defer wg.Done()
				executeRequest(ctx, client, cfg, s, metrics, i)
			}(iter)
		}
	}

done:
	wg.Wait()

	summary := metrics.Summary()
	printSummary(s, summary)
}

func executeRequest(ctx context.Context, client *http.Client, cfg Config, s Scenario, metrics *Metrics, iteration int) {
	url := cfg.BaseURL + s.Endpoint
	body := s.Body(iteration)

	req, err := http.NewRequestWithContext(ctx, s.Method, url, bytes.NewReader(body))
	if err != nil {
		metrics.Record(0, false, false)
		return
	}

	for k, v := range s.Headers(cfg, iteration) {
		req.Header.Set(k, v)
	}

	start := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(start)

	if err != nil {
		if cfg.Verbose {
			fmt.Printf("  Error: %v\n", err)
		}
		metrics.Record(latency, false, false)
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	success, rateLimited := s.CheckResult(resp.StatusCode, respBody)

	if cfg.Verbose && !success {
		fmt.Printf("  Status: %d, Body: %s\n", resp.StatusCode, string(respBody))
	}

	metrics.Record(latency, success, rateLimited)
}

func printSummary(s Scenario, m MetricsSummary) {
	fmt.Println()
	fmt.Println("Results:")
	fmt.Printf("  Total Requests:   %d\n", m.TotalRequests)
	fmt.Printf("  Success:          %d (%.1f%%)\n", m.SuccessCount, pct(m.SuccessCount, m.TotalRequests))
	fmt.Printf("  Errors:           %d (%.1f%%)\n", m.ErrorCount, pct(m.ErrorCount, m.TotalRequests))
	if m.RateLimitCount > 0 {
		fmt.Printf("  Rate Limited:     %d (%.1f%%)\n", m.RateLimitCount, pct(m.RateLimitCount, m.TotalRequests))
	}
	fmt.Printf("  Duration:         %s\n", m.Elapsed.Round(time.Millisecond))
	fmt.Printf("  Actual RPS:       %.1f\n", m.RPS)
	fmt.Println()
	fmt.Println("Latency:")
	fmt.Printf("  Min:              %s\n", m.Min)
	fmt.Printf("  Avg:              %s\n", m.Avg)
	fmt.Printf("  P50:              %s\n", m.P50)
	fmt.Printf("  P95:              %s\n", m.P95)
	fmt.Printf("  P99:              %s\n", m.P99)
	fmt.Printf("  Max:              %s\n", m.Max)
	fmt.Println()

	// Check against target
	if m.P99 <= s.TargetP99 {
		fmt.Printf("PASS: p99 (%s) <= target (%s)\n", m.P99, s.TargetP99)
	} else {
		fmt.Printf("FAIL: p99 (%s) > target (%s)\n", m.P99, s.TargetP99)
	}

	// For rate limiting scenario, check that rate limiting kicked in
	if s.Name == "resetpassword" {
		expectedRateLimits := int(math.Max(0, float64(m.TotalRequests)-30)) // ~3 per email * 10 emails
		if m.RateLimitCount > 0 {
			fmt.Printf("PASS: Rate limiting detected (%d requests limited)\n", m.RateLimitCount)
		} else if m.TotalRequests > 60 && m.RateLimitCount < expectedRateLimits/2 {
			fmt.Printf("WARN: Expected more rate limiting (got %d, expected ~%d+)\n", m.RateLimitCount, expectedRateLimits/2)
		}
	}
}

func pct(count, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(count) / float64(total) * 100
}

func maskToken(token string) string {
	if token == "" {
		return "(not provided)"
	}
	if len(token) < 20 {
		return "***"
	}
	return token[:10] + "..." + token[len(token)-5:]
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
