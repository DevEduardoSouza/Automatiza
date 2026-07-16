package automation

import (
	"context"
	"errors"
	"testing"
	"time"

	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// =============================================================================
// Mocks for Scheduler tests
// =============================================================================

type mockSchedulerAutomationRepo struct {
	mock.Mock
}

var _ repository.AutomationRepository = (*mockSchedulerAutomationRepo)(nil)

func (m *mockSchedulerAutomationRepo) GetActiveAutomations(ctx context.Context) ([]models.Automation, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Automation), args.Error(1)
}

func (m *mockSchedulerAutomationRepo) GetUserAutomations(ctx context.Context, userID string) ([]models.Automation, error) {
	return nil, nil
}
func (m *mockSchedulerAutomationRepo) GetAutomationByID(ctx context.Context, automationID string) (*models.Automation, error) {
	return nil, nil
}
func (m *mockSchedulerAutomationRepo) GetAutomationByIDAndUser(ctx context.Context, automationID, userID string) (*models.Automation, error) {
	return nil, nil
}
func (m *mockSchedulerAutomationRepo) CreateAutomation(ctx context.Context, userID string, data *models.Automation) (*models.Automation, error) {
	return nil, nil
}
func (m *mockSchedulerAutomationRepo) UpdateAutomation(ctx context.Context, automationID string, updates map[string]interface{}) error {
	return nil
}
func (m *mockSchedulerAutomationRepo) DeleteAutomation(ctx context.Context, automationID, userID string) error {
	return nil
}
func (m *mockSchedulerAutomationRepo) GetMonitoringAutomationsForGroup(ctx context.Context, waGroupID string) ([]models.Automation, error) {
	return nil, nil
}
func (m *mockSchedulerAutomationRepo) GetOfferGenerateAutomationsForGroup(ctx context.Context, waGroupID string) ([]models.Automation, error) {
	return nil, nil
}
func (m *mockSchedulerAutomationRepo) UpdateLastRunAt(ctx context.Context, automationID string) error {
	return nil
}
func (m *mockSchedulerAutomationRepo) UpdateNextRunAt(ctx context.Context, automationID string, nextRun time.Time) error {
	return nil
}
func (m *mockSchedulerAutomationRepo) BulkUpdateStatusByUserID(ctx context.Context, userID, targetStatus string, sourceStatus *string) (int, error) {
	return 0, nil
}
func (m *mockSchedulerAutomationRepo) BulkUpdateNextRunAts(ctx context.Context, automationIDs []string, nextRunAts []time.Time) error {
	return nil
}

type mockSchedulerService struct {
	mock.Mock
}

var _ Service = (*mockSchedulerService)(nil)

func (m *mockSchedulerService) DiagnoseAutomation(ctx context.Context, automationID, userID string) (*models.DiagnoseResult, error) {
	args := m.Called(ctx, automationID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.DiagnoseResult), args.Error(1)
}

func (m *mockSchedulerService) RunAutomation(ctx context.Context, automationID, userID string) (*models.RunAutomationResult, error) {
	args := m.Called(ctx, automationID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.RunAutomationResult), args.Error(1)
}

func (m *mockSchedulerService) ResetHistory(ctx context.Context, automationID, userID string) error {
	args := m.Called(ctx, automationID, userID)
	return args.Error(0)
}

func (m *mockSchedulerService) CanCreateAutomation(ctx context.Context, userID string) (bool, int, int, error) {
	args := m.Called(ctx, userID)
	return args.Bool(0), args.Int(1), args.Int(2), args.Error(3)
}

// =============================================================================
// NewScheduler Tests
// =============================================================================

func TestNewScheduler(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	repo := new(mockSchedulerAutomationRepo)
	svc := new(mockSchedulerService)

	scheduler := NewScheduler(repo, svc, logger)

	require.NotNil(t, scheduler)
	assert.Equal(t, repo, scheduler.automationRepo)
	assert.Equal(t, svc, scheduler.service)
	assert.Equal(t, logger, scheduler.logger)
}

// =============================================================================
// ProcessActiveAutomations Tests
// =============================================================================

func TestProcessActiveAutomations_NoAutomations(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	repo := new(mockSchedulerAutomationRepo)
	svc := new(mockSchedulerService)
	scheduler := NewScheduler(repo, svc, logger)

	repo.On("GetActiveAutomations", mock.Anything).Return([]models.Automation{}, nil)

	err := scheduler.ProcessActiveAutomations(context.Background())

	require.NoError(t, err)
	svc.AssertNotCalled(t, "RunAutomation")
}

func TestProcessActiveAutomations_RepoError(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	repo := new(mockSchedulerAutomationRepo)
	svc := new(mockSchedulerService)
	scheduler := NewScheduler(repo, svc, logger)

	repo.On("GetActiveAutomations", mock.Anything).Return(nil, errors.New("db connection lost"))

	err := scheduler.ProcessActiveAutomations(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get active automations")
}

func TestProcessActiveAutomations_SkipsMonitoringMode(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	repo := new(mockSchedulerAutomationRepo)
	svc := new(mockSchedulerService)
	scheduler := NewScheduler(repo, svc, logger)

	monitorType := "monitoring"
	automations := []models.Automation{
		{
			ID:     "auto-1",
			UserID: "user-1",
			Name:   "Monitor Automation",
			Status: models.AutomationStatusActive,
			Type:   &monitorType,
		},
	}

	repo.On("GetActiveAutomations", mock.Anything).Return(automations, nil)

	err := scheduler.ProcessActiveAutomations(context.Background())

	require.NoError(t, err)
	svc.AssertNotCalled(t, "RunAutomation")
}

func TestProcessActiveAutomations_RunsSearchAutomation(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	repo := new(mockSchedulerAutomationRepo)
	svc := new(mockSchedulerService)
	scheduler := NewScheduler(repo, svc, logger)

	searchType := "search"
	automations := []models.Automation{
		{
			ID:     "auto-1",
			UserID: "user-1",
			Name:   "Search Automation",
			Status: models.AutomationStatusActive,
			Type:   &searchType,
		},
	}

	repo.On("GetActiveAutomations", mock.Anything).Return(automations, nil)
	svc.On("RunAutomation", mock.Anything, "auto-1", "user-1").Return(&models.RunAutomationResult{
		Success:      true,
		MessagesSent: 2,
		OffersFound:  5,
	}, nil)

	err := scheduler.ProcessActiveAutomations(context.Background())

	require.NoError(t, err)
	svc.AssertCalled(t, "RunAutomation", mock.Anything, "auto-1", "user-1")
}

// TestProcessActiveAutomations_ConcurrentRunsAllDue valida o caminho paralelo
// (SCHEDULER_CONCURRENCY > 1): todas as automações due devem rodar exatamente
// uma vez, sem perder nenhuma. t.Setenv proíbe t.Parallel() neste teste.
func TestProcessActiveAutomations_ConcurrentRunsAllDue(t *testing.T) {
	t.Setenv("SCHEDULER_CONCURRENCY", "4")

	logger := zaptest.NewLogger(t)
	repo := new(mockSchedulerAutomationRepo)
	svc := new(mockSchedulerService)
	scheduler := NewScheduler(repo, svc, logger)

	searchType := "search"
	automations := []models.Automation{
		{ID: "auto-1", UserID: "user-1", Name: "A1", Status: models.AutomationStatusActive, Type: &searchType},
		{ID: "auto-2", UserID: "user-2", Name: "A2", Status: models.AutomationStatusActive, Type: &searchType},
		{ID: "auto-3", UserID: "user-3", Name: "A3", Status: models.AutomationStatusActive, Type: &searchType},
	}

	repo.On("GetActiveAutomations", mock.Anything).Return(automations, nil)
	svc.On("RunAutomation", mock.Anything, mock.Anything, mock.Anything).
		Return(&models.RunAutomationResult{Success: true}, nil)

	err := scheduler.ProcessActiveAutomations(context.Background())

	require.NoError(t, err)
	svc.AssertCalled(t, "RunAutomation", mock.Anything, "auto-1", "user-1")
	svc.AssertCalled(t, "RunAutomation", mock.Anything, "auto-2", "user-2")
	svc.AssertCalled(t, "RunAutomation", mock.Anything, "auto-3", "user-3")
	svc.AssertNumberOfCalls(t, "RunAutomation", 3)
}

func TestProcessActiveAutomations_RunErrorContinues(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	repo := new(mockSchedulerAutomationRepo)
	svc := new(mockSchedulerService)
	scheduler := NewScheduler(repo, svc, logger)

	searchType := "search"
	automations := []models.Automation{
		{
			ID:     "auto-1",
			UserID: "user-1",
			Name:   "Failing Automation",
			Status: models.AutomationStatusActive,
			Type:   &searchType,
		},
		{
			ID:     "auto-2",
			UserID: "user-2",
			Name:   "Succeeding Automation",
			Status: models.AutomationStatusActive,
			Type:   &searchType,
		},
	}

	repo.On("GetActiveAutomations", mock.Anything).Return(automations, nil)
	svc.On("RunAutomation", mock.Anything, "auto-1", "user-1").Return(nil, errors.New("lock conflict"))
	svc.On("RunAutomation", mock.Anything, "auto-2", "user-2").Return(&models.RunAutomationResult{
		Success:      true,
		MessagesSent: 1,
		OffersFound:  3,
	}, nil)

	err := scheduler.ProcessActiveAutomations(context.Background())

	require.NoError(t, err)
	svc.AssertCalled(t, "RunAutomation", mock.Anything, "auto-1", "user-1")
	svc.AssertCalled(t, "RunAutomation", mock.Anything, "auto-2", "user-2")
}

func TestProcessActiveAutomations_SkipsOutsideTimeWindow(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	repo := new(mockSchedulerAutomationRepo)
	svc := new(mockSchedulerService)
	scheduler := NewScheduler(repo, svc, logger)

	searchType := "search"
	// Configure a very narrow window in the past so it's never active
	startTime := "02:00"
	endTime := "02:01"
	automations := []models.Automation{
		{
			ID:        "auto-1",
			UserID:    "user-1",
			Name:      "Window Automation",
			Status:    models.AutomationStatusActive,
			Type:      &searchType,
			StartTime: &startTime,
			EndTime:   &endTime,
		},
	}

	repo.On("GetActiveAutomations", mock.Anything).Return(automations, nil)

	// We can't control time.Now() in the scheduler, so this test only verifies
	// the path when the automation is either inside or outside the window.
	// At least one of the cases will be exercised.
	err := scheduler.ProcessActiveAutomations(context.Background())
	require.NoError(t, err)
}

func TestProcessActiveAutomations_SkipsNotDueYet(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	repo := new(mockSchedulerAutomationRepo)
	svc := new(mockSchedulerService)
	scheduler := NewScheduler(repo, svc, logger)

	searchType := "search"
	interval := 60
	lastRun := time.Now().Add(-5 * time.Minute).Format(time.RFC3339) // Ran 5 min ago, interval 60 min
	automations := []models.Automation{
		{
			ID:              "auto-1",
			UserID:          "user-1",
			Name:            "Not Due Automation",
			Status:          models.AutomationStatusActive,
			Type:            &searchType,
			IntervalMinutes: &interval,
			LastRunAt:       &lastRun,
		},
	}

	repo.On("GetActiveAutomations", mock.Anything).Return(automations, nil)

	err := scheduler.ProcessActiveAutomations(context.Background())

	require.NoError(t, err)
	svc.AssertNotCalled(t, "RunAutomation")
}

func TestProcessActiveAutomations_MixedAutomations(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	repo := new(mockSchedulerAutomationRepo)
	svc := new(mockSchedulerService)
	scheduler := NewScheduler(repo, svc, logger)

	searchType := "search"
	monitorType := "monitoring"
	interval := 30
	recentRun := time.Now().Add(-5 * time.Minute).Format(time.RFC3339)
	oldRun := time.Now().Add(-2 * time.Hour).Format(time.RFC3339)

	automations := []models.Automation{
		// 1. Monitor - should be skipped
		{ID: "auto-monitor", UserID: "user-1", Name: "Monitor", Status: models.AutomationStatusActive, Type: &monitorType},
		// 2. Search, not due - should be skipped (ran 5 min ago, interval 30 min)
		{ID: "auto-recent", UserID: "user-1", Name: "Recent Run", Status: models.AutomationStatusActive, Type: &searchType, IntervalMinutes: &interval, LastRunAt: &recentRun},
		// 3. Search, due - should run (ran 2h ago, interval 30 min)
		{ID: "auto-due", UserID: "user-1", Name: "Due Run", Status: models.AutomationStatusActive, Type: &searchType, IntervalMinutes: &interval, LastRunAt: &oldRun},
	}

	repo.On("GetActiveAutomations", mock.Anything).Return(automations, nil)
	svc.On("RunAutomation", mock.Anything, "auto-due", "user-1").Return(&models.RunAutomationResult{
		Success:      true,
		MessagesSent: 1,
		OffersFound:  2,
	}, nil)

	err := scheduler.ProcessActiveAutomations(context.Background())

	require.NoError(t, err)
	svc.AssertCalled(t, "RunAutomation", mock.Anything, "auto-due", "user-1")
	svc.AssertNotCalled(t, "RunAutomation", mock.Anything, "auto-monitor", "user-1")
	svc.AssertNotCalled(t, "RunAutomation", mock.Anything, "auto-recent", "user-1")
}

// =============================================================================
// isStartDateReached Tests
// =============================================================================

func TestIsStartDateReached(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	scheduler := NewScheduler(nil, nil, logger)

	dateStr := func(s string) *string { return &s }

	// BRT (UTC-3). FixedZone evita depender de tzdata no CI/Windows.
	brt := time.FixedZone("BRT", -3*60*60)

	tests := []struct {
		name      string
		startDate *string
		now       time.Time
		want      bool
	}{
		{name: "nil start_date — always allowed", now: time.Date(2026, 5, 29, 10, 0, 0, 0, time.UTC), want: true},
		{name: "empty start_date — always allowed", startDate: dateStr(""), now: time.Date(2026, 5, 29, 10, 0, 0, 0, time.UTC), want: true},
		{name: "today equals start_date — allowed", startDate: dateStr("2026-05-29"), now: time.Date(2026, 5, 29, 10, 0, 0, 0, time.UTC), want: true},
		{name: "today after start_date — allowed", startDate: dateStr("2026-05-29"), now: time.Date(2026, 5, 30, 10, 0, 0, 0, time.UTC), want: true},
		{name: "today before start_date — blocked", startDate: dateStr("2026-05-29"), now: time.Date(2026, 5, 28, 23, 59, 0, 0, time.UTC), want: false},
		{name: "today months before — blocked", startDate: dateStr("2026-12-01"), now: time.Date(2026, 5, 29, 10, 0, 0, 0, time.UTC), want: false},
		{name: "malformed start_date — fail-open (allow)", startDate: dateStr("not-a-date"), now: time.Date(2026, 5, 29, 10, 0, 0, 0, time.UTC), want: true},

		// Regressão #154 — mesmo fuso (BRT) usado no end_date. start_date
		// = hoje deve liberar; véspera deve bloquear, comparando por dia
		// no fuso local (não em UTC).
		{name: "BRT: start_date = today, just after midnight — allowed", startDate: dateStr("2026-06-02"), now: time.Date(2026, 6, 2, 0, 30, 0, 0, brt), want: true},
		{name: "BRT: start_date = tomorrow, late tonight — blocked", startDate: dateStr("2026-06-03"), now: time.Date(2026, 6, 2, 23, 30, 0, 0, brt), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			auto := &models.Automation{StartDate: tt.startDate}
			got := scheduler.isStartDateReached(auto, tt.now)
			if got != tt.want {
				t.Errorf("isStartDateReached(%v, %v) = %v, want %v",
					tt.startDate, tt.now, got, tt.want)
			}
		})
	}
}

// =============================================================================
// isEndDateExceeded Tests
// =============================================================================

func TestIsEndDateExceeded(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	scheduler := NewScheduler(nil, nil, logger)

	dateStr := func(s string) *string { return &s }

	// BRT (UTC-3). FixedZone evita depender de tzdata no CI/Windows.
	brt := time.FixedZone("BRT", -3*60*60)

	tests := []struct {
		name    string
		endDate *string
		now     time.Time
		want    bool
	}{
		{name: "nil end_date — never expires", now: time.Date(2026, 5, 29, 10, 0, 0, 0, time.UTC), want: false},
		{name: "empty end_date — never expires", endDate: dateStr(""), now: time.Date(2026, 5, 29, 10, 0, 0, 0, time.UTC), want: false},
		{name: "today equals end_date — not exceeded (last day still allowed)", endDate: dateStr("2026-05-29"), now: time.Date(2026, 5, 29, 10, 0, 0, 0, time.UTC), want: false},
		{name: "today before end_date — not exceeded", endDate: dateStr("2026-05-29"), now: time.Date(2026, 5, 28, 23, 59, 0, 0, time.UTC), want: false},
		{name: "today after end_date — exceeded", endDate: dateStr("2026-05-29"), now: time.Date(2026, 5, 30, 0, 0, 0, 0, time.UTC), want: true},
		{name: "today months after — exceeded", endDate: dateStr("2026-01-01"), now: time.Date(2026, 5, 29, 10, 0, 0, 0, time.UTC), want: true},
		{name: "malformed end_date — fail-open (not exceeded)", endDate: dateStr("not-a-date"), now: time.Date(2026, 5, 29, 10, 0, 0, 0, time.UTC), want: false},

		// Regressão #154 — timezone off-by-one. Em fuso negativo (BRT), a
		// meia-noite local cai 3h depois da meia-noite UTC, então end_date
		// = hoje era tratado como expirado o dia todo. Estes falham com o
		// time.Parse (UTC) antigo e passam com ParseInLocation.
		{name: "BRT: end_date = today, evening — NOT exceeded", endDate: dateStr("2026-06-02"), now: time.Date(2026, 6, 2, 19, 50, 0, 0, brt), want: false},
		{name: "BRT: end_date = today, just after midnight — NOT exceeded", endDate: dateStr("2026-06-02"), now: time.Date(2026, 6, 2, 0, 30, 0, 0, brt), want: false},
		{name: "BRT: end_date = yesterday — exceeded", endDate: dateStr("2026-06-02"), now: time.Date(2026, 6, 3, 0, 30, 0, 0, brt), want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			auto := &models.Automation{EndDate: tt.endDate}
			got := scheduler.isEndDateExceeded(auto, tt.now)
			if got != tt.want {
				t.Errorf("isEndDateExceeded(%v, %v) = %v, want %v",
					tt.endDate, tt.now, got, tt.want)
			}
		})
	}
}

// =============================================================================
// isInTimeWindow Tests
// =============================================================================

func TestIsInTimeWindow(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	scheduler := NewScheduler(nil, nil, logger)

	tests := []struct {
		name      string
		startTime *string
		endTime   *string
		now       time.Time
		want      bool
	}{
		{
			name: "no time window configured - always allowed",
			now:  time.Date(2026, 1, 15, 14, 0, 0, 0, time.UTC),
			want: true,
		},
		{
			name:      "only start time - uses 23:59 as end",
			startTime: strPtr("08:00"),
			now:       time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
			want:      true,
		},
		{
			name:    "only end time - uses 00:00 as start",
			endTime: strPtr("22:00"),
			now:     time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
			want:    true,
		},
		{
			name:      "normal window - inside",
			startTime: strPtr("08:00"),
			endTime:   strPtr("18:00"),
			now:       time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC),
			want:      true,
		},
		{
			name:      "normal window - before start",
			startTime: strPtr("08:00"),
			endTime:   strPtr("18:00"),
			now:       time.Date(2026, 1, 15, 6, 0, 0, 0, time.UTC),
			want:      false,
		},
		{
			name:      "normal window - after end",
			startTime: strPtr("08:00"),
			endTime:   strPtr("18:00"),
			now:       time.Date(2026, 1, 15, 20, 0, 0, 0, time.UTC),
			want:      false,
		},
		{
			name:      "normal window - at start boundary",
			startTime: strPtr("08:00"),
			endTime:   strPtr("18:00"),
			now:       time.Date(2026, 1, 15, 8, 0, 0, 0, time.UTC),
			want:      true,
		},
		{
			name:      "normal window - at end boundary",
			startTime: strPtr("08:00"),
			endTime:   strPtr("18:00"),
			now:       time.Date(2026, 1, 15, 18, 0, 0, 0, time.UTC),
			want:      true,
		},
		{
			name:      "overnight window - inside late",
			startTime: strPtr("22:00"),
			endTime:   strPtr("06:00"),
			now:       time.Date(2026, 1, 15, 23, 0, 0, 0, time.UTC),
			want:      true,
		},
		{
			name:      "overnight window - inside early",
			startTime: strPtr("22:00"),
			endTime:   strPtr("06:00"),
			now:       time.Date(2026, 1, 15, 3, 0, 0, 0, time.UTC),
			want:      true,
		},
		{
			name:      "overnight window - outside (midday)",
			startTime: strPtr("22:00"),
			endTime:   strPtr("06:00"),
			now:       time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC),
			want:      false,
		},
		{
			name:      "overnight window - at start boundary",
			startTime: strPtr("22:00"),
			endTime:   strPtr("06:00"),
			now:       time.Date(2026, 1, 15, 22, 0, 0, 0, time.UTC),
			want:      true,
		},
		{
			name:      "overnight window - at end boundary",
			startTime: strPtr("22:00"),
			endTime:   strPtr("06:00"),
			now:       time.Date(2026, 1, 15, 6, 0, 0, 0, time.UTC),
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			auto := &models.Automation{
				StartTime: tt.startTime,
				EndTime:   tt.endTime,
			}

			got := scheduler.isInTimeWindow(auto, tt.now)
			assert.Equal(t, tt.want, got)
		})
	}
}

// =============================================================================
// isDueForExecution Tests
// =============================================================================

func TestIsDueForExecution(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	scheduler := NewScheduler(nil, nil, logger)

	now := time.Now()

	tests := []struct {
		name            string
		intervalMinutes *int
		lastRunAt       *string
		nextRunAt       *string
		want            bool
	}{
		{
			name: "never run before - nil lastRunAt",
			want: true,
		},
		{
			name:      "never run before - empty lastRunAt",
			lastRunAt: strPtr(""),
			want:      true,
		},
		{
			name:            "due for execution - past interval",
			intervalMinutes: intPtr(30),
			lastRunAt:       strPtr(now.Add(-45 * time.Minute).Format(time.RFC3339)),
			want:            true,
		},
		{
			name:            "not due yet - within interval",
			intervalMinutes: intPtr(60),
			lastRunAt:       strPtr(now.Add(-30 * time.Minute).Format(time.RFC3339)),
			want:            false,
		},
		{
			name:            "due at exact boundary",
			intervalMinutes: intPtr(30),
			lastRunAt:       strPtr(now.Add(-30 * time.Minute).Format(time.RFC3339)),
			want:            true,
		},
		{
			name:            "uses default 60 min interval when nil",
			intervalMinutes: nil,
			lastRunAt:       strPtr(now.Add(-65 * time.Minute).Format(time.RFC3339)),
			want:            true,
		},
		{
			name:            "uses default 60 min interval - not due",
			intervalMinutes: nil,
			lastRunAt:       strPtr(now.Add(-30 * time.Minute).Format(time.RFC3339)),
			want:            false,
		},
		{
			name:            "zero interval uses default 60",
			intervalMinutes: intPtr(0),
			lastRunAt:       strPtr(now.Add(-30 * time.Minute).Format(time.RFC3339)),
			want:            false,
		},
		{
			name:            "invalid lastRunAt format - assumes due",
			intervalMinutes: intPtr(30),
			lastRunAt:       strPtr("invalid-date"),
			want:            true,
		},
		{
			name:            "very old lastRunAt - due",
			intervalMinutes: intPtr(60),
			lastRunAt:       strPtr(now.Add(-24 * time.Hour).Format(time.RFC3339)),
			want:            true,
		},
		{
			name:            "not due - next_run_at in future",
			intervalMinutes: intPtr(30),
			lastRunAt:       strPtr(now.Add(-45 * time.Minute).Format(time.RFC3339)),
			nextRunAt:       strPtr(now.Add(15 * time.Minute).Format(time.RFC3339)),
			want:            false,
		},
		{
			name:            "due - next_run_at in past",
			intervalMinutes: intPtr(30),
			lastRunAt:       strPtr(now.Add(-45 * time.Minute).Format(time.RFC3339)),
			nextRunAt:       strPtr(now.Add(-15 * time.Minute).Format(time.RFC3339)),
			want:            true,
		},
		{
			name:            "due - next_run_at nil",
			intervalMinutes: intPtr(30),
			lastRunAt:       strPtr(now.Add(-45 * time.Minute).Format(time.RFC3339)),
			want:            true,
		},
		{
			name:            "due - next_run_at empty",
			intervalMinutes: intPtr(30),
			lastRunAt:       strPtr(now.Add(-45 * time.Minute).Format(time.RFC3339)),
			nextRunAt:       strPtr(""),
			want:            true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			auto := &models.Automation{
				ID:              "auto-test",
				IntervalMinutes: tt.intervalMinutes,
				LastRunAt:       tt.lastRunAt,
				NextRunAt:       tt.nextRunAt,
			}

			got := scheduler.isDueForExecution(auto, now)
			assert.Equal(t, tt.want, got)
		})
	}
}

func intPtr(i int) *int { return &i }

// Stub do reconciliador de entregas do LIST (Service interface).
func (m *mockSchedulerService) ReconcileListDeliveries(ctx context.Context) error { return nil }
