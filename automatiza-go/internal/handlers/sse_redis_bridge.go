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
	"automatiza-go/internal/observability"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/getsentry/sentry-go"
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

	// Backoff config for reconnect loop. Zero values use defaults.
	baseBackoff  time.Duration // default 100ms
	maxBackoff   time.Duration // default 30s
	closePubSub  func(*redis.PubSub) error // injectable for testing
}

// NewSSERedisBridge creates a new Redis bridge for SSE event relay.
func NewSSERedisBridge(redisClient *redis.Client, logger *zap.Logger) *SSERedisBridge {
	return &SSERedisBridge{
		redisClient: redisClient,
		logger:      logger.Named("sse-redis-bridge"),
		channel:     sseRedisChannel,
		closePubSub: func(ps *redis.PubSub) error { return ps.Close() },
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
// Uses context.WithTimeout (W30) to prevent indefinite blocking on Redis publish.
func (b *SSERedisBridge) Publish(ctx context.Context, event SSEEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal SSE event: %w", err)
	}

	// W30: Use context.WithTimeout to prevent indefinite blocking
	pubCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := b.redisClient.Publish(pubCtx, b.channel, data).Err(); err != nil {
		observability.LogAndCapture(b.logger, sentry.LevelWarning,
			"Failed to publish SSE event to Redis",
			fmt.Errorf("redis publish event %s: %w", event.Type, err),
			map[string]string{
				"module":    "sse-redis-bridge",
				"eventType": string(event.Type),
			})
		return fmt.Errorf("redis publish: %w", err)
	}

	b.logger.Info("Published SSE event to Redis",
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
// Uses SafeGo (S68), cancel-before-close (C8), and reconnect with exponential backoff (N10).
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

	// S68: Use SafeGo for panic recovery + Sentry capture
	observability.SafeGo("sse-redis-bridge", func() {
		b.subscribeLoop(ctx, pubsub)
	})

	return nil
}

// subscribeLoop runs the subscription loop with reconnect support.
// On channel close or error, it reconnects with exponential backoff (N10).
func (b *SSERedisBridge) subscribeLoop(ctx context.Context, pubsub *redis.PubSub) {
	baseBackoff := b.baseBackoff
	if baseBackoff == 0 {
		baseBackoff = 100 * time.Millisecond
	}
	maxBackoff := b.maxBackoff
	if maxBackoff == 0 {
		maxBackoff = 30 * time.Second
	}
	const multiplier = 2

	// Run initial subscription
	b.drainPubSub(ctx, pubsub)

	// C8: Cancel context before closing pubsub to avoid race
	if err := b.closePubSub(pubsub); err != nil {
		b.logger.Warn("Failed to close Redis subscription on initial drain", zap.Error(err))
	}

	// N10: Reconnect with exponential backoff
	backoff := baseBackoff
	for {
		select {
		case <-ctx.Done():
			b.logger.Info("Redis SSE bridge shutting down, no reconnect")
			return
		default:
		}

		b.logger.Info("Redis SSE bridge reconnecting",
			zap.Duration("backoff", backoff))

		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			b.logger.Info("Redis SSE bridge shutting down during backoff")
			return
		case <-timer.C:
		}

		newPubSub := b.redisClient.Subscribe(ctx, b.channel)
		_, err := newPubSub.Receive(ctx)
		if err != nil {
			observability.LogAndCapture(b.logger, sentry.LevelWarning,
				"Redis SSE bridge reconnect failed",
				fmt.Errorf("redis resubscribe: %w", err),
				map[string]string{
					"module":  "sse-redis-bridge",
					"channel": b.channel,
				})

			// Exponential backoff
			backoff *= time.Duration(multiplier)
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}

		// Reconnected successfully — reset backoff
		b.logger.Info("Redis SSE bridge reconnected successfully",
			zap.String("channel", b.channel))
		backoff = baseBackoff

		b.drainPubSub(ctx, newPubSub)

		// C8: Close after context is already handled
		if err := b.closePubSub(newPubSub); err != nil {
			b.logger.Warn("Failed to close Redis subscription after drain", zap.Error(err))
		}
	}
}

// drainPubSub reads messages from the pubsub channel until context cancellation
// or the channel is closed.
func (b *SSERedisBridge) drainPubSub(ctx context.Context, pubsub *redis.PubSub) {
	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			b.logger.Info("Redis SSE bridge shutting down")
			return
		case msg, ok := <-ch:
			if !ok {
				b.logger.Warn("Redis subscription channel closed, will reconnect")
				return
			}
			b.handleMessage(msg)
		}
	}
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

	b.logger.Info("Received SSE event from Redis",
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
		// Session likely deleted — transient, not worth Sentry noise
		n.logger.Debug("Session not resolved (likely deleted)",
			zap.String("session", sessionName),
			zap.Error(err))
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
// Also persists the latest progress in a Redis key so the polling endpoint (GetSyncStatus)
// can read real-time progress without relying on fire-and-forget Pub/Sub alone.
func (p *RedisJobProgressPublisher) PublishJobProgress(userID string, data map[string]any) {
	p.logger.Info("[redis-job-progress] PublishJobProgress called",
		zap.String("userID", userID),
		zap.Any("jobId", data["jobId"]),
		zap.Any("progress", data["progress"]),
		zap.Any("status", data["status"]))

	event := SSEEvent{
		Type:   SSEEventJobStatus,
		UserID: userID,
		Data:   data,
	}

	// W30: Use context.WithTimeout for Redis publish
	pubCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := p.bridge.Publish(pubCtx, event); err != nil {
		observability.LogAndCapture(p.logger, sentry.LevelWarning,
			"Failed to publish job progress via Redis",
			fmt.Errorf("publish job progress for user %s: %w", userID, err),
			map[string]string{
				"module": "redis-job-progress-publisher",
				"userID": userID,
			})
	}

	// Persist latest progress in a Redis key for polling (GetSyncStatus)
	if jobID, ok := data["jobId"].(string); ok && jobID != "" {
		progressJSON, marshalErr := json.Marshal(data)
		if marshalErr != nil {
			observability.LogAndCapture(p.logger, sentry.LevelWarning,
				"Failed to marshal sync progress data",
				fmt.Errorf("marshal sync progress for job %s: %w", jobID, marshalErr),
				map[string]string{
					"module": "redis-job-progress-publisher",
					"jobID":  jobID,
				})
		} else {
			key := "sync_progress:" + jobID
			setCtx, setCancel := context.WithTimeout(context.Background(), 5*time.Second)
			setErr := p.bridge.redisClient.Set(setCtx, key, progressJSON, 5*time.Minute).Err()
			setCancel()
			if setErr != nil {
				observability.LogAndCapture(p.logger, sentry.LevelWarning,
					"Failed to SET sync_progress in Redis",
					fmt.Errorf("redis SET %s: %w", key, setErr),
					map[string]string{
						"module": "redis-job-progress-publisher",
						"key":    key,
					})
			} else {
				p.logger.Info("[redis-job-progress] Redis SET sync_progress OK",
					zap.String("key", key))
			}
		}
	}
}

// =============================================================================
// RedisSyncProgressReader — server-side sync progress reader
// =============================================================================

// RedisSyncProgressReader reads sync job progress from Redis keys
// persisted by RedisJobProgressPublisher. Used by the GetSyncStatus RPC
// to return real-time progress data to the frontend polling endpoint.
type RedisSyncProgressReader struct {
	redisClient *redis.Client
}

// NewRedisSyncProgressReader creates a new Redis-backed sync progress reader.
func NewRedisSyncProgressReader(redisClient *redis.Client) *RedisSyncProgressReader {
	return &RedisSyncProgressReader{redisClient: redisClient}
}

// GetSyncProgress reads the latest progress data for a sync job from Redis.
func (r *RedisSyncProgressReader) GetSyncProgress(ctx context.Context, jobID string) (map[string]any, error) {
	key := "sync_progress:" + jobID
	data, err := r.redisClient.Get(ctx, key).Bytes()
	if err != nil {
		return nil, err
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}
