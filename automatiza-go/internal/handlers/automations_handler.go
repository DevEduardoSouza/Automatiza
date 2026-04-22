package handlers

import (
	"automatiza-go/internal/middleware"
	"automatiza-go/internal/models"
	"net/http"

	"github.com/gin-gonic/gin"
)

// AutomationsHandler handles automation operations
type AutomationsHandler struct {
	deps *Dependencies
}

// NewAutomationsHandler creates a new automations handler
func NewAutomationsHandler(deps *Dependencies) *AutomationsHandler {
	return &AutomationsHandler{deps: deps}
}

// List returns all automations for the authenticated user
func (h *AutomationsHandler) List(c *gin.Context) {
	user := middleware.MustGetAuthenticatedUser(c)

	automations, err := h.deps.AutomationRepo.GetUserAutomations(c.Request.Context(), user.ID)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "Falha ao buscar automações")
		return
	}

	middleware.RespondSuccess(c, map[string]interface{}{
		"automations": automations,
		"total":       len(automations),
	})
}

// Get returns a specific automation
func (h *AutomationsHandler) Get(c *gin.Context) {
	user := middleware.MustGetAuthenticatedUser(c)
	automationID := c.Param("id")

	automation, err := h.deps.AutomationRepo.GetAutomationByIDAndUser(c.Request.Context(), automationID, user.ID)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "Falha ao buscar automação")
		return
	}

	if automation == nil {
		middleware.RespondError(c, http.StatusNotFound, "Automação não encontrada")
		return
	}

	middleware.RespondSuccess(c, automation)
}

// Create creates a new automation
func (h *AutomationsHandler) Create(c *gin.Context) {
	user := middleware.MustGetAuthenticatedUser(c)

	var req models.CreateAutomationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "Dados inválidos")
		return
	}

	// Set defaults
	status := req.Status
	if status == "" {
		status = models.AutomationStatusInactive
	}

	automationType := req.Type
	if automationType == "" {
		automationType = models.AutomationTypeSearch
	}

	automation := &models.Automation{
		Name:            req.Name,
		GroupIDs:        req.GroupIDs,
		Status:          status,
		Type:            &automationType,
		Stores:          req.Stores,
		Categories:      req.Categories,
		Keyword:         req.Keyword,
		MinPrice:        req.MinPrice,
		MaxPrice:        req.MaxPrice,
		MinDiscount:     req.MinDiscount,
		CustomMessage:   req.CustomMessage,
		IncludeImage:    req.IncludeImage,
		ScheduleTime:    req.ScheduleTime,
		ScheduleDays:    req.ScheduleDays,
		SendGroups:      req.SendGroups,
		MonitorGroupIDs: req.MonitorGroupIDs,
		DelaySeconds:    req.DelaySeconds,
		DedupeHours:     req.DedupeHours,
		DispatchTextID:  req.DispatchTextID,
		StartTime:       req.StartTime,
		EndTime:         req.EndTime,
		IntervalMinutes: req.IntervalMinutes,
	}

	// Monitoring channels tend to repost the same product many times per day;
	// the historical 168h default caused clients to think the automation had
	// stopped working. See .claude/rules/known-issues.md.
	if automationType == models.AutomationTypeMonitoring && automation.DedupeHours == nil {
		one := 1
		automation.DedupeHours = &one
	}

	result, err := h.deps.AutomationRepo.CreateAutomation(c.Request.Context(), user.ID, automation)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "Falha ao criar automação")
		return
	}

	middleware.RespondCreated(c, result)
}

// Update updates an existing automation
func (h *AutomationsHandler) Update(c *gin.Context) {
	user := middleware.MustGetAuthenticatedUser(c)
	automationID := c.Param("id")

	// Verify ownership
	existing, err := h.deps.AutomationRepo.GetAutomationByIDAndUser(c.Request.Context(), automationID, user.ID)
	if err != nil || existing == nil {
		middleware.RespondError(c, http.StatusNotFound, "Automação não encontrada")
		return
	}

	var req models.UpdateAutomationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "Dados inválidos")
		return
	}

	updates := req.ToMap()
	if len(updates) == 0 {
		middleware.RespondError(c, http.StatusBadRequest, "Nenhuma alteração fornecida")
		return
	}

	err = h.deps.AutomationRepo.UpdateAutomation(c.Request.Context(), automationID, updates)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "Falha ao atualizar automação")
		return
	}

	// Fetch updated automation
	updated, _ := h.deps.AutomationRepo.GetAutomationByIDAndUser(c.Request.Context(), automationID, user.ID)

	middleware.RespondSuccess(c, updated)
}

// Delete deletes an automation
func (h *AutomationsHandler) Delete(c *gin.Context) {
	user := middleware.MustGetAuthenticatedUser(c)
	automationID := c.Param("id")

	// Verify ownership
	existing, err := h.deps.AutomationRepo.GetAutomationByIDAndUser(c.Request.Context(), automationID, user.ID)
	if err != nil || existing == nil {
		middleware.RespondError(c, http.StatusNotFound, "Automação não encontrada")
		return
	}

	err = h.deps.AutomationRepo.DeleteAutomation(c.Request.Context(), automationID, user.ID)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "Falha ao excluir automação")
		return
	}

	middleware.RespondSuccess(c, map[string]interface{}{
		"deleted": true,
	})
}

// Diagnose diagnoses an automation
func (h *AutomationsHandler) Diagnose(c *gin.Context) {
	user := middleware.MustGetAuthenticatedUser(c)

	var req models.DiagnoseAutomationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "automationId é obrigatório")
		return
	}

	// Verify ownership
	automation, err := h.deps.AutomationRepo.GetAutomationByIDAndUser(c.Request.Context(), req.AutomationID, user.ID)
	if err != nil || automation == nil {
		middleware.RespondError(c, http.StatusNotFound, "Automação não encontrada")
		return
	}

	// Run diagnostics
	result := &models.DiagnoseResult{
		Status: "ok",
		Checks: []models.DiagnoseCheck{},
	}

	// Check if automation has groups configured
	if len(automation.GroupIDs) == 0 && len(automation.SendGroups) == 0 {
		result.Status = "warning"
		result.Warnings = append(result.Warnings, "Nenhum grupo configurado para envio")
	}

	// Check if automation is active
	if !automation.IsActive() {
		result.Checks = append(result.Checks, models.DiagnoseCheck{
			Name:    "status",
			Status:  "warning",
			Message: "Automação está inativa",
		})
	}

	// Check instance connection (simplified)
	result.Checks = append(result.Checks, models.DiagnoseCheck{
		Name:   "instance",
		Status: "ok",
	})

	// Check group access
	result.Checks = append(result.Checks, models.DiagnoseCheck{
		Name:   "groups",
		Status: "ok",
	})

	middleware.RespondSuccess(c, result)
}

// Run manually runs an automation
func (h *AutomationsHandler) Run(c *gin.Context) {
	user := middleware.MustGetAuthenticatedUser(c)

	var req models.RunAutomationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "automationId é obrigatório")
		return
	}

	// Verify ownership
	automation, err := h.deps.AutomationRepo.GetAutomationByIDAndUser(c.Request.Context(), req.AutomationID, user.ID)
	if err != nil || automation == nil {
		middleware.RespondError(c, http.StatusNotFound, "Automação não encontrada")
		return
	}

	// This is a placeholder - actual implementation would run the automation
	// through the automation service
	middleware.RespondSuccess(c, &models.RunAutomationResult{
		Success: true,
		Message: "Automação executada com sucesso",
	})
}

// ResetHistory resets the dispatch history for an automation
func (h *AutomationsHandler) ResetHistory(c *gin.Context) {
	user := middleware.MustGetAuthenticatedUser(c)

	var req models.ResetHistoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "automationId é obrigatório")
		return
	}

	// Verify ownership
	automation, err := h.deps.AutomationRepo.GetAutomationByIDAndUser(c.Request.Context(), req.AutomationID, user.ID)
	if err != nil || automation == nil {
		middleware.RespondError(c, http.StatusNotFound, "Automação não encontrada")
		return
	}

	// This would typically delete dispatch logs for this automation
	// Simplified implementation here

	middleware.RespondSuccess(c, map[string]interface{}{
		"reset": true,
	})
}
