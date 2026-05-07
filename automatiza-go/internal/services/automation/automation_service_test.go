package automation_test

import (
	"context"
	"errors"
	"testing"
	"time"

	bpLib "automatiza-go/internal/lib/backpressure"
	"automatiza-go/internal/lib/urlsafety"
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"
	"automatiza-go/internal/services/automation"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// =============================================================================
// Mock Implementations
// =============================================================================

// MockAutomationRepository implements repository.AutomationRepository
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

// MockAutomationRunRepository implements repository.AutomationRunRepository
type MockAutomationRunRepository struct {
	mock.Mock
}

func (m *MockAutomationRunRepository) GetLock(ctx context.Context, automationID string) (*time.Time, error) {
	args := m.Called(ctx, automationID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*time.Time), args.Error(1)
}

func (m *MockAutomationRunRepository) AcquireLock(ctx context.Context, automationID string, duration time.Duration) (*time.Time, error) {
	args := m.Called(ctx, automationID, duration)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*time.Time), args.Error(1)
}

func (m *MockAutomationRunRepository) ReleaseLock(ctx context.Context, automationID string) error {
	args := m.Called(ctx, automationID)
	return args.Error(0)
}

func (m *MockAutomationRunRepository) CreateRun(ctx context.Context, automationID string, status string) (string, error) {
	args := m.Called(ctx, automationID, status)
	return args.String(0), args.Error(1)
}

func (m *MockAutomationRunRepository) UpdateRunStatus(ctx context.Context, runID string, status string, errorMsg *string) error {
	args := m.Called(ctx, runID, status, errorMsg)
	return args.Error(0)
}

func (m *MockAutomationRunRepository) UpdateRunStats(ctx context.Context, runID string, offersFound, messagesSent, messagesFailed int) error {
	args := m.Called(ctx, runID, offersFound, messagesSent, messagesFailed)
	return args.Error(0)
}

func (m *MockAutomationRunRepository) GetRunByID(ctx context.Context, runID string) (*models.AutomationRun, error) {
	args := m.Called(ctx, runID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.AutomationRun), args.Error(1)
}

func (m *MockAutomationRunRepository) GetRunsByAutomationID(ctx context.Context, automationID string, limit int) ([]models.AutomationRun, error) {
	args := m.Called(ctx, automationID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.AutomationRun), args.Error(1)
}

func (m *MockAutomationRunRepository) CleanupOldRuns(ctx context.Context, olderThan time.Duration) (int64, error) {
	args := m.Called(ctx, olderThan)
	return args.Get(0).(int64), args.Error(1)
}

// MockInstanceRepository implements repository.InstanceRepository
type MockInstanceRepository struct {
	mock.Mock
}

func (m *MockInstanceRepository) GetUserInstances(ctx context.Context, userID string) ([]models.Instance, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Instance), args.Error(1)
}

func (m *MockInstanceRepository) GetInstanceByID(ctx context.Context, instanceID int) (*models.Instance, error) {
	args := m.Called(ctx, instanceID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Instance), args.Error(1)
}

func (m *MockInstanceRepository) GetInstanceByName(ctx context.Context, instanceName string) (*models.Instance, error) {
	args := m.Called(ctx, instanceName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Instance), args.Error(1)
}

func (m *MockInstanceRepository) GetInstanceByUserAndName(ctx context.Context, userID, instanceName string) (*models.Instance, error) {
	args := m.Called(ctx, userID, instanceName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Instance), args.Error(1)
}

func (m *MockInstanceRepository) GetAllConnectedInstances(ctx context.Context) ([]models.Instance, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Instance), args.Error(1)
}

func (m *MockInstanceRepository) UpsertInstance(ctx context.Context, userID string, data *models.Instance) (*models.Instance, error) {
	args := m.Called(ctx, userID, data)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Instance), args.Error(1)
}

func (m *MockInstanceRepository) UpdateInstanceStatus(ctx context.Context, instanceID int, status string) error {
	args := m.Called(ctx, instanceID, status)
	return args.Error(0)
}

func (m *MockInstanceRepository) DeleteInstance(ctx context.Context, instanceID int) error {
	args := m.Called(ctx, instanceID)
	return args.Error(0)
}

func (m *MockInstanceRepository) SetDefaultInstance(ctx context.Context, userID string, instanceID int) error {
	args := m.Called(ctx, userID, instanceID)
	return args.Error(0)
}

func (m *MockInstanceRepository) CountUserInstances(ctx context.Context, userID string) (int, error) {
	args := m.Called(ctx, userID)
	return args.Int(0), args.Error(1)
}

func (m *MockInstanceRepository) UpdateInstanceLabel(ctx context.Context, userID string, instanceID int, friendlyName string) error {
	args := m.Called(ctx, userID, instanceID, friendlyName)
	return args.Error(0)
}

func (m *MockInstanceRepository) UpdateInstancePhoneNumber(ctx context.Context, instanceID int, phoneNumber string) error {
	args := m.Called(ctx, instanceID, phoneNumber)
	return args.Error(0)
}

func (m *MockInstanceRepository) SupersedePriorInstances(ctx context.Context, userID string, currentInstanceID int, phoneNumber string) (int, error) {
	args := m.Called(ctx, userID, currentInstanceID, phoneNumber)
	return args.Int(0), args.Error(1)
}

// MockGroupRepository implements repository.GroupRepository
type MockGroupRepository struct {
	mock.Mock
}

func (m *MockGroupRepository) GetUserGroups(ctx context.Context, userID string, adminOnly bool) ([]models.Group, error) {
	args := m.Called(ctx, userID, adminOnly)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Group), args.Error(1)
}

func (m *MockGroupRepository) GetUserGroupsPaginated(ctx context.Context, userID string, page, pageSize int, adminOnly bool, search string) (*repository.GroupPaginatedResult, error) {
	args := m.Called(ctx, userID, page, pageSize, adminOnly, search)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.GroupPaginatedResult), args.Error(1)
}

func (m *MockGroupRepository) GetGroupByID(ctx context.Context, groupID string) (*models.Group, error) {
	args := m.Called(ctx, groupID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Group), args.Error(1)
}

func (m *MockGroupRepository) GetGroupByWAGroupID(ctx context.Context, userID, waGroupID string) (*models.Group, error) {
	args := m.Called(ctx, userID, waGroupID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Group), args.Error(1)
}

func (m *MockGroupRepository) GetGroupsByIDs(ctx context.Context, userID string, groupIDs []string) ([]models.Group, error) {
	args := m.Called(ctx, userID, groupIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Group), args.Error(1)
}

func (m *MockGroupRepository) GetFavoriteGroups(ctx context.Context, userID string) ([]models.Group, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Group), args.Error(1)
}

func (m *MockGroupRepository) CreateGroup(ctx context.Context, userID string, group *models.Group) (*models.Group, error) {
	args := m.Called(ctx, userID, group)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Group), args.Error(1)
}

func (m *MockGroupRepository) SyncGroups(ctx context.Context, userID string, instanceID int, groups []models.Group, options repository.SyncOptions) error {
	args := m.Called(ctx, userID, instanceID, groups, options)
	return args.Error(0)
}

func (m *MockGroupRepository) SyncGroupsWithStats(ctx context.Context, userID string, instanceID int, groups []models.Group, options repository.SyncOptions) (*repository.SyncResult, error) {
	args := m.Called(ctx, userID, instanceID, groups, options)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.SyncResult), args.Error(1)
}

func (m *MockGroupRepository) IsUserAdminInGroup(ctx context.Context, userID, waGroupID string) (bool, error) {
	args := m.Called(ctx, userID, waGroupID)
	return args.Bool(0), args.Error(1)
}

func (m *MockGroupRepository) UpdateGroup(ctx context.Context, groupID string, updates map[string]interface{}) error {
	args := m.Called(ctx, groupID, updates)
	return args.Error(0)
}

func (m *MockGroupRepository) ToggleFavorite(ctx context.Context, groupID, userID string, favorite bool) error {
	args := m.Called(ctx, groupID, userID, favorite)
	return args.Error(0)
}

func (m *MockGroupRepository) DeleteGroup(ctx context.Context, groupID, userID string) error {
	args := m.Called(ctx, groupID, userID)
	return args.Error(0)
}

func (m *MockGroupRepository) DeleteGroupsByInstance(ctx context.Context, userID string, instanceID int) error {
	args := m.Called(ctx, userID, instanceID)
	return args.Error(0)
}

func (m *MockGroupRepository) BulkUpdateGroups(ctx context.Context, userID string, groupIDs []string, updates map[string]interface{}) (int64, error) {
	args := m.Called(ctx, userID, groupIDs, updates)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockGroupRepository) CountAll(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockGroupRepository) CountGroupsByInstance(ctx context.Context, userID string, instanceID int) (int, error) {
	args := m.Called(ctx, userID, instanceID)
	return args.Int(0), args.Error(1)
}

func (m *MockGroupRepository) GetInstancesForGroups(ctx context.Context, userID string, waGroupIDs []string) (map[string]*models.Instance, error) {
	args := m.Called(ctx, userID, waGroupIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]*models.Instance), args.Error(1)
}

// MockDispatchLogRepository implements repository.DispatchLogRepository
type MockDispatchLogRepository struct {
	mock.Mock
}

func (m *MockDispatchLogRepository) Create(ctx context.Context, log *models.DispatchLog) (*models.DispatchLog, error) {
	args := m.Called(ctx, log)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.DispatchLog), args.Error(1)
}

func (m *MockDispatchLogRepository) Update(ctx context.Context, logID int, updates map[string]interface{}) error {
	args := m.Called(ctx, logID, updates)
	return args.Error(0)
}

func (m *MockDispatchLogRepository) GetRecentByCanonicalKey(ctx context.Context, userID, groupID, canonicalKey string, hours int) (*models.DispatchLog, error) {
	args := m.Called(ctx, userID, groupID, canonicalKey, hours)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.DispatchLog), args.Error(1)
}

func (m *MockDispatchLogRepository) GetUserLogs(ctx context.Context, userID string, limit, offset int) ([]models.DispatchLog, error) {
	args := m.Called(ctx, userID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.DispatchLog), args.Error(1)
}

func (m *MockDispatchLogRepository) DeleteRecentByAutomation(ctx context.Context, automationID string, sinceDays int) (int64, error) {
	args := m.Called(ctx, automationID, sinceDays)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockDispatchLogRepository) GetByAutomationIDWithCount(ctx context.Context, automationID, userID string, limit, offset int) ([]repository.DispatchLogWithGroup, int, error) {
	args := m.Called(ctx, automationID, userID, limit, offset)
	if args.Get(0) == nil {
		return nil, 0, args.Error(2)
	}
	return args.Get(0).([]repository.DispatchLogWithGroup), args.Int(1), args.Error(2)
}

func (m *MockDispatchLogRepository) GetUserLogsWithFilters(ctx context.Context, userID string, filters *models.DispatchLogFilters, limit, offset int) ([]repository.DispatchLogWithGroup, int64, error) {
	args := m.Called(ctx, userID, filters, limit, offset)
	if args.Get(0) == nil {
		return nil, 0, args.Error(2)
	}
	return args.Get(0).([]repository.DispatchLogWithGroup), args.Get(1).(int64), args.Error(2)
}

func (m *MockDispatchLogRepository) ExistsByCanonicalKey(ctx context.Context, userID, groupID, canonicalKey string, hours int) (bool, error) {
	args := m.Called(ctx, userID, groupID, canonicalKey, hours)
	return args.Bool(0), args.Error(1)
}

func (m *MockDispatchLogRepository) CreateIfNotExists(ctx context.Context, log *models.DispatchLog, dedupWindowHours int) (*models.DispatchLog, bool, error) {
	args := m.Called(ctx, log, dedupWindowHours)
	if args.Get(0) == nil {
		return nil, false, args.Error(2)
	}
	return args.Get(0).(*models.DispatchLog), args.Bool(1), args.Error(2)
}

func (m *MockDispatchLogRepository) DeleteOldLogsByStatus(ctx context.Context, status string, olderThan time.Duration) (int64, error) {
	args := m.Called(ctx, status, olderThan)
	return args.Get(0).(int64), args.Error(1)
}

// MockActionHistoryRepository implements repository.ActionHistoryRepository
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

func (m *MockActionHistoryRepository) DeleteUserHistory(ctx context.Context, userID string, beforeDate *time.Time, actionTypes []string) (int64, error) {
	args := m.Called(ctx, userID, beforeDate, actionTypes)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockActionHistoryRepository) CountUserHistoryFiltered(ctx context.Context, req models.HistoryListRequest, userID string) (models.HistoryStatsResult, error) {
	args := m.Called(ctx, req, userID)
	return args.Get(0).(models.HistoryStatsResult), args.Error(1)
}

// MockSettingsRepository implements repository.SettingsRepository
type MockSettingsRepository struct {
	mock.Mock
}

func (m *MockSettingsRepository) Get(ctx context.Context, keys ...string) (models.SettingsMap, error) {
	args := m.Called(ctx, keys)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(models.SettingsMap), args.Error(1)
}

func (m *MockSettingsRepository) GetAll(ctx context.Context) (models.SettingsMap, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(models.SettingsMap), args.Error(1)
}

func (m *MockSettingsRepository) Update(ctx context.Context, key string, value interface{}) error {
	args := m.Called(ctx, key, value)
	return args.Error(0)
}

func (m *MockSettingsRepository) GetDispatchText(ctx context.Context, dispatchTextID string) (*models.DispatchText, error) {
	args := m.Called(ctx, dispatchTextID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.DispatchText), args.Error(1)
}

func (m *MockSettingsRepository) GetDispatchTextsByUser(ctx context.Context, userID string) ([]models.DispatchText, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.DispatchText), args.Error(1)
}

func (m *MockSettingsRepository) GetActiveDispatchTextByID(ctx context.Context, dispatchTextID, userID string) (*models.DispatchText, error) {
	args := m.Called(ctx, dispatchTextID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.DispatchText), args.Error(1)
}

func (m *MockSettingsRepository) GetActiveDispatchTextsByUser(ctx context.Context, userID string) ([]models.DispatchText, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.DispatchText), args.Error(1)
}

func (m *MockSettingsRepository) CreateDispatchText(ctx context.Context, userID string, dt *models.DispatchText) (*models.DispatchText, error) {
	args := m.Called(ctx, userID, dt)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.DispatchText), args.Error(1)
}

func (m *MockSettingsRepository) UpdateDispatchText(ctx context.Context, dispatchTextID string, updates map[string]interface{}) error {
	args := m.Called(ctx, dispatchTextID, updates)
	return args.Error(0)
}

func (m *MockSettingsRepository) DeleteDispatchText(ctx context.Context, dispatchTextID, userID string) error {
	args := m.Called(ctx, dispatchTextID, userID)
	return args.Error(0)
}

// MockProfileRepository implements repository.ProfileRepository
type MockProfileRepository struct {
	mock.Mock
}

func (m *MockProfileRepository) EnsureUserProfile(ctx context.Context, userID, email string) (*models.Profile, error) {
	args := m.Called(ctx, userID, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Profile), args.Error(1)
}

func (m *MockProfileRepository) GetProfile(ctx context.Context, userID string) (*models.Profile, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Profile), args.Error(1)
}

func (m *MockProfileRepository) GetProfileByEmail(ctx context.Context, email string) (*models.Profile, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Profile), args.Error(1)
}

func (m *MockProfileRepository) GetProfileByEmailWithBilling(ctx context.Context, email string) (*models.Profile, *models.ProfileBillingFields, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, nil, args.Error(2)
	}
	if args.Get(1) == nil {
		return args.Get(0).(*models.Profile), nil, args.Error(2)
	}
	return args.Get(0).(*models.Profile), args.Get(1).(*models.ProfileBillingFields), args.Error(2)
}

func (m *MockProfileRepository) CreateProfile(ctx context.Context, profile *models.Profile) (*models.Profile, error) {
	args := m.Called(ctx, profile)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Profile), args.Error(1)
}

func (m *MockProfileRepository) UpdateProfile(ctx context.Context, userID string, updates map[string]interface{}) error {
	args := m.Called(ctx, userID, updates)
	return args.Error(0)
}

func (m *MockProfileRepository) UpdatePassword(ctx context.Context, userID, passwordHash string) error {
	args := m.Called(ctx, userID, passwordHash)
	return args.Error(0)
}

func (m *MockProfileRepository) UpdateBillingStatus(ctx context.Context, userID string, billing *models.ProfileBillingFields) error {
	args := m.Called(ctx, userID, billing)
	return args.Error(0)
}

func (m *MockProfileRepository) PauseUserAutomations(ctx context.Context, userID string) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *MockProfileRepository) ListProfiles(ctx context.Context, cursor string, limit int) ([]*models.Profile, string, error) {
	args := m.Called(ctx, cursor, limit)
	if args.Get(0) == nil {
		return nil, "", args.Error(2)
	}
	return args.Get(0).([]*models.Profile), args.String(1), args.Error(2)
}

func (m *MockProfileRepository) SearchProfiles(ctx context.Context, query, cursor string, limit int) ([]*models.Profile, string, error) {
	args := m.Called(ctx, query, cursor, limit)
	if args.Get(0) == nil {
		return nil, "", args.Error(2)
	}
	return args.Get(0).([]*models.Profile), args.String(1), args.Error(2)
}

func (m *MockProfileRepository) GetMaxAutomations(ctx context.Context, userID string) (int, error) {
	args := m.Called(ctx, userID)
	return args.Int(0), args.Error(1)
}

func (m *MockProfileRepository) CountUserAutomations(ctx context.Context, userID string) (int, error) {
	args := m.Called(ctx, userID)
	return args.Int(0), args.Error(1)
}

func (m *MockProfileRepository) CreateUserCredentials(ctx context.Context, userID, passwordHash string) error {
	args := m.Called(ctx, userID, passwordHash)
	return args.Error(0)
}

func (m *MockProfileRepository) GetProfileStats(ctx context.Context) (*repository.ProfileStats, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.ProfileStats), args.Error(1)
}

// MockMlLinkListItemRepository implements repository.MlLinkListItemRepository
type MockMlLinkListItemRepository struct {
	mock.Mock
}

func (m *MockMlLinkListItemRepository) GetNextUnconsumed(ctx context.Context, listID string, limit int) ([]*models.MlLinkListItem, error) {
	args := m.Called(ctx, listID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.MlLinkListItem), args.Error(1)
}

func (m *MockMlLinkListItemRepository) MarkAsConsumed(ctx context.Context, itemIDs []string, automationID string, groupID *string) error {
	args := m.Called(ctx, itemIDs, automationID, groupID)
	return args.Error(0)
}

func (m *MockMlLinkListItemRepository) CountRemaining(ctx context.Context, listID string) (int, error) {
	args := m.Called(ctx, listID)
	return args.Int(0), args.Error(1)
}

func (m *MockMlLinkListItemRepository) GetByListID(ctx context.Context, listID string) ([]*models.MlLinkListItem, error) {
	args := m.Called(ctx, listID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.MlLinkListItem), args.Error(1)
}

func (m *MockMlLinkListItemRepository) Create(ctx context.Context, item *models.MlLinkListItem) error {
	args := m.Called(ctx, item)
	return args.Error(0)
}

func (m *MockMlLinkListItemRepository) BulkCreate(ctx context.Context, items []*models.MlLinkListItem) error {
	args := m.Called(ctx, items)
	return args.Error(0)
}

func (m *MockMlLinkListItemRepository) ResetConsumed(ctx context.Context, listID string) (int64, error) {
	args := m.Called(ctx, listID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockMlLinkListItemRepository) GetConsumedByAutomation(ctx context.Context, automationID string) ([]*models.MlLinkListItem, error) {
	args := m.Called(ctx, automationID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.MlLinkListItem), args.Error(1)
}

// MockShopeeService implements shopee.Service
type MockShopeeService struct {
	mock.Mock
}

func (m *MockShopeeService) SearchOffers(ctx context.Context, userID string, params models.ShopeeSearchParams) (*models.ShopeeSearchResult, error) {
	args := m.Called(ctx, userID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ShopeeSearchResult), args.Error(1)
}

func (m *MockShopeeService) GenerateAffiliateLink(ctx context.Context, productURL, userID string) (*models.ShopeeAffiliateLink, error) {
	args := m.Called(ctx, productURL, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ShopeeAffiliateLink), args.Error(1)
}

func (m *MockShopeeService) GetCategories(ctx context.Context, parentID *int64) ([]models.ShopeeCategory, error) {
	args := m.Called(ctx, parentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.ShopeeCategory), args.Error(1)
}

func (m *MockShopeeService) GetProduct(ctx context.Context, itemID, shopID int64, userID string) (*models.ShopeeOffer, error) {
	args := m.Called(ctx, itemID, shopID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ShopeeOffer), args.Error(1)
}

func (m *MockShopeeService) ExpandURL(ctx context.Context, shortURL string) (string, error) {
	args := m.Called(ctx, shortURL)
	return args.String(0), args.Error(1)
}

func (m *MockShopeeService) ExtractProductIDs(productURL string) (shopID, itemID int64, err error) {
	args := m.Called(productURL)
	return args.Get(0).(int64), args.Get(1).(int64), args.Error(2)
}

// MockMLService implements mercadolivre.Service
type MockMLService struct {
	mock.Mock
}

func (m *MockMLService) CreateAffiliateLink(ctx context.Context, originURL, userID string) (*models.MLAffiliateLink, error) {
	args := m.Called(ctx, originURL, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.MLAffiliateLink), args.Error(1)
}

func (m *MockMLService) GetProductPreview(ctx context.Context, originURL, userID string) (*models.MLProductPreview, error) {
	args := m.Called(ctx, originURL, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.MLProductPreview), args.Error(1)
}

func (m *MockMLService) LoadCredentials(ctx context.Context, userID string) (*models.MLCredentials, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.MLCredentials), args.Error(1)
}

func (m *MockMLService) SaveCredentials(ctx context.Context, userID, cookie, csrf, tag string) error {
	args := m.Called(ctx, userID, cookie, csrf, tag)
	return args.Error(0)
}

func (m *MockMLService) IsConnected(ctx context.Context, userID string) (bool, error) {
	args := m.Called(ctx, userID)
	return args.Bool(0), args.Error(1)
}

func (m *MockMLService) ExtractProductID(productURL string) (string, error) {
	args := m.Called(productURL)
	return args.String(0), args.Error(1)
}

func (m *MockMLService) InvalidatePreviewCache(ctx context.Context, originURL string) error {
	args := m.Called(ctx, originURL)
	return args.Error(0)
}

// =============================================================================
// Test Fixtures
// =============================================================================

type testServiceDeps struct {
	automationRepo     *MockAutomationRepository
	automationRunRepo  *MockAutomationRunRepository
	instanceRepo       *MockInstanceRepository
	groupRepo          *MockGroupRepository
	dispatchLogRepo    *MockDispatchLogRepository
	historyRepo        *MockActionHistoryRepository
	settingsRepo       *MockSettingsRepository
	profileRepo        *MockProfileRepository
	mlLinkListItemRepo *MockMlLinkListItemRepository
	shopeeService      *MockShopeeService
	mlService          *MockMLService
}

func newTestService(t *testing.T) (automation.Service, *testServiceDeps) {
	t.Helper()
	logger := zaptest.NewLogger(t)

	deps := &testServiceDeps{
		automationRepo:     new(MockAutomationRepository),
		automationRunRepo:  new(MockAutomationRunRepository),
		instanceRepo:       new(MockInstanceRepository),
		groupRepo:          new(MockGroupRepository),
		dispatchLogRepo:    new(MockDispatchLogRepository),
		historyRepo:        new(MockActionHistoryRepository),
		settingsRepo:       new(MockSettingsRepository),
		profileRepo:        new(MockProfileRepository),
		mlLinkListItemRepo: new(MockMlLinkListItemRepository),
		shopeeService:      new(MockShopeeService),
		mlService:          new(MockMLService),
	}

	// Default fire-and-forget expectations for history repo methods
	// called during automation runs (MarkRunning/MarkCompleted/MarkFailed/UpdateCounts/IncrementSuccessCount/IncrementErrorCount).
	// Individual tests can override with explicit expectations.
	deps.historyRepo.On("MarkRunning", mock.Anything, mock.AnythingOfType("string")).Return(nil).Maybe()
	deps.historyRepo.On("MarkCompleted", mock.Anything, mock.AnythingOfType("string")).Return(nil).Maybe()
	deps.historyRepo.On("MarkFailed", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(nil).Maybe()
	deps.historyRepo.On("UpdateCounts", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("int32"), mock.AnythingOfType("int32")).Return(nil).Maybe()
	deps.historyRepo.On("IncrementSuccessCount", mock.Anything, mock.AnythingOfType("string")).Return(nil).Maybe()
	deps.historyRepo.On("IncrementErrorCount", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(nil).Maybe()

	svc := automation.NewAutomationService(
		deps.automationRepo,
		deps.automationRunRepo,
		deps.instanceRepo,
		deps.groupRepo,
		deps.dispatchLogRepo,
		deps.historyRepo,
		deps.settingsRepo,
		deps.profileRepo,
		deps.mlLinkListItemRepo,
		nil, // ctaRepo
		deps.shopeeService,
		deps.mlService,
		nil, // urlSafetyService
		nil, // backpressure
		nil, // queueClient
		nil, // cache
		nil, // couponRepo
		logger,
	)

	return svc, deps
}

// setupDefaultHistoryExpectations adds Maybe() expectations for fire-and-forget
// history repo methods called during automation runs. Use this when creating
// testServiceDeps manually (outside newTestService).
func setupDefaultHistoryExpectations(historyRepo *MockActionHistoryRepository) {
	historyRepo.On("MarkRunning", mock.Anything, mock.AnythingOfType("string")).Return(nil).Maybe()
	historyRepo.On("MarkCompleted", mock.Anything, mock.AnythingOfType("string")).Return(nil).Maybe()
	historyRepo.On("MarkFailed", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(nil).Maybe()
	historyRepo.On("UpdateCounts", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("int32"), mock.AnythingOfType("int32")).Return(nil).Maybe()
	historyRepo.On("IncrementSuccessCount", mock.Anything, mock.AnythingOfType("string")).Return(nil).Maybe()
	historyRepo.On("IncrementErrorCount", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(nil).Maybe()
}

func ptr[T any](v T) *T {
	return &v
}

// =============================================================================
// DiagnoseAutomation Tests
// =============================================================================

func TestDiagnoseAutomation_Success(t *testing.T) {
	svc, deps := newTestService(t)
	ctx := context.Background()

	searchType := "search"
	automation := &models.Automation{
		ID:       "auto-123",
		UserID:   "user-1",
		Name:     "Test Automation",
		Status:   models.AutomationStatusActive,
		Type:     &searchType,
		GroupIDs: []string{"group1@g.us", "group2@g.us"},
		Stores:   []string{"shopee"},
	}

	connectedInstances := []models.Instance{
		{ID: 1, InstanceID: "instance-1", UserID: "user-1", Status: "connected"},
	}

	deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "auto-123", "user-1").Return(automation, nil)
	deps.instanceRepo.On("GetUserInstances", ctx, "user-1").Return(connectedInstances, nil)
	deps.shopeeService.On("GetCategories", ctx, (*int64)(nil)).Return([]models.ShopeeCategory{}, nil)

	result, err := svc.DiagnoseAutomation(ctx, "auto-123", "user-1")

	require.NoError(t, err)
	assert.Equal(t, "ok", result.Status)
	assert.GreaterOrEqual(t, len(result.Checks), 4)
	deps.automationRepo.AssertExpectations(t)
	deps.instanceRepo.AssertExpectations(t)
}

func TestDiagnoseAutomation_NotFound(t *testing.T) {
	svc, deps := newTestService(t)
	ctx := context.Background()

	deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "nonexistent", "user-1").Return(nil, nil)

	result, err := svc.DiagnoseAutomation(ctx, "nonexistent", "user-1")

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not found")
}

func TestDiagnoseAutomation_NoConnectedInstance(t *testing.T) {
	svc, deps := newTestService(t)
	ctx := context.Background()

	searchType := "search"
	automation := &models.Automation{
		ID:       "auto-123",
		UserID:   "user-1",
		Name:     "Test Automation",
		Status:   models.AutomationStatusActive,
		Type:     &searchType,
		GroupIDs: []string{"group1@g.us"},
		Stores:   []string{"shopee"},
	}

	disconnectedInstances := []models.Instance{
		{ID: 1, InstanceID: "instance-1", UserID: "user-1", Status: "disconnected"},
	}

	deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "auto-123", "user-1").Return(automation, nil)
	deps.instanceRepo.On("GetUserInstances", ctx, "user-1").Return(disconnectedInstances, nil)
	deps.shopeeService.On("GetCategories", ctx, (*int64)(nil)).Return([]models.ShopeeCategory{}, nil)

	result, err := svc.DiagnoseAutomation(ctx, "auto-123", "user-1")

	require.NoError(t, err)
	assert.Equal(t, "error", result.Status)

	// Find the WHATSAPP check
	var whatsappCheck *models.DiagnoseCheck
	for _, c := range result.Checks {
		if c.Name == "WHATSAPP" {
			whatsappCheck = &c
			break
		}
	}
	require.NotNil(t, whatsappCheck)
	assert.Equal(t, "error", whatsappCheck.Status)
}

func TestDiagnoseAutomation_NoTargetGroups(t *testing.T) {
	svc, deps := newTestService(t)
	ctx := context.Background()

	searchType := "search"
	automation := &models.Automation{
		ID:       "auto-123",
		UserID:   "user-1",
		Name:     "Test Automation",
		Status:   models.AutomationStatusActive,
		Type:     &searchType,
		GroupIDs: []string{}, // No groups
		Stores:   []string{"shopee"},
	}

	connectedInstances := []models.Instance{
		{ID: 1, InstanceID: "instance-1", UserID: "user-1", Status: "connected"},
	}

	deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "auto-123", "user-1").Return(automation, nil)
	deps.instanceRepo.On("GetUserInstances", ctx, "user-1").Return(connectedInstances, nil)
	deps.shopeeService.On("GetCategories", ctx, (*int64)(nil)).Return([]models.ShopeeCategory{}, nil)

	result, err := svc.DiagnoseAutomation(ctx, "auto-123", "user-1")

	require.NoError(t, err)
	assert.Contains(t, []string{"warning", "error"}, result.Status)
	assert.NotEmpty(t, result.Warnings)
}

func TestDiagnoseAutomation_InactiveAutomation(t *testing.T) {
	svc, deps := newTestService(t)
	ctx := context.Background()

	searchType := "search"
	automation := &models.Automation{
		ID:       "auto-123",
		UserID:   "user-1",
		Name:     "Test Automation",
		Status:   models.AutomationStatusInactive, // Inactive
		Type:     &searchType,
		GroupIDs: []string{"group1@g.us"},
		Stores:   []string{"shopee"},
	}

	connectedInstances := []models.Instance{
		{ID: 1, InstanceID: "instance-1", UserID: "user-1", Status: "connected"},
	}

	deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "auto-123", "user-1").Return(automation, nil)
	deps.instanceRepo.On("GetUserInstances", ctx, "user-1").Return(connectedInstances, nil)
	deps.shopeeService.On("GetCategories", ctx, (*int64)(nil)).Return([]models.ShopeeCategory{}, nil)

	result, err := svc.DiagnoseAutomation(ctx, "auto-123", "user-1")

	require.NoError(t, err)

	// Find the CONFIG check
	var configCheck *models.DiagnoseCheck
	for _, c := range result.Checks {
		if c.Name == "CONFIG" {
			configCheck = &c
			break
		}
	}
	require.NotNil(t, configCheck)
	assert.Equal(t, "warning", configCheck.Status)
}

// =============================================================================
// CanCreateAutomation Tests
// =============================================================================

func TestCanCreateAutomation_Success_CanCreate(t *testing.T) {
	svc, deps := newTestService(t)
	ctx := context.Background()

	deps.profileRepo.On("GetMaxAutomations", ctx, "user-1").Return(10, nil)
	deps.profileRepo.On("CountUserAutomations", ctx, "user-1").Return(5, nil)

	canCreate, currentCount, maxCount, err := svc.CanCreateAutomation(ctx, "user-1")

	require.NoError(t, err)
	assert.True(t, canCreate)
	assert.Equal(t, 5, currentCount)
	assert.Equal(t, 10, maxCount)
}

func TestCanCreateAutomation_AtLimit(t *testing.T) {
	svc, deps := newTestService(t)
	ctx := context.Background()

	deps.profileRepo.On("GetMaxAutomations", ctx, "user-1").Return(5, nil)
	deps.profileRepo.On("CountUserAutomations", ctx, "user-1").Return(5, nil)

	canCreate, currentCount, maxCount, err := svc.CanCreateAutomation(ctx, "user-1")

	require.NoError(t, err)
	assert.False(t, canCreate)
	assert.Equal(t, 5, currentCount)
	assert.Equal(t, 5, maxCount)
}

func TestCanCreateAutomation_OverLimit(t *testing.T) {
	svc, deps := newTestService(t)
	ctx := context.Background()

	deps.profileRepo.On("GetMaxAutomations", ctx, "user-1").Return(5, nil)
	deps.profileRepo.On("CountUserAutomations", ctx, "user-1").Return(6, nil)

	canCreate, currentCount, maxCount, err := svc.CanCreateAutomation(ctx, "user-1")

	require.NoError(t, err)
	assert.False(t, canCreate)
	assert.Equal(t, 6, currentCount)
	assert.Equal(t, 5, maxCount)
}

func TestCanCreateAutomation_ProfileRepoError(t *testing.T) {
	svc, deps := newTestService(t)
	ctx := context.Background()

	deps.profileRepo.On("GetMaxAutomations", ctx, "user-1").Return(5, nil)
	deps.profileRepo.On("CountUserAutomations", ctx, "user-1").Return(0, errors.New("db error"))

	canCreate, _, _, err := svc.CanCreateAutomation(ctx, "user-1")

	assert.Error(t, err)
	assert.False(t, canCreate)
}

// =============================================================================
// ResetHistory Tests
// =============================================================================

func TestResetHistory_Success(t *testing.T) {
	svc, deps := newTestService(t)
	ctx := context.Background()

	searchType := "search"
	automation := &models.Automation{
		ID:     "auto-123",
		UserID: "user-1",
		Name:   "Test Automation",
		Status: models.AutomationStatusActive,
		Type:   &searchType,
	}

	deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "auto-123", "user-1").Return(automation, nil)
	// ResetHistory calls DeleteRecentByAutomation with 1 day window (24h, Node.js parity)
	deps.dispatchLogRepo.On("DeleteRecentByAutomation", ctx, "auto-123", 1).Return(int64(5), nil)

	err := svc.ResetHistory(ctx, "auto-123", "user-1")

	require.NoError(t, err)
	deps.dispatchLogRepo.AssertExpectations(t)
}

func TestResetHistory_NotFound(t *testing.T) {
	svc, deps := newTestService(t)
	ctx := context.Background()

	deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "nonexistent", "user-1").Return(nil, nil)

	err := svc.ResetHistory(ctx, "nonexistent", "user-1")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// =============================================================================
// RunAutomation Tests (General)
// =============================================================================

func TestRunAutomation_NotFound(t *testing.T) {
	svc, deps := newTestService(t)
	ctx := context.Background()

	deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "nonexistent", "user-1").Return(nil, nil)

	result, err := svc.RunAutomation(ctx, "nonexistent", "user-1")

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not found")
}

func TestRunAutomation_RepositoryError(t *testing.T) {
	svc, deps := newTestService(t)
	ctx := context.Background()

	deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "auto-123", "user-1").Return(nil, errors.New("db error"))

	result, err := svc.RunAutomation(ctx, "auto-123", "user-1")

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to get automation")
}

// =============================================================================
// Table-Driven Tests for Automation Types
// =============================================================================

// =============================================================================
// DiagnoseAutomation - Additional Gap Coverage Tests
// =============================================================================

func TestDiagnoseAutomation_RepoError(t *testing.T) {
	t.Parallel()
	svc, deps := newTestService(t)
	ctx := context.Background()

	deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "auto-123", "user-1").Return(nil, errors.New("db error"))

	result, err := svc.DiagnoseAutomation(ctx, "auto-123", "user-1")
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to get automation")
}

func TestDiagnoseAutomation_ListTypeWithValidList(t *testing.T) {
	t.Parallel()
	svc, deps := newTestService(t)
	ctx := context.Background()

	listType := "list"
	listID := "list-123"
	automation := &models.Automation{
		ID:           "auto-123",
		UserID:       "user-1",
		Name:         "List Automation",
		Status:       models.AutomationStatusActive,
		Type:         &listType,
		GroupIDs:     []string{"group1@g.us"},
		MlLinkListID: &listID,
		Stores:       []string{"mercadolivre"},
	}

	connectedInstances := []models.Instance{
		{ID: 1, InstanceID: "instance-1", UserID: "user-1", Status: "connected"},
	}

	deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "auto-123", "user-1").Return(automation, nil)
	deps.instanceRepo.On("GetUserInstances", ctx, "user-1").Return(connectedInstances, nil)
	deps.mlLinkListItemRepo.On("GetNextUnconsumed", ctx, "list-123", 1).Return([]*models.MlLinkListItem{
		{ID: "item-1", OriginURL: "https://produto.mercadolivre.com.br/MLB-123"},
	}, nil)

	result, err := svc.DiagnoseAutomation(ctx, "auto-123", "user-1")
	require.NoError(t, err)
	assert.Equal(t, "ok", result.Status)

	// Verify ML_LINK_LIST check is present
	var mlCheck *models.DiagnoseCheck
	for _, c := range result.Checks {
		if c.Name == "ML_LINK_LIST" {
			mlCheck = &c
			break
		}
	}
	require.NotNil(t, mlCheck)
	assert.Equal(t, "ok", mlCheck.Status)
	assert.Contains(t, mlCheck.Message, "itens disponiveis")
}

func TestDiagnoseAutomation_ListTypeWithEmptyList(t *testing.T) {
	t.Parallel()
	svc, deps := newTestService(t)
	ctx := context.Background()

	listType := "list"
	listID := "list-123"
	automation := &models.Automation{
		ID:           "auto-123",
		UserID:       "user-1",
		Name:         "List Automation",
		Status:       models.AutomationStatusActive,
		Type:         &listType,
		GroupIDs:     []string{"group1@g.us"},
		MlLinkListID: &listID,
	}

	connectedInstances := []models.Instance{
		{ID: 1, InstanceID: "instance-1", UserID: "user-1", Status: "connected"},
	}

	deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "auto-123", "user-1").Return(automation, nil)
	deps.instanceRepo.On("GetUserInstances", ctx, "user-1").Return(connectedInstances, nil)
	deps.mlLinkListItemRepo.On("GetNextUnconsumed", ctx, "list-123", 1).Return([]*models.MlLinkListItem{}, nil)

	result, err := svc.DiagnoseAutomation(ctx, "auto-123", "user-1")
	require.NoError(t, err)

	var mlCheck *models.DiagnoseCheck
	for _, c := range result.Checks {
		if c.Name == "ML_LINK_LIST" {
			mlCheck = &c
			break
		}
	}
	require.NotNil(t, mlCheck)
	assert.Equal(t, "warning", mlCheck.Status)
	assert.Contains(t, mlCheck.Message, "sem itens")
}

func TestDiagnoseAutomation_ListTypeWithListError(t *testing.T) {
	t.Parallel()
	svc, deps := newTestService(t)
	ctx := context.Background()

	listType := "list"
	listID := "list-123"
	automation := &models.Automation{
		ID:           "auto-123",
		UserID:       "user-1",
		Name:         "List Automation",
		Status:       models.AutomationStatusActive,
		Type:         &listType,
		GroupIDs:     []string{"group1@g.us"},
		MlLinkListID: &listID,
	}

	connectedInstances := []models.Instance{
		{ID: 1, InstanceID: "instance-1", UserID: "user-1", Status: "connected"},
	}

	deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "auto-123", "user-1").Return(automation, nil)
	deps.instanceRepo.On("GetUserInstances", ctx, "user-1").Return(connectedInstances, nil)
	deps.mlLinkListItemRepo.On("GetNextUnconsumed", ctx, "list-123", 1).Return(nil, errors.New("db error"))

	result, err := svc.DiagnoseAutomation(ctx, "auto-123", "user-1")
	require.NoError(t, err)

	var mlCheck *models.DiagnoseCheck
	for _, c := range result.Checks {
		if c.Name == "ML_LINK_LIST" {
			mlCheck = &c
			break
		}
	}
	require.NotNil(t, mlCheck)
	assert.Equal(t, "warning", mlCheck.Status)
	assert.Contains(t, mlCheck.Message, "Erro")
}

func TestDiagnoseAutomation_ShopeeAPIError(t *testing.T) {
	t.Parallel()
	svc, deps := newTestService(t)
	ctx := context.Background()

	searchType := "search"
	automation := &models.Automation{
		ID:       "auto-123",
		UserID:   "user-1",
		Name:     "Test Automation",
		Status:   models.AutomationStatusActive,
		Type:     &searchType,
		GroupIDs: []string{"group1@g.us"},
		Stores:   []string{"shopee"},
	}

	connectedInstances := []models.Instance{
		{ID: 1, InstanceID: "instance-1", UserID: "user-1", Status: "connected"},
	}

	deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "auto-123", "user-1").Return(automation, nil)
	deps.instanceRepo.On("GetUserInstances", ctx, "user-1").Return(connectedInstances, nil)
	deps.shopeeService.On("GetCategories", ctx, (*int64)(nil)).Return(nil, errors.New("API timeout"))

	result, err := svc.DiagnoseAutomation(ctx, "auto-123", "user-1")
	require.NoError(t, err)

	var buscaCheck *models.DiagnoseCheck
	for _, c := range result.Checks {
		if c.Name == "BUSCA_STORE" {
			buscaCheck = &c
			break
		}
	}
	require.NotNil(t, buscaCheck)
	assert.Equal(t, "error", buscaCheck.Status)
	assert.Contains(t, buscaCheck.Message, "Shopee")
}

func TestDiagnoseAutomation_NoCacheConfigured(t *testing.T) {
	t.Parallel()
	svc, deps := newTestService(t)
	ctx := context.Background()

	searchType := "search"
	automation := &models.Automation{
		ID:       "auto-123",
		UserID:   "user-1",
		Name:     "Test Automation",
		Status:   models.AutomationStatusActive,
		Type:     &searchType,
		GroupIDs: []string{"group1@g.us"},
		Stores:   []string{"shopee"},
	}

	connectedInstances := []models.Instance{
		{ID: 1, InstanceID: "instance-1", UserID: "user-1", Status: "connected"},
	}

	deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "auto-123", "user-1").Return(automation, nil)
	deps.instanceRepo.On("GetUserInstances", ctx, "user-1").Return(connectedInstances, nil)
	deps.shopeeService.On("GetCategories", ctx, (*int64)(nil)).Return([]models.ShopeeCategory{}, nil)

	result, err := svc.DiagnoseAutomation(ctx, "auto-123", "user-1")
	require.NoError(t, err)

	// Cache is nil in newTestService, so ANTIDUP should be warning
	var antidupCheck *models.DiagnoseCheck
	for _, c := range result.Checks {
		if c.Name == "ANTIDUP" {
			antidupCheck = &c
			break
		}
	}
	require.NotNil(t, antidupCheck)
	assert.Equal(t, "warning", antidupCheck.Status)
	assert.Contains(t, antidupCheck.Message, "Cache nao configurado")
}

func TestDiagnoseAutomation_NoInstances(t *testing.T) {
	t.Parallel()
	svc, deps := newTestService(t)
	ctx := context.Background()

	searchType := "search"
	automation := &models.Automation{
		ID:       "auto-123",
		UserID:   "user-1",
		Name:     "Test Automation",
		Status:   models.AutomationStatusActive,
		Type:     &searchType,
		GroupIDs: []string{"group1@g.us"},
		Stores:   []string{"shopee"},
	}

	deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "auto-123", "user-1").Return(automation, nil)
	deps.instanceRepo.On("GetUserInstances", ctx, "user-1").Return(nil, errors.New("db error"))
	deps.shopeeService.On("GetCategories", ctx, (*int64)(nil)).Return([]models.ShopeeCategory{}, nil)

	result, err := svc.DiagnoseAutomation(ctx, "auto-123", "user-1")
	require.NoError(t, err)
	assert.Equal(t, "error", result.Status)
}

// =============================================================================
// RunAutomation - Additional Gap Coverage Tests
// =============================================================================

func TestRunAutomation_DefaultType(t *testing.T) {
	t.Parallel()
	svc, deps := newTestService(t)
	ctx := context.Background()

	// No Type set → defaults to search via default case
	automation := &models.Automation{
		ID:       "auto-123",
		UserID:   "user-1",
		Name:     "Default Type Automation",
		Status:   models.AutomationStatusActive,
		Type:     nil, // No type → default case
		GroupIDs: []string{"group1@g.us"},
		Stores:   []string{"shopee"},
	}

	connectedInstances := []models.Instance{
		{ID: 1, InstanceID: "instance-1", UserID: "user-1", Status: "connected"},
	}

	lockTime := time.Now().Add(10 * time.Minute)

	deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "auto-123", "user-1").Return(automation, nil)
	deps.automationRunRepo.On("GetLock", ctx, "auto-123").Return(nil, nil)
	deps.automationRunRepo.On("AcquireLock", ctx, "auto-123", mock.AnythingOfType("time.Duration")).Return(&lockTime, nil)
	deps.automationRunRepo.On("ReleaseLock", ctx, "auto-123").Return(nil)
	deps.historyRepo.On("Create", ctx, "user-1", "automation_run", mock.AnythingOfType("string"), mock.Anything).Return(&models.ActionHistory{ID: "history-1"}, nil)
	deps.historyRepo.On("Update", ctx, "history-1", mock.Anything).Return(nil)
	deps.instanceRepo.On("GetUserInstances", ctx, "user-1").Return(connectedInstances, nil)
	deps.shopeeService.On("SearchOffers", ctx, "user-1", mock.AnythingOfType("models.ShopeeSearchParams")).Return(&models.ShopeeSearchResult{Offers: []models.ShopeeOffer{}}, nil)
	deps.automationRepo.On("UpdateLastRunAt", ctx, "auto-123").Return(nil)
	deps.automationRepo.On("UpdateNextRunAt", ctx, "auto-123", mock.AnythingOfType("time.Time")).Return(nil).Maybe()

	result, err := svc.RunAutomation(ctx, "auto-123", "user-1")
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestRunAutomation_HistoryCreateError(t *testing.T) {
	t.Parallel()
	svc, deps := newTestService(t)
	ctx := context.Background()

	searchType := "search"
	automation := &models.Automation{
		ID:       "auto-123",
		UserID:   "user-1",
		Name:     "Test Automation",
		Status:   models.AutomationStatusActive,
		Type:     &searchType,
		GroupIDs: []string{"group1@g.us"},
		Stores:   []string{"shopee"},
	}

	connectedInstances := []models.Instance{
		{ID: 1, InstanceID: "instance-1", UserID: "user-1", Status: "connected"},
	}

	lockTime := time.Now().Add(10 * time.Minute)

	deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "auto-123", "user-1").Return(automation, nil)
	deps.automationRunRepo.On("GetLock", ctx, "auto-123").Return(nil, nil)
	deps.automationRunRepo.On("AcquireLock", ctx, "auto-123", mock.AnythingOfType("time.Duration")).Return(&lockTime, nil)
	deps.automationRunRepo.On("ReleaseLock", ctx, "auto-123").Return(nil)
	// History create fails (fire-and-forget, should not stop execution)
	deps.historyRepo.On("Create", ctx, "user-1", "automation_run", mock.AnythingOfType("string"), mock.Anything).Return(nil, errors.New("db error"))
	deps.instanceRepo.On("GetUserInstances", ctx, "user-1").Return(connectedInstances, nil)
	deps.shopeeService.On("SearchOffers", ctx, "user-1", mock.AnythingOfType("models.ShopeeSearchParams")).Return(&models.ShopeeSearchResult{Offers: []models.ShopeeOffer{}}, nil)
	deps.automationRepo.On("UpdateLastRunAt", ctx, "auto-123").Return(nil)
	deps.automationRepo.On("UpdateNextRunAt", ctx, "auto-123", mock.AnythingOfType("time.Time")).Return(nil).Maybe()

	result, err := svc.RunAutomation(ctx, "auto-123", "user-1")
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestRunAutomation_WithIntervalUpdateNextRunAt(t *testing.T) {
	t.Parallel()
	svc, deps := newTestService(t)
	ctx := context.Background()

	searchType := "search"
	interval := 30
	automation := &models.Automation{
		ID:              "auto-123",
		UserID:          "user-1",
		Name:            "Test Automation",
		Status:          models.AutomationStatusActive,
		Type:            &searchType,
		GroupIDs:        []string{"group1@g.us"},
		Stores:          []string{"shopee"},
		IntervalMinutes: &interval,
	}

	connectedInstances := []models.Instance{
		{ID: 1, InstanceID: "instance-1", UserID: "user-1", Status: "connected"},
	}

	lockTime := time.Now().Add(10 * time.Minute)

	deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "auto-123", "user-1").Return(automation, nil)
	deps.automationRunRepo.On("GetLock", ctx, "auto-123").Return(nil, nil)
	deps.automationRunRepo.On("AcquireLock", ctx, "auto-123", mock.AnythingOfType("time.Duration")).Return(&lockTime, nil)
	deps.automationRunRepo.On("ReleaseLock", ctx, "auto-123").Return(nil)
	deps.historyRepo.On("Create", ctx, "user-1", "automation_run", mock.AnythingOfType("string"), mock.Anything).Return(&models.ActionHistory{ID: "history-1"}, nil)
	deps.historyRepo.On("Update", ctx, "history-1", mock.Anything).Return(nil)
	deps.instanceRepo.On("GetUserInstances", ctx, "user-1").Return(connectedInstances, nil)
	deps.shopeeService.On("SearchOffers", ctx, "user-1", mock.AnythingOfType("models.ShopeeSearchParams")).Return(&models.ShopeeSearchResult{Offers: []models.ShopeeOffer{}}, nil)
	deps.automationRepo.On("UpdateLastRunAt", ctx, "auto-123").Return(nil)
	// The defer should call UpdateNextRunAt with the interval
	deps.automationRepo.On("UpdateNextRunAt", ctx, "auto-123", mock.AnythingOfType("time.Time")).Return(nil)

	result, err := svc.RunAutomation(ctx, "auto-123", "user-1")
	require.NoError(t, err)
	assert.NotNil(t, result)
	deps.automationRepo.AssertCalled(t, "UpdateNextRunAt", ctx, "auto-123", mock.AnythingOfType("time.Time"))
}

func TestRunAutomation_HistoryUpdateErrors(t *testing.T) {
	t.Parallel()
	svc, deps := newTestService(t)
	ctx := context.Background()

	searchType := "search"
	automation := &models.Automation{
		ID:       "auto-123",
		UserID:   "user-1",
		Name:     "Test Automation",
		Status:   models.AutomationStatusActive,
		Type:     &searchType,
		GroupIDs: []string{"group1@g.us"},
		Stores:   []string{"shopee"},
	}

	connectedInstances := []models.Instance{
		{ID: 1, InstanceID: "instance-1", UserID: "user-1", Status: "connected"},
	}

	lockTime := time.Now().Add(10 * time.Minute)

	deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "auto-123", "user-1").Return(automation, nil)
	deps.automationRunRepo.On("GetLock", ctx, "auto-123").Return(nil, nil)
	deps.automationRunRepo.On("AcquireLock", ctx, "auto-123", mock.AnythingOfType("time.Duration")).Return(&lockTime, nil)
	deps.automationRunRepo.On("ReleaseLock", ctx, "auto-123").Return(nil)
	deps.historyRepo.On("Create", ctx, "user-1", "automation_run", mock.AnythingOfType("string"), mock.Anything).Return(&models.ActionHistory{ID: "history-1"}, nil)
	// History Update fails (fire-and-forget)
	deps.historyRepo.On("Update", ctx, "history-1", mock.Anything).Return(errors.New("update error"))
	deps.instanceRepo.On("GetUserInstances", ctx, "user-1").Return(connectedInstances, nil)
	deps.shopeeService.On("SearchOffers", ctx, "user-1", mock.AnythingOfType("models.ShopeeSearchParams")).Return(&models.ShopeeSearchResult{Offers: []models.ShopeeOffer{}}, nil)
	deps.automationRepo.On("UpdateLastRunAt", ctx, "auto-123").Return(nil)
	deps.automationRepo.On("UpdateNextRunAt", ctx, "auto-123", mock.AnythingOfType("time.Time")).Return(nil).Maybe()

	// Should still succeed despite history update errors
	result, err := svc.RunAutomation(ctx, "auto-123", "user-1")
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestRunAutomation_MonitoringType(t *testing.T) {
	t.Parallel()
	svc, deps := newTestService(t)
	ctx := context.Background()

	monitorType := "monitoring"
	automation := &models.Automation{
		ID:              "auto-123",
		UserID:          "user-1",
		Name:            "Monitor Automation",
		Status:          models.AutomationStatusActive,
		Type:            &monitorType,
		GroupIDs:        []string{"target@g.us"},
		MonitorGroupIDs: []string{"source@g.us"},
		Stores:          []string{"shopee"},
	}

	lockTime := time.Now().Add(10 * time.Minute)

	deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "auto-123", "user-1").Return(automation, nil)
	deps.automationRunRepo.On("GetLock", ctx, "auto-123").Return(nil, nil)
	deps.automationRunRepo.On("AcquireLock", ctx, "auto-123", mock.AnythingOfType("time.Duration")).Return(&lockTime, nil)
	deps.automationRunRepo.On("ReleaseLock", ctx, "auto-123").Return(nil)
	deps.historyRepo.On("Create", ctx, "user-1", "automation_run", mock.AnythingOfType("string"), mock.Anything).Return(&models.ActionHistory{ID: "history-1"}, nil)
	deps.historyRepo.On("Update", ctx, "history-1", mock.Anything).Return(nil)
	deps.instanceRepo.On("GetUserInstances", ctx, "user-1").Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)
	deps.automationRepo.On("UpdateLastRunAt", ctx, "auto-123").Return(nil)
	deps.automationRepo.On("UpdateNextRunAt", ctx, "auto-123", mock.AnythingOfType("time.Time")).Return(nil).Maybe()
	deps.dispatchLogRepo.On("Create", mock.Anything, mock.Anything).Return(&models.DispatchLog{}, nil).Maybe()

	result, err := svc.RunAutomation(ctx, "auto-123", "user-1")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success)
	assert.Contains(t, result.Message, "Monitoramento ativo")
}

func TestRunAutomation_ListType(t *testing.T) {
	t.Parallel()
	svc, deps := newTestService(t)
	ctx := context.Background()

	listType := "list"
	listID := "list-123"
	automation := &models.Automation{
		ID:           "auto-123",
		UserID:       "user-1",
		Name:         "List Automation",
		Status:       models.AutomationStatusActive,
		Type:         &listType,
		GroupIDs:     []string{"target@g.us"},
		MlLinkListID: &listID,
	}

	lockTime := time.Now().Add(10 * time.Minute)

	deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "auto-123", "user-1").Return(automation, nil)
	deps.automationRunRepo.On("GetLock", ctx, "auto-123").Return(nil, nil)
	deps.automationRunRepo.On("AcquireLock", ctx, "auto-123", mock.AnythingOfType("time.Duration")).Return(&lockTime, nil)
	deps.automationRunRepo.On("ReleaseLock", ctx, "auto-123").Return(nil)
	deps.historyRepo.On("Create", ctx, "user-1", "automation_run", mock.AnythingOfType("string"), mock.Anything).Return(&models.ActionHistory{ID: "history-1"}, nil)
	deps.historyRepo.On("Update", ctx, "history-1", mock.Anything).Return(nil)
	deps.instanceRepo.On("GetUserInstances", ctx, "user-1").Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)
	deps.mlLinkListItemRepo.On("GetNextUnconsumed", ctx, "list-123", 1).Return([]*models.MlLinkListItem{}, nil)
	deps.automationRepo.On("UpdateLastRunAt", ctx, "auto-123").Return(nil)
	deps.automationRepo.On("UpdateNextRunAt", ctx, "auto-123", mock.AnythingOfType("time.Time")).Return(nil).Maybe()
	deps.dispatchLogRepo.On("Create", mock.Anything, mock.Anything).Return(&models.DispatchLog{}, nil).Maybe()

	result, err := svc.RunAutomation(ctx, "auto-123", "user-1")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success)
}

func TestRunAutomation_ExecutionErrorUpdatesHistory(t *testing.T) {
	t.Parallel()
	svc, deps := newTestService(t)
	ctx := context.Background()

	searchType := "search"
	automation := &models.Automation{
		ID:       "auto-123",
		UserID:   "user-1",
		Name:     "Test Automation",
		Status:   models.AutomationStatusActive,
		Type:     &searchType,
		GroupIDs: []string{}, // No groups → executeSearchAutomation returns error
		Stores:   []string{"shopee"},
	}

	lockTime := time.Now().Add(10 * time.Minute)

	deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "auto-123", "user-1").Return(automation, nil)
	deps.automationRunRepo.On("GetLock", ctx, "auto-123").Return(nil, nil)
	deps.automationRunRepo.On("AcquireLock", ctx, "auto-123", mock.AnythingOfType("time.Duration")).Return(&lockTime, nil)
	deps.automationRunRepo.On("ReleaseLock", ctx, "auto-123").Return(nil)
	deps.historyRepo.On("Create", ctx, "user-1", "automation_run", mock.AnythingOfType("string"), mock.Anything).Return(&models.ActionHistory{ID: "history-1"}, nil)
	deps.historyRepo.On("Update", ctx, "history-1", mock.Anything).Return(nil)
	deps.instanceRepo.On("GetUserInstances", ctx, "user-1").Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)
	deps.automationRepo.On("UpdateNextRunAt", ctx, "auto-123", mock.AnythingOfType("time.Time")).Return(nil).Maybe()

	result, err := svc.RunAutomation(ctx, "auto-123", "user-1")
	require.NoError(t, err) // RunAutomation wraps inner error into result
	require.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Contains(t, result.Message, "no target groups")
}

func TestRunAutomation_LockConflict(t *testing.T) {
	t.Parallel()
	svc, deps := newTestService(t)
	ctx := context.Background()

	searchType := "search"
	automation := &models.Automation{
		ID:     "auto-123",
		UserID: "user-1",
		Name:   "Test Automation",
		Type:   &searchType,
	}

	existingLock := time.Now().Add(5 * time.Minute)

	deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "auto-123", "user-1").Return(automation, nil)
	deps.automationRunRepo.On("GetLock", ctx, "auto-123").Return(&existingLock, nil)

	result, err := svc.RunAutomation(ctx, "auto-123", "user-1")
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "lock")
}

func TestRunAutomation_LockCheckError(t *testing.T) {
	t.Parallel()
	svc, deps := newTestService(t)
	ctx := context.Background()

	searchType := "search"
	automation := &models.Automation{
		ID:     "auto-123",
		UserID: "user-1",
		Name:   "Test Automation",
		Type:   &searchType,
	}

	deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "auto-123", "user-1").Return(automation, nil)
	deps.automationRunRepo.On("GetLock", ctx, "auto-123").Return(nil, errors.New("db error"))

	result, err := svc.RunAutomation(ctx, "auto-123", "user-1")
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to acquire lock")
}

func TestCanCreateAutomation_GetMaxError(t *testing.T) {
	t.Parallel()
	svc, deps := newTestService(t)
	ctx := context.Background()

	// GetMaxAutomations errors → uses default of 5
	deps.profileRepo.On("GetMaxAutomations", ctx, "user-1").Return(0, errors.New("db error"))
	deps.profileRepo.On("CountUserAutomations", ctx, "user-1").Return(3, nil)

	canCreate, currentCount, maxCount, err := svc.CanCreateAutomation(ctx, "user-1")
	require.NoError(t, err)
	assert.True(t, canCreate)
	assert.Equal(t, 3, currentCount)
	assert.Equal(t, 5, maxCount) // default
}

func TestResetHistory_DeleteError(t *testing.T) {
	t.Parallel()
	svc, deps := newTestService(t)
	ctx := context.Background()

	searchType := "search"
	automation := &models.Automation{
		ID:     "auto-123",
		UserID: "user-1",
		Name:   "Test Automation",
		Type:   &searchType,
	}

	deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "auto-123", "user-1").Return(automation, nil)
	deps.dispatchLogRepo.On("DeleteRecentByAutomation", ctx, "auto-123", 1).Return(int64(0), errors.New("db error"))

	err := svc.ResetHistory(ctx, "auto-123", "user-1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to reset history")
}

func TestDiagnoseAutomation_WithTimeWindow(t *testing.T) {
	t.Parallel()
	svc, deps := newTestService(t)
	ctx := context.Background()

	searchType := "search"
	startTime := "03:00"
	endTime := "03:01"
	scheduleDays := []string{"0", "1", "2", "3", "4", "5", "6"}
	automation := &models.Automation{
		ID:           "auto-123",
		UserID:       "user-1",
		Name:         "Test Automation",
		Status:       models.AutomationStatusActive,
		Type:         &searchType,
		GroupIDs:     []string{"group1@g.us"},
		Stores:       []string{"shopee"},
		StartTime:    &startTime,
		EndTime:      &endTime,
		ScheduleDays: scheduleDays,
	}

	deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "auto-123", "user-1").Return(automation, nil)
	deps.instanceRepo.On("GetUserInstances", ctx, "user-1").Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)
	deps.shopeeService.On("GetCategories", ctx, (*int64)(nil)).Return([]models.ShopeeCategory{}, nil)

	result, err := svc.DiagnoseAutomation(ctx, "auto-123", "user-1")
	require.NoError(t, err)
	// Time window is 03:00-03:01, almost certainly outside current time
	// so we should see a warning about being outside the time window
	assert.NotNil(t, result)
}

func TestDiagnoseAutomation_MonitoringTypeStoreCheck(t *testing.T) {
	t.Parallel()
	svc, deps := newTestService(t)
	ctx := context.Background()

	monitorType := "monitoring"
	automation := &models.Automation{
		ID:              "auto-123",
		UserID:          "user-1",
		Name:            "Monitor Automation",
		Status:          models.AutomationStatusActive,
		Type:            &monitorType,
		GroupIDs:        []string{"target@g.us"},
		MonitorGroupIDs: []string{"source@g.us"},
	}

	deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "auto-123", "user-1").Return(automation, nil)
	deps.instanceRepo.On("GetUserInstances", ctx, "user-1").Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)

	result, err := svc.DiagnoseAutomation(ctx, "auto-123", "user-1")
	require.NoError(t, err)

	// For non-search types, BUSCA_STORE should be "ok" with "nao necessaria" message
	var buscaCheck *models.DiagnoseCheck
	for _, c := range result.Checks {
		if c.Name == "BUSCA_STORE" {
			buscaCheck = &c
			break
		}
	}
	require.NotNil(t, buscaCheck)
	assert.Equal(t, "ok", buscaCheck.Status)
	assert.Contains(t, buscaCheck.Message, "nao necessaria")
}

func TestDiagnoseAutomation_ListTypeNoListID(t *testing.T) {
	t.Parallel()
	svc, deps := newTestService(t)
	ctx := context.Background()

	listType := "list"
	automation := &models.Automation{
		ID:           "auto-123",
		UserID:       "user-1",
		Name:         "List Automation",
		Status:       models.AutomationStatusActive,
		Type:         &listType,
		GroupIDs:     []string{"group1@g.us"},
		MlLinkListID: nil, // No list ID
	}

	deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "auto-123", "user-1").Return(automation, nil)
	deps.instanceRepo.On("GetUserInstances", ctx, "user-1").Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)

	result, err := svc.DiagnoseAutomation(ctx, "auto-123", "user-1")
	require.NoError(t, err)
	assert.Equal(t, "error", result.Status)

	var mlCheck *models.DiagnoseCheck
	for _, c := range result.Checks {
		if c.Name == "ML_LINK_LIST" {
			mlCheck = &c
			break
		}
	}
	require.NotNil(t, mlCheck)
	assert.Equal(t, "error", mlCheck.Status)
	assert.Contains(t, mlCheck.Message, "Lista de links nao configurada")
}

func TestDiagnoseAutomation_EmptyStoresDefaultsToShopee(t *testing.T) {
	t.Parallel()
	svc, deps := newTestService(t)
	ctx := context.Background()

	searchType := "search"
	automation := &models.Automation{
		ID:       "auto-123",
		UserID:   "user-1",
		Name:     "Test Automation",
		Status:   models.AutomationStatusActive,
		Type:     &searchType,
		GroupIDs: []string{"group1@g.us"},
		Stores:   []string{}, // Empty → defaults to shopee
	}

	deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "auto-123", "user-1").Return(automation, nil)
	deps.instanceRepo.On("GetUserInstances", ctx, "user-1").Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)
	deps.shopeeService.On("GetCategories", ctx, (*int64)(nil)).Return([]models.ShopeeCategory{}, nil)

	result, err := svc.DiagnoseAutomation(ctx, "auto-123", "user-1")
	require.NoError(t, err)

	var buscaCheck *models.DiagnoseCheck
	for _, c := range result.Checks {
		if c.Name == "BUSCA_STORE" {
			buscaCheck = &c
			break
		}
	}
	require.NotNil(t, buscaCheck)
	assert.Equal(t, "ok", buscaCheck.Status)
	assert.Contains(t, buscaCheck.Message, "shopee")
}

func TestDiagnoseAutomation_AllTypes(t *testing.T) {
	tests := []struct {
		name            string
		automationType  string
		groupIDs        []string
		monitorGroupIDs []string
		expectStatus    string
	}{
		{
			name:           "search type with groups",
			automationType: "search",
			groupIDs:       []string{"group1@g.us"},
			expectStatus:   "ok",
		},
		{
			name:            "monitoring type with all groups",
			automationType:  "monitoring",
			groupIDs:        []string{"target1@g.us"},
			monitorGroupIDs: []string{"source1@g.us"},
			expectStatus:    "ok",
		},
		{
			name:           "list type with groups",
			automationType: "list",
			groupIDs:       []string{"group1@g.us"},
			expectStatus:   "error", // No MlLinkListID configured → error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, deps := newTestService(t)
			ctx := context.Background()

			automation := &models.Automation{
				ID:              "auto-123",
				UserID:          "user-1",
				Name:            "Test Automation",
				Status:          models.AutomationStatusActive,
				Type:            &tt.automationType,
				GroupIDs:        tt.groupIDs,
				MonitorGroupIDs: tt.monitorGroupIDs,
				Stores:          []string{"shopee"},
			}

			connectedInstances := []models.Instance{
				{ID: 1, InstanceID: "instance-1", UserID: "user-1", Status: "connected"},
			}

			deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "auto-123", "user-1").Return(automation, nil)
			deps.instanceRepo.On("GetUserInstances", ctx, "user-1").Return(connectedInstances, nil)

			if tt.automationType == "search" {
				deps.shopeeService.On("GetCategories", ctx, (*int64)(nil)).Return([]models.ShopeeCategory{}, nil)
			}

			result, err := svc.DiagnoseAutomation(ctx, "auto-123", "user-1")

			require.NoError(t, err)
			assert.Equal(t, tt.expectStatus, result.Status)
		})
	}
}

func TestNewAutomationService_WithNonNilDependencies(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)

	usSvc := urlsafety.NewService(urlsafety.Config{Mode: "off"}, nil, logger)
	bpSvc := bpLib.NewProducerBackpressure(bpLib.Config{Enabled: false}, nil, logger)

	svc := automation.NewAutomationService(
		nil, nil, nil, nil, nil, nil, nil, nil, nil,
		nil, // ctaRepo
		nil, nil,
		usSvc, bpSvc,
		nil, nil,
		nil, // couponRepo
		logger,
	)
	assert.NotNil(t, svc)
}
