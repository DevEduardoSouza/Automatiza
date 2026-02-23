// Package handlers provides HTTP handlers for the API.
// This file implements Server-Sent Events (SSE) for real-time notifications
// to the frontend, particularly for WhatsApp session status changes.
package handlers

import (
	"automatiza-go/internal/observability"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// =============================================================================
// Types
// =============================================================================

// SSEEventType represents the type of SSE event.
type SSEEventType string

const (
	// SSEEventSessionStatus is sent when a WhatsApp session status changes.
	SSEEventSessionStatus SSEEventType = "session.status"

	// SSEEventQRCode is sent when a new QR code is available.
	SSEEventQRCode SSEEventType = "qr.code"

	// SSEEventGroupSync is sent when group sync completes.
	SSEEventGroupSync SSEEventType = "group.sync"

	// SSEEventHeartbeat is sent periodically to keep the connection alive.
	SSEEventHeartbeat SSEEventType = "heartbeat"

	// SSEEventGroupCreated is sent when a group is created.
	SSEEventGroupCreated SSEEventType = "group.created"

	// SSEEventGroupUpdated is sent when a group is updated.
	SSEEventGroupUpdated SSEEventType = "group.updated"

	// SSEEventGroupDeleted is sent when a group is deleted.
	SSEEventGroupDeleted SSEEventType = "group.deleted"

	// SSEEventGroupMemberChanged is sent when group membership changes (add/remove/promote/demote).
	SSEEventGroupMemberChanged SSEEventType = "group.member.changed"

	// SSEEventJobStatus is sent when a job status changes (pending → running → done/failed).
	SSEEventJobStatus SSEEventType = "job.status"

	// SSEEventHistoryChange is sent when an action history entry is created or updated.
	SSEEventHistoryChange SSEEventType = "history.change"
)

// SSEEvent represents an event to be sent via SSE.
type SSEEvent struct {
	Type      SSEEventType   `json:"type"`
	Session   string         `json:"session,omitempty"`
	UserID    string         `json:"userId,omitempty"`
	Data      map[string]any `json:"data"`
	Timestamp int64          `json:"timestamp"`
}

// sseClient represents a connected SSE client.
type sseClient struct {
	userID   string
	channel  chan SSEEvent
	doneChan chan struct{}
}

// =============================================================================
// SSE Hub (Pub/Sub)
// =============================================================================

// SSEHub manages SSE client connections and event broadcasting.
// It uses a pub/sub pattern to notify connected clients of events.
type SSEHub struct {
	// clients maps userID to their SSE client
	clients   map[string]*sseClient
	clientsMu sync.RWMutex

	// broadcast channel for events
	broadcast chan SSEEvent

	// register/unregister channels
	register   chan *sseClient
	unregister chan *sseClient

	logger *zap.Logger

	// done channel for graceful shutdown
	done chan struct{}

	// heartbeatInterval controls how often heartbeat events are sent.
	// Defaults to 30s when zero.
	heartbeatInterval time.Duration
}

// NewSSEHub creates a new SSE hub for managing client connections.
func NewSSEHub(logger *zap.Logger) *SSEHub {
	hub := &SSEHub{
		clients:    make(map[string]*sseClient),
		broadcast:  make(chan SSEEvent, 256),
		register:   make(chan *sseClient, 16),
		unregister: make(chan *sseClient, 16),
		logger:     logger.Named("sse-hub"),
		done:       make(chan struct{}),
	}

	// Start the hub's run loop with panic recovery
	observability.SafeGo("sse-hub", hub.run)

	return hub
}

// run is the main loop that handles client registration and event broadcasting.
func (h *SSEHub) run() {
	// Heartbeat ticker to keep connections alive
	interval := h.heartbeatInterval
	if interval == 0 {
		interval = 30 * time.Second
	}
	heartbeatTicker := time.NewTicker(interval)
	defer heartbeatTicker.Stop()

	for {
		select {
		case <-h.done:
			// Graceful shutdown: close all client channels
			h.clientsMu.Lock()
			for _, client := range h.clients {
				close(client.channel)
			}
			h.clients = make(map[string]*sseClient)
			h.clientsMu.Unlock()
			return

		case client := <-h.register:
			h.clientsMu.Lock()
			// Close existing client if any (single connection per user)
			if existing, ok := h.clients[client.userID]; ok {
				close(existing.channel)
			}
			h.clients[client.userID] = client
			h.clientsMu.Unlock()
			h.logger.Debug("SSE client registered",
				zap.String("userID", client.userID),
				zap.Int("totalClients", len(h.clients)))

		case client := <-h.unregister:
			h.clientsMu.Lock()
			if existing, ok := h.clients[client.userID]; ok && existing == client {
				delete(h.clients, client.userID)
				close(client.channel)
			}
			h.clientsMu.Unlock()
			h.logger.Debug("SSE client unregistered",
				zap.String("userID", client.userID),
				zap.Int("totalClients", len(h.clients)))

		case event := <-h.broadcast:
			h.clientsMu.RLock()
			// If event has a specific userID, only send to that user
			if event.UserID != "" {
				if client, ok := h.clients[event.UserID]; ok {
					h.logger.Info("[sse-hub] Sending event to client",
						zap.String("eventType", string(event.Type)),
						zap.String("userID", event.UserID))
					select {
					case client.channel <- event:
					default:
						// W28: Client channel full — Warning with rate-limiting
						observability.LogAndCapture(h.logger, sentry.LevelWarning,
							"SSE client channel full, dropping event",
							fmt.Errorf("client channel saturated for user %s, event %s", event.UserID, event.Type),
							map[string]string{
								"module":    "sse-hub",
								"userID":    event.UserID,
								"eventType": string(event.Type),
							})
					}
				}
			} else {
				// Broadcast to all clients
				for _, client := range h.clients {
					select {
					case client.channel <- event:
					default:
						// W28: Client channel full — Warning with rate-limiting
						observability.LogAndCapture(h.logger, sentry.LevelWarning,
							"SSE client channel full, dropping event",
							fmt.Errorf("client channel saturated for user %s, event %s", client.userID, event.Type),
							map[string]string{
								"module":    "sse-hub",
								"userID":    client.userID,
								"eventType": string(event.Type),
							})
					}
				}
			}
			h.clientsMu.RUnlock()

		case <-heartbeatTicker.C:
			// Send heartbeat to all clients
			heartbeat := SSEEvent{
				Type:      SSEEventHeartbeat,
				Data:      map[string]any{"ping": true},
				Timestamp: time.Now().UnixMilli(),
			}
			h.clientsMu.RLock()
			for _, client := range h.clients {
				select {
				case client.channel <- heartbeat:
				default:
					// Skip if channel full
				}
			}
			h.clientsMu.RUnlock()
		}
	}
}

// Publish sends an event to the appropriate clients.
// If event.UserID is set, only that user receives the event.
// Otherwise, the event is broadcast to all clients.
func (h *SSEHub) Publish(event SSEEvent) {
	if event.Timestamp == 0 {
		event.Timestamp = time.Now().UnixMilli()
	}

	h.logger.Info("[sse-hub] Publish called",
		zap.String("eventType", string(event.Type)),
		zap.String("userID", event.UserID))

	select {
	case h.broadcast <- event:
	default:
		// W29: Broadcast channel full — Critical (affects ALL users)
		observability.LogAndCapture(h.logger, sentry.LevelFatal,
			"SSE broadcast channel full, dropping event",
			fmt.Errorf("broadcast channel saturated, event %s dropped", event.Type),
			map[string]string{
				"module":    "sse-hub",
				"eventType": string(event.Type),
				"severity":  "critical",
			})
	}
}

// PublishSessionStatus is a convenience method to publish session status changes.
func (h *SSEHub) PublishSessionStatus(userID, sessionName, status string, qrCode string) {
	data := map[string]any{
		"session": sessionName,
		"status":  status,
	}
	if qrCode != "" {
		data["qrCode"] = qrCode
	}

	h.Publish(SSEEvent{
		Type:    SSEEventSessionStatus,
		Session: sessionName,
		UserID:  userID,
		Data:    data,
	})
}

// PublishGroupChange is a convenience method to publish group change events.
// Accepts string for eventType to satisfy cross-package interfaces.
func (h *SSEHub) PublishGroupChange(userID string, eventType string, groupID string) {
	h.Publish(SSEEvent{
		Type:   SSEEventType(eventType),
		UserID: userID,
		Data: map[string]any{
			"groupId": groupID,
		},
	})
}

// PublishJobStatus sends a job status change event to the job owner.
func (h *SSEHub) PublishJobStatus(userID, jobID, status string, errorMessage *string) {
	data := map[string]any{
		"jobId":  jobID,
		"status": status,
	}
	if errorMessage != nil {
		data["errorMessage"] = *errorMessage
	}

	h.Publish(SSEEvent{
		Type:   SSEEventJobStatus,
		UserID: userID,
		Data:   data,
	})
}

// PublishHistoryChange sends an action history change event to the specified user.
func (h *SSEHub) PublishHistoryChange(userID, historyID, status string) {
	h.Publish(SSEEvent{
		Type:   SSEEventHistoryChange,
		UserID: userID,
		Data: map[string]any{
			"historyId": historyID,
			"status":    status,
		},
	})
}

// PublishJobProgress sends a job progress event to the job owner.
// Used by workers to emit real-time progress updates during long-running tasks.
func (h *SSEHub) PublishJobProgress(userID string, data map[string]any) {
	h.logger.Info("[sse-hub] PublishJobProgress called",
		zap.String("userID", userID),
		zap.Any("eventType", SSEEventJobStatus))
	h.Publish(SSEEvent{
		Type:   SSEEventJobStatus,
		UserID: userID,
		Data:   data,
	})
}

// Close gracefully shuts down the SSE hub.
func (h *SSEHub) Close() {
	close(h.done)
}

// ClientCount returns the number of connected clients.
func (h *SSEHub) ClientCount() int {
	h.clientsMu.RLock()
	defer h.clientsMu.RUnlock()
	return len(h.clients)
}

// =============================================================================
// SSE Handler
// =============================================================================

// SSEHandler handles SSE connections for real-time notifications.
type SSEHandler struct {
	hub    *SSEHub
	logger *zap.Logger
}

// NewSSEHandler creates a new SSE handler.
func NewSSEHandler(hub *SSEHub, logger *zap.Logger) *SSEHandler {
	return &SSEHandler{
		hub:    hub,
		logger: logger.Named("sse-handler"),
	}
}

// HandleSSE handles GET /api/v1/sse/events
// Clients connect to this endpoint to receive real-time events.
//
// The handler expects the userID to be set in the Gin context by authentication middleware.
// Events are sent as SSE format: "event: <type>\ndata: <json>\n\n"
func (h *SSEHandler) HandleSSE(c *gin.Context) {
	// Get user ID from context (set by auth middleware)
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	userIDStr, ok := userID.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user ID"})
		return
	}

	h.logger.Debug("SSE connection initiated",
		zap.String("userID", userIDStr),
		zap.String("ip", c.ClientIP()))

	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no") // Disable nginx buffering
	c.Header("Access-Control-Allow-Origin", "*")

	// Create client
	client := &sseClient{
		userID:   userIDStr,
		channel:  make(chan SSEEvent, 32),
		doneChan: make(chan struct{}),
	}

	// Register client
	h.hub.register <- client

	// Ensure cleanup on disconnect
	defer func() {
		h.hub.unregister <- client
	}()

	// Get the underlying response writer for flushing
	w := c.Writer

	// Send initial connection event
	h.writeEvent(w, SSEEvent{
		Type:      "connected",
		UserID:    userIDStr,
		Data:      map[string]any{"connected": true},
		Timestamp: time.Now().UnixMilli(),
	})

	// Event loop
	clientGone := c.Request.Context().Done()
	for {
		select {
		case <-clientGone:
			h.logger.Debug("SSE client disconnected",
				zap.String("userID", userIDStr))
			return

		case event, ok := <-client.channel:
			if !ok {
				// Channel closed, client was replaced or hub shutdown
				h.logger.Debug("SSE client channel closed",
					zap.String("userID", userIDStr))
				return
			}

			h.writeEvent(w, event)
		}
	}
}

// writeEvent writes an SSE event to the response writer.
func (h *SSEHandler) writeEvent(w gin.ResponseWriter, event SSEEvent) {
	data, err := json.Marshal(event)
	if err != nil {
		h.logger.Error("Failed to marshal SSE event",
			zap.Error(err))
		return
	}

	h.logger.Info("[sse-handler] writeEvent",
		zap.String("eventType", string(event.Type)),
		zap.String("userID", event.UserID))

	// SSE format: "event: <type>\ndata: <json>\n\n"
	// Ignore errors - connection may be closed by client
	_, _ = fmt.Fprintf(w, "event: %s\n", event.Type)
	_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
	w.Flush()
}

// =============================================================================
// Integration with Webhook Worker
// =============================================================================

// SSEStatusNotifier implements StatusWebhookProcessor to notify SSE clients
// when session status changes.
type SSEStatusNotifier struct {
	hub    *SSEHub
	logger *zap.Logger

	// userResolver resolves session name to user ID
	// This is needed because the webhook only contains the session name
	userResolver func(sessionName string) (userID string, err error)
}

// NewSSEStatusNotifier creates a new SSE status notifier.
func NewSSEStatusNotifier(hub *SSEHub, logger *zap.Logger, userResolver func(string) (string, error)) *SSEStatusNotifier {
	return &SSEStatusNotifier{
		hub:          hub,
		logger:       logger.Named("sse-status-notifier"),
		userResolver: userResolver,
	}
}

// ProcessStatus implements StatusWebhookProcessor.
// It resolves the user ID from the session name and publishes the status change via SSE.
func (n *SSEStatusNotifier) ProcessStatus(ctx context.Context, sessionName, status string) error {
	// Resolve session name to user ID
	userID, err := n.userResolver(sessionName)
	if err != nil {
		// Session likely deleted — transient, not worth Sentry noise
		n.logger.Debug("Session not resolved (likely deleted)",
			zap.String("session", sessionName),
			zap.Error(err))
		return nil
	}

	n.logger.Info("Publishing session status via SSE",
		zap.String("session", sessionName),
		zap.String("userID", userID),
		zap.String("status", status))

	// Publish to SSE
	n.hub.PublishSessionStatus(userID, sessionName, status, "")

	return nil
}
