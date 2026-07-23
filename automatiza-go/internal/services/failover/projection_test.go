package failover

import (
	"testing"

	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"
)

func row(id string, instanceIDs []string, targets []string) repository.FailoverAutomationRouting {
	typ := models.AutomationTypeSearch
	a := models.Automation{
		ID:         id,
		UserID:     "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
		Type:       &typ,
		SendGroups: targets,
	}
	if len(instanceIDs) > 0 {
		first := instanceIDs[0]
		a.InstanceID = &first
		a.InstanceIDs = instanceIDs
	}
	return repository.FailoverAutomationRouting{Automation: a, Name: "auto-" + id, Status: "active"}
}

// Só automações que USAM a primária entram no conjunto.
func TestBuildSwapSet_SelectsOnlyAutomationsUsingPrimary(t *testing.T) {
	rows := []repository.FailoverAutomationRouting{
		row("a", []string{"10"}, []string{"g1@g.us"}),
		row("b", []string{"20"}, []string{"g2@g.us"}),
		row("c", []string{"30", "10"}, []string{"g3@g.us"}),
	}
	set := BuildSwapSet(rows, 10, 99)

	if set.Blocked() {
		t.Fatalf("unexpected blocking reasons: %v", set.BlockingReasons)
	}
	ids := set.AutomationIDs()
	if len(ids) != 2 || ids[0] != "a" || ids[1] != "c" {
		t.Fatalf("expected [a c], got %v", ids)
	}
}

// Automações de QUALQUER status entram: uma pausada volta amanhã, e se ficar
// fora do snapshot volta apontando para o chip trocado.
func TestBuildSwapSet_IncludesPausedAutomations(t *testing.T) {
	r := row("a", []string{"10"}, []string{"g1@g.us"})
	r.Status = models.AutomationStatusPaused
	r.Automation.Status = models.AutomationStatusPaused

	set := BuildSwapSet([]repository.FailoverAutomationRouting{r}, 10, 20)
	if len(set.Items) != 1 {
		t.Fatal("paused automations must be part of the snapshot")
	}
}

// 13.3.6 — uma automação implícita bloqueia a CONTA inteira, mesmo sem
// referenciar a primária: ela cai no "primeiro chip conectado" do dispatcher,
// que pode ser justamente o chip que estamos trocando.
func TestBuildSwapSet_ImplicitAutomationBlocksAccount(t *testing.T) {
	rows := []repository.FailoverAutomationRouting{
		row("a", []string{"10"}, []string{"g1@g.us"}),
		row("b", nil, []string{"g2@g.us"}), // sem escalar e sem lista
	}
	set := BuildSwapSet(rows, 10, 20)

	if !set.Blocked() {
		t.Fatal("implicit routing anywhere in the account must block")
	}
	if !hasStr(set.BlockingReasons, models.ReasonUnmanagedImplicit) {
		t.Fatalf("expected unmanaged_implicit, got %v", set.BlockingReasons)
	}
}

// 13.3.3 — configuração inválida numa automação elegível bloqueia tudo.
func TestBuildSwapSet_InvalidEligibleConfigBlocks(t *testing.T) {
	r := row("a", []string{"10", "20"}, []string{"g1@g.us"})
	bad := "20" // escalar diferente do primeiro item
	r.Automation.InstanceID = &bad

	set := BuildSwapSet([]repository.FailoverAutomationRouting{r}, 10, 30)
	if !set.Blocked() {
		t.Fatal("inconsistent configuration must block instead of being auto-fixed")
	}
}

func TestBuildSwapSet_ProjectionIsApplied(t *testing.T) {
	set := BuildSwapSet([]repository.FailoverAutomationRouting{
		row("a", []string{"10", "20"}, []string{"g1@g.us"}),
	}, 10, 30)

	if len(set.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(set.Items))
	}
	after := set.Items[0].After
	if after.InstanceIDs[0] != 30 || after.InstanceIDs[1] != 20 {
		t.Fatalf("projection wrong: %v", after.InstanceIDs)
	}
	if set.Items[0].Before.Fingerprint() == after.Fingerprint() {
		t.Fatal("before and after must differ")
	}
}

// A reversão usa o SNAPSHOT original, não uma substituição textual inversa.
func TestBuildRevertSet_UsesOriginalSnapshot(t *testing.T) {
	original := row("a", []string{"10", "20"}, []string{"g1@g.us"})
	beforeSnap, _ := models.NewRoutingSnapshot(&original.Automation)
	afterSnap := beforeSnap.ProjectSwap(10, 30)

	beforeRaw, _ := beforeSnap.MarshalSnapshot()
	afterRaw, _ := afterSnap.MarshalSnapshot()

	// Estado atual = pós-troca.
	live := row("a", []string{"30", "20"}, []string{"g1@g.us"})
	current := map[string]*repository.FailoverAutomationRouting{"a": &live}

	set, err := BuildRevertSet([]models.InstanceFailoverOperationItem{{
		AutomationIDSnapshot: "a",
		FingerprintBefore:    beforeSnap.Fingerprint(),
		FingerprintAfter:     afterSnap.Fingerprint(),
		SnapshotBefore:       beforeRaw,
		SnapshotAfter:        afterRaw,
	}}, current)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if set.Blocked() {
		t.Fatalf("unexpected blocking: %v", set.BlockingReasons)
	}
	if set.Items[0].After.Fingerprint() != beforeSnap.Fingerprint() {
		t.Fatal("revert target must be exactly the original snapshot")
	}
}

// 13.5.10 — item apagado é no-op; item divergente bloqueia a reversão inteira.
func TestBuildRevertSet_DeletedIsNoopDivergentBlocks(t *testing.T) {
	original := row("a", []string{"10"}, []string{"g1@g.us"})
	beforeSnap, _ := models.NewRoutingSnapshot(&original.Automation)
	afterSnap := beforeSnap.ProjectSwap(10, 30)
	beforeRaw, _ := beforeSnap.MarshalSnapshot()
	afterRaw, _ := afterSnap.MarshalSnapshot()

	item := models.InstanceFailoverOperationItem{
		AutomationIDSnapshot: "a",
		FingerprintBefore:    beforeSnap.Fingerprint(),
		FingerprintAfter:     afterSnap.Fingerprint(),
		SnapshotBefore:       beforeRaw,
		SnapshotAfter:        afterRaw,
	}

	// Apagada: sai do conjunto, sem bloquear e sem ser recriada.
	set, err := BuildRevertSet([]models.InstanceFailoverOperationItem{item},
		map[string]*repository.FailoverAutomationRouting{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if set.Blocked() || len(set.Items) != 0 {
		t.Fatalf("deleted automation must be a silent no-op, got %+v", set)
	}

	// Divergente: bloqueia TUDO. Não existe force.
	diverged := row("a", []string{"77"}, []string{"g1@g.us"})
	set, err = BuildRevertSet([]models.InstanceFailoverOperationItem{item},
		map[string]*repository.FailoverAutomationRouting{"a": &diverged})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasStr(set.BlockingReasons, models.ReasonRoutingDiverged) {
		t.Fatalf("expected routing_diverged, got %v", set.BlockingReasons)
	}
}

// Automação NOVA (não estava no switch) não entra na reversão.
func TestBuildRevertSet_IgnoresNewAutomations(t *testing.T) {
	original := row("a", []string{"10"}, []string{"g1@g.us"})
	beforeSnap, _ := models.NewRoutingSnapshot(&original.Automation)
	afterSnap := beforeSnap.ProjectSwap(10, 30)
	beforeRaw, _ := beforeSnap.MarshalSnapshot()
	afterRaw, _ := afterSnap.MarshalSnapshot()

	live := row("a", []string{"30"}, []string{"g1@g.us"})
	brandNew := row("z", []string{"30"}, []string{"g9@g.us"})

	set, err := BuildRevertSet([]models.InstanceFailoverOperationItem{{
		AutomationIDSnapshot: "a",
		FingerprintBefore:    beforeSnap.Fingerprint(),
		FingerprintAfter:     afterSnap.Fingerprint(),
		SnapshotBefore:       beforeRaw,
		SnapshotAfter:        afterRaw,
	}}, map[string]*repository.FailoverAutomationRouting{"a": &live, "z": &brandNew})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(set.Items) != 1 || set.Items[0].AutomationID != "a" {
		t.Fatalf("only the original snapshot's automations revert: %v", set.AutomationIDs())
	}
}

func TestToOperationItems(t *testing.T) {
	set := BuildSwapSet([]repository.FailoverAutomationRouting{
		row("a", []string{"10"}, []string{"g1@g.us"}),
	}, 10, 20)

	items, err := set.ToOperationItems("op-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Result != models.FailoverItemPending {
		t.Fatalf("items start pending, got %s", items[0].Result)
	}
	if items[0].FingerprintBefore == items[0].FingerprintAfter {
		t.Fatal("before/after fingerprints must differ")
	}
}
