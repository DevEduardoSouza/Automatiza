package aliexpress

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

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

func repoOK() *fakeRepo {
	return &fakeRepo{creds: &models.AffiliateCredentials{
		Store: storeKey,
		Credentials: map[string]string{
			"appKey":     "12345",
			"appSecret":  "segredo",
			"trackingId": "default",
		},
	}}
}

func convert(t *testing.T, repo repository.AffiliateCredentialsRepository, baseURL, raw string) (string, error) {
	t.Helper()
	u, err := commerceurl.Parse(raw)
	if err != nil {
		t.Fatalf("Parse(%q): %v", raw, err)
	}
	// cache nil: o conversor precisa funcionar sem Redis (fail-open de cache).
	return New(repo, nil, zap.NewNop(), baseURL).Convert(context.Background(), u, "user-1")
}

// TestSignEhDeterministicaEIgnoraSign congela o algoritmo de assinatura: é o
// único ponto da integração que falha em silêncio (assinatura errada volta
// como erro de negócio genérico, não como exceção).
func TestSignEhDeterministicaEIgnoraSign(t *testing.T) {
	params := map[string]string{
		"method":    "aliexpress.affiliate.link.generate",
		"app_key":   "12345",
		"timestamp": "1700000000000",
	}
	first := sign(params, "segredo")
	if first == "" {
		t.Fatal("assinatura vazia")
	}
	// Hex maiúsculo de SHA-256: 64 caracteres.
	if len(first) != 64 {
		t.Errorf("len = %d, queria 64", len(first))
	}
	if first != sign(params, "segredo") {
		t.Error("assinatura não é determinística")
	}
	// O próprio campo sign não entra no cálculo — senão o valor mudaria a cada
	// tentativa de reassinar o mesmo request.
	params[signParam] = "LIXO"
	if got := sign(params, "segredo"); got != first {
		t.Error("o campo sign contaminou o cálculo")
	}
	// Secret diferente tem de mudar a assinatura.
	if sign(params, "outro") == first {
		t.Error("o secret não afeta a assinatura")
	}
}

// TestConvertExtraiPromotionLink cobre o caminho feliz e prova que o request
// sai assinado, com o tracking da fonte já removido do source_values.
func TestConvertExtraiPromotionLink(t *testing.T) {
	var gotForm url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		gotForm = r.Form
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"aliexpress_affiliate_link_generate_response": {
				"resp_result": {"resp_code": 200, "result": {"promotion_links": {
					"promotion_link": [{"promotion_link": "https://s.click.aliexpress.com/e/_DONO"}]
				}}}
			}
		}`))
	}))
	defer srv.Close()

	got, err := convert(t, repoOK(), srv.URL,
		"https://pt.aliexpress.com/item/1005006123456789.html?utm_source=fonte&aff_fcid=xyz")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if got != "https://s.click.aliexpress.com/e/_DONO" {
		t.Errorf("link = %q", got)
	}
	if gotForm.Get("sign") == "" {
		t.Error("request saiu sem assinatura")
	}
	if gotForm.Get("method") != apiMethod {
		t.Errorf("method = %q", gotForm.Get("method"))
	}
	if gotForm.Get("tracking_id") != "default" {
		t.Errorf("tracking_id = %q", gotForm.Get("tracking_id"))
	}
	source := gotForm.Get("source_values")
	if source != "https://pt.aliexpress.com/item/1005006123456789.html" {
		t.Errorf("source_values = %q — o tracking da fonte tem de sumir", source)
	}
}

// Erro determinístico de assinatura NÃO pode virar indisponibilidade: é
// credencial recusada, que aciona o cooldown e pede ação do usuário.
func TestConvertAssinaturaInvalidaEhCredencialRecusada(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"error_response":{"code":"15","msg":"Invalid signature","sub_code":"isv.invalid-signature"}}`))
	}))
	defer srv.Close()

	_, err := convert(t, repoOK(), srv.URL, "https://pt.aliexpress.com/item/1005006123456789.html")
	if !errors.Is(err, affiliatelink.ErrRejectedCredential) {
		t.Errorf("erro = %v, queria ErrRejectedCredential", err)
	}
}

// 5xx é transitório: vale retry, e o breaker conta.
func TestConvert5xxEhIndisponibilidade(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	_, err := convert(t, repoOK(), srv.URL, "https://pt.aliexpress.com/item/1005006123456789.html")
	if !errors.Is(err, affiliatelink.ErrUnavailable) {
		t.Errorf("erro = %v, queria ErrUnavailable", err)
	}
}

// 200 sem promotion_link não pode virar link vazio no corpo do cliente.
func TestConvertRespostaSemLinkFalha(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"aliexpress_affiliate_link_generate_response":{"resp_result":{"resp_code":500}}}`))
	}))
	defer srv.Close()

	got, err := convert(t, repoOK(), srv.URL, "https://pt.aliexpress.com/item/1005006123456789.html")
	if err == nil {
		t.Fatalf("esperava erro, veio %q", got)
	}
	if got != "" {
		t.Errorf("link = %q, queria vazio", got)
	}
}

func TestConvertSemCredencialFalha(t *testing.T) {
	cases := map[string]*models.AffiliateCredentials{
		"sem appSecret": {Store: storeKey, Credentials: map[string]string{"appKey": "1", "trackingId": "d"}},
		"sem appKey":    {Store: storeKey, Credentials: map[string]string{"appSecret": "s", "trackingId": "d"}},
		"sem tracking":  {Store: storeKey, Credentials: map[string]string{"appKey": "1", "appSecret": "s"}},
	}
	for name, creds := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := convert(t, &fakeRepo{creds: creds}, "http://unused.test",
				"https://pt.aliexpress.com/item/1005006123456789.html")
			if !errors.Is(err, affiliatelink.ErrNoCredentials) {
				t.Errorf("erro = %v, queria ErrNoCredentials", err)
			}
		})
	}
}

// O breaker precisa abrir depois de falhas seguidas para não empilhar timeout
// de 8s no scheduler serial.
func TestBreakerAbreDepoisDeFalhas(t *testing.T) {
	b := newBreaker(2, time.Hour)
	if !b.allow() {
		t.Fatal("breaker nasce fechado")
	}
	b.recordFailure()
	if !b.allow() {
		t.Fatal("uma falha não pode abrir")
	}
	b.recordFailure()
	if b.allow() {
		t.Error("duas falhas deveriam abrir o circuito")
	}
}
