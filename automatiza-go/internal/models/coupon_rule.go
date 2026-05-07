package models

// CouponRule is a matching rule that picks a Coupon to inject when a
// product is dispatched. Replaces the per-automation `coupon_id` flow:
// instead of an automation carrying one fixed coupon, the worker
// queries active rules at dispatch time and resolves the best match
// based on platform + period + product price.
//
// Tie-break order at match time: highest MinPrice DESC (the rule whose
// minimum price tier the product just clears), then Priority DESC, then
// CreatedAt DESC. The "highest min_price" default mirrors how operators
// naturally design tiers (R$50 OFF for produtos > R$300, R$100 OFF for
// produtos > R$700) — bigger product earns the bigger coupon.
type CouponRule struct {
	ID          string   `json:"id" db:"id"`
	UserID      string   `json:"user_id" db:"user_id"`
	CouponID    string   `json:"coupon_id" db:"coupon_id"`
	Platform    string   `json:"platform" db:"platform"`
	PeriodStart *string  `json:"period_start,omitempty" db:"period_start"`
	PeriodEnd   *string  `json:"period_end,omitempty" db:"period_end"`
	MinPrice    float64  `json:"min_price" db:"min_price"`
	Priority    int      `json:"priority" db:"priority"`
	IsActive    bool     `json:"is_active" db:"is_active"`
	CreatedAt   string   `json:"created_at" db:"created_at"`
	UpdatedAt   string   `json:"updated_at" db:"updated_at"`
}

// CreateCouponRuleRequest captures a new rule from the API. Period and
// MinPrice are optional — empty period means "always valid", MinPrice=0
// means "any product price".
type CreateCouponRuleRequest struct {
	CouponID    string   `json:"coupon_id" validate:"required"`
	Platform    string   `json:"platform" validate:"required"`
	PeriodStart *string  `json:"period_start,omitempty"`
	PeriodEnd   *string  `json:"period_end,omitempty"`
	MinPrice    *float64 `json:"min_price,omitempty"`
	Priority    *int     `json:"priority,omitempty"`
	IsActive    *bool    `json:"is_active,omitempty"`
}

// UpdateCouponRuleRequest carries optional updates. All fields are
// pointers so the handler can tell "leave alone" (nil) from "set to
// zero value" (pointer to 0/empty/false).
type UpdateCouponRuleRequest struct {
	CouponID    *string  `json:"coupon_id,omitempty"`
	Platform    *string  `json:"platform,omitempty"`
	PeriodStart *string  `json:"period_start,omitempty"`
	PeriodEnd   *string  `json:"period_end,omitempty"`
	MinPrice    *float64 `json:"min_price,omitempty"`
	Priority    *int     `json:"priority,omitempty"`
	IsActive    *bool    `json:"is_active,omitempty"`
}

// ToMap converts the update payload into a column-keyed map for the
// dynamic UPDATE in the repo.
func (r *UpdateCouponRuleRequest) ToMap() map[string]interface{} {
	result := make(map[string]interface{})
	if r.CouponID != nil {
		result["coupon_id"] = *r.CouponID
	}
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
	return result
}
