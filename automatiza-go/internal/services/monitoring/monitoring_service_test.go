package monitoring

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/lib/cache"
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"

	"go.uber.org/zap/zaptest"
)

// ---------------------------------------------------------------------------
// Mock implementations
// ---------------------------------------------------------------------------

type mockAutomationRepo struct {
	GetMonitoringAutomationsForGroupFn func(ctx context.Context, waGroupID string) ([]models.Automation, error)
	GetUserAutomationsFn               func(ctx context.Context, userID string) ([]models.Automation, error)
	GetActiveAutomationsFn             func(ctx context.Context) ([]models.Automation, error)
	GetAutomationByIDFn                func(ctx context.Context, automationID string) (*models.Automation, error)
	GetAutomationByIDAndUserFn         func(ctx context.Context, automationID, userID string) (*models.Automation, error)
	CreateAutomationFn                 func(ctx context.Context, userID string, data *models.Automation) (*models.Automation, error)
	UpdateAutomationFn                 func(ctx context.Context, automationID string, updates map[string]interface{}) error
	DeleteAutomationFn                 func(ctx context.Context, automationID, userID string) error
	UpdateLastRunAtFn                  func(ctx context.Context, automationID string) error
	UpdateNextRunAtFn                  func(ctx context.Context, automationID string, nextRun time.Time) error
	BulkUpdateStatusByUserIDFn         func(ctx context.Context, userID, targetStatus string, sourceStatus *string) (int, error)
	BulkUpdateNextRunAtsFn             func(ctx context.Context, automationIDs []string, nextRunAts []time.Time) error
}

func (m *mockAutomationRepo) GetMonitoringAutomationsForGroup(ctx context.Context, waGroupID string) ([]models.Automation, error) {
	if m.GetMonitoringAutomationsForGroupFn != nil {
		return m.GetMonitoringAutomationsForGroupFn(ctx, waGroupID)
	}
	return nil, nil
}

func (m *mockAutomationRepo) GetUserAutomations(ctx context.Context, userID string) ([]models.Automation, error) {
	if m.GetUserAutomationsFn != nil {
		return m.GetUserAutomationsFn(ctx, userID)
	}
	return nil, nil
}

func (m *mockAutomationRepo) GetActiveAutomations(ctx context.Context) ([]models.Automation, error) {
	if m.GetActiveAutomationsFn != nil {
		return m.GetActiveAutomationsFn(ctx)
	}
	return nil, nil
}

func (m *mockAutomationRepo) GetAutomationByID(ctx context.Context, automationID string) (*models.Automation, error) {
	if m.GetAutomationByIDFn != nil {
		return m.GetAutomationByIDFn(ctx, automationID)
	}
	return nil, nil
}

func (m *mockAutomationRepo) GetAutomationByIDAndUser(ctx context.Context, automationID, userID string) (*models.Automation, error) {
	if m.GetAutomationByIDAndUserFn != nil {
		return m.GetAutomationByIDAndUserFn(ctx, automationID, userID)
	}
	return nil, nil
}

func (m *mockAutomationRepo) CreateAutomation(ctx context.Context, userID string, data *models.Automation) (*models.Automation, error) {
	if m.CreateAutomationFn != nil {
		return m.CreateAutomationFn(ctx, userID, data)
	}
	return nil, nil
}

func (m *mockAutomationRepo) UpdateAutomation(ctx context.Context, automationID string, updates map[string]interface{}) error {
	if m.UpdateAutomationFn != nil {
		return m.UpdateAutomationFn(ctx, automationID, updates)
	}
	return nil
}

func (m *mockAutomationRepo) DeleteAutomation(ctx context.Context, automationID, userID string) error {
	if m.DeleteAutomationFn != nil {
		return m.DeleteAutomationFn(ctx, automationID, userID)
	}
	return nil
}

func (m *mockAutomationRepo) UpdateLastRunAt(ctx context.Context, automationID string) error {
	if m.UpdateLastRunAtFn != nil {
		return m.UpdateLastRunAtFn(ctx, automationID)
	}
	return nil
}

func (m *mockAutomationRepo) UpdateNextRunAt(ctx context.Context, automationID string, nextRun time.Time) error {
	if m.UpdateNextRunAtFn != nil {
		return m.UpdateNextRunAtFn(ctx, automationID, nextRun)
	}
	return nil
}

func (m *mockAutomationRepo) BulkUpdateStatusByUserID(ctx context.Context, userID, targetStatus string, sourceStatus *string) (int, error) {
	if m.BulkUpdateStatusByUserIDFn != nil {
		return m.BulkUpdateStatusByUserIDFn(ctx, userID, targetStatus, sourceStatus)
	}
	return 0, nil
}

func (m *mockAutomationRepo) BulkUpdateNextRunAts(ctx context.Context, automationIDs []string, nextRunAts []time.Time) error {
	if m.BulkUpdateNextRunAtsFn != nil {
		return m.BulkUpdateNextRunAtsFn(ctx, automationIDs, nextRunAts)
	}
	return nil
}

func (m *mockAutomationRepo) GetOfferGenerateAutomationsForGroup(ctx context.Context, waGroupID string) ([]models.Automation, error) {
	return nil, nil
}

type mockInstanceRepo struct {
	GetUserInstancesFn          func(ctx context.Context, userID string) ([]models.Instance, error)
	GetInstanceByIDFn           func(ctx context.Context, instanceID int) (*models.Instance, error)
	GetInstanceByNameFn         func(ctx context.Context, instanceName string) (*models.Instance, error)
	GetInstanceByUserAndNameFn  func(ctx context.Context, userID, instanceName string) (*models.Instance, error)
	GetAllConnectedInstancesFn  func(ctx context.Context) ([]models.Instance, error)
	UpsertInstanceFn            func(ctx context.Context, userID string, data *models.Instance) (*models.Instance, error)
	UpdateInstanceStatusFn      func(ctx context.Context, instanceID int, status string) error
	DeleteInstanceFn            func(ctx context.Context, instanceID int) error
	SetDefaultInstanceFn        func(ctx context.Context, userID string, instanceID int) error
	CountUserInstancesFn        func(ctx context.Context, userID string) (int, error)
}

func (m *mockInstanceRepo) GetUserInstances(ctx context.Context, userID string) ([]models.Instance, error) {
	if m.GetUserInstancesFn != nil {
		return m.GetUserInstancesFn(ctx, userID)
	}
	return nil, nil
}

func (m *mockInstanceRepo) GetInstanceByID(ctx context.Context, instanceID int) (*models.Instance, error) {
	if m.GetInstanceByIDFn != nil {
		return m.GetInstanceByIDFn(ctx, instanceID)
	}
	return nil, nil
}

func (m *mockInstanceRepo) GetInstanceByName(ctx context.Context, instanceName string) (*models.Instance, error) {
	if m.GetInstanceByNameFn != nil {
		return m.GetInstanceByNameFn(ctx, instanceName)
	}
	return nil, nil
}

func (m *mockInstanceRepo) GetInstanceByUserAndName(ctx context.Context, userID, instanceName string) (*models.Instance, error) {
	if m.GetInstanceByUserAndNameFn != nil {
		return m.GetInstanceByUserAndNameFn(ctx, userID, instanceName)
	}
	return nil, nil
}

func (m *mockInstanceRepo) GetAllConnectedInstances(ctx context.Context) ([]models.Instance, error) {
	if m.GetAllConnectedInstancesFn != nil {
		return m.GetAllConnectedInstancesFn(ctx)
	}
	return nil, nil
}

func (m *mockInstanceRepo) UpsertInstance(ctx context.Context, userID string, data *models.Instance) (*models.Instance, error) {
	if m.UpsertInstanceFn != nil {
		return m.UpsertInstanceFn(ctx, userID, data)
	}
	return nil, nil
}

func (m *mockInstanceRepo) UpdateInstanceStatus(ctx context.Context, instanceID int, status string) error {
	if m.UpdateInstanceStatusFn != nil {
		return m.UpdateInstanceStatusFn(ctx, instanceID, status)
	}
	return nil
}

func (m *mockInstanceRepo) DeleteInstance(ctx context.Context, instanceID int) error {
	if m.DeleteInstanceFn != nil {
		return m.DeleteInstanceFn(ctx, instanceID)
	}
	return nil
}

func (m *mockInstanceRepo) SetDefaultInstance(ctx context.Context, userID string, instanceID int) error {
	if m.SetDefaultInstanceFn != nil {
		return m.SetDefaultInstanceFn(ctx, userID, instanceID)
	}
	return nil
}

func (m *mockInstanceRepo) CountUserInstances(ctx context.Context, userID string) (int, error) {
	if m.CountUserInstancesFn != nil {
		return m.CountUserInstancesFn(ctx, userID)
	}
	return 0, nil
}

type mockDispatchLogRepo struct {
	CreateFn                   func(ctx context.Context, log *models.DispatchLog) (*models.DispatchLog, error)
	UpdateFn                   func(ctx context.Context, logID int, updates map[string]interface{}) error
	GetRecentByCanonicalKeyFn  func(ctx context.Context, userID, groupID, canonicalKey string, hours int) (*models.DispatchLog, error)
	GetUserLogsFn              func(ctx context.Context, userID string, limit, offset int) ([]models.DispatchLog, error)
	DeleteRecentByAutomationFn func(ctx context.Context, automationID string, sinceDays int) (int64, error)
	GetByAutomationIDWithCountFn func(ctx context.Context, automationID, userID string, limit, offset int) ([]repository.DispatchLogWithGroup, int, error)
	GetUserLogsWithFiltersFn     func(ctx context.Context, userID string, filters *models.DispatchLogFilters, limit, offset int) ([]repository.DispatchLogWithGroup, int64, error)
	ExistsByCanonicalKeyFn       func(ctx context.Context, userID, groupID, canonicalKey string, hours int) (bool, error)
	CreateIfNotExistsFn          func(ctx context.Context, log *models.DispatchLog, dedupWindowHours int) (*models.DispatchLog, bool, error)
}

func (m *mockDispatchLogRepo) Create(ctx context.Context, log *models.DispatchLog) (*models.DispatchLog, error) {
	if m.CreateFn != nil {
		return m.CreateFn(ctx, log)
	}
	return log, nil
}

func (m *mockDispatchLogRepo) Update(ctx context.Context, logID int, updates map[string]interface{}) error {
	if m.UpdateFn != nil {
		return m.UpdateFn(ctx, logID, updates)
	}
	return nil
}

func (m *mockDispatchLogRepo) GetRecentByCanonicalKey(ctx context.Context, userID, groupID, canonicalKey string, hours int) (*models.DispatchLog, error) {
	if m.GetRecentByCanonicalKeyFn != nil {
		return m.GetRecentByCanonicalKeyFn(ctx, userID, groupID, canonicalKey, hours)
	}
	return nil, nil
}

func (m *mockDispatchLogRepo) GetUserLogs(ctx context.Context, userID string, limit, offset int) ([]models.DispatchLog, error) {
	if m.GetUserLogsFn != nil {
		return m.GetUserLogsFn(ctx, userID, limit, offset)
	}
	return nil, nil
}

func (m *mockDispatchLogRepo) DeleteRecentByAutomation(ctx context.Context, automationID string, sinceDays int) (int64, error) {
	if m.DeleteRecentByAutomationFn != nil {
		return m.DeleteRecentByAutomationFn(ctx, automationID, sinceDays)
	}
	return 0, nil
}

func (m *mockDispatchLogRepo) GetByAutomationIDWithCount(ctx context.Context, automationID, userID string, limit, offset int) ([]repository.DispatchLogWithGroup, int, error) {
	if m.GetByAutomationIDWithCountFn != nil {
		return m.GetByAutomationIDWithCountFn(ctx, automationID, userID, limit, offset)
	}
	return nil, 0, nil
}

func (m *mockDispatchLogRepo) GetUserLogsWithFilters(ctx context.Context, userID string, filters *models.DispatchLogFilters, limit, offset int) ([]repository.DispatchLogWithGroup, int64, error) {
	if m.GetUserLogsWithFiltersFn != nil {
		return m.GetUserLogsWithFiltersFn(ctx, userID, filters, limit, offset)
	}
	return nil, 0, nil
}

func (m *mockDispatchLogRepo) ExistsByCanonicalKey(ctx context.Context, userID, groupID, canonicalKey string, hours int) (bool, error) {
	if m.ExistsByCanonicalKeyFn != nil {
		return m.ExistsByCanonicalKeyFn(ctx, userID, groupID, canonicalKey, hours)
	}
	return false, nil
}

func (m *mockDispatchLogRepo) CreateIfNotExists(ctx context.Context, log *models.DispatchLog, dedupWindowHours int) (*models.DispatchLog, bool, error) {
	if m.CreateIfNotExistsFn != nil {
		return m.CreateIfNotExistsFn(ctx, log, dedupWindowHours)
	}
	return log, true, nil
}

func (m *mockDispatchLogRepo) DeleteOldLogsByStatus(ctx context.Context, status string, olderThan time.Duration) (int64, error) {
	return 0, nil
}

type mockShopeeService struct {
	GenerateAffiliateLinkFn func(ctx context.Context, productURL, userID string) (*models.ShopeeAffiliateLink, error)
	ExtractProductIDsFn     func(productURL string) (shopID, itemID int64, err error)
}

func (m *mockShopeeService) SearchOffers(_ context.Context, _ string, _ models.ShopeeSearchParams) (*models.ShopeeSearchResult, error) {
	return nil, nil
}
func (m *mockShopeeService) GenerateAffiliateLink(ctx context.Context, productURL, userID string) (*models.ShopeeAffiliateLink, error) {
	if m.GenerateAffiliateLinkFn != nil {
		return m.GenerateAffiliateLinkFn(ctx, productURL, userID)
	}
	return nil, fmt.Errorf("not implemented")
}
func (m *mockShopeeService) GetCategories(_ context.Context, _ *int64) ([]models.ShopeeCategory, error) {
	return nil, nil
}
func (m *mockShopeeService) GetProduct(_ context.Context, _, _ int64, _ string) (*models.ShopeeOffer, error) {
	return nil, nil
}
func (m *mockShopeeService) ExpandURL(_ context.Context, _ string) (string, error) {
	return "", nil
}
func (m *mockShopeeService) ExtractProductIDs(productURL string) (int64, int64, error) {
	if m.ExtractProductIDsFn != nil {
		return m.ExtractProductIDsFn(productURL)
	}
	return 0, 0, fmt.Errorf("not implemented")
}

type mockMLService struct {
	CreateAffiliateLinkFn    func(ctx context.Context, originURL, userID string) (*models.MLAffiliateLink, error)
	GetProductPreviewFn      func(ctx context.Context, originURL, userID string) (*models.MLProductPreview, error)
	LoadCredentialsFn        func(ctx context.Context, userID string) (*models.MLCredentials, error)
	SaveCredentialsFn        func(ctx context.Context, userID, cookie, csrf, tag string) error
	IsConnectedFn            func(ctx context.Context, userID string) (bool, error)
	ExtractProductIDFn       func(productURL string) (string, error)
	InvalidatePreviewCacheFn func(ctx context.Context, originURL string) error
}

func (m *mockMLService) CreateAffiliateLink(ctx context.Context, originURL, userID string) (*models.MLAffiliateLink, error) {
	if m.CreateAffiliateLinkFn != nil {
		return m.CreateAffiliateLinkFn(ctx, originURL, userID)
	}
	return nil, fmt.Errorf("not implemented")
}

func (m *mockMLService) GetProductPreview(ctx context.Context, originURL, userID string) (*models.MLProductPreview, error) {
	if m.GetProductPreviewFn != nil {
		return m.GetProductPreviewFn(ctx, originURL, userID)
	}
	return nil, nil
}

func (m *mockMLService) LoadCredentials(ctx context.Context, userID string) (*models.MLCredentials, error) {
	if m.LoadCredentialsFn != nil {
		return m.LoadCredentialsFn(ctx, userID)
	}
	return nil, nil
}

func (m *mockMLService) SaveCredentials(ctx context.Context, userID, cookie, csrf, tag string) error {
	if m.SaveCredentialsFn != nil {
		return m.SaveCredentialsFn(ctx, userID, cookie, csrf, tag)
	}
	return nil
}

func (m *mockMLService) IsConnected(ctx context.Context, userID string) (bool, error) {
	if m.IsConnectedFn != nil {
		return m.IsConnectedFn(ctx, userID)
	}
	return false, nil
}

func (m *mockMLService) ExtractProductID(productURL string) (string, error) {
	if m.ExtractProductIDFn != nil {
		return m.ExtractProductIDFn(productURL)
	}
	return "", fmt.Errorf("not implemented")
}

func (m *mockMLService) InvalidatePreviewCache(ctx context.Context, originURL string) error {
	if m.InvalidatePreviewCacheFn != nil {
		return m.InvalidatePreviewCacheFn(ctx, originURL)
	}
	return nil
}

type mockWAProvider struct {
	SendTextFn           func(ctx context.Context, input whatsapp.SendTextInput) (*whatsapp.MessageResult, error)
	SendImageFn          func(ctx context.Context, input whatsapp.SendImageInput) (*whatsapp.MessageResult, error)
	DownloadMediaByURLFn func(ctx context.Context, mediaURL string) (*whatsapp.DownloadMediaResult, error)
	RedownloadMediaFn    func(ctx context.Context, sessionName, chatID, messageID string) (*whatsapp.DownloadMediaResult, error)
}

// SessionManager methods
func (m *mockWAProvider) CreateSession(_ context.Context, _ whatsapp.CreateSessionInput) (*whatsapp.SessionInfo, error) {
	return nil, nil
}
func (m *mockWAProvider) DeleteSession(_ context.Context, _ string) error { return nil }
func (m *mockWAProvider) GetSessionStatus(_ context.Context, _ string) (*whatsapp.SessionInfo, error) {
	return nil, nil
}
func (m *mockWAProvider) GetQRCode(_ context.Context, _ string) (*whatsapp.QRCodeData, error) {
	return nil, nil
}
func (m *mockWAProvider) RestartSession(_ context.Context, _ string) error { return nil }
func (m *mockWAProvider) LogoutSession(_ context.Context, _ string) error  { return nil }
func (m *mockWAProvider) ClearStatusCache(_ string)                        {}

// GroupManager methods
func (m *mockWAProvider) GetGroups(_ context.Context, _ string) ([]whatsapp.GroupInfo, error) {
	return nil, nil
}
func (m *mockWAProvider) GetGroupParticipants(_ context.Context, _, _ string) ([]whatsapp.ParticipantInfo, error) {
	return nil, nil
}
func (m *mockWAProvider) GetGroupInviteCode(_ context.Context, _, _ string) (string, error) {
	return "", nil
}
func (m *mockWAProvider) UpdateGroupSubject(_ context.Context, _ whatsapp.UpdateGroupSubjectInput) error {
	return nil
}
func (m *mockWAProvider) UpdateGroupDescription(_ context.Context, _ whatsapp.UpdateGroupDescriptionInput) error {
	return nil
}
func (m *mockWAProvider) UpdateGroupPicture(_ context.Context, _ whatsapp.UpdateGroupPictureInput) error {
	return nil
}
func (m *mockWAProvider) SetGroupAnnouncement(_ context.Context, _ whatsapp.SetGroupAnnouncementInput) error {
	return nil
}
func (m *mockWAProvider) AddGroupMembers(_ context.Context, _ whatsapp.AddGroupMembersInput) (*whatsapp.MemberOperationResult, error) {
	return nil, nil
}
func (m *mockWAProvider) RemoveGroupMembers(_ context.Context, _ whatsapp.RemoveGroupMembersInput) (*whatsapp.MemberOperationResult, error) {
	return nil, nil
}
func (m *mockWAProvider) LeaveGroup(_ context.Context, _ whatsapp.LeaveGroupInput) error { return nil }
func (m *mockWAProvider) PromoteMember(_ context.Context, _ whatsapp.PromoteMemberInput) error {
	return nil
}
func (m *mockWAProvider) DemoteMember(_ context.Context, _ whatsapp.DemoteMemberInput) error {
	return nil
}

// MessageSender methods
func (m *mockWAProvider) SendText(ctx context.Context, input whatsapp.SendTextInput) (*whatsapp.MessageResult, error) {
	if m.SendTextFn != nil {
		return m.SendTextFn(ctx, input)
	}
	return &whatsapp.MessageResult{ID: "msg-1"}, nil
}

func (m *mockWAProvider) SendImage(ctx context.Context, input whatsapp.SendImageInput) (*whatsapp.MessageResult, error) {
	if m.SendImageFn != nil {
		return m.SendImageFn(ctx, input)
	}
	return &whatsapp.MessageResult{ID: "msg-img-1"}, nil
}

func (m *mockWAProvider) SendVideo(_ context.Context, _ whatsapp.SendVideoInput) (*whatsapp.MessageResult, error) {
	return nil, nil
}
func (m *mockWAProvider) SendDocument(_ context.Context, _ whatsapp.SendDocumentInput) (*whatsapp.MessageResult, error) {
	return nil, nil
}
func (m *mockWAProvider) SendVoice(_ context.Context, _ whatsapp.SendVoiceInput) (*whatsapp.MessageResult, error) {
	return nil, nil
}
func (m *mockWAProvider) SendPoll(_ context.Context, _ whatsapp.SendPollInput) (*whatsapp.MessageResult, error) {
	return nil, nil
}
func (m *mockWAProvider) SendLinkCustomPreview(_ context.Context, _ whatsapp.SendLinkPreviewInput) (*whatsapp.MessageResult, error) {
	return nil, nil
}

// WebhookManager methods
func (m *mockWAProvider) SetWebhook(_ context.Context, _ whatsapp.SetWebhookInput) error { return nil }

// MessageReconciler methods
func (m *mockWAProvider) GetMessages(_ context.Context, _ whatsapp.GetMessagesInput) (*whatsapp.MessagesResult, error) {
	return nil, nil
}

// MediaDownloader methods
func (m *mockWAProvider) DownloadMedia(_ context.Context, _ whatsapp.DownloadMediaInput) (*whatsapp.DownloadMediaResult, error) {
	return nil, nil
}

func (m *mockWAProvider) DownloadMediaByURL(ctx context.Context, mediaURL string) (*whatsapp.DownloadMediaResult, error) {
	if m.DownloadMediaByURLFn != nil {
		return m.DownloadMediaByURLFn(ctx, mediaURL)
	}
	return nil, fmt.Errorf("not implemented")
}

func (m *mockWAProvider) RedownloadMedia(ctx context.Context, sessionName, chatID, messageID string) (*whatsapp.DownloadMediaResult, error) {
	if m.RedownloadMediaFn != nil {
		return m.RedownloadMediaFn(ctx, sessionName, chatID, messageID)
	}
	return nil, fmt.Errorf("not implemented")
}

// Verify interfaces.
var (
	_ repository.AutomationRepository  = (*mockAutomationRepo)(nil)
	_ repository.InstanceRepository    = (*mockInstanceRepo)(nil)
	_ repository.DispatchLogRepository = (*mockDispatchLogRepo)(nil)
	_ whatsapp.WhatsAppProvider        = (*mockWAProvider)(nil)
)

// ---------------------------------------------------------------------------
// Helper to create a test service
// ---------------------------------------------------------------------------

// disabledCache returns a non-nil cache with disabled=true (all operations are no-ops).
// This avoids nil pointer panics in production code that calls s.cache.Set without nil check.
func disabledCache() *cache.Cache {
	return cache.NewCache(nil, "test", true)
}

// connectedInstanceRepo returns a mockInstanceRepo that always returns a connected instance.
func connectedInstanceRepo() *mockInstanceRepo {
	return &mockInstanceRepo{
		GetUserInstancesFn: func(_ context.Context, _ string) ([]models.Instance, error) {
			return []models.Instance{{InstanceID: "test-session", Status: "connected"}}, nil
		},
	}
}

func newTestService(t *testing.T, opts ...func(*monitoringService)) *monitoringService {
	t.Helper()
	svc := &monitoringService{
		automationRepo:  &mockAutomationRepo{},
		instanceRepo:    connectedInstanceRepo(),
		dispatchLogRepo: &mockDispatchLogRepo{},
		shopeeService:   &mockShopeeService{},
		mlService:       &mockMLService{},
		waProvider:      &mockWAProvider{},
		cache:           disabledCache(),
		logger:          zaptest.NewLogger(t),
	}
	for _, opt := range opts {
		opt(svc)
	}
	return svc
}

// ---------------------------------------------------------------------------
// ProcessIncomingMessage
// ---------------------------------------------------------------------------

func TestProcessIncomingMessage(t *testing.T) {
	t.Run("non-group message skipped", func(t *testing.T) {
		svc := newTestService(t)
		err := svc.ProcessIncomingMessage(context.Background(), &models.IncomingMessage{
			RemoteJID: "5511999999999@s.whatsapp.net", // not a group
			Body:      "https://shopee.com.br/product",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("own message (FromMe) skipped", func(t *testing.T) {
		svc := newTestService(t)
		err := svc.ProcessIncomingMessage(context.Background(), &models.IncomingMessage{
			RemoteJID: "123456@g.us",
			FromMe:    true,
			Body:      "https://shopee.com.br/product",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("no links in message skipped", func(t *testing.T) {
		svc := newTestService(t)
		err := svc.ProcessIncomingMessage(context.Background(), &models.IncomingMessage{
			RemoteJID: "123456@g.us",
			Body:      "Hello world, no links here!",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("no matching automations returns nil", func(t *testing.T) {
		svc := newTestService(t, func(s *monitoringService) {
			s.automationRepo = &mockAutomationRepo{
				GetMonitoringAutomationsForGroupFn: func(_ context.Context, _ string) ([]models.Automation, error) {
					return nil, nil
				},
			}
		})

		err := svc.ProcessIncomingMessage(context.Background(), &models.IncomingMessage{
			InstanceName: "instance1",
			RemoteJID:    "123456@g.us",
			MessageID:    "msg-001",
			Body:         "Check https://shopee.com.br/product123",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("with cache nil dedup returns false (fail-open)", func(t *testing.T) {
		svc := newTestService(t, func(s *monitoringService) {
			s.automationRepo = &mockAutomationRepo{
				GetMonitoringAutomationsForGroupFn: func(_ context.Context, _ string) ([]models.Automation, error) {
					return nil, nil // no automations
				},
			}
		})

		// With nil cache, dedup checks return false (fail-open)
		err := svc.ProcessIncomingMessage(context.Background(), &models.IncomingMessage{
			InstanceName: "instance1",
			RemoteJID:    "123456@g.us",
			MessageID:    "msg-002",
			Body:         "https://shopee.com.br/product456",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("repo error propagated", func(t *testing.T) {
		svc := newTestService(t, func(s *monitoringService) {
			s.automationRepo = &mockAutomationRepo{
				GetMonitoringAutomationsForGroupFn: func(_ context.Context, _ string) ([]models.Automation, error) {
					return nil, fmt.Errorf("db connection error")
				},
			}
		})

		err := svc.ProcessIncomingMessage(context.Background(), &models.IncomingMessage{
			InstanceName: "instance1",
			RemoteJID:    "123456@g.us",
			MessageID:    "msg-003",
			Body:         "https://shopee.com.br/product789",
		})
		if err == nil {
			t.Fatal("expected error from repo")
		}
	})
}

// ---------------------------------------------------------------------------
// extractLinks
// ---------------------------------------------------------------------------

func TestExtractLinks(t *testing.T) {
	svc := newTestService(t)

	tests := []struct {
		name  string
		text  string
		count int
		first string
	}{
		{
			name:  "single HTTPS link",
			text:  "check https://example.com here",
			count: 1,
			first: "https://example.com",
		},
		{
			name:  "single HTTP link",
			text:  "check http://example.com here",
			count: 1,
			first: "http://example.com",
		},
		{
			name:  "trailing punctuation trimmed",
			text:  "see https://example.com/path.",
			count: 1,
			first: "https://example.com/path",
		},
		{
			name:  "multiple links",
			text:  "https://a.com and https://b.com",
			count: 2,
			first: "https://a.com",
		},
		{
			name:  "no links",
			text:  "no links in this message",
			count: 0,
		},
		{
			name:  "link with path and query",
			text:  "https://example.com/path?q=1&x=2 ok",
			count: 1,
			first: "https://example.com/path?q=1&x=2",
		},
		{
			name:  "trailing comma removed",
			text:  "https://example.com/path, then",
			count: 1,
			first: "https://example.com/path",
		},
		{
			name:  "trailing parenthesis removed",
			text:  "(https://example.com/path)",
			count: 1,
			first: "https://example.com/path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			links := svc.extractLinks(tt.text)
			if len(links) != tt.count {
				t.Errorf("extractLinks() returned %d links, want %d: %v", len(links), tt.count, links)
			}
			if tt.count > 0 && links[0] != tt.first {
				t.Errorf("first link = %q, want %q", links[0], tt.first)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// categorizeLinks
// ---------------------------------------------------------------------------

func TestCategorizeLinks(t *testing.T) {
	svc := newTestService(t)

	tests := []struct {
		name        string
		links       []string
		wantShopee  int
		wantML      int
	}{
		{
			name:       "shopee.com.br link",
			links:      []string{"https://shopee.com.br/product/123"},
			wantShopee: 1,
			wantML:     0,
		},
		{
			name:       "mercadolivre.com.br link",
			links:      []string{"https://www.mercadolivre.com.br/product/p/MLB123"},
			wantShopee: 0,
			wantML:     1,
		},
		{
			name:       "shope.ee short link",
			links:      []string{"https://shope.ee/abc123"},
			wantShopee: 1,
			wantML:     0,
		},
		{
			name:       "meli.co short link",
			links:      []string{"https://meli.co/short123"},
			wantShopee: 0,
			wantML:     1,
		},
		{
			name:       "other URL goes to neither",
			links:      []string{"https://google.com/search"},
			wantShopee: 0,
			wantML:     0,
		},
		{
			name:       "mixed links",
			links:      []string{"https://shopee.com.br/p1", "https://www.mercadolivre.com.br/p2", "https://google.com"},
			wantShopee: 1,
			wantML:     1,
		},
		{
			name:       "empty links",
			links:      []string{},
			wantShopee: 0,
			wantML:     0,
		},
		{
			name:       "mercadolibre.com link (Spanish version)",
			links:      []string{"https://www.mercadolibre.com/product"},
			wantShopee: 0,
			wantML:     1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shopee, ml := svc.categorizeLinks(tt.links)
			if len(shopee) != tt.wantShopee {
				t.Errorf("shopeeLinks count = %d, want %d", len(shopee), tt.wantShopee)
			}
			if len(ml) != tt.wantML {
				t.Errorf("mlLinks count = %d, want %d", len(ml), tt.wantML)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// buildContentKey
// ---------------------------------------------------------------------------

func TestBuildContentKey(t *testing.T) {
	svc := newTestService(t)

	t.Run("deterministic hash", func(t *testing.T) {
		key1 := svc.buildContentKey("inst1", "group1", "hello")
		key2 := svc.buildContentKey("inst1", "group1", "hello")
		if key1 != key2 {
			t.Errorf("expected deterministic keys, got %q and %q", key1, key2)
		}
	})

	t.Run("different inputs produce different keys", func(t *testing.T) {
		key1 := svc.buildContentKey("inst1", "group1", "hello")
		key2 := svc.buildContentKey("inst2", "group1", "hello")
		if key1 == key2 {
			t.Error("expected different keys for different instances")
		}
	})

	t.Run("prefix is content:", func(t *testing.T) {
		key := svc.buildContentKey("a", "b", "c")
		hash := sha256.Sum256([]byte("a:b:c"))
		expected := "content:" + hex.EncodeToString(hash[:])
		if key != expected {
			t.Errorf("key = %q, want %q", key, expected)
		}
	})
}

// ---------------------------------------------------------------------------
// isDuplicateContent
// ---------------------------------------------------------------------------

func TestIsDuplicateContent(t *testing.T) {
	t.Run("nil cache returns false (fail-open)", func(t *testing.T) {
		svc := newTestService(t, func(s *monitoringService) {
			s.cache = nil
		})
		result := svc.isDuplicateContent(context.Background(), "some-key")
		if result {
			t.Error("expected false for nil cache")
		}
	})

	t.Run("disabled cache returns false", func(t *testing.T) {
		svc := newTestService(t) // uses disabledCache
		result := svc.isDuplicateContent(context.Background(), "some-key")
		if result {
			t.Error("expected false for disabled cache")
		}
	})
}

// ---------------------------------------------------------------------------
// extractCanonicalKey
// ---------------------------------------------------------------------------

func TestExtractCanonicalKey(t *testing.T) {
	t.Run("shopee link with extractable IDs", func(t *testing.T) {
		svc := newTestService(t, func(s *monitoringService) {
			s.shopeeService = &mockShopeeService{
				ExtractProductIDsFn: func(_ string) (int64, int64, error) {
					return 12345, 67890, nil
				},
			}
		})

		key := svc.extractCanonicalKey([]string{"https://shopee.com.br/product-i.12345.67890"})
		if key != "shopee:12345:67890" {
			t.Errorf("key = %q, want 'shopee:12345:67890'", key)
		}
	})

	t.Run("ML link with extractable ID", func(t *testing.T) {
		svc := newTestService(t, func(s *monitoringService) {
			s.mlService = &mockMLService{
				ExtractProductIDFn: func(_ string) (string, error) {
					return "MLB999888", nil
				},
			}
		})

		key := svc.extractCanonicalKey([]string{"https://www.mercadolivre.com.br/p/MLB999888"})
		if key != "ml:MLB999888" {
			t.Errorf("key = %q, want 'ml:MLB999888'", key)
		}
	})

	t.Run("unknown link falls back to hash", func(t *testing.T) {
		svc := newTestService(t)
		key := svc.extractCanonicalKey([]string{"https://unknown.com/product"})
		if key == "" {
			t.Error("expected non-empty key for unknown link")
		}
		if len(key) < 5 || key[:5] != "link:" {
			t.Errorf("expected 'link:' prefix, got %q", key)
		}
	})

	t.Run("empty links returns empty", func(t *testing.T) {
		svc := newTestService(t)
		key := svc.extractCanonicalKey([]string{})
		if key != "" {
			t.Errorf("expected empty key, got %q", key)
		}
	})

	t.Run("nil shopeeService falls back to hash", func(t *testing.T) {
		svc := newTestService(t, func(s *monitoringService) {
			s.shopeeService = nil
		})

		key := svc.extractCanonicalKey([]string{"https://shopee.com.br/product"})
		if key == "" {
			t.Error("expected non-empty key")
		}
		if len(key) < 5 || key[:5] != "link:" {
			t.Errorf("expected 'link:' prefix fallback, got %q", key)
		}
	})

	t.Run("nil mlService falls back to hash", func(t *testing.T) {
		svc := newTestService(t, func(s *monitoringService) {
			s.mlService = nil
		})

		key := svc.extractCanonicalKey([]string{"https://www.mercadolivre.com.br/p/MLB111"})
		if key == "" {
			t.Error("expected non-empty key")
		}
		if len(key) < 5 || key[:5] != "link:" {
			t.Errorf("expected 'link:' prefix fallback, got %q", key)
		}
	})
}

// ---------------------------------------------------------------------------
// claimProductSlot (replaces isProductDuplicate + markProductSent)
// ---------------------------------------------------------------------------

func TestClaimProductSlot(t *testing.T) {
	t.Run("nil cache allows claim", func(t *testing.T) {
		svc := newTestService(t, func(s *monitoringService) {
			s.cache = nil
		})
		result := svc.claimProductSlot(context.Background(), "user1", "group1", "key1", 24)
		if !result {
			t.Error("expected claim to succeed with nil cache")
		}
	})

	t.Run("empty canonicalKey allows claim", func(t *testing.T) {
		svc := newTestService(t)
		result := svc.claimProductSlot(context.Background(), "user1", "group1", "", 24)
		if !result {
			t.Error("expected claim to succeed with empty canonical key")
		}
	})

	t.Run("disabled cache allows claim", func(t *testing.T) {
		svc := newTestService(t) // uses disabledCache
		result := svc.claimProductSlot(context.Background(), "user1", "group1", "key1", 24)
		if !result {
			t.Error("expected claim to succeed with disabled cache")
		}
	})
}

// ---------------------------------------------------------------------------
// detectStore
// ---------------------------------------------------------------------------

func TestDetectStore(t *testing.T) {
	svc := newTestService(t)

	t.Run("shopee link", func(t *testing.T) {
		store := svc.detectStore([]string{"https://s.shopee.com.br/abc123"})
		if store != "shopee" {
			t.Errorf("expected 'shopee', got %q", store)
		}
	})

	t.Run("mercadolivre link", func(t *testing.T) {
		store := svc.detectStore([]string{"https://mercadolivre.com.br/sec/abc"})
		if store != "mercadolivre" {
			t.Errorf("expected 'mercadolivre', got %q", store)
		}
	})

	t.Run("unknown link", func(t *testing.T) {
		store := svc.detectStore([]string{"https://example.com/product"})
		if store != "" {
			t.Errorf("expected empty string, got %q", store)
		}
	})

	t.Run("empty links", func(t *testing.T) {
		store := svc.detectStore(nil)
		if store != "" {
			t.Errorf("expected empty string, got %q", store)
		}
	})
}

// ---------------------------------------------------------------------------
// truncateStr
// ---------------------------------------------------------------------------

func TestTruncateStr(t *testing.T) {
	tests := []struct {
		name string
		s    string
		max  int
		want string
	}{
		{
			name: "short string unchanged",
			s:    "hello",
			max:  10,
			want: "hello",
		},
		{
			name: "exact length unchanged",
			s:    "hello",
			max:  5,
			want: "hello",
		},
		{
			name: "long string truncated with ellipsis",
			s:    "hello world this is long",
			max:  5,
			want: "hello...",
		},
		{
			name: "empty string",
			s:    "",
			max:  5,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateStr(tt.s, tt.max)
			if got != tt.want {
				t.Errorf("truncateStr(%q, %d) = %q, want %q", tt.s, tt.max, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// findMatchingAutomations
// ---------------------------------------------------------------------------

func TestFindMatchingAutomations(t *testing.T) {
	t.Run("repo error propagated", func(t *testing.T) {
		svc := newTestService(t, func(s *monitoringService) {
			s.automationRepo = &mockAutomationRepo{
				GetMonitoringAutomationsForGroupFn: func(_ context.Context, _ string) ([]models.Automation, error) {
					return nil, fmt.Errorf("db error")
				},
			}
		})

		_, err := svc.findMatchingAutomations(context.Background(), "inst1", "group1@g.us")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("filters inactive automations", func(t *testing.T) {
		svc := newTestService(t, func(s *monitoringService) {
			s.automationRepo = &mockAutomationRepo{
				GetMonitoringAutomationsForGroupFn: func(_ context.Context, _ string) ([]models.Automation, error) {
					return []models.Automation{
						{ID: "a1", Status: "active"},
						{ID: "a2", Status: "inactive"},
						{ID: "a3", Status: "active"},
						{ID: "a4", Status: "paused"},
					}, nil
				},
			}
		})

		result, err := svc.findMatchingAutomations(context.Background(), "inst1", "group1@g.us")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result) != 2 {
			t.Errorf("expected 2 active automations, got %d", len(result))
		}
		for _, a := range result {
			if a.Status != "active" {
				t.Errorf("expected only active automations, got status=%q", a.Status)
			}
		}
	})

	t.Run("empty result", func(t *testing.T) {
		svc := newTestService(t, func(s *monitoringService) {
			s.automationRepo = &mockAutomationRepo{
				GetMonitoringAutomationsForGroupFn: func(_ context.Context, _ string) ([]models.Automation, error) {
					return nil, nil
				},
			}
		})

		result, err := svc.findMatchingAutomations(context.Background(), "inst1", "group1@g.us")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result) != 0 {
			t.Errorf("expected 0 automations, got %d", len(result))
		}
	})
}

// ---------------------------------------------------------------------------
// logDispatch
// ---------------------------------------------------------------------------

func TestLogDispatch(t *testing.T) {
	t.Run("nil dispatchLogRepo is no-op", func(t *testing.T) {
		svc := newTestService(t, func(s *monitoringService) {
			s.dispatchLogRepo = nil
		})

		// Should not panic
		svc.logDispatch(context.Background(), &models.Automation{ID: "a1", UserID: "u1", Name: "test"}, "g1", "url", "sent", "", "key1", "shopee")
	})
}

// ---------------------------------------------------------------------------
// NewMonitoringService
// ---------------------------------------------------------------------------

func TestNewMonitoringService(t *testing.T) {
	logger := zaptest.NewLogger(t)
	svc := NewMonitoringService(
		&mockAutomationRepo{},
		&mockInstanceRepo{},
		nil, // groupRepo: per-group routing disabled (tests don't exercise multi-instance path)
		&mockDispatchLogRepo{},
		&mockShopeeService{},
		&mockMLService{},
		&mockWAProvider{},
		nil, // no cache
		logger,
	)
	if svc == nil {
		t.Fatal("NewMonitoringService returned nil")
	}
}

// ---------------------------------------------------------------------------
// markContentProcessed with nil/disabled cache (no panic)
// ---------------------------------------------------------------------------

func TestMarkContentProcessed_NilCache(t *testing.T) {
	svc := newTestService(t, func(s *monitoringService) {
		s.cache = nil
	})
	// Should not panic with nil cache
	svc.markContentProcessed(context.Background(), "some-key")
}

func TestMarkContentProcessed_DisabledCache(t *testing.T) {
	svc := newTestService(t)
	// Should not panic with disabled cache
	svc.markContentProcessed(context.Background(), "some-key")
}

// ---------------------------------------------------------------------------
// processAutomation - key branches
// ---------------------------------------------------------------------------

func TestProcessAutomation_NoDestinationGroups(t *testing.T) {
	svc := newTestService(t)

	auto := &models.Automation{
		ID:         "auto-1",
		UserID:     "user-1",
		Name:       "test-auto",
		SendGroups: []string{},
		GroupIDs:   []string{},
		Status:     "active",
	}

	err := svc.processAutomation(context.Background(), auto, &models.IncomingMessage{
		InstanceName: "inst1",
		RemoteJID:    "source@g.us",
		Body:         "https://shopee.com.br/product",
	}, []string{"https://shopee.com.br/product"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestProcessAutomation_SameGroupSkipped(t *testing.T) {
	var sentGroups []string
	svc := newTestService(t, func(s *monitoringService) {
		s.waProvider = &mockWAProvider{
			SendTextFn: func(_ context.Context, input whatsapp.SendTextInput) (*whatsapp.MessageResult, error) {
				sentGroups = append(sentGroups, input.ChatID)
				return &whatsapp.MessageResult{ID: "msg-1"}, nil
			},
		}
	})

	auto := &models.Automation{
		ID:         "auto-1",
		UserID:     "user-1",
		Name:       "test-auto",
		SendGroups: []string{"source@g.us", "dest@g.us"},
		Status:     "active",
	}

	delayZero := 0
	auto.DelaySeconds = &delayZero

	err := svc.processAutomation(context.Background(), auto, &models.IncomingMessage{
		InstanceName: "inst1",
		RemoteJID:    "source@g.us",
		Body:         "https://shopee.com.br/product",
	}, []string{"https://shopee.com.br/product"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should only send to dest@g.us, not source@g.us
	for _, g := range sentGroups {
		if g == "source@g.us" {
			t.Error("should not send to source group")
		}
	}
}

// ---------------------------------------------------------------------------
// processAutomation - store filtering
// ---------------------------------------------------------------------------

func TestProcessAutomation_StoreFiltering(t *testing.T) {
	t.Run("stores=[shopee] + shopee link → forwards", func(t *testing.T) {
		var sent bool
		svc := newTestService(t, func(s *monitoringService) {
			s.waProvider = &mockWAProvider{
				SendTextFn: func(_ context.Context, _ whatsapp.SendTextInput) (*whatsapp.MessageResult, error) {
					sent = true
					return &whatsapp.MessageResult{ID: "msg-1"}, nil
				},
			}
		})

		delayZero := 0
		auto := &models.Automation{
			ID:           "auto-1",
			UserID:       "user-1",
			Name:         "test",
			Stores:       []string{"shopee"},
			SendGroups:   []string{"dest@g.us"},
			Status:       "active",
			DelaySeconds: &delayZero,
		}

		err := svc.processAutomation(context.Background(), auto, &models.IncomingMessage{
			InstanceName: "inst1",
			RemoteJID:    "source@g.us",
			Body:         "https://shopee.com.br/product-i.123.456",
		}, []string{"https://shopee.com.br/product-i.123.456"})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !sent {
			t.Error("expected message to be forwarded for matching store")
		}
	})

	t.Run("stores=[shopee] + ML link → skips", func(t *testing.T) {
		var sent bool
		svc := newTestService(t, func(s *monitoringService) {
			s.waProvider = &mockWAProvider{
				SendTextFn: func(_ context.Context, _ whatsapp.SendTextInput) (*whatsapp.MessageResult, error) {
					sent = true
					return &whatsapp.MessageResult{ID: "msg-1"}, nil
				},
			}
		})

		auto := &models.Automation{
			ID:         "auto-1",
			UserID:     "user-1",
			Name:       "test",
			Stores:     []string{"shopee"},
			SendGroups: []string{"dest@g.us"},
			Status:     "active",
		}

		err := svc.processAutomation(context.Background(), auto, &models.IncomingMessage{
			InstanceName: "inst1",
			RemoteJID:    "source@g.us",
			Body:         "https://www.mercadolivre.com.br/product/MLB123",
		}, []string{"https://www.mercadolivre.com.br/product/MLB123"})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if sent {
			t.Error("should NOT forward ML link when stores=[shopee]")
		}
	})

	t.Run("stores=[shopee] + generic link → skips", func(t *testing.T) {
		var sent bool
		svc := newTestService(t, func(s *monitoringService) {
			s.waProvider = &mockWAProvider{
				SendTextFn: func(_ context.Context, _ whatsapp.SendTextInput) (*whatsapp.MessageResult, error) {
					sent = true
					return &whatsapp.MessageResult{ID: "msg-1"}, nil
				},
			}
		})

		auto := &models.Automation{
			ID:         "auto-1",
			UserID:     "user-1",
			Name:       "test",
			Stores:     []string{"shopee"},
			SendGroups: []string{"dest@g.us"},
			Status:     "active",
		}

		err := svc.processAutomation(context.Background(), auto, &models.IncomingMessage{
			InstanceName: "inst1",
			RemoteJID:    "source@g.us",
			Body:         "https://amazon.com.br/product/123",
		}, []string{"https://amazon.com.br/product/123"})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if sent {
			t.Error("should NOT forward generic link when stores=[shopee]")
		}
	})

	t.Run("stores=[] (empty) defaults to shopee → forwards shopee", func(t *testing.T) {
		var sent bool
		svc := newTestService(t, func(s *monitoringService) {
			s.waProvider = &mockWAProvider{
				SendTextFn: func(_ context.Context, _ whatsapp.SendTextInput) (*whatsapp.MessageResult, error) {
					sent = true
					return &whatsapp.MessageResult{ID: "msg-1"}, nil
				},
			}
		})

		delayZero := 0
		auto := &models.Automation{
			ID:           "auto-1",
			UserID:       "user-1",
			Name:         "test",
			Stores:       []string{},
			SendGroups:   []string{"dest@g.us"},
			Status:       "active",
			DelaySeconds: &delayZero,
		}

		err := svc.processAutomation(context.Background(), auto, &models.IncomingMessage{
			InstanceName: "inst1",
			RemoteJID:    "source@g.us",
			Body:         "https://shopee.com.br/product-i.123.456",
		}, []string{"https://shopee.com.br/product-i.123.456"})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !sent {
			t.Error("expected default stores to include shopee")
		}
	})

	t.Run("stores=nil defaults to shopee → skips ML", func(t *testing.T) {
		var sent bool
		svc := newTestService(t, func(s *monitoringService) {
			s.waProvider = &mockWAProvider{
				SendTextFn: func(_ context.Context, _ whatsapp.SendTextInput) (*whatsapp.MessageResult, error) {
					sent = true
					return &whatsapp.MessageResult{ID: "msg-1"}, nil
				},
			}
		})

		auto := &models.Automation{
			ID:         "auto-1",
			UserID:     "user-1",
			Name:       "test",
			Stores:     nil,
			SendGroups: []string{"dest@g.us"},
			Status:     "active",
		}

		err := svc.processAutomation(context.Background(), auto, &models.IncomingMessage{
			InstanceName: "inst1",
			RemoteJID:    "source@g.us",
			Body:         "https://www.mercadolivre.com.br/product/MLB123",
		}, []string{"https://www.mercadolivre.com.br/product/MLB123"})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if sent {
			t.Error("stores=nil should default to [shopee] and skip ML links")
		}
	})
}

// ---------------------------------------------------------------------------
// processAutomation - session resolution
// ---------------------------------------------------------------------------

func TestProcessAutomation_UsesOwnerSession(t *testing.T) {
	var usedSession string
	svc := newTestService(t, func(s *monitoringService) {
		s.instanceRepo = &mockInstanceRepo{
			GetUserInstancesFn: func(_ context.Context, userID string) ([]models.Instance, error) {
				if userID != "owner-user" {
					t.Errorf("expected userID=owner-user, got %s", userID)
				}
				return []models.Instance{{InstanceID: "owner-session-abc", Status: "connected"}}, nil
			},
		}
		s.waProvider = &mockWAProvider{
			SendTextFn: func(_ context.Context, input whatsapp.SendTextInput) (*whatsapp.MessageResult, error) {
				usedSession = input.SessionName
				return &whatsapp.MessageResult{ID: "msg-1"}, nil
			},
		}
	})

	delayZero := 0
	auto := &models.Automation{
		ID:           "auto-1",
		UserID:       "owner-user",
		Name:         "test",
		Stores:       []string{"shopee"},
		SendGroups:   []string{"dest@g.us"},
		Status:       "active",
		DelaySeconds: &delayZero,
	}

	err := svc.processAutomation(context.Background(), auto, &models.IncomingMessage{
		InstanceName: "other-user-session", // webhook from a different user's session
		RemoteJID:    "source@g.us",
		Body:         "https://shopee.com.br/product-i.123.456",
	}, []string{"https://shopee.com.br/product-i.123.456"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if usedSession != "owner-session-abc" {
		t.Errorf("expected session=owner-session-abc, got %s", usedSession)
	}
}

func TestProcessAutomation_NoConnectedInstance_Skips(t *testing.T) {
	var sent bool
	svc := newTestService(t, func(s *monitoringService) {
		s.instanceRepo = &mockInstanceRepo{
			GetUserInstancesFn: func(_ context.Context, _ string) ([]models.Instance, error) {
				return []models.Instance{{InstanceID: "old-session", Status: "disconnected"}}, nil
			},
		}
		s.waProvider = &mockWAProvider{
			SendTextFn: func(_ context.Context, _ whatsapp.SendTextInput) (*whatsapp.MessageResult, error) {
				sent = true
				return &whatsapp.MessageResult{ID: "msg-1"}, nil
			},
		}
	})

	delayZero := 0
	auto := &models.Automation{
		ID:           "auto-1",
		UserID:       "user-1",
		Name:         "test",
		Stores:       []string{"shopee"},
		SendGroups:   []string{"dest@g.us"},
		Status:       "active",
		DelaySeconds: &delayZero,
	}

	err := svc.processAutomation(context.Background(), auto, &models.IncomingMessage{
		InstanceName: "inst1",
		RemoteJID:    "source@g.us",
		Body:         "https://shopee.com.br/product-i.123.456",
	}, []string{"https://shopee.com.br/product-i.123.456"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sent {
		t.Error("should NOT send when owner has no connected instance")
	}
}
