package groupmembers

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"automatiza-go/internal/adapters/whatsapp"
	appconfig "automatiza-go/internal/config"
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"

	"go.uber.org/zap"
)

// EventTypeParticipants é o ÚNICO evento que gera fato de membro. Os legados
// group.join/group.leave continuam assinados durante a transição (o contrato
// os marca deprecated, não removidos), mas são no-op aqui — por isso a
// coexistência não duplica contagem.
const EventTypeParticipants = "group.v2.participants"

// Dimensionamento da reconciliação. O executor do cron impõe timeout FIXO de
// 5 minutos por execução (cron_service.go), então o lote foi dimensionado para
// caber nele: 250 grupos / 10 em paralelo × 10s de teto por chamada ≈ 4min no
// pior caso absoluto, e ordens de magnitude menos no caso real.
const (
	// reconcileBatchSize dimensionado pelo throughput MEDIDO em produção
	// (23/07/2026): 250 grupos levaram 40s com 10 chamadas concorrentes, ou
	// ~0,16s/grupo. A 1000, a rodada leva ~160s — folga confortável dentro do
	// teto FIXO de 5min que o cron impõe (cron_service.go), e o excedente é
	// seguro de qualquer forma: o cancelamento do contexto libera os claims em
	// voo em vez de travá-los até o lease.
	//
	// A 250/h os 28k grupos levariam ~4,7 dias para ter dado na tela; a 1000/h,
	// ~28h. O custo é ~160s/hora de 10 leituras concorrentes no WAHA — ordem de
	// grandeza abaixo do tráfego de envio.
	reconcileBatchSize   = 1000
	reconcileConcurrency = 10
	reconcileCallTimeout = 10 * time.Second
	// reconcileLease: um claim mais velho que isto volta a ser elegível, o que
	// impede que um processo morto trave grupos para sempre.
	reconcileLease = 15 * time.Minute
	// claimReleaseTimeout: liberar o claim usa contexto DESACOPLADO do ctx da
	// rodada — se o cron estourou o timeout, o claim ainda precisa sair.
	claimReleaseTimeout = 5 * time.Second
)

// Retenção. A PROJEÇÃO não tem retenção (é estado atual, não histórico).
const (
	eventsRetention    = 180 * 24 * time.Hour
	snapshotsRetention = 400 * 24 * time.Hour
	cleanupBatchSize   = 10_000
	cleanupMaxBatches  = 20
)

// ParticipantFetcher lê a fotografia atual de participantes de um grupo.
// Interface LOCAL de propósito: o contrato global do adapter não muda, e a
// variante "ForTracking" não resolve LID (uma consulta global de mappings por
// grupo seria desperdício — a identidade não depende do telefone).
type ParticipantFetcher interface {
	GetGroupParticipantsForTracking(ctx context.Context, sessionName, groupID string) ([]whatsapp.TrackingParticipant, error)
}

// Service implementa a Fase A: ingestão de fatos, reconciliação e retenção.
type Service struct {
	repo    repository.GroupMemberRepository
	fetcher ParticipantFetcher
	secret  []byte
	logger  *zap.Logger
	// now é injetável para que os testes provem ordem/janelas sem time.Sleep.
	now func() time.Time
	// noop: flag ligada com segredo ausente/curto. Nasce inerte em vez de
	// derrubar o processo — tracking histórico não pode matar o caminho quente.
	noop bool
}

// NewService constrói o serviço. NUNCA devolve o segredo (nem tamanho, nem
// prefixo, nem hash) em erro ou log: só a constatação de que é inválido.
func NewService(
	repo repository.GroupMemberRepository,
	fetcher ParticipantFetcher,
	secret string,
	logger *zap.Logger,
) *Service {
	s := &Service{
		repo:    repo,
		fetcher: fetcher,
		secret:  []byte(secret),
		logger:  logger,
		now:     time.Now,
	}

	if appconfig.GroupMemberTrackingEnabled() && len(s.secret) < minSecretLen {
		s.noop = true
		// Um único erro de inicialização — não por evento.
		logger.Error("group member tracking habilitado sem GROUP_MEMBER_HASH_SECRET válido; serviço inerte",
			zap.Int("min_bytes", minSecretLen))
	}

	return s
}

// SetClock troca o relógio do serviço. Uso restrito a testes.
func (s *Service) SetClock(now func() time.Time) { s.now = now }

// disabled reporta se o serviço deve ser inerte: zero escrita, zero WAHA.
func (s *Service) disabled() bool {
	return s.noop || !appconfig.GroupMemberTrackingEnabled()
}

// =============================================================================
// Ingestão de webhook
// =============================================================================

// ProcessGroupEvent implementa workers.GroupEventWebhookProcessor.
//
// Retorna erro APENAS quando vale retentar (tenant ainda inexistente, falha de
// banco). Payload malformado, evento legado e tenant ambíguo devolvem nil: o
// retry não os consertaria e só empurraria o evento para a DLQ.
func (s *Service) ProcessGroupEvent(
	ctx context.Context,
	sessionName, eventType, providerEventID string,
	providerTimestamp int64,
	payload map[string]interface{},
) error {
	if s.disabled() {
		return nil
	}

	// Só o evento v2 é fonte de fatos. group.join/group.leave/group.v2.join/
	// group.v2.leave/group.v2.update e qualquer outro group.* saem aqui.
	if !strings.EqualFold(eventType, EventTypeParticipants) {
		return nil
	}

	eventID, err := ValidateProviderEventID(providerEventID)
	if err != nil {
		s.logMalformed(sessionName, eventType, err)
		return nil
	}

	providerCreatedAt, err := ParseEnvelopeTimestamp(providerTimestamp)
	if err != nil {
		s.logMalformed(sessionName, eventType, err)
		return nil
	}

	parsed, err := parsePayload(payload)
	if err != nil {
		s.logMalformed(sessionName, eventType, err)
		return nil
	}

	tenant, err := s.repo.ResolveWebhookTenant(ctx, sessionName)
	if err != nil {
		if errors.Is(err, repository.ErrAmbiguousTrackingTenant) {
			// Dois usuários com o mesmo nome de sessão: escrever escolheria um
			// tenant no escuro. Não escrevemos nada.
			s.logger.Warn("group member tracking: sessão ambígua entre tenants, evento descartado",
				zap.String("session", sessionName),
				zap.String("provider_event_id", eventID),
				zap.String("result", "tenant_ambiguous"))
			return nil
		}
		if errors.Is(err, repository.ErrUnknownTrackingTenant) {
			// Sessão sem linha em `instances` (órfã): não existe tenant para
			// descobrir num retry. Retentar aqui só queima slot do worker e
			// enche o DLQ — a WAHA entrega esses eventos indefinidamente.
			s.logger.Warn("group member tracking: sessão sem instância, evento descartado",
				zap.String("session", sessionName),
				zap.String("provider_event_id", eventID),
				zap.String("result", "tenant_unknown"))
			return nil
		}
		// Erro de banco (pool/timeout): genuinamente transitório, o worker retenta.
		return err
	}

	// Pseudonimização + dedupe dentro do próprio envelope: o mesmo participante
	// repetido no array vira UMA identidade (e uma linha de ledger).
	participants := make([]models.GroupMemberEventParticipant, 0, len(parsed.participants))
	seen := make(map[string]struct{}, len(parsed.participants))
	skipped := 0
	for _, raw := range parsed.participants {
		identity, err := BuildIdentity(s.secret, tenant.UserID, raw.id, raw.pn)
		if err != nil {
			skipped++
			continue
		}
		if _, dup := seen[identity.ParticipantHash]; dup {
			skipped++
			continue
		}
		seen[identity.ParticipantHash] = struct{}{}
		participants = append(participants, models.GroupMemberEventParticipant{
			GroupMemberIdentity: identity,
			Role:                raw.role,
		})
	}

	if len(participants) == 0 {
		s.logger.Warn("group member tracking: envelope sem participante válido",
			zap.String("session", sessionName),
			zap.String("provider_event_id", eventID),
			zap.Int("skipped", skipped),
			zap.String("result", "malformed"))
		return nil
	}

	event := models.GroupMemberWebhookEvent{
		ProviderEventID:   eventID,
		ProviderCreatedAt: providerCreatedAt,
		WAGroupID:         parsed.groupID,
		EventType:         parsed.eventType,
		OccurredAt:        parsed.occurredAt,
		AppliedAt:         s.now().UTC(),
		Participants:      participants,
	}

	result, err := s.repo.RecordWebhookBatch(ctx, *tenant, event)
	if err != nil {
		return fmt.Errorf("record group member batch: %w", err)
	}

	outcome := "processed"
	if result.Inserted == 0 {
		outcome = "duplicate"
	}
	s.logger.Info("group member tracking: envelope aplicado",
		zap.String("provider_event_id", eventID),
		zap.String("user_id", tenant.UserID),
		zap.String("wa_group_id", parsed.groupID),
		zap.String("event_type", parsed.eventType),
		zap.Int("participants_total", len(parsed.participants)),
		zap.Int("inserted", result.Inserted),
		zap.Int("duplicates", result.Duplicates),
		zap.Int("skipped", skipped),
		zap.String("result", outcome))

	return nil
}

// logMalformed registra rejeição sem retry. Nunca inclui payload nem
// identificador de participante — só a classe do erro.
func (s *Service) logMalformed(sessionName, eventType string, err error) {
	s.logger.Warn("group member tracking: envelope rejeitado",
		zap.String("session", sessionName),
		zap.String("event_type", eventType),
		zap.String("reason", err.Error()),
		zap.String("result", "malformed"))
}

// =============================================================================
// Parse do payload (contrato WAHA GroupV2ParticipantsEvent)
// =============================================================================

type rawParticipant struct {
	id   string
	pn   string
	role string
}

type parsedPayload struct {
	eventType    string
	occurredAt   time.Time
	groupID      string
	participants []rawParticipant
}

// parsePayload valida o payload contra o contrato oficial. `_data` é exigido e
// validado como objeto (é `required` no schema) e DESCARTADO — nunca é
// persistido nem logado.
func parsePayload(payload map[string]interface{}) (parsedPayload, error) {
	var out parsedPayload
	if payload == nil {
		return out, ErrInvalidPayloadStructure
	}

	eventType, _ := payload["type"].(string)
	if !isValidEventType(eventType) {
		return out, ErrInvalidEventType
	}
	out.eventType = eventType

	tsRaw, ok := payload["timestamp"].(float64)
	if !ok {
		return out, ErrInvalidPayloadTime
	}
	occurredAt, err := ParsePayloadTimestamp(tsRaw)
	if err != nil {
		return out, err
	}
	out.occurredAt = occurredAt

	group, ok := payload["group"].(map[string]interface{})
	if !ok {
		return out, ErrInvalidGroupID
	}
	groupID, _ := group["id"].(string)
	if groupID == "" {
		return out, ErrInvalidGroupID
	}
	out.groupID = groupID

	if _, ok := payload["_data"].(map[string]interface{}); !ok {
		return out, ErrInvalidPayloadStructure
	}

	list, ok := payload["participants"].([]interface{})
	if !ok {
		return out, ErrInvalidPayloadStructure
	}
	out.participants = make([]rawParticipant, 0, len(list))
	for _, item := range list {
		p, ok := item.(map[string]interface{})
		if !ok {
			// Item não-objeto: contado como skipped pelo chamador via id vazio.
			out.participants = append(out.participants, rawParticipant{})
			continue
		}
		id, _ := p["id"].(string)
		pn, _ := p["pn"].(string)
		role, _ := p["role"].(string)
		if !isValidRole(role) {
			// Role fora do enum invalida o participante, não o envelope.
			out.participants = append(out.participants, rawParticipant{})
			continue
		}
		out.participants = append(out.participants, rawParticipant{id: id, pn: pn, role: role})
	}

	return out, nil
}

// =============================================================================
// Reconciliação
// =============================================================================

// ReconcileMemberships roda uma rodada de reconciliação: reivindica um lote de
// grupos, lê a fotografia atual de cada um no WAHA e aplica.
//
// Falha de um grupo não cancela os demais; o erro agregado aparece no status do
// cron sem perder os sucessos.
func (s *Service) ReconcileMemberships(ctx context.Context) error {
	if s.disabled() {
		return nil
	}
	if s.fetcher == nil {
		s.logger.Warn("group member tracking: reconciliação sem provedor WAHA configurado")
		return nil
	}

	start := s.now()
	candidates, err := s.repo.ClaimGroupsForReconciliation(ctx, reconcileBatchSize, reconcileLease)
	if err != nil {
		return fmt.Errorf("claim groups for reconciliation: %w", err)
	}
	if len(candidates) == 0 {
		return nil
	}

	var (
		wg  sync.WaitGroup
		sem = make(chan struct{}, reconcileConcurrency)
		mu  sync.Mutex
		// failures conta por CLASSE de erro, não guarda os erros inteiros: o
		// corpo de um 500 do WAHA traz stack trace JS completa, e um
		// errors.Join de 27 deles vira uma linha de ~20KB no log E um evento
		// gigante no Sentry, toda hora. Classe + contagem diz a mesma coisa.
		failures     = map[string]int{}
		reconciled   int
		failed       int
		participants int
	)

	for i := range candidates {
		candidate := candidates[i]

		wg.Add(1)
		go func() {
			defer wg.Done()

			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				// Rodada cancelada antes de começar este grupo: devolve o claim
				// para a próxima rodada não esperar o lease inteiro.
				s.releaseClaim(ctx, candidate)
				return
			}

			n, err := s.reconcileOne(ctx, candidate)

			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				failed++
				failures[errorClass(err)]++
				return
			}
			reconciled++
			participants += n
		}()
	}
	wg.Wait()

	s.logger.Info("group member tracking: rodada de reconciliação",
		zap.Int("claimed", len(candidates)),
		zap.Int("reconciled", reconciled),
		zap.Int("failed", failed),
		zap.Int("participants", participants),
		zap.Duration("duration", s.now().Sub(start)))

	if len(failures) > 0 {
		return fmt.Errorf("reconciliação de membros: %d de %d grupos falharam (%s)",
			failed, len(candidates), formatFailureClasses(failures))
	}
	return nil
}

// errorClass reduz um erro à sua CLASSE, descartando o corpo do provedor.
// Um 500 do WAHA carrega o JSON inteiro com stack trace JS; agregar 27 desses
// produzia ~20KB por rodada em log e Sentry, toda hora, sem informação nova.
func errorClass(err error) string {
	s := err.Error()
	// O corpo do provedor começa no primeiro '{' — tudo dali pra frente é ruído.
	if i := strings.IndexByte(s, '{'); i >= 0 {
		s = s[:i]
	}
	// Identificadores de grupo/sessão viram ruído na agregação por classe.
	if i := strings.IndexByte(s, '('); i >= 0 {
		if j := strings.IndexByte(s[i:], ')'); j >= 0 {
			s = s[:i] + s[i+j+1:]
		}
	}
	s = strings.TrimSpace(strings.Trim(strings.TrimSpace(s), ":"))
	if len(s) > 120 {
		s = s[:120]
	}
	return s
}

// formatFailureClasses rende "classe xN; classe xM", limitado às 3 mais comuns.
func formatFailureClasses(classes map[string]int) string {
	type kv struct {
		class string
		n     int
	}
	all := make([]kv, 0, len(classes))
	for c, n := range classes {
		all = append(all, kv{c, n})
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].n != all[j].n {
			return all[i].n > all[j].n
		}
		return all[i].class < all[j].class
	})
	if len(all) > 3 {
		all = all[:3]
	}
	parts := make([]string, 0, len(all))
	for _, e := range all {
		parts = append(parts, fmt.Sprintf("%s x%d", e.class, e.n))
	}
	return strings.Join(parts, "; ")
}

// reconcileOne reconcilia um grupo. Devolve a quantidade de participantes
// aplicados.
func (s *Service) reconcileOne(ctx context.Context, c models.GroupReconciliationCandidate) (int, error) {
	// observation_started_at é capturado ANTES do HTTP: qualquer webhook
	// aplicado a partir daqui é mais novo que esta fotografia e vence dela.
	observationStartedAt := s.now().UTC()

	callCtx, cancel := context.WithTimeout(ctx, reconcileCallTimeout)
	raw, err := s.fetcher.GetGroupParticipantsForTracking(callCtx, c.SessionName, c.WAGroupID)
	cancel()
	if err != nil {
		s.releaseClaim(ctx, c)
		return 0, fmt.Errorf("fetch participants (group=%s session=%s): %w", c.GroupID, c.SessionName, err)
	}

	members := make([]models.GroupReconciliationMember, 0, len(raw))
	for _, p := range raw {
		if !isValidRole(p.Role) {
			// Lista parcial NÃO pode ser aplicada: os ausentes seriam
			// desativados, apagando membros legítimos. Aborta só este grupo.
			s.releaseClaim(ctx, c)
			return 0, fmt.Errorf("invalid participant role in reconciliation (group=%s)", c.GroupID)
		}
		identity, idErr := BuildIdentity(s.secret, c.UserID, p.ID, p.PN)
		if idErr != nil {
			s.releaseClaim(ctx, c)
			return 0, fmt.Errorf("invalid participant id in reconciliation (group=%s)", c.GroupID)
		}
		members = append(members, models.GroupReconciliationMember{
			GroupMemberIdentity: identity,
			Role:                p.Role,
		})
	}

	if err := s.repo.ReconcileGroup(ctx, c, members, observationStartedAt); err != nil {
		s.releaseClaim(ctx, c)
		return 0, fmt.Errorf("reconcile group %s: %w", c.GroupID, err)
	}
	return len(members), nil
}

// releaseClaim devolve o grupo à fila. Usa contexto desacoplado: o claim
// precisa sair mesmo quando o ctx da rodada já morreu.
func (s *Service) releaseClaim(ctx context.Context, c models.GroupReconciliationCandidate) {
	relCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), claimReleaseTimeout)
	defer cancel()

	if err := s.repo.ReleaseReconciliationClaim(relCtx, c.GroupID); err != nil {
		s.logger.Warn("group member tracking: falha ao liberar claim de reconciliação",
			zap.String("group_id", c.GroupID),
			zap.String("session", c.SessionName),
			zap.Error(err))
	}
}

// =============================================================================
// Retenção
// =============================================================================

// CleanupTracking apaga ledger e snapshots fora da janela de retenção.
func (s *Service) CleanupTracking(ctx context.Context) error {
	if s.disabled() {
		return nil
	}

	start := s.now()
	now := s.now().UTC()

	events, err := s.repo.CleanupOldGroupMemberEvents(
		ctx, now.Add(-eventsRetention), cleanupBatchSize, cleanupMaxBatches)
	if err != nil {
		return fmt.Errorf("cleanup group member events: %w", err)
	}

	snapshots, err := s.repo.CleanupOldGroupMemberSnapshots(
		ctx, now.Add(-snapshotsRetention), cleanupBatchSize, cleanupMaxBatches)
	if err != nil {
		return fmt.Errorf("cleanup group member snapshots: %w", err)
	}

	if events > 0 || snapshots > 0 {
		s.logger.Info("group member tracking: retenção aplicada",
			zap.Int64("events_deleted", events),
			zap.Int64("snapshots_deleted", snapshots),
			zap.Duration("duration", s.now().Sub(start)))
	}
	return nil
}
