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

	// Cadência do espelho (Melhoria 7). Rótulos SEMPRE de conjunto fechado —
	// nunca grupo, telefone, URL ou corpo de mensagem.
	MonitorCadenceAdmissions  metric.Int64Counter
	MonitorCadenceEnqueues    metric.Int64Counter
	MonitorCadenceDestination metric.Int64Counter
	MonitorCadenceRecovery    metric.Int64Counter
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

	if m.MonitorCadenceAdmissions, err = meter.Int64Counter("automatiza.monitor.cadence.admissions",
		metric.WithDescription("Mensagens-fonte avaliadas pela cadência do espelho"),
		metric.WithUnit("{message}")); err != nil {
		return err
	}
	if m.MonitorCadenceEnqueues, err = meter.Int64Counter("automatiza.monitor.cadence.enqueues",
		metric.WithDescription("Resultados de enfileiramento do item da fila de cadência"),
		metric.WithUnit("{item}")); err != nil {
		return err
	}
	if m.MonitorCadenceDestination, err = meter.Int64Counter("automatiza.monitor.cadence.destinations",
		metric.WithDescription("Desfechos por grupo de destino na cadência do espelho"),
		metric.WithUnit("{destination}")); err != nil {
		return err
	}
	if m.MonitorCadenceRecovery, err = meter.Int64Counter("automatiza.monitor.cadence.recovery",
		metric.WithDescription("Ações de recuperação de lease da cadência do espelho"),
		metric.WithUnit("{action}")); err != nil {
		return err
	}

	GlobalMetrics = m
	return nil
}

// RecordMonitorCadenceAdmission conta o veredito de UMA mensagem-fonte.
// Só números: nada aqui identifica grupo, usuário ou conteúdo.
func RecordMonitorCadenceAdmission(deferred, suppressed int, redelivery bool) {
	if GlobalMetrics == nil || GlobalMetrics.MonitorCadenceAdmissions == nil {
		return
	}
	ctx := context.Background()
	result := "accepted"
	if redelivery {
		result = "duplicate"
	}
	if deferred > 0 {
		GlobalMetrics.MonitorCadenceAdmissions.Add(ctx, int64(deferred),
			metric.WithAttributes(attribute.String("result", result)))
	}
	if suppressed > 0 {
		GlobalMetrics.MonitorCadenceAdmissions.Add(ctx, int64(suppressed),
			metric.WithAttributes(attribute.String("result", "content_duplicate")))
	}
}

// RecordMonitorCadenceEnqueue conta o desfecho de um enfileiramento.
func RecordMonitorCadenceEnqueue(result string) {
	if GlobalMetrics == nil || GlobalMetrics.MonitorCadenceEnqueues == nil {
		return
	}
	GlobalMetrics.MonitorCadenceEnqueues.Add(context.Background(), 1,
		metric.WithAttributes(attribute.String("result", result)))
}

// RecordMonitorCadenceDestination conta o desfecho de um destino (sent,
// assumed_sent, skipped, failed, canceled, pending).
func RecordMonitorCadenceDestination(outcome string) {
	if GlobalMetrics == nil || GlobalMetrics.MonitorCadenceDestination == nil {
		return
	}
	GlobalMetrics.MonitorCadenceDestination.Add(context.Background(), 1,
		metric.WithAttributes(attribute.String("outcome", outcome)))
}

// RecordMonitorCadenceRecovery conta uma ação de recuperação de lease.
func RecordMonitorCadenceRecovery(action string, count int) {
	if GlobalMetrics == nil || GlobalMetrics.MonitorCadenceRecovery == nil || count <= 0 {
		return
	}
	GlobalMetrics.MonitorCadenceRecovery.Add(context.Background(), int64(count),
		metric.WithAttributes(attribute.String("action", action)))
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
