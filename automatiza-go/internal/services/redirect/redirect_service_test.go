package redirect

import (
	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"
	"context"
	"errors"
	"fmt"
	"testing"

	"go.uber.org/zap/zaptest"
)

// ---------------------------------------------------------------------------
// Mock: RedirectLinkRepository
// ---------------------------------------------------------------------------

type mockRedirectLinkRepo struct {
	GetByIDFn         func(ctx context.Context, linkID string) (*models.RedirectLink, error)
	GetBySlugFn       func(ctx context.Context, slug string) (*models.RedirectLink, error)
	GetByUserIDFn     func(ctx context.Context, userID string) ([]models.RedirectLink, error)
	CheckSlugExistsFn func(ctx context.Context, slug string, excludeID string) (bool, error)
	CreateFn          func(ctx context.Context, link *models.RedirectLink) (*models.RedirectLink, error)
	UpdateFn          func(ctx context.Context, linkID string, updates map[string]interface{}) error
	DeleteFn          func(ctx context.Context, linkID, userID string) error
	IncrementClicksFn func(ctx context.Context, linkID, groupID string) error
}

func (m *mockRedirectLinkRepo) GetByID(ctx context.Context, linkID string) (*models.RedirectLink, error) {
	if m.GetByIDFn != nil {
		return m.GetByIDFn(ctx, linkID)
	}
	return nil, nil
}
func (m *mockRedirectLinkRepo) GetBySlug(ctx context.Context, slug string) (*models.RedirectLink, error) {
	if m.GetBySlugFn != nil {
		return m.GetBySlugFn(ctx, slug)
	}
	return nil, nil
}
func (m *mockRedirectLinkRepo) GetByUserID(ctx context.Context, userID string) ([]models.RedirectLink, error) {
	if m.GetByUserIDFn != nil {
		return m.GetByUserIDFn(ctx, userID)
	}
	return nil, nil
}
func (m *mockRedirectLinkRepo) CheckSlugExists(ctx context.Context, slug string, excludeID string) (bool, error) {
	if m.CheckSlugExistsFn != nil {
		return m.CheckSlugExistsFn(ctx, slug, excludeID)
	}
	return false, nil
}
func (m *mockRedirectLinkRepo) Create(ctx context.Context, link *models.RedirectLink) (*models.RedirectLink, error) {
	if m.CreateFn != nil {
		return m.CreateFn(ctx, link)
	}
	link.ID = "generated-id"
	return link, nil
}
func (m *mockRedirectLinkRepo) Update(ctx context.Context, linkID string, updates map[string]interface{}) error {
	if m.UpdateFn != nil {
		return m.UpdateFn(ctx, linkID, updates)
	}
	return nil
}
func (m *mockRedirectLinkRepo) Delete(ctx context.Context, linkID, userID string) error {
	if m.DeleteFn != nil {
		return m.DeleteFn(ctx, linkID, userID)
	}
	return nil
}
func (m *mockRedirectLinkRepo) IncrementClicks(ctx context.Context, linkID, groupID string) error {
	if m.IncrementClicksFn != nil {
		return m.IncrementClicksFn(ctx, linkID, groupID)
	}
	return nil
}

var _ repository.RedirectLinkRepository = (*mockRedirectLinkRepo)(nil)

// ---------------------------------------------------------------------------
// Mock: GroupRepository
// ---------------------------------------------------------------------------

type mockGroupRepo struct {
	GetUserGroupsFn          func(ctx context.Context, userID string, adminOnly bool) ([]models.Group, error)
	GetUserGroupsPaginatedFn func(ctx context.Context, userID string, page, pageSize int, adminOnly bool, search string) (*repository.GroupPaginatedResult, error)
	GetGroupByIDFn           func(ctx context.Context, groupID string) (*models.Group, error)
	GetGroupByWAGroupIDFn    func(ctx context.Context, userID, waGroupID string) (*models.Group, error)
	GetGroupsByIDsFn         func(ctx context.Context, userID string, groupIDs []string) ([]models.Group, error)
	GetFavoriteGroupsFn      func(ctx context.Context, userID string) ([]models.Group, error)
	CreateGroupFn            func(ctx context.Context, userID string, group *models.Group) (*models.Group, error)
	SyncGroupsFn             func(ctx context.Context, userID string, instanceID int, groups []models.Group, options repository.SyncOptions) error
	SyncGroupsWithStatsFn    func(ctx context.Context, userID string, instanceID int, groups []models.Group, options repository.SyncOptions) (*repository.SyncResult, error)
	IsUserAdminInGroupFn     func(ctx context.Context, userID, waGroupID string) (bool, error)
	UpdateGroupFn            func(ctx context.Context, groupID string, updates map[string]interface{}) error
	ToggleFavoriteFn         func(ctx context.Context, groupID, userID string, favorite bool) error
	DeleteGroupFn            func(ctx context.Context, groupID, userID string) error
	DeleteGroupsByInstanceFn func(ctx context.Context, userID string, instanceID int) error
	BulkUpdateGroupsFn       func(ctx context.Context, userID string, groupIDs []string, updates map[string]interface{}) (int64, error)
	CountAllFn               func(ctx context.Context) (int64, error)
}

func (m *mockGroupRepo) GetUserGroups(ctx context.Context, userID string, adminOnly bool) ([]models.Group, error) {
	if m.GetUserGroupsFn != nil {
		return m.GetUserGroupsFn(ctx, userID, adminOnly)
	}
	return nil, nil
}
func (m *mockGroupRepo) GetUserGroupsPaginated(ctx context.Context, userID string, page, pageSize int, adminOnly bool, search string) (*repository.GroupPaginatedResult, error) {
	if m.GetUserGroupsPaginatedFn != nil {
		return m.GetUserGroupsPaginatedFn(ctx, userID, page, pageSize, adminOnly, search)
	}
	return nil, nil
}
func (m *mockGroupRepo) GetGroupByID(ctx context.Context, groupID string) (*models.Group, error) {
	if m.GetGroupByIDFn != nil {
		return m.GetGroupByIDFn(ctx, groupID)
	}
	return nil, nil
}
func (m *mockGroupRepo) GetGroupByWAGroupID(ctx context.Context, userID, waGroupID string) (*models.Group, error) {
	if m.GetGroupByWAGroupIDFn != nil {
		return m.GetGroupByWAGroupIDFn(ctx, userID, waGroupID)
	}
	return nil, nil
}
func (m *mockGroupRepo) GetGroupsByIDs(ctx context.Context, userID string, groupIDs []string) ([]models.Group, error) {
	if m.GetGroupsByIDsFn != nil {
		return m.GetGroupsByIDsFn(ctx, userID, groupIDs)
	}
	return nil, nil
}
func (m *mockGroupRepo) GetFavoriteGroups(ctx context.Context, userID string) ([]models.Group, error) {
	if m.GetFavoriteGroupsFn != nil {
		return m.GetFavoriteGroupsFn(ctx, userID)
	}
	return nil, nil
}
func (m *mockGroupRepo) CreateGroup(ctx context.Context, userID string, group *models.Group) (*models.Group, error) {
	if m.CreateGroupFn != nil {
		return m.CreateGroupFn(ctx, userID, group)
	}
	return nil, nil
}
func (m *mockGroupRepo) SyncGroups(ctx context.Context, userID string, instanceID int, groups []models.Group, options repository.SyncOptions) error {
	if m.SyncGroupsFn != nil {
		return m.SyncGroupsFn(ctx, userID, instanceID, groups, options)
	}
	return nil
}
func (m *mockGroupRepo) SyncGroupsWithStats(ctx context.Context, userID string, instanceID int, groups []models.Group, options repository.SyncOptions) (*repository.SyncResult, error) {
	if m.SyncGroupsWithStatsFn != nil {
		return m.SyncGroupsWithStatsFn(ctx, userID, instanceID, groups, options)
	}
	return nil, nil
}
func (m *mockGroupRepo) IsUserAdminInGroup(ctx context.Context, userID, waGroupID string) (bool, error) {
	if m.IsUserAdminInGroupFn != nil {
		return m.IsUserAdminInGroupFn(ctx, userID, waGroupID)
	}
	return false, nil
}
func (m *mockGroupRepo) UpdateGroup(ctx context.Context, groupID string, updates map[string]interface{}) error {
	if m.UpdateGroupFn != nil {
		return m.UpdateGroupFn(ctx, groupID, updates)
	}
	return nil
}
func (m *mockGroupRepo) ToggleFavorite(ctx context.Context, groupID, userID string, favorite bool) error {
	if m.ToggleFavoriteFn != nil {
		return m.ToggleFavoriteFn(ctx, groupID, userID, favorite)
	}
	return nil
}
func (m *mockGroupRepo) DeleteGroup(ctx context.Context, groupID, userID string) error {
	if m.DeleteGroupFn != nil {
		return m.DeleteGroupFn(ctx, groupID, userID)
	}
	return nil
}
func (m *mockGroupRepo) DeleteGroupsByInstance(ctx context.Context, userID string, instanceID int) error {
	if m.DeleteGroupsByInstanceFn != nil {
		return m.DeleteGroupsByInstanceFn(ctx, userID, instanceID)
	}
	return nil
}
func (m *mockGroupRepo) BulkUpdateGroups(ctx context.Context, userID string, groupIDs []string, updates map[string]interface{}) (int64, error) {
	if m.BulkUpdateGroupsFn != nil {
		return m.BulkUpdateGroupsFn(ctx, userID, groupIDs, updates)
	}
	return 0, nil
}
func (m *mockGroupRepo) CountAll(ctx context.Context) (int64, error) {
	if m.CountAllFn != nil {
		return m.CountAllFn(ctx)
	}
	return 0, nil
}

var _ repository.GroupRepository = (*mockGroupRepo)(nil)

// ---------------------------------------------------------------------------
// Mock: InstanceRepository
// ---------------------------------------------------------------------------

type mockInstanceRepo struct {
	GetUserInstancesFn        func(ctx context.Context, userID string) ([]models.Instance, error)
	GetInstanceByIDFn         func(ctx context.Context, instanceID int) (*models.Instance, error)
	GetInstanceByNameFn       func(ctx context.Context, instanceName string) (*models.Instance, error)
	GetInstanceByUserAndNameFn func(ctx context.Context, userID, instanceName string) (*models.Instance, error)
	GetAllConnectedInstancesFn func(ctx context.Context) ([]models.Instance, error)
	UpsertInstanceFn          func(ctx context.Context, userID string, data *models.Instance) (*models.Instance, error)
	UpdateInstanceStatusFn    func(ctx context.Context, instanceID int, status string) error
	DeleteInstanceFn          func(ctx context.Context, instanceID int) error
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

var _ repository.InstanceRepository = (*mockInstanceRepo)(nil)

// ---------------------------------------------------------------------------
// Mock: WhatsAppProvider (minimal, only methods used by redirect)
// ---------------------------------------------------------------------------

type mockWAProvider struct {
	GetGroupInviteCodeFn func(ctx context.Context, sessionName, groupID string) (string, error)
}

// SessionManager
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

// GroupManager
func (m *mockWAProvider) GetGroups(_ context.Context, _ string) ([]whatsapp.GroupInfo, error) {
	return nil, nil
}
func (m *mockWAProvider) GetGroupParticipants(_ context.Context, _, _ string) ([]whatsapp.ParticipantInfo, error) {
	return nil, nil
}
func (m *mockWAProvider) GetGroupInviteCode(ctx context.Context, sessionName, groupID string) (string, error) {
	if m.GetGroupInviteCodeFn != nil {
		return m.GetGroupInviteCodeFn(ctx, sessionName, groupID)
	}
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

// MessageSender
func (m *mockWAProvider) SendText(_ context.Context, _ whatsapp.SendTextInput) (*whatsapp.MessageResult, error) {
	return nil, nil
}
func (m *mockWAProvider) SendImage(_ context.Context, _ whatsapp.SendImageInput) (*whatsapp.MessageResult, error) {
	return nil, nil
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

// WebhookManager
func (m *mockWAProvider) SetWebhook(_ context.Context, _ whatsapp.SetWebhookInput) error { return nil }

// MessageReconciler
func (m *mockWAProvider) GetMessages(_ context.Context, _ whatsapp.GetMessagesInput) (*whatsapp.MessagesResult, error) {
	return nil, nil
}

// MediaDownloader
func (m *mockWAProvider) DownloadMedia(_ context.Context, _ whatsapp.DownloadMediaInput) (*whatsapp.DownloadMediaResult, error) {
	return nil, nil
}
func (m *mockWAProvider) DownloadMediaByURL(_ context.Context, _ string) (*whatsapp.DownloadMediaResult, error) {
	return nil, nil
}

var _ whatsapp.WhatsAppProvider = (*mockWAProvider)(nil)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func newTestService(t *testing.T, redirectRepo *mockRedirectLinkRepo, groupRepo *mockGroupRepo, instanceRepo *mockInstanceRepo, waProvider whatsapp.WhatsAppProvider) *redirectService {
	t.Helper()
	return &redirectService{
		redirectLinkRepo: redirectRepo,
		groupRepo:        groupRepo,
		instanceRepo:     instanceRepo,
		wahaProvider:     waProvider,
		logger:           zaptest.NewLogger(t),
	}
}

func strPtr(s string) *string { return &s }
func boolPtr(b bool) *bool    { return &b }

func makeActiveLink(strategy string, groups []models.GroupPriority) *models.RedirectLink {
	return &models.RedirectLink{
		ID:           "link-1",
		UserID:       "user-1",
		Slug:         "test-slug",
		Name:         "Test Link",
		Strategy:     strategy,
		TargetGroups: groups,
		IsActive:     true,
		GroupClicks:  make(map[string]int),
	}
}

func makeGroup(id, instanceID, waGroupID, name string, inviteCode *string) *models.Group {
	return &models.Group{
		ID:         id,
		Name:       name,
		WAGroupID:  waGroupID,
		InstanceID: instanceID,
		IsAdmin:    true,
		InviteCode: inviteCode,
		Status:     "open",
	}
}

func makeInstance(id int, instanceID string) *models.Instance {
	return &models.Instance{
		ID:         id,
		InstanceID: instanceID,
		Status:     "connected",
	}
}

// ---------------------------------------------------------------------------
// Tests: ProcessRedirect
// ---------------------------------------------------------------------------

func TestProcessRedirect(t *testing.T) {
	ctx := context.Background()

	t.Run("both slug and linkID empty returns error", func(t *testing.T) {
		svc := newTestService(t, &mockRedirectLinkRepo{}, &mockGroupRepo{}, &mockInstanceRepo{}, &mockWAProvider{})
		result, err := svc.ProcessRedirect(ctx, "", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Success {
			t.Error("expected Success=false")
		}
		if result.Error != "slug or linkId is required" {
			t.Errorf("error = %q, want %q", result.Error, "slug or linkId is required")
		}
	})

	t.Run("link not found (nil) returns error", func(t *testing.T) {
		redirectRepo := &mockRedirectLinkRepo{
			GetBySlugFn: func(_ context.Context, _ string) (*models.RedirectLink, error) {
				return nil, nil
			},
		}
		svc := newTestService(t, redirectRepo, &mockGroupRepo{}, &mockInstanceRepo{}, &mockWAProvider{})
		result, err := svc.ProcessRedirect(ctx, "test-slug", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Error != "link not found" {
			t.Errorf("error = %q, want %q", result.Error, "link not found")
		}
	})

	t.Run("repo error returns internal error", func(t *testing.T) {
		redirectRepo := &mockRedirectLinkRepo{
			GetBySlugFn: func(_ context.Context, _ string) (*models.RedirectLink, error) {
				return nil, fmt.Errorf("db connection failed")
			},
		}
		svc := newTestService(t, redirectRepo, &mockGroupRepo{}, &mockInstanceRepo{}, &mockWAProvider{})
		result, err := svc.ProcessRedirect(ctx, "test-slug", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Error != "internal error" {
			t.Errorf("error = %q, want %q", result.Error, "internal error")
		}
	})

	t.Run("link inactive returns error", func(t *testing.T) {
		link := makeActiveLink(models.RedirectStrategyRoundRobin, []models.GroupPriority{{GroupID: "g1", IsActive: true}})
		link.IsActive = false
		redirectRepo := &mockRedirectLinkRepo{
			GetBySlugFn: func(_ context.Context, _ string) (*models.RedirectLink, error) {
				return link, nil
			},
		}
		svc := newTestService(t, redirectRepo, &mockGroupRepo{}, &mockInstanceRepo{}, &mockWAProvider{})
		result, _ := svc.ProcessRedirect(ctx, "test-slug", "")
		if result.Error != "link is inactive" {
			t.Errorf("error = %q, want %q", result.Error, "link is inactive")
		}
	})

	t.Run("no target groups returns error", func(t *testing.T) {
		link := makeActiveLink(models.RedirectStrategyRoundRobin, []models.GroupPriority{})
		redirectRepo := &mockRedirectLinkRepo{
			GetBySlugFn: func(_ context.Context, _ string) (*models.RedirectLink, error) {
				return link, nil
			},
		}
		svc := newTestService(t, redirectRepo, &mockGroupRepo{}, &mockInstanceRepo{}, &mockWAProvider{})
		result, _ := svc.ProcessRedirect(ctx, "test-slug", "")
		if result.Error != "no target groups configured" {
			t.Errorf("error = %q, want %q", result.Error, "no target groups configured")
		}
	})

	t.Run("all groups full returns error", func(t *testing.T) {
		link := makeActiveLink(models.RedirectStrategyClickLimit, []models.GroupPriority{
			{GroupID: "g1", IsActive: true, ClickLimit: 1},
		})
		link.GroupClicks = map[string]int{"g1": 1}
		redirectRepo := &mockRedirectLinkRepo{
			GetBySlugFn: func(_ context.Context, _ string) (*models.RedirectLink, error) {
				return link, nil
			},
		}
		svc := newTestService(t, redirectRepo, &mockGroupRepo{}, &mockInstanceRepo{}, &mockWAProvider{})
		result, _ := svc.ProcessRedirect(ctx, "test-slug", "")
		if result.Success {
			t.Error("expected Success=false")
		}
		// The error comes from GetAvailableGroup returning nil
		if result.Error == "" {
			t.Error("expected non-empty error for full groups")
		}
	})

	t.Run("group not found returns error", func(t *testing.T) {
		link := makeActiveLink(models.RedirectStrategyRoundRobin, []models.GroupPriority{
			{GroupID: "g1", IsActive: true, Priority: 1},
		})
		redirectRepo := &mockRedirectLinkRepo{
			GetBySlugFn: func(_ context.Context, _ string) (*models.RedirectLink, error) {
				return link, nil
			},
		}
		groupRepo := &mockGroupRepo{
			GetGroupByIDFn: func(_ context.Context, _ string) (*models.Group, error) {
				return nil, nil
			},
		}
		svc := newTestService(t, redirectRepo, groupRepo, &mockInstanceRepo{}, &mockWAProvider{})
		result, _ := svc.ProcessRedirect(ctx, "test-slug", "")
		if result.Error != "group not found" {
			t.Errorf("error = %q, want %q", result.Error, "group not found")
		}
	})

	t.Run("instance not found returns error", func(t *testing.T) {
		link := makeActiveLink(models.RedirectStrategyRoundRobin, []models.GroupPriority{
			{GroupID: "g1", IsActive: true, Priority: 1},
		})
		redirectRepo := &mockRedirectLinkRepo{
			GetBySlugFn: func(_ context.Context, _ string) (*models.RedirectLink, error) {
				return link, nil
			},
		}
		groupRepo := &mockGroupRepo{
			GetGroupByIDFn: func(_ context.Context, _ string) (*models.Group, error) {
				return makeGroup("g1", "5", "wa-g1@g.us", "Group 1", nil), nil
			},
		}
		instanceRepo := &mockInstanceRepo{
			GetInstanceByIDFn: func(_ context.Context, _ int) (*models.Instance, error) {
				return nil, nil
			},
		}
		svc := newTestService(t, redirectRepo, groupRepo, instanceRepo, &mockWAProvider{})
		result, _ := svc.ProcessRedirect(ctx, "test-slug", "")
		if result.Error != "instance not found" {
			t.Errorf("error = %q, want %q", result.Error, "instance not found")
		}
	})

	t.Run("invalid instance ID (non-numeric) returns error", func(t *testing.T) {
		link := makeActiveLink(models.RedirectStrategyRoundRobin, []models.GroupPriority{
			{GroupID: "g1", IsActive: true, Priority: 1},
		})
		redirectRepo := &mockRedirectLinkRepo{
			GetBySlugFn: func(_ context.Context, _ string) (*models.RedirectLink, error) {
				return link, nil
			},
		}
		groupRepo := &mockGroupRepo{
			GetGroupByIDFn: func(_ context.Context, _ string) (*models.Group, error) {
				return makeGroup("g1", "not-a-number", "wa-g1@g.us", "Group 1", nil), nil
			},
		}
		svc := newTestService(t, redirectRepo, groupRepo, &mockInstanceRepo{}, &mockWAProvider{})
		result, _ := svc.ProcessRedirect(ctx, "test-slug", "")
		if result.Error != "instance not found" {
			t.Errorf("error = %q, want %q", result.Error, "instance not found")
		}
	})

	t.Run("invalid instance ID (zero) returns error", func(t *testing.T) {
		link := makeActiveLink(models.RedirectStrategyRoundRobin, []models.GroupPriority{
			{GroupID: "g1", IsActive: true, Priority: 1},
		})
		redirectRepo := &mockRedirectLinkRepo{
			GetBySlugFn: func(_ context.Context, _ string) (*models.RedirectLink, error) {
				return link, nil
			},
		}
		groupRepo := &mockGroupRepo{
			GetGroupByIDFn: func(_ context.Context, _ string) (*models.Group, error) {
				return makeGroup("g1", "0", "wa-g1@g.us", "Group 1", nil), nil
			},
		}
		svc := newTestService(t, redirectRepo, groupRepo, &mockInstanceRepo{}, &mockWAProvider{})
		result, _ := svc.ProcessRedirect(ctx, "test-slug", "")
		if result.Error != "instance not found" {
			t.Errorf("error = %q, want %q", result.Error, "instance not found")
		}
	})

	t.Run("invalid instance ID (negative) returns error", func(t *testing.T) {
		link := makeActiveLink(models.RedirectStrategyRoundRobin, []models.GroupPriority{
			{GroupID: "g1", IsActive: true, Priority: 1},
		})
		redirectRepo := &mockRedirectLinkRepo{
			GetBySlugFn: func(_ context.Context, _ string) (*models.RedirectLink, error) {
				return link, nil
			},
		}
		groupRepo := &mockGroupRepo{
			GetGroupByIDFn: func(_ context.Context, _ string) (*models.Group, error) {
				return makeGroup("g1", "-1", "wa-g1@g.us", "Group 1", nil), nil
			},
		}
		svc := newTestService(t, redirectRepo, groupRepo, &mockInstanceRepo{}, &mockWAProvider{})
		result, _ := svc.ProcessRedirect(ctx, "test-slug", "")
		if result.Error != "instance not found" {
			t.Errorf("error = %q, want %q", result.Error, "instance not found")
		}
	})

	t.Run("success with saved invite code (no WAHA call)", func(t *testing.T) {
		inviteCode := "ABCDEF123"
		link := makeActiveLink(models.RedirectStrategyRoundRobin, []models.GroupPriority{
			{GroupID: "g1", IsActive: true, Priority: 1},
		})
		redirectRepo := &mockRedirectLinkRepo{
			GetBySlugFn: func(_ context.Context, _ string) (*models.RedirectLink, error) {
				return link, nil
			},
		}
		groupRepo := &mockGroupRepo{
			GetGroupByIDFn: func(_ context.Context, _ string) (*models.Group, error) {
				return makeGroup("g1", "5", "wa-g1@g.us", "Group 1", &inviteCode), nil
			},
		}
		instanceRepo := &mockInstanceRepo{
			GetInstanceByIDFn: func(_ context.Context, _ int) (*models.Instance, error) {
				return makeInstance(5, "session-1"), nil
			},
		}
		waProviderCalled := false
		waProvider := &mockWAProvider{
			GetGroupInviteCodeFn: func(_ context.Context, _, _ string) (string, error) {
				waProviderCalled = true
				return "SHOULD_NOT_BE_CALLED", nil
			},
		}
		svc := newTestService(t, redirectRepo, groupRepo, instanceRepo, waProvider)
		result, err := svc.ProcessRedirect(ctx, "test-slug", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Success {
			t.Errorf("expected Success=true, error=%q", result.Error)
		}
		if result.RedirectURL != "https://chat.whatsapp.com/ABCDEF123" {
			t.Errorf("redirectURL = %q, want %q", result.RedirectURL, "https://chat.whatsapp.com/ABCDEF123")
		}
		if waProviderCalled {
			t.Error("WAHA provider should not be called when invite code is saved in DB")
		}
	})

	t.Run("success with WAHA invite code (no saved code)", func(t *testing.T) {
		link := makeActiveLink(models.RedirectStrategyRoundRobin, []models.GroupPriority{
			{GroupID: "g1", IsActive: true, Priority: 1},
		})
		redirectRepo := &mockRedirectLinkRepo{
			GetBySlugFn: func(_ context.Context, _ string) (*models.RedirectLink, error) {
				return link, nil
			},
		}
		groupRepo := &mockGroupRepo{
			GetGroupByIDFn: func(_ context.Context, _ string) (*models.Group, error) {
				return makeGroup("g1", "5", "wa-g1@g.us", "Group 1", nil), nil
			},
		}
		instanceRepo := &mockInstanceRepo{
			GetInstanceByIDFn: func(_ context.Context, _ int) (*models.Instance, error) {
				return makeInstance(5, "session-1"), nil
			},
		}
		waProvider := &mockWAProvider{
			GetGroupInviteCodeFn: func(_ context.Context, _, _ string) (string, error) {
				return "WAHA_CODE_XYZ", nil
			},
		}
		svc := newTestService(t, redirectRepo, groupRepo, instanceRepo, waProvider)
		result, err := svc.ProcessRedirect(ctx, "test-slug", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Success {
			t.Errorf("expected Success=true, error=%q", result.Error)
		}
		if result.RedirectURL != "https://chat.whatsapp.com/WAHA_CODE_XYZ" {
			t.Errorf("redirectURL = %q, want %q", result.RedirectURL, "https://chat.whatsapp.com/WAHA_CODE_XYZ")
		}
	})

	t.Run("WAHA error (rate limited) returns service temporarily unavailable", func(t *testing.T) {
		link := makeActiveLink(models.RedirectStrategyRoundRobin, []models.GroupPriority{
			{GroupID: "g1", IsActive: true, Priority: 1},
		})
		redirectRepo := &mockRedirectLinkRepo{
			GetBySlugFn: func(_ context.Context, _ string) (*models.RedirectLink, error) {
				return link, nil
			},
		}
		groupRepo := &mockGroupRepo{
			GetGroupByIDFn: func(_ context.Context, _ string) (*models.Group, error) {
				return makeGroup("g1", "5", "wa-g1@g.us", "Group 1", nil), nil
			},
		}
		instanceRepo := &mockInstanceRepo{
			GetInstanceByIDFn: func(_ context.Context, _ int) (*models.Instance, error) {
				return makeInstance(5, "session-1"), nil
			},
		}
		waProvider := &mockWAProvider{
			GetGroupInviteCodeFn: func(_ context.Context, _, _ string) (string, error) {
				return "", whatsapp.NewProviderError(whatsapp.ErrCodeRateLimited, "rate limited", nil)
			},
		}
		svc := newTestService(t, redirectRepo, groupRepo, instanceRepo, waProvider)
		result, _ := svc.ProcessRedirect(ctx, "test-slug", "")
		if result.Error != "service temporarily unavailable" {
			t.Errorf("error = %q, want %q", result.Error, "service temporarily unavailable")
		}
	})

	t.Run("WAHA error (timeout) returns service temporarily unavailable", func(t *testing.T) {
		link := makeActiveLink(models.RedirectStrategyRoundRobin, []models.GroupPriority{
			{GroupID: "g1", IsActive: true, Priority: 1},
		})
		redirectRepo := &mockRedirectLinkRepo{
			GetBySlugFn: func(_ context.Context, _ string) (*models.RedirectLink, error) {
				return link, nil
			},
		}
		groupRepo := &mockGroupRepo{
			GetGroupByIDFn: func(_ context.Context, _ string) (*models.Group, error) {
				return makeGroup("g1", "5", "wa-g1@g.us", "Group 1", nil), nil
			},
		}
		instanceRepo := &mockInstanceRepo{
			GetInstanceByIDFn: func(_ context.Context, _ int) (*models.Instance, error) {
				return makeInstance(5, "session-1"), nil
			},
		}
		waProvider := &mockWAProvider{
			GetGroupInviteCodeFn: func(_ context.Context, _, _ string) (string, error) {
				return "", whatsapp.NewProviderError(whatsapp.ErrCodeTimeout, "timeout", nil)
			},
		}
		svc := newTestService(t, redirectRepo, groupRepo, instanceRepo, waProvider)
		result, _ := svc.ProcessRedirect(ctx, "test-slug", "")
		if result.Error != "service temporarily unavailable" {
			t.Errorf("error = %q, want %q", result.Error, "service temporarily unavailable")
		}
	})

	t.Run("WAHA error (permission denied) returns bot not admin", func(t *testing.T) {
		link := makeActiveLink(models.RedirectStrategyRoundRobin, []models.GroupPriority{
			{GroupID: "g1", IsActive: true, Priority: 1},
		})
		redirectRepo := &mockRedirectLinkRepo{
			GetBySlugFn: func(_ context.Context, _ string) (*models.RedirectLink, error) {
				return link, nil
			},
		}
		groupRepo := &mockGroupRepo{
			GetGroupByIDFn: func(_ context.Context, _ string) (*models.Group, error) {
				return makeGroup("g1", "5", "wa-g1@g.us", "Group 1", nil), nil
			},
		}
		instanceRepo := &mockInstanceRepo{
			GetInstanceByIDFn: func(_ context.Context, _ int) (*models.Instance, error) {
				return makeInstance(5, "session-1"), nil
			},
		}
		waProvider := &mockWAProvider{
			GetGroupInviteCodeFn: func(_ context.Context, _, _ string) (string, error) {
				return "", whatsapp.NewProviderError(whatsapp.ErrCodePermissionDenied, "not admin", nil)
			},
		}
		svc := newTestService(t, redirectRepo, groupRepo, instanceRepo, waProvider)
		result, _ := svc.ProcessRedirect(ctx, "test-slug", "")
		if result.Error != "bot is not admin in group" {
			t.Errorf("error = %q, want %q", result.Error, "bot is not admin in group")
		}
	})

	t.Run("WAHA error (generic) returns failed to get invite code", func(t *testing.T) {
		link := makeActiveLink(models.RedirectStrategyRoundRobin, []models.GroupPriority{
			{GroupID: "g1", IsActive: true, Priority: 1},
		})
		redirectRepo := &mockRedirectLinkRepo{
			GetBySlugFn: func(_ context.Context, _ string) (*models.RedirectLink, error) {
				return link, nil
			},
		}
		groupRepo := &mockGroupRepo{
			GetGroupByIDFn: func(_ context.Context, _ string) (*models.Group, error) {
				return makeGroup("g1", "5", "wa-g1@g.us", "Group 1", nil), nil
			},
		}
		instanceRepo := &mockInstanceRepo{
			GetInstanceByIDFn: func(_ context.Context, _ int) (*models.Instance, error) {
				return makeInstance(5, "session-1"), nil
			},
		}
		waProvider := &mockWAProvider{
			GetGroupInviteCodeFn: func(_ context.Context, _, _ string) (string, error) {
				return "", fmt.Errorf("some unknown error")
			},
		}
		svc := newTestService(t, redirectRepo, groupRepo, instanceRepo, waProvider)
		result, _ := svc.ProcessRedirect(ctx, "test-slug", "")
		if result.Error != "failed to get invite code" {
			t.Errorf("error = %q, want %q", result.Error, "failed to get invite code")
		}
	})

	t.Run("empty invite code from WAHA", func(t *testing.T) {
		link := makeActiveLink(models.RedirectStrategyRoundRobin, []models.GroupPriority{
			{GroupID: "g1", IsActive: true, Priority: 1},
		})
		redirectRepo := &mockRedirectLinkRepo{
			GetBySlugFn: func(_ context.Context, _ string) (*models.RedirectLink, error) {
				return link, nil
			},
		}
		groupRepo := &mockGroupRepo{
			GetGroupByIDFn: func(_ context.Context, _ string) (*models.Group, error) {
				return makeGroup("g1", "5", "wa-g1@g.us", "Group 1", nil), nil
			},
		}
		instanceRepo := &mockInstanceRepo{
			GetInstanceByIDFn: func(_ context.Context, _ int) (*models.Instance, error) {
				return makeInstance(5, "session-1"), nil
			},
		}
		waProvider := &mockWAProvider{
			GetGroupInviteCodeFn: func(_ context.Context, _, _ string) (string, error) {
				return "", nil
			},
		}
		svc := newTestService(t, redirectRepo, groupRepo, instanceRepo, waProvider)
		result, _ := svc.ProcessRedirect(ctx, "test-slug", "")
		if result.Error != "invite code not available" {
			t.Errorf("error = %q, want %q", result.Error, "invite code not available")
		}
	})

	t.Run("nil wahaProvider returns error", func(t *testing.T) {
		link := makeActiveLink(models.RedirectStrategyRoundRobin, []models.GroupPriority{
			{GroupID: "g1", IsActive: true, Priority: 1},
		})
		redirectRepo := &mockRedirectLinkRepo{
			GetBySlugFn: func(_ context.Context, _ string) (*models.RedirectLink, error) {
				return link, nil
			},
		}
		groupRepo := &mockGroupRepo{
			GetGroupByIDFn: func(_ context.Context, _ string) (*models.Group, error) {
				return makeGroup("g1", "5", "wa-g1@g.us", "Group 1", nil), nil
			},
		}
		instanceRepo := &mockInstanceRepo{
			GetInstanceByIDFn: func(_ context.Context, _ int) (*models.Instance, error) {
				return makeInstance(5, "session-1"), nil
			},
		}
		svc := newTestService(t, redirectRepo, groupRepo, instanceRepo, nil)
		result, _ := svc.ProcessRedirect(ctx, "test-slug", "")
		if result.Error != "WhatsApp provider not configured" {
			t.Errorf("error = %q, want %q", result.Error, "WhatsApp provider not configured")
		}
	})

	t.Run("saves invite code to DB after WAHA fetch", func(t *testing.T) {
		link := makeActiveLink(models.RedirectStrategyRoundRobin, []models.GroupPriority{
			{GroupID: "g1", IsActive: true, Priority: 1},
		})
		redirectRepo := &mockRedirectLinkRepo{
			GetBySlugFn: func(_ context.Context, _ string) (*models.RedirectLink, error) {
				return link, nil
			},
		}
		var savedInviteCode string
		groupRepo := &mockGroupRepo{
			GetGroupByIDFn: func(_ context.Context, _ string) (*models.Group, error) {
				return makeGroup("g1", "5", "wa-g1@g.us", "Group 1", nil), nil
			},
			UpdateGroupFn: func(_ context.Context, _ string, updates map[string]interface{}) error {
				if ic, ok := updates["invite_code"].(string); ok {
					savedInviteCode = ic
				}
				return nil
			},
		}
		instanceRepo := &mockInstanceRepo{
			GetInstanceByIDFn: func(_ context.Context, _ int) (*models.Instance, error) {
				return makeInstance(5, "session-1"), nil
			},
		}
		waProvider := &mockWAProvider{
			GetGroupInviteCodeFn: func(_ context.Context, _, _ string) (string, error) {
				return "SAVED_CODE", nil
			},
		}
		svc := newTestService(t, redirectRepo, groupRepo, instanceRepo, waProvider)
		result, err := svc.ProcessRedirect(ctx, "test-slug", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Success {
			t.Errorf("expected Success=true, error=%q", result.Error)
		}
		if savedInviteCode != "SAVED_CODE" {
			t.Errorf("saved invite code = %q, want %q", savedInviteCode, "SAVED_CODE")
		}
	})

	t.Run("increments clicks (warns on error, does not fail)", func(t *testing.T) {
		inviteCode := "CODE123"
		link := makeActiveLink(models.RedirectStrategyRoundRobin, []models.GroupPriority{
			{GroupID: "g1", IsActive: true, Priority: 1},
		})
		clickIncrementCalled := false
		redirectRepo := &mockRedirectLinkRepo{
			GetBySlugFn: func(_ context.Context, _ string) (*models.RedirectLink, error) {
				return link, nil
			},
			IncrementClicksFn: func(_ context.Context, _, _ string) error {
				clickIncrementCalled = true
				return fmt.Errorf("increment error")
			},
		}
		groupRepo := &mockGroupRepo{
			GetGroupByIDFn: func(_ context.Context, _ string) (*models.Group, error) {
				return makeGroup("g1", "5", "wa-g1@g.us", "Group 1", &inviteCode), nil
			},
		}
		instanceRepo := &mockInstanceRepo{
			GetInstanceByIDFn: func(_ context.Context, _ int) (*models.Instance, error) {
				return makeInstance(5, "session-1"), nil
			},
		}
		svc := newTestService(t, redirectRepo, groupRepo, instanceRepo, &mockWAProvider{})
		result, err := svc.ProcessRedirect(ctx, "test-slug", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Success {
			t.Errorf("expected Success=true even on increment error, error=%q", result.Error)
		}
		if !clickIncrementCalled {
			t.Error("expected IncrementClicks to be called")
		}
	})

	t.Run("lookup by linkID vs slug", func(t *testing.T) {
		inviteCode := "BY_ID_CODE"
		link := makeActiveLink(models.RedirectStrategyRoundRobin, []models.GroupPriority{
			{GroupID: "g1", IsActive: true, Priority: 1},
		})
		getByIDCalled := false
		redirectRepo := &mockRedirectLinkRepo{
			GetByIDFn: func(_ context.Context, _ string) (*models.RedirectLink, error) {
				getByIDCalled = true
				return link, nil
			},
			GetBySlugFn: func(_ context.Context, _ string) (*models.RedirectLink, error) {
				t.Error("GetBySlug should not be called when linkID is provided")
				return nil, nil
			},
		}
		groupRepo := &mockGroupRepo{
			GetGroupByIDFn: func(_ context.Context, _ string) (*models.Group, error) {
				return makeGroup("g1", "5", "wa-g1@g.us", "Group 1", &inviteCode), nil
			},
		}
		instanceRepo := &mockInstanceRepo{
			GetInstanceByIDFn: func(_ context.Context, _ int) (*models.Instance, error) {
				return makeInstance(5, "session-1"), nil
			},
		}
		svc := newTestService(t, redirectRepo, groupRepo, instanceRepo, &mockWAProvider{})
		result, _ := svc.ProcessRedirect(ctx, "", "link-id-123")
		if !getByIDCalled {
			t.Error("expected GetByID to be called when linkID is provided")
		}
		if !result.Success {
			t.Errorf("expected Success=true, error=%q", result.Error)
		}
	})

	t.Run("WAHA provider error (network error) returns service temporarily unavailable", func(t *testing.T) {
		link := makeActiveLink(models.RedirectStrategyRoundRobin, []models.GroupPriority{
			{GroupID: "g1", IsActive: true, Priority: 1},
		})
		redirectRepo := &mockRedirectLinkRepo{
			GetBySlugFn: func(_ context.Context, _ string) (*models.RedirectLink, error) {
				return link, nil
			},
		}
		groupRepo := &mockGroupRepo{
			GetGroupByIDFn: func(_ context.Context, _ string) (*models.Group, error) {
				return makeGroup("g1", "5", "wa-g1@g.us", "Group 1", nil), nil
			},
		}
		instanceRepo := &mockInstanceRepo{
			GetInstanceByIDFn: func(_ context.Context, _ int) (*models.Instance, error) {
				return makeInstance(5, "session-1"), nil
			},
		}
		waProvider := &mockWAProvider{
			GetGroupInviteCodeFn: func(_ context.Context, _, _ string) (string, error) {
				return "", whatsapp.NewProviderError(whatsapp.ErrCodeNetworkError, "network error", nil)
			},
		}
		svc := newTestService(t, redirectRepo, groupRepo, instanceRepo, waProvider)
		result, _ := svc.ProcessRedirect(ctx, "test-slug", "")
		if result.Error != "service temporarily unavailable" {
			t.Errorf("error = %q, want %q", result.Error, "service temporarily unavailable")
		}
	})

	t.Run("WAHA provider error (provider error) returns service temporarily unavailable", func(t *testing.T) {
		link := makeActiveLink(models.RedirectStrategyRoundRobin, []models.GroupPriority{
			{GroupID: "g1", IsActive: true, Priority: 1},
		})
		redirectRepo := &mockRedirectLinkRepo{
			GetBySlugFn: func(_ context.Context, _ string) (*models.RedirectLink, error) {
				return link, nil
			},
		}
		groupRepo := &mockGroupRepo{
			GetGroupByIDFn: func(_ context.Context, _ string) (*models.Group, error) {
				return makeGroup("g1", "5", "wa-g1@g.us", "Group 1", nil), nil
			},
		}
		instanceRepo := &mockInstanceRepo{
			GetInstanceByIDFn: func(_ context.Context, _ int) (*models.Instance, error) {
				return makeInstance(5, "session-1"), nil
			},
		}
		waProvider := &mockWAProvider{
			GetGroupInviteCodeFn: func(_ context.Context, _, _ string) (string, error) {
				return "", whatsapp.NewProviderError(whatsapp.ErrCodeProviderError, "provider error", nil)
			},
		}
		svc := newTestService(t, redirectRepo, groupRepo, instanceRepo, waProvider)
		result, _ := svc.ProcessRedirect(ctx, "test-slug", "")
		if result.Error != "service temporarily unavailable" {
			t.Errorf("error = %q, want %q", result.Error, "service temporarily unavailable")
		}
	})

	t.Run("member limit strategy checks member count", func(t *testing.T) {
		link := makeActiveLink(models.RedirectStrategyMemberLimit, []models.GroupPriority{
			{GroupID: "g1", IsActive: true, Priority: 1, MemberLimit: 100},
		})
		inviteCode := "ML_CODE"
		redirectRepo := &mockRedirectLinkRepo{
			GetBySlugFn: func(_ context.Context, _ string) (*models.RedirectLink, error) {
				return link, nil
			},
		}
		groupRepo := &mockGroupRepo{
			GetGroupByIDFn: func(_ context.Context, groupID string) (*models.Group, error) {
				g := makeGroup("g1", "5", "wa-g1@g.us", "Group 1", &inviteCode)
				g.MembersCount = 50
				return g, nil
			},
		}
		instanceRepo := &mockInstanceRepo{
			GetInstanceByIDFn: func(_ context.Context, _ int) (*models.Instance, error) {
				return makeInstance(5, "session-1"), nil
			},
		}
		svc := newTestService(t, redirectRepo, groupRepo, instanceRepo, &mockWAProvider{})
		result, _ := svc.ProcessRedirect(ctx, "test-slug", "")
		if !result.Success {
			t.Errorf("expected Success=true, error=%q", result.Error)
		}
	})
}

// ---------------------------------------------------------------------------
// Tests: ListGroups
// ---------------------------------------------------------------------------

func TestListGroups(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		groupRepo := &mockGroupRepo{
			GetUserGroupsFn: func(_ context.Context, _ string, _ bool) ([]models.Group, error) {
				return []models.Group{
					{ID: "g1", Name: "Group 1"},
					{ID: "g2", Name: "Group 2"},
				}, nil
			},
		}
		svc := newTestService(t, &mockRedirectLinkRepo{}, groupRepo, &mockInstanceRepo{}, &mockWAProvider{})
		groups, err := svc.ListGroups(ctx, "user1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(groups) != 2 {
			t.Errorf("expected 2 groups, got %d", len(groups))
		}
	})

	t.Run("error", func(t *testing.T) {
		groupRepo := &mockGroupRepo{
			GetUserGroupsFn: func(_ context.Context, _ string, _ bool) ([]models.Group, error) {
				return nil, fmt.Errorf("db error")
			},
		}
		svc := newTestService(t, &mockRedirectLinkRepo{}, groupRepo, &mockInstanceRepo{}, &mockWAProvider{})
		_, err := svc.ListGroups(ctx, "user1")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

// ---------------------------------------------------------------------------
// Tests: ListRedirectLinks
// ---------------------------------------------------------------------------

func TestListRedirectLinks(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		redirectRepo := &mockRedirectLinkRepo{
			GetByUserIDFn: func(_ context.Context, _ string) ([]models.RedirectLink, error) {
				return []models.RedirectLink{
					{ID: "l1", Slug: "slug1"},
					{ID: "l2", Slug: "slug2"},
				}, nil
			},
		}
		svc := newTestService(t, redirectRepo, &mockGroupRepo{}, &mockInstanceRepo{}, &mockWAProvider{})
		links, err := svc.ListRedirectLinks(ctx, "user1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(links) != 2 {
			t.Errorf("expected 2 links, got %d", len(links))
		}
	})

	t.Run("error", func(t *testing.T) {
		redirectRepo := &mockRedirectLinkRepo{
			GetByUserIDFn: func(_ context.Context, _ string) ([]models.RedirectLink, error) {
				return nil, fmt.Errorf("db error")
			},
		}
		svc := newTestService(t, redirectRepo, &mockGroupRepo{}, &mockInstanceRepo{}, &mockWAProvider{})
		_, err := svc.ListRedirectLinks(ctx, "user1")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

// ---------------------------------------------------------------------------
// Tests: GetRedirectLink
// ---------------------------------------------------------------------------

func TestGetRedirectLink(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		redirectRepo := &mockRedirectLinkRepo{
			GetByIDFn: func(_ context.Context, _ string) (*models.RedirectLink, error) {
				return &models.RedirectLink{ID: "l1", Slug: "test"}, nil
			},
		}
		svc := newTestService(t, redirectRepo, &mockGroupRepo{}, &mockInstanceRepo{}, &mockWAProvider{})
		link, err := svc.GetRedirectLink(ctx, "l1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if link.ID != "l1" {
			t.Errorf("ID = %q, want %q", link.ID, "l1")
		}
	})

	t.Run("error", func(t *testing.T) {
		redirectRepo := &mockRedirectLinkRepo{
			GetByIDFn: func(_ context.Context, _ string) (*models.RedirectLink, error) {
				return nil, fmt.Errorf("not found")
			},
		}
		svc := newTestService(t, redirectRepo, &mockGroupRepo{}, &mockInstanceRepo{}, &mockWAProvider{})
		_, err := svc.GetRedirectLink(ctx, "l1")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

// ---------------------------------------------------------------------------
// Tests: CreateRedirectLink
// ---------------------------------------------------------------------------

func TestCreateRedirectLink(t *testing.T) {
	ctx := context.Background()

	t.Run("slug already exists error", func(t *testing.T) {
		redirectRepo := &mockRedirectLinkRepo{
			CheckSlugExistsFn: func(_ context.Context, _ string, _ string) (bool, error) {
				return true, nil
			},
		}
		svc := newTestService(t, redirectRepo, &mockGroupRepo{}, &mockInstanceRepo{}, &mockWAProvider{})
		_, err := svc.CreateRedirectLink(ctx, "user1", &models.RedirectLinkCreateRequest{
			Slug:     "existing-slug",
			Name:     "Test",
			Strategy: models.RedirectStrategyRoundRobin,
			TargetGroups: []models.GroupPriority{
				{GroupID: "g1", IsActive: true},
			},
		})
		if err == nil {
			t.Fatal("expected error for existing slug")
		}
		if !errors.Is(err, errors.New("slug already in use")) && err.Error() != "slug already in use" {
			t.Errorf("error = %q, want %q", err.Error(), "slug already in use")
		}
	})

	t.Run("invalid strategy error", func(t *testing.T) {
		redirectRepo := &mockRedirectLinkRepo{
			CheckSlugExistsFn: func(_ context.Context, _ string, _ string) (bool, error) {
				return false, nil
			},
		}
		svc := newTestService(t, redirectRepo, &mockGroupRepo{}, &mockInstanceRepo{}, &mockWAProvider{})
		_, err := svc.CreateRedirectLink(ctx, "user1", &models.RedirectLinkCreateRequest{
			Slug:     "new-slug",
			Name:     "Test",
			Strategy: "invalid_strategy",
		})
		if err == nil {
			t.Fatal("expected error for invalid strategy")
		}
	})

	t.Run("valid strategies", func(t *testing.T) {
		strategies := []string{
			models.RedirectStrategyMemberLimit,
			models.RedirectStrategyClickLimit,
			models.RedirectStrategyRoundRobin,
		}
		for _, strategy := range strategies {
			t.Run(strategy, func(t *testing.T) {
				redirectRepo := &mockRedirectLinkRepo{
					CheckSlugExistsFn: func(_ context.Context, _ string, _ string) (bool, error) {
						return false, nil
					},
					CreateFn: func(_ context.Context, link *models.RedirectLink) (*models.RedirectLink, error) {
						link.ID = "new-id"
						return link, nil
					},
				}
				svc := newTestService(t, redirectRepo, &mockGroupRepo{}, &mockInstanceRepo{}, &mockWAProvider{})
				link, err := svc.CreateRedirectLink(ctx, "user1", &models.RedirectLinkCreateRequest{
					Slug:         "slug-" + strategy,
					Name:         "Test " + strategy,
					Strategy:     strategy,
					TargetGroups: []models.GroupPriority{{GroupID: "g1", IsActive: true}},
					IsActive:     true,
				})
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if link.ID == "" {
					t.Error("expected non-empty ID")
				}
				if link.Strategy != strategy {
					t.Errorf("strategy = %q, want %q", link.Strategy, strategy)
				}
			})
		}
	})

	t.Run("success", func(t *testing.T) {
		redirectRepo := &mockRedirectLinkRepo{
			CheckSlugExistsFn: func(_ context.Context, _ string, _ string) (bool, error) {
				return false, nil
			},
			CreateFn: func(_ context.Context, link *models.RedirectLink) (*models.RedirectLink, error) {
				link.ID = "created-id"
				return link, nil
			},
		}
		svc := newTestService(t, redirectRepo, &mockGroupRepo{}, &mockInstanceRepo{}, &mockWAProvider{})
		link, err := svc.CreateRedirectLink(ctx, "user1", &models.RedirectLinkCreateRequest{
			Slug:     "new-slug",
			Name:     "New Link",
			Strategy: models.RedirectStrategyRoundRobin,
			TargetGroups: []models.GroupPriority{
				{GroupID: "g1", IsActive: true},
			},
			IsActive: true,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if link.ID != "created-id" {
			t.Errorf("ID = %q, want %q", link.ID, "created-id")
		}
		if link.UserID != "user1" {
			t.Errorf("userID = %q, want %q", link.UserID, "user1")
		}
	})

	t.Run("check slug error propagated", func(t *testing.T) {
		redirectRepo := &mockRedirectLinkRepo{
			CheckSlugExistsFn: func(_ context.Context, _ string, _ string) (bool, error) {
				return false, fmt.Errorf("db error")
			},
		}
		svc := newTestService(t, redirectRepo, &mockGroupRepo{}, &mockInstanceRepo{}, &mockWAProvider{})
		_, err := svc.CreateRedirectLink(ctx, "user1", &models.RedirectLinkCreateRequest{
			Slug:     "new-slug",
			Name:     "Test",
			Strategy: models.RedirectStrategyRoundRobin,
		})
		if err == nil {
			t.Fatal("expected error on slug check failure")
		}
	})
}

// ---------------------------------------------------------------------------
// Tests: UpdateRedirectLink
// ---------------------------------------------------------------------------

func TestUpdateRedirectLink(t *testing.T) {
	ctx := context.Background()

	t.Run("link not found", func(t *testing.T) {
		redirectRepo := &mockRedirectLinkRepo{
			GetByIDFn: func(_ context.Context, _ string) (*models.RedirectLink, error) {
				return nil, nil
			},
		}
		svc := newTestService(t, redirectRepo, &mockGroupRepo{}, &mockInstanceRepo{}, &mockWAProvider{})
		err := svc.UpdateRedirectLink(ctx, "link-1", "user-1", &models.RedirectLinkUpdateRequest{})
		if err == nil {
			t.Fatal("expected error for not found link")
		}
	})

	t.Run("unauthorized (different userID)", func(t *testing.T) {
		redirectRepo := &mockRedirectLinkRepo{
			GetByIDFn: func(_ context.Context, _ string) (*models.RedirectLink, error) {
				return &models.RedirectLink{ID: "link-1", UserID: "owner-user"}, nil
			},
		}
		svc := newTestService(t, redirectRepo, &mockGroupRepo{}, &mockInstanceRepo{}, &mockWAProvider{})
		err := svc.UpdateRedirectLink(ctx, "link-1", "other-user", &models.RedirectLinkUpdateRequest{})
		if err == nil {
			t.Fatal("expected error for unauthorized update")
		}
		if err.Error() != "unauthorized" {
			t.Errorf("error = %q, want %q", err.Error(), "unauthorized")
		}
	})

	t.Run("slug collision", func(t *testing.T) {
		redirectRepo := &mockRedirectLinkRepo{
			GetByIDFn: func(_ context.Context, _ string) (*models.RedirectLink, error) {
				return &models.RedirectLink{ID: "link-1", UserID: "user-1"}, nil
			},
			CheckSlugExistsFn: func(_ context.Context, _ string, _ string) (bool, error) {
				return true, nil
			},
		}
		svc := newTestService(t, redirectRepo, &mockGroupRepo{}, &mockInstanceRepo{}, &mockWAProvider{})
		newSlug := "taken-slug"
		err := svc.UpdateRedirectLink(ctx, "link-1", "user-1", &models.RedirectLinkUpdateRequest{
			Slug: &newSlug,
		})
		if err == nil {
			t.Fatal("expected error for slug collision")
		}
	})

	t.Run("invalid strategy", func(t *testing.T) {
		redirectRepo := &mockRedirectLinkRepo{
			GetByIDFn: func(_ context.Context, _ string) (*models.RedirectLink, error) {
				return &models.RedirectLink{ID: "link-1", UserID: "user-1"}, nil
			},
		}
		svc := newTestService(t, redirectRepo, &mockGroupRepo{}, &mockInstanceRepo{}, &mockWAProvider{})
		badStrategy := "bad_strategy"
		err := svc.UpdateRedirectLink(ctx, "link-1", "user-1", &models.RedirectLinkUpdateRequest{
			Strategy: &badStrategy,
		})
		if err == nil {
			t.Fatal("expected error for invalid strategy")
		}
	})

	t.Run("no updates (empty req)", func(t *testing.T) {
		redirectRepo := &mockRedirectLinkRepo{
			GetByIDFn: func(_ context.Context, _ string) (*models.RedirectLink, error) {
				return &models.RedirectLink{ID: "link-1", UserID: "user-1"}, nil
			},
		}
		svc := newTestService(t, redirectRepo, &mockGroupRepo{}, &mockInstanceRepo{}, &mockWAProvider{})
		err := svc.UpdateRedirectLink(ctx, "link-1", "user-1", &models.RedirectLinkUpdateRequest{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("partial updates", func(t *testing.T) {
		var capturedUpdates map[string]interface{}
		redirectRepo := &mockRedirectLinkRepo{
			GetByIDFn: func(_ context.Context, _ string) (*models.RedirectLink, error) {
				return &models.RedirectLink{ID: "link-1", UserID: "user-1"}, nil
			},
			UpdateFn: func(_ context.Context, _ string, updates map[string]interface{}) error {
				capturedUpdates = updates
				return nil
			},
		}
		svc := newTestService(t, redirectRepo, &mockGroupRepo{}, &mockInstanceRepo{}, &mockWAProvider{})
		newName := "Updated Name"
		isActive := false
		err := svc.UpdateRedirectLink(ctx, "link-1", "user-1", &models.RedirectLinkUpdateRequest{
			Name:     &newName,
			IsActive: &isActive,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if capturedUpdates["name"] != "Updated Name" {
			t.Errorf("name = %v, want %q", capturedUpdates["name"], "Updated Name")
		}
		if capturedUpdates["is_active"] != false {
			t.Errorf("is_active = %v, want false", capturedUpdates["is_active"])
		}
		if _, exists := capturedUpdates["slug"]; exists {
			t.Error("slug should not be in updates")
		}
	})

	t.Run("success full update", func(t *testing.T) {
		var capturedUpdates map[string]interface{}
		redirectRepo := &mockRedirectLinkRepo{
			GetByIDFn: func(_ context.Context, _ string) (*models.RedirectLink, error) {
				return &models.RedirectLink{ID: "link-1", UserID: "user-1"}, nil
			},
			CheckSlugExistsFn: func(_ context.Context, _ string, _ string) (bool, error) {
				return false, nil
			},
			UpdateFn: func(_ context.Context, _ string, updates map[string]interface{}) error {
				capturedUpdates = updates
				return nil
			},
		}
		svc := newTestService(t, redirectRepo, &mockGroupRepo{}, &mockInstanceRepo{}, &mockWAProvider{})
		newSlug := "new-slug"
		newName := "New Name"
		newStrategy := models.RedirectStrategyClickLimit
		newGroups := []models.GroupPriority{{GroupID: "g2", IsActive: true}}
		isActive := true
		err := svc.UpdateRedirectLink(ctx, "link-1", "user-1", &models.RedirectLinkUpdateRequest{
			Slug:         &newSlug,
			Name:         &newName,
			Strategy:     &newStrategy,
			TargetGroups: &newGroups,
			IsActive:     &isActive,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if capturedUpdates["slug"] != "new-slug" {
			t.Errorf("slug = %v, want %q", capturedUpdates["slug"], "new-slug")
		}
		if capturedUpdates["strategy"] != models.RedirectStrategyClickLimit {
			t.Errorf("strategy = %v, want %q", capturedUpdates["strategy"], models.RedirectStrategyClickLimit)
		}
	})

	t.Run("get link error propagated", func(t *testing.T) {
		redirectRepo := &mockRedirectLinkRepo{
			GetByIDFn: func(_ context.Context, _ string) (*models.RedirectLink, error) {
				return nil, fmt.Errorf("db error")
			},
		}
		svc := newTestService(t, redirectRepo, &mockGroupRepo{}, &mockInstanceRepo{}, &mockWAProvider{})
		err := svc.UpdateRedirectLink(ctx, "link-1", "user-1", &models.RedirectLinkUpdateRequest{})
		if err == nil {
			t.Fatal("expected error on get failure")
		}
	})
}

// ---------------------------------------------------------------------------
// Tests: DeleteRedirectLink
// ---------------------------------------------------------------------------

func TestDeleteRedirectLink(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		redirectRepo := &mockRedirectLinkRepo{
			DeleteFn: func(_ context.Context, _, _ string) error {
				return nil
			},
		}
		svc := newTestService(t, redirectRepo, &mockGroupRepo{}, &mockInstanceRepo{}, &mockWAProvider{})
		err := svc.DeleteRedirectLink(ctx, "link-1", "user-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("error", func(t *testing.T) {
		redirectRepo := &mockRedirectLinkRepo{
			DeleteFn: func(_ context.Context, _, _ string) error {
				return fmt.Errorf("delete failed")
			},
		}
		svc := newTestService(t, redirectRepo, &mockGroupRepo{}, &mockInstanceRepo{}, &mockWAProvider{})
		err := svc.DeleteRedirectLink(ctx, "link-1", "user-1")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

// ---------------------------------------------------------------------------
// Tests: CheckSlugAvailability
// ---------------------------------------------------------------------------

func TestCheckSlugAvailability(t *testing.T) {
	ctx := context.Background()

	t.Run("available (slug does not exist)", func(t *testing.T) {
		redirectRepo := &mockRedirectLinkRepo{
			CheckSlugExistsFn: func(_ context.Context, _ string, _ string) (bool, error) {
				return false, nil
			},
		}
		svc := newTestService(t, redirectRepo, &mockGroupRepo{}, &mockInstanceRepo{}, &mockWAProvider{})
		available, err := svc.CheckSlugAvailability(ctx, "new-slug", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !available {
			t.Error("expected slug to be available")
		}
	})

	t.Run("not available (slug exists)", func(t *testing.T) {
		redirectRepo := &mockRedirectLinkRepo{
			CheckSlugExistsFn: func(_ context.Context, _ string, _ string) (bool, error) {
				return true, nil
			},
		}
		svc := newTestService(t, redirectRepo, &mockGroupRepo{}, &mockInstanceRepo{}, &mockWAProvider{})
		available, err := svc.CheckSlugAvailability(ctx, "taken-slug", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if available {
			t.Error("expected slug to be not available")
		}
	})

	t.Run("error", func(t *testing.T) {
		redirectRepo := &mockRedirectLinkRepo{
			CheckSlugExistsFn: func(_ context.Context, _ string, _ string) (bool, error) {
				return false, fmt.Errorf("db error")
			},
		}
		svc := newTestService(t, redirectRepo, &mockGroupRepo{}, &mockInstanceRepo{}, &mockWAProvider{})
		_, err := svc.CheckSlugAvailability(ctx, "slug", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("exclude ID passed to repo", func(t *testing.T) {
		var receivedExcludeID string
		redirectRepo := &mockRedirectLinkRepo{
			CheckSlugExistsFn: func(_ context.Context, _ string, excludeID string) (bool, error) {
				receivedExcludeID = excludeID
				return false, nil
			},
		}
		svc := newTestService(t, redirectRepo, &mockGroupRepo{}, &mockInstanceRepo{}, &mockWAProvider{})
		_, err := svc.CheckSlugAvailability(ctx, "slug", "exclude-123")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if receivedExcludeID != "exclude-123" {
			t.Errorf("excludeID = %q, want %q", receivedExcludeID, "exclude-123")
		}
	})
}

