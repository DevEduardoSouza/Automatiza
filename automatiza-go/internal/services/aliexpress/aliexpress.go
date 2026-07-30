// Package aliexpress converte links do AliExpress para o link de afiliado do
// usuário via API oficial de afiliados.
//
// É o único dos três conversores novos que faz chamada de rede por link — o
// AliExpress não expõe um formato de link determinístico como a tag da Amazon
// ou o deeplink da Awin. Por isso ele carrega o mesmo arsenal da Shopee:
// cache de resultado, breaker e cooldown de credencial ruim. A regra de ouro
// continua valendo: o pior desfecho é o link sumir do corpo, nunca a mensagem
// inteira ser retida nem a automação ser pausada.
package aliexpress

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"automatiza-go/internal/lib/cache"
	"automatiza-go/internal/lib/commerceurl"
	"automatiza-go/internal/repository"
	"automatiza-go/internal/services/affiliatelink"
)

const (
	// storeKey é a chave em affiliate_credentials.store.
	storeKey = "aliexpress"
	// defaultBaseURL é o gateway TOP de produção.
	defaultBaseURL = "https://api-sg.aliexpress.com/sync"
	// apiMethod gera o link de promoção a partir de uma URL de produto.
	apiMethod = "aliexpress.affiliate.link.generate"
	// signParam é o nome do parâmetro de assinatura (excluído do próprio cálculo).
	signParam = "sign"
	// linkCacheTTL espelha o cache de link do ML (6h): link de afiliado é
	// estável, e cada miss custa cota da API.
	linkCacheTTL = 6 * time.Hour
	// badCredentialTTL silencia um usuário com credencial errada por 1h, em
	// vez de queimar cota a cada disparo. Mesma ideia do cooldown de 10035 da
	// Shopee — só que aqui NUNCA pausa a automação.
	badCredentialTTL = time.Hour
	// requestTimeout: o conversor roda dentro do scheduler, que é serial.
	requestTimeout = 8 * time.Second
)

// Converter implementa affiliatelink.Converter para o AliExpress.
type Converter struct {
	affiliateRepo repository.AffiliateCredentialsRepository
	cache         *cache.Cache
	logger        *zap.Logger
	httpClient    *http.Client
	breaker       *breaker
	baseURL       string
}

// New monta o conversor. baseURL vazio usa produção (parametrizado nos testes).
func New(
	affiliateRepo repository.AffiliateCredentialsRepository,
	c *cache.Cache,
	logger *zap.Logger,
	baseURL string,
) *Converter {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &Converter{
		affiliateRepo: affiliateRepo,
		cache:         c,
		logger:        logger,
		httpClient:    &http.Client{Timeout: requestTimeout},
		breaker:       newBreaker(5, 30*time.Second),
		baseURL:       baseURL,
	}
}

type credentials struct {
	appKey     string
	appSecret  string
	trackingID string
}

// Convert devolve o link de promoção do usuário para a URL de produto.
func (c *Converter) Convert(ctx context.Context, u commerceurl.URL, userID string) (string, error) {
	creds, err := c.credentials(ctx, userID)
	if err != nil {
		return "", err
	}

	// Tracking da fonte fora antes de qualquer coisa: ele não pode entrar nem
	// na chave de cache nem no link pedido à API.
	source := commerceurl.StripTracking(u.Parsed)

	cacheKey := linkCacheKey(userID, source)
	if cached := c.cachedLink(ctx, cacheKey); cached != "" {
		return cached, nil
	}

	if c.inBadCredentialCooldown(ctx, userID) {
		return "", affiliatelink.ErrRejectedCredential
	}
	if !c.breaker.allow() {
		return "", affiliatelink.ErrUnavailable
	}

	link, err := c.generateLink(ctx, creds, source)
	if err != nil {
		switch {
		case isRejectedCredential(err):
			c.breaker.recordSuccess() // o serviço respondeu; o problema é a credencial
			c.startBadCredentialCooldown(ctx, userID)
			return "", affiliatelink.ErrRejectedCredential
		case isPermanent(err):
			c.breaker.recordSuccess()
			return "", affiliatelink.ErrUnconvertible
		default:
			c.breaker.recordFailure()
			return "", affiliatelink.ErrUnavailable
		}
	}
	c.breaker.recordSuccess()

	c.storeLink(ctx, cacheKey, link)
	return link, nil
}

// generateLink faz a chamada assinada e extrai o promotion_link.
func (c *Converter) generateLink(ctx context.Context, creds credentials, sourceURL string) (string, error) {
	params := map[string]string{
		"method":              apiMethod,
		"app_key":             creds.appKey,
		"sign_method":         "hmac-sha256",
		"timestamp":           strconv.FormatInt(time.Now().UnixMilli(), 10),
		"v":                   "2.0",
		"format":              "json",
		"promotion_link_type": "0",
		"tracking_id":         creds.trackingID,
		"source_values":       sourceURL,
	}
	params[signParam] = sign(params, creds.appSecret)

	form := url.Values{}
	for k, v := range params {
		form.Set(k, v)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests {
		return "", fmt.Errorf("aliexpress: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	return parsePromotionLink(body)
}

// credentials lê e valida o trio exigido pela assinatura.
func (c *Converter) credentials(ctx context.Context, userID string) (credentials, error) {
	stored, err := c.affiliateRepo.GetByUserAndStore(ctx, userID, storeKey)
	if err != nil || stored == nil {
		return credentials{}, affiliatelink.ErrNoCredentials
	}

	out := credentials{
		appKey:     strings.TrimSpace(stored.GetCredential("appKey")),
		appSecret:  strings.TrimSpace(stored.GetCredential("appSecret")),
		trackingID: strings.TrimSpace(stored.GetCredential("trackingId")),
	}
	if out.appKey == "" {
		out.appKey = strings.TrimSpace(stored.GetCredential("app_key"))
	}
	if out.appSecret == "" {
		out.appSecret = strings.TrimSpace(stored.GetCredential("app_secret"))
	}
	if out.trackingID == "" {
		out.trackingID = strings.TrimSpace(stored.GetCredential("tracking_id"))
	}

	if out.appKey == "" || out.appSecret == "" || out.trackingID == "" {
		return credentials{}, affiliatelink.ErrNoCredentials
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Cache e cooldown
// ---------------------------------------------------------------------------

func linkCacheKey(userID, sourceURL string) string {
	sum := sha256.Sum256([]byte(sourceURL))
	return "alix_link:" + userID + ":" + hex.EncodeToString(sum[:])[:16]
}

func badCredentialKey(userID string) string { return "alix:badcred:" + userID }

func (c *Converter) cachedLink(ctx context.Context, key string) string {
	if c.cache == nil {
		return ""
	}
	v, err := c.cache.Get(ctx, key)
	if err != nil {
		return ""
	}
	return v
}

func (c *Converter) storeLink(ctx context.Context, key, link string) {
	if c.cache == nil || link == "" {
		return
	}
	if err := c.cache.Set(ctx, key, link, linkCacheTTL); err != nil {
		c.logger.Debug("aliexpress: falha ao cachear link", zap.Error(err))
	}
}

func (c *Converter) inBadCredentialCooldown(ctx context.Context, userID string) bool {
	if c.cache == nil {
		return false
	}
	ok, err := c.cache.Exists(ctx, badCredentialKey(userID))
	return err == nil && ok
}

func (c *Converter) startBadCredentialCooldown(ctx context.Context, userID string) {
	if c.cache == nil {
		return
	}
	if _, err := c.cache.SetNX(ctx, badCredentialKey(userID), "1", badCredentialTTL); err != nil {
		c.logger.Debug("aliexpress: falha ao gravar cooldown", zap.Error(err))
	}
}

// ---------------------------------------------------------------------------
// Resposta
// ---------------------------------------------------------------------------

// apiError é o envelope de erro do gateway TOP.
type apiError struct {
	Code    string `json:"code"`
	Message string `json:"msg"`
	SubCode string `json:"sub_code"`
	SubMsg  string `json:"sub_msg"`
}

// respErr carrega o código de erro para a classificação em Convert.
type respErr struct {
	apiError
}

func (e *respErr) Error() string {
	return fmt.Sprintf("aliexpress: %s/%s %s %s", e.Code, e.SubCode, e.Message, e.SubMsg)
}

// isRejectedCredential reconhece os erros determinísticos de credencial do
// gateway TOP: assinatura inválida, app key desconhecida, app sem permissão.
func isRejectedCredential(err error) bool {
	var e *respErr
	if !asRespErr(err, &e) {
		return false
	}
	joined := strings.ToLower(e.Code + " " + e.SubCode + " " + e.Message + " " + e.SubMsg)
	for _, marker := range []string{
		"invalidsignature", "invalid-signature", "invalid signature",
		"incomplete signature", "invalid_app_key", "invalidappkey",
		"missing signature", "app call limited", "permission",
	} {
		if strings.Contains(joined, marker) {
			return true
		}
	}
	return false
}

// isPermanent reconhece "essa URL não converte" (produto fora do programa,
// parâmetro inválido) — não vale retry, mas também não é culpa da credencial.
func isPermanent(err error) bool {
	var e *respErr
	if !asRespErr(err, &e) {
		return false
	}
	joined := strings.ToLower(e.Code + " " + e.SubCode + " " + e.Message + " " + e.SubMsg)
	return strings.Contains(joined, "invalid param") ||
		strings.Contains(joined, "invalid_param") ||
		strings.Contains(joined, "not exist")
}

func asRespErr(err error, target **respErr) bool {
	e, ok := err.(*respErr)
	if ok {
		*target = e
	}
	return ok
}

// parsePromotionLink extrai o link do envelope do gateway.
//
// O shape é aninhado e varia entre versões do gateway, então a varredura é
// tolerante: procura a primeira chave "promotion_link" em qualquer nível. O
// envelope de erro é checado ANTES — um erro de negócio volta com HTTP 200.
func parsePromotionLink(body []byte) (string, error) {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(body, &envelope); err != nil {
		return "", fmt.Errorf("aliexpress: resposta ilegivel: %w", err)
	}

	if raw, ok := envelope["error_response"]; ok {
		var e apiError
		_ = json.Unmarshal(raw, &e)
		return "", &respErr{apiError: e}
	}

	var generic interface{}
	if err := json.Unmarshal(body, &generic); err != nil {
		return "", fmt.Errorf("aliexpress: resposta ilegivel: %w", err)
	}
	if link := findPromotionLink(generic); link != "" {
		return link, nil
	}

	// 200 sem link e sem envelope de erro: o AliExpress devolve resp_code de
	// negócio assim. Tratar como falha transitória evita gravar cache vazio.
	return "", fmt.Errorf("aliexpress: resposta sem promotion_link")
}

// findPromotionLink varre o JSON decodificado atrás da chave promotion_link
// cujo valor seja uma URL.
func findPromotionLink(node interface{}) string {
	switch v := node.(type) {
	case map[string]interface{}:
		if raw, ok := v["promotion_link"]; ok {
			if s, ok := raw.(string); ok && strings.HasPrefix(s, "http") {
				return s
			}
			if nested := findPromotionLink(raw); nested != "" {
				return nested
			}
		}
		for _, child := range v {
			if link := findPromotionLink(child); link != "" {
				return link
			}
		}
	case []interface{}:
		for _, child := range v {
			if link := findPromotionLink(child); link != "" {
				return link
			}
		}
	}
	return ""
}
