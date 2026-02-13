package connectrpc

import (
	"context"
	"fmt"
	"log"

	"automatiza-go/internal/adapters/whatsapp"
	groupsv1 "automatiza-go/internal/gen/groups/v1"
	"automatiza-go/internal/models"
	"automatiza-go/internal/observability"
	"automatiza-go/internal/repository"
	"automatiza-go/internal/workers"

	"connectrpc.com/connect"
	"github.com/getsentry/sentry-go"
	"github.com/hibiken/asynq"
	"go.uber.org/zap"
)

// GroupEventPublisher publishes real-time group change notifications.
// Implemented by SSEHub in the handlers package.
type GroupEventPublisher interface {
	PublishGroupChange(userID string, eventType string, groupID string)
}

// GroupHistoryPublisher publishes real-time history change notifications.
// Implemented by SSEHub in the handlers package.
type GroupHistoryPublisher interface {
	PublishHistoryChange(userID, historyID, status string)
}

// SyncProgressReader reads sync job progress from a persistent store (e.g. Redis).
type SyncProgressReader interface {
	GetSyncProgress(ctx context.Context, jobID string) (map[string]any, error)
}

// GroupsHandler implements the groupsv1connect.GroupsServiceHandler interface.
type GroupsHandler struct {
	groupRepo         repository.GroupRepository
	instanceRepo      repository.InstanceRepository
	actionHistoryRepo repository.ActionHistoryRepository
	waProvider        whatsapp.WhatsAppProvider
	queueClient       *workers.QueueClient
	asynqInspector    *asynq.Inspector
	eventPublisher    GroupEventPublisher
	historyPublisher  GroupHistoryPublisher
	progressReader    SyncProgressReader
	logger            *zap.Logger
}

// NewGroupsHandler creates a new groups handler.
func NewGroupsHandler(
	groupRepo repository.GroupRepository,
	instanceRepo repository.InstanceRepository,
	waProvider whatsapp.WhatsAppProvider,
	queueClient *workers.QueueClient,
	opts ...GroupsHandlerOption,
) *GroupsHandler {
	h := &GroupsHandler{
		groupRepo:    groupRepo,
		instanceRepo: instanceRepo,
		waProvider:   waProvider,
		queueClient:  queueClient,
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// GroupsHandlerOption is a functional option for GroupsHandler.
type GroupsHandlerOption func(*GroupsHandler)

// WithGroupEventPublisher sets the event publisher for real-time notifications.
func WithGroupEventPublisher(pub GroupEventPublisher) GroupsHandlerOption {
	return func(h *GroupsHandler) {
		h.eventPublisher = pub
	}
}

// WithGroupActionHistoryRepo sets the action history repository for tracking group operations.
func WithGroupActionHistoryRepo(repo repository.ActionHistoryRepository) GroupsHandlerOption {
	return func(h *GroupsHandler) {
		h.actionHistoryRepo = repo
	}
}

// WithGroupHistoryPublisher sets the history publisher for real-time history change notifications.
func WithGroupHistoryPublisher(pub GroupHistoryPublisher) GroupsHandlerOption {
	return func(h *GroupsHandler) {
		h.historyPublisher = pub
	}
}

// WithAsynqInspector sets the asynq inspector for checking job status.
func WithAsynqInspector(inspector *asynq.Inspector) GroupsHandlerOption {
	return func(h *GroupsHandler) {
		h.asynqInspector = inspector
	}
}

// WithSyncProgressReader sets the progress reader for real-time sync progress from Redis.
func WithSyncProgressReader(reader SyncProgressReader) GroupsHandlerOption {
	return func(h *GroupsHandler) {
		h.progressReader = reader
	}
}

// WithGroupLogger sets the logger for the groups handler.
func WithGroupLogger(logger *zap.Logger) GroupsHandlerOption {
	return func(h *GroupsHandler) {
		h.logger = logger
	}
}

// NewGroupsHandlerWithAdapter creates a new groups handler with WhatsApp adapter support.
func NewGroupsHandlerWithAdapter(
	groupRepo repository.GroupRepository,
	instanceRepo repository.InstanceRepository,
	actionHistoryRepo repository.ActionHistoryRepository,
	waProvider whatsapp.WhatsAppProvider,
	queueClient *workers.QueueClient,
	asynqInspector *asynq.Inspector,
	opts ...GroupsHandlerOption,
) *GroupsHandler {
	h := &GroupsHandler{
		groupRepo:         groupRepo,
		instanceRepo:      instanceRepo,
		actionHistoryRepo: actionHistoryRepo,
		waProvider:        waProvider,
		queueClient:       queueClient,
		asynqInspector:    asynqInspector,
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// publishEvent safely publishes an event if a publisher is configured.
func (h *GroupsHandler) publishEvent(userID, eventType, groupID string) {
	if h.eventPublisher != nil {
		h.eventPublisher.PublishGroupChange(userID, eventType, groupID)
	}
}

// recordHistory creates an action_history entry and marks it as completed.
// Returns immediately if actionHistoryRepo is not configured.
func (h *GroupsHandler) recordHistory(ctx context.Context, userID, actionType, description string, details map[string]interface{}) {
	if h.actionHistoryRepo == nil {
		log.Printf("[recordHistory] actionHistoryRepo is nil, skipping history for action=%s user=%s", actionType, userID)
		return
	}

	history, err := h.actionHistoryRepo.Create(ctx, userID, actionType, description, details)
	if err != nil {
		log.Printf("[recordHistory] failed to create history entry: action=%s user=%s err=%v", actionType, userID, err)
		return
	}
	if history == nil {
		log.Printf("[recordHistory] history entry is nil after create: action=%s user=%s", actionType, userID)
		return
	}

	if err := h.actionHistoryRepo.Update(ctx, history.ID, map[string]interface{}{
		"target_count":  int32(1),
		"success_count": int32(1),
	}); err != nil {
		log.Printf("[recordHistory] failed to update counts: historyID=%s err=%v", history.ID, err)
	}

	if err := h.actionHistoryRepo.MarkCompleted(ctx, history.ID); err != nil {
		log.Printf("[recordHistory] failed to mark completed: historyID=%s err=%v", history.ID, err)
	}

	if h.historyPublisher != nil {
		h.historyPublisher.PublishHistoryChange(userID, history.ID, "completed")
	}
}

// ListGroups returns all groups for the authenticated user.
// Supports optional pagination: if page or page_size is provided, uses paginated query.
func (h *GroupsHandler) ListGroups(
	ctx context.Context,
	req *connect.Request[groupsv1.ListGroupsRequest],
) (*connect.Response[groupsv1.ListGroupsResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	adminOnly := req.Msg.GetAdminOnly()

	// Check if this is a paginated request
	isPaginated := req.Msg.Page != nil || req.Msg.PageSize != nil || req.Msg.Search != nil

	if isPaginated {
		// Use paginated query
		page := int(req.Msg.GetPage())
		if page < 1 {
			page = 1
		}
		pageSize := int(req.Msg.GetPageSize())
		if pageSize < 1 {
			pageSize = 20
		}
		if pageSize > 100 {
			pageSize = 100
		}
		search := req.Msg.GetSearch()

		result, err := h.groupRepo.GetUserGroupsPaginated(ctx, user.ID, page, pageSize, adminOnly, search)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get groups: %w", err))
		}

		protoGroups := make([]*groupsv1.Group, len(result.Groups))
		for i, g := range result.Groups {
			protoGroups[i] = groupToProto(&g)
		}

		return connect.NewResponse(&groupsv1.ListGroupsResponse{
			Groups:     protoGroups,
			TotalCount: int32(result.TotalCount),
			Page:       int32(result.Page),
			PageSize:   int32(result.PageSize),
			TotalPages: int32(result.TotalPages),
			HasMore:    result.HasMore,
		}), nil
	}

	// Non-paginated (legacy) request - return all groups
	groups, err := h.groupRepo.GetUserGroups(ctx, user.ID, adminOnly)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get groups: %w", err))
	}

	protoGroups := make([]*groupsv1.Group, len(groups))
	for i, g := range groups {
		protoGroups[i] = groupToProto(&g)
	}

	return connect.NewResponse(&groupsv1.ListGroupsResponse{
		Groups:     protoGroups,
		TotalCount: int32(len(groups)),
		Page:       1,
		PageSize:   int32(len(groups)),
		TotalPages: 1,
		HasMore:    false,
	}), nil
}

// GetGroup returns a specific group by ID.
func (h *GroupsHandler) GetGroup(
	ctx context.Context,
	req *connect.Request[groupsv1.GetGroupRequest],
) (*connect.Response[groupsv1.GetGroupResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	group, err := h.groupRepo.GetGroupByID(ctx, req.Msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get group: %w", err))
	}

	if group == nil || group.UserID != user.ID {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("group not found"))
	}

	return connect.NewResponse(&groupsv1.GetGroupResponse{
		Group: groupToProto(group),
	}), nil
}

// GetGroupsByIds returns multiple groups by their IDs.
func (h *GroupsHandler) GetGroupsByIds(
	ctx context.Context,
	req *connect.Request[groupsv1.GetGroupsByIdsRequest],
) (*connect.Response[groupsv1.GetGroupsByIdsResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	groups, err := h.groupRepo.GetGroupsByIDs(ctx, user.ID, req.Msg.GetIds())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get groups: %w", err))
	}

	protoGroups := make([]*groupsv1.Group, len(groups))
	for i, g := range groups {
		protoGroups[i] = groupToProto(&g)
	}

	return connect.NewResponse(&groupsv1.GetGroupsByIdsResponse{
		Groups: protoGroups,
	}), nil
}

// CreateGroup creates a new group record.
func (h *GroupsHandler) CreateGroup(
	ctx context.Context,
	req *connect.Request[groupsv1.CreateGroupRequest],
) (*connect.Response[groupsv1.CreateGroupResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	group := &models.Group{
		WAGroupID:    req.Msg.GetWaGroupId(),
		Name:         req.Msg.GetName(),
		InstanceID:   req.Msg.GetInstanceId(),
		MembersCount: int(req.Msg.GetMembersCount()),
		MemberLimit:  int(req.Msg.GetMemberLimit()),
		Status:       protoStatusToModel(req.Msg.GetStatus()),
		IsAdmin:      req.Msg.GetIsAdmin(),
	}

	if desc := req.Msg.GetDescription(); desc != "" {
		group.Description = &desc
	}
	if code := req.Msg.GetInviteCode(); code != "" {
		group.InviteCode = &code
	}

	createdGroup, err := h.groupRepo.CreateGroup(ctx, user.ID, group)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create group: %w", err))
	}

	h.publishEvent(user.ID, "group.created", createdGroup.ID)

	return connect.NewResponse(&groupsv1.CreateGroupResponse{
		Group: groupToProto(createdGroup),
	}), nil
}

// UpdateGroup updates an existing group.
func (h *GroupsHandler) UpdateGroup(
	ctx context.Context,
	req *connect.Request[groupsv1.UpdateGroupRequest],
) (*connect.Response[groupsv1.UpdateGroupResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	// Verify ownership
	existing, err := h.groupRepo.GetGroupByID(ctx, req.Msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get group: %w", err))
	}
	if existing == nil || existing.UserID != user.ID {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("group not found"))
	}

	// Build updates
	updates := make(map[string]interface{})
	if req.Msg.Name != nil {
		updates["name"] = req.Msg.GetName()
	}
	if req.Msg.Description != nil {
		updates["description"] = req.Msg.GetDescription()
	}
	if req.Msg.MembersCount != nil {
		updates["members_count"] = req.Msg.GetMembersCount()
	}
	if req.Msg.MemberLimit != nil {
		updates["member_limit"] = req.Msg.GetMemberLimit()
	}
	if req.Msg.Status != nil {
		updates["status"] = protoStatusToModel(req.Msg.GetStatus())
	}
	if req.Msg.InviteCode != nil {
		updates["invite_code"] = req.Msg.GetInviteCode()
	}
	if req.Msg.IsAdmin != nil {
		updates["is_admin"] = req.Msg.GetIsAdmin()
	}
	if req.Msg.IsFavorite != nil {
		updates["is_favorite"] = req.Msg.GetIsFavorite()
	}

	if len(updates) > 0 {
		if err := h.groupRepo.UpdateGroup(ctx, req.Msg.GetId(), updates); err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update group: %w", err))
		}
	}

	// Fetch updated group
	updated, err := h.groupRepo.GetGroupByID(ctx, req.Msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get updated group: %w", err))
	}

	h.publishEvent(user.ID, "group.updated", req.Msg.GetId())

	return connect.NewResponse(&groupsv1.UpdateGroupResponse{
		Group: groupToProto(updated),
	}), nil
}

// ToggleGroupFavorite toggles the favorite status of a group.
func (h *GroupsHandler) ToggleGroupFavorite(
	ctx context.Context,
	req *connect.Request[groupsv1.ToggleGroupFavoriteRequest],
) (*connect.Response[groupsv1.ToggleGroupFavoriteResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	// Verify ownership
	existing, err := h.groupRepo.GetGroupByID(ctx, req.Msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get group: %w", err))
	}
	if existing == nil || existing.UserID != user.ID {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("group not found"))
	}

	updates := map[string]interface{}{
		"is_favorite": req.Msg.GetIsFavorite(),
	}

	if err := h.groupRepo.UpdateGroup(ctx, req.Msg.GetId(), updates); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update favorite status: %w", err))
	}

	// Fetch updated group
	updated, err := h.groupRepo.GetGroupByID(ctx, req.Msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get updated group: %w", err))
	}

	return connect.NewResponse(&groupsv1.ToggleGroupFavoriteResponse{
		Group: groupToProto(updated),
	}), nil
}

// SyncGroups starts an asynchronous group synchronization job.
func (h *GroupsHandler) SyncGroups(
	ctx context.Context,
	req *connect.Request[groupsv1.SyncGroupsRequest],
) (*connect.Response[groupsv1.SyncGroupsResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	instanceName := req.Msg.GetInstanceName()

	// Verify instance belongs to user
	instance, err := h.instanceRepo.GetInstanceByUserAndName(ctx, user.ID, instanceName)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get instance: %w", err))
	}
	if instance == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("instance not found"))
	}

	// Enqueue sync job
	taskInfo, err := h.queueClient.EnqueueGroupsSync(ctx, &workers.GroupsSyncJobData{
		UserID:       user.ID,
		InstanceName: instanceName,
		InstanceID:   instance.ID,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to enqueue sync job: %w", err))
	}

	jobID := taskInfo.ID
	return connect.NewResponse(&groupsv1.SyncGroupsResponse{
		Success: true,
		JobId:   &jobID,
	}), nil
}

// DeleteGroup deletes a group record.
func (h *GroupsHandler) DeleteGroup(
	ctx context.Context,
	req *connect.Request[groupsv1.DeleteGroupRequest],
) (*connect.Response[groupsv1.DeleteGroupResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	if err := h.groupRepo.DeleteGroup(ctx, req.Msg.GetId(), user.ID); err != nil {
		if err.Error() == "group not found" {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("group not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete group: %w", err))
	}

	h.publishEvent(user.ID, "group.deleted", req.Msg.GetId())

	return connect.NewResponse(&groupsv1.DeleteGroupResponse{}), nil
}

// BulkUpdateGroups updates multiple groups at once.
func (h *GroupsHandler) BulkUpdateGroups(
	ctx context.Context,
	req *connect.Request[groupsv1.BulkUpdateGroupsRequest],
) (*connect.Response[groupsv1.BulkUpdateGroupsResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	updates := make(map[string]interface{})
	if req.Msg.Status != nil {
		updates["status"] = protoStatusToModel(req.Msg.GetStatus())
	}
	if req.Msg.Name != nil {
		updates["name"] = req.Msg.GetName()
	}
	if req.Msg.IsFavorite != nil {
		updates["is_favorite"] = req.Msg.GetIsFavorite()
	}

	count, err := h.groupRepo.BulkUpdateGroups(ctx, user.ID, req.Msg.GetIds(), updates)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to bulk update groups: %w", err))
	}

	return connect.NewResponse(&groupsv1.BulkUpdateGroupsResponse{
		UpdatedCount: int32(count),
	}), nil
}

// Helper functions

func groupToProto(g *models.Group) *groupsv1.Group {
	if g == nil {
		return nil
	}

	proto := &groupsv1.Group{
		Id:           g.ID,
		UserId:       g.UserID,
		WaGroupId:    g.WAGroupID,
		Name:         g.Name,
		MembersCount: int32(g.MembersCount),
		MemberLimit:  int32(g.MemberLimit),
		Status:       modelStatusToProto(g.Status),
		IsAdmin:      g.IsAdmin,
		IsFavorite:   g.IsFavorite,
		CreatedAt:    g.CreatedAt,
		UpdatedAt:    g.UpdatedAt,
	}

	if g.InstanceID != "" {
		proto.InstanceId = &g.InstanceID
	}
	if g.Description != nil {
		proto.Description = g.Description
	}
	if g.InviteCode != nil {
		proto.InviteCode = g.InviteCode
	}

	return proto
}

func modelStatusToProto(status string) groupsv1.GroupStatus {
	switch status {
	case models.GroupStatusOpen:
		return groupsv1.GroupStatus_GROUP_STATUS_OPEN
	case models.GroupStatusClosed:
		return groupsv1.GroupStatus_GROUP_STATUS_CLOSED
	case models.GroupStatusFull:
		return groupsv1.GroupStatus_GROUP_STATUS_FULL
	default:
		return groupsv1.GroupStatus_GROUP_STATUS_UNSPECIFIED
	}
}

func protoStatusToModel(status groupsv1.GroupStatus) string {
	switch status {
	case groupsv1.GroupStatus_GROUP_STATUS_OPEN:
		return models.GroupStatusOpen
	case groupsv1.GroupStatus_GROUP_STATUS_CLOSED:
		return models.GroupStatusClosed
	case groupsv1.GroupStatus_GROUP_STATUS_FULL:
		return models.GroupStatusFull
	default:
		return ""
	}
}

// GetGroupInviteCode retrieves the invite code for a group.
func (h *GroupsHandler) GetGroupInviteCode(
	ctx context.Context,
	req *connect.Request[groupsv1.GetGroupInviteCodeRequest],
) (*connect.Response[groupsv1.GetGroupInviteCodeResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	instanceName := req.Msg.GetInstanceName()
	groupID := req.Msg.GetGroupId()

	// Verify instance belongs to user
	instance, err := h.instanceRepo.GetInstanceByUserAndName(ctx, user.ID, instanceName)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get instance: %w", err))
	}
	if instance == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("instance not found"))
	}

	// Get invite code from WhatsApp provider
	if h.waProvider == nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("WhatsApp provider not configured"))
	}

	inviteCode, err := h.waProvider.GetGroupInviteCode(ctx, instanceName, groupID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get invite code: %w", err))
	}

	inviteURL := fmt.Sprintf("https://chat.whatsapp.com/%s", inviteCode)

	return connect.NewResponse(&groupsv1.GetGroupInviteCodeResponse{
		InviteCode: inviteCode,
		InviteUrl:  inviteURL,
	}), nil
}

// GetSyncStatus returns the status of a sync job.
func (h *GroupsHandler) GetSyncStatus(
	ctx context.Context,
	req *connect.Request[groupsv1.GetSyncStatusRequest],
) (*connect.Response[groupsv1.GetSyncStatusResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	jobID := req.Msg.GetJobId()

	log.Printf("[GetSyncStatus] called with jobID=%s", jobID)

	// 1. Try to read real-time progress from Redis (persisted by the worker)
	if h.progressReader != nil {
		progressData, err := h.progressReader.GetSyncProgress(ctx, jobID)
		if err == nil && progressData != nil {
			progress := int32(0)
			if p, ok := progressData["progress"].(float64); ok {
				progress = int32(p)
			}
			status := "active"
			if s, ok := progressData["status"].(string); ok && s != "" {
				status = s
			}
			total := int32(0)
			if t, ok := progressData["total"].(float64); ok {
				total = int32(t)
			}
			synced := int32(0)
			if c, ok := progressData["current"].(float64); ok {
				synced = int32(c)
			}
			log.Printf("[GetSyncStatus] Redis HIT jobID=%s status=%s progress=%d total=%d synced=%d", jobID, status, progress, total, synced)
			return connect.NewResponse(&groupsv1.GetSyncStatusResponse{
				Status:       status,
				Progress:     progress,
				TotalGroups:  &total,
				GroupsSynced: synced,
			}), nil
		}
		if err != nil {
			log.Printf("[GetSyncStatus] Redis MISS/error jobID=%s err=%v", jobID, err)
		}
	}

	// 2. Fallback: use asynq inspector (no detailed progress)
	if h.asynqInspector != nil {
		info, err := h.asynqInspector.GetTaskInfo("default", jobID)
		if err != nil {
			log.Printf("[GetSyncStatus] Asynq fallback failed jobID=%s err=%v", jobID, err)
			return connect.NewResponse(&groupsv1.GetSyncStatusResponse{
				Status:   "unknown",
				Progress: 0,
			}), nil
		}

		var status string
		switch info.State.String() {
		case "pending":
			status = "pending"
		case "active":
			status = "active"
		case "completed":
			status = "completed"
		default:
			status = "failed"
		}

		log.Printf("[GetSyncStatus] Asynq fallback jobID=%s state=%s", jobID, status)

		return connect.NewResponse(&groupsv1.GetSyncStatusResponse{
			Status:   status,
			Progress: 0,
		}), nil
	}

	log.Printf("[GetSyncStatus] No progressReader or inspector available, returning unknown")

	// Without inspector or progress reader, return unknown status
	return connect.NewResponse(&groupsv1.GetSyncStatusResponse{
		Status:   "unknown",
		Progress: 0,
	}), nil
}

// SendGroupMessage sends a text message to a group.
func (h *GroupsHandler) SendGroupMessage(
	ctx context.Context,
	req *connect.Request[groupsv1.SendGroupMessageRequest],
) (*connect.Response[groupsv1.SendGroupMessageResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	// Verify instance belongs to user
	instance, err := h.instanceRepo.GetInstanceByUserAndName(ctx, user.ID, req.Msg.GetInstanceName())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get instance: %w", err))
	}
	if instance == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("instance not found"))
	}

	if h.waProvider == nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("WhatsApp provider not configured"))
	}

	result, err := h.waProvider.SendText(ctx, whatsapp.SendTextInput{
		SessionName: req.Msg.GetInstanceName(),
		ChatID:      req.Msg.GetGroupId(),
		Text:        req.Msg.GetMessage(),
		Mentions:    req.Msg.GetMentions(),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to send message: %w", err))
	}

	messageID := ""
	if result != nil {
		messageID = result.ID
	}

	h.recordHistory(ctx, user.ID, models.ActionTypeSendMessage,
		"Enviar mensagem de texto",
		map[string]interface{}{"group_id": req.Msg.GetGroupId()})

	return connect.NewResponse(&groupsv1.SendGroupMessageResponse{
		MessageId: messageID,
		ActionId:  "",
	}), nil
}

// SendGroupAudio sends an audio message to a group.
func (h *GroupsHandler) SendGroupAudio(
	ctx context.Context,
	req *connect.Request[groupsv1.SendGroupAudioRequest],
) (*connect.Response[groupsv1.SendGroupAudioResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	// Verify instance belongs to user
	instance, err := h.instanceRepo.GetInstanceByUserAndName(ctx, user.ID, req.Msg.GetInstanceName())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get instance: %w", err))
	}
	if instance == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("instance not found"))
	}

	if h.waProvider == nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("WhatsApp provider not configured"))
	}

	mimetype := req.Msg.GetMimetype()
	if mimetype == "" {
		mimetype = "audio/ogg"
	}

	audioData := req.Msg.GetAudio()
	audioPreview := audioData
	if len(audioPreview) > 80 {
		audioPreview = audioPreview[:40] + "..." + audioPreview[len(audioPreview)-40:]
	}
	log.Printf("[DEBUG SendGroupAudio] instance=%s group=%s mimetype=%s ptt=%v audioLen=%d audioPreview=%s",
		req.Msg.GetInstanceName(), req.Msg.GetGroupId(), mimetype, req.Msg.GetPtt(), len(audioData), audioPreview)

	result, err := h.waProvider.SendVoice(ctx, whatsapp.SendVoiceInput{
		SessionName: req.Msg.GetInstanceName(),
		ChatID:      req.Msg.GetGroupId(),
		AudioBase64: audioData,
		Mimetype:    mimetype,
	})
	if err != nil {
		log.Printf("[DEBUG SendGroupAudio] ERROR: %v", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to send audio: %w", err))
	}

	messageID := ""
	if result != nil {
		messageID = result.ID
	}

	log.Printf("[DEBUG SendGroupAudio] SUCCESS messageId=%s", messageID)

	h.recordHistory(ctx, user.ID, models.ActionTypeSendAudio,
		"Enviar audio",
		map[string]interface{}{"group_id": req.Msg.GetGroupId()})

	return connect.NewResponse(&groupsv1.SendGroupAudioResponse{
		MessageId: messageID,
		ActionId:  "",
	}), nil
}

// SendGroupImage sends an image to a group.
func (h *GroupsHandler) SendGroupImage(
	ctx context.Context,
	req *connect.Request[groupsv1.SendGroupImageRequest],
) (*connect.Response[groupsv1.SendGroupImageResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	// Verify instance belongs to user
	instance, err := h.instanceRepo.GetInstanceByUserAndName(ctx, user.ID, req.Msg.GetInstanceName())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get instance: %w", err))
	}
	if instance == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("instance not found"))
	}

	if h.waProvider == nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("WhatsApp provider not configured"))
	}

	caption := ""
	if req.Msg.Caption != nil {
		caption = req.Msg.GetCaption()
	}

	result, err := h.waProvider.SendImage(ctx, whatsapp.SendImageInput{
		SessionName: req.Msg.GetInstanceName(),
		ChatID:      req.Msg.GetGroupId(),
		Caption:     caption,
		ImageURL:    req.Msg.GetImage(),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to send image: %w", err))
	}

	messageID := ""
	if result != nil {
		messageID = result.ID
	}

	h.recordHistory(ctx, user.ID, models.ActionTypeSendImage,
		"Enviar imagem",
		map[string]interface{}{"group_id": req.Msg.GetGroupId()})

	return connect.NewResponse(&groupsv1.SendGroupImageResponse{
		MessageId: messageID,
		ActionId:  "",
	}), nil
}

// SendGroupPoll sends a poll to a group.
func (h *GroupsHandler) SendGroupPoll(
	ctx context.Context,
	req *connect.Request[groupsv1.SendGroupPollRequest],
) (*connect.Response[groupsv1.SendGroupPollResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	// Verify instance belongs to user
	instance, err := h.instanceRepo.GetInstanceByUserAndName(ctx, user.ID, req.Msg.GetInstanceName())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get instance: %w", err))
	}
	if instance == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("instance not found"))
	}

	if h.waProvider == nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("WhatsApp provider not configured"))
	}

	selectableCount := 1
	if req.Msg.GetMultipleAnswers() {
		selectableCount = 0 // 0 means unlimited
	}

	result, err := h.waProvider.SendPoll(ctx, whatsapp.SendPollInput{
		SessionName:     req.Msg.GetInstanceName(),
		ChatID:          req.Msg.GetGroupId(),
		Question:        req.Msg.GetName(),
		Options:         req.Msg.GetOptions(),
		SelectableCount: selectableCount,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to send poll: %w", err))
	}

	messageID := ""
	if result != nil {
		messageID = result.ID
	}

	h.recordHistory(ctx, user.ID, models.ActionTypeSendPoll,
		"Enviar enquete",
		map[string]interface{}{"group_id": req.Msg.GetGroupId()})

	return connect.NewResponse(&groupsv1.SendGroupPollResponse{
		MessageId: messageID,
		ActionId:  "",
	}), nil
}

// UpdateGroupSubject updates the group subject/name.
func (h *GroupsHandler) UpdateGroupSubject(
	ctx context.Context,
	req *connect.Request[groupsv1.UpdateGroupSubjectRequest],
) (*connect.Response[groupsv1.UpdateGroupSubjectResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	// Verify instance belongs to user
	instance, err := h.instanceRepo.GetInstanceByUserAndName(ctx, user.ID, req.Msg.GetInstanceName())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get instance: %w", err))
	}
	if instance == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("instance not found"))
	}

	if h.waProvider == nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("WhatsApp provider not configured"))
	}

	// Let WAHA enforce admin permission — the local is_admin cache can be stale
	err = h.waProvider.UpdateGroupSubject(ctx, whatsapp.UpdateGroupSubjectInput{
		SessionName: req.Msg.GetInstanceName(),
		GroupID:     req.Msg.GetGroupId(),
		Subject:     req.Msg.GetSubject(),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update group subject: %w", err))
	}

	h.recordHistory(ctx, user.ID, models.ActionTypeChangeName,
		fmt.Sprintf("Alterar nome do grupo: %s", req.Msg.GetSubject()),
		map[string]interface{}{
			"group_id":    req.Msg.GetGroupId(),
			"new_subject": req.Msg.GetSubject(),
		})

	return connect.NewResponse(&groupsv1.UpdateGroupSubjectResponse{
		Success: true,
	}), nil
}

// UpdateGroupDescription updates the group description.
func (h *GroupsHandler) UpdateGroupDescription(
	ctx context.Context,
	req *connect.Request[groupsv1.UpdateGroupDescriptionRequest],
) (*connect.Response[groupsv1.UpdateGroupDescriptionResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	// Verify instance belongs to user
	instance, err := h.instanceRepo.GetInstanceByUserAndName(ctx, user.ID, req.Msg.GetInstanceName())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get instance: %w", err))
	}
	if instance == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("instance not found"))
	}

	if h.waProvider == nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("WhatsApp provider not configured"))
	}

	// Let WAHA enforce admin permission — the local is_admin cache can be stale
	err = h.waProvider.UpdateGroupDescription(ctx, whatsapp.UpdateGroupDescriptionInput{
		SessionName: req.Msg.GetInstanceName(),
		GroupID:     req.Msg.GetGroupId(),
		Description: req.Msg.GetDescription(),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update group description: %w", err))
	}

	h.recordHistory(ctx, user.ID, models.ActionTypeChangeDescription,
		"Alterar descricao do grupo",
		map[string]interface{}{
			"group_id": req.Msg.GetGroupId(),
		})

	return connect.NewResponse(&groupsv1.UpdateGroupDescriptionResponse{
		Success: true,
	}), nil
}

// UpdateGroupPicture updates the group picture.
func (h *GroupsHandler) UpdateGroupPicture(
	ctx context.Context,
	req *connect.Request[groupsv1.UpdateGroupPictureRequest],
) (*connect.Response[groupsv1.UpdateGroupPictureResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	// Verify instance belongs to user
	instance, err := h.instanceRepo.GetInstanceByUserAndName(ctx, user.ID, req.Msg.GetInstanceName())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get instance: %w", err))
	}
	if instance == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("instance not found"))
	}

	if h.waProvider == nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("WhatsApp provider not configured"))
	}

	// Handle either URL or base64 data
	var imageData string
	if url := req.Msg.GetUrl(); url != "" {
		imageData = url
	} else if base64Data := req.Msg.GetBase64Data(); base64Data != "" {
		imageData = base64Data
	} else {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("either url or base64_data must be provided"))
	}

	err = h.waProvider.UpdateGroupPicture(ctx, whatsapp.UpdateGroupPictureInput{
		SessionName: req.Msg.GetInstanceName(),
		GroupID:     req.Msg.GetGroupId(),
		ImageBase64: imageData,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update group picture: %w", err))
	}

	h.recordHistory(ctx, user.ID, models.ActionTypeChangePhoto,
		"Alterar foto do grupo",
		map[string]interface{}{
			"group_id": req.Msg.GetGroupId(),
		})

	return connect.NewResponse(&groupsv1.UpdateGroupPictureResponse{
		Success: true,
	}), nil
}

// UpdateGroupSettings updates group settings (e.g., announcement mode).
func (h *GroupsHandler) UpdateGroupSettings(
	ctx context.Context,
	req *connect.Request[groupsv1.UpdateGroupSettingsRequest],
) (*connect.Response[groupsv1.UpdateGroupSettingsResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	// Verify instance belongs to user
	instance, err := h.instanceRepo.GetInstanceByUserAndName(ctx, user.ID, req.Msg.GetInstanceName())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get instance: %w", err))
	}
	if instance == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("instance not found"))
	}

	if h.waProvider == nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("WhatsApp provider not configured"))
	}

	// Let WAHA enforce admin permission — the local is_admin cache can be stale
	err = h.waProvider.SetGroupAnnouncement(ctx, whatsapp.SetGroupAnnouncementInput{
		SessionName: req.Msg.GetInstanceName(),
		GroupID:     req.Msg.GetGroupId(),
		Announce:    req.Msg.GetMessagesAdminOnly(),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update group settings: %w", err))
	}

	if req.Msg.GetMessagesAdminOnly() {
		h.recordHistory(ctx, user.ID, models.ActionTypeCloseGroups,
			"Fechar grupo",
			map[string]interface{}{"group_id": req.Msg.GetGroupId()})
	} else {
		h.recordHistory(ctx, user.ID, models.ActionTypeOpenGroups,
			"Abrir grupo",
			map[string]interface{}{"group_id": req.Msg.GetGroupId()})
	}

	return connect.NewResponse(&groupsv1.UpdateGroupSettingsResponse{
		Success: true,
	}), nil
}

// GetGroupParticipants retrieves the participants of a group.
func (h *GroupsHandler) GetGroupParticipants(
	ctx context.Context,
	req *connect.Request[groupsv1.GetGroupParticipantsRequest],
) (*connect.Response[groupsv1.GetGroupParticipantsResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	instanceName := req.Msg.GetInstanceName()
	groupID := req.Msg.GetGroupId()

	// Verify instance belongs to user
	instance, err := h.instanceRepo.GetInstanceByUserAndName(ctx, user.ID, instanceName)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get instance: %w", err))
	}
	if instance == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("instance not found"))
	}

	// Get participants from WhatsApp provider
	if h.waProvider == nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("WhatsApp provider not configured"))
	}

	participants, err := h.waProvider.GetGroupParticipants(ctx, instanceName, groupID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get group participants: %w", err))
	}

	protoParticipants := make([]*groupsv1.GroupParticipant, len(participants))
	for i, p := range participants {
		protoParticipants[i] = &groupsv1.GroupParticipant{
			Id:           p.ID,
			IsAdmin:      p.IsAdmin,
			IsSuperAdmin: p.IsSuperAdmin,
			PhoneNumber:  p.PhoneNumber,
		}
	}

	return connect.NewResponse(&groupsv1.GetGroupParticipantsResponse{
		Participants: protoParticipants,
	}), nil
}

// =============================================================================
// Member Management Operations
// =============================================================================

// AddGroupMember adds members to a group.
func (h *GroupsHandler) AddGroupMember(
	ctx context.Context,
	req *connect.Request[groupsv1.AddGroupMemberRequest],
) (*connect.Response[groupsv1.AddGroupMemberResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	instanceName := req.Msg.GetInstanceName()
	groupID := req.Msg.GetGroupId()

	// Verify instance belongs to user
	instance, err := h.instanceRepo.GetInstanceByUserAndName(ctx, user.ID, instanceName)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get instance: %w", err))
	}
	if instance == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("instance not found"))
	}

	if h.waProvider == nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("WhatsApp provider not configured"))
	}

	// Let WAHA enforce admin permission — the local is_admin cache can be stale
	result, err := h.waProvider.AddGroupMembers(ctx, whatsapp.AddGroupMembersInput{
		SessionName:  instanceName,
		GroupID:      groupID,
		PhoneNumbers: req.Msg.GetPhoneNumbers(),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to add group members: %w", err))
	}

	h.publishEvent(user.ID, "group.member.changed", groupID)

	h.recordHistory(ctx, user.ID, models.ActionTypeAddMember,
		"Adicionar membros ao grupo",
		map[string]interface{}{
			"group_id":     groupID,
			"added_count":  result.SuccessCount,
			"failed_count": len(result.FailedNumbers),
		})

	return connect.NewResponse(&groupsv1.AddGroupMemberResponse{
		AddedCount:    int32(result.SuccessCount),
		FailedNumbers: result.FailedNumbers,
	}), nil
}

// RemoveGroupMember removes members from a group.
func (h *GroupsHandler) RemoveGroupMember(
	ctx context.Context,
	req *connect.Request[groupsv1.RemoveGroupMemberRequest],
) (*connect.Response[groupsv1.RemoveGroupMemberResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	instanceName := req.Msg.GetInstanceName()
	groupID := req.Msg.GetGroupId()

	// Verify instance belongs to user
	instance, err := h.instanceRepo.GetInstanceByUserAndName(ctx, user.ID, instanceName)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get instance: %w", err))
	}
	if instance == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("instance not found"))
	}

	if h.waProvider == nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("WhatsApp provider not configured"))
	}

	// Let WAHA enforce admin permission — the local is_admin cache can be stale
	result, err := h.waProvider.RemoveGroupMembers(ctx, whatsapp.RemoveGroupMembersInput{
		SessionName:  instanceName,
		GroupID:      groupID,
		PhoneNumbers: req.Msg.GetPhoneNumbers(),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to remove group members: %w", err))
	}

	h.publishEvent(user.ID, "group.member.changed", groupID)

	h.recordHistory(ctx, user.ID, models.ActionTypeRemoveMember,
		"Remover membros do grupo",
		map[string]interface{}{
			"group_id":      groupID,
			"removed_count": result.SuccessCount,
			"failed_count":  len(result.FailedNumbers),
		})

	return connect.NewResponse(&groupsv1.RemoveGroupMemberResponse{
		RemovedCount:  int32(result.SuccessCount),
		FailedNumbers: result.FailedNumbers,
	}), nil
}

// LeaveGroup makes the authenticated user leave a group.
func (h *GroupsHandler) LeaveGroup(
	ctx context.Context,
	req *connect.Request[groupsv1.LeaveGroupRequest],
) (*connect.Response[groupsv1.LeaveGroupResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	instanceName := req.Msg.GetInstanceName()
	groupID := req.Msg.GetGroupId()

	// Verify instance belongs to user
	instance, err := h.instanceRepo.GetInstanceByUserAndName(ctx, user.ID, instanceName)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get instance: %w", err))
	}
	if instance == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("instance not found"))
	}

	if h.waProvider == nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("WhatsApp provider not configured"))
	}

	err = h.waProvider.LeaveGroup(ctx, whatsapp.LeaveGroupInput{
		SessionName: instanceName,
		GroupID:     groupID,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to leave group: %w", err))
	}

	// Remove group from database after leaving
	group, groupErr := h.groupRepo.GetGroupByWAGroupID(ctx, user.ID, groupID)
	if groupErr == nil && group != nil {
		if deleteErr := h.groupRepo.DeleteGroup(ctx, group.ID, user.ID); deleteErr != nil {
			logger := h.logger
			if logger == nil {
				logger = zap.NewNop()
			}
			observability.LogAndCapture(logger, sentry.LevelWarning, "failed to delete group after leaving", deleteErr, map[string]string{
				"module": "groups", "operation": "delete_after_leave", "group_id": group.ID,
			})
		}
	}

	h.recordHistory(ctx, user.ID, models.ActionTypeLeaveGroup,
		"Sair do grupo",
		map[string]interface{}{"group_id": groupID})

	return connect.NewResponse(&groupsv1.LeaveGroupResponse{
		Success: true,
	}), nil
}

// PromoteMember promotes a group member to admin.
func (h *GroupsHandler) PromoteMember(
	ctx context.Context,
	req *connect.Request[groupsv1.PromoteMemberRequest],
) (*connect.Response[groupsv1.PromoteMemberResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	instanceName := req.Msg.GetInstanceName()
	groupID := req.Msg.GetGroupId()

	// Verify instance belongs to user
	instance, err := h.instanceRepo.GetInstanceByUserAndName(ctx, user.ID, instanceName)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get instance: %w", err))
	}
	if instance == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("instance not found"))
	}

	if h.waProvider == nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("WhatsApp provider not configured"))
	}

	// Let WAHA enforce admin permission — the local is_admin cache can be stale
	err = h.waProvider.PromoteMember(ctx, whatsapp.PromoteMemberInput{
		SessionName: instanceName,
		GroupID:     groupID,
		PhoneNumber: req.Msg.GetPhoneNumber(),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to promote member: %w", err))
	}

	h.recordHistory(ctx, user.ID, models.ActionTypePromoteMember,
		"Promover membro a admin",
		map[string]interface{}{
			"group_id":     groupID,
			"phone_number": req.Msg.GetPhoneNumber(),
		})

	return connect.NewResponse(&groupsv1.PromoteMemberResponse{
		Success: true,
	}), nil
}

// DemoteMember demotes a group admin to regular member.
func (h *GroupsHandler) DemoteMember(
	ctx context.Context,
	req *connect.Request[groupsv1.DemoteMemberRequest],
) (*connect.Response[groupsv1.DemoteMemberResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	instanceName := req.Msg.GetInstanceName()
	groupID := req.Msg.GetGroupId()

	// Verify instance belongs to user
	instance, err := h.instanceRepo.GetInstanceByUserAndName(ctx, user.ID, instanceName)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get instance: %w", err))
	}
	if instance == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("instance not found"))
	}

	if h.waProvider == nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("WhatsApp provider not configured"))
	}

	// Let WAHA enforce admin permission — the local is_admin cache can be stale
	err = h.waProvider.DemoteMember(ctx, whatsapp.DemoteMemberInput{
		SessionName: instanceName,
		GroupID:     groupID,
		PhoneNumber: req.Msg.GetPhoneNumber(),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to demote member: %w", err))
	}

	h.recordHistory(ctx, user.ID, models.ActionTypeDemoteMember,
		"Rebaixar admin a membro",
		map[string]interface{}{
			"group_id":     groupID,
			"phone_number": req.Msg.GetPhoneNumber(),
		})

	return connect.NewResponse(&groupsv1.DemoteMemberResponse{
		Success: true,
	}), nil
}
