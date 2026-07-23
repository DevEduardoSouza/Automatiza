package automation_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"
	"automatiza-go/internal/services/automation"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// =============================================================================
// ProcessListMode Tests
// =============================================================================

func TestProcessListMode_Success(t *testing.T) {
	// Items fetched, processed, marked consumed
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
		MlLinkListID: &listID,
		GroupIDs:     []string{"group1@g.us", "group2@g.us"},
		Stores:       []string{"mercadolivre"},
	}

	connectedInstances := []models.Instance{
		{ID: 1, InstanceID: "instance-1", UserID: "user-1", Status: "connected"},
	}

	unconsumedItems := []*models.MlLinkListItem{
		{
			ID:        "item-1",
			ListID:    listID,
			OriginURL: "https://produto.mercadolivre.com.br/MLB-123456",
			IsActive:  true,
		},
	}

	productPreview := &models.MLProductPreview{
		ID:    "MLB-123456",
		Title: "Test Product",
		Price: 99.90,
		Image: "https://image.url/product.jpg",
	}

	affiliateLink := &models.MLAffiliateLink{
		OriginalURL:   "https://produto.mercadolivre.com.br/MLB-123456",
		AffiliateLink: "https://affiliate.mercadolivre.com.br/xyz",
	}

	lockTime := time.Now().Add(10 * time.Minute)

	deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "auto-123", "user-1").Return(automation, nil)
	deps.automationRunRepo.On("GetLock", ctx, "auto-123").Return(nil, nil)
	deps.automationRunRepo.On("AcquireLock", ctx, "auto-123", mock.AnythingOfType("time.Duration")).Return(&lockTime, nil)
	deps.automationRunRepo.On("ReleaseLock", ctx, "auto-123").Return(nil)
	deps.historyRepo.On("Create", ctx, "user-1", "automation_run", mock.AnythingOfType("string"), mock.Anything).Return(&models.ActionHistory{ID: "history-1"}, nil)
	deps.historyRepo.On("Update", ctx, "history-1", mock.Anything).Return(nil)
	deps.instanceRepo.On("GetUserInstances", ctx, "user-1").Return(connectedInstances, nil)
	deps.mlLinkListItemRepo.On("GetNextUnconsumed", ctx, listID, 10).Return(unconsumedItems, nil)
	deps.settingsRepo.On("GetActiveDispatchTextByID", ctx, mock.AnythingOfType("string"), "user-1").Return(nil, nil).Maybe()
	deps.mlService.On("GetProductPreview", ctx, "https://produto.mercadolivre.com.br/MLB-123456", "user-1").Return(productPreview, nil)
	deps.mlService.On("CreateAffiliateLink", ctx, "https://produto.mercadolivre.com.br/MLB-123456", "user-1").Return(affiliateLink, nil)
	deps.mlLinkListItemRepo.On("MarkAsConsumed", ctx, []string{"item-1"}, "auto-123", mock.AnythingOfType("*string")).Return(nil)
	deps.dispatchLogRepo.On("Create", ctx, mock.AnythingOfType("*models.DispatchLog")).Return(&models.DispatchLog{}, nil).Maybe()
	deps.dispatchLogRepo.On("GetRecentByCanonicalKey", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil).Maybe()
	deps.automationRepo.On("UpdateLastRunAt", mock.Anything, "auto-123").Return(nil)

	result, err := svc.RunAutomation(ctx, "auto-123", "user-1")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success)
	assert.GreaterOrEqual(t, result.OffersFound, 1)
}

func TestProcessListMode_NoItems(t *testing.T) {
	// No unconsumed items left
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
		MlLinkListID: &listID,
		GroupIDs:     []string{"group1@g.us"},
		Stores:       []string{"mercadolivre"},
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
	deps.mlLinkListItemRepo.On("GetNextUnconsumed", ctx, listID, 10).Return([]*models.MlLinkListItem{}, nil)
	deps.dispatchLogRepo.On("Create", ctx, mock.AnythingOfType("*models.DispatchLog")).Return(&models.DispatchLog{}, nil).Maybe()
	deps.dispatchLogRepo.On("GetRecentByCanonicalKey", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil).Maybe()
	deps.automationRepo.On("UpdateLastRunAt", mock.Anything, "auto-123").Return(nil)

	result, err := svc.RunAutomation(ctx, "auto-123", "user-1")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success)
	assert.Equal(t, 0, result.OffersFound)
	assert.Contains(t, result.Message, "consumidos")
}

func TestProcessListMode_NoListID(t *testing.T) {
	// Error: no ml_link_list_id configured
	svc, deps := newTestService(t)
	ctx := context.Background()

	listType := "list"
	automation := &models.Automation{
		ID:           "auto-123",
		UserID:       "user-1",
		Name:         "List Automation",
		Status:       models.AutomationStatusActive,
		Type:         &listType,
		MlLinkListID: nil, // No list ID configured
		GroupIDs:     []string{"group1@g.us"},
		Stores:       []string{"mercadolivre"},
	}

	lockTime := time.Now().Add(10 * time.Minute)

	deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "auto-123", "user-1").Return(automation, nil)
	deps.automationRunRepo.On("GetLock", ctx, "auto-123").Return(nil, nil)
	deps.automationRunRepo.On("AcquireLock", ctx, "auto-123", mock.AnythingOfType("time.Duration")).Return(&lockTime, nil)
	deps.automationRunRepo.On("ReleaseLock", ctx, "auto-123").Return(nil)
	deps.historyRepo.On("Create", ctx, "user-1", "automation_run", mock.AnythingOfType("string"), mock.Anything).Return(&models.ActionHistory{ID: "history-1"}, nil)
	deps.historyRepo.On("Update", ctx, "history-1", mock.Anything).Return(nil)
	deps.dispatchLogRepo.On("Create", ctx, mock.AnythingOfType("*models.DispatchLog")).Return(&models.DispatchLog{}, nil).Maybe()
	deps.dispatchLogRepo.On("GetRecentByCanonicalKey", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil).Maybe()
	deps.automationRepo.On("UpdateLastRunAt", mock.Anything, "auto-123").Return(nil)

	result, err := svc.RunAutomation(ctx, "auto-123", "user-1")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Contains(t, result.Message, "Lista de Links")
}

func TestProcessListMode_EmptyListID(t *testing.T) {
	// Error: empty ml_link_list_id configured
	svc, deps := newTestService(t)
	ctx := context.Background()

	listType := "list"
	emptyListID := "   " // Whitespace only
	automation := &models.Automation{
		ID:           "auto-123",
		UserID:       "user-1",
		Name:         "List Automation",
		Status:       models.AutomationStatusActive,
		Type:         &listType,
		MlLinkListID: &emptyListID,
		GroupIDs:     []string{"group1@g.us"},
		Stores:       []string{"mercadolivre"},
	}

	lockTime := time.Now().Add(10 * time.Minute)

	deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "auto-123", "user-1").Return(automation, nil)
	deps.automationRunRepo.On("GetLock", ctx, "auto-123").Return(nil, nil)
	deps.automationRunRepo.On("AcquireLock", ctx, "auto-123", mock.AnythingOfType("time.Duration")).Return(&lockTime, nil)
	deps.automationRunRepo.On("ReleaseLock", ctx, "auto-123").Return(nil)
	deps.historyRepo.On("Create", ctx, "user-1", "automation_run", mock.AnythingOfType("string"), mock.Anything).Return(&models.ActionHistory{ID: "history-1"}, nil)
	deps.historyRepo.On("Update", ctx, "history-1", mock.Anything).Return(nil)
	deps.dispatchLogRepo.On("Create", ctx, mock.AnythingOfType("*models.DispatchLog")).Return(&models.DispatchLog{}, nil).Maybe()
	deps.dispatchLogRepo.On("GetRecentByCanonicalKey", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil).Maybe()
	deps.automationRepo.On("UpdateLastRunAt", mock.Anything, "auto-123").Return(nil)

	result, err := svc.RunAutomation(ctx, "auto-123", "user-1")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Contains(t, result.Message, "Lista de Links")
}

func TestProcessListMode_NoTargetGroups(t *testing.T) {
	// Error: no target groups configured
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
		MlLinkListID: &listID,
		GroupIDs:     []string{}, // No groups
		Stores:       []string{"mercadolivre"},
	}

	lockTime := time.Now().Add(10 * time.Minute)

	deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "auto-123", "user-1").Return(automation, nil)
	deps.automationRunRepo.On("GetLock", ctx, "auto-123").Return(nil, nil)
	deps.automationRunRepo.On("AcquireLock", ctx, "auto-123", mock.AnythingOfType("time.Duration")).Return(&lockTime, nil)
	deps.automationRunRepo.On("ReleaseLock", ctx, "auto-123").Return(nil)
	deps.historyRepo.On("Create", ctx, "user-1", "automation_run", mock.AnythingOfType("string"), mock.Anything).Return(&models.ActionHistory{ID: "history-1"}, nil)
	deps.historyRepo.On("Update", ctx, "history-1", mock.Anything).Return(nil)
	deps.dispatchLogRepo.On("Create", ctx, mock.AnythingOfType("*models.DispatchLog")).Return(&models.DispatchLog{}, nil).Maybe()
	deps.dispatchLogRepo.On("GetRecentByCanonicalKey", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil).Maybe()
	deps.automationRepo.On("UpdateLastRunAt", mock.Anything, "auto-123").Return(nil)

	result, err := svc.RunAutomation(ctx, "auto-123", "user-1")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Contains(t, result.Message, "grupo")
}

func TestProcessListMode_InvalidGroupIDs(t *testing.T) {
	// Error: groups don't have valid WhatsApp JIDs
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
		MlLinkListID: &listID,
		GroupIDs:     []string{"invalid-group", "another-invalid"}, // No @g.us suffix
		Stores:       []string{"mercadolivre"},
	}

	lockTime := time.Now().Add(10 * time.Minute)

	deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "auto-123", "user-1").Return(automation, nil)
	deps.automationRunRepo.On("GetLock", ctx, "auto-123").Return(nil, nil)
	deps.automationRunRepo.On("AcquireLock", ctx, "auto-123", mock.AnythingOfType("time.Duration")).Return(&lockTime, nil)
	deps.automationRunRepo.On("ReleaseLock", ctx, "auto-123").Return(nil)
	deps.historyRepo.On("Create", ctx, "user-1", "automation_run", mock.AnythingOfType("string"), mock.Anything).Return(&models.ActionHistory{ID: "history-1"}, nil)
	deps.historyRepo.On("Update", ctx, "history-1", mock.Anything).Return(nil)
	deps.dispatchLogRepo.On("Create", ctx, mock.AnythingOfType("*models.DispatchLog")).Return(&models.DispatchLog{}, nil).Maybe()
	deps.dispatchLogRepo.On("GetRecentByCanonicalKey", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil).Maybe()
	deps.automationRepo.On("UpdateLastRunAt", mock.Anything, "auto-123").Return(nil)

	result, err := svc.RunAutomation(ctx, "auto-123", "user-1")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Contains(t, result.Message, "@g.us")
}

func TestProcessListMode_InstanceNotConnected(t *testing.T) {
	// Error: WhatsApp not connected
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
		MlLinkListID: &listID,
		GroupIDs:     []string{"group1@g.us"},
		Stores:       []string{"mercadolivre"},
	}

	disconnectedInstances := []models.Instance{
		{ID: 1, InstanceID: "instance-1", UserID: "user-1", Status: "disconnected"},
	}

	lockTime := time.Now().Add(10 * time.Minute)

	deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "auto-123", "user-1").Return(automation, nil)
	deps.automationRunRepo.On("GetLock", ctx, "auto-123").Return(nil, nil)
	deps.automationRunRepo.On("AcquireLock", ctx, "auto-123", mock.AnythingOfType("time.Duration")).Return(&lockTime, nil)
	deps.automationRunRepo.On("ReleaseLock", ctx, "auto-123").Return(nil)
	deps.historyRepo.On("Create", ctx, "user-1", "automation_run", mock.AnythingOfType("string"), mock.Anything).Return(&models.ActionHistory{ID: "history-1"}, nil)
	deps.historyRepo.On("Update", ctx, "history-1", mock.Anything).Return(nil)
	deps.instanceRepo.On("GetUserInstances", ctx, "user-1").Return(disconnectedInstances, nil)
	deps.dispatchLogRepo.On("Create", ctx, mock.AnythingOfType("*models.DispatchLog")).Return(&models.DispatchLog{}, nil).Maybe()
	deps.dispatchLogRepo.On("GetRecentByCanonicalKey", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil).Maybe()
	deps.automationRepo.On("UpdateLastRunAt", mock.Anything, "auto-123").Return(nil)

	result, err := svc.RunAutomation(ctx, "auto-123", "user-1")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Contains(t, result.Message, "conectado")
}

func TestProcessListMode_NoInstancesFound(t *testing.T) {
	// Error: no WhatsApp instances found
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
		MlLinkListID: &listID,
		GroupIDs:     []string{"group1@g.us"},
		Stores:       []string{"mercadolivre"},
	}

	lockTime := time.Now().Add(10 * time.Minute)

	deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "auto-123", "user-1").Return(automation, nil)
	deps.automationRunRepo.On("GetLock", ctx, "auto-123").Return(nil, nil)
	deps.automationRunRepo.On("AcquireLock", ctx, "auto-123", mock.AnythingOfType("time.Duration")).Return(&lockTime, nil)
	deps.automationRunRepo.On("ReleaseLock", ctx, "auto-123").Return(nil)
	deps.historyRepo.On("Create", ctx, "user-1", "automation_run", mock.AnythingOfType("string"), mock.Anything).Return(&models.ActionHistory{ID: "history-1"}, nil)
	deps.historyRepo.On("Update", ctx, "history-1", mock.Anything).Return(nil)
	deps.instanceRepo.On("GetUserInstances", ctx, "user-1").Return(nil, errors.New("no instances"))
	deps.dispatchLogRepo.On("Create", ctx, mock.AnythingOfType("*models.DispatchLog")).Return(&models.DispatchLog{}, nil).Maybe()
	deps.dispatchLogRepo.On("GetRecentByCanonicalKey", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil).Maybe()
	deps.automationRepo.On("UpdateLastRunAt", mock.Anything, "auto-123").Return(nil)

	result, err := svc.RunAutomation(ctx, "auto-123", "user-1")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Success)
}

func TestProcessListMode_ItemsMarkedConsumed(t *testing.T) {
	// Verify items are marked as consumed after processing
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
		MlLinkListID: &listID,
		GroupIDs:     []string{"group1@g.us"},
		Stores:       []string{"mercadolivre"},
	}

	connectedInstances := []models.Instance{
		{ID: 1, InstanceID: "instance-1", UserID: "user-1", Status: "connected"},
	}

	unconsumedItems := []*models.MlLinkListItem{
		{
			ID:        "item-1",
			ListID:    listID,
			OriginURL: "https://produto.mercadolivre.com.br/MLB-111111",
			IsActive:  true,
		},
	}

	productPreview := &models.MLProductPreview{
		ID:    "MLB-111111",
		Title: "Test Product",
		Price: 49.90,
		Image: "https://image.url/product.jpg",
	}

	affiliateLink := &models.MLAffiliateLink{
		OriginalURL:   "https://produto.mercadolivre.com.br/MLB-111111",
		AffiliateLink: "https://affiliate.mercadolivre.com.br/abc",
	}

	lockTime := time.Now().Add(10 * time.Minute)

	deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "auto-123", "user-1").Return(automation, nil)
	deps.automationRunRepo.On("GetLock", ctx, "auto-123").Return(nil, nil)
	deps.automationRunRepo.On("AcquireLock", ctx, "auto-123", mock.AnythingOfType("time.Duration")).Return(&lockTime, nil)
	deps.automationRunRepo.On("ReleaseLock", ctx, "auto-123").Return(nil)
	deps.historyRepo.On("Create", ctx, "user-1", "automation_run", mock.AnythingOfType("string"), mock.Anything).Return(&models.ActionHistory{ID: "history-1"}, nil)
	deps.historyRepo.On("Update", ctx, "history-1", mock.Anything).Return(nil)
	deps.instanceRepo.On("GetUserInstances", ctx, "user-1").Return(connectedInstances, nil)
	deps.mlLinkListItemRepo.On("GetNextUnconsumed", ctx, listID, 10).Return(unconsumedItems, nil)
	deps.mlService.On("GetProductPreview", ctx, "https://produto.mercadolivre.com.br/MLB-111111", "user-1").Return(productPreview, nil)
	deps.mlService.On("CreateAffiliateLink", ctx, "https://produto.mercadolivre.com.br/MLB-111111", "user-1").Return(affiliateLink, nil)
	deps.dispatchLogRepo.On("Create", ctx, mock.AnythingOfType("*models.DispatchLog")).Return(&models.DispatchLog{}, nil).Maybe()
	deps.dispatchLogRepo.On("GetRecentByCanonicalKey", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil).Maybe()

	// This is the key assertion: verify MarkAsConsumed is called with correct item IDs
	deps.mlLinkListItemRepo.On("MarkAsConsumed", ctx, []string{"item-1"}, "auto-123", mock.AnythingOfType("*string")).Return(nil).Once()
	deps.automationRepo.On("UpdateLastRunAt", mock.Anything, "auto-123").Return(nil)

	result, err := svc.RunAutomation(ctx, "auto-123", "user-1")

	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify MarkAsConsumed was called
	deps.mlLinkListItemRepo.AssertCalled(t, "MarkAsConsumed", ctx, []string{"item-1"}, "auto-123", mock.AnythingOfType("*string"))
}

func TestProcessListMode_MultipleItemsPerRun(t *testing.T) {
	// Test with multiple items per run configured
	svc, deps := newTestService(t)
	ctx := context.Background()

	listType := "list"
	listID := "list-123"
	itemsPerRun := 3
	automation := &models.Automation{
		ID:              "auto-123",
		UserID:          "user-1",
		Name:            "List Automation",
		Status:          models.AutomationStatusActive,
		Type:            &listType,
		MlLinkListID:    &listID,
		ListItemsPerRun: &itemsPerRun,
		GroupIDs:        []string{"group1@g.us"},
		Stores:          []string{"mercadolivre"},
	}

	connectedInstances := []models.Instance{
		{ID: 1, InstanceID: "instance-1", UserID: "user-1", Status: "connected"},
	}

	unconsumedItems := []*models.MlLinkListItem{
		{ID: "item-1", ListID: listID, OriginURL: "https://produto.mercadolivre.com.br/MLB-111", IsActive: true},
		{ID: "item-2", ListID: listID, OriginURL: "https://produto.mercadolivre.com.br/MLB-222", IsActive: true},
		{ID: "item-3", ListID: listID, OriginURL: "https://produto.mercadolivre.com.br/MLB-333", IsActive: true},
	}

	lockTime := time.Now().Add(10 * time.Minute)

	deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "auto-123", "user-1").Return(automation, nil)
	deps.automationRunRepo.On("GetLock", ctx, "auto-123").Return(nil, nil)
	deps.automationRunRepo.On("AcquireLock", ctx, "auto-123", mock.AnythingOfType("time.Duration")).Return(&lockTime, nil)
	deps.automationRunRepo.On("ReleaseLock", ctx, "auto-123").Return(nil)
	deps.historyRepo.On("Create", ctx, "user-1", "automation_run", mock.AnythingOfType("string"), mock.Anything).Return(&models.ActionHistory{ID: "history-1"}, nil)
	deps.historyRepo.On("Update", ctx, "history-1", mock.Anything).Return(nil)
	deps.instanceRepo.On("GetUserInstances", ctx, "user-1").Return(connectedInstances, nil)
	deps.mlLinkListItemRepo.On("GetNextUnconsumed", ctx, listID, 12).Return(unconsumedItems, nil)
	deps.dispatchLogRepo.On("Create", ctx, mock.AnythingOfType("*models.DispatchLog")).Return(&models.DispatchLog{}, nil).Maybe()
	deps.dispatchLogRepo.On("GetRecentByCanonicalKey", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil).Maybe()

	// Mock ML service calls for each item
	for _, item := range unconsumedItems {
		deps.mlService.On("GetProductPreview", ctx, item.OriginURL, "user-1").Return(&models.MLProductPreview{
			ID:    "MLB-ID",
			Title: "Product",
			Price: 99.90,
		}, nil)
		deps.mlService.On("CreateAffiliateLink", ctx, item.OriginURL, "user-1").Return(&models.MLAffiliateLink{
			AffiliateLink: "https://affiliate.link",
		}, nil)
		deps.mlLinkListItemRepo.On("MarkAsConsumed", ctx, []string{item.ID}, "auto-123", mock.AnythingOfType("*string")).Return(nil)
	}
	deps.automationRepo.On("UpdateLastRunAt", mock.Anything, "auto-123").Return(nil)

	result, err := svc.RunAutomation(ctx, "auto-123", "user-1")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success)
	assert.Equal(t, 3, result.OffersFound)
}

func TestProcessListMode_NonMLURL_Passthrough(t *testing.T) {
	// URL de loja não-convertida (Amazon etc.) é PASSTHROUGH: o usuário colou a
	// própria URL afiliada e o sistema apenas despacha (ver comentário em
	// process_list.go — só Shopee é convertida pelo backend).
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
		MlLinkListID: &listID,
		GroupIDs:     []string{"group1@g.us"},
		Stores:       []string{"mercadolivre"},
	}

	connectedInstances := []models.Instance{
		{ID: 1, InstanceID: "instance-1", UserID: "user-1", Status: "connected"},
	}

	// Item with non-ML URL
	unconsumedItems := []*models.MlLinkListItem{
		{
			ID:        "item-1",
			ListID:    listID,
			OriginURL: "https://www.amazon.com.br/product/123",
			IsActive:  true,
		},
	}

	lockTime := time.Now().Add(10 * time.Minute)

	deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "auto-123", "user-1").Return(automation, nil)
	deps.automationRunRepo.On("GetLock", ctx, "auto-123").Return(nil, nil)
	deps.automationRunRepo.On("AcquireLock", ctx, "auto-123", mock.AnythingOfType("time.Duration")).Return(&lockTime, nil)
	deps.automationRunRepo.On("ReleaseLock", ctx, "auto-123").Return(nil)
	deps.historyRepo.On("Create", ctx, "user-1", "automation_run", mock.AnythingOfType("string"), mock.Anything).Return(&models.ActionHistory{ID: "history-1"}, nil)
	deps.historyRepo.On("Update", ctx, "history-1", mock.Anything).Return(nil)
	deps.instanceRepo.On("GetUserInstances", ctx, "user-1").Return(connectedInstances, nil)
	deps.mlLinkListItemRepo.On("GetNextUnconsumed", ctx, listID, 10).Return(unconsumedItems, nil)
	deps.mlLinkListItemRepo.On("MarkAsConsumed", mock.Anything, []string{"item-1"}, "auto-123", mock.Anything).Return(nil)
	deps.dispatchLogRepo.On("Create", ctx, mock.AnythingOfType("*models.DispatchLog")).Return(&models.DispatchLog{}, nil).Maybe()
	deps.dispatchLogRepo.On("GetRecentByCanonicalKey", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil).Maybe()
	deps.automationRepo.On("UpdateLastRunAt", mock.Anything, "auto-123").Return(nil)

	result, err := svc.RunAutomation(ctx, "auto-123", "user-1")

	require.NoError(t, err)
	require.NotNil(t, result)
	// Passthrough: a URL do usuário é despachada como está e o item consumido.
	assert.True(t, result.Success)
	assert.Equal(t, 1, result.MessagesSent)
	deps.mlLinkListItemRepo.AssertCalled(t, "MarkAsConsumed", mock.Anything, []string{"item-1"}, "auto-123", mock.Anything)
}

func TestProcessListMode_MLServiceError(t *testing.T) {
	// Test handling of ML service errors gracefully
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
		MlLinkListID: &listID,
		GroupIDs:     []string{"group1@g.us"},
		Stores:       []string{"mercadolivre"},
	}

	connectedInstances := []models.Instance{
		{ID: 1, InstanceID: "instance-1", UserID: "user-1", Status: "connected"},
	}

	unconsumedItems := []*models.MlLinkListItem{
		{
			ID:        "item-1",
			ListID:    listID,
			OriginURL: "https://produto.mercadolivre.com.br/MLB-123456",
			IsActive:  true,
		},
	}

	lockTime := time.Now().Add(10 * time.Minute)

	deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "auto-123", "user-1").Return(automation, nil)
	deps.automationRunRepo.On("GetLock", ctx, "auto-123").Return(nil, nil)
	deps.automationRunRepo.On("AcquireLock", ctx, "auto-123", mock.AnythingOfType("time.Duration")).Return(&lockTime, nil)
	deps.automationRunRepo.On("ReleaseLock", ctx, "auto-123").Return(nil)
	deps.historyRepo.On("Create", ctx, "user-1", "automation_run", mock.AnythingOfType("string"), mock.Anything).Return(&models.ActionHistory{ID: "history-1"}, nil)
	deps.historyRepo.On("Update", ctx, "history-1", mock.Anything).Return(nil)
	deps.instanceRepo.On("GetUserInstances", ctx, "user-1").Return(connectedInstances, nil)
	deps.mlLinkListItemRepo.On("GetNextUnconsumed", ctx, listID, 10).Return(unconsumedItems, nil)
	deps.dispatchLogRepo.On("Create", ctx, mock.AnythingOfType("*models.DispatchLog")).Return(&models.DispatchLog{}, nil).Maybe()
	deps.dispatchLogRepo.On("GetRecentByCanonicalKey", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil).Maybe()

	// ML service returns error for preview
	deps.mlService.On("GetProductPreview", ctx, "https://produto.mercadolivre.com.br/MLB-123456", "user-1").Return(nil, errors.New("ML API error"))
	// Affiliate link still works but uses origin URL
	deps.mlService.On("CreateAffiliateLink", ctx, "https://produto.mercadolivre.com.br/MLB-123456", "user-1").Return(nil, errors.New("affiliate error"))
	deps.mlLinkListItemRepo.On("MarkAsConsumed", ctx, []string{"item-1"}, "auto-123", mock.AnythingOfType("*string")).Return(nil)
	deps.automationRepo.On("UpdateLastRunAt", mock.Anything, "auto-123").Return(nil)

	result, err := svc.RunAutomation(ctx, "auto-123", "user-1")

	require.NoError(t, err)
	require.NotNil(t, result)
	// Should still succeed even with ML service errors (uses fallback)
	assert.True(t, result.Success)
}

// =============================================================================
// Table-Driven Tests for LIST Mode Edge Cases
// =============================================================================

func TestProcessListMode_EdgeCases(t *testing.T) {
	tests := []struct {
		name           string
		listID         *string
		groupIDs       []string
		instanceStatus string
		wantSuccess    bool
		wantMsgContain string
	}{
		{
			name:           "nil list ID",
			listID:         nil,
			groupIDs:       []string{"group1@g.us"},
			instanceStatus: "connected",
			wantSuccess:    false,
			wantMsgContain: "Lista",
		},
		{
			name:           "empty list ID",
			listID:         ptr(""),
			groupIDs:       []string{"group1@g.us"},
			instanceStatus: "connected",
			wantSuccess:    false,
			wantMsgContain: "Lista",
		},
		{
			name:           "no groups",
			listID:         ptr("list-123"),
			groupIDs:       []string{},
			instanceStatus: "connected",
			wantSuccess:    false,
			wantMsgContain: "grupo",
		},
		{
			name:           "invalid group format",
			listID:         ptr("list-123"),
			groupIDs:       []string{"invalid-group-id"},
			instanceStatus: "connected",
			wantSuccess:    false,
			wantMsgContain: "@g.us",
		},
		{
			name:           "disconnected instance",
			listID:         ptr("list-123"),
			groupIDs:       []string{"group1@g.us"},
			instanceStatus: "disconnected",
			wantSuccess:    false,
			wantMsgContain: "conectado",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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

			setupDefaultHistoryExpectations(deps.historyRepo)

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
				nil, // mlLinkListRepo
				nil, // ctaRepo
				deps.shopeeService,
				deps.mlService,
				nil, nil, nil, nil,
				nil, // couponRepo
				nil, // couponRuleRepo
				logger,
			)

			ctx := context.Background()
			listType := "list"
			auto := &models.Automation{
				ID:           "auto-123",
				UserID:       "user-1",
				Name:         "Test Automation",
				Status:       models.AutomationStatusActive,
				Type:         &listType,
				MlLinkListID: tt.listID,
				GroupIDs:     tt.groupIDs,
			}

			lockTime := time.Now().Add(10 * time.Minute)

			deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "auto-123", "user-1").Return(auto, nil)
			deps.automationRunRepo.On("GetLock", ctx, "auto-123").Return(nil, nil)
			deps.automationRunRepo.On("AcquireLock", ctx, "auto-123", mock.AnythingOfType("time.Duration")).Return(&lockTime, nil)
			deps.automationRunRepo.On("ReleaseLock", ctx, "auto-123").Return(nil)
			deps.historyRepo.On("Create", ctx, "user-1", "automation_run", mock.AnythingOfType("string"), mock.Anything).Return(&models.ActionHistory{ID: "h-1"}, nil)
			deps.historyRepo.On("Update", ctx, "h-1", mock.Anything).Return(nil)
			deps.dispatchLogRepo.On("Create", ctx, mock.AnythingOfType("*models.DispatchLog")).Return(&models.DispatchLog{}, nil).Maybe()
			deps.dispatchLogRepo.On("GetRecentByCanonicalKey", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil).Maybe()

			if tt.listID != nil && *tt.listID != "" && len(tt.groupIDs) > 0 {
				// Only check groups if we have valid list ID
				hasValidGroup := false
				for _, g := range tt.groupIDs {
					if len(g) > 5 && g[len(g)-5:] == "@g.us" {
						hasValidGroup = true
						break
					}
				}
				if hasValidGroup {
					deps.instanceRepo.On("GetUserInstances", ctx, "user-1").Return([]models.Instance{
						{ID: 1, InstanceID: "instance-1", Status: tt.instanceStatus},
					}, nil)
				}
			}
			deps.automationRepo.On("UpdateLastRunAt", mock.Anything, "auto-123").Return(nil)

			result, err := svc.RunAutomation(ctx, "auto-123", "user-1")

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, tt.wantSuccess, result.Success)
			if tt.wantMsgContain != "" {
				assert.Contains(t, result.Message, tt.wantMsgContain)
			}
		})
	}
}

// setupShopeeDegradedListTest wires an automation with a single Shopee list item
// whose enrichment fails, returning the service + deps so each test can plug in
// the specific failure error. Shared by the transient and permanent cases below.
func setupShopeeDegradedListTest(t *testing.T) (automation.Service, *testServiceDeps, string) {
	t.Helper()
	svc, deps := newTestService(t)

	listType := "list"
	listID := "list-123"
	shopeeURL := "https://s.shopee.com.br/8fPcxpGjSs"
	auto := &models.Automation{
		ID:           "auto-123",
		UserID:       "user-1",
		Name:         "List Shopee",
		Status:       models.AutomationStatusActive,
		Type:         &listType,
		MlLinkListID: &listID,
		GroupIDs:     []string{"group1@g.us"},
		Stores:       []string{"shopee"},
	}
	connected := []models.Instance{{ID: 1, InstanceID: "instance-1", UserID: "user-1", Status: "connected"}}
	items := []*models.MlLinkListItem{
		{ID: "item-1", ListID: listID, OriginURL: shopeeURL, IsActive: true},
	}
	lockTime := time.Now().Add(10 * time.Minute)

	deps.automationRepo.On("GetAutomationByIDAndUser", mock.Anything, "auto-123", "user-1").Return(auto, nil)
	deps.automationRunRepo.On("GetLock", mock.Anything, "auto-123").Return(nil, nil)
	deps.automationRunRepo.On("AcquireLock", mock.Anything, "auto-123", mock.AnythingOfType("time.Duration")).Return(&lockTime, nil)
	deps.automationRunRepo.On("ReleaseLock", mock.Anything, "auto-123").Return(nil)
	deps.historyRepo.On("Create", mock.Anything, "user-1", "automation_run", mock.AnythingOfType("string"), mock.Anything).Return(&models.ActionHistory{ID: "history-1"}, nil)
	deps.historyRepo.On("Update", mock.Anything, "history-1", mock.Anything).Return(nil)
	deps.instanceRepo.On("GetUserInstances", mock.Anything, "user-1").Return(connected, nil)
	deps.mlLinkListItemRepo.On("GetNextUnconsumed", mock.Anything, listID, 10).Return(items, nil)
	deps.settingsRepo.On("GetActiveDispatchTextByID", mock.Anything, mock.AnythingOfType("string"), "user-1").Return(nil, nil).Maybe()
	deps.automationRepo.On("UpdateLastRunAt", mock.Anything, "auto-123").Return(nil)
	deps.dispatchLogRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.DispatchLog")).Return(&models.DispatchLog{}, nil).Maybe()
	deps.dispatchLogRepo.On("GetRecentByCanonicalKey", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil).Maybe()

	return svc, deps, shopeeURL
}

// Regressão dupla:
//  1. (isaa_deoliveira@hotmail.com, 04/06 13:52) o enriquecimento Shopee degradado
//     NÃO deve enviar a oferta quebrada (texto cru, sem imagem e sem preço).
//  2. (conta "110 a 120", 10/07 ~21:52) uma falha TRANSIENTE do Shopee
//     (rate limit 10030 / timeout / 5xx) NÃO deve consumir o item. Consumir
//     avança GetNextUnconsumed sobre um link nunca entregue, drenando a lista
//     inteira a zero envios durante a janela de rate-limit (a automação "roda
//     sem entregar"). O item fica pendente e é re-tentado no próximo ciclo.
func TestProcessListMode_ShopeeEnrichTransient_RetriesAndKeepsItem(t *testing.T) {
	svc, deps, shopeeURL := setupShopeeDegradedListTest(t)
	ctx := context.Background()

	// Enriquecimento Shopee falha com erro TRANSIENTE em TODAS as tentativas.
	deps.shopeeService.On("GenerateAffiliateLink", mock.Anything, shopeeURL, "user-1").
		Return((*models.ShopeeAffiliateLink)(nil), errors.New("failed to expand short URL: context deadline exceeded"))

	result, err := svc.RunAutomation(ctx, "auto-123", "user-1")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 0, result.MessagesSent, "oferta degradada (sem preco/imagem) nao deve ser enviada")
	// Retry: GenerateAffiliateLink chamado 2x.
	deps.shopeeService.AssertNumberOfCalls(t, "GenerateAffiliateLink", 2)
	// NÃO consome em falha transiente — item preservado para retry no proximo ciclo.
	deps.mlLinkListItemRepo.AssertNotCalled(t, "MarkAsConsumed", mock.Anything, []string{"item-1"}, mock.Anything, mock.Anything)
}

// Complemento: numa falha PERMANENTE não-credencial (produto inexistente, URL
// inválida) o item AINDA é consumido, senão GetNextUnconsumed devolveria o mesmo
// link morto todo ciclo e travaria a lista nele — a razão original do skip consumir.
func TestProcessListMode_ShopeeEnrichPermanent_ConsumesItem(t *testing.T) {
	svc, deps, shopeeURL := setupShopeeDegradedListTest(t)
	ctx := context.Background()

	// Erro PERMANENTE não-credencial: retryar para sempre nunca resolveria.
	deps.shopeeService.On("GenerateAffiliateLink", mock.Anything, shopeeURL, "user-1").
		Return((*models.ShopeeAffiliateLink)(nil), errors.New("invalid shopee URL: no item id"))
	// Skip degradado consome o item pra lista avançar.
	deps.mlLinkListItemRepo.On("MarkAsConsumed", mock.Anything, []string{"item-1"}, "auto-123", mock.Anything).Return(nil)

	result, err := svc.RunAutomation(ctx, "auto-123", "user-1")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 0, result.MessagesSent, "oferta degradada (sem preco/imagem) nao deve ser enviada")
	// Item consumido p/ a lista nao travar num link morto.
	deps.mlLinkListItemRepo.AssertCalled(t, "MarkAsConsumed", mock.Anything, []string{"item-1"}, "auto-123", mock.Anything)
}

// Regressão (isaa_deoliveira@hotmail.com, 11/07): erro de CREDENCIAL não pode
// queimar itens da lista — a automação PAUSA (status paused + last_error) e o
// item fica pendente para quando a credencial for recadastrada.
func TestProcessListMode_ShopeeEnrichCredential_PausesWithoutConsuming(t *testing.T) {
	svc, deps, shopeeURL := setupShopeeDegradedListTest(t)
	ctx := context.Background()

	deps.shopeeService.On("GenerateAffiliateLink", mock.Anything, shopeeURL, "user-1").
		Return((*models.ShopeeAffiliateLink)(nil), errors.New("shopee credentials not configured"))
	deps.automationRepo.On("UpdateAutomation", mock.Anything, "auto-123", mock.MatchedBy(func(u map[string]interface{}) bool {
		return u["status"] == "paused"
	})).Return(nil)

	result, err := svc.RunAutomation(ctx, "auto-123", "user-1")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 0, result.MessagesSent)
	// Automação pausada com erro de credencial.
	deps.automationRepo.AssertCalled(t, "UpdateAutomation", mock.Anything, "auto-123", mock.Anything)
	// Item NÃO consumido — produto preservado para depois do recadastro.
	deps.mlLinkListItemRepo.AssertNotCalled(t, "MarkAsConsumed", mock.Anything, []string{"item-1"}, mock.Anything, mock.Anything)
}

// =============================================================================
// Melhoria 6 — CTA estritamente opt-in: prova de consulta EVITADA
// =============================================================================

// countingCTARepo prova a parte que o teste de builder não alcança: que o
// repositório sequer é CONSULTADO quando o template não pede CTA. Embute a
// interface de propósito — qualquer método de CRUD chamado a partir do
// dispatch estoura com nil pointer, que é exatamente o sinal desejado.
type countingCTARepo struct {
	repository.CTARepository
	pool      []models.CTA
	err       error
	userCalls int
	idCalls   int
}

func (c *countingCTARepo) GetUserCTAs(_ context.Context, _ string, _ map[string]interface{}) ([]models.CTA, error) {
	c.userCalls++
	if c.err != nil {
		return nil, c.err
	}
	return c.pool, nil
}

func (c *countingCTARepo) GetCTAsByIDs(_ context.Context, _ []string, _ string) ([]models.CTA, error) {
	c.idCalls++
	if c.err != nil {
		return nil, c.err
	}
	return c.pool, nil
}

func (c *countingCTARepo) assertCalls(t *testing.T, wantUser, wantID int) {
	t.Helper()
	if c.userCalls != wantUser {
		t.Errorf("GetUserCTAs chamado %d vez(es), want %d", c.userCalls, wantUser)
	}
	if c.idCalls != wantID {
		t.Errorf("GetCTAsByIDs chamado %d vez(es), want %d", c.idCalls, wantID)
	}
}

func ctaPoolShopee() []models.CTA {
	return []models.CTA{{Message: "CTA_POOL_SHOPEE", Platform: models.CTAPlatformShopee, IsActive: true}}
}

// newTestServiceWithCTARepo espelha newTestService, plugando um ctaRepo real
// (contador) no lugar do nil — a única diferença.
func newTestServiceWithCTARepo(t *testing.T, ctaRepo repository.CTARepository) (automation.Service, *testServiceDeps) {
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
	setupDefaultHistoryExpectations(deps.historyRepo)

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
		nil, // mlLinkListRepo
		ctaRepo,
		deps.shopeeService,
		deps.mlService,
		nil, // urlSafetyService
		nil, // backpressure
		nil, // queueClient
		nil, // cache
		nil, // couponRepo
		nil, // couponRuleRepo
		logger,
	)
	return svc, deps
}

// runSearchWithDispatchBody roda uma automação de BUSCA completa com o corpo de
// dispatch text informado, devolvendo o resultado. ctaIDs != nil exercita o
// ramo GetCTAsByIDs; inline alimenta o fallback automation.CTAs.
func runSearchWithDispatchBody(t *testing.T, ctaRepo repository.CTARepository, body string, ctaIDs, inline []string) *models.RunAutomationResult {
	t.Helper()
	svc, deps := newTestServiceWithCTARepo(t, ctaRepo)
	ctx := context.Background()

	searchType := "search"
	keyword := "smartphone"
	dispatchTextID := "dispatch-123"
	auto := &models.Automation{
		ID:             "auto-123",
		UserID:         "user-1",
		Name:           "Search CTA",
		Status:         models.AutomationStatusActive,
		Type:           &searchType,
		Keyword:        &keyword,
		GroupIDs:       []string{"group1@g.us"},
		Stores:         []string{"shopee"},
		DispatchTextID: &dispatchTextID,
		CTAIDs:         ctaIDs,
		CTAs:           inline,
	}

	offers := &models.ShopeeSearchResult{
		Offers: []models.ShopeeOffer{{
			ItemID: 12345, ShopID: 67890,
			Name: "Smartphone XYZ", Price: 999.90, OriginalPrice: 1299.90, Discount: 23,
			AffiliateLink: "https://affiliate.shopee.com/xyz",
			ProductURL:    "https://shopee.com.br/product/67890/12345",
		}},
		Total: 1,
	}
	lockTime := time.Now().Add(10 * time.Minute)

	deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "auto-123", "user-1").Return(auto, nil)
	deps.automationRunRepo.On("GetLock", ctx, "auto-123").Return(nil, nil)
	deps.automationRunRepo.On("AcquireLock", ctx, "auto-123", mock.AnythingOfType("time.Duration")).Return(&lockTime, nil)
	deps.automationRunRepo.On("ReleaseLock", mock.Anything, "auto-123").Return(nil)
	deps.automationRepo.On("UpdateNextRunAt", mock.Anything, "auto-123", mock.Anything).Return(nil).Maybe()
	deps.historyRepo.On("Create", ctx, "user-1", "automation_run", mock.AnythingOfType("string"), mock.Anything).Return(&models.ActionHistory{ID: "history-1"}, nil)
	deps.historyRepo.On("Update", ctx, "history-1", mock.Anything).Return(nil)
	deps.instanceRepo.On("GetUserInstances", ctx, "user-1").Return([]models.Instance{
		{ID: 1, InstanceID: "instance-1", UserID: "user-1", Status: "connected"},
	}, nil)
	deps.shopeeService.On("SearchOffers", ctx, "user-1", mock.AnythingOfType("models.ShopeeSearchParams")).Return(offers, nil)
	deps.shopeeService.On("GetProduct", ctx, int64(12345), int64(67890), "user-1").Return(nil, errors.New("not found")).Maybe()
	deps.shopeeService.On("GenerateAffiliateLink", ctx, "https://shopee.com.br/product/67890/12345", "user-1").Return(
		&models.ShopeeAffiliateLink{AffiliateLink: "https://s.shopee.com.br/tracked-link"}, nil).Maybe()
	deps.settingsRepo.On("GetActiveDispatchTextByID", ctx, "dispatch-123", "user-1").Return(
		&models.DispatchText{ID: "dispatch-123", UserID: "user-1", Body: body, IsActive: true}, nil)
	deps.dispatchLogRepo.On("GetRecentByCanonicalKey", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil).Maybe()
	deps.dispatchLogRepo.On("Create", ctx, mock.AnythingOfType("*models.DispatchLog")).Return(&models.DispatchLog{}, nil).Maybe()
	deps.automationRepo.On("UpdateLastRunAt", mock.Anything, "auto-123").Return(nil)

	result, err := svc.RunAutomation(ctx, "auto-123", "user-1")
	require.NoError(t, err)
	require.NotNil(t, result)
	return result
}

// Busca: com o texto de disparo SEM {cta}, nem o pool global nem os CTAIDs
// podem ser consultados — nem que a conta tenha CTA cadastrada e inline.
func TestProcessSearchMode_CTAOptIn_NoVariableNoQuery(t *testing.T) {
	repo := &countingCTARepo{pool: ctaPoolShopee()}
	runSearchWithDispatchBody(t, repo,
		"OFERTA: {{PRODUTO_NOME}} - {{LINK_AFILIADO}}",
		[]string{"cta-1"}, []string{"CTA_INLINE"})
	repo.assertCalls(t, 0, 0)
}

// Busca com {cta} e sem CTAIDs: exatamente UMA consulta ao pool ativo.
func TestProcessSearchMode_CTAOptIn_PoolQueriedOnce(t *testing.T) {
	repo := &countingCTARepo{pool: ctaPoolShopee()}
	runSearchWithDispatchBody(t, repo, "OFERTA: {{PRODUTO_NOME}}\n{cta}", nil, nil)
	repo.assertCalls(t, 1, 0)
}

// Busca com {cta} e CTAIDs preenchido: só o ramo por IDs, exatamente uma vez.
func TestProcessSearchMode_CTAOptIn_IDsQueriedOnce(t *testing.T) {
	repo := &countingCTARepo{pool: ctaPoolShopee()}
	runSearchWithDispatchBody(t, repo, "OFERTA: {{PRODUTO_NOME}}\n{cta}", []string{"cta-1"}, nil)
	repo.assertCalls(t, 0, 1)
}

// Repositório de CTA em erro não pode impedir o envio nem virar retry por
// grupo: uma tentativa, fail-open, run bem-sucedido.
func TestProcessSearchMode_CTAOptIn_RepoErrorIsFailOpen(t *testing.T) {
	repo := &countingCTARepo{err: errors.New("db down")}
	result := runSearchWithDispatchBody(t, repo, "OFERTA: {{PRODUTO_NOME}}\n{cta}", nil, []string{"CTA_INLINE"})
	repo.assertCalls(t, 1, 0)
	assert.True(t, result.Success, "erro de CTA não pode derrubar o envio")
}

// Run que não constrói oferta nenhuma (busca vazia) não consulta CTA, mesmo
// com {cta} no template: o gate é avaliado por mensagem, não por automação.
func TestProcessSearchMode_CTAOptIn_NoOffersNoQuery(t *testing.T) {
	repo := &countingCTARepo{pool: ctaPoolShopee()}
	svc, deps := newTestServiceWithCTARepo(t, repo)
	ctx := context.Background()

	searchType := "search"
	keyword := "smartphone"
	dispatchTextID := "dispatch-123"
	auto := &models.Automation{
		ID: "auto-123", UserID: "user-1", Name: "Search CTA",
		Status: models.AutomationStatusActive, Type: &searchType, Keyword: &keyword,
		GroupIDs: []string{"group1@g.us"}, Stores: []string{"shopee"},
		DispatchTextID: &dispatchTextID,
	}
	lockTime := time.Now().Add(10 * time.Minute)

	deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "auto-123", "user-1").Return(auto, nil)
	deps.automationRunRepo.On("GetLock", ctx, "auto-123").Return(nil, nil)
	deps.automationRunRepo.On("AcquireLock", ctx, "auto-123", mock.AnythingOfType("time.Duration")).Return(&lockTime, nil)
	deps.automationRunRepo.On("ReleaseLock", mock.Anything, "auto-123").Return(nil)
	deps.automationRepo.On("UpdateNextRunAt", mock.Anything, "auto-123", mock.Anything).Return(nil).Maybe()
	deps.historyRepo.On("Create", ctx, "user-1", "automation_run", mock.AnythingOfType("string"), mock.Anything).Return(&models.ActionHistory{ID: "history-1"}, nil)
	deps.historyRepo.On("Update", ctx, "history-1", mock.Anything).Return(nil)
	deps.instanceRepo.On("GetUserInstances", ctx, "user-1").Return([]models.Instance{
		{ID: 1, InstanceID: "instance-1", UserID: "user-1", Status: "connected"},
	}, nil)
	deps.shopeeService.On("SearchOffers", ctx, "user-1", mock.AnythingOfType("models.ShopeeSearchParams")).Return(
		&models.ShopeeSearchResult{Offers: nil}, nil)
	deps.settingsRepo.On("GetActiveDispatchTextByID", ctx, "dispatch-123", "user-1").Return(
		&models.DispatchText{ID: "dispatch-123", UserID: "user-1", Body: "{{PRODUTO_NOME}}\n{cta}", IsActive: true}, nil)
	deps.dispatchLogRepo.On("Create", ctx, mock.AnythingOfType("*models.DispatchLog")).Return(&models.DispatchLog{}, nil).Maybe()
	deps.automationRepo.On("UpdateLastRunAt", mock.Anything, "auto-123").Return(nil)

	_, err := svc.RunAutomation(ctx, "auto-123", "user-1")
	require.NoError(t, err)
	repo.assertCalls(t, 0, 0)
}

// runListWithDispatchBody roda uma automação de LISTA com N itens ML
// passthrough (o caminho que não depende da API Shopee) e o corpo informado.
func runListWithDispatchBody(t *testing.T, ctaRepo repository.CTARepository, body string, ctaIDs []string, itemCount int) {
	t.Helper()
	svc, deps := newTestServiceWithCTARepo(t, ctaRepo)
	ctx := context.Background()

	listType := "list"
	listID := "list-123"
	dispatchTextID := "dispatch-123"
	auto := &models.Automation{
		ID: "auto-123", UserID: "user-1", Name: "List CTA",
		Status: models.AutomationStatusActive, Type: &listType,
		MlLinkListID: &listID, GroupIDs: []string{"group1@g.us"},
		Stores: []string{"mercadolivre"}, DispatchTextID: &dispatchTextID,
		CTAIDs: ctaIDs, ListItemsPerRun: &itemCount,
	}

	items := make([]*models.MlLinkListItem, 0, itemCount)
	for i := 0; i < itemCount; i++ {
		title := "Produto Lista"
		price := 99.90
		items = append(items, &models.MlLinkListItem{
			ID:           "item-" + string(rune('a'+i)),
			OriginURL:    "https://produto.mercadolivre.com.br/MLB-" + string(rune('a'+i)),
			ProductTitle: &title,
			ProductPrice: &price,
		})
	}

	lockTime := time.Now().Add(10 * time.Minute)
	deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "auto-123", "user-1").Return(auto, nil)
	deps.automationRunRepo.On("GetLock", ctx, "auto-123").Return(nil, nil)
	deps.automationRunRepo.On("AcquireLock", ctx, "auto-123", mock.AnythingOfType("time.Duration")).Return(&lockTime, nil)
	deps.automationRunRepo.On("ReleaseLock", mock.Anything, "auto-123").Return(nil)
	deps.automationRepo.On("UpdateNextRunAt", mock.Anything, "auto-123", mock.Anything).Return(nil).Maybe()
	deps.historyRepo.On("Create", ctx, "user-1", "automation_run", mock.AnythingOfType("string"), mock.Anything).Return(&models.ActionHistory{ID: "history-1"}, nil)
	deps.historyRepo.On("Update", ctx, "history-1", mock.Anything).Return(nil)
	deps.instanceRepo.On("GetUserInstances", ctx, "user-1").Return([]models.Instance{
		{ID: 1, InstanceID: "instance-1", UserID: "user-1", Status: "connected"},
	}, nil)
	deps.mlLinkListItemRepo.On("GetNextUnconsumed", mock.Anything, "list-123", mock.AnythingOfType("int")).Return(items, nil)
	deps.mlLinkListItemRepo.On("MarkAsConsumed", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	deps.mlService.On("GetProductPreview", mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("preview off")).Maybe()
	deps.settingsRepo.On("GetActiveDispatchTextByID", ctx, "dispatch-123", "user-1").Return(
		&models.DispatchText{ID: "dispatch-123", UserID: "user-1", Body: body, IsActive: true}, nil)
	deps.dispatchLogRepo.On("GetRecentByCanonicalKey", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil).Maybe()
	deps.dispatchLogRepo.On("Create", ctx, mock.AnythingOfType("*models.DispatchLog")).Return(&models.DispatchLog{}, nil).Maybe()
	deps.automationRepo.On("UpdateLastRunAt", mock.Anything, "auto-123").Return(nil)

	_, err := svc.RunAutomation(ctx, "auto-123", "user-1")
	require.NoError(t, err)
}

// Lista: texto de disparo sem {cta} → zero consulta, com CTAIDs configurado.
func TestProcessListMode_CTAOptIn_NoVariableNoQuery(t *testing.T) {
	repo := &countingCTARepo{pool: ctaPoolShopee()}
	runListWithDispatchBody(t, repo, "{nome_produto} - {link_afiliado}", []string{"cta-1"}, 1)
	repo.assertCalls(t, 0, 0)
}

// Lista com {cta}: UMA consulta ao pool ativo por run.
func TestProcessListMode_CTAOptIn_PoolQueriedOnce(t *testing.T) {
	repo := &countingCTARepo{pool: ctaPoolShopee()}
	runListWithDispatchBody(t, repo, "{nome_produto}\n{cta}", nil, 1)
	repo.assertCalls(t, 1, 0)
}

// Lista com {cta} e CTAIDs: só o ramo por IDs, uma vez.
func TestProcessListMode_CTAOptIn_IDsQueriedOnce(t *testing.T) {
	repo := &countingCTARepo{pool: ctaPoolShopee()}
	runListWithDispatchBody(t, repo, "{nome_produto}\n{cta}", []string{"cta-1"}, 1)
	repo.assertCalls(t, 0, 1)
}

// Vários itens no MESMO run com {cta}: o pool é carregado uma vez só — a
// seleção continua por mensagem, a consulta não.
func TestProcessListMode_CTAOptIn_PoolLoadedOncePerRun(t *testing.T) {
	repo := &countingCTARepo{pool: ctaPoolShopee()}
	runListWithDispatchBody(t, repo, "{nome_produto}\n{cta}", nil, 3)
	repo.assertCalls(t, 1, 0)
}

// A garantia mais frágil do loader lazy é o caminho de FALHA: sem marcar a
// tentativa antes da chamada, cada item repetiria a query que já deu erro.
// Três itens, repositório em erro → ainda uma consulta só.
func TestProcessListMode_CTAOptIn_RepoErrorNotRetriedPerItem(t *testing.T) {
	repo := &countingCTARepo{err: errors.New("db down")}
	runListWithDispatchBody(t, repo, "{nome_produto}\n{cta}", nil, 3)
	repo.assertCalls(t, 1, 0)
}

// Mesma prova no ramo por IDs (CTAIDs preenchido).
func TestProcessListMode_CTAOptIn_IDsErrorNotRetriedPerItem(t *testing.T) {
	repo := &countingCTARepo{err: errors.New("db down")}
	runListWithDispatchBody(t, repo, "{nome_produto}\n{cta}", []string{"cta-1"}, 3)
	repo.assertCalls(t, 0, 1)
}
