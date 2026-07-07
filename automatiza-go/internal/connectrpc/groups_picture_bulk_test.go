package connectrpc

import "testing"

func TestPickAdminChipForGroup(t *testing.T) {
	connected := map[string]string{
		"10": "session_a",
		"20": "session_b",
		"30": "session_c",
	}

	t.Run("no admin ids returns empty", func(t *testing.T) {
		if got := pickAdminChipForGroup("123@g.us", nil, connected); got != "" {
			t.Fatalf("expected empty, got %q", got)
		}
	})

	t.Run("admin chip not connected returns empty", func(t *testing.T) {
		// chip 99 administers the group but isn't in the connected map.
		if got := pickAdminChipForGroup("123@g.us", []string{"99"}, connected); got != "" {
			t.Fatalf("expected empty (admin offline), got %q", got)
		}
	})

	t.Run("single connected admin chip is used", func(t *testing.T) {
		if got := pickAdminChipForGroup("123@g.us", []string{"20"}, connected); got != "session_b" {
			t.Fatalf("expected session_b, got %q", got)
		}
	})

	t.Run("only connected admin chips are eligible", func(t *testing.T) {
		// 99 offline, 30 connected -> must pick 30's session.
		if got := pickAdminChipForGroup("123@g.us", []string{"99", "30"}, connected); got != "session_c" {
			t.Fatalf("expected session_c, got %q", got)
		}
	})

	t.Run("sticky: same group always maps to the same chip", func(t *testing.T) {
		admins := []string{"10", "20", "30"}
		first := pickAdminChipForGroup("456@g.us", admins, connected)
		if first == "" {
			t.Fatal("expected a chip, got empty")
		}
		for i := 0; i < 20; i++ {
			// Order of admins must not affect the pick.
			if got := pickAdminChipForGroup("456@g.us", []string{"30", "10", "20"}, connected); got != first {
				t.Fatalf("unstable pick: %q vs %q", got, first)
			}
		}
	})
}
