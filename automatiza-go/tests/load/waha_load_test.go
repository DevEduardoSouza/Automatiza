// Package load provides load testing for the WAHA WhatsApp integration.
// These tests measure performance, throughput, and behavior under load.
//
// To run load tests:
//
//	go test -v -tags=load -timeout=30m ./tests/load/...
//
// For benchmarks:
//
//	go test -v -tags=load -bench=. -benchtime=10s ./tests/load/...
//
// Environment variables for real WAHA testing:
//   - WAHA_API_URL: URL of the WAHA Plus server
//   - WAHA_API_KEY: API key for WAHA authentication
//   - WAHA_TEST_SESSION: Name of existing test session
//
//go:build load

package load

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	mathrand "math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"automatiza-go/internal/adapters/whatsapp"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// =============================================================================
// Load Test Configuration
// =============================================================================

type loadTestConfig struct {
	// Concurrency settings
	NumWorkers        int
	RequestsPerWorker int

	// Timing
	RampUpDuration time.Duration
	TestDuration   time.Duration
	RequestTimeout time.Duration

	// Rate limiting
	MaxRequestsPerSecond int
}

func defaultLoadConfig() *loadTestConfig {
	return &loadTestConfig{
		NumWorkers:           10,
		RequestsPerWorker:    100,
		RampUpDuration:       5 * time.Second,
		TestDuration:         30 * time.Second,
		RequestTimeout:       10 * time.Second,
		MaxRequestsPerSecond: 100,
	}
}

// loadTestMetrics collects metrics during load tests.
type loadTestMetrics struct {
	totalRequests      int64
	successfulRequests int64
	failedRequests     int64
	totalLatencyMs     int64
	minLatencyMs       int64
	maxLatencyMs       int64
	mu                 sync.Mutex
}

func newLoadTestMetrics() *loadTestMetrics {
	return &loadTestMetrics{
		minLatencyMs: int64(^uint64(0) >> 1), // MaxInt64
	}
}

func (m *loadTestMetrics) recordSuccess(latencyMs int64) {
	atomic.AddInt64(&m.totalRequests, 1)
	atomic.AddInt64(&m.successfulRequests, 1)
	atomic.AddInt64(&m.totalLatencyMs, latencyMs)

	m.mu.Lock()
	defer m.mu.Unlock()
	if latencyMs < m.minLatencyMs {
		m.minLatencyMs = latencyMs
	}
	if latencyMs > m.maxLatencyMs {
		m.maxLatencyMs = latencyMs
	}
}

func (m *loadTestMetrics) recordFailure() {
	atomic.AddInt64(&m.totalRequests, 1)
	atomic.AddInt64(&m.failedRequests, 1)
}

func (m *loadTestMetrics) report(t *testing.T, duration time.Duration) {
	total := atomic.LoadInt64(&m.totalRequests)
	success := atomic.LoadInt64(&m.successfulRequests)
	failed := atomic.LoadInt64(&m.failedRequests)
	totalLatency := atomic.LoadInt64(&m.totalLatencyMs)

	if total == 0 {
		t.Log("No requests were made")
		return
	}

	avgLatency := float64(totalLatency) / float64(success)
	throughput := float64(total) / duration.Seconds()
	successRate := float64(success) / float64(total) * 100

	t.Logf("\n=== Load Test Results ===")
	t.Logf("Duration:        %v", duration)
	t.Logf("Total Requests:  %d", total)
	t.Logf("Successful:      %d (%.2f%%)", success, successRate)
	t.Logf("Failed:          %d", failed)
	t.Logf("Throughput:      %.2f req/s", throughput)
	t.Logf("Avg Latency:     %.2f ms", avgLatency)
	t.Logf("Min Latency:     %d ms", m.minLatencyMs)
	t.Logf("Max Latency:     %d ms", m.maxLatencyMs)
}

// =============================================================================
// FakeAdapter Load Tests (no external dependencies)
// =============================================================================

func TestLoad_FakeAdapter_SessionStatus(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping load test in short mode")
	}

	cfg := defaultLoadConfig()
	adapter := whatsapp.NewFakeAdapter()
	metrics := newLoadTestMetrics()

	ctx, cancel := context.WithTimeout(context.Background(), cfg.TestDuration)
	defer cancel()

	var wg sync.WaitGroup
	startTime := time.Now()

	for i := 0; i < cfg.NumWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for j := 0; j < cfg.RequestsPerWorker; j++ {
				select {
				case <-ctx.Done():
					return
				default:
				}

				start := time.Now()
				_, err := adapter.GetSessionStatus(ctx, fmt.Sprintf("session-%d", workerID))
				latency := time.Since(start).Milliseconds()

				if err != nil {
					metrics.recordFailure()
				} else {
					metrics.recordSuccess(latency)
				}
			}
		}(i)
	}

	wg.Wait()
	duration := time.Since(startTime)

	metrics.report(t, duration)

	// Assertions
	assert.Equal(t, int64(0), atomic.LoadInt64(&metrics.failedRequests),
		"FakeAdapter should have zero failures")
}

func TestLoad_FakeAdapter_ConcurrentOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping load test in short mode")
	}

	adapter := whatsapp.NewFakeAdapter()
	metrics := newLoadTestMetrics()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	numOperations := 1000
	startTime := time.Now()

	operations := []func(ctx context.Context, adapter *whatsapp.FakeAdapter) error{
		func(ctx context.Context, a *whatsapp.FakeAdapter) error {
			_, err := a.GetSessionStatus(ctx, "test-session")
			return err
		},
		func(ctx context.Context, a *whatsapp.FakeAdapter) error {
			_, err := a.GetGroups(ctx, "test-session")
			return err
		},
		func(ctx context.Context, a *whatsapp.FakeAdapter) error {
			_, err := a.SendText(ctx, whatsapp.SendTextInput{
				SessionName: "test-session",
				ChatID:      "test-chat@g.us",
				Text:        "Load test message",
			})
			return err
		},
		func(ctx context.Context, a *whatsapp.FakeAdapter) error {
			_, err := a.GetMessages(ctx, whatsapp.GetMessagesInput{
				SessionName: "test-session",
				ChatID:      "test-chat@g.us",
				Limit:       50,
			})
			return err
		},
	}

	for i := 0; i < numOperations; i++ {
		wg.Add(1)
		go func(opIndex int) {
			defer wg.Done()

			op := operations[opIndex%len(operations)]

			start := time.Now()
			err := op(ctx, adapter)
			latency := time.Since(start).Milliseconds()

			if err != nil {
				metrics.recordFailure()
			} else {
				metrics.recordSuccess(latency)
			}
		}(i)
	}

	wg.Wait()
	duration := time.Since(startTime)

	metrics.report(t, duration)

	// Verify all operations completed successfully
	total := atomic.LoadInt64(&metrics.totalRequests)
	assert.Equal(t, int64(numOperations), total, "All operations should complete")
}

// =============================================================================
// HTTP Webhook Handler Load Tests
// =============================================================================

func TestLoad_WebhookHandler_Throughput(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping load test in short mode")
	}

	hmacSecret := "test-load-secret"
	validator := whatsapp.NewHMACValidator(hmacSecret)
	metrics := newLoadTestMetrics()

	// Create a simple handler that validates HMAC
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)

		signature := r.Header.Get("X-Signature")
		if err := validator.Validate(body, signature); err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		w.WriteHeader(http.StatusOK)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	numWorkers := 20
	requestsPerWorker := 500
	startTime := time.Now()

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			client := &http.Client{Timeout: 5 * time.Second}

			for j := 0; j < requestsPerWorker; j++ {
				select {
				case <-ctx.Done():
					return
				default:
				}

				// Generate unique event ID for each request
				eventID := generateEventID()
				payload := fmt.Sprintf(`{"id":"%s","event":"message","session":"worker-%d"}`, eventID, workerID)
				signature := validator.ComputeSignature([]byte(payload))

				start := time.Now()

				req, _ := http.NewRequestWithContext(ctx, "POST", server.URL,
					newStringReader(payload))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("X-Signature", signature)

				resp, err := client.Do(req)
				latency := time.Since(start).Milliseconds()

				if err != nil || resp.StatusCode != http.StatusOK {
					metrics.recordFailure()
				} else {
					metrics.recordSuccess(latency)
				}

				if resp != nil {
					resp.Body.Close()
				}
			}
		}(i)
	}

	wg.Wait()
	duration := time.Since(startTime)

	metrics.report(t, duration)

	// Performance assertions
	throughput := float64(atomic.LoadInt64(&metrics.totalRequests)) / duration.Seconds()
	successRate := float64(atomic.LoadInt64(&metrics.successfulRequests)) /
		float64(atomic.LoadInt64(&metrics.totalRequests)) * 100

	assert.Greater(t, throughput, 100.0, "Should handle at least 100 req/s")
	assert.Greater(t, successRate, 99.0, "Success rate should be > 99%")
}

func TestLoad_HMACValidation_Performance(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping load test in short mode")
	}

	secret := "load-test-hmac-secret-key"
	validator := whatsapp.NewHMACValidator(secret)

	// Pre-generate test payloads
	payloads := make([][]byte, 1000)
	signatures := make([]string, 1000)

	for i := 0; i < 1000; i++ {
		payloads[i] = []byte(fmt.Sprintf(`{"id":"evt_%05d","event":"message","timestamp":%d}`,
			i, time.Now().UnixNano()))
		signatures[i] = validator.ComputeSignature(payloads[i])
	}

	metrics := newLoadTestMetrics()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	numWorkers := 50
	startTime := time.Now()

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for j := 0; j < 10000; j++ {
				select {
				case <-ctx.Done():
					return
				default:
				}

				idx := (workerID*10000 + j) % 1000

				start := time.Now()
				err := validator.Validate(payloads[idx], signatures[idx])
				latency := time.Since(start).Microseconds() // Use microseconds for more precision

				if err != nil {
					metrics.recordFailure()
				} else {
					metrics.recordSuccess(latency)
				}
			}
		}(i)
	}

	wg.Wait()
	duration := time.Since(startTime)

	total := atomic.LoadInt64(&metrics.totalRequests)
	throughput := float64(total) / duration.Seconds()

	t.Logf("\n=== HMAC Validation Performance ===")
	t.Logf("Total validations: %d", total)
	t.Logf("Duration: %v", duration)
	t.Logf("Throughput: %.2f validations/sec", throughput)
	t.Logf("Avg Latency: %.2f µs", float64(atomic.LoadInt64(&metrics.totalLatencyMs))/float64(total))

	// HMAC should be very fast
	assert.Greater(t, throughput, 50000.0, "Should validate at least 50K/s")
}

// =============================================================================
// Message Sending Load Tests
// =============================================================================

func TestLoad_MessageSending_Throughput(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping load test in short mode")
	}

	adapter := whatsapp.NewFakeAdapter()

	// Configure fake adapter to simulate realistic latency
	adapter.SendTextFunc = func(ctx context.Context, input whatsapp.SendTextInput) (*whatsapp.MessageResult, error) {
		// Simulate 50-100ms network latency
		time.Sleep(time.Duration(50+mathrand.Intn(50)) * time.Millisecond)
		return &whatsapp.MessageResult{
			ID:        generateEventID(),
			Timestamp: time.Now(),
		}, nil
	}

	adapter.GetSessionStatusFunc = func(ctx context.Context, name string) (*whatsapp.SessionInfo, error) {
		return &whatsapp.SessionInfo{
			Name:   name,
			Status: whatsapp.SessionStatusConnected,
		}, nil
	}

	metrics := newLoadTestMetrics()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	numWorkers := 10
	messagesPerWorker := 50
	startTime := time.Now()

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for j := 0; j < messagesPerWorker; j++ {
				select {
				case <-ctx.Done():
					return
				default:
				}

				start := time.Now()
				_, err := adapter.SendText(ctx, whatsapp.SendTextInput{
					SessionName: fmt.Sprintf("session-%d", workerID),
					ChatID:      fmt.Sprintf("group-%d@g.us", j%10),
					Text:        fmt.Sprintf("Load test message %d from worker %d", j, workerID),
				})
				latency := time.Since(start).Milliseconds()

				if err != nil {
					metrics.recordFailure()
				} else {
					metrics.recordSuccess(latency)
				}
			}
		}(i)
	}

	wg.Wait()
	duration := time.Since(startTime)

	metrics.report(t, duration)

	// Verify all messages were sent
	total := atomic.LoadInt64(&metrics.totalRequests)
	expected := int64(numWorkers * messagesPerWorker)
	assert.Equal(t, expected, total, "All messages should be sent")
}

// =============================================================================
// Real WAHA Load Tests (requires staging environment)
// =============================================================================

func TestLoad_RealWAHA_SessionStatus(t *testing.T) {
	apiURL := os.Getenv("WAHA_API_URL")
	apiKey := os.Getenv("WAHA_API_KEY")
	testSession := os.Getenv("WAHA_TEST_SESSION")

	if apiURL == "" || apiKey == "" || testSession == "" {
		t.Skip("WAHA credentials not set, skipping real load test")
	}

	logger := zap.NewNop()
	adapter, err := whatsapp.NewWahaAdapter(whatsapp.WahaAdapterConfig{
		APIURL:      apiURL,
		APIKey:      apiKey,
		HTTPTimeout: 30 * time.Second,
	}, logger)
	require.NoError(t, err)

	ctx := context.Background()

	// Verify session exists
	status, err := adapter.GetSessionStatus(ctx, testSession)
	require.NoError(t, err)

	if status.Status != whatsapp.SessionStatusConnected {
		t.Skipf("Session not connected (status: %s)", status.Status)
	}

	metrics := newLoadTestMetrics()
	testCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	numWorkers := 5 // Lower concurrency for real API
	requestsPerWorker := 20
	startTime := time.Now()

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for j := 0; j < requestsPerWorker; j++ {
				select {
				case <-testCtx.Done():
					return
				default:
				}

				start := time.Now()
				_, err := adapter.GetSessionStatus(testCtx, testSession)
				latency := time.Since(start).Milliseconds()

				if err != nil {
					metrics.recordFailure()
					t.Logf("Request failed: %v", err)
				} else {
					metrics.recordSuccess(latency)
				}

				// Rate limiting: don't overwhelm the server
				time.Sleep(50 * time.Millisecond)
			}
		}(i)
	}

	wg.Wait()
	duration := time.Since(startTime)

	metrics.report(t, duration)
}

// =============================================================================
// Stress Tests
// =============================================================================

func TestStress_FakeAdapter_HighConcurrency(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	adapter := whatsapp.NewFakeAdapter()
	metrics := newLoadTestMetrics()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	numWorkers := 100 // High concurrency
	requestsPerWorker := 1000
	startTime := time.Now()

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for j := 0; j < requestsPerWorker; j++ {
				select {
				case <-ctx.Done():
					return
				default:
				}

				start := time.Now()

				// Mix of operations
				switch j % 4 {
				case 0:
					_, _ = adapter.GetSessionStatus(ctx, "stress-session")
				case 1:
					_, _ = adapter.GetGroups(ctx, "stress-session")
				case 2:
					_, _ = adapter.SendText(ctx, whatsapp.SendTextInput{
						SessionName: "stress-session",
						ChatID:      "group@g.us",
						Text:        "Stress test",
					})
				case 3:
					_, _ = adapter.GetMessages(ctx, whatsapp.GetMessagesInput{
						SessionName: "stress-session",
						ChatID:      "group@g.us",
						Limit:       10,
					})
				}

				latency := time.Since(start).Milliseconds()
				metrics.recordSuccess(latency)
			}
		}(i)
	}

	wg.Wait()
	duration := time.Since(startTime)

	metrics.report(t, duration)

	// High concurrency should still complete all requests
	total := atomic.LoadInt64(&metrics.totalRequests)
	expected := int64(numWorkers * requestsPerWorker)
	assert.Equal(t, expected, total, "All stress test requests should complete")
}

// =============================================================================
// Benchmarks
// =============================================================================

func BenchmarkFakeAdapter_GetSessionStatus(b *testing.B) {
	adapter := whatsapp.NewFakeAdapter()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = adapter.GetSessionStatus(ctx, "bench-session")
	}
}

func BenchmarkFakeAdapter_SendText(b *testing.B) {
	adapter := whatsapp.NewFakeAdapter()
	ctx := context.Background()

	input := whatsapp.SendTextInput{
		SessionName: "bench-session",
		ChatID:      "group@g.us",
		Text:        "Benchmark message",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = adapter.SendText(ctx, input)
	}
}

func BenchmarkHMACValidator_Validate(b *testing.B) {
	validator := whatsapp.NewHMACValidator("benchmark-secret-key")
	payload := []byte(`{"id":"evt_benchmark","event":"message","session":"test"}`)
	signature := validator.ComputeSignature(payload)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = validator.Validate(payload, signature)
	}
}

func BenchmarkHMACValidator_ComputeSignature(b *testing.B) {
	validator := whatsapp.NewHMACValidator("benchmark-secret-key")
	payload := []byte(`{"id":"evt_benchmark","event":"message","session":"test"}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = validator.ComputeSignature(payload)
	}
}

func BenchmarkFakeAdapter_Parallel(b *testing.B) {
	adapter := whatsapp.NewFakeAdapter()
	ctx := context.Background()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = adapter.GetSessionStatus(ctx, "parallel-session")
		}
	})
}

// =============================================================================
// Helper Functions
// =============================================================================

func generateEventID() string {
	bytes := make([]byte, 13)
	rand.Read(bytes)
	return "evt_" + hex.EncodeToString(bytes)
}

type stringReader struct {
	s string
	i int64
}

func newStringReader(s string) *stringReader {
	return &stringReader{s: s}
}

func (r *stringReader) Read(b []byte) (n int, err error) {
	if r.i >= int64(len(r.s)) {
		return 0, io.EOF
	}
	n = copy(b, r.s[r.i:])
	r.i += int64(n)
	return
}
