package connectrpc

import (
	"context"
	"errors"
	"testing"

	"automatiza-go/internal/config"
	"automatiza-go/internal/lib/quota"
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"
)

// enableGroupLimits flips the FEATURE_ENFORCE_GROUP_LIMITS feature flag on
// for the duration of a test. Needed because the default is off; without it,
// validateAutomationGroupQuota returns nil unconditionally and the per-chip
// behavior these tests assert can't be observed.
func enableGroupLimits(t *testing.T) {
	t.Helper()
	t.Setenv("FEATURE_ENFORCE_GROUP_LIMITS", "true")
	config.ResetFeatureFlags()
	t.Cleanup(config.ResetFeatureFlags)
}

// fakeQuotaGroupRepo is a minimal GroupRepository stub focused on
// GetInstancesForGroups for the per-chip quota tests below. Other methods
// panic if called so accidental coupling surfaces immediately.
type fakeQuotaGroupRepo struct {
	mapping map[string]*models.Instance
	err     error
}

func (f *fakeQuotaGroupRepo) GetInstancesForGroups(ctx context.Context, userID string, waGroupIDs []string) (map[string]*models.Instance, error) {
	if f.err != nil {
		return nil, f.err
	}
	out := make(map[string]*models.Instance, len(waGroupIDs))
	for _, gid := range waGroupIDs {
		if inst, ok := f.mapping[gid]; ok {
			out[gid] = inst
		}
	}
	return out, nil
}

// Methods below are not exercised by the quota tests; they panic so any
// accidental call is loud rather than silently returning zero values.
func (f *fakeQuotaGroupRepo) GetUserGroups(context.Context, string, bool) ([]models.Group, error) {
	panic("GetUserGroups not implemented in fakeQuotaGroupRepo")
}
func (f *fakeQuotaGroupRepo) GetUserGroupsPaginated(context.Context, string, int, int, bool, string) (*repository.GroupPaginatedResult, error) {
	panic("GetUserGroupsPaginated not implemented in fakeQuotaGroupRepo")
}
func (f *fakeQuotaGroupRepo) GetGroupByID(context.Context, string) (*models.Group, error) {
	panic("GetGroupByID not implemented in fakeQuotaGroupRepo")
}
func (f *fakeQuotaGroupRepo) GetGroupByWAGroupID(context.Context, string, string) (*models.Group, error) {
	panic("GetGroupByWAGroupID not implemented in fakeQuotaGroupRepo")
}
func (f *fakeQuotaGroupRepo) GetGroupsByWAGroupIDs(context.Context, string, []string) ([]models.Group, error) {
	return nil, nil
}
func (f *fakeQuotaGroupRepo) GetGroupsByIDs(context.Context, string, []string) ([]models.Group, error) {
	panic("GetGroupsByIDs not implemented in fakeQuotaGroupRepo")
}
func (f *fakeQuotaGroupRepo) GetFavoriteGroups(context.Context, string) ([]models.Group, error) {
	panic("GetFavoriteGroups not implemented in fakeQuotaGroupRepo")
}
func (f *fakeQuotaGroupRepo) CreateGroup(context.Context, string, *models.Group) (*models.Group, error) {
	panic("CreateGroup not implemented in fakeQuotaGroupRepo")
}
func (f *fakeQuotaGroupRepo) SyncGroups(context.Context, string, int, []models.Group, repository.SyncOptions) error {
	panic("SyncGroups not implemented in fakeQuotaGroupRepo")
}
func (f *fakeQuotaGroupRepo) SyncGroupsWithStats(context.Context, string, int, []models.Group, repository.SyncOptions) (*repository.SyncResult, error) {
	panic("SyncGroupsWithStats not implemented in fakeQuotaGroupRepo")
}
func (f *fakeQuotaGroupRepo) IsUserAdminInGroup(context.Context, string, string) (bool, error) {
	panic("IsUserAdminInGroup not implemented in fakeQuotaGroupRepo")
}
func (f *fakeQuotaGroupRepo) UpdateGroup(context.Context, string, map[string]interface{}) error {
	panic("UpdateGroup not implemented in fakeQuotaGroupRepo")
}
func (f *fakeQuotaGroupRepo) ToggleFavorite(context.Context, string, string, bool) error {
	panic("ToggleFavorite not implemented in fakeQuotaGroupRepo")
}
func (f *fakeQuotaGroupRepo) DeleteGroup(context.Context, string, string) error {
	panic("DeleteGroup not implemented in fakeQuotaGroupRepo")
}
func (f *fakeQuotaGroupRepo) DeleteGroupsByInstance(context.Context, string, int) error {
	panic("DeleteGroupsByInstance not implemented in fakeQuotaGroupRepo")
}
func (f *fakeQuotaGroupRepo) BulkUpdateGroups(context.Context, string, []string, map[string]interface{}) (int64, error) {
	panic("BulkUpdateGroups not implemented in fakeQuotaGroupRepo")
}
func (f *fakeQuotaGroupRepo) CountAll(context.Context) (int64, error) {
	panic("CountAll not implemented in fakeQuotaGroupRepo")
}
func (f *fakeQuotaGroupRepo) CountGroupsByInstance(context.Context, string, int) (int, error) {
	panic("CountGroupsByInstance not implemented in fakeQuotaGroupRepo")
}

func makeGroups(count int, instanceID int) (groups []string, mapping map[string]*models.Instance) {
	mapping = map[string]*models.Instance{}
	inst := &models.Instance{ID: instanceID, InstanceID: "chip"}
	for i := 0; i < count; i++ {
		gid := groupID(i)
		groups = append(groups, gid)
		mapping[gid] = inst
	}
	return groups, mapping
}

func groupID(i int) string {
	return "group-" + itoa(i) + "@g.us"
}

func itoa(i int) string {
	if i < 10 {
		return string(rune('0' + i))
	}
	return string(rune('0'+i/10)) + string(rune('0'+i%10))
}

func TestValidateAutomationGroupQuota_NilRepoIsNoOp(t *testing.T) {
	h := &AutomationsHandler{} // groupRepo nil
	groups := []string{"a@g.us", "b@g.us"}
	if err := h.validateAutomationGroupQuota(context.Background(), "user-1", groups, "send_groups"); err != nil {
		t.Fatalf("expected nil with nil repo, got %v", err)
	}
}

func TestValidateAutomationGroupQuota_EmptyListIsNoOp(t *testing.T) {
	h := &AutomationsHandler{groupRepo: &fakeQuotaGroupRepo{}}
	if err := h.validateAutomationGroupQuota(context.Background(), "user-1", nil, "send_groups"); err != nil {
		t.Fatalf("expected nil for empty list, got %v", err)
	}
}

func TestValidateAutomationGroupQuota_BelowCapAllowed(t *testing.T) {
	enableGroupLimits(t)
	groups, mapping := makeGroups(quota.MaxGroupsPerInstance, 42)
	h := &AutomationsHandler{groupRepo: &fakeQuotaGroupRepo{mapping: mapping}}
	if err := h.validateAutomationGroupQuota(context.Background(), "user-1", groups, "send_groups"); err != nil {
		t.Fatalf("expected nil at cap, got %v", err)
	}
}

func TestValidateAutomationGroupQuota_OverCapRejected(t *testing.T) {
	enableGroupLimits(t)
	groups, mapping := makeGroups(quota.MaxGroupsPerInstance+1, 42)
	h := &AutomationsHandler{groupRepo: &fakeQuotaGroupRepo{mapping: mapping}}
	err := h.validateAutomationGroupQuota(context.Background(), "user-1", groups, "send_groups")
	if !errors.Is(err, quota.ErrGroupQuotaExceeded) {
		t.Fatalf("expected ErrGroupQuotaExceeded, got %v", err)
	}
}

func TestValidateAutomationGroupQuota_TwoChipsBothBelowCap(t *testing.T) {
	enableGroupLimits(t)
	mapping := map[string]*models.Instance{
		"a@g.us": {ID: 1},
		"b@g.us": {ID: 1},
		"c@g.us": {ID: 2},
		"d@g.us": {ID: 2},
	}
	h := &AutomationsHandler{groupRepo: &fakeQuotaGroupRepo{mapping: mapping}}
	groups := []string{"a@g.us", "b@g.us", "c@g.us", "d@g.us"}
	if err := h.validateAutomationGroupQuota(context.Background(), "user-1", groups, "send_groups"); err != nil {
		t.Fatalf("expected nil with two chips below cap, got %v", err)
	}
}

func TestValidateAutomationGroupQuota_GroupsNotInMapAreIgnored(t *testing.T) {
	enableGroupLimits(t)
	// Groups with no matching record (e.g. user removed the chip) simply
	// do not contribute to any chip's count. They neither pass nor fail
	// the check; the validation focuses on chips that *are* resolvable.
	mapping := map[string]*models.Instance{
		"resolved@g.us": {ID: 1},
	}
	h := &AutomationsHandler{groupRepo: &fakeQuotaGroupRepo{mapping: mapping}}
	groups := []string{"resolved@g.us", "missing@g.us"}
	if err := h.validateAutomationGroupQuota(context.Background(), "user-1", groups, "send_groups"); err != nil {
		t.Fatalf("expected nil ignoring missing groups, got %v", err)
	}
}

func TestValidateAutomationGroupQuota_RepoErrorPropagates(t *testing.T) {
	enableGroupLimits(t)
	h := &AutomationsHandler{groupRepo: &fakeQuotaGroupRepo{err: errors.New("db down")}}
	err := h.validateAutomationGroupQuota(context.Background(), "user-1", []string{"a@g.us"}, "send_groups")
	if err == nil {
		t.Fatal("expected error to propagate")
	}
	if errors.Is(err, quota.ErrGroupQuotaExceeded) {
		t.Fatalf("repo error should not be classified as quota error: %v", err)
	}
}

func TestValidateAutomationGroupQuota_FlagOffSkipsCap(t *testing.T) {
	// Default (flag off) — even an over-cap payload must return nil.
	t.Setenv("FEATURE_ENFORCE_GROUP_LIMITS", "false")
	config.ResetFeatureFlags()
	t.Cleanup(config.ResetFeatureFlags)
	groups, mapping := makeGroups(quota.MaxGroupsPerInstance+5, 42)
	h := &AutomationsHandler{groupRepo: &fakeQuotaGroupRepo{mapping: mapping}}
	if err := h.validateAutomationGroupQuota(context.Background(), "user-1", groups, "send_groups"); err != nil {
		t.Fatalf("expected nil with flag off, got %v", err)
	}
}
