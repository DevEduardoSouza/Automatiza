package connectrpc

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"automatiza-go/internal/auth"
	affiliatesv1 "automatiza-go/internal/gen/affiliates/v1"
	"automatiza-go/internal/models"
	"automatiza-go/internal/services/awin"

	"connectrpc.com/connect"
)

type stubCredentialsRepo struct {
	upserted *models.AffiliateCredentials
}

func (s *stubCredentialsRepo) GetByUserAndStore(_ context.Context, _, _ string) (*models.AffiliateCredentials, error) {
	return s.upserted, nil
}
func (s *stubCredentialsRepo) GetAllByUser(_ context.Context, _ string) ([]models.AffiliateCredentials, error) {
	return nil, nil
}
func (s *stubCredentialsRepo) Upsert(_ context.Context, _, _ string, data *models.AffiliateCredentials) error {
	s.upserted = data
	return nil
}
func (s *stubCredentialsRepo) Delete(_ context.Context, _, _ string) error     { return nil }
func (s *stubCredentialsRepo) Deactivate(_ context.Context, _, _ string) error { return nil }
func (s *stubCredentialsRepo) GetActiveStores(_ context.Context, _ string) ([]string, error) {
	return nil, nil
}

// Secret errado salvo no cadastro só aparece DEPOIS, como rajada de 10020
// queimando listas (incidente 11/07). O Upsert precisa validar AO VIVO e
// rejeitar credencial que a Shopee não reconhece.
func TestUpsertCredential_ShopeeValidation(t *testing.T) {
	newReq := func() *connect.Request[affiliatesv1.UpsertCredentialRequest] {
		return connect.NewRequest(&affiliatesv1.UpsertCredentialRequest{
			Store:       affiliatesv1.AffiliateStore_AFFILIATE_STORE_SHOPEE,
			Credentials: `{"appId":"18330220029","password":"secret-de-teste-32-caracteres-ok"}`,
		})
	}
	ctxWithUser := context.WithValue(context.Background(), UserContextKey, &auth.User{ID: "user-1"})

	t.Run("secret invalido → salvamento REJEITADO", func(t *testing.T) {
		repo := &stubCredentialsRepo{}
		h := &AffiliateCredentialsHandler{
			credentialsRepo: repo,
			validateShopee: func(_ context.Context, _, _ string) error {
				return fmt.Errorf("credenciais Shopee invalidas")
			},
		}
		_, err := h.UpsertCredential(ctxWithUser, newReq())
		if err == nil {
			t.Fatal("expected save to be rejected when Shopee says Invalid Signature")
		}
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("expected CodeInvalidArgument, got %v", connect.CodeOf(err))
		}
		if repo.upserted != nil {
			t.Error("invalid credentials must NOT be persisted")
		}
	})

	t.Run("secret valido → salva normalmente", func(t *testing.T) {
		repo := &stubCredentialsRepo{}
		h := &AffiliateCredentialsHandler{
			credentialsRepo: repo,
			validateShopee:  func(_ context.Context, _, _ string) error { return nil },
		}
		if _, err := h.UpsertCredential(ctxWithUser, newReq()); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if repo.upserted == nil {
			t.Fatal("valid credentials must be persisted")
		}
	})

	// O GET devolve secrets mascarados; um save que traz a máscara de volta
	// (ou vazio) tem que PRESERVAR o secret armazenado — nunca sobrescrever
	// com a máscara (incidente 11/07: secret válido virou inválido sem
	// ninguém digitar credencial).
	t.Run("secret mascarado no save → preserva o armazenado", func(t *testing.T) {
		repo := &stubCredentialsRepo{upserted: &models.AffiliateCredentials{
			Store:       "shopee",
			Credentials: map[string]string{"appId": "18330220029", "password": "secret-real-armazenado"},
		}}
		var validatedSecret string
		h := &AffiliateCredentialsHandler{
			credentialsRepo: repo,
			validateShopee: func(_ context.Context, _, secret string) error {
				validatedSecret = secret
				return nil
			},
		}
		req := connect.NewRequest(&affiliatesv1.UpsertCredentialRequest{
			Store:       affiliatesv1.AffiliateStore_AFFILIATE_STORE_SHOPEE,
			Credentials: `{"appId":"18330220029","password":"••••••••"}`,
		})
		if _, err := h.UpsertCredential(ctxWithUser, req); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := repo.upserted.Credentials["password"]; got != "secret-real-armazenado" {
			t.Errorf("masked save must keep stored secret, persisted %q", got)
		}
		if validatedSecret != "secret-real-armazenado" {
			t.Errorf("live validation must run on the MERGED real secret, got %q", validatedSecret)
		}
	})

	// A resposta do GET/List nunca pode carregar o secret em claro.
	t.Run("credentialToProto mascara o secret", func(t *testing.T) {
		proto := credentialToProto(&models.AffiliateCredentials{
			Store:       "shopee",
			Credentials: map[string]string{"appId": "18330220029", "password": "secret-real"},
		})
		if proto == nil {
			t.Fatal("nil proto")
		}
		if want := `"password":"` + secretMask + `"`; !strings.Contains(proto.Credentials, want) {
			t.Errorf("expected masked password in %s", proto.Credentials)
		}
		if strings.Contains(proto.Credentials, "secret-real") {
			t.Error("raw secret must NEVER leave the API")
		}
		if !strings.Contains(proto.Credentials, "18330220029") {
			t.Error("appId must stay visible")
		}
	})
}

// ---------------------------------------------------------------------------
// Lojas que ganharam formulário depois (Amazon, AliExpress, Awin)
// ---------------------------------------------------------------------------

func ctxUser() context.Context {
	return context.WithValue(context.Background(), UserContextKey, &auth.User{ID: "user-1"})
}

// Os segredos novos não podem voltar em claro para o navegador — era assim que
// o autofill sobrescrevia credencial válida (incidente 11/07).
func TestCredentialToProto_MascaraSegredosNovos(t *testing.T) {
	cases := []struct {
		key    string
		masked bool
	}{
		{"oauth2Token", true},
		{"appSecret", true},
		{"secretKey", true},
		{"clientSecret", true},
		// Identificadores NÃO são mascarados: o usuário precisa conferi-los.
		{"awinId", false},
		{"appKey", false},
		{"tag", false},
	}
	for _, tc := range cases {
		t.Run(tc.key, func(t *testing.T) {
			out := credentialToProto(&models.AffiliateCredentials{
				Store:       "awin",
				Credentials: map[string]string{tc.key: "valor-real-secreto"},
			})
			if tc.masked && strings.Contains(out.GetCredentials(), "valor-real-secreto") {
				t.Errorf("%s vazou em claro: %s", tc.key, out.GetCredentials())
			}
			if !tc.masked && !strings.Contains(out.GetCredentials(), "valor-real-secreto") {
				t.Errorf("%s deveria continuar visível: %s", tc.key, out.GetCredentials())
			}
		})
	}
}

// O mapa domínio→awinmid é interno: some da resposta em vez de ser mascarado
// (mascarar faria o próximo save gravar "••••••••" por cima do mapa real).
func TestCredentialToProto_RemoveChaveInterna(t *testing.T) {
	out := credentialToProto(&models.AffiliateCredentials{
		Store: "awin",
		Credentials: map[string]string{
			"awinId":     "112233",
			"programmes": `{"netshoes.com.br":"9988"}`,
		},
	})
	if strings.Contains(out.GetCredentials(), "programmes") {
		t.Errorf("chave interna vazou: %s", out.GetCredentials())
	}
	if strings.Contains(out.GetCredentials(), secretMask) {
		t.Errorf("chave interna foi mascarada em vez de removida: %s", out.GetCredentials())
	}
}

// Bug que só apareceria no SEGUNDO save: a chave interna não volta do
// formulário, e sem preservá-la o mapa de programas seria apagado.
func TestUpsertCredential_PreservaMapaDeProgramas(t *testing.T) {
	repo := &stubCredentialsRepo{upserted: &models.AffiliateCredentials{
		Store: "awin",
		Credentials: map[string]string{
			"awinId":     "112233",
			"programmes": `{"netshoes.com.br":"9988"}`,
		},
	}}
	h := &AffiliateCredentialsHandler{credentialsRepo: repo}

	_, err := h.UpsertCredential(ctxUser(), connect.NewRequest(&affiliatesv1.UpsertCredentialRequest{
		Store:       affiliatesv1.AffiliateStore_AFFILIATE_STORE_AWIN,
		Credentials: `{"awinId":"112233","oauth2Token":"tok"}`,
	}))
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if got := repo.upserted.GetCredential("programmes"); got != `{"netshoes.com.br":"9988"}` {
		t.Errorf("mapa de programas = %q — foi apagado no segundo save", got)
	}
}

// Awin: token recusado é determinístico e bloqueia o cadastro, com mensagem
// acionável. Instabilidade da rede NÃO bloqueia (mesmo contrato da Shopee).
func TestUpsertCredential_AwinValidation(t *testing.T) {
	newReq := func() *connect.Request[affiliatesv1.UpsertCredentialRequest] {
		return connect.NewRequest(&affiliatesv1.UpsertCredentialRequest{
			Store:       affiliatesv1.AffiliateStore_AFFILIATE_STORE_AWIN,
			Credentials: `{"awinId":"112233","oauth2Token":"tok"}`,
		})
	}

	t.Run("token recusado → REJEITADO", func(t *testing.T) {
		h := &AffiliateCredentialsHandler{
			credentialsRepo: &stubCredentialsRepo{},
			fetchAwinProgrammes: func(_ context.Context, _, _ string) ([]awin.Programme, error) {
				return nil, awin.ErrInvalidCredentials
			},
		}
		_, err := h.UpsertCredential(ctxUser(), newReq())
		if err == nil || connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Fatalf("erro = %v, queria InvalidArgument", err)
		}
	})

	t.Run("Awin fora do ar → salva assim mesmo", func(t *testing.T) {
		h := &AffiliateCredentialsHandler{
			credentialsRepo: &stubCredentialsRepo{},
			fetchAwinProgrammes: func(_ context.Context, _, _ string) ([]awin.Programme, error) {
				return nil, fmt.Errorf("awin: HTTP 502")
			},
		}
		if _, err := h.UpsertCredential(ctxUser(), newReq()); err != nil {
			t.Fatalf("instabilidade não pode bloquear o cadastro: %v", err)
		}
	})

	t.Run("sucesso → grava mapa e nomes na credencial", func(t *testing.T) {
		repo := &stubCredentialsRepo{}
		h := &AffiliateCredentialsHandler{
			credentialsRepo: repo,
			fetchAwinProgrammes: func(_ context.Context, _, _ string) ([]awin.Programme, error) {
				return []awin.Programme{{ID: "9988", Name: "Netshoes", DisplayURL: "netshoes.com.br"}}, nil
			},
		}
		if _, err := h.UpsertCredential(ctxUser(), newReq()); err != nil {
			t.Fatalf("erro inesperado: %v", err)
		}
		if got := repo.upserted.GetCredential("programmes"); !strings.Contains(got, "9988") {
			t.Errorf("mapa não foi gravado: %q", got)
		}
		if got := repo.upserted.GetCredential("programme_names"); !strings.Contains(got, "Netshoes") {
			t.Errorf("nomes não foram gravados: %q", got)
		}
	})
}

func TestListAwinProgrammes(t *testing.T) {
	t.Run("sem credencial salva → FailedPrecondition", func(t *testing.T) {
		h := &AffiliateCredentialsHandler{
			credentialsRepo:     &stubCredentialsRepo{},
			fetchAwinProgrammes: func(_ context.Context, _, _ string) ([]awin.Programme, error) { return nil, nil },
		}
		_, err := h.ListAwinProgrammes(ctxUser(), connect.NewRequest(&affiliatesv1.ListAwinProgrammesRequest{}))
		if err == nil || connect.CodeOf(err) != connect.CodeFailedPrecondition {
			t.Fatalf("erro = %v, queria FailedPrecondition", err)
		}
	})

	t.Run("token recusado → InvalidArgument", func(t *testing.T) {
		h := &AffiliateCredentialsHandler{
			credentialsRepo: &stubCredentialsRepo{upserted: &models.AffiliateCredentials{
				Store:       "awin",
				Credentials: map[string]string{"awinId": "112233", "oauth2Token": "ruim"},
			}},
			fetchAwinProgrammes: func(_ context.Context, _, _ string) ([]awin.Programme, error) {
				return nil, awin.ErrInvalidCredentials
			},
		}
		_, err := h.ListAwinProgrammes(ctxUser(), connect.NewRequest(&affiliatesv1.ListAwinProgrammesRequest{}))
		if err == nil || connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Fatalf("erro = %v, queria InvalidArgument", err)
		}
	})

	t.Run("sucesso → lista e persiste o mapa", func(t *testing.T) {
		repo := &stubCredentialsRepo{upserted: &models.AffiliateCredentials{
			Store:       "awin",
			Credentials: map[string]string{"awinId": "112233", "oauth2Token": "ok"},
		}}
		h := &AffiliateCredentialsHandler{
			credentialsRepo: repo,
			fetchAwinProgrammes: func(_ context.Context, _, _ string) ([]awin.Programme, error) {
				return []awin.Programme{
					{ID: "9988", Name: "Netshoes", DisplayURL: "netshoes.com.br"},
					{ID: "5544", Name: "Dafiti", DisplayURL: "dafiti.com.br"},
				}, nil
			},
		}
		res, err := h.ListAwinProgrammes(ctxUser(), connect.NewRequest(&affiliatesv1.ListAwinProgrammesRequest{}))
		if err != nil {
			t.Fatalf("erro inesperado: %v", err)
		}
		if n := len(res.Msg.GetProgrammes()); n != 2 {
			t.Fatalf("%d programas, queria 2", n)
		}
		if got := repo.upserted.GetCredential("programmes"); !strings.Contains(got, "9988") {
			t.Errorf("o mapa não foi persistido: %q", got)
		}
	})
}
