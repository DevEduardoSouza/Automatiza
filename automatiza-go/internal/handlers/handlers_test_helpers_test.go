package handlers

import (
	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/config"
	"automatiza-go/internal/middleware"
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"
	"automatiza-go/internal/services/coupons"
	"automatiza-go/internal/services/jobs"
	"automatiza-go/internal/services/loadbalancer"
	"automatiza-go/internal/services/user"
	"automatiza-go/internal/workers"
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	"go.uber.org/zap"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// ---------------------------------------------------------------------------
// Test context helpers
// ---------------------------------------------------------------------------

// setAuthUser injects an authenticated user into the gin context.
func setAuthUser(c *gin.Context, userID, email, role string) {
	c.Set(string(middleware.UserContextKey), &middleware.AuthenticatedUser{
		ID:    userID,
		Email: email,
		Role:  role,
	})
}

// newTestContext creates a gin context with recorder, body, and optionally injected user.
func newTestContext(method, path, body string) (*httptest.ResponseRecorder, *gin.Context) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	if body != "" {
		c.Request = httptest.NewRequest(method, path, strings.NewReader(body))
	} else {
		c.Request = httptest.NewRequest(method, path, nil)
	}
	c.Request.Header.Set("Content-Type", "application/json")
	return w, c
}

// newAuthContext creates a gin context with an authenticated user.
func newAuthContext(method, path, body string) (*httptest.ResponseRecorder, *gin.Context) {
	w, c := newTestContext(method, path, body)
	setAuthUser(c, "user-123", "user@test.com", "user")
	return w, c
}

// newAdminContext creates a gin context with an admin user.
func newAdminContext(method, path, body string) (*httptest.ResponseRecorder, *gin.Context) {
	w, c := newTestContext(method, path, body)
	setAuthUser(c, "admin-123", "admin@test.com", "admin")
	return w, c
}

// testLogger returns a no-op logger for tests.
func testLogger() *zap.Logger {
	return zap.NewNop()
}

// testConfig returns a minimal config for tests.
func testConfig() *config.Config {
	return &config.Config{
		APIVersion:       "test",
		NodeEnv:          "test",
		BackendPublicURL: "http://localhost:3000",
	}
}

// mockBackpressureChecker implements BackpressureChecker for tests.
type mockBackpressureChecker struct {
	CanEnqueueFunc    func(ctx context.Context, userID string) (bool, string, error)
	RecordEnqueueFunc func(ctx context.Context, userID string) error
}

func (m *mockBackpressureChecker) CanEnqueue(ctx context.Context, userID string) (bool, string, error) {
	if m.CanEnqueueFunc != nil {
		return m.CanEnqueueFunc(ctx, userID)
	}
	return true, "", nil
}

func (m *mockBackpressureChecker) RecordEnqueue(ctx context.Context, userID string) error {
	if m.RecordEnqueueFunc != nil {
		return m.RecordEnqueueFunc(ctx, userID)
	}
	return nil
}

// newTestDeps creates a Dependencies struct with all mocks wired up.
func newTestDeps() (*Dependencies, *testMocks) {
	mocks := &testMocks{
		ProfileRepo:       &mockProfileRepo{},
		InstanceRepo:      &mockInstanceRepo{},
		GroupRepo:         &mockGroupRepo{},
		AutomationRepo:    &mockAutomationRepo{},
		ActionHistoryRepo: &mockActionHistoryRepo{},
		SettingsRepo:      &mockSettingsRepo{},
		CouponRepo:        &mockCouponRepo{},
		DispatchLogRepo:   &mockDispatchLogRepo{},
		ScheduledJobRepo:  &mockScheduledJobRepo{},
		SystemEventRepo:   &mockSystemEventRepo{},
		UserRoleRepo:      &mockUserRoleRepo{},
		WebhookEventsRepo: &MockWebhookEventsRepository{},
		WhatsAppProvider:  &mockWhatsAppProvider{},
		ShopeeService:     &mockShopeeService{},
		MLService:         &mockMLService{},
		MonitoringService: &mockMonitoringService{},
		JobsService:       &mockJobsService{},
		UserService:       &mockUserService{},
		CouponsService:    &mockCouponsService{},
		LBService:         &mockLBService{},
		RedirectService:   &mockRedirectService{},
		KiwifyService:     &mockKiwifyService{},
		QueueClient:       &mockQueueClientWrapper{},
		Backpressure:      &mockBackpressureChecker{},
	}

	deps := &Dependencies{
		Config:            testConfig(),
		Cache:             nil,
		Logger:            testLogger(),
		WhatsAppProvider:  mocks.WhatsAppProvider,
		ShopeeService:     mocks.ShopeeService,
		MLService:         mocks.MLService,
		MonitoringService: mocks.MonitoringService,
		JobsService:       mocks.JobsService,
		UserService:       mocks.UserService,
		CouponsService:    mocks.CouponsService,
		LBService:         mocks.LBService,
		RedirectService:   mocks.RedirectService,
		KiwifyService:     mocks.KiwifyService,
		ProfileRepo:       mocks.ProfileRepo,
		InstanceRepo:      mocks.InstanceRepo,
		GroupRepo:         mocks.GroupRepo,
		AutomationRepo:    mocks.AutomationRepo,
		ActionHistoryRepo: mocks.ActionHistoryRepo,
		SettingsRepo:      mocks.SettingsRepo,
		CouponRepo:        mocks.CouponRepo,
		DispatchLogRepo:   mocks.DispatchLogRepo,
		ScheduledJobRepo:  mocks.ScheduledJobRepo,
		SystemEventRepo:   mocks.SystemEventRepo,
		UserRoleRepo:      mocks.UserRoleRepo,
		WebhookEventsRepo: mocks.WebhookEventsRepo,
		Backpressure:      mocks.Backpressure,
		QueueClient:       nil, // set via mock wrapper where needed
	}

	return deps, mocks
}

// testMocks holds all mock instances for easy access in tests.
type testMocks struct {
	ProfileRepo       *mockProfileRepo
	InstanceRepo      *mockInstanceRepo
	GroupRepo         *mockGroupRepo
	AutomationRepo    *mockAutomationRepo
	ActionHistoryRepo *mockActionHistoryRepo
	SettingsRepo      *mockSettingsRepo
	CouponRepo        *mockCouponRepo
	DispatchLogRepo   *mockDispatchLogRepo
	ScheduledJobRepo  *mockScheduledJobRepo
	SystemEventRepo   *mockSystemEventRepo
	UserRoleRepo      *mockUserRoleRepo
	WebhookEventsRepo *MockWebhookEventsRepository
	WhatsAppProvider  *mockWhatsAppProvider
	ShopeeService     *mockShopeeService
	MLService         *mockMLService
	MonitoringService *mockMonitoringService
	JobsService       *mockJobsService
	UserService       *mockUserService
	CouponsService    *mockCouponsService
	LBService         *mockLBService
	RedirectService   *mockRedirectService
	KiwifyService     *mockKiwifyService
	QueueClient       *mockQueueClientWrapper
	Backpressure      *mockBackpressureChecker
}

// ---------------------------------------------------------------------------
// Mock: ProfileRepository
// ---------------------------------------------------------------------------

type mockProfileRepo struct {
	GetProfileFunc func(ctx context.Context, userID string) (*models.Profile, error)
}

func (m *mockProfileRepo) EnsureUserProfile(ctx context.Context, userID, email string) (*models.Profile, error) {
	return &models.Profile{ID: userID, Email: email}, nil
}
func (m *mockProfileRepo) GetProfile(ctx context.Context, userID string) (*models.Profile, error) {
	if m.GetProfileFunc != nil {
		return m.GetProfileFunc(ctx, userID)
	}
	return &models.Profile{ID: userID}, nil
}
func (m *mockProfileRepo) GetProfileByEmail(ctx context.Context, email string) (*models.Profile, error) {
	return nil, nil
}
func (m *mockProfileRepo) GetProfileByEmailWithBilling(ctx context.Context, email string) (*models.Profile, *models.ProfileBillingFields, error) {
	return nil, nil, nil
}
func (m *mockProfileRepo) CreateProfile(ctx context.Context, profile *models.Profile) (*models.Profile, error) {
	return profile, nil
}
func (m *mockProfileRepo) CreateUserCredentials(ctx context.Context, userID, passwordHash string) error {
	return nil
}
func (m *mockProfileRepo) UpdateProfile(ctx context.Context, userID string, updates map[string]interface{}) error {
	return nil
}
func (m *mockProfileRepo) UpdatePassword(ctx context.Context, userID, passwordHash string) error {
	return nil
}
func (m *mockProfileRepo) UpdateBillingStatus(ctx context.Context, userID string, billing *models.ProfileBillingFields) error {
	return nil
}
func (m *mockProfileRepo) PauseUserAutomations(ctx context.Context, userID string) error {
	return nil
}
func (m *mockProfileRepo) ListProfiles(ctx context.Context, cursor string, limit int) ([]*models.Profile, string, error) {
	return nil, "", nil
}
func (m *mockProfileRepo) SearchProfiles(ctx context.Context, query, cursor string, limit int) ([]*models.Profile, string, error) {
	return nil, "", nil
}
func (m *mockProfileRepo) GetMaxAutomations(ctx context.Context, userID string) (int, error) {
	return 10, nil
}
func (m *mockProfileRepo) CountUserAutomations(ctx context.Context, userID string) (int, error) {
	return 0, nil
}
func (m *mockProfileRepo) GetProfileStats(ctx context.Context) (*repository.ProfileStats, error) {
	return nil, nil
}

// ---------------------------------------------------------------------------
// Mock: InstanceRepository
// ---------------------------------------------------------------------------

type mockInstanceRepo struct {
	GetInstanceByUserAndNameFunc func(ctx context.Context, userID, name string) (*models.Instance, error)
	GetInstanceByNameFunc       func(ctx context.Context, name string) (*models.Instance, error)
	GetAllConnectedFunc         func(ctx context.Context) ([]models.Instance, error)
	UpsertInstanceFunc          func(ctx context.Context, userID string, data *models.Instance) (*models.Instance, error)
	UpdateInstanceStatusFunc    func(ctx context.Context, instanceID int, status string) error
	DeleteInstanceFunc          func(ctx context.Context, instanceID int) error
}

func (m *mockInstanceRepo) GetUserInstances(ctx context.Context, userID string) ([]models.Instance, error) {
	return nil, nil
}
func (m *mockInstanceRepo) GetInstanceByID(ctx context.Context, instanceID int) (*models.Instance, error) {
	return nil, nil
}
func (m *mockInstanceRepo) GetInstanceByName(ctx context.Context, name string) (*models.Instance, error) {
	if m.GetInstanceByNameFunc != nil {
		return m.GetInstanceByNameFunc(ctx, name)
	}
	return &models.Instance{ID: 1, InstanceID: name, Status: "connected"}, nil
}
func (m *mockInstanceRepo) GetInstanceByUserAndName(ctx context.Context, userID, name string) (*models.Instance, error) {
	if m.GetInstanceByUserAndNameFunc != nil {
		return m.GetInstanceByUserAndNameFunc(ctx, userID, name)
	}
	return &models.Instance{ID: 1, InstanceID: name, Status: "connected"}, nil
}
func (m *mockInstanceRepo) GetAllConnectedInstances(ctx context.Context) ([]models.Instance, error) {
	if m.GetAllConnectedFunc != nil {
		return m.GetAllConnectedFunc(ctx)
	}
	return []models.Instance{{ID: 1, InstanceID: "inst-1"}}, nil
}
func (m *mockInstanceRepo) UpsertInstance(ctx context.Context, userID string, data *models.Instance) (*models.Instance, error) {
	if m.UpsertInstanceFunc != nil {
		return m.UpsertInstanceFunc(ctx, userID, data)
	}
	data.ID = 1
	return data, nil
}
func (m *mockInstanceRepo) UpdateInstanceStatus(ctx context.Context, instanceID int, status string) error {
	if m.UpdateInstanceStatusFunc != nil {
		return m.UpdateInstanceStatusFunc(ctx, instanceID, status)
	}
	return nil
}
func (m *mockInstanceRepo) DeleteInstance(ctx context.Context, instanceID int) error {
	if m.DeleteInstanceFunc != nil {
		return m.DeleteInstanceFunc(ctx, instanceID)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Mock: GroupRepository
// ---------------------------------------------------------------------------

type mockGroupRepo struct {
	GetUserGroupsFunc  func(ctx context.Context, userID string, adminOnly bool) ([]models.Group, error)
	GetGroupByIDFunc   func(ctx context.Context, groupID string) (*models.Group, error)
	DeleteByInstanceFn func(ctx context.Context, userID string, instanceID int) error
}

func (m *mockGroupRepo) GetUserGroups(ctx context.Context, userID string, adminOnly bool) ([]models.Group, error) {
	if m.GetUserGroupsFunc != nil {
		return m.GetUserGroupsFunc(ctx, userID, adminOnly)
	}
	return []models.Group{{ID: "g1", UserID: userID, Name: "Test Group"}}, nil
}
func (m *mockGroupRepo) GetUserGroupsPaginated(ctx context.Context, userID string, page, pageSize int, adminOnly bool, search string) (*repository.GroupPaginatedResult, error) {
	return nil, nil
}
func (m *mockGroupRepo) GetGroupByID(ctx context.Context, groupID string) (*models.Group, error) {
	if m.GetGroupByIDFunc != nil {
		return m.GetGroupByIDFunc(ctx, groupID)
	}
	return &models.Group{ID: groupID, UserID: "user-123", Name: "Test Group"}, nil
}
func (m *mockGroupRepo) GetGroupByWAGroupID(ctx context.Context, userID, waGroupID string) (*models.Group, error) {
	return nil, nil
}
func (m *mockGroupRepo) GetGroupsByIDs(ctx context.Context, userID string, groupIDs []string) ([]models.Group, error) {
	return nil, nil
}
func (m *mockGroupRepo) GetFavoriteGroups(ctx context.Context, userID string) ([]models.Group, error) {
	return nil, nil
}
func (m *mockGroupRepo) CreateGroup(ctx context.Context, userID string, group *models.Group) (*models.Group, error) {
	return group, nil
}
func (m *mockGroupRepo) SyncGroups(ctx context.Context, userID string, instanceID int, groups []models.Group, options repository.SyncOptions) error {
	return nil
}
func (m *mockGroupRepo) SyncGroupsWithStats(ctx context.Context, userID string, instanceID int, groups []models.Group, options repository.SyncOptions) (*repository.SyncResult, error) {
	return nil, nil
}
func (m *mockGroupRepo) IsUserAdminInGroup(ctx context.Context, userID, waGroupID string) (bool, error) {
	return false, nil
}
func (m *mockGroupRepo) UpdateGroup(ctx context.Context, groupID string, updates map[string]interface{}) error {
	return nil
}
func (m *mockGroupRepo) ToggleFavorite(ctx context.Context, groupID, userID string, favorite bool) error {
	return nil
}
func (m *mockGroupRepo) DeleteGroup(ctx context.Context, groupID, userID string) error {
	return nil
}
func (m *mockGroupRepo) DeleteGroupsByInstance(ctx context.Context, userID string, instanceID int) error {
	if m.DeleteByInstanceFn != nil {
		return m.DeleteByInstanceFn(ctx, userID, instanceID)
	}
	return nil
}
func (m *mockGroupRepo) BulkUpdateGroups(ctx context.Context, userID string, groupIDs []string, updates map[string]interface{}) (int64, error) {
	return 0, nil
}
func (m *mockGroupRepo) CountAll(ctx context.Context) (int64, error) {
	return 0, nil
}

// ---------------------------------------------------------------------------
// Mock: AutomationRepository
// ---------------------------------------------------------------------------

type mockAutomationRepo struct {
	GetUserAutomationsFunc     func(ctx context.Context, userID string) ([]models.Automation, error)
	GetByIDAndUserFunc         func(ctx context.Context, id, userID string) (*models.Automation, error)
	CreateAutomationFunc       func(ctx context.Context, userID string, data *models.Automation) (*models.Automation, error)
	UpdateAutomationFunc       func(ctx context.Context, id string, updates map[string]interface{}) error
	DeleteAutomationFunc       func(ctx context.Context, id, userID string) error
}

func (m *mockAutomationRepo) GetUserAutomations(ctx context.Context, userID string) ([]models.Automation, error) {
	if m.GetUserAutomationsFunc != nil {
		return m.GetUserAutomationsFunc(ctx, userID)
	}
	return []models.Automation{{ID: "a1", Name: "Test"}}, nil
}
func (m *mockAutomationRepo) GetActiveAutomations(ctx context.Context) ([]models.Automation, error) {
	return nil, nil
}
func (m *mockAutomationRepo) GetAutomationByID(ctx context.Context, id string) (*models.Automation, error) {
	return nil, nil
}
func (m *mockAutomationRepo) GetAutomationByIDAndUser(ctx context.Context, id, userID string) (*models.Automation, error) {
	if m.GetByIDAndUserFunc != nil {
		return m.GetByIDAndUserFunc(ctx, id, userID)
	}
	status := models.AutomationStatusActive
	return &models.Automation{ID: id, Name: "Test", Status: status, GroupIDs: []string{"g1"}, SendGroups: []string{"g2"}}, nil
}
func (m *mockAutomationRepo) CreateAutomation(ctx context.Context, userID string, data *models.Automation) (*models.Automation, error) {
	if m.CreateAutomationFunc != nil {
		return m.CreateAutomationFunc(ctx, userID, data)
	}
	data.ID = "new-auto-1"
	return data, nil
}
func (m *mockAutomationRepo) UpdateAutomation(ctx context.Context, id string, updates map[string]interface{}) error {
	if m.UpdateAutomationFunc != nil {
		return m.UpdateAutomationFunc(ctx, id, updates)
	}
	return nil
}
func (m *mockAutomationRepo) DeleteAutomation(ctx context.Context, id, userID string) error {
	if m.DeleteAutomationFunc != nil {
		return m.DeleteAutomationFunc(ctx, id, userID)
	}
	return nil
}
func (m *mockAutomationRepo) GetMonitoringAutomationsForGroup(ctx context.Context, waGroupID string) ([]models.Automation, error) {
	return nil, nil
}
func (m *mockAutomationRepo) UpdateLastRunAt(ctx context.Context, id string) error {
	return nil
}
func (m *mockAutomationRepo) UpdateNextRunAt(ctx context.Context, id string, nextRun time.Time) error {
	return nil
}
func (m *mockAutomationRepo) BulkUpdateStatusByUserID(ctx context.Context, userID, targetStatus string, sourceStatus *string) (int, error) {
	return 0, nil
}
func (m *mockAutomationRepo) BulkUpdateNextRunAts(ctx context.Context, automationIDs []string, nextRunAts []time.Time) error {
	return nil
}
func (m *mockAutomationRepo) GetOfferGenerateAutomationsForGroup(ctx context.Context, waGroupID string) ([]models.Automation, error) {
	return nil, nil
}

// ---------------------------------------------------------------------------
// Mock: ActionHistoryRepository
// ---------------------------------------------------------------------------

type mockActionHistoryRepo struct {
	GetUserHistoryFunc func(ctx context.Context, userID string, limit, offset int) ([]models.ActionHistory, error)
}

func (m *mockActionHistoryRepo) Create(ctx context.Context, userID, actionType, description string, details map[string]interface{}) (*models.ActionHistory, error) {
	return nil, nil
}
func (m *mockActionHistoryRepo) Update(ctx context.Context, historyID string, updates map[string]interface{}) error {
	return nil
}
func (m *mockActionHistoryRepo) UpdateStatus(ctx context.Context, id, status, errorMessage string) error {
	return nil
}
func (m *mockActionHistoryRepo) GetByID(ctx context.Context, historyID string) (*models.ActionHistory, error) {
	return nil, nil
}
func (m *mockActionHistoryRepo) GetUserHistory(ctx context.Context, userID string, limit, offset int) ([]models.ActionHistory, error) {
	if m.GetUserHistoryFunc != nil {
		return m.GetUserHistoryFunc(ctx, userID, limit, offset)
	}
	return []models.ActionHistory{{ID: "h1", UserID: userID}}, nil
}
func (m *mockActionHistoryRepo) GetByUserID(ctx context.Context, userID string, limit, offset int) ([]models.ActionHistory, error) {
	return nil, nil
}
func (m *mockActionHistoryRepo) GetByActionType(ctx context.Context, userID, actionType string, limit int) ([]models.ActionHistory, error) {
	return nil, nil
}
func (m *mockActionHistoryRepo) GetUserHistoryFiltered(ctx context.Context, req models.HistoryListRequest, userID string) ([]models.ActionHistory, error) {
	return nil, nil
}
func (m *mockActionHistoryRepo) CountUserHistoryFiltered(ctx context.Context, req models.HistoryListRequest, userID string) (models.HistoryStatsResult, error) {
	return models.HistoryStatsResult{}, nil
}
func (m *mockActionHistoryRepo) MarkCompleted(ctx context.Context, historyID string) error {
	return nil
}
func (m *mockActionHistoryRepo) MarkFailed(ctx context.Context, historyID, errorMessage string) error {
	return nil
}
func (m *mockActionHistoryRepo) MarkRunning(ctx context.Context, historyID string) error {
	return nil
}
func (m *mockActionHistoryRepo) IncrementSuccessCount(ctx context.Context, historyID string) error {
	return nil
}
func (m *mockActionHistoryRepo) IncrementErrorCount(ctx context.Context, historyID string, errorMsg string) error {
	return nil
}
func (m *mockActionHistoryRepo) UpdateCounts(ctx context.Context, historyID string, successCount, errorCount int32) error {
	return nil
}
func (m *mockActionHistoryRepo) DeleteUserHistory(ctx context.Context, userID string, beforeDate *time.Time, actionTypes []string) (int64, error) {
	return 0, nil
}

// ---------------------------------------------------------------------------
// Mock: SettingsRepository
// ---------------------------------------------------------------------------

type mockSettingsRepo struct {
	GetAllFunc func(ctx context.Context) (models.SettingsMap, error)
	UpdateFunc func(ctx context.Context, key string, value interface{}) error
}

func (m *mockSettingsRepo) Get(ctx context.Context, keys ...string) (models.SettingsMap, error) {
	return nil, nil
}
func (m *mockSettingsRepo) GetAll(ctx context.Context) (models.SettingsMap, error) {
	if m.GetAllFunc != nil {
		return m.GetAllFunc(ctx)
	}
	return models.SettingsMap{"key": "value"}, nil
}
func (m *mockSettingsRepo) Update(ctx context.Context, key string, value interface{}) error {
	if m.UpdateFunc != nil {
		return m.UpdateFunc(ctx, key, value)
	}
	return nil
}
func (m *mockSettingsRepo) GetDispatchText(ctx context.Context, id string) (*models.DispatchText, error) {
	return nil, nil
}
func (m *mockSettingsRepo) GetDispatchTextsByUser(ctx context.Context, userID string) ([]models.DispatchText, error) {
	return nil, nil
}
func (m *mockSettingsRepo) GetActiveDispatchTextByID(ctx context.Context, id, userID string) (*models.DispatchText, error) {
	return nil, nil
}
func (m *mockSettingsRepo) GetActiveDispatchTextsByUser(ctx context.Context, userID string) ([]models.DispatchText, error) {
	return nil, nil
}
func (m *mockSettingsRepo) CreateDispatchText(ctx context.Context, userID string, dt *models.DispatchText) (*models.DispatchText, error) {
	return nil, nil
}
func (m *mockSettingsRepo) UpdateDispatchText(ctx context.Context, id string, updates map[string]interface{}) error {
	return nil
}
func (m *mockSettingsRepo) DeleteDispatchText(ctx context.Context, id, userID string) error {
	return nil
}

// ---------------------------------------------------------------------------
// Mock: CouponRepository (unused directly but required by Dependencies type)
// ---------------------------------------------------------------------------

type mockCouponRepo struct{}

func (m *mockCouponRepo) GetUserCoupons(ctx context.Context, userID string, filters map[string]interface{}) ([]models.Coupon, error) {
	return nil, nil
}
func (m *mockCouponRepo) GetCouponByID(ctx context.Context, couponID, userID string) (*models.Coupon, error) {
	return nil, nil
}
func (m *mockCouponRepo) GetCouponByCode(ctx context.Context, userID, code string) (*models.Coupon, error) {
	return nil, nil
}
func (m *mockCouponRepo) CreateCoupon(ctx context.Context, userID string, data *models.Coupon) (*models.Coupon, error) {
	return data, nil
}
func (m *mockCouponRepo) UpdateCoupon(ctx context.Context, couponID, userID string, updates map[string]interface{}) error {
	return nil
}
func (m *mockCouponRepo) DeleteCoupon(ctx context.Context, couponID, userID string) error {
	return nil
}
func (m *mockCouponRepo) GetActiveCouponsByStore(ctx context.Context, userID, store string) ([]models.Coupon, error) {
	return nil, nil
}
func (m *mockCouponRepo) GetBestCouponForProduct(ctx context.Context, userID, store string, productPrice float64, category *string) (*models.Coupon, error) {
	return nil, nil
}

// ---------------------------------------------------------------------------
// Mock: DispatchLogRepository
// ---------------------------------------------------------------------------

type mockDispatchLogRepo struct {
	CreateFunc func(ctx context.Context, log *models.DispatchLog) (*models.DispatchLog, error)
}

func (m *mockDispatchLogRepo) Create(ctx context.Context, log *models.DispatchLog) (*models.DispatchLog, error) {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, log)
	}
	log.ID = 1
	return log, nil
}
func (m *mockDispatchLogRepo) Update(ctx context.Context, logID int, updates map[string]interface{}) error {
	return nil
}
func (m *mockDispatchLogRepo) GetRecentByCanonicalKey(ctx context.Context, userID, groupID, canonicalKey string, hours int) (*models.DispatchLog, error) {
	return nil, nil
}
func (m *mockDispatchLogRepo) GetUserLogs(ctx context.Context, userID string, limit, offset int) ([]models.DispatchLog, error) {
	return nil, nil
}
func (m *mockDispatchLogRepo) DeleteRecentByAutomation(ctx context.Context, automationID string, sinceDays int) (int64, error) {
	return 0, nil
}
func (m *mockDispatchLogRepo) GetByAutomationIDWithCount(ctx context.Context, automationID, userID string, limit, offset int) ([]repository.DispatchLogWithGroup, int, error) {
	return nil, 0, nil
}
func (m *mockDispatchLogRepo) GetUserLogsWithFilters(ctx context.Context, userID string, filters *models.DispatchLogFilters, limit, offset int) ([]repository.DispatchLogWithGroup, int64, error) {
	return nil, 0, nil
}
func (m *mockDispatchLogRepo) ExistsByCanonicalKey(ctx context.Context, userID, groupID, canonicalKey string, hours int) (bool, error) {
	return false, nil
}
func (m *mockDispatchLogRepo) CreateIfNotExists(ctx context.Context, log *models.DispatchLog, dedupWindowHours int) (*models.DispatchLog, bool, error) {
	return nil, false, nil
}
func (m *mockDispatchLogRepo) DeleteOldLogsByStatus(ctx context.Context, status string, olderThan time.Duration) (int64, error) {
	return 0, nil
}

// ---------------------------------------------------------------------------
// Mock: ScheduledJobRepository
// ---------------------------------------------------------------------------

type mockScheduledJobRepo struct{}

func (m *mockScheduledJobRepo) GetPendingJobs(ctx context.Context) ([]models.ScheduledJob, error) {
	return nil, nil
}
func (m *mockScheduledJobRepo) UpdateJobStatus(ctx context.Context, jobID, status string, errorMsg *string) error {
	return nil
}
func (m *mockScheduledJobRepo) CreateJob(ctx context.Context, job *models.ScheduledJob) (*models.ScheduledJob, error) {
	return job, nil
}
func (m *mockScheduledJobRepo) GetJobByID(ctx context.Context, jobID string) (*models.ScheduledJob, error) {
	return nil, nil
}

// ---------------------------------------------------------------------------
// Mock: SystemEventRepository
// ---------------------------------------------------------------------------

type mockSystemEventRepo struct {
	GetRecentFunc func(ctx context.Context, limit int) ([]models.SystemEvent, error)
}

func (m *mockSystemEventRepo) Create(ctx context.Context, eventType string, payload map[string]interface{}) error {
	return nil
}
func (m *mockSystemEventRepo) GetRecent(ctx context.Context, limit int) ([]models.SystemEvent, error) {
	if m.GetRecentFunc != nil {
		return m.GetRecentFunc(ctx, limit)
	}
	return []models.SystemEvent{}, nil
}

// ---------------------------------------------------------------------------
// Mock: UserRoleRepository
// ---------------------------------------------------------------------------

type mockUserRoleRepo struct{}

func (m *mockUserRoleRepo) GetUserRole(ctx context.Context, userID string) (*models.UserRole, error) {
	return nil, nil
}
func (m *mockUserRoleRepo) SetUserRole(ctx context.Context, userID, role string) error {
	return nil
}
func (m *mockUserRoleRepo) RemoveUserRole(ctx context.Context, userID, role string) error {
	return nil
}
func (m *mockUserRoleRepo) IsAdmin(ctx context.Context, userID string) (bool, error) {
	return false, nil
}
func (m *mockUserRoleRepo) GetAllUserRoles(ctx context.Context, userID string) ([]string, error) {
	return nil, nil
}
func (m *mockUserRoleRepo) GetAdminUsers(ctx context.Context) ([]models.UserRole, error) {
	return nil, nil
}

// ---------------------------------------------------------------------------
// Mock: WhatsAppProvider
// ---------------------------------------------------------------------------

type mockWhatsAppProvider struct {
	CreateSessionFunc         func(ctx context.Context, input whatsapp.CreateSessionInput) (*whatsapp.SessionInfo, error)
	DeleteSessionFunc         func(ctx context.Context, name string) error
	GetSessionStatusFunc      func(ctx context.Context, name string) (*whatsapp.SessionInfo, error)
	GetQRCodeFunc             func(ctx context.Context, name string) (*whatsapp.QRCodeData, error)
	RestartSessionFunc        func(ctx context.Context, name string) error
	GetGroupsFunc             func(ctx context.Context, name string) ([]whatsapp.GroupInfo, error)
	GetGroupParticipantsFunc  func(ctx context.Context, name, groupID string) ([]whatsapp.ParticipantInfo, error)
	SendPollFunc              func(ctx context.Context, input whatsapp.SendPollInput) (*whatsapp.MessageResult, error)
	UpdateGroupSubjectFunc    func(ctx context.Context, input whatsapp.UpdateGroupSubjectInput) error
	UpdateGroupDescFunc       func(ctx context.Context, input whatsapp.UpdateGroupDescriptionInput) error
	UpdateGroupPictureFunc    func(ctx context.Context, input whatsapp.UpdateGroupPictureInput) error
	SetGroupAnnouncementFunc  func(ctx context.Context, input whatsapp.SetGroupAnnouncementInput) error
	SetWebhookFunc            func(ctx context.Context, input whatsapp.SetWebhookInput) error
}

func (m *mockWhatsAppProvider) CreateSession(ctx context.Context, input whatsapp.CreateSessionInput) (*whatsapp.SessionInfo, error) {
	if m.CreateSessionFunc != nil {
		return m.CreateSessionFunc(ctx, input)
	}
	return &whatsapp.SessionInfo{Name: input.SessionName, Status: whatsapp.SessionStatusScanQR}, nil
}
func (m *mockWhatsAppProvider) DeleteSession(ctx context.Context, name string) error {
	if m.DeleteSessionFunc != nil {
		return m.DeleteSessionFunc(ctx, name)
	}
	return nil
}
func (m *mockWhatsAppProvider) GetSessionStatus(ctx context.Context, name string) (*whatsapp.SessionInfo, error) {
	if m.GetSessionStatusFunc != nil {
		return m.GetSessionStatusFunc(ctx, name)
	}
	return &whatsapp.SessionInfo{Name: name, Status: whatsapp.SessionStatusConnected}, nil
}
func (m *mockWhatsAppProvider) GetQRCode(ctx context.Context, name string) (*whatsapp.QRCodeData, error) {
	if m.GetQRCodeFunc != nil {
		return m.GetQRCodeFunc(ctx, name)
	}
	return &whatsapp.QRCodeData{Base64Image: "base64qr"}, nil
}
func (m *mockWhatsAppProvider) RestartSession(ctx context.Context, name string) error {
	if m.RestartSessionFunc != nil {
		return m.RestartSessionFunc(ctx, name)
	}
	return nil
}
func (m *mockWhatsAppProvider) LogoutSession(ctx context.Context, name string) error {
	return nil
}
func (m *mockWhatsAppProvider) ClearStatusCache(name string) {}
func (m *mockWhatsAppProvider) GetGroups(ctx context.Context, name string) ([]whatsapp.GroupInfo, error) {
	if m.GetGroupsFunc != nil {
		return m.GetGroupsFunc(ctx, name)
	}
	return []whatsapp.GroupInfo{{ID: "g1@g.us", Subject: "Test", Size: 10}}, nil
}
func (m *mockWhatsAppProvider) GetGroupParticipants(ctx context.Context, name, groupID string) ([]whatsapp.ParticipantInfo, error) {
	if m.GetGroupParticipantsFunc != nil {
		return m.GetGroupParticipantsFunc(ctx, name, groupID)
	}
	return []whatsapp.ParticipantInfo{{ID: "5511999999999@c.us", IsAdmin: true}}, nil
}
func (m *mockWhatsAppProvider) GetGroupInviteCode(ctx context.Context, name, groupID string) (string, error) {
	return "invite-code", nil
}
func (m *mockWhatsAppProvider) UpdateGroupSubject(ctx context.Context, input whatsapp.UpdateGroupSubjectInput) error {
	if m.UpdateGroupSubjectFunc != nil {
		return m.UpdateGroupSubjectFunc(ctx, input)
	}
	return nil
}
func (m *mockWhatsAppProvider) UpdateGroupDescription(ctx context.Context, input whatsapp.UpdateGroupDescriptionInput) error {
	if m.UpdateGroupDescFunc != nil {
		return m.UpdateGroupDescFunc(ctx, input)
	}
	return nil
}
func (m *mockWhatsAppProvider) UpdateGroupPicture(ctx context.Context, input whatsapp.UpdateGroupPictureInput) error {
	if m.UpdateGroupPictureFunc != nil {
		return m.UpdateGroupPictureFunc(ctx, input)
	}
	return nil
}
func (m *mockWhatsAppProvider) SetGroupAnnouncement(ctx context.Context, input whatsapp.SetGroupAnnouncementInput) error {
	if m.SetGroupAnnouncementFunc != nil {
		return m.SetGroupAnnouncementFunc(ctx, input)
	}
	return nil
}
func (m *mockWhatsAppProvider) AddGroupMembers(ctx context.Context, input whatsapp.AddGroupMembersInput) (*whatsapp.MemberOperationResult, error) {
	return &whatsapp.MemberOperationResult{SuccessCount: len(input.PhoneNumbers)}, nil
}
func (m *mockWhatsAppProvider) RemoveGroupMembers(ctx context.Context, input whatsapp.RemoveGroupMembersInput) (*whatsapp.MemberOperationResult, error) {
	return &whatsapp.MemberOperationResult{SuccessCount: len(input.PhoneNumbers)}, nil
}
func (m *mockWhatsAppProvider) LeaveGroup(ctx context.Context, input whatsapp.LeaveGroupInput) error {
	return nil
}
func (m *mockWhatsAppProvider) PromoteMember(ctx context.Context, input whatsapp.PromoteMemberInput) error {
	return nil
}
func (m *mockWhatsAppProvider) DemoteMember(ctx context.Context, input whatsapp.DemoteMemberInput) error {
	return nil
}
func (m *mockWhatsAppProvider) SendText(ctx context.Context, input whatsapp.SendTextInput) (*whatsapp.MessageResult, error) {
	return &whatsapp.MessageResult{ID: "msg-1"}, nil
}
func (m *mockWhatsAppProvider) SendImage(ctx context.Context, input whatsapp.SendImageInput) (*whatsapp.MessageResult, error) {
	return &whatsapp.MessageResult{ID: "msg-1"}, nil
}
func (m *mockWhatsAppProvider) SendVideo(ctx context.Context, input whatsapp.SendVideoInput) (*whatsapp.MessageResult, error) {
	return &whatsapp.MessageResult{ID: "msg-1"}, nil
}
func (m *mockWhatsAppProvider) SendDocument(ctx context.Context, input whatsapp.SendDocumentInput) (*whatsapp.MessageResult, error) {
	return &whatsapp.MessageResult{ID: "msg-1"}, nil
}
func (m *mockWhatsAppProvider) SendVoice(ctx context.Context, input whatsapp.SendVoiceInput) (*whatsapp.MessageResult, error) {
	return &whatsapp.MessageResult{ID: "msg-1"}, nil
}
func (m *mockWhatsAppProvider) SendPoll(ctx context.Context, input whatsapp.SendPollInput) (*whatsapp.MessageResult, error) {
	if m.SendPollFunc != nil {
		return m.SendPollFunc(ctx, input)
	}
	return &whatsapp.MessageResult{ID: "poll-1"}, nil
}
func (m *mockWhatsAppProvider) SendLinkCustomPreview(ctx context.Context, input whatsapp.SendLinkPreviewInput) (*whatsapp.MessageResult, error) {
	return &whatsapp.MessageResult{ID: "msg-1"}, nil
}
func (m *mockWhatsAppProvider) SetWebhook(ctx context.Context, input whatsapp.SetWebhookInput) error {
	if m.SetWebhookFunc != nil {
		return m.SetWebhookFunc(ctx, input)
	}
	return nil
}
func (m *mockWhatsAppProvider) GetMessages(ctx context.Context, input whatsapp.GetMessagesInput) (*whatsapp.MessagesResult, error) {
	return &whatsapp.MessagesResult{}, nil
}
func (m *mockWhatsAppProvider) DownloadMedia(ctx context.Context, input whatsapp.DownloadMediaInput) (*whatsapp.DownloadMediaResult, error) {
	return nil, nil
}
func (m *mockWhatsAppProvider) DownloadMediaByURL(ctx context.Context, mediaURL string) (*whatsapp.DownloadMediaResult, error) {
	return nil, nil
}
func (m *mockWhatsAppProvider) RedownloadMedia(ctx context.Context, sessionName, chatID, messageID string) (*whatsapp.DownloadMediaResult, error) {
	return nil, nil
}

// ---------------------------------------------------------------------------
// Mock: Services
// ---------------------------------------------------------------------------

type mockShopeeService struct {
	SearchOffersFunc         func(ctx context.Context, userID string, params models.ShopeeSearchParams) (*models.ShopeeSearchResult, error)
	GenerateAffiliateLinkFunc func(ctx context.Context, productURL, userID string) (*models.ShopeeAffiliateLink, error)
}

func (m *mockShopeeService) SearchOffers(ctx context.Context, userID string, params models.ShopeeSearchParams) (*models.ShopeeSearchResult, error) {
	if m.SearchOffersFunc != nil {
		return m.SearchOffersFunc(ctx, userID, params)
	}
	return &models.ShopeeSearchResult{}, nil
}
func (m *mockShopeeService) GenerateAffiliateLink(ctx context.Context, productURL, userID string) (*models.ShopeeAffiliateLink, error) {
	if m.GenerateAffiliateLinkFunc != nil {
		return m.GenerateAffiliateLinkFunc(ctx, productURL, userID)
	}
	return &models.ShopeeAffiliateLink{}, nil
}
func (m *mockShopeeService) GetCategories(ctx context.Context, parentID *int64) ([]models.ShopeeCategory, error) {
	return nil, nil
}
func (m *mockShopeeService) GetProduct(ctx context.Context, itemID, shopID int64, userID string) (*models.ShopeeOffer, error) {
	return nil, nil
}
func (m *mockShopeeService) ExpandURL(ctx context.Context, shortURL string) (string, error) {
	return shortURL, nil
}
func (m *mockShopeeService) ExtractProductIDs(productURL string) (shopID, itemID int64, err error) {
	return 0, 0, nil
}

type mockMLService struct {
	CreateAffiliateLinkFunc func(ctx context.Context, originURL, userID string) (*models.MLAffiliateLink, error)
	GetProductPreviewFunc   func(ctx context.Context, originURL, userID string) (*models.MLProductPreview, error)
	SaveCredentialsFunc     func(ctx context.Context, userID, cookie, csrf, tag string) error
}

func (m *mockMLService) CreateAffiliateLink(ctx context.Context, originURL, userID string) (*models.MLAffiliateLink, error) {
	if m.CreateAffiliateLinkFunc != nil {
		return m.CreateAffiliateLinkFunc(ctx, originURL, userID)
	}
	return &models.MLAffiliateLink{}, nil
}
func (m *mockMLService) GetProductPreview(ctx context.Context, originURL, userID string) (*models.MLProductPreview, error) {
	if m.GetProductPreviewFunc != nil {
		return m.GetProductPreviewFunc(ctx, originURL, userID)
	}
	return &models.MLProductPreview{}, nil
}
func (m *mockMLService) LoadCredentials(ctx context.Context, userID string) (*models.MLCredentials, error) {
	return nil, nil
}
func (m *mockMLService) SaveCredentials(ctx context.Context, userID, cookie, csrf, tag string) error {
	if m.SaveCredentialsFunc != nil {
		return m.SaveCredentialsFunc(ctx, userID, cookie, csrf, tag)
	}
	return nil
}
func (m *mockMLService) IsConnected(ctx context.Context, userID string) (bool, error) {
	return true, nil
}
func (m *mockMLService) ExtractProductID(productURL string) (string, error) {
	return "", nil
}
func (m *mockMLService) InvalidatePreviewCache(ctx context.Context, originURL string) error {
	return nil
}

type mockMonitoringService struct {
	ProcessIncomingMessageFunc func(ctx context.Context, msg *models.IncomingMessage) error
}

func (m *mockMonitoringService) ProcessIncomingMessage(ctx context.Context, msg *models.IncomingMessage) error {
	if m.ProcessIncomingMessageFunc != nil {
		return m.ProcessIncomingMessageFunc(ctx, msg)
	}
	return nil
}

type mockJobsService struct {
	ProcessScheduledJobsFunc func(ctx context.Context) (*jobs.ProcessResult, error)
	GetQueueStatusFunc       func(ctx context.Context, userID string) (*jobs.QueueStatus, error)
}

func (m *mockJobsService) ProcessScheduledJobs(ctx context.Context) (*jobs.ProcessResult, error) {
	if m.ProcessScheduledJobsFunc != nil {
		return m.ProcessScheduledJobsFunc(ctx)
	}
	return &jobs.ProcessResult{Processed: 5, Failed: 0}, nil
}
func (m *mockJobsService) GetQueueStatus(ctx context.Context, userID string) (*jobs.QueueStatus, error) {
	if m.GetQueueStatusFunc != nil {
		return m.GetQueueStatusFunc(ctx, userID)
	}
	return &jobs.QueueStatus{Pending: 2, Running: 1}, nil
}
func (m *mockJobsService) ScheduleJob(ctx context.Context, job *models.ScheduledJob) error {
	return nil
}
func (m *mockJobsService) CancelJob(ctx context.Context, jobID string) error { return nil }
func (m *mockJobsService) CreateJob(ctx context.Context, userID string, job *models.StandaloneJob) (*models.StandaloneJob, error) {
	return nil, nil
}
func (m *mockJobsService) GetJob(ctx context.Context, id, userID string) (*models.StandaloneJob, error) {
	return nil, nil
}
func (m *mockJobsService) ListJobs(ctx context.Context, userID string) ([]models.StandaloneJob, error) {
	return nil, nil
}
func (m *mockJobsService) ListJobsPaginated(ctx context.Context, params repository.JobListParams) (*repository.JobListResult, error) {
	return nil, nil
}
func (m *mockJobsService) UpdateJob(ctx context.Context, id, userID string, updates map[string]interface{}) (*models.StandaloneJob, error) {
	return nil, nil
}
func (m *mockJobsService) DeleteJob(ctx context.Context, id, userID string) error { return nil }

type mockUserService struct {
	ResetUserPasswordFunc func(ctx context.Context, userID, adminID, adminEmail string) (*user.PasswordResetResult, error)
}

func (m *mockUserService) EnsureUserProfile(ctx context.Context, userID, email string) (*models.Profile, error) {
	return nil, nil
}
func (m *mockUserService) GetProfile(ctx context.Context, userID string) (*models.Profile, error) {
	return nil, nil
}
func (m *mockUserService) UpdateProfile(ctx context.Context, userID string, updates map[string]interface{}) error {
	return nil
}
func (m *mockUserService) CreateUserFromWebhook(ctx context.Context, data *user.WebhookUserData, plan *string) (*models.Profile, error) {
	return nil, nil
}
func (m *mockUserService) UpdateUserPlan(ctx context.Context, userID, plan string) error {
	return nil
}
func (m *mockUserService) ResetUserPassword(ctx context.Context, userID, adminID, adminEmail string) (*user.PasswordResetResult, error) {
	if m.ResetUserPasswordFunc != nil {
		return m.ResetUserPasswordFunc(ctx, userID, adminID, adminEmail)
	}
	return &user.PasswordResetResult{}, nil
}
func (m *mockUserService) SuspendUser(ctx context.Context, userID, reason string) error {
	return nil
}
func (m *mockUserService) ReactivateUser(ctx context.Context, userID string) error { return nil }

type mockCouponsService struct {
	GetUserCouponsFunc   func(ctx context.Context, userID string, filters *coupons.CouponFilters) ([]models.Coupon, error)
	GetCouponByIDFunc    func(ctx context.Context, couponID, userID string) (*models.Coupon, error)
	GetCouponsByStoreFunc func(ctx context.Context, userID, store string) ([]models.Coupon, error)
	CreateCouponFunc     func(ctx context.Context, userID string, data *coupons.CreateCouponData) (*models.Coupon, error)
	UpdateCouponFunc     func(ctx context.Context, couponID, userID string, data *coupons.UpdateCouponData) (*models.Coupon, error)
	DeleteCouponFunc     func(ctx context.Context, couponID, userID string) error
}

func (m *mockCouponsService) GetUserCoupons(ctx context.Context, userID string, filters *coupons.CouponFilters) ([]models.Coupon, error) {
	if m.GetUserCouponsFunc != nil {
		return m.GetUserCouponsFunc(ctx, userID, filters)
	}
	return []models.Coupon{{ID: "c1", Code: "TEST10"}}, nil
}
func (m *mockCouponsService) GetCouponByID(ctx context.Context, couponID, userID string) (*models.Coupon, error) {
	if m.GetCouponByIDFunc != nil {
		return m.GetCouponByIDFunc(ctx, couponID, userID)
	}
	return &models.Coupon{ID: couponID, Code: "TEST10"}, nil
}
func (m *mockCouponsService) GetCouponsByStore(ctx context.Context, userID, store string) ([]models.Coupon, error) {
	if m.GetCouponsByStoreFunc != nil {
		return m.GetCouponsByStoreFunc(ctx, userID, store)
	}
	return []models.Coupon{{ID: "c1", Code: "TEST10"}}, nil
}
func (m *mockCouponsService) CreateCoupon(ctx context.Context, userID string, data *coupons.CreateCouponData) (*models.Coupon, error) {
	if m.CreateCouponFunc != nil {
		return m.CreateCouponFunc(ctx, userID, data)
	}
	return &models.Coupon{ID: "new-c1", Code: data.Code}, nil
}
func (m *mockCouponsService) UpdateCoupon(ctx context.Context, couponID, userID string, data *coupons.UpdateCouponData) (*models.Coupon, error) {
	if m.UpdateCouponFunc != nil {
		return m.UpdateCouponFunc(ctx, couponID, userID, data)
	}
	return &models.Coupon{ID: couponID}, nil
}
func (m *mockCouponsService) DeleteCoupon(ctx context.Context, couponID, userID string) error {
	if m.DeleteCouponFunc != nil {
		return m.DeleteCouponFunc(ctx, couponID, userID)
	}
	return nil
}
func (m *mockCouponsService) GetActiveCoupons(ctx context.Context, userID string) ([]models.Coupon, error) {
	return nil, nil
}
func (m *mockCouponsService) ValidateCoupon(ctx context.Context, couponID, userID string) (*coupons.CouponValidation, error) {
	return nil, nil
}

type mockLBService struct {
	GetServerStatsFunc   func(ctx context.Context) (*loadbalancer.ServerStats, error)
	GetServerForUserFunc func(ctx context.Context, userID string) (*models.WahaServer, error)
	MigrateUserFunc      func(ctx context.Context, userID, targetServerID string) error
}

func (m *mockLBService) GetServerForUser(ctx context.Context, userID string) (*models.WahaServer, error) {
	if m.GetServerForUserFunc != nil {
		return m.GetServerForUserFunc(ctx, userID)
	}
	return &models.WahaServer{ID: "srv-1", URL: "http://waha:3000"}, nil
}
func (m *mockLBService) AssignUserToServer(ctx context.Context, userID string) (*models.WahaServer, error) {
	return nil, nil
}
func (m *mockLBService) MigrateUser(ctx context.Context, userID, targetServerID string) error {
	if m.MigrateUserFunc != nil {
		return m.MigrateUserFunc(ctx, userID, targetServerID)
	}
	return nil
}
func (m *mockLBService) RemoveUserServer(ctx context.Context, userID string) error { return nil }
func (m *mockLBService) GetServerStats(ctx context.Context) (*loadbalancer.ServerStats, error) {
	if m.GetServerStatsFunc != nil {
		return m.GetServerStatsFunc(ctx)
	}
	return &loadbalancer.ServerStats{
		TotalUsers: 5,
		Servers: []loadbalancer.ServerInfo{
			{ID: "srv-1", URL: "http://waha:3000", UserCount: 5, MaxUsers: 100, Utilization: 5.0, IsActive: true},
		},
	}, nil
}
func (m *mockLBService) GetLeastLoadedServer(ctx context.Context) (*models.WahaServer, error) {
	return nil, nil
}
func (m *mockLBService) HealthCheck(ctx context.Context) (*loadbalancer.HealthCheckResult, error) {
	return nil, nil
}

type mockRedirectService struct {
	ProcessRedirectFunc func(ctx context.Context, slug, linkID string) (*models.ProcessRedirectResult, error)
}

func (m *mockRedirectService) ProcessRedirect(ctx context.Context, slug, linkID string) (*models.ProcessRedirectResult, error) {
	if m.ProcessRedirectFunc != nil {
		return m.ProcessRedirectFunc(ctx, slug, linkID)
	}
	return &models.ProcessRedirectResult{Success: true, RedirectURL: "https://chat.whatsapp.com/XXX", GroupName: "Test", GroupID: "g1"}, nil
}
func (m *mockRedirectService) ListGroups(ctx context.Context, userID string) ([]models.Group, error) {
	return nil, nil
}
func (m *mockRedirectService) ListRedirectLinks(ctx context.Context, userID string) ([]models.RedirectLink, error) {
	return nil, nil
}
func (m *mockRedirectService) GetRedirectLink(ctx context.Context, linkID string) (*models.RedirectLink, error) {
	return nil, nil
}
func (m *mockRedirectService) CreateRedirectLink(ctx context.Context, userID string, req *models.RedirectLinkCreateRequest) (*models.RedirectLink, error) {
	return nil, nil
}
func (m *mockRedirectService) UpdateRedirectLink(ctx context.Context, linkID, userID string, req *models.RedirectLinkUpdateRequest) error {
	return nil
}
func (m *mockRedirectService) DeleteRedirectLink(ctx context.Context, linkID, userID string) error {
	return nil
}
func (m *mockRedirectService) CheckSlugAvailability(ctx context.Context, slug string, excludeID string) (bool, error) {
	return true, nil
}

type mockKiwifyService struct {
	ProcessWebhookFunc func(ctx context.Context, payload *models.KiwifyWebhookPayload, signature string) (*models.KiwifyWebhookResult, error)
}

func (m *mockKiwifyService) ProcessWebhook(ctx context.Context, payload *models.KiwifyWebhookPayload, signature string) (*models.KiwifyWebhookResult, error) {
	if m.ProcessWebhookFunc != nil {
		return m.ProcessWebhookFunc(ctx, payload, signature)
	}
	return &models.KiwifyWebhookResult{Success: true, Message: "ok"}, nil
}

// ---------------------------------------------------------------------------
// Mock: QueueClient wrapper (for handlers that use deps.QueueClient)
// ---------------------------------------------------------------------------

type mockQueueClientWrapper struct {
	EnqueueMessageFunc    func(ctx context.Context, data *workers.MessageJobData, delay time.Duration) (*asynq.TaskInfo, error)
	EnqueueGroupsSyncFunc func(ctx context.Context, data *workers.GroupsSyncJobData) (*asynq.TaskInfo, error)
	EnqueueWebhookFunc    func(ctx context.Context, data *workers.WebhookJobData) (*asynq.TaskInfo, error)
}

func (m *mockQueueClientWrapper) EnqueueMessage(ctx context.Context, data *workers.MessageJobData, delay time.Duration) (*asynq.TaskInfo, error) {
	if m.EnqueueMessageFunc != nil {
		return m.EnqueueMessageFunc(ctx, data, delay)
	}
	return &asynq.TaskInfo{ID: "task-1", Queue: "default"}, nil
}
func (m *mockQueueClientWrapper) EnqueueGroupsSync(ctx context.Context, data *workers.GroupsSyncJobData) (*asynq.TaskInfo, error) {
	if m.EnqueueGroupsSyncFunc != nil {
		return m.EnqueueGroupsSyncFunc(ctx, data)
	}
	return &asynq.TaskInfo{ID: "sync-1", Queue: "default"}, nil
}
func (m *mockQueueClientWrapper) EnqueueWebhook(ctx context.Context, data *workers.WebhookJobData) (*asynq.TaskInfo, error) {
	if m.EnqueueWebhookFunc != nil {
		return m.EnqueueWebhookFunc(ctx, data)
	}
	return &asynq.TaskInfo{ID: "wh-1", Queue: "webhooks"}, nil
}

// ---------------------------------------------------------------------------
// Helpers for reading JSON response
// ---------------------------------------------------------------------------

func mustParseBody(w *httptest.ResponseRecorder) map[string]interface{} {
	var result map[string]interface{}
	if err := parseJSON(w.Body.Bytes(), &result); err != nil {
		return nil
	}
	return result
}

func parseJSON(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

