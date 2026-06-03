package mercadolivre

import (
	"math"
	"testing"
)

// HTML sintético espelhando a estrutura REAL da página /social do afiliado:
// o objeto price tem previous_price (original), current_price (ATUAL), discount
// ({"value":31} = % de desconto) e installments ({"value":31.83} = parcela).
// O parser DEVE pegar current_price (190.98), nunca o discount (31) nem a
// parcela (31.83) — regressão real vista em prod. Há um 2º card depois pra
// garantir que pegamos o preço do PRIMEIRO card (produto-alvo).
const socialFixtureDiscount = `<html><head>` +
	`<meta property="og:title" content="Body Spray Carolina Herrera 212 Vip Rose Feminino 250ml"/>` +
	`<meta property="og:image" content="https://http2.mlstatic.com/D_NQ_NP_839289-MLA93823127015_092025-O.webp"/>` +
	`</head><body><script>{"components":[` +
	`{"type":"title","title":{"text":"Body Spray Carolina Herrera 212 Vip Rose Feminino 250ml"}},` +
	`{"type":"seller","seller":{"text":"Por Loja Oficial"}},` +
	`{"type":"price","price":{"previous_price":{"value":278.9,"currency":"BRL","decimal_style":"normal"},"current_price":{"value":190.98,"currency":"BRL","decimal_style":"superscript"},"discount":{"value":31},"installments":{"text":"6x {price} sem juros","no_interest":true,"values":[{"value":31.83,"currency":"BRL"}]}}},` +
	`{"type":"title","title":{"text":"Outro Produto Qualquer"}},` +
	`{"type":"price","price":{"current_price":{"value":99.9,"currency":"BRL"}}}` +
	`]}</script></body></html>`

// Sem desconto: 1º card só com "value" (sem previous_price).
const socialFixtureNoDiscount = `<html><head>` +
	`<meta property="og:title" content="Produto Sem Desconto"/>` +
	`<meta property="og:image" content="https://http2.mlstatic.com/D_NQ_x.webp"/>` +
	`</head><body><script>{"components":[` +
	`{"type":"title","title":{"text":"Produto Sem Desconto"}},` +
	`{"type":"price","price":{"value":59.9,"currency":"BRL"}}` +
	`]}</script></body></html>`

func almostEqual(a, b float64) bool { return math.Abs(a-b) < 0.001 }

func TestParseAffiliateSocialHTML_Discount(t *testing.T) {
	p := parseAffiliateSocialHTML(socialFixtureDiscount, "https://meli.la/2Uh8z3c")
	if p == nil {
		t.Fatal("retornou nil")
	}
	if !almostEqual(p.Price, 190.98) {
		t.Errorf("Price = %v, want 190.98 (current_price — NÃO 31 discount, NÃO 31.83 parcela, NÃO 99.9 do 2º card)", p.Price)
	}
	if p.OriginalPrice == nil || !almostEqual(*p.OriginalPrice, 278.9) {
		t.Errorf("OriginalPrice = %v, want 278.9", p.OriginalPrice)
	}
	if p.DiscountPercent == nil || *p.DiscountPercent < 31 || *p.DiscountPercent > 32 {
		t.Errorf("DiscountPercent = %v, want ~31.5", p.DiscountPercent)
	}
	// Imagem deve ser promovida pra 2X (alta resolução).
	if p.Image != "https://http2.mlstatic.com/D_NQ_NP_2X_839289-MLA93823127015_092025-O.webp" {
		t.Errorf("Image = %q, want a variante 2X", p.Image)
	}
	if p.Permalink != "https://meli.la/2Uh8z3c" {
		t.Errorf("Permalink = %q", p.Permalink)
	}
}

func TestParseAffiliateSocialHTML_NoDiscount(t *testing.T) {
	p := parseAffiliateSocialHTML(socialFixtureNoDiscount, "https://meli.la/abc")
	if p == nil {
		t.Fatal("retornou nil")
	}
	if !almostEqual(p.Price, 59.9) {
		t.Errorf("Price = %v, want 59.9", p.Price)
	}
	if p.OriginalPrice != nil {
		t.Errorf("OriginalPrice = %v, want nil (sem desconto)", *p.OriginalPrice)
	}
	if p.DiscountPercent != nil {
		t.Errorf("DiscountPercent = %v, want nil", *p.DiscountPercent)
	}
}

func TestParseAffiliateSocialHTML_NoOgTitle(t *testing.T) {
	if p := parseAffiliateSocialHTML(`<html><body>nada</body></html>`, "https://meli.la/x"); p != nil {
		t.Errorf("esperado nil quando não há og:title, veio %+v", p)
	}
}

func TestIsAffiliateURL(t *testing.T) {
	cases := map[string]bool{
		"https://meli.la/1FP9R6P":                          true,
		"https://mercadolivre.com/sec/2abcXYZ":             true,
		"https://www.mercadolivre.com.br/sec/2abcXYZ":      true,
		"https://produto.mercadolivre.com.br/MLB-123-x_JM": false,
		"https://www.mercadolivre.com.br/p/MLB123":         false,
	}
	for u, want := range cases {
		if got := isAffiliateURL(u); got != want {
			t.Errorf("isAffiliateURL(%q) = %v, want %v", u, got, want)
		}
	}
}

func TestUpgradeMLImageURL(t *testing.T) {
	cases := map[string]string{
		// D_NQ_NP_ vira 2X
		"https://http2.mlstatic.com/D_NQ_NP_839289-MLA9382_092025-O.webp": "https://http2.mlstatic.com/D_NQ_NP_2X_839289-MLA9382_092025-O.webp",
		// já 2X: no-op
		"https://http2.mlstatic.com/D_NQ_NP_2X_839289-MLA9382_092025-O.webp": "https://http2.mlstatic.com/D_NQ_NP_2X_839289-MLA9382_092025-O.webp",
		// outro host/formato: no-op
		"https://cf.shopee.com.br/file/abc": "https://cf.shopee.com.br/file/abc",
		"": "",
	}
	for in, want := range cases {
		if got := upgradeMLImageURL(in); got != want {
			t.Errorf("upgradeMLImageURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAffiliateShortCode(t *testing.T) {
	if c := affiliateShortCode("https://meli.la/2Uh8z3c"); c != "2Uh8z3c" {
		t.Errorf("shortCode = %q, want 2Uh8z3c", c)
	}
	if c := affiliateShortCode("https://www.mercadolivre.com.br/sec/2abcXYZ"); c != "2abcXYZ" {
		t.Errorf("shortCode = %q, want 2abcXYZ", c)
	}
}
