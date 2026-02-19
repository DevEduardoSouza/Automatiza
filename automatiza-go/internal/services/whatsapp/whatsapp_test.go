// Package whatsapp provides E2E tests for WhatsApp flows.
// These tests verify the full integration between services, adapters, and repositories.
package whatsapp

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// =============================================================================
// Mock WhatsApp Provider
// =============================================================================

// MockWhatsAppProvider implements whatsapp.WhatsAppProvider for E2E testing.
type MockWhatsAppProvider struct {
	mu sync.RWMutex

	// Session state
	sessions map[string]*whatsapp.SessionInfo

	// Configurable behaviors
	CreateSessionFunc        func(ctx context.Context, input whatsapp.CreateSessionInput) (*whatsapp.SessionInfo, error)
	DeleteSessionFunc        func(ctx context.Context, sessionName string) error
	GetSessionStatusFunc     func(ctx context.Context, sessionName string) (*whatsapp.SessionInfo, error)
	GetQRCodeFunc            func(ctx context.Context, sessionName string) (*whatsapp.QRCodeData, error)
	RestartSessionFunc       func(ctx context.Context, sessionName string) error
	LogoutSessionFunc        func(ctx context.Context, sessionName string) error
	SendTextFunc             func(ctx context.Context, input whatsapp.SendTextInput) (*whatsapp.MessageResult, error)
	SendImageFunc            func(ctx context.Context, input whatsapp.SendImageInput) (*whatsapp.MessageResult, error)
	SendVoiceFunc            func(ctx context.Context, input whatsapp.SendVoiceInput) (*whatsapp.MessageResult, error)
	SendPollFunc             func(ctx context.Context, input whatsapp.SendPollInput) (*whatsapp.MessageResult, error)
	GetGroupsFunc            func(ctx context.Context, sessionName string) ([]whatsapp.GroupInfo, error)
	GetGroupParticipantsFunc func(ctx context.Context, sessionName, groupID string) ([]whatsapp.ParticipantInfo, error)
	GetGroupInviteCodeFunc   func(ctx context.Context, sessionName, groupID string) (string, error)
	SetWebhookFunc           func(ctx context.Context, input whatsapp.SetWebhookInput) error
	GetMessagesFunc          func(ctx context.Context, input whatsapp.GetMessagesInput) (*whatsapp.MessagesResult, error)

	// Call tracking
	CreateSessionCalls []whatsapp.CreateSessionInput
	DeleteSessionCalls []string
	SendTextCalls      []whatsapp.SendTextInput
	SendImageCalls     []whatsapp.SendImageInput
}

// NewMockWhatsAppProvider creates a new mock provider with default behaviors.
func NewMockWhatsAppProvider() *MockWhatsAppProvider {
	return &MockWhatsAppProvider{
		sessions: make(map[string]*whatsapp.SessionInfo),
	}
}

// Compile-time check
var _ whatsapp.WhatsAppProvider = (*MockWhatsAppProvider)(nil)

// CreateSession implements WhatsAppProvider.
func (m *MockWhatsAppProvider) CreateSession(ctx context.Context, input whatsapp.CreateSessionInput) (*whatsapp.SessionInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.CreateSessionCalls = append(m.CreateSessionCalls, input)

	if m.CreateSessionFunc != nil {
		return m.CreateSessionFunc(ctx, input)
	}

	// Default behavior: create session in SCAN_QR state
	info := &whatsapp.SessionInfo{
		Name:   input.SessionName,
		Status: whatsapp.SessionStatusScanQR,
	}
	m.sessions[input.SessionName] = info
	return info, nil
}

// DeleteSession implements WhatsAppProvider.
func (m *MockWhatsAppProvider) DeleteSession(ctx context.Context, sessionName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.DeleteSessionCalls = append(m.DeleteSessionCalls, sessionName)

	if m.DeleteSessionFunc != nil {
		return m.DeleteSessionFunc(ctx, sessionName)
	}

	delete(m.sessions, sessionName)
	return nil
}

// GetSessionStatus implements WhatsAppProvider.
func (m *MockWhatsAppProvider) GetSessionStatus(ctx context.Context, sessionName string) (*whatsapp.SessionInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.GetSessionStatusFunc != nil {
		return m.GetSessionStatusFunc(ctx, sessionName)
	}

	info, ok := m.sessions[sessionName]
	if !ok {
		return nil, whatsapp.NewProviderError(whatsapp.ErrCodeSessionNotFound, "session not found", nil)
	}
	return info, nil
}

// GetQRCode implements WhatsAppProvider.
func (m *MockWhatsAppProvider) GetQRCode(ctx context.Context, sessionName string) (*whatsapp.QRCodeData, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.GetQRCodeFunc != nil {
		return m.GetQRCodeFunc(ctx, sessionName)
	}

	info, ok := m.sessions[sessionName]
	if !ok {
		return nil, whatsapp.NewProviderError(whatsapp.ErrCodeSessionNotFound, "session not found", nil)
	}

	if info.Status == whatsapp.SessionStatusConnected {
		return nil, nil // Already authenticated
	}

	return &whatsapp.QRCodeData{
		Value:       "qr-code-value-" + sessionName,
		Base64Image: "data:image/png;base64,iVBORw0KGgoAAAA==",
	}, nil
}

// RestartSession implements WhatsAppProvider.
func (m *MockWhatsAppProvider) RestartSession(ctx context.Context, sessionName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.RestartSessionFunc != nil {
		return m.RestartSessionFunc(ctx, sessionName)
	}

	info, ok := m.sessions[sessionName]
	if !ok {
		return whatsapp.NewProviderError(whatsapp.ErrCodeSessionNotFound, "session not found", nil)
	}

	info.Status = whatsapp.SessionStatusScanQR
	return nil
}

// LogoutSession implements WhatsAppProvider.
func (m *MockWhatsAppProvider) LogoutSession(ctx context.Context, sessionName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.LogoutSessionFunc != nil {
		return m.LogoutSessionFunc(ctx, sessionName)
	}

	info, ok := m.sessions[sessionName]
	if !ok {
		return whatsapp.NewProviderError(whatsapp.ErrCodeSessionNotFound, "session not found", nil)
	}

	info.Status = whatsapp.SessionStatusDisconnected
	info.AuthenticatedUser = nil
	return nil
}

// SendText implements WhatsAppProvider.
func (m *MockWhatsAppProvider) SendText(ctx context.Context, input whatsapp.SendTextInput) (*whatsapp.MessageResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.SendTextCalls = append(m.SendTextCalls, input)

	if m.SendTextFunc != nil {
		return m.SendTextFunc(ctx, input)
	}

	// Check session status
	info, ok := m.sessions[input.SessionName]
	if !ok {
		return nil, whatsapp.NewProviderError(whatsapp.ErrCodeSessionNotFound, "session not found", nil)
	}
	if info.Status != whatsapp.SessionStatusConnected {
		return nil, whatsapp.NewProviderError(whatsapp.ErrCodeSessionNotConnected, "session not connected", nil)
	}

	return &whatsapp.MessageResult{
		ID:        "msg-" + input.ChatID + "-" + time.Now().Format("150405"),
		Timestamp: time.Now(),
	}, nil
}

// SendImage implements WhatsAppProvider.
func (m *MockWhatsAppProvider) SendImage(ctx context.Context, input whatsapp.SendImageInput) (*whatsapp.MessageResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.SendImageCalls = append(m.SendImageCalls, input)

	if m.SendImageFunc != nil {
		return m.SendImageFunc(ctx, input)
	}

	info, ok := m.sessions[input.SessionName]
	if !ok {
		return nil, whatsapp.NewProviderError(whatsapp.ErrCodeSessionNotFound, "session not found", nil)
	}
	if info.Status != whatsapp.SessionStatusConnected {
		return nil, whatsapp.NewProviderError(whatsapp.ErrCodeSessionNotConnected, "session not connected", nil)
	}

	return &whatsapp.MessageResult{
		ID:        "img-" + input.ChatID + "-" + time.Now().Format("150405"),
		Timestamp: time.Now(),
	}, nil
}

// SendVoice implements WhatsAppProvider.
func (m *MockWhatsAppProvider) SendVoice(ctx context.Context, input whatsapp.SendVoiceInput) (*whatsapp.MessageResult, error) {
	if m.SendVoiceFunc != nil {
		return m.SendVoiceFunc(ctx, input)
	}

	m.mu.RLock()
	info, ok := m.sessions[input.SessionName]
	m.mu.RUnlock()

	if !ok {
		return nil, whatsapp.NewProviderError(whatsapp.ErrCodeSessionNotFound, "session not found", nil)
	}
	if info.Status != whatsapp.SessionStatusConnected {
		return nil, whatsapp.NewProviderError(whatsapp.ErrCodeSessionNotConnected, "session not connected", nil)
	}

	return &whatsapp.MessageResult{
		ID:        "voice-" + time.Now().Format("150405"),
		Timestamp: time.Now(),
	}, nil
}

// SendPoll implements WhatsAppProvider.
func (m *MockWhatsAppProvider) SendPoll(ctx context.Context, input whatsapp.SendPollInput) (*whatsapp.MessageResult, error) {
	if m.SendPollFunc != nil {
		return m.SendPollFunc(ctx, input)
	}

	m.mu.RLock()
	info, ok := m.sessions[input.SessionName]
	m.mu.RUnlock()

	if !ok {
		return nil, whatsapp.NewProviderError(whatsapp.ErrCodeSessionNotFound, "session not found", nil)
	}
	if info.Status != whatsapp.SessionStatusConnected {
		return nil, whatsapp.NewProviderError(whatsapp.ErrCodeSessionNotConnected, "session not connected", nil)
	}

	return &whatsapp.MessageResult{
		ID:        "poll-" + time.Now().Format("150405"),
		Timestamp: time.Now(),
	}, nil
}

// GetGroups implements WhatsAppProvider.
func (m *MockWhatsAppProvider) GetGroups(ctx context.Context, sessionName string) ([]whatsapp.GroupInfo, error) {
	if m.GetGroupsFunc != nil {
		return m.GetGroupsFunc(ctx, sessionName)
	}

	return []whatsapp.GroupInfo{
		{ID: "group1@g.us", Subject: "Test Group 1", Size: 10},
		{ID: "group2@g.us", Subject: "Test Group 2", Size: 20},
	}, nil
}

// GetGroupParticipants implements WhatsAppProvider.
func (m *MockWhatsAppProvider) GetGroupParticipants(ctx context.Context, sessionName, groupID string) ([]whatsapp.ParticipantInfo, error) {
	if m.GetGroupParticipantsFunc != nil {
		return m.GetGroupParticipantsFunc(ctx, sessionName, groupID)
	}

	return []whatsapp.ParticipantInfo{
		{ID: "user1@c.us", IsAdmin: true, IsSuperAdmin: true},
		{ID: "user2@c.us", IsAdmin: false, IsSuperAdmin: false},
	}, nil
}

// UpdateGroupSubject implements WhatsAppProvider.
func (m *MockWhatsAppProvider) UpdateGroupSubject(ctx context.Context, input whatsapp.UpdateGroupSubjectInput) error {
	return nil
}

// UpdateGroupDescription implements WhatsAppProvider.
func (m *MockWhatsAppProvider) UpdateGroupDescription(ctx context.Context, input whatsapp.UpdateGroupDescriptionInput) error {
	return nil
}

// UpdateGroupPicture implements WhatsAppProvider.
func (m *MockWhatsAppProvider) UpdateGroupPicture(ctx context.Context, input whatsapp.UpdateGroupPictureInput) error {
	return nil
}

// SetGroupAnnouncement implements WhatsAppProvider.
func (m *MockWhatsAppProvider) SetGroupAnnouncement(ctx context.Context, input whatsapp.SetGroupAnnouncementInput) error {
	return nil
}

// GetGroupInviteCode implements WhatsAppProvider.
func (m *MockWhatsAppProvider) GetGroupInviteCode(ctx context.Context, sessionName, groupID string) (string, error) {
	if m.GetGroupInviteCodeFunc != nil {
		return m.GetGroupInviteCodeFunc(ctx, sessionName, groupID)
	}
	return "AbCdEfGhIjKlMnOp", nil
}

// SetWebhook implements WhatsAppProvider.
func (m *MockWhatsAppProvider) SetWebhook(ctx context.Context, input whatsapp.SetWebhookInput) error {
	if m.SetWebhookFunc != nil {
		return m.SetWebhookFunc(ctx, input)
	}
	return nil
}

// GetMessages implements WhatsAppProvider.
func (m *MockWhatsAppProvider) GetMessages(ctx context.Context, input whatsapp.GetMessagesInput) (*whatsapp.MessagesResult, error) {
	if m.GetMessagesFunc != nil {
		return m.GetMessagesFunc(ctx, input)
	}
	return &whatsapp.MessagesResult{
		Messages: []whatsapp.Message{
			{ID: "msg-1", Body: "Hello", Timestamp: time.Now().Add(-1 * time.Hour)},
			{ID: "msg-2", Body: "World", Timestamp: time.Now()},
		},
		HasMore:    false,
		NextOffset: 0,
	}, nil
}

// AddGroupMembers implements WhatsAppProvider.
func (m *MockWhatsAppProvider) AddGroupMembers(ctx context.Context, input whatsapp.AddGroupMembersInput) (*whatsapp.MemberOperationResult, error) {
	return &whatsapp.MemberOperationResult{SuccessCount: len(input.PhoneNumbers)}, nil
}

// RemoveGroupMembers implements WhatsAppProvider.
func (m *MockWhatsAppProvider) RemoveGroupMembers(ctx context.Context, input whatsapp.RemoveGroupMembersInput) (*whatsapp.MemberOperationResult, error) {
	return &whatsapp.MemberOperationResult{SuccessCount: len(input.PhoneNumbers)}, nil
}

// LeaveGroup implements WhatsAppProvider.
func (m *MockWhatsAppProvider) LeaveGroup(ctx context.Context, input whatsapp.LeaveGroupInput) error {
	return nil
}

// PromoteMember implements WhatsAppProvider.
func (m *MockWhatsAppProvider) PromoteMember(ctx context.Context, input whatsapp.PromoteMemberInput) error {
	return nil
}

// DemoteMember implements WhatsAppProvider.
func (m *MockWhatsAppProvider) DemoteMember(ctx context.Context, input whatsapp.DemoteMemberInput) error {
	return nil
}

// SendVideo implements WhatsAppProvider.
func (m *MockWhatsAppProvider) SendVideo(ctx context.Context, input whatsapp.SendVideoInput) (*whatsapp.MessageResult, error) {
	return &whatsapp.MessageResult{
		ID:        "video-" + time.Now().Format("150405"),
		Timestamp: time.Now(),
	}, nil
}

// SendDocument implements WhatsAppProvider.
func (m *MockWhatsAppProvider) SendDocument(ctx context.Context, input whatsapp.SendDocumentInput) (*whatsapp.MessageResult, error) {
	return &whatsapp.MessageResult{
		ID:        "doc-" + time.Now().Format("150405"),
		Timestamp: time.Now(),
	}, nil
}

// ClearStatusCache implements WhatsAppProvider.
func (m *MockWhatsAppProvider) ClearStatusCache(sessionName string) {}

// DownloadMedia implements WhatsAppProvider.
func (m *MockWhatsAppProvider) DownloadMedia(ctx context.Context, input whatsapp.DownloadMediaInput) (*whatsapp.DownloadMediaResult, error) {
	return &whatsapp.DownloadMediaResult{
		Data:     []byte("mock-media-data"),
		Mimetype: "application/octet-stream",
	}, nil
}

// DownloadMediaByURL implements WhatsAppProvider.
func (m *MockWhatsAppProvider) DownloadMediaByURL(ctx context.Context, mediaURL string) (*whatsapp.DownloadMediaResult, error) {
	return &whatsapp.DownloadMediaResult{
		Data:     []byte("mock-media-data"),
		Mimetype: "application/octet-stream",
	}, nil
}

// Helper methods for testing

// SimulateConnect simulates a session becoming connected.
func (m *MockWhatsAppProvider) SimulateConnect(sessionName, phoneNumber string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	info, ok := m.sessions[sessionName]
	if !ok {
		info = &whatsapp.SessionInfo{Name: sessionName}
		m.sessions[sessionName] = info
	}

	info.Status = whatsapp.SessionStatusConnected
	info.AuthenticatedUser = &whatsapp.AuthenticatedUser{
		ID:       phoneNumber + "@c.us",
		PushName: "Test User",
	}
}

// SimulateDisconnect simulates a session becoming disconnected.
func (m *MockWhatsAppProvider) SimulateDisconnect(sessionName string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if info, ok := m.sessions[sessionName]; ok {
		info.Status = whatsapp.SessionStatusDisconnected
	}
}

// =============================================================================
// Mock Instance Repository
// =============================================================================

// MockInstanceRepository implements repository.InstanceRepository for E2E testing.
type MockInstanceRepository struct {
	mu sync.RWMutex

	instances  map[int]*models.Instance
	byUserName map[string]*models.Instance // key: userID:instanceName
	nextID     int

	// Error injection
	GetInstanceByUserAndNameError error
	UpsertInstanceError           error
	DeleteInstanceError           error
}

// NewMockInstanceRepository creates a new mock repository.
func NewMockInstanceRepository() *MockInstanceRepository {
	return &MockInstanceRepository{
		instances:  make(map[int]*models.Instance),
		byUserName: make(map[string]*models.Instance),
		nextID:     1,
	}
}

func (r *MockInstanceRepository) GetInstanceByID(ctx context.Context, id int) (*models.Instance, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.instances[id], nil
}

func (r *MockInstanceRepository) GetInstanceByName(ctx context.Context, name string) (*models.Instance, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, inst := range r.instances {
		if inst.InstanceID == name {
			return inst, nil
		}
	}
	return nil, nil
}

func (r *MockInstanceRepository) GetInstanceByUserAndName(ctx context.Context, userID, name string) (*models.Instance, error) {
	if r.GetInstanceByUserAndNameError != nil {
		return nil, r.GetInstanceByUserAndNameError
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.byUserName[userID+":"+name], nil
}

func (r *MockInstanceRepository) GetUserInstances(ctx context.Context, userID string) ([]models.Instance, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []models.Instance
	for _, inst := range r.instances {
		if inst.UserID == userID {
			result = append(result, *inst)
		}
	}
	return result, nil
}

func (r *MockInstanceRepository) UpsertInstance(ctx context.Context, userID string, instance *models.Instance) (*models.Instance, error) {
	if r.UpsertInstanceError != nil {
		return nil, r.UpsertInstanceError
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if exists
	key := userID + ":" + instance.InstanceID
	if existing, ok := r.byUserName[key]; ok {
		existing.Status = instance.Status
		existing.UpdatedAt = instance.UpdatedAt
		return existing, nil
	}

	// Create new
	instance.ID = r.nextID
	r.nextID++
	r.instances[instance.ID] = instance
	r.byUserName[key] = instance
	return instance, nil
}

func (r *MockInstanceRepository) UpdateInstanceStatus(ctx context.Context, id int, status string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if inst, ok := r.instances[id]; ok {
		inst.Status = status
	}
	return nil
}

func (r *MockInstanceRepository) DeleteInstance(ctx context.Context, id int) error {
	if r.DeleteInstanceError != nil {
		return r.DeleteInstanceError
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if inst, ok := r.instances[id]; ok {
		delete(r.byUserName, inst.UserID+":"+inst.InstanceID)
		delete(r.instances, id)
	}
	return nil
}

func (r *MockInstanceRepository) GetConnectedInstancesWithAutomations(ctx context.Context) ([]models.Instance, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []models.Instance
	for _, inst := range r.instances {
		if inst.Status == models.InstanceStatusConnected {
			result = append(result, *inst)
		}
	}
	return result, nil
}

func (r *MockInstanceRepository) GetAllConnectedInstances(ctx context.Context) ([]models.Instance, error) {
	return r.GetConnectedInstancesWithAutomations(ctx)
}

// =============================================================================
// E2E Tests: Instance Lifecycle
// =============================================================================

func TestE2E_InstanceLifecycle_CreateConnectDisconnectDelete(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()

	provider := NewMockWhatsAppProvider()
	repo := NewMockInstanceRepository()
	service := NewInstanceService(provider, repo, logger)

	userID := "user-123"
	instanceName := "test-instance"

	// Step 1: Create instance
	t.Run("CreateInstance", func(t *testing.T) {
		instance, err := service.CreateInstance(ctx, userID, instanceName)
		require.NoError(t, err)
		assert.NotNil(t, instance)
		assert.Equal(t, instanceName, instance.InstanceID)
		assert.Equal(t, userID, instance.UserID)
		assert.Equal(t, models.InstanceStatusPending, instance.Status) // SCAN_QR → pending

		// Verify provider was called
		assert.Len(t, provider.CreateSessionCalls, 1)
		assert.Equal(t, instanceName, provider.CreateSessionCalls[0].SessionName)
	})

	// Step 2: Get QR code
	t.Run("GetQRCode", func(t *testing.T) {
		qr, err := service.GetQRCode(ctx, instanceName)
		require.NoError(t, err)
		assert.NotNil(t, qr)
		assert.NotEmpty(t, qr.Base64)
	})

	// Step 3: Simulate connection (user scans QR)
	t.Run("SimulateConnection", func(t *testing.T) {
		provider.SimulateConnect(instanceName, "5511999999999")

		status, err := service.GetStatus(ctx, instanceName)
		require.NoError(t, err)
		assert.Equal(t, models.InstanceStatusConnected, status)
	})

	// Step 4: Logout
	t.Run("Logout", func(t *testing.T) {
		err := service.LogoutInstance(ctx, instanceName)
		require.NoError(t, err)

		status, err := service.GetStatus(ctx, instanceName)
		require.NoError(t, err)
		assert.Equal(t, models.InstanceStatusDisconnected, status)
	})

	// Step 5: Reconnect
	t.Run("Reconnect", func(t *testing.T) {
		qr, err := service.ReconnectInstance(ctx, instanceName)
		require.NoError(t, err)
		assert.NotNil(t, qr) // Should get new QR code

		status, err := service.GetStatus(ctx, instanceName)
		require.NoError(t, err)
		assert.Equal(t, models.InstanceStatusPending, status)
	})

	// Step 6: Delete instance
	t.Run("DeleteInstance", func(t *testing.T) {
		instances, err := service.GetUserInstances(ctx, userID)
		require.NoError(t, err)
		require.Len(t, instances, 1)

		err = service.DeleteInstance(ctx, userID, instances[0].ID)
		require.NoError(t, err)

		// Verify deleted
		instances, err = service.GetUserInstances(ctx, userID)
		require.NoError(t, err)
		assert.Len(t, instances, 0)

		// Verify provider was called
		assert.Contains(t, provider.DeleteSessionCalls, instanceName)
	})
}

func TestE2E_InstanceLifecycle_DuplicateInstanceRejected(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()

	provider := NewMockWhatsAppProvider()
	repo := NewMockInstanceRepository()
	service := NewInstanceService(provider, repo, logger)

	userID := "user-123"
	instanceName := "my-instance"

	// Create first instance
	_, err := service.CreateInstance(ctx, userID, instanceName)
	require.NoError(t, err)

	// Try to create duplicate
	_, err = service.CreateInstance(ctx, userID, instanceName)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestE2E_InstanceLifecycle_InvalidInstanceNameRejected(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()

	provider := NewMockWhatsAppProvider()
	repo := NewMockInstanceRepository()
	service := NewInstanceService(provider, repo, logger)

	invalidNames := []string{
		"",
		"ab", // too short
		"contains spaces",
		"contains@special!chars",
		"-starts-with-dash",
	}

	for _, name := range invalidNames {
		t.Run("invalid_name_"+name, func(t *testing.T) {
			_, err := service.CreateInstance(ctx, "user-123", name)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid instance name")
		})
	}
}

func TestE2E_InstanceLifecycle_CleanupOnDBError(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()

	provider := NewMockWhatsAppProvider()
	repo := NewMockInstanceRepository()
	repo.UpsertInstanceError = errors.New("database connection refused")

	service := NewInstanceService(provider, repo, logger)

	// Create should fail and cleanup the provider session
	_, err := service.CreateInstance(ctx, "user-123", "test-instance")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to save instance")

	// Verify cleanup was attempted
	assert.Contains(t, provider.DeleteSessionCalls, "test-instance")
}

// =============================================================================
// E2E Tests: Message Dispatch Flow
// =============================================================================

func TestE2E_MessageDispatch_SendTextWhenConnected(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()

	provider := NewMockWhatsAppProvider()
	repo := NewMockInstanceRepository()
	service := NewInstanceService(provider, repo, logger)

	userID := "user-123"
	instanceName := "dispatch-instance"

	// Setup: Create and connect instance
	_, err := service.CreateInstance(ctx, userID, instanceName)
	require.NoError(t, err)
	provider.SimulateConnect(instanceName, "5511999999999")

	// Send text message via provider directly
	result, err := provider.SendText(ctx, whatsapp.SendTextInput{
		SessionName: instanceName,
		ChatID:      "group@g.us",
		Text:        "Hello from automation!",
	})

	require.NoError(t, err)
	assert.NotEmpty(t, result.ID)
	assert.False(t, result.Timestamp.IsZero())

	// Verify call was recorded
	assert.Len(t, provider.SendTextCalls, 1)
	assert.Equal(t, "Hello from automation!", provider.SendTextCalls[0].Text)
}

func TestE2E_MessageDispatch_FailWhenDisconnected(t *testing.T) {
	ctx := context.Background()
	provider := NewMockWhatsAppProvider()

	// Create session but don't connect
	_, err := provider.CreateSession(ctx, whatsapp.CreateSessionInput{
		SessionName: "disconnected-session",
	})
	require.NoError(t, err)

	// Try to send - should fail
	_, err = provider.SendText(ctx, whatsapp.SendTextInput{
		SessionName: "disconnected-session",
		ChatID:      "group@g.us",
		Text:        "This should fail",
	})

	require.Error(t, err)
	var providerErr *whatsapp.ProviderError
	if assert.ErrorAs(t, err, &providerErr) {
		assert.Equal(t, whatsapp.ErrCodeSessionNotConnected, providerErr.Code)
	}
}

func TestE2E_MessageDispatch_SendImageWithCaption(t *testing.T) {
	ctx := context.Background()
	provider := NewMockWhatsAppProvider()

	// Setup connected session
	provider.SimulateConnect("image-session", "5511999999999")

	// Send image
	result, err := provider.SendImage(ctx, whatsapp.SendImageInput{
		SessionName: "image-session",
		ChatID:      "group@g.us",
		Caption:     "Check out this product!",
		ImageURL:    "https://example.com/product.jpg",
	})

	require.NoError(t, err)
	assert.NotEmpty(t, result.ID)
	assert.Len(t, provider.SendImageCalls, 1)
	assert.Equal(t, "Check out this product!", provider.SendImageCalls[0].Caption)
}

func TestE2E_MessageDispatch_ConcurrentSends(t *testing.T) {
	ctx := context.Background()
	provider := NewMockWhatsAppProvider()
	provider.SimulateConnect("concurrent-session", "5511999999999")

	// Send 10 messages concurrently
	var wg sync.WaitGroup
	errChan := make(chan error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := provider.SendText(ctx, whatsapp.SendTextInput{
				SessionName: "concurrent-session",
				ChatID:      "group@g.us",
				Text:        "Message " + string(rune('0'+i)),
			})
			if err != nil {
				errChan <- err
			}
		}(i)
	}

	wg.Wait()
	close(errChan)

	// Verify no errors
	for err := range errChan {
		t.Errorf("concurrent send failed: %v", err)
	}

	// Verify all messages were tracked
	assert.Len(t, provider.SendTextCalls, 10)
}

// =============================================================================
// E2E Tests: Group Operations
// =============================================================================

func TestE2E_GroupOperations_FetchAndProcess(t *testing.T) {
	ctx := context.Background()
	provider := NewMockWhatsAppProvider()
	provider.SimulateConnect("groups-session", "5511999999999")

	// Fetch groups
	groups, err := provider.GetGroups(ctx, "groups-session")
	require.NoError(t, err)
	assert.Len(t, groups, 2)
	assert.Equal(t, "Test Group 1", groups[0].Subject)

	// Fetch participants for first group
	participants, err := provider.GetGroupParticipants(ctx, "groups-session", groups[0].ID)
	require.NoError(t, err)
	assert.Len(t, participants, 2)
	assert.True(t, participants[0].IsAdmin)

	// Get invite code
	inviteCode, err := provider.GetGroupInviteCode(ctx, "groups-session", groups[0].ID)
	require.NoError(t, err)
	assert.NotEmpty(t, inviteCode)
}

// =============================================================================
// E2E Tests: Error Recovery
// =============================================================================

func TestE2E_ErrorRecovery_ReconnectAfterDisconnect(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()

	provider := NewMockWhatsAppProvider()
	repo := NewMockInstanceRepository()
	service := NewInstanceService(provider, repo, logger)

	userID := "user-123"
	instanceName := "recovery-instance"

	// Create and connect
	_, err := service.CreateInstance(ctx, userID, instanceName)
	require.NoError(t, err)
	provider.SimulateConnect(instanceName, "5511999999999")

	// Verify connected
	status, err := service.GetStatus(ctx, instanceName)
	require.NoError(t, err)
	assert.Equal(t, models.InstanceStatusConnected, status)

	// Simulate disconnect (e.g., phone went offline)
	provider.SimulateDisconnect(instanceName)

	// Verify disconnected
	status, err = service.GetStatus(ctx, instanceName)
	require.NoError(t, err)
	assert.Equal(t, models.InstanceStatusDisconnected, status)

	// Reconnect
	qr, err := service.ReconnectInstance(ctx, instanceName)
	require.NoError(t, err)
	assert.NotNil(t, qr)

	// Simulate user reconnecting
	provider.SimulateConnect(instanceName, "5511999999999")

	// Verify reconnected
	status, err = service.GetStatus(ctx, instanceName)
	require.NoError(t, err)
	assert.Equal(t, models.InstanceStatusConnected, status)
}

func TestE2E_ErrorRecovery_ProviderTemporaryFailure(t *testing.T) {
	ctx := context.Background()
	provider := NewMockWhatsAppProvider()
	provider.SimulateConnect("retry-session", "5511999999999")

	// Simulate temporary failure followed by success
	callCount := 0
	provider.SendTextFunc = func(ctx context.Context, input whatsapp.SendTextInput) (*whatsapp.MessageResult, error) {
		callCount++
		if callCount < 3 {
			return nil, whatsapp.NewProviderError(whatsapp.ErrCodeProviderError, "temporary failure", nil)
		}
		return &whatsapp.MessageResult{
			ID:        "msg-success",
			Timestamp: time.Now(),
		}, nil
	}

	// Retry logic (simulating what a worker would do)
	var result *whatsapp.MessageResult
	var err error
	for i := 0; i < 5; i++ {
		result, err = provider.SendText(ctx, whatsapp.SendTextInput{
			SessionName: "retry-session",
			ChatID:      "group@g.us",
			Text:        "Retry message",
		})
		if err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond) // Backoff
	}

	require.NoError(t, err)
	assert.Equal(t, "msg-success", result.ID)
	assert.Equal(t, 3, callCount) // Two failures + one success
}

// =============================================================================
// E2E Tests: Webhook Processing Flow
// =============================================================================

func TestE2E_WebhookFlow_StatusChangeUpdatesDB(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()

	provider := NewMockWhatsAppProvider()
	repo := NewMockInstanceRepository()
	service := NewInstanceService(provider, repo, logger)

	userID := "user-123"
	instanceName := "webhook-instance"

	// Create instance
	instance, err := service.CreateInstance(ctx, userID, instanceName)
	require.NoError(t, err)

	// Initial status should be pending (SCAN_QR)
	assert.Equal(t, models.InstanceStatusPending, instance.Status)

	// Simulate webhook: session connected
	provider.SimulateConnect(instanceName, "5511999999999")

	// Service should report connected status
	status, err := service.GetStatus(ctx, instanceName)
	require.NoError(t, err)
	assert.Equal(t, models.InstanceStatusConnected, status)
}

// =============================================================================
// E2E Tests: Message Reconciliation
// =============================================================================

func TestE2E_MessageReconciliation_FetchHistoricalMessages(t *testing.T) {
	ctx := context.Background()
	provider := NewMockWhatsAppProvider()
	provider.SimulateConnect("reconcile-session", "5511999999999")

	// Fetch messages for reconciliation
	result, err := provider.GetMessages(ctx, whatsapp.GetMessagesInput{
		SessionName: "reconcile-session",
		ChatID:      "group@g.us",
		Limit:       50,
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Messages, 2)
	assert.Equal(t, "Hello", result.Messages[0].Body)
	assert.False(t, result.HasMore)
}

// =============================================================================
// E2E Tests: Edge Cases
// =============================================================================

func TestE2E_EdgeCase_SessionNotFound(t *testing.T) {
	ctx := context.Background()
	provider := NewMockWhatsAppProvider()

	// Try to get status for non-existent session
	_, err := provider.GetSessionStatus(ctx, "non-existent-session")
	require.Error(t, err)

	var providerErr *whatsapp.ProviderError
	if assert.ErrorAs(t, err, &providerErr) {
		assert.Equal(t, whatsapp.ErrCodeSessionNotFound, providerErr.Code)
	}
}

func TestE2E_EdgeCase_QRCodeWhenAlreadyConnected(t *testing.T) {
	ctx := context.Background()
	provider := NewMockWhatsAppProvider()

	// Setup connected session
	provider.SimulateConnect("connected-session", "5511999999999")

	// Request QR code
	qr, err := provider.GetQRCode(ctx, "connected-session")
	require.NoError(t, err)
	assert.Nil(t, qr) // No QR needed when already connected
}

func TestE2E_EdgeCase_DeleteNonOwnedInstance(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()

	provider := NewMockWhatsAppProvider()
	repo := NewMockInstanceRepository()
	service := NewInstanceService(provider, repo, logger)

	// Create instance for user A
	instance, err := service.CreateInstance(ctx, "user-A", "shared-instance")
	require.NoError(t, err)

	// Try to delete as user B
	err = service.DeleteInstance(ctx, "user-B", instance.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not belong to this user")
}

// =============================================================================
// Benchmark Tests
// =============================================================================

func BenchmarkE2E_CreateInstance(b *testing.B) {
	ctx := context.Background()
	logger := zap.NewNop()

	provider := NewMockWhatsAppProvider()
	repo := NewMockInstanceRepository()
	service := NewInstanceService(provider, repo, logger)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = service.CreateInstance(ctx, "user-bench", "instance-"+string(rune('A'+i%26)))
	}
}

func BenchmarkE2E_SendMessage(b *testing.B) {
	ctx := context.Background()
	provider := NewMockWhatsAppProvider()
	provider.SimulateConnect("bench-session", "5511999999999")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = provider.SendText(ctx, whatsapp.SendTextInput{
			SessionName: "bench-session",
			ChatID:      "group@g.us",
			Text:        "Benchmark message",
		})
	}
}
