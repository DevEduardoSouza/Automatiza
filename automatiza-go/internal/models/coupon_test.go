package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUpdateCouponRequest_ToMap(t *testing.T) {
	t.Parallel()

	t.Run("all nil returns empty map", func(t *testing.T) {
		t.Parallel()
		r := &UpdateCouponRequest{}
		m := r.ToMap()
		assert.Empty(t, m)
	})

	t.Run("single field", func(t *testing.T) {
		t.Parallel()
		r := &UpdateCouponRequest{Code: ptr("SAVE10")}
		m := r.ToMap()
		assert.Len(t, m, 1)
		assert.Equal(t, "SAVE10", m["code"])
	})

	t.Run("all 12 fields populated", func(t *testing.T) {
		t.Parallel()
		r := &UpdateCouponRequest{
			Code:            ptr("SAVE10"),
			Title:           ptr("Save 10%"),
			Store:           ptr("shopee"),
			AffiliateLink:   ptr("https://aff.link/123"),
			ExpiresAt:       ptr("2025-12-31"),
			IsActive:        ptr(true),
			Category:        ptr("electronics"),
			DiscountType:    ptr("percentage"),
			DiscountValue:   ptr(10.0),
			MaxDiscount:     ptr(50.0),
			MinProductPrice: ptr(100.0),
			CouponText:      ptr("Use code SAVE10"),
		}
		m := r.ToMap()
		assert.Len(t, m, 12)
		assert.Equal(t, "SAVE10", m["code"])
		assert.Equal(t, "Save 10%", m["title"])
		assert.Equal(t, "shopee", m["store"])
		assert.Equal(t, "https://aff.link/123", m["affiliate_link"])
		assert.Equal(t, "2025-12-31", m["expires_at"])
		assert.Equal(t, true, m["is_active"])
		assert.Equal(t, "electronics", m["category"])
		assert.Equal(t, "percentage", m["discount_type"])
		assert.Equal(t, 10.0, m["discount_value"])
		assert.Equal(t, 50.0, m["max_discount"])
		assert.Equal(t, 100.0, m["min_product_price"])
		assert.Equal(t, "Use code SAVE10", m["coupon_text"])
	})
}
