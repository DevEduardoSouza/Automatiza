package connectrpc

import (
	"context"
	"errors"
	"testing"

	"automatiza-go/internal/adapters/whatsapp"
	whatsappv1 "automatiza-go/internal/gen/whatsapp/v1"
	"automatiza-go/internal/models"
	"connectrpc.com/connect"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// RequestPairingCode é o caminho ALTERNATIVO ao QR: mesma posse do
// DeleteInstance e validação de telefone na fronteira de confiança (só dígitos,
// 8..15) antes de tocar a WAHA.
func TestRequestPairingCode(t *testing.T) {
	t.Run("telefone inválido (curto) → InvalidArgument, nenhuma chamada à WAHA", func(t *testing.T) {
		repo := new(mockInstanceRepo)
		repo.On("GetInstanceByUserAndName", mock.Anything, "u1", "sess").
			Return(&models.Instance{ID: 1, InstanceID: "sess"}, nil)
		fake := whatsapp.NewFakeAdapter()
		h := NewWhatsAppHandler(fake, repo, nil, nil, nil, "")

		_, err := h.RequestPairingCode(contextWithUser("u1", "u1@example.com"),
			connect.NewRequest(&whatsappv1.RequestPairingCodeRequest{InstanceName: "sess", PhoneNumber: "123"}))
		require.Error(t, err)
		assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
		assert.Equal(t, 0, fake.CallCount("RequestPairingCode"))
	})

	t.Run("sessão de OUTRO usuário → NotFound, sem tocar a WAHA", func(t *testing.T) {
		repo := new(mockInstanceRepo)
		repo.On("GetInstanceByUserAndName", mock.Anything, "u1", "alheia").Return(nil, nil)
		fake := whatsapp.NewFakeAdapter()
		h := NewWhatsAppHandler(fake, repo, nil, nil, nil, "")

		_, err := h.RequestPairingCode(contextWithUser("u1", "u1@example.com"),
			connect.NewRequest(&whatsappv1.RequestPairingCodeRequest{InstanceName: "alheia", PhoneNumber: "5531999998888"}))
		require.Error(t, err)
		assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
		assert.Equal(t, 0, fake.CallCount("RequestPairingCode"))
	})

	t.Run("própria sessão + telefone com máscara → normaliza e devolve o código", func(t *testing.T) {
		repo := new(mockInstanceRepo)
		repo.On("GetInstanceByUserAndName", mock.Anything, "u1", "minha").
			Return(&models.Instance{ID: 7, InstanceID: "minha"}, nil)
		fake := whatsapp.NewFakeAdapter()
		fake.GetSessionStatusFunc = func(_ context.Context, _ string) (*whatsapp.SessionInfo, error) {
			return &whatsapp.SessionInfo{Name: "minha", Status: whatsapp.SessionStatusScanQR}, nil
		}
		var gotPhone string
		fake.RequestPairingCodeFunc = func(_ context.Context, _, phone string) (string, error) {
			gotPhone = phone
			return "3TM6-5LLH", nil
		}
		h := NewWhatsAppHandler(fake, repo, nil, nil, nil, "")

		resp, err := h.RequestPairingCode(contextWithUser("u1", "u1@example.com"),
			connect.NewRequest(&whatsappv1.RequestPairingCodeRequest{InstanceName: "minha", PhoneNumber: "+55 (31) 99999-8888"}))
		require.NoError(t, err)
		assert.True(t, resp.Msg.GetSuccess())
		assert.Equal(t, "3TM6-5LLH", resp.Msg.GetCode())
		assert.Equal(t, "5531999998888", gotPhone, "deve enviar só dígitos à WAHA")
	})

	t.Run("sessão não em SCAN_QR → Success=false não-pronto, sem tocar request-code", func(t *testing.T) {
		repo := new(mockInstanceRepo)
		repo.On("GetInstanceByUserAndName", mock.Anything, "u1", "minha").
			Return(&models.Instance{ID: 7, InstanceID: "minha"}, nil)
		fake := whatsapp.NewFakeAdapter()
		fake.GetSessionStatusFunc = func(_ context.Context, _ string) (*whatsapp.SessionInfo, error) {
			return &whatsapp.SessionInfo{Name: "minha", Status: whatsapp.SessionStatusConnected}, nil
		}
		h := NewWhatsAppHandler(fake, repo, nil, nil, nil, "")

		resp, err := h.RequestPairingCode(contextWithUser("u1", "u1@example.com"),
			connect.NewRequest(&whatsappv1.RequestPairingCodeRequest{InstanceName: "minha", PhoneNumber: "5531999998888"}))
		require.NoError(t, err)
		assert.False(t, resp.Msg.GetSuccess())
		assert.NotEmpty(t, resp.Msg.GetError(), "motivo deve chegar ao usuário via response.error")
		assert.Equal(t, 0, fake.CallCount("RequestPairingCode"))
	})

	t.Run("WAHA falha → Success=false com erro na resposta (não connect error)", func(t *testing.T) {
		repo := new(mockInstanceRepo)
		repo.On("GetInstanceByUserAndName", mock.Anything, "u1", "minha").
			Return(&models.Instance{ID: 7, InstanceID: "minha"}, nil)
		fake := whatsapp.NewFakeAdapter()
		fake.GetSessionStatusFunc = func(_ context.Context, _ string) (*whatsapp.SessionInfo, error) {
			return &whatsapp.SessionInfo{Name: "minha", Status: whatsapp.SessionStatusScanQR}, nil
		}
		fake.RequestPairingCodeFunc = func(_ context.Context, _, _ string) (string, error) {
			return "", errors.New("waha 422")
		}
		h := NewWhatsAppHandler(fake, repo, nil, nil, nil, "")

		resp, err := h.RequestPairingCode(contextWithUser("u1", "u1@example.com"),
			connect.NewRequest(&whatsappv1.RequestPairingCodeRequest{InstanceName: "minha", PhoneNumber: "5531999998888"}))
		require.NoError(t, err, "falha do WAHA vira response.error, não connect error")
		assert.False(t, resp.Msg.GetSuccess())
		assert.NotEmpty(t, resp.Msg.GetError())
	})

	t.Run("sem autenticação → Unauthenticated", func(t *testing.T) {
		h := NewWhatsAppHandler(whatsapp.NewFakeAdapter(), new(mockInstanceRepo), nil, nil, nil, "")
		_, err := h.RequestPairingCode(context.Background(),
			connect.NewRequest(&whatsappv1.RequestPairingCodeRequest{InstanceName: "x", PhoneNumber: "5531999998888"}))
		require.Error(t, err)
		assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
	})
}
