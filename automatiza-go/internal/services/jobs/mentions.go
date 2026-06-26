package jobs

import (
	"context"
	"strings"

	"automatiza-go/internal/adapters/whatsapp"
)

// ExpandMentions handles the @todos marker by collecting every group
// participant into the mentionedJid list while leaving the visible text
// untouched. Returns the list of IDs to mention and the (unchanged) message.
// If @todos is not present or fails to expand, returns nil and original message.
//
// WhatsApp has no native "@todos"/"@everyone": mentioning all participants is
// done purely through the message's mentionedJid list. Previously this function
// replaced "@todos" with "@<num1> @<num2> ..." in the body, which dumped every
// participant number (often unresolved @lid garbage like "@+160168507666673")
// one-by-one into the message the client sees. We now keep the literal "@todos"
// text and rely on the mentions array for the ping — everyone still gets the
// notification, but the message stays clean.
func ExpandMentions(ctx context.Context, provider whatsapp.WhatsAppProvider, sessionName, groupID, message string) ([]string, string) {
	if !strings.Contains(message, "@todos") {
		return nil, message
	}

	// Get group participants
	participants, err := provider.GetGroupParticipants(ctx, sessionName, groupID)
	if err != nil {
		// Fail silently - return original message without mentions
		return nil, message
	}

	if len(participants) == 0 {
		return nil, message
	}

	// Build mention list (extract phone/LID from JID). These feed WAHA's
	// mentions field (-> contextInfo.mentionedJid), which is what actually
	// notifies the participants regardless of what the visible text says.
	mentions := make([]string, 0, len(participants))
	for _, p := range participants {
		// JID format: "5511999999999@c.us", "5511999999999@s.whatsapp.net"
		// or "<lid>@lid" for unresolved participants.
		id := strings.Split(p.ID, "@")[0]
		mentions = append(mentions, id)
	}

	// Keep the literal "@todos" text — do not expand the participant numbers
	// into the body. The mentions array above silently pings everyone.
	return mentions, message
}

// ContainsMentions checks if the message contains @todos marker
func ContainsMentions(message string) bool {
	return strings.Contains(message, "@todos")
}
