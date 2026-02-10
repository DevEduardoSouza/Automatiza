package workers

import (
	"context"
	"testing"
	"time"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"

	"go.uber.org/zap"
)

// mockDispatchLogRepo is a mock implementation of DispatchLogRepository
type mockDispatchLogRepo struct {
	createIfNotExistsCalled bool
	lastLog                 *models.DispatchLog
	lastDedupWindow         int
	existsResult            bool
	createResult            *models.DispatchLog
	wasCreated              bool
}

func (m *mockDispatchLogRepo) Create(ctx context.Context, log *models.DispatchLog) (*models.DispatchLog, error) {
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
	return m.existsResult, nil
}

func (m *mockDispatchLogRepo) CreateIfNotExists(ctx context.Context, log *models.DispatchLog, dedupWindowHours int) (*models.DispatchLog, bool, error) {
	m.createIfNotExistsCalled = true
	m.lastLog = log
	m.lastDedupWindow = dedupWindowHours
	if m.createResult != nil {
		return m.createResult, m.wasCreated, nil
	}
	log.ID = 1
	return log, true, nil
}

// stubInstanceRepo is a stub implementation of InstanceRepository for dispatch log tests
type stubInstanceRepo struct {
	instance *models.Instance
}

func (m *stubInstanceRepo) GetUserInstances(ctx context.Context, userID string) ([]models.Instance, error) {
	return nil, nil
}

func (m *stubInstanceRepo) GetInstanceByID(ctx context.Context, instanceID int) (*models.Instance, error) {
	return m.instance, nil
}

func (m *stubInstanceRepo) GetInstanceByName(ctx context.Context, instanceName string) (*models.Instance, error) {
	return m.instance, nil
}

func (m *stubInstanceRepo) GetInstanceByUserAndName(ctx context.Context, userID, instanceName string) (*models.Instance, error) {
	return m.instance, nil
}

func (m *stubInstanceRepo) GetAllConnectedInstances(ctx context.Context) ([]models.Instance, error) {
	return nil, nil
}

func (m *stubInstanceRepo) UpsertInstance(ctx context.Context, userID string, data *models.Instance) (*models.Instance, error) {
	return data, nil
}

func (m *stubInstanceRepo) UpdateInstanceStatus(ctx context.Context, instanceID int, status string) error {
	return nil
}

func (m *stubInstanceRepo) DeleteInstance(ctx context.Context, instanceID int) error {
	return nil
}

func TestDispatchLogProcessor_ProcessMessage(t *testing.T) {
	logger := zap.NewNop()

	tests := []struct {
		name             string
		message          *whatsapp.Message
		instance         *models.Instance
		wantLogCreated   bool
		wantStatus       string
		wantCanonicalKey string
	}{
		{
			name: "creates log for incoming message",
			message: &whatsapp.Message{
				ID:        "msg123",
				ChatID:    "group123@g.us",
				FromMe:    false,
				Body:      "Check this out: https://mercadolivre.com.br/product",
				Timestamp: time.Now(),
			},
			instance: &models.Instance{
				ID:         1,
				UserID:     "user123",
				InstanceID: "test_session",
			},
			wantLogCreated:   true,
			wantStatus:       models.DispatchLogStatusReconciled,
			wantCanonicalKey: "reconcile:test_session:msg123",
		},
		{
			name: "skips outbound message",
			message: &whatsapp.Message{
				ID:        "msg456",
				ChatID:    "group123@g.us",
				FromMe:    true,
				Body:      "Sent by me",
				Timestamp: time.Now(),
			},
			instance: &models.Instance{
				ID:         1,
				UserID:     "user123",
				InstanceID: "test_session",
			},
			wantLogCreated: false,
		},
		{
			name: "skips empty message",
			message: &whatsapp.Message{
				ID:        "msg789",
				ChatID:    "group123@g.us",
				FromMe:    false,
				Body:      "",
				HasMedia:  false,
				Timestamp: time.Now(),
			},
			instance: &models.Instance{
				ID:         1,
				UserID:     "user123",
				InstanceID: "test_session",
			},
			wantLogCreated: false,
		},
		{
			name: "processes message with media",
			message: &whatsapp.Message{
				ID:        "msg_media",
				ChatID:    "group456@g.us",
				FromMe:    false,
				Body:      "",
				HasMedia:  true,
				MediaType: "image",
				Timestamp: time.Now(),
			},
			instance: &models.Instance{
				ID:         1,
				UserID:     "user123",
				InstanceID: "test_session",
			},
			wantLogCreated:   true,
			wantStatus:       models.DispatchLogStatusReconciled,
			wantCanonicalKey: "reconcile:test_session:msg_media",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dispatchLogRepo := &mockDispatchLogRepo{}
			instanceRepo := &stubInstanceRepo{instance: tt.instance}

			processor := NewDispatchLogProcessor(DispatchLogProcessorConfig{
				DispatchLogRepo:  dispatchLogRepo,
				InstanceRepo:     instanceRepo,
				Logger:           logger,
				DedupWindowHours: 168,
			})

			err := processor.ProcessMessage(context.Background(), "test_session", tt.message)
			if err != nil {
				t.Errorf("ProcessMessage() error = %v", err)
				return
			}

			if tt.wantLogCreated && !dispatchLogRepo.createIfNotExistsCalled {
				t.Error("ProcessMessage() expected CreateIfNotExists to be called")
			}

			if !tt.wantLogCreated && dispatchLogRepo.createIfNotExistsCalled {
				t.Error("ProcessMessage() expected CreateIfNotExists to NOT be called")
			}

			if tt.wantLogCreated && dispatchLogRepo.lastLog != nil {
				if dispatchLogRepo.lastLog.Status != tt.wantStatus {
					t.Errorf("ProcessMessage() status = %v, want %v", dispatchLogRepo.lastLog.Status, tt.wantStatus)
				}

				if dispatchLogRepo.lastLog.CanonicalKey == nil || *dispatchLogRepo.lastLog.CanonicalKey != tt.wantCanonicalKey {
					got := ""
					if dispatchLogRepo.lastLog.CanonicalKey != nil {
						got = *dispatchLogRepo.lastLog.CanonicalKey
					}
					t.Errorf("ProcessMessage() canonicalKey = %v, want %v", got, tt.wantCanonicalKey)
				}
			}
		})
	}
}

func TestDispatchLogProcessor_ExtractProductURL(t *testing.T) {
	logger := zap.NewNop()
	processor := NewDispatchLogProcessor(DispatchLogProcessorConfig{
		DispatchLogRepo:  &mockDispatchLogRepo{},
		InstanceRepo:     &stubInstanceRepo{},
		Logger:           logger,
		DedupWindowHours: 168,
	})

	tests := []struct {
		name    string
		body    string
		wantURL bool
	}{
		{
			name:    "extracts mercadolivre URL",
			body:    "Check this: https://mercadolivre.com.br/produto/12345",
			wantURL: true,
		},
		{
			name:    "extracts amazon URL",
			body:    "Buy here: https://amazon.com/dp/B08N5WRWNW",
			wantURL: true,
		},
		{
			name:    "extracts shopee URL",
			body:    "Great deal: https://shopee.com.br/product/123",
			wantURL: true,
		},
		{
			name:    "no URL in text",
			body:    "Just a regular message without links",
			wantURL: false,
		},
		{
			name:    "empty body",
			body:    "",
			wantURL: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.extractProductURL(tt.body)
			if tt.wantURL && result == nil {
				t.Error("extractProductURL() expected URL to be found")
			}
			if !tt.wantURL && result != nil {
				t.Errorf("extractProductURL() expected nil, got %v", *result)
			}
		})
	}
}
