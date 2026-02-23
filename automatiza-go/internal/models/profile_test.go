package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProfile_IsSuspended(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		billingStatus *string
		want          bool
	}{
		{"nil billing status", nil, false},
		{"suspended", ptr(BillingStatusSuspended), true},
		{"active", ptr(BillingStatusActive), false},
		{"canceled", ptr(BillingStatusCancelled), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := &Profile{BillingStatus: tt.billingStatus}
			assert.Equal(t, tt.want, p.IsSuspended())
		})
	}
}

func TestProfile_IsFree(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		plan string
		want bool
	}{
		{"free plan", PlanFree, true},
		{"pro plan", PlanPro, false},
		{"elite plan", PlanElite, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := &Profile{Plan: tt.plan}
			assert.Equal(t, tt.want, p.IsFree())
		})
	}
}

func TestProfile_IsPro(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		plan string
		want bool
	}{
		{"pro plan", PlanPro, true},
		{"free plan", PlanFree, false},
		{"elite plan", PlanElite, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := &Profile{Plan: tt.plan}
			assert.Equal(t, tt.want, p.IsPro())
		})
	}
}

func TestProfile_IsElite(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		plan string
		want bool
	}{
		{"elite plan", PlanElite, true},
		{"free plan", PlanFree, false},
		{"pro plan", PlanPro, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := &Profile{Plan: tt.plan}
			assert.Equal(t, tt.want, p.IsElite())
		})
	}
}

func TestUpdateProfileRequest_ToMap(t *testing.T) {
	t.Parallel()

	t.Run("all nil returns empty map", func(t *testing.T) {
		t.Parallel()
		r := &UpdateProfileRequest{}
		m := r.ToMap()
		assert.Empty(t, m)
	})

	t.Run("single field", func(t *testing.T) {
		t.Parallel()
		r := &UpdateProfileRequest{Plan: ptr("pro")}
		m := r.ToMap()
		assert.Len(t, m, 1)
		assert.Equal(t, "pro", m["plan"])
	})

	t.Run("all fields populated", func(t *testing.T) {
		t.Parallel()
		r := &UpdateProfileRequest{
			PhoneNumber:   ptr("+5511999999999"),
			Plan:          ptr("elite"),
			BillingStatus: ptr("active"),
		}
		m := r.ToMap()
		assert.Len(t, m, 3)
		assert.Equal(t, "+5511999999999", m["phone_number"])
		assert.Equal(t, "elite", m["plan"])
		assert.Equal(t, "active", m["billing_status"])
	})
}
