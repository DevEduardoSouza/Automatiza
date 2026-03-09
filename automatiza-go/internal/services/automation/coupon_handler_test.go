package automation

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// mockCouponRepoForAutomation implements repository.CouponRepository
type mockCouponRepoForAutomation struct {
	mock.Mock
}

var _ repository.CouponRepository = (*mockCouponRepoForAutomation)(nil)

func (m *mockCouponRepoForAutomation) GetUserCoupons(ctx context.Context, userID string, filters map[string]interface{}) ([]models.Coupon, error) {
	args := m.Called(ctx, userID, filters)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Coupon), args.Error(1)
}

func (m *mockCouponRepoForAutomation) GetCouponByID(ctx context.Context, couponID, userID string) (*models.Coupon, error) {
	args := m.Called(ctx, couponID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Coupon), args.Error(1)
}

func (m *mockCouponRepoForAutomation) GetCouponByCode(ctx context.Context, userID, code string) (*models.Coupon, error) {
	args := m.Called(ctx, userID, code)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Coupon), args.Error(1)
}

func (m *mockCouponRepoForAutomation) CreateCoupon(ctx context.Context, userID string, data *models.Coupon) (*models.Coupon, error) {
	args := m.Called(ctx, userID, data)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Coupon), args.Error(1)
}

func (m *mockCouponRepoForAutomation) UpdateCoupon(ctx context.Context, couponID, userID string, updates map[string]interface{}) error {
	args := m.Called(ctx, couponID, userID, updates)
	return args.Error(0)
}

func (m *mockCouponRepoForAutomation) DeleteCoupon(ctx context.Context, couponID, userID string) error {
	args := m.Called(ctx, couponID, userID)
	return args.Error(0)
}

func (m *mockCouponRepoForAutomation) GetActiveCouponsByStore(ctx context.Context, userID, store string) ([]models.Coupon, error) {
	args := m.Called(ctx, userID, store)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Coupon), args.Error(1)
}

func (m *mockCouponRepoForAutomation) GetBestCouponForProduct(ctx context.Context, userID, store string, productPrice float64, category *string) (*models.Coupon, error) {
	args := m.Called(ctx, userID, store, productPrice, category)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Coupon), args.Error(1)
}

func newTestCouponHandler(repo repository.CouponRepository) *CouponHandler {
	logger, _ := zap.NewDevelopment()
	return NewCouponHandler(repo, logger)
}

func strPtr(s string) *string       { return &s }
func float64Ptr(f float64) *float64 { return &f }

// ---------------------------------------------------------------------------
// ApplyCoupon tests
// ---------------------------------------------------------------------------

func TestApplyCoupon_NoCouponConfigured(t *testing.T) {
	t.Parallel()
	repo := new(mockCouponRepoForAutomation)
	handler := newTestCouponHandler(repo)

	automation := &models.Automation{
		ID:       "auto-1",
		UserID:   "user-1",
		CouponID: nil, // no coupon
	}

	result, err := handler.ApplyCoupon(context.Background(), automation, 100.0, "shopee")
	require.NoError(t, err)
	assert.False(t, result.Applied)
	assert.Equal(t, "no coupon configured", result.SkipReason)
	repo.AssertExpectations(t)
}

func TestApplyCoupon_EmptyCouponID(t *testing.T) {
	t.Parallel()
	repo := new(mockCouponRepoForAutomation)
	handler := newTestCouponHandler(repo)

	automation := &models.Automation{
		ID:       "auto-1",
		UserID:   "user-1",
		CouponID: strPtr(""),
	}

	result, err := handler.ApplyCoupon(context.Background(), automation, 100.0, "shopee")
	require.NoError(t, err)
	assert.False(t, result.Applied)
	assert.Equal(t, "no coupon configured", result.SkipReason)
	repo.AssertExpectations(t)
}

func TestApplyCoupon_CouponNotFound(t *testing.T) {
	t.Parallel()
	repo := new(mockCouponRepoForAutomation)
	handler := newTestCouponHandler(repo)

	repo.On("GetCouponByID", mock.Anything, "coupon-1", "user-1").
		Return((*models.Coupon)(nil), nil)

	automation := &models.Automation{
		ID:       "auto-1",
		UserID:   "user-1",
		CouponID: strPtr("coupon-1"),
	}

	result, err := handler.ApplyCoupon(context.Background(), automation, 100.0, "shopee")
	require.NoError(t, err)
	assert.False(t, result.Applied)
	assert.Equal(t, "coupon not found", result.SkipReason)
	repo.AssertExpectations(t)
}

func TestApplyCoupon_RepositoryError(t *testing.T) {
	t.Parallel()
	repo := new(mockCouponRepoForAutomation)
	handler := newTestCouponHandler(repo)

	repo.On("GetCouponByID", mock.Anything, "coupon-1", "user-1").
		Return((*models.Coupon)(nil), fmt.Errorf("db connection failed"))

	automation := &models.Automation{
		ID:       "auto-1",
		UserID:   "user-1",
		CouponID: strPtr("coupon-1"),
	}

	result, err := handler.ApplyCoupon(context.Background(), automation, 100.0, "shopee")
	require.NoError(t, err) // does not propagate error, just skips
	assert.False(t, result.Applied)
	assert.Equal(t, "coupon retrieval failed", result.SkipReason)
	repo.AssertExpectations(t)
}

func TestApplyCoupon_CouponInactive(t *testing.T) {
	t.Parallel()
	repo := new(mockCouponRepoForAutomation)
	handler := newTestCouponHandler(repo)

	coupon := &models.Coupon{
		ID:       "coupon-1",
		UserID:   "user-1",
		Code:     "SAVE10",
		Store:    "shopee",
		IsActive: false,
	}
	repo.On("GetCouponByID", mock.Anything, "coupon-1", "user-1").Return(coupon, nil)

	automation := &models.Automation{
		ID:       "auto-1",
		UserID:   "user-1",
		CouponID: strPtr("coupon-1"),
	}

	result, err := handler.ApplyCoupon(context.Background(), automation, 100.0, "shopee")
	require.NoError(t, err)
	assert.False(t, result.Applied)
	assert.Equal(t, "coupon is inactive", result.SkipReason)
	assert.Equal(t, "SAVE10", result.CouponCode)
	repo.AssertExpectations(t)
}

func TestApplyCoupon_CouponExpired(t *testing.T) {
	t.Parallel()
	repo := new(mockCouponRepoForAutomation)
	handler := newTestCouponHandler(repo)

	pastTime := time.Now().Add(-48 * time.Hour).Format(time.RFC3339)
	coupon := &models.Coupon{
		ID:        "coupon-1",
		UserID:    "user-1",
		Code:      "EXPIRED10",
		Store:     "shopee",
		IsActive:  true,
		ExpiresAt: &pastTime,
	}
	repo.On("GetCouponByID", mock.Anything, "coupon-1", "user-1").Return(coupon, nil)

	automation := &models.Automation{
		ID:       "auto-1",
		UserID:   "user-1",
		CouponID: strPtr("coupon-1"),
	}

	result, err := handler.ApplyCoupon(context.Background(), automation, 100.0, "shopee")
	require.NoError(t, err)
	assert.False(t, result.Applied)
	assert.Equal(t, "coupon is expired", result.SkipReason)
	repo.AssertExpectations(t)
}

func TestApplyCoupon_NoExpirationPassesThrough(t *testing.T) {
	t.Parallel()
	repo := new(mockCouponRepoForAutomation)
	handler := newTestCouponHandler(repo)

	coupon := &models.Coupon{
		ID:        "coupon-1",
		UserID:    "user-1",
		Code:      "NOEXPIRY",
		Store:     "shopee",
		IsActive:  true,
		ExpiresAt: nil, // no expiration
	}
	repo.On("GetCouponByID", mock.Anything, "coupon-1", "user-1").Return(coupon, nil)

	automation := &models.Automation{
		ID:       "auto-1",
		UserID:   "user-1",
		CouponID: strPtr("coupon-1"),
	}

	result, err := handler.ApplyCoupon(context.Background(), automation, 100.0, "shopee")
	require.NoError(t, err)
	assert.True(t, result.Applied)
	assert.Empty(t, result.SkipReason)
	repo.AssertExpectations(t)
}

func TestApplyCoupon_StoreMismatch(t *testing.T) {
	t.Parallel()
	repo := new(mockCouponRepoForAutomation)
	handler := newTestCouponHandler(repo)

	coupon := &models.Coupon{
		ID:       "coupon-1",
		UserID:   "user-1",
		Code:     "SHOPEE20",
		Store:    "shopee",
		IsActive: true,
	}
	repo.On("GetCouponByID", mock.Anything, "coupon-1", "user-1").Return(coupon, nil)

	automation := &models.Automation{
		ID:       "auto-1",
		UserID:   "user-1",
		CouponID: strPtr("coupon-1"),
	}

	result, err := handler.ApplyCoupon(context.Background(), automation, 100.0, "amazon")
	require.NoError(t, err)
	assert.False(t, result.Applied)
	assert.Contains(t, result.SkipReason, "coupon is for store")
	assert.Contains(t, result.SkipReason, "shopee")
	assert.Contains(t, result.SkipReason, "amazon")
	repo.AssertExpectations(t)
}

func TestApplyCoupon_BelowMinProductPrice(t *testing.T) {
	t.Parallel()
	repo := new(mockCouponRepoForAutomation)
	handler := newTestCouponHandler(repo)

	coupon := &models.Coupon{
		ID:              "coupon-1",
		UserID:          "user-1",
		Code:            "MIN50",
		Store:           "shopee",
		IsActive:        true,
		MinProductPrice: float64Ptr(50.0),
	}
	repo.On("GetCouponByID", mock.Anything, "coupon-1", "user-1").Return(coupon, nil)

	automation := &models.Automation{
		ID:       "auto-1",
		UserID:   "user-1",
		CouponID: strPtr("coupon-1"),
	}

	result, err := handler.ApplyCoupon(context.Background(), automation, 30.0, "shopee")
	require.NoError(t, err)
	assert.False(t, result.Applied)
	assert.Contains(t, result.SkipReason, "below minimum")
	assert.Contains(t, result.SkipReason, "R$ 30.00")
	assert.Contains(t, result.SkipReason, "R$ 50.00")
	repo.AssertExpectations(t)
}

func TestApplyCoupon_SuccessWithAllFields(t *testing.T) {
	t.Parallel()
	repo := new(mockCouponRepoForAutomation)
	handler := newTestCouponHandler(repo)

	futureTime := time.Now().Add(72 * time.Hour).Format(time.RFC3339)
	coupon := &models.Coupon{
		ID:              "coupon-1",
		UserID:          "user-1",
		Code:            "MEGA30",
		Store:           "shopee",
		IsActive:        true,
		ExpiresAt:       &futureTime,
		DiscountType:    strPtr("percent"),
		DiscountValue:   float64Ptr(30.0),
		MaxDiscount:     float64Ptr(15.0),
		MinProductPrice: float64Ptr(20.0),
		CouponText:      strPtr("Use no app!"),
		AffiliateLink:   strPtr("https://example.com/cupom"),
	}
	repo.On("GetCouponByID", mock.Anything, "coupon-1", "user-1").Return(coupon, nil)

	automation := &models.Automation{
		ID:       "auto-1",
		UserID:   "user-1",
		CouponID: strPtr("coupon-1"),
	}

	result, err := handler.ApplyCoupon(context.Background(), automation, 100.0, "shopee")
	require.NoError(t, err)
	assert.True(t, result.Applied)
	assert.Equal(t, "MEGA30", result.CouponCode)
	assert.Empty(t, result.SkipReason)
	assert.Contains(t, result.CouponText, "MEGA30")
	assert.Contains(t, result.CouponText, "30% OFF")
	assert.Contains(t, result.CouponText, "max R$ 15.00")
	assert.Contains(t, result.CouponText, "Use no app!")
	assert.Contains(t, result.CouponText, "https://example.com/cupom")
	repo.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// formatCouponText tests (tested indirectly via ApplyCoupon success paths)
// ---------------------------------------------------------------------------

func TestFormatCouponText_OnlyCode(t *testing.T) {
	t.Parallel()
	repo := new(mockCouponRepoForAutomation)
	handler := newTestCouponHandler(repo)

	coupon := &models.Coupon{
		ID:       "coupon-1",
		UserID:   "user-1",
		Code:     "SIMPLE",
		Store:    "shopee",
		IsActive: true,
	}
	repo.On("GetCouponByID", mock.Anything, "coupon-1", "user-1").Return(coupon, nil)

	automation := &models.Automation{
		ID:       "auto-1",
		UserID:   "user-1",
		CouponID: strPtr("coupon-1"),
	}

	result, err := handler.ApplyCoupon(context.Background(), automation, 100.0, "shopee")
	require.NoError(t, err)
	assert.True(t, result.Applied)
	assert.Contains(t, result.CouponText, "CUPOM: SIMPLE")
	// No discount, no extra text, no link
	assert.NotContains(t, result.CouponText, "Desconto")
	assert.NotContains(t, result.CouponText, "Link do cupom")
	repo.AssertExpectations(t)
}

func TestFormatCouponText_FixedDiscount(t *testing.T) {
	t.Parallel()
	repo := new(mockCouponRepoForAutomation)
	handler := newTestCouponHandler(repo)

	coupon := &models.Coupon{
		ID:            "coupon-1",
		UserID:        "user-1",
		Code:          "FIXED5",
		Store:         "shopee",
		IsActive:      true,
		DiscountType:  strPtr("fixed"),
		DiscountValue: float64Ptr(5.50),
	}
	repo.On("GetCouponByID", mock.Anything, "coupon-1", "user-1").Return(coupon, nil)

	automation := &models.Automation{
		ID:       "auto-1",
		UserID:   "user-1",
		CouponID: strPtr("coupon-1"),
	}

	result, err := handler.ApplyCoupon(context.Background(), automation, 100.0, "shopee")
	require.NoError(t, err)
	assert.True(t, result.Applied)
	assert.Contains(t, result.CouponText, "R$ 5.50 OFF")
	repo.AssertExpectations(t)
}

func TestFormatCouponText_PercentDiscountWithMax(t *testing.T) {
	t.Parallel()
	repo := new(mockCouponRepoForAutomation)
	handler := newTestCouponHandler(repo)

	coupon := &models.Coupon{
		ID:            "coupon-1",
		UserID:        "user-1",
		Code:          "PCT20",
		Store:         "shopee",
		IsActive:      true,
		DiscountType:  strPtr("percent"),
		DiscountValue: float64Ptr(20.0),
		MaxDiscount:   float64Ptr(10.0),
	}
	repo.On("GetCouponByID", mock.Anything, "coupon-1", "user-1").Return(coupon, nil)

	automation := &models.Automation{
		ID:       "auto-1",
		UserID:   "user-1",
		CouponID: strPtr("coupon-1"),
	}

	result, err := handler.ApplyCoupon(context.Background(), automation, 100.0, "shopee")
	require.NoError(t, err)
	assert.True(t, result.Applied)
	assert.Contains(t, result.CouponText, "20% OFF")
	assert.Contains(t, result.CouponText, "max R$ 10.00")
	repo.AssertExpectations(t)
}

func TestFormatCouponText_WithCouponText(t *testing.T) {
	t.Parallel()
	repo := new(mockCouponRepoForAutomation)
	handler := newTestCouponHandler(repo)

	coupon := &models.Coupon{
		ID:         "coupon-1",
		UserID:     "user-1",
		Code:       "EXTRA",
		Store:      "shopee",
		IsActive:   true,
		CouponText: strPtr("Valido apenas no app"),
	}
	repo.On("GetCouponByID", mock.Anything, "coupon-1", "user-1").Return(coupon, nil)

	automation := &models.Automation{
		ID:       "auto-1",
		UserID:   "user-1",
		CouponID: strPtr("coupon-1"),
	}

	result, err := handler.ApplyCoupon(context.Background(), automation, 100.0, "shopee")
	require.NoError(t, err)
	assert.True(t, result.Applied)
	assert.Contains(t, result.CouponText, "Valido apenas no app")
	repo.AssertExpectations(t)
}

func TestFormatCouponText_WithAffiliateLink(t *testing.T) {
	t.Parallel()
	repo := new(mockCouponRepoForAutomation)
	handler := newTestCouponHandler(repo)

	coupon := &models.Coupon{
		ID:            "coupon-1",
		UserID:        "user-1",
		Code:          "LINK10",
		Store:         "shopee",
		IsActive:      true,
		AffiliateLink: strPtr("https://shopee.com.br/cupom"),
	}
	repo.On("GetCouponByID", mock.Anything, "coupon-1", "user-1").Return(coupon, nil)

	automation := &models.Automation{
		ID:       "auto-1",
		UserID:   "user-1",
		CouponID: strPtr("coupon-1"),
	}

	result, err := handler.ApplyCoupon(context.Background(), automation, 100.0, "shopee")
	require.NoError(t, err)
	assert.True(t, result.Applied)
	assert.Contains(t, result.CouponText, "Link do cupom: https://shopee.com.br/cupom")
	repo.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// LoadCouponOnce tests
// ---------------------------------------------------------------------------

func TestLoadCouponOnce_NoCouponConfigured(t *testing.T) {
	t.Parallel()
	repo := new(mockCouponRepoForAutomation)
	handler := newTestCouponHandler(repo)

	automation := &models.Automation{
		ID:       "auto-1",
		UserID:   "user-1",
		CouponID: nil,
	}

	coupon, reason := handler.LoadCouponOnce(context.Background(), automation)
	assert.Nil(t, coupon)
	assert.Empty(t, reason)
	repo.AssertExpectations(t)
}

func TestLoadCouponOnce_NotFound(t *testing.T) {
	t.Parallel()
	repo := new(mockCouponRepoForAutomation)
	handler := newTestCouponHandler(repo)

	repo.On("GetCouponByID", mock.Anything, "coupon-1", "user-1").
		Return((*models.Coupon)(nil), nil)

	automation := &models.Automation{
		ID:       "auto-1",
		UserID:   "user-1",
		CouponID: strPtr("coupon-1"),
	}

	coupon, reason := handler.LoadCouponOnce(context.Background(), automation)
	assert.Nil(t, coupon)
	assert.Equal(t, "Cupom nao encontrado", reason)
	repo.AssertExpectations(t)
}

func TestLoadCouponOnce_Inactive(t *testing.T) {
	t.Parallel()
	repo := new(mockCouponRepoForAutomation)
	handler := newTestCouponHandler(repo)

	coupon := &models.Coupon{
		ID:       "coupon-1",
		UserID:   "user-1",
		Code:     "INACTIVE",
		Store:    "shopee",
		IsActive: false,
	}
	repo.On("GetCouponByID", mock.Anything, "coupon-1", "user-1").Return(coupon, nil)

	automation := &models.Automation{
		ID:       "auto-1",
		UserID:   "user-1",
		CouponID: strPtr("coupon-1"),
	}

	result, reason := handler.LoadCouponOnce(context.Background(), automation)
	assert.Nil(t, result)
	assert.Equal(t, "Cupom inativo", reason)
	repo.AssertExpectations(t)
}

func TestLoadCouponOnce_Expired(t *testing.T) {
	t.Parallel()
	repo := new(mockCouponRepoForAutomation)
	handler := newTestCouponHandler(repo)

	pastTime := time.Now().Add(-24 * time.Hour).Format(time.RFC3339)
	coupon := &models.Coupon{
		ID:        "coupon-1",
		UserID:    "user-1",
		Code:      "EXPIRED",
		Store:     "shopee",
		IsActive:  true,
		ExpiresAt: &pastTime,
	}
	repo.On("GetCouponByID", mock.Anything, "coupon-1", "user-1").Return(coupon, nil)

	automation := &models.Automation{
		ID:       "auto-1",
		UserID:   "user-1",
		CouponID: strPtr("coupon-1"),
	}

	result, reason := handler.LoadCouponOnce(context.Background(), automation)
	assert.Nil(t, result)
	assert.Equal(t, "Cupom expirado", reason)
	repo.AssertExpectations(t)
}

func TestLoadCouponOnce_Valid(t *testing.T) {
	t.Parallel()
	repo := new(mockCouponRepoForAutomation)
	handler := newTestCouponHandler(repo)

	futureTime := time.Now().Add(48 * time.Hour).Format(time.RFC3339)
	coupon := &models.Coupon{
		ID:        "coupon-1",
		UserID:    "user-1",
		Code:      "VALID10",
		Store:     "shopee",
		IsActive:  true,
		ExpiresAt: &futureTime,
	}
	repo.On("GetCouponByID", mock.Anything, "coupon-1", "user-1").Return(coupon, nil)

	automation := &models.Automation{
		ID:       "auto-1",
		UserID:   "user-1",
		CouponID: strPtr("coupon-1"),
	}

	result, reason := handler.LoadCouponOnce(context.Background(), automation)
	require.NotNil(t, result)
	assert.Equal(t, "VALID10", result.Code)
	assert.Empty(t, reason)
	repo.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// ApplyCouponWithPreloaded tests
// ---------------------------------------------------------------------------

func TestApplyCouponWithPreloaded_NilCoupon(t *testing.T) {
	t.Parallel()
	repo := new(mockCouponRepoForAutomation)
	handler := newTestCouponHandler(repo)

	result := handler.ApplyCouponWithPreloaded(nil, 100.0, "shopee")
	assert.False(t, result.Applied)
	assert.Equal(t, "no coupon configured", result.SkipReason)
}

func TestApplyCouponWithPreloaded_StoreMismatch(t *testing.T) {
	t.Parallel()
	repo := new(mockCouponRepoForAutomation)
	handler := newTestCouponHandler(repo)

	coupon := &models.Coupon{
		ID:       "coupon-1",
		Code:     "SHOPEE10",
		Store:    "shopee",
		IsActive: true,
	}

	result := handler.ApplyCouponWithPreloaded(coupon, 100.0, "amazon")
	assert.False(t, result.Applied)
	assert.Contains(t, result.SkipReason, "coupon is for store shopee")
	assert.Equal(t, "SHOPEE10", result.CouponCode)
}

func TestApplyCouponWithPreloaded_BelowPrice(t *testing.T) {
	t.Parallel()
	repo := new(mockCouponRepoForAutomation)
	handler := newTestCouponHandler(repo)

	coupon := &models.Coupon{
		ID:              "coupon-1",
		Code:            "MINPRICE",
		Store:           "shopee",
		IsActive:        true,
		MinProductPrice: float64Ptr(100.0),
	}

	result := handler.ApplyCouponWithPreloaded(coupon, 50.0, "shopee")
	assert.False(t, result.Applied)
	assert.Contains(t, result.SkipReason, "below minimum")
	assert.Contains(t, result.SkipReason, "R$ 50.00")
}

func TestApplyCouponWithPreloaded_Success(t *testing.T) {
	t.Parallel()
	repo := new(mockCouponRepoForAutomation)
	handler := newTestCouponHandler(repo)

	coupon := &models.Coupon{
		ID:            "coupon-1",
		Code:          "PRELOAD20",
		Store:         "shopee",
		IsActive:      true,
		DiscountType:  strPtr("fixed"),
		DiscountValue: float64Ptr(20.0),
	}

	result := handler.ApplyCouponWithPreloaded(coupon, 100.0, "shopee")
	assert.True(t, result.Applied)
	assert.Equal(t, "PRELOAD20", result.CouponCode)
	assert.Empty(t, result.SkipReason)
	assert.Contains(t, result.CouponText, "CUPOM: PRELOAD20")
	assert.Contains(t, result.CouponText, "R$ 20.00 OFF")
}

// ---------------------------------------------------------------------------
// Edge cases
// ---------------------------------------------------------------------------

func TestApplyCoupon_EmptyStoresMatchAny(t *testing.T) {
	t.Parallel()
	repo := new(mockCouponRepoForAutomation)
	handler := newTestCouponHandler(repo)

	// Coupon with empty store should match any offer store
	coupon := &models.Coupon{
		ID:       "coupon-1",
		UserID:   "user-1",
		Code:     "UNIVERSAL",
		Store:    "",
		IsActive: true,
	}
	repo.On("GetCouponByID", mock.Anything, "coupon-1", "user-1").Return(coupon, nil)

	automation := &models.Automation{
		ID:       "auto-1",
		UserID:   "user-1",
		CouponID: strPtr("coupon-1"),
	}

	result, err := handler.ApplyCoupon(context.Background(), automation, 100.0, "amazon")
	require.NoError(t, err)
	assert.True(t, result.Applied)
	repo.AssertExpectations(t)
}

func TestApplyCouponWithPreloaded_EmptyStoresMatchAny(t *testing.T) {
	t.Parallel()
	repo := new(mockCouponRepoForAutomation)
	handler := newTestCouponHandler(repo)

	coupon := &models.Coupon{
		ID:       "coupon-1",
		Code:     "UNIVERSAL",
		Store:    "",
		IsActive: true,
	}

	result := handler.ApplyCouponWithPreloaded(coupon, 100.0, "ml")
	assert.True(t, result.Applied)
}

func TestFormatCouponText_PercentDiscountWithoutMax(t *testing.T) {
	t.Parallel()
	repo := new(mockCouponRepoForAutomation)
	handler := newTestCouponHandler(repo)

	coupon := &models.Coupon{
		ID:            "coupon-1",
		UserID:        "user-1",
		Code:          "PCT15",
		Store:         "shopee",
		IsActive:      true,
		DiscountType:  strPtr("percent"),
		DiscountValue: float64Ptr(15.0),
		MaxDiscount:   nil, // no max
	}
	repo.On("GetCouponByID", mock.Anything, "coupon-1", "user-1").Return(coupon, nil)

	automation := &models.Automation{
		ID:       "auto-1",
		UserID:   "user-1",
		CouponID: strPtr("coupon-1"),
	}

	result, err := handler.ApplyCoupon(context.Background(), automation, 100.0, "shopee")
	require.NoError(t, err)
	assert.True(t, result.Applied)
	assert.Contains(t, result.CouponText, "15% OFF")
	// Should NOT contain "max" since MaxDiscount is nil
	lines := strings.Split(result.CouponText, "\n")
	for _, line := range lines {
		if strings.Contains(line, "OFF") {
			assert.NotContains(t, line, "max")
		}
	}
	repo.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// CalculateDiscountedPrice tests
// ---------------------------------------------------------------------------

func TestCalculateDiscountedPrice_NilCoupon(t *testing.T) {
	t.Parallel()
	assert.InDelta(t, 150.0, CalculateDiscountedPrice(150, nil), 0.01)
}

func TestCalculateDiscountedPrice_FixedDiscount(t *testing.T) {
	t.Parallel()
	coupon := &models.Coupon{
		Code:          "FIX15",
		DiscountType:  strPtr("fixed"),
		DiscountValue: float64Ptr(15),
	}
	assert.InDelta(t, 135.0, CalculateDiscountedPrice(150, coupon), 0.01)
}

func TestCalculateDiscountedPrice_PercentDiscount(t *testing.T) {
	t.Parallel()
	coupon := &models.Coupon{
		Code:          "PCT10",
		DiscountType:  strPtr("percent"),
		DiscountValue: float64Ptr(10),
	}
	assert.InDelta(t, 135.0, CalculateDiscountedPrice(150, coupon), 0.01)
}

func TestCalculateDiscountedPrice_PercentWithMaxCap(t *testing.T) {
	t.Parallel()
	coupon := &models.Coupon{
		Code:          "PCT50MAX20",
		DiscountType:  strPtr("percent"),
		DiscountValue: float64Ptr(50),
		MaxDiscount:   float64Ptr(20),
	}
	assert.InDelta(t, 180.0, CalculateDiscountedPrice(200, coupon), 0.01)
}

func TestCalculateDiscountedPrice_FixedExceedsPriceReturnsZero(t *testing.T) {
	t.Parallel()
	coupon := &models.Coupon{
		Code:          "FIX50",
		DiscountType:  strPtr("fixed"),
		DiscountValue: float64Ptr(50),
	}
	assert.InDelta(t, 0.0, CalculateDiscountedPrice(10, coupon), 0.01)
}

func TestCalculateDiscountedPrice_NoDiscountTypeReturnsOriginal(t *testing.T) {
	t.Parallel()
	coupon := &models.Coupon{
		Code: "NODISCOUNT",
	}
	assert.InDelta(t, 100.0, CalculateDiscountedPrice(100, coupon), 0.01)
}

func TestApplyCouponWithPreloaded_PopulatesDiscountedPrice(t *testing.T) {
	t.Parallel()
	handler := newTestCouponHandler(new(mockCouponRepoForAutomation))

	coupon := &models.Coupon{
		ID:            "coupon-1",
		Code:          "PCT10",
		Store:         "ml",
		IsActive:      true,
		DiscountType:  strPtr("percent"),
		DiscountValue: float64Ptr(10),
	}

	result := handler.ApplyCouponWithPreloaded(coupon, 150.0, "ml")
	assert.True(t, result.Applied)
	assert.InDelta(t, 135.0, result.DiscountedPrice, 0.01)
}

func TestLoadCouponOnce_RepoError(t *testing.T) {
	t.Parallel()
	repo := new(mockCouponRepoForAutomation)
	handler := newTestCouponHandler(repo)

	repo.On("GetCouponByID", mock.Anything, "coupon-1", "user-1").
		Return((*models.Coupon)(nil), fmt.Errorf("db connection failed"))

	automation := &models.Automation{
		ID:       "auto-1",
		UserID:   "user-1",
		CouponID: strPtr("coupon-1"),
	}

	coupon, reason := handler.LoadCouponOnce(context.Background(), automation)
	assert.Nil(t, coupon)
	assert.Equal(t, "Falha ao carregar cupom", reason)
	repo.AssertExpectations(t)
}
