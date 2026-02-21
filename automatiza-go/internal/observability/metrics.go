package observability

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

// Metrics holds custom business metrics for the application.
type Metrics struct {
	MessagesDispatched metric.Int64Counter
	AutomationRuns     metric.Int64Counter
	WebhookEvents      metric.Int64Counter
	WorkerTaskDuration metric.Float64Histogram
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

	GlobalMetrics = m
	return nil
}
