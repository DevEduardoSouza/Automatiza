package connectrpc

import (
	"context"
	"fmt"
	"strconv"
	"time"

	historyv1 "automatiza-go/internal/gen/history/v1"
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/structpb"
)

// HistoryHandler implements the historyv1connect.HistoryServiceHandler interface.
type HistoryHandler struct {
	historyRepo     repository.ActionHistoryRepository
	dispatchLogRepo repository.DispatchLogRepository
	groupRepo       repository.GroupRepository
}

// NewHistoryHandler creates a new history handler.
func NewHistoryHandler(historyRepo repository.ActionHistoryRepository, dispatchLogRepo repository.DispatchLogRepository) *HistoryHandler {
	return &HistoryHandler{
		historyRepo:     historyRepo,
		dispatchLogRepo: dispatchLogRepo,
	}
}

// NewHistoryHandlerWithGroups creates a new history handler with group repository for name lookups.
func NewHistoryHandlerWithGroups(
	historyRepo repository.ActionHistoryRepository,
	dispatchLogRepo repository.DispatchLogRepository,
	groupRepo repository.GroupRepository,
) *HistoryHandler {
	return &HistoryHandler{
		historyRepo:     historyRepo,
		dispatchLogRepo: dispatchLogRepo,
		groupRepo:       groupRepo,
	}
}

// ListHistory returns recent events for the authenticated user.
func (h *HistoryHandler) ListHistory(
	ctx context.Context,
	req *connect.Request[historyv1.ListHistoryRequest],
) (*connect.Response[historyv1.ListHistoryResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	// Handle page-based pagination (preferred) or legacy limit/offset
	var limit, offset int
	var page, pageSize int32

	if req.Msg.Page != nil && *req.Msg.Page > 0 {
		// Page-based pagination
		page = *req.Msg.Page
		pageSize = 20 // default
		if req.Msg.PageSize != nil && *req.Msg.PageSize > 0 {
			pageSize = *req.Msg.PageSize
		}
		if pageSize > 100 {
			pageSize = 100
		}
		limit = int(pageSize)
		offset = int((page - 1) * pageSize)
	} else {
		// Legacy limit/offset pagination
		limit = int(req.Msg.GetLimit())
		if limit <= 0 {
			limit = 50
		}
		if limit > 100 {
			limit = 100
		}
		offset = int(req.Msg.GetOffset())
		if offset < 0 {
			offset = 0
		}
		// Calculate page from offset for response
		pageSize = int32(limit)
		page = int32(offset/limit) + 1
	}

	// Build filter request
	listReq := models.HistoryListRequest{
		Limit:  limit,
		Offset: offset,
	}

	// Apply event type filter
	var actionTypeFilter *string
	if req.Msg.EventType != nil && *req.Msg.EventType != historyv1.EventType_EVENT_TYPE_UNSPECIFIED {
		actionType := mapEventTypeToActionType(*req.Msg.EventType)
		listReq.ActionType = &actionType
		actionTypeFilter = &actionType
	}

	// Apply status filter
	if req.Msg.Status != nil && *req.Msg.Status != "" {
		listReq.Status = req.Msg.Status
	}

	// Get filtered history
	history, err := h.historyRepo.GetUserHistoryFiltered(ctx, listReq, user.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get history: %w", err))
	}

	// Get total count for pagination (with same filters, but no limit)
	countReq := models.HistoryListRequest{
		Limit:      10000, // Large limit to get all for counting
		Offset:     0,
		ActionType: actionTypeFilter,
		Status:     listReq.Status,
	}
	allHistory, err := h.historyRepo.GetUserHistoryFiltered(ctx, countReq, user.ID)
	if err != nil {
		// Fallback: use current page count
		allHistory = history
	}
	totalCount := int32(len(allHistory))

	// Calculate total pages
	totalPages := totalCount / pageSize
	if totalCount%pageSize > 0 {
		totalPages++
	}
	if totalPages < 1 {
		totalPages = 1
	}

	// Calculate stats from ALL history (not just current page)
	stats := &historyv1.HistoryStats{
		Total:     totalCount,
		Completed: 0,
		Running:   0,
		Failed:    0,
	}

	for _, histItem := range allHistory {
		switch histItem.Status {
		case models.ActionStatusCompleted:
			stats.Completed++
		case models.ActionStatusRunning, models.ActionStatusPending:
			stats.Running++
		case models.ActionStatusFailed:
			stats.Failed++
		}
	}

	// Convert current page events to proto
	protoEvents := make([]*historyv1.HistoryEvent, len(history))
	for i, histItem := range history {
		protoEvents[i] = historyEventToProto(&histItem)
	}

	return connect.NewResponse(&historyv1.ListHistoryResponse{
		Events:     protoEvents,
		HasMore:    int32(len(history)) == pageSize && page < totalPages,
		Stats:      stats,
		TotalCount: totalCount,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}), nil
}

// ClearHistory deletes history entries for the authenticated user.
func (h *HistoryHandler) ClearHistory(
	ctx context.Context,
	req *connect.Request[historyv1.ClearHistoryRequest],
) (*connect.Response[historyv1.ClearHistoryResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	var beforeDate *time.Time
	if req.Msg.BeforeDate != nil && *req.Msg.BeforeDate != "" {
		t, err := time.Parse(time.RFC3339, *req.Msg.BeforeDate)
		if err == nil {
			beforeDate = &t
		}
	}

	var actionTypes []string
	for _, et := range req.Msg.EventTypes {
		if et != historyv1.EventType_EVENT_TYPE_UNSPECIFIED {
			actionTypes = append(actionTypes, mapEventTypeToActionType(et))
		}
	}

	deleted, err := h.historyRepo.DeleteUserHistory(ctx, user.ID, beforeDate, actionTypes)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to clear history: %w", err))
	}

	return connect.NewResponse(&historyv1.ClearHistoryResponse{
		DeletedCount: int32(deleted),
	}), nil
}

// Helper functions

func historyEventToProto(h *models.ActionHistory) *historyv1.HistoryEvent {
	if h == nil {
		return nil
	}

	// Convert details to structpb.Struct
	var metadata *structpb.Struct
	if h.Details != nil {
		if detailsMap, ok := h.Details.(map[string]interface{}); ok {
			metadata, _ = structpb.NewStruct(detailsMap)
		}
	}

	event := &historyv1.HistoryEvent{
		Id:          h.ID,
		UserId:      h.UserID,
		EventType:   mapActionTypeToEventType(h.ActionType),
		Description: h.Description,
		Metadata:    metadata,
		CreatedAt:   h.CreatedAt,
	}

	// Set optional status field
	if h.Status != "" {
		event.Status = &h.Status
	}

	// Set optional error message
	if h.ErrorMessage != nil {
		event.ErrorMessage = h.ErrorMessage
	}

	// Set optional completed_at
	if h.CompletedAt != nil {
		event.CompletedAt = h.CompletedAt
	}

	// Set optional counts from model fields (paridade com sistema antigo)
	if h.TargetCount != nil {
		event.TargetCount = h.TargetCount
	}
	if h.SuccessCount != nil {
		event.SuccessCount = h.SuccessCount
	}
	if h.ErrorCount != nil {
		event.ErrorCount = h.ErrorCount
	}

	return event
}

func mapActionTypeToEventType(actionType string) historyv1.EventType {
	switch actionType {
	case "message_sent", "send_message":
		return historyv1.EventType_EVENT_TYPE_MESSAGE_SENT
	case "automation_run", "run_automation":
		return historyv1.EventType_EVENT_TYPE_AUTOMATION_RUN
	case "coupon_used":
		return historyv1.EventType_EVENT_TYPE_COUPON_USED
	case "link_clicked":
		return historyv1.EventType_EVENT_TYPE_LINK_CLICKED
	case "group_sync", "sync_groups", "import_groups":
		return historyv1.EventType_EVENT_TYPE_GROUP_SYNC
	case "error":
		return historyv1.EventType_EVENT_TYPE_ERROR
	case "close_groups":
		return historyv1.EventType_EVENT_TYPE_CLOSE_GROUPS
	case "open_groups":
		return historyv1.EventType_EVENT_TYPE_OPEN_GROUPS
	case "change_name":
		return historyv1.EventType_EVENT_TYPE_CHANGE_NAME
	case "change_photo":
		return historyv1.EventType_EVENT_TYPE_CHANGE_PHOTO
	case "change_description":
		return historyv1.EventType_EVENT_TYPE_CHANGE_DESCRIPTION
	case "send_campaign":
		return historyv1.EventType_EVENT_TYPE_SEND_CAMPAIGN
	case "generate_image":
		return historyv1.EventType_EVENT_TYPE_GENERATE_IMAGE
	case "send_audio", "send_image", "send_poll":
		return historyv1.EventType_EVENT_TYPE_MESSAGE_SENT
	case "add_member":
		return historyv1.EventType_EVENT_TYPE_ADD_MEMBER
	case "remove_member":
		return historyv1.EventType_EVENT_TYPE_REMOVE_MEMBER
	case "promote_member":
		return historyv1.EventType_EVENT_TYPE_PROMOTE_MEMBER
	case "demote_member":
		return historyv1.EventType_EVENT_TYPE_DEMOTE_MEMBER
	case "leave_group":
		return historyv1.EventType_EVENT_TYPE_LEAVE_GROUP
	default:
		return historyv1.EventType_EVENT_TYPE_UNSPECIFIED
	}
}

func mapEventTypeToActionType(eventType historyv1.EventType) string {
	switch eventType {
	case historyv1.EventType_EVENT_TYPE_MESSAGE_SENT:
		return "send_message"
	case historyv1.EventType_EVENT_TYPE_AUTOMATION_RUN:
		return "run_automation"
	case historyv1.EventType_EVENT_TYPE_COUPON_USED:
		return "coupon_used"
	case historyv1.EventType_EVENT_TYPE_LINK_CLICKED:
		return "link_clicked"
	case historyv1.EventType_EVENT_TYPE_GROUP_SYNC:
		return "sync_groups"
	case historyv1.EventType_EVENT_TYPE_ERROR:
		return "error"
	case historyv1.EventType_EVENT_TYPE_CLOSE_GROUPS:
		return "close_groups"
	case historyv1.EventType_EVENT_TYPE_OPEN_GROUPS:
		return "open_groups"
	case historyv1.EventType_EVENT_TYPE_CHANGE_NAME:
		return "change_name"
	case historyv1.EventType_EVENT_TYPE_CHANGE_PHOTO:
		return "change_photo"
	case historyv1.EventType_EVENT_TYPE_CHANGE_DESCRIPTION:
		return "change_description"
	case historyv1.EventType_EVENT_TYPE_IMPORT_GROUPS:
		return "import_groups"
	case historyv1.EventType_EVENT_TYPE_SEND_CAMPAIGN:
		return "send_campaign"
	case historyv1.EventType_EVENT_TYPE_GENERATE_IMAGE:
		return "generate_image"
	case historyv1.EventType_EVENT_TYPE_ADD_MEMBER:
		return "add_member"
	case historyv1.EventType_EVENT_TYPE_REMOVE_MEMBER:
		return "remove_member"
	case historyv1.EventType_EVENT_TYPE_PROMOTE_MEMBER:
		return "promote_member"
	case historyv1.EventType_EVENT_TYPE_DEMOTE_MEMBER:
		return "demote_member"
	case historyv1.EventType_EVENT_TYPE_LEAVE_GROUP:
		return "leave_group"
	default:
		return ""
	}
}

// ListDispatchLogs returns dispatch logs for the authenticated user.
func (h *HistoryHandler) ListDispatchLogs(
	ctx context.Context,
	req *connect.Request[historyv1.ListDispatchLogsRequest],
) (*connect.Response[historyv1.ListDispatchLogsResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	// Parse and validate pagination parameters
	limit := int(req.Msg.GetLimit())
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	offset := int(req.Msg.GetOffset())
	if offset < 0 {
		offset = 0
	}

	// Get dispatch logs for the user
	logs, err := h.dispatchLogRepo.GetUserLogs(ctx, user.ID, limit, offset)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get dispatch logs: %w", err))
	}

	// Build a map of group IDs to names for efficient lookup
	groupNameMap := make(map[string]string)
	if h.groupRepo != nil {
		// Collect unique group IDs
		groupIDs := make([]string, 0)
		seen := make(map[string]bool)
		for _, log := range logs {
			if log.GroupID != nil && !seen[*log.GroupID] {
				groupIDs = append(groupIDs, *log.GroupID)
				seen[*log.GroupID] = true
			}
		}

		// Fetch groups if there are any group IDs
		if len(groupIDs) > 0 {
			groups, err := h.groupRepo.GetGroupsByIDs(ctx, user.ID, groupIDs)
			if err == nil {
				for _, g := range groups {
					groupNameMap[g.ID] = g.Name
				}
			}
			// Silently ignore errors - group names are optional enrichment
		}
	}

	// Convert to proto messages
	protoLogs := make([]*historyv1.DispatchLog, len(logs))
	for i, log := range logs {
		protoLogs[i] = dispatchLogToProto(&log, groupNameMap)
	}

	return connect.NewResponse(&historyv1.ListDispatchLogsResponse{
		Logs:    protoLogs,
		HasMore: len(logs) == limit,
	}), nil
}

// dispatchLogToProto converts a dispatch log model to its proto representation.
func dispatchLogToProto(log *models.DispatchLog, groupNameMap map[string]string) *historyv1.DispatchLog {
	if log == nil {
		return nil
	}

	proto := &historyv1.DispatchLog{
		Id:        int32(log.ID),
		UserId:    log.UserID,
		Status:    mapDispatchLogStatus(log.Status),
		CreatedAt: log.CreatedAt,
	}

	if log.AutomationID != nil {
		proto.AutomationId = log.AutomationID
	}
	if log.AutomationName != nil {
		proto.AutomationName = log.AutomationName
	}
	if log.Store != nil {
		proto.Store = log.Store
	}
	if log.GroupID != nil {
		proto.GroupId = log.GroupID
		// Look up group name from the map
		if name, ok := groupNameMap[*log.GroupID]; ok {
			proto.GroupName = &name
		}
	}
	if log.ProductURL != nil {
		proto.ProductUrl = log.ProductURL
	}
	if log.ProductPageURL != nil {
		proto.ProductPageUrl = log.ProductPageURL
	}
	if log.CanonicalKey != nil {
		proto.CanonicalKey = log.CanonicalKey
	}
	if log.Error != nil {
		proto.Error = log.Error
	}
	if log.Type != nil {
		proto.Type = log.Type
	}

	return proto
}

// mapDispatchLogStatus converts a string status to its proto enum.
func mapDispatchLogStatus(status string) historyv1.DispatchLogStatus {
	switch status {
	case models.DispatchLogStatusPending:
		return historyv1.DispatchLogStatus_DISPATCH_LOG_STATUS_PENDING
	case models.DispatchLogStatusQueued:
		return historyv1.DispatchLogStatus_DISPATCH_LOG_STATUS_QUEUED
	case models.DispatchLogStatusSent:
		return historyv1.DispatchLogStatus_DISPATCH_LOG_STATUS_SENT
	case models.DispatchLogStatusFailed:
		return historyv1.DispatchLogStatus_DISPATCH_LOG_STATUS_FAILED
	case models.DispatchLogStatusSkipped:
		return historyv1.DispatchLogStatus_DISPATCH_LOG_STATUS_SKIPPED
	case models.DispatchLogStatusBlocked:
		return historyv1.DispatchLogStatus_DISPATCH_LOG_STATUS_BLOCKED
	default:
		return historyv1.DispatchLogStatus_DISPATCH_LOG_STATUS_UNSPECIFIED
	}
}

// protoStatusToDispatchLogStatus converts a proto enum to string status (for filtering).
func protoStatusToDispatchLogStatus(status historyv1.DispatchLogStatus) string {
	switch status {
	case historyv1.DispatchLogStatus_DISPATCH_LOG_STATUS_PENDING:
		return models.DispatchLogStatusPending
	case historyv1.DispatchLogStatus_DISPATCH_LOG_STATUS_QUEUED:
		return models.DispatchLogStatusQueued
	case historyv1.DispatchLogStatus_DISPATCH_LOG_STATUS_SENT:
		return models.DispatchLogStatusSent
	case historyv1.DispatchLogStatus_DISPATCH_LOG_STATUS_FAILED:
		return models.DispatchLogStatusFailed
	case historyv1.DispatchLogStatus_DISPATCH_LOG_STATUS_SKIPPED:
		return models.DispatchLogStatusSkipped
	case historyv1.DispatchLogStatus_DISPATCH_LOG_STATUS_BLOCKED:
		return models.DispatchLogStatusBlocked
	default:
		return ""
	}
}

// parseDispatchLogID converts a string ID to an integer.
func parseDispatchLogID(id string) (int, error) {
	return strconv.Atoi(id)
}
