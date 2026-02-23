package jobs

import (
	"context"
	"errors"
	"testing"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// =============================================================================
// ExecuteSendPoll Tests
// =============================================================================

func TestExecuteSendPoll_Success(t *testing.T) {
	ctx := context.Background()
	provider := whatsapp.NewFakeAdapter()
	logger := zaptest.NewLogger(t)

	job := &models.StandaloneJob{
		ActionType: models.JobActionSendPoll,
		Payload: models.JobPayload{
			Groups:          []string{"group1@g.us"},
			Question:        "Qual sua cor favorita?",
			Options:         []string{"Vermelho", "Azul", "Verde"},
			SelectableCount: 1,
		},
	}

	results := ExecuteSendPoll(ctx, provider, "session1", job, logger)

	require.Len(t, results, 1)
	assert.True(t, results[0].Success)
	assert.Equal(t, 1, provider.CallCount("SendPoll"))

	call := provider.LastCall("SendPoll")
	require.NotNil(t, call)
	input := call.Args[0].(whatsapp.SendPollInput)
	assert.Equal(t, "Qual sua cor favorita?", input.Question)
	assert.Equal(t, []string{"Vermelho", "Azul", "Verde"}, input.Options)
	assert.Equal(t, 1, input.SelectableCount)
}

func TestExecuteSendPoll_MultipleGroups(t *testing.T) {
	ctx := context.Background()
	provider := whatsapp.NewFakeAdapter()
	logger := zaptest.NewLogger(t)

	job := &models.StandaloneJob{
		ActionType: models.JobActionSendPoll,
		Payload: models.JobPayload{
			Groups:          []string{"group1@g.us", "group2@g.us"},
			Question:        "Pergunta?",
			Options:         []string{"A", "B"},
			SelectableCount: 1,
		},
	}

	results := ExecuteSendPoll(ctx, provider, "session1", job, logger)

	require.Len(t, results, 2)
	for _, r := range results {
		assert.True(t, r.Success)
	}
	assert.Equal(t, 2, provider.CallCount("SendPoll"))
}

func TestExecuteSendPoll_InvalidOptions_TooFew(t *testing.T) {
	ctx := context.Background()
	provider := whatsapp.NewFakeAdapter()
	logger := zaptest.NewLogger(t)

	job := &models.StandaloneJob{
		ActionType: models.JobActionSendPoll,
		Payload: models.JobPayload{
			Groups:   []string{"group1@g.us", "group2@g.us"},
			Question: "Pergunta?",
			Options:  []string{"A"},
		},
	}

	results := ExecuteSendPoll(ctx, provider, "session1", job, logger)

	require.Len(t, results, 2)
	for _, r := range results {
		assert.False(t, r.Success)
		assert.Contains(t, r.Error, "mínimo 2")
	}
	assert.Equal(t, 0, provider.CallCount("SendPoll"))
}

func TestExecuteSendPoll_InvalidOptions_TooMany(t *testing.T) {
	ctx := context.Background()
	provider := whatsapp.NewFakeAdapter()
	logger := zaptest.NewLogger(t)

	options := make([]string, 13)
	for i := range options {
		options[i] = "Opt"
	}

	job := &models.StandaloneJob{
		ActionType: models.JobActionSendPoll,
		Payload: models.JobPayload{
			Groups:   []string{"group1@g.us"},
			Question: "Pergunta?",
			Options:  options,
		},
	}

	results := ExecuteSendPoll(ctx, provider, "session1", job, logger)

	require.Len(t, results, 1)
	assert.False(t, results[0].Success)
	assert.Contains(t, results[0].Error, "máximo 12")
}

func TestExecuteSendPoll_DefaultSelectableCount(t *testing.T) {
	ctx := context.Background()
	provider := whatsapp.NewFakeAdapter()
	logger := zaptest.NewLogger(t)

	job := &models.StandaloneJob{
		ActionType: models.JobActionSendPoll,
		Payload: models.JobPayload{
			Groups:          []string{"group1@g.us"},
			Question:        "Pergunta?",
			Options:         []string{"A", "B", "C"},
			SelectableCount: 0, // Should default to 1
		},
	}

	results := ExecuteSendPoll(ctx, provider, "session1", job, logger)

	require.Len(t, results, 1)
	assert.True(t, results[0].Success)

	call := provider.LastCall("SendPoll")
	require.NotNil(t, call)
	input := call.Args[0].(whatsapp.SendPollInput)
	assert.Equal(t, 1, input.SelectableCount)
}

func TestExecuteSendPoll_MultiSelect(t *testing.T) {
	ctx := context.Background()
	provider := whatsapp.NewFakeAdapter()
	logger := zaptest.NewLogger(t)

	job := &models.StandaloneJob{
		ActionType: models.JobActionSendPoll,
		Payload: models.JobPayload{
			Groups:          []string{"group1@g.us"},
			Question:        "Selecione 3:",
			Options:         []string{"A", "B", "C", "D", "E"},
			SelectableCount: 3,
		},
	}

	results := ExecuteSendPoll(ctx, provider, "session1", job, logger)

	require.Len(t, results, 1)
	assert.True(t, results[0].Success)

	call := provider.LastCall("SendPoll")
	require.NotNil(t, call)
	input := call.Args[0].(whatsapp.SendPollInput)
	assert.Equal(t, 3, input.SelectableCount)
}

func TestExecuteSendPoll_ProviderError(t *testing.T) {
	ctx := context.Background()
	provider := whatsapp.NewFakeAdapter()
	provider.SendPollFunc = func(_ context.Context, _ whatsapp.SendPollInput) (*whatsapp.MessageResult, error) {
		return nil, errors.New("poll send failed")
	}
	logger := zaptest.NewLogger(t)

	job := &models.StandaloneJob{
		ActionType: models.JobActionSendPoll,
		Payload: models.JobPayload{
			Groups:   []string{"group1@g.us"},
			Question: "?",
			Options:  []string{"A", "B"},
		},
	}

	results := ExecuteSendPoll(ctx, provider, "session1", job, logger)

	require.Len(t, results, 1)
	assert.False(t, results[0].Success)
	assert.Equal(t, "poll send failed", results[0].Error)
}

func TestExecuteSendPoll_NilOptions(t *testing.T) {
	ctx := context.Background()
	provider := whatsapp.NewFakeAdapter()
	logger := zaptest.NewLogger(t)

	job := &models.StandaloneJob{
		ActionType: models.JobActionSendPoll,
		Payload: models.JobPayload{
			Groups:   []string{"group1@g.us"},
			Question: "?",
			Options:  nil,
		},
	}

	results := ExecuteSendPoll(ctx, provider, "session1", job, logger)

	require.Len(t, results, 1)
	assert.False(t, results[0].Success)
	assert.Contains(t, results[0].Error, "mínimo 2")
}
