// Package amazon converte links da Amazon para o link de afiliado do usuário.
//
// Não há chamada de rede aqui, e isso é uma decisão, não uma limitação: um link
// de Associates é a URL canônica do produto com ?tag=<associate-id>. A
// Product Advertising API 5.0 foi desligada em 15/05/2026 e a Creators API que
// a substituiu exige OAuth2 e 10 vendas aprovadas nos últimos 30 dias por
// conta — barreira que a maioria dos afiliados não atende. Nenhuma das duas é
// necessária para reatribuir a comissão; elas servem para BUSCAR dados de
// produto (preço, título, imagem), que é outro problema.
package amazon

import (
	"context"
	"net/url"
	"strings"

	"go.uber.org/zap"

	"automatiza-go/internal/lib/commerceurl"
	"automatiza-go/internal/repository"
	"automatiza-go/internal/services/affiliatelink"
)

// storeKey é a chave em affiliate_credentials.store.
const storeKey = "amazon"

// Converter implementa affiliatelink.Converter para a Amazon.
type Converter struct {
	affiliateRepo repository.AffiliateCredentialsRepository
	logger        *zap.Logger
}

// New monta o conversor. Sem cache: a conversão é uma concatenação de string,
// mais barata que consultar qualquer cache.
func New(affiliateRepo repository.AffiliateCredentialsRepository, logger *zap.Logger) *Converter {
	return &Converter{affiliateRepo: affiliateRepo, logger: logger}
}

// Convert devolve https://<host-de-origem>/dp/<ASIN>?tag=<tag>.
//
// O host da origem é preservado porque a tag de Associates é POR marketplace:
// uma tag "-20" do amazon.com.br não credita venda no amazon.com. Reescrever
// tudo para .com.br mandaria o comprador para outra loja.
func (c *Converter) Convert(ctx context.Context, u commerceurl.URL, userID string) (string, error) {
	tag, err := c.affiliateTag(ctx, userID)
	if err != nil {
		return "", err
	}

	asin, ok := commerceurl.AmazonASIN(u.Parsed)
	if !ok {
		// Encurtador (amzn.to/a.co), busca, storefront ou lista: sem ASIN não
		// existe URL canônica de produto para carimbar. Emitir a origem com a
		// tag colada na query também não serve — encurtador ignora query.
		return "", affiliatelink.ErrUnconvertible
	}

	host := u.Host
	if host == "" {
		return "", affiliatelink.ErrUnconvertible
	}

	out := url.URL{
		Scheme:   "https",
		Host:     host,
		Path:     "/dp/" + asin,
		RawQuery: url.Values{"tag": {tag}}.Encode(),
	}
	return out.String(), nil
}

// affiliateTag lê a tag do usuário. Aceita a grafia camelCase do formulário e a
// snake_case legada, como o resto do código de credenciais.
func (c *Converter) affiliateTag(ctx context.Context, userID string) (string, error) {
	creds, err := c.affiliateRepo.GetByUserAndStore(ctx, userID, storeKey)
	if err != nil || creds == nil {
		return "", affiliatelink.ErrNoCredentials
	}
	tag := strings.TrimSpace(creds.GetCredential("tag"))
	if tag == "" {
		tag = strings.TrimSpace(creds.GetCredential("partner_tag"))
	}
	if tag == "" {
		return "", affiliatelink.ErrNoCredentials
	}
	return tag, nil
}
