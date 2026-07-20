package workers

import (
	"context"
	"errors"
	"testing"

	"automatiza-go/internal/config"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// Checagens executáveis do modo lean-persist (WEBHOOK_LEAN_PERSIST=true):
// caminho feliz usa SÓ MarkProcessedEnsure (nunca Exists/MarkProcessing/
// MarkProcessed); caminho de erro preserva a forense (InsertIfNotExists) e os
// branches de retry/DLQ idênticos ao clássico.

func leanWorker(t *testing.T, repo *mockWebhookEventsRepo) *WebhookConsumerWorker {
	t.Helper()
	t.Setenv("WEBHOOK_LEAN_PERSIST", "true")
	config.ResetFeatureFlags()
	t.Cleanup(config.ResetFeatureFlags)
	return &WebhookConsumerWorker{
		webhookEventsRepo: repo,
		logger:            zap.NewNop(),
	}
}

func leanTask(t *testing.T) *asynq.Task {
	t.Helper()
	// Tipo desconhecido: routeEvent devolve nil (skip) — serve como "sucesso"
	// sem precisar de processors injetados.
	payload := []byte(`{"event_id":"evt-lean-1","session":"sess-1","event":"unknown.event"}`)
	return asynq.NewTask(TypeWAHAWebhook, payload)
}

func TestLeanPersist_SuccessUsesOnlyMarkProcessedEnsure(t *testing.T) {
	repo := new(mockWebhookEventsRepo)
	w := leanWorker(t, repo)

	repo.On("MarkProcessedEnsure", mock.Anything, "evt-lean-1", "sess-1", "unknown.event").Return(nil).Once()

	err := w.ProcessTask(context.Background(), leanTask(t))
	require.NoError(t, err)

	repo.AssertExpectations(t)
	repo.AssertNotCalled(t, "Exists", mock.Anything, mock.Anything)
	repo.AssertNotCalled(t, "MarkProcessing", mock.Anything, mock.Anything)
	repo.AssertNotCalled(t, "MarkProcessed", mock.Anything, mock.Anything)
	repo.AssertNotCalled(t, "InsertIfNotExists", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestLeanPersist_MarkProcessedEnsureErrorDoesNotFailTask(t *testing.T) {
	repo := new(mockWebhookEventsRepo)
	w := leanWorker(t, repo)

	repo.On("MarkProcessedEnsure", mock.Anything, "evt-lean-1", "sess-1", "unknown.event").
		Return(errors.New("pg down")).Once()

	// Paridade com o clássico (:303): persistência falhar não falha a task.
	err := w.ProcessTask(context.Background(), leanTask(t))
	require.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestLeanPersist_ErrorPathEnsuresRowAndRetries(t *testing.T) {
	repo := new(mockWebhookEventsRepo)
	w := leanWorker(t, repo)

	// Processor de message que falha com erro retryável real do fluxo.
	msgProc := new(mockMessageProcessor)
	msgProc.On("ProcessMessage", mock.Anything, "sess-1", mock.Anything).
		Return(errors.New("temporary downstream failure")).Once()
	w.messageProcessor = msgProc

	payload := []byte(`{"event_id":"evt-lean-2","session":"sess-1","event":"message","payload":{"id":"m1"}}`)
	task := asynq.NewTask(TypeWAHAWebhook, payload)

	repo.On("InsertIfNotExists", mock.Anything, "evt-lean-2", "sess-1", "message").Return(true, nil).Once()
	// handleProcessingError (retryCount 0 < 3, erro retryável) → MarkFailed + return err
	repo.On("MarkFailed", mock.Anything, "evt-lean-2", mock.AnythingOfType("string")).Return(nil).Once()

	err := w.ProcessTask(context.Background(), task)
	require.Error(t, err, "erro retryável deve propagar para o asynq re-tentar")

	repo.AssertExpectations(t)
	repo.AssertNotCalled(t, "MarkProcessedEnsure", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	repo.AssertNotCalled(t, "MarkProcessing", mock.Anything, mock.Anything)
}

func TestLeanPersist_FlagOffKeepsClassicPath(t *testing.T) {
	repo := new(mockWebhookEventsRepo)
	t.Setenv("WEBHOOK_LEAN_PERSIST", "false")
	config.ResetFeatureFlags()
	t.Cleanup(config.ResetFeatureFlags)
	w := &WebhookConsumerWorker{webhookEventsRepo: repo, logger: zap.NewNop()}

	repo.On("Exists", mock.Anything, "evt-lean-1").Return(true, nil).Once()
	repo.On("MarkProcessing", mock.Anything, "evt-lean-1").Return(nil).Once()
	repo.On("MarkProcessed", mock.Anything, "evt-lean-1").Return(nil).Once()

	err := w.ProcessTask(context.Background(), leanTask(t))
	require.NoError(t, err)
	repo.AssertExpectations(t)
	repo.AssertNotCalled(t, "MarkProcessedEnsure", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}
