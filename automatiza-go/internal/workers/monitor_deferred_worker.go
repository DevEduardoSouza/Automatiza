package workers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
	"go.uber.org/zap"
)

// Worker do item diferido da cadência do espelho (Melhoria 7).
//
// É de propósito o handler mais burro do repositório: ele não decide nada.
// Toda a decisão (o item ainda vale? a janela abriu? que destino falta?) mora
// no serviço, que a toma contra o banco no momento da execução — porque entre
// o enfileiramento e agora tudo pode ter mudado.

// MonitorCadenceRunner é a fatia do serviço de monitoramento usada aqui.
type MonitorCadenceRunner interface {
	// RunDeferredDispatch executa o item. O par (generation, token) é a prova
	// de posse: se não casar com o banco, a execução é no-op.
	RunDeferredDispatch(ctx context.Context, pendingID int64, generation int, token string) error
}

// MonitorDeferredWorker processa TypeMonitoringDeferredDispatch.
type MonitorDeferredWorker struct {
	runner MonitorCadenceRunner
	logger *zap.Logger
}

// NewMonitorDeferredWorker cria o worker.
func NewMonitorDeferredWorker(runner MonitorCadenceRunner, logger *zap.Logger) *MonitorDeferredWorker {
	return &MonitorDeferredWorker{runner: runner, logger: logger}
}

// ProcessTask executa um item da fila de cadência.
//
// Payload malformado retorna nil (não retenta): não existe conserto para JSON
// quebrado, e o item continua protegido pela lease — se ele realmente estava
// devido, o poller o reemite quando ela vencer.
func (w *MonitorDeferredWorker) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var data DeferredDispatchJobData
	if err := json.Unmarshal(task.Payload(), &data); err != nil {
		w.logger.Error("monitor deferred: invalid payload", zap.Error(err))
		return nil
	}
	if data.PendingID <= 0 || data.Token == "" {
		w.logger.Error("monitor deferred: incomplete payload",
			zap.Int64("pendingID", data.PendingID))
		return nil
	}
	if w.runner == nil {
		return nil
	}

	if err := w.runner.RunDeferredDispatch(ctx, data.PendingID, data.Generation, data.Token); err != nil {
		w.logger.Error("monitor deferred: dispatch failed",
			zap.Int64("pendingID", data.PendingID),
			zap.Int("generation", data.Generation),
			zap.Error(err))
		return fmt.Errorf("deferred dispatch %d: %w", data.PendingID, err)
	}
	return nil
}
