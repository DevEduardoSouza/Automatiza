package observability

import (
	"context"
	"fmt"
	"testing"

	"connectrpc.com/connect"
	"github.com/getsentry/sentry-go"
	"github.com/stretchr/testify/assert"
)

func TestCaptureConnectError_IgnoredCodes(t *testing.T) {
	ignoredCodes := []connect.Code{
		connect.CodeInvalidArgument,
		connect.CodeUnauthenticated,
		connect.CodeNotFound,
		connect.CodeCanceled,
		connect.CodeAlreadyExists,
	}

	for _, code := range ignoredCodes {
		err := connect.NewError(code, fmt.Errorf("test error"))
		// Should not panic
		captureConnectError(context.Background(), "/test.Service/Method", err)
	}
}

func TestCaptureConnectError_WarningCodes(t *testing.T) {
	warningCodes := []connect.Code{
		connect.CodePermissionDenied,
		connect.CodeResourceExhausted,
		connect.CodeFailedPrecondition,
	}

	for _, code := range warningCodes {
		err := connect.NewError(code, fmt.Errorf("test error"))
		captureConnectError(context.Background(), "/test.Service/Method", err)
	}
}

func TestCaptureConnectError_ErrorCodes(t *testing.T) {
	errorCodes := []connect.Code{
		connect.CodeInternal,
		connect.CodeUnknown,
		connect.CodeDataLoss,
		connect.CodeUnavailable,
		connect.CodeDeadlineExceeded,
		connect.CodeUnimplemented,
		connect.CodeAborted,
	}

	for _, code := range errorCodes {
		err := connect.NewError(code, fmt.Errorf("test error"))
		captureConnectError(context.Background(), "/test.Service/Method", err)
	}
}

func TestCapturePanic_Error(t *testing.T) {
	// Should not panic
	capturePanic(context.Background(), "/test.Service/Method", fmt.Errorf("panic error"))
}

func TestCapturePanic_String(t *testing.T) {
	// Should not panic
	capturePanic(context.Background(), "/test.Service/Method", "panic string")
}

func TestGrpcCodeClassification_Coverage(t *testing.T) {
	// Ensure all expected codes are classified
	assert.Equal(t, sentry.Level(""), grpcCodeClassification[connect.CodeInvalidArgument])
	assert.Equal(t, sentry.Level(""), grpcCodeClassification[connect.CodeNotFound])
	assert.Equal(t, sentry.LevelWarning, grpcCodeClassification[connect.CodePermissionDenied])
	assert.Equal(t, sentry.LevelError, grpcCodeClassification[connect.CodeInternal])
	assert.Equal(t, sentry.LevelError, grpcCodeClassification[connect.CodeDataLoss])
}
