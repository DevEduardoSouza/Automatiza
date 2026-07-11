package quota_test

import (
	"context"
	"errors"
	"testing"

	"automatiza-go/internal/lib/quota"
)

type fakeInstanceCounter struct {
	count int
	err   error
}

func (f *fakeInstanceCounter) CountUserInstances(ctx context.Context, userID string) (int, error) {
	return f.count, f.err
}

type fakeGroupCounter struct {
	count int
	err   error
}

func (f *fakeGroupCounter) CountGroupsByInstance(ctx context.Context, userID string, instanceID int) (int, error) {
	return f.count, f.err
}

func TestCheckCanCreateInstance(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		count   int
		wantErr error
	}{
		{name: "zero instances allowed", count: 0, wantErr: nil},
		{name: "below cap allowed", count: quota.MaxInstancesPerUser - 1, wantErr: nil},
		{name: "at cap rejected", count: quota.MaxInstancesPerUser, wantErr: quota.ErrInstanceQuotaExceeded},
		{name: "above cap rejected", count: quota.MaxInstancesPerUser + 5, wantErr: quota.ErrInstanceQuotaExceeded},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeInstanceCounter{count: tt.count}
			err := quota.CheckCanCreateInstance(ctx, repo, "user-1", quota.MaxInstancesPerUser)
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("expected nil, got %v", err)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestCheckCanCreateInstance_PropagatesRepoError(t *testing.T) {
	repo := &fakeInstanceCounter{err: errors.New("db down")}
	err := quota.CheckCanCreateInstance(context.Background(), repo, "user-1", quota.MaxInstancesPerUser)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if errors.Is(err, quota.ErrInstanceQuotaExceeded) {
		t.Fatalf("repo error should not surface as quota error: %v", err)
	}
}

func TestCheckCanCreateInstance_NilRepo(t *testing.T) {
	if err := quota.CheckCanCreateInstance(context.Background(), nil, "user-1", quota.MaxInstancesPerUser); err == nil {
		t.Fatal("expected error for nil repo")
	}
}

func TestCheckCanAddGroups(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		existing  int
		toAdd     int
		wantErr   error
	}{
		{name: "n=0 always ok", existing: quota.MaxGroupsPerInstance, toAdd: 0, wantErr: nil},
		{name: "n<0 always ok", existing: quota.MaxGroupsPerInstance, toAdd: -1, wantErr: nil},
		{name: "fits exactly", existing: 0, toAdd: quota.MaxGroupsPerInstance, wantErr: nil},
		{name: "fits below cap", existing: 5, toAdd: 10, wantErr: nil},
		{name: "exceeds by one", existing: quota.MaxGroupsPerInstance, toAdd: 1, wantErr: quota.ErrGroupQuotaExceeded},
		{name: "single add over cap", existing: 19, toAdd: 5, wantErr: quota.ErrGroupQuotaExceeded},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeGroupCounter{count: tt.existing}
			err := quota.CheckCanAddGroups(ctx, repo, "user-1", 42, tt.toAdd)
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("expected nil, got %v", err)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestCheckCanAddGroups_PropagatesRepoError(t *testing.T) {
	repo := &fakeGroupCounter{err: errors.New("db down")}
	err := quota.CheckCanAddGroups(context.Background(), repo, "user-1", 1, 1)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if errors.Is(err, quota.ErrGroupQuotaExceeded) {
		t.Fatalf("repo error should not surface as quota error: %v", err)
	}
}

func TestCheckCanAddGroups_NilRepo(t *testing.T) {
	if err := quota.CheckCanAddGroups(context.Background(), nil, "user-1", 1, 1); err == nil {
		t.Fatal("expected error for nil repo")
	}
}

func TestQuotaConstants(t *testing.T) {
	// Sanity guard: tests downstream rely on these specific values.
	// 15, not 30: the client has always stopped users at 15 while the server
	// allowed 30, and that disagreement is what made "limite atingido"
	// impossible to explain. The two now share one number, and a single account
	// is lifted above it via profiles.max_instances — never by moving this.
	// Keep in sync with DEFAULT_MAX_INSTANCES in client instances-list.tsx.
	if quota.MaxInstancesPerUser != 15 {
		t.Errorf("MaxInstancesPerUser = %d, want 15", quota.MaxInstancesPerUser)
	}
	if quota.MaxGroupsPerInstance != 20 {
		t.Errorf("MaxGroupsPerInstance = %d, want 20", quota.MaxGroupsPerInstance)
	}
}

func TestCheckAutomationGroupsPerInstance(t *testing.T) {
	tests := []struct {
		name             string
		countByInstance  map[int]int
		wantErr          error
	}{
		{name: "empty map ok", countByInstance: map[int]int{}, wantErr: nil},
		{name: "single chip below cap", countByInstance: map[int]int{1: 5}, wantErr: nil},
		{name: "single chip at cap", countByInstance: map[int]int{1: quota.MaxGroupsPerInstance}, wantErr: nil},
		{name: "single chip over cap", countByInstance: map[int]int{1: quota.MaxGroupsPerInstance + 1}, wantErr: quota.ErrGroupQuotaExceeded},
		{name: "two chips both ok", countByInstance: map[int]int{1: 15, 2: 10}, wantErr: nil},
		{name: "two chips one over", countByInstance: map[int]int{1: 15, 2: 25}, wantErr: quota.ErrGroupQuotaExceeded},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := quota.CheckAutomationGroupsPerInstance(tt.countByInstance)
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("expected nil, got %v", err)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}
		})
	}
}

// TestEffectiveMaxInstances_OverrideOnlyRaises locks the property the whole
// per-account upgrade rests on: profiles.max_instances may lift an account's
// cap but must never lower it. The column is full of stale seed values (542
// accounts say 1, 295 say 5) while several of those same accounts legitimately
// run 9-20 chips today — honouring it downward would strand them.
func TestEffectiveMaxInstances_OverrideOnlyRaises(t *testing.T) {
	i := func(n int) *int { return &n }

	cases := []struct {
		name     string
		override *int
		want     int
	}{
		{"no override → default", nil, quota.MaxInstancesPerUser},
		{"stale 1 must NOT downgrade", i(1), quota.MaxInstancesPerUser},
		{"stale 5 must NOT downgrade", i(5), quota.MaxInstancesPerUser},
		{"equal to default → default", i(quota.MaxInstancesPerUser), quota.MaxInstancesPerUser},
		{"zero must NOT lock the account out", i(0), quota.MaxInstancesPerUser},
		{"negative must NOT lock the account out", i(-3), quota.MaxInstancesPerUser},
		{"genuine upgrade wins", i(30), 30},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := quota.EffectiveMaxInstances(tc.override); got != tc.want {
				t.Fatalf("EffectiveMaxInstances(%v) = %d, want %d", tc.override, got, tc.want)
			}
		})
	}
}

// TestCheckCanCreateInstance_HonoursPerAccountLimit: an upgraded account keeps
// creating chips past the point where a default account is cut off.
func TestCheckCanCreateInstance_HonoursPerAccountLimit(t *testing.T) {
	ctx := context.Background()
	atDefault := &fakeInstanceCounter{count: quota.MaxInstancesPerUser}

	// Default account sitting exactly at the cap → blocked.
	err := quota.CheckCanCreateInstance(ctx, atDefault, "u1", quota.MaxInstancesPerUser)
	if !errors.Is(err, quota.ErrInstanceQuotaExceeded) {
		t.Fatalf("default account at cap: want ErrInstanceQuotaExceeded, got %v", err)
	}

	// Same usage, upgraded account → allowed.
	if err := quota.CheckCanCreateInstance(ctx, atDefault, "u1", 30); err != nil {
		t.Fatalf("upgraded account below its cap: want nil, got %v", err)
	}

	// Upgraded account that reached its own higher cap → blocked again.
	atUpgraded := &fakeInstanceCounter{count: 30}
	if err := quota.CheckCanCreateInstance(ctx, atUpgraded, "u1", 30); !errors.Is(err, quota.ErrInstanceQuotaExceeded) {
		t.Fatalf("upgraded account at its cap: want ErrInstanceQuotaExceeded, got %v", err)
	}

	// A miswired caller passing 0 must fall back to the default, not lock out.
	fresh := &fakeInstanceCounter{count: 0}
	if err := quota.CheckCanCreateInstance(ctx, fresh, "u1", 0); err != nil {
		t.Fatalf("limit=0 must fall back to default, got %v", err)
	}
}
