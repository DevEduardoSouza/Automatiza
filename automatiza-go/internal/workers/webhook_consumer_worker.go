// Package workers provides background job processing.
// This file implements the webhook consumer worker that processes WAHA webhook events
// from the Asynq queue with retry logic, DLQ handling, and idempotent processing.
package workers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"automatiza-go/internal/metrics"
	"automatiza-go/internal/models"
	"automatiza-go/internal/observability"
	"automatiza-go/internal/repository"
	"automatiza-go/internal/services/reconciliation"

	"github.com/getsentry/sentry-go"
	"github.com/hibiken/asynq"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
)

// =============================================================================
// Constants
// =============================================================================

const (
	// MaxWebhookRetries is the maximum number of retry attempts before moving to DLQ.
	MaxWebhookRetries = 3

	// TypeWebhookDLQ is the task type for dead letter queue entries.
	TypeWebhookDLQ = "whatsapp:webhook:dlq"
)

// =============================================================================
// Processor Interfaces
// =============================================================================

// MessageWebhookProcessor handles message events from WAHA webhooks.
// Implementations must be idempotent as the same message may be processed multiple times.
type MessageWebhookProcessor interface {
	// ProcessMessage processes an incoming WhatsApp message.
	// sessionName is the WAHA session identifier.
	// payload contains the raw message data from the webhook.
	// Returns nil if successfully processed or already processed (idempotent).
	ProcessMessage(ctx context.Context, sessionName string, payload map[string]interface{}) error
}

// StatusWebhookProcessor handles session status events from WAHA webhooks.
type StatusWebhookProcessor interface {
	// ProcessStatus handles a session status change.
	// sessionName is the WAHA session identifier.
	// status is the new session status (e.g., "CONNECTED", "DISCONNECTED", "SCAN_QR_CODE").
	ProcessStatus(ctx context.Context, sessionName, status string) error
}

// GroupEventWebhookProcessor handles group-related events from WAHA webhooks.
type GroupEventWebhookProcessor interface {
	// ProcessGroupEvent handles a group event (join, leave, update, etc.).
	// sessionName is the WAHA session identifier.
	// eventType is the specific group event type (e.g., "group.v2.join", "group.v2.leave").
	// payload contains the raw event data from the webhook.
	ProcessGroupEvent(ctx context.Context, sessionName, eventType string, payload map[string]interface{}) error
}

// =============================================================================
// Errors
// =============================================================================

var (
	// ErrEventNotFound indicates the event was not found in the database.
	ErrEventNotFound = errors.New("webhook event not found")

	// ErrEventAlreadyProcessed indicates the event was already processed.
	ErrEventAlreadyProcessed = errors.New("webhook event already processed")

	// ErrMaxRetriesExceeded indicates the event exceeded the maximum retry count.
	ErrMaxRetriesExceeded = errors.New("maximum retries exceeded")
)

// =============================================================================
// Worker Implementation
// =============================================================================

// WebhookConsumerWorker processes WAHA webhook events from the Asynq queue.
// It handles different event types (messages, status changes, group events)
// with retry logic and dead letter queue support.
//
// Architecture:
//   - Receives tasks from Asynq queue (TypeWAHAWebhook)
//   - Verifies event exists in webhook_events table (idempotency)
//   - Routes to appropriate processor based on event type
//   - Updates event status in database
//   - Moves to DLQ after max retries
type WebhookConsumerWorker struct {
	webhookEventsRepo repository.WebhookEventsRepository
	reconcileSvc      *reconciliation.Service
	instanceRepo      repository.InstanceRepository
	groupRepo         repository.GroupRepository
	queueClient       *QueueClient
	logger            *zap.Logger
	metrics           *metrics.WebhookMetrics

	// Event processors (optional - can be nil if not needed)
	messageProcessor    MessageWebhookProcessor
	statusProcessor     StatusWebhookProcessor
	groupEventProcessor GroupEventWebhookProcessor

	// connectedSessions caches sessions known to be connected to avoid
	// redundant DB checks on every message event. Self-healing: if we
	// receive messages from a session, it must be WORKING in WAHA.
	connectedSessions sync.Map
}

// WebhookConsumerWorkerConfig holds configuration for creating a WebhookConsumerWorker.
type WebhookConsumerWorkerConfig struct {
	WebhookEventsRepo   repository.WebhookEventsRepository
	ReconcileSvc        *reconciliation.Service
	InstanceRepo        repository.InstanceRepository
	GroupRepo           repository.GroupRepository
	QueueClient         *QueueClient
	Logger              *zap.Logger
	Metrics             *metrics.WebhookMetrics
	MessageProcessor    MessageWebhookProcessor
	StatusProcessor     StatusWebhookProcessor
	GroupEventProcessor GroupEventWebhookProcessor
}

// NewWebhookConsumerWorker creates a new webhook consumer worker.
//
// Required parameters:
//   - webhookEventsRepo: Repository for event idempotency and status tracking
//   - instanceRepo: Repository for instance status updates
//   - queueClient: Queue client for enqueueing DLQ tasks
//   - logger: Structured logger
//
// Optional parameters (passed via config):
//   - reconcileSvc: Reconciliation service for recording events
//   - groupRepo: Repository for group updates
//   - messageProcessor: Handler for message events
//   - statusProcessor: Handler for status events
//   - groupEventProcessor: Handler for group events
func NewWebhookConsumerWorker(config WebhookConsumerWorkerConfig) *WebhookConsumerWorker {
	return &WebhookConsumerWorker{
		webhookEventsRepo:   config.WebhookEventsRepo,
		reconcileSvc:        config.ReconcileSvc,
		instanceRepo:        config.InstanceRepo,
		groupRepo:           config.GroupRepo,
		queueClient:         config.QueueClient,
		logger:              config.Logger.Named("webhook-consumer"),
		metrics:             config.Metrics,
		messageProcessor:    config.MessageProcessor,
		statusProcessor:     config.StatusProcessor,
		groupEventProcessor: config.GroupEventProcessor,
	}
}

// ProcessTask processes a webhook task from the queue.
// This is the main entry point called by Asynq.
//
// Process flow:
//  1. Parse task payload (WebhookJobData)
//  2. Verify event exists in webhook_events table
//  3. Mark as processing
//  4. Route to appropriate processor based on event type
//  5. On success: mark as processed
//  6. On failure: check retry count and handle accordingly
func (w *WebhookConsumerWorker) ProcessTask(ctx context.Context, task *asynq.Task) error {
	// T3.9: Check context cancellation before processing
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// 1. Parse task payload
	var data WebhookJobData
	if err := json.Unmarshal(task.Payload(), &data); err != nil {
		w.logger.Error("Failed to unmarshal webhook job data",
			zap.Error(err))
		// Don't retry malformed payloads
		return nil
	}

	// Create a logger with trace ID for distributed tracing
	logger := w.logger
	if data.TraceID != "" {
		logger = w.logger.With(zap.String("trace_id", data.TraceID))
	}

	logger.Debug("Processing webhook event",
		zap.String("eventID", data.EventID),
		zap.String("eventType", data.Event),
		zap.String("session", data.Session))

	// Track processing start time for metrics
	startTime := time.Now()

	// Increment in-flight gauge
	if w.metrics != nil {
		w.metrics.IncrementInFlight(data.Session)
		defer w.metrics.DecrementInFlight(data.Session)
	}

	// 2. Record event for reconciliation service (resets silence timer)
	if w.reconcileSvc != nil {
		w.reconcileSvc.RecordEvent(data.Session)
	}

	// 3. Verify event exists and get current status
	exists, err := w.webhookEventsRepo.Exists(ctx, data.EventID)
	if err != nil {
		logger.Error("Failed to check event existence",
			zap.String("eventID", data.EventID),
			zap.Error(err))
		return fmt.Errorf("check event existence: %w", err)
	}

	if !exists {
		// Event was not recorded - this shouldn't happen but handle gracefully
		// Try to insert it now
		inserted, err := w.webhookEventsRepo.InsertIfNotExists(ctx, data.EventID, data.Session, data.Event)
		if err != nil {
			logger.Error("Failed to insert missing event",
				zap.String("eventID", data.EventID),
				zap.Error(err))
			return fmt.Errorf("insert missing event: %w", err)
		}
		if !inserted {
			// Race condition - event exists now, continue processing
			logger.Debug("Event inserted by concurrent process",
				zap.String("eventID", data.EventID))
		}
	}

	// 4. Mark as processing
	if err := w.webhookEventsRepo.MarkProcessing(ctx, data.EventID); err != nil {
		logger.Error("Failed to mark event as processing",
			zap.String("eventID", data.EventID),
			zap.Error(err))
		// Continue anyway - this is not critical
	}

	// 5. Route to appropriate processor
	processErr := w.routeEvent(ctx, &data)

	// 6. Handle result
	if processErr != nil {
		return w.handleProcessingError(ctx, task, &data, processErr)
	}

	// 7. Mark as processed on success
	if err := w.webhookEventsRepo.MarkProcessed(ctx, data.EventID); err != nil {
		logger.Error("Failed to mark event as processed",
			zap.String("eventID", data.EventID),
			zap.Error(err))
		// Don't fail the task - processing was successful
	}

	logger.Debug("Webhook event processed successfully",
		zap.String("eventID", data.EventID),
		zap.String("eventType", data.Event))

	// Record success metrics
	if w.metrics != nil {
		duration := time.Since(startTime).Seconds()
		w.metrics.RecordProcessed(data.Session, data.Event, duration)
	}

	if observability.GlobalMetrics != nil {
		observability.GlobalMetrics.WebhookEvents.Add(ctx, 1,
			metric.WithAttributes(attribute.String("event_type", data.Event)))
	}

	return nil
}

// routeEvent routes the webhook event to the appropriate processor.
func (w *WebhookConsumerWorker) routeEvent(ctx context.Context, data *WebhookJobData) error {
	// T3.9: Check context cancellation before routing
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	eventType := strings.ToLower(data.Event)

	switch {
	// Message events
	case eventType == "message" || eventType == "message.any":
		return w.handleMessageEvent(ctx, data)

	// Session status events
	case eventType == "session.status":
		return w.handleSessionStatusEvent(ctx, data)

	// Group events (v2 format from WAHA)
	case strings.HasPrefix(eventType, "group.v2.") || strings.HasPrefix(eventType, "group."):
		return w.handleGroupEvent(ctx, data)

	// Message acknowledgment events
	case eventType == "message.ack":
		return w.handleMessageAckEvent(ctx, data)

	// Message reaction events
	case eventType == "message.reaction":
		return w.handleMessageReactionEvent(ctx, data)

	// QR code events
	case eventType == "qr" || eventType == "qr.code":
		return w.handleQRCodeEvent(ctx, data)

	// Call events
	case eventType == "call":
		w.logger.Debug("Ignoring call event",
			zap.String("session", data.Session))
		return nil

	// Unknown event types - log and continue
	default:
		w.logger.Debug("Unknown event type, skipping",
			zap.String("eventType", data.Event),
			zap.String("session", data.Session))
		return nil
	}
}

// handleMessageEvent processes message events.
func (w *WebhookConsumerWorker) handleMessageEvent(ctx context.Context, data *WebhookJobData) error {
	if w.messageProcessor == nil {
		w.logger.Debug("No message processor configured, skipping message event",
			zap.String("session", data.Session))
		return nil
	}

	// Self-healing: receiving a message proves the session is WORKING.
	// Update the DB if the instance status is stale (pending/disconnected).
	w.ensureInstanceConnected(ctx, data.Session)

	return w.messageProcessor.ProcessMessage(ctx, data.Session, data.Payload)
}

// ensureInstanceConnected checks if the instance for a session is marked as
// connected in the database. If not, it updates the status. This self-heals
// cases where session.status webhooks were lost or never enqueued.
// Uses an in-memory cache to avoid DB lookups on every message.
func (w *WebhookConsumerWorker) ensureInstanceConnected(ctx context.Context, sessionName string) {
	// Fast path: already verified this session
	if _, ok := w.connectedSessions.Load(sessionName); ok {
		return
	}

	if w.instanceRepo == nil {
		return
	}

	instance, err := w.instanceRepo.GetInstanceByName(ctx, sessionName)
	if err != nil || instance == nil {
		return
	}

	if instance.Status == models.InstanceStatusConnected {
		// Already connected, cache it
		w.connectedSessions.Store(sessionName, true)
		return
	}

	// Instance is not connected but we're receiving messages — fix it
	if err := w.instanceRepo.UpdateInstanceStatus(ctx, instance.ID, models.InstanceStatusConnected); err != nil {
		w.logger.Warn("Failed to self-heal instance status",
			zap.String("session", sessionName),
			zap.Int("instanceID", instance.ID),
			zap.String("oldStatus", instance.Status),
			zap.Error(err))
		return
	}

	w.logger.Info("Self-healed instance status to connected (receiving messages)",
		zap.String("session", sessionName),
		zap.Int("instanceID", instance.ID),
		zap.String("oldStatus", instance.Status))

	w.connectedSessions.Store(sessionName, true)
}

// handleSessionStatusEvent processes session status change events.
func (w *WebhookConsumerWorker) handleSessionStatusEvent(ctx context.Context, data *WebhookJobData) error {
	// Extract status from payload
	status, _ := extractString(data.Payload, "status")
	if status == "" {
		w.logger.Warn("Session status event missing status field",
			zap.String("session", data.Session),
			zap.Any("payload", data.Payload))
		return nil
	}

	w.logger.Info("Processing session status change",
		zap.String("session", data.Session),
		zap.String("status", status))

	// Update instance status in database
	if w.instanceRepo != nil {
		instance, err := w.instanceRepo.GetInstanceByName(ctx, data.Session)
		if err != nil {
			w.logger.Warn("Failed to get instance by name",
				zap.String("session", data.Session),
				zap.Error(err))
		} else if instance != nil {
			dbStatus := mapWAHAStatusToDBStatus(status)

			// Invalidate connected-sessions cache when status changes away from connected
			if dbStatus != models.InstanceStatusConnected {
				w.connectedSessions.Delete(data.Session)
			}

			if err := w.instanceRepo.UpdateInstanceStatus(ctx, instance.ID, dbStatus); err != nil {
				w.logger.Error("Failed to update instance status",
					zap.String("session", data.Session),
					zap.Int("instanceID", instance.ID),
					zap.String("status", dbStatus),
					zap.Error(err))
				return fmt.Errorf("update instance status: %w", err)
			}

			w.logger.Info("Updated instance status",
				zap.String("session", data.Session),
				zap.Int("instanceID", instance.ID),
				zap.String("status", dbStatus))

			// Update cache if connected
			if dbStatus == models.InstanceStatusConnected {
				w.connectedSessions.Store(data.Session, true)
			}

			// Auto-sync groups when session becomes connected
			// This migrates groups from old disconnected instances to the new one
			if dbStatus == "connected" && w.queueClient != nil {
				w.enqueueAutoGroupSync(ctx, instance, data.Session)
			}
		}
	}

	// Call status processor if configured
	if w.statusProcessor != nil {
		if err := w.statusProcessor.ProcessStatus(ctx, data.Session, status); err != nil {
			w.logger.Warn("Status processor returned error",
				zap.String("session", data.Session),
				zap.String("status", status),
				zap.Error(err))
			// Don't fail the task - DB update was successful
		}
	}

	return nil
}

// handleGroupEvent processes group-related events.
func (w *WebhookConsumerWorker) handleGroupEvent(ctx context.Context, data *WebhookJobData) error {
	w.logger.Debug("Processing group event",
		zap.String("eventType", data.Event),
		zap.String("session", data.Session))

	if w.groupEventProcessor == nil {
		w.logger.Debug("No group event processor configured, skipping",
			zap.String("session", data.Session))
		return nil
	}

	return w.groupEventProcessor.ProcessGroupEvent(ctx, data.Session, data.Event, data.Payload)
}

// handleMessageAckEvent processes message acknowledgment events.
// These indicate delivery/read status of sent messages.
func (w *WebhookConsumerWorker) handleMessageAckEvent(ctx context.Context, data *WebhookJobData) error {
	// Extract relevant fields for logging
	messageID, _ := extractString(data.Payload, "id")
	ack, _ := extractInt(data.Payload, "ack")

	w.logger.Debug("Message ACK received",
		zap.String("session", data.Session),
		zap.String("messageID", messageID),
		zap.Int("ack", ack))

	// ACK values from WAHA:
	// 0 = pending, 1 = sent (server), 2 = received, 3 = read, 4 = played (for audio)
	// Currently we just log these - could update dispatch_logs in the future

	return nil
}

// handleMessageReactionEvent processes message reaction events.
func (w *WebhookConsumerWorker) handleMessageReactionEvent(ctx context.Context, data *WebhookJobData) error {
	// Extract relevant fields for logging
	messageID, _ := extractString(data.Payload, "msgId")
	reaction, _ := extractString(data.Payload, "text")
	from, _ := extractString(data.Payload, "from")

	w.logger.Debug("Message reaction received",
		zap.String("session", data.Session),
		zap.String("messageID", messageID),
		zap.String("reaction", reaction),
		zap.String("from", from))

	// Currently we just log reactions - could store/forward them in the future

	return nil
}

// handleQRCodeEvent processes QR code events for session authentication.
// When WAHA generates a new QR code, notify the frontend via SSE so it can
// fetch the updated QR image through the CheckStatus RPC.
func (w *WebhookConsumerWorker) handleQRCodeEvent(ctx context.Context, data *WebhookJobData) error {
	w.logger.Debug("QR code event received",
		zap.String("session", data.Session))

	// Notify via SSE that a new QR code is available.
	// The frontend receives SCAN_QR_CODE status and triggers a CheckStatus RPC
	// to fetch the actual QR code image from WAHA.
	if w.statusProcessor != nil {
		if err := w.statusProcessor.ProcessStatus(ctx, data.Session, "SCAN_QR_CODE"); err != nil {
			w.logger.Warn("Failed to notify QR code event via SSE",
				zap.String("session", data.Session),
				zap.Error(err))
		}
	}

	return nil
}

// handleProcessingError handles errors from event processing.
// It checks retry count and either returns error for retry or moves to DLQ.
func (w *WebhookConsumerWorker) handleProcessingError(
	ctx context.Context,
	task *asynq.Task,
	data *WebhookJobData,
	processErr error,
) error {
	// Create logger with trace ID for distributed tracing
	logger := w.logger
	if data.TraceID != "" {
		logger = w.logger.With(zap.String("trace_id", data.TraceID))
	}

	// Get current retry count from Asynq context
	retryCount, _ := asynq.GetRetryCount(ctx)

	logger.Warn("Webhook event processing failed",
		zap.String("eventID", data.EventID),
		zap.String("eventType", data.Event),
		zap.Int("retryCount", retryCount),
		zap.Int("maxRetries", MaxWebhookRetries),
		zap.Error(processErr))

	// Check if we should move to DLQ
	if retryCount >= MaxWebhookRetries {
		logger.Error("Max retries exceeded, moving to DLQ",
			zap.String("eventID", data.EventID),
			zap.String("eventType", data.Event),
			zap.Int("retryCount", retryCount))

		// T3.9: Capture DLQ events in Sentry for alerting
		observability.LogAndCapture(logger, sentry.LevelError,
			"Webhook event moved to DLQ after max retries",
			fmt.Errorf("DLQ: event %s (%s) failed after %d retries: %w", data.EventID, data.Event, retryCount, processErr),
			map[string]string{
				"module":     "webhook-consumer",
				"eventID":    data.EventID,
				"eventType":  data.Event,
				"session":    data.Session,
				"retryCount": fmt.Sprintf("%d", retryCount),
			})

		// Mark as DLQ in database
		if err := w.webhookEventsRepo.MarkDLQ(ctx, data.EventID, processErr.Error()); err != nil {
			logger.Error("Failed to mark event as DLQ",
				zap.String("eventID", data.EventID),
				zap.Error(err))
		}

		// Record DLQ metric
		if w.metrics != nil {
			w.metrics.RecordDLQ(data.Session, data.Event)
		}

		// Optionally enqueue to DLQ queue for later inspection/reprocessing
		if w.queueClient != nil {
			w.enqueueToDLQ(ctx, data, processErr)
		}

		// Return nil to stop retries
		return nil
	}

	// Check if error is retryable
	if !w.isRetryableError(processErr) {
		logger.Warn("Non-retryable error, marking as failed",
			zap.String("eventID", data.EventID),
			zap.Error(processErr))

		if err := w.webhookEventsRepo.MarkFailed(ctx, data.EventID, processErr.Error()); err != nil {
			logger.Error("Failed to mark event as failed",
				zap.String("eventID", data.EventID),
				zap.Error(err))
		}

		// Record failed metric (non-retryable)
		if w.metrics != nil {
			w.metrics.RecordFailed(data.Session, data.Event, "non_retryable")
		}

		return nil
	}

	// Mark as failed (will be retried by Asynq)
	if err := w.webhookEventsRepo.MarkFailed(ctx, data.EventID, processErr.Error()); err != nil {
		logger.Error("Failed to mark event as failed",
			zap.String("eventID", data.EventID),
			zap.Error(err))
	}

	// Record retry metric
	if w.metrics != nil {
		w.metrics.RecordRetry(data.Session, data.Event, retryCount+1)
	}

	// Return error to trigger Asynq retry
	return processErr
}

// isRetryableError determines if an error should trigger a retry.
func (w *WebhookConsumerWorker) isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())

	// Non-retryable errors
	nonRetryable := []string{
		"invalid",
		"malformed",
		"not found",
		"unauthorized",
		"forbidden",
		"permission denied",
		"already processed",
		"already exists",
	}

	for _, keyword := range nonRetryable {
		if strings.Contains(errStr, keyword) {
			return false
		}
	}

	// Retryable errors
	retryable := []string{
		"timeout",
		"deadline exceeded",
		"connection refused",
		"connection reset",
		"temporary",
		"unavailable",
		"rate limit",
		"too many requests",
		"internal server error",
		"bad gateway",
		"service unavailable",
	}

	for _, keyword := range retryable {
		if strings.Contains(errStr, keyword) {
			return true
		}
	}

	// Default: retry unknown errors
	return true
}

// enqueueToDLQ enqueues the event to the dead letter queue.
func (w *WebhookConsumerWorker) enqueueToDLQ(ctx context.Context, data *WebhookJobData, processErr error) {
	dlqPayload := map[string]interface{}{
		"original_event": data,
		"error":          processErr.Error(),
		"moved_at":       time.Now().UTC().Unix(),
	}

	payload, err := json.Marshal(dlqPayload)
	if err != nil {
		w.logger.Error("Failed to marshal DLQ payload",
			zap.String("eventID", data.EventID),
			zap.Error(err))
		return
	}

	task := asynq.NewTask(TypeWebhookDLQ, payload,
		asynq.Queue("dlq"),
		asynq.MaxRetry(0),               // No retries for DLQ
		asynq.Retention(7*24*time.Hour), // Keep for 7 days
		asynq.TaskID("dlq:"+data.EventID),
	)

	if _, err := w.queueClient.client.EnqueueContext(ctx, task); err != nil {
		if !IsDuplicateTaskError(err) {
			w.logger.Error("Failed to enqueue to DLQ",
				zap.String("eventID", data.EventID),
				zap.Error(err))
		}
	} else {
		w.logger.Info("Event moved to DLQ",
			zap.String("eventID", data.EventID))
	}
}

// ProcessTaskWrapper returns an asynq.HandlerFunc for registration.
func (w *WebhookConsumerWorker) ProcessTaskWrapper() asynq.HandlerFunc {
	return func(ctx context.Context, task *asynq.Task) error {
		return w.ProcessTask(ctx, task)
	}
}

// enqueueAutoGroupSync enqueues a background group sync job when a session connects.
// This ensures groups are automatically migrated from old disconnected instances
// to the newly connected one, preventing orphaned groups.
func (w *WebhookConsumerWorker) enqueueAutoGroupSync(ctx context.Context, instance *models.Instance, sessionName string) {
	syncData := &GroupsSyncJobData{
		UserID:       instance.UserID,
		InstanceName: sessionName,
		InstanceID:   instance.ID,
	}

	_, err := w.queueClient.EnqueueGroupsSync(ctx, syncData)
	if err != nil {
		w.logger.Warn("Failed to enqueue auto group sync on session connect",
			zap.String("session", sessionName),
			zap.Int("instanceID", instance.ID),
			zap.String("userID", instance.UserID),
			zap.Error(err))
		return
	}

	w.logger.Info("Auto group sync enqueued on session connect",
		zap.String("session", sessionName),
		zap.Int("instanceID", instance.ID),
		zap.String("userID", instance.UserID))
}

// =============================================================================
// Helper Functions
// =============================================================================

// mapWAHAStatusToDBStatus maps WAHA session status to database status.
func mapWAHAStatusToDBStatus(wahaStatus string) string {
	switch strings.ToUpper(wahaStatus) {
	case "CONNECTED", "WORKING":
		return "connected"
	case "DISCONNECTED", "FAILED", "STOPPED":
		return "disconnected"
	case "SCAN_QR_CODE", "STARTING":
		return "pending"
	default:
		return "disconnected"
	}
}

// extractString safely extracts a string from a map.
func extractString(m map[string]interface{}, key string) (string, bool) {
	if m == nil {
		return "", false
	}
	v, ok := m[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

// extractInt safely extracts an int from a map.
func extractInt(m map[string]interface{}, key string) (int, bool) {
	if m == nil {
		return 0, false
	}
	v, ok := m[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	default:
		return 0, false
	}
}

// =============================================================================
// Ensure interface compliance
// =============================================================================

var _ asynq.Handler = (*WebhookConsumerWorker)(nil)
