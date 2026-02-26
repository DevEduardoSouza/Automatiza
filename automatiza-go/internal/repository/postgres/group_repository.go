package postgres

import (
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

type groupRepository struct {
	db     *DB
	logger *zap.Logger
}

// NewGroupRepository creates a new PostgreSQL group repository
func NewGroupRepository(db *DB, logger *zap.Logger) repository.GroupRepository {
	return &groupRepository{
		db:     db,
		logger: logger,
	}
}

func (r *groupRepository) GetUserGroups(ctx context.Context, userID string, adminOnly bool) ([]models.Group, error) {
	query := `
		SELECT id, user_id, instance_id, wa_group_id, name, description, picture_url,
		       members_count, member_limit, status, is_admin, invite_code, admin_only,
		       is_favorite, created_at, updated_at
		FROM groups
		WHERE user_id = $1
	`
	args := []interface{}{userID}

	if adminOnly {
		query += " AND is_admin = true"
	}

	query += " ORDER BY name ASC"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanGroups(rows)
}

func (r *groupRepository) GetUserGroupsPaginated(ctx context.Context, userID string, page, pageSize int, adminOnly bool, search string) (*repository.GroupPaginatedResult, error) {
	// Validate and set defaults
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	// Build WHERE clause
	whereClause := "WHERE user_id = $1"
	args := []interface{}{userID}
	argIndex := 2

	if adminOnly {
		whereClause += " AND is_admin = true"
	}

	if search != "" {
		whereClause += fmt.Sprintf(" AND name ILIKE $%d", argIndex)
		args = append(args, "%"+search+"%")
		argIndex++
	}

	// Count total matching records
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM groups %s", whereClause)
	var totalCount int
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&totalCount); err != nil {
		return nil, fmt.Errorf("failed to count groups: %w", err)
	}

	// Calculate pagination
	totalPages := (totalCount + pageSize - 1) / pageSize
	if totalPages < 1 {
		totalPages = 1
	}
	offset := (page - 1) * pageSize
	hasMore := page < totalPages

	// Fetch paginated data
	dataQuery := fmt.Sprintf(`
		SELECT id, user_id, instance_id, wa_group_id, name, description, picture_url,
		       members_count, member_limit, status, is_admin, invite_code, admin_only,
		       is_favorite, created_at, updated_at
		FROM groups
		%s
		ORDER BY name ASC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIndex, argIndex+1)

	args = append(args, pageSize, offset)

	rows, err := r.db.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch groups: %w", err)
	}
	defer rows.Close()

	groups, err := r.scanGroups(rows)
	if err != nil {
		return nil, err
	}

	return &repository.GroupPaginatedResult{
		Groups:     groups,
		TotalCount: totalCount,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
		HasMore:    hasMore,
	}, nil
}

func (r *groupRepository) GetGroupByID(ctx context.Context, groupID string) (*models.Group, error) {
	query := `
		SELECT id, user_id, instance_id, wa_group_id, name, description, picture_url,
		       members_count, member_limit, status, is_admin, invite_code, admin_only,
		       is_favorite, created_at, updated_at
		FROM groups
		WHERE id = $1
	`

	return r.scanGroup(r.db.QueryRow(ctx, query, groupID))
}

func (r *groupRepository) GetGroupByWAGroupID(ctx context.Context, userID, waGroupID string) (*models.Group, error) {
	query := `
		SELECT id, user_id, instance_id, wa_group_id, name, description, picture_url,
		       members_count, member_limit, status, is_admin, invite_code, admin_only,
		       is_favorite, created_at, updated_at
		FROM groups
		WHERE user_id = $1 AND wa_group_id = $2
	`

	return r.scanGroup(r.db.QueryRow(ctx, query, userID, waGroupID))
}

func (r *groupRepository) GetGroupsByIDs(ctx context.Context, userID string, groupIDs []string) ([]models.Group, error) {
	if len(groupIDs) == 0 {
		return []models.Group{}, nil
	}

	query := `
		SELECT id, user_id, instance_id, wa_group_id, name, description, picture_url,
		       members_count, member_limit, status, is_admin, invite_code, admin_only,
		       is_favorite, created_at, updated_at
		FROM groups
		WHERE user_id = $1 AND id = ANY($2)
		ORDER BY name ASC
	`

	rows, err := r.db.Query(ctx, query, userID, groupIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanGroups(rows)
}

func (r *groupRepository) SyncGroups(ctx context.Context, userID string, instanceID int, groups []models.Group, options repository.SyncOptions) error {
	if len(groups) == 0 {
		return nil
	}

	return r.db.WithTx(ctx, func(tx pgx.Tx) error {
		// Get existing groups for this instance
		existingQuery := `
			SELECT wa_group_id FROM groups
			WHERE user_id = $1 AND instance_id = $2
		`
		existingRows, err := tx.Query(ctx, existingQuery, userID, instanceID)
		if err != nil {
			return err
		}

		existingIDs := make(map[string]bool)
		for existingRows.Next() {
			var waGroupID string
			if err := existingRows.Scan(&waGroupID); err != nil {
				existingRows.Close()
				return err
			}
			existingIDs[waGroupID] = true
		}
		existingRows.Close()

		// Track which IDs we're syncing
		syncedIDs := make(map[string]bool)

		// Upsert groups
		upsertQuery := `
			INSERT INTO groups (user_id, instance_id, wa_group_id, name, description, picture_url,
			                    members_count, member_limit, is_admin, admin_only, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
			ON CONFLICT (user_id, wa_group_id) DO UPDATE SET
				instance_id = EXCLUDED.instance_id,
				name = EXCLUDED.name,
				description = EXCLUDED.description,
				picture_url = EXCLUDED.picture_url,
				members_count = EXCLUDED.members_count,
				member_limit = EXCLUDED.member_limit,
				is_admin = EXCLUDED.is_admin,
				admin_only = EXCLUDED.admin_only,
				updated_at = EXCLUDED.updated_at
		`

		now := time.Now().UTC()
		for _, g := range groups {
			_, err := tx.Exec(ctx, upsertQuery,
				userID, instanceID, g.WAGroupID, g.Name, g.Description, g.PictureURL,
				g.MembersCount, g.MemberLimit, g.IsAdmin, g.AdminOnly, now, now,
			)
			if err != nil {
				r.logger.Error("Failed to upsert group",
					zap.String("waGroupID", g.WAGroupID),
					zap.Error(err))
				return err
			}
			syncedIDs[g.WAGroupID] = true
		}

		// Delete groups that no longer exist
		for waGroupID := range existingIDs {
			if !syncedIDs[waGroupID] {
				deleteQuery := `
					DELETE FROM groups
					WHERE user_id = $1 AND instance_id = $2 AND wa_group_id = $3
				`
				_, err := tx.Exec(ctx, deleteQuery, userID, instanceID, waGroupID)
				if err != nil {
					r.logger.Error("Failed to delete removed group",
						zap.String("waGroupID", waGroupID),
						zap.Error(err))
					return err
				}
			}
		}

		return nil
	})
}

func (r *groupRepository) SyncGroupsWithStats(ctx context.Context, userID string, instanceID int, groups []models.Group, options repository.SyncOptions) (*repository.SyncResult, error) {
	result := &repository.SyncResult{}
	if len(groups) == 0 {
		return result, nil
	}

	err := r.db.WithTx(ctx, func(tx pgx.Tx) error {
		// Get existing groups for this instance
		existingQuery := `
			SELECT wa_group_id FROM groups
			WHERE user_id = $1 AND instance_id = $2
		`
		existingRows, err := tx.Query(ctx, existingQuery, userID, instanceID)
		if err != nil {
			return err
		}

		existingIDs := make(map[string]bool)
		for existingRows.Next() {
			var waGroupID string
			if err := existingRows.Scan(&waGroupID); err != nil {
				existingRows.Close()
				return err
			}
			existingIDs[waGroupID] = true
		}
		existingRows.Close()

		// Track which IDs we're syncing
		syncedIDs := make(map[string]bool)

		// Upsert groups
		upsertQuery := `
			INSERT INTO groups (user_id, instance_id, wa_group_id, name, description, picture_url,
			                    members_count, member_limit, is_admin, admin_only, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
			ON CONFLICT (user_id, wa_group_id) DO UPDATE SET
				instance_id = EXCLUDED.instance_id,
				name = EXCLUDED.name,
				description = EXCLUDED.description,
				picture_url = EXCLUDED.picture_url,
				members_count = EXCLUDED.members_count,
				member_limit = EXCLUDED.member_limit,
				is_admin = EXCLUDED.is_admin,
				admin_only = EXCLUDED.admin_only,
				updated_at = EXCLUDED.updated_at
		`

		now := time.Now().UTC()
		for _, g := range groups {
			_, err := tx.Exec(ctx, upsertQuery,
				userID, instanceID, g.WAGroupID, g.Name, g.Description, g.PictureURL,
				g.MembersCount, g.MemberLimit, g.IsAdmin, g.AdminOnly, now, now,
			)
			if err != nil {
				r.logger.Error("Failed to upsert group",
					zap.String("waGroupID", g.WAGroupID),
					zap.Error(err))
				return err
			}
			syncedIDs[g.WAGroupID] = true

			// Count created vs updated
			if existingIDs[g.WAGroupID] {
				result.Updated++
			} else {
				result.Created++
			}
		}

		// Delete groups that no longer exist
		for waGroupID := range existingIDs {
			if !syncedIDs[waGroupID] {
				deleteQuery := `
					DELETE FROM groups
					WHERE user_id = $1 AND instance_id = $2 AND wa_group_id = $3
				`
				_, err := tx.Exec(ctx, deleteQuery, userID, instanceID, waGroupID)
				if err != nil {
					r.logger.Error("Failed to delete removed group",
						zap.String("waGroupID", waGroupID),
						zap.Error(err))
					return err
				}
				result.Deleted++
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

func (r *groupRepository) IsUserAdminInGroup(ctx context.Context, userID, waGroupID string) (bool, error) {
	query := `
		SELECT is_admin FROM groups
		WHERE user_id = $1 AND wa_group_id = $2
	`

	var isAdmin bool
	err := r.db.QueryRow(ctx, query, userID, waGroupID).Scan(&isAdmin)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}

	return isAdmin, nil
}

func (r *groupRepository) UpdateGroup(ctx context.Context, groupID string, updates map[string]interface{}) error {
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
		setClauses += fmt.Sprintf("%s = $%d", key, argIndex)
		args = append(args, value)
		argIndex++
	}

	// Always update updated_at
	setClauses += fmt.Sprintf(", updated_at = $%d", argIndex)
	args = append(args, time.Now().UTC())
	argIndex++

	// Add groupID as the last argument
	args = append(args, groupID)

	query := fmt.Sprintf("UPDATE groups SET %s WHERE id = $%d", setClauses, argIndex)

	_, err := r.db.pool.Exec(ctx, query, args...)
	if err != nil {
		r.logger.Error("Failed to update group",
			zap.String("groupID", groupID),
			zap.Error(err))
		return err
	}

	return nil
}

func (r *groupRepository) DeleteGroupsByInstance(ctx context.Context, userID string, instanceID int) error {
	query := `DELETE FROM groups WHERE user_id = $1 AND instance_id = $2`

	_, err := r.db.pool.Exec(ctx, query, userID, instanceID)
	if err != nil {
		r.logger.Error("Failed to delete groups by instance",
			zap.String("userID", userID),
			zap.Int("instanceID", instanceID),
			zap.Error(err))
		return err
	}

	return nil
}

func (r *groupRepository) CountAll(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM groups WHERE is_admin = true`).Scan(&count)
	if err != nil {
		r.logger.Error("Failed to count all groups", zap.Error(err))
		return 0, err
	}
	return count, nil
}

// Helper functions

func (r *groupRepository) scanGroup(row pgx.Row) (*models.Group, error) {
	var g models.Group
	var instanceID int
	var status, inviteCode *string
	var createdAt, updatedAt time.Time

	err := row.Scan(
		&g.ID, &g.UserID, &instanceID, &g.WAGroupID, &g.Name, &g.Description, &g.PictureURL,
		&g.MembersCount, &g.MemberLimit, &status, &g.IsAdmin, &inviteCode, &g.AdminOnly,
		&g.IsFavorite, &createdAt, &updatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	g.InstanceID = fmt.Sprintf("%d", instanceID)
	if status != nil {
		g.Status = *status
	}
	g.InviteCode = inviteCode
	g.CreatedAt = createdAt.Format(time.RFC3339)
	g.UpdatedAt = updatedAt.Format(time.RFC3339)
	return &g, nil
}

func (r *groupRepository) scanGroups(rows pgx.Rows) ([]models.Group, error) {
	var groups []models.Group
	for rows.Next() {
		var g models.Group
		var instanceID int
		var status, inviteCode *string
		var createdAt, updatedAt time.Time

		err := rows.Scan(
			&g.ID, &g.UserID, &instanceID, &g.WAGroupID, &g.Name, &g.Description, &g.PictureURL,
			&g.MembersCount, &g.MemberLimit, &status, &g.IsAdmin, &inviteCode, &g.AdminOnly,
			&g.IsFavorite, &createdAt, &updatedAt,
		)
		if err != nil {
			return nil, err
		}

		g.InstanceID = fmt.Sprintf("%d", instanceID)
		if status != nil {
			g.Status = *status
		}
		g.InviteCode = inviteCode
		g.CreatedAt = createdAt.Format(time.RFC3339)
		g.UpdatedAt = updatedAt.Format(time.RFC3339)
		groups = append(groups, g)
	}

	return groups, rows.Err()
}

// GetGroupsByWAGroupIDs retrieves groups by their WhatsApp group IDs
func (r *groupRepository) GetGroupsByWAGroupIDs(ctx context.Context, userID string, waGroupIDs []string) ([]models.Group, error) {
	if len(waGroupIDs) == 0 {
		return []models.Group{}, nil
	}

	query := `
		SELECT id, user_id, instance_id, wa_group_id, name, description, picture_url,
		       members_count, member_limit, status, is_admin, invite_code, admin_only,
		       is_favorite, created_at, updated_at
		FROM groups
		WHERE user_id = $1 AND wa_group_id = ANY($2)
		ORDER BY name ASC
	`

	rows, err := r.db.Query(ctx, query, userID, waGroupIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanGroups(rows)
}

// CreateGroup creates a new group in the database
func (r *groupRepository) CreateGroup(ctx context.Context, userID string, group *models.Group) (*models.Group, error) {
	query := `
		INSERT INTO groups (user_id, instance_id, wa_group_id, name, description, picture_url,
		                    members_count, member_limit, status, is_admin, invite_code, admin_only,
		                    is_favorite, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		RETURNING id
	`

	now := time.Now().UTC()
	var instanceID int
	if group.InstanceID != "" {
		_, _ = fmt.Sscanf(group.InstanceID, "%d", &instanceID)
	}

	var id string
	err := r.db.QueryRow(ctx, query,
		userID, instanceID, group.WAGroupID, group.Name, group.Description, group.PictureURL,
		group.MembersCount, group.MemberLimit, group.Status, group.IsAdmin, group.InviteCode, group.AdminOnly,
		group.IsFavorite, now, now,
	).Scan(&id)
	if err != nil {
		r.logger.Error("Failed to create group",
			zap.String("userID", userID),
			zap.Error(err))
		return nil, err
	}

	group.ID = id
	group.UserID = userID
	group.CreatedAt = now.Format(time.RFC3339)
	group.UpdatedAt = now.Format(time.RFC3339)
	return group, nil
}

// DeleteGroup deletes a group by ID
func (r *groupRepository) DeleteGroup(ctx context.Context, groupID, userID string) error {
	query := `DELETE FROM groups WHERE id = $1 AND user_id = $2`

	result, err := r.db.pool.Exec(ctx, query, groupID, userID)
	if err != nil {
		r.logger.Error("Failed to delete group",
			zap.String("groupID", groupID),
			zap.Error(err))
		return err
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("group not found")
	}

	return nil
}

// BulkUpdateGroups updates multiple groups at once
func (r *groupRepository) BulkUpdateGroups(ctx context.Context, userID string, groupIDs []string, updates map[string]interface{}) (int64, error) {
	if len(groupIDs) == 0 || len(updates) == 0 {
		return 0, nil
	}

	// Build dynamic UPDATE query
	setClauses := ""
	args := make([]interface{}, 0, len(updates)+2)
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

	// Add userID and groupIDs
	args = append(args, userID, groupIDs)

	query := fmt.Sprintf("UPDATE groups SET %s WHERE user_id = $%d AND id = ANY($%d)", setClauses, argIndex, argIndex+1)

	result, err := r.db.pool.Exec(ctx, query, args...)
	if err != nil {
		r.logger.Error("Failed to bulk update groups",
			zap.Strings("groupIDs", groupIDs),
			zap.Error(err))
		return 0, err
	}

	return result.RowsAffected(), nil
}

// ToggleFavorite toggles the is_favorite status of a group
func (r *groupRepository) ToggleFavorite(ctx context.Context, groupID, userID string, favorite bool) error {
	query := `
		UPDATE groups
		SET is_favorite = $1, updated_at = $2
		WHERE id = $3 AND user_id = $4
	`

	result, err := r.db.pool.Exec(ctx, query, favorite, time.Now().UTC(), groupID, userID)
	if err != nil {
		r.logger.Error("Failed to toggle favorite",
			zap.String("groupID", groupID),
			zap.String("userID", userID),
			zap.Bool("favorite", favorite),
			zap.Error(err))
		return err
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("group not found or not owned by user")
	}

	return nil
}

// GetFavoriteGroups retrieves all favorite groups for a user
func (r *groupRepository) GetFavoriteGroups(ctx context.Context, userID string) ([]models.Group, error) {
	query := `
		SELECT id, user_id, instance_id, wa_group_id, name, description, picture_url,
		       members_count, member_limit, status, is_admin, invite_code, admin_only,
		       is_favorite, created_at, updated_at
		FROM groups
		WHERE user_id = $1 AND is_favorite = TRUE
		ORDER BY name ASC
	`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanGroups(rows)
}
