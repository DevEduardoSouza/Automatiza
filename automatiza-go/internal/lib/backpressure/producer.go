package backpressure

import (
	"context"
	"errors"
	"fmt"
	"time"

	"automatiza-go/internal/lib/cache"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// QueueInspector provides queue depth information for global capacity checks.
type QueueInspector interface {
	// GetQueueDepth returns the total number of pending + active tasks in the queue.
	GetQueueDepth(ctx context.Context) (int, error)
}

// ProducerBackpressure manages backpressure for message producers
type ProducerBackpressure struct {
	cache             *cache.Cache
	logger            *zap.Logger
	delayMS           int
	userMaxEnqueued   int
	globalMaxEnqueued int
	windowMinutes     int
	enabled           bool
	queueInspector    QueueInspector
}

// Config for producer backpressure
type Config struct {
	DelayMS           int // Delay to add when queue is busy
	UserMaxEnqueued   int // Max messages per user in window
	GlobalMaxEnqueued int // Max messages globally across all users (0 = unlimited)
	WindowMinutes     int // Window size in minutes
	Enabled           bool
}

// NewProducerBackpressure creates a new backpressure manager
func NewProducerBackpressure(cfg Config, cache *cache.Cache, logger *zap.Logger) *ProducerBackpressure {
	if cfg.WindowMinutes <= 0 {
		cfg.WindowMinutes = 10
	}
	if cfg.DelayMS <= 0 {
		cfg.DelayMS = 15000
	}
	if cfg.UserMaxEnqueued <= 0 {
		cfg.UserMaxEnqueued = 200
	}

	return &ProducerBackpressure{
		cache:             cache,
		logger:            logger,
		delayMS:           cfg.DelayMS,
		userMaxEnqueued:   cfg.UserMaxEnqueued,
		globalMaxEnqueued: cfg.GlobalMaxEnqueued,
		windowMinutes:     cfg.WindowMinutes,
		enabled:           cfg.Enabled,
	}
}

// SetQueueInspector sets the queue inspector for global capacity checks.
// This should be called after initialization when the asynq inspector is available.
func (p *ProducerBackpressure) SetQueueInspector(inspector QueueInspector) {
	p.queueInspector = inspector
}

// CanEnqueue checks if a user can enqueue more messages
func (p *ProducerBackpressure) CanEnqueue(ctx context.Context, userID string) (bool, string, error) {
	if !p.enabled {
		return true, "", nil
	}

	// Check global queue capacity first
	if canEnqueue, reason := p.checkGlobalCapacity(ctx); !canEnqueue {
		return false, reason, nil
	}

	// Check per-user limit
	count, err := p.getUserEnqueuedCount(ctx, userID)
	if err != nil {
		p.logger.Warn("Failed to get enqueued count, allowing",
			zap.String("userID", userID),
			zap.Error(err))
		return true, "", nil
	}

	if count >= p.userMaxEnqueued {
		return false, fmt.Sprintf("Limite de %d mensagens em %d minutos atingido", p.userMaxEnqueued, p.windowMinutes), nil
	}

	return true, "", nil
}

// checkGlobalCapacity checks if the global queue capacity allows more messages.
// Returns (canEnqueue, reason).
func (p *ProducerBackpressure) checkGlobalCapacity(ctx context.Context) (bool, string) {
	// If global limit is disabled (0) or no inspector, allow
	if p.globalMaxEnqueued <= 0 || p.queueInspector == nil {
		return true, ""
	}

	depth, err := p.queueInspector.GetQueueDepth(ctx)
	if err != nil {
		p.logger.Warn("Failed to get queue depth for global capacity check, allowing",
			zap.Error(err))
		return true, ""
	}

	if depth >= p.globalMaxEnqueued {
		p.logger.Warn("Global queue capacity reached",
			zap.Int("currentDepth", depth),
			zap.Int("maxCapacity", p.globalMaxEnqueued))
		return false, fmt.Sprintf("Sistema ocupado, aguarde alguns minutos (fila: %d/%d)", depth, p.globalMaxEnqueued)
	}

	return true, ""
}

// GetGlobalQueueDepth returns the current global queue depth.
// Returns 0 if queue inspector is not configured.
func (p *ProducerBackpressure) GetGlobalQueueDepth(ctx context.Context) (int, error) {
	if p.queueInspector == nil {
		return 0, nil
	}
	return p.queueInspector.GetQueueDepth(ctx)
}

// GetGlobalMaxEnqueued returns the configured global max enqueued limit.
func (p *ProducerBackpressure) GetGlobalMaxEnqueued() int {
	return p.globalMaxEnqueued
}

// RecordEnqueue records that a message was enqueued
func (p *ProducerBackpressure) RecordEnqueue(ctx context.Context, userID string) error {
	if !p.enabled {
		return nil
	}

	key := p.getUserKey(userID)

	if p.cache == nil {
		return nil
	}

	count, err := p.cache.Incr(ctx, key)
	if err != nil {
		p.logger.Warn("Failed to increment enqueue count",
			zap.String("userID", userID),
			zap.Error(err))
		return nil
	}
	if count == 1 {
		// Set expiration on first increment
		_ = p.cache.Expire(ctx, key, time.Duration(p.windowMinutes)*time.Minute)
	}

	return nil
}

// GetDelay returns the delay to add based on current queue depth
func (p *ProducerBackpressure) GetDelay(ctx context.Context, queueDepth int) time.Duration {
	if !p.enabled {
		return 0
	}

	// Add delay if queue is getting full
	if queueDepth > 100 {
		return time.Duration(p.delayMS) * time.Millisecond
	}
	if queueDepth > 50 {
		return time.Duration(p.delayMS/2) * time.Millisecond
	}

	return 0
}

// getUserEnqueuedCount gets the current enqueue count for a user
func (p *ProducerBackpressure) getUserEnqueuedCount(ctx context.Context, userID string) (int, error) {
	key := p.getUserKey(userID)

	if p.cache == nil {
		// Without Redis, backpressure cannot track counts reliably — allow all
		return 0, nil
	}

	countStr, err := p.cache.Get(ctx, key)
	if err != nil {
		// Key not found in Redis means the window expired — count is 0
		if errors.Is(err, redis.Nil) {
			return 0, nil
		}
		// Real Redis error — allow enqueue (fail-open)
		p.logger.Warn("Redis error getting backpressure count, allowing",
			zap.String("userID", userID),
			zap.Error(err))
		return 0, nil
	}

	var count int
	_, _ = fmt.Sscanf(countStr, "%d", &count)
	return count, nil
}

func (p *ProducerBackpressure) getUserKey(userID string) string {
	return fmt.Sprintf("backpressure:user:%s", userID)
}

// ResetUserCount resets the enqueue count for a user
func (p *ProducerBackpressure) ResetUserCount(ctx context.Context, userID string) error {
	if p.cache != nil {
		return p.cache.Delete(ctx, p.getUserKey(userID))
	}
	return nil
}

// GetUserCount returns the current count for a user
func (p *ProducerBackpressure) GetUserCount(ctx context.Context, userID string) (int, error) {
	return p.getUserEnqueuedCount(ctx, userID)
}
