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

// mockMlService implements mercadolivre.Service for testing.
type mockMlService struct {
	mock.Mock
}

func (m *mockMlService) CreateAffiliateLink(ctx context.Context, originURL, userID string) (*models.MLAffiliateLink, error) {
	args := m.Called(ctx, originURL, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.MLAffiliateLink), args.Error(1)
}

func (m *mockMlService) GetProductPreview(ctx context.Context, originURL, userID string) (*models.MLProductPreview, error) {
	args := m.Called(ctx, originURL, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.MLProductPreview), args.Error(1)
}

func (m *mockMlService) LoadCredentials(ctx context.Context, userID string) (*models.MLCredentials, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.MLCredentials), args.Error(1)
}

func (m *mockMlService) SaveCredentials(ctx context.Context, userID, cookie, csrf, tag string) error {
	args := m.Called(ctx, userID, cookie, csrf, tag)
	return args.Error(0)
}

func (m *mockMlService) IsConnected(ctx context.Context, userID string) (bool, error) {
	args := m.Called(ctx, userID)
	return args.Bool(0), args.Error(1)
}

func (m *mockMlService) ExtractProductID(productURL string) (string, error) {
	args := m.Called(productURL)
	return args.String(0), args.Error(1)
}

func (m *mockMlService) InvalidatePreviewCache(ctx context.Context, originURL string) error {
	args := m.Called(ctx, originURL)
	return args.Error(0)
}

// =============================================================================
// Helper Functions
// =============================================================================

// createMlHandler creates a MercadoLivreHandler with mocked dependencies.
func createMlHandler(t *testing.T) (*MercadoLivreHandler, *mockMlService) {
	t.Helper()
	svc := new(mockMlService)
	handler := NewMercadoLivreHandler(svc)
	return handler, svc
}

// =============================================================================
// Connect Tests
// =============================================================================

func TestMlConnect_Success(t *testing.T) {
	handler, svc := createMlHandler(t)

	ctx := contextWithUser("user-123", "user@example.com")

	svc.On("SaveCredentials", ctx, "user-123", "my-cookie-value", "my-csrf-token", "my-tag").Return(nil)

	req := connect.NewRequest(&affiliatesv1.MlConnectRequest{
		Cookie:    strPtr("my-cookie-value"),
		CsrfToken: strPtr("my-csrf-token"),
		Tag:       "my-tag",
	})

	resp, err := handler.Connect(ctx, req)

	require.NoError(t, err)
	assert.True(t, resp.Msg.GetSuccess())
	assert.NotNil(t, resp.Msg.Message)
	svc.AssertExpectations(t)
}

func TestMlConnect_Unauthenticated(t *testing.T) {
	handler, _ := createMlHandler(t)

	ctx := context.Background()

	req := connect.NewRequest(&affiliatesv1.MlConnectRequest{
		Cookie:    strPtr("my-cookie-value"),
		CsrfToken: strPtr("my-csrf-token"),
	})

	_, err := handler.Connect(ctx, req)

	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestMlConnect_MissingCookie(t *testing.T) {
	handler, _ := createMlHandler(t)

	ctx := contextWithUser("user-123", "user@example.com")

	req := connect.NewRequest(&affiliatesv1.MlConnectRequest{
		Cookie:    strPtr(""),
		CsrfToken: strPtr("my-csrf-token"),
		Tag:       "my-tag",
	})

	_, err := handler.Connect(ctx, req)

	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestMlConnect_MissingTag(t *testing.T) {
	handler, _ := createMlHandler(t)

	ctx := contextWithUser("user-123", "user@example.com")

	req := connect.NewRequest(&affiliatesv1.MlConnectRequest{
		Cookie:    strPtr("my-cookie-value"),
		CsrfToken: strPtr("my-csrf-token"),
		Tag:       "",
	})

	_, err := handler.Connect(ctx, req)

	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestMlConnect_ServiceError(t *testing.T) {
	handler, svc := createMlHandler(t)

	ctx := contextWithUser("user-123", "user@example.com")

	svc.On("SaveCredentials", ctx, "user-123", "my-cookie", "my-csrf", "my-tag").
		Return(errors.New("encryption error"))

	req := connect.NewRequest(&affiliatesv1.MlConnectRequest{
		Cookie:    strPtr("my-cookie"),
		CsrfToken: strPtr("my-csrf"),
		Tag:       "my-tag",
	})

	_, err := handler.Connect(ctx, req)

	require.Error(t, err)
	assert.Equal(t, connect.CodeInternal, connect.CodeOf(err))
	svc.AssertExpectations(t)
}

// =============================================================================
// CreateLink Tests
// =============================================================================

func TestMlCreateLink_Success(t *testing.T) {
	handler, svc := createMlHandler(t)

	ctx := contextWithUser("user-123", "user@example.com")

	originURL := "https://www.mercadolivre.com.br/produto/MLB12345"

	svc.On("CreateAffiliateLink", ctx, originURL, "user-123").Return(&models.MLAffiliateLink{
		OriginalURL:   originURL,
		AffiliateLink: "https://mercado.link/short123",
		ProductID:     "MLB12345",
	}, nil)

	req := connect.NewRequest(&affiliatesv1.MlCreateLinkRequest{
		OriginUrl: originURL,
	})

	resp, err := handler.CreateLink(ctx, req)

	require.NoError(t, err)
	assert.Len(t, resp.Msg.GetUrls(), 1)
	assert.Equal(t, originURL, resp.Msg.GetUrls()[0].GetLongUrl())
	assert.Equal(t, "https://mercado.link/short123", resp.Msg.GetUrls()[0].GetShortUrl())
	svc.AssertExpectations(t)
}

func TestMlCreateLink_Unauthenticated(t *testing.T) {
	handler, _ := createMlHandler(t)

	ctx := context.Background()

	req := connect.NewRequest(&affiliatesv1.MlCreateLinkRequest{
		OriginUrl: "https://www.mercadolivre.com.br/produto/MLB12345",
	})

	_, err := handler.CreateLink(ctx, req)

	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestMlCreateLink_MissingOriginUrl(t *testing.T) {
	handler, _ := createMlHandler(t)

	ctx := contextWithUser("user-123", "user@example.com")

	req := connect.NewRequest(&affiliatesv1.MlCreateLinkRequest{
		OriginUrl: "",
	})

	_, err := handler.CreateLink(ctx, req)

	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestMlCreateLink_ServiceError(t *testing.T) {
	handler, svc := createMlHandler(t)

	ctx := contextWithUser("user-123", "user@example.com")

	originURL := "https://www.mercadolivre.com.br/produto/MLB12345"

	svc.On("CreateAffiliateLink", ctx, originURL, "user-123").
		Return(nil, errors.New("ML session expired"))

	req := connect.NewRequest(&affiliatesv1.MlCreateLinkRequest{
		OriginUrl: originURL,
	})

	_, err := handler.CreateLink(ctx, req)

	require.Error(t, err)
	assert.Equal(t, connect.CodeInternal, connect.CodeOf(err))
	svc.AssertExpectations(t)
}

// =============================================================================
// Preview Tests
// =============================================================================

func TestMlPreview_SuccessWithAllFields(t *testing.T) {
	handler, svc := createMlHandler(t)

	ctx := contextWithUser("user-123", "user@example.com")

	originURL := "https://www.mercadolivre.com.br/produto/MLB12345"
	condition := "new"
	soldQty := 150
	rating := 4.5
	reviews := 80
	originalPrice := 199.99
	discount := 25.0
	installments := "em 12x R$ 12,49"

	svc.On("GetProductPreview", ctx, originURL, "user-123").Return(&models.MLProductPreview{
		Title:           "Product Title",
		Price:           149.99,
		Image:           "https://example.com/image.jpg",
		Condition:       &condition,
		SoldQuantity:    &soldQty,
		RatingAverage:   &rating,
		ReviewsTotal:    &reviews,
		OriginalPrice:   &originalPrice,
		DiscountPercent: &discount,
		Installments:    &installments,
	}, nil)

	req := connect.NewRequest(&affiliatesv1.MlPreviewRequest{
		OriginUrl: originURL,
	})

	resp, err := handler.Preview(ctx, req)

	require.NoError(t, err)
	preview := resp.Msg.GetPreview()
	assert.NotNil(t, preview)
	assert.Equal(t, "Product Title", preview.GetTitle())
	assert.Equal(t, float64(149.99), preview.GetPrice())
	assert.Equal(t, "https://example.com/image.jpg", preview.GetImageUrl())
	assert.Equal(t, "BRL", preview.GetCurrency())
	assert.Equal(t, "new", preview.GetCondition())
	assert.Equal(t, int32(150), preview.GetSoldQuantity())
	assert.Equal(t, 4.5, preview.GetRatingAverage())
	assert.Equal(t, int32(80), preview.GetReviewsTotal())
	assert.Equal(t, 199.99, preview.GetOriginalPrice())
	assert.Equal(t, 25.0, preview.GetDiscountPercent())
	assert.Equal(t, "em 12x R$ 12,49", preview.GetInstallmentsText())
	svc.AssertExpectations(t)
}

func TestMlPreview_Unauthenticated(t *testing.T) {
	handler, _ := createMlHandler(t)

	ctx := context.Background()

	req := connect.NewRequest(&affiliatesv1.MlPreviewRequest{
		OriginUrl: "https://www.mercadolivre.com.br/produto/MLB12345",
	})

	_, err := handler.Preview(ctx, req)

	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestMlPreview_MissingOriginUrl(t *testing.T) {
	handler, _ := createMlHandler(t)

	ctx := contextWithUser("user-123", "user@example.com")

	req := connect.NewRequest(&affiliatesv1.MlPreviewRequest{
		OriginUrl: "",
	})

	_, err := handler.Preview(ctx, req)

	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestMlPreview_ProductNotFound(t *testing.T) {
	handler, svc := createMlHandler(t)

	ctx := contextWithUser("user-123", "user@example.com")

	originURL := "https://www.mercadolivre.com.br/produto/MLB99999"

	svc.On("GetProductPreview", ctx, originURL, "user-123").Return(nil, nil)

	req := connect.NewRequest(&affiliatesv1.MlPreviewRequest{
		OriginUrl: originURL,
	})

	_, err := handler.Preview(ctx, req)

	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
	svc.AssertExpectations(t)
}

func TestMlPreview_ServiceError(t *testing.T) {
	handler, svc := createMlHandler(t)

	ctx := contextWithUser("user-123", "user@example.com")

	originURL := "https://www.mercadolivre.com.br/produto/MLB12345"

	svc.On("GetProductPreview", ctx, originURL, "user-123").
		Return(nil, errors.New("failed to fetch product"))

	req := connect.NewRequest(&affiliatesv1.MlPreviewRequest{
		OriginUrl: originURL,
	})

	_, err := handler.Preview(ctx, req)

	require.Error(t, err)
	assert.Equal(t, connect.CodeInternal, connect.CodeOf(err))
	svc.AssertExpectations(t)
}

func TestMlPreview_SuccessWithRequiredFieldsOnly(t *testing.T) {
	handler, svc := createMlHandler(t)

	ctx := contextWithUser("user-123", "user@example.com")

	originURL := "https://www.mercadolivre.com.br/produto/MLB12345"

	svc.On("GetProductPreview", ctx, originURL, "user-123").Return(&models.MLProductPreview{
		Title: "Basic Product",
		Price: 99.99,
		Image: "https://example.com/basic.jpg",
		// No optional fields set
	}, nil)

	req := connect.NewRequest(&affiliatesv1.MlPreviewRequest{
		OriginUrl: originURL,
	})

	resp, err := handler.Preview(ctx, req)

	require.NoError(t, err)
	preview := resp.Msg.GetPreview()
	assert.NotNil(t, preview)
	assert.Equal(t, "Basic Product", preview.GetTitle())
	assert.Equal(t, float64(99.99), preview.GetPrice())
	assert.Equal(t, "https://example.com/basic.jpg", preview.GetImageUrl())
	assert.Equal(t, "BRL", preview.GetCurrency())
	// Optional fields should be zero/nil
	assert.Equal(t, "", preview.GetCondition())
	assert.Equal(t, int32(0), preview.GetSoldQuantity())
	assert.Equal(t, float64(0), preview.GetRatingAverage())
	assert.Equal(t, int32(0), preview.GetReviewsTotal())
	assert.Equal(t, float64(0), preview.GetOriginalPrice())
	assert.Equal(t, float64(0), preview.GetDiscountPercent())
	assert.Equal(t, "", preview.GetInstallmentsText())
	svc.AssertExpectations(t)
}

// =============================================================================
// InvalidatePreviewCache Tests
// =============================================================================

func TestMlInvalidatePreviewCache_Success(t *testing.T) {
	handler, svc := createMlHandler(t)

	ctx := contextWithUser("user-123", "user@example.com")

	originURL := "https://www.mercadolivre.com.br/produto/MLB12345"

	svc.On("InvalidatePreviewCache", ctx, originURL).Return(nil)

	req := connect.NewRequest(&affiliatesv1.MlInvalidatePreviewCacheRequest{
		OriginUrl: originURL,
	})

	resp, err := handler.InvalidatePreviewCache(ctx, req)

	require.NoError(t, err)
	assert.True(t, resp.Msg.GetSuccess())
	assert.NotNil(t, resp.Msg.Message)
	svc.AssertExpectations(t)
}

func TestMlInvalidatePreviewCache_Unauthenticated(t *testing.T) {
	handler, _ := createMlHandler(t)

	ctx := context.Background()

	req := connect.NewRequest(&affiliatesv1.MlInvalidatePreviewCacheRequest{
		OriginUrl: "https://www.mercadolivre.com.br/produto/MLB12345",
	})

	_, err := handler.InvalidatePreviewCache(ctx, req)

	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestMlInvalidatePreviewCache_MissingOriginUrl(t *testing.T) {
	handler, _ := createMlHandler(t)

	ctx := contextWithUser("user-123", "user@example.com")

	req := connect.NewRequest(&affiliatesv1.MlInvalidatePreviewCacheRequest{
		OriginUrl: "",
	})

	_, err := handler.InvalidatePreviewCache(ctx, req)

	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestMlInvalidatePreviewCache_ServiceError(t *testing.T) {
	handler, svc := createMlHandler(t)

	ctx := contextWithUser("user-123", "user@example.com")

	originURL := "https://www.mercadolivre.com.br/produto/MLB12345"

	svc.On("InvalidatePreviewCache", ctx, originURL).Return(errors.New("cache error"))

	req := connect.NewRequest(&affiliatesv1.MlInvalidatePreviewCacheRequest{
		OriginUrl: originURL,
	})

	_, err := handler.InvalidatePreviewCache(ctx, req)

	require.Error(t, err)
	assert.Equal(t, connect.CodeInternal, connect.CodeOf(err))
	svc.AssertExpectations(t)
}
