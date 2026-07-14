package connectrpc

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	whatsappv1 "automatiza-go/internal/gen/whatsapp/v1"
	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// DeleteInstance só pode deletar sessão do PRÓPRIO usuário. Sem a checagem de
// posse, qualquer autenticado deletava sessão alheia pelo nome (o guard do
// waha-router protege só as pareadas — stub de QR de terceiro ficava exposto).
func TestDeleteInstance_Ownership(t *testing.T) {
	t.Run("sessão de OUTRO usuário → NotFound e nenhum DELETE na WAHA", func(t *testing.T) {
		repo := new(mockInstanceRepo)
		repo.On("GetInstanceByUserAndName", mock.Anything, "u1", "session_alheia").Return(nil, nil)
		fake := whatsapp.NewFakeAdapter()
		h := NewWhatsAppHandler(fake, repo, nil, nil, nil, "")

		_, err := h.DeleteInstance(contextWithUser("u1", "u1@example.com"),
			connect.NewRequest(&whatsappv1.DeleteInstanceRequest{InstanceName: "session_alheia"}))
		require.Error(t, err)
		assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
		assert.Equal(t, 0, fake.CallCount("DeleteSession"), "não pode tocar a WAHA sem posse confirmada")
	})

	t.Run("sessão do próprio usuário → deleta na WAHA", func(t *testing.T) {
		repo := new(mockInstanceRepo)
		repo.On("GetInstanceByUserAndName", mock.Anything, "u1", "session_minha").
			Return(&models.Instance{ID: 7, InstanceID: "session_minha"}, nil)
		fake := whatsapp.NewFakeAdapter()
		h := NewWhatsAppHandler(fake, repo, nil, nil, nil, "")

		resp, err := h.DeleteInstance(contextWithUser("u1", "u1@example.com"),
			connect.NewRequest(&whatsappv1.DeleteInstanceRequest{InstanceName: "session_minha"}))
		require.NoError(t, err)
		assert.True(t, resp.Msg.GetSuccess())
		assert.Equal(t, 1, fake.CallCount("DeleteSession"))
	})

	t.Run("sem autenticação → Unauthenticated", func(t *testing.T) {
		h := NewWhatsAppHandler(whatsapp.NewFakeAdapter(), new(mockInstanceRepo), nil, nil, nil, "")
		_, err := h.DeleteInstance(context.Background(),
			connect.NewRequest(&whatsappv1.DeleteInstanceRequest{InstanceName: "x"}))
		require.Error(t, err)
		assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
	})
}
