package connectrpc

import (
	"testing"

	"automatiza-go/internal/adapters/whatsapp"
	whatsappv1 "automatiza-go/internal/gen/whatsapp/v1"
	"automatiza-go/internal/models"
	"connectrpc.com/connect"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Clique repetido em "Adicionar novo número" NÃO pode criar uma sessão WAHA
// nova a cada tentativa: se o usuário já tem uma instância sem telefone cuja
// sessão ainda está em SCAN_QR, o CreateInstance reaproveita essa sessão.
func TestCreateInstance_ReusesPendingSession(t *testing.T) {
	t.Run("tentativa pendente viva → reusa, sem CreateSession novo", func(t *testing.T) {
		repo := new(mockInstanceRepo)
		repo.On("CountUserInstances", mock.Anything, "u1").Return(1, nil)
		repo.On("GetUserInstances", mock.Anything, "u1").Return([]models.Instance{
			{ID: 1, InstanceID: "session_pending", Status: models.InstanceStatusPending},
		}, nil)
		repo.On("UpsertInstance", mock.Anything, "u1", mock.Anything).Return(&models.Instance{ID: 1}, nil)

		fake := whatsapp.NewFakeAdapter()
		fake.Sessions["session_pending"] = &whatsapp.SessionInfo{
			Name:   "session_pending",
			Status: whatsapp.SessionStatusScanQR,
		}
		h := NewWhatsAppHandler(fake, repo, nil, nil, nil, "")

		resp, err := h.CreateInstance(contextWithUser("u1", "u1@example.com"),
			connect.NewRequest(&whatsappv1.CreateInstanceRequest{}))
		require.NoError(t, err)
		assert.Equal(t, "session_pending", resp.Msg.GetInstanceName())
		assert.Equal(t, 0, fake.CallCount("CreateSession"), "não pode criar sessão nova havendo pendente reutilizável")
	})

	t.Run("sem pendente reutilizável → cria sessão normalmente", func(t *testing.T) {
		repo := new(mockInstanceRepo)
		repo.On("CountUserInstances", mock.Anything, "u1").Return(0, nil)
		repo.On("GetUserInstances", mock.Anything, "u1").Return([]models.Instance{}, nil)
		repo.On("UpsertInstance", mock.Anything, "u1", mock.Anything).Return(&models.Instance{ID: 2}, nil)

		fake := whatsapp.NewFakeAdapter()
		h := NewWhatsAppHandler(fake, repo, nil, nil, nil, "")

		_, err := h.CreateInstance(contextWithUser("u1", "u1@example.com"),
			connect.NewRequest(&whatsappv1.CreateInstanceRequest{}))
		require.NoError(t, err)
		assert.Equal(t, 1, fake.CallCount("CreateSession"))
	})
}
