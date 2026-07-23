package monitoring

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/config"
	"automatiza-go/internal/lib/selfsend"
	"automatiza-go/internal/models"
	"automatiza-go/internal/observability"
	"automatiza-go/internal/repository"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Cadência durável do espelho (Melhoria 7) — lado do serviço.
//
// Este arquivo tem duas metades:
//
//	ADMISSÃO  (PrepareCadencedMessage) roda SÍNCRONA no consumidor de webhook,
//	          antes de qualquer efeito colateral. Se ela falha, nada aconteceu
//	          e o webhook retenta; se ela devolve sem erro, a mensagem já é uma
//	          linha no banco e nenhum crash posterior a apaga.
//
//	DESPACHO  (RunDeferredDispatch) roda no worker, mais tarde, e reusa o MESMO
//	          pipeline de espelhamento do caminho vivo (processAutomation) —
//	          conversão de link, cupom, template, reputação, dead-group, chip,
//	          pacing e cascata de envio. Nenhuma regra de negócio é reescrita
//	          aqui; o que este arquivo acrescenta é o ledger que decide o que
//	          ainda pode ser enviado.

// CadenceAdmission é o veredito da admissão para UMA mensagem.
type CadenceAdmission struct {
	// Deferred: automações cuja mensagem entrou na fila. O caminho imediato
	// NÃO deve processá-las para este evento (senão sai duas vezes).
	Deferred map[string]struct{}
	// Suppressed: conteúdo idêntico já aceito na janela de dedup — nem fila
	// nem envio imediato.
	Suppressed map[string]struct{}
}

// Excluded devolve o conjunto que o caminho imediato deve pular.
func (a CadenceAdmission) Excluded() map[string]struct{} {
	if len(a.Deferred) == 0 && len(a.Suppressed) == 0 {
		return nil
	}
	out := make(map[string]struct{}, len(a.Deferred)+len(a.Suppressed))
	for id := range a.Deferred {
		out[id] = struct{}{}
	}
	for id := range a.Suppressed {
		out[id] = struct{}{}
	}
	return out
}

// CadenceAdmitter é a fatia da cadência consumida pelo processador de webhook.
// Interface separada (em vez de crescer monitoring.Service) para que todo mock
// existente de Service continue válido: quem não implementa isto simplesmente
// não tem cadência.
type CadenceAdmitter interface {
	// PrepareCadencedMessage aceita (ou não) a mensagem na fila durável.
	// Erro = nada foi aceito e o evento deve ser retentado.
	PrepareCadencedMessage(ctx context.Context, msg *models.IncomingMessage) (CadenceAdmission, error)
	// ProcessIncomingMessageExcluding é o caminho imediato pulando as
	// automações que a cadência assumiu.
	ProcessIncomingMessageExcluding(ctx context.Context, msg *models.IncomingMessage, excluded map[string]struct{}) error
}

// WithCadence liga a cadência durável. Sem esta option (ou com a env
// FEATURE_MONITOR_DISPATCH_CADENCE desligada) o serviço se comporta
// exatamente como antes — a admissão retorna antes de tocar o banco.
func WithCadence(repo repository.MonitorCadenceRepository, media whatsapp.DurableMediaFetcher) Option {
	return func(s *monitoringService) {
		s.cadenceRepo = repo
		s.durableMedia = media
	}
}

// cadenceEnabled: a flag é consultada a cada mensagem de propósito (é um
// sync.OnceValue, custo zero) para o kill-switch valer sem redeploy.
func (s *monitoringService) cadenceEnabled() bool {
	return s.cadenceRepo != nil && config.MonitorDispatchCadenceEnabled()
}

// PrepareCadencedMessage implements CadenceAdmitter.
func (s *monitoringService) PrepareCadencedMessage(ctx context.Context, msg *models.IncomingMessage) (CadenceAdmission, error) {
	var empty CadenceAdmission
	if !s.cadenceEnabled() {
		return empty, nil
	}
	// Mesmos guards do caminho vivo, na mesma ordem — uma mensagem que o
	// espelho não processaria também não pode ocupar fila.
	if !strings.HasSuffix(msg.RemoteJID, "@g.us") || msg.FromMe || msg.MessageID == "" {
		return empty, nil
	}
	if len(s.extractLinks(msg.Body)) == 0 {
		return empty, nil
	}
	if selfsend.IsPlatformSend(ctx, s.cache, msg.RemoteJID, msg.Body) {
		return empty, nil
	}

	automations, err := s.findMatchingAutomations(ctx, msg.InstanceName, msg.RemoteJID)
	if err != nil {
		// Falha de leitura é RETENTÁVEL: devolver "sem cadência" aqui faria a
		// mensagem sair imediata, furando o intervalo configurado.
		return empty, fmt.Errorf("cadence: resolve automations: %w", err)
	}
	if len(automations) == 0 {
		return empty, nil
	}

	userID := automations[0].UserID
	candidates := make([]models.MonitorAdmissionAutomation, 0, len(automations))
	ids := make([]string, 0, len(automations))
	for i := range automations {
		a := &automations[i]
		if !a.IsMonitoring() || a.UserID != userID {
			continue
		}
		ids = append(ids, a.ID)
		if a.GetMonitorIntervalMinutes() > 0 {
			candidates = append(candidates, s.admissionAutomation(a))
		}
	}
	if len(ids) == 0 {
		return empty, nil
	}

	// Intervalo zerado em todas: só continua se ainda houver fila drenando —
	// caso contrário mensagem nova ultrapassaria itens já reservados.
	if len(candidates) == 0 {
		queued, hErr := s.cadenceRepo.HasNonTerminalPending(ctx, ids)
		if hErr != nil {
			return empty, fmt.Errorf("cadence: check queue: %w", hErr)
		}
		if !queued {
			return empty, nil
		}
		for i := range automations {
			if automations[i].IsMonitoring() && automations[i].UserID == userID {
				candidates = append(candidates, s.admissionAutomation(&automations[i]))
			}
		}
	}

	// Mídia DURÁVEL antes da transação: baixar segurando transação aberta
	// prenderia conexão do pool por segundos. Falhar aqui recusa a mensagem
	// inteira — imagem é parte do conteúdo, e agendar um envio cuja imagem
	// vai expirar é prometer o que não se pode cumprir.
	media, err := s.acquireDurableMedia(ctx, msg)
	if err != nil {
		return empty, err
	}

	body := sha256.Sum256([]byte(msg.Body))
	receivedAt := msg.GetTimestamp()
	if receivedAt.IsZero() || msg.Timestamp <= 0 {
		receivedAt = time.Now().UTC()
	}

	res, err := s.cadenceRepo.Admit(ctx, models.MonitorAdmissionInput{
		UserID:            userID,
		SourceSessionName: msg.InstanceName,
		SourceChatID:      msg.RemoteJID,
		SourceMessageID:   msg.MessageID,
		ContentSHA256:     hex.EncodeToString(body[:]),
		ReceivedAt:        receivedAt,
		MessageSnapshot:   models.NewMonitorMessageSnapshot(msg),
		Media:             media,
		Automations:       candidates,
		Schedule: func(base time.Time, w models.MonitorWindowSnapshot) (time.Time, error) {
			return AdvanceToAllowedWindow(base, w, monitorLocation())
		},
	})
	if err != nil {
		return empty, err
	}

	admission := CadenceAdmission{
		Deferred:   make(map[string]struct{}, len(res.Deferred)),
		Suppressed: res.Suppressed,
	}
	for id := range res.Deferred {
		admission.Deferred[id] = struct{}{}
	}
	s.logger.Info("Monitor cadence admission",
		zap.Int("deferred", len(admission.Deferred)),
		zap.Int("suppressed", len(admission.Suppressed)),
		zap.Bool("redelivery", res.Redelivery))
	observability.RecordMonitorCadenceAdmission(len(admission.Deferred), len(admission.Suppressed), res.Redelivery)
	return admission, nil
}

func (s *monitoringService) admissionAutomation(a *models.Automation) models.MonitorAdmissionAutomation {
	delay := 0
	if a.DelaySeconds != nil && *a.DelaySeconds > 0 {
		delay = *a.DelaySeconds
	}
	return models.MonitorAdmissionAutomation{
		ID:              a.ID,
		IntervalMinutes: a.GetMonitorIntervalMinutes(),
		DelaySeconds:    delay,
		Window:          WindowSnapshotFor(a),
	}
}

// ProcessIncomingMessageExcluding implements CadenceAdmitter.
func (s *monitoringService) ProcessIncomingMessageExcluding(ctx context.Context, msg *models.IncomingMessage, excluded map[string]struct{}) error {
	return s.processIncoming(ctx, msg, false, excluded)
}

// acquireDurableMedia baixa e valida os bytes da imagem-fonte que o espelho
// encaminharia. Devolve (nil, nil) quando não há imagem a persistir.
//
// A ordem é a mesma do caminho vivo (media.url → re-download → base64 inline),
// mas o desfecho é diferente: aqui, falhar em obter bytes é ERRO. Guardar
// apenas a URL da WAHA seria guardar uma referência que a doc oficial não
// promete manter viva (waha-plus-openapi.json, schema WAMedia) — e o item pode
// só sair daqui a horas.
func (s *monitoringService) acquireDurableMedia(ctx context.Context, msg *models.IncomingMessage) (*models.MonitorPayloadMedia, error) {
	isImage := msg.HasMedia && (msg.MediaType == "image" || strings.HasPrefix(msg.MimeType, "image/"))
	if !isImage {
		return nil, nil
	}

	var data []byte
	var mimetype, filename string

	if s.durableMedia != nil {
		if msg.MediaURL != "" {
			res, err := s.durableMedia.DownloadMediaByURLLimited(ctx, msg.MediaURL, models.MonitorMaxMediaBytes)
			switch {
			case errors.Is(err, whatsapp.ErrMediaTooLarge):
				return nil, errors.New(models.MonitorCodeMediaTooLarge)
			case err == nil && res != nil && len(res.Data) > 0:
				data, mimetype, filename = res.Data, res.Mimetype, res.Filename
			}
		}
		if len(data) == 0 && msg.MessageID != "" && msg.RemoteJID != "" {
			res, err := s.durableMedia.RedownloadMediaLimited(ctx, msg.InstanceName, msg.RemoteJID, msg.MessageID, models.MonitorMaxMediaBytes)
			switch {
			case errors.Is(err, whatsapp.ErrMediaTooLarge):
				return nil, errors.New(models.MonitorCodeMediaTooLarge)
			case err == nil && res != nil && len(res.Data) > 0:
				data, mimetype, filename = res.Data, res.Mimetype, res.Filename
			}
		}
	}

	// Último recurso: o base64 inline do webhook (costuma ser o thumbnail do
	// link preview). Baixa resolução, mas é conteúdo real e já está em mãos.
	if len(data) == 0 && msg.MediaBase64 != "" {
		decoded, err := base64.StdEncoding.DecodeString(msg.MediaBase64)
		if err == nil && len(decoded) > 0 {
			data, mimetype = decoded, msg.MimeType
		}
	}

	if len(data) == 0 {
		return nil, errors.New(models.MonitorCodeMediaUnavailable)
	}
	if len(data) > models.MonitorMaxMediaBytes {
		return nil, errors.New(models.MonitorCodeMediaTooLarge)
	}
	if mimetype == "" {
		mimetype = msg.MimeType
	}
	if mimetype == "" || mimetype == "application/octet-stream" {
		mimetype = "image/jpeg"
	}

	sum := sha256.Sum256(data)
	return &models.MonitorPayloadMedia{
		Mimetype:  mimetype,
		Filename:  filename,
		SHA256:    hex.EncodeToString(sum[:]),
		SizeBytes: len(data),
		Bytes:     data,
	}, nil
}

// errWindowClosed sinaliza, de dentro do pipeline compartilhado, que a janela
// da automação fechou — o item volta para a fila em vez de ser descartado.
var errWindowClosed = errors.New(models.MonitorCodeWindowClosed)

// RunDeferredDispatch executa UM item da fila. Chamado pelo worker asynq com o
// par (geração, token) que o poller emitiu: qualquer task mais velha que isso
// perde o CAS e vira no-op silencioso.
func (s *monitoringService) RunDeferredDispatch(ctx context.Context, pendingID int64, generation int, token string) error {
	if s.cadenceRepo == nil {
		return nil
	}

	work, err := s.cadenceRepo.ClaimPending(ctx, pendingID, generation, token)
	if err != nil {
		return err
	}
	if work == nil {
		// Task velha, replay ou item já recuperado por lease: nada a fazer.
		observability.RecordMonitorCadenceEnqueue("stale")
		return nil
	}

	automation, err := s.loadDispatchableAutomation(ctx, work)
	if err != nil {
		return s.retryOrFail(ctx, work, models.MonitorCodePrepareFailed, err)
	}
	if automation == nil {
		// Apagada, inativa ou deixou de ser monitoring: cancela de forma
		// auditável e não envia mais nada por ela.
		if fErr := s.cadenceRepo.FinalizePending(ctx, work.PendingID, work.ProcessingToken,
			models.MonitorPendingCanceled, models.MonitorPendingResult{}, models.MonitorCodeAutomationGone); fErr != nil {
			return fErr
		}
		if _, cErr := s.cadenceRepo.CancelForAutomation(ctx, work.AutomationSnapshot); cErr != nil {
			s.logger.Warn("cadence: cancel remaining queue failed", zap.Error(cErr))
		}
		return nil
	}

	// Janela ATUAL (não o snapshot): o cliente pode tê-la editado depois da
	// reserva, e o que vale para sair agora é a configuração de agora.
	now := time.Now()
	currentWindow := WindowSnapshotFor(automation)
	next, wErr := AdvanceToAllowedWindow(now, currentWindow, monitorLocation())
	if wErr != nil {
		return s.cadenceRepo.FinalizePending(ctx, work.PendingID, work.ProcessingToken,
			models.MonitorPendingFailed, models.MonitorPendingResult{}, models.MonitorCodeInvalidWindow)
	}
	if next.After(now) {
		// Só o horário de liberação muda. sequence_no e scheduled_at são
		// imutáveis, então nenhum sucessor ultrapassa este item.
		return s.cadenceRepo.ReschedulePending(ctx, work.PendingID, work.ProcessingToken, next, models.MonitorCodeWindowClosed)
	}

	if !work.FirstStarted {
		if err := s.cadenceRepo.MarkPendingStarted(ctx, work.PendingID, work.ProcessingToken); err != nil {
			return err
		}
	}

	msg := work.Message.ToIncomingMessage()
	links := s.extractLinks(msg.Body)
	run := &deferredRun{repo: s.cadenceRepo, work: work, logger: s.logger}

	dispatchErr := s.processAutomation(ctx, automation, msg, links, false, run)
	if errors.Is(dispatchErr, errWindowClosed) {
		reopen, rErr := AdvanceToAllowedWindow(time.Now(), currentWindow, monitorLocation())
		if rErr != nil {
			reopen = time.Now().Add(time.Minute)
		}
		return s.cadenceRepo.ReschedulePending(ctx, work.PendingID, work.ProcessingToken, reopen, models.MonitorCodeWindowClosed)
	}

	return s.finalizeDeferred(ctx, work, run, dispatchErr)
}

// loadDispatchableAutomation revalida dono, existência, status e tipo. nil,nil
// = não despachável (cancelar); erro = falha de leitura (retentável).
func (s *monitoringService) loadDispatchableAutomation(ctx context.Context, work *models.MonitorPendingWork) (*models.Automation, error) {
	if work.AutomationID == "" {
		return nil, nil
	}
	automation, err := s.automationRepo.GetAutomationByIDAndUser(ctx, work.AutomationID, work.UserID)
	if err != nil {
		return nil, err
	}
	if automation == nil || !automation.IsActive() || !automation.IsMonitoring() {
		return nil, nil
	}
	return automation, nil
}

// finalizeDeferred fecha o item pelo ESTADO DOS DESTINOS, nunca pelo retorno da
// função de envio: é o ledger que sabe o que saiu.
func (s *monitoringService) finalizeDeferred(ctx context.Context, work *models.MonitorPendingWork, run *deferredRun, dispatchErr error) error {
	dests, err := s.cadenceRepo.ListDestinations(ctx, work.PendingID)
	if err != nil {
		return err
	}

	var result models.MonitorPendingResult
	var open int
	var earliest time.Time
	for i := range dests {
		d := &dests[i]
		switch d.Status {
		case models.MonitorDestinationSent:
			result.SentCount++
		case models.MonitorDestinationAssumedSent:
			result.AssumedSentCount++
		case models.MonitorDestinationSkipped:
			result.SkippedCount++
		case models.MonitorDestinationFailed:
			result.FailedCount++
		case models.MonitorDestinationCanceled:
			result.CanceledCount++
		default:
			open++
			if earliest.IsZero() || d.NotBeforeAt.Before(earliest) {
				earliest = d.NotBeforeAt
			}
		}
	}

	// Nenhum destino materializado: a preparação decidiu que não havia nada a
	// enviar (sem link da loja configurada, repasse retido, texto vazio) ou
	// falhou antes de materializar.
	if len(dests) == 0 {
		if dispatchErr != nil {
			return s.retryOrFail(ctx, work, models.MonitorCodePrepareFailed, dispatchErr)
		}
		return s.cadenceRepo.FinalizePending(ctx, work.PendingID, work.ProcessingToken,
			models.MonitorPendingCompleted, result, "")
	}

	if open > 0 {
		// Sobrou destino retentável: o item VOLTA para a fila com o horário do
		// destino mais cedo. Ele continua sendo o head, então os sucessores
		// esperam — a ordem por sequência é preservada.
		if earliest.Before(time.Now()) {
			earliest = time.Now()
		}
		return s.cadenceRepo.ReschedulePending(ctx, work.PendingID, work.ProcessingToken, earliest, models.MonitorCodeSendFailed)
	}

	status := models.MonitorPendingCompleted
	code := ""
	if result.FailedCount > 0 {
		status = models.MonitorPendingFailed
		code = models.MonitorCodeSendFailed
	}
	return s.cadenceRepo.FinalizePending(ctx, work.PendingID, work.ProcessingToken, status, result, code)
}

// retryOrFail aplica o backoff quadrático (1, 4, 9, 16, 25 min) às falhas
// ANTES de existir ledger de destino. Depois que os destinos existem, quem
// conta tentativa é cada destino — o item não pode morrer no meio.
func (s *monitoringService) retryOrFail(ctx context.Context, work *models.MonitorPendingWork, code string, cause error) error {
	if work.ProcessingAttempts >= models.MonitorMaxDestinationAttempts {
		s.logger.Error("cadence: giving up on pending item",
			zap.Int64("pendingID", work.PendingID),
			zap.Int("attempts", work.ProcessingAttempts),
			zap.String("code", code))
		return s.cadenceRepo.FinalizePending(ctx, work.PendingID, work.ProcessingToken,
			models.MonitorPendingFailed, models.MonitorPendingResult{}, code)
	}
	backoff := time.Duration(work.ProcessingAttempts*work.ProcessingAttempts) * time.Minute
	if backoff <= 0 {
		backoff = time.Minute
	}
	s.logger.Warn("cadence: rescheduling pending item",
		zap.Int64("pendingID", work.PendingID),
		zap.Int("attempt", work.ProcessingAttempts),
		zap.Duration("backoff", backoff),
		zap.Error(cause))
	return s.cadenceRepo.ReschedulePending(ctx, work.PendingID, work.ProcessingToken,
		time.Now().Add(backoff), code)
}

// deferredRun é o ledger de UMA execução, plugado no pipeline compartilhado.
// Cada método corresponde a uma fronteira de crash da matriz do plano.
type deferredRun struct {
	repo   repository.MonitorCadenceRepository
	work   *models.MonitorPendingWork
	logger *zap.Logger

	tokens   map[string]string
	ids      map[string]int64
	attempts map[string]int
}

// materialize congela a lista de destinos na primeira execução e devolve, em
// ordem de ordinal, apenas os que ainda podem ser trabalhados. Em retomada, a
// lista computada agora é IGNORADA em favor das linhas já gravadas: reembaralhar
// entre tentativas faria um destino já atendido reaparecer em outra posição.
func (r *deferredRun) materialize(ctx context.Context, groups []string, canonicalKey, store string) ([]string, error) {
	rows := make([]models.MonitorPendingDestination, 0, len(groups))
	for i, g := range groups {
		snap, _ := json.Marshal(map[string]string{"store": store})
		rows = append(rows, models.MonitorPendingDestination{
			Ordinal:      i,
			GroupID:      g,
			CanonicalKey: canonicalKey,
			Snapshot:     snap,
		})
	}
	dests, err := r.repo.MaterializeDestinations(ctx, r.work.PendingID, rows)
	if err != nil {
		return nil, err
	}

	r.tokens = make(map[string]string, len(dests))
	r.ids = make(map[string]int64, len(dests))
	r.attempts = make(map[string]int, len(dests))
	out := make([]string, 0, len(dests))
	for i := range dests {
		d := &dests[i]
		r.ids[d.GroupID] = d.ID
		r.attempts[d.GroupID] = d.Attempts
		if d.IsTerminal() {
			continue
		}
		out = append(out, d.GroupID)
	}
	return out, nil
}

// claim reivindica o destino. false = outro processo pegou, o destino já é
// terminal ou o backoff dele ainda não venceu.
func (r *deferredRun) claim(ctx context.Context, groupID string) bool {
	id, ok := r.ids[groupID]
	if !ok {
		return false
	}
	token := newDestinationToken()
	claimed, err := r.repo.ClaimDestination(ctx, id, token)
	if err != nil {
		r.logger.Warn("cadence: claim destination failed", zap.Error(err))
		return false
	}
	if !claimed {
		return false
	}
	r.tokens[groupID] = token
	// ClaimDestination já incrementou attempts no banco; espelha aqui para o
	// backoff não precisar de outra ida ao banco.
	r.attempts[groupID]++
	return true
}

// authorizeSend cruza a fronteira do resultado incerto. A partir daqui, este
// destino NUNCA recebe outro POST sem prova de que o primeiro não começou —
// e essa prova não existe, então o desfecho na dúvida é assumed_sent.
func (r *deferredRun) authorizeSend(ctx context.Context, groupID string) bool {
	id, ok := r.ids[groupID]
	if !ok {
		return false
	}
	ok, err := r.repo.MarkDestinationSending(ctx, id, r.tokens[groupID])
	if err != nil {
		r.logger.Warn("cadence: mark sending failed", zap.Error(err))
		return false
	}
	return ok
}

// finish grava o desfecho do destino. retryIn > 0 devolve o destino para
// pending com backoff (falha comprovadamente anterior à entrega).
func (r *deferredRun) finish(ctx context.Context, groupID, status, code string, retryIn time.Duration) {
	id, ok := r.ids[groupID]
	if !ok {
		return
	}
	var notBefore *time.Time
	if retryIn > 0 {
		t := time.Now().Add(retryIn)
		notBefore = &t
	}
	if err := r.repo.FinishDestination(ctx, id, status, code, notBefore); err != nil {
		r.logger.Error("cadence: finish destination failed",
			zap.Int64("destinationID", id), zap.Error(err))
	}
	observability.RecordMonitorCadenceDestination(status)
}

// retryOrFailDestination decide entre devolver o destino à fila (com backoff
// quadrático) e encerrá-lo como falho na 5ª tentativa.
func (r *deferredRun) retryOrFailDestination(ctx context.Context, groupID, code string) {
	attempts := r.attempts[groupID]
	if attempts >= models.MonitorMaxDestinationAttempts {
		r.finish(ctx, groupID, models.MonitorDestinationFailed, code, 0)
		return
	}
	backoff := time.Duration(attempts*attempts) * time.Minute
	if backoff <= 0 {
		backoff = time.Minute
	}
	r.finish(ctx, groupID, models.MonitorDestinationPending, code, backoff)
}

// newDestinationToken gera o token de posse de um destino. UUID v4: o valor
// nunca é adivinhável, então uma execução zumbi não consegue forjar o CAS de
// outra.
func newDestinationToken() string {
	return uuid.NewString()
}
