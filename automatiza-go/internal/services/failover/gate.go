package failover

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	"automatiza-go/internal/observability"
	"automatiza-go/internal/repository"

	"github.com/getsentry/sentry-go"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	// leaseTTL é fixo em 60s: curto o bastante para que um produtor morto não
	// segure o gate por muito tempo, longo o bastante para sobreviver a um
	// soluço de rede entre dois heartbeats.
	leaseTTL = 60 * time.Second
	// leaseHeartbeat renova a cada 15s (4 tentativas dentro de um TTL).
	leaseHeartbeat = 15 * time.Second
	// releaseTimeout usa contexto independente: a lease precisa ser liberada
	// mesmo quando a execução foi cancelada.
	releaseTimeout = 5 * time.Second
)

// Gate é a barreira distribuída entre os produtores de envio e uma troca de
// chip em andamento.
//
// Contrato, na ordem em que importa:
//
//  1. Com a feature desligada e nenhuma operação pendente no boot, Acquire é um
//     no-op SEM I/O — o caminho quente não paga nada por uma feature apagada.
//  2. Se a automação está numa operação não terminal, Acquire falha ANTES de o
//     produtor começar qualquer trabalho.
//  3. Erro de banco ou heartbeat perdido é FAIL-CLOSED: o contexto da execução
//     é cancelado e nenhum envio novo sai. Numa troca de chip, "não sei se
//     ainda posso enviar" tem que parar, não continuar.
type Gate struct {
	repo    repository.InstanceFailoverRepository
	logger  *zap.Logger
	enabled func() bool

	// armed fica true quando o boot encontra operação não terminal. Ele existe
	// para o rollback da flag: desligar FEATURE_INSTANCE_FAILOVER com uma
	// operação em curso não pode abrir o gate no meio da troca.
	armed atomic.Bool

	holderID  string
	ttl       time.Duration
	heartbeat time.Duration
}

// NewGate cria o gate. `enabled` é lido a cada chamada (a flag pode ser
// desligada em runtime por restart).
func NewGate(repo repository.InstanceFailoverRepository, enabled func() bool, logger *zap.Logger) *Gate {
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "unknown"
	}
	return &Gate{
		repo:    repo,
		logger:  logger,
		enabled: enabled,
		// holder_id identifica o PROCESSO, nunca o usuário.
		holderID:  fmt.Sprintf("%s:%d:%s", host, os.Getpid(), uuid.NewString()),
		ttl:       leaseTTL,
		heartbeat: leaseHeartbeat,
	}
}

// SetArmed liga/desliga o gate de segurança do boot (ver Gate.armed).
func (g *Gate) SetArmed(v bool) {
	if g == nil {
		return
	}
	g.armed.Store(v)
}

// HolderID identifica este processo nas leases.
func (g *Gate) HolderID() string {
	if g == nil {
		return ""
	}
	return g.holderID
}

// active reporta se o gate deve fazer I/O.
func (g *Gate) active() bool {
	if g == nil || g.repo == nil {
		return false
	}
	if g.enabled != nil && g.enabled() {
		return true
	}
	return g.armed.Load()
}

// Lease é o que um produtor recebe ao passar pelo gate.
//
// Dois contextos, porque os produtores têm dois regimes diferentes e misturá-los
// quebra um dos dois:
//
//   - Ctx herda o contexto do chamador. É o certo para o produtor de
//     busca/lista, cujo cron impõe timeout por automação: usar um contexto
//     independente ali faria a execução ignorar o próprio timeout.
//   - SendCtx NÃO herda o cancelamento do chamador. É o certo para os fan-outs
//     síncronos (espelho e offer-generate), que hoje enviam sob
//     context.Background() de propósito — o cancelamento do consumidor de
//     webhook não pode abortar um envio já em andamento.
//
// Os DOIS morrem quando a lease morre. É isso que impede um fan-out de
// continuar enviando pelo chip antigo depois de uma troca.
type Lease struct {
	Ctx     context.Context
	SendCtx context.Context
	Release func()
}

func noopRelease() {}

// inactiveLease preserva exatamente o comportamento histórico: contexto do
// chamador para o fluxo, Background para envio.
func inactiveLease(ctx context.Context) *Lease {
	return &Lease{Ctx: ctx, SendCtx: context.Background(), Release: noopRelease}
}

// Acquire pega a lease de dispatch de uma automação.
func (g *Gate) Acquire(ctx context.Context, userID, automationID string) (*Lease, error) {
	if !g.active() {
		return inactiveLease(ctx), nil
	}
	if userID == "" || automationID == "" {
		// Sem identidade não há como provar que a automação está fora de uma
		// operação: fail-closed.
		observability.RecordFailoverGate("error")
		return nil, fmt.Errorf("failover gate: missing user/automation id")
	}

	leaseID, err := g.repo.AcquireLease(ctx, repository.FailoverLeaseInput{
		UserID:       userID,
		AutomationID: automationID,
		HolderID:     g.holderID,
		TTL:          g.ttl,
	})
	if err != nil {
		// ErrFailoverInProgress e erro de banco terminam igual para o produtor:
		// não comece. Só a métrica e o log distinguem.
		if errors.Is(err, repository.ErrFailoverInProgress) {
			observability.RecordFailoverGate("blocked")
		} else {
			observability.RecordFailoverGate("error")
		}
		return nil, err
	}
	observability.RecordFailoverGate("acquired")

	leaseCtx, cancelLease := context.WithCancel(ctx)
	sendCtx, cancelSend := context.WithCancel(context.WithoutCancel(ctx))
	stop := make(chan struct{})

	cancelBoth := func() {
		cancelLease()
		cancelSend()
	}
	go g.heartbeatLoop(leaseCtx, leaseID, cancelBoth, stop)

	var released bool
	release := func() {
		if released {
			return
		}
		released = true
		close(stop)
		cancelBoth()
		relCtx, relCancel := context.WithTimeout(context.WithoutCancel(ctx), releaseTimeout)
		defer relCancel()
		if err := g.repo.ReleaseLease(relCtx, leaseID); err != nil {
			g.logger.Warn("failover gate: failed to release lease",
				zap.String("leaseID", leaseID), zap.Error(err))
		}
	}
	return &Lease{Ctx: leaseCtx, SendCtx: sendCtx, Release: release}, nil
}

// heartbeatLoop renova a lease até o release. Renovação perdida cancela a
// execução.
func (g *Gate) heartbeatLoop(ctx context.Context, leaseID string, cancel context.CancelFunc, stop <-chan struct{}) {
	ticker := time.NewTicker(g.heartbeat)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Contexto independente: o heartbeat não pode morrer junto com um
			// envio cancelado, senão a lease "expiraria" sozinha enquanto o
			// produtor ainda trabalha.
			hbCtx, hbCancel := context.WithTimeout(context.WithoutCancel(ctx), releaseTimeout)
			ok, err := g.repo.RenewLease(hbCtx, leaseID, g.ttl)
			hbCancel()

			if err != nil || !ok {
				// Alerta: perder a lease no meio de um fan-out significa que o
				// produtor parou de enviar. Sem contagem/rastro, isso vira
				// "sumiram mensagens" sem causa aparente. Nunca leva snapshot
				// nem identificador de usuário — só o id da lease e o motivo.
				observability.RecordFailoverGate("lease_lost")
				observability.LogAndCapture(g.logger, sentry.LevelWarning,
					"failover gate: lease lost, cancelling dispatch",
					err, map[string]string{
						"module":  "failover",
						"flow":    "gate_heartbeat",
						"renewed": strconv.FormatBool(ok),
					})
				cancel()
				return
			}
		}
	}
}
