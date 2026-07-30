package monitoring

import (
	"context"
	"testing"

	"automatiza-go/internal/lib/commerceurl"
	"automatiza-go/internal/models"
	"automatiza-go/internal/services/affiliatelink"
)

type stubConverter struct{ out string }

func (s stubConverter) Convert(_ context.Context, _ commerceurl.URL, _ string) (string, error) {
	return s.out, nil
}

func storeSetOf(stores ...string) map[string]bool {
	out := make(map[string]bool, len(stores))
	for _, s := range stores {
		out[s] = true
	}
	return out
}

const (
	amazonLink = "https://www.amazon.com.br/dp/B09B8V1LZ3"
	shopeeLink = "https://shopee.com.br/product/1/2"
	awinLink   = "https://www.netshoes.com.br/produto/tenis-1"
)

// O opt-in é DUPLO. Cada porta fechada isolada tem de manter o link fora da
// conversão — e um link fora da conversão é removido do corpo, que é o
// comportamento que o espelho sempre teve para essas lojas.
func TestSelectConvertibleLinks(t *testing.T) {
	registry := affiliatelink.New(map[string]affiliatelink.Converter{
		models.MonitorStoreAmazon: stubConverter{out: "https://link.do/TENANT"},
	})
	links := []string{shopeeLink, amazonLink, awinLink}

	cases := []struct {
		name     string
		stores   map[string]bool
		registry *affiliatelink.Registry
		want     []string
	}{
		{
			name:     "loja habilitada e com conversor → converte",
			stores:   storeSetOf("shopee", models.MonitorStoreAmazon),
			registry: registry,
			want:     []string{amazonLink},
		},
		{
			name:     "loja NÃO habilitada na automação → não converte",
			stores:   storeSetOf("shopee"),
			registry: registry,
			want:     nil,
		},
		{
			name:     "habilitada mas sem conversor no registry → não converte",
			stores:   storeSetOf("shopee", models.MonitorStoreAwin),
			registry: registry,
			want:     nil,
		},
		{
			name:     "gate desligado (registry nil) → nada converte",
			stores:   storeSetOf("shopee", models.MonitorStoreAmazon),
			registry: nil,
			want:     nil,
		},
		{
			name:     "sem lojas configuradas → nada converte",
			stores:   nil,
			registry: registry,
			want:     nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := selectConvertibleLinks(links, tc.stores, tc.registry)
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, queria %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("got[%d] = %q, queria %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// Shopee e ML nunca podem entrar por aqui: eles têm loop próprio, com política
// de falha própria (retenção da mensagem). Se vazassem para o caminho do
// registry, seriam convertidos duas vezes ou perderiam o gate de comissão.
func TestSelectConvertibleLinksIgnoraShopeeEML(t *testing.T) {
	registry := affiliatelink.New(map[string]affiliatelink.Converter{
		models.MonitorStoreShopee:      stubConverter{out: "https://x"},
		models.MonitorStoreMercadoLivr: stubConverter{out: "https://y"},
		models.MonitorStoreAmazon:      stubConverter{out: "https://z"},
	})
	links := []string{shopeeLink, "https://www.mercadolivre.com.br/p/MLB123", amazonLink}
	stores := storeSetOf("shopee", "mercadolivre", models.MonitorStoreAmazon)

	got := selectConvertibleLinks(links, stores, registry)

	if len(got) != 1 || got[0] != amazonLink {
		t.Fatalf("got %v, queria só o link Amazon — Shopee/ML têm loop próprio "+
			"e converter aqui também seria conversão dupla", got)
	}
}
