package connectrpc

import (
	"context"
	"fmt"

	ctasv1 "automatiza-go/internal/gen/ctas/v1"
	"automatiza-go/internal/models"
	"automatiza-go/internal/services/ctas"

	"connectrpc.com/connect"
)

// CTAsHandler implements the ctasv1connect.CTAsServiceHandler interface.
type CTAsHandler struct {
	ctasService ctas.Service
}

// NewCTAsHandler creates a new CTAs handler.
func NewCTAsHandler(ctasService ctas.Service) *CTAsHandler {
	return &CTAsHandler{
		ctasService: ctasService,
	}
}

// ListCTAs returns all CTAs for the authenticated user.
func (h *CTAsHandler) ListCTAs(
	ctx context.Context,
	req *connect.Request[ctasv1.ListCTAsRequest],
) (*connect.Response[ctasv1.ListCTAsResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	var filters *ctas.CTAFilters
	if filter := req.Msg.GetFilter(); filter != nil {
		filters = &ctas.CTAFilters{}
		if filter.Platform != nil {
			filters.Platform = protoPlatformToModel(filter.GetPlatform())
		}
		if filter.CtaType != nil {
			filters.CTAType = protoCTATypeToModel(filter.GetCtaType())
		}
		if filter.IsActive != nil {
			isActive := filter.GetIsActive()
			filters.IsActive = &isActive
		}
	}

	ctaList, err := h.ctasService.GetUserCTAs(ctx, user.ID, filters)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get CTAs: %w", err))
	}

	protoCTAs := make([]*ctasv1.CTA, len(ctaList))
	for i, c := range ctaList {
		protoCTAs[i] = ctaToProto(&c)
	}

	return connect.NewResponse(&ctasv1.ListCTAsResponse{
		Ctas: protoCTAs,
	}), nil
}

// GetCTA returns a specific CTA by ID.
func (h *CTAsHandler) GetCTA(
	ctx context.Context,
	req *connect.Request[ctasv1.GetCTARequest],
) (*connect.Response[ctasv1.GetCTAResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	cta, err := h.ctasService.GetCTAByID(ctx, req.Msg.GetId(), user.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get CTA: %w", err))
	}
	if cta == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("CTA not found"))
	}

	return connect.NewResponse(&ctasv1.GetCTAResponse{
		Cta: ctaToProto(cta),
	}), nil
}

// CreateCTA creates a new CTA.
func (h *CTAsHandler) CreateCTA(
	ctx context.Context,
	req *connect.Request[ctasv1.CreateCTARequest],
) (*connect.Response[ctasv1.CreateCTAResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	data := &ctas.CreateCTAData{
		Message:  req.Msg.GetMessage(),
		Platform: protoPlatformToModel(req.Msg.GetPlatform()),
		CTAType:  protoCTATypeToModel(req.Msg.GetCtaType()),
	}

	created, err := h.ctasService.CreateCTA(ctx, user.ID, data)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create CTA: %w", err))
	}

	return connect.NewResponse(&ctasv1.CreateCTAResponse{
		Cta: ctaToProto(created),
	}), nil
}

// UpdateCTA updates an existing CTA.
func (h *CTAsHandler) UpdateCTA(
	ctx context.Context,
	req *connect.Request[ctasv1.UpdateCTARequest],
) (*connect.Response[ctasv1.UpdateCTAResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	data := &ctas.UpdateCTAData{}
	if req.Msg.Message != nil {
		message := req.Msg.GetMessage()
		data.Message = &message
	}
	if req.Msg.Platform != nil {
		platform := protoPlatformToModel(req.Msg.GetPlatform())
		data.Platform = &platform
	}
	if req.Msg.CtaType != nil {
		ctaType := protoCTATypeToModel(req.Msg.GetCtaType())
		data.CTAType = &ctaType
	}
	if req.Msg.IsActive != nil {
		isActive := req.Msg.GetIsActive()
		data.IsActive = &isActive
	}

	updated, err := h.ctasService.UpdateCTA(ctx, req.Msg.GetId(), user.ID, data)
	if err != nil {
		if ctas.IsCTANotFoundError(err) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update CTA: %w", err))
	}

	return connect.NewResponse(&ctasv1.UpdateCTAResponse{
		Cta: ctaToProto(updated),
	}), nil
}

// DeleteCTA deletes a CTA.
func (h *CTAsHandler) DeleteCTA(
	ctx context.Context,
	req *connect.Request[ctasv1.DeleteCTARequest],
) (*connect.Response[ctasv1.DeleteCTAResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	if err := h.ctasService.DeleteCTA(ctx, req.Msg.GetId(), user.ID); err != nil {
		if ctas.IsCTANotFoundError(err) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete CTA: %w", err))
	}

	return connect.NewResponse(&ctasv1.DeleteCTAResponse{}), nil
}

// Helper functions

func ctaToProto(c *models.CTA) *ctasv1.CTA {
	if c == nil {
		return nil
	}

	return &ctasv1.CTA{
		Id:        c.ID,
		UserId:    c.UserID,
		Message:   c.Message,
		Platform:  modelPlatformToProto(c.Platform),
		CtaType:   modelCTATypeToProto(c.CTAType),
		IsActive:  c.IsActive,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
}

func modelPlatformToProto(platform string) ctasv1.Platform {
	switch platform {
	case "all":
		return ctasv1.Platform_PLATFORM_ALL
	case "shopee":
		return ctasv1.Platform_PLATFORM_SHOPEE
	case "ml":
		return ctasv1.Platform_PLATFORM_MERCADO_LIVRE
	default:
		return ctasv1.Platform_PLATFORM_UNSPECIFIED
	}
}

func protoPlatformToModel(platform ctasv1.Platform) string {
	switch platform {
	case ctasv1.Platform_PLATFORM_ALL:
		return "all"
	case ctasv1.Platform_PLATFORM_SHOPEE:
		return "shopee"
	case ctasv1.Platform_PLATFORM_MERCADO_LIVRE:
		return "ml"
	default:
		return "all"
	}
}

func modelCTATypeToProto(ctaType string) ctasv1.CTAType {
	switch ctaType {
	case "all":
		return ctasv1.CTAType_CTA_TYPE_ALL
	case "normal":
		return ctasv1.CTAType_CTA_TYPE_NORMAL
	case "discount":
		return ctasv1.CTAType_CTA_TYPE_DISCOUNT
	default:
		return ctasv1.CTAType_CTA_TYPE_UNSPECIFIED
	}
}

func protoCTATypeToModel(ctaType ctasv1.CTAType) string {
	switch ctaType {
	case ctasv1.CTAType_CTA_TYPE_ALL:
		return "all"
	case ctasv1.CTAType_CTA_TYPE_NORMAL:
		return "normal"
	case ctasv1.CTAType_CTA_TYPE_DISCOUNT:
		return "discount"
	default:
		return "all"
	}
}
