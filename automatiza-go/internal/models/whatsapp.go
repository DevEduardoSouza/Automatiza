package models

// Contact represents a WhatsApp contact (group participant)
type Contact struct {
	ID           string `json:"id"`
	Name         string `json:"name,omitempty"`
	PushName     string `json:"pushName,omitempty"`
	PhoneNumber  string `json:"phoneNumber,omitempty"`
	IsAdmin      *bool  `json:"isAdmin,omitempty"`
	IsSuperAdmin *bool  `json:"isSuperAdmin,omitempty"`
}

// ContactExtractRequest represents a request to extract contacts from a group
type ContactExtractRequest struct {
	InstanceName string `json:"instanceName" validate:"required"`
	GroupID      string `json:"groupId" validate:"required"`
}

// ContactExtractResponse represents the response from contact extraction
type ContactExtractResponse struct {
	Contacts []Contact `json:"contacts"`
	Total    int       `json:"total"`
}

// WhatsAppMessage represents a message to send via WhatsApp
type WhatsAppMessage struct {
	InstanceName string   `json:"instanceName" validate:"required"`
	GroupID      string   `json:"groupId" validate:"required"`
	Message      string   `json:"message" validate:"required"`
	Mentions     []string `json:"mentions,omitempty"`
	ImageURL     *string  `json:"imageUrl,omitempty"`
	ImageBase64  *string  `json:"imageBase64,omitempty"`
}

// WhatsAppAudioMessage represents an audio message
type WhatsAppAudioMessage struct {
	InstanceName string `json:"instanceName" validate:"required"`
	GroupID      string `json:"groupId" validate:"required"`
	AudioBase64  string `json:"audioBase64" validate:"required"`
	Mimetype     string `json:"mimetype" validate:"required"`
	PTT          bool   `json:"ptt"` // Push to talk (voice note)
}

// WhatsAppPollMessage represents a poll message
type WhatsAppPollMessage struct {
	InstanceName    string   `json:"instanceName" validate:"required"`
	GroupID         string   `json:"groupId" validate:"required"`
	Question        string   `json:"question" validate:"required"`
	Options         []string `json:"options" validate:"required,min=2,max=12"`
	SelectableCount int      `json:"selectableCount" validate:"min=1"`
}

// WebhookMessage represents an incoming webhook message from WhatsApp
type WebhookMessage struct {
	Key       WebhookMessageKey `json:"key"`
	Message   interface{}       `json:"message"`
	Body      string            `json:"body,omitempty"`
	RemoteJid string            `json:"remoteJid"`
	Timestamp int64             `json:"messageTimestamp"`
	HasMedia  bool              `json:"hasMedia"`
	MediaType string            `json:"mediaType,omitempty"`
}

// WebhookMessageKey represents the message key
type WebhookMessageKey struct {
	ID          string `json:"id"`
	RemoteJid   string `json:"remoteJid"`
	FromMe      bool   `json:"fromMe"`
	Participant string `json:"participant,omitempty"`
}

// WahaSendTextRequest represents a text message request to Waha
type WahaSendTextRequest struct {
	Session  string   `json:"session"`
	ChatID   string   `json:"chatId"`
	Text     string   `json:"text"`
	Mentions []string `json:"mentions,omitempty"`
}

// WahaSendImageRequest represents an image message request to Waha
type WahaSendImageRequest struct {
	Session string    `json:"session"`
	ChatID  string    `json:"chatId"`
	Caption string    `json:"caption,omitempty"`
	File    *WahaFile `json:"file,omitempty"`
}

// WahaSendAudioRequest represents an audio message request to Waha
type WahaSendAudioRequest struct {
	Session string    `json:"session"`
	ChatID  string    `json:"chatId"`
	File    *WahaFile `json:"file"`
}

// WahaSendPollRequest represents a poll message request to Waha
type WahaSendPollRequest struct {
	Session string       `json:"session"`
	ChatID  string       `json:"chatId"`
	Poll    WahaPollData `json:"poll"`
}

// WahaPollData represents the nested poll object in a WAHA poll request.
type WahaPollData struct {
	Name            string   `json:"name"`
	Options         []string `json:"options"`
	MultipleAnswers bool     `json:"multipleAnswers"`
}

// WahaFile represents a file in Waha requests
type WahaFile struct {
	Data     string `json:"data,omitempty"` // Base64 data
	Mimetype string `json:"mimetype,omitempty"`
	Filename string `json:"filename,omitempty"`
	URL      string `json:"url,omitempty"` // Alternative to data
}

// WahaWebhookPayload represents the webhook payload from Waha
type WahaWebhookPayload struct {
	Event   string                 `json:"event"`
	Session string                 `json:"session"`
	Engine  string                 `json:"engine"`
	Payload map[string]interface{} `json:"payload"`
}

// WahaMessagePayload represents the message payload in webhooks
type WahaMessagePayload struct {
	ID          string                 `json:"id"`
	Timestamp   int64                  `json:"timestamp"`
	From        string                 `json:"from"`
	To          string                 `json:"to"`
	Body        string                 `json:"body"`
	HasMedia    bool                   `json:"hasMedia"`
	MediaType   string                 `json:"mediaType,omitempty"`
	Ack         int                    `json:"ack"`
	AckName     string                 `json:"ackName"`
	IsForwarded bool                   `json:"isForwarded"`
	FromMe      bool                   `json:"fromMe"`
	Participant string                 `json:"participant,omitempty"`
	Media       map[string]interface{} `json:"media,omitempty"`
}
