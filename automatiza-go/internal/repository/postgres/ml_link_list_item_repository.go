package postgres

import (
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

// mlLinkListItemRepository implements MlLinkListItemRepository for LIST mode automation consumption
type mlLinkListItemRepository struct {
	db     *DB
	logger *zap.Logger
}

// NewMlLinkListItemRepository creates a new PostgreSQL ML link list item repository
func NewMlLinkListItemRepository(db *DB, logger *zap.Logger) repository.MlLinkListItemRepository {
	return &mlLinkListItemRepository{
		db:     db,
		logger: logger,
	}
}

// GetNextUnconsumed returns the next N unconsumed items from a list, ordered by order_index
func (r *mlLinkListItemRepository) GetNextUnconsumed(ctx context.Context, listID string, limit int) ([]*models.MlLinkListItem, error) {
	if limit <= 0 {
		limit = 1
	}

	query := `
		SELECT id, list_id, origin_url, order_index,
		       is_active, consumed_at, consumed_by_automation_id, consumed_by_group_id,
		       product_title, product_price, product_image_url, metadata, info,
		       created_at, updated_at
		FROM ml_link_list_items
		WHERE list_id = $1
		  AND consumed_at IS NULL
		  AND is_active = true
		ORDER BY order_index ASC, created_at ASC
		LIMIT $2
	`

	rows, err := r.db.Query(ctx, query, listID, limit)
	if err != nil {
		r.logger.Error("Failed to get next unconsumed items",
			zap.String("listID", listID),
			zap.Int("limit", limit),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get next unconsumed items: %w", err)
	}
	defer rows.Close()

	return r.scanItems(rows)
}

// MarkAsConsumed marks items as consumed with the automation ID and optional group ID
func (r *mlLinkListItemRepository) MarkAsConsumed(ctx context.Context, itemIDs []string, automationID string, groupID *string) error {
	if len(itemIDs) == 0 {
		return nil
	}

	now := time.Now().UTC()

	// Build placeholders for item IDs
	placeholders := make([]string, len(itemIDs))
	args := make([]interface{}, len(itemIDs)+3)

	for i, id := range itemIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}

	// Add remaining parameters
	args[len(itemIDs)] = now
	args[len(itemIDs)+1] = automationID
	args[len(itemIDs)+2] = groupID

	query := fmt.Sprintf(`
		UPDATE ml_link_list_items
		SET consumed_at = $%d,
		    consumed_by_automation_id = $%d,
		    consumed_by_group_id = $%d,
		    updated_at = $%d
		WHERE id IN (%s)
	`,
		len(itemIDs)+1, // consumed_at
		len(itemIDs)+2, // consumed_by_automation_id
		len(itemIDs)+3, // consumed_by_group_id
		len(itemIDs)+1, // updated_at (same as consumed_at)
		strings.Join(placeholders, ", "),
	)

	result, err := r.db.pool.Exec(ctx, query, args...)
	if err != nil {
		r.logger.Error("Failed to mark items as consumed",
			zap.Strings("itemIDs", itemIDs),
			zap.String("automationID", automationID),
			zap.Error(err))
		return fmt.Errorf("failed to mark items as consumed: %w", err)
	}

	r.logger.Debug("Marked items as consumed",
		zap.Int64("rowsAffected", result.RowsAffected()),
		zap.Int("itemCount", len(itemIDs)),
		zap.String("automationID", automationID))

	return nil
}

// CountRemaining returns the count of unconsumed items in a list
// ResetConsumedByIDs devolve itens específicos ao estado não-consumido.
// Usado pelo reconciliador de entregas do LIST (ReconcileListDeliveries):
// um item consumido no ENFILEIRAMENTO cujo envio falhou terminalmente em
// todos os destinos volta a ser elegível no próximo ciclo, em vez de ficar
// queimado sem nunca ter sido entregue.
func (r *mlLinkListItemRepository) ResetConsumedByIDs(ctx context.Context, itemIDs []string) (int64, error) {
	if len(itemIDs) == 0 {
		return 0, nil
	}

	query := `
		UPDATE ml_link_list_items
		SET consumed_at = NULL,
		    consumed_by_automation_id = NULL,
		    consumed_by_group_id = NULL
		WHERE id = ANY($1::uuid[]) AND consumed_at IS NOT NULL
	`

	result, err := r.db.pool.Exec(ctx, query, itemIDs)
	if err != nil {
		r.logger.Error("Failed to reset consumed items by IDs",
			zap.Int("count", len(itemIDs)),
			zap.Error(err))
		return 0, fmt.Errorf("reset consumed by ids: %w", err)
	}

	reset := result.RowsAffected()
	if reset > 0 {
		r.logger.Info("Reset consumed list items for retry (never delivered)",
			zap.Int64("reset", reset))
	}
	return reset, nil
}

func (r *mlLinkListItemRepository) CountRemaining(ctx context.Context, listID string) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM ml_link_list_items
		WHERE list_id = $1
		  AND consumed_at IS NULL
		  AND is_active = true
	`

	var count int
	err := r.db.QueryRow(ctx, query, listID).Scan(&count)
	if err != nil {
		r.logger.Error("Failed to count remaining items",
			zap.String("listID", listID),
			zap.Error(err))
		return 0, fmt.Errorf("failed to count remaining items: %w", err)
	}

	return count, nil
}

// GetByListID returns all items for a list ordered by order_index
func (r *mlLinkListItemRepository) GetByListID(ctx context.Context, listID string) ([]*models.MlLinkListItem, error) {
	query := `
		SELECT id, list_id, origin_url, order_index,
		       is_active, consumed_at, consumed_by_automation_id, consumed_by_group_id,
		       product_title, product_price, product_image_url, metadata, info,
		       created_at, updated_at
		FROM ml_link_list_items
		WHERE list_id = $1
		ORDER BY order_index ASC, created_at ASC
	`

	rows, err := r.db.Query(ctx, query, listID)
	if err != nil {
		r.logger.Error("Failed to get items by list ID",
			zap.String("listID", listID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get items by list ID: %w", err)
	}
	defer rows.Close()

	return r.scanItems(rows)
}

// Create creates a new list item
func (r *mlLinkListItemRepository) Create(ctx context.Context, item *models.MlLinkListItem) error {
	now := time.Now().UTC()

	// Default metadata to empty JSON object if nil
	metadata := item.Metadata
	if metadata == nil {
		metadata = []byte("{}")
	}

	query := `
		INSERT INTO ml_link_list_items (
			list_id, origin_url, order_index,
			is_active, product_title, product_price, product_image_url, metadata, info,
			created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at, updated_at
	`

	var createdAt, updatedAt time.Time
	err := r.db.QueryRow(ctx, query,
		item.ListID,          // list_id
		item.OriginURL,       // origin_url
		item.OrderIndex,      // order_index
		item.IsActive,        // is_active
		item.ProductTitle,    // product_title
		item.ProductPrice,    // product_price
		item.ProductImageURL, // product_image_url
		metadata,             // metadata
		item.Info,            // info
		now,                  // created_at
		now,                  // updated_at
	).Scan(&item.ID, &createdAt, &updatedAt)

	if err != nil {
		r.logger.Error("Failed to create ML link list item",
			zap.String("listID", item.ListID),
			zap.String("originURL", item.OriginURL),
			zap.Error(err))
		return fmt.Errorf("failed to create ML link list item: %w", err)
	}

	item.CreatedAt = createdAt.Format(time.RFC3339)
	item.UpdatedAt = updatedAt.Format(time.RFC3339)

	return nil
}

// BulkCreate creates multiple list items in a single transaction
func (r *mlLinkListItemRepository) BulkCreate(ctx context.Context, items []*models.MlLinkListItem) error {
	if len(items) == 0 {
		return nil
	}

	err := r.db.WithTx(ctx, func(tx pgx.Tx) error {
		query := `
			INSERT INTO ml_link_list_items (
				list_id, origin_url, order_index,
				is_active, product_title, product_price, product_image_url, metadata, info,
				created_at, updated_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
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
				item.Info,
				now,
				now,
			).Scan(&id, &createdAt, &updatedAt)

			if err != nil {
				return fmt.Errorf("failed to create item: %w", err)
			}

			item.ID = id
			item.CreatedAt = createdAt.Format(time.RFC3339)
			item.UpdatedAt = updatedAt.Format(time.RFC3339)
		}

		return nil
	})

	if err != nil {
		r.logger.Error("Failed to bulk create ML link list items",
			zap.Int("count", len(items)),
			zap.Error(err))
		return fmt.Errorf("failed to bulk create ML link list items: %w", err)
	}

	return nil
}

// ResetConsumed resets consumed status for all items in a list
func (r *mlLinkListItemRepository) ResetConsumed(ctx context.Context, listID string) (int64, error) {
	query := `
		UPDATE ml_link_list_items
		SET consumed_at = NULL,
		    consumed_by_automation_id = NULL,
		    consumed_by_group_id = NULL,
		    updated_at = $2
		WHERE list_id = $1
		  AND consumed_at IS NOT NULL
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

// GetConsumedByAutomation returns all items consumed by a specific automation
func (r *mlLinkListItemRepository) GetConsumedByAutomation(ctx context.Context, automationID string) ([]*models.MlLinkListItem, error) {
	query := `
		SELECT id, list_id, origin_url, order_index,
		       is_active, consumed_at, consumed_by_automation_id, consumed_by_group_id,
		       product_title, product_price, product_image_url, metadata, info,
		       created_at, updated_at
		FROM ml_link_list_items
		WHERE consumed_by_automation_id = $1
		ORDER BY consumed_at DESC
	`

	rows, err := r.db.Query(ctx, query, automationID)
	if err != nil {
		r.logger.Error("Failed to get items consumed by automation",
			zap.String("automationID", automationID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get items consumed by automation: %w", err)
	}
	defer rows.Close()

	return r.scanItems(rows)
}

// scanItems scans rows into MlLinkListItem pointers
func (r *mlLinkListItemRepository) scanItems(rows pgx.Rows) ([]*models.MlLinkListItem, error) {
	var items []*models.MlLinkListItem

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
			&item.Info,
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

		items = append(items, &item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating ML link list items: %w", err)
	}

	return items, nil
}
