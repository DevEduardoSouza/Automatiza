// Package config provides application configuration.
// This file implements feature flags for gradual migration control.
package config

import (
	"os"
	"strconv"
	"sync"

	"go.uber.org/zap"
)

// FeatureFlags holds all feature flag configurations for the application.
// These flags enable gradual migration from Node.js to Go and provide
// rollback capability if issues are detected.
type FeatureFlags struct {
	// UseGoDispatchLogs controls whether Go creates dispatch_logs in PostgreSQL.
	// When true (default), the Go backend creates dispatch logs.
	// When false, dispatch log creation is skipped.
	// Environment variable: FEATURE_USE_GO_DISPATCH_LOGS
	UseGoDispatchLogs bool

	// UseGoReconciliation controls whether Go performs message reconciliation.
	// When true, the Go reconciliation service creates dispatch logs for missed messages.
	// When false, reconciliation dispatch log creation is skipped.
	// Environment variable: FEATURE_USE_GO_RECONCILIATION
	UseGoReconciliation bool

	// UseMultiInstanceDispatch controls per-group routing in the dispatch layer.
	// When true, each message is sent through the WhatsApp instance that owns the
	// destination group (groups.instance_id), falling back to the user's first
	// connected instance only when the group's own chip is disconnected.
	// When false (default), legacy behavior is preserved: every message goes
	// through the user's first connected instance regardless of group ownership.
	// Defaulting to false provides a rollback path during the multi-instance
	// rollout.
	// Environment variable: FEATURE_USE_MULTI_INSTANCE_DISPATCH
	UseMultiInstanceDispatch bool

	// EnforceGroupLimits gates the per-chip group quota check inside an
	// automation's send_groups / monitor_group_ids lists. When true, the
	// handler rejects payloads where a single chip would host more than
	// quota.MaxGroupsPerInstance groups. When false (default), the check is
	// skipped and the user may pile any number of groups onto one chip — the
	// frontend is expected to honor the same flag so the UI matches.
	// Flip to true to re-enable the WhatsApp-ban-protection cap.
	// Environment variable: FEATURE_ENFORCE_GROUP_LIMITS
	EnforceGroupLimits bool

	// EnforcePlanGroupLimits gates the per-PLAN total group cap: a user may only
	// target up to their plan's group_limit distinct groups across all their
	// active automations (send_groups). When true, Create/Update automation is
	// rejected if it would push the distinct-group total over the limit. When
	// false (default), no plan limit is enforced — existing users are unaffected
	// (grandfathered), so this can ship dark and be flipped on after validation.
	// Environment variable: FEATURE_ENFORCE_PLAN_GROUP_LIMITS
	EnforcePlanGroupLimits bool

	// EnforceKiwifyStrictSignature, when true, REJECTS Kiwify webhooks whose
	// HMAC-SHA1 signature (over the raw body, keyed by KIWIFY_WEBHOOK_TOKEN)
	// does not match. When false (default), the signature is only logged
	// (observe). Kiwify signs with HMAC-SHA1 (confirmed from real purchase +
	// refund webhooks). Ship dark, flip on after confirming legit webhooks pass.
	// Environment variable: FEATURE_KIWIFY_STRICT_SIGNATURE
	EnforceKiwifyStrictSignature bool
}

var (
	// globalFeatureFlags is the singleton instance of feature flags
	globalFeatureFlags *FeatureFlags
	featureFlagsOnce   sync.Once
	featureFlagsLogger *zap.Logger
)

// SetFeatureFlagsLogger sets the logger for feature flag operations.
// Should be called early in application startup.
func SetFeatureFlagsLogger(logger *zap.Logger) {
	featureFlagsLogger = logger
}

// LoadFeatureFlags loads feature flags from environment variables.
// This function is safe to call multiple times; it will only load once.
// Subsequent calls return the cached instance.
func LoadFeatureFlags() *FeatureFlags {
	featureFlagsOnce.Do(func() {
		globalFeatureFlags = &FeatureFlags{
			UseGoDispatchLogs:            getEnvBool("FEATURE_USE_GO_DISPATCH_LOGS", true),
			UseGoReconciliation:          getEnvBool("FEATURE_USE_GO_RECONCILIATION", true),
			UseMultiInstanceDispatch:     getEnvBool("FEATURE_USE_MULTI_INSTANCE_DISPATCH", false),
			EnforceGroupLimits:           getEnvBool("FEATURE_ENFORCE_GROUP_LIMITS", false),
			EnforcePlanGroupLimits:       getEnvBool("FEATURE_ENFORCE_PLAN_GROUP_LIMITS", false),
			EnforceKiwifyStrictSignature: getEnvBool("FEATURE_KIWIFY_STRICT_SIGNATURE", false),
		}

		// Log feature flag values on load
		if featureFlagsLogger != nil {
			featureFlagsLogger.Info("Feature flags loaded",
				zap.Bool("useGoDispatchLogs", globalFeatureFlags.UseGoDispatchLogs),
				zap.Bool("useGoReconciliation", globalFeatureFlags.UseGoReconciliation),
				zap.Bool("useMultiInstanceDispatch", globalFeatureFlags.UseMultiInstanceDispatch),
				zap.Bool("enforceGroupLimits", globalFeatureFlags.EnforceGroupLimits),
				zap.Bool("enforcePlanGroupLimits", globalFeatureFlags.EnforcePlanGroupLimits),
				zap.Bool("enforceKiwifyStrictSignature", globalFeatureFlags.EnforceKiwifyStrictSignature))
		}
	})

	return globalFeatureFlags
}

// GetFeatureFlags returns the current feature flags instance.
// Delega sempre ao LoadFeatureFlags: o sync.Once dá o happens-before correto —
// a leitura direta de globalFeatureFlags sem sincronização era um data race
// com o init lazy (a primeira chamada acontece já com 40 goroutines do worker).
func GetFeatureFlags() *FeatureFlags {
	return LoadFeatureFlags()
}

// ShouldCreateDispatchLog returns true if Go should create dispatch logs.
// This is a convenience method that checks the UseGoDispatchLogs flag.
func (f *FeatureFlags) ShouldCreateDispatchLog() bool {
	return f.UseGoDispatchLogs
}

// ShouldReconcile returns true if Go should perform reconciliation.
// This is a convenience method that checks the UseGoReconciliation flag.
func (f *FeatureFlags) ShouldReconcile() bool {
	return f.UseGoReconciliation
}

// logDedupeSkipsFlag: kill-switch do corte de dispatch_logs de dedupe.
// Default FALSE = skips de dedupe NÃO viram linha (eram 96% dos inserts,
// ~63/s; ninguém os consome além da listagem do usuário, e a informação é
// derivável da linha 'sent' anterior). LOG_DEDUPE_SKIPS=true restaura o
// comportamento antigo sem deploy (rollback: gh variable + bounce).
var logDedupeSkipsFlag = sync.OnceValue(func() bool {
	return getEnvBool("LOG_DEDUPE_SKIPS", false)
})

// LogDedupeSkips reports whether dedupe skips should still be written to
// dispatch_logs. See logDedupeSkipsFlag.
func LogDedupeSkips() bool {
	return logDedupeSkipsFlag()
}

// webhookLeanPersistFlag: caminho enxuto de persistência do consumer de
// webhooks. Default FALSE = comportamento clássico (Exists + InsertIfNotExists
// + MarkProcessing + MarkProcessed, 4 statements/evento no caminho feliz —
// ~5,3M calls/3,7d cada no pg_stat_statements). TRUE = 2 statements: o INSERT
// do handler + MarkProcessedEnsure (upsert que finaliza e cobre linha
// ausente); MarkProcessing morre (provado não-lock e sem leitores). Rollout
// sem deploy: gh variable set WEBHOOK_LEAN_PERSIST + bounce (a var precisa
// existir no heredoc do deploy.yml — o .env de produção é allowlist).
var webhookLeanPersistFlag = sync.OnceValue(loadWebhookLeanPersist)

func loadWebhookLeanPersist() bool {
	return getEnvBool("WEBHOOK_LEAN_PERSIST", false)
}

// WebhookLeanPersist reports whether the webhook consumer should use the
// 2-statement lean persistence path. See webhookLeanPersistFlag.
func WebhookLeanPersist() bool {
	return webhookLeanPersistFlag()
}

// groupMemberTrackingFlag: gate da fundação histórica de membros de grupo
// (Melhoria 2, Fase A). Default FALSE = ZERO escrita nova, zero chamada WAHA de
// reconciliação e zero efeito de negócio: o processor devolve nil antes de
// parse/tenant/banco, e reconciliação e cleanup retornam antes de qualquer I/O.
//
// Atenção ao que a flag NÃO cobre: ligar group.v2.participants na config das
// sessões WAHA aumenta o tráfego de webhook independentemente dela — os
// envelopes novos ainda atravessam handler/fila e terminam em no-op aqui.
var groupMemberTrackingFlag = sync.OnceValue(loadGroupMemberTracking)

func loadGroupMemberTracking() bool {
	return getEnvBool("GROUP_MEMBER_TRACKING_ENABLED", false)
}

// GroupMemberTrackingEnabled reporta se o tracking histórico de membros está
// ligado. Ver groupMemberTrackingFlag.
func GroupMemberTrackingEnabled() bool {
	return groupMemberTrackingFlag()
}

// instanceFailoverFlag: gate da Melhoria 5 (chip reserva e failover de
// instância). Default FALSE = feature inerte: o handler ConnectRPC continua
// registrado, mas toda RPC responde FailedPrecondition/INSTANCE_FAILOVER_DISABLED;
// detector e recovery não fazem I/O; e o DispatchGate é no-op SEM tocar o banco,
// então o caminho quente de envio não paga nada.
//
// Uma coisa a flag deliberadamente NÃO faz: abandonar uma operação em curso.
// Se o boot encontra operação não terminal, o gate é armado mesmo com a flag
// desligada — desligar a flag no meio de uma troca abriria o gate exatamente
// quando ele mais precisa estar fechado. Transição de flag só é segura com zero
// operações não terminais.
//
// Desligar a flag também não reverte automações que já estão na reserva: voltar
// exige reativar, rodar dry-run de reversão e confirmar. Nunca editar o banco
// na mão.
var instanceFailoverFlag = sync.OnceValue(loadInstanceFailover)

func loadInstanceFailover() bool {
	return getEnvBool("FEATURE_INSTANCE_FAILOVER", false)
}

// InstanceFailoverEnabled reporta se a Melhoria 5 está ligada.
func InstanceFailoverEnabled() bool {
	return instanceFailoverFlag()
}

// monitorDispatchCadenceFlag: gate da Melhoria 7 (intervalo mínimo entre
// mensagens espelhadas). Default FALSE = feature INERTE no caminho quente: a
// admissão retorna antes de qualquer query, nenhum pending nasce e o espelho
// segue imediato como sempre foi.
//
// O que a flag NÃO faz: apagar fila já aceita. Poller, worker e manutenção
// continuam registrados justamente para DRENAR em ordem os itens admitidos
// antes do desligamento — desligar a flag é rollback gracioso, não um botão de
// descarte. Enquanto houver fila drenando com a flag off, mensagens novas
// seguem pelo caminho imediato (podem ultrapassar a fila em dreno); é o custo
// aceito para o caminho quente não pagar nada com a feature desligada.
var monitorDispatchCadenceFlag = sync.OnceValue(loadMonitorDispatchCadence)

func loadMonitorDispatchCadence() bool {
	return getEnvBool("FEATURE_MONITOR_DISPATCH_CADENCE", false)
}

// MonitorDispatchCadenceEnabled reporta se a cadência do espelho está ligada.
func MonitorDispatchCadenceEnabled() bool {
	return monitorDispatchCadenceFlag()
}

// getEnvBool reads a boolean value from an environment variable.
// Returns the default value if the variable is not set or cannot be parsed.
// Accepted values: "true", "false", "1", "0", "yes", "no" (case-insensitive)
func getEnvBool(key string, defaultVal bool) bool {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}

	b, err := strconv.ParseBool(val)
	if err != nil {
		// Log warning if logger is available
		if featureFlagsLogger != nil {
			featureFlagsLogger.Warn("Invalid boolean value for feature flag, using default",
				zap.String("key", key),
				zap.String("value", val),
				zap.Bool("default", defaultVal))
		}
		return defaultVal
	}

	return b
}

// ResetFeatureFlags resets the global feature flags instance.
// This is primarily useful for testing.
func ResetFeatureFlags() {
	globalFeatureFlags = nil
	featureFlagsOnce = sync.Once{}
	// Flags standalone (sync.OnceValue) também re-leem o ambiente no próximo
	// acesso — necessário para testes cobrirem os dois lados da flag.
	webhookLeanPersistFlag = sync.OnceValue(loadWebhookLeanPersist)
	groupMemberTrackingFlag = sync.OnceValue(loadGroupMemberTracking)
	instanceFailoverFlag = sync.OnceValue(loadInstanceFailover)
	monitorDispatchCadenceFlag = sync.OnceValue(loadMonitorDispatchCadence)
}
