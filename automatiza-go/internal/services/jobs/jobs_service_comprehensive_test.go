package jobs_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"
	"automatiza-go/internal/services/jobs"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// =============================================================================
// Mock Notifiers
// =============================================================================

type MockJobStatusNotifier struct {
	mock.Mock
}

func (m *MockJobStatusNotifier) PublishJobStatus(userID, jobID, status string, errorMessage *string) {
	m.Called(userID, jobID, status, errorMessage)
}

type MockHistoryNotifier struct {
	mock.Mock
}

func (m *MockHistoryNotifier) PublishHistoryChange(userID, historyID, status string) {
	m.Called(userID, historyID, status)
}

// =============================================================================
// Mock ActionHistoryRepository
// =============================================================================

type MockActionHistoryRepository struct {
	mock.Mock
}

func (m *MockActionHistoryRepository) Create(ctx context.Context, userID, actionType, description string, details map[string]interface{}) (*models.ActionHistory, error) {
	args := m.Called(ctx, userID, actionType, description, details)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ActionHistory), args.Error(1)
}

func (m *MockActionHistoryRepository) Update(ctx context.Context, historyID string, updates map[string]interface{}) error {
	args := m.Called(ctx, historyID, updates)
	return args.Error(0)
}

func (m *MockActionHistoryRepository) UpdateStatus(ctx context.Context, id, status, errorMessage string) error {
	args := m.Called(ctx, id, status, errorMessage)
	return args.Error(0)
}

func (m *MockActionHistoryRepository) GetByID(ctx context.Context, historyID string) (*models.ActionHistory, error) {
	args := m.Called(ctx, historyID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ActionHistory), args.Error(1)
}

func (m *MockActionHistoryRepository) GetUserHistory(ctx context.Context, userID string, limit, offset int) ([]models.ActionHistory, error) {
	args := m.Called(ctx, userID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.ActionHistory), args.Error(1)
}

func (m *MockActionHistoryRepository) GetByUserID(ctx context.Context, userID string, limit, offset int) ([]models.ActionHistory, error) {
	args := m.Called(ctx, userID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.ActionHistory), args.Error(1)
}

func (m *MockActionHistoryRepository) GetByActionType(ctx context.Context, userID, actionType string, limit int) ([]models.ActionHistory, error) {
	args := m.Called(ctx, userID, actionType, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.ActionHistory), args.Error(1)
}

func (m *MockActionHistoryRepository) GetUserHistoryFiltered(ctx context.Context, req models.HistoryListRequest, userID string) ([]models.ActionHistory, error) {
	args := m.Called(ctx, req, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.ActionHistory), args.Error(1)
}

func (m *MockActionHistoryRepository) CountUserHistoryFiltered(ctx context.Context, req models.HistoryListRequest, userID string) (models.HistoryStatsResult, error) {
	args := m.Called(ctx, req, userID)
	return args.Get(0).(models.HistoryStatsResult), args.Error(1)
}

func (m *MockActionHistoryRepository) MarkCompleted(ctx context.Context, historyID string) error {
	args := m.Called(ctx, historyID)
	return args.Error(0)
}

func (m *MockActionHistoryRepository) MarkFailed(ctx context.Context, historyID, errorMessage string) error {
	args := m.Called(ctx, historyID, errorMessage)
	return args.Error(0)
}

func (m *MockActionHistoryRepository) MarkRunning(ctx context.Context, historyID string) error {
	args := m.Called(ctx, historyID)
	return args.Error(0)
}

func (m *MockActionHistoryRepository) IncrementSuccessCount(ctx context.Context, historyID string) error {
	args := m.Called(ctx, historyID)
	return args.Error(0)
}

func (m *MockActionHistoryRepository) IncrementErrorCount(ctx context.Context, historyID string, errorMsg string) error {
	args := m.Called(ctx, historyID, errorMsg)
	return args.Error(0)
}

func (m *MockActionHistoryRepository) UpdateCounts(ctx context.Context, historyID string, successCount, errorCount int32) error {
	args := m.Called(ctx, historyID, successCount, errorCount)
	return args.Error(0)
}

func (m *MockActionHistoryRepository) CleanupOldByActionType(_ context.Context, _ string, _ time.Duration) (int64, error) {
	return 0, nil
}

func (m *MockActionHistoryRepository) DeleteUserHistory(ctx context.Context, userID string, beforeDate *time.Time, actionTypes []string) (int64, error) {
	args := m.Called(ctx, userID, beforeDate, actionTypes)
	return args.Get(0).(int64), args.Error(1)
}

// =============================================================================
// Full Service Constructor with All Dependencies
// =============================================================================

func newFullTestService(t *testing.T) (
	jobs.Service,
	*MockJobRepository,
	*MockInstanceRepository,
	*MockGroupRepository,
	*whatsapp.FakeAdapter,
	*MockMLService,
	*MockJobStatusNotifier,
	*MockActionHistoryRepository,
	*MockHistoryNotifier,
) {
	t.Helper()
	logger := zaptest.NewLogger(t)

	jobRepo := new(MockJobRepository)
	instanceRepo := new(MockInstanceRepository)
	groupRepo := new(MockGroupRepository)
	waProvider := whatsapp.NewFakeAdapter()
	mlService := new(MockMLService)
	notifier := new(MockJobStatusNotifier)
	historyRepo := new(MockActionHistoryRepository)
	historyNotifier := new(MockHistoryNotifier)

	_ = mlService // kept for return signature compatibility with older tests
	svc := jobs.NewService(jobRepo, instanceRepo, groupRepo, waProvider, notifier, historyRepo, historyNotifier, logger)

	return svc, jobRepo, instanceRepo, groupRepo, waProvider, mlService, notifier, historyRepo, historyNotifier
}

// =============================================================================
// ListJobsPaginated Tests
// =============================================================================

func TestListJobsPaginated_Success(t *testing.T) {
	svc, jobRepo, _, _, _, _ := newTestService(t)
	ctx := context.Background()

	params := repository.JobListParams{
		UserID:   "user-1",
		PageSize: 10,
	}
	expected := &repository.JobListResult{
		Jobs: []models.StandaloneJob{
			{ID: "job-1", UserID: "user-1"},
			{ID: "job-2", UserID: "user-1"},
		},
		TotalCount: 2,
	}

	jobRepo.On("ListByUserID", ctx, params).Return(expected, nil)

	result, err := svc.ListJobsPaginated(ctx, params)

	require.NoError(t, err)
	assert.Len(t, result.Jobs, 2)
	assert.Equal(t, 2, result.TotalCount)
	jobRepo.AssertExpectations(t)
}

func TestListJobsPaginated_Error(t *testing.T) {
	svc, jobRepo, _, _, _, _ := newTestService(t)
	ctx := context.Background()

	params := repository.JobListParams{UserID: "user-1", PageSize: 10}
	jobRepo.On("ListByUserID", ctx, params).Return(nil, errors.New("database error"))

	result, err := svc.ListJobsPaginated(ctx, params)

	assert.Error(t, err)
	assert.Nil(t, result)
	jobRepo.AssertExpectations(t)
}

// =============================================================================
// ListJobs Error Tests
// =============================================================================

func TestListJobs_Error(t *testing.T) {
	svc, jobRepo, _, _, _, _ := newTestService(t)
	ctx := context.Background()

	jobRepo.On("GetByUserID", ctx, "user-1", (*models.StandaloneJobStatus)(nil), 100).
		Return(nil, errors.New("repo error"))

	result, err := svc.ListJobs(ctx, "user-1")

	assert.Error(t, err)
	assert.Nil(t, result)
	jobRepo.AssertExpectations(t)
}

// =============================================================================
// GetJob Error from Repository
// =============================================================================

func TestGetJob_RepositoryError(t *testing.T) {
	svc, jobRepo, _, _, _, _ := newTestService(t)
	ctx := context.Background()

	jobRepo.On("GetByID", ctx, "job-123").Return(nil, errors.New("db connection failed"))

	result, err := svc.GetJob(ctx, "job-123", "user-1")

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "db connection failed")
	jobRepo.AssertExpectations(t)
}

// =============================================================================
// UpdateJob Edge Cases
// =============================================================================

func TestUpdateJob_WrongOwner_Error(t *testing.T) {
	svc, jobRepo, _, _, _, _ := newTestService(t)
	ctx := context.Background()

	existingJob := &models.StandaloneJob{
		ID:     "job-123",
		UserID: "user-1",
		Status: models.StandaloneJobStatusPending,
	}

	jobRepo.On("GetByID", ctx, "job-123").Return(existingJob, nil)

	updates := map[string]interface{}{"message": "Updated"}
	result, err := svc.UpdateJob(ctx, "job-123", "user-2", updates)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not found")
}

func TestUpdateJob_RepoUpdateError(t *testing.T) {
	svc, jobRepo, _, _, _, _ := newTestService(t)
	ctx := context.Background()

	existingJob := &models.StandaloneJob{
		ID:     "job-123",
		UserID: "user-1",
		Status: models.StandaloneJobStatusPending,
	}

	jobRepo.On("GetByID", ctx, "job-123").Return(existingJob, nil)
	jobRepo.On("Update", ctx, "job-123", mock.Anything).Return(errors.New("update failed"))

	updates := map[string]interface{}{"message": "Updated"}
	result, err := svc.UpdateJob(ctx, "job-123", "user-1", updates)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "update failed")
}

func TestUpdateJob_DoneStatus_Error(t *testing.T) {
	svc, jobRepo, _, _, _, _ := newTestService(t)
	ctx := context.Background()

	existingJob := &models.StandaloneJob{
		ID:     "job-123",
		UserID: "user-1",
		Status: models.StandaloneJobStatusDone,
	}

	jobRepo.On("GetByID", ctx, "job-123").Return(existingJob, nil)

	updates := map[string]interface{}{"message": "Updated"}
	result, err := svc.UpdateJob(ctx, "job-123", "user-1", updates)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "pendentes")
}

func TestUpdateJob_FailedStatus_Error(t *testing.T) {
	svc, jobRepo, _, _, _, _ := newTestService(t)
	ctx := context.Background()

	existingJob := &models.StandaloneJob{
		ID:     "job-123",
		UserID: "user-1",
		Status: models.StandaloneJobStatusFailed,
	}

	jobRepo.On("GetByID", ctx, "job-123").Return(existingJob, nil)

	result, err := svc.UpdateJob(ctx, "job-123", "user-1", map[string]interface{}{"message": "X"})

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestUpdateJob_CancelledStatus_Error(t *testing.T) {
	svc, jobRepo, _, _, _, _ := newTestService(t)
	ctx := context.Background()

	existingJob := &models.StandaloneJob{
		ID:     "job-123",
		UserID: "user-1",
		Status: models.StandaloneJobStatusCancelled,
	}

	jobRepo.On("GetByID", ctx, "job-123").Return(existingJob, nil)

	result, err := svc.UpdateJob(ctx, "job-123", "user-1", map[string]interface{}{"message": "X"})

	assert.Error(t, err)
	assert.Nil(t, result)
}

// =============================================================================
// DeleteJob Edge Cases
// =============================================================================

func TestDeleteJob_NotFound_Error(t *testing.T) {
	svc, jobRepo, _, _, _, _ := newTestService(t)
	ctx := context.Background()

	jobRepo.On("GetByID", ctx, "job-not-exists").Return(nil, nil)

	err := svc.DeleteJob(ctx, "job-not-exists", "user-1")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestDeleteJob_WrongOwner_Error(t *testing.T) {
	svc, jobRepo, _, _, _, _ := newTestService(t)
	ctx := context.Background()

	existingJob := &models.StandaloneJob{
		ID:     "job-123",
		UserID: "user-1",
		Status: models.StandaloneJobStatusPending,
	}

	jobRepo.On("GetByID", ctx, "job-123").Return(existingJob, nil)

	err := svc.DeleteJob(ctx, "job-123", "user-other")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestDeleteJob_Failed_Error(t *testing.T) {
	svc, jobRepo, _, _, _, _ := newTestService(t)
	ctx := context.Background()

	existingJob := &models.StandaloneJob{
		ID:     "job-123",
		UserID: "user-1",
		Status: models.StandaloneJobStatusFailed,
	}

	jobRepo.On("GetByID", ctx, "job-123").Return(existingJob, nil)

	err := svc.DeleteJob(ctx, "job-123", "user-1")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pendentes ou cancelados")
}

func TestDeleteJob_RepoDeleteError(t *testing.T) {
	svc, jobRepo, _, _, _, _ := newTestService(t)
	ctx := context.Background()

	existingJob := &models.StandaloneJob{
		ID:     "job-123",
		UserID: "user-1",
		Status: models.StandaloneJobStatusPending,
	}

	jobRepo.On("GetByID", ctx, "job-123").Return(existingJob, nil)
	jobRepo.On("Delete", ctx, "job-123", "user-1").Return(errors.New("delete failed"))

	err := svc.DeleteJob(ctx, "job-123", "user-1")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "delete failed")
}

// =============================================================================
// CreateJob — Default MaxRetries
// =============================================================================

func TestCreateJob_SetsDefaultMaxRetries(t *testing.T) {
	svc, jobRepo, _, _, _, _ := newTestService(t)
	ctx := context.Background()

	job := &models.StandaloneJob{
		ActionType:   models.JobActionSendMessage,
		ScheduledFor: time.Now().Add(time.Hour),
		MaxRetries:   0, // Should be set to 3
		Payload:      validJobPayload([]string{"group1@g.us"}),
	}

	jobRepo.On("Create", ctx, mock.AnythingOfType("*models.StandaloneJob")).
		Run(func(args mock.Arguments) {
			j := args.Get(1).(*models.StandaloneJob)
			assert.Equal(t, 3, j.MaxRetries)
		}).
		Return(&models.StandaloneJob{ID: "job-created", MaxRetries: 3}, nil)

	result, err := svc.CreateJob(ctx, "user-1", job)

	require.NoError(t, err)
	assert.NotNil(t, result)
	jobRepo.AssertExpectations(t)
}

func TestCreateJob_KeepsCustomMaxRetries(t *testing.T) {
	svc, jobRepo, _, _, _, _ := newTestService(t)
	ctx := context.Background()

	job := &models.StandaloneJob{
		ActionType:   models.JobActionSendMessage,
		ScheduledFor: time.Now().Add(time.Hour),
		MaxRetries:   5,
		Payload:      validJobPayload([]string{"group1@g.us"}),
	}

	jobRepo.On("Create", ctx, mock.AnythingOfType("*models.StandaloneJob")).
		Run(func(args mock.Arguments) {
			j := args.Get(1).(*models.StandaloneJob)
			assert.Equal(t, 5, j.MaxRetries)
		}).
		Return(&models.StandaloneJob{ID: "job-created", MaxRetries: 5}, nil)

	result, err := svc.CreateJob(ctx, "user-1", job)

	require.NoError(t, err)
	assert.NotNil(t, result)
}

// =============================================================================
// CreateJob — UpdateDescription and CloseGroups/OpenGroups validation
// =============================================================================

func TestCreateJob_UpdateDescription_NoDescription_Success(t *testing.T) {
	svc, jobRepo, _, _, _, _ := newTestService(t)
	ctx := context.Background()

	// UpdateDescription with empty description should still work (no validation for it in CreateJob)
	job := &models.StandaloneJob{
		ActionType:   models.JobActionUpdateDescription,
		ScheduledFor: time.Now().Add(time.Hour),
		Payload: models.JobPayload{
			Groups:      []string{"group1@g.us"},
			Description: "",
		},
	}

	createdJob := *job
	createdJob.ID = "job-desc"
	jobRepo.On("Create", ctx, mock.AnythingOfType("*models.StandaloneJob")).Return(&createdJob, nil)

	result, err := svc.CreateJob(ctx, "user-1", job)

	require.NoError(t, err)
	assert.NotNil(t, result)
}

// =============================================================================
// CreateJob — SelectableCount Validation on SendPoll
// =============================================================================

func TestCreateJob_SendPoll_SelectableCountValid(t *testing.T) {
	svc, jobRepo, _, _, _, _ := newTestService(t)
	ctx := context.Background()

	job := &models.StandaloneJob{
		ActionType:   models.JobActionSendPoll,
		ScheduledFor: time.Now().Add(time.Hour),
		Payload: models.JobPayload{
			Groups:          []string{"group1@g.us"},
			Question:        "Qual?",
			Options:         []string{"A", "B", "C"},
			SelectableCount: 2,
		},
	}

	createdJob := *job
	createdJob.ID = "job-poll-sc"
	jobRepo.On("Create", ctx, mock.AnythingOfType("*models.StandaloneJob")).Return(&createdJob, nil)

	result, err := svc.CreateJob(ctx, "user-1", job)

	require.NoError(t, err)
	assert.NotNil(t, result)
}

// =============================================================================
// ProcessScheduledJobs — With Notifiers
// =============================================================================

func TestProcessScheduledJobs_WithNotifiers(t *testing.T) {
	svc, jobRepo, instanceRepo, groupRepo, waProvider, _, notifier, historyRepo, historyNotifier := newFullTestService(t)
	ctx := context.Background()

	pendingJobs := []models.StandaloneJob{
		{
			ID:         "job-1",
			UserID:     "user-1",
			ActionType: models.JobActionSendMessage,
			Status:     models.StandaloneJobStatusPending,
			MaxRetries: 3,
			Payload: models.JobPayload{
				Groups:  []string{"db-group-1"},
				Message: "Hello",
			},
		},
	}

	resolvedGroups := []models.Group{
		{ID: "db-group-1", WAGroupID: "group1@g.us", IsAdmin: true},
	}

	jobRepo.On("ClaimPendingDueJobs", ctx, 50).Return(pendingJobs, nil)
	instanceRepo.On("GetUserInstances", ctx, "user-1").Return([]models.Instance{{InstanceID: "inst-1", Status: "connected"}}, nil)
	groupRepo.On("GetGroupsByIDs", ctx, "user-1", []string{"db-group-1"}).Return(resolvedGroups, nil)
	jobRepo.On("UpdateStatus", mock.Anything, "job-1", models.StandaloneJobStatusDone, (*string)(nil)).Return(nil)

	// Notifier expectations
	notifier.On("PublishJobStatus", "user-1", "job-1", "running", (*string)(nil)).Return()
	notifier.On("PublishJobStatus", "user-1", "job-1", "done", (*string)(nil)).Return()

	// History expectations
	historyRepo.On("Create", ctx, "user-1", "send_message", mock.AnythingOfType("string"), mock.Anything).
		Return(&models.ActionHistory{ID: "hist-1"}, nil)
	historyRepo.On("Update", mock.Anything, "hist-1", mock.Anything).Return(nil)
	historyRepo.On("UpdateCounts", mock.Anything, "hist-1", int32(1), int32(0)).Return(nil)
	historyRepo.On("UpdateStatus", mock.Anything, "hist-1", "completed", "").Return(nil)

	historyNotifier.On("PublishHistoryChange", "user-1", "hist-1", "running").Return()
	historyNotifier.On("PublishHistoryChange", "user-1", "hist-1", "completed").Return()

	result, err := svc.ProcessScheduledJobs(ctx)

	require.NoError(t, err)
	assert.Equal(t, 1, result.Processed)
	assert.Equal(t, 0, result.Failed)
	assert.Equal(t, 1, waProvider.CallCount("SendText"))
	notifier.AssertExpectations(t)
	historyNotifier.AssertExpectations(t)
	historyRepo.AssertExpectations(t)
}

// =============================================================================
// ProcessScheduledJobs — All Failed With Retry
// =============================================================================

func TestProcessScheduledJobs_AllFailed_WithRetry(t *testing.T) {
	svc, jobRepo, instanceRepo, groupRepo, waProvider, _, notifier, historyRepo, historyNotifier := newFullTestService(t)
	ctx := context.Background()

	waProvider.SimulateError = errors.New("WAHA down")

	pendingJobs := []models.StandaloneJob{
		{
			ID:         "job-1",
			UserID:     "user-1",
			ActionType: models.JobActionSendMessage,
			Status:     models.StandaloneJobStatusPending,
			RetryCount: 0,
			MaxRetries: 3,
			Payload: models.JobPayload{
				Groups:  []string{"db-group-1"},
				Message: "Test",
			},
		},
	}

	resolvedGroups := []models.Group{
		{ID: "db-group-1", WAGroupID: "group1@g.us", IsAdmin: true},
	}

	jobRepo.On("ClaimPendingDueJobs", ctx, 50).Return(pendingJobs, nil)
	instanceRepo.On("GetUserInstances", ctx, "user-1").Return([]models.Instance{{InstanceID: "inst-1", Status: "connected"}}, nil)
	groupRepo.On("GetGroupsByIDs", ctx, "user-1", []string{"db-group-1"}).Return(resolvedGroups, nil)
	jobRepo.On("ScheduleRetry", mock.Anything, "job-1", mock.AnythingOfType("time.Time")).Return(nil)

	notifier.On("PublishJobStatus", "user-1", "job-1", "running", (*string)(nil)).Return()
	notifier.On("PublishJobStatus", "user-1", "job-1", "pending", (*string)(nil)).Return()

	historyRepo.On("Create", ctx, "user-1", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.Anything).
		Return(&models.ActionHistory{ID: "hist-1"}, nil)
	historyRepo.On("Update", mock.Anything, "hist-1", mock.Anything).Return(nil)
	historyRepo.On("UpdateCounts", mock.Anything, "hist-1", mock.AnythingOfType("int32"), mock.AnythingOfType("int32")).Return(nil)
	historyRepo.On("UpdateStatus", mock.Anything, "hist-1", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(nil)
	historyNotifier.On("PublishHistoryChange", "user-1", "hist-1", mock.AnythingOfType("string")).Return()

	result, err := svc.ProcessScheduledJobs(ctx)

	require.NoError(t, err)
	assert.Equal(t, 0, result.Processed)
	assert.Equal(t, 1, result.Skipped)
	assert.Equal(t, 0, result.Failed)
	jobRepo.AssertExpectations(t)
}

// =============================================================================
// ProcessScheduledJobs — All Failed, Retries Exhausted
// =============================================================================

func TestProcessScheduledJobs_AllFailed_RetriesExhausted(t *testing.T) {
	svc, jobRepo, instanceRepo, groupRepo, waProvider, _, notifier, historyRepo, historyNotifier := newFullTestService(t)
	ctx := context.Background()

	waProvider.SimulateError = errors.New("WAHA down")

	pendingJobs := []models.StandaloneJob{
		{
			ID:         "job-1",
			UserID:     "user-1",
			ActionType: models.JobActionSendMessage,
			Status:     models.StandaloneJobStatusPending,
			RetryCount: 3, // At max
			MaxRetries: 3,
			Payload: models.JobPayload{
				Groups:  []string{"db-group-1"},
				Message: "Test",
			},
		},
	}

	resolvedGroups := []models.Group{
		{ID: "db-group-1", WAGroupID: "group1@g.us", IsAdmin: true},
	}

	jobRepo.On("ClaimPendingDueJobs", ctx, 50).Return(pendingJobs, nil)
	instanceRepo.On("GetUserInstances", ctx, "user-1").Return([]models.Instance{{InstanceID: "inst-1", Status: "connected"}}, nil)
	groupRepo.On("GetGroupsByIDs", ctx, "user-1", []string{"db-group-1"}).Return(resolvedGroups, nil)
	jobRepo.On("UpdateStatus", mock.Anything, "job-1", models.StandaloneJobStatusFailed, mock.AnythingOfType("*string")).Return(nil)

	notifier.On("PublishJobStatus", "user-1", "job-1", "running", (*string)(nil)).Return()
	notifier.On("PublishJobStatus", "user-1", "job-1", "failed", mock.AnythingOfType("*string")).Return()

	historyRepo.On("Create", ctx, "user-1", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.Anything).
		Return(&models.ActionHistory{ID: "hist-1"}, nil)
	historyRepo.On("Update", mock.Anything, "hist-1", mock.Anything).Return(nil)
	historyRepo.On("UpdateCounts", mock.Anything, "hist-1", mock.AnythingOfType("int32"), mock.AnythingOfType("int32")).Return(nil)
	historyRepo.On("UpdateStatus", mock.Anything, "hist-1", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(nil)
	historyNotifier.On("PublishHistoryChange", "user-1", "hist-1", mock.AnythingOfType("string")).Return()

	result, err := svc.ProcessScheduledJobs(ctx)

	require.NoError(t, err)
	assert.Equal(t, 0, result.Processed)
	assert.Equal(t, 1, result.Failed)
	assert.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0], "retries exhausted")
	notifier.AssertExpectations(t)
}

// =============================================================================
// ProcessScheduledJobs — ScheduleRetry Error
// =============================================================================

func TestProcessScheduledJobs_ScheduleRetryError(t *testing.T) {
	svc, jobRepo, instanceRepo, groupRepo, waProvider, _, notifier, historyRepo, historyNotifier := newFullTestService(t)
	ctx := context.Background()

	waProvider.SimulateError = errors.New("error")

	pendingJobs := []models.StandaloneJob{
		{
			ID:         "job-1",
			UserID:     "user-1",
			ActionType: models.JobActionSendMessage,
			Status:     models.StandaloneJobStatusPending,
			RetryCount: 0,
			MaxRetries: 3,
			Payload: models.JobPayload{
				Groups:  []string{"db-group-1"},
				Message: "Test",
			},
		},
	}

	resolvedGroups := []models.Group{
		{ID: "db-group-1", WAGroupID: "group1@g.us", IsAdmin: true},
	}

	jobRepo.On("ClaimPendingDueJobs", ctx, 50).Return(pendingJobs, nil)
	instanceRepo.On("GetUserInstances", ctx, "user-1").Return([]models.Instance{{InstanceID: "inst-1", Status: "connected"}}, nil)
	groupRepo.On("GetGroupsByIDs", ctx, "user-1", []string{"db-group-1"}).Return(resolvedGroups, nil)
	jobRepo.On("ScheduleRetry", mock.Anything, "job-1", mock.AnythingOfType("time.Time")).Return(errors.New("redis down"))
	jobRepo.On("UpdateStatus", mock.Anything, "job-1", models.StandaloneJobStatusFailed, mock.AnythingOfType("*string")).Return(nil)

	notifier.On("PublishJobStatus", "user-1", "job-1", "running", (*string)(nil)).Return()
	notifier.On("PublishJobStatus", "user-1", "job-1", "failed", mock.AnythingOfType("*string")).Return()

	historyRepo.On("Create", ctx, "user-1", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.Anything).
		Return(&models.ActionHistory{ID: "hist-1"}, nil)
	historyRepo.On("Update", mock.Anything, "hist-1", mock.Anything).Return(nil)
	historyRepo.On("UpdateCounts", mock.Anything, "hist-1", mock.AnythingOfType("int32"), mock.AnythingOfType("int32")).Return(nil)
	historyRepo.On("UpdateStatus", mock.Anything, "hist-1", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(nil)
	historyNotifier.On("PublishHistoryChange", "user-1", "hist-1", mock.AnythingOfType("string")).Return()

	result, err := svc.ProcessScheduledJobs(ctx)

	require.NoError(t, err)
	assert.Equal(t, 1, result.Failed)
}

// =============================================================================
// ProcessScheduledJobs — Error Fetching Pending Jobs
// =============================================================================

func TestProcessScheduledJobs_FetchError(t *testing.T) {
	svc, jobRepo, _, _, _, _ := newTestService(t)
	ctx := context.Background()

	jobRepo.On("ClaimPendingDueJobs", ctx, 50).Return(nil, errors.New("database unreachable"))

	result, err := svc.ProcessScheduledJobs(ctx)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to claim pending jobs")
}

// =============================================================================
// ProcessScheduledJobs — Contexto expirado devolve o lote reivindicado
// =============================================================================

// O claim atômico marca TODOS os jobs do lote como 'running' de uma vez. Como o
// ctx do cron tem cap de 5min, um lote grande pode ser cortado no meio — e o
// que não rodou precisa voltar para 'pending', senão só o reaper o resolveria
// (30min depois, e como 'failed'). Este teste é o que impede essa regressão.
//
// Substitui o antigo TestProcessScheduledJobs_MarkRunningError_SkipsJob: o
// UpdateStatus('running') separado deixou de existir, então a falha que aquele
// teste cobria é agora estruturalmente impossível.
func TestProcessScheduledJobs_ContextExpired_ReleasesClaimedJobs(t *testing.T) {
	svc, jobRepo, _, _, _, _ := newTestService(t)

	ctx, cancel := context.WithCancel(context.Background())

	claimed := []models.StandaloneJob{
		{
			ID:         "job-1",
			UserID:     "user-1",
			ActionType: models.JobActionSendMessage,
			Status:     models.StandaloneJobStatusRunning,
			Payload:    validJobPayload([]string{"group1@g.us"}),
		},
		{
			ID:         "job-2",
			UserID:     "user-1",
			ActionType: models.JobActionSendMessage,
			Status:     models.StandaloneJobStatusRunning,
			Payload:    validJobPayload([]string{"group2@g.us"}),
		},
	}

	jobRepo.On("ClaimPendingDueJobs", ctx, 50).Return(claimed, nil).Run(func(mock.Arguments) {
		// O ctx morre logo após o claim: nenhum job chega a ser executado.
		cancel()
	})
	jobRepo.On("ReleaseClaimedJobs", mock.Anything, []string{"job-1", "job-2"}).Return(int64(2), nil)

	result, err := svc.ProcessScheduledJobs(ctx)

	require.NoError(t, err)
	assert.Equal(t, 0, result.Processed)
	assert.Equal(t, 0, result.Failed)
	// A prova: os dois jobs voltaram para a fila em vez de ficarem presos.
	jobRepo.AssertCalled(t, "ReleaseClaimedJobs", mock.Anything, []string{"job-1", "job-2"})
}

// =============================================================================
// ProcessScheduledJobs — Instance Error
// =============================================================================

func TestProcessScheduledJobs_InstanceRepoError(t *testing.T) {
	svc, jobRepo, instanceRepo, _, _, _ := newTestService(t)
	ctx := context.Background()

	pendingJobs := []models.StandaloneJob{
		{
			ID:         "job-1",
			UserID:     "user-1",
			ActionType: models.JobActionSendMessage,
			Payload:    validJobPayload([]string{"group1@g.us"}),
		},
	}

	jobRepo.On("ClaimPendingDueJobs", ctx, 50).Return(pendingJobs, nil)
	instanceRepo.On("GetUserInstances", ctx, "user-1").Return(nil, errors.New("instance service unavailable"))
	jobRepo.On("UpdateStatus", mock.Anything, "job-1", models.StandaloneJobStatusFailed, mock.AnythingOfType("*string")).Return(nil)

	result, err := svc.ProcessScheduledJobs(ctx)

	require.NoError(t, err)
	assert.Equal(t, 0, result.Processed)
	assert.Equal(t, 1, result.Failed)
	assert.Len(t, result.Errors, 1)
}

// =============================================================================
// ProcessScheduledJobs — No Instances At All
// =============================================================================

func TestProcessScheduledJobs_NoInstances(t *testing.T) {
	svc, jobRepo, instanceRepo, _, _, _ := newTestService(t)
	ctx := context.Background()

	pendingJobs := []models.StandaloneJob{
		{
			ID:         "job-1",
			UserID:     "user-1",
			ActionType: models.JobActionSendMessage,
			Payload:    validJobPayload([]string{"group1@g.us"}),
		},
	}

	jobRepo.On("ClaimPendingDueJobs", ctx, 50).Return(pendingJobs, nil)
	instanceRepo.On("GetUserInstances", ctx, "user-1").Return([]models.Instance{}, nil)
	jobRepo.On("UpdateStatus", mock.Anything, "job-1", models.StandaloneJobStatusFailed, mock.AnythingOfType("*string")).Return(nil)

	result, err := svc.ProcessScheduledJobs(ctx)

	require.NoError(t, err)
	assert.Equal(t, 1, result.Failed)
}

// =============================================================================
// ProcessScheduledJobs — Group Resolution Fails
// =============================================================================

func TestProcessScheduledJobs_GroupResolutionFails(t *testing.T) {
	svc, jobRepo, instanceRepo, groupRepo, _, _ := newTestService(t)
	ctx := context.Background()

	pendingJobs := []models.StandaloneJob{
		{
			ID:         "job-1",
			UserID:     "user-1",
			ActionType: models.JobActionSendMessage,
			RetryCount: 3, // Exhausted so it goes to failed path directly
			MaxRetries: 3,
			Payload: models.JobPayload{
				Groups:  []string{"db-group-1"},
				Message: "Test",
			},
		},
	}

	jobRepo.On("ClaimPendingDueJobs", ctx, 50).Return(pendingJobs, nil)
	instanceRepo.On("GetUserInstances", ctx, "user-1").Return([]models.Instance{{InstanceID: "inst-1", Status: "connected"}}, nil)
	groupRepo.On("GetGroupsByIDs", ctx, "user-1", []string{"db-group-1"}).Return(nil, errors.New("group repo error"))
	jobRepo.On("UpdateStatus", mock.Anything, "job-1", models.StandaloneJobStatusFailed, mock.AnythingOfType("*string")).Return(nil)

	result, err := svc.ProcessScheduledJobs(ctx)

	require.NoError(t, err)
	assert.Equal(t, 0, result.Processed)
	assert.Equal(t, 1, result.Failed)
}

// =============================================================================
// ProcessScheduledJobs — Group Not Found
// =============================================================================

func TestProcessScheduledJobs_GroupNotFound(t *testing.T) {
	svc, jobRepo, instanceRepo, groupRepo, _, _ := newTestService(t)
	ctx := context.Background()

	pendingJobs := []models.StandaloneJob{
		{
			ID:         "job-1",
			UserID:     "user-1",
			ActionType: models.JobActionSendMessage,
			RetryCount: 3, // Exhausted so it goes to failed path directly
			MaxRetries: 3,
			Payload: models.JobPayload{
				Groups:  []string{"db-group-nonexistent"},
				Message: "Test",
			},
		},
	}

	jobRepo.On("ClaimPendingDueJobs", ctx, 50).Return(pendingJobs, nil)
	instanceRepo.On("GetUserInstances", ctx, "user-1").Return([]models.Instance{{InstanceID: "inst-1", Status: "connected"}}, nil)
	groupRepo.On("GetGroupsByIDs", ctx, "user-1", []string{"db-group-nonexistent"}).Return([]models.Group{}, nil)
	jobRepo.On("UpdateStatus", mock.Anything, "job-1", models.StandaloneJobStatusFailed, mock.AnythingOfType("*string")).Return(nil)

	result, err := svc.ProcessScheduledJobs(ctx)

	require.NoError(t, err)
	assert.Equal(t, 0, result.Processed)
	assert.Equal(t, 1, result.Failed)
}

// =============================================================================
// ProcessScheduledJobs — Unknown Action Type
// =============================================================================

func TestProcessScheduledJobs_UnknownActionType(t *testing.T) {
	svc, jobRepo, instanceRepo, groupRepo, _, _ := newTestService(t)
	ctx := context.Background()

	pendingJobs := []models.StandaloneJob{
		{
			ID:         "job-1",
			UserID:     "user-1",
			ActionType: "unknown_action",
			RetryCount: 3, // Exhausted so it goes to failed path directly
			MaxRetries: 3,
			Payload: models.JobPayload{
				Groups: []string{"db-group-1"},
			},
		},
	}

	resolvedGroups := []models.Group{
		{ID: "db-group-1", WAGroupID: "group1@g.us", IsAdmin: true},
	}

	jobRepo.On("ClaimPendingDueJobs", ctx, 50).Return(pendingJobs, nil)
	instanceRepo.On("GetUserInstances", ctx, "user-1").Return([]models.Instance{{InstanceID: "inst-1", Status: "connected"}}, nil)
	groupRepo.On("GetGroupsByIDs", ctx, "user-1", []string{"db-group-1"}).Return(resolvedGroups, nil)
	jobRepo.On("UpdateStatus", mock.Anything, "job-1", models.StandaloneJobStatusFailed, mock.AnythingOfType("*string")).Return(nil)

	result, err := svc.ProcessScheduledJobs(ctx)

	require.NoError(t, err)
	assert.Equal(t, 0, result.Processed)
	assert.Equal(t, 1, result.Failed)
}

// =============================================================================
// ProcessScheduledJobs — Partial Success
// =============================================================================

func TestProcessScheduledJobs_PartialSuccess(t *testing.T) {
	svc, jobRepo, instanceRepo, groupRepo, waProvider, _ := newTestService(t)
	ctx := context.Background()

	callCount := 0
	waProvider.SendTextFunc = func(_ context.Context, input whatsapp.SendTextInput) (*whatsapp.MessageResult, error) {
		callCount++
		if callCount == 2 {
			return nil, errors.New("failed for group2")
		}
		return &whatsapp.MessageResult{ID: "msg"}, nil
	}

	pendingJobs := []models.StandaloneJob{
		{
			ID:         "job-1",
			UserID:     "user-1",
			ActionType: models.JobActionSendMessage,
			MaxRetries: 3,
			Payload: models.JobPayload{
				Groups:  []string{"db-group-1", "db-group-2"},
				Message: "Test",
			},
		},
	}

	resolvedGroups := []models.Group{
		{ID: "db-group-1", WAGroupID: "group1@g.us", IsAdmin: true},
		{ID: "db-group-2", WAGroupID: "group2@g.us", IsAdmin: true},
	}

	jobRepo.On("ClaimPendingDueJobs", ctx, 50).Return(pendingJobs, nil)
	instanceRepo.On("GetUserInstances", ctx, "user-1").Return([]models.Instance{{InstanceID: "inst-1", Status: "connected"}}, nil)
	groupRepo.On("GetGroupsByIDs", ctx, "user-1", []string{"db-group-1", "db-group-2"}).Return(resolvedGroups, nil)
	jobRepo.On("UpdateStatus", mock.Anything, "job-1", models.StandaloneJobStatusDone, mock.AnythingOfType("*string")).Return(nil)

	result, err := svc.ProcessScheduledJobs(ctx)

	require.NoError(t, err)
	assert.Equal(t, 1, result.Processed) // Partial success = done
	assert.Equal(t, 0, result.Failed)
}

// =============================================================================
// ProcessScheduledJobs — All Action Types
// =============================================================================

func TestProcessScheduledJobs_AllActionTypes(t *testing.T) {
	actionTypes := []struct {
		actionType models.JobActionType
		payload    models.JobPayload
		method     string
	}{
		{models.JobActionSendPoll, models.JobPayload{Groups: []string{"db-1"}, Question: "Q?", Options: []string{"A", "B"}, SelectableCount: 1}, "SendPoll"},
		{models.JobActionUpdateDescription, models.JobPayload{Groups: []string{"db-1"}, Description: "Desc"}, "UpdateGroupDescription"},
		{models.JobActionCloseGroups, models.JobPayload{Groups: []string{"db-1"}}, "SetGroupAnnouncement"},
		{models.JobActionOpenGroups, models.JobPayload{Groups: []string{"db-1"}}, "SetGroupAnnouncement"},
		{models.JobActionChangeGroupName, models.JobPayload{Groups: []string{"db-1"}, Name: "Name"}, "UpdateGroupSubject"},
		{models.JobActionChangeGroupPhoto, models.JobPayload{Groups: []string{"db-1"}, Image: "data"}, "UpdateGroupPicture"},
	}

	for _, tt := range actionTypes {
		t.Run(string(tt.actionType), func(t *testing.T) {
			svc, jobRepo, instanceRepo, groupRepo, waProvider, _ := newTestService(t)
			ctx := context.Background()

			pendingJobs := []models.StandaloneJob{
				{
					ID:         "job-1",
					UserID:     "user-1",
					ActionType: tt.actionType,
					MaxRetries: 3,
					Payload:    tt.payload,
				},
			}

			resolvedGroups := []models.Group{
				{ID: "db-1", WAGroupID: "group1@g.us", IsAdmin: true},
			}

			jobRepo.On("ClaimPendingDueJobs", ctx, 50).Return(pendingJobs, nil)
			instanceRepo.On("GetUserInstances", ctx, "user-1").Return([]models.Instance{{InstanceID: "inst-1", Status: "connected"}}, nil)
			groupRepo.On("GetGroupsByIDs", ctx, "user-1", []string{"db-1"}).Return(resolvedGroups, nil)
			jobRepo.On("UpdateStatus", mock.Anything, "job-1", models.StandaloneJobStatusDone, (*string)(nil)).Return(nil)

			result, err := svc.ProcessScheduledJobs(ctx)

			require.NoError(t, err)
			assert.Equal(t, 1, result.Processed)
			assert.Equal(t, 1, waProvider.CallCount(tt.method))
		})
	}
}

// =============================================================================
// Multiple Jobs in Single Batch
// =============================================================================

func TestProcessScheduledJobs_MultipleJobs(t *testing.T) {
	svc, jobRepo, instanceRepo, groupRepo, waProvider, _ := newTestService(t)
	ctx := context.Background()

	pendingJobs := []models.StandaloneJob{
		{
			ID: "job-1", UserID: "user-1", ActionType: models.JobActionSendMessage, MaxRetries: 3,
			Payload: models.JobPayload{Groups: []string{"db-1"}, Message: "Msg1"},
		},
		{
			ID: "job-2", UserID: "user-1", ActionType: models.JobActionCloseGroups, MaxRetries: 3,
			Payload: models.JobPayload{Groups: []string{"db-2"}},
		},
	}

	resolvedGroups1 := []models.Group{{ID: "db-1", WAGroupID: "g1@g.us", IsAdmin: true}}
	resolvedGroups2 := []models.Group{{ID: "db-2", WAGroupID: "g2@g.us", IsAdmin: true}}

	jobRepo.On("ClaimPendingDueJobs", ctx, 50).Return(pendingJobs, nil)
	instanceRepo.On("GetUserInstances", ctx, "user-1").Return([]models.Instance{{InstanceID: "inst-1", Status: "connected"}}, nil)
	groupRepo.On("GetGroupsByIDs", ctx, "user-1", []string{"db-1"}).Return(resolvedGroups1, nil)
	groupRepo.On("GetGroupsByIDs", ctx, "user-1", []string{"db-2"}).Return(resolvedGroups2, nil)
	jobRepo.On("UpdateStatus", mock.Anything, "job-1", models.StandaloneJobStatusDone, (*string)(nil)).Return(nil)
	jobRepo.On("UpdateStatus", mock.Anything, "job-2", models.StandaloneJobStatusDone, (*string)(nil)).Return(nil)

	result, err := svc.ProcessScheduledJobs(ctx)

	require.NoError(t, err)
	assert.Equal(t, 2, result.Processed)
	assert.Equal(t, 1, waProvider.CallCount("SendText"))
	assert.Equal(t, 1, waProvider.CallCount("SetGroupAnnouncement"))
}

// =============================================================================
// ProcessScheduledJobs — UpdateStatus Done Error (observability path)
// =============================================================================

func TestProcessScheduledJobs_UpdateStatusDoneError(t *testing.T) {
	svc, jobRepo, instanceRepo, groupRepo, _, _ := newTestService(t)
	ctx := context.Background()

	pendingJobs := []models.StandaloneJob{
		{
			ID: "job-1", UserID: "user-1", ActionType: models.JobActionSendMessage, MaxRetries: 3,
			Payload: models.JobPayload{Groups: []string{"db-1"}, Message: "Test"},
		},
	}

	resolvedGroups := []models.Group{{ID: "db-1", WAGroupID: "g1@g.us", IsAdmin: true}}

	jobRepo.On("ClaimPendingDueJobs", ctx, 50).Return(pendingJobs, nil)
	instanceRepo.On("GetUserInstances", ctx, "user-1").Return([]models.Instance{{InstanceID: "inst-1", Status: "connected"}}, nil)
	groupRepo.On("GetGroupsByIDs", ctx, "user-1", []string{"db-1"}).Return(resolvedGroups, nil)
	// Done update fails — should still count as processed
	jobRepo.On("UpdateStatus", mock.Anything, "job-1", models.StandaloneJobStatusDone, (*string)(nil)).Return(errors.New("status update error"))

	result, err := svc.ProcessScheduledJobs(ctx)

	require.NoError(t, err)
	assert.Equal(t, 1, result.Processed)
}

// =============================================================================
// ProcessScheduledJobs — Retries Exhausted with UpdateStatus Error
// =============================================================================

func TestProcessScheduledJobs_RetriesExhausted_UpdateStatusError(t *testing.T) {
	svc, jobRepo, instanceRepo, groupRepo, waProvider, _ := newTestService(t)
	ctx := context.Background()

	waProvider.SimulateError = errors.New("WAHA down")

	pendingJobs := []models.StandaloneJob{
		{
			ID: "job-1", UserID: "user-1", ActionType: models.JobActionSendMessage,
			RetryCount: 3, MaxRetries: 3,
			Payload: models.JobPayload{Groups: []string{"db-1"}, Message: "Test"},
		},
	}

	resolvedGroups := []models.Group{{ID: "db-1", WAGroupID: "g1@g.us", IsAdmin: true}}

	jobRepo.On("ClaimPendingDueJobs", ctx, 50).Return(pendingJobs, nil)
	instanceRepo.On("GetUserInstances", ctx, "user-1").Return([]models.Instance{{InstanceID: "inst-1", Status: "connected"}}, nil)
	groupRepo.On("GetGroupsByIDs", ctx, "user-1", []string{"db-1"}).Return(resolvedGroups, nil)
	// UpdateStatus to failed also fails — observability path
	jobRepo.On("UpdateStatus", mock.Anything, "job-1", models.StandaloneJobStatusFailed, mock.AnythingOfType("*string")).Return(errors.New("db error"))

	result, err := svc.ProcessScheduledJobs(ctx)

	require.NoError(t, err)
	assert.Equal(t, 1, result.Failed)
}

// =============================================================================
// ProcessScheduledJobs — Instance Error with UpdateStatus Error
// =============================================================================

func TestProcessScheduledJobs_InstanceError_UpdateStatusError(t *testing.T) {
	svc, jobRepo, instanceRepo, _, _, _ := newTestService(t)
	ctx := context.Background()

	pendingJobs := []models.StandaloneJob{
		{
			ID: "job-1", UserID: "user-1", ActionType: models.JobActionSendMessage,
			Payload: validJobPayload([]string{"g1@g.us"}),
		},
	}

	jobRepo.On("ClaimPendingDueJobs", ctx, 50).Return(pendingJobs, nil)
	instanceRepo.On("GetUserInstances", ctx, "user-1").Return(nil, errors.New("instance error"))
	// UpdateStatus to failed also fails
	jobRepo.On("UpdateStatus", mock.Anything, "job-1", models.StandaloneJobStatusFailed, mock.AnythingOfType("*string")).Return(errors.New("db error"))

	result, err := svc.ProcessScheduledJobs(ctx)

	require.NoError(t, err)
	assert.Equal(t, 1, result.Failed)
}

// =============================================================================
// Standalone Service — ScheduleJob and CancelJob (unsupported)
// =============================================================================

func TestStandaloneService_ScheduleJob_Unsupported(t *testing.T) {
	svc, _, _, _, _, _ := newTestService(t)
	ctx := context.Background()

	err := svc.ScheduleJob(ctx, &models.ScheduledJob{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not supported")
}

func TestStandaloneService_CancelJob_Unsupported(t *testing.T) {
	svc, _, _, _, _, _ := newTestService(t)
	ctx := context.Background()

	err := svc.CancelJob(ctx, "some-id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not supported")
}

// =============================================================================
// Standalone Service — GetQueueStatus
// =============================================================================

func TestStandaloneGetQueueStatus_Success(t *testing.T) {
	svc, jobRepo, _, _, _, _ := newTestService(t)
	ctx := context.Background()

	counts := map[models.StandaloneJobStatus]int{
		models.StandaloneJobStatusPending: 5,
		models.StandaloneJobStatusRunning: 2,
		models.StandaloneJobStatusDone:    10,
		models.StandaloneJobStatusFailed:  1,
	}
	jobRepo.On("CountByStatus", ctx, "user-1").Return(counts, nil)

	status, err := svc.GetQueueStatus(ctx, "user-1")

	require.NoError(t, err)
	assert.Equal(t, 5, status.Pending)
	assert.Equal(t, 2, status.Running)
	assert.Equal(t, 10, status.Completed)
	assert.Equal(t, 1, status.Failed)
}

func TestStandaloneGetQueueStatus_Error(t *testing.T) {
	svc, jobRepo, _, _, _, _ := newTestService(t)
	ctx := context.Background()

	jobRepo.On("CountByStatus", ctx, "user-1").Return(nil, errors.New("db error"))

	status, err := svc.GetQueueStatus(ctx, "user-1")

	assert.Error(t, err)
	assert.Nil(t, status)
}

// =============================================================================
// History Integration — createActionHistory Error Path
// =============================================================================

func TestProcessScheduledJobs_HistoryCreateError_StillProcesses(t *testing.T) {
	svc, jobRepo, instanceRepo, groupRepo, _, _, notifier, historyRepo, _ := newFullTestService(t)
	ctx := context.Background()

	pendingJobs := []models.StandaloneJob{
		{
			ID: "job-1", UserID: "user-1", ActionType: models.JobActionSendMessage, MaxRetries: 3,
			Payload: models.JobPayload{Groups: []string{"db-1"}, Message: "Test"},
		},
	}

	resolvedGroups := []models.Group{{ID: "db-1", WAGroupID: "g1@g.us", IsAdmin: true}}

	jobRepo.On("ClaimPendingDueJobs", ctx, 50).Return(pendingJobs, nil)
	instanceRepo.On("GetUserInstances", ctx, "user-1").Return([]models.Instance{{InstanceID: "inst-1", Status: "connected"}}, nil)
	groupRepo.On("GetGroupsByIDs", ctx, "user-1", []string{"db-1"}).Return(resolvedGroups, nil)
	jobRepo.On("UpdateStatus", mock.Anything, "job-1", models.StandaloneJobStatusDone, (*string)(nil)).Return(nil)

	notifier.On("PublishJobStatus", "user-1", "job-1", "running", (*string)(nil)).Return()
	notifier.On("PublishJobStatus", "user-1", "job-1", "done", (*string)(nil)).Return()

	// History create fails — should still process the job
	historyRepo.On("Create", ctx, "user-1", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.Anything).
		Return(nil, errors.New("history create error"))

	result, err := svc.ProcessScheduledJobs(ctx)

	require.NoError(t, err)
	assert.Equal(t, 1, result.Processed)
}

// =============================================================================
// History Integration — failActionHistory with historyRepo nil
// =============================================================================

func TestProcessScheduledJobs_FailActionHistory_NoHistoryRepo(t *testing.T) {
	svc, jobRepo, instanceRepo, _, _, _ := newTestService(t) // nil historyRepo
	ctx := context.Background()

	pendingJobs := []models.StandaloneJob{
		{
			ID: "job-1", UserID: "user-1", ActionType: models.JobActionSendMessage,
			Payload: validJobPayload([]string{"g1@g.us"}),
		},
	}

	jobRepo.On("ClaimPendingDueJobs", ctx, 50).Return(pendingJobs, nil)
	instanceRepo.On("GetUserInstances", ctx, "user-1").Return([]models.Instance{}, nil)
	jobRepo.On("UpdateStatus", mock.Anything, "job-1", models.StandaloneJobStatusFailed, mock.AnythingOfType("*string")).Return(nil)

	result, err := svc.ProcessScheduledJobs(ctx)

	require.NoError(t, err)
	assert.Equal(t, 1, result.Failed)
}

// =============================================================================
// History Integration — completeActionHistory Error Paths
// =============================================================================

func TestProcessScheduledJobs_CompleteHistory_UpdateCountsError(t *testing.T) {
	svc, jobRepo, instanceRepo, groupRepo, _, _, notifier, historyRepo, historyNotifier := newFullTestService(t)
	ctx := context.Background()

	pendingJobs := []models.StandaloneJob{
		{
			ID: "job-1", UserID: "user-1", ActionType: models.JobActionSendMessage, MaxRetries: 3,
			Payload: models.JobPayload{Groups: []string{"db-1"}, Message: "Test"},
		},
	}

	resolvedGroups := []models.Group{{ID: "db-1", WAGroupID: "g1@g.us", IsAdmin: true}}

	jobRepo.On("ClaimPendingDueJobs", ctx, 50).Return(pendingJobs, nil)
	instanceRepo.On("GetUserInstances", ctx, "user-1").Return([]models.Instance{{InstanceID: "inst-1", Status: "connected"}}, nil)
	groupRepo.On("GetGroupsByIDs", ctx, "user-1", []string{"db-1"}).Return(resolvedGroups, nil)
	jobRepo.On("UpdateStatus", mock.Anything, "job-1", models.StandaloneJobStatusDone, (*string)(nil)).Return(nil)

	notifier.On("PublishJobStatus", "user-1", "job-1", "running", (*string)(nil)).Return()
	notifier.On("PublishJobStatus", "user-1", "job-1", "done", (*string)(nil)).Return()

	historyRepo.On("Create", ctx, "user-1", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.Anything).
		Return(&models.ActionHistory{ID: "hist-1"}, nil)
	historyRepo.On("Update", mock.Anything, "hist-1", mock.Anything).Return(nil)
	// UpdateCounts fails
	historyRepo.On("UpdateCounts", mock.Anything, "hist-1", mock.AnythingOfType("int32"), mock.AnythingOfType("int32")).Return(errors.New("counts error"))
	historyRepo.On("UpdateStatus", mock.Anything, "hist-1", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(nil)

	historyNotifier.On("PublishHistoryChange", "user-1", "hist-1", mock.AnythingOfType("string")).Return()

	result, err := svc.ProcessScheduledJobs(ctx)

	require.NoError(t, err)
	assert.Equal(t, 1, result.Processed)
}

func TestProcessScheduledJobs_CompleteHistory_UpdateStatusError(t *testing.T) {
	svc, jobRepo, instanceRepo, groupRepo, _, _, notifier, historyRepo, historyNotifier := newFullTestService(t)
	ctx := context.Background()

	pendingJobs := []models.StandaloneJob{
		{
			ID: "job-1", UserID: "user-1", ActionType: models.JobActionSendMessage, MaxRetries: 3,
			Payload: models.JobPayload{Groups: []string{"db-1"}, Message: "Test"},
		},
	}

	resolvedGroups := []models.Group{{ID: "db-1", WAGroupID: "g1@g.us", IsAdmin: true}}

	jobRepo.On("ClaimPendingDueJobs", ctx, 50).Return(pendingJobs, nil)
	instanceRepo.On("GetUserInstances", ctx, "user-1").Return([]models.Instance{{InstanceID: "inst-1", Status: "connected"}}, nil)
	groupRepo.On("GetGroupsByIDs", ctx, "user-1", []string{"db-1"}).Return(resolvedGroups, nil)
	jobRepo.On("UpdateStatus", mock.Anything, "job-1", models.StandaloneJobStatusDone, (*string)(nil)).Return(nil)

	notifier.On("PublishJobStatus", "user-1", "job-1", "running", (*string)(nil)).Return()
	notifier.On("PublishJobStatus", "user-1", "job-1", "done", (*string)(nil)).Return()

	historyRepo.On("Create", ctx, "user-1", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.Anything).
		Return(&models.ActionHistory{ID: "hist-1"}, nil)
	historyRepo.On("Update", mock.Anything, "hist-1", mock.Anything).Return(nil)
	historyRepo.On("UpdateCounts", mock.Anything, "hist-1", mock.AnythingOfType("int32"), mock.AnythingOfType("int32")).Return(nil)
	// UpdateStatus fails
	historyRepo.On("UpdateStatus", mock.Anything, "hist-1", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(errors.New("status error"))

	historyNotifier.On("PublishHistoryChange", "user-1", "hist-1", mock.AnythingOfType("string")).Return()

	result, err := svc.ProcessScheduledJobs(ctx)

	require.NoError(t, err)
	assert.Equal(t, 1, result.Processed)
}

// =============================================================================
// History Integration — failActionHistory with MarkFailed Error
// =============================================================================

func TestProcessScheduledJobs_FailHistory_MarkFailedError(t *testing.T) {
	svc, jobRepo, instanceRepo, _, _, _, notifier, historyRepo, historyNotifier := newFullTestService(t)
	ctx := context.Background()

	pendingJobs := []models.StandaloneJob{
		{
			ID: "job-1", UserID: "user-1", ActionType: models.JobActionSendMessage,
			Payload: validJobPayload([]string{"g1@g.us"}),
		},
	}

	jobRepo.On("ClaimPendingDueJobs", ctx, 50).Return(pendingJobs, nil)
	instanceRepo.On("GetUserInstances", ctx, "user-1").Return([]models.Instance{}, nil)
	jobRepo.On("UpdateStatus", mock.Anything, "job-1", models.StandaloneJobStatusFailed, mock.AnythingOfType("*string")).Return(nil)

	notifier.On("PublishJobStatus", "user-1", "job-1", "running", (*string)(nil)).Return()
	notifier.On("PublishJobStatus", "user-1", "job-1", "failed", mock.AnythingOfType("*string")).Return()

	historyRepo.On("Create", ctx, "user-1", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.Anything).
		Return(&models.ActionHistory{ID: "hist-1"}, nil)
	historyRepo.On("Update", mock.Anything, "hist-1", mock.Anything).Return(nil)
	// MarkFailed fails
	historyRepo.On("MarkFailed", mock.Anything, "hist-1", mock.AnythingOfType("string")).Return(errors.New("mark failed error"))

	historyNotifier.On("PublishHistoryChange", "user-1", "hist-1", mock.AnythingOfType("string")).Return()

	result, err := svc.ProcessScheduledJobs(ctx)

	require.NoError(t, err)
	assert.Equal(t, 1, result.Failed)
}

// =============================================================================
// History Integration — createActionHistory returns nil history
// =============================================================================

func TestProcessScheduledJobs_HistoryCreateReturnsNil(t *testing.T) {
	svc, jobRepo, instanceRepo, groupRepo, _, _, notifier, historyRepo, _ := newFullTestService(t)
	ctx := context.Background()

	pendingJobs := []models.StandaloneJob{
		{
			ID: "job-1", UserID: "user-1", ActionType: models.JobActionSendMessage, MaxRetries: 3,
			Payload: models.JobPayload{Groups: []string{"db-1"}, Message: "Test"},
		},
	}

	resolvedGroups := []models.Group{{ID: "db-1", WAGroupID: "g1@g.us", IsAdmin: true}}

	jobRepo.On("ClaimPendingDueJobs", ctx, 50).Return(pendingJobs, nil)
	instanceRepo.On("GetUserInstances", ctx, "user-1").Return([]models.Instance{{InstanceID: "inst-1", Status: "connected"}}, nil)
	groupRepo.On("GetGroupsByIDs", ctx, "user-1", []string{"db-1"}).Return(resolvedGroups, nil)
	jobRepo.On("UpdateStatus", mock.Anything, "job-1", models.StandaloneJobStatusDone, (*string)(nil)).Return(nil)

	notifier.On("PublishJobStatus", "user-1", "job-1", "running", (*string)(nil)).Return()
	notifier.On("PublishJobStatus", "user-1", "job-1", "done", (*string)(nil)).Return()

	// Create returns nil history (no error but nil) - historyID will be ""
	historyRepo.On("Create", ctx, "user-1", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.Anything).
		Return((*models.ActionHistory)(nil), nil)

	result, err := svc.ProcessScheduledJobs(ctx)

	require.NoError(t, err)
	assert.Equal(t, 1, result.Processed)
}

// =============================================================================
// History Integration — createActionHistory Update error
// =============================================================================

func TestProcessScheduledJobs_HistoryUpdateTargetCountError(t *testing.T) {
	svc, jobRepo, instanceRepo, groupRepo, _, _, notifier, historyRepo, historyNotifier := newFullTestService(t)
	ctx := context.Background()

	pendingJobs := []models.StandaloneJob{
		{
			ID: "job-1", UserID: "user-1", ActionType: models.JobActionSendMessage, MaxRetries: 3,
			Payload: models.JobPayload{Groups: []string{"db-1"}, Message: "Test"},
		},
	}

	resolvedGroups := []models.Group{{ID: "db-1", WAGroupID: "g1@g.us", IsAdmin: true}}

	jobRepo.On("ClaimPendingDueJobs", ctx, 50).Return(pendingJobs, nil)
	instanceRepo.On("GetUserInstances", ctx, "user-1").Return([]models.Instance{{InstanceID: "inst-1", Status: "connected"}}, nil)
	groupRepo.On("GetGroupsByIDs", ctx, "user-1", []string{"db-1"}).Return(resolvedGroups, nil)
	jobRepo.On("UpdateStatus", mock.Anything, "job-1", models.StandaloneJobStatusDone, (*string)(nil)).Return(nil)

	notifier.On("PublishJobStatus", "user-1", "job-1", "running", (*string)(nil)).Return()
	notifier.On("PublishJobStatus", "user-1", "job-1", "done", (*string)(nil)).Return()

	historyRepo.On("Create", ctx, "user-1", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.Anything).
		Return(&models.ActionHistory{ID: "hist-1"}, nil)
	// Update (target_count) fails
	historyRepo.On("Update", mock.Anything, "hist-1", mock.Anything).Return(errors.New("update error"))
	historyRepo.On("UpdateCounts", mock.Anything, "hist-1", mock.AnythingOfType("int32"), mock.AnythingOfType("int32")).Return(nil)
	historyRepo.On("UpdateStatus", mock.Anything, "hist-1", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(nil)

	historyNotifier.On("PublishHistoryChange", "user-1", "hist-1", mock.AnythingOfType("string")).Return()

	result, err := svc.ProcessScheduledJobs(ctx)

	require.NoError(t, err)
	assert.Equal(t, 1, result.Processed)
}

// =============================================================================
// ProcessScheduledJobs — All Action Types for History Type Mapping
// =============================================================================

func TestProcessScheduledJobs_HistoryMapping_AllTypes(t *testing.T) {
	actionTypes := []struct {
		actionType models.JobActionType
		payload    models.JobPayload
	}{
		{models.JobActionSendPoll, models.JobPayload{Groups: []string{"db-1"}, Question: "Q?", Options: []string{"A", "B"}, SelectableCount: 1}},
		{models.JobActionUpdateDescription, models.JobPayload{Groups: []string{"db-1"}, Description: "Desc"}},
		{models.JobActionCloseGroups, models.JobPayload{Groups: []string{"db-1"}}},
		{models.JobActionOpenGroups, models.JobPayload{Groups: []string{"db-1"}}},
		{models.JobActionChangeGroupName, models.JobPayload{Groups: []string{"db-1"}, Name: "Name"}},
		{models.JobActionChangeGroupPhoto, models.JobPayload{Groups: []string{"db-1"}, Image: "data"}},
	}

	for _, tt := range actionTypes {
		t.Run(string(tt.actionType), func(t *testing.T) {
			svc, jobRepo, instanceRepo, groupRepo, _, _, notifier, historyRepo, historyNotifier := newFullTestService(t)
			ctx := context.Background()

			pendingJobs := []models.StandaloneJob{
				{
					ID: "job-1", UserID: "user-1", ActionType: tt.actionType, MaxRetries: 3,
					Payload: tt.payload,
				},
			}

			resolvedGroups := []models.Group{{ID: "db-1", WAGroupID: "g1@g.us", IsAdmin: true}}

			jobRepo.On("ClaimPendingDueJobs", ctx, 50).Return(pendingJobs, nil)
			instanceRepo.On("GetUserInstances", ctx, "user-1").Return([]models.Instance{{InstanceID: "inst-1", Status: "connected"}}, nil)
			groupRepo.On("GetGroupsByIDs", ctx, "user-1", []string{"db-1"}).Return(resolvedGroups, nil)
			jobRepo.On("UpdateStatus", mock.Anything, "job-1", models.StandaloneJobStatusDone, (*string)(nil)).Return(nil)

			notifier.On("PublishJobStatus", "user-1", "job-1", mock.AnythingOfType("string"), mock.Anything).Return()

			historyRepo.On("Create", ctx, "user-1", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.Anything).
				Return(&models.ActionHistory{ID: "hist-1"}, nil)
			historyRepo.On("Update", mock.Anything, "hist-1", mock.Anything).Return(nil)
			historyRepo.On("UpdateCounts", mock.Anything, "hist-1", mock.AnythingOfType("int32"), mock.AnythingOfType("int32")).Return(nil)
			historyRepo.On("UpdateStatus", mock.Anything, "hist-1", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(nil)
			historyNotifier.On("PublishHistoryChange", "user-1", "hist-1", mock.AnythingOfType("string")).Return()

			result, err := svc.ProcessScheduledJobs(ctx)

			require.NoError(t, err)
			assert.Equal(t, 1, result.Processed)
			historyRepo.AssertExpectations(t)
		})
	}
}

// =============================================================================
// Legacy Service Tests
// =============================================================================

// MockScheduledJobRepository implements repository.ScheduledJobRepository for testing
type MockScheduledJobRepository struct {
	mock.Mock
}

func (m *MockScheduledJobRepository) GetPendingJobs(ctx context.Context) ([]models.ScheduledJob, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.ScheduledJob), args.Error(1)
}

func (m *MockScheduledJobRepository) UpdateJobStatus(ctx context.Context, jobID, status string, errorMsg *string) error {
	args := m.Called(ctx, jobID, status, errorMsg)
	return args.Error(0)
}

func (m *MockScheduledJobRepository) CreateJob(ctx context.Context, job *models.ScheduledJob) (*models.ScheduledJob, error) {
	args := m.Called(ctx, job)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ScheduledJob), args.Error(1)
}

func (m *MockScheduledJobRepository) GetJobByID(ctx context.Context, jobID string) (*models.ScheduledJob, error) {
	args := m.Called(ctx, jobID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ScheduledJob), args.Error(1)
}

// MockAutomationRepository implements repository.AutomationRepository for testing
type MockAutomationRepository struct {
	mock.Mock
}

func (m *MockAutomationRepository) GetUserAutomations(ctx context.Context, userID string) ([]models.Automation, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Automation), args.Error(1)
}

func (m *MockAutomationRepository) GetActiveAutomations(ctx context.Context) ([]models.Automation, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Automation), args.Error(1)
}

func (m *MockAutomationRepository) GetAutomationByID(ctx context.Context, automationID string) (*models.Automation, error) {
	args := m.Called(ctx, automationID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Automation), args.Error(1)
}

func (m *MockAutomationRepository) GetAutomationByIDAndUser(ctx context.Context, automationID, userID string) (*models.Automation, error) {
	args := m.Called(ctx, automationID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Automation), args.Error(1)
}

func (m *MockAutomationRepository) CreateAutomation(ctx context.Context, userID string, data *models.Automation) (*models.Automation, error) {
	args := m.Called(ctx, userID, data)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Automation), args.Error(1)
}

func (m *MockAutomationRepository) UpdateAutomation(ctx context.Context, automationID string, updates map[string]interface{}) error {
	args := m.Called(ctx, automationID, updates)
	return args.Error(0)
}

func (m *MockAutomationRepository) DeleteAutomation(ctx context.Context, automationID, userID string) error {
	args := m.Called(ctx, automationID, userID)
	return args.Error(0)
}

func (m *MockAutomationRepository) GetMonitoringAutomationsForGroup(ctx context.Context, waGroupID string) ([]models.Automation, error) {
	args := m.Called(ctx, waGroupID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Automation), args.Error(1)
}

func (m *MockAutomationRepository) UpdateLastRunAt(ctx context.Context, automationID string) error {
	args := m.Called(ctx, automationID)
	return args.Error(0)
}

func (m *MockAutomationRepository) UpdateNextRunAt(ctx context.Context, automationID string, nextRun time.Time) error {
	args := m.Called(ctx, automationID, nextRun)
	return args.Error(0)
}

func (m *MockAutomationRepository) BulkUpdateStatusByUserID(ctx context.Context, userID, targetStatus string, sourceStatus *string) (int, error) {
	args := m.Called(ctx, userID, targetStatus, sourceStatus)
	return args.Int(0), args.Error(1)
}

func (m *MockAutomationRepository) BulkUpdateNextRunAts(ctx context.Context, automationIDs []string, nextRunAts []time.Time) error {
	args := m.Called(ctx, automationIDs, nextRunAts)
	return args.Error(0)
}

func (m *MockAutomationRepository) GetOfferGenerateAutomationsForGroup(ctx context.Context, waGroupID string) ([]models.Automation, error) {
	args := m.Called(ctx, waGroupID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Automation), args.Error(1)
}

func newLegacyTestService(t *testing.T) (jobs.Service, *MockScheduledJobRepository, *MockAutomationRepository, *MockInstanceRepository) {
	t.Helper()
	logger := zaptest.NewLogger(t)

	scheduledJobRepo := new(MockScheduledJobRepository)
	automationRepo := new(MockAutomationRepository)
	instanceRepo := new(MockInstanceRepository)

	svc := jobs.NewJobsService(scheduledJobRepo, automationRepo, instanceRepo, nil, nil, logger)

	return svc, scheduledJobRepo, automationRepo, instanceRepo
}

// =============================================================================
// Legacy Service — Unsupported Methods
// =============================================================================

func TestLegacyService_CreateJob_Unsupported(t *testing.T) {
	svc, _, _, _ := newLegacyTestService(t)
	_, err := svc.CreateJob(context.Background(), "user-1", &models.StandaloneJob{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not supported")
}

func TestLegacyService_GetJob_Unsupported(t *testing.T) {
	svc, _, _, _ := newLegacyTestService(t)
	_, err := svc.GetJob(context.Background(), "id", "user-1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not supported")
}

func TestLegacyService_ListJobs_Unsupported(t *testing.T) {
	svc, _, _, _ := newLegacyTestService(t)
	_, err := svc.ListJobs(context.Background(), "user-1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not supported")
}

func TestLegacyService_ListJobsPaginated_Unsupported(t *testing.T) {
	svc, _, _, _ := newLegacyTestService(t)
	_, err := svc.ListJobsPaginated(context.Background(), repository.JobListParams{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not supported")
}

func TestLegacyService_UpdateJob_Unsupported(t *testing.T) {
	svc, _, _, _ := newLegacyTestService(t)
	_, err := svc.UpdateJob(context.Background(), "id", "user-1", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not supported")
}

func TestLegacyService_DeleteJob_Unsupported(t *testing.T) {
	svc, _, _, _ := newLegacyTestService(t)
	err := svc.DeleteJob(context.Background(), "id", "user-1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not supported")
}

// =============================================================================
// Legacy Service — ScheduleJob
// =============================================================================

func TestLegacyService_ScheduleJob_Success(t *testing.T) {
	svc, scheduledJobRepo, _, _ := newLegacyTestService(t)
	ctx := context.Background()

	job := &models.ScheduledJob{
		AutomationID: "auto-1",
		ScheduledFor: time.Now().Add(time.Hour).Format(time.RFC3339),
	}

	scheduledJobRepo.On("CreateJob", ctx, mock.AnythingOfType("*models.ScheduledJob")).Return(job, nil)

	err := svc.ScheduleJob(ctx, job)

	assert.NoError(t, err)
	assert.Equal(t, "pending", job.Status)
	scheduledJobRepo.AssertExpectations(t)
}

func TestLegacyService_ScheduleJob_NoAutomationID(t *testing.T) {
	svc, _, _, _ := newLegacyTestService(t)

	err := svc.ScheduleJob(context.Background(), &models.ScheduledJob{ScheduledFor: "2026-01-01T00:00:00Z"})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "automation_id is required")
}

func TestLegacyService_ScheduleJob_NoScheduledFor(t *testing.T) {
	svc, _, _, _ := newLegacyTestService(t)

	err := svc.ScheduleJob(context.Background(), &models.ScheduledJob{AutomationID: "auto-1"})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "scheduled_for is required")
}

// =============================================================================
// Legacy Service — CancelJob
// =============================================================================

func TestLegacyService_CancelJob_Success(t *testing.T) {
	svc, scheduledJobRepo, _, _ := newLegacyTestService(t)
	ctx := context.Background()

	errMsg := "canceled by user"
	scheduledJobRepo.On("UpdateJobStatus", ctx, "job-1", "canceled", &errMsg).Return(nil)

	err := svc.CancelJob(ctx, "job-1")

	assert.NoError(t, err)
	scheduledJobRepo.AssertExpectations(t)
}

// =============================================================================
// Legacy Service — GetQueueStatus
// =============================================================================

func TestLegacyService_GetQueueStatus_Success(t *testing.T) {
	svc, scheduledJobRepo, _, _ := newLegacyTestService(t)
	ctx := context.Background()

	scheduledJobs := []models.ScheduledJob{
		{ID: "1", Status: "pending"},
		{ID: "2", Status: "pending"},
		{ID: "3", Status: "processing"},
		{ID: "4", Status: "completed"},
		{ID: "5", Status: "failed"},
	}

	scheduledJobRepo.On("GetPendingJobs", ctx).Return(scheduledJobs, nil)

	status, err := svc.GetQueueStatus(ctx, "user-1")

	require.NoError(t, err)
	assert.Equal(t, 2, status.Pending)
	assert.Equal(t, 1, status.Running)
	assert.Equal(t, 1, status.Completed)
	assert.Equal(t, 1, status.Failed)
}

func TestLegacyService_GetQueueStatus_Error(t *testing.T) {
	svc, scheduledJobRepo, _, _ := newLegacyTestService(t)
	ctx := context.Background()

	scheduledJobRepo.On("GetPendingJobs", ctx).Return(nil, errors.New("db error"))

	status, err := svc.GetQueueStatus(ctx, "user-1")

	assert.Error(t, err)
	assert.Nil(t, status)
}

// =============================================================================
// Legacy Service — ProcessScheduledJobs
// =============================================================================

func TestLegacyService_ProcessScheduledJobs_FetchError(t *testing.T) {
	svc, scheduledJobRepo, _, _ := newLegacyTestService(t)
	ctx := context.Background()

	scheduledJobRepo.On("GetPendingJobs", ctx).Return(nil, errors.New("db error"))

	result, err := svc.ProcessScheduledJobs(ctx)

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestLegacyService_ProcessScheduledJobs_NoJobs(t *testing.T) {
	svc, scheduledJobRepo, _, _ := newLegacyTestService(t)
	ctx := context.Background()

	scheduledJobRepo.On("GetPendingJobs", ctx).Return([]models.ScheduledJob{}, nil)

	result, err := svc.ProcessScheduledJobs(ctx)

	require.NoError(t, err)
	assert.Equal(t, 0, result.Processed)
}

func TestLegacyService_ProcessScheduledJobs_InvalidScheduledTime(t *testing.T) {
	svc, scheduledJobRepo, _, _ := newLegacyTestService(t)
	ctx := context.Background()

	scheduledJobs := []models.ScheduledJob{
		{ID: "job-1", AutomationID: "auto-1", ScheduledFor: "not-a-date"},
	}

	scheduledJobRepo.On("GetPendingJobs", ctx).Return(scheduledJobs, nil)

	result, err := svc.ProcessScheduledJobs(ctx)

	require.NoError(t, err)
	assert.Equal(t, 0, result.Processed)
	assert.Equal(t, 1, result.Skipped)
}

func TestLegacyService_ProcessScheduledJobs_FutureJob_Skipped(t *testing.T) {
	svc, scheduledJobRepo, _, _ := newLegacyTestService(t)
	ctx := context.Background()

	futureTime := time.Now().Add(24 * time.Hour).Format(time.RFC3339)
	scheduledJobs := []models.ScheduledJob{
		{ID: "job-1", AutomationID: "auto-1", ScheduledFor: futureTime},
	}

	scheduledJobRepo.On("GetPendingJobs", ctx).Return(scheduledJobs, nil)

	result, err := svc.ProcessScheduledJobs(ctx)

	require.NoError(t, err)
	assert.Equal(t, 0, result.Processed)
	assert.Equal(t, 1, result.Skipped)
}

func TestLegacyService_ProcessScheduledJobs_UpdateStatusError_Skips(t *testing.T) {
	svc, scheduledJobRepo, _, _ := newLegacyTestService(t)
	ctx := context.Background()

	pastTime := time.Now().Add(-time.Hour).Format(time.RFC3339)
	scheduledJobs := []models.ScheduledJob{
		{ID: "job-1", AutomationID: "auto-1", ScheduledFor: pastTime},
	}

	scheduledJobRepo.On("GetPendingJobs", ctx).Return(scheduledJobs, nil)
	scheduledJobRepo.On("UpdateJobStatus", ctx, "job-1", "processing", (*string)(nil)).Return(errors.New("lock error"))

	result, err := svc.ProcessScheduledJobs(ctx)

	require.NoError(t, err)
	assert.Equal(t, 1, result.Skipped)
}

func TestLegacyService_ProcessScheduledJobs_AutomationNotFound(t *testing.T) {
	svc, scheduledJobRepo, automationRepo, _ := newLegacyTestService(t)
	ctx := context.Background()

	pastTime := time.Now().Add(-time.Hour).Format(time.RFC3339)
	scheduledJobs := []models.ScheduledJob{
		{ID: "job-1", AutomationID: "auto-1", ScheduledFor: pastTime},
	}

	scheduledJobRepo.On("GetPendingJobs", ctx).Return(scheduledJobs, nil)
	scheduledJobRepo.On("UpdateJobStatus", ctx, "job-1", "processing", (*string)(nil)).Return(nil)
	automationRepo.On("GetAutomationByID", ctx, "auto-1").Return(nil, errors.New("not found"))
	errMsg := "automation not found"
	scheduledJobRepo.On("UpdateJobStatus", ctx, "job-1", "failed", &errMsg).Return(nil)

	result, err := svc.ProcessScheduledJobs(ctx)

	require.NoError(t, err)
	assert.Equal(t, 1, result.Failed)
}

func TestLegacyService_ProcessScheduledJobs_AutomationInactive(t *testing.T) {
	svc, scheduledJobRepo, automationRepo, _ := newLegacyTestService(t)
	ctx := context.Background()

	pastTime := time.Now().Add(-time.Hour).Format(time.RFC3339)
	scheduledJobs := []models.ScheduledJob{
		{ID: "job-1", AutomationID: "auto-1", ScheduledFor: pastTime},
	}

	automation := &models.Automation{
		ID:     "auto-1",
		UserID: "user-1",
		Status: models.AutomationStatusInactive,
	}

	scheduledJobRepo.On("GetPendingJobs", ctx).Return(scheduledJobs, nil)
	scheduledJobRepo.On("UpdateJobStatus", ctx, "job-1", "processing", (*string)(nil)).Return(nil)
	automationRepo.On("GetAutomationByID", ctx, "auto-1").Return(automation, nil)
	errMsg := "automation is not active"
	scheduledJobRepo.On("UpdateJobStatus", ctx, "job-1", "failed", &errMsg).Return(nil)

	result, err := svc.ProcessScheduledJobs(ctx)

	require.NoError(t, err)
	assert.Equal(t, 1, result.Failed)
}

func TestLegacyService_ProcessScheduledJobs_NoInstances(t *testing.T) {
	svc, scheduledJobRepo, automationRepo, instanceRepo := newLegacyTestService(t)
	ctx := context.Background()

	pastTime := time.Now().Add(-time.Hour).Format(time.RFC3339)
	scheduledJobs := []models.ScheduledJob{
		{ID: "job-1", AutomationID: "auto-1", ScheduledFor: pastTime},
	}

	automation := &models.Automation{
		ID:     "auto-1",
		UserID: "user-1",
		Status: models.AutomationStatusActive,
	}

	scheduledJobRepo.On("GetPendingJobs", ctx).Return(scheduledJobs, nil)
	scheduledJobRepo.On("UpdateJobStatus", ctx, "job-1", "processing", (*string)(nil)).Return(nil)
	automationRepo.On("GetAutomationByID", ctx, "auto-1").Return(automation, nil)
	instanceRepo.On("GetUserInstances", ctx, "user-1").Return([]models.Instance{}, nil)
	errMsg := "no connected instance found"
	scheduledJobRepo.On("UpdateJobStatus", ctx, "job-1", "failed", &errMsg).Return(nil)

	result, err := svc.ProcessScheduledJobs(ctx)

	require.NoError(t, err)
	assert.Equal(t, 1, result.Failed)
}

func TestLegacyService_ProcessScheduledJobs_NoConnectedInstance(t *testing.T) {
	svc, scheduledJobRepo, automationRepo, instanceRepo := newLegacyTestService(t)
	ctx := context.Background()

	pastTime := time.Now().Add(-time.Hour).Format(time.RFC3339)
	scheduledJobs := []models.ScheduledJob{
		{ID: "job-1", AutomationID: "auto-1", ScheduledFor: pastTime},
	}

	automation := &models.Automation{
		ID:     "auto-1",
		UserID: "user-1",
		Status: models.AutomationStatusActive,
	}

	scheduledJobRepo.On("GetPendingJobs", ctx).Return(scheduledJobs, nil)
	scheduledJobRepo.On("UpdateJobStatus", ctx, "job-1", "processing", (*string)(nil)).Return(nil)
	automationRepo.On("GetAutomationByID", ctx, "auto-1").Return(automation, nil)
	instanceRepo.On("GetUserInstances", ctx, "user-1").Return([]models.Instance{
		{InstanceID: "inst-1", Status: "disconnected"},
	}, nil)
	errMsg := "no connected instance"
	scheduledJobRepo.On("UpdateJobStatus", ctx, "job-1", "failed", &errMsg).Return(nil)

	result, err := svc.ProcessScheduledJobs(ctx)

	require.NoError(t, err)
	assert.Equal(t, 1, result.Failed)
}

func TestLegacyService_ProcessScheduledJobs_Success_NoTargetGroups(t *testing.T) {
	svc, scheduledJobRepo, automationRepo, instanceRepo := newLegacyTestService(t)
	ctx := context.Background()

	pastTime := time.Now().Add(-time.Hour).Format(time.RFC3339)
	scheduledJobs := []models.ScheduledJob{
		{ID: "job-1", AutomationID: "auto-1", ScheduledFor: pastTime},
	}

	automation := &models.Automation{
		ID:       "auto-1",
		UserID:   "user-1",
		Status:   models.AutomationStatusActive,
		GroupIDs: []string{}, // No target groups
	}

	scheduledJobRepo.On("GetPendingJobs", ctx).Return(scheduledJobs, nil)
	scheduledJobRepo.On("UpdateJobStatus", ctx, "job-1", "processing", (*string)(nil)).Return(nil)
	automationRepo.On("GetAutomationByID", ctx, "auto-1").Return(automation, nil)
	instanceRepo.On("GetUserInstances", ctx, "user-1").Return([]models.Instance{
		{InstanceID: "inst-1", Status: "connected"},
	}, nil)
	errMsg := "no target groups configured"
	scheduledJobRepo.On("UpdateJobStatus", ctx, "job-1", "failed", &errMsg).Return(nil)

	result, err := svc.ProcessScheduledJobs(ctx)

	require.NoError(t, err)
	assert.Equal(t, 1, result.Failed)
}

func TestLegacyService_ProcessScheduledJobs_Success_WithGroups(t *testing.T) {
	svc, scheduledJobRepo, automationRepo, instanceRepo := newLegacyTestService(t)
	ctx := context.Background()

	pastTime := time.Now().Add(-time.Hour).Format(time.RFC3339)
	scheduledJobs := []models.ScheduledJob{
		{ID: "job-1", AutomationID: "auto-1", ScheduledFor: pastTime},
	}

	customMsg := "Oferta especial!"
	automation := &models.Automation{
		ID:            "auto-1",
		UserID:        "user-1",
		Status:        models.AutomationStatusActive,
		GroupIDs:      []string{"group1@g.us", "group2@g.us"},
		CustomMessage: &customMsg,
	}

	scheduledJobRepo.On("GetPendingJobs", ctx).Return(scheduledJobs, nil)
	scheduledJobRepo.On("UpdateJobStatus", ctx, "job-1", "processing", (*string)(nil)).Return(nil)
	automationRepo.On("GetAutomationByID", ctx, "auto-1").Return(automation, nil)
	instanceRepo.On("GetUserInstances", ctx, "user-1").Return([]models.Instance{
		{InstanceID: "inst-1", Status: "connected"},
	}, nil)
	// QueueClient is nil, so enqueue is skipped - executeAutomation returns nil
	scheduledJobRepo.On("UpdateJobStatus", ctx, "job-1", "completed", (*string)(nil)).Return(nil)
	automationRepo.On("UpdateAutomation", ctx, "auto-1", mock.Anything).Return(nil)

	result, err := svc.ProcessScheduledJobs(ctx)

	require.NoError(t, err)
	assert.Equal(t, 1, result.Processed)
}

func TestLegacyService_ProcessScheduledJobs_DefaultMessage(t *testing.T) {
	svc, scheduledJobRepo, automationRepo, instanceRepo := newLegacyTestService(t)
	ctx := context.Background()

	pastTime := time.Now().Add(-time.Hour).Format(time.RFC3339)
	scheduledJobs := []models.ScheduledJob{
		{ID: "job-1", AutomationID: "auto-1", ScheduledFor: pastTime},
	}

	automation := &models.Automation{
		ID:       "auto-1",
		UserID:   "user-1",
		Status:   models.AutomationStatusActive,
		GroupIDs: []string{"group1@g.us"},
		// No CustomMessage - uses default
	}

	scheduledJobRepo.On("GetPendingJobs", ctx).Return(scheduledJobs, nil)
	scheduledJobRepo.On("UpdateJobStatus", ctx, "job-1", "processing", (*string)(nil)).Return(nil)
	automationRepo.On("GetAutomationByID", ctx, "auto-1").Return(automation, nil)
	instanceRepo.On("GetUserInstances", ctx, "user-1").Return([]models.Instance{
		{InstanceID: "inst-1", Status: "connected"},
	}, nil)
	scheduledJobRepo.On("UpdateJobStatus", ctx, "job-1", "completed", (*string)(nil)).Return(nil)
	automationRepo.On("UpdateAutomation", ctx, "auto-1", mock.Anything).Return(nil)

	result, err := svc.ProcessScheduledJobs(ctx)

	require.NoError(t, err)
	assert.Equal(t, 1, result.Processed)
}

func TestLegacyService_ProcessScheduledJobs_CompletedUpdateError(t *testing.T) {
	svc, scheduledJobRepo, automationRepo, instanceRepo := newLegacyTestService(t)
	ctx := context.Background()

	pastTime := time.Now().Add(-time.Hour).Format(time.RFC3339)
	scheduledJobs := []models.ScheduledJob{
		{ID: "job-1", AutomationID: "auto-1", ScheduledFor: pastTime},
	}

	automation := &models.Automation{
		ID:       "auto-1",
		UserID:   "user-1",
		Status:   models.AutomationStatusActive,
		GroupIDs: []string{"group1@g.us"},
	}

	scheduledJobRepo.On("GetPendingJobs", ctx).Return(scheduledJobs, nil)
	scheduledJobRepo.On("UpdateJobStatus", ctx, "job-1", "processing", (*string)(nil)).Return(nil)
	automationRepo.On("GetAutomationByID", ctx, "auto-1").Return(automation, nil)
	instanceRepo.On("GetUserInstances", ctx, "user-1").Return([]models.Instance{
		{InstanceID: "inst-1", Status: "connected"},
	}, nil)
	// Completed status update fails
	scheduledJobRepo.On("UpdateJobStatus", ctx, "job-1", "completed", (*string)(nil)).Return(errors.New("update error"))
	automationRepo.On("UpdateAutomation", ctx, "auto-1", mock.Anything).Return(errors.New("update error"))

	result, err := svc.ProcessScheduledJobs(ctx)

	require.NoError(t, err)
	assert.Equal(t, 1, result.Processed)
}

func TestLegacyService_ProcessScheduledJobs_AutomationNotFound_UpdateStatusError(t *testing.T) {
	svc, scheduledJobRepo, automationRepo, _ := newLegacyTestService(t)
	ctx := context.Background()

	pastTime := time.Now().Add(-time.Hour).Format(time.RFC3339)
	scheduledJobs := []models.ScheduledJob{
		{ID: "job-1", AutomationID: "auto-1", ScheduledFor: pastTime},
	}

	scheduledJobRepo.On("GetPendingJobs", ctx).Return(scheduledJobs, nil)
	scheduledJobRepo.On("UpdateJobStatus", ctx, "job-1", "processing", (*string)(nil)).Return(nil)
	automationRepo.On("GetAutomationByID", ctx, "auto-1").Return(nil, errors.New("not found"))
	errMsg := "automation not found"
	scheduledJobRepo.On("UpdateJobStatus", ctx, "job-1", "failed", &errMsg).Return(errors.New("db error"))

	result, err := svc.ProcessScheduledJobs(ctx)

	require.NoError(t, err)
	assert.Equal(t, 1, result.Failed)
}

func TestLegacyService_ProcessScheduledJobs_InactiveAutomation_UpdateStatusError(t *testing.T) {
	svc, scheduledJobRepo, automationRepo, _ := newLegacyTestService(t)
	ctx := context.Background()

	pastTime := time.Now().Add(-time.Hour).Format(time.RFC3339)
	scheduledJobs := []models.ScheduledJob{
		{ID: "job-1", AutomationID: "auto-1", ScheduledFor: pastTime},
	}

	automation := &models.Automation{ID: "auto-1", Status: models.AutomationStatusInactive}

	scheduledJobRepo.On("GetPendingJobs", ctx).Return(scheduledJobs, nil)
	scheduledJobRepo.On("UpdateJobStatus", ctx, "job-1", "processing", (*string)(nil)).Return(nil)
	automationRepo.On("GetAutomationByID", ctx, "auto-1").Return(automation, nil)
	errMsg := "automation is not active"
	scheduledJobRepo.On("UpdateJobStatus", ctx, "job-1", "failed", &errMsg).Return(errors.New("db error"))

	result, err := svc.ProcessScheduledJobs(ctx)

	require.NoError(t, err)
	assert.Equal(t, 1, result.Failed)
}

func TestLegacyService_ProcessScheduledJobs_NoInstances_UpdateStatusError(t *testing.T) {
	svc, scheduledJobRepo, automationRepo, instanceRepo := newLegacyTestService(t)
	ctx := context.Background()

	pastTime := time.Now().Add(-time.Hour).Format(time.RFC3339)
	scheduledJobs := []models.ScheduledJob{
		{ID: "job-1", AutomationID: "auto-1", ScheduledFor: pastTime},
	}

	automation := &models.Automation{ID: "auto-1", UserID: "user-1", Status: models.AutomationStatusActive}

	scheduledJobRepo.On("GetPendingJobs", ctx).Return(scheduledJobs, nil)
	scheduledJobRepo.On("UpdateJobStatus", ctx, "job-1", "processing", (*string)(nil)).Return(nil)
	automationRepo.On("GetAutomationByID", ctx, "auto-1").Return(automation, nil)
	instanceRepo.On("GetUserInstances", ctx, "user-1").Return(nil, errors.New("instance error"))
	errMsg := "no connected instance found"
	scheduledJobRepo.On("UpdateJobStatus", ctx, "job-1", "failed", &errMsg).Return(errors.New("db error"))

	result, err := svc.ProcessScheduledJobs(ctx)

	require.NoError(t, err)
	assert.Equal(t, 1, result.Failed)
}

func TestLegacyService_ProcessScheduledJobs_NoConnected_UpdateStatusError(t *testing.T) {
	svc, scheduledJobRepo, automationRepo, instanceRepo := newLegacyTestService(t)
	ctx := context.Background()

	pastTime := time.Now().Add(-time.Hour).Format(time.RFC3339)
	scheduledJobs := []models.ScheduledJob{
		{ID: "job-1", AutomationID: "auto-1", ScheduledFor: pastTime},
	}

	automation := &models.Automation{ID: "auto-1", UserID: "user-1", Status: models.AutomationStatusActive}

	scheduledJobRepo.On("GetPendingJobs", ctx).Return(scheduledJobs, nil)
	scheduledJobRepo.On("UpdateJobStatus", ctx, "job-1", "processing", (*string)(nil)).Return(nil)
	automationRepo.On("GetAutomationByID", ctx, "auto-1").Return(automation, nil)
	instanceRepo.On("GetUserInstances", ctx, "user-1").Return([]models.Instance{{InstanceID: "inst-1", Status: "disconnected"}}, nil)
	errMsg := "no connected instance"
	scheduledJobRepo.On("UpdateJobStatus", ctx, "job-1", "failed", &errMsg).Return(errors.New("db error"))

	result, err := svc.ProcessScheduledJobs(ctx)

	require.NoError(t, err)
	assert.Equal(t, 1, result.Failed)
}

// =============================================================================
// ProcessScheduledJobs — ScheduleRetry AND UpdateStatus Both Fail
// =============================================================================

func TestProcessScheduledJobs_ScheduleRetryAndUpdateStatusBothFail(t *testing.T) {
	svc, jobRepo, instanceRepo, groupRepo, waProvider, _, notifier, historyRepo, historyNotifier := newFullTestService(t)
	ctx := context.Background()

	waProvider.SimulateError = errors.New("error")

	pendingJobs := []models.StandaloneJob{
		{
			ID: "job-1", UserID: "user-1", ActionType: models.JobActionSendMessage,
			RetryCount: 0, MaxRetries: 3,
			Payload: models.JobPayload{Groups: []string{"db-1"}, Message: "Test"},
		},
	}

	resolvedGroups := []models.Group{{ID: "db-1", WAGroupID: "g1@g.us", IsAdmin: true}}

	jobRepo.On("ClaimPendingDueJobs", ctx, 50).Return(pendingJobs, nil)
	instanceRepo.On("GetUserInstances", ctx, "user-1").Return([]models.Instance{{InstanceID: "inst-1", Status: "connected"}}, nil)
	groupRepo.On("GetGroupsByIDs", ctx, "user-1", []string{"db-1"}).Return(resolvedGroups, nil)
	// ScheduleRetry fails
	jobRepo.On("ScheduleRetry", mock.Anything, "job-1", mock.AnythingOfType("time.Time")).Return(errors.New("redis down"))
	// UpdateStatus to failed ALSO fails (observability path)
	jobRepo.On("UpdateStatus", mock.Anything, "job-1", models.StandaloneJobStatusFailed, mock.AnythingOfType("*string")).Return(errors.New("db error"))

	notifier.On("PublishJobStatus", "user-1", "job-1", mock.AnythingOfType("string"), mock.Anything).Return()

	historyRepo.On("Create", ctx, "user-1", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.Anything).
		Return(&models.ActionHistory{ID: "hist-1"}, nil)
	historyRepo.On("Update", mock.Anything, "hist-1", mock.Anything).Return(nil)
	historyRepo.On("UpdateCounts", mock.Anything, "hist-1", mock.AnythingOfType("int32"), mock.AnythingOfType("int32")).Return(nil)
	historyRepo.On("UpdateStatus", mock.Anything, "hist-1", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(nil)
	historyNotifier.On("PublishHistoryChange", "user-1", "hist-1", mock.AnythingOfType("string")).Return()

	result, err := svc.ProcessScheduledJobs(ctx)

	require.NoError(t, err)
	assert.Equal(t, 1, result.Failed)
}

// =============================================================================
// ProcessScheduledJobs — Unknown Action Type with History (covers default branch)
// =============================================================================

func TestProcessScheduledJobs_UnknownActionType_WithHistory(t *testing.T) {
	svc, jobRepo, instanceRepo, groupRepo, _, _, notifier, historyRepo, historyNotifier := newFullTestService(t)
	ctx := context.Background()

	pendingJobs := []models.StandaloneJob{
		{
			ID: "job-1", UserID: "user-1", ActionType: "unknown_action",
			RetryCount: 3, MaxRetries: 3, // Exhausted
			Payload: models.JobPayload{Groups: []string{"db-1"}},
		},
	}

	resolvedGroups := []models.Group{{ID: "db-1", WAGroupID: "g1@g.us", IsAdmin: true}}

	jobRepo.On("ClaimPendingDueJobs", ctx, 50).Return(pendingJobs, nil)
	instanceRepo.On("GetUserInstances", ctx, "user-1").Return([]models.Instance{{InstanceID: "inst-1", Status: "connected"}}, nil)
	groupRepo.On("GetGroupsByIDs", ctx, "user-1", []string{"db-1"}).Return(resolvedGroups, nil)
	jobRepo.On("UpdateStatus", mock.Anything, "job-1", models.StandaloneJobStatusFailed, mock.AnythingOfType("*string")).Return(nil)

	notifier.On("PublishJobStatus", "user-1", "job-1", mock.AnythingOfType("string"), mock.Anything).Return()

	// History for unknown action type uses default branch
	historyRepo.On("Create", ctx, "user-1", "unknown_action", mock.AnythingOfType("string"), mock.Anything).
		Return(&models.ActionHistory{ID: "hist-1"}, nil)
	historyRepo.On("Update", mock.Anything, "hist-1", mock.Anything).Return(nil)
	historyRepo.On("UpdateCounts", mock.Anything, "hist-1", mock.AnythingOfType("int32"), mock.AnythingOfType("int32")).Return(nil)
	historyRepo.On("UpdateStatus", mock.Anything, "hist-1", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(nil)
	historyNotifier.On("PublishHistoryChange", "user-1", "hist-1", mock.AnythingOfType("string")).Return()

	result, err := svc.ProcessScheduledJobs(ctx)

	require.NoError(t, err)
	assert.Equal(t, 1, result.Failed)
}

// =============================================================================
// Legacy Service — executeAutomation Failed with UpdateStatus Error
// =============================================================================

func TestLegacyService_ProcessScheduledJobs_ExecuteFailed_UpdateStatusError(t *testing.T) {
	svc, scheduledJobRepo, automationRepo, instanceRepo := newLegacyTestService(t)
	ctx := context.Background()

	pastTime := time.Now().Add(-time.Hour).Format(time.RFC3339)
	scheduledJobs := []models.ScheduledJob{
		{ID: "job-1", AutomationID: "auto-1", ScheduledFor: pastTime},
	}

	automation := &models.Automation{
		ID:       "auto-1",
		UserID:   "user-1",
		Status:   models.AutomationStatusActive,
		GroupIDs: []string{}, // No groups = error
	}

	scheduledJobRepo.On("GetPendingJobs", ctx).Return(scheduledJobs, nil)
	scheduledJobRepo.On("UpdateJobStatus", ctx, "job-1", "processing", (*string)(nil)).Return(nil)
	automationRepo.On("GetAutomationByID", ctx, "auto-1").Return(automation, nil)
	instanceRepo.On("GetUserInstances", ctx, "user-1").Return([]models.Instance{
		{InstanceID: "inst-1", Status: "connected"},
	}, nil)
	// executeAutomation fails, then UpdateJobStatus also fails
	errMsg := "no target groups configured"
	scheduledJobRepo.On("UpdateJobStatus", ctx, "job-1", "failed", &errMsg).Return(errors.New("db error"))

	result, err := svc.ProcessScheduledJobs(ctx)

	require.NoError(t, err)
	assert.Equal(t, 1, result.Failed)
}

func TestLegacyService_ProcessScheduledJobs_SendGroups(t *testing.T) {
	svc, scheduledJobRepo, automationRepo, instanceRepo := newLegacyTestService(t)
	ctx := context.Background()

	pastTime := time.Now().Add(-time.Hour).Format(time.RFC3339)
	scheduledJobs := []models.ScheduledJob{
		{ID: "job-1", AutomationID: "auto-1", ScheduledFor: pastTime},
	}

	// SendGroups takes priority over GroupIDs
	automation := &models.Automation{
		ID:         "auto-1",
		UserID:     "user-1",
		Status:     models.AutomationStatusActive,
		GroupIDs:   []string{"group-old@g.us"},
		SendGroups: []string{"group-new@g.us"},
	}

	scheduledJobRepo.On("GetPendingJobs", ctx).Return(scheduledJobs, nil)
	scheduledJobRepo.On("UpdateJobStatus", ctx, "job-1", "processing", (*string)(nil)).Return(nil)
	automationRepo.On("GetAutomationByID", ctx, "auto-1").Return(automation, nil)
	instanceRepo.On("GetUserInstances", ctx, "user-1").Return([]models.Instance{
		{InstanceID: "inst-1", Status: "connected"},
	}, nil)
	scheduledJobRepo.On("UpdateJobStatus", ctx, "job-1", "completed", (*string)(nil)).Return(nil)
	automationRepo.On("UpdateAutomation", ctx, "auto-1", mock.Anything).Return(nil)

	result, err := svc.ProcessScheduledJobs(ctx)

	require.NoError(t, err)
	assert.Equal(t, 1, result.Processed)
}
