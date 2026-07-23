package failover

import (
	"context"
	"errors"
	"testing"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/models"
)

// fakeProvider grava TODAS as chamadas feitas. Um teste desta suíte checa a
// lista para provar que o dry-run nunca toca um endpoint mutável.
type fakeProvider struct {
	status       *whatsapp.FreshSessionStatus
	statusErr    error
	groups       []whatsapp.GroupInfo
	groupsErr    error
	participants map[string][]whatsapp.TrackingParticipant
	partErr      error
	calls        []string
}

func (f *fakeProvider) GetSessionStatusFresh(_ context.Context, session string) (*whatsapp.FreshSessionStatus, error) {
	f.calls = append(f.calls, "GetSessionStatusFresh:"+session)
	return f.status, f.statusErr
}

func (f *fakeProvider) GetGroups(_ context.Context, session string) ([]whatsapp.GroupInfo, error) {
	f.calls = append(f.calls, "GetGroups:"+session)
	return f.groups, f.groupsErr
}

func (f *fakeProvider) GetGroupParticipantsForTracking(_ context.Context, session, groupID string) ([]whatsapp.TrackingParticipant, error) {
	f.calls = append(f.calls, "GetGroupParticipants:"+groupID)
	return f.participants[groupID], f.partErr
}

func workingStatus() *whatsapp.FreshSessionStatus {
	return &whatsapp.FreshSessionStatus{
		Name:      "s1",
		RawStatus: whatsapp.WahaStatusWorking,
		Known:     true,
		MeID:      "5511999999999@c.us",
	}
}

func newChecker(p *fakeProvider) *CoverageChecker {
	c := NewCoverageChecker(p)
	c.gap = 0 // o teste não paga o intervalo anti-rajada
	return c
}

// 13.2.1 — WORKING fresh passa.
func TestInspect_WorkingPasses(t *testing.T) {
	p := &fakeProvider{
		status: workingStatus(),
		groups: []whatsapp.GroupInfo{{ID: "g1@g.us"}, {ID: "g2@g.us"}},
	}
	cov := newChecker(p).Inspect(context.Background(), "s1", []string{"g1@g.us"}, []string{"g1@g.us"})

	if !cov.SessionOK || cov.Unavailable || len(cov.Reasons) != 0 {
		t.Fatalf("expected clean coverage, got %+v", cov)
	}
	if !cov.MemberOf["g1@g.us"] {
		t.Fatal("membership not detected")
	}
}

// 13.2.1 — status não-WORKING bloqueia, mesmo que o cache dissesse outra coisa.
func TestInspect_NonWorkingBlocks(t *testing.T) {
	for _, raw := range []string{
		whatsapp.WahaStatusFailed,
		whatsapp.WahaStatusStopped,
		whatsapp.WahaStatusStarting,
		whatsapp.WahaStatusScanQRCode,
	} {
		p := &fakeProvider{status: &whatsapp.FreshSessionStatus{RawStatus: raw, Known: true, MeID: "x@c.us"}}
		cov := newChecker(p).Inspect(context.Background(), "s1", nil, nil)
		if cov.SessionOK {
			t.Fatalf("%s must not pass as working", raw)
		}
		if !hasStr(cov.Reasons, models.ReasonTargetNotWorking) {
			t.Fatalf("%s: expected target_session_not_working, got %v", raw, cov.Reasons)
		}
	}
}

// 13.2.2 — erro/timeout do provider vira `unavailable`, NUNCA `missing`.
//
// A distinção é o que impede um soluço do WAHA de ser lido como "o chip não
// está mais no grupo".
func TestInspect_ProviderErrorIsUnavailableNotMissing(t *testing.T) {
	p := &fakeProvider{statusErr: errors.New("timeout")}
	cov := newChecker(p).Inspect(context.Background(), "s1", []string{"g1@g.us"}, []string{"g1@g.us"})

	if !cov.Unavailable {
		t.Fatal("provider error must mark coverage as unavailable")
	}
	if !hasStr(cov.Reasons, models.ReasonProviderUnavailable) {
		t.Fatalf("expected waha_unavailable, got %v", cov.Reasons)
	}
	if hasStr(cov.Reasons, models.ReasonGroupMissing) {
		t.Fatal("uncertainty must never be reported as a missing group")
	}
}

func TestInspect_GroupsErrorIsUnavailable(t *testing.T) {
	p := &fakeProvider{status: workingStatus(), groupsErr: errors.New("500")}
	cov := newChecker(p).Inspect(context.Background(), "s1", []string{"g1@g.us"}, nil)
	if !cov.Unavailable {
		t.Fatal("groups error must mark coverage as unavailable")
	}
}

// 13.2.6 — sem me.id não dá para identificar a própria sessão: bloqueia.
func TestInspect_MissingIdentityBlocks(t *testing.T) {
	p := &fakeProvider{status: &whatsapp.FreshSessionStatus{RawStatus: whatsapp.WahaStatusWorking, Known: true}}
	cov := newChecker(p).Inspect(context.Background(), "s1", nil, nil)
	if !hasStr(cov.Reasons, models.ReasonReserveIdentity) {
		t.Fatalf("expected reserve_identity_unknown, got %v", cov.Reasons)
	}
}

// 13.2.4 — Announce=false não exige admin (e nem busca participantes).
func TestInspect_NoAdminNeededWhenNotAnnounce(t *testing.T) {
	p := &fakeProvider{
		status: workingStatus(),
		groups: []whatsapp.GroupInfo{{ID: "g1@g.us", Announce: false}},
	}
	cov := newChecker(p).Inspect(context.Background(), "s1", []string{"g1@g.us"}, []string{"g1@g.us"})
	if cov.Announce["g1@g.us"] {
		t.Fatal("group should not be announce-only")
	}
	for _, c := range p.calls {
		if len(c) > 20 && c[:20] == "GetGroupParticipants" {
			t.Fatalf("participants must not be fetched for non-announce groups: %v", p.calls)
		}
	}
}

// 13.2.5 — Announce=true aceita admin/superadmin e rejeita participant/left.
func TestInspect_AdminRoles(t *testing.T) {
	cases := map[string]bool{"admin": true, "superadmin": true, "participant": false, "left": false}
	for role, wantAdmin := range cases {
		t.Run(role, func(t *testing.T) {
			p := &fakeProvider{
				status: workingStatus(),
				groups: []whatsapp.GroupInfo{{ID: "g1@g.us", Announce: true}},
				participants: map[string][]whatsapp.TrackingParticipant{
					"g1@g.us": {{ID: "5511999999999@c.us", Role: role}},
				},
			}
			cov := newChecker(p).Inspect(context.Background(), "s1", []string{"g1@g.us"}, []string{"g1@g.us"})
			if cov.AdminOf["g1@g.us"] != wantAdmin {
				t.Fatalf("role %s: admin=%v, want %v", role, cov.AdminOf["g1@g.us"], wantAdmin)
			}
			if role == "left" && cov.MemberOf["g1@g.us"] {
				t.Fatal("role 'left' must drop membership: the member list is the fresher truth")
			}
		})
	}
}

// Identidade por LID: o me.id vem em @c.us e o participante em @lid com pn.
func TestInspect_MatchesSelfByPN(t *testing.T) {
	p := &fakeProvider{
		status: workingStatus(),
		groups: []whatsapp.GroupInfo{{ID: "g1@g.us", Announce: true}},
		participants: map[string][]whatsapp.TrackingParticipant{
			"g1@g.us": {{ID: "8888@lid", PN: "5511999999999@c.us", Role: "admin"}},
		},
	}
	cov := newChecker(p).Inspect(context.Background(), "s1", []string{"g1@g.us"}, []string{"g1@g.us"})
	if !cov.AdminOf["g1@g.us"] {
		t.Fatal("self must be matched by pn when the id is a LID")
	}
}

// Sufixo de dispositivo no me.id (":12") não pode impedir o match.
func TestInspect_MatchesSelfIgnoringDeviceSuffix(t *testing.T) {
	st := workingStatus()
	st.MeID = "5511999999999:12@c.us"
	p := &fakeProvider{
		status: st,
		groups: []whatsapp.GroupInfo{{ID: "g1@g.us", Announce: true}},
		participants: map[string][]whatsapp.TrackingParticipant{
			"g1@g.us": {{ID: "5511999999999@c.us", Role: "superadmin"}},
		},
	}
	cov := newChecker(p).Inspect(context.Background(), "s1", []string{"g1@g.us"}, []string{"g1@g.us"})
	if !cov.AdminOf["g1@g.us"] {
		t.Fatal("device suffix must not break self identification")
	}
}

// Sessão que não se encontra na lista do próprio grupo: indeterminado.
func TestInspect_SelfNotInParticipantsIsUnavailable(t *testing.T) {
	p := &fakeProvider{
		status: workingStatus(),
		groups: []whatsapp.GroupInfo{{ID: "g1@g.us", Announce: true}},
		participants: map[string][]whatsapp.TrackingParticipant{
			"g1@g.us": {{ID: "outro@c.us", Role: "admin"}},
		},
	}
	cov := newChecker(p).Inspect(context.Background(), "s1", []string{"g1@g.us"}, []string{"g1@g.us"})
	if !cov.Unavailable {
		t.Fatal("not finding ourselves in the list must be unavailable, not 'no admin'")
	}
}

// 13.2.7 — nenhum método mutável é chamado. Em especial /groups/refresh.
func TestInspect_OnlyReadOnlyCalls(t *testing.T) {
	p := &fakeProvider{
		status: workingStatus(),
		groups: []whatsapp.GroupInfo{{ID: "g1@g.us", Announce: true}},
		participants: map[string][]whatsapp.TrackingParticipant{
			"g1@g.us": {{ID: "5511999999999@c.us", Role: "admin"}},
		},
	}
	newChecker(p).Inspect(context.Background(), "s1", []string{"g1@g.us"}, []string{"g1@g.us"})

	allowed := map[string]bool{
		"GetSessionStatusFresh:s1":     true,
		"GetGroups:s1":                 true,
		"GetGroupParticipants:g1@g.us": true,
	}
	for _, c := range p.calls {
		if !allowed[c] {
			t.Fatalf("unexpected provider call %q — the dry run is read-only", c)
		}
	}
}

// 13.2.3/13.2.8 — grupo ausente bloqueia com o ID; capacidade 20 passa, 21 não.
func TestEvaluateItem(t *testing.T) {
	cov := &CoverageResult{
		SessionOK: true,
		MemberOf:  map[string]bool{"g1@g.us": true},
		AdminOf:   map[string]bool{},
		Announce:  map[string]bool{},
	}
	item := mkItem(t, []string{"g1@g.us", "g2@g.us"}, nil)

	res := EvaluateItem(item, cov)
	if res.Result != models.FailoverCoverageBlocked {
		t.Fatal("missing group must block")
	}
	if len(res.MissingGroups) != 1 || res.MissingGroups[0] != "g2@g.us" {
		t.Fatalf("missing group id must be reported: %v", res.MissingGroups)
	}
}

func TestEvaluateItem_AdminMissingBlocks(t *testing.T) {
	cov := &CoverageResult{
		SessionOK: true,
		MemberOf:  map[string]bool{"g1@g.us": true},
		AdminOf:   map[string]bool{},
		Announce:  map[string]bool{"g1@g.us": true},
	}
	res := EvaluateItem(mkItem(t, []string{"g1@g.us"}, nil), cov)
	if !hasStr(res.Reasons, models.ReasonAdminMissing) {
		t.Fatalf("announce-only destination without admin must block: %v", res.Reasons)
	}
}

func TestEvaluateItem_CapacityCap(t *testing.T) {
	// 20 destinos passam; 21 bloqueiam — mesmo com FEATURE_ENFORCE_GROUP_LIMITS
	// desligada, que é o default em produção.
	mkGroups := func(n int) []string {
		out := make([]string, 0, n)
		for i := 0; i < n; i++ {
			out = append(out, string(rune('a'+i%26))+"-"+string(rune('a'+i/26))+"@g.us")
		}
		return out
	}

	for _, tc := range []struct {
		n         int
		wantBlock bool
	}{{20, false}, {21, true}} {
		groups := mkGroups(tc.n)
		member := map[string]bool{}
		for _, g := range groups {
			member[g] = true
		}
		cov := &CoverageResult{SessionOK: true, MemberOf: member, AdminOf: map[string]bool{}, Announce: map[string]bool{}}

		res := EvaluateItem(mkItem(t, groups, nil), cov)
		blocked := hasStr(res.Reasons, models.ReasonCapacityExceeded)
		if blocked != tc.wantBlock {
			t.Fatalf("%d target groups: capacity blocked=%v, want %v", tc.n, blocked, tc.wantBlock)
		}
	}

	// Origem conta SEPARADAMENTE da lista de destinos.
	sources := mkGroups(21)
	member := map[string]bool{}
	for _, g := range sources {
		member[g] = true
	}
	member["dest@g.us"] = true
	cov := &CoverageResult{SessionOK: true, MemberOf: member, AdminOf: map[string]bool{}, Announce: map[string]bool{}}
	res := EvaluateItem(mkItem(t, []string{"dest@g.us"}, sources), cov)
	if !hasStr(res.Reasons, models.ReasonCapacityExceeded) {
		t.Fatal("source groups must be capped separately from destinations")
	}
}

func TestEvaluateItem_UnavailableBlocksWithoutFakeMissing(t *testing.T) {
	cov := &CoverageResult{Unavailable: true, Reasons: []string{models.ReasonProviderUnavailable}}
	res := EvaluateItem(mkItem(t, []string{"g1@g.us"}, nil), cov)
	if res.Result != models.FailoverCoverageBlocked {
		t.Fatal("unavailable coverage must block")
	}
	if len(res.MissingGroups) != 0 {
		t.Fatal("unavailable must not fabricate missing groups")
	}
}

func TestCoverageHash_StableAndSensitive(t *testing.T) {
	a := map[string]ItemCoverage{"x": {RequiredTargets: []string{"g1"}, Result: models.FailoverCoverageCovered}}
	b := map[string]ItemCoverage{"x": {RequiredTargets: []string{"g1"}, Result: models.FailoverCoverageCovered}}
	if CoverageHash(a) != CoverageHash(b) {
		t.Fatal("same coverage must hash the same")
	}
	c := map[string]ItemCoverage{"x": {RequiredTargets: []string{"g1", "g2"}, Result: models.FailoverCoverageCovered}}
	if CoverageHash(a) == CoverageHash(c) {
		t.Fatal("different coverage must hash differently")
	}
}

// mkItem monta um EligibleItem com o escopo pedido.
func mkItem(t *testing.T, targets, sources []string) *EligibleItem {
	t.Helper()
	typ := models.AutomationTypeOfferGenerate
	a := &models.Automation{
		ID:              "11111111-1111-1111-1111-111111111111",
		UserID:          "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
		Type:            &typ,
		SendGroups:      targets,
		MonitorGroupIDs: sources,
		InstanceID:      ptr("10"),
		InstanceIDs:     []string{"10"},
	}
	snap, reasons := models.NewRoutingSnapshot(a)
	if len(reasons) != 0 {
		t.Fatalf("fixture must be valid, got %v", reasons)
	}
	return &EligibleItem{AutomationID: a.ID, Before: snap, After: snap.ProjectSwap(10, 20)}
}

func ptr(s string) *string { return &s }

func hasStr(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}
