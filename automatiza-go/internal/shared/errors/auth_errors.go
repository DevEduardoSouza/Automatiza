// Package errors provides standardized error types for the application.
package errors

import "errors"

// Authentication errors
var (
	// ErrInvalidCredentials indicates invalid email or password.
	ErrInvalidCredentials = errors.New("invalid_credentials")

	// ErrAccountLocked indicates the account is locked due to too many failed attempts.
	ErrAccountLocked = errors.New("account_locked")

	// ErrAccountSuspended indicates the account is suspended due to billing issues.
	ErrAccountSuspended = errors.New("account_suspended")

	// ErrSessionExpired indicates the session has expired.
	ErrSessionExpired = errors.New("session_expired")

	// ErrSessionNotFound indicates the session was not found.
	ErrSessionNotFound = errors.New("session_not_found")

	// ErrInvalidSession indicates the session is invalid.
	ErrInvalidSession = errors.New("invalid_session")
)

// Token errors
var (
	// ErrInvalidToken indicates the token is invalid.
	ErrInvalidToken = errors.New("invalid_token")

	// ErrTokenExpired indicates the token has expired.
	ErrTokenExpired = errors.New("token_expired")

	// ErrTokenUsed indicates the token has already been used.
	ErrTokenUsed = errors.New("token_used")

	// ErrInvalidCSRF indicates the CSRF token is invalid.
	ErrInvalidCSRF = errors.New("invalid_csrf")
)

// Rate limiting errors
var (
	// ErrRateLimited indicates too many requests have been made.
	ErrRateLimited = errors.New("rate_limited")

	// ErrIPBlocked indicates the IP address is blocked.
	ErrIPBlocked = errors.New("ip_blocked")
)

// Registration errors
var (
	// ErrEmailExists indicates the email is already registered.
	ErrEmailExists = errors.New("email_exists")

	// ErrWeakPassword indicates the password does not meet complexity requirements.
	ErrWeakPassword = errors.New("weak_password")

	// ErrInvalidEmail indicates the email format is invalid.
	ErrInvalidEmail = errors.New("invalid_email")

	// ErrEmailNotVerified indicates the email has not been verified.
	ErrEmailNotVerified = errors.New("email_not_verified")
)

// Authorization errors
var (
	// ErrUnauthorized indicates the user is not authenticated.
	ErrUnauthorized = errors.New("unauthorized")

	// ErrNotAdmin indicates the user is not an admin.
	ErrNotAdmin = errors.New("not_admin")

	// ErrPermissionDenied indicates the user does not have permission.
	ErrPermissionDenied = errors.New("permission_denied")

	// ErrInsufficientRole indicates the user's role is insufficient.
	ErrInsufficientRole = errors.New("insufficient_role")
)

// User errors
var (
	// ErrUserNotFound indicates the user was not found.
	ErrUserNotFound = errors.New("user_not_found")

	// ErrSelfPasswordReset indicates a user tried to reset their own password via admin.
	ErrSelfPasswordReset = errors.New("self_password_reset")

	// ErrInvalidUserID indicates the user ID is invalid.
	ErrInvalidUserID = errors.New("invalid_user_id")
)

// Database errors
var (
	// ErrDuplicateEntry indicates a duplicate entry constraint violation.
	ErrDuplicateEntry = errors.New("duplicate_entry")

	// ErrInvalidReference indicates a foreign key constraint violation.
	ErrInvalidReference = errors.New("invalid_reference")

	// ErrNotFound indicates the requested record was not found.
	ErrNotFound = errors.New("not_found")

	// ErrRetryable indicates the operation should be retried.
	ErrRetryable = errors.New("retryable")

	// ErrTimeout indicates the operation timed out.
	ErrTimeout = errors.New("timeout")
)

// External service errors
var (
	// ErrEmailSendFailed indicates email sending failed.
	ErrEmailSendFailed = errors.New("email_send_failed")

	// ErrRedisUnavailable indicates Redis is not available.
	ErrRedisUnavailable = errors.New("redis_unavailable")

	// ErrExternalServiceDown indicates an external service is down.
	ErrExternalServiceDown = errors.New("external_service_down")
)
