// Package whatsapp provides the WhatsApp adapter abstraction layer.
// This file contains supplementary domain models that complement the port interface.
// These models extend the core types defined in port.go with additional structures
// for sessions, groups, messages, and webhook events.
//
// IMPORTANT: Core types (SessionStatus, SendTextInput, MessageResult, etc.) are
// defined in port.go. This file adds complementary types for extended functionality.
package whatsapp

import (
	"strings"
	"time"
)

// =============================================================================
// Extended Session Models
// =============================================================================

// Session represents a full WhatsApp session with all metadata.
// This extends SessionInfo from port.go with additional configuration details.
type Session struct {
	// Name is the unique identifier for this session.
	Name string `json:"name"`

	// Status indicates the current session state.
	Status SessionStatus `json:"status"`

	// Me contains information about the authenticated WhatsApp account, if connected.
	Me *SessionMe `json:"me,omitempty"`

	// Config contains session configuration options.
	Config *SessionConfig `json:"config,omitempty"`

	// CreatedAt is when the session was created.
	CreatedAt *time.Time `json:"createdAt,omitempty"`

	// UpdatedAt is when the session was last updated.
	UpdatedAt *time.Time `json:"updatedAt,omitempty"`
}

// SessionMe contains detailed information about the authenticated WhatsApp account.
// This extends AuthenticatedUser from port.go with additional fields.
type SessionMe struct {
	// ID is the WhatsApp ID (phone@c.us format).
	ID string `json:"id"`

	// Phone is the phone number without formatting.
	Phone string `json:"phone,omitempty"`

	// PushName is the display name set by the user.
	PushName string `json:"pushName,omitempty"`
}

// SessionConfig contains extended session configuration options.
type SessionConfig struct {
	// Webhooks contains webhook configuration for this session.
	Webhooks []WebhookConfigEntry `json:"webhooks,omitempty"`

	// ProxyURL is the proxy server URL if configured.
	ProxyURL string `json:"proxyUrl,omitempty"`

	// DebugMode indicates if debug logging is enabled.
	DebugMode bool `json:"debugMode,omitempty"`
}

// WebhookConfigEntry represents a single webhook configuration for a session.
type WebhookConfigEntry struct {
	// URL is the webhook endpoint URL.
	URL string `json:"url"`

	// Events is the list of events to subscribe to.
	Events []string `json:"events,omitempty"`

	// HMACSecret is the secret for HMAC signature verification.
	HMACSecret string `json:"hmacSecret,omitempty"`
}

// SessionStatusDetails provides detailed status information beyond the basic SessionStatus.
type SessionStatusDetails struct {
	// Status is the session state string.
	Status SessionStatus `json:"status"`

	// Connected indicates if the session is currently connected to WhatsApp.
	Connected bool `json:"connected"`

	// Phone is the connected phone number, if available.
	Phone string `json:"phone,omitempty"`

	// PushName is the display name, if available.
	PushName string `json:"pushName,omitempty"`

	// LastSeen is the timestamp of last activity.
	LastSeen *time.Time `json:"lastSeen,omitempty"`
}

// IsConnected returns true if the session status indicates an active connection.
func (s *SessionStatusDetails) IsConnected() bool {
	return s.Connected || s.Status == SessionStatusConnected
}

// QRCode represents a QR code for WhatsApp authentication with expiration.
// This extends QRCodeData from port.go with expiration tracking.
type QRCode struct {
	// Value is the base64-encoded QR code image.
	Value string `json:"value"`

	// Raw is the raw QR code string data (for generating QR locally).
	Raw string `json:"raw,omitempty"`

	// MimeType is the image MIME type (e.g., "image/png").
	MimeType string `json:"mimeType,omitempty"`

	// ExpiresAt is when the QR code expires and a new one must be requested.
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
}

// =============================================================================
// Extended Group Models
// =============================================================================

// Group represents a WhatsApp group with full details.
// This extends GroupInfo from port.go with additional fields.
type Group struct {
	// ID is the WhatsApp group ID (e.g., "123456789@g.us").
	ID string `json:"id"`

	// Name is the group subject/name.
	Name string `json:"name"`

	// Description is the group description.
	Description string `json:"description,omitempty"`

	// Picture is the URL or base64 of the group picture.
	Picture string `json:"picture,omitempty"`

	// ParticipantsCount is the number of participants in the group.
	ParticipantsCount int `json:"participantsCount"`

	// Participants is the list of group participants (may be empty if not fetched).
	Participants []Participant `json:"participants,omitempty"`

	// InviteCode is the group invite code/link.
	InviteCode string `json:"inviteCode,omitempty"`

	// IsAdmin indicates if the current session user is an admin of this group.
	IsAdmin bool `json:"isAdmin"`

	// IsSuperAdmin indicates if the current session user is the group owner.
	IsSuperAdmin bool `json:"isSuperAdmin"`

	// AdminOnly indicates if only admins can send messages (announcement mode).
	AdminOnly bool `json:"adminOnly"`

	// CreatedAt is the group creation timestamp.
	CreatedAt *time.Time `json:"createdAt,omitempty"`

	// Owner is the WhatsApp ID of the group creator.
	Owner string `json:"owner,omitempty"`
}

// Participant represents a participant in a WhatsApp group with extended details.
// This extends ParticipantInfo from port.go with phone and name.
type Participant struct {
	// ID is the participant's WhatsApp ID (e.g., "5511999999999@c.us").
	ID string `json:"id"`

	// Phone is the phone number extracted from the ID.
	Phone string `json:"phone,omitempty"`

	// Name is the participant's push name or contact name.
	Name string `json:"name,omitempty"`

	// IsAdmin indicates if the participant is a group admin.
	IsAdmin bool `json:"isAdmin"`

	// IsSuperAdmin indicates if the participant is the group owner.
	IsSuperAdmin bool `json:"isSuperAdmin"`
}

// =============================================================================
// Extended Message Models
// =============================================================================

// ExtendedMessage represents a WhatsApp message with all available fields.
// This extends Message from port.go with additional metadata.
type ExtendedMessage struct {
	// ID is the unique message identifier.
	ID string `json:"id"`

	// ChatID is the chat/group where the message was sent.
	ChatID string `json:"chatId"`

	// From is the sender's WhatsApp ID.
	From string `json:"from,omitempty"`

	// To is the recipient's WhatsApp ID.
	To string `json:"to,omitempty"`

	// FromMe indicates if the message was sent by the current session.
	FromMe bool `json:"fromMe"`

	// Timestamp is when the message was sent.
	Timestamp time.Time `json:"timestamp"`

	// Body is the text content of the message.
	Body string `json:"body,omitempty"`

	// HasMedia indicates if the message contains media.
	HasMedia bool `json:"hasMedia"`

	// MediaType is the type of media (image, video, audio, document, sticker).
	MediaType string `json:"mediaType,omitempty"`

	// MediaURL is the URL to download the media.
	MediaURL string `json:"mediaUrl,omitempty"`

	// MimeType is the MIME type of the media.
	MimeType string `json:"mimeType,omitempty"`

	// Caption is the caption for media messages.
	Caption string `json:"caption,omitempty"`

	// QuotedMessageID is the ID of the message being replied to.
	QuotedMessageID string `json:"quotedMessageId,omitempty"`

	// Mentions is the list of mentioned WhatsApp IDs.
	Mentions []string `json:"mentions,omitempty"`

	// IsForwarded indicates if the message was forwarded.
	IsForwarded bool `json:"isForwarded"`

	// Participant is the sender in a group message (different from ChatID).
	Participant string `json:"participant,omitempty"`

	// Ack is the message acknowledgment status.
	Ack MessageAckLevel `json:"ack"`
}

// MessageAckLevel represents message acknowledgment status with helper methods.
type MessageAckLevel int

// MessageAckLevel constants.
const (
	MessageAckError   MessageAckLevel = -1
	MessageAckPending MessageAckLevel = 0
	MessageAckServer  MessageAckLevel = 1
	MessageAckDevice  MessageAckLevel = 2
	MessageAckRead    MessageAckLevel = 3
	MessageAckPlayed  MessageAckLevel = 4
)

// String returns the human-readable ack status.
func (a MessageAckLevel) String() string {
	switch a {
	case MessageAckError:
		return "error"
	case MessageAckPending:
		return "pending"
	case MessageAckServer:
		return "sent"
	case MessageAckDevice:
		return "delivered"
	case MessageAckRead:
		return "read"
	case MessageAckPlayed:
		return "played"
	default:
		return "unknown"
	}
}

// MediaType constants for message media.
const (
	MediaTypeImage    = "image"
	MediaTypeVideo    = "video"
	MediaTypeAudio    = "audio"
	MediaTypeVoice    = "voice" // PTT audio
	MediaTypeDocument = "document"
	MediaTypeSticker  = "sticker"
)

// ExtendedMessageResult extends MessageResult with additional fields.
type ExtendedMessageResult struct {
	// ID is the message identifier assigned by WhatsApp.
	ID string `json:"id"`

	// Timestamp is when the message was sent.
	Timestamp time.Time `json:"timestamp"`

	// Status is the initial message status (usually "sent").
	Status string `json:"status"`

	// Ack is the acknowledgment level.
	Ack MessageAckLevel `json:"ack"`
}

// MessagesPage represents a paginated list of messages with cursor support.
type MessagesPage struct {
	// Messages is the list of messages in this page.
	Messages []Message `json:"messages"`

	// HasMore indicates if there are more messages to fetch.
	HasMore bool `json:"hasMore"`

	// NextOffset is the offset to use for fetching the next page.
	NextOffset int `json:"nextOffset,omitempty"`

	// NextCursor is the cursor for cursor-based pagination.
	NextCursor string `json:"nextCursor,omitempty"`

	// Total is the total number of messages (if known).
	Total int `json:"total,omitempty"`
}

// =============================================================================
// Extended Input Models
// =============================================================================

// SendVideoInput contains the data needed to send a video message.
type SendVideoInput struct {
	// SessionName is the session to use for sending.
	SessionName string `json:"sessionName" validate:"required"`

	// ChatID is the recipient chat ID.
	ChatID string `json:"chatId" validate:"required"`

	// Caption is the optional video caption.
	Caption string `json:"caption,omitempty"`

	// MediaURL is the URL to fetch the video from (mutually exclusive with MediaBase64).
	MediaURL string `json:"mediaUrl,omitempty"`

	// MediaBase64 is the base64-encoded video data (mutually exclusive with MediaURL).
	MediaBase64 string `json:"mediaBase64,omitempty"`

	// MimeType is the video MIME type (e.g., "video/mp4").
	MimeType string `json:"mimeType,omitempty"`

	// Filename is the optional filename for the video.
	Filename string `json:"filename,omitempty"`

	// Mentions is the list of WhatsApp IDs to mention in the caption.
	Mentions []string `json:"mentions,omitempty"`

	// QuotedMessageID is the ID of a message to reply to.
	QuotedMessageID string `json:"quotedMessageId,omitempty"`
}

// Validate checks if the input has valid media source.
func (v *SendVideoInput) Validate() error {
	if v.MediaURL == "" && v.MediaBase64 == "" {
		return ErrNoMediaSource
	}
	if v.MediaURL != "" && v.MediaBase64 != "" {
		return ErrMultipleMediaSources
	}
	return nil
}

// SendDocumentInput contains the data needed to send a document.
type SendDocumentInput struct {
	// SessionName is the session to use for sending.
	SessionName string `json:"sessionName" validate:"required"`

	// ChatID is the recipient chat ID.
	ChatID string `json:"chatId" validate:"required"`

	// Caption is the optional document caption.
	Caption string `json:"caption,omitempty"`

	// MediaURL is the URL to fetch the document from (mutually exclusive with MediaBase64).
	MediaURL string `json:"mediaUrl,omitempty"`

	// MediaBase64 is the base64-encoded document data (mutually exclusive with MediaURL).
	MediaBase64 string `json:"mediaBase64,omitempty"`

	// MimeType is the document MIME type.
	MimeType string `json:"mimeType,omitempty"`

	// Filename is the document filename (required for documents).
	Filename string `json:"filename" validate:"required"`

	// Mentions is the list of WhatsApp IDs to mention in the caption.
	Mentions []string `json:"mentions,omitempty"`

	// QuotedMessageID is the ID of a message to reply to.
	QuotedMessageID string `json:"quotedMessageId,omitempty"`
}

// Validate checks if the input has valid media source.
func (d *SendDocumentInput) Validate() error {
	if d.MediaURL == "" && d.MediaBase64 == "" {
		return ErrNoMediaSource
	}
	if d.MediaURL != "" && d.MediaBase64 != "" {
		return ErrMultipleMediaSources
	}
	return nil
}

// SendReactionInput contains the data needed to send a reaction.
type SendReactionInput struct {
	// SessionName is the session to use for sending.
	SessionName string `json:"sessionName" validate:"required"`

	// ChatID is the chat containing the message.
	ChatID string `json:"chatId" validate:"required"`

	// MessageID is the ID of the message to react to.
	MessageID string `json:"messageId" validate:"required"`

	// Emoji is the reaction emoji (empty string to remove reaction).
	Emoji string `json:"emoji"`
}

// =============================================================================
// Webhook/Event Models
// =============================================================================

// WebhookEvent represents an incoming webhook event from WhatsApp.
type WebhookEvent struct {
	// Event is the event type (e.g., "message", "message.ack", "session.status").
	Event string `json:"event"`

	// SessionName is the session that triggered this event.
	SessionName string `json:"session"`

	// Timestamp is when the event occurred.
	Timestamp time.Time `json:"timestamp"`

	// Payload contains the event-specific data.
	Payload interface{} `json:"payload"`
}

// Common webhook event types.
const (
	EventMessage         = "message"
	EventMessageAck      = "message.ack"
	EventMessageReaction = "message.reaction"
	EventSessionStatus   = "session.status"
	EventGroupJoin       = "group.join"
	EventGroupLeave      = "group.leave"
	EventPresenceUpdate  = "presence.update"
	EventPollVote        = "poll.vote"
)

// =============================================================================
// Validation Errors
// =============================================================================

// DomainError represents a domain-level error.
type DomainError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *DomainError) Error() string {
	return e.Message
}

// Common domain errors (supplement ProviderError from port.go).
var (
	ErrNoMediaSource        = &DomainError{Code: "NO_MEDIA_SOURCE", Message: "either MediaURL or MediaBase64 must be provided"}
	ErrMultipleMediaSources = &DomainError{Code: "MULTIPLE_MEDIA_SOURCES", Message: "only one of MediaURL or MediaBase64 can be provided"}
	ErrPollMinOptions       = &DomainError{Code: "POLL_MIN_OPTIONS", Message: "poll must have at least 2 options"}
	ErrPollMaxOptions       = &DomainError{Code: "POLL_MAX_OPTIONS", Message: "poll cannot have more than 12 options"}
	ErrInvalidChatID        = &DomainError{Code: "INVALID_CHAT_ID", Message: "invalid chat ID format"}
	ErrMessageNotFound      = &DomainError{Code: "MESSAGE_NOT_FOUND", Message: "message not found"}
)

// =============================================================================
// Utility Types and Functions
// =============================================================================

// ChatType represents the type of a WhatsApp chat.
type ChatType string

const (
	ChatTypePrivate   ChatType = "private"
	ChatTypeGroup     ChatType = "group"
	ChatTypeBroadcast ChatType = "broadcast"
)

// ParseChatType determines the chat type from a WhatsApp ID.
func ParseChatType(chatID string) ChatType {
	if len(chatID) == 0 {
		return ChatTypePrivate
	}
	// Group IDs end with @g.us
	if len(chatID) > 5 && chatID[len(chatID)-5:] == "@g.us" {
		return ChatTypeGroup
	}
	// Broadcast lists end with @broadcast
	if len(chatID) > 10 && chatID[len(chatID)-10:] == "@broadcast" {
		return ChatTypeBroadcast
	}
	return ChatTypePrivate
}

// IsGroupChat returns true if the chat ID represents a group.
func IsGroupChat(chatID string) bool {
	return ParseChatType(chatID) == ChatTypeGroup
}

// ExtractPhoneFromID extracts the phone number from a WhatsApp ID.
// For example, "5511999999999@c.us" returns "5511999999999".
func ExtractPhoneFromID(waID string) string {
	if len(waID) == 0 {
		return ""
	}
	for i, c := range waID {
		if c == '@' {
			return waID[:i]
		}
	}
	return waID
}

// EnsureParticipantID ensures a value is a full WhatsApp participant ID.
// If it already contains "@" (e.g. "56243167223861@lid"), it's returned as-is.
// Otherwise, "@c.us" is appended (e.g. "5511999999999" → "5511999999999@c.us").
// The WAHA GOWS engine requires the @c.us suffix for participant operations.
func EnsureParticipantID(idOrPhone string) string {
	if strings.Contains(idOrPhone, "@") {
		return idOrPhone
	}
	return idOrPhone + "@c.us"
}

// FormatPhoneAsWAID formats a phone number as a WhatsApp private chat ID.
// For example, "5511999999999" returns "5511999999999@c.us".
func FormatPhoneAsWAID(phone string) string {
	if len(phone) == 0 {
		return ""
	}
	// Remove any existing suffix
	phone = ExtractPhoneFromID(phone)
	return phone + "@c.us"
}

// =============================================================================
// Conversion Helpers
// =============================================================================

// ToSessionInfo converts a Session to SessionInfo (port.go type).
func (s *Session) ToSessionInfo() *SessionInfo {
	info := &SessionInfo{
		Name:   s.Name,
		Status: s.Status,
	}
	if s.Me != nil {
		info.AuthenticatedUser = &AuthenticatedUser{
			ID:       s.Me.ID,
			PushName: s.Me.PushName,
		}
	}
	return info
}

// ToGroupInfo converts a Group to GroupInfo (port.go type).
func (g *Group) ToGroupInfo() GroupInfo {
	participants := make([]ParticipantInfo, len(g.Participants))
	for i, p := range g.Participants {
		participants[i] = ParticipantInfo{
			ID:           p.ID,
			IsAdmin:      p.IsAdmin,
			IsSuperAdmin: p.IsSuperAdmin,
		}
	}

	var createdAt time.Time
	if g.CreatedAt != nil {
		createdAt = *g.CreatedAt
	}

	return GroupInfo{
		ID:           g.ID,
		Subject:      g.Name,
		Description:  g.Description,
		Owner:        g.Owner,
		CreatedAt:    createdAt,
		Announce:     g.AdminOnly,
		Restrict:     false, // Not tracked in Group
		Size:         g.ParticipantsCount,
		Participants: participants,
		IsAdmin:      g.IsAdmin,
		IsSuperAdmin: g.IsSuperAdmin,
	}
}

// ToParticipantInfo converts a Participant to ParticipantInfo (port.go type).
func (p *Participant) ToParticipantInfo() ParticipantInfo {
	return ParticipantInfo{
		ID:           p.ID,
		IsAdmin:      p.IsAdmin,
		IsSuperAdmin: p.IsSuperAdmin,
	}
}
