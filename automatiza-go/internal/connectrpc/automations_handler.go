package connectrpc

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"automatiza-go/internal/config"
	automationsv1 "automatiza-go/internal/gen/automations/v1"
	"automatiza-go/internal/lib/quota"
	"automatiza-go/internal/models"
	"automatiza-go/internal/observability"
	"automatiza-go/internal/repository"
	"automatiza-go/internal/services/automation"
	"automatiza-go/internal/services/jobs"
	sharedErrors "automatiza-go/internal/shared/errors"

	"connectrpc.com/connect"
	"github.com/getsentry/sentry-go"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// AutomationsHandler implements the automationsv1connect.AutomationsServiceHandler interface.
type AutomationsHandler struct {
	automationRepo    repository.AutomationRepository
	dispatchLogRepo   repository.DispatchLogRepository
	groupRepo         repository.GroupRepository
	profileRepo       repository.ProfileRepository
	planRepo          repository.PlanRepository
	automationService automation.Service
	jobsService       jobs.Service
	logger            *zap.Logger
}

// NewAutomationsHandler creates a new automations handler.
// groupRepo is optional: when nil, the per-chip group quota check is skipped
// (callers like older test setups can opt out without touching every fixture).
// profileRepo/planRepo are optional too: when either is nil, the per-plan
// group-limit check is skipped (fail-open).
func NewAutomationsHandler(
	automationRepo repository.AutomationRepository,
	dispatchLogRepo repository.DispatchLogRepository,
	groupRepo repository.GroupRepository,
	profileRepo repository.ProfileRepository,
	planRepo repository.PlanRepository,
	automationService automation.Service,
	jobsService jobs.Service,
	logger *zap.Logger,
) *AutomationsHandler {
	return &AutomationsHandler{
		automationRepo:    automationRepo,
		dispatchLogRepo:   dispatchLogRepo,
		groupRepo:         groupRepo,
		profileRepo:       profileRepo,
		planRepo:          planRepo,
		automationService: automationService,
		jobsService:       jobsService,
		logger:            logger,
	}
}

// validateChipOverrides ensures every chip referenced in a chip_overrides
// map is one of the chips enabled on the automation. Returns nil for empty
// maps. Doesn't validate that the chip is *connected* or that it actually
// participates in the group — that's the dispatcher's call and varies over
// time (a chip dropped from a group still has the override saved). We only
// guarantee the static "you can't route to a chip the automation isn't
// configured to use" invariant.
func validateChipOverrides(overrides map[string]string, instanceIDs []string) error {
	if len(overrides) == 0 {
		return nil
	}
	if len(instanceIDs) < 2 {
		// Override map without 2+ chips is meaningless. Could be a stale
		// payload from before the user cleared InstanceIds — reject so we
		// don't silently store data that the dispatch path won't read.
		return fmt.Errorf("chip_overrides requires at least 2 chips selected on the automation")
	}
	allowed := make(map[string]struct{}, len(instanceIDs))
	for _, id := range instanceIDs {
		allowed[id] = struct{}{}
	}
	for waGroupID, chipID := range overrides {
		if chipID == "" {
			return fmt.Errorf("chip_overrides[%s] is empty", waGroupID)
		}
		if _, ok := allowed[chipID]; !ok {
			return fmt.Errorf("chip_overrides[%s] references chip %s which is not in instance_ids", waGroupID, chipID)
		}
	}
	return nil
}

// validateAutomationGroupQuota looks up which chip owns each waGroupID and
// returns ErrGroupQuotaExceeded when any chip would host more than 20 of
// the automation's groups. Returns nil when the repo is unavailable, the
// list is empty, the feature flag is off, or no chip exceeds the cap.
func (h *AutomationsHandler) validateAutomationGroupQuota(ctx context.Context, userID string, waGroupIDs []string, label string) error {
	// Operator-controlled kill switch: when FEATURE_ENFORCE_GROUP_LIMITS is
	// off (default), skip the per-chip cap entirely. Flip the env var to
	// true to re-enable the WhatsApp-ban-protection limit.
	if !config.GetFeatureFlags().EnforceGroupLimits {
		return nil
	}
	if h.groupRepo == nil || len(waGroupIDs) == 0 {
		return nil
	}
	instMap, err := h.groupRepo.GetInstancesForGroups(ctx, userID, waGroupIDs)
	if err != nil {
		return fmt.Errorf("resolve %s instances: %w", label, err)
	}
	countByInstance := map[int]int{}
	for _, gid := range waGroupIDs {
		if inst, ok := instMap[gid]; ok && inst != nil {
			countByInstance[inst.ID]++
		}
	}
	return quota.CheckAutomationGroupsPerInstance(countByInstance)
}

// validatePlanGroupLimit enforces the per-PLAN total cap: a user may target at
// most plan.group_limit DISTINCT groups across all their active automations
// (destination/send groups). automationID is the automation being created
// ("" for create) or updated; newSend is its resulting send_groups. Returns a
// CodeResourceExhausted connect error only when the operation would push the
// distinct total OVER the limit AND increase it (grandfather: users already
// above the limit keep working and can edit/remove, just not add more).
//
// Fail-open by design: flag off, missing repos, unresolved plan, or any repo
// error → nil (never blocks a legitimate save on infra hiccups). Ships dark.
func (h *AutomationsHandler) validatePlanGroupLimit(ctx context.Context, userID, automationID string, newSend []string) error {
	if !config.GetFeatureFlags().EnforcePlanGroupLimits {
		return nil
	}
	if h.profileRepo == nil || h.planRepo == nil {
		return nil
	}

	profile, err := h.profileRepo.GetProfile(ctx, userID)
	if err != nil || profile == nil || profile.Plan == "" {
		return nil // can't resolve user's plan → grandfather (no limit)
	}
	plan, err := h.planRepo.GetBySlug(ctx, profile.Plan)
	if err != nil || plan == nil || plan.GroupLimit <= 0 {
		return nil // no matching plan / no limit → grandfather
	}
	limit := int(plan.GroupLimit)

	autos, err := h.automationRepo.GetUserAutomations(ctx, userID)
	if err != nil {
		return nil // fail-open on repo error
	}

	if exceeded, after := planGroupLimitExceeded(limit, autos, automationID, newSend); exceeded {
		return connect.NewError(connect.CodeResourceExhausted, fmt.Errorf(
			"Limite de %d grupos do seu plano atingido (você está usando %d). Faça upgrade do plano para adicionar mais grupos.",
			limit, after))
	}
	return nil
}

// planGroupLimitExceeded is the pure counting core of the per-plan group cap.
// It returns whether applying newSend to automationID ("" = create) would push
// the user's DISTINCT destination-group total over `limit` AND increase it
// (grandfather: users already above the limit aren't blocked unless they add
// more). `after` is the resulting distinct total, for the error message.
func planGroupLimitExceeded(limit int, autos []models.Automation, automationID string, newSend []string) (exceeded bool, after int) {
	before := make(map[string]struct{})
	afterSet := make(map[string]struct{})
	for i := range autos {
		a := &autos[i]
		isTarget := automationID != "" && a.ID == automationID
		// Non-target paused automations don't consume the quota. The target is
		// always counted (it's being saved with groups → treat as in use), so
		// editing a paused automation can't sneak past the cap.
		if !isTarget && !a.IsActive() {
			continue
		}
		if isTarget {
			for _, g := range a.GetTargetGroups() {
				before[g] = struct{}{}
			}
			for _, g := range newSend {
				afterSet[g] = struct{}{}
			}
			continue
		}
		for _, g := range a.GetTargetGroups() {
			before[g] = struct{}{}
			afterSet[g] = struct{}{}
		}
	}
	if automationID == "" {
		// Create: the new automation isn't in the repo yet.
		for _, g := range newSend {
			afterSet[g] = struct{}{}
		}
	}
	return len(afterSet) > limit && len(afterSet) > len(before), len(afterSet)
}

// ListAutomations returns all automations for the authenticated user.
func (h *AutomationsHandler) ListAutomations(
	ctx context.Context,
	req *connect.Request[automationsv1.ListAutomationsRequest],
) (*connect.Response[automationsv1.ListAutomationsResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	automations, err := h.automationRepo.GetUserAutomations(ctx, user.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get automations: %w", err))
	}

	protoAutomations := make([]*automationsv1.Automation, len(automations))
	for i, a := range automations {
		protoAutomations[i] = automationToProto(&a)
	}

	return connect.NewResponse(&automationsv1.ListAutomationsResponse{
		Automations: protoAutomations,
	}), nil
}

// GetAutomation returns a specific automation by ID.
func (h *AutomationsHandler) GetAutomation(
	ctx context.Context,
	req *connect.Request[automationsv1.GetAutomationRequest],
) (*connect.Response[automationsv1.GetAutomationResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	automation, err := h.automationRepo.GetAutomationByIDAndUser(ctx, req.Msg.GetId(), user.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get automation: %w", err))
	}
	if automation == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("automation not found"))
	}

	return connect.NewResponse(&automationsv1.GetAutomationResponse{
		Automation: automationToProto(automation),
	}), nil
}

// CreateAutomation creates a new automation.
func (h *AutomationsHandler) CreateAutomation(
	ctx context.Context,
	req *connect.Request[automationsv1.CreateAutomationRequest],
) (*connect.Response[automationsv1.CreateAutomationResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	// Check if user can create more automations
	canCreate, currentCount, maxCount, err := h.automationService.CanCreateAutomation(ctx, user.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to check automation limit: %w", err))
	}

	if !canCreate {
		return nil, connect.NewError(
			connect.CodeResourceExhausted,
			fmt.Errorf("Limite de automacoes atingido (%d/%d). Atualize seu plano para criar mais", currentCount, maxCount),
		)
	}

	automation := protoToAutomationCreate(req.Msg)

	// Manual chip routing per group (050). Validate that every override
	// references a chip enabled on the automation. We do it at the
	// handler level so the user gets InvalidArgument back instead of a
	// 500 from a DB constraint or silent drop.
	if err := validateChipOverrides(automation.ChipOverrides, automation.InstanceIDs); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// Reject when the automation would route more than MaxGroupsPerInstance
	// groups through a single chip — the user's chip would face a fan-out
	// that risks WhatsApp banning the number.
	if err := h.validateAutomationGroupQuota(ctx, user.ID, automation.SendGroups, "send_groups"); err != nil {
		if errors.Is(err, quota.ErrGroupQuotaExceeded) {
			return nil, connect.NewError(connect.CodeResourceExhausted,
				fmt.Errorf("Limite de %d grupos por chip atingido nesta automação. Distribua os grupos entre mais chips ou remova alguns", quota.MaxGroupsPerInstance))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if err := h.validateAutomationGroupQuota(ctx, user.ID, automation.MonitorGroupIDs, "monitor_group_ids"); err != nil {
		if errors.Is(err, quota.ErrGroupQuotaExceeded) {
			return nil, connect.NewError(connect.CodeResourceExhausted,
				fmt.Errorf("Limite de %d grupos por chip atingido no monitoramento. Distribua os grupos entre mais chips", quota.MaxGroupsPerInstance))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Per-plan total group cap (flag-gated; fail-open). Counts distinct
	// destination groups across the user's active automations plus this new one.
	if err := h.validatePlanGroupLimit(ctx, user.ID, "", automation.SendGroups); err != nil {
		return nil, err
	}

	created, err := h.automationRepo.CreateAutomation(ctx, user.ID, automation)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create automation: %w", err))
	}

	return connect.NewResponse(&automationsv1.CreateAutomationResponse{
		Automation: automationToProto(created),
	}), nil
}

// UpdateAutomation updates an existing automation.
func (h *AutomationsHandler) UpdateAutomation(
	ctx context.Context,
	req *connect.Request[automationsv1.UpdateAutomationRequest],
) (*connect.Response[automationsv1.UpdateAutomationResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	// Verify ownership
	existing, err := h.automationRepo.GetAutomationByIDAndUser(ctx, req.Msg.GetId(), user.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get automation: %w", err))
	}
	if existing == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("automation not found"))
	}

	updates := protoToAutomationUpdate(req.Msg)

	// Manual chip routing per group (050). Validate when the update
	// touches chip_overrides. Effective InstanceIDs come from the new
	// update value when present, otherwise fall back to the existing
	// record we already loaded for the ownership check.
	if newOverrides, ok := updates["chip_overrides"].(map[string]string); ok {
		effectiveInstanceIDs := existing.InstanceIDs
		if newInstanceIDs, hasNew := updates["instance_ids"].([]string); hasNew {
			effectiveInstanceIDs = newInstanceIDs
		}
		if err := validateChipOverrides(newOverrides, effectiveInstanceIDs); err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
	}

	// Re-validate per-chip quota only when the relevant list is being changed.
	// When the field stays untouched we trust the existing record (which was
	// validated on its own create/update path).
	if newSend, ok := updates["send_groups"].([]string); ok {
		if err := h.validateAutomationGroupQuota(ctx, user.ID, newSend, "send_groups"); err != nil {
			if errors.Is(err, quota.ErrGroupQuotaExceeded) {
				return nil, connect.NewError(connect.CodeResourceExhausted,
					fmt.Errorf("Limite de %d grupos por chip atingido nesta automação. Distribua os grupos entre mais chips ou remova alguns", quota.MaxGroupsPerInstance))
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		// Per-plan total group cap (flag-gated; fail-open).
		if err := h.validatePlanGroupLimit(ctx, user.ID, req.Msg.GetId(), newSend); err != nil {
			return nil, err
		}
	}
	if newMonitor, ok := updates["monitor_group_ids"].([]string); ok {
		if err := h.validateAutomationGroupQuota(ctx, user.ID, newMonitor, "monitor_group_ids"); err != nil {
			if errors.Is(err, quota.ErrGroupQuotaExceeded) {
				return nil, connect.NewError(connect.CodeResourceExhausted,
					fmt.Errorf("Limite de %d grupos por chip atingido no monitoramento. Distribua os grupos entre mais chips", quota.MaxGroupsPerInstance))
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	if len(updates) > 0 {
		if err := h.automationRepo.UpdateAutomation(ctx, req.Msg.GetId(), updates); err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update automation: %w", err))
		}
	}

	// Fetch updated automation
	updated, err := h.automationRepo.GetAutomationByIDAndUser(ctx, req.Msg.GetId(), user.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get updated automation: %w", err))
	}

	return connect.NewResponse(&automationsv1.UpdateAutomationResponse{
		Automation: automationToProto(updated),
	}), nil
}

// DeleteAutomation deletes an automation.
func (h *AutomationsHandler) DeleteAutomation(
	ctx context.Context,
	req *connect.Request[automationsv1.DeleteAutomationRequest],
) (*connect.Response[automationsv1.DeleteAutomationResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	if err := h.automationRepo.DeleteAutomation(ctx, req.Msg.GetId(), user.ID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete automation: %w", err))
	}

	return connect.NewResponse(&automationsv1.DeleteAutomationResponse{}), nil
}

// DiagnoseAutomation diagnoses an automation to check if it's configured correctly.
func (h *AutomationsHandler) DiagnoseAutomation(
	ctx context.Context,
	req *connect.Request[automationsv1.DiagnoseAutomationRequest],
) (*connect.Response[automationsv1.DiagnoseAutomationResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	automationID := req.Msg.GetAutomationId()
	result, err := h.automationService.DiagnoseAutomation(ctx, automationID, user.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to diagnose automation: %w", err))
	}

	return connect.NewResponse(&automationsv1.DiagnoseAutomationResponse{
		Result: diagnoseResultToProto(result, automationID),
	}), nil
}

// RunAutomation manually runs an automation.
// Executes asynchronously in a goroutine to avoid nginx proxy timeouts.
func (h *AutomationsHandler) RunAutomation(
	ctx context.Context,
	req *connect.Request[automationsv1.RunAutomationRequest],
) (*connect.Response[automationsv1.RunAutomationResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	automationID := req.Msg.GetAutomationId()
	userID := user.ID

	h.logger.Info("RunAutomation RPC received",
		zap.String("automationID", automationID),
		zap.String("userID", userID))

	// Run automation synchronously so the caller gets the real result
	result, err := h.automationService.RunAutomation(ctx, automationID, userID)
	if err != nil {
		// Lock conflict is expected (concurrent execution) — return Aborted, not Internal
		if sharedErrors.IsConflictError(err) {
			h.logger.Warn("RunAutomation lock conflict",
				zap.String("automationID", automationID),
				zap.String("userID", userID))
			return nil, sharedErrors.MapToConnectError(err)
		}

		h.logger.Error("RunAutomation failed",
			zap.String("automationID", automationID),
			zap.String("userID", userID),
			zap.Error(err))
		// Save error to automation for visibility in the UI
		now := time.Now().UTC().Format(time.RFC3339)
		errMsg := err.Error()
		if metaErr := h.automationRepo.UpdateAutomation(ctx, automationID, map[string]interface{}{
			"last_error":    errMsg,
			"last_error_at": now,
		}); metaErr != nil {
			observability.LogAndCapture(h.logger, sentry.LevelWarning, "failed to update automation error metadata", metaErr, map[string]string{
				"module": "automations", "operation": "error_metadata_update", "automation_id": automationID,
			})
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("automation execution failed: %w", err))
	}

	if result != nil {
		h.logger.Info("RunAutomation completed",
			zap.String("automationID", automationID),
			zap.String("message", result.Message),
			zap.Int("messagesSent", result.MessagesSent))
	}

	resp := &automationsv1.RunAutomationResponse{
		Success: result != nil && result.Success,
	}
	if result != nil {
		resp.Message = &result.Message
	}
	return connect.NewResponse(resp), nil
}

// ResetAutomationHistory resets the history of an automation.
func (h *AutomationsHandler) ResetAutomationHistory(
	ctx context.Context,
	req *connect.Request[automationsv1.ResetAutomationHistoryRequest],
) (*connect.Response[automationsv1.ResetAutomationHistoryResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	if err := h.automationService.ResetHistory(ctx, req.Msg.GetAutomationId(), user.ID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to reset history: %w", err))
	}

	return connect.NewResponse(&automationsv1.ResetAutomationHistoryResponse{
		Success: true,
	}), nil
}

// ListDispatchLogs returns dispatch logs for an automation or all user's automations with pagination.
// If automation_id is provided, returns logs for that specific automation (verifying user ownership).
// If automation_id is empty, returns logs from all user's automations.
// Supports optional filters: group_id, status, store, search_url.
func (h *AutomationsHandler) ListDispatchLogs(
	ctx context.Context,
	req *connect.Request[automationsv1.ListDispatchLogsRequest],
) (*connect.Response[automationsv1.ListDispatchLogsResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	// Validate and sanitize pagination params
	limit := int(req.Msg.GetLimit())
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	offset := int(req.Msg.GetOffset())
	if offset < 0 {
		offset = 0
	}

	// Build filters from request
	filters := &models.DispatchLogFilters{}

	// automation_id filter - empty means all user's automations
	if automationID := req.Msg.GetAutomationId(); automationID != "" {
		filters.AutomationID = &automationID
	}

	// Optional filters
	if groupID := req.Msg.GetGroupId(); groupID != "" {
		filters.GroupID = &groupID
	}
	if status := req.Msg.GetStatus(); status != "" {
		filters.Status = &status
	}
	if store := req.Msg.GetStore(); store != "" {
		filters.Store = &store
	}
	if searchURL := req.Msg.GetSearchUrl(); searchURL != "" {
		filters.SearchURL = &searchURL
	}

	logs, totalCount, err := h.dispatchLogRepo.GetUserLogsWithFilters(ctx, user.ID, filters, limit, offset)
	if err != nil {
		// Check if it's an access denied error
		if strings.Contains(err.Error(), "not found or access denied") {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("automation not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get dispatch logs: %w", err))
	}

	// Convert to proto
	protoLogs := make([]*automationsv1.DispatchLog, len(logs))
	for i, log := range logs {
		protoLogs[i] = dispatchLogWithGroupToProto(&log)
	}

	hasMore := offset+len(logs) < int(totalCount)

	return connect.NewResponse(&automationsv1.ListDispatchLogsResponse{
		Logs:       protoLogs,
		TotalCount: int32(totalCount),
		HasMore:    hasMore,
	}), nil
}

// Helper functions

func automationToProto(a *models.Automation) *automationsv1.Automation {
	if a == nil {
		return nil
	}

	proto := &automationsv1.Automation{
		Id:              a.ID,
		Name:            a.Name,
		Mode:            modelModeToProto(a.GetType()),
		Status:          modelAutomationStatusToProto(a.Status),
		SendGroups:      a.GetTargetGroups(),
		MonitorGroupIds: a.MonitorGroupIDs,
		Categories:      a.Categories,
	}

	if a.DispatchTextID != nil {
		proto.DispatchTextId = a.DispatchTextID
	}
	if a.StartTime != nil {
		proto.StartTime = *a.StartTime
	}
	if a.EndTime != nil {
		proto.EndTime = *a.EndTime
	}
	if a.IntervalMinutes != nil {
		proto.IntervalMinutes = int32(*a.IntervalMinutes)
	}
	if a.MinDiscount != nil {
		proto.MinDiscount = a.MinDiscount
	}
	if a.MinPrice != nil {
		proto.MinPrice = a.MinPrice
	}
	if a.MaxPrice != nil {
		proto.MaxPrice = a.MaxPrice
	}

	// Handle texts - use model Texts array first, fallback to Keyword for legacy
	if len(a.Texts) > 0 {
		proto.Texts = a.Texts
	} else if a.Keyword != nil {
		proto.Texts = []string{*a.Keyword}
	}

	// Handle CTAs
	if len(a.CTAs) > 0 {
		proto.Ctas = a.CTAs
	}

	// CTA entity IDs
	if len(a.CTAIDs) > 0 {
		proto.CtaIds = a.CTAIDs
	}

	if a.LastRunAt != nil {
		proto.LastRunAt = a.LastRunAt
	}
	if a.NextRunAt != nil {
		proto.NextRunAt = a.NextRunAt
	}
	if a.DelaySeconds != nil {
		delay := int32(*a.DelaySeconds)
		proto.DelaySeconds = &delay
	}

	// Priority and filter types
	proto.Priority = modelPriorityToProto(a.PriorityType)
	proto.FilterType = modelFilterToProto(a.FilterType)

	// Error tracking
	if a.LastError != nil {
		proto.LastError = a.LastError
	}
	if a.LastErrorAt != nil {
		if t, err := time.Parse(time.RFC3339, *a.LastErrorAt); err == nil {
			proto.LastErrorAt = timestamppb.New(t)
		}
	}

	// Coupon ID
	if a.CouponID != nil {
		proto.CouponId = a.CouponID
	}

	// LIST mode specific fields
	if a.MlLinkListID != nil {
		proto.MlLinkListId = a.MlLinkListID
	}
	if a.ListItemsPerRun != nil {
		val := int32(*a.ListItemsPerRun)
		proto.ListItemsPerRun = &val
	}
	if a.ListIncludeImage != nil {
		proto.ListIncludeImage = a.ListIncludeImage
	}
	if a.ListSendAsImage != nil {
		proto.ListSendAsImage = a.ListSendAsImage
	}

	// Preferred sender chip
	if a.InstanceID != nil {
		proto.InstanceId = a.InstanceID
	}

	// Multi-chip preference (049). Always emit — empty list signals
	// "no preference" to the client.
	proto.InstanceIds = a.InstanceIDs

	// Manual chip routing per group (050). Always emit — empty map
	// means "full auto routing" (dispatch falls through to sticky hash).
	if a.ChipOverrides != nil {
		proto.ChipOverrides = a.ChipOverrides
	} else {
		proto.ChipOverrides = map[string]string{}
	}

	// Coupon rule engine opt-out (default true server-side)
	proto.ApplyCouponRules = a.ApplyCouponRules

	// Auto-repeat de lista LIST mode (default true server-side, migration 052).
	proto.RepeatWhenFinished = a.RepeatWhenFinished

	// Data de agendamento (migration 053). Empty/unset = sem agendamento.
	if a.StartDate != nil && *a.StartDate != "" {
		proto.StartDate = a.StartDate
	}

	// Data de expiração (migration 054). Empty/unset = sem expiração.
	if a.EndDate != nil && *a.EndDate != "" {
		proto.EndDate = a.EndDate
	}

	// Convert stores
	for _, store := range a.Stores {
		proto.Stores = append(proto.Stores, modelStoreToProto(store))
	}

	// Convert monitoring text rules
	if a.MonitoringTextRules != nil {
		proto.MonitoringTextRules = monitoringTextRulesToProto(a.MonitoringTextRules)
	}

	return proto
}

func modelModeToProto(mode string) automationsv1.AutomationMode {
	switch mode {
	case models.AutomationTypeSearch:
		return automationsv1.AutomationMode_AUTOMATION_MODE_SEARCH
	case models.AutomationTypeMonitoring:
		return automationsv1.AutomationMode_AUTOMATION_MODE_MONITOR
	case models.AutomationTypeList:
		return automationsv1.AutomationMode_AUTOMATION_MODE_LIST
	case models.AutomationTypeOfferGenerate:
		return automationsv1.AutomationMode_AUTOMATION_MODE_OFFER_GENERATE
	default:
		return automationsv1.AutomationMode_AUTOMATION_MODE_SEARCH
	}
}

func protoModeToModel(mode automationsv1.AutomationMode) string {
	switch mode {
	case automationsv1.AutomationMode_AUTOMATION_MODE_SEARCH:
		return models.AutomationTypeSearch
	case automationsv1.AutomationMode_AUTOMATION_MODE_MONITOR:
		return models.AutomationTypeMonitoring
	case automationsv1.AutomationMode_AUTOMATION_MODE_LIST:
		return models.AutomationTypeList
	case automationsv1.AutomationMode_AUTOMATION_MODE_OFFER_GENERATE:
		return models.AutomationTypeOfferGenerate
	default:
		return models.AutomationTypeSearch
	}
}

func modelAutomationStatusToProto(status string) automationsv1.AutomationStatus {
	switch status {
	case models.AutomationStatusActive:
		return automationsv1.AutomationStatus_AUTOMATION_STATUS_ACTIVE
	case models.AutomationStatusPaused, models.AutomationStatusInactive:
		return automationsv1.AutomationStatus_AUTOMATION_STATUS_PAUSED
	default:
		return automationsv1.AutomationStatus_AUTOMATION_STATUS_UNSPECIFIED
	}
}

func protoAutomationStatusToModel(status automationsv1.AutomationStatus) string {
	switch status {
	case automationsv1.AutomationStatus_AUTOMATION_STATUS_ACTIVE:
		return models.AutomationStatusActive
	case automationsv1.AutomationStatus_AUTOMATION_STATUS_PAUSED:
		return models.AutomationStatusPaused
	default:
		return models.AutomationStatusInactive
	}
}

func modelStoreToProto(store string) automationsv1.Store {
	switch store {
	case "shopee":
		return automationsv1.Store_STORE_SHOPEE
	case "amazon":
		return automationsv1.Store_STORE_AMAZON
	case "magalu":
		return automationsv1.Store_STORE_MAGALU
	case "mercadolivre":
		return automationsv1.Store_STORE_MERCADO_LIVRE
	case "shein":
		return automationsv1.Store_STORE_SHEIN
	case "aliexpress":
		return automationsv1.Store_STORE_ALIEXPRESS
	case "awin":
		return automationsv1.Store_STORE_AWIN
	default:
		return automationsv1.Store_STORE_UNSPECIFIED
	}
}

func protoStoreToModel(store automationsv1.Store) string {
	switch store {
	case automationsv1.Store_STORE_SHOPEE:
		return "shopee"
	case automationsv1.Store_STORE_AMAZON:
		return "amazon"
	case automationsv1.Store_STORE_MAGALU:
		return "magalu"
	case automationsv1.Store_STORE_MERCADO_LIVRE:
		return "mercadolivre"
	case automationsv1.Store_STORE_SHEIN:
		return "shein"
	case automationsv1.Store_STORE_ALIEXPRESS:
		return "aliexpress"
	case automationsv1.Store_STORE_AWIN:
		return "awin"
	default:
		return ""
	}
}

func modelPriorityToProto(p *string) automationsv1.PriorityType {
	if p == nil {
		return automationsv1.PriorityType_PRIORITY_TYPE_UNSPECIFIED
	}
	switch *p {
	case "discount":
		return automationsv1.PriorityType_PRIORITY_TYPE_DISCOUNT
	case "price":
		return automationsv1.PriorityType_PRIORITY_TYPE_PRICE
	case "commission":
		return automationsv1.PriorityType_PRIORITY_TYPE_COMMISSION
	default:
		return automationsv1.PriorityType_PRIORITY_TYPE_UNSPECIFIED
	}
}

func protoPriorityToModel(p automationsv1.PriorityType) string {
	switch p {
	case automationsv1.PriorityType_PRIORITY_TYPE_DISCOUNT:
		return "discount"
	case automationsv1.PriorityType_PRIORITY_TYPE_PRICE:
		return "price"
	case automationsv1.PriorityType_PRIORITY_TYPE_COMMISSION:
		return "commission"
	default:
		return ""
	}
}

func modelFilterToProto(f *string) automationsv1.FilterType {
	if f == nil {
		return automationsv1.FilterType_FILTER_TYPE_UNSPECIFIED
	}
	switch *f {
	case "light":
		return automationsv1.FilterType_FILTER_TYPE_LIGHT
	case "heavy":
		return automationsv1.FilterType_FILTER_TYPE_HEAVY
	default:
		return automationsv1.FilterType_FILTER_TYPE_UNSPECIFIED
	}
}

func protoFilterToModel(f automationsv1.FilterType) string {
	switch f {
	case automationsv1.FilterType_FILTER_TYPE_LIGHT:
		return "light"
	case automationsv1.FilterType_FILTER_TYPE_HEAVY:
		return "heavy"
	default:
		return ""
	}
}

func monitoringTextRulesToProto(rules *models.MonitoringTextRules) *automationsv1.MonitoringTextRules {
	if rules == nil {
		return nil
	}

	proto := &automationsv1.MonitoringTextRules{
		RemoveTerms:   rules.RemoverTermos,
		Prefix:        rules.Prefixo,
		Suffix:        rules.Sufixo,
		CleanPhones:   rules.LimparTelefones,
		CleanMentions: rules.LimparArrobas,
	}

	for _, sub := range rules.Substituicoes {
		proto.Substitutions = append(proto.Substitutions, &automationsv1.TextSubstitution{
			From: sub.De,
			To:   sub.Para,
		})
	}

	return proto
}

func protoToMonitoringTextRules(rules *automationsv1.MonitoringTextRules) *models.MonitoringTextRules {
	if rules == nil {
		return nil
	}

	model := &models.MonitoringTextRules{
		RemoverTermos:   rules.RemoveTerms,
		Prefixo:         rules.Prefix,
		Sufixo:          rules.Suffix,
		LimparTelefones: rules.CleanPhones,
		LimparArrobas:   rules.CleanMentions,
	}

	for _, sub := range rules.Substitutions {
		model.Substituicoes = append(model.Substituicoes, models.Substituicao{
			De:   sub.From,
			Para: sub.To,
		})
	}

	return model
}

func protoToAutomationCreate(msg *automationsv1.CreateAutomationRequest) *models.Automation {
	mode := protoModeToModel(msg.GetMode())
	status := protoAutomationStatusToModel(msg.GetStatus())

	automation := &models.Automation{
		Name:            msg.GetName(),
		Type:            &mode,
		Status:          status,
		SendGroups:      msg.GetSendGroups(),
		MonitorGroupIDs: msg.GetMonitorGroupIds(),
		Categories:      msg.GetCategories(),
	}

	// Monitoring channels tend to repost the same product many times per day;
	// the historical 168h default caused clients to think the automation had
	// stopped working. See .claude/rules/known-issues.md.
	if mode == models.AutomationTypeMonitoring {
		one := 1
		automation.DedupeHours = &one
	}

	if msg.DispatchTextId != nil {
		automation.DispatchTextID = msg.DispatchTextId
	}
	if msg.GetStartTime() != "" {
		startTime := msg.GetStartTime()
		automation.StartTime = &startTime
	}
	if msg.GetEndTime() != "" {
		endTime := msg.GetEndTime()
		automation.EndTime = &endTime
	}
	if msg.GetIntervalMinutes() > 0 {
		interval := int(msg.GetIntervalMinutes())
		automation.IntervalMinutes = &interval
	}
	if msg.MinDiscount != nil {
		automation.MinDiscount = msg.MinDiscount
	}
	if msg.MinPrice != nil {
		automation.MinPrice = msg.MinPrice
	}
	if msg.MaxPrice != nil {
		automation.MaxPrice = msg.MaxPrice
	}

	// Handle texts - store full array and also set Keyword for legacy compatibility
	if len(msg.GetTexts()) > 0 {
		automation.Texts = msg.GetTexts()
		keyword := msg.GetTexts()[0]
		automation.Keyword = &keyword
	}

	// Handle CTAs
	if len(msg.GetCtas()) > 0 {
		automation.CTAs = msg.GetCtas()
	}

	// CTA entity IDs
	if len(msg.GetCtaIds()) > 0 {
		automation.CTAIDs = msg.GetCtaIds()
	}

	if msg.DelaySeconds != nil {
		delay := int(msg.GetDelaySeconds())
		automation.DelaySeconds = &delay
	}

	// Priority and filter types
	if msg.GetPriority() != automationsv1.PriorityType_PRIORITY_TYPE_UNSPECIFIED {
		pt := protoPriorityToModel(msg.GetPriority())
		automation.PriorityType = &pt
	}
	if msg.GetFilterType() != automationsv1.FilterType_FILTER_TYPE_UNSPECIFIED {
		ft := protoFilterToModel(msg.GetFilterType())
		automation.FilterType = &ft
	}

	// Coupon ID
	if msg.CouponId != nil {
		automation.CouponID = msg.CouponId
	}

	// LIST mode specific fields
	if msg.MlLinkListId != nil {
		automation.MlLinkListID = msg.MlLinkListId
	}
	if msg.ListItemsPerRun != nil {
		val := int(msg.GetListItemsPerRun())
		automation.ListItemsPerRun = &val
	}
	if msg.ListIncludeImage != nil {
		automation.ListIncludeImage = msg.ListIncludeImage
	}
	if msg.ListSendAsImage != nil {
		automation.ListSendAsImage = msg.ListSendAsImage
	}

	// Preferred sender chip — empty string == no preference, so drop it
	// to nil so the repo writes NULL.
	if msg.InstanceId != nil && *msg.InstanceId != "" {
		automation.InstanceID = msg.InstanceId
	}

	// Multi-chip preference (049). Non-empty list overrides the legacy
	// single-chip InstanceID. The repo's CreateAutomation also writes
	// instance_id = instance_ids[0] for rollback compat.
	if len(msg.InstanceIds) > 0 {
		automation.InstanceIDs = msg.InstanceIds
		if automation.InstanceID == nil || *automation.InstanceID == "" {
			first := msg.InstanceIds[0]
			automation.InstanceID = &first
		}
	}

	// Manual chip routing per group (050). Just copy; validation runs
	// at the handler level (CreateAutomation / UpdateAutomation) so we
	// can return InvalidArgument errors to the client.
	if len(msg.ChipOverrides) > 0 {
		automation.ChipOverrides = msg.ChipOverrides
	}

	// Coupon rule engine opt-out — default true when client doesn't send it
	// so create requests behave like the migration default.
	if msg.ApplyCouponRules != nil {
		automation.ApplyCouponRules = *msg.ApplyCouponRules
	} else {
		automation.ApplyCouponRules = true
	}

	// Auto-repeat de lista (default true quando o client não envia).
	if msg.RepeatWhenFinished != nil {
		automation.RepeatWhenFinished = *msg.RepeatWhenFinished
	} else {
		automation.RepeatWhenFinished = true
	}

	// Data de agendamento. Empty string = sem agendamento (column NULL).
	if msg.StartDate != nil && *msg.StartDate != "" {
		automation.StartDate = msg.StartDate
	}

	// Data de expiração. Empty string = sem expiração (column NULL).
	if msg.EndDate != nil && *msg.EndDate != "" {
		automation.EndDate = msg.EndDate
	}

	// Convert stores
	for _, store := range msg.GetStores() {
		if s := protoStoreToModel(store); s != "" {
			automation.Stores = append(automation.Stores, s)
		}
	}

	// Convert monitoring text rules
	if msg.MonitoringTextRules != nil {
		automation.MonitoringTextRules = protoToMonitoringTextRules(msg.MonitoringTextRules)
	}

	return automation
}

func protoToAutomationUpdate(msg *automationsv1.UpdateAutomationRequest) map[string]interface{} {
	updates := make(map[string]interface{})

	if msg.Name != nil {
		updates["name"] = msg.GetName()
	}
	if msg.Mode != nil {
		updates["type"] = protoModeToModel(msg.GetMode())
	}
	if msg.Status != nil {
		updates["status"] = protoAutomationStatusToModel(msg.GetStatus())
	}
	if msg.SendGroups != nil {
		updates["send_groups"] = msg.GetSendGroups()
	}
	if msg.MonitorGroupIds != nil {
		updates["monitor_group_ids"] = msg.GetMonitorGroupIds()
	}
	if msg.Categories != nil {
		updates["categories"] = msg.GetCategories()
	}
	if msg.DispatchTextId != nil {
		updates["dispatch_text_id"] = msg.GetDispatchTextId()
	}
	if msg.StartTime != nil {
		updates["start_time"] = msg.GetStartTime()
	}
	if msg.EndTime != nil {
		updates["end_time"] = msg.GetEndTime()
	}
	if msg.IntervalMinutes != nil {
		updates["interval_minutes"] = msg.GetIntervalMinutes()
	}
	if msg.MinDiscount != nil {
		updates["min_discount"] = msg.GetMinDiscount()
	}
	if msg.MinPrice != nil {
		updates["min_price"] = msg.GetMinPrice()
	}
	if msg.MaxPrice != nil {
		updates["max_price"] = msg.GetMaxPrice()
	}
	if msg.DelaySeconds != nil {
		updates["delay_seconds"] = msg.GetDelaySeconds()
	}

	// Handle texts - update both texts array and keyword for legacy
	if msg.Texts != nil {
		updates["texts"] = msg.GetTexts()
		if len(msg.GetTexts()) > 0 {
			updates["keyword"] = msg.GetTexts()[0]
		}
	}

	// Handle CTAs
	if msg.Ctas != nil {
		updates["ctas"] = msg.GetCtas()
	}

	// CTA entity IDs
	if msg.CtaIds != nil {
		updates["cta_ids"] = msg.GetCtaIds()
	}

	// Priority and filter types
	if msg.Priority != nil {
		pt := protoPriorityToModel(msg.GetPriority())
		updates["priority_type"] = pt
	}
	if msg.FilterType != nil {
		ft := protoFilterToModel(msg.GetFilterType())
		updates["filter_type"] = ft
	}

	// Coupon ID
	if msg.CouponId != nil {
		updates["coupon_id"] = msg.GetCouponId()
	}

	// LIST mode specific fields
	if msg.MlLinkListId != nil {
		updates["ml_link_list_id"] = msg.GetMlLinkListId()
	}
	if msg.ListItemsPerRun != nil {
		updates["list_items_per_run"] = int(msg.GetListItemsPerRun())
	}
	if msg.ListIncludeImage != nil {
		updates["list_include_image"] = msg.GetListIncludeImage()
	}
	if msg.ListSendAsImage != nil {
		updates["list_send_as_image"] = msg.GetListSendAsImage()
	}

	// Preferred sender chip — empty string means "clear the preference"
	// (write NULL to the FK). The repo's UpdateAutomation handles the
	// string→int coercion and the empty-string special case.
	if msg.InstanceId != nil {
		updates["instance_id"] = msg.GetInstanceId()
	}

	// Multi-chip preference (049). Whenever the field is non-nil in the
	// proto, propagate — this includes the empty-list case which clears
	// the preference. Note: proto3 `repeated` doesn't expose presence
	// directly, but the connect-web client always sends the field (even
	// as empty array) when the form intends to write it.
	if msg.InstanceIds != nil {
		updates["instance_ids"] = msg.InstanceIds
		// Sync legacy instance_id for rollback compat: first element if
		// non-empty, NULL if cleared.
		if len(msg.InstanceIds) > 0 {
			updates["instance_id"] = msg.InstanceIds[0]
		} else {
			updates["instance_id"] = ""
		}
	}

	// Manual chip routing per group (050). map<> in proto3 doesn't expose
	// presence, but the connect-web client always sends the field when
	// the form intends to write it — including the empty map (clears all
	// overrides). Validation runs at the handler level so it can call
	// the repo to read the effective InstanceIDs when the request omits
	// instance_ids.
	if msg.ChipOverrides != nil {
		updates["chip_overrides"] = msg.ChipOverrides
	}

	// Coupon rule engine opt-out
	if msg.ApplyCouponRules != nil {
		updates["apply_coupon_rules"] = *msg.ApplyCouponRules
	}

	// Auto-repeat de lista (LIST mode)
	if msg.RepeatWhenFinished != nil {
		updates["repeat_when_finished"] = *msg.RepeatWhenFinished
	}

	// Data de agendamento (LIST mode). Empty string = clear (NULL).
	if msg.StartDate != nil {
		if *msg.StartDate == "" {
			updates["start_date"] = nil
		} else {
			updates["start_date"] = *msg.StartDate
		}
	}

	// Data de expiração. Empty string = clear (NULL).
	if msg.EndDate != nil {
		if *msg.EndDate == "" {
			updates["end_date"] = nil
		} else {
			updates["end_date"] = *msg.EndDate
		}
	}

	// Monitoring text rules
	if msg.MonitoringTextRules != nil {
		updates["monitoring_text_rules"] = protoToMonitoringTextRules(msg.MonitoringTextRules)
	}

	if msg.Stores != nil {
		var stores []string
		for _, store := range msg.GetStores() {
			if s := protoStoreToModel(store); s != "" {
				stores = append(stores, s)
			}
		}
		updates["stores"] = stores
	}

	return updates
}

func diagnoseResultToProto(result *models.DiagnoseResult, automationID string) *automationsv1.DiagnosticResult {
	if result == nil {
		return nil
	}

	proto := &automationsv1.DiagnosticResult{
		AutomationId: automationID,
	}

	// Map internal status to frontend-expected codes
	// Frontend expects "SENT_OK" for success, not "ok"
	var code string
	switch result.Status {
	case "ok":
		code = "SENT_OK"
	case "error":
		code = "ERRO_DIAGNOSTICO"
	case "warning":
		code = "AVISO_DIAGNOSTICO"
	default:
		code = result.Status
	}
	proto.Code = &code

	// Build message from warnings/errors
	if len(result.Errors) > 0 {
		msg := result.Errors[0]
		proto.Message = &msg
		proto.Error = &msg
	} else if len(result.Warnings) > 0 {
		msg := result.Warnings[0]
		proto.Message = &msg
	} else {
		msg := "Diagnostico OK - automacao pronta para execucao"
		proto.Message = &msg
	}

	// Convert checks to steps
	for _, check := range result.Checks {
		msg := check.Message
		proto.Steps = append(proto.Steps, &automationsv1.DiagnosticStep{
			Step:    check.Name,
			Ok:      check.Status == "ok",
			Detail:  check.Message,
			Message: &msg,
		})
	}

	return proto
}

func dispatchLogWithGroupToProto(log *repository.DispatchLogWithGroup) *automationsv1.DispatchLog {
	if log == nil {
		return nil
	}

	proto := &automationsv1.DispatchLog{
		Id:     strconv.Itoa(log.ID),
		Status: log.Status,
	}

	// Automation ID
	if log.AutomationID != nil {
		proto.AutomationId = *log.AutomationID
	}

	// Group ID and name
	if log.GroupID != nil {
		proto.GroupId = *log.GroupID
	}
	if log.GroupName != nil {
		proto.GroupName = *log.GroupName
	}

	// Canonical key (used as offer identifier)
	if log.CanonicalKey != nil {
		proto.CanonicalKey = *log.CanonicalKey
	}

	// Offer title - use automation name or canonical key as fallback
	if log.AutomationName != nil {
		proto.OfferTitle = *log.AutomationName
	} else if log.CanonicalKey != nil {
		proto.OfferTitle = *log.CanonicalKey
	}

	// Store
	if log.Store != nil {
		proto.Store = log.Store
	}

	// Product URLs
	if log.ProductURL != nil {
		proto.ProductUrl = log.ProductURL
		// Also set OfferUrl for backwards compatibility
		proto.OfferUrl = log.ProductURL
	}
	if log.ProductPageURL != nil {
		proto.ProductPageUrl = log.ProductPageURL
		// Fallback OfferUrl if ProductURL is not set
		if proto.OfferUrl == nil {
			proto.OfferUrl = log.ProductPageURL
		}
	}

	// Affiliate link
	if log.AffiliateLink != nil {
		proto.AffiliateLink = log.AffiliateLink
	}

	// Product category
	if log.ProductCategory != nil {
		proto.ProductCategory = log.ProductCategory
	}

	// Product price - map to both OfferPrice and ProductPrice for compatibility
	if log.ProductPrice != nil {
		proto.OfferPrice = log.ProductPrice
		proto.ProductPrice = log.ProductPrice
	}

	// Coupon fields
	if log.CouponID != nil {
		proto.CouponId = log.CouponID
	}
	if log.CouponCode != nil {
		proto.CouponCode = log.CouponCode
	}
	if log.CouponApplied != nil {
		proto.CouponApplied = log.CouponApplied
	}
	if log.CouponSkipReason != nil {
		proto.CouponSkipReason = log.CouponSkipReason
	}

	// Error message
	if log.Error != nil {
		proto.ErrorMessage = log.Error
	}

	// Parse created_at timestamp
	if log.CreatedAt != "" {
		if t, err := time.Parse(time.RFC3339, log.CreatedAt); err == nil {
			proto.CreatedAt = timestamppb.New(t)
		}
	}

	return proto
}
