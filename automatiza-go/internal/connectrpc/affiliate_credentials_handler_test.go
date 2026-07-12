package connectrpc

import (
	"context"
	"fmt"
	"strings"
	"testing"

	affiliatesv1 "automatiza-go/internal/gen/affiliates/v1"
	"automatiza-go/internal/auth"
	"automatiza-go/internal/models"

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
