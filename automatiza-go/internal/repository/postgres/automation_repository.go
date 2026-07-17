package postgres

import (
	"automatiza-go/internal/models"
	"automatiza-go/internal/observability"
	"automatiza-go/internal/repository"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

// marshalJSONField marshals a value to JSON, logging and capturing errors.
// Returns the JSON bytes or the provided fallback on error.
func marshalJSONField(logger *zap.Logger, val interface{}, fieldName string, fallback []byte) []byte {
	b, err := json.Marshal(val)
	if err != nil {
		observability.LogAndCapture(logger, sentry.LevelWarning,
			"Failed to marshal automation JSON field",
			err, map[string]string{
				"module": "automation", "operation": "marshal_field",
				"field": fieldName,
			})
		return fallback
	}
	return b
}

// unmarshalJSONField unmarshals JSON bytes into dst, logging and capturing errors.
func unmarshalJSONField(logger *zap.Logger, data []byte, dst interface{}, fieldName, automationID string) {
	if err := json.Unmarshal(data, dst); err != nil {
		observability.LogAndCapture(logger, sentry.LevelWarning,
			"Failed to unmarshal automation JSON field",
			err, map[string]string{
				"module": "automation", "operation": "unmarshal_field",
				"field": fieldName, "automation_id": automationID,
			})
	}
}

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
		       coupon_id, created_at, updated_at,
		       texts, ctas, cta_ids, last_error, last_error_at,
		       ml_link_list_id, list_items_per_run, list_include_image, priority_type,
		       list_send_as_image, instance_id, instance_ids, apply_coupon_rules, chip_overrides,
		       repeat_when_finished, start_date, end_date, offer_generate_send_as_image,
		       search_send_as_image, monitoring_send_as_image, humanize_mode
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
		       coupon_id, created_at, updated_at,
		       texts, ctas, cta_ids, last_error, last_error_at,
		       ml_link_list_id, list_items_per_run, list_include_image, priority_type,
		       list_send_as_image, instance_id, instance_ids, apply_coupon_rules, chip_overrides,
		       repeat_when_finished, start_date, end_date, offer_generate_send_as_image,
		       search_send_as_image, monitoring_send_as_image, humanize_mode
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
		       coupon_id, created_at, updated_at,
		       texts, ctas, cta_ids, last_error, last_error_at,
		       ml_link_list_id, list_items_per_run, list_include_image, priority_type,
		       list_send_as_image, instance_id, instance_ids, apply_coupon_rules, chip_overrides,
		       repeat_when_finished, start_date, end_date, offer_generate_send_as_image,
		       search_send_as_image, monitoring_send_as_image, humanize_mode
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
		       coupon_id, created_at, updated_at,
		       texts, ctas, cta_ids, last_error, last_error_at,
		       ml_link_list_id, list_items_per_run, list_include_image, priority_type,
		       list_send_as_image, instance_id, instance_ids, apply_coupon_rules, chip_overrides,
		       repeat_when_finished, start_date, end_date, offer_generate_send_as_image,
		       search_send_as_image, monitoring_send_as_image, humanize_mode
		FROM automations
		WHERE id = $1 AND user_id = $2
	`

	return r.scanAutomation(r.db.QueryRow(ctx, query, automationID, userID))
}

func (r *automationRepository) CreateAutomation(ctx context.Context, userID string, data *models.Automation) (*models.Automation, error) {
	now := time.Now().UTC()

	// Convert arrays to JSON
	emptyArr := []byte("[]")
	groupIDsJSON := marshalJSONField(r.logger, data.GroupIDs, "group_ids", emptyArr)
	sendGroupsJSON := marshalJSONField(r.logger, data.SendGroups, "send_groups", emptyArr)
	monitorGroupIDsJSON := marshalJSONField(r.logger, data.MonitorGroupIDs, "monitor_group_ids", emptyArr)
	categoriesJSON := marshalJSONField(r.logger, data.Categories, "categories", emptyArr)
	storesJSON := marshalJSONField(r.logger, data.Stores, "stores", emptyArr)
	scheduleDaysJSON := marshalJSONField(r.logger, data.ScheduleDays, "schedule_days", emptyArr)

	var monitoringRulesJSON []byte
	if data.MonitoringTextRules != nil {
		monitoringRulesJSON = marshalJSONField(r.logger, data.MonitoringTextRules, "monitoring_text_rules", []byte("null"))
	}

	query := `
		INSERT INTO automations (
			user_id, name, type, status, mode, shopee_link, keyword,
			group_ids, send_groups, monitor_group_ids, monitoring_text_rules,
			categories, stores, min_price, max_price, min_discount, priority,
			custom_message, include_image, dispatch_text_id, filter_type,
			schedule_time, schedule_days, start_time, end_time, interval_minutes,
			delay_seconds, dedupe_hours, coupon_id, created_at, updated_at,
			texts, ctas, cta_ids, ml_link_list_id, list_items_per_run, list_include_image, priority_type,
			list_send_as_image, instance_id, instance_ids, apply_coupon_rules, chip_overrides,
			repeat_when_finished, start_date, end_date, offer_generate_send_as_image,
			search_send_as_image, monitoring_send_as_image, humanize_mode
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17,
			$18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29, $30, $31,
			$32, $33, $34, $35, $36, $37, $38, $39, $40, $41, $42, $43, $44, $45, $46, $47,
			$48, $49, $50
		)
		RETURNING id
	`

	// instance_id is the string form on the wire; coerce to int for the
	// INTEGER FK column. Empty string / unparseable → NULL (no preference).
	var instanceIDArg interface{}
	if data.InstanceID != nil && *data.InstanceID != "" {
		var n int
		if _, err := fmt.Sscanf(*data.InstanceID, "%d", &n); err == nil && n > 0 {
			instanceIDArg = n
		}
	}

	// instance_ids is the multi-chip preference (049). The model field is
	// []string to match the wire format; pgx writes INTEGER[] from []int.
	// Unparseable / empty entries are dropped so the array never contains
	// 0 (which would FK to a non-existent row). nil = NULL column.
	var instanceIDsArg interface{}
	if len(data.InstanceIDs) > 0 {
		ids := make([]int, 0, len(data.InstanceIDs))
		for _, s := range data.InstanceIDs {
			if s == "" {
				continue
			}
			var n int
			if _, err := fmt.Sscanf(s, "%d", &n); err == nil && n > 0 {
				ids = append(ids, n)
			}
		}
		if len(ids) > 0 {
			instanceIDsArg = ids
		}
	}

	// chip_overrides (050): JSONB column. Marshal the map; nil/empty map
	// becomes "{}" so the NOT NULL DEFAULT '{}'::jsonb constraint is
	// satisfied. Empty-map and absence are equivalent at the dispatch
	// level (full auto routing).
	chipOverridesJSON := marshalJSONField(r.logger, data.ChipOverrides, "chip_overrides", []byte("{}"))

	var id string
	err := r.db.QueryRow(ctx, query,
		userID, data.Name, data.Type, data.Status, data.Mode, data.ShopeeLink, data.Keyword,
		groupIDsJSON, sendGroupsJSON, monitorGroupIDsJSON, monitoringRulesJSON,
		categoriesJSON, storesJSON, data.MinPrice, data.MaxPrice, data.MinDiscount, 0,
		data.CustomMessage, data.IncludeImage, data.DispatchTextID, nil,
		data.ScheduleTime, scheduleDaysJSON, data.StartTime, data.EndTime, data.IntervalMinutes,
		data.DelaySeconds, data.DedupeHours, data.CouponID, now, now,
		data.Texts, data.CTAs, data.CTAIDs, data.MlLinkListID, data.ListItemsPerRun, data.ListIncludeImage, data.PriorityType,
		data.ListSendAsImage, instanceIDArg, instanceIDsArg, data.ApplyCouponRules, chipOverridesJSON,
		data.RepeatWhenFinished, data.StartDate, data.EndDate, data.OfferGenerateSendAsImage,
		data.SearchSendAsImage, data.MonitoringSendAsImage, data.HumanizeMode,
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

	// Convert TEXT[] fields: pgx handles []string natively for TEXT[] columns,
	// but they arrive as []string from the update map — no conversion needed.
	// Just ensure they're not accidentally JSON-marshaled below.

	// Convert JSONB array fields to JSON
	for _, key := range []string{"group_ids", "send_groups", "monitor_group_ids", "categories", "stores", "schedule_days"} {
		if val, ok := updates[key]; ok {
			if arr, ok := val.([]string); ok {
				jsonBytes := marshalJSONField(r.logger, arr, key, []byte("[]"))
				updates[key] = string(jsonBytes)
			}
		}
	}

	// Convert monitoring_text_rules to JSON
	if val, ok := updates["monitoring_text_rules"]; ok {
		if rules, ok := val.(*models.MonitoringTextRules); ok {
			jsonBytes := marshalJSONField(r.logger, rules, "monitoring_text_rules", []byte("null"))
			updates["monitoring_text_rules"] = string(jsonBytes)
		}
	}

	// instance_ids arrives as []string from ToMap. Convert to []int so pgx
	// can write the INTEGER[] column. Empty / non-positive entries are
	// dropped — never write 0 (FK to a non-existent row). Empty result
	// becomes nil so the column is set to NULL (clear preference).
	if val, ok := updates["instance_ids"]; ok {
		if arr, isStrSlice := val.([]string); isStrSlice {
			ids := make([]int, 0, len(arr))
			for _, s := range arr {
				if s == "" {
					continue
				}
				var n int
				if _, err := fmt.Sscanf(s, "%d", &n); err == nil && n > 0 {
					ids = append(ids, n)
				}
			}
			if len(ids) == 0 {
				updates["instance_ids"] = nil
			} else {
				updates["instance_ids"] = ids
			}
		}
	}

	// chip_overrides (050) arrives as map[string]string from ToMap.
	// Marshal to JSONB string for pgx. nil/empty map → "{}" so the
	// NOT NULL constraint holds and the dispatch path treats it as
	// "no overrides".
	if val, ok := updates["chip_overrides"]; ok {
		if m, isMap := val.(map[string]string); isMap {
			jsonBytes := marshalJSONField(r.logger, m, "chip_overrides", []byte("{}"))
			updates["chip_overrides"] = string(jsonBytes)
		} else if val == nil {
			updates["chip_overrides"] = "{}"
		}
	}

	// instance_id arrives as the wire-format string from the update map
	// (or already nil from ToMap when the user cleared it). Coerce strings
	// to int so the FK comparison works; unparseable / non-positive values
	// become NULL, matching the "no preference" default.
	if val, ok := updates["instance_id"]; ok {
		switch v := val.(type) {
		case nil:
			// already correct
		case string:
			if v == "" {
				updates["instance_id"] = nil
			} else {
				var n int
				if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 {
					updates["instance_id"] = n
				} else {
					updates["instance_id"] = nil
				}
			}
		}
	}

	// Defensive sanitize against a dead chip reference. When a user re-pairs a
	// chip, the old instances row is deleted and a new one takes its place with
	// a fresh serial id. The automation's instance_ids array (INTEGER[], no
	// per-element FK) keeps pointing at the deleted id, and on the next save the
	// handler derives the legacy scalar instance_id = instance_ids[0] — which
	// still carries the FK automations_instance_id_fkey. Writing a dead id there
	// blows up the whole UPDATE (SQLSTATE 23503), so the user can't save *any*
	// change to the automation. Drop ids that no longer exist for this owner so
	// the save self-heals: valid chips are kept, dead ones fall back to NULL
	// ("no preference" → default chip).
	if err := r.dropDeadInstanceIDs(ctx, automationID, updates); err != nil {
		// Best-effort: a lookup failure shouldn't block the save. Worst case the
		// original FK error surfaces below, which is the pre-existing behavior.
		r.logger.Warn("Failed to sanitize instance ids on update",
			zap.String("automationID", automationID),
			zap.Error(err))
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

// dropDeadInstanceIDs rewrites the instance_id / instance_ids entries in the
// updates map so they only reference chips that still exist for the owner of
// automationID. Ids that don't resolve (e.g. a chip deleted on re-pair) are
// removed; an emptied list / scalar collapses to nil (NULL → default chip).
// It only touches keys already present in updates, so it never introduces a
// write the caller didn't intend. No-op when neither key carries a positive id.
func (r *automationRepository) dropDeadInstanceIDs(ctx context.Context, automationID string, updates map[string]interface{}) error {
	scalarID, hasScalar := 0, false
	if v, ok := updates["instance_id"]; ok {
		if n, isInt := v.(int); isInt && n > 0 {
			scalarID, hasScalar = n, true
		}
	}

	var listIDs []int
	if v, ok := updates["instance_ids"]; ok {
		if ids, isIntSlice := v.([]int); isIntSlice {
			listIDs = ids
		}
	}

	if !hasScalar && len(listIDs) == 0 {
		return nil
	}

	// Collect the distinct candidate ids to check in a single round-trip.
	candidateSet := make(map[int]struct{}, len(listIDs)+1)
	if hasScalar {
		candidateSet[scalarID] = struct{}{}
	}
	for _, id := range listIDs {
		candidateSet[id] = struct{}{}
	}
	candidates := make([]int, 0, len(candidateSet))
	for id := range candidateSet {
		candidates = append(candidates, id)
	}

	rows, err := r.db.pool.Query(ctx,
		`SELECT id FROM instances
		 WHERE id = ANY($1)
		   AND user_id = (SELECT user_id FROM automations WHERE id = $2)`,
		candidates, automationID)
	if err != nil {
		return err
	}
	defer rows.Close()

	valid := make(map[int]struct{}, len(candidates))
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return err
		}
		valid[id] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	// Rewrite the scalar: keep only if it still exists, else clear.
	if hasScalar {
		if _, ok := valid[scalarID]; !ok {
			updates["instance_id"] = nil
		}
	}

	// Rewrite the list: drop dead ids preserving order; empty → NULL.
	if _, ok := updates["instance_ids"]; ok {
		if ids, isIntSlice := updates["instance_ids"].([]int); isIntSlice {
			kept := make([]int, 0, len(ids))
			for _, id := range ids {
				if _, ok := valid[id]; ok {
					kept = append(kept, id)
				}
			}
			if len(kept) == 0 {
				updates["instance_ids"] = nil
			} else {
				updates["instance_ids"] = kept
			}
		}
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
		       coupon_id, created_at, updated_at,
		       texts, ctas, cta_ids, last_error, last_error_at,
		       ml_link_list_id, list_items_per_run, list_include_image, priority_type,
		       list_send_as_image, instance_id, instance_ids, apply_coupon_rules, chip_overrides,
		       repeat_when_finished, start_date, end_date, offer_generate_send_as_image,
		       search_send_as_image, monitoring_send_as_image, humanize_mode
		FROM automations
		WHERE status = 'active'
		  AND type = 'monitoring'
		  AND monitor_group_ids @> $1::jsonb
	`

	groupIDJSON := marshalJSONField(r.logger, []string{waGroupID}, "monitor_group_filter", []byte(`[""]`))

	rows, err := r.db.Query(ctx, query, string(groupIDJSON))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanAutomations(rows)
}

func (r *automationRepository) GetOfferGenerateAutomationsForGroup(ctx context.Context, waGroupID string) ([]models.Automation, error) {
	query := `
		SELECT id, user_id, name, type, status, mode, shopee_link, keyword,
		       group_ids, send_groups, monitor_group_ids, monitoring_text_rules,
		       categories, stores, min_price, max_price, min_discount, priority,
		       custom_message, include_image, dispatch_text_id, filter_type,
		       schedule_time, schedule_days, start_time, end_time, interval_minutes,
		       delay_seconds, dedupe_hours, last_run_at, next_run_at,
		       coupon_id, created_at, updated_at,
		       texts, ctas, cta_ids, last_error, last_error_at,
		       ml_link_list_id, list_items_per_run, list_include_image, priority_type,
		       list_send_as_image, instance_id, instance_ids, apply_coupon_rules, chip_overrides,
		       repeat_when_finished, start_date, end_date, offer_generate_send_as_image,
		       search_send_as_image, monitoring_send_as_image, humanize_mode
		FROM automations
		WHERE status = 'active'
		  AND type = 'offer_generate'
		  AND monitor_group_ids @> $1::jsonb
	`

	groupIDJSON := marshalJSONField(r.logger, []string{waGroupID}, "monitor_group_filter", []byte(`[""]`))

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
	var priority *int
	var includeImage *bool
	var filterType *string
	// Local nullable vars to safely handle NULL TEXT columns (pgx NULL scan fix)
	var shopeeLink, keyword, customMessage, dispatchTextID, couponID *string
	var automationType, mode *string
	// New columns: texts, ctas, cta_ids, last_error, last_error_at, ml_link_list_id, list_items_per_run, list_include_image, priority_type
	var texts, ctas, ctaIDs []string
	var lastError, mlLinkListID, priorityType *string
	var lastErrorAt *time.Time
	var listItemsPerRun *int
	var listIncludeImage *bool
	var listSendAsImage *bool
	var offerGenerateSendAsImage *bool
	var searchSendAsImage *bool
	var monitoringSendAsImage *bool
	var instanceID *int
	var instanceIDs []int
	var chipOverridesJSON []byte
	var repeatWhenFinished bool
	var startDate *time.Time
	var endDate *time.Time
	var humanizeMode *string

	err := row.Scan(
		&a.ID, &a.UserID, &a.Name, &automationType, &a.Status, &mode, &shopeeLink, &keyword,
		&groupIDsJSON, &sendGroupsJSON, &monitorGroupIDsJSON, &monitoringRulesJSON,
		&categoriesJSON, &storesJSON, &a.MinPrice, &a.MaxPrice, &a.MinDiscount, &priority,
		&customMessage, &includeImage, &dispatchTextID, &filterType,
		&scheduleTime, &scheduleDaysJSON, &startTime, &endTime, &a.IntervalMinutes,
		&a.DelaySeconds, &a.DedupeHours, &lastRunAt, &nextRunAt,
		&couponID, &createdAt, &updatedAt,
		&texts, &ctas, &ctaIDs, &lastError, &lastErrorAt,
		&mlLinkListID, &listItemsPerRun, &listIncludeImage, &priorityType,
		&listSendAsImage, &instanceID, &instanceIDs, &a.ApplyCouponRules, &chipOverridesJSON,
		&repeatWhenFinished, &startDate, &endDate, &offerGenerateSendAsImage,
		&searchSendAsImage, &monitoringSendAsImage, &humanizeMode,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	a.Type = automationType
	a.Mode = mode
	a.ShopeeLink = shopeeLink
	a.Keyword = keyword
	a.CustomMessage = customMessage
	a.DispatchTextID = dispatchTextID
	a.CouponID = couponID
	a.FilterType = filterType
	a.Texts = texts
	a.CTAs = ctas
	a.CTAIDs = ctaIDs
	a.LastError = lastError
	a.MlLinkListID = mlLinkListID
	a.PriorityType = priorityType
	a.ListItemsPerRun = listItemsPerRun
	a.ListIncludeImage = listIncludeImage
	a.ListSendAsImage = listSendAsImage
	a.OfferGenerateSendAsImage = offerGenerateSendAsImage
	a.SearchSendAsImage = searchSendAsImage
	a.MonitoringSendAsImage = monitoringSendAsImage
	a.RepeatWhenFinished = repeatWhenFinished
	a.HumanizeMode = humanizeMode
	if startDate != nil {
		t := startDate.Format("2006-01-02")
		a.StartDate = &t
	}
	if endDate != nil {
		t := endDate.Format("2006-01-02")
		a.EndDate = &t
	}
	if instanceID != nil {
		s := fmt.Sprintf("%d", *instanceID)
		a.InstanceID = &s
	}
	if len(instanceIDs) > 0 {
		ids := make([]string, len(instanceIDs))
		for i, n := range instanceIDs {
			ids[i] = fmt.Sprintf("%d", n)
		}
		a.InstanceIDs = ids
	}
	if len(chipOverridesJSON) > 0 {
		var overrides map[string]string
		unmarshalJSONField(r.logger, chipOverridesJSON, &overrides, "chip_overrides", a.ID)
		a.ChipOverrides = overrides
	}
	if includeImage != nil {
		a.IncludeImage = *includeImage
	}
	if lastErrorAt != nil {
		t := lastErrorAt.Format(time.RFC3339)
		a.LastErrorAt = &t
	}

	// Unmarshal JSON arrays with error capture
	if groupIDsJSON != nil {
		unmarshalJSONField(r.logger, groupIDsJSON, &a.GroupIDs, "group_ids", a.ID)
	}
	if sendGroupsJSON != nil {
		unmarshalJSONField(r.logger, sendGroupsJSON, &a.SendGroups, "send_groups", a.ID)
	}
	if monitorGroupIDsJSON != nil {
		unmarshalJSONField(r.logger, monitorGroupIDsJSON, &a.MonitorGroupIDs, "monitor_group_ids", a.ID)
	}
	if categoriesJSON != nil {
		unmarshalJSONField(r.logger, categoriesJSON, &a.Categories, "categories", a.ID)
	}
	if storesJSON != nil {
		unmarshalJSONField(r.logger, storesJSON, &a.Stores, "stores", a.ID)
	}
	if scheduleDaysJSON != nil {
		unmarshalJSONField(r.logger, scheduleDaysJSON, &a.ScheduleDays, "schedule_days", a.ID)
	}
	if monitoringRulesJSON != nil {
		var rules models.MonitoringTextRules
		unmarshalJSONField(r.logger, monitoringRulesJSON, &rules, "monitoring_text_rules", a.ID)
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
		var priority *int
		var includeImage *bool
		var filterType *string
		// Local nullable vars to safely handle NULL TEXT columns (pgx NULL scan fix)
		var shopeeLink, keyword, customMessage, dispatchTextID, couponID *string
		var automationType, mode *string
		// New columns: texts, ctas, cta_ids, last_error, last_error_at, ml_link_list_id, list_items_per_run, list_include_image, priority_type
		var texts, ctas, ctaIDs []string
		var lastError, mlLinkListID, priorityType *string
		var lastErrorAt *time.Time
		var listItemsPerRun *int
		var listIncludeImage *bool
		var listSendAsImage *bool
		var offerGenerateSendAsImage *bool
		var searchSendAsImage *bool
		var monitoringSendAsImage *bool
		var instanceID *int
		var instanceIDs []int
		var chipOverridesJSON []byte
		var repeatWhenFinished bool
		var startDate *time.Time
		var endDate *time.Time
		var humanizeMode *string

		err := rows.Scan(
			&a.ID, &a.UserID, &a.Name, &automationType, &a.Status, &mode, &shopeeLink, &keyword,
			&groupIDsJSON, &sendGroupsJSON, &monitorGroupIDsJSON, &monitoringRulesJSON,
			&categoriesJSON, &storesJSON, &a.MinPrice, &a.MaxPrice, &a.MinDiscount, &priority,
			&customMessage, &includeImage, &dispatchTextID, &filterType,
			&scheduleTime, &scheduleDaysJSON, &startTime, &endTime, &a.IntervalMinutes,
			&a.DelaySeconds, &a.DedupeHours, &lastRunAt, &nextRunAt,
			&couponID, &createdAt, &updatedAt,
			&texts, &ctas, &ctaIDs, &lastError, &lastErrorAt,
			&mlLinkListID, &listItemsPerRun, &listIncludeImage, &priorityType,
			&listSendAsImage, &instanceID, &instanceIDs, &a.ApplyCouponRules, &chipOverridesJSON,
			&repeatWhenFinished, &startDate, &endDate, &offerGenerateSendAsImage,
			&searchSendAsImage, &monitoringSendAsImage, &humanizeMode,
		)
		if err != nil {
			return nil, err
		}

		a.Type = automationType
		a.Mode = mode
		a.ShopeeLink = shopeeLink
		a.Keyword = keyword
		a.CustomMessage = customMessage
		a.DispatchTextID = dispatchTextID
		a.CouponID = couponID
		a.FilterType = filterType
		a.Texts = texts
		a.CTAs = ctas
		a.CTAIDs = ctaIDs
		a.LastError = lastError
		a.MlLinkListID = mlLinkListID
		a.PriorityType = priorityType
		a.ListItemsPerRun = listItemsPerRun
		a.ListIncludeImage = listIncludeImage
		a.ListSendAsImage = listSendAsImage
		a.OfferGenerateSendAsImage = offerGenerateSendAsImage
		a.SearchSendAsImage = searchSendAsImage
		a.MonitoringSendAsImage = monitoringSendAsImage
		a.RepeatWhenFinished = repeatWhenFinished
		a.HumanizeMode = humanizeMode
		if startDate != nil {
			t := startDate.Format("2006-01-02")
			a.StartDate = &t
		}
		if endDate != nil {
			t := endDate.Format("2006-01-02")
			a.EndDate = &t
		}
		if instanceID != nil {
			s := fmt.Sprintf("%d", *instanceID)
			a.InstanceID = &s
		}
		if len(instanceIDs) > 0 {
			ids := make([]string, len(instanceIDs))
			for i, n := range instanceIDs {
				ids[i] = fmt.Sprintf("%d", n)
			}
			a.InstanceIDs = ids
		}
		if len(chipOverridesJSON) > 0 {
			var overrides map[string]string
			unmarshalJSONField(r.logger, chipOverridesJSON, &overrides, "chip_overrides", a.ID)
			a.ChipOverrides = overrides
		}
		if includeImage != nil {
			a.IncludeImage = *includeImage
		}
		if lastErrorAt != nil {
			t := lastErrorAt.Format(time.RFC3339)
			a.LastErrorAt = &t
		}

		// Unmarshal JSON arrays with error capture
		if groupIDsJSON != nil {
			unmarshalJSONField(r.logger, groupIDsJSON, &a.GroupIDs, "group_ids", a.ID)
		}
		if sendGroupsJSON != nil {
			unmarshalJSONField(r.logger, sendGroupsJSON, &a.SendGroups, "send_groups", a.ID)
		}
		if monitorGroupIDsJSON != nil {
			unmarshalJSONField(r.logger, monitorGroupIDsJSON, &a.MonitorGroupIDs, "monitor_group_ids", a.ID)
		}
		if categoriesJSON != nil {
			unmarshalJSONField(r.logger, categoriesJSON, &a.Categories, "categories", a.ID)
		}
		if storesJSON != nil {
			unmarshalJSONField(r.logger, storesJSON, &a.Stores, "stores", a.ID)
		}
		if scheduleDaysJSON != nil {
			unmarshalJSONField(r.logger, scheduleDaysJSON, &a.ScheduleDays, "schedule_days", a.ID)
		}
		if monitoringRulesJSON != nil {
			var rules models.MonitoringTextRules
			unmarshalJSONField(r.logger, monitoringRulesJSON, &rules, "monitoring_text_rules", a.ID)
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
