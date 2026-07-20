package observability

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/hibiken/asynq"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// SentryAsynqMiddleware wraps an Asynq handler with Sentry error capture,
// panic recovery, and OpenTelemetry distributed tracing. It creates a span
// per task for end-to-end visibility in Jaeger.
func SentryAsynqMiddleware(handlerName string, handler asynq.Handler) asynq.Handler {
	tracer := otel.Tracer("asynq")

	return asynq.HandlerFunc(func(ctx context.Context, task *asynq.Task) (retErr error) {
		taskStart := time.Now()
		defer func() {
			if GlobalMetrics != nil {
				durationMs := float64(time.Since(taskStart).Milliseconds())
				GlobalMetrics.WorkerTaskDuration.Record(ctx, durationMs,
					metric.WithAttributes(
						attribute.String("task_type", task.Type()),
						attribute.String("handler", handlerName),
					))
			}
		}()

		// Extract traceId from payload to create span link to the enqueue trace
		var spanOpts []trace.SpanStartOption
		spanOpts = append(spanOpts,
			trace.WithSpanKind(trace.SpanKindConsumer),
			trace.WithAttributes(
				attribute.String("asynq.task_type", task.Type()),
				attribute.String("asynq.handler", handlerName),
			),
		)

		// Try to extract trace_id from payload for enqueue→worker correlation
		var payload struct {
			TraceID string `json:"trace_id"`
		}
		if err := json.Unmarshal(task.Payload(), &payload); err == nil && payload.TraceID != "" {
			if tid, err := trace.TraceIDFromHex(payload.TraceID); err == nil {
				linkedCtx := trace.NewSpanContext(trace.SpanContextConfig{
					TraceID: tid,
				})
				spanOpts = append(spanOpts, trace.WithLinks(trace.Link{
					SpanContext: linkedCtx,
					Attributes: []attribute.KeyValue{
						attribute.String("link.type", "enqueue"),
					},
				}))
			}
		}

		// Start OTel span for the task
		ctx, span := tracer.Start(ctx, "asynq/"+task.Type(), spanOpts...)
		defer func() {
			if retErr != nil {
				span.SetStatus(codes.Error, retErr.Error())
				span.RecordError(retErr)
			}
			span.End()
		}()

		hub := sentry.CurrentHub().Clone()
		hub.ConfigureScope(func(scope *sentry.Scope) {
			scope.SetTag("worker", handlerName)
			scope.SetTag("task_type", task.Type())
			// Link Sentry scope to OTel trace for correlation
			if span.SpanContext().HasTraceID() {
				scope.SetTag("traceId", span.SpanContext().TraceID().String())
			}
		})
		ctx = sentry.SetHubOnContext(ctx, hub)

		// Panic recovery
		defer func() {
			if r := recover(); r != nil {
				hub.WithScope(func(scope *sentry.Scope) {
					scope.SetLevel(sentry.LevelFatal)
					scope.SetTag("panic", "true")
					scope.SetTag("worker", handlerName)
					scope.SetTag("task_type", task.Type())
					scope.SetExtra("stack", string(debug.Stack()))

					switch val := r.(type) {
					case error:
						hub.CaptureException(val)
					default:
						hub.CaptureMessage(fmt.Sprintf("panic in worker %s: %v", handlerName, val))
					}
				})
				hub.Flush(2 * time.Second)

				retErr = fmt.Errorf("panic in worker %s: %v", handlerName, r)
			}
		}()

		err := handler.ProcessTask(ctx, task)
		if err != nil {
			// Determine retry info from task metadata
			retried, _ := asynq.GetRetryCount(ctx)
			maxRetry, _ := asynq.GetMaxRetry(ctx)
			isLastRetry := shouldCaptureAsynqError(retried, maxRetry)

			queueName, _ := asynq.GetQueueName(ctx)

			span.SetAttributes(
				attribute.Int("asynq.retry_count", retried),
				attribute.String("asynq.queue", queueName),
			)

			level := sentry.LevelError
			tags := map[string]string{
				"worker":      handlerName,
				"task_type":   task.Type(),
				"retry_count": fmt.Sprintf("%d", retried),
				"queue":       queueName,
			}

			// Only report to Sentry on the TERMINAL attempt. Capturing every
			// intermediate retry amplified transient floods (e.g. "mirror fan-out
			// busy", MaxRetry(15)) by up to ~15× and exhausted the monthly error
			// quota. An error that resolves on retry isn't actionable; one that
			// fails for good is still captured here (as Fatal/dlq). The OTel span
			// attributes below stay per-attempt — they don't consume the quota.
			if isLastRetry {
				level = sentry.LevelFatal
				tags["dlq"] = "true"
				span.SetAttributes(attribute.Bool("asynq.dlq", true))

				hub.WithScope(func(scope *sentry.Scope) {
					scope.SetLevel(level)
					for k, v := range tags {
						scope.SetTag(k, v)
					}
					hub.CaptureException(err)
				})
			}
		}

		return err
	})
}

// shouldCaptureAsynqError reports whether an asynq handler error should be sent
// to Sentry: only on the terminal attempt (retried >= maxRetry-1). This caps
// per-retry amplification so a transient error retried N times costs at most one
// Sentry event instead of N. With asynq's defaults (retried=0, maxRetry=0) the
// task has no retries left, so it captures — a one-shot task failing once is
// terminal.
func shouldCaptureAsynqError(retried, maxRetry int) bool {
	return retried >= maxRetry-1
}
