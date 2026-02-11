package ai

import (
	"automatiza-go/internal/lib/cache"
	"automatiza-go/internal/models"
	"automatiza-go/internal/observability"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
	"go.uber.org/zap"
)

// Service defines the AI service interface
type Service interface {
	GenerateCampaignStrategy(ctx context.Context, params *models.CampaignStrategyRequest) (*models.CampaignStrategy, error)
	EnhanceMessage(ctx context.Context, message, context string) (string, error)
	SuggestOfferText(ctx context.Context, offer *models.ShopeeOffer) (string, error)
	SuggestHeadlines(ctx context.Context, category, store, mode string) ([]string, error)
	AnalyzeEngagement(ctx context.Context, groupID string, messages []string) (*models.EngagementAnalysis, error)
	AnalyzeTrends(ctx context.Context, category string) (*models.TrendsAnalysis, error)
	ClearClientCache()
}

type aiService struct {
	httpClient *http.Client
	apiURL     string
	apiKey     string
	model      string
	cache      *cache.Cache
	logger     *zap.Logger
}

// NewAIService creates a new AI service
func NewAIService(
	apiURL, apiKey, model string,
	cache *cache.Cache,
	logger *zap.Logger,
) Service {
	if model == "" {
		model = "gpt-3.5-turbo"
	}

	return &aiService{
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		apiURL: apiURL,
		apiKey: apiKey,
		model:  model,
		cache:  cache,
		logger: logger,
	}
}

func (s *aiService) GenerateCampaignStrategy(ctx context.Context, params *models.CampaignStrategyRequest) (*models.CampaignStrategy, error) {
	if s.apiURL == "" || s.apiKey == "" {
		return nil, fmt.Errorf("AI service not configured")
	}

	prompt := fmt.Sprintf(`Crie uma estratégia de campanha de marketing para WhatsApp com os seguintes parâmetros:
- Objetivo: %s
- Público-alvo: %s
- Produtos/Categoria: %s
- Tom de voz: %s
- Orçamento: %s

Retorne em JSON com os campos:
{
  "title": "título da campanha",
  "description": "descrição detalhada",
  "targetAudience": "público-alvo refinado",
  "messageTemplates": ["template1", "template2", "template3"],
  "schedule": {
    "frequency": "diária/semanal",
    "bestTimes": ["09:00", "14:00", "19:00"],
    "duration": "7 dias"
  },
  "kpis": ["KPI1", "KPI2"]
}`,
		params.Objective,
		params.TargetAudience,
		params.Category,
		params.Tone,
		params.Budget,
	)

	response, err := s.callAI(ctx, prompt, "Você é um especialista em marketing digital e automação de WhatsApp.")
	if err != nil {
		return nil, err
	}

	var strategy models.CampaignStrategy
	if err := json.Unmarshal([]byte(response), &strategy); err != nil {
		// If JSON parsing fails, create a basic response
		strategy = models.CampaignStrategy{
			Title:       "Campanha Personalizada",
			Description: response,
		}
	}

	return &strategy, nil
}

func (s *aiService) EnhanceMessage(ctx context.Context, message, contextInfo string) (string, error) {
	if s.apiURL == "" || s.apiKey == "" {
		return message, nil // Return original if not configured
	}

	prompt := fmt.Sprintf(`Melhore esta mensagem de marketing para WhatsApp, tornando-a mais atrativa e persuasiva:

Mensagem original:
%s

Contexto adicional:
%s

Regras:
- Mantenha curta (máximo 300 caracteres)
- Use emojis moderadamente
- Inclua call-to-action claro
- Mantenha tom professional mas amigável

Retorne apenas a mensagem melhorada, sem explicações.`, message, contextInfo)

	return s.callAI(ctx, prompt, "Você é um copywriter especialista em mensagens de WhatsApp para e-commerce.")
}

func (s *aiService) SuggestOfferText(ctx context.Context, offer *models.ShopeeOffer) (string, error) {
	if s.apiURL == "" || s.apiKey == "" {
		// Return default template if not configured
		return fmt.Sprintf("🔥 %s\n\n💰 Por apenas R$ %.2f\n\n🛒 Compre agora: %s",
			offer.Name, offer.Price, offer.AffiliateLink), nil
	}

	prompt := fmt.Sprintf(`Crie uma mensagem promotional para WhatsApp para este produto:

Nome: %s
Preço: R$ %.2f
Preço Original: R$ %.2f
Desconto: %.0f%%
Loja: %s

Regras:
- Máximo 200 caracteres
- Use emojis relevantes
- Destaque o desconto
- Inclua urgência
- Termine com call-to-action

Retorne apenas a mensagem, sem explicações.`,
		offer.Name,
		offer.Price,
		offer.OriginalPrice,
		offer.Discount,
		offer.ShopName,
	)

	return s.callAI(ctx, prompt, "Você é um especialista em copywriting para promoções.")
}

func (s *aiService) SuggestHeadlines(ctx context.Context, category, store, mode string) ([]string, error) {
	if s.apiURL == "" || s.apiKey == "" {
		return nil, fmt.Errorf("AI service not configured")
	}

	storeInfo := ""
	if store != "" {
		storeInfo = fmt.Sprintf("Loja/plataforma: %s\n", store)
	}
	categoryInfo := ""
	if category != "" {
		categoryInfo = fmt.Sprintf("Categoria: %s\n", category)
	}
	modeInfo := ""
	if mode != "" {
		modeInfo = fmt.Sprintf("Modo: %s\n", mode)
	}

	prompt := fmt.Sprintf(`Gere exatamente 3 textos curtos e criativos para envio de ofertas via WhatsApp.
%s%s%s
Regras:
- Cada texto deve ter no máximo 200 caracteres
- Use emojis relevantes (🔥, 💰, 🛒, ⚡, etc.)
- Inclua call-to-action (ex: "Confira!", "Aproveite!", "Corra!")
- Varie o estilo entre os 3 textos (urgência, exclusividade, economia)
- Os textos devem ter placeholder {link} onde o link do produto será inserido
- Escreva em português brasileiro

Retorne APENAS um JSON array com exatamente 3 strings, sem explicações:
["texto1", "texto2", "texto3"]`, categoryInfo, storeInfo, modeInfo)

	response, err := s.callAI(ctx, prompt, "Você é um copywriter especialista em mensagens promocionais para WhatsApp.")
	if err != nil {
		return nil, err
	}

	var texts []string
	if err := json.Unmarshal([]byte(response), &texts); err != nil {
		// Try to clean response (remove markdown fences)
		cleaned := strings.TrimSpace(response)
		cleaned = strings.TrimPrefix(cleaned, "```json")
		cleaned = strings.TrimPrefix(cleaned, "```")
		cleaned = strings.TrimSuffix(cleaned, "```")
		cleaned = strings.TrimSpace(cleaned)
		if err2 := json.Unmarshal([]byte(cleaned), &texts); err2 != nil {
			return nil, fmt.Errorf("failed to parse AI response as JSON array: %w", err)
		}
	}

	return texts, nil
}

func (s *aiService) AnalyzeEngagement(ctx context.Context, groupID string, messages []string) (*models.EngagementAnalysis, error) {
	if s.apiURL == "" || s.apiKey == "" {
		return &models.EngagementAnalysis{
			Score:           0,
			Recommendations: []string{"Configure a API de IA para análise detalhada"},
		}, nil
	}

	// Sample messages for analysis
	sampleSize := 20
	if len(messages) < sampleSize {
		sampleSize = len(messages)
	}
	sample := messages[:sampleSize]

	prompt := fmt.Sprintf(`Analise o engajamento destas mensagens de um grupo de WhatsApp e forneça insights:

Mensagens:
%s

Retorne em JSON:
{
  "score": 0-100,
  "sentiment": "positivo/neutro/negativo",
  "topTopics": ["topic1", "topic2"],
  "peakHours": ["09:00", "14:00"],
  "recommendations": ["recomendação1", "recomendação2"]
}`, sample)

	response, err := s.callAI(ctx, prompt, "Você é um analista de dados especializado em engajamento de comunidades.")
	if err != nil {
		return nil, err
	}

	var analysis models.EngagementAnalysis
	if err := json.Unmarshal([]byte(response), &analysis); err != nil {
		return &models.EngagementAnalysis{
			Score:           50,
			Recommendations: []string{response},
		}, nil
	}

	return &analysis, nil
}

func (s *aiService) AnalyzeTrends(ctx context.Context, category string) (*models.TrendsAnalysis, error) {
	if s.apiURL == "" || s.apiKey == "" {
		return &models.TrendsAnalysis{
			Category: category,
			Insights: []string{"Configure a API de IA para análise de tendências"},
		}, nil
	}

	prompt := fmt.Sprintf(`Analise as tendências atuais de e-commerce para a categoria "%s" no Brasil.

Retorne em JSON:
{
  "category": "%s",
  "trendingProducts": ["produto1", "produto2", "produto3"],
  "priceRange": {"min": 0, "max": 100},
  "seasonality": "alta/média/baixa",
  "insights": ["insight1", "insight2"],
  "recommendedKeywords": ["keyword1", "keyword2"]
}`, category, category)

	response, err := s.callAI(ctx, prompt, "Você é um analista de tendências de e-commerce no Brasil.")
	if err != nil {
		return nil, err
	}

	var trends models.TrendsAnalysis
	if err := json.Unmarshal([]byte(response), &trends); err != nil {
		return &models.TrendsAnalysis{
			Category: category,
			Insights: []string{response},
		}, nil
	}

	return &trends, nil
}

func (s *aiService) ClearClientCache() {
	if s.cache != nil {
		s.cache.ClearLocal()
	}
}

func (s *aiService) callAI(ctx context.Context, prompt, systemPrompt string) (string, error) {
	payload := map[string]interface{}{
		"model": s.model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": prompt},
		},
		"temperature": 0.7,
		"max_tokens":  1000,
	}

	jsonPayload, marshalErr := json.Marshal(payload)
	if marshalErr != nil {
		observability.LogAndCapture(s.logger, sentry.LevelError, "Failed to marshal AI request payload", marshalErr, map[string]string{
			"external_service": "ai",
			"endpoint":        s.apiURL,
		})
		return "", fmt.Errorf("failed to marshal AI payload: %w", marshalErr)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", s.apiURL, bytes.NewReader(jsonPayload))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.apiKey)

	span := sentry.StartSpan(ctx, "http.client", sentry.WithDescription("POST "+s.apiURL))
	span.SetTag("external_service", "ai")
	span.SetTag("endpoint", s.apiURL)
	span.SetTag("ai.model", s.model)
	resp, err := s.httpClient.Do(req)
	span.Finish()
	if err != nil {
		// N5: Detect timeout pattern explicitly for observability
		level := sentry.LevelError
		msg := "AI API HTTP request failed"
		tags := map[string]string{
			"external_service":          "ai",
			"endpoint":                  s.apiURL,
			"circuit_breaker_candidate": "true",
		}
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			tags["error_pattern"] = "timeout"
			msg = "AI API timeout"
			level = sentry.LevelWarning
		}
		observability.LogAndCapture(s.logger, level, msg, err, tags)
		return "", fmt.Errorf("failed to call AI API: %w", err)
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB limit
	if readErr != nil {
		observability.LogAndCapture(s.logger, sentry.LevelError, "Failed to read AI API response body", readErr, map[string]string{
			"external_service": "ai",
			"endpoint":        s.apiURL,
		})
		return "", fmt.Errorf("failed to read AI response: %w", readErr)
	}

	if resp.StatusCode != http.StatusOK {
		httpErr := fmt.Errorf("AI API error %d: %s", resp.StatusCode, string(body))
		observability.LogAndCapture(s.logger, sentry.LevelError, "AI API returned non-200 status", httpErr, map[string]string{
			"external_service":         "ai",
			"endpoint":                 s.apiURL,
			"http_status":              fmt.Sprintf("%d", resp.StatusCode),
			"circuit_breaker_candidate": "true",
		})
		return "", httpErr
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		observability.LogAndCapture(s.logger, sentry.LevelError, "Failed to unmarshal AI API response", err, map[string]string{
			"external_service": "ai",
			"endpoint":        s.apiURL,
		})
		return "", fmt.Errorf("failed to parse AI response: %w", err)
	}

	if len(result.Choices) == 0 {
		noRespErr := fmt.Errorf("no response from AI")
		observability.LogAndCapture(s.logger, sentry.LevelWarning, "AI API returned empty choices", noRespErr, map[string]string{
			"external_service": "ai",
			"endpoint":        s.apiURL,
		})
		return "", noRespErr
	}

	return result.Choices[0].Message.Content, nil
}
