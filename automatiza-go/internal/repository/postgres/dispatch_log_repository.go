package postgres

import (
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

type dispatchLogRepository struct {
	db     *DB
	logger *zap.Logger
}

// NewDispatchLogRepository creates a new PostgreSQL dispatch log repository
func NewDispatchLogRepository(db *DB, logger *zap.Logger) repository.DispatchLogRepository {
	return &dispatchLogRepository{
		db:     db,
		logger: logger,
	}
}

func (r *dispatchLogRepository) Create(ctx context.Context, log *models.DispatchLog) (*models.DispatchLog, error) {
	query := `
		INSERT INTO dispatch_logs (
			user_id, automation_id, automation_name, store, group_id,
			product_url, product_page_url, canonical_key, status, type, created_at, copy_index,
			affiliate_link, error, list_item_id,
			coupon_id, coupon_code, coupon_applied, coupon_skip_reason
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)
		RETURNING id, created_at
	`

	now := time.Now().UTC()
	var id int
	var createdAt time.Time

	err := r.db.QueryRow(ctx, query,
		log.UserID, log.AutomationID, log.AutomationName, log.Store, log.GroupID,
		log.ProductURL, log.ProductPageURL, log.CanonicalKey, log.Status, log.Type, now, log.CopyIndex,
		log.AffiliateLink, log.Error, log.ListItemID,
		log.CouponID, log.CouponCode, log.CouponApplied, log.CouponSkipReason,
	).Scan(&id, &createdAt)

	if err != nil {
		// FK violation (automation deleted between check and insert) — non-critical
		if strings.Contains(err.Error(), "violates foreign key constraint") {
			r.logger.Warn("dispatch_log FK violation — automation likely deleted",
				zap.String("userID", log.UserID),
				zap.Error(err))
			return nil, nil
		}
		r.logger.Error("Failed to create dispatch log",
			zap.String("userID", log.UserID),
			zap.Error(err))
		return nil, err
	}

	log.ID = id
	log.CreatedAt = createdAt.Format(time.RFC3339)
	return log, nil
}

func (r *dispatchLogRepository) Update(ctx context.Context, logID int, updates map[string]interface{}) error {
	// Build dynamic UPDATE query
	setClauses := "status = $1"
	args := []interface{}{updates["status"]}
	argIndex := 2

	if errMsg, ok := updates["error"]; ok {
		setClauses += ", error = $" + string(rune('0'+argIndex))
		args = append(args, errMsg)
		argIndex++
	}

	args = append(args, logID)
	query := "UPDATE dispatch_logs SET " + setClauses + " WHERE id = $" + string(rune('0'+argIndex))

	_, err := r.db.pool.Exec(ctx, query, args...)
	if err != nil {
		r.logger.Error("Failed to update dispatch log",
			zap.Int("logID", logID),
			zap.Error(err))
		return err
	}

	return nil
}

// MarkStaleQueuedListLogs marca como 'error' dispatch de LIST preso em 'queued'
// há mais de staleAfter dentro da janela window. Um 'queued' legítimo vive
// minutos (fila → worker → sent/failed); horas em 'queued' = task perdida
// (redis reiniciou, worker morreu com a task em voo além do lease). Sem este
// desfecho terminal, o item de lista associado ficava consumido para sempre
// sem nunca ter sido entregue — invisível para o reconciliador.
func (r *dispatchLogRepository) MarkStaleQueuedListLogs(ctx context.Context, staleAfter, window time.Duration) (int64, error) {
	query := `
		UPDATE dispatch_logs
		SET status = 'error', error = 'reconciliacao: envio perdido (queued sem desfecho)'
		WHERE list_item_id IS NOT NULL
		  AND status = 'queued'
		  AND created_at < NOW() - make_interval(secs => $1)
		  AND created_at > NOW() - make_interval(secs => $2)
	`

	result, err := r.db.pool.Exec(ctx, query, staleAfter.Seconds(), window.Seconds())
	if err != nil {
		r.logger.Error("Failed to mark stale queued list logs", zap.Error(err))
		return 0, fmt.Errorf("mark stale queued list logs: %w", err)
	}
	return result.RowsAffected(), nil
}

// MarkStaleQueuedOrphans marca como 'reconciled' os dispatch presos em 'queued'
// há mais de staleAfter (dentro de window) que NÃO são LIST-com-item — o gap que
// o MarkStaleQueuedListLogs (list_item_id IS NOT NULL) deixa aberto: search,
// monitoring/offer e os dispatch de LIST sem list_item_id. Um 'queued' legítimo
// vive minutos (fila → worker → sent/failed); horas em 'queued' = task perdida
// (worker morto além do lease, ou restart de deploy com o job em voo — o update
// final nunca rodou). Sem desfecho terminal o log ficava 'queued' para sempre,
// poluindo o painel de suporte (que conta 'queued' como backlog do usuário) e a
// contagem de "Enviadas".
//
// Marca 'reconciled', NÃO 'sent': o desfecho é DESCONHECIDO — a msg PODE ter
// saído e só o update de 'sent' se perdeu (DB saturado no timeout do
// updateDispatchLog). Afirmar 'sent' supercontaria entregas e, como 'sent' conta
// no dedup, poderia mascarar não-entregas; 'reconciled' é o status neutro de
// backfill e — ao contrário de 'error', usado pelo path LIST p/ alimentar o
// resurrect — não entra na golden query de falhas. list_item_id IS NULL garante
// que não pisamos no território do reconciliador de LIST. Planeja via
// idx_dispatch_logs_status (só varre as linhas 'queued', não a tabela toda).
// staleAfter deve ficar acima do staleMessageTTL (90min) do worker p/ nunca
// competir com um descarte legítimo em andamento.
func (r *dispatchLogRepository) MarkStaleQueuedOrphans(ctx context.Context, staleAfter, window time.Duration) (int64, error) {
	query := `
		UPDATE dispatch_logs
		SET status = 'reconciled', error = 'reconciliacao: queued sem desfecho (task perdida)'
		WHERE list_item_id IS NULL
		  AND status = 'queued'
		  AND created_at < NOW() - make_interval(secs => $1)
		  AND created_at > NOW() - make_interval(secs => $2)
	`

	result, err := r.db.pool.Exec(ctx, query, staleAfter.Seconds(), window.Seconds())
	if err != nil {
		r.logger.Error("Failed to mark stale queued orphans", zap.Error(err))
		return 0, fmt.Errorf("mark stale queued orphans: %w", err)
	}
	return result.RowsAffected(), nil
}

// GetListResurrectCandidates devolve os destinos FALHOS de itens de lista que
// foram consumidos mas cuja entrega comprovadamente NÃO aconteceu: dentro da
// janela, o item tem 0 'sent', 0 'queued' (nada em voo), ≥1 'failed' (desfecho
// TERMINAL escrito pelo worker — prova positiva de não-entrega) e 0 'error'.
//
// 'error' fica de fora dos gatilhos E veta o item: é o status que o
// MarkStaleQueuedListLogs grava em 'queued' órfão — desfecho DESCONHECIDO
// (a task pode ter enviado e só o update de 'sent' se perdeu, ex.: DB
// saturado no timeout de 5s do updateDispatchLog). Devolver um item nessa
// condição reenviaria um produto possivelmente já entregue — duplicar é pior
// que perder o retry. Só itens ainda consumidos entram (JOIN consumed_at
// IS NOT NULL), evitando re-processar o mesmo item a cada ciclo.
func (r *dispatchLogRepository) GetListResurrectCandidates(ctx context.Context, window time.Duration) ([]repository.ListResurrectCandidate, error) {
	query := `
		WITH agg AS (
			SELECT list_item_id,
			       COUNT(*) FILTER (WHERE status = 'sent')   AS sent,
			       COUNT(*) FILTER (WHERE status = 'queued') AS queued,
			       COUNT(*) FILTER (WHERE status = 'failed') AS failed,
			       COUNT(*) FILTER (WHERE status = 'error')  AS uncertain
			FROM dispatch_logs
			WHERE list_item_id IS NOT NULL
			  AND created_at > NOW() - make_interval(secs => $1)
			GROUP BY list_item_id
		)
		SELECT dl.list_item_id::text, dl.user_id, dl.group_id, dl.canonical_key
		FROM dispatch_logs dl
		JOIN agg ON agg.list_item_id = dl.list_item_id
		JOIN ml_link_list_items i ON i.id = dl.list_item_id AND i.consumed_at IS NOT NULL
		WHERE agg.sent = 0 AND agg.queued = 0 AND agg.failed > 0 AND agg.uncertain = 0
		  AND dl.status = 'failed'
		  AND dl.created_at > NOW() - make_interval(secs => $1)
		LIMIT 500
	`

	rows, err := r.db.Query(ctx, query, window.Seconds())
	if err != nil {
		r.logger.Error("Failed to get list resurrect candidates", zap.Error(err))
		return nil, fmt.Errorf("get list resurrect candidates: %w", err)
	}
	defer rows.Close()

	var out []repository.ListResurrectCandidate
	for rows.Next() {
		var c repository.ListResurrectCandidate
		if err := rows.Scan(&c.ListItemID, &c.UserID, &c.GroupID, &c.CanonicalKey); err != nil {
			return nil, fmt.Errorf("scan list resurrect candidate: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate list resurrect candidates: %w", err)
	}
	return out, nil
}

func (r *dispatchLogRepository) GetRecentByCanonicalKey(ctx context.Context, userID, groupID, canonicalKey string, window time.Duration) (*models.DispatchLog, error) {
	query := `
		SELECT id, user_id, automation_id, automation_name, store, group_id,
		       product_url, product_page_url, canonical_key, status, type, error, created_at
		FROM dispatch_logs
		WHERE user_id = $1 AND group_id = $2 AND canonical_key = $3
		  AND status IN ('sent', 'queued', 'pending')
		  AND created_at > NOW() - make_interval(secs => $4)
		ORDER BY created_at DESC
		LIMIT 1
	`

	var log models.DispatchLog
	var createdAt time.Time

	// make_interval(secs => ...) takes float seconds; using window.Seconds()
	// gives sub-minute precision when callers want a 30min window etc.
	err := r.db.QueryRow(ctx, query, userID, groupID, canonicalKey, window.Seconds()).Scan(
		&log.ID, &log.UserID, &log.AutomationID, &log.AutomationName, &log.Store, &log.GroupID,
		&log.ProductURL, &log.ProductPageURL, &log.CanonicalKey, &log.Status, &log.Type, &log.Error, &createdAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	log.CreatedAt = createdAt.Format(time.RFC3339)
	return &log, nil
}

func (r *dispatchLogRepository) GetUserLogs(ctx context.Context, userID string, limit, offset int) ([]models.DispatchLog, error) {
	if limit <= 0 {
		limit = 50
	}

	query := `
		SELECT id, user_id, automation_id, automation_name, store, group_id,
		       product_url, product_page_url, canonical_key, status, type, error, created_at
		FROM dispatch_logs
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []models.DispatchLog
	for rows.Next() {
		var log models.DispatchLog
		var createdAt time.Time

		err := rows.Scan(
			&log.ID, &log.UserID, &log.AutomationID, &log.AutomationName, &log.Store, &log.GroupID,
			&log.ProductURL, &log.ProductPageURL, &log.CanonicalKey, &log.Status, &log.Type, &log.Error, &createdAt,
		)
		if err != nil {
			return nil, err
		}

		log.CreatedAt = createdAt.Format(time.RFC3339)
		logs = append(logs, log)
	}

	return logs, rows.Err()
}

// GetLogsByAutomation retrieves dispatch logs for a specific automation
func (r *dispatchLogRepository) GetLogsByAutomation(ctx context.Context, automationID string, limit, offset int) ([]models.DispatchLog, error) {
	if limit <= 0 {
		limit = 50
	}

	query := `
		SELECT id, user_id, automation_id, automation_name, store, group_id,
		       product_url, product_page_url, canonical_key, status, type, error, created_at
		FROM dispatch_logs
		WHERE automation_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.Query(ctx, query, automationID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []models.DispatchLog
	for rows.Next() {
		var log models.DispatchLog
		var createdAt time.Time

		err := rows.Scan(
			&log.ID, &log.UserID, &log.AutomationID, &log.AutomationName, &log.Store, &log.GroupID,
			&log.ProductURL, &log.ProductPageURL, &log.CanonicalKey, &log.Status, &log.Type, &log.Error, &createdAt,
		)
		if err != nil {
			return nil, err
		}

		log.CreatedAt = createdAt.Format(time.RFC3339)
		logs = append(logs, log)
	}

	return logs, rows.Err()
}

// CheckDedup checks if a product has been dispatched recently (deduplication)
func (r *dispatchLogRepository) CheckDedup(ctx context.Context, userID, groupID, canonicalKey string, windowHours int) (bool, error) {
	query := `
		SELECT EXISTS (
			SELECT 1 FROM dispatch_logs
			WHERE user_id = $1 AND group_id = $2 AND canonical_key = $3
			  AND status IN ('sent', 'queued', 'pending')
			  AND created_at > NOW() - make_interval(hours => $4)
		)
	`

	var exists bool
	err := r.db.QueryRow(ctx, query, userID, groupID, canonicalKey, windowHours).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}

// GetStats returns dispatch statistics for a user
func (r *dispatchLogRepository) GetStats(ctx context.Context, userID string, since time.Time) (map[string]int, error) {
	query := `
		SELECT status, COUNT(*) as count
		FROM dispatch_logs
		WHERE user_id = $1 AND created_at >= $2
		GROUP BY status
	`

	rows, err := r.db.Query(ctx, query, userID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := make(map[string]int)
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		stats[status] = count
	}

	return stats, rows.Err()
}

// DeleteOldLogs deletes dispatch logs older than the specified duration
func (r *dispatchLogRepository) DeleteOldLogs(ctx context.Context, olderThan time.Duration) (int64, error) {
	query := `DELETE FROM dispatch_logs WHERE created_at < $1`
	cutoff := time.Now().Add(-olderThan)

	result, err := r.db.pool.Exec(ctx, query, cutoff)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected(), nil
}

// DeleteOldLogsByStatus deletes dispatch logs with a specific status older than
// the given duration. Apaga em lotes (ctid + LIMIT, mesmo padrão de
// CleanupUnrecoverable em webhook_events) em vez de um DELETE único: com
// milhões de linhas expirando de uma vez, o single-shot segurava lock/WAL/IO
// e, se estourasse o ctx de 5min do cron, dava rollback de TUDO — o lote
// commita a cada iteração, então progresso parcial sobrevive ao timeout e o
// restante sai na próxima rodada diária.
func (r *dispatchLogRepository) DeleteOldLogsByStatus(ctx context.Context, status string, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)
	const batch = 50000
	query := `
		DELETE FROM dispatch_logs WHERE ctid IN (
			SELECT ctid FROM dispatch_logs
			WHERE status = $1 AND created_at < $2
			LIMIT $3
		)`

	var total int64
	for i := 0; i < 120; i++ { // teto de 6M linhas por execução (roda diariamente)
		result, err := r.db.pool.Exec(ctx, query, status, cutoff, batch)
		if err != nil {
			// Devolve o progresso já commitado junto com o erro: o caller loga
			// e a próxima rodada diária continua de onde parou.
			return total, err
		}
		n := result.RowsAffected()
		total += n
		if n < batch {
			break
		}
	}

	return total, nil
}

// DeleteRecentByAutomation deletes dispatch logs for an automation within the last N days
func (r *dispatchLogRepository) DeleteRecentByAutomation(ctx context.Context, automationID string, sinceDays int) (int64, error) {
	query := `
		DELETE FROM dispatch_logs
		WHERE automation_id = $1
		AND created_at >= $2
	`

	cutoff := time.Now().UTC().Add(-time.Duration(sinceDays) * 24 * time.Hour)
	result, err := r.db.pool.Exec(ctx, query, automationID, cutoff)
	if err != nil {
		r.logger.Error("Failed to delete dispatch logs by automation",
			zap.String("automationID", automationID),
			zap.Int("sinceDays", sinceDays),
			zap.Error(err))
		return 0, fmt.Errorf("failed to delete dispatch logs: %w", err)
	}

	r.logger.Info("Deleted dispatch logs by automation",
		zap.String("automationID", automationID),
		zap.Int("sinceDays", sinceDays),
		zap.Int64("rowsDeleted", result.RowsAffected()))

	return result.RowsAffected(), nil
}

// GetByAutomationIDWithCount retrieves dispatch logs for a specific automation with total count for pagination.
// Verifies user ownership via automation_id -> automations.user_id check.
func (r *dispatchLogRepository) GetByAutomationIDWithCount(ctx context.Context, automationID, userID string, limit, offset int) ([]repository.DispatchLogWithGroup, int, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	// First, verify user ownership of the automation
	ownershipQuery := `SELECT 1 FROM automations WHERE id = $1 AND user_id = $2`
	var exists int
	err := r.db.QueryRow(ctx, ownershipQuery, automationID, userID).Scan(&exists)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, 0, fmt.Errorf("automation not found or access denied")
		}
		r.logger.Error("Failed to verify automation ownership",
			zap.String("automationID", automationID),
			zap.String("userID", userID),
			zap.Error(err))
		return nil, 0, fmt.Errorf("failed to verify automation ownership: %w", err)
	}

	// Get total count for pagination
	countQuery := `SELECT COUNT(*) FROM dispatch_logs WHERE automation_id = $1`
	var totalCount int
	err = r.db.QueryRow(ctx, countQuery, automationID).Scan(&totalCount)
	if err != nil {
		r.logger.Error("Failed to count dispatch logs",
			zap.String("automationID", automationID),
			zap.Error(err))
		return nil, 0, fmt.Errorf("failed to count dispatch logs: %w", err)
	}

	// Get logs with group name via LEFT JOIN
	query := `
		SELECT
			dl.id, dl.user_id, dl.automation_id, dl.automation_name, dl.store, dl.group_id,
			dl.product_url, dl.product_page_url, dl.canonical_key, dl.status, dl.type,
			dl.error, dl.created_at, dl.product_price,
			g.name as group_name
		FROM dispatch_logs dl
		LEFT JOIN groups g ON dl.group_id = g.wa_group_id AND g.user_id = dl.user_id
		WHERE dl.automation_id = $1
		ORDER BY dl.created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.Query(ctx, query, automationID, limit, offset)
	if err != nil {
		r.logger.Error("Failed to query dispatch logs",
			zap.String("automationID", automationID),
			zap.Error(err))
		return nil, 0, fmt.Errorf("failed to query dispatch logs: %w", err)
	}
	defer rows.Close()

	var logs []repository.DispatchLogWithGroup
	for rows.Next() {
		var log repository.DispatchLogWithGroup
		var createdAt time.Time
		var productPrice *float64

		err := rows.Scan(
			&log.ID, &log.UserID, &log.AutomationID, &log.AutomationName, &log.Store, &log.GroupID,
			&log.ProductURL, &log.ProductPageURL, &log.CanonicalKey, &log.Status, &log.Type,
			&log.Error, &createdAt, &productPrice,
			&log.GroupName,
		)
		if err != nil {
			r.logger.Error("Failed to scan dispatch log row",
				zap.Error(err))
			return nil, 0, fmt.Errorf("failed to scan dispatch log: %w", err)
		}

		log.CreatedAt = createdAt.Format(time.RFC3339)
		logs = append(logs, log)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating dispatch logs: %w", err)
	}

	return logs, totalCount, nil
}

// ExistsByCanonicalKey checks if a dispatch log with the given canonical_key exists
// for the user and group within the specified time window.
// Used for reconciliation to avoid creating duplicate logs.
func (r *dispatchLogRepository) ExistsByCanonicalKey(ctx context.Context, userID, groupID, canonicalKey string, hours int) (bool, error) {
	if canonicalKey == "" {
		return false, nil
	}

	query := `
		SELECT EXISTS (
			SELECT 1 FROM dispatch_logs
			WHERE user_id = $1 AND group_id = $2 AND canonical_key = $3
			  AND created_at > NOW() - make_interval(hours => $4)
		)
	`

	var exists bool
	err := r.db.QueryRow(ctx, query, userID, groupID, canonicalKey, hours).Scan(&exists)
	if err != nil {
		r.logger.Error("Failed to check canonical key existence",
			zap.String("userID", userID),
			zap.String("groupID", groupID),
			zap.String("canonicalKey", canonicalKey),
			zap.Error(err))
		return false, fmt.Errorf("failed to check canonical key existence: %w", err)
	}

	return exists, nil
}

// CreateIfNotExists creates a dispatch log only if one doesn't already exist
// for the same user, group, and canonical_key within the time window.
// Returns the created log and true if created, or nil and false if already exists.
// This is idempotent and safe for concurrent reconciliation.
func (r *dispatchLogRepository) CreateIfNotExists(ctx context.Context, log *models.DispatchLog, dedupWindowHours int) (*models.DispatchLog, bool, error) {
	// If no canonical_key, just create the log
	if log.CanonicalKey == nil || *log.CanonicalKey == "" {
		created, err := r.Create(ctx, log)
		if err != nil {
			return nil, false, err
		}
		return created, true, nil
	}

	groupID := ""
	if log.GroupID != nil {
		groupID = *log.GroupID
	}

	// Use a single atomic query to check and insert if not exists
	// This prevents race conditions in concurrent reconciliation
	// Separate parameters for INSERT and WHERE to avoid pgx type inference conflicts
	query := `
		INSERT INTO dispatch_logs (
			user_id, automation_id, automation_name, store, group_id,
			product_url, product_page_url, canonical_key, status, type, created_at
		)
		SELECT $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
		WHERE NOT EXISTS (
			SELECT 1 FROM dispatch_logs
			WHERE user_id = $12 AND group_id = $13 AND canonical_key = $14
			  AND created_at > NOW() - make_interval(hours => $15)
		)
		RETURNING id, created_at
	`

	now := time.Now().UTC()
	var id int
	var createdAt time.Time

	canonicalKeyStr := ""
	if log.CanonicalKey != nil {
		canonicalKeyStr = *log.CanonicalKey
	}

	err := r.db.QueryRow(ctx, query,
		log.UserID, log.AutomationID, log.AutomationName, log.Store, log.GroupID,
		log.ProductURL, log.ProductPageURL, log.CanonicalKey, log.Status, log.Type, now,
		log.UserID, groupID, canonicalKeyStr, dedupWindowHours,
	).Scan(&id, &createdAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Log already exists within dedup window
			r.logger.Debug("Dispatch log already exists (dedup)",
				zap.String("userID", log.UserID),
				zap.String("groupID", groupID),
				zap.String("canonicalKey", *log.CanonicalKey))
			return nil, false, nil
		}
		r.logger.Error("Failed to create dispatch log",
			zap.String("userID", log.UserID),
			zap.Error(err))
		return nil, false, fmt.Errorf("failed to create dispatch log: %w", err)
	}

	log.ID = id
	log.CreatedAt = createdAt.Format(time.RFC3339)

	r.logger.Debug("Created dispatch log (reconciliation)",
		zap.Int("logID", id),
		zap.String("userID", log.UserID),
		zap.String("groupID", groupID),
		zap.String("status", log.Status))

	return log, true, nil
}

// GetUserLogsWithFilters retrieves dispatch logs for a user with optional filters.
// If filters.AutomationID is provided, filters by that automation (verifying user ownership).
// If filters.AutomationID is empty/nil, returns logs from all user's automations.
// Supports optional filters: group_id, status, store, search_url (ILIKE on product_url).
// Returns logs with group names and total count for pagination.
func (r *dispatchLogRepository) GetUserLogsWithFilters(ctx context.Context, userID string, filters *models.DispatchLogFilters, limit, offset int) ([]repository.DispatchLogWithGroup, int64, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	if offset < 0 {
		offset = 0
	}

	// Build the WHERE clause dynamically
	var conditions []string
	args := []interface{}{}
	argIndex := 1

	// Only filter by user when userID is provided (empty = admin-wide query)
	if userID != "" {
		conditions = append(conditions, fmt.Sprintf("dl.user_id = $%d", argIndex))
		args = append(args, userID)
		argIndex++
	}

	// Apply optional filters
	if filters != nil {
		if filters.AutomationID != nil && *filters.AutomationID != "" {
			conditions = append(conditions, fmt.Sprintf("dl.automation_id = $%d", argIndex))
			args = append(args, *filters.AutomationID)
			argIndex++
		}
		if filters.GroupID != nil && *filters.GroupID != "" {
			conditions = append(conditions, fmt.Sprintf("dl.group_id = $%d", argIndex))
			args = append(args, *filters.GroupID)
			argIndex++
		}
		if filters.Status != nil && *filters.Status != "" {
			conditions = append(conditions, fmt.Sprintf("dl.status = $%d", argIndex))
			args = append(args, *filters.Status)
			argIndex++
		}
		if filters.Store != nil && *filters.Store != "" {
			conditions = append(conditions, fmt.Sprintf("dl.store = $%d", argIndex))
			args = append(args, *filters.Store)
			argIndex++
		}
		if filters.SearchURL != nil && *filters.SearchURL != "" {
			conditions = append(conditions, fmt.Sprintf("dl.product_url LIKE $%d", argIndex))
			args = append(args, "%"+*filters.SearchURL+"%")
			argIndex++
		}
		if filters.Since != nil {
			conditions = append(conditions, fmt.Sprintf("dl.created_at >= $%d", argIndex))
			args = append(args, *filters.Since)
			argIndex++
		}
	}

	whereClause := "TRUE"
	if len(conditions) > 0 {
		whereClause = strings.Join(conditions, " AND ")
	}

	// Get total count for pagination
	countQuery := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM dispatch_logs dl
		WHERE %s
	`, whereClause)

	var totalCount int64
	err := r.db.QueryRow(ctx, countQuery, args...).Scan(&totalCount)
	if err != nil {
		r.logger.Error("Failed to count dispatch logs with filters",
			zap.String("userID", userID),
			zap.Error(err))
		return nil, 0, fmt.Errorf("failed to count dispatch logs: %w", err)
	}

	// Get logs with all fields including group name via LEFT JOIN
	// Add limit and offset to args
	args = append(args, limit, offset)

	query := fmt.Sprintf(`
		SELECT
			dl.id, dl.user_id, dl.automation_id, dl.automation_name, dl.store, dl.group_id,
			dl.product_url, dl.product_page_url, dl.canonical_key, dl.status, dl.type,
			dl.error, dl.created_at, dl.product_price, dl.product_category, dl.product_discount,
			dl.affiliate_link,
			dl.coupon_id, dl.coupon_code, dl.coupon_applied, dl.coupon_skip_reason,
			g.name as group_name
		FROM dispatch_logs dl
		LEFT JOIN groups g ON dl.group_id = g.wa_group_id AND g.user_id = dl.user_id
		WHERE %s
		ORDER BY dl.created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIndex, argIndex+1)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		r.logger.Error("Failed to query dispatch logs with filters",
			zap.String("userID", userID),
			zap.Error(err))
		return nil, 0, fmt.Errorf("failed to query dispatch logs: %w", err)
	}
	defer rows.Close()

	var logs []repository.DispatchLogWithGroup
	for rows.Next() {
		var log repository.DispatchLogWithGroup
		var createdAt time.Time

		err := rows.Scan(
			&log.ID, &log.UserID, &log.AutomationID, &log.AutomationName, &log.Store, &log.GroupID,
			&log.ProductURL, &log.ProductPageURL, &log.CanonicalKey, &log.Status, &log.Type,
			&log.Error, &createdAt, &log.ProductPrice, &log.ProductCategory, &log.ProductDiscount,
			&log.AffiliateLink, &log.CouponID, &log.CouponCode, &log.CouponApplied, &log.CouponSkipReason,
			&log.GroupName,
		)
		if err != nil {
			r.logger.Error("Failed to scan dispatch log row",
				zap.Error(err))
			return nil, 0, fmt.Errorf("failed to scan dispatch log: %w", err)
		}

		log.CreatedAt = createdAt.Format(time.RFC3339)
		logs = append(logs, log)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating dispatch logs: %w", err)
	}

	return logs, totalCount, nil
}
