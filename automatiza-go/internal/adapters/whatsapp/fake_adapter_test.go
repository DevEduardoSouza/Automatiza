package whatsapp

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNewFakeAdapter(t *testing.T) {
	adapter := NewFakeAdapter()

	if adapter == nil {
		t.Fatal("NewFakeAdapter returned nil")
	}

	if adapter.Calls == nil {
		t.Error("Calls slice not initialized")
	}

	if adapter.Sessions == nil {
		t.Error("Sessions map not initialized")
	}

	if adapter.Groups == nil {
		t.Error("Groups map not initialized")
	}

	if adapter.Participants == nil {
		t.Error("Participants map not initialized")
	}

	if adapter.InviteCodes == nil {
		t.Error("InviteCodes map not initialized")
	}

	if adapter.QRCodes == nil {
		t.Error("QRCodes map not initialized")
	}

	if adapter.Messages == nil {
		t.Error("Messages map not initialized")
	}
}

func TestFakeAdapter_Reset(t *testing.T) {
	adapter := NewFakeAdapter()
	ctx := context.Background()

	// Set up some data
	adapter.SetSession("test", &SessionInfo{Name: "test", Status: SessionStatusConnected})
	adapter.SetGroups("test", []GroupInfo{{ID: "group1"}})
	_, _ = adapter.CreateSession(ctx, CreateSessionInput{SessionName: "test2"})

	// Verify data exists
	if len(adapter.Sessions) == 0 {
		t.Error("Sessions should not be empty before reset")
	}
	if len(adapter.Calls) == 0 {
		t.Error("Calls should not be empty before reset")
	}

	// Reset
	adapter.Reset()

	// Verify data is cleared
	if len(adapter.Sessions) != 0 {
		t.Error("Sessions should be empty after reset")
	}
	if len(adapter.Groups) != 0 {
		t.Error("Groups should be empty after reset")
	}
	if len(adapter.Calls) != 0 {
		t.Error("Calls should be empty after reset")
	}
}

func TestFakeAdapter_CallTracking(t *testing.T) {
	adapter := NewFakeAdapter()
	ctx := context.Background()

	// Make some calls
	_, _ = adapter.CreateSession(ctx, CreateSessionInput{SessionName: "session1"})
	_, _ = adapter.GetSessionStatus(ctx, "session1")
	_, _ = adapter.GetSessionStatus(ctx, "session1")

	// Test CallCount
	if count := adapter.CallCount("CreateSession"); count != 1 {
		t.Errorf("Expected CreateSession call count 1, got %d", count)
	}

	if count := adapter.CallCount("GetSessionStatus"); count != 2 {
		t.Errorf("Expected GetSessionStatus call count 2, got %d", count)
	}

	if count := adapter.CallCount("DeleteSession"); count != 0 {
		t.Errorf("Expected DeleteSession call count 0, got %d", count)
	}

	// Test LastCall
	lastCall := adapter.LastCall("CreateSession")
	if lastCall == nil {
		t.Fatal("LastCall returned nil for CreateSession")
	}

	if lastCall.Method != "CreateSession" {
		t.Errorf("Expected method CreateSession, got %s", lastCall.Method)
	}

	if len(lastCall.Args) != 1 {
		t.Errorf("Expected 1 arg, got %d", len(lastCall.Args))
	}

	input, ok := lastCall.Args[0].(CreateSessionInput)
	if !ok {
		t.Error("First arg is not CreateSessionInput")
	}
	if input.SessionName != "session1" {
		t.Errorf("Expected session name 'session1', got '%s'", input.SessionName)
	}

	// Test GetCalls
	calls := adapter.GetCalls("GetSessionStatus")
	if len(calls) != 2 {
		t.Errorf("Expected 2 calls, got %d", len(calls))
	}

	// Test LastCall for non-existent method
	if adapter.LastCall("NonExistent") != nil {
		t.Error("LastCall should return nil for non-existent method")
	}
}

func TestFakeAdapter_SimulateError(t *testing.T) {
	adapter := NewFakeAdapter()
	ctx := context.Background()

	expectedErr := errors.New("simulated error")
	adapter.SimulateError = expectedErr

	// All methods should return the simulated error
	_, err := adapter.CreateSession(ctx, CreateSessionInput{})
	if !errors.Is(err, expectedErr) {
		t.Errorf("CreateSession: expected simulated error, got %v", err)
	}

	err = adapter.DeleteSession(ctx, "test")
	if !errors.Is(err, expectedErr) {
		t.Errorf("DeleteSession: expected simulated error, got %v", err)
	}

	_, err = adapter.GetSessionStatus(ctx, "test")
	if !errors.Is(err, expectedErr) {
		t.Errorf("GetSessionStatus: expected simulated error, got %v", err)
	}

	_, err = adapter.SendText(ctx, SendTextInput{})
	if !errors.Is(err, expectedErr) {
		t.Errorf("SendText: expected simulated error, got %v", err)
	}
}

func TestFakeAdapter_FunctionOverrides(t *testing.T) {
	adapter := NewFakeAdapter()
	ctx := context.Background()

	customSession := &SessionInfo{
		Name:   "custom",
		Status: SessionStatusConnected,
		AuthenticatedUser: &AuthenticatedUser{
			ID:       "123456789@c.us",
			PushName: "Test User",
		},
	}

	adapter.CreateSessionFunc = func(ctx context.Context, input CreateSessionInput) (*SessionInfo, error) {
		return customSession, nil
	}

	result, err := adapter.CreateSession(ctx, CreateSessionInput{SessionName: "any"})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Name != "custom" {
		t.Errorf("Expected custom session name, got %s", result.Name)
	}

	if result.AuthenticatedUser == nil {
		t.Error("Expected authenticated user")
	}
}

func TestFakeAdapter_SessionManagement(t *testing.T) {
	adapter := NewFakeAdapter()
	ctx := context.Background()

	// Create session
	session, err := adapter.CreateSession(ctx, CreateSessionInput{
		SessionName: "test-session",
		WebhookURL:  "http://example.com/webhook",
	})
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	if session.Name != "test-session" {
		t.Errorf("Expected session name 'test-session', got '%s'", session.Name)
	}

	if session.Status != SessionStatusScanQR {
		t.Errorf("Expected status SCAN_QR, got %s", session.Status)
	}

	// Get session status
	status, err := adapter.GetSessionStatus(ctx, "test-session")
	if err != nil {
		t.Fatalf("GetSessionStatus failed: %v", err)
	}

	if status.Name != "test-session" {
		t.Errorf("Expected session name 'test-session', got '%s'", status.Name)
	}

	// Get non-existent session
	_, err = adapter.GetSessionStatus(ctx, "non-existent")
	if err == nil {
		t.Error("Expected error for non-existent session")
	}

	var providerErr *ProviderError
	if errors.As(err, &providerErr) {
		if providerErr.Code != ErrCodeSessionNotFound {
			t.Errorf("Expected SESSION_NOT_FOUND error code, got %s", providerErr.Code)
		}
	} else {
		t.Error("Expected ProviderError")
	}

	// Delete session
	err = adapter.DeleteSession(ctx, "test-session")
	if err != nil {
		t.Fatalf("DeleteSession failed: %v", err)
	}

	// Verify deleted
	_, err = adapter.GetSessionStatus(ctx, "test-session")
	if err == nil {
		t.Error("Expected error after deletion")
	}
}

func TestFakeAdapter_SetSession(t *testing.T) {
	adapter := NewFakeAdapter()
	ctx := context.Background()

	customSession := &SessionInfo{
		Name:   "preset-session",
		Status: SessionStatusConnected,
		AuthenticatedUser: &AuthenticatedUser{
			ID:       "5511999999999@c.us",
			PushName: "Preset User",
		},
	}

	adapter.SetSession("preset-session", customSession)

	status, err := adapter.GetSessionStatus(ctx, "preset-session")
	if err != nil {
		t.Fatalf("GetSessionStatus failed: %v", err)
	}

	if status.Status != SessionStatusConnected {
		t.Errorf("Expected CONNECTED status, got %s", status.Status)
	}

	if status.AuthenticatedUser == nil || status.AuthenticatedUser.PushName != "Preset User" {
		t.Error("Expected authenticated user with 'Preset User' push name")
	}
}

func TestFakeAdapter_GroupOperations(t *testing.T) {
	adapter := NewFakeAdapter()
	ctx := context.Background()

	// Set up groups
	groups := []GroupInfo{
		{ID: "group1@g.us", Subject: "Group 1", Size: 10},
		{ID: "group2@g.us", Subject: "Group 2", Size: 20},
	}
	adapter.SetGroups("test-session", groups)

	// Get groups
	result, err := adapter.GetGroups(ctx, "test-session")
	if err != nil {
		t.Fatalf("GetGroups failed: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("Expected 2 groups, got %d", len(result))
	}

	// Get groups for non-existent session (should return empty, not error)
	result, err = adapter.GetGroups(ctx, "non-existent")
	if err != nil {
		t.Fatalf("GetGroups should not fail for non-existent session: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("Expected 0 groups for non-existent session, got %d", len(result))
	}
}

func TestFakeAdapter_Participants(t *testing.T) {
	adapter := NewFakeAdapter()
	ctx := context.Background()

	participants := []ParticipantInfo{
		{ID: "user1@c.us", IsAdmin: true, IsSuperAdmin: true},
		{ID: "user2@c.us", IsAdmin: true, IsSuperAdmin: false},
		{ID: "user3@c.us", IsAdmin: false, IsSuperAdmin: false},
	}
	adapter.SetParticipants("test-session", "group1@g.us", participants)

	result, err := adapter.GetGroupParticipants(ctx, "test-session", "group1@g.us")
	if err != nil {
		t.Fatalf("GetGroupParticipants failed: %v", err)
	}

	if len(result) != 3 {
		t.Errorf("Expected 3 participants, got %d", len(result))
	}

	// Verify admin status
	if !result[0].IsSuperAdmin {
		t.Error("First participant should be super admin")
	}

	if !result[1].IsAdmin || result[1].IsSuperAdmin {
		t.Error("Second participant should be admin but not super admin")
	}
}

func TestFakeAdapter_GetGroupInviteCode(t *testing.T) {
	adapter := NewFakeAdapter()
	ctx := context.Background()

	// Set up an invite code
	adapter.SetInviteCode("test-session", "group1@g.us", "customInviteCode123")

	// Get the invite code
	result, err := adapter.GetGroupInviteCode(ctx, "test-session", "group1@g.us")
	if err != nil {
		t.Fatalf("GetGroupInviteCode failed: %v", err)
	}

	if result != "customInviteCode123" {
		t.Errorf("Expected 'customInviteCode123', got '%s'", result)
	}

	// Get invite code for group without explicit code (should return default fake code)
	result, err = adapter.GetGroupInviteCode(ctx, "test-session", "group2@g.us")
	if err != nil {
		t.Fatalf("GetGroupInviteCode for non-configured group failed: %v", err)
	}

	if result == "" {
		t.Error("Expected non-empty default invite code")
	}

	// Verify call tracking
	if count := adapter.CallCount("GetGroupInviteCode"); count != 2 {
		t.Errorf("Expected GetGroupInviteCode call count 2, got %d", count)
	}
}

func TestFakeAdapter_UpdateGroupSubject(t *testing.T) {
	adapter := NewFakeAdapter()
	ctx := context.Background()

	groups := []GroupInfo{
		{ID: "group1@g.us", Subject: "Original Name"},
	}
	adapter.SetGroups("test-session", groups)

	err := adapter.UpdateGroupSubject(ctx, UpdateGroupSubjectInput{
		SessionName: "test-session",
		GroupID:     "group1@g.us",
		Subject:     "New Name",
	})
	if err != nil {
		t.Fatalf("UpdateGroupSubject failed: %v", err)
	}

	// Verify update
	result, _ := adapter.GetGroups(ctx, "test-session")
	if result[0].Subject != "New Name" {
		t.Errorf("Expected subject 'New Name', got '%s'", result[0].Subject)
	}
}

func TestFakeAdapter_SendMessages(t *testing.T) {
	adapter := NewFakeAdapter()
	ctx := context.Background()

	// Send text
	result, err := adapter.SendText(ctx, SendTextInput{
		SessionName: "test-session",
		ChatID:      "group1@g.us",
		Text:        "Hello, World!",
	})
	if err != nil {
		t.Fatalf("SendText failed: %v", err)
	}

	if result.ID == "" {
		t.Error("Expected message ID")
	}

	if result.Timestamp.IsZero() {
		t.Error("Expected timestamp")
	}

	// Send image
	result, err = adapter.SendImage(ctx, SendImageInput{
		SessionName: "test-session",
		ChatID:      "group1@g.us",
		Caption:     "Check this out",
		ImageBase64: "base64data",
	})
	if err != nil {
		t.Fatalf("SendImage failed: %v", err)
	}

	if result.ID == "" {
		t.Error("Expected message ID for image")
	}

	// Send voice
	result, err = adapter.SendVoice(ctx, SendVoiceInput{
		SessionName: "test-session",
		ChatID:      "group1@g.us",
		AudioBase64: "base64audio",
	})
	if err != nil {
		t.Fatalf("SendVoice failed: %v", err)
	}

	if result.ID == "" {
		t.Error("Expected message ID for voice")
	}

	// Send poll
	result, err = adapter.SendPoll(ctx, SendPollInput{
		SessionName:     "test-session",
		ChatID:          "group1@g.us",
		Question:        "What's your favorite color?",
		Options:         []string{"Red", "Blue", "Green"},
		SelectableCount: 1,
	})
	if err != nil {
		t.Fatalf("SendPoll failed: %v", err)
	}

	if result.ID == "" {
		t.Error("Expected message ID for poll")
	}
}

func TestFakeAdapter_GetMessages(t *testing.T) {
	adapter := NewFakeAdapter()
	ctx := context.Background()

	// Set up messages
	messages := []Message{
		{ID: "msg1", ChatID: "group1@g.us", Body: "Message 1", Timestamp: time.Now()},
		{ID: "msg2", ChatID: "group1@g.us", Body: "Message 2", Timestamp: time.Now()},
		{ID: "msg3", ChatID: "group1@g.us", Body: "Message 3", Timestamp: time.Now()},
		{ID: "msg4", ChatID: "group1@g.us", Body: "Message 4", Timestamp: time.Now()},
		{ID: "msg5", ChatID: "group1@g.us", Body: "Message 5", Timestamp: time.Now()},
	}
	adapter.SetMessages("test-session", "group1@g.us", messages)

	// Get first page
	result, err := adapter.GetMessages(ctx, GetMessagesInput{
		SessionName: "test-session",
		ChatID:      "group1@g.us",
		Limit:       2,
		Offset:      0,
	})
	if err != nil {
		t.Fatalf("GetMessages failed: %v", err)
	}

	if len(result.Messages) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(result.Messages))
	}

	if !result.HasMore {
		t.Error("Expected HasMore to be true")
	}

	if result.NextOffset != 2 {
		t.Errorf("Expected NextOffset 2, got %d", result.NextOffset)
	}

	// Get second page
	result, err = adapter.GetMessages(ctx, GetMessagesInput{
		SessionName: "test-session",
		ChatID:      "group1@g.us",
		Limit:       2,
		Offset:      result.NextOffset,
	})
	if err != nil {
		t.Fatalf("GetMessages second page failed: %v", err)
	}

	if len(result.Messages) != 2 {
		t.Errorf("Expected 2 messages on second page, got %d", len(result.Messages))
	}

	// Get last page
	result, err = adapter.GetMessages(ctx, GetMessagesInput{
		SessionName: "test-session",
		ChatID:      "group1@g.us",
		Limit:       2,
		Offset:      4,
	})
	if err != nil {
		t.Fatalf("GetMessages last page failed: %v", err)
	}

	if len(result.Messages) != 1 {
		t.Errorf("Expected 1 message on last page, got %d", len(result.Messages))
	}

	if result.HasMore {
		t.Error("Expected HasMore to be false on last page")
	}
}

func TestFakeAdapter_QRCode(t *testing.T) {
	adapter := NewFakeAdapter()
	ctx := context.Background()

	// No QR code set - should return nil
	qr, err := adapter.GetQRCode(ctx, "test-session")
	if err != nil {
		t.Fatalf("GetQRCode should not fail: %v", err)
	}

	if qr != nil {
		t.Error("Expected nil QR code when not set")
	}

	// Set QR code
	adapter.SetQRCode("test-session", &QRCodeData{
		Value:       "qr-data-here",
		Base64Image: "base64-qr-image",
		Mimetype:    "image/png",
	})

	qr, err = adapter.GetQRCode(ctx, "test-session")
	if err != nil {
		t.Fatalf("GetQRCode failed: %v", err)
	}

	if qr == nil {
		t.Fatal("Expected QR code")
	}

	if qr.Value != "qr-data-here" {
		t.Errorf("Expected QR value 'qr-data-here', got '%s'", qr.Value)
	}
}

func TestFakeAdapter_RestartAndLogout(t *testing.T) {
	adapter := NewFakeAdapter()
	ctx := context.Background()

	// Set up connected session
	adapter.SetSession("test-session", &SessionInfo{
		Name:   "test-session",
		Status: SessionStatusConnected,
		AuthenticatedUser: &AuthenticatedUser{
			ID:       "123@c.us",
			PushName: "Test",
		},
	})

	// Restart
	err := adapter.RestartSession(ctx, "test-session")
	if err != nil {
		t.Fatalf("RestartSession failed: %v", err)
	}

	status, _ := adapter.GetSessionStatus(ctx, "test-session")
	if status.Status != SessionStatusStarting {
		t.Errorf("Expected STARTING status after restart, got %s", status.Status)
	}

	// Reset status for logout test
	adapter.Sessions["test-session"].Status = SessionStatusConnected

	// Logout
	err = adapter.LogoutSession(ctx, "test-session")
	if err != nil {
		t.Fatalf("LogoutSession failed: %v", err)
	}

	status, _ = adapter.GetSessionStatus(ctx, "test-session")
	if status.Status != SessionStatusScanQR {
		t.Errorf("Expected SCAN_QR status after logout, got %s", status.Status)
	}

	if status.AuthenticatedUser != nil {
		t.Error("Expected nil authenticated user after logout")
	}

	// Restart non-existent session
	err = adapter.RestartSession(ctx, "non-existent")
	if err == nil {
		t.Error("Expected error for non-existent session")
	}

	// Logout non-existent session
	err = adapter.LogoutSession(ctx, "non-existent")
	if err == nil {
		t.Error("Expected error for non-existent session")
	}
}

func TestFakeAdapter_SetWebhook(t *testing.T) {
	adapter := NewFakeAdapter()
	ctx := context.Background()

	err := adapter.SetWebhook(ctx, SetWebhookInput{
		SessionName: "test-session",
		URL:         "http://example.com/webhook",
		Events:      []string{"message", "message.ack"},
		HMACKey:     "secret-key",
	})
	if err != nil {
		t.Fatalf("SetWebhook failed: %v", err)
	}

	// Verify call was recorded
	if adapter.CallCount("SetWebhook") != 1 {
		t.Error("SetWebhook call not recorded")
	}

	lastCall := adapter.LastCall("SetWebhook")
	input := lastCall.Args[0].(SetWebhookInput)
	if input.URL != "http://example.com/webhook" {
		t.Errorf("Expected URL 'http://example.com/webhook', got '%s'", input.URL)
	}
}

func TestFakeAdapter_InterfaceCompliance(t *testing.T) {
	// This test verifies that FakeAdapter implements WhatsAppProvider
	var _ WhatsAppProvider = (*FakeAdapter)(nil)

	// Also verify sub-interfaces
	var _ SessionManager = (*FakeAdapter)(nil)
	var _ GroupManager = (*FakeAdapter)(nil)
	var _ MessageSender = (*FakeAdapter)(nil)
	var _ WebhookManager = (*FakeAdapter)(nil)
	var _ MessageReconciler = (*FakeAdapter)(nil)
}
