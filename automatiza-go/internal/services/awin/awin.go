package awin

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"

	"go.uber.org/zap"

	"automatiza-go/internal/lib/commerceurl"
	"automatiza-go/internal/repository"
	"automatiza-go/internal/services/affiliatelink"
)

const (
	// storeKey é a chave em affiliate_credentials.store.
	storeKey = "awin"
	// ProgrammesCredentialKey guarda o mapa domínio→awinmid no JSONB da
	// credencial. É chave INTERNA: o handler a remove antes de responder ao
	// navegador (não é segredo, é volume — alguns KB que a UI não usa).
	ProgrammesCredentialKey = "programmes"
	// ProgrammeNamesCredentialKey guarda os nomes das marcas aprovadas, que a
	// UI usa para montar o seletor.
	ProgrammeNamesCredentialKey = "programme_names"
	// SelectedBrandsCredentialKey guarda as marcas que o usuário escolheu no
	// seletor (array JSON de nomes). Vazio/ausente = todas as aprovadas.
	SelectedBrandsCredentialKey = "selected_brands"
	// BrandMapCredentialKey guarda nome→awinmid: é o que liga a seleção do
	// formulário (nomes) ao anunciante real (id). Chave INTERNA.
	BrandMapCredentialKey = "programme_brand_ids"
	// redirectBase é o endpoint de redirect da rede.
	redirectBase = "https://www.awin1.com/cread.php"
	// clickRef identifica a plataforma no relatório da Awin, espelhando o
	// shopeeSubIDDefault. Sem ele a venda entra sem origem.
	clickRef = "AUTOMATISA"
)

// Converter implementa affiliatelink.Converter para a rede Awin.
type Converter struct {
	affiliateRepo repository.AffiliateCredentialsRepository
	logger        *zap.Logger
}

// New monta o conversor. Sem cliente HTTP: a conversão é determinística.
func New(affiliateRepo repository.AffiliateCredentialsRepository, logger *zap.Logger) *Converter {
	return &Converter{affiliateRepo: affiliateRepo, logger: logger}
}

// Convert devolve o deeplink Awin do usuário para a URL de destino.
//
// Duas entradas possíveis:
//   - URL de um anunciante (netshoes.com.br/...): vira ued do deeplink;
//   - deeplink Awin de OUTRO afiliado (awin1.com/cread.php?awinaffid=...): o
//     destino real é extraído de ued e reatribuído ao publisher do tenant.
//     Sem isso, repassar o link recebido pagaria comissão a quem postou.
func (c *Converter) Convert(ctx context.Context, u commerceurl.URL, userID string) (string, error) {
	publisherID, domains, allowed, err := c.credentials(ctx, userID)
	if err != nil {
		return "", err
	}

	target := u
	if commerceurl.IsAwinRedirect(u) {
		inner, ok := innerTarget(u)
		if !ok {
			return "", affiliatelink.ErrUnconvertible
		}
		target = inner
	}

	domain, ok := commerceurl.AwinAdvertiserHost(target.Host)
	if !ok {
		return "", affiliatelink.ErrUnconvertible
	}
	advertiserID := domains[domain]
	if advertiserID == "" {
		// O usuário não está aprovado neste anunciante (ou nunca sincronizou a
		// lista). Fail-closed: emitir deeplink com awinmid errado gera clique
		// sem comissão e suja o relatório.
		return "", affiliatelink.ErrUnconvertible
	}
	// Seleção de marcas do formulário. Sem ela, o seletor da UI seria decorativo
	// (o usuário desmarca uma marca e ela continuaria sendo convertida).
	// Seleção vazia = o usuário nunca escolheu, então valem todas as aprovadas.
	if len(allowed) > 0 && !allowed[advertiserID] {
		return "", affiliatelink.ErrUnconvertible
	}

	// StripTracking remove utm_*/af_* da origem: sem isso o tracking de quem
	// postou viajaria dentro do nosso deeplink.
	destination := commerceurl.StripTracking(target.Parsed)

	q := url.Values{
		"awinmid":   {advertiserID},
		"awinaffid": {publisherID},
		"clickref":  {clickRef},
		"ued":       {destination},
	}
	return redirectBase + "?" + q.Encode(), nil
}

// innerTarget extrai e revalida o destino de um deeplink Awin recebido.
func innerTarget(u commerceurl.URL) (commerceurl.URL, bool) {
	raw := strings.TrimSpace(u.Parsed.Query().Get("ued"))
	if raw == "" {
		return commerceurl.URL{}, false
	}
	// Parse aplica de novo a validação anti-spoof/SSRF: o ued veio de terceiro.
	inner, err := commerceurl.Parse(raw)
	if err != nil {
		return commerceurl.URL{}, false
	}
	return inner, true
}

// credentials lê publisher ID, o mapa domínio→awinmid e o conjunto de
// advertiser IDs que o usuário selecionou no formulário.
func (c *Converter) credentials(
	ctx context.Context,
	userID string,
) (publisherID string, domains map[string]string, allowed map[string]bool, err error) {
	creds, err := c.affiliateRepo.GetByUserAndStore(ctx, userID, storeKey)
	if err != nil || creds == nil {
		return "", nil, nil, affiliatelink.ErrNoCredentials
	}

	publisherID = strings.TrimSpace(creds.GetCredential("awinId"))
	if publisherID == "" {
		publisherID = strings.TrimSpace(creds.GetCredential("publisher_id"))
	}
	if publisherID == "" {
		return "", nil, nil, affiliatelink.ErrNoCredentials
	}

	domains = ParseDomainMap(creds.GetCredential(ProgrammesCredentialKey))
	if len(domains) == 0 {
		// Credencial salva mas lista de programas nunca sincronizada: não dá
		// para saber o awinmid de nada.
		return "", nil, nil, affiliatelink.ErrNoCredentials
	}

	allowed = selectedAdvertisers(
		creds.GetCredential(SelectedBrandsCredentialKey),
		creds.GetCredential(BrandMapCredentialKey),
	)
	return publisherID, domains, allowed, nil
}

// selectedAdvertisers traduz a seleção do formulário (nomes de marca) para o
// conjunto de advertiser IDs aceitos.
//
// O casamento nome→ID vem de BrandMapCredentialKey, gravado pelo handler a
// partir da mesma resposta da Awin que produziu o mapa de domínios — nada de
// adivinhar por ordem ou por rótulo parecido. Sem esse mapa (credencial salva
// antes desta versão), devolve nil = sem filtro, que é o comportamento anterior.
func selectedAdvertisers(selectedRaw, brandMapRaw string) map[string]bool {
	if strings.TrimSpace(selectedRaw) == "" || strings.TrimSpace(brandMapRaw) == "" {
		return nil
	}
	var selected []string
	if err := json.Unmarshal([]byte(selectedRaw), &selected); err != nil || len(selected) == 0 {
		return nil
	}
	var brandToID map[string]string
	if err := json.Unmarshal([]byte(brandMapRaw), &brandToID); err != nil || len(brandToID) == 0 {
		return nil
	}

	out := make(map[string]bool, len(selected))
	for _, name := range selected {
		if id := brandToID[strings.TrimSpace(name)]; id != "" {
			out[id] = true
		}
	}
	if len(out) == 0 {
		// Nenhum nome selecionado casou com um programa conhecido: tratar como
		// "sem filtro" em vez de bloquear tudo — o usuário não pediu para parar
		// de converter, e a lista de nomes pode ter mudado do lado da Awin.
		return nil
	}
	return out
}

// BrandMap reduz os programas ao mapa nome→awinmid usado pelo filtro de marcas.
func BrandMap(programmes []Programme) map[string]string {
	out := make(map[string]string, len(programmes))
	for _, p := range programmes {
		if p.Name == "" || p.ID == "" {
			continue
		}
		out[strings.TrimSpace(p.Name)] = p.ID
	}
	return out
}

// EncodeBrandMap serializa o mapa nome→awinmid para gravação no JSONB.
func EncodeBrandMap(brands map[string]string) (string, error) {
	b, err := json.Marshal(brands)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// ParseDomainMap decodifica o mapa domínio→awinmid guardado no JSONB.
func ParseDomainMap(raw string) map[string]string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var out map[string]string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}

// EncodeDomainMap serializa o mapa para gravação no JSONB.
func EncodeDomainMap(domains map[string]string) (string, error) {
	b, err := json.Marshal(domains)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
