package errors_test

import (
	"errors"
	"fmt"
	"testing"

	apperrors "automatiza-go/internal/shared/errors"
)

// TestSentinelErrors_Exist verifies that all 26 sentinel error variables
// are non-nil and carry the expected message string.
func TestSentinelErrors_Exist(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		err     error
		message string
	}{
		// Authentication errors
		{"ErrInvalidCredentials", apperrors.ErrInvalidCredentials, "invalid_credentials"},
		{"ErrAccountLocked", apperrors.ErrAccountLocked, "account_locked"},
		{"ErrAccountSuspended", apperrors.ErrAccountSuspended, "account_suspended"},
		{"ErrSessionExpired", apperrors.ErrSessionExpired, "session_expired"},
		{"ErrSessionNotFound", apperrors.ErrSessionNotFound, "session_not_found"},
		{"ErrInvalidSession", apperrors.ErrInvalidSession, "invalid_session"},

		// Token errors
		{"ErrInvalidToken", apperrors.ErrInvalidToken, "invalid_token"},
		{"ErrTokenExpired", apperrors.ErrTokenExpired, "token_expired"},
		{"ErrTokenUsed", apperrors.ErrTokenUsed, "token_used"},
		{"ErrInvalidCSRF", apperrors.ErrInvalidCSRF, "invalid_csrf"},

		// Rate limiting errors
		{"ErrRateLimited", apperrors.ErrRateLimited, "rate_limited"},
		{"ErrIPBlocked", apperrors.ErrIPBlocked, "ip_blocked"},

		// Registration errors
		{"ErrEmailExists", apperrors.ErrEmailExists, "email_exists"},
		{"ErrWeakPassword", apperrors.ErrWeakPassword, "weak_password"},
		{"ErrInvalidEmail", apperrors.ErrInvalidEmail, "invalid_email"},
		{"ErrEmailNotVerified", apperrors.ErrEmailNotVerified, "email_not_verified"},

		// Authorization errors
		{"ErrUnauthorized", apperrors.ErrUnauthorized, "unauthorized"},
		{"ErrNotAdmin", apperrors.ErrNotAdmin, "not_admin"},
		{"ErrPermissionDenied", apperrors.ErrPermissionDenied, "permission_denied"},
		{"ErrInsufficientRole", apperrors.ErrInsufficientRole, "insufficient_role"},

		// User errors
		{"ErrUserNotFound", apperrors.ErrUserNotFound, "user_not_found"},
		{"ErrSelfPasswordReset", apperrors.ErrSelfPasswordReset, "self_password_reset"},
		{"ErrInvalidUserID", apperrors.ErrInvalidUserID, "invalid_user_id"},

		// Database errors
		{"ErrDuplicateEntry", apperrors.ErrDuplicateEntry, "duplicate_entry"},
		{"ErrInvalidReference", apperrors.ErrInvalidReference, "invalid_reference"},
		{"ErrNotFound", apperrors.ErrNotFound, "not_found"},
		{"ErrRetryable", apperrors.ErrRetryable, "retryable"},
		{"ErrTimeout", apperrors.ErrTimeout, "timeout"},

		// External service errors
		{"ErrEmailSendFailed", apperrors.ErrEmailSendFailed, "email_send_failed"},
		{"ErrRedisUnavailable", apperrors.ErrRedisUnavailable, "redis_unavailable"},
		{"ErrExternalServiceDown", apperrors.ErrExternalServiceDown, "external_service_down"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.err == nil {
				t.Fatalf("expected %s to be non-nil", tt.name)
			}
			if got := tt.err.Error(); got != tt.message {
				t.Errorf("%s.Error() = %q, want %q", tt.name, got, tt.message)
			}
		})
	}
}

// TestSentinelErrors_ErrorsIs verifies that errors.Is works correctly
// for sentinel errors, both positive and negative matching.
func TestSentinelErrors_ErrorsIs(t *testing.T) {
	t.Parallel()

	t.Run("direct match", func(t *testing.T) {
		t.Parallel()

		if !errors.Is(apperrors.ErrNotFound, apperrors.ErrNotFound) {
			t.Error("expected errors.Is(ErrNotFound, ErrNotFound) to be true")
		}
	})

	t.Run("wrapped match", func(t *testing.T) {
		t.Parallel()

		wrapped := fmt.Errorf("wrapped: %w", apperrors.ErrNotFound)
		if !errors.Is(wrapped, apperrors.ErrNotFound) {
			t.Error("expected errors.Is on wrapped ErrNotFound to be true")
		}
	})

	t.Run("no match between different sentinels", func(t *testing.T) {
		t.Parallel()

		if errors.Is(apperrors.ErrNotFound, apperrors.ErrTimeout) {
			t.Error("expected errors.Is(ErrNotFound, ErrTimeout) to be false")
		}
	})

	t.Run("no match with unrelated error", func(t *testing.T) {
		t.Parallel()

		other := errors.New("something else")
		if errors.Is(other, apperrors.ErrNotFound) {
			t.Error("expected errors.Is(other, ErrNotFound) to be false")
		}
	})

	t.Run("double wrapped match", func(t *testing.T) {
		t.Parallel()

		inner := fmt.Errorf("inner: %w", apperrors.ErrUnauthorized)
		outer := fmt.Errorf("outer: %w", inner)
		if !errors.Is(outer, apperrors.ErrUnauthorized) {
			t.Error("expected errors.Is on double-wrapped ErrUnauthorized to be true")
		}
	})
}

// TestSentinelErrors_Uniqueness ensures all sentinel errors are distinct.
func TestSentinelErrors_Uniqueness(t *testing.T) {
	t.Parallel()

	allErrors := []error{
		apperrors.ErrInvalidCredentials,
		apperrors.ErrAccountLocked,
		apperrors.ErrAccountSuspended,
		apperrors.ErrSessionExpired,
		apperrors.ErrSessionNotFound,
		apperrors.ErrInvalidSession,
		apperrors.ErrInvalidToken,
		apperrors.ErrTokenExpired,
		apperrors.ErrTokenUsed,
		apperrors.ErrInvalidCSRF,
		apperrors.ErrRateLimited,
		apperrors.ErrIPBlocked,
		apperrors.ErrEmailExists,
		apperrors.ErrWeakPassword,
		apperrors.ErrInvalidEmail,
		apperrors.ErrEmailNotVerified,
		apperrors.ErrUnauthorized,
		apperrors.ErrNotAdmin,
		apperrors.ErrPermissionDenied,
		apperrors.ErrInsufficientRole,
		apperrors.ErrUserNotFound,
		apperrors.ErrSelfPasswordReset,
		apperrors.ErrInvalidUserID,
		apperrors.ErrDuplicateEntry,
		apperrors.ErrInvalidReference,
		apperrors.ErrNotFound,
		apperrors.ErrRetryable,
		apperrors.ErrTimeout,
		apperrors.ErrEmailSendFailed,
		apperrors.ErrRedisUnavailable,
		apperrors.ErrExternalServiceDown,
	}

	seen := make(map[string]int)
	for i, err := range allErrors {
		msg := err.Error()
		if prevIdx, ok := seen[msg]; ok {
			t.Errorf("sentinel errors at index %d and %d share the same message %q", prevIdx, i, msg)
		}
		seen[msg] = i
	}
}
