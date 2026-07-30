package awin

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"go.uber.org/zap"

	"automatiza-go/internal/lib/commerceurl"
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"
	"automatiza-go/internal/services/affiliatelink"
)

type fakeRepo struct {
	repository.AffiliateCredentialsRepository
	creds *models.AffiliateCredentials
}

func (f *fakeRepo) GetByUserAndStore(_ context.Context, _, _ string) (*models.AffiliateCredentials, error) {
	return f.creds, nil
}

func repoWith(publisherID string, domains map[string]string) *fakeRepo {
	encoded, _ := EncodeDomainMap(domains)
	return &fakeRepo{creds: &models.AffiliateCredentials{
		Store: storeKey,
		Credentials: map[string]string{
			"awinId":                publisherID,
			ProgrammesCredentialKey: encoded,
		},
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

// TestConvertMontaDeeplink é a checagem executável do conversor: o destino tem
// de chegar percent-encoded dentro de ued, e o tracking da fonte tem de sumir.
func TestConvertMontaDeeplink(t *testing.T) {
	repo := repoWith("112233", map[string]string{"netshoes.com.br": "9988"})

	got, err := convert(t, repo,
		"https://www.netshoes.com.br/produto/tenis-corrida-123?utm_source=fonte&cor=preto")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatalf("deeplink ilegivel: %v", err)
	}
	if parsed.Scheme+"://"+parsed.Host+parsed.Path != redirectBase {
		t.Errorf("base = %q, queria %q", got, redirectBase)
	}
	q := parsed.Query()
	if q.Get("awinmid") != "9988" {
		t.Errorf("awinmid = %q, queria 9988", q.Get("awinmid"))
	}
	if q.Get("awinaffid") != "112233" {
		t.Errorf("awinaffid = %q, queria 112233", q.Get("awinaffid"))
	}
	if q.Get("clickref") != clickRef {
		t.Errorf("clickref = %q, queria %q", q.Get("clickref"), clickRef)
	}
	// O destino preserva o que é do produto (cor) e perde o tracking da fonte.
	ued := q.Get("ued")
	if ued != "https://www.netshoes.com.br/produto/tenis-corrida-123?cor=preto" {
		t.Errorf("ued = %q", ued)
	}
	// O ued precisa estar ESCAPADO no link cru, senão a Awin corta no primeiro &.
	if !strings.Contains(got, "ued=https%3A%2F%2F") {
		t.Errorf("ued não foi percent-encoded:\n%s", got)
	}
}

// Deeplink recebido do grupo-fonte carrega o awinaffid de OUTRO afiliado:
// precisa ser reatribuído, não repassado.
func TestConvertReatribuiDeeplinkDeTerceiro(t *testing.T) {
	repo := repoWith("112233", map[string]string{"dafiti.com.br": "5544"})
	origem := "https://www.awin1.com/cread.php?awinmid=5544&awinaffid=999999&ued=" +
		url.QueryEscape("https://www.dafiti.com.br/produto/bota-1")

	got, err := convert(t, repo, origem)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	q, _ := url.Parse(got)
	if v := q.Query().Get("awinaffid"); v != "112233" {
		t.Errorf("awinaffid = %q — o afiliado da fonte sobreviveu", v)
	}
	if v := q.Query().Get("ued"); v != "https://www.dafiti.com.br/produto/bota-1" {
		t.Errorf("ued = %q", v)
	}
}

// Marca fora do mapa = usuário não aprovado naquele programa. Fail-closed:
// deeplink com awinmid errado gera clique sem comissão.
func TestConvertMarcaForaDoProgramaFalha(t *testing.T) {
	repo := repoWith("112233", map[string]string{"netshoes.com.br": "9988"})
	_, err := convert(t, repo, "https://www.dafiti.com.br/produto/bota-1")
	if !errors.Is(err, affiliatelink.ErrUnconvertible) {
		t.Errorf("erro = %v, queria ErrUnconvertible", err)
	}
}

func TestConvertSemCredencialFalha(t *testing.T) {
	cases := map[string]repository.AffiliateCredentialsRepository{
		"sem linha":          &fakeRepo{},
		"sem publisher":      repoWith("", map[string]string{"netshoes.com.br": "1"}),
		"sem mapa sincroniz": repoWith("112233", nil),
	}
	for name, repo := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := convert(t, repo, "https://www.netshoes.com.br/produto/1")
			if !errors.Is(err, affiliatelink.ErrNoCredentials) {
				t.Errorf("erro = %v, queria ErrNoCredentials", err)
			}
		})
	}
}

// O ued de um deeplink de terceiro é conteúdo não confiável: revalidar impede
// que um destino forjado (host spoof) saia carimbado como nosso.
func TestConvertUedForjadoNaoPassa(t *testing.T) {
	repo := repoWith("112233", map[string]string{"netshoes.com.br": "9988"})
	origem := "https://www.awin1.com/cread.php?awinmid=1&awinaffid=2&ued=" +
		url.QueryEscape("https://netshoes.com.br.evil.test/produto/1")

	if got, err := convert(t, repo, origem); err == nil {
		t.Errorf("destino forjado virou deeplink: %q", got)
	}
}

func TestFetchProgrammes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer token-ok" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.URL.Query().Get("relationship") != "joined" {
			t.Errorf("relationship = %q, queria joined", r.URL.Query().Get("relationship"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"id": 9988, "name": "Netshoes", "displayUrl": "https://www.netshoes.com.br"},
			{"id": 5544, "name": "Dafiti", "displayUrl": "", "clickThroughUrl": "https://www.dafiti.com.br/"},
			{"id": 0, "name": "Lixo", "displayUrl": "https://x.test"}
		]`))
	}))
	defer srv.Close()

	got, err := FetchProgrammes(context.Background(), srv.URL, "112233", "token-ok")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("%d programas, queria 2 (id 0 é descartado)", len(got))
	}
	domains := DomainMap(got)
	if domains["netshoes.com.br"] != "9988" {
		t.Errorf("netshoes = %q", domains["netshoes.com.br"])
	}
	// displayUrl vazio cai para clickThroughUrl — senão a marca fica sem chave.
	if domains["dafiti.com.br"] != "5544" {
		t.Errorf("dafiti = %q", domains["dafiti.com.br"])
	}
}

func TestFetchProgrammesCredencialRuim(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	if _, err := FetchProgrammes(context.Background(), srv.URL, "112233", "token-ruim"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("erro = %v, queria ErrInvalidCredentials", err)
	}
}

// 5xx é instabilidade da Awin, não credencial errada: não pode bloquear o
// cadastro do usuário (mesmo contrato fail-open da Shopee).
func TestFetchProgrammesInstabilidadeNaoEhCredencialRuim(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	_, err := FetchProgrammes(context.Background(), srv.URL, "112233", "token-ok")
	if err == nil {
		t.Fatal("esperava erro")
	}
	if errors.Is(err, ErrInvalidCredentials) {
		t.Error("5xx não pode ser tratado como credencial invalida")
	}
}
