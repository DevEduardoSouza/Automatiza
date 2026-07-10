package connectrpc

import (
	"context"
	"fmt"
	"strings"

	dispatchtextsv1 "automatiza-go/internal/gen/dispatch_texts/v1"
	"automatiza-go/internal/lib/text"
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"

	"connectrpc.com/connect"
	"go.uber.org/zap"
)

// dispatchTextsSpamLogger is an OPTIONAL logger for warn-only spam-phrase linting
// on dispatch-text copies (anti-ban content hygiene). Installed once at startup
// via SetDispatchTextsSpamLogger; nil disables the warning. Kept package-level so
// the handler constructor signature (and its many test call sites) is unchanged.
var dispatchTextsSpamLogger *zap.Logger

// spamLinter flags high-risk phrases; warn-only, never blocks a save.
var spamLinter = text.DefaultSpamLinter()

// SetDispatchTextsSpamLogger installs the spam-phrase warning logger.
func SetDispatchTextsSpamLogger(l *zap.Logger) { dispatchTextsSpamLogger = l }

// warnSpamPhrases logs (never blocks) any high-risk spam phrases found in copies.
func warnSpamPhrases(copies []string) {
	if dispatchTextsSpamLogger == nil {
		return
	}
	for i, c := range copies {
		if hits := spamLinter.Check(c); len(hits) > 0 {
			dispatchTextsSpamLogger.Warn("dispatch copy contains high-risk spam phrases (anti-ban warning)",
				zap.Int("copyIndex", i+1), zap.Strings("phrases", hits))
		}
	}
}

// DispatchTextsHandler implements the dispatch_textsv1connect.DispatchTextsServiceHandler interface.
type DispatchTextsHandler struct {
	settingsRepo repository.SettingsRepository
}

// NewDispatchTextsHandler creates a new dispatch texts handler.
func NewDispatchTextsHandler(settingsRepo repository.SettingsRepository) *DispatchTextsHandler {
	return &DispatchTextsHandler{
		settingsRepo: settingsRepo,
	}
}

// ListDispatchTexts lists all dispatch texts for the authenticated user.
func (h *DispatchTextsHandler) ListDispatchTexts(
	ctx context.Context,
	req *connect.Request[dispatchtextsv1.ListDispatchTextsRequest],
) (*connect.Response[dispatchtextsv1.ListDispatchTextsResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	texts, err := h.settingsRepo.GetDispatchTextsByUser(ctx, user.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get dispatch texts: %w", err))
	}

	protoTexts := make([]*dispatchtextsv1.DispatchText, len(texts))
	for i, t := range texts {
		protoTexts[i] = dispatchTextToProto(&t)
	}

	return connect.NewResponse(&dispatchtextsv1.ListDispatchTextsResponse{
		DispatchTexts: protoTexts,
	}), nil
}

// GetDispatchText gets a specific dispatch text by ID.
func (h *DispatchTextsHandler) GetDispatchText(
	ctx context.Context,
	req *connect.Request[dispatchtextsv1.GetDispatchTextRequest],
) (*connect.Response[dispatchtextsv1.GetDispatchTextResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	text, err := h.settingsRepo.GetDispatchText(ctx, req.Msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get dispatch text: %w", err))
	}
	if text == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("dispatch text not found"))
	}

	// Verify ownership
	if text.UserID != user.ID {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("dispatch text not found"))
	}

	return connect.NewResponse(&dispatchtextsv1.GetDispatchTextResponse{
		DispatchText: dispatchTextToProto(text),
	}), nil
}

// CreateDispatchText creates a new dispatch text template.
func (h *DispatchTextsHandler) CreateDispatchText(
	ctx context.Context,
	req *connect.Request[dispatchtextsv1.CreateDispatchTextRequest],
) (*connect.Response[dispatchtextsv1.CreateDispatchTextResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	// Resolve copies: prefer copies field, fall back to deprecated body
	copies := req.Msg.GetCopies()
	if len(copies) == 0 && req.Msg.GetBody() != "" {
		copies = []string{req.Msg.GetBody()}
	}

	// Validate copies
	if err := validateCopies(copies); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	// Anti-ban content hygiene: warn (never block) on high-risk spam phrases.
	warnSpamPhrases(copies)

	// Default is_active to true if not provided
	isActive := true
	if req.Msg.IsActive != nil {
		isActive = *req.Msg.IsActive
	}

	dt := &models.DispatchText{
		Name:     req.Msg.GetName(),
		Body:     copies[0],
		IsActive: isActive,
		Copies:   copies,
	}

	created, err := h.settingsRepo.CreateDispatchText(ctx, user.ID, dt)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create dispatch text: %w", err))
	}

	return connect.NewResponse(&dispatchtextsv1.CreateDispatchTextResponse{
		DispatchText: dispatchTextToProto(created),
	}), nil
}

// UpdateDispatchText updates an existing dispatch text template.
func (h *DispatchTextsHandler) UpdateDispatchText(
	ctx context.Context,
	req *connect.Request[dispatchtextsv1.UpdateDispatchTextRequest],
) (*connect.Response[dispatchtextsv1.UpdateDispatchTextResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	// Verify ownership
	existing, err := h.settingsRepo.GetDispatchText(ctx, req.Msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get dispatch text: %w", err))
	}
	if existing == nil || existing.UserID != user.ID {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("dispatch text not found"))
	}

	updates := make(map[string]interface{})
	if req.Msg.Name != nil {
		updates["name"] = req.Msg.GetName()
	}
	// Prefer copies over deprecated body
	if len(req.Msg.GetCopies()) > 0 {
		if err := validateCopies(req.Msg.GetCopies()); err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		warnSpamPhrases(req.Msg.GetCopies())
		updates["copies"] = req.Msg.GetCopies()
	} else if req.Msg.Body != nil {
		updates["body"] = req.Msg.GetBody()
	}
	if req.Msg.IsActive != nil {
		updates["is_active"] = *req.Msg.IsActive
	}

	if len(updates) > 0 {
		if err := h.settingsRepo.UpdateDispatchText(ctx, req.Msg.GetId(), updates); err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update dispatch text: %w", err))
		}
	}

	// Fetch updated record
	updated, err := h.settingsRepo.GetDispatchText(ctx, req.Msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get updated dispatch text: %w", err))
	}

	return connect.NewResponse(&dispatchtextsv1.UpdateDispatchTextResponse{
		DispatchText: dispatchTextToProto(updated),
	}), nil
}

// DeleteDispatchText deletes a dispatch text template.
func (h *DispatchTextsHandler) DeleteDispatchText(
	ctx context.Context,
	req *connect.Request[dispatchtextsv1.DeleteDispatchTextRequest],
) (*connect.Response[dispatchtextsv1.DeleteDispatchTextResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	if err := h.settingsRepo.DeleteDispatchText(ctx, req.Msg.GetId(), user.ID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete dispatch text: %w", err))
	}

	return connect.NewResponse(&dispatchtextsv1.DeleteDispatchTextResponse{}), nil
}

// Helper functions

func dispatchTextToProto(dt *models.DispatchText) *dispatchtextsv1.DispatchText {
	if dt == nil {
		return nil
	}

	return &dispatchtextsv1.DispatchText{
		Id:        dt.ID,
		UserId:    dt.UserID,
		Name:      dt.Name,
		Body:      dt.GetBody(),
		IsActive:  dt.IsActive,
		CreatedAt: dt.CreatedAt,
		UpdatedAt: dt.UpdatedAt,
		Copies:    dt.Copies,
	}
}

// validateCopies validates the copies array constraints.
func validateCopies(copies []string) error {
	if len(copies) == 0 {
		return fmt.Errorf("at least 1 copy variation is required")
	}
	if len(copies) > 10 {
		return fmt.Errorf("maximum 10 copy variations allowed, got %d", len(copies))
	}
	for i, c := range copies {
		if len(c) == 0 {
			return fmt.Errorf("copy variation %d is empty", i+1)
		}
		if len(c) > 4000 {
			return fmt.Errorf("copy variation %d exceeds 4000 character limit (%d chars)", i+1, len(c))
		}
		if !containsLinkVariable(c) {
			return fmt.Errorf("copy variation %d must contain {link_afiliado} or {{LINK_AFILIADO}}", i+1)
		}
	}
	return nil
}

// containsLinkVariable checks if the text contains the affiliate link variable.
func containsLinkVariable(text string) bool {
	return strings.Contains(text, "{link_afiliado}") || strings.Contains(text, "{{LINK_AFILIADO}}")
}
