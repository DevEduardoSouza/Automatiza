package observability

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"connectrpc.com/connect"
	"github.com/getsentry/sentry-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"
)

// mockStreamingHandlerConn implements connect.StreamingHandlerConn for tests.
type mockStreamingHandlerConn struct {
	spec    connect.Spec
	sendErr error
	recvErr error
}

func (m *mockStreamingHandlerConn) Spec() connect.Spec           { return m.spec }
func (m *mockStreamingHandlerConn) Peer() connect.Peer           { return connect.Peer{} }
func (m *mockStreamingHandlerConn) Receive(_ any) error          { return m.recvErr }
func (m *mockStreamingHandlerConn) RequestHeader() http.Header   { return http.Header{} }
func (m *mockStreamingHandlerConn) Send(_ any) error             { return m.sendErr }
func (m *mockStreamingHandlerConn) ResponseHeader() http.Header  { return http.Header{} }
func (m *mockStreamingHandlerConn) ResponseTrailer() http.Header { return http.Header{} }

// mockStreamingClientConn implements connect.StreamingClientConn for tests.
type mockStreamingClientConn struct {
	spec connect.Spec
}

func (m *mockStreamingClientConn) Spec() connect.Spec           { return m.spec }
func (m *mockStreamingClientConn) Peer() connect.Peer           { return connect.Peer{} }
func (m *mockStreamingClientConn) Send(_ any) error             { return nil }
func (m *mockStreamingClientConn) RequestHeader() http.Header   { return http.Header{} }
func (m *mockStreamingClientConn) CloseRequest() error          { return nil }
func (m *mockStreamingClientConn) Receive(_ any) error          { return nil }
func (m *mockStreamingClientConn) ResponseHeader() http.Header  { return http.Header{} }
func (m *mockStreamingClientConn) ResponseTrailer() http.Header { return http.Header{} }
func (m *mockStreamingClientConn) CloseResponse() error         { return nil }

// --- sentryInterceptor ---

func TestNewSentryInterceptor_ReturnsNonNil(t *testing.T) {
	i := NewSentryInterceptor()
	require.NotNil(t, i)
}

func TestSentryInterceptor_WrapUnary_NoError(t *testing.T) {
	i := NewSentryInterceptor()
	expectedResp := connect.NewResponse(&emptypb.Empty{})

	next := connect.UnaryFunc(func(_ context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
		return expectedResp, nil
	})
	wrapped := i.WrapUnary(next)

	req := connect.NewRequest(&emptypb.Empty{})
	resp, err := wrapped(context.Background(), req)
	assert.NoError(t, err)
	assert.Equal(t, expectedResp, resp)
}

func TestSentryInterceptor_WrapUnary_WithIgnoredCode_NoCaptured(t *testing.T) {
	transport := setupTestSentry(t)
	i := NewSentryInterceptor()

	connectErr := connect.NewError(connect.CodeNotFound, errors.New("resource not found"))
	next := connect.UnaryFunc(func(_ context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
		return nil, connectErr
	})
	wrapped := i.WrapUnary(next)

	req := connect.NewRequest(&emptypb.Empty{})
	_, err := wrapped(context.Background(), req)
	assert.Equal(t, connectErr, err)

	sentry.Flush(0)
	assert.Empty(t, transport.Events(), "ignored codes must not produce Sentry events")
}

func TestSentryInterceptor_WrapUnary_WithErrorCode_Captured(t *testing.T) {
	transport := setupTestSentry(t)
	i := NewSentryInterceptor()

	connectErr := connect.NewError(connect.CodeInternal, errors.New("internal failure"))
	next := connect.UnaryFunc(func(_ context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
		return nil, connectErr
	})
	wrapped := i.WrapUnary(next)

	req := connect.NewRequest(&emptypb.Empty{})
	_, err := wrapped(context.Background(), req)
	assert.Equal(t, connectErr, err)

	sentry.Flush(0)
	assert.NotEmpty(t, transport.Events(), "error codes must produce a Sentry event")
}

func TestSentryInterceptor_WrapUnary_WithTraceIDInContext(t *testing.T) {
	transport := setupTestSentry(t)
	i := NewSentryInterceptor()

	connectErr := connect.NewError(connect.CodeInternal, errors.New("internal with trace"))
	next := connect.UnaryFunc(func(_ context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
		return nil, connectErr
	})
	wrapped := i.WrapUnary(next)

	ctx := context.WithValue(context.Background(), "traceID", "abc-trace-123") //nolint:staticcheck
	req := connect.NewRequest(&emptypb.Empty{})
	_, _ = wrapped(ctx, req)

	sentry.Flush(0)
	assert.NotEmpty(t, transport.Events())
}

func TestSentryInterceptor_WrapStreamingClient_Passthrough(t *testing.T) {
	i := NewSentryInterceptor()

	called := false
	next := connect.StreamingClientFunc(func(_ context.Context, _ connect.Spec) connect.StreamingClientConn {
		called = true
		return &mockStreamingClientConn{}
	})
	wrapped := i.WrapStreamingClient(next)
	require.NotNil(t, wrapped)

	wrapped(context.Background(), connect.Spec{})
	assert.True(t, called, "WrapStreamingClient must delegate to next")
}

func TestSentryInterceptor_WrapStreamingHandler_NoError(t *testing.T) {
	i := NewSentryInterceptor()

	next := connect.StreamingHandlerFunc(func(_ context.Context, _ connect.StreamingHandlerConn) error {
		return nil
	})
	wrapped := i.WrapStreamingHandler(next)

	conn := &mockStreamingHandlerConn{spec: connect.Spec{Procedure: "/test.Svc/Stream"}}
	err := wrapped(context.Background(), conn)
	assert.NoError(t, err)
}

func TestSentryInterceptor_WrapStreamingHandler_WithError_Captured(t *testing.T) {
	transport := setupTestSentry(t)
	i := NewSentryInterceptor()

	streamErr := connect.NewError(connect.CodeUnavailable, errors.New("stream unavailable"))
	next := connect.StreamingHandlerFunc(func(_ context.Context, _ connect.StreamingHandlerConn) error {
		return streamErr
	})
	wrapped := i.WrapStreamingHandler(next)

	conn := &mockStreamingHandlerConn{spec: connect.Spec{Procedure: "/test.Svc/Stream"}}
	err := wrapped(context.Background(), conn)
	assert.Equal(t, streamErr, err)

	sentry.Flush(0)
	assert.NotEmpty(t, transport.Events(), "streaming handler error must produce a Sentry event")
}

func TestSentryInterceptor_WrapStreamingHandler_IgnoredCode_NoCaptured(t *testing.T) {
	transport := setupTestSentry(t)
	i := NewSentryInterceptor()

	streamErr := connect.NewError(connect.CodeCanceled, errors.New("canceled"))
	next := connect.StreamingHandlerFunc(func(_ context.Context, _ connect.StreamingHandlerConn) error {
		return streamErr
	})
	wrapped := i.WrapStreamingHandler(next)

	conn := &mockStreamingHandlerConn{spec: connect.Spec{Procedure: "/test.Svc/Stream"}}
	err := wrapped(context.Background(), conn)
	assert.Equal(t, streamErr, err)

	sentry.Flush(0)
	assert.Empty(t, transport.Events(), "ignored codes must not produce Sentry events")
}

// --- SentryRecoveryInterceptor ---

func TestNewSentryRecoveryInterceptor_ReturnsNonNil(t *testing.T) {
	i := NewSentryRecoveryInterceptor()
	require.NotNil(t, i)
}

func TestSentryRecoveryInterceptor_WrapUnary_NoError(t *testing.T) {
	i := NewSentryRecoveryInterceptor()
	expectedResp := connect.NewResponse(&emptypb.Empty{})

	next := connect.UnaryFunc(func(_ context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
		return expectedResp, nil
	})
	wrapped := i.WrapUnary(next)

	req := connect.NewRequest(&emptypb.Empty{})
	resp, err := wrapped(context.Background(), req)
	assert.NoError(t, err)
	assert.Equal(t, expectedResp, resp)
}

func TestSentryRecoveryInterceptor_WrapUnary_PanicWithString_ReturnsInternal(t *testing.T) {
	setupTestSentry(t)
	i := NewSentryRecoveryInterceptor()

	next := connect.UnaryFunc(func(_ context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
		panic("string panic in handler")
	})
	wrapped := i.WrapUnary(next)

	req := connect.NewRequest(&emptypb.Empty{})
	_, err := wrapped(context.Background(), req)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInternal, connect.CodeOf(err))
}

func TestSentryRecoveryInterceptor_WrapUnary_PanicWithError_ReturnsInternal(t *testing.T) {
	setupTestSentry(t)
	i := NewSentryRecoveryInterceptor()

	next := connect.UnaryFunc(func(_ context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
		panic(errors.New("error-type panic in handler"))
	})
	wrapped := i.WrapUnary(next)

	req := connect.NewRequest(&emptypb.Empty{})
	_, err := wrapped(context.Background(), req)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInternal, connect.CodeOf(err))
}

func TestSentryRecoveryInterceptor_WrapStreamingClient_Passthrough(t *testing.T) {
	i := NewSentryRecoveryInterceptor()

	called := false
	next := connect.StreamingClientFunc(func(_ context.Context, _ connect.Spec) connect.StreamingClientConn {
		called = true
		return &mockStreamingClientConn{}
	})
	wrapped := i.WrapStreamingClient(next)
	require.NotNil(t, wrapped)

	wrapped(context.Background(), connect.Spec{})
	assert.True(t, called, "WrapStreamingClient must delegate to next")
}

func TestSentryRecoveryInterceptor_WrapStreamingHandler_NoError(t *testing.T) {
	i := NewSentryRecoveryInterceptor()

	next := connect.StreamingHandlerFunc(func(_ context.Context, _ connect.StreamingHandlerConn) error {
		return nil
	})
	wrapped := i.WrapStreamingHandler(next)

	conn := &mockStreamingHandlerConn{spec: connect.Spec{Procedure: "/test.Svc/Stream"}}
	err := wrapped(context.Background(), conn)
	assert.NoError(t, err)
}

func TestSentryRecoveryInterceptor_WrapStreamingHandler_PanicWithString_ReturnsInternal(t *testing.T) {
	setupTestSentry(t)
	i := NewSentryRecoveryInterceptor()

	next := connect.StreamingHandlerFunc(func(_ context.Context, _ connect.StreamingHandlerConn) error {
		panic("stream panic string")
	})
	wrapped := i.WrapStreamingHandler(next)

	conn := &mockStreamingHandlerConn{spec: connect.Spec{Procedure: "/test.Svc/StreamRecovery"}}
	err := wrapped(context.Background(), conn)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInternal, connect.CodeOf(err))
}

func TestSentryRecoveryInterceptor_WrapStreamingHandler_PanicWithError_ReturnsInternal(t *testing.T) {
	setupTestSentry(t)
	i := NewSentryRecoveryInterceptor()

	next := connect.StreamingHandlerFunc(func(_ context.Context, _ connect.StreamingHandlerConn) error {
		panic(errors.New("stream panic error"))
	})
	wrapped := i.WrapStreamingHandler(next)

	conn := &mockStreamingHandlerConn{spec: connect.Spec{Procedure: "/test.Svc/StreamRecovery"}}
	err := wrapped(context.Background(), conn)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInternal, connect.CodeOf(err))
}

// --- captureConnectError edge cases ---

func TestCaptureConnectError_UnknownCode_DefaultsToError(t *testing.T) {
	transport := setupTestSentry(t)

	// Use a code not in the classification map — falls back to LevelError
	err := connect.NewError(connect.Code(99), errors.New("unknown code error"))
	captureConnectError(context.Background(), "/test.Svc/Unknown", err)

	sentry.Flush(0)
	assert.NotEmpty(t, transport.Events(), "unknown codes must default to error level and be captured")
}

func TestCaptureConnectError_WithHubInContext(t *testing.T) {
	transport := setupTestSentry(t)

	hub := sentry.CurrentHub().Clone()
	ctx := sentry.SetHubOnContext(context.Background(), hub)

	err := connect.NewError(connect.CodeInternal, errors.New("hub ctx error"))
	captureConnectError(ctx, "/test.Svc/WithHub", err)

	sentry.Flush(0)
	assert.NotEmpty(t, transport.Events())
}

func TestCaptureConnectError_WithTraceIDInContext(t *testing.T) {
	transport := setupTestSentry(t)

	ctx := context.WithValue(context.Background(), "traceID", "trace-abc-123") //nolint:staticcheck
	err := connect.NewError(connect.CodeInternal, errors.New("trace ctx error"))
	captureConnectError(ctx, "/test.Svc/TraceCtx", err)

	sentry.Flush(0)
	assert.NotEmpty(t, transport.Events())
	if len(transport.Events()) > 0 {
		assert.Equal(t, "trace-abc-123", transport.Events()[0].Tags["trace_id"])
	}
}

// --- capturePanic edge cases ---

func TestCapturePanic_WithHubInContext(t *testing.T) {
	transport := setupTestSentry(t)

	hub := sentry.CurrentHub().Clone()
	ctx := sentry.SetHubOnContext(context.Background(), hub)

	capturePanic(ctx, "/test.Svc/PanicHub", fmt.Errorf("panic with hub"))

	sentry.Flush(0)
	assert.NotEmpty(t, transport.Events())
}
