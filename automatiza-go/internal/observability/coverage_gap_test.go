package observability

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/zap"
)

// =============================================================================
// metrics.go — error branches for each instrument creation
// =============================================================================

// failingMeter embeds noop.Meter but returns an error for a specific instrument name.
type failingMeter struct {
	metricnoop.Meter
	failAt string
}

func (m *failingMeter) Int64Counter(name string, _ ...metric.Int64CounterOption) (metric.Int64Counter, error) {
	if name == m.failAt {
		return nil, errors.New("intentional Int64Counter failure")
	}
	return m.Meter.Int64Counter(name)
}

func (m *failingMeter) Float64Histogram(name string, _ ...metric.Float64HistogramOption) (metric.Float64Histogram, error) {
	if name == m.failAt {
		return nil, errors.New("intentional Float64Histogram failure")
	}
	return m.Meter.Float64Histogram(name)
}

// failingMeterProvider embeds noop.MeterProvider to satisfy the sealed interface
// and overrides Meter to return a failingMeter.
type failingMeterProvider struct {
	metricnoop.MeterProvider
	meter metric.Meter
}

func (p *failingMeterProvider) Meter(_ string, _ ...metric.MeterOption) metric.Meter {
	return p.meter
}

func TestInitMetrics_FailsOnMessagesDispatched(t *testing.T) {
	origMetrics := GlobalMetrics
	origMP := otel.GetMeterProvider()
	defer func() {
		GlobalMetrics = origMetrics
		otel.SetMeterProvider(origMP)
	}()

	otel.SetMeterProvider(&failingMeterProvider{
		meter: &failingMeter{failAt: "automatiza.messages.dispatched"},
	})

	err := InitMetrics(true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "intentional Int64Counter failure")
}

func TestInitMetrics_FailsOnAutomationRuns(t *testing.T) {
	origMetrics := GlobalMetrics
	origMP := otel.GetMeterProvider()
	defer func() {
		GlobalMetrics = origMetrics
		otel.SetMeterProvider(origMP)
	}()

	otel.SetMeterProvider(&failingMeterProvider{
		meter: &failingMeter{failAt: "automatiza.automations.runs"},
	})

	err := InitMetrics(true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "intentional Int64Counter failure")
}

func TestInitMetrics_FailsOnWebhookEvents(t *testing.T) {
	origMetrics := GlobalMetrics
	origMP := otel.GetMeterProvider()
	defer func() {
		GlobalMetrics = origMetrics
		otel.SetMeterProvider(origMP)
	}()

	otel.SetMeterProvider(&failingMeterProvider{
		meter: &failingMeter{failAt: "automatiza.webhooks.events"},
	})

	err := InitMetrics(true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "intentional Int64Counter failure")
}

func TestInitMetrics_FailsOnWorkerTaskDuration(t *testing.T) {
	origMetrics := GlobalMetrics
	origMP := otel.GetMeterProvider()
	defer func() {
		GlobalMetrics = origMetrics
		otel.SetMeterProvider(origMP)
	}()

	otel.SetMeterProvider(&failingMeterProvider{
		meter: &failingMeter{failAt: "automatiza.worker.task.duration"},
	})

	err := InitMetrics(true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "intentional Float64Histogram failure")
}

// =============================================================================
// otel.go — error branches for resource merge, exporters, and runtime.Start
// =============================================================================

// helper to create an OTLP-accepting test server endpoint
func otelTestEndpoint(t *testing.T) string {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	return strings.TrimPrefix(server.URL, "http://")
}

func TestInitOTel_ResourceMergeError_ReturnsError(t *testing.T) {
	origFn := mergeResources
	defer func() { mergeResources = origFn }()

	mergeResources = func(_, _ *resource.Resource) (*resource.Resource, error) {
		return nil, errors.New("injected merge error")
	}

	cfg := OTelConfig{Enabled: true, OTLPEndpoint: "localhost:4318"}
	_, _, err := InitOTel(context.Background(), cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create OTel resource")
	assert.Contains(t, err.Error(), "injected merge error")
}

func TestInitOTel_TraceExporterError_ReturnsError(t *testing.T) {
	origFn := newTraceExporter
	defer func() { newTraceExporter = origFn }()

	newTraceExporter = func(_ context.Context, _ ...otlptracehttp.Option) (*otlptrace.Exporter, error) {
		return nil, errors.New("injected trace exporter error")
	}

	cfg := OTelConfig{Enabled: true, OTLPEndpoint: "localhost:4318"}
	_, _, err := InitOTel(context.Background(), cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create trace exporter")
	assert.Contains(t, err.Error(), "injected trace exporter error")
}

func TestInitOTel_MetricExporterError_ReturnsError(t *testing.T) {
	origFn := newMetricExporter
	defer func() { newMetricExporter = origFn }()

	newMetricExporter = func(_ context.Context, _ ...otlpmetrichttp.Option) (*otlpmetrichttp.Exporter, error) {
		return nil, errors.New("injected metric exporter error")
	}

	endpoint := otelTestEndpoint(t)
	cfg := OTelConfig{Enabled: true, OTLPEndpoint: endpoint}
	_, _, err := InitOTel(context.Background(), cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create metric exporter")
	assert.Contains(t, err.Error(), "injected metric exporter error")
}

func TestInitOTel_LogExporterError_ReturnsError(t *testing.T) {
	origFn := newLogExporter
	defer func() { newLogExporter = origFn }()

	newLogExporter = func(_ context.Context, _ ...otlploghttp.Option) (*otlploghttp.Exporter, error) {
		return nil, errors.New("injected log exporter error")
	}

	endpoint := otelTestEndpoint(t)
	cfg := OTelConfig{Enabled: true, OTLPEndpoint: endpoint}
	_, _, err := InitOTel(context.Background(), cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create log exporter")
	assert.Contains(t, err.Error(), "injected log exporter error")
}

func TestInitOTel_RuntimeStartError_ReturnsError(t *testing.T) {
	origFn := startRuntimeMetrics
	defer func() { startRuntimeMetrics = origFn }()

	startRuntimeMetrics = func(_ ...runtime.Option) error {
		return errors.New("injected runtime start error")
	}

	endpoint := otelTestEndpoint(t)
	cfg := OTelConfig{Enabled: true, OTLPEndpoint: endpoint}
	_, _, err := InitOTel(context.Background(), cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to start runtime metrics")
	assert.Contains(t, err.Error(), "injected runtime start error")
}

// =============================================================================
// sentry.go — default runtimeCaller body (lines 297-300)
// =============================================================================

func TestLogAndCapture_DefaultRuntimeCaller(t *testing.T) {
	logger := zap.NewNop()

	// Use a fresh rate limiter to avoid interference from other tests
	origRL := globalRateLimiter
	defer func() { globalRateLimiter = origRL }()
	globalRateLimiter = &rateLimiter{
		entries:      make(map[string]*rateLimitEntry),
		maxPerWindow: 100,
		window:       60 * time.Second,
	}

	// Do NOT mock runtimeCaller — exercises the default func body
	LogAndCapture(logger, sentry.LevelError, "default caller test", errors.New("test err"), nil)
}

// =============================================================================
// sentry_asynq.go — HasTraceID() branch (lines 81-83)
// =============================================================================

func TestSentryAsynqMiddleware_RealTracerProvider_SetsTraceIdTag(t *testing.T) {
	setupTestSentry(t)

	// Set up a real TracerProvider so spans have actual trace IDs
	tp := sdktrace.NewTracerProvider()
	otel.SetTracerProvider(tp)
	defer func() {
		_ = tp.Shutdown(context.Background())
		otel.SetTracerProvider(tracenoop.NewTracerProvider())
	}()

	inner := asynq.HandlerFunc(func(_ context.Context, _ *asynq.Task) error {
		return nil
	})
	wrapped := SentryAsynqMiddleware("traceid-worker", inner)

	task := asynq.NewTask("test:real_traceid", nil)
	err := wrapped.ProcessTask(context.Background(), task)
	assert.NoError(t, err)
}
