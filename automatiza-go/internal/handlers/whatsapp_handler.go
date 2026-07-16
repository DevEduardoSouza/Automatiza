package handlers

import (
	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/lib/quota"
	"automatiza-go/internal/middleware"
	"automatiza-go/internal/models"
	"automatiza-go/internal/workers"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// WhatsAppHandler handles WhatsApp instance operations
type WhatsAppHandler struct {
	deps *Dependencies
}

// NewWhatsAppHandler creates a new WhatsApp handler
func NewWhatsAppHandler(deps *Dependencies) *WhatsAppHandler {
	return &WhatsAppHandler{deps: deps}
}

// CreateInstance creates a new WhatsApp instance
func (h *WhatsAppHandler) CreateInstance(c *gin.Context) {
	user := middleware.MustGetAuthenticatedUser(c)

	var req struct {
		InstanceName string `json:"instanceName"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "Dados inválidos")
		return
	}

	if h.deps.WhatsAppProvider == nil {
		middleware.RespondError(c, http.StatusServiceUnavailable, "WhatsApp provider não configurado")
		return
	}

	if h.deps.InstanceRepo != nil {
		// Same per-account cap the Connect handler applies (profiles.max_instances
		// upgrade, else default). Fails open to the default on a profile read error.
		limit := quota.MaxInstancesPerUser
		if h.deps.ProfileRepo != nil {
			if profile, perr := h.deps.ProfileRepo.GetProfile(c.Request.Context(), user.ID); perr == nil && profile != nil {
				limit = quota.EffectiveMaxInstances(profile.MaxInstances)
			}
		}
		if err := quota.CheckCanCreateInstance(c.Request.Context(), h.deps.InstanceRepo, user.ID, limit); err != nil {
			if errors.Is(err, quota.ErrInstanceQuotaExceeded) {
				middleware.RespondError(c, http.StatusUnprocessableEntity,
					fmt.Sprintf("Limite de %d instâncias atingido. Remova um chip antes de adicionar outro.", limit))
				return
			}
			h.deps.Logger.Error("Failed to check instance quota",
				zap.String("userID", user.ID), zap.Error(err))
			middleware.RespondError(c, http.StatusInternalServerError, "Falha ao verificar limite de instâncias")
			return
		}
	}

	// Generate instance name if not provided
	instanceName := req.InstanceName
	if instanceName == "" {
		instanceName = fmt.Sprintf("user_%s_%s", user.ID[:8], uuid.New().String()[:5])
	}

	session, err := h.deps.WhatsAppProvider.CreateSession(c.Request.Context(), whatsapp.CreateSessionInput{
		SessionName: instanceName,
		WebhookURL:  "", // Will be set by adapter from config
		// WebhookEvents vazio => default do adapter (message.any + ack +
		// session.status + group.join/leave), alinhado ao WHATSAPP_HOOK_EVENTS
		// da produção. A lista explícita antiga ("message", sem fromMe e sem
		// eventos de grupo) fazia TODO chip criado pela UI perder o gatilho
		// fromMe do offer_generate e os joins/leaves de grupo.
	})
	if err != nil {
		h.deps.Logger.Error("Failed to create instance",
			zap.String("userID", user.ID),
			zap.Error(err))
		middleware.RespondError(c, http.StatusInternalServerError, "Falha ao criar instância")
		return
	}

	// Save to database. FriendlyName fica NULL até o usuário renomear: se
	// defaultarmos pra instanceName, a UI mostra "session_01krk9..." em vez
	// do telefone, porque o front prioriza friendly_name. UpsertInstance
	// COALESCE preserva renomeações existentes em caso de re-pareamento.
	_, err = h.deps.InstanceRepo.UpsertInstance(c.Request.Context(), user.ID, &models.Instance{
		InstanceID: instanceName,
		Status:     "pending",
	})

	if err != nil {
		h.deps.Logger.Error("Failed to save instance",
			zap.String("userID", user.ID),
			zap.Error(err))
	}

	// Get QR code
	response := map[string]interface{}{
		"instanceName": instanceName,
		"friendlyName": instanceName,
		"status":       string(session.Status),
	}

	qrData, err := h.deps.WhatsAppProvider.GetQRCode(c.Request.Context(), instanceName)
	if err == nil && qrData != nil {
		response["qrcode"] = &models.QRCode{Base64: qrData.Base64Image}
	}

	middleware.RespondSuccess(c, response)
}

// DeleteInstance deletes a WhatsApp instance
func (h *WhatsAppHandler) DeleteInstance(c *gin.Context) {
	user := middleware.MustGetAuthenticatedUser(c)

	var req struct {
		InstanceName string `json:"instanceName" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "instanceName é obrigatório")
		return
	}

	// Validate instance ownership
	instance, err := h.deps.InstanceRepo.GetInstanceByUserAndName(c.Request.Context(), user.ID, req.InstanceName)
	if err != nil || instance == nil {
		middleware.RespondError(c, http.StatusNotFound, "Instância não encontrada")
		return
	}

	// Delete from WhatsApp provider
	if h.deps.WhatsAppProvider != nil {
		if err := h.deps.WhatsAppProvider.DeleteSession(c.Request.Context(), req.InstanceName); err != nil {
			h.deps.Logger.Warn("Failed to delete instance",
				zap.String("instanceName", req.InstanceName),
				zap.Error(err))
		}
	}

	// Delete from database
	if err := h.deps.InstanceRepo.DeleteInstance(c.Request.Context(), instance.ID); err != nil {
		h.deps.Logger.Error("Failed to delete instance from database",
			zap.Int("instanceID", instance.ID),
			zap.Error(err))
	}

	// Delete associated groups
	if err := h.deps.GroupRepo.DeleteGroupsByInstance(c.Request.Context(), user.ID, instance.ID); err != nil {
		h.deps.Logger.Warn("Failed to delete groups",
			zap.Int("instanceID", instance.ID),
			zap.Error(err))
	}

	middleware.RespondSuccess(c, map[string]interface{}{
		"deleted": true,
	})
}

// ReconnectInstance reconnects a WhatsApp instance
func (h *WhatsAppHandler) ReconnectInstance(c *gin.Context) {
	user := middleware.MustGetAuthenticatedUser(c)

	var req struct {
		InstanceName string `json:"instanceName" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "instanceName é obrigatório")
		return
	}

	// Validate instance ownership
	instance, err := h.deps.InstanceRepo.GetInstanceByUserAndName(c.Request.Context(), user.ID, req.InstanceName)
	if err != nil || instance == nil {
		middleware.RespondError(c, http.StatusNotFound, "Instância não encontrada")
		return
	}

	if h.deps.WhatsAppProvider == nil {
		middleware.RespondError(c, http.StatusServiceUnavailable, "WhatsApp provider não configurado")
		return
	}

	if err := h.deps.WhatsAppProvider.RestartSession(c.Request.Context(), req.InstanceName); err != nil {
		h.deps.Logger.Error("Failed to reconnect instance",
			zap.String("instanceName", req.InstanceName),
			zap.Error(err))
		middleware.RespondError(c, http.StatusInternalServerError, "Falha ao reconectar instância")
		return
	}

	response := map[string]interface{}{
		"instanceName": req.InstanceName,
		"status":       "reconnecting",
	}

	// Get QR code after restart
	qrData, err := h.deps.WhatsAppProvider.GetQRCode(c.Request.Context(), req.InstanceName)
	if err == nil && qrData != nil {
		response["qrcode"] = &models.QRCode{Base64: qrData.Base64Image}
	}

	// Update status in database
	_ = h.deps.InstanceRepo.UpdateInstanceStatus(c.Request.Context(), instance.ID, "pending")

	middleware.RespondSuccess(c, response)
}

// CheckStatus checks the status of a WhatsApp instance
func (h *WhatsAppHandler) CheckStatus(c *gin.Context) {
	user := middleware.MustGetAuthenticatedUser(c)

	instanceName := c.Query("instanceName")
	if instanceName == "" {
		middleware.RespondError(c, http.StatusBadRequest, "instanceName é obrigatório")
		return
	}

	// Validate instance ownership
	instance, err := h.deps.InstanceRepo.GetInstanceByUserAndName(c.Request.Context(), user.ID, instanceName)
	if err != nil || instance == nil {
		middleware.RespondError(c, http.StatusNotFound, "Instância não encontrada")
		return
	}

	if h.deps.WhatsAppProvider == nil {
		middleware.RespondError(c, http.StatusServiceUnavailable, "WhatsApp provider não configurado")
		return
	}

	var status string
	var isConnected bool

	sessionInfo, err := h.deps.WhatsAppProvider.GetSessionStatus(c.Request.Context(), instanceName)
	if err != nil {
		h.deps.Logger.Warn("Failed to check status",
			zap.String("instanceName", instanceName),
			zap.Error(err))
		status = "unknown"
	} else {
		status = string(sessionInfo.Status)
		isConnected = sessionInfo.Status == whatsapp.SessionStatusConnected
	}

	// Map status to our status
	dbStatus := mapWahaStatusToInternal(status)
	if dbStatus != instance.Status {
		_ = h.deps.InstanceRepo.UpdateInstanceStatus(c.Request.Context(), instance.ID, dbStatus)
	}

	response := map[string]interface{}{
		"instanceName": instanceName,
		"status":       status,
		"connected":    isConnected,
	}

	// Get QR code if not connected
	if !isConnected {
		qrData, _ := h.deps.WhatsAppProvider.GetQRCode(c.Request.Context(), instanceName)
		if qrData != nil {
			response["qrcode"] = &models.QRCode{Base64: qrData.Base64Image}
		}
	}

	middleware.RespondSuccess(c, response)
}

// FetchGroups fetches groups from WhatsApp
func (h *WhatsAppHandler) FetchGroups(c *gin.Context) {
	user := middleware.MustGetAuthenticatedUser(c)

	instanceName := c.Query("instanceName")
	if instanceName == "" {
		middleware.RespondError(c, http.StatusBadRequest, "instanceName é obrigatório")
		return
	}

	// Validate instance ownership
	instance, err := h.deps.InstanceRepo.GetInstanceByUserAndName(c.Request.Context(), user.ID, instanceName)
	if err != nil || instance == nil {
		middleware.RespondError(c, http.StatusNotFound, "Instância não encontrada")
		return
	}

	if h.deps.WhatsAppProvider == nil {
		middleware.RespondError(c, http.StatusServiceUnavailable, "WhatsApp provider não configurado")
		return
	}

	groups, err := h.deps.WhatsAppProvider.GetGroups(c.Request.Context(), instanceName)
	if err != nil {
		h.deps.Logger.Error("Failed to fetch groups",
			zap.String("instanceName", instanceName),
			zap.Error(err))
		middleware.RespondError(c, http.StatusInternalServerError, "Falha ao buscar grupos")
		return
	}

	// Convert to response format
	responseGroups := make([]map[string]interface{}, len(groups))
	for i, g := range groups {
		responseGroups[i] = map[string]interface{}{
			"id":          g.ID,
			"subject":     g.Subject,
			"description": g.Description,
			"owner":       g.Owner,
			"size":        g.Size,
			"announce":    g.Announce,
			"restrict":    g.Restrict,
		}
	}

	middleware.RespondSuccess(c, map[string]interface{}{
		"groups": responseGroups,
		"total":  len(groups),
	})
}

// SyncGroups synchronizes groups to the database
func (h *WhatsAppHandler) SyncGroups(c *gin.Context) {
	user := middleware.MustGetAuthenticatedUser(c)

	var req models.GroupSyncRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "instanceName é obrigatório")
		return
	}

	// Validate instance ownership
	instance, err := h.deps.InstanceRepo.GetInstanceByUserAndName(c.Request.Context(), user.ID, req.InstanceName)
	if err != nil || instance == nil {
		middleware.RespondError(c, http.StatusNotFound, "Instância não encontrada")
		return
	}

	// Enqueue sync job
	_, err = h.deps.QueueClient.EnqueueGroupsSync(c.Request.Context(), &workers.GroupsSyncJobData{
		UserID:       user.ID,
		InstanceName: req.InstanceName,
		InstanceID:   instance.ID,
	})

	if err != nil {
		h.deps.Logger.Error("Failed to enqueue groups sync",
			zap.String("instanceName", req.InstanceName),
			zap.Error(err))
		middleware.RespondError(c, http.StatusInternalServerError, "Falha ao iniciar sincronização")
		return
	}

	middleware.RespondSuccess(c, map[string]interface{}{
		"status":  "processing",
		"message": "Sincronização iniciada",
	})
}

// SyncGroupsStatus returns the status of a groups sync job
func (h *WhatsAppHandler) SyncGroupsStatus(c *gin.Context) {
	// This is a simplified implementation
	// In production, you'd track the job status in the database or Redis
	middleware.RespondSuccess(c, map[string]interface{}{
		"status": "completed",
	})
}

// FetchGroupParticipants fetches participants of a group
func (h *WhatsAppHandler) FetchGroupParticipants(c *gin.Context) {
	user := middleware.MustGetAuthenticatedUser(c)

	instanceName := c.Query("instanceName")
	groupID := c.Query("groupId")

	if instanceName == "" || groupID == "" {
		middleware.RespondError(c, http.StatusBadRequest, "instanceName e groupId são obrigatórios")
		return
	}

	// Validate instance ownership
	instance, err := h.deps.InstanceRepo.GetInstanceByUserAndName(c.Request.Context(), user.ID, instanceName)
	if err != nil || instance == nil {
		middleware.RespondError(c, http.StatusNotFound, "Instância não encontrada")
		return
	}

	if h.deps.WhatsAppProvider == nil {
		middleware.RespondError(c, http.StatusServiceUnavailable, "WhatsApp provider não configurado")
		return
	}

	participants, err := h.deps.WhatsAppProvider.GetGroupParticipants(c.Request.Context(), instanceName, groupID)
	if err != nil {
		h.deps.Logger.Error("Failed to fetch participants",
			zap.String("instanceName", instanceName),
			zap.String("groupID", groupID),
			zap.Error(err))
		middleware.RespondError(c, http.StatusInternalServerError, "Falha ao buscar participants")
		return
	}

	// Convert to Contact format
	contacts := make([]models.Contact, len(participants))
	for i, p := range participants {
		isAdmin := p.IsAdmin || p.IsSuperAdmin
		contacts[i] = models.Contact{
			ID:      p.ID,
			IsAdmin: &isAdmin,
		}
	}

	middleware.RespondSuccess(c, map[string]interface{}{
		"participants": contacts,
		"total":        len(contacts),
	})
}

// SendMessage sends a message to a group
func (h *WhatsAppHandler) SendMessage(c *gin.Context) {
	user := middleware.MustGetAuthenticatedUser(c)

	var req models.WhatsAppMessage
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "Dados inválidos")
		return
	}

	// Validate instance ownership
	instance, err := h.deps.InstanceRepo.GetInstanceByUserAndName(c.Request.Context(), user.ID, req.InstanceName)
	if err != nil || instance == nil {
		middleware.RespondError(c, http.StatusForbidden, "Instância não pertence ao usuário")
		return
	}

	// Check backpressure
	canEnqueue, reason, err := h.deps.Backpressure.CanEnqueue(c.Request.Context(), user.ID)
	if err != nil {
		h.deps.Logger.Warn("Backpressure check failed", zap.Error(err))
	} else if !canEnqueue {
		middleware.RespondError(c, http.StatusTooManyRequests, reason)
		return
	}

	// Create dispatch log
	log, err := h.deps.DispatchLogRepo.Create(c.Request.Context(), &models.DispatchLog{
		UserID:  user.ID,
		GroupID: &req.GroupID,
		Status:  models.DispatchLogStatusQueued,
	})

	var dispatchLogID *int
	if err == nil && log != nil {
		dispatchLogID = &log.ID
	}

	// Enqueue message
	jobData := &workers.MessageJobData{
		InstanceName:  req.InstanceName,
		GroupID:       req.GroupID,
		Message:       req.Message,
		Mentions:      req.Mentions,
		DispatchLogID: dispatchLogID,
		UserID:        user.ID,
	}

	if req.ImageURL != nil {
		jobData.ImageURL = req.ImageURL
	}
	if req.ImageBase64 != nil {
		jobData.ImageBase64 = req.ImageBase64
	}

	_, err = h.deps.QueueClient.EnqueueMessage(c.Request.Context(), jobData, 0)
	if err != nil {
		h.deps.Logger.Error("Failed to enqueue message",
			zap.String("instanceName", req.InstanceName),
			zap.Error(err))
		middleware.RespondError(c, http.StatusInternalServerError, "Falha ao enfileirar mensagem")
		return
	}

	// Record backpressure
	_ = h.deps.Backpressure.RecordEnqueue(c.Request.Context(), user.ID)

	middleware.RespondSuccess(c, map[string]interface{}{
		"status":        "queued",
		"dispatchLogId": dispatchLogID,
	})
}

// SendAudio sends an audio message
func (h *WhatsAppHandler) SendAudio(c *gin.Context) {
	user := middleware.MustGetAuthenticatedUser(c)

	var req models.WhatsAppAudioMessage
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "Dados inválidos")
		return
	}

	// Validate instance ownership
	instance, err := h.deps.InstanceRepo.GetInstanceByUserAndName(c.Request.Context(), user.ID, req.InstanceName)
	if err != nil || instance == nil {
		middleware.RespondError(c, http.StatusForbidden, "Instância não pertence ao usuário")
		return
	}

	// Enqueue audio message
	jobData := &workers.MessageJobData{
		InstanceName: req.InstanceName,
		GroupID:      req.GroupID,
		Message:      "", // Audio only
		AudioBase64:  &req.AudioBase64,
		Mimetype:     &req.Mimetype,
		PTT:          req.PTT,
		UserID:       user.ID,
	}

	_, err = h.deps.QueueClient.EnqueueMessage(c.Request.Context(), jobData, 0)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "Falha ao enfileirar áudio")
		return
	}

	middleware.RespondSuccess(c, map[string]interface{}{
		"status": "queued",
	})
}

// SendPoll sends a poll message
func (h *WhatsAppHandler) SendPoll(c *gin.Context) {
	user := middleware.MustGetAuthenticatedUser(c)

	var req models.WhatsAppPollMessage
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "Dados inválidos")
		return
	}

	// Validate instance ownership
	instance, err := h.deps.InstanceRepo.GetInstanceByUserAndName(c.Request.Context(), user.ID, req.InstanceName)
	if err != nil || instance == nil {
		middleware.RespondError(c, http.StatusForbidden, "Instância não pertence ao usuário")
		return
	}

	if h.deps.WhatsAppProvider == nil {
		middleware.RespondError(c, http.StatusServiceUnavailable, "WhatsApp provider não configurado")
		return
	}

	_, err = h.deps.WhatsAppProvider.SendPoll(c.Request.Context(), whatsapp.SendPollInput{
		SessionName:     req.InstanceName,
		ChatID:          req.GroupID,
		Question:        req.Question,
		Options:         req.Options,
		SelectableCount: req.SelectableCount,
	})
	if err != nil {
		h.deps.Logger.Error("Failed to send poll",
			zap.String("instanceName", req.InstanceName),
			zap.Error(err))
		middleware.RespondError(c, http.StatusInternalServerError, "Falha ao enviar enquete")
		return
	}

	middleware.RespondSuccess(c, map[string]interface{}{
		"status": "sent",
	})
}

// UpdateGroupSubject updates a group's subject
func (h *WhatsAppHandler) UpdateGroupSubject(c *gin.Context) {
	user := middleware.MustGetAuthenticatedUser(c)

	var req models.GroupUpdateSubjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "Dados inválidos")
		return
	}

	// Validate instance ownership
	instance, err := h.deps.InstanceRepo.GetInstanceByUserAndName(c.Request.Context(), user.ID, req.InstanceName)
	if err != nil || instance == nil {
		middleware.RespondError(c, http.StatusForbidden, "Instância não pertence ao usuário")
		return
	}

	if h.deps.WhatsAppProvider == nil {
		middleware.RespondError(c, http.StatusServiceUnavailable, "WhatsApp provider não configurado")
		return
	}

	err = h.deps.WhatsAppProvider.UpdateGroupSubject(c.Request.Context(), whatsapp.UpdateGroupSubjectInput{
		SessionName: req.InstanceName,
		GroupID:     req.GroupID,
		Subject:     req.Subject,
	})
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "Falha ao atualizar nome do grupo")
		return
	}

	middleware.RespondSuccess(c, map[string]interface{}{
		"updated": true,
	})
}

// UpdateGroupDescription updates a group's description
func (h *WhatsAppHandler) UpdateGroupDescription(c *gin.Context) {
	user := middleware.MustGetAuthenticatedUser(c)

	var req models.GroupUpdateDescriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "Dados inválidos")
		return
	}

	// Validate instance ownership
	instance, err := h.deps.InstanceRepo.GetInstanceByUserAndName(c.Request.Context(), user.ID, req.InstanceName)
	if err != nil || instance == nil {
		middleware.RespondError(c, http.StatusForbidden, "Instância não pertence ao usuário")
		return
	}

	if h.deps.WhatsAppProvider == nil {
		middleware.RespondError(c, http.StatusServiceUnavailable, "WhatsApp provider não configurado")
		return
	}

	err = h.deps.WhatsAppProvider.UpdateGroupDescription(c.Request.Context(), whatsapp.UpdateGroupDescriptionInput{
		SessionName: req.InstanceName,
		GroupID:     req.GroupID,
		Description: req.Description,
	})
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "Falha ao atualizar descrição do grupo")
		return
	}

	middleware.RespondSuccess(c, map[string]interface{}{
		"updated": true,
	})
}

// UpdateGroupPicture updates a group's picture
func (h *WhatsAppHandler) UpdateGroupPicture(c *gin.Context) {
	user := middleware.MustGetAuthenticatedUser(c)

	var req models.GroupUpdatePictureRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "Dados inválidos")
		return
	}

	// Validate instance ownership
	instance, err := h.deps.InstanceRepo.GetInstanceByUserAndName(c.Request.Context(), user.ID, req.InstanceName)
	if err != nil || instance == nil {
		middleware.RespondError(c, http.StatusForbidden, "Instância não pertence ao usuário")
		return
	}

	if h.deps.WhatsAppProvider == nil {
		middleware.RespondError(c, http.StatusServiceUnavailable, "WhatsApp provider não configurado")
		return
	}

	err = h.deps.WhatsAppProvider.UpdateGroupPicture(c.Request.Context(), whatsapp.UpdateGroupPictureInput{
		SessionName: req.InstanceName,
		GroupID:     req.GroupID,
		ImageBase64: req.ImageBase64,
	})
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "Falha ao atualizar foto do grupo")
		return
	}

	middleware.RespondSuccess(c, map[string]interface{}{
		"updated": true,
	})
}

// UpdateGroupSettings updates a group's settings
func (h *WhatsAppHandler) UpdateGroupSettings(c *gin.Context) {
	user := middleware.MustGetAuthenticatedUser(c)

	var req models.GroupUpdateSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "Dados inválidos")
		return
	}

	// Validate instance ownership
	instance, err := h.deps.InstanceRepo.GetInstanceByUserAndName(c.Request.Context(), user.ID, req.InstanceName)
	if err != nil || instance == nil {
		middleware.RespondError(c, http.StatusForbidden, "Instância não pertence ao usuário")
		return
	}

	if h.deps.WhatsAppProvider == nil {
		middleware.RespondError(c, http.StatusServiceUnavailable, "WhatsApp provider não configurado")
		return
	}

	// Convert action to boolean
	announce := req.Action == "announcement"
	err = h.deps.WhatsAppProvider.SetGroupAnnouncement(c.Request.Context(), whatsapp.SetGroupAnnouncementInput{
		SessionName: req.InstanceName,
		GroupID:     req.GroupID,
		Announce:    announce,
	})
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "Falha ao atualizar configurações do grupo")
		return
	}

	middleware.RespondSuccess(c, map[string]interface{}{
		"updated": true,
	})
}

// Helper functions

func mapWahaStatusToInternal(status string) string {
	switch status {
	case "WORKING", "open", "connected":
		return models.InstanceStatusConnected
	case "STOPPED", "close", "closed":
		return models.InstanceStatusDisconnected
	default:
		return models.InstanceStatusPending
	}
}

var _ = time.Now // import time
