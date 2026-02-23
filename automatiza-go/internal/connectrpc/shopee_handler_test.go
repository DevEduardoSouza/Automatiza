package connectrpc

import (
	"context"
	"errors"
	"testing"

	affiliatesv1 "automatiza-go/internal/gen/affiliates/v1"
	"automatiza-go/internal/models"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Mock Implementations
// =============================================================================

// mockShopeeService implements shopee.Service for testing.
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

// =============================================================================
// Helper Functions
// =============================================================================

// createShopeeHandler creates a ShopeeHandler with mocked dependencies.
func createShopeeHandler(t *testing.T) (*ShopeeHandler, *mockShopeeService) {
	t.Helper()
	svc := new(mockShopeeService)
	handler := NewShopeeHandler(svc)
	return handler, svc
}

// =============================================================================
// SearchOffers Tests
// =============================================================================

func TestSearchOffers_SuccessWithDefaultLimit(t *testing.T) {
	handler, svc := createShopeeHandler(t)

	ctx := contextWithUser("user-123", "user@example.com")

	keyword := "smartphone"

	svc.On("SearchOffers", ctx, "user-123", mock.MatchedBy(func(params models.ShopeeSearchParams) bool {
		return params.Limit == 20 && params.Keyword != nil && *params.Keyword == keyword
	})).Return(&models.ShopeeSearchResult{
		Offers: []models.ShopeeOffer{
			{
				ProductURL:    "https://shopee.com.br/product/1/1001",
				Name:          "Smartphone XYZ",
				OriginalPrice: 1999.99,
				Price:         1499.99,
				Discount:      25,
				Image:         "https://example.com/img.jpg",
				ShopName:      "TechStore",
				CommissionRate: 5.5,
			},
			{
				ProductURL:    "https://shopee.com.br/product/2/2001",
				Name:          "Smartphone ABC",
				OriginalPrice: 999.99,
				Price:         799.99,
				Discount:      20,
			},
		},
		Total: 2,
	}, nil)

	req := connect.NewRequest(&affiliatesv1.SearchOffersRequest{
		Keyword: &keyword,
	})

	resp, err := handler.SearchOffers(ctx, req)

	require.NoError(t, err)
	assert.Len(t, resp.Msg.GetOffers(), 2)
	assert.Equal(t, "Smartphone XYZ", resp.Msg.GetOffers()[0].GetTitle())
	assert.Equal(t, float64(1499.99), resp.Msg.GetOffers()[0].GetFinalPrice())
	assert.Equal(t, float64(1999.99), resp.Msg.GetOffers()[0].GetOriginalPrice())
	assert.Equal(t, int32(25), resp.Msg.GetOffers()[0].GetDiscountPercent())
	svc.AssertExpectations(t)
}

func TestSearchOffers_Unauthenticated(t *testing.T) {
	handler, _ := createShopeeHandler(t)

	ctx := context.Background()

	keyword := "test"
	req := connect.NewRequest(&affiliatesv1.SearchOffersRequest{
		Keyword: &keyword,
	})

	_, err := handler.SearchOffers(ctx, req)

	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestSearchOffers_CustomParams(t *testing.T) {
	handler, svc := createShopeeHandler(t)

	ctx := contextWithUser("user-123", "user@example.com")

	keyword := "notebook"
	minPrice := float64(500)
	maxPrice := float64(5000)
	sortBy := affiliatesv1.SortBy_SORT_BY_PRICE_LOW
	limit := int32(10)

	svc.On("SearchOffers", ctx, "user-123", mock.MatchedBy(func(params models.ShopeeSearchParams) bool {
		return params.Keyword != nil && *params.Keyword == keyword &&
			params.MinPrice != nil && *params.MinPrice == minPrice &&
			params.MaxPrice != nil && *params.MaxPrice == maxPrice &&
			params.SortBy != nil && *params.SortBy == "price_low" &&
			params.Limit == 10
	})).Return(&models.ShopeeSearchResult{
		Offers: []models.ShopeeOffer{},
		Total:  0,
	}, nil)

	req := connect.NewRequest(&affiliatesv1.SearchOffersRequest{
		Keyword:  &keyword,
		MinPrice: &minPrice,
		MaxPrice: &maxPrice,
		SortBy:   &sortBy,
		Limit:    &limit,
	})

	resp, err := handler.SearchOffers(ctx, req)

	require.NoError(t, err)
	assert.Len(t, resp.Msg.GetOffers(), 0)
	svc.AssertExpectations(t)
}

func TestSearchOffers_ServiceError(t *testing.T) {
	handler, svc := createShopeeHandler(t)

	ctx := contextWithUser("user-123", "user@example.com")

	svc.On("SearchOffers", ctx, "user-123", mock.AnythingOfType("models.ShopeeSearchParams")).
		Return(nil, errors.New("shopee API error"))

	keyword := "test"
	req := connect.NewRequest(&affiliatesv1.SearchOffersRequest{
		Keyword: &keyword,
	})

	_, err := handler.SearchOffers(ctx, req)

	require.Error(t, err)
	assert.Equal(t, connect.CodeInternal, connect.CodeOf(err))
	svc.AssertExpectations(t)
}

func TestSearchOffers_EmptyResults(t *testing.T) {
	handler, svc := createShopeeHandler(t)

	ctx := contextWithUser("user-123", "user@example.com")

	svc.On("SearchOffers", ctx, "user-123", mock.AnythingOfType("models.ShopeeSearchParams")).
		Return(&models.ShopeeSearchResult{
			Offers: []models.ShopeeOffer{},
			Total:  0,
		}, nil)

	keyword := "nonexistent-product"
	req := connect.NewRequest(&affiliatesv1.SearchOffersRequest{
		Keyword: &keyword,
	})

	resp, err := handler.SearchOffers(ctx, req)

	require.NoError(t, err)
	assert.Len(t, resp.Msg.GetOffers(), 0)
	svc.AssertExpectations(t)
}

func TestSearchOffers_OptionalFieldsMapping(t *testing.T) {
	handler, svc := createShopeeHandler(t)

	ctx := contextWithUser("user-123", "user@example.com")

	svc.On("SearchOffers", ctx, "user-123", mock.AnythingOfType("models.ShopeeSearchParams")).
		Return(&models.ShopeeSearchResult{
			Offers: []models.ShopeeOffer{
				{
					ProductURL:     "https://shopee.com.br/product/1/1001",
					Name:           "Product With All Fields",
					OriginalPrice:  199.99,
					Price:          149.99,
					Discount:       25,
					Image:          "https://example.com/image.jpg",
					ShopName:       "BestShop",
					CommissionRate: 8.5,
				},
			},
			Total: 1,
		}, nil)

	keyword := "test"
	req := connect.NewRequest(&affiliatesv1.SearchOffersRequest{
		Keyword: &keyword,
	})

	resp, err := handler.SearchOffers(ctx, req)

	require.NoError(t, err)
	require.Len(t, resp.Msg.GetOffers(), 1)

	offer := resp.Msg.GetOffers()[0]
	assert.Equal(t, "https://example.com/image.jpg", offer.GetImageUrl())
	assert.Equal(t, "BestShop", offer.GetStoreName())
	assert.Equal(t, 8.5, offer.GetCommissionRate())
	svc.AssertExpectations(t)
}

// =============================================================================
// GenerateAffiliateLink Tests
// =============================================================================

func TestGenerateAffiliateLink_Success(t *testing.T) {
	handler, svc := createShopeeHandler(t)

	ctx := contextWithUser("user-123", "user@example.com")

	productURL := "https://shopee.com.br/product/123/456"

	svc.On("GenerateAffiliateLink", ctx, productURL, "user-123").Return(&models.ShopeeAffiliateLink{
		OriginalURL:   productURL,
		AffiliateLink: "https://shope.ee/abc123",
	}, nil)

	req := connect.NewRequest(&affiliatesv1.GenerateAffiliateLinkRequest{
		ProductUrl: productURL,
	})

	resp, err := handler.GenerateAffiliateLink(ctx, req)

	require.NoError(t, err)
	assert.Equal(t, "https://shope.ee/abc123", resp.Msg.GetAffiliateUrl())
	svc.AssertExpectations(t)
}

func TestGenerateAffiliateLink_Unauthenticated(t *testing.T) {
	handler, _ := createShopeeHandler(t)

	ctx := context.Background()

	req := connect.NewRequest(&affiliatesv1.GenerateAffiliateLinkRequest{
		ProductUrl: "https://shopee.com.br/product/123/456",
	})

	_, err := handler.GenerateAffiliateLink(ctx, req)

	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestGenerateAffiliateLink_MissingProductUrl(t *testing.T) {
	handler, _ := createShopeeHandler(t)

	ctx := contextWithUser("user-123", "user@example.com")

	req := connect.NewRequest(&affiliatesv1.GenerateAffiliateLinkRequest{
		ProductUrl: "",
	})

	_, err := handler.GenerateAffiliateLink(ctx, req)

	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestGenerateAffiliateLink_ServiceError(t *testing.T) {
	handler, svc := createShopeeHandler(t)

	ctx := contextWithUser("user-123", "user@example.com")

	productURL := "https://shopee.com.br/product/123/456"

	svc.On("GenerateAffiliateLink", ctx, productURL, "user-123").
		Return(nil, errors.New("shopee credentials not configured"))

	req := connect.NewRequest(&affiliatesv1.GenerateAffiliateLinkRequest{
		ProductUrl: productURL,
	})

	_, err := handler.GenerateAffiliateLink(ctx, req)

	require.Error(t, err)
	assert.Equal(t, connect.CodeInternal, connect.CodeOf(err))
	svc.AssertExpectations(t)
}

// =============================================================================
// mapProtoSortBy Tests
// =============================================================================

func TestMapProtoSortBy(t *testing.T) {
	tests := []struct {
		input    affiliatesv1.SortBy
		expected string
	}{
		{affiliatesv1.SortBy_SORT_BY_COMMISSION, "commission"},
		{affiliatesv1.SortBy_SORT_BY_PRICE_LOW, "price_low"},
		{affiliatesv1.SortBy_SORT_BY_PRICE_HIGH, "price_high"},
		{affiliatesv1.SortBy_SORT_BY_SALES, "sales"},
		{affiliatesv1.SortBy_SORT_BY_DISCOUNT, "discount"},
		{affiliatesv1.SortBy_SORT_BY_SAVING, "saving"},
		{affiliatesv1.SortBy(999), ""},
	}

	for _, tt := range tests {
		t.Run(tt.input.String(), func(t *testing.T) {
			result := mapProtoSortBy(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
