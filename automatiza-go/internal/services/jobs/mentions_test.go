package jobs

import (
	"context"
	"errors"
	"testing"

	"automatiza-go/internal/adapters/whatsapp"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// ContainsMentions Tests
// =============================================================================

func TestContainsMentions(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    bool
	}{
		{"contains @todos", "Olá @todos!", true},
		{"contains @todos at start", "@todos venham ver!", true},
		{"contains @todos at end", "Confira isso @todos", true},
		{"contains @todos in middle", "Oi @todos, tudo bem?", true},
		{"no mentions", "Mensagem normal sem menções", false},
		{"empty message", "", false},
		{"partial match @todo", "Mensagem com @todo sem s", false},
		{"case sensitive @Todos", "Mensagem com @Todos", false},
		{"multiple @todos", "@todos e @todos novamente", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ContainsMentions(tt.message)
			assert.Equal(t, tt.want, result)
		})
	}
}

// =============================================================================
// ExpandMentions Tests
// =============================================================================

func TestExpandMentions_NoMentions(t *testing.T) {
	ctx := context.Background()
	provider := whatsapp.NewFakeAdapter()

	mentions, msg := ExpandMentions(ctx, provider, "session1", "group1@g.us", "Hello world")

	assert.Nil(t, mentions)
	assert.Equal(t, "Hello world", msg)
	assert.Equal(t, 0, provider.CallCount("GetGroupParticipants"))
}

func TestExpandMentions_WithParticipants(t *testing.T) {
	ctx := context.Background()
	provider := whatsapp.NewFakeAdapter()

	provider.SetParticipants("session1", "group1@g.us", []whatsapp.ParticipantInfo{
		{ID: "5511999999999@s.whatsapp.net", IsAdmin: false},
		{ID: "5511888888888@s.whatsapp.net", IsAdmin: true},
		{ID: "5511777777777@c.us", IsAdmin: false},
	})

	mentions, msg := ExpandMentions(ctx, provider, "session1", "group1@g.us", "Olá @todos!")

	require.NotNil(t, mentions)
	assert.Len(t, mentions, 3)
	assert.Contains(t, mentions, "5511999999999")
	assert.Contains(t, mentions, "5511888888888")
	assert.Contains(t, mentions, "5511777777777")
	assert.Contains(t, msg, "@5511999999999")
	assert.Contains(t, msg, "@5511888888888")
	assert.Contains(t, msg, "@5511777777777")
	assert.NotContains(t, msg, "@todos")
	assert.Equal(t, 1, provider.CallCount("GetGroupParticipants"))
}

func TestExpandMentions_ProviderError(t *testing.T) {
	ctx := context.Background()
	provider := whatsapp.NewFakeAdapter()
	provider.SimulateError = errors.New("connection failed")

	mentions, msg := ExpandMentions(ctx, provider, "session1", "group1@g.us", "Hey @todos!")

	assert.Nil(t, mentions)
	assert.Equal(t, "Hey @todos!", msg)
}

func TestExpandMentions_EmptyParticipants(t *testing.T) {
	ctx := context.Background()
	provider := whatsapp.NewFakeAdapter()
	// SetParticipants not called, defaults to empty

	mentions, msg := ExpandMentions(ctx, provider, "session1", "group1@g.us", "Hello @todos!")

	assert.Nil(t, mentions)
	assert.Equal(t, "Hello @todos!", msg)
}

func TestExpandMentions_OnlyFirstOccurrence(t *testing.T) {
	ctx := context.Background()
	provider := whatsapp.NewFakeAdapter()
	provider.SetParticipants("session1", "group1@g.us", []whatsapp.ParticipantInfo{
		{ID: "5511999999999@s.whatsapp.net"},
	})

	mentions, msg := ExpandMentions(ctx, provider, "session1", "group1@g.us", "@todos primeiro @todos segundo")

	require.NotNil(t, mentions)
	assert.Len(t, mentions, 1)
	// strings.Replace with count 1 only replaces first occurrence
	assert.Contains(t, msg, "@5511999999999")
	assert.Contains(t, msg, "@todos segundo")
}

func TestExpandMentions_SingleParticipant(t *testing.T) {
	ctx := context.Background()
	provider := whatsapp.NewFakeAdapter()
	provider.SetParticipants("session1", "group1@g.us", []whatsapp.ParticipantInfo{
		{ID: "5511999999999@s.whatsapp.net"},
	})

	mentions, msg := ExpandMentions(ctx, provider, "session1", "group1@g.us", "Atenção @todos!")

	require.NotNil(t, mentions)
	assert.Len(t, mentions, 1)
	assert.Equal(t, "5511999999999", mentions[0])
	assert.Equal(t, "Atenção @5511999999999!", msg)
}
