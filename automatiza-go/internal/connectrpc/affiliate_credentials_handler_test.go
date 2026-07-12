package connectrpc

import (
	"context"
	"fmt"
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
}
