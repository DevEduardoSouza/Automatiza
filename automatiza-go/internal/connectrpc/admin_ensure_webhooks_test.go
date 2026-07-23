package connectrpc

import (
	"context"
	"sync"
	"testing"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/auth"
	adminv1 "automatiza-go/internal/gen/admin/v1"
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Fakes mínimos — só o que EnsureWebhooks toca
// =============================================================================

type fakeAdminRoleRepo struct{ admin bool }

func (f *fakeAdminRoleRepo) GetUserRole(context.Context, string) (*models.UserRole, error) {
	return nil, nil
}
func (f *fakeAdminRoleRepo) SetUserRole(context.Context, string, string) error    { return nil }
func (f *fakeAdminRoleRepo) RemoveUserRole(context.Context, string, string) error { return nil }
func (f *fakeAdminRoleRepo) IsAdmin(context.Context, string) (bool, error)        { return f.admin, nil }
func (f *fakeAdminRoleRepo) GetAllUserRoles(context.Context, string) ([]string, error) {
	return nil, nil
}
func (f *fakeAdminRoleRepo) GetAdminUsers(context.Context) ([]models.UserRole, error) {
	return nil, nil
}

// fakeConnectedInstanceRepo embute a interface: só GetAllConnectedInstances é
// exercitada, e qualquer outro método chamado por engano estoura nil pointer em
// vez de passar despercebido.
type fakeConnectedInstanceRepo struct {
	repository.InstanceRepository
	instances []models.Instance
}

func (f *fakeConnectedInstanceRepo) GetAllConnectedInstances(context.Context) ([]models.Instance, error) {
	return f.instances, nil
}

func TestEnsureWebhooks_UsesCanonicalEventList(t *testing.T) {
	var (
		mu     sync.Mutex
		inputs []whatsapp.SetWebhookInput
	)

	provider := whatsapp.NewFakeAdapter()
	provider.SetWebhookFunc = func(_ context.Context, in whatsapp.SetWebhookInput) error {
		mu.Lock()
		defer mu.Unlock()
		inputs = append(inputs, in)
		return nil
	}

	h := &AdminHandler{
		userRoleRepo: &fakeAdminRoleRepo{admin: true},
		instanceRepo: &fakeConnectedInstanceRepo{instances: []models.Instance{
			{ID: 1, InstanceID: "session-a"},
			{ID: 2, InstanceID: "session-b"},
		}},
		waProvider:     provider,
		webhookBaseURL: "http://server:3000",
	}

	ctx := context.WithValue(context.Background(), UserContextKey, &auth.User{ID: "admin-user"})
	resp, err := h.EnsureWebhooks(ctx, connect.NewRequest(&adminv1.EnsureWebhooksRequest{}))
	require.NoError(t, err)
	assert.Equal(t, int32(2), resp.Msg.GetOk())

	require.Len(t, inputs, 2)
	for _, in := range inputs {
		// A lista precisa ser EXATAMENTE a canônica: quando o admin reaplica
		// webhooks em massa, a config por-sessão SUBSTITUI a anterior inteira —
		// uma lista própria aqui removeria silenciosamente eventos dos quais o
		// backend depende (fromMe do offer_generate, tracking de membros).
		assert.Equal(t, whatsapp.DefaultWebhookEvents(), in.Events)
		assert.Contains(t, in.Events, "group.v2.participants")
		assert.Contains(t, in.Events, "message.any")
		assert.NotContains(t, in.Events, "message")
		assert.Equal(t, "http://server:3000/api/v1/webhooks/waha", in.URL)
	}
}
