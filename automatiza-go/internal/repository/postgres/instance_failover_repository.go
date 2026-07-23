package postgres

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

// instanceFailoverRepository implementa repository.InstanceFailoverRepository.
//
// ORDEM GLOBAL DE LOCKS (vale em TODA transação deste arquivo, sem exceção):
//
//  1. pg_advisory_xact_lock(hashtextextended('instance-failover:' || user_id, 0))
//  2. instance_failover_policies FOR UPDATE
//  3. instance_failover_operations FOR UPDATE
//  4. automations por UUID em ordem ASCENDENTE FOR UPDATE
//  5. inserts/updates de ledger
//
// Nunca adquirir o advisory lock do usuário DEPOIS de um lock de linha: com
// duas réplicas isso é a receita canônica de deadlock.
type instanceFailoverRepository struct {
	db     *DB
	logger *zap.Logger
}

// NewInstanceFailoverRepository cria o repositório de failover.
func NewInstanceFailoverRepository(db *DB, logger *zap.Logger) repository.InstanceFailoverRepository {
	return &instanceFailoverRepository{db: db, logger: logger}
}

const policyColumns = `
	id, user_id, primary_instance_id, backup_instance_id, mode, state, version,
	current_operation_id, unhealthy_after_seconds, required_consecutive_probes,
	drain_deadline_seconds, proposal_ttl_seconds, primary_unhealthy_since,
	consecutive_unhealthy_probes, last_probe_at, last_probe_status,
	proposal_expires_at, created_at, updated_at`

const operationColumns = `
	id, policy_id, policy_id_snapshot, user_id, kind, status, return_policy_state,
	idempotency_key, request_hash, dry_run_id, source_operation_id,
	start_policy_version, gate_closed_at, drain_deadline_at, applied_at,
	completed_at, report, created_at, updated_at`

const dryRunColumns = `
	id, policy_id, policy_id_snapshot, user_id, direction, idempotency_key,
	request_hash, policy_version, dry_run_ref, status, report, created_at, expires_at`

const eventColumns = `
	id, policy_id, policy_id_snapshot, operation_id, dry_run_id, user_id,
	event_type, actor_type, actor_user_id, idempotency_scope, idempotency_key,
	request_hash, policy_version, snapshot_before, snapshot_after, report, created_at`

// advisoryLockUser adquire o lock de transação do usuário. É SEMPRE o primeiro
// statement de qualquer transação mutável desta feature.
func advisoryLockUser(ctx context.Context, tx pgx.Tx, userID string) error {
	_, err := tx.Exec(ctx,
		`SELECT pg_advisory_xact_lock(hashtextextended('instance-failover:' || $1::text, 0))`, userID)
	if err != nil {
		return fmt.Errorf("failover: advisory lock: %w", err)
	}
	return nil
}

func scanPolicy(row pgx.Row) (*models.InstanceFailoverPolicy, error) {
	var p models.InstanceFailoverPolicy
	err := row.Scan(
		&p.ID, &p.UserID, &p.PrimaryInstanceID, &p.BackupInstanceID, &p.Mode, &p.State,
		&p.Version, &p.CurrentOperationID, &p.UnhealthyAfterSeconds, &p.RequiredConsecutiveProbes,
		&p.DrainDeadlineSeconds, &p.ProposalTTLSeconds, &p.PrimaryUnhealthySince,
		&p.ConsecutiveUnhealthyProbes, &p.LastProbeAt, &p.LastProbeStatus,
		&p.ProposalExpiresAt, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &p, nil
}

func scanOperation(row pgx.Row) (*models.InstanceFailoverOperation, error) {
	var o models.InstanceFailoverOperation
	err := row.Scan(
		&o.ID, &o.PolicyID, &o.PolicyIDSnapshot, &o.UserID, &o.Kind, &o.Status,
		&o.ReturnPolicyState, &o.IdempotencyKey, &o.RequestHash, &o.DryRunID,
		&o.SourceOperationID, &o.StartPolicyVersion, &o.GateClosedAt, &o.DrainDeadlineAt,
		&o.AppliedAt, &o.CompletedAt, &o.Report, &o.CreatedAt, &o.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &o, nil
}

func scanDryRun(row pgx.Row) (*models.InstanceFailoverDryRun, error) {
	var d models.InstanceFailoverDryRun
	err := row.Scan(
		&d.ID, &d.PolicyID, &d.PolicyIDSnapshot, &d.UserID, &d.Direction,
		&d.IdempotencyKey, &d.RequestHash, &d.PolicyVersion, &d.DryRunRef,
		&d.Status, &d.Report, &d.CreatedAt, &d.ExpiresAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &d, nil
}

func scanEvent(row pgx.Row) (*models.InstanceFailoverEvent, error) {
	var e models.InstanceFailoverEvent
	err := row.Scan(
		&e.ID, &e.PolicyID, &e.PolicyIDSnapshot, &e.OperationID, &e.DryRunID, &e.UserID,
		&e.EventType, &e.ActorType, &e.ActorUserID, &e.IdempotencyScope, &e.IdempotencyKey,
		&e.RequestHash, &e.PolicyVersion, &e.SnapshotBefore, &e.SnapshotAfter, &e.Report, &e.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &e, nil
}

// =============================================================================
// Leitura de política
// =============================================================================

func (r *instanceFailoverRepository) GetPolicy(ctx context.Context, policyID int64, userID string) (*models.InstanceFailoverPolicy, error) {
	return scanPolicy(r.db.pool.QueryRow(ctx,
		`SELECT `+policyColumns+` FROM instance_failover_policies WHERE id = $1 AND user_id = $2`,
		policyID, userID))
}

func (r *instanceFailoverRepository) ListPolicies(ctx context.Context, userID string) ([]models.InstanceFailoverPolicy, error) {
	rows, err := r.db.pool.Query(ctx,
		`SELECT `+policyColumns+` FROM instance_failover_policies WHERE user_id = $1 ORDER BY id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.InstanceFailoverPolicy
	for rows.Next() {
		p, err := scanPolicy(rows)
		if err != nil {
			return nil, err
		}
		if p != nil {
			out = append(out, *p)
		}
	}
	return out, rows.Err()
}

func (r *instanceFailoverRepository) GetRoleByInstance(ctx context.Context, instanceID int) (*repository.InstanceFailoverRole, error) {
	var role repository.InstanceFailoverRole
	err := r.db.pool.QueryRow(ctx,
		`SELECT instance_id, policy_id, user_id, role FROM instance_failover_roles WHERE instance_id = $1`,
		instanceID).Scan(&role.InstanceID, &role.PolicyID, &role.UserID, &role.Role)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &role, nil
}

func (r *instanceFailoverRepository) ListAssistedPolicies(ctx context.Context) ([]models.InstanceFailoverPolicy, error) {
	rows, err := r.db.pool.Query(ctx,
		`SELECT `+policyColumns+` FROM instance_failover_policies
		  WHERE mode = 'assisted' AND state IN ('active','proposed')
		  ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.InstanceFailoverPolicy
	for rows.Next() {
		p, err := scanPolicy(rows)
		if err != nil {
			return nil, err
		}
		if p != nil {
			out = append(out, *p)
		}
	}
	return out, rows.Err()
}

// =============================================================================
// Mutação de política
// =============================================================================

// findEventReceipt procura o recibo idempotente de SetPolicy/DisablePolicy.
func findEventReceipt(ctx context.Context, tx pgx.Tx, userID, scope, key string) (*models.InstanceFailoverEvent, error) {
	return scanEvent(tx.QueryRow(ctx,
		`SELECT `+eventColumns+` FROM instance_failover_events
		  WHERE user_id = $1 AND idempotency_scope = $2 AND idempotency_key = $3`,
		userID, scope, key))
}

func (r *instanceFailoverRepository) SetPolicy(ctx context.Context, in repository.SetFailoverPolicyInput) (*repository.FailoverPolicyMutation, error) {
	var out repository.FailoverPolicyMutation

	err := r.db.WithTx(ctx, func(tx pgx.Tx) error {
		if err := advisoryLockUser(ctx, tx, in.UserID); err != nil {
			return err
		}

		// Recibo idempotente: mesma chave + mesmo payload devolve o resultado
		// anterior sem mutar nada; payload diferente é AlreadyExists.
		receipt, err := findEventReceipt(ctx, tx, in.UserID, models.FailoverScopeSetPolicy, in.IdempotencyKey)
		if err != nil {
			return err
		}
		if receipt != nil {
			if receipt.RequestHash == nil || *receipt.RequestHash != in.RequestHash {
				return repository.ErrFailoverIdempotencyMismatch
			}
			out.Replayed = true
			out.Event = receipt
			if receipt.PolicyID != nil {
				out.Policy, err = scanPolicy(tx.QueryRow(ctx,
					`SELECT `+policyColumns+` FROM instance_failover_policies WHERE id = $1 AND user_id = $2`,
					*receipt.PolicyID, in.UserID))
				if err != nil {
					return err
				}
			}
			return nil
		}

		var policy *models.InstanceFailoverPolicy
		var snapshotBefore []byte
		eventType := models.FailoverEventPolicyCreated

		if in.PolicyID == 0 {
			// CREATE
			policy, err = scanPolicy(tx.QueryRow(ctx, `
				INSERT INTO instance_failover_policies
					(user_id, primary_instance_id, backup_instance_id, mode,
					 unhealthy_after_seconds, required_consecutive_probes,
					 drain_deadline_seconds, proposal_ttl_seconds)
				VALUES ($1,$2,$3,$4,
					COALESCE($5, 900), COALESCE($6, 3), COALESCE($7, 120), COALESCE($8, 1800))
				RETURNING `+policyColumns,
				in.UserID, in.PrimaryInstanceID, in.BackupInstanceID, in.Mode,
				in.UnhealthyAfterSeconds, in.RequiredConsecutiveProbes,
				in.DrainDeadlineSeconds, in.ProposalTTLSeconds))
			if err != nil {
				return err
			}
			if policy == nil {
				return errors.New("failover: insert policy returned no row")
			}

			for _, role := range []struct {
				instanceID int
				name       string
			}{
				{in.PrimaryInstanceID, "primary"},
				{in.BackupInstanceID, "backup"},
			} {
				if _, err := tx.Exec(ctx, `
					INSERT INTO instance_failover_roles (instance_id, policy_id, user_id, role)
					VALUES ($1,$2,$3,$4)`,
					role.instanceID, policy.ID, in.UserID, role.name); err != nil {
					return err
				}
			}
		} else {
			// UPDATE — a primária é IMUTÁVEL; só reserva/modo/limiares mudam.
			eventType = models.FailoverEventPolicyUpdated

			current, err := scanPolicy(tx.QueryRow(ctx,
				`SELECT `+policyColumns+` FROM instance_failover_policies
				  WHERE id = $1 AND user_id = $2 FOR UPDATE`,
				in.PolicyID, in.UserID))
			if err != nil {
				return err
			}
			if current == nil {
				return pgx.ErrNoRows
			}
			if current.Version != in.ExpectedVersion {
				return repository.ErrFailoverVersionConflict
			}
			snapshotBefore, _ = json.Marshal(current)

			// Atualizar uma proposta emite policy_updated, LIMPA a proposta e
			// volta a active: a evidência que a originou já não descreve a
			// configuração nova.
			policy, err = scanPolicy(tx.QueryRow(ctx, `
				UPDATE instance_failover_policies SET
					backup_instance_id          = $3,
					mode                        = $4,
					unhealthy_after_seconds     = COALESCE($5, unhealthy_after_seconds),
					required_consecutive_probes = COALESCE($6, required_consecutive_probes),
					drain_deadline_seconds      = COALESCE($7, drain_deadline_seconds),
					proposal_ttl_seconds        = COALESCE($8, proposal_ttl_seconds),
					state                       = 'active',
					proposal_expires_at         = NULL,
					primary_unhealthy_since     = NULL,
					consecutive_unhealthy_probes= 0,
					version                     = version + 1,
					updated_at                  = now()
				WHERE id = $1 AND user_id = $2
				  AND version = $9
				  AND state IN ('active','proposed')
				  AND current_operation_id IS NULL
				RETURNING `+policyColumns,
				in.PolicyID, in.UserID, in.BackupInstanceID, in.Mode,
				in.UnhealthyAfterSeconds, in.RequiredConsecutiveProbes,
				in.DrainDeadlineSeconds, in.ProposalTTLSeconds, in.ExpectedVersion))
			if err != nil {
				return err
			}
			if policy == nil {
				// Versão/estado mudaram entre a leitura e o UPDATE, ou existe
				// operação em curso.
				return repository.ErrFailoverVersionConflict
			}

			// Reescreve os papéis SEMPRE: o trigger da migração 077 exige que
			// role.backup aponte para policy.backup_instance_id, e é aqui que
			// a troca de reserva se materializa.
			if _, err := tx.Exec(ctx,
				`DELETE FROM instance_failover_roles WHERE policy_id = $1 AND role = 'backup'`,
				policy.ID); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO instance_failover_roles (instance_id, policy_id, user_id, role)
				VALUES ($1,$2,$3,'backup')`,
				in.BackupInstanceID, policy.ID, in.UserID); err != nil {
				return err
			}
		}

		snapshotAfter, _ := json.Marshal(policy)
		ev, err := insertEventTx(ctx, tx, &models.InstanceFailoverEvent{
			PolicyID:         &policy.ID,
			PolicyIDSnapshot: policy.ID,
			UserID:           in.UserID,
			EventType:        eventType,
			ActorType:        models.FailoverActorUser,
			ActorUserID:      failoverStrPtrOrNil(in.ActorUserID),
			IdempotencyScope: failoverStrPtr(models.FailoverScopeSetPolicy),
			IdempotencyKey:   failoverStrPtr(in.IdempotencyKey),
			RequestHash:      failoverStrPtr(in.RequestHash),
			PolicyVersion:    &policy.Version,
			SnapshotBefore:   snapshotBefore,
			SnapshotAfter:    snapshotAfter,
			Report:           []byte(`{}`),
		})
		if err != nil {
			return err
		}

		out.Policy = policy
		out.Event = ev
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *instanceFailoverRepository) DisablePolicy(ctx context.Context, in repository.DisableFailoverPolicyInput) (*repository.FailoverPolicyMutation, error) {
	var out repository.FailoverPolicyMutation

	err := r.db.WithTx(ctx, func(tx pgx.Tx) error {
		if err := advisoryLockUser(ctx, tx, in.UserID); err != nil {
			return err
		}

		receipt, err := findEventReceipt(ctx, tx, in.UserID, models.FailoverScopeDisablePolicy, in.IdempotencyKey)
		if err != nil {
			return err
		}
		if receipt != nil {
			if receipt.RequestHash == nil || *receipt.RequestHash != in.RequestHash {
				return repository.ErrFailoverIdempotencyMismatch
			}
			out.Replayed = true
			out.Event = receipt
			return nil
		}

		current, err := scanPolicy(tx.QueryRow(ctx,
			`SELECT `+policyColumns+` FROM instance_failover_policies
			  WHERE id = $1 AND user_id = $2 FOR UPDATE`,
			in.PolicyID, in.UserID))
		if err != nil {
			return err
		}
		if current == nil {
			return pgx.ErrNoRows
		}
		if current.Version != in.ExpectedVersion {
			return repository.ErrFailoverVersionConflict
		}
		// Em active_on_backup, desligar apagaria o único registro de qual era o
		// roteamento original. Reverter primeiro é obrigatório.
		if current.State != models.FailoverStateActive && current.State != models.FailoverStateProposed {
			return repository.ErrFailoverVersionConflict
		}

		snapshotBefore, _ := json.Marshal(current)

		// O evento entra ANTES do delete: a FK vira NULL por ON DELETE SET NULL
		// e policy_id_snapshot preserva a identidade histórica.
		ev, err := insertEventTx(ctx, tx, &models.InstanceFailoverEvent{
			PolicyID:         &current.ID,
			PolicyIDSnapshot: current.ID,
			UserID:           in.UserID,
			EventType:        models.FailoverEventPolicyDisabled,
			ActorType:        models.FailoverActorUser,
			ActorUserID:      failoverStrPtrOrNil(in.ActorUserID),
			IdempotencyScope: failoverStrPtr(models.FailoverScopeDisablePolicy),
			IdempotencyKey:   failoverStrPtr(in.IdempotencyKey),
			RequestHash:      failoverStrPtr(in.RequestHash),
			PolicyVersion:    &current.Version,
			SnapshotBefore:   snapshotBefore,
			Report:           []byte(`{}`),
		})
		if err != nil {
			return err
		}

		tag, err := tx.Exec(ctx,
			`DELETE FROM instance_failover_policies WHERE id = $1 AND user_id = $2 AND version = $3`,
			in.PolicyID, in.UserID, in.ExpectedVersion)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return repository.ErrFailoverVersionConflict
		}

		out.Event = ev
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// RecordProbe grava a telemetria do detector. Nunca incrementa `version`.
func (r *instanceFailoverRepository) RecordProbe(ctx context.Context, in repository.FailoverProbeInput) (*models.InstanceFailoverPolicy, *models.InstanceFailoverEvent, error) {
	var policy *models.InstanceFailoverPolicy
	var event *models.InstanceFailoverEvent

	err := r.db.WithTx(ctx, func(tx pgx.Tx) error {
		// O lock do DETECTOR vem antes do lock do usuário e é o único lugar do
		// sistema que o adquire — logo não pode fechar ciclo com nada. Perder a
		// disputa é normal (a outra réplica está sondando esta política agora):
		// sai em silêncio.
		if in.RequireDetectorLock {
			var got bool
			if err := tx.QueryRow(ctx,
				`SELECT pg_try_advisory_xact_lock(hashtextextended('instance-failover-detector:' || $1::text, 0))`,
				strconv.FormatInt(in.PolicyID, 10)).Scan(&got); err != nil {
				return err
			}
			if !got {
				return nil
			}
		}

		if err := advisoryLockUser(ctx, tx, in.UserID); err != nil {
			return err
		}

		current, err := scanPolicy(tx.QueryRow(ctx,
			`SELECT `+policyColumns+` FROM instance_failover_policies
			  WHERE id = $1 AND user_id = $2 FOR UPDATE`, in.PolicyID, in.UserID))
		if err != nil {
			return err
		}
		if current == nil {
			return nil
		}
		// O detector só age em estados estáveis sem operação em curso.
		if current.State != models.FailoverStateActive && current.State != models.FailoverStateProposed {
			policy = current
			return nil
		}

		// Probe saudável, ou indeterminado (WAHA fora do ar, payload
		// desconhecido, divergência DB×WAHA): zera a acumulação. Incerteza
		// NUNCA vira evidência de indisponibilidade.
		if in.Healthy || !in.Valid {
			newState := current.State
			recovered := false
			if in.Healthy && current.State == models.FailoverStateProposed {
				newState = models.FailoverStateActive
				recovered = true
			}
			policy, err = scanPolicy(tx.QueryRow(ctx, `
				UPDATE instance_failover_policies SET
					primary_unhealthy_since      = NULL,
					consecutive_unhealthy_probes = 0,
					last_probe_at                = $3,
					last_probe_status            = $4,
					state                        = $5,
					proposal_expires_at          = CASE WHEN $5 = 'active' THEN NULL ELSE proposal_expires_at END,
					updated_at                   = now()
				WHERE id = $1 AND user_id = $2
				RETURNING `+policyColumns,
				in.PolicyID, in.UserID, in.Now, in.Status, newState))
			if err != nil {
				return err
			}
			if recovered && policy != nil {
				event, err = insertEventTx(ctx, tx, &models.InstanceFailoverEvent{
					PolicyID:         &policy.ID,
					PolicyIDSnapshot: policy.ID,
					UserID:           in.UserID,
					EventType:        models.FailoverEventProposalRecovered,
					ActorType:        models.FailoverActorCron,
					PolicyVersion:    &policy.Version,
					Report:           []byte(`{}`),
				})
				if err != nil {
					return err
				}
			}
			return nil
		}

		// Probe VÁLIDO e não saudável: acumula.
		policy, err = scanPolicy(tx.QueryRow(ctx, `
			UPDATE instance_failover_policies SET
				primary_unhealthy_since      = COALESCE(primary_unhealthy_since, $3),
				consecutive_unhealthy_probes = consecutive_unhealthy_probes + 1,
				last_probe_at                = $3,
				last_probe_status            = $4,
				updated_at                   = now()
			WHERE id = $1 AND user_id = $2
			RETURNING `+policyColumns,
			in.PolicyID, in.UserID, in.Now, in.Status))
		if err != nil {
			return err
		}
		if policy == nil || policy.State != models.FailoverStateActive {
			return nil
		}

		// Limiares: duração contínua E contagem de probes. Ambos.
		if policy.PrimaryUnhealthySince == nil {
			return nil
		}
		elapsed := in.Now.Sub(*policy.PrimaryUnhealthySince)
		if elapsed < time.Duration(policy.UnhealthyAfterSeconds)*time.Second {
			return nil
		}
		if policy.ConsecutiveUnhealthyProbes < policy.RequiredConsecutiveProbes {
			return nil
		}

		expires := in.Now.Add(time.Duration(policy.ProposalTTLSeconds) * time.Second)
		policy, err = scanPolicy(tx.QueryRow(ctx, `
			UPDATE instance_failover_policies SET
				state               = 'proposed',
				proposal_expires_at = $3,
				updated_at          = now()
			WHERE id = $1 AND user_id = $2 AND state = 'active'
			RETURNING `+policyColumns,
			in.PolicyID, in.UserID, expires))
		if err != nil {
			return err
		}
		if policy == nil {
			return nil
		}

		// O evento guarda horários e a sequência, NUNCA texto de erro.
		report, _ := json.Marshal(map[string]any{
			"unhealthy_since":     policy.PrimaryUnhealthySince,
			"consecutive_probes":  policy.ConsecutiveUnhealthyProbes,
			"elapsed_seconds":     int(elapsed.Seconds()),
			"proposal_expires_at": expires,
			"last_probe_status":   in.Status,
		})
		event, err = insertEventTx(ctx, tx, &models.InstanceFailoverEvent{
			PolicyID:         &policy.ID,
			PolicyIDSnapshot: policy.ID,
			UserID:           in.UserID,
			EventType:        models.FailoverEventProposalCreated,
			ActorType:        models.FailoverActorCron,
			PolicyVersion:    &policy.Version,
			Report:           report,
		})
		return err
	})
	if err != nil {
		return nil, nil, err
	}
	return policy, event, nil
}

func (r *instanceFailoverRepository) ExpireProposals(ctx context.Context, now time.Time) (int, error) {
	rows, err := r.db.pool.Query(ctx, `
		UPDATE instance_failover_policies SET
			state               = 'active',
			proposal_expires_at = NULL,
			updated_at          = now()
		WHERE state = 'proposed'
		  AND proposal_expires_at IS NOT NULL
		  AND proposal_expires_at <= $1
		RETURNING id, user_id, version`, now)
	if err != nil {
		return 0, err
	}

	type expired struct {
		id      int64
		userID  string
		version int64
	}
	var list []expired
	for rows.Next() {
		var e expired
		if err := rows.Scan(&e.id, &e.userID, &e.version); err != nil {
			rows.Close()
			return 0, err
		}
		list = append(list, e)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	for _, e := range list {
		if _, err := r.InsertEvent(ctx, &models.InstanceFailoverEvent{
			PolicyID:         &e.id,
			PolicyIDSnapshot: e.id,
			UserID:           e.userID,
			EventType:        models.FailoverEventProposalExpired,
			ActorType:        models.FailoverActorCron,
			PolicyVersion:    &e.version,
			Report:           []byte(`{}`),
		}); err != nil {
			r.logger.Warn("failover: proposal expired but ledger write failed",
				zap.Int64("policyID", e.id), zap.Error(err))
		}
	}
	return len(list), nil
}

// =============================================================================
// Dry-run
// =============================================================================

func (r *instanceFailoverRepository) FindDryRunByKey(ctx context.Context, userID, direction, key string) (*models.InstanceFailoverDryRun, error) {
	return scanDryRun(r.db.pool.QueryRow(ctx,
		`SELECT `+dryRunColumns+` FROM instance_failover_dry_runs
		  WHERE user_id = $1 AND direction = $2 AND idempotency_key = $3`,
		userID, direction, key))
}

func (r *instanceFailoverRepository) GetDryRun(ctx context.Context, dryRunID, userID string) (*models.InstanceFailoverDryRun, error) {
	return scanDryRun(r.db.pool.QueryRow(ctx,
		`SELECT `+dryRunColumns+` FROM instance_failover_dry_runs WHERE id = $1 AND user_id = $2`,
		dryRunID, userID))
}

func (r *instanceFailoverRepository) ListDryRunItems(ctx context.Context, dryRunID string) ([]models.InstanceFailoverDryRunItem, error) {
	rows, err := r.db.pool.Query(ctx, `
		SELECT dry_run_id, automation_id, automation_id_snapshot, config_fingerprint,
		       snapshot_before, snapshot_after, required_source_groups, required_target_groups,
		       admin_required_groups, missing_groups, missing_admin_groups, result, reason_codes
		  FROM instance_failover_dry_run_items
		 WHERE dry_run_id = $1
		 ORDER BY automation_id_snapshot`, dryRunID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.InstanceFailoverDryRunItem
	for rows.Next() {
		var it models.InstanceFailoverDryRunItem
		if err := rows.Scan(&it.DryRunID, &it.AutomationID, &it.AutomationIDSnapshot,
			&it.ConfigFingerprint, &it.SnapshotBefore, &it.SnapshotAfter,
			&it.RequiredSourceGroups, &it.RequiredTargetGroups, &it.AdminRequiredGroups,
			&it.MissingGroups, &it.MissingAdminGroups, &it.Result, &it.ReasonCodes); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func (r *instanceFailoverRepository) SaveDryRun(ctx context.Context, dr *models.InstanceFailoverDryRun, items []models.InstanceFailoverDryRunItem, ev *models.InstanceFailoverEvent) error {
	return r.db.WithTx(ctx, func(tx pgx.Tx) error {
		var id string
		err := tx.QueryRow(ctx, `
			INSERT INTO instance_failover_dry_runs
				(policy_id, policy_id_snapshot, user_id, direction, idempotency_key,
				 request_hash, policy_version, dry_run_ref, status, report, expires_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
			RETURNING id`,
			dr.PolicyID, dr.PolicyIDSnapshot, dr.UserID, dr.Direction, dr.IdempotencyKey,
			dr.RequestHash, dr.PolicyVersion, dr.DryRunRef, dr.Status, dr.Report, dr.ExpiresAt).Scan(&id)
		if err != nil {
			return err
		}
		dr.ID = id

		for i := range items {
			it := &items[i]
			if _, err := tx.Exec(ctx, `
				INSERT INTO instance_failover_dry_run_items
					(dry_run_id, automation_id, automation_id_snapshot, config_fingerprint,
					 snapshot_before, snapshot_after, required_source_groups, required_target_groups,
					 admin_required_groups, missing_groups, missing_admin_groups, result, reason_codes)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
				id, it.AutomationID, it.AutomationIDSnapshot, it.ConfigFingerprint,
				it.SnapshotBefore, it.SnapshotAfter,
				nonNilStrings(it.RequiredSourceGroups), nonNilStrings(it.RequiredTargetGroups),
				nonNilStrings(it.AdminRequiredGroups), nonNilStrings(it.MissingGroups),
				nonNilStrings(it.MissingAdminGroups), it.Result, nonNilStrings(it.ReasonCodes)); err != nil {
				return err
			}
		}

		if ev != nil {
			ev.DryRunID = &id
			if _, err := insertEventTx(ctx, tx, ev); err != nil {
				return err
			}
		}
		return nil
	})
}

// =============================================================================
// Operações
// =============================================================================

func (r *instanceFailoverRepository) InsertOperation(ctx context.Context, op *models.InstanceFailoverOperation) (*models.InstanceFailoverOperation, bool, error) {
	var result *models.InstanceFailoverOperation
	replay := false

	err := r.db.WithTx(ctx, func(tx pgx.Tx) error {
		if err := advisoryLockUser(ctx, tx, op.UserID); err != nil {
			return err
		}

		existing, err := scanOperation(tx.QueryRow(ctx,
			`SELECT `+operationColumns+` FROM instance_failover_operations
			  WHERE user_id = $1 AND kind = $2 AND idempotency_key = $3`,
			op.UserID, op.Kind, op.IdempotencyKey))
		if err != nil {
			return err
		}
		if existing != nil {
			if existing.RequestHash != op.RequestHash {
				return repository.ErrFailoverIdempotencyMismatch
			}
			replay = true
			result = existing
			return nil
		}

		// Chave diferente concorrente perde no índice parcial único por
		// usuário — mapeado para Aborted na borda.
		other, err := scanOperation(tx.QueryRow(ctx,
			`SELECT `+operationColumns+` FROM instance_failover_operations
			  WHERE user_id = $1 AND status IN ('preparing','draining','verifying')
			  LIMIT 1`, op.UserID))
		if err != nil {
			return err
		}
		if other != nil {
			return repository.ErrFailoverConcurrentOperation
		}

		result, err = scanOperation(tx.QueryRow(ctx, `
			INSERT INTO instance_failover_operations
				(policy_id, policy_id_snapshot, user_id, kind, status, return_policy_state,
				 idempotency_key, request_hash, dry_run_id, source_operation_id,
				 start_policy_version, report)
			VALUES ($1,$2,$3,$4,'preparing',$5,$6,$7,$8,$9,$10,'{}'::jsonb)
			RETURNING `+operationColumns,
			op.PolicyID, op.PolicyIDSnapshot, op.UserID, op.Kind, op.ReturnPolicyState,
			op.IdempotencyKey, op.RequestHash, op.DryRunID, op.SourceOperationID,
			op.StartPolicyVersion))
		return err
	})
	if err != nil {
		return nil, false, err
	}
	return result, replay, nil
}

func (r *instanceFailoverRepository) GetOperation(ctx context.Context, operationID, userID string) (*models.InstanceFailoverOperation, error) {
	return scanOperation(r.db.pool.QueryRow(ctx,
		`SELECT `+operationColumns+` FROM instance_failover_operations WHERE id = $1 AND user_id = $2`,
		operationID, userID))
}

func (r *instanceFailoverRepository) FindOperationByKey(ctx context.Context, userID, kind, key string) (*models.InstanceFailoverOperation, error) {
	return scanOperation(r.db.pool.QueryRow(ctx,
		`SELECT `+operationColumns+` FROM instance_failover_operations
		  WHERE user_id = $1 AND kind = $2 AND idempotency_key = $3`, userID, kind, key))
}

func (r *instanceFailoverRepository) GetNonTerminalOperation(ctx context.Context, userID string) (*models.InstanceFailoverOperation, error) {
	return scanOperation(r.db.pool.QueryRow(ctx,
		`SELECT `+operationColumns+` FROM instance_failover_operations
		  WHERE user_id = $1 AND status IN ('preparing','draining','verifying') LIMIT 1`, userID))
}

func (r *instanceFailoverRepository) ListNonTerminalOperations(ctx context.Context) ([]models.InstanceFailoverOperation, error) {
	rows, err := r.db.pool.Query(ctx,
		`SELECT `+operationColumns+` FROM instance_failover_operations
		  WHERE status IN ('preparing','draining','verifying') ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.InstanceFailoverOperation
	for rows.Next() {
		op, err := scanOperation(rows)
		if err != nil {
			return nil, err
		}
		if op != nil {
			out = append(out, *op)
		}
	}
	return out, rows.Err()
}

func (r *instanceFailoverRepository) ListOperationItems(ctx context.Context, operationID string) ([]models.InstanceFailoverOperationItem, error) {
	rows, err := r.db.pool.Query(ctx, `
		SELECT operation_id, automation_id, automation_id_snapshot, fingerprint_before,
		       fingerprint_after, snapshot_before, snapshot_after, result
		  FROM instance_failover_operation_items
		 WHERE operation_id = $1
		 ORDER BY automation_id_snapshot`, operationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.InstanceFailoverOperationItem
	for rows.Next() {
		var it models.InstanceFailoverOperationItem
		if err := rows.Scan(&it.OperationID, &it.AutomationID, &it.AutomationIDSnapshot,
			&it.FingerprintBefore, &it.FingerprintAfter, &it.SnapshotBefore,
			&it.SnapshotAfter, &it.Result); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func (r *instanceFailoverRepository) GetLastCompletedOperation(ctx context.Context, policyID int64, userID, kind string) (*models.InstanceFailoverOperation, error) {
	return scanOperation(r.db.pool.QueryRow(ctx,
		`SELECT `+operationColumns+` FROM instance_failover_operations
		  WHERE policy_id_snapshot = $1 AND user_id = $2 AND kind = $3 AND status = 'completed'
		  ORDER BY completed_at DESC NULLS LAST, created_at DESC LIMIT 1`,
		policyID, userID, kind))
}

// CloseGate é a transação que FECHA o gate: a partir do commit, nenhum produtor
// consegue lease para as automações do snapshot.
func (r *instanceFailoverRepository) CloseGate(ctx context.Context, in repository.FailoverCloseGateInput) (*models.InstanceFailoverOperation, error) {
	var op *models.InstanceFailoverOperation

	err := r.db.WithTx(ctx, func(tx pgx.Tx) error {
		if err := advisoryLockUser(ctx, tx, in.UserID); err != nil {
			return err
		}

		policy, err := scanPolicy(tx.QueryRow(ctx,
			`SELECT `+policyColumns+` FROM instance_failover_policies
			  WHERE id = $1 AND user_id = $2 FOR UPDATE`, in.PolicyID, in.UserID))
		if err != nil {
			return err
		}
		if policy == nil {
			return pgx.ErrNoRows
		}
		if policy.Version != in.ExpectedVersion || policy.State != in.ExpectedState {
			return repository.ErrFailoverVersionConflict
		}
		if policy.CurrentOperationID != nil {
			return repository.ErrFailoverConcurrentOperation
		}

		op, err = scanOperation(tx.QueryRow(ctx,
			`SELECT `+operationColumns+` FROM instance_failover_operations
			  WHERE id = $1 AND user_id = $2 FOR UPDATE`, in.OperationID, in.UserID))
		if err != nil {
			return err
		}
		if op == nil {
			return pgx.ErrNoRows
		}
		if op.Status != models.FailoverOpPreparing {
			return repository.ErrFailoverConcurrentOperation
		}

		// Itens em ordem ascendente de UUID (ordem global de locks).
		items := append([]models.InstanceFailoverOperationItem(nil), in.Items...)
		sort.Slice(items, func(i, j int) bool {
			return items[i].AutomationIDSnapshot < items[j].AutomationIDSnapshot
		})
		for i := range items {
			it := &items[i]
			if _, err := tx.Exec(ctx, `
				INSERT INTO instance_failover_operation_items
					(operation_id, automation_id, automation_id_snapshot,
					 fingerprint_before, fingerprint_after, snapshot_before, snapshot_after, result)
				VALUES ($1,$2,$3,$4,$5,$6,$7,'pending')`,
				in.OperationID, it.AutomationID, it.AutomationIDSnapshot,
				it.FingerprintBefore, it.FingerprintAfter, it.SnapshotBefore, it.SnapshotAfter); err != nil {
				return err
			}
		}

		if _, err := tx.Exec(ctx, `
			UPDATE instance_failover_policies SET
				state = $3, current_operation_id = $4, version = version + 1, updated_at = now()
			WHERE id = $1 AND user_id = $2 AND version = $5`,
			in.PolicyID, in.UserID, in.NewState, in.OperationID, in.ExpectedVersion); err != nil {
			return err
		}

		op, err = scanOperation(tx.QueryRow(ctx, `
			UPDATE instance_failover_operations SET
				status = 'draining', gate_closed_at = now(), drain_deadline_at = $3, updated_at = now()
			WHERE id = $1 AND user_id = $2 AND status = 'preparing'
			RETURNING `+operationColumns,
			in.OperationID, in.UserID, in.DrainDeadlineAt))
		if err != nil {
			return err
		}
		if op == nil {
			return repository.ErrFailoverConcurrentOperation
		}

		for _, evType := range []string{in.RequestedEvent, in.GateClosedEvent} {
			if evType == "" {
				continue
			}
			if _, err := insertEventTx(ctx, tx, &models.InstanceFailoverEvent{
				PolicyID:         &in.PolicyID,
				PolicyIDSnapshot: in.PolicyID,
				OperationID:      &in.OperationID,
				UserID:           in.UserID,
				EventType:        evType,
				ActorType:        models.FailoverActorUser,
				ActorUserID:      failoverStrPtrOrNil(in.ActorUserID),
				PolicyVersion:    &policy.Version,
				Report:           []byte(`{"items":` + strconv.Itoa(len(items)) + `}`),
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return op, nil
}

// ApplyProjection escreve a projeção de TODAS as automações do snapshot numa
// única transação. Antes da primeira escrita, cada automação é travada em
// ordem de UUID e o fingerprint atual é recalculado: qualquer divergência
// aborta tudo com ErrFailoverStaleConfiguration.
func (r *instanceFailoverRepository) ApplyProjection(ctx context.Context, in repository.FailoverApplyInput) error {
	return r.db.WithTx(ctx, func(tx pgx.Tx) error {
		if err := advisoryLockUser(ctx, tx, in.UserID); err != nil {
			return err
		}

		policy, err := scanPolicy(tx.QueryRow(ctx,
			`SELECT `+policyColumns+` FROM instance_failover_policies
			  WHERE id = $1 AND user_id = $2 FOR UPDATE`, in.PolicyID, in.UserID))
		if err != nil {
			return err
		}
		if policy == nil {
			return pgx.ErrNoRows
		}
		if policy.State != in.ExpectedState {
			return repository.ErrFailoverVersionConflict
		}
		if policy.CurrentOperationID == nil || *policy.CurrentOperationID != in.OperationID {
			return repository.ErrFailoverVersionConflict
		}

		if _, err := tx.Exec(ctx,
			`SELECT id FROM instance_failover_operations WHERE id = $1 AND user_id = $2 FOR UPDATE`,
			in.OperationID, in.UserID); err != nil {
			return err
		}

		items := append([]models.InstanceFailoverOperationItem(nil), in.Items...)
		sort.Slice(items, func(i, j int) bool {
			return items[i].AutomationIDSnapshot < items[j].AutomationIDSnapshot
		})

		// Passo 1: travar e CONFERIR tudo. Nenhuma escrita ainda.
		type pending struct {
			item     *models.InstanceFailoverOperationItem
			target   *models.RoutingSnapshot
			existing bool
		}
		toWrite := make([]pending, 0, len(items))

		for i := range items {
			it := &items[i]

			row, err := lockAutomationRouting(ctx, tx, it.AutomationIDSnapshot, in.UserID)
			if err != nil {
				return err
			}
			if row == nil {
				// Automação apagada depois do gate: no-op explícito, nunca
				// recriada.
				toWrite = append(toWrite, pending{item: it, existing: false})
				continue
			}

			snap, reasons := models.NewRoutingSnapshot(&row.Automation)
			if len(reasons) > 0 {
				return fmt.Errorf("%w: automation %s: %s",
					repository.ErrFailoverStaleConfiguration, it.AutomationIDSnapshot, strings.Join(reasons, ","))
			}

			expected := it.FingerprintBefore
			raw := it.SnapshotAfter
			if in.ExpectFingerprintAfter {
				expected = it.FingerprintAfter
				raw = it.SnapshotBefore
			}
			if snap.Fingerprint() != expected {
				return fmt.Errorf("%w: automation %s", repository.ErrFailoverStaleConfiguration, it.AutomationIDSnapshot)
			}

			target, err := models.UnmarshalRoutingSnapshot(raw)
			if err != nil {
				return err
			}
			toWrite = append(toWrite, pending{item: it, target: target, existing: true})
		}

		// Passo 2: escrever. Só aqui, e só as três colunas de roteamento.
		for _, p := range toWrite {
			if !p.existing {
				if _, err := tx.Exec(ctx, `
					UPDATE instance_failover_operation_items SET result = 'deleted_noop'
					WHERE operation_id = $1 AND automation_id_snapshot = $2`,
					in.OperationID, p.item.AutomationIDSnapshot); err != nil {
					return err
				}
				continue
			}

			upd := p.target.ToRoutingUpdate()
			overrides, err := json.Marshal(upd.ChipOverrides)
			if err != nil {
				return err
			}
			var scalar *int
			if upd.InstanceID != nil {
				v, convErr := strconv.Atoi(*upd.InstanceID)
				if convErr != nil {
					return fmt.Errorf("failover: invalid scalar instance id in snapshot: %w", convErr)
				}
				scalar = &v
			}
			list := make([]int, 0, len(upd.InstanceIDs))
			for _, s := range upd.InstanceIDs {
				v, convErr := strconv.Atoi(s)
				if convErr != nil {
					return fmt.Errorf("failover: invalid instance id in snapshot: %w", convErr)
				}
				list = append(list, v)
			}

			tag, err := tx.Exec(ctx, `
				UPDATE automations SET instance_id = $3, instance_ids = $4, chip_overrides = $5
				WHERE id = $1 AND user_id = $2`,
				p.item.AutomationIDSnapshot, in.UserID, scalar, list, string(overrides))
			if err != nil {
				return err
			}
			if tag.RowsAffected() != 1 {
				return fmt.Errorf("%w: automation %s vanished mid-transaction",
					repository.ErrFailoverStaleConfiguration, p.item.AutomationIDSnapshot)
			}

			if _, err := tx.Exec(ctx, `
				UPDATE instance_failover_operation_items SET result = $3
				WHERE operation_id = $1 AND automation_id_snapshot = $2`,
				in.OperationID, p.item.AutomationIDSnapshot, in.ItemResult); err != nil {
				return err
			}
		}

		// A política permanece em switching/reverting: o gate SÓ abre na
		// finalização, depois da pós-verificação.
		newStatus := models.FailoverOpVerifying
		if _, err := tx.Exec(ctx, `
			UPDATE instance_failover_operations SET status = $3, applied_at = COALESCE(applied_at, now()), updated_at = now()
			WHERE id = $1 AND user_id = $2`,
			in.OperationID, in.UserID, newStatus); err != nil {
			return err
		}

		_, err = insertEventTx(ctx, tx, &models.InstanceFailoverEvent{
			PolicyID:         &in.PolicyID,
			PolicyIDSnapshot: in.PolicyID,
			OperationID:      &in.OperationID,
			UserID:           in.UserID,
			EventType:        in.Event,
			ActorType:        models.FailoverActorUser,
			ActorUserID:      failoverStrPtrOrNil(in.ActorUserID),
			PolicyVersion:    &policy.Version,
			Report:           []byte(`{"items":` + strconv.Itoa(len(toWrite)) + `}`),
		})
		return err
	})
}

// TerminateOperation leva a operação a um desfecho e devolve a política ao
// estado combinado (ou a deixa em failed com o gate fechado).
func (r *instanceFailoverRepository) TerminateOperation(ctx context.Context, in repository.FailoverTerminateInput) (*models.InstanceFailoverPolicy, error) {
	var policy *models.InstanceFailoverPolicy

	err := r.db.WithTx(ctx, func(tx pgx.Tx) error {
		if err := advisoryLockUser(ctx, tx, in.UserID); err != nil {
			return err
		}

		current, err := scanPolicy(tx.QueryRow(ctx,
			`SELECT `+policyColumns+` FROM instance_failover_policies
			  WHERE id = $1 AND user_id = $2 FOR UPDATE`, in.PolicyID, in.UserID))
		if err != nil {
			return err
		}

		if _, err := tx.Exec(ctx, `
			UPDATE instance_failover_operations SET
				status = $3,
				completed_at = CASE WHEN $3 IN ('completed','aborted','failed') THEN now() ELSE completed_at END,
				report = $4,
				updated_at = now()
			WHERE id = $1 AND user_id = $2`,
			in.OperationID, in.UserID, in.OperationStatus, jsonOrEmpty(in.Report)); err != nil {
			return err
		}

		if current != nil {
			switch {
			case in.PolicyState == models.FailoverStateFailed:
				// Gate CONTINUA fechado: current_operation_id permanece.
				policy, err = scanPolicy(tx.QueryRow(ctx, `
					UPDATE instance_failover_policies SET state = 'failed', version = version + 1, updated_at = now()
					WHERE id = $1 AND user_id = $2 RETURNING `+policyColumns,
					in.PolicyID, in.UserID))
			case in.PolicyState != "":
				// A guarda `(state <> $3 OR current_operation_id IS NOT NULL)`
				// evita incrementar `version` num no-op. Versão é o optimistic
				// lock do wire: subi-la à toa invalidaria dry-runs em
				// andamento sem nada ter mudado.
				policy, err = scanPolicy(tx.QueryRow(ctx, `
					UPDATE instance_failover_policies SET
						state = $3, current_operation_id = NULL, version = version + 1, updated_at = now()
					WHERE id = $1 AND user_id = $2
					  AND (state <> $3 OR current_operation_id IS NOT NULL)
					RETURNING `+policyColumns,
					in.PolicyID, in.UserID, in.PolicyState))
				if err == nil && policy == nil {
					policy = current
				}
			default:
				policy = current
			}
			if err != nil {
				return err
			}
		}

		report := in.Report
		if len(report) == 0 {
			payload, _ := json.Marshal(map[string]any{"reason_codes": in.ReasonCodes})
			report = payload
		}
		var version *int64
		if policy != nil {
			version = &policy.Version
		}
		_, err = insertEventTx(ctx, tx, &models.InstanceFailoverEvent{
			PolicyID:         policyIDPtrIfExists(policy, in.PolicyID),
			PolicyIDSnapshot: in.PolicyID,
			OperationID:      &in.OperationID,
			UserID:           in.UserID,
			EventType:        in.Event,
			ActorType:        models.FailoverActorSystem,
			ActorUserID:      failoverStrPtrOrNil(in.ActorUserID),
			PolicyVersion:    version,
			Report:           report,
		})
		return err
	})
	if err != nil {
		return nil, err
	}
	return policy, nil
}

// =============================================================================
// Gate de dispatch (leases)
// =============================================================================

// AcquireLease é FAIL-CLOSED: se a automação está numa operação não terminal,
// devolve ErrFailoverInProgress e o produtor não inicia trabalho nenhum.
func (r *instanceFailoverRepository) AcquireLease(ctx context.Context, in repository.FailoverLeaseInput) (string, error) {
	var leaseID string

	err := r.db.WithTx(ctx, func(tx pgx.Tx) error {
		if err := advisoryLockUser(ctx, tx, in.UserID); err != nil {
			return err
		}

		var blocked bool
		if err := tx.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1
				  FROM instance_failover_operation_items i
				  JOIN instance_failover_operations o ON o.id = i.operation_id
				 WHERE o.user_id = $1
				   AND o.status IN ('preparing','draining','verifying')
				   AND i.automation_id_snapshot = $2
			)`, in.UserID, in.AutomationID).Scan(&blocked); err != nil {
			return err
		}
		if blocked {
			return repository.ErrFailoverInProgress
		}

		return tx.QueryRow(ctx, `
			INSERT INTO instance_failover_dispatch_leases
				(user_id, automation_id, holder_id, expires_at)
			VALUES ($1,$2,$3, now() + $4::interval)
			RETURNING id`,
			in.UserID, in.AutomationID, in.HolderID, intervalArg(in.TTL)).Scan(&leaseID)
	})
	if err != nil {
		return "", err
	}
	return leaseID, nil
}

// RenewLease só sucede enquanto a lease está viva e não liberada. Zero linhas
// significa que o dono perdeu a lease — o chamador DEVE cancelar o contexto de
// execução (fail-closed).
func (r *instanceFailoverRepository) RenewLease(ctx context.Context, leaseID string, ttl time.Duration) (bool, error) {
	tag, err := r.db.pool.Exec(ctx, `
		UPDATE instance_failover_dispatch_leases SET
			heartbeat_at = now(), expires_at = now() + $2::interval
		WHERE id = $1 AND released_at IS NULL AND expires_at > now()`,
		leaseID, intervalArg(ttl))
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (r *instanceFailoverRepository) ReleaseLease(ctx context.Context, leaseID string) error {
	_, err := r.db.pool.Exec(ctx,
		`UPDATE instance_failover_dispatch_leases SET released_at = now()
		  WHERE id = $1 AND released_at IS NULL`, leaseID)
	return err
}

func (r *instanceFailoverRepository) CountActiveLeases(ctx context.Context, automationIDs []string, now time.Time) (int, error) {
	if len(automationIDs) == 0 {
		return 0, nil
	}
	var count int
	err := r.db.pool.QueryRow(ctx, `
		SELECT count(*) FROM instance_failover_dispatch_leases
		 WHERE automation_id = ANY($1::uuid[]) AND released_at IS NULL AND expires_at > $2`,
		automationIDs, now).Scan(&count)
	return count, err
}

func (r *instanceFailoverRepository) CleanupLeases(ctx context.Context, before time.Time) (int64, error) {
	tag, err := r.db.pool.Exec(ctx, `
		DELETE FROM instance_failover_dispatch_leases
		 WHERE (released_at IS NOT NULL AND released_at < $1) OR expires_at < $1`, before)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (r *instanceFailoverRepository) CountNonTerminalDispatches(ctx context.Context, automationIDs []string, before time.Time) (int, error) {
	if len(automationIDs) == 0 {
		return 0, nil
	}
	var count int
	err := r.db.pool.QueryRow(ctx, `
		SELECT count(*) FROM dispatch_logs
		 WHERE automation_id = ANY($1::uuid[])
		   AND status IN ('pending','queued')
		   AND created_at <= $2`, automationIDs, before).Scan(&count)
	return count, err
}

// =============================================================================
// Automações (leitura de roteamento)
// =============================================================================

const automationRoutingColumns = `
	id, user_id, name, type, status, group_ids, send_groups, monitor_group_ids,
	instance_id, instance_ids, chip_overrides`

func scanAutomationRouting(row pgx.Row) (*repository.FailoverAutomationRouting, error) {
	var out repository.FailoverAutomationRouting
	var automationType *string
	var groupIDsJSON, sendGroupsJSON, monitorGroupIDsJSON, chipOverridesJSON []byte
	var instanceID *int
	var instanceIDs []int

	err := row.Scan(&out.Automation.ID, &out.Automation.UserID, &out.Name, &automationType,
		&out.Status, &groupIDsJSON, &sendGroupsJSON, &monitorGroupIDsJSON,
		&instanceID, &instanceIDs, &chipOverridesJSON)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	out.Automation.Name = out.Name
	out.Automation.Status = out.Status
	out.Automation.Type = automationType
	out.Automation.GroupIDs = unmarshalStringArray(groupIDsJSON)
	out.Automation.SendGroups = unmarshalStringArray(sendGroupsJSON)
	out.Automation.MonitorGroupIDs = unmarshalStringArray(monitorGroupIDsJSON)
	if instanceID != nil {
		s := strconv.Itoa(*instanceID)
		out.Automation.InstanceID = &s
	}
	for _, v := range instanceIDs {
		out.Automation.InstanceIDs = append(out.Automation.InstanceIDs, strconv.Itoa(v))
	}
	if len(chipOverridesJSON) > 0 {
		var overrides map[string]string
		if err := json.Unmarshal(chipOverridesJSON, &overrides); err == nil {
			out.Automation.ChipOverrides = overrides
		}
	}
	return &out, nil
}

// lockAutomationRouting trava UMA automação e devolve seu roteamento.
// nil sem erro = automação apagada.
func lockAutomationRouting(ctx context.Context, tx pgx.Tx, automationID, userID string) (*repository.FailoverAutomationRouting, error) {
	return scanAutomationRouting(tx.QueryRow(ctx,
		`SELECT `+automationRoutingColumns+` FROM automations
		  WHERE id = $1 AND user_id = $2 FOR UPDATE`, automationID, userID))
}

func (r *instanceFailoverRepository) ListUserAutomationsRouting(ctx context.Context, userID string) ([]repository.FailoverAutomationRouting, error) {
	// TODAS as automações do usuário, em QUALQUER status: uma automação pausada
	// hoje volta amanhã apontando para o chip antigo se ficar fora do snapshot.
	rows, err := r.db.pool.Query(ctx,
		`SELECT `+automationRoutingColumns+` FROM automations WHERE user_id = $1 ORDER BY id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []repository.FailoverAutomationRouting
	for rows.Next() {
		item, err := scanAutomationRouting(rows)
		if err != nil {
			return nil, err
		}
		if item != nil {
			out = append(out, *item)
		}
	}
	return out, rows.Err()
}

func (r *instanceFailoverRepository) FindBlockingOperationForAutomations(ctx context.Context, userID string, automationIDs []string) (*models.InstanceFailoverOperation, error) {
	if len(automationIDs) == 0 {
		// Create implícito durante operação: qualquer operação não terminal do
		// usuário bloqueia, porque a automação nova nasceria fora do snapshot.
		return r.GetNonTerminalOperation(ctx, userID)
	}
	return scanOperation(r.db.pool.QueryRow(ctx, `
		SELECT `+prefixColumns(operationColumns, "o")+`
		  FROM instance_failover_operations o
		  JOIN instance_failover_operation_items i ON i.operation_id = o.id
		 WHERE o.user_id = $1
		   AND o.status IN ('preparing','draining','verifying')
		   AND i.automation_id_snapshot = ANY($2::uuid[])
		 LIMIT 1`, userID, automationIDs))
}

// =============================================================================
// Ledger
// =============================================================================

func insertEventTx(ctx context.Context, tx pgx.Tx, ev *models.InstanceFailoverEvent) (*models.InstanceFailoverEvent, error) {
	return scanEvent(tx.QueryRow(ctx, `
		INSERT INTO instance_failover_events
			(policy_id, policy_id_snapshot, operation_id, dry_run_id, user_id, event_type,
			 actor_type, actor_user_id, idempotency_scope, idempotency_key, request_hash,
			 policy_version, snapshot_before, snapshot_after, report)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
		RETURNING `+eventColumns,
		ev.PolicyID, ev.PolicyIDSnapshot, ev.OperationID, ev.DryRunID, ev.UserID, ev.EventType,
		ev.ActorType, ev.ActorUserID, ev.IdempotencyScope, ev.IdempotencyKey, ev.RequestHash,
		ev.PolicyVersion, failoverNilIfEmpty(ev.SnapshotBefore), failoverNilIfEmpty(ev.SnapshotAfter), jsonOrEmpty(ev.Report)))
}

func (r *instanceFailoverRepository) InsertEvent(ctx context.Context, ev *models.InstanceFailoverEvent) (*models.InstanceFailoverEvent, error) {
	var out *models.InstanceFailoverEvent
	err := r.db.WithTx(ctx, func(tx pgx.Tx) error {
		var err error
		out, err = insertEventTx(ctx, tx, ev)
		return err
	})
	return out, err
}

func (r *instanceFailoverRepository) GetLastEvent(ctx context.Context, policyID int64, userID string) (*models.InstanceFailoverEvent, error) {
	return scanEvent(r.db.pool.QueryRow(ctx,
		`SELECT `+eventColumns+` FROM instance_failover_events
		  WHERE policy_id_snapshot = $1 AND user_id = $2
		  ORDER BY created_at DESC, id DESC LIMIT 1`, policyID, userID))
}

func (r *instanceFailoverRepository) ListEvents(ctx context.Context, in repository.ListFailoverEventsInput) ([]models.InstanceFailoverEvent, string, error) {
	limit := in.Limit
	if limit <= 0 || limit > 100 {
		limit = 100
	}

	query := `SELECT ` + eventColumns + ` FROM instance_failover_events
	           WHERE policy_id_snapshot = $1 AND user_id = $2`
	args := []any{in.PolicyID, in.UserID}

	if in.Cursor != "" {
		createdAt, id, err := decodeEventCursor(in.Cursor)
		if err != nil {
			return nil, "", err
		}
		query += ` AND (created_at, id) < ($3, $4)`
		args = append(args, createdAt, id)
	}
	query += ` ORDER BY created_at DESC, id DESC LIMIT ` + strconv.Itoa(limit+1)

	rows, err := r.db.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var out []models.InstanceFailoverEvent
	for rows.Next() {
		ev, err := scanEvent(rows)
		if err != nil {
			return nil, "", err
		}
		if ev != nil {
			out = append(out, *ev)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}

	next := ""
	if len(out) > limit {
		last := out[limit-1]
		next = encodeEventCursor(last.CreatedAt, last.ID)
		out = out[:limit]
	}
	return out, next, nil
}

func encodeEventCursor(createdAt time.Time, id int64) string {
	return base64.RawURLEncoding.EncodeToString(
		[]byte(createdAt.UTC().Format(time.RFC3339Nano) + "|" + strconv.FormatInt(id, 10)))
}

func decodeEventCursor(cursor string) (time.Time, int64, error) {
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, 0, fmt.Errorf("failover: invalid cursor")
	}
	parts := strings.SplitN(string(raw), "|", 2)
	if len(parts) != 2 {
		return time.Time{}, 0, fmt.Errorf("failover: invalid cursor")
	}
	ts, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return time.Time{}, 0, fmt.Errorf("failover: invalid cursor")
	}
	id, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return time.Time{}, 0, fmt.Errorf("failover: invalid cursor")
	}
	return ts, id, nil
}

// =============================================================================
// Helpers locais
// =============================================================================

func failoverStrPtr(s string) *string { return &s }

func failoverStrPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func policyIDPtrIfExists(p *models.InstanceFailoverPolicy, fallback int64) *int64 {
	if p != nil {
		return &p.ID
	}
	// Política já removida: a FK vira NULL e policy_id_snapshot preserva a
	// identidade.
	_ = fallback
	return nil
}

func nonNilStrings(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}

func failoverNilIfEmpty(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return b
}

func jsonOrEmpty(b []byte) []byte {
	if len(b) == 0 {
		return []byte(`{}`)
	}
	return b
}

func intervalArg(d time.Duration) string {
	if d <= 0 {
		d = time.Minute
	}
	return strconv.Itoa(int(d.Seconds())) + " seconds"
}

// unmarshalStringArray lê as colunas JSONB de grupos (group_ids, send_groups,
// monitor_group_ids). Payload inválido devolve lista vazia — o snapshot então
// acusa roteamento implícito/escopo vazio em vez de inventar grupos.
func unmarshalStringArray(raw []byte) []string {
	if len(raw) == 0 {
		return nil
	}
	var out []string
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil
	}
	return out
}

// prefixColumns qualifica uma lista de colunas com um alias de tabela.
func prefixColumns(columns, alias string) string {
	parts := strings.Split(columns, ",")
	for i, p := range parts {
		parts[i] = alias + "." + strings.TrimSpace(p)
	}
	return strings.Join(parts, ", ")
}
