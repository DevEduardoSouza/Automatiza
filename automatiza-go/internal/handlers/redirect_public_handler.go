package handlers

import (
	"automatiza-go/internal/middleware"
	"automatiza-go/internal/models"
	"automatiza-go/internal/services/redirect"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// RedirectPublicHandler handles public redirect operations (no auth required)
type RedirectPublicHandler struct {
	redirectService redirect.Service
	logger          *zap.Logger
}

// NewRedirectPublicHandler creates a new redirect public handler
func NewRedirectPublicHandler(redirectService redirect.Service, logger *zap.Logger) *RedirectPublicHandler {
	return &RedirectPublicHandler{
		redirectService: redirectService,
		logger:          logger,
	}
}

// Process handles POST /api/v1/redirect
// This is a public endpoint that processes redirect requests
// Request body: { "slug": "xxx" } or { "linkId": "xxx" }
// Response: { "success": true, "redirect_url": "https://chat.whatsapp.com/XXX", "group_name": "...", "group_id": "..." }
func (h *RedirectPublicHandler) Process(c *gin.Context) {
	var req models.PublicRedirectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondError(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate that at least one identifier is provided
	if req.Slug == "" && req.LinkID == "" {
		middleware.RespondError(c, http.StatusBadRequest, "slug or linkId is required")
		return
	}

	// Check if redirect service is available
	if h.redirectService == nil {
		h.logger.Error("Redirect service not configured")
		middleware.RespondError(c, http.StatusServiceUnavailable, "Redirect service not configured")
		return
	}

	// Process the redirect
	result, err := h.redirectService.ProcessRedirect(c.Request.Context(), req.Slug, req.LinkID)
	if err != nil {
		h.logger.Error("Failed to process redirect",
			zap.String("slug", req.Slug),
			zap.String("linkID", req.LinkID),
			zap.Error(err))
		middleware.RespondError(c, http.StatusInternalServerError, "Failed to process redirect")
		return
	}

	// Return the result
	response := models.PublicRedirectResponse{
		Success:     result.Success,
		RedirectURL: result.RedirectURL,
		GroupName:   result.GroupName,
		GroupID:     result.GroupID,
		Error:       result.Error,
	}

	if !result.Success {
		// Return 200 with success=false for client-side handling
		c.JSON(http.StatusOK, response)
		return
	}

	c.JSON(http.StatusOK, response)
}
