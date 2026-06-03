package reconciliation

import (
	"context"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/models"

	"go.uber.org/zap"
)

// sessionLister lista as sessões do provedor WhatsApp (WAHA).
// *whatsapp.WahaAdapter implementa via ListAllSessions.
type sessionLister interface {
	ListAllSessions(ctx context.Context) ([]whatsapp.SessionInfo, error)
}

// instanceStore é o subconjunto do InstanceRepository usado na reconciliação.
type instanceStore interface {
	GetInstanceByName(ctx context.Context, sessionName string) (*models.Instance, error)
	UpdateInstanceStatus(ctx context.Context, instanceID int, status string) error
}

// InstanceStatusReconciler alinha o status das instâncias no banco com a verdade
// do WAHA. Existe porque, a cada restart/deploy do server, eventos
// session.status=WORKING se perdem e o banco fica travado em 'pending'/'disconnected'
// mesmo a sessão estando WORKING no WAHA — o que bloqueia TODOS os dispatches do
// cliente ("WhatsApp não está conectado") até alguém corrigir na mão.
type InstanceStatusReconciler struct {
	instances instanceStore
	sessions  sessionLister
	logger    *zap.Logger
}

// NewInstanceStatusReconciler cria o reconciliador.
func NewInstanceStatusReconciler(instances instanceStore, sessions sessionLister, logger *zap.Logger) *InstanceStatusReconciler {
	return &InstanceStatusReconciler{instances: instances, sessions: sessions, logger: logger}
}

// ReconcileStats resume o resultado de uma rodada.
type ReconcileStats struct {
	ConnectedSessions int // sessões WORKING no WAHA com instância correspondente
	Fixed             int // instâncias promovidas pra 'connected'
}

// Reconcile percorre as sessões do WAHA e PROMOVE pra 'connected' qualquer
// instância cujo banco esteja divergente (pending/disconnected) enquanto a
// sessão está WORKING. É CONSERVADOR de propósito: só promove, nunca rebaixa —
// o webhook de session.status trata desconexões em tempo real, e não rebaixar
// evita flapping por status transitório do WAHA. Falha de uma instância não
// interrompe as demais.
func (r *InstanceStatusReconciler) Reconcile(ctx context.Context) (ReconcileStats, error) {
	var stats ReconcileStats

	sessions, err := r.sessions.ListAllSessions(ctx)
	if err != nil {
		return stats, err
	}

	for i := range sessions {
		s := sessions[i]
		// Só nos interessa o caso WORKING (CONNECTED) — é o que destrava dispatch.
		if s.Name == "" || s.Status != whatsapp.SessionStatusConnected {
			continue
		}

		inst, err := r.instances.GetInstanceByName(ctx, s.Name)
		if err != nil || inst == nil {
			// Sessão sem instância correspondente no banco (órfã) — ignora.
			continue
		}
		stats.ConnectedSessions++

		if inst.Status == models.InstanceStatusConnected {
			continue // já bate, nada a fazer
		}

		if err := r.instances.UpdateInstanceStatus(ctx, inst.ID, models.InstanceStatusConnected); err != nil {
			r.logger.Warn("reconcile: falha ao corrigir status da instância",
				zap.String("session", s.Name),
				zap.Int("instanceID", inst.ID),
				zap.Error(err))
			continue
		}

		stats.Fixed++
		r.logger.Info("reconcile: status corrigido (WAHA WORKING, banco divergente)",
			zap.String("session", s.Name),
			zap.Int("instanceID", inst.ID),
			zap.String("from", inst.Status),
			zap.String("to", models.InstanceStatusConnected))
	}

	if stats.Fixed > 0 {
		r.logger.Info("reconcile de status de instâncias concluído",
			zap.Int("connectedSessions", stats.ConnectedSessions),
			zap.Int("fixed", stats.Fixed))
	}
	return stats, nil
}
