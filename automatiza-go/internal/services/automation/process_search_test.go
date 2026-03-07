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
// ProcessSearchMode Tests
// =============================================================================

func TestProcessSearchMode_Success(t *testing.T) {
	// Offers found, messages sent
	svc, deps := newTestService(t)
	ctx := context.Background()

	searchType := "search"
	keyword := "smartphone"
	automation := &models.Automation{
		ID:           "auto-123",
		UserID:       "user-1",
		Name:         "Search Automation",
		Status:       models.AutomationStatusActive,
		Type:         &searchType,
		Keyword:      &keyword,
		GroupIDs:     []string{"group1@g.us", "group2@g.us"},
		Stores:       []string{"shopee"},
		IncludeImage: true,
	}

	connectedInstances := []models.Instance{
		{ID: 1, InstanceID: "instance-1", UserID: "user-1", Status: "connected"},
	}

	shopeeOffers := &models.ShopeeSearchResult{
		Offers: []models.ShopeeOffer{
			{
				ItemID:        12345,
				ShopID:        67890,
				Name:          "Smartphone XYZ",
				Price:         999.90,
				OriginalPrice: 1299.90,
				Discount:      23,
				Image:         "https://image.url/product.jpg",
				AffiliateLink: "https://affiliate.shopee.com/xyz",
				ProductURL:    "https://shopee.com.br/product/67890/12345",
			},
		},
		Total:   1,
		HasMore: false,
	}

	lockTime := time.Now().Add(10 * time.Minute)

	deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "auto-123", "user-1").Return(automation, nil)
	deps.automationRunRepo.On("GetLock", ctx, "auto-123").Return(nil, nil)
	deps.automationRunRepo.On("AcquireLock", ctx, "auto-123", mock.AnythingOfType("time.Duration")).Return(&lockTime, nil)
	deps.automationRunRepo.On("ReleaseLock", ctx, "auto-123").Return(nil)
	deps.historyRepo.On("Create", ctx, "user-1", "automation_run", mock.AnythingOfType("string"), mock.Anything).Return(&models.ActionHistory{ID: "history-1"}, nil)
	deps.historyRepo.On("Update", ctx, "history-1", mock.Anything).Return(nil)
	deps.instanceRepo.On("GetUserInstances", ctx, "user-1").Return(connectedInstances, nil)
	deps.shopeeService.On("SearchOffers", ctx, "user-1", mock.AnythingOfType("models.ShopeeSearchParams")).Return(shopeeOffers, nil)
	// Price refetch before sending (GetProduct call per offer)
	deps.shopeeService.On("GetProduct", ctx, int64(12345), int64(67890), "user-1").Return(&models.ShopeeOffer{
		ItemID:        12345,
		ShopID:        67890,
		Name:          "Smartphone XYZ",
		Price:         979.90,
		OriginalPrice: 1299.90,
	}, nil).Maybe()
	// Affiliate link generation
	deps.shopeeService.On("GenerateAffiliateLink", ctx, "https://shopee.com.br/product/67890/12345", "user-1").Return(&models.ShopeeAffiliateLink{
		AffiliateLink: "https://s.shopee.com.br/tracked-link",
	}, nil).Maybe()
	deps.dispatchLogRepo.On("GetRecentByCanonicalKey", ctx, "user-1", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("int")).Return(nil, nil).Maybe()
	deps.dispatchLogRepo.On("Create", ctx, mock.AnythingOfType("*models.DispatchLog")).Return(&models.DispatchLog{}, nil).Maybe()
	deps.automationRepo.On("UpdateLastRunAt", ctx, "auto-123").Return(nil)

	result, err := svc.RunAutomation(ctx, "auto-123", "user-1")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success)
}

func TestProcessSearchMode_NoOffers(t *testing.T) {
	// No offers found
	svc, deps := newTestService(t)
	ctx := context.Background()

	searchType := "search"
	keyword := "produto-inexistente-xyz"
	automation := &models.Automation{
		ID:       "auto-123",
		UserID:   "user-1",
		Name:     "Search Automation",
		Status:   models.AutomationStatusActive,
		Type:     &searchType,
		Keyword:  &keyword,
		GroupIDs: []string{"group1@g.us"},
		Stores:   []string{"shopee"},
	}

	connectedInstances := []models.Instance{
		{ID: 1, InstanceID: "instance-1", UserID: "user-1", Status: "connected"},
	}

	emptyResult := &models.ShopeeSearchResult{
		Offers:  []models.ShopeeOffer{},
		Total:   0,
		HasMore: false,
	}

	lockTime := time.Now().Add(10 * time.Minute)

	deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "auto-123", "user-1").Return(automation, nil)
	deps.automationRunRepo.On("GetLock", ctx, "auto-123").Return(nil, nil)
	deps.automationRunRepo.On("AcquireLock", ctx, "auto-123", mock.AnythingOfType("time.Duration")).Return(&lockTime, nil)
	deps.automationRunRepo.On("ReleaseLock", ctx, "auto-123").Return(nil)
	deps.historyRepo.On("Create", ctx, "user-1", "automation_run", mock.AnythingOfType("string"), mock.Anything).Return(&models.ActionHistory{ID: "history-1"}, nil)
	deps.historyRepo.On("Update", ctx, "history-1", mock.Anything).Return(nil)
	deps.instanceRepo.On("GetUserInstances", ctx, "user-1").Return(connectedInstances, nil)
	deps.shopeeService.On("SearchOffers", ctx, "user-1", mock.AnythingOfType("models.ShopeeSearchParams")).Return(emptyResult, nil)
	deps.automationRepo.On("UpdateLastRunAt", ctx, "auto-123").Return(nil)

	result, err := svc.RunAutomation(ctx, "auto-123", "user-1")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success)
	assert.Equal(t, 0, result.OffersFound)
	assert.Contains(t, result.Message, "Nenhuma oferta")
}

func TestProcessSearchMode_DedupeFilters(t *testing.T) {
	// Some offers filtered by dedupe
	svc, deps := newTestService(t)
	ctx := context.Background()

	searchType := "search"
	keyword := "smartphone"
	dedupeHours := 24
	automation := &models.Automation{
		ID:          "auto-123",
		UserID:      "user-1",
		Name:        "Search Automation",
		Status:      models.AutomationStatusActive,
		Type:        &searchType,
		Keyword:     &keyword,
		GroupIDs:    []string{"group1@g.us"},
		Stores:      []string{"shopee"},
		DedupeHours: &dedupeHours,
	}

	connectedInstances := []models.Instance{
		{ID: 1, InstanceID: "instance-1", UserID: "user-1", Status: "connected"},
	}

	shopeeOffers := &models.ShopeeSearchResult{
		Offers: []models.ShopeeOffer{
			{
				ItemID:        12345,
				ShopID:        67890,
				Name:          "Already Sent Product",
				Price:         499.90,
				OriginalPrice: 699.90,
				Discount:      29,
				AffiliateLink: "https://affiliate.shopee.com/xyz",
				ProductURL:    "https://shopee.com.br/product/67890/12345",
			},
		},
		Total:   1,
		HasMore: false,
	}

	// This dispatch log indicates the offer was already sent
	existingLog := &models.DispatchLog{
		ID:        1,
		UserID:    "user-1",
		Status:    "sent",
		CreatedAt: time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
	}

	lockTime := time.Now().Add(10 * time.Minute)

	deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "auto-123", "user-1").Return(automation, nil)
	deps.automationRunRepo.On("GetLock", ctx, "auto-123").Return(nil, nil)
	deps.automationRunRepo.On("AcquireLock", ctx, "auto-123", mock.AnythingOfType("time.Duration")).Return(&lockTime, nil)
	deps.automationRunRepo.On("ReleaseLock", ctx, "auto-123").Return(nil)
	deps.historyRepo.On("Create", ctx, "user-1", "automation_run", mock.AnythingOfType("string"), mock.Anything).Return(&models.ActionHistory{ID: "history-1"}, nil)
	deps.historyRepo.On("Update", ctx, "history-1", mock.Anything).Return(nil)
	deps.instanceRepo.On("GetUserInstances", ctx, "user-1").Return(connectedInstances, nil)
	deps.shopeeService.On("SearchOffers", ctx, "user-1", mock.AnythingOfType("models.ShopeeSearchParams")).Return(shopeeOffers, nil)
	// Price refetch before sending
	deps.shopeeService.On("GetProduct", ctx, int64(12345), int64(67890), "user-1").Return(nil, errors.New("not found")).Maybe()
	// Return existing log to simulate dedupe
	deps.dispatchLogRepo.On("GetRecentByCanonicalKey", ctx, "user-1", "group1@g.us", mock.AnythingOfType("string"), 24).Return(existingLog, nil)
	deps.dispatchLogRepo.On("Create", ctx, mock.AnythingOfType("*models.DispatchLog")).Return(&models.DispatchLog{}, nil).Maybe()
	deps.automationRepo.On("UpdateLastRunAt", ctx, "auto-123").Return(nil)

	result, err := svc.RunAutomation(ctx, "auto-123", "user-1")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success)
	// The message was skipped due to dedupe
	assert.Contains(t, result.Message, "ignoradas")
}

func TestProcessSearchMode_NoTargetGroups(t *testing.T) {
	// Error: no target groups configured
	svc, deps := newTestService(t)
	ctx := context.Background()

	searchType := "search"
	keyword := "smartphone"
	automation := &models.Automation{
		ID:       "auto-123",
		UserID:   "user-1",
		Name:     "Search Automation",
		Status:   models.AutomationStatusActive,
		Type:     &searchType,
		Keyword:  &keyword,
		GroupIDs: []string{}, // No groups
		Stores:   []string{"shopee"},
	}

	lockTime := time.Now().Add(10 * time.Minute)

	deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "auto-123", "user-1").Return(automation, nil)
	deps.automationRunRepo.On("GetLock", ctx, "auto-123").Return(nil, nil)
	deps.automationRunRepo.On("AcquireLock", ctx, "auto-123", mock.AnythingOfType("time.Duration")).Return(&lockTime, nil)
	deps.automationRunRepo.On("ReleaseLock", ctx, "auto-123").Return(nil)
	deps.historyRepo.On("Create", ctx, "user-1", "automation_run", mock.AnythingOfType("string"), mock.Anything).Return(&models.ActionHistory{ID: "history-1"}, nil)
	deps.historyRepo.On("Update", ctx, "history-1", mock.Anything).Return(nil)
	deps.automationRepo.On("UpdateLastRunAt", ctx, "auto-123").Return(nil)

	result, err := svc.RunAutomation(ctx, "auto-123", "user-1")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Contains(t, result.Message, "target groups")
}

func TestProcessSearchMode_InstanceNotConnected(t *testing.T) {
	// Error: WhatsApp not connected
	svc, deps := newTestService(t)
	ctx := context.Background()

	searchType := "search"
	keyword := "smartphone"
	automation := &models.Automation{
		ID:       "auto-123",
		UserID:   "user-1",
		Name:     "Search Automation",
		Status:   models.AutomationStatusActive,
		Type:     &searchType,
		Keyword:  &keyword,
		GroupIDs: []string{"group1@g.us"},
		Stores:   []string{"shopee"},
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
	deps.automationRepo.On("UpdateLastRunAt", ctx, "auto-123").Return(nil)

	result, err := svc.RunAutomation(ctx, "auto-123", "user-1")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Contains(t, result.Message, "connected")
}

func TestProcessSearchMode_NoInstances(t *testing.T) {
	// Error: no WhatsApp instances found
	svc, deps := newTestService(t)
	ctx := context.Background()

	searchType := "search"
	keyword := "smartphone"
	automation := &models.Automation{
		ID:       "auto-123",
		UserID:   "user-1",
		Name:     "Search Automation",
		Status:   models.AutomationStatusActive,
		Type:     &searchType,
		Keyword:  &keyword,
		GroupIDs: []string{"group1@g.us"},
		Stores:   []string{"shopee"},
	}

	lockTime := time.Now().Add(10 * time.Minute)

	deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "auto-123", "user-1").Return(automation, nil)
	deps.automationRunRepo.On("GetLock", ctx, "auto-123").Return(nil, nil)
	deps.automationRunRepo.On("AcquireLock", ctx, "auto-123", mock.AnythingOfType("time.Duration")).Return(&lockTime, nil)
	deps.automationRunRepo.On("ReleaseLock", ctx, "auto-123").Return(nil)
	deps.historyRepo.On("Create", ctx, "user-1", "automation_run", mock.AnythingOfType("string"), mock.Anything).Return(&models.ActionHistory{ID: "history-1"}, nil)
	deps.historyRepo.On("Update", ctx, "history-1", mock.Anything).Return(nil)
	deps.instanceRepo.On("GetUserInstances", ctx, "user-1").Return([]models.Instance{}, nil)
	deps.automationRepo.On("UpdateLastRunAt", ctx, "auto-123").Return(nil)

	result, err := svc.RunAutomation(ctx, "auto-123", "user-1")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Contains(t, result.Message, "instance")
}

func TestProcessSearchMode_ShopeeServiceError(t *testing.T) {
	// Shopee service returns error
	svc, deps := newTestService(t)
	ctx := context.Background()

	searchType := "search"
	keyword := "smartphone"
	automation := &models.Automation{
		ID:       "auto-123",
		UserID:   "user-1",
		Name:     "Search Automation",
		Status:   models.AutomationStatusActive,
		Type:     &searchType,
		Keyword:  &keyword,
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
	deps.shopeeService.On("SearchOffers", ctx, "user-1", mock.AnythingOfType("models.ShopeeSearchParams")).Return(nil, errors.New("shopee API error"))
	deps.automationRepo.On("UpdateLastRunAt", ctx, "auto-123").Return(nil)

	result, err := svc.RunAutomation(ctx, "auto-123", "user-1")

	require.NoError(t, err)
	require.NotNil(t, result)
	// Should return success with 0 offers since error is handled gracefully
	assert.True(t, result.Success)
	assert.Equal(t, 0, result.OffersFound)
}

func TestProcessSearchMode_DefaultsToShopee(t *testing.T) {
	// When no stores configured, defaults to shopee
	svc, deps := newTestService(t)
	ctx := context.Background()

	searchType := "search"
	keyword := "smartphone"
	automation := &models.Automation{
		ID:       "auto-123",
		UserID:   "user-1",
		Name:     "Search Automation",
		Status:   models.AutomationStatusActive,
		Type:     &searchType,
		Keyword:  &keyword,
		GroupIDs: []string{"group1@g.us"},
		Stores:   []string{}, // Empty - should default to shopee
	}

	connectedInstances := []models.Instance{
		{ID: 1, InstanceID: "instance-1", UserID: "user-1", Status: "connected"},
	}

	emptyResult := &models.ShopeeSearchResult{
		Offers:  []models.ShopeeOffer{},
		Total:   0,
		HasMore: false,
	}

	lockTime := time.Now().Add(10 * time.Minute)

	deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "auto-123", "user-1").Return(automation, nil)
	deps.automationRunRepo.On("GetLock", ctx, "auto-123").Return(nil, nil)
	deps.automationRunRepo.On("AcquireLock", ctx, "auto-123", mock.AnythingOfType("time.Duration")).Return(&lockTime, nil)
	deps.automationRunRepo.On("ReleaseLock", ctx, "auto-123").Return(nil)
	deps.historyRepo.On("Create", ctx, "user-1", "automation_run", mock.AnythingOfType("string"), mock.Anything).Return(&models.ActionHistory{ID: "history-1"}, nil)
	deps.historyRepo.On("Update", ctx, "history-1", mock.Anything).Return(nil)
	deps.instanceRepo.On("GetUserInstances", ctx, "user-1").Return(connectedInstances, nil)
	// Should still call shopee service
	deps.shopeeService.On("SearchOffers", ctx, "user-1", mock.AnythingOfType("models.ShopeeSearchParams")).Return(emptyResult, nil)
	deps.automationRepo.On("UpdateLastRunAt", ctx, "auto-123").Return(nil)

	result, err := svc.RunAutomation(ctx, "auto-123", "user-1")

	require.NoError(t, err)
	require.NotNil(t, result)
	// Verify shopee was called
	deps.shopeeService.AssertCalled(t, "SearchOffers", ctx, "user-1", mock.AnythingOfType("models.ShopeeSearchParams"))
}

func TestProcessSearchMode_WithPriceFilters(t *testing.T) {
	// Search with price filters
	svc, deps := newTestService(t)
	ctx := context.Background()

	searchType := "search"
	keyword := "smartphone"
	minPrice := 500.0
	maxPrice := 1500.0
	automation := &models.Automation{
		ID:       "auto-123",
		UserID:   "user-1",
		Name:     "Search Automation",
		Status:   models.AutomationStatusActive,
		Type:     &searchType,
		Keyword:  &keyword,
		MinPrice: &minPrice,
		MaxPrice: &maxPrice,
		GroupIDs: []string{"group1@g.us"},
		Stores:   []string{"shopee"},
	}

	connectedInstances := []models.Instance{
		{ID: 1, InstanceID: "instance-1", UserID: "user-1", Status: "connected"},
	}

	shopeeOffers := &models.ShopeeSearchResult{
		Offers: []models.ShopeeOffer{
			{
				ItemID:        12345,
				ShopID:        67890,
				Name:          "Smartphone in Range",
				Price:         999.90,
				OriginalPrice: 1299.90,
				Discount:      23,
				AffiliateLink: "https://affiliate.shopee.com/xyz",
				ProductURL:    "https://shopee.com.br/product/67890/12345",
			},
		},
		Total:   1,
		HasMore: false,
	}

	lockTime := time.Now().Add(10 * time.Minute)

	deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "auto-123", "user-1").Return(automation, nil)
	deps.automationRunRepo.On("GetLock", ctx, "auto-123").Return(nil, nil)
	deps.automationRunRepo.On("AcquireLock", ctx, "auto-123", mock.AnythingOfType("time.Duration")).Return(&lockTime, nil)
	deps.automationRunRepo.On("ReleaseLock", ctx, "auto-123").Return(nil)
	deps.historyRepo.On("Create", ctx, "user-1", "automation_run", mock.AnythingOfType("string"), mock.Anything).Return(&models.ActionHistory{ID: "history-1"}, nil)
	deps.historyRepo.On("Update", ctx, "history-1", mock.Anything).Return(nil)
	deps.instanceRepo.On("GetUserInstances", ctx, "user-1").Return(connectedInstances, nil)
	deps.shopeeService.On("SearchOffers", ctx, "user-1", mock.AnythingOfType("models.ShopeeSearchParams")).Return(shopeeOffers, nil)
	// Price refetch before sending
	deps.shopeeService.On("GetProduct", ctx, int64(12345), int64(67890), "user-1").Return(nil, errors.New("not found")).Maybe()
	// Affiliate link generation
	deps.shopeeService.On("GenerateAffiliateLink", ctx, "https://shopee.com.br/product/67890/12345", "user-1").Return(&models.ShopeeAffiliateLink{
		AffiliateLink: "https://s.shopee.com.br/tracked-link",
	}, nil).Maybe()
	deps.dispatchLogRepo.On("GetRecentByCanonicalKey", ctx, "user-1", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("int")).Return(nil, nil).Maybe()
	deps.dispatchLogRepo.On("Create", ctx, mock.AnythingOfType("*models.DispatchLog")).Return(&models.DispatchLog{}, nil).Maybe()
	deps.automationRepo.On("UpdateLastRunAt", ctx, "auto-123").Return(nil)

	result, err := svc.RunAutomation(ctx, "auto-123", "user-1")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success)
}

func TestProcessSearchMode_WithCustomDispatchText(t *testing.T) {
	// Search with custom dispatch text template
	svc, deps := newTestService(t)
	ctx := context.Background()

	searchType := "search"
	keyword := "smartphone"
	dispatchTextID := "dispatch-123"
	automation := &models.Automation{
		ID:             "auto-123",
		UserID:         "user-1",
		Name:           "Search Automation",
		Status:         models.AutomationStatusActive,
		Type:           &searchType,
		Keyword:        &keyword,
		GroupIDs:       []string{"group1@g.us"},
		Stores:         []string{"shopee"},
		DispatchTextID: &dispatchTextID,
	}

	connectedInstances := []models.Instance{
		{ID: 1, InstanceID: "instance-1", UserID: "user-1", Status: "connected"},
	}

	shopeeOffers := &models.ShopeeSearchResult{
		Offers: []models.ShopeeOffer{
			{
				ItemID:        12345,
				ShopID:        67890,
				Name:          "Smartphone XYZ",
				Price:         999.90,
				OriginalPrice: 1299.90,
				Discount:      23,
				AffiliateLink: "https://affiliate.shopee.com/xyz",
				ProductURL:    "https://shopee.com.br/product/67890/12345",
			},
		},
		Total:   1,
		HasMore: false,
	}

	customTemplate := &models.DispatchText{
		ID:       "dispatch-123",
		UserID:   "user-1",
		Body:     "OFERTA: {{PRODUTO_NOME}} por {{PRODUTO_PRECO}} - {{LINK_AFILIADO}}",
		IsActive: true,
	}

	lockTime := time.Now().Add(10 * time.Minute)

	deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "auto-123", "user-1").Return(automation, nil)
	deps.automationRunRepo.On("GetLock", ctx, "auto-123").Return(nil, nil)
	deps.automationRunRepo.On("AcquireLock", ctx, "auto-123", mock.AnythingOfType("time.Duration")).Return(&lockTime, nil)
	deps.automationRunRepo.On("ReleaseLock", ctx, "auto-123").Return(nil)
	deps.historyRepo.On("Create", ctx, "user-1", "automation_run", mock.AnythingOfType("string"), mock.Anything).Return(&models.ActionHistory{ID: "history-1"}, nil)
	deps.historyRepo.On("Update", ctx, "history-1", mock.Anything).Return(nil)
	deps.instanceRepo.On("GetUserInstances", ctx, "user-1").Return(connectedInstances, nil)
	deps.shopeeService.On("SearchOffers", ctx, "user-1", mock.AnythingOfType("models.ShopeeSearchParams")).Return(shopeeOffers, nil)
	// Price refetch before sending
	deps.shopeeService.On("GetProduct", ctx, int64(12345), int64(67890), "user-1").Return(nil, errors.New("not found")).Maybe()
	// Affiliate link generation
	deps.shopeeService.On("GenerateAffiliateLink", ctx, "https://shopee.com.br/product/67890/12345", "user-1").Return(&models.ShopeeAffiliateLink{
		AffiliateLink: "https://s.shopee.com.br/tracked-link",
	}, nil).Maybe()
	deps.settingsRepo.On("GetActiveDispatchTextByID", ctx, "dispatch-123", "user-1").Return(customTemplate, nil)
	deps.dispatchLogRepo.On("GetRecentByCanonicalKey", ctx, "user-1", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("int")).Return(nil, nil).Maybe()
	deps.dispatchLogRepo.On("Create", ctx, mock.AnythingOfType("*models.DispatchLog")).Return(&models.DispatchLog{}, nil).Maybe()
	deps.automationRepo.On("UpdateLastRunAt", ctx, "auto-123").Return(nil)

	result, err := svc.RunAutomation(ctx, "auto-123", "user-1")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success)
	// Custom template was fetched
	deps.settingsRepo.AssertCalled(t, "GetActiveDispatchTextByID", ctx, "dispatch-123", "user-1")
}

// =============================================================================
// Table-Driven Tests for SEARCH Mode Edge Cases
// =============================================================================

func TestProcessSearchMode_EdgeCases(t *testing.T) {
	tests := []struct {
		name           string
		groupIDs       []string
		instanceStatus string
		offersCount    int
		wantSuccess    bool
		wantMsgContain string
	}{
		{
			name:           "no groups",
			groupIDs:       []string{},
			instanceStatus: "connected",
			offersCount:    0,
			wantSuccess:    false,
			wantMsgContain: "target groups",
		},
		{
			name:           "disconnected instance",
			groupIDs:       []string{"group1@g.us"},
			instanceStatus: "disconnected",
			offersCount:    0,
			wantSuccess:    false,
			wantMsgContain: "connected",
		},
		{
			name:           "no offers found",
			groupIDs:       []string{"group1@g.us"},
			instanceStatus: "connected",
			offersCount:    0,
			wantSuccess:    true,
			wantMsgContain: "Nenhuma oferta",
		},
		{
			name:           "offers found",
			groupIDs:       []string{"group1@g.us"},
			instanceStatus: "connected",
			offersCount:    3,
			wantSuccess:    true,
			wantMsgContain: "",
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
				logger,
			)

			ctx := context.Background()
			searchType := "search"
			keyword := "test"
			auto := &models.Automation{
				ID:       "auto-123",
				UserID:   "user-1",
				Name:     "Test Automation",
				Status:   models.AutomationStatusActive,
				Type:     &searchType,
				Keyword:  &keyword,
				GroupIDs: tt.groupIDs,
				Stores:   []string{"shopee"},
			}

			lockTime := time.Now().Add(10 * time.Minute)

			deps.automationRepo.On("GetAutomationByIDAndUser", ctx, "auto-123", "user-1").Return(auto, nil)
			deps.automationRunRepo.On("GetLock", ctx, "auto-123").Return(nil, nil)
			deps.automationRunRepo.On("AcquireLock", ctx, "auto-123", mock.AnythingOfType("time.Duration")).Return(&lockTime, nil)
			deps.automationRunRepo.On("ReleaseLock", ctx, "auto-123").Return(nil)
			deps.historyRepo.On("Create", ctx, "user-1", "automation_run", mock.AnythingOfType("string"), mock.Anything).Return(&models.ActionHistory{ID: "h-1"}, nil)
			deps.historyRepo.On("Update", ctx, "h-1", mock.Anything).Return(nil)

			if len(tt.groupIDs) > 0 {
				deps.instanceRepo.On("GetUserInstances", ctx, "user-1").Return([]models.Instance{
					{ID: 1, InstanceID: "instance-1", Status: tt.instanceStatus},
				}, nil)

				if tt.instanceStatus == "connected" {
					offers := make([]models.ShopeeOffer, tt.offersCount)
					for i := 0; i < tt.offersCount; i++ {
						offers[i] = models.ShopeeOffer{
							ItemID:        int64(12345 + i),
							ShopID:        67890,
							Name:          "Product",
							Price:         99.90,
							OriginalPrice: 199.90,
							Discount:      50,
							AffiliateLink: "https://link",
							ProductURL:    "https://shopee.com.br/product/67890/12345",
						}
					}
					deps.shopeeService.On("SearchOffers", ctx, "user-1", mock.AnythingOfType("models.ShopeeSearchParams")).Return(&models.ShopeeSearchResult{
						Offers: offers,
						Total:  tt.offersCount,
					}, nil)

					if tt.offersCount > 0 {
						// Price refetch before sending
						deps.shopeeService.On("GetProduct", ctx, mock.AnythingOfType("int64"), mock.AnythingOfType("int64"), "user-1").Return(nil, errors.New("not found")).Maybe()
						// Affiliate link generation
						deps.shopeeService.On("GenerateAffiliateLink", ctx, mock.AnythingOfType("string"), "user-1").Return(&models.ShopeeAffiliateLink{
							AffiliateLink: "https://s.shopee.com.br/tracked-link",
						}, nil).Maybe()
						deps.dispatchLogRepo.On("GetRecentByCanonicalKey", ctx, "user-1", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("int")).Return(nil, nil).Maybe()
						deps.dispatchLogRepo.On("Create", ctx, mock.AnythingOfType("*models.DispatchLog")).Return(&models.DispatchLog{}, nil).Maybe()
					}
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
