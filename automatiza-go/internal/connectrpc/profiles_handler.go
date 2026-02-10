package connectrpc

import (
	"context"
	"errors"
	"fmt"
	"time"

	profilesv1 "automatiza-go/internal/gen/profiles/v1"
	"automatiza-go/internal/models"
	"automatiza-go/internal/services/profile"

	"connectrpc.com/connect"
	"go.uber.org/zap"
)

// ProfilesHandler implements the profilesv1connect.ProfilesServiceHandler interface.
type ProfilesHandler struct {
	profileService  *profile.ProfileService
	passwordService *profile.PasswordService
	logger          *zap.Logger
}

// NewProfilesHandler creates a new profiles handler.
func NewProfilesHandler(
	profileService *profile.ProfileService,
	passwordService *profile.PasswordService,
	logger *zap.Logger,
) *ProfilesHandler {
	return &ProfilesHandler{
		profileService:  profileService,
		passwordService: passwordService,
		logger:          logger,
	}
}

// GetProfile returns the current user's profile.
func (h *ProfilesHandler) GetProfile(
	ctx context.Context,
	req *connect.Request[profilesv1.GetProfileRequest],
) (*connect.Response[profilesv1.GetProfileResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	p, err := h.profileService.GetProfile(ctx, user.ID)
	if err != nil {
		if errors.Is(err, profile.ErrProfileNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("profile not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get profile: %w", err))
	}

	return connect.NewResponse(&profilesv1.GetProfileResponse{
		Profile: profileToProto(p),
	}), nil
}

// UpdateProfile updates the current user's profile.
func (h *ProfilesHandler) UpdateProfile(
	ctx context.Context,
	req *connect.Request[profilesv1.UpdateProfileRequest],
) (*connect.Response[profilesv1.UpdateProfileResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	input := profile.UpdateProfileInput{}
	if req.Msg.PhoneNumber != nil {
		pn := req.Msg.GetPhoneNumber()
		input.PhoneNumber = &pn
	}

	updated, err := h.profileService.UpdateProfile(ctx, user.ID, input)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update profile: %w", err))
	}

	return connect.NewResponse(&profilesv1.UpdateProfileResponse{
		Profile: profileToProto(updated),
	}), nil
}

// GetUserLimits returns the current user's limits and usage.
func (h *ProfilesHandler) GetUserLimits(
	ctx context.Context,
	req *connect.Request[profilesv1.GetUserLimitsRequest],
) (*connect.Response[profilesv1.GetUserLimitsResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	limits, err := h.profileService.GetUserLimits(ctx, user.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get limits: %w", err))
	}

	var maxAutomations *int32
	if limits.MaxAutomations > 0 {
		ma := limits.MaxAutomations
		maxAutomations = &ma
	}

	return connect.NewResponse(&profilesv1.GetUserLimitsResponse{
		Limits: &profilesv1.UserLimits{
			AutomationsCount:        limits.AutomationsCount,
			MaxAutomations:          maxAutomations,
			ConnectedInstancesCount: limits.ConnectedInstancesCount,
		},
	}), nil
}

// ListProfiles returns all user profiles (admin only) with cursor pagination.
func (h *ProfilesHandler) ListProfiles(
	ctx context.Context,
	req *connect.Request[profilesv1.ListProfilesRequest],
) (*connect.Response[profilesv1.ListProfilesResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	cursor := req.Msg.GetCursor()
	limit := int(req.Msg.GetLimit())

	page, err := h.profileService.ListProfiles(ctx, user.ID, cursor, limit)
	if err != nil {
		if errors.Is(err, profile.ErrPermissionDenied) {
			return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("admin access required"))
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to list profiles: %w", err))
	}

	protoProfiles := make([]*profilesv1.Profile, 0, len(page.Profiles))
	for _, p := range page.Profiles {
		protoProfiles = append(protoProfiles, profileToProto(p))
	}

	resp := &profilesv1.ListProfilesResponse{
		Profiles: protoProfiles,
		HasMore:  page.HasMore,
	}
	if page.NextCursor != "" {
		resp.NextCursor = &page.NextCursor
	}

	return connect.NewResponse(resp), nil
}

// SearchProfiles searches user profiles (admin only) with cursor pagination.
func (h *ProfilesHandler) SearchProfiles(
	ctx context.Context,
	req *connect.Request[profilesv1.SearchProfilesRequest],
) (*connect.Response[profilesv1.SearchProfilesResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	query := req.Msg.GetQuery()
	cursor := req.Msg.GetCursor()
	limit := int(req.Msg.GetLimit())

	page, err := h.profileService.SearchProfiles(ctx, user.ID, query, cursor, limit)
	if err != nil {
		if errors.Is(err, profile.ErrPermissionDenied) {
			return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("admin access required"))
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to search profiles: %w", err))
	}

	protoProfiles := make([]*profilesv1.Profile, 0, len(page.Profiles))
	for _, p := range page.Profiles {
		protoProfiles = append(protoProfiles, profileToProto(p))
	}

	resp := &profilesv1.SearchProfilesResponse{
		Profiles: protoProfiles,
		HasMore:  page.HasMore,
	}
	if page.NextCursor != "" {
		resp.NextCursor = &page.NextCursor
	}

	return connect.NewResponse(resp), nil
}

// BlockUser blocks a user (admin only).
func (h *ProfilesHandler) BlockUser(
	ctx context.Context,
	req *connect.Request[profilesv1.BlockUserRequest],
) (*connect.Response[profilesv1.BlockUserResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	updated, err := h.profileService.BlockUser(ctx, user.ID, req.Msg.GetUserId())
	if err != nil {
		if errors.Is(err, profile.ErrPermissionDenied) {
			return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("admin access required"))
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to block user: %w", err))
	}

	return connect.NewResponse(&profilesv1.BlockUserResponse{
		Profile: profileToProto(updated),
	}), nil
}

// UnblockUser unblocks a user (admin only).
func (h *ProfilesHandler) UnblockUser(
	ctx context.Context,
	req *connect.Request[profilesv1.UnblockUserRequest],
) (*connect.Response[profilesv1.UnblockUserResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	updated, err := h.profileService.UnblockUser(ctx, user.ID, req.Msg.GetUserId())
	if err != nil {
		if errors.Is(err, profile.ErrPermissionDenied) {
			return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("admin access required"))
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to unblock user: %w", err))
	}

	return connect.NewResponse(&profilesv1.UnblockUserResponse{
		Profile: profileToProto(updated),
	}), nil
}

// UpdateUserPlan updates a user's subscription plan (admin only).
func (h *ProfilesHandler) UpdateUserPlan(
	ctx context.Context,
	req *connect.Request[profilesv1.UpdateUserPlanRequest],
) (*connect.Response[profilesv1.UpdateUserPlanResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	var plan *string
	var maxAutomations *int32
	var planAccessUntil *string

	if req.Msg.Plan != nil {
		p := req.Msg.GetPlan()
		plan = &p
	}
	if req.Msg.MaxAutomations != nil {
		ma := req.Msg.GetMaxAutomations()
		maxAutomations = &ma
	}
	if req.Msg.PlanAccessUntil != nil {
		pau := req.Msg.GetPlanAccessUntil()
		planAccessUntil = &pau
	}

	updated, err := h.profileService.UpdateUserPlan(ctx, user.ID, req.Msg.GetUserId(), plan, maxAutomations, planAccessUntil)
	if err != nil {
		if errors.Is(err, profile.ErrPermissionDenied) {
			return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("admin access required"))
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update plan: %w", err))
	}

	return connect.NewResponse(&profilesv1.UpdateUserPlanResponse{
		Profile: profileToProto(updated),
	}), nil
}

// ============================================================
// Password Management RPCs
// ============================================================

// RequestPasswordReset initiates password reset flow.
// Always returns success to prevent email enumeration.
func (h *ProfilesHandler) RequestPasswordReset(
	ctx context.Context,
	req *connect.Request[profilesv1.RequestPasswordResetRequest],
) (*connect.Response[profilesv1.RequestPasswordResetResponse], error) {
	ip := getIPFromContext(ctx)
	userAgent := getUserAgentFromContext(ctx)

	err := h.passwordService.RequestPasswordReset(ctx, profile.RequestResetInput{
		Email:     req.Msg.GetEmail(),
		IPAddress: ip,
		UserAgent: userAgent,
	})

	// Always return success (don't reveal if email exists)
	if err != nil {
		// Log internally but don't expose to user
		h.logger.Warn("password reset request failed",
			zap.String("email", req.Msg.GetEmail()),
			zap.Error(err))

		// Rate limit error should be exposed
		if errors.Is(err, profile.ErrRateLimitExceeded) {
			return connect.NewResponse(&profilesv1.RequestPasswordResetResponse{
				Success: false,
				Message: "Muitas tentativas. Por favor, aguarde alguns minutos.",
			}), nil
		}
	}

	return connect.NewResponse(&profilesv1.RequestPasswordResetResponse{
		Success: true,
		Message: "Se o email existir em nossa base, voce recebera instrucoes em breve.",
	}), nil
}

// ValidateResetToken validates a password reset token.
func (h *ProfilesHandler) ValidateResetToken(
	ctx context.Context,
	req *connect.Request[profilesv1.ValidateResetTokenRequest],
) (*connect.Response[profilesv1.ValidateResetTokenResponse], error) {
	result, err := h.passwordService.ValidateResetToken(ctx, req.Msg.GetToken())
	if err != nil {
		h.logger.Error("failed to validate reset token", zap.Error(err))
		return connect.NewResponse(&profilesv1.ValidateResetTokenResponse{
			Valid:  false,
			Reason: strPtr("invalid_token"),
		}), nil
	}

	resp := &profilesv1.ValidateResetTokenResponse{
		Valid: result.Valid,
	}

	if result.Valid {
		resp.Email = &result.Email
	} else {
		resp.Reason = &result.Reason
	}

	return connect.NewResponse(resp), nil
}

// ResetPassword resets password using a token.
func (h *ProfilesHandler) ResetPassword(
	ctx context.Context,
	req *connect.Request[profilesv1.ResetPasswordRequest],
) (*connect.Response[profilesv1.ResetPasswordResponse], error) {
	ip := getIPFromContext(ctx)
	userAgent := getUserAgentFromContext(ctx)

	err := h.passwordService.ResetPassword(ctx, profile.ResetPasswordInput{
		Token:       req.Msg.GetToken(),
		NewPassword: req.Msg.GetNewPassword(),
		IPAddress:   ip,
		UserAgent:   userAgent,
	})

	if err != nil {
		h.logger.Warn("password reset failed",
			zap.Error(err))

		switch {
		case errors.Is(err, profile.ErrInvalidToken):
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("link invalido ou expirado"))
		case errors.Is(err, profile.ErrTokenExpired):
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("link expirado, solicite um novo"))
		case errors.Is(err, profile.ErrTokenUsed):
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("link ja utilizado"))
		case errors.Is(err, profile.ErrPasswordTooWeak):
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("senha deve ter pelo menos 8 caracteres"))
		default:
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("falha ao redefinir senha"))
		}
	}

	return connect.NewResponse(&profilesv1.ResetPasswordResponse{
		Success: true,
	}), nil
}

// ChangePassword changes password for authenticated user.
func (h *ProfilesHandler) ChangePassword(
	ctx context.Context,
	req *connect.Request[profilesv1.ChangePasswordRequest],
) (*connect.Response[profilesv1.ChangePasswordResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	ip := getIPFromContext(ctx)

	err := h.passwordService.ChangePassword(ctx, user.ID, profile.ChangePasswordInput{
		CurrentPassword: req.Msg.GetCurrentPassword(),
		NewPassword:     req.Msg.GetNewPassword(),
		IPAddress:       ip,
	})

	if err != nil {
		h.logger.Warn("password change failed",
			zap.String("userID", user.ID),
			zap.Error(err))

		switch {
		case errors.Is(err, profile.ErrInvalidPassword):
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("senha atual incorreta"))
		case errors.Is(err, profile.ErrPasswordTooWeak):
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("nova senha deve ter pelo menos 8 caracteres"))
		case errors.Is(err, profile.ErrPasswordSameAsOld):
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("nova senha deve ser diferente da atual"))
		case errors.Is(err, profile.ErrProfileNotFound):
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("perfil nao encontrado"))
		default:
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("falha ao alterar senha"))
		}
	}

	return connect.NewResponse(&profilesv1.ChangePasswordResponse{
		Success: true,
	}), nil
}

// ============================================================
// Helper Functions
// ============================================================

func profileToProto(p *models.Profile) *profilesv1.Profile {
	if p == nil {
		return nil
	}

	proto := &profilesv1.Profile{
		Id:        p.ID,
		CreatedAt: p.CreatedAt,
		UpdatedAt: p.UpdatedAt,
	}

	if p.Email != "" {
		proto.Email = &p.Email
	}
	if p.Plan != "" {
		proto.Plan = &p.Plan
	}
	if p.PhoneNumber != nil {
		proto.PhoneNumber = p.PhoneNumber
	}

	// Map billing status to is_blocked
	if p.BillingStatus != nil && *p.BillingStatus == models.BillingStatusSuspended {
		proto.IsBlocked = true
	}

	// Set max automations: DB value first, fallback to plan default
	if p.MaxAutomations != nil {
		ma := int32(*p.MaxAutomations)
		proto.MaxAutomations = &ma
	} else {
		ma := profile.GetMaxAutomationsForPlan(p.Plan)
		proto.MaxAutomations = &ma
	}

	// Map billing dates
	if p.BillingNextPaymentAt != nil {
		s := p.BillingNextPaymentAt.Format(time.RFC3339)
		proto.BillingNextPaymentAt = &s
	}
	if p.PlanAccessUntil != nil {
		s := p.PlanAccessUntil.Format(time.RFC3339)
		proto.PlanAccessUntil = &s
	}

	return proto
}

// getIPFromContext extracts IP address from request context
// getIPFromContext extracts client IP from request context (alias for interceptor function)
func getIPFromContext(ctx context.Context) string {
	return GetClientIPFromContext(ctx)
}

// getUserAgentFromContext extracts user agent from request context (alias for interceptor function)
func getUserAgentFromContext(ctx context.Context) string {
	return GetUserAgentFromContext(ctx)
}

// strPtr creates a pointer to a string
func strPtr(s string) *string {
	return &s
}
