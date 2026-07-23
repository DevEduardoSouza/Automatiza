package commerceurl

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"syscall"
	"time"

	"automatiza-go/internal/models"
)

// MaxRedirects é o teto de saltos numa expansão (§5.2).
const MaxRedirects = 5

// MaxBodyBytes é o teto de corpo HTTP lido (§5.2). Lemos MaxBodyBytes+1 para
// distinguir "exatamente no limite" de "estourou" sem parsear HTML truncado.
const MaxBodyBytes = 5 << 20

// ErrResponseTooLarge indica corpo acima de MaxBodyBytes.
var ErrResponseTooLarge = &Error{Class: models.ErrorClassTransient, Reason: models.ReasonResponseTooLarge}

// SafeClient é um http.Client endurecido contra SSRF.
//
// As três garantias:
//  1. o dialer valida o IP JÁ RESOLVIDO em cada conexão (hook Control), então
//     não há janela entre resolver e conectar — nem no primeiro request, nem
//     em redirect, nem em retry de conexão;
//  2. transport próprio, sem proxy de ambiente (um HTTP_PROXY no container
//     mandaria o request para fora da validação);
//  3. política de redirect que exige HTTPS após o primeiro salto e host da
//     mesma loja, e apaga Cookie/Authorization ao trocar de host.
type SafeClient struct {
	client *http.Client
	// store, quando != "", prende toda a cadeia de redirect aos hosts dessa
	// loja. Vazio = qualquer host público válido (usado só onde a loja ainda
	// não é conhecida).
	store string
}

// safeControl recusa conectar a endereços não roteáveis publicamente.
// address chega como "ip:porta" com o IP já resolvido pelo runtime.
func safeControl(network, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return blockErr(models.ReasonPrivateAddress, "")
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return blockErr(models.ReasonPrivateAddress, "")
	}
	if !IsPublicIP(ip) {
		return blockErr(models.ReasonPrivateAddress, "")
	}
	if network != "tcp4" && network != "tcp6" && network != "tcp" {
		return blockErr(models.ReasonPrivateAddress, "")
	}
	return nil
}

// IsPublicIP reporta se ip é roteável publicamente.
//
// Rejeita loopback, link-local (v4 e v6), privado (RFC1918 / ULA fc00::/7),
// multicast, unspecified, e o endereço de metadata de cloud 169.254.169.254
// (já coberto por link-local, mas explicitado porque é o alvo real).
func IsPublicIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() ||
		ip.IsPrivate() {
		return false
	}
	if v4 := ip.To4(); v4 != nil {
		switch {
		case v4[0] == 0: // "this network"
			return false
		case v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127: // CGNAT 100.64/10
			return false
		case v4[0] == 192 && v4[1] == 0 && v4[2] == 0: // IETF protocol assignments
			return false
		case v4[0] == 192 && v4[1] == 0 && v4[2] == 2: // TEST-NET-1
			return false
		case v4[0] == 198 && (v4[1] == 18 || v4[1] == 19): // benchmarking
			return false
		case v4[0] == 198 && v4[1] == 51 && v4[2] == 100: // TEST-NET-2
			return false
		case v4[0] == 203 && v4[1] == 0 && v4[2] == 113: // TEST-NET-3
			return false
		case v4[0] >= 240: // reservado / broadcast
			return false
		}
		return true
	}
	// IPv6: recusa mapeado-v4 fora do bloco público e endereços reservados.
	if ip.Equal(net.IPv6loopback) {
		return false
	}
	return true
}

// ClientOptions configura um SafeClient.
type ClientOptions struct {
	// Store prende a cadeia de redirect aos hosts dessa loja.
	Store string
	// Timeout é o teto por request (inclui os redirects).
	Timeout time.Duration
	// FollowRedirects=false devolve o 3xx ao chamador em vez de segui-lo.
	// Usado no request autenticado do ML: um 3xx precisa voltar ao resolvedor
	// anônimo antes de qualquer novo request com cookie.
	FollowRedirects bool
	// DialContext permite injetar um dialer nos testes (httptest em loopback,
	// que o safeControl recusaria). Em produção fica nil.
	DialContext func(ctx context.Context, network, addr string) (net.Conn, error)
	// TLSClientConfig é o par de DialContext para httptest.NewTLSServer, cujo
	// certificado é auto-assinado. SOMENTE teste — em produção fica nil e o
	// transport usa a verificação padrão.
	TLSClientConfig *tls.Config
}

// NewSafeClient cria um cliente endurecido.
func NewSafeClient(opts ClientOptions) *SafeClient {
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	dial := opts.DialContext
	if dial == nil {
		d := &net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
			Control:   safeControl,
		}
		dial = d.DialContext
	}

	transport := &http.Transport{
		// Proxy nil de propósito: um proxy de ambiente burlaria safeControl.
		Proxy:                 nil,
		DialContext:           dial,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          32,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DisableCompression:    false,
		TLSClientConfig:       opts.TLSClientConfig,
	}

	sc := &SafeClient{store: opts.Store}
	sc.client = &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if !opts.FollowRedirects {
				return http.ErrUseLastResponse
			}
			return sc.checkHop(req, via)
		},
	}
	return sc
}

// checkHop valida cada salto de redirect.
func (s *SafeClient) checkHop(req *http.Request, via []*http.Request) error {
	if len(via) >= MaxRedirects {
		return blockErr(models.ReasonRedirectEscape, "")
	}

	target, err := Parse(req.URL.String())
	if err != nil {
		return err
	}

	// A partir do segundo salto só HTTPS: um redirect para http downgrade a
	// cadeia e permite interceptação no meio.
	if target.Parsed.Scheme != "https" {
		return blockErr(models.ReasonRedirectEscape, target.Host)
	}

	if s.store != "" && target.Store != s.store {
		return blockErr(models.ReasonRedirectEscape, target.Host)
	}

	// Cinto e suspensório: o net/http já apaga estes headers em redirect
	// cross-domain, mas aqui a regra é "trocou de host, apaga", inclusive
	// entre subdomínios da mesma loja.
	prev := via[len(via)-1]
	if prev.URL != nil && !strings.EqualFold(prev.URL.Hostname(), req.URL.Hostname()) {
		req.Header.Del("Cookie")
		req.Header.Del("Authorization")
		req.Header.Del("X-Api-Key")
		req.Header.Del("X-Csrf-Token")
	}
	return nil
}

// Do executa o request já validado.
func (s *SafeClient) Do(req *http.Request) (*http.Response, error) {
	return s.client.Do(req)
}

// Get faz um GET validando a URL antes de sair.
func (s *SafeClient) Get(ctx context.Context, u URL, headers map[string]string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.Parsed.String(), nil)
	if err != nil {
		return nil, blockErr(models.ReasonInvalidURL, u.Host)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return s.client.Do(req)
}

// ReadLimited lê no máximo MaxBodyBytes do corpo.
//
// Lê MaxBodyBytes+1 e devolve ErrResponseTooLarge se o byte excedente existir —
// nunca entrega HTML truncado ao parser, porque HTML truncado gera "200 sem
// dados de produto", que é exatamente o sinal que o classificador NÃO pode
// confundir com campanha.
func ReadLimited(r io.Reader) ([]byte, error) {
	buf, err := io.ReadAll(io.LimitReader(r, MaxBodyBytes+1))
	if err != nil {
		return nil, err
	}
	if len(buf) > MaxBodyBytes {
		return nil, ErrResponseTooLarge
	}
	return buf, nil
}

// ClassifyHTTPStatus mapeia um status HTTP para (classe, reason) segundo §9.
func ClassifyHTTPStatus(status int) (class, reason string) {
	switch {
	case status >= 200 && status < 300:
		return models.ErrorClassNone, models.ReasonNone
	case status >= 300 && status < 400:
		return models.ErrorClassTransient, models.ReasonHTTP3xx
	case status == http.StatusRequestTimeout:
		return models.ErrorClassTransient, models.ReasonHTTP408
	case status == http.StatusTooEarly:
		return models.ErrorClassTransient, models.ReasonHTTP425
	case status == http.StatusTooManyRequests:
		return models.ErrorClassTransient, models.ReasonHTTP429
	case status == http.StatusNotFound, status == http.StatusGone:
		return models.ErrorClassPermanentConte, models.ReasonNotFound
	case status == http.StatusUnauthorized, status == http.StatusForbidden:
		return models.ErrorClassPermanentConfig, models.ReasonInvalidToken
	case status >= 400 && status < 500:
		return models.ErrorClassPermanentConte, models.ReasonHTTP4xx
	default:
		return models.ErrorClassTransient, models.ReasonHTTP5xx
	}
}

// ClassifyTransportError mapeia um erro de transporte para (classe, reason).
//
// Erro de segurança (nosso *Error) atravessa intacto: ele já é terminal e não
// pode virar "transient/timeout" só porque veio embrulhado num url.Error.
func ClassifyTransportError(err error) (class, reason string) {
	if err == nil {
		return models.ErrorClassNone, models.ReasonNone
	}
	if e, ok := AsError(err); ok {
		return e.Class, e.Reason
	}
	if errors.Is(err, context.DeadlineExceeded) || os_IsTimeout(err) {
		return models.ErrorClassTransient, models.ReasonTimeout
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return models.ErrorClassTransient, models.ReasonDNS
	}
	return models.ErrorClassTransient, models.ReasonTimeout
}

func os_IsTimeout(err error) bool {
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}
