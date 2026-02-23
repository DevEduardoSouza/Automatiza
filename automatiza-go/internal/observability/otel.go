package observability

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.39.0"
)

// Test seams for error injection in exporters and resource creation.
var (
	mergeResources      = resource.Merge
	newTraceExporter    = otlptracehttp.New
	newMetricExporter   = otlpmetrichttp.New
	newLogExporter      = otlploghttp.New
	startRuntimeMetrics = runtime.Start
)

// OTelConfig holds configuration for OpenTelemetry initialization.
type OTelConfig struct {
	ServiceName    string
	ServiceVersion string
	Environment    string
	OTLPEndpoint   string
	Enabled        bool
}

// InitOTel initializes OpenTelemetry providers (tracer, meter, logger).
// Returns a shutdown function that must be called on application exit.
// When Enabled is false, returns a no-op shutdown with zero overhead.
func InitOTel(ctx context.Context, cfg OTelConfig) (shutdown func(context.Context) error, logProvider *sdklog.LoggerProvider, err error) {
	if !cfg.Enabled {
		return func(context.Context) error { return nil }, nil, nil
	}

	// Build resource with service metadata
	res, err := mergeResources(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
			semconv.DeploymentEnvironmentName(cfg.Environment),
		),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create OTel resource: %w", err)
	}

	// Set global propagator (W3C TraceContext + Baggage)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// --- Trace Provider ---
	traceExporter, err := newTraceExporter(ctx,
		otlptracehttp.WithEndpoint(cfg.OTLPEndpoint),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create trace exporter: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	// --- Metric Provider ---
	metricExporter, err := newMetricExporter(ctx,
		otlpmetrichttp.WithEndpoint(cfg.OTLPEndpoint),
		otlpmetrichttp.WithInsecure(),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create metric exporter: %w", err)
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(mp)

	// --- Log Provider ---
	logExporter, err := newLogExporter(ctx,
		otlploghttp.WithEndpoint(cfg.OTLPEndpoint),
		otlploghttp.WithInsecure(),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create log exporter: %w", err)
	}

	lp := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExporter)),
		sdklog.WithResource(res),
	)
	global.SetLoggerProvider(lp)

	// Runtime metrics (goroutines, heap, GC)
	if err := startRuntimeMetrics(runtime.WithMinimumReadMemStatsInterval(15 * time.Second)); err != nil {
		return nil, nil, fmt.Errorf("failed to start runtime metrics: %w", err)
	}

	// Composite shutdown
	shutdownFn := func(ctx context.Context) error {
		var errs []error
		if err := tp.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("trace provider shutdown: %w", err))
		}
		if err := mp.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("metric provider shutdown: %w", err))
		}
		if err := lp.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("log provider shutdown: %w", err))
		}
		if len(errs) > 0 {
			return fmt.Errorf("OTel shutdown errors: %v", errs)
		}
		return nil
	}

	return shutdownFn, lp, nil
}
