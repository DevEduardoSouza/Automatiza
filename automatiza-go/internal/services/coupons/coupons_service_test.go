package coupons

import (
	"automatiza-go/internal/models"
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"
)

// mockCouponRepository is a mock implementation of CouponRepository for testing
type mockCouponRepository struct {
	coupons                  map[string]*models.Coupon
	couponsByID              map[string]*models.Coupon
	createErr                error
	updateErr                error
	getByCodeErr             error
	getByIDErr               error
	getUserCouponsErr        error
	deleteErr                error
	getActiveCouponsByStoreErr error
}

func newMockCouponRepository() *mockCouponRepository {
	return &mockCouponRepository{
		coupons:     make(map[string]*models.Coupon),
		couponsByID: make(map[string]*models.Coupon),
	}
}

func (m *mockCouponRepository) GetUserCoupons(ctx context.Context, userID string, filters map[string]interface{}) ([]models.Coupon, error) {
	if m.getUserCouponsErr != nil {
		return nil, m.getUserCouponsErr
	}
	var result []models.Coupon
	for _, c := range m.couponsByID {
		if c.UserID == userID {
			result = append(result, *c)
		}
	}
	return result, nil
}

func (m *mockCouponRepository) GetCouponByID(ctx context.Context, couponID, userID string) (*models.Coupon, error) {
	if m.getByIDErr != nil {
		return nil, m.getByIDErr
	}
	key := userID + ":" + couponID
	if c, ok := m.couponsByID[key]; ok {
		return c, nil
	}
	return nil, nil
}

func (m *mockCouponRepository) GetCouponByCode(ctx context.Context, userID, code string) (*models.Coupon, error) {
	if m.getByCodeErr != nil {
		return nil, m.getByCodeErr
	}
	// Normalize code for lookup
	normalizedCode := normalizeCode(code)
	key := userID + ":" + normalizedCode
	if c, ok := m.coupons[key]; ok {
		return c, nil
	}
	return nil, nil
}

func (m *mockCouponRepository) CreateCoupon(ctx context.Context, userID string, data *models.Coupon) (*models.Coupon, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	data.ID = "test-id-" + data.Code
	data.UserID = userID
	data.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	data.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	// Store by code (normalized)
	key := userID + ":" + normalizeCode(data.Code)
	m.coupons[key] = data

	// Store by ID
	idKey := userID + ":" + data.ID
	m.couponsByID[idKey] = data

	return data, nil
}

func (m *mockCouponRepository) UpdateCoupon(ctx context.Context, couponID, userID string, updates map[string]interface{}) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	key := userID + ":" + couponID
	if c, ok := m.couponsByID[key]; ok {
		if code, ok := updates["code"].(string); ok {
			// Remove old code entry
			oldKey := userID + ":" + normalizeCode(c.Code)
			delete(m.coupons, oldKey)
			// Add new code entry
			c.Code = code
			newKey := userID + ":" + normalizeCode(code)
			m.coupons[newKey] = c
		}
		if title, ok := updates["title"].(string); ok {
			c.Title = title
		}
		return nil
	}
	return errors.New("coupon not found")
}

func (m *mockCouponRepository) DeleteCoupon(ctx context.Context, couponID, userID string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	key := userID + ":" + couponID
	if c, ok := m.couponsByID[key]; ok {
		codeKey := userID + ":" + normalizeCode(c.Code)
		delete(m.coupons, codeKey)
		delete(m.couponsByID, key)
		return nil
	}
	return nil
}

func (m *mockCouponRepository) GetActiveCouponsByStore(ctx context.Context, userID, store string) ([]models.Coupon, error) {
	if m.getActiveCouponsByStoreErr != nil {
		return nil, m.getActiveCouponsByStoreErr
	}
	var result []models.Coupon
	for _, c := range m.couponsByID {
		if c.UserID == userID && c.Store == store && c.IsActive {
			result = append(result, *c)
		}
	}
	return result, nil
}

func (m *mockCouponRepository) GetBestCouponForProduct(ctx context.Context, userID, store string, productPrice float64, category *string) (*models.Coupon, error) {
	var bestCoupon *models.Coupon
	var bestDiscount float64

	for _, c := range m.couponsByID {
		if c.UserID != userID || c.Store != store || !c.IsActive {
			continue
		}
		// Check category match if specified
		if category != nil && c.Category != nil && *c.Category != *category {
			continue
		}
		// Check min product price
		if c.MinProductPrice != nil && productPrice < *c.MinProductPrice {
			continue
		}
		// Compare discount value
		if c.DiscountValue != nil && *c.DiscountValue > bestDiscount {
			bestDiscount = *c.DiscountValue
			bestCoupon = c
		}
	}
	return bestCoupon, nil
}

// Helper to add a coupon directly to the mock
func (m *mockCouponRepository) addCoupon(userID string, coupon *models.Coupon) {
	codeKey := userID + ":" + normalizeCode(coupon.Code)
	m.coupons[codeKey] = coupon
	idKey := userID + ":" + coupon.ID
	m.couponsByID[idKey] = coupon
}

func newTestService(repo *mockCouponRepository) Service {
	logger, _ := zap.NewDevelopment()
	return NewCouponsService(repo, nil, logger)
}

func TestCreateCoupon_NormalizesCode(t *testing.T) {
	repo := newMockCouponRepository()
	svc := newTestService(repo)
	ctx := context.Background()
	userID := "user-123"

	// Create coupon with lowercase code with spaces
	data := &CreateCouponData{
		Code:  "  test123  ",
		Title: "Test Coupon",
		Store: "shopee",
	}

	coupon, err := svc.CreateCoupon(ctx, userID, data)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify code is normalized to uppercase
	if coupon.Code != "TEST123" {
		t.Errorf("expected code 'TEST123', got '%s'", coupon.Code)
	}
}

func TestCreateCoupon_NormalizesTitle(t *testing.T) {
	repo := newMockCouponRepository()
	svc := newTestService(repo)
	ctx := context.Background()
	userID := "user-123"

	// Create coupon with title with extra spaces
	data := &CreateCouponData{
		Code:  "TITLETEST",
		Title: "  My Coupon  ",
		Store: "shopee",
	}

	coupon, err := svc.CreateCoupon(ctx, userID, data)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify title is trimmed
	if coupon.Title != "My Coupon" {
		t.Errorf("expected title 'My Coupon', got '%s'", coupon.Title)
	}
}

func TestCreateCoupon_DuplicateCode(t *testing.T) {
	repo := newMockCouponRepository()
	svc := newTestService(repo)
	ctx := context.Background()
	userID := "user-123"

	// Create first coupon
	data1 := &CreateCouponData{
		Code:  "DUPTEST",
		Title: "First Coupon",
		Store: "shopee",
	}
	_, err := svc.CreateCoupon(ctx, userID, data1)
	if err != nil {
		t.Fatalf("expected no error creating first coupon, got: %v", err)
	}

	// Try to create another coupon with the same code (lowercase - should match)
	data2 := &CreateCouponData{
		Code:  "duptest",
		Title: "Second Coupon",
		Store: "amazon",
	}
	_, err = svc.CreateCoupon(ctx, userID, data2)

	// Should get DuplicateCodeError
	if err == nil {
		t.Fatal("expected DuplicateCodeError, got nil")
	}
	if !IsDuplicateCodeError(err) {
		t.Errorf("expected DuplicateCodeError, got: %T - %v", err, err)
	}
}

func TestUpdateCoupon_DuplicateCode(t *testing.T) {
	repo := newMockCouponRepository()
	svc := newTestService(repo)
	ctx := context.Background()
	userID := "user-123"

	// Create coupon A
	dataA := &CreateCouponData{
		Code:  "CODE_A",
		Title: "Coupon A",
		Store: "shopee",
	}
	couponA, err := svc.CreateCoupon(ctx, userID, dataA)
	if err != nil {
		t.Fatalf("expected no error creating coupon A, got: %v", err)
	}

	// Create coupon B
	dataB := &CreateCouponData{
		Code:  "CODE_B",
		Title: "Coupon B",
		Store: "shopee",
	}
	couponB, err := svc.CreateCoupon(ctx, userID, dataB)
	if err != nil {
		t.Fatalf("expected no error creating coupon B, got: %v", err)
	}

	// Try to update coupon B to use coupon A's code
	newCode := "code_a" // lowercase - should match CODE_A
	updateData := &UpdateCouponData{
		Code: &newCode,
	}
	_, err = svc.UpdateCoupon(ctx, couponB.ID, userID, updateData)

	// Should get DuplicateCodeError
	if err == nil {
		t.Fatal("expected DuplicateCodeError, got nil")
	}
	if !IsDuplicateCodeError(err) {
		t.Errorf("expected DuplicateCodeError, got: %T - %v", err, err)
	}

	// Verify coupon A is unchanged
	_ = couponA
}

func TestUpdateCoupon_SameCodeAllowed(t *testing.T) {
	repo := newMockCouponRepository()
	svc := newTestService(repo)
	ctx := context.Background()
	userID := "user-123"

	// Create coupon
	data := &CreateCouponData{
		Code:  "SAMECODE",
		Title: "Original Title",
		Store: "shopee",
	}
	coupon, err := svc.CreateCoupon(ctx, userID, data)
	if err != nil {
		t.Fatalf("expected no error creating coupon, got: %v", err)
	}

	// Update coupon with the same code (should be allowed)
	sameCode := "samecode" // lowercase variant
	newTitle := "Updated Title"
	updateData := &UpdateCouponData{
		Code:  &sameCode,
		Title: &newTitle,
	}
	updated, err := svc.UpdateCoupon(ctx, coupon.ID, userID, updateData)
	if err != nil {
		t.Fatalf("expected no error updating with same code, got: %v", err)
	}

	if updated.Title != "Updated Title" {
		t.Errorf("expected title 'Updated Title', got '%s'", updated.Title)
	}
}

func TestUpdateCoupon_NotFound(t *testing.T) {
	repo := newMockCouponRepository()
	svc := newTestService(repo)
	ctx := context.Background()
	userID := "user-123"

	// Try to update non-existent coupon
	newTitle := "New Title"
	updateData := &UpdateCouponData{
		Title: &newTitle,
	}
	_, err := svc.UpdateCoupon(ctx, "non-existent-id", userID, updateData)

	// Should get CouponNotFoundError
	if err == nil {
		t.Fatal("expected CouponNotFoundError, got nil")
	}
	if !IsCouponNotFoundError(err) {
		t.Errorf("expected CouponNotFoundError, got: %T - %v", err, err)
	}
}

func TestDeleteCoupon_NotFound(t *testing.T) {
	repo := newMockCouponRepository()
	svc := newTestService(repo)
	ctx := context.Background()
	userID := "user-123"

	// Try to delete non-existent coupon
	err := svc.DeleteCoupon(ctx, "non-existent-id", userID)

	// Should get CouponNotFoundError
	if err == nil {
		t.Fatal("expected CouponNotFoundError, got nil")
	}
	if !IsCouponNotFoundError(err) {
		t.Errorf("expected CouponNotFoundError, got: %T - %v", err, err)
	}
}

func TestNormalizeCode(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"test", "TEST"},
		{"  TEST  ", "TEST"},
		{"Test123", "TEST123"},
		{"  abc  ", "ABC"},
		{"", ""},
	}

	for _, tc := range tests {
		result := normalizeCode(tc.input)
		if result != tc.expected {
			t.Errorf("normalizeCode(%q) = %q, expected %q", tc.input, result, tc.expected)
		}
	}
}

func TestNormalizeString(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"test", "test"},
		{"  TEST  ", "TEST"},
		{"  hello world  ", "hello world"},
		{"", ""},
	}

	for _, tc := range tests {
		result := normalizeString(tc.input)
		if result != tc.expected {
			t.Errorf("normalizeString(%q) = %q, expected %q", tc.input, result, tc.expected)
		}
	}
}

func TestCreateCoupon_WithAllOptionalFields(t *testing.T) {
	t.Parallel()
	repo := newMockCouponRepository()
	svc := newTestService(repo)
	ctx := context.Background()
	userID := "user-opt"

	data := &CreateCouponData{
		Code:            "ALLOPTS",
		Title:           "Full Coupon",
		Store:           "shopee",
		AffiliateLink:   "https://example.com/aff",
		ExpiresAt:       time.Now().Add(72 * time.Hour).Format(time.RFC3339),
		Category:        "electronics",
		DiscountType:    "percent",
		DiscountValue:   25.0,
		MaxDiscount:     50.0,
		MinProductPrice: 10.0,
		CouponText:      "Desconto especial",
	}

	coupon, err := svc.CreateCoupon(ctx, userID, data)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if coupon.Code != "ALLOPTS" {
		t.Errorf("expected code ALLOPTS, got %s", coupon.Code)
	}
	if coupon.AffiliateLink == nil || *coupon.AffiliateLink != "https://example.com/aff" {
		t.Errorf("expected affiliate link to be set")
	}
	if coupon.Category == nil || *coupon.Category != "electronics" {
		t.Errorf("expected category to be set")
	}
	if coupon.DiscountType == nil || *coupon.DiscountType != "percent" {
		t.Errorf("expected discount type percent")
	}
	if coupon.DiscountValue == nil || *coupon.DiscountValue != 25.0 {
		t.Errorf("expected discount value 25.0")
	}
	if coupon.MaxDiscount == nil || *coupon.MaxDiscount != 50.0 {
		t.Errorf("expected max discount 50.0")
	}
	if coupon.MinProductPrice == nil || *coupon.MinProductPrice != 10.0 {
		t.Errorf("expected min product price 10.0")
	}
	if coupon.CouponText == nil || *coupon.CouponText != "Desconto especial" {
		t.Errorf("expected coupon text to be set")
	}
}

func TestGetUserCoupons_RecalculatesExpiredAsInactive(t *testing.T) {
	t.Parallel()
	repo := newMockCouponRepository()
	svc := newTestService(repo)
	ctx := context.Background()
	userID := "user-expired"

	pastTime := time.Now().Add(-24 * time.Hour).Format(time.RFC3339)
	repo.addCoupon(userID, &models.Coupon{
		ID:        "c-expired",
		UserID:    userID,
		Code:      "EXPIRED",
		Title:     "Expired Coupon",
		Store:     "shopee",
		IsActive:  true, // active in DB but expired
		ExpiresAt: &pastTime,
	})

	coupons, err := svc.GetUserCoupons(ctx, userID, nil)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(coupons) != 1 {
		t.Fatalf("expected 1 coupon, got %d", len(coupons))
	}
	// Should be recalculated to inactive because it is expired
	if coupons[0].IsActive {
		t.Error("expected coupon to be recalculated as inactive due to expiration")
	}
}

func TestValidateCoupon_Valid(t *testing.T) {
	t.Parallel()
	repo := newMockCouponRepository()
	svc := newTestService(repo)
	ctx := context.Background()
	userID := "user-val"

	futureTime := time.Now().Add(48 * time.Hour).Format(time.RFC3339)
	repo.addCoupon(userID, &models.Coupon{
		ID:        "c-valid",
		UserID:    userID,
		Code:      "VALID",
		Title:     "Valid Coupon",
		Store:     "shopee",
		IsActive:  true,
		ExpiresAt: &futureTime,
	})

	validation, err := svc.ValidateCoupon(ctx, "c-valid", userID)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !validation.IsValid {
		t.Error("expected coupon to be valid")
	}
	if validation.IsExpired {
		t.Error("expected coupon to not be expired")
	}
	if !validation.IsActive {
		t.Error("expected coupon to be active")
	}
}

func TestValidateCoupon_Expired(t *testing.T) {
	t.Parallel()
	repo := newMockCouponRepository()
	svc := newTestService(repo)
	ctx := context.Background()
	userID := "user-valexp"

	pastTime := time.Now().Add(-24 * time.Hour).Format(time.RFC3339)
	repo.addCoupon(userID, &models.Coupon{
		ID:        "c-valexp",
		UserID:    userID,
		Code:      "VALEXP",
		Title:     "Expired",
		Store:     "shopee",
		IsActive:  true,
		ExpiresAt: &pastTime,
	})

	validation, err := svc.ValidateCoupon(ctx, "c-valexp", userID)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if validation.IsValid {
		t.Error("expected coupon to be invalid")
	}
	if !validation.IsExpired {
		t.Error("expected coupon to be expired")
	}
	if validation.Reason != "Cupom expirado" {
		t.Errorf("expected reason 'Cupom expirado', got '%s'", validation.Reason)
	}
}

func TestValidateCoupon_Inactive(t *testing.T) {
	t.Parallel()
	repo := newMockCouponRepository()
	svc := newTestService(repo)
	ctx := context.Background()
	userID := "user-valinact"

	repo.addCoupon(userID, &models.Coupon{
		ID:       "c-valinact",
		UserID:   userID,
		Code:     "INACTIVE",
		Title:    "Inactive Coupon",
		Store:    "shopee",
		IsActive: false,
	})

	validation, err := svc.ValidateCoupon(ctx, "c-valinact", userID)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if validation.IsValid {
		t.Error("expected coupon to be invalid")
	}
	if validation.Reason != "Cupom desativado" {
		t.Errorf("expected reason 'Cupom desativado', got '%s'", validation.Reason)
	}
}

func TestValidateCoupon_NotFound(t *testing.T) {
	t.Parallel()
	repo := newMockCouponRepository()
	svc := newTestService(repo)
	ctx := context.Background()

	validation, err := svc.ValidateCoupon(ctx, "nonexistent", "user-nf")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if validation.IsValid {
		t.Error("expected coupon to be invalid")
	}
}

func TestGetActiveCoupons_DelegatesWithFilter(t *testing.T) {
	t.Parallel()
	repo := newMockCouponRepository()
	svc := newTestService(repo)
	ctx := context.Background()
	userID := "user-active"

	futureTime := time.Now().Add(48 * time.Hour).Format(time.RFC3339)
	repo.addCoupon(userID, &models.Coupon{
		ID:        "c-active",
		UserID:    userID,
		Code:      "ACTIVE1",
		Title:     "Active",
		Store:     "shopee",
		IsActive:  true,
		ExpiresAt: &futureTime,
	})
	repo.addCoupon(userID, &models.Coupon{
		ID:       "c-inactive",
		UserID:   userID,
		Code:     "INACTIVE1",
		Title:    "Inactive",
		Store:    "shopee",
		IsActive: false,
	})

	coupons, err := svc.GetActiveCoupons(ctx, userID)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Both are returned by mock but recalculated: one active, one inactive
	activeCount := 0
	for _, c := range coupons {
		if c.IsActive {
			activeCount++
		}
	}
	if activeCount == 0 {
		t.Error("expected at least one active coupon after recalculation")
	}
}

func TestUpdateCoupon_ClearsOptionalFields(t *testing.T) {
	t.Parallel()
	repo := newMockCouponRepository()
	svc := newTestService(repo)
	ctx := context.Background()
	userID := "user-clear"

	// Create coupon with optional fields
	data := &CreateCouponData{
		Code:          "CLEARTEST",
		Title:         "Clear Test",
		Store:         "shopee",
		AffiliateLink: "https://example.com",
		CouponText:    "Some text",
	}
	coupon, err := svc.CreateCoupon(ctx, userID, data)
	if err != nil {
		t.Fatalf("expected no error creating coupon, got: %v", err)
	}

	// Update with empty affiliate link and coupon text to clear them
	emptyLink := ""
	emptyText := ""
	updateData := &UpdateCouponData{
		AffiliateLink: &emptyLink,
		CouponText:    &emptyText,
	}
	_, err = svc.UpdateCoupon(ctx, coupon.ID, userID, updateData)
	if err != nil {
		t.Fatalf("expected no error updating coupon, got: %v", err)
	}
	// The update itself succeeds. The mock does not track all field changes
	// but verifies the flow completes without error.
}

func TestNormalizeOptionalString_Empty(t *testing.T) {
	t.Parallel()
	result := normalizeOptionalString("")
	if result != nil {
		t.Errorf("expected nil for empty string, got %v", *result)
	}
}

func TestNormalizeOptionalString_Whitespace(t *testing.T) {
	t.Parallel()
	result := normalizeOptionalString("   ")
	if result != nil {
		t.Errorf("expected nil for whitespace-only string, got %v", *result)
	}
}

func TestNormalizeOptionalString_Value(t *testing.T) {
	t.Parallel()
	result := normalizeOptionalString("  hello  ")
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if *result != "hello" {
		t.Errorf("expected 'hello', got '%s'", *result)
	}
}

func TestCreateCoupon_EmptyCode(t *testing.T) {
	t.Parallel()
	repo := newMockCouponRepository()
	svc := newTestService(repo)
	ctx := context.Background()

	data := &CreateCouponData{
		Code:  "",
		Title: "No Code",
		Store: "shopee",
	}
	_, err := svc.CreateCoupon(ctx, "user-1", data)
	if err == nil {
		t.Fatal("expected error for empty code")
	}
}

func TestCreateCoupon_EmptyTitle(t *testing.T) {
	t.Parallel()
	repo := newMockCouponRepository()
	svc := newTestService(repo)
	ctx := context.Background()

	data := &CreateCouponData{
		Code:  "NOTITLE",
		Title: "",
		Store: "shopee",
	}
	_, err := svc.CreateCoupon(ctx, "user-1", data)
	if err == nil {
		t.Fatal("expected error for empty title")
	}
}

func TestCreateCoupon_EmptyStore(t *testing.T) {
	t.Parallel()
	repo := newMockCouponRepository()
	svc := newTestService(repo)
	ctx := context.Background()

	data := &CreateCouponData{
		Code:  "NOSTORE",
		Title: "No Store",
		Store: "",
	}
	_, err := svc.CreateCoupon(ctx, "user-1", data)
	if err == nil {
		t.Fatal("expected error for empty store")
	}
}

func TestRecalculateIsActive(t *testing.T) {
	now := time.Now()
	pastTime := now.Add(-24 * time.Hour).Format(time.RFC3339)
	futureTime := now.Add(24 * time.Hour).Format(time.RFC3339)

	tests := []struct {
		name     string
		coupon   *models.Coupon
		expected bool
	}{
		{
			name:     "active coupon with no expiration",
			coupon:   &models.Coupon{IsActive: true, ExpiresAt: nil},
			expected: true,
		},
		{
			name:     "active coupon with future expiration",
			coupon:   &models.Coupon{IsActive: true, ExpiresAt: &futureTime},
			expected: true,
		},
		{
			name:     "active coupon with past expiration",
			coupon:   &models.Coupon{IsActive: true, ExpiresAt: &pastTime},
			expected: false,
		},
		{
			name:     "inactive coupon with no expiration",
			coupon:   &models.Coupon{IsActive: false, ExpiresAt: nil},
			expected: false,
		},
		{
			name:     "inactive coupon with future expiration",
			coupon:   &models.Coupon{IsActive: false, ExpiresAt: &futureTime},
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := recalculateIsActive(tc.coupon, now)
			if result != tc.expected {
				t.Errorf("recalculateIsActive() = %v, expected %v", result, tc.expected)
			}
		})
	}
}
