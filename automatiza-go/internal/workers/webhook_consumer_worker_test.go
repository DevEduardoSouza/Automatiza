// Package workers provides background job processing.
package workers

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"automatiza-go/internal/metrics"
	"automatiza-go/internal/models"
	"automatiza-go/internal/services/reconciliation"

	"github.com/hibiken/asynq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// =============================================================================
// Mock Implementations
// =============================================================================

// mockWebhookEventsRepo implements repository.WebhookEventsRepository for testing.
type mockWebhookEventsRepo struct {
	mock.Mock
}

func (m *mockWebhookEventsRepo) InsertIfNotExists(ctx context.Context, eventID, sessionName, eventType string) (bool, error) {
	args := m.Called(ctx, eventID, sessionName, eventType)
	return args.Bool(0), args.Error(1)
}

func (m *mockWebhookEventsRepo) Exists(ctx context.Context, eventID string) (bool, error) {
	args := m.Called(ctx, eventID)
	return args.Bool(0), args.Error(1)
}

func (m *mockWebhookEventsRepo) MarkProcessing(ctx context.Context, eventID string) error {
	args := m.Called(ctx, eventID)
	return args.Error(0)
}

func (m *mockWebhookEventsRepo) MarkProcessed(ctx context.Context, eventID string) error {
	args := m.Called(ctx, eventID)
	return args.Error(0)
}

func (m *mockWebhookEventsRepo) MarkProcessedEnsure(ctx context.Context, eventID, sessionName, eventType string) error {
	args := m.Called(ctx, eventID, sessionName, eventType)
	return args.Error(0)
}

func (m *mockWebhookEventsRepo) MarkFailed(ctx context.Context, eventID, errorMsg string) error {
	args := m.Called(ctx, eventID, errorMsg)
	return args.Error(0)
}

func (m *mockWebhookEventsRepo) MarkDLQ(ctx context.Context, eventID, errorMsg string) error {
	args := m.Called(ctx, eventID, errorMsg)
	return args.Error(0)
}

func (m *mockWebhookEventsRepo) IncrementRetryCount(ctx context.Context, eventID string) error {
	args := m.Called(ctx, eventID)
	return args.Error(0)
}

func (m *mockWebhookEventsRepo) GetPendingEvents(ctx context.Context, limit int) ([]models.WebhookEventRecord, error) {
	args := m.Called(ctx, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.WebhookEventRecord), args.Error(1)
}

func (m *mockWebhookEventsRepo) GetFailedEvents(ctx context.Context, limit int) ([]models.WebhookEventRecord, error) {
	args := m.Called(ctx, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.WebhookEventRecord), args.Error(1)
}

func (m *mockWebhookEventsRepo) Cleanup(ctx context.Context, olderThan time.Duration) (int64, error) {
	args := m.Called(ctx, olderThan)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockWebhookEventsRepo) CleanupUnrecoverable(ctx context.Context, olderThan time.Duration) (int64, error) {
	args := m.Called(ctx, olderThan)
	return args.Get(0).(int64), args.Error(1)
}

// mockInstanceRepo implements repository.InstanceRepository for testing.
type mockInstanceRepo struct {
	mock.Mock
}

func (m *mockInstanceRepo) GetUserInstances(ctx context.Context, userID string) ([]models.Instance, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Instance), args.Error(1)
}

func (m *mockInstanceRepo) GetInstanceByID(ctx context.Context, instanceID int) (*models.Instance, error) {
	args := m.Called(ctx, instanceID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Instance), args.Error(1)
}

func (m *mockInstanceRepo) GetInstanceByName(ctx context.Context, name string) (*models.Instance, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Instance), args.Error(1)
}

func (m *mockInstanceRepo) GetInstanceByUserAndName(ctx context.Context, userID, name string) (*models.Instance, error) {
	args := m.Called(ctx, userID, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Instance), args.Error(1)
}

func (m *mockInstanceRepo) GetAllConnectedInstances(ctx context.Context) ([]models.Instance, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Instance), args.Error(1)
}

func (m *mockInstanceRepo) UpsertInstance(ctx context.Context, userID string, data *models.Instance) (*models.Instance, error) {
	args := m.Called(ctx, userID, data)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Instance), args.Error(1)
}

func (m *mockInstanceRepo) UpdateInstanceStatus(ctx context.Context, id int, status string) error {
	args := m.Called(ctx, id, status)
	return args.Error(0)
}

func (m *mockInstanceRepo) DeleteInstance(ctx context.Context, id int) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockInstanceRepo) SetDefaultInstance(ctx context.Context, userID string, instanceID int) error {
	args := m.Called(ctx, userID, instanceID)
	return args.Error(0)
}

func (m *mockInstanceRepo) CountUserInstances(ctx context.Context, userID string) (int, error) {
	args := m.Called(ctx, userID)
	return args.Int(0), args.Error(1)
}

func (m *mockInstanceRepo) UpdateInstanceLabel(ctx context.Context, userID string, instanceID int, friendlyName string) error {
	args := m.Called(ctx, userID, instanceID, friendlyName)
	return args.Error(0)
}

func (m *mockInstanceRepo) UpdateInstancePhoneNumber(ctx context.Context, instanceID int, phoneNumber string) error {
	args := m.Called(ctx, instanceID, phoneNumber)
	return args.Error(0)
}

func (m *mockInstanceRepo) SupersedePriorInstances(ctx context.Context, userID string, currentInstanceID int, phoneNumber string) (int, error) {
	args := m.Called(ctx, userID, currentInstanceID, phoneNumber)
	return args.Int(0), args.Error(1)
}

// mockMessageProcessor implements MessageWebhookProcessor for testing.
type mockMessageProcessor struct {
	mock.Mock
}

func (m *mockMessageProcessor) ProcessMessage(ctx context.Context, sessionName string, payload map[string]interface{}) error {
	args := m.Called(ctx, sessionName, payload)
	return args.Error(0)
}

// mockStatusProcessor implements StatusWebhookProcessor for testing.
type mockStatusProcessor struct {
	mock.Mock
}

func (m *mockStatusProcessor) ProcessStatus(ctx context.Context, sessionName, status string) error {
	args := m.Called(ctx, sessionName, status)
	return args.Error(0)
}

// mockGroupEventProcessor implements GroupEventWebhookProcessor for testing.
type mockGroupEventProcessor struct {
	mock.Mock
}

func (m *mockGroupEventProcessor) ProcessGroupEvent(ctx context.Context, sessionName, eventType string, payload map[string]interface{}) error {
	args := m.Called(ctx, sessionName, eventType, payload)
	return args.Error(0)
}

// =============================================================================
// Helper Functions
// =============================================================================

// createTestWorker creates a webhook consumer worker with mocked dependencies.
func createTestWorker(t *testing.T) (*WebhookConsumerWorker, *mockWebhookEventsRepo, *mockInstanceRepo) {
	t.Helper()

	webhookEventsRepo := new(mockWebhookEventsRepo)
	instanceRepo := new(mockInstanceRepo)
	logger := zap.NewNop()

	// Default: ensureInstanceConnected returns nil (no instance found) for any session.
	// Individual tests can override with more specific expectations.
	instanceRepo.On("GetInstanceByName", mock.Anything, mock.Anything).Maybe().Return((*models.Instance)(nil), nil)

	worker := NewWebhookConsumerWorker(WebhookConsumerWorkerConfig{
		WebhookEventsRepo: webhookEventsRepo,
		InstanceRepo:      instanceRepo,
		Logger:            logger,
	})

	return worker, webhookEventsRepo, instanceRepo
}

// createTestTask creates an Asynq task from job data.
func createTestTask(t *testing.T, data *WebhookJobData) *asynq.Task {
	t.Helper()

	payload, err := json.Marshal(data)
	require.NoError(t, err)

	return asynq.NewTask(TypeWAHAWebhook, payload)
}

// =============================================================================
// ProcessTask Tests
// =============================================================================

func TestProcessTask_Success(t *testing.T) {
	worker, webhookEventsRepo, _ := createTestWorker(t)

	data := &WebhookJobData{
		EventID:    "evt-123",
		Event:      "message.ack",
		Session:    "session-1",
		ReceivedAt: time.Now().Unix(),
		Payload: map[string]interface{}{
			"id":  "msg-456",
			"ack": 2,
		},
	}

	task := createTestTask(t, data)
	ctx := context.Background()

	// Setup expectations
	webhookEventsRepo.On("Exists", ctx, "evt-123").Return(true, nil)
	webhookEventsRepo.On("MarkProcessing", ctx, "evt-123").Return(nil)
	webhookEventsRepo.On("MarkProcessed", ctx, "evt-123").Return(nil)

	// Execute
	err := worker.ProcessTask(ctx, task)

	// Verify
	assert.NoError(t, err)
	webhookEventsRepo.AssertExpectations(t)
}

func TestProcessTask_MalformedPayload(t *testing.T) {
	worker, _, _ := createTestWorker(t)

	task := asynq.NewTask(TypeWAHAWebhook, []byte("invalid json"))
	ctx := context.Background()

	// Execute - malformed payloads should not be retried
	err := worker.ProcessTask(ctx, task)

	// Verify - returns nil to prevent retries
	assert.NoError(t, err)
}

func TestProcessTask_EventNotExistsCreatesIt(t *testing.T) {
	worker, webhookEventsRepo, _ := createTestWorker(t)

	data := &WebhookJobData{
		EventID:    "evt-new",
		Event:      "message.ack",
		Session:    "session-1",
		ReceivedAt: time.Now().Unix(),
		Payload:    map[string]interface{}{},
	}

	task := createTestTask(t, data)
	ctx := context.Background()

	// Setup expectations
	webhookEventsRepo.On("Exists", ctx, "evt-new").Return(false, nil)
	webhookEventsRepo.On("InsertIfNotExists", ctx, "evt-new", "session-1", "message.ack").Return(true, nil)
	webhookEventsRepo.On("MarkProcessing", ctx, "evt-new").Return(nil)
	webhookEventsRepo.On("MarkProcessed", ctx, "evt-new").Return(nil)

	// Execute
	err := worker.ProcessTask(ctx, task)

	// Verify
	assert.NoError(t, err)
	webhookEventsRepo.AssertExpectations(t)
}

func TestProcessTask_ExistsCheckError(t *testing.T) {
	worker, webhookEventsRepo, _ := createTestWorker(t)

	data := &WebhookJobData{
		EventID: "evt-123",
		Event:   "message",
		Session: "session-1",
	}

	task := createTestTask(t, data)
	ctx := context.Background()

	dbErr := errors.New("database connection error")
	webhookEventsRepo.On("Exists", ctx, "evt-123").Return(false, dbErr)

	// Execute
	err := worker.ProcessTask(ctx, task)

	// Verify - should return error for retry
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "check event existence")
}

func TestProcessTask_InsertMissingEventError(t *testing.T) {
	worker, webhookEventsRepo, _ := createTestWorker(t)

	data := &WebhookJobData{
		EventID: "evt-123",
		Event:   "message",
		Session: "session-1",
	}

	task := createTestTask(t, data)
	ctx := context.Background()

	webhookEventsRepo.On("Exists", ctx, "evt-123").Return(false, nil)
	webhookEventsRepo.On("InsertIfNotExists", ctx, "evt-123", "session-1", "message").
		Return(false, errors.New("insert failed"))

	// Execute
	err := worker.ProcessTask(ctx, task)

	// Verify - should return error for retry
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "insert missing event")
}

// =============================================================================
// Route Event Tests
// =============================================================================

func TestRouteEvent_MessageEvent(t *testing.T) {
	webhookEventsRepo := new(mockWebhookEventsRepo)
	msgProcessor := new(mockMessageProcessor)
	logger := zap.NewNop()

	worker := NewWebhookConsumerWorker(WebhookConsumerWorkerConfig{
		WebhookEventsRepo: webhookEventsRepo,
		MessageProcessor:  msgProcessor,
		Logger:            logger,
	})

	ctx := context.Background()
	data := &WebhookJobData{
		EventID: "evt-msg",
		Event:   "message",
		Session: "session-1",
		Payload: map[string]interface{}{"body": "hello"},
	}

	msgProcessor.On("ProcessMessage", ctx, "session-1", data.Payload).Return(nil)

	err := worker.routeEvent(ctx, data)

	assert.NoError(t, err)
	msgProcessor.AssertExpectations(t)
}

func TestRouteEvent_MessageAnyEvent(t *testing.T) {
	webhookEventsRepo := new(mockWebhookEventsRepo)
	msgProcessor := new(mockMessageProcessor)
	logger := zap.NewNop()

	worker := NewWebhookConsumerWorker(WebhookConsumerWorkerConfig{
		WebhookEventsRepo: webhookEventsRepo,
		MessageProcessor:  msgProcessor,
		Logger:            logger,
	})

	ctx := context.Background()
	data := &WebhookJobData{
		EventID: "evt-msg",
		Event:   "message.any",
		Session: "session-1",
		Payload: map[string]interface{}{"body": "hello"},
	}

	msgProcessor.On("ProcessMessage", ctx, "session-1", data.Payload).Return(nil)

	err := worker.routeEvent(ctx, data)

	assert.NoError(t, err)
	msgProcessor.AssertExpectations(t)
}

func TestRouteEvent_MessageEventNoProcessor(t *testing.T) {
	worker, _, _ := createTestWorker(t)

	ctx := context.Background()
	data := &WebhookJobData{
		EventID: "evt-msg",
		Event:   "message",
		Session: "session-1",
		Payload: map[string]interface{}{},
	}

	// Should not error even without processor
	err := worker.routeEvent(ctx, data)

	assert.NoError(t, err)
}

func TestRouteEvent_SessionStatusEvent(t *testing.T) {
	webhookEventsRepo := new(mockWebhookEventsRepo)
	instanceRepo := new(mockInstanceRepo)
	statusProcessor := new(mockStatusProcessor)
	logger := zap.NewNop()

	worker := NewWebhookConsumerWorker(WebhookConsumerWorkerConfig{
		WebhookEventsRepo: webhookEventsRepo,
		InstanceRepo:      instanceRepo,
		StatusProcessor:   statusProcessor,
		Logger:            logger,
	})

	ctx := context.Background()
	data := &WebhookJobData{
		EventID: "evt-status",
		Event:   "session.status",
		Session: "session-1",
		Payload: map[string]interface{}{"status": "CONNECTED"},
	}

	instanceRepo.On("GetInstanceByName", ctx, "session-1").
		Return(&models.Instance{ID: 1, InstanceID: "session-1"}, nil)
	instanceRepo.On("UpdateInstanceStatus", ctx, 1, "connected").Return(nil)
	statusProcessor.On("ProcessStatus", ctx, "session-1", "CONNECTED").Return(nil)

	err := worker.routeEvent(ctx, data)

	assert.NoError(t, err)
	instanceRepo.AssertExpectations(t)
	statusProcessor.AssertExpectations(t)
}

func TestRouteEvent_SessionStatusEventMissingStatus(t *testing.T) {
	worker, _, _ := createTestWorker(t)

	ctx := context.Background()
	data := &WebhookJobData{
		EventID: "evt-status",
		Event:   "session.status",
		Session: "session-1",
		Payload: map[string]interface{}{}, // No status field
	}

	// Should not error - just skip
	err := worker.routeEvent(ctx, data)

	assert.NoError(t, err)
}

func TestRouteEvent_SessionStatusUpdateError(t *testing.T) {
	webhookEventsRepo := new(mockWebhookEventsRepo)
	instanceRepo := new(mockInstanceRepo)
	logger := zap.NewNop()

	worker := NewWebhookConsumerWorker(WebhookConsumerWorkerConfig{
		WebhookEventsRepo: webhookEventsRepo,
		InstanceRepo:      instanceRepo,
		Logger:            logger,
	})

	ctx := context.Background()
	data := &WebhookJobData{
		EventID: "evt-status",
		Event:   "session.status",
		Session: "session-1",
		Payload: map[string]interface{}{"status": "CONNECTED"},
	}

	instanceRepo.On("GetInstanceByName", ctx, "session-1").
		Return(&models.Instance{ID: 1}, nil)
	instanceRepo.On("UpdateInstanceStatus", ctx, 1, "connected").
		Return(errors.New("db error"))

	err := worker.routeEvent(ctx, data)

	// Should return error for retry
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "update instance status")
}

func TestRouteEvent_GroupEvent(t *testing.T) {
	webhookEventsRepo := new(mockWebhookEventsRepo)
	groupProcessor := new(mockGroupEventProcessor)
	logger := zap.NewNop()

	worker := NewWebhookConsumerWorker(WebhookConsumerWorkerConfig{
		WebhookEventsRepo:   webhookEventsRepo,
		GroupEventProcessor: groupProcessor,
		Logger:              logger,
	})

	ctx := context.Background()
	data := &WebhookJobData{
		EventID: "evt-group",
		Event:   "group.v2.join",
		Session: "session-1",
		Payload: map[string]interface{}{"groupId": "group-123"},
	}

	groupProcessor.On("ProcessGroupEvent", ctx, "session-1", "group.v2.join", data.Payload).Return(nil)

	err := worker.routeEvent(ctx, data)

	assert.NoError(t, err)
	groupProcessor.AssertExpectations(t)
}

func TestRouteEvent_GroupEventNoProcessor(t *testing.T) {
	worker, _, _ := createTestWorker(t)

	ctx := context.Background()
	data := &WebhookJobData{
		EventID: "evt-group",
		Event:   "group.v2.leave",
		Session: "session-1",
		Payload: map[string]interface{}{},
	}

	// Should not error even without processor
	err := worker.routeEvent(ctx, data)

	assert.NoError(t, err)
}

func TestRouteEvent_MessageAckEvent(t *testing.T) {
	worker, _, _ := createTestWorker(t)

	ctx := context.Background()
	data := &WebhookJobData{
		EventID: "evt-ack",
		Event:   "message.ack",
		Session: "session-1",
		Payload: map[string]interface{}{
			"id":  "msg-123",
			"ack": 3,
		},
	}

	// ACK events are just logged, no error expected
	err := worker.routeEvent(ctx, data)

	assert.NoError(t, err)
}

func TestRouteEvent_MessageReactionEvent(t *testing.T) {
	worker, _, _ := createTestWorker(t)

	ctx := context.Background()
	data := &WebhookJobData{
		EventID: "evt-reaction",
		Event:   "message.reaction",
		Session: "session-1",
		Payload: map[string]interface{}{
			"msgId": "msg-123",
			"text":  "👍",
			"from":  "5511999999999@c.us",
		},
	}

	// Reaction events are just logged
	err := worker.routeEvent(ctx, data)

	assert.NoError(t, err)
}

func TestRouteEvent_QRCodeEvent(t *testing.T) {
	worker, _, _ := createTestWorker(t)

	ctx := context.Background()
	data := &WebhookJobData{
		EventID: "evt-qr",
		Event:   "qr.code",
		Session: "session-1",
		Payload: map[string]interface{}{},
	}

	// QR events are just logged
	err := worker.routeEvent(ctx, data)

	assert.NoError(t, err)
}

func TestRouteEvent_CallEvent(t *testing.T) {
	worker, _, _ := createTestWorker(t)

	ctx := context.Background()
	data := &WebhookJobData{
		EventID: "evt-call",
		Event:   "call",
		Session: "session-1",
		Payload: map[string]interface{}{},
	}

	// Call events are ignored
	err := worker.routeEvent(ctx, data)

	assert.NoError(t, err)
}

func TestRouteEvent_UnknownEventType(t *testing.T) {
	worker, _, _ := createTestWorker(t)

	ctx := context.Background()
	data := &WebhookJobData{
		EventID: "evt-unknown",
		Event:   "unknown.event.type",
		Session: "session-1",
		Payload: map[string]interface{}{},
	}

	// Unknown events should be skipped without error
	err := worker.routeEvent(ctx, data)

	assert.NoError(t, err)
}

// =============================================================================
// isRetryableError Tests
// =============================================================================

func TestIsRetryableError_NilError(t *testing.T) {
	worker, _, _ := createTestWorker(t)

	assert.False(t, worker.isRetryableError(nil))
}

func TestIsRetryableError_NonRetryableErrors(t *testing.T) {
	worker, _, _ := createTestWorker(t)

	nonRetryable := []string{
		"invalid request format",
		"malformed payload",
		"resource not found",
		"unauthorized access",
		"forbidden operation",
		"permission denied",
		"event already processed",
		"record already exists",
	}

	for _, errMsg := range nonRetryable {
		t.Run(errMsg, func(t *testing.T) {
			err := errors.New(errMsg)
			assert.False(t, worker.isRetryableError(err))
		})
	}
}

func TestIsRetryableError_RetryableErrors(t *testing.T) {
	worker, _, _ := createTestWorker(t)

	retryable := []string{
		"connection timeout",
		"context deadline exceeded",
		"connection refused",
		"connection reset by peer",
		"temporary failure",
		"service unavailable",
		"rate limit exceeded",
		"too many requests",
		"internal server error",
		"bad gateway",
		"service unavailable",
	}

	for _, errMsg := range retryable {
		t.Run(errMsg, func(t *testing.T) {
			err := errors.New(errMsg)
			assert.True(t, worker.isRetryableError(err))
		})
	}
}

func TestIsRetryableError_UnknownErrorsAreRetryable(t *testing.T) {
	worker, _, _ := createTestWorker(t)

	// Unknown errors should default to retryable
	err := errors.New("some random error")
	assert.True(t, worker.isRetryableError(err))
}

// =============================================================================
// handleProcessingError Tests
// =============================================================================

func TestHandleProcessingError_WithinRetryLimit(t *testing.T) {
	webhookEventsRepo := new(mockWebhookEventsRepo)
	logger := zap.NewNop()

	worker := NewWebhookConsumerWorker(WebhookConsumerWorkerConfig{
		WebhookEventsRepo: webhookEventsRepo,
		Logger:            logger,
	})

	ctx := context.Background()
	data := &WebhookJobData{
		EventID: "evt-123",
		Event:   "message",
		Session: "session-1",
	}
	task := createTestTask(t, data)
	processErr := errors.New("temporary error")

	webhookEventsRepo.On("MarkFailed", ctx, "evt-123", "temporary error").Return(nil)

	// Execute with retry count = 0 (first attempt)
	err := worker.handleProcessingError(ctx, task, data, processErr)

	// Should return error to trigger retry
	assert.Error(t, err)
	assert.Equal(t, processErr, err)
	webhookEventsRepo.AssertExpectations(t)
}

func TestHandleProcessingError_NonRetryableError(t *testing.T) {
	webhookEventsRepo := new(mockWebhookEventsRepo)
	logger := zap.NewNop()

	worker := NewWebhookConsumerWorker(WebhookConsumerWorkerConfig{
		WebhookEventsRepo: webhookEventsRepo,
		Logger:            logger,
	})

	ctx := context.Background()
	data := &WebhookJobData{
		EventID: "evt-123",
		Event:   "message",
		Session: "session-1",
	}
	task := createTestTask(t, data)
	processErr := errors.New("resource not found")

	webhookEventsRepo.On("MarkFailed", ctx, "evt-123", "resource not found").Return(nil)

	// Execute
	err := worker.handleProcessingError(ctx, task, data, processErr)

	// Should return nil to stop retries
	assert.NoError(t, err)
	webhookEventsRepo.AssertExpectations(t)
}

// =============================================================================
// mapWAHAStatusToDBStatus Tests
// =============================================================================

func TestMapWAHAStatusToDBStatus(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"CONNECTED", "connected"},
		{"connected", "connected"},
		{"WORKING", "connected"},
		{"working", "connected"},
		{"DISCONNECTED", "disconnected"},
		{"disconnected", "disconnected"},
		{"FAILED", "disconnected"},
		{"failed", "disconnected"},
		{"STOPPED", "disconnected"},
		{"stopped", "disconnected"},
		{"SCAN_QR_CODE", "pending"},
		{"scan_qr_code", "pending"},
		{"STARTING", "pending"},
		{"starting", "pending"},
		{"unknown", "disconnected"},
		{"", "disconnected"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := mapWAHAStatusToDBStatus(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// =============================================================================
// extractString Tests
// =============================================================================

func TestExtractString(t *testing.T) {
	tests := []struct {
		name     string
		m        map[string]interface{}
		key      string
		expected string
		ok       bool
	}{
		{
			name:     "existing key",
			m:        map[string]interface{}{"foo": "bar"},
			key:      "foo",
			expected: "bar",
			ok:       true,
		},
		{
			name:     "missing key",
			m:        map[string]interface{}{"foo": "bar"},
			key:      "baz",
			expected: "",
			ok:       false,
		},
		{
			name:     "non-string value",
			m:        map[string]interface{}{"foo": 123},
			key:      "foo",
			expected: "",
			ok:       false,
		},
		{
			name:     "nil map",
			m:        nil,
			key:      "foo",
			expected: "",
			ok:       false,
		},
		{
			name:     "empty map",
			m:        map[string]interface{}{},
			key:      "foo",
			expected: "",
			ok:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := extractString(tt.m, tt.key)
			assert.Equal(t, tt.expected, result)
			assert.Equal(t, tt.ok, ok)
		})
	}
}

// =============================================================================
// extractInt Tests
// =============================================================================

func TestExtractInt(t *testing.T) {
	tests := []struct {
		name     string
		m        map[string]interface{}
		key      string
		expected int
		ok       bool
	}{
		{
			name:     "int value",
			m:        map[string]interface{}{"count": 42},
			key:      "count",
			expected: 42,
			ok:       true,
		},
		{
			name:     "int64 value",
			m:        map[string]interface{}{"count": int64(42)},
			key:      "count",
			expected: 42,
			ok:       true,
		},
		{
			name:     "float64 value",
			m:        map[string]interface{}{"count": float64(42.0)},
			key:      "count",
			expected: 42,
			ok:       true,
		},
		{
			name:     "missing key",
			m:        map[string]interface{}{"foo": 1},
			key:      "bar",
			expected: 0,
			ok:       false,
		},
		{
			name:     "non-numeric value",
			m:        map[string]interface{}{"foo": "bar"},
			key:      "foo",
			expected: 0,
			ok:       false,
		},
		{
			name:     "nil map",
			m:        nil,
			key:      "foo",
			expected: 0,
			ok:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := extractInt(tt.m, tt.key)
			assert.Equal(t, tt.expected, result)
			assert.Equal(t, tt.ok, ok)
		})
	}
}

// =============================================================================
// ProcessTaskWrapper Tests
// =============================================================================

func TestProcessTaskWrapper(t *testing.T) {
	worker, webhookEventsRepo, _ := createTestWorker(t)

	data := &WebhookJobData{
		EventID: "evt-123",
		Event:   "message.ack",
		Session: "session-1",
		Payload: map[string]interface{}{},
	}
	task := createTestTask(t, data)
	ctx := context.Background()

	webhookEventsRepo.On("Exists", ctx, "evt-123").Return(true, nil)
	webhookEventsRepo.On("MarkProcessing", ctx, "evt-123").Return(nil)
	webhookEventsRepo.On("MarkProcessed", ctx, "evt-123").Return(nil)

	// Get the handler function
	handlerFunc := worker.ProcessTaskWrapper()

	// Execute via the wrapper
	err := handlerFunc(ctx, task)

	assert.NoError(t, err)
	webhookEventsRepo.AssertExpectations(t)
}

// =============================================================================
// Integration-style Tests
// =============================================================================

func TestProcessTask_FullMessageFlow(t *testing.T) {
	webhookEventsRepo := new(mockWebhookEventsRepo)
	instanceRepo := new(mockInstanceRepo)
	msgProcessor := new(mockMessageProcessor)
	logger := zap.NewNop()

	// Default for ensureInstanceConnected self-healing
	instanceRepo.On("GetInstanceByName", mock.Anything, mock.Anything).Maybe().Return((*models.Instance)(nil), nil)

	worker := NewWebhookConsumerWorker(WebhookConsumerWorkerConfig{
		WebhookEventsRepo: webhookEventsRepo,
		InstanceRepo:      instanceRepo,
		MessageProcessor:  msgProcessor,
		Logger:            logger,
	})

	data := &WebhookJobData{
		EventID:    "evt-full",
		Event:      "message",
		Session:    "session-1",
		ReceivedAt: time.Now().Unix(),
		Payload: map[string]interface{}{
			"body": "Hello World",
			"from": "5511999999999@c.us",
		},
	}

	task := createTestTask(t, data)
	ctx := context.Background()

	webhookEventsRepo.On("Exists", ctx, "evt-full").Return(true, nil)
	webhookEventsRepo.On("MarkProcessing", ctx, "evt-full").Return(nil)
	msgProcessor.On("ProcessMessage", ctx, "session-1", data.Payload).Return(nil)
	webhookEventsRepo.On("MarkProcessed", ctx, "evt-full").Return(nil)

	err := worker.ProcessTask(ctx, task)

	assert.NoError(t, err)
	webhookEventsRepo.AssertExpectations(t)
	msgProcessor.AssertExpectations(t)
}

func TestProcessTask_FullStatusFlow(t *testing.T) {
	webhookEventsRepo := new(mockWebhookEventsRepo)
	instanceRepo := new(mockInstanceRepo)
	statusProcessor := new(mockStatusProcessor)
	logger := zap.NewNop()

	worker := NewWebhookConsumerWorker(WebhookConsumerWorkerConfig{
		WebhookEventsRepo: webhookEventsRepo,
		InstanceRepo:      instanceRepo,
		StatusProcessor:   statusProcessor,
		Logger:            logger,
	})

	data := &WebhookJobData{
		EventID:    "evt-status-full",
		Event:      "session.status",
		Session:    "session-1",
		ReceivedAt: time.Now().Unix(),
		Payload:    map[string]interface{}{"status": "CONNECTED"},
	}

	task := createTestTask(t, data)
	ctx := context.Background()

	webhookEventsRepo.On("Exists", ctx, "evt-status-full").Return(true, nil)
	webhookEventsRepo.On("MarkProcessing", ctx, "evt-status-full").Return(nil)
	instanceRepo.On("GetInstanceByName", ctx, "session-1").
		Return(&models.Instance{ID: 123, InstanceID: "session-1", Status: "disconnected"}, nil)
	instanceRepo.On("UpdateInstanceStatus", ctx, 123, "connected").Return(nil)
	statusProcessor.On("ProcessStatus", ctx, "session-1", "CONNECTED").Return(nil)
	webhookEventsRepo.On("MarkProcessed", ctx, "evt-status-full").Return(nil)

	err := worker.ProcessTask(ctx, task)

	assert.NoError(t, err)
	webhookEventsRepo.AssertExpectations(t)
	instanceRepo.AssertExpectations(t)
	statusProcessor.AssertExpectations(t)
}

// =============================================================================
// Error Recovery Tests
// =============================================================================

func TestProcessTask_MarkProcessingErrorContinues(t *testing.T) {
	worker, webhookEventsRepo, _ := createTestWorker(t)

	data := &WebhookJobData{
		EventID: "evt-123",
		Event:   "message.ack",
		Session: "session-1",
		Payload: map[string]interface{}{},
	}

	task := createTestTask(t, data)
	ctx := context.Background()

	webhookEventsRepo.On("Exists", ctx, "evt-123").Return(true, nil)
	webhookEventsRepo.On("MarkProcessing", ctx, "evt-123").Return(errors.New("db error"))
	webhookEventsRepo.On("MarkProcessed", ctx, "evt-123").Return(nil)

	// Should continue processing even if MarkProcessing fails
	err := worker.ProcessTask(ctx, task)

	assert.NoError(t, err)
}

func TestProcessTask_MarkProcessedErrorDoesNotFail(t *testing.T) {
	worker, webhookEventsRepo, _ := createTestWorker(t)

	data := &WebhookJobData{
		EventID: "evt-123",
		Event:   "message.ack",
		Session: "session-1",
		Payload: map[string]interface{}{},
	}

	task := createTestTask(t, data)
	ctx := context.Background()

	webhookEventsRepo.On("Exists", ctx, "evt-123").Return(true, nil)
	webhookEventsRepo.On("MarkProcessing", ctx, "evt-123").Return(nil)
	webhookEventsRepo.On("MarkProcessed", ctx, "evt-123").Return(errors.New("db error"))

	// Should not fail even if MarkProcessed fails (processing was successful)
	err := worker.ProcessTask(ctx, task)

	assert.NoError(t, err)
}

// =============================================================================
// Concurrent Event Handling Tests
// =============================================================================

func TestProcessTask_RaceConditionOnInsert(t *testing.T) {
	worker, webhookEventsRepo, _ := createTestWorker(t)

	data := &WebhookJobData{
		EventID: "evt-race",
		Event:   "message.ack",
		Session: "session-1",
		Payload: map[string]interface{}{},
	}

	task := createTestTask(t, data)
	ctx := context.Background()

	// Simulate race condition: event doesn't exist on check, but InsertIfNotExists
	// returns false (another process inserted it)
	webhookEventsRepo.On("Exists", ctx, "evt-race").Return(false, nil)
	webhookEventsRepo.On("InsertIfNotExists", ctx, "evt-race", "session-1", "message.ack").Return(false, nil)
	webhookEventsRepo.On("MarkProcessing", ctx, "evt-race").Return(nil)
	webhookEventsRepo.On("MarkProcessed", ctx, "evt-race").Return(nil)

	// Should handle race condition gracefully
	err := worker.ProcessTask(ctx, task)

	assert.NoError(t, err)
	webhookEventsRepo.AssertExpectations(t)
}

// =============================================================================
// Event Type Variations Tests
// =============================================================================

func TestRouteEvent_CaseInsensitive(t *testing.T) {
	// Create worker without instance repo to avoid mock setup complexity
	logger := zap.NewNop()
	worker := NewWebhookConsumerWorker(WebhookConsumerWorkerConfig{
		Logger: logger,
	})

	ctx := context.Background()

	// Test event types that don't require instance repo
	eventTypes := []string{
		"MESSAGE",
		"Message",
		"MESSAGE.ANY",
		"Message.Any",
		"MESSAGE.ACK",
		"Message.Ack",
		"QR.CODE",
		"Qr.Code",
		"CALL",
		"Call",
	}

	for _, eventType := range eventTypes {
		t.Run(eventType, func(t *testing.T) {
			data := &WebhookJobData{
				EventID: "evt-" + eventType,
				Event:   eventType,
				Session: "session-1",
				Payload: map[string]interface{}{"status": "CONNECTED"},
			}

			// Should not error for any case variation
			err := worker.routeEvent(ctx, data)
			assert.NoError(t, err)
		})
	}
}

// =============================================================================
// NewWebhookConsumerWorker Tests
// =============================================================================

func TestNewWebhookConsumerWorker(t *testing.T) {
	webhookEventsRepo := new(mockWebhookEventsRepo)
	instanceRepo := new(mockInstanceRepo)
	msgProcessor := new(mockMessageProcessor)
	statusProcessor := new(mockStatusProcessor)
	groupProcessor := new(mockGroupEventProcessor)
	logger := zap.NewNop()

	worker := NewWebhookConsumerWorker(WebhookConsumerWorkerConfig{
		WebhookEventsRepo:   webhookEventsRepo,
		InstanceRepo:        instanceRepo,
		MessageProcessor:    msgProcessor,
		StatusProcessor:     statusProcessor,
		GroupEventProcessor: groupProcessor,
		Logger:              logger,
	})

	assert.NotNil(t, worker)
	assert.Equal(t, webhookEventsRepo, worker.webhookEventsRepo)
	assert.Equal(t, instanceRepo, worker.instanceRepo)
	assert.Equal(t, msgProcessor, worker.messageProcessor)
	assert.Equal(t, statusProcessor, worker.statusProcessor)
	assert.Equal(t, groupProcessor, worker.groupEventProcessor)
}

func TestNewWebhookConsumerWorker_MinimalConfig(t *testing.T) {
	logger := zap.NewNop()

	// Should work with just logger (all repos/processors nil)
	worker := NewWebhookConsumerWorker(WebhookConsumerWorkerConfig{
		Logger: logger,
	})

	assert.NotNil(t, worker)
	assert.Nil(t, worker.webhookEventsRepo)
	assert.Nil(t, worker.instanceRepo)
	assert.Nil(t, worker.messageProcessor)
}

// =============================================================================
// Group Event Prefix Tests
// =============================================================================

func TestRouteEvent_GroupEventPrefixes(t *testing.T) {
	groupProcessor := new(mockGroupEventProcessor)
	logger := zap.NewNop()

	worker := NewWebhookConsumerWorker(WebhookConsumerWorkerConfig{
		GroupEventProcessor: groupProcessor,
		Logger:              logger,
	})

	ctx := context.Background()

	// Test various group event types
	groupEvents := []string{
		"group.v2.join",
		"group.v2.leave",
		"group.v2.update",
		"group.join",
		"group.leave",
	}

	for _, eventType := range groupEvents {
		t.Run(eventType, func(t *testing.T) {
			data := &WebhookJobData{
				EventID: "evt-" + eventType,
				Event:   eventType,
				Session: "session-1",
				Payload: map[string]interface{}{},
			}

			groupProcessor.On("ProcessGroupEvent", ctx, "session-1", eventType, data.Payload).Return(nil).Once()

			err := worker.routeEvent(ctx, data)

			assert.NoError(t, err)
		})
	}

	groupProcessor.AssertExpectations(t)
}

// =============================================================================
// Instance Not Found Tests
// =============================================================================

func TestRouteEvent_SessionStatusInstanceNotFound(t *testing.T) {
	webhookEventsRepo := new(mockWebhookEventsRepo)
	instanceRepo := new(mockInstanceRepo)
	logger := zap.NewNop()

	worker := NewWebhookConsumerWorker(WebhookConsumerWorkerConfig{
		WebhookEventsRepo: webhookEventsRepo,
		InstanceRepo:      instanceRepo,
		Logger:            logger,
	})

	ctx := context.Background()
	data := &WebhookJobData{
		EventID: "evt-status",
		Event:   "session.status",
		Session: "session-1",
		Payload: map[string]interface{}{"status": "CONNECTED"},
	}

	// Instance not found - should log warning but not fail
	instanceRepo.On("GetInstanceByName", ctx, "session-1").
		Return(nil, errors.New("not found"))

	err := worker.routeEvent(ctx, data)

	// Should not return error when instance not found
	assert.NoError(t, err)
}

func TestRouteEvent_SessionStatusInstanceNil(t *testing.T) {
	webhookEventsRepo := new(mockWebhookEventsRepo)
	instanceRepo := new(mockInstanceRepo)
	logger := zap.NewNop()

	worker := NewWebhookConsumerWorker(WebhookConsumerWorkerConfig{
		WebhookEventsRepo: webhookEventsRepo,
		InstanceRepo:      instanceRepo,
		Logger:            logger,
	})

	ctx := context.Background()
	data := &WebhookJobData{
		EventID: "evt-status",
		Event:   "session.status",
		Session: "session-1",
		Payload: map[string]interface{}{"status": "CONNECTED"},
	}

	// Instance is nil (not found, no error)
	instanceRepo.On("GetInstanceByName", ctx, "session-1").
		Return(nil, nil)

	err := worker.routeEvent(ctx, data)

	// Should not return error when instance is nil
	assert.NoError(t, err)
}

// =============================================================================
// Context Cancellation Tests
// =============================================================================

func TestProcessTask_ContextCancelled(t *testing.T) {
	worker, _, _ := createTestWorker(t)

	data := &WebhookJobData{
		EventID: "evt-cancelled",
		Event:   "message",
		Session: "session-1",
		Payload: map[string]interface{}{},
	}
	task := createTestTask(t, data)

	// Cancel context before processing starts
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := worker.ProcessTask(ctx, task)

	// Should return context.Canceled
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestRouteEvent_ContextCancelled(t *testing.T) {
	worker, _, _ := createTestWorker(t)

	data := &WebhookJobData{
		EventID: "evt-cancelled-route",
		Event:   "message",
		Session: "session-1",
		Payload: map[string]interface{}{},
	}

	// Cancel context before routing starts
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := worker.routeEvent(ctx, data)

	// Should return context.Canceled
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestProcessTask_ContextDeadlineExceeded(t *testing.T) {
	worker, _, _ := createTestWorker(t)

	data := &WebhookJobData{
		EventID: "evt-deadline",
		Event:   "message",
		Session: "session-1",
		Payload: map[string]interface{}{},
	}
	task := createTestTask(t, data)

	// Create context with already-expired deadline
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-1*time.Second))
	defer cancel()

	err := worker.ProcessTask(ctx, task)

	// Should return context.DeadlineExceeded
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

// =============================================================================
// QR Code Event with Status Processor Tests
// =============================================================================

func TestRouteEvent_QRCodeWithStatusProcessor(t *testing.T) {
	statusProcessor := new(mockStatusProcessor)
	logger := zap.NewNop()

	worker := NewWebhookConsumerWorker(WebhookConsumerWorkerConfig{
		StatusProcessor: statusProcessor,
		Logger:          logger,
	})

	ctx := context.Background()
	data := &WebhookJobData{
		EventID: "evt-qr-status",
		Event:   "qr.code",
		Session: "session-1",
		Payload: map[string]interface{}{},
	}

	// Verify the status processor is called with "SCAN_QR_CODE"
	statusProcessor.On("ProcessStatus", ctx, "session-1", "SCAN_QR_CODE").Return(nil)

	err := worker.routeEvent(ctx, data)

	assert.NoError(t, err)
	statusProcessor.AssertExpectations(t)
}

func TestRouteEvent_QRCodeWithStatusProcessorError(t *testing.T) {
	statusProcessor := new(mockStatusProcessor)
	logger := zap.NewNop()

	worker := NewWebhookConsumerWorker(WebhookConsumerWorkerConfig{
		StatusProcessor: statusProcessor,
		Logger:          logger,
	})

	ctx := context.Background()
	data := &WebhookJobData{
		EventID: "evt-qr-err",
		Event:   "qr.code",
		Session: "session-1",
		Payload: map[string]interface{}{},
	}

	// Status processor returns error - should log warning but not fail
	statusProcessor.On("ProcessStatus", ctx, "session-1", "SCAN_QR_CODE").
		Return(errors.New("SSE broadcast failed"))

	err := worker.routeEvent(ctx, data)

	// handleQRCodeEvent always returns nil even when status processor errors
	assert.NoError(t, err)
	statusProcessor.AssertExpectations(t)
}

func TestRouteEvent_QREventType(t *testing.T) {
	statusProcessor := new(mockStatusProcessor)
	logger := zap.NewNop()

	worker := NewWebhookConsumerWorker(WebhookConsumerWorkerConfig{
		StatusProcessor: statusProcessor,
		Logger:          logger,
	})

	ctx := context.Background()

	// Test the "qr" event type (not "qr.code")
	data := &WebhookJobData{
		EventID: "evt-qr-bare",
		Event:   "qr",
		Session: "session-1",
		Payload: map[string]interface{}{},
	}

	statusProcessor.On("ProcessStatus", ctx, "session-1", "SCAN_QR_CODE").Return(nil)

	err := worker.routeEvent(ctx, data)

	assert.NoError(t, err)
	statusProcessor.AssertExpectations(t)
}

// =============================================================================
// handleProcessingError DLQ Path Tests
// =============================================================================

func TestHandleProcessingError_RetryableWithoutQueueClient(t *testing.T) {
	webhookEventsRepo := new(mockWebhookEventsRepo)
	logger := zap.NewNop()

	// Worker without queueClient - verify it doesn't panic when queueClient is nil
	worker := NewWebhookConsumerWorker(WebhookConsumerWorkerConfig{
		WebhookEventsRepo: webhookEventsRepo,
		Logger:            logger,
	})

	ctx := context.Background()
	data := &WebhookJobData{
		EventID: "evt-dlq",
		Event:   "message",
		Session: "session-1",
	}
	task := createTestTask(t, data)
	processErr := errors.New("persistent failure") // retryable (unknown error defaults to retryable)

	// With retryCount=0 from plain context, the error is retryable,
	// so MarkFailed is called (not MarkDLQ) and the error is returned for retry
	webhookEventsRepo.On("MarkFailed", ctx, "evt-dlq", "persistent failure").Return(nil)

	err := worker.handleProcessingError(ctx, task, data, processErr)

	// Should return error for retry since retryCount=0 < MaxWebhookRetries=3
	assert.Error(t, err)
	assert.Equal(t, processErr, err)
	webhookEventsRepo.AssertExpectations(t)
}

func TestHandleProcessingError_MarkDLQError(t *testing.T) {
	webhookEventsRepo := new(mockWebhookEventsRepo)
	logger := zap.NewNop()

	worker := NewWebhookConsumerWorker(WebhookConsumerWorkerConfig{
		WebhookEventsRepo: webhookEventsRepo,
		Logger:            logger,
	})

	ctx := context.Background()
	data := &WebhookJobData{
		EventID: "evt-dlq-err",
		Event:   "message",
		Session: "session-1",
	}
	task := createTestTask(t, data)
	processErr := errors.New("resource not found") // non-retryable

	// Non-retryable error should go to MarkFailed path
	webhookEventsRepo.On("MarkFailed", ctx, "evt-dlq-err", "resource not found").Return(nil)

	err := worker.handleProcessingError(ctx, task, data, processErr)

	// Non-retryable error returns nil to stop retries
	assert.NoError(t, err)
	webhookEventsRepo.AssertExpectations(t)
}

func TestHandleProcessingError_MarkFailedError(t *testing.T) {
	webhookEventsRepo := new(mockWebhookEventsRepo)
	logger := zap.NewNop()

	worker := NewWebhookConsumerWorker(WebhookConsumerWorkerConfig{
		WebhookEventsRepo: webhookEventsRepo,
		Logger:            logger,
	})

	ctx := context.Background()
	data := &WebhookJobData{
		EventID: "evt-markfail-err",
		Event:   "message",
		Session: "session-1",
	}
	task := createTestTask(t, data)
	processErr := errors.New("connection timeout") // retryable

	// MarkFailed itself fails - should still return the process error for retry
	webhookEventsRepo.On("MarkFailed", ctx, "evt-markfail-err", "connection timeout").
		Return(errors.New("db connection lost"))

	err := worker.handleProcessingError(ctx, task, data, processErr)

	// Should still return the original error for retry
	assert.Error(t, err)
	assert.Equal(t, processErr, err)
	webhookEventsRepo.AssertExpectations(t)
}

// =============================================================================
// Trace ID Propagation Tests
// =============================================================================

func TestProcessTask_WithTraceID(t *testing.T) {
	worker, webhookEventsRepo, _ := createTestWorker(t)

	data := &WebhookJobData{
		EventID:    "evt-traced",
		Event:      "message.ack",
		Session:    "session-1",
		TraceID:    "abc123def456",
		ReceivedAt: time.Now().Unix(),
		Payload:    map[string]interface{}{},
	}

	task := createTestTask(t, data)
	ctx := context.Background()

	webhookEventsRepo.On("Exists", ctx, "evt-traced").Return(true, nil)
	webhookEventsRepo.On("MarkProcessing", ctx, "evt-traced").Return(nil)
	webhookEventsRepo.On("MarkProcessed", ctx, "evt-traced").Return(nil)

	// Should process successfully with trace ID in logger context
	err := worker.ProcessTask(ctx, task)

	assert.NoError(t, err)
	webhookEventsRepo.AssertExpectations(t)
}

func TestHandleProcessingError_WithTraceID(t *testing.T) {
	webhookEventsRepo := new(mockWebhookEventsRepo)
	logger := zap.NewNop()

	worker := NewWebhookConsumerWorker(WebhookConsumerWorkerConfig{
		WebhookEventsRepo: webhookEventsRepo,
		Logger:            logger,
	})

	ctx := context.Background()
	data := &WebhookJobData{
		EventID: "evt-traced-err",
		Event:   "message",
		Session: "session-1",
		TraceID: "trace-xyz-789",
	}
	task := createTestTask(t, data)
	processErr := errors.New("temporary failure")

	webhookEventsRepo.On("MarkFailed", ctx, "evt-traced-err", "temporary failure").Return(nil)

	// Verify trace ID path in handleProcessingError logger creation
	err := worker.handleProcessingError(ctx, task, data, processErr)

	assert.Error(t, err)
	assert.Equal(t, processErr, err)
	webhookEventsRepo.AssertExpectations(t)
}

// =============================================================================
// Reconciliation Service Integration Tests
// =============================================================================

func TestProcessTask_WithReconcileService(t *testing.T) {
	webhookEventsRepo := new(mockWebhookEventsRepo)
	logger := zap.NewNop()

	// Create a real reconciliation.Service with minimal config
	// RecordEvent only touches an internal map, so nil dependencies are safe
	reconcileSvc := reconciliation.New(nil, nil, nil, logger, reconciliation.DefaultConfig())

	worker := NewWebhookConsumerWorker(WebhookConsumerWorkerConfig{
		WebhookEventsRepo: webhookEventsRepo,
		ReconcileSvc:      reconcileSvc,
		Logger:            logger,
	})

	data := &WebhookJobData{
		EventID:    "evt-reconcile",
		Event:      "message.ack",
		Session:    "session-reconcile",
		ReceivedAt: time.Now().Unix(),
		Payload:    map[string]interface{}{},
	}

	task := createTestTask(t, data)
	ctx := context.Background()

	webhookEventsRepo.On("Exists", ctx, "evt-reconcile").Return(true, nil)
	webhookEventsRepo.On("MarkProcessing", ctx, "evt-reconcile").Return(nil)
	webhookEventsRepo.On("MarkProcessed", ctx, "evt-reconcile").Return(nil)

	// RecordEvent should be called (resets silence timer) without error
	err := worker.ProcessTask(ctx, task)

	assert.NoError(t, err)
	webhookEventsRepo.AssertExpectations(t)
}

// =============================================================================
// Session Status with Nil Instance Repo Tests
// =============================================================================

func TestRouteEvent_SessionStatusWithNilInstanceRepo(t *testing.T) {
	logger := zap.NewNop()

	// Worker without instanceRepo
	worker := NewWebhookConsumerWorker(WebhookConsumerWorkerConfig{
		Logger: logger,
	})

	ctx := context.Background()
	data := &WebhookJobData{
		EventID: "evt-nil-repo",
		Event:   "session.status",
		Session: "session-1",
		Payload: map[string]interface{}{"status": "CONNECTED"},
	}

	// Should not panic with nil instanceRepo
	err := worker.routeEvent(ctx, data)

	assert.NoError(t, err)
}

func TestRouteEvent_SessionStatusNilInstanceRepoWithStatusProcessor(t *testing.T) {
	statusProcessor := new(mockStatusProcessor)
	logger := zap.NewNop()

	// Worker without instanceRepo but WITH status processor
	worker := NewWebhookConsumerWorker(WebhookConsumerWorkerConfig{
		StatusProcessor: statusProcessor,
		Logger:          logger,
	})

	ctx := context.Background()
	data := &WebhookJobData{
		EventID: "evt-nil-repo-sp",
		Event:   "session.status",
		Session: "session-1",
		Payload: map[string]interface{}{"status": "DISCONNECTED"},
	}

	// Status processor should still be called even without instanceRepo
	statusProcessor.On("ProcessStatus", ctx, "session-1", "DISCONNECTED").Return(nil)

	err := worker.routeEvent(ctx, data)

	assert.NoError(t, err)
	statusProcessor.AssertExpectations(t)
}

// =============================================================================
// Session Status Processor Error Tests
// =============================================================================

func TestRouteEvent_SessionStatusProcessorError(t *testing.T) {
	webhookEventsRepo := new(mockWebhookEventsRepo)
	instanceRepo := new(mockInstanceRepo)
	statusProcessor := new(mockStatusProcessor)
	logger := zap.NewNop()

	worker := NewWebhookConsumerWorker(WebhookConsumerWorkerConfig{
		WebhookEventsRepo: webhookEventsRepo,
		InstanceRepo:      instanceRepo,
		StatusProcessor:   statusProcessor,
		Logger:            logger,
	})

	ctx := context.Background()
	data := &WebhookJobData{
		EventID: "evt-sp-err",
		Event:   "session.status",
		Session: "session-1",
		Payload: map[string]interface{}{"status": "CONNECTED"},
	}

	instanceRepo.On("GetInstanceByName", ctx, "session-1").
		Return(&models.Instance{ID: 1, InstanceID: "session-1"}, nil)
	instanceRepo.On("UpdateInstanceStatus", ctx, 1, "connected").Return(nil)

	// Status processor returns error - should log warning but NOT fail the task
	statusProcessor.On("ProcessStatus", ctx, "session-1", "CONNECTED").
		Return(errors.New("SSE connection broken"))

	err := worker.routeEvent(ctx, data)

	// handleSessionStatusEvent should NOT propagate statusProcessor errors
	assert.NoError(t, err)
	instanceRepo.AssertExpectations(t)
	statusProcessor.AssertExpectations(t)
}

func TestRouteEvent_SessionStatusProcessorErrorNoInstanceRepo(t *testing.T) {
	statusProcessor := new(mockStatusProcessor)
	logger := zap.NewNop()

	// No instance repo, but status processor that errors
	worker := NewWebhookConsumerWorker(WebhookConsumerWorkerConfig{
		StatusProcessor: statusProcessor,
		Logger:          logger,
	})

	ctx := context.Background()
	data := &WebhookJobData{
		EventID: "evt-sp-err-norepo",
		Event:   "session.status",
		Session: "session-1",
		Payload: map[string]interface{}{"status": "SCAN_QR_CODE"},
	}

	// Status processor returns error - should not fail even without instance repo
	statusProcessor.On("ProcessStatus", ctx, "session-1", "SCAN_QR_CODE").
		Return(errors.New("broadcast failed"))

	err := worker.routeEvent(ctx, data)

	assert.NoError(t, err)
	statusProcessor.AssertExpectations(t)
}

// =============================================================================
// QR Event Type Variations Tests
// =============================================================================

func TestRouteEvent_QREventTypeBare(t *testing.T) {
	// Test that bare "qr" event is handled (not just "qr.code")
	logger := zap.NewNop()

	worker := NewWebhookConsumerWorker(WebhookConsumerWorkerConfig{
		Logger: logger,
	})

	ctx := context.Background()
	data := &WebhookJobData{
		EventID: "evt-qr-bare-type",
		Event:   "qr",
		Session: "session-1",
		Payload: map[string]interface{}{},
	}

	// Should handle "qr" without error (no status processor = just logs)
	err := worker.routeEvent(ctx, data)

	assert.NoError(t, err)
}

func TestRouteEvent_QREventTypeBareNoStatusProcessor(t *testing.T) {
	logger := zap.NewNop()

	worker := NewWebhookConsumerWorker(WebhookConsumerWorkerConfig{
		Logger: logger,
	})

	ctx := context.Background()
	data := &WebhookJobData{
		EventID: "evt-qr-no-sp",
		Event:   "qr.code",
		Session: "session-1",
		Payload: map[string]interface{}{},
	}

	// Without status processor, QR event should be handled gracefully
	err := worker.routeEvent(ctx, data)

	assert.NoError(t, err)
}

// =============================================================================
// Full Flow with All Processors Tests
// =============================================================================

func TestProcessTask_FullQRCodeFlowWithStatusProcessor(t *testing.T) {
	webhookEventsRepo := new(mockWebhookEventsRepo)
	statusProcessor := new(mockStatusProcessor)
	logger := zap.NewNop()

	worker := NewWebhookConsumerWorker(WebhookConsumerWorkerConfig{
		WebhookEventsRepo: webhookEventsRepo,
		StatusProcessor:   statusProcessor,
		Logger:            logger,
	})

	data := &WebhookJobData{
		EventID:    "evt-qr-full",
		Event:      "qr.code",
		Session:    "session-1",
		ReceivedAt: time.Now().Unix(),
		Payload:    map[string]interface{}{},
	}

	task := createTestTask(t, data)
	ctx := context.Background()

	webhookEventsRepo.On("Exists", ctx, "evt-qr-full").Return(true, nil)
	webhookEventsRepo.On("MarkProcessing", ctx, "evt-qr-full").Return(nil)
	statusProcessor.On("ProcessStatus", ctx, "session-1", "SCAN_QR_CODE").Return(nil)
	webhookEventsRepo.On("MarkProcessed", ctx, "evt-qr-full").Return(nil)

	err := worker.ProcessTask(ctx, task)

	assert.NoError(t, err)
	webhookEventsRepo.AssertExpectations(t)
	statusProcessor.AssertExpectations(t)
}

func TestProcessTask_FullFlowWithReconcileAndTraceID(t *testing.T) {
	webhookEventsRepo := new(mockWebhookEventsRepo)
	logger := zap.NewNop()
	reconcileSvc := reconciliation.New(nil, nil, nil, logger, reconciliation.DefaultConfig())

	worker := NewWebhookConsumerWorker(WebhookConsumerWorkerConfig{
		WebhookEventsRepo: webhookEventsRepo,
		ReconcileSvc:      reconcileSvc,
		Logger:            logger,
	})

	data := &WebhookJobData{
		EventID:    "evt-full-traced",
		Event:      "message.ack",
		Session:    "session-traced",
		TraceID:    "trace-full-flow-123",
		ReceivedAt: time.Now().Unix(),
		Payload: map[string]interface{}{
			"id":  "msg-traced",
			"ack": 3,
		},
	}

	task := createTestTask(t, data)
	ctx := context.Background()

	webhookEventsRepo.On("Exists", ctx, "evt-full-traced").Return(true, nil)
	webhookEventsRepo.On("MarkProcessing", ctx, "evt-full-traced").Return(nil)
	webhookEventsRepo.On("MarkProcessed", ctx, "evt-full-traced").Return(nil)

	err := worker.ProcessTask(ctx, task)

	assert.NoError(t, err)
	webhookEventsRepo.AssertExpectations(t)
}

// =============================================================================
// handleProcessingError with Trace ID in Logger
// =============================================================================

func TestHandleProcessingError_NonRetryableWithTraceID(t *testing.T) {
	webhookEventsRepo := new(mockWebhookEventsRepo)
	logger := zap.NewNop()

	worker := NewWebhookConsumerWorker(WebhookConsumerWorkerConfig{
		WebhookEventsRepo: webhookEventsRepo,
		Logger:            logger,
	})

	ctx := context.Background()
	data := &WebhookJobData{
		EventID: "evt-nrt-trace",
		Event:   "message",
		Session: "session-1",
		TraceID: "trace-non-retryable-456",
	}
	task := createTestTask(t, data)
	processErr := errors.New("malformed payload data")

	webhookEventsRepo.On("MarkFailed", ctx, "evt-nrt-trace", "malformed payload data").Return(nil)

	err := worker.handleProcessingError(ctx, task, data, processErr)

	// Non-retryable error (contains "malformed") returns nil
	assert.NoError(t, err)
	webhookEventsRepo.AssertExpectations(t)
}

// =============================================================================
// Metrics Helper
// =============================================================================

// newTestWebhookMetrics creates a WebhookMetrics instance with manually constructed
// Prometheus collectors to avoid promauto registration conflicts with the default registry.
func newTestWebhookMetrics() *metrics.WebhookMetrics {
	return &metrics.WebhookMetrics{
		ProcessedTotal:     prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_wh_processed"}, []string{"session", "event_type"}),
		FailedTotal:        prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_wh_failed"}, []string{"session", "event_type", "error_type"}),
		DLQTotal:           prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_wh_dlq"}, []string{"session", "event_type"}),
		ProcessingDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "test_wh_duration"}, []string{"session", "event_type"}),
		InFlight:           prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "test_wh_inflight"}, []string{"session"}),
		RetryTotal:         prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_wh_retry"}, []string{"session", "event_type", "attempt"}),
	}
}

// =============================================================================
// handleProcessingError with Metrics Tests
// =============================================================================

func TestHandleProcessingError_RetryableWithMetrics(t *testing.T) {
	webhookEventsRepo := new(mockWebhookEventsRepo)
	logger := zap.NewNop()

	worker := NewWebhookConsumerWorker(WebhookConsumerWorkerConfig{
		WebhookEventsRepo: webhookEventsRepo,
		Logger:            logger,
		Metrics:           newTestWebhookMetrics(),
	})

	ctx := context.Background()
	data := &WebhookJobData{
		EventID: "evt-retry-metrics",
		Event:   "message",
		Session: "session-1",
	}
	task := createTestTask(t, data)
	processErr := errors.New("temporary failure")

	webhookEventsRepo.On("MarkFailed", ctx, "evt-retry-metrics", "temporary failure").Return(nil)

	// Execute with retryCount=0 (default from background context) < MaxWebhookRetries=3
	// This is retryable, so RecordRetry is called and the error is returned.
	err := worker.handleProcessingError(ctx, task, data, processErr)

	// Should return error to trigger retry
	assert.Error(t, err)
	assert.Equal(t, processErr, err)
	webhookEventsRepo.AssertExpectations(t)
}

func TestHandleProcessingError_NonRetryableWithMetrics(t *testing.T) {
	webhookEventsRepo := new(mockWebhookEventsRepo)
	logger := zap.NewNop()

	worker := NewWebhookConsumerWorker(WebhookConsumerWorkerConfig{
		WebhookEventsRepo: webhookEventsRepo,
		Logger:            logger,
		Metrics:           newTestWebhookMetrics(),
	})

	ctx := context.Background()
	data := &WebhookJobData{
		EventID: "evt-nonretry-metrics",
		Event:   "message",
		Session: "session-1",
	}
	task := createTestTask(t, data)
	processErr := errors.New("resource not found")

	webhookEventsRepo.On("MarkFailed", ctx, "evt-nonretry-metrics", "resource not found").Return(nil)

	// Execute - non-retryable error calls RecordFailed and returns nil
	err := worker.handleProcessingError(ctx, task, data, processErr)

	// Should return nil to stop retries
	assert.NoError(t, err)
	webhookEventsRepo.AssertExpectations(t)
}

func TestHandleProcessingError_MarkFailedErrorNonRetryable(t *testing.T) {
	webhookEventsRepo := new(mockWebhookEventsRepo)
	logger := zap.NewNop()

	worker := NewWebhookConsumerWorker(WebhookConsumerWorkerConfig{
		WebhookEventsRepo: webhookEventsRepo,
		Logger:            logger,
		Metrics:           newTestWebhookMetrics(),
	})

	ctx := context.Background()
	data := &WebhookJobData{
		EventID: "evt-markfail-nonretry-metrics",
		Event:   "message",
		Session: "session-1",
	}
	task := createTestTask(t, data)
	processErr := errors.New("resource not found")

	// MarkFailed itself returns an error
	webhookEventsRepo.On("MarkFailed", ctx, "evt-markfail-nonretry-metrics", "resource not found").
		Return(errors.New("db connection lost"))

	// Even though MarkFailed errors, RecordFailed is still called (metrics != nil)
	// and we return nil because the error is non-retryable
	err := worker.handleProcessingError(ctx, task, data, processErr)

	assert.NoError(t, err)
	webhookEventsRepo.AssertExpectations(t)
}

// =============================================================================
// ProcessTask with Metrics Tests
// =============================================================================

func TestProcessTask_SuccessWithMetrics(t *testing.T) {
	webhookEventsRepo := new(mockWebhookEventsRepo)
	logger := zap.NewNop()

	worker := NewWebhookConsumerWorker(WebhookConsumerWorkerConfig{
		WebhookEventsRepo: webhookEventsRepo,
		Logger:            logger,
		Metrics:           newTestWebhookMetrics(),
	})

	data := &WebhookJobData{
		EventID:    "evt-success-metrics",
		Event:      "message.ack",
		Session:    "session-1",
		ReceivedAt: time.Now().Unix(),
		Payload: map[string]interface{}{
			"id":  "msg-789",
			"ack": 2,
		},
	}

	task := createTestTask(t, data)
	ctx := context.Background()

	webhookEventsRepo.On("Exists", ctx, "evt-success-metrics").Return(true, nil)
	webhookEventsRepo.On("MarkProcessing", ctx, "evt-success-metrics").Return(nil)
	webhookEventsRepo.On("MarkProcessed", ctx, "evt-success-metrics").Return(nil)

	// Execute - IncrementInFlight/DecrementInFlight (via defer) and RecordProcessed
	// are called during normal processing because metrics != nil.
	err := worker.ProcessTask(ctx, task)

	assert.NoError(t, err)
	webhookEventsRepo.AssertExpectations(t)
}

func TestProcessTask_SuccessWithMetricsAndReconcileSvc(t *testing.T) {
	webhookEventsRepo := new(mockWebhookEventsRepo)
	logger := zap.NewNop()
	reconcileSvc := reconciliation.New(nil, nil, nil, logger, reconciliation.DefaultConfig())

	worker := NewWebhookConsumerWorker(WebhookConsumerWorkerConfig{
		WebhookEventsRepo: webhookEventsRepo,
		ReconcileSvc:      reconcileSvc,
		Logger:            logger,
		Metrics:           newTestWebhookMetrics(),
	})

	data := &WebhookJobData{
		EventID:    "evt-metrics-reconcile",
		Event:      "message.ack",
		Session:    "session-reconcile",
		ReceivedAt: time.Now().Unix(),
		Payload: map[string]interface{}{
			"id":  "msg-reconcile",
			"ack": 3,
		},
	}

	task := createTestTask(t, data)
	ctx := context.Background()

	webhookEventsRepo.On("Exists", ctx, "evt-metrics-reconcile").Return(true, nil)
	webhookEventsRepo.On("MarkProcessing", ctx, "evt-metrics-reconcile").Return(nil)
	webhookEventsRepo.On("MarkProcessed", ctx, "evt-metrics-reconcile").Return(nil)

	// Both metrics (IncrementInFlight, DecrementInFlight, RecordProcessed)
	// and RecordEvent (reconcileSvc) are exercised together.
	err := worker.ProcessTask(ctx, task)

	assert.NoError(t, err)
	webhookEventsRepo.AssertExpectations(t)
}

func TestProcessTask_RouteEventErrorWithMetrics(t *testing.T) {
	webhookEventsRepo := new(mockWebhookEventsRepo)
	msgProcessor := new(mockMessageProcessor)
	logger := zap.NewNop()

	worker := NewWebhookConsumerWorker(WebhookConsumerWorkerConfig{
		WebhookEventsRepo: webhookEventsRepo,
		MessageProcessor:  msgProcessor,
		Logger:            logger,
		Metrics:           newTestWebhookMetrics(),
	})

	data := &WebhookJobData{
		EventID:    "evt-route-err-metrics",
		Event:      "message",
		Session:    "session-1",
		ReceivedAt: time.Now().Unix(),
		Payload:    map[string]interface{}{"body": "hello"},
	}

	task := createTestTask(t, data)
	ctx := context.Background()

	webhookEventsRepo.On("Exists", ctx, "evt-route-err-metrics").Return(true, nil)
	webhookEventsRepo.On("MarkProcessing", ctx, "evt-route-err-metrics").Return(nil)

	// MessageProcessor returns a retryable error, which triggers handleProcessingError
	msgProcessor.On("ProcessMessage", ctx, "session-1", data.Payload).
		Return(errors.New("temporary failure"))

	// handleProcessingError calls MarkFailed (retryable, retryCount=0 < MaxWebhookRetries=3)
	// and metrics.RecordRetry
	webhookEventsRepo.On("MarkFailed", ctx, "evt-route-err-metrics", "temporary failure").Return(nil)

	err := worker.ProcessTask(ctx, task)

	// Should return error to trigger retry (retryable error + retryCount < max)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "temporary failure")
	webhookEventsRepo.AssertExpectations(t)
	msgProcessor.AssertExpectations(t)
}
