package startup

import (
	"context"
	"errors"
	"testing"
	"time"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/models"
	"automatiza-go/internal/services/reconciliation"

	"go.uber.org/zap"
)

// mockInstanceRepo implements repository.InstanceRepository for testing
type mockInstanceRepo struct {
	instances []models.Instance
	err       error
}

func (m *mockInstanceRepo) GetAllConnectedInstances(ctx context.Context) ([]models.Instance, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.instances, nil
}

func (m *mockInstanceRepo) GetUserInstances(ctx context.Context, userID string) ([]models.Instance, error) {
	return nil, nil
}
func (m *mockInstanceRepo) GetInstanceByID(ctx context.Context, instanceID int) (*models.Instance, error) {
	return nil, nil
}
func (m *mockInstanceRepo) GetInstanceByName(ctx context.Context, instanceName string) (*models.Instance, error) {
	return nil, nil
}
func (m *mockInstanceRepo) GetInstanceByUserAndName(ctx context.Context, userID, instanceName string) (*models.Instance, error) {
	return nil, nil
}
func (m *mockInstanceRepo) UpsertInstance(ctx context.Context, userID string, data *models.Instance) (*models.Instance, error) {
	return nil, nil
}
func (m *mockInstanceRepo) UpdateInstanceStatus(ctx context.Context, instanceID int, status string) error {
	return nil
}
func (m *mockInstanceRepo) DeleteInstance(ctx context.Context, instanceID int) error {
	return nil
}

// mockCheckpointRepo implements reconciliation.CheckpointRepository for testing
type mockCheckpointRepo struct {
	checkpoints map[string][]reconciliation.Checkpoint
	created     []string // tracks created checkpoints
}

func newMockCheckpointRepo() *mockCheckpointRepo {
	return &mockCheckpointRepo{
		checkpoints: make(map[string][]reconciliation.Checkpoint),
		created:     make([]string, 0),
	}
}

func (m *mockCheckpointRepo) GetOrCreate(ctx context.Context, sessionName, chatID string) (*reconciliation.Checkpoint, error) {
	if cps, ok := m.checkpoints[sessionName]; ok {
		for _, cp := range cps {
			if cp.ChatID == chatID {
				return &cp, nil
			}
		}
	}
	// Create new checkpoint
	cp := reconciliation.Checkpoint{
		SessionName:          sessionName,
		ChatID:               chatID,
		LastMessageTimestamp: 0,
		LastMessageID:        "",
		LastRunAt:            time.Now(),
		Status:               reconciliation.CheckpointStatusIdle,
	}
	m.checkpoints[sessionName] = append(m.checkpoints[sessionName], cp)
	m.created = append(m.created, sessionName+":"+chatID)
	return &cp, nil
}

func (m *mockCheckpointRepo) MarkRunning(ctx context.Context, sessionName, chatID string) error {
	return nil
}

func (m *mockCheckpointRepo) MarkIdle(ctx context.Context, sessionName, chatID string) error {
	return nil
}

func (m *mockCheckpointRepo) MarkFailed(ctx context.Context, sessionName, chatID string) error {
	return nil
}

func (m *mockCheckpointRepo) UpdateCheckpoint(ctx context.Context, sessionName, chatID string, lastTimestamp int64, lastMessageID string) error {
	return nil
}

func (m *mockCheckpointRepo) GetActiveSessionsChats(ctx context.Context) ([]reconciliation.SessionChat, error) {
	var result []reconciliation.SessionChat
	for sessionName, cps := range m.checkpoints {
		for _, cp := range cps {
			result = append(result, reconciliation.SessionChat{
				SessionName: sessionName,
				ChatID:      cp.ChatID,
			})
		}
	}
	return result, nil
}

func (m *mockCheckpointRepo) GetBySession(ctx context.Context, sessionName string) ([]reconciliation.Checkpoint, error) {
	return m.checkpoints[sessionName], nil
}

func (m *mockCheckpointRepo) AddCheckpoint(sessionName string, checkpoint reconciliation.Checkpoint) {
	m.checkpoints[sessionName] = append(m.checkpoints[sessionName], checkpoint)
}

// mockWahaProvider implements whatsapp.WhatsAppProvider for testing
type mockWahaProvider struct {
	sessionStatus map[string]*whatsapp.SessionInfo
	messages      map[string][]whatsapp.Message
	statusErr     error
	messagesErr   error
}

func newMockWahaProvider() *mockWahaProvider {
	return &mockWahaProvider{
		sessionStatus: make(map[string]*whatsapp.SessionInfo),
		messages:      make(map[string][]whatsapp.Message),
	}
}

func (m *mockWahaProvider) GetSessionStatus(ctx context.Context, sessionName string) (*whatsapp.SessionInfo, error) {
	if m.statusErr != nil {
		return nil, m.statusErr
	}
	if status, ok := m.sessionStatus[sessionName]; ok {
		return status, nil
	}
	return &whatsapp.SessionInfo{
		Name:   sessionName,
		Status: whatsapp.SessionStatusConnected,
	}, nil
}

func (m *mockWahaProvider) GetMessages(ctx context.Context, input whatsapp.GetMessagesInput) (*whatsapp.MessagesResult, error) {
	if m.messagesErr != nil {
		return nil, m.messagesErr
	}
	key := input.SessionName + ":" + input.ChatID
	msgs := m.messages[key]
	return &whatsapp.MessagesResult{
		Messages:   msgs,
		HasMore:    false,
		NextOffset: 0,
	}, nil
}

func (m *mockWahaProvider) SetSessionStatus(sessionName string, status whatsapp.SessionStatus) {
	m.sessionStatus[sessionName] = &whatsapp.SessionInfo{
		Name:   sessionName,
		Status: status,
	}
}

func (m *mockWahaProvider) SetMessages(sessionName, chatID string, messages []whatsapp.Message) {
	key := sessionName + ":" + chatID
	m.messages[key] = messages
}

// Stub implementations for other interface methods
func (m *mockWahaProvider) CreateSession(ctx context.Context, input whatsapp.CreateSessionInput) (*whatsapp.SessionInfo, error) {
	return nil, nil
}
func (m *mockWahaProvider) DeleteSession(ctx context.Context, sessionName string) error {
	return nil
}
func (m *mockWahaProvider) GetQRCode(ctx context.Context, sessionName string) (*whatsapp.QRCodeData, error) {
	return nil, nil
}
func (m *mockWahaProvider) RestartSession(ctx context.Context, sessionName string) error {
	return nil
}
func (m *mockWahaProvider) LogoutSession(ctx context.Context, sessionName string) error {
	return nil
}
func (m *mockWahaProvider) GetGroups(ctx context.Context, sessionName string) ([]whatsapp.GroupInfo, error) {
	return nil, nil
}
func (m *mockWahaProvider) GetGroupParticipants(ctx context.Context, sessionName, groupID string) ([]whatsapp.ParticipantInfo, error) {
	return nil, nil
}
func (m *mockWahaProvider) GetGroupInviteCode(ctx context.Context, sessionName, groupID string) (string, error) {
	return "", nil
}
func (m *mockWahaProvider) UpdateGroupSubject(ctx context.Context, input whatsapp.UpdateGroupSubjectInput) error {
	return nil
}
func (m *mockWahaProvider) UpdateGroupDescription(ctx context.Context, input whatsapp.UpdateGroupDescriptionInput) error {
	return nil
}
func (m *mockWahaProvider) UpdateGroupPicture(ctx context.Context, input whatsapp.UpdateGroupPictureInput) error {
	return nil
}
func (m *mockWahaProvider) SetGroupAnnouncement(ctx context.Context, input whatsapp.SetGroupAnnouncementInput) error {
	return nil
}
func (m *mockWahaProvider) SendText(ctx context.Context, input whatsapp.SendTextInput) (*whatsapp.MessageResult, error) {
	return nil, nil
}
func (m *mockWahaProvider) SendImage(ctx context.Context, input whatsapp.SendImageInput) (*whatsapp.MessageResult, error) {
	return nil, nil
}
func (m *mockWahaProvider) SendVoice(ctx context.Context, input whatsapp.SendVoiceInput) (*whatsapp.MessageResult, error) {
	return nil, nil
}
func (m *mockWahaProvider) SendPoll(ctx context.Context, input whatsapp.SendPollInput) (*whatsapp.MessageResult, error) {
	return nil, nil
}
func (m *mockWahaProvider) SetWebhook(ctx context.Context, input whatsapp.SetWebhookInput) error {
	return nil
}
func (m *mockWahaProvider) DownloadMedia(ctx context.Context, input whatsapp.DownloadMediaInput) ([]byte, error) {
	return nil, nil
}

// mockQueueClient implements a minimal queue client for testing
type mockQueueClient struct {
	enqueuedSessions []string
	enqueuedChats    []string
}

func newMockQueueClient() *mockQueueClient {
	return &mockQueueClient{
		enqueuedSessions: make([]string, 0),
		enqueuedChats:    make([]string, 0),
	}
}

// We can't directly test with the real QueueClient since it requires Redis,
// so these tests focus on the service logic with mocked dependencies.

func TestBackfillConfig_Validate(t *testing.T) {
	tests := []struct {
		name     string
		config   BackfillConfig
		expected BackfillConfig
	}{
		{
			name:   "zero values get defaults",
			config: BackfillConfig{},
			expected: BackfillConfig{
				MaxConcurrent:   5,
				Timeout:         10 * time.Minute,
				GapThreshold:    60 * time.Second,
				SkipHealthCheck: false,
				ContinueOnError: false,
			},
		},
		{
			name: "non-zero values preserved",
			config: BackfillConfig{
				MaxConcurrent:   10,
				Timeout:         5 * time.Minute,
				GapThreshold:    30 * time.Second,
				SkipHealthCheck: true,
				ContinueOnError: true,
			},
			expected: BackfillConfig{
				MaxConcurrent:   10,
				Timeout:         5 * time.Minute,
				GapThreshold:    30 * time.Second,
				SkipHealthCheck: true,
				ContinueOnError: true,
			},
		},
		{
			name: "negative values get defaults",
			config: BackfillConfig{
				MaxConcurrent: -1,
				Timeout:       -1,
				GapThreshold:  -1,
			},
			expected: BackfillConfig{
				MaxConcurrent: 5,
				Timeout:       10 * time.Minute,
				GapThreshold:  60 * time.Second,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.config.Validate()

			if tt.config.MaxConcurrent != tt.expected.MaxConcurrent {
				t.Errorf("MaxConcurrent: got %d, want %d", tt.config.MaxConcurrent, tt.expected.MaxConcurrent)
			}
			if tt.config.Timeout != tt.expected.Timeout {
				t.Errorf("Timeout: got %v, want %v", tt.config.Timeout, tt.expected.Timeout)
			}
			if tt.config.GapThreshold != tt.expected.GapThreshold {
				t.Errorf("GapThreshold: got %v, want %v", tt.config.GapThreshold, tt.expected.GapThreshold)
			}
		})
	}
}

func TestDefaultBackfillConfig(t *testing.T) {
	config := DefaultBackfillConfig()

	if config.MaxConcurrent != 5 {
		t.Errorf("MaxConcurrent: got %d, want 5", config.MaxConcurrent)
	}
	if config.Timeout != 10*time.Minute {
		t.Errorf("Timeout: got %v, want 10m", config.Timeout)
	}
	if config.GapThreshold != 60*time.Second {
		t.Errorf("GapThreshold: got %v, want 60s", config.GapThreshold)
	}
	if config.SkipHealthCheck != false {
		t.Errorf("SkipHealthCheck: got %v, want false", config.SkipHealthCheck)
	}
	if config.ContinueOnError != true {
		t.Errorf("ContinueOnError: got %v, want true", config.ContinueOnError)
	}
}

func TestBackfillService_NoActiveSessionsReturnsEarly(t *testing.T) {
	logger := zap.NewNop()
	instanceRepo := &mockInstanceRepo{instances: []models.Instance{}}
	checkpointRepo := newMockCheckpointRepo()
	wahaProvider := newMockWahaProvider()

	svc := NewBackfillService(
		instanceRepo,
		checkpointRepo,
		wahaProvider,
		nil, // No queue client - we're testing the empty case
		nil, // No reconcile service
		logger,
		DefaultBackfillConfig(),
	)

	result, err := svc.RunOnStartup(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalSessions != 0 {
		t.Errorf("TotalSessions: got %d, want 0", result.TotalSessions)
	}
	if result.SessionsChecked != 0 {
		t.Errorf("SessionsChecked: got %d, want 0", result.SessionsChecked)
	}
	if result.SessionsWithGaps != 0 {
		t.Errorf("SessionsWithGaps: got %d, want 0", result.SessionsWithGaps)
	}
}

func TestBackfillService_GetInstancesError(t *testing.T) {
	logger := zap.NewNop()
	instanceRepo := &mockInstanceRepo{err: errors.New("database error")}
	checkpointRepo := newMockCheckpointRepo()
	wahaProvider := newMockWahaProvider()

	svc := NewBackfillService(
		instanceRepo,
		checkpointRepo,
		wahaProvider,
		nil,
		nil,
		logger,
		DefaultBackfillConfig(),
	)

	_, err := svc.RunOnStartup(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, errors.Unwrap(err)) {
		// Just check that the error wraps properly
		if err.Error() == "" {
			t.Error("expected non-empty error message")
		}
	}
}

func TestBackfillService_SkipsDisconnectedSessions(t *testing.T) {
	logger := zap.NewNop()
	instanceRepo := &mockInstanceRepo{
		instances: []models.Instance{
			{ID: 1, InstanceID: "session1", Status: "connected"},
		},
	}
	checkpointRepo := newMockCheckpointRepo()
	wahaProvider := newMockWahaProvider()

	// Set session as disconnected in WAHA
	wahaProvider.SetSessionStatus("session1", whatsapp.SessionStatusDisconnected)

	// Add a checkpoint to avoid the "no checkpoints" path
	checkpointRepo.AddCheckpoint("session1", reconciliation.Checkpoint{
		SessionName:          "session1",
		ChatID:               "chat1@g.us",
		LastMessageTimestamp: time.Now().Add(-1 * time.Hour).UnixMilli(),
		Status:               reconciliation.CheckpointStatusIdle,
	})

	svc := NewBackfillService(
		instanceRepo,
		checkpointRepo,
		wahaProvider,
		nil, // No queue client needed - session should be skipped
		nil,
		logger,
		BackfillConfig{
			MaxConcurrent:   5,
			Timeout:         10 * time.Minute,
			GapThreshold:    60 * time.Second,
			SkipHealthCheck: false,
			ContinueOnError: true,
		},
	)

	result, err := svc.RunOnStartup(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalSessions != 1 {
		t.Errorf("TotalSessions: got %d, want 1", result.TotalSessions)
	}
	// Session should be skipped because it's disconnected
	if result.SessionsWithGaps != 0 {
		t.Errorf("SessionsWithGaps: got %d, want 0", result.SessionsWithGaps)
	}
}

func TestBackfillService_CheckChatForGap_NoMessages(t *testing.T) {
	logger := zap.NewNop()
	instanceRepo := &mockInstanceRepo{}
	checkpointRepo := newMockCheckpointRepo()
	wahaProvider := newMockWahaProvider()

	// Set no messages for the chat
	wahaProvider.SetMessages("session1", "chat1@g.us", []whatsapp.Message{})

	svc := NewBackfillService(
		instanceRepo,
		checkpointRepo,
		wahaProvider,
		nil,
		nil,
		logger,
		DefaultBackfillConfig(),
	)

	checkpoint := reconciliation.Checkpoint{
		SessionName:          "session1",
		ChatID:               "chat1@g.us",
		LastMessageTimestamp: time.Now().Add(-1 * time.Hour).UnixMilli(),
		Status:               reconciliation.CheckpointStatusIdle,
	}

	hasGap, err := svc.checkChatForGap(context.Background(), "session1", checkpoint)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if hasGap {
		t.Error("expected no gap when chat has no messages")
	}
}

func TestBackfillService_CheckChatForGap_ZeroCheckpoint(t *testing.T) {
	logger := zap.NewNop()
	instanceRepo := &mockInstanceRepo{}
	checkpointRepo := newMockCheckpointRepo()
	wahaProvider := newMockWahaProvider()

	// Set a message in WAHA
	wahaProvider.SetMessages("session1", "chat1@g.us", []whatsapp.Message{
		{
			ID:        "msg1",
			ChatID:    "chat1@g.us",
			Timestamp: time.Now(),
			Body:      "Hello",
		},
	})

	svc := NewBackfillService(
		instanceRepo,
		checkpointRepo,
		wahaProvider,
		nil,
		nil,
		logger,
		DefaultBackfillConfig(),
	)

	// Checkpoint with zero timestamp means we've never processed this chat
	checkpoint := reconciliation.Checkpoint{
		SessionName:          "session1",
		ChatID:               "chat1@g.us",
		LastMessageTimestamp: 0,
		Status:               reconciliation.CheckpointStatusIdle,
	}

	hasGap, err := svc.checkChatForGap(context.Background(), "session1", checkpoint)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !hasGap {
		t.Error("expected gap when checkpoint timestamp is 0")
	}
}

func TestBackfillService_CheckChatForGap_WithinThreshold(t *testing.T) {
	logger := zap.NewNop()
	instanceRepo := &mockInstanceRepo{}
	checkpointRepo := newMockCheckpointRepo()
	wahaProvider := newMockWahaProvider()

	now := time.Now()

	// Set a message in WAHA that's only 30 seconds newer than checkpoint
	wahaProvider.SetMessages("session1", "chat1@g.us", []whatsapp.Message{
		{
			ID:        "msg1",
			ChatID:    "chat1@g.us",
			Timestamp: now,
			Body:      "Hello",
		},
	})

	svc := NewBackfillService(
		instanceRepo,
		checkpointRepo,
		wahaProvider,
		nil,
		nil,
		logger,
		BackfillConfig{
			MaxConcurrent: 5,
			Timeout:       10 * time.Minute,
			GapThreshold:  60 * time.Second, // 60 second threshold
		},
	)

	// Checkpoint is 30 seconds ago - within threshold
	checkpoint := reconciliation.Checkpoint{
		SessionName:          "session1",
		ChatID:               "chat1@g.us",
		LastMessageTimestamp: now.Add(-30 * time.Second).UnixMilli(),
		LastMessageID:        "msg1", // Same message ID
		Status:               reconciliation.CheckpointStatusIdle,
	}

	hasGap, err := svc.checkChatForGap(context.Background(), "session1", checkpoint)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if hasGap {
		t.Error("expected no gap when within threshold and same message ID")
	}
}

func TestBackfillService_CheckChatForGap_ExceedsThreshold(t *testing.T) {
	logger := zap.NewNop()
	instanceRepo := &mockInstanceRepo{}
	checkpointRepo := newMockCheckpointRepo()
	wahaProvider := newMockWahaProvider()

	now := time.Now()

	// Set a message in WAHA that's 2 minutes newer than checkpoint
	wahaProvider.SetMessages("session1", "chat1@g.us", []whatsapp.Message{
		{
			ID:        "msg2",
			ChatID:    "chat1@g.us",
			Timestamp: now,
			Body:      "Hello",
		},
	})

	svc := NewBackfillService(
		instanceRepo,
		checkpointRepo,
		wahaProvider,
		nil,
		nil,
		logger,
		BackfillConfig{
			MaxConcurrent: 5,
			Timeout:       10 * time.Minute,
			GapThreshold:  60 * time.Second, // 60 second threshold
		},
	)

	// Checkpoint is 2 minutes ago - exceeds threshold
	checkpoint := reconciliation.Checkpoint{
		SessionName:          "session1",
		ChatID:               "chat1@g.us",
		LastMessageTimestamp: now.Add(-2 * time.Minute).UnixMilli(),
		LastMessageID:        "msg1",
		Status:               reconciliation.CheckpointStatusIdle,
	}

	hasGap, err := svc.checkChatForGap(context.Background(), "session1", checkpoint)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !hasGap {
		t.Error("expected gap when time difference exceeds threshold")
	}
}

func TestBackfillService_CheckChatForGap_DifferentMessageID(t *testing.T) {
	logger := zap.NewNop()
	instanceRepo := &mockInstanceRepo{}
	checkpointRepo := newMockCheckpointRepo()
	wahaProvider := newMockWahaProvider()

	now := time.Now()

	// Set a different message in WAHA
	wahaProvider.SetMessages("session1", "chat1@g.us", []whatsapp.Message{
		{
			ID:        "msg2",
			ChatID:    "chat1@g.us",
			Timestamp: now,
			Body:      "New message",
		},
	})

	svc := NewBackfillService(
		instanceRepo,
		checkpointRepo,
		wahaProvider,
		nil,
		nil,
		logger,
		BackfillConfig{
			MaxConcurrent: 5,
			Timeout:       10 * time.Minute,
			GapThreshold:  60 * time.Second,
		},
	)

	// Checkpoint has a different message ID and older timestamp
	checkpoint := reconciliation.Checkpoint{
		SessionName:          "session1",
		ChatID:               "chat1@g.us",
		LastMessageTimestamp: now.Add(-30 * time.Second).UnixMilli(), // Within threshold by time
		LastMessageID:        "msg1",                                 // Different message ID
		Status:               reconciliation.CheckpointStatusIdle,
	}

	hasGap, err := svc.checkChatForGap(context.Background(), "session1", checkpoint)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !hasGap {
		t.Error("expected gap when message IDs differ and latest is newer")
	}
}
