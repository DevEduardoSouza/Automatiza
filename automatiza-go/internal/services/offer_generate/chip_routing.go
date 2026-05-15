package offer_generate

import "hash/fnv"

// ChipRoutingInput collects everything needed to decide which chip session
// should dispatch a single destination group. Pulled into its own struct
// so the routing logic can be unit-tested without spinning up the full
// offer_generate service. See PickChipForGroup for the algorithm.
type ChipRoutingInput struct {
	// WaGroupID is the WhatsApp group id (e.g. "120363@g.us") that's the
	// dispatch target.
	WaGroupID string
	// PreferenceIDs is the ordered list of chip ids enabled on the
	// automation. Each entry is the instances.id as string; matches the
	// wire format used by the multi-chip preference (migration 049).
	PreferenceIDs []string
	// GroupParticipants lists every chip id that has the group in its
	// participation set (groups.participating_instance_ids). Empty when
	// the group has no recorded participation yet — typically a chip that
	// just synced for the first time.
	GroupParticipants []string
	// ConnectedByID maps chip id → session name for chips that are
	// currently connected. Chips absent here are considered offline and
	// won't be picked (overrides included).
	ConnectedByID map[string]string
	// ChipOverrides is the user-pinned routing map (migration 050).
	// Keys are wa_group_id, values are chip ids. Honored only when the
	// pinned chip is in PreferenceIDs *and* in ConnectedByID — a stale
	// override silently falls through to sticky hash.
	ChipOverrides map[string]string
	// SenderSession is the fallback session used when there's no
	// preference at all (len(PreferenceIDs) == 0). Picked by the caller
	// from "first connected chip" or similar; opaque to the routing
	// algorithm.
	SenderSession string
}

// PickChipForGroup decides which chip session should dispatch a given
// destination group. The algorithm:
//
//  1. No preference set → return SenderSession (pre-049 single-chip path).
//
//  2. Single chip in PreferenceIDs → return that chip's session. Single-
//     chip automations skip every multi-chip code path so their behaviour
//     is bit-for-bit unchanged after migration 050.
//
//  3. Multi-chip + ChipOverrides[WaGroupID] points at a chip that's still
//     enabled AND connected → use it. Stale overrides (chip removed from
//     preference, chip disconnected) fall through to step 4 silently —
//     dropping the dispatch over a stale config would surprise the user.
//
//  4. Sticky hash across the intersection of (PreferenceIDs ∩ participants).
//     FNV-1a of WaGroupID modulo the eligible-chip count picks a stable
//     chip: same group always lands on the same chip across runs, which
//     is what WhatsApp's anti-ban heuristics expect (consistent sender per
//     group). When the group has no participation data yet, the algorithm
//     hashes over all PreferenceIDs and lets WAHA fail loudly if the
//     picked chip really isn't in the group — same best-effort behaviour
//     as pre-050.
//
//  5. No eligible chip → return "". The caller treats this as "skip this
//     group" and logs a `no_chip_in_group` dispatch event.
func PickChipForGroup(in ChipRoutingInput) string {
	// 1 & 2: single-chip / no-preference path. Behaviour is identical to
	// pre-050 — multi-chip routing only kicks in when 2+ chips are set.
	if len(in.PreferenceIDs) == 0 {
		return in.SenderSession
	}
	if len(in.PreferenceIDs) == 1 {
		return in.ConnectedByID[in.PreferenceIDs[0]]
	}

	// 3: explicit override wins when valid.
	if pinned, ok := in.ChipOverrides[in.WaGroupID]; ok && pinned != "" {
		for _, prefID := range in.PreferenceIDs {
			if prefID == pinned {
				if session, connected := in.ConnectedByID[pinned]; connected {
					return session
				}
				break
			}
		}
	}

	// 4: sticky hash across eligible chips.
	var eligible []string
	if len(in.GroupParticipants) == 0 {
		eligible = in.PreferenceIDs
	} else {
		participantSet := make(map[string]struct{}, len(in.GroupParticipants))
		for _, p := range in.GroupParticipants {
			participantSet[p] = struct{}{}
		}
		eligible = make([]string, 0, len(in.PreferenceIDs))
		for _, prefID := range in.PreferenceIDs {
			if _, ok := participantSet[prefID]; ok {
				eligible = append(eligible, prefID)
			}
		}
	}
	if len(eligible) == 0 {
		return ""
	}

	h := fnv.New32a()
	_, _ = h.Write([]byte(in.WaGroupID))
	idx := h.Sum32() % uint32(len(eligible))
	return in.ConnectedByID[eligible[idx]]
}
