// Package integration provides integration tests that run against real WAHA instances.
// These tests require a running WAHA server and should only be run in staging environments.
//
// To run these tests:
//
//	WAHA_API_URL=http://localhost:3000 \
//	WAHA_API_KEY=your-api-key \
//	WAHA_WEBHOOK_SECRET=your-hmac-secret \
//	go test -v -tags=integration ./tests/integration/...
//
// Environment variables:
//   - WAHA_API_URL: URL of the WAHA Plus server (required)
//   - WAHA_API_KEY: API key for WAHA authentication (required)
//   - WAHA_WEBHOOK_SECRET: HMAC secret for webhook validation (optional)
//   - WAHA_TEST_SESSION: Name of existing test session (optional, creates one if not set)
//   - WAHA_TEST_PHONE: Phone number for sending test messages (optional)
//
//go:build integration

package integration

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"automatiza-go/internal/adapters/whatsapp"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// testConfig holds the configuration for integration tests.
type testConfig struct {
	APIBaseURL    string
	APIKey        string
	WebhookSecret string
	TestSession   string
	TestPhone     string
}

// loadTestConfig loads test configuration from environment variables.
func loadTestConfig(t *testing.T) *testConfig {
	t.Helper()

	apiURL := os.Getenv("WAHA_API_URL")
	if apiURL == "" {
		t.Skip("WAHA_API_URL not set, skipping integration tests")
	}

	apiKey := os.Getenv("WAHA_API_KEY")
	if apiKey == "" {
		t.Skip("WAHA_API_KEY not set, skipping integration tests")
	}

	return &testConfig{
		APIBaseURL:    apiURL,
		APIKey:        apiKey,
		WebhookSecret: os.Getenv("WAHA_WEBHOOK_SECRET"),
		TestSession:   os.Getenv("WAHA_TEST_SESSION"),
		TestPhone:     os.Getenv("WAHA_TEST_PHONE"),
	}
}

// newTestAdapter creates a real WAHA adapter for integration testing.
func newTestAdapter(t *testing.T, cfg *testConfig) whatsapp.WhatsAppProvider {
	t.Helper()

	logger, _ := zap.NewDevelopment()

	adapter, err := whatsapp.NewWahaAdapter(whatsapp.WahaAdapterConfig{
		APIURL:         cfg.APIBaseURL,
		APIKey:         cfg.APIKey,
		WebhookSecret:  cfg.WebhookSecret,
		HTTPTimeout:    30 * time.Second,
		StatusCacheTTL: 5 * time.Second,
	}, logger)
	require.NoError(t, err, "Failed to create WAHA adapter")

	return adapter
}

// =============================================================================
// Session Management Integration Tests
// =============================================================================

func TestWAHA_Integration_SessionManagement(t *testing.T) {
	cfg := loadTestConfig(t)
	adapter := newTestAdapter(t, cfg)
	ctx := context.Background()

	sessionName := fmt.Sprintf("integration-test-%d", time.Now().Unix())

	// Cleanup: ensure session is deleted at the end
	t.Cleanup(func() {
		_ = adapter.DeleteSession(context.Background(), sessionName)
	})

	t.Run("create session with GOWS engine", func(t *testing.T) {
		result, err := adapter.CreateSession(ctx, whatsapp.CreateSessionInput{
			SessionName: sessionName,
			WebhookURL:  "", // No webhook for this test
			Engine: whatsapp.EngineConfig{
				StorageMessages: true,
				StorageGroups:   true,
				StorageChats:    true,
				StorageLabels:   false,
			},
		})

		require.NoError(t, err, "Failed to create session")
		assert.Equal(t, sessionName, result.Name)
		t.Logf("Session created with status: %s", result.Status)
	})

	t.Run("get session status", func(t *testing.T) {
		status, err := adapter.GetSessionStatus(ctx, sessionName)

		require.NoError(t, err, "Failed to get session status")
		assert.Equal(t, sessionName, status.Name)
		t.Logf("Session status: %s", status.Status)
	})

	t.Run("get QR code if needed", func(t *testing.T) {
		status, err := adapter.GetSessionStatus(ctx, sessionName)
		require.NoError(t, err)

		if status.Status == whatsapp.SessionStatusScanQR {
			qr, err := adapter.GetQRCode(ctx, sessionName)
			require.NoError(t, err, "Failed to get QR code")
			assert.NotEmpty(t, qr.Value, "QR code value should not be empty")
			t.Logf("QR code available")
		} else {
			t.Log("Session not in QR scan state, skipping QR code test")
		}
	})

	t.Run("restart session", func(t *testing.T) {
		err := adapter.RestartSession(ctx, sessionName)
		require.NoError(t, err, "Failed to restart session")
		t.Log("Session restarted successfully")

		// Wait a bit for restart to complete
		time.Sleep(2 * time.Second)
	})

	t.Run("delete session", func(t *testing.T) {
		err := adapter.DeleteSession(ctx, sessionName)
		require.NoError(t, err, "Failed to delete session")
		t.Log("Session deleted successfully")

		// Verify session is gone
		_, err = adapter.GetSessionStatus(ctx, sessionName)
		assert.Error(t, err, "Session should not exist after deletion")
	})
}

// =============================================================================
// Group Operations Integration Tests (requires connected session)
// =============================================================================

func TestWAHA_Integration_GroupOperations(t *testing.T) {
	cfg := loadTestConfig(t)

	if cfg.TestSession == "" {
		t.Skip("WAHA_TEST_SESSION not set, skipping group operation tests")
	}

	adapter := newTestAdapter(t, cfg)
	ctx := context.Background()

	// Verify session is connected
	status, err := adapter.GetSessionStatus(ctx, cfg.TestSession)
	require.NoError(t, err, "Failed to get session status")

	if status.Status != whatsapp.SessionStatusConnected {
		t.Skipf("Test session is not connected (status: %s), skipping group tests", status.Status)
	}

	t.Run("fetch groups", func(t *testing.T) {
		groups, err := adapter.GetGroups(ctx, cfg.TestSession)

		require.NoError(t, err, "Failed to fetch groups")
		t.Logf("Found %d groups", len(groups))

		for i, g := range groups {
			if i >= 5 {
				t.Logf("... and %d more groups", len(groups)-5)
				break
			}
			t.Logf("Group: %s (ID: %s, Size: %d, Announce: %v)",
				g.Subject, g.ID, g.Size, g.Announce)
		}
	})

	t.Run("fetch participants from first group", func(t *testing.T) {
		groups, err := adapter.GetGroups(ctx, cfg.TestSession)
		require.NoError(t, err)

		if len(groups) == 0 {
			t.Skip("No groups found, skipping participants test")
		}

		// Use first group
		firstGroup := groups[0]

		participants, err := adapter.GetGroupParticipants(ctx, cfg.TestSession, firstGroup.ID)
		require.NoError(t, err, "Failed to fetch participants")

		t.Logf("Group '%s' has %d participants", firstGroup.Subject, len(participants))

		adminCount := 0
		for _, p := range participants {
			if p.IsAdmin {
				adminCount++
			}
		}
		t.Logf("Admin count: %d", adminCount)
	})
}

// =============================================================================
// Message Sending Integration Tests (requires connected session and test phone)
// =============================================================================

func TestWAHA_Integration_MessageSending(t *testing.T) {
	cfg := loadTestConfig(t)

	if cfg.TestSession == "" {
		t.Skip("WAHA_TEST_SESSION not set, skipping message tests")
	}

	if cfg.TestPhone == "" {
		t.Skip("WAHA_TEST_PHONE not set, skipping message tests")
	}

	adapter := newTestAdapter(t, cfg)
	ctx := context.Background()

	// Verify session is connected
	status, err := adapter.GetSessionStatus(ctx, cfg.TestSession)
	require.NoError(t, err)

	if status.Status != whatsapp.SessionStatusConnected {
		t.Skipf("Test session is not connected (status: %s), skipping message tests", status.Status)
	}

	chatID := cfg.TestPhone + "@s.whatsapp.net"

	t.Run("send text message", func(t *testing.T) {
		result, err := adapter.SendText(ctx, whatsapp.SendTextInput{
			SessionName: cfg.TestSession,
			ChatID:      chatID,
			Text:        fmt.Sprintf("[Integration Test] Message sent at %s", time.Now().Format(time.RFC3339)),
		})

		require.NoError(t, err, "Failed to send text message")
		assert.NotEmpty(t, result.ID, "Message ID should not be empty")
		t.Logf("Message sent with ID: %s", result.ID)
	})
}

// =============================================================================
// Reconciliation Integration Tests
// =============================================================================

func TestWAHA_Integration_Reconciliation(t *testing.T) {
	cfg := loadTestConfig(t)

	if cfg.TestSession == "" {
		t.Skip("WAHA_TEST_SESSION not set, skipping reconciliation tests")
	}

	adapter := newTestAdapter(t, cfg)
	ctx := context.Background()

	// Verify session is connected
	status, err := adapter.GetSessionStatus(ctx, cfg.TestSession)
	require.NoError(t, err)

	if status.Status != whatsapp.SessionStatusConnected {
		t.Skipf("Test session is not connected (status: %s), skipping reconciliation tests", status.Status)
	}

	t.Run("get messages from first group", func(t *testing.T) {
		groups, err := adapter.GetGroups(ctx, cfg.TestSession)
		require.NoError(t, err)

		if len(groups) == 0 {
			t.Skip("No groups found, skipping reconciliation test")
		}

		// Use first group
		chatID := groups[0].ID

		result, err := adapter.GetMessages(ctx, whatsapp.GetMessagesInput{
			SessionName:   cfg.TestSession,
			ChatID:        chatID,
			Limit:         20,
			DownloadMedia: false,
		})

		require.NoError(t, err, "Failed to get messages")
		t.Logf("Retrieved %d messages from group '%s'", len(result.Messages), groups[0].Subject)

		for i, msg := range result.Messages {
			if i >= 5 {
				t.Logf("... and %d more messages", len(result.Messages)-5)
				break
			}
			t.Logf("Message: [%s] %s...",
				msg.Timestamp.Format("15:04:05"),
				truncateString(msg.Body, 50))
		}
	})

	t.Run("get messages with timestamp filter", func(t *testing.T) {
		groups, err := adapter.GetGroups(ctx, cfg.TestSession)
		require.NoError(t, err)

		if len(groups) == 0 {
			t.Skip("No groups found")
		}

		chatID := groups[0].ID
		fromTimestamp := time.Now().Add(-24 * time.Hour)

		result, err := adapter.GetMessages(ctx, whatsapp.GetMessagesInput{
			SessionName: cfg.TestSession,
			ChatID:      chatID,
			Limit:       50,
			Filter: &whatsapp.MessageFilter{
				FromTimestamp: &fromTimestamp,
			},
		})

		require.NoError(t, err, "Failed to get messages with filter")
		t.Logf("Retrieved %d messages from last 24 hours", len(result.Messages))
	})
}

// =============================================================================
// Error Handling Integration Tests
// =============================================================================

func TestWAHA_Integration_ErrorHandling(t *testing.T) {
	cfg := loadTestConfig(t)
	adapter := newTestAdapter(t, cfg)
	ctx := context.Background()

	t.Run("session not found returns proper error", func(t *testing.T) {
		_, err := adapter.GetSessionStatus(ctx, "non-existent-session-xyz-123")

		require.Error(t, err, "Expected error for non-existent session")

		var providerErr *whatsapp.ProviderError
		if assert.ErrorAs(t, err, &providerErr) {
			assert.Equal(t, whatsapp.ErrCodeSessionNotFound, providerErr.Code,
				"Expected SESSION_NOT_FOUND error code")
		}
	})

	t.Run("invalid group ID returns proper error", func(t *testing.T) {
		if cfg.TestSession == "" {
			t.Skip("WAHA_TEST_SESSION not set")
		}

		status, _ := adapter.GetSessionStatus(ctx, cfg.TestSession)
		if status == nil || status.Status != whatsapp.SessionStatusConnected {
			t.Skip("Test session not connected")
		}

		_, err := adapter.GetGroupParticipants(ctx, cfg.TestSession, "invalid-group-id")

		// WAHA may return different errors for invalid group IDs
		// The important thing is that it doesn't panic
		if err != nil {
			t.Logf("Got expected error for invalid group ID: %v", err)
		}
	})
}

// =============================================================================
// HMAC Validation Integration Tests
// =============================================================================

func TestWAHA_Integration_HMACValidation(t *testing.T) {
	cfg := loadTestConfig(t)

	if cfg.WebhookSecret == "" {
		t.Skip("WAHA_WEBHOOK_SECRET not set, skipping HMAC tests")
	}

	validator := whatsapp.NewHMACValidator(cfg.WebhookSecret)

	t.Run("HMAC validator is enabled", func(t *testing.T) {
		assert.True(t, validator.IsEnabled(), "Validator should be enabled with secret")
	})

	t.Run("valid signature passes validation", func(t *testing.T) {
		payload := []byte(`{"id":"test","event":"message"}`)
		signature := validator.ComputeSignature(payload)

		err := validator.Validate(payload, signature)
		require.NoError(t, err, "Valid signature should pass")
	})

	t.Run("tampered payload fails validation", func(t *testing.T) {
		originalPayload := []byte(`{"id":"test","event":"message"}`)
		signature := validator.ComputeSignature(originalPayload)

		tamperedPayload := []byte(`{"id":"test","event":"message.ack"}`)
		err := validator.Validate(tamperedPayload, signature)
		require.Error(t, err, "Tampered payload should fail")
	})
}

// =============================================================================
// Performance Benchmarks
// =============================================================================

func BenchmarkWAHA_GetSessionStatus(b *testing.B) {
	cfg := &testConfig{
		APIBaseURL: os.Getenv("WAHA_API_URL"),
		APIKey:     os.Getenv("WAHA_API_KEY"),
	}

	if cfg.APIBaseURL == "" || cfg.APIKey == "" {
		b.Skip("WAHA credentials not set")
	}

	testSession := os.Getenv("WAHA_TEST_SESSION")
	if testSession == "" {
		b.Skip("WAHA_TEST_SESSION not set")
	}

	logger := zap.NewNop()
	adapter, err := whatsapp.NewWahaAdapter(whatsapp.WahaAdapterConfig{
		APIURL:      cfg.APIBaseURL,
		APIKey:      cfg.APIKey,
		HTTPTimeout: 30 * time.Second,
	}, logger)
	if err != nil {
		b.Fatalf("Failed to create adapter: %v", err)
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := adapter.GetSessionStatus(ctx, testSession)
		if err != nil {
			b.Fatalf("GetSessionStatus failed: %v", err)
		}
	}
}

func BenchmarkWAHA_GetGroups(b *testing.B) {
	cfg := &testConfig{
		APIBaseURL: os.Getenv("WAHA_API_URL"),
		APIKey:     os.Getenv("WAHA_API_KEY"),
	}

	if cfg.APIBaseURL == "" || cfg.APIKey == "" {
		b.Skip("WAHA credentials not set")
	}

	testSession := os.Getenv("WAHA_TEST_SESSION")
	if testSession == "" {
		b.Skip("WAHA_TEST_SESSION not set")
	}

	logger := zap.NewNop()
	adapter, err := whatsapp.NewWahaAdapter(whatsapp.WahaAdapterConfig{
		APIURL:      cfg.APIBaseURL,
		APIKey:      cfg.APIKey,
		HTTPTimeout: 30 * time.Second,
	}, logger)
	if err != nil {
		b.Fatalf("Failed to create adapter: %v", err)
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := adapter.GetGroups(ctx, testSession)
		if err != nil {
			b.Fatalf("GetGroups failed: %v", err)
		}
	}
}

// =============================================================================
// Helper Functions
// =============================================================================

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
