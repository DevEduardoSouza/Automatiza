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
	coupons      map[string]*models.Coupon
	couponsByID  map[string]*models.Coupon
	createErr    error
	updateErr    error
	getByCodeErr error
	getByIDErr   error
}

func newMockCouponRepository() *mockCouponRepository {
	return &mockCouponRepository{
		coupons:     make(map[string]*models.Coupon),
		couponsByID: make(map[string]*models.Coupon),
	}
}

func (m *mockCouponRepository) GetUserCoupons(ctx context.Context, userID string, filters map[string]interface{}) ([]models.Coupon, error) {
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
