package automation

import (
	"context"
	"fmt"
	"time"

	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"

	"go.uber.org/zap"
)

// CouponResult contains the result of applying a coupon to an offer.
// It tracks whether the coupon was applied successfully and provides
// the formatted coupon text for inclusion in the message.
type CouponResult struct {
	// Applied indicates whether the coupon was successfully applied
	Applied bool
	// CouponText contains the formatted coupon text to append to the message
	CouponText string
	// CouponCode contains just the code (for logging/tracking)
	CouponCode string
	// SkipReason explains why the coupon was not applied (empty if applied)
	SkipReason string
}

// CouponHandler handles coupon validation and application for automations.
// It encapsulates the logic for determining if a coupon should be applied
// to an offer based on various criteria (store match, price threshold, etc.).
type CouponHandler struct {
	couponRepo repository.CouponRepository
	logger     *zap.Logger
}

// NewCouponHandler creates a new coupon handler with the given repository and logger.
func NewCouponHandler(couponRepo repository.CouponRepository, logger *zap.Logger) *CouponHandler {
	return &CouponHandler{
		couponRepo: couponRepo,
		logger:     logger,
	}
}

// ApplyCoupon checks if a coupon should be applied to an offer and returns the result.
//
// The function validates:
// - Automation has a coupon configured
// - Coupon exists and belongs to the user
// - Coupon is active and not expired
// - Coupon store matches the offer store (if store is specified)
// - Offer price meets the minimum price requirement (if configured)
//
// Parameters:
// - ctx: Context for cancellation and timeouts
// - automation: The automation containing the coupon configuration
// - offerPrice: The price of the offer being sent
// - offerStore: The store the offer is from (e.g., "shopee", "ml", "amazon")
//
// Returns CouponResult with Applied=true and formatted CouponText if the coupon should be applied,
// or Applied=false with SkipReason explaining why the coupon was not applied.
func (h *CouponHandler) ApplyCoupon(
	ctx context.Context,
	automation *models.Automation,
	offerPrice float64,
	offerStore string,
) (*CouponResult, error) {
	result := &CouponResult{Applied: false}

	// Check if automation has coupon configured
	if automation.CouponID == nil || *automation.CouponID == "" {
		result.SkipReason = "no coupon configured"
		return result, nil
	}

	couponID := *automation.CouponID

	// Get coupon with ownership validation
	coupon, err := h.couponRepo.GetCouponByID(ctx, couponID, automation.UserID)
	if err != nil {
		h.logger.Error("Failed to get coupon",
			zap.String("couponID", couponID),
			zap.String("userID", automation.UserID),
			zap.Error(err))
		result.SkipReason = "coupon retrieval failed"
		return result, nil // Don't fail the automation, just skip coupon
	}

	if coupon == nil {
		h.logger.Warn("Coupon not found or not owned by user",
			zap.String("couponID", couponID),
			zap.String("userID", automation.UserID))
		result.SkipReason = "coupon not found"
		return result, nil
	}

	result.CouponCode = coupon.Code

	// Check if coupon is active
	if !coupon.IsActive {
		result.SkipReason = "coupon is inactive"
		return result, nil
	}

	// Check if coupon is expired
	if coupon.ExpiresAt != nil && *coupon.ExpiresAt != "" {
		expiresAt, err := time.Parse(time.RFC3339, *coupon.ExpiresAt)
		if err == nil && expiresAt.Before(time.Now()) {
			result.SkipReason = "coupon is expired"
			return result, nil
		}
	}

	// Check if coupon store matches offer store (if coupon has store restriction)
	if coupon.Store != "" && offerStore != "" {
		if coupon.Store != offerStore {
			result.SkipReason = fmt.Sprintf("coupon is for store %s (offer is from %s)", coupon.Store, offerStore)
			return result, nil
		}
	}

	// Check minimum price requirement
	if coupon.MinProductPrice != nil {
		minPrice := *coupon.MinProductPrice
		if offerPrice < minPrice {
			result.SkipReason = fmt.Sprintf("offer price R$ %.2f below minimum R$ %.2f", offerPrice, minPrice)
			return result, nil
		}
	}

	// Coupon is valid - apply it
	result.Applied = true
	result.CouponText = h.formatCouponText(coupon)

	h.logger.Debug("Coupon applied successfully",
		zap.String("couponID", coupon.ID),
		zap.String("couponCode", coupon.Code),
		zap.String("store", offerStore),
		zap.Float64("offerPrice", offerPrice))

	return result, nil
}

// formatCouponText formats the coupon information for inclusion in the message.
// It builds a multi-line coupon block with the code, discount info, and any extra text.
//
// Format follows the Node.js parity:
// - Line 1: Coupon emoji + code
// - Line 2: Discount info (if configured)
// - Line 3: Extra text (if configured)
// - Line 4: Affiliate link (if configured)
func (h *CouponHandler) formatCouponText(coupon *models.Coupon) string {
	var lines []string

	// Line 1: Coupon code
	lines = append(lines, fmt.Sprintf("🏷️ CUPOM: %s", coupon.Code))

	// Line 2: Discount info (if configured)
	if coupon.DiscountType != nil && coupon.DiscountValue != nil {
		discountType := *coupon.DiscountType
		discountValue := *coupon.DiscountValue

		if discountType == "fixed" {
			lines = append(lines, fmt.Sprintf("💰 Desconto: R$ %.2f OFF", discountValue))
		} else if discountType == "percent" {
			discountLine := fmt.Sprintf("💰 Desconto: %.0f%% OFF", discountValue)
			if coupon.MaxDiscount != nil && *coupon.MaxDiscount > 0 {
				discountLine += fmt.Sprintf(" (max R$ %.2f)", *coupon.MaxDiscount)
			}
			lines = append(lines, discountLine)
		}
	}

	// Line 3: Extra text (if configured)
	if coupon.CouponText != nil && *coupon.CouponText != "" {
		lines = append(lines, fmt.Sprintf("📝 %s", *coupon.CouponText))
	}

	// Line 4: Affiliate link (if configured)
	if coupon.AffiliateLink != nil && *coupon.AffiliateLink != "" {
		lines = append(lines, fmt.Sprintf("🔗 Link do cupom: %s", *coupon.AffiliateLink))
	}

	// Join all lines with newlines and prefix with double newline for separation
	if len(lines) > 0 {
		result := "\n\n"
		for i, line := range lines {
			if i > 0 {
				result += "\n"
			}
			result += line
		}
		return result
	}

	return ""
}

// LoadCouponOnce loads and validates a coupon once at the start of automation execution.
// This is an optimization to avoid repeated database lookups when sending to multiple groups.
//
// Returns the loaded coupon (or nil if not configured/invalid) and any error message.
// The coupon is pre-validated for active status and expiration.
func (h *CouponHandler) LoadCouponOnce(
	ctx context.Context,
	automation *models.Automation,
) (*models.Coupon, string) {
	// Check if automation has coupon configured
	if automation.CouponID == nil || *automation.CouponID == "" {
		return nil, ""
	}

	couponID := *automation.CouponID

	// Get coupon with ownership validation
	coupon, err := h.couponRepo.GetCouponByID(ctx, couponID, automation.UserID)
	if err != nil {
		h.logger.Error("Failed to load coupon",
			zap.String("couponID", couponID),
			zap.Error(err))
		return nil, "Falha ao carregar cupom"
	}

	if coupon == nil {
		return nil, "Cupom nao encontrado"
	}

	// Check if active
	if !coupon.IsActive {
		return nil, "Cupom inativo"
	}

	// Check expiration
	if coupon.ExpiresAt != nil && *coupon.ExpiresAt != "" {
		expiresAt, err := time.Parse(time.RFC3339, *coupon.ExpiresAt)
		if err == nil && expiresAt.Before(time.Now()) {
			return nil, "Cupom expirado"
		}
	}

	return coupon, ""
}

// ApplyCouponWithPreloaded applies a preloaded coupon to an offer.
// This is more efficient when processing multiple offers with the same coupon.
//
// Parameters:
// - coupon: The preloaded coupon (can be nil if no coupon configured)
// - offerPrice: The price of the offer being sent
// - offerStore: The store the offer is from
//
// Returns CouponResult with the application result.
func (h *CouponHandler) ApplyCouponWithPreloaded(
	coupon *models.Coupon,
	offerPrice float64,
	offerStore string,
) *CouponResult {
	result := &CouponResult{Applied: false}

	// No coupon configured
	if coupon == nil {
		result.SkipReason = "no coupon configured"
		return result
	}

	result.CouponCode = coupon.Code

	// Check store match
	if coupon.Store != "" && offerStore != "" {
		if coupon.Store != offerStore {
			result.SkipReason = fmt.Sprintf("coupon is for store %s (offer is from %s)", coupon.Store, offerStore)
			return result
		}
	}

	// Check minimum price
	if coupon.MinProductPrice != nil {
		minPrice := *coupon.MinProductPrice
		if offerPrice < minPrice {
			result.SkipReason = fmt.Sprintf("offer price R$ %.2f below minimum R$ %.2f", offerPrice, minPrice)
			return result
		}
	}

	// Coupon is valid
	result.Applied = true
	result.CouponText = h.formatCouponText(coupon)

	return result
}
