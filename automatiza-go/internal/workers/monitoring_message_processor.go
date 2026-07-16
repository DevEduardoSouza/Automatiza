// Package workers provides background job processing.
// This file implements the MonitoringMessageProcessor that integrates
// the webhook consumer with the monitoring service for MONITOR mode automations.
//
// Flow:
//  1. WebhookConsumerWorker receives a message event from Asynq queue
//  2. Routes to MonitoringMessageProcessor.ProcessMessage
//  3. Processor parses WAHA payload into IncomingMessage
//  4. Calls MonitoringService.ProcessIncomingMessage
//  5. Monitoring service finds matching automations and forwards messages
package workers

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"automatiza-go/internal/models"
	"automatiza-go/internal/services/monitoring"
	"automatiza-go/internal/services/offer_generate"

	"go.uber.org/zap"
)

// MessageCheckpointRecorder registra o watermark de mensagens processadas por
// (sessão, chat). Implementado pelo reconciliation.Service (RecordMessage);
// chamado DEPOIS do fan-out concluir, para que um crash no meio deixe o
// checkpoint para trás e o backfill recupere a mensagem.
type MessageCheckpointRecorder interface {
	RecordMessage(sessionName, chatID string, ts time.Time, messageID string)
}

// MonitoringMessageProcessor processes incoming WhatsApp messages for MONITOR mode automations.
// It implements the MessageWebhookProcessor interface.
//
// Architecture:
//   - Receives message events from WebhookConsumerWorker
//   - Parses WAHA webhook payload into IncomingMessage model
//   - Delegates to MonitoringService for business logic
//   - Processing is idempotent (monitoring service handles deduplication)
type MonitoringMessageProcessor struct {
	monitoringService    monitoring.Service
	offerGenerateService offer_generate.Service
	logger               *zap.Logger

	// fanoutSlots limita os fan-outs de espelho processados em goroutine.
	// O fan-out leva dezenas de segundos (pacing + digitação) e não pode
	// segurar o slot do consumidor asynq — ver ProcessMessage.
	fanoutSlots chan struct{}

	// fanoutWG rastreia as goroutines de fan-out em voo para o Drain no
	// shutdown: sem isso, cada deploy matava até 64 espelhos no meio do envio
	// com o evento já 'processed' e o slot de dedup já reivindicado — os
	// grupos restantes nunca recebiam a oferta.
	fanoutWG sync.WaitGroup

	// chatLocks serializa o processamento POR GRUPO-FONTE (RemoteJID): dois
	// webhooks do mesmo grupo processados concorrentemente espelhavam fora
	// de ordem ("parte 2" antes da "parte 1") e tornavam o avanço do
	// checkpoint não-monotônico. Grupos distintos seguem paralelos.
	chatLocks sync.Map // RemoteJID -> *sync.Mutex

	// checkpointRecorder avança o watermark vivo (sessão, chat) após o
	// fan-out concluir. Opcional (nil = desligado).
	checkpointRecorder MessageCheckpointRecorder

	// syncForTests força processamento inline (sem goroutine) para os
	// testes poderem assertar deterministicamente logo após ProcessMessage.
	syncForTests bool
}

// SetCheckpointRecorder instala o gravador de watermark (reconciliation
// service). Chamar uma vez no wiring, antes de o consumidor começar a servir.
func (p *MonitoringMessageProcessor) SetCheckpointRecorder(r MessageCheckpointRecorder) {
	p.checkpointRecorder = r
}

// lockChat devolve o mutex do grupo-fonte, criando-o na primeira mensagem.
func (p *MonitoringMessageProcessor) lockChat(remoteJID string) *sync.Mutex {
	v, _ := p.chatLocks.LoadOrStore(remoteJID, &sync.Mutex{})
	return v.(*sync.Mutex)
}

// Drain espera os fan-outs em voo terminarem, até o timeout. Chamar após o
// shutdown do servidor asynq (nenhum ProcessMessage novo chega) e antes do
// processo sair. Retorna true se drenou tudo dentro do prazo.
func (p *MonitoringMessageProcessor) Drain(timeout time.Duration) bool {
	done := make(chan struct{})
	go func() {
		p.fanoutWG.Wait()
		close(done)
	}()
	select {
	case <-done:
		return true
	case <-time.After(timeout):
		p.logger.Warn("Drain timeout: mirror fan-outs still in flight at shutdown",
			zap.Duration("timeout", timeout))
		return false
	}
}

// DisableAsyncFanoutForTests força o processamento inline. Só para testes.
func (p *MonitoringMessageProcessor) DisableAsyncFanoutForTests() {
	p.syncForTests = true
}

// NewMonitoringMessageProcessor creates a new monitoring message processor.
//
// Parameters:
//   - monitoringService: The monitoring service for processing messages
//   - offerGenerateService: The offer generate service for processing offer links (can be nil)
//   - logger: Structured logger for logging
func NewMonitoringMessageProcessor(
	monitoringService monitoring.Service,
	offerGenerateService offer_generate.Service,
	logger *zap.Logger,
) *MonitoringMessageProcessor {
	return &MonitoringMessageProcessor{
		monitoringService:    monitoringService,
		offerGenerateService: offerGenerateService,
		logger:               logger.Named("monitoring-processor"),
		fanoutSlots:          make(chan struct{}, 64),
	}
}

// ProcessMessage processes an incoming WhatsApp message.
// This implements the MessageWebhookProcessor interface.
//
// The method:
//  1. Parses the WAHA webhook payload into an IncomingMessage
//  2. Validates the message (must be from a group, not from self)
//  3. Calls the monitoring service to process the message
//
// Processing is idempotent - the monitoring service handles deduplication
// via content fingerprinting and message ID checks.
//
// Parameters:
//   - ctx: Context for cancellation and timeouts
//   - sessionName: The WAHA session identifier (instance name)
//   - payload: The raw webhook payload from WAHA
//
// Returns:
//   - nil if successfully processed or skipped (idempotent)
//   - error if processing failed and should be retried
func (p *MonitoringMessageProcessor) ProcessMessage(
	ctx context.Context,
	sessionName string,
	payload map[string]interface{},
) error {
	// Parse the payload into an IncomingMessage
	msg, err := p.parsePayload(sessionName, payload)
	if err != nil {
		p.logger.Debug("Failed to parse message payload",
			zap.String("session", sessionName),
			zap.Error(err))
		// Non-retryable error - malformed payload
		return nil
	}

	// Skip if not a group message
	if !strings.HasSuffix(msg.RemoteJID, "@g.us") {
		p.logger.Debug("Skipping non-group message",
			zap.String("session", sessionName),
			zap.String("remoteJID", msg.RemoteJID))
		return nil
	}

	// Skip empty messages (nothing for either service to act on)
	if msg.Body == "" && !msg.HasMedia {
		p.logger.Debug("Skipping empty message",
			zap.String("session", sessionName),
			zap.String("messageID", msg.MessageID))
		return nil
	}

	p.logger.Info("Processing message",
		zap.String("session", sessionName),
		zap.String("messageID", msg.MessageID),
		zap.String("group", msg.RemoteJID),
		zap.Bool("hasMedia", msg.HasMedia),
		zap.Bool("fromMe", msg.FromMe),
		zap.Int("bodyLen", len(msg.Body)))

	// Own (FromMe) messages must NOT go to monitoring — that mode never copies
	// the chip's own posts. But they MUST still reach offer_generate: a chip
	// owner manually posting a product/offer into a monitor group (or a monitor
	// group that has only the owner's chip in it, like a private staging group)
	// is a legitimate trigger. offer_generate has its own loop-guard
	// (og_self_send, set on every send) to ignore its own relayed output, so it
	// can safely receive FromMe messages. A blanket early-return here used to
	// drop them before offer_generate ever saw them — meaning users who fed
	// offers by posting themselves got nothing relayed.
	// fanout executa o espelhamento. PRESSUPÕE o lock do chat (lockChat) já
	// adquirido pelo chamador — a serialização por grupo-fonte (anti
	// out-of-order + checkpoint monotônico) é responsabilidade de quem chama.
	fanout := func(procCtx context.Context) {
		// Re-checagem de frescor PÓS-lock: num burst do mesmo grupo, uma
		// mensagem pode esperar minutos na fila do mutex — espelhar depois
		// disso é oferta velha (mesma regra do gate de entrada, que rodou
		// ANTES da espera). Também drena rápido a fila de waiters de um
		// grupo quente: cada um acorda, vê que envelheceu e retorna.
		// Timestamp zero/agora (payload sem ts) passa.
		if ts := msg.GetTimestamp(); !ts.IsZero() && time.Since(ts) > staleWebhookMessageTTL {
			p.logger.Warn("Discarding message gone stale while waiting for chat lock",
				zap.String("session", sessionName),
				zap.String("group", msg.RemoteJID),
				zap.String("messageID", msg.MessageID),
				zap.Duration("age", time.Since(ts)))
			return
		}

		if !msg.FromMe {
			if err := p.monitoringService.ProcessIncomingMessage(procCtx, msg); err != nil {
				p.logger.Error("Failed to process message for monitoring",
					zap.String("session", sessionName),
					zap.String("messageID", msg.MessageID),
					zap.Error(err))
			}
		}
		// Also process through the offer generate service (filters internally by
		// automation type, and handles FromMe via its own loop-guard)
		if p.offerGenerateService != nil {
			if err := p.offerGenerateService.ProcessIncomingMessage(procCtx, msg); err != nil {
				p.logger.Error("Failed to process message for offer generation",
					zap.String("session", sessionName),
					zap.String("messageID", msg.MessageID),
					zap.Error(err))
			}
		}

		// Watermark vivo SÓ após o fan-out concluir: um hard-kill no meio
		// deixa o checkpoint para trás → o gap fica visível e o backfill
		// re-busca a mensagem (dedup por destino absorve o overlap). Este é
		// o mecanismo que torna o crash de fan-out RECUPERÁVEL em vez de
		// perda silenciosa.
		if p.checkpointRecorder != nil {
			p.checkpointRecorder.RecordMessage(sessionName, msg.RemoteJID, msg.GetTimestamp(), msg.MessageID)
		}
	}

	// O fan-out do espelho leva dezenas de segundos (pacing + digitação) e
	// NÃO pode segurar o slot do consumidor asynq: sob rajada (jobs de lista
	// drenando na mesma piscina), a fila de webhooks acumulava minutos e o
	// espelho estourava o teto de 1min ponta a ponta. O consumidor agora só
	// parseia e DELEGA: goroutine com semáforo (64) e ctx PRÓPRIO (o da task
	// morre no retorno). Pool cheio → processa inline (backpressure real,
	// mensagem nunca é descartada). Trade-off deliberado: sem retry do asynq
	// para erro de espelho — oferta espelhada perde valor em minutos e o
	// dedup/claim já torna retries quase sempre no-op.
	if p.syncForTests {
		lock := p.lockChat(msg.RemoteJID)
		lock.Lock()
		fanout(ctx)
		lock.Unlock()
		p.logger.Debug("Message processed successfully (sync test mode)",
			zap.String("session", sessionName),
			zap.String("messageID", msg.MessageID))
		return nil
	}

	select {
	case p.fanoutSlots <- struct{}{}:
		p.fanoutWG.Add(1)
		go func() {
			defer func() {
				<-p.fanoutSlots
				p.fanoutWG.Done()
				if r := recover(); r != nil {
					p.logger.Error("Panic in async mirror fan-out",
						zap.String("messageID", msg.MessageID),
						zap.Any("panic", r))
				}
			}()
			procCtx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			defer cancel()
			// A espera do lock acontece DENTRO da goroutine (slot ocupado
			// enquanto espera) — aceitável porque a fila de waiters de um
			// grupo quente drena rápido: o gate de frescor pós-lock (em
			// fanout) descarta quem envelheceu esperando.
			lock := p.lockChat(msg.RemoteJID)
			lock.Lock()
			defer lock.Unlock()
			fanout(procCtx)
		}()
	default:
		// Pool cheio: processa inline (backpressure real) — mas NUNCA
		// bloqueia o handler asynq esperando o lock de um grupo ocupado.
		// Bloquear aqui, sob burst de UM grupo, segurava slots do consumidor
		// por até dezenas de minutos (cada retry do asynq empilhava mais um
		// handler no mesmo mutex) e travava a fila de webhooks inteira.
		// TryLock: se o grupo está ocupado, devolve erro RETRYÁVEL — o asynq
		// re-tenta com backoff e os gates de frescor descartam se envelhecer.
		lock := p.lockChat(msg.RemoteJID)
		if !lock.TryLock() {
			return fmt.Errorf("mirror fan-out busy for chat %s (backpressure, temporary — will retry)", msg.RemoteJID)
		}
		fanout(ctx)
		lock.Unlock()
	}

	p.logger.Debug("Message processed successfully",
		zap.String("session", sessionName),
		zap.String("messageID", msg.MessageID))

	return nil
}

// parsePayload parses a WAHA webhook payload into an IncomingMessage.
// Supports both flat payload format and nested message format from WAHA.
//
// WAHA webhook payload formats:
//
// Format 1 (message.any event):
//
//	{
//	  "id": "message_id",
//	  "from": "5511999999999@c.us",
//	  "to": "120363xxxxx@g.us",
//	  "body": "message text",
//	  "fromMe": false,
//	  "timestamp": 1234567890,
//	  "hasMedia": false
//	}
//
// Format 2 (nested message):
//
//	{
//	  "message": {
//	    "id": {...},
//	    "body": "message text",
//	    ...
//	  }
//	}
func (p *MonitoringMessageProcessor) parsePayload(
	sessionName string,
	payload map[string]interface{},
) (*models.IncomingMessage, error) {
	msg := &models.IncomingMessage{
		InstanceName: sessionName,
		Timestamp:    time.Now().Unix(), // Default to now if not provided
	}

	// Check if payload has nested "message" object
	if msgObj, ok := payload["message"].(map[string]interface{}); ok {
		return p.parseMessageObject(sessionName, msgObj)
	}

	// Parse flat payload format
	// Message ID
	if id, ok := extractStringFromPayload(payload, "id"); ok {
		msg.MessageID = id
	} else if idObj, ok := payload["id"].(map[string]interface{}); ok {
		// WAHA sometimes sends ID as object: {"id": {"_serialized": "..."}}
		if serialized, ok := extractStringFromPayload(idObj, "_serialized"); ok {
			msg.MessageID = serialized
		}
	}

	// Remote JID (group or contact)
	if from, ok := extractStringFromPayload(payload, "from"); ok {
		msg.RemoteJID = from
	} else if to, ok := extractStringFromPayload(payload, "to"); ok {
		msg.RemoteJID = to
	} else if chatID, ok := extractStringFromPayload(payload, "chatId"); ok {
		msg.RemoteJID = chatID
	}

	// Sender JID (for group messages)
	if participant, ok := extractStringFromPayload(payload, "participant"); ok {
		msg.SenderJID = participant
	} else if author, ok := extractStringFromPayload(payload, "author"); ok {
		msg.SenderJID = author
	}

	// Push name
	if pushName, ok := extractStringFromPayload(payload, "pushName"); ok {
		msg.PushName = pushName
	} else if notifyName, ok := extractStringFromPayload(payload, "notifyName"); ok {
		msg.PushName = notifyName
	}

	// Message body
	if body, ok := extractStringFromPayload(payload, "body"); ok {
		msg.Body = body
	} else if text, ok := extractStringFromPayload(payload, "text"); ok {
		msg.Body = text
	}

	// FromMe flag
	if fromMe, ok := extractBoolFromPayload(payload, "fromMe"); ok {
		msg.FromMe = fromMe
	}

	// Timestamp
	if ts, ok := extractInt64FromPayload(payload, "timestamp"); ok {
		msg.Timestamp = ts
	} else if t, ok := extractInt64FromPayload(payload, "t"); ok {
		msg.Timestamp = t
	}

	// Media fields
	if hasMedia, ok := extractBoolFromPayload(payload, "hasMedia"); ok {
		msg.HasMedia = hasMedia
	}

	if mediaType, ok := extractStringFromPayload(payload, "mediaType"); ok {
		msg.MediaType = mediaType
	} else if msgType, ok := extractStringFromPayload(payload, "type"); ok {
		// Map WAHA message types to media types
		switch msgType {
		case "image":
			msg.MediaType = "image"
			msg.HasMedia = true
		case "video":
			msg.MediaType = "video"
			msg.HasMedia = true
		case "audio", "ptt":
			msg.MediaType = "audio"
			msg.HasMedia = true
		case "document":
			msg.MediaType = "document"
			msg.HasMedia = true
		}
	}

	if mediaURL, ok := extractStringFromPayload(payload, "mediaUrl"); ok {
		msg.MediaURL = mediaURL
	}
	// WAHA GoWS sends media as nested object: media.url (preferred over deprecated mediaUrl)
	if mediaObj, ok := payload["media"].(map[string]interface{}); ok {
		if mediaObjURL, ok := extractStringFromPayload(mediaObj, "url"); ok && mediaObjURL != "" {
			msg.MediaURL = mediaObjURL
		}
		if mediaMime, ok := extractStringFromPayload(mediaObj, "mimetype"); ok && mediaMime != "" {
			msg.MimeType = mediaMime
		}
	}

	if mimeType, ok := extractStringFromPayload(payload, "mimetype"); ok {
		msg.MimeType = mimeType
	} else if mimeType, ok := extractStringFromPayload(payload, "mimeType"); ok {
		msg.MimeType = mimeType
	}

	// Quando o WAHA (GOWS) entrega a mídia só como hasMedia + media.url/mimetype,
	// sem o campo mediaType nem um type mapeável, inferimos o tipo do mime. Sem
	// isso o relay do offer_generate cai no default e manda só o texto, perdendo
	// a imagem. Ver inferMediaTypeFromMime.
	if msg.HasMedia && msg.MediaType == "" && msg.MimeType != "" {
		msg.MediaType = inferMediaTypeFromMime(msg.MimeType)
	}

	// Parse quoted message if present
	if quotedMsg, ok := payload["quotedMsg"].(map[string]interface{}); ok {
		msg.QuotedMsg = p.parseQuotedMessage(quotedMsg)
	}

	// Extract link preview data from extendedTextMessage (Shopee/ML links with thumbnail)
	p.enrichWithLinkPreview(msg, payload)

	// Validate required fields
	if msg.MessageID == "" {
		return nil, fmt.Errorf("message ID not found in payload")
	}
	if msg.RemoteJID == "" {
		return nil, fmt.Errorf("remote JID not found in payload")
	}

	return msg, nil
}

// parseMessageObject parses a nested message object from WAHA payload.
func (p *MonitoringMessageProcessor) parseMessageObject(
	sessionName string,
	msgObj map[string]interface{},
) (*models.IncomingMessage, error) {
	msg := &models.IncomingMessage{
		InstanceName: sessionName,
		Timestamp:    time.Now().Unix(),
	}

	// Message ID - can be string or object
	if idObj, ok := msgObj["id"].(map[string]interface{}); ok {
		if serialized, ok := extractStringFromPayload(idObj, "_serialized"); ok {
			msg.MessageID = serialized
		}
		if fromMe, ok := extractBoolFromPayload(idObj, "fromMe"); ok {
			msg.FromMe = fromMe
		}
		if remote, ok := extractStringFromPayload(idObj, "remote"); ok {
			msg.RemoteJID = remote
		}
		if participant, ok := extractStringFromPayload(idObj, "participant"); ok {
			msg.SenderJID = participant
		}
	} else if id, ok := extractStringFromPayload(msgObj, "id"); ok {
		msg.MessageID = id
	}

	// Fallback for remote JID
	if msg.RemoteJID == "" {
		if from, ok := extractStringFromPayload(msgObj, "from"); ok {
			msg.RemoteJID = from
		} else if to, ok := extractStringFromPayload(msgObj, "to"); ok {
			msg.RemoteJID = to
		}
	}

	// Message body
	if body, ok := extractStringFromPayload(msgObj, "body"); ok {
		msg.Body = body
	}

	// FromMe override from message object
	if fromMe, ok := extractBoolFromPayload(msgObj, "fromMe"); ok {
		msg.FromMe = fromMe
	}

	// Timestamp
	if ts, ok := extractInt64FromPayload(msgObj, "timestamp"); ok {
		msg.Timestamp = ts
	} else if t, ok := extractInt64FromPayload(msgObj, "t"); ok {
		msg.Timestamp = t
	}

	// Media
	if hasMedia, ok := extractBoolFromPayload(msgObj, "hasMedia"); ok {
		msg.HasMedia = hasMedia
	}
	if mediaType, ok := extractStringFromPayload(msgObj, "type"); ok {
		switch mediaType {
		case "image":
			msg.MediaType = "image"
			msg.HasMedia = true
		case "video":
			msg.MediaType = "video"
			msg.HasMedia = true
		case "audio", "ptt":
			msg.MediaType = "audio"
			msg.HasMedia = true
		case "document":
			msg.MediaType = "document"
			msg.HasMedia = true
		}
	}

	// Media URL from nested media object
	if mediaObj, ok := msgObj["media"].(map[string]interface{}); ok {
		if mediaObjURL, ok := extractStringFromPayload(mediaObj, "url"); ok && mediaObjURL != "" {
			msg.MediaURL = mediaObjURL
		}
		if mediaMime, ok := extractStringFromPayload(mediaObj, "mimetype"); ok && mediaMime != "" {
			msg.MimeType = mediaMime
		}
	}

	// Infere o MediaType do mime quando o webhook trouxe a mídia sem type
	// explícito (mesmo motivo do parsePayload).
	if msg.HasMedia && msg.MediaType == "" && msg.MimeType != "" {
		msg.MediaType = inferMediaTypeFromMime(msg.MimeType)
	}

	// Push name
	if pushName, ok := extractStringFromPayload(msgObj, "notifyName"); ok {
		msg.PushName = pushName
	}

	// Extract link preview from extendedTextMessage inside nested message object
	p.enrichWithLinkPreview(msg, msgObj)

	// Validate
	if msg.MessageID == "" {
		return nil, fmt.Errorf("message ID not found in message object")
	}
	if msg.RemoteJID == "" {
		return nil, fmt.Errorf("remote JID not found in message object")
	}

	return msg, nil
}

// parseQuotedMessage parses a quoted message from the payload.
func (p *MonitoringMessageProcessor) parseQuotedMessage(
	quotedMsg map[string]interface{},
) *models.QuotedMessage {
	quoted := &models.QuotedMessage{}

	if id, ok := extractStringFromPayload(quotedMsg, "id"); ok {
		quoted.MessageID = id
	} else if idObj, ok := quotedMsg["id"].(map[string]interface{}); ok {
		if serialized, ok := extractStringFromPayload(idObj, "_serialized"); ok {
			quoted.MessageID = serialized
		}
	}

	if body, ok := extractStringFromPayload(quotedMsg, "body"); ok {
		quoted.Body = body
	}

	if sender, ok := extractStringFromPayload(quotedMsg, "participant"); ok {
		quoted.SenderJID = sender
	} else if sender, ok := extractStringFromPayload(quotedMsg, "author"); ok {
		quoted.SenderJID = sender
	}

	return quoted
}

// enrichWithLinkPreview extracts link preview data (description, title, thumbnail)
// from WAHA extendedTextMessage payloads and enriches the IncomingMessage.
//
// When a user sends a Shopee/ML link, WhatsApp generates a link preview with
// product description, title, and a JPEG thumbnail. This data arrives in the
// extendedTextMessage structure. Without extraction, only the bare URL is forwarded.
func (p *MonitoringMessageProcessor) enrichWithLinkPreview(msg *models.IncomingMessage, payload map[string]interface{}) {
	description, title, thumbnail := extractLinkPreviewData(payload)

	if description == "" && title == "" && thumbnail == "" {
		return
	}

	p.logger.Debug("Link preview data extracted",
		zap.String("messageID", msg.MessageID),
		zap.Bool("hasDescription", description != ""),
		zap.Bool("hasTitle", title != ""),
		zap.Bool("hasThumbnail", thumbnail != ""))

	// Enrich body if it's just a URL and we have description/title
	if (description != "" || title != "") && isPlainURL(msg.Body) {
		var parts []string
		if title != "" {
			parts = append(parts, "*"+title+"*")
		}
		if description != "" {
			parts = append(parts, description)
		}
		parts = append(parts, msg.Body)
		msg.Body = strings.Join(parts, "\n\n")
	}

	// Propagate title and description for link custom preview in monitoring service
	if title != "" {
		msg.LinkPreviewTitle = title
	}
	if description != "" {
		msg.LinkPreviewDescription = description
	}

	// Set thumbnail as inline image (avoids needing DownloadMedia).
	// Always set when thumbnail is available — even if HasMedia is already true,
	// since the original media.url may have expired (WAHA deletes files after WHATSAPP_FILES_LIFETIME).
	if thumbnail != "" {
		msg.HasMedia = true
		msg.MediaBase64 = thumbnail
		msg.MediaType = "image"
		msg.MimeType = "image/jpeg"
	}
}

// extractLinkPreviewData searches for link preview fields in multiple locations
// within the WAHA webhook payload. WAHA GoWS engine places extendedTextMessage
// data in different structures depending on the payload format.
//
// Search order:
//  1. payload["_data"]["message"]["extendedTextMessage"]
//  2. payload["message"]["extendedTextMessage"]
//  3. payload["_data"] (flat fields)
//  4. Top-level payload (flat fields)
func extractLinkPreviewData(payload map[string]interface{}) (description, title, thumbnail string) {
	// Try nested paths first (most specific)
	searchPaths := [][]string{
		{"_data", "message", "extendedTextMessage"},
		{"message", "extendedTextMessage"},
		{"_data"},
	}

	for _, path := range searchPaths {
		obj := navigatePayload(payload, path)
		if obj == nil {
			continue
		}
		d, _ := extractStringFromPayload(obj, "description")
		t, _ := extractStringFromPayload(obj, "title")
		th, _ := extractStringFromPayload(obj, "jpegThumbnail")
		if th == "" {
			th, _ = extractStringFromPayload(obj, "JPEGThumbnail")
		}
		if d != "" || t != "" || th != "" {
			return d, t, th
		}
	}

	// Fallback: check top-level
	d, _ := extractStringFromPayload(payload, "description")
	t, _ := extractStringFromPayload(payload, "title")
	th, _ := extractStringFromPayload(payload, "jpegThumbnail")
	if th == "" {
		th, _ = extractStringFromPayload(payload, "JPEGThumbnail")
	}
	return d, t, th
}

// navigatePayload traverses a nested map following the given key path.
func navigatePayload(payload map[string]interface{}, keys []string) map[string]interface{} {
	current := payload
	for _, key := range keys {
		next, ok := current[key].(map[string]interface{})
		if !ok {
			return nil
		}
		current = next
	}
	return current
}

// isPlainURL checks if the string is just a URL (no surrounding text).
func isPlainURL(s string) bool {
	trimmed := strings.TrimSpace(s)
	return trimmed != "" &&
		(strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://")) &&
		!strings.ContainsAny(trimmed, " \n\r\t")
}

// extractStringFromPayload safely extracts a string from a map.
func extractStringFromPayload(m map[string]interface{}, key string) (string, bool) {
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

// extractBoolFromPayload safely extracts a bool from a map.
func extractBoolFromPayload(m map[string]interface{}, key string) (bool, bool) {
	if m == nil {
		return false, false
	}
	v, ok := m[key]
	if !ok {
		return false, false
	}
	b, ok := v.(bool)
	return b, ok
}

// extractInt64FromPayload safely extracts an int64 from a map.
// Handles various numeric types from JSON unmarshaling.
func extractInt64FromPayload(m map[string]interface{}, key string) (int64, bool) {
	if m == nil {
		return 0, false
	}
	v, ok := m[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case int:
		return int64(n), true
	case int64:
		return n, true
	case float64:
		return int64(n), true
	case uint64:
		return int64(n), true
	default:
		return 0, false
	}
}

// inferMediaTypeFromMime deriva o MediaType ("image"/"video"/"audio"/"document")
// a partir do MimeType. Usado quando o webhook do WAHA traz a mídia (hasMedia +
// media.url/mimetype) mas sem o campo mediaType nem um type mapeável — caso comum
// no engine GOWS. Sem essa inferência o relay do offer_generate trata a mensagem
// como texto puro e descarta a imagem. Retorna "" quando o mime é vazio.
func inferMediaTypeFromMime(mime string) string {
	mime = strings.ToLower(strings.TrimSpace(mime))
	switch {
	case strings.HasPrefix(mime, "image/"):
		return "image"
	case strings.HasPrefix(mime, "video/"):
		return "video"
	case strings.HasPrefix(mime, "audio/"):
		return "audio"
	case mime != "":
		return "document"
	default:
		return ""
	}
}

// Ensure interface compliance at compile time
var _ MessageWebhookProcessor = (*MonitoringMessageProcessor)(nil)
