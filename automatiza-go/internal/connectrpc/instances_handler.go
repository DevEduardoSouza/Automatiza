package connectrpc

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"automatiza-go/internal/adapters/whatsapp"
	instancesv1 "automatiza-go/internal/gen/instances/v1"
	"automatiza-go/internal/models"
	"automatiza-go/internal/observability"
	"automatiza-go/internal/repository"

	"connectrpc.com/connect"
	"github.com/getsentry/sentry-go"
	"go.uber.org/zap"
)

// InstancesHandler implements the instancesv1connect.InstancesServiceHandler interface.
type InstancesHandler struct {
	instanceRepo repository.InstanceRepository
	waProvider   whatsapp.WhatsAppProvider
}

// NewInstancesHandler creates a new instances handler.
func NewInstancesHandler(instanceRepo repository.InstanceRepository) *InstancesHandler {
	return &InstancesHandler{
		instanceRepo: instanceRepo,
	}
}

// NewInstancesHandlerWithAdapter creates a new instances handler with WhatsApp adapter support.
func NewInstancesHandlerWithAdapter(
	instanceRepo repository.InstanceRepository,
	waProvider whatsapp.WhatsAppProvider,
) *InstancesHandler {
	return &InstancesHandler{
		instanceRepo: instanceRepo,
		waProvider:   waProvider,
	}
}

// GetLatestInstance returns the most recently updated instance for the user.
func (h *InstancesHandler) GetLatestInstance(
	ctx context.Context,
	req *connect.Request[instancesv1.GetLatestInstanceRequest],
) (*connect.Response[instancesv1.GetLatestInstanceResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	instances, err := h.instanceRepo.GetUserInstances(ctx, user.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get instances: %w", err))
	}

	if len(instances) == 0 {
		return connect.NewResponse(&instancesv1.GetLatestInstanceResponse{
			Instance: nil,
		}), nil
	}

	// Return the first instance (should be sorted by updated_at DESC)
	return connect.NewResponse(&instancesv1.GetLatestInstanceResponse{
		Instance: instanceToProto(&instances[0]),
	}), nil
}

// ListUserInstances returns all instances for the authenticated user.
func (h *InstancesHandler) ListUserInstances(
	ctx context.Context,
	req *connect.Request[instancesv1.ListUserInstancesRequest],
) (*connect.Response[instancesv1.ListUserInstancesResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	instances, err := h.instanceRepo.GetUserInstances(ctx, user.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get instances: %w", err))
	}

	// Backfill phone_number for any connected chip whose row has it empty.
	// Webhook_consumer_worker normally writes this field when a session.status
	// event arrives carrying me.id, but a chip that paired before that path
	// existed (or whose connect event missed me) ends up stuck with an empty
	// phone, and the UI badge falls back to the raw session id. WAHA always
	// has the AuthenticatedUser populated for connected sessions, so a single
	// REST round-trip per affected chip resolves it. Errors are warned and
	// swallowed — we'd rather show the chip without a phone than block the
	// entire list.
	h.backfillMissingPhones(ctx, instances)

	protoInstances := make([]*instancesv1.Instance, len(instances))
	for i, inst := range instances {
		protoInstances[i] = instanceToProto(&inst)
	}

	return connect.NewResponse(&instancesv1.ListUserInstancesResponse{
		Instances: protoInstances,
	}), nil
}

// backfillMissingPhones fills the empty PhoneNumber field on any connected
// instance by reading me.id from WAHA's session status. Mutates `instances`
// in place so the proto serialization picks up the new value, and persists
// the result so subsequent reads don't pay the WAHA hop.
func (h *InstancesHandler) backfillMissingPhones(ctx context.Context, instances []models.Instance) {
	if h.waProvider == nil {
		return
	}
	for i := range instances {
		inst := &instances[i]
		if inst.Status != models.InstanceStatusConnected {
			continue
		}
		if inst.PhoneNumber != nil && *inst.PhoneNumber != "" {
			continue
		}
		session, err := h.waProvider.GetSessionStatus(ctx, inst.InstanceID)
		if err != nil || session == nil || session.AuthenticatedUser == nil {
			if err != nil {
				h.logger().Warn("Failed to fetch WAHA session for phone backfill",
					zap.String("session", inst.InstanceID),
					zap.Error(err))
			}
			continue
		}
		phone := whatsapp.ExtractPhoneFromID(session.AuthenticatedUser.ID)
		if phone == "" {
			continue
		}
		if err := h.instanceRepo.UpdateInstancePhoneNumber(ctx, inst.ID, phone); err != nil {
			observability.LogAndCapture(h.logger(), sentry.LevelWarning,
				"Failed to persist backfilled phone_number", err,
				map[string]string{"module": "instances", "operation": "list_backfill_phone"})
			continue
		}
		inst.PhoneNumber = &phone
	}
}

// logger returns a zap.Logger usable inside this handler. The struct does
// not carry one yet, so we lazily build a no-op logger; replace with a
// constructor-injected logger if more methods need structured logging.
func (h *InstancesHandler) logger() *zap.Logger {
	return zap.NewNop()
}

// UpsertInstanceStatus creates or updates an instance's status.
func (h *InstancesHandler) UpsertInstanceStatus(
	ctx context.Context,
	req *connect.Request[instancesv1.UpsertInstanceStatusRequest],
) (*connect.Response[instancesv1.UpsertInstanceStatusResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	instance := &models.Instance{
		InstanceID: req.Msg.GetInstanceId(),
		Status:     protoInstanceStatusToModel(req.Msg.GetStatus()),
	}

	upserted, err := h.instanceRepo.UpsertInstance(ctx, user.ID, instance)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to upsert instance: %w", err))
	}

	return connect.NewResponse(&instancesv1.UpsertInstanceStatusResponse{
		Instance: instanceToProto(upserted),
	}), nil
}

// CleanupInvalidInstances cleans up invalid connected instances.
func (h *InstancesHandler) CleanupInvalidInstances(
	ctx context.Context,
	req *connect.Request[instancesv1.CleanupInvalidInstancesRequest],
) (*connect.Response[instancesv1.CleanupInvalidInstancesResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	// Get all instances and filter invalid ones
	instances, err := h.instanceRepo.GetUserInstances(ctx, user.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get instances: %w", err))
	}

	var deletedCount int32
	for _, inst := range instances {
		if inst.InstanceID == "" && inst.Status == "connected" {
			if err := h.instanceRepo.DeleteInstance(ctx, inst.ID); err == nil {
				deletedCount++
			}
		}
	}

	return connect.NewResponse(&instancesv1.CleanupInvalidInstancesResponse{
		DeletedCount: deletedCount,
	}), nil
}

// DeleteDisconnectedInstances deletes all disconnected/pending instances.
func (h *InstancesHandler) DeleteDisconnectedInstances(
	ctx context.Context,
	req *connect.Request[instancesv1.DeleteDisconnectedInstancesRequest],
) (*connect.Response[instancesv1.DeleteDisconnectedInstancesResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	instances, err := h.instanceRepo.GetUserInstances(ctx, user.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get instances: %w", err))
	}

	var deletedCount int32
	for _, inst := range instances {
		if inst.Status == "disconnected" || inst.Status == "pending" {
			if err := h.instanceRepo.DeleteInstance(ctx, inst.ID); err == nil {
				deletedCount++
			}
		}
	}

	return connect.NewResponse(&instancesv1.DeleteDisconnectedInstancesResponse{
		DeletedCount: deletedCount,
	}), nil
}

// DeleteInstance deletes a specific instance.
func (h *InstancesHandler) DeleteInstance(
	ctx context.Context,
	req *connect.Request[instancesv1.DeleteInstanceRequest],
) (*connect.Response[instancesv1.DeleteInstanceResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	// Parse instance ID
	instanceID, err := strconv.Atoi(req.Msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid instance ID"))
	}

	// Get instance to verify ownership
	instance, err := h.instanceRepo.GetInstanceByID(ctx, instanceID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get instance: %w", err))
	}
	if instance == nil || instance.UserID != user.ID {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("instance not found"))
	}

	if err := h.instanceRepo.DeleteInstance(ctx, instanceID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete instance: %w", err))
	}

	return connect.NewResponse(&instancesv1.DeleteInstanceResponse{}), nil
}

// UpdateInstanceLabel sets the human-readable friendly_name on an instance.
// Empty value clears the label (the UI then falls back to the WAHA session id).
func (h *InstancesHandler) UpdateInstanceLabel(
	ctx context.Context,
	req *connect.Request[instancesv1.UpdateInstanceLabelRequest],
) (*connect.Response[instancesv1.UpdateInstanceLabelResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	instanceID, err := strconv.Atoi(req.Msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid instance ID"))
	}

	// Ownership check before mutating, mirroring SetDefaultInstance: the SQL
	// also enforces ownership but a 404-style error here is friendlier on
	// cross-user attempts than a generic Internal.
	instance, err := h.instanceRepo.GetInstanceByID(ctx, instanceID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get instance: %w", err))
	}
	if instance == nil || instance.UserID != user.ID {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("instance not found"))
	}

	if err := h.instanceRepo.UpdateInstanceLabel(ctx, user.ID, instanceID, req.Msg.GetFriendlyName()); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update label: %w", err))
	}

	updated, err := h.instanceRepo.GetInstanceByID(ctx, instanceID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to fetch updated instance: %w", err))
	}

	return connect.NewResponse(&instancesv1.UpdateInstanceLabelResponse{
		Instance: instanceToProto(updated),
	}), nil
}

// SetDefaultInstance promotes the given instance to default for the user.
func (h *InstancesHandler) SetDefaultInstance(
	ctx context.Context,
	req *connect.Request[instancesv1.SetDefaultInstanceRequest],
) (*connect.Response[instancesv1.SetDefaultInstanceResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	instanceID, err := strconv.Atoi(req.Msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid instance ID"))
	}

	// Verify ownership before mutating. SetDefaultInstance also enforces
	// ownership at the SQL level, but checking here returns a clean
	// CodeNotFound instead of a generic CodeInternal on cross-user attempts.
	instance, err := h.instanceRepo.GetInstanceByID(ctx, instanceID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get instance: %w", err))
	}
	if instance == nil || instance.UserID != user.ID {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("instance not found"))
	}

	if err := h.instanceRepo.SetDefaultInstance(ctx, user.ID, instanceID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to set default: %w", err))
	}

	updated, err := h.instanceRepo.GetInstanceByID(ctx, instanceID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to fetch updated instance: %w", err))
	}

	return connect.NewResponse(&instancesv1.SetDefaultInstanceResponse{
		Instance: instanceToProto(updated),
	}), nil
}

// Helper functions

func instanceToProto(i *models.Instance) *instancesv1.Instance {
	if i == nil {
		return nil
	}

	return &instancesv1.Instance{
		Id:           strconv.Itoa(i.ID),
		UserId:       i.UserID,
		InstanceId:   i.InstanceID,
		Status:       instanceStatusToProto(i.Status),
		CreatedAt:    i.CreatedAt,
		UpdatedAt:    i.UpdatedAt,
		IsDefault:    i.IsDefault,
		FriendlyName: i.FriendlyName,
		PhoneNumber:  i.PhoneNumber,
	}
}

func instanceStatusToProto(status string) instancesv1.InstanceStatus {
	switch status {
	case models.InstanceStatusConnected:
		return instancesv1.InstanceStatus_INSTANCE_STATUS_CONNECTED
	case models.InstanceStatusDisconnected:
		return instancesv1.InstanceStatus_INSTANCE_STATUS_DISCONNECTED
	case models.InstanceStatusPending:
		return instancesv1.InstanceStatus_INSTANCE_STATUS_PENDING
	case models.InstanceStatusSuperseded:
		return instancesv1.InstanceStatus_INSTANCE_STATUS_SUPERSEDED
	default:
		return instancesv1.InstanceStatus_INSTANCE_STATUS_UNSPECIFIED
	}
}

func protoInstanceStatusToModel(status instancesv1.InstanceStatus) string {
	switch status {
	case instancesv1.InstanceStatus_INSTANCE_STATUS_CONNECTED:
		return models.InstanceStatusConnected
	case instancesv1.InstanceStatus_INSTANCE_STATUS_DISCONNECTED:
		return models.InstanceStatusDisconnected
	case instancesv1.InstanceStatus_INSTANCE_STATUS_PENDING:
		return models.InstanceStatusPending
	case instancesv1.InstanceStatus_INSTANCE_STATUS_SUPERSEDED:
		return models.InstanceStatusSuperseded
	default:
		return ""
	}
}

// CheckInstanceStatus checks the status of a WhatsApp instance connection via WAHA.
func (h *InstancesHandler) CheckInstanceStatus(
	ctx context.Context,
	req *connect.Request[instancesv1.CheckInstanceStatusRequest],
) (*connect.Response[instancesv1.CheckInstanceStatusResponse], error) {
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

	// Check status via WhatsApp provider
	if h.waProvider == nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("WhatsApp provider not configured"))
	}

	sessionInfo, err := h.waProvider.GetSessionStatus(ctx, instanceName)
	if err != nil {
		return connect.NewResponse(&instancesv1.CheckInstanceStatusResponse{
			Success: false,
			State:   "close",
			Status:  "error",
		}), nil
	}

	// Map session status to state
	state := "close"
	status := string(sessionInfo.Status)
	if sessionInfo.Status == whatsapp.SessionStatusConnected {
		state = "open"
	} else if sessionInfo.Status == whatsapp.SessionStatusScanQR {
		state = "connecting"
	}

	return connect.NewResponse(&instancesv1.CheckInstanceStatusResponse{
		Success: true,
		State:   state,
		Status:  status,
	}), nil
}

// ReconnectInstance reconnects a WhatsApp instance, returning QR code if needed.
func (h *InstancesHandler) ReconnectInstance(
	ctx context.Context,
	req *connect.Request[instancesv1.ReconnectInstanceRequest],
) (*connect.Response[instancesv1.ReconnectInstanceResponse], error) {
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

	// Reconnect via WhatsApp provider
	if h.waProvider == nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("WhatsApp provider not configured"))
	}

	// First, try to restart the session
	if err := h.waProvider.RestartSession(ctx, instanceName); err != nil {
		var providerErr *whatsapp.ProviderError
		if errors.As(err, &providerErr) && providerErr.Code == whatsapp.ErrCodeSessionNotFound {
			// Session lost in WAHA (container restart, timeout, etc.) — create a new one
			observability.LogAndCapture(zap.L(), sentry.LevelInfo, "session not found in WAHA, falling back to CreateSession", err, map[string]string{
				"module": "whatsapp", "operation": "reconnect_fallback", "instanceName": instanceName,
			})
			_, createErr := h.waProvider.CreateSession(ctx, whatsapp.CreateSessionInput{
				SessionName: instanceName,
				Engine: whatsapp.EngineConfig{
					StorageMessages: true,
					StorageGroups:   true,
					StorageChats:    true,
				},
			})
			if createErr != nil {
				errMsg := createErr.Error()
				return connect.NewResponse(&instancesv1.ReconnectInstanceResponse{
					Success: false,
					Error:   &errMsg,
				}), nil
			}
		} else {
			errMsg := err.Error()
			return connect.NewResponse(&instancesv1.ReconnectInstanceResponse{
				Success: false,
				Error:   &errMsg,
			}), nil
		}
	}

	// Then, get the QR code
	qrData, err := h.waProvider.GetQRCode(ctx, instanceName)
	if err != nil {
		errMsg := err.Error()
		return connect.NewResponse(&instancesv1.ReconnectInstanceResponse{
			Success: false,
			Error:   &errMsg,
		}), nil
	}

	var qrBase64 *string
	if qrData != nil && qrData.Base64Image != "" {
		qrBase64 = &qrData.Base64Image
	}

	return connect.NewResponse(&instancesv1.ReconnectInstanceResponse{
		Success:       true,
		QrcodeBase64:  qrBase64,
	}), nil
}
