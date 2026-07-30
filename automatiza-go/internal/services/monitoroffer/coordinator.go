package monitoroffer

import (
	"context"
	"time"

	"automatiza-go/internal/lib/commerceurl"
	"automatiza-go/internal/lib/usergate"
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"

	"go.uber.org/zap"
)

// Coordinator liga o planner ao ledger.
//
// Ele é o único lugar que decide SE o ledger é usado: com
// MONITOR_PRODUCT_RETRY off para o usuário, nenhuma linha é escrita, nenhuma
// task é enfileirada, e os serviços seguem exatamente o caminho de hoje.
//
// Regra de isolamento (fluxo quente é sagrado): TODO erro daqui é logado e
// engolido. Falha de banco no ledger não pode derrubar um envio que ia
// acontecer de qualquer jeito — a feature nova falha nela mesma.
type Coordinator struct {
	repo          repository.MonitorOfferRetryRepository
	enqueuer      RetryEnqueuer
	retryGate     usergate.Gate
	mirrorGate    usergate.Gate
	encryptionKey string
	logger        *zap.Logger
}

// RetryEnqueuer publica a task de retry. Interface estreita para o coordinator
// não importar o pacote de workers (evita ciclo).
type RetryEnqueuer interface {
	EnqueueMonitorOfferRetry(ctx context.Context, itemID string, generation int, runAt time.Time) error
}

// NewCoordinator monta o coordinator. repo nil = feature ausente: todos os
// métodos viram no-op e nenhum caminho de produção muda de comportamento.
func NewCoordinator(
	repo repository.MonitorOfferRetryRepository,
	enqueuer RetryEnqueuer,
	retryGate, mirrorGate usergate.Gate,
	encryptionKey string,
	logger *zap.Logger,
) *Coordinator {
	return &Coordinator{
		repo: repo, enqueuer: enqueuer,
		retryGate: retryGate, mirrorGate: mirrorGate,
		encryptionKey: encryptionKey, logger: logger,
	}
}

// RetryEnabledFor reporta se o ledger/retry vale para este usuário.
func (c *Coordinator) RetryEnabledFor(userID string) bool {
	return c != nil && c.repo != nil && c.retryGate.AllowsString(userID)
}

// MirrorEnabledFor reporta se o espelho de conteúdo comum vale para este
// usuário. Independente do gate de retry: dá para espelhar sem ledger.
func (c *Coordinator) MirrorEnabledFor(userID string) bool {
	return c != nil && c.mirrorGate.AllowsString(userID)
}

// RecordInput descreve a mensagem que gerou o plano.
type RecordInput struct {
	UserID        string
	AutomationID  string
	Mode          string
	SourceGroupID string
	MessageID     string
	MessageTime   time.Time
	// BodyAfterStory é o corpo cifrado no payload (conserva info/cta/desc).
	BodyAfterStory string
	FromMe         bool
	OrderedURLs    []string
}

// RecordPlan grava job + itens e agenda os retries dos itens pendentes.
//
// Devolve owned=false quando a identidade da mensagem JÁ existia: nesse caso o
// chamador NÃO deve enviar nada. É esta linha que faz redelivery de webhook
// parar de gerar uma segunda oferta no grupo do cliente.
//
// Com o gate off devolve owned=true sem tocar no banco: o serviço segue o
// caminho histórico, que não tem ledger nenhum.
func (c *Coordinator) RecordPlan(ctx context.Context, in RecordInput, plan Plan) (jobID string, owned bool) {
	if !c.RetryEnabledFor(in.UserID) {
		return "", true
	}

	payload := models.MonitorOfferPayload{
		BodyAfterStory:   in.BodyAfterStory,
		FromMe:           in.FromMe,
		MessageTimestamp: in.MessageTime,
		OrderedURLs:      in.OrderedURLs,
	}
	ciphertext, encErr := EncodePayload(payload, c.encryptionKey)
	jobReason := models.ReasonNone
	if encErr != nil {
		// Sem payload não há retry possível. O job continua sendo criado —
		// ele é a identidade que impede o redelivery duplicado — mas nasce
		// marcado, e os itens que dependeriam de retry não ficam pending.
		jobReason = models.ReasonPayloadTooLarge
		if encErr != ErrPayloadTooLarge {
			jobReason = models.ReasonPayloadEncryptFail
		}
		c.logger.Warn("monitoroffer: payload não cifrado, job segue sem retry",
			zap.String("user_id", in.UserID),
			zap.String("reason", jobReason))
		ciphertext = ""
	}

	job := &models.MonitorOfferJob{
		UserID: in.UserID, AutomationID: in.AutomationID, Mode: in.Mode,
		SourceGroupID: in.SourceGroupID, MessageID: in.MessageID,
		MessageTimestamp: in.MessageTime,
		PayloadExpiresAt: time.Now().UTC().Add(models.PayloadTTL),
		ReasonCode:       jobReason, Status: models.MonitorJobActive,
	}
	if ciphertext != "" {
		job.PayloadCiphertext = &ciphertext
	}

	items := make([]models.MonitorOfferItem, 0, len(plan.Items))
	for i := range plan.Items {
		items = append(items, itemRow(plan.Items[i], ciphertext != ""))
	}

	created, err := c.repo.CreateJobWithItems(ctx, job, items)
	if err != nil {
		// Fail-open: erro de banco no ledger não pode segurar o envio. O custo
		// é perder a idempotência DESTA mensagem, não perder a mensagem.
		c.logger.Error("monitoroffer: falha ao gravar o ledger, seguindo sem ele",
			zap.String("user_id", in.UserID), zap.Error(err))
		return "", true
	}
	if !created {
		c.logger.Info("monitoroffer: mensagem já processada, nada a enviar",
			zap.String("user_id", in.UserID), zap.String("mode", in.Mode))
		return "", false
	}

	c.schedulePending(ctx, items)
	return job.ID, true
}

// itemRow traduz o resultado do planner numa linha do ledger.
func itemRow(it ItemResult, hasPayload bool) models.MonitorOfferItem {
	row := models.MonitorOfferItem{
		Ordinal:          it.Slot.Ordinal,
		ParagraphOrdinal: it.Slot.ParagraphOrdinal,
		SourceLinkHash:   commerceurl.SourceHash(it.Slot.Raw),
		Host:             it.Slot.URL.Host,
		// Coagido: o CHECK da migração 079 só aceita shopee/mercadolivre/
		// unsupported, e StoreOf hoje devolve também amazon/aliexpress/awin.
		Store:      models.LedgerStore(it.Slot.URL.Store),
		URLClass:   it.Class,
		ErrorClass: it.ErrorClass,
		ReasonCode: it.Reason,
		Outcome:    it.Outcome,
		Status:     it.Status,
	}
	if row.Host == "" {
		// Bloqueio de segurança pode não ter host parseado; a coluna é NOT NULL
		// e o hash já identifica o link.
		row.Host = "-"
	}
	if row.Store == "" {
		row.Store = models.MonitorStoreUnsupported
	}
	if it.CanonicalKey != "" {
		k := it.CanonicalKey
		row.CanonicalKey = &k
	}
	if it.OutputKey != "" {
		ok, kind := it.OutputKey, it.OutputKind
		row.OutputKey, row.OutputKind = &ok, &kind
	}

	if row.Status == models.MonitorItemPending {
		if !hasPayload {
			// Sem payload o retry não conseguiria reclassificar: encerra agora
			// em vez de deixar um pending que nunca vai a lugar nenhum.
			row.Status = models.MonitorItemBlocked
			row.ErrorClass = models.ErrorClassPermanentConfig
			row.ReasonCode = models.ReasonPayloadTooLarge
		} else {
			at := time.Now().UTC().Add(models.NextAttemptDelay(row.SourceLinkHash, 0))
			row.NextAttemptAt = &at
		}
	}
	return row
}

// schedulePending enfileira a task dos itens pendentes.
//
// Erro de enqueue é logado e engolido (§4.16): o sweep de 1 minuto pega o item
// de qualquer forma, então o caminho quente não precisa saber disso.
func (c *Coordinator) schedulePending(ctx context.Context, items []models.MonitorOfferItem) {
	if c.enqueuer == nil {
		return
	}
	for i := range items {
		it := items[i]
		if it.Status != models.MonitorItemPending || it.NextAttemptAt == nil || it.ID == "" {
			continue
		}
		if err := c.enqueuer.EnqueueMonitorOfferRetry(ctx, it.ID, it.EnqueueGeneration, *it.NextAttemptAt); err != nil {
			c.logger.Warn("monitoroffer: enqueue do retry falhou, o sweep recupera",
				zap.String("item_id", it.ID), zap.Error(err))
		}
	}
}

// PrepareDelivery cria a linha de entrega em 'prepared'.
func (c *Coordinator) PrepareDelivery(ctx context.Context, jobID, outputKey, outputKind, destGroupID string) {
	if c == nil || c.repo == nil || jobID == "" {
		return
	}
	if err := c.repo.UpsertDelivery(ctx, models.MonitorOfferDelivery{
		JobID: jobID, OutputKey: outputKey, OutputKind: outputKind,
		DestinationGroupID: destGroupID, ReasonCode: models.ReasonNone,
	}); err != nil {
		c.logger.Warn("monitoroffer: upsert de delivery falhou",
			zap.String("job_id", jobID), zap.Error(err))
	}
}

// BeginSend é o CAS imediatamente antes do POST na WAHA.
//
// Devolve false quando o destino já foi entregue ou está ambíguo — e nesse caso
// o chamador NÃO pode enviar. É a única barreira que impede o retry de duplicar
// uma oferta que já saiu.
//
// Com o ledger desligado devolve true: o comportamento histórico não tinha essa
// barreira e não pode passar a bloquear envio.
func (c *Coordinator) BeginSend(ctx context.Context, jobID, outputKey, destGroupID, token string) bool {
	if c == nil || c.repo == nil || jobID == "" {
		return true
	}
	ok, err := c.repo.MarkDeliverySending(ctx, jobID, outputKey, destGroupID, token)
	if err != nil {
		// Fail-open consciente: um erro de banco aqui não pode bloquear o
		// envio. O risco assumido é duplicata numa janela de falha de banco,
		// contra a certeza de parar o espelho inteiro se o Postgres soluçar.
		c.logger.Warn("monitoroffer: CAS de envio falhou, seguindo",
			zap.String("job_id", jobID), zap.Error(err))
		return true
	}
	return ok
}

// FinishSend registra o desfecho do envio.
func (c *Coordinator) FinishSend(ctx context.Context, jobID, outputKey, destGroupID, token, status, reason string, providerMessageID *string) {
	if c == nil || c.repo == nil || jobID == "" {
		return
	}
	if err := c.repo.FinishDelivery(ctx, jobID, outputKey, destGroupID, token, status, reason, providerMessageID); err != nil {
		c.logger.Warn("monitoroffer: registro do desfecho falhou",
			zap.String("job_id", jobID), zap.String("status", status), zap.Error(err))
	}
}

// Sweep é o ciclo de manutenção do cron (1 min).
//
// Ordem importa: reaper ANTES de reivindicar, para que um item liberado pelo
// reaper já entre nesta mesma passagem em vez de esperar o próximo minuto.
func (c *Coordinator) Sweep(ctx context.Context, limit int) {
	if c == nil || c.repo == nil {
		return
	}

	if st, err := c.repo.ReapStaleLeases(ctx); err != nil {
		c.logger.Warn("monitoroffer: reaper falhou", zap.Error(err))
	} else if st.ItemsReleased+st.DeliveriesAmbiguous+st.DeliveriesRejected > 0 {
		c.logger.Info("monitoroffer: leases recuperadas",
			zap.Int("itens", st.ItemsReleased),
			zap.Int("deliveries_ambiguas", st.DeliveriesAmbiguous),
			zap.Int("deliveries_recusadas", st.DeliveriesRejected))
	}

	if n, err := c.repo.ExpirePayloads(ctx); err != nil {
		c.logger.Warn("monitoroffer: expiração de payload falhou", zap.Error(err))
	} else if n > 0 {
		c.logger.Info("monitoroffer: payloads expirados", zap.Int("itens", n))
	}

	leases, err := c.repo.ClaimDueItems(ctx, limit)
	if err != nil {
		c.logger.Warn("monitoroffer: claim de itens falhou", zap.Error(err))
		return
	}
	for _, l := range leases {
		if c.enqueuer == nil {
			continue
		}
		if err := c.enqueuer.EnqueueMonitorOfferRetry(ctx, l.ItemID, l.Generation, l.RunAt); err != nil {
			c.logger.Warn("monitoroffer: enqueue no sweep falhou",
				zap.String("item_id", l.ItemID), zap.Error(err))
		}
	}
}

// Cleanup apaga metadata terminal. Roda SEMPRE após a migração, inclusive com
// os gates off, para remover linhas de um rollout anterior.
func (c *Coordinator) Cleanup(ctx context.Context) {
	if c == nil || c.repo == nil {
		return
	}
	n, err := c.repo.DeleteTerminalOlderThan(ctx, models.TerminalMetadataTTL, 500)
	if err != nil {
		c.logger.Warn("monitoroffer: cleanup falhou", zap.Error(err))
		return
	}
	if n > 0 {
		c.logger.Info("monitoroffer: metadata terminal apagada", zap.Int("jobs", n))
	}
}
