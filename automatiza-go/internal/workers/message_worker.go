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
	"automatiza-go/internal/lib/humanize"
	"automatiza-go/internal/lib/text"
	"automatiza-go/internal/lib/urlsafety"
	"automatiza-go/internal/observability"
	"automatiza-go/internal/repository"

	"github.com/getsentry/sentry-go"
	"github.com/hibiken/asynq"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
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
	deadGroups       *deadGroupCache
}

// NewMessageWorker creates a new message worker.
// IMPORTANT: This now requires a WhatsAppProvider instead of waha.Service.
// The anti-ban runtime (humanizer + reputation store) is read process-wide via
// config.GetAntiBanRuntime() inside ProcessTask, so it isn't threaded here.
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
		deadGroups:       newDeadGroupCache(),
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

	// 0. Skip grupos onde a sessão sabidamente não participa mais.
	// Quando o bot é removido de um grupo, o WAHA responde 500 "you're not
	// participating in that group" a cada envio — e como a automação re-mira o
	// mesmo grupo a cada ciclo (e por produto), isso vira um storm de 500 +
	// fallback texto + retries do asynq. O primeiro envio que falhar marca o
	// grupo nesse cache (TTL curto); os seguintes saem aqui sem tocar o WAHA.
	if w.deadGroups.isDead(data.InstanceName, data.GroupID, time.Now()) {
		w.logger.Warn("Skipping dispatch — session not participating in group (cached)",
			zap.String("automationID", data.AutomationID),
			zap.String("instanceName", data.InstanceName),
			zap.String("groupId", data.GroupID))
		w.updateDispatchLog(ctx, data.DispatchLogID, map[string]interface{}{
			"status": "skipped",
			"error":  "bot nao participa do grupo",
		})
		return nil // Terminal: não adianta enviar nem retentar
	}

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

	// 2. Text Normalization + per-recipient variation (anti-ban).
	// Resolve the automation's humanize preset once (default "high") and grab the
	// process-wide anti-ban runtime (humanizer + reputation store; both nil-safe).
	resolved := config.GetHumanization().ForMode(data.HumanizeMode)
	rt := config.GetAntiBanRuntime()
	normalizedMessage := text.NormalizeOutgoingWhatsAppText(data.Message)
	// Vary runs AFTER normalize (normalize strips zero-width chars) and is seeded
	// by the group JID so each group gets byte-distinct text while an asynq retry
	// re-sends identical bytes. URLs are never mutated (masked inside Vary).
	normalizedMessage = text.Vary(normalizedMessage, resolved.Vary, data.GroupID)
	if text.IsEffectivelyEmptyWhatsAppText(normalizedMessage) {
		w.logger.Info("Message empty after normalization, skipping",
			zap.String("automationID", data.AutomationID))
		w.updateDispatchLog(ctx, data.DispatchLogID, map[string]interface{}{
			"status": "skipped",
			"error":  "Mensagem vazia apos normalizacao",
		})
		return nil // Empty message after normalization
	}

	// 2b. Human-presence choreography (typing / online / seen) before the real
	// send. Fail-open and nil-safe; the returned cleanup stops the typing
	// indicator. defer so it always clears even on an early error path.
	hasMedia := (data.ImageURL != nil && *data.ImageURL != "") ||
		(data.ImageBase64 != nil && *data.ImageBase64 != "") ||
		(data.AudioBase64 != nil && *data.AudioBase64 != "")
	stopTyping := rt.Humanizer.BeforeSend(ctx, humanize.Plan{
		Session:  data.InstanceName,
		ChatID:   data.GroupID,
		Text:     normalizedMessage,
		HasMedia: hasMedia,
		Cfg:      resolved.Humanize,
	})
	defer stopTyping()

	// 3. Send Message via WhatsAppProvider
	var sendErr error
	if data.LinkPreview != nil && data.LinkPreview.URL != "" {
		// Send with custom link preview
		w.logger.Info("Sending link custom preview",
			zap.String("automationID", data.AutomationID),
			zap.String("groupId", data.GroupID),
			zap.String("previewURL", data.LinkPreview.URL))

		previewInput := whatsapp.SendLinkPreviewInput{
			SessionName: data.InstanceName,
			ChatID:      data.GroupID,
			Text:        normalizedMessage,
			URL:         data.LinkPreview.URL,
			Title:       data.LinkPreview.Title,
			Description: data.LinkPreview.Description,
			ImageURL:    data.LinkPreview.ImageURL,
		}
		_, sendErr = w.whatsappProvider.SendLinkCustomPreview(ctx, previewInput)

		// Fallback: if link preview fails DEFINITIVELY, try sending as text.
		// Timeout/cancelamento fica de fora: o WAHA pode ter entregue o card
		// mesmo assim, e reenviar por texto duplica a promoção no grupo (o
		// mesmo bug corrigido no monitoring/offer_generate).
		if sendErr != nil && !whatsapp.IsAmbiguousOutcome(sendErr) {
			w.logger.Warn("Link preview send failed, falling back to text",
				zap.String("automationID", data.AutomationID),
				zap.String("previewURL", data.LinkPreview.URL),
				zap.Error(sendErr))
			textInput := whatsapp.SendTextInput{
				SessionName: data.InstanceName,
				ChatID:      data.GroupID,
				Text:        normalizedMessage,
				Mentions:    data.Mentions,
			}
			_, sendErr = w.whatsappProvider.SendText(ctx, textInput)
		}
	} else if data.AudioBase64 != nil && *data.AudioBase64 != "" {
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
			// Wait a bit then send text (context-aware)
			select {
			case <-time.After(1500 * time.Millisecond):
			case <-ctx.Done():
				return ctx.Err()
			}
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

		// Image aspect-normalization happens centrally in the WhatsApp adapter's
		// SendImage (covers every send path), so here we just pass the URL.
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
				observability.CaptureMessage(sentry.LevelWarning,
					fmt.Sprintf("Message send timeout (provider), assuming sent: group=%s", data.GroupID),
					map[string]string{"flow": "message-send", "reason": "timeout-assumed-sent"})
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
			observability.CaptureMessage(sentry.LevelWarning,
				fmt.Sprintf("Message send timeout (legacy), assuming sent: group=%s", data.GroupID),
				map[string]string{"flow": "message-send", "reason": "timeout-assumed-sent"})
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

		// Bot removido do grupo: WAHA responde 500 "you're not participating in
		// that group". Não adianta retentar (nem o fallback texto resolve), e
		// re-mirar o grupo todo ciclo só gera ruido. Marca no cache para os
		// próximos envios saírem cedo (passo 0) e encerra sem retry.
		if isNotParticipatingError(sendErr) {
			w.deadGroups.markDead(data.InstanceName, data.GroupID, time.Now())
			// Being removed from a group is the strongest block/report proxy —
			// weight it heavily so the dispatchers suppress this group. Fail-open.
			rt.Reputation.Penalize(ctx, data.InstanceName, data.GroupID, 3)
			w.logger.Warn("Session not participating in group, skipping (no retry)",
				zap.String("automationID", data.AutomationID),
				zap.String("instanceName", data.InstanceName),
				zap.String("groupId", data.GroupID))
			w.updateDispatchLog(ctx, data.DispatchLogID, map[string]interface{}{
				"status": "skipped",
				"error":  "bot nao participa do grupo",
			})
			return nil // Terminal
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
	// A successful send re-establishes the group's standing so a recovered group
	// returns to rotation. Fail-open / nil-safe.
	rt.Reputation.Clear(ctx, data.InstanceName, data.GroupID)
	w.logger.Info("Message sent successfully",
		zap.String("automationID", data.AutomationID),
		zap.String("instanceName", data.InstanceName),
		zap.String("groupId", data.GroupID))

	if observability.GlobalMetrics != nil {
		observability.GlobalMetrics.MessagesDispatched.Add(ctx, 1,
			metric.WithAttributes(attribute.String("instance", data.InstanceName)))
	}

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

	if err := w.historyRepo.Update(ctx, *historyID, updates); err != nil {
		observability.LogAndCapture(w.logger, sentry.LevelError,
			"Failed to update history status", err,
			map[string]string{"module": "message-worker", "operation": "history-status-update"})
	}
}

func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "context deadline exceeded")
}

// isNotParticipatingError detecta a resposta do WAHA/GOWS quando a sessão não é
// mais membro do grupo de destino ("you're not participating in that
// group" / "failed to get group members"). Casa no texto do erro porque vem
// embrulhado como erro genérico (WAHA error 500), não como ProviderError.
func isNotParticipatingError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "not participating in that group") ||
		strings.Contains(errStr, "failed to get group members")
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

	if err := w.dispatchLogRepo.Update(ctx, *dispatchLogID, updates); err != nil {
		observability.LogAndCapture(w.logger, sentry.LevelError,
			"Failed to update dispatch log", err,
			map[string]string{"module": "message-worker", "operation": "dispatch-log-update"})
	}
}
