package jobs

import (
	"context"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/models"

	"go.uber.org/zap"
)

// ExecuteSendPoll executes the send_poll action for all target groups.
// Creates a poll with 2-12 options in each group.
func ExecuteSendPoll(
	ctx context.Context,
	provider whatsapp.WhatsAppProvider,
	sessionName string,
	job *models.StandaloneJob,
	logger *zap.Logger,
) []ActionResult {
	groups := job.Payload.Groups

	// Validate poll options
	if err := ValidatePollOptions(job.Payload.Options); err != nil {
		// Return error for all groups
		results := make([]ActionResult, len(groups))
		for i, g := range groups {
			results[i] = ActionResult{GroupID: g, Success: false, Error: err.Error()}
		}
		return results
	}

	// Determine selectable count
	selectableCount := job.Payload.SelectableCount
	if selectableCount == 0 {
		selectableCount = 1 // Default to single selection
	}

	return ExecuteWithRateLimit(ctx, groups, func(groupID string) error {
		input := whatsapp.SendPollInput{
			SessionName:     sessionName,
			ChatID:          groupID,
			Question:        job.Payload.Question,
			Options:         job.Payload.Options,
			SelectableCount: selectableCount,
		}

		_, err := provider.SendPoll(ctx, input)
		if err != nil {
			logger.Warn("Failed to send poll",
				zap.String("groupID", groupID),
				zap.Error(err))
			return err
		}

		return nil
	})
}
