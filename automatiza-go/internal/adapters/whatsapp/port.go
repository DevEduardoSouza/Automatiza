// Package whatsapp provides the port (interface) for WhatsApp provider adapters.
// This file defines domain-neutral contracts that decouple business logic from
// specific WhatsApp API implementations (e.g., WAHA Plus).
//
// IMPORTANT: This is a PORT - no implementation-specific types should appear here.
// All types defined here are domain-neutral and can be used with any WhatsApp provider.
package whatsapp

import (
	"context"
	"time"
)

// WhatsAppProvider defines the contract for WhatsApp API adapters.
// Implementations must translate between these domain-neutral types and
// provider-specific formats (e.g., WAHA Plus).
//
// This interface follows the Ports and Adapters (Hexagonal) architecture pattern,
// allowing the business logic to remain decoupled from specific WhatsApp providers.
type WhatsAppProvider interface {
	// Session Management
	SessionManager

	// Group Operations
	GroupManager

	// Messaging
	MessageSender

	// Webhook Configuration
	WebhookManager

	// Reconciliation (for backfill and gap recovery)
	MessageReconciler

	// Media Operations
	MediaDownloader
}

// MediaDownloader handles media download operations.
type MediaDownloader interface {
	// DownloadMedia downloads media content from a WhatsApp message.
	// The mediaID should be the media identifier from the message.
	// Returns the media data including bytes, mimetype, and filename.
	DownloadMedia(ctx context.Context, input DownloadMediaInput) (*DownloadMediaResult, error)

	// DownloadMediaByURL downloads media from an internal WAHA file URL.
	// The URL path is extracted and rewritten to use the adapter's configured
	// API base URL and API key for authentication.
	// This is needed because WAHA webhook media.url contains an internal URL
	// (e.g. http://localhost:3000/api/files/...) that requires auth.
	DownloadMediaByURL(ctx context.Context, mediaURL string) (*DownloadMediaResult, error)

	// RedownloadMedia re-downloads media from WhatsApp servers for a specific message.
	// Uses GET /api/{session}/chats/{chatId}/messages/{messageId}?downloadMedia=true
	// to force a fresh download from WhatsApp servers (not the local WAHA cache).
	// This is useful when the local WAHA media file has expired (WHATSAPP_FILES_LIFETIME).
	RedownloadMedia(ctx context.Context, sessionName, chatID, messageID string) (*DownloadMediaResult, error)
}

// DownloadMediaResult contains the downloaded media data with metadata.
type DownloadMediaResult struct {
	// Data is the raw media bytes.
	Data []byte

	// Mimetype is the MIME type of the media (e.g., "image/jpeg", "audio/ogg").
	Mimetype string

	// Filename is the original filename if available from the provider.
	Filename string
}

// SessionManager handles WhatsApp session lifecycle operations.
type SessionManager interface {
	// CreateSession creates a new WhatsApp session with the specified configuration.
	// Returns the created session info or an error if creation fails.
	CreateSession(ctx context.Context, input CreateSessionInput) (*SessionInfo, error)

	// DeleteSession permanently removes a WhatsApp session.
	// This operation is destructive and cannot be undone.
	DeleteSession(ctx context.Context, sessionName string) error

	// GetSessionStatus retrieves the current status of a WhatsApp session.
	// Returns session info including connection state and authenticated user details.
	GetSessionStatus(ctx context.Context, sessionName string) (*SessionInfo, error)

	// GetQRCode retrieves the QR code for session authentication.
	// Returns nil if the session is already authenticated or QR is not available.
	GetQRCode(ctx context.Context, sessionName string) (*QRCodeData, error)

	// RestartSession restarts an existing WhatsApp session.
	// Useful for recovering from connection issues without full re-authentication.
	RestartSession(ctx context.Context, sessionName string) error

	// LogoutSession logs out the WhatsApp session, requiring re-authentication via QR.
	// This does not delete the session configuration, only disconnects the device.
	LogoutSession(ctx context.Context, sessionName string) error

	// ClearStatusCache removes a session from the status cache.
	// Used to force a fresh status fetch from the provider (e.g., when refreshing QR code).
	ClearStatusCache(sessionName string)
}

// GroupManager handles WhatsApp group operations.
type GroupManager interface {
	// GetGroups retrieves all groups for a session.
	// Returns a list of groups the authenticated user is a member of.
	GetGroups(ctx context.Context, sessionName string) ([]GroupInfo, error)

	// GetGroupParticipants retrieves participants for a specific group.
	// Returns participant details including admin status.
	GetGroupParticipants(ctx context.Context, sessionName, groupID string) ([]ParticipantInfo, error)

	// GetGroupInviteCode retrieves the invite code for a specific group.
	// Returns the invite code string that can be used to generate invite links.
	// Requires admin privileges in the group.
	GetGroupInviteCode(ctx context.Context, sessionName, groupID string) (string, error)

	// UpdateGroupSubject updates the group name/subject.
	// Requires admin privileges in the group.
	UpdateGroupSubject(ctx context.Context, input UpdateGroupSubjectInput) error

	// UpdateGroupDescription updates the group description.
	// Requires admin privileges in the group.
	UpdateGroupDescription(ctx context.Context, input UpdateGroupDescriptionInput) error

	// UpdateGroupPicture updates the group profile picture.
	// Requires admin privileges in the group. Image should be base64 encoded.
	UpdateGroupPicture(ctx context.Context, input UpdateGroupPictureInput) error

	// SetGroupAnnouncement toggles announcement mode (admin-only messages).
	// When enabled, only admins can send messages to the group.
	SetGroupAnnouncement(ctx context.Context, input SetGroupAnnouncementInput) error

	// AddGroupMembers adds one or more members to a group.
	// Requires admin privileges. Returns a result with success/failure per number.
	AddGroupMembers(ctx context.Context, input AddGroupMembersInput) (*MemberOperationResult, error)

	// RemoveGroupMembers removes one or more members from a group.
	// Requires admin privileges. Returns a result with success/failure per number.
	RemoveGroupMembers(ctx context.Context, input RemoveGroupMembersInput) (*MemberOperationResult, error)

	// LeaveGroup makes the authenticated user leave a group.
	LeaveGroup(ctx context.Context, input LeaveGroupInput) error

	// PromoteMember promotes a group member to admin.
	// Requires admin privileges.
	PromoteMember(ctx context.Context, input PromoteMemberInput) error

	// DemoteMember demotes a group admin to regular member.
	// Requires admin privileges.
	DemoteMember(ctx context.Context, input DemoteMemberInput) error
}

// MessageSender handles sending messages via WhatsApp.
type MessageSender interface {
	// SendText sends a text message to a chat (group or individual).
	// Supports optional mentions for group messages.
	SendText(ctx context.Context, input SendTextInput) (*MessageResult, error)

	// SendImage sends an image message with optional caption.
	// Image can be provided as base64 data or URL.
	SendImage(ctx context.Context, input SendImageInput) (*MessageResult, error)

	// SendVideo sends a video message with optional caption.
	// Video can be provided as base64 data or URL.
	SendVideo(ctx context.Context, input SendVideoInput) (*MessageResult, error)

	// SendDocument sends a file/document message with optional caption.
	// Document can be provided as base64 data or URL.
	SendDocument(ctx context.Context, input SendDocumentInput) (*MessageResult, error)

	// SendVoice sends a voice message (PTT - Push to Talk).
	// Audio should be base64 encoded with appropriate mimetype.
	SendVoice(ctx context.Context, input SendVoiceInput) (*MessageResult, error)

	// SendPoll sends a poll message to a chat.
	// Polls support 2-12 options with configurable selection count.
	SendPoll(ctx context.Context, input SendPollInput) (*MessageResult, error)

	// SendLinkCustomPreview sends a message with a custom link preview card.
	// Used for controlling the preview title, description, and image shown in WhatsApp.
	SendLinkCustomPreview(ctx context.Context, input SendLinkPreviewInput) (*MessageResult, error)
}

// WebhookManager handles webhook configuration for receiving WhatsApp events.
type WebhookManager interface {
	// SetWebhook configures the webhook URL and events for a session.
	// This determines which events are pushed to the application.
	SetWebhook(ctx context.Context, input SetWebhookInput) error
}

// MessageReconciler handles message retrieval for reconciliation and backfill.
type MessageReconciler interface {
	// GetMessages retrieves messages from a chat for reconciliation purposes.
	// Supports pagination and filtering by timestamp for gap recovery.
	GetMessages(ctx context.Context, input GetMessagesInput) (*MessagesResult, error)
}

// -----------------------------------------------------------------------------
// Session Types
// -----------------------------------------------------------------------------

// CreateSessionInput contains parameters for creating a new WhatsApp session.
type CreateSessionInput struct {
	// SessionName is the unique identifier for the session.
	SessionName string

	// WebhookURL is the endpoint to receive WhatsApp events.
	WebhookURL string

	// WebhookEvents specifies which events to receive (e.g., "message", "message.ack").
	// Empty slice means receive all events.
	WebhookEvents []string

	// HMACKey is the secret key for webhook payload HMAC validation.
	// If empty, HMAC validation is disabled.
	HMACKey string

	// Engine specifies storage configuration for the session.
	Engine EngineConfig
}

// EngineConfig defines storage behavior for WhatsApp session data.
type EngineConfig struct {
	// StorageMessages enables message storage in the session.
	StorageMessages bool

	// StorageGroups enables group metadata storage.
	StorageGroups bool

	// StorageChats enables chat list storage.
	StorageChats bool

	// StorageLabels enables label storage.
	StorageLabels bool
}

// SessionInfo represents the current state of a WhatsApp session.
type SessionInfo struct {
	// Name is the session identifier.
	Name string

	// Status indicates the connection state (e.g., "CONNECTED", "DISCONNECTED", "SCAN_QR").
	Status SessionStatus

	// AuthenticatedUser contains details of the logged-in WhatsApp account.
	// Nil if not yet authenticated.
	AuthenticatedUser *AuthenticatedUser
}

// SessionStatus represents the connection state of a WhatsApp session.
type SessionStatus string

const (
	// SessionStatusConnected indicates the session is fully connected and operational.
	SessionStatusConnected SessionStatus = "CONNECTED"

	// SessionStatusDisconnected indicates the session is not connected.
	SessionStatusDisconnected SessionStatus = "DISCONNECTED"

	// SessionStatusScanQR indicates the session is waiting for QR code scan.
	SessionStatusScanQR SessionStatus = "SCAN_QR"

	// SessionStatusStarting indicates the session is initializing.
	SessionStatusStarting SessionStatus = "STARTING"

	// SessionStatusStopped indicates the session has been stopped.
	SessionStatusStopped SessionStatus = "STOPPED"

	// SessionStatusFailed indicates the session encountered an error.
	SessionStatusFailed SessionStatus = "FAILED"
)

// AuthenticatedUser contains information about the logged-in WhatsApp account.
type AuthenticatedUser struct {
	// ID is the WhatsApp ID (phone number with suffix).
	ID string

	// PushName is the display name of the user.
	PushName string
}

// QRCodeData contains QR code information for session authentication.
type QRCodeData struct {
	// Value is the QR code content (for rendering as QR image).
	Value string

	// Base64Image is the QR code as a base64-encoded image (optional).
	Base64Image string

	// Mimetype is the image mimetype (e.g., "image/png").
	Mimetype string
}

// -----------------------------------------------------------------------------
// Group Types
// -----------------------------------------------------------------------------

// GroupInfo represents a WhatsApp group.
type GroupInfo struct {
	// ID is the WhatsApp group ID (e.g., "123456789@g.us").
	ID string

	// Subject is the group name/title.
	Subject string

	// Description is the group description text.
	Description string

	// Owner is the WhatsApp ID of the group creator.
	Owner string

	// CreatedAt is the group creation timestamp.
	CreatedAt time.Time

	// Announce indicates if only admins can send messages.
	Announce bool

	// Restrict indicates if only admins can edit group info.
	Restrict bool

	// Size is the number of participants in the group.
	Size int

	// Participants contains group member details (may be empty if not requested).
	Participants []ParticipantInfo

	// IsAdmin indicates if the authenticated session user is an admin of this group.
	// Populated by GOWS engine at the group level.
	IsAdmin bool

	// IsSuperAdmin indicates if the authenticated session user is a super admin.
	// Populated by GOWS engine at the group level.
	IsSuperAdmin bool
}

// ParticipantInfo represents a group participant.
type ParticipantInfo struct {
	// ID is the participant's WhatsApp ID.
	ID string

	// PhoneNumber is the participant's phone number (e.g., "5511999999999").
	// Extracted from the pn field (WAHA v2) or derived from the ID.
	PhoneNumber string

	// IsAdmin indicates if the participant is a group admin.
	IsAdmin bool

	// IsSuperAdmin indicates if the participant is the group owner/superadmin.
	IsSuperAdmin bool
}

// UpdateGroupSubjectInput contains parameters for updating group subject.
type UpdateGroupSubjectInput struct {
	// SessionName is the session to use.
	SessionName string

	// GroupID is the WhatsApp group ID.
	GroupID string

	// Subject is the new group name/title.
	Subject string
}

// UpdateGroupDescriptionInput contains parameters for updating group description.
type UpdateGroupDescriptionInput struct {
	// SessionName is the session to use.
	SessionName string

	// GroupID is the WhatsApp group ID.
	GroupID string

	// Description is the new group description text.
	Description string
}

// UpdateGroupPictureInput contains parameters for updating group picture.
type UpdateGroupPictureInput struct {
	// SessionName is the session to use.
	SessionName string

	// GroupID is the WhatsApp group ID.
	GroupID string

	// ImageBase64 is the new group picture as base64-encoded data.
	ImageBase64 string
}

// SetGroupAnnouncementInput contains parameters for setting announcement mode.
type SetGroupAnnouncementInput struct {
	// SessionName is the session to use.
	SessionName string

	// GroupID is the WhatsApp group ID.
	GroupID string

	// Announce enables or disables announcement mode (admin-only messages).
	Announce bool
}

// AddGroupMembersInput contains parameters for adding members to a group.
type AddGroupMembersInput struct {
	// SessionName is the session to use.
	SessionName string

	// GroupID is the WhatsApp group ID.
	GroupID string

	// PhoneNumbers is the list of phone numbers to add (E.164 without +).
	PhoneNumbers []string
}

// RemoveGroupMembersInput contains parameters for removing members from a group.
type RemoveGroupMembersInput struct {
	// SessionName is the session to use.
	SessionName string

	// GroupID is the WhatsApp group ID.
	GroupID string

	// PhoneNumbers is the list of phone numbers to remove (E.164 without +).
	PhoneNumbers []string
}

// LeaveGroupInput contains parameters for leaving a group.
type LeaveGroupInput struct {
	// SessionName is the session to use.
	SessionName string

	// GroupID is the WhatsApp group ID.
	GroupID string
}

// PromoteMemberInput contains parameters for promoting a member to admin.
type PromoteMemberInput struct {
	// SessionName is the session to use.
	SessionName string

	// GroupID is the WhatsApp group ID.
	GroupID string

	// PhoneNumber is the phone number of the member to promote (E.164 without +).
	PhoneNumber string
}

// DemoteMemberInput contains parameters for demoting a member from admin.
type DemoteMemberInput struct {
	// SessionName is the session to use.
	SessionName string

	// GroupID is the WhatsApp group ID.
	GroupID string

	// PhoneNumber is the phone number of the member to demote (E.164 without +).
	PhoneNumber string
}

// MemberOperationResult contains the result of a member add/remove operation.
type MemberOperationResult struct {
	// SuccessCount is the number of successfully processed members.
	SuccessCount int

	// FailedNumbers is the list of phone numbers that failed.
	FailedNumbers []string
}

// -----------------------------------------------------------------------------
// Message Types
// -----------------------------------------------------------------------------

// SendTextInput contains parameters for sending a text message.
type SendTextInput struct {
	// SessionName is the session to use.
	SessionName string

	// ChatID is the recipient (group or individual WhatsApp ID).
	ChatID string

	// Text is the message content.
	Text string

	// Mentions is a list of WhatsApp IDs to mention (for group messages).
	Mentions []string
}

// SendImageInput contains parameters for sending an image message.
type SendImageInput struct {
	// SessionName is the session to use.
	SessionName string

	// ChatID is the recipient (group or individual WhatsApp ID).
	ChatID string

	// Caption is the optional image caption.
	Caption string

	// ImageBase64 is the image as base64-encoded data.
	// Either ImageBase64 or ImageURL must be provided.
	ImageBase64 string

	// ImageURL is the URL to fetch the image from.
	// Either ImageBase64 or ImageURL must be provided.
	ImageURL string

	// Mimetype is the image MIME type (e.g., "image/jpeg").
	Mimetype string

	// Filename is the optional filename for the image.
	Filename string

	// Mentions is a list of WhatsApp IDs to mention (for group messages).
	// Drives contextInfo.mentionedJid so participants are notified even when
	// their number is not written in the caption (e.g. @todos ghost mention).
	Mentions []string
}

// SendVoiceInput contains parameters for sending a voice message.
type SendVoiceInput struct {
	// SessionName is the session to use.
	SessionName string

	// ChatID is the recipient (group or individual WhatsApp ID).
	ChatID string

	// AudioBase64 is the audio as base64-encoded data.
	AudioBase64 string

	// Mimetype is the audio MIME type (e.g., "audio/ogg; codecs=opus").
	Mimetype string
}

// SendPollInput contains parameters for sending a poll message.
type SendPollInput struct {
	// SessionName is the session to use.
	SessionName string

	// ChatID is the recipient (group or individual WhatsApp ID).
	ChatID string

	// Question is the poll question/title.
	Question string

	// Options is the list of poll options (2-12 options).
	Options []string

	// SelectableCount is the maximum number of options a user can select.
	// Must be >= 1 and <= len(Options).
	SelectableCount int
}

// SendLinkPreviewInput contains parameters for sending a message with custom link preview.
type SendLinkPreviewInput struct {
	// SessionName is the session to use.
	SessionName string

	// ChatID is the recipient (group or individual WhatsApp ID).
	ChatID string

	// Text is the message content. Must contain the URL from the preview.
	Text string

	// URL is the URL to show in the preview card.
	URL string

	// Title is the preview card title.
	Title string

	// Description is the preview card description (e.g., price).
	Description string

	// ImageURL is the image to display in the preview card.
	ImageURL string

	// ImageBase64 is the image as base64-encoded data (alternative to ImageURL).
	ImageBase64 string

	// Mimetype is the MIME type for the base64 image (e.g., "image/jpeg").
	Mimetype string
}

// MessageResult contains the result of a sent message.
type MessageResult struct {
	// ID is the unique message identifier.
	ID string

	// Timestamp is when the message was sent.
	Timestamp time.Time
}

// -----------------------------------------------------------------------------
// Webhook Types
// -----------------------------------------------------------------------------

// SetWebhookInput contains parameters for configuring webhooks.
type SetWebhookInput struct {
	// SessionName is the session to configure.
	SessionName string

	// URL is the webhook endpoint URL.
	URL string

	// Events is the list of events to subscribe to.
	// Empty means subscribe to all events.
	Events []string

	// HMACKey is the secret for HMAC payload validation.
	HMACKey string
}

// -----------------------------------------------------------------------------
// Reconciliation Types
// -----------------------------------------------------------------------------

// GetMessagesInput contains parameters for message retrieval.
type GetMessagesInput struct {
	// SessionName is the session to query.
	SessionName string

	// ChatID is the chat to retrieve messages from.
	ChatID string

	// Limit is the maximum number of messages to retrieve.
	Limit int

	// Offset is the number of messages to skip (for pagination).
	Offset int

	// DownloadMedia indicates whether to include media download URLs.
	DownloadMedia bool

	// Filter contains optional filtering criteria.
	Filter *MessageFilter
}

// MessageFilter defines criteria for filtering messages.
type MessageFilter struct {
	// FromMe filters messages sent by the authenticated user.
	// nil means no filter, true means only sent messages, false means only received.
	FromMe *bool

	// FromTimestamp filters messages after this timestamp (inclusive).
	FromTimestamp *time.Time

	// ToTimestamp filters messages before this timestamp (inclusive).
	ToTimestamp *time.Time
}

// MessagesResult contains the result of a message retrieval operation.
type MessagesResult struct {
	// Messages is the list of retrieved messages.
	Messages []Message

	// HasMore indicates if there are more messages available.
	HasMore bool

	// NextOffset is the offset for the next page (if HasMore is true).
	NextOffset int
}

// Message represents a WhatsApp message.
type Message struct {
	// ID is the unique message identifier.
	ID string

	// ChatID is the chat the message belongs to.
	ChatID string

	// FromMe indicates if the message was sent by the authenticated user.
	FromMe bool

	// Sender is the WhatsApp ID of the message sender.
	// For group messages, this is the participant ID.
	Sender string

	// Timestamp is when the message was sent/received.
	Timestamp time.Time

	// Body is the text content of the message.
	Body string

	// HasMedia indicates if the message contains media.
	HasMedia bool

	// MediaType is the type of media (e.g., "image", "audio", "video", "document").
	MediaType string

	// MediaURL is the URL to download the media (if DownloadMedia was true).
	MediaURL string

	// Ack is the acknowledgment status (0=pending, 1=sent, 2=delivered, 3=read).
	Ack int
}

// AckStatus represents message acknowledgment states.
type AckStatus int

const (
	// AckPending indicates the message is pending to be sent.
	AckPending AckStatus = 0

	// AckSent indicates the message was sent to the server.
	AckSent AckStatus = 1

	// AckDelivered indicates the message was delivered to the recipient.
	AckDelivered AckStatus = 2

	// AckRead indicates the message was read by the recipient.
	AckRead AckStatus = 3
)

// -----------------------------------------------------------------------------
// Media Types
// -----------------------------------------------------------------------------

// DownloadMediaInput contains parameters for downloading media.
type DownloadMediaInput struct {
	// SessionName is the session to use.
	SessionName string

	// MediaID is the unique identifier for the media to download.
	// This is typically extracted from message metadata.
	MediaID string
}

// -----------------------------------------------------------------------------
// Error Types
// -----------------------------------------------------------------------------

// ProviderError represents an error from the WhatsApp provider.
type ProviderError struct {
	// Code is a provider-agnostic error code.
	Code ErrorCode

	// Message is a human-readable error description.
	Message string

	// Details contains additional error context.
	Details map[string]any

	// Cause is the underlying error (if any).
	Cause error
}

// Error implements the error interface.
func (e *ProviderError) Error() string {
	if e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	return e.Message
}

// Unwrap returns the underlying error.
func (e *ProviderError) Unwrap() error {
	return e.Cause
}

// ErrorCode represents provider-agnostic error codes.
type ErrorCode string

const (
	// ErrCodeSessionNotFound indicates the session does not exist.
	ErrCodeSessionNotFound ErrorCode = "SESSION_NOT_FOUND"

	// ErrCodeSessionNotConnected indicates the session is not connected.
	ErrCodeSessionNotConnected ErrorCode = "SESSION_NOT_CONNECTED"

	// ErrCodeGroupNotFound indicates the group does not exist.
	ErrCodeGroupNotFound ErrorCode = "GROUP_NOT_FOUND"

	// ErrCodePermissionDenied indicates insufficient permissions.
	ErrCodePermissionDenied ErrorCode = "PERMISSION_DENIED"

	// ErrCodeInvalidInput indicates invalid input parameters.
	ErrCodeInvalidInput ErrorCode = "INVALID_INPUT"

	// ErrCodeRateLimited indicates the request was rate limited.
	ErrCodeRateLimited ErrorCode = "RATE_LIMITED"

	// ErrCodeCircuitBreakerOpen indicates the circuit breaker is open and blocking requests.
	ErrCodeCircuitBreakerOpen ErrorCode = "CIRCUIT_BREAKER_OPEN"

	// ErrCodeProviderError indicates a provider-specific error.
	ErrCodeProviderError ErrorCode = "PROVIDER_ERROR"

	// ErrCodeTimeout indicates the operation timed out.
	ErrCodeTimeout ErrorCode = "TIMEOUT"

	// ErrCodeNetworkError indicates a network connectivity issue.
	ErrCodeNetworkError ErrorCode = "NETWORK_ERROR"
)

// NewProviderError creates a new ProviderError.
func NewProviderError(code ErrorCode, message string, cause error) *ProviderError {
	return &ProviderError{
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}

// WithDetails adds details to the error.
func (e *ProviderError) WithDetails(details map[string]any) *ProviderError {
	e.Details = details
	return e
}
