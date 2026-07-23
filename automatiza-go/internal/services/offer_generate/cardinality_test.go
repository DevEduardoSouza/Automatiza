package offer_generate

import (
	"testing"

	"automatiza-go/internal/models"
)

// TestFirstProductStoreDesempataPeloPrimeiroPRODUTO é a correção do §12.7: o
// ramo do serviço é `if shopee {...} else if ml {...}`, então um cupom Shopee
// citado ANTES roubava a oferta de um produto ML. O desempate passa a ser o
// primeiro link que é PRODUTO na ordem do texto.
func TestFirstProductStoreDesempataPeloPrimeiroProduto(t *testing.T) {
	casos := []struct {
		nome  string
		links []string
		want  string
	}{
		{
			nome: "cupom Shopee antes de produto ML → oferta é do ML",
			links: []string{
				"https://shopee.com.br/m/cupom-do-dia",
				"https://www.mercadolivre.com.br/p/MLB123",
			},
			want: models.MonitorStoreMercadoLivr,
		},
		{
			nome: "produto Shopee antes de produto ML → oferta é do Shopee",
			links: []string{
				"https://shopee.com.br/product/1/2",
				"https://www.mercadolivre.com.br/p/MLB123",
			},
			want: models.MonitorStoreShopee,
		},
		{
			nome: "produto ML antes de produto Shopee → oferta é do ML",
			links: []string{
				"https://www.mercadolivre.com.br/p/MLB123",
				"https://shopee.com.br/product/1/2",
			},
			want: models.MonitorStoreMercadoLivr,
		},
		{
			nome:  "nenhum produto local (só short links) → sem desempate",
			links: []string{"https://s.shopee.com.br/abc", "https://meli.la/xyz"},
			want:  "",
		},
		{
			nome:  "só campanhas → sem desempate, ordem histórica preservada",
			links: []string{"https://shopee.com.br/m/x", "https://www.mercadolivre.com.br/cupons"},
			want:  "",
		},
		{
			nome:  "host forjado é ignorado, não vira desempate",
			links: []string{"https://evil.test/shopee.com.br/product/1/2", "https://www.mercadolivre.com.br/p/MLB1"},
			want:  models.MonitorStoreMercadoLivr,
		},
		{
			nome:  "link não-comercial não desempata",
			links: []string{"https://instagram.com/canal", "https://shopee.com.br/product/1/2"},
			want:  models.MonitorStoreShopee,
		},
		{
			nome:  "sem links",
			links: nil,
			want:  "",
		},
	}

	for _, tc := range casos {
		t.Run(tc.nome, func(t *testing.T) {
			if got := firstProductStore(tc.links); got != tc.want {
				t.Errorf("firstProductStore(%v) = %q, queria %q", tc.links, got, tc.want)
			}
		})
	}
}

// TestFirstProductStoreNaoFazIO garante que o desempate é puramente local:
// ele roda no caminho quente, antes de qualquer chamada de rede, e uma
// expansão aqui custaria latência por mensagem.
func TestFirstProductStoreNaoFazIO(t *testing.T) {
	// URLs que exigiriam rede para classificar continuam devolvendo "" —
	// prova indireta de que nenhuma expansão acontece.
	for _, l := range []string{
		"https://s.shopee.com.br/qualquercoisa",
		"https://shp.ee/abc",
		"https://meli.la/abc",
		"https://www.mercadolivre.com.br/sec/abc",
	} {
		if got := firstProductStore([]string{l}); got != "" {
			t.Errorf("%s: devolveu %q — classificar isso exigiria expansão", l, got)
		}
	}
}
