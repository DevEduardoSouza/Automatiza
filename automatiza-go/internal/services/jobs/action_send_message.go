package jobs

import (
	"context"
	"strings"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/models"

	"go.uber.org/zap"
)

// ExecuteSendMessage executes the send_message action for all target groups.
// Supports text messages with optional media (image/video) and @todos expansion.
func ExecuteSendMessage(
	ctx context.Context,
	provider whatsapp.WhatsAppProvider,
	sessionName string,
	job *models.StandaloneJob,
	logger *zap.Logger,
) []ActionResult {
	groups := job.Payload.Groups

	return ExecuteWithRateLimit(ctx, groups, func(groupID string) error {
		message := job.Payload.Message

		// Expand @todos to mention all participants
		var mentions []string
		if ContainsMentions(message) {
			mentions, message = ExpandMentions(ctx, provider, sessionName, groupID, message)
		}

		// Determine message type based on payload
		if job.Payload.Media != "" {
			// Has media attachment
			return sendMessageWithMedia(ctx, provider, sessionName, groupID, message, mentions, &job.Payload, logger)
		}

		// Text-only message
		input := whatsapp.SendTextInput{
			SessionName: sessionName,
			ChatID:      groupID,
			Text:        message,
			Mentions:    mentions,
		}

		_, err := provider.SendText(ctx, input)
		if err != nil {
			logger.Warn("Failed to send text message",
				zap.String("groupID", groupID),
				zap.Error(err))
			return err
		}

		return nil
	})
}

func sendMessageWithMedia(
	ctx context.Context,
	provider whatsapp.WhatsAppProvider,
	sessionName, groupID, caption string,
	mentions []string,
	payload *models.JobPayload,
	logger *zap.Logger,
) error {
	mediaType := strings.ToLower(payload.MediaType)

	switch mediaType {
	case "image", "":
		// Default to image if not specified
		input := whatsapp.SendImageInput{
			SessionName: sessionName,
			ChatID:      groupID,
			Caption:     caption,
			Filename:    payload.FileName,
			Mimetype:    payload.MimeType,
		}

		// Determine if media is URL or base64
		if strings.HasPrefix(payload.Media, "http") {
			input.ImageURL = payload.Media
		} else {
			input.ImageBase64 = payload.Media
		}

		_, err := provider.SendImage(ctx, input)
		return err

	case "video":
		// Videos need special handling with convert flag
		input := whatsapp.SendVoiceInput{
			SessionName: sessionName,
			ChatID:      groupID,
			Mimetype:    payload.MimeType,
		}

		if strings.HasPrefix(payload.Media, "http") {
			// For videos from URL, we need to use a different approach
			// The WAHA API handles this differently
			logger.Warn("Video URL not fully supported, attempting base64")
		}
		input.AudioBase64 = payload.Media

		_, err := provider.SendVoice(ctx, input)
		return err

	default:
		logger.Warn("Unknown media type, defaulting to image",
			zap.String("mediaType", mediaType))
		// Fall back to image
		input := whatsapp.SendImageInput{
			SessionName: sessionName,
			ChatID:      groupID,
			Caption:     caption,
			Filename:    payload.FileName,
			Mimetype:    payload.MimeType,
		}
		if strings.HasPrefix(payload.Media, "http") {
			input.ImageURL = payload.Media
		} else {
			input.ImageBase64 = payload.Media
		}
		_, err := provider.SendImage(ctx, input)
		return err
	}
}
