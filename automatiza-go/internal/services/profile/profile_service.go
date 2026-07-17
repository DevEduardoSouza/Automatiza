package profile

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"automatiza-go/internal/lib/quota"
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"

	"go.uber.org/zap"
)

// ProfileService handles profile business logic
type ProfileService struct {
	profileRepo     repository.ProfileRepository
	automationRepo  repository.AutomationRepository
	instanceRepo    repository.InstanceRepository
	userRoleRepo    repository.UserRoleRepository
	systemEventRepo repository.SystemEventRepository
	logger          *zap.Logger
}

// NewProfileService creates a new profile service
func NewProfileService(
	profileRepo repository.ProfileRepository,
	automationRepo repository.AutomationRepository,
	instanceRepo repository.InstanceRepository,
	userRoleRepo repository.UserRoleRepository,
	systemEventRepo repository.SystemEventRepository,
	logger *zap.Logger,
) *ProfileService {
	return &ProfileService{
		profileRepo:     profileRepo,
		automationRepo:  automationRepo,
		instanceRepo:    instanceRepo,
		userRoleRepo:    userRoleRepo,
		systemEventRepo: systemEventRepo,
		logger:          logger,
	}
}

// GetProfile returns the profile for a user
func (s *ProfileService) GetProfile(ctx context.Context, userID string) (*models.Profile, error) {
	profile, err := s.profileRepo.GetProfile(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get profile: %w", err)
	}
	if profile == nil {
		return nil, ErrProfileNotFound
	}
	return profile, nil
}

// GetUserLimits returns the limits and current usage for a user
func (s *ProfileService) GetUserLimits(ctx context.Context, userID string) (*UserLimits, error) {
	// Get profile for plan info
	profile, err := s.GetProfile(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Get automations count
	automations, err := s.automationRepo.GetUserAutomations(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get automations: %w", err)
	}

	// Get connected instances count
	instances, err := s.instanceRepo.GetUserInstances(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get instances: %w", err)
	}

	connectedCount := int32(0)
	for _, inst := range instances {
		if inst.Status == models.InstanceStatusConnected {
			connectedCount++
		}
	}

	var maxAutomations int32
	if profile.MaxAutomations != nil {
		maxAutomations = int32(*profile.MaxAutomations)
	} else {
		maxAutomations = GetMaxAutomationsForPlan(profile.Plan)
	}

	// Distinct destination groups across ACTIVE automations — mirrors the
	// per-plan group cap (see AutomationsHandler.validatePlanGroupLimit) so the
	// dashboard "X / N grupos" matches what the enforcement actually counts.
	groupSet := make(map[string]struct{})
	for i := range automations {
		a := &automations[i]
		if !a.IsActive() {
			continue
		}
		for _, g := range a.GetTargetGroups() {
			groupSet[g] = struct{}{}
		}
	}

	return &UserLimits{
		AutomationsCount:        int32(len(automations)),
		MaxAutomations:          maxAutomations,
		ConnectedInstancesCount: connectedCount,
		GroupLimit:              GetGroupLimitForPlan(profile.Plan),
		GroupsUsed:              int32(len(groupSet)),
		//nolint:gosec // EffectiveMaxInstances returns a small positive cap
		MaxInstances: int32(quota.EffectiveMaxInstances(profile.MaxInstances)),
	}, nil
}

// GetGroupLimitForPlan returns the distinct-group cap for a plan slug. The
// tiered plans encode the limit in the slug itself ("grupos-5" → 5,
// "grupos-200" → 200), so we derive it directly — no DB lookup and no risk of
// drifting from the seeded values. Any non-tiered slug (free/starter/pro/
// enterprise/elite/empty) returns 0 = unlimited (grandfathered).
func GetGroupLimitForPlan(plan string) int32 {
	const prefix = "grupos-"
	if !strings.HasPrefix(plan, prefix) {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimPrefix(plan, prefix))
	if err != nil || n < 0 || n > 100000 {
		return 0
	}
	return int32(n) //nolint:gosec // n is bounded to [0,100000] above; fits int32
}

// UpdateProfile updates user profile fields
func (s *ProfileService) UpdateProfile(ctx context.Context, userID string, input UpdateProfileInput) (*models.Profile, error) {
	updates := make(map[string]interface{})

	if input.PhoneNumber != nil {
		// Clean and validate phone number
		cleaned := cleanPhoneNumber(*input.PhoneNumber)
		updates["phone_number"] = cleaned
	}

	if len(updates) == 0 {
		// No updates, just return current profile
		return s.GetProfile(ctx, userID)
	}

	if err := s.profileRepo.UpdateProfile(ctx, userID, updates); err != nil {
		return nil, fmt.Errorf("update profile: %w", err)
	}

	return s.GetProfile(ctx, userID)
}

// ListProfiles returns a paginated list of profiles (admin only)
func (s *ProfileService) ListProfiles(ctx context.Context, adminUserID, cursor string, limit int) (*ProfilesPage, error) {
	// Verify admin access
	isAdmin, err := s.userRoleRepo.IsAdmin(ctx, adminUserID)
	if err != nil {
		s.logger.Warn("failed to check admin status", zap.Error(err))
		return nil, ErrPermissionDenied
	}
	if !isAdmin {
		return nil, ErrPermissionDenied
	}

	// Apply default and max limits
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	profiles, nextCursor, err := s.profileRepo.ListProfiles(ctx, cursor, limit)
	if err != nil {
		return nil, fmt.Errorf("list profiles: %w", err)
	}

	return &ProfilesPage{
		Profiles:   profiles,
		NextCursor: nextCursor,
		HasMore:    nextCursor != "",
	}, nil
}

// SearchProfiles searches profiles by email or phone (admin only)
func (s *ProfileService) SearchProfiles(ctx context.Context, adminUserID, query, cursor string, limit int) (*ProfilesPage, error) {
	// Verify admin access
	isAdmin, err := s.userRoleRepo.IsAdmin(ctx, adminUserID)
	if err != nil {
		s.logger.Warn("failed to check admin status", zap.Error(err))
		return nil, ErrPermissionDenied
	}
	if !isAdmin {
		return nil, ErrPermissionDenied
	}

	// Apply default and max limits
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	profiles, nextCursor, err := s.profileRepo.SearchProfiles(ctx, query, cursor, limit)
	if err != nil {
		return nil, fmt.Errorf("search profiles: %w", err)
	}

	return &ProfilesPage{
		Profiles:   profiles,
		NextCursor: nextCursor,
		HasMore:    nextCursor != "",
	}, nil
}

// BlockUser blocks a user (admin only)
func (s *ProfileService) BlockUser(ctx context.Context, adminUserID, targetUserID string) (*models.Profile, error) {
	// Verify admin access
	isAdmin, err := s.userRoleRepo.IsAdmin(ctx, adminUserID)
	if err != nil || !isAdmin {
		return nil, ErrPermissionDenied
	}

	// Update billing status to suspended
	suspended := models.BillingStatusSuspended
	if err := s.profileRepo.UpdateProfile(ctx, targetUserID, map[string]interface{}{
		"billing_status": suspended,
	}); err != nil {
		return nil, fmt.Errorf("block user: %w", err)
	}

	return s.profileRepo.GetProfile(ctx, targetUserID)
}

// UnblockUser unblocks a user (admin only)
func (s *ProfileService) UnblockUser(ctx context.Context, adminUserID, targetUserID string) (*models.Profile, error) {
	// Verify admin access
	isAdmin, err := s.userRoleRepo.IsAdmin(ctx, adminUserID)
	if err != nil || !isAdmin {
		return nil, ErrPermissionDenied
	}

	// Update billing status to active
	active := models.BillingStatusActive
	if err := s.profileRepo.UpdateProfile(ctx, targetUserID, map[string]interface{}{
		"billing_status": active,
	}); err != nil {
		return nil, fmt.Errorf("unblock user: %w", err)
	}

	return s.profileRepo.GetProfile(ctx, targetUserID)
}

// UpdateUserPlan updates a user's plan (admin only)
func (s *ProfileService) UpdateUserPlan(ctx context.Context, adminUserID, targetUserID string, plan *string, maxAutomations *int32, planAccessUntil *string) (*models.Profile, error) {
	// Verify admin access
	isAdmin, err := s.userRoleRepo.IsAdmin(ctx, adminUserID)
	if err != nil || !isAdmin {
		return nil, ErrPermissionDenied
	}

	updates := make(map[string]interface{})
	if plan != nil {
		updates["plan"] = *plan
	}
	if maxAutomations != nil {
		updates["max_automations"] = *maxAutomations
	}
	if planAccessUntil != nil {
		updates["plan_access_until"] = *planAccessUntil
	}

	if len(updates) == 0 {
		return s.profileRepo.GetProfile(ctx, targetUserID)
	}

	// Snapshot before values so the audit trail shows plan/limit ELEVATIONS (a
	// compromised admin bumping a user to enterprise/high limits was invisible before).
	before, _ := s.profileRepo.GetProfile(ctx, targetUserID)

	if err := s.profileRepo.UpdateProfile(ctx, targetUserID, updates); err != nil {
		return nil, fmt.Errorf("update user plan: %w", err)
	}

	s.auditPlanChange(ctx, adminUserID, targetUserID, before, updates)

	return s.profileRepo.GetProfile(ctx, targetUserID)
}

// auditPlanChange records an admin plan/limit change in system_events. Plan and limit
// mutations had no audit trail, so an attacker elevating accounts left no evidence — this
// closes that blind spot. Best-effort: never blocks the actual update.
func (s *ProfileService) auditPlanChange(ctx context.Context, adminUserID, targetUserID string, before *models.Profile, updates map[string]interface{}) {
	if s.systemEventRepo == nil {
		return
	}
	payload := map[string]interface{}{
		"admin_id":       adminUserID,
		"target_user_id": targetUserID,
		"changes":        updates, // new values (plan / max_automations / plan_access_until)
	}
	if before != nil {
		payload["old_plan"] = before.Plan
		if before.MaxAutomations != nil {
			payload["old_max_automations"] = *before.MaxAutomations
		}
		payload["target_user_email"] = before.Email
	}
	if err := s.systemEventRepo.Create(ctx, "admin_update_user_plan", payload); err != nil {
		s.logger.Warn("failed to audit plan change",
			zap.String("admin_id", adminUserID),
			zap.String("target_user_id", targetUserID),
			zap.Error(err))
	}
}

// GetMaxAutomationsForPlan returns the max automations allowed for a plan
func GetMaxAutomationsForPlan(plan string) int32 {
	switch plan {
	case models.PlanFree:
		return 1
	case models.PlanPro:
		return 5
	case models.PlanElite:
		return 20
	default:
		return 1
	}
}

// cleanPhoneNumber removes non-digit characters from phone number
func cleanPhoneNumber(phone string) string {
	// Remove all non-digit characters
	re := regexp.MustCompile(`[^\d]`)
	cleaned := re.ReplaceAllString(phone, "")

	// Trim any leading zeros
	cleaned = strings.TrimLeft(cleaned, "0")

	return cleaned
}
