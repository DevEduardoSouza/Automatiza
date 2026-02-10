package backpressure

import (
	"context"

	"github.com/hibiken/asynq"
)

// AsynqQueueInspector adapts asynq.Inspector to the QueueInspector interface.
type AsynqQueueInspector struct {
	inspector *asynq.Inspector
	queueName string
}

// NewAsynqQueueInspector creates a new AsynqQueueInspector.
func NewAsynqQueueInspector(inspector *asynq.Inspector, queueName string) *AsynqQueueInspector {
	return &AsynqQueueInspector{
		inspector: inspector,
		queueName: queueName,
	}
}

// GetQueueDepth returns the total number of pending + active tasks in the queue.
func (a *AsynqQueueInspector) GetQueueDepth(ctx context.Context) (int, error) {
	info, err := a.inspector.GetQueueInfo(a.queueName)
	if err != nil {
		return 0, err
	}

	// Total depth = pending + active + scheduled (tasks about to be processed)
	return info.Pending + info.Active, nil
}

// Compile-time check that AsynqQueueInspector implements QueueInspector.
var _ QueueInspector = (*AsynqQueueInspector)(nil)
