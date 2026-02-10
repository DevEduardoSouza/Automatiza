package errors

import (
	"fmt"
	"time"
)

// ConflictError represents a resource conflict error (HTTP 409)
type ConflictError struct {
	Resource  string
	Reason    string
	LockUntil *time.Time
}

// NewConflictError creates a new ConflictError
func NewConflictError(resource, reason string) *ConflictError {
	return &ConflictError{
		Resource: resource,
		Reason:   reason,
	}
}

// NewLockConflictError creates a ConflictError for lock conflicts with time info
func NewLockConflictError(resource string, lockUntil time.Time) *ConflictError {
	return &ConflictError{
		Resource:  resource,
		Reason:    fmt.Sprintf("resource is locked until %s", lockUntil.Format(time.RFC3339)),
		LockUntil: &lockUntil,
	}
}

// Error implements the error interface
func (e *ConflictError) Error() string {
	return fmt.Sprintf("conflict: %s - %s", e.Resource, e.Reason)
}

// Unwrap returns nil (no wrapped error)
func (e *ConflictError) Unwrap() error {
	return nil
}

// IsConflictError checks if an error is a ConflictError
func IsConflictError(err error) bool {
	_, ok := err.(*ConflictError)
	return ok
}

// GetRemainingLockTime returns the remaining lock duration if applicable
func (e *ConflictError) GetRemainingLockTime() time.Duration {
	if e.LockUntil == nil {
		return 0
	}
	remaining := time.Until(*e.LockUntil)
	if remaining < 0 {
		return 0
	}
	return remaining
}
