package monitoring

import (
	"context"
	"strings"
	"time"

	"automatiza-go/internal/lib/commerceurl"
	"automatiza-go/internal/lib/text"
	"automatiza-go/internal/models"
	"automatiza-go/internal/services/monitoroffer"

	"go.uber.org/zap"
)

// Melhoria 9 — o lado do monitoring do retry durável.
//
// Duas responsabilidades, deliberadamente separadas:
//
//  1. RecordFailedLinks: no caminho QUENTE, transforma um link que falhou de
//     forma transitória num item `pending` do ledger. Hoje esse link
//     simplesmente sumia — sem registro, sem retry, sem ninguém saber.
//  2. ExecuteRetry: no worker, reclassifica aquele link e o entrega.
//
// Ambas são no-op com o gate fechado, e ambas engolem os próprios erros: o
// fluxo de envio existente não pode cair por causa do ledger.

// retryCtxKey marca um contexto que JÁ veio de um retry. É o que impede o
// caminho de reentrada de gravar um segundo job para a mesma mensagem.
type retryCtxKey struct{}

func markRetryContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, retryCtxKey{}, true)
}

func isRetryContext(ctx context.Context) bool {
	v, _ := ctx.Value(retryCtxKey{}).(bool)
	return v
}

// RecordFailedLinks grava no ledger os links que falharam de forma transitória.
//
// Chamado do caminho quente, DEPOIS do envio do que deu certo: o registro nunca
// pode atrasar nem bloquear a mensagem que já está pronta para sair. Erro aqui
// é logado e engolido — a feature nova falha nela mesma.
//
// failed traz os links crus que não converteram; eles viram itens `pending` com
// o backoff da §8.1 e o sweep de 1 minuto os enfileira.
func (s *monitoringService) RecordFailedLinks(
	ctx context.Context,
	automation *models.Automation,
	msg *models.IncomingMessage,
	failed []string,
) {
	if s.offerCoordinator == nil || len(failed) == 0 || isRetryContext(ctx) {
		return
	}
	if !s.offerCoordinator.RetryEnabledFor(automation.UserID) {
		return
	}
	if msg.MessageID == "" {
		// §5.5: sem message_id não há identidade estável. Inventar uma a partir
		// de timestamp/corpo permitiria tanto colisão entre mensagens distintas
		// quanto redelivery duplicado da mesma.
		s.logger.Warn("monitoroffer: mensagem sem ID, retry não registrado",
			zap.String("automationID", automation.ID))
		return
	}

	slots, blocked := monitoroffer.ClassifyBody(msg.Body)
	if blocked != "" {
		s.logger.Info("monitoroffer: mensagem bloqueada por limite",
			zap.String("automationID", automation.ID), zap.String("reason", blocked))
		return
	}

	// Só os links que REALMENTE falharam viram item pendente. Os que saíram já
	// estão entregues e não podem ser retentados — reenviar duplicaria no grupo.
	failedSet := make(map[string]struct{}, len(failed))
	for _, l := range failed {
		failedSet[l] = struct{}{}
	}

	items := make([]monitoroffer.ItemResult, 0, len(slots))
	for _, slot := range slots {
		if _, isFailed := failedSet[slot.Raw]; !isFailed {
			continue
		}
		items = append(items, monitoroffer.ItemResult{
			Slot:       slot,
			Class:      models.URLClassUnknown,
			ErrorClass: models.ErrorClassTransient,
			Reason:     models.ReasonRemoteRejected,
			Status:     models.MonitorItemPending,
			Outcome:    models.MonitorOutcomeNone,
		})
	}
	if len(items) == 0 {
		return
	}

	ordered := make([]string, 0, len(slots))
	for _, slot := range slots {
		ordered = append(ordered, slot.Raw)
	}

	s.offerCoordinator.RecordPlan(ctx, monitoroffer.RecordInput{
		UserID:         automation.UserID,
		AutomationID:   automation.ID,
		Mode:           models.MonitorOfferModeMonitoring,
		SourceGroupID:  msg.RemoteJID,
		MessageID:      msg.MessageID,
		MessageTime:    time.Now().UTC(),
		BodyAfterStory: msg.Body,
		FromMe:         msg.FromMe,
		OrderedURLs:    ordered,
	}, monitoroffer.Plan{Items: items})
}

// ExecuteRetry implementa workers.RetryExecutor.
//
// A ordem das checagens é a da §8.3 e não é arbitrária: revalidar o tenant vem
// ANTES de qualquer chamada externa, e a janela vem antes de gastar tentativa.
// Um link cuja automação foi desativada não pode consumir cota da Shopee.
func (s *monitoringService) ExecuteRetry(
	ctx context.Context,
	att models.MonitorOfferAttempt,
	payload models.MonitorOfferPayload,
) (models.MonitorOfferItemOutcome, error) {
	out := models.MonitorOfferItemOutcome{
		ItemID:     att.Item.ID,
		URLClass:   att.Item.URLClass,
		ErrorClass: models.ErrorClassTransient,
		ReasonCode: models.ReasonNone,
		Outcome:    models.MonitorOutcomeNone,
		Status:     models.MonitorItemPending,
	}

	// --- 1. tenant e automação ---------------------------------------------
	auto, err := s.automationRepo.GetAutomationByIDAndUser(ctx, att.Job.AutomationID, att.Job.UserID)
	if err != nil {
		// Erro de leitura é transitório: um soluço do Postgres não pode
		// encerrar o item.
		return out, err
	}
	if auto == nil {
		return supersede(out, models.ReasonAutomationOwnerMism), nil
	}
	if auto.UserID != att.Job.UserID {
		return supersede(out, models.ReasonAutomationOwnerMism), nil
	}
	if auto.Status != models.AutomationStatusActive {
		return supersede(out, models.ReasonAutomationInactive), nil
	}
	// O modo pode ter mudado (o cliente reconfigurou a automação): entregar uma
	// oferta de espelho numa automação que virou lista seria uma mensagem
	// inesperada no grupo.
	if att.Job.Mode == models.MonitorOfferModeMonitoring &&
		(auto.Type == nil || *auto.Type != models.AutomationTypeMonitoring) {
		return supersede(out, models.ReasonModeMismatch), nil
	}
	// O grupo-fonte também: um link capturado no grupo A não pode ser
	// espelhado depois que a automação passou a observar o grupo B.
	if len(auto.MonitorGroupIDs) > 0 && !containsString(auto.MonitorGroupIDs, att.Job.SourceGroupID) {
		return supersede(out, models.ReasonSourceGroupMismatch), nil
	}

	// --- 2. janela ----------------------------------------------------------
	if auto.StartTime != nil && auto.EndTime != nil {
		tw := &models.TimeWindow{
			StartTime: *auto.StartTime, EndTime: *auto.EndTime, Timezone: "America/Sao_Paulo",
		}
		if auto.ScheduleDays != nil {
			tw.Days = auto.ScheduleDays
		}
		if !tw.IsWithinWindow(time.Now()) {
			// Fora da janela NÃO gasta tentativa: o link não tem culpa do
			// horário. O worker reagenda.
			out.ReasonCode = models.ReasonOutsideWindow
			return out, nil
		}
	}

	// --- 3. reclassificação -------------------------------------------------
	raw := urlForOrdinal(payload.OrderedURLs, att.Item.Ordinal)
	if raw == "" {
		return finish(out, models.MonitorItemExhausted, models.ErrorClassTransient,
			models.ReasonInvalidResponse), nil
	}
	u, perr := commerceurl.Parse(raw)
	if perr != nil {
		e, _ := commerceurl.AsError(perr)
		return finish(out, models.MonitorItemBlocked, e.Class, e.Reason), nil
	}

	resolver := monitoroffer.NewStoreResolver(s.shopeeService, s.mlService, s.logger)
	res := resolver.Resolve(ctx, monitoroffer.ResolveRequest{
		URL:    u,
		Local:  commerceurl.ClassifyLocal(u),
		UserID: att.Job.UserID,
		FromMe: payload.FromMe,
	})

	switch res.ErrorClass {
	case models.ErrorClassTransient:
		out.ReasonCode = res.Reason
		return out, nil // segue pending; o worker aplica o backoff
	case models.ErrorClassPermanentConfig:
		if monitoroffer.PausesAutomation(res.ErrorClass) {
			s.pauseAutomationForCredential(ctx, auto, res.Reason)
		}
		return finish(out, models.MonitorItemBlocked, res.ErrorClass, res.Reason), nil
	case models.ErrorClassPermanentConte, models.ErrorClassSecurityBlock:
		return finish(out, models.MonitorItemBlocked, res.ErrorClass, res.Reason), nil
	}

	if res.ConvertedURL == "" {
		// Sem link do tenant não há o que emitir; emitir a origem repassaria a
		// comissão de terceiro.
		out.ReasonCode = models.ReasonPreviewIncomplete
		return out, nil
	}

	// --- 4. entrega ---------------------------------------------------------
	// Reexecuta o caminho de espelho já maduro (chip routing, dedup por
	// destino, dead-group, pacing, cupom, template) com uma mensagem
	// reconstruída contendo SÓ o parágrafo deste link. O contexto marcado
	// impede que essa passagem grave um segundo job.
	replay := &models.IncomingMessage{
		MessageID:    att.Job.MessageID + ":retry:" + att.Item.ID,
		RemoteJID:    att.Job.SourceGroupID,
		Body:         paragraphFor(payload.BodyAfterStory, raw),
		FromMe:       payload.FromMe,
		InstanceName: "",
	}
	if err := s.processAutomation(markRetryContext(ctx), auto, replay,
		[]string{raw}, true, nil); err != nil {
		out.ReasonCode = models.ReasonRemoteRejected
		return out, nil // volta a pending
	}

	out.CanonicalKey = optional(res.CanonicalKey)
	return finish(out, models.MonitorItemSucceeded, models.ErrorClassNone, models.ReasonNone), nil
}

// pauseAutomationForCredential replica a pausa que o caminho quente já faz para
// erro de credencial — é o cliente que resolve, e martelar a API não ajuda.
func (s *monitoringService) pauseAutomationForCredential(ctx context.Context, auto *models.Automation, reason string) {
	now := time.Now().UTC().Format(time.RFC3339)
	if err := s.automationRepo.UpdateAutomation(ctx, auto.ID, map[string]interface{}{
		"status":        models.AutomationStatusPaused,
		"last_error":    "Credenciais da loja ausentes ou inválidas - automação pausada automaticamente",
		"last_error_at": now,
	}); err != nil {
		s.logger.Error("monitoroffer: falha ao pausar automação após erro de credencial",
			zap.String("automationID", auto.ID), zap.String("reason", reason), zap.Error(err))
	}
}

// urlForOrdinal devolve a URL na posição gravada no item.
func urlForOrdinal(urls []string, ordinal int16) string {
	if ordinal < 0 || int(ordinal) >= len(urls) {
		return ""
	}
	return urls[ordinal]
}

// paragraphFor devolve apenas o bloco do corpo que contém a URL.
//
// Reenviar o corpo inteiro no retry duplicaria o texto que já saiu na passagem
// original; o cliente veria a mesma mensagem duas vezes com um link a mais.
func paragraphFor(body, raw string) string {
	for _, p := range text.SplitParagraphs(body) {
		if strings.Contains(p, raw) {
			return p
		}
	}
	return raw
}

func supersede(out models.MonitorOfferItemOutcome, reason string) models.MonitorOfferItemOutcome {
	out.Status = models.MonitorItemSuperseded
	out.ErrorClass = models.ErrorClassNone
	out.ReasonCode = reason
	out.Outcome = models.MonitorOutcomeRemoved
	return out
}

func finish(out models.MonitorOfferItemOutcome, status, class, reason string) models.MonitorOfferItemOutcome {
	out.Status = status
	out.ErrorClass = class
	out.ReasonCode = reason
	if status == models.MonitorItemSucceeded {
		out.Outcome = models.MonitorOutcomeConverted
	} else if status == models.MonitorItemBlocked {
		out.Outcome = models.MonitorOutcomeRemoved
	}
	return out
}

func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// containsString é o "slices.Contains" local; o pacote ainda não usa slices.
func containsString(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}
