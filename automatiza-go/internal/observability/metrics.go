package observability

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Metrics holds custom business metrics for the application.
type Metrics struct {
	MessagesDispatched metric.Int64Counter
	AutomationRuns     metric.Int64Counter
	WebhookEvents      metric.Int64Counter
	WorkerTaskDuration metric.Float64Histogram
	RedirectRequests   metric.Int64Counter
}

// GlobalMetrics is the singleton instance initialized at startup.
var GlobalMetrics *Metrics

// InitMetrics registers custom business metrics with the OTel MeterProvider.
// When enabled is false, GlobalMetrics remains nil and call sites must nil-check.
func InitMetrics(enabled bool) error {
	if !enabled {
		return nil
	}

	meter := otel.Meter("automatiza")
	m := &Metrics{}
	var err error

	if m.MessagesDispatched, err = meter.Int64Counter("automatiza.messages.dispatched",
		metric.WithDescription("Total messages dispatched via WhatsApp"),
		metric.WithUnit("{message}")); err != nil {
		return err
	}
	if m.AutomationRuns, err = meter.Int64Counter("automatiza.automations.runs",
		metric.WithDescription("Total automation executions"),
		metric.WithUnit("{execution}")); err != nil {
		return err
	}
	if m.WebhookEvents, err = meter.Int64Counter("automatiza.webhooks.events",
		metric.WithDescription("Total webhook events processed"),
		metric.WithUnit("{event}")); err != nil {
		return err
	}
	if m.WorkerTaskDuration, err = meter.Float64Histogram("automatiza.worker.task.duration",
		metric.WithDescription("Worker task execution duration"),
		metric.WithUnit("ms")); err != nil {
		return err
	}
	if m.RedirectRequests, err = meter.Int64Counter("automatiza.redirect.requests",
		metric.WithDescription("Public redirect link attempts, by client class and outcome"),
		metric.WithUnit("{request}")); err != nil {
		return err
	}

	GlobalMetrics = m
	return nil
}

// RecordRedirectRequest counts one public redirect attempt.
//
// All three labels come from closed sets defined by the handler — never a
// slug, URL or User-Agent, which would make the series unbounded. No-op when
// metrics are disabled; the caller also emits an equivalent structured log, so
// the OTel flag is never the only record of an attempt.
func RecordRedirectRequest(ctx context.Context, clientClass, outcome string, httpStatus int) {
	if GlobalMetrics == nil || GlobalMetrics.RedirectRequests == nil {
		return
	}
	GlobalMetrics.RedirectRequests.Add(ctx, 1, metric.WithAttributes(
		attribute.String("client_class", clientClass),
		attribute.String("outcome", outcome),
		attribute.Int("http_status", httpStatus),
	))
}
