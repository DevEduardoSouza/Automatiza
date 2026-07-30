// Package commerceurl é a ÚNICA fonte de verdade para parsear, validar e
// classificar URLs de loja nos modos monitoring e offer_generate.
//
// Por que ele existe: antes, cada caminho decidia "isto é Shopee?" com
// strings.Contains sobre a URL inteira. Isso classifica
// https://evil.example/shopee.com.br/x como Shopee e entrega essa URL a um
// expansor que segue redirect sem validar host nem IP de destino — SSRF com
// credencial da plataforma no meio. Aqui o host é comparado exato (ou por
// sufixo com fronteira de ponto) sobre o hostname JÁ parseado, e o cliente HTTP
// valida cada salto.
//
// Contrato de log: nada neste pacote emite a URL, a query ou o corpo. Erros
// carregam host + reason code; quem loga usa Hash8.
package commerceurl

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"unicode"

	"automatiza-go/internal/models"

	"golang.org/x/net/idna"
)

// MaxURLBytes é o teto de tamanho de uma URL aceita (§5.2).
const MaxURLBytes = 2048

// URL é uma URL de origem já validada e normalizada.
//
// Raw é preservada só para substituição textual no corpo (o texto do grupo
// contém a string original, não a normalizada). Ela NUNCA vai para log.
type URL struct {
	Raw    string
	Parsed *url.URL
	// Host normalizado: ASCII (punycode), minúsculo, sem ponto final.
	Host string
	// Store é models.MonitorStore* ou "" quando não é loja reconhecida.
	Store string
}

// String devolve a forma normalizada, sem fragmento. Usada como canonical_url
// quando a classe exige uma URL limpa.
func (u URL) String() string {
	if u.Parsed == nil {
		return ""
	}
	c := *u.Parsed
	c.Fragment = ""
	c.RawFragment = ""
	c.User = nil
	return c.String()
}

// Hash8 devolve os 8 primeiros bytes do SHA-256 da URL crua, em hex. É a única
// representação de URL que pode aparecer em log.
func (u URL) Hash8() string { return Hash8(u.Raw) }

// Hash8 é a forma livre de Hash8 para strings quaisquer.
func Hash8(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:8])
}

// SourceHash devolve o SHA-256 completo em hex, usado como source_link_hash.
func SourceHash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// Error é a falha tipada do pacote. Ela carrega a classe e o reason code
// fechados; a mensagem nunca contém a URL.
type Error struct {
	Class  string // models.ErrorClass*
	Reason string // models.Reason*
	Host   string // pode ser "" quando o parse falhou antes do host
}

func (e *Error) Error() string {
	if e.Host != "" {
		return fmt.Sprintf("commerceurl: %s (%s) host=%s", e.Reason, e.Class, e.Host)
	}
	return fmt.Sprintf("commerceurl: %s (%s)", e.Reason, e.Class)
}

func blockErr(reason, host string) *Error {
	return &Error{Class: models.ErrorClassSecurityBlock, Reason: reason, Host: host}
}

// AsError extrai um *Error de err, se houver.
func AsError(err error) (*Error, bool) {
	var e *Error
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}

// Parse valida e normaliza uma URL de origem.
//
// Recusa (todas como security_block, porque nenhuma delas é uma URL de loja
// legítima que valha uma segunda chance): tamanho acima de MaxURLBytes, scheme
// fora de http/https, userinfo presente, host vazio, IP literal, label vazio,
// erro de IDNA, caractere de controle e porta fora de {vazio,80,443}.
//
// http:80 é aceito aqui porque o primeiro salto de uma expansão pode ser HTTP;
// a política de redirect (checkHop) exige HTTPS a partir do segundo salto.
func Parse(raw string) (URL, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return URL{}, blockErr(models.ReasonInvalidURL, "")
	}
	if len(trimmed) > MaxURLBytes {
		return URL{}, blockErr(models.ReasonURLTooLong, "")
	}
	// Caractere de controle em qualquer posição: request splitting / log
	// injection. url.Parse aceita alguns deles.
	for _, r := range trimmed {
		if r < 0x20 || r == 0x7f {
			return URL{}, blockErr(models.ReasonInvalidURL, "")
		}
	}

	u, err := url.Parse(trimmed)
	if err != nil {
		return URL{}, blockErr(models.ReasonInvalidURL, "")
	}

	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return URL{}, blockErr(models.ReasonUnsafeScheme, "")
	}
	u.Scheme = scheme

	// Userinfo é o vetor clássico de spoof visual
	// (https://shopee.com.br@evil.test/). Não existe URL de loja legítima com
	// userinfo, então recusar é gratuito.
	if u.User != nil {
		return URL{}, blockErr(models.ReasonUnsafeHost, "")
	}

	hostname := u.Hostname()
	if hostname == "" {
		return URL{}, blockErr(models.ReasonInvalidURL, "")
	}

	// IP literal (v4 ou v6, com ou sem colchetes) nunca é loja.
	if net.ParseIP(strings.Trim(hostname, "[]")) != nil {
		return URL{}, blockErr(models.ReasonUnsafeHost, hostname)
	}

	host, err := normalizeHost(hostname)
	if err != nil {
		return URL{}, blockErr(models.ReasonUnsafeHost, "")
	}

	port := u.Port()
	switch scheme {
	case "https":
		if port != "" && port != "443" {
			return URL{}, blockErr(models.ReasonUnsafeHost, host)
		}
	case "http":
		if port != "" && port != "80" {
			return URL{}, blockErr(models.ReasonUnsafeHost, host)
		}
	}

	// Reescreve o Host normalizado sem porta default, para que URLs
	// equivalentes tenham a mesma forma canônica.
	u.Host = host
	u.Fragment = ""
	u.RawFragment = ""

	out := URL{Raw: trimmed, Parsed: u, Host: host}
	out.Store = StoreOf(host)

	// Host que NÃO é loja mas carrega um hostname comercial (no próprio host,
	// no path ou na query) é phishing, não link externo comum. Classificar como
	// non_commerce deixaria a URL ser relayada crua; classificar como loja
	// mandaria o expansor buscá-la. Bloquear é o único desfecho seguro.
	if out.Store == "" && (hostSpoofsCommerce(host) || mentionsCommerceHost(u)) {
		return URL{}, blockErr(models.ReasonUnsafeHost, host)
	}

	return out, nil
}

// normalizeHost aplica IDNA lookup, minúsculas e remove um ponto final.
// Recusa label vazio e host com caractere não permitido.
func normalizeHost(h string) (string, error) {
	h = strings.TrimSuffix(strings.ToLower(strings.Trim(h, "[]")), ".")
	if h == "" {
		return "", errors.New("host vazio")
	}
	ascii, err := idna.Lookup.ToASCII(h)
	if err != nil {
		return "", err
	}
	ascii = strings.TrimSuffix(strings.ToLower(ascii), ".")
	if ascii == "" || strings.Contains(ascii, "..") {
		return "", errors.New("label vazio")
	}
	for _, part := range strings.Split(ascii, ".") {
		if part == "" {
			return "", errors.New("label vazio")
		}
	}
	for _, r := range ascii {
		if r > unicode.MaxASCII {
			return "", errors.New("host não-ASCII após IDNA")
		}
	}
	return ascii, nil
}

// hostMatches compara host com um domínio da tabela: igualdade exata ou
// sufixo com fronteira de ponto. "xshopee.com.br" NÃO casa com "shopee.com.br".
func hostMatches(host, domain string) bool {
	if host == domain {
		return true
	}
	return strings.HasSuffix(host, "."+domain)
}

var shopeeHosts = []string{
	"shopee.com.br",
	"shope.ee",
	"shp.ee",
}

var mlHosts = []string{
	"mercadolivre.com.br",
	// mercadolivre.com (sem .br) é domínio próprio do ML e é a forma que
	// aparece nos short links de afiliado "/sec/". O plano lista só a variante
	// .com.br e a grafia espanhola mercadolibre.com; incluir esta fecha a
	// lacuna sem afrouxar nada — continua sendo comparação de host exato.
	"mercadolivre.com",
	"mercadolibre.com",
	"ml.com.br",
	"meli.la",
}

var amazonHosts = []string{
	"amazon.com.br",
	"amazon.com",
	// Encurtadores: reconhecidos como loja para não vazarem cru no espelho,
	// mas sem ASIN no path a conversão devolve ErrUnconvertible (o link some).
	"amzn.to",
	"amzn.com",
	"a.co",
}

var aliexpressHosts = []string{
	"aliexpress.com",
	// s.click.aliexpress.com é o encurtador de afiliado; cai no domínio
	// principal pela fronteira de ponto, mas fica explícito aqui.
	"s.click.aliexpress.com",
}

// awinHosts é o domínio de redirect da própria rede Awin. Um link cread.php
// que chega do grupo-fonte carrega o awinaffid de OUTRO afiliado — precisa ser
// reconhecido como loja para ser reatribuído, não relayado.
var awinHosts = []string{"awin1.com"}

// awinAdvertiserHosts são as lojas atendidas via rede Awin. Deliberadamente
// FORA de allCommerceHosts: são domínios de varejo comuníssimos, e incluí-los
// faria mentionsCommerceHost bloquear URLs que hoje passam normalmente (um
// "google.com/url?q=...netshoes.com.br..." viraria security_block). O
// anti-spoof protege host que recebe credencial numa expansão de rede; a
// conversão Awin é reescrita determinística, sem expansor a proteger.
var awinAdvertiserHosts = []string{
	"netshoes.com.br",
	"centauro.com.br",
	"dafiti.com.br",
	"cea.com.br",
	"belezanaweb.com.br",
	"natura.com.br",
	"boticario.com.br",
	"eudora.com.br",
	"amend.com.br",
	"amaro.com",
	"zattini.com.br",
	"shop2gether.com.br",
	"tricae.com.br",
}

// sheinHosts cobre o site (br.shein.com) e o encurtador do programa de
// afiliados (onelink.shein.com) pela fronteira de ponto. A Shein não expõe API
// de geração de link nem parâmetro de afiliado anexável — o link do afiliado é
// um onelink.shein.com/<n>/<código> opaco, gerado um a um no app. Por isso a
// "conversão" é substituição pelo link fixo do dono, e não reescrita da URL do
// produto.
//
// shein.com.br fica FORA de propósito: o site brasileiro é br.shein.com, e o
// host shein.com.br casaria hostSpoofsCommerce, virando security_block.
var sheinHosts = []string{"shein.com"}

// unsupportedHosts são as lojas SEM nenhuma conversão possível: o link é
// removido do corpo em vez de repassado com a comissão da fonte.
var unsupportedHosts = []string{
	"magazineluiza.com.br",
	"magazinevoce.com.br",
	"magalu.com",
}

// StoreOf classifica o host normalizado. Devolve "" quando não é loja.
//
// s.shopee.com.br cai em shopee.com.br pela fronteira de ponto; subdomínios de
// mercadolivre.com.br/mercadolibre.com idem.
func StoreOf(host string) string {
	for _, d := range shopeeHosts {
		if hostMatches(host, d) {
			return models.MonitorStoreShopee
		}
	}
	for _, d := range mlHosts {
		if hostMatches(host, d) {
			return models.MonitorStoreMercadoLivr
		}
	}
	for _, d := range amazonHosts {
		if hostMatches(host, d) {
			return models.MonitorStoreAmazon
		}
	}
	for _, d := range aliexpressHosts {
		if hostMatches(host, d) {
			return models.MonitorStoreAliExpress
		}
	}
	for _, d := range awinHosts {
		if hostMatches(host, d) {
			return models.MonitorStoreAwin
		}
	}
	for _, d := range awinAdvertiserHosts {
		if hostMatches(host, d) {
			return models.MonitorStoreAwin
		}
	}
	for _, d := range sheinHosts {
		if hostMatches(host, d) {
			return models.MonitorStoreShein
		}
	}
	for _, d := range unsupportedHosts {
		if hostMatches(host, d) {
			return models.MonitorStoreUnsupported
		}
	}
	return ""
}

// allCommerceHosts é a união usada na varredura de token. awinAdvertiserHosts
// fica de fora de propósito — ver o comentário na declaração.
var allCommerceHosts = func() []string {
	out := make([]string, 0,
		len(shopeeHosts)+len(mlHosts)+len(amazonHosts)+
			len(aliexpressHosts)+len(awinHosts)+len(sheinHosts)+len(unsupportedHosts))
	out = append(out, shopeeHosts...)
	out = append(out, mlHosts...)
	out = append(out, amazonHosts...)
	out = append(out, aliexpressHosts...)
	out = append(out, awinHosts...)
	out = append(out, sheinHosts...)
	out = append(out, unsupportedHosts...)
	return out
}()

// hostSpoofsCommerce detecta um domínio comercial embutido num host que NÃO é
// da loja — "shopee.com.br.evil.test", "www.meli.la.phish.test".
//
// Só é chamado depois de StoreOf devolver "": a comparação por sufixo com
// fronteira de ponto já teria reconhecido o host legítimo. Logo, encontrar o
// domínio como prefixo ou no meio dos labels prova que ele foi colado para
// enganar quem lê, e a URL é bloqueada em vez de relayada como link comum.
func hostSpoofsCommerce(host string) bool {
	for _, d := range allCommerceHosts {
		if strings.HasPrefix(host, d+".") || strings.Contains(host, "."+d+".") {
			return true
		}
	}
	return false
}

// mentionsCommerceHost procura um hostname comercial como token delimitado no
// path, na query ou no userinfo bruto de u.
//
// Fronteira: o caractere anterior não pode ser [a-z0-9.-] e o seguinte não pode
// ser [a-z0-9-]. Assim "evil.test/shopee.com.br/x" casa (seguido de "/"),
// "meushopee.com.br" não casa (precedido de "s") e "shopee.com.brz" não casa.
func mentionsCommerceHost(u *url.URL) bool {
	hay := strings.ToLower(u.EscapedPath() + "?" + u.RawQuery)
	for _, d := range allCommerceHosts {
		if containsHostToken(hay, d) {
			return true
		}
	}
	return false
}

func containsHostToken(hay, needle string) bool {
	from := 0
	for {
		i := strings.Index(hay[from:], needle)
		if i < 0 {
			return false
		}
		i += from
		end := i + len(needle)
		if !isHostBodyByte(prevByte(hay, i)) && !isHostTailByte(nextByte(hay, end)) {
			return true
		}
		from = i + 1
	}
}

func prevByte(s string, i int) byte {
	if i == 0 {
		return 0
	}
	return s[i-1]
}

func nextByte(s string, i int) byte {
	if i >= len(s) {
		return 0
	}
	return s[i]
}

// isHostBodyByte: caracteres que, ANTES do token, provam que ele é sufixo de um
// hostname maior (inclui "." para pegar "sub.shopee.com.br" dentro do path).
func isHostBodyByte(b byte) bool {
	return b == '.' || b == '-' || (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9')
}

// isHostTailByte: caracteres que, DEPOIS do token, provam que ele é prefixo de
// um hostname maior. "." não entra: "shopee.com.br." ainda é o mesmo host.
func isHostTailByte(b byte) bool {
	return b == '-' || (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9')
}

// PathSegments devolve os segmentos não vazios do path decodificado.
// Classificação por path SEMPRE usa isto — nunca strings.Contains sobre a URL
// inteira, senão "?next=/cupons" viraria campanha.
func PathSegments(u *url.URL) []string {
	p := u.Path
	if p == "" {
		return nil
	}
	raw := strings.Split(strings.Trim(p, "/"), "/")
	out := make([]string, 0, len(raw))
	for _, s := range raw {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

// HasPathPrefix reporta se o path de u começa pelo segmento seg.
func HasPathPrefix(u *url.URL, seg string) bool {
	segs := PathSegments(u)
	return len(segs) > 0 && strings.EqualFold(segs[0], seg)
}
