package jobs

import (
	"context"
	"fmt"
	"strings"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/models"
	"automatiza-go/internal/services/mercadolivre"

	"go.uber.org/zap"
)

// ExecuteSendMLOffer executes the send_ml_offer action for all target groups.
// Creates an affiliate link and sends the product with optional image.
func ExecuteSendMLOffer(
	ctx context.Context,
	provider whatsapp.WhatsAppProvider,
	mlService mercadolivre.Service,
	sessionName string,
	userID string,
	job *models.StandaloneJob,
	logger *zap.Logger,
) []ActionResult {
	groups := job.Payload.Groups

	// Validate ML URL
	if err := ValidateMLURL(job.Payload.OriginURL); err != nil {
		results := make([]ActionResult, len(groups))
		for i, g := range groups {
			results[i] = ActionResult{GroupID: g, Success: false, Error: err.Error()}
		}
		return results
	}

	// Create affiliate link
	affiliateLink, err := mlService.CreateAffiliateLink(ctx, job.Payload.OriginURL, userID)
	if err != nil {
		logger.Warn("Failed to create affiliate link, using original URL",
			zap.String("originURL", job.Payload.OriginURL),
			zap.Error(err))
		// Fall back to original URL
		affiliateLink = &models.MLAffiliateLink{
			AffiliateLink: job.Payload.OriginURL,
		}
	}

	// Get product preview if we need the image
	var preview *models.MLProductPreview
	includeImage := job.Payload.IncludeImage != nil && *job.Payload.IncludeImage
	if includeImage {
		preview, err = mlService.GetProductPreview(ctx, job.Payload.OriginURL, userID)
		if err != nil {
			logger.Warn("Failed to get product preview",
				zap.String("originURL", job.Payload.OriginURL),
				zap.Error(err))
		}
	}

	// Build message
	message := buildMLOfferMessage(job.Payload.Text, affiliateLink.AffiliateLink, preview)

	return ExecuteWithRateLimit(ctx, groups, func(groupID string) error {
		// Send with image if available and requested
		if includeImage && preview != nil && preview.Image != "" {
			input := whatsapp.SendImageInput{
				SessionName: sessionName,
				ChatID:      groupID,
				Caption:     message,
				ImageURL:    preview.Image,
			}

			_, err := provider.SendImage(ctx, input)
			if err != nil {
				logger.Warn("Failed to send ML offer with image, trying text only",
					zap.String("groupID", groupID),
					zap.Error(err))
				// Fall back to text only
				return sendTextOnly(ctx, provider, sessionName, groupID, message)
			}
			return nil
		}

		// Text-only
		return sendTextOnly(ctx, provider, sessionName, groupID, message)
	})
}

func buildMLOfferMessage(customText, affiliateLink string, preview *models.MLProductPreview) string {
	var sb strings.Builder

	// Add custom text if provided
	if customText != "" {
		sb.WriteString(customText)
		sb.WriteString("\n\n")
	}

	// Add product info if available
	if preview != nil {
		if preview.Title != "" {
			sb.WriteString(preview.Title)
			sb.WriteString("\n")
		}
		if preview.Price > 0 {
			sb.WriteString(fmt.Sprintf("R$ %.2f", preview.Price))
			if preview.OriginalPrice != nil && *preview.OriginalPrice > preview.Price {
				sb.WriteString(fmt.Sprintf(" (era R$ %.2f)", *preview.OriginalPrice))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// Add affiliate link
	sb.WriteString(affiliateLink)

	return sb.String()
}

func sendTextOnly(ctx context.Context, provider whatsapp.WhatsAppProvider, sessionName, groupID, message string) error {
	input := whatsapp.SendTextInput{
		SessionName: sessionName,
		ChatID:      groupID,
		Text:        message,
	}
	_, err := provider.SendText(ctx, input)
	return err
}
