package amazon

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/zap"

	"automatiza-go/internal/lib/commerceurl"
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"
	"automatiza-go/internal/services/affiliatelink"
)

// fakeRepo devolve uma credencial fixa; o resto da interface não é exercitado.
type fakeRepo struct {
	repository.AffiliateCredentialsRepository
	creds *models.AffiliateCredentials
	err   error
}

func (f *fakeRepo) GetByUserAndStore(_ context.Context, _, _ string) (*models.AffiliateCredentials, error) {
	return f.creds, f.err
}

func withTag(tag string) *fakeRepo {
	return &fakeRepo{creds: &models.AffiliateCredentials{
		Store:       storeKey,
		Credentials: map[string]string{"tag": tag},
	}}
}

func convert(t *testing.T, repo repository.AffiliateCredentialsRepository, raw string) (string, error) {
	t.Helper()
	u, err := commerceurl.Parse(raw)
	if err != nil {
		t.Fatalf("Parse(%q): %v", raw, err)
	}
	return New(repo, zap.NewNop()).Convert(context.Background(), u, "user-1")
}

// TestConvertEmiteLinkCanonico é a checagem que falha se a montagem do link
// quebrar: byte a byte, é isto que o cliente recebe no grupo.
func TestConvertEmiteLinkCanonico(t *testing.T) {
	got, err := convert(t, withTag("automatisa-20"),
		"https://www.amazon.com.br/Echo-Dot-5a-geracao/dp/B09B8V1LZ3/ref=sr_1_1?keywords=echo&qid=123")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	const want = "https://www.amazon.com.br/dp/B09B8V1LZ3?tag=automatisa-20"
	if got != want {
		t.Errorf("link = %q, queria %q", got, want)
	}
}

// O tracking da origem (ref=, keywords, qid) não pode viajar no link emitido:
// é assinatura da sessão de quem postou no grupo-fonte.
func TestConvertDescartaTrackingDaOrigem(t *testing.T) {
	got, err := convert(t, withTag("cliente-20"),
		"https://www.amazon.com.br/dp/B09B8V1LZ3?tag=OUTRO-AFILIADO&utm_source=fonte")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	const want = "https://www.amazon.com.br/dp/B09B8V1LZ3?tag=cliente-20"
	if got != want {
		t.Errorf("link = %q, queria %q — a tag da fonte tem de sumir", got, want)
	}
}

// A tag é por marketplace: reescrever tudo para .com.br mandaria o comprador
// para outra loja e a venda não creditaria.
func TestConvertPreservaMarketplaceDaOrigem(t *testing.T) {
	got, err := convert(t, withTag("cliente-20"), "https://www.amazon.com/dp/B09B8V1LZ3")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	const want = "https://www.amazon.com/dp/B09B8V1LZ3?tag=cliente-20"
	if got != want {
		t.Errorf("link = %q, queria %q", got, want)
	}
}

// Sem ASIN não há URL canônica: melhor o link sumir do corpo do que sair um
// link que não converte (ou pior, o original com a comissão da fonte).
func TestConvertSemASINFalha(t *testing.T) {
	for _, raw := range []string{
		"https://amzn.to/3xYzAbC",
		"https://www.amazon.com.br/s?k=echo+dot",
		"https://www.amazon.com.br/stores/page/ABC123",
	} {
		if got, err := convert(t, withTag("cliente-20"), raw); !errors.Is(err, affiliatelink.ErrUnconvertible) {
			t.Errorf("Convert(%q) = %q/%v, queria ErrUnconvertible", raw, got, err)
		}
	}
}

func TestConvertSemCredencialFalha(t *testing.T) {
	cases := map[string]repository.AffiliateCredentialsRepository{
		"sem linha":  &fakeRepo{},
		"tag vazia":  withTag(""),
		"só espaços": withTag("   "),
	}
	for name, repo := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := convert(t, repo, "https://www.amazon.com.br/dp/B09B8V1LZ3")
			if !errors.Is(err, affiliatelink.ErrNoCredentials) {
				t.Errorf("erro = %v, queria ErrNoCredentials", err)
			}
		})
	}
}

// A grafia snake_case é a legada do JSONB; precisa continuar sendo lida.
func TestConvertAceitaPartnerTagLegado(t *testing.T) {
	repo := &fakeRepo{creds: &models.AffiliateCredentials{
		Store:       storeKey,
		Credentials: map[string]string{"partner_tag": "legado-20"},
	}}
	got, err := convert(t, repo, "https://www.amazon.com.br/dp/B09B8V1LZ3")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	const want = "https://www.amazon.com.br/dp/B09B8V1LZ3?tag=legado-20"
	if got != want {
		t.Errorf("link = %q, queria %q", got, want)
	}
}
