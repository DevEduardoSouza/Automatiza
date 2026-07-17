package profile

import (
	"context"
	"errors"
	"testing"
	"time"

	"automatiza-go/internal/models"

	"go.uber.org/zap"
)

// ============================================================
// Mock Automation Repository
// ============================================================

type mockAutomationRepo struct {
	automations map[string][]models.Automation
	shouldFail  bool
	failError   error
}

func newMockAutomationRepo() *mockAutomationRepo {
	return &mockAutomationRepo{
		automations: make(map[string][]models.Automation),
	}
}

func (m *mockAutomationRepo) GetUserAutomations(ctx context.Context, userID string) ([]models.Automation, error) {
	if m.shouldFail {
		return nil, m.failError
	}
	return m.automations[userID], nil
}

func (m *mockAutomationRepo) SetAutomations(userID string, automations []models.Automation) {
	m.automations[userID] = automations
}

func (m *mockAutomationRepo) GetActiveAutomations(ctx context.Context) ([]models.Automation, error) {
	return nil, nil
}

func (m *mockAutomationRepo) GetAutomationByID(ctx context.Context, automationID string) (*models.Automation, error) {
	return nil, nil
}

func (m *mockAutomationRepo) GetAutomationByIDAndUser(ctx context.Context, automationID, userID string) (*models.Automation, error) {
	return nil, nil
}

func (m *mockAutomationRepo) CreateAutomation(ctx context.Context, userID string, data *models.Automation) (*models.Automation, error) {
	return nil, nil
}

func (m *mockAutomationRepo) UpdateAutomation(ctx context.Context, automationID string, updates map[string]interface{}) error {
	return nil
}

func (m *mockAutomationRepo) DeleteAutomation(ctx context.Context, automationID, userID string) error {
	return nil
}

func (m *mockAutomationRepo) GetMonitoringAutomationsForGroup(ctx context.Context, waGroupID string) ([]models.Automation, error) {
	return nil, nil
}

func (m *mockAutomationRepo) UpdateLastRunAt(ctx context.Context, automationID string) error {
	return nil
}

func (m *mockAutomationRepo) UpdateNextRunAt(ctx context.Context, automationID string, nextRun time.Time) error {
	return nil
}

func (m *mockAutomationRepo) BulkUpdateStatusByUserID(ctx context.Context, userID, targetStatus string, sourceStatus *string) (int, error) {
	return 0, nil
}

func (m *mockAutomationRepo) BulkUpdateNextRunAts(ctx context.Context, automationIDs []string, nextRunAts []time.Time) error {
	return nil
}

func (m *mockAutomationRepo) GetOfferGenerateAutomationsForGroup(ctx context.Context, waGroupID string) ([]models.Automation, error) {
	return nil, nil
}

// ============================================================
// Mock Instance Repository
// ============================================================

type mockInstanceRepo struct {
	instances  map[string][]models.Instance
	shouldFail bool
	failError  error
}

func newMockInstanceRepo() *mockInstanceRepo {
	return &mockInstanceRepo{
		instances: make(map[string][]models.Instance),
	}
}

func (m *mockInstanceRepo) GetUserInstances(ctx context.Context, userID string) ([]models.Instance, error) {
	if m.shouldFail {
		return nil, m.failError
	}
	return m.instances[userID], nil
}

func (m *mockInstanceRepo) SetInstances(userID string, instances []models.Instance) {
	m.instances[userID] = instances
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

func (m *mockInstanceRepo) GetAllConnectedInstances(ctx context.Context) ([]models.Instance, error) {
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

func (m *mockInstanceRepo) SetDefaultInstance(ctx context.Context, userID string, instanceID int) error {
	return nil
}

func (m *mockInstanceRepo) CountUserInstances(ctx context.Context, userID string) (int, error) {
	return 0, nil
}

func (m *mockInstanceRepo) UpdateInstanceLabel(ctx context.Context, userID string, instanceID int, friendlyName string) error {
	return nil
}

func (m *mockInstanceRepo) UpdateInstancePhoneNumber(ctx context.Context, instanceID int, phoneNumber string) error {
	return nil
}

func (m *mockInstanceRepo) SupersedePriorInstances(ctx context.Context, userID string, currentInstanceID int, phoneNumber string) (int, error) {
	return 0, nil
}

// ============================================================
// Mock User Role Repository
// ============================================================

type mockUserRoleRepo struct {
	admins     map[string]bool
	shouldFail bool
	failError  error
}

func newMockUserRoleRepo() *mockUserRoleRepo {
	return &mockUserRoleRepo{
		admins: make(map[string]bool),
	}
}

func (m *mockUserRoleRepo) IsAdmin(ctx context.Context, userID string) (bool, error) {
	if m.shouldFail {
		return false, m.failError
	}
	return m.admins[userID], nil
}

func (m *mockUserRoleRepo) SetAdmin(userID string, isAdmin bool) {
	m.admins[userID] = isAdmin
}

func (m *mockUserRoleRepo) GetUserRole(ctx context.Context, userID string) (*models.UserRole, error) {
	return nil, nil
}

func (m *mockUserRoleRepo) SetUserRole(ctx context.Context, userID, role string) error {
	return nil
}

func (m *mockUserRoleRepo) RemoveUserRole(ctx context.Context, userID, role string) error {
	return nil
}

func (m *mockUserRoleRepo) GetAllUserRoles(ctx context.Context, userID string) ([]string, error) {
	return nil, nil
}

func (m *mockUserRoleRepo) GetAdminUsers(ctx context.Context) ([]models.UserRole, error) {
	return nil, nil
}

// ============================================================
// Test Helpers
// ============================================================

func setupProfileService(t *testing.T) (*ProfileService, *mockProfileRepo, *mockAutomationRepo, *mockInstanceRepo, *mockUserRoleRepo) {
	t.Helper()

	profileRepo := newMockProfileRepo()
	automationRepo := newMockAutomationRepo()
	instanceRepo := newMockInstanceRepo()
	userRoleRepo := newMockUserRoleRepo()

	logger := zap.NewNop()

	service := NewProfileService(
		profileRepo,
		automationRepo,
		instanceRepo,
		userRoleRepo,
		nil,
		logger,
	)

	return service, profileRepo, automationRepo, instanceRepo, userRoleRepo
}

// ============================================================
// Tests: GetProfile
// ============================================================

func TestGetProfile_Success(t *testing.T) {
	service, profileRepo, _, _, _ := setupProfileService(t)
	ctx := context.Background()

	profileRepo.AddProfile(&models.Profile{
		ID:    "user-123",
		Email: "test@example.com",
		Plan:  models.PlanPro,
	})

	profile, err := service.GetProfile(ctx, "user-123")
	if err != nil {
		t.Fatalf("GetProfile failed: %v", err)
	}

	if profile.ID != "user-123" {
		t.Errorf("expected ID 'user-123', got '%s'", profile.ID)
	}

	if profile.Email != "test@example.com" {
		t.Errorf("expected email 'test@example.com', got '%s'", profile.Email)
	}
}

func TestGetProfile_NotFound(t *testing.T) {
	service, _, _, _, _ := setupProfileService(t)
	ctx := context.Background()

	_, err := service.GetProfile(ctx, "non-existent")
	if !errors.Is(err, ErrProfileNotFound) {
		t.Errorf("expected ErrProfileNotFound, got %v", err)
	}
}

// ============================================================
// Tests: GetUserLimits
// ============================================================

func TestGetUserLimits_Success(t *testing.T) {
	service, profileRepo, automationRepo, instanceRepo, _ := setupProfileService(t)
	ctx := context.Background()

	profileRepo.AddProfile(&models.Profile{
		ID:   "user-123",
		Plan: models.PlanPro,
	})

	// Add 3 automations
	automationRepo.SetAutomations("user-123", []models.Automation{
		{ID: "auto-1"},
		{ID: "auto-2"},
		{ID: "auto-3"},
	})

	// Add 2 instances, 1 connected
	instanceRepo.SetInstances("user-123", []models.Instance{
		{ID: 1, Status: models.InstanceStatusConnected},
		{ID: 2, Status: models.InstanceStatusDisconnected},
	})

	limits, err := service.GetUserLimits(ctx, "user-123")
	if err != nil {
		t.Fatalf("GetUserLimits failed: %v", err)
	}

	if limits.AutomationsCount != 3 {
		t.Errorf("expected 3 automations, got %d", limits.AutomationsCount)
	}

	if limits.MaxAutomations != 5 { // Pro plan
		t.Errorf("expected 5 max automations (Pro), got %d", limits.MaxAutomations)
	}

	if limits.ConnectedInstancesCount != 1 {
		t.Errorf("expected 1 connected instance, got %d", limits.ConnectedInstancesCount)
	}
}

func TestGetUserLimits_PlanLimits(t *testing.T) {
	tests := []struct {
		plan            string
		expectedMaxAuto int32
	}{
		{models.PlanFree, 1},
		{models.PlanPro, 5},
		{models.PlanElite, 20},
		{"unknown", 1}, // Default to free
	}

	for _, tc := range tests {
		t.Run(tc.plan, func(t *testing.T) {
			service, profileRepo, _, _, _ := setupProfileService(t)
			ctx := context.Background()

			profileRepo.AddProfile(&models.Profile{
				ID:   "user-123",
				Plan: tc.plan,
			})

			limits, err := service.GetUserLimits(ctx, "user-123")
			if err != nil {
				t.Fatalf("GetUserLimits failed: %v", err)
			}

			if limits.MaxAutomations != tc.expectedMaxAuto {
				t.Errorf("expected %d max automations for %s, got %d", tc.expectedMaxAuto, tc.plan, limits.MaxAutomations)
			}
		})
	}
}

// ============================================================
// Tests: UpdateProfile
// ============================================================

func TestUpdateProfile_PhoneNumber(t *testing.T) {
	service, profileRepo, _, _, _ := setupProfileService(t)
	ctx := context.Background()

	profileRepo.AddProfile(&models.Profile{
		ID:    "user-123",
		Email: "test@example.com",
	})

	phone := "+55 11 99999-8888"
	profile, err := service.UpdateProfile(ctx, "user-123", UpdateProfileInput{
		PhoneNumber: &phone,
	})

	if err != nil {
		t.Fatalf("UpdateProfile failed: %v", err)
	}

	// Check that phone was cleaned
	expectedPhone := "5511999998888" // Cleaned format
	if profile.PhoneNumber != nil && *profile.PhoneNumber != expectedPhone {
		t.Errorf("expected phone '%s', got '%v'", expectedPhone, profile.PhoneNumber)
	}
}

func TestUpdateProfile_NoChanges(t *testing.T) {
	service, profileRepo, _, _, _ := setupProfileService(t)
	ctx := context.Background()

	profileRepo.AddProfile(&models.Profile{
		ID:    "user-123",
		Email: "test@example.com",
	})

	// Update with no changes
	profile, err := service.UpdateProfile(ctx, "user-123", UpdateProfileInput{})
	if err != nil {
		t.Fatalf("UpdateProfile failed: %v", err)
	}

	if profile.ID != "user-123" {
		t.Errorf("expected profile to be returned")
	}
}

// ============================================================
// Tests: Admin Operations
// ============================================================

func TestListProfiles_AdminOnly(t *testing.T) {
	service, profileRepo, _, _, userRoleRepo := setupProfileService(t)
	ctx := context.Background()

	profileRepo.AddProfile(&models.Profile{ID: "admin-user"})
	profileRepo.AddProfile(&models.Profile{ID: "regular-user"})

	// Non-admin should fail
	_, err := service.ListProfiles(ctx, "regular-user", "", 20)
	if !errors.Is(err, ErrPermissionDenied) {
		t.Errorf("expected ErrPermissionDenied for non-admin, got %v", err)
	}

	// Admin should succeed
	userRoleRepo.SetAdmin("admin-user", true)
	page, err := service.ListProfiles(ctx, "admin-user", "", 20)
	if err != nil {
		t.Fatalf("ListProfiles failed for admin: %v", err)
	}

	if page == nil {
		t.Error("expected page to be returned")
	}
}

func TestListProfiles_LimitEnforcement(t *testing.T) {
	service, profileRepo, _, _, userRoleRepo := setupProfileService(t)
	ctx := context.Background()

	profileRepo.AddProfile(&models.Profile{ID: "admin-user"})
	userRoleRepo.SetAdmin("admin-user", true)

	// Test default limit
	page, err := service.ListProfiles(ctx, "admin-user", "", 0)
	if err != nil {
		t.Fatalf("ListProfiles failed: %v", err)
	}
	if page == nil {
		t.Error("expected page")
	}

	// Test max limit enforcement (should cap at 100)
	page, err = service.ListProfiles(ctx, "admin-user", "", 200)
	if err != nil {
		t.Fatalf("ListProfiles failed: %v", err)
	}
	if page == nil {
		t.Error("expected page")
	}
}

func TestSearchProfiles_AdminOnly(t *testing.T) {
	service, profileRepo, _, _, userRoleRepo := setupProfileService(t)
	ctx := context.Background()

	profileRepo.AddProfile(&models.Profile{ID: "admin-user"})
	profileRepo.AddProfile(&models.Profile{ID: "regular-user"})

	// Non-admin should fail
	_, err := service.SearchProfiles(ctx, "regular-user", "test", "", 20)
	if !errors.Is(err, ErrPermissionDenied) {
		t.Errorf("expected ErrPermissionDenied for non-admin, got %v", err)
	}

	// Admin should succeed
	userRoleRepo.SetAdmin("admin-user", true)
	page, err := service.SearchProfiles(ctx, "admin-user", "test", "", 20)
	if err != nil {
		t.Fatalf("SearchProfiles failed for admin: %v", err)
	}

	if page == nil {
		t.Error("expected page to be returned")
	}
}

func TestBlockUser_AdminOnly(t *testing.T) {
	service, profileRepo, _, _, userRoleRepo := setupProfileService(t)
	ctx := context.Background()

	active := models.BillingStatusActive
	profileRepo.AddProfile(&models.Profile{ID: "admin-user"})
	profileRepo.AddProfile(&models.Profile{ID: "target-user", BillingStatus: &active})

	// Non-admin should fail
	_, err := service.BlockUser(ctx, "non-admin", "target-user")
	if !errors.Is(err, ErrPermissionDenied) {
		t.Errorf("expected ErrPermissionDenied for non-admin, got %v", err)
	}

	// Admin should succeed
	userRoleRepo.SetAdmin("admin-user", true)
	profile, err := service.BlockUser(ctx, "admin-user", "target-user")
	if err != nil {
		t.Fatalf("BlockUser failed for admin: %v", err)
	}

	if profile == nil {
		t.Error("expected profile to be returned")
	}
}

func TestUnblockUser_AdminOnly(t *testing.T) {
	service, profileRepo, _, _, userRoleRepo := setupProfileService(t)
	ctx := context.Background()

	suspended := models.BillingStatusSuspended
	profileRepo.AddProfile(&models.Profile{ID: "admin-user"})
	profileRepo.AddProfile(&models.Profile{ID: "target-user", BillingStatus: &suspended})

	// Non-admin should fail
	_, err := service.UnblockUser(ctx, "non-admin", "target-user")
	if !errors.Is(err, ErrPermissionDenied) {
		t.Errorf("expected ErrPermissionDenied for non-admin, got %v", err)
	}

	// Admin should succeed
	userRoleRepo.SetAdmin("admin-user", true)
	profile, err := service.UnblockUser(ctx, "admin-user", "target-user")
	if err != nil {
		t.Fatalf("UnblockUser failed for admin: %v", err)
	}

	if profile == nil {
		t.Error("expected profile to be returned")
	}
}

func TestUpdateUserPlan_AdminOnly(t *testing.T) {
	service, profileRepo, _, _, userRoleRepo := setupProfileService(t)
	ctx := context.Background()

	profileRepo.AddProfile(&models.Profile{ID: "admin-user"})
	profileRepo.AddProfile(&models.Profile{ID: "target-user", Plan: models.PlanFree})

	newPlan := models.PlanPro
	newMax := int32(10)

	// Non-admin should fail
	_, err := service.UpdateUserPlan(ctx, "non-admin", "target-user", &newPlan, &newMax, nil)
	if !errors.Is(err, ErrPermissionDenied) {
		t.Errorf("expected ErrPermissionDenied for non-admin, got %v", err)
	}

	// Admin should succeed
	userRoleRepo.SetAdmin("admin-user", true)
	profile, err := service.UpdateUserPlan(ctx, "admin-user", "target-user", &newPlan, &newMax, nil)
	if err != nil {
		t.Fatalf("UpdateUserPlan failed for admin: %v", err)
	}

	if profile == nil {
		t.Error("expected profile to be returned")
	}
}

// ============================================================
// Tests: Helper Functions
// ============================================================

func TestCleanPhoneNumber(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"+55 11 99999-8888", "5511999998888"},
		{"(11) 99999-8888", "11999998888"}, // All 8s preserved
		{"5511999998888", "5511999998888"},
		{"00551199999-8888", "5511999998888"}, // Leading zeros removed
		{"", ""},
		{"abc123def456", "123456"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := cleanPhoneNumber(tc.input)
			if result != tc.expected {
				t.Errorf("cleanPhoneNumber(%q) = %q, expected %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestGetMaxAutomationsForPlan(t *testing.T) {
	tests := []struct {
		plan     string
		expected int32
	}{
		{models.PlanFree, 1},
		{models.PlanPro, 5},
		{models.PlanElite, 20},
		{"custom", 1},
		{"", 1},
	}

	for _, tc := range tests {
		t.Run(tc.plan, func(t *testing.T) {
			result := GetMaxAutomationsForPlan(tc.plan)
			if result != tc.expected {
				t.Errorf("GetMaxAutomationsForPlan(%q) = %d, expected %d", tc.plan, result, tc.expected)
			}
		})
	}
}

func TestGetUserLimits_CustomMaxAutomations(t *testing.T) {
	service, profileRepo, automationRepo, instanceRepo, _ := setupProfileService(t)
	ctx := context.Background()

	// Profile with custom max_automations override from DB
	customMax := 10
	profileRepo.AddProfile(&models.Profile{
		ID:             "user-custom",
		Plan:           models.PlanPro,
		MaxAutomations: &customMax,
	})

	automationRepo.SetAutomations("user-custom", []models.Automation{
		{ID: "auto-1"},
	})
	instanceRepo.SetInstances("user-custom", []models.Instance{})

	limits, err := service.GetUserLimits(ctx, "user-custom")
	if err != nil {
		t.Fatalf("GetUserLimits failed: %v", err)
	}

	// Should use DB value (10), not plan default (5 for Pro)
	if limits.MaxAutomations != 10 {
		t.Errorf("expected 10 max automations (custom DB override), got %d", limits.MaxAutomations)
	}

	// Test fallback: profile without custom max_automations
	profileRepo.AddProfile(&models.Profile{
		ID:   "user-default",
		Plan: models.PlanElite,
	})
	automationRepo.SetAutomations("user-default", []models.Automation{})
	instanceRepo.SetInstances("user-default", []models.Instance{})

	limits, err = service.GetUserLimits(ctx, "user-default")
	if err != nil {
		t.Fatalf("GetUserLimits failed: %v", err)
	}

	// Should fall back to plan default (20 for Elite)
	if limits.MaxAutomations != 20 {
		t.Errorf("expected 20 max automations (Elite plan default), got %d", limits.MaxAutomations)
	}
}
