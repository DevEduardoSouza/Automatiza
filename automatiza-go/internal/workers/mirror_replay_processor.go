// Package workers — replay de mensagens recuperadas pelo backfill para o
// pipeline de espelho/offer_generate.
//
// Antes, o único MessageProcessor da reconciliação era o DispatchLogProcessor
// (contabilidade: cria dispatch_logs status=reconciled). Ou seja: mesmo com o
// backfill funcionando, uma mensagem-fonte perdida NUNCA era re-espelhada —
// a "recuperação de gaps" registrava o gap sem recuperar nada. Este processor
// fecha o ciclo: converte a mensagem recuperada e a injeta no MESMO pipeline
// do caminho vivo (monitoring + offer_generate), em modo replay.
package workers

import (
	"context"
	"strings"
	"time"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/lib/text"
	"automatiza-go/internal/models"
	"automatiza-go/internal/services/monitoring"
	"automatiza-go/internal/services/offer_generate"
	"automatiza-go/internal/services/reconciliation"

	"go.uber.org/zap"
)

// mirrorReplayMaxAge é o teto de idade de uma mensagem recuperada para ainda
// ser re-espelhada. TEM que ficar ≤ staleWebhookMessageTTL (10min): o caminho
// vivo não repassa nada mais velho que isso, e os guards de loop
// (og_self_send/selfsend, TTL 10min) e janelas de claim assumem esse teto.
const mirrorReplayMaxAge = staleWebhookMessageTTL

// MirrorReplayProcessor implementa reconciliation.MessageProcessor reinjetando
// mensagens recuperadas no pipeline de espelho. Idempotência: os serviços são
// chamados em modo replay (pulam claims de nível mensagem, que a passagem
// original pode ter reivindicado antes de morrer) e a proteção real é o claim
// fail-closed POR DESTINO — destinos já atendidos dedupam, só os perdidos
// recebem a mensagem.
type MirrorReplayProcessor struct {
	monitoringService    monitoring.Service
	offerGenerateService offer_generate.Service
	logger               *zap.Logger
}

// NewMirrorReplayProcessor cria o processor de replay. Qualquer serviço pode
// ser nil (é pulado).
func NewMirrorReplayProcessor(
	monitoringService monitoring.Service,
	offerGenerateService offer_generate.Service,
	logger *zap.Logger,
) *MirrorReplayProcessor {
	return &MirrorReplayProcessor{
		monitoringService:    monitoringService,
		offerGenerateService: offerGenerateService,
		logger:               logger.Named("mirror-replay"),
	}
}

// ProcessMessage implementa reconciliation.MessageProcessor.
func (p *MirrorReplayProcessor) ProcessMessage(ctx context.Context, sessionName string, msg *whatsapp.Message) error {
	if msg == nil || !strings.HasSuffix(msg.ChatID, "@g.us") {
		return nil
	}
	if msg.Body == "" && !msg.HasMedia {
		return nil
	}
	// Gate de frescor — espelha o staleWebhookMessageTTL do caminho vivo.
	// O doReconcile já clampa a janela; isto é defesa em profundidade.
	if age := time.Since(msg.Timestamp); age > mirrorReplayMaxAge {
		return nil
	}

	// Strip do {story} no re-espelho: a tag não pode vazar no texto espelhado.
	// Replay é mensagem VELHA (reconciliação), então NÃO dispara story — só
	// limpa o corpo. Incondicional é seguro (o regex só casa a tag no início).
	body := text.StripStoryTag(msg.Body)
	// Sigla de repasse literal: mesma leitura do caminho vivo. Sem isto, uma
	// mensagem recuperada pelo backfill sairia FORMULADA — diferente da que o
	// cliente postou e que a passagem original teria repassado como está.
	verbatim := text.HasVerbatimTag(body)
	if verbatim {
		body = text.StripVerbatimTag(body)
	}

	im := &models.IncomingMessage{
		InstanceName:  sessionName,
		MessageID:     msg.ID,
		RemoteJID:     msg.ChatID,
		FromMe:        msg.FromMe,
		SenderJID:     msg.Sender,
		Body:          body,
		RelayVerbatim: verbatim,
		Timestamp:     msg.Timestamp.Unix(),
		HasMedia:      msg.HasMedia,
		MediaType:     msg.MediaType,
		MediaURL:      msg.MediaURL,
	}

	p.logger.Info("Replaying reconciled message through mirror pipeline",
		zap.String("session", sessionName),
		zap.String("chat", msg.ChatID),
		zap.String("messageID", msg.ID),
		zap.Bool("fromMe", msg.FromMe))

	// Mesma divisão do caminho vivo: monitoring nunca vê fromMe; o
	// offer_generate vê (gatilho legítimo do dono) e tem os próprios guards.
	if p.monitoringService != nil && !msg.FromMe {
		if err := p.monitoringService.ProcessReconciledMessage(ctx, im); err != nil {
			p.logger.Warn("Mirror replay: monitoring processing failed",
				zap.String("messageID", msg.ID), zap.Error(err))
		}
	}
	if p.offerGenerateService != nil {
		if err := p.offerGenerateService.ProcessReconciledMessage(ctx, im); err != nil {
			p.logger.Warn("Mirror replay: offer_generate processing failed",
				zap.String("messageID", msg.ID), zap.Error(err))
		}
	}

	return nil
}

// CompositeReconcileProcessor encadeia múltiplos MessageProcessor da
// reconciliação (contabilidade + replay). Processors nil são ignorados; erros
// são independentes (um processor falhar não silencia o outro) e o primeiro
// erro é retornado para o retry do ReconcileChat.
type CompositeReconcileProcessor struct {
	processors []reconciliation.MessageProcessor
}

// NewCompositeReconcileProcessor cria o composite, filtrando nils.
func NewCompositeReconcileProcessor(processors ...reconciliation.MessageProcessor) *CompositeReconcileProcessor {
	out := &CompositeReconcileProcessor{}
	for _, p := range processors {
		if p != nil {
			out.processors = append(out.processors, p)
		}
	}
	return out
}

// ProcessMessage implementa reconciliation.MessageProcessor.
func (c *CompositeReconcileProcessor) ProcessMessage(ctx context.Context, sessionName string, msg *whatsapp.Message) error {
	var firstErr error
	for _, p := range c.processors {
		if err := p.ProcessMessage(ctx, sessionName, msg); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// Compile-time checks.
var (
	_ reconciliation.MessageProcessor = (*MirrorReplayProcessor)(nil)
	_ reconciliation.MessageProcessor = (*CompositeReconcileProcessor)(nil)
)
