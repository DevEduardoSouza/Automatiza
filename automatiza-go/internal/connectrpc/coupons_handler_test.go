package connectrpc

import (
	"context"
	"errors"
	"testing"

	"automatiza-go/internal/auth"
	couponsv1 "automatiza-go/internal/gen/coupons/v1"
	"automatiza-go/internal/models"
	"automatiza-go/internal/services/coupons"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Mock Implementation
// =============================================================================

type mockCouponsServiceForHandler struct {
	mock.Mock
}

func (m *mockCouponsServiceForHandler) GetUserCoupons(ctx context.Context, userID string, filters *coupons.CouponFilters) ([]models.Coupon, error) {
	args := m.Called(ctx, userID, filters)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Coupon), args.Error(1)
}

func (m *mockCouponsServiceForHandler) GetCouponByID(ctx context.Context, couponID, userID string) (*models.Coupon, error) {
	args := m.Called(ctx, couponID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Coupon), args.Error(1)
}

func (m *mockCouponsServiceForHandler) GetCouponsByStore(ctx context.Context, userID, store string) ([]models.Coupon, error) {
	args := m.Called(ctx, userID, store)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Coupon), args.Error(1)
}

func (m *mockCouponsServiceForHandler) CreateCoupon(ctx context.Context, userID string, data *coupons.CreateCouponData) (*models.Coupon, error) {
	args := m.Called(ctx, userID, data)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Coupon), args.Error(1)
}

func (m *mockCouponsServiceForHandler) UpdateCoupon(ctx context.Context, couponID, userID string, data *coupons.UpdateCouponData) (*models.Coupon, error) {
	args := m.Called(ctx, couponID, userID, data)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Coupon), args.Error(1)
}

func (m *mockCouponsServiceForHandler) DeleteCoupon(ctx context.Context, couponID, userID string) error {
	args := m.Called(ctx, couponID, userID)
	return args.Error(0)
}

func (m *mockCouponsServiceForHandler) GetActiveCoupons(ctx context.Context, userID string) ([]models.Coupon, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Coupon), args.Error(1)
}

func (m *mockCouponsServiceForHandler) ValidateCoupon(ctx context.Context, couponID, userID string) (*coupons.CouponValidation, error) {
	args := m.Called(ctx, couponID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*coupons.CouponValidation), args.Error(1)
}

// =============================================================================
// Helper Functions
// =============================================================================

func createTestUserContextForCoupons(userID string) context.Context {
	user := &auth.User{
		ID:    userID,
		Email: "test@example.com",
	}
	return context.WithValue(context.Background(), UserContextKey, user)
}

func ptrString(s string) *string    { return &s }
func ptrFloat64(f float64) *float64 { return &f }

// =============================================================================
// ListCoupons Tests
// =============================================================================

func TestListCoupons_Unauthenticated(t *testing.T) {
	t.Parallel()
	mockSvc := new(mockCouponsServiceForHandler)
	handler := NewCouponsHandler(mockSvc, nil)

	ctx := context.Background()

	req := connect.NewRequest(&couponsv1.ListCouponsRequest{})
	_, err := handler.ListCoupons(ctx, req)

	require.Error(t, err)
	connectErr, ok := err.(*connect.Error)
	require.True(t, ok)
	assert.Equal(t, connect.CodeUnauthenticated, connectErr.Code())
}

func TestListCoupons_SuccessWithoutFilter(t *testing.T) {
	t.Parallel()
	mockSvc := new(mockCouponsServiceForHandler)
	handler := NewCouponsHandler(mockSvc, nil)

	userID := "user-123"
	ctx := createTestUserContextForCoupons(userID)

	expectedCoupons := []models.Coupon{
		{
			ID:        "coupon-1",
			UserID:    userID,
			Code:      "SAVE10",
			Title:     "10% Off",
			Store:     "shopee",
			IsActive:  true,
			CreatedAt: "2026-01-01T00:00:00Z",
			UpdatedAt: "2026-01-01T00:00:00Z",
		},
		{
			ID:        "coupon-2",
			UserID:    userID,
			Code:      "SAVE20",
			Title:     "20% Off",
			Store:     "amazon",
			IsActive:  false,
			CreatedAt: "2026-01-01T00:00:00Z",
			UpdatedAt: "2026-01-01T00:00:00Z",
		},
	}

	mockSvc.On("GetUserCoupons", mock.Anything, userID, (*coupons.CouponFilters)(nil)).Return(expectedCoupons, nil)

	req := connect.NewRequest(&couponsv1.ListCouponsRequest{})
	resp, err := handler.ListCoupons(ctx, req)

	require.NoError(t, err)
	assert.Len(t, resp.Msg.Coupons, 2)
	assert.Equal(t, "coupon-1", resp.Msg.Coupons[0].Id)
	assert.Equal(t, "SAVE10", resp.Msg.Coupons[0].Code)
	assert.Equal(t, couponsv1.Store_STORE_SHOPEE, resp.Msg.Coupons[0].Store)
	assert.True(t, resp.Msg.Coupons[0].IsActive)
	assert.Equal(t, "coupon-2", resp.Msg.Coupons[1].Id)
	assert.False(t, resp.Msg.Coupons[1].IsActive)

	mockSvc.AssertExpectations(t)
}

func TestListCoupons_SuccessWithStoreFilter(t *testing.T) {
	t.Parallel()
	mockSvc := new(mockCouponsServiceForHandler)
	handler := NewCouponsHandler(mockSvc, nil)

	userID := "user-123"
	ctx := createTestUserContextForCoupons(userID)

	expectedCoupons := []models.Coupon{
		{
			ID:        "coupon-1",
			UserID:    userID,
			Code:      "SHOPEE10",
			Title:     "Shopee Discount",
			Store:     "shopee",
			IsActive:  true,
			CreatedAt: "2026-01-01T00:00:00Z",
			UpdatedAt: "2026-01-01T00:00:00Z",
		},
	}

	mockSvc.On("GetUserCoupons", mock.Anything, userID, mock.MatchedBy(func(f *coupons.CouponFilters) bool {
		return f != nil && f.Store == "shopee"
	})).Return(expectedCoupons, nil)

	storeFilter := couponsv1.Store_STORE_SHOPEE
	req := connect.NewRequest(&couponsv1.ListCouponsRequest{
		Filter: &couponsv1.CouponFilter{
			Store: &storeFilter,
		},
	})
	resp, err := handler.ListCoupons(ctx, req)

	require.NoError(t, err)
	assert.Len(t, resp.Msg.Coupons, 1)
	assert.Equal(t, "SHOPEE10", resp.Msg.Coupons[0].Code)

	mockSvc.AssertExpectations(t)
}

func TestListCoupons_InternalError(t *testing.T) {
	t.Parallel()
	mockSvc := new(mockCouponsServiceForHandler)
	handler := NewCouponsHandler(mockSvc, nil)

	userID := "user-123"
	ctx := createTestUserContextForCoupons(userID)

	mockSvc.On("GetUserCoupons", mock.Anything, userID, (*coupons.CouponFilters)(nil)).Return(nil, errors.New("database error"))

	req := connect.NewRequest(&couponsv1.ListCouponsRequest{})
	_, err := handler.ListCoupons(ctx, req)

	require.Error(t, err)
	connectErr, ok := err.(*connect.Error)
	require.True(t, ok)
	assert.Equal(t, connect.CodeInternal, connectErr.Code())

	mockSvc.AssertExpectations(t)
}

// =============================================================================
// GetCoupon Tests
// =============================================================================

func TestGetCoupon_Unauthenticated(t *testing.T) {
	t.Parallel()
	mockSvc := new(mockCouponsServiceForHandler)
	handler := NewCouponsHandler(mockSvc, nil)

	ctx := context.Background()

	req := connect.NewRequest(&couponsv1.GetCouponRequest{Id: "coupon-1"})
	_, err := handler.GetCoupon(ctx, req)

	require.Error(t, err)
	connectErr, ok := err.(*connect.Error)
	require.True(t, ok)
	assert.Equal(t, connect.CodeUnauthenticated, connectErr.Code())
}

func TestGetCoupon_NotFound(t *testing.T) {
	t.Parallel()
	mockSvc := new(mockCouponsServiceForHandler)
	handler := NewCouponsHandler(mockSvc, nil)

	userID := "user-123"
	ctx := createTestUserContextForCoupons(userID)

	mockSvc.On("GetCouponByID", mock.Anything, "coupon-nonexistent", userID).Return(nil, nil)

	req := connect.NewRequest(&couponsv1.GetCouponRequest{Id: "coupon-nonexistent"})
	_, err := handler.GetCoupon(ctx, req)

	require.Error(t, err)
	connectErr, ok := err.(*connect.Error)
	require.True(t, ok)
	assert.Equal(t, connect.CodeNotFound, connectErr.Code())

	mockSvc.AssertExpectations(t)
}

func TestGetCoupon_Success(t *testing.T) {
	t.Parallel()
	mockSvc := new(mockCouponsServiceForHandler)
	handler := NewCouponsHandler(mockSvc, nil)

	userID := "user-123"
	ctx := createTestUserContextForCoupons(userID)

	discountType := "percent"
	discountValue := 10.0
	expectedCoupon := &models.Coupon{
		ID:            "coupon-1",
		UserID:        userID,
		Code:          "SAVE10",
		Title:         "10% Off",
		Store:         "shopee",
		IsActive:      true,
		DiscountType:  &discountType,
		DiscountValue: &discountValue,
		CreatedAt:     "2026-01-01T00:00:00Z",
		UpdatedAt:     "2026-01-01T00:00:00Z",
	}

	mockSvc.On("GetCouponByID", mock.Anything, "coupon-1", userID).Return(expectedCoupon, nil)

	req := connect.NewRequest(&couponsv1.GetCouponRequest{Id: "coupon-1"})
	resp, err := handler.GetCoupon(ctx, req)

	require.NoError(t, err)
	assert.Equal(t, "coupon-1", resp.Msg.Coupon.Id)
	assert.Equal(t, "SAVE10", resp.Msg.Coupon.Code)
	assert.Equal(t, "10% Off", resp.Msg.Coupon.Title)
	assert.Equal(t, couponsv1.Store_STORE_SHOPEE, resp.Msg.Coupon.Store)
	assert.True(t, resp.Msg.Coupon.IsActive)
	assert.Equal(t, couponsv1.DiscountType_DISCOUNT_TYPE_PERCENT, resp.Msg.Coupon.DiscountType)

	mockSvc.AssertExpectations(t)
}

// =============================================================================
// CreateCoupon Tests
// =============================================================================

func TestCreateCoupon_Unauthenticated(t *testing.T) {
	t.Parallel()
	mockSvc := new(mockCouponsServiceForHandler)
	handler := NewCouponsHandler(mockSvc, nil)

	ctx := context.Background()

	req := connect.NewRequest(&couponsv1.CreateCouponRequest{
		Code:  "SAVE10",
		Title: "10% Off",
		Store: couponsv1.Store_STORE_SHOPEE,
	})
	_, err := handler.CreateCoupon(ctx, req)

	require.Error(t, err)
	connectErr, ok := err.(*connect.Error)
	require.True(t, ok)
	assert.Equal(t, connect.CodeUnauthenticated, connectErr.Code())
}

func TestCreateCoupon_Success(t *testing.T) {
	t.Parallel()
	mockSvc := new(mockCouponsServiceForHandler)
	handler := NewCouponsHandler(mockSvc, nil)

	userID := "user-123"
	ctx := createTestUserContextForCoupons(userID)

	createdCoupon := &models.Coupon{
		ID:        "coupon-new",
		UserID:    userID,
		Code:      "SAVE10",
		Title:     "10% Off",
		Store:     "shopee",
		IsActive:  true,
		CreatedAt: "2026-01-01T00:00:00Z",
		UpdatedAt: "2026-01-01T00:00:00Z",
	}

	mockSvc.On("CreateCoupon", mock.Anything, userID, mock.MatchedBy(func(data *coupons.CreateCouponData) bool {
		return data.Code == "SAVE10" && data.Title == "10% Off" && data.Store == "shopee"
	})).Return(createdCoupon, nil)

	req := connect.NewRequest(&couponsv1.CreateCouponRequest{
		Code:  "SAVE10",
		Title: "10% Off",
		Store: couponsv1.Store_STORE_SHOPEE,
	})
	resp, err := handler.CreateCoupon(ctx, req)

	require.NoError(t, err)
	assert.Equal(t, "coupon-new", resp.Msg.Coupon.Id)
	assert.Equal(t, "SAVE10", resp.Msg.Coupon.Code)
	assert.Equal(t, "10% Off", resp.Msg.Coupon.Title)
	assert.True(t, resp.Msg.Coupon.IsActive)

	mockSvc.AssertExpectations(t)
}

func TestCreateCoupon_DuplicateCodeError(t *testing.T) {
	t.Parallel()
	mockSvc := new(mockCouponsServiceForHandler)
	handler := NewCouponsHandler(mockSvc, nil)

	userID := "user-123"
	ctx := createTestUserContextForCoupons(userID)

	mockSvc.On("CreateCoupon", mock.Anything, userID, mock.Anything).Return(nil, &coupons.DuplicateCodeError{Code: "SAVE10"})

	req := connect.NewRequest(&couponsv1.CreateCouponRequest{
		Code:  "SAVE10",
		Title: "10% Off",
		Store: couponsv1.Store_STORE_SHOPEE,
	})
	_, err := handler.CreateCoupon(ctx, req)

	require.Error(t, err)
	connectErr, ok := err.(*connect.Error)
	require.True(t, ok)
	assert.Equal(t, connect.CodeAlreadyExists, connectErr.Code())

	mockSvc.AssertExpectations(t)
}

func TestCreateCoupon_InternalError(t *testing.T) {
	t.Parallel()
	mockSvc := new(mockCouponsServiceForHandler)
	handler := NewCouponsHandler(mockSvc, nil)

	userID := "user-123"
	ctx := createTestUserContextForCoupons(userID)

	mockSvc.On("CreateCoupon", mock.Anything, userID, mock.Anything).Return(nil, errors.New("database error"))

	req := connect.NewRequest(&couponsv1.CreateCouponRequest{
		Code:  "SAVE10",
		Title: "10% Off",
		Store: couponsv1.Store_STORE_SHOPEE,
	})
	_, err := handler.CreateCoupon(ctx, req)

	require.Error(t, err)
	connectErr, ok := err.(*connect.Error)
	require.True(t, ok)
	assert.Equal(t, connect.CodeInternal, connectErr.Code())

	mockSvc.AssertExpectations(t)
}

// =============================================================================
// UpdateCoupon Tests
// =============================================================================

func TestUpdateCoupon_Unauthenticated(t *testing.T) {
	t.Parallel()
	mockSvc := new(mockCouponsServiceForHandler)
	handler := NewCouponsHandler(mockSvc, nil)

	ctx := context.Background()

	req := connect.NewRequest(&couponsv1.UpdateCouponRequest{
		Id:   "coupon-1",
		Code: ptrString("NEWSAVE10"),
	})
	_, err := handler.UpdateCoupon(ctx, req)

	require.Error(t, err)
	connectErr, ok := err.(*connect.Error)
	require.True(t, ok)
	assert.Equal(t, connect.CodeUnauthenticated, connectErr.Code())
}

func TestUpdateCoupon_Success(t *testing.T) {
	t.Parallel()
	mockSvc := new(mockCouponsServiceForHandler)
	handler := NewCouponsHandler(mockSvc, nil)

	userID := "user-123"
	ctx := createTestUserContextForCoupons(userID)

	updatedCoupon := &models.Coupon{
		ID:        "coupon-1",
		UserID:    userID,
		Code:      "NEWSAVE10",
		Title:     "Updated Title",
		Store:     "shopee",
		IsActive:  true,
		CreatedAt: "2026-01-01T00:00:00Z",
		UpdatedAt: "2026-01-02T00:00:00Z",
	}

	mockSvc.On("UpdateCoupon", mock.Anything, "coupon-1", userID, mock.MatchedBy(func(data *coupons.UpdateCouponData) bool {
		return data.Code != nil && *data.Code == "NEWSAVE10" && data.Title != nil && *data.Title == "Updated Title"
	})).Return(updatedCoupon, nil)

	req := connect.NewRequest(&couponsv1.UpdateCouponRequest{
		Id:    "coupon-1",
		Code:  ptrString("NEWSAVE10"),
		Title: ptrString("Updated Title"),
	})
	resp, err := handler.UpdateCoupon(ctx, req)

	require.NoError(t, err)
	assert.Equal(t, "coupon-1", resp.Msg.Coupon.Id)
	assert.Equal(t, "NEWSAVE10", resp.Msg.Coupon.Code)
	assert.Equal(t, "Updated Title", resp.Msg.Coupon.Title)

	mockSvc.AssertExpectations(t)
}

func TestUpdateCoupon_DuplicateCodeError(t *testing.T) {
	t.Parallel()
	mockSvc := new(mockCouponsServiceForHandler)
	handler := NewCouponsHandler(mockSvc, nil)

	userID := "user-123"
	ctx := createTestUserContextForCoupons(userID)

	mockSvc.On("UpdateCoupon", mock.Anything, "coupon-1", userID, mock.Anything).Return(nil, &coupons.DuplicateCodeError{Code: "EXISTING"})

	req := connect.NewRequest(&couponsv1.UpdateCouponRequest{
		Id:   "coupon-1",
		Code: ptrString("EXISTING"),
	})
	_, err := handler.UpdateCoupon(ctx, req)

	require.Error(t, err)
	connectErr, ok := err.(*connect.Error)
	require.True(t, ok)
	assert.Equal(t, connect.CodeAlreadyExists, connectErr.Code())

	mockSvc.AssertExpectations(t)
}

func TestUpdateCoupon_CouponNotFoundError(t *testing.T) {
	t.Parallel()
	mockSvc := new(mockCouponsServiceForHandler)
	handler := NewCouponsHandler(mockSvc, nil)

	userID := "user-123"
	ctx := createTestUserContextForCoupons(userID)

	mockSvc.On("UpdateCoupon", mock.Anything, "coupon-nonexistent", userID, mock.Anything).Return(nil, &coupons.CouponNotFoundError{ID: "coupon-nonexistent"})

	req := connect.NewRequest(&couponsv1.UpdateCouponRequest{
		Id:    "coupon-nonexistent",
		Title: ptrString("New Title"),
	})
	_, err := handler.UpdateCoupon(ctx, req)

	require.Error(t, err)
	connectErr, ok := err.(*connect.Error)
	require.True(t, ok)
	assert.Equal(t, connect.CodeNotFound, connectErr.Code())

	mockSvc.AssertExpectations(t)
}

// =============================================================================
// DeleteCoupon Tests
// =============================================================================

func TestDeleteCoupon_Unauthenticated(t *testing.T) {
	t.Parallel()
	mockSvc := new(mockCouponsServiceForHandler)
	handler := NewCouponsHandler(mockSvc, nil)

	ctx := context.Background()

	req := connect.NewRequest(&couponsv1.DeleteCouponRequest{Id: "coupon-1"})
	_, err := handler.DeleteCoupon(ctx, req)

	require.Error(t, err)
	connectErr, ok := err.(*connect.Error)
	require.True(t, ok)
	assert.Equal(t, connect.CodeUnauthenticated, connectErr.Code())
}

func TestDeleteCoupon_Success(t *testing.T) {
	t.Parallel()
	mockSvc := new(mockCouponsServiceForHandler)
	handler := NewCouponsHandler(mockSvc, nil)

	userID := "user-123"
	ctx := createTestUserContextForCoupons(userID)

	mockSvc.On("DeleteCoupon", mock.Anything, "coupon-1", userID).Return(nil)

	req := connect.NewRequest(&couponsv1.DeleteCouponRequest{Id: "coupon-1"})
	_, err := handler.DeleteCoupon(ctx, req)

	require.NoError(t, err)

	mockSvc.AssertExpectations(t)
}

func TestDeleteCoupon_CouponNotFoundError(t *testing.T) {
	t.Parallel()
	mockSvc := new(mockCouponsServiceForHandler)
	handler := NewCouponsHandler(mockSvc, nil)

	userID := "user-123"
	ctx := createTestUserContextForCoupons(userID)

	mockSvc.On("DeleteCoupon", mock.Anything, "coupon-nonexistent", userID).Return(&coupons.CouponNotFoundError{ID: "coupon-nonexistent"})

	req := connect.NewRequest(&couponsv1.DeleteCouponRequest{Id: "coupon-nonexistent"})
	_, err := handler.DeleteCoupon(ctx, req)

	require.Error(t, err)
	connectErr, ok := err.(*connect.Error)
	require.True(t, ok)
	assert.Equal(t, connect.CodeNotFound, connectErr.Code())

	mockSvc.AssertExpectations(t)
}

// =============================================================================
// ListCouponsByStore Tests
// =============================================================================

func TestListCouponsByStore_Success(t *testing.T) {
	t.Parallel()
	mockSvc := new(mockCouponsServiceForHandler)
	handler := NewCouponsHandler(mockSvc, nil)

	userID := "user-123"
	ctx := createTestUserContextForCoupons(userID)

	expectedCoupons := []models.Coupon{
		{
			ID:        "coupon-1",
			UserID:    userID,
			Code:      "AMAZON10",
			Title:     "Amazon Discount",
			Store:     "amazon",
			IsActive:  true,
			CreatedAt: "2026-01-01T00:00:00Z",
			UpdatedAt: "2026-01-01T00:00:00Z",
		},
	}

	mockSvc.On("GetCouponsByStore", mock.Anything, userID, "amazon").Return(expectedCoupons, nil)

	req := connect.NewRequest(&couponsv1.ListCouponsByStoreRequest{
		Store: couponsv1.Store_STORE_AMAZON,
	})
	resp, err := handler.ListCouponsByStore(ctx, req)

	require.NoError(t, err)
	assert.Len(t, resp.Msg.Coupons, 1)
	assert.Equal(t, "AMAZON10", resp.Msg.Coupons[0].Code)
	assert.Equal(t, couponsv1.Store_STORE_AMAZON, resp.Msg.Coupons[0].Store)

	mockSvc.AssertExpectations(t)
}

// =============================================================================
// Helper Function Tests: couponToProto
// =============================================================================

func TestCouponToProto_NilInput(t *testing.T) {
	t.Parallel()
	result := couponToProto(nil)
	assert.Nil(t, result)
}

func TestCouponToProto_AllFieldsPopulated(t *testing.T) {
	t.Parallel()
	affiliateLink := "https://shopee.com.br/ref"
	expiresAt := "2026-12-31T23:59:59Z"
	category := "electronics"
	discountType := "percent"
	discountValue := 15.5
	maxDiscount := 50.0
	minProductPrice := 100.0
	couponText := "Use code SAVE15 for 15% off!"

	coupon := &models.Coupon{
		ID:              "coupon-1",
		UserID:          "user-123",
		Code:            "SAVE15",
		Title:           "15% Off Electronics",
		Store:           "shopee",
		AffiliateLink:   &affiliateLink,
		ExpiresAt:       &expiresAt,
		IsActive:        true,
		Category:        &category,
		DiscountType:    &discountType,
		DiscountValue:   &discountValue,
		MaxDiscount:     &maxDiscount,
		MinProductPrice: &minProductPrice,
		CouponText:      &couponText,
		CreatedAt:       "2026-01-01T00:00:00Z",
		UpdatedAt:       "2026-01-15T00:00:00Z",
	}

	result := couponToProto(coupon)

	require.NotNil(t, result)
	assert.Equal(t, "coupon-1", result.Id)
	assert.Equal(t, "SAVE15", result.Code)
	assert.Equal(t, "15% Off Electronics", result.Title)
	assert.Equal(t, couponsv1.Store_STORE_SHOPEE, result.Store)
	assert.True(t, result.IsActive)
	assert.Equal(t, "2026-01-01T00:00:00Z", result.CreatedAt)
	assert.Equal(t, "2026-01-15T00:00:00Z", result.UpdatedAt)

	require.NotNil(t, result.AffiliateLink)
	assert.Equal(t, "https://shopee.com.br/ref", *result.AffiliateLink)
	require.NotNil(t, result.ExpiresAt)
	assert.Equal(t, "2026-12-31T23:59:59Z", *result.ExpiresAt)
	require.NotNil(t, result.Category)
	assert.Equal(t, "electronics", *result.Category)
	assert.Equal(t, couponsv1.DiscountType_DISCOUNT_TYPE_PERCENT, result.DiscountType)
	require.NotNil(t, result.DiscountValue)
	assert.Equal(t, 15.5, *result.DiscountValue)
	require.NotNil(t, result.MaxDiscount)
	assert.Equal(t, 50.0, *result.MaxDiscount)
	require.NotNil(t, result.MinProductPrice)
	assert.Equal(t, 100.0, *result.MinProductPrice)
	require.NotNil(t, result.CouponText)
	assert.Equal(t, "Use code SAVE15 for 15% off!", *result.CouponText)
}

func TestCouponToProto_MinimalFields(t *testing.T) {
	t.Parallel()
	coupon := &models.Coupon{
		ID:        "coupon-1",
		UserID:    "user-123",
		Code:      "BASIC",
		Title:     "Basic Coupon",
		Store:     "amazon",
		IsActive:  false,
		CreatedAt: "2026-01-01T00:00:00Z",
		UpdatedAt: "2026-01-01T00:00:00Z",
	}

	result := couponToProto(coupon)

	require.NotNil(t, result)
	assert.Equal(t, "coupon-1", result.Id)
	assert.Equal(t, "BASIC", result.Code)
	assert.Equal(t, "Basic Coupon", result.Title)
	assert.Equal(t, couponsv1.Store_STORE_AMAZON, result.Store)
	assert.False(t, result.IsActive)

	assert.Nil(t, result.AffiliateLink)
	assert.Nil(t, result.ExpiresAt)
	assert.Nil(t, result.Category)
	assert.Equal(t, couponsv1.DiscountType_DISCOUNT_TYPE_UNSPECIFIED, result.DiscountType)
	assert.Nil(t, result.DiscountValue)
	assert.Nil(t, result.MaxDiscount)
	assert.Nil(t, result.MinProductPrice)
	assert.Nil(t, result.CouponText)
}

// =============================================================================
// Helper Function Tests: modelStoreToProtoCoupon round-trip
// =============================================================================

func TestModelStoreToProtoCoupon_AllStores(t *testing.T) {
	t.Parallel()
	tests := []struct {
		model string
		proto couponsv1.Store
	}{
		{"shopee", couponsv1.Store_STORE_SHOPEE},
		{"amazon", couponsv1.Store_STORE_AMAZON},
		{"magalu", couponsv1.Store_STORE_MAGALU},
		{"ml", couponsv1.Store_STORE_MERCADO_LIVRE},
		{"shein", couponsv1.Store_STORE_SHEIN},
		{"aliexpress", couponsv1.Store_STORE_ALIEXPRESS},
		{"awin", couponsv1.Store_STORE_AWIN},
		{"unknown", couponsv1.Store_STORE_UNSPECIFIED},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			t.Parallel()
			result := modelStoreToProtoCoupon(tt.model)
			assert.Equal(t, tt.proto, result)
		})
	}
}

func TestProtoStoreToModelCoupon_RoundTrip(t *testing.T) {
	t.Parallel()
	tests := []struct {
		proto couponsv1.Store
		model string
	}{
		{couponsv1.Store_STORE_SHOPEE, "shopee"},
		{couponsv1.Store_STORE_AMAZON, "amazon"},
		{couponsv1.Store_STORE_MAGALU, "magalu"},
		{couponsv1.Store_STORE_MERCADO_LIVRE, "ml"},
		{couponsv1.Store_STORE_SHEIN, "shein"},
		{couponsv1.Store_STORE_ALIEXPRESS, "aliexpress"},
		{couponsv1.Store_STORE_AWIN, "awin"},
		{couponsv1.Store_STORE_UNSPECIFIED, ""},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			t.Parallel()
			result := protoStoreToModelCoupon(tt.proto)
			assert.Equal(t, tt.model, result)
		})
	}
}

// =============================================================================
// Helper Function Tests: modelDiscountTypeToProto round-trip
// =============================================================================

func TestModelDiscountTypeToProto_RoundTrip(t *testing.T) {
	t.Parallel()
	tests := []struct {
		model string
		proto couponsv1.DiscountType
	}{
		{"fixed", couponsv1.DiscountType_DISCOUNT_TYPE_FIXED},
		{"percent", couponsv1.DiscountType_DISCOUNT_TYPE_PERCENT},
		{"unknown", couponsv1.DiscountType_DISCOUNT_TYPE_UNSPECIFIED},
		{"", couponsv1.DiscountType_DISCOUNT_TYPE_UNSPECIFIED},
	}

	for _, tt := range tests {
		name := tt.model
		if name == "" {
			name = "empty"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			result := modelDiscountTypeToProto(tt.model)
			assert.Equal(t, tt.proto, result)
		})
	}
}

func TestProtoDiscountTypeToModel_RoundTrip(t *testing.T) {
	t.Parallel()
	tests := []struct {
		proto couponsv1.DiscountType
		model string
	}{
		{couponsv1.DiscountType_DISCOUNT_TYPE_FIXED, "fixed"},
		{couponsv1.DiscountType_DISCOUNT_TYPE_PERCENT, "percent"},
		{couponsv1.DiscountType_DISCOUNT_TYPE_UNSPECIFIED, ""},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			t.Parallel()
			result := protoDiscountTypeToModel(tt.proto)
			assert.Equal(t, tt.model, result)
		})
	}
}

// =============================================================================
// Additional Handler Tests (missing coverage)
// =============================================================================

func TestListCouponsByStore_Unauthenticated(t *testing.T) {
	t.Parallel()
	mockSvc := new(mockCouponsServiceForHandler)
	handler := NewCouponsHandler(mockSvc, nil)

	ctx := context.Background()

	req := connect.NewRequest(&couponsv1.ListCouponsByStoreRequest{
		Store: couponsv1.Store_STORE_SHOPEE,
	})
	_, err := handler.ListCouponsByStore(ctx, req)

	require.Error(t, err)
	connectErr, ok := err.(*connect.Error)
	require.True(t, ok)
	assert.Equal(t, connect.CodeUnauthenticated, connectErr.Code())
}

func TestListCouponsByStore_InternalError(t *testing.T) {
	t.Parallel()
	mockSvc := new(mockCouponsServiceForHandler)
	handler := NewCouponsHandler(mockSvc, nil)

	userID := "user-123"
	ctx := createTestUserContextForCoupons(userID)

	mockSvc.On("GetCouponsByStore", mock.Anything, userID, "shopee").Return(nil, errors.New("database error"))

	req := connect.NewRequest(&couponsv1.ListCouponsByStoreRequest{
		Store: couponsv1.Store_STORE_SHOPEE,
	})
	_, err := handler.ListCouponsByStore(ctx, req)

	require.Error(t, err)
	connectErr, ok := err.(*connect.Error)
	require.True(t, ok)
	assert.Equal(t, connect.CodeInternal, connectErr.Code())

	mockSvc.AssertExpectations(t)
}

func TestGetCoupon_InternalError(t *testing.T) {
	t.Parallel()
	mockSvc := new(mockCouponsServiceForHandler)
	handler := NewCouponsHandler(mockSvc, nil)

	userID := "user-123"
	ctx := createTestUserContextForCoupons(userID)

	mockSvc.On("GetCouponByID", mock.Anything, "coupon-1", userID).Return(nil, errors.New("database error"))

	req := connect.NewRequest(&couponsv1.GetCouponRequest{Id: "coupon-1"})
	_, err := handler.GetCoupon(ctx, req)

	require.Error(t, err)
	connectErr, ok := err.(*connect.Error)
	require.True(t, ok)
	assert.Equal(t, connect.CodeInternal, connectErr.Code())

	mockSvc.AssertExpectations(t)
}

func TestDeleteCoupon_InternalError(t *testing.T) {
	t.Parallel()
	mockSvc := new(mockCouponsServiceForHandler)
	handler := NewCouponsHandler(mockSvc, nil)

	userID := "user-123"
	ctx := createTestUserContextForCoupons(userID)

	mockSvc.On("DeleteCoupon", mock.Anything, "coupon-1", userID).Return(errors.New("database error"))

	req := connect.NewRequest(&couponsv1.DeleteCouponRequest{Id: "coupon-1"})
	_, err := handler.DeleteCoupon(ctx, req)

	require.Error(t, err)
	connectErr, ok := err.(*connect.Error)
	require.True(t, ok)
	assert.Equal(t, connect.CodeInternal, connectErr.Code())

	mockSvc.AssertExpectations(t)
}

func TestUpdateCoupon_InternalError(t *testing.T) {
	t.Parallel()
	mockSvc := new(mockCouponsServiceForHandler)
	handler := NewCouponsHandler(mockSvc, nil)

	userID := "user-123"
	ctx := createTestUserContextForCoupons(userID)

	mockSvc.On("UpdateCoupon", mock.Anything, "coupon-1", userID, mock.Anything).Return(nil, errors.New("database error"))

	req := connect.NewRequest(&couponsv1.UpdateCouponRequest{
		Id:    "coupon-1",
		Title: ptrString("New Title"),
	})
	_, err := handler.UpdateCoupon(ctx, req)

	require.Error(t, err)
	connectErr, ok := err.(*connect.Error)
	require.True(t, ok)
	assert.Equal(t, connect.CodeInternal, connectErr.Code())

	mockSvc.AssertExpectations(t)
}

func TestCreateCoupon_WithAllOptionalFields(t *testing.T) {
	t.Parallel()
	mockSvc := new(mockCouponsServiceForHandler)
	handler := NewCouponsHandler(mockSvc, nil)

	userID := "user-123"
	ctx := createTestUserContextForCoupons(userID)

	affiliateLink := "https://example.com/ref"
	expiresAt := "2026-12-31T23:59:59Z"
	category := "electronics"
	discountType := "percent"
	discountValue := 15.0
	maxDiscount := 50.0
	minProductPrice := 100.0
	couponText := "Special discount!"

	createdCoupon := &models.Coupon{
		ID:              "coupon-full",
		UserID:          userID,
		Code:            "FULLOPT",
		Title:           "Full Options",
		Store:           "shopee",
		IsActive:        true,
		AffiliateLink:   &affiliateLink,
		ExpiresAt:       &expiresAt,
		Category:        &category,
		DiscountType:    &discountType,
		DiscountValue:   &discountValue,
		MaxDiscount:     &maxDiscount,
		MinProductPrice: &minProductPrice,
		CouponText:      &couponText,
		CreatedAt:       "2026-01-01T00:00:00Z",
		UpdatedAt:       "2026-01-01T00:00:00Z",
	}

	mockSvc.On("CreateCoupon", mock.Anything, userID, mock.MatchedBy(func(data *coupons.CreateCouponData) bool {
		return data.Code == "FULLOPT" &&
			data.AffiliateLink == affiliateLink &&
			data.ExpiresAt == expiresAt &&
			data.Category == category &&
			data.DiscountType == discountType &&
			data.DiscountValue == discountValue &&
			data.MaxDiscount == maxDiscount &&
			data.MinProductPrice == minProductPrice &&
			data.CouponText == couponText
	})).Return(createdCoupon, nil)

	req := connect.NewRequest(&couponsv1.CreateCouponRequest{
		Code:            "FULLOPT",
		Title:           "Full Options",
		Store:           couponsv1.Store_STORE_SHOPEE,
		AffiliateLink:   &affiliateLink,
		ExpiresAt:       &expiresAt,
		Category:        &category,
		DiscountType:    couponsv1.DiscountType_DISCOUNT_TYPE_PERCENT,
		DiscountValue:   &discountValue,
		MaxDiscount:     &maxDiscount,
		MinProductPrice: &minProductPrice,
		CouponText:      &couponText,
	})
	resp, err := handler.CreateCoupon(ctx, req)

	require.NoError(t, err)
	assert.Equal(t, "coupon-full", resp.Msg.Coupon.Id)
	assert.Equal(t, "FULLOPT", resp.Msg.Coupon.Code)
	require.NotNil(t, resp.Msg.Coupon.AffiliateLink)
	assert.Equal(t, affiliateLink, *resp.Msg.Coupon.AffiliateLink)
	require.NotNil(t, resp.Msg.Coupon.ExpiresAt)
	assert.Equal(t, expiresAt, *resp.Msg.Coupon.ExpiresAt)
	require.NotNil(t, resp.Msg.Coupon.Category)
	assert.Equal(t, category, *resp.Msg.Coupon.Category)
	assert.Equal(t, couponsv1.DiscountType_DISCOUNT_TYPE_PERCENT, resp.Msg.Coupon.DiscountType)
	require.NotNil(t, resp.Msg.Coupon.DiscountValue)
	assert.Equal(t, discountValue, *resp.Msg.Coupon.DiscountValue)
	require.NotNil(t, resp.Msg.Coupon.MaxDiscount)
	assert.Equal(t, maxDiscount, *resp.Msg.Coupon.MaxDiscount)
	require.NotNil(t, resp.Msg.Coupon.MinProductPrice)
	assert.Equal(t, minProductPrice, *resp.Msg.Coupon.MinProductPrice)
	require.NotNil(t, resp.Msg.Coupon.CouponText)
	assert.Equal(t, couponText, *resp.Msg.Coupon.CouponText)

	mockSvc.AssertExpectations(t)
}

func TestUpdateCoupon_WithAllOptionalFields(t *testing.T) {
	t.Parallel()
	mockSvc := new(mockCouponsServiceForHandler)
	handler := NewCouponsHandler(mockSvc, nil)

	userID := "user-123"
	ctx := createTestUserContextForCoupons(userID)

	discountType := "fixed"
	discountValue := 10.0
	maxDiscount := 30.0
	minProductPrice := 50.0
	updatedCoupon := &models.Coupon{
		ID:              "coupon-1",
		UserID:          userID,
		Code:            "NEWCODE",
		Title:           "New Title",
		Store:           "amazon",
		IsActive:        false,
		DiscountType:    &discountType,
		DiscountValue:   &discountValue,
		MaxDiscount:     &maxDiscount,
		MinProductPrice: &minProductPrice,
		CreatedAt:       "2026-01-01T00:00:00Z",
		UpdatedAt:       "2026-01-02T00:00:00Z",
	}

	mockSvc.On("UpdateCoupon", mock.Anything, "coupon-1", userID, mock.MatchedBy(func(data *coupons.UpdateCouponData) bool {
		return data.Code != nil && *data.Code == "NEWCODE" &&
			data.Title != nil && *data.Title == "New Title" &&
			data.Store != nil && *data.Store == "amazon" &&
			data.IsActive != nil && !*data.IsActive &&
			data.DiscountType != nil && *data.DiscountType == "fixed" &&
			data.DiscountValue != nil && *data.DiscountValue == 10.0 &&
			data.MaxDiscount != nil && *data.MaxDiscount == 30.0 &&
			data.MinProductPrice != nil && *data.MinProductPrice == 50.0
	})).Return(updatedCoupon, nil)

	storeEnum := couponsv1.Store_STORE_AMAZON
	isActive := false
	discTypeEnum := couponsv1.DiscountType_DISCOUNT_TYPE_FIXED
	req := connect.NewRequest(&couponsv1.UpdateCouponRequest{
		Id:              "coupon-1",
		Code:            ptrString("NEWCODE"),
		Title:           ptrString("New Title"),
		Store:           &storeEnum,
		IsActive:        &isActive,
		DiscountType:    &discTypeEnum,
		DiscountValue:   ptrFloat64(10.0),
		MaxDiscount:     ptrFloat64(30.0),
		MinProductPrice: ptrFloat64(50.0),
	})
	resp, err := handler.UpdateCoupon(ctx, req)

	require.NoError(t, err)
	assert.Equal(t, "NEWCODE", resp.Msg.Coupon.Code)
	assert.Equal(t, "New Title", resp.Msg.Coupon.Title)
	assert.Equal(t, couponsv1.Store_STORE_AMAZON, resp.Msg.Coupon.Store)
	assert.False(t, resp.Msg.Coupon.IsActive)

	mockSvc.AssertExpectations(t)
}

func TestUpdateCoupon_WithOptionalStringFields(t *testing.T) {
	t.Parallel()
	mockSvc := new(mockCouponsServiceForHandler)
	handler := NewCouponsHandler(mockSvc, nil)

	userID := "user-123"
	ctx := createTestUserContextForCoupons(userID)

	affiliateLink := "https://new-link.com"
	expiresAt := "2026-06-30T23:59:59Z"
	category := "fashion"
	couponText := "New coupon text"
	updatedCoupon := &models.Coupon{
		ID:            "coupon-1",
		UserID:        userID,
		Code:          "EXISTING",
		Title:         "Existing",
		Store:         "shopee",
		IsActive:      true,
		AffiliateLink: &affiliateLink,
		ExpiresAt:     &expiresAt,
		Category:      &category,
		CouponText:    &couponText,
		CreatedAt:     "2026-01-01T00:00:00Z",
		UpdatedAt:     "2026-01-02T00:00:00Z",
	}

	mockSvc.On("UpdateCoupon", mock.Anything, "coupon-1", userID, mock.MatchedBy(func(data *coupons.UpdateCouponData) bool {
		return data.AffiliateLink != nil && *data.AffiliateLink == affiliateLink &&
			data.ExpiresAt != nil && *data.ExpiresAt == expiresAt &&
			data.Category != nil && *data.Category == category &&
			data.CouponText != nil && *data.CouponText == couponText
	})).Return(updatedCoupon, nil)

	req := connect.NewRequest(&couponsv1.UpdateCouponRequest{
		Id:            "coupon-1",
		AffiliateLink: &affiliateLink,
		ExpiresAt:     &expiresAt,
		Category:      &category,
		CouponText:    &couponText,
	})
	resp, err := handler.UpdateCoupon(ctx, req)

	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Coupon.AffiliateLink)
	assert.Equal(t, affiliateLink, *resp.Msg.Coupon.AffiliateLink)
	require.NotNil(t, resp.Msg.Coupon.ExpiresAt)
	assert.Equal(t, expiresAt, *resp.Msg.Coupon.ExpiresAt)
	require.NotNil(t, resp.Msg.Coupon.Category)
	assert.Equal(t, category, *resp.Msg.Coupon.Category)
	require.NotNil(t, resp.Msg.Coupon.CouponText)
	assert.Equal(t, couponText, *resp.Msg.Coupon.CouponText)

	mockSvc.AssertExpectations(t)
}

func TestListCoupons_WithIsActiveFilter(t *testing.T) {
	t.Parallel()
	mockSvc := new(mockCouponsServiceForHandler)
	handler := NewCouponsHandler(mockSvc, nil)

	userID := "user-123"
	ctx := createTestUserContextForCoupons(userID)

	expectedCoupons := []models.Coupon{
		{
			ID:        "coupon-1",
			UserID:    userID,
			Code:      "ACTIVE",
			Title:     "Active Coupon",
			Store:     "shopee",
			IsActive:  true,
			CreatedAt: "2026-01-01T00:00:00Z",
			UpdatedAt: "2026-01-01T00:00:00Z",
		},
	}

	mockSvc.On("GetUserCoupons", mock.Anything, userID, mock.MatchedBy(func(f *coupons.CouponFilters) bool {
		return f != nil && f.IsActive != nil && *f.IsActive
	})).Return(expectedCoupons, nil)

	isActive := true
	req := connect.NewRequest(&couponsv1.ListCouponsRequest{
		Filter: &couponsv1.CouponFilter{
			IsActive: &isActive,
		},
	})
	resp, err := handler.ListCoupons(ctx, req)

	require.NoError(t, err)
	assert.Len(t, resp.Msg.Coupons, 1)

	mockSvc.AssertExpectations(t)
}

func TestListCoupons_WithCategoryFilter(t *testing.T) {
	t.Parallel()
	mockSvc := new(mockCouponsServiceForHandler)
	handler := NewCouponsHandler(mockSvc, nil)

	userID := "user-123"
	ctx := createTestUserContextForCoupons(userID)

	category := "electronics"
	expectedCoupons := []models.Coupon{
		{
			ID:        "coupon-1",
			UserID:    userID,
			Code:      "ELEC",
			Title:     "Electronics",
			Store:     "shopee",
			IsActive:  true,
			Category:  &category,
			CreatedAt: "2026-01-01T00:00:00Z",
			UpdatedAt: "2026-01-01T00:00:00Z",
		},
	}

	mockSvc.On("GetUserCoupons", mock.Anything, userID, mock.MatchedBy(func(f *coupons.CouponFilters) bool {
		return f != nil && f.Category == "electronics"
	})).Return(expectedCoupons, nil)

	req := connect.NewRequest(&couponsv1.ListCouponsRequest{
		Filter: &couponsv1.CouponFilter{
			Category: &category,
		},
	})
	resp, err := handler.ListCoupons(ctx, req)

	require.NoError(t, err)
	assert.Len(t, resp.Msg.Coupons, 1)

	mockSvc.AssertExpectations(t)
}

func TestListCouponsByStore_EmptyResult(t *testing.T) {
	t.Parallel()
	mockSvc := new(mockCouponsServiceForHandler)
	handler := NewCouponsHandler(mockSvc, nil)

	userID := "user-123"
	ctx := createTestUserContextForCoupons(userID)

	mockSvc.On("GetCouponsByStore", mock.Anything, userID, "shein").Return([]models.Coupon{}, nil)

	req := connect.NewRequest(&couponsv1.ListCouponsByStoreRequest{
		Store: couponsv1.Store_STORE_SHEIN,
	})
	resp, err := handler.ListCouponsByStore(ctx, req)

	require.NoError(t, err)
	assert.Len(t, resp.Msg.Coupons, 0)

	mockSvc.AssertExpectations(t)
}

func TestModelStoreToProtoCoupon_EmptyString(t *testing.T) {
	t.Parallel()
	result := modelStoreToProtoCoupon("")
	assert.Equal(t, couponsv1.Store_STORE_UNSPECIFIED, result)
}
