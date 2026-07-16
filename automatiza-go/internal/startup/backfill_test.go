package startup

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/models"
	"automatiza-go/internal/services/reconciliation"

	"go.uber.org/zap"
)

// ---------------------------------------------------------------------------
// Mock types
// ---------------------------------------------------------------------------

// mockInstanceRepo implements repository.InstanceRepository for testing.
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

func (m *mockInstanceRepo) GetUserInstances(context.Context, string) ([]models.Instance, error) {
	return nil, nil
}
func (m *mockInstanceRepo) GetInstanceByID(context.Context, int) (*models.Instance, error) {
	return nil, nil
}
func (m *mockInstanceRepo) GetInstanceByName(context.Context, string) (*models.Instance, error) {
	return nil, nil
}
func (m *mockInstanceRepo) GetInstanceByUserAndName(context.Context, string, string) (*models.Instance, error) {
	return nil, nil
}
func (m *mockInstanceRepo) UpsertInstance(context.Context, string, *models.Instance) (*models.Instance, error) {
	return nil, nil
}
func (m *mockInstanceRepo) UpdateInstanceStatus(context.Context, int, string) error { return nil }
func (m *mockInstanceRepo) DeleteInstance(context.Context, int) error               { return nil }
func (m *mockInstanceRepo) SetDefaultInstance(context.Context, string, int) error   { return nil }
func (m *mockInstanceRepo) CountUserInstances(context.Context, string) (int, error) { return 0, nil }
func (m *mockInstanceRepo) UpdateInstanceLabel(context.Context, string, int, string) error {
	return nil
}
func (m *mockInstanceRepo) UpdateInstancePhoneNumber(context.Context, int, string) error {
	return nil
}
func (m *mockInstanceRepo) SupersedePriorInstances(context.Context, string, int, string) (int, error) {
	return 0, nil
}

// mockCheckpointRepo implements reconciliation.CheckpointRepository for testing.
type mockCheckpointRepo struct {
	checkpoints     map[string][]reconciliation.Checkpoint
	getBySessionErr error
}

func newMockCheckpointRepo() *mockCheckpointRepo {
	return &mockCheckpointRepo{checkpoints: make(map[string][]reconciliation.Checkpoint)}
}

func (m *mockCheckpointRepo) AddCheckpoint(sessionName string, cp reconciliation.Checkpoint) {
	m.checkpoints[sessionName] = append(m.checkpoints[sessionName], cp)
}

func (m *mockCheckpointRepo) GetBySession(ctx context.Context, sessionName string) ([]reconciliation.Checkpoint, error) {
	if m.getBySessionErr != nil {
		return nil, m.getBySessionErr
	}
	return m.checkpoints[sessionName], nil
}

func (m *mockCheckpointRepo) GetOrCreate(ctx context.Context, sessionName, chatID string) (*reconciliation.Checkpoint, error) {
	if cps, ok := m.checkpoints[sessionName]; ok {
		for i := range cps {
			if cps[i].ChatID == chatID {
				return &cps[i], nil
			}
		}
	}
	cp := reconciliation.Checkpoint{
		SessionName:          sessionName,
		ChatID:               chatID,
		LastMessageTimestamp: 0,
		LastMessageID:        "",
		LastRunAt:            time.Now(),
		Status:               reconciliation.CheckpointStatusIdle,
	}
	m.checkpoints[sessionName] = append(m.checkpoints[sessionName], cp)
	return &cp, nil
}

func (m *mockCheckpointRepo) MarkRunning(context.Context, string, string) error { return nil }
func (m *mockCheckpointRepo) MarkIdle(context.Context, string, string) error    { return nil }
func (m *mockCheckpointRepo) MarkFailed(context.Context, string, string) error  { return nil }
func (m *mockCheckpointRepo) UpdateCheckpoint(context.Context, string, string, int64, string) error {
	return nil
}

func (m *mockCheckpointRepo) GetActiveSessionsChats(ctx context.Context) ([]reconciliation.SessionChat, error) {
	var result []reconciliation.SessionChat
	for sessionName, cps := range m.checkpoints {
		for _, cp := range cps {
			result = append(result, reconciliation.SessionChat{SessionName: sessionName, ChatID: cp.ChatID})
		}
	}
	return result, nil
}

// mockReconcileEnqueuer implements reconcileEnqueuer for testing.
type mockReconcileEnqueuer struct {
	mu               sync.Mutex
	enqueuedSessions []string
	enqueuedChats    []enqueuedChat
	sessionErr       error
	chatErr          error
}

type enqueuedChat struct {
	session string
	chatID  string
}

func newMockEnqueuer() *mockReconcileEnqueuer {
	return &mockReconcileEnqueuer{
		enqueuedSessions: make([]string, 0),
		enqueuedChats:    make([]enqueuedChat, 0),
	}
}

func (m *mockReconcileEnqueuer) EnqueueReconcileSession(_ context.Context, sessionName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enqueuedSessions = append(m.enqueuedSessions, sessionName)
	return m.sessionErr
}

func (m *mockReconcileEnqueuer) EnqueueReconcileChat(_ context.Context, sessionName, chatID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enqueuedChats = append(m.enqueuedChats, enqueuedChat{session: sessionName, chatID: chatID})
	return m.chatErr
}

// mockReconciliationRunner implements reconciliationRunner for testing.
type mockReconciliationRunner struct {
	mu                 sync.Mutex
	onStartupCalled    bool
	onStartupErr       error
	reconciledSessions []string
	reconcileErr       error
}

func newMockReconciler() *mockReconciliationRunner {
	return &mockReconciliationRunner{
		reconciledSessions: make([]string, 0),
	}
}

func (m *mockReconciliationRunner) OnStartup(context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onStartupCalled = true
	return m.onStartupErr
}

func (m *mockReconciliationRunner) ReconcileSession(_ context.Context, sessionName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reconciledSessions = append(m.reconciledSessions, sessionName)
	return m.reconcileErr
}

// mockWahaProvider implements whatsapp.WhatsAppProvider for testing.
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

func (m *mockWahaProvider) GetSessionStatus(_ context.Context, sessionName string) (*whatsapp.SessionInfo, error) {
	if m.statusErr != nil {
		return nil, m.statusErr
	}
	if status, ok := m.sessionStatus[sessionName]; ok {
		return status, nil
	}
	return &whatsapp.SessionInfo{Name: sessionName, Status: whatsapp.SessionStatusConnected}, nil
}

func (m *mockWahaProvider) GetMessages(_ context.Context, input whatsapp.GetMessagesInput) (*whatsapp.MessagesResult, error) {
	if m.messagesErr != nil {
		return nil, m.messagesErr
	}
	key := input.SessionName + ":" + input.ChatID
	return &whatsapp.MessagesResult{Messages: m.messages[key]}, nil
}

func (m *mockWahaProvider) SetSessionStatus(sessionName string, status whatsapp.SessionStatus) {
	m.sessionStatus[sessionName] = &whatsapp.SessionInfo{Name: sessionName, Status: status}
}

func (m *mockWahaProvider) SetMessages(sessionName, chatID string, messages []whatsapp.Message) {
	m.messages[sessionName+":"+chatID] = messages
}

// Stub implementations for other WhatsAppProvider interface methods.
func (m *mockWahaProvider) CreateSession(context.Context, whatsapp.CreateSessionInput) (*whatsapp.SessionInfo, error) {
	return nil, nil
}
func (m *mockWahaProvider) DeleteSession(context.Context, string) error { return nil }
func (m *mockWahaProvider) UnpairSession(context.Context, string) error { return nil }
func (m *mockWahaProvider) GetQRCode(context.Context, string) (*whatsapp.QRCodeData, error) {
	return nil, nil
}
func (m *mockWahaProvider) RestartSession(context.Context, string) error { return nil }
func (m *mockWahaProvider) LogoutSession(context.Context, string) error  { return nil }
func (m *mockWahaProvider) GetGroups(context.Context, string) ([]whatsapp.GroupInfo, error) {
	return nil, nil
}
func (m *mockWahaProvider) GetGroupParticipants(context.Context, string, string) ([]whatsapp.ParticipantInfo, error) {
	return nil, nil
}
func (m *mockWahaProvider) GetGroupInviteCode(context.Context, string, string) (string, error) {
	return "", nil
}
func (m *mockWahaProvider) UpdateGroupSubject(context.Context, whatsapp.UpdateGroupSubjectInput) error {
	return nil
}
func (m *mockWahaProvider) UpdateGroupDescription(context.Context, whatsapp.UpdateGroupDescriptionInput) error {
	return nil
}
func (m *mockWahaProvider) UpdateGroupPicture(context.Context, whatsapp.UpdateGroupPictureInput) error {
	return nil
}
func (m *mockWahaProvider) SetGroupAnnouncement(context.Context, whatsapp.SetGroupAnnouncementInput) error {
	return nil
}
func (m *mockWahaProvider) SendText(context.Context, whatsapp.SendTextInput) (*whatsapp.MessageResult, error) {
	return nil, nil
}
func (m *mockWahaProvider) SendImage(context.Context, whatsapp.SendImageInput) (*whatsapp.MessageResult, error) {
	return nil, nil
}
func (m *mockWahaProvider) SendVoice(context.Context, whatsapp.SendVoiceInput) (*whatsapp.MessageResult, error) {
	return nil, nil
}
func (m *mockWahaProvider) SendPoll(context.Context, whatsapp.SendPollInput) (*whatsapp.MessageResult, error) {
	return nil, nil
}
func (m *mockWahaProvider) SendLinkCustomPreview(context.Context, whatsapp.SendLinkPreviewInput) (*whatsapp.MessageResult, error) {
	return nil, nil
}
func (m *mockWahaProvider) StartTyping(context.Context, whatsapp.TypingInput) error      { return nil }
func (m *mockWahaProvider) StopTyping(context.Context, whatsapp.TypingInput) error       { return nil }
func (m *mockWahaProvider) SendSeen(context.Context, whatsapp.SendSeenInput) error       { return nil }
func (m *mockWahaProvider) SetPresence(context.Context, whatsapp.SetPresenceInput) error { return nil }
func (m *mockWahaProvider) SendVideo(context.Context, whatsapp.SendVideoInput) (*whatsapp.MessageResult, error) {
	return nil, nil
}
func (m *mockWahaProvider) SendDocument(context.Context, whatsapp.SendDocumentInput) (*whatsapp.MessageResult, error) {
	return nil, nil
}
func (m *mockWahaProvider) SetWebhook(context.Context, whatsapp.SetWebhookInput) error { return nil }
func (m *mockWahaProvider) AddGroupMembers(_ context.Context, input whatsapp.AddGroupMembersInput) (*whatsapp.MemberOperationResult, error) {
	return &whatsapp.MemberOperationResult{SuccessCount: len(input.PhoneNumbers)}, nil
}
func (m *mockWahaProvider) RemoveGroupMembers(_ context.Context, input whatsapp.RemoveGroupMembersInput) (*whatsapp.MemberOperationResult, error) {
	return &whatsapp.MemberOperationResult{SuccessCount: len(input.PhoneNumbers)}, nil
}
func (m *mockWahaProvider) LeaveGroup(context.Context, whatsapp.LeaveGroupInput) error { return nil }
func (m *mockWahaProvider) PromoteMember(context.Context, whatsapp.PromoteMemberInput) error {
	return nil
}
func (m *mockWahaProvider) DemoteMember(context.Context, whatsapp.DemoteMemberInput) error {
	return nil
}
func (m *mockWahaProvider) ClearStatusCache(string) {}
func (m *mockWahaProvider) DownloadMedia(context.Context, whatsapp.DownloadMediaInput) (*whatsapp.DownloadMediaResult, error) {
	return nil, nil
}
func (m *mockWahaProvider) DownloadMediaByURL(context.Context, string) (*whatsapp.DownloadMediaResult, error) {
	return nil, nil
}
func (m *mockWahaProvider) RedownloadMedia(context.Context, string, string, string) (*whatsapp.DownloadMediaResult, error) {
	return nil, nil
}

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

func newTestService(
	instanceRepo *mockInstanceRepo,
	checkpointRepo *mockCheckpointRepo,
	wahaProvider *mockWahaProvider,
	enqueuer *mockReconcileEnqueuer,
	reconciler *mockReconciliationRunner,
	config BackfillConfig,
) *BackfillService {
	config.Validate()

	var enq ReconcileEnqueuer
	if enqueuer != nil {
		enq = enqueuer
	}

	var rec ReconciliationRunner
	if reconciler != nil {
		rec = reconciler
	}

	return NewBackfillService(
		instanceRepo,
		checkpointRepo,
		wahaProvider,
		enq,
		rec,
		zap.NewNop(),
		config,
	)
}

// ---------------------------------------------------------------------------
// BackfillConfig tests
// ---------------------------------------------------------------------------

func TestBackfillConfig_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		config  BackfillConfig
		wantMax int
		wantTO  time.Duration
		wantGap time.Duration
	}{
		{"zero values get defaults", BackfillConfig{}, 5, 10 * time.Minute, 60 * time.Second},
		{"positive values preserved", BackfillConfig{MaxConcurrent: 10, Timeout: 5 * time.Minute, GapThreshold: 30 * time.Second}, 10, 5 * time.Minute, 30 * time.Second},
		{"negative values get defaults", BackfillConfig{MaxConcurrent: -1, Timeout: -1, GapThreshold: -1}, 5, 10 * time.Minute, 60 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.config.Validate()
			if tt.config.MaxConcurrent != tt.wantMax {
				t.Errorf("MaxConcurrent = %d, want %d", tt.config.MaxConcurrent, tt.wantMax)
			}
			if tt.config.Timeout != tt.wantTO {
				t.Errorf("Timeout = %v, want %v", tt.config.Timeout, tt.wantTO)
			}
			if tt.config.GapThreshold != tt.wantGap {
				t.Errorf("GapThreshold = %v, want %v", tt.config.GapThreshold, tt.wantGap)
			}
		})
	}
}

func TestBackfillConfig_Validate_PreservesBooleans(t *testing.T) {
	t.Parallel()

	c := BackfillConfig{
		SkipHealthCheck: true,
		ContinueOnError: true,
		MaxConcurrent:   1,
		Timeout:         time.Second,
		GapThreshold:    time.Second,
	}
	c.Validate()

	if !c.SkipHealthCheck {
		t.Error("SkipHealthCheck should be preserved as true")
	}
	if !c.ContinueOnError {
		t.Error("ContinueOnError should be preserved as true")
	}
}

func TestDefaultBackfillConfig(t *testing.T) {
	t.Parallel()

	c := DefaultBackfillConfig()

	// Defaults raised (8/20m) so the boot backfill can cover ~600 sessions
	// without truncating at the deadline; both are env-tunable (BACKFILL_*).
	if c.MaxConcurrent != 8 {
		t.Errorf("MaxConcurrent = %d, want 8", c.MaxConcurrent)
	}
	if c.Timeout != 20*time.Minute {
		t.Errorf("Timeout = %v, want 20m", c.Timeout)
	}
	if c.GapThreshold != 60*time.Second {
		t.Errorf("GapThreshold = %v, want 60s", c.GapThreshold)
	}
	if c.SkipHealthCheck {
		t.Error("SkipHealthCheck should be false")
	}
	if !c.ContinueOnError {
		t.Error("ContinueOnError should be true")
	}
}

// ---------------------------------------------------------------------------
// NewBackfillService tests
// ---------------------------------------------------------------------------

func TestNewBackfillService_ValidatesConfig(t *testing.T) {
	t.Parallel()

	svc := NewBackfillService(
		&mockInstanceRepo{}, newMockCheckpointRepo(), newMockWahaProvider(),
		nil, nil, zap.NewNop(), BackfillConfig{},
	)

	if svc.config.MaxConcurrent != 5 {
		t.Errorf("expected validated MaxConcurrent=5, got %d", svc.config.MaxConcurrent)
	}
	if svc.config.Timeout != 10*time.Minute {
		t.Errorf("expected validated Timeout=10m, got %v", svc.config.Timeout)
	}
	if svc.config.GapThreshold != 60*time.Second {
		t.Errorf("expected validated GapThreshold=60s, got %v", svc.config.GapThreshold)
	}
}

func TestNewBackfillService_NilDeps(t *testing.T) {
	t.Parallel()

	svc := NewBackfillService(
		&mockInstanceRepo{}, newMockCheckpointRepo(), newMockWahaProvider(),
		nil, nil, zap.NewNop(), DefaultBackfillConfig(),
	)

	if svc.enqueuer != nil {
		t.Error("expected nil enqueuer when queueClient is nil")
	}
	if svc.reconciler != nil {
		t.Error("expected nil reconciler when reconcileSvc is nil")
	}
}

// ---------------------------------------------------------------------------
// RunOnStartup tests
// ---------------------------------------------------------------------------

func TestRunOnStartup_NoActiveSessions(t *testing.T) {
	t.Parallel()

	svc := newTestService(
		&mockInstanceRepo{instances: []models.Instance{}},
		newMockCheckpointRepo(), newMockWahaProvider(), nil, nil,
		DefaultBackfillConfig(),
	)

	result, err := svc.RunOnStartup(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalSessions != 0 {
		t.Errorf("TotalSessions = %d, want 0", result.TotalSessions)
	}
	if result.SessionsChecked != 0 {
		t.Errorf("SessionsChecked = %d, want 0", result.SessionsChecked)
	}
	if result.SessionsWithGaps != 0 {
		t.Errorf("SessionsWithGaps = %d, want 0", result.SessionsWithGaps)
	}
	if result.Duration <= 0 {
		t.Error("Duration should be positive")
	}
}

func TestRunOnStartup_GetInstancesError(t *testing.T) {
	t.Parallel()

	svc := newTestService(
		&mockInstanceRepo{err: errors.New("db down")},
		newMockCheckpointRepo(), newMockWahaProvider(), nil, nil,
		DefaultBackfillConfig(),
	)

	_, err := svc.RunOnStartup(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestRunOnStartup_SkipsDisconnectedSessions(t *testing.T) {
	t.Parallel()

	wp := newMockWahaProvider()
	wp.SetSessionStatus("session1", whatsapp.SessionStatusDisconnected)

	cp := newMockCheckpointRepo()
	cp.AddCheckpoint("session1", reconciliation.Checkpoint{
		SessionName:          "session1",
		ChatID:               "chat1@g.us",
		LastMessageTimestamp: time.Now().Add(-time.Hour).UnixMilli(),
		Status:               reconciliation.CheckpointStatusIdle,
	})

	svc := newTestService(
		&mockInstanceRepo{instances: []models.Instance{{ID: 1, InstanceID: "session1", Status: "connected"}}},
		cp, wp, newMockEnqueuer(), nil,
		BackfillConfig{MaxConcurrent: 5, Timeout: 10 * time.Minute, GapThreshold: 60 * time.Second, ContinueOnError: true},
	)

	result, err := svc.RunOnStartup(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalSessions != 1 {
		t.Errorf("TotalSessions = %d, want 1", result.TotalSessions)
	}
	if result.SessionsWithGaps != 0 {
		t.Errorf("SessionsWithGaps = %d, want 0", result.SessionsWithGaps)
	}
}

func TestRunOnStartup_NoCheckpoints_EnqueuesSessionReconciliation(t *testing.T) {
	t.Parallel()

	enq := newMockEnqueuer()

	svc := newTestService(
		&mockInstanceRepo{instances: []models.Instance{{ID: 1, InstanceID: "session1", Status: "connected"}}},
		newMockCheckpointRepo(), // empty — no checkpoints
		newMockWahaProvider(),
		enq, nil,
		BackfillConfig{MaxConcurrent: 5, Timeout: 10 * time.Minute, GapThreshold: 60 * time.Second, SkipHealthCheck: true, ContinueOnError: true},
	)

	result, err := svc.RunOnStartup(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SessionsWithGaps != 1 {
		t.Errorf("SessionsWithGaps = %d, want 1", result.SessionsWithGaps)
	}
	if result.ReconciliationsEnqueued != 1 {
		t.Errorf("ReconciliationsEnqueued = %d, want 1", result.ReconciliationsEnqueued)
	}
	if len(enq.enqueuedSessions) != 1 || enq.enqueuedSessions[0] != "session1" {
		t.Errorf("expected session1 enqueued, got %v", enq.enqueuedSessions)
	}
}

func TestRunOnStartup_WithGaps_EnqueuesChatReconciliation(t *testing.T) {
	t.Parallel()

	now := time.Now()
	wp := newMockWahaProvider()
	wp.SetMessages("session1", "chat1@g.us", []whatsapp.Message{
		{ID: "msg2", ChatID: "chat1@g.us", Timestamp: now, Body: "Hello"},
	})

	cp := newMockCheckpointRepo()
	cp.AddCheckpoint("session1", reconciliation.Checkpoint{
		SessionName:          "session1",
		ChatID:               "chat1@g.us",
		LastMessageTimestamp: now.Add(-2 * time.Minute).UnixMilli(),
		LastMessageID:        "msg1",
		Status:               reconciliation.CheckpointStatusIdle,
	})

	enq := newMockEnqueuer()

	svc := newTestService(
		&mockInstanceRepo{instances: []models.Instance{{ID: 1, InstanceID: "session1", Status: "connected"}}},
		cp, wp, enq, nil,
		BackfillConfig{MaxConcurrent: 5, Timeout: 10 * time.Minute, GapThreshold: 60 * time.Second, SkipHealthCheck: true, ContinueOnError: true},
	)

	result, err := svc.RunOnStartup(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SessionsWithGaps != 1 {
		t.Errorf("SessionsWithGaps = %d, want 1", result.SessionsWithGaps)
	}
	if result.ReconciliationsEnqueued != 1 {
		t.Errorf("ReconciliationsEnqueued = %d, want 1", result.ReconciliationsEnqueued)
	}
	if len(enq.enqueuedChats) != 1 {
		t.Fatalf("expected 1 chat enqueued, got %d", len(enq.enqueuedChats))
	}
	if enq.enqueuedChats[0].chatID != "chat1@g.us" {
		t.Errorf("expected chat1@g.us enqueued, got %s", enq.enqueuedChats[0].chatID)
	}
}

func TestRunOnStartup_NoGaps(t *testing.T) {
	t.Parallel()

	now := time.Now()
	wp := newMockWahaProvider()
	wp.SetMessages("session1", "chat1@g.us", []whatsapp.Message{
		{ID: "msg1", ChatID: "chat1@g.us", Timestamp: now, Body: "Hello"},
	})

	cp := newMockCheckpointRepo()
	cp.AddCheckpoint("session1", reconciliation.Checkpoint{
		SessionName:          "session1",
		ChatID:               "chat1@g.us",
		LastMessageTimestamp: now.Add(-10 * time.Second).UnixMilli(),
		LastMessageID:        "msg1",
		Status:               reconciliation.CheckpointStatusIdle,
	})

	enq := newMockEnqueuer()

	svc := newTestService(
		&mockInstanceRepo{instances: []models.Instance{{ID: 1, InstanceID: "session1", Status: "connected"}}},
		cp, wp, enq, nil,
		BackfillConfig{MaxConcurrent: 5, Timeout: 10 * time.Minute, GapThreshold: 60 * time.Second, SkipHealthCheck: true, ContinueOnError: true},
	)

	result, err := svc.RunOnStartup(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SessionsWithGaps != 0 {
		t.Errorf("SessionsWithGaps = %d, want 0", result.SessionsWithGaps)
	}
	if result.ReconciliationsEnqueued != 0 {
		t.Errorf("ReconciliationsEnqueued = %d, want 0", result.ReconciliationsEnqueued)
	}
	if len(enq.enqueuedChats) != 0 {
		t.Errorf("expected 0 chats enqueued, got %d", len(enq.enqueuedChats))
	}
}

func TestRunOnStartup_ContextCancelled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	svc := newTestService(
		&mockInstanceRepo{instances: []models.Instance{
			{ID: 1, InstanceID: "s1"}, {ID: 2, InstanceID: "s2"}, {ID: 3, InstanceID: "s3"},
		}},
		newMockCheckpointRepo(), newMockWahaProvider(), newMockEnqueuer(), nil,
		BackfillConfig{MaxConcurrent: 1, Timeout: 10 * time.Minute, GapThreshold: 60 * time.Second, SkipHealthCheck: true, ContinueOnError: true},
	)

	result, err := svc.RunOnStartup(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalSessions != 3 {
		t.Errorf("TotalSessions = %d, want 3", result.TotalSessions)
	}
	// Goroutines exit via ctx.Done() in semaphore select, so SessionsChecked may be 0
	if result.Duration <= 0 {
		t.Error("Duration should be positive")
	}
}

func TestRunOnStartup_ContinueOnError_False(t *testing.T) {
	t.Parallel()

	cp := newMockCheckpointRepo()
	cp.getBySessionErr = errors.New("checkpoint error")

	svc := newTestService(
		&mockInstanceRepo{instances: []models.Instance{
			{ID: 1, InstanceID: "s1"}, {ID: 2, InstanceID: "s2"},
		}},
		cp, newMockWahaProvider(), newMockEnqueuer(), nil,
		BackfillConfig{MaxConcurrent: 1, Timeout: 10 * time.Minute, GapThreshold: 60 * time.Second, SkipHealthCheck: true, ContinueOnError: false},
	)

	result, err := svc.RunOnStartup(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Errors) == 0 {
		t.Error("expected at least one error")
	}
}

func TestRunOnStartup_ContinueOnError_True_CollectsErrors(t *testing.T) {
	t.Parallel()

	cp := newMockCheckpointRepo()
	cp.getBySessionErr = errors.New("checkpoint error")

	svc := newTestService(
		&mockInstanceRepo{instances: []models.Instance{
			{ID: 1, InstanceID: "s1"}, {ID: 2, InstanceID: "s2"},
		}},
		cp, newMockWahaProvider(), newMockEnqueuer(), nil,
		BackfillConfig{MaxConcurrent: 5, Timeout: 10 * time.Minute, GapThreshold: 60 * time.Second, SkipHealthCheck: true, ContinueOnError: true},
	)

	result, err := svc.RunOnStartup(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Errors) != 2 {
		t.Errorf("expected 2 errors, got %d", len(result.Errors))
	}
	if result.SessionsChecked != 2 {
		t.Errorf("SessionsChecked = %d, want 2", result.SessionsChecked)
	}
}

func TestRunOnStartup_MultipleSessions_Concurrent(t *testing.T) {
	t.Parallel()

	now := time.Now()
	wp := newMockWahaProvider()
	cp := newMockCheckpointRepo()

	// Session 1: has gap (checkpoint is 5 min old)
	wp.SetMessages("s1", "chat1@g.us", []whatsapp.Message{
		{ID: "msg2", ChatID: "chat1@g.us", Timestamp: now, Body: "new"},
	})
	cp.AddCheckpoint("s1", reconciliation.Checkpoint{
		SessionName:          "s1",
		ChatID:               "chat1@g.us",
		LastMessageTimestamp: now.Add(-5 * time.Minute).UnixMilli(),
		LastMessageID:        "msg1",
		Status:               reconciliation.CheckpointStatusIdle,
	})

	// Session 2: no gap (within threshold, same message ID)
	wp.SetMessages("s2", "chat2@g.us", []whatsapp.Message{
		{ID: "msg1", ChatID: "chat2@g.us", Timestamp: now, Body: "same"},
	})
	cp.AddCheckpoint("s2", reconciliation.Checkpoint{
		SessionName:          "s2",
		ChatID:               "chat2@g.us",
		LastMessageTimestamp: now.Add(-5 * time.Second).UnixMilli(),
		LastMessageID:        "msg1",
		Status:               reconciliation.CheckpointStatusIdle,
	})

	// Session 3: no checkpoints → enqueue session reconciliation

	enq := newMockEnqueuer()

	svc := newTestService(
		&mockInstanceRepo{instances: []models.Instance{
			{ID: 1, InstanceID: "s1"}, {ID: 2, InstanceID: "s2"}, {ID: 3, InstanceID: "s3"},
		}},
		cp, wp, enq, nil,
		BackfillConfig{MaxConcurrent: 3, Timeout: 10 * time.Minute, GapThreshold: 60 * time.Second, SkipHealthCheck: true, ContinueOnError: true},
	)

	result, err := svc.RunOnStartup(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalSessions != 3 {
		t.Errorf("TotalSessions = %d, want 3", result.TotalSessions)
	}
	if result.SessionsChecked != 3 {
		t.Errorf("SessionsChecked = %d, want 3", result.SessionsChecked)
	}
	if result.SessionsWithGaps != 2 {
		t.Errorf("SessionsWithGaps = %d, want 2 (s1 gap + s3 no checkpoints)", result.SessionsWithGaps)
	}
	if result.ReconciliationsEnqueued != 2 {
		t.Errorf("ReconciliationsEnqueued = %d, want 2", result.ReconciliationsEnqueued)
	}
}

// ---------------------------------------------------------------------------
// processSession tests
// ---------------------------------------------------------------------------

func TestProcessSession_HealthCheckError_Continues(t *testing.T) {
	t.Parallel()

	wp := newMockWahaProvider()
	wp.statusErr = errors.New("waha unreachable")

	cp := newMockCheckpointRepo()
	cp.AddCheckpoint("session1", reconciliation.Checkpoint{
		SessionName:          "session1",
		ChatID:               "chat1@g.us",
		LastMessageTimestamp: time.Now().UnixMilli(),
		LastMessageID:        "msg1",
		Status:               reconciliation.CheckpointStatusIdle,
	})
	wp.SetMessages("session1", "chat1@g.us", []whatsapp.Message{
		{ID: "msg1", ChatID: "chat1@g.us", Timestamp: time.Now(), Body: "test"},
	})

	svc := newTestService(
		&mockInstanceRepo{}, cp, wp, newMockEnqueuer(), nil,
		BackfillConfig{MaxConcurrent: 5, Timeout: 10 * time.Minute, GapThreshold: 60 * time.Second, SkipHealthCheck: false, ContinueOnError: true},
	)

	// processSession should continue despite health check error
	hasGap, _, err := svc.processSession(context.Background(), "session1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// No gap because message matches checkpoint
	if hasGap {
		t.Error("expected no gap since message matches checkpoint")
	}
}

func TestProcessSession_SkipHealthCheck(t *testing.T) {
	t.Parallel()

	wp := newMockWahaProvider()
	wp.SetSessionStatus("session1", whatsapp.SessionStatusDisconnected)

	// No checkpoints → should enqueue session
	enq := newMockEnqueuer()

	svc := newTestService(
		&mockInstanceRepo{}, newMockCheckpointRepo(), wp, enq, nil,
		BackfillConfig{MaxConcurrent: 5, Timeout: 10 * time.Minute, GapThreshold: 60 * time.Second, SkipHealthCheck: true, ContinueOnError: true},
	)

	hasGap, count, err := svc.processSession(context.Background(), "session1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasGap {
		t.Error("expected gap (no checkpoints)")
	}
	if count != 1 {
		t.Errorf("enqueued = %d, want 1", count)
	}
}

func TestProcessSession_GetBySessionError(t *testing.T) {
	t.Parallel()

	cp := newMockCheckpointRepo()
	cp.getBySessionErr = errors.New("db error")

	svc := newTestService(
		&mockInstanceRepo{}, cp, newMockWahaProvider(), newMockEnqueuer(), nil,
		BackfillConfig{MaxConcurrent: 5, Timeout: 10 * time.Minute, GapThreshold: 60 * time.Second, SkipHealthCheck: true, ContinueOnError: true},
	)

	_, _, err := svc.processSession(context.Background(), "session1")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestProcessSession_NoCheckpoints_EnqueueError(t *testing.T) {
	t.Parallel()

	enq := newMockEnqueuer()
	enq.sessionErr = errors.New("redis down")

	svc := newTestService(
		&mockInstanceRepo{}, newMockCheckpointRepo(), newMockWahaProvider(), enq, nil,
		BackfillConfig{MaxConcurrent: 5, Timeout: 10 * time.Minute, GapThreshold: 60 * time.Second, SkipHealthCheck: true, ContinueOnError: true},
	)

	_, _, err := svc.processSession(context.Background(), "session1")
	if err == nil {
		t.Fatal("expected error from enqueue")
	}
}

func TestProcessSession_ChatGapCheckError_Continues(t *testing.T) {
	t.Parallel()

	wp := newMockWahaProvider()
	wp.messagesErr = errors.New("waha error")

	cp := newMockCheckpointRepo()
	cp.AddCheckpoint("session1", reconciliation.Checkpoint{
		SessionName:          "session1",
		ChatID:               "chat1@g.us",
		LastMessageTimestamp: time.Now().Add(-5 * time.Minute).UnixMilli(),
		Status:               reconciliation.CheckpointStatusIdle,
	})
	cp.AddCheckpoint("session1", reconciliation.Checkpoint{
		SessionName:          "session1",
		ChatID:               "chat2@g.us",
		LastMessageTimestamp: time.Now().Add(-5 * time.Minute).UnixMilli(),
		Status:               reconciliation.CheckpointStatusIdle,
	})

	svc := newTestService(
		&mockInstanceRepo{}, cp, wp, newMockEnqueuer(), nil,
		BackfillConfig{MaxConcurrent: 5, Timeout: 10 * time.Minute, GapThreshold: 60 * time.Second, SkipHealthCheck: true, ContinueOnError: true},
	)

	hasGap, count, err := svc.processSession(context.Background(), "session1")
	// Should not return error — chat errors are logged and skipped
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasGap {
		t.Error("expected no gap when all chat checks fail")
	}
	if count != 0 {
		t.Errorf("enqueued = %d, want 0", count)
	}
}

func TestProcessSession_EnqueueChatError_Continues(t *testing.T) {
	t.Parallel()

	now := time.Now()
	wp := newMockWahaProvider()
	wp.SetMessages("session1", "chat1@g.us", []whatsapp.Message{
		{ID: "msg2", ChatID: "chat1@g.us", Timestamp: now, Body: "new"},
	})
	wp.SetMessages("session1", "chat2@g.us", []whatsapp.Message{
		{ID: "msg4", ChatID: "chat2@g.us", Timestamp: now, Body: "new"},
	})

	cp := newMockCheckpointRepo()
	cp.AddCheckpoint("session1", reconciliation.Checkpoint{
		SessionName:          "session1",
		ChatID:               "chat1@g.us",
		LastMessageTimestamp: now.Add(-5 * time.Minute).UnixMilli(),
		LastMessageID:        "msg1",
		Status:               reconciliation.CheckpointStatusIdle,
	})
	cp.AddCheckpoint("session1", reconciliation.Checkpoint{
		SessionName:          "session1",
		ChatID:               "chat2@g.us",
		LastMessageTimestamp: now.Add(-5 * time.Minute).UnixMilli(),
		LastMessageID:        "msg3",
		Status:               reconciliation.CheckpointStatusIdle,
	})

	enq := newMockEnqueuer()
	enq.chatErr = errors.New("redis full")

	svc := newTestService(
		&mockInstanceRepo{}, cp, wp, enq, nil,
		BackfillConfig{MaxConcurrent: 5, Timeout: 10 * time.Minute, GapThreshold: 60 * time.Second, SkipHealthCheck: true, ContinueOnError: true},
	)

	hasGap, count, err := svc.processSession(context.Background(), "session1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasGap {
		t.Error("expected gap detection despite enqueue failure")
	}
	// Enqueue failed for both chats, so count stays 0
	if count != 0 {
		t.Errorf("enqueued = %d, want 0 (enqueue failed)", count)
	}
}

func TestProcessSession_MultipleChats_MixedGaps(t *testing.T) {
	t.Parallel()

	now := time.Now()
	wp := newMockWahaProvider()

	// chat1: gap (old checkpoint)
	wp.SetMessages("session1", "chat1@g.us", []whatsapp.Message{
		{ID: "msg2", ChatID: "chat1@g.us", Timestamp: now, Body: "new"},
	})
	// chat2: no gap (within threshold, same ID)
	wp.SetMessages("session1", "chat2@g.us", []whatsapp.Message{
		{ID: "msg3", ChatID: "chat2@g.us", Timestamp: now, Body: "same"},
	})
	// chat3: gap (different message ID, newer)
	wp.SetMessages("session1", "chat3@g.us", []whatsapp.Message{
		{ID: "msg-new", ChatID: "chat3@g.us", Timestamp: now, Body: "diff"},
	})

	cp := newMockCheckpointRepo()
	cp.AddCheckpoint("session1", reconciliation.Checkpoint{
		SessionName:          "session1",
		ChatID:               "chat1@g.us",
		LastMessageTimestamp: now.Add(-5 * time.Minute).UnixMilli(),
		LastMessageID:        "msg1",
		Status:               reconciliation.CheckpointStatusIdle,
	})
	cp.AddCheckpoint("session1", reconciliation.Checkpoint{
		SessionName:          "session1",
		ChatID:               "chat2@g.us",
		LastMessageTimestamp: now.Add(-10 * time.Second).UnixMilli(),
		LastMessageID:        "msg3",
		Status:               reconciliation.CheckpointStatusIdle,
	})
	cp.AddCheckpoint("session1", reconciliation.Checkpoint{
		SessionName:          "session1",
		ChatID:               "chat3@g.us",
		LastMessageTimestamp: now.Add(-10 * time.Second).UnixMilli(),
		LastMessageID:        "msg-old",
		Status:               reconciliation.CheckpointStatusIdle,
	})

	enq := newMockEnqueuer()

	svc := newTestService(
		&mockInstanceRepo{}, cp, wp, enq, nil,
		BackfillConfig{MaxConcurrent: 5, Timeout: 10 * time.Minute, GapThreshold: 60 * time.Second, SkipHealthCheck: true, ContinueOnError: true},
	)

	hasGap, count, err := svc.processSession(context.Background(), "session1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasGap {
		t.Error("expected gap")
	}
	if count != 2 {
		t.Errorf("enqueued = %d, want 2 (chat1 + chat3)", count)
	}
}

// ---------------------------------------------------------------------------
// checkChatForGap tests
// ---------------------------------------------------------------------------

func TestCheckChatForGap_GetMessagesError(t *testing.T) {
	t.Parallel()

	wp := newMockWahaProvider()
	wp.messagesErr = errors.New("waha timeout")

	svc := newTestService(&mockInstanceRepo{}, newMockCheckpointRepo(), wp, nil, nil, DefaultBackfillConfig())

	_, err := svc.checkChatForGap(context.Background(), "s1", reconciliation.Checkpoint{
		SessionName:          "s1",
		ChatID:               "c1@g.us",
		LastMessageTimestamp: time.Now().UnixMilli(),
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCheckChatForGap_NoMessages(t *testing.T) {
	t.Parallel()

	wp := newMockWahaProvider()
	wp.SetMessages("s1", "c1@g.us", []whatsapp.Message{})

	svc := newTestService(&mockInstanceRepo{}, newMockCheckpointRepo(), wp, nil, nil, DefaultBackfillConfig())

	hasGap, err := svc.checkChatForGap(context.Background(), "s1", reconciliation.Checkpoint{
		SessionName:          "s1",
		ChatID:               "c1@g.us",
		LastMessageTimestamp: time.Now().Add(-time.Hour).UnixMilli(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasGap {
		t.Error("expected no gap when no messages")
	}
}

func TestCheckChatForGap_ZeroCheckpointTimestamp(t *testing.T) {
	t.Parallel()

	wp := newMockWahaProvider()
	wp.SetMessages("s1", "c1@g.us", []whatsapp.Message{
		{ID: "msg1", ChatID: "c1@g.us", Timestamp: time.Now(), Body: "hi"},
	})

	svc := newTestService(&mockInstanceRepo{}, newMockCheckpointRepo(), wp, nil, nil, DefaultBackfillConfig())

	hasGap, err := svc.checkChatForGap(context.Background(), "s1", reconciliation.Checkpoint{
		SessionName:          "s1",
		ChatID:               "c1@g.us",
		LastMessageTimestamp: 0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasGap {
		t.Error("expected gap when checkpoint timestamp is 0")
	}
}

func TestCheckChatForGap_WithinThreshold_SameID(t *testing.T) {
	t.Parallel()

	now := time.Now()
	wp := newMockWahaProvider()
	wp.SetMessages("s1", "c1@g.us", []whatsapp.Message{
		{ID: "msg1", ChatID: "c1@g.us", Timestamp: now, Body: "hi"},
	})

	svc := newTestService(&mockInstanceRepo{}, newMockCheckpointRepo(), wp, nil, nil,
		BackfillConfig{MaxConcurrent: 5, Timeout: time.Minute, GapThreshold: 60 * time.Second},
	)

	hasGap, err := svc.checkChatForGap(context.Background(), "s1", reconciliation.Checkpoint{
		SessionName:          "s1",
		ChatID:               "c1@g.us",
		LastMessageTimestamp: now.Add(-30 * time.Second).UnixMilli(),
		LastMessageID:        "msg1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasGap {
		t.Error("expected no gap within threshold and same ID")
	}
}

func TestCheckChatForGap_ExceedsThreshold(t *testing.T) {
	t.Parallel()

	now := time.Now()
	wp := newMockWahaProvider()
	wp.SetMessages("s1", "c1@g.us", []whatsapp.Message{
		{ID: "msg2", ChatID: "c1@g.us", Timestamp: now, Body: "hi"},
	})

	svc := newTestService(&mockInstanceRepo{}, newMockCheckpointRepo(), wp, nil, nil,
		BackfillConfig{MaxConcurrent: 5, Timeout: time.Minute, GapThreshold: 60 * time.Second},
	)

	hasGap, err := svc.checkChatForGap(context.Background(), "s1", reconciliation.Checkpoint{
		SessionName:          "s1",
		ChatID:               "c1@g.us",
		LastMessageTimestamp: now.Add(-2 * time.Minute).UnixMilli(),
		LastMessageID:        "msg1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasGap {
		t.Error("expected gap when exceeding threshold")
	}
}

func TestCheckChatForGap_DifferentMessageID_Newer(t *testing.T) {
	t.Parallel()

	now := time.Now()
	wp := newMockWahaProvider()
	wp.SetMessages("s1", "c1@g.us", []whatsapp.Message{
		{ID: "msg2", ChatID: "c1@g.us", Timestamp: now, Body: "new"},
	})

	svc := newTestService(&mockInstanceRepo{}, newMockCheckpointRepo(), wp, nil, nil,
		BackfillConfig{MaxConcurrent: 5, Timeout: time.Minute, GapThreshold: 60 * time.Second},
	)

	hasGap, err := svc.checkChatForGap(context.Background(), "s1", reconciliation.Checkpoint{
		SessionName:          "s1",
		ChatID:               "c1@g.us",
		LastMessageTimestamp: now.Add(-30 * time.Second).UnixMilli(), // within threshold
		LastMessageID:        "msg1",                                 // different
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasGap {
		t.Error("expected gap when message ID differs and latest is newer")
	}
}

func TestCheckChatForGap_DifferentMessageID_Older(t *testing.T) {
	t.Parallel()

	now := time.Now()
	wp := newMockWahaProvider()
	// Latest message in WAHA is OLDER than our checkpoint
	wp.SetMessages("s1", "c1@g.us", []whatsapp.Message{
		{ID: "msg0", ChatID: "c1@g.us", Timestamp: now.Add(-2 * time.Minute), Body: "old"},
	})

	svc := newTestService(&mockInstanceRepo{}, newMockCheckpointRepo(), wp, nil, nil,
		BackfillConfig{MaxConcurrent: 5, Timeout: time.Minute, GapThreshold: 60 * time.Second},
	)

	hasGap, err := svc.checkChatForGap(context.Background(), "s1", reconciliation.Checkpoint{
		SessionName:          "s1",
		ChatID:               "c1@g.us",
		LastMessageTimestamp: now.UnixMilli(), // checkpoint is newer
		LastMessageID:        "msg1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasGap {
		t.Error("expected no gap when latest message is older than checkpoint")
	}
}

func TestCheckChatForGap_EmptyLastMessageID(t *testing.T) {
	t.Parallel()

	now := time.Now()
	wp := newMockWahaProvider()
	wp.SetMessages("s1", "c1@g.us", []whatsapp.Message{
		{ID: "msg-different", ChatID: "c1@g.us", Timestamp: now, Body: "hi"},
	})

	svc := newTestService(&mockInstanceRepo{}, newMockCheckpointRepo(), wp, nil, nil,
		BackfillConfig{MaxConcurrent: 5, Timeout: time.Minute, GapThreshold: 60 * time.Second},
	)

	hasGap, err := svc.checkChatForGap(context.Background(), "s1", reconciliation.Checkpoint{
		SessionName:          "s1",
		ChatID:               "c1@g.us",
		LastMessageTimestamp: now.Add(-10 * time.Second).UnixMilli(),
		LastMessageID:        "", // empty — should skip ID comparison
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasGap {
		t.Error("expected no gap when LastMessageID is empty and within threshold")
	}
}

// ---------------------------------------------------------------------------
// RunWithReconciliationService tests
// ---------------------------------------------------------------------------

func TestRunWithReconciliationService_NilReconciler(t *testing.T) {
	t.Parallel()

	svc := newTestService(
		&mockInstanceRepo{}, newMockCheckpointRepo(), newMockWahaProvider(),
		nil, nil, DefaultBackfillConfig(),
	)

	_, err := svc.RunWithReconciliationService(context.Background())
	if err == nil {
		t.Fatal("expected error for nil reconciler")
	}
}

func TestRunWithReconciliationService_OnStartupError_Continues(t *testing.T) {
	t.Parallel()

	rec := newMockReconciler()
	rec.onStartupErr = errors.New("startup failed")

	svc := newTestService(
		&mockInstanceRepo{instances: []models.Instance{{ID: 1, InstanceID: "s1"}}},
		newMockCheckpointRepo(), newMockWahaProvider(), nil, rec,
		DefaultBackfillConfig(),
	)

	result, err := svc.RunWithReconciliationService(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Errors) == 0 {
		t.Error("expected errors from OnStartup")
	}
	if !rec.onStartupCalled {
		t.Error("OnStartup should be called")
	}
	if result.TotalSessions != 1 {
		t.Errorf("TotalSessions = %d, want 1", result.TotalSessions)
	}
}

func TestRunWithReconciliationService_GetInstancesError(t *testing.T) {
	t.Parallel()

	svc := newTestService(
		&mockInstanceRepo{err: errors.New("db error")},
		newMockCheckpointRepo(), newMockWahaProvider(), nil, newMockReconciler(),
		DefaultBackfillConfig(),
	)

	_, err := svc.RunWithReconciliationService(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRunWithReconciliationService_NoCheckpoints_Reconciles(t *testing.T) {
	t.Parallel()

	rec := newMockReconciler()

	svc := newTestService(
		&mockInstanceRepo{instances: []models.Instance{{ID: 1, InstanceID: "s1"}}},
		newMockCheckpointRepo(), // no checkpoints
		newMockWahaProvider(), nil, rec,
		DefaultBackfillConfig(),
	)

	result, err := svc.RunWithReconciliationService(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SessionsWithGaps != 1 {
		t.Errorf("SessionsWithGaps = %d, want 1", result.SessionsWithGaps)
	}
	if result.ReconciliationsEnqueued != 1 {
		t.Errorf("ReconciliationsEnqueued = %d, want 1", result.ReconciliationsEnqueued)
	}
	if len(rec.reconciledSessions) != 1 || rec.reconciledSessions[0] != "s1" {
		t.Errorf("expected s1 reconciled, got %v", rec.reconciledSessions)
	}
}

func TestRunWithReconciliationService_ReconcileSessionError(t *testing.T) {
	t.Parallel()

	rec := newMockReconciler()
	rec.reconcileErr = errors.New("reconcile failed")

	svc := newTestService(
		&mockInstanceRepo{instances: []models.Instance{{ID: 1, InstanceID: "s1"}}},
		newMockCheckpointRepo(), newMockWahaProvider(), nil, rec,
		DefaultBackfillConfig(),
	)

	result, err := svc.RunWithReconciliationService(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Errors) == 0 {
		t.Error("expected error from ReconcileSession")
	}
	if result.SessionsChecked != 1 {
		t.Errorf("SessionsChecked = %d, want 1", result.SessionsChecked)
	}
}

func TestRunWithReconciliationService_GetBySessionError(t *testing.T) {
	t.Parallel()

	cp := newMockCheckpointRepo()
	cp.getBySessionErr = errors.New("checkpoint db error")

	svc := newTestService(
		&mockInstanceRepo{instances: []models.Instance{
			{ID: 1, InstanceID: "s1"}, {ID: 2, InstanceID: "s2"},
		}},
		cp, newMockWahaProvider(), nil, newMockReconciler(),
		DefaultBackfillConfig(),
	)

	result, err := svc.RunWithReconciliationService(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Errors) < 2 {
		t.Errorf("expected at least 2 errors, got %d", len(result.Errors))
	}
}

func TestRunWithReconciliationService_WithCheckpoints_Skips(t *testing.T) {
	t.Parallel()

	cp := newMockCheckpointRepo()
	cp.AddCheckpoint("s1", reconciliation.Checkpoint{
		SessionName:          "s1",
		ChatID:               "c1@g.us",
		LastMessageTimestamp: time.Now().UnixMilli(),
		Status:               reconciliation.CheckpointStatusIdle,
	})

	rec := newMockReconciler()

	svc := newTestService(
		&mockInstanceRepo{instances: []models.Instance{{ID: 1, InstanceID: "s1"}}},
		cp, newMockWahaProvider(), nil, rec,
		DefaultBackfillConfig(),
	)

	result, err := svc.RunWithReconciliationService(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SessionsWithGaps != 0 {
		t.Errorf("SessionsWithGaps = %d, want 0", result.SessionsWithGaps)
	}
	if len(rec.reconciledSessions) != 0 {
		t.Error("should not reconcile sessions with checkpoints")
	}
}

func TestRunWithReconciliationService_ContextCancelled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before calling

	rec := newMockReconciler()

	svc := newTestService(
		&mockInstanceRepo{instances: []models.Instance{
			{ID: 1, InstanceID: "s1"}, {ID: 2, InstanceID: "s2"},
		}},
		newMockCheckpointRepo(), newMockWahaProvider(), nil, rec,
		DefaultBackfillConfig(),
	)

	// With cancelled context the for loop's select hits ctx.Done(),
	// but break only exits the select — processing continues with mocks.
	result, err := svc.RunWithReconciliationService(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalSessions != 2 {
		t.Errorf("TotalSessions = %d, want 2", result.TotalSessions)
	}
}

func TestRunWithReconciliationService_FullFlow(t *testing.T) {
	t.Parallel()

	cp := newMockCheckpointRepo()
	// s1 has checkpoints → skipped
	cp.AddCheckpoint("s1", reconciliation.Checkpoint{
		SessionName:          "s1",
		ChatID:               "c1@g.us",
		LastMessageTimestamp: time.Now().UnixMilli(),
		Status:               reconciliation.CheckpointStatusIdle,
	})
	// s2 has no checkpoints → reconciled

	rec := newMockReconciler()

	svc := newTestService(
		&mockInstanceRepo{instances: []models.Instance{
			{ID: 1, InstanceID: "s1"}, {ID: 2, InstanceID: "s2"},
		}},
		cp, newMockWahaProvider(), nil, rec,
		DefaultBackfillConfig(),
	)

	result, err := svc.RunWithReconciliationService(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalSessions != 2 {
		t.Errorf("TotalSessions = %d, want 2", result.TotalSessions)
	}
	if result.SessionsChecked != 2 {
		t.Errorf("SessionsChecked = %d, want 2", result.SessionsChecked)
	}
	if result.SessionsWithGaps != 1 {
		t.Errorf("SessionsWithGaps = %d, want 1", result.SessionsWithGaps)
	}
	if result.ReconciliationsEnqueued != 1 {
		t.Errorf("ReconciliationsEnqueued = %d, want 1", result.ReconciliationsEnqueued)
	}
	if len(rec.reconciledSessions) != 1 || rec.reconciledSessions[0] != "s2" {
		t.Errorf("expected s2 reconciled, got %v", rec.reconciledSessions)
	}
	if result.Duration <= 0 {
		t.Error("Duration should be positive")
	}
	if len(result.Errors) != 0 {
		t.Errorf("expected 0 errors, got %d", len(result.Errors))
	}
}

// Stubs dos métodos novos do CheckpointRepository (watermark vivo + higiene).
func (m *mockCheckpointRepo) AdvanceCheckpoint(ctx context.Context, sessionName, chatID string, lastTimestamp int64, lastMessageID string) error {
	return nil
}
func (m *mockCheckpointRepo) ResetStuckCheckpoints(ctx context.Context, olderThan time.Duration) (int64, error) {
	return 0, nil
}
func (m *mockCheckpointRepo) DeleteInactiveCheckpoints(ctx context.Context, olderThan time.Duration) (int64, error) {
	return 0, nil
}
