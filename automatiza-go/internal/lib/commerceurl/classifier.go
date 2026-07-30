package commerceurl

import (
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"automatiza-go/internal/models"
)

// Classification é o veredito local (sem I/O) de uma URL já parseada.
//
// Local é a palavra-chave: aqui só entra o que dá para decidir olhando host e
// path. Tudo que dependeria de rede sai como URLClassUnknown, e é o
// classificador de rede (expansão/preview/API) que resolve — ou deixa unknown,
// que NUNCA vira campanha.
type Classification struct {
	Class        string // models.URLClass*
	ErrorClass   string // models.ErrorClass*
	Reason       string // models.Reason*
	CanonicalKey string // "shopee:<shop>:<item>" | "ml:<MLB>" | ""
	CanonicalURL string // URL limpa correspondente, quando existe
	ShopID       int64
	ItemID       int64
	MLBID        string
}

// ClassifyLocal decide o que dá para decidir sem rede.
func ClassifyLocal(u URL) Classification {
	switch u.Store {
	case models.MonitorStoreShopee:
		return classifyShopee(u)
	case models.MonitorStoreMercadoLivr:
		return classifyML(u)
	case models.MonitorStoreAmazon:
		return classifyAmazon(u)
	case models.MonitorStoreAliExpress:
		return classifyAliExpress(u)
	case models.MonitorStoreAwin:
		return classifyAwin(u)
	case models.MonitorStoreMagalu:
		return classifyMagalu(u)
	case models.MonitorStoreShein, models.MonitorStoreUnsupported:
		// Shein divide o ramo do unsupported de propósito: o link do afiliado é
		// um onelink opaco, então não há ID de produto a extrair nem
		// classificação por path a fazer. Quem decide o destino dela é o
		// espelho (substituição pelo link do dono), não o classificador — mas
		// ela PRECISA estar aqui: cair no default a marcaria NonCommerce, e um
		// link de loja classificado como não-comércio passa cru.
		return Classification{
			Class:      models.URLClassUnsupportedCommerce,
			ErrorClass: models.ErrorClassNone,
			Reason:     models.ReasonUnsupportedStore,
		}
	default:
		return Classification{
			Class:      models.URLClassNonCommerce,
			ErrorClass: models.ErrorClassNone,
			Reason:     models.ReasonNone,
		}
	}
}

// ---------------------------------------------------------------------------
// Shopee
// ---------------------------------------------------------------------------

// shopeeShortHosts são os encurtadores: sem IDs no path, exigem expansão.
var shopeeShortHosts = []string{"shope.ee", "shp.ee"}

// IsShopeeShortener reporta se o host é um encurtador Shopee.
func IsShopeeShortener(host string) bool {
	for _, d := range shopeeShortHosts {
		if hostMatches(host, d) {
			return true
		}
	}
	// s.shopee.com.br também é encurtador, apesar de cair no domínio principal.
	return host == "s.shopee.com.br"
}

// shopeeCampaignSegments são os primeiros segmentos que provam campanha quando
// não há IDs de produto (§5.3 item 4). Comparação por SEGMENTO, nunca
// substring: "?redirect=/mall/" não é campanha.
var shopeeCampaignSegments = map[string]bool{
	"m":              true,
	"mall":           true,
	"flash_sale":     true,
	"daily_discover": true,
	"voucher":        true,
	"user":           true,
	"oficial":        true,
	"collections":    true,
	"shopeevip":      true,
}

func classifyShopee(u URL) Classification {
	segs := PathSegments(u.Parsed)

	// /cart e /cart/*: o item foi perdido na expansão. Não relayar a URL.
	if len(segs) > 0 && strings.EqualFold(segs[0], "cart") {
		return Classification{
			Class:      models.URLClassUnknown,
			ErrorClass: models.ErrorClassPermanentConte,
			Reason:     models.ReasonCart,
		}
	}

	if shopID, itemID, ok := ExtractShopeeIDs(u.Parsed); ok {
		return Classification{
			Class:        models.URLClassProduct,
			ErrorClass:   models.ErrorClassNone,
			Reason:       models.ReasonNone,
			CanonicalKey: ShopeeKey(shopID, itemID),
			CanonicalURL: ShopeeCanonicalURL(shopID, itemID),
			ShopID:       shopID,
			ItemID:       itemID,
		}
	}

	if IsShopeeShortener(u.Host) {
		// Encurtador sem IDs: só a expansão decide.
		return Classification{
			Class:      models.URLClassUnknown,
			ErrorClass: models.ErrorClassNone,
			Reason:     models.ReasonNone,
		}
	}

	if len(segs) > 0 && shopeeCampaignSegments[strings.ToLower(segs[0])] {
		return Classification{
			Class:        models.URLClassCampaignOrCoupon,
			ErrorClass:   models.ErrorClassNone,
			Reason:       models.ReasonNone,
			CanonicalURL: cleanCanonical(u.Parsed),
		}
	}

	// Outros paths sem IDs ficam unknown até generateShortLink (§5.3 item 6).
	return Classification{
		Class:      models.URLClassUnknown,
		ErrorClass: models.ErrorClassNone,
		Reason:     models.ReasonNone,
	}
}

// ShopeeKey monta a canonical key de produto Shopee.
func ShopeeKey(shopID, itemID int64) string {
	return "shopee:" + strconv.FormatInt(shopID, 10) + ":" + strconv.FormatInt(itemID, 10)
}

// ShopeeCanonicalURL reconstrói a URL de produto a partir dos IDs.
func ShopeeCanonicalURL(shopID, itemID int64) string {
	return "https://shopee.com.br/product/" +
		strconv.FormatInt(shopID, 10) + "/" + strconv.FormatInt(itemID, 10)
}

var shopeeSlugIDsRe = regexp.MustCompile(`\.(\d+)\.(\d+)$`)

// ExtractShopeeIDs lê shopID/itemID SOMENTE do path já parseado.
//
// Três formas aceitas:
//   - /product/<shop>/<item>
//   - /<slug>/<shop>/<item>  (últimos dois segmentos numéricos)
//   - /<slug>.<shop>.<item>  (sufixo do slug)
//
// Número em query, host ou fragmento não conta. Ambos os IDs precisam ser > 0:
// "shopee.com.br/x/0/0" não é produto.
func ExtractShopeeIDs(u *url.URL) (shopID, itemID int64, ok bool) {
	segs := PathSegments(u)
	if len(segs) == 0 {
		return 0, 0, false
	}

	// /product/<shop>/<item>
	if len(segs) >= 3 && strings.EqualFold(segs[0], "product") {
		if s, err1 := strconv.ParseInt(segs[1], 10, 64); err1 == nil {
			if i, err2 := strconv.ParseInt(segs[2], 10, 64); err2 == nil && s > 0 && i > 0 {
				return s, i, true
			}
		}
	}

	// últimos dois segmentos numéricos
	if len(segs) >= 3 {
		a, errA := strconv.ParseInt(segs[len(segs)-2], 10, 64)
		b, errB := strconv.ParseInt(segs[len(segs)-1], 10, 64)
		if errA == nil && errB == nil && a > 0 && b > 0 {
			return a, b, true
		}
	}

	// sufixo .<shop>.<item> no último segmento
	if m := shopeeSlugIDsRe.FindStringSubmatch(segs[len(segs)-1]); m != nil {
		a, errA := strconv.ParseInt(m[1], 10, 64)
		b, errB := strconv.ParseInt(m[2], 10, 64)
		if errA == nil && errB == nil && a > 0 && b > 0 {
			return a, b, true
		}
	}

	return 0, 0, false
}

// ---------------------------------------------------------------------------
// Mercado Livre
// ---------------------------------------------------------------------------

// mlShortHosts exigem resolução anônima antes de qualquer coisa.
var mlShortHosts = []string{"meli.la"}

// IsMLShortener reporta se o host/path é um short link ML.
func IsMLShortener(u URL) bool {
	for _, d := range mlShortHosts {
		if hostMatches(u.Host, d) {
			return true
		}
	}
	return HasPathPrefix(u.Parsed, "sec")
}

// mlAuthHosts é a allowlist EXATA que pode receber cookie/CSRF do ML (§5.2).
var mlAuthHosts = map[string]bool{
	"www.mercadolivre.com.br":     true,
	"produto.mercadolivre.com.br": true,
}

// AllowsMLCredentials reporta se u pode receber cookie/CSRF autenticado do ML.
// Exige HTTPS e host exato — subdomínio novo NÃO é aceito automaticamente.
func AllowsMLCredentials(u URL) bool {
	return u.Parsed != nil && u.Parsed.Scheme == "https" && mlAuthHosts[u.Host]
}

// mlbSegmentRe ancora no INÍCIO do segmento e deixa o slug seguir:
// "MLB-123456-nome-do-produto" é a forma real de produto.mercadolivre.com.br.
// A letra opcional ([A-Z]) cobre variantes como MLBU; o ID normalizado é
// sempre "MLB"+dígitos, igual ao que ExtractProductID já devolvia.
var mlbSegmentRe = regexp.MustCompile(`^MLB[A-Z]?-?(\d+)(?:-|$)`)

// ExtractMLBID lê o ID de anúncio SOMENTE dos segmentos do path.
//
// Aceita: segmento iniciado por MLB-?<dígitos>, ou o segmento imediatamente
// após /p/ ou /up/. MLB em query, userinfo, host ou texto posterior NÃO
// classifica produto — era exatamente por aí que "?redirect=MLB123" virava
// produto. "ofertas-mlb-do-dia" também não casa: o segmento não começa com MLB.
func ExtractMLBID(u *url.URL) (string, bool) {
	segs := PathSegments(u)
	for i, seg := range segs {
		upper := strings.ToUpper(seg)
		if m := mlbSegmentRe.FindStringSubmatch(upper); m != nil {
			return "MLB" + m[1], true
		}
		if (strings.EqualFold(seg, "p") || strings.EqualFold(seg, "up")) && i+1 < len(segs) {
			next := strings.ToUpper(segs[i+1])
			if m := mlbSegmentRe.FindStringSubmatch(next); m != nil {
				return "MLB" + m[1], true
			}
		}
	}
	return "", false
}

// MLKey monta a canonical key de produto ML.
func MLKey(mlbID string) string { return "ml:" + strings.ToUpper(mlbID) }

func classifyML(u URL) Classification {
	if mlb, ok := ExtractMLBID(u.Parsed); ok {
		return Classification{
			Class:        models.URLClassProduct,
			ErrorClass:   models.ErrorClassNone,
			Reason:       models.ReasonNone,
			CanonicalKey: MLKey(mlb),
			// Canonical de produto ignora TODA query: o ID é a identidade, e
			// query preservada é justamente onde o tracking da fonte mora.
			CanonicalURL: cleanCanonicalNoQuery(u.Parsed),
			MLBID:        mlb,
		}
	}

	// meli.la e /sec/ são unknown até resolver anonimamente (§5.4 item 2).
	if IsMLShortener(u) {
		return Classification{
			Class:      models.URLClassUnknown,
			ErrorClass: models.ErrorClassNone,
			Reason:     models.ReasonNone,
		}
	}

	// Campanha positiva: /cupons, /cupons/* e /l/* sem MLB (§5.4 item 4).
	if HasPathPrefix(u.Parsed, "cupons") || HasPathPrefix(u.Parsed, "l") {
		return Classification{
			Class:        models.URLClassCampaignOrCoupon,
			ErrorClass:   models.ErrorClassNone,
			Reason:       models.ReasonNone,
			CanonicalURL: cleanCanonicalNoQuery(u.Parsed),
		}
	}

	// /social/ e o resto continuam unknown: preview basta para renderizar, não
	// para reatribuir comissão.
	return Classification{
		Class:      models.URLClassUnknown,
		ErrorClass: models.ErrorClassNone,
		Reason:     models.ReasonNone,
	}
}

// ---------------------------------------------------------------------------
// Amazon
// ---------------------------------------------------------------------------

// asinRe casa o ASIN: 10 caracteres alfanuméricos MAIÚSCULOS. Amazon usa
// ISBN-10 legado em livros, onde o último dígito pode ser 'X' — já coberto.
var asinRe = regexp.MustCompile(`^[A-Z0-9]{10}$`)

// amazonASINSegments são os segmentos que antecedem o ASIN no path.
var amazonASINSegments = map[string]bool{
	"dp":      true,
	"product": true, // /gp/product/<ASIN>
	"d":       true, // /gp/aw/d/<ASIN>
	"asin":    true,
}

// AmazonASIN lê o ASIN do path já parseado. Só o path conta: ASIN em query
// não identifica produto.
//
// Formas aceitas: /dp/<ASIN>, /<slug>/dp/<ASIN>, /gp/product/<ASIN>,
// /gp/aw/d/<ASIN>. Encurtador (amzn.to/xxxx), busca (/s?k=) e storefront não
// têm ASIN — devolvem false, e o conversor deixa o link ser removido em vez
// de emitir uma URL que não converte.
func AmazonASIN(u *url.URL) (string, bool) {
	segs := PathSegments(u)
	for i, seg := range segs {
		if !amazonASINSegments[strings.ToLower(seg)] {
			continue
		}
		if i+1 >= len(segs) {
			continue
		}
		candidate := strings.ToUpper(segs[i+1])
		if asinRe.MatchString(candidate) {
			return candidate, true
		}
	}
	return "", false
}

// AmazonKey monta a canonical key de produto Amazon.
func AmazonKey(asin string) string { return "amazon:" + strings.ToUpper(asin) }

func classifyAmazon(u URL) Classification {
	if asin, ok := AmazonASIN(u.Parsed); ok {
		return Classification{
			Class:        models.URLClassProduct,
			ErrorClass:   models.ErrorClassNone,
			Reason:       models.ReasonNone,
			CanonicalKey: AmazonKey(asin),
			CanonicalURL: cleanCanonicalNoQuery(u.Parsed),
		}
	}
	// Sem ASIN não dá para reatribuir: encurtador precisaria de expansão,
	// busca/storefront não são produto. Unknown NUNCA vira campanha.
	return Classification{
		Class:      models.URLClassUnknown,
		ErrorClass: models.ErrorClassNone,
		Reason:     models.ReasonNone,
	}
}

// ---------------------------------------------------------------------------
// AliExpress
// ---------------------------------------------------------------------------

// aliexpressItemRe casa o ID no último segmento: /item/1005006123456789.html
var aliexpressItemRe = regexp.MustCompile(`^(\d{6,})\.html$`)

// AliExpressItemID lê o ID de produto do path já parseado.
func AliExpressItemID(u *url.URL) (string, bool) {
	segs := PathSegments(u)
	if len(segs) == 0 {
		return "", false
	}
	if m := aliexpressItemRe.FindStringSubmatch(segs[len(segs)-1]); m != nil {
		return m[1], true
	}
	return "", false
}

// AliExpressKey monta a canonical key de produto AliExpress.
func AliExpressKey(itemID string) string { return "aliexpress:" + itemID }

func classifyAliExpress(u URL) Classification {
	if id, ok := AliExpressItemID(u.Parsed); ok {
		return Classification{
			Class:        models.URLClassProduct,
			ErrorClass:   models.ErrorClassNone,
			Reason:       models.ReasonNone,
			CanonicalKey: AliExpressKey(id),
			CanonicalURL: cleanCanonicalNoQuery(u.Parsed),
		}
	}
	// s.click.aliexpress.com e /gcp/ (campanhas) não trazem ID no path. A API
	// de afiliado converte a URL inteira, então não é "impossível" — mas sem
	// ID não há identidade para dedup: unknown.
	return Classification{
		Class:      models.URLClassUnknown,
		ErrorClass: models.ErrorClassNone,
		Reason:     models.ReasonNone,
	}
}

// ---------------------------------------------------------------------------
// Awin
// ---------------------------------------------------------------------------

// AwinAdvertiserHost devolve o domínio de anunciante Awin que casa com o host.
// Devolve o domínio CANÔNICO da tabela (não o host cheio), que é a chave do
// mapa domínio→awinmid guardado na credencial do usuário.
func AwinAdvertiserHost(host string) (string, bool) {
	for _, d := range awinAdvertiserHosts {
		if hostMatches(host, d) {
			return d, true
		}
	}
	return "", false
}

// IsAwinRedirect reporta se a URL já é um deeplink da rede Awin — nesse caso
// ela carrega o awinaffid de OUTRO afiliado e precisa ser reatribuída.
func IsAwinRedirect(u URL) bool {
	for _, d := range awinHosts {
		if hostMatches(u.Host, d) {
			return true
		}
	}
	return false
}

func classifyAwin(u URL) Classification {
	// Deeplink da rede: o destino real está em ?ued=, ainda não resolvido aqui.
	if IsAwinRedirect(u) {
		return Classification{
			Class:      models.URLClassUnknown,
			ErrorClass: models.ErrorClassNone,
			Reason:     models.ReasonNone,
		}
	}
	// Loja de anunciante: qualquer URL do domínio é convertível, porque o
	// deeplink embrulha a URL inteira (não precisa de ID de produto).
	return Classification{
		Class:        models.URLClassProduct,
		ErrorClass:   models.ErrorClassNone,
		Reason:       models.ReasonNone,
		CanonicalURL: cleanCanonical(u.Parsed),
	}
}

// ---------------------------------------------------------------------------
// Magalu
// ---------------------------------------------------------------------------

// magaluSKURe casa o código do produto que segue /p/ — alfanumérico minúsculo,
// como "hbf8fjj15a" ou "235111500". Não é o slug: o slug é o texto legível que
// vem ANTES de /p/.
var magaluSKURe = regexp.MustCompile(`^[a-z0-9]{4,32}$`)

// IsMagaluShowcase reporta se o host é a Vitrine (magazinevoce.com.br), onde o
// primeiro segmento do path é a loja do divulgador.
func IsMagaluShowcase(host string) bool {
	for _, d := range magaluShowcaseHosts {
		if hostMatches(host, d) {
			return true
		}
	}
	return false
}

// MagaluProductPath devolve os segmentos do PRODUTO (slug + "p" + sku + resto)
// e o SKU. É a única fonte da forma da URL Magalu: classificador e conversor
// leem daqui.
//
// Host-aware de propósito. Na Vitrine o primeiro segmento é a LOJA de quem
// postou — descartá-lo é o que permite reatribuir ao tenant o link de outro
// divulgador; mantê-lo transformaria a loja de terceiro no "slug do produto" e
// emitiria uma URL quebrada:
//
//	magazineluiza.com.br/<slug>/p/<sku>/<cat>/<sub>/      → [slug p sku cat sub]
//	magazinevoce.com.br/<OUTRA>/<slug>/p/<sku>/<cat>/     → [slug p sku cat]
//	magazinevoce.com.br/<OUTRA>/p/<sku>/                  → [p sku]
//
// Busca, storefront, home e qualquer path sem /p/<sku> válido devolvem false —
// o caller trata como não-convertível e o link é removido em vez de repassado.
func MagaluProductPath(u URL) (segs []string, sku string, ok bool) {
	if u.Parsed == nil {
		return nil, "", false
	}
	path := PathSegments(u.Parsed)
	if IsMagaluShowcase(u.Host) {
		if len(path) == 0 {
			return nil, "", false
		}
		path = path[1:]
	}
	for i, seg := range path {
		if !strings.EqualFold(seg, "p") || i+1 >= len(path) {
			continue
		}
		candidate := strings.ToLower(path[i+1])
		if !magaluSKURe.MatchString(candidate) {
			continue
		}
		start := i
		if i > 0 {
			// Preserva o slug legível do produto, que a Magalu espera na URL
			// completa. A forma mínima (sem slug) também funciona — o servidor
			// canonicaliza com 302 — mas emitir o que já temos evita o salto.
			start = i - 1
		}
		return path[start:], candidate, true
	}
	return nil, "", false
}

// MagaluKey monta a canonical key de produto Magalu.
func MagaluKey(sku string) string { return "magalu:" + strings.ToLower(sku) }

func classifyMagalu(u URL) Classification {
	if _, sku, ok := MagaluProductPath(u); ok {
		return Classification{
			Class:        models.URLClassProduct,
			ErrorClass:   models.ErrorClassNone,
			Reason:       models.ReasonNone,
			CanonicalKey: MagaluKey(sku),
			CanonicalURL: cleanCanonicalNoQuery(u.Parsed),
		}
	}
	// Sem SKU não há produto a reatribuir (busca, storefront, home). Unknown
	// NUNCA vira campanha.
	return Classification{
		Class:      models.URLClassUnknown,
		ErrorClass: models.ErrorClassNone,
		Reason:     models.ReasonNone,
	}
}

// ---------------------------------------------------------------------------
// Tracking
// ---------------------------------------------------------------------------

// trackingParams são nomes exatos removidos de qualquer URL emitida.
var trackingParams = map[string]bool{
	"mmp_pid":          true,
	"uls_trackid":      true,
	"smtt":             true,
	"deal_print_id":    true,
	"tracking_id":      true,
	"matt_tool":        true,
	"matt_word":        true,
	"matt_source":      true,
	"matt_campaign_id": true,
}

// trackingPrefixes são prefixos removidos. `quantity`, `variation` e
// desconhecidos NÃO entram: rotular parâmetro funcional como tracking quebra o
// link do cliente, e o plano exige prova antes de remover.
// aff_ cobre o tracking de afiliado do AliExpress (aff_fcid, aff_platform,
// aff_trace_key) — ele NÃO casa com "af_", que exige o underscore no terceiro
// caractere. Sem ele, o identificador de quem postou no grupo-fonte viajava
// dentro do link que emitimos.
var trackingPrefixes = []string{"utm_", "af_", "aff_", "matt_", "affiliate_"}

// StripTracking remove parâmetros de atribuição, userinfo e fragmento.
// Devolve a URL limpa. Nunca devolve o original em erro — o chamador que
// receber erro deve bloquear, não relayar cru.
func StripTracking(u *url.URL) string {
	c := *u
	c.User = nil
	c.Fragment = ""
	c.RawFragment = ""

	q := c.Query()
	for name := range q {
		lower := strings.ToLower(name)
		if trackingParams[lower] {
			q.Del(name)
			continue
		}
		for _, p := range trackingPrefixes {
			if strings.HasPrefix(lower, p) {
				q.Del(name)
				break
			}
		}
	}
	c.RawQuery = q.Encode()
	return c.String()
}

func cleanCanonical(u *url.URL) string { return StripTracking(u) }

func cleanCanonicalNoQuery(u *url.URL) string {
	c := *u
	c.User = nil
	c.Fragment = ""
	c.RawFragment = ""
	c.RawQuery = ""
	return c.String()
}
