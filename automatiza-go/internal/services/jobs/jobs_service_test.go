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
	"automatiza-go/internal/services/mercadolivre"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// =============================================================================
// Mock Implementations
// =============================================================================

// MockJobRepository implements repository.JobRepository for testing
type MockJobRepository struct {
	mock.Mock
}

func (m *MockJobRepository) GetByID(ctx context.Context, id string) (*models.StandaloneJob, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.StandaloneJob), args.Error(1)
}

func (m *MockJobRepository) GetByUserID(ctx context.Context, userID string, status *models.StandaloneJobStatus, limit int) ([]models.StandaloneJob, error) {
	args := m.Called(ctx, userID, status, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.StandaloneJob), args.Error(1)
}

func (m *MockJobRepository) GetPendingDueJobs(ctx context.Context, limit int) ([]models.StandaloneJob, error) {
	args := m.Called(ctx, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.StandaloneJob), args.Error(1)
}

func (m *MockJobRepository) Create(ctx context.Context, job *models.StandaloneJob) (*models.StandaloneJob, error) {
	args := m.Called(ctx, job)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.StandaloneJob), args.Error(1)
}

func (m *MockJobRepository) Update(ctx context.Context, id string, updates map[string]interface{}) error {
	args := m.Called(ctx, id, updates)
	return args.Error(0)
}

func (m *MockJobRepository) UpdateStatus(ctx context.Context, id string, status models.StandaloneJobStatus, errorMsg *string) error {
	args := m.Called(ctx, id, status, errorMsg)
	return args.Error(0)
}

func (m *MockJobRepository) Delete(ctx context.Context, id, userID string) error {
	args := m.Called(ctx, id, userID)
	return args.Error(0)
}

func (m *MockJobRepository) ListByUserID(ctx context.Context, params repository.JobListParams) (*repository.JobListResult, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.JobListResult), args.Error(1)
}

func (m *MockJobRepository) ScheduleRetry(ctx context.Context, id string, nextRunAt time.Time) error {
	args := m.Called(ctx, id, nextRunAt)
	return args.Error(0)
}

func (m *MockJobRepository) CountByStatus(ctx context.Context, userID string) (map[models.StandaloneJobStatus]int, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[models.StandaloneJobStatus]int), args.Error(1)
}

// MockGroupRepository implements repository.GroupRepository for testing
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

func (m *MockGroupRepository) GetGroupsByWAGroupIDs(ctx context.Context, userID string, waGroupIDs []string) ([]models.Group, error) {
	args := m.Called(ctx, userID, waGroupIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Group), args.Error(1)
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

func (m *MockGroupRepository) GetUserGroupsPaginated(ctx context.Context, userID string, page, pageSize int, adminOnly bool, search string, instanceID int) (*repository.GroupPaginatedResult, error) {
	args := m.Called(ctx, userID, page, pageSize, adminOnly, search, instanceID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.GroupPaginatedResult), args.Error(1)
}

func (m *MockGroupRepository) CountGroupsByInstance(ctx context.Context, userID string, instanceID int) (int, error) {
	args := m.Called(ctx, userID, instanceID)
	return args.Int(0), args.Error(1)
}

func (m *MockGroupRepository) GetAdminInstanceIDsForGroups(ctx context.Context, userID string, waGroupIDs []string) (map[string][]string, error) {
	return map[string][]string{}, nil
}

func (m *MockGroupRepository) GetInstancesForGroups(ctx context.Context, userID string, waGroupIDs []string) (map[string]*models.Instance, error) {
	args := m.Called(ctx, userID, waGroupIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]*models.Instance), args.Error(1)
}

// MockInstanceRepository implements repository.InstanceRepository for testing
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

// MockMLService implements mercadolivre.Service for testing
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

// Compile-time interface verification
var _ mercadolivre.Service = (*MockMLService)(nil)

// =============================================================================
// Test Fixtures
// =============================================================================

func newTestService(t *testing.T) (jobs.Service, *MockJobRepository, *MockInstanceRepository, *MockGroupRepository, *whatsapp.FakeAdapter, *MockMLService) {
	t.Helper()
	logger := zaptest.NewLogger(t)

	jobRepo := new(MockJobRepository)
	instanceRepo := new(MockInstanceRepository)
	groupRepo := new(MockGroupRepository)
	waProvider := whatsapp.NewFakeAdapter()
	mlService := new(MockMLService)

	svc := jobs.NewService(jobRepo, instanceRepo, groupRepo, waProvider, nil, nil, nil, logger)

	return svc, jobRepo, instanceRepo, groupRepo, waProvider, mlService
}

func validJobPayload(groups []string) models.JobPayload {
	return models.JobPayload{
		Groups:  groups,
		Message: "Test message",
	}
}

// =============================================================================
// CreateJob Tests
// =============================================================================

func TestCreateJob_SendMessage_Success(t *testing.T) {
	svc, jobRepo, _, _, _, _ := newTestService(t)
	ctx := context.Background()

	job := &models.StandaloneJob{
		ActionType:   models.JobActionSendMessage,
		ScheduledFor: time.Now().Add(time.Hour),
		Payload:      validJobPayload([]string{"group1@g.us", "group2@g.us"}),
	}

	createdJob := *job
	createdJob.ID = "job-123"
	createdJob.UserID = "user-1"
	createdJob.Status = models.StandaloneJobStatusPending
	createdJob.CreatedAt = time.Now()
	createdJob.UpdatedAt = time.Now()

	jobRepo.On("Create", ctx, mock.AnythingOfType("*models.StandaloneJob")).Return(&createdJob, nil)

	result, err := svc.CreateJob(ctx, "user-1", job)

	require.NoError(t, err)
	assert.Equal(t, "job-123", result.ID)
	assert.Equal(t, "user-1", result.UserID)
	jobRepo.AssertExpectations(t)
}

func TestCreateJob_SendMessage_WithMedia_Success(t *testing.T) {
	svc, jobRepo, _, _, _, _ := newTestService(t)
	ctx := context.Background()

	job := &models.StandaloneJob{
		ActionType:   models.JobActionSendMessage,
		ScheduledFor: time.Now().Add(time.Hour),
		Payload: models.JobPayload{
			Groups:    []string{"group1@g.us"},
			Media:     "base64-image-data",
			MediaType: "image",
		},
	}

	createdJob := *job
	createdJob.ID = "job-media"
	createdJob.UserID = "user-1"

	jobRepo.On("Create", ctx, mock.AnythingOfType("*models.StandaloneJob")).Return(&createdJob, nil)

	result, err := svc.CreateJob(ctx, "user-1", job)

	require.NoError(t, err)
	assert.Equal(t, "job-media", result.ID)
	jobRepo.AssertExpectations(t)
}

func TestCreateJob_EmptyGroups_Error(t *testing.T) {
	svc, _, _, _, _, _ := newTestService(t)
	ctx := context.Background()

	tests := []struct {
		name   string
		groups []string
	}{
		{"empty slice", []string{}},
		{"nil slice", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := &models.StandaloneJob{
				ActionType:   models.JobActionSendMessage,
				ScheduledFor: time.Now().Add(time.Hour),
				Payload: models.JobPayload{
					Groups:  tt.groups,
					Message: "Test",
				},
			}

			result, err := svc.CreateJob(ctx, "user-1", job)

			assert.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), "pelo menos um grupo")
		})
	}
}

func TestCreateJob_SendMessage_NoContent_Error(t *testing.T) {
	svc, _, _, _, _, _ := newTestService(t)
	ctx := context.Background()

	job := &models.StandaloneJob{
		ActionType:   models.JobActionSendMessage,
		ScheduledFor: time.Now().Add(time.Hour),
		Payload: models.JobPayload{
			Groups:  []string{"group1@g.us"},
			Message: "",
			Media:   "",
		},
	}

	result, err := svc.CreateJob(ctx, "user-1", job)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "mensagem ou midia")
}

func TestCreateJob_SendPoll_Success(t *testing.T) {
	svc, jobRepo, _, _, _, _ := newTestService(t)
	ctx := context.Background()

	job := &models.StandaloneJob{
		ActionType:   models.JobActionSendPoll,
		ScheduledFor: time.Now().Add(time.Hour),
		Payload: models.JobPayload{
			Groups:          []string{"group1@g.us"},
			Question:        "Qual sua cor favorita?",
			Options:         []string{"Vermelho", "Azul", "Verde"},
			SelectableCount: 1,
		},
	}

	createdJob := *job
	createdJob.ID = "job-poll"
	createdJob.UserID = "user-1"

	jobRepo.On("Create", ctx, mock.AnythingOfType("*models.StandaloneJob")).Return(&createdJob, nil)

	result, err := svc.CreateJob(ctx, "user-1", job)

	require.NoError(t, err)
	assert.Equal(t, "job-poll", result.ID)
	jobRepo.AssertExpectations(t)
}

func TestCreateJob_SendPoll_TooFewOptions_Error(t *testing.T) {
	svc, _, _, _, _, _ := newTestService(t)
	ctx := context.Background()

	job := &models.StandaloneJob{
		ActionType:   models.JobActionSendPoll,
		ScheduledFor: time.Now().Add(time.Hour),
		Payload: models.JobPayload{
			Groups:   []string{"group1@g.us"},
			Question: "Teste",
			Options:  []string{"Opcao1"},
		},
	}

	result, err := svc.CreateJob(ctx, "user-1", job)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "mínimo 2")
}

func TestCreateJob_SendPoll_TooManyOptions_Error(t *testing.T) {
	svc, _, _, _, _, _ := newTestService(t)
	ctx := context.Background()

	options := make([]string, 15) // More than 12
	for i := range options {
		options[i] = "Option"
	}

	job := &models.StandaloneJob{
		ActionType:   models.JobActionSendPoll,
		ScheduledFor: time.Now().Add(time.Hour),
		Payload: models.JobPayload{
			Groups:   []string{"group1@g.us"},
			Question: "Teste",
			Options:  options,
		},
	}

	result, err := svc.CreateJob(ctx, "user-1", job)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "máximo 12")
}

func TestCreateJob_SendPoll_NoQuestion_Error(t *testing.T) {
	svc, _, _, _, _, _ := newTestService(t)
	ctx := context.Background()

	job := &models.StandaloneJob{
		ActionType:   models.JobActionSendPoll,
		ScheduledFor: time.Now().Add(time.Hour),
		Payload: models.JobPayload{
			Groups:   []string{"group1@g.us"},
			Question: "",
			Options:  []string{"A", "B"},
		},
	}

	result, err := svc.CreateJob(ctx, "user-1", job)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "pergunta")
}

func TestCreateJob_ChangeGroupPhoto_Success(t *testing.T) {
	svc, jobRepo, _, _, _, _ := newTestService(t)
	ctx := context.Background()

	job := &models.StandaloneJob{
		ActionType:   models.JobActionChangeGroupPhoto,
		ScheduledFor: time.Now().Add(time.Hour),
		Payload: models.JobPayload{
			Groups: []string{"group1@g.us"},
			Image:  "base64-image-data",
		},
	}

	createdJob := *job
	createdJob.ID = "job-photo"
	createdJob.UserID = "user-1"

	jobRepo.On("Create", ctx, mock.AnythingOfType("*models.StandaloneJob")).Return(&createdJob, nil)

	result, err := svc.CreateJob(ctx, "user-1", job)

	require.NoError(t, err)
	assert.Equal(t, "job-photo", result.ID)
	jobRepo.AssertExpectations(t)
}

func TestCreateJob_ChangeGroupPhoto_NoImage_Error(t *testing.T) {
	svc, _, _, _, _, _ := newTestService(t)
	ctx := context.Background()

	job := &models.StandaloneJob{
		ActionType:   models.JobActionChangeGroupPhoto,
		ScheduledFor: time.Now().Add(time.Hour),
		Payload: models.JobPayload{
			Groups: []string{"group1@g.us"},
			Image:  "",
		},
	}

	result, err := svc.CreateJob(ctx, "user-1", job)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "imagem")
}

func TestCreateJob_ChangeGroupName_Success(t *testing.T) {
	svc, jobRepo, _, _, _, _ := newTestService(t)
	ctx := context.Background()

	job := &models.StandaloneJob{
		ActionType:   models.JobActionChangeGroupName,
		ScheduledFor: time.Now().Add(time.Hour),
		Payload: models.JobPayload{
			Groups: []string{"group1@g.us"},
			Name:   "Novo Nome do Grupo",
		},
	}

	createdJob := *job
	createdJob.ID = "job-name"
	createdJob.UserID = "user-1"

	jobRepo.On("Create", ctx, mock.AnythingOfType("*models.StandaloneJob")).Return(&createdJob, nil)

	result, err := svc.CreateJob(ctx, "user-1", job)

	require.NoError(t, err)
	assert.Equal(t, "job-name", result.ID)
	jobRepo.AssertExpectations(t)
}

func TestCreateJob_ChangeGroupName_NoName_Error(t *testing.T) {
	svc, _, _, _, _, _ := newTestService(t)
	ctx := context.Background()

	job := &models.StandaloneJob{
		ActionType:   models.JobActionChangeGroupName,
		ScheduledFor: time.Now().Add(time.Hour),
		Payload: models.JobPayload{
			Groups: []string{"group1@g.us"},
			Name:   "",
		},
	}

	result, err := svc.CreateJob(ctx, "user-1", job)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "nome do grupo")
}

func TestCreateJob_RepositoryError(t *testing.T) {
	svc, jobRepo, _, _, _, _ := newTestService(t)
	ctx := context.Background()

	job := &models.StandaloneJob{
		ActionType:   models.JobActionSendMessage,
		ScheduledFor: time.Now().Add(time.Hour),
		Payload:      validJobPayload([]string{"group1@g.us"}),
	}

	jobRepo.On("Create", ctx, mock.AnythingOfType("*models.StandaloneJob")).Return(nil, errors.New("database error"))

	result, err := svc.CreateJob(ctx, "user-1", job)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to create job")
	jobRepo.AssertExpectations(t)
}

// =============================================================================
// GetJob Tests
// =============================================================================

func TestGetJob_Success(t *testing.T) {
	svc, jobRepo, _, _, _, _ := newTestService(t)
	ctx := context.Background()

	existingJob := &models.StandaloneJob{
		ID:         "job-123",
		UserID:     "user-1",
		ActionType: models.JobActionSendMessage,
		Status:     models.StandaloneJobStatusPending,
	}

	jobRepo.On("GetByID", ctx, "job-123").Return(existingJob, nil)

	result, err := svc.GetJob(ctx, "job-123", "user-1")

	require.NoError(t, err)
	assert.Equal(t, "job-123", result.ID)
	assert.Equal(t, "user-1", result.UserID)
	jobRepo.AssertExpectations(t)
}

func TestGetJob_NotFound(t *testing.T) {
	svc, jobRepo, _, _, _, _ := newTestService(t)
	ctx := context.Background()

	jobRepo.On("GetByID", ctx, "job-not-exists").Return(nil, nil)

	result, err := svc.GetJob(ctx, "job-not-exists", "user-1")

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not found")
	jobRepo.AssertExpectations(t)
}

func TestGetJob_WrongOwner(t *testing.T) {
	svc, jobRepo, _, _, _, _ := newTestService(t)
	ctx := context.Background()

	existingJob := &models.StandaloneJob{
		ID:     "job-123",
		UserID: "user-1",
	}

	jobRepo.On("GetByID", ctx, "job-123").Return(existingJob, nil)

	result, err := svc.GetJob(ctx, "job-123", "user-2")

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not found")
	jobRepo.AssertExpectations(t)
}

// =============================================================================
// ListJobs Tests
// =============================================================================

func TestListJobs_Success(t *testing.T) {
	svc, jobRepo, _, _, _, _ := newTestService(t)
	ctx := context.Background()

	existingJobs := []models.StandaloneJob{
		{ID: "job-1", UserID: "user-1", ActionType: models.JobActionSendMessage},
		{ID: "job-2", UserID: "user-1", ActionType: models.JobActionSendPoll},
	}

	jobRepo.On("GetByUserID", ctx, "user-1", (*models.StandaloneJobStatus)(nil), 100).Return(existingJobs, nil)

	result, err := svc.ListJobs(ctx, "user-1")

	require.NoError(t, err)
	assert.Len(t, result, 2)
	jobRepo.AssertExpectations(t)
}

func TestListJobs_Empty(t *testing.T) {
	svc, jobRepo, _, _, _, _ := newTestService(t)
	ctx := context.Background()

	jobRepo.On("GetByUserID", ctx, "user-1", (*models.StandaloneJobStatus)(nil), 100).Return([]models.StandaloneJob{}, nil)

	result, err := svc.ListJobs(ctx, "user-1")

	require.NoError(t, err)
	assert.Len(t, result, 0)
	jobRepo.AssertExpectations(t)
}

// =============================================================================
// UpdateJob Tests
// =============================================================================

func TestUpdateJob_Success(t *testing.T) {
	svc, jobRepo, _, _, _, _ := newTestService(t)
	ctx := context.Background()

	existingJob := &models.StandaloneJob{
		ID:     "job-123",
		UserID: "user-1",
		Status: models.StandaloneJobStatusPending,
	}

	updatedJob := *existingJob
	updatedJob.Payload.Message = "Updated message"

	jobRepo.On("GetByID", ctx, "job-123").Return(existingJob, nil).Once()
	jobRepo.On("Update", ctx, "job-123", map[string]interface{}{"message": "Updated message"}).Return(nil)
	jobRepo.On("GetByID", ctx, "job-123").Return(&updatedJob, nil).Once()

	updates := map[string]interface{}{"message": "Updated message"}
	result, err := svc.UpdateJob(ctx, "job-123", "user-1", updates)

	require.NoError(t, err)
	assert.NotNil(t, result)
	jobRepo.AssertExpectations(t)
}

func TestUpdateJob_NotPending_Error(t *testing.T) {
	svc, jobRepo, _, _, _, _ := newTestService(t)
	ctx := context.Background()

	existingJob := &models.StandaloneJob{
		ID:     "job-123",
		UserID: "user-1",
		Status: models.StandaloneJobStatusRunning,
	}

	jobRepo.On("GetByID", ctx, "job-123").Return(existingJob, nil)

	updates := map[string]interface{}{"message": "Updated message"}
	result, err := svc.UpdateJob(ctx, "job-123", "user-1", updates)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "pendentes")
	jobRepo.AssertExpectations(t)
}

func TestUpdateJob_NotFound_Error(t *testing.T) {
	svc, jobRepo, _, _, _, _ := newTestService(t)
	ctx := context.Background()

	jobRepo.On("GetByID", ctx, "job-not-found").Return(nil, nil)

	updates := map[string]interface{}{"message": "Updated message"}
	result, err := svc.UpdateJob(ctx, "job-not-found", "user-1", updates)

	assert.Error(t, err)
	assert.Nil(t, result)
	jobRepo.AssertExpectations(t)
}

// =============================================================================
// DeleteJob Tests
// =============================================================================

func TestDeleteJob_Pending_Success(t *testing.T) {
	svc, jobRepo, _, _, _, _ := newTestService(t)
	ctx := context.Background()

	existingJob := &models.StandaloneJob{
		ID:     "job-123",
		UserID: "user-1",
		Status: models.StandaloneJobStatusPending,
	}

	jobRepo.On("GetByID", ctx, "job-123").Return(existingJob, nil)
	jobRepo.On("Delete", ctx, "job-123", "user-1").Return(nil)

	err := svc.DeleteJob(ctx, "job-123", "user-1")

	require.NoError(t, err)
	jobRepo.AssertExpectations(t)
}

func TestDeleteJob_Canceled_Success(t *testing.T) {
	svc, jobRepo, _, _, _, _ := newTestService(t)
	ctx := context.Background()

	existingJob := &models.StandaloneJob{
		ID:     "job-123",
		UserID: "user-1",
		Status: models.StandaloneJobStatusCancelled,
	}

	jobRepo.On("GetByID", ctx, "job-123").Return(existingJob, nil)
	jobRepo.On("Delete", ctx, "job-123", "user-1").Return(nil)

	err := svc.DeleteJob(ctx, "job-123", "user-1")

	require.NoError(t, err)
	jobRepo.AssertExpectations(t)
}

func TestDeleteJob_Running_Error(t *testing.T) {
	svc, jobRepo, _, _, _, _ := newTestService(t)
	ctx := context.Background()

	existingJob := &models.StandaloneJob{
		ID:     "job-123",
		UserID: "user-1",
		Status: models.StandaloneJobStatusRunning,
	}

	jobRepo.On("GetByID", ctx, "job-123").Return(existingJob, nil)

	err := svc.DeleteJob(ctx, "job-123", "user-1")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pendentes ou cancelados")
	jobRepo.AssertExpectations(t)
}

func TestDeleteJob_Done_Error(t *testing.T) {
	svc, jobRepo, _, _, _, _ := newTestService(t)
	ctx := context.Background()

	existingJob := &models.StandaloneJob{
		ID:     "job-123",
		UserID: "user-1",
		Status: models.StandaloneJobStatusDone,
	}

	jobRepo.On("GetByID", ctx, "job-123").Return(existingJob, nil)

	err := svc.DeleteJob(ctx, "job-123", "user-1")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pendentes ou cancelados")
	jobRepo.AssertExpectations(t)
}

// =============================================================================
// GetQueueStatus Tests
// =============================================================================

func TestGetQueueStatus_Success(t *testing.T) {
	svc, jobRepo, _, _, _, _ := newTestService(t)
	ctx := context.Background()

	counts := map[models.StandaloneJobStatus]int{
		models.StandaloneJobStatusPending: 5,
		models.StandaloneJobStatusRunning: 2,
		models.StandaloneJobStatusDone:    10,
		models.StandaloneJobStatusFailed:  1,
	}

	jobRepo.On("CountByStatus", ctx, "user-1").Return(counts, nil)

	result, err := svc.GetQueueStatus(ctx, "user-1")

	require.NoError(t, err)
	assert.Equal(t, 5, result.Pending)
	assert.Equal(t, 2, result.Running)
	assert.Equal(t, 10, result.Completed)
	assert.Equal(t, 1, result.Failed)
	jobRepo.AssertExpectations(t)
}

func TestGetQueueStatus_Empty(t *testing.T) {
	svc, jobRepo, _, _, _, _ := newTestService(t)
	ctx := context.Background()

	counts := map[models.StandaloneJobStatus]int{}

	jobRepo.On("CountByStatus", ctx, "user-1").Return(counts, nil)

	result, err := svc.GetQueueStatus(ctx, "user-1")

	require.NoError(t, err)
	assert.Equal(t, 0, result.Pending)
	assert.Equal(t, 0, result.Running)
	assert.Equal(t, 0, result.Completed)
	assert.Equal(t, 0, result.Failed)
	jobRepo.AssertExpectations(t)
}

func TestGetQueueStatus_Error(t *testing.T) {
	svc, jobRepo, _, _, _, _ := newTestService(t)
	ctx := context.Background()

	jobRepo.On("CountByStatus", ctx, "user-1").Return(nil, errors.New("database error"))

	result, err := svc.GetQueueStatus(ctx, "user-1")

	assert.Error(t, err)
	assert.Nil(t, result)
	jobRepo.AssertExpectations(t)
}

// =============================================================================
// ProcessScheduledJobs Tests
// =============================================================================

func TestProcessScheduledJobs_NoPendingJobs(t *testing.T) {
	svc, jobRepo, _, _, _, _ := newTestService(t)
	ctx := context.Background()

	jobRepo.On("GetPendingDueJobs", ctx, 50).Return([]models.StandaloneJob{}, nil)

	result, err := svc.ProcessScheduledJobs(ctx)

	require.NoError(t, err)
	assert.Equal(t, 0, result.Processed)
	assert.Equal(t, 0, result.Failed)
	jobRepo.AssertExpectations(t)
}

func TestProcessScheduledJobs_Success(t *testing.T) {
	svc, jobRepo, instanceRepo, groupRepo, waProvider, _ := newTestService(t)
	ctx := context.Background()

	pendingJobs := []models.StandaloneJob{
		{
			ID:         "job-1",
			UserID:     "user-1",
			ActionType: models.JobActionSendMessage,
			Status:     models.StandaloneJobStatusPending,
			Payload: models.JobPayload{
				Groups:  []string{"group1@g.us"},
				Message: "Test message",
			},
		},
	}

	connectedInstance := []models.Instance{
		{InstanceID: "instance-1", Status: "connected"},
	}

	// Mock group resolution: DB group ID -> WA group ID
	resolvedGroups := []models.Group{
		{ID: "group1@g.us", WAGroupID: "group1@g.us", IsAdmin: true},
	}

	jobRepo.On("GetPendingDueJobs", ctx, 50).Return(pendingJobs, nil)
	jobRepo.On("UpdateStatus", ctx, "job-1", models.StandaloneJobStatusRunning, (*string)(nil)).Return(nil)
	instanceRepo.On("GetUserInstances", ctx, "user-1").Return(connectedInstance, nil)
	groupRepo.On("GetGroupsByIDs", ctx, "user-1", []string{"group1@g.us"}).Return(resolvedGroups, nil)
	jobRepo.On("UpdateStatus", ctx, "job-1", models.StandaloneJobStatusDone, (*string)(nil)).Return(nil)

	result, err := svc.ProcessScheduledJobs(ctx)

	require.NoError(t, err)
	assert.Equal(t, 1, result.Processed)
	assert.Equal(t, 0, result.Failed)

	// Verify SendText was called
	assert.Equal(t, 1, waProvider.CallCount("SendText"))
	jobRepo.AssertExpectations(t)
	instanceRepo.AssertExpectations(t)
	groupRepo.AssertExpectations(t)
}

func TestProcessScheduledJobs_NoConnectedInstance(t *testing.T) {
	svc, jobRepo, instanceRepo, _, _, _ := newTestService(t)
	ctx := context.Background()

	pendingJobs := []models.StandaloneJob{
		{
			ID:         "job-1",
			UserID:     "user-1",
			ActionType: models.JobActionSendMessage,
			Status:     models.StandaloneJobStatusPending,
			Payload: models.JobPayload{
				Groups:  []string{"group1@g.us"},
				Message: "Test message",
			},
		},
	}

	disconnectedInstances := []models.Instance{
		{InstanceID: "instance-1", Status: "disconnected"},
	}

	jobRepo.On("GetPendingDueJobs", ctx, 50).Return(pendingJobs, nil)
	jobRepo.On("UpdateStatus", ctx, "job-1", models.StandaloneJobStatusRunning, (*string)(nil)).Return(nil)
	instanceRepo.On("GetUserInstances", ctx, "user-1").Return(disconnectedInstances, nil)
	jobRepo.On("UpdateStatus", ctx, "job-1", models.StandaloneJobStatusFailed, mock.AnythingOfType("*string")).Return(nil)

	result, err := svc.ProcessScheduledJobs(ctx)

	require.NoError(t, err)
	assert.Equal(t, 0, result.Processed)
	assert.Equal(t, 1, result.Failed)
	jobRepo.AssertExpectations(t)
	instanceRepo.AssertExpectations(t)
}

// =============================================================================
// Legacy Method Tests
// =============================================================================

func TestScheduleJob_NotSupported(t *testing.T) {
	svc, _, _, _, _, _ := newTestService(t)
	ctx := context.Background()

	err := svc.ScheduleJob(ctx, &models.ScheduledJob{})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not supported")
}

func TestCancelJob_NotSupported(t *testing.T) {
	svc, _, _, _, _, _ := newTestService(t)
	ctx := context.Background()

	err := svc.CancelJob(ctx, "job-id")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not supported")
}

// =============================================================================
// Table-Driven Tests for All Action Types
// =============================================================================

func TestCreateJob_AllActionTypes(t *testing.T) {
	tests := []struct {
		name        string
		actionType  models.JobActionType
		payload     models.JobPayload
		wantErr     bool
		errContains string
	}{
		{
			name:       "send_message with text",
			actionType: models.JobActionSendMessage,
			payload: models.JobPayload{
				Groups:  []string{"group1@g.us"},
				Message: "Hello",
			},
			wantErr: false,
		},
		{
			name:       "send_message with poll type",
			actionType: models.JobActionSendMessage,
			payload: models.JobPayload{
				Groups:   []string{"group1@g.us"},
				Type:     "poll",
				Question: "Q?",
				Options:  []string{"A", "B"},
			},
			wantErr: false,
		},
		{
			name:       "send_message poll with 1 option",
			actionType: models.JobActionSendMessage,
			payload: models.JobPayload{
				Groups:   []string{"group1@g.us"},
				Type:     "poll",
				Question: "Q?",
				Options:  []string{"A"},
			},
			wantErr:     true,
			errContains: "mínimo 2",
		},
		{
			name:       "send_poll success",
			actionType: models.JobActionSendPoll,
			payload: models.JobPayload{
				Groups:   []string{"group1@g.us"},
				Question: "Pergunta?",
				Options:  []string{"A", "B", "C"},
			},
			wantErr: false,
		},
		{
			name:       "update_description success",
			actionType: models.JobActionUpdateDescription,
			payload: models.JobPayload{
				Groups:      []string{"group1@g.us"},
				Description: "Nova descricao",
			},
			wantErr: false,
		},
		{
			name:       "close_groups success",
			actionType: models.JobActionCloseGroups,
			payload: models.JobPayload{
				Groups: []string{"group1@g.us", "group2@g.us"},
			},
			wantErr: false,
		},
		{
			name:       "open_groups success",
			actionType: models.JobActionOpenGroups,
			payload: models.JobPayload{
				Groups: []string{"group1@g.us"},
			},
			wantErr: false,
		},
		{
			name:       "change_group_name success",
			actionType: models.JobActionChangeGroupName,
			payload: models.JobPayload{
				Groups: []string{"group1@g.us"},
				Name:   "Novo Nome",
			},
			wantErr: false,
		},
		{
			name:       "change_group_photo success",
			actionType: models.JobActionChangeGroupPhoto,
			payload: models.JobPayload{
				Groups: []string{"group1@g.us"},
				Image:  "base64-image-data",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, jobRepo, _, _, _, _ := newTestService(t)
			ctx := context.Background()

			job := &models.StandaloneJob{
				ActionType:   tt.actionType,
				ScheduledFor: time.Now().Add(time.Hour),
				Payload:      tt.payload,
			}

			if !tt.wantErr {
				createdJob := *job
				createdJob.ID = "job-created"
				createdJob.UserID = "user-1"
				jobRepo.On("Create", ctx, mock.AnythingOfType("*models.StandaloneJob")).Return(&createdJob, nil)
			}

			result, err := svc.CreateJob(ctx, "user-1", job)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, result)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}
