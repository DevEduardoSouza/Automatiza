package models

// ActionHistory represents an action history entry in the database
type ActionHistory struct {
	ID           string      `json:"id" db:"id"`
	UserID       string      `json:"user_id" db:"user_id"`
	ActionType   string      `json:"action_type" db:"action_type"`
	Description  string      `json:"description" db:"description"`
	Details      interface{} `json:"details,omitempty" db:"details"`
	Status       string      `json:"status" db:"status"`
	ErrorMessage *string     `json:"error_message,omitempty" db:"error_message"`
	TargetCount  *int32      `json:"target_count,omitempty" db:"target_count"`
	SuccessCount *int32      `json:"success_count,omitempty" db:"success_count"`
	ErrorCount   *int32      `json:"error_count,omitempty" db:"error_count"`
	CreatedAt    string      `json:"created_at" db:"created_at"`
	CompletedAt  *string     `json:"completed_at,omitempty" db:"completed_at"`
}

// ActionType constants
const (
	ActionTypeSendMessage       = "send_message"
	ActionTypeSyncGroups        = "sync_groups"
	ActionTypeRunAutomation     = "run_automation"
	ActionTypeSendCampaign      = "send_campaign"
	ActionTypeGenerateImage     = "generate_image"
	ActionTypeCloseGroups       = "close_groups"
	ActionTypeOpenGroups        = "open_groups"
	ActionTypeChangeName        = "change_name"
	ActionTypeChangePhoto       = "change_photo"
	ActionTypeChangeDescription = "change_description"

	// Group messaging action types
	ActionTypeSendAudio = "send_audio"
	ActionTypeSendImage = "send_image"
	ActionTypeSendPoll  = "send_poll"

	// Group member management action types
	ActionTypeAddMember     = "add_member"
	ActionTypeRemoveMember  = "remove_member"
	ActionTypeLeaveGroup    = "leave_group"
	ActionTypePromoteMember = "promote_member"
	ActionTypeDemoteMember  = "demote_member"

	// Admin action types
	ActionTypeAdminUnlockAccount   = "admin_unlock_account"
	ActionTypeAdminResetPassword   = "admin_reset_password"
	ActionTypeAdminGrantRole       = "admin_grant_role"
	ActionTypeAdminRevokeRole      = "admin_revoke_role"

	// ML Link List action types
	ActionTypeMLLinkListCreate     = "ml_link_list_create"
	ActionTypeMLLinkListUpdate     = "ml_link_list_update"
	ActionTypeMLLinkListDelete     = "ml_link_list_delete"
	ActionTypeMLLinkItemCreate     = "ml_link_item_create"
	ActionTypeMLLinkItemBulkCreate = "ml_link_item_bulk_create"
	ActionTypeMLLinkItemUpdate     = "ml_link_item_update"
	ActionTypeMLLinkItemDelete     = "ml_link_item_delete"
	ActionTypeMLLinkItemsReset     = "ml_link_items_reset"
	ActionTypeMLLinkItemsReorder   = "ml_link_items_reorder"
)

// ActionStatus constants
const (
	ActionStatusPending   = "pending"
	ActionStatusRunning   = "running"
	ActionStatusCompleted = "completed"
	ActionStatusFailed    = "failed"
)

// CreateActionHistoryRequest represents parameters to create a history entry
type CreateActionHistoryRequest struct {
	UserID      string                 `json:"user_id" validate:"required"`
	ActionType  string                 `json:"action_type" validate:"required"`
	Description string                 `json:"description" validate:"required"`
	Details     map[string]interface{} `json:"details,omitempty"`
	TargetCount *int32                 `json:"target_count,omitempty"`
}

// UpdateActionHistoryRequest represents parameters to update a history entry
type UpdateActionHistoryRequest struct {
	Status       *string                `json:"status,omitempty"`
	ErrorMessage *string                `json:"error_message,omitempty"`
	CompletedAt  *string                `json:"completed_at,omitempty"`
	Details      map[string]interface{} `json:"details,omitempty"`
	TargetCount  *int32                 `json:"target_count,omitempty"`
	SuccessCount *int32                 `json:"success_count,omitempty"`
	ErrorCount   *int32                 `json:"error_count,omitempty"`
}

// HistoryListRequest represents a request to list history entries
type HistoryListRequest struct {
	Limit      int     `json:"limit" form:"limit"`
	Offset     int     `json:"offset" form:"offset"`
	ActionType *string `json:"action_type,omitempty" form:"action_type"`
	Status     *string `json:"status,omitempty" form:"status"`
}

// HistoryStatsResult holds aggregated counts by status from a COUNT(*) GROUP BY query.
type HistoryStatsResult struct {
	Total     int32
	Completed int32
	Running   int32
	Failed    int32
}
