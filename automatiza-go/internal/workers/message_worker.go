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
	"automatiza-go/internal/lib/cache"
	"automatiza-go/internal/lib/humanize"
	"automatiza-go/internal/lib/selfsend"
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

// staleMessageTTL é o atraso máximo tolerado além do horário agendado de um
// envio. Passou disso, a mensagem é descartada (skipped) em vez de enviada:
// promoção velha saindo de madrugada é o sintoma clássico de backlog pós-queda
// e é percebida como spam/duplicata pelos grupos. 90min acomoda os maiores
// delays legítimos de pacing/chip_overrides com folga.
const staleMessageTTL = 90 * time.Minute

// MessageWorker processes message sending tasks.
// It uses the WhatsAppProvider interface (adapter pattern) for sending messages,
// decoupling the worker from specific WhatsApp API implementations.
type MessageWorker struct {
	whatsappProvider whatsapp.WhatsAppProvider
	urlSafetyService *urlsafety.Service
	historyRepo      repository.ActionHistoryRepository
	dispatchLogRepo  repository.DispatchLogRepository
	cache            *cache.Cache
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
	cacheClient *cache.Cache,
	logger *zap.Logger,
) *MessageWorker {
	return &MessageWorker{
		whatsappProvider: whatsappProvider,
		urlSafetyService: urlSafetyService,
		historyRepo:      historyRepo,
		dispatchLogRepo:  dispatchLogRepo,
		cache:            cacheClient,
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

	// 0a. Descarta mensagem VELHA: se o job só chegou a rodar muito depois do
	// horário agendado (backlog pós-apagão, retries acumulados), enviar agora é
	// pior que não enviar — a promoção sai fora de hora e, somada aos reenvios
	// de jobs em voo de restarts, é percebida como duplicada no grupo. Tasks
	// antigas (sem scheduledFor) passam direto.
	if !data.ScheduledFor.IsZero() && time.Since(data.ScheduledFor) > staleMessageTTL {
		w.logger.Warn("Discarding stale message job",
			zap.String("automationID", data.AutomationID),
			zap.String("groupId", data.GroupID),
			zap.Time("scheduledFor", data.ScheduledFor),
			zap.Duration("late", time.Since(data.ScheduledFor)))
		w.updateHistoryStatus(ctx, data.HistoryID, "skipped")
		w.updateDispatchLog(ctx, data.DispatchLogID, map[string]interface{}{
			"status": "skipped",
			"error":  fmt.Sprintf("descartada: atrasada %s alem do agendado (backlog)", time.Since(data.ScheduledFor).Round(time.Minute)),
		})
		return nil // Terminal: não reenviar promoção velha
	}

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

	// Marca o texto FINAL (pós-Vary) como envio da plataforma ANTES do send:
	// o webhook desse envio chega nos chips membros do grupo e, sem a marca,
	// um grupo-fonte de espelhamento re-espelharia esse disparo automatizado.
	selfsend.Mark(ctx, w.cache, data.GroupID, normalizedMessage)

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
				// O áudio JÁ foi entregue: retornar erro aqui faria o asynq
				// re-executar a task inteira e o grupo receberia o áudio 2x.
				// Perder o texto complementar é melhor que duplicar a mensagem.
				w.logger.Warn("Context done after voice sent; skipping follow-up text (no retry)",
					zap.String("automationID", data.AutomationID),
					zap.String("groupId", data.GroupID))
				w.updateHistoryStatus(ctx, data.HistoryID, "completed")
				w.updateDispatchLog(ctx, data.DispatchLogID, map[string]interface{}{
					"status": "sent",
					"error":  "texto complementar do audio nao enviado (shutdown/timeout)",
				})
				return nil
			}
			textInput := whatsapp.SendTextInput{
				SessionName: data.InstanceName,
				ChatID:      data.GroupID,
				Text:        normalizedMessage,
				Mentions:    data.Mentions,
			}
			// Falha do texto complementar é SEMPRE terminal: o áudio JÁ foi
			// entregue, e devolver o erro faria o asynq re-executar a task
			// inteira — o grupo receberia a mensagem de voz 2x (chip
			// momentaneamente desconectado / rate-limit no SendText bastava).
			// Perder o texto complementar é melhor que duplicar o áudio.
			if _, textErr := w.whatsappProvider.SendText(ctx, textInput); textErr != nil {
				w.logger.Warn("Follow-up text after voice failed; NOT retrying (voice already delivered)",
					zap.String("automationID", data.AutomationID),
					zap.String("groupId", data.GroupID),
					zap.Error(textErr))
				w.updateHistoryStatus(ctx, data.HistoryID, "completed")
				w.updateDispatchLog(ctx, data.DispatchLogID, map[string]interface{}{
					"status": "sent",
					"error":  "texto complementar do audio nao enviado (falha pos-voz)",
				})
				return nil
			}
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

		// Fallback: if image send fails DEFINITIVELY, try sending text-only.
		// Desfecho ambíguo (timeout/cancelamento) fica de fora — o WAHA pode ter
		// entregue imagem+legenda mesmo assim, e reenviar por texto duplica a
		// promoção no grupo (mesmo guard do ramo link-preview acima).
		if sendErr != nil && !whatsapp.IsAmbiguousOutcome(sendErr) {
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
				// Na ÚLTIMA tentativa o asynq arquiva sem nos chamar de novo:
				// sem o registro terminal aqui, o dispatch_log ficava 'queued'
				// para sempre e o envio perdido sumia das queries por usuário.
				w.recordFinalFailure(ctx, &data, "Sessao desconectada (retries esgotados)")
				return sendErr // Retry

			case whatsapp.ErrCodeRateLimited:
				w.logger.Warn("Rate limited, will retry with backoff",
					zap.String("automationID", data.AutomationID),
					zap.String("instanceName", data.InstanceName),
					zap.Error(sendErr))
				w.recordFinalFailure(ctx, &data, "Rate limit (retries esgotados)")
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

		// Desfecho AMBÍGUO não-tipado (timeout legado, EOF/broken pipe/
		// connection reset/closed que não veio como ProviderError): o WAHA
		// pode ter entregue a mensagem. Retentar ou marcar failed aqui é o
		// que duplica promoção — assume enviado e mantém o claim de dedup.
		// (Superset do antigo isTimeoutError; alinhado ao classificador R2
		// do adapter via IsAmbiguousOutcome.)
		if whatsapp.IsAmbiguousOutcome(sendErr) {
			w.logger.Warn("Ambiguous send outcome, assuming sent (no retry/fallback)",
				zap.String("automationID", data.AutomationID),
				zap.String("instanceName", data.InstanceName),
				zap.String("groupId", data.GroupID),
				zap.Error(sendErr))
			observability.CaptureMessage(sentry.LevelWarning,
				fmt.Sprintf("Ambiguous send outcome, assuming sent: group=%s", data.GroupID),
				map[string]string{"flow": "message-send", "reason": "ambiguous-assumed-sent"})
			w.updateHistoryStatus(ctx, data.HistoryID, "assumedSent")
			w.updateDispatchLog(ctx, data.DispatchLogID, map[string]interface{}{
				"status": "sent",
				"error":  "Desfecho incerto (timeout/conexao) - assumido como enviado",
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

		if isConnectionRefusedError(sendErr) {
			// connection refused = o TCP nem conectou; o request comprovadamente
			// NÃO chegou ao WAHA — retry é seguro (não duplica). EOF/"connection
			// closed" NÃO entram aqui: podem ocorrer após o POST ser processado
			// e são tratados como ambíguos no gate acima.
			w.logger.Warn("Connection refused, will retry",
				zap.String("automationID", data.AutomationID),
				zap.String("instanceName", data.InstanceName),
				zap.Error(sendErr))
			w.recordFinalFailure(ctx, &data, "WAHA inacessivel (retries esgotados)")
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

		// Other errors: retry.
		// NÃO grava 'failed' em tentativa intermediária: com retry ainda vivo
		// no asynq, um 'failed' precoce fazia o reconciliador de lista ver
		// "0 queued / ≥1 failed", devolver o item e liberar o claim — e o
		// retry seguinte podia entregar → item devolvido + entrega = DUPLICATA.
		// O status fica 'queued' (em voo) até o desfecho real: 'sent' no
		// sucesso, 'failed' só na ÚLTIMA tentativa (recordFinalFailure).
		w.logger.Error("Failed to send message",
			zap.String("automationID", data.AutomationID),
			zap.String("instanceName", data.InstanceName),
			zap.String("groupId", data.GroupID),
			zap.Error(sendErr))

		w.recordFinalFailure(ctx, &data, errStr)

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

	// Mesmo racional do updateDispatchLog: o desfecho precisa ser registrado
	// mesmo com o ctx da task já cancelado/estourado.
	updCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := w.historyRepo.Update(updCtx, *historyID, updates); err != nil {
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

// isConnectionRefusedError detecta falha de CONEXÃO (TCP recusado): o request
// nunca chegou ao servidor, então retentar é seguro. Não confundir com
// EOF/"connection closed" — esses podem ocorrer DEPOIS de o WAHA processar o
// POST e são classificados como desfecho ambíguo (IsAmbiguousOutcome).
func isConnectionRefusedError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "connection refused")
}

// isFinalAttempt informa se esta execução é a ÚLTIMA tentativa da task — o
// asynq arquiva depois dela sem nos chamar de novo, então qualquer erro
// retryável precisa TAMBÉM registrar o desfecho terminal no dispatch_log.
func isFinalAttempt(ctx context.Context) bool {
	retryCount, _ := asynq.GetRetryCount(ctx)
	maxRetry, _ := asynq.GetMaxRetry(ctx)
	return retryCount >= maxRetry
}

// recordFinalFailure grava 'failed' no histórico e no dispatch_log quando a
// tentativa atual é a última. Sem isso, envios perdidos por erro retryável
// (chip caído, rate limit, WAHA fora) ficavam 'queued' para sempre —
// invisíveis nas queries por usuário e para o reconciliador de lista.
func (w *MessageWorker) recordFinalFailure(ctx context.Context, data *MessageJobData, reason string) {
	if !isFinalAttempt(ctx) {
		return
	}
	w.logger.Error("Send failed on final attempt — recording terminal failure",
		zap.String("automationID", data.AutomationID),
		zap.String("instanceName", data.InstanceName),
		zap.String("groupId", data.GroupID),
		zap.String("reason", reason))
	w.updateHistoryStatus(ctx, data.HistoryID, "failed")
	w.updateDispatchLog(ctx, data.DispatchLogID, map[string]interface{}{
		"status": "failed",
		"error":  reason,
	})
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

	// WithoutCancel + timeout próprio: o update final chega aqui com o ctx da
	// task frequentemente já estourado (envio consumiu os 120s) — era a causa
	// dos dispatch_logs presos em 'queued' para sempre ("Failed to update
	// dispatch log: context deadline exceeded"). O registro do desfecho DEVE
	// completar mesmo com a task morta.
	updCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := w.dispatchLogRepo.Update(updCtx, *dispatchLogID, updates); err != nil {
		observability.LogAndCapture(w.logger, sentry.LevelError,
			"Failed to update dispatch log", err,
			map[string]string{"module": "message-worker", "operation": "dispatch-log-update"})
	}
}
