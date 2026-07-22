package handlers

import (
	"net/http"

	"automatiza-go/internal/services/instagram"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// InstagramPublicHandler serves the unauthenticated Instagram endpoints: the
// OAuth callback (no session — validated by the signed state), the ephemeral
// story-media fetch (unguessable id, so no auth), and the Meta-required
// deauthorize / data-deletion stubs.
type InstagramPublicHandler struct {
	svc        *instagram.Service
	mediaStore *instagram.MediaStore
	frontend   string // frontend base URL for post-OAuth redirects
	logger     *zap.Logger
}

// NewInstagramPublicHandler builds the handler from resolved deps.
func NewInstagramPublicHandler(svc *instagram.Service, mediaStore *instagram.MediaStore, frontendBaseURL string, logger *zap.Logger) *InstagramPublicHandler {
	return &InstagramPublicHandler{svc: svc, mediaStore: mediaStore, frontend: frontendBaseURL, logger: logger}
}

// OAuthCallback handles GET /api/v1/instagram/oauth/callback — Instagram
// redirects here with ?code & ?state. It always ends in a redirect back to the
// frontend /instagram page carrying a connected/error flag.
func (h *InstagramPublicHandler) OAuthCallback(c *gin.Context) {
	redirect := func(path string) {
		c.Redirect(http.StatusFound, h.frontend+path)
	}
	if h.svc == nil {
		redirect("/instagram?error=disabled")
		return
	}
	if errParam := c.Query("error"); errParam != "" {
		// User denied consent or Meta returned an error on the authorize step.
		h.logger.Info("instagram oauth callback returned error", zap.String("error", errParam))
		redirect("/instagram?error=denied")
		return
	}
	path, err := h.svc.HandleCallback(c.Request.Context(), c.Query("code"), c.Query("state"))
	if err != nil {
		h.logger.Warn("instagram oauth callback failed", zap.Error(err))
	}
	redirect(path)
}

// ServeStoryMedia handles GET /api/v1/public/story-media/:id — returns the
// ephemeral JPEG so the Instagram Graph API can fetch it from a public URL.
func (h *InstagramPublicHandler) ServeStoryMedia(c *gin.Context) {
	if h.mediaStore == nil {
		c.Status(http.StatusNotFound)
		return
	}
	data, mime, err := h.mediaStore.Get(c.Request.Context(), c.Param("id"))
	if err != nil || len(data) == 0 {
		c.Status(http.StatusNotFound)
		return
	}
	c.Header("Cache-Control", "no-store")
	if mime == "" {
		mime = "image/jpeg"
	}
	c.Data(http.StatusOK, mime, data)
}

// Deauthorize handles POST /api/v1/instagram/deauthorize — Meta calls this when
// a user removes the app. Acknowledged as a stub (v1); the token simply expires.
func (h *InstagramPublicHandler) Deauthorize(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// DataDeletion handles POST /api/v1/instagram/data-deletion — Meta data-deletion
// request callback. Stub acknowledgement (v1).
func (h *InstagramPublicHandler) DataDeletion(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
