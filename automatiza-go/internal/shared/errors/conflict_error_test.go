package errors_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	apperrors "automatiza-go/internal/shared/errors"
)

// TestNewConflictError verifies that NewConflictError creates an error
// with the given resource and reason and a nil LockUntil.
func TestNewConflictError(t *testing.T) {
	t.Parallel()

	err := apperrors.NewConflictError("automation", "already running")

	if err.Resource != "automation" {
		t.Errorf("Resource = %q, want %q", err.Resource, "automation")
	}
	if err.Reason != "already running" {
		t.Errorf("Reason = %q, want %q", err.Reason, "already running")
	}
	if err.LockUntil != nil {
		t.Errorf("LockUntil = %v, want nil", err.LockUntil)
	}
}

// TestNewLockConflictError verifies lock conflict creation with time info.
func TestNewLockConflictError(t *testing.T) {
	t.Parallel()

	t.Run("future time", func(t *testing.T) {
		t.Parallel()

		futureTime := time.Now().Add(30 * time.Minute)
		err := apperrors.NewLockConflictError("session", futureTime)

		if err.Resource != "session" {
			t.Errorf("Resource = %q, want %q", err.Resource, "session")
		}
		if !strings.Contains(err.Reason, "resource is locked until") {
			t.Errorf("Reason = %q, want it to contain 'resource is locked until'", err.Reason)
		}
		if !strings.Contains(err.Reason, futureTime.Format(time.RFC3339)) {
			t.Errorf("Reason = %q, want it to contain the formatted time %s", err.Reason, futureTime.Format(time.RFC3339))
		}
		if err.LockUntil == nil {
			t.Fatal("LockUntil should not be nil")
		}
		if !err.LockUntil.Equal(futureTime) {
			t.Errorf("LockUntil = %v, want %v", *err.LockUntil, futureTime)
		}
	})

	t.Run("past time", func(t *testing.T) {
		t.Parallel()

		pastTime := time.Now().Add(-10 * time.Minute)
		err := apperrors.NewLockConflictError("job", pastTime)

		if err.Resource != "job" {
			t.Errorf("Resource = %q, want %q", err.Resource, "job")
		}
		if err.LockUntil == nil {
			t.Fatal("LockUntil should not be nil even for past time")
		}
		if !err.LockUntil.Equal(pastTime) {
			t.Errorf("LockUntil = %v, want %v", *err.LockUntil, pastTime)
		}
	})
}

// TestConflictError_Error verifies the Error() string format.
func TestConflictError_Error(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		resource string
		reason   string
		want     string
	}{
		{
			name:     "basic conflict",
			resource: "automation",
			reason:   "already running",
			want:     "conflict: automation - already running",
		},
		{
			name:     "empty fields",
			resource: "",
			reason:   "",
			want:     "conflict:  - ",
		},
		{
			name:     "special characters",
			resource: "user/profile",
			reason:   "locked (admin)",
			want:     "conflict: user/profile - locked (admin)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := apperrors.NewConflictError(tt.resource, tt.reason)
			if got := err.Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestConflictError_Unwrap verifies that Unwrap returns nil.
func TestConflictError_Unwrap(t *testing.T) {
	t.Parallel()

	err := apperrors.NewConflictError("resource", "reason")
	if unwrapped := err.Unwrap(); unwrapped != nil {
		t.Errorf("Unwrap() = %v, want nil", unwrapped)
	}
}

// TestConflictError_ImplementsErrorInterface verifies interface compliance.
func TestConflictError_ImplementsErrorInterface(t *testing.T) {
	t.Parallel()

	var _ error = apperrors.NewConflictError("r", "r")
}

// TestIsConflictError verifies the type-check helper.
func TestIsConflictError(t *testing.T) {
	t.Parallel()

	t.Run("with ConflictError", func(t *testing.T) {
		t.Parallel()

		err := apperrors.NewConflictError("resource", "reason")
		if !apperrors.IsConflictError(err) {
			t.Error("expected IsConflictError to return true for ConflictError")
		}
	})

	t.Run("with lock ConflictError", func(t *testing.T) {
		t.Parallel()

		err := apperrors.NewLockConflictError("resource", time.Now().Add(time.Hour))
		if !apperrors.IsConflictError(err) {
			t.Error("expected IsConflictError to return true for lock ConflictError")
		}
	})

	t.Run("with other error", func(t *testing.T) {
		t.Parallel()

		err := errors.New("some other error")
		if apperrors.IsConflictError(err) {
			t.Error("expected IsConflictError to return false for non-ConflictError")
		}
	})

	t.Run("with nil", func(t *testing.T) {
		t.Parallel()

		if apperrors.IsConflictError(nil) {
			t.Error("expected IsConflictError to return false for nil")
		}
	})

	t.Run("with wrapped ConflictError", func(t *testing.T) {
		t.Parallel()

		inner := apperrors.NewConflictError("resource", "reason")
		wrapped := fmt.Errorf("wrapped: %w", inner)
		// IsConflictError uses type assertion (not errors.As), so wrapped errors
		// should NOT match.
		if apperrors.IsConflictError(wrapped) {
			t.Error("expected IsConflictError to return false for wrapped ConflictError (uses type assertion)")
		}
	})
}

// TestGetRemainingLockTime verifies remaining lock time calculations.
func TestGetRemainingLockTime(t *testing.T) {
	t.Parallel()

	t.Run("nil LockUntil returns zero", func(t *testing.T) {
		t.Parallel()

		err := apperrors.NewConflictError("resource", "reason")
		remaining := err.GetRemainingLockTime()
		if remaining != 0 {
			t.Errorf("GetRemainingLockTime() = %v, want 0", remaining)
		}
	})

	t.Run("future LockUntil returns positive duration", func(t *testing.T) {
		t.Parallel()

		futureTime := time.Now().Add(30 * time.Minute)
		err := apperrors.NewLockConflictError("resource", futureTime)
		remaining := err.GetRemainingLockTime()
		if remaining <= 0 {
			t.Errorf("GetRemainingLockTime() = %v, want positive duration", remaining)
		}
		// Should be approximately 30 minutes (allow 5 second margin for test execution)
		if remaining > 31*time.Minute || remaining < 29*time.Minute {
			t.Errorf("GetRemainingLockTime() = %v, want approximately 30 minutes", remaining)
		}
	})

	t.Run("past LockUntil returns zero", func(t *testing.T) {
		t.Parallel()

		pastTime := time.Now().Add(-10 * time.Minute)
		err := apperrors.NewLockConflictError("resource", pastTime)
		remaining := err.GetRemainingLockTime()
		if remaining != 0 {
			t.Errorf("GetRemainingLockTime() = %v, want 0 for past lock", remaining)
		}
	})

	t.Run("just-expired LockUntil returns zero", func(t *testing.T) {
		t.Parallel()

		pastTime := time.Now().Add(-1 * time.Millisecond)
		err := apperrors.NewLockConflictError("resource", pastTime)
		remaining := err.GetRemainingLockTime()
		if remaining != 0 {
			t.Errorf("GetRemainingLockTime() = %v, want 0 for just-expired lock", remaining)
		}
	})

	t.Run("very near future returns small positive", func(t *testing.T) {
		t.Parallel()

		nearFuture := time.Now().Add(5 * time.Second)
		err := apperrors.NewLockConflictError("resource", nearFuture)
		remaining := err.GetRemainingLockTime()
		if remaining <= 0 {
			t.Errorf("GetRemainingLockTime() = %v, want positive duration for near future", remaining)
		}
		if remaining > 6*time.Second {
			t.Errorf("GetRemainingLockTime() = %v, want less than 6s", remaining)
		}
	})
}
