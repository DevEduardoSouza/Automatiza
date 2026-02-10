package workers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/config"
	"automatiza-go/internal/lib/text"
	"automatiza-go/internal/lib/urlsafety"
	"automatiza-go/internal/repository"

	"github.com/hibiken/asynq"
	"go.uber.org/zap"
)

// MessageWorker processes message sending tasks.
// It uses the WhatsAppProvider interface (adapter pattern) for sending messages,
// decoupling the worker from specific WhatsApp API implementations.
type MessageWorker struct {
	whatsappProvider whatsapp.WhatsAppProvider
	urlSafetyService *urlsafety.Service
	historyRepo      repository.ActionHistoryRepository
	dispatchLogRepo  repository.DispatchLogRepository
	logger           *zap.Logger
}

// NewMessageWorker creates a new message worker.
// IMPORTANT: This now requires a WhatsAppProvider instead of waha.Service.
func NewMessageWorker(
	whatsappProvider whatsapp.WhatsAppProvider,
	urlSafetyService *urlsafety.Service,
	historyRepo repository.ActionHistoryRepository,
	dispatchLogRepo repository.DispatchLogRepository,
	logger *zap.Logger,
) *MessageWorker {
	return &MessageWorker{
		whatsappProvider: whatsappProvider,
		urlSafetyService: urlSafetyService,
		historyRepo:      historyRepo,
		dispatchLogRepo:  dispatchLogRepo,
		logger:           logger,
	}
}

// ProcessTask processes a message sending task
func (w *MessageWorker) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var data MessageJobData
	if err := json.Unmarshal(task.Payload(), &data); err != nil {
		return fmt.Errorf("failed to unmarshal job data: %w", err)
	}

	w.logger.Info("Processing message job",
		zap.String("automationID", data.AutomationID),
		zap.String("instanceName", data.InstanceName),
		zap.String("groupId", data.GroupID),
		zap.Bool("hasImage", data.ImageURL != nil && *data.ImageURL != ""),
		zap.Bool("hasAudio", data.AudioBase64 != nil && *data.AudioBase64 != ""),
		zap.Int("messageLen", len(data.Message)))

	// 1. URL Safety Check
	if w.urlSafetyService != nil {
		decision, err := w.urlSafetyService.Evaluate(ctx, data.Message)
		if err != nil {
			w.logger.Warn("URL safety check failed", zap.Error(err))
			// Continue anyway
		} else if decision.Action == "block" {
			// Update dispatch log with blocked status
			w.updateDispatchLog(ctx, data.DispatchLogID, map[string]interface{}{
				"status": "blocked",
				"error":  fmt.Sprintf("URL bloqueada: %s", decision.Reason),
			})
			w.logger.Info("Message blocked by URL safety",
				zap.String("reason", decision.Reason))
			return nil // Don't retry blocked messages
		} else if decision.Action == "redact" {
			// Redact blocked URLs from message
			data.Message = w.urlSafetyService.RedactBlockedURLs(data.Message, decision)
		}
	}

	// 2. Text Normalization
	normalizedMessage := text.NormalizeOutgoingWhatsAppText(data.Message)
	if text.IsEffectivelyEmptyWhatsAppText(normalizedMessage) {
		w.logger.Info("Message empty after normalization, skipping",
			zap.String("automationID", data.AutomationID))
		w.updateDispatchLog(ctx, data.DispatchLogID, map[string]interface{}{
			"status": "skipped",
			"error":  "Mensagem vazia apos normalizacao",
		})
		return nil // Empty message after normalization
	}

	// 3. Send Message via WhatsAppProvider
	var sendErr error
	if data.AudioBase64 != nil && *data.AudioBase64 != "" {
		// Send voice message
		mimetype := "audio/ogg; codecs=opus"
		if data.Mimetype != nil && *data.Mimetype != "" {
			mimetype = *data.Mimetype
		}

		w.logger.Info("Sending voice message",
			zap.String("automationID", data.AutomationID),
			zap.String("groupId", data.GroupID))

		voiceInput := whatsapp.SendVoiceInput{
			SessionName: data.InstanceName,
			ChatID:      data.GroupID,
			AudioBase64: *data.AudioBase64,
			Mimetype:    mimetype,
		}
		_, sendErr = w.whatsappProvider.SendVoice(ctx, voiceInput)

		if sendErr == nil && normalizedMessage != "" {
			// Wait a bit then send text
			time.Sleep(1500 * time.Millisecond)
			textInput := whatsapp.SendTextInput{
				SessionName: data.InstanceName,
				ChatID:      data.GroupID,
				Text:        normalizedMessage,
				Mentions:    data.Mentions,
			}
			_, sendErr = w.whatsappProvider.SendText(ctx, textInput)
		}
	} else if data.ImageURL != nil && *data.ImageURL != "" {
		// Send image with URL
		w.logger.Info("Sending image message",
			zap.String("automationID", data.AutomationID),
			zap.String("groupId", data.GroupID),
			zap.String("imageURL", *data.ImageURL))

		imageInput := whatsapp.SendImageInput{
			SessionName: data.InstanceName,
			ChatID:      data.GroupID,
			Caption:     normalizedMessage,
			ImageURL:    *data.ImageURL,
		}
		_, sendErr = w.whatsappProvider.SendImage(ctx, imageInput)

		// Fallback: if image send fails, try sending text-only
		if sendErr != nil {
			w.logger.Warn("Image send failed, falling back to text-only",
				zap.String("automationID", data.AutomationID),
				zap.String("imageURL", *data.ImageURL),
				zap.String("groupId", data.GroupID),
				zap.Error(sendErr))
			textInput := whatsapp.SendTextInput{
				SessionName: data.InstanceName,
				ChatID:      data.GroupID,
				Text:        normalizedMessage,
				Mentions:    data.Mentions,
			}
			_, sendErr = w.whatsappProvider.SendText(ctx, textInput)
		}
	} else if data.ImageBase64 != nil && *data.ImageBase64 != "" {
		// Send image with base64
		w.logger.Info("Sending base64 image",
			zap.String("automationID", data.AutomationID),
			zap.String("groupId", data.GroupID))
		imageData := *data.ImageBase64
		if strings.HasPrefix(imageData, "data:") {
			// Extract base64 from data URL
			parts := strings.SplitN(imageData, ",", 2)
			if len(parts) == 2 {
				imageData = parts[1]
			}
		}
		imageInput := whatsapp.SendImageInput{
			SessionName: data.InstanceName,
			ChatID:      data.GroupID,
			Caption:     normalizedMessage,
			ImageBase64: imageData,
			Mimetype:    "image/jpeg",
		}
		_, sendErr = w.whatsappProvider.SendImage(ctx, imageInput)
	} else {
		// Send text message
		w.logger.Info("Sending text message",
			zap.String("automationID", data.AutomationID),
			zap.String("groupId", data.GroupID),
			zap.Int("textLen", len(normalizedMessage)))

		textInput := whatsapp.SendTextInput{
			SessionName: data.InstanceName,
			ChatID:      data.GroupID,
			Text:        normalizedMessage,
			Mentions:    data.Mentions,
		}
		_, sendErr = w.whatsappProvider.SendText(ctx, textInput)
	}

	// 4. Handle Errors
	if sendErr != nil {
		errStr := sendErr.Error()

		// Check for provider-specific error codes
		var providerErr *whatsapp.ProviderError
		if errors.As(sendErr, &providerErr) {
			switch providerErr.Code {
			case whatsapp.ErrCodeSessionNotFound:
				w.logger.Error("Session not found, failing permanently",
					zap.String("automationID", data.AutomationID),
					zap.String("instanceName", data.InstanceName),
					zap.Error(sendErr))
				w.updateHistoryStatus(ctx, data.HistoryID, "failed")
				w.updateDispatchLog(ctx, data.DispatchLogID, map[string]interface{}{
					"status": "failed",
					"error":  "Instancia nao encontrada",
				})
				return nil // Don't retry

			case whatsapp.ErrCodeSessionNotConnected:
				w.logger.Warn("Session not connected, will retry",
					zap.String("automationID", data.AutomationID),
					zap.String("instanceName", data.InstanceName),
					zap.Error(sendErr))
				return sendErr // Retry

			case whatsapp.ErrCodeRateLimited:
				w.logger.Warn("Rate limited, will retry with backoff",
					zap.String("automationID", data.AutomationID),
					zap.String("instanceName", data.InstanceName),
					zap.Error(sendErr))
				return sendErr // Retry with backoff

			case whatsapp.ErrCodeTimeout:
				// Timeout: assume sent, don't retry
				w.logger.Warn("Message send timeout, assuming sent",
					zap.String("automationID", data.AutomationID),
					zap.String("instanceName", data.InstanceName),
					zap.String("groupId", data.GroupID))
				w.updateHistoryStatus(ctx, data.HistoryID, "assumedSent")
				w.updateDispatchLog(ctx, data.DispatchLogID, map[string]interface{}{
					"status": "sent",
					"error":  "Timeout - assumido como enviado",
				})
				return nil
			}
		}

		// Legacy error detection for non-ProviderError cases
		if isTimeoutError(sendErr) {
			w.logger.Warn("Message send timeout, assuming sent",
				zap.String("automationID", data.AutomationID),
				zap.String("instanceName", data.InstanceName),
				zap.String("groupId", data.GroupID))
			w.updateHistoryStatus(ctx, data.HistoryID, "assumedSent")
			w.updateDispatchLog(ctx, data.DispatchLogID, map[string]interface{}{
				"status": "sent",
				"error":  "Timeout - assumido como enviado",
			})
			return nil
		}

		if is404Error(sendErr) {
			w.logger.Error("Instance not found, failing permanently",
				zap.String("automationID", data.AutomationID),
				zap.String("instanceName", data.InstanceName),
				zap.Error(sendErr))
			w.updateHistoryStatus(ctx, data.HistoryID, "failed")
			w.updateDispatchLog(ctx, data.DispatchLogID, map[string]interface{}{
				"status": "failed",
				"error":  "Instancia nao encontrada",
			})
			return nil // Don't retry
		}

		if isConnectionClosedError(sendErr) {
			w.logger.Warn("Connection closed, will retry",
				zap.String("automationID", data.AutomationID),
				zap.String("instanceName", data.InstanceName),
				zap.Error(sendErr))
			return sendErr // Retry
		}

		// Other errors: retry
		w.logger.Error("Failed to send message",
			zap.String("automationID", data.AutomationID),
			zap.String("instanceName", data.InstanceName),
			zap.String("groupId", data.GroupID),
			zap.Error(sendErr))

		w.updateDispatchLog(ctx, data.DispatchLogID, map[string]interface{}{
			"status": "failed",
			"error":  errStr,
		})

		return sendErr
	}

	// 5. Success
	w.logger.Info("Message sent successfully",
		zap.String("automationID", data.AutomationID),
		zap.String("instanceName", data.InstanceName),
		zap.String("groupId", data.GroupID))

	w.updateHistoryStatus(ctx, data.HistoryID, "completed")
	w.updateDispatchLog(ctx, data.DispatchLogID, map[string]interface{}{
		"status": "sent",
	})

	return nil
}

func (w *MessageWorker) updateHistoryStatus(ctx context.Context, historyID *string, status string) {
	if historyID == nil || w.historyRepo == nil {
		return
	}

	updates := map[string]interface{}{
		"status": status,
	}

	if status == "completed" || status == "failed" || status == "assumedSent" {
		updates["completed_at"] = time.Now().UTC().Format(time.RFC3339)
	}

	_ = w.historyRepo.Update(ctx, *historyID, updates)
}

func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "context deadline exceeded")
}

func is404Error(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "404") ||
		strings.Contains(errStr, "not found") ||
		strings.Contains(errStr, "instance not found")
}

func isConnectionClosedError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "connection closed") ||
		strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "eof")
}

// Ensure MessageWorker implements asynq.Handler
var _ asynq.Handler = (*MessageWorker)(nil)

// asynq.Handler interface implementation
func (w *MessageWorker) ProcessTaskWrapper() asynq.HandlerFunc {
	return func(ctx context.Context, task *asynq.Task) error {
		return w.ProcessTask(ctx, task)
	}
}

// SkipRetry is a sentinel error to skip retries
var SkipRetry = errors.New("skip retry")

// updateDispatchLog updates a dispatch log entry if the feature flag is enabled.
// This method respects the FEATURE_USE_GO_DISPATCH_LOGS flag.
func (w *MessageWorker) updateDispatchLog(ctx context.Context, dispatchLogID *int, updates map[string]interface{}) {
	if dispatchLogID == nil || w.dispatchLogRepo == nil {
		return
	}

	// Check feature flag: skip if Go should not manage dispatch logs
	featureFlags := config.GetFeatureFlags()
	if !featureFlags.ShouldCreateDispatchLog() {
		w.logger.Debug("Skipping dispatch log update (feature flag disabled)",
			zap.Int("dispatchLogID", *dispatchLogID))
		return
	}

	_ = w.dispatchLogRepo.Update(ctx, *dispatchLogID, updates)
}
