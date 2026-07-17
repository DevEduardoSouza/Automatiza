package connectrpc

import (
	"strings"
	"testing"

	"automatiza-go/internal/models"
)

func ogAutomation(id, name, status string, monitor, send []string) models.Automation {
	t := models.AutomationTypeOfferGenerate
	return models.Automation{
		ID:              id,
		Name:            name,
		Type:            &t,
		Status:          status,
		MonitorGroupIDs: monitor,
		SendGroups:      send,
	}
}

func TestOfferGenerateOverlapWarning(t *testing.T) {
	src := []string{"src-A@g.us"}
	dst := []string{"g1@g.us", "g2@g.us", "g3@g.us"}

	tests := []struct {
		name       string
		existing   []models.Automation
		excludeID  string
		monitor    []string
		send       []string
		wantWarn   bool
		wantShared string // substring expected when warning fires
	}{
		{
			name:       "same source and overlapping dest warns",
			existing:   []models.Automation{ogAutomation("a1", "Gerar Oferta", "active", src, []string{"g1@g.us", "g2@g.us"})},
			monitor:    src,
			send:       dst,
			wantWarn:   true,
			wantShared: "compartilha 2 grupo",
		},
		{
			name:     "same source but disjoint dest does not warn",
			existing: []models.Automation{ogAutomation("a1", "X", "active", src, []string{"other@g.us"})},
			monitor:  src,
			send:     dst,
			wantWarn: false,
		},
		{
			name:     "same dest but disjoint source does not warn (legit funnel)",
			existing: []models.Automation{ogAutomation("a1", "X", "active", []string{"src-B@g.us"}, dst)},
			monitor:  src,
			send:     dst,
			wantWarn: false,
		},
		{
			name:      "excludes self on update",
			existing:  []models.Automation{ogAutomation("self", "X", "active", src, dst)},
			excludeID: "self",
			monitor:   src,
			send:      dst,
			wantWarn:  false,
		},
		{
			name:     "paused existing does not warn",
			existing: []models.Automation{ogAutomation("a1", "X", "paused", src, dst)},
			monitor:  src,
			send:     dst,
			wantWarn: false,
		},
		{
			name: "non offer_generate existing does not warn",
			existing: []models.Automation{func() models.Automation {
				a := ogAutomation("a1", "X", "active", src, dst)
				mt := models.AutomationTypeMonitoring
				a.Type = &mt
				return a
			}()},
			monitor:  src,
			send:     dst,
			wantWarn: false,
		},
		{
			name:     "no existing automations does not warn",
			existing: nil,
			monitor:  src,
			send:     dst,
			wantWarn: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := offerGenerateOverlapWarning(tc.existing, tc.excludeID, tc.monitor, tc.send)
			if tc.wantWarn && got == "" {
				t.Fatalf("expected a warning, got empty")
			}
			if !tc.wantWarn && got != "" {
				t.Fatalf("expected no warning, got %q", got)
			}
			if tc.wantWarn && tc.wantShared != "" && !strings.Contains(got, tc.wantShared) {
				t.Fatalf("warning %q missing expected substring %q", got, tc.wantShared)
			}
		})
	}
}
