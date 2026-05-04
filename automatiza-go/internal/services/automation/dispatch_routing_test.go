package automation

import (
	"testing"

	"automatiza-go/internal/config"
	"automatiza-go/internal/models"
)

func newConnectedInstance(id int, waName string) *models.Instance {
	return &models.Instance{
		ID:         id,
		InstanceID: waName,
		Status:     models.InstanceStatusConnected,
	}
}

func newDisconnectedInstance(id int, waName string) *models.Instance {
	return &models.Instance{
		ID:         id,
		InstanceID: waName,
		Status:     models.InstanceStatusDisconnected,
	}
}

func TestResolveSenderForGroup_FlagOff_AlwaysUsesFallback(t *testing.T) {
	defer config.ResetFeatureFlags()
	t.Setenv("FEATURE_USE_MULTI_INSTANCE_DISPATCH", "false")
	config.ResetFeatureFlags()
	config.LoadFeatureFlags()

	svc := &automationService{}
	fallback := newConnectedInstance(1, "fallback-chip")
	groupInstances := map[string]*models.Instance{
		"group-1@g.us": newConnectedInstance(2, "owner-chip"),
	}

	got := svc.resolveSenderForGroup(fallback, groupInstances, "group-1@g.us")
	if got != "fallback-chip" {
		t.Errorf("flag off: expected fallback-chip, got %q", got)
	}
}

func TestResolveSenderForGroup_FlagOn_PrefersOwningChipWhenConnected(t *testing.T) {
	defer config.ResetFeatureFlags()
	t.Setenv("FEATURE_USE_MULTI_INSTANCE_DISPATCH", "true")
	config.ResetFeatureFlags()
	config.LoadFeatureFlags()

	svc := &automationService{}
	fallback := newConnectedInstance(1, "fallback-chip")
	groupInstances := map[string]*models.Instance{
		"group-1@g.us": newConnectedInstance(2, "owner-chip"),
	}

	got := svc.resolveSenderForGroup(fallback, groupInstances, "group-1@g.us")
	if got != "owner-chip" {
		t.Errorf("flag on, owner connected: expected owner-chip, got %q", got)
	}
}

func TestResolveSenderForGroup_FlagOn_FallsBackWhenOwnerDisconnected(t *testing.T) {
	defer config.ResetFeatureFlags()
	t.Setenv("FEATURE_USE_MULTI_INSTANCE_DISPATCH", "true")
	config.ResetFeatureFlags()
	config.LoadFeatureFlags()

	svc := &automationService{}
	fallback := newConnectedInstance(1, "fallback-chip")
	groupInstances := map[string]*models.Instance{
		"group-1@g.us": newDisconnectedInstance(2, "owner-chip"),
	}

	got := svc.resolveSenderForGroup(fallback, groupInstances, "group-1@g.us")
	if got != "fallback-chip" {
		t.Errorf("flag on, owner disconnected: expected fallback, got %q", got)
	}
}

func TestResolveSenderForGroup_FlagOn_FallsBackWhenGroupMissingFromMap(t *testing.T) {
	defer config.ResetFeatureFlags()
	t.Setenv("FEATURE_USE_MULTI_INSTANCE_DISPATCH", "true")
	config.ResetFeatureFlags()
	config.LoadFeatureFlags()

	svc := &automationService{}
	fallback := newConnectedInstance(1, "fallback-chip")

	got := svc.resolveSenderForGroup(fallback, map[string]*models.Instance{}, "missing@g.us")
	if got != "fallback-chip" {
		t.Errorf("flag on, group missing: expected fallback, got %q", got)
	}
}

func TestResolveSenderForGroup_NilFallback_ReturnsEmptyWhenOwnerDisconnected(t *testing.T) {
	defer config.ResetFeatureFlags()
	t.Setenv("FEATURE_USE_MULTI_INSTANCE_DISPATCH", "true")
	config.ResetFeatureFlags()
	config.LoadFeatureFlags()

	svc := &automationService{}
	groupInstances := map[string]*models.Instance{
		"group-1@g.us": newDisconnectedInstance(2, "owner-chip"),
	}

	got := svc.resolveSenderForGroup(nil, groupInstances, "group-1@g.us")
	if got != "" {
		t.Errorf("nil fallback + disconnected owner: expected empty, got %q", got)
	}
}

func TestResolveSenderForGroup_NilGroupInstances_UsesFallback(t *testing.T) {
	defer config.ResetFeatureFlags()
	t.Setenv("FEATURE_USE_MULTI_INSTANCE_DISPATCH", "true")
	config.ResetFeatureFlags()
	config.LoadFeatureFlags()

	svc := &automationService{}
	fallback := newConnectedInstance(1, "fallback-chip")

	got := svc.resolveSenderForGroup(fallback, nil, "group-1@g.us")
	if got != "fallback-chip" {
		t.Errorf("nil groupInstances: expected fallback, got %q", got)
	}
}
