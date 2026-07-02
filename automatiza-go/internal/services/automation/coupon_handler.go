package automation

import (
	"context"
	"fmt"
	"strings"
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
	// DiscountedPrice is the price after applying the coupon discount
	DiscountedPrice float64
}

// CouponHandler resolves which coupon (if any) to inject into an offer
// dispatch. As of migration 046 it queries the global rule engine
// (coupon_rules table) instead of reading the per-automation coupon_id.
//
// The flow is per-offer: each call to ApplyCoupon hits the matcher with
// (userID, store, productPrice). For automations dispatching the same
// product to N groups this is N queries — acceptable for the MVP given
// the partial index on (user_id, platform, is_active=true). If it shows
// up as a hotspot we can preload rules+coupons once per automation run
// and match in-memory.
//
// couponRuleRepo can be nil when callers pass no rule repo (e.g. during
// migration or tests); the handler then degrades to "no coupon".
type CouponHandler struct {
	couponRepo     repository.CouponRepository
	couponRuleRepo repository.CouponRuleRepository
	logger         *zap.Logger
}

// NewCouponHandler creates a new coupon handler. couponRuleRepo can be
// nil for callers that aren't on the rules engine yet — the handler
// then resolves to "no coupon" on every call.
func NewCouponHandler(couponRepo repository.CouponRepository, couponRuleRepo repository.CouponRuleRepository, logger *zap.Logger) *CouponHandler {
	return &CouponHandler{
		couponRepo:     couponRepo,
		couponRuleRepo: couponRuleRepo,
		logger:         logger,
	}
}

// ApplyCoupon resolves which coupon to inject into an offer using the
// rules engine and returns the formatted result.
//
// Flow:
//   - automation.ApplyCouponRules == false → result.Applied=false (the
//     operator opted this automation out of the rule engine).
//   - couponRuleRepo nil → handler runs without rules support, no coupon.
//   - Otherwise: query the rules engine with (userID, offerStore,
//     offerPrice, productIsOfficial). The engine returns the
//     best-matching coupon (highest min_price tier the product clears,
//     then priority, then created_at) and skips rules whose
//     official_only=true don't match the Shopee Mall flag.
//   - Defensive checks on the resolved coupon: must be active, must not
//     be expired. Store / min_product_price are NOT re-checked here:
//     the rule already filtered by platform and min_price.
//
// Returns CouponResult with Applied=true and formatted CouponText if a
// rule matched, or Applied=false with SkipReason describing why nothing
// was injected.
func (h *CouponHandler) ApplyCoupon(
	ctx context.Context,
	automation *models.Automation,
	offerPrice float64,
	offerStore string,
	productIsOfficial bool,
) (*CouponResult, error) {
	result := &CouponResult{Applied: false}

	if !automation.ApplyCouponRules {
		result.SkipReason = "coupon rules disabled for this automation"
		return result, nil
	}
	if h.couponRuleRepo == nil {
		result.SkipReason = "coupon rules engine not configured"
		return result, nil
	}

	// Normalize the store before querying — the rules table stores
	// platform exactly as the catalog (e.g. "shopee", "mercado_livre"),
	// so keep the same canonical form here.
	platform := normalizeStoreName(offerStore)
	coupon, err := h.couponRuleRepo.FindMatchingCoupon(ctx, automation.UserID, platform, offerPrice, productIsOfficial)
	if err != nil {
		h.logger.Error("Failed to resolve coupon rule",
			zap.String("userID", automation.UserID),
			zap.String("platform", platform),
			zap.Float64("offerPrice", offerPrice),
			zap.Error(err))
		result.SkipReason = "coupon rule lookup failed"
		return result, nil // Don't fail the automation, just skip coupon
	}
	if coupon == nil {
		result.SkipReason = "no coupon rule matched"
		return result, nil
	}

	result.CouponCode = coupon.Code

	// Defensive: a rule could point to an inactive or expired coupon
	// (operator deactivated the catalog row but left the rule on). Skip
	// rather than emit a stale coupon.
	if !coupon.IsActive {
		result.SkipReason = "coupon is inactive"
		return result, nil
	}
	if coupon.ExpiresAt != nil && *coupon.ExpiresAt != "" {
		expiresAt, err := time.Parse(time.RFC3339, *coupon.ExpiresAt)
		if err == nil && expiresAt.Before(time.Now()) {
			result.SkipReason = "coupon is expired"
			return result, nil
		}
	}

	result.Applied = true
	result.CouponText = h.formatCouponText(coupon)
	result.DiscountedPrice = CalculateDiscountedPrice(offerPrice, coupon)

	h.logger.Debug("Coupon applied via rule engine",
		zap.String("couponID", coupon.ID),
		zap.String("couponCode", coupon.Code),
		zap.String("platform", platform),
		zap.Float64("offerPrice", offerPrice))

	return result, nil
}

// formatCouponText devolve o texto livre que o operador escreveu em
// "Mensagem do cupom" (coupon.CouponText), sem mais nada. Antes a função
// montava um bloco auto-gerado com 🏷️ CUPOM: <código>, 💰 Desconto: X% OFF
// e 🔗 Link do cupom: <url>; agora o operador controla 100% do que aparece
// no disparo. Quem quiser código, desconto, emoji ou link no texto inclui
// na própria mensagem do cupom.
//
// Mantém o prefixo \n\n pra continuar separando o bloco do resto do
// template; o RenderTemplate do dispatch colapsa newlines extras (até 2)
// se o template já tiver espaçamento próprio.
//
// CouponText vazio resulta em "" — nenhum bloco é anexado e o template
// renderiza como se a regra não tivesse casado.
func (h *CouponHandler) formatCouponText(coupon *models.Coupon) string {
	if coupon.CouponText == nil || *coupon.CouponText == "" {
		return ""
	}
	return "\n\n" + *coupon.CouponText
}

// LoadCouponOnce is a legacy optimization that pre-fetched the
// automation's fixed coupon once before iterating dispatch targets.
// With the rules engine the matched coupon depends on the offer's
// price and platform, so a single preload no longer fits — every
// caller should use ApplyCoupon per offer instead.
//
// Kept as a no-op so older call sites compile during the migration;
// a follow-up will remove the function entirely.
func (h *CouponHandler) LoadCouponOnce(
	_ context.Context,
	_ *models.Automation,
) (*models.Coupon, string) {
	return nil, ""
}

// ApplyCouponWithPreloaded is a legacy fast-path that took a coupon
// already loaded by LoadCouponOnce. With the rules engine the resolved
// coupon depends on the offer's price, so the preload pattern no longer
// applies — callers should switch to ApplyCoupon. Kept as a thin
// fallback that just formats a non-nil coupon, mostly so older code
// paths compile and tests continue to exercise the formatting.
func (h *CouponHandler) ApplyCouponWithPreloaded(
	coupon *models.Coupon,
	offerPrice float64,
	offerStore string,
) *CouponResult {
	result := &CouponResult{Applied: false}
	if coupon == nil {
		result.SkipReason = "no coupon configured"
		return result
	}
	result.CouponCode = coupon.Code
	if coupon.Store != "" && offerStore != "" &&
		normalizeStoreName(coupon.Store) != normalizeStoreName(offerStore) {
		result.SkipReason = fmt.Sprintf("coupon is for store %s (offer is from %s)", coupon.Store, offerStore)
		return result
	}
	if coupon.MinProductPrice != nil && offerPrice < *coupon.MinProductPrice {
		result.SkipReason = fmt.Sprintf("offer price R$ %.2f below minimum R$ %.2f", offerPrice, *coupon.MinProductPrice)
		return result
	}
	result.Applied = true
	result.CouponText = h.formatCouponText(coupon)
	result.DiscountedPrice = CalculateDiscountedPrice(offerPrice, coupon)
	return result
}

// normalizeStoreName maps store name variants to a canonical form for comparison.
func normalizeStoreName(store string) string {
	switch strings.ToLower(strings.TrimSpace(store)) {
	case "ml", "mercadolivre", "mercado_livre":
		// Forma canonica DEVE bater com coupon_rules.platform ("mercado_livre",
		// com underscore — ver proto/coupons/v1/coupons.proto). Retornar
		// "mercadolivre" (sem underscore) fazia o WHERE cr.platform = $2 nunca
		// casar as regras ML -> TODO cupom avancado de Mercado Livre era
		// silenciosamente descartado (Shopee funcionava por coincidir o nome).
		return "mercado_livre"
	case "shopee":
		return "shopee"
	case "amazon":
		return "amazon"
	case "magalu":
		return "magalu"
	case "shein":
		return "shein"
	case "aliexpress":
		return "aliexpress"
	case "awin":
		return "awin"
	default:
		return strings.ToLower(store)
	}
}

// CalculateDiscountedPrice calculates the price after applying a coupon discount.
// Returns the original price if coupon is nil or has no discount configured.
func CalculateDiscountedPrice(price float64, coupon *models.Coupon) float64 {
	if coupon == nil || coupon.DiscountType == nil || coupon.DiscountValue == nil {
		return price
	}

	discountType := *coupon.DiscountType
	discountValue := *coupon.DiscountValue

	switch discountType {
	case "fixed":
		result := price - discountValue
		if result < 0 {
			return 0
		}
		return result
	case "percent":
		discount := price * discountValue / 100
		if coupon.MaxDiscount != nil && *coupon.MaxDiscount > 0 && discount > *coupon.MaxDiscount {
			discount = *coupon.MaxDiscount
		}
		return price - discount
	default:
		return price
	}
}
