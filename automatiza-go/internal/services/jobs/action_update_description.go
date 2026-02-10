package jobs

import (
	"context"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/models"

	"go.uber.org/zap"
)

// ExecuteUpdateDescription executes the update_description action for all target groups.
// Requires admin privileges in each group.
func ExecuteUpdateDescription(
	ctx context.Context,
	provider whatsapp.WhatsAppProvider,
	sessionName string,
	job *models.StandaloneJob,
	logger *zap.Logger,
) []ActionResult {
	groups := job.Payload.Groups
	description := job.Payload.Description

	return ExecuteWithRateLimit(ctx, groups, func(groupID string) error {
		input := whatsapp.UpdateGroupDescriptionInput{
			SessionName: sessionName,
			GroupID:     groupID,
			Description: description,
		}

		err := provider.UpdateGroupDescription(ctx, input)
		if err != nil {
			logger.Warn("Failed to update group description",
				zap.String("groupID", groupID),
				zap.Error(err))
			return err
		}

		return nil
	})
}
