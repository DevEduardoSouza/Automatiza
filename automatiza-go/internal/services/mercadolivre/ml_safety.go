package mercadolivre

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"time"

	"automatiza-go/internal/lib/commerceurl"
	"automatiza-go/internal/models"
)

// RemoteError é a falha tipada de uma chamada ao Mercado Livre.
//
// Existe porque o código antigo colapsava TODO não-200 em "preview vazio, erro
// nil". Isso é o que fazia um 429, um timeout ou um HTML mudado virarem
// "HasProductData=false", que o offer_generate lia como "então é campanha" e
// mandava um falso aviso de cupom ao grupo. Com o erro tipado, o chamador sabe
// separar transient (retry) de permanente (bloquear) e NUNCA infere campanha a
// partir de ausência de dados.
type RemoteError struct {
	StatusCode int
	Class      string // models.ErrorClass*
	Reason     string // models.Reason*
	Host       string
}

func (e *RemoteError) Error() string {
	return fmt.Sprintf("mercadolivre: %s (%s) status=%d host=%s", e.Reason, e.Class, e.StatusCode, e.Host)
}

// IsTransient reporta se vale retry.
func (e *RemoteError) IsTransient() bool { return e.Class == models.ErrorClassTransient }

// newRemoteError constrói o erro tipado a partir de um status HTTP.
func newRemoteError(status int, host string) *RemoteError {
	class, reason := commerceurl.ClassifyHTTPStatus(status)
	return &RemoteError{StatusCode: status, Class: class, Reason: reason, Host: host}
}

// mlFetchTimeout é o teto por request HTML do ML.
const mlFetchTimeout = 20 * time.Second

// safeMLClients guarda os dois clientes que o ML usa. Eles são distintos de
// propósito:
//
//   - anon segue redirects e nunca carrega credencial: é ele que resolve
//     meli.la e /sec/ até o destino final;
//   - authed NÃO segue redirect (§5.2). Um 3xx com cookie na mão precisa voltar
//     ao resolvedor anônimo, e só depois um novo request autenticado pode sair —
//     e apenas se o novo destino também estiver na allowlist exata.
type safeMLClients struct {
	anon   *commerceurl.SafeClient
	authed *commerceurl.SafeClient
}

func newSafeMLClients(opts commerceurl.ClientOptions) *safeMLClients {
	base := opts
	base.Store = models.MonitorStoreMercadoLivr
	base.Timeout = mlFetchTimeout

	anonOpts := base
	anonOpts.FollowRedirects = true

	authedOpts := base
	authedOpts.FollowRedirects = false

	return &safeMLClients{
		anon:   commerceurl.NewSafeClient(anonOpts),
		authed: commerceurl.NewSafeClient(authedOpts),
	}
}

// fetchHTML busca a página HTML de u com o cliente adequado.
//
// Regra de credencial: o cookie/CSRF só sai se AllowsMLCredentials(u) — host
// exato da allowlist, sobre HTTPS. Qualquer outro destino (subdomínio novo,
// short link, http) é buscado anonimamente. Se o request autenticado devolver
// 3xx, o cookie NÃO segue: reresolvemos anonimamente e só reautenticamos se o
// destino final continuar na allowlist.
//
// Devolve o corpo já limitado a commerceurl.MaxBodyBytes e a URL final segura.
func (c *safeMLClients) fetchHTML(ctx context.Context, u commerceurl.URL, creds *models.MLCredentials) ([]byte, commerceurl.URL, error) {
	headers := map[string]string{
		"Accept":     "text/html,application/xhtml+xml",
		"User-Agent": browserUserAgent,
	}

	useAuth := creds != nil && creds.Cookie != "" && commerceurl.AllowsMLCredentials(u)
	if useAuth {
		authHeaders := map[string]string{
			"Accept":     headers["Accept"],
			"User-Agent": headers["User-Agent"],
			"Cookie":     creds.Cookie,
		}
		if creds.CSRF != "" {
			authHeaders["X-Csrf-Token"] = creds.CSRF
		}
		resp, err := c.authed.Get(ctx, u, authHeaders)
		if err != nil {
			class, reason := commerceurl.ClassifyTransportError(err)
			return nil, u, &RemoteError{Class: class, Reason: reason, Host: u.Host}
		}
		if resp.StatusCode >= 300 && resp.StatusCode < 400 {
			// 3xx com cookie: descarta e reresolve anonimamente.
			resp.Body.Close()
			loc := resp.Header.Get("Location")
			if loc == "" {
				return nil, u, newRemoteError(resp.StatusCode, u.Host)
			}
			next, perr := resolveRelative(u, loc)
			if perr != nil {
				return nil, u, toRemoteError(perr, u.Host)
			}
			return c.fetchHTML(ctx, next, creds)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, u, newRemoteError(resp.StatusCode, u.Host)
		}
		body, rerr := commerceurl.ReadLimited(resp.Body)
		if rerr != nil {
			return nil, u, toRemoteError(rerr, u.Host)
		}
		return body, u, nil
	}

	resp, err := c.anon.Get(ctx, u, headers)
	if err != nil {
		class, reason := commerceurl.ClassifyTransportError(err)
		return nil, u, &RemoteError{Class: class, Reason: reason, Host: u.Host}
	}
	defer resp.Body.Close()

	final := u
	if resp.Request != nil && resp.Request.URL != nil {
		if f, perr := commerceurl.Parse(resp.Request.URL.String()); perr == nil {
			final = f
		}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, final, newRemoteError(resp.StatusCode, final.Host)
	}
	body, rerr := commerceurl.ReadLimited(resp.Body)
	if rerr != nil {
		return nil, final, toRemoteError(rerr, final.Host)
	}
	return body, final, nil
}

// resolveRelative resolve um Location (possivelmente relativo) contra base e
// revalida o resultado. Location nunca é confiável: é ele que a cadeia de
// redirect usa para escapar para rede interna.
func resolveRelative(base commerceurl.URL, location string) (commerceurl.URL, error) {
	ref, err := base.Parsed.Parse(location)
	if err != nil {
		return commerceurl.URL{}, &commerceurl.Error{
			Class:  models.ErrorClassSecurityBlock,
			Reason: models.ReasonInvalidURL,
			Host:   base.Host,
		}
	}
	return commerceurl.Parse(ref.String())
}

// toRemoteError converte qualquer erro do commerceurl em RemoteError,
// preservando classe e reason (um security_block nunca vira transient).
func toRemoteError(err error, host string) *RemoteError {
	class, reason := commerceurl.ClassifyTransportError(err)
	return &RemoteError{Class: class, Reason: reason, Host: host}
}

// ResolveSafeMLURL valida uma URL ML recebida de fora.
//
// Devolve erro tipado quando a URL não pode nem ser considerada: host que não é
// ML, host forjado, userinfo, IP literal, scheme errado. É a fronteira que
// impede o serviço de buscar qualquer coisa que o texto do grupo mandar.
func ResolveSafeMLURL(raw string) (commerceurl.URL, error) {
	u, err := commerceurl.Parse(raw)
	if err != nil {
		return commerceurl.URL{}, toRemoteError(err, "")
	}
	if u.Store != models.MonitorStoreMercadoLivr {
		return commerceurl.URL{}, &RemoteError{
			Class:  models.ErrorClassSecurityBlock,
			Reason: models.ReasonUnsafeHost,
			Host:   u.Host,
		}
	}
	return u, nil
}

// resolveShortLinkToProductURL resolve um short link de afiliado ML até a URL
// canônica do PRODUTO, anonimamente.
//
// "Anonimamente" é o ponto: quem postou o short é um terceiro, e mandar o
// cookie do nosso usuário para um destino que ainda não sabemos qual é seria
// entregar credencial a um host escolhido pela mensagem. Só depois de a cadeia
// terminar num host ML seguro é que o MLB é extraído.
//
// Não encontrar produto NÃO é campanha (§5.4 item 3): devolve erro
// unknown/transient e o chamador decide reter o parágrafo, nunca emitir a URL
// original.
func (s *mlService) resolveShortLinkToProductURL(ctx context.Context, raw string) (string, error) {
	u, err := ResolveSafeMLURL(raw)
	if err != nil {
		return "", err
	}

	body, final, err := s.safe.fetchHTML(ctx, u, nil)
	if err != nil {
		return "", err
	}

	// 1) A própria URL final já pode carregar o MLB (redirect para a página do
	//    anúncio é o caso comum).
	if _, ok := commerceurl.ExtractMLBID(final.Parsed); ok {
		return final.String(), nil
	}

	// 2) Senão, o permalink canônico embutido no HTML da página /social.
	if permalink := extractCanonicalProductPermalink(string(body)); permalink != "" {
		if p, perr := ResolveSafeMLURL(permalink); perr == nil {
			if _, ok := commerceurl.ExtractMLBID(p.Parsed); ok {
				return p.String(), nil
			}
		}
	}

	return "", &RemoteError{
		Class:  models.ErrorClassTransient,
		Reason: models.ReasonPreviewIncomplete,
		Host:   final.Host,
	}
}

// canonicalPermalinkRe captura o permalink canônico do produto embutido na
// página. Restrito a URLs absolutas ML: um href relativo ou de outro host não
// serve para reatribuir comissão e não pode virar destino de request.
var canonicalPermalinkRe = regexp.MustCompile(
	`https://[a-zA-Z0-9.\-]*mercadoliv?b?re\.com(?:\.br)?/[^"'\s\\<>]*MLB-?\d+[^"'\s\\<>]*`)

// extractCanonicalProductPermalink procura no HTML a primeira URL de produto ML
// absoluta. Devolve "" quando não há — e "" NUNCA vira campanha.
func extractCanonicalProductPermalink(html string) string {
	if m := canonicalPermalinkRe.FindString(html); m != "" {
		return m
	}
	return ""
}
