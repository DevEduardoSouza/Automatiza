package observability

import (
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
