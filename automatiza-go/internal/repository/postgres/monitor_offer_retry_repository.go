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

// Ledger de ofertas/campanhas do monitor (Melhoria 9, migração 079).
//
// As duas regras da MonitorCadenceRepository valem aqui igualmente:
//
//  1. O relógio é o do PostgreSQL (transaction_timestamp()). O relógio do Go
//     nunca decide lease, backoff ou expiração — duas réplicas com skew de
//     segundos reordenariam a fila e reemitiriam lease viva.
//  2. Toda transição é CAS. Um worker antigo, acordando depois de um lease
//     reemitido, afeta zero linhas e vira no-op. É isso que fecha o ABA de task
//     reenfileirada.
//
// Uma terceira, específica desta melhoria:
//
//  3. `sent` e `ambiguous` são ABSORVENTES. Nenhum caminho deste arquivo os
//     tira do lugar. Reenviar um `sent` duplica a oferta no grupo do cliente;
//     reenviar um `ambiguous` pode duplicar. A perda visível é preferível à
//     duplicata silenciosa — e é por isso que o ledger existe.
type monitorOfferRetryRepository struct {
	db     *DB
	logger *zap.Logger
}

// NewMonitorOfferRetryRepository cria o repositório do ledger da melhoria 9.
func NewMonitorOfferRetryRepository(db *DB, logger *zap.Logger) repository.MonitorOfferRetryRepository {
	return &monitorOfferRetryRepository{db: db, logger: logger}
}

// Leases, na sintaxe de interval do PostgreSQL — é o banco que as aplica.
// Ficam junto do SQL que as usa para não haver dois lugares dizendo por quanto
// tempo uma lease vale.
const (
	// Sweep morreu entre reivindicar o item e enfileirar a task.
	monitorOfferEnqueueLeaseTTL = "3 minutes"
	// Delivery reivindicada ('prepared') sem POST autorizado: seguro recusar,
	// porque 'prepared' PROVA que nenhum POST começou.
	monitorOfferPreparedLeaseTTL = "3 minutes"
	// Delivery que JÁ autorizou o POST. Curto de propósito: o desfecho aqui é
	// ambiguous (terminal) e não pode demorar a ser registrado.
	monitorOfferSendingLeaseTTL = "3 minutes"
)

// errInvalidReason falha no Go, com contexto, em vez de virar violação de CHECK
// no Postgres (que sobe como erro genérico 23514 sem dizer qual coluna).
func validateReason(reason string) error {
	if !models.IsValidReasonCode(reason) {
		return fmt.Errorf("monitor_offer: reason_code fora do enum: %q", reason)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Criação
// ---------------------------------------------------------------------------

// CreateJobWithItems grava job + itens sintáticos numa única transação, ANTES
// de qualquer I/O externo.
//
// O ON CONFLICT DO NOTHING na unique de identidade é o coração da idempotência:
// redelivery do mesmo webhook não insere, devolve created=false, e o chamador
// não chama API nem envia nada. Sem isso, o segundo delivery geraria uma
// segunda oferta no grupo do cliente.
func (r *monitorOfferRetryRepository) CreateJobWithItems(
	ctx context.Context,
	job *models.MonitorOfferJob,
	items []models.MonitorOfferItem,
) (bool, error) {
	if job == nil {
		return false, errors.New("monitor_offer: job nil")
	}
	if job.MessageID == "" {
		// §5.5: message_id vazio é payload inválido. Criar identidade baseada
		// em timestamp/corpo permitiria colisão entre mensagens distintas E
		// redelivery duplicado da mesma — os dois lados ruins.
		return false, fmt.Errorf("monitor_offer: %s", models.ReasonMissingMessageID)
	}
	if err := validateReason(job.ReasonCode); err != nil {
		return false, err
	}
	for i := range items {
		if err := validateReason(items[i].ReasonCode); err != nil {
			return false, err
		}
	}

	created := false
	err := r.db.WithTx(ctx, func(tx pgx.Tx) error {
		var jobID string
		row := tx.QueryRow(ctx, `
			INSERT INTO monitor_offer_jobs (
				user_id, automation_id, mode, source_group_id, message_id,
				message_timestamp, selected_product_key, payload_ciphertext,
				payload_expires_at, reason_code, status
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
			ON CONFLICT (user_id, automation_id, mode, source_group_id, message_id)
			DO NOTHING
			RETURNING id::text`,
			job.UserID, job.AutomationID, job.Mode, job.SourceGroupID, job.MessageID,
			job.MessageTimestamp, job.SelectedProductKey, job.PayloadCiphertext,
			job.PayloadExpiresAt, job.ReasonCode, job.Status)

		switch err := row.Scan(&jobID); {
		case errors.Is(err, pgx.ErrNoRows):
			// Identidade já existia: alguém ganhou a corrida. Não inserimos
			// itens (a passagem vencedora já inseriu os dela) e o chamador
			// para aqui.
			return nil
		case err != nil:
			return err
		}

		job.ID = jobID
		created = true

		for i := range items {
			it := &items[i]
			var itemID string
			if err := tx.QueryRow(ctx, `
				INSERT INTO monitor_offer_items (
					job_id, ordinal, paragraph_ordinal, source_link_hash, host,
					canonical_key, store, url_class, error_class, reason_code,
					output_key, output_kind, outcome, status, next_attempt_at
				) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
				RETURNING id::text`,
				jobID, it.Ordinal, it.ParagraphOrdinal, it.SourceLinkHash, it.Host,
				it.CanonicalKey, it.Store, it.URLClass, it.ErrorClass, it.ReasonCode,
				it.OutputKey, it.OutputKind, it.Outcome, it.Status, it.NextAttemptAt,
			).Scan(&itemID); err != nil {
				return err
			}
			it.ID = itemID
			it.JobID = jobID
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	return created, nil
}

// ---------------------------------------------------------------------------
// Leitura
// ---------------------------------------------------------------------------

const jobColumns = `id::text, user_id::text, automation_id::text, mode,
	source_group_id, message_id, message_timestamp, selected_product_key,
	payload_ciphertext, payload_expires_at, reason_code, status,
	created_at, updated_at`

func scanJob(row pgx.Row) (*models.MonitorOfferJob, error) {
	var j models.MonitorOfferJob
	if err := row.Scan(&j.ID, &j.UserID, &j.AutomationID, &j.Mode,
		&j.SourceGroupID, &j.MessageID, &j.MessageTimestamp, &j.SelectedProductKey,
		&j.PayloadCiphertext, &j.PayloadExpiresAt, &j.ReasonCode, &j.Status,
		&j.CreatedAt, &j.UpdatedAt); err != nil {
		return nil, err
	}
	return &j, nil
}

// GetJob lê escopado por tenant. O user_id vai no WHERE, não numa comparação
// posterior em Go: um SELECT sem ele já teria lido a linha do outro cliente.
func (r *monitorOfferRetryRepository) GetJob(ctx context.Context, userID, jobID string) (*models.MonitorOfferJob, error) {
	j, err := scanJob(r.db.Pool().QueryRow(ctx,
		`SELECT `+jobColumns+` FROM monitor_offer_jobs WHERE id = $1 AND user_id = $2`,
		jobID, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return j, err
}

const itemColumns = `id::text, job_id::text, ordinal, paragraph_ordinal,
	source_link_hash, host, canonical_key, duplicate_of_item_id::text, store,
	url_class, error_class, reason_code, output_key, output_kind, outcome,
	status, attempts, total_attempts, replay_count, enqueue_generation,
	next_attempt_at, lease_until, attempt_started_at, created_at, updated_at`

func scanItem(row pgx.Row) (*models.MonitorOfferItem, error) {
	var it models.MonitorOfferItem
	if err := row.Scan(&it.ID, &it.JobID, &it.Ordinal, &it.ParagraphOrdinal,
		&it.SourceLinkHash, &it.Host, &it.CanonicalKey, &it.DuplicateOfItemID, &it.Store,
		&it.URLClass, &it.ErrorClass, &it.ReasonCode, &it.OutputKey, &it.OutputKind,
		&it.Outcome, &it.Status, &it.Attempts, &it.TotalAttempts, &it.ReplayCount,
		&it.EnqueueGeneration, &it.NextAttemptAt, &it.LeaseUntil, &it.AttemptStartedAt,
		&it.CreatedAt, &it.UpdatedAt); err != nil {
		return nil, err
	}
	return &it, nil
}

func (r *monitorOfferRetryRepository) ListItems(ctx context.Context, jobID string) ([]models.MonitorOfferItem, error) {
	rows, err := r.db.Pool().Query(ctx,
		`SELECT `+itemColumns+` FROM monitor_offer_items WHERE job_id = $1 ORDER BY ordinal`, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.MonitorOfferItem
	for rows.Next() {
		it, sErr := scanItem(rows)
		if sErr != nil {
			return nil, sErr
		}
		out = append(out, *it)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// Ciclo de tentativa
// ---------------------------------------------------------------------------

// ClaimDueItems reivindica itens vencidos e já os leva para a próxima geração.
//
// Duas restrições no SQL merecem atenção:
//
//   - `lease_until IS NULL OR lease_until <= now`: um item já reivindicado por
//     outro sweep não é tocado até a lease vencer.
//   - o item precisa ser o de MENOR ordinal pendente do job enquanto o job não
//     tem produto escolhido. Sem isso, dois unknowns do mesmo job seriam
//     retentados em paralelo e a corrida decidiria qual vira "o produto" —
//     resultado não determinístico para o cliente.
func (r *monitorOfferRetryRepository) ClaimDueItems(ctx context.Context, limit int) ([]models.MonitorOfferItemLease, error) {
	if limit <= 0 {
		limit = 100
	}
	var leases []models.MonitorOfferItemLease

	err := r.db.WithTx(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			WITH due AS (
				SELECT i.id
				FROM monitor_offer_items i
				JOIN monitor_offer_jobs j ON j.id = i.job_id
				WHERE i.status = 'pending'
				  AND i.next_attempt_at IS NOT NULL
				  AND i.next_attempt_at <= transaction_timestamp()
				  AND (i.lease_until IS NULL OR i.lease_until <= transaction_timestamp())
				  AND j.status = 'active'
				  AND (
				      j.selected_product_key IS NOT NULL
				      OR i.ordinal = (
				          SELECT MIN(i2.ordinal)
				          FROM monitor_offer_items i2
				          WHERE i2.job_id = i.job_id AND i2.status = 'pending'
				      )
				  )
				ORDER BY i.next_attempt_at
				LIMIT $1
				FOR UPDATE OF i SKIP LOCKED
			)
			UPDATE monitor_offer_items m
			SET enqueue_generation = m.enqueue_generation + 1,
			    lease_until = transaction_timestamp() + INTERVAL '`+monitorOfferEnqueueLeaseTTL+`',
			    updated_at = transaction_timestamp()
			FROM due, monitor_offer_jobs j
			WHERE m.id = due.id AND j.id = m.job_id
			RETURNING m.id::text, m.job_id::text, j.user_id::text,
			          m.enqueue_generation, m.next_attempt_at`, limit)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var l models.MonitorOfferItemLease
			var runAt *time.Time
			if sErr := rows.Scan(&l.ItemID, &l.JobID, &l.UserID, &l.Generation, &runAt); sErr != nil {
				return sErr
			}
			if runAt != nil {
				l.RunAt = *runAt
			}
			leases = append(leases, l)
		}
		return rows.Err()
	})
	return leases, err
}

// ClaimItemForAttempt é o CAS do handler.
//
// Reivindicar e mudar o status precisam ser o MESMO statement: com um SELECT
// seguido de UPDATE, dois handlers da mesma geração passariam os dois pelo
// SELECT e ambos processariam o item.
func (r *monitorOfferRetryRepository) ClaimItemForAttempt(ctx context.Context, itemID string, generation int) (*models.MonitorOfferAttempt, error) {
	var att models.MonitorOfferAttempt

	err := r.db.WithTx(ctx, func(tx pgx.Tx) error {
		var claimedJobID string
		err := tx.QueryRow(ctx, `
			UPDATE monitor_offer_items
			SET status = 'processing',
			    lease_until = transaction_timestamp() + INTERVAL '`+monitorOfferEnqueueLeaseTTL+`',
			    updated_at = transaction_timestamp()
			WHERE id = $1 AND enqueue_generation = $2 AND status = 'pending'
			RETURNING job_id::text`, itemID, generation).Scan(&claimedJobID)
		if errors.Is(err, pgx.ErrNoRows) {
			// Geração obsoleta ou item já terminal: no-op silencioso. NÃO é
			// erro — é exatamente o desfecho que torna seguro perder um ack.
			return pgx.ErrNoRows
		}
		if err != nil {
			return err
		}

		it, err := scanItem(tx.QueryRow(ctx, `SELECT `+itemColumns+` FROM monitor_offer_items WHERE id = $1`, itemID))
		if err != nil {
			return err
		}
		j, err := scanJob(tx.QueryRow(ctx, `SELECT `+jobColumns+` FROM monitor_offer_jobs WHERE id = $1`, claimedJobID))
		if err != nil {
			return err
		}
		att.Item, att.Job = *it, *j
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &att, nil
}

// MarkAttemptStarted é a fronteira entre "reivindicado" e "tentado".
//
// Está separado do claim de propósito: um crash ANTES deste marcador devolve o
// item a pending SEM consumir tentativa (o ReapStaleLeases olha
// attempt_started_at). Sem essa separação, um processo morto no boot queimaria
// as 5 tentativas do item sem nunca ter falado com a Shopee.
func (r *monitorOfferRetryRepository) MarkAttemptStarted(ctx context.Context, itemID string) error {
	_, err := r.db.Pool().Exec(ctx, `
		UPDATE monitor_offer_items
		SET attempts = LEAST(attempts + 1, $2),
		    total_attempts = total_attempts + 1,
		    attempt_started_at = transaction_timestamp(),
		    updated_at = transaction_timestamp()
		WHERE id = $1 AND status = 'processing'`, itemID, models.MaxItemAttempts)
	return err
}

// FinishItem aplica o desfecho e recomputa o status do job na MESMA transação.
func (r *monitorOfferRetryRepository) FinishItem(ctx context.Context, out models.MonitorOfferItemOutcome) error {
	if err := validateReason(out.ReasonCode); err != nil {
		return err
	}
	return r.db.WithTx(ctx, func(tx pgx.Tx) error {
		var jobID string
		err := tx.QueryRow(ctx, `
			UPDATE monitor_offer_items
			SET status = $2, url_class = $3, error_class = $4, reason_code = $5,
			    outcome = $6,
			    canonical_key = COALESCE($7, canonical_key),
			    output_key = COALESCE($8, output_key),
			    output_kind = COALESCE($9, output_kind),
			    lease_until = NULL,
			    attempt_started_at = NULL,
			    next_attempt_at = NULL,
			    updated_at = transaction_timestamp()
			WHERE id = $1
			RETURNING job_id::text`,
			out.ItemID, out.Status, out.URLClass, out.ErrorClass, out.ReasonCode,
			out.Outcome, out.CanonicalKey, out.OutputKey, out.OutputKind).Scan(&jobID)
		if err != nil {
			return err
		}
		return recomputeJobStatus(ctx, tx, jobID)
	})
}

// RescheduleItem devolve o item a pending.
//
// consumeAttempt=false existe para os casos que NÃO são falha de negócio: fora
// da janela da automação, sem chip disponível, shutdown do processo. Contá-los
// gastaria as 5 tentativas do item sem nenhuma chamada externa ter falhado.
func (r *monitorOfferRetryRepository) RescheduleItem(ctx context.Context, itemID string, nextAttemptAt time.Time, reasonCode string, consumeAttempt bool) error {
	if err := validateReason(reasonCode); err != nil {
		return err
	}
	return r.db.WithTx(ctx, func(tx pgx.Tx) error {
		var jobID string
		// Chegar ao teto de tentativas encerra o item como exhausted em vez de
		// deixá-lo pending para sempre.
		err := tx.QueryRow(ctx, `
			UPDATE monitor_offer_items
			SET status = CASE
			        WHEN $4 AND attempts >= $5 THEN 'exhausted'
			        ELSE 'pending'
			    END,
			    reason_code = $3,
			    next_attempt_at = CASE
			        WHEN $4 AND attempts >= $5 THEN NULL
			        ELSE $2
			    END,
			    lease_until = NULL,
			    attempt_started_at = NULL,
			    updated_at = transaction_timestamp()
			WHERE id = $1
			RETURNING job_id::text`,
			itemID, nextAttemptAt, reasonCode, consumeAttempt, models.MaxItemAttempts).Scan(&jobID)
		if err != nil {
			return err
		}
		return recomputeJobStatus(ctx, tx, jobID)
	})
}

// PublishCanonicalKey resolve a corrida de duas ocorrências do mesmo produto.
//
// O SELECT ... FOR UPDATE no JOB serializa a decisão: sem ele, duas goroutines
// resolvendo links diferentes do mesmo job poderiam ambas achar a key livre e
// uma levaria violação de unique — desfecho dependente da ordem do escalonador.
//
// Regra de desempate: menor ordinal vence, PORQUE ele é o link que apareceu
// primeiro no texto e é o que o cliente espera ver virar oferta. Mas depois que
// a key pertence a uma saída já preparada/enviada, ela nunca troca de dono —
// trocar reescreveria uma oferta que já saiu.
func (r *monitorOfferRetryRepository) PublishCanonicalKey(ctx context.Context, itemID, canonicalKey string) (models.CanonicalKeyVerdict, error) {
	var verdict models.CanonicalKeyVerdict

	err := r.db.WithTx(ctx, func(tx pgx.Tx) error {
		var jobID string
		var myOrdinal int16
		if err := tx.QueryRow(ctx,
			`SELECT job_id::text, ordinal FROM monitor_offer_items WHERE id = $1`, itemID).
			Scan(&jobID, &myOrdinal); err != nil {
			return err
		}
		// Serializa a decisão por job.
		if _, err := tx.Exec(ctx, `SELECT 1 FROM monitor_offer_jobs WHERE id = $1 FOR UPDATE`, jobID); err != nil {
			return err
		}

		var holderID string
		var holderOrdinal int16
		var holderOutputKey *string
		err := tx.QueryRow(ctx, `
			SELECT id::text, ordinal, output_key
			FROM monitor_offer_items
			WHERE job_id = $1 AND canonical_key = $2 AND id <> $3`,
			jobID, canonicalKey, itemID).Scan(&holderID, &holderOrdinal, &holderOutputKey)

		switch {
		case errors.Is(err, pgx.ErrNoRows):
			// Key livre.
		case err != nil:
			return err
		case holderOutputKey != nil:
			// Já virou saída preparada/enviada: nunca troca de dono.
			verdict.Won = false
			verdict.Reason = models.ReasonDuplicateLink
			return nil
		case holderOrdinal <= myOrdinal:
			verdict.Won = false
			verdict.Reason = models.ReasonDuplicateLink
			return nil
		default:
			// Chegamos com ordinal MENOR: o detentor perde a key.
			if _, uErr := tx.Exec(ctx, `
				UPDATE monitor_offer_items
				SET canonical_key = NULL, status = 'superseded',
				    reason_code = $2, duplicate_of_item_id = $3,
				    next_attempt_at = NULL, lease_until = NULL,
				    updated_at = transaction_timestamp()
				WHERE id = $1`, holderID, models.ReasonDuplicateLink, itemID); uErr != nil {
				return uErr
			}
			held := holderID
			verdict.SupersededItemID = &held
		}

		if _, err := tx.Exec(ctx, `
			UPDATE monitor_offer_items
			SET canonical_key = $2, updated_at = transaction_timestamp()
			WHERE id = $1`, itemID, canonicalKey); err != nil {
			return err
		}
		verdict.Won = true
		verdict.Reason = models.ReasonNone
		return recomputeJobStatus(ctx, tx, jobID)
	})
	return verdict, err
}

// ---------------------------------------------------------------------------
// Entrega por destino
// ---------------------------------------------------------------------------

// UpsertDelivery cria a linha em 'prepared'. Idempotente na PK: reexecutar a
// preparação de um destino cujo desfecho já é terminal não o reabre.
func (r *monitorOfferRetryRepository) UpsertDelivery(ctx context.Context, d models.MonitorOfferDelivery) error {
	if err := validateReason(d.ReasonCode); err != nil {
		return err
	}
	_, err := r.db.Pool().Exec(ctx, `
		INSERT INTO monitor_offer_deliveries (
			job_id, output_key, output_kind, destination_group_id, status, reason_code
		) VALUES ($1,$2,$3,$4,'prepared',$5)
		ON CONFLICT (job_id, output_key, destination_group_id) DO NOTHING`,
		d.JobID, d.OutputKey, d.OutputKind, d.DestinationGroupID, d.ReasonCode)
	return err
}

// MarkDeliverySending é o CAS imediatamente ANTES do POST na WAHA.
//
// A janela entre este UPDATE e o POST é a fronteira do "não sei se entregou":
// se o processo morrer aqui, o reaper marca ambiguous e ninguém reenvia.
//
// Só 'prepared' e 'failed' comprovadamente pré-POST podem virar 'sending'.
// 'sent' e 'ambiguous' estão fora do WHERE de propósito.
func (r *monitorOfferRetryRepository) MarkDeliverySending(ctx context.Context, jobID, outputKey, destinationGroupID, token string) (bool, error) {
	tag, err := r.db.Pool().Exec(ctx, `
		UPDATE monitor_offer_deliveries
		SET status = 'sending', attempt_token = $4::uuid,
		    sending_at = transaction_timestamp(),
		    updated_at = transaction_timestamp()
		WHERE job_id = $1 AND output_key = $2 AND destination_group_id = $3
		  AND status IN ('prepared','failed')`,
		jobID, outputKey, destinationGroupID, token)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// FinishDelivery grava o desfecho, exigindo o token da tentativa.
//
// O token impede que um worker antigo — cujo POST demorou mais que a lease e
// cuja delivery já virou ambiguous — sobrescreva o desfecho com 'sent'. E o
// WHERE exclui 'sent'/'ambiguous', que são absorventes.
func (r *monitorOfferRetryRepository) FinishDelivery(ctx context.Context, jobID, outputKey, destinationGroupID, token, status, reasonCode string, providerMessageID *string) error {
	if err := validateReason(reasonCode); err != nil {
		return err
	}
	return r.db.WithTx(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE monitor_offer_deliveries
			SET status = $5, reason_code = $6, provider_message_id = $7,
			    sent_at = CASE WHEN $5 = 'sent' THEN transaction_timestamp() ELSE sent_at END,
			    updated_at = transaction_timestamp()
			WHERE job_id = $1 AND output_key = $2 AND destination_group_id = $3
			  AND (attempt_token = $4::uuid OR $4 = '')
			  AND status NOT IN ('sent','ambiguous')`,
			jobID, outputKey, destinationGroupID, token, status, reasonCode, providerMessageID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			// Nada a fazer: o desfecho já era terminal, ou o token é de uma
			// tentativa que perdeu a corrida. Não é erro.
			return nil
		}
		return recomputeJobStatus(ctx, tx, jobID)
	})
}

func (r *monitorOfferRetryRepository) ListDeliveries(ctx context.Context, jobID, outputKey string) ([]models.MonitorOfferDelivery, error) {
	rows, err := r.db.Pool().Query(ctx, `
		SELECT job_id::text, output_key, output_kind, destination_group_id, status,
		       attempt_token::text, provider_message_id, reason_code,
		       sending_at, sent_at, updated_at
		FROM monitor_offer_deliveries
		WHERE job_id = $1 AND ($2 = '' OR output_key = $2)
		ORDER BY output_key, destination_group_id`, jobID, outputKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.MonitorOfferDelivery
	for rows.Next() {
		var d models.MonitorOfferDelivery
		if sErr := rows.Scan(&d.JobID, &d.OutputKey, &d.OutputKind, &d.DestinationGroupID,
			&d.Status, &d.AttemptToken, &d.ProviderMessageID, &d.ReasonCode,
			&d.SendingAt, &d.SentAt, &d.UpdatedAt); sErr != nil {
			return nil, sErr
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// Manutenção
// ---------------------------------------------------------------------------

// ReapStaleLeases é o que transforma crash em estado visível.
//
// Os três casos são deliberadamente assimétricos:
//
//   - 'prepared' vencida → failed/delivery_rejected. 'prepared' PROVA que
//     nenhum POST começou, então retentar é seguro.
//   - 'sending' vencida → ambiguous. Aqui o POST pode ter chegado à WAHA. NUNCA
//     volta a pending: um reenvio automático duplicaria a oferta no grupo.
//   - item 'processing' sem attempt_started_at → pending sem consumir
//     tentativa (a tentativa nem começou).
func (r *monitorOfferRetryRepository) ReapStaleLeases(ctx context.Context) (models.MonitorOfferReapStats, error) {
	var st models.MonitorOfferReapStats

	err := r.db.WithTx(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE monitor_offer_deliveries
			SET status = 'failed', reason_code = $1, updated_at = transaction_timestamp()
			WHERE status = 'prepared'
			  AND updated_at < transaction_timestamp() - INTERVAL '`+monitorOfferPreparedLeaseTTL+`'`,
			models.ReasonDeliveryRejected)
		if err != nil {
			return err
		}
		st.DeliveriesRejected = int(tag.RowsAffected())

		tag, err = tx.Exec(ctx, `
			UPDATE monitor_offer_deliveries
			SET status = 'ambiguous', reason_code = $1, updated_at = transaction_timestamp()
			WHERE status = 'sending'
			  AND sending_at IS NOT NULL
			  AND sending_at < transaction_timestamp() - INTERVAL '`+monitorOfferSendingLeaseTTL+`'`,
			models.ReasonDeliveryAmbiguous)
		if err != nil {
			return err
		}
		st.DeliveriesAmbiguous = int(tag.RowsAffected())

		tag, err = tx.Exec(ctx, `
			UPDATE monitor_offer_items
			SET status = 'pending',
			    lease_until = NULL,
			    next_attempt_at = COALESCE(next_attempt_at, transaction_timestamp()),
			    updated_at = transaction_timestamp()
			WHERE status = 'processing'
			  AND attempt_started_at IS NULL
			  AND lease_until IS NOT NULL
			  AND lease_until < transaction_timestamp()`)
		if err != nil {
			return err
		}
		st.ItemsReleased = int(tag.RowsAffected())

		// Item que JÁ começou a tentativa e perdeu a lease volta a pending
		// COM a tentativa consumida (ela realmente saiu para a rede).
		if _, err := tx.Exec(ctx, `
			UPDATE monitor_offer_items
			SET status = CASE WHEN attempts >= $1 THEN 'exhausted' ELSE 'pending' END,
			    lease_until = NULL,
			    attempt_started_at = NULL,
			    next_attempt_at = CASE WHEN attempts >= $1 THEN NULL
			                           ELSE transaction_timestamp() + INTERVAL '1 minute' END,
			    updated_at = transaction_timestamp()
			WHERE status = 'processing'
			  AND attempt_started_at IS NOT NULL
			  AND lease_until IS NOT NULL
			  AND lease_until < transaction_timestamp()`, models.MaxItemAttempts); err != nil {
			return err
		}

		// Qualquer delivery ambígua contamina o job — é o sinal que um humano
		// precisa ver.
		_, err = tx.Exec(ctx, `
			UPDATE monitor_offer_jobs j
			SET status = 'ambiguous', reason_code = $1, updated_at = transaction_timestamp()
			WHERE j.status = 'active'
			  AND EXISTS (
			      SELECT 1 FROM monitor_offer_deliveries d
			      WHERE d.job_id = j.id AND d.status = 'ambiguous'
			  )`, models.ReasonDeliveryAmbiguous)
		return err
	})
	return st, err
}

// ExpirePayloads limpa ciphertext vencido e encerra o que dependia dele.
//
// Depois de 6h o payload some, e sem payload não há como refazer classificação
// nem render — então o item ativo vira exhausted/payload_expired em vez de
// ficar pending para sempre tentando decifrar um NULL.
func (r *monitorOfferRetryRepository) ExpirePayloads(ctx context.Context) (int, error) {
	var n int
	err := r.db.WithTx(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE monitor_offer_items i
			SET status = 'exhausted', error_class = 'transient', reason_code = $1,
			    next_attempt_at = NULL, lease_until = NULL,
			    updated_at = transaction_timestamp()
			FROM monitor_offer_jobs j
			WHERE j.id = i.job_id
			  AND j.payload_expires_at <= transaction_timestamp()
			  AND i.status IN ('pending','processing')`, models.ReasonPayloadExpired)
		if err != nil {
			return err
		}
		n = int(tag.RowsAffected())

		if _, err := tx.Exec(ctx, `
			UPDATE monitor_offer_jobs
			SET payload_ciphertext = NULL, updated_at = transaction_timestamp()
			WHERE payload_ciphertext IS NOT NULL
			  AND payload_expires_at <= transaction_timestamp()`); err != nil {
			return err
		}

		// Job cujas saídas já terminaram não precisa mais do payload, mesmo
		// antes das 6h: guardar corpo cifrado sem uso é superfície à toa.
		_, err = tx.Exec(ctx, `
			UPDATE monitor_offer_jobs j
			SET payload_ciphertext = NULL, updated_at = transaction_timestamp()
			WHERE j.payload_ciphertext IS NOT NULL
			  AND j.status IN ('succeeded','superseded')`)
		return err
	})
	return n, err
}

// DeleteTerminalOlderThan apaga metadata terminal em lotes. ON DELETE CASCADE
// leva itens e deliveries junto.
func (r *monitorOfferRetryRepository) DeleteTerminalOlderThan(ctx context.Context, olderThan time.Duration, batch int) (int, error) {
	if batch <= 0 {
		batch = 500
	}
	cutoff := time.Now().UTC().Add(-olderThan)
	tag, err := r.db.Pool().Exec(ctx, `
		DELETE FROM monitor_offer_jobs
		WHERE id IN (
			SELECT id FROM monitor_offer_jobs
			WHERE status IN ('succeeded','exhausted','blocked','superseded','ambiguous')
			  AND updated_at < $1
			ORDER BY updated_at
			LIMIT $2
		)`, cutoff, batch)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

// ---------------------------------------------------------------------------
// Replay manual
// ---------------------------------------------------------------------------

func (r *monitorOfferRetryRepository) GetItemForReplay(ctx context.Context, userID, itemID string) (*models.MonitorOfferItem, *models.MonitorOfferJob, error) {
	// O user_id entra no JOIN: item de outro tenant devolve not-found, não
	// "forbidden" com os dados dentro.
	it, err := scanItem(r.db.Pool().QueryRow(ctx, `
		SELECT `+itemColumns+`
		FROM monitor_offer_items i
		WHERE i.id = $1
		  AND EXISTS (SELECT 1 FROM monitor_offer_jobs j WHERE j.id = i.job_id AND j.user_id = $2)`,
		itemID, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	j, err := r.GetJob(ctx, userID, it.JobID)
	if err != nil {
		return nil, nil, err
	}
	return it, j, nil
}

// ResetItemForReplay rearma um item terminal.
//
// total_attempts é PRESERVADO de propósito: ele é o histórico real de quantas
// vezes aquele link já foi tentado, e é o que revela um link que alguém está
// replayando em loop. attempts (o contador do ciclo) é que zera.
func (r *monitorOfferRetryRepository) ResetItemForReplay(ctx context.Context, userID, itemID string, allowAmbiguous bool) error {
	return r.db.WithTx(ctx, func(tx pgx.Tx) error {
		var jobID string
		err := tx.QueryRow(ctx, `
			UPDATE monitor_offer_items i
			SET status = 'pending',
			    attempts = 0,
			    replay_count = i.replay_count + 1,
			    enqueue_generation = i.enqueue_generation + 1,
			    lease_until = NULL,
			    attempt_started_at = NULL,
			    next_attempt_at = transaction_timestamp(),
			    updated_at = transaction_timestamp()
			FROM monitor_offer_jobs j
			WHERE i.id = $1 AND j.id = i.job_id AND j.user_id = $2
			  AND j.payload_ciphertext IS NOT NULL
			  AND j.payload_expires_at > transaction_timestamp()
			  AND (
			      (i.status = 'exhausted' AND i.error_class = 'transient')
			      OR (i.status = 'blocked' AND i.error_class = 'permanent_config')
			      OR ($3 AND i.status = 'ambiguous')
			  )
			RETURNING i.job_id::text`, itemID, userID, allowAmbiguous).Scan(&jobID)
		if errors.Is(err, pgx.ErrNoRows) {
			return errors.New("item não é replayável: verifique tenant, status, error_class e validade do payload")
		}
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
			UPDATE monitor_offer_jobs
			SET status = 'active', updated_at = transaction_timestamp()
			WHERE id = $1`, jobID)
		return err
	})
}

// ---------------------------------------------------------------------------
// Status agregado
// ---------------------------------------------------------------------------

// recomputeJobStatus roda dentro da transação de cada transição.
//
// A precedência é a do §7 do plano e a ordem importa: `ambiguous` primeiro
// porque uma única entrega duvidosa não pode ser mascarada por outras que
// deram certo — o job inteiro precisa aparecer numa consulta de incidente.
//
// Nunca infere sucesso a partir de dispatch_logs: aquela tabela é por destino
// e não sabe o que era "a saída desta mensagem".
func recomputeJobStatus(ctx context.Context, tx pgx.Tx, jobID string) error {
	_, err := tx.Exec(ctx, `
		WITH agg AS (
			SELECT
				(SELECT count(*) FROM monitor_offer_items  WHERE job_id = $1 AND status = 'ambiguous')  AS it_amb,
				(SELECT count(*) FROM monitor_offer_deliveries WHERE job_id = $1 AND status = 'ambiguous') AS dl_amb,
				(SELECT count(*) FROM monitor_offer_items  WHERE job_id = $1 AND status IN ('pending','processing')) AS it_active,
				(SELECT count(*) FROM monitor_offer_deliveries WHERE job_id = $1 AND status IN ('prepared','sending')) AS dl_active,
				(SELECT count(*) FROM monitor_offer_deliveries WHERE job_id = $1 AND status = 'failed') AS dl_failed,
				(SELECT count(*) FROM monitor_offer_items  WHERE job_id = $1 AND status = 'blocked')   AS it_blocked,
				(SELECT count(*) FROM monitor_offer_items  WHERE job_id = $1 AND status = 'exhausted') AS it_exh,
				(SELECT count(*) FROM monitor_offer_items  WHERE job_id = $1 AND status = 'superseded') AS it_sup,
				(SELECT count(*) FROM monitor_offer_items  WHERE job_id = $1) AS it_total,
				(SELECT count(*) FROM monitor_offer_deliveries WHERE job_id = $1) AS dl_total
		)
		UPDATE monitor_offer_jobs j
		SET status = CASE
		        WHEN agg.it_amb > 0 OR agg.dl_amb > 0 THEN 'ambiguous'
		        WHEN agg.it_active > 0 OR agg.dl_active > 0 OR agg.dl_failed > 0 THEN 'active'
		        WHEN agg.it_blocked > 0 THEN 'blocked'
		        WHEN agg.it_exh > 0 THEN 'exhausted'
		        WHEN agg.it_total > 0 AND agg.it_sup = agg.it_total AND agg.dl_total = 0 THEN 'superseded'
		        ELSE 'succeeded'
		    END,
		    updated_at = transaction_timestamp()
		FROM agg
		WHERE j.id = $1`, jobID)
	return err
}
