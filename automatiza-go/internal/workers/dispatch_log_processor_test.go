package workers

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/config"
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
	createErr               error
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
	if m.createErr != nil {
		return nil, false, m.createErr
	}
	if m.createResult != nil {
		return m.createResult, m.wasCreated, nil
	}
	log.ID = 1
	return log, true, nil
}

func (m *mockDispatchLogRepo) DeleteOldLogsByStatus(ctx context.Context, status string, olderThan time.Duration) (int64, error) {
	return 0, nil
}

// stubInstanceRepo is a stub implementation of InstanceRepository for dispatch log tests
type stubInstanceRepo struct {
	instance *models.Instance
	err      error
}

func (m *stubInstanceRepo) GetUserInstances(ctx context.Context, userID string) ([]models.Instance, error) {
	return nil, nil
}

func (m *stubInstanceRepo) GetInstanceByID(ctx context.Context, instanceID int) (*models.Instance, error) {
	return m.instance, nil
}

func (m *stubInstanceRepo) GetInstanceByName(ctx context.Context, instanceName string) (*models.Instance, error) {
	if m.err != nil {
		return nil, m.err
	}
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

// =============================================================================
// Additional coverage tests
// =============================================================================

func TestDispatchLogProcessor_FeatureFlagDisabled(t *testing.T) {
	os.Setenv("FEATURE_USE_GO_RECONCILIATION", "false")
	config.ResetFeatureFlags()
	defer func() {
		os.Unsetenv("FEATURE_USE_GO_RECONCILIATION")
		config.ResetFeatureFlags()
	}()

	logger := zap.NewNop()
	dispatchLogRepo := &mockDispatchLogRepo{}
	instanceRepo := &stubInstanceRepo{
		instance: &models.Instance{
			ID:         1,
			UserID:     "user123",
			InstanceID: "test_session",
		},
	}

	processor := NewDispatchLogProcessor(DispatchLogProcessorConfig{
		DispatchLogRepo:  dispatchLogRepo,
		InstanceRepo:     instanceRepo,
		Logger:           logger,
		DedupWindowHours: 168,
	})

	msg := &whatsapp.Message{
		ID:        "msg_flag",
		ChatID:    "group@g.us",
		FromMe:    false,
		Body:      "https://mercadolivre.com.br/product",
		Timestamp: time.Now(),
	}

	err := processor.ProcessMessage(context.Background(), "test_session", msg)
	if err != nil {
		t.Errorf("ProcessMessage() error = %v, want nil", err)
	}

	if dispatchLogRepo.createIfNotExistsCalled {
		t.Error("ProcessMessage() expected CreateIfNotExists to NOT be called when feature flag is disabled")
	}
}

func TestDispatchLogProcessor_EmptyChatID(t *testing.T) {
	logger := zap.NewNop()
	dispatchLogRepo := &mockDispatchLogRepo{}
	instanceRepo := &stubInstanceRepo{
		instance: &models.Instance{
			ID:         1,
			UserID:     "user123",
			InstanceID: "test_session",
		},
	}

	processor := NewDispatchLogProcessor(DispatchLogProcessorConfig{
		DispatchLogRepo:  dispatchLogRepo,
		InstanceRepo:     instanceRepo,
		Logger:           logger,
		DedupWindowHours: 168,
	})

	msg := &whatsapp.Message{
		ID:        "msg_empty_chat",
		ChatID:    "",
		FromMe:    false,
		Body:      "Some message body",
		Timestamp: time.Now(),
	}

	err := processor.ProcessMessage(context.Background(), "test_session", msg)
	if err != nil {
		t.Errorf("ProcessMessage() error = %v, want nil", err)
	}

	if dispatchLogRepo.createIfNotExistsCalled {
		t.Error("ProcessMessage() expected CreateIfNotExists to NOT be called when chatID is empty")
	}
}

func TestDispatchLogProcessor_CreateIfNotExistsError(t *testing.T) {
	logger := zap.NewNop()
	expectedErr := errors.New("database connection failed")
	dispatchLogRepo := &mockDispatchLogRepo{
		createErr: expectedErr,
	}
	instanceRepo := &stubInstanceRepo{
		instance: &models.Instance{
			ID:         1,
			UserID:     "user123",
			InstanceID: "test_session",
		},
	}

	processor := NewDispatchLogProcessor(DispatchLogProcessorConfig{
		DispatchLogRepo:  dispatchLogRepo,
		InstanceRepo:     instanceRepo,
		Logger:           logger,
		DedupWindowHours: 168,
	})

	msg := &whatsapp.Message{
		ID:        "msg_db_err",
		ChatID:    "group@g.us",
		FromMe:    false,
		Body:      "https://mercadolivre.com.br/product",
		Timestamp: time.Now(),
	}

	err := processor.ProcessMessage(context.Background(), "test_session", msg)
	if err == nil {
		t.Error("ProcessMessage() expected error, got nil")
	}

	if !errors.Is(err, expectedErr) {
		t.Errorf("ProcessMessage() error = %v, want wrapped %v", err, expectedErr)
	}
}

func TestDispatchLogProcessor_InstanceNotFound(t *testing.T) {
	logger := zap.NewNop()
	dispatchLogRepo := &mockDispatchLogRepo{}
	instanceRepo := &stubInstanceRepo{
		instance: nil,
	}

	processor := NewDispatchLogProcessor(DispatchLogProcessorConfig{
		DispatchLogRepo:  dispatchLogRepo,
		InstanceRepo:     instanceRepo,
		Logger:           logger,
		DedupWindowHours: 168,
	})

	msg := &whatsapp.Message{
		ID:        "msg_no_instance",
		ChatID:    "group@g.us",
		FromMe:    false,
		Body:      "Hello world",
		Timestamp: time.Now(),
	}

	err := processor.ProcessMessage(context.Background(), "unknown_session", msg)
	if err != nil {
		t.Errorf("ProcessMessage() error = %v, want nil (should skip gracefully)", err)
	}

	if dispatchLogRepo.createIfNotExistsCalled {
		t.Error("ProcessMessage() expected CreateIfNotExists to NOT be called when instance not found")
	}
}

func TestDispatchLogProcessor_InstanceRepoError(t *testing.T) {
	logger := zap.NewNop()
	dispatchLogRepo := &mockDispatchLogRepo{}
	instanceRepo := &stubInstanceRepo{
		err: errors.New("instance repo connection error"),
	}

	processor := NewDispatchLogProcessor(DispatchLogProcessorConfig{
		DispatchLogRepo:  dispatchLogRepo,
		InstanceRepo:     instanceRepo,
		Logger:           logger,
		DedupWindowHours: 168,
	})

	msg := &whatsapp.Message{
		ID:        "msg_repo_err",
		ChatID:    "group@g.us",
		FromMe:    false,
		Body:      "Hello world",
		Timestamp: time.Now(),
	}

	err := processor.ProcessMessage(context.Background(), "test_session", msg)
	if err != nil {
		t.Errorf("ProcessMessage() error = %v, want nil (should skip gracefully on instance repo error)", err)
	}

	if dispatchLogRepo.createIfNotExistsCalled {
		t.Error("ProcessMessage() expected CreateIfNotExists to NOT be called when instance repo returns error")
	}
}

func TestDispatchLogProcessor_CustomDedupWindowHours(t *testing.T) {
	logger := zap.NewNop()
	dispatchLogRepo := &mockDispatchLogRepo{}
	instanceRepo := &stubInstanceRepo{
		instance: &models.Instance{
			ID:         1,
			UserID:     "user123",
			InstanceID: "test_session",
		},
	}

	customHours := 48
	processor := NewDispatchLogProcessor(DispatchLogProcessorConfig{
		DispatchLogRepo:  dispatchLogRepo,
		InstanceRepo:     instanceRepo,
		Logger:           logger,
		DedupWindowHours: customHours,
	})

	msg := &whatsapp.Message{
		ID:        "msg_custom_dedup",
		ChatID:    "group@g.us",
		FromMe:    false,
		Body:      "https://mercadolivre.com.br/product",
		Timestamp: time.Now(),
	}

	err := processor.ProcessMessage(context.Background(), "test_session", msg)
	if err != nil {
		t.Errorf("ProcessMessage() error = %v", err)
	}

	if dispatchLogRepo.lastDedupWindow != customHours {
		t.Errorf("ProcessMessage() dedupWindowHours = %d, want %d", dispatchLogRepo.lastDedupWindow, customHours)
	}
}

func TestDispatchLogProcessor_DefaultDedupWindowHours(t *testing.T) {
	logger := zap.NewNop()

	tests := []struct {
		name        string
		inputHours  int
		wantDefault int
	}{
		{
			name:        "zero defaults to 168",
			inputHours:  0,
			wantDefault: DefaultReconciliationDedupWindowHours,
		},
		{
			name:        "negative defaults to 168",
			inputHours:  -1,
			wantDefault: DefaultReconciliationDedupWindowHours,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dispatchLogRepo := &mockDispatchLogRepo{}
			instanceRepo := &stubInstanceRepo{
				instance: &models.Instance{
					ID:         1,
					UserID:     "user123",
					InstanceID: "test_session",
				},
			}

			processor := NewDispatchLogProcessor(DispatchLogProcessorConfig{
				DispatchLogRepo:  dispatchLogRepo,
				InstanceRepo:     instanceRepo,
				Logger:           logger,
				DedupWindowHours: tt.inputHours,
			})

			msg := &whatsapp.Message{
				ID:        "msg_default_dedup",
				ChatID:    "group@g.us",
				FromMe:    false,
				Body:      "https://mercadolivre.com.br/product",
				Timestamp: time.Now(),
			}

			err := processor.ProcessMessage(context.Background(), "test_session", msg)
			if err != nil {
				t.Errorf("ProcessMessage() error = %v", err)
			}

			if dispatchLogRepo.lastDedupWindow != tt.wantDefault {
				t.Errorf("ProcessMessage() dedupWindowHours = %d, want %d", dispatchLogRepo.lastDedupWindow, tt.wantDefault)
			}
		})
	}
}

func TestDispatchLogProcessor_ExtractURL_EdgeCases(t *testing.T) {
	logger := zap.NewNop()
	processor := NewDispatchLogProcessor(DispatchLogProcessorConfig{
		DispatchLogRepo:  &mockDispatchLogRepo{},
		InstanceRepo:     &stubInstanceRepo{},
		Logger:           logger,
		DedupWindowHours: 168,
	})

	tests := []struct {
		name    string
		text    string
		pattern string
		wantURL string
	}{
		{
			name:    "URL at beginning of text",
			text:    "https://mercadolivre.com.br/product/123 check this",
			pattern: "mercadolivre.com",
			wantURL: "https://mercadolivre.com.br/product/123",
		},
		{
			name:    "URL at end of text",
			text:    "check this https://mercadolivre.com.br/product/123",
			pattern: "mercadolivre.com",
			wantURL: "https://mercadolivre.com.br/product/123",
		},
		{
			name:    "URL without http prefix adds https",
			text:    "go to mercadolivre.com.br/product/123 now",
			pattern: "mercadolivre.com",
			wantURL: "https://mercadolivre.com.br/product/123",
		},
		{
			name:    "pattern not found in text",
			text:    "just some random text",
			pattern: "mercadolivre.com",
			wantURL: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.extractURL(tt.text, tt.pattern)
			if result != tt.wantURL {
				t.Errorf("extractURL() = %q, want %q", result, tt.wantURL)
			}
		})
	}
}

func TestDispatchLogProcessor_ExtractProductURL_AllPatterns(t *testing.T) {
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
		wantURL string
	}{
		{
			name:    "kabum.com.br pattern",
			body:    "Oferta: https://kabum.com.br/produto/12345",
			wantURL: "https://kabum.com.br/produto/12345",
		},
		{
			name:    "casasbahia.com.br pattern",
			body:    "Promo: https://casasbahia.com.br/produto/67890",
			wantURL: "https://casasbahia.com.br/produto/67890",
		},
		{
			name:    "americanas.com.br pattern",
			body:    "Desconto: https://americanas.com.br/produto/11111",
			wantURL: "https://americanas.com.br/produto/11111",
		},
		{
			name:    "magalu.com pattern",
			body:    "Veja: https://magalu.com/produto/22222",
			wantURL: "https://magalu.com/produto/22222",
		},
		{
			name:    "magazineluiza.com.br pattern",
			body:    "Link: https://magazineluiza.com.br/produto/33333",
			wantURL: "https://magazineluiza.com.br/produto/33333",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.extractProductURL(tt.body)
			if result == nil {
				t.Errorf("extractProductURL() returned nil, want %q", tt.wantURL)
				return
			}
			if *result != tt.wantURL {
				t.Errorf("extractProductURL() = %q, want %q", *result, tt.wantURL)
			}
		})
	}
}

func TestDispatchLogProcessor_DedupWasCreatedFalse(t *testing.T) {
	logger := zap.NewNop()
	existingLog := &models.DispatchLog{
		ID:     42,
		UserID: "user123",
		Status: models.DispatchLogStatusReconciled,
	}
	dispatchLogRepo := &mockDispatchLogRepo{
		createResult: existingLog,
		wasCreated:   false,
	}
	instanceRepo := &stubInstanceRepo{
		instance: &models.Instance{
			ID:         1,
			UserID:     "user123",
			InstanceID: "test_session",
		},
	}

	processor := NewDispatchLogProcessor(DispatchLogProcessorConfig{
		DispatchLogRepo:  dispatchLogRepo,
		InstanceRepo:     instanceRepo,
		Logger:           logger,
		DedupWindowHours: 168,
	})

	msg := &whatsapp.Message{
		ID:        "msg_dedup",
		ChatID:    "group@g.us",
		FromMe:    false,
		Body:      "https://mercadolivre.com.br/product",
		Timestamp: time.Now(),
	}

	err := processor.ProcessMessage(context.Background(), "test_session", msg)
	if err != nil {
		t.Errorf("ProcessMessage() error = %v, want nil when dedup skips creation", err)
	}

	if !dispatchLogRepo.createIfNotExistsCalled {
		t.Error("ProcessMessage() expected CreateIfNotExists to be called even for dedup")
	}
}

func TestDispatchLogProcessor_MediaWithoutBody(t *testing.T) {
	logger := zap.NewNop()
	dispatchLogRepo := &mockDispatchLogRepo{}
	instanceRepo := &stubInstanceRepo{
		instance: &models.Instance{
			ID:         1,
			UserID:     "user123",
			InstanceID: "test_session",
		},
	}

	processor := NewDispatchLogProcessor(DispatchLogProcessorConfig{
		DispatchLogRepo:  dispatchLogRepo,
		InstanceRepo:     instanceRepo,
		Logger:           logger,
		DedupWindowHours: 168,
	})

	msg := &whatsapp.Message{
		ID:        "msg_media_no_body",
		ChatID:    "group@g.us",
		FromMe:    false,
		Body:      "",
		HasMedia:  true,
		MediaType: "video",
		Timestamp: time.Now(),
	}

	err := processor.ProcessMessage(context.Background(), "test_session", msg)
	if err != nil {
		t.Errorf("ProcessMessage() error = %v", err)
	}

	if !dispatchLogRepo.createIfNotExistsCalled {
		t.Error("ProcessMessage() expected CreateIfNotExists to be called for media message without body")
	}

	if dispatchLogRepo.lastLog == nil {
		t.Fatal("ProcessMessage() expected lastLog to be set")
	}

	if dispatchLogRepo.lastLog.Status != models.DispatchLogStatusReconciled {
		t.Errorf("ProcessMessage() status = %v, want %v", dispatchLogRepo.lastLog.Status, models.DispatchLogStatusReconciled)
	}

	if dispatchLogRepo.lastLog.ProductURL != nil {
		t.Errorf("ProcessMessage() productURL = %v, want nil for media without body", *dispatchLogRepo.lastLog.ProductURL)
	}
}

// =============================================================================
// Additional coverage: feature flag enabled, extractURL edge cases, amzn.to pattern
// =============================================================================

func TestDispatchLogProcessor_FeatureFlagEnabled(t *testing.T) {
	// Explicitly set the feature flag to true and verify processing occurs.
	os.Setenv("FEATURE_USE_GO_RECONCILIATION", "true")
	config.ResetFeatureFlags()
	defer func() {
		os.Unsetenv("FEATURE_USE_GO_RECONCILIATION")
		config.ResetFeatureFlags()
	}()

	logger := zap.NewNop()
	dispatchLogRepo := &mockDispatchLogRepo{}
	instanceRepo := &stubInstanceRepo{
		instance: &models.Instance{
			ID:         1,
			UserID:     "user123",
			InstanceID: "test_session",
		},
	}

	processor := NewDispatchLogProcessor(DispatchLogProcessorConfig{
		DispatchLogRepo:  dispatchLogRepo,
		InstanceRepo:     instanceRepo,
		Logger:           logger,
		DedupWindowHours: 168,
	})

	msg := &whatsapp.Message{
		ID:        "msg_flag_enabled",
		ChatID:    "group@g.us",
		FromMe:    false,
		Body:      "Hello world",
		Timestamp: time.Now(),
	}

	err := processor.ProcessMessage(context.Background(), "test_session", msg)
	if err != nil {
		t.Errorf("ProcessMessage() error = %v, want nil", err)
	}

	if !dispatchLogRepo.createIfNotExistsCalled {
		t.Error("ProcessMessage() expected CreateIfNotExists to be called when feature flag is enabled")
	}
}

func TestDispatchLogProcessor_ExtractURL_NewlineAndTabDelimiters(t *testing.T) {
	logger := zap.NewNop()
	processor := NewDispatchLogProcessor(DispatchLogProcessorConfig{
		DispatchLogRepo:  &mockDispatchLogRepo{},
		InstanceRepo:     &stubInstanceRepo{},
		Logger:           logger,
		DedupWindowHours: 168,
	})

	tests := []struct {
		name    string
		text    string
		pattern string
		wantURL string
	}{
		{
			name:    "URL delimited by newline after",
			text:    "check https://mercadolivre.com.br/product/123\nnext line",
			pattern: "mercadolivre.com",
			wantURL: "https://mercadolivre.com.br/product/123",
		},
		{
			name:    "URL delimited by tab after",
			text:    "check https://mercadolivre.com.br/product/123\tmore text",
			pattern: "mercadolivre.com",
			wantURL: "https://mercadolivre.com.br/product/123",
		},
		{
			name:    "URL preceded by newline",
			text:    "line one\nhttps://amazon.com/dp/B08N5WRWNW rest",
			pattern: "amazon.com",
			wantURL: "https://amazon.com/dp/B08N5WRWNW",
		},
		{
			name:    "URL preceded by tab",
			text:    "prefix\thttps://shopee.com.br/product/789 rest",
			pattern: "shopee.com",
			wantURL: "https://shopee.com.br/product/789",
		},
		{
			name:    "URL is entire text (no delimiters)",
			text:    "https://mercadolivre.com.br/product/999",
			pattern: "mercadolivre.com",
			wantURL: "https://mercadolivre.com.br/product/999",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.extractURL(tt.text, tt.pattern)
			if result != tt.wantURL {
				t.Errorf("extractURL() = %q, want %q", result, tt.wantURL)
			}
		})
	}
}

func TestDispatchLogProcessor_ExtractProductURL_AmznAndMercadolibrePatterns(t *testing.T) {
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
		wantURL string
	}{
		{
			name:    "amzn.to short URL",
			body:    "Check this deal: https://amzn.to/3xYzAbc more text",
			wantURL: "https://amzn.to/3xYzAbc",
		},
		{
			name:    "mercadolibre.com pattern",
			body:    "Oferta: https://mercadolibre.com/producto/MLM12345 veja",
			wantURL: "https://mercadolibre.com/producto/MLM12345",
		},
		{
			name:    "produto.mercadolivre.com.br pattern",
			body:    "Link: https://produto.mercadolivre.com.br/MLB-12345 compre",
			wantURL: "https://produto.mercadolivre.com.br/MLB-12345",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.extractProductURL(tt.body)
			if result == nil {
				t.Errorf("extractProductURL() returned nil, want %q", tt.wantURL)
				return
			}
			if *result != tt.wantURL {
				t.Errorf("extractProductURL() = %q, want %q", *result, tt.wantURL)
			}
		})
	}
}

func TestDispatchLogProcessor_ProcessMessage_MessageWithBodyAndMedia(t *testing.T) {
	// Test a message that has both body with a product URL and media flag set.
	// Verifies that body-based URL extraction works even when HasMedia is true.
	logger := zap.NewNop()
	dispatchLogRepo := &mockDispatchLogRepo{}
	instanceRepo := &stubInstanceRepo{
		instance: &models.Instance{
			ID:         1,
			UserID:     "user123",
			InstanceID: "test_session",
		},
	}

	processor := NewDispatchLogProcessor(DispatchLogProcessorConfig{
		DispatchLogRepo:  dispatchLogRepo,
		InstanceRepo:     instanceRepo,
		Logger:           logger,
		DedupWindowHours: 168,
	})

	msg := &whatsapp.Message{
		ID:        "msg_body_media",
		ChatID:    "group@g.us",
		FromMe:    false,
		Body:      "Check https://amazon.com/dp/B08N5WRWNW",
		HasMedia:  true,
		MediaType: "image",
		Timestamp: time.Now(),
	}

	err := processor.ProcessMessage(context.Background(), "test_session", msg)
	if err != nil {
		t.Errorf("ProcessMessage() error = %v", err)
	}

	if !dispatchLogRepo.createIfNotExistsCalled {
		t.Fatal("ProcessMessage() expected CreateIfNotExists to be called")
	}

	if dispatchLogRepo.lastLog.ProductURL == nil {
		t.Error("ProcessMessage() expected productURL to be extracted from body")
	} else if *dispatchLogRepo.lastLog.ProductURL != "https://amazon.com/dp/B08N5WRWNW" {
		t.Errorf("ProcessMessage() productURL = %q, want %q", *dispatchLogRepo.lastLog.ProductURL, "https://amazon.com/dp/B08N5WRWNW")
	}
}

func TestDispatchLogProcessor_GenerateCanonicalKey(t *testing.T) {
	// Verify canonical key format: "reconcile:{session}:{messageID}"
	logger := zap.NewNop()
	dispatchLogRepo := &mockDispatchLogRepo{}
	instanceRepo := &stubInstanceRepo{
		instance: &models.Instance{
			ID:         1,
			UserID:     "user123",
			InstanceID: "my_session",
		},
	}

	processor := NewDispatchLogProcessor(DispatchLogProcessorConfig{
		DispatchLogRepo:  dispatchLogRepo,
		InstanceRepo:     instanceRepo,
		Logger:           logger,
		DedupWindowHours: 168,
	})

	msg := &whatsapp.Message{
		ID:        "unique_msg_id_42",
		ChatID:    "group@g.us",
		FromMe:    false,
		Body:      "hello",
		Timestamp: time.Now(),
	}

	err := processor.ProcessMessage(context.Background(), "my_session", msg)
	if err != nil {
		t.Errorf("ProcessMessage() error = %v", err)
	}

	wantKey := "reconcile:my_session:unique_msg_id_42"
	if dispatchLogRepo.lastLog == nil || dispatchLogRepo.lastLog.CanonicalKey == nil {
		t.Fatal("ProcessMessage() expected canonicalKey to be set")
	}
	if *dispatchLogRepo.lastLog.CanonicalKey != wantKey {
		t.Errorf("canonicalKey = %q, want %q", *dispatchLogRepo.lastLog.CanonicalKey, wantKey)
	}
}

func TestDispatchLogProcessor_TimestampFormat(t *testing.T) {
	// Verify the dispatch log CreatedAt is formatted from the message timestamp.
	logger := zap.NewNop()
	dispatchLogRepo := &mockDispatchLogRepo{}
	instanceRepo := &stubInstanceRepo{
		instance: &models.Instance{
			ID:         1,
			UserID:     "user123",
			InstanceID: "test_session",
		},
	}

	processor := NewDispatchLogProcessor(DispatchLogProcessorConfig{
		DispatchLogRepo:  dispatchLogRepo,
		InstanceRepo:     instanceRepo,
		Logger:           logger,
		DedupWindowHours: 168,
	})

	fixedTime := time.Date(2026, 1, 15, 10, 30, 45, 0, time.UTC)
	msg := &whatsapp.Message{
		ID:        "msg_ts",
		ChatID:    "group@g.us",
		FromMe:    false,
		Body:      "test message",
		Timestamp: fixedTime,
	}

	err := processor.ProcessMessage(context.Background(), "test_session", msg)
	if err != nil {
		t.Errorf("ProcessMessage() error = %v", err)
	}

	if dispatchLogRepo.lastLog == nil {
		t.Fatal("ProcessMessage() expected lastLog to be set")
	}

	wantCreatedAt := "2026-01-15T10:30:45Z"
	if dispatchLogRepo.lastLog.CreatedAt != wantCreatedAt {
		t.Errorf("CreatedAt = %q, want %q", dispatchLogRepo.lastLog.CreatedAt, wantCreatedAt)
	}
}

func TestDispatchLogProcessor_LogEntryFields(t *testing.T) {
	// Verify all fields of the dispatch log entry are set correctly.
	logger := zap.NewNop()
	dispatchLogRepo := &mockDispatchLogRepo{}
	instanceRepo := &stubInstanceRepo{
		instance: &models.Instance{
			ID:         1,
			UserID:     "user456",
			InstanceID: "session_x",
		},
	}

	processor := NewDispatchLogProcessor(DispatchLogProcessorConfig{
		DispatchLogRepo:  dispatchLogRepo,
		InstanceRepo:     instanceRepo,
		Logger:           logger,
		DedupWindowHours: 72,
	})

	msg := &whatsapp.Message{
		ID:        "msg_fields",
		ChatID:    "group789@g.us",
		FromMe:    false,
		Body:      "No product URL here",
		Timestamp: time.Now(),
	}

	err := processor.ProcessMessage(context.Background(), "session_x", msg)
	if err != nil {
		t.Errorf("ProcessMessage() error = %v", err)
	}

	log := dispatchLogRepo.lastLog
	if log == nil {
		t.Fatal("expected lastLog to be set")
	}

	if log.UserID != "user456" {
		t.Errorf("UserID = %q, want %q", log.UserID, "user456")
	}
	if log.GroupID == nil || *log.GroupID != "group789@g.us" {
		t.Errorf("GroupID = %v, want %q", log.GroupID, "group789@g.us")
	}
	if log.Status != models.DispatchLogStatusReconciled {
		t.Errorf("Status = %q, want %q", log.Status, models.DispatchLogStatusReconciled)
	}
	if log.Type == nil || *log.Type != "reconciliation" {
		t.Errorf("Type = %v, want %q", log.Type, "reconciliation")
	}
	if log.ProductURL != nil {
		t.Errorf("ProductURL = %v, want nil (no e-commerce URL in body)", *log.ProductURL)
	}
	if dispatchLogRepo.lastDedupWindow != 72 {
		t.Errorf("dedupWindowHours = %d, want 72", dispatchLogRepo.lastDedupWindow)
	}
}
