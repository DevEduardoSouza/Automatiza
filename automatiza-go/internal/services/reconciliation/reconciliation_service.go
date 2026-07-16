// Package reconciliation provides message reconciliation and backfill services.
// It implements gap detection via silence watchdog and recovery using WAHA GetMessages API.
//
// Key responsibilities:
//   - Track last processed message per session/chat via checkpoints
//   - Detect silence (no events) and trigger reconciliation
//   - Fetch missing messages from WAHA and process them idempotently
//   - Run startup reconciliation to recover from restarts
//
// Architecture:
//   - Uses WhatsAppProvider interface (port.go) for fetching messages
//   - Stores checkpoints in PostgreSQL via CheckpointRepository
//   - Processes messages via MessageProcessor interface (consumer)
package reconciliation

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/observability"

	"github.com/getsentry/sentry-go"
	"go.uber.org/zap"
)

// =============================================================================
// Repository Interfaces
// =============================================================================

// CheckpointRepository defines the interface for reconciliation checkpoint storage.
// Implementations handle persistence of checkpoint state in the database.
type CheckpointRepository interface {
	// GetOrCreate retrieves an existing checkpoint or creates a new one with default values.
	// Returns the checkpoint for the given session/chat pair.
	GetOrCreate(ctx context.Context, sessionName, chatID string) (*Checkpoint, error)

	// MarkRunning updates the checkpoint status to "running" if not already running.
	// Returns ErrAlreadyRunning if the checkpoint is already in running state.
	MarkRunning(ctx context.Context, sessionName, chatID string) error

	// MarkIdle updates the checkpoint status to "idle" after successful reconciliation.
	MarkIdle(ctx context.Context, sessionName, chatID string) error

	// MarkFailed updates the checkpoint status to "failed" after reconciliation error.
	MarkFailed(ctx context.Context, sessionName, chatID string) error

	// UpdateCheckpoint updates the last processed message timestamp and ID.
	// Called after successfully processing a batch of messages.
	UpdateCheckpoint(ctx context.Context, sessionName, chatID string, lastTimestamp int64, lastMessageID string) error

	// AdvanceCheckpoint upserts the checkpoint monotonically from the LIVE
	// message path (creates on first message, only moves the watermark
	// forward). This is what seeds checkpoints — without it the silence
	// watchdog and startup backfill have nothing to reconcile.
	AdvanceCheckpoint(ctx context.Context, sessionName, chatID string, lastTimestamp int64, lastMessageID string) error

	// GetActiveSessionsChats retrieves all session/chat pairs that have checkpoints.
	// Used during startup reconciliation to know which chats to reconcile.
	GetActiveSessionsChats(ctx context.Context) ([]SessionChat, error)

	// GetBySession retrieves all checkpoints for a specific session.
	GetBySession(ctx context.Context, sessionName string) ([]Checkpoint, error)

	// ResetStuckCheckpoints devolve 'running'/'failed' antigos para 'idle'
	// (crash no meio de um ReconcileChat não pode vetar o chat para sempre).
	ResetStuckCheckpoints(ctx context.Context, olderThan time.Duration) (int64, error)

	// DeleteInactiveCheckpoints remove checkpoints sem atividade (higiene).
	DeleteInactiveCheckpoints(ctx context.Context, olderThan time.Duration) (int64, error)
}

// =============================================================================
// Domain Models
// =============================================================================

// Checkpoint represents a reconciliation checkpoint for a session/chat pair.
// It tracks the last successfully processed message to enable gap recovery.
type Checkpoint struct {
	// SessionName is the WAHA session identifier (e.g., "user_123_instance_456").
	SessionName string

	// ChatID is the WhatsApp chat JID (e.g., "5511999998888@s.whatsapp.net").
	ChatID string

	// LastMessageTimestamp is the Unix timestamp (milliseconds) of the last processed message.
	LastMessageTimestamp int64

	// LastMessageID is the ID of the last processed message for precise deduplication.
	LastMessageID string

	// LastRunAt is when this checkpoint was last updated.
	LastRunAt time.Time

	// Status is the current reconciliation status: "idle", "running", or "failed".
	Status CheckpointStatus
}

// CheckpointStatus represents the state of a reconciliation checkpoint.
type CheckpointStatus string

const (
	// CheckpointStatusIdle indicates no reconciliation is in progress.
	CheckpointStatusIdle CheckpointStatus = "idle"

	// CheckpointStatusRunning indicates reconciliation is currently running.
	CheckpointStatusRunning CheckpointStatus = "running"

	// CheckpointStatusFailed indicates the last reconciliation attempt failed.
	CheckpointStatusFailed CheckpointStatus = "failed"
)

// SessionChat represents a session and chat ID pair.
type SessionChat struct {
	SessionName string
	ChatID      string
}

// =============================================================================
// Message Processor Interface
// =============================================================================

// MessageProcessor defines the interface for processing reconciled messages.
// Implementations must be idempotent as messages may be processed multiple times.
type MessageProcessor interface {
	// ProcessMessage processes a single message from reconciliation.
	// Must be idempotent - duplicate messages should be handled gracefully.
	// Returns nil if message was successfully processed or already exists.
	ProcessMessage(ctx context.Context, sessionName string, msg *whatsapp.Message) error
}

// =============================================================================
// Errors
// =============================================================================

var (
	// ErrAlreadyRunning indicates a reconciliation is already in progress for this chat.
	ErrAlreadyRunning = errors.New("reconciliation already running for this chat")

	// ErrNoCheckpoint indicates no checkpoint exists for the session/chat.
	ErrNoCheckpoint = errors.New("no checkpoint found")

	// ErrProviderNotConnected indicates the WhatsApp session is not connected.
	ErrProviderNotConnected = errors.New("WhatsApp session not connected")
)

// =============================================================================
// Configuration
// =============================================================================

// Config holds reconciliation service configuration.
type Config struct {
	// SilenceTimeout is how long without events before triggering reconciliation.
	// Default: 5 minutes.
	SilenceTimeout time.Duration

	// BatchSize is the number of messages to fetch per WAHA API call.
	// Default: 50.
	BatchSize int

	// MaxConcurrent is the maximum number of concurrent reconciliations.
	// Default: 5.
	MaxConcurrent int

	// CheckInterval is how often the watchdog checks for silence.
	// Default: 1 minute.
	CheckInterval time.Duration

	// MaxRetries is the maximum number of retry attempts for failed reconciliations.
	// Default: 3.
	MaxRetries int

	// RetryBackoff is the base duration for exponential backoff between retries.
	// Default: 30 seconds.
	RetryBackoff time.Duration

	// HealthCheckEnabled enables health checking of WAHA sessions on silence detection.
	// Default: true.
	HealthCheckEnabled bool

	// MaxRestartAttempts is the maximum number of restart attempts per session.
	// Default: 3. Set to 0 to disable restart attempts.
	MaxRestartAttempts int

	// RestartBackoff is the base duration for exponential backoff between restart attempts.
	// Default: 1 minute.
	RestartBackoff time.Duration

	// MaxBackfillWindow limita o quão para trás o backfill busca mensagens.
	// DEVE ficar alinhado ao staleWebhookMessageTTL do consumidor (10min): o
	// espelho não repassa mensagem mais velha que isso no caminho vivo, e os
	// guards de loop do offer_generate (og_self_send/selfsend, TTL 10min)
	// assumem esse teto. Sem o clamp, um checkpoint recém-criado (timestamp 0)
	// faria o doReconcile paginar o HISTÓRICO INTEIRO do grupo e re-espelhar
	// ofertas antigas. Default: 10 minutos.
	MaxBackfillWindow time.Duration

	// MaxMessagesPerReconcile limita o total de mensagens processadas por
	// ReconcileChat (backstop contra paginação descontrolada). Default: 200.
	MaxMessagesPerReconcile int

	// CheckpointWriteInterval limita a frequência de persistência do watermark
	// vivo por (sessão, chat). Default: 45s — num crash perde-se no máximo
	// isso de watermark, que o backfill re-cobre (dedup absorve o overlap).
	CheckpointWriteInterval time.Duration
}

// DefaultConfig returns the default reconciliation configuration.
func DefaultConfig() Config {
	return Config{
		SilenceTimeout:          5 * time.Minute,
		BatchSize:               50,
		MaxConcurrent:           5,
		CheckInterval:           1 * time.Minute,
		MaxRetries:              3,
		RetryBackoff:            30 * time.Second,
		HealthCheckEnabled:      true,
		MaxRestartAttempts:      3,
		RestartBackoff:          1 * time.Minute,
		MaxBackfillWindow:       10 * time.Minute,
		MaxMessagesPerReconcile: 200,
		CheckpointWriteInterval: 45 * time.Second,
	}
}

// Validate checks if the configuration is valid and applies defaults.
func (c *Config) Validate() error {
	if c.SilenceTimeout <= 0 {
		c.SilenceTimeout = 5 * time.Minute
	}
	if c.BatchSize <= 0 {
		c.BatchSize = 50
	}
	if c.MaxConcurrent <= 0 {
		c.MaxConcurrent = 5
	}
	if c.CheckInterval <= 0 {
		c.CheckInterval = 1 * time.Minute
	}
	if c.MaxRetries <= 0 {
		c.MaxRetries = 3
	}
	if c.RetryBackoff <= 0 {
		c.RetryBackoff = 30 * time.Second
	}
	// HealthCheckEnabled defaults to true (zero value is false, so check explicitly)
	// MaxRestartAttempts can be 0 (disabled), negative values become default
	if c.MaxRestartAttempts < 0 {
		c.MaxRestartAttempts = 3
	}
	if c.RestartBackoff <= 0 {
		c.RestartBackoff = 1 * time.Minute
	}
	if c.MaxBackfillWindow <= 0 {
		c.MaxBackfillWindow = 10 * time.Minute
	}
	if c.MaxMessagesPerReconcile <= 0 {
		c.MaxMessagesPerReconcile = 200
	}
	if c.CheckpointWriteInterval <= 0 {
		c.CheckpointWriteInterval = 45 * time.Second
	}
	return nil
}

// =============================================================================
// Service Implementation
// =============================================================================

// Service manages message reconciliation and backfill operations.
// It implements:
//   - Silence detection via watchdog goroutine
//   - Health checking and session restart with backoff
//   - On-demand chat reconciliation
//   - Startup recovery for all active sessions
type Service struct {
	provider         whatsapp.WhatsAppProvider
	checkpointRepo   CheckpointRepository
	messageProcessor MessageProcessor
	logger           *zap.Logger
	config           Config

	// Silence detection: tracks last event time per session
	lastEventTime map[string]time.Time
	lastEventMu   sync.RWMutex

	// Concurrency control: tracks running reconciliations
	running   map[string]struct{} // key: "sessionName:chatID"
	runningMu sync.Mutex

	// Semaphore for limiting concurrent reconciliations
	semaphore chan struct{}

	// Restart tracking: tracks restart attempts per session for backoff
	restartAttempts map[string]int       // key: sessionName
	lastRestartTime map[string]time.Time // key: sessionName
	restartMu       sync.Mutex

	// Live checkpoint write throttle (RecordMessage): last persist per
	// "session:chat" — bounds DB writes to one per CheckpointWriteInterval.
	checkpointWrites   map[string]time.Time
	checkpointWritesMu sync.Mutex

	// Watchdog maintenance pacing (stuck-reset / inactive purge).
	lastStuckReset      time.Time
	lastCheckpointPurge time.Time
	maintenanceMu       sync.Mutex
}

// New creates a new ReconciliationService.
//
// Parameters:
//   - provider: WhatsApp provider for fetching messages via GetMessages
//   - checkpointRepo: Repository for persisting checkpoint state
//   - messageProcessor: Processor for handling reconciled messages (must be idempotent)
//   - logger: Zap logger for structured logging
//   - config: Service configuration (use DefaultConfig() as base)
func New(
	provider whatsapp.WhatsAppProvider,
	checkpointRepo CheckpointRepository,
	messageProcessor MessageProcessor,
	logger *zap.Logger,
	config Config,
) *Service {
	if err := config.Validate(); err != nil {
		logger.Warn("Invalid reconciliation config, using defaults", zap.Error(err))
		config = DefaultConfig()
	}

	return &Service{
		provider:         provider,
		checkpointRepo:   checkpointRepo,
		messageProcessor: messageProcessor,
		logger:           logger.Named("reconciliation"),
		config:           config,
		lastEventTime:    make(map[string]time.Time),
		running:          make(map[string]struct{}),
		semaphore:        make(chan struct{}, config.MaxConcurrent),
		restartAttempts:  make(map[string]int),
		lastRestartTime:  make(map[string]time.Time),
		checkpointWrites: make(map[string]time.Time),
	}
}

// =============================================================================
// Event Recording (called by webhook handler)
// =============================================================================

// RecordEvent records that an event was received for a session.
// This resets the silence timer for the session.
// Should be called by the webhook handler for every received event.
func (s *Service) RecordEvent(sessionName string) {
	s.lastEventMu.Lock()
	s.lastEventTime[sessionName] = time.Now()
	s.lastEventMu.Unlock()

	s.logger.Debug("Recorded event for session",
		zap.String("session", sessionName))
}

// RecordMessage registra atividade da sessão E avança o checkpoint VIVO do
// chat (persistência limitada por CheckpointWriteInterval). É chamado pelo
// processador de espelho DEPOIS que o fan-out da mensagem conclui — assim o
// watermark cobre apenas o que foi de fato processado: um hard-kill no meio
// do fan-out deixa o checkpoint para trás, o gap fica visível e o backfill
// re-busca a mensagem (o claim de dedup por destino absorve o overlap e só os
// destinos não atendidos recebem).
//
// É este método que SEMEIA os checkpoints: sem ele a tabela ficava vazia
// (0 linhas em produção desde sempre) e watchdog/startup-backfill eram no-ops.
func (s *Service) RecordMessage(sessionName, chatID string, ts time.Time, messageID string) {
	s.RecordEvent(sessionName)

	if chatID == "" || messageID == "" || ts.IsZero() {
		return
	}

	key := sessionName + ":" + chatID
	now := time.Now()
	s.checkpointWritesMu.Lock()
	if last, ok := s.checkpointWrites[key]; ok && now.Sub(last) < s.config.CheckpointWriteInterval {
		s.checkpointWritesMu.Unlock()
		return
	}
	s.checkpointWrites[key] = now
	s.checkpointWritesMu.Unlock()

	// Contexto próprio e curto: chamado da goroutine de fan-out, cujo ctx
	// morre logo depois — o watermark tem que persistir mesmo assim.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.checkpointRepo.AdvanceCheckpoint(ctx, sessionName, chatID, ts.UnixMilli(), messageID); err != nil {
		s.logger.Warn("Failed to advance live checkpoint",
			zap.String("session", sessionName),
			zap.String("chat", chatID),
			zap.Error(err))
		// Libera o throttle para a próxima mensagem tentar de novo.
		s.checkpointWritesMu.Lock()
		delete(s.checkpointWrites, key)
		s.checkpointWritesMu.Unlock()
	}
}

// =============================================================================
// Chat Reconciliation
// =============================================================================

// ReconcileChat reconciles messages for a specific chat.
// It fetches messages from WAHA since the last checkpoint and processes them.
//
// The process is:
//  1. Get or create checkpoint for the session/chat
//  2. Mark checkpoint as running (skip if already running)
//  3. Fetch messages from WAHA since checkpoint timestamp
//  4. Process each message idempotently
//  5. Update checkpoint with last processed message
//  6. Mark checkpoint as idle (or failed on error)
//
// This method is safe to call concurrently for different chats.
// Concurrent calls for the same chat will return ErrAlreadyRunning.
func (s *Service) ReconcileChat(ctx context.Context, sessionName, chatID string) error {
	key := sessionName + ":" + chatID

	// Check if already running
	s.runningMu.Lock()
	if _, running := s.running[key]; running {
		s.runningMu.Unlock()
		s.logger.Debug("Reconciliation already running",
			zap.String("session", sessionName),
			zap.String("chat", chatID))
		return ErrAlreadyRunning
	}
	s.running[key] = struct{}{}
	s.runningMu.Unlock()

	// Ensure cleanup on exit
	defer func() {
		s.runningMu.Lock()
		delete(s.running, key)
		s.runningMu.Unlock()
	}()

	// Acquire semaphore slot
	select {
	case s.semaphore <- struct{}{}:
		defer func() { <-s.semaphore }()
	case <-ctx.Done():
		return ctx.Err()
	}

	s.logger.Info("Starting chat reconciliation",
		zap.String("session", sessionName),
		zap.String("chat", chatID))

	// Get or create checkpoint
	checkpoint, err := s.checkpointRepo.GetOrCreate(ctx, sessionName, chatID)
	if err != nil {
		observability.LogAndCapture(s.logger, sentry.LevelError, "Failed to get or create reconciliation checkpoint", err, map[string]string{
			"module":  "reconciliation",
			"session": sessionName,
			"chat":    chatID,
		})
		return fmt.Errorf("failed to get checkpoint: %w", err)
	}

	// Mark as running in database
	if err := s.checkpointRepo.MarkRunning(ctx, sessionName, chatID); err != nil {
		if errors.Is(err, ErrAlreadyRunning) {
			return err
		}
		observability.LogAndCapture(s.logger, sentry.LevelError, "Failed to mark reconciliation checkpoint as running", err, map[string]string{
			"module":  "reconciliation",
			"session": sessionName,
			"chat":    chatID,
		})
		return fmt.Errorf("failed to mark checkpoint as running: %w", err)
	}

	// Run reconciliation with retry logic
	var reconcileErr error
	for attempt := 1; attempt <= s.config.MaxRetries; attempt++ {
		reconcileErr = s.doReconcile(ctx, sessionName, chatID, checkpoint)
		if reconcileErr == nil {
			break
		}

		if errors.Is(reconcileErr, context.Canceled) || errors.Is(reconcileErr, context.DeadlineExceeded) {
			break
		}

		s.logger.Warn("Reconciliation attempt failed",
			zap.String("session", sessionName),
			zap.String("chat", chatID),
			zap.Int("attempt", attempt),
			zap.Int("maxRetries", s.config.MaxRetries),
			zap.Error(reconcileErr))

		if attempt < s.config.MaxRetries {
			backoff := s.config.RetryBackoff * time.Duration(attempt)
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	// Update checkpoint status based on result
	if reconcileErr != nil {
		observability.LogAndCapture(s.logger, sentry.LevelFatal, "Reconciliation failed after all retry attempts", reconcileErr, map[string]string{
			"module":      "reconciliation",
			"session":     sessionName,
			"chat":        chatID,
			"max_retries": fmt.Sprintf("%d", s.config.MaxRetries),
		})
		if err := s.checkpointRepo.MarkFailed(ctx, sessionName, chatID); err != nil {
			observability.LogAndCapture(s.logger, sentry.LevelError, "Failed to mark reconciliation checkpoint as failed", err, map[string]string{
				"module":  "reconciliation",
				"session": sessionName,
				"chat":    chatID,
			})
		}
		return fmt.Errorf("reconciliation failed after %d attempts: %w", s.config.MaxRetries, reconcileErr)
	}

	if err := s.checkpointRepo.MarkIdle(ctx, sessionName, chatID); err != nil {
		observability.LogAndCapture(s.logger, sentry.LevelError, "Failed to mark reconciliation checkpoint as idle", err, map[string]string{
			"module":  "reconciliation",
			"session": sessionName,
			"chat":    chatID,
		})
	}

	s.logger.Info("Chat reconciliation completed",
		zap.String("session", sessionName),
		zap.String("chat", chatID))

	return nil
}

// doReconcile performs the actual reconciliation work.
func (s *Service) doReconcile(ctx context.Context, sessionName, chatID string, checkpoint *Checkpoint) error {
	// CLAMP de janela: nunca buscar além de MaxBackfillWindow (10min, alinhado
	// ao staleWebhookMessageTTL do caminho vivo). Um checkpoint recém-criado
	// (timestamp 0) sem o clamp faria paginar o HISTÓRICO INTEIRO do grupo e
	// re-espelhar ofertas antigas — os guards de loop (og_self_send/selfsend)
	// e as janelas de claim assumem frescor ≤10min.
	minFrom := time.Now().Add(-s.config.MaxBackfillWindow)
	fromTs := minFrom
	if checkpoint.LastMessageTimestamp > 0 {
		if t := time.UnixMilli(checkpoint.LastMessageTimestamp); t.After(minFrom) {
			fromTs = t
		}
	}
	fromTimestamp := &fromTs

	offset := 0
	totalProcessed := 0
	var lastTimestamp int64
	var lastMessageID string

	for {
		// Check context cancellation
		if err := ctx.Err(); err != nil {
			return err
		}

		// Build filter for messages since checkpoint (clamped above)
		filter := &whatsapp.MessageFilter{
			FromTimestamp: fromTimestamp,
		}

		// Fetch messages from WAHA — ascending so offset paging walks forward
		// in time and the checkpoint advances deterministically.
		result, err := s.provider.GetMessages(ctx, whatsapp.GetMessagesInput{
			SessionName:   sessionName,
			ChatID:        chatID,
			Limit:         s.config.BatchSize,
			Offset:        offset,
			DownloadMedia: false,
			SortAsc:       true,
			Filter:        filter,
		})
		if err != nil {
			observability.LogAndCapture(s.logger, sentry.LevelError, "Failed to fetch messages during reconciliation", err, map[string]string{
				"module":  "reconciliation",
				"session": sessionName,
				"chat":    chatID,
			})
			return fmt.Errorf("failed to fetch messages: %w", err)
		}

		if len(result.Messages) == 0 {
			break
		}

		// Process each message
		for _, msg := range result.Messages {
			// Skip messages we've already processed (based on ID)
			if msg.ID == checkpoint.LastMessageID {
				continue
			}
			// Defesa em profundidade: mesmo se o filtro server-side falhar,
			// nunca processar mensagem fora da janela de frescor.
			if msg.Timestamp.Before(minFrom) {
				continue
			}

			if err := s.messageProcessor.ProcessMessage(ctx, sessionName, &msg); err != nil {
				s.logger.Warn("Failed to process reconciled message",
					zap.String("session", sessionName),
					zap.String("chat", chatID),
					zap.String("messageID", msg.ID),
					zap.Error(err))
				// Continue processing other messages - processor should be idempotent
			}

			totalProcessed++
			msgTimestamp := msg.Timestamp.UnixMilli()
			if msgTimestamp > lastTimestamp {
				lastTimestamp = msgTimestamp
				lastMessageID = msg.ID
			}
		}

		// Update checkpoint after each batch
		if lastTimestamp > 0 {
			if err := s.checkpointRepo.UpdateCheckpoint(ctx, sessionName, chatID, lastTimestamp, lastMessageID); err != nil {
				observability.LogAndCapture(s.logger, sentry.LevelError, "Failed to update reconciliation checkpoint after batch", err, map[string]string{
					"module":  "reconciliation",
					"session": sessionName,
					"chat":    chatID,
				})
			}
		}

		// Backstop: um chat de altíssimo volume não pode transformar um
		// reconcile em varredura sem fim.
		if totalProcessed >= s.config.MaxMessagesPerReconcile {
			s.logger.Warn("Reconciliation message cap reached — stopping this pass",
				zap.String("session", sessionName),
				zap.String("chat", chatID),
				zap.Int("cap", s.config.MaxMessagesPerReconcile))
			break
		}

		// Check if there are more messages
		if !result.HasMore {
			break
		}

		offset = result.NextOffset
	}

	s.logger.Info("Reconciliation batch completed",
		zap.String("session", sessionName),
		zap.String("chat", chatID),
		zap.Int("processedCount", totalProcessed))

	return nil
}

// =============================================================================
// Watchdog (Silence Detection)
// =============================================================================

// StartWatchdog starts the silence watchdog goroutine.
// The watchdog periodically checks if any session has been silent for too long
// and triggers reconciliation for those sessions.
//
// Returns a cancel function to stop the watchdog.
//
// Example:
//
//	cancelWatchdog := reconciliationService.StartWatchdog(ctx)
//	defer cancelWatchdog()
func (s *Service) StartWatchdog(ctx context.Context) context.CancelFunc {
	watchdogCtx, cancel := context.WithCancel(ctx)

	go func() {
		ticker := time.NewTicker(s.config.CheckInterval)
		defer ticker.Stop()

		s.logger.Info("Watchdog started",
			zap.Duration("checkInterval", s.config.CheckInterval),
			zap.Duration("silenceTimeout", s.config.SilenceTimeout))

		for {
			select {
			case <-watchdogCtx.Done():
				s.logger.Info("Watchdog stopped")
				return
			case <-ticker.C:
				s.checkAndReconcile(watchdogCtx)
			}
		}
	}()

	return cancel
}

// runMaintenance executa a higiene periódica dos checkpoints a partir do
// watchdog: destrava 'running'/'failed' órfãos (crash no meio de um
// ReconcileChat vetava o chat para sempre — o MarkRunning recusa não-idle) e
// remove checkpoints inativos (grupo morto/sessão despareada).
func (s *Service) runMaintenance(ctx context.Context) {
	s.maintenanceMu.Lock()
	doReset := time.Since(s.lastStuckReset) >= 5*time.Minute
	if doReset {
		s.lastStuckReset = time.Now()
	}
	doPurge := time.Since(s.lastCheckpointPurge) >= 24*time.Hour
	if doPurge {
		s.lastCheckpointPurge = time.Now()
	}
	s.maintenanceMu.Unlock()

	if doReset {
		if n, err := s.checkpointRepo.ResetStuckCheckpoints(ctx, 30*time.Minute); err != nil {
			s.logger.Warn("Failed to reset stuck checkpoints", zap.Error(err))
		} else if n > 0 {
			s.logger.Info("Reset stuck reconciliation checkpoints", zap.Int64("count", n))
		}
	}
	if doPurge {
		if n, err := s.checkpointRepo.DeleteInactiveCheckpoints(ctx, 7*24*time.Hour); err != nil {
			s.logger.Warn("Failed to purge inactive checkpoints", zap.Error(err))
		} else if n > 0 {
			s.logger.Info("Purged inactive reconciliation checkpoints", zap.Int64("count", n))
		}
	}
}

// checkAndReconcile checks for silent sessions, verifies health, and triggers reconciliation.
func (s *Service) checkAndReconcile(ctx context.Context) {
	s.runMaintenance(ctx)

	s.lastEventMu.RLock()
	sessions := make(map[string]time.Time, len(s.lastEventTime))
	for session, lastEvent := range s.lastEventTime {
		sessions[session] = lastEvent
	}
	s.lastEventMu.RUnlock()

	now := time.Now()
	for sessionName, lastEvent := range sessions {
		// T3.9: Check context cancellation between session iterations
		select {
		case <-ctx.Done():
			s.logger.Info("checkAndReconcile interrupted by context cancellation")
			return
		default:
		}

		silence := now.Sub(lastEvent)
		if silence < s.config.SilenceTimeout {
			continue
		}

		// Give up on sessions that already exhausted their restart budget.
		// They emit no events (so they stay silent forever), and re-checking
		// them every tick only spams logs and burns a GetSessionStatus call
		// against the (often memory-pressured) WAHA shard. Drop them from
		// tracking; if the session ever recovers, RecordEvent re-registers it
		// automatically. The VPS-side restarters (monitor.py/session-watchdog)
		// remain responsible for permanently-failed sessions.
		if s.config.HealthCheckEnabled && s.config.MaxRestartAttempts > 0 {
			s.restartMu.Lock()
			exhausted := s.restartAttempts[sessionName] >= s.config.MaxRestartAttempts
			s.restartMu.Unlock()
			if exhausted {
				s.logger.Info("Releasing permanently-failed session from watchdog (restart budget exhausted)",
					zap.String("session", sessionName),
					zap.Int("maxAttempts", s.config.MaxRestartAttempts))
				s.lastEventMu.Lock()
				delete(s.lastEventTime, sessionName)
				s.lastEventMu.Unlock()
				s.resetRestartAttempts(sessionName)
				continue
			}
		}

		s.logger.Info("Silence detected for session",
			zap.String("session", sessionName),
			zap.Duration("silenceDuration", silence))

		// Health check and restart if enabled
		if s.config.HealthCheckEnabled {
			healthy, err := s.checkAndRestartSession(ctx, sessionName)
			if err != nil {
				// Circuit breaker open: just log debug and skip — no Sentry event
				var providerErr *whatsapp.ProviderError
				if errors.As(err, &providerErr) && providerErr.Code == whatsapp.ErrCodeCircuitBreakerOpen {
					s.logger.Debug("Skipping session check — circuit breaker open",
						zap.String("session", sessionName))
					continue
				}
				observability.LogAndCapture(s.logger, sentry.LevelWarning, "Failed to check/restart session during reconciliation watchdog", err, map[string]string{
					"module":  "reconciliation",
					"session": sessionName,
				})
			}
			if !healthy {
				// Session is unhealthy and couldn't be restarted, skip reconciliation
				continue
			}
		}

		// Fetch all chats for this session and reconcile each
		sessionCopy := sessionName
		observability.SafeGo("reconciliation.reconcileSession."+sessionCopy, func() {
			s.reconcileSession(ctx, sessionCopy)
		})
	}
}

// checkAndRestartSession checks session health and attempts restart if needed.
// Returns true if session is healthy (or was successfully restarted), false otherwise.
func (s *Service) checkAndRestartSession(ctx context.Context, sessionName string) (bool, error) {
	// Check session status via WAHA
	status, err := s.provider.GetSessionStatus(ctx, sessionName)
	if err != nil {
		var providerErr *whatsapp.ProviderError
		if errors.As(err, &providerErr) {
			switch providerErr.Code {
			case whatsapp.ErrCodeSessionNotFound:
				// Session doesn't exist in WAHA — remove from watchdog tracking
				s.logger.Info("Session no longer exists in WAHA, removing from watchdog",
					zap.String("session", sessionName))
				s.lastEventMu.Lock()
				delete(s.lastEventTime, sessionName)
				s.lastEventMu.Unlock()
				s.resetRestartAttempts(sessionName)
				return false, nil
			case whatsapp.ErrCodeCircuitBreakerOpen:
				// Circuit breaker open — can't check status, don't attempt restart
				return false, err
			}
		}

		s.logger.Warn("Failed to get session status",
			zap.String("session", sessionName),
			zap.Error(err))
		// Can't determine health, assume unhealthy and try restart
		return s.tryRestartSession(ctx, sessionName)
	}

	// Nil check: GetSessionStatus may return (nil, nil) for unexpected responses
	if status == nil {
		s.logger.Warn("GetSessionStatus returned nil for session",
			zap.String("session", sessionName))
		return s.tryRestartSession(ctx, sessionName)
	}

	// Check if session is connected
	if status.Status == whatsapp.SessionStatusConnected {
		// Session is healthy, reset restart attempts
		s.resetRestartAttempts(sessionName)
		s.logger.Debug("Session is healthy",
			zap.String("session", sessionName),
			zap.String("status", string(status.Status)))
		return true, nil
	}

	// Session is not connected, log the status
	s.logger.Warn("Session is not connected",
		zap.String("session", sessionName),
		zap.String("status", string(status.Status)))

	// Try to restart the session
	return s.tryRestartSession(ctx, sessionName)
}

// tryRestartSession attempts to restart a session with exponential backoff.
// Returns true if restart was successful or we should proceed anyway, false to skip.
func (s *Service) tryRestartSession(ctx context.Context, sessionName string) (bool, error) {
	// Check if restart is disabled
	if s.config.MaxRestartAttempts == 0 {
		s.logger.Debug("Session restart disabled",
			zap.String("session", sessionName))
		return false, nil
	}

	s.restartMu.Lock()
	attempts := s.restartAttempts[sessionName]
	lastRestart := s.lastRestartTime[sessionName]
	s.restartMu.Unlock()

	// Check if max attempts exceeded
	if attempts >= s.config.MaxRestartAttempts {
		s.logger.Warn("Max restart attempts exceeded for session",
			zap.String("session", sessionName),
			zap.Int("attempts", attempts),
			zap.Int("maxAttempts", s.config.MaxRestartAttempts))
		return false, nil
	}

	// Calculate backoff: base * 2^attempts (exponential with jitter)
	backoffDuration := s.config.RestartBackoff * time.Duration(1<<uint(attempts))
	if backoffDuration > 30*time.Minute {
		backoffDuration = 30 * time.Minute // Cap at 30 minutes
	}

	// Check if enough time has passed since last restart attempt
	if time.Since(lastRestart) < backoffDuration {
		s.logger.Debug("Waiting for backoff before next restart attempt",
			zap.String("session", sessionName),
			zap.Duration("remaining", backoffDuration-time.Since(lastRestart)))
		return false, nil
	}

	// Attempt restart
	s.logger.Info("Attempting to restart session",
		zap.String("session", sessionName),
		zap.Int("attempt", attempts+1),
		zap.Int("maxAttempts", s.config.MaxRestartAttempts))

	// Update tracking before restart attempt
	s.restartMu.Lock()
	s.restartAttempts[sessionName] = attempts + 1
	s.lastRestartTime[sessionName] = time.Now()
	s.restartMu.Unlock()

	err := s.provider.RestartSession(ctx, sessionName)
	if err != nil {
		observability.LogAndCapture(s.logger, sentry.LevelError, "Failed to restart WAHA session during reconciliation", err, map[string]string{
			"module":  "reconciliation",
			"session": sessionName,
		})
		return false, err
	}

	// Wait a bit for the session to reconnect
	select {
	case <-time.After(5 * time.Second):
	case <-ctx.Done():
		return false, ctx.Err()
	}

	// Check if restart was successful
	status, err := s.provider.GetSessionStatus(ctx, sessionName)
	if err != nil {
		s.logger.Warn("Failed to verify session status after restart",
			zap.String("session", sessionName),
			zap.Error(err))
		return false, err
	}
	if status == nil {
		s.logger.Warn("Session status nil after restart",
			zap.String("session", sessionName))
		return false, nil
	}

	if status.Status == whatsapp.SessionStatusConnected {
		s.logger.Info("Session restarted successfully",
			zap.String("session", sessionName))
		s.resetRestartAttempts(sessionName)
		return true, nil
	}

	s.logger.Warn("Session still not connected after restart",
		zap.String("session", sessionName),
		zap.String("status", string(status.Status)))
	return false, nil
}

// resetRestartAttempts resets the restart attempt counter for a session.
func (s *Service) resetRestartAttempts(sessionName string) {
	s.restartMu.Lock()
	delete(s.restartAttempts, sessionName)
	delete(s.lastRestartTime, sessionName)
	s.restartMu.Unlock()
}

// reconcileSession reconciles all chats for a session.
func (s *Service) reconcileSession(ctx context.Context, sessionName string) {
	checkpoints, err := s.checkpointRepo.GetBySession(ctx, sessionName)
	if err != nil {
		observability.LogAndCapture(s.logger, sentry.LevelError, "Failed to get checkpoints for reconciliation session", err, map[string]string{
			"module":  "reconciliation",
			"session": sessionName,
		})
		return
	}

	for _, cp := range checkpoints {
		// T3.9: Check context cancellation between checkpoint iterations
		select {
		case <-ctx.Done():
			s.logger.Info("reconcileSession interrupted by context cancellation",
				zap.String("session", sessionName))
			return
		default:
		}

		if err := s.ReconcileChat(ctx, cp.SessionName, cp.ChatID); err != nil {
			if !errors.Is(err, ErrAlreadyRunning) {
				observability.LogAndCapture(s.logger, sentry.LevelError, "Failed to reconcile chat in session", err, map[string]string{
					"module":  "reconciliation",
					"session": cp.SessionName,
					"chat":    cp.ChatID,
				})
			}
		}
	}

	// Record event after successful reconciliation to reset silence timer
	s.RecordEvent(sessionName)
}

// =============================================================================
// Startup Recovery
// =============================================================================

// OnStartup runs reconciliation for all active sessions on server start.
// This ensures no messages are missed during server downtime.
//
// It fetches all session/chat pairs with checkpoints and reconciles each one.
// Errors are logged but do not stop the startup process.
func (s *Service) OnStartup(ctx context.Context) error {
	s.logger.Info("Running startup reconciliation")

	sessionChats, err := s.checkpointRepo.GetActiveSessionsChats(ctx)
	if err != nil {
		observability.LogAndCapture(s.logger, sentry.LevelError, "Failed to get active sessions for startup reconciliation", err, map[string]string{
			"module": "reconciliation",
		})
		return fmt.Errorf("failed to get active session/chats: %w", err)
	}

	if len(sessionChats) == 0 {
		s.logger.Info("No active sessions to reconcile")
		return nil
	}

	s.logger.Info("Found sessions to reconcile",
		zap.Int("count", len(sessionChats)))

	// Use a WaitGroup to track all reconciliations
	var wg sync.WaitGroup
	errChan := make(chan error, len(sessionChats))

	for _, sc := range sessionChats {
		wg.Add(1)
		sessionName := sc.SessionName
		chatID := sc.ChatID
		observability.SafeGo("reconciliation.startup."+sessionName+":"+chatID, func() {
			defer wg.Done()

			if err := s.ReconcileChat(ctx, sessionName, chatID); err != nil {
				if !errors.Is(err, ErrAlreadyRunning) {
					errChan <- fmt.Errorf("failed to reconcile %s:%s: %w", sessionName, chatID, err)
				}
			}
		})
	}

	// Wait for all reconciliations to complete
	wg.Wait()
	close(errChan)

	// Log any errors but don't fail startup
	var errCount int
	for err := range errChan {
		observability.LogAndCapture(s.logger, sentry.LevelError, "Startup reconciliation error", err, map[string]string{
			"module": "reconciliation",
		})
		errCount++
	}

	if errCount > 0 {
		s.logger.Warn("Startup reconciliation completed with errors",
			zap.Int("totalChats", len(sessionChats)),
			zap.Int("errorCount", errCount))
	} else {
		s.logger.Info("Startup reconciliation completed successfully",
			zap.Int("totalChats", len(sessionChats)))
	}

	return nil
}

// =============================================================================
// Manual Reconciliation
// =============================================================================

// manualReconcileTimeout bounds the detached ReconcileSession goroutine. It
// matches the whatsapp:reconcile:session asynq task budget (10min) so the
// fire-and-forget work has the same ceiling the queue intended.
const manualReconcileTimeout = 10 * time.Minute

// ReconcileSession manually triggers reconciliation for all chats in a session.
// This can be called when a session reconnects or on user request.
func (s *Service) ReconcileSession(ctx context.Context, sessionName string) error {
	s.logger.Info("Triggering manual session reconciliation",
		zap.String("session", sessionName))

	// Detach from the caller's context BEFORE the fire-and-forget goroutine.
	// The asynq worker (reconciliation_worker.go ProcessSession) returns nil
	// immediately after this call, which cancels the task context; running
	// reconcileSession on that raw ctx made every checkpoint/GetMessages call
	// fail with "context canceled" ("Failed to get checkpoints" storm on each
	// worker recreate) AND turned the backfill into a silent no-op. A detached,
	// bounded context lets the reconciliation actually run to completion.
	bgCtx := context.WithoutCancel(ctx)
	observability.SafeGo("reconciliation.manualReconcileSession."+sessionName, func() {
		runCtx, cancel := context.WithTimeout(bgCtx, manualReconcileTimeout)
		defer cancel()
		s.reconcileSession(runCtx, sessionName)
	})
	return nil
}

// GetCheckpoint retrieves the current checkpoint for a session/chat.
// Returns nil if no checkpoint exists.
func (s *Service) GetCheckpoint(ctx context.Context, sessionName, chatID string) (*Checkpoint, error) {
	return s.checkpointRepo.GetOrCreate(ctx, sessionName, chatID)
}

// IsReconciling checks if reconciliation is currently running for a chat.
func (s *Service) IsReconciling(sessionName, chatID string) bool {
	key := sessionName + ":" + chatID
	s.runningMu.Lock()
	defer s.runningMu.Unlock()
	_, running := s.running[key]
	return running
}
