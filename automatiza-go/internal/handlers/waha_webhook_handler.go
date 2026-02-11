// Package handlers provides HTTP handlers for the API.
// This file implements the WAHA webhook handler with HMAC validation,
// idempotency checking, and async processing via Asynq.
//
// Security considerations:
// - HMAC validation prevents webhook spoofing
// - Idempotency prevents duplicate processing
// - ACK immediately to avoid webhook retries
// - Process asynchronously to avoid blocking
package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/metrics"
	"automatiza-go/internal/observability"
	"automatiza-go/internal/repository"
	"automatiza-go/internal/workers"

	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// WAHA webhook signature header name.
// WAHA sends the HMAC-SHA256 signature in this header when HMAC is configured.
const wahaSignatureHeader = "X-WAHA-SIGNATURE"

// traceIDHeader is the header name for trace ID propagation.
// Clients can send their own trace ID in this header, otherwise one is generated.
const traceIDHeader = "X-Trace-ID"

// Maximum allowed webhook body size (10MB).
// This prevents memory exhaustion attacks from oversized payloads.
// 10MB is conservative for webhooks since media is sent via URL, not inline.
const maxWebhookBodySize = 10 << 20 // 10MB

// Task type for webhook processing in Asynq.
const TaskTypeWAHAWebhook = "whatsapp:waha_webhook"

// WAHAWebhookEvent represents a WAHA webhook event.
// The ID field is typically a ULID provided by WAHA for idempotency.
type WAHAWebhookEvent struct {
	ID      string                 `json:"id"`      // ULID for idempotency (may be in payload)
	Event   string                 `json:"event"`   // Event type (message, session.status, etc.)
	Session string                 `json:"session"` // Session name
	Engine  string                 `json:"engine"`  // Engine (GOWS, NOWEB, etc.)
	Payload map[string]interface{} `json:"payload"` // Event-specific data
}

// WAHAWebhookHandler handles incoming WAHA webhooks.
// It validates HMAC signatures, checks idempotency, and enqueues events for async processing.
type WAHAWebhookHandler struct {
	hmacValidator     *whatsapp.HMACValidator
	webhookEventsRepo repository.WebhookEventsRepository
	queueClient       *workers.QueueClient
	logger            *zap.Logger
	metrics           *metrics.WebhookMetrics
}

// NewWAHAWebhookHandler creates a new WAHA webhook handler.
//
// Parameters:
//   - hmacKey: The secret key for HMAC validation. If empty, validation is disabled (not recommended for production).
//   - webhookEventsRepo: Repository for idempotency checking and event tracking.
//   - queueClient: Queue client for enqueueing tasks.
//   - logger: Zap logger for structured logging.
//   - webhookMetrics: Optional metrics collector. If nil, metrics are not recorded.
func NewWAHAWebhookHandler(
	hmacKey string,
	webhookEventsRepo repository.WebhookEventsRepository,
	queueClient *workers.QueueClient,
	logger *zap.Logger,
	webhookMetrics *metrics.WebhookMetrics,
) *WAHAWebhookHandler {
	validator := whatsapp.NewHMACValidator(hmacKey)

	// Alert if HMAC validation is disabled in production
	if !validator.IsEnabled() {
		logger.Warn("WAHA webhook HMAC validation is disabled - this is not recommended for production")
		if os.Getenv("NODE_ENV") == "production" {
			observability.CaptureMessage(sentry.LevelError, "WAHA HMAC validation disabled in production",
				map[string]string{"security": "hmac-disabled", "module": "waha-webhook"})
		}
	}

	return &WAHAWebhookHandler{
		hmacValidator:     validator,
		webhookEventsRepo: webhookEventsRepo,
		queueClient:       queueClient,
		logger:            logger,
		metrics:           webhookMetrics,
	}
}

// HandleWebhook handles POST /api/v1/webhooks/waha
//
// Flow:
//  1. Read body
//  2. Validate HMAC signature (if enabled)
//  3. Send 200 OK immediately (ACK) - prevents WAHA retries
//  4. Parse JSON
//  5. Extract event ID (for idempotency)
//  6. Check idempotency (InsertIfNotExists)
//  7. If duplicate, log debug and return
//  8. Enqueue task to Asynq for async processing
//  9. If enqueue fails, mark as failed in DB
//
// The handler always returns 200 OK after HMAC validation to ACK receipt.
// This prevents WAHA from retrying the webhook while we process asynchronously.
func (h *WAHAWebhookHandler) HandleWebhook(c *gin.Context) {
	// Note: We don't use request context for post-ACK operations
	// because we respond 200 OK early and continue processing asynchronously

	// Generate or extract trace ID for distributed tracing
	traceID := getOrGenerateTraceID(c)

	// Add trace ID to response headers for debugging
	c.Header(traceIDHeader, traceID)

	// Create a logger with trace ID for all operations
	logger := h.logger.With(zap.String("trace_id", traceID))

	// Step 1: Read body with size limit to prevent memory exhaustion (DoS)
	// Apply limit BEFORE reading to prevent large payloads from consuming memory
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxWebhookBodySize)
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		// W13: Use errors.As instead of string comparison for MaxBytesError
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			logger.Warn("webhook body too large",
				zap.String("ip", c.ClientIP()),
				zap.Int64("limit", maxBytesErr.Limit))
			observability.CaptureWithTags(fmt.Errorf("webhook body too large: %w", err), map[string]string{
				"module":   "waha-webhook",
				"security": "body_too_large",
				"ip":       c.ClientIP(),
			})
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "payload too large"})
			return
		}
		logger.Error("failed to read webhook body", zap.Error(err))
		// Return 400 to signal WAHA should retry later
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read body"})
		return
	}

	// Step 2: Validate HMAC signature
	signature := c.GetHeader(wahaSignatureHeader)
	result := h.hmacValidator.ValidateWithResult(body, signature)

	logger.Debug("webhook HMAC validation",
		zap.Bool("valid", result.Valid),
		zap.Bool("enabled", result.Enabled),
		zap.String("reason", result.Reason),
	)

	// Record HMAC validation metrics
	if h.metrics != nil {
		if !result.Enabled {
			h.metrics.RecordHMACValidation("disabled")
		} else if result.Valid {
			h.metrics.RecordHMACValidation("valid")
		} else {
			h.metrics.RecordHMACValidation("invalid")
		}
	}

	if !result.Valid {
		logger.Warn("webhook HMAC validation failed",
			zap.String("reason", result.Reason),
			zap.String("ip", c.ClientIP()),
		)
		// B22: Capture HMAC failure with security tags
		observability.CaptureWithTags(
			fmt.Errorf("webhook HMAC validation failed: %s", result.Reason),
			map[string]string{
				"module":   "waha-webhook",
				"security": "hmac_failure",
				"ip":       c.ClientIP(),
			},
		)
		// Return 401 Unauthorized for invalid signatures
		// Do NOT return 200 - this would prevent retries for legitimate requests
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid signature"})
		return
	}

	// Step 3: ACK immediately (200 OK) - prevents WAHA retries
	// From this point on, we process asynchronously and always return success.
	// Any failures will be handled by the async worker with retries.
	c.JSON(http.StatusOK, gin.H{"received": true})

	// Step 4: Parse JSON
	var event WAHAWebhookEvent
	if err := json.Unmarshal(body, &event); err != nil {
		// M4 fix: Don't log body preview as it may contain PII (phone numbers, messages)
		logger.Warn("failed to parse webhook body",
			zap.Error(err),
			zap.Int("body_size", len(body)),
		)
		// B24: Capture JSON parse error with tags
		observability.CaptureWithTags(fmt.Errorf("webhook JSON parse failed: %w", err), map[string]string{
			"module":  "waha-webhook",
			"webhook": "parse_error",
		})
		// Already ACKed, just log and return
		return
	}

	// Step 5: Extract event ID for idempotency
	eventID := extractEventID(&event)
	if eventID == "" {
		logger.Warn("webhook event has no ID, generating one",
			zap.String("event_type", event.Event),
			zap.String("session", event.Session),
		)
		// Generate a fallback ID using timestamp + session + event type
		// This is not ideal but prevents completely losing the event
		eventID = generateFallbackEventID(event.Session, event.Event)
	}

	logger.Debug("processing webhook event",
		zap.String("event_id", eventID),
		zap.String("event_type", event.Event),
		zap.String("session", event.Session),
		zap.String("engine", event.Engine),
	)

	// Record webhook received metrics
	if h.metrics != nil {
		h.metrics.RecordReceived(event.Session, event.Event, len(body))
	}

	// Step 6: Check idempotency
	// Use a background context with timeout since we already ACKed the request
	dbCtx, dbCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer dbCancel()

	inserted, err := h.webhookEventsRepo.InsertIfNotExists(dbCtx, eventID, event.Session, event.Event)
	if err != nil {
		logger.Error("failed to check idempotency",
			zap.String("event_id", eventID),
			zap.Error(err),
		)
		// B25: Capture idempotency check failure
		observability.CaptureWithTags(fmt.Errorf("webhook idempotency check failed: %w", err), map[string]string{
			"module":   "waha-webhook",
			"webhook":  "idempotency_failure",
			"event_id": eventID,
		})
		// Don't return - try to enqueue anyway, the worker will handle deduplication
	}

	// Step 7: If duplicate, log and return
	if !inserted {
		logger.Debug("duplicate webhook event ignored",
			zap.String("event_id", eventID),
			zap.String("event_type", event.Event),
		)
		// Record duplicate metric
		if h.metrics != nil {
			h.metrics.RecordDuplicate(event.Session, event.Event)
		}
		return
	}

	// Step 8: Enqueue task to Asynq
	taskPayload := &workers.WebhookJobData{
		EventID:    eventID,
		Event:      event.Event,
		Session:    event.Session,
		Engine:     event.Engine,
		Payload:    event.Payload,
		ReceivedAt: time.Now().UnixMilli(),
		TraceID:    traceID, // Propagate trace ID to worker
	}

	// Use background context for enqueue since we already ACKed
	enqueueCtx, enqueueCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer enqueueCancel()

	info, err := h.queueClient.EnqueueWebhook(enqueueCtx, taskPayload)
	if err != nil {
		// Check if it's a duplicate task error (already enqueued)
		if workers.IsDuplicateTaskError(err) {
			logger.Debug("task already enqueued",
				zap.String("event_id", eventID),
			)
			return
		}

		// Log full error for debugging, but store sanitized message in DB (M3 fix)
		logger.Error("failed to enqueue webhook task",
			zap.String("event_id", eventID),
			zap.Error(err),
		)
		// B26: Enqueue failure is Critical — events will be lost
		observability.CaptureMessage(sentry.LevelFatal, "webhook enqueue failed — event may be lost", map[string]string{
			"module":   "waha-webhook",
			"webhook":  "enqueue_failure",
			"event_id": eventID,
		})

		// Step 9: Mark as failed in DB with sanitized error message
		// Don't expose internal error details in database
		h.markEventFailed(dbCtx, eventID, "failed to enqueue task")
		return
	}

	logger.Info("webhook task enqueued",
		zap.String("event_id", eventID),
		zap.String("event_type", event.Event),
		zap.String("session", event.Session),
		zap.String("task_id", info.ID),
		zap.String("queue", info.Queue),
	)

	// Record queued metric
	if h.metrics != nil {
		h.metrics.RecordQueued(event.Session, event.Event)
	}
}

// markEventFailed marks an event as failed in the database.
func (h *WAHAWebhookHandler) markEventFailed(ctx context.Context, eventID, errorMessage string) {
	// Use a background context with timeout since the request context may be canceled
	bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := h.webhookEventsRepo.MarkFailed(bgCtx, eventID, errorMessage); err != nil {
		h.logger.Error("failed to mark event as failed",
			zap.String("event_id", eventID),
			zap.String("original_error", errorMessage),
			zap.Error(err),
		)
		// B27: Capture mark-failed error with event_id context
		observability.CaptureWithScope(fmt.Errorf("failed to mark webhook event as failed: %w", err), func(scope *sentry.Scope) {
			scope.SetTag("module", "waha-webhook")
			scope.SetTag("webhook", "mark_failed_error")
			scope.SetExtra("event_id", eventID)
			scope.SetExtra("original_error", errorMessage)
		})
	}
}

// extractEventID extracts the event ID from a WAHA webhook event.
// WAHA typically includes the ID in the payload.id field for message events.
// For other events, we try common locations.
func extractEventID(event *WAHAWebhookEvent) string {
	// Check top-level ID first
	if event.ID != "" {
		return event.ID
	}

	// Check payload.id (most common for message events)
	if event.Payload != nil {
		if id, ok := event.Payload["id"].(string); ok && id != "" {
			return id
		}
		// Some events use _id
		if id, ok := event.Payload["_id"].(string); ok && id != "" {
			return id
		}
		// Message events might have the ID nested
		if msg, ok := event.Payload["message"].(map[string]interface{}); ok {
			if id, ok := msg["id"].(string); ok && id != "" {
				return id
			}
		}
	}

	return ""
}

// generateFallbackEventID generates a fallback event ID when none is provided.
// W14: Uses crypto/rand for uniqueness instead of timestamp-only.
func generateFallbackEventID(session, eventType string) string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp if crypto/rand fails
		return fmt.Sprintf("%d:%s:%s", time.Now().UnixNano(), session, eventType)
	}
	return fmt.Sprintf("%s:%s:%s:%s", hex.EncodeToString(b), session, eventType, time.Now().Format("20060102T150405"))
}

// truncateBytes truncates a byte slice to a maximum length, adding "..." if truncated.
// Useful for logging body previews without overwhelming logs.
func truncateBytes(b []byte, maxLen int) []byte {
	if len(b) <= maxLen {
		return b
	}
	result := make([]byte, maxLen+3)
	copy(result, b[:maxLen])
	copy(result[maxLen:], "...")
	return result
}

// generateTraceID generates a random 16-byte (128-bit) trace ID encoded as hex.
// This follows the W3C Trace Context format for trace-id.
func generateTraceID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp-based ID if crypto/rand fails
		return fmt.Sprintf("%016x%016x", time.Now().UnixNano(), time.Now().UnixNano()>>8)
	}
	return hex.EncodeToString(b)
}

// getOrGenerateTraceID extracts trace ID from request header or generates a new one.
func getOrGenerateTraceID(c *gin.Context) string {
	traceID := c.GetHeader(traceIDHeader)
	if traceID == "" {
		traceID = generateTraceID()
	}
	return traceID
}
