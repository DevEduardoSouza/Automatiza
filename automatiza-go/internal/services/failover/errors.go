// Package failover implementa a Melhoria 5: chip reserva e troca de roteamento
// entre duas instâncias WhatsApp do MESMO usuário.
//
// Três coisas que este pacote NUNCA faz, por decisão de projeto:
//
//   - inferir banimento. FAILED, STOPPED, timeout, desconexão e texto de erro
//     significam "indisponível", nunca "banido". Não existe modo automático.
//   - escrever no WAHA. Só GET de sessão, grupos e participantes. Nada de
//     logout/stop/restart/delete/unpair, e nem o POST /groups/refresh.
//   - tocar membership de grupo, status de instância, filas ou dispatches já
//     criados. A troca mexe em exatamente três colunas de `automations`.
package failover

import "errors"

var (
	// ErrDisabled indica FEATURE_INSTANCE_FAILOVER desligada.
	ErrDisabled = errors.New("instance failover is disabled")

	// ErrNotFound cobre política/dry-run/operação inexistente OU de outro
	// tenant — a borda responde NotFound nos dois casos, sem revelar qual.
	ErrNotFound = errors.New("failover resource not found")

	// ErrInvalidArgument cobre input malformado (enum desconhecido, chave de
	// idempotência fora do contrato, IDs fora de faixa).
	ErrInvalidArgument = errors.New("invalid failover argument")

	// ErrStateConflict indica transição ilegal para o estado atual.
	ErrStateConflict = errors.New("failover policy is not in a state that allows this operation")

	// ErrCoverageBlocked indica que o chip alvo não cobre o que precisaria.
	ErrCoverageBlocked = errors.New("failover coverage is incomplete")

	// ErrDryRunExpired indica ensaio vencido (TTL de 15 minutos).
	ErrDryRunExpired = errors.New("dry run expired")

	// ErrDryRunMismatch indica ref/versão que não correspondem ao ensaio.
	ErrDryRunMismatch = errors.New("dry run reference does not match")

	// ErrDrainTimeout indica que o trabalho em voo não drenou dentro do
	// deadline. NADA é alterado: prosseguir criaria split-brain (o job
	// congelado enviaria pelo chip antigo depois da troca).
	ErrDrainTimeout = errors.New("in-flight dispatch did not drain before the deadline")

	// ErrIndeterminate indica estado que não admite inferência segura.
	ErrIndeterminate = errors.New("failover state is indeterminate; human review required")
)

// ReasonDisabled é o reason code estável que a UI usa para esconder a seção
// inteira quando a feature está desligada. Qualquer outro erro é exibido: um
// Internal ou uma falha de rede NUNCA pode ser confundido com "desligado".
const ReasonDisabled = "INSTANCE_FAILOVER_DISABLED"
