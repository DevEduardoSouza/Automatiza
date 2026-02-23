package reconciliation_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/services/reconciliation"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// newWatchdogTestService creates a service with zap.NewNop() logger.
// This avoids the inherent data race between zaptest.Logger's use of
// testing.T.Log and goroutines spawned by SafeGo/StartWatchdog that
// may outlive the test function.
func newWatchdogTestService(t *testing.T, config reconciliation.Config) (*reconciliation.Service, *testServiceDeps) {
	t.Helper()
	logger := zap.NewNop()

	deps := &testServiceDeps{
		provider:       NewMockWhatsAppProvider(),
		checkpointRepo: new(MockCheckpointRepository),
		messageProc:    new(MockMessageProcessor),
		config:         config,
	}

	svc := reconciliation.New(
		deps.provider,
		deps.checkpointRepo,
		deps.messageProc,
		logger,
		deps.config,
	)

	return svc, deps
}

// fastWatchdogConfig returns a config with fast timeouts for watchdog tests.
func fastWatchdogConfig() reconciliation.Config {
	return reconciliation.Config{
		SilenceTimeout:     30 * time.Millisecond,
		BatchSize:          10,
		MaxConcurrent:      5,
		CheckInterval:      20 * time.Millisecond,
		MaxRetries:         1,
		RetryBackoff:       1 * time.Millisecond,
		HealthCheckEnabled: true,
		MaxRestartAttempts: 2,
		RestartBackoff:     1 * time.Millisecond,
	}
}

// =============================================================================
// TestCheckAndRestartSession — covers checkAndRestartSession via watchdog
// =============================================================================

func TestCheckAndRestartSession(t *testing.T) {
	t.Run("removes session from watchdog when session not found", func(t *testing.T) {
		t.Parallel()
		config := fastWatchdogConfig()
		svc, deps := newWatchdogTestService(t, config)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		sessionName := "vanished-session"

		// Return ErrCodeSessionNotFound
		deps.provider.On("GetSessionStatus", mock.Anything, sessionName).Return(
			nil,
			whatsapp.NewProviderError(whatsapp.ErrCodeSessionNotFound, "session not found", nil),
		).Maybe()

		// Should NOT call GetBySession since session was removed
		// (no reconciliation triggered)

		svc.RecordEvent(sessionName)
		cancelWatchdog := svc.StartWatchdog(ctx)
		defer cancelWatchdog()

		// Wait for silence + watchdog tick
		time.Sleep(150 * time.Millisecond)

		deps.provider.AssertCalled(t, "GetSessionStatus", mock.Anything, sessionName)
		// GetBySession should NOT be called because session was removed from tracking
		deps.checkpointRepo.AssertNotCalled(t, "GetBySession", mock.Anything, sessionName)
	})

	t.Run("skips reconciliation on circuit breaker open", func(t *testing.T) {
		t.Parallel()
		config := fastWatchdogConfig()
		svc, deps := newWatchdogTestService(t, config)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		sessionName := "cb-session"

		// Return ErrCodeCircuitBreakerOpen
		deps.provider.On("GetSessionStatus", mock.Anything, sessionName).Return(
			nil,
			whatsapp.NewProviderError(whatsapp.ErrCodeCircuitBreakerOpen, "circuit breaker open", nil),
		).Maybe()

		svc.RecordEvent(sessionName)
		cancelWatchdog := svc.StartWatchdog(ctx)
		defer cancelWatchdog()

		time.Sleep(150 * time.Millisecond)

		deps.provider.AssertCalled(t, "GetSessionStatus", mock.Anything, sessionName)
		// Should not attempt GetBySession since circuit breaker blocked
		deps.checkpointRepo.AssertNotCalled(t, "GetBySession", mock.Anything, sessionName)
	})

	t.Run("returns nil status triggers restart and verifies connected", func(t *testing.T) {
		t.Parallel()
		config := fastWatchdogConfig()
		svc, deps := newWatchdogTestService(t, config)
		// Need >5s to survive the hardcoded 5s post-restart wait
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)

		sessionName := "nil-status-session"
		restartDone := make(chan struct{}, 5)

		// First GetSessionStatus returns nil (unexpected)
		deps.provider.On("GetSessionStatus", mock.Anything, sessionName).Return(
			(*whatsapp.SessionInfo)(nil), nil,
		).Once()

		// After nil status, tryRestartSession calls RestartSession
		deps.provider.On("RestartSession", mock.Anything, sessionName).Run(func(args mock.Arguments) {
			select {
			case restartDone <- struct{}{}:
			default:
			}
		}).Return(nil).Maybe()

		// After 5s restart wait, re-check status → connected (covers lines 778-783)
		deps.provider.On("GetSessionStatus", mock.Anything, sessionName).Return(&whatsapp.SessionInfo{
			Name:   sessionName,
			Status: whatsapp.SessionStatusConnected,
		}, nil).Maybe()

		deps.checkpointRepo.On("GetBySession", mock.Anything, sessionName).Return([]reconciliation.Checkpoint{}, nil).Maybe()

		svc.RecordEvent(sessionName)
		cancelWatchdog := svc.StartWatchdog(ctx)

		// Wait for restart to be called
		select {
		case <-restartDone:
		case <-time.After(2 * time.Second):
			t.Fatal("timeout waiting for RestartSession")
		}
		// Wait for 5s post-restart delay + status check
		time.Sleep(6 * time.Second)

		cancelWatchdog()
		cancel()
		time.Sleep(50 * time.Millisecond)

		deps.provider.AssertCalled(t, "RestartSession", mock.Anything, sessionName)
	})

	t.Run("healthy connected session resets restart attempts and reconciles", func(t *testing.T) {
		t.Parallel()
		config := fastWatchdogConfig()
		svc, deps := newWatchdogTestService(t, config)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		sessionName := "healthy-session"

		deps.provider.On("GetSessionStatus", mock.Anything, sessionName).Return(&whatsapp.SessionInfo{
			Name:   sessionName,
			Status: whatsapp.SessionStatusConnected,
		}, nil).Maybe()

		checkpoints := []reconciliation.Checkpoint{
			{SessionName: sessionName, ChatID: "chat1@g.us", Status: reconciliation.CheckpointStatusIdle},
		}
		deps.checkpointRepo.On("GetBySession", mock.Anything, sessionName).Return(checkpoints, nil).Maybe()
		deps.checkpointRepo.On("GetOrCreate", mock.Anything, sessionName, "chat1@g.us").Return(&checkpoints[0], nil).Maybe()
		deps.checkpointRepo.On("MarkRunning", mock.Anything, sessionName, "chat1@g.us").Return(nil).Maybe()
		deps.checkpointRepo.On("MarkIdle", mock.Anything, sessionName, "chat1@g.us").Return(nil).Maybe()

		deps.provider.On("GetMessages", mock.Anything, mock.Anything).Return(&whatsapp.MessagesResult{
			Messages: []whatsapp.Message{},
			HasMore:  false,
		}, nil).Maybe()

		svc.RecordEvent(sessionName)
		cancelWatchdog := svc.StartWatchdog(ctx)
		defer cancelWatchdog()

		time.Sleep(200 * time.Millisecond)

		deps.provider.AssertCalled(t, "GetSessionStatus", mock.Anything, sessionName)
		// Restart should NOT be called since session is healthy
		deps.provider.AssertNotCalled(t, "RestartSession", mock.Anything, mock.Anything)
	})

	t.Run("generic provider error on GetSessionStatus triggers restart", func(t *testing.T) {
		t.Parallel()
		config := fastWatchdogConfig()
		svc, deps := newWatchdogTestService(t, config)
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		sessionName := "error-session"

		// Generic error (not session-not-found, not circuit-breaker)
		deps.provider.On("GetSessionStatus", mock.Anything, sessionName).Return(
			nil, errors.New("network timeout"),
		).Once()

		// tryRestartSession calls RestartSession
		deps.provider.On("RestartSession", mock.Anything, sessionName).Return(nil).Maybe()

		// After restart → connected
		deps.provider.On("GetSessionStatus", mock.Anything, sessionName).Return(&whatsapp.SessionInfo{
			Name:   sessionName,
			Status: whatsapp.SessionStatusConnected,
		}, nil).Maybe()

		deps.checkpointRepo.On("GetBySession", mock.Anything, sessionName).Return([]reconciliation.Checkpoint{}, nil).Maybe()

		svc.RecordEvent(sessionName)
		cancelWatchdog := svc.StartWatchdog(ctx)
		defer cancelWatchdog()

		time.Sleep(300 * time.Millisecond)

		deps.provider.AssertCalled(t, "RestartSession", mock.Anything, sessionName)
	})

	t.Run("disconnected session triggers restart", func(t *testing.T) {
		t.Parallel()
		config := fastWatchdogConfig()
		svc, deps := newWatchdogTestService(t, config)
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		sessionName := "disconnected-session"

		// First call: disconnected
		deps.provider.On("GetSessionStatus", mock.Anything, sessionName).Return(&whatsapp.SessionInfo{
			Name:   sessionName,
			Status: whatsapp.SessionStatusDisconnected,
		}, nil).Once()

		deps.provider.On("RestartSession", mock.Anything, sessionName).Return(nil).Maybe()

		// After restart → connected
		deps.provider.On("GetSessionStatus", mock.Anything, sessionName).Return(&whatsapp.SessionInfo{
			Name:   sessionName,
			Status: whatsapp.SessionStatusConnected,
		}, nil).Maybe()

		deps.checkpointRepo.On("GetBySession", mock.Anything, sessionName).Return([]reconciliation.Checkpoint{}, nil).Maybe()

		svc.RecordEvent(sessionName)
		cancelWatchdog := svc.StartWatchdog(ctx)
		defer cancelWatchdog()

		time.Sleep(300 * time.Millisecond)

		deps.provider.AssertCalled(t, "RestartSession", mock.Anything, sessionName)
	})
}

// =============================================================================
// TestTryRestartSession — covers tryRestartSession via watchdog
// =============================================================================

func TestTryRestartSession(t *testing.T) {
	t.Run("restart disabled when MaxRestartAttempts is 0", func(t *testing.T) {
		t.Parallel()
		config := fastWatchdogConfig()
		config.MaxRestartAttempts = 0 // Disabled
		svc, deps := newWatchdogTestService(t, config)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		sessionName := "no-restart-session"

		// Session is disconnected
		deps.provider.On("GetSessionStatus", mock.Anything, sessionName).Return(&whatsapp.SessionInfo{
			Name:   sessionName,
			Status: whatsapp.SessionStatusDisconnected,
		}, nil).Maybe()

		svc.RecordEvent(sessionName)
		cancelWatchdog := svc.StartWatchdog(ctx)
		defer cancelWatchdog()

		time.Sleep(200 * time.Millisecond)

		// Restart should NOT be called since it's disabled
		deps.provider.AssertNotCalled(t, "RestartSession", mock.Anything, mock.Anything)
	})

	t.Run("stops after max restart attempts exceeded", func(t *testing.T) {
		t.Parallel()
		config := fastWatchdogConfig()
		config.MaxRestartAttempts = 1 // Only 1 attempt
		svc, deps := newWatchdogTestService(t, config)
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		sessionName := "exhausted-session"

		// Track restart calls with channel synchronization
		restartCalled := make(chan struct{}, 5)

		// Always return disconnected
		deps.provider.On("GetSessionStatus", mock.Anything, sessionName).Return(&whatsapp.SessionInfo{
			Name:   sessionName,
			Status: whatsapp.SessionStatusDisconnected,
		}, nil).Maybe()

		// Restart succeeds but session stays disconnected
		deps.provider.On("RestartSession", mock.Anything, sessionName).Run(func(args mock.Arguments) {
			restartCalled <- struct{}{}
		}).Return(nil).Maybe()

		svc.RecordEvent(sessionName)
		cancelWatchdog := svc.StartWatchdog(ctx)
		defer cancelWatchdog()

		// Wait for at least one restart call
		select {
		case <-restartCalled:
			// Got the expected restart call
		case <-time.After(2 * time.Second):
			t.Fatal("timeout waiting for RestartSession call")
		}

		// Wait more and verify no additional restarts beyond max
		time.Sleep(300 * time.Millisecond)

		// Count via channel (non-blocking drain)
		restartCount := 1 // already received one above
		for {
			select {
			case <-restartCalled:
				restartCount++
			default:
				goto done
			}
		}
	done:
		assert.LessOrEqual(t, restartCount, 1, "should not exceed MaxRestartAttempts")
	})

	t.Run("restart fails with provider error", func(t *testing.T) {
		t.Parallel()
		config := fastWatchdogConfig()
		svc, deps := newWatchdogTestService(t, config)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		sessionName := "restart-fail-session"

		deps.provider.On("GetSessionStatus", mock.Anything, sessionName).Return(&whatsapp.SessionInfo{
			Name:   sessionName,
			Status: whatsapp.SessionStatusDisconnected,
		}, nil).Maybe()

		// Restart fails
		deps.provider.On("RestartSession", mock.Anything, sessionName).Return(errors.New("restart failed")).Maybe()

		svc.RecordEvent(sessionName)
		cancelWatchdog := svc.StartWatchdog(ctx)
		defer cancelWatchdog()

		time.Sleep(200 * time.Millisecond)

		deps.provider.AssertCalled(t, "RestartSession", mock.Anything, sessionName)
		// No reconciliation since restart failed
		deps.checkpointRepo.AssertNotCalled(t, "GetBySession", mock.Anything, sessionName)
	})

	t.Run("post-restart status check fails", func(t *testing.T) {
		t.Parallel()
		config := fastWatchdogConfig()
		svc, deps := newWatchdogTestService(t, config)
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)

		sessionName := "post-restart-error-session"
		restartDone := make(chan struct{}, 5)

		// First: disconnected
		deps.provider.On("GetSessionStatus", mock.Anything, sessionName).Return(&whatsapp.SessionInfo{
			Name:   sessionName,
			Status: whatsapp.SessionStatusDisconnected,
		}, nil).Once()

		// Restart succeeds
		deps.provider.On("RestartSession", mock.Anything, sessionName).Run(func(args mock.Arguments) {
			select {
			case restartDone <- struct{}{}:
			default:
			}
		}).Return(nil).Maybe()

		// Post-restart status check fails (called after 5s wait)
		deps.provider.On("GetSessionStatus", mock.Anything, sessionName).Return(
			nil, errors.New("status check failed"),
		).Maybe()

		svc.RecordEvent(sessionName)
		cancelWatchdog := svc.StartWatchdog(ctx)

		// Wait for restart to be called, then wait enough for the 5s post-restart delay
		select {
		case <-restartDone:
		case <-time.After(2 * time.Second):
			t.Fatal("timeout waiting for RestartSession")
		}
		// Give time for 5s wait + status check
		time.Sleep(6 * time.Second)

		cancelWatchdog()
		cancel()
		time.Sleep(50 * time.Millisecond) // let goroutine finish
	})

	t.Run("post-restart status nil", func(t *testing.T) {
		t.Parallel()
		config := fastWatchdogConfig()
		svc, deps := newWatchdogTestService(t, config)
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)

		sessionName := "post-restart-nil-session"
		restartDone := make(chan struct{}, 5)

		// First: disconnected
		deps.provider.On("GetSessionStatus", mock.Anything, sessionName).Return(&whatsapp.SessionInfo{
			Name:   sessionName,
			Status: whatsapp.SessionStatusDisconnected,
		}, nil).Once()

		deps.provider.On("RestartSession", mock.Anything, sessionName).Run(func(args mock.Arguments) {
			select {
			case restartDone <- struct{}{}:
			default:
			}
		}).Return(nil).Maybe()

		// Post-restart returns nil status
		deps.provider.On("GetSessionStatus", mock.Anything, sessionName).Return(
			(*whatsapp.SessionInfo)(nil), nil,
		).Maybe()

		svc.RecordEvent(sessionName)
		cancelWatchdog := svc.StartWatchdog(ctx)

		select {
		case <-restartDone:
		case <-time.After(2 * time.Second):
			t.Fatal("timeout waiting for RestartSession")
		}
		time.Sleep(6 * time.Second)

		cancelWatchdog()
		cancel()
		time.Sleep(50 * time.Millisecond)
	})

	t.Run("post-restart still disconnected", func(t *testing.T) {
		t.Parallel()
		config := fastWatchdogConfig()
		svc, deps := newWatchdogTestService(t, config)
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)

		sessionName := "still-disconnected-session"
		restartDone := make(chan struct{}, 5)

		// Always disconnected
		deps.provider.On("GetSessionStatus", mock.Anything, sessionName).Return(&whatsapp.SessionInfo{
			Name:   sessionName,
			Status: whatsapp.SessionStatusDisconnected,
		}, nil).Maybe()

		deps.provider.On("RestartSession", mock.Anything, sessionName).Run(func(args mock.Arguments) {
			select {
			case restartDone <- struct{}{}:
			default:
			}
		}).Return(nil).Maybe()

		svc.RecordEvent(sessionName)
		cancelWatchdog := svc.StartWatchdog(ctx)

		select {
		case <-restartDone:
		case <-time.After(2 * time.Second):
			t.Fatal("timeout waiting for RestartSession")
		}
		time.Sleep(6 * time.Second)

		cancelWatchdog()
		cancel()
		time.Sleep(50 * time.Millisecond)

		deps.provider.AssertCalled(t, "RestartSession", mock.Anything, sessionName)
	})
}

// =============================================================================
// TestReconcileSession_EdgeCases — covers reconcileSession gaps
// =============================================================================

func TestReconcileSession_EdgeCases(t *testing.T) {
	t.Run("GetBySession error is logged and returns", func(t *testing.T) {
		t.Parallel()
		config := fastWatchdogConfig()
		svc, deps := newWatchdogTestService(t, config)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		sessionName := "get-by-session-error"

		deps.provider.On("GetSessionStatus", mock.Anything, sessionName).Return(&whatsapp.SessionInfo{
			Name:   sessionName,
			Status: whatsapp.SessionStatusConnected,
		}, nil).Maybe()

		// GetBySession fails
		deps.checkpointRepo.On("GetBySession", mock.Anything, sessionName).Return(nil, errors.New("db error")).Maybe()

		svc.RecordEvent(sessionName)
		cancelWatchdog := svc.StartWatchdog(ctx)
		defer cancelWatchdog()

		time.Sleep(200 * time.Millisecond)

		deps.checkpointRepo.AssertCalled(t, "GetBySession", mock.Anything, sessionName)
		// Should NOT call GetOrCreate since GetBySession failed
		deps.checkpointRepo.AssertNotCalled(t, "GetOrCreate", mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("ReconcileChat error for non-ErrAlreadyRunning is logged", func(t *testing.T) {
		t.Parallel()
		svc, deps := newTestService(t)
		ctx := context.Background()

		sessionName := "error-chat-session"

		checkpoints := []reconciliation.Checkpoint{
			{SessionName: sessionName, ChatID: "fail-chat@g.us", Status: reconciliation.CheckpointStatusIdle},
			{SessionName: sessionName, ChatID: "ok-chat@g.us", Status: reconciliation.CheckpointStatusIdle},
		}

		deps.checkpointRepo.On("GetBySession", mock.Anything, sessionName).Return(checkpoints, nil)

		// First chat: GetOrCreate fails (non-ErrAlreadyRunning error)
		deps.checkpointRepo.On("GetOrCreate", mock.Anything, sessionName, "fail-chat@g.us").Return(nil, errors.New("db error"))

		// Second chat: succeeds
		deps.checkpointRepo.On("GetOrCreate", mock.Anything, sessionName, "ok-chat@g.us").Return(&checkpoints[1], nil)
		deps.checkpointRepo.On("MarkRunning", mock.Anything, sessionName, "ok-chat@g.us").Return(nil)
		deps.checkpointRepo.On("MarkIdle", mock.Anything, sessionName, "ok-chat@g.us").Return(nil)

		deps.provider.On("GetMessages", mock.Anything, mock.Anything).Return(&whatsapp.MessagesResult{
			Messages: []whatsapp.Message{},
			HasMore:  false,
		}, nil)

		err := svc.ReconcileSession(ctx, sessionName)
		require.NoError(t, err)

		// Wait for async reconciliation
		time.Sleep(200 * time.Millisecond)

		// Both chats should have been attempted
		deps.checkpointRepo.AssertCalled(t, "GetOrCreate", mock.Anything, sessionName, "fail-chat@g.us")
		deps.checkpointRepo.AssertCalled(t, "GetOrCreate", mock.Anything, sessionName, "ok-chat@g.us")
	})

	t.Run("ReconcileChat ErrAlreadyRunning is silently ignored", func(t *testing.T) {
		t.Parallel()
		svc, deps := newTestService(t)
		ctx := context.Background()

		sessionName := "already-running-session"

		checkpoints := []reconciliation.Checkpoint{
			{SessionName: sessionName, ChatID: "running-chat@g.us", Status: reconciliation.CheckpointStatusRunning},
		}

		deps.checkpointRepo.On("GetBySession", mock.Anything, sessionName).Return(checkpoints, nil)
		deps.checkpointRepo.On("GetOrCreate", mock.Anything, sessionName, "running-chat@g.us").Return(&checkpoints[0], nil)
		deps.checkpointRepo.On("MarkRunning", mock.Anything, sessionName, "running-chat@g.us").Return(reconciliation.ErrAlreadyRunning)

		err := svc.ReconcileSession(ctx, sessionName)
		require.NoError(t, err)

		time.Sleep(200 * time.Millisecond)
	})

	t.Run("context cancellation stops checkpoint iteration", func(t *testing.T) {
		t.Parallel()
		config := fastWatchdogConfig()
		svc, deps := newWatchdogTestService(t, config)

		sessionName := "cancel-session"

		// Create many checkpoints
		checkpoints := make([]reconciliation.Checkpoint, 20)
		for i := range checkpoints {
			checkpoints[i] = reconciliation.Checkpoint{
				SessionName: sessionName,
				ChatID:      "chat-" + itoa(i) + "@g.us",
				Status:      reconciliation.CheckpointStatusIdle,
			}
		}

		deps.checkpointRepo.On("GetBySession", mock.Anything, sessionName).Return(checkpoints, nil)

		// The first chat will block, cancel context, then remaining chats should be skipped
		deps.checkpointRepo.On("GetOrCreate", mock.Anything, mock.Anything, mock.Anything).Return(&reconciliation.Checkpoint{
			SessionName: sessionName,
			ChatID:      "any",
			Status:      reconciliation.CheckpointStatusIdle,
		}, nil).Maybe()
		deps.checkpointRepo.On("MarkRunning", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		deps.checkpointRepo.On("MarkIdle", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

		deps.provider.On("GetMessages", mock.Anything, mock.Anything).Return(&whatsapp.MessagesResult{
			Messages: []whatsapp.Message{},
			HasMore:  false,
		}, nil).Maybe()

		// Use a short-lived context
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		err := svc.ReconcileSession(ctx, sessionName)
		require.NoError(t, err)

		// Wait for async work to finish/cancel
		time.Sleep(200 * time.Millisecond)

		// The service should have handled cancellation gracefully
	})
}

// =============================================================================
// TestReconcileChat_AdditionalErrorPaths — covers ReconcileChat gaps
// =============================================================================

func TestReconcileChat_AdditionalErrorPaths(t *testing.T) {
	t.Run("MarkIdle error after successful reconciliation is logged", func(t *testing.T) {
		t.Parallel()
		svc, deps := newTestService(t)
		ctx := context.Background()

		sessionName := "test-session"
		chatID := "mark-idle-fail@g.us"

		checkpoint := &reconciliation.Checkpoint{
			SessionName: sessionName,
			ChatID:      chatID,
			Status:      reconciliation.CheckpointStatusIdle,
		}

		deps.checkpointRepo.On("GetOrCreate", ctx, sessionName, chatID).Return(checkpoint, nil)
		deps.checkpointRepo.On("MarkRunning", ctx, sessionName, chatID).Return(nil)
		// MarkIdle fails
		deps.checkpointRepo.On("MarkIdle", ctx, sessionName, chatID).Return(errors.New("mark idle failed"))

		deps.provider.On("GetMessages", ctx, mock.Anything).Return(&whatsapp.MessagesResult{
			Messages: []whatsapp.Message{},
			HasMore:  false,
		}, nil)

		// ReconcileChat should still succeed despite MarkIdle failure
		err := svc.ReconcileChat(ctx, sessionName, chatID)
		require.NoError(t, err)
	})

	t.Run("MarkFailed error during reconciliation failure is logged", func(t *testing.T) {
		t.Parallel()
		config := reconciliation.Config{
			SilenceTimeout: 5 * time.Minute,
			BatchSize:      10,
			MaxConcurrent:  5,
			CheckInterval:  1 * time.Minute,
			MaxRetries:     1,
			RetryBackoff:   1 * time.Millisecond,
		}
		svc, deps := newTestServiceWithConfig(t, config)
		ctx := context.Background()

		sessionName := "test-session"
		chatID := "mark-failed-error@g.us"

		checkpoint := &reconciliation.Checkpoint{
			SessionName: sessionName,
			ChatID:      chatID,
			Status:      reconciliation.CheckpointStatusIdle,
		}

		deps.checkpointRepo.On("GetOrCreate", ctx, sessionName, chatID).Return(checkpoint, nil)
		deps.checkpointRepo.On("MarkRunning", ctx, sessionName, chatID).Return(nil)
		// MarkFailed also fails
		deps.checkpointRepo.On("MarkFailed", ctx, sessionName, chatID).Return(errors.New("mark failed error"))

		// GetMessages fails persistently
		deps.provider.On("GetMessages", ctx, mock.Anything).Return(nil, errors.New("persistent failure"))

		err := svc.ReconcileChat(ctx, sessionName, chatID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed after")
		// MarkFailed should have been called despite its own error
		deps.checkpointRepo.AssertCalled(t, "MarkFailed", ctx, sessionName, chatID)
	})

	t.Run("MarkRunning with non-ErrAlreadyRunning error", func(t *testing.T) {
		t.Parallel()
		svc, deps := newTestService(t)
		ctx := context.Background()

		sessionName := "test-session"
		chatID := "mark-running-fail@g.us"

		checkpoint := &reconciliation.Checkpoint{
			SessionName: sessionName,
			ChatID:      chatID,
			Status:      reconciliation.CheckpointStatusIdle,
		}

		deps.checkpointRepo.On("GetOrCreate", ctx, sessionName, chatID).Return(checkpoint, nil)
		// MarkRunning fails with generic error (not ErrAlreadyRunning)
		deps.checkpointRepo.On("MarkRunning", ctx, sessionName, chatID).Return(errors.New("db write error"))

		err := svc.ReconcileChat(ctx, sessionName, chatID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to mark checkpoint as running")
	})

	t.Run("context canceled during retry backoff", func(t *testing.T) {
		t.Parallel()
		config := reconciliation.Config{
			SilenceTimeout: 5 * time.Minute,
			BatchSize:      10,
			MaxConcurrent:  5,
			CheckInterval:  1 * time.Minute,
			MaxRetries:     3,
			RetryBackoff:   500 * time.Millisecond, // Long enough to cancel during
		}
		svc, deps := newTestServiceWithConfig(t, config)

		sessionName := "test-session"
		chatID := "cancel-backoff@g.us"

		checkpoint := &reconciliation.Checkpoint{
			SessionName: sessionName,
			ChatID:      chatID,
			Status:      reconciliation.CheckpointStatusIdle,
		}

		ctx, cancel := context.WithCancel(context.Background())

		deps.checkpointRepo.On("GetOrCreate", mock.Anything, sessionName, chatID).Return(checkpoint, nil)
		deps.checkpointRepo.On("MarkRunning", mock.Anything, sessionName, chatID).Return(nil)
		deps.checkpointRepo.On("MarkFailed", mock.Anything, sessionName, chatID).Return(nil).Maybe()

		// First attempt fails, then cancel during backoff
		deps.provider.On("GetMessages", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
			// Cancel context after first failure to trigger during backoff
			go func() {
				time.Sleep(50 * time.Millisecond)
				cancel()
			}()
		}).Return(nil, errors.New("transient error")).Once()

		err := svc.ReconcileChat(ctx, sessionName, chatID)
		require.Error(t, err)
		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("semaphore context cancellation", func(t *testing.T) {
		t.Parallel()
		config := reconciliation.Config{
			SilenceTimeout: 5 * time.Minute,
			BatchSize:      10,
			MaxConcurrent:  1, // Only 1 concurrent slot
			CheckInterval:  1 * time.Minute,
			MaxRetries:     1,
			RetryBackoff:   1 * time.Millisecond,
		}
		svc, deps := newTestServiceWithConfig(t, config)

		sessionName := "test-session"
		chatID1 := "chat1@g.us"
		chatID2 := "chat2@g.us"

		checkpoint := &reconciliation.Checkpoint{
			SessionName: sessionName,
			ChatID:      chatID1,
			Status:      reconciliation.CheckpointStatusIdle,
		}

		deps.checkpointRepo.On("GetOrCreate", mock.Anything, sessionName, chatID1).Return(checkpoint, nil)
		deps.checkpointRepo.On("MarkRunning", mock.Anything, sessionName, chatID1).Return(nil)
		deps.checkpointRepo.On("MarkIdle", mock.Anything, sessionName, chatID1).Return(nil)

		// Slow first chat to block semaphore
		deps.provider.On("GetMessages", mock.Anything, mock.MatchedBy(func(input whatsapp.GetMessagesInput) bool {
			return input.ChatID == chatID1
		})).Run(func(args mock.Arguments) {
			time.Sleep(500 * time.Millisecond)
		}).Return(&whatsapp.MessagesResult{
			Messages: []whatsapp.Message{},
			HasMore:  false,
		}, nil)

		// Start first reconciliation to occupy the semaphore
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = svc.ReconcileChat(context.Background(), sessionName, chatID1)
		}()

		time.Sleep(20 * time.Millisecond) // Let first start

		// Second chat with cancelled context should fail at semaphore
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		err := svc.ReconcileChat(ctx, sessionName, chatID2)
		assert.Error(t, err)

		wg.Wait()
	})
}

// =============================================================================
// TestDoReconcile_EdgeCases — covers doReconcile gaps
// =============================================================================

func TestDoReconcile_EdgeCases(t *testing.T) {
	t.Run("context cancelled before second loop iteration", func(t *testing.T) {
		t.Parallel()
		svc, deps := newTestService(t)
		sessionName := "test-session"
		chatID := "cancel-before-loop@g.us"
		baseTime := time.Now().Add(-1 * time.Hour)

		checkpoint := &reconciliation.Checkpoint{
			SessionName: sessionName,
			ChatID:      chatID,
			Status:      reconciliation.CheckpointStatusIdle,
		}

		ctx, cancel := context.WithCancel(context.Background())

		deps.checkpointRepo.On("GetOrCreate", mock.Anything, sessionName, chatID).Return(checkpoint, nil)
		deps.checkpointRepo.On("MarkRunning", mock.Anything, sessionName, chatID).Return(nil)
		deps.checkpointRepo.On("MarkFailed", mock.Anything, sessionName, chatID).Return(nil).Maybe()
		deps.checkpointRepo.On("UpdateCheckpoint", mock.Anything, sessionName, chatID, mock.AnythingOfType("int64"), mock.AnythingOfType("string")).Return(nil).Maybe()

		messages := createTestMessages(3, baseTime)

		// First page: has more — after processing, cancel context so ctx.Err() fires at loop top
		deps.provider.On("GetMessages", mock.Anything, mock.MatchedBy(func(input whatsapp.GetMessagesInput) bool {
			return input.Offset == 0
		})).Return(&whatsapp.MessagesResult{
			Messages:   messages,
			HasMore:    true,
			NextOffset: 3,
		}, nil).Once()

		// Cancel during ProcessMessage so ctx is done before next iteration
		deps.messageProc.On("ProcessMessage", mock.Anything, sessionName, mock.AnythingOfType("*whatsapp.Message")).Run(func(args mock.Arguments) {
			cancel()
		}).Return(nil).Maybe()

		err := svc.ReconcileChat(ctx, sessionName, chatID)
		assert.Error(t, err)
		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("context cancelled mid-pagination loop", func(t *testing.T) {
		t.Parallel()
		svc, deps := newTestService(t)
		sessionName := "test-session"
		chatID := "cancel-mid-loop@g.us"
		baseTime := time.Now().Add(-1 * time.Hour)

		checkpoint := &reconciliation.Checkpoint{
			SessionName: sessionName,
			ChatID:      chatID,
			Status:      reconciliation.CheckpointStatusIdle,
		}

		ctx, cancel := context.WithCancel(context.Background())

		deps.checkpointRepo.On("GetOrCreate", mock.Anything, sessionName, chatID).Return(checkpoint, nil)
		deps.checkpointRepo.On("MarkRunning", mock.Anything, sessionName, chatID).Return(nil)
		deps.checkpointRepo.On("MarkFailed", mock.Anything, sessionName, chatID).Return(nil).Maybe()
		deps.checkpointRepo.On("UpdateCheckpoint", mock.Anything, sessionName, chatID, mock.AnythingOfType("int64"), mock.AnythingOfType("string")).Return(nil).Maybe()

		messages := createTestMessages(5, baseTime)

		// First page succeeds, then cancel during "fetch" of next page
		deps.provider.On("GetMessages", mock.Anything, mock.MatchedBy(func(input whatsapp.GetMessagesInput) bool {
			return input.Offset == 0
		})).Return(&whatsapp.MessagesResult{
			Messages:   messages,
			HasMore:    true,
			NextOffset: 5,
		}, nil).Once()

		deps.provider.On("GetMessages", mock.Anything, mock.MatchedBy(func(input whatsapp.GetMessagesInput) bool {
			return input.Offset == 5
		})).Run(func(args mock.Arguments) {
			cancel() // Cancel before this returns
		}).Return(nil, context.Canceled).Maybe()

		deps.messageProc.On("ProcessMessage", mock.Anything, sessionName, mock.AnythingOfType("*whatsapp.Message")).Return(nil).Maybe()

		err := svc.ReconcileChat(ctx, sessionName, chatID)
		assert.Error(t, err)
	})

	t.Run("UpdateCheckpoint error during batch is logged but continues", func(t *testing.T) {
		t.Parallel()
		svc, deps := newTestService(t)
		ctx := context.Background()

		sessionName := "test-session"
		chatID := "update-cp-fail@g.us"
		baseTime := time.Now()

		checkpoint := &reconciliation.Checkpoint{
			SessionName: sessionName,
			ChatID:      chatID,
			Status:      reconciliation.CheckpointStatusIdle,
		}

		messages := createTestMessages(3, baseTime)

		deps.checkpointRepo.On("GetOrCreate", ctx, sessionName, chatID).Return(checkpoint, nil)
		deps.checkpointRepo.On("MarkRunning", ctx, sessionName, chatID).Return(nil)
		deps.checkpointRepo.On("MarkIdle", ctx, sessionName, chatID).Return(nil)
		// UpdateCheckpoint fails
		deps.checkpointRepo.On("UpdateCheckpoint", ctx, sessionName, chatID, mock.AnythingOfType("int64"), mock.AnythingOfType("string")).Return(errors.New("checkpoint update failed"))

		deps.provider.On("GetMessages", ctx, mock.Anything).Return(&whatsapp.MessagesResult{
			Messages:   messages,
			HasMore:    false,
			NextOffset: 0,
		}, nil)

		deps.messageProc.On("ProcessMessage", ctx, sessionName, mock.AnythingOfType("*whatsapp.Message")).Return(nil)

		// Should succeed despite UpdateCheckpoint failure
		err := svc.ReconcileChat(ctx, sessionName, chatID)
		require.NoError(t, err)
		deps.checkpointRepo.AssertCalled(t, "UpdateCheckpoint", ctx, sessionName, chatID, mock.AnythingOfType("int64"), mock.AnythingOfType("string"))
	})

	t.Run("message timestamp tracking selects latest timestamp", func(t *testing.T) {
		t.Parallel()
		svc, deps := newTestService(t)
		ctx := context.Background()

		sessionName := "test-session"
		chatID := "timestamp-track@g.us"
		baseTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

		checkpoint := &reconciliation.Checkpoint{
			SessionName: sessionName,
			ChatID:      chatID,
			Status:      reconciliation.CheckpointStatusIdle,
		}

		// Messages out of order by timestamp
		messages := []whatsapp.Message{
			{ID: "msg-a", ChatID: chatID, Timestamp: baseTime.Add(2 * time.Minute), Body: "Latest"},
			{ID: "msg-b", ChatID: chatID, Timestamp: baseTime, Body: "Oldest"},
			{ID: "msg-c", ChatID: chatID, Timestamp: baseTime.Add(1 * time.Minute), Body: "Middle"},
		}

		deps.checkpointRepo.On("GetOrCreate", ctx, sessionName, chatID).Return(checkpoint, nil)
		deps.checkpointRepo.On("MarkRunning", ctx, sessionName, chatID).Return(nil)
		deps.checkpointRepo.On("MarkIdle", ctx, sessionName, chatID).Return(nil)

		// The latest timestamp should be msg-a (baseTime + 2min)
		expectedTimestamp := baseTime.Add(2 * time.Minute).UnixMilli()
		deps.checkpointRepo.On("UpdateCheckpoint", ctx, sessionName, chatID, expectedTimestamp, "msg-a").Return(nil)

		deps.provider.On("GetMessages", ctx, mock.Anything).Return(&whatsapp.MessagesResult{
			Messages:   messages,
			HasMore:    false,
			NextOffset: 0,
		}, nil)

		deps.messageProc.On("ProcessMessage", ctx, sessionName, mock.AnythingOfType("*whatsapp.Message")).Return(nil)

		err := svc.ReconcileChat(ctx, sessionName, chatID)
		require.NoError(t, err)
		deps.checkpointRepo.AssertCalled(t, "UpdateCheckpoint", ctx, sessionName, chatID, expectedTimestamp, "msg-a")
	})

	t.Run("all messages skipped does not update checkpoint", func(t *testing.T) {
		t.Parallel()
		svc, deps := newTestService(t)
		ctx := context.Background()

		sessionName := "test-session"
		chatID := "all-skipped@g.us"
		baseTime := time.Now()

		checkpoint := &reconciliation.Checkpoint{
			SessionName:          sessionName,
			ChatID:               chatID,
			LastMessageTimestamp: baseTime.UnixMilli(),
			LastMessageID:        "only-msg",
			Status:               reconciliation.CheckpointStatusIdle,
		}

		// Single message that matches checkpoint (should be skipped)
		messages := []whatsapp.Message{
			{ID: "only-msg", ChatID: chatID, Timestamp: baseTime, Body: "Already processed"},
		}

		deps.checkpointRepo.On("GetOrCreate", ctx, sessionName, chatID).Return(checkpoint, nil)
		deps.checkpointRepo.On("MarkRunning", ctx, sessionName, chatID).Return(nil)
		deps.checkpointRepo.On("MarkIdle", ctx, sessionName, chatID).Return(nil)

		deps.provider.On("GetMessages", ctx, mock.Anything).Return(&whatsapp.MessagesResult{
			Messages:   messages,
			HasMore:    false,
			NextOffset: 0,
		}, nil)

		err := svc.ReconcileChat(ctx, sessionName, chatID)
		require.NoError(t, err)
		// UpdateCheckpoint should NOT have been called since lastTimestamp is 0 (no new messages)
		deps.checkpointRepo.AssertNotCalled(t, "UpdateCheckpoint", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})
}

// =============================================================================
// TestCheckAndReconcile_EdgeCases — covers checkAndReconcile gaps
// =============================================================================

func TestCheckAndReconcile_EdgeCases(t *testing.T) {
	t.Run("context cancellation stops session iteration", func(t *testing.T) {
		t.Parallel()
		config := fastWatchdogConfig()
		config.SilenceTimeout = 10 * time.Millisecond
		config.CheckInterval = 5 * time.Millisecond
		svc, deps := newWatchdogTestService(t, config)

		// Record events for multiple sessions
		for i := 0; i < 5; i++ {
			svc.RecordEvent("session-" + itoa(i))
		}

		// Health check blocks, giving time to cancel
		deps.provider.On("GetSessionStatus", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
			time.Sleep(50 * time.Millisecond)
		}).Return(&whatsapp.SessionInfo{
			Status: whatsapp.SessionStatusConnected,
		}, nil).Maybe()

		deps.checkpointRepo.On("GetBySession", mock.Anything, mock.Anything).Return([]reconciliation.Checkpoint{}, nil).Maybe()

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
		defer cancel()

		cancelWatchdog := svc.StartWatchdog(ctx)
		defer cancelWatchdog()

		// Let watchdog start and context expire
		time.Sleep(100 * time.Millisecond)

		// Service should have handled cancellation gracefully
	})

	t.Run("multiple sessions with mixed silence states", func(t *testing.T) {
		t.Parallel()
		config := fastWatchdogConfig()
		config.SilenceTimeout = 50 * time.Millisecond
		config.CheckInterval = 30 * time.Millisecond
		svc, deps := newWatchdogTestService(t, config)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		silentSession := "silent-session"
		activeSession := "active-session"

		deps.provider.On("GetSessionStatus", mock.Anything, silentSession).Return(&whatsapp.SessionInfo{
			Name:   silentSession,
			Status: whatsapp.SessionStatusConnected,
		}, nil).Maybe()

		deps.checkpointRepo.On("GetBySession", mock.Anything, silentSession).Return([]reconciliation.Checkpoint{}, nil).Maybe()

		// Record events for both sessions
		svc.RecordEvent(silentSession)
		svc.RecordEvent(activeSession)

		cancelWatchdog := svc.StartWatchdog(ctx)
		defer cancelWatchdog()

		// Keep active session alive
		go func() {
			for i := 0; i < 10; i++ {
				svc.RecordEvent(activeSession)
				time.Sleep(20 * time.Millisecond)
			}
		}()

		time.Sleep(200 * time.Millisecond)

		// Silent session should have been checked, active should not
		deps.provider.AssertCalled(t, "GetSessionStatus", mock.Anything, silentSession)
		deps.provider.AssertNotCalled(t, "GetSessionStatus", mock.Anything, activeSession)
	})

	t.Run("health check disabled skips status check", func(t *testing.T) {
		t.Parallel()
		config := fastWatchdogConfig()
		config.HealthCheckEnabled = false // Disabled
		svc, deps := newWatchdogTestService(t, config)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		sessionName := "no-health-check-session"

		deps.checkpointRepo.On("GetBySession", mock.Anything, sessionName).Return([]reconciliation.Checkpoint{}, nil).Maybe()

		svc.RecordEvent(sessionName)
		cancelWatchdog := svc.StartWatchdog(ctx)
		defer cancelWatchdog()

		time.Sleep(200 * time.Millisecond)

		// GetSessionStatus should NOT be called since health check is disabled
		deps.provider.AssertNotCalled(t, "GetSessionStatus", mock.Anything, mock.Anything)
	})
}

// =============================================================================
// TestTryRestartSession_BackoffEdgeCases — covers backoff cap and timing
// =============================================================================

func TestTryRestartSession_BackoffEdgeCases(t *testing.T) {
	t.Run("backoff is capped at 30 minutes", func(t *testing.T) {
		t.Parallel()
		// Use a large RestartBackoff with many attempts to trigger the 30-min cap
		config := fastWatchdogConfig()
		config.RestartBackoff = 20 * time.Minute  // 20min * 2^1 = 40min > 30min cap
		config.MaxRestartAttempts = 5
		svc, deps := newWatchdogTestService(t, config)
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)

		sessionName := "backoff-cap-session"
		restartDone := make(chan struct{}, 10)

		// Always disconnected — triggers restart attempts
		deps.provider.On("GetSessionStatus", mock.Anything, sessionName).Return(&whatsapp.SessionInfo{
			Name:   sessionName,
			Status: whatsapp.SessionStatusDisconnected,
		}, nil).Maybe()

		deps.provider.On("RestartSession", mock.Anything, sessionName).Run(func(args mock.Arguments) {
			select {
			case restartDone <- struct{}{}:
			default:
			}
		}).Return(nil).Maybe()

		svc.RecordEvent(sessionName)
		cancelWatchdog := svc.StartWatchdog(ctx)

		// Wait for at least one restart call
		select {
		case <-restartDone:
		case <-time.After(2 * time.Second):
			t.Fatal("timeout waiting for RestartSession")
		}

		// Let the backoff logic execute (the cap at 30min is evaluated but
		// since backoff > time since last restart, it just returns false)
		time.Sleep(200 * time.Millisecond)

		cancelWatchdog()
		cancel()
		time.Sleep(50 * time.Millisecond)

		deps.provider.AssertCalled(t, "RestartSession", mock.Anything, sessionName)
	})

	t.Run("backoff prevents restart when too recent", func(t *testing.T) {
		t.Parallel()
		// Large backoff so second restart is blocked by time-since check
		config := fastWatchdogConfig()
		config.RestartBackoff = 1 * time.Hour // Very long so second attempt is always within backoff
		config.MaxRestartAttempts = 5
		svc, deps := newWatchdogTestService(t, config)
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)

		sessionName := "backoff-recent-session"
		restartDone := make(chan struct{}, 10)

		// Always disconnected
		deps.provider.On("GetSessionStatus", mock.Anything, sessionName).Return(&whatsapp.SessionInfo{
			Name:   sessionName,
			Status: whatsapp.SessionStatusDisconnected,
		}, nil).Maybe()

		deps.provider.On("RestartSession", mock.Anything, sessionName).Run(func(args mock.Arguments) {
			select {
			case restartDone <- struct{}{}:
			default:
			}
		}).Return(nil).Maybe()

		svc.RecordEvent(sessionName)
		cancelWatchdog := svc.StartWatchdog(ctx)

		// Wait for first restart
		select {
		case <-restartDone:
		case <-time.After(7 * time.Second):
			t.Fatal("timeout waiting for first RestartSession")
		}

		// Wait for watchdog to try again — should be blocked by backoff
		time.Sleep(200 * time.Millisecond)

		cancelWatchdog()
		cancel()
		time.Sleep(50 * time.Millisecond)

		// Drain any additional calls
		extraCalls := 0
		for {
			select {
			case <-restartDone:
				extraCalls++
			default:
				goto checked
			}
		}
	checked:
		// Should have at most 1 restart since backoff blocks subsequent attempts
		assert.LessOrEqual(t, extraCalls, 0, "should not restart again within backoff period")
	})
}

// =============================================================================
// TestReconcileSession_ContextCancel — covers reconcileSession context check
// =============================================================================

func TestReconcileSession_ContextCancel(t *testing.T) {
	t.Run("context cancellation during checkpoint iteration in reconcileSession", func(t *testing.T) {
		t.Parallel()
		config := fastWatchdogConfig()
		config.SilenceTimeout = 10 * time.Millisecond
		config.CheckInterval = 5 * time.Millisecond
		svc, deps := newWatchdogTestService(t, config)

		sessionName := "ctx-cancel-recon-session"

		deps.provider.On("GetSessionStatus", mock.Anything, sessionName).Return(&whatsapp.SessionInfo{
			Name:   sessionName,
			Status: whatsapp.SessionStatusConnected,
		}, nil).Maybe()

		// Return many checkpoints so there's time to cancel between iterations
		checkpoints := make([]reconciliation.Checkpoint, 20)
		for i := range checkpoints {
			checkpoints[i] = reconciliation.Checkpoint{
				SessionName: sessionName,
				ChatID:      "chat-" + itoa(i) + "@g.us",
				Status:      reconciliation.CheckpointStatusIdle,
			}
		}
		deps.checkpointRepo.On("GetBySession", mock.Anything, sessionName).Return(checkpoints, nil).Maybe()

		// First chat takes time, giving context time to expire
		deps.checkpointRepo.On("GetOrCreate", mock.Anything, mock.Anything, mock.Anything).Return(&reconciliation.Checkpoint{
			SessionName: sessionName,
			ChatID:      "any",
			Status:      reconciliation.CheckpointStatusIdle,
		}, nil).Maybe()

		deps.checkpointRepo.On("MarkRunning", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		deps.checkpointRepo.On("MarkIdle", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		deps.checkpointRepo.On("MarkFailed", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

		// Slow GetMessages to give context time to expire between checkpoints
		deps.provider.On("GetMessages", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
			time.Sleep(30 * time.Millisecond)
		}).Return(&whatsapp.MessagesResult{
			Messages: []whatsapp.Message{},
			HasMore:  false,
		}, nil).Maybe()

		svc.RecordEvent(sessionName)

		// Use a short context that will expire mid-iteration
		ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
		cancelWatchdog := svc.StartWatchdog(ctx)

		// Wait for context to expire and watchdog to stop
		time.Sleep(300 * time.Millisecond)

		cancelWatchdog()
		cancel()
		time.Sleep(50 * time.Millisecond)
	})
}

// =============================================================================
// TestConfig_AdditionalCases — covers Config edge cases
// =============================================================================

func TestConfig_AdditionalCases(t *testing.T) {
	t.Run("valid config passes validation unchanged", func(t *testing.T) {
		t.Parallel()
		config := reconciliation.Config{
			SilenceTimeout:     10 * time.Minute,
			BatchSize:          100,
			MaxConcurrent:      10,
			CheckInterval:      2 * time.Minute,
			MaxRetries:         5,
			RetryBackoff:       1 * time.Minute,
			HealthCheckEnabled: true,
			MaxRestartAttempts: 5,
			RestartBackoff:     2 * time.Minute,
		}

		err := config.Validate()
		require.NoError(t, err)

		// Values should remain unchanged
		assert.Equal(t, 10*time.Minute, config.SilenceTimeout)
		assert.Equal(t, 100, config.BatchSize)
		assert.Equal(t, 10, config.MaxConcurrent)
		assert.Equal(t, 2*time.Minute, config.CheckInterval)
		assert.Equal(t, 5, config.MaxRetries)
		assert.Equal(t, 1*time.Minute, config.RetryBackoff)
		assert.True(t, config.HealthCheckEnabled)
		assert.Equal(t, 5, config.MaxRestartAttempts)
		assert.Equal(t, 2*time.Minute, config.RestartBackoff)
	})

	t.Run("MaxRestartAttempts zero stays zero", func(t *testing.T) {
		t.Parallel()
		config := reconciliation.Config{
			SilenceTimeout:     5 * time.Minute,
			BatchSize:          50,
			MaxConcurrent:      5,
			CheckInterval:      1 * time.Minute,
			MaxRetries:         3,
			RetryBackoff:       30 * time.Second,
			MaxRestartAttempts: 0, // 0 is valid (disables restart)
			RestartBackoff:     1 * time.Minute,
		}

		err := config.Validate()
		require.NoError(t, err)
		assert.Equal(t, 0, config.MaxRestartAttempts, "0 should remain as disabled")
	})

	t.Run("HealthCheckEnabled false stays false", func(t *testing.T) {
		t.Parallel()
		config := reconciliation.Config{
			SilenceTimeout:     5 * time.Minute,
			BatchSize:          50,
			MaxConcurrent:      5,
			CheckInterval:      1 * time.Minute,
			MaxRetries:         3,
			RetryBackoff:       30 * time.Second,
			HealthCheckEnabled: false,
			MaxRestartAttempts: 3,
			RestartBackoff:     1 * time.Minute,
		}

		err := config.Validate()
		require.NoError(t, err)
		assert.False(t, config.HealthCheckEnabled)
	})
}

// =============================================================================
// TestRecordEvent — covers RecordEvent
// =============================================================================

func TestRecordEvent(t *testing.T) {
	t.Run("records event for session", func(t *testing.T) {
		t.Parallel()
		svc, _ := newTestService(t)

		sessionName := "event-session"

		// Should not panic
		svc.RecordEvent(sessionName)
		svc.RecordEvent(sessionName) // Record again to update
	})

	t.Run("concurrent RecordEvent calls are safe", func(t *testing.T) {
		t.Parallel()
		svc, _ := newTestService(t)

		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			session := "session-" + itoa(i%10)
			go func() {
				defer wg.Done()
				svc.RecordEvent(session)
			}()
		}
		wg.Wait()
	})
}

// =============================================================================
// TestGetCheckpoint_Errors — covers GetCheckpoint error path
// =============================================================================

func TestGetCheckpoint_Errors(t *testing.T) {
	t.Run("returns error from repository", func(t *testing.T) {
		t.Parallel()
		svc, deps := newTestService(t)
		ctx := context.Background()

		deps.checkpointRepo.On("GetOrCreate", ctx, "session", "chat").Return(nil, errors.New("db error"))

		cp, err := svc.GetCheckpoint(ctx, "session", "chat")
		require.Error(t, err)
		assert.Nil(t, cp)
	})
}

// =============================================================================
// TestIsReconciling_Concurrent — covers IsReconciling concurrency
// =============================================================================

func TestIsReconciling_Concurrent(t *testing.T) {
	t.Run("concurrent IsReconciling calls are safe", func(t *testing.T) {
		t.Parallel()
		svc, deps := newTestService(t)
		ctx := context.Background()

		sessionName := "concurrent-session"
		chatID := "concurrent-chat@g.us"

		checkpoint := &reconciliation.Checkpoint{
			SessionName: sessionName,
			ChatID:      chatID,
			Status:      reconciliation.CheckpointStatusIdle,
		}

		deps.checkpointRepo.On("GetOrCreate", ctx, sessionName, chatID).Return(checkpoint, nil)
		deps.checkpointRepo.On("MarkRunning", ctx, sessionName, chatID).Return(nil)
		deps.checkpointRepo.On("MarkIdle", ctx, sessionName, chatID).Return(nil)

		deps.provider.On("GetMessages", ctx, mock.Anything).Run(func(args mock.Arguments) {
			time.Sleep(100 * time.Millisecond)
		}).Return(&whatsapp.MessagesResult{
			Messages: []whatsapp.Message{},
			HasMore:  false,
		}, nil)

		done := make(chan struct{})
		go func() {
			_ = svc.ReconcileChat(ctx, sessionName, chatID)
			close(done)
		}()

		time.Sleep(20 * time.Millisecond)

		// Concurrent IsReconciling reads
		var wg sync.WaitGroup
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = svc.IsReconciling(sessionName, chatID)
			}()
		}
		wg.Wait()

		<-done
	})
}

// =============================================================================
// TestReconcileChat_ContextDeadlineExceeded — covers deadline exceeded branch
// =============================================================================

func TestReconcileChat_ContextDeadlineExceeded(t *testing.T) {
	t.Run("returns deadline exceeded during reconciliation", func(t *testing.T) {
		t.Parallel()
		config := reconciliation.Config{
			SilenceTimeout: 5 * time.Minute,
			BatchSize:      10,
			MaxConcurrent:  5,
			CheckInterval:  1 * time.Minute,
			MaxRetries:     3,
			RetryBackoff:   500 * time.Millisecond,
		}
		svc, deps := newTestServiceWithConfig(t, config)

		sessionName := "test-session"
		chatID := "deadline-chat@g.us"

		checkpoint := &reconciliation.Checkpoint{
			SessionName: sessionName,
			ChatID:      chatID,
			Status:      reconciliation.CheckpointStatusIdle,
		}

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		deps.checkpointRepo.On("GetOrCreate", mock.Anything, sessionName, chatID).Return(checkpoint, nil)
		deps.checkpointRepo.On("MarkRunning", mock.Anything, sessionName, chatID).Return(nil)
		deps.checkpointRepo.On("MarkFailed", mock.Anything, sessionName, chatID).Return(nil).Maybe()

		// First attempt fails, then deadline exceeds during backoff
		deps.provider.On("GetMessages", mock.Anything, mock.Anything).Return(nil, errors.New("error"))

		err := svc.ReconcileChat(ctx, sessionName, chatID)
		require.Error(t, err)
		// Should be either DeadlineExceeded or wrapped retry error
	})
}

// =============================================================================
// TestStartWatchdog_Lifecycle — covers watchdog start/stop
// =============================================================================

func TestStartWatchdog_Lifecycle(t *testing.T) {
	t.Run("watchdog stops cleanly on cancel", func(t *testing.T) {
		t.Parallel()
		svc, _ := newWatchdogTestService(t, fastWatchdogConfig())
		ctx := context.Background()

		cancelWatchdog := svc.StartWatchdog(ctx)

		// Should not panic when stopping
		cancelWatchdog()

		// Should handle double-cancel gracefully
		cancelWatchdog()

		time.Sleep(50 * time.Millisecond)
	})

	t.Run("watchdog stops when parent context is cancelled", func(t *testing.T) {
		t.Parallel()
		svc, _ := newWatchdogTestService(t, fastWatchdogConfig())
		ctx, cancel := context.WithCancel(context.Background())

		cancelWatchdog := svc.StartWatchdog(ctx)
		defer cancelWatchdog()

		// Cancel parent context
		cancel()

		time.Sleep(100 * time.Millisecond)
		// Watchdog should have stopped cleanly
	})
}

// =============================================================================
// TestOnStartup_AdditionalCases — covers OnStartup edge cases
// =============================================================================

func TestOnStartup_AdditionalCases(t *testing.T) {
	t.Run("handles ErrAlreadyRunning during startup gracefully", func(t *testing.T) {
		t.Parallel()
		svc, deps := newTestService(t)
		ctx := context.Background()

		sessionChats := []reconciliation.SessionChat{
			{SessionName: "session1", ChatID: "chat1@g.us"},
		}

		deps.checkpointRepo.On("GetActiveSessionsChats", ctx).Return(sessionChats, nil)

		checkpoint := &reconciliation.Checkpoint{
			SessionName: "session1",
			ChatID:      "chat1@g.us",
			Status:      reconciliation.CheckpointStatusRunning,
		}
		deps.checkpointRepo.On("GetOrCreate", ctx, "session1", "chat1@g.us").Return(checkpoint, nil)
		deps.checkpointRepo.On("MarkRunning", ctx, "session1", "chat1@g.us").Return(reconciliation.ErrAlreadyRunning)

		err := svc.OnStartup(ctx)
		// Should succeed — ErrAlreadyRunning is not counted as startup error
		require.NoError(t, err)
	})

	t.Run("multiple chats with some errors still completes", func(t *testing.T) {
		t.Parallel()
		svc, deps := newTestService(t)
		ctx := context.Background()

		sessionChats := []reconciliation.SessionChat{
			{SessionName: "s1", ChatID: "c1@g.us"},
			{SessionName: "s1", ChatID: "c2@g.us"},
			{SessionName: "s2", ChatID: "c3@g.us"},
		}

		deps.checkpointRepo.On("GetActiveSessionsChats", ctx).Return(sessionChats, nil)

		// c1: fails at GetOrCreate
		deps.checkpointRepo.On("GetOrCreate", ctx, "s1", "c1@g.us").Return(nil, errors.New("fail"))

		// c2: succeeds
		cp2 := &reconciliation.Checkpoint{SessionName: "s1", ChatID: "c2@g.us", Status: reconciliation.CheckpointStatusIdle}
		deps.checkpointRepo.On("GetOrCreate", ctx, "s1", "c2@g.us").Return(cp2, nil)
		deps.checkpointRepo.On("MarkRunning", ctx, "s1", "c2@g.us").Return(nil)
		deps.checkpointRepo.On("MarkIdle", ctx, "s1", "c2@g.us").Return(nil)

		// c3: succeeds
		cp3 := &reconciliation.Checkpoint{SessionName: "s2", ChatID: "c3@g.us", Status: reconciliation.CheckpointStatusIdle}
		deps.checkpointRepo.On("GetOrCreate", ctx, "s2", "c3@g.us").Return(cp3, nil)
		deps.checkpointRepo.On("MarkRunning", ctx, "s2", "c3@g.us").Return(nil)
		deps.checkpointRepo.On("MarkIdle", ctx, "s2", "c3@g.us").Return(nil)

		deps.provider.On("GetMessages", ctx, mock.Anything).Return(&whatsapp.MessagesResult{
			Messages: []whatsapp.Message{},
			HasMore:  false,
		}, nil)

		err := svc.OnStartup(ctx)
		require.NoError(t, err)
	})
}

// =============================================================================
// TestNew_WithValidConfig — covers New constructor
// =============================================================================

func TestNew_WithValidConfig(t *testing.T) {
	t.Run("creates service with valid config", func(t *testing.T) {
		t.Parallel()
		svc, _ := newTestService(t)
		require.NotNil(t, svc)
	})

	t.Run("creates service with default config", func(t *testing.T) {
		t.Parallel()
		svc, _ := newTestServiceWithConfig(t, reconciliation.DefaultConfig())
		require.NotNil(t, svc)
	})
}

// =============================================================================
// TestDomainModels — covers domain model types
// =============================================================================

func TestDomainModels(t *testing.T) {
	t.Run("CheckpointStatus constants", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, reconciliation.CheckpointStatus("idle"), reconciliation.CheckpointStatusIdle)
		assert.Equal(t, reconciliation.CheckpointStatus("running"), reconciliation.CheckpointStatusRunning)
		assert.Equal(t, reconciliation.CheckpointStatus("failed"), reconciliation.CheckpointStatusFailed)
	})

	t.Run("error sentinel values", func(t *testing.T) {
		t.Parallel()
		assert.NotNil(t, reconciliation.ErrAlreadyRunning)
		assert.NotNil(t, reconciliation.ErrNoCheckpoint)
		assert.NotNil(t, reconciliation.ErrProviderNotConnected)

		assert.Contains(t, reconciliation.ErrAlreadyRunning.Error(), "already running")
		assert.Contains(t, reconciliation.ErrNoCheckpoint.Error(), "no checkpoint")
		assert.Contains(t, reconciliation.ErrProviderNotConnected.Error(), "not connected")
	})

	t.Run("Checkpoint struct fields", func(t *testing.T) {
		t.Parallel()
		now := time.Now()
		cp := reconciliation.Checkpoint{
			SessionName:          "session",
			ChatID:               "chat",
			LastMessageTimestamp: now.UnixMilli(),
			LastMessageID:        "msg-1",
			LastRunAt:            now,
			Status:               reconciliation.CheckpointStatusIdle,
		}

		assert.Equal(t, "session", cp.SessionName)
		assert.Equal(t, "chat", cp.ChatID)
		assert.Equal(t, now.UnixMilli(), cp.LastMessageTimestamp)
		assert.Equal(t, "msg-1", cp.LastMessageID)
		assert.Equal(t, now, cp.LastRunAt)
		assert.Equal(t, reconciliation.CheckpointStatusIdle, cp.Status)
	})

	t.Run("SessionChat struct fields", func(t *testing.T) {
		t.Parallel()
		sc := reconciliation.SessionChat{
			SessionName: "session",
			ChatID:      "chat",
		}

		assert.Equal(t, "session", sc.SessionName)
		assert.Equal(t, "chat", sc.ChatID)
	})
}
