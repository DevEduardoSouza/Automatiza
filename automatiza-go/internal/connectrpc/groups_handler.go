package connectrpc

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"

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
	"google.golang.org/protobuf/proto"
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
	groupMemberRepo   repository.GroupMemberRepository
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

	// Parse the optional chip filter. Empty string and unparseable values
	// fall through as 0, which the repo treats as "no filter".
	instanceID := 0
	if req.Msg.InstanceId != nil && *req.Msg.InstanceId != "" {
		_, _ = fmt.Sscanf(*req.Msg.InstanceId, "%d", &instanceID)
	}

	// Check if this is a paginated request
	isPaginated := req.Msg.Page != nil || req.Msg.PageSize != nil || req.Msg.Search != nil || req.Msg.InstanceId != nil

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

		result, err := h.groupRepo.GetUserGroupsPaginated(ctx, user.ID, page, pageSize, adminOnly, search, instanceID)
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
		Id:                       g.ID,
		UserId:                   g.UserID,
		WaGroupId:                g.WAGroupID,
		Name:                     g.Name,
		MembersCount:             int32(g.MembersCount),
		MemberLimit:              int32(g.MemberLimit),
		Status:                   modelStatusToProto(g.Status),
		IsAdmin:                  g.IsAdmin,
		IsFavorite:               g.IsFavorite,
		CreatedAt:                g.CreatedAt,
		UpdatedAt:                g.UpdatedAt,
		ParticipatingInstanceIds: g.ParticipatingInstanceIDs,
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

// maxParticipantChipFallbacks bounds how many extra chips GetGroupParticipants
// will try when the requested chip can't resolve the group, so a user with many
// chips can't turn one modal open into a WAHA request storm on the error path.
const maxParticipantChipFallbacks = 3

// connectCodeFromProviderError maps a WhatsApp provider error to the closest
// Connect status code, so the client sees an accurate, actionable error instead
// of a blanket Internal (500). Non-provider errors (repo/network wrapping that
// never reached a ProviderError) stay Internal.
func connectCodeFromProviderError(err error) connect.Code {
	var pErr *whatsapp.ProviderError
	if !errors.As(err, &pErr) {
		return connect.CodeInternal
	}
	switch pErr.Code {
	case whatsapp.ErrCodeSessionNotFound, whatsapp.ErrCodeGroupNotFound:
		return connect.CodeNotFound
	case whatsapp.ErrCodePermissionDenied:
		return connect.CodePermissionDenied
	case whatsapp.ErrCodeInvalidInput:
		return connect.CodeInvalidArgument
	case whatsapp.ErrCodeRateLimited:
		return connect.CodeResourceExhausted
	case whatsapp.ErrCodeSessionNotConnected,
		whatsapp.ErrCodeCircuitBreakerOpen,
		whatsapp.ErrCodeNetworkError:
		return connect.CodeUnavailable
	case whatsapp.ErrCodeTimeout:
		return connect.CodeDeadlineExceeded
	default:
		// Includes ErrCodeProviderError: an opaque upstream failure the client
		// can't act on, so Internal is the honest mapping.
		return connect.CodeInternal
	}
}

// waError wraps a WhatsApp-provider error as a Connect error whose code reflects
// the underlying provider error code (see connectCodeFromProviderError).
func waError(wrap string, err error) *connect.Error {
	return connect.NewError(connectCodeFromProviderError(err), fmt.Errorf("%s: %w", wrap, err))
}

// wrongChipError reports whether a provider error suggests the queried session
// simply isn't a member of the group (so another chip might succeed), rather
// than a whole-WAHA problem (rate limit, network, circuit breaker, timeout)
// where retrying on a different chip is pointless. The dominant real-world
// case — GOWS answering GetGroupInfo with HTTP 500 "2 UNKNOWN: not found" —
// surfaces as ErrCodeProviderError, so that code is included.
func wrongChipError(err error) bool {
	var pErr *whatsapp.ProviderError
	if !errors.As(err, &pErr) {
		return false
	}
	switch pErr.Code {
	case whatsapp.ErrCodeProviderError,
		whatsapp.ErrCodeSessionNotFound,
		whatsapp.ErrCodeGroupNotFound,
		whatsapp.ErrCodeSessionNotConnected:
		return true
	default:
		return false
	}
}

// fallbackChipsForGroup returns up to maxParticipantChipFallbacks session names
// of chips (other than `exclude`) that participate in waGroupID, connected chips
// first. Best-effort: returns nil if the group or the user's instances can't be
// loaded, so the caller just surfaces the original error.
func (h *GroupsHandler) fallbackChipsForGroup(ctx context.Context, userID, waGroupID, exclude string) []string {
	group, err := h.groupRepo.GetGroupByWAGroupID(ctx, userID, waGroupID)
	if err != nil || group == nil {
		return nil
	}

	// Participating chip DB ids, owner first, deduped.
	wantIDs := make([]string, 0, len(group.ParticipatingInstanceIDs)+1)
	seen := make(map[string]struct{})
	addID := func(id string) {
		if id == "" {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		wantIDs = append(wantIDs, id)
	}
	addID(group.InstanceID)
	for _, id := range group.ParticipatingInstanceIDs {
		addID(id)
	}
	if len(wantIDs) == 0 {
		return nil
	}

	instances, err := h.instanceRepo.GetUserInstances(ctx, userID)
	if err != nil {
		return nil
	}
	byID := make(map[string]models.Instance, len(instances))
	for _, inst := range instances {
		byID[strconv.Itoa(inst.ID)] = inst
	}

	var connected, others []string
	for _, id := range wantIDs {
		inst, ok := byID[id]
		if !ok || inst.InstanceID == "" || inst.InstanceID == exclude {
			continue
		}
		if inst.Status == models.InstanceStatusConnected {
			connected = append(connected, inst.InstanceID)
		} else {
			others = append(others, inst.InstanceID)
		}
	}
	sessions := append(connected, others...)
	if len(sessions) > maxParticipantChipFallbacks {
		sessions = sessions[:maxParticipantChipFallbacks]
	}
	return sessions
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
		return nil, waError("failed to get invite code", err)
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
		return nil, waError("failed to send message", err)
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
		return nil, waError("failed to send audio", err)
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
		return nil, waError("failed to send image", err)
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
		return nil, waError("failed to send poll", err)
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
		return nil, waError("failed to update group subject", err)
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
		return nil, waError("failed to update group description", err)
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
		return nil, waError("failed to update group picture", err)
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
		return nil, waError("failed to update group settings", err)
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
	if err != nil && wrongChipError(err) {
		// The requested chip can't resolve the group — GOWS answers
		// GetGroupInfo with HTTP 500 "not found" for any session that isn't a
		// member. Fall back to the group's other participating chips so a
		// stale/disconnected owner chip doesn't surface as a hard 500 in the
		// members modal. The frontend already picks the right chip in the
		// common case (use-groups-members.ts); this is defense in depth.
		for _, session := range h.fallbackChipsForGroup(ctx, user.ID, groupID, instanceName) {
			p, ferr := h.waProvider.GetGroupParticipants(ctx, session, groupID)
			if ferr == nil {
				if h.logger != nil {
					h.logger.Info("group participants resolved via fallback chip",
						zap.String("group_id", groupID),
						zap.String("requested_session", instanceName),
						zap.String("used_session", session))
				}
				participants, err = p, nil
				break
			}
			err = ferr
		}
	}
	if err != nil {
		return nil, waError("failed to get group participants", err)
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
		return nil, waError("failed to add group members", err)
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
		return nil, waError("failed to remove group members", err)
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
		return nil, waError("failed to leave group", err)
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
		return nil, waError("failed to promote member", err)
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
		return nil, waError("failed to demote member", err)
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

// =============================================================================
// Member Metrics (Fase B)
// =============================================================================

// WithGroupMemberRepo liga o histórico de membros ao handler. Opcional: sem
// ele o RPC responde "coletando" em vez de falhar — a tela de grupos não pode
// quebrar porque o tracking não está configurado.
func WithGroupMemberRepo(repo repository.GroupMemberRepository) GroupsHandlerOption {
	return func(h *GroupsHandler) {
		h.groupMemberRepo = repo
	}
}

// Janela da série. O default de 30 dias cobre o uso normal; o teto de 90
// impede que um cliente peça um scan largo do agregado.
const (
	metricsDefaultDays = 30
	metricsMaxDays     = 90
)

// GetGroupMemberMetrics devolve o histórico de membros da conta (ou de um grupo).
//
// Contrato de honestidade: enquanto a reconciliação não tiver coberto os grupos,
// o status é COLLECTING e members_count vem AUSENTE — nunca zero. Zero só
// aparece quando foi de fato observado.
func (h *GroupsHandler) GetGroupMemberMetrics(
	ctx context.Context,
	req *connect.Request[groupsv1.GetGroupMemberMetricsRequest],
) (*connect.Response[groupsv1.GetGroupMemberMetricsResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	if h.groupMemberRepo == nil {
		return connect.NewResponse(&groupsv1.GetGroupMemberMetricsResponse{
			Status: groupsv1.MemberDataStatus_MEMBER_DATA_STATUS_COLLECTING,
		}), nil
	}

	days := int(req.Msg.GetDays())
	if days <= 0 {
		days = metricsDefaultDays
	}
	if days > metricsMaxDays {
		days = metricsMaxDays
	}

	metrics, err := h.groupMemberRepo.GetMemberMetrics(ctx, user.ID, days, req.Msg.GetGroupId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to load member metrics: %w", err))
	}

	resp := &groupsv1.GetGroupMemberMetricsResponse{
		Status:         groupsv1.MemberDataStatus_MEMBER_DATA_STATUS_COLLECTING,
		GroupsTotal:    int32(metrics.GroupsTotal),
		GroupsWithData: int32(metrics.GroupsWithData),
	}
	// OK exige cobertura real. Sem nenhum grupo com baseline a tela mostra o
	// estado de coleta, não um gráfico vazio que parece bug.
	if metrics.GroupsWithData > 0 {
		resp.Status = groupsv1.MemberDataStatus_MEMBER_DATA_STATUS_OK
	}
	if metrics.ActiveMembers != nil {
		resp.ActiveMembers = proto.Int32(int32(*metrics.ActiveMembers))
	}

	for _, p := range metrics.Series {
		point := &groupsv1.GroupMemberDailyPoint{
			Date:           p.Date.Format("2006-01-02"),
			Joins:          int32(p.Joins),
			Leaves:         int32(p.Leaves),
			GroupsObserved: int32(p.GroupsObserved),
		}
		if p.MembersCount != nil {
			point.MembersCount = proto.Int32(int32(*p.MembersCount))
		}
		resp.Series = append(resp.Series, point)
	}

	toEntries := func(src []models.GroupMemberRankingEntry) []*groupsv1.GroupMemberRankingEntry {
		out := make([]*groupsv1.GroupMemberRankingEntry, 0, len(src))
		for _, e := range src {
			entry := &groupsv1.GroupMemberRankingEntry{
				GroupId:   e.GroupID,
				GroupName: e.GroupName,
				Joins:     int32(e.Joins),
				Leaves:    int32(e.Leaves),
				Net:       int32(e.Net),
			}
			if e.MembersCount != nil {
				entry.MembersCount = proto.Int32(int32(*e.MembersCount))
			}
			out = append(out, entry)
		}
		return out
	}
	resp.TopGrowth = toEntries(metrics.TopGrowth)
	resp.TopLoss = toEntries(metrics.TopLoss)

	for _, d := range metrics.DDDDistribution {
		resp.DddDistribution = append(resp.DddDistribution, &groupsv1.DddDistributionEntry{
			Ddd:     d.DDD,
			Members: int32(d.Members),
		})
	}

	return connect.NewResponse(resp), nil
}
