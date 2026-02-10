package postgres

import (
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

type automationRepository struct {
	db     *DB
	logger *zap.Logger
}

// NewAutomationRepository creates a new PostgreSQL automation repository
func NewAutomationRepository(db *DB, logger *zap.Logger) repository.AutomationRepository {
	return &automationRepository{
		db:     db,
		logger: logger,
	}
}

func (r *automationRepository) GetUserAutomations(ctx context.Context, userID string) ([]models.Automation, error) {
	query := `
		SELECT id, user_id, name, type, status, mode, shopee_link, keyword,
		       group_ids, send_groups, monitor_group_ids, monitoring_text_rules,
		       categories, stores, min_price, max_price, min_discount, priority,
		       custom_message, include_image, dispatch_text_id, filter_type,
		       schedule_time, schedule_days, start_time, end_time, interval_minutes,
		       delay_seconds, dedupe_hours, last_run_at, next_run_at,
		       coupon_id, created_at, updated_at
		FROM automations
		WHERE user_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanAutomations(rows)
}

func (r *automationRepository) GetActiveAutomations(ctx context.Context) ([]models.Automation, error) {
	query := `
		SELECT id, user_id, name, type, status, mode, shopee_link, keyword,
		       group_ids, send_groups, monitor_group_ids, monitoring_text_rules,
		       categories, stores, min_price, max_price, min_discount, priority,
		       custom_message, include_image, dispatch_text_id, filter_type,
		       schedule_time, schedule_days, start_time, end_time, interval_minutes,
		       delay_seconds, dedupe_hours, last_run_at, next_run_at,
		       coupon_id, created_at, updated_at
		FROM automations
		WHERE status = 'active'
		ORDER BY priority DESC, created_at ASC
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanAutomations(rows)
}

func (r *automationRepository) GetAutomationByID(ctx context.Context, automationID string) (*models.Automation, error) {
	query := `
		SELECT id, user_id, name, type, status, mode, shopee_link, keyword,
		       group_ids, send_groups, monitor_group_ids, monitoring_text_rules,
		       categories, stores, min_price, max_price, min_discount, priority,
		       custom_message, include_image, dispatch_text_id, filter_type,
		       schedule_time, schedule_days, start_time, end_time, interval_minutes,
		       delay_seconds, dedupe_hours, last_run_at, next_run_at,
		       coupon_id, created_at, updated_at
		FROM automations
		WHERE id = $1
	`

	return r.scanAutomation(r.db.QueryRow(ctx, query, automationID))
}

func (r *automationRepository) GetAutomationByIDAndUser(ctx context.Context, automationID, userID string) (*models.Automation, error) {
	query := `
		SELECT id, user_id, name, type, status, mode, shopee_link, keyword,
		       group_ids, send_groups, monitor_group_ids, monitoring_text_rules,
		       categories, stores, min_price, max_price, min_discount, priority,
		       custom_message, include_image, dispatch_text_id, filter_type,
		       schedule_time, schedule_days, start_time, end_time, interval_minutes,
		       delay_seconds, dedupe_hours, last_run_at, next_run_at,
		       coupon_id, created_at, updated_at
		FROM automations
		WHERE id = $1 AND user_id = $2
	`

	return r.scanAutomation(r.db.QueryRow(ctx, query, automationID, userID))
}

func (r *automationRepository) CreateAutomation(ctx context.Context, userID string, data *models.Automation) (*models.Automation, error) {
	now := time.Now().UTC()

	// Convert arrays to JSON
	groupIDsJSON, _ := json.Marshal(data.GroupIDs)
	sendGroupsJSON, _ := json.Marshal(data.SendGroups)
	monitorGroupIDsJSON, _ := json.Marshal(data.MonitorGroupIDs)
	categoriesJSON, _ := json.Marshal(data.Categories)
	storesJSON, _ := json.Marshal(data.Stores)
	scheduleDaysJSON, _ := json.Marshal(data.ScheduleDays)

	var monitoringRulesJSON []byte
	if data.MonitoringTextRules != nil {
		monitoringRulesJSON, _ = json.Marshal(data.MonitoringTextRules)
	}

	query := `
		INSERT INTO automations (
			user_id, name, type, status, mode, shopee_link, keyword,
			group_ids, send_groups, monitor_group_ids, monitoring_text_rules,
			categories, stores, min_price, max_price, min_discount, priority,
			custom_message, include_image, dispatch_text_id, filter_type,
			schedule_time, schedule_days, start_time, end_time, interval_minutes,
			delay_seconds, dedupe_hours, coupon_id, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17,
			$18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29, $30, $31
		)
		RETURNING id
	`

	var id string
	err := r.db.QueryRow(ctx, query,
		userID, data.Name, data.Type, data.Status, data.Mode, data.ShopeeLink, data.Keyword,
		groupIDsJSON, sendGroupsJSON, monitorGroupIDsJSON, monitoringRulesJSON,
		categoriesJSON, storesJSON, data.MinPrice, data.MaxPrice, data.MinDiscount, 0,
		data.CustomMessage, data.IncludeImage, data.DispatchTextID, nil,
		data.ScheduleTime, scheduleDaysJSON, data.StartTime, data.EndTime, data.IntervalMinutes,
		data.DelaySeconds, data.DedupeHours, data.CouponID, now, now,
	).Scan(&id)

	if err != nil {
		r.logger.Error("Failed to create automation",
			zap.String("userID", userID),
			zap.Error(err))
		return nil, err
	}

	return r.GetAutomationByID(ctx, id)
}

func (r *automationRepository) UpdateAutomation(ctx context.Context, automationID string, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return nil
	}

	// Convert array fields to JSON
	for _, key := range []string{"group_ids", "send_groups", "monitor_group_ids", "categories", "stores", "schedule_days"} {
		if val, ok := updates[key]; ok {
			if arr, ok := val.([]string); ok {
				jsonBytes, _ := json.Marshal(arr)
				updates[key] = string(jsonBytes)
			}
		}
	}

	// Convert monitoring_text_rules to JSON
	if val, ok := updates["monitoring_text_rules"]; ok {
		if rules, ok := val.(*models.MonitoringTextRules); ok {
			jsonBytes, _ := json.Marshal(rules)
			updates["monitoring_text_rules"] = string(jsonBytes)
		}
	}

	// Build dynamic UPDATE query
	setClauses := ""
	args := make([]interface{}, 0, len(updates)+1)
	argIndex := 1

	for key, value := range updates {
		if setClauses != "" {
			setClauses += ", "
		}
		setClauses += fmt.Sprintf("%s = $%d", key, argIndex)
		args = append(args, value)
		argIndex++
	}

	// Always update updated_at
	setClauses += fmt.Sprintf(", updated_at = $%d", argIndex)
	args = append(args, time.Now().UTC())
	argIndex++

	// Add automationID as the last argument
	args = append(args, automationID)

	query := fmt.Sprintf("UPDATE automations SET %s WHERE id = $%d", setClauses, argIndex)

	_, err := r.db.pool.Exec(ctx, query, args...)
	if err != nil {
		r.logger.Error("Failed to update automation",
			zap.String("automationID", automationID),
			zap.Error(err))
		return err
	}

	return nil
}

func (r *automationRepository) DeleteAutomation(ctx context.Context, automationID, userID string) error {
	query := `DELETE FROM automations WHERE id = $1 AND user_id = $2`

	_, err := r.db.pool.Exec(ctx, query, automationID, userID)
	if err != nil {
		r.logger.Error("Failed to delete automation",
			zap.String("automationID", automationID),
			zap.Error(err))
		return err
	}

	return nil
}

func (r *automationRepository) GetMonitoringAutomationsForGroup(ctx context.Context, waGroupID string) ([]models.Automation, error) {
	// Use PostgreSQL's JSON array containment
	query := `
		SELECT id, user_id, name, type, status, mode, shopee_link, keyword,
		       group_ids, send_groups, monitor_group_ids, monitoring_text_rules,
		       categories, stores, min_price, max_price, min_discount, priority,
		       custom_message, include_image, dispatch_text_id, filter_type,
		       schedule_time, schedule_days, start_time, end_time, interval_minutes,
		       delay_seconds, dedupe_hours, last_run_at, next_run_at,
		       coupon_id, created_at, updated_at
		FROM automations
		WHERE status = 'active'
		  AND type = 'monitoring'
		  AND monitor_group_ids @> $1::jsonb
	`

	groupIDJSON, _ := json.Marshal([]string{waGroupID})

	rows, err := r.db.Query(ctx, query, string(groupIDJSON))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanAutomations(rows)
}

// Helper functions

func (r *automationRepository) scanAutomation(row pgx.Row) (*models.Automation, error) {
	var a models.Automation
	var groupIDsJSON, sendGroupsJSON, monitorGroupIDsJSON []byte
	var monitoringRulesJSON, categoriesJSON, storesJSON, scheduleDaysJSON []byte
	var scheduleTime, startTime, endTime, lastRunAt, nextRunAt *time.Time
	var createdAt, updatedAt time.Time
	var priority int
	var filterType *string

	err := row.Scan(
		&a.ID, &a.UserID, &a.Name, &a.Type, &a.Status, &a.Mode, &a.ShopeeLink, &a.Keyword,
		&groupIDsJSON, &sendGroupsJSON, &monitorGroupIDsJSON, &monitoringRulesJSON,
		&categoriesJSON, &storesJSON, &a.MinPrice, &a.MaxPrice, &a.MinDiscount, &priority,
		&a.CustomMessage, &a.IncludeImage, &a.DispatchTextID, &filterType,
		&scheduleTime, &scheduleDaysJSON, &startTime, &endTime, &a.IntervalMinutes,
		&a.DelaySeconds, &a.DedupeHours, &lastRunAt, &nextRunAt,
		&a.CouponID, &createdAt, &updatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	// Unmarshal JSON arrays (ignore errors - use empty arrays on failure)
	if groupIDsJSON != nil {
		_ = json.Unmarshal(groupIDsJSON, &a.GroupIDs)
	}
	if sendGroupsJSON != nil {
		_ = json.Unmarshal(sendGroupsJSON, &a.SendGroups)
	}
	if monitorGroupIDsJSON != nil {
		_ = json.Unmarshal(monitorGroupIDsJSON, &a.MonitorGroupIDs)
	}
	if categoriesJSON != nil {
		_ = json.Unmarshal(categoriesJSON, &a.Categories)
	}
	if storesJSON != nil {
		_ = json.Unmarshal(storesJSON, &a.Stores)
	}
	if scheduleDaysJSON != nil {
		_ = json.Unmarshal(scheduleDaysJSON, &a.ScheduleDays)
	}
	if monitoringRulesJSON != nil {
		var rules models.MonitoringTextRules
		_ = json.Unmarshal(monitoringRulesJSON, &rules)
		a.MonitoringTextRules = &rules
	}

	// Convert times
	if scheduleTime != nil {
		t := scheduleTime.Format("15:04")
		a.ScheduleTime = &t
	}
	if startTime != nil {
		t := startTime.Format("15:04")
		a.StartTime = &t
	}
	if endTime != nil {
		t := endTime.Format("15:04")
		a.EndTime = &t
	}
	if lastRunAt != nil {
		t := lastRunAt.Format(time.RFC3339)
		a.LastRunAt = &t
	}
	if nextRunAt != nil {
		t := nextRunAt.Format(time.RFC3339)
		a.NextRunAt = &t
	}

	a.CreatedAt = createdAt.Format(time.RFC3339)
	a.UpdatedAt = updatedAt.Format(time.RFC3339)

	return &a, nil
}

func (r *automationRepository) scanAutomations(rows pgx.Rows) ([]models.Automation, error) {
	var automations []models.Automation
	for rows.Next() {
		var a models.Automation
		var groupIDsJSON, sendGroupsJSON, monitorGroupIDsJSON []byte
		var monitoringRulesJSON, categoriesJSON, storesJSON, scheduleDaysJSON []byte
		var scheduleTime, startTime, endTime, lastRunAt, nextRunAt *time.Time
		var createdAt, updatedAt time.Time
		var priority int
		var filterType *string

		err := rows.Scan(
			&a.ID, &a.UserID, &a.Name, &a.Type, &a.Status, &a.Mode, &a.ShopeeLink, &a.Keyword,
			&groupIDsJSON, &sendGroupsJSON, &monitorGroupIDsJSON, &monitoringRulesJSON,
			&categoriesJSON, &storesJSON, &a.MinPrice, &a.MaxPrice, &a.MinDiscount, &priority,
			&a.CustomMessage, &a.IncludeImage, &a.DispatchTextID, &filterType,
			&scheduleTime, &scheduleDaysJSON, &startTime, &endTime, &a.IntervalMinutes,
			&a.DelaySeconds, &a.DedupeHours, &lastRunAt, &nextRunAt,
			&a.CouponID, &createdAt, &updatedAt,
		)
		if err != nil {
			return nil, err
		}

		// Unmarshal JSON arrays (ignore errors - use empty arrays on failure)
		if groupIDsJSON != nil {
			_ = json.Unmarshal(groupIDsJSON, &a.GroupIDs)
		}
		if sendGroupsJSON != nil {
			_ = json.Unmarshal(sendGroupsJSON, &a.SendGroups)
		}
		if monitorGroupIDsJSON != nil {
			_ = json.Unmarshal(monitorGroupIDsJSON, &a.MonitorGroupIDs)
		}
		if categoriesJSON != nil {
			_ = json.Unmarshal(categoriesJSON, &a.Categories)
		}
		if storesJSON != nil {
			_ = json.Unmarshal(storesJSON, &a.Stores)
		}
		if scheduleDaysJSON != nil {
			_ = json.Unmarshal(scheduleDaysJSON, &a.ScheduleDays)
		}
		if monitoringRulesJSON != nil {
			var rules models.MonitoringTextRules
			_ = json.Unmarshal(monitoringRulesJSON, &rules)
			a.MonitoringTextRules = &rules
		}

		// Convert times
		if scheduleTime != nil {
			t := scheduleTime.Format("15:04")
			a.ScheduleTime = &t
		}
		if startTime != nil {
			t := startTime.Format("15:04")
			a.StartTime = &t
		}
		if endTime != nil {
			t := endTime.Format("15:04")
			a.EndTime = &t
		}
		if lastRunAt != nil {
			t := lastRunAt.Format(time.RFC3339)
			a.LastRunAt = &t
		}
		if nextRunAt != nil {
			t := nextRunAt.Format(time.RFC3339)
			a.NextRunAt = &t
		}

		a.CreatedAt = createdAt.Format(time.RFC3339)
		a.UpdatedAt = updatedAt.Format(time.RFC3339)

		automations = append(automations, a)
	}

	return automations, rows.Err()
}

// UpdateLastRunAt updates the last_run_at timestamp for an automation
func (r *automationRepository) UpdateLastRunAt(ctx context.Context, automationID string) error {
	query := `
		UPDATE automations
		SET last_run_at = $1, updated_at = $1
		WHERE id = $2
	`
	_, err := r.db.pool.Exec(ctx, query, time.Now().UTC(), automationID)
	return err
}

// UpdateNextRunAt updates the next_run_at timestamp for an automation
func (r *automationRepository) UpdateNextRunAt(ctx context.Context, automationID string, nextRun time.Time) error {
	query := `
		UPDATE automations
		SET next_run_at = $1, updated_at = $2
		WHERE id = $3
	`
	_, err := r.db.pool.Exec(ctx, query, nextRun, time.Now().UTC(), automationID)
	return err
}

// BulkUpdateStatusByUserID updates the status of all automations for a user
// matching the optional source status filter. Returns the count of updated records.
func (r *automationRepository) BulkUpdateStatusByUserID(ctx context.Context, userID, targetStatus string, sourceStatus *string) (int, error) {
	now := time.Now().UTC()

	var query string
	var args []interface{}

	if sourceStatus != nil && *sourceStatus != "" {
		query = `
			UPDATE automations
			SET status = $1, updated_at = $2
			WHERE user_id = $3 AND status = $4
		`
		args = []interface{}{targetStatus, now, userID, *sourceStatus}
	} else {
		query = `
			UPDATE automations
			SET status = $1, updated_at = $2
			WHERE user_id = $3
		`
		args = []interface{}{targetStatus, now, userID}
	}

	result, err := r.db.pool.Exec(ctx, query, args...)
	if err != nil {
		r.logger.Error("Failed to bulk update automation status",
			zap.String("userID", userID),
			zap.String("targetStatus", targetStatus),
			zap.Error(err))
		return 0, err
	}

	return int(result.RowsAffected()), nil
}

// BulkUpdateNextRunAts updates the next_run_at for multiple automations at once.
// The automationIDs and nextRunAts slices must have the same length.
func (r *automationRepository) BulkUpdateNextRunAts(ctx context.Context, automationIDs []string, nextRunAts []time.Time) error {
	if len(automationIDs) == 0 {
		return nil
	}
	if len(automationIDs) != len(nextRunAts) {
		return fmt.Errorf("automationIDs and nextRunAts must have the same length")
	}

	now := time.Now().UTC()

	// Use a batch for efficiency
	batch := &pgx.Batch{}
	for i, id := range automationIDs {
		batch.Queue(`
			UPDATE automations
			SET next_run_at = $1, updated_at = $2
			WHERE id = $3
		`, nextRunAts[i], now, id)
	}

	br := r.db.pool.SendBatch(ctx, batch)
	defer br.Close()

	for range automationIDs {
		_, err := br.Exec()
		if err != nil {
			r.logger.Error("Failed to update next_run_at in batch",
				zap.Error(err))
			return err
		}
	}

	return nil
}
