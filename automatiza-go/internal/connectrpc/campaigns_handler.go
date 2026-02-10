package connectrpc

import (
	"context"
	"errors"
	"fmt"

	campaignsv1 "automatiza-go/internal/gen/campaigns/v1"

	"connectrpc.com/connect"
	"go.uber.org/zap"
)

// CampaignsHandler implements the campaignsv1connect.CampaignsServiceHandler interface.
// This handler provides AI-powered campaign generation.
type CampaignsHandler struct {
	logger *zap.Logger
	// TODO: Add AI service dependency when implemented
	// aiService ai.Service
}

// NewCampaignsHandler creates a new campaigns handler.
func NewCampaignsHandler(logger *zap.Logger) *CampaignsHandler {
	return &CampaignsHandler{
		logger: logger,
	}
}

// CreateCampaign creates a new marketing campaign using AI.
// TODO: Integrate with AI service (e.g., OpenAI) for real campaign generation.
func (h *CampaignsHandler) CreateCampaign(
	ctx context.Context,
	req *connect.Request[campaignsv1.CreateCampaignRequest],
) (*connect.Response[campaignsv1.CreateCampaignResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
	}

	prompt := req.Msg.GetPrompt()
	if prompt == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("prompt is required"))
	}

	// Validate prompt length (as per proto validation: min 10, max 2000)
	if len(prompt) < 10 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("prompt must be at least 10 characters"))
	}
	if len(prompt) > 2000 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("prompt must not exceed 2000 characters"))
	}

	h.logger.Debug("CreateCampaign called",
		zap.String("userID", user.ID),
		zap.Int("promptLength", len(prompt)),
	)

	// TODO: Implement real campaign generation using AI service
	// This would typically:
	// 1. Send the prompt to an AI service (OpenAI, etc.)
	// 2. Parse the AI response into structured campaign data
	// 3. Generate multiple message variations
	// 4. Suggest optimal posting times based on engagement data
	// 5. Generate relevant hashtags

	// For now, return mock campaign data
	description := fmt.Sprintf("Campanha gerada com base no prompt: %s", truncateString(prompt, 50))
	targetAudience1 := "Mulheres 25-45 anos interessadas em ofertas"
	targetAudience2 := "Jovens 18-30 buscando promocoes"

	campaign := &campaignsv1.Campaign{
		Title:       "Campanha de Ofertas Especiais",
		Description: &description,
		Messages: []*campaignsv1.CampaignMessage{
			{
				Text:           "Ofertas imperdíveis chegaram! Confira os melhores descontos do dia. Corra antes que acabe!",
				Emoji:          strPtr("🔥"),
				TargetAudience: &targetAudience1,
			},
			{
				Text:           "Você não vai acreditar nesses preços! Economia garantida em produtos selecionados.",
				Emoji:          strPtr("💰"),
				TargetAudience: &targetAudience2,
			},
			{
				Text:           "Atenção! Promoção relâmpago por tempo limitado. Aproveite agora!",
				Emoji:          strPtr("⚡"),
				TargetAudience: nil,
			},
		},
		Hashtags:       []string{"#promocao", "#ofertas", "#desconto", "#economia", "#compras"},
		SuggestedTimes: []string{"09:00", "12:00", "18:00", "21:00"},
	}

	return connect.NewResponse(&campaignsv1.CreateCampaignResponse{
		Success:  true,
		Campaign: campaign,
	}), nil
}

// truncateString truncates a string to the specified length and adds ellipsis if needed.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// Note: strPtr is defined in profiles_handler.go and shared across the package
