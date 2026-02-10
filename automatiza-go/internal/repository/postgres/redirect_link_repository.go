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

type redirectLinkRepository struct {
	db     *DB
	logger *zap.Logger
}

// NewRedirectLinkRepository creates a new PostgreSQL redirect link repository
func NewRedirectLinkRepository(db *DB, logger *zap.Logger) repository.RedirectLinkRepository {
	return &redirectLinkRepository{
		db:     db,
		logger: logger,
	}
}

// GetByID retrieves a redirect link by its ID
func (r *redirectLinkRepository) GetByID(ctx context.Context, linkID string) (*models.RedirectLink, error) {
	query := `
		SELECT id, user_id, slug, name, strategy, target_groups, click_count, group_clicks, is_active, created_at, updated_at
		FROM saved_redirect_links
		WHERE id = $1
	`

	var link models.RedirectLink
	var targetGroupsJSON, groupClicksJSON []byte

	err := r.db.QueryRow(ctx, query, linkID).Scan(
		&link.ID,
		&link.UserID,
		&link.Slug,
		&link.Name,
		&link.Strategy,
		&targetGroupsJSON,
		&link.ClickCount,
		&groupClicksJSON,
		&link.IsActive,
		&link.CreatedAt,
		&link.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		r.logger.Error("Failed to get redirect link by ID",
			zap.String("linkID", linkID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get redirect link: %w", err)
	}

	// Unmarshal JSONB fields
	if err := link.UnmarshalTargetGroups(targetGroupsJSON); err != nil {
		r.logger.Error("Failed to unmarshal target_groups",
			zap.String("linkID", linkID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to unmarshal target_groups: %w", err)
	}

	if err := link.UnmarshalGroupClicks(groupClicksJSON); err != nil {
		r.logger.Error("Failed to unmarshal group_clicks",
			zap.String("linkID", linkID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to unmarshal group_clicks: %w", err)
	}

	return &link, nil
}

// GetBySlug retrieves a redirect link by its slug
func (r *redirectLinkRepository) GetBySlug(ctx context.Context, slug string) (*models.RedirectLink, error) {
	query := `
		SELECT id, user_id, slug, name, strategy, target_groups, click_count, group_clicks, is_active, created_at, updated_at
		FROM saved_redirect_links
		WHERE slug = $1
	`

	var link models.RedirectLink
	var targetGroupsJSON, groupClicksJSON []byte

	err := r.db.QueryRow(ctx, query, slug).Scan(
		&link.ID,
		&link.UserID,
		&link.Slug,
		&link.Name,
		&link.Strategy,
		&targetGroupsJSON,
		&link.ClickCount,
		&groupClicksJSON,
		&link.IsActive,
		&link.CreatedAt,
		&link.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		r.logger.Error("Failed to get redirect link by slug",
			zap.String("slug", slug),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get redirect link by slug: %w", err)
	}

	// Unmarshal JSONB fields
	if err := link.UnmarshalTargetGroups(targetGroupsJSON); err != nil {
		r.logger.Error("Failed to unmarshal target_groups",
			zap.String("slug", slug),
			zap.Error(err))
		return nil, fmt.Errorf("failed to unmarshal target_groups: %w", err)
	}

	if err := link.UnmarshalGroupClicks(groupClicksJSON); err != nil {
		r.logger.Error("Failed to unmarshal group_clicks",
			zap.String("slug", slug),
			zap.Error(err))
		return nil, fmt.Errorf("failed to unmarshal group_clicks: %w", err)
	}

	return &link, nil
}

// GetByUserID retrieves all redirect links for a user
func (r *redirectLinkRepository) GetByUserID(ctx context.Context, userID string) ([]models.RedirectLink, error) {
	query := `
		SELECT id, user_id, slug, name, strategy, target_groups, click_count, group_clicks, is_active, created_at, updated_at
		FROM saved_redirect_links
		WHERE user_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		r.logger.Error("Failed to get redirect links by user ID",
			zap.String("userID", userID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get redirect links: %w", err)
	}
	defer rows.Close()

	var links []models.RedirectLink
	for rows.Next() {
		var link models.RedirectLink
		var targetGroupsJSON, groupClicksJSON []byte

		err := rows.Scan(
			&link.ID,
			&link.UserID,
			&link.Slug,
			&link.Name,
			&link.Strategy,
			&targetGroupsJSON,
			&link.ClickCount,
			&groupClicksJSON,
			&link.IsActive,
			&link.CreatedAt,
			&link.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan redirect link: %w", err)
		}

		// Unmarshal JSONB fields
		if err := link.UnmarshalTargetGroups(targetGroupsJSON); err != nil {
			return nil, fmt.Errorf("failed to unmarshal target_groups: %w", err)
		}
		if err := link.UnmarshalGroupClicks(groupClicksJSON); err != nil {
			return nil, fmt.Errorf("failed to unmarshal group_clicks: %w", err)
		}

		links = append(links, link)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating redirect links: %w", err)
	}

	return links, nil
}

// CheckSlugExists checks if a slug is already in use
func (r *redirectLinkRepository) CheckSlugExists(ctx context.Context, slug string, excludeID string) (bool, error) {
	var query string
	var args []interface{}

	if excludeID != "" {
		query = `SELECT EXISTS(SELECT 1 FROM saved_redirect_links WHERE slug = $1 AND id != $2)`
		args = []interface{}{slug, excludeID}
	} else {
		query = `SELECT EXISTS(SELECT 1 FROM saved_redirect_links WHERE slug = $1)`
		args = []interface{}{slug}
	}

	var exists bool
	err := r.db.QueryRow(ctx, query, args...).Scan(&exists)
	if err != nil {
		r.logger.Error("Failed to check slug existence",
			zap.String("slug", slug),
			zap.Error(err))
		return false, fmt.Errorf("failed to check slug existence: %w", err)
	}

	return exists, nil
}

// Create creates a new redirect link
func (r *redirectLinkRepository) Create(ctx context.Context, link *models.RedirectLink) (*models.RedirectLink, error) {
	query := `
		INSERT INTO saved_redirect_links (user_id, slug, name, strategy, target_groups, click_count, group_clicks, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at, updated_at
	`

	now := time.Now().UTC()

	targetGroupsJSON, err := link.MarshalTargetGroups()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal target_groups: %w", err)
	}

	groupClicksJSON, err := link.MarshalGroupClicks()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal group_clicks: %w", err)
	}

	err = r.db.QueryRow(ctx, query,
		link.UserID,
		link.Slug,
		link.Name,
		link.Strategy,
		targetGroupsJSON,
		0, // click_count starts at 0
		groupClicksJSON,
		link.IsActive,
		now,
		now,
	).Scan(&link.ID, &link.CreatedAt, &link.UpdatedAt)

	if err != nil {
		r.logger.Error("Failed to create redirect link",
			zap.String("userID", link.UserID),
			zap.String("slug", link.Slug),
			zap.Error(err))
		return nil, fmt.Errorf("failed to create redirect link: %w", err)
	}

	link.ClickCount = 0
	return link, nil
}

// Update updates an existing redirect link
func (r *redirectLinkRepository) Update(ctx context.Context, linkID string, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return nil
	}

	// Build dynamic update query
	setClauses := []string{}
	args := []interface{}{}
	argIndex := 1

	for key, value := range updates {
		switch key {
		case "slug", "name", "strategy", "is_active":
			setClauses = append(setClauses, fmt.Sprintf("%s = $%d", key, argIndex))
			args = append(args, value)
			argIndex++
		case "target_groups":
			// Marshal to JSON if it's a slice
			if groups, ok := value.([]models.GroupPriority); ok {
				jsonBytes, err := json.Marshal(groups)
				if err != nil {
					return fmt.Errorf("failed to marshal target_groups: %w", err)
				}
				setClauses = append(setClauses, fmt.Sprintf("%s = $%d", key, argIndex))
				args = append(args, jsonBytes)
				argIndex++
			}
		}
	}

	if len(setClauses) == 0 {
		return nil
	}

	// Add updated_at
	setClauses = append(setClauses, fmt.Sprintf("updated_at = $%d", argIndex))
	args = append(args, time.Now().UTC())
	argIndex++

	// Add linkID as last argument
	args = append(args, linkID)

	query := fmt.Sprintf(
		"UPDATE saved_redirect_links SET %s WHERE id = $%d",
		joinClauses(setClauses),
		argIndex,
	)

	result, err := r.db.pool.Exec(ctx, query, args...)
	if err != nil {
		r.logger.Error("Failed to update redirect link",
			zap.String("linkID", linkID),
			zap.Error(err))
		return fmt.Errorf("failed to update redirect link: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("redirect link not found")
	}

	return nil
}

// Delete deletes a redirect link
func (r *redirectLinkRepository) Delete(ctx context.Context, linkID, userID string) error {
	query := `DELETE FROM saved_redirect_links WHERE id = $1 AND user_id = $2`

	result, err := r.db.pool.Exec(ctx, query, linkID, userID)
	if err != nil {
		r.logger.Error("Failed to delete redirect link",
			zap.String("linkID", linkID),
			zap.String("userID", userID),
			zap.Error(err))
		return fmt.Errorf("failed to delete redirect link: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("redirect link not found or unauthorized")
	}

	return nil
}

// IncrementClicks increments the click count and group-specific click count
func (r *redirectLinkRepository) IncrementClicks(ctx context.Context, linkID, groupID string) error {
	// Use a transaction to ensure atomicity
	return r.db.WithTx(ctx, func(tx pgx.Tx) error {
		// Increment total click_count
		updateClickCountQuery := `
			UPDATE saved_redirect_links
			SET click_count = click_count + 1,
				updated_at = $2
			WHERE id = $1
		`

		_, err := tx.Exec(ctx, updateClickCountQuery, linkID, time.Now().UTC())
		if err != nil {
			r.logger.Error("Failed to increment click_count",
				zap.String("linkID", linkID),
				zap.Error(err))
			return fmt.Errorf("failed to increment click_count: %w", err)
		}

		// Increment group-specific click count in JSONB
		// Using JSONB || operator to merge and COALESCE for default value
		updateGroupClicksQuery := `
			UPDATE saved_redirect_links
			SET group_clicks = jsonb_set(
				COALESCE(group_clicks, '{}'::jsonb),
				ARRAY[$2],
				to_jsonb(COALESCE((group_clicks->>$2)::int, 0) + 1)
			)
			WHERE id = $1
		`

		_, err = tx.Exec(ctx, updateGroupClicksQuery, linkID, groupID)
		if err != nil {
			r.logger.Error("Failed to increment group_clicks",
				zap.String("linkID", linkID),
				zap.String("groupID", groupID),
				zap.Error(err))
			return fmt.Errorf("failed to increment group_clicks: %w", err)
		}

		return nil
	})
}

// Helper function to join SQL clauses
func joinClauses(clauses []string) string {
	result := ""
	for i, clause := range clauses {
		if i > 0 {
			result += ", "
		}
		result += clause
	}
	return result
}
