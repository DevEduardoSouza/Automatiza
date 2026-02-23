package observability

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitOTel_Disabled_ReturnsNoopShutdown(t *testing.T) {
	cfg := OTelConfig{Enabled: false}

	shutdown, lp, err := InitOTel(context.Background(), cfg)
	require.NoError(t, err)
	assert.Nil(t, lp, "log provider must be nil when disabled")
	require.NotNil(t, shutdown)

	// No-op shutdown must not return an error
	err = shutdown(context.Background())
	assert.NoError(t, err)
}

func TestInitOTel_Disabled_ShutdownIsIdempotent(t *testing.T) {
	cfg := OTelConfig{Enabled: false}
	shutdown, _, err := InitOTel(context.Background(), cfg)
	require.NoError(t, err)

	// Calling shutdown multiple times must be safe
	assert.NoError(t, shutdown(context.Background()))
	assert.NoError(t, shutdown(context.Background()))
}

func TestInitOTel_Enabled_InitializesProviders(t *testing.T) {
	// Lightweight HTTP server that accepts OTLP requests without validation
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	endpoint := strings.TrimPrefix(server.URL, "http://")
	cfg := OTelConfig{
		Enabled:        true,
		ServiceName:    "test-observability-service",
		ServiceVersion: "0.0.1-test",
		Environment:    "test",
		OTLPEndpoint:   endpoint,
	}

	ctx := context.Background()
	shutdown, lp, err := InitOTel(ctx, cfg)
	require.NoError(t, err)
	require.NotNil(t, shutdown, "shutdown func must not be nil")
	require.NotNil(t, lp, "log provider must not be nil")

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	assert.NoError(t, shutdown(shutdownCtx))
}

func TestInitOTel_Enabled_ShutdownCancelledContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	endpoint := strings.TrimPrefix(server.URL, "http://")
	cfg := OTelConfig{
		Enabled:        true,
		ServiceName:    "test-cancel-service",
		ServiceVersion: "0.0.1",
		Environment:    "test",
		OTLPEndpoint:   endpoint,
	}

	ctx := context.Background()
	shutdown, _, err := InitOTel(ctx, cfg)
	require.NoError(t, err)

	// Attempt shutdown with an already-cancelled context.
	// OTel providers may return context errors — we verify no panic occurs.
	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	_ = shutdown(cancelledCtx) // error is acceptable; no panic must occur
}

func TestInitOTel_Enabled_WithEmptyServiceName(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	endpoint := strings.TrimPrefix(server.URL, "http://")
	cfg := OTelConfig{
		Enabled:      true,
		OTLPEndpoint: endpoint,
		// ServiceName and ServiceVersion intentionally empty
	}

	ctx := context.Background()
	shutdown, _, err := InitOTel(ctx, cfg)
	require.NoError(t, err)

	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_ = shutdown(shutdownCtx)
}
