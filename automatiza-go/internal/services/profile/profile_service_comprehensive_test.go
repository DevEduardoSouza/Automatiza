package profile

import (
	"context"
	"errors"
	"testing"

	"automatiza-go/internal/models"

	"go.uber.org/zap"
)

// ============================================================
// Tests: GetProfile - Error Paths
// ============================================================

func TestGetProfile_RepoError(t *testing.T) {
	t.Parallel()
	service, profileRepo, _, _, _ := setupEnhancedProfileService(t)
	ctx := context.Background()

	profileRepo.getProfileErr = errors.New("database connection failed")

	_, err := service.GetProfile(ctx, "user-123")
	if err == nil {
		t.Fatal("expected error from repo failure")
	}

	if !errors.Is(err, profileRepo.getProfileErr) {
		t.Errorf("expected wrapped database error, got: %v", err)
	}
}

func TestGetProfile_ReturnsCorrectProfile(t *testing.T) {
	t.Parallel()
	service, profileRepo, _, _, _ := setupEnhancedProfileService(t)
	ctx := context.Background()

	phone := "5511999998888"
	profileRepo.AddProfile(&models.Profile{
		ID:          "user-abc",
		Email:       "abc@test.com",
		Plan:        models.PlanElite,
		PhoneNumber: &phone,
	})

	profile, err := service.GetProfile(ctx, "user-abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if profile.Plan != models.PlanElite {
		t.Errorf("expected plan Elite, got %s", profile.Plan)
	}
	if profile.PhoneNumber == nil || *profile.PhoneNumber != phone {
		t.Errorf("expected phone %s, got %v", phone, profile.PhoneNumber)
	}
}

// ============================================================
// Tests: GetUserLimits - Error Paths
// ============================================================

func TestGetUserLimits_ProfileNotFound(t *testing.T) {
	t.Parallel()
	service, _, _, _, _ := setupEnhancedProfileService(t)
	ctx := context.Background()

	_, err := service.GetUserLimits(ctx, "non-existent")
	if !errors.Is(err, ErrProfileNotFound) {
		t.Errorf("expected ErrProfileNotFound, got %v", err)
	}
}

func TestGetUserLimits_AutomationRepoError(t *testing.T) {
	t.Parallel()
	service, profileRepo, automationRepo, _, _ := setupEnhancedProfileService(t)
	ctx := context.Background()

	profileRepo.AddProfile(&models.Profile{ID: "user-123", Plan: models.PlanPro})
	automationRepo.shouldFail = true
	automationRepo.failError = errors.New("automation db error")

	_, err := service.GetUserLimits(ctx, "user-123")
	if err == nil {
		t.Fatal("expected error from automation repo failure")
	}

	if !errors.Is(err, automationRepo.failError) {
		t.Errorf("expected wrapped automation error, got: %v", err)
	}
}

func TestGetUserLimits_InstanceRepoError(t *testing.T) {
	t.Parallel()
	service, profileRepo, _, instanceRepo, _ := setupEnhancedProfileService(t)
	ctx := context.Background()

	profileRepo.AddProfile(&models.Profile{ID: "user-123", Plan: models.PlanPro})
	instanceRepo.shouldFail = true
	instanceRepo.failError = errors.New("instance db error")

	_, err := service.GetUserLimits(ctx, "user-123")
	if err == nil {
		t.Fatal("expected error from instance repo failure")
	}

	if !errors.Is(err, instanceRepo.failError) {
		t.Errorf("expected wrapped instance error, got: %v", err)
	}
}

func TestGetUserLimits_ZeroAutomationsAndInstances(t *testing.T) {
	t.Parallel()
	service, profileRepo, _, _, _ := setupEnhancedProfileService(t)
	ctx := context.Background()

	profileRepo.AddProfile(&models.Profile{ID: "user-123", Plan: models.PlanFree})

	limits, err := service.GetUserLimits(ctx, "user-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if limits.AutomationsCount != 0 {
		t.Errorf("expected 0 automations, got %d", limits.AutomationsCount)
	}
	if limits.ConnectedInstancesCount != 0 {
		t.Errorf("expected 0 connected instances, got %d", limits.ConnectedInstancesCount)
	}
	if limits.MaxAutomations != 1 {
		t.Errorf("expected max 1 (free plan), got %d", limits.MaxAutomations)
	}
}

func TestGetUserLimits_AllInstancesConnected(t *testing.T) {
	t.Parallel()
	service, profileRepo, _, instanceRepo, _ := setupEnhancedProfileService(t)
	ctx := context.Background()

	profileRepo.AddProfile(&models.Profile{ID: "user-123", Plan: models.PlanPro})
	instanceRepo.SetInstances("user-123", []models.Instance{
		{ID: 1, Status: models.InstanceStatusConnected},
		{ID: 2, Status: models.InstanceStatusConnected},
		{ID: 3, Status: models.InstanceStatusConnected},
	})

	limits, err := service.GetUserLimits(ctx, "user-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if limits.ConnectedInstancesCount != 3 {
		t.Errorf("expected 3 connected instances, got %d", limits.ConnectedInstancesCount)
	}
}

func TestGetUserLimits_AllInstancesDisconnected(t *testing.T) {
	t.Parallel()
	service, profileRepo, _, instanceRepo, _ := setupEnhancedProfileService(t)
	ctx := context.Background()

	profileRepo.AddProfile(&models.Profile{ID: "user-123", Plan: models.PlanPro})
	instanceRepo.SetInstances("user-123", []models.Instance{
		{ID: 1, Status: models.InstanceStatusDisconnected},
		{ID: 2, Status: models.InstanceStatusDisconnected},
	})

	limits, err := service.GetUserLimits(ctx, "user-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if limits.ConnectedInstancesCount != 0 {
		t.Errorf("expected 0 connected instances, got %d", limits.ConnectedInstancesCount)
	}
}

func TestGetUserLimits_ProfileRepoError(t *testing.T) {
	t.Parallel()
	service, profileRepo, _, _, _ := setupEnhancedProfileService(t)
	ctx := context.Background()

	profileRepo.getProfileErr = errors.New("profile db error")

	_, err := service.GetUserLimits(ctx, "user-123")
	if err == nil {
		t.Fatal("expected error from profile repo failure")
	}
}

// ============================================================
// Tests: UpdateProfile - Error Paths
// ============================================================

func TestUpdateProfile_RepoUpdateError(t *testing.T) {
	t.Parallel()
	service, profileRepo, _, _, _ := setupEnhancedProfileService(t)
	ctx := context.Background()

	profileRepo.AddProfile(&models.Profile{ID: "user-123", Email: "test@example.com"})
	profileRepo.updateProfileErr = errors.New("update failed")

	phone := "+55 11 99999-8888"
	_, err := service.UpdateProfile(ctx, "user-123", UpdateProfileInput{PhoneNumber: &phone})
	if err == nil {
		t.Fatal("expected error from repo update failure")
	}

	if !errors.Is(err, profileRepo.updateProfileErr) {
		t.Errorf("expected wrapped update error, got: %v", err)
	}
}

func TestUpdateProfile_GetProfileErrorAfterUpdate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	phone := "+55 11 99999-8888"

	profileRepo := newEnhancedMockProfileRepo()
	automationRepo := newEnhancedMockAutomationRepo()
	instanceRepo := newEnhancedMockInstanceRepo()
	userRoleRepo := newEnhancedMockUserRoleRepo()
	logger := zap.NewNop()
	svc := NewProfileService(profileRepo, automationRepo, instanceRepo, userRoleRepo, nil, logger)

	// No profile exists -> no changes -> GetProfile returns not found
	_, err := svc.UpdateProfile(ctx, "non-existent", UpdateProfileInput{})
	if !errors.Is(err, ErrProfileNotFound) {
		t.Errorf("expected ErrProfileNotFound for no-change update on non-existent user, got: %v", err)
	}

	// Phone update on non-existent user (UpdateProfile succeeds but GetProfile after returns not found)
	_, err = svc.UpdateProfile(ctx, "non-existent-2", UpdateProfileInput{PhoneNumber: &phone})
	if !errors.Is(err, ErrProfileNotFound) {
		t.Errorf("expected ErrProfileNotFound after update on non-existent user, got: %v", err)
	}
}

func TestUpdateProfile_PhoneNumberCleaning(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"international_format", "+55 (11) 99999-8888", "5511999998888"},
		{"local_format", "(11) 99999-8888", "11999998888"},
		{"digits_only", "5511999998888", "5511999998888"},
		{"leading_zeros", "00551199999-8888", "5511999998888"},
		{"empty_string", "", ""},
		{"mixed_chars", "abc123def456", "123456"},
		{"only_special_chars", "+-()/ ", ""},
		{"single_digit", "5", "5"},
		{"all_zeros", "0000", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			service, profileRepo, _, _, _ := setupEnhancedProfileService(t)
			ctx := context.Background()

			profileRepo.AddProfile(&models.Profile{ID: "user-123", Email: "test@example.com"})

			phone := tc.input
			profile, err := service.UpdateProfile(ctx, "user-123", UpdateProfileInput{PhoneNumber: &phone})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if profile.PhoneNumber == nil {
				if tc.expected != "" {
					t.Errorf("expected phone %q, got nil", tc.expected)
				}
			} else if *profile.PhoneNumber != tc.expected {
				t.Errorf("expected phone %q, got %q", tc.expected, *profile.PhoneNumber)
			}
		})
	}
}

func TestUpdateProfile_VerifiesUpdateWasApplied(t *testing.T) {
	t.Parallel()
	service, profileRepo, _, _, _ := setupEnhancedProfileService(t)
	ctx := context.Background()

	profileRepo.AddProfile(&models.Profile{ID: "user-123", Email: "test@example.com"})

	phone := "+55 11 88888-7777"
	_, err := service.UpdateProfile(ctx, "user-123", UpdateProfileInput{PhoneNumber: &phone})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the update call was made with correct cleaned phone
	if len(profileRepo.updateProfileCalls) != 1 {
		t.Fatalf("expected 1 update call, got %d", len(profileRepo.updateProfileCalls))
	}

	call := profileRepo.updateProfileCalls[0]
	if call.UserID != "user-123" {
		t.Errorf("expected userID 'user-123', got %s", call.UserID)
	}
	if call.Updates["phone_number"] != "5511888887777" {
		t.Errorf("expected cleaned phone '5511888887777', got %v", call.Updates["phone_number"])
	}
}

// ============================================================
// Tests: ListProfiles - Error Paths & Edge Cases
// ============================================================

func TestListProfiles_AdminCheckError(t *testing.T) {
	t.Parallel()
	service, _, _, _, userRoleRepo := setupEnhancedProfileService(t)
	ctx := context.Background()

	userRoleRepo.shouldFail = true
	userRoleRepo.failError = errors.New("role check failed")

	_, err := service.ListProfiles(ctx, "admin-user", "", 20)
	if !errors.Is(err, ErrPermissionDenied) {
		t.Errorf("expected ErrPermissionDenied on role check error, got: %v", err)
	}
}

func TestListProfiles_RepoError(t *testing.T) {
	t.Parallel()
	service, profileRepo, _, _, userRoleRepo := setupEnhancedProfileService(t)
	ctx := context.Background()

	userRoleRepo.SetAdmin("admin-user", true)
	profileRepo.listProfilesErr = errors.New("list query failed")

	_, err := service.ListProfiles(ctx, "admin-user", "", 20)
	if err == nil {
		t.Fatal("expected error from repo failure")
	}

	if !errors.Is(err, profileRepo.listProfilesErr) {
		t.Errorf("expected wrapped list error, got: %v", err)
	}
}

func TestListProfiles_WithPagination(t *testing.T) {
	t.Parallel()
	service, profileRepo, _, _, userRoleRepo := setupEnhancedProfileService(t)
	ctx := context.Background()

	userRoleRepo.SetAdmin("admin-user", true)
	profileRepo.listProfilesData = []*models.Profile{
		{ID: "user-1", Email: "a@test.com"},
		{ID: "user-2", Email: "b@test.com"},
	}
	profileRepo.listNextCursor = "cursor-abc"

	page, err := service.ListProfiles(ctx, "admin-user", "", 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(page.Profiles) != 2 {
		t.Errorf("expected 2 profiles, got %d", len(page.Profiles))
	}
	if page.NextCursor != "cursor-abc" {
		t.Errorf("expected cursor 'cursor-abc', got %s", page.NextCursor)
	}
	if !page.HasMore {
		t.Error("expected HasMore to be true when cursor is present")
	}
}

func TestListProfiles_EmptyResults(t *testing.T) {
	t.Parallel()
	service, _, _, _, userRoleRepo := setupEnhancedProfileService(t)
	ctx := context.Background()

	userRoleRepo.SetAdmin("admin-user", true)

	page, err := service.ListProfiles(ctx, "admin-user", "", 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if page.Profiles != nil && len(page.Profiles) != 0 {
		t.Errorf("expected empty profiles, got %d", len(page.Profiles))
	}
	if page.HasMore {
		t.Error("expected HasMore to be false for empty results")
	}
}

func TestListProfiles_LimitClamp(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		inputLimit  int
		expectNoErr bool
	}{
		{"negative_limit", -5, true},
		{"zero_limit", 0, true},
		{"normal_limit", 50, true},
		{"max_limit", 100, true},
		{"over_max_limit", 200, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			service, _, _, _, userRoleRepo := setupEnhancedProfileService(t)
			ctx := context.Background()

			userRoleRepo.SetAdmin("admin-user", true)

			page, err := service.ListProfiles(ctx, "admin-user", "", tc.inputLimit)
			if tc.expectNoErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if page == nil {
				t.Fatal("expected non-nil page")
			}
		})
	}
}

func TestListProfiles_WithCursor(t *testing.T) {
	t.Parallel()
	service, _, _, _, userRoleRepo := setupEnhancedProfileService(t)
	ctx := context.Background()

	userRoleRepo.SetAdmin("admin-user", true)

	// Should not error when cursor is provided
	page, err := service.ListProfiles(ctx, "admin-user", "existing-cursor", 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if page == nil {
		t.Fatal("expected non-nil page")
	}
}

// ============================================================
// Tests: SearchProfiles - Error Paths & Edge Cases
// ============================================================

func TestSearchProfiles_AdminCheckError(t *testing.T) {
	t.Parallel()
	service, _, _, _, userRoleRepo := setupEnhancedProfileService(t)
	ctx := context.Background()

	userRoleRepo.shouldFail = true
	userRoleRepo.failError = errors.New("role check failed")

	_, err := service.SearchProfiles(ctx, "admin-user", "query", "", 20)
	if !errors.Is(err, ErrPermissionDenied) {
		t.Errorf("expected ErrPermissionDenied on role check error, got: %v", err)
	}
}

func TestSearchProfiles_RepoError(t *testing.T) {
	t.Parallel()
	service, profileRepo, _, _, userRoleRepo := setupEnhancedProfileService(t)
	ctx := context.Background()

	userRoleRepo.SetAdmin("admin-user", true)
	profileRepo.searchProfilesErr = errors.New("search query failed")

	_, err := service.SearchProfiles(ctx, "admin-user", "test", "", 20)
	if err == nil {
		t.Fatal("expected error from repo failure")
	}

	if !errors.Is(err, profileRepo.searchProfilesErr) {
		t.Errorf("expected wrapped search error, got: %v", err)
	}
}

func TestSearchProfiles_LimitClamp(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		inputLimit int
	}{
		{"negative_limit", -5},
		{"zero_limit", 0},
		{"normal_limit", 50},
		{"max_limit", 100},
		{"over_max_limit", 200},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			service, _, _, _, userRoleRepo := setupEnhancedProfileService(t)
			ctx := context.Background()

			userRoleRepo.SetAdmin("admin-user", true)

			page, err := service.SearchProfiles(ctx, "admin-user", "query", "", tc.inputLimit)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if page == nil {
				t.Fatal("expected non-nil page")
			}
		})
	}
}

func TestSearchProfiles_WithPaginationAndCursor(t *testing.T) {
	t.Parallel()
	service, profileRepo, _, _, userRoleRepo := setupEnhancedProfileService(t)
	ctx := context.Background()

	userRoleRepo.SetAdmin("admin-user", true)
	profileRepo.searchProfilesData = []*models.Profile{
		{ID: "user-1", Email: "match@test.com"},
	}
	profileRepo.searchNextCursor = "next-cursor"

	page, err := service.SearchProfiles(ctx, "admin-user", "match", "prev-cursor", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(page.Profiles) != 1 {
		t.Errorf("expected 1 profile, got %d", len(page.Profiles))
	}
	if page.NextCursor != "next-cursor" {
		t.Errorf("expected cursor 'next-cursor', got %s", page.NextCursor)
	}
	if !page.HasMore {
		t.Error("expected HasMore=true")
	}
}

func TestSearchProfiles_EmptyQuery(t *testing.T) {
	t.Parallel()
	service, _, _, _, userRoleRepo := setupEnhancedProfileService(t)
	ctx := context.Background()

	userRoleRepo.SetAdmin("admin-user", true)

	page, err := service.SearchProfiles(ctx, "admin-user", "", "", 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if page == nil {
		t.Fatal("expected non-nil page")
	}
}

// ============================================================
// Tests: BlockUser - Error Paths
// ============================================================

func TestBlockUser_AdminCheckError(t *testing.T) {
	t.Parallel()
	service, _, _, _, userRoleRepo := setupEnhancedProfileService(t)
	ctx := context.Background()

	userRoleRepo.shouldFail = true
	userRoleRepo.failError = errors.New("role check error")

	_, err := service.BlockUser(ctx, "admin-user", "target-user")
	if !errors.Is(err, ErrPermissionDenied) {
		t.Errorf("expected ErrPermissionDenied on role check error, got: %v", err)
	}
}

func TestBlockUser_UpdateProfileError(t *testing.T) {
	t.Parallel()
	service, profileRepo, _, _, userRoleRepo := setupEnhancedProfileService(t)
	ctx := context.Background()

	userRoleRepo.SetAdmin("admin-user", true)
	profileRepo.AddProfile(&models.Profile{ID: "target-user"})
	profileRepo.updateProfileErr = errors.New("update failed")

	_, err := service.BlockUser(ctx, "admin-user", "target-user")
	if err == nil {
		t.Fatal("expected error from update failure")
	}
	if !errors.Is(err, profileRepo.updateProfileErr) {
		t.Errorf("expected wrapped update error, got: %v", err)
	}
}

func TestBlockUser_GetProfileErrorAfterBlock(t *testing.T) {
	t.Parallel()

	profileRepo := newEnhancedMockProfileRepo()
	automationRepo := newEnhancedMockAutomationRepo()
	instanceRepo := newEnhancedMockInstanceRepo()
	userRoleRepo := newEnhancedMockUserRoleRepo()
	logger := zap.NewNop()
	svc := NewProfileService(profileRepo, automationRepo, instanceRepo, userRoleRepo, nil, logger)

	userRoleRepo.SetAdmin("admin-user", true)
	// Don't add the target user profile - UpdateProfile succeeds but GetProfile returns nil
	profile, err := svc.BlockUser(ctx(), "admin-user", "target-user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// GetProfile returns nil for non-existent user
	if profile != nil {
		t.Error("expected nil profile for non-existent target user")
	}
}

func TestBlockUser_VerifiesBillingStatusSetToSuspended(t *testing.T) {
	t.Parallel()
	service, profileRepo, _, _, userRoleRepo := setupEnhancedProfileService(t)

	active := models.BillingStatusActive
	profileRepo.AddProfile(&models.Profile{ID: "target-user", BillingStatus: &active})
	userRoleRepo.SetAdmin("admin-user", true)

	profile, err := service.BlockUser(ctx(), "admin-user", "target-user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if profile.BillingStatus == nil || *profile.BillingStatus != models.BillingStatusSuspended {
		t.Errorf("expected billing status 'suspended', got %v", profile.BillingStatus)
	}
}

// ============================================================
// Tests: UnblockUser - Error Paths
// ============================================================

func TestUnblockUser_AdminCheckError(t *testing.T) {
	t.Parallel()
	service, _, _, _, userRoleRepo := setupEnhancedProfileService(t)

	userRoleRepo.shouldFail = true
	userRoleRepo.failError = errors.New("role check error")

	_, err := service.UnblockUser(ctx(), "admin-user", "target-user")
	if !errors.Is(err, ErrPermissionDenied) {
		t.Errorf("expected ErrPermissionDenied, got: %v", err)
	}
}

func TestUnblockUser_UpdateProfileError(t *testing.T) {
	t.Parallel()
	service, profileRepo, _, _, userRoleRepo := setupEnhancedProfileService(t)

	userRoleRepo.SetAdmin("admin-user", true)
	profileRepo.AddProfile(&models.Profile{ID: "target-user"})
	profileRepo.updateProfileErr = errors.New("update failed")

	_, err := service.UnblockUser(ctx(), "admin-user", "target-user")
	if err == nil {
		t.Fatal("expected error from update failure")
	}
	if !errors.Is(err, profileRepo.updateProfileErr) {
		t.Errorf("expected wrapped update error, got: %v", err)
	}
}

func TestUnblockUser_VerifiesBillingStatusSetToActive(t *testing.T) {
	t.Parallel()
	service, profileRepo, _, _, userRoleRepo := setupEnhancedProfileService(t)

	suspended := models.BillingStatusSuspended
	profileRepo.AddProfile(&models.Profile{ID: "target-user", BillingStatus: &suspended})
	userRoleRepo.SetAdmin("admin-user", true)

	profile, err := service.UnblockUser(ctx(), "admin-user", "target-user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if profile.BillingStatus == nil || *profile.BillingStatus != models.BillingStatusActive {
		t.Errorf("expected billing status 'active', got %v", profile.BillingStatus)
	}
}

// ============================================================
// Tests: UpdateUserPlan - Error Paths & Edge Cases
// ============================================================

func TestUpdateUserPlan_AdminCheckError(t *testing.T) {
	t.Parallel()
	service, _, _, _, userRoleRepo := setupEnhancedProfileService(t)

	userRoleRepo.shouldFail = true
	userRoleRepo.failError = errors.New("role check error")

	plan := models.PlanPro
	_, err := service.UpdateUserPlan(ctx(), "admin-user", "target-user", &plan, nil, nil)
	if !errors.Is(err, ErrPermissionDenied) {
		t.Errorf("expected ErrPermissionDenied, got: %v", err)
	}
}

func TestUpdateUserPlan_NoChanges(t *testing.T) {
	t.Parallel()
	service, profileRepo, _, _, userRoleRepo := setupEnhancedProfileService(t)

	profileRepo.AddProfile(&models.Profile{ID: "target-user", Plan: models.PlanFree})
	userRoleRepo.SetAdmin("admin-user", true)

	// All nil = no changes
	profile, err := service.UpdateUserPlan(ctx(), "admin-user", "target-user", nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if profile == nil {
		t.Fatal("expected profile to be returned")
	}
	if profile.Plan != models.PlanFree {
		t.Errorf("expected plan to remain 'free', got %s", profile.Plan)
	}
}

func TestUpdateUserPlan_OnlyPlanUpdate(t *testing.T) {
	t.Parallel()
	service, profileRepo, _, _, userRoleRepo := setupEnhancedProfileService(t)

	profileRepo.AddProfile(&models.Profile{ID: "target-user", Plan: models.PlanFree})
	userRoleRepo.SetAdmin("admin-user", true)

	plan := models.PlanPro
	profile, err := service.UpdateUserPlan(ctx(), "admin-user", "target-user", &plan, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if profile.Plan != models.PlanPro {
		t.Errorf("expected plan 'pro', got %s", profile.Plan)
	}

	// Verify update call
	if len(profileRepo.updateProfileCalls) != 1 {
		t.Fatalf("expected 1 update call, got %d", len(profileRepo.updateProfileCalls))
	}
	call := profileRepo.updateProfileCalls[0]
	if _, exists := call.Updates["plan"]; !exists {
		t.Error("expected 'plan' in updates")
	}
	if _, exists := call.Updates["max_automations"]; exists {
		t.Error("unexpected 'max_automations' in updates")
	}
}

func TestUpdateUserPlan_OnlyMaxAutomationsUpdate(t *testing.T) {
	t.Parallel()
	service, profileRepo, _, _, userRoleRepo := setupEnhancedProfileService(t)

	profileRepo.AddProfile(&models.Profile{ID: "target-user", Plan: models.PlanFree})
	userRoleRepo.SetAdmin("admin-user", true)

	maxAuto := int32(15)
	_, err := service.UpdateUserPlan(ctx(), "admin-user", "target-user", nil, &maxAuto, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(profileRepo.updateProfileCalls) != 1 {
		t.Fatalf("expected 1 update call, got %d", len(profileRepo.updateProfileCalls))
	}
	call := profileRepo.updateProfileCalls[0]
	if _, exists := call.Updates["max_automations"]; !exists {
		t.Error("expected 'max_automations' in updates")
	}
	if _, exists := call.Updates["plan"]; exists {
		t.Error("unexpected 'plan' in updates")
	}
}

func TestUpdateUserPlan_OnlyPlanAccessUntilUpdate(t *testing.T) {
	t.Parallel()
	service, profileRepo, _, _, userRoleRepo := setupEnhancedProfileService(t)

	profileRepo.AddProfile(&models.Profile{ID: "target-user", Plan: models.PlanFree})
	userRoleRepo.SetAdmin("admin-user", true)

	accessUntil := "2026-12-31T23:59:59Z"
	_, err := service.UpdateUserPlan(ctx(), "admin-user", "target-user", nil, nil, &accessUntil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(profileRepo.updateProfileCalls) != 1 {
		t.Fatalf("expected 1 update call, got %d", len(profileRepo.updateProfileCalls))
	}
	call := profileRepo.updateProfileCalls[0]
	if _, exists := call.Updates["plan_access_until"]; !exists {
		t.Error("expected 'plan_access_until' in updates")
	}
}

func TestUpdateUserPlan_AllFields(t *testing.T) {
	t.Parallel()
	service, profileRepo, _, _, userRoleRepo := setupEnhancedProfileService(t)

	profileRepo.AddProfile(&models.Profile{ID: "target-user", Plan: models.PlanFree})
	userRoleRepo.SetAdmin("admin-user", true)

	plan := models.PlanElite
	maxAuto := int32(50)
	accessUntil := "2026-12-31T23:59:59Z"

	_, err := service.UpdateUserPlan(ctx(), "admin-user", "target-user", &plan, &maxAuto, &accessUntil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(profileRepo.updateProfileCalls) != 1 {
		t.Fatalf("expected 1 update call, got %d", len(profileRepo.updateProfileCalls))
	}
	call := profileRepo.updateProfileCalls[0]
	if len(call.Updates) != 3 {
		t.Errorf("expected 3 updates, got %d", len(call.Updates))
	}
}

func TestUpdateUserPlan_RepoUpdateError(t *testing.T) {
	t.Parallel()
	service, profileRepo, _, _, userRoleRepo := setupEnhancedProfileService(t)

	profileRepo.AddProfile(&models.Profile{ID: "target-user", Plan: models.PlanFree})
	userRoleRepo.SetAdmin("admin-user", true)
	profileRepo.updateProfileErr = errors.New("update plan failed")

	plan := models.PlanPro
	_, err := service.UpdateUserPlan(ctx(), "admin-user", "target-user", &plan, nil, nil)
	if err == nil {
		t.Fatal("expected error from repo failure")
	}
	if !errors.Is(err, profileRepo.updateProfileErr) {
		t.Errorf("expected wrapped update error, got: %v", err)
	}
}

// ============================================================
// Tests: GetMaxAutomationsForPlan - Additional Edge Cases
// ============================================================

func TestGetMaxAutomationsForPlan_AllPlans(t *testing.T) {
	t.Parallel()

	tests := []struct {
		plan     string
		expected int32
	}{
		{models.PlanFree, 1},
		{models.PlanPro, 5},
		{models.PlanElite, 20},
		{"unknown_plan", 1},
		{"", 1},
		{"ELITE", 1}, // case-sensitive
		{"Premium", 1},
	}

	for _, tc := range tests {
		t.Run("plan_"+tc.plan, func(t *testing.T) {
			t.Parallel()
			result := GetMaxAutomationsForPlan(tc.plan)
			if result != tc.expected {
				t.Errorf("GetMaxAutomationsForPlan(%q) = %d, expected %d", tc.plan, result, tc.expected)
			}
		})
	}
}

// ============================================================
// Tests: cleanPhoneNumber - Additional Edge Cases
// ============================================================

func TestCleanPhoneNumber_AdditionalCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"whitespace_only", "   ", ""},
		{"unicode_digits", "１２３", ""}, // Full-width digits are NOT ASCII digits
		{"tab_separated", "55\t11\t9999", "55119999"},
		{"newline_separated", "55\n11", "5511"},
		{"very_long_number", "5511999998888000111", "5511999998888000111"},
		{"plus_only", "+", ""},
		{"parentheses_only", "()", ""},
		{"dash_only", "---", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := cleanPhoneNumber(tc.input)
			if result != tc.expected {
				t.Errorf("cleanPhoneNumber(%q) = %q, expected %q", tc.input, result, tc.expected)
			}
		})
	}
}

// ============================================================
// Helper: ctx() shorthand
// ============================================================

func ctx() context.Context {
	return context.Background()
}
