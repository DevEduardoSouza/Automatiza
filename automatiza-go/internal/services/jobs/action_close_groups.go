package jobs

import (
	"context"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/models"

	"go.uber.org/zap"
)

// ExecuteCloseGroups executes the close_groups action for all target groups.
// Sets groups to announcement mode (admins only can send messages).
func ExecuteCloseGroups(
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
			Announce:    true, // Close = announcement mode ON
		}

		err := provider.SetGroupAnnouncement(ctx, input)
		if err != nil {
			logger.Warn("Failed to close group",
				zap.String("groupID", groupID),
				zap.Error(err))
			return err
		}

		return nil
	})
}
