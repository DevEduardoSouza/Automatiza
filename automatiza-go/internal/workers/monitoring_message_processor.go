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
	"time"

	"automatiza-go/internal/models"
	"automatiza-go/internal/services/monitoring"
	"automatiza-go/internal/services/offer_generate"

	"go.uber.org/zap"
)

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

	// Skip our own messages
	if msg.FromMe {
		p.logger.Debug("Skipping own message",
			zap.String("session", sessionName),
			zap.String("messageID", msg.MessageID))
		return nil
	}

	// Skip empty messages
	if msg.Body == "" && !msg.HasMedia {
		p.logger.Debug("Skipping empty message",
			zap.String("session", sessionName),
			zap.String("messageID", msg.MessageID))
		return nil
	}

	p.logger.Info("Processing message for monitoring",
		zap.String("session", sessionName),
		zap.String("messageID", msg.MessageID),
		zap.String("group", msg.RemoteJID),
		zap.Bool("hasMedia", msg.HasMedia),
		zap.Bool("fromMe", msg.FromMe),
		zap.Int("bodyLen", len(msg.Body)))

	// Process the message through the monitoring service
	if err := p.monitoringService.ProcessIncomingMessage(ctx, msg); err != nil {
		p.logger.Error("Failed to process message for monitoring",
			zap.String("session", sessionName),
			zap.String("messageID", msg.MessageID),
			zap.Error(err))
		// Return error to trigger retry
		return fmt.Errorf("monitoring service error: %w", err)
	}

	// Also process through the offer generate service (filters internally by automation type)
	if p.offerGenerateService != nil {
		if err := p.offerGenerateService.ProcessIncomingMessage(ctx, msg); err != nil {
			p.logger.Error("Failed to process message for offer generation",
				zap.String("session", sessionName),
				zap.String("messageID", msg.MessageID),
				zap.Error(err))
			// Non-fatal: don't return error to avoid retrying monitoring
		}
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

// Ensure interface compliance at compile time
var _ MessageWebhookProcessor = (*MonitoringMessageProcessor)(nil)
