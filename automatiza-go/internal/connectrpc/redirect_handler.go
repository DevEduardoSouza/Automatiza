package connectrpc

import (
	"context"
	"fmt"

	redirectv1 "automatiza-go/internal/gen/redirect/v1"
	"automatiza-go/internal/models"
	"automatiza-go/internal/services/redirect"

	"connectrpc.com/connect"
)

// RedirectHandler implements the redirectv1connect.RedirectLinkServiceHandler interface.
type RedirectHandler struct {
	redirectService redirect.Service
}

// NewRedirectHandler creates a new redirect handler.
func NewRedirectHandler(redirectService redirect.Service) *RedirectHandler {
	return &RedirectHandler{
		redirectService: redirectService,
	}
}

// ListGroups returns all available groups for the authenticated user.
func (h *RedirectHandler) ListGroups(
	ctx context.Context,
	req *connect.Request[redirectv1.ListGroupsRequest],
) (*connect.Response[redirectv1.ListGroupsResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	groups, err := h.redirectService.ListGroups(ctx, user.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to list groups: %w", err))
	}

	protoGroups := make([]*redirectv1.RedirectGroup, len(groups))
	for i, g := range groups {
		protoGroups[i] = groupToRedirectProto(&g)
	}

	return connect.NewResponse(&redirectv1.ListGroupsResponse{
		Groups: protoGroups,
	}), nil
}

// ListRedirectLinks returns all saved redirect links for the authenticated user.
func (h *RedirectHandler) ListRedirectLinks(
	ctx context.Context,
	req *connect.Request[redirectv1.ListRedirectLinksRequest],
) (*connect.Response[redirectv1.ListRedirectLinksResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	links, err := h.redirectService.ListRedirectLinks(ctx, user.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to list redirect links: %w", err))
	}

	protoLinks := make([]*redirectv1.SavedRedirectLink, len(links))
	for i, l := range links {
		protoLinks[i] = redirectLinkToProto(&l)
	}

	return connect.NewResponse(&redirectv1.ListRedirectLinksResponse{
		Links: protoLinks,
	}), nil
}

// CreateRedirectLink creates a new redirect link.
func (h *RedirectHandler) CreateRedirectLink(
	ctx context.Context,
	req *connect.Request[redirectv1.CreateRedirectLinkRequest],
) (*connect.Response[redirectv1.CreateRedirectLinkResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	// Convert proto to model
	createReq := &models.RedirectLinkCreateRequest{
		Name:         req.Msg.GetName(),
		Slug:         req.Msg.GetSlug(),
		Strategy:     protoStrategyToString(req.Msg.GetDistributionStrategy()),
		TargetGroups: protoGroupsToModel(req.Msg.GetGroupPriorities()),
		IsActive:     true, // Default to active
	}

	link, err := h.redirectService.CreateRedirectLink(ctx, user.ID, createReq)
	if err != nil {
		if err.Error() == "slug already in use" {
			return nil, connect.NewError(connect.CodeAlreadyExists, err)
		}
		if err.Error() == "invalid strategy" {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create redirect link: %w", err))
	}

	return connect.NewResponse(&redirectv1.CreateRedirectLinkResponse{
		Link: redirectLinkToProto(link),
	}), nil
}

// UpdateRedirectLink updates an existing redirect link.
func (h *RedirectHandler) UpdateRedirectLink(
	ctx context.Context,
	req *connect.Request[redirectv1.UpdateRedirectLinkRequest],
) (*connect.Response[redirectv1.UpdateRedirectLinkResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	linkID := req.Msg.GetId()
	if linkID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("link id is required"))
	}

	// Convert proto to model update request
	updateReq := &models.RedirectLinkUpdateRequest{}

	if req.Msg.GetName() != "" {
		name := req.Msg.GetName()
		updateReq.Name = &name
	}

	if req.Msg.GetDistributionStrategy() != redirectv1.DistributionStrategy_DISTRIBUTION_STRATEGY_UNSPECIFIED {
		strategy := protoStrategyToString(req.Msg.GetDistributionStrategy())
		updateReq.Strategy = &strategy
	}

	if len(req.Msg.GetGroupPriorities()) > 0 {
		groups := protoGroupsToModel(req.Msg.GetGroupPriorities())
		updateReq.TargetGroups = &groups
	}

	err := h.redirectService.UpdateRedirectLink(ctx, linkID, user.ID, updateReq)
	if err != nil {
		if err.Error() == "redirect link not found" {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		if err.Error() == "unauthorized" {
			return nil, connect.NewError(connect.CodePermissionDenied, err)
		}
		if err.Error() == "slug already in use" {
			return nil, connect.NewError(connect.CodeAlreadyExists, err)
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update redirect link: %w", err))
	}

	// Fetch updated link
	link, err := h.redirectService.GetRedirectLink(ctx, linkID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to fetch updated link: %w", err))
	}

	return connect.NewResponse(&redirectv1.UpdateRedirectLinkResponse{
		Link: redirectLinkToProto(link),
	}), nil
}

// DeleteRedirectLink deletes a redirect link.
func (h *RedirectHandler) DeleteRedirectLink(
	ctx context.Context,
	req *connect.Request[redirectv1.DeleteRedirectLinkRequest],
) (*connect.Response[redirectv1.DeleteRedirectLinkResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	linkID := req.Msg.GetId()
	if linkID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("link id is required"))
	}

	err := h.redirectService.DeleteRedirectLink(ctx, linkID, user.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete redirect link: %w", err))
	}

	return connect.NewResponse(&redirectv1.DeleteRedirectLinkResponse{}), nil
}

// CheckSlugAvailability checks if a slug is available for use.
func (h *RedirectHandler) CheckSlugAvailability(
	ctx context.Context,
	req *connect.Request[redirectv1.CheckSlugAvailabilityRequest],
) (*connect.Response[redirectv1.CheckSlugAvailabilityResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	slug := req.Msg.GetSlug()
	if slug == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("slug is required"))
	}

	available, err := h.redirectService.CheckSlugAvailability(ctx, slug, "")
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to check slug availability: %w", err))
	}

	return connect.NewResponse(&redirectv1.CheckSlugAvailabilityResponse{
		Available: available,
	}), nil
}

// Helper functions for proto <-> model conversion

func groupToRedirectProto(g *models.Group) *redirectv1.RedirectGroup {
	return &redirectv1.RedirectGroup{
		Id:           g.ID,
		Name:         g.Name,
		WaGroupId:    g.WAGroupID,
		InviteCode:   g.InviteCode,
		MembersCount: int32(g.MembersCount),
		MemberLimit:  int32(g.MemberLimit),
		IsFavorite:   &g.IsFavorite,
	}
}

func redirectLinkToProto(l *models.RedirectLink) *redirectv1.SavedRedirectLink {
	protoGroups := make([]*redirectv1.RedirectGroup, len(l.TargetGroups))
	for i, tg := range l.TargetGroups {
		priority := int32(tg.Priority)
		clickLimit := int32(tg.ClickLimit)
		protoGroups[i] = &redirectv1.RedirectGroup{
			Id:          tg.GroupID,
			Name:        tg.GroupName,
			WaGroupId:   tg.WAGroupID,
			Priority:    &priority,
			ClickLimit:  &clickLimit,
			MemberLimit: int32(tg.MemberLimit),
		}
	}

	return &redirectv1.SavedRedirectLink{
		Id:                   l.ID,
		UserId:               l.UserID,
		Name:                 l.Name,
		Slug:                 l.Slug,
		DistributionStrategy: stringToProtoStrategy(l.Strategy),
		GroupPriorities:      protoGroups,
		IsActive:             l.IsActive,
		TotalClicks:          int32(l.ClickCount),
		CreatedAt:            l.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:            l.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func protoStrategyToString(strategy redirectv1.DistributionStrategy) string {
	switch strategy {
	case redirectv1.DistributionStrategy_DISTRIBUTION_STRATEGY_MEMBER_LIMIT:
		return models.RedirectStrategyMemberLimit
	case redirectv1.DistributionStrategy_DISTRIBUTION_STRATEGY_CLICK_LIMIT:
		return models.RedirectStrategyClickLimit
	case redirectv1.DistributionStrategy_DISTRIBUTION_STRATEGY_ROUND_ROBIN:
		return models.RedirectStrategyRoundRobin
	default:
		return models.RedirectStrategyMemberLimit // Default
	}
}

func stringToProtoStrategy(strategy string) redirectv1.DistributionStrategy {
	switch strategy {
	case models.RedirectStrategyMemberLimit:
		return redirectv1.DistributionStrategy_DISTRIBUTION_STRATEGY_MEMBER_LIMIT
	case models.RedirectStrategyClickLimit:
		return redirectv1.DistributionStrategy_DISTRIBUTION_STRATEGY_CLICK_LIMIT
	case models.RedirectStrategyRoundRobin:
		return redirectv1.DistributionStrategy_DISTRIBUTION_STRATEGY_ROUND_ROBIN
	default:
		return redirectv1.DistributionStrategy_DISTRIBUTION_STRATEGY_UNSPECIFIED
	}
}

func protoGroupsToModel(protoGroups []*redirectv1.RedirectGroup) []models.GroupPriority {
	groups := make([]models.GroupPriority, len(protoGroups))
	for i, pg := range protoGroups {
		priority := 0
		if pg.Priority != nil {
			priority = int(*pg.Priority)
		}
		clickLimit := 0
		if pg.ClickLimit != nil {
			clickLimit = int(*pg.ClickLimit)
		}
		groups[i] = models.GroupPriority{
			GroupID:     pg.GetId(),
			GroupName:   pg.GetName(),
			WAGroupID:   pg.GetWaGroupId(),
			Priority:    priority,
			MemberLimit: int(pg.GetMemberLimit()),
			ClickLimit:  clickLimit,
			IsActive:    true, // Default to active
		}
	}
	return groups
}
