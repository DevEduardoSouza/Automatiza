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
	// MaxInstancesPerUser caps how many WhatsApp chips a single user can connect.
	// Raised 10→15 (#191) and 15→30 for high-volume operators who run many chips.
	// Each chip is a WAHA session; the WAHA engine (GOWS) is multi-core and the
	// box has ample headroom, so 30 is comfortably within capacity.
	MaxInstancesPerUser = 30

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

// CheckCanCreateInstance returns nil when the user can create another instance,
// or ErrInstanceQuotaExceeded wrapped with the current count when the cap is reached.
func CheckCanCreateInstance(ctx context.Context, repo InstanceCounter, userID string) error {
	if repo == nil {
		return errors.New("quota: instance repository is nil")
	}
	count, err := repo.CountUserInstances(ctx, userID)
	if err != nil {
		return fmt.Errorf("quota: count user instances: %w", err)
	}
	if count >= MaxInstancesPerUser {
		return fmt.Errorf("%w: %d/%d", ErrInstanceQuotaExceeded, count, MaxInstancesPerUser)
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
