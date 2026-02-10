package workers_test

import (
	"context"
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
