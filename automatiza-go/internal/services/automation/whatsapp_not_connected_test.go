package automation

import (
	"errors"
	"fmt"
	"testing"
)

// TestWhatsAppNotConnectedClassification guards the "disconnected chip is not a
// dispatch error" behavior: getConnectedInstance wraps errWhatsAppNotConnected
// for the no-chip / no-connected-chip cases, and process_monitor logs those as
// 'skipped' (stand-by, off the panel error card) — but a real DB error must stay
// 'error'. If errors.Is stops matching through the wrap, disconnected chips would
// silently go back to counting as dispatch errors.
func TestWhatsAppNotConnectedClassification(t *testing.T) {
	// Exactly how getConnectedInstance wraps the two stand-by cases.
	noConnected := fmt.Errorf("nenhuma instancia WhatsApp conectada: %w", errWhatsAppNotConnected)
	noChip := fmt.Errorf("nenhuma instancia WhatsApp encontrada: %w", errWhatsAppNotConnected)
	for _, e := range []error{noConnected, noChip} {
		if !errors.Is(e, errWhatsAppNotConnected) {
			t.Fatalf("stand-by case %q must classify as not-connected (→ skipped)", e)
		}
	}

	// A real DB failure must NOT be swallowed as skipped — stays a dispatch error.
	dbErr := fmt.Errorf("failed to get instances: %w", errors.New("connection refused"))
	if errors.Is(dbErr, errWhatsAppNotConnected) {
		t.Fatal("a DB error must remain a real error, not be classified as not-connected")
	}
}
