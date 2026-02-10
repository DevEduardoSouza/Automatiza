package connectrpc

import (
	"context"
	"fmt"
	"time"

	settingsv1 "automatiza-go/internal/gen/settings/v1"
	"automatiza-go/internal/repository"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/structpb"
)

// SettingsHandler implements the settingsv1connect.SettingsServiceHandler interface.
type SettingsHandler struct {
	settingsRepo repository.SettingsRepository
	userRoleRepo repository.UserRoleRepository
}

// NewSettingsHandler creates a new settings handler.
func NewSettingsHandler(settingsRepo repository.SettingsRepository, userRoleRepo repository.UserRoleRepository) *SettingsHandler {
	return &SettingsHandler{
		settingsRepo: settingsRepo,
		userRoleRepo: userRoleRepo,
	}
}

// requireAdmin checks if the user is an admin and returns an error if not.
func (h *SettingsHandler) requireAdmin(ctx context.Context) error {
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

// GetSettings returns settings for the authenticated user.
func (h *SettingsHandler) GetSettings(
	ctx context.Context,
	req *connect.Request[settingsv1.GetSettingsRequest],
) (*connect.Response[settingsv1.GetSettingsResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	keys := req.Msg.GetKeys()
	var settings map[string]interface{}
	var err error

	if len(keys) > 0 {
		settings, err = h.settingsRepo.Get(ctx, keys...)
	} else {
		settings, err = h.settingsRepo.GetAll(ctx)
	}

	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get settings: %w", err))
	}

	protoSettings := make(map[string]*structpb.Value)
	for k, v := range settings {
		protoValue, err := structpb.NewValue(v)
		if err != nil {
			protoValue = structpb.NewStringValue(fmt.Sprintf("%v", v))
		}
		protoSettings[k] = protoValue
	}

	return connect.NewResponse(&settingsv1.GetSettingsResponse{
		Settings: protoSettings,
	}), nil
}

// UpdateSetting updates a specific setting (admin only).
func (h *SettingsHandler) UpdateSetting(
	ctx context.Context,
	req *connect.Request[settingsv1.UpdateSettingRequest],
) (*connect.Response[settingsv1.UpdateSettingResponse], error) {
	if err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}

	key := req.Msg.GetKey()
	value := req.Msg.GetValue()

	// Convert structpb.Value to interface{} for storage
	var goValue interface{}
	if value != nil {
		goValue = value.AsInterface()
	}

	if err := h.settingsRepo.Update(ctx, key, goValue); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update setting: %w", err))
	}

	return connect.NewResponse(&settingsv1.UpdateSettingResponse{
		Setting: &settingsv1.Setting{
			Key:       key,
			Value:     value,
			UpdatedAt: time.Now().UTC().Format(time.RFC3339),
		},
	}), nil
}
