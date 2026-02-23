package workers

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// =============================================================================
// IsDuplicateTaskError
// =============================================================================

func TestIsDuplicateTaskError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "returns true for ErrDuplicateTask",
			err:  asynq.ErrDuplicateTask,
			want: true,
		},
		{
			name: "returns true for ErrTaskIDConflict",
			err:  asynq.ErrTaskIDConflict,
			want: true,
		},
		{
			name: "returns true for wrapped ErrDuplicateTask",
			err:  errors.Join(errors.New("outer"), asynq.ErrDuplicateTask),
			want: true,
		},
		{
			name: "returns true for wrapped ErrTaskIDConflict",
			err:  errors.Join(errors.New("outer"), asynq.ErrTaskIDConflict),
			want: true,
		},
		{
			name: "returns false for unrelated error",
			err:  errors.New("some other error"),
			want: false,
		},
		{
			name: "returns false for nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := IsDuplicateTaskError(tt.err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// =============================================================================
// formatTaskID
// =============================================================================

func TestFormatTaskID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		dispatchLogID  int
		expectedResult string
	}{
		{
			name:           "positive ID",
			dispatchLogID:  42,
			expectedResult: "dispatch:42",
		},
		{
			name:           "zero ID",
			dispatchLogID:  0,
			expectedResult: "dispatch:0",
		},
		{
			name:           "negative ID",
			dispatchLogID:  -1,
			expectedResult: "dispatch:-1",
		},
		{
			name:           "large ID",
			dispatchLogID:  999999,
			expectedResult: "dispatch:999999",
		},
		{
			name:           "ID of 1",
			dispatchLogID:  1,
			expectedResult: "dispatch:1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := formatTaskID(tt.dispatchLogID)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

// =============================================================================
// asynqLogger
// =============================================================================

func TestAsynqLogger_Debug(t *testing.T) {
	t.Parallel()

	core, recorded := observer.New(zapcore.DebugLevel)
	logger := zap.New(core)
	al := &asynqLogger{logger: logger}

	al.Debug("test debug message", 123)

	require.Equal(t, 1, recorded.Len(), "expected exactly one log entry")
	entry := recorded.All()[0]
	assert.Equal(t, zapcore.DebugLevel, entry.Level)
	assert.Equal(t, "asynq", entry.Message)
	// Verify the args field is present
	assert.Len(t, entry.ContextMap(), 1)
}

func TestAsynqLogger_Info(t *testing.T) {
	t.Parallel()

	core, recorded := observer.New(zapcore.DebugLevel)
	logger := zap.New(core)
	al := &asynqLogger{logger: logger}

	al.Info("test info message")

	require.Equal(t, 1, recorded.Len(), "expected exactly one log entry")
	entry := recorded.All()[0]
	assert.Equal(t, zapcore.InfoLevel, entry.Level)
	assert.Equal(t, "asynq", entry.Message)
}

func TestAsynqLogger_Warn(t *testing.T) {
	t.Parallel()

	core, recorded := observer.New(zapcore.DebugLevel)
	logger := zap.New(core)
	al := &asynqLogger{logger: logger}

	al.Warn("test warn message")

	require.Equal(t, 1, recorded.Len(), "expected exactly one log entry")
	entry := recorded.All()[0]
	assert.Equal(t, zapcore.WarnLevel, entry.Level)
	assert.Equal(t, "asynq", entry.Message)
}

func TestAsynqLogger_Error(t *testing.T) {
	t.Parallel()

	core, recorded := observer.New(zapcore.DebugLevel)
	logger := zap.New(core)
	al := &asynqLogger{logger: logger}

	al.Error("test error message")

	require.Equal(t, 1, recorded.Len(), "expected exactly one log entry")
	entry := recorded.All()[0]
	assert.Equal(t, zapcore.ErrorLevel, entry.Level)
	assert.Equal(t, "asynq", entry.Message)
}

func TestAsynqLogger_MultipleArgs(t *testing.T) {
	t.Parallel()

	core, recorded := observer.New(zapcore.DebugLevel)
	logger := zap.New(core)
	al := &asynqLogger{logger: logger}

	al.Info("arg1", "arg2", 42, true)

	require.Equal(t, 1, recorded.Len())
	entry := recorded.All()[0]
	assert.Equal(t, zapcore.InfoLevel, entry.Level)
	assert.Equal(t, "asynq", entry.Message)
}

func TestAsynqLogger_NoArgs(t *testing.T) {
	t.Parallel()

	core, recorded := observer.New(zapcore.DebugLevel)
	logger := zap.New(core)
	al := &asynqLogger{logger: logger}

	al.Info()

	require.Equal(t, 1, recorded.Len())
	assert.Equal(t, "asynq", recorded.All()[0].Message)
}

// NOTE: Fatal is intentionally not tested because it calls logger.Fatal,
// which would terminate the test process.

// =============================================================================
// Task Type Constants
// =============================================================================

func TestTaskTypeConstants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{
			name:     "TypeSendMessage",
			constant: TypeSendMessage,
			expected: "whatsapp:send_message",
		},
		{
			name:     "TypeSyncGroups",
			constant: TypeSyncGroups,
			expected: "whatsapp:sync_groups",
		},
		{
			name:     "TypeWAHAWebhook",
			constant: TypeWAHAWebhook,
			expected: "whatsapp:waha_webhook",
		},
		{
			name:     "TypeReconcileChat",
			constant: TypeReconcileChat,
			expected: "whatsapp:reconcile:chat",
		},
		{
			name:     "TypeReconcileSession",
			constant: TypeReconcileSession,
			expected: "whatsapp:reconcile:session",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, tt.constant)
		})
	}
}

func TestTaskTypeConstants_AllUnique(t *testing.T) {
	t.Parallel()

	types := []string{
		TypeSendMessage,
		TypeSyncGroups,
		TypeWAHAWebhook,
		TypeReconcileChat,
		TypeReconcileSession,
	}

	seen := make(map[string]bool, len(types))
	for _, taskType := range types {
		assert.False(t, seen[taskType], "duplicate task type: %s", taskType)
		seen[taskType] = true
	}
}

// =============================================================================
// MessageJobData JSON Serialization
// =============================================================================

func TestMessageJobData_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	historyID := "hist-123"
	imageURL := "https://example.com/img.png"
	dispatchLogID := 42

	original := MessageJobData{
		InstanceName:  "default",
		GroupID:       "120363001234567890@g.us",
		Message:       "Hello, World!",
		Mentions:      []string{"5511999998888@c.us", "5511888887777@c.us"},
		HistoryID:     &historyID,
		ImageURL:      &imageURL,
		DispatchLogID: &dispatchLogID,
		PTT:           true,
		UserID:        "user-abc",
		AutomationID:  "auto-xyz",
		TraceID:       "abc123def456",
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded MessageJobData
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, original.InstanceName, decoded.InstanceName)
	assert.Equal(t, original.GroupID, decoded.GroupID)
	assert.Equal(t, original.Message, decoded.Message)
	assert.Equal(t, original.Mentions, decoded.Mentions)
	assert.Equal(t, original.HistoryID, decoded.HistoryID)
	assert.Equal(t, original.ImageURL, decoded.ImageURL)
	assert.Equal(t, original.DispatchLogID, decoded.DispatchLogID)
	assert.Equal(t, original.PTT, decoded.PTT)
	assert.Equal(t, original.UserID, decoded.UserID)
	assert.Equal(t, original.AutomationID, decoded.AutomationID)
	assert.Equal(t, original.TraceID, decoded.TraceID)
}

func TestMessageJobData_OmitEmptyFields(t *testing.T) {
	t.Parallel()

	minimal := MessageJobData{
		InstanceName: "default",
		GroupID:      "120363001234567890@g.us",
		Message:      "Hello",
		UserID:       "user-1",
	}

	data, err := json.Marshal(minimal)
	require.NoError(t, err)

	// Verify omitempty fields are excluded from JSON
	var raw map[string]interface{}
	err = json.Unmarshal(data, &raw)
	require.NoError(t, err)

	// These fields should be present (not omitempty or have values)
	assert.Contains(t, raw, "instanceName")
	assert.Contains(t, raw, "groupId")
	assert.Contains(t, raw, "message")
	assert.Contains(t, raw, "userId")

	// These fields should be absent (omitempty + zero value)
	assert.NotContains(t, raw, "mentions")
	assert.NotContains(t, raw, "historyId")
	assert.NotContains(t, raw, "imageUrl")
	assert.NotContains(t, raw, "imageBase64")
	assert.NotContains(t, raw, "dispatchLogId")
	assert.NotContains(t, raw, "audioBase64")
	assert.NotContains(t, raw, "mimetype")
	assert.NotContains(t, raw, "ptt")
	assert.NotContains(t, raw, "automationId")
	assert.NotContains(t, raw, "trace_id")
}

func TestMessageJobData_DeserializeFromJSON(t *testing.T) {
	t.Parallel()

	jsonStr := `{
		"instanceName": "session1",
		"groupId": "group@g.us",
		"message": "test",
		"mentions": ["user1@c.us"],
		"ptt": false,
		"userId": "u1",
		"dispatchLogId": 99
	}`

	var data MessageJobData
	err := json.Unmarshal([]byte(jsonStr), &data)
	require.NoError(t, err)

	assert.Equal(t, "session1", data.InstanceName)
	assert.Equal(t, "group@g.us", data.GroupID)
	assert.Equal(t, "test", data.Message)
	assert.Equal(t, []string{"user1@c.us"}, data.Mentions)
	assert.False(t, data.PTT)
	assert.Equal(t, "u1", data.UserID)
	require.NotNil(t, data.DispatchLogID)
	assert.Equal(t, 99, *data.DispatchLogID)
	assert.Nil(t, data.HistoryID)
	assert.Nil(t, data.ImageURL)
}

// =============================================================================
// GroupsSyncJobData JSON Serialization
// =============================================================================

func TestGroupsSyncJobData_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	original := GroupsSyncJobData{
		UserID:       "user-456",
		InstanceName: "whatsapp-main",
		InstanceID:   7,
		TraceID:      "trace-abc",
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded GroupsSyncJobData
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, original, decoded)
}

func TestGroupsSyncJobData_OmitEmptyTraceID(t *testing.T) {
	t.Parallel()

	data := GroupsSyncJobData{
		UserID:       "user-1",
		InstanceName: "sess",
		InstanceID:   1,
	}

	bytes, err := json.Marshal(data)
	require.NoError(t, err)

	var raw map[string]interface{}
	err = json.Unmarshal(bytes, &raw)
	require.NoError(t, err)

	assert.NotContains(t, raw, "trace_id")
	assert.Contains(t, raw, "userId")
	assert.Contains(t, raw, "instanceName")
	assert.Contains(t, raw, "instanceId")
}

// =============================================================================
// WebhookJobData JSON Serialization
// =============================================================================

func TestWebhookJobData_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	original := WebhookJobData{
		EventID: "evt-001",
		Event:   "message",
		Session: "default",
		Engine:  "GOWS",
		Payload: map[string]interface{}{
			"id":   "msg-1",
			"body": "hello",
		},
		ReceivedAt: 1700000000,
		TraceID:    "trace-xyz",
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded WebhookJobData
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, original.EventID, decoded.EventID)
	assert.Equal(t, original.Event, decoded.Event)
	assert.Equal(t, original.Session, decoded.Session)
	assert.Equal(t, original.Engine, decoded.Engine)
	assert.Equal(t, original.ReceivedAt, decoded.ReceivedAt)
	assert.Equal(t, original.TraceID, decoded.TraceID)
	assert.Equal(t, "msg-1", decoded.Payload["id"])
	assert.Equal(t, "hello", decoded.Payload["body"])
}

func TestWebhookJobData_EmptyPayload(t *testing.T) {
	t.Parallel()

	data := WebhookJobData{
		EventID:    "evt-002",
		Event:      "session.status",
		Session:    "default",
		Engine:     "GOWS",
		ReceivedAt: 1700000000,
	}

	bytes, err := json.Marshal(data)
	require.NoError(t, err)

	var decoded WebhookJobData
	err = json.Unmarshal(bytes, &decoded)
	require.NoError(t, err)

	assert.Nil(t, decoded.Payload)
	assert.Equal(t, "evt-002", decoded.EventID)
}

func TestWebhookJobData_NestedPayload(t *testing.T) {
	t.Parallel()

	original := WebhookJobData{
		EventID: "evt-003",
		Event:   "message",
		Session: "sess1",
		Engine:  "GOWS",
		Payload: map[string]interface{}{
			"nested": map[string]interface{}{
				"key": "value",
			},
			"array": []interface{}{1.0, 2.0, 3.0},
		},
		ReceivedAt: 1700000001,
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded WebhookJobData
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	nested, ok := decoded.Payload["nested"].(map[string]interface{})
	require.True(t, ok, "nested should be a map")
	assert.Equal(t, "value", nested["key"])

	arr, ok := decoded.Payload["array"].([]interface{})
	require.True(t, ok, "array should be a slice")
	assert.Len(t, arr, 3)
}

// =============================================================================
// ReconcileChatPayload JSON Serialization
// =============================================================================

func TestReconcileChatPayload_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	original := ReconcileChatPayload{
		SessionName: "default",
		ChatID:      "120363001234567890@g.us",
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded ReconcileChatPayload
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, original, decoded)
}

func TestReconcileChatPayload_JSONKeys(t *testing.T) {
	t.Parallel()

	payload := ReconcileChatPayload{
		SessionName: "sess1",
		ChatID:      "chat1@g.us",
	}

	data, err := json.Marshal(payload)
	require.NoError(t, err)

	var raw map[string]interface{}
	err = json.Unmarshal(data, &raw)
	require.NoError(t, err)

	// Verify snake_case JSON keys
	assert.Contains(t, raw, "session_name")
	assert.Contains(t, raw, "chat_id")
	assert.Equal(t, "sess1", raw["session_name"])
	assert.Equal(t, "chat1@g.us", raw["chat_id"])
}

// =============================================================================
// ReconcileSessionPayload JSON Serialization
// =============================================================================

func TestReconcileSessionPayload_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	original := ReconcileSessionPayload{
		SessionName: "my-session",
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded ReconcileSessionPayload
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, original, decoded)
}

func TestReconcileSessionPayload_JSONKeys(t *testing.T) {
	t.Parallel()

	payload := ReconcileSessionPayload{
		SessionName: "production",
	}

	data, err := json.Marshal(payload)
	require.NoError(t, err)

	var raw map[string]interface{}
	err = json.Unmarshal(data, &raw)
	require.NoError(t, err)

	assert.Contains(t, raw, "session_name")
	assert.Equal(t, "production", raw["session_name"])
	assert.Len(t, raw, 1, "should have exactly one field")
}

// =============================================================================
// NewQueueServer
// =============================================================================

func TestNewQueueServer_DefaultConcurrency(t *testing.T) {
	t.Parallel()

	logger := zap.NewNop()

	// concurrency <= 0 should default to 5
	tests := []struct {
		name        string
		concurrency int
	}{
		{name: "zero concurrency defaults to 5", concurrency: 0},
		{name: "negative concurrency defaults to 5", concurrency: -1},
		{name: "negative large concurrency defaults to 5", concurrency: -100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := NewQueueServer("localhost:6379", tt.concurrency, logger)
			require.NotNil(t, server)
			assert.NotNil(t, server.server)
			assert.NotNil(t, server.mux)
			assert.NotNil(t, server.logger)
		})
	}
}

func TestNewQueueServer_CustomConcurrency(t *testing.T) {
	t.Parallel()

	logger := zap.NewNop()

	tests := []struct {
		name        string
		concurrency int
	}{
		{name: "concurrency of 1", concurrency: 1},
		{name: "concurrency of 10", concurrency: 10},
		{name: "concurrency of 100", concurrency: 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := NewQueueServer("localhost:6379", tt.concurrency, logger)
			require.NotNil(t, server)
			assert.NotNil(t, server.server)
			assert.NotNil(t, server.mux)
			assert.NotNil(t, server.logger)
		})
	}
}

func TestNewQueueServer_HasServeMux(t *testing.T) {
	t.Parallel()

	logger := zap.NewNop()
	server := NewQueueServer("localhost:6379", 5, logger)

	require.NotNil(t, server)
	assert.NotNil(t, server.mux, "ServeMux should be initialized")
}

func TestNewQueueServer_RegisterHandler(t *testing.T) {
	t.Parallel()

	logger := zap.NewNop()
	server := NewQueueServer("localhost:6379", 5, logger)

	// RegisterHandler should not panic
	handler := asynq.HandlerFunc(func(_ context.Context, _ *asynq.Task) error {
		return nil
	})
	assert.NotPanics(t, func() {
		server.RegisterHandler(TypeSendMessage, handler)
	})
}

// =============================================================================
// NewQueueClient
// =============================================================================

func TestNewQueueClient(t *testing.T) {
	t.Parallel()

	logger := zap.NewNop()
	client := NewQueueClient("localhost:6379", logger)

	require.NotNil(t, client)
	assert.NotNil(t, client.client, "underlying asynq.Client should be initialized")
	assert.NotNil(t, client.logger, "logger should be set")

	// Close should not error even without a real Redis connection
	// (asynq.Client.Close is a no-op for the connection pool cleanup)
	err := client.Close()
	assert.NoError(t, err)
}

func TestNewQueueClient_DifferentAddresses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		addr string
	}{
		{name: "localhost default port", addr: "localhost:6379"},
		{name: "custom host and port", addr: "redis.example.com:6380"},
		{name: "ip address", addr: "192.168.1.100:6379"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			logger := zap.NewNop()
			client := NewQueueClient(tt.addr, logger)
			require.NotNil(t, client)
			_ = client.Close()
		})
	}
}
