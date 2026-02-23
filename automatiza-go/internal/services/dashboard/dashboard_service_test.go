package dashboard

import (
	"automatiza-go/internal/repository"
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap/zaptest"
)

// ---------------------------------------------------------------------------
// Mock
// ---------------------------------------------------------------------------

type mockDashboardRepo struct {
	getGroupsStatsFn      func(ctx context.Context, userID string) (*repository.DashboardStats, error)
	getActiveLinksCountFn func(ctx context.Context, userID string) (int, error)
	getAnnouncementFn     func(ctx context.Context) (*repository.AnnouncementSettings, error)
	getHistoryStatsFn     func(ctx context.Context, userID string, period repository.HistoryPeriod) ([]repository.HistoryDataPoint, error)
}

func (m *mockDashboardRepo) GetGroupsStats(ctx context.Context, userID string) (*repository.DashboardStats, error) {
	if m.getGroupsStatsFn != nil {
		return m.getGroupsStatsFn(ctx, userID)
	}
	return &repository.DashboardStats{}, nil
}

func (m *mockDashboardRepo) GetActiveLinksCount(ctx context.Context, userID string) (int, error) {
	if m.getActiveLinksCountFn != nil {
		return m.getActiveLinksCountFn(ctx, userID)
	}
	return 0, nil
}

func (m *mockDashboardRepo) GetAnnouncementSettings(ctx context.Context) (*repository.AnnouncementSettings, error) {
	if m.getAnnouncementFn != nil {
		return m.getAnnouncementFn(ctx)
	}
	return &repository.AnnouncementSettings{}, nil
}

func (m *mockDashboardRepo) GetHistoryStats(ctx context.Context, userID string, period repository.HistoryPeriod) ([]repository.HistoryDataPoint, error) {
	if m.getHistoryStatsFn != nil {
		return m.getHistoryStatsFn(ctx, userID, period)
	}
	return nil, nil
}

// newTestService creates a dashboardService with a short cache TTL for testing.
func newTestService(t *testing.T, repo *mockDashboardRepo) *dashboardService {
	t.Helper()
	svc := NewDashboardService(repo, zaptest.NewLogger(t))
	return svc.(*dashboardService)
}

// ---------------------------------------------------------------------------
// GetStats
// ---------------------------------------------------------------------------

func TestGetStats_Success(t *testing.T) {
	t.Parallel()

	var groupsCalled, linksCalled atomic.Int32

	repo := &mockDashboardRepo{
		getGroupsStatsFn: func(_ context.Context, userID string) (*repository.DashboardStats, error) {
			groupsCalled.Add(1)
			if userID != "user-1" {
				t.Errorf("unexpected userID: %s", userID)
			}
			return &repository.DashboardStats{
				TotalGroups:  10,
				FullGroups:   3,
				TotalMembers: 500,
			}, nil
		},
		getActiveLinksCountFn: func(_ context.Context, userID string) (int, error) {
			linksCalled.Add(1)
			if userID != "user-1" {
				t.Errorf("unexpected userID: %s", userID)
			}
			return 7, nil
		},
	}

	svc := newTestService(t, repo)
	stats, err := svc.GetStats(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify both queries were called (parallel execution)
	if groupsCalled.Load() != 1 {
		t.Errorf("expected GetGroupsStats called once, got %d", groupsCalled.Load())
	}
	if linksCalled.Load() != 1 {
		t.Errorf("expected GetActiveLinksCount called once, got %d", linksCalled.Load())
	}

	if stats.TotalGroups != 10 {
		t.Errorf("TotalGroups = %d, want 10", stats.TotalGroups)
	}
	if stats.FullGroups != 3 {
		t.Errorf("FullGroups = %d, want 3", stats.FullGroups)
	}
	if stats.TotalMembers != 500 {
		t.Errorf("TotalMembers = %d, want 500", stats.TotalMembers)
	}
	if stats.ActiveLinks != 7 {
		t.Errorf("ActiveLinks = %d, want 7", stats.ActiveLinks)
	}
}

func TestGetStats_GroupsError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("groups query failed")

	repo := &mockDashboardRepo{
		getGroupsStatsFn: func(_ context.Context, _ string) (*repository.DashboardStats, error) {
			return nil, wantErr
		},
		getActiveLinksCountFn: func(_ context.Context, _ string) (int, error) {
			return 5, nil
		},
	}

	svc := newTestService(t, repo)
	stats, err := svc.GetStats(context.Background(), "user-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("expected %v, got %v", wantErr, err)
	}
	if stats != nil {
		t.Errorf("expected nil stats on error, got %+v", stats)
	}
}

func TestGetStats_LinksError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("links query failed")

	repo := &mockDashboardRepo{
		getGroupsStatsFn: func(_ context.Context, _ string) (*repository.DashboardStats, error) {
			return &repository.DashboardStats{TotalGroups: 1}, nil
		},
		getActiveLinksCountFn: func(_ context.Context, _ string) (int, error) {
			return 0, wantErr
		},
	}

	svc := newTestService(t, repo)
	stats, err := svc.GetStats(context.Background(), "user-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("expected %v, got %v", wantErr, err)
	}
	if stats != nil {
		t.Errorf("expected nil stats on error, got %+v", stats)
	}
}

func TestGetStats_ContextCanceled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	repo := &mockDashboardRepo{
		getGroupsStatsFn: func(ctx context.Context, _ string) (*repository.DashboardStats, error) {
			// Simulate slow query that checks context
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
				return &repository.DashboardStats{}, nil
			}
		},
		getActiveLinksCountFn: func(ctx context.Context, _ string) (int, error) {
			select {
			case <-ctx.Done():
				return 0, ctx.Err()
			default:
				return 0, nil
			}
		},
	}

	svc := newTestService(t, repo)
	stats, err := svc.GetStats(ctx, "user-1")
	if err == nil {
		t.Fatal("expected error for canceled context, got nil")
	}
	if stats != nil {
		t.Errorf("expected nil stats, got %+v", stats)
	}
}

func TestGetStats_BothErrors_ReturnsGroupsError(t *testing.T) {
	t.Parallel()

	groupsErr := errors.New("groups failed")

	repo := &mockDashboardRepo{
		getGroupsStatsFn: func(_ context.Context, _ string) (*repository.DashboardStats, error) {
			return nil, groupsErr
		},
		getActiveLinksCountFn: func(_ context.Context, _ string) (int, error) {
			return 0, errors.New("links failed")
		},
	}

	svc := newTestService(t, repo)
	_, err := svc.GetStats(context.Background(), "user-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// The implementation checks groupErr first, so we should get that
	if !errors.Is(err, groupsErr) {
		t.Errorf("expected groupsErr, got: %v", err)
	}
}

func TestGetStats_ZeroValues(t *testing.T) {
	t.Parallel()

	repo := &mockDashboardRepo{
		getGroupsStatsFn: func(_ context.Context, _ string) (*repository.DashboardStats, error) {
			return &repository.DashboardStats{
				TotalGroups:  0,
				FullGroups:   0,
				TotalMembers: 0,
			}, nil
		},
		getActiveLinksCountFn: func(_ context.Context, _ string) (int, error) {
			return 0, nil
		},
	}

	svc := newTestService(t, repo)
	stats, err := svc.GetStats(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stats.TotalGroups != 0 || stats.FullGroups != 0 || stats.TotalMembers != 0 || stats.ActiveLinks != 0 {
		t.Errorf("expected all zeros, got %+v", stats)
	}
}

// ---------------------------------------------------------------------------
// GetAnnouncement
// ---------------------------------------------------------------------------

func TestGetAnnouncement_Success(t *testing.T) {
	t.Parallel()

	var callCount atomic.Int32

	repo := &mockDashboardRepo{
		getAnnouncementFn: func(_ context.Context) (*repository.AnnouncementSettings, error) {
			callCount.Add(1)
			return &repository.AnnouncementSettings{
				Enabled: true,
				Title:   "System Update",
				Body:    "Scheduled maintenance tonight.",
			}, nil
		},
	}

	svc := newTestService(t, repo)
	ann, err := svc.GetAnnouncement(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !ann.Enabled {
		t.Error("expected Enabled=true")
	}
	if ann.Title != "System Update" {
		t.Errorf("Title = %q, want %q", ann.Title, "System Update")
	}
	if ann.Body != "Scheduled maintenance tonight." {
		t.Errorf("Body = %q, want %q", ann.Body, "Scheduled maintenance tonight.")
	}
	if callCount.Load() != 1 {
		t.Errorf("expected repo called once, got %d", callCount.Load())
	}
}

func TestGetAnnouncement_CacheHit(t *testing.T) {
	t.Parallel()

	var callCount atomic.Int32

	repo := &mockDashboardRepo{
		getAnnouncementFn: func(_ context.Context) (*repository.AnnouncementSettings, error) {
			callCount.Add(1)
			return &repository.AnnouncementSettings{
				Enabled: true,
				Title:   "Announcement",
				Body:    "Body text",
			}, nil
		},
	}

	svc := newTestService(t, repo)

	// First call fetches from repo
	ann1, err := svc.GetAnnouncement(context.Background())
	if err != nil {
		t.Fatalf("first call: unexpected error: %v", err)
	}

	// Second call should return cached data
	ann2, err := svc.GetAnnouncement(context.Background())
	if err != nil {
		t.Fatalf("second call: unexpected error: %v", err)
	}

	if callCount.Load() != 1 {
		t.Errorf("expected repo called once (cache hit), got %d", callCount.Load())
	}

	if ann1.Title != ann2.Title || ann1.Body != ann2.Body || ann1.Enabled != ann2.Enabled {
		t.Errorf("cached data mismatch: first=%+v, second=%+v", ann1, ann2)
	}
}

func TestGetAnnouncement_CacheExpiry(t *testing.T) {
	t.Parallel()

	var callCount atomic.Int32

	repo := &mockDashboardRepo{
		getAnnouncementFn: func(_ context.Context) (*repository.AnnouncementSettings, error) {
			callCount.Add(1)
			return &repository.AnnouncementSettings{
				Enabled: true,
				Title:   "Announcement",
				Body:    "Body text",
			}, nil
		},
	}

	svc := newTestService(t, repo)
	// Set a very short TTL for testing cache expiry
	svc.announcementCache.ttl = 1 * time.Millisecond

	// First call
	_, err := svc.GetAnnouncement(context.Background())
	if err != nil {
		t.Fatalf("first call: unexpected error: %v", err)
	}

	// Wait for cache to expire
	time.Sleep(5 * time.Millisecond)

	// Second call should fetch from repo again
	_, err = svc.GetAnnouncement(context.Background())
	if err != nil {
		t.Fatalf("second call: unexpected error: %v", err)
	}

	if callCount.Load() != 2 {
		t.Errorf("expected repo called twice (cache expired), got %d", callCount.Load())
	}
}

func TestGetAnnouncement_RepoError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("db connection failed")

	repo := &mockDashboardRepo{
		getAnnouncementFn: func(_ context.Context) (*repository.AnnouncementSettings, error) {
			return nil, wantErr
		},
	}

	svc := newTestService(t, repo)
	ann, err := svc.GetAnnouncement(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("expected %v, got %v", wantErr, err)
	}
	if ann != nil {
		t.Errorf("expected nil announcement on error, got %+v", ann)
	}
}

func TestGetAnnouncement_ErrorDoesNotCache(t *testing.T) {
	t.Parallel()

	var callCount atomic.Int32
	shouldFail := true

	repo := &mockDashboardRepo{
		getAnnouncementFn: func(_ context.Context) (*repository.AnnouncementSettings, error) {
			callCount.Add(1)
			if shouldFail {
				shouldFail = false
				return nil, errors.New("transient error")
			}
			return &repository.AnnouncementSettings{
				Enabled: false,
				Title:   "OK",
				Body:    "Recovered",
			}, nil
		},
	}

	svc := newTestService(t, repo)

	// First call fails
	_, err := svc.GetAnnouncement(context.Background())
	if err == nil {
		t.Fatal("expected error on first call")
	}

	// Second call should succeed (repo called again, not cached)
	ann, err := svc.GetAnnouncement(context.Background())
	if err != nil {
		t.Fatalf("expected success on second call, got: %v", err)
	}
	if ann.Title != "OK" {
		t.Errorf("Title = %q, want %q", ann.Title, "OK")
	}
	if callCount.Load() != 2 {
		t.Errorf("expected repo called twice, got %d", callCount.Load())
	}
}

func TestGetAnnouncement_Disabled(t *testing.T) {
	t.Parallel()

	repo := &mockDashboardRepo{
		getAnnouncementFn: func(_ context.Context) (*repository.AnnouncementSettings, error) {
			return &repository.AnnouncementSettings{
				Enabled: false,
				Title:   "",
				Body:    "",
			}, nil
		},
	}

	svc := newTestService(t, repo)
	ann, err := svc.GetAnnouncement(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ann.Enabled {
		t.Error("expected Enabled=false")
	}
}

// ---------------------------------------------------------------------------
// GetHistory
// ---------------------------------------------------------------------------

func TestGetHistory_Success(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		period     repository.HistoryPeriod
		repoData   []repository.HistoryDataPoint
		wantLen    int
		wantFirst  HistoryDataPoint
	}{
		{
			name:   "weekly period with data",
			period: repository.HistoryPeriodWeekly,
			repoData: []repository.HistoryDataPoint{
				{Label: "Mon", Members: 100, Groups: 5, NewMembers: 10, NewGroups: 1},
				{Label: "Tue", Members: 110, Groups: 6, NewMembers: 15, NewGroups: 2},
				{Label: "Wed", Members: 120, Groups: 6, NewMembers: 5, NewGroups: 0},
			},
			wantLen:   3,
			wantFirst: HistoryDataPoint{Label: "Mon", Members: 100, Groups: 5, NewMembers: 10, NewGroups: 1},
		},
		{
			name:   "monthly period with data",
			period: repository.HistoryPeriodMonthly,
			repoData: []repository.HistoryDataPoint{
				{Label: "Week 1", Members: 500, Groups: 20, NewMembers: 50, NewGroups: 5},
			},
			wantLen:   1,
			wantFirst: HistoryDataPoint{Label: "Week 1", Members: 500, Groups: 20, NewMembers: 50, NewGroups: 5},
		},
		{
			name:   "yearly period with data",
			period: repository.HistoryPeriodYearly,
			repoData: []repository.HistoryDataPoint{
				{Label: "Jan", Members: 1000, Groups: 50, NewMembers: 100, NewGroups: 10},
				{Label: "Feb", Members: 1200, Groups: 55, NewMembers: 200, NewGroups: 5},
			},
			wantLen:   2,
			wantFirst: HistoryDataPoint{Label: "Jan", Members: 1000, Groups: 50, NewMembers: 100, NewGroups: 10},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := &mockDashboardRepo{
				getHistoryStatsFn: func(_ context.Context, userID string, period repository.HistoryPeriod) ([]repository.HistoryDataPoint, error) {
					if userID != "user-1" {
						t.Errorf("unexpected userID: %s", userID)
					}
					if period != tt.period {
						t.Errorf("period = %q, want %q", period, tt.period)
					}
					return tt.repoData, nil
				},
			}

			svc := newTestService(t, repo)
			result, err := svc.GetHistory(context.Background(), "user-1", tt.period)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(result) != tt.wantLen {
				t.Fatalf("len(result) = %d, want %d", len(result), tt.wantLen)
			}
			if result[0] != tt.wantFirst {
				t.Errorf("first data point = %+v, want %+v", result[0], tt.wantFirst)
			}
		})
	}
}

func TestGetHistory_EmptyData(t *testing.T) {
	t.Parallel()

	repo := &mockDashboardRepo{
		getHistoryStatsFn: func(_ context.Context, _ string, _ repository.HistoryPeriod) ([]repository.HistoryDataPoint, error) {
			return []repository.HistoryDataPoint{}, nil
		},
	}

	svc := newTestService(t, repo)
	result, err := svc.GetHistory(context.Background(), "user-1", repository.HistoryPeriodWeekly)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty result, got %d items", len(result))
	}
}

func TestGetHistory_NilData(t *testing.T) {
	t.Parallel()

	repo := &mockDashboardRepo{
		getHistoryStatsFn: func(_ context.Context, _ string, _ repository.HistoryPeriod) ([]repository.HistoryDataPoint, error) {
			return nil, nil
		},
	}

	svc := newTestService(t, repo)
	result, err := svc.GetHistory(context.Background(), "user-1", repository.HistoryPeriodMonthly)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty result for nil data, got %d items", len(result))
	}
}

func TestGetHistory_RepoError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("history query failed")

	repo := &mockDashboardRepo{
		getHistoryStatsFn: func(_ context.Context, _ string, _ repository.HistoryPeriod) ([]repository.HistoryDataPoint, error) {
			return nil, wantErr
		},
	}

	svc := newTestService(t, repo)
	result, err := svc.GetHistory(context.Background(), "user-1", repository.HistoryPeriodWeekly)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("expected %v, got %v", wantErr, err)
	}
	if result != nil {
		t.Errorf("expected nil result on error, got %+v", result)
	}
}

func TestGetHistory_DataConversion(t *testing.T) {
	t.Parallel()

	// Verify int -> int32 conversion works for large values
	repo := &mockDashboardRepo{
		getHistoryStatsFn: func(_ context.Context, _ string, _ repository.HistoryPeriod) ([]repository.HistoryDataPoint, error) {
			return []repository.HistoryDataPoint{
				{Label: "Large", Members: 100000, Groups: 50000, NewMembers: 25000, NewGroups: 12000},
			}, nil
		},
	}

	svc := newTestService(t, repo)
	result, err := svc.GetHistory(context.Background(), "user-1", repository.HistoryPeriodYearly)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 data point, got %d", len(result))
	}
	dp := result[0]
	if dp.Members != 100000 {
		t.Errorf("Members = %d, want 100000", dp.Members)
	}
	if dp.Groups != 50000 {
		t.Errorf("Groups = %d, want 50000", dp.Groups)
	}
	if dp.NewMembers != 25000 {
		t.Errorf("NewMembers = %d, want 25000", dp.NewMembers)
	}
	if dp.NewGroups != 12000 {
		t.Errorf("NewGroups = %d, want 12000", dp.NewGroups)
	}
}

// ---------------------------------------------------------------------------
// Service constructor
// ---------------------------------------------------------------------------

func TestNewDashboardService(t *testing.T) {
	t.Parallel()

	repo := &mockDashboardRepo{}
	logger := zaptest.NewLogger(t)
	svc := NewDashboardService(repo, logger)

	if svc == nil {
		t.Fatal("expected non-nil service")
	}

	// Verify the service implements the Service interface
	var _ Service = svc
}

func TestNewDashboardService_CacheInitialized(t *testing.T) {
	t.Parallel()

	repo := &mockDashboardRepo{}
	logger := zaptest.NewLogger(t)
	svc := NewDashboardService(repo, logger).(*dashboardService)

	if svc.announcementCache == nil {
		t.Fatal("expected announcementCache to be initialized")
	}
	if svc.announcementCache.ttl != 5*time.Minute {
		t.Errorf("cache TTL = %v, want %v", svc.announcementCache.ttl, 5*time.Minute)
	}
	if svc.announcementCache.data != nil {
		t.Error("expected cache data to be nil initially")
	}
}
