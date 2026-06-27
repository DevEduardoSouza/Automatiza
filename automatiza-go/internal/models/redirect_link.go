package models

import (
	"encoding/json"
	"time"
)

// RedirectLink represents a saved redirect link for group distribution
type RedirectLink struct {
	ID           string          `json:"id" db:"id"`
	UserID       string          `json:"user_id" db:"user_id"`
	Slug         string          `json:"slug" db:"slug"`
	Name         string          `json:"name" db:"name"`
	Strategy     string          `json:"strategy" db:"strategy"` // member_limit, click_limit, round_robin
	TargetGroups []GroupPriority `json:"target_groups" db:"target_groups"`
	ClickCount   int             `json:"click_count" db:"click_count"`
	GroupClicks  map[string]int  `json:"group_clicks" db:"group_clicks"` // group_id -> click_count
	IsActive     bool            `json:"is_active" db:"is_active"`
	CreatedAt    time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at" db:"updated_at"`
}

// GroupPriority represents a target group with priority and limits for redirect links
type GroupPriority struct {
	GroupID     string `json:"group_id"`
	GroupName   string `json:"group_name,omitempty"`
	WAGroupID   string `json:"wa_group_id,omitempty"`
	Priority    int    `json:"priority"`
	MemberLimit int    `json:"member_limit,omitempty"` // Used when strategy is member_limit
	ClickLimit  int    `json:"click_limit,omitempty"`  // Used when strategy is click_limit
	IsActive    bool   `json:"is_active"`
}

// ProcessRedirectResult represents the result of processing a redirect
type ProcessRedirectResult struct {
	Success     bool   `json:"success"`
	RedirectURL string `json:"redirect_url"`
	GroupName   string `json:"group_name"`
	GroupID     string `json:"group_id"`
	WAGroupID   string `json:"wa_group_id"`
	Error       string `json:"error,omitempty"`
}

// RedirectLinkStrategy constants
const (
	RedirectStrategyMemberLimit = "member_limit"
	RedirectStrategyClickLimit  = "click_limit"
	RedirectStrategyRoundRobin  = "round_robin"
)

// DefaultGroupMemberLimit is the fallback per-group capacity used by the
// member_limit strategy when a target group has no explicit limit configured.
// Set slightly BELOW WhatsApp's hard group cap (1024) on purpose: members_count
// is refreshed by the groups sync only periodically (~25min), so a group sitting
// right at the 1024 cap reads as "still has a few slots" while already full in
// WhatsApp — the link would jam newcomers into a full group and never rotate.
// The ~24-slot buffer makes the link advance to the next group before the real
// cap, so clicks always land somewhere joinable. (The earlier bug was treating
// an unset/zero limit as INFINITE capacity, pinning every click to the 1st group.)
const DefaultGroupMemberLimit = 1000

// RedirectLinkCreateRequest represents a request to create a redirect link
type RedirectLinkCreateRequest struct {
	Slug         string          `json:"slug" validate:"required,min=3,max=50"`
	Name         string          `json:"name" validate:"required,min=1,max=255"`
	Strategy     string          `json:"strategy" validate:"required,oneof=member_limit click_limit round_robin"`
	TargetGroups []GroupPriority `json:"target_groups" validate:"required,min=1"`
	IsActive     bool            `json:"is_active"`
}

// RedirectLinkUpdateRequest represents a request to update a redirect link
type RedirectLinkUpdateRequest struct {
	Slug         *string          `json:"slug,omitempty" validate:"omitempty,min=3,max=50"`
	Name         *string          `json:"name,omitempty" validate:"omitempty,min=1,max=255"`
	Strategy     *string          `json:"strategy,omitempty" validate:"omitempty,oneof=member_limit click_limit round_robin"`
	TargetGroups *[]GroupPriority `json:"target_groups,omitempty" validate:"omitempty,min=1"`
	IsActive     *bool            `json:"is_active,omitempty"`
}

// PublicRedirectRequest represents a public request to process a redirect
type PublicRedirectRequest struct {
	Slug   string `json:"slug,omitempty"`
	LinkID string `json:"linkId,omitempty"`
}

// PublicRedirectResponse represents the response for a public redirect request
type PublicRedirectResponse struct {
	Success     bool   `json:"success"`
	RedirectURL string `json:"redirect_url,omitempty"`
	GroupName   string `json:"group_name,omitempty"`
	GroupID     string `json:"group_id,omitempty"`
	Error       string `json:"error,omitempty"`
}

// MarshalTargetGroups converts TargetGroups to JSON bytes for database storage
func (r *RedirectLink) MarshalTargetGroups() ([]byte, error) {
	if r.TargetGroups == nil {
		return []byte("[]"), nil
	}
	return json.Marshal(r.TargetGroups)
}

// UnmarshalTargetGroups converts JSON bytes to TargetGroups from database
func (r *RedirectLink) UnmarshalTargetGroups(data []byte) error {
	if data == nil || len(data) == 0 {
		r.TargetGroups = []GroupPriority{}
		return nil
	}
	return json.Unmarshal(data, &r.TargetGroups)
}

// MarshalGroupClicks converts GroupClicks to JSON bytes for database storage
func (r *RedirectLink) MarshalGroupClicks() ([]byte, error) {
	if r.GroupClicks == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(r.GroupClicks)
}

// UnmarshalGroupClicks converts JSON bytes to GroupClicks from database
func (r *RedirectLink) UnmarshalGroupClicks(data []byte) error {
	if data == nil || len(data) == 0 {
		r.GroupClicks = make(map[string]int)
		return nil
	}
	return json.Unmarshal(data, &r.GroupClicks)
}

// GetAvailableGroup returns the first available group based on strategy
func (r *RedirectLink) GetAvailableGroup(groupMemberCounts map[string]int) *GroupPriority {
	if len(r.TargetGroups) == 0 {
		return nil
	}

	// Sort by priority (already assumed sorted in target_groups)
	for i := range r.TargetGroups {
		group := &r.TargetGroups[i]
		if !group.IsActive {
			continue
		}

		switch r.Strategy {
		case RedirectStrategyMemberLimit:
			// Check if group has reached member limit. When no explicit limit
			// is configured (<= 0), fall back to WhatsApp's cap instead of
			// treating the group as having infinite capacity — otherwise the
			// link would always return this first group and never rotate when
			// it fills up.
			limit := group.MemberLimit
			if limit <= 0 {
				limit = DefaultGroupMemberLimit
			}
			memberCount := groupMemberCounts[group.GroupID]
			if memberCount < limit {
				return group
			}
		case RedirectStrategyClickLimit:
			// Check if group has reached click limit
			clickCount := r.GroupClicks[group.GroupID]
			if group.ClickLimit <= 0 || clickCount < group.ClickLimit {
				return group
			}
		case RedirectStrategyRoundRobin:
			// Simple round robin - find group with lowest clicks
			minClicks := -1
			var selectedGroup *GroupPriority
			for j := range r.TargetGroups {
				g := &r.TargetGroups[j]
				if !g.IsActive {
					continue
				}
				clicks := r.GroupClicks[g.GroupID]
				if minClicks == -1 || clicks < minClicks {
					minClicks = clicks
					selectedGroup = g
				}
			}
			return selectedGroup
		default:
			// Default to first active group
			return group
		}
	}

	return nil
}
