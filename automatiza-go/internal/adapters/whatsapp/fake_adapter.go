// Package whatsapp provides the port (interface) for WhatsApp provider adapters.
// This file contains a fake/mock implementation for testing purposes.
package whatsapp

import (
	"context"
	"sync"
	"time"
)

// FakeCall records a method invocation for test assertions.
type FakeCall struct {
	// Method is the name of the method that was called.
	Method string

	// Args contains the arguments passed to the method.
	Args []any

	// Time is when the call was made.
	Time time.Time
}

// FakeAdapter is a mock implementation of WhatsAppProvider for testing.
// It allows configuration of responses and tracks method calls for assertions.
type FakeAdapter struct {
	mu    sync.Mutex
	Calls []FakeCall

	// Session Management function overrides
	CreateSessionFunc    func(ctx context.Context, input CreateSessionInput) (*SessionInfo, error)
	DeleteSessionFunc    func(ctx context.Context, sessionName string) error
	GetSessionStatusFunc func(ctx context.Context, sessionName string) (*SessionInfo, error)
	GetQRCodeFunc        func(ctx context.Context, sessionName string) (*QRCodeData, error)
	RestartSessionFunc   func(ctx context.Context, sessionName string) error
	LogoutSessionFunc    func(ctx context.Context, sessionName string) error

	// Group Operations function overrides
	GetGroupsFunc              func(ctx context.Context, sessionName string) ([]GroupInfo, error)
	GetGroupParticipantsFunc   func(ctx context.Context, sessionName, groupID string) ([]ParticipantInfo, error)
	GetGroupInviteCodeFunc     func(ctx context.Context, sessionName, groupID string) (string, error)
	UpdateGroupSubjectFunc     func(ctx context.Context, input UpdateGroupSubjectInput) error
	UpdateGroupDescriptionFunc func(ctx context.Context, input UpdateGroupDescriptionInput) error
	UpdateGroupPictureFunc     func(ctx context.Context, input UpdateGroupPictureInput) error
	SetGroupAnnouncementFunc   func(ctx context.Context, input SetGroupAnnouncementInput) error

	// Member Management function overrides
	AddGroupMembersFunc    func(ctx context.Context, input AddGroupMembersInput) (*MemberOperationResult, error)
	RemoveGroupMembersFunc func(ctx context.Context, input RemoveGroupMembersInput) (*MemberOperationResult, error)
	LeaveGroupFunc         func(ctx context.Context, input LeaveGroupInput) error
	PromoteMemberFunc      func(ctx context.Context, input PromoteMemberInput) error
	DemoteMemberFunc       func(ctx context.Context, input DemoteMemberInput) error

	// Messaging function overrides
	SendTextFunc     func(ctx context.Context, input SendTextInput) (*MessageResult, error)
	SendImageFunc    func(ctx context.Context, input SendImageInput) (*MessageResult, error)
	SendVideoFunc    func(ctx context.Context, input SendVideoInput) (*MessageResult, error)
	SendDocumentFunc func(ctx context.Context, input SendDocumentInput) (*MessageResult, error)
	SendVoiceFunc    func(ctx context.Context, input SendVoiceInput) (*MessageResult, error)
	SendPollFunc     func(ctx context.Context, input SendPollInput) (*MessageResult, error)

	// Webhook function overrides
	SetWebhookFunc func(ctx context.Context, input SetWebhookInput) error

	// Reconciliation function overrides
	GetMessagesFunc func(ctx context.Context, input GetMessagesInput) (*MessagesResult, error)

	// Media function overrides
	DownloadMediaFunc func(ctx context.Context, input DownloadMediaInput) (*DownloadMediaResult, error)

	// Default data stores (used when Func is nil)
	Sessions     map[string]*SessionInfo
	Groups       map[string][]GroupInfo       // key: sessionName
	Participants map[string][]ParticipantInfo // key: sessionName:groupID
	InviteCodes  map[string]string            // key: sessionName:groupID
	QRCodes      map[string]*QRCodeData       // key: sessionName
	Messages     map[string][]Message         // key: sessionName:chatID
	MediaData    map[string][]byte            // key: sessionName:mediaID

	// Error simulation - when set, all methods return this error
	SimulateError error
}

// NewFakeAdapter creates a new FakeAdapter with initialized maps.
func NewFakeAdapter() *FakeAdapter {
	return &FakeAdapter{
		Calls:        make([]FakeCall, 0),
		Sessions:     make(map[string]*SessionInfo),
		Groups:       make(map[string][]GroupInfo),
		Participants: make(map[string][]ParticipantInfo),
		InviteCodes:  make(map[string]string),
		QRCodes:      make(map[string]*QRCodeData),
		Messages:     make(map[string][]Message),
		MediaData:    make(map[string][]byte),
	}
}

// Reset clears all recorded calls and configured responses.
func (f *FakeAdapter) Reset() {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.Calls = make([]FakeCall, 0)
	f.Sessions = make(map[string]*SessionInfo)
	f.Groups = make(map[string][]GroupInfo)
	f.Participants = make(map[string][]ParticipantInfo)
	f.InviteCodes = make(map[string]string)
	f.QRCodes = make(map[string]*QRCodeData)
	f.Messages = make(map[string][]Message)
	f.MediaData = make(map[string][]byte)
	f.SimulateError = nil

	// Clear function overrides
	f.CreateSessionFunc = nil
	f.DeleteSessionFunc = nil
	f.GetSessionStatusFunc = nil
	f.GetQRCodeFunc = nil
	f.RestartSessionFunc = nil
	f.LogoutSessionFunc = nil
	f.GetGroupsFunc = nil
	f.GetGroupParticipantsFunc = nil
	f.GetGroupInviteCodeFunc = nil
	f.UpdateGroupSubjectFunc = nil
	f.UpdateGroupDescriptionFunc = nil
	f.UpdateGroupPictureFunc = nil
	f.SetGroupAnnouncementFunc = nil
	f.AddGroupMembersFunc = nil
	f.RemoveGroupMembersFunc = nil
	f.LeaveGroupFunc = nil
	f.PromoteMemberFunc = nil
	f.DemoteMemberFunc = nil
	f.SendTextFunc = nil
	f.SendImageFunc = nil
	f.SendVoiceFunc = nil
	f.SendPollFunc = nil
	f.SetWebhookFunc = nil
	f.GetMessagesFunc = nil
	f.DownloadMediaFunc = nil
}

// recordCall records a method invocation.
func (f *FakeAdapter) recordCall(method string, args ...any) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.Calls = append(f.Calls, FakeCall{
		Method: method,
		Args:   args,
		Time:   time.Now(),
	})
}

// CallCount returns the number of times a method was called.
func (f *FakeAdapter) CallCount(method string) int {
	f.mu.Lock()
	defer f.mu.Unlock()

	count := 0
	for _, call := range f.Calls {
		if call.Method == method {
			count++
		}
	}
	return count
}

// LastCall returns the most recent call to a method, or nil if never called.
func (f *FakeAdapter) LastCall(method string) *FakeCall {
	f.mu.Lock()
	defer f.mu.Unlock()

	for i := len(f.Calls) - 1; i >= 0; i-- {
		if f.Calls[i].Method == method {
			call := f.Calls[i]
			return &call
		}
	}
	return nil
}

// GetCalls returns all recorded calls for a method.
func (f *FakeAdapter) GetCalls(method string) []FakeCall {
	f.mu.Lock()
	defer f.mu.Unlock()

	calls := make([]FakeCall, 0)
	for _, call := range f.Calls {
		if call.Method == method {
			calls = append(calls, call)
		}
	}
	return calls
}

// SetSession configures a session in the fake adapter.
func (f *FakeAdapter) SetSession(name string, session *SessionInfo) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.Sessions[name] = session
}

// SetGroups configures groups for a session.
func (f *FakeAdapter) SetGroups(sessionName string, groups []GroupInfo) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.Groups[sessionName] = groups
}

// SetParticipants configures participants for a group.
func (f *FakeAdapter) SetParticipants(sessionName, groupID string, participants []ParticipantInfo) {
	f.mu.Lock()
	defer f.mu.Unlock()

	key := sessionName + ":" + groupID
	f.Participants[key] = participants
}

// SetQRCode configures a QR code for a session.
func (f *FakeAdapter) SetQRCode(sessionName string, qr *QRCodeData) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.QRCodes[sessionName] = qr
}

// SetMessages configures messages for a chat.
func (f *FakeAdapter) SetMessages(sessionName, chatID string, messages []Message) {
	f.mu.Lock()
	defer f.mu.Unlock()

	key := sessionName + ":" + chatID
	f.Messages[key] = messages
}

// SetInviteCode configures an invite code for a group.
func (f *FakeAdapter) SetInviteCode(sessionName, groupID, inviteCode string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	key := sessionName + ":" + groupID
	f.InviteCodes[key] = inviteCode
}

// -----------------------------------------------------------------------------
// SessionManager Implementation
// -----------------------------------------------------------------------------

// CreateSession implements SessionManager.CreateSession.
func (f *FakeAdapter) CreateSession(ctx context.Context, input CreateSessionInput) (*SessionInfo, error) {
	f.recordCall("CreateSession", input)

	if f.SimulateError != nil {
		return nil, f.SimulateError
	}

	if f.CreateSessionFunc != nil {
		return f.CreateSessionFunc(ctx, input)
	}

	session := &SessionInfo{
		Name:   input.SessionName,
		Status: SessionStatusScanQR,
	}

	f.mu.Lock()
	f.Sessions[input.SessionName] = session
	f.mu.Unlock()

	return session, nil
}

// DeleteSession implements SessionManager.DeleteSession.
func (f *FakeAdapter) DeleteSession(ctx context.Context, sessionName string) error {
	f.recordCall("DeleteSession", sessionName)

	if f.SimulateError != nil {
		return f.SimulateError
	}

	if f.DeleteSessionFunc != nil {
		return f.DeleteSessionFunc(ctx, sessionName)
	}

	f.mu.Lock()
	delete(f.Sessions, sessionName)
	delete(f.Groups, sessionName)
	delete(f.QRCodes, sessionName)
	f.mu.Unlock()

	return nil
}

// GetSessionStatus implements SessionManager.GetSessionStatus.
func (f *FakeAdapter) GetSessionStatus(ctx context.Context, sessionName string) (*SessionInfo, error) {
	f.recordCall("GetSessionStatus", sessionName)

	if f.SimulateError != nil {
		return nil, f.SimulateError
	}

	if f.GetSessionStatusFunc != nil {
		return f.GetSessionStatusFunc(ctx, sessionName)
	}

	f.mu.Lock()
	session, exists := f.Sessions[sessionName]
	f.mu.Unlock()

	if !exists {
		return nil, NewProviderError(ErrCodeSessionNotFound, "session not found: "+sessionName, nil)
	}

	return session, nil
}

// GetQRCode implements SessionManager.GetQRCode.
func (f *FakeAdapter) GetQRCode(ctx context.Context, sessionName string) (*QRCodeData, error) {
	f.recordCall("GetQRCode", sessionName)

	if f.SimulateError != nil {
		return nil, f.SimulateError
	}

	if f.GetQRCodeFunc != nil {
		return f.GetQRCodeFunc(ctx, sessionName)
	}

	f.mu.Lock()
	qr, exists := f.QRCodes[sessionName]
	f.mu.Unlock()

	if !exists {
		// Return nil QR if session is already connected or QR not available
		return nil, nil
	}

	return qr, nil
}

// RestartSession implements SessionManager.RestartSession.
func (f *FakeAdapter) RestartSession(ctx context.Context, sessionName string) error {
	f.recordCall("RestartSession", sessionName)

	if f.SimulateError != nil {
		return f.SimulateError
	}

	if f.RestartSessionFunc != nil {
		return f.RestartSessionFunc(ctx, sessionName)
	}

	f.mu.Lock()
	session, exists := f.Sessions[sessionName]
	if exists {
		session.Status = SessionStatusStarting
	}
	f.mu.Unlock()

	if !exists {
		return NewProviderError(ErrCodeSessionNotFound, "session not found: "+sessionName, nil)
	}

	return nil
}

// LogoutSession implements SessionManager.LogoutSession.
func (f *FakeAdapter) LogoutSession(ctx context.Context, sessionName string) error {
	f.recordCall("LogoutSession", sessionName)

	if f.SimulateError != nil {
		return f.SimulateError
	}

	if f.LogoutSessionFunc != nil {
		return f.LogoutSessionFunc(ctx, sessionName)
	}

	f.mu.Lock()
	session, exists := f.Sessions[sessionName]
	if exists {
		session.Status = SessionStatusScanQR
		session.AuthenticatedUser = nil
	}
	f.mu.Unlock()

	if !exists {
		return NewProviderError(ErrCodeSessionNotFound, "session not found: "+sessionName, nil)
	}

	return nil
}

// ClearStatusCache implements SessionManager.ClearStatusCache (no-op for fake).
func (f *FakeAdapter) ClearStatusCache(_ string) {}

// -----------------------------------------------------------------------------
// GroupManager Implementation
// -----------------------------------------------------------------------------

// GetGroups implements GroupManager.GetGroups.
func (f *FakeAdapter) GetGroups(ctx context.Context, sessionName string) ([]GroupInfo, error) {
	f.recordCall("GetGroups", sessionName)

	if f.SimulateError != nil {
		return nil, f.SimulateError
	}

	if f.GetGroupsFunc != nil {
		return f.GetGroupsFunc(ctx, sessionName)
	}

	f.mu.Lock()
	groups, exists := f.Groups[sessionName]
	f.mu.Unlock()

	if !exists {
		return []GroupInfo{}, nil
	}

	return groups, nil
}

// GetGroupParticipants implements GroupManager.GetGroupParticipants.
func (f *FakeAdapter) GetGroupParticipants(ctx context.Context, sessionName, groupID string) ([]ParticipantInfo, error) {
	f.recordCall("GetGroupParticipants", sessionName, groupID)

	if f.SimulateError != nil {
		return nil, f.SimulateError
	}

	if f.GetGroupParticipantsFunc != nil {
		return f.GetGroupParticipantsFunc(ctx, sessionName, groupID)
	}

	f.mu.Lock()
	key := sessionName + ":" + groupID
	participants, exists := f.Participants[key]
	f.mu.Unlock()

	if !exists {
		return []ParticipantInfo{}, nil
	}

	return participants, nil
}

// GetGroupInviteCode implements GroupManager.GetGroupInviteCode.
func (f *FakeAdapter) GetGroupInviteCode(ctx context.Context, sessionName, groupID string) (string, error) {
	f.recordCall("GetGroupInviteCode", sessionName, groupID)

	if f.SimulateError != nil {
		return "", f.SimulateError
	}

	if f.GetGroupInviteCodeFunc != nil {
		return f.GetGroupInviteCodeFunc(ctx, sessionName, groupID)
	}

	f.mu.Lock()
	key := sessionName + ":" + groupID
	inviteCode, exists := f.InviteCodes[key]
	f.mu.Unlock()

	if !exists {
		// Return a default fake invite code
		return "fake_invite_code_" + groupID, nil
	}

	return inviteCode, nil
}

// UpdateGroupSubject implements GroupManager.UpdateGroupSubject.
func (f *FakeAdapter) UpdateGroupSubject(ctx context.Context, input UpdateGroupSubjectInput) error {
	f.recordCall("UpdateGroupSubject", input)

	if f.SimulateError != nil {
		return f.SimulateError
	}

	if f.UpdateGroupSubjectFunc != nil {
		return f.UpdateGroupSubjectFunc(ctx, input)
	}

	// Update group in storage if it exists
	f.mu.Lock()
	defer f.mu.Unlock()

	groups, exists := f.Groups[input.SessionName]
	if exists {
		for i := range groups {
			if groups[i].ID == input.GroupID {
				groups[i].Subject = input.Subject
				break
			}
		}
	}

	return nil
}

// UpdateGroupDescription implements GroupManager.UpdateGroupDescription.
func (f *FakeAdapter) UpdateGroupDescription(ctx context.Context, input UpdateGroupDescriptionInput) error {
	f.recordCall("UpdateGroupDescription", input)

	if f.SimulateError != nil {
		return f.SimulateError
	}

	if f.UpdateGroupDescriptionFunc != nil {
		return f.UpdateGroupDescriptionFunc(ctx, input)
	}

	// Update group in storage if it exists
	f.mu.Lock()
	defer f.mu.Unlock()

	groups, exists := f.Groups[input.SessionName]
	if exists {
		for i := range groups {
			if groups[i].ID == input.GroupID {
				groups[i].Description = input.Description
				break
			}
		}
	}

	return nil
}

// UpdateGroupPicture implements GroupManager.UpdateGroupPicture.
func (f *FakeAdapter) UpdateGroupPicture(ctx context.Context, input UpdateGroupPictureInput) error {
	f.recordCall("UpdateGroupPicture", input)

	if f.SimulateError != nil {
		return f.SimulateError
	}

	if f.UpdateGroupPictureFunc != nil {
		return f.UpdateGroupPictureFunc(ctx, input)
	}

	// No-op in fake implementation (picture stored externally)
	return nil
}

// SetGroupAnnouncement implements GroupManager.SetGroupAnnouncement.
func (f *FakeAdapter) SetGroupAnnouncement(ctx context.Context, input SetGroupAnnouncementInput) error {
	f.recordCall("SetGroupAnnouncement", input)

	if f.SimulateError != nil {
		return f.SimulateError
	}

	if f.SetGroupAnnouncementFunc != nil {
		return f.SetGroupAnnouncementFunc(ctx, input)
	}

	// Update group in storage if it exists
	f.mu.Lock()
	defer f.mu.Unlock()

	groups, exists := f.Groups[input.SessionName]
	if exists {
		for i := range groups {
			if groups[i].ID == input.GroupID {
				groups[i].Announce = input.Announce
				break
			}
		}
	}

	return nil
}

// -----------------------------------------------------------------------------
// Member Management Implementation
// -----------------------------------------------------------------------------

// AddGroupMembers implements GroupManager.AddGroupMembers.
func (f *FakeAdapter) AddGroupMembers(ctx context.Context, input AddGroupMembersInput) (*MemberOperationResult, error) {
	f.recordCall("AddGroupMembers", input)

	if f.SimulateError != nil {
		return nil, f.SimulateError
	}

	if f.AddGroupMembersFunc != nil {
		return f.AddGroupMembersFunc(ctx, input)
	}

	return &MemberOperationResult{
		SuccessCount:  len(input.PhoneNumbers),
		FailedNumbers: []string{},
	}, nil
}

// RemoveGroupMembers implements GroupManager.RemoveGroupMembers.
func (f *FakeAdapter) RemoveGroupMembers(ctx context.Context, input RemoveGroupMembersInput) (*MemberOperationResult, error) {
	f.recordCall("RemoveGroupMembers", input)

	if f.SimulateError != nil {
		return nil, f.SimulateError
	}

	if f.RemoveGroupMembersFunc != nil {
		return f.RemoveGroupMembersFunc(ctx, input)
	}

	return &MemberOperationResult{
		SuccessCount:  len(input.PhoneNumbers),
		FailedNumbers: []string{},
	}, nil
}

// LeaveGroup implements GroupManager.LeaveGroup.
func (f *FakeAdapter) LeaveGroup(ctx context.Context, input LeaveGroupInput) error {
	f.recordCall("LeaveGroup", input)

	if f.SimulateError != nil {
		return f.SimulateError
	}

	if f.LeaveGroupFunc != nil {
		return f.LeaveGroupFunc(ctx, input)
	}

	// Remove group from storage
	f.mu.Lock()
	defer f.mu.Unlock()

	groups, exists := f.Groups[input.SessionName]
	if exists {
		for i, g := range groups {
			if g.ID == input.GroupID {
				f.Groups[input.SessionName] = append(groups[:i], groups[i+1:]...)
				break
			}
		}
	}

	return nil
}

// PromoteMember implements GroupManager.PromoteMember.
func (f *FakeAdapter) PromoteMember(ctx context.Context, input PromoteMemberInput) error {
	f.recordCall("PromoteMember", input)

	if f.SimulateError != nil {
		return f.SimulateError
	}

	if f.PromoteMemberFunc != nil {
		return f.PromoteMemberFunc(ctx, input)
	}

	// Update participant in storage
	f.mu.Lock()
	defer f.mu.Unlock()

	key := input.SessionName + ":" + input.GroupID
	participants, exists := f.Participants[key]
	if exists {
		waID := input.PhoneNumber + "@s.whatsapp.net"
		for i, p := range participants {
			if p.ID == waID {
				participants[i].IsAdmin = true
				break
			}
		}
	}

	return nil
}

// DemoteMember implements GroupManager.DemoteMember.
func (f *FakeAdapter) DemoteMember(ctx context.Context, input DemoteMemberInput) error {
	f.recordCall("DemoteMember", input)

	if f.SimulateError != nil {
		return f.SimulateError
	}

	if f.DemoteMemberFunc != nil {
		return f.DemoteMemberFunc(ctx, input)
	}

	// Update participant in storage
	f.mu.Lock()
	defer f.mu.Unlock()

	key := input.SessionName + ":" + input.GroupID
	participants, exists := f.Participants[key]
	if exists {
		waID := input.PhoneNumber + "@s.whatsapp.net"
		for i, p := range participants {
			if p.ID == waID {
				participants[i].IsAdmin = false
				break
			}
		}
	}

	return nil
}

// -----------------------------------------------------------------------------
// MessageSender Implementation
// -----------------------------------------------------------------------------

// SendText implements MessageSender.SendText.
func (f *FakeAdapter) SendText(ctx context.Context, input SendTextInput) (*MessageResult, error) {
	f.recordCall("SendText", input)

	if f.SimulateError != nil {
		return nil, f.SimulateError
	}

	if f.SendTextFunc != nil {
		return f.SendTextFunc(ctx, input)
	}

	return &MessageResult{
		ID:        generateFakeMessageID(),
		Timestamp: time.Now(),
	}, nil
}

// SendImage implements MessageSender.SendImage.
func (f *FakeAdapter) SendImage(ctx context.Context, input SendImageInput) (*MessageResult, error) {
	f.recordCall("SendImage", input)

	if f.SimulateError != nil {
		return nil, f.SimulateError
	}

	if f.SendImageFunc != nil {
		return f.SendImageFunc(ctx, input)
	}

	return &MessageResult{
		ID:        generateFakeMessageID(),
		Timestamp: time.Now(),
	}, nil
}

// SendVideo implements MessageSender.SendVideo.
func (f *FakeAdapter) SendVideo(ctx context.Context, input SendVideoInput) (*MessageResult, error) {
	f.recordCall("SendVideo", input)

	if f.SimulateError != nil {
		return nil, f.SimulateError
	}

	if f.SendVideoFunc != nil {
		return f.SendVideoFunc(ctx, input)
	}

	return &MessageResult{
		ID:        generateFakeMessageID(),
		Timestamp: time.Now(),
	}, nil
}

// SendDocument implements MessageSender.SendDocument.
func (f *FakeAdapter) SendDocument(ctx context.Context, input SendDocumentInput) (*MessageResult, error) {
	f.recordCall("SendDocument", input)

	if f.SimulateError != nil {
		return nil, f.SimulateError
	}

	if f.SendDocumentFunc != nil {
		return f.SendDocumentFunc(ctx, input)
	}

	return &MessageResult{
		ID:        generateFakeMessageID(),
		Timestamp: time.Now(),
	}, nil
}

// SendVoice implements MessageSender.SendVoice.
func (f *FakeAdapter) SendVoice(ctx context.Context, input SendVoiceInput) (*MessageResult, error) {
	f.recordCall("SendVoice", input)

	if f.SimulateError != nil {
		return nil, f.SimulateError
	}

	if f.SendVoiceFunc != nil {
		return f.SendVoiceFunc(ctx, input)
	}

	return &MessageResult{
		ID:        generateFakeMessageID(),
		Timestamp: time.Now(),
	}, nil
}

// SendPoll implements MessageSender.SendPoll.
func (f *FakeAdapter) SendPoll(ctx context.Context, input SendPollInput) (*MessageResult, error) {
	f.recordCall("SendPoll", input)

	if f.SimulateError != nil {
		return nil, f.SimulateError
	}

	if f.SendPollFunc != nil {
		return f.SendPollFunc(ctx, input)
	}

	return &MessageResult{
		ID:        generateFakeMessageID(),
		Timestamp: time.Now(),
	}, nil
}

// -----------------------------------------------------------------------------
// WebhookManager Implementation
// -----------------------------------------------------------------------------

// SetWebhook implements WebhookManager.SetWebhook.
func (f *FakeAdapter) SetWebhook(ctx context.Context, input SetWebhookInput) error {
	f.recordCall("SetWebhook", input)

	if f.SimulateError != nil {
		return f.SimulateError
	}

	if f.SetWebhookFunc != nil {
		return f.SetWebhookFunc(ctx, input)
	}

	// No-op in fake implementation
	return nil
}

// -----------------------------------------------------------------------------
// MessageReconciler Implementation
// -----------------------------------------------------------------------------

// GetMessages implements MessageReconciler.GetMessages.
func (f *FakeAdapter) GetMessages(ctx context.Context, input GetMessagesInput) (*MessagesResult, error) {
	f.recordCall("GetMessages", input)

	if f.SimulateError != nil {
		return nil, f.SimulateError
	}

	if f.GetMessagesFunc != nil {
		return f.GetMessagesFunc(ctx, input)
	}

	f.mu.Lock()
	key := input.SessionName + ":" + input.ChatID
	messages, exists := f.Messages[key]
	f.mu.Unlock()

	if !exists {
		return &MessagesResult{
			Messages:   []Message{},
			HasMore:    false,
			NextOffset: 0,
		}, nil
	}

	// Apply pagination
	start := input.Offset
	if start > len(messages) {
		start = len(messages)
	}

	end := start + input.Limit
	if end > len(messages) {
		end = len(messages)
	}

	pagedMessages := messages[start:end]
	hasMore := end < len(messages)

	return &MessagesResult{
		Messages:   pagedMessages,
		HasMore:    hasMore,
		NextOffset: end,
	}, nil
}

// -----------------------------------------------------------------------------
// MediaDownloader Implementation
// -----------------------------------------------------------------------------

// DownloadMedia implements MediaDownloader.DownloadMedia.
func (f *FakeAdapter) DownloadMedia(ctx context.Context, input DownloadMediaInput) (*DownloadMediaResult, error) {
	f.recordCall("DownloadMedia", input)

	if f.SimulateError != nil {
		return nil, f.SimulateError
	}

	if f.DownloadMediaFunc != nil {
		return f.DownloadMediaFunc(ctx, input)
	}

	f.mu.Lock()
	key := input.SessionName + ":" + input.MediaID
	data, exists := f.MediaData[key]
	f.mu.Unlock()

	if !exists {
		return nil, NewProviderError(ErrCodeProviderError, "media not found: "+input.MediaID, nil)
	}

	return &DownloadMediaResult{
		Data:     data,
		Mimetype: "application/octet-stream",
		Filename: input.MediaID,
	}, nil
}

func (f *FakeAdapter) DownloadMediaByURL(ctx context.Context, mediaURL string) (*DownloadMediaResult, error) {
	f.recordCall("DownloadMediaByURL", mediaURL)

	if f.SimulateError != nil {
		return nil, f.SimulateError
	}

	return &DownloadMediaResult{
		Data:     []byte("fake-media-data"),
		Mimetype: "image/jpeg",
	}, nil
}

// RedownloadMedia implements MediaDownloader.RedownloadMedia.
func (f *FakeAdapter) RedownloadMedia(ctx context.Context, sessionName, chatID, messageID string) (*DownloadMediaResult, error) {
	f.recordCall("RedownloadMedia", map[string]string{
		"sessionName": sessionName,
		"chatID":      chatID,
		"messageID":   messageID,
	})

	if f.SimulateError != nil {
		return nil, f.SimulateError
	}

	return &DownloadMediaResult{
		Data:     []byte("fake-redownloaded-media"),
		Mimetype: "image/jpeg",
	}, nil
}

// SetMediaData configures media data for testing.
func (f *FakeAdapter) SetMediaData(sessionName, mediaID string, data []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()

	key := sessionName + ":" + mediaID
	f.MediaData[key] = data
}

// -----------------------------------------------------------------------------
// Helper Functions
// -----------------------------------------------------------------------------

var fakeMessageCounter int64
var fakeMessageMu sync.Mutex

// generateFakeMessageID generates a unique fake message ID.
func generateFakeMessageID() string {
	fakeMessageMu.Lock()
	defer fakeMessageMu.Unlock()

	fakeMessageCounter++
	return "fake_msg_" + time.Now().Format("20060102150405") + "_" + itoa(fakeMessageCounter)
}

// itoa converts int64 to string without importing strconv.
func itoa(i int64) string {
	if i == 0 {
		return "0"
	}

	var neg bool
	if i < 0 {
		neg = true
		i = -i
	}

	var b [20]byte
	idx := len(b)
	for i > 0 {
		idx--
		b[idx] = byte('0' + i%10)
		i /= 10
	}

	if neg {
		idx--
		b[idx] = '-'
	}

	return string(b[idx:])
}

// -----------------------------------------------------------------------------
// Compile-time interface verification
// -----------------------------------------------------------------------------

var _ WhatsAppProvider = (*FakeAdapter)(nil)
