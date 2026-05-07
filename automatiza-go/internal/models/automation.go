package models

import (
	"encoding/json"
)

// Automation represents an automation configuration in the database
// CRITICAL: Field names must match exactly with database columns for compatibility
type Automation struct {
	ID              string   `json:"id" db:"id"`
	UserID          string   `json:"user_id" db:"user_id"`
	Name            string   `json:"name" db:"name"`
	GroupIDs        []string `json:"group_ids" db:"group_ids"`
	ShopeeLink      *string  `json:"shopee_link,omitempty" db:"shopee_link"`
	CustomMessage   *string  `json:"custom_message,omitempty" db:"custom_message"`
	IncludeImage    bool     `json:"include_image" db:"include_image"`
	ScheduleTime    *string  `json:"schedule_time,omitempty" db:"schedule_time"`
	ScheduleDays    []string `json:"schedule_days,omitempty" db:"schedule_days"`
	Status          string   `json:"status" db:"status"`       // "active" | "inactive" | "paused"
	Type            *string  `json:"type,omitempty" db:"type"` // "search" | "monitoring" | "list"
	MonitorGroupIDs []string `json:"monitor_group_ids,omitempty" db:"monitor_group_ids"`
	DelaySeconds    *int     `json:"delay_seconds,omitempty" db:"delay_seconds"`
	DedupeHours     *int     `json:"dedupe_hours,omitempty" db:"dedupe_hours"`
	DispatchTextID  *string  `json:"dispatch_text_id,omitempty" db:"dispatch_text_id"`
	CreatedAt       string   `json:"created_at" db:"created_at"`
	UpdatedAt       string   `json:"updated_at" db:"updated_at"`

	// Additional fields from database
	Mode            *string  `json:"mode,omitempty" db:"mode"`
	Stores          []string `json:"stores,omitempty" db:"stores"`
	Categories      []string `json:"categories,omitempty" db:"categories"`
	Keyword         *string  `json:"keyword,omitempty" db:"keyword"`
	MinPrice        *float64 `json:"min_price,omitempty" db:"min_price"`
	MaxPrice        *float64 `json:"max_price,omitempty" db:"max_price"`
	MinDiscount     *float64 `json:"min_discount,omitempty" db:"min_discount"`
	SendGroups      []string `json:"send_groups,omitempty" db:"send_groups"`
	StartTime       *string  `json:"start_time,omitempty" db:"start_time"`
	EndTime         *string  `json:"end_time,omitempty" db:"end_time"`
	IntervalMinutes *int     `json:"interval_minutes,omitempty" db:"interval_minutes"`
	LastRunAt       *string  `json:"last_run_at,omitempty" db:"last_run_at"`
	NextRunAt       *string  `json:"next_run_at,omitempty" db:"next_run_at"`

	// Monitoring-specific fields
	MonitoringTextRules *MonitoringTextRules `json:"monitoring_text_rules,omitempty" db:"monitoring_text_rules"`

	// Coupon integration
	CouponID *string `json:"coupon_id,omitempty" db:"coupon_id"`

	// LIST mode specific fields
	MlLinkListID     *string `json:"ml_link_list_id,omitempty" db:"ml_link_list_id"`
	ListItemsPerRun  *int    `json:"list_items_per_run,omitempty" db:"list_items_per_run"`
	ListIncludeImage *bool   `json:"list_include_image,omitempty" db:"list_include_image"`
	ListSendAsImage  *bool   `json:"list_send_as_image,omitempty" db:"list_send_as_image"`

	// Priority and filter settings
	PriorityType *string `json:"priority_type,omitempty" db:"priority_type"` // "discount" | "price" | "commission"
	FilterType   *string `json:"filter_type,omitempty" db:"filter_type"`     // "light" | "heavy"

	// Call-to-action texts (inline, legacy)
	CTAs []string `json:"ctas,omitempty" db:"ctas"`

	// CTA entity IDs (references ctas table)
	CTAIDs []string `json:"cta_ids,omitempty" db:"cta_ids"`

	// Search keywords (multiple)
	Texts []string `json:"texts,omitempty" db:"texts"`

	// Error tracking
	LastError   *string `json:"last_error,omitempty" db:"last_error"`
	LastErrorAt *string `json:"last_error_at,omitempty" db:"last_error_at"`

	// Preferred sender chip (database id of an instance row, string form
	// to match the wire format). NULL = no preference, fall back to
	// per-group ownership. When set and the chip is connected, every
	// dispatch from this automation goes through it.
	InstanceID *string `json:"instance_id,omitempty" db:"instance_id"`

	// Per-automation opt-out for the global coupon rule engine. Default
	// TRUE: the worker matches active CouponRules on every dispatch.
	// Set FALSE when the automation already pulls products from a
	// promo source (e.g. Shopee Big Sale via search mode) and adding
	// our own coupon on top would be redundant.
	ApplyCouponRules bool `json:"apply_coupon_rules" db:"apply_coupon_rules"`
}

// AutomationStatus constants
const (
	AutomationStatusActive   = "active"
	AutomationStatusInactive = "inactive"
	AutomationStatusPaused   = "paused"
)

// AutomationType constants
const (
	AutomationTypeSearch        = "search"
	AutomationTypeMonitoring    = "monitoring"
	AutomationTypeList          = "list"
	AutomationTypeOfferGenerate = "offer_generate"
)

// MonitoringTextRules defines text processing rules for monitoring
type MonitoringTextRules struct {
	Substituicoes   []Substituicao `json:"substituicoes,omitempty"`
	RemoverTermos   []string       `json:"remover_termos,omitempty"`
	Prefixo         string         `json:"prefixo,omitempty"`
	Sufixo          string         `json:"sufixo,omitempty"`
	LimparTelefones bool           `json:"limpar_telefones,omitempty"`
	LimparArrobas   bool           `json:"limpar_arrobas,omitempty"`
}

// Substituicao represents a text substitution rule
type Substituicao struct {
	De   string `json:"de"`
	Para string `json:"para"`
}

// IsActive returns true if the automation is active
func (a *Automation) IsActive() bool {
	return a.Status == AutomationStatusActive
}

// IsMonitoring returns true if this is a monitoring automation
func (a *Automation) IsMonitoring() bool {
	return a.Type != nil && *a.Type == AutomationTypeMonitoring
}

// IsSearch returns true if this is a search automation
func (a *Automation) IsSearch() bool {
	return a.Type == nil || *a.Type == AutomationTypeSearch
}

// IsList returns true if this is a list automation
func (a *Automation) IsList() bool {
	return a.Type != nil && *a.Type == AutomationTypeList
}

// IsOfferGenerate returns true if this is an offer generate automation
func (a *Automation) IsOfferGenerate() bool {
	return a.Type != nil && *a.Type == AutomationTypeOfferGenerate
}

// GetDedupeHours returns the dedupe hours with a default value
func (a *Automation) GetDedupeHours() int {
	if a.DedupeHours != nil {
		return *a.DedupeHours
	}
	return 168 // 7 days default
}

// GetDelaySeconds returns the delay seconds with a default value
func (a *Automation) GetDelaySeconds() int {
	if a.DelaySeconds != nil {
		return *a.DelaySeconds
	}
	return 1
}

// GetTargetGroups returns the destination groups for the automation
func (a *Automation) GetTargetGroups() []string {
	if len(a.SendGroups) > 0 {
		return a.SendGroups
	}
	return a.GroupIDs
}

// GetType returns the automation type with a default value
func (a *Automation) GetType() string {
	if a.Type != nil {
		return *a.Type
	}
	return AutomationTypeSearch
}

// GetCustomMessage returns the custom message or empty string
func (a *Automation) GetCustomMessage() string {
	if a.CustomMessage != nil {
		return *a.CustomMessage
	}
	return ""
}

// CreateAutomationRequest represents a request to create an automation
type CreateAutomationRequest struct {
	Name            string   `json:"name" validate:"required"`
	GroupIDs        []string `json:"group_ids"`
	Type            string   `json:"type,omitempty"`
	Status          string   `json:"status,omitempty"`
	Stores          []string `json:"stores,omitempty"`
	Categories      []string `json:"categories,omitempty"`
	Keyword         *string  `json:"keyword,omitempty"`
	MinPrice        *float64 `json:"min_price,omitempty"`
	MaxPrice        *float64 `json:"max_price,omitempty"`
	MinDiscount     *float64 `json:"min_discount,omitempty"`
	CustomMessage   *string  `json:"custom_message,omitempty"`
	IncludeImage    bool     `json:"include_image"`
	ScheduleTime    *string  `json:"schedule_time,omitempty"`
	ScheduleDays    []string `json:"schedule_days,omitempty"`
	SendGroups      []string `json:"send_groups,omitempty"`
	MonitorGroupIDs []string `json:"monitor_group_ids,omitempty"`
	DelaySeconds    *int     `json:"delay_seconds,omitempty"`
	DedupeHours     *int     `json:"dedupe_hours,omitempty"`
	DispatchTextID  *string  `json:"dispatch_text_id,omitempty"`
	StartTime       *string  `json:"start_time,omitempty"`
	EndTime         *string  `json:"end_time,omitempty"`
	IntervalMinutes *int     `json:"interval_minutes,omitempty"`
	CouponID        *string  `json:"coupon_id,omitempty"`
	InstanceID      *string  `json:"instance_id,omitempty"`
}

// UpdateAutomationRequest represents a request to update an automation
type UpdateAutomationRequest struct {
	Name            *string  `json:"name,omitempty"`
	GroupIDs        []string `json:"group_ids,omitempty"`
	Type            *string  `json:"type,omitempty"`
	Status          *string  `json:"status,omitempty"`
	Stores          []string `json:"stores,omitempty"`
	Categories      []string `json:"categories,omitempty"`
	Keyword         *string  `json:"keyword,omitempty"`
	MinPrice        *float64 `json:"min_price,omitempty"`
	MaxPrice        *float64 `json:"max_price,omitempty"`
	MinDiscount     *float64 `json:"min_discount,omitempty"`
	CustomMessage   *string  `json:"custom_message,omitempty"`
	IncludeImage    *bool    `json:"include_image,omitempty"`
	ScheduleTime    *string  `json:"schedule_time,omitempty"`
	ScheduleDays    []string `json:"schedule_days,omitempty"`
	SendGroups      []string `json:"send_groups,omitempty"`
	MonitorGroupIDs []string `json:"monitor_group_ids,omitempty"`
	DelaySeconds    *int     `json:"delay_seconds,omitempty"`
	DedupeHours     *int     `json:"dedupe_hours,omitempty"`
	DispatchTextID  *string  `json:"dispatch_text_id,omitempty"`
	StartTime       *string  `json:"start_time,omitempty"`
	EndTime         *string  `json:"end_time,omitempty"`
	IntervalMinutes *int     `json:"interval_minutes,omitempty"`
	CouponID        *string  `json:"coupon_id,omitempty"`
	// InstanceID is a tri-state pointer: nil = not provided (no change),
	// pointer to "" = explicitly clear the preference, otherwise = set.
	InstanceID *string `json:"instance_id,omitempty"`
}

// ToMap converts UpdateAutomationRequest to a map for database updates
func (r *UpdateAutomationRequest) ToMap() map[string]interface{} {
	result := make(map[string]interface{})

	if r.Name != nil {
		result["name"] = *r.Name
	}
	if r.GroupIDs != nil {
		result["group_ids"] = r.GroupIDs
	}
	if r.Type != nil {
		result["type"] = *r.Type
	}
	if r.Status != nil {
		result["status"] = *r.Status
	}
	if r.Stores != nil {
		result["stores"] = r.Stores
	}
	if r.Categories != nil {
		result["categories"] = r.Categories
	}
	if r.Keyword != nil {
		result["keyword"] = *r.Keyword
	}
	if r.MinPrice != nil {
		result["min_price"] = *r.MinPrice
	}
	if r.MaxPrice != nil {
		result["max_price"] = *r.MaxPrice
	}
	if r.MinDiscount != nil {
		result["min_discount"] = *r.MinDiscount
	}
	if r.CustomMessage != nil {
		result["custom_message"] = *r.CustomMessage
	}
	if r.IncludeImage != nil {
		result["include_image"] = *r.IncludeImage
	}
	if r.ScheduleTime != nil {
		result["schedule_time"] = *r.ScheduleTime
	}
	if r.ScheduleDays != nil {
		result["schedule_days"] = r.ScheduleDays
	}
	if r.SendGroups != nil {
		result["send_groups"] = r.SendGroups
	}
	if r.MonitorGroupIDs != nil {
		result["monitor_group_ids"] = r.MonitorGroupIDs
	}
	if r.DelaySeconds != nil {
		result["delay_seconds"] = *r.DelaySeconds
	}
	if r.DedupeHours != nil {
		result["dedupe_hours"] = *r.DedupeHours
	}
	if r.DispatchTextID != nil {
		result["dispatch_text_id"] = *r.DispatchTextID
	}
	if r.StartTime != nil {
		result["start_time"] = *r.StartTime
	}
	if r.EndTime != nil {
		result["end_time"] = *r.EndTime
	}
	if r.IntervalMinutes != nil {
		result["interval_minutes"] = *r.IntervalMinutes
	}
	if r.CouponID != nil {
		result["coupon_id"] = *r.CouponID
	}
	if r.InstanceID != nil {
		// Empty string = explicit clear (FK to NULL); non-empty = parsed
		// to int when written, but at this layer we leave it as the raw
		// string and let the repo coerce. Keeping the empty-string path
		// mapped to nil here lets the repo treat both the same.
		if *r.InstanceID == "" {
			result["instance_id"] = nil
		} else {
			result["instance_id"] = *r.InstanceID
		}
	}

	return result
}

// DiagnoseAutomationRequest represents a request to diagnose an automation
type DiagnoseAutomationRequest struct {
	AutomationID string `json:"automationId" validate:"required"`
}

// DiagnoseResult represents the result of automation diagnosis
type DiagnoseResult struct {
	Status   string          `json:"status"` // "ok" | "warning" | "error"
	Checks   []DiagnoseCheck `json:"checks"`
	Warnings []string        `json:"warnings,omitempty"`
	Errors   []string        `json:"errors,omitempty"`
}

// DiagnoseCheck represents a single diagnostic check
type DiagnoseCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // "ok" | "warning" | "error"
	Message string `json:"message,omitempty"`
}

// RunAutomationRequest represents a request to run an automation
type RunAutomationRequest struct {
	AutomationID string `json:"automationId" validate:"required"`
}

// RunAutomationResult represents the result of running an automation
type RunAutomationResult struct {
	Success       bool   `json:"success"`
	Message       string `json:"message,omitempty"`
	ErrorMessage  string `json:"errorMessage,omitempty"`
	OffersFound   int    `json:"offersFound,omitempty"`
	MessagesSent  int    `json:"messagesSent,omitempty"`
	MessagesQueue int    `json:"messagesQueued,omitempty"`
}

// ResetHistoryRequest represents a request to reset automation history
type ResetHistoryRequest struct {
	AutomationID string `json:"automationId" validate:"required"`
}

// MarshalJSON custom marshaler for Automation to handle arrays
func (a *Automation) MarshalJSON() ([]byte, error) {
	type Alias Automation
	return json.Marshal(&struct {
		*Alias
	}{
		Alias: (*Alias)(a),
	})
}
