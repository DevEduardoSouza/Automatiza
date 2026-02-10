package workers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"automatiza-go/internal/services/reconciliation"

	"github.com/hibiken/asynq"
	"go.uber.org/zap"
)

// ReconciliationWorker handles reconciliation tasks from the queue.
// It processes both single chat reconciliation and full session reconciliation
// using the ReconciliationService for the actual backfill logic.
type ReconciliationWorker struct {
	reconcileSvc *reconciliation.Service
	logger       *zap.Logger
}

// NewReconciliationWorker creates a new reconciliation worker.
//
// Parameters:
//   - reconcileSvc: The reconciliation service that performs the actual backfill
//   - logger: Zap logger for structured logging
func NewReconciliationWorker(
	reconcileSvc *reconciliation.Service,
	logger *zap.Logger,
) *ReconciliationWorker {
	return &ReconciliationWorker{
		reconcileSvc: reconcileSvc,
		logger:       logger.Named("reconciliation-worker"),
	}
}

// ProcessChat handles single chat reconciliation tasks.
// It unmarshals the payload and delegates to the reconciliation service.
func (w *ReconciliationWorker) ProcessChat(ctx context.Context, task *asynq.Task) error {
	var payload ReconcileChatPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		w.logger.Error("Failed to unmarshal reconcile chat payload",
			zap.Error(err),
			zap.ByteString("payload", task.Payload()))
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	if payload.SessionName == "" {
		w.logger.Error("Missing session_name in reconcile chat payload")
		return fmt.Errorf("missing session_name in payload")
	}

	if payload.ChatID == "" {
		w.logger.Error("Missing chat_id in reconcile chat payload",
			zap.String("session", payload.SessionName))
		return fmt.Errorf("missing chat_id in payload")
	}

	w.logger.Info("Processing chat reconciliation task",
		zap.String("session", payload.SessionName),
		zap.String("chat", payload.ChatID))

	err := w.reconcileSvc.ReconcileChat(ctx, payload.SessionName, payload.ChatID)
	if err != nil {
		// Check for specific error types that should not be retried
		if errors.Is(err, reconciliation.ErrAlreadyRunning) {
			w.logger.Debug("Chat reconciliation already running, skipping",
				zap.String("session", payload.SessionName),
				zap.String("chat", payload.ChatID))
			return nil // Don't retry - another worker is handling it
		}

		w.logger.Error("Chat reconciliation failed",
			zap.String("session", payload.SessionName),
			zap.String("chat", payload.ChatID),
			zap.Error(err))
		return err // Will be retried by asynq
	}

	w.logger.Info("Chat reconciliation completed successfully",
		zap.String("session", payload.SessionName),
		zap.String("chat", payload.ChatID))

	return nil
}

// ProcessSession handles full session reconciliation tasks.
// It reconciles all chats associated with the given session.
func (w *ReconciliationWorker) ProcessSession(ctx context.Context, task *asynq.Task) error {
	var payload ReconcileSessionPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		w.logger.Error("Failed to unmarshal reconcile session payload",
			zap.Error(err),
			zap.ByteString("payload", task.Payload()))
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	if payload.SessionName == "" {
		w.logger.Error("Missing session_name in reconcile session payload")
		return fmt.Errorf("missing session_name in payload")
	}

	w.logger.Info("Processing session reconciliation task",
		zap.String("session", payload.SessionName))

	// ReconcileSession triggers reconciliation for all chats in the session
	// It runs asynchronously internally, so we just trigger it
	err := w.reconcileSvc.ReconcileSession(ctx, payload.SessionName)
	if err != nil {
		w.logger.Error("Session reconciliation failed",
			zap.String("session", payload.SessionName),
			zap.Error(err))
		return err
	}

	w.logger.Info("Session reconciliation triggered successfully",
		zap.String("session", payload.SessionName))

	return nil
}

// ProcessChatWrapper returns an asynq.HandlerFunc wrapper for ProcessChat.
// Use this when registering the handler with asynq.ServeMux.
func (w *ReconciliationWorker) ProcessChatWrapper() func(ctx context.Context, task *asynq.Task) error {
	return func(ctx context.Context, task *asynq.Task) error {
		return w.ProcessChat(ctx, task)
	}
}

// ProcessSessionWrapper returns an asynq.HandlerFunc wrapper for ProcessSession.
// Use this when registering the handler with asynq.ServeMux.
func (w *ReconciliationWorker) ProcessSessionWrapper() func(ctx context.Context, task *asynq.Task) error {
	return func(ctx context.Context, task *asynq.Task) error {
		return w.ProcessSession(ctx, task)
	}
}
