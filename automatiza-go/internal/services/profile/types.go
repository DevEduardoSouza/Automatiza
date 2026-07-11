package profile

import "automatiza-go/internal/models"

// RequestResetInput contains data for password reset request
type RequestResetInput struct {
	Email     string
	IPAddress string
	UserAgent string
}

// ValidateTokenResult contains the result of token validation
type ValidateTokenResult struct {
	Valid  bool
	Email  string // Masked email if valid
	Reason string // "invalid_token", "token_expired", "token_used"
}

// ResetPasswordInput contains data for password reset
type ResetPasswordInput struct {
	Token       string
	NewPassword string
	IPAddress   string
	UserAgent   string
}

// ChangePasswordInput contains data for password change (logged in user)
type ChangePasswordInput struct {
	CurrentPassword string
	NewPassword     string
	IPAddress       string
}

// UpdateProfileInput contains data for profile update
type UpdateProfileInput struct {
	PhoneNumber *string
}

// UserLimits contains user limits and counts
type UserLimits struct {
	AutomationsCount        int32
	MaxAutomations          int32
	ConnectedInstancesCount int32
	// GroupLimit is the max distinct destination groups the user's plan allows
	// across active automations. 0 = unlimited (legacy plans / no tiered plan).
	GroupLimit int32
	// GroupsUsed is the current distinct destination-group count across the
	// user's ACTIVE automations (mirrors the per-plan enforcement count).
	GroupsUsed int32
	// MaxInstances is the account's chip cap — the same number the server
	// enforces on create, so the UI can stop keeping its own copy.
	MaxInstances int32
}

// ProfilesPage contains a page of profiles for admin listing
type ProfilesPage struct {
	Profiles   []*models.Profile
	NextCursor string
	HasMore    bool
}
