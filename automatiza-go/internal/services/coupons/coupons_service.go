package coupons

import (
	"automatiza-go/internal/lib/cache"
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"
	"context"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
)

// normalizeString trims whitespace from a string
func normalizeString(s string) string {
	return strings.TrimSpace(s)
}

// normalizeCode normalizes a coupon code to uppercase and trimmed
func normalizeCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

// normalizeOptionalString normalizes an optional string, returning nil for empty strings
func normalizeOptionalString(s string) *string {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

// recalculateIsActive recalculates is_active based on expiration date
// A coupon is active only if: is_active=true AND (expires_at is null OR expires_at > now)
func recalculateIsActive(coupon *models.Coupon, now time.Time) bool {
	if !coupon.IsActive {
		return false
	}
	if coupon.ExpiresAt == nil {
		return true
	}
	expiresAt, err := time.Parse(time.RFC3339, *coupon.ExpiresAt)
	if err != nil {
		// If we can't parse the date, assume it's still active
		return true
	}
	return expiresAt.After(now)
}

// Service defines the coupons service interface
type Service interface {
	GetUserCoupons(ctx context.Context, userID string, filters *CouponFilters) ([]models.Coupon, error)
	GetCouponByID(ctx context.Context, couponID, userID string) (*models.Coupon, error)
	GetCouponsByStore(ctx context.Context, userID, store string) ([]models.Coupon, error)
	CreateCoupon(ctx context.Context, userID string, data *CreateCouponData) (*models.Coupon, error)
	UpdateCoupon(ctx context.Context, couponID, userID string, data *UpdateCouponData) (*models.Coupon, error)
	DeleteCoupon(ctx context.Context, couponID, userID string) error
	GetActiveCoupons(ctx context.Context, userID string) ([]models.Coupon, error)
	ValidateCoupon(ctx context.Context, couponID, userID string) (*CouponValidation, error)
}

// CouponFilters represents filters for coupon queries
type CouponFilters struct {
	Store       string   `json:"store,omitempty"`
	IsActive    *bool    `json:"isActive,omitempty"`
	Category    string   `json:"category,omitempty"`
	MinDiscount *float64 `json:"minDiscount,omitempty"`
}

// CreateCouponData represents data for creating a coupon
type CreateCouponData struct {
	Code            string  `json:"code" validate:"required"`
	Title           string  `json:"title" validate:"required"`
	Store           string  `json:"store" validate:"required"`
	AffiliateLink   string  `json:"affiliateLink,omitempty"`
	ExpiresAt       string  `json:"expiresAt,omitempty"`
	Category        string  `json:"category,omitempty"`
	DiscountType    string  `json:"discountType,omitempty"` // percentage, fixed
	DiscountValue   float64 `json:"discountValue,omitempty"`
	MaxDiscount     float64 `json:"maxDiscount,omitempty"`
	MinProductPrice float64 `json:"minProductPrice,omitempty"`
	CouponText      string  `json:"couponText,omitempty"`
}

// UpdateCouponData represents data for updating a coupon
type UpdateCouponData struct {
	Code            *string  `json:"code,omitempty"`
	Title           *string  `json:"title,omitempty"`
	Store           *string  `json:"store,omitempty"`
	AffiliateLink   *string  `json:"affiliateLink,omitempty"`
	ExpiresAt       *string  `json:"expiresAt,omitempty"`
	IsActive        *bool    `json:"isActive,omitempty"`
	Category        *string  `json:"category,omitempty"`
	DiscountType    *string  `json:"discountType,omitempty"`
	DiscountValue   *float64 `json:"discountValue,omitempty"`
	MaxDiscount     *float64 `json:"maxDiscount,omitempty"`
	MinProductPrice *float64 `json:"minProductPrice,omitempty"`
	CouponText      *string  `json:"couponText,omitempty"`
}

// CouponValidation represents the result of coupon validation
type CouponValidation struct {
	IsValid   bool   `json:"isValid"`
	Reason    string `json:"reason,omitempty"`
	IsExpired bool   `json:"isExpired"`
	IsActive  bool   `json:"isActive"`
}

type couponsService struct {
	couponRepo repository.CouponRepository
	cache      *cache.Cache
	logger     *zap.Logger
}

// NewCouponsService creates a new coupons service
func NewCouponsService(
	couponRepo repository.CouponRepository,
	cache *cache.Cache,
	logger *zap.Logger,
) Service {
	return &couponsService{
		couponRepo: couponRepo,
		cache:      cache,
		logger:     logger,
	}
}

func (s *couponsService) GetUserCoupons(ctx context.Context, userID string, filters *CouponFilters) ([]models.Coupon, error) {
	filterMap := make(map[string]interface{})

	if filters != nil {
		if filters.Store != "" {
			filterMap["store"] = filters.Store
		}
		if filters.IsActive != nil {
			filterMap["is_active"] = *filters.IsActive
		}
		if filters.Category != "" {
			filterMap["category"] = filters.Category
		}
	}

	coupons, err := s.couponRepo.GetUserCoupons(ctx, userID, filterMap)
	if err != nil {
		return nil, fmt.Errorf("failed to get coupons: %w", err)
	}

	// Recalculate is_active based on expiration (parity with Node.js)
	now := time.Now()
	for i := range coupons {
		coupons[i].IsActive = recalculateIsActive(&coupons[i], now)
	}

	// Filter by minimum discount if specified
	if filters != nil && filters.MinDiscount != nil {
		filtered := make([]models.Coupon, 0)
		for _, coupon := range coupons {
			if coupon.DiscountValue != nil && *coupon.DiscountValue >= *filters.MinDiscount {
				filtered = append(filtered, coupon)
			}
		}
		return filtered, nil
	}

	return coupons, nil
}

func (s *couponsService) GetCouponByID(ctx context.Context, couponID, userID string) (*models.Coupon, error) {
	coupon, err := s.couponRepo.GetCouponByID(ctx, couponID, userID)
	if err != nil {
		return nil, err
	}
	if coupon == nil {
		return nil, nil
	}

	// Recalculate is_active based on expiration (parity with Node.js)
	coupon.IsActive = recalculateIsActive(coupon, time.Now())

	return coupon, nil
}

func (s *couponsService) GetCouponsByStore(ctx context.Context, userID, store string) ([]models.Coupon, error) {
	return s.couponRepo.GetActiveCouponsByStore(ctx, userID, store)
}

func (s *couponsService) CreateCoupon(ctx context.Context, userID string, data *CreateCouponData) (*models.Coupon, error) {
	// 1. Normalize code
	normalizedCode := normalizeCode(data.Code)
	if normalizedCode == "" {
		return nil, fmt.Errorf("coupon code is required")
	}

	// 2. Normalize title
	normalizedTitle := normalizeString(data.Title)
	if normalizedTitle == "" {
		return nil, fmt.Errorf("coupon title is required")
	}

	if data.Store == "" {
		return nil, fmt.Errorf("store is required")
	}

	// 3. Check for duplicate code per user
	existing, err := s.couponRepo.GetCouponByCode(ctx, userID, normalizedCode)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing coupon: %w", err)
	}
	if existing != nil {
		return nil, &DuplicateCodeError{Code: normalizedCode}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	coupon := &models.Coupon{
		UserID:    userID,
		Code:      normalizedCode,
		Title:     normalizedTitle,
		Store:     data.Store,
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Normalize optional fields
	coupon.AffiliateLink = normalizeOptionalString(data.AffiliateLink)
	if data.ExpiresAt != "" {
		coupon.ExpiresAt = &data.ExpiresAt
	}
	coupon.Category = normalizeOptionalString(data.Category)
	if data.DiscountType != "" {
		coupon.DiscountType = &data.DiscountType
	}
	if data.DiscountValue > 0 {
		coupon.DiscountValue = &data.DiscountValue
	}
	if data.MaxDiscount > 0 {
		coupon.MaxDiscount = &data.MaxDiscount
	}
	if data.MinProductPrice > 0 {
		coupon.MinProductPrice = &data.MinProductPrice
	}
	coupon.CouponText = normalizeOptionalString(data.CouponText)

	createdCoupon, err := s.couponRepo.CreateCoupon(ctx, userID, coupon)
	if err != nil {
		return nil, fmt.Errorf("failed to create coupon: %w", err)
	}
	coupon = createdCoupon

	s.logger.Info("Created coupon",
		zap.String("userID", userID),
		zap.String("code", normalizedCode),
		zap.String("store", data.Store))

	return coupon, nil
}

func (s *couponsService) UpdateCoupon(ctx context.Context, couponID, userID string, data *UpdateCouponData) (*models.Coupon, error) {
	// 1. Verify coupon exists and belongs to user
	existing, err := s.couponRepo.GetCouponByID(ctx, couponID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get coupon: %w", err)
	}
	if existing == nil {
		return nil, &CouponNotFoundError{ID: couponID}
	}

	updates := make(map[string]interface{})

	// 2. If updating code, check for duplicates
	if data.Code != nil && *data.Code != "" {
		normalizedCode := normalizeCode(*data.Code)
		if normalizedCode != existing.Code {
			dup, err := s.couponRepo.GetCouponByCode(ctx, userID, normalizedCode)
			if err != nil {
				return nil, fmt.Errorf("failed to check duplicate code: %w", err)
			}
			if dup != nil && dup.ID != couponID {
				return nil, &DuplicateCodeError{Code: normalizedCode}
			}
		}
		updates["code"] = normalizedCode
	}

	// 3. Normalize title if present
	if data.Title != nil {
		normalized := normalizeString(*data.Title)
		updates["title"] = normalized
	}

	// 4. Update store if present
	if data.Store != nil && *data.Store != "" {
		updates["store"] = *data.Store
	}

	// Normalize other optional string fields
	if data.AffiliateLink != nil {
		normalized := normalizeString(*data.AffiliateLink)
		if normalized == "" {
			updates["affiliate_link"] = nil
		} else {
			updates["affiliate_link"] = normalized
		}
	}
	if data.ExpiresAt != nil {
		updates["expires_at"] = *data.ExpiresAt
	}
	if data.IsActive != nil {
		updates["is_active"] = *data.IsActive
	}
	if data.Category != nil {
		normalized := normalizeString(*data.Category)
		if normalized == "" {
			updates["category"] = nil
		} else {
			updates["category"] = normalized
		}
	}
	if data.DiscountType != nil {
		updates["discount_type"] = *data.DiscountType
	}
	if data.DiscountValue != nil {
		updates["discount_value"] = *data.DiscountValue
	}
	if data.MaxDiscount != nil {
		updates["max_discount"] = *data.MaxDiscount
	}
	if data.MinProductPrice != nil {
		updates["min_product_price"] = *data.MinProductPrice
	}
	if data.CouponText != nil {
		normalized := normalizeString(*data.CouponText)
		if normalized == "" {
			updates["coupon_text"] = nil
		} else {
			updates["coupon_text"] = normalized
		}
	}

	err = s.couponRepo.UpdateCoupon(ctx, couponID, userID, updates)
	if err != nil {
		return nil, fmt.Errorf("failed to update coupon: %w", err)
	}

	// Return updated coupon
	return s.couponRepo.GetCouponByID(ctx, couponID, userID)
}

func (s *couponsService) DeleteCoupon(ctx context.Context, couponID, userID string) error {
	// Verify coupon exists and belongs to user
	existing, err := s.couponRepo.GetCouponByID(ctx, couponID, userID)
	if err != nil {
		return fmt.Errorf("failed to get coupon: %w", err)
	}
	if existing == nil {
		return &CouponNotFoundError{ID: couponID}
	}

	err = s.couponRepo.DeleteCoupon(ctx, couponID, userID)
	if err != nil {
		return fmt.Errorf("failed to delete coupon: %w", err)
	}

	s.logger.Info("Deleted coupon",
		zap.String("couponID", couponID),
		zap.String("userID", userID))

	return nil
}

func (s *couponsService) GetActiveCoupons(ctx context.Context, userID string) ([]models.Coupon, error) {
	isActive := true
	return s.GetUserCoupons(ctx, userID, &CouponFilters{IsActive: &isActive})
}

func (s *couponsService) ValidateCoupon(ctx context.Context, couponID, userID string) (*CouponValidation, error) {
	coupon, err := s.couponRepo.GetCouponByID(ctx, couponID, userID)
	if err != nil || coupon == nil {
		return &CouponValidation{
			IsValid: false,
			Reason:  "Cupom não encontrado",
		}, nil
	}

	validation := &CouponValidation{
		IsValid:  true,
		IsActive: coupon.IsActive,
	}

	// Check if expired
	if coupon.ExpiresAt != nil {
		expiresAt, err := time.Parse(time.RFC3339, *coupon.ExpiresAt)
		if err == nil && time.Now().After(expiresAt) {
			validation.IsExpired = true
			validation.IsValid = false
			validation.Reason = "Cupom expirado"
		}
	}

	// Check if active
	if !coupon.IsActive {
		validation.IsValid = false
		validation.Reason = "Cupom desativado"
	}

	return validation, nil
}
