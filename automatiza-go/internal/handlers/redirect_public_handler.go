package handlers

import (
	"automatiza-go/internal/middleware"
	"automatiza-go/internal/models"
	"automatiza-go/internal/observability"
	"automatiza-go/internal/services/redirect"
	"net/http"
	"net/url"
	"regexp"
	"strings"

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

// ---------------------------------------------------------------------------
// GET / HEAD /r/:slug — direct HTTP redirect
// ---------------------------------------------------------------------------

// Slugs are generated lowercase alphanumeric with hyphens. Anything else is a
// probe, not one of our links: 404 without echoing it back.
var redirectSlugRe = regexp.MustCompile(`^[a-z0-9-]{1,100}$`)

// WhatsApp invite codes, as returned by WAHA GET /api/{session}/groups/{id}/invite-code.
var whatsappInviteCodeRe = regexp.MustCompile(`^[A-Za-z0-9_-]{5,128}$`)

// Outcome labels for observability. Fixed, low-cardinality set — one per
// response branch of Direct. Summing them gives total attempts.
const (
	redirectOutcomeDirectSuccess      = "direct_success"
	redirectOutcomeFallbackInApp      = "fallback_in_app"
	redirectOutcomeFallbackPreview    = "fallback_preview"
	redirectOutcomeFallbackBusiness   = "fallback_business_error"
	redirectOutcomeInvalidSlug        = "invalid_slug"
	redirectOutcomeServiceUnavailable = "service_unavailable"
	redirectOutcomeServiceError       = "service_error"
	redirectOutcomeInvalidDestination = "invalid_destination"
)

// redirectBusinessErrorCodes maps the service's business errors to the closed
// set of public codes carried in `/join/<slug>?error=<code>`. The raw message
// is NEVER forwarded: the fallback page translates the code, so the query
// string can't leak internals nor be used to inject text into the UI.
var redirectBusinessErrorCodes = map[string]string{
	"link not found":              "not_found",
	"link is inactive":            "inactive",
	"no target groups configured": "no_groups",
	"no available groups (all groups are full or inactive)": "groups_full",
	"group not found":                  "group_not_found",
	"instance not found":               "instance_not_found",
	"bot is not admin in group":        "not_admin",
	"service temporarily unavailable":  "temporary",
	"internal error":                   "temporary",
	"failed to get invite code":        "invite_unavailable",
	"invite code not available":        "invite_unavailable",
	"WhatsApp provider not configured": "provider_unavailable",
}

// Direct handles GET and HEAD /r/:slug.
//
// This is the hot path of a shared link: a real browser gets ONE 302 straight
// to chat.whatsapp.com, with no SPA bundle in between. Everything that is not
// a real browser navigation (in-app webview, crawler, HEAD, prefetch) is
// bounced to the SPA fallback WITHOUT calling the service, so it can neither
// select a group nor burn a click.
//
// Accounting semantics: one call to Direct by a normal client is one attempt.
// A refresh, a retry or a back/forward that re-issues the request is a NEW
// attempt and may count again — 302 responses are not a portable guarantee of
// "no new request".
func (h *RedirectPublicHandler) Direct(c *gin.Context) {
	// Set before any exit: these must hold for every branch, including errors.
	// The edge must not cache a per-click decision, and the invite URL must not
	// leak to WhatsApp as a Referer.
	c.Header("Cache-Control", "no-store, private")
	c.Header("Pragma", "no-cache")
	c.Header("Referrer-Policy", "no-referrer")

	class := classifyRedirectClient(c.Request.Method, c.Request.UserAgent(), c.Request.Header)

	slug := c.Param("slug")
	if !redirectSlugRe.MatchString(slug) {
		h.finishRedirect(c, class, redirectOutcomeInvalidSlug, http.StatusNotFound, "")
		return
	}

	switch class {
	case redirectClientInApp:
		// The SPA fallback is manual by definition on /join — it never
		// auto-redirects, whatever the UA says.
		h.finishRedirect(c, class, redirectOutcomeFallbackInApp, http.StatusFound, "/join/"+slug)
		return
	case redirectClientPreview:
		// mode=preview also tells the page NOT to POST, so a crawler that runs
		// JavaScript still cannot count a click.
		h.finishRedirect(c, class, redirectOutcomeFallbackPreview, http.StatusFound, "/join/"+slug+"?mode=preview")
		return
	}

	if h.redirectService == nil {
		// Degrading to /join would be worse than failing: the POST fallback
		// lands on a stub that doesn't process slugs at all.
		h.logger.Error("Redirect service not configured")
		h.finishRedirect(c, class, redirectOutcomeServiceUnavailable, http.StatusServiceUnavailable, "")
		return
	}

	// Exactly one service call per attempt.
	result, err := h.redirectService.ProcessRedirect(c.Request.Context(), slug, "")
	if err != nil || result == nil {
		h.logger.Error("Failed to process direct redirect", zap.Error(err))
		h.finishRedirect(c, class, redirectOutcomeServiceError, http.StatusInternalServerError, "")
		return
	}

	if !result.Success {
		// No second service call in the fallback: the page renders the error
		// from this code alone. A clean retry/reload starts a NEW attempt.
		code, ok := redirectBusinessErrorCodes[result.Error]
		if !ok {
			code = "generic"
		}
		h.finishRedirect(c, class, redirectOutcomeFallbackBusiness, http.StatusFound, "/join/"+slug+"?error="+code)
		return
	}

	// Second frontier: the service already validated the invite code, this
	// validates the whole URL that is about to become a Location header.
	if !isValidWhatsAppInviteURL(result.RedirectURL) {
		h.logger.Error("Refusing to redirect to a non-canonical WhatsApp invite URL",
			zap.String("groupID", result.GroupID))
		h.finishRedirect(c, class, redirectOutcomeInvalidDestination, http.StatusInternalServerError, "")
		return
	}

	h.finishRedirect(c, class, redirectOutcomeDirectSuccess, http.StatusFound, result.RedirectURL)
}

// finishRedirect writes the single response of a Direct attempt and records
// exactly one observability event for it. `location` empty means no Location
// header at all (error branches must never carry an external destination).
func (h *RedirectPublicHandler) finishRedirect(c *gin.Context, class redirectClientClass, outcome string, status int, location string) {
	if location != "" {
		c.Header("Location", location)
	}

	observability.RecordRedirectRequest(c.Request.Context(), string(class), outcome, status)
	// Fixed values only — never slug, URL, UA, phone or token.
	h.logger.Info("redirect request",
		zap.String("client_class", string(class)),
		zap.String("outcome", outcome),
		zap.Int("http_status", status))

	// AbortWithStatus (not Status) so the header is flushed even when the
	// handler is invoked directly, as the unit tests do.
	c.AbortWithStatus(status)
}

// isValidWhatsAppInviteURL accepts ONLY the canonical invite URL shape.
// Everything else — http, userinfo, a port, a host suffix, a query, a
// fragment, an extra or percent-encoded path segment, CR/LF — is rejected, so
// no value derived from the database or from WAHA can turn the Location header
// into an open redirect.
func isValidWhatsAppInviteURL(raw string) bool {
	if raw == "" {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	if u.Scheme != "https" || u.Host != "chat.whatsapp.com" {
		return false
	}
	if u.User != nil || u.Opaque != "" || u.RawPath != "" {
		return false
	}
	if u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.RawFragment != "" {
		return false
	}
	if !strings.HasPrefix(u.Path, "/") {
		return false
	}
	return whatsappInviteCodeRe.MatchString(strings.TrimPrefix(u.Path, "/"))
}
