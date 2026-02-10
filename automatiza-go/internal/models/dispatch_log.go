package models

// DispatchLog represents a dispatch log entry in the database
type DispatchLog struct {
	ID               int      `json:"id" db:"id"`
	UserID           string   `json:"user_id" db:"user_id"`
	AutomationID     *string  `json:"automation_id,omitempty" db:"automation_id"`
	AutomationName   *string  `json:"automation_name,omitempty" db:"automation_name"`
	Store            *string  `json:"store,omitempty" db:"store"`
	GroupID          *string  `json:"group_id,omitempty" db:"group_id"`
	ProductURL       *string  `json:"product_url,omitempty" db:"product_url"`
	ProductPageURL   *string  `json:"product_page_url,omitempty" db:"product_page_url"`
	CanonicalKey     *string  `json:"canonical_key,omitempty" db:"canonical_key"`
	Status           string   `json:"status" db:"status"`
	Error            *string  `json:"error,omitempty" db:"error"`
	Type             *string  `json:"type,omitempty" db:"type"`
	CreatedAt        string   `json:"created_at" db:"created_at"`
	ProductCategory  *string  `json:"product_category,omitempty" db:"product_category"`
	ProductPrice     *float64 `json:"product_price,omitempty" db:"product_price"`
	ProductDiscount  *float64 `json:"product_discount,omitempty" db:"product_discount"`
	AffiliateLink    *string  `json:"affiliate_link,omitempty" db:"affiliate_link"`
	CouponID         *string  `json:"coupon_id,omitempty" db:"coupon_id"`
	CouponCode       *string  `json:"coupon_code,omitempty" db:"coupon_code"`
	CouponApplied    *bool    `json:"coupon_applied,omitempty" db:"coupon_applied"`
	CouponSkipReason *string  `json:"coupon_skip_reason,omitempty" db:"coupon_skip_reason"`
}

// DispatchLogStatus constants
const (
	DispatchLogStatusPending    = "pending"
	DispatchLogStatusQueued     = "queued"
	DispatchLogStatusSent       = "sent"
	DispatchLogStatusFailed     = "failed"
	DispatchLogStatusSkipped    = "skipped"
	DispatchLogStatusBlocked    = "blocked"
	DispatchLogStatusReconciled = "reconciled" // Created during reconciliation/backfill
)

// CreateDispatchLogRequest represents a request to create a dispatch log
type CreateDispatchLogRequest struct {
	UserID         string  `json:"user_id" validate:"required"`
	AutomationID   *string `json:"automation_id,omitempty"`
	AutomationName *string `json:"automation_name,omitempty"`
	Store          *string `json:"store,omitempty"`
	GroupID        *string `json:"group_id,omitempty"`
	ProductURL     *string `json:"product_url,omitempty"`
	ProductPageURL *string `json:"product_page_url,omitempty"`
	CanonicalKey   *string `json:"canonical_key,omitempty"`
	Status         string  `json:"status" validate:"required"`
	Type           *string `json:"type,omitempty"`
}

// UpdateDispatchLogRequest represents a request to update a dispatch log
type UpdateDispatchLogRequest struct {
	Status *string `json:"status,omitempty"`
	Error  *string `json:"error,omitempty"`
}

// ToMap converts UpdateDispatchLogRequest to a map for database updates
func (r *UpdateDispatchLogRequest) ToMap() map[string]interface{} {
	result := make(map[string]interface{})

	if r.Status != nil {
		result["status"] = *r.Status
	}
	if r.Error != nil {
		result["error"] = *r.Error
	}

	return result
}

// DispatchLogQuery represents query parameters for listing dispatch logs
type DispatchLogQuery struct {
	UserID       string  `json:"user_id"`
	AutomationID *string `json:"automation_id,omitempty"`
	GroupID      *string `json:"group_id,omitempty"`
	Status       *string `json:"status,omitempty"`
	Limit        int     `json:"limit"`
	Offset       int     `json:"offset"`
}

// DispatchLogFilters represents filter parameters for querying dispatch logs
type DispatchLogFilters struct {
	AutomationID *string `json:"automation_id,omitempty"`
	GroupID      *string `json:"group_id,omitempty"`
	Status       *string `json:"status,omitempty"`
	Store        *string `json:"store,omitempty"`
	SearchURL    *string `json:"search_url,omitempty"`
}
