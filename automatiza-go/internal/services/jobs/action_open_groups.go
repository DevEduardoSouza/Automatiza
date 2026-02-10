package jobs

import (
	"context"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/models"

	"go.uber.org/zap"
)

// ExecuteOpenGroups executes the open_groups action for all target groups.
// Disables announcement mode (everyone can send messages).
func ExecuteOpenGroups(
	ctx context.Context,
	provider whatsapp.WhatsAppProvider,
	sessionName string,
	job *models.StandaloneJob,
	logger *zap.Logger,
) []ActionResult {
	groups := job.Payload.Groups

	return ExecuteWithRateLimit(ctx, groups, func(groupID string) error {
		input := whatsapp.SetGroupAnnouncementInput{
			SessionName: sessionName,
			GroupID:     groupID,
			Announce:    false, // Open = announcement mode OFF
		}

		err := provider.SetGroupAnnouncement(ctx, input)
		if err != nil {
			logger.Warn("Failed to open group",
				zap.String("groupID", groupID),
				zap.Error(err))
			return err
		}

		return nil
	})
}
