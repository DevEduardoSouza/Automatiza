package workers_test

import (
	"os"
	"testing"

	"automatiza-go/internal/config"
)

// TestMain disables anti-ban humanization for the workers test package so the
// per-recipient content-variation layer (which intentionally mutates outgoing
// text with spintax / zero-width chars / emoji) doesn't perturb the exact-content
// assertions in the worker tests. The variation itself is covered by its own unit
// tests in internal/lib/text, and it remains ON by default in production.
func TestMain(m *testing.M) {
	_ = os.Setenv("HUMANIZE_ENABLED", "false")
	config.ResetHumanization()
	os.Exit(m.Run())
}
