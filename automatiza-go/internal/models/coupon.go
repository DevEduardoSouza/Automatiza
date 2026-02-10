package models

// Coupon represents a coupon in the database
type Coupon struct {
	ID              string   `json:"id" db:"id"`
	UserID          string   `json:"user_id" db:"user_id"`
	Code            string   `json:"code" db:"code"`
	Title           string   `json:"title" db:"title"`
	Store           string   `json:"store" db:"store"`
	AffiliateLink   *string  `json:"affiliate_link,omitempty" db:"affiliate_link"`
	ExpiresAt       *string  `json:"expires_at,omitempty" db:"expires_at"`
	IsActive        bool     `json:"is_active" db:"is_active"`
	Category        *string  `json:"category,omitempty" db:"category"`
	DiscountType    *string  `json:"discount_type,omitempty" db:"discount_type"`
	DiscountValue   *float64 `json:"discount_value,omitempty" db:"discount_value"`
	MaxDiscount     *float64 `json:"max_discount,omitempty" db:"max_discount"`
	MinProductPrice *float64 `json:"min_product_price,omitempty" db:"min_product_price"`
	CouponText      *string  `json:"coupon_text,omitempty" db:"coupon_text"`
	CreatedAt       string   `json:"created_at" db:"created_at"`
	UpdatedAt       string   `json:"updated_at" db:"updated_at"`
}

// CouponStore constants - must match frontend/proto for parity
const (
	CouponStoreShopee     = "shopee"
	CouponStoreAmazon     = "amazon"
	CouponStoreMagalu     = "magalu"
	CouponStoreMl         = "ml" // Mercado Livre
	CouponStoreShein      = "shein"
	CouponStoreAliexpress = "aliexpress"
	CouponStoreAwin       = "awin"
)

// CreateCouponRequest represents a request to create a coupon
type CreateCouponRequest struct {
	Code            string   `json:"code" validate:"required"`
	Title           string   `json:"title" validate:"required"`
	Store           string   `json:"store" validate:"required"`
	AffiliateLink   *string  `json:"affiliate_link,omitempty"`
	ExpiresAt       *string  `json:"expires_at,omitempty"`
	IsActive        *bool    `json:"is_active,omitempty"`
	Category        *string  `json:"category,omitempty"`
	DiscountType    *string  `json:"discount_type,omitempty"`
	DiscountValue   *float64 `json:"discount_value,omitempty"`
	MaxDiscount     *float64 `json:"max_discount,omitempty"`
	MinProductPrice *float64 `json:"min_product_price,omitempty"`
	CouponText      *string  `json:"coupon_text,omitempty"`
}

// UpdateCouponRequest represents a request to update a coupon
type UpdateCouponRequest struct {
	Code            *string  `json:"code,omitempty"`
	Title           *string  `json:"title,omitempty"`
	Store           *string  `json:"store,omitempty"`
	AffiliateLink   *string  `json:"affiliate_link,omitempty"`
	ExpiresAt       *string  `json:"expires_at,omitempty"`
	IsActive        *bool    `json:"is_active,omitempty"`
	Category        *string  `json:"category,omitempty"`
	DiscountType    *string  `json:"discount_type,omitempty"`
	DiscountValue   *float64 `json:"discount_value,omitempty"`
	MaxDiscount     *float64 `json:"max_discount,omitempty"`
	MinProductPrice *float64 `json:"min_product_price,omitempty"`
	CouponText      *string  `json:"coupon_text,omitempty"`
}

// ToMap converts UpdateCouponRequest to a map for database updates
func (r *UpdateCouponRequest) ToMap() map[string]interface{} {
	result := make(map[string]interface{})

	if r.Code != nil {
		result["code"] = *r.Code
	}
	if r.Title != nil {
		result["title"] = *r.Title
	}
	if r.Store != nil {
		result["store"] = *r.Store
	}
	if r.AffiliateLink != nil {
		result["affiliate_link"] = *r.AffiliateLink
	}
	if r.ExpiresAt != nil {
		result["expires_at"] = *r.ExpiresAt
	}
	if r.IsActive != nil {
		result["is_active"] = *r.IsActive
	}
	if r.Category != nil {
		result["category"] = *r.Category
	}
	if r.DiscountType != nil {
		result["discount_type"] = *r.DiscountType
	}
	if r.DiscountValue != nil {
		result["discount_value"] = *r.DiscountValue
	}
	if r.MaxDiscount != nil {
		result["max_discount"] = *r.MaxDiscount
	}
	if r.MinProductPrice != nil {
		result["min_product_price"] = *r.MinProductPrice
	}
	if r.CouponText != nil {
		result["coupon_text"] = *r.CouponText
	}

	return result
}

// CouponListRequest represents filters for listing coupons
type CouponListRequest struct {
	Store    *string `json:"store,omitempty" form:"store"`
	IsActive *bool   `json:"is_active,omitempty" form:"is_active"`
	Category *string `json:"category,omitempty" form:"category"`
	Limit    int     `json:"limit" form:"limit"`
	Offset   int     `json:"offset" form:"offset"`
}
