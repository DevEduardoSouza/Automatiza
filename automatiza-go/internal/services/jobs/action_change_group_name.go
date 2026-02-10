package jobs

import (
	"context"
	"fmt"
	"time"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/models"

	"go.uber.org/zap"
)

// ExecuteChangeGroupName executes the change_group_name action for all target groups.
// Supports auto-numbering with #N prefix.
func ExecuteChangeGroupName(
	ctx context.Context,
	provider whatsapp.WhatsAppProvider,
	sessionName string,
	job *models.StandaloneJob,
	logger *zap.Logger,
) []ActionResult {
	groups := job.Payload.Groups
	baseName := job.Payload.Name
	autoNumber := job.Payload.AutoNumber

	results := make([]ActionResult, 0, len(groups))

	for i, groupID := range groups {
		// Add delay between groups (not before first)
		if i > 0 {
			select {
			case <-ctx.Done():
				for _, g := range groups[i:] {
					results = append(results, ActionResult{GroupID: g, Success: false, Error: "canceled"})
				}
				return results
			case <-time.After(RateLimitDelay):
			}
		}

		// Build the name (with or without number)
		name := baseName
		if autoNumber {
			name = fmt.Sprintf("#%d %s", i+1, baseName)
		}

		input := whatsapp.UpdateGroupSubjectInput{
			SessionName: sessionName,
			GroupID:     groupID,
			Subject:     name,
		}

		err := provider.UpdateGroupSubject(ctx, input)
		if err != nil {
			logger.Warn("Failed to change group name",
				zap.String("groupID", groupID),
				zap.String("name", name),
				zap.Error(err))
			results = append(results, ActionResult{GroupID: groupID, Success: false, Error: err.Error()})
		} else {
			results = append(results, ActionResult{GroupID: groupID, Success: true})
		}
	}

	return results
}
