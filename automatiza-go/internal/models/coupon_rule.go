package models

// CouponRule is a self-contained coupon-application rule. It picks
// itself when a product is dispatched — there's no FK to a Cupom
// Simples row anymore (that was the 046 model). The rule carries its
// own code + message + discount fields, mirroring the AffiliPro
// "Cupom Avançado" form: criar a regra é o ato de definir o cupom.
//
// Cupons Simples remain as a separate catalog used MANUALLY in message
// composition; they're a sibling concept, not a parent.
//
// Tie-break order at match time (ordem TOTAL, ver FindMatchingCoupon):
//  1. regra específica de categoria (ProductCategoryIDs não vazio) vence
//     qualquer regra genérica — especificidade tem precedência absoluta;
//  2. OfficialOnly DESC, somente quando officialFirst=true;
//  3. MinPrice DESC (a faixa de preço mais alta que o produto alcança);
//  4. Priority DESC;
//  5. CreatedAt DESC.
//
// O default "maior min_price" espelha como operadores desenham faixas
// (R$50 OFF para produtos > R$300, R$100 OFF para produtos > R$700) —
// produto maior ganha cupom maior.
type CouponRule struct {
	ID     string `json:"id" db:"id"`
	UserID string `json:"user_id" db:"user_id"`
	// CouponID is the legacy FK preserved for backward compat / rollback
	// only. The matcher and form no longer use it; new rules leave it
	// NULL. A follow-up migration will drop the column entirely.
	CouponID    *string `json:"coupon_id,omitempty" db:"coupon_id"`
	Platform    string  `json:"platform" db:"platform"`
	PeriodStart *string `json:"period_start,omitempty" db:"period_start"`
	PeriodEnd   *string `json:"period_end,omitempty" db:"period_end"`
	MinPrice    float64 `json:"min_price" db:"min_price"`
	Priority    int     `json:"priority" db:"priority"`
	IsActive    bool    `json:"is_active" db:"is_active"`
	// OfficialOnly: when true the rule only matches products from a
	// Shopee Mall store (ShopeeOffer.IsMall=true). Has no effect on
	// non-Shopee platforms — those products never set IsMall, so a
	// rule with OfficialOnly=true on Mercado Livre simply never matches.
	OfficialOnly bool `json:"official_only" db:"official_only"`
	// Code is the human-facing coupon code injected into the message
	// (e.g. "BLACK10"). Optional — operators sometimes want a rule that
	// only applies a discount + custom text, no actual code.
	Code *string `json:"code,omitempty" db:"code"`
	// Message is the WhatsApp-formatted body the rule injects when it
	// fires. It supports the WhatsApp markup subset (*bold*, _italic_,
	// ~strike~, `mono`). When empty the formatter falls back to building
	// a default block from Code + DiscountValue.
	Message       *string  `json:"message,omitempty" db:"message"`
	DiscountType  *string  `json:"discount_type,omitempty" db:"discount_type"`
	DiscountValue *float64 `json:"discount_value,omitempty" db:"discount_value"`
	MaxDiscount   *float64 `json:"max_discount,omitempty" db:"max_discount"`
	// ProductCategoryIDs restringe a regra aos IDs OFICIAIS de categoria
	// da Shopee (ShopeeOffer.ProductCategoryIDs). nil/vazio = regra
	// genérica, que continua sendo o fallback de qualquer produto. Não
	// vazio = a regra só casa por interseção e ganha precedência sobre
	// as genéricas. Só existe em Platform == "shopee" (constraint
	// coupon_rules_product_categories_shopee_only, migração 075).
	ProductCategoryIDs []int64 `db:"product_category_ids" json:"product_category_ids,omitempty"`
	CreatedAt          string  `json:"created_at" db:"created_at"`
	UpdatedAt          string  `json:"updated_at" db:"updated_at"`
}

// CreateCouponRuleRequest captures a new rule from the API. Period and
// MinPrice are optional — empty period means "always valid", MinPrice=0
// means "any product price".
type CreateCouponRuleRequest struct {
	Platform      string   `json:"platform" validate:"required"`
	PeriodStart   *string  `json:"period_start,omitempty"`
	PeriodEnd     *string  `json:"period_end,omitempty"`
	MinPrice      *float64 `json:"min_price,omitempty"`
	Priority      *int     `json:"priority,omitempty"`
	IsActive      *bool    `json:"is_active,omitempty"`
	OfficialOnly  *bool    `json:"official_only,omitempty"`
	Code          *string  `json:"code,omitempty"`
	Message       *string  `json:"message,omitempty"`
	DiscountType  *string  `json:"discount_type,omitempty"`
	DiscountValue *float64 `json:"discount_value,omitempty"`
	MaxDiscount   *float64 `json:"max_discount,omitempty"`
}

// UpdateCouponRuleRequest carries optional updates. All fields are
// pointers so the handler can tell "leave alone" (nil) from "set to
// zero value" (pointer to 0/empty/false).
type UpdateCouponRuleRequest struct {
	Platform      *string  `json:"platform,omitempty"`
	PeriodStart   *string  `json:"period_start,omitempty"`
	PeriodEnd     *string  `json:"period_end,omitempty"`
	MinPrice      *float64 `json:"min_price,omitempty"`
	Priority      *int     `json:"priority,omitempty"`
	IsActive      *bool    `json:"is_active,omitempty"`
	OfficialOnly  *bool    `json:"official_only,omitempty"`
	Code          *string  `json:"code,omitempty"`
	Message       *string  `json:"message,omitempty"`
	DiscountType  *string  `json:"discount_type,omitempty"`
	DiscountValue *float64 `json:"discount_value,omitempty"`
	MaxDiscount   *float64 `json:"max_discount,omitempty"`
}

// ToMap converts the update payload into a column-keyed map for the
// dynamic UPDATE in the repo.
func (r *UpdateCouponRuleRequest) ToMap() map[string]interface{} {
	result := make(map[string]interface{})
	if r.Platform != nil {
		result["platform"] = *r.Platform
	}
	if r.PeriodStart != nil {
		// Empty string = explicit clear, written as NULL.
		if *r.PeriodStart == "" {
			result["period_start"] = nil
		} else {
			result["period_start"] = *r.PeriodStart
		}
	}
	if r.PeriodEnd != nil {
		if *r.PeriodEnd == "" {
			result["period_end"] = nil
		} else {
			result["period_end"] = *r.PeriodEnd
		}
	}
	if r.MinPrice != nil {
		result["min_price"] = *r.MinPrice
	}
	if r.Priority != nil {
		result["priority"] = *r.Priority
	}
	if r.IsActive != nil {
		result["is_active"] = *r.IsActive
	}
	if r.OfficialOnly != nil {
		result["official_only"] = *r.OfficialOnly
	}
	if r.Code != nil {
		if *r.Code == "" {
			result["code"] = nil
		} else {
			result["code"] = *r.Code
		}
	}
	if r.Message != nil {
		if *r.Message == "" {
			result["message"] = nil
		} else {
			result["message"] = *r.Message
		}
	}
	if r.DiscountType != nil {
		if *r.DiscountType == "" {
			result["discount_type"] = nil
		} else {
			result["discount_type"] = *r.DiscountType
		}
	}
	if r.DiscountValue != nil {
		result["discount_value"] = *r.DiscountValue
	}
	if r.MaxDiscount != nil {
		result["max_discount"] = *r.MaxDiscount
	}
	return result
}
