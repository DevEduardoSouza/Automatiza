package observability

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/getsentry/sentry-go"
	"connectrpc.com/connect"
)

// grpcCodeClassification maps gRPC error codes to Sentry severity levels.
// - Ignored: InvalidArgument, Unauthenticated, NotFound, Canceled, AlreadyExists
// - Warning: PermissionDenied, ResourceExhausted, FailedPrecondition
// - Error: Internal, Unknown, DataLoss, Unavailable, DeadlineExceeded, Unimplemented, Aborted
var grpcCodeClassification = map[connect.Code]sentry.Level{
	connect.CodeInvalidArgument:  "", // ignored
	connect.CodeUnauthenticated:  "", // ignored
	connect.CodeNotFound:         "", // ignored
	connect.CodeCanceled:         "", // ignored
	connect.CodeAlreadyExists:    "", // ignored
	connect.CodePermissionDenied: sentry.LevelWarning,
	connect.CodeResourceExhausted: sentry.LevelWarning,
	connect.CodeFailedPrecondition: sentry.LevelWarning,
	connect.CodeInternal:          sentry.LevelError,
	connect.CodeUnknown:           sentry.LevelError,
	connect.CodeDataLoss:          sentry.LevelError,
	connect.CodeUnavailable:       sentry.LevelError,
	connect.CodeDeadlineExceeded:  sentry.LevelError,
	connect.CodeUnimplemented:     sentry.LevelError,
	connect.CodeAborted:           sentry.LevelError,
}

// sentryInterceptor captures ConnectRPC errors in Sentry with severity classification.
type sentryInterceptor struct{}

// NewSentryInterceptor creates a new ConnectRPC interceptor that captures errors in Sentry.
// Position in chain: AFTER Tracing (needs traceID), BEFORE Logging (enriches events).
func NewSentryInterceptor() connect.Interceptor {
	return &sentryInterceptor{}
}

func (i *sentryInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		resp, err := next(ctx, req)
		if err != nil {
			captureConnectError(ctx, req.Spec().Procedure, err)
		}
		return resp, err
	}
}

func (i *sentryInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (i *sentryInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		err := next(ctx, conn)
		if err != nil {
			captureConnectError(ctx, conn.Spec().Procedure, err)
		}
		return err
	}
}

// captureConnectError classifies a ConnectRPC error and captures it in Sentry
// with appropriate severity level and tags.
func captureConnectError(ctx context.Context, procedure string, err error) {
	code := connect.CodeOf(err)
	level, known := grpcCodeClassification[code]
	if !known {
		level = sentry.LevelError
	}

	// Ignored codes: don't send to Sentry
	if level == "" {
		return
	}

	hub := sentry.GetHubFromContext(ctx)
	if hub == nil {
		hub = sentry.CurrentHub().Clone()
	}

	hub.WithScope(func(scope *sentry.Scope) {
		scope.SetLevel(level)
		scope.SetTag("procedure", procedure)
		scope.SetTag("grpc_code", code.String())

		// Extract trace_id and user_id from context if available
		if traceID, ok := ctx.Value("traceID").(string); ok && traceID != "" {
			scope.SetTag("trace_id", traceID)
		}

		hub.CaptureException(err)
	})
}

// SentryRecoveryInterceptor wraps ConnectRPC panic recovery with Sentry capture.
// It should replace or wrap the existing RecoveryInterceptor to ensure panics
// are reported to Sentry before being converted to Internal errors.
type SentryRecoveryInterceptor struct{}

// NewSentryRecoveryInterceptor creates a recovery interceptor that captures panics in Sentry.
func NewSentryRecoveryInterceptor() connect.Interceptor {
	return &SentryRecoveryInterceptor{}
}

func (i *SentryRecoveryInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (resp connect.AnyResponse, err error) {
		defer func() {
			if r := recover(); r != nil {
				capturePanic(ctx, req.Spec().Procedure, r)
				err = connect.NewError(connect.CodeInternal, fmt.Errorf("internal server error"))
			}
		}()
		return next(ctx, req)
	}
}

func (i *SentryRecoveryInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (i *SentryRecoveryInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) (err error) {
		defer func() {
			if r := recover(); r != nil {
				capturePanic(ctx, conn.Spec().Procedure, r)
				err = connect.NewError(connect.CodeInternal, fmt.Errorf("internal server error"))
			}
		}()
		return next(ctx, conn)
	}
}

// capturePanic captures a recovered panic value in Sentry with full context.
func capturePanic(ctx context.Context, procedure string, recovered interface{}) {
	hub := sentry.GetHubFromContext(ctx)
	if hub == nil {
		hub = sentry.CurrentHub().Clone()
	}

	hub.WithScope(func(scope *sentry.Scope) {
		scope.SetLevel(sentry.LevelFatal)
		scope.SetTag("procedure", procedure)
		scope.SetTag("panic", "true")
		scope.SetExtra("stack", string(debug.Stack()))

		switch val := recovered.(type) {
		case error:
			hub.CaptureException(val)
		default:
			hub.CaptureMessage(fmt.Sprintf("panic in %s: %v", procedure, val))
		}
	})

	hub.Flush(2 * time.Second)
}
