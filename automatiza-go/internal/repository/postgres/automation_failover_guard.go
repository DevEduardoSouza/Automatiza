package postgres

import (
	"context"
	"errors"
	"sync/atomic"

	"automatiza-go/internal/config"
	"automatiza-go/internal/repository"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// =============================================================================
// Guard de mutação de automação (Melhoria 5, seção 6.3)
//
// Enquanto uma troca de chip está em curso, o conjunto de automações que ela vai
// alterar já está congelado no snapshot. Deixar o usuário editar esse conjunto
// no meio do caminho produziria um de dois resultados ruins: a operação aborta
// em `stale_configuration` (correto, mas o usuário não entende por quê), ou uma
// automação nasce fora do snapshot ainda apontando para o chip antigo.
//
// O guard roda na MESMA transação da mutação, atrás do advisory lock do
// usuário, para que não exista janela entre "verifiquei" e "escrevi".
// =============================================================================

// failoverGuardArmed espelha, no pacote de dados, o mesmo mecanismo de
// segurança do DispatchGate: com a feature desligada mas uma operação ainda não
// terminal, o guard continua valendo. Armado/desarmado pelo Recovery.
var failoverGuardArmed atomic.Bool

// SetFailoverGuardArmed liga/desliga o guard independentemente da feature flag.
func SetFailoverGuardArmed(v bool) { failoverGuardArmed.Store(v) }

// failoverGuardActive reporta se o guard deve fazer I/O. Com a feature
// desligada e nada pendente, toda mutação de automação segue exatamente pelo
// caminho antigo — sem transação extra, sem lock, sem query.
func failoverGuardActive() bool {
	return config.InstanceFailoverEnabled() || failoverGuardArmed.Load()
}

// routingMutationKeys são os campos cuja alteração muda escopo, cobertura ou
// roteamento — os mesmos que entram no fingerprint. Um update que só mexe em
// schedule, status ou contadores NÃO aciona o guard: o cron de automações
// escreve last_run_at/next_run_at a cada minuto e não pode ficar preso atrás de
// um advisory lock.
var routingMutationKeys = []string{
	"type", "group_ids", "send_groups", "monitor_group_ids",
	"instance_id", "instance_ids", "chip_overrides",
}

// updateTouchesRouting reporta se um update map mexe em roteamento/escopo.
func updateTouchesRouting(updates map[string]interface{}) bool {
	for _, key := range routingMutationKeys {
		if _, ok := updates[key]; ok {
			return true
		}
	}
	return false
}

// pgExecutor é o mínimo comum entre *pgxpool.Pool e pgx.Tx, para que a mesma
// mutação rode dentro ou fora de transação sem duplicar o SQL.
type pgExecutor interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// automationGuardScope descreve o que está sendo mutado.
type automationGuardScope struct {
	// userID conhecido pelo chamador (create/delete). Vazio no update, que o
	// resolve a partir do automationID.
	userID string
	// automationID vazio = create.
	automationID string
	// kind decide a severidade da checagem.
	kind automationMutationKind
}

type automationMutationKind int

const (
	automationMutationCreate automationMutationKind = iota
	automationMutationUpdateRouting
	automationMutationDelete
)

// runGuardedAutomationMutation executa `mutate` atrás do guard.
func (r *automationRepository) runGuardedAutomationMutation(
	ctx context.Context,
	scope automationGuardScope,
	mutate func(ex pgExecutor) error,
) error {
	if !failoverGuardActive() {
		return mutate(r.db.pool)
	}

	return r.db.WithTx(ctx, func(tx pgx.Tx) error {
		userID := scope.userID
		if userID == "" && scope.automationID != "" {
			err := tx.QueryRow(ctx, `SELECT user_id FROM automations WHERE id = $1`, scope.automationID).Scan(&userID)
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					// Automação inexistente: não há operação que a proteja.
					return mutate(tx)
				}
				return err
			}
		}
		if userID == "" {
			return mutate(tx)
		}

		if err := advisoryLockUser(ctx, tx, userID); err != nil {
			return err
		}
		if err := assertNoBlockingFailover(ctx, tx, userID, scope); err != nil {
			return err
		}
		return mutate(tx)
	})
}

// assertNoBlockingFailover devolve ErrFailoverInProgress quando a mutação
// colide com uma operação não terminal.
//
// A regra é deliberadamente mais restritiva que o mínimo necessário para
// create e update:
//
//   - CREATE: qualquer operação não terminal do usuário bloqueia. Uma automação
//     nova nasce fora do snapshot; se ela apontar para a primária (ou cair no
//     roteamento implícito), continua enviando pelo chip trocado sem aparecer em
//     relatório nenhum.
//   - UPDATE de roteamento: idem. Distinguir "este update encostaria no chip da
//     operação" exigiria projetar o estado resultante aqui dentro, e errar essa
//     projeção é pior do que segurar por ~2 minutos um update raro.
//   - DELETE: bloqueia só quando a automação É item da operação. Apagar
//     qualquer outra é inofensivo.
func assertNoBlockingFailover(ctx context.Context, tx pgx.Tx, userID string, scope automationGuardScope) error {
	if scope.kind == automationMutationDelete && scope.automationID != "" {
		var blocked bool
		if err := tx.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1
				  FROM instance_failover_operation_items i
				  JOIN instance_failover_operations o ON o.id = i.operation_id
				 WHERE o.user_id = $1
				   AND o.status IN ('preparing','draining','verifying')
				   AND i.automation_id_snapshot = $2
			)`, userID, scope.automationID).Scan(&blocked); err != nil {
			return err
		}
		if blocked {
			return repository.ErrFailoverInProgress
		}
		return nil
	}

	var blocked bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM instance_failover_operations
			 WHERE user_id = $1 AND status IN ('preparing','draining','verifying')
		)`, userID).Scan(&blocked); err != nil {
		return err
	}
	if blocked {
		return repository.ErrFailoverInProgress
	}
	return nil
}
