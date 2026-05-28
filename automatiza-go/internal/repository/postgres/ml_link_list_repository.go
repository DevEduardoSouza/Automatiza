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

type mlLinkListRepository struct {
	db     *DB
	logger *zap.Logger
}

// NewMLLinkListRepository creates a new PostgreSQL ML link list repository
func NewMLLinkListRepository(db *DB, logger *zap.Logger) repository.MLLinkListRepository {
	return &mlLinkListRepository{
		db:     db,
		logger: logger,
	}
}

// ============================================================================
// List operations
// ============================================================================

// GetListsByUserID retrieves all ML link lists for a user
func (r *mlLinkListRepository) GetListsByUserID(ctx context.Context, userID string) ([]models.MlLinkList, error) {
	query := `
		SELECT id, user_id, name, description, is_active, created_at, updated_at
		FROM ml_link_lists
		WHERE user_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		r.logger.Error("Failed to get ML link lists by user ID",
			zap.String("userID", userID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get ML link lists: %w", err)
	}
	defer rows.Close()

	var lists []models.MlLinkList
	for rows.Next() {
		var list models.MlLinkList
		var createdAt, updatedAt time.Time

		err := rows.Scan(
			&list.ID,
			&list.UserID,
			&list.Name,
			&list.Description,
			&list.IsActive,
			&createdAt,
			&updatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan ML link list: %w", err)
		}

		list.CreatedAt = createdAt.Format(time.RFC3339)
		list.UpdatedAt = updatedAt.Format(time.RFC3339)
		lists = append(lists, list)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating ML link lists: %w", err)
	}

	return lists, nil
}

// GetListByID retrieves an ML link list by its ID
func (r *mlLinkListRepository) GetListByID(ctx context.Context, listID string) (*models.MlLinkList, error) {
	query := `
		SELECT id, user_id, name, description, is_active, created_at, updated_at
		FROM ml_link_lists
		WHERE id = $1
	`

	var list models.MlLinkList
	var createdAt, updatedAt time.Time

	err := r.db.QueryRow(ctx, query, listID).Scan(
		&list.ID,
		&list.UserID,
		&list.Name,
		&list.Description,
		&list.IsActive,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		r.logger.Error("Failed to get ML link list by ID",
			zap.String("listID", listID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get ML link list: %w", err)
	}

	list.CreatedAt = createdAt.Format(time.RFC3339)
	list.UpdatedAt = updatedAt.Format(time.RFC3339)

	return &list, nil
}

// GetListByIDAndUser retrieves an ML link list by ID ensuring user ownership
func (r *mlLinkListRepository) GetListByIDAndUser(ctx context.Context, listID, userID string) (*models.MlLinkList, error) {
	query := `
		SELECT id, user_id, name, description, is_active, created_at, updated_at
		FROM ml_link_lists
		WHERE id = $1 AND user_id = $2
	`

	var list models.MlLinkList
	var createdAt, updatedAt time.Time

	err := r.db.QueryRow(ctx, query, listID, userID).Scan(
		&list.ID,
		&list.UserID,
		&list.Name,
		&list.Description,
		&list.IsActive,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		r.logger.Error("Failed to get ML link list by ID and user",
			zap.String("listID", listID),
			zap.String("userID", userID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get ML link list: %w", err)
	}

	list.CreatedAt = createdAt.Format(time.RFC3339)
	list.UpdatedAt = updatedAt.Format(time.RFC3339)

	return &list, nil
}

// CreateList creates a new ML link list
func (r *mlLinkListRepository) CreateList(ctx context.Context, userID string, list *models.MlLinkList) (*models.MlLinkList, error) {
	query := `
		INSERT INTO ml_link_lists (user_id, name, description, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at
	`

	now := time.Now().UTC()
	var createdAt, updatedAt time.Time

	err := r.db.QueryRow(ctx, query,
		userID,
		list.Name,
		list.Description,
		list.IsActive,
		now,
		now,
	).Scan(&list.ID, &createdAt, &updatedAt)

	if err != nil {
		r.logger.Error("Failed to create ML link list",
			zap.String("userID", userID),
			zap.String("name", list.Name),
			zap.Error(err))
		return nil, fmt.Errorf("failed to create ML link list: %w", err)
	}

	list.UserID = userID
	list.CreatedAt = createdAt.Format(time.RFC3339)
	list.UpdatedAt = updatedAt.Format(time.RFC3339)

	return list, nil
}

// UpdateList updates an existing ML link list
func (r *mlLinkListRepository) UpdateList(ctx context.Context, listID string, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return nil
	}

	// Build dynamic update query
	setClauses := []string{}
	args := []interface{}{}
	argIndex := 1

	for key, value := range updates {
		switch key {
		case "name", "description", "is_active":
			setClauses = append(setClauses, fmt.Sprintf("%s = $%d", key, argIndex))
			args = append(args, value)
			argIndex++
		}
	}

	if len(setClauses) == 0 {
		return nil
	}

	// Add updated_at
	setClauses = append(setClauses, fmt.Sprintf("updated_at = $%d", argIndex))
	args = append(args, time.Now().UTC())
	argIndex++

	// Add listID as last argument
	args = append(args, listID)

	query := fmt.Sprintf(
		"UPDATE ml_link_lists SET %s WHERE id = $%d",
		joinClauses(setClauses),
		argIndex,
	)

	result, err := r.db.pool.Exec(ctx, query, args...)
	if err != nil {
		r.logger.Error("Failed to update ML link list",
			zap.String("listID", listID),
			zap.Error(err))
		return fmt.Errorf("failed to update ML link list: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("ML link list not found")
	}

	return nil
}

// DeleteList deletes an ML link list and all its items (cascading)
func (r *mlLinkListRepository) DeleteList(ctx context.Context, listID, userID string) error {
	query := `DELETE FROM ml_link_lists WHERE id = $1 AND user_id = $2`

	result, err := r.db.pool.Exec(ctx, query, listID, userID)
	if err != nil {
		r.logger.Error("Failed to delete ML link list",
			zap.String("listID", listID),
			zap.String("userID", userID),
			zap.Error(err))
		return fmt.Errorf("failed to delete ML link list: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("ML link list not found or unauthorized")
	}

	return nil
}

// ============================================================================
// Item operations
// ============================================================================

// GetItemsByListID retrieves all items in an ML link list
func (r *mlLinkListRepository) GetItemsByListID(ctx context.Context, listID string) ([]models.MlLinkListItem, error) {
	query := `
		SELECT id, list_id, origin_url, order_index, is_active, consumed_at,
		       consumed_by_automation_id, consumed_by_group_id, product_title,
		       product_price, product_image_url, metadata, created_at, updated_at
		FROM ml_link_list_items
		WHERE list_id = $1
		ORDER BY order_index ASC, created_at ASC
	`

	rows, err := r.db.Query(ctx, query, listID)
	if err != nil {
		r.logger.Error("Failed to get ML link list items",
			zap.String("listID", listID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get ML link list items: %w", err)
	}
	defer rows.Close()

	var items []models.MlLinkListItem
	for rows.Next() {
		var item models.MlLinkListItem
		var createdAt, updatedAt time.Time
		var consumedAt *time.Time

		err := rows.Scan(
			&item.ID,
			&item.ListID,
			&item.OriginURL,
			&item.OrderIndex,
			&item.IsActive,
			&consumedAt,
			&item.ConsumedByAutomationID,
			&item.ConsumedByGroupID,
			&item.ProductTitle,
			&item.ProductPrice,
			&item.ProductImageURL,
			&item.Metadata,
			&createdAt,
			&updatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan ML link list item: %w", err)
		}

		item.CreatedAt = createdAt.Format(time.RFC3339)
		item.UpdatedAt = updatedAt.Format(time.RFC3339)
		if consumedAt != nil {
			formatted := consumedAt.Format(time.RFC3339)
			item.ConsumedAt = &formatted
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating ML link list items: %w", err)
	}

	return items, nil
}

// GetItemsByListIDPaginated retrieves items with cursor-based pagination.
// The cursor is the item ID of the last item from the previous page.
// Returns items, next cursor, total/available/sent counts, and error.
func (r *mlLinkListRepository) GetItemsByListIDPaginated(ctx context.Context, listID string, pageSize int, cursor string) (items []models.MlLinkListItem, nextCursor string, totalCount, availableCount, sentCount int, err error) {
	// Default page size
	if pageSize <= 0 {
		pageSize = 20
	}
	// Max page size
	if pageSize > 100 {
		pageSize = 100
	}

	// Counts across the entire list, not just the current page. Single
	// scan via FILTER aggregates — counters mostrados na UI ficam fiéis
	// mesmo quando a lista é maior que o pageSize (caso do bug pós-#142
	// onde "Disponíveis" mostrava o count do batch carregado).
	countQuery := `
		SELECT
			COUNT(*) AS total,
			COUNT(*) FILTER (WHERE consumed_at IS NULL AND is_active = true) AS available,
			COUNT(*) FILTER (WHERE consumed_at IS NOT NULL) AS sent
		FROM ml_link_list_items
		WHERE list_id = $1
	`
	if err := r.db.QueryRow(ctx, countQuery, listID).Scan(&totalCount, &availableCount, &sentCount); err != nil {
		r.logger.Error("Failed to count ML link list items",
			zap.String("listID", listID),
			zap.Error(err))
		return nil, "", 0, 0, 0, fmt.Errorf("failed to count ML link list items: %w", err)
	}

	// Build query based on cursor
	var query string
	var args []interface{}

	// Fetch one extra item to determine if there's a next page
	limit := pageSize + 1

	if cursor == "" {
		// First page
		query = `
			SELECT id, list_id, origin_url, order_index, is_active, consumed_at,
			       consumed_by_automation_id, consumed_by_group_id, product_title,
			       product_price, product_image_url, metadata, created_at, updated_at
			FROM ml_link_list_items
			WHERE list_id = $1
			ORDER BY order_index ASC, id ASC
			LIMIT $2
		`
		args = []interface{}{listID, limit}
	} else {
		// Subsequent page - use cursor to find position
		// First get the order_index of the cursor item
		var cursorOrderIndex int
		orderQuery := `SELECT order_index FROM ml_link_list_items WHERE id = $1`
		if err := r.db.QueryRow(ctx, orderQuery, cursor).Scan(&cursorOrderIndex); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				// Invalid cursor, start from beginning
				return r.GetItemsByListIDPaginated(ctx, listID, pageSize, "")
			}
			r.logger.Error("Failed to get cursor order index",
				zap.String("cursor", cursor),
				zap.Error(err))
			return nil, "", 0, 0, 0, fmt.Errorf("failed to get cursor position: %w", err)
		}

		query = `
			SELECT id, list_id, origin_url, order_index, is_active, consumed_at,
			       consumed_by_automation_id, consumed_by_group_id, product_title,
			       product_price, product_image_url, metadata, created_at, updated_at
			FROM ml_link_list_items
			WHERE list_id = $1
			  AND (order_index > $2 OR (order_index = $2 AND id > $3))
			ORDER BY order_index ASC, id ASC
			LIMIT $4
		`
		args = []interface{}{listID, cursorOrderIndex, cursor, limit}
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		r.logger.Error("Failed to get ML link list items paginated",
			zap.String("listID", listID),
			zap.Error(err))
		return nil, "", 0, 0, 0, fmt.Errorf("failed to get ML link list items: %w", err)
	}
	defer rows.Close()

	items = make([]models.MlLinkListItem, 0, pageSize)
	for rows.Next() {
		var item models.MlLinkListItem
		var createdAt, updatedAt time.Time
		var consumedAt *time.Time

		err := rows.Scan(
			&item.ID,
			&item.ListID,
			&item.OriginURL,
			&item.OrderIndex,
			&item.IsActive,
			&consumedAt,
			&item.ConsumedByAutomationID,
			&item.ConsumedByGroupID,
			&item.ProductTitle,
			&item.ProductPrice,
			&item.ProductImageURL,
			&item.Metadata,
			&createdAt,
			&updatedAt,
		)
		if err != nil {
			return nil, "", 0, 0, 0, fmt.Errorf("failed to scan ML link list item: %w", err)
		}

		item.CreatedAt = createdAt.Format(time.RFC3339)
		item.UpdatedAt = updatedAt.Format(time.RFC3339)
		if consumedAt != nil {
			formatted := consumedAt.Format(time.RFC3339)
			item.ConsumedAt = &formatted
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, "", 0, 0, 0, fmt.Errorf("error iterating ML link list items: %w", err)
	}

	// Check if there's a next page
	if len(items) > pageSize {
		// We have more items, set next cursor and trim the extra item
		nextCursor = items[pageSize-1].ID
		items = items[:pageSize]
	}

	return items, nextCursor, totalCount, availableCount, sentCount, nil
}

// GetItemByID retrieves an ML link list item by its ID
func (r *mlLinkListRepository) GetItemByID(ctx context.Context, itemID string) (*models.MlLinkListItem, error) {
	query := `
		SELECT id, list_id, origin_url, order_index, is_active, consumed_at,
		       consumed_by_automation_id, consumed_by_group_id, product_title,
		       product_price, product_image_url, metadata, created_at, updated_at
		FROM ml_link_list_items
		WHERE id = $1
	`

	var item models.MlLinkListItem
	var createdAt, updatedAt time.Time
	var consumedAt *time.Time

	err := r.db.QueryRow(ctx, query, itemID).Scan(
		&item.ID,
		&item.ListID,
		&item.OriginURL,
		&item.OrderIndex,
		&item.IsActive,
		&consumedAt,
		&item.ConsumedByAutomationID,
		&item.ConsumedByGroupID,
		&item.ProductTitle,
		&item.ProductPrice,
		&item.ProductImageURL,
		&item.Metadata,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		r.logger.Error("Failed to get ML link list item by ID",
			zap.String("itemID", itemID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get ML link list item: %w", err)
	}

	item.CreatedAt = createdAt.Format(time.RFC3339)
	item.UpdatedAt = updatedAt.Format(time.RFC3339)
	if consumedAt != nil {
		formatted := consumedAt.Format(time.RFC3339)
		item.ConsumedAt = &formatted
	}

	return &item, nil
}

// CreateItem creates a new ML link list item
func (r *mlLinkListRepository) CreateItem(ctx context.Context, item *models.MlLinkListItem) (*models.MlLinkListItem, error) {
	query := `
		INSERT INTO ml_link_list_items (list_id, origin_url, order_index, is_active,
		                                product_title, product_price, product_image_url, metadata,
		                                created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at, updated_at
	`

	now := time.Now().UTC()
	var createdAt, updatedAt time.Time

	// Default metadata to empty JSON object if nil
	metadata := item.Metadata
	if metadata == nil {
		metadata = []byte("{}")
	}

	err := r.db.QueryRow(ctx, query,
		item.ListID,
		item.OriginURL,
		item.OrderIndex,
		item.IsActive,
		item.ProductTitle,
		item.ProductPrice,
		item.ProductImageURL,
		metadata,
		now,
		now,
	).Scan(&item.ID, &createdAt, &updatedAt)

	if err != nil {
		r.logger.Error("Failed to create ML link list item",
			zap.String("listID", item.ListID),
			zap.String("originURL", item.OriginURL),
			zap.Error(err))
		return nil, fmt.Errorf("failed to create ML link list item: %w", err)
	}

	item.CreatedAt = createdAt.Format(time.RFC3339)
	item.UpdatedAt = updatedAt.Format(time.RFC3339)

	return item, nil
}

// BulkCreateItems creates multiple ML link list items in a single transaction
func (r *mlLinkListRepository) BulkCreateItems(ctx context.Context, items []models.MlLinkListItem) ([]models.MlLinkListItem, error) {
	if len(items) == 0 {
		return nil, nil
	}

	var createdItems []models.MlLinkListItem

	err := r.db.WithTx(ctx, func(tx pgx.Tx) error {
		query := `
			INSERT INTO ml_link_list_items (list_id, origin_url, order_index, is_active,
			                                product_title, product_price, product_image_url, metadata,
			                                created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			RETURNING id, created_at, updated_at
		`

		now := time.Now().UTC()

		for _, item := range items {
			metadata := item.Metadata
			if metadata == nil {
				metadata = []byte("{}")
			}

			var id string
			var createdAt, updatedAt time.Time

			err := tx.QueryRow(ctx, query,
				item.ListID,
				item.OriginURL,
				item.OrderIndex,
				item.IsActive,
				item.ProductTitle,
				item.ProductPrice,
				item.ProductImageURL,
				metadata,
				now,
				now,
			).Scan(&id, &createdAt, &updatedAt)

			if err != nil {
				return fmt.Errorf("failed to create item: %w", err)
			}

			item.ID = id
			item.CreatedAt = createdAt.Format(time.RFC3339)
			item.UpdatedAt = updatedAt.Format(time.RFC3339)
			createdItems = append(createdItems, item)
		}

		return nil
	})

	if err != nil {
		r.logger.Error("Failed to bulk create ML link list items",
			zap.Int("count", len(items)),
			zap.Error(err))
		return nil, fmt.Errorf("failed to bulk create ML link list items: %w", err)
	}

	return createdItems, nil
}

// UpdateItem updates an existing ML link list item
func (r *mlLinkListRepository) UpdateItem(ctx context.Context, itemID string, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return nil
	}

	// Build dynamic update query
	setClauses := []string{}
	args := []interface{}{}
	argIndex := 1

	for key, value := range updates {
		switch key {
		case "origin_url", "order_index", "is_active", "consumed_at",
			"consumed_by_automation_id", "consumed_by_group_id",
			"product_title", "product_price", "product_image_url", "metadata":
			setClauses = append(setClauses, fmt.Sprintf("%s = $%d", key, argIndex))
			args = append(args, value)
			argIndex++
		}
	}

	if len(setClauses) == 0 {
		return nil
	}

	// Add updated_at
	setClauses = append(setClauses, fmt.Sprintf("updated_at = $%d", argIndex))
	args = append(args, time.Now().UTC())
	argIndex++

	// Add itemID as last argument
	args = append(args, itemID)

	query := fmt.Sprintf(
		"UPDATE ml_link_list_items SET %s WHERE id = $%d",
		joinClauses(setClauses),
		argIndex,
	)

	result, err := r.db.pool.Exec(ctx, query, args...)
	if err != nil {
		r.logger.Error("Failed to update ML link list item",
			zap.String("itemID", itemID),
			zap.Error(err))
		return fmt.Errorf("failed to update ML link list item: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("ML link list item not found")
	}

	return nil
}

// DeleteItem deletes an ML link list item
func (r *mlLinkListRepository) DeleteItem(ctx context.Context, itemID string) error {
	query := `DELETE FROM ml_link_list_items WHERE id = $1`

	result, err := r.db.pool.Exec(ctx, query, itemID)
	if err != nil {
		r.logger.Error("Failed to delete ML link list item",
			zap.String("itemID", itemID),
			zap.Error(err))
		return fmt.Errorf("failed to delete ML link list item: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("ML link list item not found")
	}

	return nil
}

// ResetConsumedItems resets all consumed items in a list
func (r *mlLinkListRepository) ResetConsumedItems(ctx context.Context, listID string) (int64, error) {
	query := `
		UPDATE ml_link_list_items
		SET consumed_at = NULL,
		    consumed_by_automation_id = NULL,
		    consumed_by_group_id = NULL,
		    updated_at = $2
		WHERE list_id = $1
	`

	result, err := r.db.pool.Exec(ctx, query, listID, time.Now().UTC())
	if err != nil {
		r.logger.Error("Failed to reset consumed items",
			zap.String("listID", listID),
			zap.Error(err))
		return 0, fmt.Errorf("failed to reset consumed items: %w", err)
	}

	return result.RowsAffected(), nil
}

// ReorderItems updates the order_index for multiple items
func (r *mlLinkListRepository) ReorderItems(ctx context.Context, listID string, orderMap map[string]int32) error {
	if len(orderMap) == 0 {
		return nil
	}

	return r.db.WithTx(ctx, func(tx pgx.Tx) error {
		query := `UPDATE ml_link_list_items SET order_index = $1, updated_at = $2 WHERE id = $3 AND list_id = $4`
		now := time.Now().UTC()

		for itemID, orderIndex := range orderMap {
			_, err := tx.Exec(ctx, query, orderIndex, now, itemID, listID)
			if err != nil {
				return fmt.Errorf("failed to update order for item %s: %w", itemID, err)
			}
		}

		return nil
	})
}

// GetMaxOrderIndex returns the maximum order_index for a list
func (r *mlLinkListRepository) GetMaxOrderIndex(ctx context.Context, listID string) (int, error) {
	query := `SELECT COALESCE(MAX(order_index), -1) FROM ml_link_list_items WHERE list_id = $1`

	var maxIndex int
	err := r.db.QueryRow(ctx, query, listID).Scan(&maxIndex)
	if err != nil {
		r.logger.Error("Failed to get max order index",
			zap.String("listID", listID),
			zap.Error(err))
		return 0, fmt.Errorf("failed to get max order index: %w", err)
	}

	return maxIndex, nil
}

// ============================================================================
// Automation consumption operations
// ============================================================================

// GetUnconsumedItems retrieves unconsumed items from a list, ordered by order_index and created_at.
// Items are considered unconsumed when is_active=true AND consumed_at IS NULL.
func (r *mlLinkListRepository) GetUnconsumedItems(ctx context.Context, listID string, limit int) ([]models.MlLinkListItem, error) {
	query := `
		SELECT id, list_id, origin_url, order_index, is_active, consumed_at,
		       consumed_by_automation_id, consumed_by_group_id, product_title,
		       product_price, product_image_url, metadata, created_at, updated_at
		FROM ml_link_list_items
		WHERE list_id = $1 AND is_active = true AND consumed_at IS NULL
		ORDER BY order_index ASC, created_at ASC
		LIMIT $2
	`

	rows, err := r.db.Query(ctx, query, listID, limit)
	if err != nil {
		r.logger.Error("Failed to get unconsumed items",
			zap.String("listID", listID),
			zap.Int("limit", limit),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get unconsumed items: %w", err)
	}
	defer rows.Close()

	var items []models.MlLinkListItem
	for rows.Next() {
		var item models.MlLinkListItem
		var createdAt, updatedAt time.Time
		var consumedAt *time.Time

		err := rows.Scan(
			&item.ID,
			&item.ListID,
			&item.OriginURL,
			&item.OrderIndex,
			&item.IsActive,
			&consumedAt,
			&item.ConsumedByAutomationID,
			&item.ConsumedByGroupID,
			&item.ProductTitle,
			&item.ProductPrice,
			&item.ProductImageURL,
			&item.Metadata,
			&createdAt,
			&updatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan unconsumed item: %w", err)
		}

		item.CreatedAt = createdAt.Format(time.RFC3339)
		item.UpdatedAt = updatedAt.Format(time.RFC3339)
		if consumedAt != nil {
			formatted := consumedAt.Format(time.RFC3339)
			item.ConsumedAt = &formatted
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating unconsumed items: %w", err)
	}

	return items, nil
}

// MarkConsumed marks an item as consumed using optimistic locking.
// Returns an error if the item was already consumed by another process.
func (r *mlLinkListRepository) MarkConsumed(ctx context.Context, itemID, automationID string, consumedAt time.Time) error {
	query := `
		UPDATE ml_link_list_items
		SET consumed_at = $2,
		    consumed_by_automation_id = $3,
		    updated_at = $4
		WHERE id = $1 AND consumed_at IS NULL
		RETURNING id
	`

	var returnedID string
	err := r.db.QueryRow(ctx, query, itemID, consumedAt, automationID, time.Now().UTC()).Scan(&returnedID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Item was already consumed by another process
			return fmt.Errorf("item already consumed or not found: %s", itemID)
		}
		r.logger.Error("Failed to mark item as consumed",
			zap.String("itemID", itemID),
			zap.String("automationID", automationID),
			zap.Error(err))
		return fmt.Errorf("failed to mark item as consumed: %w", err)
	}

	return nil
}
