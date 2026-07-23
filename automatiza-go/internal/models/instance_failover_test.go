package models

import (
	"testing"
)

func strp(s string) *string { return &s }

// automação bem-formada, multi-chip, com override.
func baseAutomation() *Automation {
	t := AutomationTypeOfferGenerate
	return &Automation{
		ID:              "11111111-1111-1111-1111-111111111111",
		UserID:          "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
		Name:            "auto",
		Status:          AutomationStatusActive,
		Type:            &t,
		SendGroups:      []string{"g2@g.us", "g1@g.us"},
		MonitorGroupIDs: []string{"src@g.us"},
		InstanceID:      strp("10"),
		InstanceIDs:     []string{"10", "20"},
		ChipOverrides:   map[string]string{"g1@g.us": "20"},
	}
}

func TestNewRoutingSnapshot_ValidConfig(t *testing.T) {
	snap, reasons := NewRoutingSnapshot(baseAutomation())
	if len(reasons) != 0 {
		t.Fatalf("expected no reasons, got %v", reasons)
	}
	if snap.InstanceID == nil || *snap.InstanceID != 10 {
		t.Fatalf("scalar not normalized: %v", snap.InstanceID)
	}
	if len(snap.InstanceIDs) != 2 || snap.InstanceIDs[0] != 10 || snap.InstanceIDs[1] != 20 {
		t.Fatalf("list order must be preserved (it is semantic): %v", snap.InstanceIDs)
	}
	// Grupos são CONJUNTO: normalizados em ordem estável.
	if snap.SendGroups[0] != "g1@g.us" || snap.SendGroups[1] != "g2@g.us" {
		t.Fatalf("group set should be sorted: %v", snap.SendGroups)
	}
	if snap.ChipOverrides["g1@g.us"] != 20 {
		t.Fatalf("override not normalized: %v", snap.ChipOverrides)
	}
}

// 13.3.6 — automação com roteamento implícito bloqueia.
func TestNewRoutingSnapshot_ImplicitRoutingBlocks(t *testing.T) {
	a := baseAutomation()
	a.InstanceID = nil
	a.InstanceIDs = nil
	a.ChipOverrides = nil

	_, reasons := NewRoutingSnapshot(a)
	if !hasReason(reasons, ReasonUnmanagedImplicit) {
		t.Fatalf("implicit routing must block, got %v", reasons)
	}
}

// 13.3.3 — configuração inconsistente bloqueia em vez de autocorrigir.
func TestNewRoutingSnapshot_InconsistentConfigBlocks(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Automation)
		want   string
	}{
		{"escalar não é o primeiro da lista", func(a *Automation) { a.InstanceID = strp("20") }, ReasonScalarNotFirst},
		{"id inválido", func(a *Automation) { a.InstanceIDs = []string{"10", "abc"} }, ReasonInvalidInstanceID},
		{"id duplicado", func(a *Automation) { a.InstanceIDs = []string{"10", "10"} }, ReasonDuplicateInstanceID},
		{"override para chip fora da lista", func(a *Automation) { a.ChipOverrides = map[string]string{"g1@g.us": "99"} }, ReasonOverrideUnknownChip},
		{"override para grupo que não é destino", func(a *Automation) { a.ChipOverrides = map[string]string{"outro@g.us": "20"} }, ReasonOverrideUnknownGroup},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := baseAutomation()
			tc.mutate(a)
			_, reasons := NewRoutingSnapshot(a)
			if !hasReason(reasons, tc.want) {
				t.Fatalf("expected reason %s, got %v", tc.want, reasons)
			}
		})
	}
}

// 13.3.1/13.3.2 — projeção substitui, deduplica com ordem estável e preserva
// IDs não relacionados.
func TestProjectSwap(t *testing.T) {
	snap, _ := NewRoutingSnapshot(baseAutomation())
	out := snap.ProjectSwap(10, 30)

	if out.InstanceID == nil || *out.InstanceID != 30 {
		t.Fatalf("scalar must become the first projected element: %v", out.InstanceID)
	}
	if len(out.InstanceIDs) != 2 || out.InstanceIDs[0] != 30 || out.InstanceIDs[1] != 20 {
		t.Fatalf("unexpected projected list: %v", out.InstanceIDs)
	}
	// 20 não é a primária: fica intacto no override.
	if out.ChipOverrides["g1@g.us"] != 20 {
		t.Fatalf("unrelated override must be preserved: %v", out.ChipOverrides)
	}
	// O original não pode ter sido mutado.
	if snap.InstanceIDs[0] != 10 {
		t.Fatalf("ProjectSwap must not mutate the source snapshot")
	}
}

func TestProjectSwap_DedupesWhenTargetAlreadyPresent(t *testing.T) {
	snap, _ := NewRoutingSnapshot(baseAutomation())
	// Trocar 10 → 20, com 20 já na lista: não pode gerar duplicata.
	out := snap.ProjectSwap(10, 20)
	if len(out.InstanceIDs) != 1 || out.InstanceIDs[0] != 20 {
		t.Fatalf("swap onto an existing chip must dedupe: %v", out.InstanceIDs)
	}
	if out.InstanceID == nil || *out.InstanceID != 20 {
		t.Fatalf("scalar must follow the deduped list: %v", out.InstanceID)
	}
}

func TestProjectSwap_RemapsOverrideOfPrimary(t *testing.T) {
	a := baseAutomation()
	a.ChipOverrides = map[string]string{"g1@g.us": "10"}
	snap, reasons := NewRoutingSnapshot(a)
	if len(reasons) != 0 {
		t.Fatalf("unexpected reasons: %v", reasons)
	}
	out := snap.ProjectSwap(10, 30)
	if out.ChipOverrides["g1@g.us"] != 30 {
		t.Fatalf("override pointing at the primary must be remapped: %v", out.ChipOverrides)
	}
}

// 13.3.4 — campos operacionais NÃO mudam o fingerprint.
//
// Este é o teste que justifica não usar updated_at como versão de configuração:
// UpdateLastRunAt/UpdateNextRunAt tocam a linha em operação normal.
func TestFingerprint_IgnoresOperationalFields(t *testing.T) {
	a := baseAutomation()
	base, _ := NewRoutingSnapshot(a)

	a.Status = AutomationStatusPaused
	a.LastRunAt = strp("2026-07-23T00:00:00Z")
	a.NextRunAt = strp("2026-07-23T01:00:00Z")
	a.UpdatedAt = "2026-07-23T02:00:00Z"
	a.Name = "outro nome"
	a.CustomMessage = strp("texto novo")

	after, _ := NewRoutingSnapshot(a)
	if base.Fingerprint() != after.Fingerprint() {
		t.Fatal("operational fields must not change the config fingerprint")
	}
}

// 13.3.5 — escopo e roteamento MUDAM o fingerprint.
func TestFingerprint_ChangesOnScopeOrRouting(t *testing.T) {
	base, _ := NewRoutingSnapshot(baseAutomation())

	cases := []struct {
		name   string
		mutate func(*Automation)
	}{
		{"novo grupo de destino", func(a *Automation) { a.SendGroups = append(a.SendGroups, "g3@g.us") }},
		{"novo grupo de origem", func(a *Automation) { a.MonitorGroupIDs = append(a.MonitorGroupIDs, "src2@g.us") }},
		{"tipo", func(a *Automation) { t := AutomationTypeSearch; a.Type = &t }},
		{"lista de chips", func(a *Automation) { a.InstanceIDs = []string{"10", "20", "30"} }},
		{"override", func(a *Automation) { a.ChipOverrides = map[string]string{"g2@g.us": "20"} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := baseAutomation()
			tc.mutate(a)
			after, _ := NewRoutingSnapshot(a)
			if base.Fingerprint() == after.Fingerprint() {
				t.Fatal("scope/routing change must change the fingerprint")
			}
		})
	}
}

// Reordenar a MESMA lista de grupos não é mudança de escopo.
func TestFingerprint_StableUnderGroupReorder(t *testing.T) {
	a := baseAutomation()
	base, _ := NewRoutingSnapshot(a)

	a.SendGroups = []string{"g1@g.us", "g2@g.us"}
	after, _ := NewRoutingSnapshot(a)
	if base.Fingerprint() != after.Fingerprint() {
		t.Fatal("group order is not semantic; reordering must not change the fingerprint")
	}
}

// A ordem de instance_ids É semântica (instance_id espelha o primeiro).
func TestFingerprint_SensitiveToChipListOrder(t *testing.T) {
	a := baseAutomation()
	base, _ := NewRoutingSnapshot(a)

	a.InstanceID = strp("20")
	a.InstanceIDs = []string{"20", "10"}
	after, _ := NewRoutingSnapshot(a)
	if base.Fingerprint() == after.Fingerprint() {
		t.Fatal("chip list order is semantic; reordering must change the fingerprint")
	}
}

func TestRoutingSnapshotRoundTrip(t *testing.T) {
	snap, _ := NewRoutingSnapshot(baseAutomation())
	raw, err := snap.MarshalSnapshot()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	back, err := UnmarshalRoutingSnapshot(raw)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if back.Fingerprint() != snap.Fingerprint() {
		t.Fatal("round trip must preserve the fingerprint")
	}
}

func TestToRoutingUpdate(t *testing.T) {
	snap, _ := NewRoutingSnapshot(baseAutomation())
	upd := snap.ToRoutingUpdate()
	if upd.InstanceID == nil || *upd.InstanceID != "10" {
		t.Fatalf("scalar: %v", upd.InstanceID)
	}
	if len(upd.InstanceIDs) != 2 || upd.InstanceIDs[0] != "10" {
		t.Fatalf("list: %v", upd.InstanceIDs)
	}
	if upd.ChipOverrides["g1@g.us"] != "20" {
		t.Fatalf("overrides: %v", upd.ChipOverrides)
	}
}

func TestValidIdempotencyKey(t *testing.T) {
	cases := map[string]bool{
		"":                                 false,
		"curta":                            false,
		"chave-idempotente-valida":         true,
		"chave-com-\x01-controle-invalida": false,
	}
	for key, want := range cases {
		if got := ValidIdempotencyKey(key); got != want {
			t.Fatalf("ValidIdempotencyKey(%q) = %v, want %v", key, got, want)
		}
	}
	long := make([]byte, 129)
	for i := range long {
		long[i] = 'a'
	}
	if ValidIdempotencyKey(string(long)) {
		t.Fatal("keys longer than 128 bytes must be rejected")
	}
}

func TestDryRunRef_StableUnderItemOrder(t *testing.T) {
	items := []DryRunRefItem{
		{AutomationID: "b", Before: "1", After: "2"},
		{AutomationID: "a", Before: "3", After: "4"},
	}
	reversed := []DryRunRefItem{items[1], items[0]}

	if DryRunRef(1, 2, FailoverDirectionFailover, items, "cov") !=
		DryRunRef(1, 2, FailoverDirectionFailover, reversed, "cov") {
		t.Fatal("dry run ref must not depend on item order")
	}
	if DryRunRef(1, 2, FailoverDirectionFailover, items, "cov") ==
		DryRunRef(1, 3, FailoverDirectionFailover, items, "cov") {
		t.Fatal("policy version must be part of the ref")
	}
	if DryRunRef(1, 2, FailoverDirectionFailover, items, "cov") ==
		DryRunRef(1, 2, FailoverDirectionFailover, items, "outra") {
		t.Fatal("coverage hash must be part of the ref")
	}
}

func TestSourceGroups_OnlyForEventDrivenTypes(t *testing.T) {
	a := baseAutomation()
	snap, _ := NewRoutingSnapshot(a)
	if len(snap.SourceGroups()) != 1 {
		t.Fatalf("offer_generate must expose monitor groups: %v", snap.SourceGroups())
	}

	search := AutomationTypeSearch
	a.Type = &search
	snap, _ = NewRoutingSnapshot(a)
	if len(snap.SourceGroups()) != 0 {
		t.Fatalf("search mode has no source groups: %v", snap.SourceGroups())
	}
}

func hasReason(reasons []string, want string) bool {
	for _, r := range reasons {
		if r == want {
			return true
		}
	}
	return false
}
