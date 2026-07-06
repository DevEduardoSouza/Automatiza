// Package wamentions expands the "@todos" marker into a WhatsApp mention list.
//
// It is a domain-neutral helper shared by the jobs, monitoring and
// offer_generate services so none of them has to import another service
// package (which would risk an import cycle). It depends only on the WhatsApp
// port (whatsapp.WhatsAppProvider).
//
// WhatsApp has no native "@everyone": pinging every participant is done purely
// through the message's contextInfo.mentionedJid list. The visible "@todos"
// text is left untouched — callers keep it literal and rely on the returned
// mention list for the actual notification.
package wamentions

import (
	"context"
	"strings"

	"automatiza-go/internal/adapters/whatsapp"
)

// Marker is the literal text a user types to mention everyone in a group.
const Marker = "@todos"

// Contains reports whether the message opts into an @todos mention.
func Contains(message string) bool {
	return strings.Contains(message, Marker)
}

// Expand returns the list of participant IDs to mention when message contains
// "@todos", collecting every member of groupID via the sending session.
//
// Returns nil when the marker is absent, participant lookup fails, or the group
// has no members — callers then send without mentions (fail-safe no-op).
//
// Mention value resolution (verified live against WAHA GOWS 2026.03):
//   - PhoneNumber present  -> bare phone ("557499999999"); WAHA resolves it to
//     "<phone>@s.whatsapp.net" and the real participant is notified.
//   - unresolved LID        -> keep the full "<lid>@lid" JID; WAHA keeps it as a
//     LID mention, which is valid. Stripping the "@lid" suffix here would turn
//     the LID number into a bogus "<lid>@s.whatsapp.net" phone JID — a ghost
//     mention that pings nobody real. So we must NOT strip it.
//   - other IDs (e.g. "<phone>@c.us") -> strip the suffix to the bare phone.
func Expand(ctx context.Context, provider whatsapp.WhatsAppProvider, sessionName, groupID, message string) []string {
	if !Contains(message) {
		return nil
	}

	participants, err := provider.GetGroupParticipants(ctx, sessionName, groupID)
	if err != nil || len(participants) == 0 {
		return nil
	}

	mentions := make([]string, 0, len(participants))
	for _, p := range participants {
		id := mentionID(p)
		if id != "" {
			mentions = append(mentions, id)
		}
	}
	return mentions
}

// mentionID picks the correct mention value for a participant. See Expand's
// doc comment for the resolution rules and why the "@lid" suffix is preserved.
func mentionID(p whatsapp.ParticipantInfo) string {
	if p.PhoneNumber != "" {
		return p.PhoneNumber
	}
	if strings.HasSuffix(p.ID, "@lid") {
		return p.ID
	}
	return strings.Split(p.ID, "@")[0]
}
