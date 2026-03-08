// Package whatsapp provides the WhatsApp adapter abstraction layer.
// This file contains WAHA-specific API request/response structures.
// These types are internal to the adapter and should NOT be used outside this package.
//
// IMPORTANT: These are WAHA API types, not domain types. Use the mapper to convert
// between WAHA types and domain types defined in domain_models.go and port.go.
package whatsapp

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"automatiza-go/internal/observability"

	"github.com/getsentry/sentry-go"
)

// =============================================================================
// WAHA Session Types
// =============================================================================

// wahaSession represents a WAHA session response.
// Maps to GET /api/sessions/{name} and POST /api/sessions responses.
type wahaSession struct {
	Name   string             `json:"name"`
	Status string             `json:"status"`
	Me     *wahaSessionMe     `json:"me,omitempty"`
	Config *wahaSessionConfig `json:"config,omitempty"`
}

// wahaSessionMe contains the authenticated user information from WAHA.
type wahaSessionMe struct {
	ID       string `json:"id"`
	Phone    string `json:"phone,omitempty"`
	PushName string `json:"pushName,omitempty"`
}

// wahaSessionConfig contains WAHA session configuration.
type wahaSessionConfig struct {
	Webhooks  []wahaWebhookConfig `json:"webhooks,omitempty"`
	ProxyURL  string              `json:"proxy,omitempty"`
	DebugMode bool                `json:"debug,omitempty"`
}

// wahaWebhookConfig represents a webhook configuration in WAHA.
// Maps to the WebhookConfig schema in the WAHA OpenAPI spec.
type wahaWebhookConfig struct {
	URL    string          `json:"url"`
	Events []string        `json:"events,omitempty"`
	HMAC   *wahaHMACConfig `json:"hmac,omitempty"`
}

// wahaUpdateSessionRequest represents a WAHA session update request.
// Maps to PUT /api/sessions/{session} request body (SessionUpdateRequest schema).
type wahaUpdateSessionRequest struct {
	Config *wahaSessionConfig `json:"config,omitempty"`
}

// wahaQRCode represents a WAHA QR code response.
// Maps to GET /api/{session}/auth/qr response.
type wahaQRCode struct {
	Value    string `json:"value"`
	Mimetype string `json:"mimetype,omitempty"`
}

// wahaCreateSessionRequest represents a WAHA session creation request.
// Maps to POST /api/sessions request body.
type wahaCreateSessionRequest struct {
	Name   string             `json:"name"`
	Start  bool               `json:"start"`
	Config *wahaSessionConfig `json:"config,omitempty"`
}

// =============================================================================
// WAHA Group Types
// =============================================================================

// wahaGroup represents a WAHA group response.
// Maps to GET /api/{session}/groups response items.
// Supports both GOWS engine format (PascalCase Go fields) and WEBJS/NOWEB DTO format (camelCase).
type wahaGroup struct {
	ID           string
	Subject      string
	Description  string
	Picture      string
	Owner        string
	Announce     bool
	Restrict     bool
	Size         int
	Participants []wahaParticipant
	Creation     int64
	InviteCode   string
	IsSuperAdmin bool
	IsAdmin      bool
}

// unmarshalField is a helper that unmarshals a JSON field and tracks errors.
func unmarshalField(v json.RawMessage, dst interface{}, errCount *int) {
	if err := json.Unmarshal(v, dst); err != nil {
		*errCount++
	}
}

// UnmarshalJSON handles both GOWS engine format (PascalCase) and WEBJS/NOWEB DTO format (camelCase).
// GOWS returns: {JID, Name, Topic, OwnerJID, IsAnnounce, IsLocked, Participants, MemberAddMode, ...}
// DTO returns:  {id, subject, description, invite, membersCanSendMessages, participants, ...}
func (g *wahaGroup) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	var errCount int

	// ID: "id" (DTO) or "JID" (GOWS)
	if v, ok := raw["id"]; ok {
		unmarshalField(v, &g.ID, &errCount)
	} else if v, ok := raw["JID"]; ok {
		unmarshalField(v, &g.ID, &errCount)
	}

	// Subject: "subject" (DTO) or "Name" (GOWS)
	if v, ok := raw["subject"]; ok {
		unmarshalField(v, &g.Subject, &errCount)
	} else if v, ok := raw["Name"]; ok {
		unmarshalField(v, &g.Subject, &errCount)
	}

	// Description: "description" (DTO) or "desc" (legacy) or "Topic" (GOWS)
	if v, ok := raw["description"]; ok {
		unmarshalField(v, &g.Description, &errCount)
	} else if v, ok := raw["desc"]; ok {
		unmarshalField(v, &g.Description, &errCount)
	} else if v, ok := raw["Topic"]; ok {
		unmarshalField(v, &g.Description, &errCount)
	}

	// Picture
	if v, ok := raw["picture"]; ok {
		unmarshalField(v, &g.Picture, &errCount)
	}

	// Owner: "owner" (DTO) or "OwnerJID" (GOWS)
	if v, ok := raw["owner"]; ok {
		unmarshalField(v, &g.Owner, &errCount)
	} else if v, ok := raw["OwnerJID"]; ok {
		unmarshalField(v, &g.Owner, &errCount)
	}

	// Announce: "announce" (legacy) or "IsAnnounce" (GOWS) or inverted "membersCanSendMessages" (DTO)
	if v, ok := raw["announce"]; ok {
		unmarshalField(v, &g.Announce, &errCount)
	} else if v, ok := raw["IsAnnounce"]; ok {
		unmarshalField(v, &g.Announce, &errCount)
	} else if v, ok := raw["membersCanSendMessages"]; ok {
		var canSend bool
		unmarshalField(v, &canSend, &errCount)
		g.Announce = !canSend
	}

	// Restrict: "restrict" (legacy) or "IsLocked" (GOWS)
	if v, ok := raw["restrict"]; ok {
		unmarshalField(v, &g.Restrict, &errCount)
	} else if v, ok := raw["IsLocked"]; ok {
		unmarshalField(v, &g.Restrict, &errCount)
	}

	// Size
	if v, ok := raw["size"]; ok {
		unmarshalField(v, &g.Size, &errCount)
	}

	// Participants: "participants" (DTO) or "Participants" (GOWS) — case-insensitive match handled by key check
	if v, ok := raw["participants"]; ok {
		unmarshalField(v, &g.Participants, &errCount)
	} else if v, ok := raw["Participants"]; ok {
		unmarshalField(v, &g.Participants, &errCount)
	}

	// Creation
	if v, ok := raw["creation"]; ok {
		unmarshalField(v, &g.Creation, &errCount)
	}

	// InviteCode: "inviteCode" (legacy) or "invite" (DTO)
	if v, ok := raw["inviteCode"]; ok {
		unmarshalField(v, &g.InviteCode, &errCount)
	} else if v, ok := raw["invite"]; ok {
		unmarshalField(v, &g.InviteCode, &errCount)
	}

	// IsAdmin / IsSuperAdmin
	if v, ok := raw["isAdmin"]; ok {
		unmarshalField(v, &g.IsAdmin, &errCount)
	} else if v, ok := raw["IsAdmin"]; ok {
		unmarshalField(v, &g.IsAdmin, &errCount)
	}
	if v, ok := raw["isSuperAdmin"]; ok {
		unmarshalField(v, &g.IsSuperAdmin, &errCount)
	} else if v, ok := raw["IsSuperAdmin"]; ok {
		unmarshalField(v, &g.IsSuperAdmin, &errCount)
	}

	// Derive Size from Participants if not set
	if g.Size == 0 && len(g.Participants) > 0 {
		g.Size = len(g.Participants)
	}

	if errCount > 0 {
		observability.CaptureMessage(sentry.LevelWarning,
			fmt.Sprintf("wahaGroup UnmarshalJSON: %d field(s) failed to unmarshal", errCount),
			map[string]string{
				"module": "whatsapp", "operation": "waha_group_unmarshal",
				"group_id": g.ID,
			})
	}

	return nil
}

// wahaParticipant represents a WAHA group participant.
// Supports both GOWS engine format ({JID, IsAdmin, IsSuperAdmin}) and DTO format ({id, role}).
type wahaParticipant struct {
	ID           string
	PhoneNumber  string
	IsAdmin      bool
	IsSuperAdmin bool
}

// UnmarshalJSON handles both GOWS engine format and WAHA DTO format.
// GOWS embedded:  {JID, PhoneNumber, IsAdmin, IsSuperAdmin}
// DTO endpoint:   {id, pn, role}
func (p *wahaParticipant) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	var errCount int

	// ID: "id" (DTO) or "JID" (GOWS)
	if v, ok := raw["id"]; ok {
		unmarshalField(v, &p.ID, &errCount)
	} else if v, ok := raw["JID"]; ok {
		unmarshalField(v, &p.ID, &errCount)
	}

	// Admin status: check booleans first (GOWS embedded), then role (DTO endpoint)
	if v, ok := raw["IsAdmin"]; ok {
		unmarshalField(v, &p.IsAdmin, &errCount)
	} else if v, ok := raw["isAdmin"]; ok {
		unmarshalField(v, &p.IsAdmin, &errCount)
	}
	if v, ok := raw["IsSuperAdmin"]; ok {
		unmarshalField(v, &p.IsSuperAdmin, &errCount)
	} else if v, ok := raw["isSuperAdmin"]; ok {
		unmarshalField(v, &p.IsSuperAdmin, &errCount)
	}

	// Derive from role if booleans are not set (DTO format uses role field)
	if !p.IsAdmin && !p.IsSuperAdmin {
		if v, ok := raw["role"]; ok {
			var role string
			unmarshalField(v, &role, &errCount)
			p.IsSuperAdmin = role == "superadmin"
			p.IsAdmin = role == "admin" || role == "superadmin"
		}
	}

	// PhoneNumber: "pn" (DTO v2) or "PhoneNumber" (GOWS)
	if v, ok := raw["pn"]; ok {
		unmarshalField(v, &p.PhoneNumber, &errCount)
	} else if v, ok := raw["PhoneNumber"]; ok {
		unmarshalField(v, &p.PhoneNumber, &errCount)
	}

	if errCount > 0 {
		observability.CaptureMessage(sentry.LevelWarning,
			fmt.Sprintf("wahaParticipant UnmarshalJSON: %d field(s) failed to unmarshal", errCount),
			map[string]string{
				"module": "whatsapp", "operation": "waha_participant_unmarshal",
				"participant_id": p.ID,
			})
	}

	return nil
}

// wahaLidMapping represents a LID-to-phone-number mapping from WAHA.
// Maps to GET /api/{session}/lids response items.
type wahaLidMapping struct {
	LID string `json:"lid"` // e.g. "56243167223861@lid"
	PN  string `json:"pn"`  // e.g. "558191993610@c.us"
}

// wahaUpdateGroupSubjectRequest represents a WAHA group subject update request.
// Maps to PUT /api/{session}/groups/{groupId}/subject request body.
type wahaUpdateGroupSubjectRequest struct {
	Subject string `json:"subject"`
}

// wahaUpdateGroupDescriptionRequest represents a WAHA group description update request.
// Maps to PUT /api/{session}/groups/{groupId}/description request body.
type wahaUpdateGroupDescriptionRequest struct {
	Description string `json:"description"`
}

// wahaUpdateGroupPictureRequest represents a WAHA group picture update request.
// Maps to PUT /api/{session}/groups/{groupId}/picture request body.
type wahaUpdateGroupPictureRequest struct {
	File wahaFile `json:"file"`
}

// wahaSetGroupAnnouncementRequest represents a WAHA group announcement mode request.
// Maps to PUT /api/{session}/groups/{groupId}/settings/security/messages-admin-only request body.
type wahaSetGroupAnnouncementRequest struct {
	AdminsOnly bool `json:"adminsOnly"`
}

// wahaInviteCodeResponse represents a WAHA group invite code response.
// Maps to GET /api/{session}/groups/{id}/invite-code response.
type wahaInviteCodeResponse struct {
	InviteCode string `json:"inviteCode"`
}

// =============================================================================
// WAHA Message Types
// =============================================================================

// wahaSendTextRequest represents a WAHA text message request.
// Maps to POST /api/sendText request body.
type wahaSendTextRequest struct {
	Session  string   `json:"session"`
	ChatID   string   `json:"chatId"`
	Text     string   `json:"text"`
	Mentions []string `json:"mentions,omitempty"`
}

// wahaSendImageRequest represents a WAHA image message request.
// Maps to POST /api/sendImage request body.
type wahaSendImageRequest struct {
	Session string    `json:"session"`
	ChatID  string    `json:"chatId"`
	Caption string    `json:"caption,omitempty"`
	File    *wahaFile `json:"file,omitempty"`
}

// wahaSendVoiceRequest represents a WAHA voice/audio message request.
// Maps to POST /api/sendVoice request body.
type wahaSendVoiceRequest struct {
	Session string        `json:"session"`
	ChatID  string        `json:"chatId"`
	File    wahaVoiceFile `json:"file"`
	Convert bool          `json:"convert"`
}

// wahaVoiceFile represents a VoiceBinaryFile in WAHA voice requests.
// All fields are required per the WAHA OpenAPI spec.
type wahaVoiceFile struct {
	Mimetype string `json:"mimetype"`
	Filename string `json:"filename"`
	Data     string `json:"data"`
}

// wahaSendPollRequest represents a WAHA poll message request.
// Maps to POST /api/sendPoll request body.
type wahaSendPollRequest struct {
	Session string       `json:"session"`
	ChatID  string       `json:"chatId"`
	Poll    wahaPollData `json:"poll"`
}

// wahaPollData represents the nested poll object in a WAHA poll request.
type wahaPollData struct {
	Name            string   `json:"name"`
	Options         []string `json:"options"`
	MultipleAnswers bool     `json:"multipleAnswers"`
}

// wahaSendLinkPreviewRequest represents a WAHA link custom preview request.
// Maps to POST /api/send/link-custom-preview request body.
type wahaSendLinkPreviewRequest struct {
	Session string                      `json:"session"`
	ChatID  string                      `json:"chatId"`
	Text    string                      `json:"text"`
	Preview *wahaSendLinkPreviewPreview `json:"preview"`
}

// wahaSendLinkPreviewPreview represents the preview object inside a link preview request.
type wahaSendLinkPreviewPreview struct {
	URL         string    `json:"url"`
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"`
	Image       *wahaFile `json:"image,omitempty"`
}

// wahaFile represents a file attachment in WAHA requests.
type wahaFile struct {
	Data     string `json:"data,omitempty"`     // Base64 encoded data
	Mimetype string `json:"mimetype,omitempty"` // MIME type
	Filename string `json:"filename,omitempty"` // Optional filename
	URL      string `json:"url,omitempty"`      // Alternative to base64 data
}

// wahaSendMessageResponse represents a WAHA message send response.
// Maps to POST /api/send* responses.
type wahaSendMessageResponse struct {
	ID        string `json:"id"`
	Timestamp int64  `json:"timestamp,omitempty"`
	Status    string `json:"status,omitempty"`
	Ack       int    `json:"ack,omitempty"`
}

// =============================================================================
// WAHA Message Retrieval Types
// =============================================================================

// wahaGetMessagesResponse represents a WAHA messages list response.
// Maps to GET /api/{session}/chats/{chatId}/messages response.
type wahaGetMessagesResponse struct {
	Messages []wahaMessage `json:"messages"`
	HasMore  bool          `json:"hasMore,omitempty"`
}

// wahaMessage represents a message in WAHA responses.
type wahaMessage struct {
	ID          string `json:"id"`
	ChatID      string `json:"chatId,omitempty"`
	From        string `json:"from,omitempty"`
	To          string `json:"to,omitempty"`
	FromMe      bool   `json:"fromMe"`
	Timestamp   int64  `json:"timestamp"`
	Body        string `json:"body,omitempty"`
	HasMedia    bool   `json:"hasMedia"`
	MediaType   string `json:"mediaType,omitempty"`
	MediaURL    string `json:"mediaUrl,omitempty"`
	Ack         int    `json:"ack"`
	Participant string `json:"participant,omitempty"`
}

// =============================================================================
// WAHA Webhook Types
// =============================================================================

// wahaWebhookPayload represents an incoming WAHA webhook payload.
type wahaWebhookPayload struct {
	Event   string           `json:"event"`
	Session string           `json:"session"`
	Engine  string           `json:"engine,omitempty"`
	Payload wahaEventPayload `json:"payload"`
}

// wahaEventPayload represents the event-specific payload in webhooks.
// This uses an embedded struct approach to handle different event types.
type wahaEventPayload struct {
	// Common fields for message events
	ID          string                 `json:"id,omitempty"`
	Timestamp   int64                  `json:"timestamp,omitempty"`
	From        string                 `json:"from,omitempty"`
	To          string                 `json:"to,omitempty"`
	Body        string                 `json:"body,omitempty"`
	HasMedia    bool                   `json:"hasMedia,omitempty"`
	MediaType   string                 `json:"mediaType,omitempty"`
	Ack         int                    `json:"ack,omitempty"`
	AckName     string                 `json:"ackName,omitempty"`
	IsForwarded bool                   `json:"isForwarded,omitempty"`
	FromMe      bool                   `json:"fromMe,omitempty"`
	Participant string                 `json:"participant,omitempty"`
	Media       map[string]interface{} `json:"media,omitempty"`

	// Session status event fields
	Status string `json:"status,omitempty"`

	// Additional raw fields for flexibility
	Raw map[string]interface{} `json:"-"`
}

// wahaMessagePayload represents a message payload in webhooks (alias for clarity).
// This is used when parsing message-specific events.
type wahaMessagePayload struct {
	ID          string                 `json:"id"`
	Timestamp   int64                  `json:"timestamp"`
	From        string                 `json:"from"`
	To          string                 `json:"to"`
	Body        string                 `json:"body"`
	HasMedia    bool                   `json:"hasMedia"`
	MediaType   string                 `json:"mediaType,omitempty"`
	Ack         int                    `json:"ack"`
	AckName     string                 `json:"ackName,omitempty"`
	IsForwarded bool                   `json:"isForwarded"`
	FromMe      bool                   `json:"fromMe"`
	Participant string                 `json:"participant,omitempty"`
	Media       map[string]interface{} `json:"media,omitempty"`
}

// =============================================================================
// WAHA Video/File Message Types
// =============================================================================

// wahaSendVideoRequest represents a WAHA video message request.
// Maps to POST /api/sendVideo request body.
type wahaSendVideoRequest struct {
	Session string    `json:"session"`
	ChatID  string    `json:"chatId"`
	Caption string    `json:"caption,omitempty"`
	File    *wahaFile `json:"file,omitempty"`
}

// wahaSendFileRequest represents a WAHA file/document message request.
// Maps to POST /api/sendFile request body.
type wahaSendFileRequest struct {
	Session string    `json:"session"`
	ChatID  string    `json:"chatId"`
	Caption string    `json:"caption,omitempty"`
	File    *wahaFile `json:"file,omitempty"`
}

// =============================================================================
// WAHA HMAC Configuration Types
// =============================================================================

// wahaHMACConfig represents HMAC configuration for webhooks.
// Maps to HmacConfiguration schema in the WAHA OpenAPI spec.
type wahaHMACConfig struct {
	Key string `json:"key"`
}

// =============================================================================
// WAHA Error Types
// =============================================================================

// wahaErrorResponse represents a WAHA API error response.
type wahaErrorResponse struct {
	Error   string `json:"error,omitempty"`
	Message string `json:"message,omitempty"`
	Code    int    `json:"code,omitempty"`
}

// =============================================================================
// WAHA Status Mapping
// =============================================================================

// wahaStatusMap maps WAHA status strings to domain SessionStatus values.
var wahaStatusMap = map[string]SessionStatus{
	"CONNECTED":    SessionStatusConnected,
	"WORKING":      SessionStatusConnected,
	"SCAN_QR_CODE": SessionStatusScanQR,
	"SCAN_QR":      SessionStatusScanQR,
	"STARTING":     SessionStatusStarting,
	"STOPPED":      SessionStatusStopped,
	"FAILED":       SessionStatusFailed,
	"DISCONNECTED": SessionStatusDisconnected,
}

// parseWahaStatus converts a WAHA status string to a domain SessionStatus.
func parseWahaStatus(status string) SessionStatus {
	if s, ok := wahaStatusMap[strings.ToUpper(status)]; ok {
		return s
	}
	return SessionStatusDisconnected
}

// =============================================================================
// WAHA Timestamp Helpers
// =============================================================================

// wahaTimestampToTime converts a WAHA Unix timestamp (seconds) to time.Time.
func wahaTimestampToTime(ts int64) time.Time {
	if ts == 0 {
		return time.Time{}
	}
	return time.Unix(ts, 0)
}
