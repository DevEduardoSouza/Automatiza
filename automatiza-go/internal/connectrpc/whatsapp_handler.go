package connectrpc

import (
	"context"
	"errors"
	"fmt"
	"time"

	"automatiza-go/internal/adapters/whatsapp"
	whatsappv1 "automatiza-go/internal/gen/whatsapp/v1"
	"automatiza-go/internal/lib/quota"
	"automatiza-go/internal/models"
	"automatiza-go/internal/observability"
	"automatiza-go/internal/repository"
	"automatiza-go/internal/workers"

	"connectrpc.com/connect"
	"github.com/getsentry/sentry-go"
	"go.uber.org/zap"
)

// WhatsAppHandler implements the whatsappv1connect.WhatsAppServiceHandler interface.
type WhatsAppHandler struct {
	waProvider    whatsapp.WhatsAppProvider
	instanceRepo  repository.InstanceRepository
	syncJobRepo   repository.SyncJobRepository
	profileRepo   repository.ProfileRepository
	queueClient   *workers.QueueClient
	webhookSecret string
}

// NewWhatsAppHandler creates a new WhatsApp handler.
func NewWhatsAppHandler(
	waProvider whatsapp.WhatsAppProvider,
	instanceRepo repository.InstanceRepository,
	syncJobRepo repository.SyncJobRepository,
	profileRepo repository.ProfileRepository,
	queueClient *workers.QueueClient,
	webhookSecret string,
) *WhatsAppHandler {
	return &WhatsAppHandler{
		waProvider:    waProvider,
		instanceRepo:  instanceRepo,
		syncJobRepo:   syncJobRepo,
		profileRepo:   profileRepo,
		queueClient:   queueClient,
		webhookSecret: webhookSecret,
	}
}

// maxInstancesFor resolves the caller's chip cap: the account's
// profiles.max_instances upgrade if it has one, otherwise the default.
// Fails open to the default on any lookup error — a profile read hiccup must
// not block a user from adding a chip they are entitled to.
func (h *WhatsAppHandler) maxInstancesFor(ctx context.Context, userID string) int {
	if h.profileRepo == nil {
		return quota.MaxInstancesPerUser
	}
	profile, err := h.profileRepo.GetProfile(ctx, userID)
	if err != nil || profile == nil {
		return quota.MaxInstancesPerUser
	}
	return quota.EffectiveMaxInstances(profile.MaxInstances)
}

// CreateInstance creates a new WhatsApp instance and returns QR code for pairing.
func (h *WhatsAppHandler) CreateInstance(
	ctx context.Context,
	req *connect.Request[whatsappv1.CreateInstanceRequest],
) (*connect.Response[whatsappv1.CreateInstanceResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	if h.waProvider == nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("WhatsApp provider not configured"))
	}

	if h.instanceRepo != nil {
		limit := h.maxInstancesFor(ctx, user.ID)
		if err := quota.CheckCanCreateInstance(ctx, h.instanceRepo, user.ID, limit); err != nil {
			if errors.Is(err, quota.ErrInstanceQuotaExceeded) {
				return nil, connect.NewError(connect.CodeResourceExhausted,
					fmt.Errorf("limite de %d instâncias atingido. Remova um chip antes de adicionar outro", limit))
			}
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to check instance quota: %w", err))
		}
	}

	sessionInfo, err := h.waProvider.CreateSession(ctx, whatsapp.CreateSessionInput{
		SessionName: req.Msg.GetInstanceName(),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create instance: %w", err))
	}
	if sessionInfo == nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create instance: unexpected nil session info"))
	}

	resp := &whatsappv1.CreateInstanceResponse{
		Success: true,
	}

	instanceName := sessionInfo.Name
	resp.InstanceName = &instanceName

	// Persist the instance to the DB right after WAHA accepted it. Without
	// this row, ListUserInstances will not return the new chip even after
	// the user finishes pairing — the singleton useWhatsApp hook used to
	// upsert later via UpsertInstanceStatus, but the AddChipModal (Fase 3.2)
	// does not, so chips paired through that flow were going dangling on
	// WAHA without ever appearing in "Meus chips".
	if h.instanceRepo != nil {
		friendlyName := sessionInfo.Name
		if _, upsertErr := h.instanceRepo.UpsertInstance(ctx, user.ID, &models.Instance{
			InstanceID:   sessionInfo.Name,
			FriendlyName: &friendlyName,
			Status:       models.InstanceStatusPending,
		}); upsertErr != nil {
			// Persistence failure is logged but not fatal: the WAHA session
			// already exists, and the user can still pair. UpsertInstanceStatus
			// from the polling/SSE flow will retry on the next status update.
			observability.LogAndCapture(zap.L(), sentry.LevelWarning,
				"failed to persist instance after CreateSession", upsertErr,
				map[string]string{
					"module":   "whatsapp-handler",
					"operation": "create-persist",
					"instance": sessionInfo.Name,
				})
		}
	}

	// After creation, WAHA may still be in STARTING status. Poll briefly
	// until the session reaches SCAN_QR so we can return the QR code
	// immediately to the frontend (avoids an extra round-trip).
	status := sessionInfo.Status
	if status == whatsapp.SessionStatusStarting {
		const maxRetries = 5
		const retryDelay = 1500 * time.Millisecond
		for i := 0; i < maxRetries; i++ {
			select {
			case <-ctx.Done():
				return connect.NewResponse(resp), nil
			case <-time.After(retryDelay):
			}
			info, err := h.waProvider.GetSessionStatus(ctx, sessionInfo.Name)
			if err != nil {
				// SQ11: Capture status poll failure during instance creation
				observability.LogAndCapture(zap.L(), sentry.LevelInfo, "failed to poll session status during create", err, map[string]string{
					"module": "whatsapp-handler", "operation": "create-status-poll", "instance": sessionInfo.Name,
				})
				break
			}
			if info == nil {
				break
			}
			status = info.Status
			if status == whatsapp.SessionStatusScanQR || status == whatsapp.SessionStatusConnected {
				break
			}
		}
	}

	// Fetch QR code if session is waiting for QR scan
	if status == whatsapp.SessionStatusScanQR {
		qrData, err := h.waProvider.GetQRCode(ctx, sessionInfo.Name)
		if err != nil {
			// SQ12: Capture QR fetch failure during instance creation
			observability.LogAndCapture(zap.L(), sentry.LevelWarning, "failed to fetch QR code during create", err, map[string]string{
				"module": "whatsapp-handler", "operation": "create-qr-fetch", "instance": sessionInfo.Name,
			})
		} else if qrData != nil && qrData.Base64Image != "" {
			resp.QrcodeBase64 = &qrData.Base64Image
		}
	}

	return connect.NewResponse(resp), nil
}

// DeleteInstance deletes an existing WhatsApp instance.
func (h *WhatsAppHandler) DeleteInstance(
	ctx context.Context,
	req *connect.Request[whatsappv1.DeleteInstanceRequest],
) (*connect.Response[whatsappv1.DeleteInstanceResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	if h.waProvider == nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("WhatsApp provider not configured"))
	}

	// Posse: a sessão só pode ser deletada pelo DONO da instância. Sem esta
	// checagem, qualquer usuário autenticado deletava sessão alheia pelo nome
	// (o guard do waha-router só protege as PAREADAS; stub de QR de outro
	// cliente ficava exposto). Instância sem linha no banco também é negada —
	// este RPC serve ao fluxo da UI, que sempre cria a linha antes.
	instanceName := req.Msg.GetInstanceName()
	if h.instanceRepo != nil {
		inst, err := h.instanceRepo.GetInstanceByUserAndName(ctx, user.ID, instanceName)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to verify instance ownership: %w", err))
		}
		if inst == nil {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("instance not found for this user"))
		}
	}

	// DeleteSession SEM X-Confirm-Unpair, de propósito: este chamador é o
	// fluxo de reciclagem de QR da UI e NUNCA pode desparear um número que
	// acabou de conectar (corrida do contador do modal — o waha-router bloqueia
	// e o número é salvo). Despareamento deliberado é outro caminho (UnpairSession).
	if err := h.waProvider.DeleteSession(ctx, instanceName); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete instance: %w", err))
	}

	return connect.NewResponse(&whatsappv1.DeleteInstanceResponse{
		Success: true,
	}), nil
}

// ReconnectInstance reconnects an existing instance and generates a new QR code.
func (h *WhatsAppHandler) ReconnectInstance(
	ctx context.Context,
	req *connect.Request[whatsappv1.ReconnectInstanceRequest],
) (*connect.Response[whatsappv1.ReconnectInstanceResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	if h.waProvider == nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("WhatsApp provider not configured"))
	}

	instanceName := req.Msg.GetInstanceName()

	// First logout the session to force a new QR code
	if err := h.waProvider.LogoutSession(ctx, instanceName); err != nil {
		// Logout might fail if already disconnected, continue anyway
	}

	// Restart the session
	if err := h.waProvider.RestartSession(ctx, instanceName); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to reconnect instance: %w", err))
	}

	resp := &whatsappv1.ReconnectInstanceResponse{
		Success: true,
	}

	// After restart, wait for session to reach SCAN_QR before fetching QR code
	status := whatsapp.SessionStatusStarting
	const maxRetries = 5
	const retryDelay = 1500 * time.Millisecond
	for i := 0; i < maxRetries; i++ {
		select {
		case <-ctx.Done():
			return connect.NewResponse(resp), nil
		case <-time.After(retryDelay):
		}
		info, err := h.waProvider.GetSessionStatus(ctx, instanceName)
		if err != nil {
			// SQ13: Capture status poll failure during reconnect
			observability.LogAndCapture(zap.L(), sentry.LevelInfo, "failed to poll session status during reconnect", err, map[string]string{
				"module": "whatsapp-handler", "operation": "reconnect-status-poll", "instance": instanceName,
			})
			break
		}
		if info == nil {
			break
		}
		status = info.Status
		if status == whatsapp.SessionStatusScanQR || status == whatsapp.SessionStatusConnected {
			break
		}
	}

	if status == whatsapp.SessionStatusScanQR {
		qrData, err := h.waProvider.GetQRCode(ctx, instanceName)
		if err != nil {
			// SQ14: Capture QR fetch failure during reconnect
			observability.LogAndCapture(zap.L(), sentry.LevelWarning, "failed to fetch QR code during reconnect", err, map[string]string{
				"module": "whatsapp-handler", "operation": "reconnect-qr-fetch", "instance": instanceName,
			})
		} else if qrData != nil && qrData.Base64Image != "" {
			resp.QrcodeBase64 = &qrData.Base64Image
		}
	}

	return connect.NewResponse(resp), nil
}

// CheckStatus checks the current status of a WhatsApp instance.
func (h *WhatsAppHandler) CheckStatus(
	ctx context.Context,
	req *connect.Request[whatsappv1.CheckStatusRequest],
) (*connect.Response[whatsappv1.CheckStatusResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	if h.waProvider == nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("WhatsApp provider not configured"))
	}

	// Clear cached status to ensure fresh data from WAHA (critical for QR refresh)
	h.waProvider.ClearStatusCache(req.Msg.GetInstanceName())

	sessionInfo, err := h.waProvider.GetSessionStatus(ctx, req.Msg.GetInstanceName())
	if err != nil {
		var providerErr *whatsapp.ProviderError
		if errors.As(err, &providerErr) && providerErr.Code == whatsapp.ErrCodeSessionNotFound {
			return connect.NewResponse(&whatsappv1.CheckStatusResponse{
				Success: true,
				State:   whatsappv1.InstanceState_INSTANCE_STATE_CLOSE,
			}), nil
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to check status: %w", err))
	}
	if sessionInfo == nil {
		return connect.NewResponse(&whatsappv1.CheckStatusResponse{
			Success: true,
			State:   whatsappv1.InstanceState_INSTANCE_STATE_UNSPECIFIED,
		}), nil
	}

	resp := &whatsappv1.CheckStatusResponse{
		Success: true,
		State:   waSessionStatusToProto(sessionInfo.Status),
	}

	// Include QR code when session is waiting for QR scan (used by frontend refresh)
	if sessionInfo.Status == whatsapp.SessionStatusScanQR {
		qrData, err := h.waProvider.GetQRCode(ctx, req.Msg.GetInstanceName())
		if err != nil {
			// SQ12: Capture QR fetch failure during status check
			observability.LogAndCapture(zap.L(), sentry.LevelWarning, "failed to fetch QR code during status check", err, map[string]string{
				"module": "whatsapp-handler", "operation": "check-status-qr-fetch", "instance": req.Msg.GetInstanceName(),
			})
		} else if qrData != nil && qrData.Base64Image != "" {
			resp.QrcodeBase64 = &qrData.Base64Image
		}
	}

	return connect.NewResponse(resp), nil
}

// FetchGroups fetches all groups from a WhatsApp instance.
func (h *WhatsAppHandler) FetchGroups(
	ctx context.Context,
	req *connect.Request[whatsappv1.FetchGroupsRequest],
) (*connect.Response[whatsappv1.FetchGroupsResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	if h.waProvider == nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("WhatsApp provider not configured"))
	}

	groups, err := h.waProvider.GetGroups(ctx, req.Msg.GetInstanceName())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to fetch groups: %w", err))
	}

	protoGroups := make([]*whatsappv1.WhatsAppGroup, len(groups))
	for i, g := range groups {
		protoGroups[i] = waGroupInfoToProto(&g)
	}

	return connect.NewResponse(&whatsappv1.FetchGroupsResponse{
		Groups: protoGroups,
	}), nil
}

// FetchGroupParticipants fetches participants from a specific group.
func (h *WhatsAppHandler) FetchGroupParticipants(
	ctx context.Context,
	req *connect.Request[whatsappv1.FetchGroupParticipantsRequest],
) (*connect.Response[whatsappv1.FetchGroupParticipantsResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	if h.waProvider == nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("WhatsApp provider not configured"))
	}

	participants, err := h.waProvider.GetGroupParticipants(ctx, req.Msg.GetInstanceName(), req.Msg.GetGroupId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to fetch participants: %w", err))
	}

	protoParticipants := make([]*whatsappv1.GroupParticipant, len(participants))
	for i, p := range participants {
		protoParticipants[i] = waParticipantToGroupParticipant(&p)
	}

	return connect.NewResponse(&whatsappv1.FetchGroupParticipantsResponse{
		Participants: protoParticipants,
	}), nil
}

// SyncGroups starts an asynchronous group synchronization job.
func (h *WhatsAppHandler) SyncGroups(
	ctx context.Context,
	req *connect.Request[whatsappv1.SyncGroupsRequest],
) (*connect.Response[whatsappv1.SyncGroupsResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	instanceName := req.Msg.GetInstanceName()

	// Get instance to retrieve the instance ID
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

	// Create sync job record for tracking
	if h.syncJobRepo != nil {
		_, createErr := h.syncJobRepo.Create(ctx, &repository.SyncJob{
			JobID:        taskInfo.ID,
			UserID:       user.ID,
			InstanceID:   fmt.Sprintf("%d", instance.ID),
			InstanceName: instanceName,
			State:        repository.SyncJobStatePending,
		})
		if createErr != nil {
			// Log but don't fail - job was already enqueued
			// The worker can still function without tracking
		}
	}

	return connect.NewResponse(&whatsappv1.SyncGroupsResponse{
		Success: true,
		JobId:   taskInfo.ID,
	}), nil
}

// SyncGroupsStatus checks the status of a group synchronization job.
func (h *WhatsAppHandler) SyncGroupsStatus(
	ctx context.Context,
	req *connect.Request[whatsappv1.SyncGroupsStatusRequest],
) (*connect.Response[whatsappv1.SyncGroupsStatusResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	jobID := req.Msg.GetJobId()

	// If no sync job repo configured, fallback to generic response
	if h.syncJobRepo == nil {
		return connect.NewResponse(&whatsappv1.SyncGroupsStatusResponse{
			State: whatsappv1.SyncJobState_SYNC_JOB_STATE_IN_PROGRESS,
		}), nil
	}

	// Query the real job status from database
	job, err := h.syncJobRepo.GetByJobID(ctx, jobID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get job status: %w", err))
	}
	if job == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("job not found"))
	}

	// Validate that the job belongs to the user
	if job.UserID != user.ID {
		return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("job not found"))
	}

	progress := int32(job.Progress)
	resp := &whatsappv1.SyncGroupsStatusResponse{
		State:    mapSyncJobState(job.State),
		Progress: &progress,
	}

	if job.ErrorMessage != "" {
		resp.FailedReason = &job.ErrorMessage
	}

	return connect.NewResponse(resp), nil
}

// mapSyncJobState converts repository state to proto state.
func mapSyncJobState(state string) whatsappv1.SyncJobState {
	switch state {
	case repository.SyncJobStatePending:
		return whatsappv1.SyncJobState_SYNC_JOB_STATE_PENDING
	case repository.SyncJobStateInProgress:
		return whatsappv1.SyncJobState_SYNC_JOB_STATE_IN_PROGRESS
	case repository.SyncJobStateCompleted:
		return whatsappv1.SyncJobState_SYNC_JOB_STATE_COMPLETED
	case repository.SyncJobStateFailed:
		return whatsappv1.SyncJobState_SYNC_JOB_STATE_FAILED
	default:
		return whatsappv1.SyncJobState_SYNC_JOB_STATE_UNSPECIFIED
	}
}

// SendMessage sends a text message to a group (optionally with images and mentions).
func (h *WhatsAppHandler) SendMessage(
	ctx context.Context,
	req *connect.Request[whatsappv1.SendMessageRequest],
) (*connect.Response[whatsappv1.SendMessageResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	if h.waProvider == nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("WhatsApp provider not configured"))
	}

	// Check if images are provided
	images := req.Msg.GetImages()
	if len(images) > 0 {
		// Send message with first image
		_, err := h.waProvider.SendImage(ctx, whatsapp.SendImageInput{
			SessionName: req.Msg.GetInstanceName(),
			ChatID:      req.Msg.GetGroupId(),
			Caption:     req.Msg.GetMessage(),
			ImageURL:    images[0],
		})
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to send message with image: %w", err))
		}
	} else {
		_, err := h.waProvider.SendText(ctx, whatsapp.SendTextInput{
			SessionName: req.Msg.GetInstanceName(),
			ChatID:      req.Msg.GetGroupId(),
			Text:        req.Msg.GetMessage(),
			Mentions:    req.Msg.GetMentions(),
		})
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to send message: %w", err))
		}
	}

	return connect.NewResponse(&whatsappv1.SendMessageResponse{
		Success: true,
	}), nil
}

// SendAudio sends an audio message to a group.
func (h *WhatsAppHandler) SendAudio(
	ctx context.Context,
	req *connect.Request[whatsappv1.SendAudioRequest],
) (*connect.Response[whatsappv1.SendAudioResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	if h.waProvider == nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("WhatsApp provider not configured"))
	}

	mimetype := "audio/ogg"
	if req.Msg.Mimetype != nil {
		mimetype = *req.Msg.Mimetype
	}

	_, err := h.waProvider.SendVoice(ctx, whatsapp.SendVoiceInput{
		SessionName: req.Msg.GetInstanceName(),
		ChatID:      req.Msg.GetGroupId(),
		AudioBase64: req.Msg.GetAudio(),
		Mimetype:    mimetype,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to send audio: %w", err))
	}

	return connect.NewResponse(&whatsappv1.SendAudioResponse{
		Success: true,
	}), nil
}

// SendPoll sends a poll to a group.
func (h *WhatsAppHandler) SendPoll(
	ctx context.Context,
	req *connect.Request[whatsappv1.SendPollRequest],
) (*connect.Response[whatsappv1.SendPollResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	if h.waProvider == nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("WhatsApp provider not configured"))
	}

	selectableCount := 1
	if req.Msg.SelectableCount != nil {
		selectableCount = int(*req.Msg.SelectableCount)
	}

	_, err := h.waProvider.SendPoll(ctx, whatsapp.SendPollInput{
		SessionName:     req.Msg.GetInstanceName(),
		ChatID:          req.Msg.GetGroupId(),
		Question:        req.Msg.GetName(),
		Options:         req.Msg.GetOptions(),
		SelectableCount: selectableCount,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to send poll: %w", err))
	}

	return connect.NewResponse(&whatsappv1.SendPollResponse{
		Success: true,
	}), nil
}

// UpdateGroupSubject updates a group's subject/name.
func (h *WhatsAppHandler) UpdateGroupSubject(
	ctx context.Context,
	req *connect.Request[whatsappv1.UpdateGroupSubjectRequest],
) (*connect.Response[whatsappv1.UpdateGroupSubjectResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	if h.waProvider == nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("WhatsApp provider not configured"))
	}

	err := h.waProvider.UpdateGroupSubject(ctx, whatsapp.UpdateGroupSubjectInput{
		SessionName: req.Msg.GetInstanceName(),
		GroupID:     req.Msg.GetGroupId(),
		Subject:     req.Msg.GetSubject(),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update group subject: %w", err))
	}

	return connect.NewResponse(&whatsappv1.UpdateGroupSubjectResponse{
		Success: true,
	}), nil
}

// UpdateGroupDescription updates a group's description.
func (h *WhatsAppHandler) UpdateGroupDescription(
	ctx context.Context,
	req *connect.Request[whatsappv1.UpdateGroupDescriptionRequest],
) (*connect.Response[whatsappv1.UpdateGroupDescriptionResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	if h.waProvider == nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("WhatsApp provider not configured"))
	}

	err := h.waProvider.UpdateGroupDescription(ctx, whatsapp.UpdateGroupDescriptionInput{
		SessionName: req.Msg.GetInstanceName(),
		GroupID:     req.Msg.GetGroupId(),
		Description: req.Msg.GetDescription(),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update group description: %w", err))
	}

	return connect.NewResponse(&whatsappv1.UpdateGroupDescriptionResponse{
		Success: true,
	}), nil
}

// UpdateGroupPicture updates a group's picture.
func (h *WhatsAppHandler) UpdateGroupPicture(
	ctx context.Context,
	req *connect.Request[whatsappv1.UpdateGroupPictureRequest],
) (*connect.Response[whatsappv1.UpdateGroupPictureResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	if h.waProvider == nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("WhatsApp provider not configured"))
	}

	err := h.waProvider.UpdateGroupPicture(ctx, whatsapp.UpdateGroupPictureInput{
		SessionName: req.Msg.GetInstanceName(),
		GroupID:     req.Msg.GetGroupId(),
		ImageBase64: req.Msg.GetImageUrl(),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update group picture: %w", err))
	}

	return connect.NewResponse(&whatsappv1.UpdateGroupPictureResponse{
		Success: true,
	}), nil
}

// UpdateGroupSettings updates a group's settings (open/close).
func (h *WhatsAppHandler) UpdateGroupSettings(
	ctx context.Context,
	req *connect.Request[whatsappv1.UpdateGroupSettingsRequest],
) (*connect.Response[whatsappv1.UpdateGroupSettingsResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	if h.waProvider == nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("WhatsApp provider not configured"))
	}

	// Convert proto action to announcement boolean
	announce := req.Msg.GetAction() == whatsappv1.GroupSettingsAction_GROUP_SETTINGS_ACTION_ANNOUNCEMENT

	err := h.waProvider.SetGroupAnnouncement(ctx, whatsapp.SetGroupAnnouncementInput{
		SessionName: req.Msg.GetInstanceName(),
		GroupID:     req.Msg.GetGroupId(),
		Announce:    announce,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update group settings: %w", err))
	}

	return connect.NewResponse(&whatsappv1.UpdateGroupSettingsResponse{
		Success: true,
	}), nil
}

// Helper functions for WhatsApp handler

// waSessionStatusToProto converts whatsapp.SessionStatus to proto InstanceState.
func waSessionStatusToProto(status whatsapp.SessionStatus) whatsappv1.InstanceState {
	switch status {
	case whatsapp.SessionStatusConnected:
		return whatsappv1.InstanceState_INSTANCE_STATE_OPEN
	case whatsapp.SessionStatusDisconnected, whatsapp.SessionStatusStopped, whatsapp.SessionStatusFailed:
		return whatsappv1.InstanceState_INSTANCE_STATE_CLOSE
	case whatsapp.SessionStatusStarting, whatsapp.SessionStatusScanQR:
		return whatsappv1.InstanceState_INSTANCE_STATE_CONNECTING
	default:
		return whatsappv1.InstanceState_INSTANCE_STATE_UNSPECIFIED
	}
}

// waGroupInfoToProto converts whatsapp.GroupInfo to proto WhatsAppGroup.
func waGroupInfoToProto(g *whatsapp.GroupInfo) *whatsappv1.WhatsAppGroup {
	if g == nil {
		return nil
	}

	proto := &whatsappv1.WhatsAppGroup{
		Id: g.ID,
	}

	if g.Subject != "" {
		proto.Subject = &g.Subject
	}
	if g.Description != "" {
		proto.Description = &g.Description
	}
	if g.Size > 0 {
		size := int32(g.Size)
		proto.Size = &size
	}

	return proto
}

// waParticipantToGroupParticipant converts whatsapp.ParticipantInfo to proto GroupParticipant.
func waParticipantToGroupParticipant(p *whatsapp.ParticipantInfo) *whatsappv1.GroupParticipant {
	if p == nil {
		return nil
	}

	isAdmin := p.IsAdmin || p.IsSuperAdmin
	proto := &whatsappv1.GroupParticipant{
		Id:      p.ID,
		IsAdmin: &isAdmin,
	}

	return proto
}

// GetGroupInviteCode gets the invite code for a group (requires admin privileges).
func (h *WhatsAppHandler) GetGroupInviteCode(
	ctx context.Context,
	req *connect.Request[whatsappv1.GetGroupInviteCodeRequest],
) (*connect.Response[whatsappv1.GetGroupInviteCodeResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	if h.waProvider == nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("WhatsApp provider not configured"))
	}

	code, err := h.waProvider.GetGroupInviteCode(ctx, req.Msg.GetInstanceName(), req.Msg.GetGroupId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get invite code: %w", err))
	}

	return connect.NewResponse(&whatsappv1.GetGroupInviteCodeResponse{
		InviteCode: code,
		InviteUrl:  fmt.Sprintf("https://chat.whatsapp.com/%s", code),
	}), nil
}

// DownloadMedia downloads media content from a message.
func (h *WhatsAppHandler) DownloadMedia(
	ctx context.Context,
	req *connect.Request[whatsappv1.DownloadMediaRequest],
) (*connect.Response[whatsappv1.DownloadMediaResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	if h.waProvider == nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("WhatsApp provider not configured"))
	}

	result, err := h.waProvider.DownloadMedia(ctx, whatsapp.DownloadMediaInput{
		SessionName: req.Msg.GetInstanceName(),
		MediaID:     req.Msg.GetMediaId(),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to download media: %w", err))
	}

	// Limit response size (16MB max)
	const maxSize = 16 * 1024 * 1024
	if len(result.Data) > maxSize {
		return nil, connect.NewError(connect.CodeResourceExhausted, fmt.Errorf("media too large: %d bytes exceeds 16MB limit", len(result.Data)))
	}

	resp := &whatsappv1.DownloadMediaResponse{
		Data:     result.Data,
		Mimetype: result.Mimetype,
	}
	if result.Filename != "" {
		resp.Filename = &result.Filename
	}

	return connect.NewResponse(resp), nil
}

// SetWebhook configures webhook URL and events for a session.
func (h *WhatsAppHandler) SetWebhook(
	ctx context.Context,
	req *connect.Request[whatsappv1.SetWebhookRequest],
) (*connect.Response[whatsappv1.SetWebhookResponse], error) {
	user := GetUserFromContext(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authentication required"))
	}

	if h.waProvider == nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("WhatsApp provider not configured"))
	}

	err := h.waProvider.SetWebhook(ctx, whatsapp.SetWebhookInput{
		SessionName: req.Msg.GetInstanceName(),
		URL:         req.Msg.GetUrl(),
		Events:      req.Msg.GetEvents(),
		HMACKey:     h.webhookSecret,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to set webhook: %w", err))
	}

	return connect.NewResponse(&whatsappv1.SetWebhookResponse{
		Success: true,
	}), nil
}
