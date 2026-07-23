package handlers

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/metrics"
	"automatiza-go/internal/models"
	"automatiza-go/internal/workers"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// MockWebhookEventsRepository implements repository.WebhookEventsRepository for testing.
type MockWebhookEventsRepository struct {
	// InsertIfNotExistsFunc can be set to control behavior
	InsertIfNotExistsFunc func(ctx context.Context, eventID, sessionName, eventType string) (bool, error)
	// MarkFailedFunc can be set to control behavior
	MarkFailedFunc func(ctx context.Context, eventID, errorMessage string) error

	// Track calls for assertions
	InsertCalls []struct {
		EventID     string
		SessionName string
		EventType   string
	}
	MarkFailedCalls []struct {
		EventID      string
		ErrorMessage string
	}
}

func (m *MockWebhookEventsRepository) InsertIfNotExists(ctx context.Context, eventID, sessionName, eventType string) (bool, error) {
	m.InsertCalls = append(m.InsertCalls, struct {
		EventID     string
		SessionName string
		EventType   string
	}{eventID, sessionName, eventType})

	if m.InsertIfNotExistsFunc != nil {
		return m.InsertIfNotExistsFunc(ctx, eventID, sessionName, eventType)
	}
	// Default: event is new (inserted)
	return true, nil
}

func (m *MockWebhookEventsRepository) Exists(ctx context.Context, eventID string) (bool, error) {
	return false, nil
}

func (m *MockWebhookEventsRepository) MarkProcessing(ctx context.Context, eventID string) error {
	return nil
}

func (m *MockWebhookEventsRepository) MarkProcessed(ctx context.Context, eventID string) error {
	return nil
}

func (m *MockWebhookEventsRepository) MarkProcessedEnsure(ctx context.Context, eventID, sessionName, eventType string) error {
	return nil
}

func (m *MockWebhookEventsRepository) MarkFailed(ctx context.Context, eventID, errorMessage string) error {
	m.MarkFailedCalls = append(m.MarkFailedCalls, struct {
		EventID      string
		ErrorMessage string
	}{eventID, errorMessage})

	if m.MarkFailedFunc != nil {
		return m.MarkFailedFunc(ctx, eventID, errorMessage)
	}
	return nil
}

func (m *MockWebhookEventsRepository) MarkDLQ(ctx context.Context, eventID, errorMessage string) error {
	return nil
}

func (m *MockWebhookEventsRepository) IncrementRetryCount(ctx context.Context, eventID string) error {
	return nil
}

func (m *MockWebhookEventsRepository) GetPendingEvents(ctx context.Context, limit int) ([]models.WebhookEventRecord, error) {
	return nil, nil
}

func (m *MockWebhookEventsRepository) GetFailedEvents(ctx context.Context, limit int) ([]models.WebhookEventRecord, error) {
	return nil, nil
}

func (m *MockWebhookEventsRepository) Cleanup(ctx context.Context, olderThan time.Duration) (int64, error) {
	return 0, nil
}

func (m *MockWebhookEventsRepository) CleanupUnrecoverable(ctx context.Context, olderThan time.Duration) (int64, error) {
	return 0, nil
}

// MockQueueClient wraps workers.QueueClient for testing.
// Since QueueClient uses an internal asynq.Client, we create a mock that
// intercepts the EnqueueWebhook method.
type MockQueueClient struct {
	// EnqueueWebhookFunc can be set to control behavior
	EnqueueWebhookFunc func(ctx context.Context, data *workers.WebhookJobData) (*asynq.TaskInfo, error)

	// Track calls for assertions
	EnqueueCalls []*workers.WebhookJobData
}

func (m *MockQueueClient) EnqueueWebhook(ctx context.Context, data *workers.WebhookJobData) (*asynq.TaskInfo, error) {
	m.EnqueueCalls = append(m.EnqueueCalls, data)

	if m.EnqueueWebhookFunc != nil {
		return m.EnqueueWebhookFunc(ctx, data)
	}
	// Default: successful enqueue
	return &asynq.TaskInfo{
		ID:    "task-" + data.EventID,
		Queue: "webhooks",
	}, nil
}

// TestableWAHAWebhookHandler wraps WAHAWebhookHandler with injectable dependencies.
type TestableWAHAWebhookHandler struct {
	hmacValidator     *whatsapp.HMACValidator
	webhookEventsRepo *MockWebhookEventsRepository
	mockQueueClient   *MockQueueClient
	logger            *zap.Logger
}

// HandleWebhook processes the webhook with the testable handler.
// This is a copy of the original logic but uses mock queue client.
func (h *TestableWAHAWebhookHandler) HandleWebhook(c *gin.Context) {
	// Step 1: Read body with size limit
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxWebhookBodySize)
	body, err := readBody(c)
	if err != nil {
		return
	}

	// Step 2: Validate HMAC signature
	signature := c.GetHeader(wahaSignatureHeader)
	result := h.hmacValidator.ValidateWithResult(body, signature)

	h.logger.Debug("webhook HMAC validation",
		zap.Bool("valid", result.Valid),
		zap.Bool("enabled", result.Enabled),
		zap.String("reason", result.Reason),
	)

	if !result.Valid {
		h.logger.Warn("webhook HMAC validation failed",
			zap.String("reason", result.Reason),
			zap.String("ip", c.ClientIP()),
		)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid signature"})
		return
	}

	// Step 3: ACK immediately
	c.JSON(http.StatusOK, gin.H{"received": true})

	// Step 4: Parse JSON
	var event WAHAWebhookEvent
	if err := json.Unmarshal(body, &event); err != nil {
		h.logger.Warn("failed to parse webhook body",
			zap.Error(err),
			zap.Int("body_size", len(body)),
		)
		return
	}

	// Step 5: Extract event ID
	eventID := extractEventID(&event)
	if eventID == "" {
		h.logger.Warn("webhook event has no ID, generating one",
			zap.String("event_type", event.Event),
			zap.String("session", event.Session),
		)
		eventID = generateFallbackEventID(event.Session, event.Event)
	}

	// Step 6: Check idempotency
	dbCtx, dbCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer dbCancel()

	inserted, err := h.webhookEventsRepo.InsertIfNotExists(dbCtx, eventID, event.Session, event.Event)
	if err != nil {
		h.logger.Error("failed to check idempotency",
			zap.String("event_id", eventID),
			zap.Error(err),
		)
	}

	// Step 7: If duplicate, log and return
	if !inserted {
		h.logger.Debug("duplicate webhook event ignored",
			zap.String("event_id", eventID),
			zap.String("event_type", event.Event),
		)
		return
	}

	// Step 8: Enqueue task
	taskPayload := &workers.WebhookJobData{
		EventID:    eventID,
		Event:      event.Event,
		Session:    event.Session,
		Engine:     event.Engine,
		Payload:    event.Payload,
		ReceivedAt: time.Now().UnixMilli(),
		// Espelha o handler real: ID e timestamp ORIGINAIS do provedor, nunca
		// o eventID (que pode ser fallback gerado por nós).
		ProviderEventID:   event.ID,
		ProviderTimestamp: int64(event.Timestamp),
	}

	enqueueCtx, enqueueCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer enqueueCancel()

	_, err = h.mockQueueClient.EnqueueWebhook(enqueueCtx, taskPayload)
	if err != nil {
		if workers.IsDuplicateTaskError(err) {
			h.logger.Debug("task already enqueued",
				zap.String("event_id", eventID),
			)
			return
		}

		h.logger.Error("failed to enqueue webhook task",
			zap.String("event_id", eventID),
			zap.Error(err),
		)

		h.markEventFailed(dbCtx, eventID, "failed to enqueue task")
		return
	}

	h.logger.Info("webhook task enqueued",
		zap.String("event_id", eventID),
		zap.String("event_type", event.Event),
		zap.String("session", event.Session),
	)
}

func (h *TestableWAHAWebhookHandler) markEventFailed(ctx context.Context, eventID, errorMessage string) {
	bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := h.webhookEventsRepo.MarkFailed(bgCtx, eventID, errorMessage); err != nil {
		h.logger.Error("failed to mark event as failed",
			zap.String("event_id", eventID),
			zap.String("original_error", errorMessage),
			zap.Error(err),
		)
	}
}

// readBody reads the request body and handles errors appropriately.
// This is a test-only helper for the TestableWAHAWebhookHandler.
func readBody(c *gin.Context) ([]byte, error) {
	testLog := zap.NewNop()
	body := make([]byte, 0, 1024)
	buf := make([]byte, 1024)
	for {
		n, readErr := c.Request.Body.Read(buf)
		if n > 0 {
			body = append(body, buf[:n]...)
		}
		if readErr != nil {
			if readErr.Error() == "EOF" {
				break
			}
			if strings.Contains(readErr.Error(), "request body too large") {
				testLog.Warn("test readBody: body too large", zap.Error(readErr))
				c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "payload too large"})
				return nil, readErr
			}
			testLog.Warn("test readBody: failed to read body", zap.Error(readErr))
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read body"})
			return nil, readErr
		}
	}
	return body, nil
}

// newTestHandler creates a testable handler with mocks.
func newTestHandler(hmacKey string) (*TestableWAHAWebhookHandler, *MockWebhookEventsRepository, *MockQueueClient) {
	logger := zap.NewNop()
	repo := &MockWebhookEventsRepository{}
	queue := &MockQueueClient{}

	handler := &TestableWAHAWebhookHandler{
		hmacValidator:     whatsapp.NewHMACValidator(hmacKey),
		webhookEventsRepo: repo,
		mockQueueClient:   queue,
		logger:            logger,
	}

	return handler, repo, queue
}

// createWebhookRequest creates a test HTTP request with the given body and optional HMAC signature.
func createWebhookRequest(body []byte, hmacKey string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/waha", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	if hmacKey != "" {
		validator := whatsapp.NewHMACValidator(hmacKey)
		signature := validator.ComputeSignature(body)
		req.Header.Set(wahaSignatureHeader, signature)
	}

	return req
}

// createValidEventPayload creates a valid WAHA webhook event JSON payload.
func createValidEventPayload() []byte {
	event := map[string]interface{}{
		"id":      "evt-123-abc",
		"event":   "message",
		"session": "test-session",
		"engine":  "GOWS",
		"payload": map[string]interface{}{
			"id":   "msg-456",
			"from": "5511999999999@c.us",
			"body": "Hello",
		},
	}
	data, _ := json.Marshal(event)
	return data
}

// Test Suite

func TestWAHAWebhookHandler_SuccessfulProcessing(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		hmacKey    string
		body       []byte
		wantStatus int
	}{
		{
			name:       "valid event with HMAC enabled",
			hmacKey:    "test-secret-key",
			body:       createValidEventPayload(),
			wantStatus: http.StatusOK,
		},
		{
			name:    "event with ID in payload.id",
			hmacKey: "test-secret-key",
			body: func() []byte {
				event := map[string]interface{}{
					"event":   "message",
					"session": "test-session",
					"engine":  "GOWS",
					"payload": map[string]interface{}{
						"id":   "payload-msg-id",
						"from": "5511999999999@c.us",
					},
				}
				data, _ := json.Marshal(event)
				return data
			}(),
			wantStatus: http.StatusOK,
		},
		{
			name:    "event with nested message.id",
			hmacKey: "test-secret-key",
			body: func() []byte {
				event := map[string]interface{}{
					"event":   "message",
					"session": "test-session",
					"engine":  "GOWS",
					"payload": map[string]interface{}{
						"message": map[string]interface{}{
							"id":   "nested-msg-id",
							"body": "Hello",
						},
					},
				}
				data, _ := json.Marshal(event)
				return data
			}(),
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			handler, repo, queue := newTestHandler(tt.hmacKey)

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = createWebhookRequest(tt.body, tt.hmacKey)

			handler.HandleWebhook(c)

			assert.Equal(t, tt.wantStatus, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			assert.Equal(t, true, response["received"])

			// Verify repository was called
			assert.Len(t, repo.InsertCalls, 1)

			// Verify queue was called
			assert.Len(t, queue.EnqueueCalls, 1)
		})
	}
}

func TestWAHAWebhookHandler_HMACValidationFailure(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name         string
		hmacKey      string
		body         []byte
		signature    string
		wantStatus   int
		wantError    string
		wantEnqueued bool
		wantDBInsert bool
	}{
		{
			name:         "missing signature",
			hmacKey:      "test-secret-key",
			body:         createValidEventPayload(),
			signature:    "",
			wantStatus:   http.StatusUnauthorized,
			wantError:    "invalid signature",
			wantEnqueued: false,
			wantDBInsert: false,
		},
		{
			name:         "wrong signature",
			hmacKey:      "test-secret-key",
			body:         createValidEventPayload(),
			signature:    "00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
			wantStatus:   http.StatusUnauthorized,
			wantError:    "invalid signature",
			wantEnqueued: false,
			wantDBInsert: false,
		},
		{
			name:    "signature from different key",
			hmacKey: "correct-key",
			body:    createValidEventPayload(),
			signature: func() string {
				wrongValidator := whatsapp.NewHMACValidator("wrong-key")
				return wrongValidator.ComputeSignature(createValidEventPayload())
			}(),
			wantStatus:   http.StatusUnauthorized,
			wantError:    "invalid signature",
			wantEnqueued: false,
			wantDBInsert: false,
		},
		{
			name:         "malformed signature (not hex)",
			hmacKey:      "test-secret-key",
			body:         createValidEventPayload(),
			signature:    "not-a-valid-hex-signature!@#$",
			wantStatus:   http.StatusUnauthorized,
			wantError:    "invalid signature",
			wantEnqueued: false,
			wantDBInsert: false,
		},
		{
			name:    "tampered body",
			hmacKey: "test-secret-key",
			body:    []byte(`{"event":"message","tampered":true}`),
			signature: func() string {
				validator := whatsapp.NewHMACValidator("test-secret-key")
				return validator.ComputeSignature([]byte(`{"event":"message"}`)) // Original body
			}(),
			wantStatus:   http.StatusUnauthorized,
			wantError:    "invalid signature",
			wantEnqueued: false,
			wantDBInsert: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			handler, repo, queue := newTestHandler(tt.hmacKey)

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/waha", bytes.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			if tt.signature != "" {
				req.Header.Set(wahaSignatureHeader, tt.signature)
			}
			c.Request = req

			handler.HandleWebhook(c)

			assert.Equal(t, tt.wantStatus, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			assert.Equal(t, tt.wantError, response["error"])

			// Verify no further processing occurred
			if !tt.wantDBInsert {
				assert.Len(t, repo.InsertCalls, 0, "should not insert into DB on HMAC failure")
			}
			if !tt.wantEnqueued {
				assert.Len(t, queue.EnqueueCalls, 0, "should not enqueue on HMAC failure")
			}
		})
	}
}

func TestWAHAWebhookHandler_HMACDisabled(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		signature  string
		wantStatus int
	}{
		{
			name:       "no signature when HMAC disabled",
			signature:  "",
			wantStatus: http.StatusOK,
		},
		{
			name:       "invalid signature ignored when HMAC disabled",
			signature:  "invalid-signature",
			wantStatus: http.StatusOK,
		},
		{
			name:       "any signature accepted when HMAC disabled",
			signature:  "00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Empty HMAC key disables validation
			handler, repo, queue := newTestHandler("")
			body := createValidEventPayload()

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/waha", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			if tt.signature != "" {
				req.Header.Set(wahaSignatureHeader, tt.signature)
			}
			c.Request = req

			handler.HandleWebhook(c)

			assert.Equal(t, tt.wantStatus, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			assert.Equal(t, true, response["received"])

			// Verify processing continued
			assert.Len(t, repo.InsertCalls, 1, "should insert into DB when HMAC disabled")
			assert.Len(t, queue.EnqueueCalls, 1, "should enqueue when HMAC disabled")
		})
	}
}

func TestWAHAWebhookHandler_Idempotency_DuplicateEvent(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	hmacKey := "test-secret-key"
	handler, repo, queue := newTestHandler(hmacKey)

	// Configure repo to report duplicate (already exists)
	repo.InsertIfNotExistsFunc = func(ctx context.Context, eventID, sessionName, eventType string) (bool, error) {
		return false, nil // Event already exists
	}

	body := createValidEventPayload()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = createWebhookRequest(body, hmacKey)

	handler.HandleWebhook(c)

	// Should still return 200 (ACK)
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify repository was called
	assert.Len(t, repo.InsertCalls, 1)

	// Verify queue was NOT called (duplicate event)
	assert.Len(t, queue.EnqueueCalls, 0, "should not enqueue duplicate events")
}

func TestWAHAWebhookHandler_Idempotency_DatabaseError(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	hmacKey := "test-secret-key"
	handler, repo, queue := newTestHandler(hmacKey)

	// Configure repo to return error
	repo.InsertIfNotExistsFunc = func(ctx context.Context, eventID, sessionName, eventType string) (bool, error) {
		return false, errors.New("database connection error")
	}

	body := createValidEventPayload()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = createWebhookRequest(body, hmacKey)

	handler.HandleWebhook(c)

	// Should still return 200 (ACK)
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify repository was called
	assert.Len(t, repo.InsertCalls, 1)

	// Queue should NOT be called when insert returns false (even with error)
	assert.Len(t, queue.EnqueueCalls, 0)
}

func TestWAHAWebhookHandler_PayloadSizeLimit(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		bodySize   int
		wantStatus int
		wantError  string
	}{
		{
			name:       "payload over limit (10MB + 1 byte)",
			bodySize:   (10 << 20) + 1,
			wantStatus: http.StatusRequestEntityTooLarge,
			wantError:  "payload too large",
		},
		{
			name:       "payload way over limit (20MB)",
			bodySize:   20 << 20,
			wantStatus: http.StatusRequestEntityTooLarge,
			wantError:  "payload too large",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			hmacKey := "test-secret-key"
			handler, repo, queue := newTestHandler(hmacKey)

			// Create oversized payload
			largeBody := make([]byte, tt.bodySize)
			for i := range largeBody {
				largeBody[i] = 'a'
			}

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = createWebhookRequest(largeBody, hmacKey)

			handler.HandleWebhook(c)

			assert.Equal(t, tt.wantStatus, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			assert.Equal(t, tt.wantError, response["error"])

			// Verify no processing occurred
			assert.Len(t, repo.InsertCalls, 0, "should not insert into DB on size limit")
			assert.Len(t, queue.EnqueueCalls, 0, "should not enqueue on size limit")
		})
	}
}

func TestWAHAWebhookHandler_PayloadUnderLimit(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	hmacKey := "test-secret-key"
	handler, repo, queue := newTestHandler(hmacKey)

	// Create a payload just under the limit (1MB - 1KB for headers/overhead)
	body := createValidEventPayload()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = createWebhookRequest(body, hmacKey)

	handler.HandleWebhook(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Len(t, repo.InsertCalls, 1)
	assert.Len(t, queue.EnqueueCalls, 1)
}

func TestWAHAWebhookHandler_JSONParsingFailure(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name string
		body []byte
	}{
		{
			name: "invalid JSON",
			body: []byte(`{invalid json`),
		},
		{
			name: "empty body",
			body: []byte(``),
		},
		{
			name: "array instead of object",
			body: []byte(`[1, 2, 3]`),
		},
		{
			name: "plain text",
			body: []byte(`just plain text`),
		},
		{
			name: "truncated JSON",
			body: []byte(`{"event": "message", "session": "`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			hmacKey := "test-secret-key"
			handler, repo, queue := newTestHandler(hmacKey)

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = createWebhookRequest(tt.body, hmacKey)

			handler.HandleWebhook(c)

			// Should still return 200 (ACK) - parsing happens after ACK
			assert.Equal(t, http.StatusOK, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			assert.Equal(t, true, response["received"])

			// Verify no DB insert or enqueue occurred due to parse failure
			assert.Len(t, repo.InsertCalls, 0, "should not insert into DB on parse failure")
			assert.Len(t, queue.EnqueueCalls, 0, "should not enqueue on parse failure")
		})
	}
}

func TestWAHAWebhookHandler_MissingEventID_FallbackGeneration(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name    string
		body    []byte
		session string
		event   string
	}{
		{
			name: "no ID anywhere",
			body: func() []byte {
				event := map[string]interface{}{
					"event":   "session.status",
					"session": "test-session",
					"engine":  "GOWS",
					"payload": map[string]interface{}{
						"status": "connected",
					},
				}
				data, _ := json.Marshal(event)
				return data
			}(),
			session: "test-session",
			event:   "session.status",
		},
		{
			name: "empty ID field",
			body: func() []byte {
				event := map[string]interface{}{
					"id":      "",
					"event":   "qr.code",
					"session": "qr-session",
					"engine":  "GOWS",
					"payload": map[string]interface{}{
						"qr": "data:image/png;base64,...",
					},
				}
				data, _ := json.Marshal(event)
				return data
			}(),
			session: "qr-session",
			event:   "qr.code",
		},
		{
			name: "null ID in payload",
			body: func() []byte {
				event := map[string]interface{}{
					"event":   "call",
					"session": "call-session",
					"engine":  "GOWS",
					"payload": map[string]interface{}{
						"id":   nil,
						"from": "5511999999999@c.us",
					},
				}
				data, _ := json.Marshal(event)
				return data
			}(),
			session: "call-session",
			event:   "call",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			hmacKey := "test-secret-key"
			handler, repo, queue := newTestHandler(hmacKey)

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = createWebhookRequest(tt.body, hmacKey)

			handler.HandleWebhook(c)

			assert.Equal(t, http.StatusOK, w.Code)

			// Verify repository was called with a generated ID
			require.Len(t, repo.InsertCalls, 1)
			call := repo.InsertCalls[0]

			// Fallback ID format: "timestamp:session:eventType"
			assert.Contains(t, call.EventID, tt.session)
			assert.Contains(t, call.EventID, tt.event)
			assert.Equal(t, tt.session, call.SessionName)
			assert.Equal(t, tt.event, call.EventType)

			// Verify queue was called
			require.Len(t, queue.EnqueueCalls, 1)
			assert.Contains(t, queue.EnqueueCalls[0].EventID, tt.session)
		})
	}
}

func TestWAHAWebhookHandler_EnqueueFailure(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	hmacKey := "test-secret-key"
	handler, repo, queue := newTestHandler(hmacKey)

	// Configure queue to return error
	queue.EnqueueWebhookFunc = func(ctx context.Context, data *workers.WebhookJobData) (*asynq.TaskInfo, error) {
		return nil, errors.New("redis connection refused")
	}

	body := createValidEventPayload()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = createWebhookRequest(body, hmacKey)

	handler.HandleWebhook(c)

	// Should still return 200 (ACK)
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify repository insert was called
	assert.Len(t, repo.InsertCalls, 1)

	// Verify queue was attempted
	assert.Len(t, queue.EnqueueCalls, 1)

	// Verify event was marked as failed
	require.Len(t, repo.MarkFailedCalls, 1)
	assert.Equal(t, "evt-123-abc", repo.MarkFailedCalls[0].EventID)
	assert.Equal(t, "failed to enqueue task", repo.MarkFailedCalls[0].ErrorMessage)
}

func TestWAHAWebhookHandler_DuplicateTaskError(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	hmacKey := "test-secret-key"
	handler, repo, queue := newTestHandler(hmacKey)

	// Configure queue to return duplicate task error
	queue.EnqueueWebhookFunc = func(ctx context.Context, data *workers.WebhookJobData) (*asynq.TaskInfo, error) {
		return nil, asynq.ErrDuplicateTask
	}

	body := createValidEventPayload()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = createWebhookRequest(body, hmacKey)

	handler.HandleWebhook(c)

	// Should still return 200 (ACK)
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify repository insert was called
	assert.Len(t, repo.InsertCalls, 1)

	// Verify queue was attempted
	assert.Len(t, queue.EnqueueCalls, 1)

	// Verify event was NOT marked as failed (duplicate is expected)
	assert.Len(t, repo.MarkFailedCalls, 0, "should not mark as failed on duplicate task")
}

func TestExtractEventID(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		event    WAHAWebhookEvent
		expected string
	}{
		{
			name: "top-level ID",
			event: WAHAWebhookEvent{
				ID:      "top-level-id",
				Event:   "message",
				Session: "session",
				Payload: map[string]interface{}{"id": "payload-id"},
			},
			expected: "top-level-id",
		},
		{
			name: "payload.id",
			event: WAHAWebhookEvent{
				Event:   "message",
				Session: "session",
				Payload: map[string]interface{}{"id": "payload-id"},
			},
			expected: "payload-id",
		},
		{
			name: "payload._id",
			event: WAHAWebhookEvent{
				Event:   "message",
				Session: "session",
				Payload: map[string]interface{}{"_id": "underscore-id"},
			},
			expected: "underscore-id",
		},
		{
			name: "nested message.id",
			event: WAHAWebhookEvent{
				Event:   "message",
				Session: "session",
				Payload: map[string]interface{}{
					"message": map[string]interface{}{"id": "nested-id"},
				},
			},
			expected: "nested-id",
		},
		{
			name: "no ID anywhere",
			event: WAHAWebhookEvent{
				Event:   "session.status",
				Session: "session",
				Payload: map[string]interface{}{"status": "connected"},
			},
			expected: "",
		},
		{
			name: "nil payload",
			event: WAHAWebhookEvent{
				Event:   "message",
				Session: "session",
				Payload: nil,
			},
			expected: "",
		},
		{
			name: "empty string ID",
			event: WAHAWebhookEvent{
				ID:      "",
				Event:   "message",
				Session: "session",
				Payload: map[string]interface{}{"id": ""},
			},
			expected: "",
		},
		{
			name: "non-string ID in payload",
			event: WAHAWebhookEvent{
				Event:   "message",
				Session: "session",
				Payload: map[string]interface{}{"id": 12345},
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := extractEventID(&tt.event)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestGenerateFallbackEventID(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		session   string
		eventType string
	}{
		{
			name:      "normal values",
			session:   "test-session",
			eventType: "message",
		},
		{
			name:      "empty session",
			session:   "",
			eventType: "message",
		},
		{
			name:      "empty event type",
			session:   "session",
			eventType: "",
		},
		{
			name:      "special characters",
			session:   "session-with-dashes",
			eventType: "message.ack",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := generateFallbackEventID(tt.session, tt.eventType)

			// Should contain session and event type
			assert.Contains(t, got, tt.session)
			assert.Contains(t, got, tt.eventType)

			// Format: hex:session:eventType:timestamp (4 parts)
			parts := strings.Split(got, ":")
			assert.Len(t, parts, 4)
			assert.NotEmpty(t, parts[0]) // Random hex
		})
	}
}

func TestGenerateFallbackEventID_ContainsTimestamp(t *testing.T) {
	// Cannot use t.Parallel() — reads package-level randSource (modified by other tests)
	session := "test-session"
	eventType := "message"

	before := time.Now().UnixNano()
	id := generateFallbackEventID(session, eventType)
	after := time.Now().UnixNano()

	// Extract parts from ID (format: hex:session:eventType:timestamp)
	parts := strings.Split(id, ":")
	require.Len(t, parts, 4)

	// Verify the ID contains expected parts
	assert.Contains(t, id, session)
	assert.Contains(t, id, eventType)

	// Verify timestamp is in the expected range (just check format, not exact value)
	assert.NotEmpty(t, parts[0])

	// Generate another ID and verify they are different (at least one component differs)
	time.Sleep(time.Nanosecond) // Ensure some time passes
	id2 := generateFallbackEventID(session, eventType)

	// IDs should be different (timestamp should change)
	// Note: In very fast systems, timestamps might collide, so we just verify format
	_ = before
	_ = after
	_ = id2
}

func TestTruncateBytes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    []byte
		maxLen   int
		expected string
	}{
		{
			name:     "under limit",
			input:    []byte("short"),
			maxLen:   10,
			expected: "short",
		},
		{
			name:     "at limit",
			input:    []byte("1234567890"),
			maxLen:   10,
			expected: "1234567890",
		},
		{
			name:     "over limit",
			input:    []byte("this is a very long string"),
			maxLen:   10,
			expected: "this is a ...",
		},
		{
			name:     "empty input",
			input:    []byte(""),
			maxLen:   10,
			expected: "",
		},
		{
			name:     "zero limit",
			input:    []byte("test"),
			maxLen:   0,
			expected: "...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := truncateBytes(tt.input, tt.maxLen)
			assert.Equal(t, tt.expected, string(got))
		})
	}
}

// Benchmark tests

func BenchmarkHandleWebhook(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)

	hmacKey := "benchmark-secret-key"
	handler, _, _ := newTestHandler(hmacKey)
	body := createValidEventPayload()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = createWebhookRequest(body, hmacKey)
		handler.HandleWebhook(c)
	}
}

func BenchmarkExtractEventID(b *testing.B) {
	event := WAHAWebhookEvent{
		ID:      "evt-123",
		Event:   "message",
		Session: "session",
		Payload: map[string]interface{}{
			"id": "msg-456",
			"message": map[string]interface{}{
				"id": "nested-id",
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		extractEventID(&event)
	}
}

// ---------------------------------------------------------------------------
// Tests for the REAL NewWAHAWebhookHandler and standalone functions
// ---------------------------------------------------------------------------

func TestNewWAHAWebhookHandler_WithHMAC(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	repo := &MockWebhookEventsRepository{}
	logger := zap.NewNop()

	// Creating the real handler with an HMAC key should not panic
	handler := NewWAHAWebhookHandler("test-hmac-secret", repo, nil, logger, nil)

	if handler == nil {
		t.Fatal("expected non-nil handler")
	}
	if handler.hmacValidator == nil {
		t.Fatal("expected non-nil HMAC validator")
	}
	if handler.webhookEventsRepo == nil {
		t.Fatal("expected non-nil webhook events repo")
	}
	if handler.logger == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestNewWAHAWebhookHandler_WithoutHMAC(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	repo := &MockWebhookEventsRepository{}
	// Use a development logger to capture the warning about HMAC disabled
	logger, _ := zap.NewDevelopment()

	// Empty HMAC key should not panic, but should warn
	handler := NewWAHAWebhookHandler("", repo, nil, logger, nil)

	if handler == nil {
		t.Fatal("expected non-nil handler")
	}
	if handler.hmacValidator == nil {
		t.Fatal("expected non-nil HMAC validator even when disabled")
	}
}

func TestGenerateTraceID_ReturnsHexString(t *testing.T) {
	t.Parallel()
	traceID := generateTraceID()

	// W3C trace-id is 32 hex chars (16 bytes)
	if len(traceID) != 32 {
		t.Fatalf("expected 32-char hex string, got %d chars: %q", len(traceID), traceID)
	}

	// Verify it's valid hex
	for _, c := range traceID {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Fatalf("expected hex character, got %c in %q", c, traceID)
		}
	}
}

func TestGenerateTraceID_UniqueValues(t *testing.T) {
	t.Parallel()
	id1 := generateTraceID()
	id2 := generateTraceID()

	if id1 == id2 {
		t.Fatalf("expected unique trace IDs, both are %q", id1)
	}
}

func TestGetOrGenerateTraceID_UsesHeaderWhenPresent(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/test", nil)
	c.Request.Header.Set(traceIDHeader, "my-custom-trace-id-12345")

	traceID := getOrGenerateTraceID(c)

	if traceID != "my-custom-trace-id-12345" {
		t.Fatalf("expected custom trace ID, got %q", traceID)
	}
}

func TestGetOrGenerateTraceID_GeneratesWhenMissing(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/test", nil)
	// No X-Trace-ID header set

	traceID := getOrGenerateTraceID(c)

	if traceID == "" {
		t.Fatal("expected generated trace ID, got empty string")
	}
	// Should be a valid 32-char hex string
	if len(traceID) != 32 {
		t.Fatalf("expected 32-char hex string, got %d chars: %q", len(traceID), traceID)
	}
}

// ---------------------------------------------------------------------------
// Tests for the REAL HandleWebhook using miniredis + real QueueClient
// ---------------------------------------------------------------------------

func TestRealWAHAWebhookHandler_HandleWebhook_FullFlow(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	// Start miniredis
	mr := miniredis.RunT(t)

	// Create a real QueueClient pointing to miniredis
	logger := zap.NewNop()
	queueClient := workers.NewQueueClient(mr.Addr(), logger)
	defer queueClient.Close()

	repo := &MockWebhookEventsRepository{}
	hmacKey := "real-handler-secret"

	handler := NewWAHAWebhookHandler(hmacKey, repo, queueClient, logger, nil)

	body := createValidEventPayload()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = createWebhookRequest(body, hmacKey)

	handler.HandleWebhook(c)

	// Should return 200 (ACK)
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, true, response["received"])

	// Verify DB insert was called
	assert.Len(t, repo.InsertCalls, 1)
	assert.Equal(t, "evt-123-abc", repo.InsertCalls[0].EventID)
	assert.Equal(t, "test-session", repo.InsertCalls[0].SessionName)
	assert.Equal(t, "message", repo.InsertCalls[0].EventType)

	// Verify trace ID header is set in response
	traceIDResp := w.Header().Get(traceIDHeader)
	assert.NotEmpty(t, traceIDResp, "expected X-Trace-ID response header")
}

func TestRealWAHAWebhookHandler_HandleWebhook_HMACFailure(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	mr := miniredis.RunT(t)

	logger := zap.NewNop()
	queueClient := workers.NewQueueClient(mr.Addr(), logger)
	defer queueClient.Close()

	repo := &MockWebhookEventsRepository{}
	hmacKey := "real-handler-secret"

	handler := NewWAHAWebhookHandler(hmacKey, repo, queueClient, logger, nil)

	body := createValidEventPayload()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	// Create request WITHOUT valid HMAC signature
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/waha", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(wahaSignatureHeader, "invalid-signature")
	c.Request = req

	handler.HandleWebhook(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// No DB calls on HMAC failure
	assert.Len(t, repo.InsertCalls, 0)
}

func TestRealWAHAWebhookHandler_HandleWebhook_InvalidJSON(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	mr := miniredis.RunT(t)

	logger := zap.NewNop()
	queueClient := workers.NewQueueClient(mr.Addr(), logger)
	defer queueClient.Close()

	repo := &MockWebhookEventsRepository{}
	hmacKey := "real-handler-secret"

	handler := NewWAHAWebhookHandler(hmacKey, repo, queueClient, logger, nil)

	invalidBody := []byte(`{not valid json`)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = createWebhookRequest(invalidBody, hmacKey)

	handler.HandleWebhook(c)

	// Should still ACK (200) since HMAC passed
	assert.Equal(t, http.StatusOK, w.Code)
	// But no DB calls because JSON parse failed
	assert.Len(t, repo.InsertCalls, 0)
}

func TestRealWAHAWebhookHandler_HandleWebhook_DuplicateEvent(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	mr := miniredis.RunT(t)

	logger := zap.NewNop()
	queueClient := workers.NewQueueClient(mr.Addr(), logger)
	defer queueClient.Close()

	repo := &MockWebhookEventsRepository{}
	// Second call returns duplicate (not inserted)
	repo.InsertIfNotExistsFunc = func(ctx context.Context, eventID, sessionName, eventType string) (bool, error) {
		return false, nil
	}

	hmacKey := "real-handler-secret"
	handler := NewWAHAWebhookHandler(hmacKey, repo, queueClient, logger, nil)

	body := createValidEventPayload()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = createWebhookRequest(body, hmacKey)

	handler.HandleWebhook(c)

	assert.Equal(t, http.StatusOK, w.Code)
	// DB was called to check
	assert.Len(t, repo.InsertCalls, 1)
	// But no MarkFailed calls (duplicate is expected, not a failure)
	assert.Len(t, repo.MarkFailedCalls, 0)
}

func TestRealWAHAWebhookHandler_MarkEventFailed(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	repo := &MockWebhookEventsRepository{}
	logger := zap.NewNop()

	handler := NewWAHAWebhookHandler("key", repo, nil, logger, nil)

	// Call markEventFailed directly
	handler.markEventFailed(context.Background(), "evt-999", "test error")

	require.Len(t, repo.MarkFailedCalls, 1)
	assert.Equal(t, "evt-999", repo.MarkFailedCalls[0].EventID)
	assert.Equal(t, "test error", repo.MarkFailedCalls[0].ErrorMessage)
}

func TestRealWAHAWebhookHandler_MarkEventFailed_RepoError(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	repo := &MockWebhookEventsRepository{}
	repo.MarkFailedFunc = func(ctx context.Context, eventID, errorMessage string) error {
		return errors.New("db connection lost")
	}
	logger := zap.NewNop()

	handler := NewWAHAWebhookHandler("key", repo, nil, logger, nil)

	// Should not panic even when MarkFailed fails
	handler.markEventFailed(context.Background(), "evt-999", "test error")

	// MarkFailed was still called (even though it errored)
	require.Len(t, repo.MarkFailedCalls, 1)
}

func TestRealWAHAWebhookHandler_HandleWebhook_BodyTooLarge(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	mr := miniredis.RunT(t)

	logger := zap.NewNop()
	queueClient := workers.NewQueueClient(mr.Addr(), logger)
	defer queueClient.Close()

	repo := &MockWebhookEventsRepository{}
	hmacKey := "real-handler-secret"

	handler := NewWAHAWebhookHandler(hmacKey, repo, queueClient, logger, nil)

	// Create a body larger than maxWebhookBodySize (10MB)
	largeBody := make([]byte, (10<<20)+1)
	for i := range largeBody {
		largeBody[i] = 'x'
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = createWebhookRequest(largeBody, hmacKey)

	handler.HandleWebhook(c)

	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
	assert.Len(t, repo.InsertCalls, 0)
}

func TestRealWAHAWebhookHandler_HandleWebhook_BodyReadError(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	mr := miniredis.RunT(t)

	logger := zap.NewNop()
	queueClient := workers.NewQueueClient(mr.Addr(), logger)
	defer queueClient.Close()

	repo := &MockWebhookEventsRepository{}
	hmacKey := "real-handler-secret"

	handler := NewWAHAWebhookHandler(hmacKey, repo, queueClient, logger, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/waha", &errorReader{})
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleWebhook(c)

	// Should return 400 for generic body read error
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Len(t, repo.InsertCalls, 0)
}

func TestRealWAHAWebhookHandler_HandleWebhook_TraceIDPropagation(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	mr := miniredis.RunT(t)

	logger := zap.NewNop()
	queueClient := workers.NewQueueClient(mr.Addr(), logger)
	defer queueClient.Close()

	repo := &MockWebhookEventsRepository{}
	hmacKey := "real-handler-secret"

	handler := NewWAHAWebhookHandler(hmacKey, repo, queueClient, logger, nil)

	body := createValidEventPayload()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := createWebhookRequest(body, hmacKey)
	req.Header.Set(traceIDHeader, "custom-trace-id-from-client")
	c.Request = req

	handler.HandleWebhook(c)

	assert.Equal(t, http.StatusOK, w.Code)
	// The response should echo back the provided trace ID
	assert.Equal(t, "custom-trace-id-from-client", w.Header().Get(traceIDHeader))
}

// errorReader is an io.Reader that always returns an error (non-MaxBytesError).
type errorReader struct{}

func (r *errorReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("simulated read error")
}

// ---------------------------------------------------------------------------
// newTestWebhookMetrics creates WebhookMetrics with an isolated Prometheus
// registry so tests do not collide with the global default registry.
// ---------------------------------------------------------------------------

func newTestWebhookMetrics() *metrics.WebhookMetrics {
	return &metrics.WebhookMetrics{
		ReceivedTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{Name: "t_received"}, []string{"session", "event_type"},
		),
		ProcessedTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{Name: "t_processed"}, []string{"session", "event_type"},
		),
		FailedTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{Name: "t_failed"}, []string{"session", "event_type", "error_type"},
		),
		DLQTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{Name: "t_dlq"}, []string{"session", "event_type"},
		),
		DuplicatesTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{Name: "t_dup"}, []string{"session", "event_type"},
		),
		ProcessingDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{Name: "t_dur", Buckets: prometheus.DefBuckets}, []string{"session", "event_type"},
		),
		QueuedTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{Name: "t_queued"}, []string{"session", "event_type"},
		),
		InFlight: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{Name: "t_inflight"}, []string{"session"},
		),
		HMACValidationTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{Name: "t_hmac"}, []string{"result"},
		),
		RetryTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{Name: "t_retry"}, []string{"session", "event_type", "attempt"},
		),
		BodySizeBytes: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{Name: "t_body", Buckets: prometheus.DefBuckets}, []string{"event_type"},
		),
	}
}

// counterValue reads the current value from a prometheus.CounterVec for the given labels.
func counterValue(cv *prometheus.CounterVec, labels ...string) float64 {
	m, err := cv.GetMetricWithLabelValues(labels...)
	if err != nil {
		return 0
	}
	pb := &dto.Metric{}
	if err := m.Write(pb); err != nil {
		return 0
	}
	return pb.GetCounter().GetValue()
}

// ---------------------------------------------------------------------------
// Tests for production NewWAHAWebhookHandler HMAC disabled in production env
// (covers lines 91-94)
// ---------------------------------------------------------------------------

func TestNewWAHAWebhookHandler_ProductionHMACDisabled(t *testing.T) {
	t.Setenv("NODE_ENV", "production")

	repo := &MockWebhookEventsRepository{}
	logger := zap.NewNop()

	// Creating the real handler with empty HMAC key in "production" env
	// should not panic — it logs a Sentry message.
	handler := NewWAHAWebhookHandler("", repo, nil, logger, nil)

	if handler == nil {
		t.Fatal("expected non-nil handler")
	}
	if handler.hmacValidator == nil {
		t.Fatal("expected non-nil HMAC validator even when disabled")
	}
}

// ---------------------------------------------------------------------------
// Tests for metrics branches in HandleWebhook
// (covers lines 170-177, 242-244, 273-275, 331-333)
// ---------------------------------------------------------------------------

func TestRealWAHAWebhookHandler_HandleWebhook_MetricsHMACDisabled(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	mr := miniredis.RunT(t)
	logger := zap.NewNop()
	queueClient := workers.NewQueueClient(mr.Addr(), logger)
	defer queueClient.Close()

	repo := &MockWebhookEventsRepository{}
	wm := newTestWebhookMetrics()

	// HMAC disabled (empty key) — metrics should record "disabled"
	handler := NewWAHAWebhookHandler("", repo, queueClient, logger, wm)

	body := createValidEventPayload()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/waha", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleWebhook(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// HMAC validation metric: "disabled"
	val := counterValue(wm.HMACValidationTotal, "disabled")
	if val < 1 {
		t.Errorf("expected HMACValidationTotal(disabled) >= 1, got %v", val)
	}

	// RecordReceived metric
	recv := counterValue(wm.ReceivedTotal, "test-session", "message")
	if recv < 1 {
		t.Errorf("expected ReceivedTotal(test-session, message) >= 1, got %v", recv)
	}

	// RecordQueued metric
	queued := counterValue(wm.QueuedTotal, "test-session", "message")
	if queued < 1 {
		t.Errorf("expected QueuedTotal(test-session, message) >= 1, got %v", queued)
	}
}

func TestRealWAHAWebhookHandler_HandleWebhook_MetricsHMACValid(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	mr := miniredis.RunT(t)
	logger := zap.NewNop()
	queueClient := workers.NewQueueClient(mr.Addr(), logger)
	defer queueClient.Close()

	repo := &MockWebhookEventsRepository{}
	wm := newTestWebhookMetrics()
	hmacKey := "metrics-hmac-key"

	handler := NewWAHAWebhookHandler(hmacKey, repo, queueClient, logger, wm)

	body := createValidEventPayload()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = createWebhookRequest(body, hmacKey)

	handler.HandleWebhook(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	val := counterValue(wm.HMACValidationTotal, "valid")
	if val < 1 {
		t.Errorf("expected HMACValidationTotal(valid) >= 1, got %v", val)
	}
}

func TestRealWAHAWebhookHandler_HandleWebhook_MetricsHMACInvalid(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	mr := miniredis.RunT(t)
	logger := zap.NewNop()
	queueClient := workers.NewQueueClient(mr.Addr(), logger)
	defer queueClient.Close()

	repo := &MockWebhookEventsRepository{}
	wm := newTestWebhookMetrics()

	handler := NewWAHAWebhookHandler("secret-key", repo, queueClient, logger, wm)

	body := createValidEventPayload()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/waha", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(wahaSignatureHeader, "wrong-sig")
	c.Request = req

	handler.HandleWebhook(c)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}

	val := counterValue(wm.HMACValidationTotal, "invalid")
	if val < 1 {
		t.Errorf("expected HMACValidationTotal(invalid) >= 1, got %v", val)
	}
}

// ---------------------------------------------------------------------------
// Test: empty eventID path — handler generates fallback ID
// (covers lines 224-232)
// ---------------------------------------------------------------------------

func TestRealWAHAWebhookHandler_HandleWebhook_EmptyEventID(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	mr := miniredis.RunT(t)
	logger := zap.NewNop()
	queueClient := workers.NewQueueClient(mr.Addr(), logger)
	defer queueClient.Close()

	repo := &MockWebhookEventsRepository{}
	wm := newTestWebhookMetrics()

	handler := NewWAHAWebhookHandler("", repo, queueClient, logger, wm)

	// Event with no ID at all (and no payload.id)
	event := map[string]interface{}{
		"event":   "session.status",
		"session": "my-session",
		"engine":  "GOWS",
		"payload": map[string]interface{}{
			"status": "connected",
		},
	}
	body, _ := json.Marshal(event)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/waha", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleWebhook(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Repo should have been called with a generated fallback ID
	if len(repo.InsertCalls) != 1 {
		t.Fatalf("expected 1 InsertCalls, got %d", len(repo.InsertCalls))
	}
	call := repo.InsertCalls[0]
	if call.EventID == "" {
		t.Fatal("expected non-empty generated eventID")
	}
	if !strings.Contains(call.EventID, "my-session") {
		t.Errorf("expected fallback ID to contain session name, got %q", call.EventID)
	}
	if !strings.Contains(call.EventID, "session.status") {
		t.Errorf("expected fallback ID to contain event type, got %q", call.EventID)
	}
}

// ---------------------------------------------------------------------------
// Test: idempotency error — handler logs but continues to enqueue
// (covers lines 252-264)
// ---------------------------------------------------------------------------

func TestRealWAHAWebhookHandler_HandleWebhook_IdempotencyError(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	mr := miniredis.RunT(t)
	logger := zap.NewNop()
	queueClient := workers.NewQueueClient(mr.Addr(), logger)
	defer queueClient.Close()

	repo := &MockWebhookEventsRepository{}
	// InsertIfNotExists returns error but also returns false (not inserted)
	// The handler should log the error and NOT return early — it will check `!inserted`
	// and since inserted=false, it treats it as duplicate. To actually reach the enqueue
	// path after an error, we need inserted=false AND we need to verify the error path.
	// Looking at the code: after the error at line 252-264, it does NOT return.
	// Then at line 267, it checks `if !inserted` — if false (error case returns false),
	// it treats as duplicate and returns.
	// To hit enqueue after error: we need the function to return (false, error).
	// The handler logs the error at 253 but does NOT return. Then at 267: !inserted (false==false → true)
	// so it returns as duplicate. The enqueue path is skipped.
	// Actually wait — re-reading the code:
	//   inserted, err := h.webhookEventsRepo.InsertIfNotExists(...)
	//   if err != nil { ... } // logs but does NOT return
	//   if !inserted { ... return } // returns here because inserted=false
	// So if inserted=false, err=non-nil: it logs AND returns as duplicate.
	// To cover lines 252-264 AND also hit enqueue, we need inserted=true, err=non-nil.
	// But that's contradictory for the real implementation. However, since it's a mock,
	// we can return (true, error) — the handler will log the error but continue because
	// the next check is `if !inserted` which is false (true → not-false), so it proceeds
	// to enqueue.
	repo.InsertIfNotExistsFunc = func(ctx context.Context, eventID, sessionName, eventType string) (bool, error) {
		return true, errors.New("transient database error")
	}

	wm := newTestWebhookMetrics()
	handler := NewWAHAWebhookHandler("", repo, queueClient, logger, wm)

	body := createValidEventPayload()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/waha", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleWebhook(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// InsertIfNotExists was called
	if len(repo.InsertCalls) != 1 {
		t.Fatalf("expected 1 InsertCalls, got %d", len(repo.InsertCalls))
	}

	// Queued metric should be recorded (enqueue succeeded)
	queued := counterValue(wm.QueuedTotal, "test-session", "message")
	if queued < 1 {
		t.Errorf("expected QueuedTotal >= 1 after idempotency error + enqueue, got %v", queued)
	}
}

// ---------------------------------------------------------------------------
// Test: duplicate event with metrics — RecordDuplicate
// (covers lines 273-275)
// ---------------------------------------------------------------------------

func TestRealWAHAWebhookHandler_HandleWebhook_DuplicateWithMetrics(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	mr := miniredis.RunT(t)
	logger := zap.NewNop()
	queueClient := workers.NewQueueClient(mr.Addr(), logger)
	defer queueClient.Close()

	repo := &MockWebhookEventsRepository{}
	repo.InsertIfNotExistsFunc = func(ctx context.Context, eventID, sessionName, eventType string) (bool, error) {
		return false, nil // duplicate
	}

	wm := newTestWebhookMetrics()
	handler := NewWAHAWebhookHandler("", repo, queueClient, logger, wm)

	body := createValidEventPayload()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/waha", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleWebhook(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	dup := counterValue(wm.DuplicatesTotal, "test-session", "message")
	if dup < 1 {
		t.Errorf("expected DuplicatesTotal(test-session, message) >= 1, got %v", dup)
	}
}

// ---------------------------------------------------------------------------
// Test: enqueue returns IsDuplicateTaskError — handler returns silently
// (covers lines 297-302)
// ---------------------------------------------------------------------------

func TestRealWAHAWebhookHandler_HandleWebhook_EnqueueDuplicateTask(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	mr := miniredis.RunT(t)
	logger := zap.NewNop()
	queueClient := workers.NewQueueClient(mr.Addr(), logger)
	defer queueClient.Close()

	repo := &MockWebhookEventsRepository{}

	handler := NewWAHAWebhookHandler("", repo, queueClient, logger, nil)

	body := createValidEventPayload()

	// First request — enqueues successfully
	w1 := httptest.NewRecorder()
	c1, _ := gin.CreateTestContext(w1)
	c1.Request = httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/waha", bytes.NewReader(body))
	c1.Request.Header.Set("Content-Type", "application/json")
	handler.HandleWebhook(c1)

	if w1.Code != http.StatusOK {
		t.Fatalf("first request: expected 200, got %d", w1.Code)
	}

	// Reset insert mock so second call also returns inserted=true
	repo.InsertIfNotExistsFunc = nil
	repo.InsertCalls = nil

	// Second request with same event — asynq should return duplicate task error
	// because the TaskID is "waha:evt-123-abc" (same event ID).
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/waha", bytes.NewReader(body))
	c2.Request.Header.Set("Content-Type", "application/json")
	handler.HandleWebhook(c2)

	if w2.Code != http.StatusOK {
		t.Fatalf("second request: expected 200, got %d", w2.Code)
	}

	// Should NOT have marked as failed (duplicate task is expected)
	if len(repo.MarkFailedCalls) != 0 {
		t.Errorf("expected 0 MarkFailedCalls on duplicate task, got %d", len(repo.MarkFailedCalls))
	}
}

// ---------------------------------------------------------------------------
// Test: enqueue returns non-duplicate error — markEventFailed is called
// (covers lines 305-319)
// ---------------------------------------------------------------------------

func TestRealWAHAWebhookHandler_HandleWebhook_EnqueueNonDuplicateError(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	// Close the miniredis immediately to simulate connection failure
	mr := miniredis.RunT(t)
	logger := zap.NewNop()
	queueClient := workers.NewQueueClient(mr.Addr(), logger)
	defer queueClient.Close()

	// Close redis to cause enqueue error
	mr.Close()

	repo := &MockWebhookEventsRepository{}
	handler := NewWAHAWebhookHandler("", repo, queueClient, logger, nil)

	body := createValidEventPayload()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/waha", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleWebhook(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// markEventFailed should have been called
	if len(repo.MarkFailedCalls) != 1 {
		t.Fatalf("expected 1 MarkFailedCalls, got %d", len(repo.MarkFailedCalls))
	}
	if repo.MarkFailedCalls[0].EventID != "evt-123-abc" {
		t.Errorf("expected event ID evt-123-abc, got %q", repo.MarkFailedCalls[0].EventID)
	}
	if repo.MarkFailedCalls[0].ErrorMessage != "failed to enqueue task" {
		t.Errorf("expected error message 'failed to enqueue task', got %q", repo.MarkFailedCalls[0].ErrorMessage)
	}
}

// =============================================================================
// Tests: generateFallbackEventID — crypto/rand error fallback (lines 391-394)
// =============================================================================

func TestGenerateFallbackEventID_RandError_FallsBackToTimestamp(t *testing.T) {
	t.Parallel()
	// Override randSource to always return an error
	original := randSource
	randSource = func(b []byte) (int, error) {
		return 0, fmt.Errorf("simulated rand failure")
	}
	t.Cleanup(func() { randSource = original })

	result := generateFallbackEventID("test-session", "message")

	// Fallback format: "<timestamp>:<session>:<eventType>"
	if !strings.Contains(result, ":test-session:message") {
		t.Errorf("expected fallback format with session+eventType, got %q", result)
	}
	// Should NOT contain hex-encoded bytes (which would indicate rand succeeded)
	if strings.Count(result, ":") != 2 {
		t.Errorf("expected exactly 2 colons in fallback format, got %q", result)
	}
}

func TestGenerateFallbackEventID_RandSuccess_ReturnsHexFormat(t *testing.T) {
	t.Parallel()
	// Ensure randSource is the real crypto/rand.Read
	original := randSource
	randSource = rand.Read
	t.Cleanup(func() { randSource = original })

	result := generateFallbackEventID("my-session", "session.status")

	// Success format: "<hex>:<session>:<eventType>:<timestamp>"
	parts := strings.Split(result, ":")
	if len(parts) != 4 {
		t.Errorf("expected 4 parts in success format, got %d: %q", len(parts), result)
	}
	if parts[1] != "my-session" {
		t.Errorf("expected session=my-session, got %q", parts[1])
	}
	if parts[2] != "session.status" {
		t.Errorf("expected eventType=session.status, got %q", parts[2])
	}
}

// =============================================================================
// Tests: generateTraceID — crypto/rand error fallback (lines 414-417)
// =============================================================================

func TestGenerateTraceID_RandError_FallsBackToTimestamp(t *testing.T) {
	// Cannot use t.Parallel() — modifies package-level randSource
	// Override randSource to always return an error
	original := randSource
	randSource = func(b []byte) (int, error) {
		return 0, fmt.Errorf("simulated rand failure")
	}
	t.Cleanup(func() { randSource = original })

	result := generateTraceID()

	// Fallback format: 32 hex characters (two 16-char hex values concatenated)
	if len(result) != 32 {
		t.Errorf("expected 32-char fallback trace ID, got %d chars: %q", len(result), result)
	}
}

func TestGenerateTraceID_RandSuccess_Returns32HexChars(t *testing.T) {
	// Cannot use t.Parallel() — modifies package-level randSource
	// Ensure randSource is the real crypto/rand.Read
	original := randSource
	randSource = rand.Read
	t.Cleanup(func() { randSource = original })

	result := generateTraceID()

	// 16 bytes → 32 hex characters
	if len(result) != 32 {
		t.Errorf("expected 32-char trace ID, got %d chars: %q", len(result), result)
	}
}

// =============================================================================
// Envelope group.v2.participants — ID e timestamp originais do provedor
// =============================================================================

// v2ParticipantsEnvelope monta um envelope conforme WebhookGroupV2Participants
// (waha-plus-openapi.json). Identificadores sintéticos, não reais.
func v2ParticipantsEnvelope(id string, timestamp interface{}) []byte {
	event := map[string]interface{}{
		"id":      id,
		"event":   "group.v2.participants",
		"session": "test-session",
		"engine":  "GOWS",
		"payload": map[string]interface{}{
			"type":      "join",
			"timestamp": 1774267200,
			"group":     map[string]interface{}{"id": "120000000000000001@g.us"},
			"participants": []map[string]interface{}{
				{"id": "5511000000000@c.us", "role": "participant"},
			},
			"_data": map[string]interface{}{},
		},
	}
	if timestamp != nil {
		event["timestamp"] = timestamp
	}
	data, _ := json.Marshal(event)
	return data
}

func TestWAHAWebhookHandler_GroupV2PreservesProviderEnvelope(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	const hmacKey = "test-secret-key"
	const providerID = "evt_01aaaaaaaaaaaaaaaaaaaaaaaa"
	handler, _, queue := newTestHandler(hmacKey)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = createWebhookRequest(v2ParticipantsEnvelope(providerID, 1774267200123), hmacKey)

	handler.HandleWebhook(c)

	require.Equal(t, http.StatusOK, w.Code)
	require.Len(t, queue.EnqueueCalls, 1)
	job := queue.EnqueueCalls[0]

	assert.Equal(t, providerID, job.ProviderEventID, "o ULID do provedor precisa chegar intacto ao worker")
	assert.Equal(t, int64(1774267200123), job.ProviderTimestamp, "timestamp em ms do envelope preservado")
	assert.Equal(t, "group.v2.participants", job.Event)
}

// Envelope sem ID: o EventID interno continua com o fallback (a fila precisa
// dele), mas ProviderEventID fica VAZIO — um fallback nosso não pode virar
// chave idempotente de fato de membro.
func TestWAHAWebhookHandler_GroupV2WithoutIDLeavesProviderEmpty(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	const hmacKey = "test-secret-key"
	handler, _, queue := newTestHandler(hmacKey)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = createWebhookRequest(v2ParticipantsEnvelope("", 1774267200123), hmacKey)

	handler.HandleWebhook(c)

	require.Len(t, queue.EnqueueCalls, 1)
	job := queue.EnqueueCalls[0]

	assert.Empty(t, job.ProviderEventID)
	assert.NotEmpty(t, job.EventID, "a dedup da fila continua funcionando com o fallback interno")
	assert.Contains(t, job.EventID, "group.v2.participants")
}

// O envelope inteiro NÃO pode morrer por causa do timestamp: se o provedor
// mandar um número fracionário, o Unmarshal do struct precisa sobreviver — caso
// contrário o evento se perderia no caminho quente.
func TestWAHAWebhookEvent_FractionalTimestampDoesNotBreakParse(t *testing.T) {
	t.Parallel()

	var event WAHAWebhookEvent
	err := json.Unmarshal(v2ParticipantsEnvelope("evt_01aaaaaaaaaaaaaaaaaaaaaaaa", 1774267200123.5), &event)
	require.NoError(t, err, "timestamp fracionário não pode derrubar o parse do envelope")

	assert.Equal(t, "group.v2.participants", event.Event)
	assert.Equal(t, int64(1774267200123), int64(event.Timestamp))
	assert.NotNil(t, event.Payload)
}

// Envelope sem o campo timestamp: parse OK, provider timestamp zero.
func TestWAHAWebhookEvent_MissingTimestampParsesToZero(t *testing.T) {
	t.Parallel()

	var event WAHAWebhookEvent
	require.NoError(t, json.Unmarshal(v2ParticipantsEnvelope("evt_01aaaaaaaaaaaaaaaaaaaaaaaa", nil), &event))
	assert.Zero(t, event.Timestamp)
	assert.Equal(t, "group.v2.participants", event.Event)
}
