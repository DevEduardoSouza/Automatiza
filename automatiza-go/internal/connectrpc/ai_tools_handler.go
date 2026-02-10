package connectrpc

import (
	"context"
	"errors"
	"fmt"

	aiv1 "automatiza-go/internal/gen/ai/v1"
	"automatiza-go/internal/services/ai"

	"connectrpc.com/connect"
	"go.uber.org/zap"
)

// AiToolsHandler implements the aiv1connect.AiToolsServiceHandler interface.
// This handler provides AI-powered tools for marketing automation.
type AiToolsHandler struct {
	logger    *zap.Logger
	aiService ai.Service
}

// NewAiToolsHandler creates a new AI tools handler.
func NewAiToolsHandler(aiService ai.Service, logger *zap.Logger) *AiToolsHandler {
	return &AiToolsHandler{
		logger:    logger,
		aiService: aiService,
	}
}

// AnalyzeEngagement analyzes engagement metrics for a WhatsApp group.
// TODO: Integrate with AI service for real engagement analysis.
func (h *AiToolsHandler) AnalyzeEngagement(
	ctx context.Context,
	req *connect.Request[aiv1.AnalyzeEngagementRequest],
) (*connect.Response[aiv1.AnalyzeEngagementResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
	}

	groupID := req.Msg.GetGroupId()
	if groupID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("group_id is required"))
	}

	h.logger.Debug("AnalyzeEngagement called",
		zap.String("userID", user.ID),
		zap.String("groupID", groupID),
	)

	// TODO: Implement real engagement analysis using AI service
	// For now, return mock data
	bestTime := "19:00"
	summary := "Este grupo apresenta engajamento moderado. Considere postar mais conteudo durante o horario de pico."

	return connect.NewResponse(&aiv1.AnalyzeEngagementResponse{
		Metrics: &aiv1.EngagementMetrics{
			TotalMessages:  150,
			ActiveMembers:  45,
			EngagementRate: 35.5,
			BestTimeToPost: &bestTime,
		},
		Summary: &summary,
	}), nil
}

// AnalyzeTrends analyzes product trends on affiliate platforms.
// TODO: Integrate with AI service for real trend analysis.
func (h *AiToolsHandler) AnalyzeTrends(
	ctx context.Context,
	req *connect.Request[aiv1.AnalyzeTrendsRequest],
) (*connect.Response[aiv1.AnalyzeTrendsResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
	}

	platform := req.Msg.GetPlatform()
	category := req.Msg.GetCategory()

	if platform == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("platform is required"))
	}
	if category == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("category is required"))
	}

	h.logger.Debug("AnalyzeTrends called",
		zap.String("userID", user.ID),
		zap.String("platform", platform),
		zap.String("category", category),
	)

	// TODO: Implement real trend analysis using AI service and platform APIs
	// For now, return mock data
	desc1 := "Produto com alta demanda no momento"
	desc2 := "Tendencia em ascensao nas ultimas semanas"
	analysis := fmt.Sprintf("Analise de tendencias para %s na categoria %s: os produtos listados estao em alta demanda.", platform, category)

	return connect.NewResponse(&aiv1.AnalyzeTrendsResponse{
		Trends: []*aiv1.TrendItem{
			{
				Name:        "Fone Bluetooth",
				Score:       95.5,
				Description: &desc1,
			},
			{
				Name:        "Carregador Portatil",
				Score:       87.2,
				Description: &desc2,
			},
		},
		Analysis: &analysis,
	}), nil
}

// EnhanceMessage generates enhanced promotional messages using AI.
// TODO: Integrate with AI service (e.g., OpenAI) for real message enhancement.
func (h *AiToolsHandler) EnhanceMessage(
	ctx context.Context,
	req *connect.Request[aiv1.EnhanceMessageRequest],
) (*connect.Response[aiv1.EnhanceMessageResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
	}

	productLinks := req.Msg.GetProductLinks()
	if productLinks == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("product_links is required"))
	}

	copyStyle := req.Msg.GetCopyStyle()
	affiliateLink := req.Msg.GetAffiliateLink()

	h.logger.Debug("EnhanceMessage called",
		zap.String("userID", user.ID),
		zap.String("copyStyle", copyStyle),
		zap.Bool("hasAffiliateLink", affiliateLink != ""),
	)

	// TODO: Implement real message enhancement using AI service
	// For now, return mock enhanced messages
	linkSuffix := ""
	if affiliateLink != "" {
		linkSuffix = fmt.Sprintf("\n\nCompre aqui: %s", affiliateLink)
	}

	messages := []*aiv1.EnhancedMessage{
		{
			Message: fmt.Sprintf("Oferta imperdivel! Confira esse produto incrivel com desconto especial. Corra antes que acabe!%s", linkSuffix),
		},
		{
			Message: fmt.Sprintf("Voce nao vai acreditar nessa promocao! Produto de qualidade com preco baixissimo. Aproveite agora!%s", linkSuffix),
		},
		{
			Message: fmt.Sprintf("Atencao! Descobri esse produto fantastico e precisava compartilhar. Preco sensacional por tempo limitado!%s", linkSuffix),
		},
	}

	return connect.NewResponse(&aiv1.EnhanceMessageResponse{
		Messages: messages,
	}), nil
}

// SuggestOfferText suggests offer text based on category and store using AI.
func (h *AiToolsHandler) SuggestOfferText(
	ctx context.Context,
	req *connect.Request[aiv1.SuggestOfferTextRequest],
) (*connect.Response[aiv1.SuggestOfferTextResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
	}

	category := req.Msg.GetCategory()
	store := req.Msg.GetStore()
	mode := req.Msg.GetMode()

	h.logger.Debug("SuggestOfferText called",
		zap.String("userID", user.ID),
		zap.String("category", category),
		zap.String("store", store),
		zap.String("mode", mode),
	)

	texts, err := h.aiService.SuggestHeadlines(ctx, category, store, mode)
	if err != nil {
		h.logger.Error("Failed to suggest offer text", zap.Error(err))
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to generate suggestions: %w", err))
	}

	return connect.NewResponse(&aiv1.SuggestOfferTextResponse{
		Texts: texts,
	}), nil
}
