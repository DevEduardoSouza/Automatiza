package observability

import (
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/stretchr/testify/assert"
)

func newTestBudget() *eventBudget {
	return &eventBudget{lastByFingerprint: make(map[string]time.Time)}
}

func TestEventBudget_DedupSameFingerprint(t *testing.T) {
	b := newTestBudget()
	now := time.Now()

	assert.True(t, b.allow("ErrX:boom", now))
	// Same signature within the dedup window is suppressed.
	assert.False(t, b.allow("ErrX:boom", now.Add(time.Minute)))
	// After the dedup window it may send again.
	assert.True(t, b.allow("ErrX:boom", now.Add(fingerprintDedup+time.Second)))
}

func TestEventBudget_GlobalCap(t *testing.T) {
	b := newTestBudget()
	now := time.Now()

	for i := 0; i < budgetMax; i++ {
		assert.True(t, b.allow(string(rune('a'+i)), now), "distinct error %d within budget", i)
	}
	// Budget exhausted: even a brand-new signature is dropped.
	assert.False(t, b.allow("novel:error", now))
	// New window resets the budget.
	assert.True(t, b.allow("novel:error", now.Add(budgetWindow+time.Second)))
}

func TestEventFingerprint(t *testing.T) {
	withExc := &sentry.Event{
		Message:   "ignored when exception exists",
		Exception: []sentry.Exception{{Type: "ErrTimeout", Value: "dial tcp"}},
	}
	assert.Equal(t, "ErrTimeout:dial tcp", eventFingerprint(withExc))

	msgOnly := &sentry.Event{Message: "plain message"}
	assert.Equal(t, "plain message", eventFingerprint(msgOnly))
}
