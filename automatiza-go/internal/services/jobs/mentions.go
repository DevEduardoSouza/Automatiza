package jobs

import (
	"context"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/lib/wamentions"
)

// ExpandMentions handles the @todos marker by collecting every group
// participant into the mentionedJid list while leaving the visible text
// untouched. Returns the list of IDs to mention and the (unchanged) message.
// If @todos is not present or fails to expand, returns nil and original message.
//
// This is a thin wrapper over wamentions.Expand — the shared, domain-neutral
// implementation also used by the monitoring and offer_generate relays so the
// @todos behaviour stays identical across features.
func ExpandMentions(ctx context.Context, provider whatsapp.WhatsAppProvider, sessionName, groupID, message string) ([]string, string) {
	return wamentions.Expand(ctx, provider, sessionName, groupID, message), message
}

// ContainsMentions checks if the message contains the @todos marker.
func ContainsMentions(message string) bool {
	return wamentions.Contains(message)
}
