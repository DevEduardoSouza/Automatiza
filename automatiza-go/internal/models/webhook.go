package models

import "time"

// WhatsAppWebhookPayload represents a webhook payload from WhatsApp (via Waha)
type WhatsAppWebhookPayload struct {
	Event     string                 `json:"event"`
	Session   string                 `json:"session"`
	Engine    string                 `json:"engine"`
	Payload   map[string]interface{} `json:"payload"`
	Timestamp int64                  `json:"timestamp,omitempty"`
}

// WhatsApp webhook event types
const (
	WhatsAppEventMessage         = "message"
	WhatsAppEventMessageAck      = "message.ack"
	WhatsAppEventMessageReaction = "message.reaction"
	WhatsAppEventGroupJoin       = "group.join"
	WhatsAppEventGroupLeave      = "group.leave"
	WhatsAppEventSessionStatus   = "session.status"
	WhatsAppEventQRCode          = "qr.code"
	WhatsAppEventCall            = "call"
)

// WhatsAppMessageWebhook represents the message portion of a WhatsApp webhook
type WhatsAppMessageWebhook struct {
	ID          string                 `json:"id"`
	Timestamp   int64                  `json:"timestamp"`
	From        string                 `json:"from"`
	To          string                 `json:"to"`
	Body        string                 `json:"body,omitempty"`
	FromMe      bool                   `json:"fromMe"`
	Participant string                 `json:"participant,omitempty"`
	HasMedia    bool                   `json:"hasMedia"`
	MediaType   string                 `json:"mediaType,omitempty"`
	Media       map[string]interface{} `json:"media,omitempty"`
}

// IsGroupMessage returns true if the message is from a group
func (w *WhatsAppMessageWebhook) IsGroupMessage() bool {
	return len(w.From) > 0 && (w.From[len(w.From)-4:] == "@g.us" || len(w.To) > 0 && w.To[len(w.To)-4:] == "@g.us")
}

// RedirectRequest represents a request to process a redirect
type RedirectRequest struct {
	URL    string `json:"url" validate:"required"`
	UserID string `json:"userId,omitempty"`
}

// RedirectResponse represents the result of processing a redirect
type RedirectResponse struct {
	OriginalURL string `json:"originalUrl"`
	FinalURL    string `json:"finalUrl"`
	Store       string `json:"store,omitempty"`
}

// WebhookEventRecord represents a row in webhook_events table for idempotency checking.
// This is different from WebhookEvent in monitoring.go which represents incoming webhook payloads.
type WebhookEventRecord struct {
	ID           int        `json:"id"`
	EventID      string     `json:"event_id"`
	SessionName  string     `json:"session_name"`
	EventType    string     `json:"event_type"`
	ReceivedAt   time.Time  `json:"received_at"`
	ProcessedAt  *time.Time `json:"processed_at,omitempty"`
	Status       string     `json:"status"`
	ErrorMessage *string    `json:"error_message,omitempty"`
	RetryCount   int        `json:"retry_count"`
}

// WebhookEventRecordStatus constants for event lifecycle tracking
const (
	WebhookEventRecordStatusReceived   = "received"
	WebhookEventRecordStatusProcessing = "processing"
	WebhookEventRecordStatusProcessed  = "processed"
	WebhookEventRecordStatusFailed     = "failed"
	WebhookEventRecordStatusDLQ        = "dlq"
)
