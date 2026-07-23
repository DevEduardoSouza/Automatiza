package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

// snapshotTZ é o timezone do bucket diário. Fixo em America/Sao_Paulo: o
// produto é brasileiro e "quantos entraram hoje" precisa bater com o dia do
// usuário, não com UTC.
const snapshotTZ = "America/Sao_Paulo"

// groupMemberRepository implementa repository.GroupMemberRepository.
type groupMemberRepository struct {
	db     *DB
	logger *zap.Logger
}

// NewGroupMemberRepository cria o repositório de tracking de membros.
func NewGroupMemberRepository(db *DB, logger *zap.Logger) repository.GroupMemberRepository {
	return &groupMemberRepository{db: db, logger: logger}
}

// =============================================================================
// Resolução de tenant
// =============================================================================

// ResolveWebhookTenant resolve o nome da sessão para no máximo uma instância.
//
// Não usamos InstanceRepository.GetInstanceByName: ele faz LIMIT implícito de 1
// linha sobre uma coluna que NÃO é única globalmente (a tabela só garante
// UNIQUE(user_id, instance_id)), então um nome repetido entre dois usuários
// atribuiria silenciosamente o webhook ao tenant errado. Aqui lemos duas linhas
// de propósito para poder distinguir "ainda não existe" de "ambíguo".
func (r *groupMemberRepository) ResolveWebhookTenant(ctx context.Context, sessionName string) (*models.TrackingTenant, error) {
	const query = `
		SELECT user_id, id, instance_id
		FROM instances
		WHERE instance_id = $1
		ORDER BY id
		LIMIT 2`

	rows, err := r.db.Query(ctx, query, sessionName)
	if err != nil {
		// Transitório por natureza (pool/timeout): o worker retenta.
		return nil, fmt.Errorf("resolve tracking tenant: %w", err)
	}
	defer rows.Close()

	var found []models.TrackingTenant
	for rows.Next() {
		var t models.TrackingTenant
		if err := rows.Scan(&t.UserID, &t.InstancePK, &t.SessionName); err != nil {
			return nil, fmt.Errorf("resolve tracking tenant scan: %w", err)
		}
		found = append(found, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("resolve tracking tenant rows: %w", err)
	}

	switch len(found) {
	case 0:
		// "unavailable" é substring retryable no matcher do worker: a
		// instância pode estar sendo criada agora (webhook chega antes do
		// commit da linha).
		return nil, errors.New("tracking tenant unavailable")
	case 1:
		return &found[0], nil
	default:
		return nil, repository.ErrAmbiguousTrackingTenant
	}
}

// =============================================================================
// Ingestão de webhook — unidade transacional única
// =============================================================================

// participantColumns são os arrays paralelos passados por UNNEST. Manter um
// array por coluna evita N statements (um por participante) no caminho quente.
type participantColumns struct {
	hashes    []string
	phoneHash []*string
	idTypes   []string
	ddds      []*string
	roles     []string
}

func toParticipantColumns(ps []models.GroupMemberEventParticipant) participantColumns {
	c := participantColumns{
		hashes:    make([]string, 0, len(ps)),
		phoneHash: make([]*string, 0, len(ps)),
		idTypes:   make([]string, 0, len(ps)),
		ddds:      make([]*string, 0, len(ps)),
		roles:     make([]string, 0, len(ps)),
	}
	for i := range ps {
		c.hashes = append(c.hashes, ps[i].ParticipantHash)
		c.phoneHash = append(c.phoneHash, ps[i].PhoneHash)
		c.idTypes = append(c.idTypes, ps[i].IDType)
		c.ddds = append(c.ddds, ps[i].DDD)
		c.roles = append(c.roles, ps[i].Role)
	}
	return c
}

// RecordWebhookBatch aplica um envelope inteiro numa única transação.
//
// Sequência (seção 6.1 do plano): advisory lock por (tenant, grupo) → resolve
// group_id → insere o lote com ON CONFLICT DO NOTHING RETURNING → projeta SÓ as
// linhas retornadas → incrementa o agregado diário → atualiza a observação de
// membros quando já existe baseline. Qualquer erro desfaz tudo; o retry
// reaplica o mesmo envelope e a unique constraint o torna no-op.
func (r *groupMemberRepository) RecordWebhookBatch(
	ctx context.Context,
	tenant models.TrackingTenant,
	event models.GroupMemberWebhookEvent,
) (models.RecordBatchResult, error) {
	result := models.RecordBatchResult{}
	if len(event.Participants) == 0 {
		return result, nil
	}

	err := r.db.WithTx(ctx, func(tx pgx.Tx) error {
		// 1. Serializa envelopes do mesmo (tenant, grupo). Grupos distintos
		//    seguem concorrentes. Lock de transação: liberado no commit.
		if err := lockTenantGroup(ctx, tx, tenant.UserID, event.WAGroupID); err != nil {
			return err
		}

		// 2. group_id resolvido AQUI dentro (nunca vem calculado de fora).
		groupID, err := resolveGroupID(ctx, tx, tenant.UserID, event.WAGroupID)
		if err != nil {
			return err
		}

		cols := toParticipantColumns(event.Participants)

		// 3. Insert idempotente do lote.
		const insertLedger = `
			INSERT INTO group_member_events (
				user_id, group_id, wa_group_id, instance_id,
				provider_event_id, provider_created_at,
				participant_hash, phone_hash, hash_version, id_type, ddd,
				event_type, role, occurred_at, applied_at)
			SELECT $1, $2, $3, $4, $5, $6,
			       p.participant_hash, p.phone_hash, 1, p.id_type, p.ddd,
			       $7, p.role, $8, $9
			FROM UNNEST($10::text[], $11::text[], $12::text[], $13::text[], $14::text[])
			     AS p(participant_hash, phone_hash, id_type, ddd, role)
			ON CONFLICT (user_id, provider_event_id, participant_hash) DO NOTHING
			RETURNING participant_hash, phone_hash, id_type, ddd, role`

		rows, err := tx.Query(ctx, insertLedger,
			tenant.UserID, groupID, event.WAGroupID, tenant.InstancePK,
			event.ProviderEventID, event.ProviderCreatedAt,
			event.EventType, event.OccurredAt, event.AppliedAt,
			cols.hashes, cols.phoneHash, cols.idTypes, cols.ddds, cols.roles,
		)
		if err != nil {
			return fmt.Errorf("insert group member events: %w", err)
		}

		var inserted participantColumns
		for rows.Next() {
			var h, idType, role string
			var ph, ddd *string
			if err := rows.Scan(&h, &ph, &idType, &ddd, &role); err != nil {
				rows.Close()
				return fmt.Errorf("scan inserted group member events: %w", err)
			}
			inserted.hashes = append(inserted.hashes, h)
			inserted.phoneHash = append(inserted.phoneHash, ph)
			inserted.idTypes = append(inserted.idTypes, idType)
			inserted.ddds = append(inserted.ddds, ddd)
			inserted.roles = append(inserted.roles, role)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return fmt.Errorf("insert group member events rows: %w", err)
		}

		result.Inserted = len(inserted.hashes)
		result.Duplicates = len(event.Participants) - result.Inserted

		// Replay puro: nada novo, nada a projetar nem a contar.
		if result.Inserted == 0 {
			return nil
		}

		// 4. Projeção — só as linhas efetivamente inseridas.
		if err := applyProjection(ctx, tx, tenant, event, groupID, inserted); err != nil {
			return err
		}

		// 5. Agregado diário: join/leave contam; promote/demote não movimentam
		//    membros, então não entram na contagem de entradas/saídas.
		if event.EventType == models.GroupMemberEventJoin || event.EventType == models.GroupMemberEventLeave {
			if err := bumpDailyCounters(ctx, tx, tenant.UserID, groupID, event, result.Inserted); err != nil {
				return err
			}
		}

		// 6. Observação de membros (só depois do baseline).
		return refreshMembersObservation(ctx, tx, tenant.UserID, groupID, event.WAGroupID, event.AppliedAt)
	})

	if err != nil {
		return models.RecordBatchResult{}, err
	}
	return result, nil
}

// lockTenantGroup adquire o advisory lock transacional que serializa TODAS as
// mutações de um (tenant, grupo) — webhook e reconciliação usam o mesmo.
func lockTenantGroup(ctx context.Context, tx pgx.Tx, userID, waGroupID string) error {
	const q = `SELECT pg_advisory_xact_lock(hashtextextended($1 || ':' || $2, 0))`
	if _, err := tx.Exec(ctx, q, userID, waGroupID); err != nil {
		return fmt.Errorf("acquire group member lock: %w", err)
	}
	return nil
}

// resolveGroupID devolve groups.id do par (user_id, wa_group_id), ou nil quando
// o grupo ainda não foi sincronizado — o fato continua válido sem ele.
func resolveGroupID(ctx context.Context, tx pgx.Tx, userID, waGroupID string) (*string, error) {
	const q = `SELECT id::text FROM groups WHERE user_id = $1 AND wa_group_id = $2`
	var id string
	err := tx.QueryRow(ctx, q, userID, waGroupID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("resolve group id: %w", err)
	}
	return &id, nil
}

// projectionOrderGuard é a ordem total aplicada à projeção. Um evento só
// avança o estado quando sua tupla é ESTRITAMENTE maior que a última aplicada;
// evento atrasado entra no ledger mas não retrocede a projeção. Os COALESCE
// dão sentinela para a linha de baseline (last_event_at NULL), que perde de
// qualquer evento real. O provider_event_id desempata timestamps idênticos, o
// que torna a ordem determinística mesmo com dois fatos no mesmo milissegundo.
const projectionOrderGuard = `
	(COALESCE(group_memberships.last_event_at, '-infinity'::timestamptz),
	 COALESCE(group_memberships.last_provider_created_at, '-infinity'::timestamptz),
	 COALESCE(group_memberships.last_provider_event_id, ''))
	<
	(EXCLUDED.last_event_at, EXCLUDED.last_provider_created_at, EXCLUDED.last_provider_event_id)`

// applyProjection aplica os fatos inseridos ao estado atual.
//
// join/leave fazem upsert (criam a projeção se ausente); promote/demote são
// UPDATE puro: mudam papel e metadados de ordem, nunca criam membro nem mexem
// em is_active/joined_at/left_at.
func applyProjection(
	ctx context.Context,
	tx pgx.Tx,
	tenant models.TrackingTenant,
	event models.GroupMemberWebhookEvent,
	groupID *string,
	p participantColumns,
) error {
	if event.EventType == models.GroupMemberEventPromote || event.EventType == models.GroupMemberEventDemote {
		const updateRole = `
			UPDATE group_memberships m
			SET role = p.role,
			    phone_hash = COALESCE(m.phone_hash, p.phone_hash),
			    ddd = COALESCE(m.ddd, p.ddd),
			    group_id = COALESCE($3::uuid, m.group_id),
			    last_event_at = $4,
			    last_provider_created_at = $5,
			    last_provider_event_id = $6,
			    last_event_type = $7,
			    last_event_applied_at = $8,
			    updated_at = now()
			FROM UNNEST($9::text[], $10::text[], $11::text[], $12::text[])
			     AS p(participant_hash, phone_hash, ddd, role)
			WHERE m.user_id = $1
			  AND m.wa_group_id = $2
			  AND m.participant_hash = p.participant_hash
			  AND (COALESCE(m.last_event_at, '-infinity'::timestamptz),
			       COALESCE(m.last_provider_created_at, '-infinity'::timestamptz),
			       COALESCE(m.last_provider_event_id, '')) < ($4, $5, $6)`

		if _, err := tx.Exec(ctx, updateRole,
			tenant.UserID, event.WAGroupID, groupID,
			event.OccurredAt, event.ProviderCreatedAt, event.ProviderEventID,
			event.EventType, event.AppliedAt,
			p.hashes, p.phoneHash, p.ddds, p.roles,
		); err != nil {
			return fmt.Errorf("update group memberships role: %w", err)
		}
		return nil
	}

	isJoin := event.EventType == models.GroupMemberEventJoin

	const upsert = `
		INSERT INTO group_memberships (
			user_id, group_id, wa_group_id, participant_hash, phone_hash,
			hash_version, id_type, ddd, role, is_active, joined_at, left_at,
			first_seen_at, first_seen_source,
			last_event_at, last_provider_created_at, last_provider_event_id,
			last_event_type, last_event_applied_at, updated_at)
		SELECT $1, $2, $3, p.participant_hash, p.phone_hash,
		       1, p.id_type, p.ddd, p.role, $4,
		       CASE WHEN $4 THEN $5::timestamptz ELSE NULL END,
		       CASE WHEN $4 THEN NULL ELSE $5::timestamptz END,
		       $6::timestamptz, 'webhook',
		       $5::timestamptz, $7::timestamptz, $8, $9, $6::timestamptz, now()
		FROM UNNEST($10::text[], $11::text[], $12::text[], $13::text[], $14::text[])
		     AS p(participant_hash, phone_hash, id_type, ddd, role)
		ON CONFLICT (user_id, wa_group_id, participant_hash) DO UPDATE SET
			role = EXCLUDED.role,
			is_active = EXCLUDED.is_active,
			-- join novo substitui joined_at (nova permanência); leave preserva
			-- o joined_at da permanência que acabou de terminar.
			joined_at = CASE WHEN EXCLUDED.last_event_type = 'join'
			                 THEN EXCLUDED.joined_at
			                 ELSE group_memberships.joined_at END,
			-- reentrada limpa a saída anterior.
			left_at = CASE WHEN EXCLUDED.last_event_type = 'leave'
			               THEN EXCLUDED.left_at
			               ELSE NULL END,
			phone_hash = COALESCE(EXCLUDED.phone_hash, group_memberships.phone_hash),
			ddd = COALESCE(EXCLUDED.ddd, group_memberships.ddd),
			group_id = COALESCE(EXCLUDED.group_id, group_memberships.group_id),
			last_event_at = EXCLUDED.last_event_at,
			last_provider_created_at = EXCLUDED.last_provider_created_at,
			last_provider_event_id = EXCLUDED.last_provider_event_id,
			last_event_type = EXCLUDED.last_event_type,
			last_event_applied_at = EXCLUDED.last_event_applied_at,
			updated_at = now()
		WHERE ` + projectionOrderGuard

	if _, err := tx.Exec(ctx, upsert,
		tenant.UserID, groupID, event.WAGroupID, isJoin,
		event.OccurredAt, event.AppliedAt, event.ProviderCreatedAt,
		event.ProviderEventID, event.EventType,
		p.hashes, p.phoneHash, p.idTypes, p.ddds, p.roles,
	); err != nil {
		return fmt.Errorf("upsert group memberships: %w", err)
	}
	return nil
}

// bumpDailyCounters incrementa entradas/saídas na data LOCAL do fato. Evento
// atrasado atualiza sua data histórica correta, mesmo dias depois; duplicata
// não chega aqui (só as linhas realmente inseridas são contadas).
func bumpDailyCounters(
	ctx context.Context,
	tx pgx.Tx,
	userID string,
	groupID *string,
	event models.GroupMemberWebhookEvent,
	n int,
) error {
	const q = `
		INSERT INTO group_member_daily_snapshots (
			user_id, group_id, wa_group_id, snapshot_date, joins_count, leaves_count)
		VALUES ($1, $2, $3, ($4::timestamptz AT TIME ZONE $5)::date,
		        CASE WHEN $6 = 'join' THEN $7 ELSE 0 END,
		        CASE WHEN $6 = 'leave' THEN $7 ELSE 0 END)
		ON CONFLICT (user_id, wa_group_id, snapshot_date) DO UPDATE SET
			joins_count = group_member_daily_snapshots.joins_count + EXCLUDED.joins_count,
			leaves_count = group_member_daily_snapshots.leaves_count + EXCLUDED.leaves_count,
			group_id = COALESCE(EXCLUDED.group_id, group_member_daily_snapshots.group_id),
			updated_at = now()`

	if _, err := tx.Exec(ctx, q,
		userID, groupID, event.WAGroupID, event.OccurredAt, snapshotTZ, event.EventType, n,
	); err != nil {
		return fmt.Errorf("bump group member daily counters: %w", err)
	}
	return nil
}

// refreshMembersObservation grava a contagem de membros ativos como observação
// PONTUAL na data local do instante observado (não do fato) — é o único
// significado honesto de members_count. No-op antes do baseline: sem
// reconciliação, a projeção só tem quem apareceu em webhook e a contagem seria
// um número falso, então members_count fica NULL.
func refreshMembersObservation(
	ctx context.Context,
	tx pgx.Tx,
	userID string,
	groupID *string,
	waGroupID string,
	observedAt time.Time,
) error {
	const q = `
		INSERT INTO group_member_daily_snapshots (
			user_id, group_id, wa_group_id, snapshot_date, members_count, members_observed_at)
		SELECT $1, $2, $3, ($4::timestamptz AT TIME ZONE $5)::date, cnt.n, $4
		FROM (
			SELECT count(*)::int AS n
			FROM group_memberships
			WHERE user_id = $1 AND wa_group_id = $3 AND is_active
		) cnt
		WHERE EXISTS (
			SELECT 1 FROM groups
			WHERE user_id = $1 AND wa_group_id = $3
			  AND member_tracking_started_at IS NOT NULL
		)
		ON CONFLICT (user_id, wa_group_id, snapshot_date) DO UPDATE SET
			members_count = EXCLUDED.members_count,
			members_observed_at = EXCLUDED.members_observed_at,
			group_id = COALESCE(EXCLUDED.group_id, group_member_daily_snapshots.group_id),
			updated_at = now()
		WHERE group_member_daily_snapshots.members_observed_at IS NULL
		   OR group_member_daily_snapshots.members_observed_at < EXCLUDED.members_observed_at`

	if _, err := tx.Exec(ctx, q, userID, groupID, waGroupID, observedAt, snapshotTZ); err != nil {
		return fmt.Errorf("refresh group members observation: %w", err)
	}
	return nil
}

// =============================================================================
// Reconciliação
// =============================================================================

// ClaimGroupsForReconciliation reivindica um lote disjunto de grupos.
//
// O chip é escolhido por LATERAL determinístico: instância CONECTADA do mesmo
// usuário que participe do grupo, preferindo a owner. `groups.instance_id` é
// INTEGER no banco (só o model Go o expõe como string), então não há cast — e
// participating_instance_ids cobre o caso multichip em que a owner caiu.
func (r *groupMemberRepository) ClaimGroupsForReconciliation(
	ctx context.Context,
	limit int,
	lease time.Duration,
) ([]models.GroupReconciliationCandidate, error) {
	const q = `
		WITH candidates AS (
			SELECT g.id AS group_id, g.user_id, g.wa_group_id,
			       i.id AS instance_pk, i.instance_id AS session_name
			FROM groups g
			JOIN LATERAL (
				SELECT i.id, i.instance_id
				FROM instances i
				WHERE i.user_id = g.user_id
				  AND i.status = 'connected'
				  AND (i.id = g.instance_id OR i.id = ANY(g.participating_instance_ids))
				ORDER BY (i.id = g.instance_id) DESC, i.id
				LIMIT 1
			) i ON TRUE
			WHERE g.status = 'active'
			  AND (g.members_reconcile_claimed_at IS NULL
			       OR g.members_reconcile_claimed_at < now() - $2::interval)
			ORDER BY g.members_reconciled_at ASC NULLS FIRST, g.id
			LIMIT $1
			FOR UPDATE OF g SKIP LOCKED
		)
		UPDATE groups g
		SET members_reconcile_claimed_at = now()
		FROM candidates c
		WHERE g.id = c.group_id
		RETURNING c.group_id::text, c.user_id::text, c.wa_group_id, c.instance_pk, c.session_name`

	rows, err := r.db.Query(ctx, q, limit, lease)
	if err != nil {
		return nil, fmt.Errorf("claim groups for reconciliation: %w", err)
	}
	defer rows.Close()

	var out []models.GroupReconciliationCandidate
	for rows.Next() {
		var c models.GroupReconciliationCandidate
		if err := rows.Scan(&c.GroupID, &c.UserID, &c.WAGroupID, &c.InstancePK, &c.SessionName); err != nil {
			return nil, fmt.Errorf("scan reconciliation candidate: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("claim groups rows: %w", err)
	}
	return out, nil
}

// ReconcileGroup aplica a fotografia atual de participantes de um grupo.
//
// Nunca escreve no ledger: reconciliação não é fato de webhook, é baseline. Por
// isso não inventa joined_at/left_at — quem só foi VISTO entra com joined_at
// NULL e quem sumiu sai com left_at NULL.
func (r *groupMemberRepository) ReconcileGroup(
	ctx context.Context,
	candidate models.GroupReconciliationCandidate,
	members []models.GroupReconciliationMember,
	observationStartedAt time.Time,
) error {
	hashes := make([]string, 0, len(members))
	phoneHashes := make([]*string, 0, len(members))
	idTypes := make([]string, 0, len(members))
	ddds := make([]*string, 0, len(members))
	roles := make([]string, 0, len(members))
	for i := range members {
		hashes = append(hashes, members[i].ParticipantHash)
		phoneHashes = append(phoneHashes, members[i].PhoneHash)
		idTypes = append(idTypes, members[i].IDType)
		ddds = append(ddds, members[i].DDD)
		roles = append(roles, members[i].Role)
	}

	return r.db.WithTx(ctx, func(tx pgx.Tx) error {
		// Mesmo lock do webhook: as duas mutações do grupo se serializam.
		if err := lockTenantGroup(ctx, tx, candidate.UserID, candidate.WAGroupID); err != nil {
			return err
		}

		groupID, err := resolveGroupID(ctx, tx, candidate.UserID, candidate.WAGroupID)
		if err != nil {
			return err
		}

		// 1. Presentes: upsert. O guard só protege o ESTADO — o enriquecimento
		//    de phone_hash/ddd é não destrutivo e pode acontecer sempre.
		if len(members) > 0 {
			const upsertPresent = `
				INSERT INTO group_memberships (
					user_id, group_id, wa_group_id, participant_hash, phone_hash,
					hash_version, id_type, ddd, role, is_active, joined_at, left_at,
					first_seen_at, first_seen_source, last_reconciled_at, updated_at)
				SELECT $1, $2, $3, p.participant_hash, p.phone_hash,
				       1, p.id_type, p.ddd, p.role, TRUE, NULL, NULL,
				       $4::timestamptz, 'reconciliation', $4::timestamptz, now()
				FROM UNNEST($5::text[], $6::text[], $7::text[], $8::text[], $9::text[])
				     AS p(participant_hash, phone_hash, id_type, ddd, role)
				ON CONFLICT (user_id, wa_group_id, participant_hash) DO UPDATE SET
					role = CASE WHEN group_memberships.last_event_applied_at IS NULL
					                 OR group_memberships.last_event_applied_at < $4::timestamptz
					            THEN EXCLUDED.role ELSE group_memberships.role END,
					is_active = CASE WHEN group_memberships.last_event_applied_at IS NULL
					                      OR group_memberships.last_event_applied_at < $4::timestamptz
					                 THEN TRUE ELSE group_memberships.is_active END,
					-- reativação vista só na foto: não inventamos QUANDO entrou.
					joined_at = CASE WHEN (group_memberships.last_event_applied_at IS NULL
					                       OR group_memberships.last_event_applied_at < $4::timestamptz)
					                      AND NOT group_memberships.is_active
					                 THEN NULL ELSE group_memberships.joined_at END,
					left_at = CASE WHEN (group_memberships.last_event_applied_at IS NULL
					                     OR group_memberships.last_event_applied_at < $4::timestamptz)
					               THEN NULL ELSE group_memberships.left_at END,
					phone_hash = COALESCE(group_memberships.phone_hash, EXCLUDED.phone_hash),
					ddd = COALESCE(group_memberships.ddd, EXCLUDED.ddd),
					group_id = COALESCE(EXCLUDED.group_id, group_memberships.group_id),
					last_reconciled_at = EXCLUDED.last_reconciled_at,
					updated_at = now()`

			if _, err := tx.Exec(ctx, upsertPresent,
				candidate.UserID, groupID, candidate.WAGroupID, observationStartedAt,
				hashes, phoneHashes, idTypes, ddds, roles,
			); err != nil {
				return fmt.Errorf("upsert reconciled memberships: %w", err)
			}
		}

		// 2. Ausentes: desativa. Uma resposta válida VAZIA é legítima (grupo
		//    esvaziou) e desativa todos os não protegidos pelo guard.
		const deactivateAbsent = `
			UPDATE group_memberships m
			SET is_active = FALSE,
			    left_at = NULL,
			    last_reconciled_at = $3,
			    updated_at = now()
			WHERE m.user_id = $1
			  AND m.wa_group_id = $2
			  AND m.is_active
			  AND NOT (m.participant_hash = ANY($4::text[]))
			  AND (m.last_event_applied_at IS NULL OR m.last_event_applied_at < $3)`

		if _, err := tx.Exec(ctx, deactivateAbsent,
			candidate.UserID, candidate.WAGroupID, observationStartedAt, hashes,
		); err != nil {
			return fmt.Errorf("deactivate absent memberships: %w", err)
		}

		// 3. Baseline + fim do claim. member_tracking_started_at só é definido
		//    aqui: um webhook isolado não estabelece cobertura do grupo.
		const markGroup = `
			UPDATE groups
			SET member_tracking_started_at = COALESCE(member_tracking_started_at, $2),
			    members_reconciled_at = $2,
			    members_reconcile_claimed_at = NULL
			WHERE id = $1`

		if _, err := tx.Exec(ctx, markGroup, candidate.GroupID, observationStartedAt); err != nil {
			return fmt.Errorf("mark group reconciled: %w", err)
		}

		// 4. Observação diária (o baseline acabou de existir).
		return refreshMembersObservation(ctx, tx, candidate.UserID, groupID, candidate.WAGroupID, observationStartedAt)
	})
}

// ReleaseReconciliationClaim devolve o grupo à fila após falha, sem tocar em
// members_reconciled_at (a rodada não observou nada).
func (r *groupMemberRepository) ReleaseReconciliationClaim(ctx context.Context, groupID string) error {
	const q = `UPDATE groups SET members_reconcile_claimed_at = NULL WHERE id = $1`
	if err := r.db.Exec(ctx, q, groupID); err != nil {
		return fmt.Errorf("release reconciliation claim: %w", err)
	}
	return nil
}

// =============================================================================
// Retenção
// =============================================================================

// CleanupOldGroupMemberEvents apaga fatos antigos em lotes por ctid.
func (r *groupMemberRepository) CleanupOldGroupMemberEvents(
	ctx context.Context,
	olderThan time.Time,
	batchSize, maxBatches int,
) (int64, error) {
	const q = `
		DELETE FROM group_member_events
		WHERE ctid IN (
			SELECT ctid FROM group_member_events WHERE occurred_at < $1 LIMIT $2
		)`
	return r.deleteInBatches(ctx, q, olderThan, batchSize, maxBatches)
}

// CleanupOldGroupMemberSnapshots apaga agregados antigos em lotes por ctid.
// A PROJEÇÃO nunca é apagada por retenção: ela é o estado atual, não histórico.
func (r *groupMemberRepository) CleanupOldGroupMemberSnapshots(
	ctx context.Context,
	olderThan time.Time,
	batchSize, maxBatches int,
) (int64, error) {
	const q = `
		DELETE FROM group_member_daily_snapshots
		WHERE ctid IN (
			SELECT ctid FROM group_member_daily_snapshots WHERE snapshot_date < $1::date LIMIT $2
		)`
	return r.deleteInBatches(ctx, q, olderThan, batchSize, maxBatches)
}

// deleteInBatches roda a deleção em lotes limitados, parando no primeiro lote
// vazio ou no cancelamento do contexto. O teto de lotes impede que uma
// execução do cron monopolize I/O do Postgres.
func (r *groupMemberRepository) deleteInBatches(
	ctx context.Context,
	query string,
	olderThan time.Time,
	batchSize, maxBatches int,
) (int64, error) {
	var total int64
	for i := 0; i < maxBatches; i++ {
		select {
		case <-ctx.Done():
			return total, ctx.Err()
		default:
		}

		tag, err := r.db.pool.Exec(ctx, query, olderThan, batchSize)
		if err != nil {
			return total, fmt.Errorf("cleanup group member tracking: %w", err)
		}
		n := tag.RowsAffected()
		total += n
		if n < int64(batchSize) {
			break
		}
	}
	return total, nil
}

// =============================================================================
// Leitura de métricas (Fase B)
// =============================================================================

// metricsRankingLimit e metricsDDDLimit limitam o que vai para a tela. Um
// ranking longo não é lido, e um eixo com 40 DDDs vira ruído.
const (
	metricsRankingLimit = 5
	metricsDDDLimit     = 12
)

// GetMemberMetrics monta o pacote de leitura da tela de métricas.
//
// Tudo é escopado por user_id. groupID vazio => agregado da conta.
// Nenhuma query devolve zero no lugar de "não observado": members_count segue
// NULL até haver baseline, e a interface distingue as duas coisas.
func (r *groupMemberRepository) GetMemberMetrics(
	ctx context.Context,
	userID string,
	days int,
	groupID string,
) (*models.GroupMemberMetrics, error) {
	var groupFilter *string
	if groupID != "" {
		groupFilter = &groupID
	}

	out := &models.GroupMemberMetrics{}

	// 1. Cobertura + total ativo. É o que permite à tela dizer "coletando"
	//    com número em vez de mostrar um vazio ambíguo.
	const coverage = `
		SELECT
			(SELECT count(*) FROM groups WHERE user_id = $1),
			(SELECT count(*) FROM groups WHERE user_id = $1 AND member_tracking_started_at IS NOT NULL),
			(SELECT count(*) FROM group_memberships WHERE user_id = $1 AND is_active)`

	var active int
	if err := r.db.QueryRow(ctx, coverage, userID).
		Scan(&out.GroupsTotal, &out.GroupsWithData, &active); err != nil {
		return nil, fmt.Errorf("group member metrics coverage: %w", err)
	}
	// Sem baseline nenhum, "0 membros" seria uma afirmação falsa.
	if out.GroupsWithData > 0 {
		out.ActiveMembers = &active
	}

	// 2. Série diária. O bucket já está gravado em America/Sao_Paulo, então a
	//    janela também é contada no dia LOCAL — senão a borda erraria por 3h.
	const series = `
		SELECT snapshot_date,
		       sum(members_count) FILTER (WHERE members_count IS NOT NULL)::int,
		       count(*) FILTER (WHERE members_count IS NOT NULL)::int,
		       coalesce(sum(joins_count), 0)::int,
		       coalesce(sum(leaves_count), 0)::int
		FROM group_member_daily_snapshots
		WHERE user_id = $1
		  AND snapshot_date >= ((now() AT TIME ZONE 'America/Sao_Paulo')::date - $2::int)
		  AND ($3::uuid IS NULL OR group_id = $3::uuid)
		GROUP BY snapshot_date
		ORDER BY snapshot_date`

	rows, err := r.db.Query(ctx, series, userID, days, groupFilter)
	if err != nil {
		return nil, fmt.Errorf("group member metrics series: %w", err)
	}
	for rows.Next() {
		var p models.GroupMemberDailyPoint
		if err := rows.Scan(&p.Date, &p.MembersCount, &p.GroupsObserved, &p.Joins, &p.Leaves); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan group member metrics series: %w", err)
		}
		out.Series = append(out.Series, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("group member metrics series rows: %w", err)
	}

	// 3. Ranking. members_count é a observação MAIS RECENTE do grupo na
	//    janela (não um max/sum, que misturaria dias diferentes).
	const ranking = `
		SELECT g.id::text, g.name,
		       coalesce(sum(s.joins_count), 0)::int,
		       coalesce(sum(s.leaves_count), 0)::int,
		       (coalesce(sum(s.joins_count), 0) - coalesce(sum(s.leaves_count), 0))::int AS net,
		       (array_agg(s.members_count ORDER BY s.snapshot_date DESC)
		          FILTER (WHERE s.members_count IS NOT NULL))[1]
		FROM group_member_daily_snapshots s
		JOIN groups g ON g.id = s.group_id
		WHERE s.user_id = $1
		  AND s.snapshot_date >= ((now() AT TIME ZONE 'America/Sao_Paulo')::date - $2::int)
		GROUP BY g.id, g.name
		HAVING coalesce(sum(s.joins_count), 0) + coalesce(sum(s.leaves_count), 0) > 0
		ORDER BY net ` + "%s" + `, g.name
		LIMIT $3`

	loadRanking := func(direction string) ([]models.GroupMemberRankingEntry, error) {
		q := fmt.Sprintf(ranking, direction)
		rs, err := r.db.Query(ctx, q, userID, days, metricsRankingLimit)
		if err != nil {
			return nil, fmt.Errorf("group member metrics ranking: %w", err)
		}
		defer rs.Close()

		var list []models.GroupMemberRankingEntry
		for rs.Next() {
			var e models.GroupMemberRankingEntry
			if err := rs.Scan(&e.GroupID, &e.GroupName, &e.Joins, &e.Leaves, &e.Net, &e.MembersCount); err != nil {
				return nil, fmt.Errorf("scan group member metrics ranking: %w", err)
			}
			list = append(list, e)
		}
		return list, rs.Err()
	}

	if out.TopGrowth, err = loadRanking("DESC"); err != nil {
		return nil, err
	}
	if out.TopLoss, err = loadRanking("ASC"); err != nil {
		return nil, err
	}

	// 4. Distribuição por DDD dos membros ATIVOS. Só entra quem tem telefone
	//    identificado — um @lid sem pn não tem DDD e não é contado.
	const ddd = `
		SELECT ddd, count(*)::int
		FROM group_memberships
		WHERE user_id = $1 AND is_active AND ddd IS NOT NULL
		GROUP BY ddd
		ORDER BY 2 DESC, 1
		LIMIT $2`

	dRows, err := r.db.Query(ctx, ddd, userID, metricsDDDLimit)
	if err != nil {
		return nil, fmt.Errorf("group member metrics ddd: %w", err)
	}
	defer dRows.Close()
	for dRows.Next() {
		var e models.DDDDistributionEntry
		if err := dRows.Scan(&e.DDD, &e.Members); err != nil {
			return nil, fmt.Errorf("scan group member metrics ddd: %w", err)
		}
		out.DDDDistribution = append(out.DDDDistribution, e)
	}
	if err := dRows.Err(); err != nil {
		return nil, fmt.Errorf("group member metrics ddd rows: %w", err)
	}

	return out, nil
}
