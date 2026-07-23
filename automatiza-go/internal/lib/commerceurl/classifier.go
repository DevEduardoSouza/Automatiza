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
	case models.MonitorStoreUnsupported:
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
var trackingPrefixes = []string{"utm_", "af_", "matt_", "affiliate_"}

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
