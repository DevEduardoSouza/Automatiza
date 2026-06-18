package offer_generate //nolint:revive // package name already has underscore

import (
	"context"
	"testing"
	"time"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"

	"go.uber.org/zap"
)

type twoChipInstanceRepo struct {
	repository.InstanceRepository
}

func (twoChipInstanceRepo) GetUserInstances(_ context.Context, _ string) ([]models.Instance, error) {
	return []models.Instance{
		{ID: 1, InstanceID: "sessA", Status: models.InstanceStatusConnected},
		{ID: 2, InstanceID: "sessB", Status: models.InstanceStatusConnected},
	}, nil
}

// TestProcessAutomationRelay_FansOutInParallelByChip proves the perf fix: two
// groups routed (via ChipOverrides) to two different chips are sent
// CONCURRENTLY, not one-after-the-other. The fake blocks each send until both
// have started; with the old serial loop only the first send would ever start
// (the loop can't reach the second until the first returns), so the second
// arrival would never come and the test would time out.
func TestProcessAutomationRelay_FansOutInParallelByChip(t *testing.T) {
	fake := whatsapp.NewFakeAdapter()

	arrived := make(chan string, 2)
	release := make(chan struct{})
	fake.SendTextFunc = func(_ context.Context, in whatsapp.SendTextInput) (*whatsapp.MessageResult, error) {
		arrived <- in.SessionName
		<-release // hold the send open so a serial loop can't advance
		return &whatsapp.MessageResult{ID: "ok"}, nil
	}

	svc := &offerGenerateService{
		logger:       zap.NewNop(),
		waProvider:   fake,
		instanceRepo: twoChipInstanceRepo{},
		// cache, dispatchLogRepo, groupRepo = nil → claimProductSlot allows,
		// logDispatch no-op, no participant filtering.
	}

	d := 1
	auto := &models.Automation{
		ID:            "a-relay",
		UserID:        "u1",
		Status:        models.AutomationStatusActive,
		SendGroups:    []string{"g1@g.us", "g2@g.us"},
		InstanceIDs:   []string{"1", "2"},
		ChipOverrides: map[string]string{"g1@g.us": "1", "g2@g.us": "2"},
		DelaySeconds:  &d,
	}
	msg := &models.IncomingMessage{
		RemoteJID: "src@g.us",
		MessageID: "m1",
		Body:      "oferta de teste",
	}

	done := make(chan error, 1)
	go func() { done <- svc.processAutomationRelay(context.Background(), auto, msg) }()

	seen := map[string]bool{}
	for i := 0; i < 2; i++ {
		select {
		case s := <-arrived:
			seen[s] = true
		case <-time.After(8 * time.Second):
			t.Fatalf("only %d/2 sends started concurrently — fan-out is not parallel by chip", len(seen))
		}
	}
	close(release)

	if err := <-done; err != nil {
		t.Fatalf("processAutomationRelay returned error: %v", err)
	}
	if !seen["sessA"] || !seen["sessB"] {
		t.Fatalf("both chips must have sent concurrently, saw: %v", seen)
	}
}
