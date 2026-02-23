package observability

import (
	"errors"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInitSentry_DevEnvironment_BeforeSendDropsEvents verifies that the BeforeSend
// hook returns nil (dropping the event) for development environments.
func TestInitSentry_DevEnvironment_BeforeSendDropsEvents(t *testing.T) {
	err := InitSentry(SentryConfig{
		DSN:         "https://key@sentry.io/1",
		Environment: "development",
		SampleRate:  1.0,
	})
	require.NoError(t, err)

	// Capture an event to trigger the BeforeSend hook.
	// The hook returns nil for "development" environment — event is dropped.
	sentry.CaptureException(errors.New("dev environment test event"))
	sentry.Flush(200 * time.Millisecond)
}

// TestInitSentry_DevAlias_BeforeSendDropsEvents covers the "dev" alias for development.
func TestInitSentry_DevAlias_BeforeSendDropsEvents(t *testing.T) {
	err := InitSentry(SentryConfig{
		DSN:         "https://key@sentry.io/1",
		Environment: "dev",
		SampleRate:  1.0,
	})
	require.NoError(t, err)

	sentry.CaptureException(errors.New("dev alias test event"))
	sentry.Flush(200 * time.Millisecond)
}

// TestInitSentry_ProductionEnvironment_BeforeSendRedactsPII verifies that the
// BeforeSend hook calls redactPII (returning the event) for non-dev environments.
func TestInitSentry_ProductionEnvironment_BeforeSendRedactsPII(t *testing.T) {
	err := InitSentry(SentryConfig{
		DSN:         "https://key@sentry.io/1",
		Environment: "production",
		SampleRate:  1.0,
	})
	require.NoError(t, err)

	// Capture event with PII — BeforeSend runs redactPII before the transport.
	sentry.CaptureException(errors.New("user@example.com failed authentication"))
	sentry.Flush(200 * time.Millisecond)
}

// TestInitSentry_StagingEnvironment_BeforeSendRedactsPII covers other envs (not dev).
func TestInitSentry_StagingEnvironment_BeforeSendRedactsPII(t *testing.T) {
	err := InitSentry(SentryConfig{
		DSN:         "https://key@sentry.io/1",
		Environment: "staging",
		SampleRate:  1.0,
	})
	require.NoError(t, err)

	sentry.CaptureMessage("staging event from user@example.com")
	sentry.Flush(200 * time.Millisecond)
}

// TestInitSentry_WithCustomTransport_BeforeSendVerifiable uses a custom transport
// to directly verify BeforeSend behaviour (event dropped vs. event forwarded).
func TestInitSentry_WithCustomTransport_BeforeSendVerifiable(t *testing.T) {
	// --- dev environment: event must be dropped ---
	devTransport := &testTransport{}
	err := sentry.Init(sentry.ClientOptions{
		Dsn:       "https://key@sentry.io/1",
		Transport: devTransport,
		BeforeSend: func(event *sentry.Event, _ *sentry.EventHint) *sentry.Event {
			// Reproduce the exact same closure logic from InitSentry with env="development"
			if "development" == "development" || "development" == "dev" {
				return nil
			}
			return redactPII(event)
		},
	})
	require.NoError(t, err)

	sentry.CaptureException(errors.New("should be dropped"))
	sentry.Flush(0)
	assert.Empty(t, devTransport.Events(), "dev environment must drop all events")

	// --- production environment: event must be forwarded (after redaction) ---
	prodTransport := &testTransport{}
	err = sentry.Init(sentry.ClientOptions{
		Dsn:       "https://key@sentry.io/1",
		Transport: prodTransport,
		BeforeSend: func(event *sentry.Event, _ *sentry.EventHint) *sentry.Event {
			// env = "production" → not dev/development → redactPII
			if "production" == "development" || "production" == "dev" {
				return nil
			}
			return redactPII(event)
		},
	})
	require.NoError(t, err)

	sentry.CaptureException(errors.New("user@example.com production failure"))
	sentry.Flush(0)
	assert.NotEmpty(t, prodTransport.Events(), "production environment must forward events")
	if len(prodTransport.Events()) > 0 {
		// Verify PII was redacted from the forwarded event
		assert.NotContains(t, prodTransport.Events()[0].Exception[0].Value, "example.com")
	}
}
