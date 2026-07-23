package observability

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// =============================================================================
// Métricas da Melhoria 5 — chip reserva e failover de instância.
//
// Arquivo separado do metrics.go de propósito: os instrumentos daqui têm um
// ciclo de vida próprio (ligados junto com a feature) e mantê-los fora do
// struct compartilhado evita que duas frentes de trabalho disputem o mesmo
// bloco de código.
//
// REGRA DE RÓTULO, sem exceção: todo label é de conjunto FECHADO — kind,
// result, reason code. Nunca policy_id, user_id, session, grupo, telefone ou
// texto de erro do provider. Um único rótulo de alta cardinalidade aqui
// multiplicaria séries por usuário (~870) × política, e derrubaria o backend de
// métricas antes de ajudar em qualquer diagnóstico.
// =============================================================================

// FailoverMetrics agrupa os instrumentos da feature.
type FailoverMetrics struct {
	Operations      metric.Int64Counter
	OperationTime   metric.Float64Histogram
	DrainTime       metric.Float64Histogram
	CoverageBlocked metric.Int64Counter
	Proposals       metric.Int64Counter
	Recovery        metric.Int64Counter
	Gate            metric.Int64Counter
}

// GlobalFailoverMetrics é o singleton. Nil = métricas desligadas; todo call
// site é nil-safe.
var GlobalFailoverMetrics *FailoverMetrics

// InitFailoverMetrics registra os instrumentos da Melhoria 5.
// Com enabled=false o singleton fica nil e as funções Record viram no-op.
func InitFailoverMetrics(enabled bool) error {
	if !enabled {
		return nil
	}

	meter := otel.Meter("automatiza")
	m := &FailoverMetrics{}
	var err error

	if m.Operations, err = meter.Int64Counter("instance_failover_operations_total",
		metric.WithDescription("Failover/revert operations by kind and outcome"),
		metric.WithUnit("{operation}")); err != nil {
		return err
	}
	if m.OperationTime, err = meter.Float64Histogram("instance_failover_operation_duration_seconds",
		metric.WithDescription("End-to-end duration of a failover/revert operation"),
		metric.WithUnit("s")); err != nil {
		return err
	}
	if m.DrainTime, err = meter.Float64Histogram("instance_failover_drain_duration_seconds",
		metric.WithDescription("Time waiting for in-flight dispatch to drain after the gate closed"),
		metric.WithUnit("s")); err != nil {
		return err
	}
	if m.CoverageBlocked, err = meter.Int64Counter("instance_failover_coverage_blocked_total",
		metric.WithDescription("Coverage checks that blocked, by reason code"),
		metric.WithUnit("{check}")); err != nil {
		return err
	}
	if m.Proposals, err = meter.Int64Counter("instance_failover_proposals_total",
		metric.WithDescription("Assisted-mode proposals by outcome"),
		metric.WithUnit("{proposal}")); err != nil {
		return err
	}
	if m.Recovery, err = meter.Int64Counter("instance_failover_recovery_total",
		metric.WithDescription("Recovery outcomes for non-terminal operations"),
		metric.WithUnit("{recovery}")); err != nil {
		return err
	}
	if m.Gate, err = meter.Int64Counter("instance_failover_gate_total",
		metric.WithDescription("Dispatch gate acquisitions by outcome"),
		metric.WithUnit("{acquire}")); err != nil {
		return err
	}

	GlobalFailoverMetrics = m
	return nil
}

// RecordFailoverOperation conta uma operação terminal.
// kind: failover|revert. result: completed|aborted|failed.
func RecordFailoverOperation(kind, result string) {
	if GlobalFailoverMetrics == nil || GlobalFailoverMetrics.Operations == nil {
		return
	}
	GlobalFailoverMetrics.Operations.Add(context.Background(), 1,
		metric.WithAttributes(attribute.String("kind", kind), attribute.String("result", result)))
}

// RecordFailoverOperationDuration registra a duração total da operação.
func RecordFailoverOperationDuration(kind string, seconds float64) {
	if GlobalFailoverMetrics == nil || GlobalFailoverMetrics.OperationTime == nil {
		return
	}
	GlobalFailoverMetrics.OperationTime.Record(context.Background(), seconds,
		metric.WithAttributes(attribute.String("kind", kind)))
}

// RecordFailoverDrainDuration registra quanto tempo o drain levou.
//
// É a métrica que diz se drain_deadline_seconds está bem calibrado: drains
// consistentemente perto do teto significam que a próxima troca vai abortar.
func RecordFailoverDrainDuration(kind string, seconds float64) {
	if GlobalFailoverMetrics == nil || GlobalFailoverMetrics.DrainTime == nil {
		return
	}
	GlobalFailoverMetrics.DrainTime.Record(context.Background(), seconds,
		metric.WithAttributes(attribute.String("kind", kind)))
}

// RecordFailoverCoverageBlocked conta um bloqueio de cobertura por reason code.
func RecordFailoverCoverageBlocked(reason string) {
	if GlobalFailoverMetrics == nil || GlobalFailoverMetrics.CoverageBlocked == nil {
		return
	}
	GlobalFailoverMetrics.CoverageBlocked.Add(context.Background(), 1,
		metric.WithAttributes(attribute.String("reason", reason)))
}

// RecordFailoverProposal conta uma proposta do detector.
// result: created|recovered|expired.
func RecordFailoverProposal(result string) {
	if GlobalFailoverMetrics == nil || GlobalFailoverMetrics.Proposals == nil {
		return
	}
	GlobalFailoverMetrics.Proposals.Add(context.Background(), 1,
		metric.WithAttributes(attribute.String("result", result)))
}

// RecordFailoverRecovery conta um desfecho de recuperação.
// result: completed|aborted|failed.
func RecordFailoverRecovery(result string) {
	if GlobalFailoverMetrics == nil || GlobalFailoverMetrics.Recovery == nil {
		return
	}
	GlobalFailoverMetrics.Recovery.Add(context.Background(), 1,
		metric.WithAttributes(attribute.String("result", result)))
}

// RecordFailoverGate conta uma tentativa de aquisição de lease.
// result: acquired|blocked|error|released_lost.
func RecordFailoverGate(result string) {
	if GlobalFailoverMetrics == nil || GlobalFailoverMetrics.Gate == nil {
		return
	}
	GlobalFailoverMetrics.Gate.Add(context.Background(), 1,
		metric.WithAttributes(attribute.String("result", result)))
}
