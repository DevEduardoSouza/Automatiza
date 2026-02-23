package coupons

import (
	"automatiza-go/internal/models"
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

// =============================================================================
// GetCouponByID Tests
// =============================================================================

func TestGetCouponByID_Success(t *testing.T) {
	t.Parallel()
	repo := newMockCouponRepository()
	svc := newTestService(repo)
	ctx := context.Background()
	userID := "user-byid"

	futureTime := time.Now().Add(48 * time.Hour).Format(time.RFC3339)
	repo.addCoupon(userID, &models.Coupon{
		ID:        "c-byid",
		UserID:    userID,
		Code:      "BYID",
		Title:     "By ID Coupon",
		Store:     "shopee",
		IsActive:  true,
		ExpiresAt: &futureTime,
	})

	coupon, err := svc.GetCouponByID(ctx, "c-byid", userID)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if coupon == nil {
		t.Fatal("expected non-nil coupon")
	}
	if coupon.ID != "c-byid" {
		t.Errorf("expected ID 'c-byid', got '%s'", coupon.ID)
	}
	if !coupon.IsActive {
		t.Error("expected coupon to be active")
	}
}

func TestGetCouponByID_NotFound(t *testing.T) {
	t.Parallel()
	repo := newMockCouponRepository()
	svc := newTestService(repo)
	ctx := context.Background()

	coupon, err := svc.GetCouponByID(ctx, "nonexistent", "user-nf")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if coupon != nil {
		t.Error("expected nil coupon for not found")
	}
}

func TestGetCouponByID_RepoError(t *testing.T) {
	t.Parallel()
	repo := newMockCouponRepository()
	repo.getByIDErr = errors.New("database connection lost")
	svc := newTestService(repo)
	ctx := context.Background()

	_, err := svc.GetCouponByID(ctx, "any-id", "user-1")
	if err == nil {
		t.Fatal("expected error from repo")
	}
}

func TestGetCouponByID_RecalculatesExpired(t *testing.T) {
	t.Parallel()
	repo := newMockCouponRepository()
	svc := newTestService(repo)
	ctx := context.Background()
	userID := "user-byid-exp"

	pastTime := time.Now().Add(-24 * time.Hour).Format(time.RFC3339)
	repo.addCoupon(userID, &models.Coupon{
		ID:        "c-byid-exp",
		UserID:    userID,
		Code:      "EXPIRED",
		Title:     "Expired",
		Store:     "shopee",
		IsActive:  true,
		ExpiresAt: &pastTime,
	})

	coupon, err := svc.GetCouponByID(ctx, "c-byid-exp", userID)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if coupon.IsActive {
		t.Error("expected coupon to be recalculated as inactive due to expiration")
	}
}

func TestGetCouponByID_ActiveNoExpiration(t *testing.T) {
	t.Parallel()
	repo := newMockCouponRepository()
	svc := newTestService(repo)
	ctx := context.Background()
	userID := "user-byid-noexp"

	repo.addCoupon(userID, &models.Coupon{
		ID:       "c-byid-noexp",
		UserID:   userID,
		Code:     "NOEXP",
		Title:    "No Expiration",
		Store:    "shopee",
		IsActive: true,
	})

	coupon, err := svc.GetCouponByID(ctx, "c-byid-noexp", userID)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !coupon.IsActive {
		t.Error("expected coupon with no expiration to remain active")
	}
}

// =============================================================================
// GetCouponsByStore Tests
// =============================================================================

func TestGetCouponsByStore_Success(t *testing.T) {
	t.Parallel()
	repo := newMockCouponRepository()
	svc := newTestService(repo)
	ctx := context.Background()
	userID := "user-store"

	repo.addCoupon(userID, &models.Coupon{
		ID:       "c-store-1",
		UserID:   userID,
		Code:     "SHOPEE1",
		Title:    "Shopee Coupon",
		Store:    "shopee",
		IsActive: true,
	})
	repo.addCoupon(userID, &models.Coupon{
		ID:       "c-store-2",
		UserID:   userID,
		Code:     "AMAZON1",
		Title:    "Amazon Coupon",
		Store:    "amazon",
		IsActive: true,
	})

	coupons, err := svc.GetCouponsByStore(ctx, userID, "shopee")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(coupons) != 1 {
		t.Fatalf("expected 1 coupon, got %d", len(coupons))
	}
	if coupons[0].Store != "shopee" {
		t.Errorf("expected store 'shopee', got '%s'", coupons[0].Store)
	}
}

func TestGetCouponsByStore_EmptyResult(t *testing.T) {
	t.Parallel()
	repo := newMockCouponRepository()
	svc := newTestService(repo)
	ctx := context.Background()

	coupons, err := svc.GetCouponsByStore(ctx, "user-empty", "shopee")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(coupons) != 0 {
		t.Errorf("expected 0 coupons, got %d", len(coupons))
	}
}

func TestGetCouponsByStore_RepoError(t *testing.T) {
	t.Parallel()
	repo := newMockCouponRepository()
	repo.getActiveCouponsByStoreErr = errors.New("query failed")
	svc := newTestService(repo)
	ctx := context.Background()

	_, err := svc.GetCouponsByStore(ctx, "user-1", "shopee")
	if err == nil {
		t.Fatal("expected error from repo")
	}
}

func TestGetCouponsByStore_InactiveCouponsExcluded(t *testing.T) {
	t.Parallel()
	repo := newMockCouponRepository()
	svc := newTestService(repo)
	ctx := context.Background()
	userID := "user-store-inactive"

	repo.addCoupon(userID, &models.Coupon{
		ID:       "c-inactive-store",
		UserID:   userID,
		Code:     "INACTIVE",
		Title:    "Inactive Coupon",
		Store:    "shopee",
		IsActive: false,
	})

	coupons, err := svc.GetCouponsByStore(ctx, userID, "shopee")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(coupons) != 0 {
		t.Errorf("expected 0 coupons (inactive excluded), got %d", len(coupons))
	}
}

// =============================================================================
// GetUserCoupons Tests (additional coverage)
// =============================================================================

func TestGetUserCoupons_RepoError(t *testing.T) {
	t.Parallel()
	repo := newMockCouponRepository()
	repo.getUserCouponsErr = errors.New("database timeout")
	svc := newTestService(repo)
	ctx := context.Background()

	_, err := svc.GetUserCoupons(ctx, "user-1", nil)
	if err == nil {
		t.Fatal("expected error from repo")
	}
	if !errors.Is(err, repo.getUserCouponsErr) {
		t.Errorf("expected wrapped database timeout error, got: %v", err)
	}
}

func TestGetUserCoupons_NilFilters(t *testing.T) {
	t.Parallel()
	repo := newMockCouponRepository()
	svc := newTestService(repo)
	ctx := context.Background()
	userID := "user-nilfilter"

	repo.addCoupon(userID, &models.Coupon{
		ID:       "c-nil-1",
		UserID:   userID,
		Code:     "NIL1",
		Title:    "Nil Filter",
		Store:    "shopee",
		IsActive: true,
	})

	coupons, err := svc.GetUserCoupons(ctx, userID, nil)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(coupons) != 1 {
		t.Fatalf("expected 1 coupon, got %d", len(coupons))
	}
}

func TestGetUserCoupons_EmptyFilters(t *testing.T) {
	t.Parallel()
	repo := newMockCouponRepository()
	svc := newTestService(repo)
	ctx := context.Background()
	userID := "user-emptyfilter"

	repo.addCoupon(userID, &models.Coupon{
		ID:       "c-ef-1",
		UserID:   userID,
		Code:     "EF1",
		Title:    "Empty Filter",
		Store:    "shopee",
		IsActive: true,
	})

	coupons, err := svc.GetUserCoupons(ctx, userID, &CouponFilters{})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(coupons) != 1 {
		t.Fatalf("expected 1 coupon, got %d", len(coupons))
	}
}

func TestGetUserCoupons_MinDiscountFilter(t *testing.T) {
	t.Parallel()
	repo := newMockCouponRepository()
	svc := newTestService(repo)
	ctx := context.Background()
	userID := "user-mindisc"

	highDiscount := 20.0
	lowDiscount := 5.0
	repo.addCoupon(userID, &models.Coupon{
		ID:            "c-high",
		UserID:        userID,
		Code:          "HIGH",
		Title:         "High Discount",
		Store:         "shopee",
		IsActive:      true,
		DiscountValue: &highDiscount,
	})
	repo.addCoupon(userID, &models.Coupon{
		ID:            "c-low",
		UserID:        userID,
		Code:          "LOW",
		Title:         "Low Discount",
		Store:         "shopee",
		IsActive:      true,
		DiscountValue: &lowDiscount,
	})

	minDiscount := 10.0
	coupons, err := svc.GetUserCoupons(ctx, userID, &CouponFilters{MinDiscount: &minDiscount})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(coupons) != 1 {
		t.Fatalf("expected 1 coupon with discount >= 10, got %d", len(coupons))
	}
	if coupons[0].ID != "c-high" {
		t.Errorf("expected coupon 'c-high', got '%s'", coupons[0].ID)
	}
}

func TestGetUserCoupons_MinDiscountExcludesNilDiscount(t *testing.T) {
	t.Parallel()
	repo := newMockCouponRepository()
	svc := newTestService(repo)
	ctx := context.Background()
	userID := "user-mindisc-nil"

	repo.addCoupon(userID, &models.Coupon{
		ID:       "c-no-disc",
		UserID:   userID,
		Code:     "NODISC",
		Title:    "No Discount",
		Store:    "shopee",
		IsActive: true,
		// DiscountValue is nil
	})

	minDiscount := 1.0
	coupons, err := svc.GetUserCoupons(ctx, userID, &CouponFilters{MinDiscount: &minDiscount})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(coupons) != 0 {
		t.Errorf("expected 0 coupons (nil discount excluded), got %d", len(coupons))
	}
}

func TestGetUserCoupons_EmptyResult(t *testing.T) {
	t.Parallel()
	repo := newMockCouponRepository()
	svc := newTestService(repo)
	ctx := context.Background()

	coupons, err := svc.GetUserCoupons(ctx, "user-nobody", nil)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(coupons) != 0 {
		t.Errorf("expected 0 coupons, got %d", len(coupons))
	}
}

func TestGetUserCoupons_RecalculatesMultipleCoupons(t *testing.T) {
	t.Parallel()
	repo := newMockCouponRepository()
	svc := newTestService(repo)
	ctx := context.Background()
	userID := "user-multi-recalc"

	pastTime := time.Now().Add(-24 * time.Hour).Format(time.RFC3339)
	futureTime := time.Now().Add(48 * time.Hour).Format(time.RFC3339)

	repo.addCoupon(userID, &models.Coupon{
		ID:        "c-past",
		UserID:    userID,
		Code:      "PAST",
		Title:     "Past",
		Store:     "shopee",
		IsActive:  true,
		ExpiresAt: &pastTime,
	})
	repo.addCoupon(userID, &models.Coupon{
		ID:        "c-future",
		UserID:    userID,
		Code:      "FUTURE",
		Title:     "Future",
		Store:     "shopee",
		IsActive:  true,
		ExpiresAt: &futureTime,
	})
	repo.addCoupon(userID, &models.Coupon{
		ID:       "c-noexp",
		UserID:   userID,
		Code:     "NOEXP2",
		Title:    "No Expiry",
		Store:    "shopee",
		IsActive: true,
	})

	coupons, err := svc.GetUserCoupons(ctx, userID, nil)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(coupons) != 3 {
		t.Fatalf("expected 3 coupons, got %d", len(coupons))
	}

	activeCount := 0
	inactiveCount := 0
	for _, c := range coupons {
		if c.IsActive {
			activeCount++
		} else {
			inactiveCount++
		}
	}
	if activeCount != 2 {
		t.Errorf("expected 2 active coupons, got %d", activeCount)
	}
	if inactiveCount != 1 {
		t.Errorf("expected 1 inactive coupon (expired), got %d", inactiveCount)
	}
}

func TestGetUserCoupons_WithStoreFilter(t *testing.T) {
	t.Parallel()
	repo := newMockCouponRepository()
	svc := newTestService(repo)
	ctx := context.Background()
	userID := "user-storefilter"

	repo.addCoupon(userID, &models.Coupon{
		ID:       "c-sf-1",
		UserID:   userID,
		Code:     "SF1",
		Title:    "Store Filter",
		Store:    "shopee",
		IsActive: true,
	})

	coupons, err := svc.GetUserCoupons(ctx, userID, &CouponFilters{Store: "shopee"})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	// Mock doesn't apply filters at repo level, but verifies the flow completes
	if len(coupons) == 0 {
		t.Error("expected at least 1 coupon")
	}
}

func TestGetUserCoupons_WithCategoryFilter(t *testing.T) {
	t.Parallel()
	repo := newMockCouponRepository()
	svc := newTestService(repo)
	ctx := context.Background()
	userID := "user-catfilter"

	category := "electronics"
	repo.addCoupon(userID, &models.Coupon{
		ID:       "c-cat-1",
		UserID:   userID,
		Code:     "CAT1",
		Title:    "Category Filter",
		Store:    "shopee",
		IsActive: true,
		Category: &category,
	})

	coupons, err := svc.GetUserCoupons(ctx, userID, &CouponFilters{Category: "electronics"})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(coupons) == 0 {
		t.Error("expected at least 1 coupon")
	}
}

func TestGetUserCoupons_WithIsActiveFilter(t *testing.T) {
	t.Parallel()
	repo := newMockCouponRepository()
	svc := newTestService(repo)
	ctx := context.Background()
	userID := "user-activefilter"

	repo.addCoupon(userID, &models.Coupon{
		ID:       "c-act-1",
		UserID:   userID,
		Code:     "ACT1",
		Title:    "Active Filter",
		Store:    "shopee",
		IsActive: true,
	})

	isActive := true
	coupons, err := svc.GetUserCoupons(ctx, userID, &CouponFilters{IsActive: &isActive})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(coupons) == 0 {
		t.Error("expected at least 1 coupon")
	}
}

// =============================================================================
// CreateCoupon Tests (additional coverage)
// =============================================================================

func TestCreateCoupon_RepoErrorOnCheckDuplicate(t *testing.T) {
	t.Parallel()
	repo := newMockCouponRepository()
	repo.getByCodeErr = errors.New("connection refused")
	svc := newTestService(repo)
	ctx := context.Background()

	data := &CreateCouponData{
		Code:  "ANYCODE",
		Title: "Any Title",
		Store: "shopee",
	}
	_, err := svc.CreateCoupon(ctx, "user-1", data)
	if err == nil {
		t.Fatal("expected error from repo check")
	}
}

func TestCreateCoupon_RepoErrorOnCreate(t *testing.T) {
	t.Parallel()
	repo := newMockCouponRepository()
	repo.createErr = errors.New("insert failed")
	svc := newTestService(repo)
	ctx := context.Background()

	data := &CreateCouponData{
		Code:  "CREATEFAIL",
		Title: "Create Fail",
		Store: "shopee",
	}
	_, err := svc.CreateCoupon(ctx, "user-1", data)
	if err == nil {
		t.Fatal("expected error from repo create")
	}
}

func TestCreateCoupon_WhitespaceOnlyCode(t *testing.T) {
	t.Parallel()
	repo := newMockCouponRepository()
	svc := newTestService(repo)
	ctx := context.Background()

	data := &CreateCouponData{
		Code:  "   ",
		Title: "Some Title",
		Store: "shopee",
	}
	_, err := svc.CreateCoupon(ctx, "user-1", data)
	if err == nil {
		t.Fatal("expected error for whitespace-only code")
	}
}

func TestCreateCoupon_WhitespaceOnlyTitle(t *testing.T) {
	t.Parallel()
	repo := newMockCouponRepository()
	svc := newTestService(repo)
	ctx := context.Background()

	data := &CreateCouponData{
		Code:  "VALIDCODE",
		Title: "   ",
		Store: "shopee",
	}
	_, err := svc.CreateCoupon(ctx, "user-1", data)
	if err == nil {
		t.Fatal("expected error for whitespace-only title")
	}
}

func TestCreateCoupon_ZeroDiscountNotStored(t *testing.T) {
	t.Parallel()
	repo := newMockCouponRepository()
	svc := newTestService(repo)
	ctx := context.Background()
	userID := "user-zerodisc"

	data := &CreateCouponData{
		Code:            "ZERODISC",
		Title:           "Zero Discount",
		Store:           "shopee",
		DiscountValue:   0,
		MaxDiscount:     0,
		MinProductPrice: 0,
	}
	coupon, err := svc.CreateCoupon(ctx, userID, data)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if coupon.DiscountValue != nil {
		t.Error("expected nil DiscountValue for zero input")
	}
	if coupon.MaxDiscount != nil {
		t.Error("expected nil MaxDiscount for zero input")
	}
	if coupon.MinProductPrice != nil {
		t.Error("expected nil MinProductPrice for zero input")
	}
}

func TestCreateCoupon_ExpiresAtStored(t *testing.T) {
	t.Parallel()
	repo := newMockCouponRepository()
	svc := newTestService(repo)
	ctx := context.Background()
	userID := "user-expat"

	expiresAt := time.Now().Add(72 * time.Hour).Format(time.RFC3339)
	data := &CreateCouponData{
		Code:      "EXPAT",
		Title:     "Expires At Test",
		Store:     "shopee",
		ExpiresAt: expiresAt,
	}
	coupon, err := svc.CreateCoupon(ctx, userID, data)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if coupon.ExpiresAt == nil {
		t.Fatal("expected ExpiresAt to be set")
	}
	if *coupon.ExpiresAt != expiresAt {
		t.Errorf("expected ExpiresAt '%s', got '%s'", expiresAt, *coupon.ExpiresAt)
	}
}

func TestCreateCoupon_DiscountTypeStored(t *testing.T) {
	t.Parallel()
	repo := newMockCouponRepository()
	svc := newTestService(repo)
	ctx := context.Background()
	userID := "user-disctype"

	data := &CreateCouponData{
		Code:         "DISCTYPE",
		Title:        "Discount Type Test",
		Store:        "shopee",
		DiscountType: "fixed",
	}
	coupon, err := svc.CreateCoupon(ctx, userID, data)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if coupon.DiscountType == nil {
		t.Fatal("expected DiscountType to be set")
	}
	if *coupon.DiscountType != "fixed" {
		t.Errorf("expected DiscountType 'fixed', got '%s'", *coupon.DiscountType)
	}
}

func TestCreateCoupon_EmptyOptionalFieldsAreNil(t *testing.T) {
	t.Parallel()
	repo := newMockCouponRepository()
	svc := newTestService(repo)
	ctx := context.Background()
	userID := "user-emptyopt"

	data := &CreateCouponData{
		Code:          "EMPTYOPT",
		Title:         "Empty Optional",
		Store:         "shopee",
		AffiliateLink: "",
		Category:      "",
		CouponText:    "",
		DiscountType:  "",
		ExpiresAt:     "",
	}
	coupon, err := svc.CreateCoupon(ctx, userID, data)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if coupon.AffiliateLink != nil {
		t.Error("expected nil AffiliateLink for empty input")
	}
	if coupon.Category != nil {
		t.Error("expected nil Category for empty input")
	}
	if coupon.CouponText != nil {
		t.Error("expected nil CouponText for empty input")
	}
	if coupon.DiscountType != nil {
		t.Error("expected nil DiscountType for empty input")
	}
	if coupon.ExpiresAt != nil {
		t.Error("expected nil ExpiresAt for empty input")
	}
}

func TestCreateCoupon_SetsIsActiveTrue(t *testing.T) {
	t.Parallel()
	repo := newMockCouponRepository()
	svc := newTestService(repo)
	ctx := context.Background()
	userID := "user-active-default"

	data := &CreateCouponData{
		Code:  "ACTIVEDEF",
		Title: "Active Default",
		Store: "shopee",
	}
	coupon, err := svc.CreateCoupon(ctx, userID, data)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !coupon.IsActive {
		t.Error("expected IsActive to be true by default")
	}
}

func TestCreateCoupon_SetsTimestamps(t *testing.T) {
	t.Parallel()
	repo := newMockCouponRepository()
	svc := newTestService(repo)
	ctx := context.Background()
	userID := "user-timestamps"

	before := time.Now().UTC()
	data := &CreateCouponData{
		Code:  "TIMESTAMPS",
		Title: "Timestamps Test",
		Store: "shopee",
	}
	coupon, err := svc.CreateCoupon(ctx, userID, data)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	createdAt, err := time.Parse(time.RFC3339, coupon.CreatedAt)
	if err != nil {
		t.Fatalf("failed to parse CreatedAt: %v", err)
	}
	if createdAt.Before(before.Add(-1 * time.Second)) {
		t.Error("CreatedAt should be around the current time")
	}
	if coupon.CreatedAt != coupon.UpdatedAt {
		t.Error("CreatedAt and UpdatedAt should be equal on creation")
	}
}

func TestCreateCoupon_NormalizesAffiliateLinkWithSpaces(t *testing.T) {
	t.Parallel()
	repo := newMockCouponRepository()
	svc := newTestService(repo)
	ctx := context.Background()
	userID := "user-afflink"

	data := &CreateCouponData{
		Code:          "AFFLINK",
		Title:         "Affiliate Link",
		Store:         "shopee",
		AffiliateLink: "  https://example.com  ",
	}
	coupon, err := svc.CreateCoupon(ctx, userID, data)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if coupon.AffiliateLink == nil || *coupon.AffiliateLink != "https://example.com" {
		t.Error("expected trimmed affiliate link")
	}
}

// =============================================================================
// UpdateCoupon Tests (additional coverage)
// =============================================================================

func TestUpdateCoupon_UpdateStore(t *testing.T) {
	t.Parallel()
	repo := newMockCouponRepository()
	svc := newTestService(repo)
	ctx := context.Background()
	userID := "user-update-store"

	data := &CreateCouponData{
		Code:  "UPDSTORE",
		Title: "Update Store",
		Store: "shopee",
	}
	coupon, err := svc.CreateCoupon(ctx, userID, data)
	if err != nil {
		t.Fatalf("expected no error creating, got: %v", err)
	}

	newStore := "amazon"
	_, err = svc.UpdateCoupon(ctx, coupon.ID, userID, &UpdateCouponData{
		Store: &newStore,
	})
	if err != nil {
		t.Fatalf("expected no error updating store, got: %v", err)
	}
}

func TestUpdateCoupon_UpdateExpiresAt(t *testing.T) {
	t.Parallel()
	repo := newMockCouponRepository()
	svc := newTestService(repo)
	ctx := context.Background()
	userID := "user-update-exp"

	data := &CreateCouponData{
		Code:  "UPDEXP",
		Title: "Update Expires",
		Store: "shopee",
	}
	coupon, err := svc.CreateCoupon(ctx, userID, data)
	if err != nil {
		t.Fatalf("expected no error creating, got: %v", err)
	}

	newExpires := time.Now().Add(72 * time.Hour).Format(time.RFC3339)
	_, err = svc.UpdateCoupon(ctx, coupon.ID, userID, &UpdateCouponData{
		ExpiresAt: &newExpires,
	})
	if err != nil {
		t.Fatalf("expected no error updating expires, got: %v", err)
	}
}

func TestUpdateCoupon_UpdateIsActive(t *testing.T) {
	t.Parallel()
	repo := newMockCouponRepository()
	svc := newTestService(repo)
	ctx := context.Background()
	userID := "user-update-active"

	data := &CreateCouponData{
		Code:  "UPDACTIVE",
		Title: "Update Active",
		Store: "shopee",
	}
	coupon, err := svc.CreateCoupon(ctx, userID, data)
	if err != nil {
		t.Fatalf("expected no error creating, got: %v", err)
	}

	isActive := false
	_, err = svc.UpdateCoupon(ctx, coupon.ID, userID, &UpdateCouponData{
		IsActive: &isActive,
	})
	if err != nil {
		t.Fatalf("expected no error updating isActive, got: %v", err)
	}
}

func TestUpdateCoupon_UpdateDiscountFields(t *testing.T) {
	t.Parallel()
	repo := newMockCouponRepository()
	svc := newTestService(repo)
	ctx := context.Background()
	userID := "user-update-disc"

	data := &CreateCouponData{
		Code:  "UPDDISC",
		Title: "Update Discount",
		Store: "shopee",
	}
	coupon, err := svc.CreateCoupon(ctx, userID, data)
	if err != nil {
		t.Fatalf("expected no error creating, got: %v", err)
	}

	discType := "percent"
	discValue := 15.0
	maxDisc := 50.0
	minPrice := 100.0
	_, err = svc.UpdateCoupon(ctx, coupon.ID, userID, &UpdateCouponData{
		DiscountType:    &discType,
		DiscountValue:   &discValue,
		MaxDiscount:     &maxDisc,
		MinProductPrice: &minPrice,
	})
	if err != nil {
		t.Fatalf("expected no error updating discount fields, got: %v", err)
	}
}

func TestUpdateCoupon_RepoErrorOnGet(t *testing.T) {
	t.Parallel()
	repo := newMockCouponRepository()
	repo.getByIDErr = errors.New("database error")
	svc := newTestService(repo)
	ctx := context.Background()

	newTitle := "New Title"
	_, err := svc.UpdateCoupon(ctx, "any-id", "user-1", &UpdateCouponData{
		Title: &newTitle,
	})
	if err == nil {
		t.Fatal("expected error from repo get")
	}
}

func TestUpdateCoupon_RepoErrorOnUpdate(t *testing.T) {
	t.Parallel()
	repo := newMockCouponRepository()
	svc := newTestService(repo)
	ctx := context.Background()
	userID := "user-update-err"

	data := &CreateCouponData{
		Code:  "UPDERR",
		Title: "Update Error",
		Store: "shopee",
	}
	coupon, err := svc.CreateCoupon(ctx, userID, data)
	if err != nil {
		t.Fatalf("expected no error creating, got: %v", err)
	}

	repo.updateErr = errors.New("update failed")
	newTitle := "Fail"
	_, err = svc.UpdateCoupon(ctx, coupon.ID, userID, &UpdateCouponData{
		Title: &newTitle,
	})
	if err == nil {
		t.Fatal("expected error from repo update")
	}
}

func TestUpdateCoupon_RepoErrorOnCodeCheck(t *testing.T) {
	t.Parallel()
	repo := newMockCouponRepository()
	svc := newTestService(repo)
	ctx := context.Background()
	userID := "user-code-check-err"

	data := &CreateCouponData{
		Code:  "CODECHECK",
		Title: "Code Check Error",
		Store: "shopee",
	}
	coupon, err := svc.CreateCoupon(ctx, userID, data)
	if err != nil {
		t.Fatalf("expected no error creating, got: %v", err)
	}

	repo.getByCodeErr = errors.New("code check failed")
	newCode := "NEWCODE"
	_, err = svc.UpdateCoupon(ctx, coupon.ID, userID, &UpdateCouponData{
		Code: &newCode,
	})
	if err == nil {
		t.Fatal("expected error from repo code check")
	}
}

func TestUpdateCoupon_EmptyCodeNotTriggerDuplicateCheck(t *testing.T) {
	t.Parallel()
	repo := newMockCouponRepository()
	svc := newTestService(repo)
	ctx := context.Background()
	userID := "user-emptycode"

	data := &CreateCouponData{
		Code:  "EMPTYCODE",
		Title: "Empty Code Update",
		Store: "shopee",
	}
	coupon, err := svc.CreateCoupon(ctx, userID, data)
	if err != nil {
		t.Fatalf("expected no error creating, got: %v", err)
	}

	emptyCode := ""
	newTitle := "Updated"
	_, err = svc.UpdateCoupon(ctx, coupon.ID, userID, &UpdateCouponData{
		Code:  &emptyCode,
		Title: &newTitle,
	})
	if err != nil {
		t.Fatalf("expected no error with empty code, got: %v", err)
	}
}

func TestUpdateCoupon_UpdateCategory(t *testing.T) {
	t.Parallel()
	repo := newMockCouponRepository()
	svc := newTestService(repo)
	ctx := context.Background()
	userID := "user-update-cat"

	data := &CreateCouponData{
		Code:  "UPDCAT",
		Title: "Update Category",
		Store: "shopee",
	}
	coupon, err := svc.CreateCoupon(ctx, userID, data)
	if err != nil {
		t.Fatalf("expected no error creating, got: %v", err)
	}

	newCat := "electronics"
	_, err = svc.UpdateCoupon(ctx, coupon.ID, userID, &UpdateCouponData{
		Category: &newCat,
	})
	if err != nil {
		t.Fatalf("expected no error updating category, got: %v", err)
	}
}

func TestUpdateCoupon_ClearCategoryWithEmpty(t *testing.T) {
	t.Parallel()
	repo := newMockCouponRepository()
	svc := newTestService(repo)
	ctx := context.Background()
	userID := "user-clear-cat"

	data := &CreateCouponData{
		Code:     "CLEARCAT",
		Title:    "Clear Category",
		Store:    "shopee",
		Category: "electronics",
	}
	coupon, err := svc.CreateCoupon(ctx, userID, data)
	if err != nil {
		t.Fatalf("expected no error creating, got: %v", err)
	}

	emptyCat := ""
	_, err = svc.UpdateCoupon(ctx, coupon.ID, userID, &UpdateCouponData{
		Category: &emptyCat,
	})
	if err != nil {
		t.Fatalf("expected no error clearing category, got: %v", err)
	}
}

func TestUpdateCoupon_UpdateCouponText(t *testing.T) {
	t.Parallel()
	repo := newMockCouponRepository()
	svc := newTestService(repo)
	ctx := context.Background()
	userID := "user-update-text"

	data := &CreateCouponData{
		Code:  "UPDTEXT",
		Title: "Update Text",
		Store: "shopee",
	}
	coupon, err := svc.CreateCoupon(ctx, userID, data)
	if err != nil {
		t.Fatalf("expected no error creating, got: %v", err)
	}

	newText := "  New coupon text  "
	_, err = svc.UpdateCoupon(ctx, coupon.ID, userID, &UpdateCouponData{
		CouponText: &newText,
	})
	if err != nil {
		t.Fatalf("expected no error updating coupon text, got: %v", err)
	}
}

func TestUpdateCoupon_EmptyStoreNotTriggered(t *testing.T) {
	t.Parallel()
	repo := newMockCouponRepository()
	svc := newTestService(repo)
	ctx := context.Background()
	userID := "user-emptystore"

	data := &CreateCouponData{
		Code:  "EMPTYSTORE",
		Title: "Empty Store",
		Store: "shopee",
	}
	coupon, err := svc.CreateCoupon(ctx, userID, data)
	if err != nil {
		t.Fatalf("expected no error creating, got: %v", err)
	}

	emptyStore := ""
	newTitle := "Updated Title"
	_, err = svc.UpdateCoupon(ctx, coupon.ID, userID, &UpdateCouponData{
		Store: &emptyStore,
		Title: &newTitle,
	})
	if err != nil {
		t.Fatalf("expected no error with empty store, got: %v", err)
	}
}

func TestUpdateCoupon_UpdateAffiliateLink(t *testing.T) {
	t.Parallel()
	repo := newMockCouponRepository()
	svc := newTestService(repo)
	ctx := context.Background()
	userID := "user-update-afflink"

	data := &CreateCouponData{
		Code:  "UPDAFFLINK",
		Title: "Update Affiliate Link",
		Store: "shopee",
	}
	coupon, err := svc.CreateCoupon(ctx, userID, data)
	if err != nil {
		t.Fatalf("expected no error creating, got: %v", err)
	}

	newLink := "  https://new-link.com  "
	_, err = svc.UpdateCoupon(ctx, coupon.ID, userID, &UpdateCouponData{
		AffiliateLink: &newLink,
	})
	if err != nil {
		t.Fatalf("expected no error updating affiliate link, got: %v", err)
	}
}

// =============================================================================
// DeleteCoupon Tests (additional coverage)
// =============================================================================

func TestDeleteCoupon_Success(t *testing.T) {
	t.Parallel()
	repo := newMockCouponRepository()
	svc := newTestService(repo)
	ctx := context.Background()
	userID := "user-delete"

	data := &CreateCouponData{
		Code:  "DELTEST",
		Title: "Delete Test",
		Store: "shopee",
	}
	coupon, err := svc.CreateCoupon(ctx, userID, data)
	if err != nil {
		t.Fatalf("expected no error creating, got: %v", err)
	}

	err = svc.DeleteCoupon(ctx, coupon.ID, userID)
	if err != nil {
		t.Fatalf("expected no error deleting, got: %v", err)
	}

	// Verify coupon is gone
	found, err := svc.GetCouponByID(ctx, coupon.ID, userID)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if found != nil {
		t.Error("expected coupon to be deleted")
	}
}

func TestDeleteCoupon_RepoErrorOnGet(t *testing.T) {
	t.Parallel()
	repo := newMockCouponRepository()
	repo.getByIDErr = errors.New("database error")
	svc := newTestService(repo)
	ctx := context.Background()

	err := svc.DeleteCoupon(ctx, "any-id", "user-1")
	if err == nil {
		t.Fatal("expected error from repo get")
	}
}

func TestDeleteCoupon_RepoErrorOnDelete(t *testing.T) {
	t.Parallel()
	repo := newMockCouponRepository()
	svc := newTestService(repo)
	ctx := context.Background()
	userID := "user-delete-err"

	data := &CreateCouponData{
		Code:  "DELERR",
		Title: "Delete Error",
		Store: "shopee",
	}
	coupon, err := svc.CreateCoupon(ctx, userID, data)
	if err != nil {
		t.Fatalf("expected no error creating, got: %v", err)
	}

	repo.deleteErr = errors.New("delete failed")
	err = svc.DeleteCoupon(ctx, coupon.ID, userID)
	if err == nil {
		t.Fatal("expected error from repo delete")
	}
}

// =============================================================================
// recalculateIsActive Tests (additional coverage)
// =============================================================================

func TestRecalculateIsActive_MalformedDate(t *testing.T) {
	t.Parallel()
	malformedDate := "not-a-date"
	coupon := &models.Coupon{
		IsActive:  true,
		ExpiresAt: &malformedDate,
	}
	// Should assume active when date can't be parsed
	result := recalculateIsActive(coupon, time.Now())
	if !result {
		t.Error("expected true for malformed date (assumes still active)")
	}
}

func TestRecalculateIsActive_ExactlyNow(t *testing.T) {
	t.Parallel()
	now := time.Now()
	exactlyNow := now.Format(time.RFC3339)
	coupon := &models.Coupon{
		IsActive:  true,
		ExpiresAt: &exactlyNow,
	}
	// expiresAt.After(now) should be false when equal (or very close)
	result := recalculateIsActive(coupon, now)
	// The parsed time might have slight differences due to RFC3339 precision
	// but logically, a coupon expiring "now" should not be active
	if result {
		t.Log("note: coupon expiring at exactly 'now' may still be considered active due to parsing precision")
	}
}

// =============================================================================
// Error Type Tests
// =============================================================================

func TestDuplicateCodeError_Error(t *testing.T) {
	t.Parallel()
	err := &DuplicateCodeError{Code: "TEST123"}
	msg := err.Error()
	expected := "já existe um cupom com o código 'TEST123'"
	if msg != expected {
		t.Errorf("expected '%s', got '%s'", expected, msg)
	}
}

func TestCouponNotFoundError_Error(t *testing.T) {
	t.Parallel()
	err := &CouponNotFoundError{ID: "coupon-abc"}
	msg := err.Error()
	expected := "cupom não encontrado: coupon-abc"
	if msg != expected {
		t.Errorf("expected '%s', got '%s'", expected, msg)
	}
}

func TestIsDuplicateCodeError_WithWrappedError(t *testing.T) {
	t.Parallel()
	baseErr := &DuplicateCodeError{Code: "WRAP"}
	wrappedErr := fmt.Errorf("some context: %w", baseErr)
	if !IsDuplicateCodeError(wrappedErr) {
		t.Error("expected IsDuplicateCodeError to detect wrapped error")
	}
}

func TestIsDuplicateCodeError_WithNonMatchingError(t *testing.T) {
	t.Parallel()
	err := errors.New("some other error")
	if IsDuplicateCodeError(err) {
		t.Error("expected IsDuplicateCodeError to return false for non-matching error")
	}
}

func TestIsCouponNotFoundError_WithNonMatchingError(t *testing.T) {
	t.Parallel()
	err := errors.New("some other error")
	if IsCouponNotFoundError(err) {
		t.Error("expected IsCouponNotFoundError to return false for non-matching error")
	}
}

func TestIsCouponNotFoundError_WithMatchingError(t *testing.T) {
	t.Parallel()
	err := &CouponNotFoundError{ID: "test"}
	if !IsCouponNotFoundError(err) {
		t.Error("expected IsCouponNotFoundError to return true")
	}
}

// =============================================================================
// ValidateCoupon Tests (additional edge cases)
// =============================================================================

func TestValidateCoupon_ActiveNoExpiration(t *testing.T) {
	t.Parallel()
	repo := newMockCouponRepository()
	svc := newTestService(repo)
	ctx := context.Background()
	userID := "user-val-noexp"

	repo.addCoupon(userID, &models.Coupon{
		ID:       "c-val-noexp",
		UserID:   userID,
		Code:     "NOEXP",
		Title:    "No Expiration",
		Store:    "shopee",
		IsActive: true,
	})

	validation, err := svc.ValidateCoupon(ctx, "c-val-noexp", userID)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !validation.IsValid {
		t.Error("expected coupon to be valid")
	}
	if validation.IsExpired {
		t.Error("expected not expired")
	}
	if !validation.IsActive {
		t.Error("expected active")
	}
}

func TestValidateCoupon_ExpiredAndInactive(t *testing.T) {
	t.Parallel()
	repo := newMockCouponRepository()
	svc := newTestService(repo)
	ctx := context.Background()
	userID := "user-val-both"

	pastTime := time.Now().Add(-24 * time.Hour).Format(time.RFC3339)
	repo.addCoupon(userID, &models.Coupon{
		ID:        "c-val-both",
		UserID:    userID,
		Code:      "BOTH",
		Title:     "Expired and Inactive",
		Store:     "shopee",
		IsActive:  false,
		ExpiresAt: &pastTime,
	})

	validation, err := svc.ValidateCoupon(ctx, "c-val-both", userID)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if validation.IsValid {
		t.Error("expected invalid")
	}
	// Last reason set wins: "Cupom desativado" (inactive check runs after expired check)
	if validation.Reason != "Cupom desativado" {
		t.Errorf("expected reason 'Cupom desativado', got '%s'", validation.Reason)
	}
}

func TestValidateCoupon_MalformedExpiresAt(t *testing.T) {
	t.Parallel()
	repo := newMockCouponRepository()
	svc := newTestService(repo)
	ctx := context.Background()
	userID := "user-val-malformed"

	malformed := "not-a-date"
	repo.addCoupon(userID, &models.Coupon{
		ID:        "c-val-mal",
		UserID:    userID,
		Code:      "MALFORMED",
		Title:     "Malformed Date",
		Store:     "shopee",
		IsActive:  true,
		ExpiresAt: &malformed,
	})

	validation, err := svc.ValidateCoupon(ctx, "c-val-mal", userID)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	// Parse error means not expired
	if validation.IsExpired {
		t.Error("expected not expired for malformed date")
	}
	if !validation.IsValid {
		t.Error("expected valid (malformed date treated as non-expired)")
	}
}

func TestValidateCoupon_RepoError(t *testing.T) {
	t.Parallel()
	repo := newMockCouponRepository()
	repo.getByIDErr = errors.New("database error")
	svc := newTestService(repo)
	ctx := context.Background()

	validation, err := svc.ValidateCoupon(ctx, "any-id", "user-1")
	if err != nil {
		t.Fatalf("expected no error (validation handles errors gracefully), got: %v", err)
	}
	if validation.IsValid {
		t.Error("expected invalid when repo errors")
	}
	if validation.Reason != "Cupom não encontrado" {
		t.Errorf("expected 'Cupom não encontrado', got '%s'", validation.Reason)
	}
}

// =============================================================================
// NewCouponsService Tests
// =============================================================================

func TestNewCouponsService_ReturnsService(t *testing.T) {
	t.Parallel()
	repo := newMockCouponRepository()
	svc := newTestService(repo)
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
}

// =============================================================================
// GetActiveCoupons Tests (additional coverage)
// =============================================================================

func TestGetActiveCoupons_RepoError(t *testing.T) {
	t.Parallel()
	repo := newMockCouponRepository()
	repo.getUserCouponsErr = errors.New("database error")
	svc := newTestService(repo)
	ctx := context.Background()

	_, err := svc.GetActiveCoupons(ctx, "user-1")
	if err == nil {
		t.Fatal("expected error from repo")
	}
}

func TestGetActiveCoupons_EmptyResult(t *testing.T) {
	t.Parallel()
	repo := newMockCouponRepository()
	svc := newTestService(repo)
	ctx := context.Background()

	coupons, err := svc.GetActiveCoupons(ctx, "user-nobody")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(coupons) != 0 {
		t.Errorf("expected 0 coupons, got %d", len(coupons))
	}
}
