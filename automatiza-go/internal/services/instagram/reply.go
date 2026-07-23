package instagram

import (
	"context"
	"errors"
	"strings"

	"automatiza-go/internal/models"

	"go.uber.org/zap"
)

// ErrInstagramAIUnavailable: a IA não respondeu (não configurada, fora do ar,
// resposta vazia). É terminal para ESTE evento — insistir gastaria custo sem
// perspectiva de melhora dentro da janela.
var ErrInstagramAIUnavailable = errors.New("instagram: AI unavailable")

// ReplyOutcome é o desfecho de uma tentativa de resposta.
type ReplyOutcome struct {
	Text             string
	PromptTokens     int
	CompletionTokens int
	// Skipped indica decisão deliberada de não responder (canal desligado,
	// assunto proibido). SkipCode diz qual, para o ledger.
	Skipped  bool
	SkipCode string
}

// AIReplyGenerator é a fatia do serviço de IA que o Instagram usa. Interface
// própria para o pacote não depender do serviço de IA inteiro.
type AIReplyGenerator interface {
	GenerateInstagramReply(ctx context.Context, req *AIReplyRequest) (*AIReplyResult, error)
}

// AIReplyRequest/AIReplyResult espelham o contrato do serviço de IA sem
// importá-lo aqui (evita ciclo de dependência entre os dois pacotes).
type AIReplyRequest struct {
	Persona         string
	BusinessContext string
	Tone            string
	BlockedTopics   string
	Channel         string
	Content         string
	MaxChars        int
}

type AIReplyResult struct {
	Text             string
	PromptTokens     int
	CompletionTokens int
}

// AIReplyEnabled é o kill switch GLOBAL da resposta automática, separado do
// switch de Stories: dá para ter o canal ligado (cliente conectando conta,
// publicando Story) com a IA desligada para todo mundo.
func (s *Service) AIReplyEnabled() bool {
	return s != nil && s.cfg.AIReplyEnabled && s.aiGen != nil && s.aiConfigRepo != nil && s.FeatureEnabled()
}

// DailyReplyCap devolve a cota diária do plano do usuário. Zero significa "usa
// o padrão do worker" — nunca "ilimitado".
func (s *Service) DailyReplyCap(ctx context.Context, userID string) int {
	if s.planCapRepo == nil {
		return 0
	}
	n, err := s.planCapRepo.MaxAIRepliesPerDay(ctx, userID)
	if err != nil {
		s.logger.Warn("instagram reply: cota do plano indisponivel", zap.String("userID", userID))
		return 0
	}
	return n
}

// ReplyToEvent gera e envia a resposta a um evento de entrada.
//
// A ordem importa: as decisões baratas e locais (canal ligado? assunto
// proibido?) vêm ANTES de gastar uma chamada de IA, que custa dinheiro nosso.
func (s *Service) ReplyToEvent(ctx context.Context, userID string, ev *models.InstagramInboundEvent) (*ReplyOutcome, error) {
	if !s.AIReplyEnabled() {
		return nil, ErrInstagramFeatureDisabled
	}
	cfg, err := s.aiConfigRepo.Get(ctx, userID)
	if err != nil {
		return nil, err
	}
	// Sem configuração ou com a IA desligada pelo próprio cliente: não é erro,
	// é decisão dele.
	if cfg == nil || !cfg.IsActive {
		return &ReplyOutcome{Skipped: true, SkipCode: models.InstagramErrAIDisabled}, nil
	}
	if !channelEnabled(cfg, ev.EventType) {
		return &ReplyOutcome{Skipped: true, SkipCode: models.InstagramErrChannelOff}, nil
	}

	// Assunto proibido: barra ANTES da IA. Confiar só na instrução do prompt
	// deixaria o custo acontecer e dependeria do modelo obedecer.
	if hitsBlockedTopic(cfg.BlockedTopics, ev.Content) {
		if fb := strings.TrimSpace(cfg.FallbackMessage); fb != "" {
			sent, err := s.sendReply(ctx, userID, ev, fb)
			if err != nil {
				return nil, err
			}
			return sent, nil
		}
		return &ReplyOutcome{Skipped: true, SkipCode: models.InstagramErrChannelOff}, nil
	}

	gen, err := s.aiGen.GenerateInstagramReply(ctx, &AIReplyRequest{
		Persona:         cfg.Persona,
		BusinessContext: cfg.BusinessContext,
		Tone:            cfg.Tone,
		BlockedTopics:   cfg.BlockedTopics,
		Channel:         ev.EventType,
		Content:         ev.Content,
		MaxChars:        replyMaxChars,
	})
	if err != nil || gen == nil || strings.TrimSpace(gen.Text) == "" {
		// Mensagem de reserva existe justamente para a IA falhar sem o cliente
		// final ficar sem resposta.
		if fb := strings.TrimSpace(cfg.FallbackMessage); fb != "" {
			out, sendErr := s.sendReply(ctx, userID, ev, fb)
			if sendErr != nil {
				return nil, sendErr
			}
			return out, nil
		}
		return nil, ErrInstagramAIUnavailable
	}

	out, err := s.sendReply(ctx, userID, ev, gen.Text)
	if err != nil {
		return nil, err
	}
	out.PromptTokens = gen.PromptTokens
	out.CompletionTokens = gen.CompletionTokens
	return out, nil
}

// replyMaxChars mantém a resposta dentro do que o Instagram aceita.
const replyMaxChars = 480

// sendReply despacha pelo canal certo.
func (s *Service) sendReply(ctx context.Context, userID string, ev *models.InstagramInboundEvent, text string) (*ReplyOutcome, error) {
	client, token, igUserID, err := s.resolveCredential(ctx, userID)
	if err != nil {
		return nil, err
	}
	if len(text) > replyMaxChars {
		text = text[:replyMaxChars]
	}

	switch ev.EventType {
	case models.InstagramEventComment, models.InstagramEventMention:
		// A resposta vai no próprio comentário (ProviderEventID é o id dele).
		if _, err := client.ReplyToComment(ctx, ev.ProviderEventID, token, text); err != nil {
			return nil, err
		}
	case models.InstagramEventMessage:
		if ev.SenderID == "" {
			// Sem o IGSID de quem escreveu não há para quem responder.
			return &ReplyOutcome{Skipped: true, SkipCode: models.InstagramErrInvalidPayload}, nil
		}
		if _, err := client.SendDirectMessage(ctx, igUserID, token, ev.SenderID, text); err != nil {
			return nil, err
		}
	default:
		return &ReplyOutcome{Skipped: true, SkipCode: models.InstagramErrUnsupportedKind}, nil
	}
	return &ReplyOutcome{Text: text}, nil
}

func channelEnabled(cfg *models.InstagramAIConfig, eventType string) bool {
	switch eventType {
	case models.InstagramEventComment:
		return cfg.ReplyComments
	case models.InstagramEventMessage:
		return cfg.ReplyDMs
	case models.InstagramEventMention:
		return cfg.ReplyMentions
	default:
		return false
	}
}

// hitsBlockedTopic faz uma checagem simples por palavra-chave, uma por linha ou
// separada por vírgula.
//
// ponytail: comparação ingênua (substring, sem acento/plural). O teto é
// conhecido — "reembolso" não pega "reembolsos" escrito com erro. Serve como
// barreira barata ANTES do custo de IA; o refinamento (normalizar acento,
// radical) só vale quando houver caso real mostrando falha.
func hitsBlockedTopic(blocked, content string) bool {
	if strings.TrimSpace(blocked) == "" || content == "" {
		return false
	}
	lower := strings.ToLower(content)
	for _, raw := range strings.FieldsFunc(blocked, func(r rune) bool {
		return r == ',' || r == '\n' || r == ';'
	}) {
		term := strings.ToLower(strings.TrimSpace(raw))
		if len(term) >= 3 && strings.Contains(lower, term) {
			return true
		}
	}
	return false
}

// --- configuração da IA (CRUD para a tela) ---

// GetAIConfig devolve a configuração do cliente. nil quando ele ainda não
// configurou nada.
func (s *Service) GetAIConfig(ctx context.Context, userID string) (*models.InstagramAIConfig, error) {
	if s.aiConfigRepo == nil {
		return nil, nil
	}
	return s.aiConfigRepo.Get(ctx, userID)
}

// SaveAIConfig grava a configuração.
//
// Só liga a IA (IsActive) se houver pelo menos um canal marcado e uma persona
// escrita: ativar sem isso produziria respostas genéricas em nome do cliente,
// que é exatamente o que arrisca a conta dele.
func (s *Service) SaveAIConfig(ctx context.Context, userID string, in *models.InstagramAIConfig) (*models.InstagramAIConfig, error) {
	if s.aiConfigRepo == nil {
		return nil, errors.New("instagram: ai config repository not configured")
	}
	cfg := *in
	cfg.UserID = userID
	cfg.Persona = strings.TrimSpace(cfg.Persona)
	cfg.BusinessContext = strings.TrimSpace(cfg.BusinessContext)
	cfg.Tone = strings.TrimSpace(cfg.Tone)
	cfg.BlockedTopics = strings.TrimSpace(cfg.BlockedTopics)
	cfg.FallbackMessage = strings.TrimSpace(cfg.FallbackMessage)

	if cfg.IsActive {
		anyChannel := cfg.ReplyComments || cfg.ReplyDMs || cfg.ReplyMentions
		if !anyChannel || cfg.Persona == "" {
			cfg.IsActive = false
		}
	}
	return s.aiConfigRepo.Upsert(ctx, &cfg)
}

// ActivityItem junta o evento recebido com a resposta enviada, para a tela.
type ActivityItem struct {
	ID         string
	EventType  string
	Content    string
	Status     string
	ErrorCode  string
	ReceivedAt string
	ReplyText  string
}

// ListActivity monta o histórico do cliente.
func (s *Service) ListActivity(ctx context.Context, userID string, limit int) ([]ActivityItem, error) {
	if s.inboundRepo == nil {
		return nil, nil
	}
	events, err := s.inboundRepo.ListRecentByUser(ctx, userID, limit)
	if err != nil {
		return nil, err
	}
	// Índice das respostas por evento, para não fazer N+1.
	replyByEvent := map[string]string{}
	if s.replyRepo != nil {
		if replies, err := s.replyRepo.ListRecentByUser(ctx, userID, limit); err == nil {
			for i := range replies {
				replyByEvent[replies[i].ProviderEventID] = replies[i].ReplyText
			}
		}
	}
	out := make([]ActivityItem, 0, len(events))
	for i := range events {
		e := &events[i]
		out = append(out, ActivityItem{
			ID: e.ID, EventType: e.EventType, Content: e.Content,
			Status: e.Status, ErrorCode: e.ErrorCode,
			ReceivedAt: e.ReceivedAt, ReplyText: replyByEvent[e.ProviderEventID],
		})
	}
	return out, nil
}
