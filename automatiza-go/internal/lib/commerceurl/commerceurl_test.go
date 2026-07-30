package commerceurl

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"automatiza-go/internal/models"
)

func mustParse(t *testing.T, raw string) URL {
	t.Helper()
	u, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse(%q) inesperadamente falhou: %v", raw, err)
	}
	return u
}

// TestParseRejeitaHostForjado cobre a matriz de §13.2: toda variação de host
// falso precisa virar security_block, nunca "loja" e nunca non_commerce
// relayável.
func TestParseRejeitaHostForjado(t *testing.T) {
	blocked := []struct {
		name string
		raw  string
	}{
		{"path com nome de loja", "https://evil.test/shopee.com.br/x"},
		{"sufixo colado", "https://shopee.com.br.evil.test/produto"},
		{"path com meli.la", "https://evil.test/meli.la/x"},
		{"userinfo spoof", "https://shopee.com.br@evil.test/"},
		{"ip literal v4", "https://93.184.216.34/produto"},
		{"ip literal v6 loopback", "https://[::1]/produto"},
		{"label vazio", "https://shopee..com.br/x"},
		{"porta inesperada", "https://shopee.com.br:8443/x"},
		{"porta http inesperada", "http://shopee.com.br:8080/x"},
		{"scheme file", "file:///etc/passwd"},
		{"scheme gopher", "gopher://shopee.com.br/x"},
		{"query com host de loja", "https://evil.test/r?to=shopee.com.br/x"},
		{"controle embutido", "https://shopee.com.br/\x00x"},
	}

	for _, tc := range blocked {
		t.Run(tc.name, func(t *testing.T) {
			u, err := Parse(tc.raw)
			if err == nil {
				t.Fatalf("esperava bloqueio, veio store=%q host=%q", u.Store, u.Host)
			}
			e, ok := AsError(err)
			if !ok {
				t.Fatalf("erro não tipado: %v", err)
			}
			if e.Class != models.ErrorClassSecurityBlock {
				t.Fatalf("classe = %q, queria security_block", e.Class)
			}
			if strings.Contains(e.Error(), "evil.test/") || strings.Contains(e.Error(), "?") {
				t.Fatalf("mensagem de erro vazou a URL: %s", e.Error())
			}
		})
	}
}

func TestParseURLLonga(t *testing.T) {
	long := "https://shopee.com.br/" + strings.Repeat("a", MaxURLBytes)
	_, err := Parse(long)
	e, ok := AsError(err)
	if !ok || e.Reason != models.ReasonURLTooLong {
		t.Fatalf("esperava url_too_long, veio %v", err)
	}
	// Exatamente no limite passa.
	exact := "https://shopee.com.br/" + strings.Repeat("a", MaxURLBytes-len("https://shopee.com.br/"))
	if len(exact) != MaxURLBytes {
		t.Fatalf("fixture errada: %d", len(exact))
	}
	if _, err := Parse(exact); err != nil {
		t.Fatalf("URL no limite deveria passar: %v", err)
	}
}

func TestStoreOfFronteiraDePonto(t *testing.T) {
	cases := map[string]string{
		"shopee.com.br":                  models.MonitorStoreShopee,
		"s.shopee.com.br":                models.MonitorStoreShopee,
		"shope.ee":                       models.MonitorStoreShopee,
		"shp.ee":                         models.MonitorStoreShopee,
		"www.mercadolivre.com.br":        models.MonitorStoreMercadoLivr,
		"produto.mercadolivre.com.br":    models.MonitorStoreMercadoLivr,
		"meli.la":                        models.MonitorStoreMercadoLivr,
		"amazon.com.br":                  models.MonitorStoreAmazon,
		"www.amazon.com.br":              models.MonitorStoreAmazon,
		"amzn.to":                        models.MonitorStoreAmazon,
		"pt.aliexpress.com":              models.MonitorStoreAliExpress,
		"s.click.aliexpress.com":         models.MonitorStoreAliExpress,
		"www.awin1.com":                  models.MonitorStoreAwin,
		"netshoes.com.br":                models.MonitorStoreAwin,
		"www.dafiti.com.br":              models.MonitorStoreAwin,
		"magazinevoce.com.br":            models.MonitorStoreMagalu,
		"www.magazineluiza.com.br":       models.MonitorStoreMagalu,
		"magalu.com":                     models.MonitorStoreMagalu,
		"shein.com":                      models.MonitorStoreShein,
		"xshopee.com.br":                 "",
		"shopee.com.br.evil.test":        "",
		"notmeli.la":                     "",
		"instagram.com":                  "",
		"mercadolivre.com.br.phish.test": "",
		// Anunciante Awin fora de allCommerceHosts: host forjado NÃO vira
		// loja, e continua sendo relayado como link comum (não security_block).
		"netshoes.com.br.evil.test": "",
	}
	for host, want := range cases {
		if got := StoreOf(host); got != want {
			t.Errorf("StoreOf(%q) = %q, queria %q", host, got, want)
		}
	}
}

func TestParseNormalizaHost(t *testing.T) {
	u := mustParse(t, "https://WWW.Shopee.COM.BR./product/1/2")
	if u.Host != "www.shopee.com.br" {
		t.Fatalf("host = %q", u.Host)
	}
	if u.Store != models.MonitorStoreShopee {
		t.Fatalf("store = %q", u.Store)
	}
}

func TestParseAceitaNonCommerce(t *testing.T) {
	u := mustParse(t, "https://instagram.com/canal")
	if u.Store != "" {
		t.Fatalf("store = %q, queria vazio", u.Store)
	}
	if c := ClassifyLocal(u); c.Class != models.URLClassNonCommerce {
		t.Fatalf("class = %q", c.Class)
	}
}

func TestExtractShopeeIDsSomenteDoPath(t *testing.T) {
	cases := []struct {
		raw        string
		shop, item int64
		ok         bool
	}{
		{"https://shopee.com.br/product/123/456", 123, 456, true},
		{"https://shopee.com.br/slug-do-produto/123/456", 123, 456, true},
		{"https://shopee.com.br/Camiseta-i.123.456", 123, 456, true},
		{"https://shopee.com.br/x?shop=123&item=456", 0, 0, false},
		{"https://shopee.com.br/product/0/456", 0, 0, false},
		{"https://shopee.com.br/mall/oficial", 0, 0, false},
	}
	for _, tc := range cases {
		u := mustParse(t, tc.raw)
		shop, item, ok := ExtractShopeeIDs(u.Parsed)
		if ok != tc.ok || (ok && (shop != tc.shop || item != tc.item)) {
			t.Errorf("%s → (%d,%d,%v), queria (%d,%d,%v)", tc.raw, shop, item, ok, tc.shop, tc.item, tc.ok)
		}
	}
}

func TestExtractMLBIDSomenteDoPath(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{"https://produto.mercadolivre.com.br/MLB-123456-foo", "MLB123456"},
		{"https://www.mercadolivre.com.br/p/MLB987654", "MLB987654"},
		{"https://www.mercadolivre.com.br/up/MLB111", "MLB111"},
		{"https://www.mercadolivre.com.br/x?redirect=MLB123", ""},
		{"https://www.mercadolivre.com.br/x#MLB123", ""},
		{"https://www.mercadolivre.com.br/ofertas-mlb-do-dia", ""},
	}
	for _, tc := range cases {
		u := mustParse(t, tc.raw)
		got, _ := ExtractMLBID(u.Parsed)
		if got != tc.want {
			t.Errorf("%s → %q, queria %q", tc.raw, got, tc.want)
		}
	}
}

func TestClassifyLocalCampanhas(t *testing.T) {
	cases := []struct {
		raw   string
		class string
	}{
		{"https://www.mercadolivre.com.br/cupons", models.URLClassCampaignOrCoupon},
		{"https://www.mercadolivre.com.br/cupons/abc", models.URLClassCampaignOrCoupon},
		{"https://www.mercadolivre.com.br/l/promocoes", models.URLClassCampaignOrCoupon},
		{"https://www.mercadolivre.com.br/social/canal", models.URLClassUnknown},
		{"https://shopee.com.br/mall/x", models.URLClassCampaignOrCoupon},
		{"https://shopee.com.br/flash_sale", models.URLClassCampaignOrCoupon},
		{"https://shopee.com.br/algum-path", models.URLClassUnknown},
		{"https://s.shopee.com.br/abc123", models.URLClassUnknown},
		// Magalu: "1" não é SKU (mínimo 4 alfanuméricos), então a URL não
		// identifica produto — unknown, nunca campanha. Com SKU real, produto.
		{"https://magazinevoce.com.br/loja/p/1", models.URLClassUnknown},
		{"https://magazinevoce.com.br/loja/p/hbf8fjj15a/", models.URLClassProduct},
		{"https://www.magazineluiza.com.br/geladeira-brastemp/p/hbf8fjj15a/ed/refr/", models.URLClassProduct},
		{"https://www.magazineluiza.com.br/busca/geladeira/", models.URLClassUnknown},
		// ASIN tem 10 caracteres: "B01" não identifica produto, então não dá
		// para reatribuir a comissão — unknown, nunca campanha.
		{"https://amazon.com.br/dp/B01", models.URLClassUnknown},
		{"https://www.amazon.com.br/Echo-Dot/dp/B09B8V1LZ3", models.URLClassProduct},
		{"https://amzn.to/3xYzAbC", models.URLClassUnknown},
		{"https://pt.aliexpress.com/item/1005006123456789.html", models.URLClassProduct},
		{"https://s.click.aliexpress.com/e/_abc", models.URLClassUnknown},
		{"https://www.netshoes.com.br/produto/tenis-123", models.URLClassProduct},
		{"https://www.awin1.com/cread.php?awinmid=1&awinaffid=2&ued=x", models.URLClassUnknown},
	}
	for _, tc := range cases {
		u := mustParse(t, tc.raw)
		if got := ClassifyLocal(u).Class; got != tc.class {
			t.Errorf("%s → %q, queria %q", tc.raw, got, tc.class)
		}
	}
}

func TestClassifyLocalCartEhPermanente(t *testing.T) {
	u := mustParse(t, "https://shopee.com.br/cart/xyz")
	c := ClassifyLocal(u)
	if c.ErrorClass != models.ErrorClassPermanentConte || c.Reason != models.ReasonCart {
		t.Fatalf("cart → %+v", c)
	}
	if c.Class == models.URLClassCampaignOrCoupon {
		t.Fatal("cart jamais pode virar campanha")
	}
}

func TestStripTrackingPreservaFuncional(t *testing.T) {
	u := mustParse(t, "https://shopee.com.br/product/1/2?utm_source=x&af_id=y&mmp_pid=z&quantity=3&variation=44")
	got := StripTracking(u.Parsed)
	parsed, _ := url.Parse(got)
	q := parsed.Query()
	for _, gone := range []string{"utm_source", "af_id", "mmp_pid"} {
		if q.Has(gone) {
			t.Errorf("tracking %q sobreviveu", gone)
		}
	}
	for _, kept := range []string{"quantity", "variation"} {
		if !q.Has(kept) {
			t.Errorf("parâmetro funcional %q foi removido sem prova", kept)
		}
	}
}

func TestIsPublicIP(t *testing.T) {
	blocked := []string{
		"127.0.0.1", "10.0.0.5", "192.168.1.1", "172.16.0.1",
		"169.254.169.254", "0.0.0.0", "224.0.0.1", "100.64.0.1",
		"::1", "fe80::1", "fc00::1", "ff02::1",
	}
	for _, s := range blocked {
		if IsPublicIP(net.ParseIP(s)) {
			t.Errorf("%s deveria ser recusado", s)
		}
	}
	for _, s := range []string{"93.184.216.34", "8.8.8.8", "2606:2800:220:1:248:1893:25c8:1946"} {
		if !IsPublicIP(net.ParseIP(s)) {
			t.Errorf("%s deveria ser aceito", s)
		}
	}
}

// TestSafeControlBloqueiaLoopback prova que o guard de IP recusa a conexão
// mesmo quando o host resolve para loopback (rebinding DNS).
func TestSafeControlBloqueiaLoopback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	// Cliente SEM dialer injetado: usa o safeControl real.
	c := NewSafeClient(ClientOptions{Timeout: 3 * time.Second, FollowRedirects: true})
	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	if _, err := c.Do(req); err == nil {
		t.Fatal("conexão a loopback deveria falhar")
	}
}

// loopbackDialer redireciona qualquer host para o servidor de teste, como o
// httptest exige, sem afrouxar a validação de host/redirect do cliente.
func loopbackDialer(addr string) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, _ string) (net.Conn, error) {
		var d net.Dialer
		return d.DialContext(ctx, network, addr)
	}
}

// TestRedirectEscapaParaOutroHost prova que o segundo servidor nunca é
// alcançado quando o redirect sai da loja.
func TestRedirectCrossStoreBloqueado(t *testing.T) {
	var forasteiroHits int32
	forasteiro := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&forasteiroHits, 1)
		w.WriteHeader(200)
	}))
	defer forasteiro.Close()

	origem := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://evil.test/x", http.StatusFound)
	}))
	defer origem.Close()

	origemAddr := strings.TrimPrefix(origem.URL, "http://")
	c := NewSafeClient(ClientOptions{
		Store:           models.MonitorStoreShopee,
		Timeout:         3 * time.Second,
		FollowRedirects: true,
		DialContext:     loopbackDialer(origemAddr),
	})
	req, _ := http.NewRequest(http.MethodGet, "http://s.shopee.com.br/abc", nil)
	resp, err := c.Do(req)
	if err == nil {
		resp.Body.Close()
		t.Fatal("redirect cross-store deveria falhar")
	}
	if atomic.LoadInt32(&forasteiroHits) != 0 {
		t.Fatalf("servidor forasteiro recebeu %d requests", forasteiroHits)
	}
}

// TestRedirectDowngradeHTTPBloqueado prova a exigência de HTTPS após o
// primeiro salto.
func TestRedirectDowngradeHTTPBloqueado(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://shopee.com.br/product/1/2", http.StatusFound)
	}))
	defer srv.Close()

	c := NewSafeClient(ClientOptions{
		Store:           models.MonitorStoreShopee,
		Timeout:         3 * time.Second,
		FollowRedirects: true,
		DialContext:     loopbackDialer(strings.TrimPrefix(srv.URL, "http://")),
	})
	req, _ := http.NewRequest(http.MethodGet, "http://s.shopee.com.br/abc", nil)
	resp, err := c.Do(req)
	if err == nil {
		resp.Body.Close()
		t.Fatal("downgrade para http deveria falhar")
	}
	e, ok := AsError(err)
	if !ok || e.Reason != models.ReasonRedirectEscape {
		t.Fatalf("reason = %v, queria redirect_escape", err)
	}
}

// TestRedirectApagaCredencialAoTrocarDeHost prova que cookie/CSRF não seguem
// para outro host da MESMA loja.
func TestRedirectApagaCredencialAoTrocarDeHost(t *testing.T) {
	var sawCookie int32
	destino := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Cookie") != "" || r.Header.Get("X-Csrf-Token") != "" {
			atomic.StoreInt32(&sawCookie, 1)
		}
		w.WriteHeader(200)
	}))
	defer destino.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/salto", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://produto.mercadolivre.com.br/MLB-1", http.StatusFound)
	})
	mux.HandleFunc("/MLB-1", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Cookie") != "" || r.Header.Get("X-Csrf-Token") != "" {
			atomic.StoreInt32(&sawCookie, 1)
		}
		w.WriteHeader(200)
	})
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()

	c := NewSafeClient(ClientOptions{
		Store:           models.MonitorStoreMercadoLivr,
		Timeout:         3 * time.Second,
		FollowRedirects: true,
		DialContext:     loopbackDialer(strings.TrimPrefix(srv.URL, "https://")),
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	})
	req, _ := http.NewRequest(http.MethodGet, "https://www.mercadolivre.com.br/salto", nil)
	req.Header.Set("Cookie", "sessao=secreta")
	req.Header.Set("X-Csrf-Token", "csrf-secreto")
	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("request falhou: %v", err)
	}
	resp.Body.Close()
	if atomic.LoadInt32(&sawCookie) == 1 {
		t.Fatal("credencial vazou no salto para outro host")
	}
}

func TestAllowsMLCredentials(t *testing.T) {
	ok := mustParse(t, "https://www.mercadolivre.com.br/p/MLB1")
	if !AllowsMLCredentials(ok) {
		t.Fatal("host exato deveria aceitar credencial")
	}
	naoOk := mustParse(t, "https://outro.mercadolivre.com.br/p/MLB1")
	if AllowsMLCredentials(naoOk) {
		t.Fatal("subdomínio fora da allowlist não pode receber credencial")
	}
	httpOnly := mustParse(t, "http://www.mercadolivre.com.br/p/MLB1")
	if AllowsMLCredentials(httpOnly) {
		t.Fatal("http nunca recebe credencial")
	}
}

func TestReadLimited(t *testing.T) {
	if _, err := ReadLimited(strings.NewReader(strings.Repeat("a", MaxBodyBytes+1))); err == nil {
		t.Fatal("corpo acima do teto deveria falhar")
	}
	buf, err := ReadLimited(strings.NewReader(strings.Repeat("a", MaxBodyBytes)))
	if err != nil || len(buf) != MaxBodyBytes {
		t.Fatalf("corpo no limite: len=%d err=%v", len(buf), err)
	}
}

func TestClassifyHTTPStatus(t *testing.T) {
	cases := map[int][2]string{
		200: {models.ErrorClassNone, models.ReasonNone},
		302: {models.ErrorClassTransient, models.ReasonHTTP3xx},
		408: {models.ErrorClassTransient, models.ReasonHTTP408},
		425: {models.ErrorClassTransient, models.ReasonHTTP425},
		429: {models.ErrorClassTransient, models.ReasonHTTP429},
		404: {models.ErrorClassPermanentConte, models.ReasonNotFound},
		410: {models.ErrorClassPermanentConte, models.ReasonNotFound},
		401: {models.ErrorClassPermanentConfig, models.ReasonInvalidToken},
		422: {models.ErrorClassPermanentConte, models.ReasonHTTP4xx},
		500: {models.ErrorClassTransient, models.ReasonHTTP5xx},
		503: {models.ErrorClassTransient, models.ReasonHTTP5xx},
	}
	for status, want := range cases {
		c, r := ClassifyHTTPStatus(status)
		if c != want[0] || r != want[1] {
			t.Errorf("%d → (%s,%s), queria (%s,%s)", status, c, r, want[0], want[1])
		}
	}
}

// TestClassifyTransportErrorPreservaSecurityBlock prova que um bloqueio de
// segurança não é rebaixado a transient por vir embrulhado num url.Error.
func TestClassifyTransportErrorPreservaSecurityBlock(t *testing.T) {
	wrapped := &url.Error{Op: "Get", URL: "https://x", Err: blockErr(models.ReasonPrivateAddress, "h")}
	class, reason := ClassifyTransportError(wrapped)
	if class != models.ErrorClassSecurityBlock || reason != models.ReasonPrivateAddress {
		t.Fatalf("(%s,%s) — bloqueio virou transient", class, reason)
	}
}

func TestTodosOsReasonCodesSaoValidos(t *testing.T) {
	for _, r := range models.AllReasonCodes() {
		if !models.IsValidReasonCode(r) {
			t.Errorf("reason %q não passa na própria validação", r)
		}
	}
	if models.IsValidReasonCode("inventado") {
		t.Error("reason fora do enum foi aceito")
	}
}

// TestAmazonASIN guarda o contrato do extrator: só path, ASIN de 10 chars.
// Emitir link sem ASIN válido significaria mandar uma URL que não converte.
func TestAmazonASIN(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{"https://www.amazon.com.br/Echo-Dot-5a-geracao/dp/B09B8V1LZ3/ref=sr_1_1?keywords=echo", "B09B8V1LZ3"},
		{"https://www.amazon.com.br/dp/B09B8V1LZ3", "B09B8V1LZ3"},
		{"https://www.amazon.com.br/gp/product/B09B8V1LZ3", "B09B8V1LZ3"},
		{"https://www.amazon.com.br/gp/aw/d/B09B8V1LZ3", "B09B8V1LZ3"},
		{"https://www.amazon.com.br/dp/b09b8v1lz3", "B09B8V1LZ3"},
		{"https://amzn.to/3xYzAbC", ""},
		{"https://www.amazon.com.br/s?k=echo+dot", ""},
		{"https://www.amazon.com.br/stores/page/ABC", ""},
		{"https://www.amazon.com.br/dp/TOO_SHORT", ""},
		// ASIN em query não conta: só o path identifica produto.
		{"https://www.amazon.com.br/qualquer?asin=B09B8V1LZ3", ""},
	}
	for _, tc := range cases {
		u := mustParse(t, tc.raw)
		got, ok := AmazonASIN(u.Parsed)
		if tc.want == "" {
			if ok {
				t.Errorf("AmazonASIN(%q) = %q, queria falha", tc.raw, got)
			}
			continue
		}
		if !ok || got != tc.want {
			t.Errorf("AmazonASIN(%q) = %q/%v, queria %q", tc.raw, got, ok, tc.want)
		}
	}
}

// TestAntiSpoofSegueValendoComLojasNovas prova que quebrar unsupportedHosts em
// tabelas por loja não afrouxou a varredura de token — Amazon e AliExpress
// continuam em allCommerceHosts.
func TestAntiSpoofSegueValendoComLojasNovas(t *testing.T) {
	blocked := []string{
		"https://amazon.com.br.evil.test/dp/B09B8V1LZ3",
		"https://evil.test/amazon.com.br/dp/B09B8V1LZ3",
		"https://aliexpress.com.phish.test/item/1.html",
		"https://shopee.com.br.evil.test/product/1/2",
	}
	for _, raw := range blocked {
		if _, err := Parse(raw); err == nil {
			t.Errorf("Parse(%q) passou, queria bloqueio de spoof", raw)
		}
	}
	// Anunciante Awin fica FORA de allCommerceHosts de propósito: uma URL que
	// só MENCIONA netshoes.com.br continua sendo relayada como link comum.
	if _, err := Parse("https://www.google.com/url?q=https://netshoes.com.br/x"); err != nil {
		t.Errorf("URL que menciona anunciante Awin foi bloqueada: %v", err)
	}
}
