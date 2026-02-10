// Package watchdog provides a WAHA webhook silence watchdog service.
// It monitors for gaps in webhook events and triggers health checks and reconnection
// when sessions become silent for too long.
//
// Key responsibilities:
//   - Track last event time per session via Redis (cross-instance aware)
//   - Detect silence (no events) and trigger health checks
//   - Attempt session reconnection with exponential backoff
//   - Record Prometheus metrics for observability
//
// Architecture:
//   - Uses WhatsAppProvider interface for session health checks and restarts
//   - Uses Redis for distributed last event tracking
//   - Designed to work in multi-instance deployments
package watchdog

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"time"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/repository"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// =============================================================================
// Configuration
// =============================================================================

// Config holds watchdog service configuration.
type Config struct {
	// SilenceThreshold is how long without events before triggering health check.
	// Default: 5 minutes.
	SilenceThreshold time.Duration

	// CheckInterval is how often the watchdog checks for silence.
	// Default: 1 minute.
	CheckInterval time.Duration

	// MaxReconnectAttempts is the maximum number of reconnection attempts per session.
	// Default: 3. Set to 0 to disable reconnection.
	MaxReconnectAttempts int

	// ReconnectBackoff is the base duration for exponential backoff between reconnect attempts.
	// Default: 1 minute.
	ReconnectBackoff time.Duration

	// MaxReconnectBackoff is the maximum backoff duration.
	// Default: 30 minutes.
	MaxReconnectBackoff time.Duration

	// ReconnectCooldown is the minimum time between reconnect attempts for a session.
	// Default: 5 minutes.
	ReconnectCooldown time.Duration

	// RedisKeyPrefix is the prefix for Redis keys.
	// Default: "watchdog".
	RedisKeyPrefix string

	// EventTTL is how long to keep event timestamps in Redis.
	// Default: 24 hours.
	EventTTL time.Duration

	// Enabled controls whether the watchdog is active.
	// Default: true.
	Enabled bool
}

// DefaultConfig returns the default watchdog configuration.
func DefaultConfig() Config {
	return Config{
		SilenceThreshold:     5 * time.Minute,
		CheckInterval:        1 * time.Minute,
		MaxReconnectAttempts: 3,
		ReconnectBackoff:     1 * time.Minute,
		MaxReconnectBackoff:  30 * time.Minute,
		ReconnectCooldown:    5 * time.Minute,
		RedisKeyPrefix:       "watchdog",
		EventTTL:             24 * time.Hour,
		Enabled:              true,
	}
}

// Validate checks if the configuration is valid and applies defaults.
func (c *Config) Validate() error {
	if c.SilenceThreshold <= 0 {
		c.SilenceThreshold = 5 * time.Minute
	}
	if c.CheckInterval <= 0 {
		c.CheckInterval = 1 * time.Minute
	}
	if c.MaxReconnectAttempts < 0 {
		c.MaxReconnectAttempts = 3
	}
	if c.ReconnectBackoff <= 0 {
		c.ReconnectBackoff = 1 * time.Minute
	}
	if c.MaxReconnectBackoff <= 0 {
		c.MaxReconnectBackoff = 30 * time.Minute
	}
	if c.ReconnectCooldown <= 0 {
		c.ReconnectCooldown = 5 * time.Minute
	}
	if c.RedisKeyPrefix == "" {
		c.RedisKeyPrefix = "watchdog"
	}
	if c.EventTTL <= 0 {
		c.EventTTL = 24 * time.Hour
	}
	return nil
}

// =============================================================================
// Metrics
// =============================================================================

// Metrics contains Prometheus metrics for the watchdog service.
type Metrics struct {
	// SilenceDetected counts silence detections per session.
	SilenceDetected *prometheus.CounterVec

	// HealthCheckTotal counts health check attempts per session and result.
	HealthCheckTotal *prometheus.CounterVec

	// ReconnectTotal counts reconnection attempts per session and result.
	ReconnectTotal *prometheus.CounterVec

	// LastEventAge tracks the age of the last event per session in seconds.
	LastEventAge *prometheus.GaugeVec

	// ActiveSessions tracks the number of sessions being monitored.
	ActiveSessions prometheus.Gauge

	// WatchdogRunDuration tracks the duration of watchdog check cycles.
	WatchdogRunDuration prometheus.Histogram
}

// NewMetrics creates and registers watchdog metrics.
func NewMetrics() *Metrics {
	return &Metrics{
		SilenceDetected: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "automatiza",
				Subsystem: "watchdog",
				Name:      "silence_detected_total",
				Help:      "Total number of silence detections per session",
			},
			[]string{"session"},
		),

		HealthCheckTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "automatiza",
				Subsystem: "watchdog",
				Name:      "health_check_total",
				Help:      "Total number of health check attempts",
			},
			[]string{"session", "result"}, // result: "healthy", "unhealthy", "error"
		),

		ReconnectTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "automatiza",
				Subsystem: "watchdog",
				Name:      "reconnect_total",
				Help:      "Total number of reconnection attempts",
			},
			[]string{"session", "result"}, // result: "success", "failed", "skipped"
		),

		LastEventAge: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "automatiza",
				Subsystem: "watchdog",
				Name:      "last_event_age_seconds",
				Help:      "Time since last event for each session in seconds",
			},
			[]string{"session"},
		),

		ActiveSessions: promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "automatiza",
				Subsystem: "watchdog",
				Name:      "active_sessions",
				Help:      "Number of sessions being monitored",
			},
		),

		WatchdogRunDuration: promauto.NewHistogram(
			prometheus.HistogramOpts{
				Namespace: "automatiza",
				Subsystem: "watchdog",
				Name:      "run_duration_seconds",
				Help:      "Duration of watchdog check cycles in seconds",
				Buckets:   []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
			},
		),
	}
}

// =============================================================================
// Interfaces
// =============================================================================

// SessionProvider defines the interface for session-related operations.
// This is a subset of WhatsAppProvider to allow easier testing.
type SessionProvider interface {
	// GetSessionStatus retrieves the current status of a WhatsApp session.
	GetSessionStatus(ctx context.Context, sessionName string) (*whatsapp.SessionInfo, error)

	// RestartSession restarts an existing WhatsApp session.
	RestartSession(ctx context.Context, sessionName string) error
}

// EventTimeStore defines the interface for storing and retrieving event times.
// Default implementation uses Redis for cross-instance tracking.
type EventTimeStore interface {
	// SetLastEventTime records the last event time for a session.
	SetLastEventTime(ctx context.Context, sessionName string, t time.Time) error

	// GetLastEventTime retrieves the last event time for a session.
	// Returns zero time if no event recorded.
	GetLastEventTime(ctx context.Context, sessionName string) (time.Time, error)

	// GetAllLastEventTimes retrieves all session last event times.
	GetAllLastEventTimes(ctx context.Context) (map[string]time.Time, error)

	// RemoveSession removes a session from tracking.
	RemoveSession(ctx context.Context, sessionName string) error
}

// SessionDiscovery defines the interface for discovering active sessions.
type SessionDiscovery interface {
	// GetActiveSessions returns all active session names.
	GetActiveSessions(ctx context.Context) ([]string, error)
}

// =============================================================================
// Redis Event Time Store
// =============================================================================

// RedisEventTimeStore implements EventTimeStore using Redis.
type RedisEventTimeStore struct {
	redis     *redis.Client
	keyPrefix string
	ttl       time.Duration
	logger    *zap.Logger
}

// NewRedisEventTimeStore creates a new Redis-based event time store.
func NewRedisEventTimeStore(redisClient *redis.Client, keyPrefix string, ttl time.Duration, logger *zap.Logger) *RedisEventTimeStore {
	return &RedisEventTimeStore{
		redis:     redisClient,
		keyPrefix: keyPrefix,
		ttl:       ttl,
		logger:    logger.Named("event-time-store"),
	}
}

// eventTimeKey returns the Redis key for a session's last event time.
func (s *RedisEventTimeStore) eventTimeKey(sessionName string) string {
	return fmt.Sprintf("%s:last_event:%s", s.keyPrefix, sessionName)
}

// sessionsSetKey returns the Redis key for the set of tracked sessions.
func (s *RedisEventTimeStore) sessionsSetKey() string {
	return fmt.Sprintf("%s:sessions", s.keyPrefix)
}

// SetLastEventTime implements EventTimeStore.
func (s *RedisEventTimeStore) SetLastEventTime(ctx context.Context, sessionName string, t time.Time) error {
	pipe := s.redis.Pipeline()

	// Store the timestamp
	key := s.eventTimeKey(sessionName)
	pipe.Set(ctx, key, t.UnixMilli(), s.ttl)

	// Add to sessions set
	pipe.SAdd(ctx, s.sessionsSetKey(), sessionName)
	pipe.Expire(ctx, s.sessionsSetKey(), s.ttl)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to set last event time: %w", err)
	}

	s.logger.Debug("Set last event time",
		zap.String("session", sessionName),
		zap.Time("time", t))

	return nil
}

// GetLastEventTime implements EventTimeStore.
func (s *RedisEventTimeStore) GetLastEventTime(ctx context.Context, sessionName string) (time.Time, error) {
	key := s.eventTimeKey(sessionName)
	result, err := s.redis.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return time.Time{}, nil
		}
		return time.Time{}, fmt.Errorf("failed to get last event time: %w", err)
	}

	millis, err := strconv.ParseInt(result, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse timestamp: %w", err)
	}

	return time.UnixMilli(millis), nil
}

// GetAllLastEventTimes implements EventTimeStore.
func (s *RedisEventTimeStore) GetAllLastEventTimes(ctx context.Context) (map[string]time.Time, error) {
	// Get all session names from the set
	sessions, err := s.redis.SMembers(ctx, s.sessionsSetKey()).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return make(map[string]time.Time), nil
		}
		return nil, fmt.Errorf("failed to get sessions: %w", err)
	}

	if len(sessions) == 0 {
		return make(map[string]time.Time), nil
	}

	// Build keys for all sessions
	keys := make([]string, len(sessions))
	for i, session := range sessions {
		keys[i] = s.eventTimeKey(session)
	}

	// Get all timestamps in a single MGET
	values, err := s.redis.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get event times: %w", err)
	}

	result := make(map[string]time.Time, len(sessions))
	for i, session := range sessions {
		if values[i] == nil {
			continue
		}

		str, ok := values[i].(string)
		if !ok {
			continue
		}

		millis, err := strconv.ParseInt(str, 10, 64)
		if err != nil {
			s.logger.Warn("Failed to parse timestamp",
				zap.String("session", session),
				zap.Error(err))
			continue
		}

		result[session] = time.UnixMilli(millis)
	}

	return result, nil
}

// RemoveSession implements EventTimeStore.
func (s *RedisEventTimeStore) RemoveSession(ctx context.Context, sessionName string) error {
	pipe := s.redis.Pipeline()
	pipe.Del(ctx, s.eventTimeKey(sessionName))
	pipe.SRem(ctx, s.sessionsSetKey(), sessionName)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to remove session: %w", err)
	}

	s.logger.Debug("Removed session from tracking",
		zap.String("session", sessionName))

	return nil
}

// =============================================================================
// Instance Repository Session Discovery
// =============================================================================

// InstanceRepoSessionDiscovery implements SessionDiscovery using InstanceRepository.
type InstanceRepoSessionDiscovery struct {
	instanceRepo repository.InstanceRepository
	logger       *zap.Logger
}

// NewInstanceRepoSessionDiscovery creates a new InstanceRepository-based session discovery.
func NewInstanceRepoSessionDiscovery(repo repository.InstanceRepository, logger *zap.Logger) *InstanceRepoSessionDiscovery {
	return &InstanceRepoSessionDiscovery{
		instanceRepo: repo,
		logger:       logger.Named("session-discovery"),
	}
}

// GetActiveSessions implements SessionDiscovery.
func (d *InstanceRepoSessionDiscovery) GetActiveSessions(ctx context.Context) ([]string, error) {
	instances, err := d.instanceRepo.GetAllConnectedInstances(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get connected instances: %w", err)
	}

	sessions := make([]string, 0, len(instances))
	for _, inst := range instances {
		if inst.InstanceID != "" {
			sessions = append(sessions, inst.InstanceID)
		}
	}

	return sessions, nil
}

// =============================================================================
// Watchdog Service
// =============================================================================

// Service is the WAHA watchdog service that monitors for silence and triggers reconnection.
type Service struct {
	provider         SessionProvider
	eventStore       EventTimeStore
	sessionDiscovery SessionDiscovery
	config           Config
	logger           *zap.Logger
	metrics          *Metrics

	// Reconnect tracking: tracks attempts per session (in-memory for this instance)
	reconnectAttempts map[string]int
	lastReconnectTime map[string]time.Time
	reconnectMu       sync.Mutex

	// Running state
	running bool
	stopCh  chan struct{}
	wg      sync.WaitGroup
	mu      sync.Mutex
}

// New creates a new WatchdogService.
//
// Parameters:
//   - provider: Session provider for health checks and restarts (typically WahaAdapter)
//   - eventStore: Store for tracking last event times (typically RedisEventTimeStore)
//   - sessionDiscovery: Discovery for finding active sessions (typically InstanceRepoSessionDiscovery)
//   - config: Service configuration
//   - logger: Zap logger for structured logging
//   - metrics: Optional Prometheus metrics (pass nil to disable metrics)
func New(
	provider SessionProvider,
	eventStore EventTimeStore,
	sessionDiscovery SessionDiscovery,
	config Config,
	logger *zap.Logger,
	metrics *Metrics,
) (*Service, error) {
	if provider == nil {
		return nil, errors.New("provider is required")
	}
	if eventStore == nil {
		return nil, errors.New("event store is required")
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	if logger == nil {
		logger = zap.NewNop()
	}

	return &Service{
		provider:          provider,
		eventStore:        eventStore,
		sessionDiscovery:  sessionDiscovery,
		config:            config,
		logger:            logger.Named("watchdog"),
		metrics:           metrics,
		reconnectAttempts: make(map[string]int),
		lastReconnectTime: make(map[string]time.Time),
	}, nil
}

// =============================================================================
// Event Recording (called by webhook handler)
// =============================================================================

// RecordEvent records that an event was received for a session.
// This resets the silence timer for the session.
// Should be called by the webhook handler for every received event.
func (s *Service) RecordEvent(ctx context.Context, sessionName string) error {
	if !s.config.Enabled {
		return nil
	}

	now := time.Now()
	if err := s.eventStore.SetLastEventTime(ctx, sessionName, now); err != nil {
		s.logger.Error("Failed to record event",
			zap.String("session", sessionName),
			zap.Error(err))
		return err
	}

	// Reset reconnect attempts on successful event
	s.resetReconnectAttempts(sessionName)

	s.logger.Debug("Recorded event for session",
		zap.String("session", sessionName))

	return nil
}

// =============================================================================
// Lifecycle Management
// =============================================================================

// Start starts the watchdog service.
// It runs a background goroutine that periodically checks for silence.
func (s *Service) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return errors.New("watchdog already running")
	}

	if !s.config.Enabled {
		s.mu.Unlock()
		s.logger.Info("Watchdog is disabled")
		return nil
	}

	s.running = true
	s.stopCh = make(chan struct{})
	s.mu.Unlock()

	s.wg.Add(1)
	go s.run(ctx)

	s.logger.Info("Watchdog started",
		zap.Duration("silenceThreshold", s.config.SilenceThreshold),
		zap.Duration("checkInterval", s.config.CheckInterval),
		zap.Int("maxReconnectAttempts", s.config.MaxReconnectAttempts))

	return nil
}

// Stop stops the watchdog service gracefully.
func (s *Service) Stop() error {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return nil
	}

	close(s.stopCh)
	s.running = false
	s.mu.Unlock()

	s.wg.Wait()
	s.logger.Info("Watchdog stopped")

	return nil
}

// IsRunning returns whether the watchdog is currently running.
func (s *Service) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// =============================================================================
// Main Loop
// =============================================================================

// run is the main watchdog loop.
func (s *Service) run(ctx context.Context) {
	defer s.wg.Done()

	ticker := time.NewTicker(s.config.CheckInterval)
	defer ticker.Stop()

	s.logger.Info("Watchdog loop started")

	for {
		select {
		case <-s.stopCh:
			s.logger.Info("Watchdog loop stopping due to stop signal")
			return
		case <-ctx.Done():
			s.logger.Info("Watchdog loop stopping due to context cancellation")
			return
		case <-ticker.C:
			s.checkSilence(ctx)
		}
	}
}

// checkSilence checks all sessions for silence and triggers appropriate actions.
func (s *Service) checkSilence(ctx context.Context) {
	startTime := time.Now()
	defer func() {
		if s.metrics != nil {
			s.metrics.WatchdogRunDuration.Observe(time.Since(startTime).Seconds())
		}
	}()

	// Get all sessions with their last event times
	eventTimes, err := s.eventStore.GetAllLastEventTimes(ctx)
	if err != nil {
		s.logger.Error("Failed to get last event times", zap.Error(err))
		return
	}

	// Also check sessions from discovery that might not have events yet
	if s.sessionDiscovery != nil {
		activeSessions, err := s.sessionDiscovery.GetActiveSessions(ctx)
		if err != nil {
			s.logger.Warn("Failed to get active sessions", zap.Error(err))
		} else {
			for _, session := range activeSessions {
				if _, exists := eventTimes[session]; !exists {
					// Session exists but has no recorded events - treat as silent
					eventTimes[session] = time.Time{}
				}
			}
		}
	}

	if s.metrics != nil {
		s.metrics.ActiveSessions.Set(float64(len(eventTimes)))
	}

	now := time.Now()
	for sessionName, lastEvent := range eventTimes {
		// Calculate silence duration
		var silenceDuration time.Duration
		if lastEvent.IsZero() {
			// No events recorded - use a large duration
			silenceDuration = s.config.SilenceThreshold + time.Hour
		} else {
			silenceDuration = now.Sub(lastEvent)
		}

		// Update metrics
		if s.metrics != nil {
			s.metrics.LastEventAge.WithLabelValues(sessionName).Set(silenceDuration.Seconds())
		}

		// Check if silence threshold exceeded
		if silenceDuration >= s.config.SilenceThreshold {
			s.handleSilentSession(ctx, sessionName, silenceDuration)
		}
	}
}

// handleSilentSession handles a session that has been silent for too long.
func (s *Service) handleSilentSession(ctx context.Context, sessionName string, silenceDuration time.Duration) {
	s.logger.Info("Silence detected for session",
		zap.String("session", sessionName),
		zap.Duration("silenceDuration", silenceDuration))

	if s.metrics != nil {
		s.metrics.SilenceDetected.WithLabelValues(sessionName).Inc()
	}

	// Step 1: Health check
	healthy, err := s.checkSessionHealth(ctx, sessionName)
	if err != nil {
		s.logger.Error("Health check failed",
			zap.String("session", sessionName),
			zap.Error(err))
		if s.metrics != nil {
			s.metrics.HealthCheckTotal.WithLabelValues(sessionName, "error").Inc()
		}
	} else if healthy {
		s.logger.Info("Session is healthy despite silence",
			zap.String("session", sessionName))
		if s.metrics != nil {
			s.metrics.HealthCheckTotal.WithLabelValues(sessionName, "healthy").Inc()
		}
		// Session is healthy - record a synthetic event to reset timer
		s.recordSyntheticEvent(ctx, sessionName)
		return
	} else {
		if s.metrics != nil {
			s.metrics.HealthCheckTotal.WithLabelValues(sessionName, "unhealthy").Inc()
		}
	}

	// Step 2: Attempt reconnection if health check failed
	if s.config.MaxReconnectAttempts > 0 {
		s.attemptReconnect(ctx, sessionName)
	}
}

// =============================================================================
// Health Check
// =============================================================================

// checkSessionHealth checks if a session is healthy (connected).
func (s *Service) checkSessionHealth(ctx context.Context, sessionName string) (bool, error) {
	// Use a timeout for the health check
	checkCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	status, err := s.provider.GetSessionStatus(checkCtx, sessionName)
	if err != nil {
		return false, fmt.Errorf("failed to get session status: %w", err)
	}

	isHealthy := status.Status == whatsapp.SessionStatusConnected
	s.logger.Debug("Session health check result",
		zap.String("session", sessionName),
		zap.String("status", string(status.Status)),
		zap.Bool("healthy", isHealthy))

	return isHealthy, nil
}

// =============================================================================
// Reconnection
// =============================================================================

// attemptReconnect attempts to reconnect a session with exponential backoff.
func (s *Service) attemptReconnect(ctx context.Context, sessionName string) {
	// Check cooldown and max attempts
	if !s.canAttemptReconnect(sessionName) {
		s.logger.Debug("Reconnect skipped due to cooldown or max attempts",
			zap.String("session", sessionName))
		if s.metrics != nil {
			s.metrics.ReconnectTotal.WithLabelValues(sessionName, "skipped").Inc()
		}
		return
	}

	s.reconnectMu.Lock()
	attempts := s.reconnectAttempts[sessionName]
	s.reconnectAttempts[sessionName] = attempts + 1
	s.lastReconnectTime[sessionName] = time.Now()
	s.reconnectMu.Unlock()

	s.logger.Info("Attempting to reconnect session",
		zap.String("session", sessionName),
		zap.Int("attempt", attempts+1),
		zap.Int("maxAttempts", s.config.MaxReconnectAttempts))

	// Use a timeout for the reconnect
	reconnectCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	err := s.provider.RestartSession(reconnectCtx, sessionName)
	if err != nil {
		s.logger.Error("Failed to restart session",
			zap.String("session", sessionName),
			zap.Error(err))
		if s.metrics != nil {
			s.metrics.ReconnectTotal.WithLabelValues(sessionName, "failed").Inc()
		}
		return
	}

	// Wait for session to stabilize
	s.waitForReconnect(ctx, sessionName)
}

// waitForReconnect waits for a session to reconnect after restart.
func (s *Service) waitForReconnect(ctx context.Context, sessionName string) {
	// Wait with exponential backoff, checking status periodically
	checkIntervals := []time.Duration{5 * time.Second, 10 * time.Second, 15 * time.Second}

	for i, interval := range checkIntervals {
		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}

		status, err := s.provider.GetSessionStatus(ctx, sessionName)
		if err != nil {
			s.logger.Warn("Failed to check session status after restart",
				zap.String("session", sessionName),
				zap.Int("check", i+1),
				zap.Error(err))
			continue
		}

		if status.Status == whatsapp.SessionStatusConnected {
			s.logger.Info("Session reconnected successfully",
				zap.String("session", sessionName))
			if s.metrics != nil {
				s.metrics.ReconnectTotal.WithLabelValues(sessionName, "success").Inc()
			}
			s.resetReconnectAttempts(sessionName)
			s.recordSyntheticEvent(ctx, sessionName)
			return
		}

		s.logger.Debug("Session not yet connected after restart",
			zap.String("session", sessionName),
			zap.String("status", string(status.Status)),
			zap.Int("check", i+1))
	}

	s.logger.Warn("Session did not reconnect within expected time",
		zap.String("session", sessionName))
	if s.metrics != nil {
		s.metrics.ReconnectTotal.WithLabelValues(sessionName, "failed").Inc()
	}
}

// canAttemptReconnect checks if we can attempt reconnection for a session.
func (s *Service) canAttemptReconnect(sessionName string) bool {
	s.reconnectMu.Lock()
	defer s.reconnectMu.Unlock()

	attempts := s.reconnectAttempts[sessionName]
	lastReconnect := s.lastReconnectTime[sessionName]

	// Check max attempts
	if attempts >= s.config.MaxReconnectAttempts {
		return false
	}

	// Check cooldown with exponential backoff
	if !lastReconnect.IsZero() {
		// Calculate backoff: base * 2^attempts with jitter
		backoff := s.config.ReconnectBackoff * time.Duration(1<<uint(attempts))
		if backoff > s.config.MaxReconnectBackoff {
			backoff = s.config.MaxReconnectBackoff
		}
		// Add jitter: +/- 20%
		jitter := time.Duration(float64(backoff) * 0.2 * (rand.Float64()*2 - 1))
		backoff += jitter

		if time.Since(lastReconnect) < backoff {
			return false
		}
	}

	return true
}

// resetReconnectAttempts resets the reconnection attempt counter for a session.
func (s *Service) resetReconnectAttempts(sessionName string) {
	s.reconnectMu.Lock()
	defer s.reconnectMu.Unlock()

	delete(s.reconnectAttempts, sessionName)
	delete(s.lastReconnectTime, sessionName)
}

// recordSyntheticEvent records a synthetic event to reset the silence timer.
func (s *Service) recordSyntheticEvent(ctx context.Context, sessionName string) {
	if err := s.eventStore.SetLastEventTime(ctx, sessionName, time.Now()); err != nil {
		s.logger.Warn("Failed to record synthetic event",
			zap.String("session", sessionName),
			zap.Error(err))
	}
}

// =============================================================================
// Manual Operations
// =============================================================================

// ForceHealthCheck manually triggers a health check for a session.
func (s *Service) ForceHealthCheck(ctx context.Context, sessionName string) (bool, error) {
	return s.checkSessionHealth(ctx, sessionName)
}

// ForceReconnect manually triggers a reconnection attempt for a session.
// This bypasses cooldown and attempt limits.
func (s *Service) ForceReconnect(ctx context.Context, sessionName string) error {
	s.logger.Info("Force reconnecting session",
		zap.String("session", sessionName))

	err := s.provider.RestartSession(ctx, sessionName)
	if err != nil {
		return fmt.Errorf("failed to restart session: %w", err)
	}

	s.waitForReconnect(ctx, sessionName)
	return nil
}

// GetSessionStatus returns the current silence status for a session.
func (s *Service) GetSessionStatus(ctx context.Context, sessionName string) (*SessionStatus, error) {
	lastEvent, err := s.eventStore.GetLastEventTime(ctx, sessionName)
	if err != nil {
		return nil, fmt.Errorf("failed to get last event time: %w", err)
	}

	var silenceDuration time.Duration
	if !lastEvent.IsZero() {
		silenceDuration = time.Since(lastEvent)
	}

	s.reconnectMu.Lock()
	reconnectAttempts := s.reconnectAttempts[sessionName]
	lastReconnect := s.lastReconnectTime[sessionName]
	s.reconnectMu.Unlock()

	return &SessionStatus{
		SessionName:       sessionName,
		LastEventTime:     lastEvent,
		SilenceDuration:   silenceDuration,
		IsSilent:          silenceDuration >= s.config.SilenceThreshold,
		ReconnectAttempts: reconnectAttempts,
		LastReconnectTime: lastReconnect,
	}, nil
}

// SessionStatus contains the current status of a session from the watchdog's perspective.
type SessionStatus struct {
	// SessionName is the session identifier.
	SessionName string

	// LastEventTime is the time of the last received event.
	LastEventTime time.Time

	// SilenceDuration is how long since the last event.
	SilenceDuration time.Duration

	// IsSilent indicates if the session exceeds the silence threshold.
	IsSilent bool

	// ReconnectAttempts is the current reconnection attempt count.
	ReconnectAttempts int

	// LastReconnectTime is when the last reconnection was attempted.
	LastReconnectTime time.Time
}

// GetAllStatuses returns the status for all tracked sessions.
func (s *Service) GetAllStatuses(ctx context.Context) ([]SessionStatus, error) {
	eventTimes, err := s.eventStore.GetAllLastEventTimes(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get event times: %w", err)
	}

	now := time.Now()
	statuses := make([]SessionStatus, 0, len(eventTimes))

	s.reconnectMu.Lock()
	defer s.reconnectMu.Unlock()

	for sessionName, lastEvent := range eventTimes {
		var silenceDuration time.Duration
		if !lastEvent.IsZero() {
			silenceDuration = now.Sub(lastEvent)
		}

		statuses = append(statuses, SessionStatus{
			SessionName:       sessionName,
			LastEventTime:     lastEvent,
			SilenceDuration:   silenceDuration,
			IsSilent:          silenceDuration >= s.config.SilenceThreshold,
			ReconnectAttempts: s.reconnectAttempts[sessionName],
			LastReconnectTime: s.lastReconnectTime[sessionName],
		})
	}

	return statuses, nil
}
