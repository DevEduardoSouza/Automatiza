// Package workers_test provides tests for the groups sync worker.
// Using external test package to avoid mock name conflicts with existing test files.
package workers_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"unsafe"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"
	"automatiza-go/internal/workers"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// =============================================================================
// Mock Implementations
// =============================================================================

// mockGroupRepo implements repository.GroupRepository for testing.
type mockGroupRepo struct {
	mock.Mock
}

func (m *mockGroupRepo) GetUserGroups(ctx context.Context, userID string, adminOnly bool) ([]models.Group, error) {
	args := m.Called(ctx, userID, adminOnly)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Group), args.Error(1)
}

func (m *mockGroupRepo) GetUserGroupsPaginated(ctx context.Context, userID string, page, pageSize int, adminOnly bool, search string) (*repository.GroupPaginatedResult, error) {
	args := m.Called(ctx, userID, page, pageSize, adminOnly, search)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.GroupPaginatedResult), args.Error(1)
}

func (m *mockGroupRepo) GetGroupByID(ctx context.Context, groupID string) (*models.Group, error) {
	args := m.Called(ctx, groupID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Group), args.Error(1)
}

func (m *mockGroupRepo) GetGroupByWAGroupID(ctx context.Context, userID, waGroupID string) (*models.Group, error) {
	args := m.Called(ctx, userID, waGroupID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Group), args.Error(1)
}

func (m *mockGroupRepo) GetGroupsByWAGroupIDs(ctx context.Context, userID string, waGroupIDs []string) ([]models.Group, error) {
	args := m.Called(ctx, userID, waGroupIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Group), args.Error(1)
}

func (m *mockGroupRepo) GetGroupsByIDs(ctx context.Context, userID string, groupIDs []string) ([]models.Group, error) {
	args := m.Called(ctx, userID, groupIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Group), args.Error(1)
}

func (m *mockGroupRepo) GetFavoriteGroups(ctx context.Context, userID string) ([]models.Group, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Group), args.Error(1)
}

func (m *mockGroupRepo) CreateGroup(ctx context.Context, userID string, group *models.Group) (*models.Group, error) {
	args := m.Called(ctx, userID, group)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Group), args.Error(1)
}

func (m *mockGroupRepo) SyncGroups(ctx context.Context, userID string, instanceID int, groups []models.Group, options repository.SyncOptions) error {
	args := m.Called(ctx, userID, instanceID, groups, options)
	return args.Error(0)
}

func (m *mockGroupRepo) SyncGroupsWithStats(ctx context.Context, userID string, instanceID int, groups []models.Group, options repository.SyncOptions) (*repository.SyncResult, error) {
	args := m.Called(ctx, userID, instanceID, groups, options)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.SyncResult), args.Error(1)
}

func (m *mockGroupRepo) IsUserAdminInGroup(ctx context.Context, userID, waGroupID string) (bool, error) {
	args := m.Called(ctx, userID, waGroupID)
	return args.Bool(0), args.Error(1)
}

func (m *mockGroupRepo) UpdateGroup(ctx context.Context, groupID string, updates map[string]interface{}) error {
	args := m.Called(ctx, groupID, updates)
	return args.Error(0)
}

func (m *mockGroupRepo) ToggleFavorite(ctx context.Context, groupID, userID string, favorite bool) error {
	args := m.Called(ctx, groupID, userID, favorite)
	return args.Error(0)
}

func (m *mockGroupRepo) DeleteGroup(ctx context.Context, groupID, userID string) error {
	args := m.Called(ctx, groupID, userID)
	return args.Error(0)
}

func (m *mockGroupRepo) DeleteGroupsByInstance(ctx context.Context, userID string, instanceID int) error {
	args := m.Called(ctx, userID, instanceID)
	return args.Error(0)
}

func (m *mockGroupRepo) BulkUpdateGroups(ctx context.Context, userID string, groupIDs []string, updates map[string]interface{}) (int64, error) {
	args := m.Called(ctx, userID, groupIDs, updates)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockGroupRepo) CountAll(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockGroupRepo) CountGroupsByInstance(ctx context.Context, userID string, instanceID int) (int, error) {
	args := m.Called(ctx, userID, instanceID)
	return args.Int(0), args.Error(1)
}

func (m *mockGroupRepo) GetInstancesForGroups(ctx context.Context, userID string, waGroupIDs []string) (map[string]*models.Instance, error) {
	args := m.Called(ctx, userID, waGroupIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]*models.Instance), args.Error(1)
}

// mockInstanceRepoForSync implements repository.InstanceRepository for testing.
type mockInstanceRepoForSync struct {
	mock.Mock
}

func (m *mockInstanceRepoForSync) GetUserInstances(ctx context.Context, userID string) ([]models.Instance, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Instance), args.Error(1)
}

func (m *mockInstanceRepoForSync) GetInstanceByID(ctx context.Context, instanceID int) (*models.Instance, error) {
	args := m.Called(ctx, instanceID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Instance), args.Error(1)
}

func (m *mockInstanceRepoForSync) GetInstanceByName(ctx context.Context, instanceName string) (*models.Instance, error) {
	args := m.Called(ctx, instanceName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Instance), args.Error(1)
}

func (m *mockInstanceRepoForSync) GetInstanceByUserAndName(ctx context.Context, userID, instanceName string) (*models.Instance, error) {
	args := m.Called(ctx, userID, instanceName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Instance), args.Error(1)
}

func (m *mockInstanceRepoForSync) GetAllConnectedInstances(ctx context.Context) ([]models.Instance, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Instance), args.Error(1)
}

func (m *mockInstanceRepoForSync) UpsertInstance(ctx context.Context, userID string, data *models.Instance) (*models.Instance, error) {
	args := m.Called(ctx, userID, data)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Instance), args.Error(1)
}

func (m *mockInstanceRepoForSync) UpdateInstanceStatus(ctx context.Context, instanceID int, status string) error {
	args := m.Called(ctx, instanceID, status)
	return args.Error(0)
}

func (m *mockInstanceRepoForSync) DeleteInstance(ctx context.Context, instanceID int) error {
	args := m.Called(ctx, instanceID)
	return args.Error(0)
}

func (m *mockInstanceRepoForSync) SetDefaultInstance(ctx context.Context, userID string, instanceID int) error {
	args := m.Called(ctx, userID, instanceID)
	return args.Error(0)
}

func (m *mockInstanceRepoForSync) CountUserInstances(ctx context.Context, userID string) (int, error) {
	args := m.Called(ctx, userID)
	return args.Int(0), args.Error(1)
}

func (m *mockInstanceRepoForSync) UpdateInstanceLabel(ctx context.Context, userID string, instanceID int, friendlyName string) error {
	args := m.Called(ctx, userID, instanceID, friendlyName)
	return args.Error(0)
}

func (m *mockInstanceRepoForSync) UpdateInstancePhoneNumber(ctx context.Context, instanceID int, phoneNumber string) error {
	args := m.Called(ctx, instanceID, phoneNumber)
	return args.Error(0)
}

func (m *mockInstanceRepoForSync) SupersedePriorInstances(ctx context.Context, userID string, currentInstanceID int, phoneNumber string) (int, error) {
	args := m.Called(ctx, userID, currentInstanceID, phoneNumber)
	return args.Int(0), args.Error(1)
}

// mockProgressPub implements workers.JobProgressPublisher for testing.
type mockProgressPub struct {
	mock.Mock
}

func (m *mockProgressPub) PublishJobProgress(userID string, data map[string]any) {
	m.Called(userID, data)
}

// =============================================================================
// Test Helpers
// =============================================================================

// newTestLogger creates a no-op zap logger for testing.
func newTestLogger() *zap.Logger {
	return zap.NewNop()
}

// buildSyncTask creates an asynq task from GroupsSyncJobData for testing.
// It injects a ResultWriter via reflection so that task.ResultWriter().TaskID()
// does not panic (asynq.NewTask leaves the writer nil by design).
func buildSyncTask(t *testing.T, data workers.GroupsSyncJobData) *asynq.Task {
	t.Helper()
	payload, err := json.Marshal(data)
	require.NoError(t, err)

	task := asynq.NewTask(workers.TypeSyncGroups, payload)

	// asynq.Task has an unexported field `w *ResultWriter`.
	// ResultWriter has an unexported field `id string` at offset 0.
	// We allocate a ResultWriter via reflect and set its id, then assign it
	// to the task's w field so that task.ResultWriter().TaskID() works.
	rwType := reflect.TypeOf(task).Elem()         // asynq.Task
	wField, ok := rwType.FieldByName("w")         // w *ResultWriter
	require.True(t, ok, "asynq.Task should have field 'w'")

	// Allocate a new ResultWriter (zero value with id set)
	rwPtrType := wField.Type                                  // *asynq.ResultWriter
	rwElemType := rwPtrType.Elem()                            // asynq.ResultWriter
	rwValue := reflect.New(rwElemType)                        // *ResultWriter (zeroed)

	// Set the `id` field (first field of ResultWriter)
	idField := rwValue.Elem().FieldByName("id")
	// Use unsafe to bypass unexported field restriction
	idPtr := unsafe.Pointer(idField.UnsafeAddr())
	*(*string)(idPtr) = "test-task-id"

	// Set the `w` field on the task
	taskValue := reflect.ValueOf(task).Elem()
	wFieldValue := taskValue.FieldByName("w")
	wPtr := unsafe.Pointer(wFieldValue.UnsafeAddr())
	*(*unsafe.Pointer)(wPtr) = unsafe.Pointer(rwValue.Pointer())

	return task
}

// defaultJobData returns a valid GroupsSyncJobData for testing.
func defaultJobData() workers.GroupsSyncJobData {
	return workers.GroupsSyncJobData{
		UserID:       "user-123",
		InstanceName: "test-instance",
		InstanceID:   1,
	}
}

// defaultInstance returns a valid Instance for testing.
func defaultInstance() *models.Instance {
	return &models.Instance{
		ID:         1,
		UserID:     "user-123",
		InstanceID: "test-instance",
		Status:     "connected",
	}
}

// connectedSession returns a connected SessionInfo with an authenticated user.
func connectedSession() *whatsapp.SessionInfo {
	return &whatsapp.SessionInfo{
		Name:   "test-instance",
		Status: whatsapp.SessionStatusConnected,
		AuthenticatedUser: &whatsapp.AuthenticatedUser{
			ID:       "5511999999999@c.us",
			PushName: "Test User",
		},
	}
}

// sampleGroups returns a slice of GroupInfo for testing.
func sampleGroups() []whatsapp.GroupInfo {
	return []whatsapp.GroupInfo{
		{
			ID:          "group1@g.us",
			Subject:     "Group One",
			Description: "First test group",
			Owner:       "5511888888888@c.us",
			Announce:    false,
			Size:        10,
		},
		{
			ID:          "group2@g.us",
			Subject:     "Group Two",
			Description: "",
			Owner:       "5511777777777@c.us",
			Announce:    true,
			Size:        25,
		},
	}
}

// sampleParticipantsWithAdmin returns participants where the session user is admin.
func sampleParticipantsWithAdmin() []whatsapp.ParticipantInfo {
	return []whatsapp.ParticipantInfo{
		{ID: "5511888888888@c.us", PhoneNumber: "5511888888888", IsAdmin: false, IsSuperAdmin: false},
		{ID: "5511999999999@c.us", PhoneNumber: "5511999999999", IsAdmin: true, IsSuperAdmin: false},
		{ID: "5511777777777@c.us", PhoneNumber: "5511777777777", IsAdmin: false, IsSuperAdmin: false},
	}
}

// sampleParticipantsWithoutUser returns participants where the session user is absent.
func sampleParticipantsWithoutUser() []whatsapp.ParticipantInfo {
	return []whatsapp.ParticipantInfo{
		{ID: "5511888888888@c.us", PhoneNumber: "5511888888888", IsAdmin: true, IsSuperAdmin: true},
		{ID: "5511777777777@c.us", PhoneNumber: "5511777777777", IsAdmin: false, IsSuperAdmin: false},
	}
}

// lidOnlyParticipants returns participants with only LID format IDs (no resolved phone numbers).
func lidOnlyParticipants() []whatsapp.ParticipantInfo {
	return []whatsapp.ParticipantInfo{
		{ID: "56243167223861@lid", PhoneNumber: "", IsAdmin: true, IsSuperAdmin: false},
		{ID: "56243167223862@lid", PhoneNumber: "", IsAdmin: false, IsSuperAdmin: false},
	}
}

// =============================================================================
// ProcessTask Tests
// =============================================================================

func TestGroupsSyncWorker_ProcessTask_HappyPath(t *testing.T) {
	fake := whatsapp.NewFakeAdapter()
	instanceRepo := new(mockInstanceRepoForSync)
	groupRepo := new(mockGroupRepo)
	progressPub := new(mockProgressPub)

	worker := workers.NewGroupsSyncWorker(fake, instanceRepo, groupRepo, newTestLogger(), progressPub)

	data := defaultJobData()
	task := buildSyncTask(t, data)

	// Configure mocks
	instanceRepo.On("GetInstanceByUserAndName", mock.Anything, data.UserID, data.InstanceName).
		Return(defaultInstance(), nil)

	fake.SetSession(data.InstanceName, connectedSession())
	fake.SetGroups(data.InstanceName, sampleGroups())
	fake.SetParticipants(data.InstanceName, "group1@g.us", sampleParticipantsWithAdmin())
	fake.SetParticipants(data.InstanceName, "group2@g.us", sampleParticipantsWithoutUser())

	groupRepo.On("SyncGroupsWithStats", mock.Anything, data.UserID, 1, mock.AnythingOfType("[]models.Group"), repository.SyncOptions{BatchSize: 50}).
		Return(&repository.SyncResult{Created: 1, Updated: 1, Deleted: 0}, nil)

	// Progress publisher should be called for each group + final event
	progressPub.On("PublishJobProgress", data.UserID, mock.AnythingOfType("map[string]interface {}")).Return()

	err := worker.ProcessTask(context.Background(), task)

	assert.NoError(t, err)
	instanceRepo.AssertExpectations(t)
	groupRepo.AssertExpectations(t)

	// Verify SyncGroupsWithStats was called with correct groups
	syncCall := groupRepo.Calls[0]
	groups := syncCall.Arguments[3].([]models.Group)
	assert.Len(t, groups, 2)
	assert.Equal(t, "group1@g.us", groups[0].WAGroupID)
	assert.Equal(t, "Group One", groups[0].Name)
	assert.True(t, groups[0].IsAdmin, "user should be admin in group1 (matched via participants)")
	assert.Equal(t, "group2@g.us", groups[1].WAGroupID)
	assert.False(t, groups[1].IsAdmin, "user should not be admin in group2")

	// Verify progress was published (2 intermediate + 1 final = 3 calls)
	assert.Equal(t, 3, len(progressPub.Calls))
}

func TestGroupsSyncWorker_ProcessTask_InvalidPayload(t *testing.T) {
	fake := whatsapp.NewFakeAdapter()
	instanceRepo := new(mockInstanceRepoForSync)
	groupRepo := new(mockGroupRepo)

	worker := workers.NewGroupsSyncWorker(fake, instanceRepo, groupRepo, newTestLogger(), nil)

	task := asynq.NewTask(workers.TypeSyncGroups, []byte("invalid json"))

	err := worker.ProcessTask(context.Background(), task)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal job data")
}

func TestGroupsSyncWorker_ProcessTask_InstanceNotFound(t *testing.T) {
	fake := whatsapp.NewFakeAdapter()
	instanceRepo := new(mockInstanceRepoForSync)
	groupRepo := new(mockGroupRepo)

	worker := workers.NewGroupsSyncWorker(fake, instanceRepo, groupRepo, newTestLogger(), nil)

	data := defaultJobData()
	task := buildSyncTask(t, data)

	instanceRepo.On("GetInstanceByUserAndName", mock.Anything, data.UserID, data.InstanceName).
		Return(nil, nil)

	err := worker.ProcessTask(context.Background(), task)

	// Should return nil (don't retry) when instance not found
	assert.NoError(t, err)
	instanceRepo.AssertExpectations(t)
}

func TestGroupsSyncWorker_ProcessTask_InstanceRepoError(t *testing.T) {
	fake := whatsapp.NewFakeAdapter()
	instanceRepo := new(mockInstanceRepoForSync)
	groupRepo := new(mockGroupRepo)

	worker := workers.NewGroupsSyncWorker(fake, instanceRepo, groupRepo, newTestLogger(), nil)

	data := defaultJobData()
	task := buildSyncTask(t, data)

	instanceRepo.On("GetInstanceByUserAndName", mock.Anything, data.UserID, data.InstanceName).
		Return(nil, errors.New("database connection error"))

	err := worker.ProcessTask(context.Background(), task)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get instance")
	assert.Contains(t, err.Error(), "database connection error")
	instanceRepo.AssertExpectations(t)
}

func TestGroupsSyncWorker_ProcessTask_SessionNotFound(t *testing.T) {
	fake := whatsapp.NewFakeAdapter()
	instanceRepo := new(mockInstanceRepoForSync)
	groupRepo := new(mockGroupRepo)

	worker := workers.NewGroupsSyncWorker(fake, instanceRepo, groupRepo, newTestLogger(), nil)

	data := defaultJobData()
	task := buildSyncTask(t, data)

	instanceRepo.On("GetInstanceByUserAndName", mock.Anything, data.UserID, data.InstanceName).
		Return(defaultInstance(), nil)

	// FakeAdapter default: if session not set, returns ErrCodeSessionNotFound
	// (no session configured in fake = session not found)

	err := worker.ProcessTask(context.Background(), task)

	// Should return nil (don't retry) when session not found
	assert.NoError(t, err)
	instanceRepo.AssertExpectations(t)
}

func TestGroupsSyncWorker_ProcessTask_ProviderTemporarilyUnavailable(t *testing.T) {
	fake := whatsapp.NewFakeAdapter()
	instanceRepo := new(mockInstanceRepoForSync)
	groupRepo := new(mockGroupRepo)

	worker := workers.NewGroupsSyncWorker(fake, instanceRepo, groupRepo, newTestLogger(), nil)

	data := defaultJobData()
	task := buildSyncTask(t, data)

	instanceRepo.On("GetInstanceByUserAndName", mock.Anything, data.UserID, data.InstanceName).
		Return(defaultInstance(), nil)

	// Override GetSessionStatus to return provider error
	fake.GetSessionStatusFunc = func(_ context.Context, _ string) (*whatsapp.SessionInfo, error) {
		return nil, whatsapp.NewProviderError(whatsapp.ErrCodeProviderError, "provider unavailable", nil)
	}

	err := worker.ProcessTask(context.Background(), task)

	// Should return nil (don't retry) when provider temporarily unavailable
	assert.NoError(t, err)
	instanceRepo.AssertExpectations(t)
}

func TestGroupsSyncWorker_ProcessTask_SessionNotConnected(t *testing.T) {
	fake := whatsapp.NewFakeAdapter()
	instanceRepo := new(mockInstanceRepoForSync)
	groupRepo := new(mockGroupRepo)

	worker := workers.NewGroupsSyncWorker(fake, instanceRepo, groupRepo, newTestLogger(), nil)

	data := defaultJobData()
	task := buildSyncTask(t, data)

	instanceRepo.On("GetInstanceByUserAndName", mock.Anything, data.UserID, data.InstanceName).
		Return(defaultInstance(), nil)

	// Set session as disconnected
	fake.SetSession(data.InstanceName, &whatsapp.SessionInfo{
		Name:   data.InstanceName,
		Status: whatsapp.SessionStatusDisconnected,
	})

	err := worker.ProcessTask(context.Background(), task)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "instance not connected")
	instanceRepo.AssertExpectations(t)
}

func TestGroupsSyncWorker_ProcessTask_FetchGroupsError(t *testing.T) {
	fake := whatsapp.NewFakeAdapter()
	instanceRepo := new(mockInstanceRepoForSync)
	groupRepo := new(mockGroupRepo)

	worker := workers.NewGroupsSyncWorker(fake, instanceRepo, groupRepo, newTestLogger(), nil)

	data := defaultJobData()
	task := buildSyncTask(t, data)

	instanceRepo.On("GetInstanceByUserAndName", mock.Anything, data.UserID, data.InstanceName).
		Return(defaultInstance(), nil)

	fake.SetSession(data.InstanceName, connectedSession())

	// Override GetGroups to return error
	fake.GetGroupsFunc = func(_ context.Context, _ string) ([]whatsapp.GroupInfo, error) {
		return nil, errors.New("failed to fetch groups from provider")
	}

	err := worker.ProcessTask(context.Background(), task)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch groups")
	instanceRepo.AssertExpectations(t)
}

func TestGroupsSyncWorker_ProcessTask_AdminDetectionViaParticipants(t *testing.T) {
	fake := whatsapp.NewFakeAdapter()
	instanceRepo := new(mockInstanceRepoForSync)
	groupRepo := new(mockGroupRepo)
	progressPub := new(mockProgressPub)

	worker := workers.NewGroupsSyncWorker(fake, instanceRepo, groupRepo, newTestLogger(), progressPub)

	data := defaultJobData()
	task := buildSyncTask(t, data)

	instanceRepo.On("GetInstanceByUserAndName", mock.Anything, data.UserID, data.InstanceName).
		Return(defaultInstance(), nil)

	groups := []whatsapp.GroupInfo{
		{
			ID:      "admin-group@g.us",
			Subject: "Admin Group",
			Size:    5,
		},
	}
	fake.SetSession(data.InstanceName, connectedSession())
	fake.SetGroups(data.InstanceName, groups)

	// Session user (5511999999999) is admin in this group
	fake.SetParticipants(data.InstanceName, "admin-group@g.us", []whatsapp.ParticipantInfo{
		{ID: "5511999999999@c.us", PhoneNumber: "5511999999999", IsAdmin: true, IsSuperAdmin: false},
		{ID: "5511888888888@c.us", PhoneNumber: "5511888888888", IsAdmin: false, IsSuperAdmin: false},
	})

	groupRepo.On("SyncGroupsWithStats", mock.Anything, data.UserID, 1, mock.AnythingOfType("[]models.Group"), mock.Anything).
		Return(&repository.SyncResult{Created: 1}, nil)
	progressPub.On("PublishJobProgress", data.UserID, mock.AnythingOfType("map[string]interface {}")).Return()

	err := worker.ProcessTask(context.Background(), task)

	assert.NoError(t, err)

	syncCall := groupRepo.Calls[0]
	syncedGroups := syncCall.Arguments[3].([]models.Group)
	assert.Len(t, syncedGroups, 1)
	assert.True(t, syncedGroups[0].IsAdmin, "user should be detected as admin via participants")
}

func TestGroupsSyncWorker_ProcessTask_AdminDetectionViaSuperAdmin(t *testing.T) {
	fake := whatsapp.NewFakeAdapter()
	instanceRepo := new(mockInstanceRepoForSync)
	groupRepo := new(mockGroupRepo)
	progressPub := new(mockProgressPub)

	worker := workers.NewGroupsSyncWorker(fake, instanceRepo, groupRepo, newTestLogger(), progressPub)

	data := defaultJobData()
	task := buildSyncTask(t, data)

	instanceRepo.On("GetInstanceByUserAndName", mock.Anything, data.UserID, data.InstanceName).
		Return(defaultInstance(), nil)

	groups := []whatsapp.GroupInfo{
		{
			ID:      "superadmin-group@g.us",
			Subject: "SuperAdmin Group",
			Size:    3,
		},
	}
	fake.SetSession(data.InstanceName, connectedSession())
	fake.SetGroups(data.InstanceName, groups)

	// Session user is super admin
	fake.SetParticipants(data.InstanceName, "superadmin-group@g.us", []whatsapp.ParticipantInfo{
		{ID: "5511999999999@c.us", PhoneNumber: "5511999999999", IsAdmin: false, IsSuperAdmin: true},
	})

	groupRepo.On("SyncGroupsWithStats", mock.Anything, data.UserID, 1, mock.AnythingOfType("[]models.Group"), mock.Anything).
		Return(&repository.SyncResult{Created: 1}, nil)
	progressPub.On("PublishJobProgress", data.UserID, mock.AnythingOfType("map[string]interface {}")).Return()

	err := worker.ProcessTask(context.Background(), task)

	assert.NoError(t, err)
	syncedGroups := groupRepo.Calls[0].Arguments[3].([]models.Group)
	assert.True(t, syncedGroups[0].IsAdmin, "user should be detected as admin via IsSuperAdmin")
}

func TestGroupsSyncWorker_ProcessTask_OwnerFallback(t *testing.T) {
	fake := whatsapp.NewFakeAdapter()
	instanceRepo := new(mockInstanceRepoForSync)
	groupRepo := new(mockGroupRepo)
	progressPub := new(mockProgressPub)

	worker := workers.NewGroupsSyncWorker(fake, instanceRepo, groupRepo, newTestLogger(), progressPub)

	data := defaultJobData()
	task := buildSyncTask(t, data)

	instanceRepo.On("GetInstanceByUserAndName", mock.Anything, data.UserID, data.InstanceName).
		Return(defaultInstance(), nil)

	// The session user (5511999999999) is the group owner but NOT in participants
	groups := []whatsapp.GroupInfo{
		{
			ID:      "owned-group@g.us",
			Subject: "Owned Group",
			Owner:   "5511999999999@c.us",
			Size:    5,
		},
	}
	fake.SetSession(data.InstanceName, connectedSession())
	fake.SetGroups(data.InstanceName, groups)

	// Participants do NOT include the session user
	fake.SetParticipants(data.InstanceName, "owned-group@g.us", sampleParticipantsWithoutUser())

	groupRepo.On("SyncGroupsWithStats", mock.Anything, data.UserID, 1, mock.AnythingOfType("[]models.Group"), mock.Anything).
		Return(&repository.SyncResult{Created: 1}, nil)
	progressPub.On("PublishJobProgress", data.UserID, mock.AnythingOfType("map[string]interface {}")).Return()

	err := worker.ProcessTask(context.Background(), task)

	assert.NoError(t, err)
	syncedGroups := groupRepo.Calls[0].Arguments[3].([]models.Group)
	assert.True(t, syncedGroups[0].IsAdmin, "user should be detected as admin via owner fallback")
}

func TestGroupsSyncWorker_ProcessTask_NilAuthenticatedUser(t *testing.T) {
	fake := whatsapp.NewFakeAdapter()
	instanceRepo := new(mockInstanceRepoForSync)
	groupRepo := new(mockGroupRepo)
	progressPub := new(mockProgressPub)

	worker := workers.NewGroupsSyncWorker(fake, instanceRepo, groupRepo, newTestLogger(), progressPub)

	data := defaultJobData()
	task := buildSyncTask(t, data)

	instanceRepo.On("GetInstanceByUserAndName", mock.Anything, data.UserID, data.InstanceName).
		Return(defaultInstance(), nil)

	// Session connected but AuthenticatedUser is nil
	fake.SetSession(data.InstanceName, &whatsapp.SessionInfo{
		Name:              data.InstanceName,
		Status:            whatsapp.SessionStatusConnected,
		AuthenticatedUser: nil,
	})
	fake.SetGroups(data.InstanceName, []whatsapp.GroupInfo{
		{ID: "group@g.us", Subject: "Test Group", Size: 5},
	})
	fake.SetParticipants(data.InstanceName, "group@g.us", sampleParticipantsWithAdmin())

	groupRepo.On("SyncGroupsWithStats", mock.Anything, data.UserID, 1, mock.AnythingOfType("[]models.Group"), mock.Anything).
		Return(&repository.SyncResult{Created: 1}, nil)
	progressPub.On("PublishJobProgress", data.UserID, mock.AnythingOfType("map[string]interface {}")).Return()

	err := worker.ProcessTask(context.Background(), task)

	assert.NoError(t, err)

	// Admin detection should be skipped when AuthenticatedUser is nil
	syncedGroups := groupRepo.Calls[0].Arguments[3].([]models.Group)
	assert.False(t, syncedGroups[0].IsAdmin, "admin should not be detected when AuthenticatedUser is nil")
}

func TestGroupsSyncWorker_ProcessTask_ContextCancellation(t *testing.T) {
	fake := whatsapp.NewFakeAdapter()
	instanceRepo := new(mockInstanceRepoForSync)
	groupRepo := new(mockGroupRepo)

	worker := workers.NewGroupsSyncWorker(fake, instanceRepo, groupRepo, newTestLogger(), nil)

	data := defaultJobData()
	task := buildSyncTask(t, data)

	instanceRepo.On("GetInstanceByUserAndName", mock.Anything, data.UserID, data.InstanceName).
		Return(defaultInstance(), nil)

	// Create many groups so the loop has something to iterate
	manyGroups := make([]whatsapp.GroupInfo, 10)
	for i := range manyGroups {
		manyGroups[i] = whatsapp.GroupInfo{
			ID:      "group" + string(rune('A'+i)) + "@g.us",
			Subject: "Group " + string(rune('A'+i)),
			Size:    5,
		}
	}

	fake.SetSession(data.InstanceName, connectedSession())
	fake.SetGroups(data.InstanceName, manyGroups)

	// Cancel context immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := worker.ProcessTask(ctx, task)

	assert.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled), "error should be context.Canceled")
}

func TestGroupsSyncWorker_ProcessTask_InvalidInstanceID(t *testing.T) {
	fake := whatsapp.NewFakeAdapter()
	instanceRepo := new(mockInstanceRepoForSync)
	groupRepo := new(mockGroupRepo)

	worker := workers.NewGroupsSyncWorker(fake, instanceRepo, groupRepo, newTestLogger(), nil)

	data := defaultJobData()
	task := buildSyncTask(t, data)

	// Return instance with invalid ID (0 or negative)
	invalidInstance := &models.Instance{
		ID:         0,
		UserID:     "user-123",
		InstanceID: "test-instance",
		Status:     "connected",
	}

	instanceRepo.On("GetInstanceByUserAndName", mock.Anything, data.UserID, data.InstanceName).
		Return(invalidInstance, nil)

	fake.SetSession(data.InstanceName, connectedSession())
	fake.SetGroups(data.InstanceName, sampleGroups())
	fake.SetParticipants(data.InstanceName, "group1@g.us", sampleParticipantsWithAdmin())
	fake.SetParticipants(data.InstanceName, "group2@g.us", sampleParticipantsWithoutUser())

	err := worker.ProcessTask(context.Background(), task)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid instance ID format")
}

func TestGroupsSyncWorker_ProcessTask_SyncError(t *testing.T) {
	fake := whatsapp.NewFakeAdapter()
	instanceRepo := new(mockInstanceRepoForSync)
	groupRepo := new(mockGroupRepo)
	progressPub := new(mockProgressPub)

	worker := workers.NewGroupsSyncWorker(fake, instanceRepo, groupRepo, newTestLogger(), progressPub)

	data := defaultJobData()
	task := buildSyncTask(t, data)

	instanceRepo.On("GetInstanceByUserAndName", mock.Anything, data.UserID, data.InstanceName).
		Return(defaultInstance(), nil)

	fake.SetSession(data.InstanceName, connectedSession())
	fake.SetGroups(data.InstanceName, sampleGroups())
	fake.SetParticipants(data.InstanceName, "group1@g.us", sampleParticipantsWithAdmin())
	fake.SetParticipants(data.InstanceName, "group2@g.us", sampleParticipantsWithoutUser())

	groupRepo.On("SyncGroupsWithStats", mock.Anything, data.UserID, 1, mock.AnythingOfType("[]models.Group"), mock.Anything).
		Return(nil, errors.New("database constraint violation"))

	progressPub.On("PublishJobProgress", data.UserID, mock.AnythingOfType("map[string]interface {}")).Return()

	err := worker.ProcessTask(context.Background(), task)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to sync groups to database")
	assert.Contains(t, err.Error(), "database constraint violation")
}

func TestGroupsSyncWorker_ProcessTask_ProgressPublishing(t *testing.T) {
	fake := whatsapp.NewFakeAdapter()
	instanceRepo := new(mockInstanceRepoForSync)
	groupRepo := new(mockGroupRepo)
	progressPub := new(mockProgressPub)

	worker := workers.NewGroupsSyncWorker(fake, instanceRepo, groupRepo, newTestLogger(), progressPub)

	data := defaultJobData()
	task := buildSyncTask(t, data)

	instanceRepo.On("GetInstanceByUserAndName", mock.Anything, data.UserID, data.InstanceName).
		Return(defaultInstance(), nil)

	groups := []whatsapp.GroupInfo{
		{ID: "g1@g.us", Subject: "G1", Size: 3},
		{ID: "g2@g.us", Subject: "G2", Size: 5},
		{ID: "g3@g.us", Subject: "G3", Size: 7},
	}
	fake.SetSession(data.InstanceName, connectedSession())
	fake.SetGroups(data.InstanceName, groups)

	groupRepo.On("SyncGroupsWithStats", mock.Anything, data.UserID, 1, mock.AnythingOfType("[]models.Group"), mock.Anything).
		Return(&repository.SyncResult{Created: 3}, nil)

	progressPub.On("PublishJobProgress", data.UserID, mock.AnythingOfType("map[string]interface {}")).Return()

	err := worker.ProcessTask(context.Background(), task)

	assert.NoError(t, err)

	// 3 intermediate progress events + 1 final completion event = 4 calls
	assert.Equal(t, 4, len(progressPub.Calls))

	// Verify intermediate progress calls
	for i := 0; i < 3; i++ {
		callData := progressPub.Calls[i].Arguments[1].(map[string]any)
		assert.Equal(t, "groups_sync", callData["jobType"])
		assert.Equal(t, i+1, callData["current"])
		assert.Equal(t, 3, callData["total"])

		// Verify group info is included
		groupData := callData["group"].(map[string]any)
		assert.NotEmpty(t, groupData["id"])
		assert.NotEmpty(t, groupData["name"])
	}

	// Verify final completion event
	finalData := progressPub.Calls[3].Arguments[1].(map[string]any)
	assert.Equal(t, "completed", finalData["status"])
	assert.Equal(t, 100, finalData["progress"])
	assert.Equal(t, 3, finalData["newGroups"])
}

func TestGroupsSyncWorker_ProcessTask_NilProgressPublisher(t *testing.T) {
	fake := whatsapp.NewFakeAdapter()
	instanceRepo := new(mockInstanceRepoForSync)
	groupRepo := new(mockGroupRepo)

	// Pass nil for progressPub
	worker := workers.NewGroupsSyncWorker(fake, instanceRepo, groupRepo, newTestLogger(), nil)

	data := defaultJobData()
	task := buildSyncTask(t, data)

	instanceRepo.On("GetInstanceByUserAndName", mock.Anything, data.UserID, data.InstanceName).
		Return(defaultInstance(), nil)

	fake.SetSession(data.InstanceName, connectedSession())
	fake.SetGroups(data.InstanceName, sampleGroups())
	fake.SetParticipants(data.InstanceName, "group1@g.us", sampleParticipantsWithAdmin())
	fake.SetParticipants(data.InstanceName, "group2@g.us", sampleParticipantsWithoutUser())

	groupRepo.On("SyncGroupsWithStats", mock.Anything, data.UserID, 1, mock.AnythingOfType("[]models.Group"), mock.Anything).
		Return(&repository.SyncResult{Created: 2}, nil)

	err := worker.ProcessTask(context.Background(), task)

	// Should work without progress publisher (no panic)
	assert.NoError(t, err)
	groupRepo.AssertExpectations(t)
}

func TestGroupsSyncWorker_ProcessTaskWrapper(t *testing.T) {
	fake := whatsapp.NewFakeAdapter()
	instanceRepo := new(mockInstanceRepoForSync)
	groupRepo := new(mockGroupRepo)

	worker := workers.NewGroupsSyncWorker(fake, instanceRepo, groupRepo, newTestLogger(), nil)

	handler := worker.ProcessTaskWrapper()

	// handler should be a valid asynq.HandlerFunc
	assert.NotNil(t, handler)

	// Test that it delegates to ProcessTask by passing invalid payload
	task := asynq.NewTask(workers.TypeSyncGroups, []byte("bad json"))
	err := handler(context.Background(), task)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal job data")
}

func TestGroupsSyncWorker_ProcessTask_ZeroAdminDetectionWarning(t *testing.T) {
	fake := whatsapp.NewFakeAdapter()
	instanceRepo := new(mockInstanceRepoForSync)
	groupRepo := new(mockGroupRepo)
	progressPub := new(mockProgressPub)

	worker := workers.NewGroupsSyncWorker(fake, instanceRepo, groupRepo, newTestLogger(), progressPub)

	data := defaultJobData()
	task := buildSyncTask(t, data)

	instanceRepo.On("GetInstanceByUserAndName", mock.Anything, data.UserID, data.InstanceName).
		Return(defaultInstance(), nil)

	// Groups where the session user is NOT admin in any of them
	groups := []whatsapp.GroupInfo{
		{ID: "g1@g.us", Subject: "G1", Size: 5},
		{ID: "g2@g.us", Subject: "G2", Size: 5},
	}
	fake.SetSession(data.InstanceName, connectedSession())
	fake.SetGroups(data.InstanceName, groups)

	// User not found in participants, no owner match
	fake.SetParticipants(data.InstanceName, "g1@g.us", sampleParticipantsWithoutUser())
	fake.SetParticipants(data.InstanceName, "g2@g.us", sampleParticipantsWithoutUser())

	groupRepo.On("SyncGroupsWithStats", mock.Anything, data.UserID, 1, mock.AnythingOfType("[]models.Group"), mock.Anything).
		Return(&repository.SyncResult{Created: 2}, nil)
	progressPub.On("PublishJobProgress", data.UserID, mock.AnythingOfType("map[string]interface {}")).Return()

	err := worker.ProcessTask(context.Background(), task)

	// Should succeed (the zero admin warning is logged + Sentry captured but doesn't fail)
	assert.NoError(t, err)

	// Verify no groups marked as admin
	syncedGroups := groupRepo.Calls[0].Arguments[3].([]models.Group)
	for _, g := range syncedGroups {
		assert.False(t, g.IsAdmin, "no group should be marked as admin")
	}
}

func TestGroupsSyncWorker_ProcessTask_LIDOnlyParticipants(t *testing.T) {
	fake := whatsapp.NewFakeAdapter()
	instanceRepo := new(mockInstanceRepoForSync)
	groupRepo := new(mockGroupRepo)
	progressPub := new(mockProgressPub)

	worker := workers.NewGroupsSyncWorker(fake, instanceRepo, groupRepo, newTestLogger(), progressPub)

	data := defaultJobData()
	task := buildSyncTask(t, data)

	instanceRepo.On("GetInstanceByUserAndName", mock.Anything, data.UserID, data.InstanceName).
		Return(defaultInstance(), nil)

	groups := []whatsapp.GroupInfo{
		{ID: "lid-group@g.us", Subject: "LID Group", Size: 5},
	}
	fake.SetSession(data.InstanceName, connectedSession())
	fake.SetGroups(data.InstanceName, groups)

	// All participants have LID-only IDs (phone number not resolved)
	fake.SetParticipants(data.InstanceName, "lid-group@g.us", lidOnlyParticipants())

	groupRepo.On("SyncGroupsWithStats", mock.Anything, data.UserID, 1, mock.AnythingOfType("[]models.Group"), mock.Anything).
		Return(&repository.SyncResult{Created: 1}, nil)
	progressPub.On("PublishJobProgress", data.UserID, mock.AnythingOfType("map[string]interface {}")).Return()

	err := worker.ProcessTask(context.Background(), task)

	assert.NoError(t, err)

	// With LID-only participants, ExtractPhoneFromID will extract the LID number
	// which won't match the session phone (5511999999999), so admin should be false
	syncedGroups := groupRepo.Calls[0].Arguments[3].([]models.Group)
	assert.False(t, syncedGroups[0].IsAdmin, "LID-only participants should not match session phone")
}

func TestGroupsSyncWorker_ProcessTask_EmptyGroupsList(t *testing.T) {
	fake := whatsapp.NewFakeAdapter()
	instanceRepo := new(mockInstanceRepoForSync)
	groupRepo := new(mockGroupRepo)

	worker := workers.NewGroupsSyncWorker(fake, instanceRepo, groupRepo, newTestLogger(), nil)

	data := defaultJobData()
	task := buildSyncTask(t, data)

	instanceRepo.On("GetInstanceByUserAndName", mock.Anything, data.UserID, data.InstanceName).
		Return(defaultInstance(), nil)

	fake.SetSession(data.InstanceName, connectedSession())
	// No groups set -> will return empty slice

	groupRepo.On("SyncGroupsWithStats", mock.Anything, data.UserID, 1, mock.AnythingOfType("[]models.Group"), mock.Anything).
		Return(&repository.SyncResult{}, nil)

	err := worker.ProcessTask(context.Background(), task)

	assert.NoError(t, err)

	// SyncGroupsWithStats should be called with empty groups
	syncCall := groupRepo.Calls[0]
	groups := syncCall.Arguments[3].([]models.Group)
	assert.Len(t, groups, 0)
}

func TestGroupsSyncWorker_ProcessTask_GroupDescriptionHandling(t *testing.T) {
	fake := whatsapp.NewFakeAdapter()
	instanceRepo := new(mockInstanceRepoForSync)
	groupRepo := new(mockGroupRepo)

	worker := workers.NewGroupsSyncWorker(fake, instanceRepo, groupRepo, newTestLogger(), nil)

	data := defaultJobData()
	task := buildSyncTask(t, data)

	instanceRepo.On("GetInstanceByUserAndName", mock.Anything, data.UserID, data.InstanceName).
		Return(defaultInstance(), nil)

	groups := []whatsapp.GroupInfo{
		{ID: "desc@g.us", Subject: "With Desc", Description: "Has description", Size: 3},
		{ID: "nodesc@g.us", Subject: "No Desc", Description: "", Size: 2},
	}
	fake.SetSession(data.InstanceName, connectedSession())
	fake.SetGroups(data.InstanceName, groups)

	groupRepo.On("SyncGroupsWithStats", mock.Anything, data.UserID, 1, mock.AnythingOfType("[]models.Group"), mock.Anything).
		Return(&repository.SyncResult{Created: 2}, nil)

	err := worker.ProcessTask(context.Background(), task)

	assert.NoError(t, err)

	syncedGroups := groupRepo.Calls[0].Arguments[3].([]models.Group)
	assert.Len(t, syncedGroups, 2)

	// Group with description: Description should be non-nil
	assert.NotNil(t, syncedGroups[0].Description)
	assert.Equal(t, "Has description", *syncedGroups[0].Description)

	// Group without description: Description should be nil
	assert.Nil(t, syncedGroups[1].Description)
}

func TestGroupsSyncWorker_ProcessTask_GroupFieldMapping(t *testing.T) {
	fake := whatsapp.NewFakeAdapter()
	instanceRepo := new(mockInstanceRepoForSync)
	groupRepo := new(mockGroupRepo)

	worker := workers.NewGroupsSyncWorker(fake, instanceRepo, groupRepo, newTestLogger(), nil)

	data := defaultJobData()
	task := buildSyncTask(t, data)

	instanceRepo.On("GetInstanceByUserAndName", mock.Anything, data.UserID, data.InstanceName).
		Return(defaultInstance(), nil)

	groups := []whatsapp.GroupInfo{
		{
			ID:       "map-test@g.us",
			Subject:  "Mapped Group",
			Announce: true,
			Size:     42,
		},
	}
	fake.SetSession(data.InstanceName, connectedSession())
	fake.SetGroups(data.InstanceName, groups)

	groupRepo.On("SyncGroupsWithStats", mock.Anything, data.UserID, 1, mock.AnythingOfType("[]models.Group"), mock.Anything).
		Return(&repository.SyncResult{Created: 1}, nil)

	err := worker.ProcessTask(context.Background(), task)

	assert.NoError(t, err)

	syncedGroups := groupRepo.Calls[0].Arguments[3].([]models.Group)
	require.Len(t, syncedGroups, 1)

	g := syncedGroups[0]
	assert.Equal(t, "map-test@g.us", g.WAGroupID, "WAGroupID should map from GroupInfo.ID")
	assert.Equal(t, "Mapped Group", g.Name, "Name should map from GroupInfo.Subject")
	assert.Equal(t, "1", g.InstanceID, "InstanceID should be string representation of Instance.ID")
	assert.Equal(t, "user-123", g.UserID, "UserID should come from job data")
	assert.True(t, g.AdminOnly, "AdminOnly should map from GroupInfo.Announce")
	assert.Equal(t, 42, g.MembersCount, "MembersCount should map from GroupInfo.Size")
}

func TestGroupsSyncWorker_ProcessTask_GetSessionStatusUnknownError(t *testing.T) {
	fake := whatsapp.NewFakeAdapter()
	instanceRepo := new(mockInstanceRepoForSync)
	groupRepo := new(mockGroupRepo)

	worker := workers.NewGroupsSyncWorker(fake, instanceRepo, groupRepo, newTestLogger(), nil)

	data := defaultJobData()
	task := buildSyncTask(t, data)

	instanceRepo.On("GetInstanceByUserAndName", mock.Anything, data.UserID, data.InstanceName).
		Return(defaultInstance(), nil)

	// Override to return a generic (non-ProviderError) error
	fake.GetSessionStatusFunc = func(_ context.Context, _ string) (*whatsapp.SessionInfo, error) {
		return nil, errors.New("unexpected network failure")
	}

	err := worker.ProcessTask(context.Background(), task)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to check status")
	assert.Contains(t, err.Error(), "unexpected network failure")
}

func TestGroupsSyncWorker_ProcessTask_GetParticipantsError(t *testing.T) {
	fake := whatsapp.NewFakeAdapter()
	instanceRepo := new(mockInstanceRepoForSync)
	groupRepo := new(mockGroupRepo)
	progressPub := new(mockProgressPub)

	worker := workers.NewGroupsSyncWorker(fake, instanceRepo, groupRepo, newTestLogger(), progressPub)

	data := defaultJobData()
	task := buildSyncTask(t, data)

	instanceRepo.On("GetInstanceByUserAndName", mock.Anything, data.UserID, data.InstanceName).
		Return(defaultInstance(), nil)

	groups := []whatsapp.GroupInfo{
		{ID: "g1@g.us", Subject: "G1", Size: 5},
	}
	fake.SetSession(data.InstanceName, connectedSession())
	fake.SetGroups(data.InstanceName, groups)

	// Override GetGroupParticipants to return error
	fake.GetGroupParticipantsFunc = func(_ context.Context, _, _ string) ([]whatsapp.ParticipantInfo, error) {
		return nil, errors.New("participants fetch failed")
	}

	groupRepo.On("SyncGroupsWithStats", mock.Anything, data.UserID, 1, mock.AnythingOfType("[]models.Group"), mock.Anything).
		Return(&repository.SyncResult{Created: 1}, nil)
	progressPub.On("PublishJobProgress", data.UserID, mock.AnythingOfType("map[string]interface {}")).Return()

	err := worker.ProcessTask(context.Background(), task)

	// Should succeed (participant errors are non-fatal, group is still synced)
	assert.NoError(t, err)

	syncedGroups := groupRepo.Calls[0].Arguments[3].([]models.Group)
	assert.Len(t, syncedGroups, 1)
	// When participants fail, admin detection is skipped
	assert.False(t, syncedGroups[0].IsAdmin)
}

func TestGroupsSyncWorker_ProcessTask_NegativeInstanceID(t *testing.T) {
	fake := whatsapp.NewFakeAdapter()
	instanceRepo := new(mockInstanceRepoForSync)
	groupRepo := new(mockGroupRepo)

	worker := workers.NewGroupsSyncWorker(fake, instanceRepo, groupRepo, newTestLogger(), nil)

	data := defaultJobData()
	task := buildSyncTask(t, data)

	invalidInstance := &models.Instance{
		ID:         -5,
		UserID:     "user-123",
		InstanceID: "test-instance",
		Status:     "connected",
	}

	instanceRepo.On("GetInstanceByUserAndName", mock.Anything, data.UserID, data.InstanceName).
		Return(invalidInstance, nil)

	fake.SetSession(data.InstanceName, connectedSession())
	fake.SetGroups(data.InstanceName, sampleGroups())
	fake.SetParticipants(data.InstanceName, "group1@g.us", sampleParticipantsWithAdmin())
	fake.SetParticipants(data.InstanceName, "group2@g.us", sampleParticipantsWithoutUser())

	err := worker.ProcessTask(context.Background(), task)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid instance ID format")
}

// =============================================================================
// Table-Driven Tests for Session Status Handling
// =============================================================================

func TestGroupsSyncWorker_ProcessTask_SessionStatusVariants(t *testing.T) {
	tests := []struct {
		name        string
		status      whatsapp.SessionStatus
		expectError bool
		errorMsg    string
	}{
		{
			name:        "disconnected session returns error",
			status:      whatsapp.SessionStatusDisconnected,
			expectError: true,
			errorMsg:    "instance not connected",
		},
		{
			name:        "scan QR session returns error",
			status:      whatsapp.SessionStatusScanQR,
			expectError: true,
			errorMsg:    "instance not connected",
		},
		{
			name:        "starting session returns error",
			status:      whatsapp.SessionStatusStarting,
			expectError: true,
			errorMsg:    "instance not connected",
		},
		{
			name:        "stopped session returns error",
			status:      whatsapp.SessionStatusStopped,
			expectError: true,
			errorMsg:    "instance not connected",
		},
		{
			name:        "failed session returns error",
			status:      whatsapp.SessionStatusFailed,
			expectError: true,
			errorMsg:    "instance not connected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := whatsapp.NewFakeAdapter()
			instanceRepo := new(mockInstanceRepoForSync)
			groupRepo := new(mockGroupRepo)

			worker := workers.NewGroupsSyncWorker(fake, instanceRepo, groupRepo, newTestLogger(), nil)

			data := defaultJobData()
			task := buildSyncTask(t, data)

			instanceRepo.On("GetInstanceByUserAndName", mock.Anything, data.UserID, data.InstanceName).
				Return(defaultInstance(), nil)

			fake.SetSession(data.InstanceName, &whatsapp.SessionInfo{
				Name:   data.InstanceName,
				Status: tt.status,
			})

			err := worker.ProcessTask(context.Background(), task)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// =============================================================================
// Table-Driven Tests for Provider Error Handling
// =============================================================================

func TestGroupsSyncWorker_ProcessTask_ProviderErrorVariants(t *testing.T) {
	tests := []struct {
		name        string
		errorCode   whatsapp.ErrorCode
		expectNil   bool
		errorMsg    string
	}{
		{
			name:      "SESSION_NOT_FOUND returns nil (skip)",
			errorCode: whatsapp.ErrCodeSessionNotFound,
			expectNil: true,
		},
		{
			name:      "PROVIDER_ERROR returns nil (skip)",
			errorCode: whatsapp.ErrCodeProviderError,
			expectNil: true,
		},
		{
			name:      "RATE_LIMITED returns error (retry)",
			errorCode: whatsapp.ErrCodeRateLimited,
			expectNil: false,
			errorMsg:  "failed to check status",
		},
		{
			name:      "TIMEOUT returns error (retry)",
			errorCode: whatsapp.ErrCodeTimeout,
			expectNil: false,
			errorMsg:  "failed to check status",
		},
		{
			name:      "NETWORK_ERROR returns error (retry)",
			errorCode: whatsapp.ErrCodeNetworkError,
			expectNil: false,
			errorMsg:  "failed to check status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := whatsapp.NewFakeAdapter()
			instanceRepo := new(mockInstanceRepoForSync)
			groupRepo := new(mockGroupRepo)

			worker := workers.NewGroupsSyncWorker(fake, instanceRepo, groupRepo, newTestLogger(), nil)

			data := defaultJobData()
			task := buildSyncTask(t, data)

			instanceRepo.On("GetInstanceByUserAndName", mock.Anything, data.UserID, data.InstanceName).
				Return(defaultInstance(), nil)

			fake.GetSessionStatusFunc = func(_ context.Context, _ string) (*whatsapp.SessionInfo, error) {
				return nil, whatsapp.NewProviderError(tt.errorCode, string(tt.errorCode), nil)
			}

			err := worker.ProcessTask(context.Background(), task)

			if tt.expectNil {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			}
		})
	}
}

// =============================================================================
// Test: Participant Phone Resolution Variants
// =============================================================================

func TestGroupsSyncWorker_ProcessTask_PhoneResolutionVariants(t *testing.T) {
	tests := []struct {
		name         string
		participants []whatsapp.ParticipantInfo
		expectAdmin  bool
		description  string
	}{
		{
			name: "matched via PhoneNumber field",
			participants: []whatsapp.ParticipantInfo{
				{ID: "random@lid", PhoneNumber: "5511999999999", IsAdmin: true},
			},
			expectAdmin: true,
			description: "phone resolved from PhoneNumber field should match session user",
		},
		{
			name: "matched via ID extraction when PhoneNumber empty",
			participants: []whatsapp.ParticipantInfo{
				{ID: "5511999999999@c.us", PhoneNumber: "", IsAdmin: true},
			},
			expectAdmin: true,
			description: "phone extracted from ID should match when PhoneNumber is empty",
		},
		{
			name: "no match with different phone",
			participants: []whatsapp.ParticipantInfo{
				{ID: "5511888888888@c.us", PhoneNumber: "5511888888888", IsAdmin: true},
			},
			expectAdmin: false,
			description: "different phone should not match",
		},
		{
			name:         "empty participants list",
			participants: []whatsapp.ParticipantInfo{},
			expectAdmin:  false,
			description:  "empty participants should result in no admin detection",
		},
		{
			name: "matched but not admin",
			participants: []whatsapp.ParticipantInfo{
				{ID: "5511999999999@c.us", PhoneNumber: "5511999999999", IsAdmin: false, IsSuperAdmin: false},
			},
			expectAdmin: false,
			description: "matched user who is not admin should not be marked as admin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := whatsapp.NewFakeAdapter()
			instanceRepo := new(mockInstanceRepoForSync)
			groupRepo := new(mockGroupRepo)

			worker := workers.NewGroupsSyncWorker(fake, instanceRepo, groupRepo, newTestLogger(), nil)

			data := defaultJobData()
			task := buildSyncTask(t, data)

			instanceRepo.On("GetInstanceByUserAndName", mock.Anything, data.UserID, data.InstanceName).
				Return(defaultInstance(), nil)

			groups := []whatsapp.GroupInfo{
				{ID: "test@g.us", Subject: "Test", Size: 5},
			}
			fake.SetSession(data.InstanceName, connectedSession())
			fake.SetGroups(data.InstanceName, groups)
			fake.SetParticipants(data.InstanceName, "test@g.us", tt.participants)

			groupRepo.On("SyncGroupsWithStats", mock.Anything, data.UserID, 1, mock.AnythingOfType("[]models.Group"), mock.Anything).
				Return(&repository.SyncResult{Created: 1}, nil)

			err := worker.ProcessTask(context.Background(), task)

			assert.NoError(t, err)
			syncedGroups := groupRepo.Calls[0].Arguments[3].([]models.Group)
			assert.Equal(t, tt.expectAdmin, syncedGroups[0].IsAdmin, tt.description)
		})
	}
}

// =============================================================================
// Test: SyncOptions are passed correctly
// =============================================================================

func TestGroupsSyncWorker_ProcessTask_MoreThanThreeParticipants(t *testing.T) {
	// Exercises the `j >= 3` break in the participant debug logging loop (lines 192-193).
	// With >3 participants, the debug sample is capped at 3, and the remaining
	// participants are still checked for admin detection.
	fake := whatsapp.NewFakeAdapter()
	instanceRepo := new(mockInstanceRepoForSync)
	groupRepo := new(mockGroupRepo)

	worker := workers.NewGroupsSyncWorker(fake, instanceRepo, groupRepo, newTestLogger(), nil)

	data := defaultJobData()
	task := buildSyncTask(t, data)

	instanceRepo.On("GetInstanceByUserAndName", mock.Anything, data.UserID, data.InstanceName).
		Return(defaultInstance(), nil)

	groups := []whatsapp.GroupInfo{
		{ID: "big-group@g.us", Subject: "Big Group", Size: 6},
	}
	fake.SetSession(data.InstanceName, connectedSession())
	fake.SetGroups(data.InstanceName, groups)

	// 5 participants; session user (5511999999999) is the 5th and is admin.
	// The debug logging loop caps at j=3, but admin detection must still find the user at j=4.
	fake.SetParticipants(data.InstanceName, "big-group@g.us", []whatsapp.ParticipantInfo{
		{ID: "5511111111111@c.us", PhoneNumber: "5511111111111", IsAdmin: false},
		{ID: "5511222222222@c.us", PhoneNumber: "5511222222222", IsAdmin: false},
		{ID: "5511333333333@c.us", PhoneNumber: "5511333333333", IsAdmin: false},
		{ID: "5511444444444@c.us", PhoneNumber: "5511444444444", IsAdmin: false},
		{ID: "5511999999999@c.us", PhoneNumber: "5511999999999", IsAdmin: true, IsSuperAdmin: false},
	})

	groupRepo.On("SyncGroupsWithStats", mock.Anything, data.UserID, 1, mock.AnythingOfType("[]models.Group"), mock.Anything).
		Return(&repository.SyncResult{Created: 1}, nil)

	err := worker.ProcessTask(context.Background(), task)

	assert.NoError(t, err)

	syncedGroups := groupRepo.Calls[0].Arguments[3].([]models.Group)
	require.Len(t, syncedGroups, 1)
	assert.True(t, syncedGroups[0].IsAdmin, "user at index 4 should still be detected as admin despite debug log capping at 3")
}

// =============================================================================
// Tiered Admin Detection Tests
// =============================================================================

func TestGroupsSyncWorker_ProcessTask_Tier1_GroupLevelIsAdmin(t *testing.T) {
	fake := whatsapp.NewFakeAdapter()
	instanceRepo := new(mockInstanceRepoForSync)
	groupRepo := new(mockGroupRepo)
	progressPub := new(mockProgressPub)

	worker := workers.NewGroupsSyncWorker(fake, instanceRepo, groupRepo, newTestLogger(), progressPub)

	data := defaultJobData()
	task := buildSyncTask(t, data)

	instanceRepo.On("GetInstanceByUserAndName", mock.Anything, data.UserID, data.InstanceName).
		Return(defaultInstance(), nil)

	// Group with IsAdmin=true at the group level (GOWS engine flag)
	groups := []whatsapp.GroupInfo{
		{
			ID:      "tier1-group@g.us",
			Subject: "Tier 1 Group",
			Size:    10,
			IsAdmin: true,
		},
	}
	fake.SetSession(data.InstanceName, connectedSession())
	fake.SetGroups(data.InstanceName, groups)
	// Do NOT set participants — tier 1 should short-circuit before any API call

	groupRepo.On("SyncGroupsWithStats", mock.Anything, data.UserID, 1, mock.AnythingOfType("[]models.Group"), mock.Anything).
		Return(&repository.SyncResult{Created: 1}, nil)
	progressPub.On("PublishJobProgress", data.UserID, mock.AnythingOfType("map[string]interface {}")).Return()

	err := worker.ProcessTask(context.Background(), task)

	assert.NoError(t, err)

	syncedGroups := groupRepo.Calls[0].Arguments[3].([]models.Group)
	require.Len(t, syncedGroups, 1)
	assert.True(t, syncedGroups[0].IsAdmin, "tier 1: group-level IsAdmin should set admin=true")

	// Verify GetGroupParticipants was NOT called (tier 1 short-circuits)
	assert.Equal(t, 0, fake.CallCount("GetGroupParticipants"),
		"tier 1 should not call GetGroupParticipants")
}

func TestGroupsSyncWorker_ProcessTask_Tier1_GroupLevelIsSuperAdmin(t *testing.T) {
	fake := whatsapp.NewFakeAdapter()
	instanceRepo := new(mockInstanceRepoForSync)
	groupRepo := new(mockGroupRepo)

	worker := workers.NewGroupsSyncWorker(fake, instanceRepo, groupRepo, newTestLogger(), nil)

	data := defaultJobData()
	task := buildSyncTask(t, data)

	instanceRepo.On("GetInstanceByUserAndName", mock.Anything, data.UserID, data.InstanceName).
		Return(defaultInstance(), nil)

	groups := []whatsapp.GroupInfo{
		{
			ID:           "tier1-super@g.us",
			Subject:      "Tier 1 SuperAdmin",
			Size:         5,
			IsSuperAdmin: true,
		},
	}
	fake.SetSession(data.InstanceName, connectedSession())
	fake.SetGroups(data.InstanceName, groups)

	groupRepo.On("SyncGroupsWithStats", mock.Anything, data.UserID, 1, mock.AnythingOfType("[]models.Group"), mock.Anything).
		Return(&repository.SyncResult{Created: 1}, nil)

	err := worker.ProcessTask(context.Background(), task)

	assert.NoError(t, err)

	syncedGroups := groupRepo.Calls[0].Arguments[3].([]models.Group)
	require.Len(t, syncedGroups, 1)
	assert.True(t, syncedGroups[0].IsAdmin, "tier 1: group-level IsSuperAdmin should set admin=true")
	assert.Equal(t, 0, fake.CallCount("GetGroupParticipants"))
}

func TestGroupsSyncWorker_ProcessTask_Tier2_EmbeddedParticipants(t *testing.T) {
	fake := whatsapp.NewFakeAdapter()
	instanceRepo := new(mockInstanceRepoForSync)
	groupRepo := new(mockGroupRepo)
	progressPub := new(mockProgressPub)

	worker := workers.NewGroupsSyncWorker(fake, instanceRepo, groupRepo, newTestLogger(), progressPub)

	data := defaultJobData()
	task := buildSyncTask(t, data)

	instanceRepo.On("GetInstanceByUserAndName", mock.Anything, data.UserID, data.InstanceName).
		Return(defaultInstance(), nil)

	// Group without IsAdmin flag but WITH embedded participants
	groups := []whatsapp.GroupInfo{
		{
			ID:      "tier2-group@g.us",
			Subject: "Tier 2 Group",
			Size:    3,
			Participants: []whatsapp.ParticipantInfo{
				{ID: "5511888888888@c.us", PhoneNumber: "5511888888888", IsAdmin: false},
				{ID: "5511999999999@c.us", PhoneNumber: "5511999999999", IsAdmin: true}, // session user
			},
		},
	}
	fake.SetSession(data.InstanceName, connectedSession())
	fake.SetGroups(data.InstanceName, groups)

	groupRepo.On("SyncGroupsWithStats", mock.Anything, data.UserID, 1, mock.AnythingOfType("[]models.Group"), mock.Anything).
		Return(&repository.SyncResult{Created: 1}, nil)
	progressPub.On("PublishJobProgress", data.UserID, mock.AnythingOfType("map[string]interface {}")).Return()

	err := worker.ProcessTask(context.Background(), task)

	assert.NoError(t, err)

	syncedGroups := groupRepo.Calls[0].Arguments[3].([]models.Group)
	require.Len(t, syncedGroups, 1)
	assert.True(t, syncedGroups[0].IsAdmin, "tier 2: embedded participant match should set admin=true")

	// Verify GetGroupParticipants was NOT called (tier 2 resolves without API call)
	assert.Equal(t, 0, fake.CallCount("GetGroupParticipants"),
		"tier 2 should not call GetGroupParticipants")
}

func TestGroupsSyncWorker_ProcessTask_Tier2_EmbeddedNotAdmin(t *testing.T) {
	fake := whatsapp.NewFakeAdapter()
	instanceRepo := new(mockInstanceRepoForSync)
	groupRepo := new(mockGroupRepo)

	worker := workers.NewGroupsSyncWorker(fake, instanceRepo, groupRepo, newTestLogger(), nil)

	data := defaultJobData()
	task := buildSyncTask(t, data)

	instanceRepo.On("GetInstanceByUserAndName", mock.Anything, data.UserID, data.InstanceName).
		Return(defaultInstance(), nil)

	// Group with embedded participants where session user is present but NOT admin
	groups := []whatsapp.GroupInfo{
		{
			ID:      "tier2-notadmin@g.us",
			Subject: "Tier 2 Not Admin",
			Size:    3,
			Participants: []whatsapp.ParticipantInfo{
				{ID: "5511999999999@c.us", PhoneNumber: "5511999999999", IsAdmin: false, IsSuperAdmin: false},
			},
		},
	}
	fake.SetSession(data.InstanceName, connectedSession())
	fake.SetGroups(data.InstanceName, groups)

	groupRepo.On("SyncGroupsWithStats", mock.Anything, data.UserID, 1, mock.AnythingOfType("[]models.Group"), mock.Anything).
		Return(&repository.SyncResult{Created: 1}, nil)

	err := worker.ProcessTask(context.Background(), task)

	assert.NoError(t, err)

	syncedGroups := groupRepo.Calls[0].Arguments[3].([]models.Group)
	require.Len(t, syncedGroups, 1)
	assert.False(t, syncedGroups[0].IsAdmin, "tier 2: session user found but not admin")
	assert.Equal(t, 0, fake.CallCount("GetGroupParticipants"),
		"tier 2 matched user, should not fall through to tier 3")
}

func TestGroupsSyncWorker_ProcessTask_Tier1_SkipsApiCallForMixedGroups(t *testing.T) {
	fake := whatsapp.NewFakeAdapter()
	instanceRepo := new(mockInstanceRepoForSync)
	groupRepo := new(mockGroupRepo)
	progressPub := new(mockProgressPub)

	worker := workers.NewGroupsSyncWorker(fake, instanceRepo, groupRepo, newTestLogger(), progressPub)

	data := defaultJobData()
	task := buildSyncTask(t, data)

	instanceRepo.On("GetInstanceByUserAndName", mock.Anything, data.UserID, data.InstanceName).
		Return(defaultInstance(), nil)

	// Mix of tier 1 and tier 3 groups
	groups := []whatsapp.GroupInfo{
		{
			ID:      "tier1@g.us",
			Subject: "Tier 1 (flag)",
			Size:    5,
			IsAdmin: true,
		},
		{
			ID:      "tier3@g.us",
			Subject: "Tier 3 (api call)",
			Size:    10,
		},
	}
	fake.SetSession(data.InstanceName, connectedSession())
	fake.SetGroups(data.InstanceName, groups)

	// Only tier3 group needs participants
	fake.SetParticipants(data.InstanceName, "tier3@g.us", []whatsapp.ParticipantInfo{
		{ID: "5511999999999@c.us", PhoneNumber: "5511999999999", IsAdmin: true},
	})

	groupRepo.On("SyncGroupsWithStats", mock.Anything, data.UserID, 1, mock.AnythingOfType("[]models.Group"), mock.Anything).
		Return(&repository.SyncResult{Created: 2}, nil)
	progressPub.On("PublishJobProgress", data.UserID, mock.AnythingOfType("map[string]interface {}")).Return()

	err := worker.ProcessTask(context.Background(), task)

	assert.NoError(t, err)

	syncedGroups := groupRepo.Calls[0].Arguments[3].([]models.Group)
	require.Len(t, syncedGroups, 2)
	assert.True(t, syncedGroups[0].IsAdmin, "tier 1 group should be admin")
	assert.True(t, syncedGroups[1].IsAdmin, "tier 3 group should be admin via API call")

	// Only 1 GetGroupParticipants call (for tier3 group only)
	assert.Equal(t, 1, fake.CallCount("GetGroupParticipants"),
		"only tier 3 group should trigger GetGroupParticipants call")
}

func TestGroupsSyncWorker_ProcessTask_AllTiersFail_NonAdmin(t *testing.T) {
	fake := whatsapp.NewFakeAdapter()
	instanceRepo := new(mockInstanceRepoForSync)
	groupRepo := new(mockGroupRepo)

	worker := workers.NewGroupsSyncWorker(fake, instanceRepo, groupRepo, newTestLogger(), nil)

	data := defaultJobData()
	task := buildSyncTask(t, data)

	instanceRepo.On("GetInstanceByUserAndName", mock.Anything, data.UserID, data.InstanceName).
		Return(defaultInstance(), nil)

	// Group with no flags, no embedded participants, session user not in API participants, not owner
	groups := []whatsapp.GroupInfo{
		{
			ID:      "noadmin@g.us",
			Subject: "No Admin",
			Owner:   "5511777777777@c.us",
			Size:    5,
		},
	}
	fake.SetSession(data.InstanceName, connectedSession())
	fake.SetGroups(data.InstanceName, groups)
	fake.SetParticipants(data.InstanceName, "noadmin@g.us", sampleParticipantsWithoutUser())

	groupRepo.On("SyncGroupsWithStats", mock.Anything, data.UserID, 1, mock.AnythingOfType("[]models.Group"), mock.Anything).
		Return(&repository.SyncResult{Created: 1}, nil)

	err := worker.ProcessTask(context.Background(), task)

	assert.NoError(t, err)

	syncedGroups := groupRepo.Calls[0].Arguments[3].([]models.Group)
	require.Len(t, syncedGroups, 1)
	assert.False(t, syncedGroups[0].IsAdmin, "all tiers failed — group should be non-admin")
}

func TestGroupsSyncWorker_ProcessTask_SyncOptionsBatchSize(t *testing.T) {
	fake := whatsapp.NewFakeAdapter()
	instanceRepo := new(mockInstanceRepoForSync)
	groupRepo := new(mockGroupRepo)

	worker := workers.NewGroupsSyncWorker(fake, instanceRepo, groupRepo, newTestLogger(), nil)

	data := defaultJobData()
	task := buildSyncTask(t, data)

	instanceRepo.On("GetInstanceByUserAndName", mock.Anything, data.UserID, data.InstanceName).
		Return(defaultInstance(), nil)

	fake.SetSession(data.InstanceName, connectedSession())
	fake.SetGroups(data.InstanceName, []whatsapp.GroupInfo{
		{ID: "g@g.us", Subject: "G", Size: 1},
	})

	groupRepo.On("SyncGroupsWithStats", mock.Anything, data.UserID, 1, mock.AnythingOfType("[]models.Group"), repository.SyncOptions{BatchSize: 50}).
		Return(&repository.SyncResult{}, nil)

	err := worker.ProcessTask(context.Background(), task)

	assert.NoError(t, err)
	groupRepo.AssertExpectations(t)

	// Verify the exact SyncOptions passed
	syncCall := groupRepo.Calls[0]
	opts := syncCall.Arguments[4].(repository.SyncOptions)
	assert.Equal(t, 50, opts.BatchSize)
}
