// Package magalu converte links do Magazine Luiza para o link de Vitrine do
// usuário (programa Parceiro Magalu / Divulgador).
//
// Não há chamada de rede aqui, e isso é uma decisão dupla. Primeiro: o Magalu
// NÃO tem API de afiliado — o programa é loja-baseado ("Magazine Você"), então
// o link do divulgador é a própria URL do produto servida sob
// magazinevoce.com.br/<loja>/, e a atribuição acontece quando o comprador abre
// essa URL (o site grava o cookie mvc_showcase_name). Segundo: mesmo para
// CONFERIR se a loja existe não há caminho — magazineluiza.com.br devolveu 403
// para HTTP puro tanto do laptop quanto da VPS em 30/07/2026 (WAF
// Radware/Azion), então qualquer validação online seria falso-negativo. Não
// "melhore" isto com um HEAD de verificação.
package magalu

import (
	"context"
	"net/url"
	"regexp"
	"strings"

	"go.uber.org/zap"

	"automatiza-go/internal/lib/commerceurl"
	"automatiza-go/internal/repository"
	"automatiza-go/internal/services/affiliatelink"
)

const (
	// storeKey é a chave em affiliate_credentials.store.
	storeKey = "magalu"
	// showcaseHost é o host da Vitrine, o único que credita o divulgador.
	showcaseHost = "www.magazinevoce.com.br"
)

// slugRe valida o identificador da loja do divulgador.
//
// O ponto fica DE FORA do conjunto de propósito: NormalizeStoreSlug decide se o
// primeiro pedaço do que o cliente colou é um host ou a loja justamente por
// haver um ponto ali. Aceitar ponto no slug tornaria "loja.legal" ambíguo com
// um hostname, e nenhuma Vitrine observada usa ponto.
var slugRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{1,49}$`)

// reservedSlugs são segmentos que existem na Vitrine mas não são loja de
// ninguém. "sualoja" está aqui porque é literalmente o texto do placeholder do
// formulário — alguém vai salvá-lo como se fosse o valor.
var reservedSlugs = map[string]bool{
	"p":       true,
	"busca":   true,
	"lojas":   true,
	"produto": true,
	"sualoja": true,
}

// Converter implementa affiliatelink.Converter para o Magazine Luiza.
type Converter struct {
	affiliateRepo repository.AffiliateCredentialsRepository
	logger        *zap.Logger
}

// New monta o conversor. Sem cliente HTTP e sem cache: a conversão é uma
// concatenação de string, mais barata que consultar qualquer cache.
func New(affiliateRepo repository.AffiliateCredentialsRepository, logger *zap.Logger) *Converter {
	return &Converter{affiliateRepo: affiliateRepo, logger: logger}
}

// Convert devolve https://www.magazinevoce.com.br/<loja>/<caminho-do-produto>/.
//
// Vale tanto para o link cru do catálogo quanto para a Vitrine de OUTRO
// divulgador que chega do grupo-fonte: MagaluProductPath descarta a loja de
// quem postou, e é essa reatribuição que impede a comissão de ir para ele.
//
// A query da origem é descartada inteira (o url.URL montado não tem RawQuery):
// é lá que viajam partner_id, seller_id e utm_* de quem postou.
func (c *Converter) Convert(ctx context.Context, u commerceurl.URL, userID string) (string, error) {
	slug, err := c.storeSlug(ctx, userID)
	if err != nil {
		return "", err
	}

	segs, _, ok := commerceurl.MagaluProductPath(u)
	if !ok {
		// Busca, storefront, home ou path desconhecido: não há produto para
		// carimbar. Fail-closed — o caller remove o link, que é exatamente o
		// que acontecia com todo link Magalu antes deste pacote existir.
		return "", affiliatelink.ErrUnconvertible
	}

	out := url.URL{
		Scheme: "https",
		Host:   showcaseHost,
		Path:   "/" + slug + "/" + strings.Join(segs, "/") + "/",
	}
	return out.String(), nil
}

// storeSlug lê a loja do usuário. Aceita a grafia camelCase do formulário e a
// snake_case legada, como o resto do código de credenciais.
func (c *Converter) storeSlug(ctx context.Context, userID string) (string, error) {
	creds, err := c.affiliateRepo.GetByUserAndStore(ctx, userID, storeKey)
	if err != nil || creds == nil {
		return "", affiliatelink.ErrNoCredentials
	}
	slug := NormalizeStoreSlug(creds.GetCredential("partnerStore"))
	if slug == "" {
		slug = NormalizeStoreSlug(creds.GetCredential("partner_store"))
	}
	if slug == "" {
		// Credencial salva antes desta versão pode conter qualquer coisa; sem
		// slug utilizável o desfecho é o mesmo de não ter credencial.
		return "", affiliatelink.ErrNoCredentials
	}
	return slug, nil
}

// NormalizeStoreSlug extrai a loja do divulgador de qualquer forma que o
// cliente cole no formulário, e devolve "" quando não dá para extrair.
//
// Aceita:
//
//	weekpromos
//	magazinevoce.com.br/weekpromos
//	https://www.magazinevoce.com.br/weekpromos/
//	https://www.magazinevoce.com.br/weekpromos/geladeira/p/abc123/ed/refr/
//
// A última forma funciona porque na Vitrine o primeiro segmento É a loja — o
// cliente pode colar qualquer link da própria vitrine e acertar.
//
// Rejeita link do CATÁLOGO (magazineluiza.com.br/...): ali o primeiro segmento
// é o slug do produto, e aceitá-lo cadastraria uma loja inexistente que geraria
// links sem comissão para ninguém.
func NormalizeStoreSlug(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	if s == "" {
		return ""
	}
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	if i := strings.IndexAny(s, "?#"); i >= 0 {
		s = s[:i]
	}
	s = strings.Trim(s, "/")
	if s == "" {
		return ""
	}

	segs := strings.Split(s, "/")
	first := segs[0]
	if i := strings.Index(first, ":"); i >= 0 { // porta explícita
		first = first[:i]
	}
	if strings.Contains(first, ".") {
		// É um hostname. Só a Vitrine tem a loja no path; catálogo Magalu e
		// domínio alheio não têm o que extrair.
		if !commerceurl.IsMagaluShowcase(strings.TrimSuffix(first, ".")) {
			return ""
		}
		segs = segs[1:]
	}
	if len(segs) == 0 {
		return ""
	}

	slug := segs[0]
	if reservedSlugs[slug] || !slugRe.MatchString(slug) {
		return ""
	}
	return slug
}
