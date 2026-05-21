package automation

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	bpLib "automatiza-go/internal/lib/backpressure"
	"automatiza-go/internal/lib/cache"
	"automatiza-go/internal/lib/urlsafety"
	"automatiza-go/internal/config"
	"automatiza-go/internal/models"
	"automatiza-go/internal/observability"
	"automatiza-go/internal/repository"
	"automatiza-go/internal/workers"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// =============================================================================
// Mock implementations for internal tests (different package from automation_test)
// =============================================================================

type mockAutomationRepo struct {
	mock.Mock
}

func (m *mockAutomationRepo) GetUserAutomations(ctx context.Context, userID string) ([]models.Automation, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Automation), args.Error(1)
}

func (m *mockAutomationRepo) GetActiveAutomations(ctx context.Context) ([]models.Automation, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Automation), args.Error(1)
}

func (m *mockAutomationRepo) GetAutomationByID(ctx context.Context, automationID string) (*models.Automation, error) {
	args := m.Called(ctx, automationID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Automation), args.Error(1)
}

func (m *mockAutomationRepo) GetAutomationByIDAndUser(ctx context.Context, automationID, userID string) (*models.Automation, error) {
	args := m.Called(ctx, automationID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Automation), args.Error(1)
}

func (m *mockAutomationRepo) CreateAutomation(ctx context.Context, userID string, data *models.Automation) (*models.Automation, error) {
	args := m.Called(ctx, userID, data)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Automation), args.Error(1)
}

func (m *mockAutomationRepo) UpdateAutomation(ctx context.Context, automationID string, updates map[string]interface{}) error {
	args := m.Called(ctx, automationID, updates)
	return args.Error(0)
}

func (m *mockAutomationRepo) DeleteAutomation(ctx context.Context, automationID, userID string) error {
	args := m.Called(ctx, automationID, userID)
	return args.Error(0)
}

func (m *mockAutomationRepo) GetMonitoringAutomationsForGroup(ctx context.Context, waGroupID string) ([]models.Automation, error) {
	args := m.Called(ctx, waGroupID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Automation), args.Error(1)
}

func (m *mockAutomationRepo) UpdateLastRunAt(ctx context.Context, automationID string) error {
	args := m.Called(ctx, automationID)
	return args.Error(0)
}

func (m *mockAutomationRepo) UpdateNextRunAt(ctx context.Context, automationID string, nextRun time.Time) error {
	args := m.Called(ctx, automationID, nextRun)
	return args.Error(0)
}

func (m *mockAutomationRepo) BulkUpdateStatusByUserID(ctx context.Context, userID, targetStatus string, sourceStatus *string) (int, error) {
	args := m.Called(ctx, userID, targetStatus, sourceStatus)
	return args.Int(0), args.Error(1)
}

func (m *mockAutomationRepo) BulkUpdateNextRunAts(ctx context.Context, automationIDs []string, nextRunAts []time.Time) error {
	args := m.Called(ctx, automationIDs, nextRunAts)
	return args.Error(0)
}

func (m *mockAutomationRepo) GetOfferGenerateAutomationsForGroup(ctx context.Context, waGroupID string) ([]models.Automation, error) {
	args := m.Called(ctx, waGroupID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Automation), args.Error(1)
}

type mockInstanceRepo struct {
	mock.Mock
}

func (m *mockInstanceRepo) GetUserInstances(ctx context.Context, userID string) ([]models.Instance, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Instance), args.Error(1)
}

func (m *mockInstanceRepo) GetInstanceByID(ctx context.Context, instanceID int) (*models.Instance, error) {
	args := m.Called(ctx, instanceID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Instance), args.Error(1)
}

func (m *mockInstanceRepo) GetInstanceByName(ctx context.Context, instanceName string) (*models.Instance, error) {
	args := m.Called(ctx, instanceName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Instance), args.Error(1)
}

func (m *mockInstanceRepo) GetInstanceByUserAndName(ctx context.Context, userID, instanceName string) (*models.Instance, error) {
	args := m.Called(ctx, userID, instanceName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Instance), args.Error(1)
}

func (m *mockInstanceRepo) GetAllConnectedInstances(ctx context.Context) ([]models.Instance, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Instance), args.Error(1)
}

func (m *mockInstanceRepo) UpsertInstance(ctx context.Context, userID string, data *models.Instance) (*models.Instance, error) {
	args := m.Called(ctx, userID, data)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Instance), args.Error(1)
}

func (m *mockInstanceRepo) UpdateInstanceStatus(ctx context.Context, instanceID int, status string) error {
	args := m.Called(ctx, instanceID, status)
	return args.Error(0)
}

func (m *mockInstanceRepo) DeleteInstance(ctx context.Context, instanceID int) error {
	args := m.Called(ctx, instanceID)
	return args.Error(0)
}

func (m *mockInstanceRepo) SetDefaultInstance(ctx context.Context, userID string, instanceID int) error {
	args := m.Called(ctx, userID, instanceID)
	return args.Error(0)
}

func (m *mockInstanceRepo) CountUserInstances(ctx context.Context, userID string) (int, error) {
	args := m.Called(ctx, userID)
	return args.Int(0), args.Error(1)
}

func (m *mockInstanceRepo) UpdateInstanceLabel(ctx context.Context, userID string, instanceID int, friendlyName string) error {
	args := m.Called(ctx, userID, instanceID, friendlyName)
	return args.Error(0)
}

func (m *mockInstanceRepo) UpdateInstancePhoneNumber(ctx context.Context, instanceID int, phoneNumber string) error {
	args := m.Called(ctx, instanceID, phoneNumber)
	return args.Error(0)
}

func (m *mockInstanceRepo) SupersedePriorInstances(ctx context.Context, userID string, currentInstanceID int, phoneNumber string) (int, error) {
	args := m.Called(ctx, userID, currentInstanceID, phoneNumber)
	return args.Int(0), args.Error(1)
}

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

func (m *mockDispatchLogRepo) GetRecentByCanonicalKey(ctx context.Context, userID, groupID, canonicalKey string, window time.Duration) (*models.DispatchLog, error) {
	args := m.Called(ctx, userID, groupID, canonicalKey, window)
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
		return nil, 0, args.Error(2)
	}
	return args.Get(0).([]repository.DispatchLogWithGroup), args.Int(1), args.Error(2)
}

func (m *mockDispatchLogRepo) GetUserLogsWithFilters(ctx context.Context, userID string, filters *models.DispatchLogFilters, limit, offset int) ([]repository.DispatchLogWithGroup, int64, error) {
	args := m.Called(ctx, userID, filters, limit, offset)
	if args.Get(0) == nil {
		return nil, 0, args.Error(2)
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
		return nil, false, args.Error(2)
	}
	return args.Get(0).(*models.DispatchLog), args.Bool(1), args.Error(2)
}

func (m *mockDispatchLogRepo) DeleteOldLogsByStatus(ctx context.Context, status string, olderThan time.Duration) (int64, error) {
	args := m.Called(ctx, status, olderThan)
	return args.Get(0).(int64), args.Error(1)
}

type mockHistoryRepo struct {
	mock.Mock
}

func (m *mockHistoryRepo) Create(ctx context.Context, userID, actionType, description string, details map[string]interface{}) (*models.ActionHistory, error) {
	args := m.Called(ctx, userID, actionType, description, details)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ActionHistory), args.Error(1)
}

func (m *mockHistoryRepo) Update(ctx context.Context, historyID string, updates map[string]interface{}) error {
	args := m.Called(ctx, historyID, updates)
	return args.Error(0)
}

func (m *mockHistoryRepo) UpdateStatus(ctx context.Context, id, status, errorMessage string) error {
	args := m.Called(ctx, id, status, errorMessage)
	return args.Error(0)
}

func (m *mockHistoryRepo) GetByID(ctx context.Context, historyID string) (*models.ActionHistory, error) {
	args := m.Called(ctx, historyID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ActionHistory), args.Error(1)
}

func (m *mockHistoryRepo) GetUserHistory(ctx context.Context, userID string, limit, offset int) ([]models.ActionHistory, error) {
	args := m.Called(ctx, userID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.ActionHistory), args.Error(1)
}

func (m *mockHistoryRepo) GetByUserID(ctx context.Context, userID string, limit, offset int) ([]models.ActionHistory, error) {
	args := m.Called(ctx, userID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.ActionHistory), args.Error(1)
}

func (m *mockHistoryRepo) GetByActionType(ctx context.Context, userID, actionType string, limit int) ([]models.ActionHistory, error) {
	args := m.Called(ctx, userID, actionType, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.ActionHistory), args.Error(1)
}

func (m *mockHistoryRepo) GetUserHistoryFiltered(ctx context.Context, req models.HistoryListRequest, userID string) ([]models.ActionHistory, error) {
	args := m.Called(ctx, req, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.ActionHistory), args.Error(1)
}

func (m *mockHistoryRepo) MarkCompleted(ctx context.Context, historyID string) error {
	args := m.Called(ctx, historyID)
	return args.Error(0)
}

func (m *mockHistoryRepo) MarkFailed(ctx context.Context, historyID, errorMessage string) error {
	args := m.Called(ctx, historyID, errorMessage)
	return args.Error(0)
}

func (m *mockHistoryRepo) MarkRunning(ctx context.Context, historyID string) error {
	args := m.Called(ctx, historyID)
	return args.Error(0)
}

func (m *mockHistoryRepo) IncrementSuccessCount(ctx context.Context, historyID string) error {
	args := m.Called(ctx, historyID)
	return args.Error(0)
}

func (m *mockHistoryRepo) IncrementErrorCount(ctx context.Context, historyID string, errorMsg string) error {
	args := m.Called(ctx, historyID, errorMsg)
	return args.Error(0)
}

func (m *mockHistoryRepo) UpdateCounts(ctx context.Context, historyID string, successCount, errorCount int32) error {
	args := m.Called(ctx, historyID, successCount, errorCount)
	return args.Error(0)
}

func (m *mockHistoryRepo) DeleteUserHistory(ctx context.Context, userID string, beforeDate *time.Time, actionTypes []string) (int64, error) {
	args := m.Called(ctx, userID, beforeDate, actionTypes)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockHistoryRepo) CountUserHistoryFiltered(ctx context.Context, req models.HistoryListRequest, userID string) (models.HistoryStatsResult, error) {
	args := m.Called(ctx, req, userID)
	return args.Get(0).(models.HistoryStatsResult), args.Error(1)
}

type mockSettingsRepo struct {
	mock.Mock
}

func (m *mockSettingsRepo) Get(ctx context.Context, keys ...string) (models.SettingsMap, error) {
	args := m.Called(ctx, keys)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(models.SettingsMap), args.Error(1)
}

func (m *mockSettingsRepo) GetAll(ctx context.Context) (models.SettingsMap, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(models.SettingsMap), args.Error(1)
}

func (m *mockSettingsRepo) Update(ctx context.Context, key string, value interface{}) error {
	args := m.Called(ctx, key, value)
	return args.Error(0)
}

func (m *mockSettingsRepo) GetDispatchText(ctx context.Context, dispatchTextID string) (*models.DispatchText, error) {
	args := m.Called(ctx, dispatchTextID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.DispatchText), args.Error(1)
}

func (m *mockSettingsRepo) GetDispatchTextsByUser(ctx context.Context, userID string) ([]models.DispatchText, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.DispatchText), args.Error(1)
}

func (m *mockSettingsRepo) GetActiveDispatchTextByID(ctx context.Context, dispatchTextID, userID string) (*models.DispatchText, error) {
	args := m.Called(ctx, dispatchTextID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.DispatchText), args.Error(1)
}

func (m *mockSettingsRepo) GetActiveDispatchTextsByUser(ctx context.Context, userID string) ([]models.DispatchText, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.DispatchText), args.Error(1)
}

func (m *mockSettingsRepo) CreateDispatchText(ctx context.Context, userID string, dt *models.DispatchText) (*models.DispatchText, error) {
	args := m.Called(ctx, userID, dt)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.DispatchText), args.Error(1)
}

func (m *mockSettingsRepo) UpdateDispatchText(ctx context.Context, dispatchTextID string, updates map[string]interface{}) error {
	args := m.Called(ctx, dispatchTextID, updates)
	return args.Error(0)
}

func (m *mockSettingsRepo) DeleteDispatchText(ctx context.Context, dispatchTextID, userID string) error {
	args := m.Called(ctx, dispatchTextID, userID)
	return args.Error(0)
}

type mockProfileRepo struct {
	mock.Mock
}

func (m *mockProfileRepo) EnsureUserProfile(ctx context.Context, userID, email string) (*models.Profile, error) {
	args := m.Called(ctx, userID, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Profile), args.Error(1)
}

func (m *mockProfileRepo) GetProfile(ctx context.Context, userID string) (*models.Profile, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Profile), args.Error(1)
}

func (m *mockProfileRepo) GetProfileByEmail(ctx context.Context, email string) (*models.Profile, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Profile), args.Error(1)
}

func (m *mockProfileRepo) GetProfileByEmailWithBilling(ctx context.Context, email string) (*models.Profile, *models.ProfileBillingFields, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, nil, args.Error(2)
	}
	if args.Get(1) == nil {
		return args.Get(0).(*models.Profile), nil, args.Error(2)
	}
	return args.Get(0).(*models.Profile), args.Get(1).(*models.ProfileBillingFields), args.Error(2)
}

func (m *mockProfileRepo) CreateProfile(ctx context.Context, profile *models.Profile) (*models.Profile, error) {
	args := m.Called(ctx, profile)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Profile), args.Error(1)
}

func (m *mockProfileRepo) UpdateProfile(ctx context.Context, userID string, updates map[string]interface{}) error {
	args := m.Called(ctx, userID, updates)
	return args.Error(0)
}

func (m *mockProfileRepo) UpdatePassword(ctx context.Context, userID, passwordHash string) error {
	args := m.Called(ctx, userID, passwordHash)
	return args.Error(0)
}

func (m *mockProfileRepo) UpdateBillingStatus(ctx context.Context, userID string, billing *models.ProfileBillingFields) error {
	args := m.Called(ctx, userID, billing)
	return args.Error(0)
}

func (m *mockProfileRepo) PauseUserAutomations(ctx context.Context, userID string) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *mockProfileRepo) ListProfiles(ctx context.Context, cursor string, limit int) ([]*models.Profile, string, error) {
	args := m.Called(ctx, cursor, limit)
	if args.Get(0) == nil {
		return nil, "", args.Error(2)
	}
	return args.Get(0).([]*models.Profile), args.String(1), args.Error(2)
}

func (m *mockProfileRepo) SearchProfiles(ctx context.Context, query, cursor string, limit int) ([]*models.Profile, string, error) {
	args := m.Called(ctx, query, cursor, limit)
	if args.Get(0) == nil {
		return nil, "", args.Error(2)
	}
	return args.Get(0).([]*models.Profile), args.String(1), args.Error(2)
}

func (m *mockProfileRepo) GetMaxAutomations(ctx context.Context, userID string) (int, error) {
	args := m.Called(ctx, userID)
	return args.Int(0), args.Error(1)
}

func (m *mockProfileRepo) CountUserAutomations(ctx context.Context, userID string) (int, error) {
	args := m.Called(ctx, userID)
	return args.Int(0), args.Error(1)
}

func (m *mockProfileRepo) CreateUserCredentials(ctx context.Context, userID, passwordHash string) error {
	args := m.Called(ctx, userID, passwordHash)
	return args.Error(0)
}

func (m *mockProfileRepo) GetProfileStats(ctx context.Context) (*repository.ProfileStats, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.ProfileStats), args.Error(1)
}

type mockMlLinkListItemRepo struct {
	mock.Mock
}

func (m *mockMlLinkListItemRepo) GetNextUnconsumed(ctx context.Context, listID string, limit int) ([]*models.MlLinkListItem, error) {
	args := m.Called(ctx, listID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.MlLinkListItem), args.Error(1)
}

func (m *mockMlLinkListItemRepo) MarkAsConsumed(ctx context.Context, itemIDs []string, automationID string, groupID *string) error {
	args := m.Called(ctx, itemIDs, automationID, groupID)
	return args.Error(0)
}

func (m *mockMlLinkListItemRepo) CountRemaining(ctx context.Context, listID string) (int, error) {
	args := m.Called(ctx, listID)
	return args.Int(0), args.Error(1)
}

func (m *mockMlLinkListItemRepo) GetByListID(ctx context.Context, listID string) ([]*models.MlLinkListItem, error) {
	args := m.Called(ctx, listID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.MlLinkListItem), args.Error(1)
}

func (m *mockMlLinkListItemRepo) Create(ctx context.Context, item *models.MlLinkListItem) error {
	args := m.Called(ctx, item)
	return args.Error(0)
}

func (m *mockMlLinkListItemRepo) BulkCreate(ctx context.Context, items []*models.MlLinkListItem) error {
	args := m.Called(ctx, items)
	return args.Error(0)
}

func (m *mockMlLinkListItemRepo) ResetConsumed(ctx context.Context, listID string) (int64, error) {
	args := m.Called(ctx, listID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockMlLinkListItemRepo) GetConsumedByAutomation(ctx context.Context, automationID string) ([]*models.MlLinkListItem, error) {
	args := m.Called(ctx, automationID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.MlLinkListItem), args.Error(1)
}

// =============================================================================
// Mock implementations for interface dependencies (urlSafetyEvaluator, backpressureProvider)
// =============================================================================

type mockURLSafetyEvaluator struct {
	mock.Mock
}

func (m *mockURLSafetyEvaluator) Evaluate(ctx context.Context, message string) (*urlsafety.UrlSafetyDecision, error) {
	args := m.Called(ctx, message)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*urlsafety.UrlSafetyDecision), args.Error(1)
}

type mockBackpressureProvider struct {
	mock.Mock
}

func (m *mockBackpressureProvider) CanEnqueue(ctx context.Context, userID string) (bool, string, error) {
	args := m.Called(ctx, userID)
	return args.Bool(0), args.String(1), args.Error(2)
}

func (m *mockBackpressureProvider) RecordEnqueue(ctx context.Context, userID string) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

// newInternalTestService creates an automationService for internal testing.
// Returns the concrete *automationService so unexported methods can be called.
func newInternalTestService(t *testing.T) (*automationService, *internalTestDeps) {
	t.Helper()
	logger := zaptest.NewLogger(t)

	deps := &internalTestDeps{
		automationRepo:     new(mockAutomationRepo),
		instanceRepo:       new(mockInstanceRepo),
		dispatchLogRepo:    new(mockDispatchLogRepo),
		historyRepo:        new(mockHistoryRepo),
		settingsRepo:       new(mockSettingsRepo),
		profileRepo:        new(mockProfileRepo),
		mlLinkListItemRepo: new(mockMlLinkListItemRepo),
	}

	svc := &automationService{
		automationRepo:     deps.automationRepo,
		instanceRepo:       deps.instanceRepo,
		dispatchLogRepo:    deps.dispatchLogRepo,
		historyRepo:        deps.historyRepo,
		settingsRepo:       deps.settingsRepo,
		profileRepo:        deps.profileRepo,
		mlLinkListItemRepo: deps.mlLinkListItemRepo,
		logger:             logger,
	}

	return svc, deps
}

type internalTestDeps struct {
	automationRepo     *mockAutomationRepo
	instanceRepo       *mockInstanceRepo
	dispatchLogRepo    *mockDispatchLogRepo
	historyRepo        *mockHistoryRepo
	settingsRepo       *mockSettingsRepo
	profileRepo        *mockProfileRepo
	mlLinkListItemRepo *mockMlLinkListItemRepo
}

// =============================================================================
// stringPtrValue Tests (0% → 100%)
// =============================================================================

func TestStringPtrValue(t *testing.T) {
	t.Parallel()

	t.Run("nil pointer returns empty string", func(t *testing.T) {
		t.Parallel()
		result := stringPtrValue(nil)
		assert.Equal(t, "", result)
	})

	t.Run("non-nil pointer returns value", func(t *testing.T) {
		t.Parallel()
		val := "hello"
		result := stringPtrValue(&val)
		assert.Equal(t, "hello", result)
	})

	t.Run("empty string pointer returns empty string", func(t *testing.T) {
		t.Parallel()
		val := ""
		result := stringPtrValue(&val)
		assert.Equal(t, "", result)
	})
}

// =============================================================================
// isDuplicate Tests (37.5% → higher)
// =============================================================================

func TestIsDuplicate(t *testing.T) {
	t.Parallel()

	t.Run("nil cache and nil dispatchLogRepo returns false", func(t *testing.T) {
		t.Parallel()
		logger := zaptest.NewLogger(t)
		svc := &automationService{
			cache:  nil,
			logger: logger,
		}
		result := svc.isDuplicate(context.Background(), "user-1", "group-1", "key-1", 24)
		assert.False(t, result)
	})

	t.Run("nil cache, DB returns match (duplicate found)", func(t *testing.T) {
		t.Parallel()
		svc, deps := newInternalTestService(t)
		// cache is nil by default

		deps.dispatchLogRepo.On("GetRecentByCanonicalKey", mock.Anything, "user-1", "group-1", "key-1", 24).
			Return(&models.DispatchLog{Status: "queued"}, nil)

		result := svc.isDuplicate(context.Background(), "user-1", "group-1", "key-1", 24)
		assert.True(t, result)
	})

	t.Run("nil cache, DB returns nil (no duplicate)", func(t *testing.T) {
		t.Parallel()
		svc, deps := newInternalTestService(t)

		deps.dispatchLogRepo.On("GetRecentByCanonicalKey", mock.Anything, "user-1", "group-1", "key-1", 24).
			Return(nil, nil)

		result := svc.isDuplicate(context.Background(), "user-1", "group-1", "key-1", 24)
		assert.False(t, result)
	})

	t.Run("nil cache, DB returns error (not duplicate, fail-open)", func(t *testing.T) {
		t.Parallel()
		svc, deps := newInternalTestService(t)

		deps.dispatchLogRepo.On("GetRecentByCanonicalKey", mock.Anything, "user-1", "group-1", "key-1", 24).
			Return(nil, errors.New("db error"))

		result := svc.isDuplicate(context.Background(), "user-1", "group-1", "key-1", 24)
		assert.False(t, result)
	})

	t.Run("nil cache and nil dispatchLogRepo both nil", func(t *testing.T) {
		t.Parallel()
		logger := zaptest.NewLogger(t)
		svc := &automationService{
			cache:           nil,
			dispatchLogRepo: nil,
			logger:          logger,
		}
		result := svc.isDuplicate(context.Background(), "user-1", "group-1", "key-1", 24)
		assert.False(t, result)
	})
}

// =============================================================================
// markAsSent Tests (40% → higher)
// =============================================================================

func TestMarkAsSent(t *testing.T) {
	t.Parallel()

	t.Run("nil cache does nothing", func(t *testing.T) {
		t.Parallel()
		logger := zaptest.NewLogger(t)
		svc := &automationService{
			cache:  nil,
			logger: logger,
		}
		// Should not panic
		svc.markAsSent(context.Background(), "user-1", "group-1", "key-1", 24)
	})
}

// =============================================================================
// logDispatch Tests (69.2% → higher)
// =============================================================================

func TestLogDispatch(t *testing.T) {
	// Not parallel: subtests modify env vars via t.Setenv + config.ResetFeatureFlags

	t.Run("nil dispatchLogRepo does nothing", func(t *testing.T) {
		logger := zaptest.NewLogger(t)
		svc := &automationService{
			dispatchLogRepo: nil,
			logger:          logger,
		}

		automation := &models.Automation{
			ID:     "auto-1",
			UserID: "user-1",
			Name:   "Test",
		}

		// Should not panic
		svc.logDispatch(context.Background(), automation, "group-1", "https://test.com", "queued", "")
	})

	t.Run("feature flag disabled skips creation", func(t *testing.T) {
		svc, deps := newInternalTestService(t)

		t.Setenv("FEATURE_USE_GO_DISPATCH_LOGS", "false")
		config.ResetFeatureFlags()

		automation := &models.Automation{
			ID:     "auto-1",
			UserID: "user-1",
			Name:   "Test",
		}

		svc.logDispatch(context.Background(), automation, "group-1", "https://test.com", "queued", "")

		deps.dispatchLogRepo.AssertNotCalled(t, "Create")
	})

	t.Run("create error is logged (not returned)", func(t *testing.T) {
		svc, deps := newInternalTestService(t)

		t.Setenv("FEATURE_USE_GO_DISPATCH_LOGS", "true")
		config.ResetFeatureFlags()

		automation := &models.Automation{
			ID:     "auto-1",
			UserID: "user-1",
			Name:   "Test",
		}

		deps.dispatchLogRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.DispatchLog")).
			Return(nil, errors.New("db insert failed"))

		svc.logDispatch(context.Background(), automation, "group-1", "https://test.com", "queued", "")

		deps.dispatchLogRepo.AssertCalled(t, "Create", mock.Anything, mock.AnythingOfType("*models.DispatchLog"))
	})

	t.Run("successful creation with error message", func(t *testing.T) {
		svc, deps := newInternalTestService(t)

		t.Setenv("FEATURE_USE_GO_DISPATCH_LOGS", "true")
		config.ResetFeatureFlags()

		automation := &models.Automation{
			ID:     "auto-1",
			UserID: "user-1",
			Name:   "Test",
		}

		deps.dispatchLogRepo.On("Create", mock.Anything, mock.MatchedBy(func(log *models.DispatchLog) bool {
			return log.Error != nil && *log.Error == "some error"
		})).Return(&models.DispatchLog{}, nil)

		svc.logDispatch(context.Background(), automation, "group-1", "https://test.com", "failed", "some error")

		deps.dispatchLogRepo.AssertExpectations(t)
	})

	t.Run("with canonical key option", func(t *testing.T) {
		svc, deps := newInternalTestService(t)

		t.Setenv("FEATURE_USE_GO_DISPATCH_LOGS", "true")
		config.ResetFeatureFlags()

		automation := &models.Automation{
			ID:     "auto-1",
			UserID: "user-1",
			Name:   "Test",
		}

		deps.dispatchLogRepo.On("Create", mock.Anything, mock.MatchedBy(func(log *models.DispatchLog) bool {
			return log.CanonicalKey != nil && *log.CanonicalKey == "shopee:12345" &&
				log.Store != nil && *log.Store == "shopee"
		})).Return(&models.DispatchLog{}, nil)

		svc.logDispatch(context.Background(), automation, "group-1", "https://test.com", "queued", "",
			withCanonicalKey("shopee:12345", "shopee"))

		deps.dispatchLogRepo.AssertExpectations(t)
	})
}

// =============================================================================
// validateMonitorAutomation Tests (0% → 100%)
// =============================================================================

func TestValidateMonitorAutomation(t *testing.T) {
	t.Parallel()

	t.Run("no monitor groups returns error", func(t *testing.T) {
		t.Parallel()
		svc, deps := newInternalTestService(t)

		automation := &models.Automation{
			ID:              "auto-1",
			UserID:          "user-1",
			MonitorGroupIDs: []string{},
			GroupIDs:        []string{"target@g.us"},
		}

		deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
			Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)

		result, err := svc.validateMonitorAutomation(context.Background(), automation)
		require.NoError(t, err)
		assert.Equal(t, "error", result.Status)

		var monitorCheck *models.DiagnoseCheck
		for _, c := range result.Checks {
			if c.Name == "GRUPOS_MONITORADOS" {
				monitorCheck = &c
				break
			}
		}
		require.NotNil(t, monitorCheck)
		assert.Equal(t, "error", monitorCheck.Status)
		assert.Contains(t, monitorCheck.Message, "Nenhum grupo de monitoramento configurado")
	})

	t.Run("monitor groups without @g.us returns error", func(t *testing.T) {
		t.Parallel()
		svc, deps := newInternalTestService(t)

		automation := &models.Automation{
			ID:              "auto-1",
			UserID:          "user-1",
			MonitorGroupIDs: []string{"invalid-group"},
			GroupIDs:        []string{"target@g.us"},
		}

		deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
			Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)

		result, err := svc.validateMonitorAutomation(context.Background(), automation)
		require.NoError(t, err)
		assert.Equal(t, "error", result.Status)

		var monitorCheck *models.DiagnoseCheck
		for _, c := range result.Checks {
			if c.Name == "GRUPOS_MONITORADOS" {
				monitorCheck = &c
				break
			}
		}
		require.NotNil(t, monitorCheck)
		assert.Equal(t, "error", monitorCheck.Status)
		assert.Contains(t, monitorCheck.Message, "@g.us")
	})

	t.Run("valid monitor groups, no target groups returns error", func(t *testing.T) {
		t.Parallel()
		svc, deps := newInternalTestService(t)

		automation := &models.Automation{
			ID:              "auto-1",
			UserID:          "user-1",
			MonitorGroupIDs: []string{"source@g.us"},
			GroupIDs:        []string{}, // no target groups
		}

		deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
			Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)

		result, err := svc.validateMonitorAutomation(context.Background(), automation)
		require.NoError(t, err)
		assert.Equal(t, "error", result.Status)
	})

	t.Run("valid monitor groups, target groups without @g.us returns error", func(t *testing.T) {
		t.Parallel()
		svc, deps := newInternalTestService(t)

		automation := &models.Automation{
			ID:              "auto-1",
			UserID:          "user-1",
			MonitorGroupIDs: []string{"source@g.us"},
			GroupIDs:        []string{"invalid-target"},
		}

		deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
			Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)

		result, err := svc.validateMonitorAutomation(context.Background(), automation)
		require.NoError(t, err)
		assert.Equal(t, "error", result.Status)
	})

	t.Run("all valid, connected instance", func(t *testing.T) {
		t.Parallel()
		svc, deps := newInternalTestService(t)

		automation := &models.Automation{
			ID:              "auto-1",
			UserID:          "user-1",
			MonitorGroupIDs: []string{"source1@g.us", "source2@g.us"},
			GroupIDs:        []string{"target@g.us"},
		}

		deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
			Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)

		result, err := svc.validateMonitorAutomation(context.Background(), automation)
		require.NoError(t, err)
		assert.Equal(t, "ok", result.Status)
		assert.Len(t, result.Checks, 5) // GRUPOS_MONITORADOS, GRUPOS_DESTINO, WHATSAPP, LOJAS, ANTI_DUPLICACAO
	})

	t.Run("all valid but no connected instance", func(t *testing.T) {
		t.Parallel()
		svc, deps := newInternalTestService(t)

		automation := &models.Automation{
			ID:              "auto-1",
			UserID:          "user-1",
			MonitorGroupIDs: []string{"source@g.us"},
			GroupIDs:        []string{"target@g.us"},
		}

		deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
			Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "disconnected"}}, nil)

		result, err := svc.validateMonitorAutomation(context.Background(), automation)
		require.NoError(t, err)
		assert.Equal(t, "error", result.Status)
	})

	t.Run("with time window outside range", func(t *testing.T) {
		t.Parallel()
		svc, deps := newInternalTestService(t)

		// Use a time window that's definitely in the past (e.g., 01:00-02:00)
		startTime := "01:00"
		endTime := "02:00"
		automation := &models.Automation{
			ID:              "auto-1",
			UserID:          "user-1",
			MonitorGroupIDs: []string{"source@g.us"},
			GroupIDs:        []string{"target@g.us"},
			StartTime:       &startTime,
			EndTime:         &endTime,
		}

		deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
			Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)

		result, err := svc.validateMonitorAutomation(context.Background(), automation)
		require.NoError(t, err)
		// May be "ok" or "warning" depending on current time
		assert.NotEmpty(t, result.Checks)
	})

	t.Run("with custom stores", func(t *testing.T) {
		t.Parallel()
		svc, deps := newInternalTestService(t)

		automation := &models.Automation{
			ID:              "auto-1",
			UserID:          "user-1",
			MonitorGroupIDs: []string{"source@g.us"},
			GroupIDs:        []string{"target@g.us"},
			Stores:          []string{"shopee", "mercadolivre"},
		}

		deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
			Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)

		result, err := svc.validateMonitorAutomation(context.Background(), automation)
		require.NoError(t, err)

		var storesCheck *models.DiagnoseCheck
		for _, c := range result.Checks {
			if c.Name == "LOJAS" {
				storesCheck = &c
				break
			}
		}
		require.NotNil(t, storesCheck)
		assert.Contains(t, storesCheck.Message, "shopee")
		assert.Contains(t, storesCheck.Message, "mercadolivre")
	})

	t.Run("dedupe hours zero produces warning", func(t *testing.T) {
		t.Parallel()
		svc, deps := newInternalTestService(t)

		dedupeHours := 0
		automation := &models.Automation{
			ID:              "auto-1",
			UserID:          "user-1",
			MonitorGroupIDs: []string{"source@g.us"},
			GroupIDs:        []string{"target@g.us"},
			DedupeHours:     &dedupeHours,
		}

		deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
			Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)

		result, err := svc.validateMonitorAutomation(context.Background(), automation)
		require.NoError(t, err)

		var dedupeCheck *models.DiagnoseCheck
		for _, c := range result.Checks {
			if c.Name == "ANTI_DUPLICACAO" {
				dedupeCheck = &c
				break
			}
		}
		require.NotNil(t, dedupeCheck)
		assert.Equal(t, "warning", dedupeCheck.Status)
	})

	t.Run("dedupe hours positive produces ok", func(t *testing.T) {
		t.Parallel()
		svc, deps := newInternalTestService(t)

		dedupeHours := 48
		automation := &models.Automation{
			ID:              "auto-1",
			UserID:          "user-1",
			MonitorGroupIDs: []string{"source@g.us"},
			GroupIDs:        []string{"target@g.us"},
			DedupeHours:     &dedupeHours,
		}

		deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
			Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)

		result, err := svc.validateMonitorAutomation(context.Background(), automation)
		require.NoError(t, err)

		var dedupeCheck *models.DiagnoseCheck
		for _, c := range result.Checks {
			if c.Name == "ANTI_DUPLICACAO" {
				dedupeCheck = &c
				break
			}
		}
		require.NotNil(t, dedupeCheck)
		assert.Equal(t, "ok", dedupeCheck.Status)
		assert.Contains(t, dedupeCheck.Message, "48h")
	})
}

// =============================================================================
// buildListModeMessage Tests (65.6% → higher)
// =============================================================================

func TestBuildListModeMessage(t *testing.T) {
	t.Parallel()

	t.Run("nil preview uses item metadata fallback", func(t *testing.T) {
		t.Parallel()
		svc, _ := newInternalTestService(t)

		title := "Product Title"
		price := 49.90
		imageURL := "https://img.test/product.jpg"

		automation := &models.Automation{
			ID:     "auto-1",
			UserID: "user-1",
			CTAs:   []string{"Buy now!"},
		}

		item := &models.MlLinkListItem{
			ID:              "item-1",
			OriginURL:       "https://produto.mercadolivre.com.br/MLB-123",
			ProductTitle:    &title,
			ProductPrice:    &price,
			ProductImageURL: &imageURL,
		}

		message := svc.buildListModeMessage(automation, item, nil, nil, "https://affiliate.link", "", false, nil, nil)
		assert.NotEmpty(t, message)
		assert.Contains(t, message, "Product Title")
	})

	t.Run("preview with all optional fields", func(t *testing.T) {
		t.Parallel()
		svc, _ := newInternalTestService(t)

		automation := &models.Automation{
			ID:     "auto-1",
			UserID: "user-1",
		}

		item := &models.MlLinkListItem{
			ID:        "item-1",
			OriginURL: "https://produto.mercadolivre.com.br/MLB-123",
		}

		condition := "new"
		soldCount := 100
		rating := 4.5
		reviewCount := 50
		installments := "12x R$ 8,33"
		originalPrice := 120.0
		preview := &models.MLProductPreview{
			Title:         "Test Product Full",
			Price:         99.90,
			OriginalPrice: &originalPrice,
			Discount:      17,
			Image:         "https://img.test/full.jpg",
			Condition:     &condition,
			SoldCount:     &soldCount,
			Rating:        &rating,
			ReviewCount:   &reviewCount,
			Installments:  &installments,
		}

		message := svc.buildListModeMessage(automation, item, preview, nil, "https://affiliate.link", "", false, nil, nil)
		assert.NotEmpty(t, message)
		assert.Contains(t, message, "Test Product Full")
	})

	t.Run("with dispatch template", func(t *testing.T) {
		t.Parallel()
		svc, _ := newInternalTestService(t)

		automation := &models.Automation{
			ID:     "auto-1",
			UserID: "user-1",
			CTAs:   []string{"CTA1", "CTA2"},
		}

		item := &models.MlLinkListItem{
			ID:        "item-1",
			OriginURL: "https://produto.mercadolivre.com.br/MLB-123",
		}

		preview := &models.MLProductPreview{
			Title: "Custom Template Product",
			Price: 59.90,
			Image: "https://img.test/custom.jpg",
		}

		template := "{{titulo}} por {{preco}} - {{link}}"
		message := svc.buildListModeMessage(automation, item, preview, nil, "https://affiliate.link", template, false, nil, nil)
		assert.NotEmpty(t, message)
	})

	t.Run("nil item metadata fields (all nil)", func(t *testing.T) {
		t.Parallel()
		svc, _ := newInternalTestService(t)

		automation := &models.Automation{
			ID:     "auto-1",
			UserID: "user-1",
		}

		item := &models.MlLinkListItem{
			ID:              "item-1",
			OriginURL:       "https://produto.mercadolivre.com.br/MLB-123",
			ProductTitle:    nil,
			ProductPrice:    nil,
			ProductImageURL: nil,
		}

		message := svc.buildListModeMessage(automation, item, nil, nil, "https://affiliate.link", "", false, nil, nil)
		assert.NotEmpty(t, message) // Should still render with empty/zero values
	})
}

// =============================================================================
// getConnectedInstance Tests (88.9% → higher)
// =============================================================================

func TestGetConnectedInstance(t *testing.T) {
	t.Parallel()

	t.Run("no instances returns error", func(t *testing.T) {
		t.Parallel()
		svc, deps := newInternalTestService(t)

		deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
			Return([]models.Instance{}, nil)

		instance, err := svc.getConnectedInstance(context.Background(), "user-1")
		assert.Error(t, err)
		assert.Nil(t, instance)
	})

	t.Run("repo error returns error", func(t *testing.T) {
		t.Parallel()
		svc, deps := newInternalTestService(t)

		deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
			Return(nil, errors.New("db error"))

		instance, err := svc.getConnectedInstance(context.Background(), "user-1")
		assert.Error(t, err)
		assert.Nil(t, instance)
	})

	t.Run("all disconnected returns error", func(t *testing.T) {
		t.Parallel()
		svc, deps := newInternalTestService(t)

		deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
			Return([]models.Instance{
				{ID: 1, Status: "disconnected"},
				{ID: 2, Status: "stopped"},
			}, nil)

		instance, err := svc.getConnectedInstance(context.Background(), "user-1")
		assert.Error(t, err)
		assert.Nil(t, instance)
		assert.Contains(t, err.Error(), "conectada")
	})

	t.Run("connected instance found", func(t *testing.T) {
		t.Parallel()
		svc, deps := newInternalTestService(t)

		deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
			Return([]models.Instance{
				{ID: 1, InstanceID: "inst-1", Status: "disconnected"},
				{ID: 2, InstanceID: "inst-2", Status: "connected"},
			}, nil)

		instance, err := svc.getConnectedInstance(context.Background(), "user-1")
		require.NoError(t, err)
		require.NotNil(t, instance)
		assert.Equal(t, "inst-2", instance.InstanceID)
	})
}

// =============================================================================
// withCanonicalKey Tests (100% - verifying option pattern)
// =============================================================================

func TestWithCanonicalKey(t *testing.T) {
	t.Parallel()

	log := &models.DispatchLog{}
	opt := withCanonicalKey("test-key", "shopee")
	opt(log)

	require.NotNil(t, log.CanonicalKey)
	assert.Equal(t, "test-key", *log.CanonicalKey)
	require.NotNil(t, log.Store)
	assert.Equal(t, "shopee", *log.Store)
}

// =============================================================================
// processListMode internal gap tests
// =============================================================================

func TestProcessListMode_EmptyOriginURL(t *testing.T) {
	t.Parallel()
	svc, deps := newInternalTestService(t)

	listID := "list-123"
	automation := &models.Automation{
		ID:           "auto-1",
		UserID:       "user-1",
		Name:         "List Test",
		MlLinkListID: &listID,
		GroupIDs:     []string{"group@g.us"},
	}

	deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
		Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)
	deps.mlLinkListItemRepo.On("GetNextUnconsumed", mock.Anything, listID, 1).
		Return([]*models.MlLinkListItem{
			{ID: "item-1", OriginURL: "", ListID: listID},     // Empty URL → skipped
			{ID: "item-2", OriginURL: "   ", ListID: listID},  // Whitespace → skipped
		}, nil)
	deps.dispatchLogRepo.On("Create", mock.Anything, mock.Anything).Return(&models.DispatchLog{}, nil).Maybe()

	result, err := svc.processListMode(context.Background(), automation)
	require.NoError(t, err)
	assert.Equal(t, 0, result.MessagesSent)
}

func TestProcessListMode_InvalidMLURL(t *testing.T) {
	t.Parallel()
	svc, deps := newInternalTestService(t)

	listID := "list-123"
	automation := &models.Automation{
		ID:           "auto-1",
		UserID:       "user-1",
		Name:         "List Test",
		MlLinkListID: &listID,
		GroupIDs:     []string{"group@g.us"},
	}

	deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
		Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)
	deps.mlLinkListItemRepo.On("GetNextUnconsumed", mock.Anything, listID, 1).
		Return([]*models.MlLinkListItem{
			{ID: "item-1", OriginURL: "https://amazon.com/product", ListID: listID}, // Not ML URL
		}, nil)
	deps.dispatchLogRepo.On("Create", mock.Anything, mock.Anything).Return(&models.DispatchLog{}, nil).Maybe()

	result, err := svc.processListMode(context.Background(), automation)
	require.NoError(t, err)
	assert.Equal(t, 0, result.MessagesSent)
}

func TestProcessListMode_NoMlLinkListID(t *testing.T) {
	t.Parallel()
	svc, deps := newInternalTestService(t)

	automation := &models.Automation{
		ID:           "auto-1",
		UserID:       "user-1",
		Name:         "List Test",
		MlLinkListID: nil, // No list ID
		GroupIDs:     []string{"group@g.us"},
	}

	deps.dispatchLogRepo.On("Create", mock.Anything, mock.Anything).Return(&models.DispatchLog{}, nil).Maybe()

	result, err := svc.processListMode(context.Background(), automation)
	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Message, "Lista de Links")
}

func TestProcessListMode_NoValidGroups(t *testing.T) {
	t.Parallel()
	svc, deps := newInternalTestService(t)

	listID := "list-123"
	automation := &models.Automation{
		ID:           "auto-1",
		UserID:       "user-1",
		Name:         "List Test",
		MlLinkListID: &listID,
		GroupIDs:     []string{"invalid-group"}, // No @g.us
	}

	deps.dispatchLogRepo.On("Create", mock.Anything, mock.Anything).Return(&models.DispatchLog{}, nil).Maybe()

	result, err := svc.processListMode(context.Background(), automation)
	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Message, "@g.us")
}

func TestProcessListMode_InstanceError(t *testing.T) {
	t.Parallel()
	svc, deps := newInternalTestService(t)

	listID := "list-123"
	automation := &models.Automation{
		ID:           "auto-1",
		UserID:       "user-1",
		Name:         "List Test",
		MlLinkListID: &listID,
		GroupIDs:     []string{"group@g.us"},
	}

	deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
		Return(nil, errors.New("db error"))

	_, err := svc.processListMode(context.Background(), automation)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "WhatsApp")
}

func TestProcessListMode_NoUnconsumedItems(t *testing.T) {
	t.Parallel()
	svc, deps := newInternalTestService(t)

	listID := "list-123"
	automation := &models.Automation{
		ID:           "auto-1",
		UserID:       "user-1",
		Name:         "List Test",
		MlLinkListID: &listID,
		GroupIDs:     []string{"group@g.us"},
	}

	deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
		Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)
	deps.mlLinkListItemRepo.On("GetNextUnconsumed", mock.Anything, listID, 1).
		Return([]*models.MlLinkListItem{}, nil)
	deps.dispatchLogRepo.On("Create", mock.Anything, mock.Anything).Return(&models.DispatchLog{}, nil).Maybe()

	result, err := svc.processListMode(context.Background(), automation)
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, 0, result.OffersFound)
}

func TestProcessListMode_WithDispatchTemplate(t *testing.T) {
	t.Parallel()
	svc, deps := newInternalTestService(t)

	listID := "list-123"
	dtID := "dt-1"
	automation := &models.Automation{
		ID:             "auto-1",
		UserID:         "user-1",
		Name:           "List Test",
		MlLinkListID:   &listID,
		GroupIDs:       []string{"group@g.us"},
		DispatchTextID: &dtID,
	}

	deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
		Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)
	deps.settingsRepo.On("GetActiveDispatchTextByID", mock.Anything, "dt-1", "user-1").
		Return(&models.DispatchText{ID: "dt-1", Body: "{{titulo}} - {{link}}"}, nil)
	deps.mlLinkListItemRepo.On("GetNextUnconsumed", mock.Anything, listID, 1).
		Return([]*models.MlLinkListItem{
			{ID: "item-1", OriginURL: "https://produto.mercadolivre.com.br/MLB-123", ListID: listID},
		}, nil)
	deps.mlLinkListItemRepo.On("MarkAsConsumed", mock.Anything, []string{"item-1"}, "auto-1", mock.AnythingOfType("*string")).Return(nil)
	deps.dispatchLogRepo.On("Create", mock.Anything, mock.Anything).Return(&models.DispatchLog{}, nil).Maybe()

	result, err := svc.processListMode(context.Background(), automation)
	require.NoError(t, err)
	// With nil queueClient, messages still count as sent (skips enqueue, falls through)
	assert.Equal(t, 1, result.MessagesSent)
}

func TestProcessListMode_CustomItemsPerRun(t *testing.T) {
	t.Parallel()
	svc, deps := newInternalTestService(t)

	listID := "list-123"
	itemsPerRun := 5
	automation := &models.Automation{
		ID:              "auto-1",
		UserID:          "user-1",
		Name:            "List Test",
		MlLinkListID:    &listID,
		GroupIDs:        []string{"group@g.us"},
		ListItemsPerRun: &itemsPerRun,
	}

	deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
		Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)
	// Expect GetNextUnconsumed to be called with limit=5
	deps.mlLinkListItemRepo.On("GetNextUnconsumed", mock.Anything, listID, 5).
		Return([]*models.MlLinkListItem{}, nil)
	deps.dispatchLogRepo.On("Create", mock.Anything, mock.Anything).Return(&models.DispatchLog{}, nil).Maybe()

	result, err := svc.processListMode(context.Background(), automation)
	require.NoError(t, err)
	assert.Equal(t, 0, result.OffersFound)
	deps.mlLinkListItemRepo.AssertCalled(t, "GetNextUnconsumed", mock.Anything, listID, 5)
}

func TestProcessListMode_ItemsPerRunCapped(t *testing.T) {
	t.Parallel()
	svc, deps := newInternalTestService(t)

	listID := "list-123"
	itemsPerRun := 50 // Exceeds max of 20
	automation := &models.Automation{
		ID:              "auto-1",
		UserID:          "user-1",
		Name:            "List Test",
		MlLinkListID:    &listID,
		GroupIDs:        []string{"group@g.us"},
		ListItemsPerRun: &itemsPerRun,
	}

	deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
		Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)
	// Expect GetNextUnconsumed to be called with limit=20 (capped)
	deps.mlLinkListItemRepo.On("GetNextUnconsumed", mock.Anything, listID, 20).
		Return([]*models.MlLinkListItem{}, nil)
	deps.dispatchLogRepo.On("Create", mock.Anything, mock.Anything).Return(&models.DispatchLog{}, nil).Maybe()

	result, err := svc.processListMode(context.Background(), automation)
	require.NoError(t, err)
	deps.mlLinkListItemRepo.AssertCalled(t, "GetNextUnconsumed", mock.Anything, listID, 20)
	_ = result
}

func TestProcessListMode_ImageFromItemWhenNoPreview(t *testing.T) {
	t.Parallel()
	svc, deps := newInternalTestService(t)

	listID := "list-123"
	includeImage := true
	imageURL := "https://img.test/fallback.jpg"
	automation := &models.Automation{
		ID:               "auto-1",
		UserID:           "user-1",
		Name:             "List Test",
		MlLinkListID:     &listID,
		GroupIDs:         []string{"group@g.us"},
		ListIncludeImage: &includeImage,
	}

	deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
		Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)
	deps.mlLinkListItemRepo.On("GetNextUnconsumed", mock.Anything, listID, 1).
		Return([]*models.MlLinkListItem{
			{
				ID:              "item-1",
				OriginURL:       "https://produto.mercadolivre.com.br/MLB-123",
				ListID:          listID,
				ProductImageURL: &imageURL,
			},
		}, nil)
	deps.mlLinkListItemRepo.On("MarkAsConsumed", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	deps.dispatchLogRepo.On("Create", mock.Anything, mock.Anything).Return(&models.DispatchLog{}, nil).Maybe()

	// mlService is nil in newInternalTestService, so preview will be nil
	// The fallback to item.ProductImageURL should be used
	result, err := svc.processListMode(context.Background(), automation)
	require.NoError(t, err)
	_ = result
}

func TestProcessListMode_NoConnectedInstance(t *testing.T) {
	t.Parallel()
	svc, deps := newInternalTestService(t)

	listID := "list-123"
	automation := &models.Automation{
		ID:           "auto-1",
		UserID:       "user-1",
		Name:         "List Test",
		MlLinkListID: &listID,
		GroupIDs:     []string{"group@g.us"},
	}

	deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
		Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "disconnected"}}, nil)
	deps.dispatchLogRepo.On("Create", mock.Anything, mock.Anything).Return(&models.DispatchLog{}, nil).Maybe()

	result, err := svc.processListMode(context.Background(), automation)
	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Message, "WhatsApp")
}

func TestProcessListMode_WhitespaceListID(t *testing.T) {
	t.Parallel()
	svc, deps := newInternalTestService(t)

	listID := "   " // whitespace-only
	automation := &models.Automation{
		ID:           "auto-1",
		UserID:       "user-1",
		Name:         "List Test",
		MlLinkListID: &listID,
		GroupIDs:     []string{"group@g.us"},
	}

	deps.dispatchLogRepo.On("Create", mock.Anything, mock.Anything).Return(&models.DispatchLog{}, nil).Maybe()

	result, err := svc.processListMode(context.Background(), automation)
	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Message, "Lista de Links")
}

func TestProcessListMode_GetItemsError(t *testing.T) {
	t.Parallel()
	svc, deps := newInternalTestService(t)

	listID := "list-123"
	automation := &models.Automation{
		ID:           "auto-1",
		UserID:       "user-1",
		Name:         "List Test",
		MlLinkListID: &listID,
		GroupIDs:     []string{"group@g.us"},
	}

	deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
		Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)
	deps.mlLinkListItemRepo.On("GetNextUnconsumed", mock.Anything, listID, 1).
		Return(nil, fmt.Errorf("db error"))

	_, err := svc.processListMode(context.Background(), automation)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unconsumed")
}

// =============================================================================
// acquireExecutionLock Tests (93.8% → 100%)
// =============================================================================

type mockInternalRunRepo struct {
	mock.Mock
}

func (m *mockInternalRunRepo) GetLock(ctx context.Context, automationID string) (*time.Time, error) {
	args := m.Called(ctx, automationID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*time.Time), args.Error(1)
}

func (m *mockInternalRunRepo) AcquireLock(ctx context.Context, automationID string, duration time.Duration) (*time.Time, error) {
	args := m.Called(ctx, automationID, duration)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*time.Time), args.Error(1)
}

func (m *mockInternalRunRepo) ReleaseLock(ctx context.Context, automationID string) error {
	args := m.Called(ctx, automationID)
	return args.Error(0)
}

func (m *mockInternalRunRepo) CreateRun(ctx context.Context, automationID string, status string) (string, error) {
	args := m.Called(ctx, automationID, status)
	return args.String(0), args.Error(1)
}

func (m *mockInternalRunRepo) UpdateRunStatus(ctx context.Context, runID string, status string, errorMsg *string) error {
	args := m.Called(ctx, runID, status, errorMsg)
	return args.Error(0)
}

func (m *mockInternalRunRepo) UpdateRunStats(ctx context.Context, runID string, offersFound, messagesSent, messagesFailed int) error {
	args := m.Called(ctx, runID, offersFound, messagesSent, messagesFailed)
	return args.Error(0)
}

func (m *mockInternalRunRepo) GetRunByID(ctx context.Context, runID string) (*models.AutomationRun, error) {
	args := m.Called(ctx, runID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.AutomationRun), args.Error(1)
}

func (m *mockInternalRunRepo) GetRunsByAutomationID(ctx context.Context, automationID string, limit int) ([]models.AutomationRun, error) {
	args := m.Called(ctx, automationID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.AutomationRun), args.Error(1)
}

func (m *mockInternalRunRepo) CleanupOldRuns(ctx context.Context, olderThan time.Duration) (int64, error) {
	args := m.Called(ctx, olderThan)
	return args.Get(0).(int64), args.Error(1)
}

func TestAcquireExecutionLock_NilRepo(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	svc := &automationService{logger: logger}

	lockTime, err := svc.acquireExecutionLock(context.Background(), "auto-1")
	assert.NoError(t, err)
	assert.Nil(t, lockTime)
}

func TestAcquireExecutionLock_LockHeld(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	runRepo := new(mockInternalRunRepo)

	existingLock := time.Now().Add(5 * time.Minute)
	runRepo.On("GetLock", mock.Anything, "auto-1").Return(&existingLock, nil)

	svc := &automationService{
		automationRunRepo: runRepo,
		logger:            logger,
	}

	lockTime, err := svc.acquireExecutionLock(context.Background(), "auto-1")
	assert.Error(t, err)
	assert.Nil(t, lockTime)
	assert.Contains(t, err.Error(), "lock")
}

func TestAcquireExecutionLock_AcquireError(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	runRepo := new(mockInternalRunRepo)

	runRepo.On("GetLock", mock.Anything, "auto-1").Return(nil, nil)
	runRepo.On("AcquireLock", mock.Anything, "auto-1", mock.AnythingOfType("time.Duration")).Return(nil, errors.New("db error"))

	svc := &automationService{
		automationRunRepo: runRepo,
		logger:            logger,
	}

	lockTime, err := svc.acquireExecutionLock(context.Background(), "auto-1")
	assert.Error(t, err)
	assert.Nil(t, lockTime)
	assert.Contains(t, err.Error(), "failed to acquire lock")
}

func TestAcquireExecutionLock_RaceCondition_Internal(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	runRepo := new(mockInternalRunRepo)

	// GetLock returns nil (no existing lock), but AcquireLock returns nil (race condition)
	runRepo.On("GetLock", mock.Anything, "auto-1").Return(nil, nil)
	runRepo.On("AcquireLock", mock.Anything, "auto-1", mock.AnythingOfType("time.Duration")).Return(nil, nil)

	svc := &automationService{
		automationRunRepo: runRepo,
		logger:            logger,
	}

	lockTime, err := svc.acquireExecutionLock(context.Background(), "auto-1")
	assert.Error(t, err)
	assert.Nil(t, lockTime)
}

func TestAcquireExecutionLock_RaceConditionWithExistingLock(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	runRepo := new(mockInternalRunRepo)

	existingLock := time.Now().Add(5 * time.Minute)

	// First GetLock: no lock, AcquireLock: nil (race), second GetLock: lock found
	runRepo.On("GetLock", mock.Anything, "auto-1").Return(nil, nil).Once()
	runRepo.On("AcquireLock", mock.Anything, "auto-1", mock.AnythingOfType("time.Duration")).Return(nil, nil)
	runRepo.On("GetLock", mock.Anything, "auto-1").Return(&existingLock, nil).Once()

	svc := &automationService{
		automationRunRepo: runRepo,
		logger:            logger,
	}

	lockTime, err := svc.acquireExecutionLock(context.Background(), "auto-1")
	assert.Error(t, err)
	assert.Nil(t, lockTime)
	assert.Contains(t, err.Error(), "lock")
}

// =============================================================================
// processListMode - Additional Gap Coverage
// =============================================================================

func TestProcessListMode_MarkConsumedError(t *testing.T) {
	t.Parallel()
	svc, deps := newInternalTestService(t)

	listID := "list-123"
	automation := &models.Automation{
		ID:           "auto-1",
		UserID:       "user-1",
		Name:         "List Test",
		MlLinkListID: &listID,
		GroupIDs:     []string{"group@g.us"},
	}

	deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
		Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)
	deps.mlLinkListItemRepo.On("GetNextUnconsumed", mock.Anything, listID, 1).
		Return([]*models.MlLinkListItem{
			{ID: "item-1", OriginURL: "https://produto.mercadolivre.com.br/MLB-123", ListID: listID},
		}, nil)
	deps.mlLinkListItemRepo.On("MarkAsConsumed", mock.Anything, []string{"item-1"}, "auto-1", mock.AnythingOfType("*string")).
		Return(errors.New("db error")) // Error is logged, not returned
	deps.dispatchLogRepo.On("Create", mock.Anything, mock.Anything).Return(&models.DispatchLog{}, nil).Maybe()

	result, err := svc.processListMode(context.Background(), automation)
	require.NoError(t, err)
	assert.Equal(t, 1, result.MessagesSent) // Message still sent despite consume error
}

func TestProcessListMode_IncludeImageFalse(t *testing.T) {
	t.Parallel()
	svc, deps := newInternalTestService(t)

	listID := "list-123"
	includeImage := false
	imageURL := "https://img.test/product.jpg"
	automation := &models.Automation{
		ID:               "auto-1",
		UserID:           "user-1",
		Name:             "List Test",
		MlLinkListID:     &listID,
		GroupIDs:         []string{"group@g.us"},
		ListIncludeImage: &includeImage,
	}

	deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
		Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)
	deps.mlLinkListItemRepo.On("GetNextUnconsumed", mock.Anything, listID, 1).
		Return([]*models.MlLinkListItem{
			{ID: "item-1", OriginURL: "https://produto.mercadolivre.com.br/MLB-123", ListID: listID, ProductImageURL: &imageURL},
		}, nil)
	deps.mlLinkListItemRepo.On("MarkAsConsumed", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	deps.dispatchLogRepo.On("Create", mock.Anything, mock.Anything).Return(&models.DispatchLog{}, nil).Maybe()

	result, err := svc.processListMode(context.Background(), automation)
	require.NoError(t, err)
	assert.Equal(t, 1, result.MessagesSent)
}

func TestProcessListMode_MercadolibreURL(t *testing.T) {
	t.Parallel()
	svc, deps := newInternalTestService(t)

	listID := "list-123"
	automation := &models.Automation{
		ID:           "auto-1",
		UserID:       "user-1",
		Name:         "List Test",
		MlLinkListID: &listID,
		GroupIDs:     []string{"group@g.us"},
	}

	deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
		Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)
	deps.mlLinkListItemRepo.On("GetNextUnconsumed", mock.Anything, listID, 1).
		Return([]*models.MlLinkListItem{
			{ID: "item-1", OriginURL: "https://producto.mercadolibre.com.ar/MLA-456", ListID: listID},
		}, nil)
	deps.mlLinkListItemRepo.On("MarkAsConsumed", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	deps.dispatchLogRepo.On("Create", mock.Anything, mock.Anything).Return(&models.DispatchLog{}, nil).Maybe()

	result, err := svc.processListMode(context.Background(), automation)
	require.NoError(t, err)
	assert.Equal(t, 1, result.MessagesSent) // mercadolibre. URL is valid
}

func TestProcessListMode_NoGroups(t *testing.T) {
	t.Parallel()
	svc, deps := newInternalTestService(t)

	listID := "list-123"
	automation := &models.Automation{
		ID:           "auto-1",
		UserID:       "user-1",
		Name:         "List Test",
		MlLinkListID: &listID,
		GroupIDs:     []string{}, // Empty groups
	}

	deps.dispatchLogRepo.On("Create", mock.Anything, mock.Anything).Return(&models.DispatchLog{}, nil).Maybe()

	result, err := svc.processListMode(context.Background(), automation)
	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Message, "Nenhum grupo de envio")
}

// =============================================================================
// processMonitorMode - Additional Gap Coverage
// =============================================================================

func TestProcessMonitorMode_ValidMonitorInvalidTarget(t *testing.T) {
	t.Parallel()
	svc, deps := newInternalTestService(t)

	automation := &models.Automation{
		ID:              "auto-1",
		UserID:          "user-1",
		Name:            "Monitor Test",
		MonitorGroupIDs: []string{"source@g.us"},
		GroupIDs:        []string{"invalid-target"}, // No @g.us
	}

	deps.dispatchLogRepo.On("Create", mock.Anything, mock.Anything).Return(&models.DispatchLog{}, nil).Maybe()

	result, err := svc.processMonitorMode(context.Background(), automation)
	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Message, "grupo de destino valido")
}

func TestProcessMonitorMode_NoConnectedInstance(t *testing.T) {
	t.Parallel()
	svc, deps := newInternalTestService(t)

	automation := &models.Automation{
		ID:              "auto-1",
		UserID:          "user-1",
		Name:            "Monitor Test",
		MonitorGroupIDs: []string{"source@g.us"},
		GroupIDs:        []string{"target@g.us"},
	}

	deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
		Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "disconnected"}}, nil)
	deps.dispatchLogRepo.On("Create", mock.Anything, mock.Anything).Return(&models.DispatchLog{}, nil).Maybe()

	result, err := svc.processMonitorMode(context.Background(), automation)
	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Message, "WhatsApp")
}

func TestProcessMonitorMode_WithTimeWindowAndScheduleDays(t *testing.T) {
	t.Parallel()
	svc, deps := newInternalTestService(t)

	startTime := "03:00"
	endTime := "03:01"
	scheduleDays := []string{"0", "1", "2", "3", "4", "5", "6"}
	automation := &models.Automation{
		ID:              "auto-1",
		UserID:          "user-1",
		Name:            "Monitor Test",
		MonitorGroupIDs: []string{"source@g.us"},
		GroupIDs:        []string{"target@g.us"},
		Stores:          []string{"shopee", "mercadolivre"},
		StartTime:       &startTime,
		EndTime:         &endTime,
		ScheduleDays:    scheduleDays,
	}

	deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
		Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)
	deps.dispatchLogRepo.On("Create", mock.Anything, mock.Anything).Return(&models.DispatchLog{}, nil).Maybe()

	result, err := svc.processMonitorMode(context.Background(), automation)
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Contains(t, result.Message, "Monitoramento ativo")
	assert.Contains(t, result.Message, "Lojas: shopee, mercadolivre")
}

func TestProcessMonitorMode_DefaultStores(t *testing.T) {
	t.Parallel()
	svc, deps := newInternalTestService(t)

	automation := &models.Automation{
		ID:              "auto-1",
		UserID:          "user-1",
		Name:            "Monitor Test",
		MonitorGroupIDs: []string{"source@g.us"},
		GroupIDs:        []string{"target@g.us"},
		Stores:          []string{}, // Empty → defaults to shopee
	}

	deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
		Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)
	deps.dispatchLogRepo.On("Create", mock.Anything, mock.Anything).Return(&models.DispatchLog{}, nil).Maybe()

	result, err := svc.processMonitorMode(context.Background(), automation)
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Contains(t, result.Message, "Lojas: shopee")
}

// =============================================================================
// Mock Shopee & ML Services for internal tests
// =============================================================================

type mockShopeeService struct {
	mock.Mock
}

func (m *mockShopeeService) SearchOffers(ctx context.Context, userID string, params models.ShopeeSearchParams) (*models.ShopeeSearchResult, error) {
	args := m.Called(ctx, userID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ShopeeSearchResult), args.Error(1)
}

func (m *mockShopeeService) GenerateAffiliateLink(ctx context.Context, productURL, userID string) (*models.ShopeeAffiliateLink, error) {
	args := m.Called(ctx, productURL, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ShopeeAffiliateLink), args.Error(1)
}

func (m *mockShopeeService) GetCategories(ctx context.Context, parentID *int64) ([]models.ShopeeCategory, error) {
	args := m.Called(ctx, parentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.ShopeeCategory), args.Error(1)
}

func (m *mockShopeeService) GetProduct(ctx context.Context, itemID, shopID int64, userID string) (*models.ShopeeOffer, error) {
	args := m.Called(ctx, itemID, shopID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ShopeeOffer), args.Error(1)
}

func (m *mockShopeeService) ExpandURL(ctx context.Context, shortURL string) (string, error) {
	args := m.Called(ctx, shortURL)
	return args.String(0), args.Error(1)
}

func (m *mockShopeeService) ExtractProductIDs(productURL string) (int64, int64, error) {
	args := m.Called(productURL)
	return args.Get(0).(int64), args.Get(1).(int64), args.Error(2)
}

type mockMLService struct {
	mock.Mock
}

func (m *mockMLService) CreateAffiliateLink(ctx context.Context, originURL, userID string) (*models.MLAffiliateLink, error) {
	args := m.Called(ctx, originURL, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.MLAffiliateLink), args.Error(1)
}

func (m *mockMLService) GetProductPreview(ctx context.Context, originURL, userID string) (*models.MLProductPreview, error) {
	args := m.Called(ctx, originURL, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.MLProductPreview), args.Error(1)
}

func (m *mockMLService) LoadCredentials(ctx context.Context, userID string) (*models.MLCredentials, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.MLCredentials), args.Error(1)
}

func (m *mockMLService) SaveCredentials(ctx context.Context, userID, cookie, csrf, tag string) error {
	args := m.Called(ctx, userID, cookie, csrf, tag)
	return args.Error(0)
}

func (m *mockMLService) IsConnected(ctx context.Context, userID string) (bool, error) {
	args := m.Called(ctx, userID)
	return args.Bool(0), args.Error(1)
}

func (m *mockMLService) ExtractProductID(productURL string) (string, error) {
	args := m.Called(productURL)
	return args.String(0), args.Error(1)
}

func (m *mockMLService) InvalidatePreviewCache(ctx context.Context, originURL string) error {
	args := m.Called(ctx, originURL)
	return args.Error(0)
}

// =============================================================================
// Helper: create test cache with miniredis
// =============================================================================

func newTestCache(t *testing.T) (*cache.Cache, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rc.Close() })
	return cache.NewCache(rc, "test:", false), mr
}

// =============================================================================
// isDuplicate with real cache (covers cache-hit paths: lines 828-833)
// =============================================================================

func TestIsDuplicate_CacheHit(t *testing.T) {
	t.Parallel()

	t.Run("cache hit returns true immediately (skips DB)", func(t *testing.T) {
		t.Parallel()
		logger := zaptest.NewLogger(t)
		c, _ := newTestCache(t)

		// Pre-populate cache
		err := c.Set(context.Background(), "dedup:user-1:group-1:key-1", "1", time.Hour)
		require.NoError(t, err)

		svc := &automationService{
			cache:  c,
			logger: logger,
			// dispatchLogRepo is nil → proves DB is never consulted
		}

		result := svc.isDuplicate(context.Background(), "user-1", "group-1", "key-1", 24)
		assert.True(t, result)
	})

	t.Run("cache miss falls through to DB", func(t *testing.T) {
		t.Parallel()
		logger := zaptest.NewLogger(t)
		c, _ := newTestCache(t)
		dlRepo := new(mockDispatchLogRepo)

		dlRepo.On("GetRecentByCanonicalKey", mock.Anything, "user-1", "group-1", "key-1", 24).
			Return(nil, nil)

		svc := &automationService{
			cache:           c,
			dispatchLogRepo: dlRepo,
			logger:          logger,
		}

		result := svc.isDuplicate(context.Background(), "user-1", "group-1", "key-1", 24)
		assert.False(t, result)
		dlRepo.AssertCalled(t, "GetRecentByCanonicalKey", mock.Anything, "user-1", "group-1", "key-1", 24)
	})

	t.Run("cache miss, DB hit backfills cache", func(t *testing.T) {
		t.Parallel()
		logger := zaptest.NewLogger(t)
		c, _ := newTestCache(t)
		dlRepo := new(mockDispatchLogRepo)

		dlRepo.On("GetRecentByCanonicalKey", mock.Anything, "user-1", "group-1", "key-1", 24).
			Return(&models.DispatchLog{Status: "queued"}, nil)

		svc := &automationService{
			cache:           c,
			dispatchLogRepo: dlRepo,
			logger:          logger,
		}

		result := svc.isDuplicate(context.Background(), "user-1", "group-1", "key-1", 24)
		assert.True(t, result)

		// Verify cache was backfilled
		exists, err := c.Exists(context.Background(), "dedup:user-1:group-1:key-1")
		require.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("cache error falls through to DB", func(t *testing.T) {
		t.Parallel()
		logger := zaptest.NewLogger(t)
		c, mr := newTestCache(t)
		dlRepo := new(mockDispatchLogRepo)

		// Close miniredis to force cache error
		mr.Close()

		dlRepo.On("GetRecentByCanonicalKey", mock.Anything, "user-1", "group-1", "key-1", 24).
			Return(nil, nil)

		svc := &automationService{
			cache:           c,
			dispatchLogRepo: dlRepo,
			logger:          logger,
		}

		result := svc.isDuplicate(context.Background(), "user-1", "group-1", "key-1", 24)
		assert.False(t, result)
		dlRepo.AssertCalled(t, "GetRecentByCanonicalKey", mock.Anything, "user-1", "group-1", "key-1", 24)
	})
}

// =============================================================================
// markAsSent with real cache (covers cache-set paths: lines 867-872)
// =============================================================================

func TestMarkAsSent_WithCache(t *testing.T) {
	t.Parallel()

	t.Run("sets cache key successfully", func(t *testing.T) {
		t.Parallel()
		logger := zaptest.NewLogger(t)
		c, _ := newTestCache(t)

		svc := &automationService{
			cache:  c,
			logger: logger,
		}

		svc.markAsSent(context.Background(), "user-1", "group-1", "key-1", 24)

		// Verify the key was set in cache
		exists, err := c.Exists(context.Background(), "dedup:user-1:group-1:key-1")
		require.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("cache error is handled gracefully (fail-open)", func(t *testing.T) {
		t.Parallel()
		logger := zaptest.NewLogger(t)
		c, mr := newTestCache(t)

		// Close miniredis to force cache error
		mr.Close()

		svc := &automationService{
			cache:  c,
			logger: logger,
		}

		// Should not panic even with cache error
		svc.markAsSent(context.Background(), "user-1", "group-1", "key-1", 24)
	})
}

// =============================================================================
// executeSearchAutomation tests (covers shopee search + dispatch loop)
// =============================================================================

func TestExecuteSearchAutomation(t *testing.T) {
	// Not parallel: some subtests use t.Setenv

	t.Run("no target groups returns error", func(t *testing.T) {
		t.Parallel()
		svc, _ := newInternalTestService(t)

		automation := &models.Automation{
			ID:       "auto-1",
			UserID:   "user-1",
			GroupIDs: []string{}, // no groups
		}

		result, err := svc.executeSearchAutomation(context.Background(), automation, "hist-1")
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "no target groups")
	})

	t.Run("no instances returns error", func(t *testing.T) {
		t.Parallel()
		svc, deps := newInternalTestService(t)

		automation := &models.Automation{
			ID:       "auto-1",
			UserID:   "user-1",
			GroupIDs: []string{"group@g.us"},
		}

		deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
			Return([]models.Instance{}, nil)

		result, err := svc.executeSearchAutomation(context.Background(), automation, "hist-1")
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "no WhatsApp instance")
	})

	t.Run("no connected instance returns error", func(t *testing.T) {
		t.Parallel()
		svc, deps := newInternalTestService(t)

		automation := &models.Automation{
			ID:       "auto-1",
			UserID:   "user-1",
			GroupIDs: []string{"group@g.us"},
		}

		deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
			Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "disconnected"}}, nil)

		result, err := svc.executeSearchAutomation(context.Background(), automation, "hist-1")
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "no connected WhatsApp instance")
	})

	t.Run("no offers found returns success with zero offers", func(t *testing.T) {
		t.Parallel()
		svc, deps := newInternalTestService(t)
		shopee := new(mockShopeeService)
		svc.shopeeService = shopee

		keyword := "test"
		automation := &models.Automation{
			ID:       "auto-1",
			UserID:   "user-1",
			GroupIDs: []string{"group@g.us"},
			Stores:   []string{"shopee"},
			Keyword:  &keyword,
		}

		deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
			Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)
		shopee.On("SearchOffers", mock.Anything, "user-1", mock.Anything).
			Return(&models.ShopeeSearchResult{Offers: []models.ShopeeOffer{}}, nil)

		result, err := svc.executeSearchAutomation(context.Background(), automation, "hist-1")
		require.NoError(t, err)
		assert.True(t, result.Success)
		assert.Equal(t, 0, result.OffersFound)
	})

	t.Run("shopee search error is handled gracefully", func(t *testing.T) {
		t.Parallel()
		svc, deps := newInternalTestService(t)
		shopee := new(mockShopeeService)
		svc.shopeeService = shopee

		keyword := "test"
		automation := &models.Automation{
			ID:       "auto-1",
			UserID:   "user-1",
			GroupIDs: []string{"group@g.us"},
			Stores:   []string{"shopee"},
			Keyword:  &keyword,
		}

		deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
			Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)
		shopee.On("SearchOffers", mock.Anything, "user-1", mock.Anything).
			Return(nil, errors.New("shopee API error"))

		result, err := svc.executeSearchAutomation(context.Background(), automation, "hist-1")
		require.NoError(t, err)
		assert.True(t, result.Success)
		assert.Equal(t, 0, result.OffersFound)
	})

	t.Run("offer sent successfully with dedup and dispatch", func(t *testing.T) {
		// Not parallel: uses t.Setenv for feature flags
		svc, deps := newInternalTestService(t)
		shopee := new(mockShopeeService)
		svc.shopeeService = shopee

		t.Setenv("FEATURE_USE_GO_DISPATCH_LOGS", "true")
		config.ResetFeatureFlags()

		keyword := "test"
		automation := &models.Automation{
			ID:       "auto-1",
			UserID:   "user-1",
			GroupIDs: []string{"group@g.us"},
			Stores:   []string{"shopee"},
			Keyword:  &keyword,
		}

		deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
			Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)

		offers := []models.ShopeeOffer{
			{Name: "Product 1", ProductURL: "https://shopee.com/p1", Price: 99.90, OriginalPrice: 199.90, Discount: 50, Image: "https://img.com/1.jpg", ItemID: 123, ShopID: 456},
		}
		shopee.On("SearchOffers", mock.Anything, "user-1", mock.Anything).
			Return(&models.ShopeeSearchResult{Offers: offers}, nil)
		shopee.On("GetProduct", mock.Anything, int64(123), int64(456), "user-1").
			Return(&models.ShopeeOffer{Price: 89.90, OriginalPrice: 199.90}, nil)
		shopee.On("GenerateAffiliateLink", mock.Anything, "https://shopee.com/p1", "user-1").
			Return(&models.ShopeeAffiliateLink{AffiliateLink: "https://s.shopee.com/aff1"}, nil)

		// dispatchLogRepo for dedup check (no duplicate) and logging
		deps.dispatchLogRepo.On("GetRecentByCanonicalKey", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(nil, nil)
		deps.dispatchLogRepo.On("Create", mock.Anything, mock.Anything).Return(&models.DispatchLog{}, nil).Maybe()

		result, err := svc.executeSearchAutomation(context.Background(), automation, "hist-1")
		require.NoError(t, err)
		assert.True(t, result.Success)
		assert.Equal(t, 1, result.OffersFound)
		assert.Equal(t, 1, result.MessagesSent)
	})

	t.Run("all offers are duplicates", func(t *testing.T) {
		// Not parallel: uses t.Setenv for feature flags
		svc, deps := newInternalTestService(t)
		shopee := new(mockShopeeService)
		svc.shopeeService = shopee

		t.Setenv("FEATURE_USE_GO_DISPATCH_LOGS", "true")
		config.ResetFeatureFlags()

		keyword := "test"
		automation := &models.Automation{
			ID:       "auto-1",
			UserID:   "user-1",
			GroupIDs: []string{"group@g.us"},
			Stores:   []string{"shopee"},
			Keyword:  &keyword,
		}

		deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
			Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)

		offers := []models.ShopeeOffer{
			{Name: "Product 1", ProductURL: "https://shopee.com/p1", Price: 99.90, OriginalPrice: 199.90, Discount: 50},
		}
		shopee.On("SearchOffers", mock.Anything, "user-1", mock.Anything).
			Return(&models.ShopeeSearchResult{Offers: offers}, nil)

		// All groups show duplicate
		deps.dispatchLogRepo.On("GetRecentByCanonicalKey", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(&models.DispatchLog{Status: "queued"}, nil)
		deps.dispatchLogRepo.On("Create", mock.Anything, mock.Anything).Return(&models.DispatchLog{}, nil).Maybe()

		result, err := svc.executeSearchAutomation(context.Background(), automation, "hist-1")
		require.NoError(t, err)
		assert.True(t, result.Success)
		assert.Equal(t, 0, result.MessagesSent)
	})

	t.Run("uses fallback keyword when Texts is empty", func(t *testing.T) {
		t.Parallel()
		svc, deps := newInternalTestService(t)
		shopee := new(mockShopeeService)
		svc.shopeeService = shopee

		automation := &models.Automation{
			ID:       "auto-1",
			UserID:   "user-1",
			GroupIDs: []string{"group@g.us"},
			Stores:   []string{"shopee"},
			Texts:    nil,   // empty texts
			Keyword:  nil,   // nil keyword → broad search
		}

		deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
			Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)
		shopee.On("SearchOffers", mock.Anything, "user-1", mock.Anything).
			Return(&models.ShopeeSearchResult{Offers: []models.ShopeeOffer{}}, nil)

		result, err := svc.executeSearchAutomation(context.Background(), automation, "")
		require.NoError(t, err)
		assert.Equal(t, 0, result.OffersFound)
	})

	t.Run("mercadolivre store is handled without error", func(t *testing.T) {
		t.Parallel()
		svc, deps := newInternalTestService(t)

		automation := &models.Automation{
			ID:       "auto-1",
			UserID:   "user-1",
			GroupIDs: []string{"group@g.us"},
			Stores:   []string{"mercadolivre"},
		}

		deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
			Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)

		result, err := svc.executeSearchAutomation(context.Background(), automation, "")
		require.NoError(t, err)
		assert.True(t, result.Success)
		assert.Equal(t, 0, result.OffersFound)
	})

	t.Run("dispatch text template from settings", func(t *testing.T) {
		// Not parallel: uses t.Setenv
		svc, deps := newInternalTestService(t)
		shopee := new(mockShopeeService)
		svc.shopeeService = shopee

		t.Setenv("FEATURE_USE_GO_DISPATCH_LOGS", "true")
		config.ResetFeatureFlags()

		dtID := "dt-1"
		keyword := "test"
		automation := &models.Automation{
			ID:              "auto-1",
			UserID:          "user-1",
			GroupIDs:        []string{"group@g.us"},
			Stores:          []string{"shopee"},
			Keyword:         &keyword,
			DispatchTextID:  &dtID,
		}

		deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
			Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)

		offers := []models.ShopeeOffer{
			{Name: "Product 1", ProductURL: "https://shopee.com/p1", Price: 50.0, OriginalPrice: 100.0, Discount: 50},
		}
		shopee.On("SearchOffers", mock.Anything, "user-1", mock.Anything).
			Return(&models.ShopeeSearchResult{Offers: offers}, nil)

		deps.settingsRepo.On("GetActiveDispatchTextByID", mock.Anything, "dt-1", "user-1").
			Return(&models.DispatchText{Body: "Custom: {{PRODUTO_NOME}} - {{PRODUTO_PRECO}}"}, nil)

		shopee.On("GenerateAffiliateLink", mock.Anything, mock.Anything, "user-1").
			Return(&models.ShopeeAffiliateLink{AffiliateLink: "https://s.shopee.com/aff1"}, nil)

		deps.dispatchLogRepo.On("GetRecentByCanonicalKey", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(nil, nil)
		deps.dispatchLogRepo.On("Create", mock.Anything, mock.Anything).Return(&models.DispatchLog{}, nil).Maybe()

		result, err := svc.executeSearchAutomation(context.Background(), automation, "hist-1")
		require.NoError(t, err)
		assert.Equal(t, 1, result.MessagesSent)
	})

	t.Run("offer with refetch error uses search data", func(t *testing.T) {
		// Not parallel: uses t.Setenv
		svc, deps := newInternalTestService(t)
		shopee := new(mockShopeeService)
		svc.shopeeService = shopee

		t.Setenv("FEATURE_USE_GO_DISPATCH_LOGS", "true")
		config.ResetFeatureFlags()

		keyword := "test"
		automation := &models.Automation{
			ID:       "auto-1",
			UserID:   "user-1",
			GroupIDs: []string{"group@g.us"},
			Stores:   []string{"shopee"},
			Keyword:  &keyword,
		}

		deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
			Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)

		offers := []models.ShopeeOffer{
			{Name: "Product 1", ProductURL: "https://shopee.com/p1", Price: 99.90, OriginalPrice: 199.90, Discount: 50, ItemID: 123, ShopID: 456},
		}
		shopee.On("SearchOffers", mock.Anything, "user-1", mock.Anything).
			Return(&models.ShopeeSearchResult{Offers: offers}, nil)
		shopee.On("GetProduct", mock.Anything, int64(123), int64(456), "user-1").
			Return(nil, errors.New("refetch failed"))
		shopee.On("GenerateAffiliateLink", mock.Anything, mock.Anything, "user-1").
			Return(nil, errors.New("affiliate link failed"))

		deps.dispatchLogRepo.On("GetRecentByCanonicalKey", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(nil, nil)
		deps.dispatchLogRepo.On("Create", mock.Anything, mock.Anything).Return(&models.DispatchLog{}, nil).Maybe()

		result, err := svc.executeSearchAutomation(context.Background(), automation, "hist-1")
		require.NoError(t, err)
		assert.Equal(t, 1, result.MessagesSent)
	})

	t.Run("per-group dedup skips some groups", func(t *testing.T) {
		// Not parallel: uses t.Setenv
		svc, deps := newInternalTestService(t)
		shopee := new(mockShopeeService)
		svc.shopeeService = shopee

		t.Setenv("FEATURE_USE_GO_DISPATCH_LOGS", "true")
		config.ResetFeatureFlags()

		keyword := "test"
		automation := &models.Automation{
			ID:       "auto-1",
			UserID:   "user-1",
			GroupIDs: []string{"group1@g.us", "group2@g.us"},
			Stores:   []string{"shopee"},
			Keyword:  &keyword,
		}

		deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
			Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)

		offers := []models.ShopeeOffer{
			{Name: "Product 1", ProductURL: "https://shopee.com/p1", Price: 99.90, OriginalPrice: 199.90, Discount: 50},
		}
		shopee.On("SearchOffers", mock.Anything, "user-1", mock.Anything).
			Return(&models.ShopeeSearchResult{Offers: offers}, nil)

		// group1 = not duplicate, group2 = not duplicate (both pass allGroupsDuped check)
		deps.dispatchLogRepo.On("GetRecentByCanonicalKey", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(nil, nil)
		deps.dispatchLogRepo.On("Create", mock.Anything, mock.Anything).Return(&models.DispatchLog{}, nil).Maybe()

		shopee.On("GenerateAffiliateLink", mock.Anything, mock.Anything, "user-1").
			Return(&models.ShopeeAffiliateLink{AffiliateLink: "https://s.shopee.com/aff1"}, nil)

		result, err := svc.executeSearchAutomation(context.Background(), automation, "hist-1")
		require.NoError(t, err)
		assert.Equal(t, 2, result.MessagesSent)
	})

	t.Run("empty stores defaults to shopee", func(t *testing.T) {
		t.Parallel()
		svc, deps := newInternalTestService(t)
		shopee := new(mockShopeeService)
		svc.shopeeService = shopee

		automation := &models.Automation{
			ID:       "auto-1",
			UserID:   "user-1",
			GroupIDs: []string{"group@g.us"},
			Stores:   []string{}, // empty → defaults to shopee
		}

		deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
			Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)
		shopee.On("SearchOffers", mock.Anything, "user-1", mock.Anything).
			Return(&models.ShopeeSearchResult{Offers: []models.ShopeeOffer{}}, nil)

		result, err := svc.executeSearchAutomation(context.Background(), automation, "")
		require.NoError(t, err)
		assert.Equal(t, 0, result.OffersFound)
		shopee.AssertCalled(t, "SearchOffers", mock.Anything, "user-1", mock.Anything)
	})
}

// =============================================================================
// buildOfferMessage tests (cover coupon path: line 784-799)
// =============================================================================

func TestBuildOfferMessage_WithCoupon(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	svc := &automationService{logger: logger}

	offer := &models.ShopeeOffer{
		Name:          "Product A",
		Price:         50.0,
		OriginalPrice: 100.0,
		Discount:      50,
		ProductURL:    "https://shopee.com/p1",
		AffiliateLink: "https://s.shopee.com/aff1",
		Image:         "https://img.com/1.jpg",
	}

	automation := &models.Automation{
		ID:     "auto-1",
		UserID: "user-1",
		CTAs:   []string{"Compre agora!", "Aproveite!"},
	}

	template := "{{PRODUTO_NOME}}\n{{LINK_AFILIADO}}\n{{CTA}}"
	message := svc.buildOfferMessage(template, offer, automation, nil)
	assert.Contains(t, message, "Product A")
	assert.Contains(t, message, "https://s.shopee.com/aff1")
}

// =============================================================================
// processListMode advanced tests (ML URL, image handling, mark consumed)
// =============================================================================

func TestProcessListMode_MLServiceIntegration(t *testing.T) {
	// Not parallel: subtests use t.Setenv

	t.Run("ML preview and affiliate link generation", func(t *testing.T) {
		// Not parallel: uses t.Setenv
		svc, deps := newInternalTestService(t)
		mlSvc := new(mockMLService)
		svc.mlService = mlSvc

		t.Setenv("FEATURE_USE_GO_DISPATCH_LOGS", "true")
		config.ResetFeatureFlags()

		listID := "list-1"
		automation := &models.Automation{
			ID:           "auto-1",
			UserID:       "user-1",
			GroupIDs:     []string{"group@g.us"},
			MlLinkListID: &listID,
		}

		deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
			Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)

		items := []*models.MlLinkListItem{
			{ID: "item-1", OriginURL: "https://www.mercadolivre.com.br/product-1"},
		}
		deps.mlLinkListItemRepo.On("GetNextUnconsumed", mock.Anything, "list-1", 1).Return(items, nil)

		preview := &models.MLProductPreview{
			Title: "ML Product 1",
			Price: 149.90,
			Image: "https://img.ml/1.jpg",
		}
		mlSvc.On("GetProductPreview", mock.Anything, "https://www.mercadolivre.com.br/product-1", "user-1").Return(preview, nil)
		mlSvc.On("CreateAffiliateLink", mock.Anything, "https://www.mercadolivre.com.br/product-1", "user-1").
			Return(&models.MLAffiliateLink{AffiliateLink: "https://aff.ml/link1"}, nil)

		deps.dispatchLogRepo.On("Create", mock.Anything, mock.Anything).Return(&models.DispatchLog{}, nil).Maybe()
		deps.mlLinkListItemRepo.On("MarkAsConsumed", mock.Anything, []string{"item-1"}, "auto-1", mock.Anything).Return(nil)

		result, err := svc.processListMode(context.Background(), automation)
		require.NoError(t, err)
		assert.True(t, result.Success)
		assert.Equal(t, 1, result.MessagesSent)
		mlSvc.AssertCalled(t, "GetProductPreview", mock.Anything, "https://www.mercadolivre.com.br/product-1", "user-1")
		mlSvc.AssertCalled(t, "CreateAffiliateLink", mock.Anything, "https://www.mercadolivre.com.br/product-1", "user-1")
	})

	t.Run("ML preview error falls back to item metadata", func(t *testing.T) {
		// Not parallel: uses t.Setenv
		svc, deps := newInternalTestService(t)
		mlSvc := new(mockMLService)
		svc.mlService = mlSvc

		t.Setenv("FEATURE_USE_GO_DISPATCH_LOGS", "true")
		config.ResetFeatureFlags()

		listID := "list-1"
		title := "Fallback Product"
		price := 99.0
		imgURL := "https://fallback.img/1.jpg"
		automation := &models.Automation{
			ID:           "auto-1",
			UserID:       "user-1",
			GroupIDs:     []string{"group@g.us"},
			MlLinkListID: &listID,
		}

		deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
			Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)

		items := []*models.MlLinkListItem{
			{ID: "item-1", OriginURL: "https://www.mercadolivre.com.br/product-1", ProductTitle: &title, ProductPrice: &price, ProductImageURL: &imgURL},
		}
		deps.mlLinkListItemRepo.On("GetNextUnconsumed", mock.Anything, "list-1", 1).Return(items, nil)

		mlSvc.On("GetProductPreview", mock.Anything, mock.Anything, "user-1").Return(nil, errors.New("preview error"))
		mlSvc.On("CreateAffiliateLink", mock.Anything, mock.Anything, "user-1").Return(nil, errors.New("link error"))

		deps.dispatchLogRepo.On("Create", mock.Anything, mock.Anything).Return(&models.DispatchLog{}, nil).Maybe()
		deps.mlLinkListItemRepo.On("MarkAsConsumed", mock.Anything, []string{"item-1"}, "auto-1", mock.Anything).Return(nil)

		result, err := svc.processListMode(context.Background(), automation)
		require.NoError(t, err)
		assert.True(t, result.Success)
		assert.Equal(t, 1, result.MessagesSent)
	})

	t.Run("non-ML URL is skipped", func(t *testing.T) {
		// Not parallel: uses t.Setenv
		svc, deps := newInternalTestService(t)

		t.Setenv("FEATURE_USE_GO_DISPATCH_LOGS", "true")
		config.ResetFeatureFlags()

		listID := "list-1"
		automation := &models.Automation{
			ID:           "auto-1",
			UserID:       "user-1",
			GroupIDs:     []string{"group@g.us"},
			MlLinkListID: &listID,
		}

		deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
			Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)

		items := []*models.MlLinkListItem{
			{ID: "item-1", OriginURL: "https://www.shopee.com/product-1"}, // not ML URL
		}
		deps.mlLinkListItemRepo.On("GetNextUnconsumed", mock.Anything, "list-1", 1).Return(items, nil)
		deps.dispatchLogRepo.On("Create", mock.Anything, mock.Anything).Return(&models.DispatchLog{}, nil).Maybe()

		result, err := svc.processListMode(context.Background(), automation)
		require.NoError(t, err)
		assert.Equal(t, 0, result.MessagesSent)
	})

	t.Run("empty origin URL is skipped", func(t *testing.T) {
		// Not parallel: uses t.Setenv
		svc, deps := newInternalTestService(t)

		t.Setenv("FEATURE_USE_GO_DISPATCH_LOGS", "true")
		config.ResetFeatureFlags()

		listID := "list-1"
		automation := &models.Automation{
			ID:           "auto-1",
			UserID:       "user-1",
			GroupIDs:     []string{"group@g.us"},
			MlLinkListID: &listID,
		}

		deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
			Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)

		items := []*models.MlLinkListItem{
			{ID: "item-1", OriginURL: "   "}, // whitespace only
		}
		deps.mlLinkListItemRepo.On("GetNextUnconsumed", mock.Anything, "list-1", 1).Return(items, nil)

		result, err := svc.processListMode(context.Background(), automation)
		require.NoError(t, err)
		assert.Equal(t, 0, result.MessagesSent)
	})

	t.Run("includeImage false skips image", func(t *testing.T) {
		// Not parallel: uses t.Setenv
		svc, deps := newInternalTestService(t)

		t.Setenv("FEATURE_USE_GO_DISPATCH_LOGS", "true")
		config.ResetFeatureFlags()

		listID := "list-1"
		includeImage := false
		automation := &models.Automation{
			ID:               "auto-1",
			UserID:           "user-1",
			GroupIDs:         []string{"group@g.us"},
			MlLinkListID:    &listID,
			ListIncludeImage: &includeImage,
		}

		deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
			Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)

		items := []*models.MlLinkListItem{
			{ID: "item-1", OriginURL: "https://www.mercadolivre.com.br/product-1"},
		}
		deps.mlLinkListItemRepo.On("GetNextUnconsumed", mock.Anything, "list-1", 1).Return(items, nil)
		deps.dispatchLogRepo.On("Create", mock.Anything, mock.Anything).Return(&models.DispatchLog{}, nil).Maybe()
		deps.mlLinkListItemRepo.On("MarkAsConsumed", mock.Anything, []string{"item-1"}, "auto-1", mock.Anything).Return(nil)

		result, err := svc.processListMode(context.Background(), automation)
		require.NoError(t, err)
		assert.Equal(t, 1, result.MessagesSent)
	})

	t.Run("listItemsPerRun clamped to 20", func(t *testing.T) {
		// Not parallel: feature flag default may call dispatchLogRepo
		svc, deps := newInternalTestService(t)

		listID := "list-1"
		itemsPerRun := 50 // exceeds max → clamped to 20
		automation := &models.Automation{
			ID:              "auto-1",
			UserID:          "user-1",
			GroupIDs:        []string{"group@g.us"},
			MlLinkListID:   &listID,
			ListItemsPerRun: &itemsPerRun,
		}

		deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
			Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)
		deps.dispatchLogRepo.On("Create", mock.Anything, mock.Anything).Return(&models.DispatchLog{}, nil).Maybe()

		// Expect clamped value of 20
		deps.mlLinkListItemRepo.On("GetNextUnconsumed", mock.Anything, "list-1", 20).Return([]*models.MlLinkListItem{}, nil)

		result, err := svc.processListMode(context.Background(), automation)
		require.NoError(t, err)
		assert.Equal(t, 0, result.MessagesSent)
		deps.mlLinkListItemRepo.AssertCalled(t, "GetNextUnconsumed", mock.Anything, "list-1", 20)
	})

	t.Run("get instances error", func(t *testing.T) {
		t.Parallel()
		svc, deps := newInternalTestService(t)

		listID := "list-1"
		automation := &models.Automation{
			ID:           "auto-1",
			UserID:       "user-1",
			GroupIDs:     []string{"group@g.us"},
			MlLinkListID: &listID,
		}

		deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
			Return(nil, errors.New("db error"))

		_, err := svc.processListMode(context.Background(), automation)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get WhatsApp instances")
	})

	t.Run("no valid JID groups", func(t *testing.T) {
		// Not parallel: uses t.Setenv
		svc, deps := newInternalTestService(t)

		t.Setenv("FEATURE_USE_GO_DISPATCH_LOGS", "true")
		config.ResetFeatureFlags()

		listID := "list-1"
		automation := &models.Automation{
			ID:           "auto-1",
			UserID:       "user-1",
			GroupIDs:     []string{"not-a-jid"}, // no @g.us
			MlLinkListID: &listID,
		}

		deps.dispatchLogRepo.On("Create", mock.Anything, mock.Anything).Return(&models.DispatchLog{}, nil).Maybe()

		result, err := svc.processListMode(context.Background(), automation)
		require.NoError(t, err)
		assert.False(t, result.Success)
		assert.Contains(t, result.Message, "Nenhum grupo valido")
	})

	t.Run("unconsumed items error", func(t *testing.T) {
		t.Parallel()
		svc, deps := newInternalTestService(t)

		listID := "list-1"
		automation := &models.Automation{
			ID:           "auto-1",
			UserID:       "user-1",
			GroupIDs:     []string{"group@g.us"},
			MlLinkListID: &listID,
		}

		deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
			Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)
		deps.mlLinkListItemRepo.On("GetNextUnconsumed", mock.Anything, "list-1", 1).
			Return(nil, errors.New("db error"))

		_, err := svc.processListMode(context.Background(), automation)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get unconsumed items")
	})

	t.Run("with dispatch text template", func(t *testing.T) {
		// Not parallel: uses t.Setenv
		svc, deps := newInternalTestService(t)

		t.Setenv("FEATURE_USE_GO_DISPATCH_LOGS", "true")
		config.ResetFeatureFlags()

		listID := "list-1"
		dtID := "dt-1"
		automation := &models.Automation{
			ID:             "auto-1",
			UserID:         "user-1",
			GroupIDs:       []string{"group@g.us"},
			MlLinkListID:  &listID,
			DispatchTextID: &dtID,
		}

		deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
			Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)
		deps.settingsRepo.On("GetActiveDispatchTextByID", mock.Anything, "dt-1", "user-1").
			Return(&models.DispatchText{Body: "Custom ML: {{PRODUTO_NOME}}"}, nil)

		items := []*models.MlLinkListItem{
			{ID: "item-1", OriginURL: "https://www.mercadolivre.com.br/product-1"},
		}
		deps.mlLinkListItemRepo.On("GetNextUnconsumed", mock.Anything, "list-1", 1).Return(items, nil)
		deps.dispatchLogRepo.On("Create", mock.Anything, mock.Anything).Return(&models.DispatchLog{}, nil).Maybe()
		deps.mlLinkListItemRepo.On("MarkAsConsumed", mock.Anything, []string{"item-1"}, "auto-1", mock.Anything).Return(nil)

		result, err := svc.processListMode(context.Background(), automation)
		require.NoError(t, err)
		assert.Equal(t, 1, result.MessagesSent)
	})
}

// =============================================================================
// RunAutomation deferred next_run_at update
// =============================================================================

func TestRunAutomation_NextRunAtUpdate(t *testing.T) {
	// Not parallel: uses t.Setenv
	svc, deps := newInternalTestService(t)
	runRepo := new(mockInternalRunRepo)
	svc.automationRunRepo = runRepo

	t.Setenv("FEATURE_USE_GO_DISPATCH_LOGS", "true")
	config.ResetFeatureFlags()

	interval := 30
	searchType := "search"
	automation := &models.Automation{
		ID:              "auto-1",
		UserID:          "user-1",
		Name:            "Test Auto",
		Type:            &searchType,
		GroupIDs:        []string{"group@g.us"},
		IntervalMinutes: &interval,
	}

	lockUntil := time.Now().Add(5 * time.Minute)

	deps.automationRepo.On("GetAutomationByIDAndUser", mock.Anything, "auto-1", "user-1").Return(automation, nil)
	runRepo.On("GetLock", mock.Anything, "auto-1").Return(nil, nil)
	runRepo.On("AcquireLock", mock.Anything, "auto-1", mock.AnythingOfType("time.Duration")).Return(&lockUntil, nil)
	runRepo.On("ReleaseLock", mock.Anything, "auto-1").Return(nil)

	deps.historyRepo.On("Create", mock.Anything, "user-1", "automation_run", mock.Anything, mock.Anything).Return(nil, nil)

	deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
		Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)

	// UpdateNextRunAt is the key assertion - deferred call
	deps.automationRepo.On("UpdateNextRunAt", mock.Anything, "auto-1", mock.AnythingOfType("time.Time")).Return(nil)
	deps.automationRepo.On("UpdateLastRunAt", mock.Anything, "auto-1").Return(nil)

	result, err := svc.RunAutomation(context.Background(), "auto-1", "user-1")
	require.NoError(t, err)
	assert.NotNil(t, result)
	deps.automationRepo.AssertCalled(t, "UpdateNextRunAt", mock.Anything, "auto-1", mock.AnythingOfType("time.Time"))
}

// =============================================================================
// executeSearchAutomation: URL safety block, backpressure, enqueue paths
// =============================================================================

func TestExecuteSearchAutomation_URLSafetyBlock(t *testing.T) {
	// Not parallel: uses t.Setenv
	svc, deps := newInternalTestService(t)
	shopee := new(mockShopeeService)
	svc.shopeeService = shopee

	t.Setenv("FEATURE_USE_GO_DISPATCH_LOGS", "true")
	config.ResetFeatureFlags()

	logger := zaptest.NewLogger(t)

	// Create URL safety service that blocks "blocked.com"
	usSvc := urlsafety.NewService(urlsafety.Config{
		Mode:         "block",
		BlockedHosts: []string{"blocked.com"},
	}, nil, logger)
	svc.urlSafetyService = usSvc

	keyword := "test"
	automation := &models.Automation{
		ID:       "auto-1",
		UserID:   "user-1",
		GroupIDs: []string{"group@g.us"},
		Stores:   []string{"shopee"},
		Keyword:  &keyword,
	}

	deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
		Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)

	// Return offer with a blocked URL
	offers := []models.ShopeeOffer{
		{Name: "Product 1", ProductURL: "https://blocked.com/p1", Price: 99.90, OriginalPrice: 199.90, Discount: 50, AffiliateLink: "https://blocked.com/aff1"},
	}
	shopee.On("SearchOffers", mock.Anything, "user-1", mock.Anything).
		Return(&models.ShopeeSearchResult{Offers: offers}, nil)
	shopee.On("GenerateAffiliateLink", mock.Anything, mock.Anything, "user-1").
		Return(&models.ShopeeAffiliateLink{AffiliateLink: "https://blocked.com/aff1"}, nil)

	deps.dispatchLogRepo.On("GetRecentByCanonicalKey", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, nil)
	deps.dispatchLogRepo.On("Create", mock.Anything, mock.Anything).Return(&models.DispatchLog{}, nil).Maybe()

	result, err := svc.executeSearchAutomation(context.Background(), automation, "hist-1")
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, 0, result.MessagesSent) // blocked by URL safety
}

func TestExecuteSearchAutomation_BackpressureBlock(t *testing.T) {
	// Not parallel: uses t.Setenv
	svc, deps := newInternalTestService(t)
	shopee := new(mockShopeeService)
	svc.shopeeService = shopee

	t.Setenv("FEATURE_USE_GO_DISPATCH_LOGS", "true")
	config.ResetFeatureFlags()

	logger := zaptest.NewLogger(t)

	// Create backpressure with very low limit (needs real Redis to track counts)
	bpCache, _ := newTestCache(t)
	bp := bpLib.NewProducerBackpressure(bpLib.Config{
		Enabled:           true,
		UserMaxEnqueued:   1,
		GlobalMaxEnqueued: 0, // no global limit
		WindowMinutes:     10,
	}, bpCache, logger)
	svc.backpressure = bp

	// Pre-fill the user's count in Redis
	_ = bp.RecordEnqueue(context.Background(), "user-1")

	keyword := "test"
	automation := &models.Automation{
		ID:       "auto-1",
		UserID:   "user-1",
		GroupIDs: []string{"group@g.us"},
		Stores:   []string{"shopee"},
		Keyword:  &keyword,
	}

	deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
		Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)

	offers := []models.ShopeeOffer{
		{Name: "Product 1", ProductURL: "https://shopee.com/p1", Price: 99.90, OriginalPrice: 199.90, Discount: 50},
	}
	shopee.On("SearchOffers", mock.Anything, "user-1", mock.Anything).
		Return(&models.ShopeeSearchResult{Offers: offers}, nil)
	shopee.On("GenerateAffiliateLink", mock.Anything, mock.Anything, "user-1").
		Return(&models.ShopeeAffiliateLink{AffiliateLink: "https://s.shopee.com/aff1"}, nil)

	deps.dispatchLogRepo.On("GetRecentByCanonicalKey", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, nil)
	deps.dispatchLogRepo.On("Create", mock.Anything, mock.Anything).Return(&models.DispatchLog{}, nil).Maybe()

	result, err := svc.executeSearchAutomation(context.Background(), automation, "hist-1")
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, 0, result.MessagesSent) // blocked by backpressure
}

func TestExecuteSearchAutomation_WithQueueClient(t *testing.T) {
	// Not parallel: uses t.Setenv
	svc, deps := newInternalTestService(t)
	shopee := new(mockShopeeService)
	svc.shopeeService = shopee

	t.Setenv("FEATURE_USE_GO_DISPATCH_LOGS", "true")
	config.ResetFeatureFlags()

	logger := zaptest.NewLogger(t)

	// Create real queue client with miniredis
	mr := miniredis.RunT(t)
	qc := workers.NewQueueClient(mr.Addr(), logger)
	t.Cleanup(func() { qc.Close() })
	svc.queueClient = qc

	keyword := "test"
	automation := &models.Automation{
		ID:       "auto-1",
		UserID:   "user-1",
		GroupIDs: []string{"group@g.us"},
		Stores:   []string{"shopee"},
		Keyword:  &keyword,
	}

	deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
		Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)

	offers := []models.ShopeeOffer{
		{Name: "Product 1", ProductURL: "https://shopee.com/p1", Price: 99.90, OriginalPrice: 199.90, Discount: 50, Image: "https://img.com/1.jpg"},
	}
	shopee.On("SearchOffers", mock.Anything, "user-1", mock.Anything).
		Return(&models.ShopeeSearchResult{Offers: offers}, nil)
	shopee.On("GenerateAffiliateLink", mock.Anything, mock.Anything, "user-1").
		Return(&models.ShopeeAffiliateLink{AffiliateLink: "https://s.shopee.com/aff1"}, nil)

	deps.dispatchLogRepo.On("GetRecentByCanonicalKey", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, nil)
	deps.dispatchLogRepo.On("Create", mock.Anything, mock.Anything).Return(&models.DispatchLog{}, nil).Maybe()

	result, err := svc.executeSearchAutomation(context.Background(), automation, "hist-1")
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, 1, result.MessagesSent)
}

func TestExecuteSearchAutomation_WithBackpressureRecord(t *testing.T) {
	// Not parallel: uses t.Setenv
	svc, deps := newInternalTestService(t)
	shopee := new(mockShopeeService)
	svc.shopeeService = shopee

	t.Setenv("FEATURE_USE_GO_DISPATCH_LOGS", "true")
	config.ResetFeatureFlags()

	logger := zaptest.NewLogger(t)

	// Create backpressure with high limit (allows enqueue) to cover RecordEnqueue path
	bpCache, _ := newTestCache(t)
	bp := bpLib.NewProducerBackpressure(bpLib.Config{
		Enabled:           true,
		UserMaxEnqueued:   100,
		GlobalMaxEnqueued: 0,
		WindowMinutes:     10,
	}, bpCache, logger)
	svc.backpressure = bp

	keyword := "test"
	automation := &models.Automation{
		ID:       "auto-1",
		UserID:   "user-1",
		GroupIDs: []string{"group@g.us"},
		Stores:   []string{"shopee"},
		Keyword:  &keyword,
	}

	deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
		Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)

	offers := []models.ShopeeOffer{
		{Name: "Product 1", ProductURL: "https://shopee.com/p1", Price: 99.90, OriginalPrice: 199.90, Discount: 50},
	}
	shopee.On("SearchOffers", mock.Anything, "user-1", mock.Anything).
		Return(&models.ShopeeSearchResult{Offers: offers}, nil)
	shopee.On("GenerateAffiliateLink", mock.Anything, mock.Anything, "user-1").
		Return(&models.ShopeeAffiliateLink{AffiliateLink: "https://s.shopee.com/aff1"}, nil)

	deps.dispatchLogRepo.On("GetRecentByCanonicalKey", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, nil)
	deps.dispatchLogRepo.On("Create", mock.Anything, mock.Anything).Return(&models.DispatchLog{}, nil).Maybe()

	result, err := svc.executeSearchAutomation(context.Background(), automation, "hist-1")
	require.NoError(t, err)
	assert.Equal(t, 1, result.MessagesSent) // sent + RecordEnqueue called
}

// =============================================================================
// processListMode: URL safety block + backpressure + queue client
// =============================================================================

func TestProcessListMode_URLSafetyBlock(t *testing.T) {
	// Not parallel: uses t.Setenv
	svc, deps := newInternalTestService(t)

	t.Setenv("FEATURE_USE_GO_DISPATCH_LOGS", "true")
	config.ResetFeatureFlags()

	logger := zaptest.NewLogger(t)

	// Create URL safety that blocks mercadolivre.com.br
	usSvc := urlsafety.NewService(urlsafety.Config{
		Mode:             "block",
		AllowedHosts:     []string{},
		EnforceAllowlist: true, // blocks everything not in allowlist
	}, nil, logger)
	svc.urlSafetyService = usSvc

	listID := "list-1"
	automation := &models.Automation{
		ID:           "auto-1",
		UserID:       "user-1",
		GroupIDs:     []string{"group@g.us"},
		MlLinkListID: &listID,
	}

	deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
		Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)

	items := []*models.MlLinkListItem{
		{ID: "item-1", OriginURL: "https://www.mercadolivre.com.br/product-1"},
	}
	deps.mlLinkListItemRepo.On("GetNextUnconsumed", mock.Anything, "list-1", 1).Return(items, nil)
	deps.dispatchLogRepo.On("Create", mock.Anything, mock.Anything).Return(&models.DispatchLog{}, nil).Maybe()

	result, err := svc.processListMode(context.Background(), automation)
	require.NoError(t, err)
	assert.Equal(t, 0, result.MessagesSent) // blocked by URL safety
}

func TestProcessListMode_BackpressureBlock(t *testing.T) {
	// Not parallel: uses t.Setenv
	svc, deps := newInternalTestService(t)

	t.Setenv("FEATURE_USE_GO_DISPATCH_LOGS", "true")
	config.ResetFeatureFlags()

	logger := zaptest.NewLogger(t)

	// Create backpressure with very low limit (needs real Redis to track counts)
	bpCache, _ := newTestCache(t)
	bp := bpLib.NewProducerBackpressure(bpLib.Config{
		Enabled:           true,
		UserMaxEnqueued:   1,
		GlobalMaxEnqueued: 0,
		WindowMinutes:     10,
	}, bpCache, logger)
	svc.backpressure = bp

	// Pre-fill user count in Redis
	_ = bp.RecordEnqueue(context.Background(), "user-1")

	listID := "list-1"
	automation := &models.Automation{
		ID:           "auto-1",
		UserID:       "user-1",
		GroupIDs:     []string{"group@g.us"},
		MlLinkListID: &listID,
	}

	deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
		Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)

	items := []*models.MlLinkListItem{
		{ID: "item-1", OriginURL: "https://www.mercadolivre.com.br/product-1"},
	}
	deps.mlLinkListItemRepo.On("GetNextUnconsumed", mock.Anything, "list-1", 1).Return(items, nil)
	deps.dispatchLogRepo.On("Create", mock.Anything, mock.Anything).Return(&models.DispatchLog{}, nil).Maybe()

	result, err := svc.processListMode(context.Background(), automation)
	require.NoError(t, err)
	assert.Equal(t, 0, result.MessagesSent) // blocked by backpressure
}

func TestProcessListMode_WithQueueClient(t *testing.T) {
	// Not parallel: uses t.Setenv
	svc, deps := newInternalTestService(t)

	t.Setenv("FEATURE_USE_GO_DISPATCH_LOGS", "true")
	config.ResetFeatureFlags()

	logger := zaptest.NewLogger(t)

	// Create queue client with miniredis
	mr := miniredis.RunT(t)
	qc := workers.NewQueueClient(mr.Addr(), logger)
	t.Cleanup(func() { qc.Close() })
	svc.queueClient = qc

	listID := "list-1"
	automation := &models.Automation{
		ID:           "auto-1",
		UserID:       "user-1",
		GroupIDs:     []string{"group@g.us"},
		MlLinkListID: &listID,
	}

	deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
		Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)

	items := []*models.MlLinkListItem{
		{ID: "item-1", OriginURL: "https://www.mercadolivre.com.br/product-1"},
	}
	deps.mlLinkListItemRepo.On("GetNextUnconsumed", mock.Anything, "list-1", 1).Return(items, nil)
	deps.dispatchLogRepo.On("Create", mock.Anything, mock.Anything).Return(&models.DispatchLog{}, nil).Maybe()
	deps.mlLinkListItemRepo.On("MarkAsConsumed", mock.Anything, []string{"item-1"}, "auto-1", mock.Anything).Return(nil)

	result, err := svc.processListMode(context.Background(), automation)
	require.NoError(t, err)
	assert.Equal(t, 1, result.MessagesSent)
}

// TestProcessListMode_SendAsImageTrue verifies the LIST mode runs end-to-end
// when ListSendAsImage=true. With mlService/shopeeService nil in the fixture, the
// preview branch (now gated by !sendAsImage) is additionally skipped; the test's
// purpose is to ensure the new flag does not break the enqueue path.
func TestProcessListMode_SendAsImageTrue(t *testing.T) {
	svc, deps := newInternalTestService(t)

	t.Setenv("FEATURE_USE_GO_DISPATCH_LOGS", "true")
	config.ResetFeatureFlags()

	logger := zaptest.NewLogger(t)

	mr := miniredis.RunT(t)
	qc := workers.NewQueueClient(mr.Addr(), logger)
	t.Cleanup(func() { qc.Close() })
	svc.queueClient = qc

	listID := "list-1"
	sendAsImage := true
	automation := &models.Automation{
		ID:              "auto-1",
		UserID:          "user-1",
		GroupIDs:        []string{"group@g.us"},
		MlLinkListID:    &listID,
		ListSendAsImage: &sendAsImage,
	}

	deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
		Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)

	items := []*models.MlLinkListItem{
		{ID: "item-1", OriginURL: "https://www.mercadolivre.com.br/MLB-9999"},
	}
	deps.mlLinkListItemRepo.On("GetNextUnconsumed", mock.Anything, "list-1", 1).Return(items, nil)
	deps.dispatchLogRepo.On("Create", mock.Anything, mock.Anything).Return(&models.DispatchLog{}, nil).Maybe()
	deps.mlLinkListItemRepo.On("MarkAsConsumed", mock.Anything, []string{"item-1"}, "auto-1", mock.Anything).Return(nil)

	result, err := svc.processListMode(context.Background(), automation)
	require.NoError(t, err)
	assert.Equal(t, 1, result.MessagesSent)
}

// TestProcessListMode_SendAsImageFalse ensures the default/explicit-false path
// still runs end-to-end (no regression in the preview branch).
func TestProcessListMode_SendAsImageFalse(t *testing.T) {
	svc, deps := newInternalTestService(t)

	t.Setenv("FEATURE_USE_GO_DISPATCH_LOGS", "true")
	config.ResetFeatureFlags()

	logger := zaptest.NewLogger(t)

	mr := miniredis.RunT(t)
	qc := workers.NewQueueClient(mr.Addr(), logger)
	t.Cleanup(func() { qc.Close() })
	svc.queueClient = qc

	listID := "list-1"
	sendAsImage := false
	automation := &models.Automation{
		ID:              "auto-1",
		UserID:          "user-1",
		GroupIDs:        []string{"group@g.us"},
		MlLinkListID:    &listID,
		ListSendAsImage: &sendAsImage,
	}

	deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
		Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)

	items := []*models.MlLinkListItem{
		{ID: "item-1", OriginURL: "https://www.mercadolivre.com.br/MLB-8888"},
	}
	deps.mlLinkListItemRepo.On("GetNextUnconsumed", mock.Anything, "list-1", 1).Return(items, nil)
	deps.dispatchLogRepo.On("Create", mock.Anything, mock.Anything).Return(&models.DispatchLog{}, nil).Maybe()
	deps.mlLinkListItemRepo.On("MarkAsConsumed", mock.Anything, []string{"item-1"}, "auto-1", mock.Anything).Return(nil)

	result, err := svc.processListMode(context.Background(), automation)
	require.NoError(t, err)
	assert.Equal(t, 1, result.MessagesSent)
}

func TestProcessListMode_WithBackpressureRecord(t *testing.T) {
	// Not parallel: uses t.Setenv
	svc, deps := newInternalTestService(t)

	t.Setenv("FEATURE_USE_GO_DISPATCH_LOGS", "true")
	config.ResetFeatureFlags()

	logger := zaptest.NewLogger(t)

	bpCache, _ := newTestCache(t)
	bp := bpLib.NewProducerBackpressure(bpLib.Config{
		Enabled:           true,
		UserMaxEnqueued:   100,
		GlobalMaxEnqueued: 0,
		WindowMinutes:     10,
	}, bpCache, logger)
	svc.backpressure = bp

	listID := "list-1"
	automation := &models.Automation{
		ID:           "auto-1",
		UserID:       "user-1",
		GroupIDs:     []string{"group@g.us"},
		MlLinkListID: &listID,
	}

	deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
		Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)

	items := []*models.MlLinkListItem{
		{ID: "item-1", OriginURL: "https://www.mercadolivre.com.br/product-1"},
	}
	deps.mlLinkListItemRepo.On("GetNextUnconsumed", mock.Anything, "list-1", 1).Return(items, nil)
	deps.dispatchLogRepo.On("Create", mock.Anything, mock.Anything).Return(&models.DispatchLog{}, nil).Maybe()
	deps.mlLinkListItemRepo.On("MarkAsConsumed", mock.Anything, []string{"item-1"}, "auto-1", mock.Anything).Return(nil)

	result, err := svc.processListMode(context.Background(), automation)
	require.NoError(t, err)
	assert.Equal(t, 1, result.MessagesSent) // sent + RecordEnqueue called
}

// =============================================================================
// RunAutomation: error branch with history update to "failed"
// =============================================================================

func TestRunAutomation_SearchError_UpdatesHistoryToFailed(t *testing.T) {
	// Not parallel: uses t.Setenv
	svc, deps := newInternalTestService(t)
	runRepo := new(mockInternalRunRepo)
	svc.automationRunRepo = runRepo

	t.Setenv("FEATURE_USE_GO_DISPATCH_LOGS", "true")
	config.ResetFeatureFlags()

	searchType := "search"
	automation := &models.Automation{
		ID:       "auto-1",
		UserID:   "user-1",
		Name:     "Test Auto",
		Type:     &searchType,
		GroupIDs: []string{"group@g.us"},
	}

	lockUntil := time.Now().Add(5 * time.Minute)

	deps.automationRepo.On("GetAutomationByIDAndUser", mock.Anything, "auto-1", "user-1").Return(automation, nil)
	runRepo.On("GetLock", mock.Anything, "auto-1").Return(nil, nil)
	runRepo.On("AcquireLock", mock.Anything, "auto-1", mock.AnythingOfType("time.Duration")).Return(&lockUntil, nil)
	runRepo.On("ReleaseLock", mock.Anything, "auto-1").Return(nil)

	historyEntry := &models.ActionHistory{ID: "hist-1"}
	deps.historyRepo.On("Create", mock.Anything, "user-1", "automation_run", mock.Anything, mock.Anything).Return(historyEntry, nil)
	deps.historyRepo.On("Update", mock.Anything, "hist-1", mock.Anything).Return(nil)
	deps.historyRepo.On("MarkRunning", mock.Anything, "hist-1").Return(nil)
	deps.historyRepo.On("UpdateCounts", mock.Anything, "hist-1", mock.Anything, mock.Anything).Return(nil).Maybe()

	// No instances → executeSearchAutomation returns error
	deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
		Return([]models.Instance{}, nil)

	deps.automationRepo.On("UpdateLastRunAt", mock.Anything, "auto-1").Return(nil).Maybe()

	result, err := svc.RunAutomation(context.Background(), "auto-1", "user-1")
	require.NoError(t, err) // RunAutomation catches errors and returns result
	assert.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Contains(t, result.Message, "no WhatsApp instance")

	// Verify history was updated with "failed" status
	deps.historyRepo.AssertCalled(t, "Update", mock.Anything, "hist-1", mock.MatchedBy(func(data map[string]interface{}) bool {
		status, ok := data["status"].(string)
		return ok && status == "failed"
	}))
}

// =============================================================================
// RunAutomation: default type dispatches as search
// =============================================================================

func TestRunAutomation_DefaultType(t *testing.T) {
	// Not parallel: uses t.Setenv
	svc, deps := newInternalTestService(t)
	runRepo := new(mockInternalRunRepo)
	svc.automationRunRepo = runRepo

	t.Setenv("FEATURE_USE_GO_DISPATCH_LOGS", "true")
	config.ResetFeatureFlags()

	unknownType := "unknown"
	automation := &models.Automation{
		ID:       "auto-1",
		UserID:   "user-1",
		Name:     "Test Auto",
		Type:     &unknownType,
		GroupIDs: []string{"group@g.us"},
	}

	lockUntil := time.Now().Add(5 * time.Minute)

	deps.automationRepo.On("GetAutomationByIDAndUser", mock.Anything, "auto-1", "user-1").Return(automation, nil)
	runRepo.On("GetLock", mock.Anything, "auto-1").Return(nil, nil)
	runRepo.On("AcquireLock", mock.Anything, "auto-1", mock.AnythingOfType("time.Duration")).Return(&lockUntil, nil)
	runRepo.On("ReleaseLock", mock.Anything, "auto-1").Return(nil)

	deps.historyRepo.On("Create", mock.Anything, "user-1", "automation_run", mock.Anything, mock.Anything).Return(nil, nil)
	deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
		Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)
	deps.automationRepo.On("UpdateLastRunAt", mock.Anything, "auto-1").Return(nil)

	result, err := svc.RunAutomation(context.Background(), "auto-1", "user-1")
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Success)
	assert.Equal(t, 0, result.OffersFound) // no shopee service → no offers
}

// =============================================================================
// DiagnoseAutomation: nil service branches + store errors
// =============================================================================

func TestDiagnoseAutomation_NilServicesStoreErrors(t *testing.T) {
	t.Parallel()

	t.Run("nil shopee and ml services produce store errors", func(t *testing.T) {
		t.Parallel()
		svc, deps := newInternalTestService(t)
		// shopeeService and mlService are nil by default

		searchType := "search"
		automation := &models.Automation{
			ID:       "auto-1",
			UserID:   "user-1",
			Name:     "Test Auto",
			Type:     &searchType,
			GroupIDs: []string{"group@g.us"},
			Stores:   []string{"shopee", "mercadolivre"},
		}

		deps.automationRepo.On("GetAutomationByIDAndUser", mock.Anything, "auto-1", "user-1").Return(automation, nil)
		deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
			Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)

		result, err := svc.DiagnoseAutomation(context.Background(), "auto-1", "user-1")
		require.NoError(t, err)
		assert.NotNil(t, result)

		// Should have warnings about nil services
		found := false
		for _, w := range result.Warnings {
			if fmt.Sprintf("%v", w) == "Shopee: servico nao configurado" || fmt.Sprintf("%v", w) == "Mercado Livre: servico nao configurado" {
				found = true
			}
		}
		assert.True(t, found, "expected store config warnings")
	})

	t.Run("shopee service configured, API accessible", func(t *testing.T) {
		t.Parallel()
		svc, deps := newInternalTestService(t)
		shopee := new(mockShopeeService)
		svc.shopeeService = shopee

		searchType := "search"
		automation := &models.Automation{
			ID:       "auto-1",
			UserID:   "user-1",
			Name:     "Test Auto",
			Type:     &searchType,
			GroupIDs: []string{"group@g.us"},
			Stores:   []string{"shopee"},
		}

		deps.automationRepo.On("GetAutomationByIDAndUser", mock.Anything, "auto-1", "user-1").Return(automation, nil)
		deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
			Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)
		shopee.On("GetCategories", mock.Anything, (*int64)(nil)).Return([]models.ShopeeCategory{}, nil)

		result, err := svc.DiagnoseAutomation(context.Background(), "auto-1", "user-1")
		require.NoError(t, err)

		// Should have BUSCA_STORE check with "APIs acessiveis"
		foundAPICheck := false
		for _, check := range result.Checks {
			if check.Name == "BUSCA_STORE" && check.Status == "ok" {
				foundAPICheck = true
			}
		}
		assert.True(t, foundAPICheck, "expected BUSCA_STORE ok check")
	})

	t.Run("shopee service API error", func(t *testing.T) {
		t.Parallel()
		svc, deps := newInternalTestService(t)
		shopee := new(mockShopeeService)
		svc.shopeeService = shopee

		searchType := "search"
		automation := &models.Automation{
			ID:       "auto-1",
			UserID:   "user-1",
			Name:     "Test Auto",
			Type:     &searchType,
			GroupIDs: []string{"group@g.us"},
			Stores:   []string{"shopee"},
		}

		deps.automationRepo.On("GetAutomationByIDAndUser", mock.Anything, "auto-1", "user-1").Return(automation, nil)
		deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
			Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)
		shopee.On("GetCategories", mock.Anything, (*int64)(nil)).Return(nil, errors.New("shopee API down"))

		result, err := svc.DiagnoseAutomation(context.Background(), "auto-1", "user-1")
		require.NoError(t, err)

		// Should have BUSCA_STORE error
		foundError := false
		for _, check := range result.Checks {
			if check.Name == "BUSCA_STORE" && check.Status == "error" {
				foundError = true
			}
		}
		assert.True(t, foundError, "expected BUSCA_STORE error check")
	})

}

// =============================================================================
// RunAutomation: history error paths + UpdateNextRunAt error
// =============================================================================

func TestRunAutomation_HistoryUpdateErrors(t *testing.T) {
	// Not parallel: may use feature flags indirectly
	svc, deps := newInternalTestService(t)
	runRepo := new(mockInternalRunRepo)
	svc.automationRunRepo = runRepo

	searchType := "search"
	interval := 30
	automation := &models.Automation{
		ID:              "auto-1",
		UserID:          "user-1",
		Name:            "Test Auto",
		Type:            &searchType,
		GroupIDs:        []string{"group@g.us"},
		IntervalMinutes: &interval,
	}

	lockUntil := time.Now().Add(5 * time.Minute)
	deps.automationRepo.On("GetAutomationByIDAndUser", mock.Anything, "auto-1", "user-1").Return(automation, nil)
	runRepo.On("GetLock", mock.Anything, "auto-1").Return(nil, nil)
	runRepo.On("AcquireLock", mock.Anything, "auto-1", mock.AnythingOfType("time.Duration")).Return(&lockUntil, nil)
	runRepo.On("ReleaseLock", mock.Anything, "auto-1").Return(nil)

	historyEntry := &models.ActionHistory{ID: "hist-1"}
	deps.historyRepo.On("Create", mock.Anything, "user-1", "automation_run", mock.Anything, mock.Anything).Return(historyEntry, nil)

	// History Update for target_count FAILS
	deps.historyRepo.On("Update", mock.Anything, "hist-1", mock.Anything).Return(errors.New("update failed"))
	// History MarkRunning FAILS
	deps.historyRepo.On("MarkRunning", mock.Anything, "hist-1").Return(errors.New("mark running failed"))
	// History UpdateCounts FAILS
	deps.historyRepo.On("UpdateCounts", mock.Anything, "hist-1", mock.Anything, mock.Anything).Return(errors.New("counts failed"))

	deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
		Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)

	// UpdateNextRunAt FAILS
	deps.automationRepo.On("UpdateNextRunAt", mock.Anything, "auto-1", mock.AnythingOfType("time.Time")).Return(errors.New("update next run failed"))
	deps.automationRepo.On("UpdateLastRunAt", mock.Anything, "auto-1").Return(nil)

	result, err := svc.RunAutomation(context.Background(), "auto-1", "user-1")
	require.NoError(t, err)
	assert.NotNil(t, result)
	// Despite all errors, the run should still succeed (fire-and-forget)
	assert.True(t, result.Success)
}

// =============================================================================
// executeSearchAutomation: template with no variables + enqueue error
// =============================================================================

func TestExecuteSearchAutomation_TemplateNoVariables(t *testing.T) {
	// Not parallel: uses t.Setenv
	svc, deps := newInternalTestService(t)
	shopee := new(mockShopeeService)
	svc.shopeeService = shopee

	t.Setenv("FEATURE_USE_GO_DISPATCH_LOGS", "true")
	config.ResetFeatureFlags()

	keyword := "test"
	dtID := "dt-1"
	automation := &models.Automation{
		ID:             "auto-1",
		UserID:         "user-1",
		GroupIDs:       []string{"group@g.us"},
		Stores:         []string{"shopee"},
		Keyword:        &keyword,
		DispatchTextID: &dtID,
	}

	deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
		Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)

	offers := []models.ShopeeOffer{
		{Name: "Product 1", ProductURL: "https://shopee.com/p1", Price: 99.90, OriginalPrice: 199.90, Discount: 50},
	}
	shopee.On("SearchOffers", mock.Anything, "user-1", mock.Anything).
		Return(&models.ShopeeSearchResult{Offers: offers}, nil)
	shopee.On("GenerateAffiliateLink", mock.Anything, mock.Anything, "user-1").
		Return(&models.ShopeeAffiliateLink{AffiliateLink: "https://s.shopee.com/aff1"}, nil)

	// Return a dispatch text with NO template variables
	deps.settingsRepo.On("GetActiveDispatchTextByID", mock.Anything, "dt-1", "user-1").
		Return(&models.DispatchText{Body: "This is a static message with no variables"}, nil)

	deps.dispatchLogRepo.On("GetRecentByCanonicalKey", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, nil)
	deps.dispatchLogRepo.On("Create", mock.Anything, mock.Anything).Return(&models.DispatchLog{}, nil).Maybe()

	result, err := svc.executeSearchAutomation(context.Background(), automation, "hist-1")
	require.NoError(t, err)
	// Still sends the message even with no variables (just a warning logged)
	assert.Equal(t, 1, result.MessagesSent)
}

func TestExecuteSearchAutomation_EnqueueError(t *testing.T) {
	// Not parallel: uses t.Setenv
	svc, deps := newInternalTestService(t)
	shopee := new(mockShopeeService)
	svc.shopeeService = shopee

	t.Setenv("FEATURE_USE_GO_DISPATCH_LOGS", "true")
	config.ResetFeatureFlags()

	logger := zaptest.NewLogger(t)

	// Create queue client pointed at closed redis → enqueue will fail
	mr := miniredis.RunT(t)
	qc := workers.NewQueueClient(mr.Addr(), logger)
	t.Cleanup(func() { qc.Close() })
	svc.queueClient = qc
	mr.Close() // close redis → enqueue will fail

	keyword := "test"
	automation := &models.Automation{
		ID:       "auto-1",
		UserID:   "user-1",
		GroupIDs: []string{"group@g.us"},
		Stores:   []string{"shopee"},
		Keyword:  &keyword,
	}

	deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
		Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)

	offers := []models.ShopeeOffer{
		{Name: "Product 1", ProductURL: "https://shopee.com/p1", Price: 99.90, OriginalPrice: 199.90, Discount: 50},
	}
	shopee.On("SearchOffers", mock.Anything, "user-1", mock.Anything).
		Return(&models.ShopeeSearchResult{Offers: offers}, nil)
	shopee.On("GenerateAffiliateLink", mock.Anything, mock.Anything, "user-1").
		Return(&models.ShopeeAffiliateLink{AffiliateLink: "https://s.shopee.com/aff1"}, nil)

	deps.dispatchLogRepo.On("GetRecentByCanonicalKey", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, nil)
	deps.dispatchLogRepo.On("Create", mock.Anything, mock.Anything).Return(&models.DispatchLog{}, nil).Maybe()

	result, err := svc.executeSearchAutomation(context.Background(), automation, "hist-1")
	require.NoError(t, err)
	assert.Equal(t, 0, result.MessagesSent) // enqueue failed
}

// =============================================================================
// processListMode: enqueue error
// =============================================================================

func TestProcessListMode_EnqueueError(t *testing.T) {
	// Not parallel: uses t.Setenv
	svc, deps := newInternalTestService(t)

	t.Setenv("FEATURE_USE_GO_DISPATCH_LOGS", "true")
	config.ResetFeatureFlags()

	logger := zaptest.NewLogger(t)

	// Create queue client with closed miniredis
	mr := miniredis.RunT(t)
	qc := workers.NewQueueClient(mr.Addr(), logger)
	t.Cleanup(func() { qc.Close() })
	svc.queueClient = qc
	mr.Close() // force enqueue failure

	listID := "list-1"
	automation := &models.Automation{
		ID:           "auto-1",
		UserID:       "user-1",
		GroupIDs:     []string{"group@g.us"},
		MlLinkListID: &listID,
	}

	deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
		Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)

	items := []*models.MlLinkListItem{
		{ID: "item-1", OriginURL: "https://www.mercadolivre.com.br/product-1"},
	}
	deps.mlLinkListItemRepo.On("GetNextUnconsumed", mock.Anything, "list-1", 1).Return(items, nil)
	deps.dispatchLogRepo.On("Create", mock.Anything, mock.Anything).Return(&models.DispatchLog{}, nil).Maybe()

	result, err := svc.processListMode(context.Background(), automation)
	require.NoError(t, err)
	assert.Equal(t, 0, result.MessagesSent) // enqueue failed
}

// =============================================================================
// validateMonitorAutomation: time window in/out branches
// =============================================================================

func TestProcessMonitorMode_TimeWindowOutside(t *testing.T) {
	t.Parallel()
	svc, deps := newInternalTestService(t)

	// Set a very narrow time window guaranteed to be outside (02:00-02:01)
	startTime := "02:00"
	endTime := "02:01"
	automation := &models.Automation{
		ID:              "auto-1",
		UserID:          "user-1",
		Name:            "Monitor Test",
		MonitorGroupIDs: []string{"source@g.us"},
		GroupIDs:        []string{"target@g.us"},
		Stores:          []string{"shopee"},
		StartTime:       &startTime,
		EndTime:         &endTime,
	}

	deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
		Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)
	deps.dispatchLogRepo.On("Create", mock.Anything, mock.Anything).Return(&models.DispatchLog{}, nil).Maybe()

	result, err := svc.processMonitorMode(context.Background(), automation)
	require.NoError(t, err)
	assert.True(t, result.Success)
	// The message should contain time window info
	assert.Contains(t, result.Message, "Janela de horario")
}

func TestProcessMonitorMode_TimeWindowInside(t *testing.T) {
	t.Parallel()
	svc, deps := newInternalTestService(t)

	// Set a very wide time window (00:00-23:59) to guarantee we're inside
	startTime := "00:00"
	endTime := "23:59"
	automation := &models.Automation{
		ID:              "auto-1",
		UserID:          "user-1",
		Name:            "Monitor Test",
		MonitorGroupIDs: []string{"source@g.us"},
		GroupIDs:        []string{"target@g.us"},
		Stores:          []string{"shopee"},
		StartTime:       &startTime,
		EndTime:         &endTime,
	}

	deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
		Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)
	deps.dispatchLogRepo.On("Create", mock.Anything, mock.Anything).Return(&models.DispatchLog{}, nil).Maybe()

	result, err := svc.processMonitorMode(context.Background(), automation)
	require.NoError(t, err)
	assert.True(t, result.Success)
	// Should contain "DENTRO do horario"
	assert.Contains(t, result.Message, "DENTRO do horario")
}

// =============================================================================
// isDuplicate: cache backfill error path
// =============================================================================

func TestIsDuplicate_CacheBackfillError(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	c, mr := newTestCache(t)
	dlRepo := new(mockDispatchLogRepo)

	// DB returns a duplicate
	dlRepo.On("GetRecentByCanonicalKey", mock.Anything, "user-1", "group-1", "key-1", 24).
		Return(&models.DispatchLog{Status: "queued"}, nil)

	// Close miniredis to force cache backfill error
	mr.Close()

	svc := &automationService{
		cache:           c,
		dispatchLogRepo: dlRepo,
		logger:          logger,
	}

	// Should still return true (duplicate found in DB) even though cache backfill fails
	result := svc.isDuplicate(context.Background(), "user-1", "group-1", "key-1", 24)
	assert.True(t, result)
}

// =============================================================================
// DiagnoseAutomation: ANTIDUP cache Ping success and error paths (lines 239-245)
// =============================================================================

func TestDiagnoseAutomation_CachePingSuccess(t *testing.T) {
	t.Parallel()
	svc, deps := newInternalTestService(t)
	c, _ := newTestCache(t) // miniredis running → Ping will succeed
	svc.cache = c

	automation := &models.Automation{
		ID:       "auto-1",
		UserID:   "user-1",
		Name:     "Test",
		GroupIDs: []string{"group@g.us"},
		Stores:   []string{"shopee"},
	}

	deps.automationRepo.On("GetAutomationByIDAndUser", mock.Anything, "auto-1", "user-1").Return(automation, nil)
	deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
		Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)

	shopee := new(mockShopeeService)
	svc.shopeeService = shopee
	shopee.On("GetCategories", mock.Anything, mock.Anything).Return([]models.ShopeeCategory{}, nil)

	result, err := svc.DiagnoseAutomation(context.Background(), "auto-1", "user-1")
	require.NoError(t, err)

	var antidupCheck *models.DiagnoseCheck
	for _, c := range result.Checks {
		if c.Name == "ANTIDUP" {
			antidupCheck = &c
			break
		}
	}
	require.NotNil(t, antidupCheck)
	assert.Equal(t, "ok", antidupCheck.Status)
	assert.Contains(t, antidupCheck.Message, "operational")
}

func TestDiagnoseAutomation_CachePingError(t *testing.T) {
	t.Parallel()
	svc, deps := newInternalTestService(t)
	c, mr := newTestCache(t)
	svc.cache = c
	mr.Close() // close redis → Ping will fail

	automation := &models.Automation{
		ID:       "auto-1",
		UserID:   "user-1",
		Name:     "Test",
		GroupIDs: []string{"group@g.us"},
		Stores:   []string{"shopee"},
	}

	deps.automationRepo.On("GetAutomationByIDAndUser", mock.Anything, "auto-1", "user-1").Return(automation, nil)
	deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
		Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)

	shopee := new(mockShopeeService)
	svc.shopeeService = shopee
	shopee.On("GetCategories", mock.Anything, mock.Anything).Return([]models.ShopeeCategory{}, nil)

	result, err := svc.DiagnoseAutomation(context.Background(), "auto-1", "user-1")
	require.NoError(t, err)

	var antidupCheck *models.DiagnoseCheck
	for _, c := range result.Checks {
		if c.Name == "ANTIDUP" {
			antidupCheck = &c
			break
		}
	}
	require.NotNil(t, antidupCheck)
	assert.Equal(t, "warning", antidupCheck.Status)
	assert.Contains(t, antidupCheck.Message, "Cache indisponivel")
}

// =============================================================================
// RunAutomation: successful search with UpdateCounts + UpdateLastRunAt + GlobalMetrics (lines 418-446)
// =============================================================================

func TestRunAutomation_SuccessfulSearch_UpdateCountsAndMetrics(t *testing.T) {
	// Not parallel: uses t.Setenv
	svc, deps := newInternalTestService(t)
	runRepo := new(mockInternalRunRepo)
	svc.automationRunRepo = runRepo

	shopee := new(mockShopeeService)
	svc.shopeeService = shopee

	t.Setenv("FEATURE_USE_GO_DISPATCH_LOGS", "true")
	config.ResetFeatureFlags()

	searchType := "search"
	interval := 30
	mode := "search"
	automation := &models.Automation{
		ID:              "auto-1",
		UserID:          "user-1",
		Name:            "Test Auto",
		Type:            &searchType,
		Mode:            &mode,
		GroupIDs:        []string{"group@g.us"},
		IntervalMinutes: &interval,
		Stores:          []string{"shopee"},
	}

	lockUntil := time.Now().Add(5 * time.Minute)
	deps.automationRepo.On("GetAutomationByIDAndUser", mock.Anything, "auto-1", "user-1").Return(automation, nil)
	runRepo.On("GetLock", mock.Anything, "auto-1").Return(nil, nil)
	runRepo.On("AcquireLock", mock.Anything, "auto-1", mock.AnythingOfType("time.Duration")).Return(&lockUntil, nil)
	runRepo.On("ReleaseLock", mock.Anything, "auto-1").Return(nil)

	historyEntry := &models.ActionHistory{ID: "hist-1"}
	deps.historyRepo.On("Create", mock.Anything, "user-1", "automation_run", mock.Anything, mock.Anything).Return(historyEntry, nil)
	deps.historyRepo.On("Update", mock.Anything, "hist-1", mock.Anything).Return(nil)
	deps.historyRepo.On("MarkRunning", mock.Anything, "hist-1").Return(nil)
	// UpdateCounts will be called with MessagesSent > 0
	deps.historyRepo.On("UpdateCounts", mock.Anything, "hist-1", int32(1), int32(0)).Return(nil)

	deps.automationRepo.On("UpdateNextRunAt", mock.Anything, "auto-1", mock.AnythingOfType("time.Time")).Return(nil)
	deps.automationRepo.On("UpdateLastRunAt", mock.Anything, "auto-1").Return(nil)

	deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
		Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)

	offers := []models.ShopeeOffer{
		{Name: "Product 1", ProductURL: "https://shopee.com/p1", Price: 99.90, OriginalPrice: 199.90, Discount: 50},
	}
	shopee.On("SearchOffers", mock.Anything, "user-1", mock.Anything).
		Return(&models.ShopeeSearchResult{Offers: offers}, nil)
	shopee.On("GenerateAffiliateLink", mock.Anything, mock.Anything, "user-1").
		Return(&models.ShopeeAffiliateLink{AffiliateLink: "https://s.shopee.com/aff1"}, nil)

	deps.dispatchLogRepo.On("GetRecentByCanonicalKey", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, nil)
	deps.dispatchLogRepo.On("Create", mock.Anything, mock.Anything).Return(&models.DispatchLog{}, nil).Maybe()

	result, err := svc.RunAutomation(context.Background(), "auto-1", "user-1")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success)
	assert.Equal(t, 1, result.MessagesSent)

	// Verify UpdateCounts was called
	deps.historyRepo.AssertCalled(t, "UpdateCounts", mock.Anything, "hist-1", int32(1), int32(0))
	// Verify UpdateLastRunAt was called
	deps.automationRepo.AssertCalled(t, "UpdateLastRunAt", mock.Anything, "auto-1")
}

func TestRunAutomation_SuccessfulSearch_UpdateCountsError(t *testing.T) {
	// Not parallel: uses t.Setenv
	svc, deps := newInternalTestService(t)
	runRepo := new(mockInternalRunRepo)
	svc.automationRunRepo = runRepo

	shopee := new(mockShopeeService)
	svc.shopeeService = shopee

	t.Setenv("FEATURE_USE_GO_DISPATCH_LOGS", "true")
	config.ResetFeatureFlags()

	searchType := "search"
	interval := 30
	automation := &models.Automation{
		ID:              "auto-1",
		UserID:          "user-1",
		Name:            "Test Auto",
		Type:            &searchType,
		GroupIDs:        []string{"group@g.us"},
		IntervalMinutes: &interval,
		Stores:          []string{"shopee"},
	}

	lockUntil := time.Now().Add(5 * time.Minute)
	deps.automationRepo.On("GetAutomationByIDAndUser", mock.Anything, "auto-1", "user-1").Return(automation, nil)
	runRepo.On("GetLock", mock.Anything, "auto-1").Return(nil, nil)
	runRepo.On("AcquireLock", mock.Anything, "auto-1", mock.AnythingOfType("time.Duration")).Return(&lockUntil, nil)
	runRepo.On("ReleaseLock", mock.Anything, "auto-1").Return(nil)

	historyEntry := &models.ActionHistory{ID: "hist-1"}
	deps.historyRepo.On("Create", mock.Anything, "user-1", "automation_run", mock.Anything, mock.Anything).Return(historyEntry, nil)
	deps.historyRepo.On("Update", mock.Anything, "hist-1", mock.Anything).Return(nil)
	deps.historyRepo.On("MarkRunning", mock.Anything, "hist-1").Return(nil)
	// UpdateCounts returns error (fire-and-forget)
	deps.historyRepo.On("UpdateCounts", mock.Anything, "hist-1", int32(1), int32(0)).Return(errors.New("counts update failed"))

	deps.automationRepo.On("UpdateNextRunAt", mock.Anything, "auto-1", mock.AnythingOfType("time.Time")).Return(nil)
	deps.automationRepo.On("UpdateLastRunAt", mock.Anything, "auto-1").Return(nil)

	deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
		Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)

	offers := []models.ShopeeOffer{
		{Name: "Product 1", ProductURL: "https://shopee.com/p1", Price: 99.90, OriginalPrice: 199.90, Discount: 50},
	}
	shopee.On("SearchOffers", mock.Anything, "user-1", mock.Anything).
		Return(&models.ShopeeSearchResult{Offers: offers}, nil)
	shopee.On("GenerateAffiliateLink", mock.Anything, mock.Anything, "user-1").
		Return(&models.ShopeeAffiliateLink{AffiliateLink: "https://s.shopee.com/aff1"}, nil)

	deps.dispatchLogRepo.On("GetRecentByCanonicalKey", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, nil)
	deps.dispatchLogRepo.On("Create", mock.Anything, mock.Anything).Return(&models.DispatchLog{}, nil).Maybe()

	result, err := svc.RunAutomation(context.Background(), "auto-1", "user-1")
	require.NoError(t, err)
	require.NotNil(t, result)
	// Still succeeds despite UpdateCounts error
	assert.True(t, result.Success)
	assert.Equal(t, 1, result.MessagesSent)
}

func TestRunAutomation_UpdateLastRunAtError(t *testing.T) {
	// Not parallel: uses t.Setenv
	svc, deps := newInternalTestService(t)
	runRepo := new(mockInternalRunRepo)
	svc.automationRunRepo = runRepo

	shopee := new(mockShopeeService)
	svc.shopeeService = shopee

	t.Setenv("FEATURE_USE_GO_DISPATCH_LOGS", "true")
	config.ResetFeatureFlags()

	searchType := "search"
	automation := &models.Automation{
		ID:       "auto-1",
		UserID:   "user-1",
		Name:     "Test Auto",
		Type:     &searchType,
		GroupIDs: []string{"group@g.us"},
		Stores:   []string{"shopee"},
	}

	lockUntil := time.Now().Add(5 * time.Minute)
	deps.automationRepo.On("GetAutomationByIDAndUser", mock.Anything, "auto-1", "user-1").Return(automation, nil)
	runRepo.On("GetLock", mock.Anything, "auto-1").Return(nil, nil)
	runRepo.On("AcquireLock", mock.Anything, "auto-1", mock.AnythingOfType("time.Duration")).Return(&lockUntil, nil)
	runRepo.On("ReleaseLock", mock.Anything, "auto-1").Return(nil)

	historyEntry := &models.ActionHistory{ID: "hist-1"}
	deps.historyRepo.On("Create", mock.Anything, "user-1", "automation_run", mock.Anything, mock.Anything).Return(historyEntry, nil)
	deps.historyRepo.On("Update", mock.Anything, "hist-1", mock.Anything).Return(nil)
	deps.historyRepo.On("MarkRunning", mock.Anything, "hist-1").Return(nil)
	deps.historyRepo.On("UpdateCounts", mock.Anything, "hist-1", int32(1), int32(0)).Return(nil)

	// UpdateLastRunAt returns error (logged as warning)
	deps.automationRepo.On("UpdateLastRunAt", mock.Anything, "auto-1").Return(errors.New("last run at update failed"))

	deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
		Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)

	offers := []models.ShopeeOffer{
		{Name: "Product 1", ProductURL: "https://shopee.com/p1", Price: 99.90, OriginalPrice: 199.90, Discount: 50},
	}
	shopee.On("SearchOffers", mock.Anything, "user-1", mock.Anything).
		Return(&models.ShopeeSearchResult{Offers: offers}, nil)
	shopee.On("GenerateAffiliateLink", mock.Anything, mock.Anything, "user-1").
		Return(&models.ShopeeAffiliateLink{AffiliateLink: "https://s.shopee.com/aff1"}, nil)

	deps.dispatchLogRepo.On("GetRecentByCanonicalKey", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, nil)
	deps.dispatchLogRepo.On("Create", mock.Anything, mock.Anything).Return(&models.DispatchLog{}, nil).Maybe()

	result, err := svc.RunAutomation(context.Background(), "auto-1", "user-1")
	require.NoError(t, err)
	require.NotNil(t, result)
	// Still succeeds despite UpdateLastRunAt error
	assert.True(t, result.Success)
}

// =============================================================================
// RunAutomation: GlobalMetrics recording path (lines 440-446)
// =============================================================================

func TestRunAutomation_GlobalMetricsRecording(t *testing.T) {
	// Not parallel: mutates observability.GlobalMetrics
	svc, deps := newInternalTestService(t)
	runRepo := new(mockInternalRunRepo)
	svc.automationRunRepo = runRepo

	shopee := new(mockShopeeService)
	svc.shopeeService = shopee

	t.Setenv("FEATURE_USE_GO_DISPATCH_LOGS", "true")
	config.ResetFeatureFlags()

	// Set up GlobalMetrics
	origMetrics := observability.GlobalMetrics
	t.Cleanup(func() { observability.GlobalMetrics = origMetrics })
	err := observability.InitMetrics(true)
	require.NoError(t, err)
	require.NotNil(t, observability.GlobalMetrics)

	searchType := "search"
	mode := "search"
	automation := &models.Automation{
		ID:       "auto-1",
		UserID:   "user-1",
		Name:     "Test Auto",
		Type:     &searchType,
		Mode:     &mode,
		GroupIDs: []string{"group@g.us"},
		Stores:   []string{"shopee"},
	}

	lockUntil := time.Now().Add(5 * time.Minute)
	deps.automationRepo.On("GetAutomationByIDAndUser", mock.Anything, "auto-1", "user-1").Return(automation, nil)
	runRepo.On("GetLock", mock.Anything, "auto-1").Return(nil, nil)
	runRepo.On("AcquireLock", mock.Anything, "auto-1", mock.AnythingOfType("time.Duration")).Return(&lockUntil, nil)
	runRepo.On("ReleaseLock", mock.Anything, "auto-1").Return(nil)

	historyEntry := &models.ActionHistory{ID: "hist-1"}
	deps.historyRepo.On("Create", mock.Anything, "user-1", "automation_run", mock.Anything, mock.Anything).Return(historyEntry, nil)
	deps.historyRepo.On("Update", mock.Anything, "hist-1", mock.Anything).Return(nil)
	deps.historyRepo.On("MarkRunning", mock.Anything, "hist-1").Return(nil)
	deps.historyRepo.On("UpdateCounts", mock.Anything, "hist-1", mock.Anything, mock.Anything).Return(nil)

	deps.automationRepo.On("UpdateLastRunAt", mock.Anything, "auto-1").Return(nil)

	deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
		Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)

	offers := []models.ShopeeOffer{
		{Name: "Product 1", ProductURL: "https://shopee.com/p1", Price: 99.90, OriginalPrice: 199.90, Discount: 50},
	}
	shopee.On("SearchOffers", mock.Anything, "user-1", mock.Anything).
		Return(&models.ShopeeSearchResult{Offers: offers}, nil)
	shopee.On("GenerateAffiliateLink", mock.Anything, mock.Anything, "user-1").
		Return(&models.ShopeeAffiliateLink{AffiliateLink: "https://s.shopee.com/aff1"}, nil)

	deps.dispatchLogRepo.On("GetRecentByCanonicalKey", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, nil)
	deps.dispatchLogRepo.On("Create", mock.Anything, mock.Anything).Return(&models.DispatchLog{}, nil).Maybe()

	result, runErr := svc.RunAutomation(context.Background(), "auto-1", "user-1")
	require.NoError(t, runErr)
	require.NotNil(t, result)
	assert.True(t, result.Success)
	// GlobalMetrics.AutomationRuns.Add was called (no panic)
}

func TestRunAutomation_GlobalMetricsNilMode(t *testing.T) {
	// Not parallel: mutates observability.GlobalMetrics
	svc, deps := newInternalTestService(t)
	runRepo := new(mockInternalRunRepo)
	svc.automationRunRepo = runRepo

	shopee := new(mockShopeeService)
	svc.shopeeService = shopee

	t.Setenv("FEATURE_USE_GO_DISPATCH_LOGS", "true")
	config.ResetFeatureFlags()

	// Set up GlobalMetrics
	origMetrics := observability.GlobalMetrics
	t.Cleanup(func() { observability.GlobalMetrics = origMetrics })
	err := observability.InitMetrics(true)
	require.NoError(t, err)

	searchType := "search"
	// Mode is nil → should default to "unknown"
	automation := &models.Automation{
		ID:       "auto-1",
		UserID:   "user-1",
		Name:     "Test Auto",
		Type:     &searchType,
		Mode:     nil,
		GroupIDs: []string{"group@g.us"},
		Stores:   []string{"shopee"},
	}

	lockUntil := time.Now().Add(5 * time.Minute)
	deps.automationRepo.On("GetAutomationByIDAndUser", mock.Anything, "auto-1", "user-1").Return(automation, nil)
	runRepo.On("GetLock", mock.Anything, "auto-1").Return(nil, nil)
	runRepo.On("AcquireLock", mock.Anything, "auto-1", mock.AnythingOfType("time.Duration")).Return(&lockUntil, nil)
	runRepo.On("ReleaseLock", mock.Anything, "auto-1").Return(nil)

	historyEntry := &models.ActionHistory{ID: "hist-1"}
	deps.historyRepo.On("Create", mock.Anything, "user-1", "automation_run", mock.Anything, mock.Anything).Return(historyEntry, nil)
	deps.historyRepo.On("Update", mock.Anything, "hist-1", mock.Anything).Return(nil)
	deps.historyRepo.On("MarkRunning", mock.Anything, "hist-1").Return(nil)
	deps.historyRepo.On("UpdateCounts", mock.Anything, "hist-1", mock.Anything, mock.Anything).Return(nil)

	deps.automationRepo.On("UpdateLastRunAt", mock.Anything, "auto-1").Return(nil)

	deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
		Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)

	offers := []models.ShopeeOffer{
		{Name: "Product 1", ProductURL: "https://shopee.com/p1", Price: 99.90, OriginalPrice: 199.90, Discount: 50},
	}
	shopee.On("SearchOffers", mock.Anything, "user-1", mock.Anything).
		Return(&models.ShopeeSearchResult{Offers: offers}, nil)
	shopee.On("GenerateAffiliateLink", mock.Anything, mock.Anything, "user-1").
		Return(&models.ShopeeAffiliateLink{AffiliateLink: "https://s.shopee.com/aff1"}, nil)

	deps.dispatchLogRepo.On("GetRecentByCanonicalKey", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, nil)
	deps.dispatchLogRepo.On("Create", mock.Anything, mock.Anything).Return(&models.DispatchLog{}, nil).Maybe()

	result, runErr := svc.RunAutomation(context.Background(), "auto-1", "user-1")
	require.NoError(t, runErr)
	require.NotNil(t, result)
	assert.True(t, result.Success)
	// GlobalMetrics.AutomationRuns.Add was called with mode="unknown" (no panic)
}

// =============================================================================
// executeSearchAutomation: per-group dedup hit inside loop (lines 659-666)
// =============================================================================

func TestExecuteSearchAutomation_PerGroupDedupHit(t *testing.T) {
	// Not parallel: uses t.Setenv
	svc, deps := newInternalTestService(t)
	shopee := new(mockShopeeService)
	svc.shopeeService = shopee

	// Use real cache so isDuplicate works per-group
	c, _ := newTestCache(t)
	svc.cache = c

	t.Setenv("FEATURE_USE_GO_DISPATCH_LOGS", "true")
	config.ResetFeatureFlags()

	keyword := "test"
	automation := &models.Automation{
		ID:       "auto-1",
		UserID:   "user-1",
		GroupIDs: []string{"group1@g.us", "group2@g.us"},
		Stores:   []string{"shopee"},
		Keyword:  &keyword,
	}

	deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
		Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)

	offers := []models.ShopeeOffer{
		{Name: "Product 1", ProductURL: "https://shopee.com/p1", Price: 99.90, OriginalPrice: 199.90, Discount: 50},
	}
	shopee.On("SearchOffers", mock.Anything, "user-1", mock.Anything).
		Return(&models.ShopeeSearchResult{Offers: offers}, nil)
	shopee.On("GenerateAffiliateLink", mock.Anything, mock.Anything, "user-1").
		Return(&models.ShopeeAffiliateLink{AffiliateLink: "https://s.shopee.com/aff1"}, nil)

	// group1 is NOT a duplicate (DB returns nil)
	deps.dispatchLogRepo.On("GetRecentByCanonicalKey", mock.Anything, "user-1", "group1@g.us", mock.Anything, mock.Anything).
		Return(nil, nil)
	// group2 IS a duplicate (DB returns existing dispatch)
	deps.dispatchLogRepo.On("GetRecentByCanonicalKey", mock.Anything, "user-1", "group2@g.us", mock.Anything, mock.Anything).
		Return(&models.DispatchLog{Status: "queued"}, nil)

	deps.dispatchLogRepo.On("Create", mock.Anything, mock.Anything).Return(&models.DispatchLog{}, nil).Maybe()

	result, err := svc.executeSearchAutomation(context.Background(), automation, "hist-1")
	require.NoError(t, err)
	// group1 sent, group2 skipped due to dedup
	assert.Equal(t, 1, result.MessagesSent)
}

// =============================================================================
// executeSearchAutomation: backpressure CanEnqueue error path (lines 695-699)
// =============================================================================

func TestExecuteSearchAutomation_BackpressureCanEnqueueError(t *testing.T) {
	// Not parallel: uses t.Setenv
	svc, deps := newInternalTestService(t)
	shopee := new(mockShopeeService)
	svc.shopeeService = shopee

	t.Setenv("FEATURE_USE_GO_DISPATCH_LOGS", "true")
	config.ResetFeatureFlags()

	logger := zaptest.NewLogger(t)

	// Use a real backpressure with closed Redis to force CanEnqueue error
	bpCache, bpMr := newTestCache(t)
	bp := bpLib.NewProducerBackpressure(bpLib.Config{
		Enabled:        true,
		UserMaxEnqueued: 100,
	}, bpCache, logger)
	svc.backpressure = bp
	bpMr.Close() // force Redis error → CanEnqueue returns error but allows enqueue

	keyword := "test"
	automation := &models.Automation{
		ID:       "auto-1",
		UserID:   "user-1",
		GroupIDs: []string{"group@g.us"},
		Stores:   []string{"shopee"},
		Keyword:  &keyword,
	}

	deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
		Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)

	offers := []models.ShopeeOffer{
		{Name: "Product 1", ProductURL: "https://shopee.com/p1", Price: 99.90, OriginalPrice: 199.90, Discount: 50},
	}
	shopee.On("SearchOffers", mock.Anything, "user-1", mock.Anything).
		Return(&models.ShopeeSearchResult{Offers: offers}, nil)
	shopee.On("GenerateAffiliateLink", mock.Anything, mock.Anything, "user-1").
		Return(&models.ShopeeAffiliateLink{AffiliateLink: "https://s.shopee.com/aff1"}, nil)

	deps.dispatchLogRepo.On("GetRecentByCanonicalKey", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, nil)
	deps.dispatchLogRepo.On("Create", mock.Anything, mock.Anything).Return(&models.DispatchLog{}, nil).Maybe()

	result, err := svc.executeSearchAutomation(context.Background(), automation, "hist-1")
	require.NoError(t, err)
	// Should still send since CanEnqueue error allows enqueue (fail-open)
	assert.Equal(t, 1, result.MessagesSent)
}

// =============================================================================
// executeSearchAutomation: backpressure RecordEnqueue error (lines 744-748)
// =============================================================================

func TestExecuteSearchAutomation_BackpressureRecordEnqueueError(t *testing.T) {
	// Not parallel: uses t.Setenv
	svc, deps := newInternalTestService(t)
	shopee := new(mockShopeeService)
	svc.shopeeService = shopee

	t.Setenv("FEATURE_USE_GO_DISPATCH_LOGS", "true")
	config.ResetFeatureFlags()

	logger := zaptest.NewLogger(t)

	// Create backpressure with running miniredis for CanEnqueue, then close for RecordEnqueue
	bpCache, bpMr := newTestCache(t)
	bp := bpLib.NewProducerBackpressure(bpLib.Config{
		Enabled:        true,
		UserMaxEnqueued: 100,
	}, bpCache, logger)
	svc.backpressure = bp

	keyword := "test"
	automation := &models.Automation{
		ID:       "auto-1",
		UserID:   "user-1",
		GroupIDs: []string{"group@g.us"},
		Stores:   []string{"shopee"},
		Keyword:  &keyword,
	}

	deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
		Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)

	offers := []models.ShopeeOffer{
		{Name: "Product 1", ProductURL: "https://shopee.com/p1", Price: 99.90, OriginalPrice: 199.90, Discount: 50},
	}
	shopee.On("SearchOffers", mock.Anything, "user-1", mock.Anything).
		Return(&models.ShopeeSearchResult{Offers: offers}, nil)
	shopee.On("GenerateAffiliateLink", mock.Anything, mock.Anything, "user-1").
		Return(&models.ShopeeAffiliateLink{AffiliateLink: "https://s.shopee.com/aff1"}, nil)

	deps.dispatchLogRepo.On("GetRecentByCanonicalKey", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, nil)
	deps.dispatchLogRepo.On("Create", mock.Anything, mock.Anything).Return(&models.DispatchLog{}, nil).Maybe()

	// Close miniredis AFTER CanEnqueue succeeds but before RecordEnqueue
	// Since we can't time this precisely, close redis before running — CanEnqueue error is fail-open,
	// so it proceeds to send, then RecordEnqueue also fails
	bpMr.Close()

	result, err := svc.executeSearchAutomation(context.Background(), automation, "hist-1")
	require.NoError(t, err)
	// Still sends (CanEnqueue error is fail-open), RecordEnqueue error is logged but not fatal
	assert.Equal(t, 1, result.MessagesSent)
}

// =============================================================================
// executeSearchAutomation: queueClient enqueue error in per-group loop (lines 781-783)
// =============================================================================

func TestExecuteSearchAutomation_QueueClientEnqueueErrorPerGroup(t *testing.T) {
	// Not parallel: uses t.Setenv
	svc, deps := newInternalTestService(t)
	shopee := new(mockShopeeService)
	svc.shopeeService = shopee

	t.Setenv("FEATURE_USE_GO_DISPATCH_LOGS", "true")
	config.ResetFeatureFlags()

	logger := zaptest.NewLogger(t)

	// Create queue client with closed miniredis → enqueue will fail
	mr := miniredis.RunT(t)
	qc := workers.NewQueueClient(mr.Addr(), logger)
	t.Cleanup(func() { qc.Close() })
	svc.queueClient = qc
	mr.Close()

	keyword := "test"
	automation := &models.Automation{
		ID:       "auto-1",
		UserID:   "user-1",
		GroupIDs: []string{"group1@g.us", "group2@g.us"},
		Stores:   []string{"shopee"},
		Keyword:  &keyword,
	}

	deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
		Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)

	offers := []models.ShopeeOffer{
		{Name: "Product 1", ProductURL: "https://shopee.com/p1", Price: 99.90, OriginalPrice: 199.90, Discount: 50},
	}
	shopee.On("SearchOffers", mock.Anything, "user-1", mock.Anything).
		Return(&models.ShopeeSearchResult{Offers: offers}, nil)
	shopee.On("GenerateAffiliateLink", mock.Anything, mock.Anything, "user-1").
		Return(&models.ShopeeAffiliateLink{AffiliateLink: "https://s.shopee.com/aff1"}, nil)

	deps.dispatchLogRepo.On("GetRecentByCanonicalKey", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, nil)
	deps.dispatchLogRepo.On("Create", mock.Anything, mock.Anything).Return(&models.DispatchLog{}, nil).Maybe()

	result, err := svc.executeSearchAutomation(context.Background(), automation, "hist-1")
	require.NoError(t, err)
	// Both groups fail enqueue → 0 sent
	assert.Equal(t, 0, result.MessagesSent)
}

// =============================================================================
// formatCouponText: empty code produces fallback return "" (line 195)
// This is unreachable in practice since line 155 always adds the coupon code line.
// However, we can test the function directly to confirm it always returns non-empty.
// =============================================================================

func TestFormatCouponText_EmptyCode(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	handler := &CouponHandler{logger: logger}

	// Even with empty code, line 155 still appends "🏷️ CUPOM: " so lines > 0
	coupon := &models.Coupon{Code: ""}
	result := handler.formatCouponText(coupon)
	// Should not be empty since the code line is always added
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "CUPOM:")
}

// =============================================================================
// processListMode: URL safety evaluate error (line 210-214)
// =============================================================================

func TestProcessListMode_URLSafetyEvaluateError(t *testing.T) {
	// Not parallel: uses t.Setenv
	svc, deps := newInternalTestService(t)

	t.Setenv("FEATURE_USE_GO_DISPATCH_LOGS", "true")
	config.ResetFeatureFlags()

	logger := zaptest.NewLogger(t)

	// URL safety service that returns an error on Evaluate
	usSvc := urlsafety.NewService(urlsafety.Config{
		Mode:         "block",
		BlockedHosts: []string{"blocked.example.com"},
	}, nil, logger)
	svc.urlSafetyService = usSvc

	listID := "list-1"
	automation := &models.Automation{
		ID:           "auto-1",
		UserID:       "user-1",
		GroupIDs:     []string{"group@g.us"},
		MlLinkListID: &listID,
	}

	deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
		Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)

	items := []*models.MlLinkListItem{
		{ID: "item-1", OriginURL: "https://blocked.example.com/product-1"},
	}
	deps.mlLinkListItemRepo.On("GetNextUnconsumed", mock.Anything, "list-1", 1).Return(items, nil)
	deps.mlLinkListItemRepo.On("MarkConsumed", mock.Anything, "item-1").Return(nil).Maybe()
	deps.dispatchLogRepo.On("Create", mock.Anything, mock.Anything).Return(&models.DispatchLog{}, nil).Maybe()

	result, err := svc.processListMode(context.Background(), automation)
	require.NoError(t, err)
	// Item with blocked URL should be skipped
	assert.NotNil(t, result)
}

// =============================================================================
// validateMonitorAutomation: time window with ScheduleDays + inside/outside (lines 273-286)
// =============================================================================

func TestValidateMonitorAutomation_TimeWindowOutsideWithDays(t *testing.T) {
	t.Parallel()
	svc, deps := newInternalTestService(t)

	// Use a time window that is guaranteed to be OUTSIDE current time (03:00-03:01)
	startTime := "03:00"
	endTime := "03:01"
	automation := &models.Automation{
		ID:              "auto-1",
		UserID:          "user-1",
		MonitorGroupIDs: []string{"source@g.us"},
		GroupIDs:        []string{"target@g.us"},
		StartTime:       &startTime,
		EndTime:         &endTime,
		ScheduleDays:    []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"},
	}

	deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
		Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)

	result, err := svc.validateMonitorAutomation(context.Background(), automation)
	require.NoError(t, err)

	var timeCheck *models.DiagnoseCheck
	for _, c := range result.Checks {
		if c.Name == "JANELA_HORARIO" {
			timeCheck = &c
			break
		}
	}
	require.NotNil(t, timeCheck)
	assert.Equal(t, "warning", timeCheck.Status)
	assert.Contains(t, timeCheck.Message, "Fora da janela")
	// Result should have warning
	assert.Contains(t, result.Warnings, timeCheck.Message)
}

func TestValidateMonitorAutomation_TimeWindowInsideWithDays(t *testing.T) {
	t.Parallel()
	svc, deps := newInternalTestService(t)

	// Use a time window that covers the entire day (00:00-23:59)
	startTime := "00:00"
	endTime := "23:59"

	// Use all days of the week to ensure it matches regardless of test run day
	automation := &models.Automation{
		ID:              "auto-1",
		UserID:          "user-1",
		MonitorGroupIDs: []string{"source@g.us"},
		GroupIDs:        []string{"target@g.us"},
		StartTime:       &startTime,
		EndTime:         &endTime,
		ScheduleDays:    []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"},
	}

	deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
		Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)

	result, err := svc.validateMonitorAutomation(context.Background(), automation)
	require.NoError(t, err)

	var timeCheck *models.DiagnoseCheck
	for _, c := range result.Checks {
		if c.Name == "JANELA_HORARIO" {
			timeCheck = &c
			break
		}
	}
	require.NotNil(t, timeCheck)
	assert.Equal(t, "ok", timeCheck.Status)
	assert.Contains(t, timeCheck.Message, "Dentro da janela")
}

// =============================================================================
// executeSearchAutomation: backpressure CanEnqueue error via mock (line 695-699)
// =============================================================================

func TestExecuteSearchAutomation_BackpressureCanEnqueueError_Mock(t *testing.T) {
	// Not parallel: uses t.Setenv
	svc, deps := newInternalTestService(t)
	shopee := new(mockShopeeService)
	svc.shopeeService = shopee

	t.Setenv("FEATURE_USE_GO_DISPATCH_LOGS", "true")
	config.ResetFeatureFlags()

	// Use mock backpressure that returns an error from CanEnqueue
	bp := new(mockBackpressureProvider)
	bp.On("CanEnqueue", mock.Anything, "user-1").Return(true, "", errors.New("redis connection error"))
	bp.On("RecordEnqueue", mock.Anything, "user-1").Return(nil)
	svc.backpressure = bp

	keyword := "test"
	automation := &models.Automation{
		ID:       "auto-1",
		UserID:   "user-1",
		GroupIDs: []string{"group@g.us"},
		Stores:   []string{"shopee"},
		Keyword:  &keyword,
	}

	deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
		Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)

	offers := []models.ShopeeOffer{
		{Name: "Product 1", ProductURL: "https://shopee.com/p1", Price: 99.90, OriginalPrice: 199.90, Discount: 50},
	}
	shopee.On("SearchOffers", mock.Anything, "user-1", mock.Anything).
		Return(&models.ShopeeSearchResult{Offers: offers}, nil)
	shopee.On("GenerateAffiliateLink", mock.Anything, mock.Anything, "user-1").
		Return(&models.ShopeeAffiliateLink{AffiliateLink: "https://s.shopee.com/aff1"}, nil)

	deps.dispatchLogRepo.On("GetRecentByCanonicalKey", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, nil)
	deps.dispatchLogRepo.On("Create", mock.Anything, mock.Anything).Return(&models.DispatchLog{}, nil).Maybe()

	result, err := svc.executeSearchAutomation(context.Background(), automation, "hist-1")
	require.NoError(t, err)
	// CanEnqueue error is fail-open → still sends
	assert.Equal(t, 1, result.MessagesSent)
	bp.AssertCalled(t, "CanEnqueue", mock.Anything, "user-1")
}

// =============================================================================
// executeSearchAutomation: backpressure RecordEnqueue error via mock (line 744-748)
// =============================================================================

func TestExecuteSearchAutomation_BackpressureRecordEnqueueError_Mock(t *testing.T) {
	// Not parallel: uses t.Setenv
	svc, deps := newInternalTestService(t)
	shopee := new(mockShopeeService)
	svc.shopeeService = shopee

	t.Setenv("FEATURE_USE_GO_DISPATCH_LOGS", "true")
	config.ResetFeatureFlags()

	// Use mock backpressure: CanEnqueue succeeds, RecordEnqueue returns error
	bp := new(mockBackpressureProvider)
	bp.On("CanEnqueue", mock.Anything, "user-1").Return(true, "", nil)
	bp.On("RecordEnqueue", mock.Anything, "user-1").Return(errors.New("redis write error"))
	svc.backpressure = bp

	keyword := "test"
	automation := &models.Automation{
		ID:       "auto-1",
		UserID:   "user-1",
		GroupIDs: []string{"group@g.us"},
		Stores:   []string{"shopee"},
		Keyword:  &keyword,
	}

	deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
		Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)

	offers := []models.ShopeeOffer{
		{Name: "Product 1", ProductURL: "https://shopee.com/p1", Price: 99.90, OriginalPrice: 199.90, Discount: 50},
	}
	shopee.On("SearchOffers", mock.Anything, "user-1", mock.Anything).
		Return(&models.ShopeeSearchResult{Offers: offers}, nil)
	shopee.On("GenerateAffiliateLink", mock.Anything, mock.Anything, "user-1").
		Return(&models.ShopeeAffiliateLink{AffiliateLink: "https://s.shopee.com/aff1"}, nil)

	deps.dispatchLogRepo.On("GetRecentByCanonicalKey", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, nil)
	deps.dispatchLogRepo.On("Create", mock.Anything, mock.Anything).Return(&models.DispatchLog{}, nil).Maybe()

	result, err := svc.executeSearchAutomation(context.Background(), automation, "hist-1")
	require.NoError(t, err)
	// RecordEnqueue error is logged but not fatal → message still sent
	assert.Equal(t, 1, result.MessagesSent)
	bp.AssertCalled(t, "RecordEnqueue", mock.Anything, "user-1")
}

// =============================================================================
// processListMode: URL safety Evaluate error via mock (line 210-214)
// =============================================================================

func TestProcessListMode_URLSafetyEvaluateError_Mock(t *testing.T) {
	// Not parallel: uses t.Setenv
	svc, deps := newInternalTestService(t)

	t.Setenv("FEATURE_USE_GO_DISPATCH_LOGS", "true")
	config.ResetFeatureFlags()

	// Use mock URL safety evaluator that returns an error
	usMock := new(mockURLSafetyEvaluator)
	usMock.On("Evaluate", mock.Anything, mock.Anything).Return(nil, errors.New("safety check unavailable"))
	svc.urlSafetyService = usMock

	listID := "list-1"
	automation := &models.Automation{
		ID:           "auto-1",
		UserID:       "user-1",
		GroupIDs:     []string{"group@g.us"},
		MlLinkListID: &listID,
	}

	deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").
		Return([]models.Instance{{ID: 1, InstanceID: "inst-1", Status: "connected"}}, nil)

	items := []*models.MlLinkListItem{
		{ID: "item-1", OriginURL: "https://www.mercadolivre.com.br/product-1"},
	}
	deps.mlLinkListItemRepo.On("GetNextUnconsumed", mock.Anything, "list-1", 1).Return(items, nil)
	deps.mlLinkListItemRepo.On("MarkAsConsumed", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	deps.dispatchLogRepo.On("Create", mock.Anything, mock.Anything).Return(&models.DispatchLog{}, nil).Maybe()

	result, err := svc.processListMode(context.Background(), automation)
	require.NoError(t, err)
	// Evaluate error is logged as warning but not blocking → message still sent
	assert.Equal(t, 1, result.MessagesSent)
	usMock.AssertCalled(t, "Evaluate", mock.Anything, mock.Anything)
}
