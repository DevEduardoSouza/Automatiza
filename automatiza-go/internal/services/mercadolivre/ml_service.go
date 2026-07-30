package mercadolivre

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"automatiza-go/internal/lib/cache"
	"automatiza-go/internal/lib/commerceurl"
	"automatiza-go/internal/lib/crypto"
	"automatiza-go/internal/lib/text"
	"automatiza-go/internal/models"
	"automatiza-go/internal/observability"
	"automatiza-go/internal/repository"

	"github.com/getsentry/sentry-go"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.uber.org/zap"
)

// Service defines the Mercado Livre service interface
type Service interface {
	CreateAffiliateLink(ctx context.Context, originURL, userID string) (*models.MLAffiliateLink, error)
	GetProductPreview(ctx context.Context, originURL, userID string) (*models.MLProductPreview, error)
	LoadCredentials(ctx context.Context, userID string) (*models.MLCredentials, error)
	SaveCredentials(ctx context.Context, userID, cookie, csrf, tag string) error
	IsConnected(ctx context.Context, userID string) (bool, error)
	ExtractProductID(productURL string) (string, error)
	InvalidatePreviewCache(ctx context.Context, originURL string) error
}

type mlService struct {
	httpClient *http.Client
	// safe é o par de clientes endurecidos usado em TODA busca de página HTML
	// (preview de produto e página /social de afiliado). httpClient continua
	// servindo só a API GraphQL de afiliado, cujo endpoint é fixo e
	// configurado — não vem do texto de um grupo.
	safe          *safeMLClients
	cache         *cache.Cache
	mlCredsRepo   repository.MLCredentialsRepository
	mlLinkRepo    repository.MLLinkRepository
	encryptionKey string
	logger        *zap.Logger
}

// NewMLService creates a new Mercado Livre service
func NewMLService(
	cache *cache.Cache,
	mlCredsRepo repository.MLCredentialsRepository,
	mlLinkRepo repository.MLLinkRepository,
	encryptionKey string,
	logger *zap.Logger,
) Service {
	return &mlService{
		httpClient: &http.Client{
			Timeout:   30 * time.Second,
			Transport: otelhttp.NewTransport(http.DefaultTransport),
		},
		safe:          newSafeMLClients(commerceurl.ClientOptions{}),
		cache:         cache,
		mlCredsRepo:   mlCredsRepo,
		mlLinkRepo:    mlLinkRepo,
		encryptionKey: encryptionKey,
		logger:        logger,
	}
}

// Cache TTLs for ML (matching Node.js for parity)
const (
	previewCacheTTLUseful = 10 * time.Minute // Cache preview with useful data for 10 min
	previewCacheTTLEmpty  = 30 * time.Second // Cache empty preview for 30 sec
	linkCacheTTL          = 6 * time.Hour    // Cache affiliate links for 6 hours
)

func (s *mlService) CreateAffiliateLink(ctx context.Context, originURL, userID string) (*models.MLAffiliateLink, error) {
	// Short links de afiliado (meli.la/XXX, .../sec/XXX) não carregam o MLB id
	// na URL. A versão anterior devolvia a URL RECEBIDA como se fosse o link
	// afiliado do usuário — ou seja, repassava a comissão de um terceiro
	// assinada como nossa. Agora resolvemos o short anonimamente até o produto
	// e geramos o link DO DONO; se não der para resolver, devolvemos erro
	// tipado e quem chamou decide (nunca emitir o original).
	if isAffiliateURL(originURL) {
		resolved, err := s.resolveShortLinkToProductURL(ctx, originURL)
		if err != nil {
			return nil, err
		}
		originURL = resolved
	}

	// Extract product ID
	productID, err := s.ExtractProductID(originURL)
	if err != nil {
		return nil, fmt.Errorf("invalid mercadolivre URL: %w", err)
	}

	// Check cache
	cacheKey := fmt.Sprintf("ml_link:%s:%s", userID, productID)
	var cached models.MLAffiliateLink
	if s.cache != nil {
		if err := s.cache.GetJSON(ctx, cacheKey, &cached); err == nil {
			return &cached, nil
		}
	}

	// Check if we already have this link in database
	existing, dbCheckErr := s.mlLinkRepo.GetByOriginalURL(ctx, userID, originURL)
	if dbCheckErr != nil {
		observability.LogAndCapture(s.logger, sentry.LevelWarning, "Failed to check existing ML affiliate link in database", dbCheckErr, map[string]string{
			"external_service": "mercadolivre",
			"endpoint":         "getByOriginalURL",
		})
	}
	if existing != nil {
		return &models.MLAffiliateLink{
			OriginalURL:   existing.OriginalURL,
			AffiliateLink: existing.ShortURL,
			ProductID:     existing.ProductID,
		}, nil
	}

	// Load credentials
	creds, err := s.LoadCredentials(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("ML credentials not configured: %w", err)
	}

	// Create short link via ML API (with CSRF refresh on 401/403)
	affiliateLink, err := s.createShortLink(ctx, originURL, creds, userID)
	if err != nil {
		return nil, err
	}

	result := &models.MLAffiliateLink{
		OriginalURL:   originURL,
		AffiliateLink: affiliateLink,
		ProductID:     productID,
	}

	// Save to database (best effort - don't fail if DB write fails)
	if s.mlLinkRepo != nil {
		if dbErr := s.mlLinkRepo.Create(ctx, &models.MLLinkEntry{
			UserID:      userID,
			OriginalURL: originURL,
			ShortURL:    affiliateLink,
			ProductID:   productID,
			CreatedAt:   time.Now().UTC().Format(time.RFC3339),
		}); dbErr != nil {
			observability.LogAndCapture(s.logger, sentry.LevelWarning, "Failed to save ML affiliate link to database", dbErr, map[string]string{
				"external_service": "mercadolivre",
				"endpoint":         "db_save_link",
			})
		}
	}

	// Cache for 6 hours (matching Node.js for parity)
	if s.cache != nil {
		if cacheErr := s.cache.SetJSON(ctx, cacheKey, result, linkCacheTTL); cacheErr != nil {
			observability.LogAndCapture(s.logger, sentry.LevelWarning, "Failed to cache ML affiliate link", cacheErr, map[string]string{
				"external_service": "mercadolivre",
				"endpoint":         "cache_set_link",
			})
		}
	}

	return result, nil
}

// createShortLink creates an affiliate short link via ML API
// Implements auto-refresh of CSRF token on 401/403 errors for parity with Node.js
func (s *mlService) createShortLink(ctx context.Context, originURL string, creds *models.MLCredentials, userID string) (string, error) {
	// ML affiliate link API (official endpoint)
	const createLinkEndpoint = "https://www.mercadolivre.com.br/affiliate-program/api/v2/affiliates/createLink"
	const referer = "https://www.mercadolivre.com.br/afiliados/linkbuilder"

	csrf := creds.CSRF

	// If no CSRF, try to refresh it first
	if csrf == "" {
		refresher := NewCSRFRefresher(s.httpClient)
		newCSRF, err := refresher.RefreshCSRFToken(ctx, creds.Cookie)
		if err != nil {
			s.logger.Warn("Failed to get initial CSRF token", zap.Error(err))
		} else {
			csrf = newCSRF
			// Update stored credentials with new CSRF (best-effort)
			csrfCopy := csrf
			observability.SafeGo("ml.saveCredentials.initialCSRF", func() {
				if saveErr := s.SaveCredentials(context.Background(), userID, creds.Cookie, csrfCopy, creds.Tag); saveErr != nil {
					observability.LogAndCapture(s.logger, sentry.LevelWarning, "Failed to save initial CSRF credentials", saveErr, map[string]string{
						"external_service": "mercadolivre",
						"endpoint":         "saveCredentials",
					})
				}
			})
		}
	}

	if csrf == "" {
		return "", fmt.Errorf("CSRF token is required; could not obtain automatically")
	}

	doRequest := func(csrfToken string) (*http.Response, []byte, error) {
		payload := map[string]interface{}{
			"urls": []string{originURL},
			"tag":  creds.Tag,
		}

		jsonPayload, marshalErr := json.Marshal(payload)
		if marshalErr != nil {
			observability.LogAndCapture(s.logger, sentry.LevelError, "Failed to marshal ML createLink payload", marshalErr, map[string]string{
				"external_service": "mercadolivre",
				"endpoint":         "createLink",
			})
			return nil, nil, fmt.Errorf("failed to marshal createLink payload: %w", marshalErr)
		}

		req, err := http.NewRequestWithContext(ctx, "POST", createLinkEndpoint, bytes.NewReader(jsonPayload))
		if err != nil {
			return nil, nil, err
		}

		req.Header.Set("Accept", "application/json, text/plain, */*")
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Cookie", creds.Cookie)
		req.Header.Set("X-CSRF-Token", csrfToken)
		req.Header.Set("Origin", "https://www.mercadolivre.com.br")
		req.Header.Set("Referer", referer)
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120 Safari/537.36")

		span := sentry.StartSpan(ctx, "http.client", sentry.WithDescription("POST "+createLinkEndpoint))
		span.SetTag("external_service", "mercadolivre")
		span.SetTag("endpoint", "createLink")
		resp, err := s.httpClient.Do(req)
		span.Finish()
		if err != nil {
			observability.LogAndCapture(s.logger, sentry.LevelError, "ML createLink HTTP request failed", err, map[string]string{
				"external_service": "mercadolivre",
				"endpoint":         "createLink",
			})
			return nil, nil, fmt.Errorf("failed to create short link: %w", err)
		}
		defer resp.Body.Close()

		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB limit
		if readErr != nil {
			observability.LogAndCapture(s.logger, sentry.LevelError, "Failed to read ML createLink response body", readErr, map[string]string{
				"external_service": "mercadolivre",
				"endpoint":         "createLink",
			})
			return nil, nil, fmt.Errorf("failed to read createLink response: %w", readErr)
		}
		return resp, body, nil
	}

	// First attempt
	resp, body, err := doRequest(csrf)
	if err != nil {
		return "", err
	}

	// Auto-refresh CSRF on 401/403 and retry
	if IsCSRFError(resp.StatusCode) {
		s.logger.Info("CSRF error, attempting refresh", zap.Int("status", resp.StatusCode))

		refresher := NewCSRFRefresher(s.httpClient)
		newCSRF, refreshErr := refresher.RefreshCSRFToken(ctx, creds.Cookie)
		if refreshErr != nil {
			s.logger.Warn("Failed to refresh CSRF token", zap.Error(refreshErr))
			return "", fmt.Errorf("ML session expired (401/403) and CSRF refresh failed: %w", refreshErr)
		}

		csrf = newCSRF
		s.logger.Info("CSRF refreshed successfully, retrying request")

		// Update stored credentials with new CSRF (best-effort, async)
		csrfRetry := csrf
		observability.SafeGo("ml.saveCredentials.csrfRefresh", func() {
			if saveErr := s.SaveCredentials(context.Background(), userID, creds.Cookie, csrfRetry, creds.Tag); saveErr != nil {
				observability.LogAndCapture(s.logger, sentry.LevelWarning, "Failed to save refreshed CSRF credentials", saveErr, map[string]string{
					"external_service": "mercadolivre",
					"endpoint":         "saveCredentials",
				})
			}
		})

		// Retry with new CSRF
		resp, body, err = doRequest(csrf)
		if err != nil {
			return "", err
		}
	}

	if resp.StatusCode != http.StatusOK {
		s.logger.Warn("ML createLink non-200",
			zap.Int("status", resp.StatusCode),
			zap.String("body", string(body[:min(len(body), 300)])))
		bodySnippet := string(body[:min(len(body), 500)])
		httpErr := fmt.Errorf("ML API error %d: %s", resp.StatusCode, bodySnippet)
		observability.LogAndCapture(s.logger, sentry.LevelError, "ML createLink returned non-200 status", httpErr, map[string]string{
			"external_service": "mercadolivre",
			"endpoint":         "createLink",
			"http_status":      fmt.Sprintf("%d", resp.StatusCode),
		})
		return "", httpErr
	}

	// Parse ML API response: { status: 200, urls: [{ original_url, long_url, short_url }] }
	var result struct {
		Status int `json:"status"`
		URLs   []struct {
			OriginalURL string `json:"original_url"`
			LongURL     string `json:"long_url"`
			ShortURL    string `json:"short_url"`
		} `json:"urls"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		observability.LogAndCapture(s.logger, sentry.LevelError, "Failed to unmarshal ML createLink response", err, map[string]string{
			"external_service": "mercadolivre",
			"endpoint":         "createLink",
		})
		return "", fmt.Errorf("failed to parse ML response: %w", err)
	}

	if result.Status != 200 || len(result.URLs) == 0 {
		return "", fmt.Errorf("unexpected ML response: status=%d, urls=%d", result.Status, len(result.URLs))
	}

	// Prefer short_url, fallback to long_url
	if result.URLs[0].ShortURL != "" {
		return result.URLs[0].ShortURL, nil
	}
	if result.URLs[0].LongURL != "" {
		return result.URLs[0].LongURL, nil
	}

	return "", fmt.Errorf("no affiliate link in response")
}

func (s *mlService) GetProductPreview(ctx context.Context, originURL, userID string) (*models.MLProductPreview, error) {
	// Normalize URL (remove hash, trim)
	cleanURL := normalizeURL(originURL)

	// Extract product ID
	productID, err := s.ExtractProductID(cleanURL)
	if err != nil {
		// Links de afiliado (meli.la/XXX, mercadolivre.com/sec/XXX) não têm o
		// MLB id na URL. Esses links redirecionam pra página /social do afiliado, que
		// é pública (sem o anti-bot das páginas de produto) e traz os dados do
		// produto-alvo no JSON embutido. Ver issue #158.
		if isAffiliateURL(cleanURL) {
			return s.getAffiliateProductPreview(ctx, cleanURL, userID)
		}
		return nil, fmt.Errorf("invalid mercadolivre URL: %w", err)
	}

	// Build cache key with version (for cache invalidation when extraction changes)
	// Format: ml:preview:v3:{sha1(userId|cleanUrl)} - matching Node.js
	cacheKey := fmt.Sprintf("ml:preview:v3:%s:%s", userID, productID)
	var cached models.MLProductPreview
	if s.cache != nil {
		if err := s.cache.GetJSON(ctx, cacheKey, &cached); err == nil {
			return &cached, nil
		}
	}

	// Load credentials for authenticated request (optional - preview works without auth)
	creds, credsErr := s.LoadCredentials(ctx, userID)
	if credsErr != nil {
		s.logger.Debug("ML credentials not available for preview (optional)", zap.Error(credsErr))
	}

	// Busca a página de produto pelo cliente ENDURECIDO.
	//
	// Duas mudanças de contrato aqui:
	//  1. cleanURL é revalidada — só um host ML real recebe request, e o cookie
	//     do usuário só sai para os hosts exatos da allowlist;
	//  2. não-200 deixou de virar "preview vazio com erro nil". Aquele colapso
	//     era a origem do falso aviso de cupom: um 429 ou um HTML mudado viravam
	//     HasProductData=false, que o chamador lia como "então é campanha".
	//     Agora sai RemoteError tipado e quem chama decide retry × bloqueio.
	safeURL, urlErr := ResolveSafeMLURL(cleanURL)
	if urlErr != nil {
		return nil, urlErr
	}

	span := sentry.StartSpan(ctx, "http.client", sentry.WithDescription("GET mercadolivre product page"))
	span.SetTag("external_service", "mercadolivre")
	span.SetTag("endpoint", "productPreview")
	body, finalURL, fetchErr := s.safe.fetchHTML(ctx, safeURL, creds)
	span.Finish()
	if fetchErr != nil {
		observability.LogAndCapture(s.logger, sentry.LevelError, "Failed to fetch ML product preview", fetchErr, map[string]string{
			"external_service": "mercadolivre",
			"endpoint":         "productPreview",
			"host":             safeURL.Host,
			"url_hash":         safeURL.Hash8(),
		})
		return nil, fetchErr
	}
	cleanURL = finalURL.String()
	htmlContent := string(body)

	// Parse HTML for product info using full extraction logic
	preview := s.parseProductFromHTML(htmlContent, cleanURL, productID)

	// Cache with appropriate TTL based on whether we got useful data
	if s.cache != nil && preview != nil {
		hasUsefulData := preview.Title != "" || preview.Image != "" || preview.Price > 0
		ttl := previewCacheTTLEmpty
		if hasUsefulData {
			ttl = previewCacheTTLUseful
		}
		if cacheErr := s.cache.SetJSON(ctx, cacheKey, preview, ttl); cacheErr != nil {
			observability.LogAndCapture(s.logger, sentry.LevelWarning, "Failed to cache ML product preview", cacheErr, map[string]string{
				"external_service": "mercadolivre",
				"endpoint":         "cache_set_preview",
			})
		}
	}

	return preview, nil
}

// browserUserAgent é o UA usado nas requisições HTML (preview/afiliado) pra
// não ser tratado como bot óbvio.
const browserUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120 Safari/537.36"

// Extração de preço do 1º objeto "price" da página /social (produto-alvo = 1º
// card). O objeto tem previous_price (original), current_price (ATUAL), e também
// discount ({"value":31} = % de desconto) e installments ({"value":31.83} =
// parcela) — por isso miramos current_price explicitamente; pegar um "value"
// genérico traria o % de desconto ou a parcela (bug visto em prod: R$31 no
// lugar de R$190).
var (
	// current_price.value = preço ATUAL (formato atual do ML).
	affiliateCurrentPriceRe = regexp.MustCompile(`"current_price":\{"value":([0-9.]+)`)
	// previous_price.value = preço ORIGINAL (quando há desconto).
	affiliatePrevPriceRe = regexp.MustCompile(`"previous_price":\{"value":([0-9.]+)`)
	// Fallback p/ produto sem desconto: "price":{"value":N} direto.
	affiliateBareValueRe = regexp.MustCompile(`"price":\{"value":([0-9.]+)`)
)

// affiliatePriceWindow é o teto de bytes lidos a partir da âncora do card. Os
// 800 originais deixaram de bastar em 30/07/2026, quando o ML passou a
// intercalar promoções/campanhas antes do componente de preço (medição em
// produção: current_price ~1.1KB depois do bloco novo).
const affiliatePriceWindow = 2000

// isAffiliateURL detecta links de afiliado ML (meli.la/XXX, .../sec/XXX), que não
// têm o MLB id na URL e precisam ser resolvidos pela página /social.
//
// Delega ao commerceurl: o strings.Contains anterior casava qualquer URL cujo
// PATH contivesse "meli.la/", então https://evil.test/meli.la/x era tratado
// como link de afiliado ML e ia direto para um GET — SSRF com um host
// arbitrário. Aqui o host é comparado exato sobre a URL já parseada.
func isAffiliateURL(rawURL string) bool {
	u, err := commerceurl.Parse(rawURL)
	if err != nil || u.Store != models.MonitorStoreMercadoLivr {
		return false
	}
	return commerceurl.IsMLShortener(u)
}

// affiliateShortCode devolve o último segmento do path do link de afiliado
// (ex.: meli.la/2Uh8z3c -> "2Uh8z3c"), usado como chave de cache.
func affiliateShortCode(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	p := strings.Trim(u.Path, "/")
	if i := strings.LastIndex(p, "/"); i >= 0 {
		p = p[i+1:]
	}
	if p == "" {
		return rawURL
	}
	return p
}

// getAffiliateProductPreview resolve um link de afiliado seguindo o redirect até
// a página /social do afiliado (pública, sem o anti-bot das páginas de produto)
// e extrai o produto-alvo do JSON embutido. Ver issue #158 e a memória do projeto.
func (s *mlService) getAffiliateProductPreview(ctx context.Context, affiliateURL, userID string) (*models.MLProductPreview, error) {
	// shortCode é calculado ANTES do fetch: mais abaixo affiliateURL é
	// reatribuído para a URL final resolvida (.../social/<apelido-do-afiliado>),
	// e daí o "short code" viraria o apelido — inútil para reproduzir a página.
	shortCode := affiliateShortCode(affiliateURL)

	// v2 (30/07/2026): o bump invalida em massa os previews com Price=0 que a
	// mudança de layout do ML gerou. Sem ele, o deploy do parser novo passa os
	// primeiros 10min servindo zero do Redis e parece não ter funcionado.
	cacheKey := "ml:preview:aff:v2:" + userID + ":" + shortCode
	if s.cache != nil {
		var cached models.MLProductPreview
		if err := s.cache.GetJSON(ctx, cacheKey, &cached); err == nil {
			return &cached, nil
		}
	}

	// Página /social é pública: buscada SEMPRE anonimamente (creds=nil). O
	// destino de um short link é escolhido por quem postou a mensagem — mandar
	// o cookie do nosso usuário para lá seria entregar credencial a terceiro.
	safeURL, urlErr := ResolveSafeMLURL(affiliateURL)
	if urlErr != nil {
		return nil, urlErr
	}

	span := sentry.StartSpan(ctx, "http.client", sentry.WithDescription("GET mercadolivre social page"))
	span.SetTag("external_service", "mercadolivre")
	span.SetTag("endpoint", "affiliatePreview")
	body, finalURL, fetchErr := s.safe.fetchHTML(ctx, safeURL, nil)
	span.Finish()
	if fetchErr != nil {
		// Não-200 e timeout deixaram de virar "preview vazio, erro nil": o
		// chamador precisa distinguir transient de permanente, senão ausência
		// de dados vira campanha.
		return nil, fetchErr
	}
	affiliateURL = finalURL.String()

	preview := parseAffiliateSocialHTML(string(body), affiliateURL)
	if preview == nil {
		preview = &models.MLProductPreview{Permalink: affiliateURL, Currency: "BRL"}
	}

	// Título sem preço é o estado em que a rampa de 20→1210 skips/dia (27→30/07)
	// passou 3 dias invisível: nenhum erro era gerado. Agora deixa rastro.
	//
	// Fica em log, NÃO no Sentry: link de cupom/campanha ML (que não é produto)
	// vive legitimamente nesse estado — ver models/mercadolivre.go:62-67 e o
	// caminho normal do espelho em monitoring_service.go:1649. Como o
	// fingerprint do Sentry é a mensagem (constante) e o dedup é de 10min, este
	// aviso sozinho consumiria os 6 eventos/h do orçamento do processo e
	// afogaria erro real.
	if preview.Title != "" && preview.Price <= 0 {
		s.logger.Warn("ML /social sem preço (cupom/campanha, anti-bot ou formato mudou)",
			zap.String("short_code", shortCode))
	}

	if s.cache != nil {
		ttl := previewCacheTTLEmpty
		// Só Price>0 conta como útil (10min). Título sem preço é o estado
		// quebrado acima — cachear por 10min congelaria a falha; com 30s a
		// próxima janela re-tenta.
		if preview.Price > 0 {
			ttl = previewCacheTTLUseful
		}
		if cacheErr := s.cache.SetJSON(ctx, cacheKey, preview, ttl); cacheErr != nil {
			observability.LogAndCapture(s.logger, sentry.LevelWarning, "Failed to cache ML affiliate preview", cacheErr, map[string]string{
				"external_service": "mercadolivre",
				"endpoint":         "cache_set_affiliate_preview",
			})
		}
	}
	return preview, nil
}

// parseAffiliateSocialHTML extrai título, preço, preço original e imagem do
// produto-alvo a partir do HTML da página /social do afiliado. O alvo é sempre
// o 1º card (produto que o link de afiliado destaca), cujo componente de preço
// é isolado por affiliatePriceSegment — evitando casar título (que quebra com
// acentos no JSON).
// Falha graciosamente: sem og:title retorna nil; sem preço retorna o que tiver.
// upgradeMLImageURL troca a variante de baixa resolução do mlstatic
// (D_NQ_NP_...) pela 2X (retina), dobrando a resolução. O og:image vem pequeno
// (ex.: 146px de largura) e fica pixelado no WhatsApp; a 2X mantém o mesmo
// recorte/proporção, só com o dobro de pixels. No-op p/ URLs que não casam ou
// que já são 2X.
func upgradeMLImageURL(u string) string {
	if u == "" {
		return u
	}
	if strings.Contains(u, "mlstatic.com/D_NQ_NP_") && !strings.Contains(u, "_2X_") {
		return strings.Replace(u, "/D_NQ_NP_", "/D_NQ_NP_2X_", 1)
	}
	return u
}

// affiliatePriceSegment devolve o trecho de JSON que descreve o preço do
// PRIMEIRO card (produto-alvo) da página /social.
//
// Ancorar no 1º `"price":{` deixou de funcionar em 30/07/2026: o ML passou a
// emitir, ANTES do card, um bloco de promoções
// (`"price":{"promotions":[...],"price":1395.39}`) que capturava o índice e
// empurrava o current_price real para fora da janela — Price=0 e item pulado
// (1210 skips/dia em 30/07). A âncora agora é o COMPONENTE de card,
// `{"type":"price"`, conferido em 14/14 páginas de produção.
//
// A aspa final da âncora é OBRIGATÓRIA: sem ela, `{"type":"price_decrement"`
// (o objeto de promoção) casaria e o bug voltaria por outra porta.
func affiliatePriceSegment(html string) string {
	window := affiliatePriceWindow
	start := strings.Index(html, `{"type":"price"`)
	if start < 0 {
		// Página sem componentes tipados: cai no comportamento histórico —
		// âncora E janela originais. Alargar a janela AQUI seria perigoso: sem
		// componente tipado não há como cortar no card seguinte (o corte abaixo
		// procura a mesma âncora que acabou de não existir), então 2000 bytes
		// atravessariam de um blob de tracking até o card de outro produto e
		// sairia preço plausível do item errado — pior que o Price=0 de hoje.
		start = strings.Index(html, `"price":{`)
		window = 800
	}
	if start < 0 {
		return ""
	}

	end := start + window
	if end > len(html) {
		end = len(html)
	}
	// NUNCA invade o card seguinte. Com a janela maior isto deixou de ser zelo
	// e virou requisito: sem o corte, um card sem previous_price herdaria o do
	// card vizinho e inventaria um "De X por Y" que não existe.
	if next := strings.Index(html[start+1:], `{"type":"price"`); next >= 0 && start+1+next < end {
		end = start + 1 + next
	}
	return html[start:end]
}

func parseAffiliateSocialHTML(html, finalURL string) *models.MLProductPreview {
	title := extractMeta(html, "og:title")
	if title == "" {
		return nil
	}
	preview := &models.MLProductPreview{
		Title:     title,
		Currency:  "BRL",
		Permalink: finalURL,
		Image:     upgradeMLImageURL(extractMeta(html, "og:image")),
	}

	// Escopa ao componente de preço do 1º card (produto-alvo). Ver
	// affiliatePriceSegment para por que a âncora não é mais o 1º `"price":{`.
	if seg := affiliatePriceSegment(html); seg != "" {
		// Preço atual: current_price.value; fallback p/ "price":{"value":N} (sem desconto).
		if m := affiliateCurrentPriceRe.FindStringSubmatch(seg); m != nil {
			if v, err := strconv.ParseFloat(m[1], 64); err == nil {
				preview.Price = v
			}
		} else if m := affiliateBareValueRe.FindStringSubmatch(seg); m != nil {
			if v, err := strconv.ParseFloat(m[1], 64); err == nil {
				preview.Price = v
			}
		}

		// Preço original: previous_price.value.
		if m := affiliatePrevPriceRe.FindStringSubmatch(seg); m != nil {
			if op, err := strconv.ParseFloat(m[1], 64); err == nil && op > 0 {
				preview.OriginalPrice = &op
			}
		}
	}

	if preview.Image != "" {
		preview.Thumbnail = preview.Image
		preview.Pictures = []string{preview.Image}
	}

	if preview.OriginalPrice != nil && *preview.OriginalPrice > preview.Price && preview.Price > 0 {
		d := ((*preview.OriginalPrice - preview.Price) / *preview.OriginalPrice) * 100
		preview.DiscountPercent = &d
		// O ML não devolve percentual de desconto: aqui os preços SÃO a fonte,
		// e são pré-cupom (o cupom só entra depois, ao montar a mensagem).
		// Discount alimenta template, então vai normalizado (inteiro, 1..100);
		// DiscountPercent segue cru para não mexer no card já exibido.
		preview.Discount = text.DiscountPercentFromPrices(*preview.OriginalPrice, preview.Price)
	}

	return preview
}

// normalizeURL normaliza uma URL para cache e para emissão.
//
// Antes removia SÓ o fragmento, então userinfo, porta e — o que importa
// comercialmente — os parâmetros de tracking da FONTE sobreviviam até a URL
// emitida ao grupo. Agora delega ao commerceurl.StripTracking, que remove
// userinfo, fragmento e os parâmetros de atribuição conhecidos (utm_*, af_*,
// matt_*, affiliate_*, deal_print_id, tracking_id), preservando parâmetros
// funcionais como quantity e variation, que não são tracking.
func normalizeURL(rawURL string) string {
	u, err := parseURL(rawURL)
	if err != nil {
		return strings.TrimSpace(rawURL)
	}
	return commerceurl.StripTracking(u)
}

// parseURL parses a URL safely
func parseURL(rawURL string) (*url.URL, error) {
	return url.Parse(strings.TrimSpace(rawURL))
}

// parseProductFromHTML extracts product preview data from HTML
// Uses the html_parser.go functions for full parity with Node.js implementation
func (s *mlService) parseProductFromHTML(html, originURL, productID string) *models.MLProductPreview {
	preview := &models.MLProductPreview{
		ID:        productID,
		Permalink: originURL,
		Currency:  "BRL", // Default for Brazil
	}

	// Extract title using og:title, twitter:title, or <title> fallback
	preview.Title = extractTitle(html)

	// Extract price using meta tags, JSON-LD, or text regex
	price, currency := extractPrice(html)
	preview.Price = price
	if currency != "" {
		preview.Currency = currency
	}

	// Extract image using og:image or twitter:image
	preview.Image = extractImage(html)
	preview.Thumbnail = preview.Image
	if preview.Image != "" {
		preview.Pictures = []string{preview.Image}
	}

	// Extract original price
	preview.OriginalPrice = extractOriginalPrice(html)

	// Calculate discount from original price (mesma regra do caminho de
	// afiliado: preços são a única fonte no ML e são pré-cupom).
	if preview.OriginalPrice != nil && *preview.OriginalPrice > preview.Price && preview.Price > 0 {
		discount := ((*preview.OriginalPrice - preview.Price) / *preview.OriginalPrice) * 100
		preview.Discount = text.DiscountPercentFromPrices(*preview.OriginalPrice, preview.Price)
		preview.DiscountPercent = &discount
	}

	// Try extracting discount percent directly from "X% OFF" text
	if discountPct := extractDiscountPercent(html); discountPct != nil {
		preview.DiscountPercent = discountPct
		if preview.Discount == 0 {
			preview.Discount = *discountPct
		}
	}

	// Extract condition
	preview.Condition = extractCondition(html)

	// Extract sold quantity
	soldQty := extractSoldQuantity(html)
	preview.SoldCount = soldQty
	preview.SoldQuantity = soldQty

	// Extract rating and review count
	ratingAvg, reviewCount := extractRating(html)
	preview.Rating = ratingAvg
	preview.RatingAverage = ratingAvg
	preview.ReviewCount = reviewCount
	preview.ReviewsTotal = reviewCount

	// Extract installments info
	if installments := extractInstallments(html); installments != nil {
		installStr := installments.Text
		if installStr == "" {
			installStr = fmt.Sprintf("em %dx R$ %.2f", installments.Quantity, installments.Amount)
		}
		preview.Installments = &installStr
	}

	return preview
}

func (s *mlService) LoadCredentials(ctx context.Context, userID string) (*models.MLCredentials, error) {
	dbCreds, err := s.mlCredsRepo.Get(ctx, userID)
	if err != nil {
		observability.LogAndCapture(s.logger, sentry.LevelError, "Failed to get ML credentials from repository", err, map[string]string{
			"external_service": "mercadolivre",
			"endpoint":         "loadCredentials",
		})
		return nil, err
	}
	if dbCreds == nil {
		return nil, fmt.Errorf("no ML credentials found for user")
	}

	// Decrypt credentials
	var creds models.MLCredentials
	var blob crypto.EncryptedBlobV1
	if err := json.Unmarshal([]byte(dbCreds.EncryptedData), &blob); err != nil {
		observability.LogAndCapture(s.logger, sentry.LevelError, "Failed to unmarshal ML encrypted credentials blob", err, map[string]string{
			"external_service": "mercadolivre",
			"endpoint":         "loadCredentials",
		})
		return nil, fmt.Errorf("failed to parse encrypted credentials: %w", err)
	}

	if err := crypto.DecryptJSONV1(&blob, s.encryptionKey, &creds); err != nil {
		observability.LogAndCapture(s.logger, sentry.LevelError, "Failed to decrypt ML credentials", err, map[string]string{
			"external_service": "mercadolivre",
			"endpoint":         "loadCredentials",
		})
		return nil, fmt.Errorf("failed to decrypt credentials: %w", err)
	}

	return &creds, nil
}

func (s *mlService) SaveCredentials(ctx context.Context, userID, cookie, csrf, tag string) error {
	creds := &models.MLCredentials{
		Cookie: cookie,
		CSRF:   csrf,
		Tag:    tag,
	}

	// Encrypt credentials
	blob, err := crypto.EncryptJSONV1(creds, s.encryptionKey)
	if err != nil {
		observability.LogAndCapture(s.logger, sentry.LevelError, "Failed to encrypt ML credentials", err, map[string]string{
			"external_service": "mercadolivre",
			"endpoint":         "saveCredentials",
		})
		return fmt.Errorf("failed to encrypt credentials: %w", err)
	}

	encryptedData, err := json.Marshal(blob)
	if err != nil {
		observability.LogAndCapture(s.logger, sentry.LevelError, "Failed to marshal ML encrypted blob", err, map[string]string{
			"external_service": "mercadolivre",
			"endpoint":         "saveCredentials",
		})
		return fmt.Errorf("failed to marshal encrypted blob: %w", err)
	}

	if saveErr := s.mlCredsRepo.Save(ctx, userID, string(encryptedData)); saveErr != nil {
		observability.LogAndCapture(s.logger, sentry.LevelError, "Failed to save ML credentials to repository", saveErr, map[string]string{
			"external_service": "mercadolivre",
			"endpoint":         "saveCredentials",
		})
		return saveErr
	}
	return nil
}

func (s *mlService) IsConnected(ctx context.Context, userID string) (bool, error) {
	creds, err := s.LoadCredentials(ctx, userID)
	if err != nil {
		return false, nil
	}
	return creds != nil && creds.Cookie != "" && creds.CSRF != "", nil
}

// Patterns match UPPERCASED URLs (see ExtractProductID)
var (
	mlProductIDRegex  = regexp.MustCompile(`MLB[A-Z]?-?(\d+)`)
	mlProductIDRegex2 = regexp.MustCompile(`/(?:P|UP)/(MLB[A-Z]?\d+)`)
	mlProductIDRegex3 = regexp.MustCompile(`MERCADOLIVRE\.COM\.BR/[^/]+/(?:P|UP)/(MLB[A-Z]?\d+)`)
)

func (s *mlService) ExtractProductID(productURL string) (string, error) {
	// Normalize URL
	productURL = strings.ToUpper(productURL)

	// Try different patterns
	patterns := []*regexp.Regexp{mlProductIDRegex3, mlProductIDRegex2, mlProductIDRegex}

	for _, pattern := range patterns {
		matches := pattern.FindStringSubmatch(productURL)
		if len(matches) > 1 {
			productID := matches[1]
			// Normalize to MLB format without hyphen
			productID = strings.ReplaceAll(productID, "-", "")
			if !strings.HasPrefix(productID, "MLB") {
				productID = "MLB" + productID
			}
			return productID, nil
		}
	}

	return "", fmt.Errorf("could not extract product ID from URL")
}

// InvalidatePreviewCache invalidates the cached preview for a product.
// This allows forcing a fresh fetch on the next GetProductPreview call.
func (s *mlService) InvalidatePreviewCache(ctx context.Context, originURL string) error {
	if s.cache == nil {
		return nil // No cache configured, nothing to invalidate
	}

	// Extract product ID to build the cache key
	productID, err := s.ExtractProductID(originURL)
	if err != nil {
		return fmt.Errorf("invalid mercadolivre URL: %w", err)
	}

	cacheKey := fmt.Sprintf("ml_preview:%s", productID)

	if err := s.cache.Delete(ctx, cacheKey); err != nil {
		s.logger.Warn("Failed to invalidate preview cache",
			zap.String("cacheKey", cacheKey),
			zap.Error(err))
		// Don't return error - cache invalidation is best effort
	}

	s.logger.Info("ML preview cache invalidated",
		zap.String("productID", productID),
		zap.String("originURL", originURL))

	return nil
}
