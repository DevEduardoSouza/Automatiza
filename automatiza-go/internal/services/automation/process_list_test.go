package automation_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"automatiza-go/internal/models"
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
	deps.mlLinkListItemRepo.On("GetNextUnconsumed", ctx, listID, 1).Return(unconsumedItems, nil)
	deps.settingsRepo.On("GetActiveDispatchTextByID", ctx, mock.AnythingOfType("string"), "user-1").Return(nil, nil).Maybe()
	deps.mlService.On("GetProductPreview", ctx, "https://produto.mercadolivre.com.br/MLB-123456", "user-1").Return(productPreview, nil)
	deps.mlService.On("CreateAffiliateLink", ctx, "https://produto.mercadolivre.com.br/MLB-123456", "user-1").Return(affiliateLink, nil)
	deps.mlLinkListItemRepo.On("MarkAsConsumed", ctx, []string{"item-1"}, "auto-123", mock.AnythingOfType("*string")).Return(nil)
	deps.dispatchLogRepo.On("Create", ctx, mock.AnythingOfType("*models.DispatchLog")).Return(&models.DispatchLog{}, nil).Maybe()
	deps.automationRepo.On("UpdateLastRunAt", ctx, "auto-123").Return(nil)

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
	deps.mlLinkListItemRepo.On("GetNextUnconsumed", ctx, listID, 1).Return([]*models.MlLinkListItem{}, nil)
	deps.dispatchLogRepo.On("Create", ctx, mock.AnythingOfType("*models.DispatchLog")).Return(&models.DispatchLog{}, nil).Maybe()
	deps.automationRepo.On("UpdateLastRunAt", ctx, "auto-123").Return(nil)

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
	deps.automationRepo.On("UpdateLastRunAt", ctx, "auto-123").Return(nil)

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
	deps.automationRepo.On("UpdateLastRunAt", ctx, "auto-123").Return(nil)

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
	deps.automationRepo.On("UpdateLastRunAt", ctx, "auto-123").Return(nil)

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
	deps.automationRepo.On("UpdateLastRunAt", ctx, "auto-123").Return(nil)

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
	deps.automationRepo.On("UpdateLastRunAt", ctx, "auto-123").Return(nil)

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
	deps.automationRepo.On("UpdateLastRunAt", ctx, "auto-123").Return(nil)

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
	deps.mlLinkListItemRepo.On("GetNextUnconsumed", ctx, listID, 1).Return(unconsumedItems, nil)
	deps.mlService.On("GetProductPreview", ctx, "https://produto.mercadolivre.com.br/MLB-111111", "user-1").Return(productPreview, nil)
	deps.mlService.On("CreateAffiliateLink", ctx, "https://produto.mercadolivre.com.br/MLB-111111", "user-1").Return(affiliateLink, nil)
	deps.dispatchLogRepo.On("Create", ctx, mock.AnythingOfType("*models.DispatchLog")).Return(&models.DispatchLog{}, nil).Maybe()

	// This is the key assertion: verify MarkAsConsumed is called with correct item IDs
	deps.mlLinkListItemRepo.On("MarkAsConsumed", ctx, []string{"item-1"}, "auto-123", mock.AnythingOfType("*string")).Return(nil).Once()
	deps.automationRepo.On("UpdateLastRunAt", ctx, "auto-123").Return(nil)

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
	deps.mlLinkListItemRepo.On("GetNextUnconsumed", ctx, listID, 3).Return(unconsumedItems, nil)
	deps.dispatchLogRepo.On("Create", ctx, mock.AnythingOfType("*models.DispatchLog")).Return(&models.DispatchLog{}, nil).Maybe()

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
	deps.automationRepo.On("UpdateLastRunAt", ctx, "auto-123").Return(nil)

	result, err := svc.RunAutomation(ctx, "auto-123", "user-1")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success)
	assert.Equal(t, 3, result.OffersFound)
}

func TestProcessListMode_InvalidMLURL(t *testing.T) {
	// Test with URL that is not a Mercado Livre URL
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
	deps.mlLinkListItemRepo.On("GetNextUnconsumed", ctx, listID, 1).Return(unconsumedItems, nil)
	deps.dispatchLogRepo.On("Create", ctx, mock.AnythingOfType("*models.DispatchLog")).Return(&models.DispatchLog{}, nil).Maybe()
	deps.automationRepo.On("UpdateLastRunAt", ctx, "auto-123").Return(nil)

	result, err := svc.RunAutomation(ctx, "auto-123", "user-1")

	require.NoError(t, err)
	require.NotNil(t, result)
	// Should skip invalid URLs but still succeed
	assert.True(t, result.Success)
	assert.Equal(t, 0, result.MessagesSent)
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
	deps.mlLinkListItemRepo.On("GetNextUnconsumed", ctx, listID, 1).Return(unconsumedItems, nil)
	deps.dispatchLogRepo.On("Create", ctx, mock.AnythingOfType("*models.DispatchLog")).Return(&models.DispatchLog{}, nil).Maybe()

	// ML service returns error for preview
	deps.mlService.On("GetProductPreview", ctx, "https://produto.mercadolivre.com.br/MLB-123456", "user-1").Return(nil, errors.New("ML API error"))
	// Affiliate link still works but uses origin URL
	deps.mlService.On("CreateAffiliateLink", ctx, "https://produto.mercadolivre.com.br/MLB-123456", "user-1").Return(nil, errors.New("affiliate error"))
	deps.mlLinkListItemRepo.On("MarkAsConsumed", ctx, []string{"item-1"}, "auto-123", mock.AnythingOfType("*string")).Return(nil)
	deps.automationRepo.On("UpdateLastRunAt", ctx, "auto-123").Return(nil)

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
			deps.automationRepo.On("UpdateLastRunAt", ctx, "auto-123").Return(nil)

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
