// Package whatsapp provides the WhatsApp adapter abstraction layer.
// This file contains mappers for converting between WAHA-specific types
// and domain-neutral types defined in domain_models.go and port.go.
//
// IMPORTANT: All conversions between WAHA API types and domain types MUST go through
// this mapper. This ensures the adapter remains the only place that knows about WAHA specifics.
package whatsapp

import (
	"strings"
	"time"

	"go.uber.org/zap"
)

// mapperLogger is a package-level logger for the mapper.
// Initialized lazily; callers in this package that need structured logging
// should use this instead of the standard log package.
var mapperLogger = zap.NewNop()

// SetMapperLogger configures the structured logger for mapper functions.
func SetMapperLogger(l *zap.Logger) {
	if l != nil {
		mapperLogger = l.Named("mapper")
	}
}

// Mapper handles conversion between WAHA types and domain types.
// Using a struct allows for future extension (e.g., adding configuration or logging).
type Mapper struct{}

// NewMapper creates a new Mapper instance.
func NewMapper() *Mapper {
	return &Mapper{}
}

// newMapperWithLogger creates a new Mapper and sets the package-level structured logger.
func newMapperWithLogger(logger *zap.Logger) *Mapper {
	SetMapperLogger(logger)
	return &Mapper{}
}

// stripDataURIPrefix removes the "data:...;base64," prefix from a base64 string if present.
func stripDataURIPrefix(data string) string {
	if idx := strings.Index(data, ";base64,"); idx != -1 {
		prefix := data[:idx+8]
		stripped := data[idx+8:]
		mapperLogger.Debug("stripDataURIPrefix: found prefix",
			zap.String("prefix", prefix),
			zap.Int("strippedLen", len(stripped)))
		return stripped
	}
	mapperLogger.Debug("stripDataURIPrefix: no prefix found",
		zap.Int("dataLen", len(data)))
	return data
}

// =============================================================================
// Session Mapping
// =============================================================================

// SessionFromWaha converts a WAHA session response to a domain Session.
func (m *Mapper) SessionFromWaha(w *wahaSession) *Session {
	if w == nil {
		return nil
	}

	session := &Session{
		Name:   w.Name,
		Status: parseWahaStatus(w.Status),
	}

	if w.Me != nil {
		session.Me = &SessionMe{
			ID:       w.Me.ID,
			Phone:    w.Me.Phone,
			PushName: w.Me.PushName,
		}
	}

	if w.Config != nil {
		session.Config = &SessionConfig{
			ProxyURL:  w.Config.ProxyURL,
			DebugMode: w.Config.DebugMode,
		}
		for _, wh := range w.Config.Webhooks {
			entry := WebhookConfigEntry{
				URL:    wh.URL,
				Events: wh.Events,
			}
			if wh.HMAC != nil {
				entry.HMACSecret = wh.HMAC.Key
			}
			session.Config.Webhooks = append(session.Config.Webhooks, entry)
		}
	}

	return session
}

// SessionInfoFromWaha converts a WAHA session response to a domain SessionInfo (port type).
func (m *Mapper) SessionInfoFromWaha(w *wahaSession) *SessionInfo {
	if w == nil {
		return nil
	}

	info := &SessionInfo{
		Name:   w.Name,
		Status: parseWahaStatus(w.Status),
	}

	if w.Me != nil {
		info.AuthenticatedUser = &AuthenticatedUser{
			ID:       w.Me.ID,
			PushName: w.Me.PushName,
		}
	}

	return info
}

// =============================================================================
// QR Code Mapping
// =============================================================================

// QRCodeFromWaha converts a WAHA QR code response to a domain QRCode.
func (m *Mapper) QRCodeFromWaha(w *wahaQRCode) *QRCode {
	if w == nil {
		return nil
	}

	return &QRCode{
		Value:    w.Value,
		MimeType: w.Mimetype,
		// Raw is the same as Value for WAHA
		Raw: w.Value,
	}
}

// QRCodeDataFromWaha converts a WAHA QR code response to a domain QRCodeData (port type).
func (m *Mapper) QRCodeDataFromWaha(w *wahaQRCode) *QRCodeData {
	if w == nil {
		return nil
	}

	return &QRCodeData{
		Value:       w.Value,
		Base64Image: w.Value,
		Mimetype:    w.Mimetype,
	}
}

// =============================================================================
// Group Mapping
// =============================================================================

// GroupFromWaha converts a WAHA group response to a domain Group.
func (m *Mapper) GroupFromWaha(w *wahaGroup) *Group {
	if w == nil {
		return nil
	}

	group := &Group{
		ID:                w.ID,
		Name:              w.Subject,
		Description:       w.Description,
		Picture:           w.Picture,
		Owner:             w.Owner,
		AdminOnly:         w.Announce,
		InviteCode:        w.InviteCode,
		IsAdmin:           w.IsAdmin,
		IsSuperAdmin:      w.IsSuperAdmin,
		ParticipantsCount: w.Size,
	}

	// Convert creation timestamp
	if w.Creation > 0 {
		t := wahaTimestampToTime(w.Creation)
		group.CreatedAt = &t
	}

	// Convert participants
	group.Participants = m.ParticipantsFromWaha(w.Participants)

	// Update count if we have actual participants
	if len(group.Participants) > 0 && group.ParticipantsCount == 0 {
		group.ParticipantsCount = len(group.Participants)
	}

	return group
}

// GroupsFromWaha converts a slice of WAHA groups to domain Groups.
func (m *Mapper) GroupsFromWaha(w []wahaGroup) []Group {
	if w == nil {
		return nil
	}

	groups := make([]Group, 0, len(w))
	for i := range w {
		if g := m.GroupFromWaha(&w[i]); g != nil {
			groups = append(groups, *g)
		}
	}
	return groups
}

// GroupInfoFromWaha converts a WAHA group response to a domain GroupInfo (port type).
func (m *Mapper) GroupInfoFromWaha(w *wahaGroup) GroupInfo {
	if w == nil {
		return GroupInfo{}
	}

	info := GroupInfo{
		ID:           w.ID,
		Subject:      w.Subject,
		Description:  w.Description,
		Owner:        w.Owner,
		Announce:     w.Announce,
		Restrict:     w.Restrict,
		Size:         w.Size,
		IsAdmin:      w.IsAdmin,
		IsSuperAdmin: w.IsSuperAdmin,
	}

	// Convert creation timestamp
	if w.Creation > 0 {
		info.CreatedAt = wahaTimestampToTime(w.Creation)
	}

	// Convert participants
	info.Participants = m.ParticipantInfosFromWaha(w.Participants)

	// Update size if we have actual participants
	if len(info.Participants) > 0 && info.Size == 0 {
		info.Size = len(info.Participants)
	}

	return info
}

// GroupInfosFromWaha converts a slice of WAHA groups to domain GroupInfos (port type).
func (m *Mapper) GroupInfosFromWaha(w []wahaGroup) []GroupInfo {
	if w == nil {
		return nil
	}

	groups := make([]GroupInfo, 0, len(w))
	for i := range w {
		groups = append(groups, m.GroupInfoFromWaha(&w[i]))
	}
	return groups
}

// =============================================================================
// Participant Mapping
// =============================================================================

// ParticipantFromWaha converts a WAHA participant to a domain Participant.
func (m *Mapper) ParticipantFromWaha(w *wahaParticipant) *Participant {
	if w == nil {
		return nil
	}

	return &Participant{
		ID:           w.ID,
		Phone:        ExtractPhoneFromID(w.ID),
		IsAdmin:      w.IsAdmin,
		IsSuperAdmin: w.IsSuperAdmin,
	}
}

// ParticipantsFromWaha converts a slice of WAHA participants to domain Participants.
func (m *Mapper) ParticipantsFromWaha(w []wahaParticipant) []Participant {
	if w == nil {
		return nil
	}

	participants := make([]Participant, 0, len(w))
	for i := range w {
		if p := m.ParticipantFromWaha(&w[i]); p != nil {
			participants = append(participants, *p)
		}
	}
	return participants
}

// ParticipantInfoFromWaha converts a WAHA participant to a domain ParticipantInfo (port type).
func (m *Mapper) ParticipantInfoFromWaha(w *wahaParticipant) ParticipantInfo {
	if w == nil {
		return ParticipantInfo{}
	}

	phoneNumber := w.PhoneNumber
	if phoneNumber == "" {
		phoneNumber = ExtractPhoneFromID(w.ID)
	} else {
		// PhoneNumber may come as "5511999999999@c.us", strip the suffix
		phoneNumber = ExtractPhoneFromID(phoneNumber)
	}

	return ParticipantInfo{
		ID:           w.ID,
		PhoneNumber:  phoneNumber,
		IsAdmin:      w.IsAdmin,
		IsSuperAdmin: w.IsSuperAdmin,
	}
}

// ParticipantInfosFromWaha converts a slice of WAHA participants to domain ParticipantInfos.
func (m *Mapper) ParticipantInfosFromWaha(w []wahaParticipant) []ParticipantInfo {
	if w == nil {
		return nil
	}

	participants := make([]ParticipantInfo, 0, len(w))
	for i := range w {
		participants = append(participants, m.ParticipantInfoFromWaha(&w[i]))
	}
	return participants
}

// =============================================================================
// Message Mapping (Domain -> WAHA)
// =============================================================================

// SendTextToWaha converts a domain SendTextInput to a WAHA request.
func (m *Mapper) SendTextToWaha(input *SendTextInput) *wahaSendTextRequest {
	if input == nil {
		return nil
	}

	return &wahaSendTextRequest{
		Session:  input.SessionName,
		ChatID:   input.ChatID,
		Text:     input.Text,
		Mentions: input.Mentions,
	}
}

// SendImageToWaha converts a domain SendImageInput to a WAHA request.
func (m *Mapper) SendImageToWaha(input *SendImageInput) *wahaSendImageRequest {
	if input == nil {
		return nil
	}

	req := &wahaSendImageRequest{
		Session: input.SessionName,
		ChatID:  input.ChatID,
		Caption: input.Caption,
	}

	// Use URL if provided, otherwise use base64 file
	if input.ImageURL != "" {
		mimetype := input.Mimetype
		if mimetype == "" {
			mimetype = "image/jpeg"
		}
		req.File = &wahaFile{
			URL:      input.ImageURL,
			Mimetype: mimetype,
		}
	} else if input.ImageBase64 != "" {
		req.File = &wahaFile{
			Data:     stripDataURIPrefix(input.ImageBase64),
			Mimetype: input.Mimetype,
			Filename: input.Filename,
		}
	}

	return req
}

// SendVoiceToWaha converts a domain SendVoiceInput to a WAHA request.
func (m *Mapper) SendVoiceToWaha(input *SendVoiceInput) *wahaSendVoiceRequest {
	if input == nil {
		return nil
	}

	return &wahaSendVoiceRequest{
		Session: input.SessionName,
		ChatID:  input.ChatID,
		File: wahaVoiceFile{
			Data:     stripDataURIPrefix(input.AudioBase64),
			Mimetype: input.Mimetype,
			Filename: "voice-message.ogg",
		},
		Convert: true,
	}
}

// SendPollToWaha converts a domain SendPollInput to a WAHA request.
func (m *Mapper) SendPollToWaha(input *SendPollInput) *wahaSendPollRequest {
	if input == nil {
		return nil
	}

	return &wahaSendPollRequest{
		Session: input.SessionName,
		ChatID:  input.ChatID,
		Poll: wahaPollData{
			Name:            input.Question,
			Options:         input.Options,
			MultipleAnswers: input.SelectableCount > 1,
		},
	}
}

// SendLinkPreviewToWaha converts a domain SendLinkPreviewInput to a WAHA request.
func (m *Mapper) SendLinkPreviewToWaha(input *SendLinkPreviewInput) *wahaSendLinkPreviewRequest {
	if input == nil {
		return nil
	}

	req := &wahaSendLinkPreviewRequest{
		Session: input.SessionName,
		ChatID:  input.ChatID,
		Text:    input.Text,
		Preview: &wahaSendLinkPreviewPreview{
			URL:         input.URL,
			Title:       input.Title,
			Description: input.Description,
		},
	}

	if input.ImageURL != "" {
		req.Preview.Image = &wahaFile{
			URL: input.ImageURL,
		}
	}

	return req
}

// SendVideoToWaha converts a domain SendVideoInput to a WAHA request.
func (m *Mapper) SendVideoToWaha(input *SendVideoInput) *wahaSendVideoRequest {
	if input == nil {
		return nil
	}

	req := &wahaSendVideoRequest{
		Session: input.SessionName,
		ChatID:  input.ChatID,
		Caption: input.Caption,
	}

	if input.MediaURL != "" {
		req.File = &wahaFile{URL: input.MediaURL}
	} else if input.MediaBase64 != "" {
		req.File = &wahaFile{
			Data:     input.MediaBase64,
			Mimetype: input.MimeType,
			Filename: input.Filename,
		}
	}

	return req
}

// SendDocumentToWaha converts a domain SendDocumentInput to a WAHA request.
func (m *Mapper) SendDocumentToWaha(input *SendDocumentInput) *wahaSendFileRequest {
	if input == nil {
		return nil
	}

	req := &wahaSendFileRequest{
		Session: input.SessionName,
		ChatID:  input.ChatID,
		Caption: input.Caption,
	}

	if input.MediaURL != "" {
		req.File = &wahaFile{URL: input.MediaURL}
	} else if input.MediaBase64 != "" {
		req.File = &wahaFile{
			Data:     input.MediaBase64,
			Mimetype: input.MimeType,
			Filename: input.Filename,
		}
	}

	return req
}

// =============================================================================
// Message Result Mapping (WAHA -> Domain)
// =============================================================================

// MessageResultFromWaha converts a WAHA send response to a domain MessageResult.
func (m *Mapper) MessageResultFromWaha(w *wahaSendMessageResponse) *MessageResult {
	if w == nil {
		return nil
	}

	result := &MessageResult{
		ID: w.ID,
	}

	if w.Timestamp > 0 {
		result.Timestamp = wahaTimestampToTime(w.Timestamp)
	} else {
		result.Timestamp = time.Now()
	}

	return result
}

// =============================================================================
// Message Retrieval Mapping (WAHA -> Domain)
// =============================================================================

// MessageFromWaha converts a WAHA message to a domain Message.
func (m *Mapper) MessageFromWaha(w *wahaMessage) Message {
	if w == nil {
		return Message{}
	}

	msg := Message{
		ID:        w.ID,
		ChatID:    w.ChatID,
		FromMe:    w.FromMe,
		Body:      w.Body,
		HasMedia:  w.HasMedia,
		MediaType: w.MediaType,
		MediaURL:  w.MediaURL,
		Ack:       w.Ack,
		Timestamp: wahaTimestampToTime(w.Timestamp),
	}

	// Determine sender based on fromMe and participant
	if w.FromMe {
		msg.Sender = w.From
	} else if w.Participant != "" {
		// Group message - participant is the actual sender
		msg.Sender = w.Participant
	} else {
		msg.Sender = w.From
	}

	return msg
}

// MessagesFromWaha converts a slice of WAHA messages to domain Messages.
func (m *Mapper) MessagesFromWaha(w []wahaMessage) []Message {
	if w == nil {
		return nil
	}

	messages := make([]Message, 0, len(w))
	for i := range w {
		messages = append(messages, m.MessageFromWaha(&w[i]))
	}
	return messages
}

// MessagesResultFromWaha converts a WAHA messages response to domain MessagesResult.
func (m *Mapper) MessagesResultFromWaha(w *wahaGetMessagesResponse, offset int) *MessagesResult {
	if w == nil {
		return nil
	}

	return &MessagesResult{
		Messages:   m.MessagesFromWaha(w.Messages),
		HasMore:    w.HasMore,
		NextOffset: offset + len(w.Messages),
	}
}

// =============================================================================
// Webhook Mapping
// =============================================================================

// SetWebhookToWahaConfig converts a domain SetWebhookInput to a WAHA webhook config.
// This is used when updating the session via PUT /api/sessions/{session}.
func (m *Mapper) SetWebhookToWahaConfig(input *SetWebhookInput) *wahaWebhookConfig {
	if input == nil {
		return nil
	}

	cfg := &wahaWebhookConfig{
		URL:    input.URL,
		Events: input.Events,
	}

	if input.HMACKey != "" {
		cfg.HMAC = &wahaHMACConfig{
			Key: input.HMACKey,
		}
	}

	return cfg
}

// CreateSessionToWaha converts a domain CreateSessionInput to a WAHA request.
func (m *Mapper) CreateSessionToWaha(input *CreateSessionInput) *wahaCreateSessionRequest {
	if input == nil {
		return nil
	}

	req := &wahaCreateSessionRequest{
		Name:  input.SessionName,
		Start: true,
	}

	// Build config with webhook if provided
	if input.WebhookURL != "" {
		webhookConfig := wahaWebhookConfig{
			URL:    input.WebhookURL,
			Events: input.WebhookEvents,
		}

		if input.HMACKey != "" {
			webhookConfig.HMAC = &wahaHMACConfig{
				Key: input.HMACKey,
			}
		}

		req.Config = &wahaSessionConfig{
			Webhooks: []wahaWebhookConfig{webhookConfig},
		}
	}

	return req
}

// =============================================================================
// Group Update Mapping (Domain -> WAHA)
// =============================================================================

// UpdateGroupSubjectToWaha converts a domain UpdateGroupSubjectInput to a WAHA request.
func (m *Mapper) UpdateGroupSubjectToWaha(input *UpdateGroupSubjectInput) *wahaUpdateGroupSubjectRequest {
	if input == nil {
		return nil
	}

	return &wahaUpdateGroupSubjectRequest{
		Subject: input.Subject,
	}
}

// UpdateGroupDescriptionToWaha converts a domain UpdateGroupDescriptionInput to a WAHA request.
func (m *Mapper) UpdateGroupDescriptionToWaha(input *UpdateGroupDescriptionInput) *wahaUpdateGroupDescriptionRequest {
	if input == nil {
		return nil
	}

	return &wahaUpdateGroupDescriptionRequest{
		Description: input.Description,
	}
}

// UpdateGroupPictureToWaha converts a domain UpdateGroupPictureInput to a WAHA request.
func (m *Mapper) UpdateGroupPictureToWaha(input *UpdateGroupPictureInput) *wahaUpdateGroupPictureRequest {
	if input == nil {
		return nil
	}

	return &wahaUpdateGroupPictureRequest{
		File: wahaFile{
			Data:     stripDataURIPrefix(input.ImageBase64),
			Mimetype: "image/jpeg", // Default mimetype for group pictures
		},
	}
}

// SetGroupAnnouncementToWaha converts a domain SetGroupAnnouncementInput to a WAHA request.
func (m *Mapper) SetGroupAnnouncementToWaha(input *SetGroupAnnouncementInput) *wahaSetGroupAnnouncementRequest {
	if input == nil {
		return nil
	}

	return &wahaSetGroupAnnouncementRequest{
		AdminsOnly: input.Announce,
	}
}

// =============================================================================
// Webhook Event Mapping (WAHA -> Domain)
// =============================================================================

// WebhookEventFromWaha converts a WAHA webhook payload to a domain WebhookEvent.
func (m *Mapper) WebhookEventFromWaha(w *wahaWebhookPayload) *WebhookEvent {
	if w == nil {
		return nil
	}

	event := &WebhookEvent{
		Event:       w.Event,
		SessionName: w.Session,
		Timestamp:   time.Now(), // WAHA doesn't always provide timestamp at top level
	}

	// Set timestamp from payload if available
	if w.Payload.Timestamp > 0 {
		event.Timestamp = wahaTimestampToTime(w.Payload.Timestamp)
	}

	// Pass through the payload - caller can use typed assertions
	event.Payload = w.Payload

	return event
}

// ExtendedMessageFromWahaPayload converts a WAHA message payload to domain ExtendedMessage.
func (m *Mapper) ExtendedMessageFromWahaPayload(w *wahaMessagePayload) *ExtendedMessage {
	if w == nil {
		return nil
	}

	msg := &ExtendedMessage{
		ID:          w.ID,
		From:        w.From,
		To:          w.To,
		FromMe:      w.FromMe,
		Body:        w.Body,
		HasMedia:    w.HasMedia,
		MediaType:   w.MediaType,
		IsForwarded: w.IsForwarded,
		Participant: w.Participant,
		Timestamp:   wahaTimestampToTime(w.Timestamp),
		Ack:         MessageAckLevel(w.Ack),
	}

	// Determine ChatID based on fromMe
	if w.FromMe {
		msg.ChatID = w.To
	} else {
		msg.ChatID = w.From
	}

	return msg
}

// =============================================================================
// Error Mapping
// =============================================================================

// ErrorFromWaha converts a WAHA error response to a domain ProviderError.
func (m *Mapper) ErrorFromWaha(w *wahaErrorResponse, cause error) *ProviderError {
	if w == nil {
		return NewProviderError(ErrCodeProviderError, "unknown error", cause)
	}

	message := w.Message
	if message == "" {
		message = w.Error
	}
	if message == "" {
		message = "unknown error"
	}

	// Map WAHA error codes to domain error codes
	var code ErrorCode
	switch w.Code {
	case 404:
		code = ErrCodeSessionNotFound
	case 403:
		code = ErrCodePermissionDenied
	case 400:
		code = ErrCodeInvalidInput
	case 429:
		code = ErrCodeRateLimited
	default:
		code = ErrCodeProviderError
	}

	return NewProviderError(code, message, cause)
}
