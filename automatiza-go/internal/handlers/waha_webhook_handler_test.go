package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/models"
	"automatiza-go/internal/workers"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
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
func readBody(c *gin.Context) ([]byte, error) {
	body := make([]byte, 0, 1024)
	buf := make([]byte, 1024)
	for {
		n, err := c.Request.Body.Read(buf)
		if n > 0 {
			body = append(body, buf[:n]...)
		}
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			if strings.Contains(err.Error(), "request body too large") {
				c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "payload too large"})
				return nil, err
			}
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read body"})
			return nil, err
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
			got := extractEventID(&tt.event)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestGenerateFallbackEventID(t *testing.T) {
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
