package jobs

import (
	"context"
	"strings"

	"automatiza-go/internal/adapters/whatsapp"
)

// ExpandMentions expands @todos in a message to mention all group participants.
// Returns the list of JIDs to mention and the expanded message.
// If @todos is not present or fails to expand, returns nil and original message.
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

	// Build mention list (extract phone from JID)
	mentions := make([]string, 0, len(participants))
	for _, p := range participants {
		// JID format: "5511999999999@c.us" or "5511999999999@s.whatsapp.net"
		phone := strings.Split(p.ID, "@")[0]
		mentions = append(mentions, phone)
	}

	// Build @mention list for message
	mentionTags := make([]string, 0, len(mentions))
	for _, phone := range mentions {
		mentionTags = append(mentionTags, "@"+phone)
	}

	// Replace @todos with all mentions
	expandedMessage := strings.Replace(message, "@todos", strings.Join(mentionTags, " "), 1)

	return mentions, expandedMessage
}

// ContainsMentions checks if the message contains @todos marker
func ContainsMentions(message string) bool {
	return strings.Contains(message, "@todos")
}
