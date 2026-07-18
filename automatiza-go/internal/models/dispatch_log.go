package models

import "time"

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
	CopyIndex        *int     `json:"copy_index,omitempty" db:"copy_index"`
	// ListItemID liga o dispatch ao ml_link_list_items que o originou (LIST
	// mode). Permite ao reconciliador detectar "item consumido mas nunca
	// entregue" (0 sent, ≥1 failed) e devolvê-lo para retry. NULL fora do LIST.
	ListItemID *string `json:"list_item_id,omitempty" db:"list_item_id"`
}

// couponCodeMaxLen is the width of dispatch_logs.coupon_code (VARCHAR(50) in
// 000_postgresql_init.sql). Rule codes come from coupon_rules.code (VARCHAR(100)),
// so the audit copy must be truncated or the INSERT overflows (SQLSTATE 22001)
// and the whole dispatch log is lost on a send that already went out.
const couponCodeMaxLen = 50

// SetCouponAudit records the coupon outcome on the dispatch log for support
// auditing. A coupon counts as "evaluated" only if it applied OR produced a
// skip reason; a zero result (coupon never evaluated — no rules engine, no
// price) leaves ALL coupon columns NULL, the semantics support queries rely on
// (NULL = not evaluated; applied=false + reason = evaluated and skipped;
// applied=true = injected). code is truncated to the column width. coupon_id is
// deliberately NOT written here: that column is declared as a coupons(id)
// reference, and the rule engine's id is a coupon_rules.id — writing it would
// break that contract, so the rule is identified by code + user + timestamp.
func (l *DispatchLog) SetCouponAudit(applied bool, code, skipReason string) {
	if !applied && skipReason == "" {
		return
	}
	a := applied
	l.CouponApplied = &a
	if code != "" {
		if r := []rune(code); len(r) > couponCodeMaxLen {
			code = string(r[:couponCodeMaxLen])
		}
		l.CouponCode = &code
	}
	if skipReason != "" {
		l.CouponSkipReason = &skipReason
	}
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
	AutomationID *string    `json:"automation_id,omitempty"`
	GroupID      *string    `json:"group_id,omitempty"`
	Status       *string    `json:"status,omitempty"`
	Store        *string    `json:"store,omitempty"`
	SearchURL    *string    `json:"search_url,omitempty"`
	Since        *time.Time `json:"since,omitempty"`
}
