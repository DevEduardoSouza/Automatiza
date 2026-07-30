package mercadolivre

import (
	"math"
	"strings"
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

// padding devolve n bytes de JSON inócuo — sem nenhuma âncora de preço dentro.
func padding(n int) string {
	return strings.Repeat(`{"type":"seller","seller":{"text":"Loja"}},`, n/43)
}

// socialFixturePromoBlock reproduz o layout que o ML passou a servir em
// 30/07/2026: um bloco de promoções cujo JSON começa com `"price":{` aparece
// ANTES do componente do card, e o current_price real fica >800 bytes adiante.
// Com a âncora antiga (1º `"price":{` + janela de 800) o parser lê Price=0 e a
// oferta é descartada — 1210 skips/dia em produção.
var socialFixturePromoBlock = `<html><head>` +
	`<meta property="og:title" content="Guarda Roupa Casal Bartira Genova"/>` +
	`<meta property="og:image" content="https://http2.mlstatic.com/D_NQ_NP_908475-MLA1092717_042026-O.webp"/>` +
	`</head><body><script>{"components":[` +
	`{"type":"highlight","price":{"promotions":[{"campaigns":[{"id":"14025772","type":"SELLER_COUPON"}],"type":"price_decrement","campaign_id":"14025772"}],"price":1395.39}},` +
	padding(1100) +
	`{"type":"price","id":"price","column":1,"price":{"previous_price":{"value":1468,"currency":"BRL","decimal_style":"normal"},"current_price":{"value":1395,"currency":"BRL","decimal_style":"superscript"},"discount_label":{"text":"com Cupom"}}}` +
	`]}</script></body></html>`

func TestParseAffiliateSocialHTML_PromoBlockBeforeCard(t *testing.T) {
	// Guarda o próprio fixture: ele só prova alguma coisa enquanto o
	// current_price estiver FORA da janela de 800 bytes contada a partir do 1º
	// `"price":{`, que é exatamente o que a âncora antiga fazia. Se alguém
	// encurtar o padding, o teste passaria a verde sem testar nada.
	oldAnchor := strings.Index(socialFixturePromoBlock, `"price":{`)
	target := strings.Index(socialFixturePromoBlock, `"current_price"`)
	if oldAnchor < 0 || target <= oldAnchor+800 {
		t.Fatalf("fixture não reproduz mais o bug: âncora antiga em %d, current_price em %d (precisa de >800 de distância)", oldAnchor, target)
	}

	p := parseAffiliateSocialHTML(socialFixturePromoBlock, "https://meli.la/27yUNPK")
	if p == nil {
		t.Fatal("retornou nil")
	}
	if !almostEqual(p.Price, 1395) {
		t.Errorf("Price = %v, want 1395 (current_price do card — o bloco de promoções não pode capturar a âncora)", p.Price)
	}
	if p.OriginalPrice == nil || !almostEqual(*p.OriginalPrice, 1468) {
		t.Errorf("OriginalPrice = %v, want 1468", p.OriginalPrice)
	}
}

// O objeto de promoção começa com `{"type":"price_decrement"`. Sem a aspa final
// na âncora (`{"type":"price"` e não `{"type":"price`) ele capturaria o índice e
// o bug voltaria por outra porta.
func TestParseAffiliateSocialHTML_PriceDecrementNaoEhAncora(t *testing.T) {
	html := `<html><head><meta property="og:title" content="X"/></head><body><script>{"components":[` +
		`{"type":"price_decrement","campaign_id":"14025772","price":{"promotions":[{"id":"1"}],"price":999.99}},` +
		padding(900) +
		`{"type":"price","price":{"current_price":{"value":291.97,"currency":"BRL"}}}` +
		`]}</script></body></html>`
	p := parseAffiliateSocialHTML(html, "https://meli.la/13GZG9q")
	if p == nil {
		t.Fatal("retornou nil")
	}
	if !almostEqual(p.Price, 291.97) {
		t.Errorf("Price = %v, want 291.97 (price_decrement não pode ser âncora)", p.Price)
	}
}

// A janela subiu de 800 para 2000 bytes; sem o corte no próximo `{"type":"price"`
// ela passaria a alcançar o card VIZINHO. Card 1 em formato desconhecido deve
// render Price=0 (item pulado, comportamento seguro), nunca o preço do card 2.
func TestParseAffiliateSocialHTML_NaoInvadeCardSeguinte(t *testing.T) {
	html := `<html><head><meta property="og:title" content="X"/></head><body><script>{"components":[` +
		`{"type":"price","price":{"formato_novo_desconhecido":{"amount":123.45}}},` +
		`{"type":"price","price":{"current_price":{"value":99.9,"currency":"BRL"}}}` +
		`]}</script></body></html>`
	p := parseAffiliateSocialHTML(html, "https://meli.la/x")
	if p == nil {
		t.Fatal("retornou nil")
	}
	if p.Price != 0 {
		t.Errorf("Price = %v, want 0 — o preço do 2º card não pode vazar para o 1º", p.Price)
	}
}

// Ramo legado (documento SEM componentes tipados): a janela tem de continuar
// nos 800 originais. Ali não existe `{"type":"price"` para cortar no card
// seguinte, então uma janela de 2000 atravessaria de um blob de tracking até o
// card de OUTRO produto e emitiria preço plausível do item errado — pior que o
// Price=0, porque o guard de skip não pega.
func TestParseAffiliateSocialHTML_RamoLegadoMantemJanela800(t *testing.T) {
	html := `<html><head><meta property="og:title" content="X"/></head><body><script>{"components":[` +
		`{"tracks":{"price":{"promotions":[{"id":"1"}],"amount":999.99}}},` +
		padding(1000) +
		`{"nao_tipado":{"current_price":{"value":123.45,"currency":"BRL"}}}` +
		`]}</script></body></html>`
	if strings.Contains(html, `{"type":"price"`) {
		t.Fatal("fixture inválida: precisa NÃO ter componente tipado para exercitar o ramo legado")
	}
	p := parseAffiliateSocialHTML(html, "https://meli.la/x")
	if p == nil {
		t.Fatal("retornou nil")
	}
	if p.Price != 0 {
		t.Errorf("Price = %v, want 0 — a janela do ramo legado não pode alcançar o bloco seguinte", p.Price)
	}
}

// Mesmo corte, pelo lado do previous_price: card 1 sem desconto não pode herdar
// o previous_price do card 2 e inventar um "De R$ 500 por R$ 59,90".
func TestParseAffiliateSocialHTML_NaoHerdaPreviousPriceDoCardSeguinte(t *testing.T) {
	html := `<html><head><meta property="og:title" content="X"/></head><body><script>{"components":[` +
		`{"type":"price","price":{"current_price":{"value":59.9,"currency":"BRL"}}},` +
		`{"type":"price","price":{"previous_price":{"value":500,"currency":"BRL"},"current_price":{"value":420,"currency":"BRL"}}}` +
		`]}</script></body></html>`
	p := parseAffiliateSocialHTML(html, "https://meli.la/x")
	if p == nil {
		t.Fatal("retornou nil")
	}
	if !almostEqual(p.Price, 59.9) {
		t.Errorf("Price = %v, want 59.9", p.Price)
	}
	if p.OriginalPrice != nil {
		t.Errorf("OriginalPrice = %v, want nil — desconto fabricado com o preço do card vizinho", *p.OriginalPrice)
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
		"":                                  "",
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
