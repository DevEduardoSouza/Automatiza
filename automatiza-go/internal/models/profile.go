package models

import "time"

// Profile represents a user profile in the database
type Profile struct {
	ID                   string     `json:"id" db:"id"`
	Email                string     `json:"email" db:"email"`
	Plan                 string     `json:"plan" db:"plan"` // free|pro|elite
	PhoneNumber          *string    `json:"phone_number,omitempty" db:"phone_number"`
	PasswordHash         *string    `json:"-" db:"password_hash"` // Never expose in JSON
	BillingStatus        *string    `json:"billing_status,omitempty" db:"billing_status"`
	BillingNextPaymentAt *time.Time `json:"billing_next_payment_at,omitempty" db:"billing_next_payment_at"`
	PlanAccessUntil      *time.Time `json:"plan_access_until,omitempty" db:"plan_access_until"`
	MaxAutomations       *int       `json:"max_automations,omitempty" db:"max_automations"`
	MaxInstances         *int       `json:"max_instances,omitempty" db:"max_instances"`
	CreatedAt            string     `json:"created_at" db:"created_at"`
	UpdatedAt            string     `json:"updated_at" db:"updated_at"`
}

// Plan constants
const (
	PlanFree  = "free"
	PlanPro   = "pro"
	PlanElite = "elite"
)

// BillingStatus constants
const (
	BillingStatusActive    = "active"
	BillingStatusSuspended = "suspended"
	BillingStatusCancelled = "canceled"
)

// IsSuspended returns true if the profile is suspended
func (p *Profile) IsSuspended() bool {
	return p.BillingStatus != nil && *p.BillingStatus == BillingStatusSuspended
}

// IsFree returns true if the profile is on free plan
func (p *Profile) IsFree() bool {
	return p.Plan == PlanFree
}

// IsPro returns true if the profile is on pro plan
func (p *Profile) IsPro() bool {
	return p.Plan == PlanPro
}

// IsElite returns true if the profile is on elite plan
func (p *Profile) IsElite() bool {
	return p.Plan == PlanElite
}

// UserRole represents a user role in the database
type UserRole struct {
	ID        string `json:"id" db:"id"`
	UserID    string `json:"user_id" db:"user_id"`
	Role      string `json:"role" db:"role"`
	CreatedAt string `json:"created_at" db:"created_at"`
}

// Role constants
const (
	RoleAdmin = "admin"
	RoleUser  = "user"
)

// UpdateProfileRequest represents a request to update a profile
type UpdateProfileRequest struct {
	PhoneNumber   *string `json:"phone_number,omitempty"`
	Plan          *string `json:"plan,omitempty"`
	BillingStatus *string `json:"billing_status,omitempty"`
}

// ToMap converts UpdateProfileRequest to a map for database updates
func (r *UpdateProfileRequest) ToMap() map[string]interface{} {
	result := make(map[string]interface{})

	if r.PhoneNumber != nil {
		result["phone_number"] = *r.PhoneNumber
	}
	if r.Plan != nil {
		result["plan"] = *r.Plan
	}
	if r.BillingStatus != nil {
		result["billing_status"] = *r.BillingStatus
	}

	return result
}

// WebhookUserData represents user data from a webhook (e.g., Kiwify)
type WebhookUserData struct {
	Email       string  `json:"email"`
	Name        *string `json:"name,omitempty"`
	PhoneNumber *string `json:"phone_number,omitempty"`
}

// PasswordResetResult represents the result of a password reset
type PasswordResetResult struct {
	Success     bool   `json:"success"`
	NewPassword string `json:"newPassword,omitempty"`
	Message     string `json:"message,omitempty"`
}
