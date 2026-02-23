package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// sentryEnvelopeHeader is the first line of a Sentry envelope.
type sentryEnvelopeHeader struct {
	DSN string `json:"dsn"`
}

// SentryTunnelHandler proxies Sentry envelopes through the app's origin,
// bypassing ad blockers that block requests to *.sentry.io.
type SentryTunnelHandler struct {
	allowedHost string
	logger      *zap.Logger
	client      *http.Client
	newRequest  func(ctx context.Context, method, url string, body io.Reader) (*http.Request, error)
}

// NewSentryTunnelHandler creates a handler that only forwards envelopes
// whose DSN host matches allowedHost (e.g. "o4510830190592000.ingest.us.sentry.io").
func NewSentryTunnelHandler(allowedHost string, logger *zap.Logger) *SentryTunnelHandler {
	return &SentryTunnelHandler{
		allowedHost: allowedHost,
		logger:      logger,
		client:      &http.Client{},
		newRequest:  http.NewRequestWithContext,
	}
}

// Handle receives a Sentry envelope, validates the DSN, and forwards it.
func (h *SentryTunnelHandler) Handle(c *gin.Context) {
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20)) // 1 MB limit
	if err != nil {
		h.logger.Warn("sentry-tunnel: failed to read body", zap.Error(err))
		c.Status(http.StatusBadRequest)
		return
	}

	// The envelope is newline-delimited; first line is the header JSON.
	firstNewline := strings.IndexByte(string(body), '\n')
	if firstNewline < 0 {
		c.Status(http.StatusBadRequest)
		return
	}

	var header sentryEnvelopeHeader
	if err := json.Unmarshal(body[:firstNewline], &header); err != nil {
		h.logger.Warn("sentry-tunnel: invalid envelope header", zap.Error(err))
		c.Status(http.StatusBadRequest)
		return
	}

	dsn, err := url.Parse(header.DSN)
	if err != nil || dsn.Host == "" {
		h.logger.Warn("sentry-tunnel: invalid DSN", zap.String("dsn", header.DSN))
		c.Status(http.StatusUnauthorized)
		return
	}

	// Security: only allow our own Sentry host
	if dsn.Host != h.allowedHost {
		h.logger.Warn("sentry-tunnel: DSN host not allowed",
			zap.String("got", dsn.Host),
			zap.String("allowed", h.allowedHost),
		)
		c.Status(http.StatusUnauthorized)
		return
	}

	// Build the upstream URL: https://<host>/api/<project_id>/envelope/
	projectID := strings.TrimPrefix(dsn.Path, "/")
	upstream := "https://" + dsn.Host + "/api/" + projectID + "/envelope/"

	req, err := h.newRequest(c.Request.Context(), http.MethodPost, upstream, strings.NewReader(string(body)))
	if err != nil {
		h.logger.Error("sentry-tunnel: failed to create upstream request", zap.Error(err))
		c.Status(http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", "application/x-sentry-envelope")

	resp, err := h.client.Do(req)
	if err != nil {
		h.logger.Warn("sentry-tunnel: upstream request failed", zap.Error(err))
		c.Status(http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Discard upstream body; just relay status
	io.Copy(io.Discard, resp.Body) //nolint:errcheck
	c.Status(resp.StatusCode)
}
