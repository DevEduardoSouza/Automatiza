package connectrpc

import (
	"context"
	"fmt"

	plansv1 "automatiza-go/internal/gen/plans/v1"
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"

	"connectrpc.com/connect"
)

// PlansHandler implements the plansv1connect.PlansServiceHandler interface.
type PlansHandler struct {
	planRepo     repository.PlanRepository
	userRoleRepo repository.UserRoleRepository
}

// NewPlansHandler creates a new plans handler.
func NewPlansHandler(
	planRepo repository.PlanRepository,
	userRoleRepo repository.UserRoleRepository,
) *PlansHandler {
	return &PlansHandler{
		planRepo:     planRepo,
		userRoleRepo: userRoleRepo,
	}
}

// requireAdmin checks if the user is an admin and returns an error if not.
func (h *PlansHandler) requireAdmin(ctx context.Context) error {
	user := GetUserFromContext(ctx)
	if user == nil {
		return connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	isAdmin, err := h.userRoleRepo.IsAdmin(ctx, user.ID)
	if err != nil {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("failed to check admin status: %w", err))
	}

	if !isAdmin {
		return connect.NewError(connect.CodePermissionDenied, fmt.Errorf("admin access required"))
	}

	return nil
}

// ListPlans returns all plans.
func (h *PlansHandler) ListPlans(
	ctx context.Context,
	req *connect.Request[plansv1.ListPlansRequest],
) (*connect.Response[plansv1.ListPlansResponse], error) {
	if err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}

	var plans []models.Plan
	var err error

	if req.Msg.GetIncludeInactive() {
		plans, err = h.planRepo.GetAllPlans(ctx)
	} else {
		plans, err = h.planRepo.GetActivePlans(ctx)
	}

	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to list plans: %w", err))
	}

	protoPlans := make([]*plansv1.Plan, len(plans))
	for i, p := range plans {
		protoPlans[i] = planModelToProto(&p)
	}

	return connect.NewResponse(&plansv1.ListPlansResponse{
		Plans: protoPlans,
	}), nil
}

// GetPlan returns a specific plan by ID.
func (h *PlansHandler) GetPlan(
	ctx context.Context,
	req *connect.Request[plansv1.GetPlanRequest],
) (*connect.Response[plansv1.GetPlanResponse], error) {
	if err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}

	plan, err := h.planRepo.GetByID(ctx, req.Msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get plan: %w", err))
	}
	if plan == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("plan not found"))
	}

	return connect.NewResponse(&plansv1.GetPlanResponse{
		Plan: planModelToProto(plan),
	}), nil
}

// CreatePlan creates a new plan.
func (h *PlansHandler) CreatePlan(
	ctx context.Context,
	req *connect.Request[plansv1.CreatePlanRequest],
) (*connect.Response[plansv1.CreatePlanResponse], error) {
	if err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}

	plan := &models.Plan{
		Slug:         req.Msg.GetSlug(),
		Name:         req.Msg.GetName(),
		IsActive:     req.Msg.GetIsActive(),
		GroupLimit:   req.Msg.GetGroupLimit(),
		StorageLimit: req.Msg.GetStorageLimit(),
		Features:     req.Msg.GetFeatures(),
	}

	if desc := req.Msg.GetDescription(); desc != "" {
		plan.Description = &desc
	}
	if priceMonthly := req.Msg.GetPriceMonthly(); priceMonthly > 0 {
		plan.PriceMonthly = &priceMonthly
	}
	if priceSemester := req.Msg.GetPriceSemester(); priceSemester > 0 {
		plan.PriceSemester = &priceSemester
	}
	if priceAnnual := req.Msg.GetPriceAnnual(); priceAnnual > 0 {
		plan.PriceAnnual = &priceAnnual
	}
	if kiwifyMonthly := req.Msg.GetKiwifyProductIdMonthly(); kiwifyMonthly != "" {
		plan.KiwifyProductIDMonthly = &kiwifyMonthly
	}
	if kiwifySemester := req.Msg.GetKiwifyProductIdSemester(); kiwifySemester != "" {
		plan.KiwifyProductIDSemester = &kiwifySemester
	}
	if kiwifyAnnual := req.Msg.GetKiwifyProductIdAnnual(); kiwifyAnnual != "" {
		plan.KiwifyProductIDAnnual = &kiwifyAnnual
	}

	created, err := h.planRepo.Create(ctx, plan)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create plan: %w", err))
	}

	return connect.NewResponse(&plansv1.CreatePlanResponse{
		Plan: planModelToProto(created),
	}), nil
}

// UpdatePlan updates an existing plan.
func (h *PlansHandler) UpdatePlan(
	ctx context.Context,
	req *connect.Request[plansv1.UpdatePlanRequest],
) (*connect.Response[plansv1.UpdatePlanResponse], error) {
	if err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}

	// Check if plan exists
	existing, err := h.planRepo.GetByID(ctx, req.Msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get plan: %w", err))
	}
	if existing == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("plan not found"))
	}

	// Build updates map
	updates := make(map[string]interface{})
	if req.Msg.Slug != nil {
		updates["slug"] = req.Msg.GetSlug()
	}
	if req.Msg.Name != nil {
		updates["name"] = req.Msg.GetName()
	}
	if req.Msg.IsActive != nil {
		updates["is_active"] = req.Msg.GetIsActive()
	}
	if req.Msg.Description != nil {
		val := req.Msg.GetDescription()
		if val == "" {
			updates["description"] = nil
		} else {
			updates["description"] = val
		}
	}
	if req.Msg.PriceMonthly != nil {
		updates["price_monthly"] = req.Msg.GetPriceMonthly()
	}
	if req.Msg.PriceSemester != nil {
		updates["price_semester"] = req.Msg.GetPriceSemester()
	}
	if req.Msg.PriceAnnual != nil {
		updates["price_annual"] = req.Msg.GetPriceAnnual()
	}
	if req.Msg.GroupLimit != nil {
		updates["group_limit"] = req.Msg.GetGroupLimit()
	}
	if req.Msg.StorageLimit != nil {
		updates["storage_limit"] = req.Msg.GetStorageLimit()
	}
	if req.Msg.Features != nil {
		updates["features"] = req.Msg.GetFeatures()
	}
	if req.Msg.KiwifyProductIdMonthly != nil {
		val := req.Msg.GetKiwifyProductIdMonthly()
		if val == "" {
			updates["kiwify_product_id_monthly"] = nil
		} else {
			updates["kiwify_product_id_monthly"] = val
		}
	}
	if req.Msg.KiwifyProductIdSemester != nil {
		val := req.Msg.GetKiwifyProductIdSemester()
		if val == "" {
			updates["kiwify_product_id_semester"] = nil
		} else {
			updates["kiwify_product_id_semester"] = val
		}
	}
	if req.Msg.KiwifyProductIdAnnual != nil {
		val := req.Msg.GetKiwifyProductIdAnnual()
		if val == "" {
			updates["kiwify_product_id_annual"] = nil
		} else {
			updates["kiwify_product_id_annual"] = val
		}
	}

	if len(updates) > 0 {
		if err := h.planRepo.Update(ctx, req.Msg.GetId(), updates); err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update plan: %w", err))
		}
	}

	// Fetch updated plan
	updated, err := h.planRepo.GetByID(ctx, req.Msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get updated plan: %w", err))
	}

	return connect.NewResponse(&plansv1.UpdatePlanResponse{
		Plan: planModelToProto(updated),
	}), nil
}

// DeletePlan deletes a plan.
func (h *PlansHandler) DeletePlan(
	ctx context.Context,
	req *connect.Request[plansv1.DeletePlanRequest],
) (*connect.Response[plansv1.DeletePlanResponse], error) {
	if err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}

	if err := h.planRepo.Delete(ctx, req.Msg.GetId()); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete plan: %w", err))
	}

	return connect.NewResponse(&plansv1.DeletePlanResponse{
		Success: true,
	}), nil
}

// planModelToProto converts a models.Plan to plansv1.Plan
func planModelToProto(p *models.Plan) *plansv1.Plan {
	if p == nil {
		return nil
	}

	proto := &plansv1.Plan{
		Id:           p.ID,
		Slug:         p.Slug,
		Name:         p.Name,
		IsActive:     p.IsActive,
		CreatedAt:    p.CreatedAt,
		UpdatedAt:    p.UpdatedAt,
		GroupLimit:   p.GroupLimit,
		StorageLimit: p.StorageLimit,
		Features:     p.Features,
	}

	if p.Description != nil {
		proto.Description = p.Description
	}
	if p.PriceMonthly != nil {
		proto.PriceMonthly = p.PriceMonthly
	}
	if p.PriceSemester != nil {
		proto.PriceSemester = p.PriceSemester
	}
	if p.PriceAnnual != nil {
		proto.PriceAnnual = p.PriceAnnual
	}
	if p.KiwifyProductIDMonthly != nil {
		proto.KiwifyProductIdMonthly = p.KiwifyProductIDMonthly
	}
	if p.KiwifyProductIDSemester != nil {
		proto.KiwifyProductIdSemester = p.KiwifyProductIDSemester
	}
	if p.KiwifyProductIDAnnual != nil {
		proto.KiwifyProductIdAnnual = p.KiwifyProductIDAnnual
	}

	return proto
}
