package jobs

import (
	"context"
	"strings"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/models"

	"go.uber.org/zap"
)

// ExecuteChangeGroupPhoto executes the change_group_photo action for all target groups.
// Image can be base64 or URL.
func ExecuteChangeGroupPhoto(
	ctx context.Context,
	provider whatsapp.WhatsAppProvider,
	sessionName string,
	job *models.StandaloneJob,
	logger *zap.Logger,
) []ActionResult {
	groups := job.Payload.Groups
	image := job.Payload.Image

	if image == "" {
		// Return error for all groups if no image
		results := make([]ActionResult, len(groups))
		for i, g := range groups {
			results[i] = ActionResult{GroupID: g, Success: false, Error: "imagem e obrigatoria"}
		}
		return results
	}

	return ExecuteWithRateLimit(ctx, groups, func(groupID string) error {
		input := whatsapp.UpdateGroupPictureInput{
			SessionName: sessionName,
			GroupID:     groupID,
		}

		// Determine if image is base64 or needs to be treated as base64
		// The WAHA API expects base64 for group pictures
		if strings.HasPrefix(image, "http") {
			// For URLs, we would need to download first
			// For now, log warning and try anyway
			logger.Warn("Image URL provided for group photo, API expects base64",
				zap.String("groupID", groupID))
		}
		input.ImageBase64 = image

		err := provider.UpdateGroupPicture(ctx, input)
		if err != nil {
			logger.Warn("Failed to change group photo",
				zap.String("groupID", groupID),
				zap.Error(err))
			return err
		}

		return nil
	})
}
