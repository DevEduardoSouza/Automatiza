package connectrpc

import (
	"context"
	"errors"

	commonv1 "automatiza-go/internal/gen/common/v1"
	mllinksv1 "automatiza-go/internal/gen/ml_links/v1"
	"automatiza-go/internal/models"
	"automatiza-go/internal/services/mllinklists"

	apperrors "automatiza-go/internal/errors"

	"connectrpc.com/connect"
)

// MlLinksHandler implements the ml_linksv1connect.MlLinksServiceHandler interface.
type MlLinksHandler struct {
	service mllinklists.Service
}

// NewMlLinksHandler creates a new ML links handler.
func NewMlLinksHandler(service mllinklists.Service) *MlLinksHandler {
	return &MlLinksHandler{
		service: service,
	}
}

// ListMlLinkLists lists all ML link lists for the authenticated user.
func (h *MlLinksHandler) ListMlLinkLists(
	ctx context.Context,
	req *connect.Request[mllinksv1.ListMlLinkListsRequest],
) (*connect.Response[mllinksv1.ListMlLinkListsResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
	}

	lists, err := h.service.ListLists(ctx, user.ID)
	if err != nil {
		return nil, mapErrorToConnect(err)
	}

	protoLists := make([]*mllinksv1.MlLinkList, len(lists))
	for i, l := range lists {
		protoLists[i] = mlLinkListToProto(&l)
	}

	return connect.NewResponse(&mllinksv1.ListMlLinkListsResponse{
		Lists: protoLists,
	}), nil
}

// GetMlLinkList gets a specific ML link list by ID.
func (h *MlLinksHandler) GetMlLinkList(
	ctx context.Context,
	req *connect.Request[mllinksv1.GetMlLinkListRequest],
) (*connect.Response[mllinksv1.GetMlLinkListResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
	}

	list, err := h.service.GetList(ctx, req.Msg.GetId(), user.ID)
	if err != nil {
		return nil, mapErrorToConnect(err)
	}

	return connect.NewResponse(&mllinksv1.GetMlLinkListResponse{
		List: mlLinkListToProto(list),
	}), nil
}

// CreateMlLinkList creates a new ML link list.
func (h *MlLinksHandler) CreateMlLinkList(
	ctx context.Context,
	req *connect.Request[mllinksv1.CreateMlLinkListRequest],
) (*connect.Response[mllinksv1.CreateMlLinkListResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
	}

	input := mllinklists.CreateListInput{
		Name: req.Msg.GetName(),
	}
	if req.Msg.Description != nil {
		desc := req.Msg.GetDescription()
		input.Description = &desc
	}
	if req.Msg.IsActive != nil {
		active := req.Msg.GetIsActive()
		input.IsActive = &active
	}

	created, err := h.service.CreateList(ctx, user.ID, input)
	if err != nil {
		return nil, mapErrorToConnect(err)
	}

	return connect.NewResponse(&mllinksv1.CreateMlLinkListResponse{
		List: mlLinkListToProto(created),
	}), nil
}

// UpdateMlLinkList updates an existing ML link list.
func (h *MlLinksHandler) UpdateMlLinkList(
	ctx context.Context,
	req *connect.Request[mllinksv1.UpdateMlLinkListRequest],
) (*connect.Response[mllinksv1.UpdateMlLinkListResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
	}

	input := mllinklists.UpdateListInput{}
	if req.Msg.Name != nil {
		name := req.Msg.GetName()
		input.Name = &name
	}
	if req.Msg.Description != nil {
		desc := req.Msg.GetDescription()
		input.Description = &desc
	}
	if req.Msg.IsActive != nil {
		active := req.Msg.GetIsActive()
		input.IsActive = &active
	}

	updated, err := h.service.UpdateList(ctx, req.Msg.GetId(), user.ID, input)
	if err != nil {
		return nil, mapErrorToConnect(err)
	}

	return connect.NewResponse(&mllinksv1.UpdateMlLinkListResponse{
		List: mlLinkListToProto(updated),
	}), nil
}

// DeleteMlLinkList deletes an ML link list and all its items.
func (h *MlLinksHandler) DeleteMlLinkList(
	ctx context.Context,
	req *connect.Request[mllinksv1.DeleteMlLinkListRequest],
) (*connect.Response[mllinksv1.DeleteMlLinkListResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
	}

	if err := h.service.DeleteList(ctx, req.Msg.GetId(), user.ID); err != nil {
		return nil, mapErrorToConnect(err)
	}

	return connect.NewResponse(&mllinksv1.DeleteMlLinkListResponse{}), nil
}

// ListMlLinkListItems lists all items in an ML link list.
// Supports optional pagination via the pagination field.
func (h *MlLinksHandler) ListMlLinkListItems(
	ctx context.Context,
	req *connect.Request[mllinksv1.ListMlLinkListItemsRequest],
) (*connect.Response[mllinksv1.ListMlLinkListItemsResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
	}

	// Check if pagination is requested
	if req.Msg.Pagination != nil {
		pagination := mllinklists.PaginationInput{
			PageSize:  req.Msg.Pagination.GetPageSize(),
			PageToken: req.Msg.Pagination.GetPageToken(),
		}

		result, err := h.service.ListItemsPaginated(ctx, req.Msg.GetListId(), user.ID, pagination)
		if err != nil {
			return nil, mapErrorToConnect(err)
		}

		protoItems := make([]*mllinksv1.MlLinkListItem, len(result.Items))
		for i, item := range result.Items {
			protoItems[i] = mlLinkListItemToProto(&item)
		}

		return connect.NewResponse(&mllinksv1.ListMlLinkListItemsResponse{
			Items: protoItems,
			Pagination: &commonv1.PaginationResponse{
				NextPageToken: result.NextPageToken,
				TotalCount:    result.TotalCount,
				HasMore:       result.HasMore,
			},
		}), nil
	}

	// No pagination requested, return all items (backwards compatible)
	items, err := h.service.ListItems(ctx, req.Msg.GetListId(), user.ID)
	if err != nil {
		return nil, mapErrorToConnect(err)
	}

	protoItems := make([]*mllinksv1.MlLinkListItem, len(items))
	for i, item := range items {
		protoItems[i] = mlLinkListItemToProto(&item)
	}

	return connect.NewResponse(&mllinksv1.ListMlLinkListItemsResponse{
		Items: protoItems,
	}), nil
}

// CreateMlLinkListItem creates a new item in an ML link list.
func (h *MlLinksHandler) CreateMlLinkListItem(
	ctx context.Context,
	req *connect.Request[mllinksv1.CreateMlLinkListItemRequest],
) (*connect.Response[mllinksv1.CreateMlLinkListItemResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
	}

	input := mllinklists.CreateItemInput{
		ListID:    req.Msg.GetListId(),
		OriginURL: req.Msg.GetOriginUrl(),
	}
	if req.Msg.OrderIndex != nil {
		idx := req.Msg.GetOrderIndex()
		input.OrderIndex = &idx
	}
	if req.Msg.IsActive != nil {
		active := req.Msg.GetIsActive()
		input.IsActive = &active
	}
	if req.Msg.ProductTitle != nil {
		title := req.Msg.GetProductTitle()
		input.ProductTitle = &title
	}
	if req.Msg.ProductPrice != nil {
		price := req.Msg.GetProductPrice()
		input.ProductPrice = &price
	}
	if req.Msg.ProductImageUrl != nil {
		imageURL := req.Msg.GetProductImageUrl()
		input.ProductImageURL = &imageURL
	}
	if req.Msg.Metadata != nil {
		metadata := req.Msg.GetMetadata()
		input.Metadata = &metadata
	}

	created, err := h.service.CreateItem(ctx, user.ID, input)
	if err != nil {
		return nil, mapErrorToConnect(err)
	}

	return connect.NewResponse(&mllinksv1.CreateMlLinkListItemResponse{
		Item: mlLinkListItemToProto(created),
	}), nil
}

// BulkCreateMlLinkListItems creates multiple items in an ML link list.
func (h *MlLinksHandler) BulkCreateMlLinkListItems(
	ctx context.Context,
	req *connect.Request[mllinksv1.BulkCreateMlLinkListItemsRequest],
) (*connect.Response[mllinksv1.BulkCreateMlLinkListItemsResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
	}

	input := mllinklists.BulkCreateItemsInput{
		ListID:     req.Msg.GetListId(),
		OriginURLs: req.Msg.GetOriginUrls(),
	}
	if req.Msg.BaseOrderIndex != nil {
		idx := req.Msg.GetBaseOrderIndex()
		input.BaseOrderIndex = &idx
	}

	result, err := h.service.BulkCreateItems(ctx, user.ID, input)
	if err != nil {
		return nil, mapErrorToConnect(err)
	}

	protoItems := make([]*mllinksv1.MlLinkListItem, len(result.Items))
	for i, item := range result.Items {
		protoItems[i] = mlLinkListItemToProto(&item)
	}

	return connect.NewResponse(&mllinksv1.BulkCreateMlLinkListItemsResponse{
		CreatedCount: int32(result.CreatedCount),
		SkippedCount: int32(result.SkippedCount),
		SkippedUrls:  result.SkippedURLs,
		Items:        protoItems,
	}), nil
}

// UpdateMlLinkListItem updates an item in an ML link list.
func (h *MlLinksHandler) UpdateMlLinkListItem(
	ctx context.Context,
	req *connect.Request[mllinksv1.UpdateMlLinkListItemRequest],
) (*connect.Response[mllinksv1.UpdateMlLinkListItemResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
	}

	input := mllinklists.UpdateItemInput{}
	if req.Msg.IsActive != nil {
		active := req.Msg.GetIsActive()
		input.IsActive = &active
	}
	if req.Msg.OrderIndex != nil {
		idx := req.Msg.GetOrderIndex()
		input.OrderIndex = &idx
	}

	updated, err := h.service.UpdateItem(ctx, user.ID, req.Msg.GetId(), input)
	if err != nil {
		return nil, mapErrorToConnect(err)
	}

	return connect.NewResponse(&mllinksv1.UpdateMlLinkListItemResponse{
		Item: mlLinkListItemToProto(updated),
	}), nil
}

// DeleteMlLinkListItem deletes an item from an ML link list.
func (h *MlLinksHandler) DeleteMlLinkListItem(
	ctx context.Context,
	req *connect.Request[mllinksv1.DeleteMlLinkListItemRequest],
) (*connect.Response[mllinksv1.DeleteMlLinkListItemResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
	}

	if err := h.service.DeleteItem(ctx, user.ID, req.Msg.GetId()); err != nil {
		return nil, mapErrorToConnect(err)
	}

	return connect.NewResponse(&mllinksv1.DeleteMlLinkListItemResponse{}), nil
}

// ResetConsumedItems resets all consumed items in a list.
func (h *MlLinksHandler) ResetConsumedItems(
	ctx context.Context,
	req *connect.Request[mllinksv1.ResetConsumedItemsRequest],
) (*connect.Response[mllinksv1.ResetConsumedItemsResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
	}

	count, err := h.service.ResetConsumed(ctx, user.ID, req.Msg.GetListId())
	if err != nil {
		return nil, mapErrorToConnect(err)
	}

	return connect.NewResponse(&mllinksv1.ResetConsumedItemsResponse{
		ResetCount: int32(count),
	}), nil
}

// ReorderItems reorders items in a list.
func (h *MlLinksHandler) ReorderItems(
	ctx context.Context,
	req *connect.Request[mllinksv1.ReorderItemsRequest],
) (*connect.Response[mllinksv1.ReorderItemsResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
	}

	if err := h.service.ReorderItems(ctx, user.ID, req.Msg.GetListId(), req.Msg.GetOrderMap()); err != nil {
		return nil, mapErrorToConnect(err)
	}

	return connect.NewResponse(&mllinksv1.ReorderItemsResponse{}), nil
}

// ============================================================================
// Error mapping
// ============================================================================

// mapErrorToConnect converts domain errors to connect errors with i18n-friendly error codes.
// Error codes follow the pattern: ML_LINK_*, INVALID_*, etc.
// Frontend should map these codes to localized messages.
func mapErrorToConnect(err error) error {
	if err == nil {
		return nil
	}

	// Check for specific domain errors - return error code for i18n
	if errors.Is(err, apperrors.ErrListNotFound) {
		return connect.NewError(connect.CodeNotFound, errors.New("ML_LINK_LIST_NOT_FOUND"))
	}
	if errors.Is(err, apperrors.ErrItemNotFound) {
		return connect.NewError(connect.CodeNotFound, errors.New("ML_LINK_LIST_ITEM_NOT_FOUND"))
	}
	if errors.Is(err, apperrors.ErrUnauthorized) {
		return connect.NewError(connect.CodePermissionDenied, errors.New("UNAUTHORIZED"))
	}
	if errors.Is(err, apperrors.ErrNoItemsAvailable) {
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("NO_ITEMS_AVAILABLE"))
	}
	if errors.Is(err, apperrors.ErrListNotConfigured) {
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("ML_LINK_LIST_NOT_CONFIGURED"))
	}
	if errors.Is(err, apperrors.ErrNoGroupsConfigured) {
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("NO_GROUPS_CONFIGURED"))
	}
	if errors.Is(err, apperrors.ErrWhatsAppNotConnected) {
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("WHATSAPP_NOT_CONNECTED"))
	}

	// Validation errors - return error code for i18n
	if errors.Is(err, apperrors.ErrInvalidURL) {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("INVALID_MERCADOLIVRE_URL"))
	}
	if errors.Is(err, apperrors.ErrInvalidURLFormat) {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("INVALID_URL_FORMAT"))
	}
	if errors.Is(err, apperrors.ErrInvalidMetadataJSON) {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("INVALID_METADATA_JSON"))
	}

	// Already exists errors
	if errors.Is(err, apperrors.ErrURLAlreadyExists) {
		return connect.NewError(connect.CodeAlreadyExists, errors.New("URL_ALREADY_EXISTS"))
	}
	if errors.Is(err, apperrors.ErrItemAlreadyConsumed) {
		return connect.NewError(connect.CodeAlreadyExists, errors.New("ITEM_ALREADY_CONSUMED"))
	}

	// Check for MLLinksError with custom code
	var mlErr *apperrors.MLLinksError
	if errors.As(err, &mlErr) {
		switch mlErr.Code {
		case "INTERNAL_ERROR":
			return connect.NewError(connect.CodeInternal, errors.New("INTERNAL_ERROR"))
		default:
			return connect.NewError(connect.CodeInvalidArgument, errors.New(mlErr.Code))
		}
	}

	// Default to internal error with code
	return connect.NewError(connect.CodeInternal, errors.New("INTERNAL_ERROR"))
}

// ============================================================================
// Helper functions
// ============================================================================

func mlLinkListToProto(list *models.MlLinkList) *mllinksv1.MlLinkList {
	if list == nil {
		return nil
	}

	proto := &mllinksv1.MlLinkList{
		Id:        list.ID,
		UserId:    list.UserID,
		Name:      list.Name,
		IsActive:  list.IsActive,
		CreatedAt: list.CreatedAt,
		UpdatedAt: list.UpdatedAt,
	}

	if list.Description != nil {
		proto.Description = list.Description
	}

	return proto
}

func mlLinkListItemToProto(item *models.MlLinkListItem) *mllinksv1.MlLinkListItem {
	if item == nil {
		return nil
	}

	proto := &mllinksv1.MlLinkListItem{
		Id:         item.ID,
		ListId:     item.ListID,
		OriginUrl:  item.OriginURL,
		OrderIndex: int32(item.OrderIndex),
		IsActive:   item.IsActive,
		CreatedAt:  item.CreatedAt,
		UpdatedAt:  item.UpdatedAt,
	}

	if item.ConsumedAt != nil {
		proto.ConsumedAt = item.ConsumedAt
	}
	if item.ConsumedByAutomationID != nil {
		proto.ConsumedByAutomationId = item.ConsumedByAutomationID
	}
	if item.ConsumedByGroupID != nil {
		proto.ConsumedByGroupId = item.ConsumedByGroupID
	}
	if item.ProductTitle != nil {
		proto.ProductTitle = item.ProductTitle
	}
	if item.ProductPrice != nil {
		proto.ProductPrice = item.ProductPrice
	}
	if item.ProductImageURL != nil {
		proto.ProductImageUrl = item.ProductImageURL
	}
	if item.Metadata != nil && len(item.Metadata) > 0 {
		metadataStr := string(item.Metadata)
		proto.Metadata = &metadataStr
	}

	return proto
}
