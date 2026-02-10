// Package handlers provides HTTP handlers for the API.
// This file implements the Redis Pub/Sub bridge that relays SSE events between
// the worker process (which processes webhooks) and the server process (which
// serves SSE connections to the frontend).
//
// Problem: SSEHub is in-memory. Worker and server run as separate OS processes,
// so publishing to the worker's SSEHub never reaches the server's SSEHub.
//
// Solution: Worker publishes events to a Redis channel. Server subscribes to
// that channel and forwards events to its local SSEHub.
package handlers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const sseRedisChannel = "sse:events"

// SessionCacheClearer allows clearing the adapter's status cache.
// Implemented by *WahaAdapter to invalidate stale cached status
// when a session.status event arrives via Redis Pub/Sub.
type SessionCacheClearer interface {
	ClearStatusCache(sessionName string)
}

// SSERedisBridge relays SSE events between processes via Redis Pub/Sub.
// The worker side calls Publish to send events to Redis.
// The server side calls Subscribe to receive events and forward them to SSEHub.
type SSERedisBridge struct {
	redisClient *redis.Client
	hub         *SSEHub           // non-nil only on server (subscriber side)
	cacheClear  SessionCacheClearer // optional: clear adapter cache on status events
	logger      *zap.Logger
	channel     string
}

// NewSSERedisBridge creates a new Redis bridge for SSE event relay.
func NewSSERedisBridge(redisClient *redis.Client, logger *zap.Logger) *SSERedisBridge {
	return &SSERedisBridge{
		redisClient: redisClient,
		logger:      logger.Named("sse-redis-bridge"),
		channel:     sseRedisChannel,
	}
}

// SetHub sets the SSEHub for the subscriber side (server process).
func (b *SSERedisBridge) SetHub(hub *SSEHub) {
	b.hub = hub
}

// SetCacheClearer sets the cache clearer for invalidating stale adapter status cache.
func (b *SSERedisBridge) SetCacheClearer(cc SessionCacheClearer) {
	b.cacheClear = cc
}

// Publish serializes an SSEEvent to JSON and publishes it to the Redis channel.
// Called by the worker process when a webhook event produces an SSE notification.
func (b *SSERedisBridge) Publish(ctx context.Context, event SSEEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal SSE event: %w", err)
	}

	if err := b.redisClient.Publish(ctx, b.channel, data).Err(); err != nil {
		b.logger.Warn("Failed to publish SSE event to Redis",
			zap.String("eventType", string(event.Type)),
			zap.Error(err))
		return fmt.Errorf("redis publish: %w", err)
	}

	b.logger.Debug("Published SSE event to Redis",
		zap.String("eventType", string(event.Type)),
		zap.String("session", event.Session),
		zap.String("userID", event.UserID))

	return nil
}

// Subscribe starts a goroutine that subscribes to the Redis channel and forwards
// received events to the local SSEHub. Also clears adapter status cache on
// session.status events so polling returns fresh data immediately.
//
// This should be called only on the server process (the one serving SSE to clients).
// The context controls the lifetime of the subscription.
func (b *SSERedisBridge) Subscribe(ctx context.Context) error {
	if b.hub == nil {
		return fmt.Errorf("SSEHub not set; call SetHub before Subscribe")
	}

	pubsub := b.redisClient.Subscribe(ctx, b.channel)

	// Verify subscription is active
	_, err := pubsub.Receive(ctx)
	if err != nil {
		return fmt.Errorf("redis subscribe: %w", err)
	}

	b.logger.Info("Redis SSE bridge subscribed",
		zap.String("channel", b.channel))

	go func() {
		defer func() {
			if err := pubsub.Close(); err != nil {
				b.logger.Warn("Failed to close Redis subscription", zap.Error(err))
			}
		}()

		ch := pubsub.Channel()
		for {
			select {
			case <-ctx.Done():
				b.logger.Info("Redis SSE bridge shutting down")
				return
			case msg, ok := <-ch:
				if !ok {
					b.logger.Warn("Redis subscription channel closed")
					return
				}
				b.handleMessage(msg)
			}
		}
	}()

	return nil
}

// handleMessage deserializes a Redis message and forwards it to the SSEHub.
func (b *SSERedisBridge) handleMessage(msg *redis.Message) {
	var event SSEEvent
	if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
		b.logger.Warn("Failed to unmarshal SSE event from Redis",
			zap.Error(err),
			zap.String("payload", msg.Payload))
		return
	}

	b.logger.Debug("Received SSE event from Redis",
		zap.String("eventType", string(event.Type)),
		zap.String("session", event.Session),
		zap.String("userID", event.UserID))

	// Clear adapter cache on session.status events so next polling RPC returns fresh status
	if event.Type == SSEEventSessionStatus && b.cacheClear != nil && event.Session != "" {
		b.cacheClear.ClearStatusCache(event.Session)
	}

	// Forward to local SSEHub
	b.hub.Publish(event)
}

// =============================================================================
// RedisSSEStatusNotifier — worker-side status processor
// =============================================================================

// RedisSSEStatusNotifier implements workers.StatusWebhookProcessor by publishing
// session status events to Redis (instead of directly to a local SSEHub).
// Used in the worker process so that status events reach the server process.
type RedisSSEStatusNotifier struct {
	bridge       *SSERedisBridge
	logger       *zap.Logger
	userResolver func(sessionName string) (string, error)
}

// NewRedisSSEStatusNotifier creates a new Redis-backed SSE status notifier.
func NewRedisSSEStatusNotifier(
	bridge *SSERedisBridge,
	logger *zap.Logger,
	userResolver func(string) (string, error),
) *RedisSSEStatusNotifier {
	return &RedisSSEStatusNotifier{
		bridge:       bridge,
		logger:       logger.Named("redis-sse-status-notifier"),
		userResolver: userResolver,
	}
}

// ProcessStatus implements workers.StatusWebhookProcessor.
// Resolves the user ID from the session name and publishes the status change
// to Redis so the server process can forward it via SSE.
func (n *RedisSSEStatusNotifier) ProcessStatus(ctx context.Context, sessionName, status string) error {
	userID, err := n.userResolver(sessionName)
	if err != nil {
		n.logger.Warn("Failed to resolve user ID for session",
			zap.String("session", sessionName),
			zap.Error(err))
		// Don't fail — SSE notification is best-effort
		return nil
	}

	n.logger.Info("Publishing session status via Redis",
		zap.String("session", sessionName),
		zap.String("userID", userID),
		zap.String("status", status))

	event := SSEEvent{
		Type:    SSEEventSessionStatus,
		Session: sessionName,
		UserID:  userID,
		Data: map[string]any{
			"session": sessionName,
			"status":  status,
		},
	}

	return n.bridge.Publish(ctx, event)
}

// =============================================================================
// RedisJobProgressPublisher — worker-side job progress publisher
// =============================================================================

// RedisJobProgressPublisher implements workers.JobProgressPublisher by publishing
// job progress events to Redis (instead of directly to a local SSEHub).
// Used in the worker process so that progress events reach the server process.
type RedisJobProgressPublisher struct {
	bridge *SSERedisBridge
	logger *zap.Logger
}

// NewRedisJobProgressPublisher creates a new Redis-backed job progress publisher.
func NewRedisJobProgressPublisher(bridge *SSERedisBridge, logger *zap.Logger) *RedisJobProgressPublisher {
	return &RedisJobProgressPublisher{
		bridge: bridge,
		logger: logger.Named("redis-job-progress-publisher"),
	}
}

// PublishJobProgress implements workers.JobProgressPublisher.
// Publishes job progress events to Redis so the server process can forward them via SSE.
func (p *RedisJobProgressPublisher) PublishJobProgress(userID string, data map[string]any) {
	event := SSEEvent{
		Type:   SSEEventJobStatus,
		UserID: userID,
		Data:   data,
	}

	if err := p.bridge.Publish(context.Background(), event); err != nil {
		p.logger.Warn("Failed to publish job progress via Redis",
			zap.String("userID", userID),
			zap.Error(err))
	}
}
