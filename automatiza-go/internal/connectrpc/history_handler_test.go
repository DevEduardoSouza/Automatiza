// Package connectrpc provides ConnectRPC handlers for the Automatiza platform.
package connectrpc

import (
	"context"
	"errors"
	"testing"
	"time"

	"automatiza-go/internal/auth"
	historyv1 "automatiza-go/internal/gen/history/v1"
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Mock Implementations
// =============================================================================

// mockActionHistoryRepo implements repository.ActionHistoryRepository for testing.
type mockActionHistoryRepo struct {
	mock.Mock
}

func (m *mockActionHistoryRepo) Create(ctx context.Context, userID, actionType, description string, details map[string]interface{}) (*models.ActionHistory, error) {
	args := m.Called(ctx, userID, actionType, description, details)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ActionHistory), args.Error(1)
}

func (m *mockActionHistoryRepo) Update(ctx context.Context, historyID string, updates map[string]interface{}) error {
	args := m.Called(ctx, historyID, updates)
	return args.Error(0)
}

func (m *mockActionHistoryRepo) UpdateStatus(ctx context.Context, id, status, errorMessage string) error {
	args := m.Called(ctx, id, status, errorMessage)
	return args.Error(0)
}

func (m *mockActionHistoryRepo) GetByID(ctx context.Context, historyID string) (*models.ActionHistory, error) {
	args := m.Called(ctx, historyID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ActionHistory), args.Error(1)
}

func (m *mockActionHistoryRepo) GetUserHistory(ctx context.Context, userID string, limit, offset int) ([]models.ActionHistory, error) {
	args := m.Called(ctx, userID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.ActionHistory), args.Error(1)
}

func (m *mockActionHistoryRepo) GetByUserID(ctx context.Context, userID string, limit, offset int) ([]models.ActionHistory, error) {
	args := m.Called(ctx, userID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.ActionHistory), args.Error(1)
}

func (m *mockActionHistoryRepo) GetByActionType(ctx context.Context, userID, actionType string, limit int) ([]models.ActionHistory, error) {
	args := m.Called(ctx, userID, actionType, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.ActionHistory), args.Error(1)
}

func (m *mockActionHistoryRepo) GetUserHistoryFiltered(ctx context.Context, req models.HistoryListRequest, userID string) ([]models.ActionHistory, error) {
	args := m.Called(ctx, req, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.ActionHistory), args.Error(1)
}

func (m *mockActionHistoryRepo) CountUserHistoryFiltered(ctx context.Context, req models.HistoryListRequest, userID string) (models.HistoryStatsResult, error) {
	args := m.Called(ctx, req, userID)
	return args.Get(0).(models.HistoryStatsResult), args.Error(1)
}

func (m *mockActionHistoryRepo) MarkCompleted(ctx context.Context, historyID string) error {
	args := m.Called(ctx, historyID)
	return args.Error(0)
}

func (m *mockActionHistoryRepo) MarkFailed(ctx context.Context, historyID, errorMessage string) error {
	args := m.Called(ctx, historyID, errorMessage)
	return args.Error(0)
}

func (m *mockActionHistoryRepo) MarkRunning(ctx context.Context, historyID string) error {
	args := m.Called(ctx, historyID)
	return args.Error(0)
}

func (m *mockActionHistoryRepo) IncrementSuccessCount(ctx context.Context, historyID string) error {
	args := m.Called(ctx, historyID)
	return args.Error(0)
}

func (m *mockActionHistoryRepo) IncrementErrorCount(ctx context.Context, historyID string, errorMsg string) error {
	args := m.Called(ctx, historyID, errorMsg)
	return args.Error(0)
}

func (m *mockActionHistoryRepo) UpdateCounts(ctx context.Context, historyID string, successCount, errorCount int32) error {
	args := m.Called(ctx, historyID, successCount, errorCount)
	return args.Error(0)
}

func (m *mockActionHistoryRepo) DeleteUserHistory(ctx context.Context, userID string, beforeDate *time.Time, actionTypes []string) (int64, error) {
	args := m.Called(ctx, userID, beforeDate, actionTypes)
	return args.Get(0).(int64), args.Error(1)
}

// mockDispatchLogRepo implements repository.DispatchLogRepository for testing.
type mockDispatchLogRepo struct {
	mock.Mock
}

func (m *mockDispatchLogRepo) Create(ctx context.Context, log *models.DispatchLog) (*models.DispatchLog, error) {
	args := m.Called(ctx, log)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.DispatchLog), args.Error(1)
}

func (m *mockDispatchLogRepo) Update(ctx context.Context, logID int, updates map[string]interface{}) error {
	args := m.Called(ctx, logID, updates)
	return args.Error(0)
}

func (m *mockDispatchLogRepo) GetRecentByCanonicalKey(ctx context.Context, userID, groupID, canonicalKey string, hours int) (*models.DispatchLog, error) {
	args := m.Called(ctx, userID, groupID, canonicalKey, hours)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.DispatchLog), args.Error(1)
}

func (m *mockDispatchLogRepo) GetUserLogs(ctx context.Context, userID string, limit, offset int) ([]models.DispatchLog, error) {
	args := m.Called(ctx, userID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.DispatchLog), args.Error(1)
}

func (m *mockDispatchLogRepo) DeleteRecentByAutomation(ctx context.Context, automationID string, sinceDays int) (int64, error) {
	args := m.Called(ctx, automationID, sinceDays)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockDispatchLogRepo) GetByAutomationIDWithCount(ctx context.Context, automationID, userID string, limit, offset int) ([]repository.DispatchLogWithGroup, int, error) {
	args := m.Called(ctx, automationID, userID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]repository.DispatchLogWithGroup), args.Int(1), args.Error(2)
}

func (m *mockDispatchLogRepo) GetUserLogsWithFilters(ctx context.Context, userID string, filters *models.DispatchLogFilters, limit, offset int) ([]repository.DispatchLogWithGroup, int64, error) {
	args := m.Called(ctx, userID, filters, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]repository.DispatchLogWithGroup), args.Get(1).(int64), args.Error(2)
}

func (m *mockDispatchLogRepo) ExistsByCanonicalKey(ctx context.Context, userID, groupID, canonicalKey string, hours int) (bool, error) {
	args := m.Called(ctx, userID, groupID, canonicalKey, hours)
	return args.Bool(0), args.Error(1)
}

func (m *mockDispatchLogRepo) CreateIfNotExists(ctx context.Context, log *models.DispatchLog, dedupWindowHours int) (*models.DispatchLog, bool, error) {
	args := m.Called(ctx, log, dedupWindowHours)
	if args.Get(0) == nil {
		return nil, args.Bool(1), args.Error(2)
	}
	return args.Get(0).(*models.DispatchLog), args.Bool(1), args.Error(2)
}

// mockGroupRepoForHistory implements repository.GroupRepository for testing.
// Named differently to avoid collision with mockGroupRepo in groups_handler_test.go.
type mockGroupRepoForHistory struct {
	mock.Mock
}

func (m *mockGroupRepoForHistory) GetUserGroups(ctx context.Context, userID string, adminOnly bool) ([]models.Group, error) {
	args := m.Called(ctx, userID, adminOnly)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Group), args.Error(1)
}

func (m *mockGroupRepoForHistory) GetGroupByID(ctx context.Context, groupID string) (*models.Group, error) {
	args := m.Called(ctx, groupID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Group), args.Error(1)
}

func (m *mockGroupRepoForHistory) GetGroupByWAGroupID(ctx context.Context, userID, waGroupID string) (*models.Group, error) {
	args := m.Called(ctx, userID, waGroupID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Group), args.Error(1)
}

func (m *mockGroupRepoForHistory) GetGroupsByIDs(ctx context.Context, userID string, groupIDs []string) ([]models.Group, error) {
	args := m.Called(ctx, userID, groupIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Group), args.Error(1)
}

func (m *mockGroupRepoForHistory) GetFavoriteGroups(ctx context.Context, userID string) ([]models.Group, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Group), args.Error(1)
}

func (m *mockGroupRepoForHistory) CreateGroup(ctx context.Context, userID string, group *models.Group) (*models.Group, error) {
	args := m.Called(ctx, userID, group)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Group), args.Error(1)
}

func (m *mockGroupRepoForHistory) SyncGroups(ctx context.Context, userID string, instanceID int, groups []models.Group, options repository.SyncOptions) error {
	args := m.Called(ctx, userID, instanceID, groups, options)
	return args.Error(0)
}

func (m *mockGroupRepoForHistory) SyncGroupsWithStats(ctx context.Context, userID string, instanceID int, groups []models.Group, options repository.SyncOptions) (*repository.SyncResult, error) {
	args := m.Called(ctx, userID, instanceID, groups, options)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.SyncResult), args.Error(1)
}

func (m *mockGroupRepoForHistory) IsUserAdminInGroup(ctx context.Context, userID, waGroupID string) (bool, error) {
	args := m.Called(ctx, userID, waGroupID)
	return args.Bool(0), args.Error(1)
}

func (m *mockGroupRepoForHistory) UpdateGroup(ctx context.Context, groupID string, updates map[string]interface{}) error {
	args := m.Called(ctx, groupID, updates)
	return args.Error(0)
}

func (m *mockGroupRepoForHistory) ToggleFavorite(ctx context.Context, groupID, userID string, favorite bool) error {
	args := m.Called(ctx, groupID, userID, favorite)
	return args.Error(0)
}

func (m *mockGroupRepoForHistory) DeleteGroup(ctx context.Context, groupID, userID string) error {
	args := m.Called(ctx, groupID, userID)
	return args.Error(0)
}

func (m *mockGroupRepoForHistory) DeleteGroupsByInstance(ctx context.Context, userID string, instanceID int) error {
	args := m.Called(ctx, userID, instanceID)
	return args.Error(0)
}

func (m *mockGroupRepoForHistory) BulkUpdateGroups(ctx context.Context, userID string, groupIDs []string, updates map[string]interface{}) (int64, error) {
	args := m.Called(ctx, userID, groupIDs, updates)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockGroupRepoForHistory) GetUserGroupsPaginated(ctx context.Context, userID string, page, pageSize int, adminOnly bool, search string) (*repository.GroupPaginatedResult, error) {
	args := m.Called(ctx, userID, page, pageSize, adminOnly, search)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.GroupPaginatedResult), args.Error(1)
}

func (m *mockGroupRepoForHistory) CountAll(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

// =============================================================================
// Helper Functions
// =============================================================================

// createTestUserContextForHistory creates a context with a user for testing.
func createTestUserContextForHistory(userID string) context.Context {
	user := &auth.User{
		ID:    userID,
		Email: "test@example.com",
	}
	return context.WithValue(context.Background(), UserContextKey, user)
}

// =============================================================================
// ListHistory Tests
// =============================================================================

func TestListHistory_Success(t *testing.T) {
	t.Parallel()

	historyRepo := new(mockActionHistoryRepo)
	dispatchLogRepo := new(mockDispatchLogRepo)
	handler := NewHistoryHandler(historyRepo, dispatchLogRepo)

	userID := "user-123"
	ctx := createTestUserContextForHistory(userID)

	expectedHistory := []models.ActionHistory{
		{
			ID:          "h-1",
			UserID:      userID,
			ActionType:  "send_message",
			Description: "Sent message to group",
			Status:      "completed",
			CreatedAt:   "2026-01-01T00:00:00Z",
		},
		{
			ID:          "h-2",
			UserID:      userID,
			ActionType:  "sync_groups",
			Description: "Synced groups",
			Status:      "running",
			CreatedAt:   "2026-01-02T00:00:00Z",
		},
	}

	historyRepo.On("GetUserHistoryFiltered", mock.Anything, mock.MatchedBy(func(req models.HistoryListRequest) bool {
		return req.Limit == 50 && req.Offset == 0
	}), userID).Return(expectedHistory, nil)

	historyRepo.On("CountUserHistoryFiltered", mock.Anything, mock.Anything, userID).Return(models.HistoryStatsResult{
		Total:     2,
		Completed: 1,
		Running:   1,
		Failed:    0,
	}, nil)

	req := connect.NewRequest(&historyv1.ListHistoryRequest{})
	resp, err := handler.ListHistory(ctx, req)

	require.NoError(t, err)
	assert.Len(t, resp.Msg.Events, 2)
	assert.Equal(t, "h-1", resp.Msg.Events[0].Id)
	assert.Equal(t, historyv1.EventType_EVENT_TYPE_MESSAGE_SENT, resp.Msg.Events[0].EventType)
	assert.Equal(t, "h-2", resp.Msg.Events[1].Id)
	assert.Equal(t, historyv1.EventType_EVENT_TYPE_GROUP_SYNC, resp.Msg.Events[1].EventType)
	assert.Equal(t, int32(2), resp.Msg.TotalCount)
	assert.Equal(t, int32(1), resp.Msg.Stats.Completed)
	assert.Equal(t, int32(1), resp.Msg.Stats.Running)
	assert.Equal(t, int32(0), resp.Msg.Stats.Failed)

	historyRepo.AssertExpectations(t)
}

func TestListHistory_Unauthenticated(t *testing.T) {
	t.Parallel()

	historyRepo := new(mockActionHistoryRepo)
	dispatchLogRepo := new(mockDispatchLogRepo)
	handler := NewHistoryHandler(historyRepo, dispatchLogRepo)

	ctx := context.Background()

	req := connect.NewRequest(&historyv1.ListHistoryRequest{})
	_, err := handler.ListHistory(ctx, req)

	require.Error(t, err)
	connectErr, ok := err.(*connect.Error)
	require.True(t, ok)
	assert.Equal(t, connect.CodeUnauthenticated, connectErr.Code())
}

func TestListHistory_PageBasedPagination(t *testing.T) {
	t.Parallel()

	historyRepo := new(mockActionHistoryRepo)
	dispatchLogRepo := new(mockDispatchLogRepo)
	handler := NewHistoryHandler(historyRepo, dispatchLogRepo)

	userID := "user-123"
	ctx := createTestUserContextForHistory(userID)

	page := int32(2)
	pageSize := int32(10)

	historyRepo.On("GetUserHistoryFiltered", mock.Anything, mock.MatchedBy(func(req models.HistoryListRequest) bool {
		return req.Limit == 10 && req.Offset == 10
	}), userID).Return([]models.ActionHistory{
		{ID: "h-11", UserID: userID, ActionType: "send_message", Description: "Message", Status: "completed", CreatedAt: "2026-01-01T00:00:00Z"},
	}, nil)

	historyRepo.On("CountUserHistoryFiltered", mock.Anything, mock.Anything, userID).Return(models.HistoryStatsResult{
		Total: 15,
	}, nil)

	req := connect.NewRequest(&historyv1.ListHistoryRequest{
		Page:     &page,
		PageSize: &pageSize,
	})
	resp, err := handler.ListHistory(ctx, req)

	require.NoError(t, err)
	assert.Equal(t, int32(2), resp.Msg.Page)
	assert.Equal(t, int32(10), resp.Msg.PageSize)
	assert.Equal(t, int32(2), resp.Msg.TotalPages)
	assert.Equal(t, int32(15), resp.Msg.TotalCount)

	historyRepo.AssertExpectations(t)
}

func TestListHistory_LegacyPagination(t *testing.T) {
	t.Parallel()

	historyRepo := new(mockActionHistoryRepo)
	dispatchLogRepo := new(mockDispatchLogRepo)
	handler := NewHistoryHandler(historyRepo, dispatchLogRepo)

	userID := "user-123"
	ctx := createTestUserContextForHistory(userID)

	limit := int32(25)
	offset := int32(50)

	historyRepo.On("GetUserHistoryFiltered", mock.Anything, mock.MatchedBy(func(req models.HistoryListRequest) bool {
		return req.Limit == 25 && req.Offset == 50
	}), userID).Return([]models.ActionHistory{}, nil)

	historyRepo.On("CountUserHistoryFiltered", mock.Anything, mock.Anything, userID).Return(models.HistoryStatsResult{
		Total: 60,
	}, nil)

	req := connect.NewRequest(&historyv1.ListHistoryRequest{
		Limit:  &limit,
		Offset: &offset,
	})
	resp, err := handler.ListHistory(ctx, req)

	require.NoError(t, err)
	// page = (50/25) + 1 = 3
	assert.Equal(t, int32(3), resp.Msg.Page)
	assert.Equal(t, int32(25), resp.Msg.PageSize)

	historyRepo.AssertExpectations(t)
}

func TestListHistory_MaxPageSize100(t *testing.T) {
	t.Parallel()

	historyRepo := new(mockActionHistoryRepo)
	dispatchLogRepo := new(mockDispatchLogRepo)
	handler := NewHistoryHandler(historyRepo, dispatchLogRepo)

	userID := "user-123"
	ctx := createTestUserContextForHistory(userID)

	page := int32(1)
	pageSize := int32(500) // exceeds max

	historyRepo.On("GetUserHistoryFiltered", mock.Anything, mock.MatchedBy(func(req models.HistoryListRequest) bool {
		return req.Limit == 100 // capped to 100
	}), userID).Return([]models.ActionHistory{}, nil)

	historyRepo.On("CountUserHistoryFiltered", mock.Anything, mock.Anything, userID).Return(models.HistoryStatsResult{
		Total: 0,
	}, nil)

	req := connect.NewRequest(&historyv1.ListHistoryRequest{
		Page:     &page,
		PageSize: &pageSize,
	})
	resp, err := handler.ListHistory(ctx, req)

	require.NoError(t, err)
	assert.Equal(t, int32(100), resp.Msg.PageSize)

	historyRepo.AssertExpectations(t)
}

func TestListHistory_WithEventTypeFilter(t *testing.T) {
	t.Parallel()

	historyRepo := new(mockActionHistoryRepo)
	dispatchLogRepo := new(mockDispatchLogRepo)
	handler := NewHistoryHandler(historyRepo, dispatchLogRepo)

	userID := "user-123"
	ctx := createTestUserContextForHistory(userID)

	eventType := historyv1.EventType_EVENT_TYPE_AUTOMATION_RUN

	historyRepo.On("GetUserHistoryFiltered", mock.Anything, mock.MatchedBy(func(req models.HistoryListRequest) bool {
		return req.ActionType != nil && *req.ActionType == "run_automation"
	}), userID).Return([]models.ActionHistory{
		{ID: "h-1", UserID: userID, ActionType: "run_automation", Description: "Automation run", Status: "completed", CreatedAt: "2026-01-01T00:00:00Z"},
	}, nil)

	historyRepo.On("CountUserHistoryFiltered", mock.Anything, mock.MatchedBy(func(req models.HistoryListRequest) bool {
		return req.ActionType != nil && *req.ActionType == "run_automation"
	}), userID).Return(models.HistoryStatsResult{Total: 1, Completed: 1}, nil)

	req := connect.NewRequest(&historyv1.ListHistoryRequest{
		EventType: &eventType,
	})
	resp, err := handler.ListHistory(ctx, req)

	require.NoError(t, err)
	assert.Len(t, resp.Msg.Events, 1)
	assert.Equal(t, historyv1.EventType_EVENT_TYPE_AUTOMATION_RUN, resp.Msg.Events[0].EventType)

	historyRepo.AssertExpectations(t)
}

func TestListHistory_WithStatusFilter(t *testing.T) {
	t.Parallel()

	historyRepo := new(mockActionHistoryRepo)
	dispatchLogRepo := new(mockDispatchLogRepo)
	handler := NewHistoryHandler(historyRepo, dispatchLogRepo)

	userID := "user-123"
	ctx := createTestUserContextForHistory(userID)

	status := "failed"

	historyRepo.On("GetUserHistoryFiltered", mock.Anything, mock.MatchedBy(func(req models.HistoryListRequest) bool {
		return req.Status != nil && *req.Status == "failed"
	}), userID).Return([]models.ActionHistory{
		{ID: "h-fail", UserID: userID, ActionType: "error", Description: "Failed action", Status: "failed", CreatedAt: "2026-01-01T00:00:00Z"},
	}, nil)

	historyRepo.On("CountUserHistoryFiltered", mock.Anything, mock.Anything, userID).Return(models.HistoryStatsResult{
		Total: 1, Failed: 1,
	}, nil)

	req := connect.NewRequest(&historyv1.ListHistoryRequest{
		Status: &status,
	})
	resp, err := handler.ListHistory(ctx, req)

	require.NoError(t, err)
	assert.Len(t, resp.Msg.Events, 1)
	assert.Equal(t, int32(1), resp.Msg.Stats.Failed)

	historyRepo.AssertExpectations(t)
}

func TestListHistory_EmptyResult(t *testing.T) {
	t.Parallel()

	historyRepo := new(mockActionHistoryRepo)
	dispatchLogRepo := new(mockDispatchLogRepo)
	handler := NewHistoryHandler(historyRepo, dispatchLogRepo)

	userID := "user-123"
	ctx := createTestUserContextForHistory(userID)

	historyRepo.On("GetUserHistoryFiltered", mock.Anything, mock.Anything, userID).Return([]models.ActionHistory{}, nil)
	historyRepo.On("CountUserHistoryFiltered", mock.Anything, mock.Anything, userID).Return(models.HistoryStatsResult{
		Total: 0,
	}, nil)

	req := connect.NewRequest(&historyv1.ListHistoryRequest{})
	resp, err := handler.ListHistory(ctx, req)

	require.NoError(t, err)
	assert.Empty(t, resp.Msg.Events)
	assert.False(t, resp.Msg.HasMore)
	assert.Equal(t, int32(0), resp.Msg.TotalCount)
	assert.Equal(t, int32(1), resp.Msg.TotalPages) // minimum 1 page

	historyRepo.AssertExpectations(t)
}

func TestListHistory_RepoError(t *testing.T) {
	t.Parallel()

	historyRepo := new(mockActionHistoryRepo)
	dispatchLogRepo := new(mockDispatchLogRepo)
	handler := NewHistoryHandler(historyRepo, dispatchLogRepo)

	userID := "user-123"
	ctx := createTestUserContextForHistory(userID)

	historyRepo.On("GetUserHistoryFiltered", mock.Anything, mock.Anything, userID).Return(nil, errors.New("database error"))

	req := connect.NewRequest(&historyv1.ListHistoryRequest{})
	_, err := handler.ListHistory(ctx, req)

	require.Error(t, err)
	connectErr, ok := err.(*connect.Error)
	require.True(t, ok)
	assert.Equal(t, connect.CodeInternal, connectErr.Code())

	historyRepo.AssertExpectations(t)
}

func TestListHistory_StatsCountError_Fallback(t *testing.T) {
	t.Parallel()

	historyRepo := new(mockActionHistoryRepo)
	dispatchLogRepo := new(mockDispatchLogRepo)
	handler := NewHistoryHandler(historyRepo, dispatchLogRepo)

	userID := "user-123"
	ctx := createTestUserContextForHistory(userID)

	returnedHistory := []models.ActionHistory{
		{ID: "h-1", UserID: userID, ActionType: "send_message", Description: "Msg", Status: "completed", CreatedAt: "2026-01-01T00:00:00Z"},
		{ID: "h-2", UserID: userID, ActionType: "sync_groups", Description: "Sync", Status: "running", CreatedAt: "2026-01-02T00:00:00Z"},
		{ID: "h-3", UserID: userID, ActionType: "error", Description: "Err", Status: "failed", CreatedAt: "2026-01-03T00:00:00Z"},
	}

	historyRepo.On("GetUserHistoryFiltered", mock.Anything, mock.Anything, userID).Return(returnedHistory, nil)
	// CountUserHistoryFiltered fails - handler should fallback
	historyRepo.On("CountUserHistoryFiltered", mock.Anything, mock.Anything, userID).Return(models.HistoryStatsResult{}, errors.New("count query failed"))

	req := connect.NewRequest(&historyv1.ListHistoryRequest{})
	resp, err := handler.ListHistory(ctx, req)

	require.NoError(t, err)
	assert.Len(t, resp.Msg.Events, 3)
	// Fallback: totalCount = len(history) = 3
	assert.Equal(t, int32(3), resp.Msg.TotalCount)

	historyRepo.AssertExpectations(t)
}

func TestListHistory_TotalPagesCalculation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		total          int32
		pageSize       int32
		expectedPages  int32
	}{
		{"exact division", 20, 10, 2},
		{"remainder adds page", 21, 10, 3},
		{"single item", 1, 10, 1},
		{"zero items", 0, 10, 1},
		{"full single page", 10, 10, 1},
		{"large dataset", 250, 100, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			historyRepo := new(mockActionHistoryRepo)
			dispatchLogRepo := new(mockDispatchLogRepo)
			handler := NewHistoryHandler(historyRepo, dispatchLogRepo)

			userID := "user-calc"
			ctx := createTestUserContextForHistory(userID)

			page := int32(1)

			historyRepo.On("GetUserHistoryFiltered", mock.Anything, mock.Anything, userID).Return([]models.ActionHistory{}, nil)
			historyRepo.On("CountUserHistoryFiltered", mock.Anything, mock.Anything, userID).Return(models.HistoryStatsResult{
				Total: tt.total,
			}, nil)

			req := connect.NewRequest(&historyv1.ListHistoryRequest{
				Page:     &page,
				PageSize: &tt.pageSize,
			})
			resp, err := handler.ListHistory(ctx, req)

			require.NoError(t, err)
			assert.Equal(t, tt.expectedPages, resp.Msg.TotalPages)
		})
	}
}

func TestListHistory_HasMoreCalculation(t *testing.T) {
	t.Parallel()

	historyRepo := new(mockActionHistoryRepo)
	dispatchLogRepo := new(mockDispatchLogRepo)
	handler := NewHistoryHandler(historyRepo, dispatchLogRepo)

	userID := "user-hasmore"
	ctx := createTestUserContextForHistory(userID)

	page := int32(1)
	pageSize := int32(2)

	// Return exactly pageSize items to trigger hasMore=true
	historyRepo.On("GetUserHistoryFiltered", mock.Anything, mock.Anything, userID).Return([]models.ActionHistory{
		{ID: "h-1", UserID: userID, ActionType: "send_message", Description: "Msg 1", Status: "completed", CreatedAt: "2026-01-01T00:00:00Z"},
		{ID: "h-2", UserID: userID, ActionType: "send_message", Description: "Msg 2", Status: "completed", CreatedAt: "2026-01-02T00:00:00Z"},
	}, nil)
	historyRepo.On("CountUserHistoryFiltered", mock.Anything, mock.Anything, userID).Return(models.HistoryStatsResult{
		Total: 5,
	}, nil)

	req := connect.NewRequest(&historyv1.ListHistoryRequest{
		Page:     &page,
		PageSize: &pageSize,
	})
	resp, err := handler.ListHistory(ctx, req)

	require.NoError(t, err)
	// hasMore = len(history)==pageSize && page < totalPages => 2==2 && 1 < 3 => true
	assert.True(t, resp.Msg.HasMore)

	historyRepo.AssertExpectations(t)
}

// =============================================================================
// ClearHistory Tests
// =============================================================================

func TestClearHistory_Success(t *testing.T) {
	t.Parallel()

	historyRepo := new(mockActionHistoryRepo)
	dispatchLogRepo := new(mockDispatchLogRepo)
	handler := NewHistoryHandler(historyRepo, dispatchLogRepo)

	userID := "user-123"
	ctx := createTestUserContextForHistory(userID)

	historyRepo.On("DeleteUserHistory", mock.Anything, userID, (*time.Time)(nil), []string(nil)).Return(int64(5), nil)

	req := connect.NewRequest(&historyv1.ClearHistoryRequest{})
	resp, err := handler.ClearHistory(ctx, req)

	require.NoError(t, err)
	assert.Equal(t, int32(5), resp.Msg.DeletedCount)

	historyRepo.AssertExpectations(t)
}

func TestClearHistory_Unauthenticated(t *testing.T) {
	t.Parallel()

	historyRepo := new(mockActionHistoryRepo)
	dispatchLogRepo := new(mockDispatchLogRepo)
	handler := NewHistoryHandler(historyRepo, dispatchLogRepo)

	ctx := context.Background()

	req := connect.NewRequest(&historyv1.ClearHistoryRequest{})
	_, err := handler.ClearHistory(ctx, req)

	require.Error(t, err)
	connectErr, ok := err.(*connect.Error)
	require.True(t, ok)
	assert.Equal(t, connect.CodeUnauthenticated, connectErr.Code())
}

func TestClearHistory_WithBeforeDate(t *testing.T) {
	t.Parallel()

	historyRepo := new(mockActionHistoryRepo)
	dispatchLogRepo := new(mockDispatchLogRepo)
	handler := NewHistoryHandler(historyRepo, dispatchLogRepo)

	userID := "user-123"
	ctx := createTestUserContextForHistory(userID)

	beforeDateStr := "2026-01-15T00:00:00Z"
	parsedTime, _ := time.Parse(time.RFC3339, beforeDateStr)

	historyRepo.On("DeleteUserHistory", mock.Anything, userID, mock.MatchedBy(func(t *time.Time) bool {
		return t != nil && t.Equal(parsedTime)
	}), []string(nil)).Return(int64(3), nil)

	req := connect.NewRequest(&historyv1.ClearHistoryRequest{
		BeforeDate: &beforeDateStr,
	})
	resp, err := handler.ClearHistory(ctx, req)

	require.NoError(t, err)
	assert.Equal(t, int32(3), resp.Msg.DeletedCount)

	historyRepo.AssertExpectations(t)
}

func TestClearHistory_WithEventTypes(t *testing.T) {
	t.Parallel()

	historyRepo := new(mockActionHistoryRepo)
	dispatchLogRepo := new(mockDispatchLogRepo)
	handler := NewHistoryHandler(historyRepo, dispatchLogRepo)

	userID := "user-123"
	ctx := createTestUserContextForHistory(userID)

	historyRepo.On("DeleteUserHistory", mock.Anything, userID, (*time.Time)(nil), mock.MatchedBy(func(types []string) bool {
		return len(types) == 2
	})).Return(int64(7), nil)

	req := connect.NewRequest(&historyv1.ClearHistoryRequest{
		EventTypes: []historyv1.EventType{
			historyv1.EventType_EVENT_TYPE_MESSAGE_SENT,
			historyv1.EventType_EVENT_TYPE_ERROR,
		},
	})
	resp, err := handler.ClearHistory(ctx, req)

	require.NoError(t, err)
	assert.Equal(t, int32(7), resp.Msg.DeletedCount)

	historyRepo.AssertExpectations(t)
}

func TestClearHistory_InvalidDate_IgnoresFilter(t *testing.T) {
	t.Parallel()

	historyRepo := new(mockActionHistoryRepo)
	dispatchLogRepo := new(mockDispatchLogRepo)
	handler := NewHistoryHandler(historyRepo, dispatchLogRepo)

	userID := "user-123"
	ctx := createTestUserContextForHistory(userID)

	invalidDate := "not-a-date"

	// When date parsing fails, beforeDate is nil
	historyRepo.On("DeleteUserHistory", mock.Anything, userID, (*time.Time)(nil), []string(nil)).Return(int64(10), nil)

	req := connect.NewRequest(&historyv1.ClearHistoryRequest{
		BeforeDate: &invalidDate,
	})
	resp, err := handler.ClearHistory(ctx, req)

	require.NoError(t, err)
	assert.Equal(t, int32(10), resp.Msg.DeletedCount)

	historyRepo.AssertExpectations(t)
}

func TestClearHistory_RepoError(t *testing.T) {
	t.Parallel()

	historyRepo := new(mockActionHistoryRepo)
	dispatchLogRepo := new(mockDispatchLogRepo)
	handler := NewHistoryHandler(historyRepo, dispatchLogRepo)

	userID := "user-123"
	ctx := createTestUserContextForHistory(userID)

	historyRepo.On("DeleteUserHistory", mock.Anything, userID, (*time.Time)(nil), []string(nil)).Return(int64(0), errors.New("database error"))

	req := connect.NewRequest(&historyv1.ClearHistoryRequest{})
	_, err := handler.ClearHistory(ctx, req)

	require.Error(t, err)
	connectErr, ok := err.(*connect.Error)
	require.True(t, ok)
	assert.Equal(t, connect.CodeInternal, connectErr.Code())

	historyRepo.AssertExpectations(t)
}

// =============================================================================
// ListDispatchLogs Tests
// =============================================================================

func TestListDispatchLogs_Success(t *testing.T) {
	t.Parallel()

	historyRepo := new(mockActionHistoryRepo)
	dispatchLogRepo := new(mockDispatchLogRepo)
	handler := NewHistoryHandler(historyRepo, dispatchLogRepo)

	userID := "user-123"
	ctx := createTestUserContextForHistory(userID)

	automationID := "auto-1"
	automationName := "Test Automation"
	store := "shopee"
	groupID := "group@g.us"

	expectedLogs := []models.DispatchLog{
		{
			ID:             1,
			UserID:         userID,
			AutomationID:   &automationID,
			AutomationName: &automationName,
			Store:          &store,
			GroupID:        &groupID,
			Status:         "sent",
			CreatedAt:      "2026-01-01T00:00:00Z",
		},
		{
			ID:        2,
			UserID:    userID,
			Status:    "failed",
			CreatedAt: "2026-01-02T00:00:00Z",
		},
	}

	dispatchLogRepo.On("GetUserLogs", mock.Anything, userID, 50, 0).Return(expectedLogs, nil)

	req := connect.NewRequest(&historyv1.ListDispatchLogsRequest{})
	resp, err := handler.ListDispatchLogs(ctx, req)

	require.NoError(t, err)
	assert.Len(t, resp.Msg.Logs, 2)
	assert.Equal(t, int32(1), resp.Msg.Logs[0].Id)
	assert.Equal(t, "auto-1", resp.Msg.Logs[0].GetAutomationId())
	assert.Equal(t, "Test Automation", resp.Msg.Logs[0].GetAutomationName())
	assert.Equal(t, "shopee", resp.Msg.Logs[0].GetStore())
	assert.Equal(t, historyv1.DispatchLogStatus_DISPATCH_LOG_STATUS_SENT, resp.Msg.Logs[0].Status)
	assert.Equal(t, int32(2), resp.Msg.Logs[1].Id)
	assert.Equal(t, historyv1.DispatchLogStatus_DISPATCH_LOG_STATUS_FAILED, resp.Msg.Logs[1].Status)

	dispatchLogRepo.AssertExpectations(t)
}

func TestListDispatchLogs_Unauthenticated(t *testing.T) {
	t.Parallel()

	historyRepo := new(mockActionHistoryRepo)
	dispatchLogRepo := new(mockDispatchLogRepo)
	handler := NewHistoryHandler(historyRepo, dispatchLogRepo)

	ctx := context.Background()

	req := connect.NewRequest(&historyv1.ListDispatchLogsRequest{})
	_, err := handler.ListDispatchLogs(ctx, req)

	require.Error(t, err)
	connectErr, ok := err.(*connect.Error)
	require.True(t, ok)
	assert.Equal(t, connect.CodeUnauthenticated, connectErr.Code())
}

func TestListDispatchLogs_WithGroupNames(t *testing.T) {
	t.Parallel()

	historyRepo := new(mockActionHistoryRepo)
	dispatchLogRepo := new(mockDispatchLogRepo)
	groupRepo := new(mockGroupRepoForHistory)
	handler := NewHistoryHandlerWithGroups(historyRepo, dispatchLogRepo, groupRepo)

	userID := "user-123"
	ctx := createTestUserContextForHistory(userID)

	groupID1 := "group-1@g.us"
	groupID2 := "group-2@g.us"

	expectedLogs := []models.DispatchLog{
		{ID: 1, UserID: userID, GroupID: &groupID1, Status: "sent", CreatedAt: "2026-01-01T00:00:00Z"},
		{ID: 2, UserID: userID, GroupID: &groupID2, Status: "sent", CreatedAt: "2026-01-02T00:00:00Z"},
		{ID: 3, UserID: userID, GroupID: &groupID1, Status: "failed", CreatedAt: "2026-01-03T00:00:00Z"},
	}

	dispatchLogRepo.On("GetUserLogs", mock.Anything, userID, 50, 0).Return(expectedLogs, nil)

	groupRepo.On("GetGroupsByIDs", mock.Anything, userID, mock.MatchedBy(func(ids []string) bool {
		return len(ids) == 2
	})).Return([]models.Group{
		{ID: groupID1, Name: "Group One"},
		{ID: groupID2, Name: "Group Two"},
	}, nil)

	req := connect.NewRequest(&historyv1.ListDispatchLogsRequest{})
	resp, err := handler.ListDispatchLogs(ctx, req)

	require.NoError(t, err)
	assert.Len(t, resp.Msg.Logs, 3)
	assert.Equal(t, "Group One", resp.Msg.Logs[0].GetGroupName())
	assert.Equal(t, "Group Two", resp.Msg.Logs[1].GetGroupName())
	assert.Equal(t, "Group One", resp.Msg.Logs[2].GetGroupName())

	dispatchLogRepo.AssertExpectations(t)
	groupRepo.AssertExpectations(t)
}

func TestListDispatchLogs_GroupRepoError_SilentFallback(t *testing.T) {
	t.Parallel()

	historyRepo := new(mockActionHistoryRepo)
	dispatchLogRepo := new(mockDispatchLogRepo)
	groupRepo := new(mockGroupRepoForHistory)
	handler := NewHistoryHandlerWithGroups(historyRepo, dispatchLogRepo, groupRepo)

	userID := "user-123"
	ctx := createTestUserContextForHistory(userID)

	groupID := "group-1@g.us"

	expectedLogs := []models.DispatchLog{
		{ID: 1, UserID: userID, GroupID: &groupID, Status: "sent", CreatedAt: "2026-01-01T00:00:00Z"},
	}

	dispatchLogRepo.On("GetUserLogs", mock.Anything, userID, 50, 0).Return(expectedLogs, nil)

	// Group repo fails but should be silently ignored
	groupRepo.On("GetGroupsByIDs", mock.Anything, userID, mock.Anything).Return(nil, errors.New("groups db error"))

	req := connect.NewRequest(&historyv1.ListDispatchLogsRequest{})
	resp, err := handler.ListDispatchLogs(ctx, req)

	// No error returned - group names are optional enrichment
	require.NoError(t, err)
	assert.Len(t, resp.Msg.Logs, 1)
	// group_name should not be set since the lookup failed
	assert.Nil(t, resp.Msg.Logs[0].GroupName)

	dispatchLogRepo.AssertExpectations(t)
	groupRepo.AssertExpectations(t)
}

func TestListDispatchLogs_MaxLimit100(t *testing.T) {
	t.Parallel()

	historyRepo := new(mockActionHistoryRepo)
	dispatchLogRepo := new(mockDispatchLogRepo)
	handler := NewHistoryHandler(historyRepo, dispatchLogRepo)

	userID := "user-123"
	ctx := createTestUserContextForHistory(userID)

	limit := int32(500) // exceeds max

	dispatchLogRepo.On("GetUserLogs", mock.Anything, userID, 100, 0).Return([]models.DispatchLog{}, nil)

	req := connect.NewRequest(&historyv1.ListDispatchLogsRequest{
		Limit: &limit,
	})
	_, err := handler.ListDispatchLogs(ctx, req)

	require.NoError(t, err)

	dispatchLogRepo.AssertExpectations(t)
}

func TestListDispatchLogs_HasMore(t *testing.T) {
	t.Parallel()

	historyRepo := new(mockActionHistoryRepo)
	dispatchLogRepo := new(mockDispatchLogRepo)
	handler := NewHistoryHandler(historyRepo, dispatchLogRepo)

	userID := "user-123"
	ctx := createTestUserContextForHistory(userID)

	limit := int32(2)

	// Return exactly limit items -> hasMore = true
	dispatchLogRepo.On("GetUserLogs", mock.Anything, userID, 2, 0).Return([]models.DispatchLog{
		{ID: 1, UserID: userID, Status: "sent", CreatedAt: "2026-01-01T00:00:00Z"},
		{ID: 2, UserID: userID, Status: "sent", CreatedAt: "2026-01-02T00:00:00Z"},
	}, nil)

	req := connect.NewRequest(&historyv1.ListDispatchLogsRequest{
		Limit: &limit,
	})
	resp, err := handler.ListDispatchLogs(ctx, req)

	require.NoError(t, err)
	assert.True(t, resp.Msg.HasMore)

	dispatchLogRepo.AssertExpectations(t)
}

func TestListDispatchLogs_RepoError(t *testing.T) {
	t.Parallel()

	historyRepo := new(mockActionHistoryRepo)
	dispatchLogRepo := new(mockDispatchLogRepo)
	handler := NewHistoryHandler(historyRepo, dispatchLogRepo)

	userID := "user-123"
	ctx := createTestUserContextForHistory(userID)

	dispatchLogRepo.On("GetUserLogs", mock.Anything, userID, 50, 0).Return(nil, errors.New("database error"))

	req := connect.NewRequest(&historyv1.ListDispatchLogsRequest{})
	_, err := handler.ListDispatchLogs(ctx, req)

	require.Error(t, err)
	connectErr, ok := err.(*connect.Error)
	require.True(t, ok)
	assert.Equal(t, connect.CodeInternal, connectErr.Code())

	dispatchLogRepo.AssertExpectations(t)
}

// =============================================================================
// Table-Driven Tests: mapActionTypeToEventType
// =============================================================================

func TestMapActionTypeToEventType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		actionType string
		expected   historyv1.EventType
	}{
		// Primary mappings
		{"message_sent", "message_sent", historyv1.EventType_EVENT_TYPE_MESSAGE_SENT},
		{"send_message alias", "send_message", historyv1.EventType_EVENT_TYPE_MESSAGE_SENT},
		{"automation_run", "automation_run", historyv1.EventType_EVENT_TYPE_AUTOMATION_RUN},
		{"run_automation alias", "run_automation", historyv1.EventType_EVENT_TYPE_AUTOMATION_RUN},
		{"coupon_used", "coupon_used", historyv1.EventType_EVENT_TYPE_COUPON_USED},
		{"link_clicked", "link_clicked", historyv1.EventType_EVENT_TYPE_LINK_CLICKED},
		{"group_sync", "group_sync", historyv1.EventType_EVENT_TYPE_GROUP_SYNC},
		{"sync_groups alias", "sync_groups", historyv1.EventType_EVENT_TYPE_GROUP_SYNC},
		{"import_groups alias", "import_groups", historyv1.EventType_EVENT_TYPE_GROUP_SYNC},
		{"error", "error", historyv1.EventType_EVENT_TYPE_ERROR},
		{"close_groups", "close_groups", historyv1.EventType_EVENT_TYPE_CLOSE_GROUPS},
		{"open_groups", "open_groups", historyv1.EventType_EVENT_TYPE_OPEN_GROUPS},
		{"change_name", "change_name", historyv1.EventType_EVENT_TYPE_CHANGE_NAME},
		{"change_photo", "change_photo", historyv1.EventType_EVENT_TYPE_CHANGE_PHOTO},
		{"change_description", "change_description", historyv1.EventType_EVENT_TYPE_CHANGE_DESCRIPTION},
		{"send_campaign", "send_campaign", historyv1.EventType_EVENT_TYPE_SEND_CAMPAIGN},
		{"generate_image", "generate_image", historyv1.EventType_EVENT_TYPE_GENERATE_IMAGE},
		// Messaging aliases that map to MESSAGE_SENT
		{"send_audio -> MESSAGE_SENT", "send_audio", historyv1.EventType_EVENT_TYPE_MESSAGE_SENT},
		{"send_image -> MESSAGE_SENT", "send_image", historyv1.EventType_EVENT_TYPE_MESSAGE_SENT},
		{"send_poll -> MESSAGE_SENT", "send_poll", historyv1.EventType_EVENT_TYPE_MESSAGE_SENT},
		// Member management
		{"add_member", "add_member", historyv1.EventType_EVENT_TYPE_ADD_MEMBER},
		{"remove_member", "remove_member", historyv1.EventType_EVENT_TYPE_REMOVE_MEMBER},
		{"promote_member", "promote_member", historyv1.EventType_EVENT_TYPE_PROMOTE_MEMBER},
		{"demote_member", "demote_member", historyv1.EventType_EVENT_TYPE_DEMOTE_MEMBER},
		{"leave_group", "leave_group", historyv1.EventType_EVENT_TYPE_LEAVE_GROUP},
		// Default/unknown
		{"unknown type", "unknown_action", historyv1.EventType_EVENT_TYPE_UNSPECIFIED},
		{"empty string", "", historyv1.EventType_EVENT_TYPE_UNSPECIFIED},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := mapActionTypeToEventType(tt.actionType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// =============================================================================
// Table-Driven Tests: mapEventTypeToActionType
// =============================================================================

func TestMapEventTypeToActionType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		eventType historyv1.EventType
		expected  string
	}{
		{"MESSAGE_SENT", historyv1.EventType_EVENT_TYPE_MESSAGE_SENT, "send_message"},
		{"AUTOMATION_RUN", historyv1.EventType_EVENT_TYPE_AUTOMATION_RUN, "run_automation"},
		{"COUPON_USED", historyv1.EventType_EVENT_TYPE_COUPON_USED, "coupon_used"},
		{"LINK_CLICKED", historyv1.EventType_EVENT_TYPE_LINK_CLICKED, "link_clicked"},
		{"GROUP_SYNC", historyv1.EventType_EVENT_TYPE_GROUP_SYNC, "sync_groups"},
		{"ERROR", historyv1.EventType_EVENT_TYPE_ERROR, "error"},
		{"CLOSE_GROUPS", historyv1.EventType_EVENT_TYPE_CLOSE_GROUPS, "close_groups"},
		{"OPEN_GROUPS", historyv1.EventType_EVENT_TYPE_OPEN_GROUPS, "open_groups"},
		{"CHANGE_NAME", historyv1.EventType_EVENT_TYPE_CHANGE_NAME, "change_name"},
		{"CHANGE_PHOTO", historyv1.EventType_EVENT_TYPE_CHANGE_PHOTO, "change_photo"},
		{"CHANGE_DESCRIPTION", historyv1.EventType_EVENT_TYPE_CHANGE_DESCRIPTION, "change_description"},
		{"IMPORT_GROUPS", historyv1.EventType_EVENT_TYPE_IMPORT_GROUPS, "import_groups"},
		{"SEND_CAMPAIGN", historyv1.EventType_EVENT_TYPE_SEND_CAMPAIGN, "send_campaign"},
		{"GENERATE_IMAGE", historyv1.EventType_EVENT_TYPE_GENERATE_IMAGE, "generate_image"},
		{"ADD_MEMBER", historyv1.EventType_EVENT_TYPE_ADD_MEMBER, "add_member"},
		{"REMOVE_MEMBER", historyv1.EventType_EVENT_TYPE_REMOVE_MEMBER, "remove_member"},
		{"PROMOTE_MEMBER", historyv1.EventType_EVENT_TYPE_PROMOTE_MEMBER, "promote_member"},
		{"DEMOTE_MEMBER", historyv1.EventType_EVENT_TYPE_DEMOTE_MEMBER, "demote_member"},
		{"LEAVE_GROUP", historyv1.EventType_EVENT_TYPE_LEAVE_GROUP, "leave_group"},
		{"UNSPECIFIED", historyv1.EventType_EVENT_TYPE_UNSPECIFIED, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := mapEventTypeToActionType(tt.eventType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// =============================================================================
// Table-Driven Tests: historyEventToProto
// =============================================================================

func TestHistoryEventToProto_NilInput(t *testing.T) {
	t.Parallel()

	result := historyEventToProto(nil)
	assert.Nil(t, result)
}

func TestHistoryEventToProto_WithAllFields(t *testing.T) {
	t.Parallel()

	errMsg := "something failed"
	completedAt := "2026-01-01T01:00:00Z"
	targetCount := int32(10)
	successCount := int32(8)
	errorCount := int32(2)

	h := &models.ActionHistory{
		ID:           "h-full",
		UserID:       "user-123",
		ActionType:   "sync_groups",
		Description:  "Synced 10 groups",
		Details:      map[string]interface{}{"instance": "test-instance", "count": float64(10)},
		Status:       "completed",
		ErrorMessage: &errMsg,
		TargetCount:  &targetCount,
		SuccessCount: &successCount,
		ErrorCount:   &errorCount,
		CreatedAt:    "2026-01-01T00:00:00Z",
		CompletedAt:  &completedAt,
	}

	result := historyEventToProto(h)

	require.NotNil(t, result)
	assert.Equal(t, "h-full", result.Id)
	assert.Equal(t, "user-123", result.UserId)
	assert.Equal(t, historyv1.EventType_EVENT_TYPE_GROUP_SYNC, result.EventType)
	assert.Equal(t, "Synced 10 groups", result.Description)
	assert.NotNil(t, result.Metadata)
	assert.Equal(t, "2026-01-01T00:00:00Z", result.CreatedAt)
	require.NotNil(t, result.Status)
	assert.Equal(t, "completed", *result.Status)
	require.NotNil(t, result.ErrorMessage)
	assert.Equal(t, "something failed", *result.ErrorMessage)
	require.NotNil(t, result.CompletedAt)
	assert.Equal(t, "2026-01-01T01:00:00Z", *result.CompletedAt)
	require.NotNil(t, result.TargetCount)
	assert.Equal(t, int32(10), *result.TargetCount)
	require.NotNil(t, result.SuccessCount)
	assert.Equal(t, int32(8), *result.SuccessCount)
	require.NotNil(t, result.ErrorCount)
	assert.Equal(t, int32(2), *result.ErrorCount)
}

func TestHistoryEventToProto_WithoutOptionalFields(t *testing.T) {
	t.Parallel()

	h := &models.ActionHistory{
		ID:          "h-minimal",
		UserID:      "user-123",
		ActionType:  "send_message",
		Description: "Sent a message",
		Status:      "",
		CreatedAt:   "2026-01-01T00:00:00Z",
	}

	result := historyEventToProto(h)

	require.NotNil(t, result)
	assert.Equal(t, "h-minimal", result.Id)
	assert.Equal(t, historyv1.EventType_EVENT_TYPE_MESSAGE_SENT, result.EventType)
	assert.Nil(t, result.Metadata)
	assert.Nil(t, result.Status)
	assert.Nil(t, result.ErrorMessage)
	assert.Nil(t, result.CompletedAt)
	assert.Nil(t, result.TargetCount)
	assert.Nil(t, result.SuccessCount)
	assert.Nil(t, result.ErrorCount)
}

func TestHistoryEventToProto_WithDetailsMapConversion(t *testing.T) {
	t.Parallel()

	h := &models.ActionHistory{
		ID:          "h-details",
		UserID:      "user-123",
		ActionType:  "run_automation",
		Description: "Automation executed",
		Details:     map[string]interface{}{"automation_name": "Test Auto", "groups_count": float64(5)},
		Status:      "running",
		CreatedAt:   "2026-01-01T00:00:00Z",
	}

	result := historyEventToProto(h)

	require.NotNil(t, result)
	require.NotNil(t, result.Metadata)

	fields := result.Metadata.GetFields()
	assert.Equal(t, "Test Auto", fields["automation_name"].GetStringValue())
	assert.Equal(t, float64(5), fields["groups_count"].GetNumberValue())
}

func TestHistoryEventToProto_DetailsNotAMap(t *testing.T) {
	t.Parallel()

	// Details is not a map[string]interface{} - metadata should be nil
	h := &models.ActionHistory{
		ID:          "h-baddetails",
		UserID:      "user-123",
		ActionType:  "send_message",
		Description: "Test",
		Details:     "not a map",
		Status:      "completed",
		CreatedAt:   "2026-01-01T00:00:00Z",
	}

	result := historyEventToProto(h)

	require.NotNil(t, result)
	assert.Nil(t, result.Metadata)
}

// =============================================================================
// Table-Driven Tests: dispatchLogToProto
// =============================================================================

func TestDispatchLogToProto_NilInput(t *testing.T) {
	t.Parallel()

	result := dispatchLogToProto(nil, nil)
	assert.Nil(t, result)
}

func TestDispatchLogToProto_WithAllFields(t *testing.T) {
	t.Parallel()

	automationID := "auto-1"
	automationName := "Test Automation"
	store := "shopee"
	groupID := "group@g.us"
	productURL := "https://shopee.com/product/123"
	productPageURL := "https://shopee.com/product/123?affiliate=xyz"
	canonicalKey := "shopee:product:123"
	errMsg := "network timeout"
	logType := "monitoring"

	log := &models.DispatchLog{
		ID:             42,
		UserID:         "user-123",
		AutomationID:   &automationID,
		AutomationName: &automationName,
		Store:          &store,
		GroupID:        &groupID,
		ProductURL:     &productURL,
		ProductPageURL: &productPageURL,
		CanonicalKey:   &canonicalKey,
		Status:         "sent",
		Error:          &errMsg,
		Type:           &logType,
		CreatedAt:      "2026-01-01T00:00:00Z",
	}

	groupNameMap := map[string]string{
		"group@g.us": "My Group",
	}

	result := dispatchLogToProto(log, groupNameMap)

	require.NotNil(t, result)
	assert.Equal(t, int32(42), result.Id)
	assert.Equal(t, "user-123", result.UserId)
	assert.Equal(t, historyv1.DispatchLogStatus_DISPATCH_LOG_STATUS_SENT, result.Status)
	assert.Equal(t, "2026-01-01T00:00:00Z", result.CreatedAt)
	assert.Equal(t, "auto-1", result.GetAutomationId())
	assert.Equal(t, "Test Automation", result.GetAutomationName())
	assert.Equal(t, "shopee", result.GetStore())
	assert.Equal(t, "group@g.us", result.GetGroupId())
	assert.Equal(t, "My Group", result.GetGroupName())
	assert.Equal(t, "https://shopee.com/product/123", result.GetProductUrl())
	assert.Equal(t, "https://shopee.com/product/123?affiliate=xyz", result.GetProductPageUrl())
	assert.Equal(t, "shopee:product:123", result.GetCanonicalKey())
	assert.Equal(t, "network timeout", result.GetError())
	assert.Equal(t, "monitoring", result.GetType())
}

func TestDispatchLogToProto_WithoutOptionalFields(t *testing.T) {
	t.Parallel()

	log := &models.DispatchLog{
		ID:        1,
		UserID:    "user-123",
		Status:    "pending",
		CreatedAt: "2026-01-01T00:00:00Z",
	}

	result := dispatchLogToProto(log, map[string]string{})

	require.NotNil(t, result)
	assert.Equal(t, int32(1), result.Id)
	assert.Equal(t, "user-123", result.UserId)
	assert.Equal(t, historyv1.DispatchLogStatus_DISPATCH_LOG_STATUS_PENDING, result.Status)
	assert.Nil(t, result.AutomationId)
	assert.Nil(t, result.AutomationName)
	assert.Nil(t, result.Store)
	assert.Nil(t, result.GroupId)
	assert.Nil(t, result.GroupName)
	assert.Nil(t, result.ProductUrl)
	assert.Nil(t, result.ProductPageUrl)
	assert.Nil(t, result.CanonicalKey)
	assert.Nil(t, result.Error)
	assert.Nil(t, result.Type)
}

func TestDispatchLogToProto_GroupNameNotInMap(t *testing.T) {
	t.Parallel()

	groupID := "unknown-group@g.us"
	log := &models.DispatchLog{
		ID:        1,
		UserID:    "user-123",
		GroupID:   &groupID,
		Status:    "sent",
		CreatedAt: "2026-01-01T00:00:00Z",
	}

	// The group is not in the map
	result := dispatchLogToProto(log, map[string]string{
		"other-group@g.us": "Other Group",
	})

	require.NotNil(t, result)
	assert.Equal(t, "unknown-group@g.us", result.GetGroupId())
	assert.Nil(t, result.GroupName)
}

// =============================================================================
// Table-Driven Tests: mapDispatchLogStatus
// =============================================================================

func TestMapDispatchLogStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		status   string
		expected historyv1.DispatchLogStatus
	}{
		{"pending", models.DispatchLogStatusPending, historyv1.DispatchLogStatus_DISPATCH_LOG_STATUS_PENDING},
		{"queued", models.DispatchLogStatusQueued, historyv1.DispatchLogStatus_DISPATCH_LOG_STATUS_QUEUED},
		{"sent", models.DispatchLogStatusSent, historyv1.DispatchLogStatus_DISPATCH_LOG_STATUS_SENT},
		{"failed", models.DispatchLogStatusFailed, historyv1.DispatchLogStatus_DISPATCH_LOG_STATUS_FAILED},
		{"skipped", models.DispatchLogStatusSkipped, historyv1.DispatchLogStatus_DISPATCH_LOG_STATUS_SKIPPED},
		{"blocked", models.DispatchLogStatusBlocked, historyv1.DispatchLogStatus_DISPATCH_LOG_STATUS_BLOCKED},
		{"unknown status", "unknown", historyv1.DispatchLogStatus_DISPATCH_LOG_STATUS_UNSPECIFIED},
		{"empty string", "", historyv1.DispatchLogStatus_DISPATCH_LOG_STATUS_UNSPECIFIED},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := mapDispatchLogStatus(tt.status)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// =============================================================================
// Constructor Tests
// =============================================================================

func TestNewHistoryHandler(t *testing.T) {
	t.Parallel()

	historyRepo := new(mockActionHistoryRepo)
	dispatchLogRepo := new(mockDispatchLogRepo)

	handler := NewHistoryHandler(historyRepo, dispatchLogRepo)

	require.NotNil(t, handler)
	assert.NotNil(t, handler.historyRepo)
	assert.NotNil(t, handler.dispatchLogRepo)
	assert.Nil(t, handler.groupRepo)
}

func TestNewHistoryHandlerWithGroups(t *testing.T) {
	t.Parallel()

	historyRepo := new(mockActionHistoryRepo)
	dispatchLogRepo := new(mockDispatchLogRepo)
	groupRepo := new(mockGroupRepoForHistory)

	handler := NewHistoryHandlerWithGroups(historyRepo, dispatchLogRepo, groupRepo)

	require.NotNil(t, handler)
	assert.NotNil(t, handler.historyRepo)
	assert.NotNil(t, handler.dispatchLogRepo)
	assert.NotNil(t, handler.groupRepo)
}

// =============================================================================
// Additional Edge Cases
// =============================================================================

func TestListHistory_LegacyPagination_NegativeOffset(t *testing.T) {
	t.Parallel()

	historyRepo := new(mockActionHistoryRepo)
	dispatchLogRepo := new(mockDispatchLogRepo)
	handler := NewHistoryHandler(historyRepo, dispatchLogRepo)

	userID := "user-123"
	ctx := createTestUserContextForHistory(userID)

	limit := int32(10)
	offset := int32(-5) // negative offset should be clamped to 0

	historyRepo.On("GetUserHistoryFiltered", mock.Anything, mock.MatchedBy(func(req models.HistoryListRequest) bool {
		return req.Offset == 0
	}), userID).Return([]models.ActionHistory{}, nil)
	historyRepo.On("CountUserHistoryFiltered", mock.Anything, mock.Anything, userID).Return(models.HistoryStatsResult{Total: 0}, nil)

	req := connect.NewRequest(&historyv1.ListHistoryRequest{
		Limit:  &limit,
		Offset: &offset,
	})
	_, err := handler.ListHistory(ctx, req)

	require.NoError(t, err)

	historyRepo.AssertExpectations(t)
}

func TestListHistory_LegacyPagination_LimitExceeds100(t *testing.T) {
	t.Parallel()

	historyRepo := new(mockActionHistoryRepo)
	dispatchLogRepo := new(mockDispatchLogRepo)
	handler := NewHistoryHandler(historyRepo, dispatchLogRepo)

	userID := "user-123"
	ctx := createTestUserContextForHistory(userID)

	limit := int32(200) // exceeds max

	historyRepo.On("GetUserHistoryFiltered", mock.Anything, mock.MatchedBy(func(req models.HistoryListRequest) bool {
		return req.Limit == 100 // capped
	}), userID).Return([]models.ActionHistory{}, nil)
	historyRepo.On("CountUserHistoryFiltered", mock.Anything, mock.Anything, userID).Return(models.HistoryStatsResult{Total: 0}, nil)

	req := connect.NewRequest(&historyv1.ListHistoryRequest{
		Limit: &limit,
	})
	_, err := handler.ListHistory(ctx, req)

	require.NoError(t, err)

	historyRepo.AssertExpectations(t)
}

func TestListHistory_UnspecifiedEventTypeFilter_Ignored(t *testing.T) {
	t.Parallel()

	historyRepo := new(mockActionHistoryRepo)
	dispatchLogRepo := new(mockDispatchLogRepo)
	handler := NewHistoryHandler(historyRepo, dispatchLogRepo)

	userID := "user-123"
	ctx := createTestUserContextForHistory(userID)

	unspecified := historyv1.EventType_EVENT_TYPE_UNSPECIFIED

	historyRepo.On("GetUserHistoryFiltered", mock.Anything, mock.MatchedBy(func(req models.HistoryListRequest) bool {
		return req.ActionType == nil // unspecified should not set the filter
	}), userID).Return([]models.ActionHistory{}, nil)
	historyRepo.On("CountUserHistoryFiltered", mock.Anything, mock.Anything, userID).Return(models.HistoryStatsResult{Total: 0}, nil)

	req := connect.NewRequest(&historyv1.ListHistoryRequest{
		EventType: &unspecified,
	})
	_, err := handler.ListHistory(ctx, req)

	require.NoError(t, err)

	historyRepo.AssertExpectations(t)
}

func TestListHistory_EmptyStatusFilter_Ignored(t *testing.T) {
	t.Parallel()

	historyRepo := new(mockActionHistoryRepo)
	dispatchLogRepo := new(mockDispatchLogRepo)
	handler := NewHistoryHandler(historyRepo, dispatchLogRepo)

	userID := "user-123"
	ctx := createTestUserContextForHistory(userID)

	emptyStatus := ""

	historyRepo.On("GetUserHistoryFiltered", mock.Anything, mock.MatchedBy(func(req models.HistoryListRequest) bool {
		return req.Status == nil // empty string should not set the filter
	}), userID).Return([]models.ActionHistory{}, nil)
	historyRepo.On("CountUserHistoryFiltered", mock.Anything, mock.Anything, userID).Return(models.HistoryStatsResult{Total: 0}, nil)

	req := connect.NewRequest(&historyv1.ListHistoryRequest{
		Status: &emptyStatus,
	})
	_, err := handler.ListHistory(ctx, req)

	require.NoError(t, err)

	historyRepo.AssertExpectations(t)
}

func TestClearHistory_UnspecifiedEventType_Skipped(t *testing.T) {
	t.Parallel()

	historyRepo := new(mockActionHistoryRepo)
	dispatchLogRepo := new(mockDispatchLogRepo)
	handler := NewHistoryHandler(historyRepo, dispatchLogRepo)

	userID := "user-123"
	ctx := createTestUserContextForHistory(userID)

	// UNSPECIFIED should be skipped, only ERROR should be included
	historyRepo.On("DeleteUserHistory", mock.Anything, userID, (*time.Time)(nil), mock.MatchedBy(func(types []string) bool {
		return len(types) == 1 && types[0] == "error"
	})).Return(int64(2), nil)

	req := connect.NewRequest(&historyv1.ClearHistoryRequest{
		EventTypes: []historyv1.EventType{
			historyv1.EventType_EVENT_TYPE_UNSPECIFIED,
			historyv1.EventType_EVENT_TYPE_ERROR,
		},
	})
	resp, err := handler.ClearHistory(ctx, req)

	require.NoError(t, err)
	assert.Equal(t, int32(2), resp.Msg.DeletedCount)

	historyRepo.AssertExpectations(t)
}

func TestListDispatchLogs_NoGroupRepo_SkipsGroupLookup(t *testing.T) {
	t.Parallel()

	historyRepo := new(mockActionHistoryRepo)
	dispatchLogRepo := new(mockDispatchLogRepo)
	// groupRepo is nil (NewHistoryHandler, not NewHistoryHandlerWithGroups)
	handler := NewHistoryHandler(historyRepo, dispatchLogRepo)

	userID := "user-123"
	ctx := createTestUserContextForHistory(userID)

	groupID := "group@g.us"

	dispatchLogRepo.On("GetUserLogs", mock.Anything, userID, 50, 0).Return([]models.DispatchLog{
		{ID: 1, UserID: userID, GroupID: &groupID, Status: "sent", CreatedAt: "2026-01-01T00:00:00Z"},
	}, nil)

	req := connect.NewRequest(&historyv1.ListDispatchLogsRequest{})
	resp, err := handler.ListDispatchLogs(ctx, req)

	require.NoError(t, err)
	assert.Len(t, resp.Msg.Logs, 1)
	// group_name should not be set since groupRepo is nil
	assert.Nil(t, resp.Msg.Logs[0].GroupName)

	dispatchLogRepo.AssertExpectations(t)
}

func TestListDispatchLogs_NoGroupIDs_SkipsGroupLookup(t *testing.T) {
	t.Parallel()

	historyRepo := new(mockActionHistoryRepo)
	dispatchLogRepo := new(mockDispatchLogRepo)
	groupRepo := new(mockGroupRepoForHistory)
	handler := NewHistoryHandlerWithGroups(historyRepo, dispatchLogRepo, groupRepo)

	userID := "user-123"
	ctx := createTestUserContextForHistory(userID)

	// Logs have no group IDs -> should not call GetGroupsByIDs
	dispatchLogRepo.On("GetUserLogs", mock.Anything, userID, 50, 0).Return([]models.DispatchLog{
		{ID: 1, UserID: userID, Status: "sent", CreatedAt: "2026-01-01T00:00:00Z"},
	}, nil)

	req := connect.NewRequest(&historyv1.ListDispatchLogsRequest{})
	resp, err := handler.ListDispatchLogs(ctx, req)

	require.NoError(t, err)
	assert.Len(t, resp.Msg.Logs, 1)

	dispatchLogRepo.AssertExpectations(t)
	// GetGroupsByIDs should NOT have been called
	groupRepo.AssertNotCalled(t, "GetGroupsByIDs")
}

// =============================================================================
// Table-Driven Tests: protoStatusToDispatchLogStatus
// =============================================================================

func TestProtoStatusToDispatchLogStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		status   historyv1.DispatchLogStatus
		expected string
	}{
		{"PENDING", historyv1.DispatchLogStatus_DISPATCH_LOG_STATUS_PENDING, models.DispatchLogStatusPending},
		{"QUEUED", historyv1.DispatchLogStatus_DISPATCH_LOG_STATUS_QUEUED, models.DispatchLogStatusQueued},
		{"SENT", historyv1.DispatchLogStatus_DISPATCH_LOG_STATUS_SENT, models.DispatchLogStatusSent},
		{"FAILED", historyv1.DispatchLogStatus_DISPATCH_LOG_STATUS_FAILED, models.DispatchLogStatusFailed},
		{"SKIPPED", historyv1.DispatchLogStatus_DISPATCH_LOG_STATUS_SKIPPED, models.DispatchLogStatusSkipped},
		{"BLOCKED", historyv1.DispatchLogStatus_DISPATCH_LOG_STATUS_BLOCKED, models.DispatchLogStatusBlocked},
		{"UNSPECIFIED", historyv1.DispatchLogStatus_DISPATCH_LOG_STATUS_UNSPECIFIED, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := protoStatusToDispatchLogStatus(tt.status)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// =============================================================================
// Table-Driven Tests: parseDispatchLogID
// =============================================================================

func TestParseDispatchLogID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		expected  int
		expectErr bool
	}{
		{"valid integer", "42", 42, false},
		{"zero", "0", 0, false},
		{"negative", "-1", -1, false},
		{"empty string", "", 0, true},
		{"not a number", "abc", 0, true},
		{"float", "1.5", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := parseDispatchLogID(tt.input)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
