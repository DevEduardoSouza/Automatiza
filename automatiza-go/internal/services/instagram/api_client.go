package instagram

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.uber.org/zap"
)

// APIClient fala com a Graph API do Instagram na variante "Instagram API with
// Instagram Login" (login de negócio direto, SEM Facebook Page). Cobre o fluxo
// OAuth (code -> short -> long-lived -> refresh) e a publicação de Stories
// (container -> status -> publish). Confirmado na doc oficial
// developers.facebook.com: host de negócio graph.instagram.com, versão v25.0.
//
// Fail-soft: se faltar credencial de app, Enabled() é false e a feature fica
// morta — nunca quebra o fluxo quente de quem chama.
const apiVersion = "v25.0"

// ErrInstagramAuth marca erro PERMANENTE (auth/permissão): 4xx de OAuth, token
// inválido ou código 190. O worker usa errors.Is para decidir SkipRetry — erro
// TRANSITÓRIO (5xx, timeout de rede) NÃO carrega este sentinel e pode reprocessar.
var ErrInstagramAuth = errors.New("instagram: auth/permission error")

type APIClient struct {
	appID       string
	appSecret   string
	redirectURI string
	http        *http.Client
	logger      *zap.Logger

	// Hosts base — defaults nos hosts reais da Meta; sobrescrevíveis no teste
	// do mesmo pacote para apontar ao httptest. Não mexer em produção.
	baseGraph     string // graph.instagram.com — tokens long-lived, perfil, mídia
	baseAPI       string // api.instagram.com — troca code->short token
	baseAuthorize string // www.instagram.com — tela de consentimento OAuth
}

// NewAPIClient monta o client. Sem appID/appSecret, Enabled() é false e a
// integração Instagram fica desligada (fail-soft, nunca quebra).
func NewAPIClient(appID, appSecret, redirectURI string, logger *zap.Logger) *APIClient {
	return &APIClient{
		appID:         appID,
		appSecret:     appSecret,
		redirectURI:   redirectURI,
		http:          &http.Client{Timeout: 30 * time.Second},
		logger:        logger,
		baseGraph:     "https://graph.instagram.com",
		baseAPI:       "https://api.instagram.com",
		baseAuthorize: "https://www.instagram.com",
	}
}

// Enabled diz se há credenciais de app para falar com o Instagram.
func (c *APIClient) Enabled() bool {
	return c.appID != "" && c.appSecret != ""
}

// AuthorizeURL monta (não faz request) a URL da tela de consentimento OAuth.
// Escopo de negócio: basic (perfil) + content_publish (postar).
func (c *APIClient) AuthorizeURL(state string) string {
	q := url.Values{}
	q.Set("client_id", c.appID)
	q.Set("redirect_uri", c.redirectURI)
	q.Set("response_type", "code")
	q.Set("scope", "instagram_business_basic,instagram_business_content_publish")
	q.Set("state", state)
	return c.baseAuthorize + "/oauth/authorize?" + q.Encode()
}

// ExchangeCode troca o code de redirect por um short-lived token + o id da conta
// IG. user_id pode vir number OU string no JSON — flexString trata os dois.
func (c *APIClient) ExchangeCode(ctx context.Context, code string) (shortToken, igUserID string, err error) {
	form := url.Values{}
	form.Set("client_id", c.appID)
	form.Set("client_secret", c.appSecret)
	form.Set("grant_type", "authorization_code")
	form.Set("redirect_uri", c.redirectURI)
	form.Set("code", code)

	var body struct {
		AccessToken string     `json:"access_token"`
		UserID      flexString `json:"user_id"`
	}
	if err := c.do(ctx, http.MethodPost, c.baseAPI+"/oauth/access_token", form, "exchange_code", &body); err != nil {
		return "", "", err
	}
	if body.AccessToken == "" {
		return "", "", fmt.Errorf("instagram exchange_code: empty access_token")
	}
	return body.AccessToken, string(body.UserID), nil
}

// ExchangeLongLived troca o short-lived por um long-lived token (~60 dias).
func (c *APIClient) ExchangeLongLived(ctx context.Context, shortToken string) (token string, expiresIn time.Duration, err error) {
	q := url.Values{}
	q.Set("grant_type", "ig_exchange_token")
	q.Set("client_secret", c.appSecret)
	q.Set("access_token", shortToken)

	var body tokenResponse
	if err := c.do(ctx, http.MethodGet, c.baseGraph+"/access_token?"+q.Encode(), nil, "exchange_long_lived", &body); err != nil {
		return "", 0, err
	}
	if body.AccessToken == "" {
		return "", 0, fmt.Errorf("instagram exchange_long_lived: empty access_token")
	}
	return body.AccessToken, time.Duration(body.ExpiresIn) * time.Second, nil
}

// RefreshLongLived renova um long-lived token antes de expirar (mesmo shape da
// troca long-lived).
func (c *APIClient) RefreshLongLived(ctx context.Context, token string) (newToken string, expiresIn time.Duration, err error) {
	q := url.Values{}
	q.Set("grant_type", "ig_refresh_token")
	q.Set("access_token", token)

	var body tokenResponse
	if err := c.do(ctx, http.MethodGet, c.baseGraph+"/refresh_access_token?"+q.Encode(), nil, "refresh_long_lived", &body); err != nil {
		return "", 0, err
	}
	if body.AccessToken == "" {
		return "", 0, fmt.Errorf("instagram refresh_long_lived: empty access_token")
	}
	return body.AccessToken, time.Duration(body.ExpiresIn) * time.Second, nil
}

// GetMe devolve o id e o username da conta IG dona do token.
func (c *APIClient) GetMe(ctx context.Context, token string) (igUserID, username string, err error) {
	q := url.Values{}
	q.Set("fields", "user_id,username")
	q.Set("access_token", token)

	var body struct {
		UserID   flexString `json:"user_id"`
		Username string     `json:"username"`
	}
	if err := c.do(ctx, http.MethodGet, c.graphURL("me")+"?"+q.Encode(), nil, "get_me", &body); err != nil {
		return "", "", err
	}
	return string(body.UserID), body.Username, nil
}

// CreateStoryContainer cria o container de mídia de um Story a partir de uma URL
// pública de imagem. Story ignora caption — por isso não a enviamos. Retorna o
// creation_id a ser publicado depois que o status virar FINISHED.
func (c *APIClient) CreateStoryContainer(ctx context.Context, igUserID, token, imageURL string) (creationID string, err error) {
	form := url.Values{}
	form.Set("media_type", "STORIES")
	form.Set("image_url", imageURL)
	form.Set("access_token", token)

	var body struct {
		ID string `json:"id"`
	}
	if err := c.do(ctx, http.MethodPost, c.graphURL(igUserID, "media"), form, "create_story", &body); err != nil {
		return "", err
	}
	if body.ID == "" {
		return "", fmt.Errorf("instagram create_story: empty container id")
	}
	return body.ID, nil
}

// GetContainerStatus lê o status_code de um container
// (FINISHED|IN_PROGRESS|ERROR|EXPIRED|PUBLISHED).
func (c *APIClient) GetContainerStatus(ctx context.Context, containerID, token string) (statusCode string, err error) {
	q := url.Values{}
	q.Set("fields", "status_code")
	q.Set("access_token", token)

	var body struct {
		StatusCode string `json:"status_code"`
	}
	if err := c.do(ctx, http.MethodGet, c.graphURL(containerID)+"?"+q.Encode(), nil, "container_status", &body); err != nil {
		return "", err
	}
	return body.StatusCode, nil
}

// PublishContainer publica um container FINISHED e devolve o media_id.
func (c *APIClient) PublishContainer(ctx context.Context, igUserID, token, creationID string) (mediaID string, err error) {
	form := url.Values{}
	form.Set("creation_id", creationID)
	form.Set("access_token", token)

	var body struct {
		ID string `json:"id"`
	}
	if err := c.do(ctx, http.MethodPost, c.graphURL(igUserID, "media_publish"), form, "publish", &body); err != nil {
		return "", err
	}
	if body.ID == "" {
		return "", fmt.Errorf("instagram publish: empty media id")
	}
	return body.ID, nil
}

// GetPublishingLimit devolve quanto da cota de publicação (janela de 24h) já foi
// usada pela conta. Data vazia = 0 usado.
func (c *APIClient) GetPublishingLimit(ctx context.Context, igUserID, token string) (quotaUsed int, err error) {
	q := url.Values{}
	q.Set("fields", "quota_usage")
	q.Set("access_token", token)

	var body struct {
		Data []struct {
			QuotaUsage int `json:"quota_usage"`
		} `json:"data"`
	}
	if err := c.do(ctx, http.MethodGet, c.graphURL(igUserID, "content_publishing_limit")+"?"+q.Encode(), nil, "publishing_limit", &body); err != nil {
		return 0, err
	}
	if len(body.Data) == 0 {
		return 0, nil
	}
	return body.Data[0].QuotaUsage, nil
}

// tokenResponse é o shape comum das respostas de token long-lived e refresh.
type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int64  `json:"expires_in"` // segundos
}

// graphURL monta um path versionado sob o host da Graph API
// (baseGraph/v25.0/seg1/seg2...). Cada segmento é escapado.
func (c *APIClient) graphURL(segments ...string) string {
	parts := make([]string, 0, len(segments)+2)
	parts = append(parts, c.baseGraph, apiVersion)
	for _, s := range segments {
		parts = append(parts, url.PathEscape(s))
	}
	return strings.Join(parts, "/")
}

// do executa a request (GET sem corpo, POST com form-urlencoded), lê o corpo e
// decodifica out no caminho feliz. Status fora de 2xx vira erro via apiError,
// que distingue permanente (ErrInstagramAuth) de transitório.
func (c *APIClient) do(ctx context.Context, method, urlStr string, form url.Values, op string, out any) error {
	var bodyReader io.Reader
	if form != nil {
		bodyReader = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, urlStr, bodyReader)
	if err != nil {
		return fmt.Errorf("instagram %s: build request: %w", op, err)
	}
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		// Falha de rede/timeout é transitória — não carrega ErrInstagramAuth.
		return fmt.Errorf("instagram %s: %w", op, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("instagram %s: read body: %w", op, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return c.apiError(op, resp.StatusCode, raw)
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("instagram %s: decode: %w", op, err)
		}
	}
	return nil
}

// graphError é o envelope de erro da Graph API.
type graphError struct {
	Error struct {
		Message      string `json:"message"`
		Type         string `json:"type"`
		Code         int    `json:"code"`
		ErrorSubcode int    `json:"error_subcode"`
	} `json:"error"`
}

// apiError constrói o erro de um status != 2xx, decodificando o envelope quando
// possível. Envelopa ErrInstagramAuth em 400/401/403 ou code 190 (permanente).
func (c *APIClient) apiError(op string, status int, raw []byte) error {
	msg := strings.TrimSpace(string(raw))
	var ge graphError
	if json.Unmarshal(raw, &ge) == nil && ge.Error.Message != "" {
		msg = ge.Error.Message
	}
	if isAuthError(status, ge.Error.Code) {
		return fmt.Errorf("instagram %s: http %d: %s: %w", op, status, msg, ErrInstagramAuth)
	}
	return fmt.Errorf("instagram %s: http %d: %s", op, status, msg)
}

// isAuthError classifica um erro como permanente (auth/permissão): 4xx de OAuth
// (400/401/403) ou o código 190 do OAuth. 5xx e demais status ficam transitórios.
func isAuthError(status, code int) bool {
	return status == http.StatusBadRequest ||
		status == http.StatusUnauthorized ||
		status == http.StatusForbidden ||
		code == 190
}

// flexString decodifica um campo JSON que pode vir como number OU string
// (o user_id do Instagram varia entre os endpoints) e o normaliza para string.
type flexString string

func (f *flexString) UnmarshalJSON(data []byte) error {
	s := strings.TrimSpace(string(data))
	if s == "" || s == "null" {
		*f = ""
		return nil
	}
	// Remove aspas se for string; number cru passa direto.
	*f = flexString(strings.Trim(s, `"`))
	return nil
}
