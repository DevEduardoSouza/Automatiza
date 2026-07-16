package workers_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/services/reconciliation"
	"automatiza-go/internal/workers"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// =============================================================================
// Mock Implementations
// =============================================================================

// mockCheckpointRepo implements reconciliation.CheckpointRepository for testing.
type mockCheckpointRepo struct {
	mock.Mock
}

func (m *mockCheckpointRepo) GetOrCreate(ctx context.Context, sessionName, chatID string) (*reconciliation.Checkpoint, error) {
	args := m.Called(ctx, sessionName, chatID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*reconciliation.Checkpoint), args.Error(1)
}

func (m *mockCheckpointRepo) MarkRunning(ctx context.Context, sessionName, chatID string) error {
	args := m.Called(ctx, sessionName, chatID)
	return args.Error(0)
}

func (m *mockCheckpointRepo) MarkIdle(ctx context.Context, sessionName, chatID string) error {
	args := m.Called(ctx, sessionName, chatID)
	return args.Error(0)
}

func (m *mockCheckpointRepo) MarkFailed(ctx context.Context, sessionName, chatID string) error {
	args := m.Called(ctx, sessionName, chatID)
	return args.Error(0)
}

func (m *mockCheckpointRepo) UpdateCheckpoint(ctx context.Context, sessionName, chatID string, lastTimestamp int64, lastMessageID string) error {
	args := m.Called(ctx, sessionName, chatID, lastTimestamp, lastMessageID)
	return args.Error(0)
}

func (m *mockCheckpointRepo) GetActiveSessionsChats(ctx context.Context) ([]reconciliation.SessionChat, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]reconciliation.SessionChat), args.Error(1)
}

func (m *mockCheckpointRepo) GetBySession(ctx context.Context, sessionName string) ([]reconciliation.Checkpoint, error) {
	args := m.Called(ctx, sessionName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]reconciliation.Checkpoint), args.Error(1)
}

// mockMsgProcessor implements reconciliation.MessageProcessor for testing.
type mockMsgProcessor struct {
	mock.Mock
}

func (m *mockMsgProcessor) ProcessMessage(ctx context.Context, sessionName string, msg *whatsapp.Message) error {
	args := m.Called(ctx, sessionName, msg)
	return args.Error(0)
}

// =============================================================================
// Test Helpers
// =============================================================================

// newTestTask creates an asynq.Task with the given type name and payload bytes.
func newTestTask(typeName string, payload []byte) *asynq.Task {
	return asynq.NewTask(typeName, payload)
}

// mustMarshal marshals v to JSON or panics.
func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	require.NoError(t, err, "failed to marshal test payload")
	return data
}

// newTestService creates a reconciliation.Service with the given mocks and the FakeAdapter.
// It uses DefaultConfig with zero RetryBackoff for fast tests.
func newTestService(
	adapter *whatsapp.FakeAdapter,
	cpRepo *mockCheckpointRepo,
	msgProc *mockMsgProcessor,
) *reconciliation.Service {
	cfg := reconciliation.DefaultConfig()
	cfg.RetryBackoff = 0 // No backoff delay in tests
	return reconciliation.New(adapter, cpRepo, msgProc, zap.NewNop(), cfg)
}

// =============================================================================
// Tests: NewReconciliationWorker
// =============================================================================

func TestNewReconciliationWorker(t *testing.T) {
	t.Parallel()

	adapter := whatsapp.NewFakeAdapter()
	cpRepo := new(mockCheckpointRepo)
	msgProc := new(mockMsgProcessor)
	svc := newTestService(adapter, cpRepo, msgProc)

	worker := workers.NewReconciliationWorker(svc, zap.NewNop())

	assert.NotNil(t, worker, "worker should not be nil")
}

// =============================================================================
// Tests: ProcessChat
// =============================================================================

func TestProcessChat_UnmarshalError(t *testing.T) {
	t.Parallel()

	adapter := whatsapp.NewFakeAdapter()
	cpRepo := new(mockCheckpointRepo)
	msgProc := new(mockMsgProcessor)
	svc := newTestService(adapter, cpRepo, msgProc)
	worker := workers.NewReconciliationWorker(svc, zap.NewNop())

	task := newTestTask("test:reconcile:chat", []byte("not-json{{{"))

	err := worker.ProcessChat(context.Background(), task)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal payload")
}

func TestProcessChat_MissingSessionName(t *testing.T) {
	t.Parallel()

	adapter := whatsapp.NewFakeAdapter()
	cpRepo := new(mockCheckpointRepo)
	msgProc := new(mockMsgProcessor)
	svc := newTestService(adapter, cpRepo, msgProc)
	worker := workers.NewReconciliationWorker(svc, zap.NewNop())

	payload := mustMarshal(t, workers.ReconcileChatPayload{
		SessionName: "",
		ChatID:      "chat@s.whatsapp.net",
	})
	task := newTestTask("test:reconcile:chat", payload)

	err := worker.ProcessChat(context.Background(), task)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing session_name")
}

func TestProcessChat_MissingChatID(t *testing.T) {
	t.Parallel()

	adapter := whatsapp.NewFakeAdapter()
	cpRepo := new(mockCheckpointRepo)
	msgProc := new(mockMsgProcessor)
	svc := newTestService(adapter, cpRepo, msgProc)
	worker := workers.NewReconciliationWorker(svc, zap.NewNop())

	payload := mustMarshal(t, workers.ReconcileChatPayload{
		SessionName: "test-session",
		ChatID:      "",
	})
	task := newTestTask("test:reconcile:chat", payload)

	err := worker.ProcessChat(context.Background(), task)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing chat_id")
}

func TestProcessChat_Success(t *testing.T) {
	t.Parallel()

	adapter := whatsapp.NewFakeAdapter()
	cpRepo := new(mockCheckpointRepo)
	msgProc := new(mockMsgProcessor)
	svc := newTestService(adapter, cpRepo, msgProc)
	worker := workers.NewReconciliationWorker(svc, zap.NewNop())

	sessionName := "test-session"
	chatID := "5511999998888@s.whatsapp.net"

	// Set up the FakeAdapter with a connected session (needed for health check path)
	adapter.SetSession(sessionName, &whatsapp.SessionInfo{
		Name:   sessionName,
		Status: whatsapp.SessionStatusConnected,
	})

	// Set up empty messages so reconciliation completes immediately
	adapter.SetMessages(sessionName, chatID, []whatsapp.Message{})

	// Configure checkpoint repo mock for the reconcile flow
	cpRepo.On("GetOrCreate", mock.Anything, sessionName, chatID).
		Return(&reconciliation.Checkpoint{
			SessionName:          sessionName,
			ChatID:               chatID,
			LastMessageTimestamp: 0,
			LastMessageID:        "",
			Status:               reconciliation.CheckpointStatusIdle,
		}, nil)
	cpRepo.On("MarkRunning", mock.Anything, sessionName, chatID).Return(nil)
	cpRepo.On("MarkIdle", mock.Anything, sessionName, chatID).Return(nil)

	payload := mustMarshal(t, workers.ReconcileChatPayload{
		SessionName: sessionName,
		ChatID:      chatID,
	})
	task := newTestTask("test:reconcile:chat", payload)

	err := worker.ProcessChat(context.Background(), task)

	assert.NoError(t, err)
	cpRepo.AssertCalled(t, "GetOrCreate", mock.Anything, sessionName, chatID)
	cpRepo.AssertCalled(t, "MarkRunning", mock.Anything, sessionName, chatID)
	cpRepo.AssertCalled(t, "MarkIdle", mock.Anything, sessionName, chatID)
}

func TestProcessChat_ErrAlreadyRunning(t *testing.T) {
	t.Parallel()

	adapter := whatsapp.NewFakeAdapter()
	cpRepo := new(mockCheckpointRepo)
	msgProc := new(mockMsgProcessor)
	svc := newTestService(adapter, cpRepo, msgProc)
	worker := workers.NewReconciliationWorker(svc, zap.NewNop())

	sessionName := "test-session"
	chatID := "5511999998888@s.whatsapp.net"

	adapter.SetSession(sessionName, &whatsapp.SessionInfo{
		Name:   sessionName,
		Status: whatsapp.SessionStatusConnected,
	})

	// GetOrCreate succeeds, but MarkRunning returns ErrAlreadyRunning
	// (simulating that the DB checkpoint is already in "running" state)
	cpRepo.On("GetOrCreate", mock.Anything, sessionName, chatID).
		Return(&reconciliation.Checkpoint{
			SessionName: sessionName,
			ChatID:      chatID,
			Status:      reconciliation.CheckpointStatusRunning,
		}, nil)
	cpRepo.On("MarkRunning", mock.Anything, sessionName, chatID).
		Return(reconciliation.ErrAlreadyRunning)

	payload := mustMarshal(t, workers.ReconcileChatPayload{
		SessionName: sessionName,
		ChatID:      chatID,
	})
	task := newTestTask("test:reconcile:chat", payload)

	// The worker should return nil (not retry) when ErrAlreadyRunning is received
	err := worker.ProcessChat(context.Background(), task)

	assert.NoError(t, err, "ErrAlreadyRunning should be swallowed by the worker")
	cpRepo.AssertCalled(t, "GetOrCreate", mock.Anything, sessionName, chatID)
	cpRepo.AssertCalled(t, "MarkRunning", mock.Anything, sessionName, chatID)
}

func TestProcessChat_ServiceError(t *testing.T) {
	t.Parallel()

	adapter := whatsapp.NewFakeAdapter()
	cpRepo := new(mockCheckpointRepo)
	msgProc := new(mockMsgProcessor)
	svc := newTestService(adapter, cpRepo, msgProc)
	worker := workers.NewReconciliationWorker(svc, zap.NewNop())

	sessionName := "test-session"
	chatID := "5511999998888@s.whatsapp.net"

	adapter.SetSession(sessionName, &whatsapp.SessionInfo{
		Name:   sessionName,
		Status: whatsapp.SessionStatusConnected,
	})

	// GetOrCreate fails with a database error
	dbErr := errors.New("database connection lost")
	cpRepo.On("GetOrCreate", mock.Anything, sessionName, chatID).
		Return(nil, dbErr)

	payload := mustMarshal(t, workers.ReconcileChatPayload{
		SessionName: sessionName,
		ChatID:      chatID,
	})
	task := newTestTask("test:reconcile:chat", payload)

	err := worker.ProcessChat(context.Background(), task)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get checkpoint")
}

// =============================================================================
// Tests: ProcessSession
// =============================================================================

func TestProcessSession_UnmarshalError(t *testing.T) {
	t.Parallel()

	adapter := whatsapp.NewFakeAdapter()
	cpRepo := new(mockCheckpointRepo)
	msgProc := new(mockMsgProcessor)
	svc := newTestService(adapter, cpRepo, msgProc)
	worker := workers.NewReconciliationWorker(svc, zap.NewNop())

	task := newTestTask("test:reconcile:session", []byte("not-valid-json"))

	err := worker.ProcessSession(context.Background(), task)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal payload")
}

func TestProcessSession_MissingSessionName(t *testing.T) {
	t.Parallel()

	adapter := whatsapp.NewFakeAdapter()
	cpRepo := new(mockCheckpointRepo)
	msgProc := new(mockMsgProcessor)
	svc := newTestService(adapter, cpRepo, msgProc)
	worker := workers.NewReconciliationWorker(svc, zap.NewNop())

	payload := mustMarshal(t, workers.ReconcileSessionPayload{
		SessionName: "",
	})
	task := newTestTask("test:reconcile:session", payload)

	err := worker.ProcessSession(context.Background(), task)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing session_name")
}

func TestProcessSession_Success(t *testing.T) {
	t.Parallel()

	adapter := whatsapp.NewFakeAdapter()
	cpRepo := new(mockCheckpointRepo)
	msgProc := new(mockMsgProcessor)
	svc := newTestService(adapter, cpRepo, msgProc)
	worker := workers.NewReconciliationWorker(svc, zap.NewNop())

	sessionName := "test-session"

	// ReconcileSession fires a goroutine via SafeGo internally and returns nil.
	// The goroutine calls GetBySession, so set up that mock to avoid blocking.
	cpRepo.On("GetBySession", mock.Anything, sessionName).
		Return([]reconciliation.Checkpoint{}, nil)

	payload := mustMarshal(t, workers.ReconcileSessionPayload{
		SessionName: sessionName,
	})
	task := newTestTask("test:reconcile:session", payload)

	err := worker.ProcessSession(context.Background(), task)

	assert.NoError(t, err)

	// Give the internal goroutine a moment to complete its work
	time.Sleep(50 * time.Millisecond)
	cpRepo.AssertCalled(t, "GetBySession", mock.Anything, sessionName)
}

func TestProcessSession_ServiceError(t *testing.T) {
	t.Parallel()

	// ReconcileSession always returns nil since it uses SafeGo (fire-and-forget).
	// Therefore, a "service error" at the ProcessSession level would only happen
	// if ReconcileSession itself returns an error. Looking at the implementation,
	// ReconcileSession always returns nil. This test verifies that behavior:
	// even if the background goroutine might fail, the worker returns nil.

	adapter := whatsapp.NewFakeAdapter()
	cpRepo := new(mockCheckpointRepo)
	msgProc := new(mockMsgProcessor)
	svc := newTestService(adapter, cpRepo, msgProc)
	worker := workers.NewReconciliationWorker(svc, zap.NewNop())

	sessionName := "test-session"

	// Make GetBySession fail so the background goroutine encounters an error
	cpRepo.On("GetBySession", mock.Anything, sessionName).
		Return(nil, errors.New("db error"))

	payload := mustMarshal(t, workers.ReconcileSessionPayload{
		SessionName: sessionName,
	})
	task := newTestTask("test:reconcile:session", payload)

	err := worker.ProcessSession(context.Background(), task)

	// Worker still returns nil because ReconcileSession always returns nil
	assert.NoError(t, err)

	// Give the internal goroutine time to complete
	time.Sleep(50 * time.Millisecond)
}

// =============================================================================
// Tests: Wrapper Functions
// =============================================================================

func TestProcessChatWrapper(t *testing.T) {
	t.Parallel()

	adapter := whatsapp.NewFakeAdapter()
	cpRepo := new(mockCheckpointRepo)
	msgProc := new(mockMsgProcessor)
	svc := newTestService(adapter, cpRepo, msgProc)
	worker := workers.NewReconciliationWorker(svc, zap.NewNop())

	wrapper := worker.ProcessChatWrapper()
	require.NotNil(t, wrapper, "wrapper function should not be nil")

	// Verify the wrapper delegates correctly by calling with invalid JSON
	// (which triggers the unmarshal error path)
	task := newTestTask("test:reconcile:chat", []byte("invalid"))

	err := wrapper(context.Background(), task)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal payload")
}

func TestProcessChatWrapper_Success(t *testing.T) {
	t.Parallel()

	adapter := whatsapp.NewFakeAdapter()
	cpRepo := new(mockCheckpointRepo)
	msgProc := new(mockMsgProcessor)
	svc := newTestService(adapter, cpRepo, msgProc)
	worker := workers.NewReconciliationWorker(svc, zap.NewNop())

	sessionName := "wrapper-session"
	chatID := "chat@s.whatsapp.net"

	adapter.SetSession(sessionName, &whatsapp.SessionInfo{
		Name:   sessionName,
		Status: whatsapp.SessionStatusConnected,
	})
	adapter.SetMessages(sessionName, chatID, []whatsapp.Message{})

	cpRepo.On("GetOrCreate", mock.Anything, sessionName, chatID).
		Return(&reconciliation.Checkpoint{
			SessionName: sessionName,
			ChatID:      chatID,
			Status:      reconciliation.CheckpointStatusIdle,
		}, nil)
	cpRepo.On("MarkRunning", mock.Anything, sessionName, chatID).Return(nil)
	cpRepo.On("MarkIdle", mock.Anything, sessionName, chatID).Return(nil)

	payload := mustMarshal(t, workers.ReconcileChatPayload{
		SessionName: sessionName,
		ChatID:      chatID,
	})
	task := newTestTask("test:reconcile:chat", payload)

	wrapper := worker.ProcessChatWrapper()
	err := wrapper(context.Background(), task)

	assert.NoError(t, err)
	cpRepo.AssertCalled(t, "MarkIdle", mock.Anything, sessionName, chatID)
}

func TestProcessSessionWrapper(t *testing.T) {
	t.Parallel()

	adapter := whatsapp.NewFakeAdapter()
	cpRepo := new(mockCheckpointRepo)
	msgProc := new(mockMsgProcessor)
	svc := newTestService(adapter, cpRepo, msgProc)
	worker := workers.NewReconciliationWorker(svc, zap.NewNop())

	wrapper := worker.ProcessSessionWrapper()
	require.NotNil(t, wrapper, "wrapper function should not be nil")

	// Verify the wrapper delegates correctly by calling with invalid JSON
	task := newTestTask("test:reconcile:session", []byte("invalid"))

	err := wrapper(context.Background(), task)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal payload")
}

func TestProcessSessionWrapper_Success(t *testing.T) {
	t.Parallel()

	adapter := whatsapp.NewFakeAdapter()
	cpRepo := new(mockCheckpointRepo)
	msgProc := new(mockMsgProcessor)
	svc := newTestService(adapter, cpRepo, msgProc)
	worker := workers.NewReconciliationWorker(svc, zap.NewNop())

	sessionName := "wrapper-session"

	cpRepo.On("GetBySession", mock.Anything, sessionName).
		Return([]reconciliation.Checkpoint{}, nil)

	payload := mustMarshal(t, workers.ReconcileSessionPayload{
		SessionName: sessionName,
	})
	task := newTestTask("test:reconcile:session", payload)

	wrapper := worker.ProcessSessionWrapper()
	err := wrapper(context.Background(), task)

	assert.NoError(t, err)

	// Give the internal goroutine time to complete
	time.Sleep(50 * time.Millisecond)
	cpRepo.AssertCalled(t, "GetBySession", mock.Anything, sessionName)
}

// =============================================================================
// Tests: ProcessSession - Error and Cancellation Paths
// =============================================================================

func TestProcessSession_ReconcileError(t *testing.T) {
	t.Parallel()

	// ReconcileSession internally uses SafeGo (fire-and-forget) and always returns nil.
	// Therefore the error branch at reconciliation_worker.go:110-114 is unreachable
	// with the current service implementation. This test documents that behavior:
	// even when the background goroutine encounters an error (e.g., GetBySession fails),
	// ProcessSession still returns nil because ReconcileSession itself never errors.

	adapter := whatsapp.NewFakeAdapter()
	cpRepo := new(mockCheckpointRepo)
	msgProc := new(mockMsgProcessor)
	svc := newTestService(adapter, cpRepo, msgProc)
	worker := workers.NewReconciliationWorker(svc, zap.NewNop())

	sessionName := "error-session"

	// Simulate an adapter-level error so that if reconcileSession does run,
	// its internal calls (GetBySession → reconcileChat → provider calls) would fail.
	adapter.SimulateError = errors.New("session unavailable")

	// GetBySession will fail due to internal goroutine, but ReconcileSession returns nil
	cpRepo.On("GetBySession", mock.Anything, sessionName).
		Return(nil, errors.New("database connection lost"))

	payload := mustMarshal(t, workers.ReconcileSessionPayload{
		SessionName: sessionName,
	})
	task := newTestTask("test:reconcile:session", payload)

	err := worker.ProcessSession(context.Background(), task)

	// Worker returns nil because ReconcileSession uses SafeGo and always returns nil.
	// The error branch in ProcessSession (lines 110-114) is dead code.
	assert.NoError(t, err, "ProcessSession should return nil even when internal goroutine fails")

	// Give the internal goroutine time to complete (and fail silently)
	time.Sleep(50 * time.Millisecond)
}

func TestProcessSession_ContextCancelled(t *testing.T) {
	t.Parallel()

	// Test that ProcessSession propagates context cancellation correctly.
	// Since ReconcileSession always returns nil (fire-and-forget via SafeGo),
	// the context cancellation only affects the background goroutine, not the
	// return value of ProcessSession.

	adapter := whatsapp.NewFakeAdapter()
	cpRepo := new(mockCheckpointRepo)
	msgProc := new(mockMsgProcessor)
	svc := newTestService(adapter, cpRepo, msgProc)
	worker := workers.NewReconciliationWorker(svc, zap.NewNop())

	sessionName := "cancelled-session"

	// Set up mock - the cancelled context may or may not reach GetBySession
	cpRepo.On("GetBySession", mock.Anything, sessionName).
		Return([]reconciliation.Checkpoint{
			{
				SessionName: sessionName,
				ChatID:      "chat@s.whatsapp.net",
				Status:      reconciliation.CheckpointStatusIdle,
			},
		}, nil).Maybe()

	// Also set up mocks that ReconcileChat might invoke if the goroutine runs
	cpRepo.On("GetOrCreate", mock.Anything, sessionName, "chat@s.whatsapp.net").
		Return(&reconciliation.Checkpoint{
			SessionName: sessionName,
			ChatID:      "chat@s.whatsapp.net",
			Status:      reconciliation.CheckpointStatusIdle,
		}, nil).Maybe()
	cpRepo.On("MarkRunning", mock.Anything, sessionName, "chat@s.whatsapp.net").
		Return(nil).Maybe()
	cpRepo.On("MarkIdle", mock.Anything, sessionName, "chat@s.whatsapp.net").
		Return(nil).Maybe()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	payload := mustMarshal(t, workers.ReconcileSessionPayload{
		SessionName: sessionName,
	})
	task := newTestTask("test:reconcile:session", payload)

	err := worker.ProcessSession(ctx, task)

	// ReconcileSession always returns nil regardless of context state,
	// because it delegates to SafeGo. The cancelled context only affects
	// the background goroutine's internal operations.
	assert.NoError(t, err, "ProcessSession should return nil even with cancelled context")

	// Give the internal goroutine time to detect cancellation
	time.Sleep(50 * time.Millisecond)
}

// Stubs dos métodos novos do CheckpointRepository (watermark vivo + higiene).
func (m *mockCheckpointRepo) AdvanceCheckpoint(ctx context.Context, sessionName, chatID string, lastTimestamp int64, lastMessageID string) error {
	return nil
}
func (m *mockCheckpointRepo) ResetStuckCheckpoints(ctx context.Context, olderThan time.Duration) (int64, error) {
	return 0, nil
}
func (m *mockCheckpointRepo) DeleteInactiveCheckpoints(ctx context.Context, olderThan time.Duration) (int64, error) {
	return 0, nil
}
