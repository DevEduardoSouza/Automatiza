package profile

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"

	"go.uber.org/zap"
)

// ProfileService handles profile business logic
type ProfileService struct {
	profileRepo    repository.ProfileRepository
	automationRepo repository.AutomationRepository
	instanceRepo   repository.InstanceRepository
	userRoleRepo   repository.UserRoleRepository
	logger         *zap.Logger
}

// NewProfileService creates a new profile service
func NewProfileService(
	profileRepo repository.ProfileRepository,
	automationRepo repository.AutomationRepository,
	instanceRepo repository.InstanceRepository,
	userRoleRepo repository.UserRoleRepository,
	logger *zap.Logger,
) *ProfileService {
	return &ProfileService{
		profileRepo:    profileRepo,
		automationRepo: automationRepo,
		instanceRepo:   instanceRepo,
		userRoleRepo:   userRoleRepo,
		logger:         logger,
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

	return &UserLimits{
		AutomationsCount:        int32(len(automations)),
		MaxAutomations:          maxAutomations,
		ConnectedInstancesCount: connectedCount,
	}, nil
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

	if err := s.profileRepo.UpdateProfile(ctx, targetUserID, updates); err != nil {
		return nil, fmt.Errorf("update user plan: %w", err)
	}

	return s.profileRepo.GetProfile(ctx, targetUserID)
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
