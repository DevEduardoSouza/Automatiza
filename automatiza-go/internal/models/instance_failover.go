package models

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// =============================================================================
// Melhoria 5 — chip reserva e failover de instância WhatsApp.
//
// Este arquivo é a parte PURA do domínio: estados, reason codes, normalização
// de roteamento e fingerprint. Nada aqui faz I/O, e é de propósito: a projeção
// e o fingerprint precisam ser reproduzíveis byte a byte entre o dry-run e a
// transação de aplicação, que rodam em momentos (e possivelmente réplicas)
// diferentes.
// =============================================================================

// Modos da política. NÃO existe 'automatic' — nem aqui, nem no wire, nem no
// CHECK do banco.
const (
	FailoverModeManual   = "manual"
	FailoverModeAssisted = "assisted"
)

// Estados da política.
const (
	// FailoverStateActive: primária vigente, política armada.
	FailoverStateActive = "active"
	// FailoverStateProposed: evidência assisted aguardando humano.
	FailoverStateProposed = "proposed"
	// FailoverStateSwitching: gate fechado, failover não terminal.
	FailoverStateSwitching = "switching"
	// FailoverStateActiveOnBackup: roteamento trocado para a reserva.
	FailoverStateActiveOnBackup = "active_on_backup"
	// FailoverStateReverting: gate fechado, reversão não terminal.
	FailoverStateReverting = "reverting"
	// FailoverStateFailed: inconsistência que não admite inferência segura.
	FailoverStateFailed = "failed"
)

// Direções.
const (
	FailoverDirectionFailover = "failover"
	FailoverDirectionRevert   = "revert"
)

// Status de operação.
const (
	FailoverOpPreparing = "preparing"
	FailoverOpDraining  = "draining"
	FailoverOpVerifying = "verifying"
	FailoverOpCompleted = "completed"
	FailoverOpAborted   = "aborted"
	FailoverOpFailed    = "failed"
)

// Resultados de item.
const (
	FailoverItemPending     = "pending"
	FailoverItemApplied     = "applied"
	FailoverItemRestored    = "restored"
	FailoverItemDeletedNoop = "deleted_noop"
)

// Resultados de item de dry-run.
const (
	FailoverCoverageCovered     = "covered"
	FailoverCoverageBlocked     = "blocked"
	FailoverCoverageDeletedNoop = "deleted_noop"
)

// Status de dry-run.
const (
	FailoverDryRunPass    = "pass"
	FailoverDryRunBlocked = "blocked"
)

// Tipos de evento do ledger (conjunto fechado, espelha o CHECK da migração 077).
const (
	FailoverEventPolicyCreated      = "policy_created"
	FailoverEventPolicyUpdated      = "policy_updated"
	FailoverEventPolicyDisabled     = "policy_disabled"
	FailoverEventDryRunPassed       = "dry_run_passed"
	FailoverEventDryRunBlocked      = "dry_run_blocked"
	FailoverEventProposalCreated    = "proposal_created"
	FailoverEventProposalRecovered  = "proposal_recovered"
	FailoverEventProposalExpired    = "proposal_expired"
	FailoverEventSwitchRequested    = "switch_requested"
	FailoverEventSwitchGateClosed   = "switch_gate_closed"
	FailoverEventSwitchDrainTimeout = "switch_drain_timeout"
	FailoverEventSwitchApplied      = "switch_applied"
	FailoverEventSwitchCompleted    = "switch_completed"
	FailoverEventSwitchAborted      = "switch_aborted"
	FailoverEventSwitchFailed       = "switch_failed"
	FailoverEventRevertRequested    = "revert_requested"
	FailoverEventRevertGateClosed   = "revert_gate_closed"
	FailoverEventRevertDrainTimeout = "revert_drain_timeout"
	FailoverEventRevertApplied      = "revert_applied"
	FailoverEventRevertCompleted    = "revert_completed"
	FailoverEventRevertAborted      = "revert_aborted"
	FailoverEventRevertFailed       = "revert_failed"
	FailoverEventRecoveryCompleted  = "recovery_completed"
	FailoverEventRecoveryAborted    = "recovery_aborted"
	FailoverEventRecoveryFailed     = "recovery_failed"
)

// Escopos de idempotência do ledger.
const (
	FailoverScopeSetPolicy     = "set_policy"
	FailoverScopeDisablePolicy = "disable_policy"
)

// Tipos de ator.
const (
	FailoverActorUser   = "user"
	FailoverActorCron   = "cron"
	FailoverActorSystem = "system"
)

// Reason codes — vocabulário FECHADO. São o que a UI mostra e o que as métricas
// agrupam, então nunca podem virar texto livre de erro do provider (que
// carregaria PII e explodiria a cardinalidade das labels).
const (
	// Configuração da automação.
	ReasonUnmanagedImplicit    = "unmanaged_implicit"
	ReasonInvalidInstanceID    = "invalid_instance_id"
	ReasonInstanceNotOwned     = "instance_not_owned"
	ReasonScalarNotFirst       = "scalar_not_first_in_list"
	ReasonDuplicateInstanceID  = "duplicate_instance_id"
	ReasonOverrideUnknownChip  = "override_unknown_instance"
	ReasonOverrideUnknownGroup = "override_unknown_group"

	// Chip alvo.
	ReasonTargetNotWorking    = "target_session_not_working"
	ReasonReserveIdentity     = "reserve_identity_unknown"
	ReasonProviderUnavailable = "waha_unavailable"

	// Cobertura.
	ReasonGroupMissing     = "group_missing"
	ReasonAdminMissing     = "admin_missing"
	ReasonCapacityExceeded = "capacity_exceeded"

	// Concorrência / consistência.
	ReasonStaleConfiguration = "stale_configuration"
	ReasonRoutingDiverged    = "routing_diverged"
	ReasonSetChanged         = "eligible_set_changed"
	ReasonDrainTimeout       = "drain_timeout"
	ReasonPostcheckFailed    = "postcheck_failed"
	ReasonIndeterminate      = "indeterminate_state"
)

// =============================================================================
// Registros persistidos
// =============================================================================

// InstanceFailoverPolicy espelha instance_failover_policies.
type InstanceFailoverPolicy struct {
	ID                         int64      `json:"id"`
	UserID                     string     `json:"user_id"`
	PrimaryInstanceID          int        `json:"primary_instance_id"`
	BackupInstanceID           int        `json:"backup_instance_id"`
	Mode                       string     `json:"mode"`
	State                      string     `json:"state"`
	Version                    int64      `json:"version"`
	CurrentOperationID         *string    `json:"current_operation_id,omitempty"`
	UnhealthyAfterSeconds      int        `json:"unhealthy_after_seconds"`
	RequiredConsecutiveProbes  int        `json:"required_consecutive_probes"`
	DrainDeadlineSeconds       int        `json:"drain_deadline_seconds"`
	ProposalTTLSeconds         int        `json:"proposal_ttl_seconds"`
	PrimaryUnhealthySince      *time.Time `json:"primary_unhealthy_since,omitempty"`
	ConsecutiveUnhealthyProbes int        `json:"consecutive_unhealthy_probes"`
	LastProbeAt                *time.Time `json:"last_probe_at,omitempty"`
	LastProbeStatus            *string    `json:"last_probe_status,omitempty"`
	ProposalExpiresAt          *time.Time `json:"proposal_expires_at,omitempty"`
	CreatedAt                  time.Time  `json:"created_at"`
	UpdatedAt                  time.Time  `json:"updated_at"`
}

// IsStable reporta se a política está num estado sem operação em curso.
func (p *InstanceFailoverPolicy) IsStable() bool {
	switch p.State {
	case FailoverStateActive, FailoverStateProposed, FailoverStateActiveOnBackup:
		return true
	}
	return false
}

// SourceInstanceID é o chip que serve os grupos AGORA, segundo a política.
func (p *InstanceFailoverPolicy) SourceInstanceID() int {
	if p.State == FailoverStateActiveOnBackup || p.State == FailoverStateReverting {
		return p.BackupInstanceID
	}
	return p.PrimaryInstanceID
}

// InstanceFailoverDryRun espelha instance_failover_dry_runs.
type InstanceFailoverDryRun struct {
	ID               string    `json:"id"`
	PolicyID         *int64    `json:"policy_id,omitempty"`
	PolicyIDSnapshot int64     `json:"policy_id_snapshot"`
	UserID           string    `json:"user_id"`
	Direction        string    `json:"direction"`
	IdempotencyKey   string    `json:"idempotency_key"`
	RequestHash      string    `json:"request_hash"`
	PolicyVersion    int64     `json:"policy_version"`
	DryRunRef        string    `json:"dry_run_ref"`
	Status           string    `json:"status"`
	Report           []byte    `json:"report"`
	CreatedAt        time.Time `json:"created_at"`
	ExpiresAt        time.Time `json:"expires_at"`
}

// InstanceFailoverDryRunItem espelha instance_failover_dry_run_items.
type InstanceFailoverDryRunItem struct {
	DryRunID             string   `json:"dry_run_id"`
	AutomationID         *string  `json:"automation_id,omitempty"`
	AutomationIDSnapshot string   `json:"automation_id_snapshot"`
	ConfigFingerprint    string   `json:"config_fingerprint"`
	SnapshotBefore       []byte   `json:"snapshot_before"`
	SnapshotAfter        []byte   `json:"snapshot_after"`
	RequiredSourceGroups []string `json:"required_source_groups"`
	RequiredTargetGroups []string `json:"required_target_groups"`
	AdminRequiredGroups  []string `json:"admin_required_groups"`
	MissingGroups        []string `json:"missing_groups"`
	MissingAdminGroups   []string `json:"missing_admin_groups"`
	Result               string   `json:"result"`
	ReasonCodes          []string `json:"reason_codes"`
	// AutomationName não é persistido; é preenchido para a resposta da API.
	AutomationName string `json:"automation_name,omitempty"`
}

// InstanceFailoverOperation espelha instance_failover_operations.
type InstanceFailoverOperation struct {
	ID                 string     `json:"id"`
	PolicyID           *int64     `json:"policy_id,omitempty"`
	PolicyIDSnapshot   int64      `json:"policy_id_snapshot"`
	UserID             string     `json:"user_id"`
	Kind               string     `json:"kind"`
	Status             string     `json:"status"`
	ReturnPolicyState  string     `json:"return_policy_state"`
	IdempotencyKey     string     `json:"idempotency_key"`
	RequestHash        string     `json:"request_hash"`
	DryRunID           string     `json:"dry_run_id"`
	SourceOperationID  *string    `json:"source_operation_id,omitempty"`
	StartPolicyVersion int64      `json:"start_policy_version"`
	GateClosedAt       *time.Time `json:"gate_closed_at,omitempty"`
	DrainDeadlineAt    *time.Time `json:"drain_deadline_at,omitempty"`
	AppliedAt          *time.Time `json:"applied_at,omitempty"`
	CompletedAt        *time.Time `json:"completed_at,omitempty"`
	Report             []byte     `json:"report"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

// IsTerminal reporta se a operação já chegou a um desfecho.
func (o *InstanceFailoverOperation) IsTerminal() bool {
	switch o.Status {
	case FailoverOpCompleted, FailoverOpAborted, FailoverOpFailed:
		return true
	}
	return false
}

// InstanceFailoverOperationItem espelha instance_failover_operation_items.
type InstanceFailoverOperationItem struct {
	OperationID          string  `json:"operation_id"`
	AutomationID         *string `json:"automation_id,omitempty"`
	AutomationIDSnapshot string  `json:"automation_id_snapshot"`
	FingerprintBefore    string  `json:"fingerprint_before"`
	FingerprintAfter     string  `json:"fingerprint_after"`
	SnapshotBefore       []byte  `json:"snapshot_before"`
	SnapshotAfter        []byte  `json:"snapshot_after"`
	Result               string  `json:"result"`
}

// InstanceFailoverEvent espelha instance_failover_events.
type InstanceFailoverEvent struct {
	ID               int64     `json:"id"`
	PolicyID         *int64    `json:"policy_id,omitempty"`
	PolicyIDSnapshot int64     `json:"policy_id_snapshot"`
	OperationID      *string   `json:"operation_id,omitempty"`
	DryRunID         *string   `json:"dry_run_id,omitempty"`
	UserID           string    `json:"user_id"`
	EventType        string    `json:"event_type"`
	ActorType        string    `json:"actor_type"`
	ActorUserID      *string   `json:"actor_user_id,omitempty"`
	IdempotencyScope *string   `json:"idempotency_scope,omitempty"`
	IdempotencyKey   *string   `json:"idempotency_key,omitempty"`
	RequestHash      *string   `json:"request_hash,omitempty"`
	PolicyVersion    *int64    `json:"policy_version,omitempty"`
	SnapshotBefore   []byte    `json:"snapshot_before,omitempty"`
	SnapshotAfter    []byte    `json:"snapshot_after,omitempty"`
	Report           []byte    `json:"report"`
	CreatedAt        time.Time `json:"created_at"`
}

// =============================================================================
// Snapshot de roteamento e fingerprint
// =============================================================================

// RoutingSnapshot é a fotografia NORMALIZADA dos campos que decidem por qual
// chip uma automação envia, mais o escopo que decide quais grupos ela precisa
// cobrir.
//
// Só os três campos de roteamento (InstanceID, InstanceIDs, ChipOverrides) são
// escritos de volta numa aplicação/restauração. Os campos de escopo entram no
// snapshot para auditoria e para a reversão conferir que o alvo não mudou de
// forma por baixo.
type RoutingSnapshot struct {
	AutomationID string `json:"automation_id"`
	UserID       string `json:"user_id"`
	Type         string `json:"type"`

	// Escopo (não é restaurado, é conferido).
	GroupIDs        []string `json:"group_ids"`
	SendGroups      []string `json:"send_groups"`
	MonitorGroupIDs []string `json:"monitor_group_ids"`

	// Roteamento (é o que se aplica/restaura).
	InstanceID    *int           `json:"instance_id"`
	InstanceIDs   []int          `json:"instance_ids"`
	ChipOverrides map[string]int `json:"chip_overrides"`
}

// normalizeGroupSet devolve o conjunto de grupos ordenado e sem vazios.
//
// A ordem de group_ids/send_groups/monitor_group_ids NÃO é semântica para
// roteamento — o que muda escopo é o CONJUNTO. Ordenar evita que reordenar a
// lista na UI invalide um dry-run em andamento por "mudança de configuração"
// que não mudou nada.
func normalizeGroupSet(in []string) []string {
	if len(in) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, g := range in {
		g = strings.TrimSpace(g)
		if g == "" {
			continue
		}
		if _, dup := seen[g]; dup {
			continue
		}
		seen[g] = struct{}{}
		out = append(out, g)
	}
	sort.Strings(out)
	return out
}

// parsePositiveID converte um ID de instância em forma de string para int.
// Vazio devolve (0,false) sem erro; qualquer outro valor não-inteiro-positivo é
// erro — o failover NUNCA conserta configuração silenciosamente.
func parsePositiveID(raw string) (int, bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return 0, false, fmt.Errorf("%s: %q", ReasonInvalidInstanceID, raw)
	}
	return v, true, nil
}

// NewRoutingSnapshot normaliza e VALIDA o roteamento de uma automação.
//
// Devolve reason codes (não erro) para toda condição que deve bloquear o
// dry-run da conta: eles vão para o relatório e para a UI. Erro de verdade é
// reservado para falha de serialização.
func NewRoutingSnapshot(a *Automation) (*RoutingSnapshot, []string) {
	var reasons []string

	snap := &RoutingSnapshot{
		AutomationID:    a.ID,
		UserID:          a.UserID,
		Type:            a.GetType(),
		GroupIDs:        normalizeGroupSet(a.GroupIDs),
		SendGroups:      normalizeGroupSet(a.SendGroups),
		MonitorGroupIDs: normalizeGroupSet(a.MonitorGroupIDs),
		InstanceIDs:     []int{},
		ChipOverrides:   map[string]int{},
	}

	// Escalar.
	if a.InstanceID != nil {
		v, ok, err := parsePositiveID(*a.InstanceID)
		if err != nil {
			reasons = append(reasons, ReasonInvalidInstanceID)
		} else if ok {
			snap.InstanceID = &v
		}
	}

	// Lista multi-chip. Duplicata é erro de configuração, não algo a deduplicar
	// por baixo: a lista é o que o fan-out itera.
	seen := make(map[int]struct{}, len(a.InstanceIDs))
	for _, raw := range a.InstanceIDs {
		v, ok, err := parsePositiveID(raw)
		if err != nil {
			reasons = append(reasons, ReasonInvalidInstanceID)
			continue
		}
		if !ok {
			continue
		}
		if _, dup := seen[v]; dup {
			reasons = append(reasons, ReasonDuplicateInstanceID)
			continue
		}
		seen[v] = struct{}{}
		snap.InstanceIDs = append(snap.InstanceIDs, v)
	}

	// Contrato da migração 049: instance_id espelha instance_ids[0].
	if len(snap.InstanceIDs) > 0 {
		if snap.InstanceID == nil || *snap.InstanceID != snap.InstanceIDs[0] {
			reasons = append(reasons, ReasonScalarNotFirst)
		}
	}

	// Overrides por grupo.
	targets := make(map[string]struct{}, len(snap.GroupIDs)+len(snap.SendGroups))
	for _, g := range snap.SendGroups {
		targets[g] = struct{}{}
	}
	if len(snap.SendGroups) == 0 {
		for _, g := range snap.GroupIDs {
			targets[g] = struct{}{}
		}
	}
	for group, raw := range a.ChipOverrides {
		group = strings.TrimSpace(group)
		if group == "" {
			reasons = append(reasons, ReasonOverrideUnknownGroup)
			continue
		}
		v, ok, err := parsePositiveID(raw)
		if err != nil || !ok {
			reasons = append(reasons, ReasonOverrideUnknownChip)
			continue
		}
		if _, inList := seen[v]; !inList {
			reasons = append(reasons, ReasonOverrideUnknownChip)
			continue
		}
		if _, isTarget := targets[group]; !isTarget {
			reasons = append(reasons, ReasonOverrideUnknownGroup)
			continue
		}
		snap.ChipOverrides[group] = v
	}

	// Roteamento implícito: sem escalar e sem lista, o dispatcher cai no
	// "primeiro chip conectado". Isso é exatamente o que faria a primária
	// continuar servindo os grupos DEPOIS da troca, fora de qualquer snapshot.
	if snap.InstanceID == nil && len(snap.InstanceIDs) == 0 {
		reasons = append(reasons, ReasonUnmanagedImplicit)
	}

	return snap, dedupeStrings(reasons)
}

// dedupeStrings devolve os códigos únicos em ordem estável (ordenada), para
// que o relatório e o fingerprint não dependam da ordem de descoberta.
func dedupeStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, dup := seen[s]; dup {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// UsesInstance reporta se o chip aparece em qualquer campo de roteamento.
func (s *RoutingSnapshot) UsesInstance(instanceID int) bool {
	if s.InstanceID != nil && *s.InstanceID == instanceID {
		return true
	}
	for _, v := range s.InstanceIDs {
		if v == instanceID {
			return true
		}
	}
	for _, v := range s.ChipOverrides {
		if v == instanceID {
			return true
		}
	}
	return false
}

// TargetGroups devolve os grupos de DESTINO, com a mesma regra do dispatcher
// (Automation.GetTargetGroups): send_groups quando não vazio, senão group_ids.
func (s *RoutingSnapshot) TargetGroups() []string {
	if len(s.SendGroups) > 0 {
		return s.SendGroups
	}
	return s.GroupIDs
}

// SourceGroups devolve os grupos de ORIGEM (só modos event-driven os têm).
func (s *RoutingSnapshot) SourceGroups() []string {
	if s.Type == AutomationTypeMonitoring || s.Type == AutomationTypeOfferGenerate {
		return s.MonitorGroupIDs
	}
	return nil
}

// ProjectSwap devolve uma CÓPIA do snapshot com toda ocorrência de `from`
// trocada por `to`.
//
// Deduplica preservando a primeira ocorrência (se o alvo já estava na lista, a
// troca não pode criar duplicata), redefine o escalar como o primeiro elemento
// projetado e remapeia os overrides. IDs não relacionados ficam intactos.
func (s *RoutingSnapshot) ProjectSwap(from, to int) *RoutingSnapshot {
	out := &RoutingSnapshot{
		AutomationID:    s.AutomationID,
		UserID:          s.UserID,
		Type:            s.Type,
		GroupIDs:        append([]string(nil), s.GroupIDs...),
		SendGroups:      append([]string(nil), s.SendGroups...),
		MonitorGroupIDs: append([]string(nil), s.MonitorGroupIDs...),
		InstanceIDs:     []int{},
		ChipOverrides:   map[string]int{},
	}

	seen := make(map[int]struct{}, len(s.InstanceIDs))
	for _, v := range s.InstanceIDs {
		if v == from {
			v = to
		}
		if _, dup := seen[v]; dup {
			continue
		}
		seen[v] = struct{}{}
		out.InstanceIDs = append(out.InstanceIDs, v)
	}

	switch {
	case len(out.InstanceIDs) > 0:
		first := out.InstanceIDs[0]
		out.InstanceID = &first
	case s.InstanceID != nil:
		v := *s.InstanceID
		if v == from {
			v = to
		}
		out.InstanceID = &v
	}

	for group, v := range s.ChipOverrides {
		if v == from {
			v = to
		}
		out.ChipOverrides[group] = v
	}

	return out
}

// RoutingUpdate é o conjunto MÍNIMO de colunas que uma aplicação/restauração
// escreve em `automations`. Nada além destas três colunas é tocado.
type RoutingUpdate struct {
	InstanceID    *string
	InstanceIDs   []string
	ChipOverrides map[string]string
}

// ToRoutingUpdate converte o snapshot na forma que o repositório escreve
// (strings, como o schema legado exige).
func (s *RoutingSnapshot) ToRoutingUpdate() RoutingUpdate {
	upd := RoutingUpdate{
		InstanceIDs:   make([]string, 0, len(s.InstanceIDs)),
		ChipOverrides: make(map[string]string, len(s.ChipOverrides)),
	}
	if s.InstanceID != nil {
		v := strconv.Itoa(*s.InstanceID)
		upd.InstanceID = &v
	}
	for _, v := range s.InstanceIDs {
		upd.InstanceIDs = append(upd.InstanceIDs, strconv.Itoa(v))
	}
	for g, v := range s.ChipOverrides {
		upd.ChipOverrides[g] = strconv.Itoa(v)
	}
	return upd
}

// Fingerprint é o SHA-256 da forma canônica do snapshot.
//
// Cobre EXCLUSIVAMENTE o que altera escopo, cobertura ou roteamento:
// id, user_id, type, group_ids, send_groups, monitor_group_ids, instance_id,
// instance_ids e chip_overrides.
//
// Fora, de propósito: status, name, conteúdo, last_run_at, next_run_at,
// contadores e timestamps. `updated_at` em particular NÃO serve como versão —
// UpdateLastRunAt/UpdateNextRunAt/BulkUpdateNextRunAts o tocam em operação
// normal (automation_repository.go), então usá-lo como optimistic lock geraria
// conflito permanente e o failover nunca conseguiria aplicar.
func (s *RoutingSnapshot) Fingerprint() string {
	var b strings.Builder
	b.WriteString("v1\n")
	b.WriteString("id=" + s.AutomationID + "\n")
	b.WriteString("user=" + s.UserID + "\n")
	b.WriteString("type=" + s.Type + "\n")
	b.WriteString("groups=" + strings.Join(s.GroupIDs, ",") + "\n")
	b.WriteString("send=" + strings.Join(s.SendGroups, ",") + "\n")
	b.WriteString("monitor=" + strings.Join(s.MonitorGroupIDs, ",") + "\n")

	b.WriteString("scalar=")
	if s.InstanceID != nil {
		b.WriteString(strconv.Itoa(*s.InstanceID))
	}
	b.WriteString("\n")

	// A ordem de instance_ids É semântica (instance_id espelha o primeiro), por
	// isso não é ordenada aqui.
	b.WriteString("list=")
	for i, v := range s.InstanceIDs {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(strconv.Itoa(v))
	}
	b.WriteString("\n")

	// Mapa: chaves em ordem lexicográfica para dar forma canônica.
	keys := make([]string, 0, len(s.ChipOverrides))
	for k := range s.ChipOverrides {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	b.WriteString("overrides=")
	for i, k := range keys {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(k + ":" + strconv.Itoa(s.ChipOverrides[k]))
	}
	b.WriteString("\n")

	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

// MarshalSnapshot serializa o snapshot para a coluna JSONB.
func (s *RoutingSnapshot) MarshalSnapshot() ([]byte, error) {
	return json.Marshal(s)
}

// UnmarshalRoutingSnapshot restaura um snapshot persistido.
func UnmarshalRoutingSnapshot(raw []byte) (*RoutingSnapshot, error) {
	var s RoutingSnapshot
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("failover: invalid routing snapshot: %w", err)
	}
	if s.InstanceIDs == nil {
		s.InstanceIDs = []int{}
	}
	if s.ChipOverrides == nil {
		s.ChipOverrides = map[string]int{}
	}
	return &s, nil
}

// HashRequest devolve o SHA-256 hexadecimal de uma forma canônica de request,
// usado para provar que um replay de idempotency key traz o MESMO payload.
func HashRequest(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x1f")))
	return hex.EncodeToString(sum[:])
}

// DryRunRef é a impressão digital de um ensaio: política, versão, direção e o
// conjunto ordenado de (automação, fingerprint antes, fingerprint depois), mais
// o hash do relatório de cobertura. A confirmação só é aceita com esta ref
// exata — é o que impede confirmar um ensaio que já não descreve a realidade.
func DryRunRef(policyID int64, policyVersion int64, direction string, items []DryRunRefItem, coverageHash string) string {
	ordered := append([]DryRunRefItem(nil), items...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].AutomationID < ordered[j].AutomationID })

	var b strings.Builder
	b.WriteString("v1\n")
	b.WriteString(strconv.FormatInt(policyID, 10) + "\n")
	b.WriteString(strconv.FormatInt(policyVersion, 10) + "\n")
	b.WriteString(direction + "\n")
	for _, it := range ordered {
		b.WriteString(it.AutomationID + ":" + it.Before + ":" + it.After + "\n")
	}
	b.WriteString(coverageHash + "\n")

	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

// DryRunRefItem é uma entrada do cálculo de DryRunRef.
type DryRunRefItem struct {
	AutomationID string
	Before       string
	After        string
}

// ValidIdempotencyKey aplica o mesmo contrato do CHECK da migração 077:
// 16 a 128 bytes, ASCII imprimível.
func ValidIdempotencyKey(key string) bool {
	if len(key) < 16 || len(key) > 128 {
		return false
	}
	for i := 0; i < len(key); i++ {
		if key[i] < 0x20 || key[i] > 0x7E {
			return false
		}
	}
	return true
}
