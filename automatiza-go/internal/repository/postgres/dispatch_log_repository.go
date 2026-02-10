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
			product_url, product_page_url, canonical_key, status, type, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, created_at
	`

	now := time.Now().UTC()
	var id int
	var createdAt time.Time

	err := r.db.QueryRow(ctx, query,
		log.UserID, log.AutomationID, log.AutomationName, log.Store, log.GroupID,
		log.ProductURL, log.ProductPageURL, log.CanonicalKey, log.Status, log.Type, now,
	).Scan(&id, &createdAt)

	if err != nil {
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

func (r *dispatchLogRepository) GetRecentByCanonicalKey(ctx context.Context, userID, groupID, canonicalKey string, hours int) (*models.DispatchLog, error) {
	query := `
		SELECT id, user_id, automation_id, automation_name, store, group_id,
		       product_url, product_page_url, canonical_key, status, type, error, created_at
		FROM dispatch_logs
		WHERE user_id = $1 AND group_id = $2 AND canonical_key = $3
		  AND status IN ('sent', 'queued', 'pending')
		  AND created_at > NOW() - ($4::text || ' hours')::INTERVAL
		ORDER BY created_at DESC
		LIMIT 1
	`

	var log models.DispatchLog
	var createdAt time.Time

	err := r.db.QueryRow(ctx, query, userID, groupID, canonicalKey, hours).Scan(
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
			  AND created_at > NOW() - ($4::text || ' hours')::INTERVAL
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

// DeleteRecentByAutomation deletes dispatch logs for an automation within the last N days
func (r *dispatchLogRepository) DeleteRecentByAutomation(ctx context.Context, automationID string, sinceDays int) (int64, error) {
	query := `
		DELETE FROM dispatch_logs
		WHERE automation_id = $1
		AND created_at >= NOW() - ($2::text || ' days')::INTERVAL
	`

	result, err := r.db.pool.Exec(ctx, query, automationID, sinceDays)
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
			  AND created_at > NOW() - ($4::text || ' hours')::INTERVAL
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
	query := `
		INSERT INTO dispatch_logs (
			user_id, automation_id, automation_name, store, group_id,
			product_url, product_page_url, canonical_key, status, type, created_at
		)
		SELECT $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
		WHERE NOT EXISTS (
			SELECT 1 FROM dispatch_logs
			WHERE user_id = $1 AND group_id = $5 AND canonical_key = $8
			  AND created_at > NOW() - ($12::text || ' hours')::INTERVAL
		)
		RETURNING id, created_at
	`

	now := time.Now().UTC()
	var id int
	var createdAt time.Time

	err := r.db.QueryRow(ctx, query,
		log.UserID, log.AutomationID, log.AutomationName, log.Store, log.GroupID,
		log.ProductURL, log.ProductPageURL, log.CanonicalKey, log.Status, log.Type, now,
		dedupWindowHours,
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
			conditions = append(conditions, fmt.Sprintf("dl.product_url ILIKE $%d", argIndex))
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
