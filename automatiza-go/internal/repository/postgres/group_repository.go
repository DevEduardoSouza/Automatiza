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
		       is_favorite, created_at, updated_at, participating_instance_ids
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

func (r *groupRepository) GetUserGroupsPaginated(ctx context.Context, userID string, page, pageSize int, adminOnly bool, search string, instanceID int) (*repository.GroupPaginatedResult, error) {
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

	// Per-chip filter via participating_instance_ids. The GIN index on the
	// array makes the @> lookup cheap, and matching against the array (not
	// just instance_id) means shared groups show under every chip that's
	// in them — that's the whole point of moving the filter server-side.
	// instanceID = 0 means "all chips" (the explicit "Todos os números"
	// state in the UI).
	if instanceID > 0 {
		whereClause += fmt.Sprintf(" AND participating_instance_ids @> ARRAY[$%d::INTEGER]", argIndex)
		args = append(args, instanceID)
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
		       is_favorite, created_at, updated_at, participating_instance_ids
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
		       is_favorite, created_at, updated_at, participating_instance_ids
		FROM groups
		WHERE id = $1
	`

	return r.scanGroup(r.db.QueryRow(ctx, query, groupID))
}

func (r *groupRepository) GetGroupByWAGroupID(ctx context.Context, userID, waGroupID string) (*models.Group, error) {
	query := `
		SELECT id, user_id, instance_id, wa_group_id, name, description, picture_url,
		       members_count, member_limit, status, is_admin, invite_code, admin_only,
		       is_favorite, created_at, updated_at, participating_instance_ids
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
		       is_favorite, created_at, updated_at, participating_instance_ids
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
		existingIDs, err := loadParticipatingGroupIDs(ctx, tx, userID, instanceID)
		if err != nil {
			return err
		}

		// Track which IDs we're syncing
		syncedIDs := make(map[string]bool)

		now := time.Now().UTC()
		for _, g := range groups {
			_, err := tx.Exec(ctx, upsertGroupQuery,
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

		// For groups this chip used to participate in but no longer sees,
		// remove this chip from the participation set. Helper handles
		// owner reassignment and deletion-when-empty.
		for waGroupID := range existingIDs {
			if !syncedIDs[waGroupID] {
				if err := removeChipFromGroup(ctx, tx, userID, instanceID, waGroupID); err != nil {
					r.logger.Error("Failed to demote chip from group",
						zap.String("waGroupID", waGroupID),
						zap.Error(err))
					return err
				}
			}
		}

		return nil
	})
}

// upsertGroupQuery insert-or-update for a single group seen by a chip.
//
// instance_id stays as the first chip that discovered the group (the
// "owner") to avoid ping-pong on shared groups; participating_instance_ids
// accumulates every chip whose sync has seen the group, deduped, so the
// frontend can filter by chip and render multi-chip badges.
//
// admin_instance_ids tracks per-chip admin the same way: this chip's sync
// ADDS itself when it is admin ($9=true) and REMOVES itself when it is not,
// so the array is the union of admin chips. is_admin is derived from it
// ("any chip admin") instead of being blindly overwritten by the last chip
// to sync — the old EXCLUDED.is_admin overwrite made a non-admin chip hide a
// group where another chip was still admin. Deriving in-SQL keeps the
// existing `WHERE is_admin = true` list filter correct with no read changes.
const upsertGroupQuery = `
	INSERT INTO groups (user_id, instance_id, wa_group_id, name, description, picture_url,
	                    members_count, member_limit, is_admin, admin_only,
	                    participating_instance_ids, admin_instance_ids, created_at, updated_at)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, ARRAY[$2::INTEGER],
	        CASE WHEN $9 THEN ARRAY[$2::INTEGER] ELSE ARRAY[]::INTEGER[] END, $11, $12)
	ON CONFLICT (user_id, wa_group_id) DO UPDATE SET
		name = EXCLUDED.name,
		description = EXCLUDED.description,
		picture_url = EXCLUDED.picture_url,
		members_count = EXCLUDED.members_count,
		member_limit = EXCLUDED.member_limit,
		admin_only = EXCLUDED.admin_only,
		participating_instance_ids = (
			SELECT ARRAY(
				SELECT DISTINCT unnest(groups.participating_instance_ids || ARRAY[EXCLUDED.instance_id])
			)
		),
		admin_instance_ids = (
			SELECT ARRAY(
				SELECT DISTINCT unnest(
					CASE WHEN EXCLUDED.is_admin
						THEN groups.admin_instance_ids || ARRAY[EXCLUDED.instance_id]
						ELSE array_remove(groups.admin_instance_ids, EXCLUDED.instance_id)
					END
				)
			)
		),
		is_admin = (
			CASE WHEN EXCLUDED.is_admin
				THEN TRUE
				ELSE array_length(array_remove(groups.admin_instance_ids, EXCLUDED.instance_id), 1) IS NOT NULL
			END
		),
		updated_at = EXCLUDED.updated_at
`

// loadParticipatingGroupIDs returns the wa_group_ids of every group the
// given chip currently participates in (owner OR member of the array).
// Used by sync to detect groups the chip no longer sees so it can be
// demoted from them.
func loadParticipatingGroupIDs(ctx context.Context, tx pgx.Tx, userID string, instanceID int) (map[string]bool, error) {
	rows, err := tx.Query(ctx, `
		SELECT wa_group_id FROM groups
		WHERE user_id = $1
		  AND ($2::INTEGER = instance_id OR $2::INTEGER = ANY(participating_instance_ids))
	`, userID, instanceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]bool)
	for rows.Next() {
		var waGroupID string
		if err := rows.Scan(&waGroupID); err != nil {
			return nil, err
		}
		out[waGroupID] = true
	}
	return out, rows.Err()
}

// removeChipFromGroup is called when a chip's sync no longer reports a
// group it used to see. Three cases, handled atomically:
//
//   - chip is the only participant → DELETE the row entirely.
//   - chip is a participant but not the owner → just remove it from the array.
//   - chip is the owner and others remain → reassign ownership to the
//     first remaining participant (deterministic so re-syncs are stable),
//     then remove this chip from the array.
//
// Without this, two chips sharing a group would lose data when one of them
// stopped seeing it: the older code blindly DELETEd, and the rest of the
// participants would silently lose access until they re-synced.
func removeChipFromGroup(ctx context.Context, tx pgx.Tx, userID string, instanceID int, waGroupID string) error {
	_, err := tx.Exec(ctx, `
		WITH target AS (
			SELECT id, instance_id,
			       array_remove(participating_instance_ids, $2::INTEGER) AS remaining,
			       array_remove(admin_instance_ids, $2::INTEGER) AS admin_remaining
			FROM groups
			WHERE user_id = $1 AND wa_group_id = $3
			FOR UPDATE
		),
		deleted AS (
			DELETE FROM groups g
			USING target t
			WHERE g.id = t.id AND COALESCE(array_length(t.remaining, 1), 0) = 0
			RETURNING g.id
		)
		UPDATE groups g
		SET participating_instance_ids = t.remaining,
		    admin_instance_ids = t.admin_remaining,
		    is_admin = COALESCE(array_length(t.admin_remaining, 1), 0) > 0,
		    instance_id = CASE
		        WHEN g.instance_id = $2 THEN t.remaining[1]
		        ELSE g.instance_id
		    END,
		    updated_at = NOW()
		FROM target t
		WHERE g.id = t.id
		  AND COALESCE(array_length(t.remaining, 1), 0) > 0
	`, userID, instanceID, waGroupID)
	return err
}

func (r *groupRepository) SyncGroupsWithStats(ctx context.Context, userID string, instanceID int, groups []models.Group, options repository.SyncOptions) (*repository.SyncResult, error) {
	result := &repository.SyncResult{}
	if len(groups) == 0 {
		return result, nil
	}

	err := r.db.WithTx(ctx, func(tx pgx.Tx) error {
		existingIDs, err := loadParticipatingGroupIDs(ctx, tx, userID, instanceID)
		if err != nil {
			return err
		}

		// Track which IDs we're syncing
		syncedIDs := make(map[string]bool)

		now := time.Now().UTC()
		for _, g := range groups {
			_, err := tx.Exec(ctx, upsertGroupQuery,
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

			// Count created vs updated. existingIDs covers participation,
			// not just ownership, so a chip seeing a shared group for the
			// second time correctly counts as Updated.
			if existingIDs[g.WAGroupID] {
				result.Updated++
			} else {
				result.Created++
			}
		}

		// Demote chip from groups it no longer sees. Helper deletes the
		// row only when no other chip remained in the participation set,
		// otherwise reassigns ownership and trims the array.
		for waGroupID := range existingIDs {
			if !syncedIDs[waGroupID] {
				if err := removeChipFromGroup(ctx, tx, userID, instanceID, waGroupID); err != nil {
					r.logger.Error("Failed to demote chip from group",
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

func (r *groupRepository) CountGroupsByInstance(ctx context.Context, userID string, instanceID int) (int, error) {
	var count int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM groups WHERE user_id = $1 AND instance_id = $2`,
		userID, instanceID,
	).Scan(&count)
	if err != nil {
		r.logger.Error("Failed to count groups by instance",
			zap.String("userID", userID),
			zap.Int("instanceID", instanceID),
			zap.Error(err))
		return 0, err
	}
	return count, nil
}

func (r *groupRepository) GetInstancesForGroups(ctx context.Context, userID string, waGroupIDs []string) (map[string]*models.Instance, error) {
	result := make(map[string]*models.Instance)
	if len(waGroupIDs) == 0 {
		return result, nil
	}

	query := `
		SELECT g.wa_group_id,
		       i.id, i.user_id, i.instance_id, i.friendly_name, i.status, i.qr_code, i.is_default,
		       i.phone_number, i.created_at, i.updated_at
		FROM groups g
		JOIN instances i ON i.id = g.instance_id
		WHERE g.user_id = $1 AND g.wa_group_id = ANY($2::text[])
	`

	rows, err := r.db.Query(ctx, query, userID, waGroupIDs)
	if err != nil {
		r.logger.Error("Failed to query instances for groups",
			zap.String("userID", userID),
			zap.Int("groupCount", len(waGroupIDs)),
			zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var waGroupID string
		var inst models.Instance
		var createdAt, updatedAt time.Time
		if err := rows.Scan(
			&waGroupID,
			&inst.ID, &inst.UserID, &inst.InstanceID, &inst.FriendlyName, &inst.Status, &inst.QRCode, &inst.IsDefault,
			&inst.PhoneNumber, &createdAt, &updatedAt,
		); err != nil {
			return nil, err
		}
		inst.CreatedAt = createdAt.Format(time.RFC3339)
		inst.UpdatedAt = updatedAt.Format(time.RFC3339)
		copied := inst
		result[waGroupID] = &copied
	}

	return result, rows.Err()
}

// Helper functions

func (r *groupRepository) scanGroup(row pgx.Row) (*models.Group, error) {
	var g models.Group
	var instanceID int
	var status, inviteCode *string
	var createdAt, updatedAt time.Time
	var participatingIDs []int32

	err := row.Scan(
		&g.ID, &g.UserID, &instanceID, &g.WAGroupID, &g.Name, &g.Description, &g.PictureURL,
		&g.MembersCount, &g.MemberLimit, &status, &g.IsAdmin, &inviteCode, &g.AdminOnly,
		&g.IsFavorite, &createdAt, &updatedAt, &participatingIDs,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	g.InstanceID = fmt.Sprintf("%d", instanceID)
	g.ParticipatingInstanceIDs = stringifyInstanceIDs(participatingIDs)
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
		var participatingIDs []int32

		err := rows.Scan(
			&g.ID, &g.UserID, &instanceID, &g.WAGroupID, &g.Name, &g.Description, &g.PictureURL,
			&g.MembersCount, &g.MemberLimit, &status, &g.IsAdmin, &inviteCode, &g.AdminOnly,
			&g.IsFavorite, &createdAt, &updatedAt, &participatingIDs,
		)
		if err != nil {
			return nil, err
		}

		g.InstanceID = fmt.Sprintf("%d", instanceID)
		g.ParticipatingInstanceIDs = stringifyInstanceIDs(participatingIDs)
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

// stringifyInstanceIDs converts a Postgres INTEGER[] of instance IDs into
// the string form used by the Group model (matches InstanceID's format).
// Returns an empty slice for nil/empty input — never nil — so JSON
// serialization stays as "[]" instead of "null".
func stringifyInstanceIDs(ids []int32) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, fmt.Sprintf("%d", id))
	}
	return out
}

// GetGroupsByWAGroupIDs retrieves groups by their WhatsApp group IDs
func (r *groupRepository) GetGroupsByWAGroupIDs(ctx context.Context, userID string, waGroupIDs []string) ([]models.Group, error) {
	if len(waGroupIDs) == 0 {
		return []models.Group{}, nil
	}

	query := `
		SELECT id, user_id, instance_id, wa_group_id, name, description, picture_url,
		       members_count, member_limit, status, is_admin, invite_code, admin_only,
		       is_favorite, created_at, updated_at, participating_instance_ids
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

// GetAdminInstanceIDsForGroups returns a map keyed by wa_group_id of the chip
// ids (stringified, matching Instance.ID's wire format) that ADMINISTER each
// group — i.e. groups.admin_instance_ids. Used by the bulk picture fan-out to
// route each group through a chip that can actually change its photo; a chip
// that merely participates but isn't admin gets a "not-authorized" from
// WhatsApp. Missing/empty groups are simply absent from the map.
func (r *groupRepository) GetAdminInstanceIDsForGroups(ctx context.Context, userID string, waGroupIDs []string) (map[string][]string, error) {
	result := make(map[string][]string)
	if len(waGroupIDs) == 0 {
		return result, nil
	}

	query := `
		SELECT wa_group_id, admin_instance_ids
		FROM groups
		WHERE user_id = $1 AND wa_group_id = ANY($2)
	`

	rows, err := r.db.Query(ctx, query, userID, waGroupIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var waGroupID string
		var adminIDs []int32
		if err := rows.Scan(&waGroupID, &adminIDs); err != nil {
			return nil, err
		}
		result[waGroupID] = stringifyInstanceIDs(adminIDs)
	}

	return result, rows.Err()
}

// CreateGroup creates a new group in the database
func (r *groupRepository) CreateGroup(ctx context.Context, userID string, group *models.Group) (*models.Group, error) {
	query := `
		INSERT INTO groups (user_id, instance_id, wa_group_id, name, description, picture_url,
		                    members_count, member_limit, status, is_admin, invite_code, admin_only,
		                    is_favorite, participating_instance_ids, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, ARRAY[$2::INTEGER], $14, $15)
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
		       is_favorite, created_at, updated_at, participating_instance_ids
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
