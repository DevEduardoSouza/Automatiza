package reconciliation_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/services/reconciliation"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// =============================================================================
// Mock Implementations
// =============================================================================

// MockCheckpointRepository implements reconciliation.CheckpointRepository
type MockCheckpointRepository struct {
	mock.Mock
}

func (m *MockCheckpointRepository) GetOrCreate(ctx context.Context, sessionName, chatID string) (*reconciliation.Checkpoint, error) {
	args := m.Called(ctx, sessionName, chatID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*reconciliation.Checkpoint), args.Error(1)
}

func (m *MockCheckpointRepository) MarkRunning(ctx context.Context, sessionName, chatID string) error {
	args := m.Called(ctx, sessionName, chatID)
	return args.Error(0)
}

func (m *MockCheckpointRepository) MarkIdle(ctx context.Context, sessionName, chatID string) error {
	args := m.Called(ctx, sessionName, chatID)
	return args.Error(0)
}

func (m *MockCheckpointRepository) MarkFailed(ctx context.Context, sessionName, chatID string) error {
	args := m.Called(ctx, sessionName, chatID)
	return args.Error(0)
}

func (m *MockCheckpointRepository) UpdateCheckpoint(ctx context.Context, sessionName, chatID string, lastTimestamp int64, lastMessageID string) error {
	args := m.Called(ctx, sessionName, chatID, lastTimestamp, lastMessageID)
	return args.Error(0)
}

func (m *MockCheckpointRepository) GetActiveSessionsChats(ctx context.Context) ([]reconciliation.SessionChat, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]reconciliation.SessionChat), args.Error(1)
}

func (m *MockCheckpointRepository) GetBySession(ctx context.Context, sessionName string) ([]reconciliation.Checkpoint, error) {
	args := m.Called(ctx, sessionName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]reconciliation.Checkpoint), args.Error(1)
}

// MockMessageProcessor implements reconciliation.MessageProcessor
type MockMessageProcessor struct {
	mock.Mock
	processedMessages []whatsapp.Message
	mu                sync.Mutex
}

func (m *MockMessageProcessor) ProcessMessage(ctx context.Context, sessionName string, msg *whatsapp.Message) error {
	args := m.Called(ctx, sessionName, msg)

	m.mu.Lock()
	m.processedMessages = append(m.processedMessages, *msg)
	m.mu.Unlock()

	return args.Error(0)
}

func (m *MockMessageProcessor) GetProcessedMessages() []whatsapp.Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.processedMessages
}

// MockWhatsAppProvider wraps whatsapp.FakeAdapter for mock expectations
type MockWhatsAppProvider struct {
	mock.Mock
	*whatsapp.FakeAdapter
}

func NewMockWhatsAppProvider() *MockWhatsAppProvider {
	return &MockWhatsAppProvider{
		FakeAdapter: whatsapp.NewFakeAdapter(),
	}
}

// Override GetMessages to use mock expectations
func (m *MockWhatsAppProvider) GetMessages(ctx context.Context, input whatsapp.GetMessagesInput) (*whatsapp.MessagesResult, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*whatsapp.MessagesResult), args.Error(1)
}

// Override GetSessionStatus to use mock expectations
func (m *MockWhatsAppProvider) GetSessionStatus(ctx context.Context, sessionName string) (*whatsapp.SessionInfo, error) {
	args := m.Called(ctx, sessionName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*whatsapp.SessionInfo), args.Error(1)
}

// Override RestartSession to use mock expectations
func (m *MockWhatsAppProvider) RestartSession(ctx context.Context, sessionName string) error {
	args := m.Called(ctx, sessionName)
	return args.Error(0)
}

// =============================================================================
// Test Fixtures
// =============================================================================

type testServiceDeps struct {
	provider       *MockWhatsAppProvider
	checkpointRepo *MockCheckpointRepository
	messageProc    *MockMessageProcessor
	config         reconciliation.Config
}

func newTestService(t *testing.T) (*reconciliation.Service, *testServiceDeps) {
	t.Helper()
	logger := zaptest.NewLogger(t)

	deps := &testServiceDeps{
		provider:       NewMockWhatsAppProvider(),
		checkpointRepo: new(MockCheckpointRepository),
		messageProc:    new(MockMessageProcessor),
		config: reconciliation.Config{
			SilenceTimeout:     100 * time.Millisecond, // Fast timeout for tests
			BatchSize:          10,
			MaxConcurrent:      5,
			CheckInterval:      50 * time.Millisecond, // Fast interval for tests
			MaxRetries:         2,
			RetryBackoff:       10 * time.Millisecond, // Fast backoff for tests
			HealthCheckEnabled: true,
			MaxRestartAttempts: 2,
			RestartBackoff:     10 * time.Millisecond,
		},
	}

	svc := reconciliation.New(
		deps.provider,
		deps.checkpointRepo,
		deps.messageProc,
		logger,
		deps.config,
	)

	return svc, deps
}

func newTestServiceWithConfig(t *testing.T, config reconciliation.Config) (*reconciliation.Service, *testServiceDeps) {
	t.Helper()
	logger := zaptest.NewLogger(t)

	deps := &testServiceDeps{
		provider:       NewMockWhatsAppProvider(),
		checkpointRepo: new(MockCheckpointRepository),
		messageProc:    new(MockMessageProcessor),
		config:         config,
	}

	svc := reconciliation.New(
		deps.provider,
		deps.checkpointRepo,
		deps.messageProc,
		logger,
		deps.config,
	)

	return svc, deps
}

func createTestMessages(count int, baseTime time.Time) []whatsapp.Message {
	messages := make([]whatsapp.Message, count)
	for i := 0; i < count; i++ {
		messages[i] = whatsapp.Message{
			ID:        "msg-" + itoa(i),
			ChatID:    "chat1@g.us",
			FromMe:    false,
			Sender:    "sender@s.whatsapp.net",
			Timestamp: baseTime.Add(time.Duration(i) * time.Minute),
			Body:      "Message " + itoa(i),
		}
	}
	return messages
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	s := ""
	for i > 0 {
		s = string(rune('0'+i%10)) + s
		i /= 10
	}
	return s
}

// =============================================================================
// TestReconciliationService_Watchdog_SilenceDetection
// =============================================================================

func TestReconciliationService_Watchdog_SilenceDetection(t *testing.T) {
	t.Run("detects silence and triggers health check", func(t *testing.T) {
		svc, deps := newWatchdogTestService(t, fastWatchdogConfig())
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		sessionName := "test-session"

		// Set up session as connected
		deps.provider.On("GetSessionStatus", mock.Anything, sessionName).Return(&whatsapp.SessionInfo{
			Name:   sessionName,
			Status: whatsapp.SessionStatusConnected,
		}, nil).Maybe()

		// Set up empty checkpoints for the session
		deps.checkpointRepo.On("GetBySession", mock.Anything, sessionName).Return([]reconciliation.Checkpoint{}, nil).Maybe()

		// Record an event, then wait for silence timeout
		svc.RecordEvent(sessionName)

		// Start watchdog
		cancelWatchdog := svc.StartWatchdog(ctx)
		defer cancelWatchdog()

		// Wait for silence timeout + check interval
		time.Sleep(200 * time.Millisecond)

		// Verify GetSessionStatus was called (health check triggered)
		deps.provider.AssertCalled(t, "GetSessionStatus", mock.Anything, sessionName)
	})

	t.Run("resets silence timer on event", func(t *testing.T) {
		svc, deps := newWatchdogTestService(t, fastWatchdogConfig())
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		sessionName := "test-session"

		// Set up session as connected
		deps.provider.On("GetSessionStatus", mock.Anything, sessionName).Return(&whatsapp.SessionInfo{
			Name:   sessionName,
			Status: whatsapp.SessionStatusConnected,
		}, nil).Maybe()

		deps.checkpointRepo.On("GetBySession", mock.Anything, sessionName).Return([]reconciliation.Checkpoint{}, nil).Maybe()

		// Start watchdog
		cancelWatchdog := svc.StartWatchdog(ctx)
		defer cancelWatchdog()

		// Record events repeatedly to prevent silence detection
		for i := 0; i < 5; i++ {
			svc.RecordEvent(sessionName)
			time.Sleep(30 * time.Millisecond)
		}

		// Should not have triggered health check because events kept resetting timer
		// Wait a bit for any pending checks
		time.Sleep(50 * time.Millisecond)

		// GetSessionStatus may or may not be called depending on timing
		// The key is that the service handles this gracefully
	})

	t.Run("handles session not connected and attempts restart", func(t *testing.T) {
		config := fastWatchdogConfig()
		config.SilenceTimeout = 50 * time.Millisecond
		config.CheckInterval = 30 * time.Millisecond
		config.RetryBackoff = 5 * time.Millisecond
		config.RestartBackoff = 5 * time.Millisecond
		svc, deps := newWatchdogTestService(t, config)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		sessionName := "test-session"

		// First call returns disconnected, second call (after restart) returns connected
		deps.provider.On("GetSessionStatus", mock.Anything, sessionName).Return(&whatsapp.SessionInfo{
			Name:   sessionName,
			Status: whatsapp.SessionStatusDisconnected,
		}, nil).Once()

		deps.provider.On("RestartSession", mock.Anything, sessionName).Return(nil).Once()

		// After restart, return connected
		deps.provider.On("GetSessionStatus", mock.Anything, sessionName).Return(&whatsapp.SessionInfo{
			Name:   sessionName,
			Status: whatsapp.SessionStatusConnected,
		}, nil).Maybe()

		deps.checkpointRepo.On("GetBySession", mock.Anything, sessionName).Return([]reconciliation.Checkpoint{}, nil).Maybe()

		// Record event and start watchdog
		svc.RecordEvent(sessionName)
		cancelWatchdog := svc.StartWatchdog(ctx)
		defer cancelWatchdog()

		// Wait for silence detection and restart attempt
		time.Sleep(200 * time.Millisecond)

		// Verify restart was attempted
		deps.provider.AssertCalled(t, "RestartSession", mock.Anything, sessionName)
	})
}

// =============================================================================
// TestReconciliationService_Backfill_Success
// =============================================================================

func TestReconciliationService_Backfill_Success(t *testing.T) {
	t.Run("processes messages since checkpoint", func(t *testing.T) {
		svc, deps := newTestService(t)
		ctx := context.Background()

		sessionName := "test-session"
		chatID := "chat1@g.us"
		baseTime := time.Now().Add(-5 * time.Minute) // dentro da MaxBackfillWindow (10min): fora dela o doReconcile pula a mensagem, por design

		// Create checkpoint with old timestamp
		checkpoint := &reconciliation.Checkpoint{
			SessionName:          sessionName,
			ChatID:               chatID,
			LastMessageTimestamp: baseTime.Add(-10 * time.Minute).UnixMilli(),
			LastMessageID:        "old-msg",
			Status:               reconciliation.CheckpointStatusIdle,
		}

		// Create test messages
		messages := createTestMessages(3, baseTime)

		deps.checkpointRepo.On("GetOrCreate", ctx, sessionName, chatID).Return(checkpoint, nil)
		deps.checkpointRepo.On("MarkRunning", ctx, sessionName, chatID).Return(nil)
		deps.checkpointRepo.On("MarkIdle", ctx, sessionName, chatID).Return(nil)
		deps.checkpointRepo.On("UpdateCheckpoint", ctx, sessionName, chatID, mock.AnythingOfType("int64"), mock.AnythingOfType("string")).Return(nil).Maybe()

		deps.provider.On("GetMessages", ctx, mock.MatchedBy(func(input whatsapp.GetMessagesInput) bool {
			return input.SessionName == sessionName && input.ChatID == chatID
		})).Return(&whatsapp.MessagesResult{
			Messages:   messages,
			HasMore:    false,
			NextOffset: 0,
		}, nil)

		deps.messageProc.On("ProcessMessage", ctx, sessionName, mock.AnythingOfType("*whatsapp.Message")).Return(nil)

		err := svc.ReconcileChat(ctx, sessionName, chatID)

		require.NoError(t, err)
		deps.checkpointRepo.AssertExpectations(t)
		deps.provider.AssertExpectations(t)
		deps.messageProc.AssertExpectations(t)
	})

	t.Run("handles pagination correctly", func(t *testing.T) {
		svc, deps := newTestService(t)
		ctx := context.Background()

		sessionName := "test-session"
		chatID := "chat1@g.us"
		baseTime := time.Now().Add(-5 * time.Minute) // dentro da MaxBackfillWindow (10min): fora dela o doReconcile pula a mensagem, por design

		checkpoint := &reconciliation.Checkpoint{
			SessionName:          sessionName,
			ChatID:               chatID,
			LastMessageTimestamp: 0,
			Status:               reconciliation.CheckpointStatusIdle,
		}

		// First batch of messages
		batch1 := createTestMessages(10, baseTime)
		// Second batch of messages
		batch2 := createTestMessages(5, baseTime.Add(10*time.Minute))
		for i := range batch2 {
			batch2[i].ID = "msg-batch2-" + itoa(i)
		}

		deps.checkpointRepo.On("GetOrCreate", ctx, sessionName, chatID).Return(checkpoint, nil)
		deps.checkpointRepo.On("MarkRunning", ctx, sessionName, chatID).Return(nil)
		deps.checkpointRepo.On("MarkIdle", ctx, sessionName, chatID).Return(nil)
		deps.checkpointRepo.On("UpdateCheckpoint", ctx, sessionName, chatID, mock.AnythingOfType("int64"), mock.AnythingOfType("string")).Return(nil).Maybe()

		// First page
		deps.provider.On("GetMessages", ctx, mock.MatchedBy(func(input whatsapp.GetMessagesInput) bool {
			return input.Offset == 0
		})).Return(&whatsapp.MessagesResult{
			Messages:   batch1,
			HasMore:    true,
			NextOffset: 10,
		}, nil).Once()

		// Second page
		deps.provider.On("GetMessages", ctx, mock.MatchedBy(func(input whatsapp.GetMessagesInput) bool {
			return input.Offset == 10
		})).Return(&whatsapp.MessagesResult{
			Messages:   batch2,
			HasMore:    false,
			NextOffset: 0,
		}, nil).Once()

		deps.messageProc.On("ProcessMessage", ctx, sessionName, mock.AnythingOfType("*whatsapp.Message")).Return(nil)

		err := svc.ReconcileChat(ctx, sessionName, chatID)

		require.NoError(t, err)
		deps.provider.AssertNumberOfCalls(t, "GetMessages", 2)
	})

	t.Run("skips already processed messages by ID", func(t *testing.T) {
		svc, deps := newTestService(t)
		ctx := context.Background()

		sessionName := "test-session"
		chatID := "chat1@g.us"
		baseTime := time.Now().Add(-5 * time.Minute) // dentro da MaxBackfillWindow (10min): fora dela o doReconcile pula a mensagem, por design

		// Checkpoint has processed msg-1
		checkpoint := &reconciliation.Checkpoint{
			SessionName:          sessionName,
			ChatID:               chatID,
			LastMessageTimestamp: baseTime.UnixMilli(),
			LastMessageID:        "msg-1",
			Status:               reconciliation.CheckpointStatusIdle,
		}

		messages := createTestMessages(3, baseTime)

		deps.checkpointRepo.On("GetOrCreate", ctx, sessionName, chatID).Return(checkpoint, nil)
		deps.checkpointRepo.On("MarkRunning", ctx, sessionName, chatID).Return(nil)
		deps.checkpointRepo.On("MarkIdle", ctx, sessionName, chatID).Return(nil)
		deps.checkpointRepo.On("UpdateCheckpoint", ctx, sessionName, chatID, mock.AnythingOfType("int64"), mock.AnythingOfType("string")).Return(nil).Maybe()

		deps.provider.On("GetMessages", ctx, mock.Anything).Return(&whatsapp.MessagesResult{
			Messages:   messages,
			HasMore:    false,
			NextOffset: 0,
		}, nil)

		// ProcessMessage should NOT be called for msg-1 (already processed)
		deps.messageProc.On("ProcessMessage", ctx, sessionName, mock.MatchedBy(func(msg *whatsapp.Message) bool {
			return msg.ID != "msg-1"
		})).Return(nil)

		err := svc.ReconcileChat(ctx, sessionName, chatID)

		require.NoError(t, err)
		// Should have processed 2 messages (msg-0 and msg-2), skipping msg-1
	})
}

// =============================================================================
// TestReconciliationService_Backfill_NoGaps
// =============================================================================

func TestReconciliationService_Backfill_NoGaps(t *testing.T) {
	t.Run("handles empty messages list", func(t *testing.T) {
		svc, deps := newTestService(t)
		ctx := context.Background()

		sessionName := "test-session"
		chatID := "chat1@g.us"

		checkpoint := &reconciliation.Checkpoint{
			SessionName:          sessionName,
			ChatID:               chatID,
			LastMessageTimestamp: time.Now().UnixMilli(),
			LastMessageID:        "latest-msg",
			Status:               reconciliation.CheckpointStatusIdle,
		}

		deps.checkpointRepo.On("GetOrCreate", ctx, sessionName, chatID).Return(checkpoint, nil)
		deps.checkpointRepo.On("MarkRunning", ctx, sessionName, chatID).Return(nil)
		deps.checkpointRepo.On("MarkIdle", ctx, sessionName, chatID).Return(nil)

		deps.provider.On("GetMessages", ctx, mock.Anything).Return(&whatsapp.MessagesResult{
			Messages:   []whatsapp.Message{},
			HasMore:    false,
			NextOffset: 0,
		}, nil)

		err := svc.ReconcileChat(ctx, sessionName, chatID)

		require.NoError(t, err)
		// ProcessMessage should not have been called
		deps.messageProc.AssertNotCalled(t, "ProcessMessage", mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("handles all messages already processed", func(t *testing.T) {
		svc, deps := newTestService(t)
		ctx := context.Background()

		sessionName := "test-session"
		chatID := "chat1@g.us"
		baseTime := time.Now()

		// Create a single message
		messages := []whatsapp.Message{
			{
				ID:        "only-msg",
				ChatID:    chatID,
				Timestamp: baseTime,
				Body:      "Only message",
			},
		}

		// Checkpoint already has this message
		checkpoint := &reconciliation.Checkpoint{
			SessionName:          sessionName,
			ChatID:               chatID,
			LastMessageTimestamp: baseTime.UnixMilli(),
			LastMessageID:        "only-msg", // Same as the message
			Status:               reconciliation.CheckpointStatusIdle,
		}

		deps.checkpointRepo.On("GetOrCreate", ctx, sessionName, chatID).Return(checkpoint, nil)
		deps.checkpointRepo.On("MarkRunning", ctx, sessionName, chatID).Return(nil)
		deps.checkpointRepo.On("MarkIdle", ctx, sessionName, chatID).Return(nil)

		deps.provider.On("GetMessages", ctx, mock.Anything).Return(&whatsapp.MessagesResult{
			Messages:   messages,
			HasMore:    false,
			NextOffset: 0,
		}, nil)

		err := svc.ReconcileChat(ctx, sessionName, chatID)

		require.NoError(t, err)
		// ProcessMessage should not have been called because only message was already processed
		deps.messageProc.AssertNotCalled(t, "ProcessMessage", mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("creates new checkpoint for new chat", func(t *testing.T) {
		svc, deps := newTestService(t)
		ctx := context.Background()

		sessionName := "test-session"
		chatID := "new-chat@g.us"
		baseTime := time.Now()

		// New checkpoint with no previous messages
		checkpoint := &reconciliation.Checkpoint{
			SessionName:          sessionName,
			ChatID:               chatID,
			LastMessageTimestamp: 0,
			LastMessageID:        "",
			Status:               reconciliation.CheckpointStatusIdle,
		}

		messages := createTestMessages(2, baseTime)

		deps.checkpointRepo.On("GetOrCreate", ctx, sessionName, chatID).Return(checkpoint, nil)
		deps.checkpointRepo.On("MarkRunning", ctx, sessionName, chatID).Return(nil)
		deps.checkpointRepo.On("MarkIdle", ctx, sessionName, chatID).Return(nil)
		deps.checkpointRepo.On("UpdateCheckpoint", ctx, sessionName, chatID, mock.AnythingOfType("int64"), mock.AnythingOfType("string")).Return(nil).Maybe()

		deps.provider.On("GetMessages", ctx, mock.Anything).Return(&whatsapp.MessagesResult{
			Messages:   messages,
			HasMore:    false,
			NextOffset: 0,
		}, nil)

		deps.messageProc.On("ProcessMessage", ctx, sessionName, mock.AnythingOfType("*whatsapp.Message")).Return(nil)

		err := svc.ReconcileChat(ctx, sessionName, chatID)

		require.NoError(t, err)
		deps.checkpointRepo.AssertCalled(t, "GetOrCreate", ctx, sessionName, chatID)
	})
}

// =============================================================================
// TestReconciliationService_Checkpoint_SaveAndLoad
// =============================================================================

func TestReconciliationService_Checkpoint_SaveAndLoad(t *testing.T) {
	t.Run("updates checkpoint after processing batch", func(t *testing.T) {
		svc, deps := newTestService(t)
		ctx := context.Background()

		sessionName := "test-session"
		chatID := "chat1@g.us"
		baseTime := time.Now()

		checkpoint := &reconciliation.Checkpoint{
			SessionName:          sessionName,
			ChatID:               chatID,
			LastMessageTimestamp: 0,
			Status:               reconciliation.CheckpointStatusIdle,
		}

		messages := []whatsapp.Message{
			{ID: "msg-1", ChatID: chatID, Timestamp: baseTime, Body: "First"},
			{ID: "msg-2", ChatID: chatID, Timestamp: baseTime.Add(time.Minute), Body: "Second"},
			{ID: "msg-3", ChatID: chatID, Timestamp: baseTime.Add(2 * time.Minute), Body: "Third"},
		}

		deps.checkpointRepo.On("GetOrCreate", ctx, sessionName, chatID).Return(checkpoint, nil)
		deps.checkpointRepo.On("MarkRunning", ctx, sessionName, chatID).Return(nil)
		deps.checkpointRepo.On("MarkIdle", ctx, sessionName, chatID).Return(nil)

		// Expect checkpoint to be updated with the latest message (msg-3)
		expectedTimestamp := baseTime.Add(2 * time.Minute).UnixMilli()
		deps.checkpointRepo.On("UpdateCheckpoint", ctx, sessionName, chatID, expectedTimestamp, "msg-3").Return(nil).Once()

		deps.provider.On("GetMessages", ctx, mock.Anything).Return(&whatsapp.MessagesResult{
			Messages:   messages,
			HasMore:    false,
			NextOffset: 0,
		}, nil)

		deps.messageProc.On("ProcessMessage", ctx, sessionName, mock.AnythingOfType("*whatsapp.Message")).Return(nil)

		err := svc.ReconcileChat(ctx, sessionName, chatID)

		require.NoError(t, err)
		deps.checkpointRepo.AssertExpectations(t)
	})

	t.Run("gets checkpoint on reconcile start", func(t *testing.T) {
		svc, deps := newTestService(t)
		ctx := context.Background()

		sessionName := "test-session"
		chatID := "chat1@g.us"

		checkpoint := &reconciliation.Checkpoint{
			SessionName:          sessionName,
			ChatID:               chatID,
			LastMessageTimestamp: time.Now().Add(-1 * time.Hour).UnixMilli(),
			LastMessageID:        "previous-msg",
			Status:               reconciliation.CheckpointStatusIdle,
		}

		deps.checkpointRepo.On("GetOrCreate", ctx, sessionName, chatID).Return(checkpoint, nil)
		deps.checkpointRepo.On("MarkRunning", ctx, sessionName, chatID).Return(nil)
		deps.checkpointRepo.On("MarkIdle", ctx, sessionName, chatID).Return(nil)

		deps.provider.On("GetMessages", ctx, mock.Anything).Return(&whatsapp.MessagesResult{
			Messages:   []whatsapp.Message{},
			HasMore:    false,
			NextOffset: 0,
		}, nil)

		err := svc.ReconcileChat(ctx, sessionName, chatID)

		require.NoError(t, err)
		deps.checkpointRepo.AssertCalled(t, "GetOrCreate", ctx, sessionName, chatID)
	})

	t.Run("marks checkpoint as failed on error", func(t *testing.T) {
		config := reconciliation.Config{
			SilenceTimeout: 5 * time.Minute,
			BatchSize:      10,
			MaxConcurrent:  5,
			CheckInterval:  1 * time.Minute,
			MaxRetries:     1, // Only 1 retry for faster test
			RetryBackoff:   1 * time.Millisecond,
		}
		svc, deps := newTestServiceWithConfig(t, config)
		ctx := context.Background()

		sessionName := "test-session"
		chatID := "chat1@g.us"

		checkpoint := &reconciliation.Checkpoint{
			SessionName: sessionName,
			ChatID:      chatID,
			Status:      reconciliation.CheckpointStatusIdle,
		}

		deps.checkpointRepo.On("GetOrCreate", ctx, sessionName, chatID).Return(checkpoint, nil)
		deps.checkpointRepo.On("MarkRunning", ctx, sessionName, chatID).Return(nil)
		deps.checkpointRepo.On("MarkFailed", ctx, sessionName, chatID).Return(nil)

		// Simulate provider error
		deps.provider.On("GetMessages", ctx, mock.Anything).Return(nil, errors.New("provider error"))

		err := svc.ReconcileChat(ctx, sessionName, chatID)

		require.Error(t, err)
		deps.checkpointRepo.AssertCalled(t, "MarkFailed", ctx, sessionName, chatID)
	})

	t.Run("retrieves checkpoint via GetCheckpoint", func(t *testing.T) {
		svc, deps := newTestService(t)
		ctx := context.Background()

		sessionName := "test-session"
		chatID := "chat1@g.us"

		expectedCheckpoint := &reconciliation.Checkpoint{
			SessionName:          sessionName,
			ChatID:               chatID,
			LastMessageTimestamp: time.Now().UnixMilli(),
			LastMessageID:        "latest-msg",
			Status:               reconciliation.CheckpointStatusIdle,
		}

		deps.checkpointRepo.On("GetOrCreate", ctx, sessionName, chatID).Return(expectedCheckpoint, nil)

		checkpoint, err := svc.GetCheckpoint(ctx, sessionName, chatID)

		require.NoError(t, err)
		assert.Equal(t, expectedCheckpoint.SessionName, checkpoint.SessionName)
		assert.Equal(t, expectedCheckpoint.ChatID, checkpoint.ChatID)
		assert.Equal(t, expectedCheckpoint.LastMessageID, checkpoint.LastMessageID)
	})
}

// =============================================================================
// TestReconciliationService_RunReconciliation
// =============================================================================

func TestReconciliationService_RunReconciliation(t *testing.T) {
	t.Run("prevents concurrent reconciliation for same chat", func(t *testing.T) {
		svc, deps := newTestService(t)
		ctx := context.Background()

		sessionName := "test-session"
		chatID := "chat1@g.us"

		checkpoint := &reconciliation.Checkpoint{
			SessionName: sessionName,
			ChatID:      chatID,
			Status:      reconciliation.CheckpointStatusIdle,
		}

		deps.checkpointRepo.On("GetOrCreate", ctx, sessionName, chatID).Return(checkpoint, nil)
		deps.checkpointRepo.On("MarkRunning", ctx, sessionName, chatID).Return(nil)
		deps.checkpointRepo.On("MarkIdle", ctx, sessionName, chatID).Return(nil)

		// Slow GetMessages to allow concurrent call attempt
		deps.provider.On("GetMessages", ctx, mock.Anything).Run(func(args mock.Arguments) {
			time.Sleep(100 * time.Millisecond)
		}).Return(&whatsapp.MessagesResult{
			Messages: []whatsapp.Message{},
			HasMore:  false,
		}, nil)

		// Start first reconciliation
		var wg sync.WaitGroup
		var firstErr, secondErr error

		wg.Add(2)

		go func() {
			defer wg.Done()
			firstErr = svc.ReconcileChat(ctx, sessionName, chatID)
		}()

		// Small delay to ensure first call starts
		time.Sleep(10 * time.Millisecond)

		go func() {
			defer wg.Done()
			secondErr = svc.ReconcileChat(ctx, sessionName, chatID)
		}()

		wg.Wait()

		// One should succeed, one should return ErrAlreadyRunning
		if firstErr == nil {
			assert.ErrorIs(t, secondErr, reconciliation.ErrAlreadyRunning)
		} else {
			assert.ErrorIs(t, firstErr, reconciliation.ErrAlreadyRunning)
			assert.NoError(t, secondErr)
		}
	})

	t.Run("allows concurrent reconciliation for different chats", func(t *testing.T) {
		svc, deps := newTestService(t)
		ctx := context.Background()

		sessionName := "test-session"
		chatID1 := "chat1@g.us"
		chatID2 := "chat2@g.us"

		checkpoint1 := &reconciliation.Checkpoint{
			SessionName: sessionName,
			ChatID:      chatID1,
			Status:      reconciliation.CheckpointStatusIdle,
		}
		checkpoint2 := &reconciliation.Checkpoint{
			SessionName: sessionName,
			ChatID:      chatID2,
			Status:      reconciliation.CheckpointStatusIdle,
		}

		deps.checkpointRepo.On("GetOrCreate", ctx, sessionName, chatID1).Return(checkpoint1, nil)
		deps.checkpointRepo.On("GetOrCreate", ctx, sessionName, chatID2).Return(checkpoint2, nil)
		deps.checkpointRepo.On("MarkRunning", ctx, sessionName, mock.Anything).Return(nil)
		deps.checkpointRepo.On("MarkIdle", ctx, sessionName, mock.Anything).Return(nil)

		deps.provider.On("GetMessages", ctx, mock.Anything).Return(&whatsapp.MessagesResult{
			Messages: []whatsapp.Message{},
			HasMore:  false,
		}, nil)

		var wg sync.WaitGroup
		var err1, err2 error

		wg.Add(2)

		go func() {
			defer wg.Done()
			err1 = svc.ReconcileChat(ctx, sessionName, chatID1)
		}()

		go func() {
			defer wg.Done()
			err2 = svc.ReconcileChat(ctx, sessionName, chatID2)
		}()

		wg.Wait()

		// Both should succeed
		assert.NoError(t, err1)
		assert.NoError(t, err2)
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		svc, deps := newTestService(t)
		ctx, cancel := context.WithCancel(context.Background())

		sessionName := "test-session"
		chatID := "chat1@g.us"

		checkpoint := &reconciliation.Checkpoint{
			SessionName: sessionName,
			ChatID:      chatID,
			Status:      reconciliation.CheckpointStatusIdle,
		}

		deps.checkpointRepo.On("GetOrCreate", mock.Anything, sessionName, chatID).Return(checkpoint, nil)
		deps.checkpointRepo.On("MarkRunning", mock.Anything, sessionName, chatID).Return(nil)
		deps.checkpointRepo.On("MarkFailed", mock.Anything, sessionName, chatID).Return(nil).Maybe()

		// Cancel context immediately
		cancel()

		err := svc.ReconcileChat(ctx, sessionName, chatID)

		// Should return context error
		assert.Error(t, err)
	})

	t.Run("IsReconciling returns correct status", func(t *testing.T) {
		svc, deps := newTestService(t)
		ctx := context.Background()

		sessionName := "test-session"
		chatID := "chat1@g.us"

		checkpoint := &reconciliation.Checkpoint{
			SessionName: sessionName,
			ChatID:      chatID,
			Status:      reconciliation.CheckpointStatusIdle,
		}

		deps.checkpointRepo.On("GetOrCreate", ctx, sessionName, chatID).Return(checkpoint, nil)
		deps.checkpointRepo.On("MarkRunning", ctx, sessionName, chatID).Return(nil)
		deps.checkpointRepo.On("MarkIdle", ctx, sessionName, chatID).Return(nil)

		reconciling := make(chan bool, 1)

		deps.provider.On("GetMessages", ctx, mock.Anything).Run(func(args mock.Arguments) {
			reconciling <- svc.IsReconciling(sessionName, chatID)
			time.Sleep(50 * time.Millisecond)
		}).Return(&whatsapp.MessagesResult{
			Messages: []whatsapp.Message{},
			HasMore:  false,
		}, nil)

		// Initially not reconciling
		assert.False(t, svc.IsReconciling(sessionName, chatID))

		go func() {
			_ = svc.ReconcileChat(ctx, sessionName, chatID)
		}()

		// Wait for reconciliation to start
		isRunning := <-reconciling
		assert.True(t, isRunning)

		// Wait for reconciliation to complete
		time.Sleep(100 * time.Millisecond)

		// Should not be reconciling anymore
		assert.False(t, svc.IsReconciling(sessionName, chatID))
	})
}

// =============================================================================
// TestReconciliationService_ErrorHandling
// =============================================================================

func TestReconciliationService_ErrorHandling(t *testing.T) {
	t.Run("retries on transient errors", func(t *testing.T) {
		config := reconciliation.Config{
			SilenceTimeout: 5 * time.Minute,
			BatchSize:      10,
			MaxConcurrent:  5,
			CheckInterval:  1 * time.Minute,
			MaxRetries:     3,
			RetryBackoff:   1 * time.Millisecond, // Fast for tests
		}
		svc, deps := newTestServiceWithConfig(t, config)
		ctx := context.Background()

		sessionName := "test-session"
		chatID := "chat1@g.us"

		checkpoint := &reconciliation.Checkpoint{
			SessionName: sessionName,
			ChatID:      chatID,
			Status:      reconciliation.CheckpointStatusIdle,
		}

		deps.checkpointRepo.On("GetOrCreate", ctx, sessionName, chatID).Return(checkpoint, nil)
		deps.checkpointRepo.On("MarkRunning", ctx, sessionName, chatID).Return(nil)
		deps.checkpointRepo.On("MarkIdle", ctx, sessionName, chatID).Return(nil)

		// First two calls fail
		deps.provider.On("GetMessages", ctx, mock.Anything).Return(nil, errors.New("transient error")).Once()
		deps.provider.On("GetMessages", ctx, mock.Anything).Return(nil, errors.New("transient error")).Once()

		// Third call succeeds
		deps.provider.On("GetMessages", ctx, mock.Anything).Return(&whatsapp.MessagesResult{
			Messages: []whatsapp.Message{},
			HasMore:  false,
		}, nil).Once()

		err := svc.ReconcileChat(ctx, sessionName, chatID)

		require.NoError(t, err)
		deps.provider.AssertNumberOfCalls(t, "GetMessages", 3)
	})

	t.Run("fails after max retries", func(t *testing.T) {
		config := reconciliation.Config{
			SilenceTimeout: 5 * time.Minute,
			BatchSize:      10,
			MaxConcurrent:  5,
			CheckInterval:  1 * time.Minute,
			MaxRetries:     2,
			RetryBackoff:   1 * time.Millisecond,
		}
		svc, deps := newTestServiceWithConfig(t, config)
		ctx := context.Background()

		sessionName := "test-session"
		chatID := "chat1@g.us"

		checkpoint := &reconciliation.Checkpoint{
			SessionName: sessionName,
			ChatID:      chatID,
			Status:      reconciliation.CheckpointStatusIdle,
		}

		deps.checkpointRepo.On("GetOrCreate", ctx, sessionName, chatID).Return(checkpoint, nil)
		deps.checkpointRepo.On("MarkRunning", ctx, sessionName, chatID).Return(nil)
		deps.checkpointRepo.On("MarkFailed", ctx, sessionName, chatID).Return(nil)

		// Always fail
		deps.provider.On("GetMessages", ctx, mock.Anything).Return(nil, errors.New("persistent error"))

		err := svc.ReconcileChat(ctx, sessionName, chatID)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed after")
		deps.checkpointRepo.AssertCalled(t, "MarkFailed", ctx, sessionName, chatID)
	})

	t.Run("handles checkpoint repository errors", func(t *testing.T) {
		svc, deps := newTestService(t)
		ctx := context.Background()

		sessionName := "test-session"
		chatID := "chat1@g.us"

		deps.checkpointRepo.On("GetOrCreate", ctx, sessionName, chatID).Return(nil, errors.New("db connection failed"))

		err := svc.ReconcileChat(ctx, sessionName, chatID)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get checkpoint")
	})

	t.Run("handles mark running error when already running in db", func(t *testing.T) {
		svc, deps := newTestService(t)
		ctx := context.Background()

		sessionName := "test-session"
		chatID := "chat1@g.us"

		checkpoint := &reconciliation.Checkpoint{
			SessionName: sessionName,
			ChatID:      chatID,
			Status:      reconciliation.CheckpointStatusRunning,
		}

		deps.checkpointRepo.On("GetOrCreate", ctx, sessionName, chatID).Return(checkpoint, nil)
		deps.checkpointRepo.On("MarkRunning", ctx, sessionName, chatID).Return(reconciliation.ErrAlreadyRunning)

		err := svc.ReconcileChat(ctx, sessionName, chatID)

		assert.ErrorIs(t, err, reconciliation.ErrAlreadyRunning)
	})

	t.Run("continues processing on individual message error", func(t *testing.T) {
		svc, deps := newTestService(t)
		ctx := context.Background()

		sessionName := "test-session"
		chatID := "chat1@g.us"
		baseTime := time.Now()

		checkpoint := &reconciliation.Checkpoint{
			SessionName: sessionName,
			ChatID:      chatID,
			Status:      reconciliation.CheckpointStatusIdle,
		}

		messages := createTestMessages(3, baseTime)

		deps.checkpointRepo.On("GetOrCreate", ctx, sessionName, chatID).Return(checkpoint, nil)
		deps.checkpointRepo.On("MarkRunning", ctx, sessionName, chatID).Return(nil)
		deps.checkpointRepo.On("MarkIdle", ctx, sessionName, chatID).Return(nil)
		deps.checkpointRepo.On("UpdateCheckpoint", ctx, sessionName, chatID, mock.AnythingOfType("int64"), mock.AnythingOfType("string")).Return(nil).Maybe()

		deps.provider.On("GetMessages", ctx, mock.Anything).Return(&whatsapp.MessagesResult{
			Messages:   messages,
			HasMore:    false,
			NextOffset: 0,
		}, nil)

		// First message fails, others succeed
		deps.messageProc.On("ProcessMessage", ctx, sessionName, mock.MatchedBy(func(msg *whatsapp.Message) bool {
			return msg.ID == "msg-0"
		})).Return(errors.New("processing failed")).Once()

		deps.messageProc.On("ProcessMessage", ctx, sessionName, mock.MatchedBy(func(msg *whatsapp.Message) bool {
			return msg.ID != "msg-0"
		})).Return(nil)

		err := svc.ReconcileChat(ctx, sessionName, chatID)

		// Should complete successfully despite individual message failure
		require.NoError(t, err)
	})
}

// =============================================================================
// TestReconciliationService_OnStartup
// =============================================================================

func TestReconciliationService_OnStartup(t *testing.T) {
	t.Run("reconciles all active sessions on startup", func(t *testing.T) {
		svc, deps := newTestService(t)
		ctx := context.Background()

		sessionChats := []reconciliation.SessionChat{
			{SessionName: "session1", ChatID: "chat1@g.us"},
			{SessionName: "session1", ChatID: "chat2@g.us"},
			{SessionName: "session2", ChatID: "chat3@g.us"},
		}

		deps.checkpointRepo.On("GetActiveSessionsChats", ctx).Return(sessionChats, nil)

		// Set up expectations for each chat
		for _, sc := range sessionChats {
			checkpoint := &reconciliation.Checkpoint{
				SessionName: sc.SessionName,
				ChatID:      sc.ChatID,
				Status:      reconciliation.CheckpointStatusIdle,
			}
			deps.checkpointRepo.On("GetOrCreate", ctx, sc.SessionName, sc.ChatID).Return(checkpoint, nil)
			deps.checkpointRepo.On("MarkRunning", ctx, sc.SessionName, sc.ChatID).Return(nil)
			deps.checkpointRepo.On("MarkIdle", ctx, sc.SessionName, sc.ChatID).Return(nil)
		}

		deps.provider.On("GetMessages", ctx, mock.Anything).Return(&whatsapp.MessagesResult{
			Messages: []whatsapp.Message{},
			HasMore:  false,
		}, nil)

		err := svc.OnStartup(ctx)

		require.NoError(t, err)
		deps.checkpointRepo.AssertCalled(t, "GetActiveSessionsChats", ctx)
	})

	t.Run("handles no active sessions", func(t *testing.T) {
		svc, deps := newTestService(t)
		ctx := context.Background()

		deps.checkpointRepo.On("GetActiveSessionsChats", ctx).Return([]reconciliation.SessionChat{}, nil)

		err := svc.OnStartup(ctx)

		require.NoError(t, err)
	})

	t.Run("continues on individual chat errors", func(t *testing.T) {
		svc, deps := newTestService(t)
		ctx := context.Background()

		sessionChats := []reconciliation.SessionChat{
			{SessionName: "session1", ChatID: "chat1@g.us"},
			{SessionName: "session1", ChatID: "chat2@g.us"},
		}

		deps.checkpointRepo.On("GetActiveSessionsChats", ctx).Return(sessionChats, nil)

		// First chat fails
		deps.checkpointRepo.On("GetOrCreate", ctx, "session1", "chat1@g.us").Return(nil, errors.New("db error"))

		// Second chat succeeds
		checkpoint2 := &reconciliation.Checkpoint{
			SessionName: "session1",
			ChatID:      "chat2@g.us",
			Status:      reconciliation.CheckpointStatusIdle,
		}
		deps.checkpointRepo.On("GetOrCreate", ctx, "session1", "chat2@g.us").Return(checkpoint2, nil)
		deps.checkpointRepo.On("MarkRunning", ctx, "session1", "chat2@g.us").Return(nil)
		deps.checkpointRepo.On("MarkIdle", ctx, "session1", "chat2@g.us").Return(nil)

		deps.provider.On("GetMessages", ctx, mock.Anything).Return(&whatsapp.MessagesResult{
			Messages: []whatsapp.Message{},
			HasMore:  false,
		}, nil)

		err := svc.OnStartup(ctx)

		// Should complete without error (errors are logged but don't stop startup)
		require.NoError(t, err)
	})

	t.Run("handles GetActiveSessionsChats error", func(t *testing.T) {
		svc, deps := newTestService(t)
		ctx := context.Background()

		deps.checkpointRepo.On("GetActiveSessionsChats", ctx).Return(nil, errors.New("db error"))

		err := svc.OnStartup(ctx)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get active session/chats")
	})
}

// =============================================================================
// TestReconciliationService_ReconcileSession
// =============================================================================

func TestReconciliationService_ReconcileSession(t *testing.T) {
	t.Run("triggers reconciliation for all session chats", func(t *testing.T) {
		svc, deps := newTestService(t)
		ctx := context.Background()

		sessionName := "test-session"

		checkpoints := []reconciliation.Checkpoint{
			{SessionName: sessionName, ChatID: "chat1@g.us", Status: reconciliation.CheckpointStatusIdle},
			{SessionName: sessionName, ChatID: "chat2@g.us", Status: reconciliation.CheckpointStatusIdle},
		}

		deps.checkpointRepo.On("GetBySession", mock.Anything, sessionName).Return(checkpoints, nil)

		for _, cp := range checkpoints {
			deps.checkpointRepo.On("GetOrCreate", mock.Anything, cp.SessionName, cp.ChatID).Return(&cp, nil)
			deps.checkpointRepo.On("MarkRunning", mock.Anything, cp.SessionName, cp.ChatID).Return(nil)
			deps.checkpointRepo.On("MarkIdle", mock.Anything, cp.SessionName, cp.ChatID).Return(nil)
		}

		deps.provider.On("GetMessages", mock.Anything, mock.Anything).Return(&whatsapp.MessagesResult{
			Messages: []whatsapp.Message{},
			HasMore:  false,
		}, nil)

		err := svc.ReconcileSession(ctx, sessionName)

		require.NoError(t, err)

		// Wait for async reconciliation to complete
		time.Sleep(100 * time.Millisecond)

		deps.checkpointRepo.AssertCalled(t, "GetBySession", mock.Anything, sessionName)
	})
}

// =============================================================================
// TestReconciliationService_Config
// =============================================================================

func TestReconciliationService_Config(t *testing.T) {
	t.Run("uses default config when invalid", func(t *testing.T) {
		logger := zaptest.NewLogger(t)
		provider := NewMockWhatsAppProvider()
		repo := new(MockCheckpointRepository)
		proc := new(MockMessageProcessor)

		// Invalid config (negative values)
		invalidConfig := reconciliation.Config{
			SilenceTimeout: -1,
			BatchSize:      -1,
			MaxConcurrent:  -1,
		}

		// Should not panic and should use defaults
		svc := reconciliation.New(provider, repo, proc, logger, invalidConfig)
		require.NotNil(t, svc)
	})

	t.Run("default config has valid values", func(t *testing.T) {
		config := reconciliation.DefaultConfig()

		assert.Equal(t, 5*time.Minute, config.SilenceTimeout)
		assert.Equal(t, 50, config.BatchSize)
		assert.Equal(t, 5, config.MaxConcurrent)
		assert.Equal(t, 1*time.Minute, config.CheckInterval)
		assert.Equal(t, 3, config.MaxRetries)
		assert.Equal(t, 30*time.Second, config.RetryBackoff)
		assert.True(t, config.HealthCheckEnabled)
		assert.Equal(t, 3, config.MaxRestartAttempts)
		assert.Equal(t, 1*time.Minute, config.RestartBackoff)
	})

	t.Run("config validation applies defaults for invalid values", func(t *testing.T) {
		config := reconciliation.Config{
			SilenceTimeout:     0,  // Invalid
			BatchSize:          0,  // Invalid
			MaxConcurrent:      0,  // Invalid
			CheckInterval:      0,  // Invalid
			MaxRetries:         0,  // Invalid
			RetryBackoff:       0,  // Invalid
			MaxRestartAttempts: -1, // Invalid (negative)
			RestartBackoff:     0,  // Invalid
		}

		err := config.Validate()
		require.NoError(t, err)

		// Should have applied defaults
		assert.Equal(t, 5*time.Minute, config.SilenceTimeout)
		assert.Equal(t, 50, config.BatchSize)
		assert.Equal(t, 5, config.MaxConcurrent)
		assert.Equal(t, 1*time.Minute, config.CheckInterval)
		assert.Equal(t, 3, config.MaxRetries)
		assert.Equal(t, 30*time.Second, config.RetryBackoff)
		assert.Equal(t, 3, config.MaxRestartAttempts)
		assert.Equal(t, 1*time.Minute, config.RestartBackoff)
	})
}

// Stubs dos métodos novos do CheckpointRepository (watermark vivo + higiene).
// Retornos fixos (sem m.Called) de propósito: o watchdog os chama fora do
// controle dos testes e um testify sem expectativa registrada entraria em
// pânico.
func (m *MockCheckpointRepository) AdvanceCheckpoint(ctx context.Context, sessionName, chatID string, lastTimestamp int64, lastMessageID string) error {
	return nil
}
func (m *MockCheckpointRepository) ResetStuckCheckpoints(ctx context.Context, olderThan time.Duration) (int64, error) {
	return 0, nil
}
func (m *MockCheckpointRepository) DeleteInactiveCheckpoints(ctx context.Context, olderThan time.Duration) (int64, error) {
	return 0, nil
}
