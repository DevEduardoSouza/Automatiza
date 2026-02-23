package jobs

import (
	"context"
	"errors"
	"testing"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// =============================================================================
// Mock ML Service for internal tests
// =============================================================================

type mockMLSvc struct {
	mock.Mock
}

func (m *mockMLSvc) CreateAffiliateLink(ctx context.Context, originURL, userID string) (*models.MLAffiliateLink, error) {
	args := m.Called(ctx, originURL, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.MLAffiliateLink), args.Error(1)
}

func (m *mockMLSvc) GetProductPreview(ctx context.Context, originURL, userID string) (*models.MLProductPreview, error) {
	args := m.Called(ctx, originURL, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.MLProductPreview), args.Error(1)
}

func (m *mockMLSvc) LoadCredentials(ctx context.Context, userID string) (*models.MLCredentials, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.MLCredentials), args.Error(1)
}

func (m *mockMLSvc) SaveCredentials(ctx context.Context, userID, cookie, csrf string) error {
	args := m.Called(ctx, userID, cookie, csrf)
	return args.Error(0)
}

func (m *mockMLSvc) IsConnected(ctx context.Context, userID string) (bool, error) {
	args := m.Called(ctx, userID)
	return args.Bool(0), args.Error(1)
}

func (m *mockMLSvc) ExtractProductID(productURL string) (string, error) {
	args := m.Called(productURL)
	return args.String(0), args.Error(1)
}

func (m *mockMLSvc) InvalidatePreviewCache(ctx context.Context, originURL string) error {
	args := m.Called(ctx, originURL)
	return args.Error(0)
}

// =============================================================================
// ExecuteSendMLOffer Tests
// =============================================================================

func TestExecuteSendMLOffer_Success_TextOnly(t *testing.T) {
	ctx := context.Background()
	provider := whatsapp.NewFakeAdapter()
	ml := new(mockMLSvc)
	logger := zaptest.NewLogger(t)

	ml.On("CreateAffiliateLink", ctx, "https://mercadolivre.com.br/MLB-123", "user-1").
		Return(&models.MLAffiliateLink{AffiliateLink: "https://aff.ml.com/123"}, nil)

	job := &models.StandaloneJob{
		ActionType: models.JobActionSendMLOffer,
		Payload: models.JobPayload{
			Groups:    []string{"group1@g.us"},
			OriginURL: "https://mercadolivre.com.br/MLB-123",
			Text:      "Confira!",
		},
	}

	results := ExecuteSendMLOffer(ctx, provider, ml, "session1", "user-1", job, logger)

	require.Len(t, results, 1)
	assert.True(t, results[0].Success)
	assert.Equal(t, 1, provider.CallCount("SendText"))

	call := provider.LastCall("SendText")
	require.NotNil(t, call)
	input := call.Args[0].(whatsapp.SendTextInput)
	assert.Contains(t, input.Text, "Confira!")
	assert.Contains(t, input.Text, "https://aff.ml.com/123")
	ml.AssertExpectations(t)
}

func TestExecuteSendMLOffer_WithImage(t *testing.T) {
	ctx := context.Background()
	provider := whatsapp.NewFakeAdapter()
	ml := new(mockMLSvc)
	logger := zaptest.NewLogger(t)

	includeImage := true
	origPrice := 199.90

	ml.On("CreateAffiliateLink", ctx, "https://mercadolivre.com.br/MLB-456", "user-1").
		Return(&models.MLAffiliateLink{AffiliateLink: "https://aff.ml.com/456"}, nil)
	ml.On("GetProductPreview", ctx, "https://mercadolivre.com.br/MLB-456", "user-1").
		Return(&models.MLProductPreview{
			Title:         "Produto Incrivel",
			Price:         149.90,
			OriginalPrice: &origPrice,
			Image:         "https://img.ml.com/product.jpg",
		}, nil)

	job := &models.StandaloneJob{
		ActionType: models.JobActionSendMLOffer,
		Payload: models.JobPayload{
			Groups:       []string{"group1@g.us"},
			OriginURL:    "https://mercadolivre.com.br/MLB-456",
			Text:         "Oferta imperdivel!",
			IncludeImage: &includeImage,
		},
	}

	results := ExecuteSendMLOffer(ctx, provider, ml, "session1", "user-1", job, logger)

	require.Len(t, results, 1)
	assert.True(t, results[0].Success)
	assert.Equal(t, 1, provider.CallCount("SendImage"))
	assert.Equal(t, 0, provider.CallCount("SendText"))

	call := provider.LastCall("SendImage")
	require.NotNil(t, call)
	input := call.Args[0].(whatsapp.SendImageInput)
	assert.Equal(t, "https://img.ml.com/product.jpg", input.ImageURL)
	assert.Contains(t, input.Caption, "Produto Incrivel")
	assert.Contains(t, input.Caption, "R$ 149.90")
	assert.Contains(t, input.Caption, "era R$ 199.90")
	assert.Contains(t, input.Caption, "https://aff.ml.com/456")
	ml.AssertExpectations(t)
}

func TestExecuteSendMLOffer_AffiliateLinkError_FallsBackToOriginal(t *testing.T) {
	ctx := context.Background()
	provider := whatsapp.NewFakeAdapter()
	ml := new(mockMLSvc)
	logger := zaptest.NewLogger(t)

	ml.On("CreateAffiliateLink", ctx, "https://mercadolivre.com.br/MLB-789", "user-1").
		Return(nil, errors.New("affiliate service unavailable"))

	job := &models.StandaloneJob{
		ActionType: models.JobActionSendMLOffer,
		Payload: models.JobPayload{
			Groups:    []string{"group1@g.us"},
			OriginURL: "https://mercadolivre.com.br/MLB-789",
		},
	}

	results := ExecuteSendMLOffer(ctx, provider, ml, "session1", "user-1", job, logger)

	require.Len(t, results, 1)
	assert.True(t, results[0].Success)

	call := provider.LastCall("SendText")
	require.NotNil(t, call)
	input := call.Args[0].(whatsapp.SendTextInput)
	assert.Contains(t, input.Text, "https://mercadolivre.com.br/MLB-789")
}

func TestExecuteSendMLOffer_EmptyURL_Error(t *testing.T) {
	ctx := context.Background()
	provider := whatsapp.NewFakeAdapter()
	ml := new(mockMLSvc)
	logger := zaptest.NewLogger(t)

	job := &models.StandaloneJob{
		ActionType: models.JobActionSendMLOffer,
		Payload: models.JobPayload{
			Groups:    []string{"group1@g.us", "group2@g.us"},
			OriginURL: "",
		},
	}

	results := ExecuteSendMLOffer(ctx, provider, ml, "session1", "user-1", job, logger)

	require.Len(t, results, 2)
	for _, r := range results {
		assert.False(t, r.Success)
		assert.Contains(t, r.Error, "URL")
	}
}

func TestExecuteSendMLOffer_ImageSendError_FallsBackToText(t *testing.T) {
	ctx := context.Background()
	provider := whatsapp.NewFakeAdapter()
	provider.SendImageFunc = func(_ context.Context, _ whatsapp.SendImageInput) (*whatsapp.MessageResult, error) {
		return nil, errors.New("image upload failed")
	}
	ml := new(mockMLSvc)
	logger := zaptest.NewLogger(t)

	includeImage := true
	ml.On("CreateAffiliateLink", ctx, "https://mercadolivre.com.br/MLB-123", "user-1").
		Return(&models.MLAffiliateLink{AffiliateLink: "https://aff.ml.com/123"}, nil)
	ml.On("GetProductPreview", ctx, "https://mercadolivre.com.br/MLB-123", "user-1").
		Return(&models.MLProductPreview{
			Title: "Produto",
			Price: 99.90,
			Image: "https://img.ml.com/photo.jpg",
		}, nil)

	job := &models.StandaloneJob{
		ActionType: models.JobActionSendMLOffer,
		Payload: models.JobPayload{
			Groups:       []string{"group1@g.us"},
			OriginURL:    "https://mercadolivre.com.br/MLB-123",
			IncludeImage: &includeImage,
		},
	}

	results := ExecuteSendMLOffer(ctx, provider, ml, "session1", "user-1", job, logger)

	require.Len(t, results, 1)
	assert.True(t, results[0].Success)
	// Falls back to text after image fails
	assert.Equal(t, 1, provider.CallCount("SendText"))
}

func TestExecuteSendMLOffer_PreviewError_SendsTextOnly(t *testing.T) {
	ctx := context.Background()
	provider := whatsapp.NewFakeAdapter()
	ml := new(mockMLSvc)
	logger := zaptest.NewLogger(t)

	includeImage := true
	ml.On("CreateAffiliateLink", ctx, "https://mercadolivre.com.br/MLB-123", "user-1").
		Return(&models.MLAffiliateLink{AffiliateLink: "https://aff.ml.com/123"}, nil)
	ml.On("GetProductPreview", ctx, "https://mercadolivre.com.br/MLB-123", "user-1").
		Return(nil, errors.New("preview unavailable"))

	job := &models.StandaloneJob{
		ActionType: models.JobActionSendMLOffer,
		Payload: models.JobPayload{
			Groups:       []string{"group1@g.us"},
			OriginURL:    "https://mercadolivre.com.br/MLB-123",
			IncludeImage: &includeImage,
		},
	}

	results := ExecuteSendMLOffer(ctx, provider, ml, "session1", "user-1", job, logger)

	require.Len(t, results, 1)
	assert.True(t, results[0].Success)
	assert.Equal(t, 1, provider.CallCount("SendText"))
	assert.Equal(t, 0, provider.CallCount("SendImage"))
}

func TestExecuteSendMLOffer_IncludeImageFalse(t *testing.T) {
	ctx := context.Background()
	provider := whatsapp.NewFakeAdapter()
	ml := new(mockMLSvc)
	logger := zaptest.NewLogger(t)

	includeImage := false
	ml.On("CreateAffiliateLink", ctx, "https://mercadolivre.com.br/MLB-123", "user-1").
		Return(&models.MLAffiliateLink{AffiliateLink: "https://aff.ml.com/123"}, nil)

	job := &models.StandaloneJob{
		ActionType: models.JobActionSendMLOffer,
		Payload: models.JobPayload{
			Groups:       []string{"group1@g.us"},
			OriginURL:    "https://mercadolivre.com.br/MLB-123",
			IncludeImage: &includeImage,
		},
	}

	results := ExecuteSendMLOffer(ctx, provider, ml, "session1", "user-1", job, logger)

	require.Len(t, results, 1)
	assert.True(t, results[0].Success)
	assert.Equal(t, 1, provider.CallCount("SendText"))
	assert.Equal(t, 0, provider.CallCount("SendImage"))
}

func TestExecuteSendMLOffer_IncludeImageNil(t *testing.T) {
	ctx := context.Background()
	provider := whatsapp.NewFakeAdapter()
	ml := new(mockMLSvc)
	logger := zaptest.NewLogger(t)

	ml.On("CreateAffiliateLink", ctx, "https://mercadolivre.com.br/MLB-123", "user-1").
		Return(&models.MLAffiliateLink{AffiliateLink: "https://aff.ml.com/123"}, nil)

	job := &models.StandaloneJob{
		ActionType: models.JobActionSendMLOffer,
		Payload: models.JobPayload{
			Groups:       []string{"group1@g.us"},
			OriginURL:    "https://mercadolivre.com.br/MLB-123",
			IncludeImage: nil,
		},
	}

	results := ExecuteSendMLOffer(ctx, provider, ml, "session1", "user-1", job, logger)

	require.Len(t, results, 1)
	assert.True(t, results[0].Success)
	assert.Equal(t, 0, provider.CallCount("SendImage"))
}

func TestExecuteSendMLOffer_PreviewWithEmptyImage(t *testing.T) {
	ctx := context.Background()
	provider := whatsapp.NewFakeAdapter()
	ml := new(mockMLSvc)
	logger := zaptest.NewLogger(t)

	includeImage := true
	ml.On("CreateAffiliateLink", ctx, "https://mercadolivre.com.br/MLB-123", "user-1").
		Return(&models.MLAffiliateLink{AffiliateLink: "https://aff.ml.com/123"}, nil)
	ml.On("GetProductPreview", ctx, "https://mercadolivre.com.br/MLB-123", "user-1").
		Return(&models.MLProductPreview{
			Title: "Produto",
			Price: 50.0,
			Image: "", // empty image URL
		}, nil)

	job := &models.StandaloneJob{
		ActionType: models.JobActionSendMLOffer,
		Payload: models.JobPayload{
			Groups:       []string{"group1@g.us"},
			OriginURL:    "https://mercadolivre.com.br/MLB-123",
			IncludeImage: &includeImage,
		},
	}

	results := ExecuteSendMLOffer(ctx, provider, ml, "session1", "user-1", job, logger)

	require.Len(t, results, 1)
	assert.True(t, results[0].Success)
	// Falls back to text because image URL is empty
	assert.Equal(t, 1, provider.CallCount("SendText"))
	assert.Equal(t, 0, provider.CallCount("SendImage"))
}

// =============================================================================
// buildMLOfferMessage Tests
// =============================================================================

func TestBuildMLOfferMessage_CustomTextAndLink(t *testing.T) {
	msg := buildMLOfferMessage("Aproveite!", "https://aff.ml.com/123", nil)

	assert.Contains(t, msg, "Aproveite!")
	assert.Contains(t, msg, "https://aff.ml.com/123")
}

func TestBuildMLOfferMessage_NoCustomText(t *testing.T) {
	msg := buildMLOfferMessage("", "https://aff.ml.com/123", nil)

	assert.Equal(t, "https://aff.ml.com/123", msg)
}

func TestBuildMLOfferMessage_WithPreview(t *testing.T) {
	origPrice := 200.0
	preview := &models.MLProductPreview{
		Title:         "Smartphone X",
		Price:         150.0,
		OriginalPrice: &origPrice,
		Image:         "https://img.ml.com/phone.jpg",
	}

	msg := buildMLOfferMessage("Oferta!", "https://aff.ml.com/456", preview)

	assert.Contains(t, msg, "Oferta!")
	assert.Contains(t, msg, "Smartphone X")
	assert.Contains(t, msg, "R$ 150.00")
	assert.Contains(t, msg, "era R$ 200.00")
	assert.Contains(t, msg, "https://aff.ml.com/456")
}

func TestBuildMLOfferMessage_WithPreviewNoDiscount(t *testing.T) {
	preview := &models.MLProductPreview{
		Title: "Fone Bluetooth",
		Price: 80.0,
	}

	msg := buildMLOfferMessage("", "https://aff.ml.com/789", preview)

	assert.Contains(t, msg, "Fone Bluetooth")
	assert.Contains(t, msg, "R$ 80.00")
	assert.NotContains(t, msg, "era R$")
}

func TestBuildMLOfferMessage_WithPreviewOriginalPriceLower(t *testing.T) {
	origPrice := 50.0
	preview := &models.MLProductPreview{
		Title:         "Produto",
		Price:         100.0,
		OriginalPrice: &origPrice,
	}

	msg := buildMLOfferMessage("", "https://aff.ml.com/000", preview)

	assert.Contains(t, msg, "R$ 100.00")
	// Original price is not greater than current price, so no "era" text
	assert.NotContains(t, msg, "era R$")
}

func TestBuildMLOfferMessage_PreviewEmptyTitle(t *testing.T) {
	preview := &models.MLProductPreview{
		Title: "",
		Price: 0,
	}

	msg := buildMLOfferMessage("Confira", "https://link.com", preview)

	assert.Contains(t, msg, "Confira")
	assert.Contains(t, msg, "https://link.com")
	assert.NotContains(t, msg, "R$")
}
