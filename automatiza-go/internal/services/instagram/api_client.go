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
// (container -> status -> publish). Host de negócio graph.instagram.com,
// versão v25.0.
//
// Fail-soft: se faltar credencial de app, Enabled() é false e a feature fica
// morta — nunca quebra o fluxo quente de quem chama.
const apiVersion = "v25.0"

// Sentinels de classificação. O worker decide retry/terminalidade só com
// errors.Is — nunca lendo texto de erro, que muda ao sabor da Meta.
var (
	// ErrInstagramAuth: credencial/permissão inválida. Reconectar é a única saída.
	ErrInstagramAuth = errors.New("instagram: auth/permission error")
	// ErrInstagramTransient: vale a pena tentar de novo (5xx, rede, processamento).
	ErrInstagramTransient = errors.New("instagram: transient error")
	// ErrInstagramRateLimit: cota/limite de publicação estourado. Não é problema
	// de credencial — jamais deve pedir reconexão ao usuário.
	ErrInstagramRateLimit = errors.New("instagram: rate limit / quota exceeded")
	// ErrInstagramPermanent: recusa definitiva que não é de autenticação.
	ErrInstagramPermanent = errors.New("instagram: permanent error")
	// ErrInstagramTokenExpired: token guardado já venceu — não chega a sair request.
	ErrInstagramTokenExpired = errors.New("instagram: stored token expired")
	// ErrInstagramFeatureDisabled: kill switch desligado.
	ErrInstagramFeatureDisabled = errors.New("instagram: feature disabled")
)

// Categorias seguras (entram em log; nunca carregam texto da Meta).
const (
	categoryAuth      = "auth"
	categoryTransient = "transient"
	categoryRateLimit = "rate_limit"
	categoryPermanent = "permanent"
)

// MetaAPIError é a forma tipada de uma recusa da Graph API.
//
// Guarda apenas números e uma categoria: a mensagem da Meta pode conter a URL
// da mídia, o id da conta ou trechos do conteúdo, e este erro acaba em log e,
// indiretamente, perto do usuário.
type MetaAPIError struct {
	Operation    string
	HTTPStatus   int
	Code         int
	Subcode      int
	Transient    bool
	SafeCategory string
}

func (e *MetaAPIError) Error() string {
	return fmt.Sprintf("instagram %s: %s (http=%d code=%d subcode=%d)",
		e.Operation, e.SafeCategory, e.HTTPStatus, e.Code, e.Subcode)
}

// Unwrap liga o erro concreto ao sentinel da sua categoria, para errors.Is.
func (e *MetaAPIError) Unwrap() error {
	switch e.SafeCategory {
	case categoryAuth:
		return ErrInstagramAuth
	case categoryRateLimit:
		return ErrInstagramRateLimit
	case categoryTransient:
		return ErrInstagramTransient
	default:
		return ErrInstagramPermanent
	}
}

// transientSubcodes: subcodes de processamento de mídia que a Meta documenta
// como recuperáveis — retentar publica; tratar como permanente perderia o Story.
var transientSubcodes = map[int]bool{
	2207003: true, // falha ao baixar a mídia
	2207008: true, // erro de processamento do container
	2207027: true, // erro transitório de publicação
	2207032: true, // falha ao criar o container
	2207053: true, // erro desconhecido de upload
}

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
	return c != nil && c.appID != "" && c.appSecret != ""
}

// AppSecret expõe o segredo do app para a verificação de signed_request dos
// callbacks da Meta (mesmo segredo que assina aqueles payloads).
func (c *APIClient) AppSecret() string {
	if c == nil {
		return ""
	}
	return c.appSecret
}

// RedirectURI devolve a URI registrada, para validação de configuração.
func (c *APIClient) RedirectURI() string {
	if c == nil {
		return ""
	}
	return c.redirectURI
}

// AuthorizeURL monta (não faz request) a URL da tela de consentimento OAuth.
// Escopo de negócio: basic (perfil) + content_publish (postar). Nada de
// messages/comments — esta entrega não recebe nada de volta do Instagram.
func (c *APIClient) AuthorizeURL(state string) string {
	q := url.Values{}
	q.Set("client_id", c.appID)
	q.Set("redirect_uri", c.redirectURI)
	q.Set("response_type", "code")
	q.Set("scope", "instagram_business_basic,instagram_business_content_publish")
	q.Set("state", state)
	return c.baseAuthorize + "/oauth/authorize?" + q.Encode()
}

// dataEnvelope tolera as DUAS formas que o endpoint "Instagram API with
// Instagram Login" devolve na prática: a documentada `{"data":[{...}]}` e a
// legada (herança do Basic Display) com o objeto PLANO no topo. Em produção o
// oauth/access_token respondeu plano, e o parser rígido só de envelope quebrava
// a conexão de 100% dos usuários. UnmarshalJSON normaliza ambas para Data.
type dataEnvelope[T any] struct {
	Data []T `json:"data"`
	// diagKeys guarda as chaves de topo da resposta crua, só para diagnóstico
	// quando o resultado for rejeitado (nunca contém valores). Sem isto, um 200
	// com campos inesperados vira "permanent" mudo — foi essa cegueira que
	// escondeu o bug do OAuth por dias.
	diagKeys []string
}

func (e *dataEnvelope[T]) UnmarshalJSON(b []byte) error {
	e.diagKeys = topLevelJSONKeys(b)
	// Forma documentada: {"data":[{...}]}.
	var wrapper struct {
		Data []T `json:"data"`
	}
	if err := json.Unmarshal(b, &wrapper); err == nil && wrapper.Data != nil {
		e.Data = wrapper.Data
		return nil
	}
	// Forma legada: objeto plano no topo.
	var flat T
	if err := json.Unmarshal(b, &flat); err != nil {
		return err
	}
	e.Data = []T{flat}
	return nil
}

// warnUnexpectedShape loga um fingerprint SEGURO (só chaves de topo e contagem
// de itens, nunca valores) quando um 200 é decodificado mas não traz os campos
// que precisamos. É o diagnóstico que revela drift de formato da Meta.
func (c *APIClient) warnUnexpectedShape(op string, items int, keys []string) {
	if c == nil || c.logger == nil {
		return
	}
	c.logger.Warn("instagram: resposta 2xx sem os campos esperados",
		zap.String("operation", op),
		zap.Int("data_items", items),
		zap.Strings("top_keys", keys))
}

// oneOf exige exatamente um item no envelope. Zero itens é resposta inútil e
// mais de um é ambíguo — em ambos os casos, adivinhar publicaria no objeto errado.
func oneOf[T any](env dataEnvelope[T], op string) (T, error) {
	var zero T
	if len(env.Data) != 1 {
		return zero, &MetaAPIError{Operation: op, SafeCategory: categoryPermanent}
	}
	return env.Data[0], nil
}

// ExchangeCode troca o code de redirect pelo short-lived token e pelo id
// APP-SCOPED da conta.
//
// Atenção: o id devolvido aqui NÃO serve para publicar — é escopado ao app. O
// id profissional vem do /me. Confundir os dois publica no objeto errado.
func (c *APIClient) ExchangeCode(ctx context.Context, code string) (shortToken, appScopedUserID string, err error) {
	form := url.Values{}
	form.Set("client_id", c.appID)
	form.Set("client_secret", c.appSecret)
	form.Set("grant_type", "authorization_code")
	form.Set("redirect_uri", c.redirectURI)
	form.Set("code", code)

	// permissions é omitido de propósito: não é usado, e tipá-lo como string
	// quebrava o parse quando o Meta o devolvia como array. Chave desconhecida
	// é ignorada pelo json.Unmarshal.
	var env dataEnvelope[struct {
		AccessToken string     `json:"access_token"`
		UserID      flexString `json:"user_id"`
	}]
	if err := c.do(ctx, http.MethodPost, c.baseAPI+"/oauth/access_token", form, "exchange_code", &env); err != nil {
		return "", "", err
	}
	item, err := oneOf(env, "exchange_code")
	if err != nil {
		c.warnUnexpectedShape("exchange_code", len(env.Data), env.diagKeys)
		return "", "", err
	}
	if item.AccessToken == "" || string(item.UserID) == "" {
		c.warnUnexpectedShape("exchange_code", len(env.Data), env.diagKeys)
		return "", "", &MetaAPIError{Operation: "exchange_code", SafeCategory: categoryPermanent}
	}
	return item.AccessToken, string(item.UserID), nil
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
	return validateTokenResponse(body, "exchange_long_lived")
}

// RefreshLongLived renova um long-lived token antes de expirar (mesmo shape da
// troca long-lived). A Meta exige token com pelo menos 24h de vida e ainda válido.
func (c *APIClient) RefreshLongLived(ctx context.Context, token string) (newToken string, expiresIn time.Duration, err error) {
	q := url.Values{}
	q.Set("grant_type", "ig_refresh_token")
	q.Set("access_token", token)

	var body tokenResponse
	if err := c.do(ctx, http.MethodGet, c.baseGraph+"/refresh_access_token?"+q.Encode(), nil, "refresh_long_lived", &body); err != nil {
		return "", 0, err
	}
	return validateTokenResponse(body, "refresh_long_lived")
}

// validateTokenResponse recusa token vazio e expires_in <= 0. Antes, expiry
// zerado era "corrigido" com 60 dias inventados — a credencial parecia válida
// por dois meses e todo envio no meio do caminho falhava sem explicação.
func validateTokenResponse(body tokenResponse, op string) (string, time.Duration, error) {
	if body.AccessToken == "" || body.ExpiresIn <= 0 {
		return "", 0, &MetaAPIError{Operation: op, SafeCategory: categoryPermanent}
	}
	return body.AccessToken, time.Duration(body.ExpiresIn) * time.Second, nil
}

// professionalAccountTypes: exatamente os dois valores do contrato oficial
// ("Business" e "Media_Creator"). A comparação é case-insensitive porque a API
// já respondeu em caixa alta em outros endpoints — mas nenhum valor além do
// contrato entra aqui.
var professionalAccountTypes = map[string]bool{
	"business":      true,
	"media_creator": true,
}

// GetMe devolve o id PROFISSIONAL, o username e o tipo da conta dona do token.
// Este é o id usado para publicar.
func (c *APIClient) GetMe(ctx context.Context, token string) (igUserID, username, accountType string, err error) {
	q := url.Values{}
	q.Set("fields", "user_id,username,account_type")
	q.Set("access_token", token)

	var env dataEnvelope[struct {
		UserID      flexString `json:"user_id"`
		Username    string     `json:"username"`
		AccountType string     `json:"account_type"`
	}]
	if err := c.do(ctx, http.MethodGet, c.graphURL("me")+"?"+q.Encode(), nil, "get_me", &env); err != nil {
		return "", "", "", err
	}
	item, err := oneOf(env, "get_me")
	if err != nil {
		c.warnUnexpectedShape("get_me", len(env.Data), env.diagKeys)
		return "", "", "", err
	}
	if string(item.UserID) == "" {
		c.warnUnexpectedShape("get_me", len(env.Data), env.diagKeys)
		return "", "", "", &MetaAPIError{Operation: "get_me", SafeCategory: categoryPermanent}
	}
	// account_type não aparece no exemplo oficial do envelope, então ausência
	// não reprova a conta; só um valor PRESENTE e não-profissional reprova.
	if item.AccountType != "" && !professionalAccountTypes[strings.ToLower(item.AccountType)] {
		return "", "", "", &MetaAPIError{Operation: "get_me", SafeCategory: categoryPermanent}
	}
	return string(item.UserID), item.Username, item.AccountType, nil
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
		return "", &MetaAPIError{Operation: "create_story", SafeCategory: categoryPermanent}
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
		return "", &MetaAPIError{Operation: "publish", SafeCategory: categoryPermanent}
	}
	return body.ID, nil
}

// GetPublishingLimit devolve quanto da cota de publicação (100 por janela móvel
// de 24h, por conta profissional) já foi usado. Data vazia = 0 usado.
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

// tokenResponse é o shape comum das respostas de token long-lived e refresh
// (estas SIM são objetos no topo, não envelope data[]).
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
// decodifica out no caminho feliz. Status fora de 2xx vira MetaAPIError tipado.
func (c *APIClient) do(ctx context.Context, method, urlStr string, form url.Values, op string, out any) error {
	var bodyReader io.Reader
	if form != nil {
		bodyReader = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, urlStr, bodyReader)
	if err != nil {
		return &MetaAPIError{Operation: op, SafeCategory: categoryPermanent}
	}
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		// Falha de rede/timeout é transitória. O erro cru fica fora: ele carrega
		// a URL completa, e a query traz o access_token.
		return &MetaAPIError{Operation: op, Transient: true, SafeCategory: categoryTransient}
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return &MetaAPIError{Operation: op, HTTPStatus: resp.StatusCode, Transient: true, SafeCategory: categoryTransient}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return c.apiError(op, resp.StatusCode, raw)
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			// Fingerprint SEGURO de uma resposta 2xx que não casou com o shape
			// esperado: só o tamanho e os NOMES das chaves de topo — nunca os
			// valores (token/id são secretos). A ausência disto foi o que
			// escondeu o bug do OAuth por dias (o erro saía só como "permanent").
			if c.logger != nil {
				c.logger.Warn("instagram: resposta 2xx com shape inesperado",
					zap.String("operation", op),
					zap.Int("http_status", resp.StatusCode),
					zap.Int("body_len", len(raw)),
					zap.Strings("top_keys", topLevelJSONKeys(raw)))
			}
			return &MetaAPIError{Operation: op, HTTPStatus: resp.StatusCode, SafeCategory: categoryPermanent}
		}
	}
	return nil
}

// topLevelJSONKeys devolve APENAS os nomes das chaves de topo de um JSON objeto
// (nunca os valores — podem conter token/id). Vazio se o corpo não for objeto.
func topLevelJSONKeys(raw []byte) []string {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil
	}
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	return keys
}

// graphError é o envelope de erro da Graph API.
type graphError struct {
	Error struct {
		Message      string `json:"message"`
		Type         string `json:"type"`
		Code         int    `json:"code"`
		ErrorSubcode int    `json:"error_subcode"`
		IsTransient  bool   `json:"is_transient"`
	} `json:"error"`
}

// apiError constrói o erro tipado de um status != 2xx. A mensagem da Meta é
// lida apenas para classificar; nunca é propagada.
func (c *APIClient) apiError(op string, status int, raw []byte) error {
	var ge graphError
	_ = json.Unmarshal(raw, &ge) // corpo não-JSON: classifica só pelo status

	return &MetaAPIError{
		Operation:    op,
		HTTPStatus:   status,
		Code:         ge.Error.Code,
		Subcode:      ge.Error.ErrorSubcode,
		Transient:    ge.Error.IsTransient,
		SafeCategory: classifyMetaError(status, ge.Error.Code, ge.Error.ErrorSubcode, ge.Error.IsTransient),
	}
}

// classifyMetaError decide a categoria. A ordem é a regra:
//
// cota antes de tudo (é o caso mais específico e NÃO deve pedir reconexão);
// 190 é a assinatura canônica de token inválido; is_transient da própria Meta
// vence status genérico; só então 401/403 viram auth. O 400 solto fica
// permanente-não-auth — antes TODO 400 pedia reconexão, o que transformava erro
// de processamento em "reconecte sua conta" e ainda matava o retry.
func classifyMetaError(status, code, subcode int, isTransient bool) string {
	switch {
	case code == 9 || subcode == 2207042:
		return categoryRateLimit
	case code == 190:
		return categoryAuth
	case isTransient:
		return categoryTransient
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return categoryAuth
	case status == http.StatusTooManyRequests:
		return categoryTransient
	case status >= 500 && status <= 599:
		return categoryTransient
	case code == -1 || code == -2:
		return categoryTransient
	case transientSubcodes[subcode]:
		return categoryTransient
	default:
		// Inclui 400: recusa definitiva desta requisição, mas não de credencial.
		return categoryPermanent
	}
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

// --- resposta (Fase 3/4) ---

// ReplyToComment publica uma resposta a um comentário.
//
// Endpoint oficial: POST /{IG_COMMENT_ID}/replies com `message`. Exige a
// permissão instagram_business_manage_comments. Sem janela de tempo — por isso
// comentário é o primeiro canal a ser ligado.
func (c *APIClient) ReplyToComment(ctx context.Context, commentID, token, message string) (replyID string, err error) {
	form := url.Values{}
	form.Set("message", message)
	form.Set("access_token", token)

	var body struct {
		ID string `json:"id"`
	}
	if err := c.do(ctx, http.MethodPost, c.graphURL(commentID, "replies"), form, "reply_comment", &body); err != nil {
		return "", err
	}
	if body.ID == "" {
		return "", &MetaAPIError{Operation: "reply_comment", SafeCategory: categoryPermanent}
	}
	return body.ID, nil
}

// SendDirectMessage responde uma mensagem no Direct.
//
// Endpoint oficial: POST /{IG_ID}/messages com corpo JSON
// {recipient:{id}, message:{text}} — diferente do resto da API, que é
// form-urlencoded. O id do PATH é o da conta profissional do cliente; o
// recipient é o IGSID de quem escreveu, que vem no webhook.
//
// A Meta dá 24h para responder. O gate de janela fica no worker, ANTES de
// chegar aqui: melhor não gastar a chamada do que receber a recusa dela.
func (c *APIClient) SendDirectMessage(ctx context.Context, igUserID, token, recipientIGSID, text string) (messageID string, err error) {
	payload := map[string]any{
		"recipient": map[string]string{"id": recipientIGSID},
		"message":   map[string]string{"text": text},
	}
	var body struct {
		MessageID string `json:"message_id"`
		ID        string `json:"id"`
	}
	if err := c.doJSON(ctx, c.graphURL(igUserID, "messages"), token, payload, "send_dm", &body); err != nil {
		return "", err
	}
	return firstNonEmptyStr(body.MessageID, body.ID), nil
}

func firstNonEmptyStr(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// doJSON existe porque o endpoint de mensagens é o único que espera corpo JSON
// e token no header Authorization, em vez do form-urlencoded com access_token
// na query que o resto da API usa.
func (c *APIClient) doJSON(ctx context.Context, urlStr, token string, payload any, op string, out any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return &MetaAPIError{Operation: op, SafeCategory: categoryPermanent}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, urlStr, strings.NewReader(string(raw)))
	if err != nil {
		return &MetaAPIError{Operation: op, SafeCategory: categoryPermanent}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.http.Do(req)
	if err != nil {
		return &MetaAPIError{Operation: op, Transient: true, SafeCategory: categoryTransient}
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return &MetaAPIError{Operation: op, HTTPStatus: resp.StatusCode, Transient: true, SafeCategory: categoryTransient}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return c.apiError(op, resp.StatusCode, respBody)
	}
	if out != nil {
		if err := json.Unmarshal(respBody, out); err != nil {
			return &MetaAPIError{Operation: op, HTTPStatus: resp.StatusCode, SafeCategory: categoryPermanent}
		}
	}
	return nil
}
