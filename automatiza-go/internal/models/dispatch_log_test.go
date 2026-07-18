package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDispatchLog_SetCouponAudit(t *testing.T) {
	t.Parallel()

	t.Run("not evaluated (empty result) leaves all columns NULL", func(t *testing.T) {
		t.Parallel()
		var l DispatchLog
		l.SetCouponAudit(false, "", "")
		assert.Nil(t, l.CouponApplied, "not evaluated must not stamp coupon_applied")
		assert.Nil(t, l.CouponCode)
		assert.Nil(t, l.CouponSkipReason)
	})

	t.Run("applied stamps code and applied=true, no skip reason", func(t *testing.T) {
		t.Parallel()
		var l DispatchLog
		l.SetCouponAudit(true, "OFICIAL20", "")
		if assert.NotNil(t, l.CouponApplied) {
			assert.True(t, *l.CouponApplied)
		}
		if assert.NotNil(t, l.CouponCode) {
			assert.Equal(t, "OFICIAL20", *l.CouponCode)
		}
		assert.Nil(t, l.CouponSkipReason)
	})

	t.Run("evaluated and skipped stamps applied=false with reason", func(t *testing.T) {
		t.Parallel()
		var l DispatchLog
		l.SetCouponAudit(false, "", "no coupon rule matched")
		if assert.NotNil(t, l.CouponApplied) {
			assert.False(t, *l.CouponApplied)
		}
		if assert.NotNil(t, l.CouponSkipReason) {
			assert.Equal(t, "no coupon rule matched", *l.CouponSkipReason)
		}
	})

	t.Run("code longer than column width is truncated to 50 chars", func(t *testing.T) {
		t.Parallel()
		var l DispatchLog
		long := ""
		for i := 0; i < 60; i++ {
			long += "A"
		}
		l.SetCouponAudit(true, long, "")
		if assert.NotNil(t, l.CouponCode) {
			assert.Len(t, []rune(*l.CouponCode), 50, "coupon_code must fit VARCHAR(50)")
		}
	})
}

func TestUpdateDispatchLogRequest_ToMap(t *testing.T) {
	t.Parallel()

	t.Run("all nil returns empty map", func(t *testing.T) {
		t.Parallel()
		r := &UpdateDispatchLogRequest{}
		m := r.ToMap()
		assert.Empty(t, m)
	})

	t.Run("status only", func(t *testing.T) {
		t.Parallel()
		r := &UpdateDispatchLogRequest{Status: ptr("sent")}
		m := r.ToMap()
		assert.Len(t, m, 1)
		assert.Equal(t, "sent", m["status"])
	})

	t.Run("both fields", func(t *testing.T) {
		t.Parallel()
		r := &UpdateDispatchLogRequest{
			Status: ptr("failed"),
			Error:  ptr("timeout connecting to WhatsApp"),
		}
		m := r.ToMap()
		assert.Len(t, m, 2)
		assert.Equal(t, "failed", m["status"])
		assert.Equal(t, "timeout connecting to WhatsApp", m["error"])
	})
}
