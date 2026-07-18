package reconciliation

import (
	"errors"
	"fmt"
	"testing"

	whatsapp "automatiza-go/internal/adapters/whatsapp"
)

// TestSessionNotFoundDetectedThroughWrap guards the reconciliation-starvation
// fix: ReconcileChat breaks its retry loop early when the session doesn't exist,
// but only if it can see ErrCodeSessionNotFound THROUGH the wrap that doReconcile
// adds (`fmt.Errorf("failed to fetch messages: %w", err)`). If the unwrap breaks,
// dead-session reconciles silently go back to burning ~90s of RetryBackoff each,
// re-starving the worker pool and dropping mirror events.
func TestSessionNotFoundDetectedThroughWrap(t *testing.T) {
	// Exactly how doReconcile surfaces a GetMessages failure against a dead session.
	wrapped := fmt.Errorf("failed to fetch messages: %w",
		whatsapp.NewProviderError(whatsapp.ErrCodeSessionNotFound, "resource not found", nil))

	var provErr *whatsapp.ProviderError
	if !errors.As(wrapped, &provErr) || provErr.Code != whatsapp.ErrCodeSessionNotFound {
		t.Fatal("session-not-found must be detectable through the doReconcile %w wrap to skip the retry backoff")
	}

	// A transient class (timeout) must NOT trip the early break — those still
	// deserve the retry loop.
	other := fmt.Errorf("failed to fetch messages: %w",
		whatsapp.NewProviderError(whatsapp.ErrCodeTimeout, "timeout", nil))
	var p2 *whatsapp.ProviderError
	if errors.As(other, &p2) && p2.Code == whatsapp.ErrCodeSessionNotFound {
		t.Fatal("a timeout error must not be classified as session-not-found")
	}
}
