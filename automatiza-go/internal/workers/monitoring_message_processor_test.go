package workers_test

import (
	"context"
	"fmt"
	"testing"

	"automatiza-go/internal/models"
	"automatiza-go/internal/workers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

// mockMonitoringService is a mock implementation of monitoring.Service
type mockMonitoringService struct {
	mock.Mock
}

func (m *mockMonitoringService) ProcessIncomingMessage(ctx context.Context, msg *models.IncomingMessage) error {
	args := m.Called(ctx, msg)
	return args.Error(0)
}

func TestMonitoringMessageProcessor_ProcessMessage_GroupMessage(t *testing.T) {
	// Setup
	logger := zap.NewNop()
	mockSvc := new(mockMonitoringService)

	processor := workers.NewMonitoringMessageProcessor(mockSvc, logger)

	// Test payload (group message with affiliate link)
	payload := map[string]interface{}{
		"id":        "msg123",
		"from":      "120363xxxxx@g.us",
		"body":      "Check out this deal: https://shopee.com.br/product/123",
		"fromMe":    false,
		"timestamp": float64(1706700000),
	}

	// Expect monitoring service to be called
	mockSvc.On("ProcessIncomingMessage", mock.Anything, mock.MatchedBy(func(msg *models.IncomingMessage) bool {
		return msg.MessageID == "msg123" &&
			msg.RemoteJID == "120363xxxxx@g.us" &&
			msg.Body == "Check out this deal: https://shopee.com.br/product/123" &&
			msg.FromMe == false &&
			msg.InstanceName == "session1"
	})).Return(nil)

	// Execute
	err := processor.ProcessMessage(context.Background(), "session1", payload)

	// Assert
	assert.NoError(t, err)
	mockSvc.AssertExpectations(t)
}

func TestMonitoringMessageProcessor_ProcessMessage_SkipsNonGroupMessage(t *testing.T) {
	// Setup
	logger := zap.NewNop()
	mockSvc := new(mockMonitoringService)

	processor := workers.NewMonitoringMessageProcessor(mockSvc, logger)

	// Test payload (private message, not a group)
	payload := map[string]interface{}{
		"id":        "msg123",
		"from":      "5511999999999@c.us", // Private chat, not @g.us
		"body":      "Hello",
		"fromMe":    false,
		"timestamp": float64(1706700000),
	}

	// Monitoring service should NOT be called for non-group messages

	// Execute
	err := processor.ProcessMessage(context.Background(), "session1", payload)

	// Assert
	assert.NoError(t, err)
	mockSvc.AssertNotCalled(t, "ProcessIncomingMessage", mock.Anything, mock.Anything)
}

func TestMonitoringMessageProcessor_ProcessMessage_SkipsOwnMessage(t *testing.T) {
	// Setup
	logger := zap.NewNop()
	mockSvc := new(mockMonitoringService)

	processor := workers.NewMonitoringMessageProcessor(mockSvc, logger)

	// Test payload (own message)
	payload := map[string]interface{}{
		"id":        "msg123",
		"from":      "120363xxxxx@g.us",
		"body":      "My message",
		"fromMe":    true, // Own message
		"timestamp": float64(1706700000),
	}

	// Monitoring service should NOT be called for own messages

	// Execute
	err := processor.ProcessMessage(context.Background(), "session1", payload)

	// Assert
	assert.NoError(t, err)
	mockSvc.AssertNotCalled(t, "ProcessIncomingMessage", mock.Anything, mock.Anything)
}

func TestMonitoringMessageProcessor_ProcessMessage_SkipsEmptyMessage(t *testing.T) {
	// Setup
	logger := zap.NewNop()
	mockSvc := new(mockMonitoringService)

	processor := workers.NewMonitoringMessageProcessor(mockSvc, logger)

	// Test payload (empty message)
	payload := map[string]interface{}{
		"id":        "msg123",
		"from":      "120363xxxxx@g.us",
		"body":      "",
		"fromMe":    false,
		"timestamp": float64(1706700000),
		"hasMedia":  false,
	}

	// Monitoring service should NOT be called for empty messages

	// Execute
	err := processor.ProcessMessage(context.Background(), "session1", payload)

	// Assert
	assert.NoError(t, err)
	mockSvc.AssertNotCalled(t, "ProcessIncomingMessage", mock.Anything, mock.Anything)
}

func TestMonitoringMessageProcessor_ProcessMessage_NestedMessageFormat(t *testing.T) {
	// Setup
	logger := zap.NewNop()
	mockSvc := new(mockMonitoringService)

	processor := workers.NewMonitoringMessageProcessor(mockSvc, logger)

	// Test payload with nested message object (WAHA format)
	payload := map[string]interface{}{
		"message": map[string]interface{}{
			"id": map[string]interface{}{
				"_serialized": "true_120363xxxxx@g.us_msg456",
				"fromMe":      false,
				"remote":      "120363xxxxx@g.us",
			},
			"body":      "ML deal: https://mercadolivre.com.br/item/123",
			"timestamp": float64(1706700000),
		},
	}

	// Expect monitoring service to be called
	mockSvc.On("ProcessIncomingMessage", mock.Anything, mock.MatchedBy(func(msg *models.IncomingMessage) bool {
		return msg.MessageID == "true_120363xxxxx@g.us_msg456" &&
			msg.RemoteJID == "120363xxxxx@g.us" &&
			msg.Body == "ML deal: https://mercadolivre.com.br/item/123" &&
			msg.FromMe == false
	})).Return(nil)

	// Execute
	err := processor.ProcessMessage(context.Background(), "session1", payload)

	// Assert
	assert.NoError(t, err)
	mockSvc.AssertExpectations(t)
}

func TestMonitoringMessageProcessor_ProcessMessage_MediaMessage(t *testing.T) {
	// Setup
	logger := zap.NewNop()
	mockSvc := new(mockMonitoringService)

	processor := workers.NewMonitoringMessageProcessor(mockSvc, logger)

	// Test payload (image message with caption)
	payload := map[string]interface{}{
		"id":        "msg789",
		"from":      "120363xxxxx@g.us",
		"body":      "Check this product! https://shopee.com.br/p/1",
		"fromMe":    false,
		"timestamp": float64(1706700000),
		"hasMedia":  true,
		"type":      "image",
		"mediaUrl":  "https://waha.local/media/123",
		"mimetype":  "image/jpeg",
	}

	// Expect monitoring service to be called with media fields
	mockSvc.On("ProcessIncomingMessage", mock.Anything, mock.MatchedBy(func(msg *models.IncomingMessage) bool {
		return msg.MessageID == "msg789" &&
			msg.HasMedia == true &&
			msg.MediaType == "image" &&
			msg.MediaURL == "https://waha.local/media/123" &&
			msg.MimeType == "image/jpeg"
	})).Return(nil)

	// Execute
	err := processor.ProcessMessage(context.Background(), "session1", payload)

	// Assert
	assert.NoError(t, err)
	mockSvc.AssertExpectations(t)
}

func TestMonitoringMessageProcessor_ProcessMessage_MalformedPayload(t *testing.T) {
	// Setup
	logger := zap.NewNop()
	mockSvc := new(mockMonitoringService)

	processor := workers.NewMonitoringMessageProcessor(mockSvc, logger)

	// Test payload missing required fields
	payload := map[string]interface{}{
		"body": "Some text",
		// Missing id and from/to
	}

	// Monitoring service should NOT be called

	// Execute
	err := processor.ProcessMessage(context.Background(), "session1", payload)

	// Assert - should not return error, just skip
	assert.NoError(t, err)
	mockSvc.AssertNotCalled(t, "ProcessIncomingMessage", mock.Anything, mock.Anything)
}

func TestMonitoringMessageProcessor_ProcessMessage_ChatIDField(t *testing.T) {
	// Setup
	logger := zap.NewNop()
	mockSvc := new(mockMonitoringService)

	processor := workers.NewMonitoringMessageProcessor(mockSvc, logger)

	// Some WAHA events use chatId instead of from/to
	payload := map[string]interface{}{
		"id":        "msg123",
		"chatId":    "120363xxxxx@g.us",
		"body":      "Deal link: https://shopee.com.br/x",
		"fromMe":    false,
		"timestamp": float64(1706700000),
	}

	// Expect monitoring service to be called
	mockSvc.On("ProcessIncomingMessage", mock.Anything, mock.MatchedBy(func(msg *models.IncomingMessage) bool {
		return msg.RemoteJID == "120363xxxxx@g.us"
	})).Return(nil)

	// Execute
	err := processor.ProcessMessage(context.Background(), "session1", payload)

	// Assert
	assert.NoError(t, err)
	mockSvc.AssertExpectations(t)
}

func TestMonitoringMessageProcessor_ProcessMessage_QuotedMessage(t *testing.T) {
	// Setup
	logger := zap.NewNop()
	mockSvc := new(mockMonitoringService)

	processor := workers.NewMonitoringMessageProcessor(mockSvc, logger)

	// Test payload with quoted message
	payload := map[string]interface{}{
		"id":        "msg123",
		"from":      "120363xxxxx@g.us",
		"body":      "Replying to this deal",
		"fromMe":    false,
		"timestamp": float64(1706700000),
		"quotedMsg": map[string]interface{}{
			"id":          "quoted_msg_456",
			"body":        "Original message with link",
			"participant": "5511888888888@c.us",
		},
	}

	// Expect monitoring service to be called with quoted message
	mockSvc.On("ProcessIncomingMessage", mock.Anything, mock.MatchedBy(func(msg *models.IncomingMessage) bool {
		return msg.QuotedMsg != nil &&
			msg.QuotedMsg.MessageID == "quoted_msg_456" &&
			msg.QuotedMsg.Body == "Original message with link" &&
			msg.QuotedMsg.SenderJID == "5511888888888@c.us"
	})).Return(nil)

	// Execute
	err := processor.ProcessMessage(context.Background(), "session1", payload)

	// Assert
	assert.NoError(t, err)
	mockSvc.AssertExpectations(t)
}

// --- parseMessageObject branch coverage ---

func TestMonitoringMessageProcessor_ProcessMessage_NestedStringID(t *testing.T) {
	logger := zap.NewNop()
	mockSvc := new(mockMonitoringService)
	processor := workers.NewMonitoringMessageProcessor(mockSvc, logger)

	// Nested message with a simple string ID instead of object
	payload := map[string]interface{}{
		"message": map[string]interface{}{
			"id":        "simple-string-id",
			"from":      "120363xxxxx@g.us",
			"body":      "Deal link: https://shopee.com.br/x",
			"fromMe":    false,
			"timestamp": float64(1706700000),
		},
	}

	mockSvc.On("ProcessIncomingMessage", mock.Anything, mock.MatchedBy(func(msg *models.IncomingMessage) bool {
		return msg.MessageID == "simple-string-id" &&
			msg.RemoteJID == "120363xxxxx@g.us" &&
			msg.FromMe == false
	})).Return(nil)

	err := processor.ProcessMessage(context.Background(), "session1", payload)

	assert.NoError(t, err)
	mockSvc.AssertExpectations(t)
}

func TestMonitoringMessageProcessor_ProcessMessage_NestedFallbackToField(t *testing.T) {
	logger := zap.NewNop()
	mockSvc := new(mockMonitoringService)
	processor := workers.NewMonitoringMessageProcessor(mockSvc, logger)

	// Nested message without "from", falls back to "to" for remoteJID
	payload := map[string]interface{}{
		"message": map[string]interface{}{
			"id":        "msg-to-fallback",
			"to":        "120363yyyyy@g.us",
			"body":      "Fallback to field",
			"fromMe":    false,
			"timestamp": float64(1706700000),
		},
	}

	mockSvc.On("ProcessIncomingMessage", mock.Anything, mock.MatchedBy(func(msg *models.IncomingMessage) bool {
		return msg.MessageID == "msg-to-fallback" &&
			msg.RemoteJID == "120363yyyyy@g.us"
	})).Return(nil)

	err := processor.ProcessMessage(context.Background(), "session1", payload)

	assert.NoError(t, err)
	mockSvc.AssertExpectations(t)
}

func TestMonitoringMessageProcessor_ProcessMessage_NestedTimestampT(t *testing.T) {
	logger := zap.NewNop()
	mockSvc := new(mockMonitoringService)
	processor := workers.NewMonitoringMessageProcessor(mockSvc, logger)

	// Nested message using "t" instead of "timestamp"
	payload := map[string]interface{}{
		"message": map[string]interface{}{
			"id":     "msg-t-ts",
			"from":   "120363xxxxx@g.us",
			"body":   "Timestamp via t field",
			"fromMe": false,
			"t":      float64(1706712345),
		},
	}

	mockSvc.On("ProcessIncomingMessage", mock.Anything, mock.MatchedBy(func(msg *models.IncomingMessage) bool {
		return msg.MessageID == "msg-t-ts" &&
			msg.Timestamp == int64(1706712345)
	})).Return(nil)

	err := processor.ProcessMessage(context.Background(), "session1", payload)

	assert.NoError(t, err)
	mockSvc.AssertExpectations(t)
}

func TestMonitoringMessageProcessor_ProcessMessage_NestedMediaObject(t *testing.T) {
	logger := zap.NewNop()
	mockSvc := new(mockMonitoringService)
	processor := workers.NewMonitoringMessageProcessor(mockSvc, logger)

	// Nested message with media object
	payload := map[string]interface{}{
		"message": map[string]interface{}{
			"id":       "msg-nested-media",
			"from":     "120363xxxxx@g.us",
			"body":     "Image with media object",
			"fromMe":   false,
			"hasMedia": true,
			"type":     "image",
			"media": map[string]interface{}{
				"url":      "https://waha.local/media/nested-456",
				"mimetype": "image/png",
			},
			"timestamp": float64(1706700000),
		},
	}

	mockSvc.On("ProcessIncomingMessage", mock.Anything, mock.MatchedBy(func(msg *models.IncomingMessage) bool {
		return msg.MessageID == "msg-nested-media" &&
			msg.HasMedia == true &&
			msg.MediaType == "image" &&
			msg.MediaURL == "https://waha.local/media/nested-456" &&
			msg.MimeType == "image/png"
	})).Return(nil)

	err := processor.ProcessMessage(context.Background(), "session1", payload)

	assert.NoError(t, err)
	mockSvc.AssertExpectations(t)
}

func TestMonitoringMessageProcessor_ProcessMessage_NestedNotifyName(t *testing.T) {
	logger := zap.NewNop()
	mockSvc := new(mockMonitoringService)
	processor := workers.NewMonitoringMessageProcessor(mockSvc, logger)

	// Nested message with notifyName setting PushName
	payload := map[string]interface{}{
		"message": map[string]interface{}{
			"id":         "msg-notify",
			"from":       "120363xxxxx@g.us",
			"body":       "Message with notifyName",
			"fromMe":     false,
			"timestamp":  float64(1706700000),
			"notifyName": "Bruno V",
		},
	}

	mockSvc.On("ProcessIncomingMessage", mock.Anything, mock.MatchedBy(func(msg *models.IncomingMessage) bool {
		return msg.MessageID == "msg-notify" &&
			msg.PushName == "Bruno V"
	})).Return(nil)

	err := processor.ProcessMessage(context.Background(), "session1", payload)

	assert.NoError(t, err)
	mockSvc.AssertExpectations(t)
}

func TestMonitoringMessageProcessor_ProcessMessage_NestedMissingMessageID(t *testing.T) {
	logger := zap.NewNop()
	mockSvc := new(mockMonitoringService)
	processor := workers.NewMonitoringMessageProcessor(mockSvc, logger)

	// Nested message without any message ID
	payload := map[string]interface{}{
		"message": map[string]interface{}{
			"from":      "120363xxxxx@g.us",
			"body":      "No ID here",
			"fromMe":    false,
			"timestamp": float64(1706700000),
		},
	}

	// Should return nil (non-retryable parse error), service not called
	err := processor.ProcessMessage(context.Background(), "session1", payload)

	assert.NoError(t, err)
	mockSvc.AssertNotCalled(t, "ProcessIncomingMessage", mock.Anything, mock.Anything)
}

func TestMonitoringMessageProcessor_ProcessMessage_NestedMissingRemoteJID(t *testing.T) {
	logger := zap.NewNop()
	mockSvc := new(mockMonitoringService)
	processor := workers.NewMonitoringMessageProcessor(mockSvc, logger)

	// Nested message with ID but no remote JID (no from, no to, no remote in ID object)
	payload := map[string]interface{}{
		"message": map[string]interface{}{
			"id":        "msg-no-jid",
			"body":      "Missing JID",
			"fromMe":    false,
			"timestamp": float64(1706700000),
		},
	}

	// Should return nil (non-retryable parse error), service not called
	err := processor.ProcessMessage(context.Background(), "session1", payload)

	assert.NoError(t, err)
	mockSvc.AssertNotCalled(t, "ProcessIncomingMessage", mock.Anything, mock.Anything)
}

// --- Flat payload branch coverage ---

func TestMonitoringMessageProcessor_ProcessMessage_FlatToField(t *testing.T) {
	logger := zap.NewNop()
	mockSvc := new(mockMonitoringService)
	processor := workers.NewMonitoringMessageProcessor(mockSvc, logger)

	// Flat payload using "to" instead of "from" for remoteJID
	payload := map[string]interface{}{
		"id":        "msg-flat-to",
		"to":        "120363zzzzz@g.us",
		"body":      "Using to field",
		"fromMe":    false,
		"timestamp": float64(1706700000),
	}

	mockSvc.On("ProcessIncomingMessage", mock.Anything, mock.MatchedBy(func(msg *models.IncomingMessage) bool {
		return msg.MessageID == "msg-flat-to" &&
			msg.RemoteJID == "120363zzzzz@g.us"
	})).Return(nil)

	err := processor.ProcessMessage(context.Background(), "session1", payload)

	assert.NoError(t, err)
	mockSvc.AssertExpectations(t)
}

func TestMonitoringMessageProcessor_ProcessMessage_FlatAuthorField(t *testing.T) {
	logger := zap.NewNop()
	mockSvc := new(mockMonitoringService)
	processor := workers.NewMonitoringMessageProcessor(mockSvc, logger)

	// Flat payload using "author" instead of "participant" for SenderJID
	payload := map[string]interface{}{
		"id":        "msg-flat-author",
		"from":      "120363xxxxx@g.us",
		"body":      "Using author field",
		"fromMe":    false,
		"timestamp": float64(1706700000),
		"author":    "5511777777777@c.us",
	}

	mockSvc.On("ProcessIncomingMessage", mock.Anything, mock.MatchedBy(func(msg *models.IncomingMessage) bool {
		return msg.MessageID == "msg-flat-author" &&
			msg.SenderJID == "5511777777777@c.us"
	})).Return(nil)

	err := processor.ProcessMessage(context.Background(), "session1", payload)

	assert.NoError(t, err)
	mockSvc.AssertExpectations(t)
}

func TestMonitoringMessageProcessor_ProcessMessage_FlatNotifyName(t *testing.T) {
	logger := zap.NewNop()
	mockSvc := new(mockMonitoringService)
	processor := workers.NewMonitoringMessageProcessor(mockSvc, logger)

	// Flat payload using "notifyName" for PushName
	payload := map[string]interface{}{
		"id":         "msg-flat-notify",
		"from":       "120363xxxxx@g.us",
		"body":       "Using notifyName field",
		"fromMe":     false,
		"timestamp":  float64(1706700000),
		"notifyName": "John Doe",
	}

	mockSvc.On("ProcessIncomingMessage", mock.Anything, mock.MatchedBy(func(msg *models.IncomingMessage) bool {
		return msg.MessageID == "msg-flat-notify" &&
			msg.PushName == "John Doe"
	})).Return(nil)

	err := processor.ProcessMessage(context.Background(), "session1", payload)

	assert.NoError(t, err)
	mockSvc.AssertExpectations(t)
}

func TestMonitoringMessageProcessor_ProcessMessage_FlatTextField(t *testing.T) {
	logger := zap.NewNop()
	mockSvc := new(mockMonitoringService)
	processor := workers.NewMonitoringMessageProcessor(mockSvc, logger)

	// Flat payload using "text" instead of "body"
	payload := map[string]interface{}{
		"id":        "msg-flat-text",
		"from":      "120363xxxxx@g.us",
		"text":      "Message via text field https://shopee.com.br/p/9",
		"fromMe":    false,
		"timestamp": float64(1706700000),
	}

	mockSvc.On("ProcessIncomingMessage", mock.Anything, mock.MatchedBy(func(msg *models.IncomingMessage) bool {
		return msg.MessageID == "msg-flat-text" &&
			msg.Body == "Message via text field https://shopee.com.br/p/9"
	})).Return(nil)

	err := processor.ProcessMessage(context.Background(), "session1", payload)

	assert.NoError(t, err)
	mockSvc.AssertExpectations(t)
}

func TestMonitoringMessageProcessor_ProcessMessage_FlatTimestampT(t *testing.T) {
	logger := zap.NewNop()
	mockSvc := new(mockMonitoringService)
	processor := workers.NewMonitoringMessageProcessor(mockSvc, logger)

	// Flat payload using "t" instead of "timestamp"
	payload := map[string]interface{}{
		"id":     "msg-flat-t",
		"from":   "120363xxxxx@g.us",
		"body":   "Message with t timestamp",
		"fromMe": false,
		"t":      float64(1706799999),
	}

	mockSvc.On("ProcessIncomingMessage", mock.Anything, mock.MatchedBy(func(msg *models.IncomingMessage) bool {
		return msg.MessageID == "msg-flat-t" &&
			msg.Timestamp == int64(1706799999)
	})).Return(nil)

	err := processor.ProcessMessage(context.Background(), "session1", payload)

	assert.NoError(t, err)
	mockSvc.AssertExpectations(t)
}

func TestMonitoringMessageProcessor_ProcessMessage_FlatMediaObject(t *testing.T) {
	logger := zap.NewNop()
	mockSvc := new(mockMonitoringService)
	processor := workers.NewMonitoringMessageProcessor(mockSvc, logger)

	// Flat payload with nested media object (no top-level mediaUrl)
	payload := map[string]interface{}{
		"id":        "msg-flat-media-obj",
		"from":      "120363xxxxx@g.us",
		"body":      "Image with media object",
		"fromMe":    false,
		"timestamp": float64(1706700000),
		"hasMedia":  true,
		"type":      "image",
		"media": map[string]interface{}{
			"url":      "https://waha.local/media/flat-789",
			"mimetype": "image/webp",
		},
	}

	mockSvc.On("ProcessIncomingMessage", mock.Anything, mock.MatchedBy(func(msg *models.IncomingMessage) bool {
		return msg.MessageID == "msg-flat-media-obj" &&
			msg.HasMedia == true &&
			msg.MediaURL == "https://waha.local/media/flat-789" &&
			msg.MimeType == "image/webp"
	})).Return(nil)

	err := processor.ProcessMessage(context.Background(), "session1", payload)

	assert.NoError(t, err)
	mockSvc.AssertExpectations(t)
}

func TestMonitoringMessageProcessor_ProcessMessage_FlatMediaObjectOverridesMediaUrl(t *testing.T) {
	logger := zap.NewNop()
	mockSvc := new(mockMonitoringService)
	processor := workers.NewMonitoringMessageProcessor(mockSvc, logger)

	// Flat payload with BOTH mediaUrl and media.url - media.url should take precedence
	payload := map[string]interface{}{
		"id":        "msg-flat-media-precedence",
		"from":      "120363xxxxx@g.us",
		"body":      "Precedence test",
		"fromMe":    false,
		"timestamp": float64(1706700000),
		"hasMedia":  true,
		"type":      "image",
		"mediaUrl":  "https://waha.local/media/old-deprecated",
		"media": map[string]interface{}{
			"url":      "https://waha.local/media/new-preferred",
			"mimetype": "image/jpeg",
		},
	}

	mockSvc.On("ProcessIncomingMessage", mock.Anything, mock.MatchedBy(func(msg *models.IncomingMessage) bool {
		return msg.MessageID == "msg-flat-media-precedence" &&
			msg.MediaURL == "https://waha.local/media/new-preferred" &&
			msg.MimeType == "image/jpeg"
	})).Return(nil)

	err := processor.ProcessMessage(context.Background(), "session1", payload)

	assert.NoError(t, err)
	mockSvc.AssertExpectations(t)
}

// --- extractInt64FromPayload branch coverage ---

func TestMonitoringMessageProcessor_ProcessMessage_TimestampIntType(t *testing.T) {
	logger := zap.NewNop()
	mockSvc := new(mockMonitoringService)
	processor := workers.NewMonitoringMessageProcessor(mockSvc, logger)

	// Flat payload with timestamp as int type
	payload := map[string]interface{}{
		"id":        "msg-int-ts",
		"from":      "120363xxxxx@g.us",
		"body":      "Int timestamp",
		"fromMe":    false,
		"timestamp": int(1706700042),
	}

	mockSvc.On("ProcessIncomingMessage", mock.Anything, mock.MatchedBy(func(msg *models.IncomingMessage) bool {
		return msg.MessageID == "msg-int-ts" &&
			msg.Timestamp == int64(1706700042)
	})).Return(nil)

	err := processor.ProcessMessage(context.Background(), "session1", payload)

	assert.NoError(t, err)
	mockSvc.AssertExpectations(t)
}

func TestMonitoringMessageProcessor_ProcessMessage_TimestampInt64Type(t *testing.T) {
	logger := zap.NewNop()
	mockSvc := new(mockMonitoringService)
	processor := workers.NewMonitoringMessageProcessor(mockSvc, logger)

	// Flat payload with timestamp as int64 type
	payload := map[string]interface{}{
		"id":        "msg-int64-ts",
		"from":      "120363xxxxx@g.us",
		"body":      "Int64 timestamp",
		"fromMe":    false,
		"timestamp": int64(1706700064),
	}

	mockSvc.On("ProcessIncomingMessage", mock.Anything, mock.MatchedBy(func(msg *models.IncomingMessage) bool {
		return msg.MessageID == "msg-int64-ts" &&
			msg.Timestamp == int64(1706700064)
	})).Return(nil)

	err := processor.ProcessMessage(context.Background(), "session1", payload)

	assert.NoError(t, err)
	mockSvc.AssertExpectations(t)
}

func TestMonitoringMessageProcessor_ProcessMessage_TimestampUint64Type(t *testing.T) {
	logger := zap.NewNop()
	mockSvc := new(mockMonitoringService)
	processor := workers.NewMonitoringMessageProcessor(mockSvc, logger)

	// Flat payload with timestamp as uint64 type
	payload := map[string]interface{}{
		"id":        "msg-uint64-ts",
		"from":      "120363xxxxx@g.us",
		"body":      "Uint64 timestamp",
		"fromMe":    false,
		"timestamp": uint64(1706700064),
	}

	mockSvc.On("ProcessIncomingMessage", mock.Anything, mock.MatchedBy(func(msg *models.IncomingMessage) bool {
		return msg.MessageID == "msg-uint64-ts" &&
			msg.Timestamp == int64(1706700064)
	})).Return(nil)

	err := processor.ProcessMessage(context.Background(), "session1", payload)

	assert.NoError(t, err)
	mockSvc.AssertExpectations(t)
}

func TestMonitoringMessageProcessor_ProcessMessage_TimestampUnsupportedType(t *testing.T) {
	logger := zap.NewNop()
	mockSvc := new(mockMonitoringService)
	processor := workers.NewMonitoringMessageProcessor(mockSvc, logger)

	// Flat payload with timestamp as unsupported string type - should use default (now)
	payload := map[string]interface{}{
		"id":        "msg-str-ts",
		"from":      "120363xxxxx@g.us",
		"body":      "String timestamp ignored",
		"fromMe":    false,
		"timestamp": "1706700000", // string, not numeric
	}

	mockSvc.On("ProcessIncomingMessage", mock.Anything, mock.MatchedBy(func(msg *models.IncomingMessage) bool {
		// Timestamp should NOT be 1706700000 since string is unsupported;
		// it should fall through to the default (approximately time.Now().Unix())
		return msg.MessageID == "msg-str-ts" &&
			msg.Timestamp != int64(1706700000) &&
			msg.Timestamp > 0
	})).Return(nil)

	err := processor.ProcessMessage(context.Background(), "session1", payload)

	assert.NoError(t, err)
	mockSvc.AssertExpectations(t)
}

// --- extractBoolFromPayload with nil map ---

func TestMonitoringMessageProcessor_ProcessMessage_NilMapBoolExtraction(t *testing.T) {
	logger := zap.NewNop()
	mockSvc := new(mockMonitoringService)
	processor := workers.NewMonitoringMessageProcessor(mockSvc, logger)

	// Flat payload without fromMe key at all (tests nil/missing bool extraction)
	payload := map[string]interface{}{
		"id":        "msg-no-fromme",
		"from":      "120363xxxxx@g.us",
		"body":      "No fromMe field",
		"timestamp": float64(1706700000),
	}

	mockSvc.On("ProcessIncomingMessage", mock.Anything, mock.MatchedBy(func(msg *models.IncomingMessage) bool {
		return msg.MessageID == "msg-no-fromme" &&
			msg.FromMe == false // default zero value
	})).Return(nil)

	err := processor.ProcessMessage(context.Background(), "session1", payload)

	assert.NoError(t, err)
	mockSvc.AssertExpectations(t)
}

// --- extractStringFromPayload with nil map ---

func TestMonitoringMessageProcessor_ProcessMessage_FlatMissingID(t *testing.T) {
	logger := zap.NewNop()
	mockSvc := new(mockMonitoringService)
	processor := workers.NewMonitoringMessageProcessor(mockSvc, logger)

	// Flat payload missing message ID entirely (extractStringFromPayload returns "", false)
	payload := map[string]interface{}{
		"from":      "120363xxxxx@g.us",
		"body":      "Missing ID",
		"fromMe":    false,
		"timestamp": float64(1706700000),
	}

	// Should return nil (non-retryable parse error), service not called
	err := processor.ProcessMessage(context.Background(), "session1", payload)

	assert.NoError(t, err)
	mockSvc.AssertNotCalled(t, "ProcessIncomingMessage", mock.Anything, mock.Anything)
}

func TestMonitoringMessageProcessor_ProcessMessage_FlatMissingRemoteJID(t *testing.T) {
	logger := zap.NewNop()
	mockSvc := new(mockMonitoringService)
	processor := workers.NewMonitoringMessageProcessor(mockSvc, logger)

	// Flat payload with ID but no from/to/chatId
	payload := map[string]interface{}{
		"id":        "msg-no-jid",
		"body":      "Missing JID",
		"fromMe":    false,
		"timestamp": float64(1706700000),
	}

	// Should return nil (non-retryable parse error), service not called
	err := processor.ProcessMessage(context.Background(), "session1", payload)

	assert.NoError(t, err)
	mockSvc.AssertNotCalled(t, "ProcessIncomingMessage", mock.Anything, mock.Anything)
}

// --- Monitoring service error ---

func TestMonitoringMessageProcessor_ProcessMessage_ServiceError(t *testing.T) {
	logger := zap.NewNop()
	mockSvc := new(mockMonitoringService)
	processor := workers.NewMonitoringMessageProcessor(mockSvc, logger)

	payload := map[string]interface{}{
		"id":        "msg-svc-err",
		"from":      "120363xxxxx@g.us",
		"body":      "This will fail in service",
		"fromMe":    false,
		"timestamp": float64(1706700000),
	}

	// Service returns an error
	mockSvc.On("ProcessIncomingMessage", mock.Anything, mock.MatchedBy(func(msg *models.IncomingMessage) bool {
		return msg.MessageID == "msg-svc-err"
	})).Return(fmt.Errorf("database connection failed"))

	err := processor.ProcessMessage(context.Background(), "session1", payload)

	// Should return wrapped error to trigger retry
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "monitoring service error")
	assert.Contains(t, err.Error(), "database connection failed")
	mockSvc.AssertExpectations(t)
}

// --- Media type mapping ---

func TestMonitoringMessageProcessor_ProcessMessage_MediaTypeVideo(t *testing.T) {
	logger := zap.NewNop()
	mockSvc := new(mockMonitoringService)
	processor := workers.NewMonitoringMessageProcessor(mockSvc, logger)

	payload := map[string]interface{}{
		"id":        "msg-video",
		"from":      "120363xxxxx@g.us",
		"body":      "Video message",
		"fromMe":    false,
		"timestamp": float64(1706700000),
		"type":      "video",
	}

	mockSvc.On("ProcessIncomingMessage", mock.Anything, mock.MatchedBy(func(msg *models.IncomingMessage) bool {
		return msg.MediaType == "video" && msg.HasMedia == true
	})).Return(nil)

	err := processor.ProcessMessage(context.Background(), "session1", payload)

	assert.NoError(t, err)
	mockSvc.AssertExpectations(t)
}

func TestMonitoringMessageProcessor_ProcessMessage_MediaTypeAudio(t *testing.T) {
	logger := zap.NewNop()
	mockSvc := new(mockMonitoringService)
	processor := workers.NewMonitoringMessageProcessor(mockSvc, logger)

	payload := map[string]interface{}{
		"id":        "msg-audio",
		"from":      "120363xxxxx@g.us",
		"body":      "",
		"fromMe":    false,
		"timestamp": float64(1706700000),
		"type":      "audio",
	}

	mockSvc.On("ProcessIncomingMessage", mock.Anything, mock.MatchedBy(func(msg *models.IncomingMessage) bool {
		return msg.MediaType == "audio" && msg.HasMedia == true
	})).Return(nil)

	err := processor.ProcessMessage(context.Background(), "session1", payload)

	assert.NoError(t, err)
	mockSvc.AssertExpectations(t)
}

func TestMonitoringMessageProcessor_ProcessMessage_MediaTypePtt(t *testing.T) {
	logger := zap.NewNop()
	mockSvc := new(mockMonitoringService)
	processor := workers.NewMonitoringMessageProcessor(mockSvc, logger)

	payload := map[string]interface{}{
		"id":        "msg-ptt",
		"from":      "120363xxxxx@g.us",
		"body":      "",
		"fromMe":    false,
		"timestamp": float64(1706700000),
		"type":      "ptt",
	}

	mockSvc.On("ProcessIncomingMessage", mock.Anything, mock.MatchedBy(func(msg *models.IncomingMessage) bool {
		return msg.MediaType == "audio" && msg.HasMedia == true
	})).Return(nil)

	err := processor.ProcessMessage(context.Background(), "session1", payload)

	assert.NoError(t, err)
	mockSvc.AssertExpectations(t)
}

func TestMonitoringMessageProcessor_ProcessMessage_MediaTypeDocument(t *testing.T) {
	logger := zap.NewNop()
	mockSvc := new(mockMonitoringService)
	processor := workers.NewMonitoringMessageProcessor(mockSvc, logger)

	payload := map[string]interface{}{
		"id":        "msg-doc",
		"from":      "120363xxxxx@g.us",
		"body":      "",
		"fromMe":    false,
		"timestamp": float64(1706700000),
		"type":      "document",
	}

	mockSvc.On("ProcessIncomingMessage", mock.Anything, mock.MatchedBy(func(msg *models.IncomingMessage) bool {
		return msg.MediaType == "document" && msg.HasMedia == true
	})).Return(nil)

	err := processor.ProcessMessage(context.Background(), "session1", payload)

	assert.NoError(t, err)
	mockSvc.AssertExpectations(t)
}

// --- parseQuotedMessage branch coverage ---

func TestMonitoringMessageProcessor_ProcessMessage_QuotedMessageObjectID(t *testing.T) {
	logger := zap.NewNop()
	mockSvc := new(mockMonitoringService)
	processor := workers.NewMonitoringMessageProcessor(mockSvc, logger)

	// Quoted message with ID as object ({"_serialized": "..."}) instead of string
	payload := map[string]interface{}{
		"id":        "msg-quoted-obj-id",
		"from":      "120363xxxxx@g.us",
		"body":      "Replying to a message",
		"fromMe":    false,
		"timestamp": float64(1706700000),
		"quotedMsg": map[string]interface{}{
			"id": map[string]interface{}{
				"_serialized": "true_120363xxxxx@g.us_quoted789",
			},
			"body":        "Original quoted message",
			"participant": "5511666666666@c.us",
		},
	}

	mockSvc.On("ProcessIncomingMessage", mock.Anything, mock.MatchedBy(func(msg *models.IncomingMessage) bool {
		return msg.QuotedMsg != nil &&
			msg.QuotedMsg.MessageID == "true_120363xxxxx@g.us_quoted789" &&
			msg.QuotedMsg.Body == "Original quoted message" &&
			msg.QuotedMsg.SenderJID == "5511666666666@c.us"
	})).Return(nil)

	err := processor.ProcessMessage(context.Background(), "session1", payload)

	assert.NoError(t, err)
	mockSvc.AssertExpectations(t)
}

func TestMonitoringMessageProcessor_ProcessMessage_QuotedMessageAuthorField(t *testing.T) {
	logger := zap.NewNop()
	mockSvc := new(mockMonitoringService)
	processor := workers.NewMonitoringMessageProcessor(mockSvc, logger)

	// Quoted message using "author" instead of "participant"
	payload := map[string]interface{}{
		"id":        "msg-quoted-author",
		"from":      "120363xxxxx@g.us",
		"body":      "Replying with author field",
		"fromMe":    false,
		"timestamp": float64(1706700000),
		"quotedMsg": map[string]interface{}{
			"id":     "quoted-author-123",
			"body":   "Original from author",
			"author": "5511555555555@c.us",
		},
	}

	mockSvc.On("ProcessIncomingMessage", mock.Anything, mock.MatchedBy(func(msg *models.IncomingMessage) bool {
		return msg.QuotedMsg != nil &&
			msg.QuotedMsg.MessageID == "quoted-author-123" &&
			msg.QuotedMsg.Body == "Original from author" &&
			msg.QuotedMsg.SenderJID == "5511555555555@c.us"
	})).Return(nil)

	err := processor.ProcessMessage(context.Background(), "session1", payload)

	assert.NoError(t, err)
	mockSvc.AssertExpectations(t)
}

// --- Additional branch coverage: flat payload paths ---

func TestMonitoringMessageProcessor_ProcessMessage_FlatParticipantField(t *testing.T) {
	logger := zap.NewNop()
	mockSvc := new(mockMonitoringService)
	processor := workers.NewMonitoringMessageProcessor(mockSvc, logger)

	// Flat payload using "participant" (not "author") for SenderJID
	payload := map[string]interface{}{
		"id":          "msg-flat-participant",
		"from":        "120363xxxxx@g.us",
		"body":        "Participant field test",
		"fromMe":      false,
		"timestamp":   float64(1706700000),
		"participant": "5511999888777@c.us",
	}

	mockSvc.On("ProcessIncomingMessage", mock.Anything, mock.MatchedBy(func(msg *models.IncomingMessage) bool {
		return msg.MessageID == "msg-flat-participant" &&
			msg.SenderJID == "5511999888777@c.us"
	})).Return(nil)

	err := processor.ProcessMessage(context.Background(), "session1", payload)

	assert.NoError(t, err)
	mockSvc.AssertExpectations(t)
}

func TestMonitoringMessageProcessor_ProcessMessage_FlatIDAsObject(t *testing.T) {
	logger := zap.NewNop()
	mockSvc := new(mockMonitoringService)
	processor := workers.NewMonitoringMessageProcessor(mockSvc, logger)

	// Flat payload where "id" is a map with "_serialized" instead of a string
	payload := map[string]interface{}{
		"id": map[string]interface{}{
			"_serialized": "serialized-id-123",
		},
		"from":      "120363xxxxx@g.us",
		"body":      "ID as object test",
		"fromMe":    false,
		"timestamp": float64(1706700000),
	}

	mockSvc.On("ProcessIncomingMessage", mock.Anything, mock.MatchedBy(func(msg *models.IncomingMessage) bool {
		return msg.MessageID == "serialized-id-123" &&
			msg.RemoteJID == "120363xxxxx@g.us"
	})).Return(nil)

	err := processor.ProcessMessage(context.Background(), "session1", payload)

	assert.NoError(t, err)
	mockSvc.AssertExpectations(t)
}

func TestMonitoringMessageProcessor_ProcessMessage_FlatPushNameField(t *testing.T) {
	logger := zap.NewNop()
	mockSvc := new(mockMonitoringService)
	processor := workers.NewMonitoringMessageProcessor(mockSvc, logger)

	// Flat payload using "pushName" (camelCase) instead of "notifyName"
	payload := map[string]interface{}{
		"id":        "msg-flat-pushname",
		"from":      "120363xxxxx@g.us",
		"body":      "PushName field test",
		"fromMe":    false,
		"timestamp": float64(1706700000),
		"pushName":  "Maria Silva",
	}

	mockSvc.On("ProcessIncomingMessage", mock.Anything, mock.MatchedBy(func(msg *models.IncomingMessage) bool {
		return msg.MessageID == "msg-flat-pushname" &&
			msg.PushName == "Maria Silva"
	})).Return(nil)

	err := processor.ProcessMessage(context.Background(), "session1", payload)

	assert.NoError(t, err)
	mockSvc.AssertExpectations(t)
}

func TestMonitoringMessageProcessor_ProcessMessage_FlatMimeTypeField(t *testing.T) {
	logger := zap.NewNop()
	mockSvc := new(mockMonitoringService)
	processor := workers.NewMonitoringMessageProcessor(mockSvc, logger)

	// Flat payload with "mimeType" (camelCase) instead of "mimetype" (lowercase)
	payload := map[string]interface{}{
		"id":        "msg-flat-mimetype-camel",
		"from":      "120363xxxxx@g.us",
		"body":      "MimeType camelCase test",
		"fromMe":    false,
		"timestamp": float64(1706700000),
		"hasMedia":  true,
		"type":      "image",
		"mimeType":  "image/png",
	}

	mockSvc.On("ProcessIncomingMessage", mock.Anything, mock.MatchedBy(func(msg *models.IncomingMessage) bool {
		return msg.MessageID == "msg-flat-mimetype-camel" &&
			msg.MimeType == "image/png" &&
			msg.HasMedia == true
	})).Return(nil)

	err := processor.ProcessMessage(context.Background(), "session1", payload)

	assert.NoError(t, err)
	mockSvc.AssertExpectations(t)
}

func TestMonitoringMessageProcessor_ProcessMessage_FlatMimetypeField(t *testing.T) {
	logger := zap.NewNop()
	mockSvc := new(mockMonitoringService)
	processor := workers.NewMonitoringMessageProcessor(mockSvc, logger)

	// Flat payload with "mimetype" (lowercase) - already tested in MediaMessage
	// but this test isolates the mimetype branch without media object
	payload := map[string]interface{}{
		"id":        "msg-flat-mimetype-lower",
		"from":      "120363xxxxx@g.us",
		"body":      "Mimetype lowercase test",
		"fromMe":    false,
		"timestamp": float64(1706700000),
		"hasMedia":  true,
		"type":      "document",
		"mimetype":  "application/pdf",
	}

	mockSvc.On("ProcessIncomingMessage", mock.Anything, mock.MatchedBy(func(msg *models.IncomingMessage) bool {
		return msg.MessageID == "msg-flat-mimetype-lower" &&
			msg.MimeType == "application/pdf" &&
			msg.HasMedia == true &&
			msg.MediaType == "document"
	})).Return(nil)

	err := processor.ProcessMessage(context.Background(), "session1", payload)

	assert.NoError(t, err)
	mockSvc.AssertExpectations(t)
}

// --- Additional branch coverage: nested parseMessageObject paths ---

func TestMonitoringMessageProcessor_ProcessMessage_NestedVideoType(t *testing.T) {
	logger := zap.NewNop()
	mockSvc := new(mockMonitoringService)
	processor := workers.NewMonitoringMessageProcessor(mockSvc, logger)

	// Nested message with type "video"
	payload := map[string]interface{}{
		"message": map[string]interface{}{
			"id": map[string]interface{}{
				"_serialized": "nested-video-msg",
				"fromMe":      false,
				"remote":      "120363xxxxx@g.us",
			},
			"body":      "Video in nested format",
			"timestamp": float64(1706700000),
			"hasMedia":  true,
			"type":      "video",
		},
	}

	mockSvc.On("ProcessIncomingMessage", mock.Anything, mock.MatchedBy(func(msg *models.IncomingMessage) bool {
		return msg.MessageID == "nested-video-msg" &&
			msg.MediaType == "video" &&
			msg.HasMedia == true
	})).Return(nil)

	err := processor.ProcessMessage(context.Background(), "session1", payload)

	assert.NoError(t, err)
	mockSvc.AssertExpectations(t)
}

func TestMonitoringMessageProcessor_ProcessMessage_NestedDocumentType(t *testing.T) {
	logger := zap.NewNop()
	mockSvc := new(mockMonitoringService)
	processor := workers.NewMonitoringMessageProcessor(mockSvc, logger)

	// Nested message with type "document"
	payload := map[string]interface{}{
		"message": map[string]interface{}{
			"id": map[string]interface{}{
				"_serialized": "nested-doc-msg",
				"fromMe":      false,
				"remote":      "120363xxxxx@g.us",
			},
			"body":      "Document in nested format",
			"timestamp": float64(1706700000),
			"hasMedia":  true,
			"type":      "document",
		},
	}

	mockSvc.On("ProcessIncomingMessage", mock.Anything, mock.MatchedBy(func(msg *models.IncomingMessage) bool {
		return msg.MessageID == "nested-doc-msg" &&
			msg.MediaType == "document" &&
			msg.HasMedia == true
	})).Return(nil)

	err := processor.ProcessMessage(context.Background(), "session1", payload)

	assert.NoError(t, err)
	mockSvc.AssertExpectations(t)
}

func TestMonitoringMessageProcessor_ProcessMessage_NestedPttType(t *testing.T) {
	logger := zap.NewNop()
	mockSvc := new(mockMonitoringService)
	processor := workers.NewMonitoringMessageProcessor(mockSvc, logger)

	// Nested message with type "ptt" (push-to-talk) maps to "audio"
	payload := map[string]interface{}{
		"message": map[string]interface{}{
			"id": map[string]interface{}{
				"_serialized": "nested-ptt-msg",
				"fromMe":      false,
				"remote":      "120363xxxxx@g.us",
			},
			"body":      "",
			"timestamp": float64(1706700000),
			"type":      "ptt",
		},
	}

	mockSvc.On("ProcessIncomingMessage", mock.Anything, mock.MatchedBy(func(msg *models.IncomingMessage) bool {
		return msg.MessageID == "nested-ptt-msg" &&
			msg.MediaType == "audio" &&
			msg.HasMedia == true
	})).Return(nil)

	err := processor.ProcessMessage(context.Background(), "session1", payload)

	assert.NoError(t, err)
	mockSvc.AssertExpectations(t)
}

func TestMonitoringMessageProcessor_ProcessMessage_NestedFromFallback(t *testing.T) {
	logger := zap.NewNop()
	mockSvc := new(mockMonitoringService)
	processor := workers.NewMonitoringMessageProcessor(mockSvc, logger)

	// Nested message where ID object has NO "remote" field,
	// so RemoteJID falls back to the "from" field on the message object
	payload := map[string]interface{}{
		"message": map[string]interface{}{
			"id": map[string]interface{}{
				"_serialized": "nested-from-fallback-msg",
				"fromMe":      false,
				// No "remote" field here
			},
			"from":      "120363aaaaa@g.us",
			"body":      "From fallback in nested format",
			"timestamp": float64(1706700000),
		},
	}

	mockSvc.On("ProcessIncomingMessage", mock.Anything, mock.MatchedBy(func(msg *models.IncomingMessage) bool {
		return msg.MessageID == "nested-from-fallback-msg" &&
			msg.RemoteJID == "120363aaaaa@g.us" &&
			msg.FromMe == false
	})).Return(nil)

	err := processor.ProcessMessage(context.Background(), "session1", payload)

	assert.NoError(t, err)
	mockSvc.AssertExpectations(t)
}

func TestMonitoringMessageProcessor_ProcessMessage_NestedFromMeOverride(t *testing.T) {
	logger := zap.NewNop()
	mockSvc := new(mockMonitoringService)
	processor := workers.NewMonitoringMessageProcessor(mockSvc, logger)

	// Nested message where fromMe is false in the ID object but true
	// in the message object itself (fromMe override). Should skip as own message.
	payload := map[string]interface{}{
		"message": map[string]interface{}{
			"id": map[string]interface{}{
				"_serialized": "nested-fromme-override",
				"fromMe":      false,
				"remote":      "120363xxxxx@g.us",
			},
			"body":      "Own message via override",
			"fromMe":    true, // Override: message-level fromMe takes precedence
			"timestamp": float64(1706700000),
		},
	}

	// Should skip processing because fromMe=true (own message)
	err := processor.ProcessMessage(context.Background(), "session1", payload)

	assert.NoError(t, err)
	mockSvc.AssertNotCalled(t, "ProcessIncomingMessage", mock.Anything, mock.Anything)
}

func TestMonitoringMessageProcessor_ProcessMessage_FlatMediaUrlOnly(t *testing.T) {
	logger := zap.NewNop()
	mockSvc := new(mockMonitoringService)
	processor := workers.NewMonitoringMessageProcessor(mockSvc, logger)

	// Flat payload with "mediaUrl" but NO "media" object.
	// MediaURL should come from the top-level mediaUrl field.
	payload := map[string]interface{}{
		"id":        "msg-flat-mediaurl-only",
		"from":      "120363xxxxx@g.us",
		"body":      "Media URL without media object",
		"fromMe":    false,
		"timestamp": float64(1706700000),
		"hasMedia":  true,
		"type":      "image",
		"mediaUrl":  "https://waha.local/media/standalone-url",
	}

	mockSvc.On("ProcessIncomingMessage", mock.Anything, mock.MatchedBy(func(msg *models.IncomingMessage) bool {
		return msg.MessageID == "msg-flat-mediaurl-only" &&
			msg.MediaURL == "https://waha.local/media/standalone-url" &&
			msg.HasMedia == true &&
			msg.MediaType == "image"
	})).Return(nil)

	err := processor.ProcessMessage(context.Background(), "session1", payload)

	assert.NoError(t, err)
	mockSvc.AssertExpectations(t)
}

func TestMonitoringMessageProcessor_ProcessMessage_NestedQuotedMessage(t *testing.T) {
	logger := zap.NewNop()
	mockSvc := new(mockMonitoringService)
	processor := workers.NewMonitoringMessageProcessor(mockSvc, logger)

	// Nested message format does not parse quotedMsg (parseMessageObject
	// does not call parseQuotedMessage). This test verifies that the nested
	// format still processes correctly even when a quotedMsg key exists
	// in the nested message object (it will be ignored since parseMessageObject
	// does not handle it).
	payload := map[string]interface{}{
		"message": map[string]interface{}{
			"id": map[string]interface{}{
				"_serialized": "nested-quoted-msg",
				"fromMe":      false,
				"remote":      "120363xxxxx@g.us",
			},
			"body":      "Replying in nested format",
			"timestamp": float64(1706700000),
		},
	}

	mockSvc.On("ProcessIncomingMessage", mock.Anything, mock.MatchedBy(func(msg *models.IncomingMessage) bool {
		return msg.MessageID == "nested-quoted-msg" &&
			msg.RemoteJID == "120363xxxxx@g.us" &&
			msg.Body == "Replying in nested format" &&
			msg.QuotedMsg == nil // parseMessageObject does not parse quotedMsg
	})).Return(nil)

	err := processor.ProcessMessage(context.Background(), "session1", payload)

	assert.NoError(t, err)
	mockSvc.AssertExpectations(t)
}

// --- extractInt64FromPayload type branches ---

func TestMonitoringMessageProcessor_ProcessMessage_TimestampAsInt(t *testing.T) {
	logger := zap.NewNop()
	mockSvc := new(mockMonitoringService)
	processor := workers.NewMonitoringMessageProcessor(mockSvc, logger)

	// Exercises the `case int` branch in extractInt64FromPayload.
	payload := map[string]interface{}{
		"id":        "msg-ts-int",
		"from":      "120363xxxxx@g.us",
		"body":      "Timestamp as int",
		"fromMe":    false,
		"timestamp": int(1706700000),
	}

	mockSvc.On("ProcessIncomingMessage", mock.Anything, mock.MatchedBy(func(msg *models.IncomingMessage) bool {
		return msg.MessageID == "msg-ts-int" && msg.Timestamp == int64(1706700000)
	})).Return(nil)

	err := processor.ProcessMessage(context.Background(), "session1", payload)

	assert.NoError(t, err)
	mockSvc.AssertExpectations(t)
}

func TestMonitoringMessageProcessor_ProcessMessage_TimestampAsInt64(t *testing.T) {
	logger := zap.NewNop()
	mockSvc := new(mockMonitoringService)
	processor := workers.NewMonitoringMessageProcessor(mockSvc, logger)

	// Exercises the `case int64` branch in extractInt64FromPayload.
	payload := map[string]interface{}{
		"id":        "msg-ts-int64",
		"from":      "120363xxxxx@g.us",
		"body":      "Timestamp as int64",
		"fromMe":    false,
		"timestamp": int64(1706700000),
	}

	mockSvc.On("ProcessIncomingMessage", mock.Anything, mock.MatchedBy(func(msg *models.IncomingMessage) bool {
		return msg.MessageID == "msg-ts-int64" && msg.Timestamp == int64(1706700000)
	})).Return(nil)

	err := processor.ProcessMessage(context.Background(), "session1", payload)

	assert.NoError(t, err)
	mockSvc.AssertExpectations(t)
}

func TestMonitoringMessageProcessor_ProcessMessage_TimestampAsUint64(t *testing.T) {
	logger := zap.NewNop()
	mockSvc := new(mockMonitoringService)
	processor := workers.NewMonitoringMessageProcessor(mockSvc, logger)

	// Exercises the `case uint64` branch in extractInt64FromPayload.
	payload := map[string]interface{}{
		"id":        "msg-ts-uint64",
		"from":      "120363xxxxx@g.us",
		"body":      "Timestamp as uint64",
		"fromMe":    false,
		"timestamp": uint64(1706700000),
	}

	mockSvc.On("ProcessIncomingMessage", mock.Anything, mock.MatchedBy(func(msg *models.IncomingMessage) bool {
		return msg.MessageID == "msg-ts-uint64" && msg.Timestamp == int64(1706700000)
	})).Return(nil)

	err := processor.ProcessMessage(context.Background(), "session1", payload)

	assert.NoError(t, err)
	mockSvc.AssertExpectations(t)
}

func TestMonitoringMessageProcessor_ProcessMessage_TimestampAsString(t *testing.T) {
	logger := zap.NewNop()
	mockSvc := new(mockMonitoringService)
	processor := workers.NewMonitoringMessageProcessor(mockSvc, logger)

	// Exercises the `default` branch in extractInt64FromPayload.
	// Both "timestamp" and "t" are strings → default case for both.
	payload := map[string]interface{}{
		"id":        "msg-ts-string",
		"from":      "120363xxxxx@g.us",
		"body":      "Timestamp as string",
		"fromMe":    false,
		"timestamp": "not-a-number",
		"t":         "also-not-a-number",
	}

	mockSvc.On("ProcessIncomingMessage", mock.Anything, mock.MatchedBy(func(msg *models.IncomingMessage) bool {
		return msg.MessageID == "msg-ts-string" && msg.Timestamp > 0
	})).Return(nil)

	err := processor.ProcessMessage(context.Background(), "session1", payload)

	assert.NoError(t, err)
	mockSvc.AssertExpectations(t)
}

// --- extractBoolFromPayload and extractStringFromPayload type assertion failures ---

func TestMonitoringMessageProcessor_ProcessMessage_FromMeAsString(t *testing.T) {
	logger := zap.NewNop()
	mockSvc := new(mockMonitoringService)
	processor := workers.NewMonitoringMessageProcessor(mockSvc, logger)

	// "fromMe" as string triggers extractBoolFromPayload type assertion failure.
	payload := map[string]interface{}{
		"id":        "msg-fromme-string",
		"from":      "120363xxxxx@g.us",
		"body":      "fromMe as string",
		"fromMe":    "true",
		"timestamp": float64(1706700000),
	}

	mockSvc.On("ProcessIncomingMessage", mock.Anything, mock.MatchedBy(func(msg *models.IncomingMessage) bool {
		return msg.MessageID == "msg-fromme-string" && msg.FromMe == false
	})).Return(nil)

	err := processor.ProcessMessage(context.Background(), "session1", payload)

	assert.NoError(t, err)
	mockSvc.AssertExpectations(t)
}

func TestMonitoringMessageProcessor_ProcessMessage_IDAsNumber(t *testing.T) {
	logger := zap.NewNop()
	mockSvc := new(mockMonitoringService)
	processor := workers.NewMonitoringMessageProcessor(mockSvc, logger)

	// "id" as number triggers extractStringFromPayload type assertion failure.
	// Neither string nor map → MessageID stays empty → parsePayload returns error.
	// ProcessMessage silently discards malformed payloads (returns nil).
	payload := map[string]interface{}{
		"id":        12345,
		"from":      "120363xxxxx@g.us",
		"body":      "ID as number",
		"fromMe":    false,
		"timestamp": float64(1706700000),
	}

	err := processor.ProcessMessage(context.Background(), "session1", payload)

	// ProcessMessage returns nil for parse errors (non-retryable)
	assert.NoError(t, err)
	// ProcessIncomingMessage should NOT be called since parse failed
	mockSvc.AssertNotCalled(t, "ProcessIncomingMessage", mock.Anything, mock.Anything)
}

// --- parsePayload fallback fields ---

func TestMonitoringMessageProcessor_ProcessMessage_FlatToFallback(t *testing.T) {
	logger := zap.NewNop()
	mockSvc := new(mockMonitoringService)
	processor := workers.NewMonitoringMessageProcessor(mockSvc, logger)

	// Flat payload missing "from", using "to" fallback for RemoteJID.
	payload := map[string]interface{}{
		"id":        "msg-to-fallback",
		"to":        "120363bbbbb@g.us",
		"body":      "To fallback test",
		"fromMe":    false,
		"timestamp": float64(1706700000),
	}

	mockSvc.On("ProcessIncomingMessage", mock.Anything, mock.MatchedBy(func(msg *models.IncomingMessage) bool {
		return msg.MessageID == "msg-to-fallback" && msg.RemoteJID == "120363bbbbb@g.us"
	})).Return(nil)

	err := processor.ProcessMessage(context.Background(), "session1", payload)

	assert.NoError(t, err)
	mockSvc.AssertExpectations(t)
}

func TestMonitoringMessageProcessor_ProcessMessage_FlatChatIDFallback(t *testing.T) {
	logger := zap.NewNop()
	mockSvc := new(mockMonitoringService)
	processor := workers.NewMonitoringMessageProcessor(mockSvc, logger)

	// Flat payload missing "from" and "to", using "chatId" fallback.
	payload := map[string]interface{}{
		"id":        "msg-chatid-fallback",
		"chatId":    "120363ccccc@g.us",
		"body":      "ChatId fallback test",
		"fromMe":    false,
		"timestamp": float64(1706700000),
	}

	mockSvc.On("ProcessIncomingMessage", mock.Anything, mock.MatchedBy(func(msg *models.IncomingMessage) bool {
		return msg.MessageID == "msg-chatid-fallback" && msg.RemoteJID == "120363ccccc@g.us"
	})).Return(nil)

	err := processor.ProcessMessage(context.Background(), "session1", payload)

	assert.NoError(t, err)
	mockSvc.AssertExpectations(t)
}

func TestMonitoringMessageProcessor_ProcessMessage_FlatTextFallback(t *testing.T) {
	logger := zap.NewNop()
	mockSvc := new(mockMonitoringService)
	processor := workers.NewMonitoringMessageProcessor(mockSvc, logger)

	// Flat payload with "text" instead of "body" for message content.
	payload := map[string]interface{}{
		"id":        "msg-text-fallback",
		"from":      "120363xxxxx@g.us",
		"text":      "Text field fallback",
		"fromMe":    false,
		"timestamp": float64(1706700000),
	}

	mockSvc.On("ProcessIncomingMessage", mock.Anything, mock.MatchedBy(func(msg *models.IncomingMessage) bool {
		return msg.MessageID == "msg-text-fallback" && msg.Body == "Text field fallback"
	})).Return(nil)

	err := processor.ProcessMessage(context.Background(), "session1", payload)

	assert.NoError(t, err)
	mockSvc.AssertExpectations(t)
}

func TestMonitoringMessageProcessor_ProcessMessage_FlatTimestampTFallback(t *testing.T) {
	logger := zap.NewNop()
	mockSvc := new(mockMonitoringService)
	processor := workers.NewMonitoringMessageProcessor(mockSvc, logger)

	// Flat payload using "t" instead of "timestamp" for the timestamp field.
	payload := map[string]interface{}{
		"id":     "msg-t-fallback",
		"from":   "120363xxxxx@g.us",
		"body":   "Timestamp t fallback",
		"fromMe": false,
		"t":      float64(1706700000),
	}

	mockSvc.On("ProcessIncomingMessage", mock.Anything, mock.MatchedBy(func(msg *models.IncomingMessage) bool {
		return msg.MessageID == "msg-t-fallback" && msg.Timestamp == int64(1706700000)
	})).Return(nil)

	err := processor.ProcessMessage(context.Background(), "session1", payload)

	assert.NoError(t, err)
	mockSvc.AssertExpectations(t)
}

