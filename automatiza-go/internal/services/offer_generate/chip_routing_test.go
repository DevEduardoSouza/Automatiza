package offer_generate

import (
	"fmt"
	"testing"
)

func TestPickChipForGroup_NoPreference_ReturnsSenderSession(t *testing.T) {
	got := PickChipForGroup(ChipRoutingInput{
		WaGroupID:     "120363a@g.us",
		PreferenceIDs: nil,
		ConnectedByID: map[string]string{"100": "session_A"},
		SenderSession: "session_default",
	})
	if got != "session_default" {
		t.Fatalf("expected senderSession fallback, got %q", got)
	}
}

func TestPickChipForGroup_SingleChip_PinsToThatChip(t *testing.T) {
	// Single-chip automations must behave bit-for-bit like pre-050: the
	// one preferred chip wins, regardless of override or participation.
	got := PickChipForGroup(ChipRoutingInput{
		WaGroupID:         "120363a@g.us",
		PreferenceIDs:     []string{"100"},
		GroupParticipants: []string{"200"},                    // chip 100 not in group
		ConnectedByID:     map[string]string{"100": "session_A"},
		ChipOverrides:     map[string]string{"120363a@g.us": "200"}, // ignored
		SenderSession:     "session_default",
	})
	if got != "session_A" {
		t.Fatalf("single-chip should always pick chip 100, got %q", got)
	}
}

func TestPickChipForGroup_OverrideWins(t *testing.T) {
	got := PickChipForGroup(ChipRoutingInput{
		WaGroupID:         "120363a@g.us",
		PreferenceIDs:     []string{"100", "200"},
		GroupParticipants: []string{"100", "200"},
		ConnectedByID:     map[string]string{"100": "session_A", "200": "session_B"},
		ChipOverrides:     map[string]string{"120363a@g.us": "200"},
	})
	if got != "session_B" {
		t.Fatalf("override should pin to chip 200/session_B, got %q", got)
	}
}

func TestPickChipForGroup_StaleOverride_FallsThroughToHash(t *testing.T) {
	// User pinned chip 999 but it's been removed from PreferenceIDs.
	// Algorithm must fall through to sticky hash, never return "" or
	// silently drop the dispatch.
	got := PickChipForGroup(ChipRoutingInput{
		WaGroupID:         "120363a@g.us",
		PreferenceIDs:     []string{"100", "200"},
		GroupParticipants: []string{"100", "200"},
		ConnectedByID:     map[string]string{"100": "session_A", "200": "session_B"},
		ChipOverrides:     map[string]string{"120363a@g.us": "999"},
	})
	if got != "session_A" && got != "session_B" {
		t.Fatalf("stale override should fall through to hash; got %q", got)
	}
}

func TestPickChipForGroup_DisconnectedOverride_FallsThroughToHash(t *testing.T) {
	// Override points to a chip in preference but the chip is offline.
	// Hash takes over with only the connected chip eligible.
	got := PickChipForGroup(ChipRoutingInput{
		WaGroupID:         "120363a@g.us",
		PreferenceIDs:     []string{"100", "200"},
		GroupParticipants: []string{"100", "200"},
		ConnectedByID:     map[string]string{"100": "session_A"}, // 200 offline
		ChipOverrides:     map[string]string{"120363a@g.us": "200"},
	})
	// Only chip 100 connected — sticky hash always lands there.
	if got != "session_A" {
		t.Fatalf("disconnected override should fall through to connected chip, got %q", got)
	}
}

func TestPickChipForGroup_StickyHash_DeterministicAcrossCalls(t *testing.T) {
	in := ChipRoutingInput{
		WaGroupID:         "120363shared@g.us",
		PreferenceIDs:     []string{"100", "200"},
		GroupParticipants: []string{"100", "200"},
		ConnectedByID:     map[string]string{"100": "session_A", "200": "session_B"},
	}
	first := PickChipForGroup(in)
	for i := 0; i < 100; i++ {
		got := PickChipForGroup(in)
		if got != first {
			t.Fatalf("sticky hash must be deterministic across calls, got %q then %q on iter %d", first, got, i)
		}
	}
}

func TestPickChipForGroup_HashSpreadsAcrossChips(t *testing.T) {
	// Distribute 50 different wa_group_ids across 2 chips. With FNV-1a
	// the split should be reasonably balanced — definitely not 50/0.
	prefs := []string{"100", "200"}
	connected := map[string]string{"100": "session_A", "200": "session_B"}
	participants := []string{"100", "200"}
	counts := map[string]int{"session_A": 0, "session_B": 0}
	for i := 0; i < 50; i++ {
		got := PickChipForGroup(ChipRoutingInput{
			WaGroupID:         fmt.Sprintf("120363grp%d@g.us", i),
			PreferenceIDs:     prefs,
			GroupParticipants: participants,
			ConnectedByID:     connected,
		})
		counts[got]++
	}
	// Allow generous slack — uniform expectation is 25/25; reject only
	// degenerate distributions like 50/0 or 49/1 that suggest a bug.
	if counts["session_A"] < 10 || counts["session_B"] < 10 {
		t.Fatalf("expected reasonably balanced split, got A=%d B=%d", counts["session_A"], counts["session_B"])
	}
}

func TestPickChipForGroup_ExclusiveGroup_PicksOnlyEligibleChip(t *testing.T) {
	// chip 200 is in the preference but isn't a participant of this
	// group. The hash must filter it out and always pick chip 100.
	got := PickChipForGroup(ChipRoutingInput{
		WaGroupID:         "120363only100@g.us",
		PreferenceIDs:     []string{"100", "200"},
		GroupParticipants: []string{"100"},
		ConnectedByID:     map[string]string{"100": "session_A", "200": "session_B"},
	})
	if got != "session_A" {
		t.Fatalf("only chip 100 participates, expected session_A; got %q", got)
	}
}

func TestPickChipForGroup_NoEligibleChip_ReturnsEmpty(t *testing.T) {
	// Group's participants don't intersect PreferenceIDs at all → skip.
	got := PickChipForGroup(ChipRoutingInput{
		WaGroupID:         "120363x@g.us",
		PreferenceIDs:     []string{"100", "200"},
		GroupParticipants: []string{"999"},
		ConnectedByID:     map[string]string{"100": "session_A", "200": "session_B"},
	})
	if got != "" {
		t.Fatalf("no eligible chip should return empty string for skip; got %q", got)
	}
}

func TestPickChipForGroup_NoParticipationData_UsesAllPreferences(t *testing.T) {
	// Brand-new group not yet seen by any sync. Algorithm must still
	// pick a chip (best-effort) so the dispatch happens — WAHA fails
	// loud if the chip really isn't a member.
	got := PickChipForGroup(ChipRoutingInput{
		WaGroupID:         "120363new@g.us",
		PreferenceIDs:     []string{"100", "200"},
		GroupParticipants: nil,
		ConnectedByID:     map[string]string{"100": "session_A", "200": "session_B"},
	})
	if got != "session_A" && got != "session_B" {
		t.Fatalf("expected a best-effort pick for unsynced group, got %q", got)
	}
}
