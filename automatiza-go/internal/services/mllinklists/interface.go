// Package mllinklists provides the service layer for Mercado Livre link list operations.
package mllinklists

import (
	"context"

	"automatiza-go/internal/models"
)

// Service defines the interface for ML link list business operations.
type Service interface {
	// List operations
	ListLists(ctx context.Context, userID string) ([]models.MlLinkList, error)
	GetList(ctx context.Context, listID, userID string) (*models.MlLinkList, error)
	CreateList(ctx context.Context, userID string, input CreateListInput) (*models.MlLinkList, error)
	UpdateList(ctx context.Context, listID, userID string, input UpdateListInput) (*models.MlLinkList, error)
	DeleteList(ctx context.Context, listID, userID string) error

	// Item operations
	ListItems(ctx context.Context, listID, userID string) ([]models.MlLinkListItem, error)
	ListItemsPaginated(ctx context.Context, listID, userID string, pagination PaginationInput) (*ListItemsResult, error)
	CreateItem(ctx context.Context, userID string, input CreateItemInput) (*models.MlLinkListItem, error)
	BulkCreateItems(ctx context.Context, userID string, input BulkCreateItemsInput) (*BulkCreateResult, error)
	UpdateItem(ctx context.Context, userID, itemID string, input UpdateItemInput) (*models.MlLinkListItem, error)
	DeleteItem(ctx context.Context, userID, itemID string) error
	ResetConsumed(ctx context.Context, userID, listID string) (int64, error)
	ReorderItems(ctx context.Context, userID, listID string, orderMap map[string]int32) error

	// Automation operations
	ConsumeNextItems(ctx context.Context, listID, automationID string, count int) ([]models.MlLinkListItem, error)
}

// CreateListInput contains the input for creating a new ML link list.
type CreateListInput struct {
	Name        string
	Description *string
	IsActive    *bool
}

// UpdateListInput contains the input for updating an ML link list.
type UpdateListInput struct {
	Name        *string
	Description *string
	IsActive    *bool
}

// CreateItemInput contains the input for creating a new ML link list item.
type CreateItemInput struct {
	ListID          string
	OriginURL       string
	OrderIndex      *int32
	IsActive        *bool
	ProductTitle    *string
	ProductPrice    *float64
	ProductImageURL *string
	Metadata        *string
}

// BulkCreateItemsInput contains the input for bulk creating ML link list items.
type BulkCreateItemsInput struct {
	ListID         string
	OriginURLs     []string
	BaseOrderIndex *int32
}

// BulkCreateResult contains the result of a bulk create operation.
type BulkCreateResult struct {
	CreatedCount int
	SkippedCount int
	SkippedURLs  []string
	Items        []models.MlLinkListItem
}

// UpdateItemInput contains the input for updating an ML link list item.
type UpdateItemInput struct {
	IsActive   *bool
	OrderIndex *int32
}

// PaginationInput contains pagination parameters for list operations.
type PaginationInput struct {
	PageSize  int32
	PageToken string
}

// ListItemsResult contains the paginated result of listing items.
type ListItemsResult struct {
	Items         []models.MlLinkListItem
	NextPageToken string
	TotalCount    int32
	HasMore       bool
}
