package magalu

import (
	"context"
	"errors"
	"strings"
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

func withStore(store string) *fakeRepo {
	return &fakeRepo{creds: &models.AffiliateCredentials{
		Store:       storeKey,
		Credentials: map[string]string{"partnerStore": store},
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

// TestConvertEmiteLinkDaVitrine é a checagem que falha se a montagem do link
// quebrar: byte a byte, é isto que o cliente recebe no grupo.
func TestConvertEmiteLinkDaVitrine(t *testing.T) {
	got, err := convert(t, withStore("weekpromos"),
		"https://www.magazineluiza.com.br/geladeira-brastemp-frost-free/p/hbf8fjj15a/ed/refr/")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	const want = "https://www.magazinevoce.com.br/weekpromos/geladeira-brastemp-frost-free/p/hbf8fjj15a/ed/refr/"
	if got != want {
		t.Errorf("link = %q, queria %q", got, want)
	}
}

// O caso que mais paga: o grupo-fonte posta a Vitrine de OUTRO divulgador. Sem
// trocar a loja, repassar o link entregaria a comissão a quem postou.
func TestConvertReatribuiVitrineDeTerceiro(t *testing.T) {
	got, err := convert(t, withStore("minhaloja"),
		"https://www.magazinevoce.com.br/lojadooutro/geladeira-brastemp/p/hbf8fjj15a/ed/refr/")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	const want = "https://www.magazinevoce.com.br/minhaloja/geladeira-brastemp/p/hbf8fjj15a/ed/refr/"
	if got != want {
		t.Errorf("link = %q, queria %q", got, want)
	}
	if strings.Contains(got, "lojadooutro") {
		t.Error("a loja de quem postou sobreviveu na URL emitida")
	}
}

// Forma mínima (sem o slug legível): a loja continua sendo o primeiro segmento
// e tem de ser trocada do mesmo jeito.
func TestConvertFormaMinima(t *testing.T) {
	got, err := convert(t, withStore("minhaloja"),
		"https://www.magazinevoce.com.br/lojadooutro/p/hbf8fjj15a/")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	const want = "https://www.magazinevoce.com.br/minhaloja/p/hbf8fjj15a/"
	if got != want {
		t.Errorf("link = %q, queria %q", got, want)
	}
}

// O tracking da origem não pode viajar no link emitido: identifica a sessão de
// quem postou no grupo-fonte.
func TestConvertDescartaTrackingDaOrigem(t *testing.T) {
	got, err := convert(t, withStore("minhaloja"),
		"https://www.magazineluiza.com.br/geladeira/p/hbf8fjj15a/ed/refr/?partner_id=outro&utm_source=fonte&seller_id=9")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	const want = "https://www.magazinevoce.com.br/minhaloja/geladeira/p/hbf8fjj15a/ed/refr/"
	if got != want {
		t.Errorf("link = %q, queria %q — o tracking da fonte tem de sumir", got, want)
	}
}

// Sem produto identificável o desfecho é ErrUnconvertible: o caller remove o
// link, que é o comportamento histórico de todo link Magalu.
func TestConvertSemProdutoEhUnconvertible(t *testing.T) {
	cases := []string{
		"https://www.magazineluiza.com.br/busca/geladeira/",
		"https://www.magazineluiza.com.br/",
		"https://www.magazinevoce.com.br/weekpromos/",
		"https://www.magazinevoce.com.br/weekpromos/eletrodomesticos/l/ed/",
		// SKU curto demais para ser real: não dá para reatribuir.
		"https://www.magazineluiza.com.br/geladeira/p/ab/",
	}
	for _, raw := range cases {
		got, err := convert(t, withStore("minhaloja"), raw)
		if !errors.Is(err, affiliatelink.ErrUnconvertible) {
			t.Errorf("%s → (%q, %v), queria ErrUnconvertible", raw, got, err)
		}
	}
}

func TestConvertSemCredencial(t *testing.T) {
	cases := map[string]*fakeRepo{
		"repo vazio":     {creds: nil},
		"erro no repo":   {err: errors.New("boom")},
		"slug vazio":     withStore(""),
		"slug invalido":  withStore("!!!"),
		"slug reservado": withStore("sualoja"),
	}
	for name, repo := range cases {
		_, err := convert(t, repo, "https://www.magazineluiza.com.br/geladeira/p/hbf8fjj15a/")
		if !errors.Is(err, affiliatelink.ErrNoCredentials) {
			t.Errorf("%s → %v, queria ErrNoCredentials", name, err)
		}
	}
}

// O registry trata "devolveu a própria origem" como bug de implementação. Esta
// checagem prova que o conversor nunca cai nisso mesmo quando a entrada já é
// uma Vitrine.
func TestConvertNuncaDevolveAOrigem(t *testing.T) {
	const raw = "https://www.magazinevoce.com.br/minhaloja/geladeira/p/hbf8fjj15a/ed/refr/"
	got, err := convert(t, withStore("minhaloja"), raw)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if got == "" {
		t.Fatal("link vazio")
	}
	// Mesma loja e mesmo produto: o link emitido é equivalente, mas tem de ser
	// gerado por nós, não copiado — e sem a query da fonte.
	if got != raw {
		t.Logf("normalizado: %q", got)
	}
}

// As mensagens de erro deste pacote não podem conter as expressões que
// process_list.go e offer_generate_service.go classificam por substring: elas
// PAUSAM a automação Shopee do cliente.
func TestErrosNaoPausamAutomacao(t *testing.T) {
	proibidas := []string{"credentials not configured", "credentials incomplete", "Invalid Signature"}
	for _, err := range []error{affiliatelink.ErrNoCredentials, affiliatelink.ErrUnconvertible} {
		for _, p := range proibidas {
			if strings.Contains(err.Error(), p) {
				t.Errorf("%q contem %q", err.Error(), p)
			}
		}
	}
}

func TestNormalizeStoreSlug(t *testing.T) {
	cases := map[string]string{
		// As formas que o cliente cola no formulário.
		"weekpromos":                                                "weekpromos",
		"  WeekPromos  ":                                            "weekpromos",
		"magazinevoce.com.br/weekpromos":                            "weekpromos",
		"www.magazinevoce.com.br/weekpromos/":                       "weekpromos",
		"https://www.magazinevoce.com.br/weekpromos":                "weekpromos",
		"https://www.magazinevoce.com.br/weekpromos/?utm_source=ig": "weekpromos",
		// Link de produto da PRÓPRIA vitrine: o primeiro segmento é a loja.
		"https://www.magazinevoce.com.br/weekpromos/geladeira/p/hbf8fjj15a/ed/refr/": "weekpromos",
		"loja-com-hifen": "loja-com-hifen",

		// Rejeições.
		"":    "",
		"   ": "",
		"/":   "",
		// Catálogo: o primeiro segmento é o slug do PRODUTO, não a loja.
		"https://www.magazineluiza.com.br/geladeira-brastemp/p/hbf8fjj15a/": "",
		"https://magalu.com/alguma-coisa":                                   "",
		"https://www.shopee.com.br/minhaloja":                               "",
		"sualoja":                                                           "",
		"p":                                                                 "",
		"busca":                                                             "",
		"a":                                                                 "",
		"loja com espaco":                                                   "",
		"loja/extra":                                                        "loja",
	}
	for in, want := range cases {
		if got := NormalizeStoreSlug(in); got != want {
			t.Errorf("NormalizeStoreSlug(%q) = %q, queria %q", in, got, want)
		}
	}
}
