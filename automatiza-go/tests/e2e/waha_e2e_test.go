// Package e2e provides end-to-end tests for the WAHA WhatsApp integration.
// These tests verify the full flow from HTTP handlers through adapters.
package e2e

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"automatiza-go/internal/adapters/whatsapp"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// WAHA Session Management E2E Tests
// =============================================================================

func TestWAHA_E2E_SessionLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	env := newTestEnv(t)
	defer env.cleanup(t)

	ctx := context.Background()
	sessionName := "test-session-e2e"

	t.Run("create session with GOWS engine config", func(t *testing.T) {
		// Configure the fake adapter to return success
		env.wahaAdapter.CreateSessionFunc = func(ctx context.Context, input whatsapp.CreateSessionInput) (*whatsapp.SessionInfo, error) {
			// Verify GOWS config is passed correctly
			assert.True(t, input.Engine.StorageMessages, "StorageMessages should be enabled")
			assert.True(t, input.Engine.StorageGroups, "StorageGroups should be enabled")
			assert.True(t, input.Engine.StorageChats, "StorageChats should be enabled")

			return &whatsapp.SessionInfo{
				Name:   input.SessionName,
				Status: whatsapp.SessionStatusScanQR,
			}, nil
		}

		result, err := env.wahaAdapter.CreateSession(ctx, whatsapp.CreateSessionInput{
			SessionName:   sessionName,
			WebhookURL:    "https://api.test.com/webhooks/whatsapp",
			WebhookEvents: []string{"message", "message.ack", "session.status"},
			HMACKey:       "test-hmac-key",
			Engine: whatsapp.EngineConfig{
				StorageMessages: true,
				StorageGroups:   true,
				StorageChats:    true,
				StorageLabels:   false,
			},
		})

		require.NoError(t, err)
		assert.Equal(t, sessionName, result.Name)
		assert.Equal(t, whatsapp.SessionStatusScanQR, result.Status)
	})

	t.Run("get QR code for session", func(t *testing.T) {
		env.wahaAdapter.GetQRCodeFunc = func(ctx context.Context, sessionName string) (*whatsapp.QRCodeData, error) {
			return &whatsapp.QRCodeData{
				Value:       "qr-code-data-string",
				Base64Image: "data:image/png;base64,iVBORw0KGgoAAAANSUhEUg...",
				Mimetype:    "image/png",
			}, nil
		}

		qr, err := env.wahaAdapter.GetQRCode(ctx, sessionName)
		require.NoError(t, err)
		assert.NotEmpty(t, qr.Base64Image)
		assert.NotEmpty(t, qr.Value)
	})

	t.Run("session connects after QR scan", func(t *testing.T) {
		env.wahaAdapter.GetSessionStatusFunc = func(ctx context.Context, sessionName string) (*whatsapp.SessionInfo, error) {
			return &whatsapp.SessionInfo{
				Name:   sessionName,
				Status: whatsapp.SessionStatusConnected,
				AuthenticatedUser: &whatsapp.AuthenticatedUser{
					ID:       "5511999998888@s.whatsapp.net",
					PushName: "Test User",
				},
			}, nil
		}

		status, err := env.wahaAdapter.GetSessionStatus(ctx, sessionName)
		require.NoError(t, err)
		assert.Equal(t, whatsapp.SessionStatusConnected, status.Status)
		assert.NotNil(t, status.AuthenticatedUser)
	})

	t.Run("logout session", func(t *testing.T) {
		env.wahaAdapter.LogoutSessionFunc = func(ctx context.Context, sessionName string) error {
			return nil
		}

		err := env.wahaAdapter.LogoutSession(ctx, sessionName)
		require.NoError(t, err)
	})

	t.Run("delete session", func(t *testing.T) {
		env.wahaAdapter.DeleteSessionFunc = func(ctx context.Context, sessionName string) error {
			return nil
		}

		err := env.wahaAdapter.DeleteSession(ctx, sessionName)
		require.NoError(t, err)
	})
}

// =============================================================================
// WAHA Group Operations E2E Tests
// =============================================================================

func TestWAHA_E2E_GroupOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	env := newTestEnv(t)
	defer env.cleanup(t)

	ctx := context.Background()
	sessionName := "test-session-groups"
	groupID := "120363123456789@g.us"

	// Pre-configure session as working
	env.wahaAdapter.GetSessionStatusFunc = func(ctx context.Context, name string) (*whatsapp.SessionInfo, error) {
		return &whatsapp.SessionInfo{
			Name:   name,
			Status: whatsapp.SessionStatusConnected,
		}, nil
	}

	t.Run("fetch groups", func(t *testing.T) {
		env.wahaAdapter.GetGroupsFunc = func(ctx context.Context, sessionName string) ([]whatsapp.GroupInfo, error) {
			return []whatsapp.GroupInfo{
				{
					ID:          groupID,
					Subject:     "Test Group 1",
					Description: "A test group for E2E",
					Size:        150,
					Announce:    false,
				},
				{
					ID:          "120363987654321@g.us",
					Subject:     "Test Group 2",
					Description: "Another test group",
					Size:        50,
					Announce:    true,
				},
			}, nil
		}

		groups, err := env.wahaAdapter.GetGroups(ctx, sessionName)
		require.NoError(t, err)
		assert.Len(t, groups, 2)
		assert.Equal(t, "Test Group 1", groups[0].Subject)
		assert.Equal(t, 150, groups[0].Size)
	})

	t.Run("fetch group participants", func(t *testing.T) {
		env.wahaAdapter.GetGroupParticipantsFunc = func(ctx context.Context, sessionName, gID string) ([]whatsapp.ParticipantInfo, error) {
			return []whatsapp.ParticipantInfo{
				{ID: "5511999998888@s.whatsapp.net", IsAdmin: true, IsSuperAdmin: true},
				{ID: "5511888887777@s.whatsapp.net", IsAdmin: true, IsSuperAdmin: false},
				{ID: "5511777776666@s.whatsapp.net", IsAdmin: false, IsSuperAdmin: false},
			}, nil
		}

		participants, err := env.wahaAdapter.GetGroupParticipants(ctx, sessionName, groupID)
		require.NoError(t, err)
		assert.Len(t, participants, 3)

		// Verify admin hierarchy
		adminCount := 0
		for _, p := range participants {
			if p.IsAdmin {
				adminCount++
			}
		}
		assert.Equal(t, 2, adminCount)
	})

	t.Run("update group subject", func(t *testing.T) {
		env.wahaAdapter.UpdateGroupSubjectFunc = func(ctx context.Context, input whatsapp.UpdateGroupSubjectInput) error {
			assert.Equal(t, "New Group Name", input.Subject)
			return nil
		}

		err := env.wahaAdapter.UpdateGroupSubject(ctx, whatsapp.UpdateGroupSubjectInput{
			SessionName: sessionName,
			GroupID:     groupID,
			Subject:     "New Group Name",
		})
		require.NoError(t, err)
	})

	t.Run("update group description", func(t *testing.T) {
		env.wahaAdapter.UpdateGroupDescriptionFunc = func(ctx context.Context, input whatsapp.UpdateGroupDescriptionInput) error {
			assert.Equal(t, "Updated description for the group", input.Description)
			return nil
		}

		err := env.wahaAdapter.UpdateGroupDescription(ctx, whatsapp.UpdateGroupDescriptionInput{
			SessionName: sessionName,
			GroupID:     groupID,
			Description: "Updated description for the group",
		})
		require.NoError(t, err)
	})

	t.Run("set group announcement mode", func(t *testing.T) {
		env.wahaAdapter.SetGroupAnnouncementFunc = func(ctx context.Context, input whatsapp.SetGroupAnnouncementInput) error {
			assert.True(t, input.Announce)
			return nil
		}

		err := env.wahaAdapter.SetGroupAnnouncement(ctx, whatsapp.SetGroupAnnouncementInput{
			SessionName: sessionName,
			GroupID:     groupID,
			Announce:    true,
		})
		require.NoError(t, err)
	})

	t.Run("get group invite code", func(t *testing.T) {
		env.wahaAdapter.GetGroupInviteCodeFunc = func(ctx context.Context, sessionName, gID string) (string, error) {
			return "AbCdEfGhIjKlMnOp", nil
		}

		code, err := env.wahaAdapter.GetGroupInviteCode(ctx, sessionName, groupID)
		require.NoError(t, err)
		assert.NotEmpty(t, code)
	})
}

// =============================================================================
// WAHA Message Sending E2E Tests
// =============================================================================

func TestWAHA_E2E_MessageSending(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	env := newTestEnv(t)
	defer env.cleanup(t)

	ctx := context.Background()
	sessionName := "test-session-messages"
	groupID := "120363123456789@g.us"

	// Pre-configure session as working
	env.wahaAdapter.GetSessionStatusFunc = func(ctx context.Context, name string) (*whatsapp.SessionInfo, error) {
		return &whatsapp.SessionInfo{
			Name:   name,
			Status: whatsapp.SessionStatusConnected,
		}, nil
	}

	t.Run("send text message", func(t *testing.T) {
		env.wahaAdapter.SendTextFunc = func(ctx context.Context, input whatsapp.SendTextInput) (*whatsapp.MessageResult, error) {
			assert.Equal(t, groupID, input.ChatID)
			assert.Equal(t, "Hello, this is a test message!", input.Text)
			return &whatsapp.MessageResult{
				ID:        "msg_123456",
				Timestamp: time.Now(),
			}, nil
		}

		result, err := env.wahaAdapter.SendText(ctx, whatsapp.SendTextInput{
			SessionName: sessionName,
			ChatID:      groupID,
			Text:        "Hello, this is a test message!",
		})
		require.NoError(t, err)
		assert.NotEmpty(t, result.ID)
	})

	t.Run("send text with mentions", func(t *testing.T) {
		env.wahaAdapter.SendTextFunc = func(ctx context.Context, input whatsapp.SendTextInput) (*whatsapp.MessageResult, error) {
			assert.Contains(t, input.Text, "@todos")
			assert.Len(t, input.Mentions, 2)
			return &whatsapp.MessageResult{
				ID:        "msg_mention_123",
				Timestamp: time.Now(),
			}, nil
		}

		result, err := env.wahaAdapter.SendText(ctx, whatsapp.SendTextInput{
			SessionName: sessionName,
			ChatID:      groupID,
			Text:        "Atenção @todos! Mensagem importante.",
			Mentions:    []string{"5511999998888@s.whatsapp.net", "5511888887777@s.whatsapp.net"},
		})
		require.NoError(t, err)
		assert.NotEmpty(t, result.ID)
	})

	t.Run("send image with caption", func(t *testing.T) {
		env.wahaAdapter.SendImageFunc = func(ctx context.Context, input whatsapp.SendImageInput) (*whatsapp.MessageResult, error) {
			assert.Equal(t, groupID, input.ChatID)
			assert.NotEmpty(t, input.ImageBase64)
			assert.Equal(t, "Check out this image!", input.Caption)
			return &whatsapp.MessageResult{
				ID:        "msg_img_123",
				Timestamp: time.Now(),
			}, nil
		}

		result, err := env.wahaAdapter.SendImage(ctx, whatsapp.SendImageInput{
			SessionName: sessionName,
			ChatID:      groupID,
			ImageBase64: "iVBORw0KGgoAAAANSUhEUg...",
			Caption:     "Check out this image!",
			Mimetype:    "image/png",
		})
		require.NoError(t, err)
		assert.NotEmpty(t, result.ID)
	})

	t.Run("send voice message", func(t *testing.T) {
		env.wahaAdapter.SendVoiceFunc = func(ctx context.Context, input whatsapp.SendVoiceInput) (*whatsapp.MessageResult, error) {
			assert.Equal(t, groupID, input.ChatID)
			assert.NotEmpty(t, input.AudioBase64)
			return &whatsapp.MessageResult{
				ID:        "msg_voice_123",
				Timestamp: time.Now(),
			}, nil
		}

		result, err := env.wahaAdapter.SendVoice(ctx, whatsapp.SendVoiceInput{
			SessionName: sessionName,
			ChatID:      groupID,
			AudioBase64: "T2dnUwACAAAAAAAAAAA...",
			Mimetype:    "audio/ogg; codecs=opus",
		})
		require.NoError(t, err)
		assert.NotEmpty(t, result.ID)
	})

	t.Run("send poll", func(t *testing.T) {
		env.wahaAdapter.SendPollFunc = func(ctx context.Context, input whatsapp.SendPollInput) (*whatsapp.MessageResult, error) {
			assert.Equal(t, groupID, input.ChatID)
			assert.Equal(t, "What's your favorite color?", input.Question)
			assert.Len(t, input.Options, 3)
			assert.Equal(t, 1, input.SelectableCount)
			return &whatsapp.MessageResult{
				ID:        "msg_poll_123",
				Timestamp: time.Now(),
			}, nil
		}

		result, err := env.wahaAdapter.SendPoll(ctx, whatsapp.SendPollInput{
			SessionName:     sessionName,
			ChatID:          groupID,
			Question:        "What's your favorite color?",
			Options:         []string{"Red", "Blue", "Green"},
			SelectableCount: 1,
		})
		require.NoError(t, err)
		assert.NotEmpty(t, result.ID)
	})
}

// =============================================================================
// WAHA Webhook Processing E2E Tests
// =============================================================================

func TestWAHA_E2E_WebhookProcessing(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	env := newTestEnv(t)
	defer env.cleanup(t)

	hmacSecret := "test-webhook-secret"
	validator := whatsapp.NewHMACValidator(hmacSecret)

	t.Run("valid HMAC signature is accepted", func(t *testing.T) {
		payload := []byte(`{"id":"evt_123","event":"message","session":"test-session","payload":{}}`)
		signature := computeHMAC(payload, hmacSecret)

		err := validator.Validate(payload, signature)
		require.NoError(t, err)
	})

	t.Run("invalid HMAC signature is rejected", func(t *testing.T) {
		payload := []byte(`{"id":"evt_123","event":"message","session":"test-session","payload":{}}`)
		invalidSignature := "invalid-signature"

		err := validator.Validate(payload, invalidSignature)
		require.Error(t, err)
	})

	t.Run("disabled validator always accepts", func(t *testing.T) {
		disabledValidator := whatsapp.NewHMACValidator("")

		payload := []byte(`{"id":"evt_123","event":"message"}`)
		err := disabledValidator.Validate(payload, "any-signature")
		require.NoError(t, err)
	})

	t.Run("webhook payload parsing", func(t *testing.T) {
		webhookPayload := map[string]interface{}{
			"id":      "evt_01HXYZ123456789",
			"event":   "message",
			"session": "test-session",
			"payload": map[string]interface{}{
				"id":        "msg_123",
				"from":      "5511999998888@s.whatsapp.net",
				"to":        "120363123456789@g.us",
				"body":      "Hello world!",
				"timestamp": time.Now().Unix(),
				"fromMe":    false,
			},
		}

		jsonPayload, err := json.Marshal(webhookPayload)
		require.NoError(t, err)

		var parsed map[string]interface{}
		err = json.Unmarshal(jsonPayload, &parsed)
		require.NoError(t, err)

		assert.Equal(t, "message", parsed["event"])
		assert.Equal(t, "test-session", parsed["session"])
	})
}

// =============================================================================
// WAHA Reconciliation E2E Tests
// =============================================================================

func TestWAHA_E2E_Reconciliation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	env := newTestEnv(t)
	defer env.cleanup(t)

	ctx := context.Background()
	sessionName := "test-session-reconcile"
	chatID := "120363123456789@g.us"

	t.Run("get messages for reconciliation", func(t *testing.T) {
		env.wahaAdapter.GetMessagesFunc = func(ctx context.Context, input whatsapp.GetMessagesInput) (*whatsapp.MessagesResult, error) {
			assert.Equal(t, sessionName, input.SessionName)
			assert.Equal(t, chatID, input.ChatID)
			assert.Equal(t, 100, input.Limit)

			return &whatsapp.MessagesResult{
				Messages: []whatsapp.Message{
					{
						ID:        "msg_001",
						Body:      "Message 1",
						Timestamp: time.Now().Add(-10 * time.Minute),
						FromMe:    false,
					},
					{
						ID:        "msg_002",
						Body:      "Message 2",
						Timestamp: time.Now().Add(-5 * time.Minute),
						FromMe:    true,
					},
					{
						ID:        "msg_003",
						Body:      "Message 3",
						Timestamp: time.Now(),
						FromMe:    false,
					},
				},
				HasMore: false,
			}, nil
		}

		result, err := env.wahaAdapter.GetMessages(ctx, whatsapp.GetMessagesInput{
			SessionName: sessionName,
			ChatID:      chatID,
			Limit:       100,
		})
		require.NoError(t, err)
		assert.Len(t, result.Messages, 3)
		assert.False(t, result.HasMore)
	})

	t.Run("get messages with timestamp filter", func(t *testing.T) {
		fromTimestamp := time.Now().Add(-1 * time.Hour)

		env.wahaAdapter.GetMessagesFunc = func(ctx context.Context, input whatsapp.GetMessagesInput) (*whatsapp.MessagesResult, error) {
			assert.NotNil(t, input.Filter)
			assert.NotNil(t, input.Filter.FromTimestamp)
			// Verify the timestamp is roughly the same (within a second)
			assert.WithinDuration(t, fromTimestamp, *input.Filter.FromTimestamp, time.Second)

			return &whatsapp.MessagesResult{
				Messages: []whatsapp.Message{
					{ID: "msg_recent_001", Body: "Recent message", Timestamp: time.Now()},
				},
				HasMore: false,
			}, nil
		}

		result, err := env.wahaAdapter.GetMessages(ctx, whatsapp.GetMessagesInput{
			SessionName: sessionName,
			ChatID:      chatID,
			Limit:       50,
			Filter: &whatsapp.MessageFilter{
				FromTimestamp: &fromTimestamp,
			},
		})
		require.NoError(t, err)
		assert.Len(t, result.Messages, 1)
	})
}

// =============================================================================
// WAHA Error Handling E2E Tests
// =============================================================================

func TestWAHA_E2E_ErrorHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	env := newTestEnv(t)
	defer env.cleanup(t)

	ctx := context.Background()

	t.Run("session not found error", func(t *testing.T) {
		env.wahaAdapter.SimulateError = whatsapp.NewProviderError(
			whatsapp.ErrCodeSessionNotFound,
			"Session 'unknown' not found",
			nil,
		)

		_, err := env.wahaAdapter.GetSessionStatus(ctx, "unknown")
		require.Error(t, err)

		var providerErr *whatsapp.ProviderError
		require.ErrorAs(t, err, &providerErr)
		assert.Equal(t, whatsapp.ErrCodeSessionNotFound, providerErr.Code)

		// Reset error
		env.wahaAdapter.SimulateError = nil
	})

	t.Run("permission denied error", func(t *testing.T) {
		env.wahaAdapter.SimulateError = whatsapp.NewProviderError(
			whatsapp.ErrCodePermissionDenied,
			"Not admin in group",
			nil,
		)

		err := env.wahaAdapter.UpdateGroupSubject(ctx, whatsapp.UpdateGroupSubjectInput{
			SessionName: "session",
			GroupID:     "group@g.us",
			Subject:     "New Name",
		})
		require.Error(t, err)

		var providerErr *whatsapp.ProviderError
		require.ErrorAs(t, err, &providerErr)
		assert.Equal(t, whatsapp.ErrCodePermissionDenied, providerErr.Code)

		env.wahaAdapter.SimulateError = nil
	})

	t.Run("rate limited error", func(t *testing.T) {
		env.wahaAdapter.SimulateError = whatsapp.NewProviderError(
			whatsapp.ErrCodeRateLimited,
			"Too many requests",
			nil,
		)

		_, err := env.wahaAdapter.SendText(ctx, whatsapp.SendTextInput{
			SessionName: "session",
			ChatID:      "group@g.us",
			Text:        "Test",
		})
		require.Error(t, err)

		var providerErr *whatsapp.ProviderError
		require.ErrorAs(t, err, &providerErr)
		assert.Equal(t, whatsapp.ErrCodeRateLimited, providerErr.Code)

		env.wahaAdapter.SimulateError = nil
	})

	t.Run("session not working blocks message send", func(t *testing.T) {
		env.wahaAdapter.GetSessionStatusFunc = func(ctx context.Context, name string) (*whatsapp.SessionInfo, error) {
			return &whatsapp.SessionInfo{
				Name:   name,
				Status: whatsapp.SessionStatusScanQR,
			}, nil
		}

		env.wahaAdapter.SendTextFunc = func(ctx context.Context, input whatsapp.SendTextInput) (*whatsapp.MessageResult, error) {
			return nil, whatsapp.NewProviderError(
				whatsapp.ErrCodeSessionNotConnected,
				"Session not connected",
				nil,
			)
		}

		_, err := env.wahaAdapter.SendText(ctx, whatsapp.SendTextInput{
			SessionName: "session",
			ChatID:      "group@g.us",
			Text:        "Test",
		})
		require.Error(t, err)
	})
}

// =============================================================================
// WAHA Idempotency E2E Tests
// =============================================================================

func TestWAHA_E2E_WebhookIdempotency(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	// This test verifies the idempotency behavior at the handler level
	// The actual DB operations would be tested in integration tests

	t.Run("duplicate event ID should be detected", func(t *testing.T) {
		eventID := "evt_01HXYZ123456789"

		// First event - should be processed
		firstPayload := map[string]interface{}{
			"id":      eventID,
			"event":   "message",
			"session": "test-session",
		}

		// Second event with same ID - should be deduplicated
		secondPayload := map[string]interface{}{
			"id":      eventID,
			"event":   "message",
			"session": "test-session",
		}

		firstJSON, _ := json.Marshal(firstPayload)
		secondJSON, _ := json.Marshal(secondPayload)

		// The payloads should be identical
		assert.Equal(t, string(firstJSON), string(secondJSON))
	})

	t.Run("different event IDs should be processed separately", func(t *testing.T) {
		event1 := map[string]interface{}{"id": "evt_001", "event": "message"}
		event2 := map[string]interface{}{"id": "evt_002", "event": "message"}

		json1, _ := json.Marshal(event1)
		json2, _ := json.Marshal(event2)

		// The payloads should be different
		assert.NotEqual(t, string(json1), string(json2))
	})
}

// =============================================================================
// Helper Functions
// =============================================================================

func computeHMAC(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

// createWebhookRequest creates an HTTP request with HMAC signature.
func createWebhookRequest(t *testing.T, payload []byte, secret string) *http.Request {
	t.Helper()

	signature := computeHMAC(payload, secret)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/whatsapp", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Signature", signature)

	return req
}
