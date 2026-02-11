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

type actionHistoryRepository struct {
	db     *DB
	logger *zap.Logger
}

// NewActionHistoryRepository creates a new PostgreSQL action history repository
func NewActionHistoryRepository(db *DB, logger *zap.Logger) repository.ActionHistoryRepository {
	return &actionHistoryRepository{
		db:     db,
		logger: logger,
	}
}

func (r *actionHistoryRepository) Create(ctx context.Context, userID, actionType, description string, details map[string]interface{}) (*models.ActionHistory, error) {
	now := time.Now().UTC()

	var detailsJSON []byte
	if details != nil {
		var marshalErr error
		detailsJSON, marshalErr = json.Marshal(details)
		if marshalErr != nil {
			observability.LogAndCapture(r.logger, sentry.LevelWarning,
				"Failed to marshal action history details",
				marshalErr, map[string]string{
					"module": "action_history", "operation": "create_marshal",
				})
			detailsJSON = []byte("null")
		}
	}

	query := `
		INSERT INTO action_history (user_id, action_type, description, details, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at
	`

	var id string
	var createdAt time.Time

	err := r.db.QueryRow(ctx, query, userID, actionType, description, detailsJSON, models.ActionStatusPending, now).Scan(&id, &createdAt)
	if err != nil {
		r.logger.Error("Failed to create action history",
			zap.String("userID", userID),
			zap.String("actionType", actionType),
			zap.Error(err))
		return nil, err
	}

	history := &models.ActionHistory{
		ID:          id,
		UserID:      userID,
		ActionType:  actionType,
		Description: description,
		Details:     details,
		Status:      models.ActionStatusPending,
		CreatedAt:   createdAt.Format(time.RFC3339),
	}

	return history, nil
}

func (r *actionHistoryRepository) Update(ctx context.Context, historyID string, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return nil
	}

	// Build dynamic UPDATE query
	setClauses := ""
	args := make([]interface{}, 0, len(updates)+1)
	argIndex := 1

	for key, value := range updates {
		if setClauses != "" {
			setClauses += ", "
		}

		// Handle JSON fields
		if key == "details" {
			if detailsMap, ok := value.(map[string]interface{}); ok {
				jsonBytes, marshalErr := json.Marshal(detailsMap)
				if marshalErr != nil {
					observability.LogAndCapture(r.logger, sentry.LevelWarning,
						"Failed to marshal action history update details",
						marshalErr, map[string]string{
							"module": "action_history", "operation": "update_marshal",
							"history_id": historyID,
						})
					jsonBytes = []byte("null")
				}
				value = string(jsonBytes)
			}
		}

		setClauses += fmt.Sprintf("%s = $%d", key, argIndex)
		args = append(args, value)
		argIndex++
	}

	// Add historyID as the last argument
	args = append(args, historyID)

	query := fmt.Sprintf("UPDATE action_history SET %s WHERE id = $%d", setClauses, argIndex)

	_, err := r.db.pool.Exec(ctx, query, args...)
	if err != nil {
		r.logger.Error("Failed to update action history",
			zap.String("historyID", historyID),
			zap.Error(err))
		return err
	}

	return nil
}

func (r *actionHistoryRepository) GetByID(ctx context.Context, historyID string) (*models.ActionHistory, error) {
	query := `
		SELECT id, user_id, action_type, description, details, status, error_message,
		       target_count, success_count, error_count, created_at, completed_at
		FROM action_history
		WHERE id = $1
	`

	var h models.ActionHistory
	var detailsJSON []byte
	var createdAt time.Time
	var completedAt *time.Time

	err := r.db.QueryRow(ctx, query, historyID).Scan(
		&h.ID, &h.UserID, &h.ActionType, &h.Description, &detailsJSON, &h.Status, &h.ErrorMessage,
		&h.TargetCount, &h.SuccessCount, &h.ErrorCount, &createdAt, &completedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	if detailsJSON != nil {
		var details map[string]interface{}
		if unmarshalErr := json.Unmarshal(detailsJSON, &details); unmarshalErr != nil {
			observability.LogAndCapture(r.logger, sentry.LevelWarning,
				"Failed to unmarshal action history details",
				unmarshalErr, map[string]string{
					"module": "action_history", "operation": "get_by_id_unmarshal",
					"history_id": historyID,
				})
		}
		h.Details = details
	}

	h.CreatedAt = createdAt.Format(time.RFC3339)
	if completedAt != nil {
		t := completedAt.Format(time.RFC3339)
		h.CompletedAt = &t
	}

	return &h, nil
}

func (r *actionHistoryRepository) GetUserHistory(ctx context.Context, userID string, limit, offset int) ([]models.ActionHistory, error) {
	if limit <= 0 {
		limit = 50
	}

	query := `
		SELECT id, user_id, action_type, description, details, status, error_message,
		       target_count, success_count, error_count, created_at, completed_at
		FROM action_history
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var histories []models.ActionHistory
	for rows.Next() {
		var h models.ActionHistory
		var detailsJSON []byte
		var createdAt time.Time
		var completedAt *time.Time

		err := rows.Scan(
			&h.ID, &h.UserID, &h.ActionType, &h.Description, &detailsJSON, &h.Status, &h.ErrorMessage,
			&h.TargetCount, &h.SuccessCount, &h.ErrorCount, &createdAt, &completedAt,
		)
		if err != nil {
			return nil, err
		}

		if detailsJSON != nil {
			var details map[string]interface{}
			if unmarshalErr := json.Unmarshal(detailsJSON, &details); unmarshalErr != nil {
				observability.LogAndCapture(r.logger, sentry.LevelWarning,
					"Failed to unmarshal action history details",
					unmarshalErr, map[string]string{
						"module": "action_history", "operation": "get_user_history_unmarshal",
					})
			}
			h.Details = details
		}

		h.CreatedAt = createdAt.Format(time.RFC3339)
		if completedAt != nil {
			t := completedAt.Format(time.RFC3339)
			h.CompletedAt = &t
		}

		histories = append(histories, h)
	}

	return histories, rows.Err()
}

// MarkCompleted marks an action as completed
func (r *actionHistoryRepository) MarkCompleted(ctx context.Context, historyID string) error {
	query := `
		UPDATE action_history
		SET status = $1, completed_at = $2
		WHERE id = $3
	`

	_, err := r.db.pool.Exec(ctx, query, models.ActionStatusCompleted, time.Now().UTC(), historyID)
	return err
}

// MarkFailed marks an action as failed with an error message
func (r *actionHistoryRepository) MarkFailed(ctx context.Context, historyID, errorMessage string) error {
	query := `
		UPDATE action_history
		SET status = $1, error_message = $2, completed_at = $3
		WHERE id = $4
	`

	_, err := r.db.pool.Exec(ctx, query, models.ActionStatusFailed, errorMessage, time.Now().UTC(), historyID)
	return err
}

// MarkRunning marks an action as running
func (r *actionHistoryRepository) MarkRunning(ctx context.Context, historyID string) error {
	query := `
		UPDATE action_history
		SET status = $1
		WHERE id = $2
	`

	_, err := r.db.pool.Exec(ctx, query, models.ActionStatusRunning, historyID)
	return err
}

// UpdateStatus updates the status and optional error message of an action history entry
func (r *actionHistoryRepository) UpdateStatus(ctx context.Context, id, status, errorMessage string) error {
	now := time.Now().UTC()

	var query string
	var args []interface{}

	if status == models.ActionStatusCompleted || status == models.ActionStatusFailed {
		query = `
			UPDATE action_history
			SET status = $1, error_message = $2, completed_at = $3
			WHERE id = $4
		`
		var errMsg *string
		if errorMessage != "" {
			errMsg = &errorMessage
		}
		args = []interface{}{status, errMsg, now, id}
	} else {
		query = `
			UPDATE action_history
			SET status = $1, error_message = $2
			WHERE id = $3
		`
		var errMsg *string
		if errorMessage != "" {
			errMsg = &errorMessage
		}
		args = []interface{}{status, errMsg, id}
	}

	_, err := r.db.pool.Exec(ctx, query, args...)
	if err != nil {
		r.logger.Error("Failed to update action history status",
			zap.String("historyID", id),
			zap.String("status", status),
			zap.Error(err))
		return err
	}

	return nil
}

// GetByUserID retrieves history entries by user ID (alias for GetUserHistory for interface compatibility)
func (r *actionHistoryRepository) GetByUserID(ctx context.Context, userID string, limit, offset int) ([]models.ActionHistory, error) {
	return r.GetUserHistory(ctx, userID, limit, offset)
}

// GetByActionType retrieves history entries by action type
func (r *actionHistoryRepository) GetByActionType(ctx context.Context, userID, actionType string, limit int) ([]models.ActionHistory, error) {
	if limit <= 0 {
		limit = 20
	}

	query := `
		SELECT id, user_id, action_type, description, details, status, error_message,
		       target_count, success_count, error_count, created_at, completed_at
		FROM action_history
		WHERE user_id = $1 AND action_type = $2
		ORDER BY created_at DESC
		LIMIT $3
	`

	rows, err := r.db.Query(ctx, query, userID, actionType, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var histories []models.ActionHistory
	for rows.Next() {
		var h models.ActionHistory
		var detailsJSON []byte
		var createdAt time.Time
		var completedAt *time.Time

		err := rows.Scan(
			&h.ID, &h.UserID, &h.ActionType, &h.Description, &detailsJSON, &h.Status, &h.ErrorMessage,
			&h.TargetCount, &h.SuccessCount, &h.ErrorCount, &createdAt, &completedAt,
		)
		if err != nil {
			return nil, err
		}

		if detailsJSON != nil {
			var details map[string]interface{}
			if unmarshalErr := json.Unmarshal(detailsJSON, &details); unmarshalErr != nil {
				observability.LogAndCapture(r.logger, sentry.LevelWarning,
					"Failed to unmarshal action history details",
					unmarshalErr, map[string]string{
						"module": "action_history", "operation": "get_by_action_type_unmarshal",
					})
			}
			h.Details = details
		}

		h.CreatedAt = createdAt.Format(time.RFC3339)
		if completedAt != nil {
			t := completedAt.Format(time.RFC3339)
			h.CompletedAt = &t
		}

		histories = append(histories, h)
	}

	return histories, rows.Err()
}

// IncrementSuccessCount increments the success count for an action history entry
func (r *actionHistoryRepository) IncrementSuccessCount(ctx context.Context, historyID string) error {
	query := `UPDATE action_history SET success_count = COALESCE(success_count, 0) + 1 WHERE id = $1`
	_, err := r.db.pool.Exec(ctx, query, historyID)
	return err
}

// IncrementErrorCount increments the error count for an action history entry
func (r *actionHistoryRepository) IncrementErrorCount(ctx context.Context, historyID string, errorMsg string) error {
	query := `UPDATE action_history SET error_count = COALESCE(error_count, 0) + 1, error_message = $2 WHERE id = $1`
	_, err := r.db.pool.Exec(ctx, query, historyID, errorMsg)
	return err
}

// UpdateCounts updates the success and error counts for an action history entry
func (r *actionHistoryRepository) UpdateCounts(ctx context.Context, historyID string, successCount, errorCount int32) error {
	query := `UPDATE action_history SET success_count = $2, error_count = $3 WHERE id = $1`
	_, err := r.db.pool.Exec(ctx, query, historyID, successCount, errorCount)
	return err
}

// DeleteUserHistory deletes history entries for a user with optional filters
func (r *actionHistoryRepository) DeleteUserHistory(ctx context.Context, userID string, beforeDate *time.Time, actionTypes []string) (int64, error) {
	query := `DELETE FROM action_history WHERE user_id = $1`
	args := []interface{}{userID}
	argIndex := 2

	if beforeDate != nil {
		query += fmt.Sprintf(" AND created_at < $%d", argIndex)
		args = append(args, *beforeDate)
		argIndex++
	}

	if len(actionTypes) > 0 {
		query += fmt.Sprintf(" AND action_type = ANY($%d)", argIndex)
		args = append(args, actionTypes)
	}

	result, err := r.db.pool.Exec(ctx, query, args...)
	if err != nil {
		r.logger.Error("Failed to delete user history",
			zap.String("userID", userID),
			zap.Error(err))
		return 0, err
	}

	return result.RowsAffected(), nil
}

// GetUserHistoryFiltered retrieves history entries with advanced filtering
func (r *actionHistoryRepository) GetUserHistoryFiltered(ctx context.Context, req models.HistoryListRequest, userID string) ([]models.ActionHistory, error) {
	if req.Limit <= 0 {
		req.Limit = 50
	}

	query := `
		SELECT id, user_id, action_type, description, details, status, error_message,
		       target_count, success_count, error_count, created_at, completed_at
		FROM action_history
		WHERE user_id = $1
	`
	args := []interface{}{userID}
	argIndex := 2

	if req.ActionType != nil && *req.ActionType != "" {
		query += fmt.Sprintf(" AND action_type = $%d", argIndex)
		args = append(args, *req.ActionType)
		argIndex++
	}

	if req.Status != nil && *req.Status != "" {
		query += fmt.Sprintf(" AND status = $%d", argIndex)
		args = append(args, *req.Status)
		argIndex++
	}

	query += " ORDER BY created_at DESC"
	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argIndex, argIndex+1)
	args = append(args, req.Limit, req.Offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var histories []models.ActionHistory
	for rows.Next() {
		var h models.ActionHistory
		var detailsJSON []byte
		var createdAt time.Time
		var completedAt *time.Time

		err := rows.Scan(
			&h.ID, &h.UserID, &h.ActionType, &h.Description, &detailsJSON, &h.Status, &h.ErrorMessage,
			&h.TargetCount, &h.SuccessCount, &h.ErrorCount, &createdAt, &completedAt,
		)
		if err != nil {
			return nil, err
		}

		if detailsJSON != nil {
			var details map[string]interface{}
			if unmarshalErr := json.Unmarshal(detailsJSON, &details); unmarshalErr != nil {
				observability.LogAndCapture(r.logger, sentry.LevelWarning,
					"Failed to unmarshal action history details",
					unmarshalErr, map[string]string{
						"module": "action_history", "operation": "get_filtered_unmarshal",
					})
			}
			h.Details = details
		}

		h.CreatedAt = createdAt.Format(time.RFC3339)
		if completedAt != nil {
			t := completedAt.Format(time.RFC3339)
			h.CompletedAt = &t
		}

		histories = append(histories, h)
	}

	return histories, rows.Err()
}
