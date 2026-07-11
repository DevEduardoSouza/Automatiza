// Package quota centralises hard limits enforced server-side for
// multi-instance WhatsApp setups. Hardcoded for now; will be plan-driven
// in a future iteration once plans.features carries the limits.
package quota

import (
	"context"
	"errors"
	"fmt"
)

const (
	// MaxInstancesPerUser is the chip cap every account gets by default.
	//
	// It is 15, not 30, because 15 is the number users have actually been living
	// with: the UI has always stopped them there. The server said 30 while the
	// client said 15, and that gap is what made a "limit reached" message
	// impossible to reason about. One number now, shared by both.
	//
	// Raise it for a single account with profiles.max_instances (see
	// EffectiveMaxInstances) rather than by moving this constant, which would
	// hand every account more chips than the WAHA/proxy pool is sized for.
	MaxInstancesPerUser = 15

	// MaxGroupsPerInstance caps how many groups can be bound to one chip.
	// Mirrors the operational guidance from competitor research and matches
	// WhatsApp's per-account messaging recommendations to avoid bans.
	MaxGroupsPerInstance = 20
)

// ErrInstanceQuotaExceeded indicates the user has reached MaxInstancesPerUser.
var ErrInstanceQuotaExceeded = errors.New("instance quota exceeded")

// ErrGroupQuotaExceeded indicates an instance would exceed MaxGroupsPerInstance.
var ErrGroupQuotaExceeded = errors.New("group quota exceeded for instance")

// InstanceCounter is the slice of repository.InstanceRepository this package needs.
// Defined locally so consumers can plug in any implementation (real repo, fake, mock).
type InstanceCounter interface {
	CountUserInstances(ctx context.Context, userID string) (int, error)
}

// GroupCounter is the slice of repository.GroupRepository this package needs.
type GroupCounter interface {
	CountGroupsByInstance(ctx context.Context, userID string, instanceID int) (int, error)
}

// EffectiveMaxInstances resolves one account's chip cap from the optional
// profiles.max_instances override.
//
// The override can only RAISE the cap, never lower it. That asymmetry is not
// stylistic: the column was seeded years ago and never wired up, so it is full
// of stale values — 542 accounts say 1 and 295 say 5, while several of those
// same accounts legitimately run 9 to 20 chips today. Honouring it downward
// would strand hundreds of users at a chip or two. Reading it as an upgrade-only
// lever makes the stale data harmless and turns the column into what it is
// actually useful for: lifting the cap for one account without touching anyone
// else.
//
// A nil, zero, or below-default override means "no upgrade" → the default.
func EffectiveMaxInstances(override *int) int {
	if override != nil && *override > MaxInstancesPerUser {
		return *override
	}
	return MaxInstancesPerUser
}

// CheckCanCreateInstance returns nil when the user can create another instance,
// or ErrInstanceQuotaExceeded wrapped with the current count when the cap is
// reached. limit is the account's effective cap (see EffectiveMaxInstances); a
// non-positive limit falls back to the default so a miswired caller can never
// lock a user out of creating chips entirely.
func CheckCanCreateInstance(ctx context.Context, repo InstanceCounter, userID string, limit int) error {
	if repo == nil {
		return errors.New("quota: instance repository is nil")
	}
	if limit <= 0 {
		limit = MaxInstancesPerUser
	}
	count, err := repo.CountUserInstances(ctx, userID)
	if err != nil {
		return fmt.Errorf("quota: count user instances: %w", err)
	}
	if count >= limit {
		return fmt.Errorf("%w: %d/%d", ErrInstanceQuotaExceeded, count, limit)
	}
	return nil
}

// CheckCanAddGroups returns nil when adding `n` groups to instanceID would not
// exceed MaxGroupsPerInstance. Returns ErrGroupQuotaExceeded otherwise.
// A non-positive n is a no-op.
func CheckCanAddGroups(ctx context.Context, repo GroupCounter, userID string, instanceID int, n int) error {
	if repo == nil {
		return errors.New("quota: group repository is nil")
	}
	if n <= 0 {
		return nil
	}
	count, err := repo.CountGroupsByInstance(ctx, userID, instanceID)
	if err != nil {
		return fmt.Errorf("quota: count groups: %w", err)
	}
	if count+n > MaxGroupsPerInstance {
		return fmt.Errorf("%w: %d existing + %d new > %d limit",
			ErrGroupQuotaExceeded, count, n, MaxGroupsPerInstance)
	}
	return nil
}

// CheckAutomationGroupsPerInstance enforces the per-chip cap inside a single
// automation: no chip may host more than MaxGroupsPerInstance groups within
// the automation's send/monitor list. Caller resolves groups → instance IDs
// and passes a count map; the helper just enforces the threshold so the
// quota package stays free of repository/model imports.
func CheckAutomationGroupsPerInstance(countByInstance map[int]int) error {
	for instanceID, count := range countByInstance {
		if count > MaxGroupsPerInstance {
			return fmt.Errorf("%w: chip id=%d has %d groups in automation, limit is %d",
				ErrGroupQuotaExceeded, instanceID, count, MaxGroupsPerInstance)
		}
	}
	return nil
}
