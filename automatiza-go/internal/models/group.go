package models

// Group represents a WhatsApp group in the database
// IMPORTANT: Field names must match exactly with database columns for compatibility
type Group struct {
	ID           string  `json:"id" db:"id"`
	Name         string  `json:"name" db:"name"`
	WAGroupID    string  `json:"wa_group_id" db:"wa_group_id"`
	InstanceID   string  `json:"instance_id" db:"instance_id"`
	AdminOnly    bool    `json:"admin_only" db:"admin_only"`
	Description  *string `json:"description,omitempty" db:"description"`
	PictureURL   *string `json:"picture_url,omitempty" db:"picture_url"`
	UserID       string  `json:"user_id" db:"user_id"`
	IsAdmin      bool    `json:"is_admin" db:"is_admin"`
	IsFavorite   bool    `json:"is_favorite" db:"is_favorite"`
	Status       string  `json:"status" db:"status"`
	InviteCode   *string `json:"invite_code,omitempty" db:"invite_code"`
	MembersCount int     `json:"members_count" db:"members_count"`
	MemberLimit  int     `json:"member_limit" db:"member_limit"`
	CreatedAt    string  `json:"created_at" db:"created_at"`
	UpdatedAt    string  `json:"updated_at" db:"updated_at"`
}

// GroupStatus constants
const (
	GroupStatusOpen   = "open"
	GroupStatusClosed = "closed"
	GroupStatusFull   = "full"
)

// WahaGroup represents a group from Waha API
type WahaGroup struct {
	ID           string            `json:"id"`
	Subject      string            `json:"subject"`
	Description  string            `json:"description,omitempty"`
	Owner        string            `json:"owner,omitempty"`
	Creation     int64             `json:"creation,omitempty"`
	Announce     bool              `json:"announce"` // admin-only mode
	Restrict     bool              `json:"restrict"`
	Size         int               `json:"size,omitempty"`
	Participants []WahaParticipant `json:"participants,omitempty"`
}

// WahaParticipant represents a participant in a Waha group
type WahaParticipant struct {
	ID           string `json:"id"`
	Admin        string `json:"admin,omitempty"` // "admin" | "superadmin" | ""
	IsAdmin      bool   `json:"isAdmin,omitempty"`
	IsSuperAdmin bool   `json:"isSuperAdmin,omitempty"`
}

// GroupSyncRequest represents a request to sync groups
type GroupSyncRequest struct {
	InstanceName string `json:"instanceName" validate:"required"`
}

// GroupSyncOptions represents options for group syncing
type GroupSyncOptions struct {
	BatchSize      int  `json:"batchSize"`
	IncludeMembers bool `json:"includeMembers"`
}

// GroupUpdateSubjectRequest represents a request to update group subject
type GroupUpdateSubjectRequest struct {
	InstanceName string `json:"instanceName" validate:"required"`
	GroupID      string `json:"groupId" validate:"required"`
	Subject      string `json:"subject" validate:"required"`
}

// GroupUpdateDescriptionRequest represents a request to update group description
type GroupUpdateDescriptionRequest struct {
	InstanceName string `json:"instanceName" validate:"required"`
	GroupID      string `json:"groupId" validate:"required"`
	Description  string `json:"description" validate:"required"`
}

// GroupUpdatePictureRequest represents a request to update group picture
type GroupUpdatePictureRequest struct {
	InstanceName string `json:"instanceName" validate:"required"`
	GroupID      string `json:"groupId" validate:"required"`
	ImageBase64  string `json:"imageBase64" validate:"required"`
}

// GroupUpdateSettingsRequest represents a request to update group settings
type GroupUpdateSettingsRequest struct {
	InstanceName string `json:"instanceName" validate:"required"`
	GroupID      string `json:"groupId" validate:"required"`
	Action       string `json:"action" validate:"required,oneof=announcement not_announcement"` // announcement or not_announcement
}

// ToGroup converts a WahaGroup to a Group model
func (wg *WahaGroup) ToGroup(userID, instanceID string) *Group {
	return &Group{
		WAGroupID:    wg.ID,
		Name:         wg.Subject,
		Description:  &wg.Description,
		InstanceID:   instanceID,
		UserID:       userID,
		AdminOnly:    wg.Announce,
		MembersCount: wg.Size,
	}
}
