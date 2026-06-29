// Package redirect provides the redirect link service for managing redirect links
// and processing public redirect requests.
package redirect

import (
	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/models"
	"automatiza-go/internal/observability"
	"automatiza-go/internal/repository"
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"

	"go.uber.org/zap"
)

// Service defines the redirect service interface
type Service interface {
	// ProcessRedirect processes a public redirect request and returns the invite URL
	ProcessRedirect(ctx context.Context, slug string, linkID string) (*models.ProcessRedirectResult, error)

	// ListGroups lists all groups available for redirect links for a user
	ListGroups(ctx context.Context, userID string) ([]models.Group, error)

	// ListRedirectLinks lists all redirect links for a user
	ListRedirectLinks(ctx context.Context, userID string) ([]models.RedirectLink, error)

	// GetRedirectLink gets a redirect link by ID
	GetRedirectLink(ctx context.Context, linkID string) (*models.RedirectLink, error)

	// CreateRedirectLink creates a new redirect link
	CreateRedirectLink(ctx context.Context, userID string, req *models.RedirectLinkCreateRequest) (*models.RedirectLink, error)

	// UpdateRedirectLink updates an existing redirect link
	UpdateRedirectLink(ctx context.Context, linkID, userID string, req *models.RedirectLinkUpdateRequest) error

	// DeleteRedirectLink deletes a redirect link
	DeleteRedirectLink(ctx context.Context, linkID, userID string) error

	// CheckSlugAvailability checks if a slug is available
	CheckSlugAvailability(ctx context.Context, slug string, excludeID string) (bool, error)
}

// redirectService implements the Service interface
type redirectService struct {
	redirectLinkRepo repository.RedirectLinkRepository
	groupRepo        repository.GroupRepository
	instanceRepo     repository.InstanceRepository
	wahaProvider     whatsapp.WhatsAppProvider
	logger           *zap.Logger
}

// NewRedirectService creates a new redirect service
func NewRedirectService(
	redirectLinkRepo repository.RedirectLinkRepository,
	groupRepo repository.GroupRepository,
	instanceRepo repository.InstanceRepository,
	wahaProvider whatsapp.WhatsAppProvider,
	logger *zap.Logger,
) Service {
	return &redirectService{
		redirectLinkRepo: redirectLinkRepo,
		groupRepo:        groupRepo,
		instanceRepo:     instanceRepo,
		wahaProvider:     wahaProvider,
		logger:           logger,
	}
}

// ProcessRedirect processes a public redirect request and returns the invite URL
func (s *redirectService) ProcessRedirect(ctx context.Context, slug string, linkID string) (*models.ProcessRedirectResult, error) {
	// Get the redirect link
	var link *models.RedirectLink
	var err error

	if linkID != "" {
		link, err = s.redirectLinkRepo.GetByID(ctx, linkID)
	} else if slug != "" {
		link, err = s.redirectLinkRepo.GetBySlug(ctx, slug)
	} else {
		return &models.ProcessRedirectResult{
			Success: false,
			Error:   "slug or linkId is required",
		}, nil
	}

	if err != nil {
		s.logger.Error("Failed to get redirect link",
			zap.String("slug", slug),
			zap.String("linkID", linkID),
			zap.Error(err))
		return &models.ProcessRedirectResult{
			Success: false,
			Error:   "internal error",
		}, nil
	}

	if link == nil {
		return &models.ProcessRedirectResult{
			Success: false,
			Error:   "link not found",
		}, nil
	}

	if !link.IsActive {
		return &models.ProcessRedirectResult{
			Success: false,
			Error:   "link is inactive",
		}, nil
	}

	if len(link.TargetGroups) == 0 {
		return &models.ProcessRedirectResult{
			Success: false,
			Error:   "no target groups configured",
		}, nil
	}

	// Get member counts for groups if strategy is member_limit
	groupMemberCounts := make(map[string]int)
	if link.Strategy == models.RedirectStrategyMemberLimit {
		for _, tg := range link.TargetGroups {
			group, err := s.groupRepo.GetGroupByID(ctx, tg.GroupID)
			if err != nil {
				s.logger.Warn("Failed to get group for member count",
					zap.String("groupID", tg.GroupID),
					zap.Error(err))
				continue
			}
			if group != nil {
				groupMemberCounts[tg.GroupID] = group.MembersCount
			}
		}
	}

	// Sort target groups by priority
	sortedGroups := make([]models.GroupPriority, len(link.TargetGroups))
	copy(sortedGroups, link.TargetGroups)
	sort.Slice(sortedGroups, func(i, j int) bool {
		return sortedGroups[i].Priority < sortedGroups[j].Priority
	})
	link.TargetGroups = sortedGroups

	// Get the available group based on strategy. For member_limit, the cached
	// members_count can lag reality (the periodic/connect sync may be minutes
	// old, and WhatsApp's reported group size under-counts near the cap), so a
	// group reading just under the limit may already be full. pickMemberLimitGroupLive
	// re-verifies any boundary candidate against WhatsApp's live participant
	// count before committing, so the link never hands out a full group.
	var targetGroup *models.GroupPriority
	if link.Strategy == models.RedirectStrategyMemberLimit {
		targetGroup = s.pickMemberLimitGroupLive(ctx, link, groupMemberCounts)
	} else {
		targetGroup = link.GetAvailableGroup(groupMemberCounts)
	}
	if targetGroup == nil {
		return &models.ProcessRedirectResult{
			Success: false,
			Error:   "no available groups (all groups are full or inactive)",
		}, nil
	}

	// Get the group details to find the wa_group_id and instance
	group, err := s.groupRepo.GetGroupByID(ctx, targetGroup.GroupID)
	if err != nil || group == nil {
		s.logger.Error("Failed to get group details",
			zap.String("groupID", targetGroup.GroupID),
			zap.Error(err))
		return &models.ProcessRedirectResult{
			Success: false,
			Error:   "group not found",
		}, nil
	}

	// Get the instance for this group
	instanceIDInt, parseErr := strconv.Atoi(group.InstanceID)
	if parseErr != nil || instanceIDInt <= 0 {
		s.logger.Warn("Invalid instance ID for group — skipping redirect",
			zap.String("groupID", targetGroup.GroupID),
			zap.String("rawInstanceID", group.InstanceID))
		return &models.ProcessRedirectResult{
			Success: false,
			Error:   "instance not found",
		}, nil
	}
	instance, err := s.instanceRepo.GetInstanceByID(ctx, instanceIDInt)
	if err != nil || instance == nil {
		s.logger.Error("Failed to get instance for group",
			zap.String("groupID", targetGroup.GroupID),
			zap.String("instanceID", group.InstanceID),
			zap.Error(err))
		return &models.ProcessRedirectResult{
			Success: false,
			Error:   "instance not found",
		}, nil
	}

	// Try to use the invite code saved in the database first
	var inviteCode string
	if group.InviteCode != nil && *group.InviteCode != "" {
		inviteCode = *group.InviteCode
		s.logger.Debug("Using saved invite code from database",
			zap.String("groupID", group.ID),
			zap.String("groupName", group.Name))
	} else {
		// Fallback: Get the invite code from WAHA if not saved
		if s.wahaProvider == nil {
			return &models.ProcessRedirectResult{
				Success: false,
				Error:   "WhatsApp provider not configured",
			}, nil
		}

		var err error
		inviteCode, err = s.wahaProvider.GetGroupInviteCode(ctx, instance.InstanceID, group.WAGroupID)
		if err != nil {
			s.logger.Error("Failed to get group invite code from WAHA",
				zap.String("sessionName", instance.InstanceID),
				zap.String("waGroupID", group.WAGroupID),
				zap.String("groupName", group.Name),
				zap.Bool("isAdmin", group.IsAdmin),
				zap.Error(err))
			observability.CaptureWithTags(err, map[string]string{
				"module":      "redirect",
				"flow":        "invite-code",
				"sessionName": instance.InstanceID,
				"waGroupID":   group.WAGroupID,
				"groupName":   group.Name,
			})

			// Differentiate temporary from permanent errors
			var providerErr *whatsapp.ProviderError
			if errors.As(err, &providerErr) {
				switch providerErr.Code {
				case whatsapp.ErrCodeRateLimited, whatsapp.ErrCodeTimeout, whatsapp.ErrCodeNetworkError, whatsapp.ErrCodeProviderError:
					return &models.ProcessRedirectResult{
						Success: false,
						Error:   "service temporarily unavailable",
					}, nil
				case whatsapp.ErrCodePermissionDenied:
					return &models.ProcessRedirectResult{
						Success: false,
						Error:   "bot is not admin in group",
					}, nil
				}
			}
			return &models.ProcessRedirectResult{
				Success: false,
				Error:   "failed to get invite code",
			}, nil
		}

		if inviteCode == "" {
			return &models.ProcessRedirectResult{
				Success: false,
				Error:   "invite code not available",
			}, nil
		}

		// Save the invite code to the database for future use
		if err := s.groupRepo.UpdateGroup(ctx, group.ID, map[string]interface{}{"invite_code": inviteCode}); err != nil {
			s.logger.Warn("Failed to save invite code to database",
				zap.String("groupID", group.ID),
				zap.Error(err))
		}
	}

	// Increment click counters
	if err := s.redirectLinkRepo.IncrementClicks(ctx, link.ID, targetGroup.GroupID); err != nil {
		// Log but don't fail the request
		s.logger.Warn("Failed to increment click counters",
			zap.String("linkID", link.ID),
			zap.String("groupID", targetGroup.GroupID),
			zap.Error(err))
	}

	// Build the invite URL
	inviteURL := fmt.Sprintf("https://chat.whatsapp.com/%s", inviteCode)

	return &models.ProcessRedirectResult{
		Success:     true,
		RedirectURL: inviteURL,
		GroupName:   group.Name,
		GroupID:     group.ID,
		WAGroupID:   group.WAGroupID,
	}, nil
}

// liveRefreshMargin bounds how close to the limit a cached count must be before
// ProcessRedirect re-verifies it against WhatsApp's live participant count.
// Groups reading comfortably below the limit are trusted from cache (no WAHA
// call) since even a stale count that far down won't be full. The margin covers
// both sync lag and WhatsApp's tendency to under-report group size near the cap.
const liveRefreshMargin = 150

// pickMemberLimitGroupLive selects the target group for the member_limit
// strategy, re-verifying boundary candidates against WhatsApp's live participant
// count. Each candidate is live-checked at most once; a candidate found full is
// persisted and skipped, then selection re-runs. Falls back to the cached pick
// when the live count can't be fetched.
func (s *redirectService) pickMemberLimitGroupLive(
	ctx context.Context,
	link *models.RedirectLink,
	counts map[string]int,
) *models.GroupPriority {
	verified := make(map[string]bool)
	for {
		candidate := link.GetAvailableGroup(counts)
		if candidate == nil {
			return nil
		}
		limit := candidate.MemberLimit
		if limit <= 0 {
			limit = models.DefaultGroupMemberLimit
		}
		// Trust the cache when the candidate is comfortably below the limit, or
		// when it was already live-verified during this call.
		if verified[candidate.GroupID] || counts[candidate.GroupID] < limit-liveRefreshMargin {
			return candidate
		}
		verified[candidate.GroupID] = true
		liveCount, ok := s.refreshGroupMemberCount(ctx, candidate.GroupID)
		if !ok {
			return candidate // couldn't refresh — best effort, use cached pick
		}
		counts[candidate.GroupID] = liveCount
		// Loop: re-evaluate. If now full, GetAvailableGroup skips to the next.
	}
}

// refreshGroupMemberCount fetches a group's current participant count from
// WhatsApp and persists it to members_count. Returns (count, true) on success.
func (s *redirectService) refreshGroupMemberCount(ctx context.Context, groupID string) (int, bool) {
	if s.wahaProvider == nil {
		return 0, false
	}
	group, err := s.groupRepo.GetGroupByID(ctx, groupID)
	if err != nil || group == nil {
		return 0, false
	}
	instanceIDInt, parseErr := strconv.Atoi(group.InstanceID)
	if parseErr != nil || instanceIDInt <= 0 {
		return 0, false
	}
	instance, err := s.instanceRepo.GetInstanceByID(ctx, instanceIDInt)
	if err != nil || instance == nil {
		return 0, false
	}
	participants, err := s.wahaProvider.GetGroupParticipants(ctx, instance.InstanceID, group.WAGroupID)
	if err != nil {
		s.logger.Warn("Live member-count refresh failed; falling back to cached count",
			zap.String("groupID", groupID),
			zap.String("waGroupID", group.WAGroupID),
			zap.Error(err))
		return 0, false
	}
	count := len(participants)
	if err := s.groupRepo.UpdateGroup(ctx, groupID, map[string]interface{}{"members_count": count}); err != nil {
		s.logger.Warn("Failed to persist refreshed member count",
			zap.String("groupID", groupID),
			zap.Error(err))
	}
	return count, true
}

// ListGroups lists all groups available for redirect links for a user
func (s *redirectService) ListGroups(ctx context.Context, userID string) ([]models.Group, error) {
	groups, err := s.groupRepo.GetUserGroups(ctx, userID, false)
	if err != nil {
		return nil, fmt.Errorf("failed to list groups: %w", err)
	}
	return groups, nil
}

// ListRedirectLinks lists all redirect links for a user
func (s *redirectService) ListRedirectLinks(ctx context.Context, userID string) ([]models.RedirectLink, error) {
	links, err := s.redirectLinkRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list redirect links: %w", err)
	}
	return links, nil
}

// GetRedirectLink gets a redirect link by ID
func (s *redirectService) GetRedirectLink(ctx context.Context, linkID string) (*models.RedirectLink, error) {
	link, err := s.redirectLinkRepo.GetByID(ctx, linkID)
	if err != nil {
		return nil, fmt.Errorf("failed to get redirect link: %w", err)
	}
	return link, nil
}

// CreateRedirectLink creates a new redirect link
func (s *redirectService) CreateRedirectLink(ctx context.Context, userID string, req *models.RedirectLinkCreateRequest) (*models.RedirectLink, error) {
	// Check if slug is available
	exists, err := s.redirectLinkRepo.CheckSlugExists(ctx, req.Slug, "")
	if err != nil {
		return nil, fmt.Errorf("failed to check slug availability: %w", err)
	}
	if exists {
		return nil, errors.New("slug already in use")
	}

	// Validate strategy
	switch req.Strategy {
	case models.RedirectStrategyMemberLimit, models.RedirectStrategyClickLimit, models.RedirectStrategyRoundRobin:
		// Valid strategy
	default:
		return nil, errors.New("invalid strategy")
	}

	// Create the link
	link := &models.RedirectLink{
		UserID:       userID,
		Slug:         req.Slug,
		Name:         req.Name,
		Strategy:     req.Strategy,
		TargetGroups: req.TargetGroups,
		IsActive:     req.IsActive,
		GroupClicks:  make(map[string]int),
	}

	createdLink, err := s.redirectLinkRepo.Create(ctx, link)
	if err != nil {
		return nil, fmt.Errorf("failed to create redirect link: %w", err)
	}

	s.logger.Info("Created redirect link",
		zap.String("userID", userID),
		zap.String("linkID", createdLink.ID),
		zap.String("slug", req.Slug))

	return createdLink, nil
}

// UpdateRedirectLink updates an existing redirect link
func (s *redirectService) UpdateRedirectLink(ctx context.Context, linkID, userID string, req *models.RedirectLinkUpdateRequest) error {
	// Verify the link exists and belongs to user
	link, err := s.redirectLinkRepo.GetByID(ctx, linkID)
	if err != nil {
		return fmt.Errorf("failed to get redirect link: %w", err)
	}
	if link == nil {
		return errors.New("redirect link not found")
	}
	if link.UserID != userID {
		return errors.New("unauthorized")
	}

	// Build updates map
	updates := make(map[string]interface{})

	if req.Slug != nil {
		// Check if new slug is available
		exists, err := s.redirectLinkRepo.CheckSlugExists(ctx, *req.Slug, linkID)
		if err != nil {
			return fmt.Errorf("failed to check slug availability: %w", err)
		}
		if exists {
			return errors.New("slug already in use")
		}
		updates["slug"] = *req.Slug
	}

	if req.Name != nil {
		updates["name"] = *req.Name
	}

	if req.Strategy != nil {
		switch *req.Strategy {
		case models.RedirectStrategyMemberLimit, models.RedirectStrategyClickLimit, models.RedirectStrategyRoundRobin:
			updates["strategy"] = *req.Strategy
		default:
			return errors.New("invalid strategy")
		}
	}

	if req.TargetGroups != nil {
		updates["target_groups"] = *req.TargetGroups
	}

	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}

	if len(updates) == 0 {
		return nil
	}

	if err := s.redirectLinkRepo.Update(ctx, linkID, updates); err != nil {
		return fmt.Errorf("failed to update redirect link: %w", err)
	}

	s.logger.Info("Updated redirect link",
		zap.String("userID", userID),
		zap.String("linkID", linkID))

	return nil
}

// DeleteRedirectLink deletes a redirect link
func (s *redirectService) DeleteRedirectLink(ctx context.Context, linkID, userID string) error {
	if err := s.redirectLinkRepo.Delete(ctx, linkID, userID); err != nil {
		return fmt.Errorf("failed to delete redirect link: %w", err)
	}

	s.logger.Info("Deleted redirect link",
		zap.String("userID", userID),
		zap.String("linkID", linkID))

	return nil
}

// CheckSlugAvailability checks if a slug is available
func (s *redirectService) CheckSlugAvailability(ctx context.Context, slug string, excludeID string) (bool, error) {
	exists, err := s.redirectLinkRepo.CheckSlugExists(ctx, slug, excludeID)
	if err != nil {
		return false, fmt.Errorf("failed to check slug availability: %w", err)
	}
	return !exists, nil
}
