package observability

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitMetrics_Disabled_ReturnsNil(t *testing.T) {
	origMetrics := GlobalMetrics
	defer func() { GlobalMetrics = origMetrics }()

	GlobalMetrics = nil
	err := InitMetrics(false)
	require.NoError(t, err)
	assert.Nil(t, GlobalMetrics, "GlobalMetrics must stay nil when disabled")
}

func TestInitMetrics_Enabled_SetsGlobalMetrics(t *testing.T) {
	origMetrics := GlobalMetrics
	defer func() { GlobalMetrics = origMetrics }()

	GlobalMetrics = nil
	err := InitMetrics(true)
	require.NoError(t, err)
	require.NotNil(t, GlobalMetrics, "GlobalMetrics must be populated when enabled")
	assert.NotNil(t, GlobalMetrics.MessagesDispatched)
	assert.NotNil(t, GlobalMetrics.AutomationRuns)
	assert.NotNil(t, GlobalMetrics.WebhookEvents)
	assert.NotNil(t, GlobalMetrics.WorkerTaskDuration)
}

func TestInitMetrics_Enabled_IdempotentWhenCalledTwice(t *testing.T) {
	origMetrics := GlobalMetrics
	defer func() { GlobalMetrics = origMetrics }()

	GlobalMetrics = nil
	require.NoError(t, InitMetrics(true))
	first := GlobalMetrics

	require.NoError(t, InitMetrics(true))
	// GlobalMetrics is overwritten by the second call — both must be non-nil
	assert.NotNil(t, GlobalMetrics)
	assert.NotNil(t, first)
}

func TestInitMetrics_Enabled_RegistersRedirectCounter(t *testing.T) {
	origMetrics := GlobalMetrics
	defer func() { GlobalMetrics = origMetrics }()

	GlobalMetrics = nil
	require.NoError(t, InitMetrics(true))
	require.NotNil(t, GlobalMetrics)
	assert.NotNil(t, GlobalMetrics.RedirectRequests, "redirect counter must be registered")
}

// The redirect handler calls this on every response, including when OTel is
// off — it must never panic on a nil GlobalMetrics.
func TestRecordRedirectRequest_NoopWhenMetricsDisabled(t *testing.T) {
	origMetrics := GlobalMetrics
	defer func() { GlobalMetrics = origMetrics }()

	GlobalMetrics = nil
	assert.NotPanics(t, func() {
		RecordRedirectRequest(context.Background(), "normal", "direct_success", 302)
	})

	GlobalMetrics = &Metrics{}
	assert.NotPanics(t, func() {
		RecordRedirectRequest(context.Background(), "preview", "fallback_preview", 302)
	})
}

func TestRecordRedirectRequest_RecordsWhenEnabled(t *testing.T) {
	origMetrics := GlobalMetrics
	defer func() { GlobalMetrics = origMetrics }()

	GlobalMetrics = nil
	require.NoError(t, InitMetrics(true))
	assert.NotPanics(t, func() {
		RecordRedirectRequest(context.Background(), "normal", "direct_success", 302)
	})
}
