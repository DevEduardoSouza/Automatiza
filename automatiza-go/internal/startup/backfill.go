// Package startup provides initialization services that run when the server starts.
// This includes backfill jobs that ensure data consistency after server downtime.
package startup

import (
	"context"
	"fmt"
	"sync"
	"time"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"
	"automatiza-go/internal/services/reconciliation"
	"automatiza-go/internal/workers"

	"go.uber.org/zap"
)

// BackfillConfig holds configuration for the backfill service.
type BackfillConfig struct {
	// MaxConcurrent limits parallel session checks.
	// Default: 5.
	MaxConcurrent int

	// Timeout for the entire backfill operation.
	// Default: 10 minutes.
	Timeout time.Duration

	// GapThreshold is the minimum gap (in seconds) to trigger reconciliation.
	// If the difference between WAHA's last message and our checkpoint is greater
	// than this threshold, reconciliation is triggered.
	// Default: 60 seconds.
	GapThreshold time.Duration

	// SkipHealthCheck skips WAHA session health verification.
	// Useful for testing. Default: false.
	SkipHealthCheck bool

	// ContinueOnError continues processing other sessions if one fails.
	// Default: true.
	ContinueOnError bool
}

// DefaultBackfillConfig returns sensible defaults for backfill configuration.
func DefaultBackfillConfig() BackfillConfig {
	return BackfillConfig{
		MaxConcurrent:   5,
		Timeout:         10 * time.Minute,
		GapThreshold:    60 * time.Second,
		SkipHealthCheck: false,
		ContinueOnError: true,
	}
}

// Validate checks configuration and applies defaults for zero values.
func (c *BackfillConfig) Validate() {
	if c.MaxConcurrent <= 0 {
		c.MaxConcurrent = 5
	}
	if c.Timeout <= 0 {
		c.Timeout = 10 * time.Minute
	}
	if c.GapThreshold <= 0 {
		c.GapThreshold = 60 * time.Second
	}
}

// BackfillService handles startup reconciliation to recover messages missed during downtime.
// It performs a comprehensive check of all active WAHA sessions, compares checkpoints
// with the actual last message in WAHA, and enqueues reconciliation tasks for gaps.
type BackfillService struct {
	instanceRepo   repository.InstanceRepository
	checkpointRepo reconciliation.CheckpointRepository
	wahaProvider   whatsapp.WhatsAppProvider
	queueClient    *workers.QueueClient
	reconcileSvc   *reconciliation.Service
	logger         *zap.Logger
	config         BackfillConfig
}

// BackfillResult contains statistics from a backfill run.
type BackfillResult struct {
	// TotalSessions is the number of active sessions found.
	TotalSessions int

	// SessionsChecked is the number of sessions successfully checked.
	SessionsChecked int

	// SessionsWithGaps is the number of sessions where gaps were detected.
	SessionsWithGaps int

	// ReconciliationsEnqueued is the number of reconciliation tasks enqueued.
	ReconciliationsEnqueued int

	// Errors contains any errors encountered during processing.
	Errors []error

	// Duration is how long the backfill took.
	Duration time.Duration
}

// NewBackfillService creates a new BackfillService.
//
// Parameters:
//   - instanceRepo: Repository to fetch active WAHA sessions from the database
//   - checkpointRepo: Repository to access reconciliation checkpoints
//   - wahaProvider: WhatsApp provider adapter for checking session status and messages
//   - queueClient: Queue client for enqueueing reconciliation tasks
//   - reconcileSvc: Optional reconciliation service for direct reconciliation (can be nil)
//   - logger: Zap logger for structured logging
//   - config: Backfill configuration
func NewBackfillService(
	instanceRepo repository.InstanceRepository,
	checkpointRepo reconciliation.CheckpointRepository,
	wahaProvider whatsapp.WhatsAppProvider,
	queueClient *workers.QueueClient,
	reconcileSvc *reconciliation.Service,
	logger *zap.Logger,
	config BackfillConfig,
) *BackfillService {
	config.Validate()

	return &BackfillService{
		instanceRepo:   instanceRepo,
		checkpointRepo: checkpointRepo,
		wahaProvider:   wahaProvider,
		queueClient:    queueClient,
		reconcileSvc:   reconcileSvc,
		logger:         logger.Named("backfill"),
		config:         config,
	}
}

// RunOnStartup executes the backfill job during server startup.
// This method is safe to call concurrently and will complete within the configured timeout.
//
// The backfill process:
//  1. Fetches all active (connected) WAHA sessions from the database
//  2. For each session, checks its health status with WAHA
//  3. Retrieves the last checkpoint timestamp from the database
//  4. Compares with the latest message timestamp from WAHA
//  5. If a gap is detected (beyond threshold), enqueues a reconciliation task
//
// Returns a BackfillResult with statistics and any errors encountered.
func (s *BackfillService) RunOnStartup(ctx context.Context) (*BackfillResult, error) {
	startTime := time.Now()

	// Apply timeout to the entire operation
	ctx, cancel := context.WithTimeout(ctx, s.config.Timeout)
	defer cancel()

	s.logger.Info("Starting backfill job",
		zap.Duration("timeout", s.config.Timeout),
		zap.Int("maxConcurrent", s.config.MaxConcurrent),
		zap.Duration("gapThreshold", s.config.GapThreshold))

	result := &BackfillResult{}

	// Step 1: Fetch all active (connected) sessions from the database
	instances, err := s.instanceRepo.GetAllConnectedInstances(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch connected instances: %w", err)
	}

	result.TotalSessions = len(instances)

	if len(instances) == 0 {
		s.logger.Info("No active sessions found, backfill complete")
		result.Duration = time.Since(startTime)
		return result, nil
	}

	s.logger.Info("Found active sessions to check",
		zap.Int("count", len(instances)))

	// Step 2: Process sessions concurrently with bounded parallelism
	var (
		wg          sync.WaitGroup
		mu          sync.Mutex
		semaphore   = make(chan struct{}, s.config.MaxConcurrent)
		errorsMu    sync.Mutex
		allErrors   []error
		checked     int
		withGaps    int
		enqueued    int
	)

	for _, instance := range instances {
		// Check context cancellation
		select {
		case <-ctx.Done():
			s.logger.Warn("Backfill cancelled due to timeout")
			break
		default:
		}

		wg.Add(1)
		go func(inst models.Instance) {
			defer wg.Done()

			// Acquire semaphore slot
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-ctx.Done():
				return
			}

			sessionName := inst.InstanceID

			// Process single session
			hasGap, enqueuedCount, err := s.processSession(ctx, sessionName)

			mu.Lock()
			checked++
			if hasGap {
				withGaps++
			}
			enqueued += enqueuedCount
			mu.Unlock()

			if err != nil {
				errorsMu.Lock()
				allErrors = append(allErrors, fmt.Errorf("session %s: %w", sessionName, err))
				errorsMu.Unlock()

				s.logger.Error("Failed to process session for backfill",
					zap.String("session", sessionName),
					zap.Error(err))

				if !s.config.ContinueOnError {
					cancel()
				}
			}
		}(instance)
	}

	wg.Wait()

	result.SessionsChecked = checked
	result.SessionsWithGaps = withGaps
	result.ReconciliationsEnqueued = enqueued
	result.Errors = allErrors
	result.Duration = time.Since(startTime)

	s.logger.Info("Backfill job completed",
		zap.Int("totalSessions", result.TotalSessions),
		zap.Int("sessionsChecked", result.SessionsChecked),
		zap.Int("sessionsWithGaps", result.SessionsWithGaps),
		zap.Int("reconciliationsEnqueued", result.ReconciliationsEnqueued),
		zap.Int("errors", len(result.Errors)),
		zap.Duration("duration", result.Duration))

	return result, nil
}

// processSession handles backfill for a single WAHA session.
// Returns whether a gap was detected, the count of reconciliation tasks enqueued, and any error.
func (s *BackfillService) processSession(ctx context.Context, sessionName string) (bool, int, error) {
	s.logger.Debug("Processing session for backfill",
		zap.String("session", sessionName))

	// Step 1: Check session health (optional)
	if !s.config.SkipHealthCheck {
		status, err := s.wahaProvider.GetSessionStatus(ctx, sessionName)
		if err != nil {
			s.logger.Warn("Failed to get session status",
				zap.String("session", sessionName),
				zap.Error(err))
			// Continue anyway - session might still be usable
		} else if status.Status != whatsapp.SessionStatusConnected {
			s.logger.Info("Skipping disconnected session",
				zap.String("session", sessionName),
				zap.String("status", string(status.Status)))
			return false, 0, nil
		}
	}

	// Step 2: Get checkpoints for this session from the database
	checkpoints, err := s.checkpointRepo.GetBySession(ctx, sessionName)
	if err != nil {
		return false, 0, fmt.Errorf("failed to get checkpoints: %w", err)
	}

	if len(checkpoints) == 0 {
		s.logger.Debug("No checkpoints found for session, triggering full session reconciliation",
			zap.String("session", sessionName))

		// No checkpoints means we have never tracked this session.
		// Enqueue a session-level reconciliation to discover chats and create checkpoints.
		_, err := s.queueClient.EnqueueReconcileSession(ctx, sessionName)
		if err != nil {
			return false, 0, fmt.Errorf("failed to enqueue session reconciliation: %w", err)
		}
		return true, 1, nil
	}

	// Step 3: Check each chat for gaps
	var (
		hasAnyGap    bool
		enqueuedCount int
	)

	for _, checkpoint := range checkpoints {
		hasGap, err := s.checkChatForGap(ctx, sessionName, checkpoint)
		if err != nil {
			s.logger.Warn("Failed to check chat for gap",
				zap.String("session", sessionName),
				zap.String("chat", checkpoint.ChatID),
				zap.Error(err))
			continue
		}

		if hasGap {
			hasAnyGap = true

			// Enqueue chat reconciliation
			_, err := s.queueClient.EnqueueReconcileChat(ctx, sessionName, checkpoint.ChatID)
			if err != nil {
				s.logger.Error("Failed to enqueue chat reconciliation",
					zap.String("session", sessionName),
					zap.String("chat", checkpoint.ChatID),
					zap.Error(err))
				continue
			}

			enqueuedCount++
			s.logger.Info("Gap detected, reconciliation enqueued",
				zap.String("session", sessionName),
				zap.String("chat", checkpoint.ChatID),
				zap.Int64("checkpointTimestamp", checkpoint.LastMessageTimestamp))
		}
	}

	return hasAnyGap, enqueuedCount, nil
}

// checkChatForGap compares our checkpoint with WAHA to detect message gaps.
func (s *BackfillService) checkChatForGap(
	ctx context.Context,
	sessionName string,
	checkpoint reconciliation.Checkpoint,
) (bool, error) {
	// Fetch the latest message from WAHA for this chat
	result, err := s.wahaProvider.GetMessages(ctx, whatsapp.GetMessagesInput{
		SessionName:   sessionName,
		ChatID:        checkpoint.ChatID,
		Limit:         1, // We only need the most recent message
		Offset:        0,
		DownloadMedia: false,
	})
	if err != nil {
		return false, fmt.Errorf("failed to fetch messages from WAHA: %w", err)
	}

	if len(result.Messages) == 0 {
		// No messages in chat - no gap
		s.logger.Debug("No messages in chat",
			zap.String("session", sessionName),
			zap.String("chat", checkpoint.ChatID))
		return false, nil
	}

	// Get the most recent message
	latestMsg := result.Messages[0]
	latestTimestamp := latestMsg.Timestamp.UnixMilli()
	checkpointTimestamp := checkpoint.LastMessageTimestamp

	// If checkpoint is 0, we have never processed this chat
	if checkpointTimestamp == 0 {
		s.logger.Debug("Checkpoint timestamp is 0, gap detected",
			zap.String("session", sessionName),
			zap.String("chat", checkpoint.ChatID))
		return true, nil
	}

	// Calculate the gap
	gap := time.Duration(latestTimestamp-checkpointTimestamp) * time.Millisecond

	s.logger.Debug("Gap calculation",
		zap.String("session", sessionName),
		zap.String("chat", checkpoint.ChatID),
		zap.Int64("latestTimestamp", latestTimestamp),
		zap.Int64("checkpointTimestamp", checkpointTimestamp),
		zap.Duration("gap", gap),
		zap.Duration("threshold", s.config.GapThreshold))

	// If gap exceeds threshold, we need reconciliation
	if gap > s.config.GapThreshold {
		return true, nil
	}

	// Also check if the latest message ID differs from checkpoint
	// This handles edge cases where timestamps are close but messages differ
	if checkpoint.LastMessageID != "" && latestMsg.ID != checkpoint.LastMessageID {
		// Messages are different - there might be newer messages
		// Only flag as gap if the latest message is newer
		if latestTimestamp > checkpointTimestamp {
			return true, nil
		}
	}

	return false, nil
}

// RunWithReconciliationService runs backfill using the reconciliation service directly
// instead of enqueueing tasks. This is useful for immediate reconciliation during startup.
//
// This method calls reconcileSvc.OnStartup internally but adds the additional gap detection
// logic for sessions that don't have checkpoints yet.
func (s *BackfillService) RunWithReconciliationService(ctx context.Context) (*BackfillResult, error) {
	if s.reconcileSvc == nil {
		return nil, fmt.Errorf("reconciliation service not configured")
	}

	startTime := time.Now()

	// Apply timeout to the entire operation
	ctx, cancel := context.WithTimeout(ctx, s.config.Timeout)
	defer cancel()

	s.logger.Info("Starting direct backfill with reconciliation service")

	result := &BackfillResult{}

	// Step 1: Run the existing reconciliation service startup
	if err := s.reconcileSvc.OnStartup(ctx); err != nil {
		s.logger.Warn("Reconciliation service OnStartup encountered errors",
			zap.Error(err))
		result.Errors = append(result.Errors, err)
	}

	// Step 2: Check for sessions without checkpoints and trigger reconciliation
	instances, err := s.instanceRepo.GetAllConnectedInstances(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch connected instances: %w", err)
	}

	result.TotalSessions = len(instances)

	for _, instance := range instances {
		select {
		case <-ctx.Done():
			break
		default:
		}

		sessionName := instance.InstanceID

		// Check if session has any checkpoints
		checkpoints, err := s.checkpointRepo.GetBySession(ctx, sessionName)
		if err != nil {
			s.logger.Warn("Failed to get checkpoints for session",
				zap.String("session", sessionName),
				zap.Error(err))
			result.Errors = append(result.Errors, err)
			continue
		}

		result.SessionsChecked++

		if len(checkpoints) == 0 {
			// Session has no checkpoints - trigger full session reconciliation
			s.logger.Info("Session has no checkpoints, triggering reconciliation",
				zap.String("session", sessionName))

			if err := s.reconcileSvc.ReconcileSession(ctx, sessionName); err != nil {
				s.logger.Error("Failed to reconcile session",
					zap.String("session", sessionName),
					zap.Error(err))
				result.Errors = append(result.Errors, err)
				continue
			}

			result.SessionsWithGaps++
			result.ReconciliationsEnqueued++
		}
	}

	result.Duration = time.Since(startTime)

	s.logger.Info("Direct backfill completed",
		zap.Int("totalSessions", result.TotalSessions),
		zap.Int("sessionsChecked", result.SessionsChecked),
		zap.Int("sessionsWithGaps", result.SessionsWithGaps),
		zap.Int("reconciliationsEnqueued", result.ReconciliationsEnqueued),
		zap.Int("errors", len(result.Errors)),
		zap.Duration("duration", result.Duration))

	return result, nil
}
