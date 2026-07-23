package instagram

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"automatiza-go/internal/lib/cache"
	"automatiza-go/internal/lib/crypto"
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	stateTTL = 10 * time.Minute
	// minStateKeyBytes: 32 bytes é o piso para uma chave HMAC-SHA256 fazer
	// sentido. Chave curta = state forjável = callback forjável.
	minStateKeyBytes = 32
	// retentionWindow / retentionBatch / retentionMaxBatches: a limpeza diária
	// é limitada de propósito. Um DELETE sem teto no ledger seguraria lock em
	// cima do caminho quente; sobra fica para o dia seguinte.
	retentionWindow     = 30 * 24 * time.Hour
	retentionBatch      = 1000
	retentionMaxBatches = 10
)

// ServiceConfig reúne o que o service precisa saber do ambiente. Agrupado num
// struct para o construtor não virar uma fila de strings posicionais onde
// trocar duas por engano compila e quebra em produção.
type ServiceConfig struct {
	StoriesEnabled  bool
	FrontendBaseURL string
	BackendBaseURL  string
	EncryptionKey   string
	StateSigningKey string
	// AIReplyEnabled é o kill switch GLOBAL da resposta automática, separado do
	// de Stories: dá para ter o canal ligado com a IA desligada para todos.
	AIReplyEnabled bool
	// RedirectURI é a URL de callback do OAuth. É NOSSA e idêntica para todos
	// os clientes — cada um só a cola no painel do próprio app da Meta. Quem
	// diz de qual usuário é o callback é o `state` assinado, não a URL.
	RedirectURI string
}

// DataDeletionResponse é o corpo que a Meta espera do callback de exclusão.
type DataDeletionResponse struct {
	URL              string `json:"url"`
	ConfirmationCode string `json:"confirmation_code"`
}

// Service orchestrates the per-user Instagram connection and story publishing.
// It is safe to construct even when the feature is disabled: every entry point
// degrades to a no-op / not-connected answer.
type Service struct {
	cfg          ServiceConfig
	api          *APIClient
	credsRepo    repository.InstagramCredentialsRepository
	storyRepo    repository.InstagramStoryPostRepository
	deletionRepo repository.InstagramDataDeletionRepository
	// appsRepo guarda o app da Meta de CADA cliente. Quando nil (ou sem linha
	// para o usuário), o service cai no app global de `api` — o modo antigo,
	// mantido para não quebrar quem já conectou por ele.
	appsRepo repository.InstagramAppRepository
	// inboundRepo guarda os eventos que chegam pelo webhook (Fase 2).
	inboundRepo repository.InstagramInboundRepository
	// Peças da resposta automática (Fase 3/4/5).
	aiConfigRepo repository.InstagramAIConfigRepository
	replyRepo    repository.InstagramReplyRepository
	planCapRepo  repository.InstagramPlanCapRepository
	aiGen        AIReplyGenerator
	// enqueueReply é injetado depois da construção porque a fila (workers) já
	// depende deste pacote — injetar aqui evita o ciclo de importação.
	enqueueReply func(ctx context.Context, ev *models.InstagramInboundEvent) error
	cache        *cache.Cache
	logger       *zap.Logger
}

// ServiceRepositories agrupa os repositórios do canal.
//
// Struct, e não parâmetros posicionais: eram treze argumentos, seis deles
// interfaces do MESMO formato — trocar dois de lugar compilava e só quebraria
// em produção, no caminho de credencial de cliente.
type ServiceRepositories struct {
	Credentials  repository.InstagramCredentialsRepository
	StoryPosts   repository.InstagramStoryPostRepository
	DataDeletion repository.InstagramDataDeletionRepository
	Apps         repository.InstagramAppRepository
	Inbound      repository.InstagramInboundRepository
	AIConfig     repository.InstagramAIConfigRepository
	Replies      repository.InstagramReplyRepository
	PlanCap      repository.InstagramPlanCapRepository
}

// NewService builds the Instagram service.
func NewService(
	cfg ServiceConfig,
	api *APIClient,
	repos ServiceRepositories,
	aiGen AIReplyGenerator,
	c *cache.Cache,
	logger *zap.Logger,
) *Service {
	return &Service{
		cfg:          cfg,
		api:          api,
		credsRepo:    repos.Credentials,
		storyRepo:    repos.StoryPosts,
		deletionRepo: repos.DataDeletion,
		appsRepo:     repos.Apps,
		inboundRepo:  repos.Inbound,
		aiConfigRepo: repos.AIConfig,
		replyRepo:    repos.Replies,
		planCapRepo:  repos.PlanCap,
		aiGen:        aiGen,
		cache:        c,
		logger:       logger,
	}
}

// SetReplyEnqueuer liga a ingestão à fila de resposta. Chamado no wiring, após
// a construção: o pacote workers importa este, então a dependência inversa tem
// de entrar por injeção.
func (s *Service) SetReplyEnqueuer(fn func(ctx context.Context, ev *models.InstagramInboundEvent) error) {
	s.enqueueReply = fn
}

// --- resolução do app da Meta por cliente ---

// ErrInstagramAppNotConfigured: o usuário ainda não cadastrou o app dele (e não
// há app global de fallback).
var ErrInstagramAppNotConfigured = errors.New("instagram: meta app not configured for user")

// apiClientFor devolve o client do app DAQUELE usuário.
//
// Não há cache aqui de propósito: decriptar o segredo é uma operação AES de
// microssegundos, irrelevante ao lado da chamada HTTP à Meta que vem logo
// depois. Um cache traria invalidação para gerenciar em troca de nada.
func (s *Service) apiClientFor(ctx context.Context, userID string) (*APIClient, error) {
	if s.appsRepo != nil {
		app, err := s.appsRepo.Get(ctx, userID)
		if err != nil {
			return nil, err
		}
		if app != nil {
			return s.clientFromApp(app)
		}
	}
	// Fallback: app global (modo antigo). Sem ele, o usuário precisa cadastrar.
	if s.api.Enabled() {
		return s.api, nil
	}
	return nil, ErrInstagramAppNotConfigured
}

// ClientByWebhookToken resolve o app a partir do token da URL do webhook —
// ANTES de ler o corpo da requisição, que é justamente o que permite validar a
// assinatura de um webhook multi-tenant.
func (s *Service) ClientByWebhookToken(ctx context.Context, webhookToken string) (*APIClient, *models.InstagramApp, error) {
	if s.appsRepo == nil {
		return nil, nil, ErrInstagramAppNotConfigured
	}
	app, err := s.appsRepo.GetByWebhookToken(ctx, webhookToken)
	if err != nil {
		return nil, nil, err
	}
	if app == nil {
		return nil, nil, ErrInstagramAppNotConfigured
	}
	client, err := s.clientFromApp(app)
	if err != nil {
		return nil, nil, err
	}
	return client, app, nil
}

// clientFromApp decripta o segredo e monta o client. O redirect URI é NOSSO e
// idêntico para todos os clientes — cada um apenas o cola no painel do próprio
// app; é o `state` assinado que diz de quem é o callback.
func (s *Service) clientFromApp(app *models.InstagramApp) (*APIClient, error) {
	var blob crypto.EncryptedBlobV1
	if err := json.Unmarshal([]byte(app.EncryptedAppSecret), &blob); err != nil {
		return nil, err
	}
	var secret models.InstagramAppSecret
	if err := crypto.DecryptJSONV1(&blob, s.cfg.EncryptionKey, &secret); err != nil {
		return nil, err
	}
	if app.AppID == "" || secret.AppSecret == "" {
		return nil, ErrInstagramAppNotConfigured
	}
	return NewAPIClient(app.AppID, secret.AppSecret, s.cfg.RedirectURI, s.logger), nil
}

// --- prontidão e kill switch ---

// redisReady exige Redis REAL e OPERACIONAL. Cache meramente presente não
// serve para state: com APP_CACHE desligado, Set vira no-op silencioso e o
// nonce nunca chegaria a existir.
func (s *Service) redisReady() bool {
	return s != nil && s.cache.Operational()
}

func isHTTPSURL(raw string) bool {
	if raw == "" {
		return false
	}
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Host != ""
}

// Configured diz se TODA a configuração necessária para o fluxo OAuth+publicação
// está presente e segura. Separado do kill switch: assim uma configuração
// incompleta nunca é confundida com "alguém desligou de propósito".
// Configured diz se a PLATAFORMA está pronta — não se um usuário específico
// está. Com app por cliente, exigir um app global aqui deixaria a feature
// desligada para todo mundo mesmo com clientes perfeitamente configurados.
// A prontidão do usuário é resolvida em apiClientFor.
func (s *Service) Configured() bool {
	return s != nil &&
		s.cfg.EncryptionKey != "" &&
		len(s.cfg.StateSigningKey) >= minStateKeyBytes &&
		s.redisReady() &&
		isHTTPSURL(s.redirectURI()) &&
		isHTTPSURL(s.cfg.BackendBaseURL) &&
		isHTTPSURL(s.cfg.FrontendBaseURL) &&
		s.credsRepo != nil &&
		s.storyRepo != nil
}

// redirectURI prefere o valor da config; cai no do app global apenas no modo
// antigo, em que o client global carregava essa informação.
func (s *Service) redirectURI() string {
	if s.cfg.RedirectURI != "" {
		return s.cfg.RedirectURI
	}
	return s.api.RedirectURI()
}

// TokenMaintenanceConfigured é deliberadamente mais frouxo que Configured: o
// cron de refresh precisa continuar renovando tokens JÁ CONCEDIDOS mesmo com o
// kill switch de publicação desligado — senão desligar a feature por uma semana
// venceria silenciosamente a credencial de todo mundo.
func (s *Service) TokenMaintenanceConfigured() bool {
	return s != nil && s.cfg.EncryptionKey != "" && s.credsRepo != nil
}

// DeauthorizeConfigured é mais frouxo que DataDeletionConfigured de propósito:
// revogar não devolve URL de status, então não depende de BACKEND_PUBLIC_URL.
// Exigir a URL aqui faria a Meta receber 503 numa revogação legítima —
// ou seja, o app continuaria com credencial de quem já removeu o acesso.
func (s *Service) DeauthorizeConfigured() bool {
	return s != nil && s.api.AppSecret() != "" && s.credsRepo != nil
}

// DataDeletionConfigured: os callbacks da Meta são um contrato de plataforma e
// precisam responder mesmo com a publicação desligada. Aqui a URL do backend É
// necessária, porque a resposta carrega o link de status.
func (s *Service) DataDeletionConfigured() bool {
	return s != nil && s.api.AppSecret() != "" && s.deletionRepo != nil && isHTTPSURL(s.cfg.BackendBaseURL)
}

// FeatureEnabled combina configuração completa E decisão explícita de ligar.
func (s *Service) FeatureEnabled() bool {
	return s != nil && s.cfg.StoriesEnabled && s.Configured()
}

// --- status ---

// Status is the connection snapshot for the frontend.
type Status struct {
	FeatureEnabled bool
	Connected      bool
	Username       string
	IGUserID       string
	TokenExpiresAt string
	Recent         []models.InstagramStoryPost
	// HasMetaApp: sem app próprio registrado, conectar a conta é impossível —
	// a tela usa isto para guiar o cliente ao passo certo.
	HasMetaApp bool
}

// Status returns feature availability and this user's connection state.
func (s *Service) Status(ctx context.Context, userID string) (*Status, error) {
	out := &Status{FeatureEnabled: s.FeatureEnabled()}
	if !out.FeatureEnabled {
		return out, nil
	}
	out.HasMetaApp = s.HasMetaApp(ctx, userID)
	cred, err := s.credsRepo.Get(ctx, userID)
	if err != nil {
		return nil, err
	}
	// Linha presente NÃO significa conectado: um token vencido é uma conexão
	// morta. Mostrá-la como ativa fazia a UI dizer "conectado" enquanto todo
	// envio falhava.
	if cred != nil && credentialUsable(cred, time.Now()) {
		out.Connected = true
		out.Username = cred.Username
		out.IGUserID = cred.IGUserID
		if cred.TokenExpiresAt != nil {
			out.TokenExpiresAt = cred.TokenExpiresAt.Format(time.RFC3339)
		}
	}
	if recent, err := s.storyRepo.ListRecentByUser(ctx, userID, 10); err == nil {
		out.Recent = recent
	}
	return out, nil
}

// credentialUsable exige expiry PRESENTE e no futuro. Expiry nulo é credencial
// de origem desconhecida — tratá-la como eterna é o erro que se quer evitar.
func credentialUsable(cred *models.InstagramCredentialsDB, now time.Time) bool {
	return cred != nil &&
		cred.TokenExpiresAt != nil &&
		cred.TokenExpiresAt.After(now) &&
		cred.IGUserID != ""
}

// --- OAuth ---

// BuildAuthorizeURL returns the Instagram OAuth URL with a signed, single-use
// state. O state carrega APENAS nonce + expiração: o id do usuário local fica
// no Redis, sob o nonce. Antes o UUID ia dentro do state, ou seja, trafegava
// pela Meta e pelo histórico do navegador de quem conecta.
func (s *Service) BuildAuthorizeURL(ctx context.Context, userID string) (string, error) {
	if !s.FeatureEnabled() {
		return "", ErrInstagramFeatureDisabled
	}
	if _, err := uuid.Parse(userID); err != nil {
		return "", errors.New("instagram: invalid user id")
	}
	nonce, err := randomToken(32)
	if err != nil {
		return "", err
	}
	if err := s.cache.Set(ctx, nonceKey(nonce), userID, stateTTL); err != nil {
		return "", err
	}
	client, err := s.apiClientFor(ctx, userID)
	if err != nil {
		return "", err
	}
	exp := time.Now().Add(stateTTL).Unix()
	return client.AuthorizeURL(s.signState(nonce, exp)), nil
}

// Códigos de redirect do callback. Fixos e sem detalhe técnico: a página de
// destino é pública e o valor cai no histórico do navegador.
const (
	redirectDisabled        = "/instagram?error=disabled"
	redirectInvalidState    = "/instagram?error=invalid_state"
	redirectDenied          = "/instagram?error=denied"
	redirectNoCode          = "/instagram?error=no_code"
	redirectExchangeFailed  = "/instagram?error=exchange_failed"
	redirectProfileFailed   = "/instagram?error=profile_failed"
	redirectAlreadyLinked   = "/instagram?error=already_connected"
	redirectSaveFailed      = "/instagram?error=save_failed"
	redirectConnectedOK     = "/instagram?connected=1"
	instagramCallbackOpName = "oauth_callback"
)

// HandleCallback valida o state, troca o code por um token de longa duração,
// consulta o perfil, cifra e persiste. Devolve o path de redirect do frontend.
//
// A ordem é fixa e o state é consumido PRIMEIRO, inclusive quando a Meta manda
// ?error= — antes esse ramo nem validava state, então o callback de negação era
// um endpoint sem autenticação nenhuma, repetível à vontade.
func (s *Service) HandleCallback(ctx context.Context, code, state, providerError string) (string, error) {
	if !s.FeatureEnabled() {
		return redirectDisabled, ErrInstagramFeatureDisabled
	}
	userID, err := s.consumeState(ctx, state)
	if err != nil {
		s.logger.Warn("instagram callback: invalid state", zap.String("operation", instagramCallbackOpName))
		return redirectInvalidState, err
	}
	if providerError != "" {
		// O valor cru não é logado nem refletido: vem de fora e vira conteúdo
		// de log/redirect. Basta saber que houve negação.
		s.logger.Info("instagram callback: consent denied", zap.String("userID", userID))
		return redirectDenied, errors.New("instagram: consent denied")
	}
	if code == "" {
		return redirectNoCode, errors.New("instagram: empty code")
	}

	// Só aqui sabemos de quem é o callback (o state foi consumido acima), e
	// portanto qual app da Meta usar para trocar o code.
	client, err := s.apiClientFor(ctx, userID)
	if err != nil {
		s.logCallbackErr("resolve_app", userID, err)
		return redirectExchangeFailed, err
	}

	shortToken, appScopedUserID, err := client.ExchangeCode(ctx, code)
	if err != nil {
		s.logCallbackErr("exchange_code", userID, err)
		return redirectExchangeFailed, err
	}
	longToken, expiresIn, err := client.ExchangeLongLived(ctx, shortToken)
	if err != nil {
		s.logCallbackErr("exchange_long_lived", userID, err)
		return redirectExchangeFailed, err
	}
	if expiresIn <= 0 {
		return redirectExchangeFailed, errors.New("instagram: non-positive token expiry")
	}

	// /me é OBRIGATÓRIO: só ele devolve o id PROFISSIONAL. Antes havia fallback
	// para o id da troca (app-scoped) — publicar com ele erra o objeto.
	igUserID, username, _, err := client.GetMe(ctx, longToken)
	if err != nil || igUserID == "" {
		s.logCallbackErr("get_me", userID, err)
		return redirectProfileFailed, errors.New("instagram: profile lookup failed")
	}

	if err := s.saveTokens(ctx, userID, appScopedUserID, igUserID, username, longToken, expiresIn); err != nil {
		if errors.Is(err, repository.ErrInstagramAccountAlreadyConnected) {
			return redirectAlreadyLinked, err
		}
		s.logCallbackErr("save", userID, err)
		return redirectSaveFailed, err
	}
	return redirectConnectedOK, nil
}

// logCallbackErr registra classe do erro, nunca o erro cru (que carrega URL com
// access_token na query).
func (s *Service) logCallbackErr(step, userID string, err error) {
	s.logger.Warn("instagram callback step failed",
		zap.String("operation", instagramCallbackOpName),
		zap.String("step", step),
		zap.String("userID", userID),
		zap.String("class", errorClass(err)))
}

func errorClass(err error) string {
	switch {
	case err == nil:
		return "none"
	case errors.Is(err, ErrInstagramAuth):
		return categoryAuth
	case errors.Is(err, ErrInstagramRateLimit):
		return categoryRateLimit
	case errors.Is(err, ErrInstagramTransient):
		return categoryTransient
	case errors.Is(err, ErrInstagramPermanent):
		return categoryPermanent
	default:
		return "internal"
	}
}

// Disconnect removes the user's stored credentials.
func (s *Service) Disconnect(ctx context.Context, userID string) error {
	if s.credsRepo == nil {
		return nil
	}
	return s.credsRepo.Delete(ctx, userID)
}

// AccessToken devolve o token decriptado e o id profissional do usuário.
//
// Só decripta DEPOIS de conferir dono e validade — decriptar antes gastaria a
// chave num blob que não vai ser usado e abriria caminho para devolver token
// vencido a quem não deveria recebê-lo.
func (s *Service) AccessToken(ctx context.Context, userID string) (string, string, error) {
	if s.credsRepo == nil {
		return "", "", errors.New("instagram: credentials repository not configured")
	}
	cred, err := s.credsRepo.Get(ctx, userID)
	if err != nil {
		return "", "", err
	}
	if cred == nil {
		return "", "", nil // não conectado — o chamador decide o que fazer
	}
	// Isolamento entre contas: a linha veio da query por user_id, mas conferir
	// de novo custa nada e trava qualquer regressão de query no futuro.
	if cred.UserID != userID {
		return "", "", errors.New("instagram: credential owner mismatch")
	}
	if !credentialUsable(cred, time.Now()) {
		return "", "", ErrInstagramTokenExpired
	}

	var blob crypto.EncryptedBlobV1
	if err := json.Unmarshal([]byte(cred.EncryptedData), &blob); err != nil {
		return "", "", err
	}
	var tokens models.InstagramTokens
	if err := crypto.DecryptJSONV1(&blob, s.cfg.EncryptionKey, &tokens); err != nil {
		return "", "", err
	}
	if tokens.AccessToken == "" {
		return "", "", ErrInstagramTokenExpired
	}
	return tokens.AccessToken, cred.IGUserID, nil
}

// HasCredentials reports whether a user has a USABLE Instagram connection
// (cheap check used by the story trigger before enqueuing work). Credencial
// vencida conta como não conectado: enfileirar ali só criaria job condenado.
func (s *Service) HasCredentials(ctx context.Context, userID string) bool {
	if s.credsRepo == nil {
		return false
	}
	cred, err := s.credsRepo.Get(ctx, userID)
	if err != nil || cred == nil {
		return false
	}
	return cred.UserID == userID && credentialUsable(cred, time.Now())
}

// --- publicação (wrappers que resolvem a credencial; nenhum token sai daqui) ---

// GetPublishingLimit devolve quantas publicações a conta já gastou na janela
// móvel de 24h, segundo a própria Meta.
func (s *Service) GetPublishingLimit(ctx context.Context, userID string) (int, error) {
	client, token, igUserID, err := s.resolveCredential(ctx, userID)
	if err != nil {
		return 0, err
	}
	return client.GetPublishingLimit(ctx, igUserID, token)
}

// EnsureStoryContainer devolve o container existente SEM chamar a Meta quando
// já houver um. É o que impede o retry de criar um segundo container (e
// publicar o Story duas vezes) depois de um timeout.
func (s *Service) EnsureStoryContainer(ctx context.Context, userID, publicImageURL, existingContainerID string) (string, error) {
	if existingContainerID != "" {
		return existingContainerID, nil
	}
	client, token, igUserID, err := s.resolveCredential(ctx, userID)
	if err != nil {
		return "", err
	}
	return client.CreateStoryContainer(ctx, igUserID, token, publicImageURL)
}

// GetStoryContainerStatus consulta o status_code do container.
func (s *Service) GetStoryContainerStatus(ctx context.Context, userID, containerID string) (string, error) {
	client, token, _, err := s.resolveCredential(ctx, userID)
	if err != nil {
		return "", err
	}
	return client.GetContainerStatus(ctx, containerID, token)
}

// PublishStoryContainer publica um container pronto e devolve o media id.
func (s *Service) PublishStoryContainer(ctx context.Context, userID, containerID string) (string, error) {
	client, token, igUserID, err := s.resolveCredential(ctx, userID)
	if err != nil {
		return "", err
	}
	return client.PublishContainer(ctx, igUserID, token, containerID)
}

// resolveCredential centraliza "kill switch ligado + credencial válida". O
// pacote workers nunca vê um token: ele passa userID e recebe resultado.
func (s *Service) resolveCredential(ctx context.Context, userID string) (client *APIClient, token, igUserID string, err error) {
	if !s.FeatureEnabled() {
		return nil, "", "", ErrInstagramFeatureDisabled
	}
	token, igUserID, err = s.AccessToken(ctx, userID)
	if err != nil {
		return nil, "", "", err
	}
	if token == "" || igUserID == "" {
		return nil, "", "", ErrInstagramTokenExpired
	}
	client, err = s.apiClientFor(ctx, userID)
	if err != nil {
		return nil, "", "", err
	}
	return client, token, igUserID, nil
}

// --- refresh ---

// RefreshExpiring renova os long-lived tokens que vencem em ~30 dias. Roda com
// TokenMaintenanceConfigured (não com FeatureEnabled): ver o comentário lá.
func (s *Service) RefreshExpiring(ctx context.Context) error {
	if !s.TokenMaintenanceConfigured() {
		return nil
	}
	before := time.Now().Add(30 * 24 * time.Hour)
	creds, err := s.credsRepo.ListExpiring(ctx, before)
	if err != nil {
		return err
	}
	for i := range creds {
		cred := &creds[i]
		// Uma conta com problema não pode interromper a renovação das outras.
		token, _, err := s.AccessToken(ctx, cred.UserID)
		if err != nil || token == "" {
			s.logger.Warn("instagram refresh: cannot read token",
				zap.String("userID", cred.UserID), zap.String("class", errorClass(err)))
			continue
		}
		client, err := s.apiClientFor(ctx, cred.UserID)
		if err != nil {
			s.logger.Warn("instagram refresh: cannot resolve meta app",
				zap.String("userID", cred.UserID), zap.String("class", errorClass(err)))
			continue
		}
		newToken, expiresIn, err := client.RefreshLongLived(ctx, token)
		if err != nil {
			s.logger.Warn("instagram refresh failed",
				zap.String("userID", cred.UserID), zap.String("class", errorClass(err)))
			continue
		}
		if newToken == "" || expiresIn <= 0 {
			s.logger.Warn("instagram refresh: unusable token response", zap.String("userID", cred.UserID))
			continue
		}
		if err := s.saveTokens(ctx, cred.UserID, cred.AppScopedUserID, cred.IGUserID, cred.Username, newToken, expiresIn); err != nil {
			s.logger.Warn("instagram refresh: save failed",
				zap.String("userID", cred.UserID), zap.String("class", errorClass(err)))
		}
	}
	return nil
}

// saveTokens cifra SÓ o token e grava os dois identificadores separados.
func (s *Service) saveTokens(ctx context.Context, userID, appScopedUserID, igUserID, username, token string, expiresIn time.Duration) error {
	if expiresIn <= 0 {
		// Sem fallback inventado: expiry desconhecido é erro, não 60 dias.
		return errors.New("instagram: refusing to store credential without expiry")
	}
	blob, err := crypto.EncryptJSONV1(models.InstagramTokens{AccessToken: token}, s.cfg.EncryptionKey)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(blob)
	if err != nil {
		return err
	}
	return s.credsRepo.Save(ctx, userID, string(raw), appScopedUserID, igUserID, username, time.Now().Add(expiresIn))
}

// --- desautorização e exclusão de dados (callbacks da Meta) ---

// Deauthorize apaga a credencial do sujeito do signed_request. NÃO apaga o
// ledger: remover o app é revogar acesso, não pedir exclusão de histórico —
// para isso existe o callback de data deletion.
func (s *Service) Deauthorize(ctx context.Context, signedRequest string, now time.Time) error {
	if s.credsRepo == nil || s.api.AppSecret() == "" {
		return errors.New("instagram: deauthorize not configured")
	}
	payload, err := ParseSignedRequest(signedRequest, s.api.AppSecret(), now)
	if err != nil {
		return err
	}
	if err := s.credsRepo.DeleteByAppScopedUserID(ctx, payload.UserID); err != nil {
		return err
	}
	s.logger.Info("instagram: account deauthorized")
	return nil
}

// RequestDataDeletion executa a exclusão e devolve o comprovante que a Meta
// exige (URL de status + código de confirmação).
func (s *Service) RequestDataDeletion(ctx context.Context, signedRequest string, now time.Time) (*DataDeletionResponse, error) {
	if !s.DataDeletionConfigured() {
		return nil, errors.New("instagram: data deletion not configured")
	}
	payload, err := ParseSignedRequest(signedRequest, s.api.AppSecret(), now)
	if err != nil {
		return nil, err
	}

	// subject_hash é HMAC do id app-scoped: identifica o sujeito de forma
	// estável (repetição devolve o mesmo comprovante) sem guardar o id bruto.
	subjectHash := s.subjectHash(payload.UserID)
	// O código de confirmação é ALEATÓRIO, não derivado do sujeito — derivá-lo
	// deixaria qualquer um com o id app-scoped consultar o status alheio.
	candidate, err := randomToken(32)
	if err != nil {
		return nil, err
	}

	receipt, err := s.deletionRepo.Process(ctx, payload.UserID, subjectHash, candidate, now.Add(retentionWindow))
	if err != nil {
		return nil, err
	}
	s.logger.Info("instagram: data deletion processed")
	return &DataDeletionResponse{
		URL:              strings.TrimRight(s.cfg.BackendBaseURL, "/") + "/api/v1/instagram/data-deletion/status/" + receipt.ConfirmationCode,
		ConfirmationCode: receipt.ConfirmationCode,
	}, nil
}

// DataDeletionStatus diz se o código corresponde a uma exclusão concluída e
// ainda dentro da validade do comprovante.
func (s *Service) DataDeletionStatus(ctx context.Context, confirmationCode string, now time.Time) (bool, error) {
	if s.deletionRepo == nil {
		return false, nil
	}
	if !isHex64(confirmationCode) {
		return false, nil
	}
	receipt, err := s.deletionRepo.GetByConfirmationCode(ctx, confirmationCode, now)
	if err != nil {
		return false, err
	}
	return receipt != nil, nil
}

func (s *Service) subjectHash(appScopedUserID string) string {
	mac := hmac.New(sha256.New, []byte(s.api.AppSecret()))
	mac.Write([]byte("instagram-data-deletion:" + appScopedUserID))
	return hex.EncodeToString(mac.Sum(nil))
}

// --- retenção ---

// CleanupExpired apaga ledger antigo e comprovantes vencidos, em lotes
// limitados. Guardar identificadores de grupo/mensagem para sempre é retenção
// sem finalidade — e custo.
func (s *Service) CleanupExpired(ctx context.Context, now time.Time) error {
	cutoff := now.Add(-retentionWindow)
	var firstErr error

	if s.storyRepo != nil {
		total := int64(0)
		for i := 0; i < retentionMaxBatches; i++ {
			n, err := s.storyRepo.DeleteOlderThan(ctx, cutoff, retentionBatch)
			if err != nil {
				firstErr = err
				break
			}
			total += n
			if n < retentionBatch {
				break
			}
		}
		if total > 0 {
			s.logger.Info("instagram retention: story ledger pruned", zap.Int64("rows", total))
		}
	}

	if s.deletionRepo != nil {
		total := int64(0)
		for i := 0; i < retentionMaxBatches; i++ {
			n, err := s.deletionRepo.DeleteExpired(ctx, now, retentionBatch)
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				break
			}
			total += n
			if n < retentionBatch {
				break
			}
		}
		if total > 0 {
			s.logger.Info("instagram retention: deletion receipts pruned", zap.Int64("rows", total))
		}
	}
	return firstErr
}

// --- state assinado e de uso único ---

// signState monta base64url(nonce|exp).hex(hmac). Sem id de usuário dentro.
func (s *Service) signState(nonce string, exp int64) string {
	payload := fmt.Sprintf("%s|%d", nonce, exp)
	enc := base64.RawURLEncoding.EncodeToString([]byte(payload))
	return enc + "." + s.hmac(enc)
}

// consumeState valida assinatura/prazo e CONSOME o nonce atomicamente.
//
// O GETDEL é o ponto central: com Get-depois-Delete, dois callbacks concorrentes
// liam o mesmo nonce e ambos passavam — replay do callback, duas trocas de code
// e duas persistências correndo entre si.
func (s *Service) consumeState(ctx context.Context, state string) (string, error) {
	if !s.redisReady() {
		return "", errors.New("instagram: state store unavailable")
	}
	enc, sig, found := strings.Cut(state, ".")
	if !found || enc == "" || sig == "" {
		return "", errors.New("instagram: malformed state")
	}
	if !hmac.Equal([]byte(sig), []byte(s.hmac(enc))) {
		return "", errors.New("instagram: bad state signature")
	}
	raw, err := base64.RawURLEncoding.DecodeString(enc)
	if err != nil {
		return "", errors.New("instagram: malformed state payload")
	}
	nonce, expStr, found := strings.Cut(string(raw), "|")
	if !found || nonce == "" {
		return "", errors.New("instagram: malformed state payload")
	}
	exp, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil {
		return "", errors.New("instagram: malformed state expiry")
	}
	now := time.Now()
	if now.Unix() > exp {
		return "", errors.New("instagram: state expired")
	}
	// Teto futuro: um state assinado com validade de anos seria eterno se a
	// chave vazasse. O nosso nunca passa de stateTTL.
	if time.Unix(exp, 0).After(now.Add(stateTTL)) {
		return "", errors.New("instagram: state expiry too far in the future")
	}

	userID, err := s.cache.GetDel(ctx, nonceKey(nonce))
	if err != nil || userID == "" {
		return "", errors.New("instagram: state nonce already used or unknown")
	}
	if _, err := uuid.Parse(userID); err != nil {
		return "", errors.New("instagram: state nonce maps to invalid user")
	}
	return userID, nil
}

func (s *Service) hmac(msg string) string {
	h := hmac.New(sha256.New, []byte(s.cfg.StateSigningKey))
	h.Write([]byte(msg))
	return hex.EncodeToString(h.Sum(nil))
}

func nonceKey(nonce string) string { return "ig_oauth_nonce:" + nonce }

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// isHex64 valida o formato do código de confirmação ANTES de qualquer consulta,
// para um id malformado não virar tráfego de banco.
func isHex64(s string) bool {
	if len(s) != 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// --- app da Meta do cliente (CRUD) ---

// MetaAppView é o que a UI pode ver do app do cliente. O segredo NÃO está aqui
// — depois de salvo, o frontend só aprende que existe um.
type MetaAppView struct {
	AppID       string
	HasSecret   bool
	WebhookURL  string
	VerifyToken string
	RedirectURI string
	IsVerified  bool
	UpdatedAt   string
}

// ErrInstagramInvalidAppInput sinaliza entrada malformada vinda da tela.
var ErrInstagramInvalidAppInput = errors.New("instagram: invalid meta app input")

// maxAppFieldLen evita que um campo colado errado (uma página inteira, por
// exemplo) vire uma linha gigante no banco.
const maxAppFieldLen = 512

// GetMetaApp devolve o app registrado pelo usuário, ou nil se ainda não houver.
func (s *Service) GetMetaApp(ctx context.Context, userID string) (*MetaAppView, error) {
	if s.appsRepo == nil {
		return nil, nil
	}
	app, err := s.appsRepo.Get(ctx, userID)
	if err != nil || app == nil {
		return nil, err
	}
	return s.viewOf(app), nil
}

// SaveMetaApp grava as credenciais do app da Meta do cliente.
//
// appSecret vazio numa atualização significa "mantém o que está guardado" — a
// tela nunca recebe o segredo de volta, então exigir que ele fosse reenviado a
// cada edição obrigaria o cliente a buscá-lo na Meta só para trocar o app id.
func (s *Service) SaveMetaApp(ctx context.Context, userID, appID, appSecret string) (*MetaAppView, error) {
	if s.appsRepo == nil {
		return nil, errors.New("instagram: app repository not configured")
	}
	if s.cfg.EncryptionKey == "" {
		return nil, errors.New("instagram: encryption key not configured")
	}
	appID = strings.TrimSpace(appID)
	appSecret = strings.TrimSpace(appSecret)
	if appID == "" || len(appID) > maxAppFieldLen || len(appSecret) > maxAppFieldLen {
		return nil, ErrInstagramInvalidAppInput
	}

	existing, err := s.appsRepo.Get(ctx, userID)
	if err != nil {
		return nil, err
	}
	encrypted := ""
	switch {
	case appSecret != "":
		blob, err := crypto.EncryptJSONV1(models.InstagramAppSecret{AppSecret: appSecret}, s.cfg.EncryptionKey)
		if err != nil {
			return nil, err
		}
		raw, err := json.Marshal(blob)
		if err != nil {
			return nil, err
		}
		encrypted = string(raw)
	case existing != nil:
		encrypted = existing.EncryptedAppSecret // mantém o segredo já guardado
	default:
		// Primeiro cadastro sem segredo não serve para nada: o OAuth falharia
		// depois, longe daqui, com um erro difícil de ligar à causa.
		return nil, ErrInstagramInvalidAppInput
	}

	app, err := s.appsRepo.Upsert(ctx, userID, appID, encrypted)
	if err != nil {
		return nil, err
	}
	// Nem app id entra no log: identifica o negócio do cliente na Meta.
	s.logger.Info("instagram: meta app saved", zap.String("userID", userID))
	return s.viewOf(app), nil
}

// DeleteMetaApp remove o registro do app. As credenciais da conta conectada
// continuam — desconectar a conta é outra ação, deliberadamente separada.
func (s *Service) DeleteMetaApp(ctx context.Context, userID string) error {
	if s.appsRepo == nil {
		return nil
	}
	return s.appsRepo.Delete(ctx, userID)
}

// HasMetaApp informa se o usuário já registrou o app dele.
func (s *Service) HasMetaApp(ctx context.Context, userID string) bool {
	if s.appsRepo == nil {
		return false
	}
	app, err := s.appsRepo.Get(ctx, userID)
	return err == nil && app != nil
}

func (s *Service) viewOf(app *models.InstagramApp) *MetaAppView {
	return &MetaAppView{
		AppID:     app.AppID,
		HasSecret: app.EncryptedAppSecret != "",
		// A URL que o cliente cola no painel do próprio app. O token opaco no
		// path é o que permite validar a assinatura de um webhook multi-tenant.
		WebhookURL:  strings.TrimRight(s.cfg.BackendBaseURL, "/") + "/api/v1/instagram/webhook/" + app.WebhookToken,
		VerifyToken: app.VerifyToken,
		RedirectURI: s.redirectURI(),
		IsVerified:  app.IsVerified,
		UpdatedAt:   app.UpdatedAt,
	}
}

// --- webhook de entrada (Fase 2) ---

// ErrWebhookNotConfigured: token da URL não corresponde a nenhum cliente.
var ErrWebhookNotConfigured = errors.New("instagram: unknown webhook token")

// VerifyWebhookChallenge responde ao handshake GET da Meta.
//
// A Meta chama a URL com hub.mode=subscribe, hub.verify_token e hub.challenge;
// só devolvemos o challenge se o verify_token bater com o DAQUELE cliente. O
// token vive por cliente justamente para um não conseguir validar o webhook do
// outro.
func (s *Service) VerifyWebhookChallenge(ctx context.Context, webhookToken, mode, verifyToken, challenge string) (string, error) {
	if s.appsRepo == nil {
		return "", ErrWebhookNotConfigured
	}
	app, err := s.appsRepo.GetByWebhookToken(ctx, webhookToken)
	if err != nil {
		return "", err
	}
	if app == nil {
		return "", ErrWebhookNotConfigured
	}
	if mode != "subscribe" {
		return "", ErrWebhookNotConfigured
	}
	// Tempo constante: o verify_token é um segredo compartilhado com a Meta.
	if subtle.ConstantTimeCompare([]byte(verifyToken), []byte(app.VerifyToken)) != 1 {
		return "", ErrWebhookNotConfigured
	}
	// O handshake bem-sucedido é a prova de que o cliente configurou a URL certa.
	if err := s.appsRepo.MarkVerified(ctx, app.UserID); err != nil {
		s.logger.Warn("instagram webhook: mark verified failed", zap.String("userID", app.UserID))
	}
	s.logger.Info("instagram webhook verified", zap.String("userID", app.UserID))
	return challenge, nil
}

// HandleWebhook valida a assinatura e grava os eventos.
//
// Devolve quantos eventos NOVOS entraram (reenvio da Meta não conta). O
// chamador responde 200 de qualquer forma depois de validar a assinatura: a
// Meta trata não-200 como falha e reenvia, e reenviar um evento que já
// gravamos só gasta tráfego.
func (s *Service) HandleWebhook(ctx context.Context, webhookToken string, rawBody []byte, signature string) (int, error) {
	if s.appsRepo == nil || s.inboundRepo == nil {
		return 0, ErrWebhookNotConfigured
	}
	client, app, err := s.ClientByWebhookToken(ctx, webhookToken)
	if err != nil {
		return 0, ErrWebhookNotConfigured
	}
	// Assinatura ANTES de decodificar: o corpo é conteúdo não confiável até aqui.
	if err := VerifyWebhookSignature(rawBody, signature, client.AppSecret()); err != nil {
		return 0, err
	}

	// Conta própria do cliente, para o parser descartar eco/auto-comentário.
	own := map[string]bool{}
	if cred, err := s.credsRepo.Get(ctx, app.UserID); err == nil && cred != nil && cred.IGUserID != "" {
		own[cred.IGUserID] = true
	}

	events, err := ParseWebhook(rawBody, own)
	if err != nil {
		// Payload que não decodifica não vira erro para a Meta: ela reenviaria
		// para sempre um corpo que nunca vai ser aceito.
		s.logger.Warn("instagram webhook: undecodable payload", zap.String("userID", app.UserID))
		return 0, nil
	}

	created := 0
	for i := range events {
		e := &events[i]
		row, isNew, err := s.inboundRepo.Create(ctx, &models.InstagramInboundEvent{
			UserID:          app.UserID,
			ProviderEventID: e.ProviderEventID,
			EventType:       e.EventType,
			IGAccountID:     e.IGAccountID,
			SenderID:        e.SenderID,
			ThreadID:        e.ThreadID,
			Content:         e.Content,
			OccurredAt:      e.OccurredAt,
		})
		if err != nil {
			// Uma linha problemática não pode derrubar as outras do mesmo lote.
			s.logger.Warn("instagram webhook: persist event failed",
				zap.String("userID", app.UserID), zap.String("type", e.EventType), zap.Error(err))
			continue
		}
		if !isNew {
			continue
		}
		created++
		// Enfileira a resposta apenas para evento NOVO. Reenvio da Meta não
		// pode gerar um segundo job — e o TaskID determinístico é a segunda
		// barreira, caso gere.
		if s.enqueueReply != nil {
			if err := s.enqueueReply(ctx, row); err != nil {
				// Falha ao enfileirar não invalida a ingestão: o evento está
				// gravado e visível no histórico. Perder a resposta é ruim;
				// perder o registro seria pior.
				s.logger.Warn("instagram webhook: enqueue reply failed",
					zap.String("userID", app.UserID), zap.String("type", e.EventType))
			}
		}
	}
	if created > 0 {
		// Só contagens: o conteúdo é texto de terceiro.
		s.logger.Info("instagram webhook ingested",
			zap.String("userID", app.UserID), zap.Int("novos", created), zap.Int("recebidos", len(events)))
	}
	return created, nil
}

// ListInboundEvents devolve o histórico do usuário para a aba Histórico.
func (s *Service) ListInboundEvents(ctx context.Context, userID string, limit int) ([]models.InstagramInboundEvent, error) {
	if s.inboundRepo == nil {
		return nil, nil
	}
	return s.inboundRepo.ListRecentByUser(ctx, userID, limit)
}
