package mllinklists

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	apperrors "automatiza-go/internal/errors"
	"automatiza-go/internal/lib/urlvalidation"
	"automatiza-go/internal/models"
	"automatiza-go/internal/observability"
	"automatiza-go/internal/repository"

	"github.com/getsentry/sentry-go"
	"go.uber.org/zap"
)

// service implements the Service interface.
type service struct {
	repo        repository.MLLinkListRepository
	historyRepo repository.ActionHistoryRepository
	logger      *zap.Logger
}

// NewService creates a new ML link list service.
func NewService(repo repository.MLLinkListRepository, logger *zap.Logger) Service {
	return &service{
		repo:   repo,
		logger: logger,
	}
}

// NewServiceWithAudit creates a new ML link list service with audit trail support.
func NewServiceWithAudit(repo repository.MLLinkListRepository, historyRepo repository.ActionHistoryRepository, logger *zap.Logger) Service {
	return &service{
		repo:        repo,
		historyRepo: historyRepo,
		logger:      logger,
	}
}

// logAudit logs an action to the action history (best effort - does not fail on error).
func (s *service) logAudit(ctx context.Context, userID, actionType, description string, details map[string]interface{}) {
	if s.historyRepo == nil {
		return
	}

	_, err := s.historyRepo.Create(ctx, userID, actionType, description, details)
	if err != nil {
		s.logger.Warn("Failed to log audit action",
			zap.String("actionType", actionType),
			zap.Error(err))
	}
}

// ============================================================================
// List operations
// ============================================================================

func (s *service) ListLists(ctx context.Context, userID string) ([]models.MlLinkList, error) {
	lists, err := s.repo.GetListsByUserID(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to list ML link lists",
			zap.String("userID", userID),
			zap.Error(err))
		return nil, apperrors.WrapInternal(err, "listing ML link lists")
	}
	return lists, nil
}

func (s *service) GetList(ctx context.Context, listID, userID string) (*models.MlLinkList, error) {
	list, err := s.repo.GetListByIDAndUser(ctx, listID, userID)
	if err != nil {
		s.logger.Error("Failed to get ML link list",
			zap.String("listID", listID),
			zap.String("userID", userID),
			zap.Error(err))
		return nil, apperrors.WrapInternal(err, "getting ML link list")
	}
	if list == nil {
		return nil, apperrors.ErrListNotFound
	}
	return list, nil
}

func (s *service) CreateList(ctx context.Context, userID string, input CreateListInput) (*models.MlLinkList, error) {
	// Validate input
	if input.Name == "" {
		return nil, apperrors.NewMLLinksFieldError(
			"INVALID_NAME",
			"name cannot be empty",
			"name",
			nil,
		)
	}

	list := &models.MlLinkList{
		Name:     input.Name,
		IsActive: true,
	}

	if input.Description != nil {
		list.Description = input.Description
	}

	if input.IsActive != nil {
		list.IsActive = *input.IsActive
	}

	created, err := s.repo.CreateList(ctx, userID, list)
	if err != nil {
		s.logger.Error("Failed to create ML link list",
			zap.String("userID", userID),
			zap.String("name", input.Name),
			zap.Error(err))
		return nil, apperrors.WrapInternal(err, "creating ML link list")
	}

	s.logger.Info("ML link list created",
		zap.String("userID", userID),
		zap.String("listID", created.ID),
		zap.String("name", created.Name))

	// Audit trail
	s.logAudit(ctx, userID, models.ActionTypeMLLinkListCreate, "Lista de links criada: "+created.Name, map[string]interface{}{
		"list_id":   created.ID,
		"list_name": created.Name,
	})

	return created, nil
}

func (s *service) UpdateList(ctx context.Context, listID, userID string, input UpdateListInput) (*models.MlLinkList, error) {
	// Verify ownership
	existing, err := s.repo.GetListByIDAndUser(ctx, listID, userID)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "verifying list ownership")
	}
	if existing == nil {
		return nil, apperrors.ErrListNotFound
	}

	// Build updates map
	updates := make(map[string]interface{})
	if input.Name != nil && *input.Name != "" {
		updates["name"] = *input.Name
	}
	if input.Description != nil {
		updates["description"] = *input.Description
	}
	if input.IsActive != nil {
		updates["is_active"] = *input.IsActive
	}

	if len(updates) > 0 {
		if err := s.repo.UpdateList(ctx, listID, updates); err != nil {
			s.logger.Error("Failed to update ML link list",
				zap.String("listID", listID),
				zap.Error(err))
			return nil, apperrors.WrapInternal(err, "updating ML link list")
		}
	}

	// Fetch updated record
	updated, err := s.repo.GetListByIDAndUser(ctx, listID, userID)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "fetching updated ML link list")
	}

	s.logger.Info("ML link list updated",
		zap.String("userID", userID),
		zap.String("listID", listID))

	// Audit trail
	s.logAudit(ctx, userID, models.ActionTypeMLLinkListUpdate, "Lista de links atualizada", map[string]interface{}{
		"list_id": listID,
		"updates": updates,
	})

	return updated, nil
}

func (s *service) DeleteList(ctx context.Context, listID, userID string) error {
	if err := s.repo.DeleteList(ctx, listID, userID); err != nil {
		s.logger.Error("Failed to delete ML link list",
			zap.String("listID", listID),
			zap.String("userID", userID),
			zap.Error(err))
		return apperrors.WrapInternal(err, "deleting ML link list")
	}

	s.logger.Info("ML link list deleted",
		zap.String("userID", userID),
		zap.String("listID", listID))

	// Audit trail
	s.logAudit(ctx, userID, models.ActionTypeMLLinkListDelete, "Lista de links excluida", map[string]interface{}{
		"list_id": listID,
	})

	return nil
}

// ============================================================================
// Item operations
// ============================================================================

func (s *service) ListItems(ctx context.Context, listID, userID string) ([]models.MlLinkListItem, error) {
	// Verify ownership
	list, err := s.repo.GetListByIDAndUser(ctx, listID, userID)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "verifying list ownership")
	}
	if list == nil {
		return nil, apperrors.ErrListNotFound
	}

	items, err := s.repo.GetItemsByListID(ctx, listID)
	if err != nil {
		s.logger.Error("Failed to list ML link list items",
			zap.String("listID", listID),
			zap.Error(err))
		return nil, apperrors.WrapInternal(err, "listing ML link list items")
	}

	return items, nil
}

func (s *service) ListItemsPaginated(ctx context.Context, listID, userID string, pagination PaginationInput) (*ListItemsResult, error) {
	// Verify ownership
	list, err := s.repo.GetListByIDAndUser(ctx, listID, userID)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "verifying list ownership")
	}
	if list == nil {
		return nil, apperrors.ErrListNotFound
	}

	// Default page size
	pageSize := int(pagination.PageSize)
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	items, nextCursor, totalCount, err := s.repo.GetItemsByListIDPaginated(ctx, listID, pageSize, pagination.PageToken)
	if err != nil {
		s.logger.Error("Failed to list ML link list items paginated",
			zap.String("listID", listID),
			zap.Error(err))
		return nil, apperrors.WrapInternal(err, "listing ML link list items")
	}

	return &ListItemsResult{
		Items:         items,
		NextPageToken: nextCursor,
		TotalCount:    int32(totalCount),
		HasMore:       nextCursor != "",
	}, nil
}

func (s *service) CreateItem(ctx context.Context, userID string, input CreateItemInput) (*models.MlLinkListItem, error) {
	// Verify list ownership
	list, err := s.repo.GetListByIDAndUser(ctx, input.ListID, userID)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "verifying list ownership")
	}
	if list == nil {
		return nil, apperrors.ErrListNotFound
	}

	// Validate and normalize URL
	normalizedURL, err := urlvalidation.ValidateAndNormalizeURL(input.OriginURL)
	if err != nil {
		return nil, err
	}

	// Validate metadata JSON if provided
	if input.Metadata != nil && *input.Metadata != "" {
		if err := validateMetadataJSON(*input.Metadata); err != nil {
			return nil, err
		}
	}

	// Build item
	item := &models.MlLinkListItem{
		ListID:    input.ListID,
		OriginURL: normalizedURL,
		IsActive:  true,
	}

	// Set order index
	if input.OrderIndex != nil {
		item.OrderIndex = int(*input.OrderIndex)
	} else {
		maxIndex, err := s.repo.GetMaxOrderIndex(ctx, input.ListID)
		if err != nil {
			return nil, apperrors.WrapInternal(err, "getting max order index")
		}
		item.OrderIndex = maxIndex + 1
	}

	if input.IsActive != nil {
		item.IsActive = *input.IsActive
	}
	if input.ProductTitle != nil {
		item.ProductTitle = input.ProductTitle
	}
	if input.ProductPrice != nil {
		item.ProductPrice = input.ProductPrice
	}
	if input.ProductImageURL != nil {
		item.ProductImageURL = input.ProductImageURL
	}
	if input.Metadata != nil {
		item.Metadata = []byte(*input.Metadata)
	}

	created, err := s.repo.CreateItem(ctx, item)
	if err != nil {
		s.logger.Error("Failed to create ML link list item",
			zap.String("listID", input.ListID),
			zap.String("originURL", input.OriginURL),
			zap.Error(err))
		return nil, apperrors.WrapInternal(err, "creating ML link list item")
	}

	s.logger.Info("ML link list item created",
		zap.String("listID", input.ListID),
		zap.String("itemID", created.ID))

	// Audit trail
	s.logAudit(ctx, userID, models.ActionTypeMLLinkItemCreate, "Item adicionado a lista de links", map[string]interface{}{
		"list_id":    input.ListID,
		"item_id":    created.ID,
		"origin_url": normalizedURL,
	})

	return created, nil
}

func (s *service) BulkCreateItems(ctx context.Context, userID string, input BulkCreateItemsInput) (*BulkCreateResult, error) {
	// Verify list ownership
	list, err := s.repo.GetListByIDAndUser(ctx, input.ListID, userID)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "verifying list ownership")
	}
	if list == nil {
		return nil, apperrors.ErrListNotFound
	}

	// Validate all URLs first
	validationResult := urlvalidation.ValidateAndCategorizeURLs(input.OriginURLs)

	// Dedup intencionalmente removido: o cliente importa planilhas onde a
	// ordem das linhas e a repetição de produtos são significativas (mesmo
	// link aparece N vezes = N envios). O sistema precisa respeitar o que
	// veio na planilha. Também não filtramos contra itens já existentes na
	// lista — re-importar uma planilha gera novas entradas; quem quer
	// merge incremental usa o create-one ou edita manualmente.
	newURLs := validationResult.ValidURLs

	// Get base order index
	baseOrderIndex := 0
	if input.BaseOrderIndex != nil {
		baseOrderIndex = int(*input.BaseOrderIndex)
	} else {
		maxIndex, err := s.repo.GetMaxOrderIndex(ctx, input.ListID)
		if err != nil {
			return nil, apperrors.WrapInternal(err, "getting max order index")
		}
		baseOrderIndex = maxIndex + 1
	}

	// Build items
	items := make([]models.MlLinkListItem, len(newURLs))
	for i, rawURL := range newURLs {
		normalized, normErr := urlvalidation.NormalizeURL(rawURL)
		if normErr != nil {
			observability.LogAndCapture(s.logger, sentry.LevelWarning,
				"Failed to normalize URL for item creation, using original",
				normErr, map[string]string{
					"module": "ml_link_lists", "operation": "bulk_create_normalize_item",
				})
			normalized = rawURL
		}
		items[i] = models.MlLinkListItem{
			ListID:     input.ListID,
			OriginURL:  normalized,
			OrderIndex: baseOrderIndex + i,
			IsActive:   true,
		}
	}

	// Bulk create
	var createdItems []models.MlLinkListItem
	if len(items) > 0 {
		createdItems, err = s.repo.BulkCreateItems(ctx, items)
		if err != nil {
			s.logger.Error("Failed to bulk create ML link list items",
				zap.String("listID", input.ListID),
				zap.Int("count", len(items)),
				zap.Error(err))
			return nil, apperrors.WrapInternal(err, "bulk creating ML link list items")
		}
	}

	// Skipped agora contém apenas URLs inválidas (não-http(s), unparseable,
	// vazias) — dedup foi removido, então duplicatas não entram no skipped.
	allSkipped := make([]string, 0, len(validationResult.InvalidURLs))
	for url := range validationResult.InvalidURLs {
		allSkipped = append(allSkipped, url)
	}

	result := &BulkCreateResult{
		CreatedCount: len(createdItems),
		SkippedCount: len(allSkipped),
		SkippedURLs:  allSkipped,
		Items:        createdItems,
	}

	s.logger.Info("ML link list items bulk created",
		zap.String("listID", input.ListID),
		zap.Int("created", result.CreatedCount),
		zap.Int("skipped", result.SkippedCount))

	// Audit trail
	if result.CreatedCount > 0 {
		s.logAudit(ctx, userID, models.ActionTypeMLLinkItemBulkCreate, "Items adicionados em lote a lista de links", map[string]interface{}{
			"list_id":       input.ListID,
			"created_count": result.CreatedCount,
			"skipped_count": result.SkippedCount,
		})
	}

	return result, nil
}

func (s *service) UpdateItem(ctx context.Context, userID, itemID string, input UpdateItemInput) (*models.MlLinkListItem, error) {
	// Get item to verify ownership
	item, err := s.repo.GetItemByID(ctx, itemID)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "getting item")
	}
	if item == nil {
		return nil, apperrors.ErrItemNotFound
	}

	// Verify list ownership
	list, err := s.repo.GetListByIDAndUser(ctx, item.ListID, userID)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "verifying list ownership")
	}
	if list == nil {
		return nil, apperrors.ErrListNotFound
	}

	// Build updates map
	updates := make(map[string]interface{})
	if input.IsActive != nil {
		updates["is_active"] = *input.IsActive
	}
	if input.OrderIndex != nil {
		updates["order_index"] = *input.OrderIndex
	}

	if len(updates) > 0 {
		if err := s.repo.UpdateItem(ctx, itemID, updates); err != nil {
			s.logger.Error("Failed to update ML link list item",
				zap.String("itemID", itemID),
				zap.Error(err))
			return nil, apperrors.WrapInternal(err, "updating ML link list item")
		}
	}

	// Fetch updated record
	updated, err := s.repo.GetItemByID(ctx, itemID)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "fetching updated item")
	}

	// Audit trail
	s.logAudit(ctx, userID, models.ActionTypeMLLinkItemUpdate, "Item da lista de links atualizado", map[string]interface{}{
		"list_id": item.ListID,
		"item_id": itemID,
		"updates": updates,
	})

	return updated, nil
}

func (s *service) DeleteItem(ctx context.Context, userID, itemID string) error {
	// Get item to verify ownership
	item, err := s.repo.GetItemByID(ctx, itemID)
	if err != nil {
		return apperrors.WrapInternal(err, "getting item")
	}
	if item == nil {
		return apperrors.ErrItemNotFound
	}

	// Verify list ownership
	list, err := s.repo.GetListByIDAndUser(ctx, item.ListID, userID)
	if err != nil {
		return apperrors.WrapInternal(err, "verifying list ownership")
	}
	if list == nil {
		return apperrors.ErrListNotFound
	}

	if err := s.repo.DeleteItem(ctx, itemID); err != nil {
		s.logger.Error("Failed to delete ML link list item",
			zap.String("itemID", itemID),
			zap.Error(err))
		return apperrors.WrapInternal(err, "deleting ML link list item")
	}

	s.logger.Info("ML link list item deleted",
		zap.String("itemID", itemID),
		zap.String("listID", item.ListID))

	// Audit trail
	s.logAudit(ctx, userID, models.ActionTypeMLLinkItemDelete, "Item da lista de links excluido", map[string]interface{}{
		"list_id": item.ListID,
		"item_id": itemID,
	})

	return nil
}

func (s *service) ResetConsumed(ctx context.Context, userID, listID string) (int64, error) {
	// Verify list ownership
	list, err := s.repo.GetListByIDAndUser(ctx, listID, userID)
	if err != nil {
		return 0, apperrors.WrapInternal(err, "verifying list ownership")
	}
	if list == nil {
		return 0, apperrors.ErrListNotFound
	}

	count, err := s.repo.ResetConsumedItems(ctx, listID)
	if err != nil {
		s.logger.Error("Failed to reset consumed items",
			zap.String("listID", listID),
			zap.Error(err))
		return 0, apperrors.WrapInternal(err, "resetting consumed items")
	}

	s.logger.Info("ML link list items reset",
		zap.String("listID", listID),
		zap.Int64("count", count))

	// Audit trail
	s.logAudit(ctx, userID, models.ActionTypeMLLinkItemsReset, "Items da lista de links resetados", map[string]interface{}{
		"list_id":     listID,
		"items_reset": count,
	})

	return count, nil
}

func (s *service) ReorderItems(ctx context.Context, userID, listID string, orderMap map[string]int32) error {
	// Verify list ownership
	list, err := s.repo.GetListByIDAndUser(ctx, listID, userID)
	if err != nil {
		return apperrors.WrapInternal(err, "verifying list ownership")
	}
	if list == nil {
		return apperrors.ErrListNotFound
	}

	if err := s.repo.ReorderItems(ctx, listID, orderMap); err != nil {
		s.logger.Error("Failed to reorder items",
			zap.String("listID", listID),
			zap.Error(err))
		return apperrors.WrapInternal(err, "reordering items")
	}

	// Audit trail
	s.logAudit(ctx, userID, models.ActionTypeMLLinkItemsReorder, "Items da lista de links reordenados", map[string]interface{}{
		"list_id":     listID,
		"items_count": len(orderMap),
	})

	return nil
}

// ============================================================================
// Automation operations
// ============================================================================

func (s *service) ConsumeNextItems(ctx context.Context, listID, automationID string, count int) ([]models.MlLinkListItem, error) {
	if count <= 0 {
		count = 1
	}

	// Get unconsumed items
	items, err := s.repo.GetUnconsumedItems(ctx, listID, count)
	if err != nil {
		s.logger.Error("Failed to get unconsumed items",
			zap.String("listID", listID),
			zap.Error(err))
		return nil, apperrors.WrapInternal(err, "getting unconsumed items")
	}

	if len(items) == 0 {
		return nil, apperrors.ErrNoItemsAvailable
	}

	// Mark items as consumed
	consumedItems := make([]models.MlLinkListItem, 0, len(items))
	now := time.Now().UTC()

	for _, item := range items {
		err := s.repo.MarkConsumed(ctx, item.ID, automationID, now)
		if err != nil {
			// Another worker consumed first, skip this item
			s.logger.Debug("Item already consumed by another worker",
				zap.String("itemID", item.ID))
			continue
		}

		// Update item with consumed info for return
		consumedAt := now.Format(time.RFC3339)
		item.ConsumedAt = &consumedAt
		item.ConsumedByAutomationID = &automationID
		consumedItems = append(consumedItems, item)
	}

	if len(consumedItems) == 0 {
		return nil, apperrors.ErrNoItemsAvailable
	}

	s.logger.Info("ML link list items consumed",
		zap.String("listID", listID),
		zap.String("automationID", automationID),
		zap.Int("count", len(consumedItems)))

	return consumedItems, nil
}

// ============================================================================
// Helper functions
// ============================================================================

func validateMetadataJSON(metadata string) error {
	if metadata == "" {
		return nil
	}

	var tmp map[string]interface{}
	if err := json.Unmarshal([]byte(metadata), &tmp); err != nil {
		return apperrors.NewMLLinksFieldError(
			apperrors.ErrInvalidMetadataJSON.Error(),
			fmt.Sprintf("invalid JSON: %v", err),
			"metadata",
			apperrors.ErrInvalidMetadataJSON,
		)
	}

	return nil
}
