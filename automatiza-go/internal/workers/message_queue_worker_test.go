// Package workers provides background job processing for the Automatiza platform.
package workers

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/models"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// =============================================================================
// Mock Implementations
// =============================================================================

// mockActionHistoryRepo implements repository.ActionHistoryRepository for testing.
type mockActionHistoryRepo struct {
	mock.Mock
}

func (m *mockActionHistoryRepo) Create(ctx context.Context, userID, actionType, description string, details map[string]interface{}) (*models.ActionHistory, error) {
	args := m.Called(ctx, userID, actionType, description, details)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ActionHistory), args.Error(1)
}

func (m *mockActionHistoryRepo) Update(ctx context.Context, historyID string, updates map[string]interface{}) error {
	args := m.Called(ctx, historyID, updates)
	return args.Error(0)
}

func (m *mockActionHistoryRepo) UpdateStatus(ctx context.Context, id, status, errorMessage string) error {
	args := m.Called(ctx, id, status, errorMessage)
	return args.Error(0)
}

func (m *mockActionHistoryRepo) GetByID(ctx context.Context, historyID string) (*models.ActionHistory, error) {
	args := m.Called(ctx, historyID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ActionHistory), args.Error(1)
}

func (m *mockActionHistoryRepo) GetUserHistory(ctx context.Context, userID string, limit, offset int) ([]models.ActionHistory, error) {
	args := m.Called(ctx, userID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.ActionHistory), args.Error(1)
}

func (m *mockActionHistoryRepo) GetByUserID(ctx context.Context, userID string, limit, offset int) ([]models.ActionHistory, error) {
	args := m.Called(ctx, userID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.ActionHistory), args.Error(1)
}

func (m *mockActionHistoryRepo) GetByActionType(ctx context.Context, userID, actionType string, limit int) ([]models.ActionHistory, error) {
	args := m.Called(ctx, userID, actionType, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.ActionHistory), args.Error(1)
}

func (m *mockActionHistoryRepo) GetUserHistoryFiltered(ctx context.Context, req models.HistoryListRequest, userID string) ([]models.ActionHistory, error) {
	args := m.Called(ctx, req, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.ActionHistory), args.Error(1)
}

func (m *mockActionHistoryRepo) MarkCompleted(ctx context.Context, historyID string) error {
	args := m.Called(ctx, historyID)
	return args.Error(0)
}

func (m *mockActionHistoryRepo) MarkFailed(ctx context.Context, historyID, errorMessage string) error {
	args := m.Called(ctx, historyID, errorMessage)
	return args.Error(0)
}

func (m *mockActionHistoryRepo) MarkRunning(ctx context.Context, historyID string) error {
	args := m.Called(ctx, historyID)
	return args.Error(0)
}

func (m *mockActionHistoryRepo) IncrementSuccessCount(ctx context.Context, historyID string) error {
	args := m.Called(ctx, historyID)
	return args.Error(0)
}

func (m *mockActionHistoryRepo) IncrementErrorCount(ctx context.Context, historyID string, errorMsg string) error {
	args := m.Called(ctx, historyID, errorMsg)
	return args.Error(0)
}

func (m *mockActionHistoryRepo) UpdateCounts(ctx context.Context, historyID string, successCount, errorCount int32) error {
	args := m.Called(ctx, historyID, successCount, errorCount)
	return args.Error(0)
}

func (m *mockActionHistoryRepo) CleanupOldByActionType(_ context.Context, _ string, _ time.Duration) (int64, error) {
	return 0, nil
}

func (m *mockActionHistoryRepo) DeleteUserHistory(ctx context.Context, userID string, beforeDate *time.Time, actionTypes []string) (int64, error) {
	args := m.Called(ctx, userID, beforeDate, actionTypes)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockActionHistoryRepo) CountUserHistoryFiltered(ctx context.Context, req models.HistoryListRequest, userID string) (models.HistoryStatsResult, error) {
	args := m.Called(ctx, req, userID)
	return args.Get(0).(models.HistoryStatsResult), args.Error(1)
}

// =============================================================================
// Helper Functions
// =============================================================================

// createTestMessageWorker creates a MessageQueueWorker with FakeAdapter for testing.
func createTestMessageWorker(t *testing.T) (*MessageQueueWorker, *whatsapp.FakeAdapter, *mockActionHistoryRepo) {
	t.Helper()

	fakeAdapter := whatsapp.NewFakeAdapter()
	actionHistoryRepo := new(mockActionHistoryRepo)
	logger := zap.NewNop()

	worker := NewMessageQueueWorker(
		fakeAdapter,
		actionHistoryRepo,
		nil, // inspector
		nil, // client
		logger,
	)

	return worker, fakeAdapter, actionHistoryRepo
}

// createMessageTask creates an asynq.Task from a MessageTask.
func createMessageTask(t *testing.T, msg MessageTask) *asynq.Task {
	t.Helper()

	payload, err := json.Marshal(msg)
	require.NoError(t, err)

	return asynq.NewTask(msg.Type, payload)
}

// =============================================================================
// ProcessTask Tests - Text Message
// =============================================================================

func TestProcessTask_TextMessage_Success(t *testing.T) {
	worker, fakeAdapter, actionHistoryRepo := createTestMessageWorker(t)

	ctx := context.Background()

	textPayload, _ := json.Marshal(TextMessagePayload{
		Text:     "Hello, World!",
		Mentions: []string{"user1@c.us", "user2@c.us"},
	})

	msgTask := MessageTask{
		Type:        TypeMessageText,
		UserID:      "user-123",
		InstanceID:  "instance-456",
		SessionName: "test-session",
		GroupID:     "group@g.us",
		Payload:     textPayload,
		ActionID:    "action-789",
	}

	task := createMessageTask(t, msgTask)

	// Expect action history updates
	actionHistoryRepo.On("UpdateStatus", ctx, "action-789", models.ActionStatusRunning, "").Return(nil)
	actionHistoryRepo.On("UpdateStatus", ctx, "action-789", models.ActionStatusCompleted, "").Return(nil)

	// Execute
	err := worker.ProcessTask(ctx, task)

	// Verify
	require.NoError(t, err)
	actionHistoryRepo.AssertExpectations(t)

	// Verify the fake adapter was called
	assert.Equal(t, 1, fakeAdapter.CallCount("SendText"))
	lastCall := fakeAdapter.LastCall("SendText")
	require.NotNil(t, lastCall)

	// Check the input passed to SendText
	input := lastCall.Args[0].(whatsapp.SendTextInput)
	assert.Equal(t, "test-session", input.SessionName)
	assert.Equal(t, "group@g.us", input.ChatID)
	assert.Equal(t, "Hello, World!", input.Text)
	assert.Equal(t, []string{"user1@c.us", "user2@c.us"}, input.Mentions)
}

func TestProcessTask_TextMessage_EmptyText(t *testing.T) {
	worker, _, actionHistoryRepo := createTestMessageWorker(t)

	ctx := context.Background()

	textPayload, _ := json.Marshal(TextMessagePayload{
		Text: "", // Empty text
	})

	msgTask := MessageTask{
		Type:        TypeMessageText,
		UserID:      "user-123",
		SessionName: "test-session",
		GroupID:     "group@g.us",
		Payload:     textPayload,
		ActionID:    "action-789",
	}

	task := createMessageTask(t, msgTask)

	actionHistoryRepo.On("UpdateStatus", ctx, "action-789", models.ActionStatusRunning, "").Return(nil)
	actionHistoryRepo.On("UpdateStatus", ctx, "action-789", models.ActionStatusFailed, mock.AnythingOfType("string")).Return(nil)

	// Execute - empty text should fail
	err := worker.ProcessTask(ctx, task)

	// Empty text is a validation error - currently treated as retryable by default
	// (implementation returns error for retry if not explicitly non-retryable)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "text message cannot be empty")
	actionHistoryRepo.AssertExpectations(t)
}

func TestProcessTask_TextMessage_SendFails(t *testing.T) {
	worker, fakeAdapter, actionHistoryRepo := createTestMessageWorker(t)

	ctx := context.Background()

	textPayload, _ := json.Marshal(TextMessagePayload{
		Text: "Hello!",
	})

	msgTask := MessageTask{
		Type:        TypeMessageText,
		UserID:      "user-123",
		SessionName: "test-session",
		GroupID:     "group@g.us",
		Payload:     textPayload,
		ActionID:    "action-789",
	}

	task := createMessageTask(t, msgTask)

	// Configure fake adapter to return a retryable error
	fakeAdapter.SimulateError = whatsapp.NewProviderError(
		whatsapp.ErrCodeNetworkError,
		"connection reset",
		nil,
	)

	actionHistoryRepo.On("UpdateStatus", ctx, "action-789", models.ActionStatusRunning, "").Return(nil)
	actionHistoryRepo.On("UpdateStatus", ctx, "action-789", models.ActionStatusFailed, mock.AnythingOfType("string")).Return(nil)

	// Execute
	err := worker.ProcessTask(ctx, task)

	// Should return error for retry
	require.Error(t, err)
	actionHistoryRepo.AssertExpectations(t)
}

// =============================================================================
// ProcessTask Tests - Audio Message
// =============================================================================

func TestProcessTask_AudioMessage_Success(t *testing.T) {
	worker, fakeAdapter, actionHistoryRepo := createTestMessageWorker(t)

	ctx := context.Background()

	audioPayload, _ := json.Marshal(AudioMessagePayload{
		AudioBase64: "SGVsbG8gV29ybGQh", // Base64 encoded audio
		Mimetype:    "audio/ogg; codecs=opus",
	})

	msgTask := MessageTask{
		Type:        TypeMessageAudio,
		UserID:      "user-123",
		SessionName: "test-session",
		GroupID:     "group@g.us",
		Payload:     audioPayload,
		ActionID:    "action-audio",
	}

	task := createMessageTask(t, msgTask)

	actionHistoryRepo.On("UpdateStatus", ctx, "action-audio", models.ActionStatusRunning, "").Return(nil)
	actionHistoryRepo.On("UpdateStatus", ctx, "action-audio", models.ActionStatusCompleted, "").Return(nil)

	// Execute
	err := worker.ProcessTask(ctx, task)

	require.NoError(t, err)
	assert.Equal(t, 1, fakeAdapter.CallCount("SendVoice"))

	lastCall := fakeAdapter.LastCall("SendVoice")
	require.NotNil(t, lastCall)

	input := lastCall.Args[0].(whatsapp.SendVoiceInput)
	assert.Equal(t, "test-session", input.SessionName)
	assert.Equal(t, "group@g.us", input.ChatID)
	assert.Equal(t, "SGVsbG8gV29ybGQh", input.AudioBase64)
	assert.Equal(t, "audio/ogg; codecs=opus", input.Mimetype)
}

func TestProcessTask_AudioMessage_DefaultMimetype(t *testing.T) {
	worker, fakeAdapter, actionHistoryRepo := createTestMessageWorker(t)

	ctx := context.Background()

	audioPayload, _ := json.Marshal(AudioMessagePayload{
		AudioBase64: "base64audio",
		// No mimetype - should default to "audio/ogg; codecs=opus"
	})

	msgTask := MessageTask{
		Type:        TypeMessageAudio,
		UserID:      "user-123",
		SessionName: "test-session",
		GroupID:     "group@g.us",
		Payload:     audioPayload,
		ActionID:    "action-audio",
	}

	task := createMessageTask(t, msgTask)

	actionHistoryRepo.On("UpdateStatus", ctx, "action-audio", models.ActionStatusRunning, "").Return(nil)
	actionHistoryRepo.On("UpdateStatus", ctx, "action-audio", models.ActionStatusCompleted, "").Return(nil)

	err := worker.ProcessTask(ctx, task)

	require.NoError(t, err)

	lastCall := fakeAdapter.LastCall("SendVoice")
	require.NotNil(t, lastCall)

	input := lastCall.Args[0].(whatsapp.SendVoiceInput)
	assert.Equal(t, "audio/ogg; codecs=opus", input.Mimetype)
}

func TestProcessTask_AudioMessage_EmptyAudio(t *testing.T) {
	worker, _, actionHistoryRepo := createTestMessageWorker(t)

	ctx := context.Background()

	audioPayload, _ := json.Marshal(AudioMessagePayload{
		AudioBase64: "", // Empty audio
	})

	msgTask := MessageTask{
		Type:        TypeMessageAudio,
		UserID:      "user-123",
		SessionName: "test-session",
		GroupID:     "group@g.us",
		Payload:     audioPayload,
		ActionID:    "action-audio",
	}

	task := createMessageTask(t, msgTask)

	actionHistoryRepo.On("UpdateStatus", ctx, "action-audio", models.ActionStatusRunning, "").Return(nil)
	actionHistoryRepo.On("UpdateStatus", ctx, "action-audio", models.ActionStatusFailed, mock.AnythingOfType("string")).Return(nil)

	err := worker.ProcessTask(ctx, task)

	// Validation error - currently treated as retryable by default
	require.Error(t, err)
	assert.Contains(t, err.Error(), "audio message requires audio_base64")
	actionHistoryRepo.AssertExpectations(t)
}

// =============================================================================
// ProcessTask Tests - Image Message
// =============================================================================

func TestProcessTask_ImageMessage_WithURL(t *testing.T) {
	worker, fakeAdapter, actionHistoryRepo := createTestMessageWorker(t)

	ctx := context.Background()

	imagePayload, _ := json.Marshal(ImageMessagePayload{
		Caption:  "Check this out!",
		ImageURL: "https://example.com/image.jpg",
	})

	msgTask := MessageTask{
		Type:        TypeMessageImage,
		UserID:      "user-123",
		SessionName: "test-session",
		GroupID:     "group@g.us",
		Payload:     imagePayload,
		ActionID:    "action-image",
	}

	task := createMessageTask(t, msgTask)

	actionHistoryRepo.On("UpdateStatus", ctx, "action-image", models.ActionStatusRunning, "").Return(nil)
	actionHistoryRepo.On("UpdateStatus", ctx, "action-image", models.ActionStatusCompleted, "").Return(nil)

	err := worker.ProcessTask(ctx, task)

	require.NoError(t, err)
	assert.Equal(t, 1, fakeAdapter.CallCount("SendImage"))

	lastCall := fakeAdapter.LastCall("SendImage")
	require.NotNil(t, lastCall)

	input := lastCall.Args[0].(whatsapp.SendImageInput)
	assert.Equal(t, "test-session", input.SessionName)
	assert.Equal(t, "group@g.us", input.ChatID)
	assert.Equal(t, "Check this out!", input.Caption)
	assert.Equal(t, "https://example.com/image.jpg", input.ImageURL)
}

func TestProcessTask_ImageMessage_WithBase64(t *testing.T) {
	worker, fakeAdapter, actionHistoryRepo := createTestMessageWorker(t)

	ctx := context.Background()

	imagePayload, _ := json.Marshal(ImageMessagePayload{
		Caption:     "Base64 image",
		ImageBase64: "iVBORw0KGgoAAAANSUhEUg==", // Base64 image data
		Mimetype:    "image/png",
	})

	msgTask := MessageTask{
		Type:        TypeMessageImage,
		UserID:      "user-123",
		SessionName: "test-session",
		GroupID:     "group@g.us",
		Payload:     imagePayload,
		ActionID:    "action-image",
	}

	task := createMessageTask(t, msgTask)

	actionHistoryRepo.On("UpdateStatus", ctx, "action-image", models.ActionStatusRunning, "").Return(nil)
	actionHistoryRepo.On("UpdateStatus", ctx, "action-image", models.ActionStatusCompleted, "").Return(nil)

	err := worker.ProcessTask(ctx, task)

	require.NoError(t, err)

	lastCall := fakeAdapter.LastCall("SendImage")
	require.NotNil(t, lastCall)

	input := lastCall.Args[0].(whatsapp.SendImageInput)
	assert.Equal(t, "iVBORw0KGgoAAAANSUhEUg==", input.ImageBase64)
	assert.Equal(t, "image/png", input.Mimetype)
}

func TestProcessTask_ImageMessage_DataURLFormat(t *testing.T) {
	worker, fakeAdapter, actionHistoryRepo := createTestMessageWorker(t)

	ctx := context.Background()

	// Test with data URL format that should be stripped
	imagePayload, _ := json.Marshal(ImageMessagePayload{
		ImageBase64: "data:image/png;base64,iVBORw0KGgoAAAANSUhEUg==",
	})

	msgTask := MessageTask{
		Type:        TypeMessageImage,
		UserID:      "user-123",
		SessionName: "test-session",
		GroupID:     "group@g.us",
		Payload:     imagePayload,
		ActionID:    "action-image",
	}

	task := createMessageTask(t, msgTask)

	actionHistoryRepo.On("UpdateStatus", ctx, "action-image", models.ActionStatusRunning, "").Return(nil)
	actionHistoryRepo.On("UpdateStatus", ctx, "action-image", models.ActionStatusCompleted, "").Return(nil)

	err := worker.ProcessTask(ctx, task)

	require.NoError(t, err)

	lastCall := fakeAdapter.LastCall("SendImage")
	require.NotNil(t, lastCall)

	input := lastCall.Args[0].(whatsapp.SendImageInput)
	// Should strip the data URL prefix
	assert.Equal(t, "iVBORw0KGgoAAAANSUhEUg==", input.ImageBase64)
}

func TestProcessTask_ImageMessage_NoImageData(t *testing.T) {
	worker, _, actionHistoryRepo := createTestMessageWorker(t)

	ctx := context.Background()

	imagePayload, _ := json.Marshal(ImageMessagePayload{
		Caption: "No image provided",
		// No ImageURL or ImageBase64
	})

	msgTask := MessageTask{
		Type:        TypeMessageImage,
		UserID:      "user-123",
		SessionName: "test-session",
		GroupID:     "group@g.us",
		Payload:     imagePayload,
		ActionID:    "action-image",
	}

	task := createMessageTask(t, msgTask)

	actionHistoryRepo.On("UpdateStatus", ctx, "action-image", models.ActionStatusRunning, "").Return(nil)
	actionHistoryRepo.On("UpdateStatus", ctx, "action-image", models.ActionStatusFailed, mock.AnythingOfType("string")).Return(nil)

	err := worker.ProcessTask(ctx, task)

	// Validation error - currently treated as retryable by default
	require.Error(t, err)
	assert.Contains(t, err.Error(), "image message requires image_url or image_base64")
}

// =============================================================================
// ProcessTask Tests - Poll Message
// =============================================================================

func TestProcessTask_PollMessage_Success(t *testing.T) {
	worker, fakeAdapter, actionHistoryRepo := createTestMessageWorker(t)

	ctx := context.Background()

	pollPayload, _ := json.Marshal(PollMessagePayload{
		Question:        "What's your favorite color?",
		Options:         []string{"Red", "Blue", "Green"},
		SelectableCount: 1,
	})

	msgTask := MessageTask{
		Type:        TypeMessagePoll,
		UserID:      "user-123",
		SessionName: "test-session",
		GroupID:     "group@g.us",
		Payload:     pollPayload,
		ActionID:    "action-poll",
	}

	task := createMessageTask(t, msgTask)

	actionHistoryRepo.On("UpdateStatus", ctx, "action-poll", models.ActionStatusRunning, "").Return(nil)
	actionHistoryRepo.On("UpdateStatus", ctx, "action-poll", models.ActionStatusCompleted, "").Return(nil)

	err := worker.ProcessTask(ctx, task)

	require.NoError(t, err)
	assert.Equal(t, 1, fakeAdapter.CallCount("SendPoll"))

	lastCall := fakeAdapter.LastCall("SendPoll")
	require.NotNil(t, lastCall)

	input := lastCall.Args[0].(whatsapp.SendPollInput)
	assert.Equal(t, "test-session", input.SessionName)
	assert.Equal(t, "group@g.us", input.ChatID)
	assert.Equal(t, "What's your favorite color?", input.Question)
	assert.Equal(t, []string{"Red", "Blue", "Green"}, input.Options)
	assert.Equal(t, 1, input.SelectableCount)
}

func TestProcessTask_PollMessage_DefaultSelectableCount(t *testing.T) {
	worker, fakeAdapter, actionHistoryRepo := createTestMessageWorker(t)

	ctx := context.Background()

	pollPayload, _ := json.Marshal(PollMessagePayload{
		Question: "Pick one",
		Options:  []string{"A", "B"},
		// SelectableCount defaults to 0, should be set to 1
	})

	msgTask := MessageTask{
		Type:        TypeMessagePoll,
		UserID:      "user-123",
		SessionName: "test-session",
		GroupID:     "group@g.us",
		Payload:     pollPayload,
		ActionID:    "action-poll",
	}

	task := createMessageTask(t, msgTask)

	actionHistoryRepo.On("UpdateStatus", ctx, "action-poll", models.ActionStatusRunning, "").Return(nil)
	actionHistoryRepo.On("UpdateStatus", ctx, "action-poll", models.ActionStatusCompleted, "").Return(nil)

	err := worker.ProcessTask(ctx, task)

	require.NoError(t, err)

	lastCall := fakeAdapter.LastCall("SendPoll")
	require.NotNil(t, lastCall)

	input := lastCall.Args[0].(whatsapp.SendPollInput)
	assert.Equal(t, 1, input.SelectableCount)
}

func TestProcessTask_PollMessage_SelectableCountExceedsOptions(t *testing.T) {
	worker, fakeAdapter, actionHistoryRepo := createTestMessageWorker(t)

	ctx := context.Background()

	pollPayload, _ := json.Marshal(PollMessagePayload{
		Question:        "Pick multiple",
		Options:         []string{"A", "B", "C"},
		SelectableCount: 10, // Exceeds number of options
	})

	msgTask := MessageTask{
		Type:        TypeMessagePoll,
		UserID:      "user-123",
		SessionName: "test-session",
		GroupID:     "group@g.us",
		Payload:     pollPayload,
		ActionID:    "action-poll",
	}

	task := createMessageTask(t, msgTask)

	actionHistoryRepo.On("UpdateStatus", ctx, "action-poll", models.ActionStatusRunning, "").Return(nil)
	actionHistoryRepo.On("UpdateStatus", ctx, "action-poll", models.ActionStatusCompleted, "").Return(nil)

	err := worker.ProcessTask(ctx, task)

	require.NoError(t, err)

	lastCall := fakeAdapter.LastCall("SendPoll")
	require.NotNil(t, lastCall)

	input := lastCall.Args[0].(whatsapp.SendPollInput)
	// Should be capped to number of options
	assert.Equal(t, 3, input.SelectableCount)
}

func TestProcessTask_PollMessage_EmptyQuestion(t *testing.T) {
	worker, _, actionHistoryRepo := createTestMessageWorker(t)

	ctx := context.Background()

	pollPayload, _ := json.Marshal(PollMessagePayload{
		Question: "",
		Options:  []string{"A", "B"},
	})

	msgTask := MessageTask{
		Type:        TypeMessagePoll,
		UserID:      "user-123",
		SessionName: "test-session",
		GroupID:     "group@g.us",
		Payload:     pollPayload,
		ActionID:    "action-poll",
	}

	task := createMessageTask(t, msgTask)

	actionHistoryRepo.On("UpdateStatus", ctx, "action-poll", models.ActionStatusRunning, "").Return(nil)
	actionHistoryRepo.On("UpdateStatus", ctx, "action-poll", models.ActionStatusFailed, mock.AnythingOfType("string")).Return(nil)

	err := worker.ProcessTask(ctx, task)

	// Validation error - currently treated as retryable by default
	require.Error(t, err)
	assert.Contains(t, err.Error(), "poll message requires a question")
}

func TestProcessTask_PollMessage_TooFewOptions(t *testing.T) {
	worker, _, actionHistoryRepo := createTestMessageWorker(t)

	ctx := context.Background()

	pollPayload, _ := json.Marshal(PollMessagePayload{
		Question: "Only one option",
		Options:  []string{"A"}, // Needs at least 2
	})

	msgTask := MessageTask{
		Type:        TypeMessagePoll,
		UserID:      "user-123",
		SessionName: "test-session",
		GroupID:     "group@g.us",
		Payload:     pollPayload,
		ActionID:    "action-poll",
	}

	task := createMessageTask(t, msgTask)

	actionHistoryRepo.On("UpdateStatus", ctx, "action-poll", models.ActionStatusRunning, "").Return(nil)
	actionHistoryRepo.On("UpdateStatus", ctx, "action-poll", models.ActionStatusFailed, mock.AnythingOfType("string")).Return(nil)

	err := worker.ProcessTask(ctx, task)

	// Validation error - currently treated as retryable by default
	require.Error(t, err)
	assert.Contains(t, err.Error(), "poll message requires at least 2 options")
}

func TestProcessTask_PollMessage_TooManyOptions(t *testing.T) {
	worker, _, actionHistoryRepo := createTestMessageWorker(t)

	ctx := context.Background()

	pollPayload, _ := json.Marshal(PollMessagePayload{
		Question: "Too many options",
		Options:  []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12", "13"}, // Max is 12
	})

	msgTask := MessageTask{
		Type:        TypeMessagePoll,
		UserID:      "user-123",
		SessionName: "test-session",
		GroupID:     "group@g.us",
		Payload:     pollPayload,
		ActionID:    "action-poll",
	}

	task := createMessageTask(t, msgTask)

	actionHistoryRepo.On("UpdateStatus", ctx, "action-poll", models.ActionStatusRunning, "").Return(nil)
	actionHistoryRepo.On("UpdateStatus", ctx, "action-poll", models.ActionStatusFailed, mock.AnythingOfType("string")).Return(nil)

	err := worker.ProcessTask(ctx, task)

	// Validation error - currently treated as retryable by default
	require.Error(t, err)
	assert.Contains(t, err.Error(), "poll message cannot have more than 12 options")
}

// =============================================================================
// ProcessTask Tests - Error Handling
// =============================================================================

func TestMessageWorker_ProcessTask_MalformedPayload(t *testing.T) {
	worker, _, _ := createTestMessageWorker(t)

	ctx := context.Background()

	// Create a task with invalid JSON payload
	task := asynq.NewTask(TypeMessageText, []byte("invalid json"))

	err := worker.ProcessTask(ctx, task)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal")
}

func TestProcessTask_UnknownMessageType(t *testing.T) {
	worker, _, actionHistoryRepo := createTestMessageWorker(t)

	ctx := context.Background()

	msgTask := MessageTask{
		Type:        "unknown:type",
		UserID:      "user-123",
		SessionName: "test-session",
		GroupID:     "group@g.us",
		Payload:     []byte("{}"),
		ActionID:    "action-unknown",
	}

	task := createMessageTask(t, msgTask)

	actionHistoryRepo.On("UpdateStatus", ctx, "action-unknown", models.ActionStatusRunning, "").Return(nil)
	actionHistoryRepo.On("UpdateStatus", ctx, "action-unknown", models.ActionStatusFailed, mock.AnythingOfType("string")).Return(nil)

	err := worker.ProcessTask(ctx, task)

	// Unknown type - currently treated as retryable by default
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown message type")
}

func TestProcessTask_NoActionID(t *testing.T) {
	worker, fakeAdapter, _ := createTestMessageWorker(t)

	ctx := context.Background()

	textPayload, _ := json.Marshal(TextMessagePayload{
		Text: "No action ID",
	})

	msgTask := MessageTask{
		Type:        TypeMessageText,
		UserID:      "user-123",
		SessionName: "test-session",
		GroupID:     "group@g.us",
		Payload:     textPayload,
		ActionID:    "", // No action ID
	}

	task := createMessageTask(t, msgTask)

	// Should still process without updating action history
	err := worker.ProcessTask(ctx, task)

	require.NoError(t, err)
	assert.Equal(t, 1, fakeAdapter.CallCount("SendText"))
}

// =============================================================================
// isRetryableError Tests
// =============================================================================

func TestIsRetryableError_Nil(t *testing.T) {
	worker, _, _ := createTestMessageWorker(t)

	assert.False(t, worker.isRetryableError(nil))
}

func TestIsRetryableError_ProviderErrors(t *testing.T) {
	worker, _, _ := createTestMessageWorker(t)

	tests := []struct {
		name      string
		errorCode whatsapp.ErrorCode
		retryable bool
	}{
		{"SessionNotFound", whatsapp.ErrCodeSessionNotFound, false},
		{"GroupNotFound", whatsapp.ErrCodeGroupNotFound, false},
		{"InvalidInput", whatsapp.ErrCodeInvalidInput, false},
		{"PermissionDenied", whatsapp.ErrCodePermissionDenied, false},
		{"Timeout", whatsapp.ErrCodeTimeout, false},
		{"SessionNotConnected", whatsapp.ErrCodeSessionNotConnected, true},
		{"RateLimited", whatsapp.ErrCodeRateLimited, true},
		{"NetworkError", whatsapp.ErrCodeNetworkError, true},
		{"ProviderError", whatsapp.ErrCodeProviderError, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := whatsapp.NewProviderError(tt.errorCode, "test error", nil)
			result := worker.isRetryableError(err)
			assert.Equal(t, tt.retryable, result)
		})
	}
}

func TestIsRetryableError_LegacyErrors(t *testing.T) {
	worker, _, _ := createTestMessageWorker(t)

	tests := []struct {
		name      string
		errMsg    string
		retryable bool
	}{
		{"404 error", "HTTP 404 not found", false},
		{"not found", "resource not found", false},
		{"invalid", "invalid request", false},
		{"permission", "permission denied", false},
		{"timeout", "connection timeout", false},
		{"context deadline", "context deadline exceeded", false},
		{"connection error", "connection reset", true},
		{"EOF error", "unexpected EOF", true},
		{"rate limit", "rate limit exceeded", true},
		{"unknown error", "something went wrong", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := errors.New(tt.errMsg)
			result := worker.isRetryableError(err)
			assert.Equal(t, tt.retryable, result, "expected %v for error: %s", tt.retryable, tt.errMsg)
		})
	}
}

// =============================================================================
// Backpressure Calculation Tests
// =============================================================================

func TestBackpressureCalculation(t *testing.T) {
	// Test the backpressure delay calculation logic
	tests := []struct {
		name          string
		queueDepth    int
		expectedDelay time.Duration
	}{
		{"empty queue", 0, 0},
		{"10 pending", 10, 1 * time.Second},
		{"50 pending", 50, 5 * time.Second},   // Should be capped at maxBackpressureDelay
		{"100 pending", 100, 5 * time.Second}, // Should still be capped
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			delay := time.Duration(tt.queueDepth) * backpressureDelayPerMs
			if delay > maxBackpressureDelay {
				delay = maxBackpressureDelay
			}
			assert.Equal(t, tt.expectedDelay, delay)
		})
	}
}

// =============================================================================
// Action History Update Tests
// =============================================================================

func TestProcessTask_ActionHistoryUpdateOnSuccess(t *testing.T) {
	worker, _, actionHistoryRepo := createTestMessageWorker(t)

	ctx := context.Background()

	textPayload, _ := json.Marshal(TextMessagePayload{Text: "Test"})
	msgTask := MessageTask{
		Type:        TypeMessageText,
		UserID:      "user-123",
		SessionName: "test-session",
		GroupID:     "group@g.us",
		Payload:     textPayload,
		ActionID:    "action-test",
	}

	task := createMessageTask(t, msgTask)

	// Expect running then completed
	actionHistoryRepo.On("UpdateStatus", ctx, "action-test", models.ActionStatusRunning, "").Return(nil).Once()
	actionHistoryRepo.On("UpdateStatus", ctx, "action-test", models.ActionStatusCompleted, "").Return(nil).Once()

	err := worker.ProcessTask(ctx, task)

	require.NoError(t, err)
	actionHistoryRepo.AssertExpectations(t)
}

func TestProcessTask_ActionHistoryUpdateOnFailure(t *testing.T) {
	worker, fakeAdapter, actionHistoryRepo := createTestMessageWorker(t)

	ctx := context.Background()

	textPayload, _ := json.Marshal(TextMessagePayload{Text: "Test"})
	msgTask := MessageTask{
		Type:        TypeMessageText,
		UserID:      "user-123",
		SessionName: "test-session",
		GroupID:     "group@g.us",
		Payload:     textPayload,
		ActionID:    "action-test",
	}

	task := createMessageTask(t, msgTask)

	// Configure error
	fakeAdapter.SimulateError = whatsapp.NewProviderError(
		whatsapp.ErrCodeSessionNotFound,
		"session not found",
		nil,
	)

	// Expect running then failed
	actionHistoryRepo.On("UpdateStatus", ctx, "action-test", models.ActionStatusRunning, "").Return(nil).Once()
	actionHistoryRepo.On("UpdateStatus", ctx, "action-test", models.ActionStatusFailed, mock.AnythingOfType("string")).Return(nil).Once()

	err := worker.ProcessTask(ctx, task)

	// Non-retryable error returns nil
	require.NoError(t, err)
	actionHistoryRepo.AssertExpectations(t)
}

func TestProcessTask_ActionHistoryUpdateErrorContinues(t *testing.T) {
	worker, _, actionHistoryRepo := createTestMessageWorker(t)

	ctx := context.Background()

	textPayload, _ := json.Marshal(TextMessagePayload{Text: "Test"})
	msgTask := MessageTask{
		Type:        TypeMessageText,
		UserID:      "user-123",
		SessionName: "test-session",
		GroupID:     "group@g.us",
		Payload:     textPayload,
		ActionID:    "action-test",
	}

	task := createMessageTask(t, msgTask)

	// Simulate failure to update action history (should not affect message sending)
	actionHistoryRepo.On("UpdateStatus", ctx, "action-test", models.ActionStatusRunning, "").
		Return(errors.New("db error")).Once()
	actionHistoryRepo.On("UpdateStatus", ctx, "action-test", models.ActionStatusCompleted, "").
		Return(errors.New("db error")).Once()

	// Message should still be sent successfully
	err := worker.ProcessTask(ctx, task)

	require.NoError(t, err)
	actionHistoryRepo.AssertExpectations(t)
}

// =============================================================================
// ProcessTaskWrapper Tests
// =============================================================================

func TestMessageWorker_ProcessTaskWrapper(t *testing.T) {
	worker, fakeAdapter, actionHistoryRepo := createTestMessageWorker(t)

	ctx := context.Background()

	textPayload, _ := json.Marshal(TextMessagePayload{Text: "Wrapped test"})
	msgTask := MessageTask{
		Type:        TypeMessageText,
		UserID:      "user-123",
		SessionName: "test-session",
		GroupID:     "group@g.us",
		Payload:     textPayload,
		ActionID:    "action-wrapped",
	}

	task := createMessageTask(t, msgTask)

	actionHistoryRepo.On("UpdateStatus", ctx, "action-wrapped", models.ActionStatusRunning, "").Return(nil)
	actionHistoryRepo.On("UpdateStatus", ctx, "action-wrapped", models.ActionStatusCompleted, "").Return(nil)

	// Get the handler function
	handlerFunc := worker.ProcessTaskWrapper()

	// Execute via the wrapper
	err := handlerFunc(ctx, task)

	require.NoError(t, err)
	assert.Equal(t, 1, fakeAdapter.CallCount("SendText"))
}

// =============================================================================
// Constants Tests
// =============================================================================

func TestMessageTypeConstants(t *testing.T) {
	assert.Equal(t, "message:text", TypeMessageText)
	assert.Equal(t, "message:audio", TypeMessageAudio)
	assert.Equal(t, "message:image", TypeMessageImage)
	assert.Equal(t, "message:poll", TypeMessagePoll)
}

func TestConfigurationConstants(t *testing.T) {
	assert.Equal(t, "messages", defaultMessageQueue)
	assert.Equal(t, 3, maxRetries)
	assert.Equal(t, 30*time.Second, taskTimeout)
	assert.Equal(t, 24*time.Hour, dlqRetention)
	assert.Equal(t, 5*time.Second, maxBackpressureDelay)
	assert.Equal(t, 100*time.Millisecond, backpressureDelayPerMs)
}

// =============================================================================
// NewMessageQueueWorker Tests
// =============================================================================

func TestNewMessageQueueWorker(t *testing.T) {
	fakeAdapter := whatsapp.NewFakeAdapter()
	actionHistoryRepo := new(mockActionHistoryRepo)
	logger := zap.NewNop()

	worker := NewMessageQueueWorker(
		fakeAdapter,
		actionHistoryRepo,
		nil,
		nil,
		logger,
	)

	require.NotNil(t, worker)
	assert.Equal(t, fakeAdapter, worker.whatsappProvider)
	assert.Equal(t, actionHistoryRepo, worker.actionHistoryRepo)
	assert.Nil(t, worker.inspector)
	assert.Nil(t, worker.client)
}

func TestNewMessageQueueWorker_MinimalConfig(t *testing.T) {
	fakeAdapter := whatsapp.NewFakeAdapter()
	logger := zap.NewNop()

	// Should work without action history repo
	worker := NewMessageQueueWorker(
		fakeAdapter,
		nil, // No action history repo
		nil,
		nil,
		logger,
	)

	require.NotNil(t, worker)
	assert.Nil(t, worker.actionHistoryRepo)
}

// =============================================================================
// Invalid Payload (Unmarshal Error) Tests
// =============================================================================

func TestProcessTask_TextMessage_InvalidPayload(t *testing.T) {
	worker, _, actionHistoryRepo := createTestMessageWorker(t)

	ctx := context.Background()

	// Build outer MessageTask JSON manually so the inner "payload" is invalid JSON
	// that will fail when handleTextMessage tries to unmarshal it into TextMessagePayload.
	// Using a JSON number (42) as payload — valid JSON at the outer level but not an object
	// for TextMessagePayload unmarshal.
	outerJSON := `{"type":"message:text","user_id":"user-123","instance_id":"","session_name":"test-session","group_id":"group@g.us","payload":42,"action_id":"action-text-invalid"}`
	task := asynq.NewTask(TypeMessageText, []byte(outerJSON))

	actionHistoryRepo.On("UpdateStatus", ctx, "action-text-invalid", models.ActionStatusRunning, "").Return(nil)
	actionHistoryRepo.On("UpdateStatus", ctx, "action-text-invalid", models.ActionStatusFailed, mock.AnythingOfType("string")).Return(nil)

	err := worker.ProcessTask(ctx, task)

	// Unmarshal error for text payload — "cannot unmarshal number" triggers retryable by default
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal text payload")
	actionHistoryRepo.AssertExpectations(t)
}

func TestProcessTask_AudioMessage_InvalidPayload(t *testing.T) {
	worker, _, actionHistoryRepo := createTestMessageWorker(t)

	ctx := context.Background()

	outerJSON := `{"type":"message:audio","user_id":"user-123","instance_id":"","session_name":"test-session","group_id":"group@g.us","payload":"not-an-object","action_id":"action-audio-invalid"}`
	task := asynq.NewTask(TypeMessageAudio, []byte(outerJSON))

	actionHistoryRepo.On("UpdateStatus", ctx, "action-audio-invalid", models.ActionStatusRunning, "").Return(nil)
	actionHistoryRepo.On("UpdateStatus", ctx, "action-audio-invalid", models.ActionStatusFailed, mock.AnythingOfType("string")).Return(nil)

	err := worker.ProcessTask(ctx, task)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal audio payload")
	actionHistoryRepo.AssertExpectations(t)
}

func TestProcessTask_ImageMessage_InvalidPayload(t *testing.T) {
	worker, _, actionHistoryRepo := createTestMessageWorker(t)

	ctx := context.Background()

	outerJSON := `{"type":"message:image","user_id":"user-123","instance_id":"","session_name":"test-session","group_id":"group@g.us","payload":true,"action_id":"action-image-invalid"}`
	task := asynq.NewTask(TypeMessageImage, []byte(outerJSON))

	actionHistoryRepo.On("UpdateStatus", ctx, "action-image-invalid", models.ActionStatusRunning, "").Return(nil)
	actionHistoryRepo.On("UpdateStatus", ctx, "action-image-invalid", models.ActionStatusFailed, mock.AnythingOfType("string")).Return(nil)

	err := worker.ProcessTask(ctx, task)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal image payload")
	actionHistoryRepo.AssertExpectations(t)
}

func TestProcessTask_PollMessage_InvalidPayload(t *testing.T) {
	worker, _, actionHistoryRepo := createTestMessageWorker(t)

	ctx := context.Background()

	outerJSON := `{"type":"message:poll","user_id":"user-123","instance_id":"","session_name":"test-session","group_id":"group@g.us","payload":[1,2,3],"action_id":"action-poll-invalid"}`
	task := asynq.NewTask(TypeMessagePoll, []byte(outerJSON))

	actionHistoryRepo.On("UpdateStatus", ctx, "action-poll-invalid", models.ActionStatusRunning, "").Return(nil)
	actionHistoryRepo.On("UpdateStatus", ctx, "action-poll-invalid", models.ActionStatusFailed, mock.AnythingOfType("string")).Return(nil)

	err := worker.ProcessTask(ctx, task)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal poll payload")
	actionHistoryRepo.AssertExpectations(t)
}

// =============================================================================
// Image Message Edge Cases
// =============================================================================

func TestProcessTask_ImageMessage_DataURLWithoutComma(t *testing.T) {
	worker, fakeAdapter, actionHistoryRepo := createTestMessageWorker(t)

	ctx := context.Background()

	// data: prefix present but no comma separator — SplitN produces len(parts) != 2,
	// so imageData is NOT stripped and stays as the full string.
	imagePayload, _ := json.Marshal(ImageMessagePayload{
		ImageBase64: "data:image/png;base64CORRUPT",
	})

	msgTask := MessageTask{
		Type:        TypeMessageImage,
		UserID:      "user-123",
		SessionName: "test-session",
		GroupID:     "group@g.us",
		Payload:     imagePayload,
		ActionID:    "action-image-nocomma",
	}

	task := createMessageTask(t, msgTask)

	actionHistoryRepo.On("UpdateStatus", ctx, "action-image-nocomma", models.ActionStatusRunning, "").Return(nil)
	actionHistoryRepo.On("UpdateStatus", ctx, "action-image-nocomma", models.ActionStatusCompleted, "").Return(nil)

	err := worker.ProcessTask(ctx, task)

	require.NoError(t, err)

	lastCall := fakeAdapter.LastCall("SendImage")
	require.NotNil(t, lastCall)

	input := lastCall.Args[0].(whatsapp.SendImageInput)
	// The full string should be kept since SplitN(",", 2) returns only 1 part
	assert.Equal(t, "data:image/png;base64CORRUPT", input.ImageBase64)
	actionHistoryRepo.AssertExpectations(t)
}

func TestProcessTask_ImageMessage_DefaultMimetype(t *testing.T) {
	worker, fakeAdapter, actionHistoryRepo := createTestMessageWorker(t)

	ctx := context.Background()

	// ImageBase64 set but Mimetype empty — should default to "image/jpeg"
	imagePayload, _ := json.Marshal(ImageMessagePayload{
		ImageBase64: "iVBORw0KGgoAAAANSUhEUg==",
		Mimetype:    "", // empty
	})

	msgTask := MessageTask{
		Type:        TypeMessageImage,
		UserID:      "user-123",
		SessionName: "test-session",
		GroupID:     "group@g.us",
		Payload:     imagePayload,
		ActionID:    "action-image-defmime",
	}

	task := createMessageTask(t, msgTask)

	actionHistoryRepo.On("UpdateStatus", ctx, "action-image-defmime", models.ActionStatusRunning, "").Return(nil)
	actionHistoryRepo.On("UpdateStatus", ctx, "action-image-defmime", models.ActionStatusCompleted, "").Return(nil)

	err := worker.ProcessTask(ctx, task)

	require.NoError(t, err)

	lastCall := fakeAdapter.LastCall("SendImage")
	require.NotNil(t, lastCall)

	input := lastCall.Args[0].(whatsapp.SendImageInput)
	assert.Equal(t, "image/jpeg", input.Mimetype)
	actionHistoryRepo.AssertExpectations(t)
}

func TestProcessTask_ImageMessage_URLPriority(t *testing.T) {
	worker, fakeAdapter, actionHistoryRepo := createTestMessageWorker(t)

	ctx := context.Background()

	// Both ImageURL and ImageBase64 are provided.
	// handleImageMessage passes both to SendImageInput — the adapter decides priority.
	imagePayload, _ := json.Marshal(ImageMessagePayload{
		ImageURL:    "https://example.com/photo.jpg",
		ImageBase64: "iVBORw0KGgoAAAANSUhEUg==",
		Mimetype:    "image/png",
		Caption:     "Both provided",
	})

	msgTask := MessageTask{
		Type:        TypeMessageImage,
		UserID:      "user-123",
		SessionName: "test-session",
		GroupID:     "group@g.us",
		Payload:     imagePayload,
		ActionID:    "action-image-both",
	}

	task := createMessageTask(t, msgTask)

	actionHistoryRepo.On("UpdateStatus", ctx, "action-image-both", models.ActionStatusRunning, "").Return(nil)
	actionHistoryRepo.On("UpdateStatus", ctx, "action-image-both", models.ActionStatusCompleted, "").Return(nil)

	err := worker.ProcessTask(ctx, task)

	require.NoError(t, err)

	lastCall := fakeAdapter.LastCall("SendImage")
	require.NotNil(t, lastCall)

	input := lastCall.Args[0].(whatsapp.SendImageInput)
	// Both fields should be passed to the adapter
	assert.Equal(t, "https://example.com/photo.jpg", input.ImageURL)
	assert.Equal(t, "iVBORw0KGgoAAAANSUhEUg==", input.ImageBase64)
	assert.Equal(t, "Both provided", input.Caption)
	assert.Equal(t, "image/png", input.Mimetype)
	actionHistoryRepo.AssertExpectations(t)
}

// =============================================================================
// Send Failure Tests (ProviderError retryable vs non-retryable)
// =============================================================================

func TestProcessTask_AudioMessage_SendFails(t *testing.T) {
	worker, fakeAdapter, actionHistoryRepo := createTestMessageWorker(t)

	ctx := context.Background()

	audioPayload, _ := json.Marshal(AudioMessagePayload{
		AudioBase64: "base64audiodata",
		Mimetype:    "audio/ogg; codecs=opus",
	})

	msgTask := MessageTask{
		Type:        TypeMessageAudio,
		UserID:      "user-123",
		SessionName: "test-session",
		GroupID:     "group@g.us",
		Payload:     audioPayload,
		ActionID:    "action-audio-fail",
	}

	task := createMessageTask(t, msgTask)

	// Retryable error — SessionNotConnected
	fakeAdapter.SimulateError = whatsapp.NewProviderError(
		whatsapp.ErrCodeSessionNotConnected,
		"session not connected",
		nil,
	)

	actionHistoryRepo.On("UpdateStatus", ctx, "action-audio-fail", models.ActionStatusRunning, "").Return(nil)
	actionHistoryRepo.On("UpdateStatus", ctx, "action-audio-fail", models.ActionStatusFailed, mock.AnythingOfType("string")).Return(nil)

	err := worker.ProcessTask(ctx, task)

	// Retryable error should be returned for retry
	require.Error(t, err)
	assert.Contains(t, err.Error(), "session not connected")
	actionHistoryRepo.AssertExpectations(t)
}

func TestProcessTask_PollMessage_SendFails(t *testing.T) {
	worker, fakeAdapter, actionHistoryRepo := createTestMessageWorker(t)

	ctx := context.Background()

	pollPayload, _ := json.Marshal(PollMessagePayload{
		Question:        "Best language?",
		Options:         []string{"Go", "Rust", "Python"},
		SelectableCount: 1,
	})

	msgTask := MessageTask{
		Type:        TypeMessagePoll,
		UserID:      "user-123",
		SessionName: "test-session",
		GroupID:     "group@g.us",
		Payload:     pollPayload,
		ActionID:    "action-poll-fail",
	}

	task := createMessageTask(t, msgTask)

	// Non-retryable error — InvalidInput
	fakeAdapter.SimulateError = whatsapp.NewProviderError(
		whatsapp.ErrCodeInvalidInput,
		"invalid poll format",
		nil,
	)

	actionHistoryRepo.On("UpdateStatus", ctx, "action-poll-fail", models.ActionStatusRunning, "").Return(nil)
	actionHistoryRepo.On("UpdateStatus", ctx, "action-poll-fail", models.ActionStatusFailed, mock.AnythingOfType("string")).Return(nil)

	err := worker.ProcessTask(ctx, task)

	// Non-retryable error returns nil (no retry)
	require.NoError(t, err)
	actionHistoryRepo.AssertExpectations(t)
}

func TestProcessTask_ImageMessage_SendFails(t *testing.T) {
	worker, fakeAdapter, actionHistoryRepo := createTestMessageWorker(t)

	ctx := context.Background()

	imagePayload, _ := json.Marshal(ImageMessagePayload{
		ImageURL: "https://example.com/broken.jpg",
		Caption:  "This will fail",
	})

	msgTask := MessageTask{
		Type:        TypeMessageImage,
		UserID:      "user-123",
		SessionName: "test-session",
		GroupID:     "group@g.us",
		Payload:     imagePayload,
		ActionID:    "action-image-fail",
	}

	task := createMessageTask(t, msgTask)

	// Retryable error — RateLimited
	fakeAdapter.SimulateError = whatsapp.NewProviderError(
		whatsapp.ErrCodeRateLimited,
		"rate limited",
		nil,
	)

	actionHistoryRepo.On("UpdateStatus", ctx, "action-image-fail", models.ActionStatusRunning, "").Return(nil)
	actionHistoryRepo.On("UpdateStatus", ctx, "action-image-fail", models.ActionStatusFailed, mock.AnythingOfType("string")).Return(nil)

	err := worker.ProcessTask(ctx, task)

	// Retryable error should be returned for retry
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rate limited")
	actionHistoryRepo.AssertExpectations(t)
}
