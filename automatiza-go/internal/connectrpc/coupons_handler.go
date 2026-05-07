package connectrpc

import (
	"context"
	"errors"
	"fmt"

	couponsv1 "automatiza-go/internal/gen/coupons/v1"
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"
	"automatiza-go/internal/services/coupons"

	"connectrpc.com/connect"
)

// CouponsHandler implements the couponsv1connect.CouponsServiceHandler interface.
//
// Holds two collaborators: the legacy coupons.Service (catalog CRUD)
// and the new couponRuleRepo (auto-apply rules introduced with
// migration 046). The rule RPCs talk to the repo directly — no service
// layer wraps them yet, since the matcher is the only consumer and it
// already lives next to the dispatch flow.
type CouponsHandler struct {
	couponsService coupons.Service
	couponRuleRepo repository.CouponRuleRepository
}

// NewCouponsHandler creates a new coupons handler. couponRuleRepo can
// be nil during transitional configurations; the rule RPCs then return
// Unimplemented instead of crashing.
func NewCouponsHandler(couponsService coupons.Service, couponRuleRepo repository.CouponRuleRepository) *CouponsHandler {
	return &CouponsHandler{
		couponsService: couponsService,
		couponRuleRepo: couponRuleRepo,
	}
}

// ListCoupons returns all coupons for the authenticated user.
func (h *CouponsHandler) ListCoupons(
	ctx context.Context,
	req *connect.Request[couponsv1.ListCouponsRequest],
) (*connect.Response[couponsv1.ListCouponsResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	var filters *coupons.CouponFilters
	if filter := req.Msg.GetFilter(); filter != nil {
		filters = &coupons.CouponFilters{}
		if filter.Store != nil {
			filters.Store = protoStoreToModelCoupon(filter.GetStore())
		}
		if filter.IsActive != nil {
			isActive := filter.GetIsActive()
			filters.IsActive = &isActive
		}
		if filter.Category != nil {
			filters.Category = filter.GetCategory()
		}
	}

	couponList, err := h.couponsService.GetUserCoupons(ctx, user.ID, filters)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get coupons: %w", err))
	}

	protoCoupons := make([]*couponsv1.Coupon, len(couponList))
	for i, c := range couponList {
		protoCoupons[i] = couponToProto(&c)
	}

	return connect.NewResponse(&couponsv1.ListCouponsResponse{
		Coupons: protoCoupons,
	}), nil
}

// GetCoupon returns a specific coupon by ID.
func (h *CouponsHandler) GetCoupon(
	ctx context.Context,
	req *connect.Request[couponsv1.GetCouponRequest],
) (*connect.Response[couponsv1.GetCouponResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	coupon, err := h.couponsService.GetCouponByID(ctx, req.Msg.GetId(), user.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get coupon: %w", err))
	}
	if coupon == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("coupon not found"))
	}

	return connect.NewResponse(&couponsv1.GetCouponResponse{
		Coupon: couponToProto(coupon),
	}), nil
}

// CreateCoupon creates a new coupon.
func (h *CouponsHandler) CreateCoupon(
	ctx context.Context,
	req *connect.Request[couponsv1.CreateCouponRequest],
) (*connect.Response[couponsv1.CreateCouponResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	data := &coupons.CreateCouponData{
		Code:  req.Msg.GetCode(),
		Title: req.Msg.GetTitle(),
		Store: protoStoreToModelCoupon(req.Msg.GetStore()),
	}

	if req.Msg.AffiliateLink != nil {
		data.AffiliateLink = req.Msg.GetAffiliateLink()
	}
	if req.Msg.ExpiresAt != nil {
		data.ExpiresAt = req.Msg.GetExpiresAt()
	}
	if req.Msg.Category != nil {
		data.Category = req.Msg.GetCategory()
	}
	data.DiscountType = protoDiscountTypeToModel(req.Msg.GetDiscountType())
	if req.Msg.DiscountValue != nil {
		data.DiscountValue = req.Msg.GetDiscountValue()
	}
	if req.Msg.MaxDiscount != nil {
		data.MaxDiscount = req.Msg.GetMaxDiscount()
	}
	if req.Msg.MinProductPrice != nil {
		data.MinProductPrice = req.Msg.GetMinProductPrice()
	}
	if req.Msg.CouponText != nil {
		data.CouponText = req.Msg.GetCouponText()
	}

	created, err := h.couponsService.CreateCoupon(ctx, user.ID, data)
	if err != nil {
		if coupons.IsDuplicateCodeError(err) {
			return nil, connect.NewError(connect.CodeAlreadyExists, err)
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create coupon: %w", err))
	}

	return connect.NewResponse(&couponsv1.CreateCouponResponse{
		Coupon: couponToProto(created),
	}), nil
}

// UpdateCoupon updates an existing coupon.
func (h *CouponsHandler) UpdateCoupon(
	ctx context.Context,
	req *connect.Request[couponsv1.UpdateCouponRequest],
) (*connect.Response[couponsv1.UpdateCouponResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	data := &coupons.UpdateCouponData{}
	if req.Msg.Code != nil {
		code := req.Msg.GetCode()
		data.Code = &code
	}
	if req.Msg.Title != nil {
		title := req.Msg.GetTitle()
		data.Title = &title
	}
	if req.Msg.Store != nil {
		store := protoStoreToModelCoupon(req.Msg.GetStore())
		data.Store = &store
	}
	if req.Msg.AffiliateLink != nil {
		link := req.Msg.GetAffiliateLink()
		data.AffiliateLink = &link
	}
	if req.Msg.ExpiresAt != nil {
		expires := req.Msg.GetExpiresAt()
		data.ExpiresAt = &expires
	}
	if req.Msg.IsActive != nil {
		isActive := req.Msg.GetIsActive()
		data.IsActive = &isActive
	}
	if req.Msg.Category != nil {
		category := req.Msg.GetCategory()
		data.Category = &category
	}
	if req.Msg.DiscountType != nil {
		discountType := protoDiscountTypeToModel(req.Msg.GetDiscountType())
		data.DiscountType = &discountType
	}
	if req.Msg.DiscountValue != nil {
		discountValue := req.Msg.GetDiscountValue()
		data.DiscountValue = &discountValue
	}
	if req.Msg.MaxDiscount != nil {
		maxDiscount := req.Msg.GetMaxDiscount()
		data.MaxDiscount = &maxDiscount
	}
	if req.Msg.MinProductPrice != nil {
		minProductPrice := req.Msg.GetMinProductPrice()
		data.MinProductPrice = &minProductPrice
	}
	if req.Msg.CouponText != nil {
		couponText := req.Msg.GetCouponText()
		data.CouponText = &couponText
	}

	updated, err := h.couponsService.UpdateCoupon(ctx, req.Msg.GetId(), user.ID, data)
	if err != nil {
		if coupons.IsDuplicateCodeError(err) {
			return nil, connect.NewError(connect.CodeAlreadyExists, err)
		}
		if coupons.IsCouponNotFoundError(err) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update coupon: %w", err))
	}

	return connect.NewResponse(&couponsv1.UpdateCouponResponse{
		Coupon: couponToProto(updated),
	}), nil
}

// DeleteCoupon deletes a coupon.
func (h *CouponsHandler) DeleteCoupon(
	ctx context.Context,
	req *connect.Request[couponsv1.DeleteCouponRequest],
) (*connect.Response[couponsv1.DeleteCouponResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	if err := h.couponsService.DeleteCoupon(ctx, req.Msg.GetId(), user.ID); err != nil {
		if coupons.IsCouponNotFoundError(err) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete coupon: %w", err))
	}

	return connect.NewResponse(&couponsv1.DeleteCouponResponse{}), nil
}

// ListCouponsByStore returns active coupons for a specific store.
func (h *CouponsHandler) ListCouponsByStore(
	ctx context.Context,
	req *connect.Request[couponsv1.ListCouponsByStoreRequest],
) (*connect.Response[couponsv1.ListCouponsByStoreResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	store := protoStoreToModelCoupon(req.Msg.GetStore())
	couponList, err := h.couponsService.GetCouponsByStore(ctx, user.ID, store)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get coupons: %w", err))
	}

	protoCoupons := make([]*couponsv1.Coupon, len(couponList))
	for i, c := range couponList {
		protoCoupons[i] = couponToProto(&c)
	}

	return connect.NewResponse(&couponsv1.ListCouponsByStoreResponse{
		Coupons: protoCoupons,
	}), nil
}

// Helper functions

func couponToProto(c *models.Coupon) *couponsv1.Coupon {
	if c == nil {
		return nil
	}

	proto := &couponsv1.Coupon{
		Id:        c.ID,
		Code:      c.Code,
		Title:     c.Title,
		Store:     modelStoreToProtoCoupon(c.Store),
		IsActive:  c.IsActive,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}

	if c.AffiliateLink != nil {
		proto.AffiliateLink = c.AffiliateLink
	}
	if c.ExpiresAt != nil {
		proto.ExpiresAt = c.ExpiresAt
	}
	if c.Category != nil {
		proto.Category = c.Category
	}
	if c.DiscountType != nil {
		proto.DiscountType = modelDiscountTypeToProto(*c.DiscountType)
	}
	if c.DiscountValue != nil {
		proto.DiscountValue = c.DiscountValue
	}
	if c.MaxDiscount != nil {
		proto.MaxDiscount = c.MaxDiscount
	}
	if c.MinProductPrice != nil {
		proto.MinProductPrice = c.MinProductPrice
	}
	if c.CouponText != nil {
		proto.CouponText = c.CouponText
	}

	return proto
}

func modelStoreToProtoCoupon(store string) couponsv1.Store {
	switch store {
	case "shopee":
		return couponsv1.Store_STORE_SHOPEE
	case "amazon":
		return couponsv1.Store_STORE_AMAZON
	case "magalu":
		return couponsv1.Store_STORE_MAGALU
	case "ml":
		return couponsv1.Store_STORE_MERCADO_LIVRE
	case "shein":
		return couponsv1.Store_STORE_SHEIN
	case "aliexpress":
		return couponsv1.Store_STORE_ALIEXPRESS
	case "awin":
		return couponsv1.Store_STORE_AWIN
	default:
		return couponsv1.Store_STORE_UNSPECIFIED
	}
}

func protoStoreToModelCoupon(store couponsv1.Store) string {
	switch store {
	case couponsv1.Store_STORE_SHOPEE:
		return "shopee"
	case couponsv1.Store_STORE_AMAZON:
		return "amazon"
	case couponsv1.Store_STORE_MAGALU:
		return "magalu"
	case couponsv1.Store_STORE_MERCADO_LIVRE:
		return "ml"
	case couponsv1.Store_STORE_SHEIN:
		return "shein"
	case couponsv1.Store_STORE_ALIEXPRESS:
		return "aliexpress"
	case couponsv1.Store_STORE_AWIN:
		return "awin"
	default:
		return ""
	}
}

func modelDiscountTypeToProto(discountType string) couponsv1.DiscountType {
	switch discountType {
	case "fixed":
		return couponsv1.DiscountType_DISCOUNT_TYPE_FIXED
	case "percent":
		return couponsv1.DiscountType_DISCOUNT_TYPE_PERCENT
	default:
		return couponsv1.DiscountType_DISCOUNT_TYPE_UNSPECIFIED
	}
}

func protoDiscountTypeToModel(discountType couponsv1.DiscountType) string {
	switch discountType {
	case couponsv1.DiscountType_DISCOUNT_TYPE_FIXED:
		return "fixed"
	case couponsv1.DiscountType_DISCOUNT_TYPE_PERCENT:
		return "percent"
	default:
		return ""
	}
}

// ============================================================
// Coupon Rules (auto-apply rules)
// ============================================================

var errRulesEngineDisabled = errors.New("coupon rules engine is not configured")

func (h *CouponsHandler) requireRules() error {
	if h.couponRuleRepo == nil {
		return errRulesEngineDisabled
	}
	return nil
}

func (h *CouponsHandler) ListCouponRules(
	ctx context.Context,
	_ *connect.Request[couponsv1.ListCouponRulesRequest],
) (*connect.Response[couponsv1.ListCouponRulesResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}
	if err := h.requireRules(); err != nil {
		return nil, connect.NewError(connect.CodeUnimplemented, err)
	}
	rules, err := h.couponRuleRepo.GetUserCouponRules(ctx, user.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to list coupon rules: %w", err))
	}
	out := make([]*couponsv1.CouponRule, len(rules))
	for i := range rules {
		out[i] = couponRuleToProto(&rules[i])
	}
	return connect.NewResponse(&couponsv1.ListCouponRulesResponse{Rules: out}), nil
}

func (h *CouponsHandler) GetCouponRule(
	ctx context.Context,
	req *connect.Request[couponsv1.GetCouponRuleRequest],
) (*connect.Response[couponsv1.GetCouponRuleResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}
	if err := h.requireRules(); err != nil {
		return nil, connect.NewError(connect.CodeUnimplemented, err)
	}
	rule, err := h.couponRuleRepo.GetCouponRuleByID(ctx, req.Msg.GetId(), user.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get coupon rule: %w", err))
	}
	if rule == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("coupon rule not found"))
	}
	return connect.NewResponse(&couponsv1.GetCouponRuleResponse{Rule: couponRuleToProto(rule)}), nil
}

func (h *CouponsHandler) CreateCouponRule(
	ctx context.Context,
	req *connect.Request[couponsv1.CreateCouponRuleRequest],
) (*connect.Response[couponsv1.CreateCouponRuleResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}
	if err := h.requireRules(); err != nil {
		return nil, connect.NewError(connect.CodeUnimplemented, err)
	}
	rule := &models.CouponRule{
		CouponID: req.Msg.GetCouponId(),
		Platform: req.Msg.GetPlatform(),
		IsActive: true, // default if not provided
	}
	if v := req.Msg.PeriodStart; v != nil && *v != "" {
		rule.PeriodStart = v
	}
	if v := req.Msg.PeriodEnd; v != nil && *v != "" {
		rule.PeriodEnd = v
	}
	if v := req.Msg.MinPrice; v != nil {
		rule.MinPrice = *v
	}
	if v := req.Msg.Priority; v != nil {
		rule.Priority = int(*v)
	}
	if v := req.Msg.IsActive; v != nil {
		rule.IsActive = *v
	}
	created, err := h.couponRuleRepo.CreateCouponRule(ctx, user.ID, rule)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create coupon rule: %w", err))
	}
	return connect.NewResponse(&couponsv1.CreateCouponRuleResponse{Rule: couponRuleToProto(created)}), nil
}

func (h *CouponsHandler) UpdateCouponRule(
	ctx context.Context,
	req *connect.Request[couponsv1.UpdateCouponRuleRequest],
) (*connect.Response[couponsv1.UpdateCouponRuleResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}
	if err := h.requireRules(); err != nil {
		return nil, connect.NewError(connect.CodeUnimplemented, err)
	}
	updates := make(map[string]interface{})
	if v := req.Msg.CouponId; v != nil {
		updates["coupon_id"] = *v
	}
	if v := req.Msg.Platform; v != nil {
		updates["platform"] = *v
	}
	if v := req.Msg.PeriodStart; v != nil {
		if *v == "" {
			updates["period_start"] = nil
		} else {
			updates["period_start"] = *v
		}
	}
	if v := req.Msg.PeriodEnd; v != nil {
		if *v == "" {
			updates["period_end"] = nil
		} else {
			updates["period_end"] = *v
		}
	}
	if v := req.Msg.MinPrice; v != nil {
		updates["min_price"] = *v
	}
	if v := req.Msg.Priority; v != nil {
		updates["priority"] = int(*v)
	}
	if v := req.Msg.IsActive; v != nil {
		updates["is_active"] = *v
	}
	if err := h.couponRuleRepo.UpdateCouponRule(ctx, req.Msg.GetId(), user.ID, updates); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update coupon rule: %w", err))
	}
	updated, err := h.couponRuleRepo.GetCouponRuleByID(ctx, req.Msg.GetId(), user.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to reload coupon rule: %w", err))
	}
	return connect.NewResponse(&couponsv1.UpdateCouponRuleResponse{Rule: couponRuleToProto(updated)}), nil
}

func (h *CouponsHandler) DeleteCouponRule(
	ctx context.Context,
	req *connect.Request[couponsv1.DeleteCouponRuleRequest],
) (*connect.Response[couponsv1.DeleteCouponRuleResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}
	if err := h.requireRules(); err != nil {
		return nil, connect.NewError(connect.CodeUnimplemented, err)
	}
	if err := h.couponRuleRepo.DeleteCouponRule(ctx, req.Msg.GetId(), user.ID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete coupon rule: %w", err))
	}
	return connect.NewResponse(&couponsv1.DeleteCouponRuleResponse{}), nil
}

func couponRuleToProto(r *models.CouponRule) *couponsv1.CouponRule {
	if r == nil {
		return nil
	}
	out := &couponsv1.CouponRule{
		Id:        r.ID,
		UserId:    r.UserID,
		CouponId:  r.CouponID,
		Platform:  r.Platform,
		MinPrice:  r.MinPrice,
		Priority:  int32(r.Priority),
		IsActive:  r.IsActive,
		CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
	}
	if r.PeriodStart != nil {
		out.PeriodStart = r.PeriodStart
	}
	if r.PeriodEnd != nil {
		out.PeriodEnd = r.PeriodEnd
	}
	return out
}
