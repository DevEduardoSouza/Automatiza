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
	"regexp"
	"sort"
	"strconv"
	"sync"

	"go.uber.org/zap"
)

// Shape of a WhatsApp group invite code, per WAHA
// GET /api/{session}/groups/{id}/invite-code (waha-plus-openapi.json:6093,
// 200 = plain string). Validated before the code is persisted, incremented
// against or turned into a URL.
var inviteCodeRe = regexp.MustCompile(`^[A-Za-z0-9_-]{5,128}$`)

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

	// Per-link serialization of ProcessRedirect. Zero value is ready to use,
	// so a redirectService built literally (as the tests do) still locks.
	linkLocksMu sync.Mutex
	linkLocks   map[string]*linkLock
}

// linkLock serializes the resolve → select → increment sequence of ONE link.
// refs lets the entry be removed once nobody is using it, so a long tail of
// slugs doesn't grow the map forever.
type linkLock struct {
	mu   sync.Mutex
	refs int
}

// lockLink blocks until this process owns the given link, and returns the
// release function.
//
// ponytail: an in-process mutex is exactly enough for the current topology —
// ONE server container serves every /r hit. It buys `click_limit` and
// `round_robin` a consistent snapshot (select and increment can't interleave)
// without a round-trip to Postgres. It is NOT distributed: a second server
// replica would need a row lock (SELECT ... FOR UPDATE) or an idempotency key
// in Postgres instead. See the concurrency note on ProcessRedirect.
func (s *redirectService) lockLink(linkID string) func() {
	s.linkLocksMu.Lock()
	if s.linkLocks == nil {
		s.linkLocks = make(map[string]*linkLock)
	}
	l, ok := s.linkLocks[linkID]
	if !ok {
		l = &linkLock{}
		s.linkLocks[linkID] = l
	}
	l.refs++
	s.linkLocksMu.Unlock()

	l.mu.Lock()

	return func() {
		l.mu.Unlock()
		s.linkLocksMu.Lock()
		l.refs--
		if l.refs == 0 {
			delete(s.linkLocks, linkID)
		}
		s.linkLocksMu.Unlock()
	}
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

// loadRedirectLink resolves the link by id or slug. A non-nil result means
// "stop and return this to the caller"; both returns are never non-nil at once.
func (s *redirectService) loadRedirectLink(ctx context.Context, slug string, linkID string) (*models.RedirectLink, *models.ProcessRedirectResult) {
	var link *models.RedirectLink
	var err error

	if linkID != "" {
		link, err = s.redirectLinkRepo.GetByID(ctx, linkID)
	} else if slug != "" {
		link, err = s.redirectLinkRepo.GetBySlug(ctx, slug)
	} else {
		return nil, &models.ProcessRedirectResult{
			Success: false,
			Error:   "slug or linkId is required",
		}
	}

	if err != nil {
		s.logger.Error("Failed to get redirect link",
			zap.String("slug", slug),
			zap.String("linkID", linkID),
			zap.Error(err))
		return nil, &models.ProcessRedirectResult{
			Success: false,
			Error:   "internal error",
		}
	}

	if link == nil {
		return nil, &models.ProcessRedirectResult{
			Success: false,
			Error:   "link not found",
		}
	}

	return link, nil
}

// coldInvite says the selection landed on a group whose invite code is not
// cached: the caller must fetch it from WAHA OUTSIDE the link lock.
type coldInvite struct {
	group    *models.Group
	instance *models.Instance
}

// ProcessRedirect processes a public redirect request and returns the invite URL.
//
// Concurrency: the resolve → select → increment sequence is serialized per
// link, so two simultaneous clicks cannot read the same click_count and pick
// the same group. A success is only returned after the click was recorded.
//
// The WAHA call is deliberately OUTSIDE that critical section. It can take
// ~2min (30s timeout × up to 4 attempts — waha_adapter.go:48-53 and :525-528),
// and holding a per-link lock across it would serialize every other click on a
// link whose group has no cached code yet — 153 of 208 active links are in that
// state, including paid-campaign links doing ~50 hits/min. So a cold selection
// returns, releases the lock, fetches, and repeats the selection.
//
// What this does NOT promise: exactly-once across replicas (see lockLink), nor
// against an ambiguous commit or an invisible transport retry. And member_limit
// still reads the synced members_count — serializing clicks does not reserve a
// seat, so the documented lag between a click and a real join remains.
func (s *redirectService) ProcessRedirect(ctx context.Context, slug string, linkID string) (*models.ProcessRedirectResult, error) {
	first, res := s.loadRedirectLink(ctx, slug, linkID)
	if res != nil {
		return res, nil
	}

	// Codes fetched from WAHA during THIS request, keyed by group. Carried into
	// the second pass so a failed cache write (UpdateGroup is fail-open) cannot
	// turn a code we already hold into an error.
	fetched := make(map[string]string, 1)

	// Two passes at most: the first may end cold, and the second runs with the
	// code already in hand.
	for attempt := 0; attempt < 2; attempt++ {
		result, cold := s.selectAndCount(ctx, first.ID, slug, linkID, fetched)
		if result != nil {
			return result, nil
		}

		// Lock is released at this point.
		inviteCode, failure := s.fetchInviteCode(ctx, cold.instance, cold.group)
		if failure != nil {
			return failure, nil
		}
		fetched[cold.group.ID] = inviteCode

		// Cache for future clicks. Fail-open: losing the cache write is not a
		// reason to refuse a code that is already valid in hand.
		if err := s.groupRepo.UpdateGroup(ctx, cold.group.ID, map[string]interface{}{"invite_code": inviteCode}); err != nil {
			s.logger.Warn("Failed to save invite code to database",
				zap.String("groupID", cold.group.ID),
				zap.Error(err))
		}
	}

	// Both passes ended cold, i.e. the rotation moved to a different uncached
	// group in between. Retryable error beats a third WAHA round trip.
	return &models.ProcessRedirectResult{
		Success: false,
		Error:   "service temporarily unavailable",
	}, nil
}

// selectAndCount runs the part that MUST be serialized per link: reload the
// snapshot, pick a group and record the click. It holds the link lock for its
// whole body and never calls WAHA.
//
// Returns exactly one of: a final result, or a coldInvite asking the caller to
// fetch the invite code with the lock released.
func (s *redirectService) selectAndCount(
	ctx context.Context,
	lockID string,
	slug string,
	linkID string,
	fetched map[string]string,
) (*models.ProcessRedirectResult, *coldInvite) {
	release := s.lockLink(lockID)
	defer release()

	// Re-read under the lock: the snapshot from the first lookup may already be
	// stale (another click just incremented it).
	link, res := s.loadRedirectLink(ctx, slug, linkID)
	if res != nil {
		return res, nil
	}

	// The lock is keyed on the id learned BEFORE it was held. If the slug now
	// resolves to a different link, this goroutine holds the wrong lock and must
	// not increment under it.
	if link.ID != lockID {
		s.logger.Warn("Redirect link changed identity between lookups — refusing to count under the wrong lock",
			zap.String("lockedID", lockID),
			zap.String("loadedID", link.ID))
		return &models.ProcessRedirectResult{
			Success: false,
			Error:   "service temporarily unavailable",
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

	// Get the available group based on strategy
	targetGroup := link.GetAvailableGroup(groupMemberCounts)
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

	// Resolve the invite code without leaving the lock: from the database
	// cache, or from what this request already fetched on an earlier pass.
	var inviteCode string
	switch {
	case group.InviteCode != nil && *group.InviteCode != "":
		inviteCode = *group.InviteCode
		// Validate the stored value before it becomes a URL: the column is
		// written by other flows too, and a malformed code must not reach the
		// caller's Location header nor burn a click.
		if !inviteCodeRe.MatchString(inviteCode) {
			s.logger.Error("Saved invite code has an invalid shape — refusing to redirect",
				zap.String("groupID", group.ID),
				zap.Int("codeLength", len(inviteCode)))
			return &models.ProcessRedirectResult{
				Success: false,
				Error:   "invite code not available",
			}, nil
		}
		s.logger.Debug("Using saved invite code from database",
			zap.String("groupID", group.ID),
			zap.String("groupName", group.Name))
	case fetched[group.ID] != "":
		inviteCode = fetched[group.ID]
	default:
		if s.wahaProvider == nil {
			return &models.ProcessRedirectResult{
				Success: false,
				Error:   "WhatsApp provider not configured",
			}, nil
		}
		// Cold: hand the slow call back to the caller, lock-free.
		return nil, &coldInvite{group: group, instance: instance}
	}

	// Increment click counters. Fail-CLOSED on purpose: the counters are what
	// click_limit and round_robin decide on, so handing out a destination
	// without recording the click would silently overshoot the limit and skew
	// the rotation. A caller that gets no URL retries; a caller that got an
	// uncounted URL is a lost invariant.
	if err := s.redirectLinkRepo.IncrementClicks(ctx, link.ID, targetGroup.GroupID); err != nil {
		s.logger.Error("Failed to increment click counters — refusing the redirect",
			zap.String("linkID", link.ID),
			zap.String("groupID", targetGroup.GroupID),
			zap.Error(err))
		return &models.ProcessRedirectResult{
			Success: false,
			Error:   "service temporarily unavailable",
		}, nil
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

// fetchInviteCode asks WAHA for a group invite code. Called with NO link lock
// held — see the note on ProcessRedirect.
func (s *redirectService) fetchInviteCode(
	ctx context.Context,
	instance *models.Instance,
	group *models.Group,
) (string, *models.ProcessRedirectResult) {
	inviteCode, err := s.wahaProvider.GetGroupInviteCode(ctx, instance.InstanceID, group.WAGroupID)
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
				return "", &models.ProcessRedirectResult{
					Success: false,
					Error:   "service temporarily unavailable",
				}
			case whatsapp.ErrCodePermissionDenied:
				return "", &models.ProcessRedirectResult{
					Success: false,
					Error:   "bot is not admin in group",
				}
			}
		}
		return "", &models.ProcessRedirectResult{
			Success: false,
			Error:   "failed to get invite code",
		}
	}

	// Validate BEFORE persisting: an unusable code must not be cached in the
	// groups table for every future click to trip over.
	if !inviteCodeRe.MatchString(inviteCode) {
		s.logger.Error("WAHA returned an invite code with an invalid shape",
			zap.String("sessionName", instance.InstanceID),
			zap.String("waGroupID", group.WAGroupID),
			zap.Int("codeLength", len(inviteCode)))
		return "", &models.ProcessRedirectResult{
			Success: false,
			Error:   "invite code not available",
		}
	}

	return inviteCode, nil
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
