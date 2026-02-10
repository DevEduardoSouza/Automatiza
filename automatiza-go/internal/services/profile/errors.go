package profile

import "errors"

// Sentinel errors for profile service
var (
	ErrProfileNotFound   = errors.New("profile not found")
	ErrPermissionDenied  = errors.New("permission denied")
	ErrInvalidPassword   = errors.New("invalid password")
	ErrInvalidToken      = errors.New("invalid or expired token")
	ErrTokenExpired      = errors.New("token expired")
	ErrTokenUsed         = errors.New("token already used")
	ErrRateLimitExceeded = errors.New("rate limit exceeded, try again later")
	ErrPasswordTooWeak   = errors.New("password must be at least 8 characters")
	ErrPasswordSameAsOld = errors.New("new password must be different from current")
)
