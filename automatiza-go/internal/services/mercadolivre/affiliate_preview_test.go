package mercadolivre

import (
	"math"
	"testing"
)

// HTML sintético espelhando a estrutura real da página /social do afiliado:
// og:title/og:image + lista de cards (title -> seller -> price). O 1º card é o
// produto-alvo (com desconto: previous_price + value). Há um 2º card depois pra
// garantir que pegamos o preço do PRIMEIRO, não de outro.
const socialFixtureDiscount = `<html><head>` +
	`<meta property="og:title" content="Capacete Moto Fechado Pro Tork R8 Fosco Feminino Masculino"/>` +
	`<meta property="og:image" content="https://http2.mlstatic.com/D_NQ_capacete.webp"/>` +
	`</head><body><script>{"components":[` +
	`{"type":"title","title":{"text":"Capacete Moto Fechado Pro Tork R8 Fosco Feminino Masculino"}},` +
	`{"type":"seller","seller":{"text":"Por Sportbay"}},` +
	`{"type":"price","price":{"previous_price":{"value":186.9,"currency":"BRL","decimal_style":"normal"},"value":170.92,"currency":"BRL"}},` +
	`{"type":"title","title":{"text":"Outro Produto Qualquer"}},` +
	`{"type":"price","price":{"value":99.9,"currency":"BRL"}}` +
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
	if p.Title != "Capacete Moto Fechado Pro Tork R8 Fosco Feminino Masculino" {
		t.Errorf("Title = %q", p.Title)
	}
	if !almostEqual(p.Price, 170.92) {
		t.Errorf("Price = %v, want 170.92 (preço atual, não o original nem o do 2º card)", p.Price)
	}
	if p.OriginalPrice == nil || !almostEqual(*p.OriginalPrice, 186.9) {
		t.Errorf("OriginalPrice = %v, want 186.9", p.OriginalPrice)
	}
	if p.DiscountPercent == nil || *p.DiscountPercent < 8 || *p.DiscountPercent > 9 {
		t.Errorf("DiscountPercent = %v, want ~8.55", p.DiscountPercent)
	}
	if p.Image != "https://http2.mlstatic.com/D_NQ_capacete.webp" {
		t.Errorf("Image = %q", p.Image)
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

func TestAffiliateShortCode(t *testing.T) {
	if c := affiliateShortCode("https://meli.la/2Uh8z3c"); c != "2Uh8z3c" {
		t.Errorf("shortCode = %q, want 2Uh8z3c", c)
	}
	if c := affiliateShortCode("https://www.mercadolivre.com.br/sec/2abcXYZ"); c != "2abcXYZ" {
		t.Errorf("shortCode = %q, want 2abcXYZ", c)
	}
}
