// Package workers provides background job processing for the Automatiza platform.
package workers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"

	"github.com/hibiken/asynq"
	"go.uber.org/zap"
)

// Message task type constants for the queue.
const (
	TypeMessageText  = "message:text"
	TypeMessageAudio = "message:audio"
	TypeMessageImage = "message:image"
	TypeMessagePoll  = "message:poll"
)

// Default queue configuration.
const (
	defaultMessageQueue    = "messages"
	maxRetries             = 3
	taskTimeout            = 30 * time.Second
	dlqRetention           = 24 * time.Hour
	maxBackpressureDelay   = 5 * time.Second
	backpressureDelayPerMs = 100 * time.Millisecond
)

// MessageTask represents a message to be sent via the queue.
// This struct is serialized as the asynq task payload.
type MessageTask struct {
	// Type is the message type: text, audio, image, poll
	Type string `json:"type"`

	// UserID is the owner of the action
	UserID string `json:"user_id"`

	// InstanceID is the database instance ID (for logging/tracking)
	InstanceID string `json:"instance_id"`

	// SessionName is the WAHA session name to use for sending
	SessionName string `json:"session_name"`

	// GroupID is the WhatsApp chat/group ID (e.g., "123456789@g.us")
	GroupID string `json:"group_id"`

	// Payload contains type-specific message data
	Payload json.RawMessage `json:"payload"`

	// ActionID is the ID of the action_history entry to update
	ActionID string `json:"action_id"`
}

// TextMessagePayload is the payload for text messages.
type TextMessagePayload struct {
	Text     string   `json:"text"`
	Mentions []string `json:"mentions,omitempty"`
}

// AudioMessagePayload is the payload for audio/voice messages.
type AudioMessagePayload struct {
	AudioBase64 string `json:"audio_base64"`
	Mimetype    string `json:"mimetype,omitempty"`
}

// ImageMessagePayload is the payload for image messages.
type ImageMessagePayload struct {
	Caption     string `json:"caption,omitempty"`
	ImageURL    string `json:"image_url,omitempty"`
	ImageBase64 string `json:"image_base64,omitempty"`
	Mimetype    string `json:"mimetype,omitempty"`
	Filename    string `json:"filename,omitempty"`
}

// PollMessagePayload is the payload for poll messages.
type PollMessagePayload struct {
	Question        string   `json:"question"`
	Options         []string `json:"options"`
	SelectableCount int      `json:"selectable_count,omitempty"`
}

// MessageQueueWorker processes message sending tasks from the queue.
// It uses the WhatsAppProvider interface (adapter pattern) for sending messages,
// decoupling the worker from specific WhatsApp API implementations.
type MessageQueueWorker struct {
	whatsappProvider  whatsapp.WhatsAppProvider
	actionHistoryRepo repository.ActionHistoryRepository
	inspector         *asynq.Inspector
	client            *asynq.Client
	logger            *zap.Logger
}

// NewMessageQueueWorker creates a new message queue worker.
func NewMessageQueueWorker(
	whatsappProvider whatsapp.WhatsAppProvider,
	actionHistoryRepo repository.ActionHistoryRepository,
	inspector *asynq.Inspector,
	client *asynq.Client,
	logger *zap.Logger,
) *MessageQueueWorker {
	return &MessageQueueWorker{
		whatsappProvider:  whatsappProvider,
		actionHistoryRepo: actionHistoryRepo,
		inspector:         inspector,
		client:            client,
		logger:            logger,
	}
}

// ProcessTask processes a message task from the queue.
// It parses the task, executes the appropriate handler based on message type,
// and updates the action_history status.
func (w *MessageQueueWorker) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var msgTask MessageTask
	if err := json.Unmarshal(task.Payload(), &msgTask); err != nil {
		w.logger.Error("Failed to unmarshal message task",
			zap.Error(err),
			zap.String("taskType", task.Type()))
		return fmt.Errorf("failed to unmarshal message task: %w", err)
	}

	w.logger.Debug("Processing message task",
		zap.String("type", msgTask.Type),
		zap.String("userID", msgTask.UserID),
		zap.String("sessionName", msgTask.SessionName),
		zap.String("groupID", msgTask.GroupID),
		zap.String("actionID", msgTask.ActionID))

	// Mark action as running
	if msgTask.ActionID != "" && w.actionHistoryRepo != nil {
		if err := w.actionHistoryRepo.UpdateStatus(ctx, msgTask.ActionID, models.ActionStatusRunning, ""); err != nil {
			w.logger.Warn("Failed to mark action as running",
				zap.String("actionID", msgTask.ActionID),
				zap.Error(err))
		}
	}

	// Execute based on message type
	var sendErr error
	switch msgTask.Type {
	case TypeMessageText:
		sendErr = w.handleTextMessage(ctx, &msgTask)
	case TypeMessageAudio:
		sendErr = w.handleAudioMessage(ctx, &msgTask)
	case TypeMessageImage:
		sendErr = w.handleImageMessage(ctx, &msgTask)
	case TypeMessagePoll:
		sendErr = w.handlePollMessage(ctx, &msgTask)
	default:
		sendErr = fmt.Errorf("unknown message type: %s", msgTask.Type)
	}

	// Update action_history status.
	// Desfechos terminais usam ctx DESACOPLADO: com Timeout(30s) da task, o
	// ctx cru chega morto aqui após um envio lento e a action ficava presa em
	// 'running' para sempre (mesmo bug do dispatch_log 'queued' eterno).
	if sendErr != nil {
		// Desfecho AMBÍGUO (timeout/reset/EOF/502): o WAHA pode ter entregue.
		// Marcar failed + retry re-enviaria — assume-sent, mesmo contrato do
		// message_worker.
		if whatsapp.IsAmbiguousOutcome(sendErr) {
			w.logger.Warn("Ambiguous send outcome — assuming sent (no retry)",
				zap.String("type", msgTask.Type),
				zap.String("groupID", msgTask.GroupID),
				zap.Error(sendErr))
			w.updateActionStatus(ctx, msgTask.ActionID, models.ActionStatusCompleted,
				"desfecho incerto (timeout/conexao) - assumido como enviado")
			return nil
		}

		w.logger.Error("Message send failed",
			zap.String("type", msgTask.Type),
			zap.String("groupID", msgTask.GroupID),
			zap.Error(sendErr))

		w.updateActionStatus(ctx, msgTask.ActionID, models.ActionStatusFailed, sendErr.Error())

		// Check if error is retryable
		if !w.isRetryableError(sendErr) {
			// Return nil to prevent retry for non-retryable errors
			return nil
		}

		return sendErr
	}

	// Success: update action_history
	w.logger.Debug("Message sent successfully",
		zap.String("type", msgTask.Type),
		zap.String("groupID", msgTask.GroupID),
		zap.String("actionID", msgTask.ActionID))

	w.updateActionStatus(ctx, msgTask.ActionID, models.ActionStatusCompleted, "")

	return nil
}

// handleTextMessage processes a text message task.
func (w *MessageQueueWorker) handleTextMessage(ctx context.Context, task *MessageTask) error {
	var payload TextMessagePayload
	if err := json.Unmarshal(task.Payload, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal text payload: %w", err)
	}

	if payload.Text == "" {
		return fmt.Errorf("text message cannot be empty")
	}

	input := whatsapp.SendTextInput{
		SessionName: task.SessionName,
		ChatID:      task.GroupID,
		Text:        payload.Text,
		Mentions:    payload.Mentions,
	}

	_, err := w.whatsappProvider.SendText(ctx, input)
	return err
}

// handleAudioMessage processes an audio/voice message task.
func (w *MessageQueueWorker) handleAudioMessage(ctx context.Context, task *MessageTask) error {
	var payload AudioMessagePayload
	if err := json.Unmarshal(task.Payload, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal audio payload: %w", err)
	}

	if payload.AudioBase64 == "" {
		return fmt.Errorf("audio message requires audio_base64")
	}

	// Default mimetype for voice messages
	mimetype := payload.Mimetype
	if mimetype == "" {
		mimetype = "audio/ogg; codecs=opus"
	}

	input := whatsapp.SendVoiceInput{
		SessionName: task.SessionName,
		ChatID:      task.GroupID,
		AudioBase64: payload.AudioBase64,
		Mimetype:    mimetype,
	}

	_, err := w.whatsappProvider.SendVoice(ctx, input)
	return err
}

// handleImageMessage processes an image message task.
func (w *MessageQueueWorker) handleImageMessage(ctx context.Context, task *MessageTask) error {
	var payload ImageMessagePayload
	if err := json.Unmarshal(task.Payload, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal image payload: %w", err)
	}

	if payload.ImageURL == "" && payload.ImageBase64 == "" {
		return fmt.Errorf("image message requires image_url or image_base64")
	}

	// Handle base64 data URL format
	imageBase64 := payload.ImageBase64
	if imageBase64 != "" && strings.HasPrefix(imageBase64, "data:") {
		parts := strings.SplitN(imageBase64, ",", 2)
		if len(parts) == 2 {
			imageBase64 = parts[1]
		}
	}

	// Default mimetype
	mimetype := payload.Mimetype
	if mimetype == "" && imageBase64 != "" {
		mimetype = "image/jpeg"
	}

	input := whatsapp.SendImageInput{
		SessionName: task.SessionName,
		ChatID:      task.GroupID,
		Caption:     payload.Caption,
		ImageURL:    payload.ImageURL,
		ImageBase64: imageBase64,
		Mimetype:    mimetype,
		Filename:    payload.Filename,
	}

	_, err := w.whatsappProvider.SendImage(ctx, input)
	return err
}

// handlePollMessage processes a poll message task.
func (w *MessageQueueWorker) handlePollMessage(ctx context.Context, task *MessageTask) error {
	var payload PollMessagePayload
	if err := json.Unmarshal(task.Payload, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal poll payload: %w", err)
	}

	if payload.Question == "" {
		return fmt.Errorf("poll message requires a question")
	}

	if len(payload.Options) < 2 {
		return fmt.Errorf("poll message requires at least 2 options")
	}

	if len(payload.Options) > 12 {
		return fmt.Errorf("poll message cannot have more than 12 options")
	}

	selectableCount := payload.SelectableCount
	if selectableCount <= 0 {
		selectableCount = 1
	}
	if selectableCount > len(payload.Options) {
		selectableCount = len(payload.Options)
	}

	input := whatsapp.SendPollInput{
		SessionName:     task.SessionName,
		ChatID:          task.GroupID,
		Question:        payload.Question,
		Options:         payload.Options,
		SelectableCount: selectableCount,
	}

	_, err := w.whatsappProvider.SendPoll(ctx, input)
	return err
}

// isRetryableError determines if an error should trigger a retry.
func (w *MessageQueueWorker) isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Check for provider-specific error codes
	var providerErr *whatsapp.ProviderError
	if errors.As(err, &providerErr) {
		switch providerErr.Code {
		case whatsapp.ErrCodeSessionNotFound:
			// Session doesn't exist - no point retrying
			return false
		case whatsapp.ErrCodeGroupNotFound:
			// Group doesn't exist - no point retrying
			return false
		case whatsapp.ErrCodeInvalidInput:
			// Invalid input - no point retrying
			return false
		case whatsapp.ErrCodePermissionDenied:
			// No permission - no point retrying
			return false
		case whatsapp.ErrCodeTimeout:
			// Timeout might be transient - assume sent, don't retry
			return false
		case whatsapp.ErrCodeSessionNotConnected:
			// Session might reconnect - retry
			return true
		case whatsapp.ErrCodeRateLimited:
			// Rate limit might clear - retry with backoff
			return true
		case whatsapp.ErrCodeNetworkError:
			// Network issues might resolve - retry
			return true
		case whatsapp.ErrCodeProviderError:
			// Provider error might be transient - retry
			return true
		}
	}

	// Desfecho ambíguo (timeout/reset/EOF/502 pós-POST): NUNCA retryable —
	// re-executar pode duplicar a mensagem. (O chamador já trata ambíguo como
	// assumed-sent antes de chegar aqui; isto é defesa em profundidade,
	// alinhada ao IsAmbiguousOutcome/send_outcome.go.)
	if whatsapp.IsAmbiguousOutcome(err) {
		return false
	}

	// Legacy error detection
	errStr := strings.ToLower(err.Error())

	// Non-retryable errors
	if strings.Contains(errStr, "404") ||
		strings.Contains(errStr, "not found") ||
		strings.Contains(errStr, "invalid") ||
		strings.Contains(errStr, "permission") {
		return false
	}

	// Retryable errors (refused = TCP nem conectou; comprovadamente não enviado)
	if strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "rate limit") {
		return true
	}

	// Default: retry
	return true
}

// updateActionStatus grava o status da action com contexto DESACOPLADO — o
// desfecho precisa persistir mesmo com o ctx da task já estourado (envio
// consumiu o Timeout de 30s), senão a action fica presa em 'running'.
func (w *MessageQueueWorker) updateActionStatus(ctx context.Context, actionID, status, note string) {
	if actionID == "" || w.actionHistoryRepo == nil {
		return
	}
	updCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := w.actionHistoryRepo.UpdateStatus(updCtx, actionID, status, note); err != nil {
		w.logger.Warn("Failed to update action status",
			zap.String("actionID", actionID),
			zap.String("status", status),
			zap.Error(err))
	}
}

// EnqueueWithBackpressure enqueues a message task with backpressure based on queue depth.
// If the queue is deep, it adds a delay to the task to prevent overwhelming the system.
func (w *MessageQueueWorker) EnqueueWithBackpressure(ctx context.Context, task MessageTask) error {
	payload, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("failed to marshal task: %w", err)
	}

	// Calculate delay based on queue depth
	delay := time.Duration(0)
	if w.inspector != nil {
		queueInfo, err := w.inspector.GetQueueInfo(defaultMessageQueue)
		if err == nil && queueInfo != nil {
			depth := queueInfo.Pending + queueInfo.Active
			delay = time.Duration(depth) * backpressureDelayPerMs
			if delay > maxBackpressureDelay {
				delay = maxBackpressureDelay
			}
		}
	}

	// Build asynq options
	opts := []asynq.Option{
		asynq.MaxRetry(maxRetries),
		asynq.Timeout(taskTimeout),
		asynq.Queue(defaultMessageQueue),
		asynq.Retention(dlqRetention),
	}

	if delay > 0 {
		opts = append(opts, asynq.ProcessIn(delay))
		w.logger.Debug("Enqueuing task with backpressure delay",
			zap.Duration("delay", delay),
			zap.String("type", task.Type),
			zap.String("groupID", task.GroupID))
	}

	// Use ActionID as task ID for deduplication
	if task.ActionID != "" {
		opts = append(opts, asynq.TaskID("msg:"+task.ActionID))
	}

	asynqTask := asynq.NewTask(task.Type, payload, opts...)
	_, err = w.client.EnqueueContext(ctx, asynqTask)
	if err != nil {
		if IsDuplicateTaskError(err) {
			w.logger.Debug("Duplicate task ignored",
				zap.String("actionID", task.ActionID))
			return nil
		}
		return fmt.Errorf("failed to enqueue task: %w", err)
	}

	return nil
}

// Enqueue enqueues a message task without backpressure.
func (w *MessageQueueWorker) Enqueue(ctx context.Context, task MessageTask) error {
	payload, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("failed to marshal task: %w", err)
	}

	opts := []asynq.Option{
		asynq.MaxRetry(maxRetries),
		asynq.Timeout(taskTimeout),
		asynq.Queue(defaultMessageQueue),
		asynq.Retention(dlqRetention),
	}

	// Use ActionID as task ID for deduplication
	if task.ActionID != "" {
		opts = append(opts, asynq.TaskID("msg:"+task.ActionID))
	}

	asynqTask := asynq.NewTask(task.Type, payload, opts...)
	_, err = w.client.EnqueueContext(ctx, asynqTask)
	if err != nil {
		if IsDuplicateTaskError(err) {
			w.logger.Debug("Duplicate task ignored",
				zap.String("actionID", task.ActionID))
			return nil
		}
		return fmt.Errorf("failed to enqueue task: %w", err)
	}

	return nil
}

// EnqueueTextMessage is a convenience method to enqueue a text message.
func (w *MessageQueueWorker) EnqueueTextMessage(
	ctx context.Context,
	userID, instanceID, sessionName, groupID, actionID, text string,
	mentions []string,
) error {
	payload, err := json.Marshal(TextMessagePayload{
		Text:     text,
		Mentions: mentions,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal text payload: %w", err)
	}

	task := MessageTask{
		Type:        TypeMessageText,
		UserID:      userID,
		InstanceID:  instanceID,
		SessionName: sessionName,
		GroupID:     groupID,
		Payload:     payload,
		ActionID:    actionID,
	}

	return w.EnqueueWithBackpressure(ctx, task)
}

// EnqueueAudioMessage is a convenience method to enqueue an audio message.
func (w *MessageQueueWorker) EnqueueAudioMessage(
	ctx context.Context,
	userID, instanceID, sessionName, groupID, actionID, audioBase64, mimetype string,
) error {
	payload, err := json.Marshal(AudioMessagePayload{
		AudioBase64: audioBase64,
		Mimetype:    mimetype,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal audio payload: %w", err)
	}

	task := MessageTask{
		Type:        TypeMessageAudio,
		UserID:      userID,
		InstanceID:  instanceID,
		SessionName: sessionName,
		GroupID:     groupID,
		Payload:     payload,
		ActionID:    actionID,
	}

	return w.EnqueueWithBackpressure(ctx, task)
}

// EnqueueImageMessage is a convenience method to enqueue an image message.
func (w *MessageQueueWorker) EnqueueImageMessage(
	ctx context.Context,
	userID, instanceID, sessionName, groupID, actionID string,
	imageURL, imageBase64, caption, mimetype, filename string,
) error {
	payload, err := json.Marshal(ImageMessagePayload{
		Caption:     caption,
		ImageURL:    imageURL,
		ImageBase64: imageBase64,
		Mimetype:    mimetype,
		Filename:    filename,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal image payload: %w", err)
	}

	task := MessageTask{
		Type:        TypeMessageImage,
		UserID:      userID,
		InstanceID:  instanceID,
		SessionName: sessionName,
		GroupID:     groupID,
		Payload:     payload,
		ActionID:    actionID,
	}

	return w.EnqueueWithBackpressure(ctx, task)
}

// EnqueuePollMessage is a convenience method to enqueue a poll message.
func (w *MessageQueueWorker) EnqueuePollMessage(
	ctx context.Context,
	userID, instanceID, sessionName, groupID, actionID, question string,
	options []string,
	selectableCount int,
) error {
	payload, err := json.Marshal(PollMessagePayload{
		Question:        question,
		Options:         options,
		SelectableCount: selectableCount,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal poll payload: %w", err)
	}

	task := MessageTask{
		Type:        TypeMessagePoll,
		UserID:      userID,
		InstanceID:  instanceID,
		SessionName: sessionName,
		GroupID:     groupID,
		Payload:     payload,
		ActionID:    actionID,
	}

	return w.EnqueueWithBackpressure(ctx, task)
}

// ProcessTaskWrapper returns an asynq.HandlerFunc that wraps ProcessTask.
// This is used to register the worker with the asynq server.
func (w *MessageQueueWorker) ProcessTaskWrapper() asynq.HandlerFunc {
	return func(ctx context.Context, task *asynq.Task) error {
		return w.ProcessTask(ctx, task)
	}
}

// Ensure MessageQueueWorker implements asynq.Handler interface.
var _ asynq.Handler = (*MessageQueueWorker)(nil)
