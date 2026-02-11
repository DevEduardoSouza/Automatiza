package observability

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/hibiken/asynq"
)

// SentryAsynqMiddleware wraps an Asynq handler with Sentry error capture
// and panic recovery. It clones the Sentry hub per task, captures errors
// with worker/task tags, and marks max-retries-reached as Critical (dlq=true).
func SentryAsynqMiddleware(handlerName string, handler asynq.Handler) asynq.Handler {
	return asynq.HandlerFunc(func(ctx context.Context, task *asynq.Task) (retErr error) {
		hub := sentry.CurrentHub().Clone()
		hub.ConfigureScope(func(scope *sentry.Scope) {
			scope.SetTag("worker", handlerName)
			scope.SetTag("task_type", task.Type())
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
			isLastRetry := retried >= maxRetry-1

			queueName, _ := asynq.GetQueueName(ctx)

			level := sentry.LevelError
			tags := map[string]string{
				"worker":      handlerName,
				"task_type":   task.Type(),
				"retry_count": fmt.Sprintf("%d", retried),
				"queue":       queueName,
			}

			if isLastRetry {
				level = sentry.LevelFatal
				tags["dlq"] = "true"
			}

			hub.WithScope(func(scope *sentry.Scope) {
				scope.SetLevel(level)
				for k, v := range tags {
					scope.SetTag(k, v)
				}
				hub.CaptureException(err)
			})
		}

		return err
	})
}
