package wamentions

import (
	"context"
	"errors"
	"testing"

	"automatiza-go/internal/adapters/whatsapp"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContains(t *testing.T) {
	assert.True(t, Contains("Olá @todos!"))
	assert.True(t, Contains("@todos venham ver"))
	assert.False(t, Contains("mensagem normal"))
	assert.False(t, Contains("@todo sem s"))
	assert.False(t, Contains("@Todos maiúsculo"))
	assert.False(t, Contains(""))
}

func TestExpand_NoMarker(t *testing.T) {
	provider := whatsapp.NewFakeAdapter()
	provider.SetParticipants("s", "g@g.us", []whatsapp.ParticipantInfo{
		{ID: "5511999999999@c.us"},
	})

	got := Expand(context.Background(), provider, "s", "g@g.us", "sem marcador")
	assert.Nil(t, got)
	// Não deve nem buscar os membros quando não há @todos.
	assert.Equal(t, 0, provider.CallCount("GetGroupParticipants"))
}

func TestExpand_ResolvedPhone(t *testing.T) {
	// Quando PhoneNumber está resolvido, usa o telefone bare — o WAHA resolve
	// para @s.whatsapp.net e o membro real é notificado.
	provider := whatsapp.NewFakeAdapter()
	provider.SetParticipants("s", "g@g.us", []whatsapp.ParticipantInfo{
		{ID: "16785386532987@lid", PhoneNumber: "557488359787"},
	})

	got := Expand(context.Background(), provider, "s", "g@g.us", "oi @todos")
	require.Len(t, got, 1)
	assert.Equal(t, "557488359787", got[0])
}

func TestExpand_UnresolvedLIDKeepsSuffix(t *testing.T) {
	// LID sem telefone resolvido: mantém "<lid>@lid" (menção LID válida).
	// Stripar o sufixo viraria "<lid>@s.whatsapp.net" = ghost mention.
	provider := whatsapp.NewFakeAdapter()
	provider.SetParticipants("s", "g@g.us", []whatsapp.ParticipantInfo{
		{ID: "144538869973215@lid"},
	})

	got := Expand(context.Background(), provider, "s", "g@g.us", "oi @todos")
	require.Len(t, got, 1)
	assert.Equal(t, "144538869973215@lid", got[0])
}

func TestExpand_PhoneStyleIDWithoutPn(t *testing.T) {
	// ID estilo telefone sem PhoneNumber: strip do sufixo → telefone bare.
	// (Mantém paridade com o comportamento histórico do jobs.)
	provider := whatsapp.NewFakeAdapter()
	provider.SetParticipants("s", "g@g.us", []whatsapp.ParticipantInfo{
		{ID: "5511999999999@s.whatsapp.net"},
		{ID: "5511888888888@c.us"},
	})

	got := Expand(context.Background(), provider, "s", "g@g.us", "oi @todos")
	require.Len(t, got, 2)
	assert.Contains(t, got, "5511999999999")
	assert.Contains(t, got, "5511888888888")
}

func TestExpand_ProviderError(t *testing.T) {
	provider := whatsapp.NewFakeAdapter()
	provider.SimulateError = errors.New("boom")

	got := Expand(context.Background(), provider, "s", "g@g.us", "oi @todos")
	assert.Nil(t, got)
}

func TestExpand_EmptyParticipants(t *testing.T) {
	provider := whatsapp.NewFakeAdapter()
	got := Expand(context.Background(), provider, "s", "g@g.us", "oi @todos")
	assert.Nil(t, got)
}
