package monitoring

import (
	"context"
	"errors"
	"strings"
	"testing"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"
)

// ---------------------------------------------------------------------------
// Shein no espelho: substituição em vez de remoção.
//
// A Shein não tem API de geração de link, então a "conversão" é trocar o link
// da fonte pelo onelink fixo que o dono cadastrou. O que estes testes guardam é
// o opt-in DUPLO: sem loja marcada na automação, sem credencial ativa, ou sem
// repositório wirado, o comportamento tem que ser byte a byte o histórico —
// link bloqueado e mensagem só-Shein descartada.
// ---------------------------------------------------------------------------

const (
	sheinSourceLink = "https://br.shein.com/Vestido-Floral-p-12345678.html"
	sheinOwnerLink  = "https://onelink.shein.com/4/abc123xyz"
)

// mockSheinCredRepo embute a interface para implementar só o método que o
// espelho usa; qualquer outro chamado por engano estoura em vez de mentir.
// O contador prova a metade que o corpo não mostra: que o banco NÃO foi
// consultado quando a loja não está marcada.
type mockSheinCredRepo struct {
	repository.AffiliateCredentialsRepository
	fn    func(ctx context.Context, userID, store string) (*models.AffiliateCredentials, error)
	calls int
}

func (m *mockSheinCredRepo) GetByUserAndStore(ctx context.Context, userID, store string) (*models.AffiliateCredentials, error) {
	m.calls++
	if m.fn == nil {
		return nil, nil
	}
	return m.fn(ctx, userID, store)
}

func sheinCredRepo(link string) *mockSheinCredRepo {
	return &mockSheinCredRepo{
		fn: func(_ context.Context, _, _ string) (*models.AffiliateCredentials, error) {
			return &models.AffiliateCredentials{
				Store:       models.MonitorStoreShein,
				IsActive:    true,
				Credentials: map[string]string{"affiliateLink": link},
			}, nil
		},
	}
}

func sheinAutomation(stores ...string) *models.Automation {
	zero := 0
	sendAsImageOff := false
	return &models.Automation{
		ID: "auto-shein", UserID: "user-1", Name: "espelho shein",
		Stores:                stores,
		SendGroups:            []string{"dest@g.us"},
		Status:                "active",
		DelaySeconds:          &zero,
		MonitoringSendAsImage: &sendAsImageOff,
	}
}

func sheinMessage() *models.IncomingMessage {
	return &models.IncomingMessage{
		InstanceName: "inst1",
		RemoteJID:    "source@g.us",
		Body:         "Vestido lindo por R$ 39,90\n" + sheinSourceLink,
	}
}

// captureBody instala mocks de envio que guardam o texto entregue, seja qual
// for o caminho (texto puro ou card de preview).
func captureBody(s *monitoringService, body *string, sent *bool) {
	s.waProvider = &mockWAProvider{
		SendTextFn: func(_ context.Context, in whatsapp.SendTextInput) (*whatsapp.MessageResult, error) {
			*body, *sent = in.Text, true
			return &whatsapp.MessageResult{ID: "txt-1"}, nil
		},
		SendLinkCustomPreviewFn: func(_ context.Context, in whatsapp.SendLinkPreviewInput) (*whatsapp.MessageResult, error) {
			*body, *sent = in.Text, true
			return &whatsapp.MessageResult{ID: "card-1"}, nil
		},
		SendImageFn: func(_ context.Context, in whatsapp.SendImageInput) (*whatsapp.MessageResult, error) {
			*body, *sent = in.Caption, true
			return &whatsapp.MessageResult{ID: "img-1"}, nil
		},
	}
}

// O caminho feliz: loja marcada + credencial ativa => a mensagem que hoje
// SUMIRIA sai, com o link do dono no lugar do link da fonte.
func TestShein_OptedIn_ReplacesSourceLink(t *testing.T) {
	var body string
	var sent bool
	repo := sheinCredRepo(sheinOwnerLink)
	svc := newTestService(t, func(s *monitoringService) {
		s.affiliateCredRepo = repo
		captureBody(s, &body, &sent)
	})

	err := svc.processAutomation(context.Background(),
		sheinAutomation("shopee", "shein"), sheinMessage(), []string{sheinSourceLink}, false, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !sent {
		t.Fatal("esperava envio: mensagem só-Shein com credencial ativa deve ser espelhada")
	}
	if !strings.Contains(body, sheinOwnerLink) {
		t.Errorf("corpo não tem o link do dono.\nbody = %q", body)
	}
	if strings.Contains(body, sheinSourceLink) {
		t.Errorf("corpo AINDA tem o link da fonte — a comissão iria para quem postou.\nbody = %q", body)
	}
	if repo.calls != 1 {
		t.Errorf("GetByUserAndStore chamado %d vez(es), want 1", repo.calls)
	}
}

// Sem "shein" nas lojas da automação, nada muda: o link continua sendo
// removido e a mensagem só-Shein descartada. E o banco nem é consultado — o
// caminho quente não pode pagar uma query por causa de loja não marcada.
func TestShein_NotOptedIn_StaysBlockedAndSkipsLookup(t *testing.T) {
	var body string
	var sent bool
	repo := sheinCredRepo(sheinOwnerLink)
	svc := newTestService(t, func(s *monitoringService) {
		s.affiliateCredRepo = repo
		captureBody(s, &body, &sent)
	})

	err := svc.processAutomation(context.Background(),
		sheinAutomation("shopee"), sheinMessage(), []string{sheinSourceLink}, false, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sent {
		t.Errorf("não deveria enviar: automação não marcou Shein.\nbody = %q", body)
	}
	if repo.calls != 0 {
		t.Errorf("GetByUserAndStore chamado %d vez(es) sem a loja marcada, want 0", repo.calls)
	}
}

// Loja marcada mas credencial ausente (ou vazia): volta ao histórico.
func TestShein_NoCredential_StaysBlocked(t *testing.T) {
	cases := map[string]*mockSheinCredRepo{
		"credencial inexistente": {fn: func(_ context.Context, _, _ string) (*models.AffiliateCredentials, error) {
			return nil, nil
		}},
		"link em branco": {fn: func(_ context.Context, _, _ string) (*models.AffiliateCredentials, error) {
			return &models.AffiliateCredentials{Credentials: map[string]string{"affiliateLink": "   "}}, nil
		}},
		"erro de leitura": {fn: func(_ context.Context, _, _ string) (*models.AffiliateCredentials, error) {
			return nil, errors.New("boom")
		}},
	}

	for name, repo := range cases {
		t.Run(name, func(t *testing.T) {
			var body string
			var sent bool
			svc := newTestService(t, func(s *monitoringService) {
				s.affiliateCredRepo = repo
				captureBody(s, &body, &sent)
			})

			err := svc.processAutomation(context.Background(),
				sheinAutomation("shopee", "shein"), sheinMessage(), []string{sheinSourceLink}, false, nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if sent {
				t.Errorf("não deveria enviar sem link de afiliado utilizável.\nbody = %q", body)
			}
		})
	}
}

// Serviço sem o repositório wirado = feature ausente. É esta a garantia de que
// um deploy sem a Option se comporta exatamente como antes.
func TestShein_NilRepo_IsInert(t *testing.T) {
	var body string
	var sent bool
	svc := newTestService(t, func(s *monitoringService) {
		s.affiliateCredRepo = nil
		captureBody(s, &body, &sent)
	})

	err := svc.processAutomation(context.Background(),
		sheinAutomation("shopee", "shein"), sheinMessage(), []string{sheinSourceLink}, false, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sent {
		t.Errorf("repo nil tem que manter o comportamento histórico.\nbody = %q", body)
	}
}

// Shopee + Shein na mesma mensagem: a Shopee converte pela API e a Shein pela
// substituição, sem uma atrapalhar a outra. Guarda contra o link Shein entrar
// como substituição E como remoção ao mesmo tempo.
func TestShein_AlongsideShopee_BothConverted(t *testing.T) {
	const shopeeSource = "https://shopee.com.br/product-i.123.456"
	var body string
	var sent bool
	svc := newTestService(t, func(s *monitoringService) {
		s.affiliateCredRepo = sheinCredRepo(sheinOwnerLink)
		s.shopeeService = shopeeConvertAndEnrich("Produto XYZ", 50.0, 100.0, false)
		captureBody(s, &body, &sent)
	})

	msg := sheinMessage()
	msg.Body = "Combo\n" + shopeeSource + "\n" + sheinSourceLink

	err := svc.processAutomation(context.Background(),
		sheinAutomation("shopee", "shein"), msg, []string{shopeeSource, sheinSourceLink}, false, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !sent {
		t.Fatal("esperava envio")
	}
	if strings.Contains(body, sheinSourceLink) {
		t.Errorf("link Shein da fonte sobreviveu.\nbody = %q", body)
	}
	if !strings.Contains(body, sheinOwnerLink) {
		t.Errorf("link Shein do dono ausente.\nbody = %q", body)
	}
	if strings.Contains(body, shopeeSource) {
		t.Errorf("link Shopee da fonte sobreviveu.\nbody = %q", body)
	}
}

// ---------------------------------------------------------------------------
// Unidades
// ---------------------------------------------------------------------------

func TestSheinLinksIn(t *testing.T) {
	svc := newTestService(t)

	cases := []struct {
		name string
		link string
		want bool
	}{
		{"site brasileiro", "https://br.shein.com/produto-p-1.html", true},
		{"onelink do afiliado", sheinOwnerLink, true},
		{"dominio raiz", "https://shein.com/x", true},
		{"spoof com sufixo colado", "https://shein.com.evil.test/x", false},
		{"outra loja", "https://shopee.com.br/product-i.1.2", false},
		{"nao comercial", "https://example.com/x", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := len(svc.sheinLinksIn([]string{tc.link})) == 1
			if got != tc.want {
				t.Errorf("sheinLinksIn(%q) reconheceu = %v, want %v", tc.link, got, tc.want)
			}
		})
	}
}

func TestSheinAffiliateFor(t *testing.T) {
	if got := sheinAffiliateFor(nil, sheinSourceLink); got != "" {
		t.Errorf("credencial nil devolveu %q, want vazio", got)
	}

	cred := &models.AffiliateCredentials{
		Credentials: map[string]string{"affiliateLink": "  " + sheinOwnerLink + "  "},
	}
	if got := sheinAffiliateFor(cred, sheinSourceLink); got != sheinOwnerLink {
		t.Errorf("sheinAffiliateFor = %q, want %q (com trim)", got, sheinOwnerLink)
	}
}
