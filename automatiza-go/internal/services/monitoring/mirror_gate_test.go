package monitoring

import (
	"context"
	"errors"
	"testing"

	"automatiza-go/internal/lib/usergate"
	"automatiza-go/internal/models"
)

// TestGateFechadoNaoConsultaORepositorio é o que garante "gate OFF = byte a
// byte" TAMBÉM em custo: o caminho quente recebe mensagem sem link o tempo
// todo, e com o gate fechado ele não pode pagar uma query a mais por mensagem.
func TestGateFechadoNaoConsultaORepositorio(t *testing.T) {
	svc := &monitoringService{
		instanceRepo: &mockInstanceRepo{
			GetInstanceByNameFn: func(context.Context, string) (*models.Instance, error) {
				t.Error("gate fechado não pode consultar o repositório")
				return nil, nil
			},
		},
		// mirrorContent no valor zero = globalmente fechado
	}
	msg := &models.IncomingMessage{InstanceName: "chip-1"}

	if svc.mirrorCommonAllowed(context.Background(), msg) {
		t.Error("gate fechado não pode autorizar espelho")
	}
}

// TestGateAbertoResolveODonoDoChip: o gate é por usuário e o webhook não traz
// user_id, então o dono vem do chip que entregou a mensagem.
func TestGateAbertoResolveODonoDoChip(t *testing.T) {
	g, err := usergate.Parse("all")
	if err != nil {
		t.Fatal(err)
	}
	svc := &monitoringService{
		instanceRepo:  instanceOwnedBy("user-1"),
		mirrorContent: g,
	}
	if !svc.mirrorCommonAllowed(context.Background(), &models.IncomingMessage{InstanceName: "chip-1"}) {
		t.Error("gate 'all' deveria autorizar")
	}
}

// TestDonoNaoResolvidoNega: um gate que abre quando não sabe de quem é a
// mensagem não é um gate.
func TestDonoNaoResolvidoNega(t *testing.T) {
	g, _ := usergate.Parse("all")
	casos := map[string]*mockInstanceRepo{
		"erro de banco": {GetInstanceByNameFn: func(context.Context, string) (*models.Instance, error) {
			return nil, errors.New("timeout")
		}},
		"instância sem dono": {GetInstanceByNameFn: func(context.Context, string) (*models.Instance, error) {
			return &models.Instance{UserID: ""}, nil
		}},
		"instância inexistente": {GetInstanceByNameFn: func(context.Context, string) (*models.Instance, error) {
			return nil, nil
		}},
	}
	for nome, repo := range casos {
		svc := &monitoringService{instanceRepo: repo, mirrorContent: g}
		if svc.mirrorCommonAllowed(context.Background(), &models.IncomingMessage{InstanceName: "x"}) {
			t.Errorf("%s deveria negar", nome)
		}
	}
}

// TestAllowlistSoAutorizaOCanario prova o rollout canário por usuário.
func TestAllowlistSoAutorizaOCanario(t *testing.T) {
	const canario = "11111111-1111-1111-1111-111111111111"
	const outro = "22222222-2222-2222-2222-222222222222"
	g, err := usergate.Parse(canario)
	if err != nil {
		t.Fatal(err)
	}

	svcCanario := &monitoringService{instanceRepo: instanceOwnedBy(canario), mirrorContent: g}
	if !svcCanario.mirrorCommonAllowed(context.Background(), &models.IncomingMessage{InstanceName: "x"}) {
		t.Error("o canário deveria ser autorizado")
	}
	svcOutro := &monitoringService{instanceRepo: instanceOwnedBy(outro), mirrorContent: g}
	if svcOutro.mirrorCommonAllowed(context.Background(), &models.IncomingMessage{InstanceName: "x"}) {
		t.Error("usuário fora da allowlist não pode ser autorizado")
	}
}

// instanceOwnedBy devolve um repositório que resolve qualquer chip para o dono
// informado.
func instanceOwnedBy(userID string) *mockInstanceRepo {
	return &mockInstanceRepo{
		GetInstanceByNameFn: func(context.Context, string) (*models.Instance, error) {
			return &models.Instance{UserID: userID}, nil
		},
	}
}
