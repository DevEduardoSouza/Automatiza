// Package workers provides background job processing.
package workers

import (
	"context"
	"testing"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

// fakeSessionStatusChecker devolve um status fixo do "WAHA vivo".
type fakeSessionStatusChecker struct {
	info  *whatsapp.SessionInfo
	err   error
	calls int
}

func (f *fakeSessionStatusChecker) GetSessionStatus(_ context.Context, _ string) (*whatsapp.SessionInfo, error) {
	f.calls++
	return f.info, f.err
}

// Regressão (incidente isaa_deoliveira@hotmail.com, 04/06): numa reconexão o
// WAHA emite WORKING e STARTING quase juntos; processados fora de ordem pelo
// worker, o STARTING (→pending) sobrescrevia o WORKING (→connected) e a
// instância ficava travada em 'pending' bloqueando todos os dispatches.
// O fix confirma o status vivo no WAHA antes de aplicar um rebaixamento: se a
// sessão está de fato connected, o evento atrasado é ignorado.
func TestSessionStatus_StaleDowngradeIgnoredWhenWAHAConnected(t *testing.T) {
	instanceRepo := new(mockInstanceRepo)
	checker := &fakeSessionStatusChecker{
		info: &whatsapp.SessionInfo{Status: whatsapp.SessionStatusConnected},
	}

	worker := NewWebhookConsumerWorker(WebhookConsumerWorkerConfig{
		WebhookEventsRepo:    new(mockWebhookEventsRepo),
		InstanceRepo:         instanceRepo,
		SessionStatusChecker: checker,
		Logger:               zap.NewNop(),
	})

	ctx := context.Background()
	data := &WebhookJobData{
		EventID: "evt-stale-starting",
		Event:   "session.status",
		Session: "session-1",
		Payload: map[string]interface{}{"status": "STARTING"}, // rebaixamento → pending
	}

	instanceRepo.On("GetInstanceByName", ctx, "session-1").
		Return(&models.Instance{ID: 1, InstanceID: "session-1", Status: "connected"}, nil)
	// Não configuramos UpdateInstanceStatus: o rebaixamento deve ser ignorado.

	err := worker.routeEvent(ctx, data)

	assert.NoError(t, err)
	assert.Equal(t, 1, checker.calls, "deveria consultar o status vivo no WAHA antes de rebaixar")
	instanceRepo.AssertNotCalled(t, "UpdateInstanceStatus", mock.Anything, mock.Anything, mock.Anything)
}

// Contraparte: quando o WAHA confirma que a sessão NÃO está connected, o
// rebaixamento é legítimo e deve ser aplicado normalmente.
func TestSessionStatus_GenuineDowngradeApplied(t *testing.T) {
	instanceRepo := new(mockInstanceRepo)
	checker := &fakeSessionStatusChecker{
		info: &whatsapp.SessionInfo{Status: whatsapp.SessionStatusStarting},
	}

	worker := NewWebhookConsumerWorker(WebhookConsumerWorkerConfig{
		WebhookEventsRepo:    new(mockWebhookEventsRepo),
		InstanceRepo:         instanceRepo,
		SessionStatusChecker: checker,
		Logger:               zap.NewNop(),
	})

	ctx := context.Background()
	data := &WebhookJobData{
		EventID: "evt-genuine-starting",
		Event:   "session.status",
		Session: "session-1",
		Payload: map[string]interface{}{"status": "STARTING"},
	}

	instanceRepo.On("GetInstanceByName", ctx, "session-1").
		Return(&models.Instance{ID: 1, InstanceID: "session-1", Status: "connected"}, nil)
	instanceRepo.On("UpdateInstanceStatus", ctx, 1, "pending").Return(nil)

	err := worker.routeEvent(ctx, data)

	assert.NoError(t, err)
	instanceRepo.AssertExpectations(t)
	assert.Equal(t, 1, checker.calls)
}
