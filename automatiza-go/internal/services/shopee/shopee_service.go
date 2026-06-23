package shopee

import (
	"automatiza-go/internal/lib/cache"
	"automatiza-go/internal/models"
	"automatiza-go/internal/observability"
	"automatiza-go/internal/repository"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.uber.org/zap"
)

// Service defines the Shopee service interface
type Service interface {
	SearchOffers(ctx context.Context, userID string, params models.ShopeeSearchParams) (*models.ShopeeSearchResult, error)
	GenerateAffiliateLink(ctx context.Context, productURL, userID string) (*models.ShopeeAffiliateLink, error)
	GetCategories(ctx context.Context, parentID *int64) ([]models.ShopeeCategory, error)
	GetProduct(ctx context.Context, itemID, shopID int64, userID string) (*models.ShopeeOffer, error)
	ExpandURL(ctx context.Context, shortURL string) (string, error)
	ExtractProductIDs(productURL string) (shopID, itemID int64, err error)
}

type shopeeService struct {
	httpClient    *http.Client
	cache         *cache.Cache
	affiliateRepo repository.AffiliateCredentialsRepository
	settingsRepo  repository.SettingsRepository
	baseURL       string
	categoryMap   map[string][]int
	logger        *zap.Logger
	breaker       *circuitBreaker
}

// ErrShopeeNoAPIAccess marks a Shopee Affiliate Open API access denial — error
// 10035 ("You currently do not have access to the Shopee Affiliate Open API
// Platform"). It is a per-account permission problem, NOT a transient failure:
// every call with those credentials fails identically. Callers short-circuit on
// it (and set a per-user cooldown) instead of grinding through the slow
// productOfferV2 → generateShortLink fallback chain for every link of every
// message. Without this, accounts lacking Open API access pile up calls until the
// offer_generate context deadline trips (measured: 693 timeouts/6h across many
// clients), starving delivery for everyone.
var ErrShopeeNoAPIAccess = errors.New("shopee: account has no Affiliate Open API Platform access (error 10035)")

// noAPIAccessCooldown is how long Shopee calls are skipped for a user after a
// 10035, so one client's missing API access stops hammering the worker.
const noAPIAccessCooldown = 10 * time.Minute

// isNoAPIAccessMessage detects the Shopee 10035 "no Open API access" error.
func isNoAPIAccessMessage(msg string) bool {
	return strings.Contains(msg, "10035") ||
		strings.Contains(msg, "do not have access to the Shopee Affiliate Open API")
}

func shopeeNoAPIAccessKey(userID string) string {
	return "shopee:noapi:" + userID
}

// inNoAPIAccessCooldown reports whether this user recently hit a 10035 and should
// skip Shopee calls entirely for now. Fail-open (false) on nil cache / errors.
func (s *shopeeService) inNoAPIAccessCooldown(ctx context.Context, userID string) bool {
	if s.cache == nil || userID == "" {
		return false
	}
	val, err := s.cache.Get(ctx, shopeeNoAPIAccessKey(userID))
	return err == nil && val != ""
}

// markNoAPIAccess records a 10035 cooldown for the user (best-effort).
func (s *shopeeService) markNoAPIAccess(ctx context.Context, userID string) {
	if s.cache == nil || userID == "" {
		return
	}
	_, _ = s.cache.SetNX(ctx, shopeeNoAPIAccessKey(userID), "1", noAPIAccessCooldown)
}

// NewShopeeService creates a new Shopee service
func NewShopeeService(
	cache *cache.Cache,
	affiliateRepo repository.AffiliateCredentialsRepository,
	settingsRepo repository.SettingsRepository,
	logger *zap.Logger,
) Service {
	return &shopeeService{
		httpClient: &http.Client{
			// 8s (was 30s): Shopee GraphQL normally responds in <2s. A 30s
			// timeout means each call to a DOWN Shopee hangs 30s; in the serial
			// automation scheduler that burns the shared 5min tick budget after
			// ~10 hung calls, starving every other automation (ML lists, etc).
			// Fail fast and let the circuit breaker take over.
			Timeout:   8 * time.Second,
			Transport: otelhttp.NewTransport(http.DefaultTransport),
		},
		cache:         cache,
		affiliateRepo: affiliateRepo,
		settingsRepo:  settingsRepo,
		baseURL:       "https://open-api.affiliate.shopee.com.br",
		categoryMap:   initCategoryMapping(),
		logger:        logger,
		// Open after 5 consecutive infra failures, stay open 30s, then probe.
		breaker: newCircuitBreaker(5, 30*time.Second),
	}
}

// getShopeeCredentials extracts appID and secret from the credentials,
// checking the JSONB Credentials map first (camelCase keys from frontend),
// then falling back to the legacy AppID/AppSecret columns.
func getShopeeCredentials(creds *models.AffiliateCredentials) (appID, secret string) {
	appID = creds.GetCredential("appId")
	if appID == "" {
		appID = creds.GetCredential("app_id")
	}
	if appID == "" {
		appID = creds.AppID
	}

	secret = creds.GetCredential("password")
	if secret == "" {
		secret = creds.GetCredential("app_secret")
	}
	if secret == "" {
		secret = creds.AppSecret
	}
	return
}

// shopeeSubIDDefault é o Sub ID fixo enviado pra Shopee em todo link
// gerado pela plataforma. Identifica a Automatisa como fonte da venda no
// relatório de afiliado, sem expor configuração ao usuário final — todo
// usuário recebe a mesma tag. Granularidade adicional (ex.: sub_id por
// user/automation) pode vir num próximo PR usando os outros 4 slots de
// subIds que a API aceita.
const shopeeSubIDDefault = "AUTOMATISA"

// buildGenerateShortLinkMutation monta o GraphQL mutation da Shopee para
// generateShortLink, opcionalmente incluindo subIds. Tag vazia → omite
// subIds. Em produção, callers passam shopeeSubIDDefault.
func buildGenerateShortLinkMutation(originURL, trackingTag string) string {
	if trackingTag == "" {
		return fmt.Sprintf(`mutation { generateShortLink(input: {originUrl: %q}) { shortLink } }`, originURL)
	}
	return fmt.Sprintf(`mutation { generateShortLink(input: {originUrl: %q, subIds: [%q]}) { shortLink } }`, originURL, trackingTag)
}

// shopeeSign generates the SHA256 signature for the Shopee Affiliate Open API.
// Signature = SHA256(appID + timestamp + payload + secret)
func shopeeSign(appID, timestamp, payload, secret string) string {
	h := sha256.New()
	h.Write([]byte(appID + timestamp + payload + secret))
	return hex.EncodeToString(h.Sum(nil))
}

// shopeeGraphQL executes an authenticated GraphQL request to the Shopee Open API.
func (s *shopeeService) shopeeGraphQL(ctx context.Context, appID, secret, payload string) (json.RawMessage, error) {
	// Circuit breaker: when Shopee has been failing, short-circuit instantly
	// instead of hanging up to the HTTP timeout. This is what keeps a Shopee
	// outage from cascading into the shared automation-scheduler context and
	// stalling ML / list / offer automations for everyone.
	if !s.breaker.canRequest() {
		s.logger.Warn("Shopee circuit breaker open, skipping GraphQL call",
			zap.String("external_service", "shopee"))
		return nil, ErrShopeeUnavailable
	}

	ts := strconv.FormatInt(time.Now().Unix(), 10)
	sig := shopeeSign(appID, ts, payload, secret)

	endpoint := s.baseURL + "/graphql"
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewBufferString(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("SHA256 Credential=%s,Timestamp=%s,Signature=%s", appID, ts, sig))

	span := sentry.StartSpan(ctx, "http.client", sentry.WithDescription("POST "+endpoint))
	span.SetTag("external_service", "shopee")
	span.SetTag("endpoint", "graphql")
	resp, err := s.httpClient.Do(req)
	span.Finish()
	if err != nil {
		// Network error / timeout / context deadline = infra failure → trip breaker.
		s.breaker.recordFailure()
		observability.LogAndCapture(s.logger, sentry.LevelError, "Shopee GraphQL HTTP request failed", err, map[string]string{
			"external_service": "shopee",
			"endpoint":         "graphql",
		})
		return nil, fmt.Errorf("shopee API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20)) // 2MB limit
	if err != nil {
		s.breaker.recordFailure()
		observability.LogAndCapture(s.logger, sentry.LevelError, "Failed to read Shopee GraphQL response body", err, map[string]string{
			"external_service": "shopee",
			"endpoint":         "graphql",
		})
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		// 5xx = server-side outage → trip breaker. 4xx is a request/config
		// problem (bad creds, bad query): the endpoint is up, don't trip.
		if resp.StatusCode >= 500 {
			s.breaker.recordFailure()
		} else {
			s.breaker.recordSuccess()
		}
		httpErr := fmt.Errorf("shopee API error %d: %s", resp.StatusCode, string(body))
		observability.LogAndCapture(s.logger, sentry.LevelError, "Shopee GraphQL returned non-200 status", httpErr, map[string]string{
			"external_service": "shopee",
			"endpoint":         "graphql",
			"http_status":      fmt.Sprintf("%d", resp.StatusCode),
		})
		return nil, httpErr
	}

	// Reached the endpoint and got 200 — Shopee is up. GraphQL-level errors
	// below (e.g. invalid signature) are config issues, not an outage.
	s.breaker.recordSuccess()

	// Check for GraphQL errors
	var gqlResp struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(body, &gqlResp); err != nil {
		observability.LogAndCapture(s.logger, sentry.LevelError, "Failed to unmarshal Shopee GraphQL response", err, map[string]string{
			"external_service": "shopee",
			"endpoint":         "graphql",
		})
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	if len(gqlResp.Errors) > 0 {
		errMsg := gqlResp.Errors[0].Message
		gqlErr := fmt.Errorf("shopee API error: %s", errMsg)

		// Error 10035 = the account has no Open API Platform access. Permanent
		// per-account permission issue, not a transient/code error: return the
		// sentinel so callers fail fast (and set a cooldown) instead of retrying
		// the slow fallback chain. Log locally only — no Sentry flood.
		if isNoAPIAccessMessage(errMsg) {
			s.logger.Warn("Shopee account has no Open API access (10035), failing fast (not sent to Sentry)",
				zap.String("external_service", "shopee"),
				zap.String("error", errMsg))
			return nil, fmt.Errorf("%w: %s", ErrShopeeNoAPIAccess, errMsg)
		}

		// "Invalid Signature" / "invalid value" are user-config issues (wrong credentials),
		// not code bugs. Log locally only — no Sentry event to avoid flooding.
		if strings.Contains(errMsg, "Invalid Signature") ||
			strings.Contains(errMsg, "invalid value") {
			s.logger.Warn("Shopee GraphQL returned user-config error (not sent to Sentry)",
				zap.String("external_service", "shopee"),
				zap.String("error", errMsg))
			return nil, gqlErr
		}

		observability.LogAndCapture(s.logger, sentry.LevelError, "Shopee GraphQL returned error", gqlErr, map[string]string{
			"external_service": "shopee",
			"endpoint":         "graphql",
		})
		return nil, gqlErr
	}

	return gqlResp.Data, nil
}

func (s *shopeeService) SearchOffers(ctx context.Context, userID string, params models.ShopeeSearchParams) (*models.ShopeeSearchResult, error) {
	s.logger.Info("Shopee SearchOffers started",
		zap.String("userID", userID),
		zap.Any("keyword", params.Keyword),
		zap.Strings("categories", params.Categories),
		zap.Int("limit", params.Limit),
		zap.Any("minPrice", params.MinPrice),
		zap.Any("maxPrice", params.MaxPrice))

	// Skip if this user's Shopee account recently returned 10035 (no Open API
	// access) — avoids piling up doomed calls/timeouts.
	if s.inNoAPIAccessCooldown(ctx, userID) {
		return nil, ErrShopeeNoAPIAccess
	}

	// Get affiliate credentials
	creds, err := s.affiliateRepo.GetByUserAndStore(ctx, userID, "shopee")
	if err != nil {
		return nil, fmt.Errorf("failed to get shopee credentials: %w", err)
	}
	if creds == nil {
		return nil, fmt.Errorf("shopee credentials not configured")
	}
	appID, secret := getShopeeCredentials(creds)
	if appID == "" || secret == "" {
		return nil, fmt.Errorf("shopee credentials incomplete - please configure App ID and Password")
	}

	// Resolve category names to Shopee numeric IDs
	categoryIDs := s.resolveCategoryIDs(params.Categories)

	// If we have explicit CategoryID param, use it
	if params.CategoryID != nil {
		categoryIDs = []int{int(*params.CategoryID)}
	}

	// Build GraphQL query
	limit := params.Limit
	if limit <= 0 {
		limit = 20
	}

	// sortType: 1=relevance, 2=sales, 3=highest price, 4=lowest price, 5=commission
	// When SortBy is explicitly set, use it for all categories.
	// When nil (automations), randomize per category for maximum product variety.
	var fixedSortType *int
	if params.SortBy != nil {
		st := 2
		switch *params.SortBy {
		case "relevance":
			st = 1
		case "sales":
			st = 2
		case "price_desc":
			st = 3
		case "price_asc":
			st = 4
		case "commission":
			st = 5
		}
		fixedSortType = &st
	}
	// Pool of sort types for variety: relevance, sales, commission
	varietySortTypes := []int{1, 2, 5}

	// If no categories resolved (or "Ofertas gerais"), search without category filter
	if len(categoryIDs) == 0 {
		sortType := 2
		if fixedSortType != nil {
			sortType = *fixedSortType
		} else {
			sortType = varietySortTypes[rand.Intn(len(varietySortTypes))]
		}
		res, serr := s.searchShopee(ctx, appID, secret, params.Keyword, nil, sortType, limit, params)
		if serr != nil && errors.Is(serr, ErrShopeeNoAPIAccess) {
			s.markNoAPIAccess(ctx, userID)
		}
		return res, serr
	}

	// Search per category and aggregate results
	var allOffers []models.ShopeeOffer
	perCategoryLimit := limit
	if len(categoryIDs) > 1 {
		perCategoryLimit = max(5, limit/len(categoryIDs))
	}

	for _, catID := range categoryIDs {
		catIDCopy := int64(catID)
		sortType := 2
		if fixedSortType != nil {
			sortType = *fixedSortType
		} else {
			sortType = varietySortTypes[rand.Intn(len(varietySortTypes))]
		}
		result, err := s.searchShopee(ctx, appID, secret, params.Keyword, &catIDCopy, sortType, perCategoryLimit, params)
		if err != nil {
			if errors.Is(err, ErrShopeeNoAPIAccess) {
				// No Open API access: every category will fail the same way.
				// Set the cooldown and stop iterating.
				s.markNoAPIAccess(ctx, userID)
				return nil, err
			}
			s.logger.Warn("Shopee search failed for category",
				zap.Int("categoryID", catID),
				zap.Error(err))
			continue
		}
		allOffers = append(allOffers, result.Offers...)
	}

	s.logger.Info("Shopee SearchOffers completed",
		zap.Int("offersAfterFilter", len(allOffers)),
		zap.Int("categoriesSearched", len(categoryIDs)))

	return &models.ShopeeSearchResult{
		Offers:  allOffers,
		Total:   len(allOffers),
		HasMore: false,
	}, nil
}

// resolveCategoryIDs maps category display names to Shopee numeric IDs.
// "Ofertas gerais" returns empty (no filter). Each category name can map to
// multiple IDs; we pick the first one for the API call.
func (s *shopeeService) resolveCategoryIDs(categories []string) []int {
	if len(categories) == 0 {
		return nil
	}

	var ids []int
	for _, cat := range categories {
		if cat == "Ofertas gerais" || cat == "ofertasGerais" {
			// No category filter — search everything
			return nil
		}
		if catIDs, ok := s.categoryMap[cat]; ok && len(catIDs) > 0 {
			ids = append(ids, catIDs[0])
		}
	}
	return ids
}

// searchShopee executes a single Shopee GraphQL search with optional keyword and category.
func (s *shopeeService) searchShopee(ctx context.Context, appID, secret string, keyword *string, categoryID *int64, sortType, limit int, params models.ShopeeSearchParams) (*models.ShopeeSearchResult, error) {
	// Build GraphQL query with variables for correct type handling
	varDefs := "$sortType: Int!, $limit: Int!"
	queryArgs := "sortType: $sortType, limit: $limit"
	variables := map[string]interface{}{
		"sortType": sortType,
		"limit":    limit,
	}

	if keyword != nil && *keyword != "" {
		varDefs += ", $keyword: String"
		queryArgs += ", keyword: $keyword"
		variables["keyword"] = *keyword
	}
	if categoryID != nil {
		varDefs += ", $productCatId: Int"
		queryArgs += ", productCatId: $productCatId"
		variables["productCatId"] = *categoryID
	}
	if params.Offset > 0 {
		page := params.Offset/limit + 1
		varDefs += ", $page: Int"
		queryArgs += ", page: $page"
		variables["page"] = page
	}

	gqlQuery := fmt.Sprintf(`query(%s) { productOfferV2(%s) { nodes { productName itemId shopId productLink offerLink imageUrl priceMin priceMax price commission commissionRate sales ratingStar priceDiscountRate shopName productCatIds } } }`, varDefs, queryArgs)

	payload, marshalErr := json.Marshal(map[string]interface{}{
		"query":     gqlQuery,
		"variables": variables,
	})
	if marshalErr != nil {
		observability.LogAndCapture(s.logger, sentry.LevelError, "Failed to marshal Shopee search payload", marshalErr, map[string]string{
			"external_service": "shopee",
			"endpoint":         "searchOffers",
		})
		return nil, fmt.Errorf("failed to marshal search payload: %w", marshalErr)
	}
	payloadStr := string(payload)

	data, err := s.shopeeGraphQL(ctx, appID, secret, payloadStr)
	if err != nil {
		return nil, err
	}

	// Parse response
	var result struct {
		ProductOfferV2 struct {
			Nodes []struct {
				ProductName       string `json:"productName"`
				ItemID            int64  `json:"itemId"`
				ShopID            int64  `json:"shopId"`
				ProductLink       string `json:"productLink"`
				OfferLink         string `json:"offerLink"`
				ImageURL          string `json:"imageUrl"`
				PriceMin          string `json:"priceMin"`
				PriceMax          string `json:"priceMax"`
				Price             string `json:"price"`
				Commission        string `json:"commission"`
				CommissionRate    string `json:"commissionRate"`
				Sales             int64  `json:"sales"`
				RatingStar        string `json:"ratingStar"`
				PriceDiscountRate int    `json:"priceDiscountRate"`
				ShopName          string `json:"shopName"`
			} `json:"nodes"`
		} `json:"productOfferV2"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		observability.LogAndCapture(s.logger, sentry.LevelError, "Failed to unmarshal Shopee search results", err, map[string]string{
			"external_service": "shopee",
			"endpoint":         "searchOffers",
		})
		return nil, fmt.Errorf("failed to parse search results: %w", err)
	}

	s.logger.Debug("Shopee API response",
		zap.Int("rawNodes", len(result.ProductOfferV2.Nodes)))

	// Convert to our model
	offers := make([]models.ShopeeOffer, 0, len(result.ProductOfferV2.Nodes))
	for _, node := range result.ProductOfferV2.Nodes {
		discount := float64(node.PriceDiscountRate)

		// Apply discount filter
		if params.MinDiscount != nil && discount < *params.MinDiscount {
			continue
		}

		priceMin, _ := strconv.ParseFloat(node.PriceMin, 64)
		priceMax, _ := strconv.ParseFloat(node.PriceMax, 64)
		price, _ := strconv.ParseFloat(node.Price, 64)
		commissionRate, _ := strconv.ParseFloat(node.CommissionRate, 64)
		ratingStar, _ := strconv.ParseFloat(node.RatingStar, 64)

		// Apply price filters
		effectivePrice := price
		if effectivePrice == 0 {
			effectivePrice = priceMin
		}
		if params.MinPrice != nil && effectivePrice < *params.MinPrice {
			continue
		}
		if params.MaxPrice != nil && effectivePrice > *params.MaxPrice {
			continue
		}

		// Calculate original price from discount
		var originalPrice float64
		if discount > 0 && effectivePrice > 0 {
			originalPrice = effectivePrice / (1 - discount/100)
		}
		// If no discount data, use priceMax as original or fall back to effective price
		if originalPrice == 0 && effectivePrice > 0 {
			if priceMax > effectivePrice {
				originalPrice = priceMax
			} else {
				originalPrice = effectivePrice
			}
		}

		// Skip offers with no real discount
		if effectivePrice > 0 && originalPrice > 0 && effectivePrice >= originalPrice {
			continue
		}

		offer := models.ShopeeOffer{
			ItemID:         node.ItemID,
			ShopID:         node.ShopID,
			Name:           node.ProductName,
			Image:          node.ImageURL,
			Price:          effectivePrice,
			OriginalPrice:  originalPrice,
			PriceMin:       priceMin,
			PriceMax:       priceMax,
			Discount:       discount,
			CommissionRate: commissionRate,
			Sales:          int(node.Sales),
			ShopName:       node.ShopName,
			ShopRating:     ratingStar,
			AffiliateLink:  node.OfferLink,
			ProductURL:     node.ProductLink,
		}

		offers = append(offers, offer)
	}

	return &models.ShopeeSearchResult{
		Offers:  offers,
		Total:   len(offers),
		HasMore: len(result.ProductOfferV2.Nodes) >= limit,
	}, nil
}

func isShopeeShortURL(rawURL string) bool {
	lower := strings.ToLower(rawURL)
	return strings.Contains(lower, "s.shopee.com.br/") ||
		strings.Contains(lower, "shp.ee/") ||
		strings.Contains(lower, "shope.ee/")
}

// isNonProductShopeeURL detects Shopee URLs that are not product pages
// (e.g., coupon pages, campaign pages, voucher wallets, store profiles)
// and must be regenerated through generateShortLink with the user's credentials
// rather than productOfferV2.
func isNonProductShopeeURL(expandedURL string) bool {
	lower := strings.ToLower(expandedURL)
	return strings.Contains(lower, "shopee.com.br/m/") ||
		strings.Contains(lower, "shopee.com.br/mall/") ||
		strings.Contains(lower, "shopee.com.br/flash_sale") ||
		strings.Contains(lower, "shopee.com.br/daily_discover") ||
		strings.Contains(lower, "shopee.com.br/voucher") ||
		strings.Contains(lower, "shopee.com.br/user/") ||
		strings.Contains(lower, "shopee.com.br/oficial")
}

// generateNonProductAffiliateLink converts non-product Shopee URLs (coupons, campaigns)
// into affiliate short links using the client's credentials via generateShortLink mutation.
func (s *shopeeService) generateNonProductAffiliateLink(ctx context.Context, originalURL, expandedURL, userID string) (*models.ShopeeAffiliateLink, error) {
	// Get affiliate credentials for this user
	creds, err := s.affiliateRepo.GetByUserAndStore(ctx, userID, "shopee")
	if err != nil || creds == nil {
		return nil, fmt.Errorf("shopee credentials not configured")
	}
	appID, secret := getShopeeCredentials(creds)

	s.logger.Info("Generating affiliate link for non-product Shopee URL",
		zap.String("original", originalURL),
		zap.String("expanded", expandedURL),
		zap.String("userID", userID))

	// Use generateShortLink mutation — works with any Shopee URL.
	// Sub ID fixo em shopeeSubIDDefault — não configurável pelo user.
	shortLinkQuery := buildGenerateShortLinkMutation(expandedURL, shopeeSubIDDefault)
	shortPayload, marshalErr := json.Marshal(map[string]string{"query": shortLinkQuery})
	if marshalErr != nil {
		return nil, fmt.Errorf("failed to marshal generateShortLink payload: %w", marshalErr)
	}

	shortData, shortErr := s.shopeeGraphQL(ctx, appID, secret, string(shortPayload))
	if shortErr != nil {
		if errors.Is(shortErr, ErrShopeeNoAPIAccess) {
			s.markNoAPIAccess(ctx, userID)
		}
		return nil, fmt.Errorf("failed to generate affiliate link for non-product URL: %w", shortErr)
	}

	var shortResult struct {
		GenerateShortLink struct {
			ShortLink string `json:"shortLink"`
		} `json:"generateShortLink"`
	}
	if unmarshalErr := json.Unmarshal(shortData, &shortResult); unmarshalErr != nil {
		return nil, fmt.Errorf("failed to parse generateShortLink response: %w", unmarshalErr)
	}

	affiliateLink := shortResult.GenerateShortLink.ShortLink
	if affiliateLink == "" {
		return nil, fmt.Errorf("no affiliate link generated for non-product URL")
	}

	s.logger.Info("Non-product affiliate link generated",
		zap.String("original", originalURL),
		zap.String("affiliateLink", affiliateLink))

	return &models.ShopeeAffiliateLink{
		OriginalURL:   originalURL,
		AffiliateLink: affiliateLink,
		ShortLink:     affiliateLink,
	}, nil
}

func (s *shopeeService) GenerateAffiliateLink(ctx context.Context, productURL, userID string) (*models.ShopeeAffiliateLink, error) {
	// Skip if this user's Shopee account recently returned 10035 (no Open API
	// access): the link can't be generated without it, so don't expand URLs or
	// make calls that will all fail and pile up timeouts.
	if s.inNoAPIAccessCooldown(ctx, userID) {
		return nil, ErrShopeeNoAPIAccess
	}

	// Extract item and shop IDs from URL
	shopID, itemID, err := s.ExtractProductIDs(productURL)
	if err != nil {
		// Short URL? Try expanding first (s.shopee.com.br, shp.ee, shope.ee)
		if isShopeeShortURL(productURL) {
			expanded, expandErr := s.ExpandURL(ctx, productURL)
			if expandErr != nil {
				return nil, fmt.Errorf("failed to expand short URL: %w", expandErr)
			}
			shopID, itemID, err = s.ExtractProductIDs(expanded)
			if err != nil {
				// Non-product pages (coupons, campaigns) — convert via generateShortLink
				if isNonProductShopeeURL(expanded) {
					return s.generateNonProductAffiliateLink(ctx, productURL, expanded, userID)
				}
				return nil, fmt.Errorf("invalid shopee URL after expansion: %w", err)
			}
			s.logger.Info("Short URL expanded for affiliate link",
				zap.String("original", productURL),
				zap.String("expanded", expanded))
		} else {
			return nil, fmt.Errorf("invalid shopee URL: %w", err)
		}
	}

	// Check cache first (keyed by userID to prevent cross-user affiliate link pollution)
	cacheKey := fmt.Sprintf("shopee_link:%s:%d:%d", userID, shopID, itemID)
	var cached models.ShopeeAffiliateLink
	if s.cache != nil {
		if err := s.cache.GetJSON(ctx, cacheKey, &cached); err == nil {
			s.logger.Info("AffiliateLink cache hit",
				zap.Int64("itemID", itemID),
				zap.Int64("shopID", shopID))
			return &cached, nil
		}
	}

	// Get affiliate credentials
	creds, err := s.affiliateRepo.GetByUserAndStore(ctx, userID, "shopee")
	if err != nil || creds == nil {
		return nil, fmt.Errorf("shopee credentials not configured")
	}
	appID, secret := getShopeeCredentials(creds)

	s.logger.Info("AffiliateLink API call",
		zap.Int64("itemID", itemID),
		zap.Int64("shopID", shopID))

	// Try productOfferV2 first — returns offerLink + imageUrl in 1 API call
	gqlQuery := fmt.Sprintf(`{ productOfferV2(itemId: %d, shopId: %d, limit: 1) { nodes { offerLink imageUrl } } }`, itemID, shopID)
	payload, marshalErr := json.Marshal(map[string]string{"query": gqlQuery})
	if marshalErr != nil {
		observability.LogAndCapture(s.logger, sentry.LevelError, "Failed to marshal Shopee productOfferV2 payload", marshalErr, map[string]string{
			"external_service": "shopee",
			"endpoint":         "productOfferV2_link",
		})
		return nil, fmt.Errorf("failed to marshal productOfferV2 payload: %w", marshalErr)
	}

	// productOfferV2 serve aqui só para obter a imagem (e o offerLink como
	// ultimo recurso). O LINK de afiliado vem do generateShortLink abaixo,
	// porque o offerLink do productOfferV2 NÃO carrega subIds custom — só o
	// generateShortLink aplica o sub_id (confirmado na doc da Shopee). Sem isso
	// o tracking por sub_id (AUTOMATISA) nunca chegava aos links de produto.
	var offerLinkFallback string
	var imageURL string

	data, err := s.shopeeGraphQL(ctx, appID, secret, string(payload))
	if err == nil {
		var offerResult struct {
			ProductOfferV2 struct {
				Nodes []struct {
					OfferLink string `json:"offerLink"`
					ImageURL  string `json:"imageUrl"`
				} `json:"nodes"`
			} `json:"productOfferV2"`
		}
		if unmarshalErr := json.Unmarshal(data, &offerResult); unmarshalErr == nil &&
			len(offerResult.ProductOfferV2.Nodes) > 0 {
			node := offerResult.ProductOfferV2.Nodes[0]
			offerLinkFallback = node.OfferLink
			imageURL = node.ImageURL
		}
	} else if errors.Is(err, ErrShopeeNoAPIAccess) {
		// No Open API access: generateShortLink would fail identically. Set the
		// cooldown and bail now instead of making a second doomed call.
		s.markNoAPIAccess(ctx, userID)
		return nil, err
	} else {
		s.logger.Warn("productOfferV2 failed, generating link via generateShortLink",
			zap.Int64("itemID", itemID),
			zap.Error(err))
	}

	// Gera o link SEMPRE via generateShortLink para que o sub_id (AUTOMATISA)
	// seja aplicado em todo link de produto.
	var affiliateLink string
	originURL := fmt.Sprintf("https://shopee.com.br/product/%d/%d", shopID, itemID)
	// Sub ID fixo em shopeeSubIDDefault — não configurável pelo user.
	shortLinkQuery := buildGenerateShortLinkMutation(originURL, shopeeSubIDDefault)
	shortPayload, shortMarshalErr := json.Marshal(map[string]string{"query": shortLinkQuery})
	if shortMarshalErr != nil {
		return nil, fmt.Errorf("failed to marshal generateShortLink payload: %w", shortMarshalErr)
	}
	shortData, shortErr := s.shopeeGraphQL(ctx, appID, secret, string(shortPayload))
	if shortErr != nil {
		s.logger.Warn("generateShortLink failed, falling back to productOfferV2 offerLink (sem sub_id)",
			zap.Int64("itemID", itemID),
			zap.Error(shortErr))
	} else {
		var shortResult struct {
			GenerateShortLink struct {
				ShortLink string `json:"shortLink"`
			} `json:"generateShortLink"`
		}
		if unmarshalErr := json.Unmarshal(shortData, &shortResult); unmarshalErr != nil {
			s.logger.Warn("failed to parse generateShortLink response, falling back to offerLink",
				zap.Int64("itemID", itemID),
				zap.Error(unmarshalErr))
		} else {
			affiliateLink = shortResult.GenerateShortLink.ShortLink
		}
	}

	// Se o generateShortLink falhou, usa o offerLink do productOfferV2 como
	// ultimo recurso (sem sub_id, mas melhor que não enviar a oferta).
	if affiliateLink == "" {
		affiliateLink = offerLinkFallback
	}

	if affiliateLink == "" {
		return nil, fmt.Errorf("no affiliate link generated")
	}

	s.logger.Info("AffiliateLink generated",
		zap.String("affiliateLink", affiliateLink),
		zap.String("imageURL", imageURL))

	result := &models.ShopeeAffiliateLink{
		OriginalURL:   productURL,
		AffiliateLink: affiliateLink,
		ShortLink:     affiliateLink,
		ImageURL:      imageURL,
		ItemID:        itemID,
		ShopID:        shopID,
	}

	// Cache for 1 hour (best effort)
	if s.cache != nil {
		if cacheErr := s.cache.SetJSON(ctx, cacheKey, result, time.Hour); cacheErr != nil {
			observability.LogAndCapture(s.logger, sentry.LevelWarning, "Failed to cache Shopee affiliate link", cacheErr, map[string]string{
				"external_service": "shopee",
				"endpoint":         "cache_set_link",
			})
		}
	}

	return result, nil
}

func (s *shopeeService) GetCategories(ctx context.Context, parentID *int64) ([]models.ShopeeCategory, error) {
	// Return from static mapping for now
	categories := make([]models.ShopeeCategory, 0)
	for name, ids := range s.categoryMap {
		if len(ids) > 0 {
			categories = append(categories, models.ShopeeCategory{
				ID:   int64(ids[0]),
				Name: name,
			})
		}
	}
	return categories, nil
}

func (s *shopeeService) GetProduct(ctx context.Context, itemID, shopID int64, userID string) (*models.ShopeeOffer, error) {
	s.logger.Info("GetProduct started",
		zap.Int64("itemID", itemID),
		zap.Int64("shopID", shopID))

	// Skip if this user's Shopee account recently returned 10035 (no Open API
	// access) — every call would fail the same way and pile up timeouts.
	if s.inNoAPIAccessCooldown(ctx, userID) {
		return nil, ErrShopeeNoAPIAccess
	}

	// Get affiliate credentials
	creds, err := s.affiliateRepo.GetByUserAndStore(ctx, userID, "shopee")
	if err != nil || creds == nil {
		return nil, fmt.Errorf("shopee credentials not configured")
	}
	appID, secret := getShopeeCredentials(creds)

	// Use productOfferV2 with itemId and shopId to get product details.
	// shopType is included so the model's IsMall flag (= shopType == 1)
	// can drive the coupon rule engine's official_only filter.
	gqlQuery := fmt.Sprintf(`{ productOfferV2(itemId: %d, shopId: %d, limit: 1) { nodes { productName itemId shopId productLink offerLink imageUrl priceMin priceMax price commission commissionRate sales ratingStar priceDiscountRate shopName shopType } } }`, itemID, shopID)
	payload, marshalErr := json.Marshal(map[string]string{"query": gqlQuery})
	if marshalErr != nil {
		observability.LogAndCapture(s.logger, sentry.LevelError, "Failed to marshal Shopee getProduct payload", marshalErr, map[string]string{
			"external_service": "shopee",
			"endpoint":         "getProduct",
		})
		return nil, fmt.Errorf("failed to marshal getProduct payload: %w", marshalErr)
	}
	payloadStr := string(payload)

	data, err := s.shopeeGraphQL(ctx, appID, secret, payloadStr)
	if err != nil {
		// No Open API access (10035): set the cooldown so the next links/messages
		// for this user skip Shopee entirely. Don't flood Sentry — it's a per-user
		// access issue, not a code/outage problem.
		if errors.Is(err, ErrShopeeNoAPIAccess) {
			s.markNoAPIAccess(ctx, userID)
			return nil, err
		}
		observability.LogAndCapture(s.logger, sentry.LevelError, "Shopee getProduct API failed", err, map[string]string{
			"external_service": "shopee",
			"endpoint":         "getProduct",
		})
		return nil, err
	}

	var result struct {
		ProductOfferV2 struct {
			Nodes []struct {
				ProductName       string `json:"productName"`
				ItemID            int64  `json:"itemId"`
				ShopID            int64  `json:"shopId"`
				ProductLink       string `json:"productLink"`
				OfferLink         string `json:"offerLink"`
				ImageURL          string `json:"imageUrl"`
				PriceMin          string `json:"priceMin"`
				PriceMax          string `json:"priceMax"`
				Price             string `json:"price"`
				Commission        string `json:"commission"`
				CommissionRate    string `json:"commissionRate"`
				Sales             int64  `json:"sales"`
				RatingStar        string `json:"ratingStar"`
				PriceDiscountRate int    `json:"priceDiscountRate"`
				ShopName          string `json:"shopName"`
				// shopType: 1=Mall (loja oficial), 2=Preferred, 3=Normal.
				// Drives ShopeeOffer.IsMall, which the coupon rule
				// matcher reads to honor official_only filters.
				//
				// Shopee's API actually returns this as either an int
				// or an int array depending on the product (some
				// products have multiple shop_type tags — Mall and
				// Preferred at once). RawMessage lets us accept both
				// shapes and resolve to "is Mall?" in one helper.
				ShopType json.RawMessage `json:"shopType"`
			} `json:"nodes"`
		} `json:"productOfferV2"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		observability.LogAndCapture(s.logger, sentry.LevelError, "Failed to unmarshal Shopee product details", err, map[string]string{
			"external_service": "shopee",
			"endpoint":         "getProduct",
		})
		return nil, fmt.Errorf("failed to parse product details: %w", err)
	}

	if len(result.ProductOfferV2.Nodes) == 0 {
		return nil, fmt.Errorf("product not found")
	}

	node := result.ProductOfferV2.Nodes[0]
	price, _ := strconv.ParseFloat(node.Price, 64)
	priceMin, _ := strconv.ParseFloat(node.PriceMin, 64)
	commissionRate, _ := strconv.ParseFloat(node.CommissionRate, 64)
	ratingStar, _ := strconv.ParseFloat(node.RatingStar, 64)

	effectivePrice := price
	if effectivePrice == 0 {
		effectivePrice = priceMin
	}

	var originalPrice float64
	discount := float64(node.PriceDiscountRate)
	priceMax, _ := strconv.ParseFloat(node.PriceMax, 64)
	if discount > 0 && effectivePrice > 0 {
		originalPrice = effectivePrice / (1 - discount/100)
	}
	// If no discount data, use priceMax as original or fall back to effective price
	if originalPrice == 0 && effectivePrice > 0 {
		if priceMax > effectivePrice {
			originalPrice = priceMax
		} else {
			originalPrice = effectivePrice
		}
	}

	s.logger.Info("GetProduct completed",
		zap.Int64("itemID", itemID),
		zap.Float64("price", effectivePrice),
		zap.Float64("originalPrice", originalPrice))

	return &models.ShopeeOffer{
		ItemID:         node.ItemID,
		ShopID:         node.ShopID,
		Name:           node.ProductName,
		Price:          effectivePrice,
		OriginalPrice:  originalPrice,
		Sales:          int(node.Sales),
		ShopName:       node.ShopName,
		ShopRating:     ratingStar,
		AffiliateLink:  node.OfferLink,
		CommissionRate: commissionRate,
		ProductURL:     node.ProductLink,
		Image:          node.ImageURL,
		Discount:       discount,
		// Shopee shopType=1 marks Mall lojas oficiais — the only signal
		// the affiliate API gives us about official storefronts. Used
		// by coupon rules with official_only=true to filter dispatches.
		IsMall: shopTypeIsMall(node.ShopType),
	}, nil
}

// shopTypeIsMall normalizes Shopee's `shopType` field, which the API
// returns as either a scalar int (`1`) or an int array (`[1, 5]`) —
// products with multiple shop tags. We treat the shop as Mall whenever
// the value 1 appears in either shape. Unknown shapes return false so
// the rule engine fails closed (won't dispatch a Mall-only coupon to
// an unverified shop).
func shopTypeIsMall(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	var asInt int
	if err := json.Unmarshal(raw, &asInt); err == nil {
		return asInt == 1
	}
	var asArr []int
	if err := json.Unmarshal(raw, &asArr); err == nil {
		for _, v := range asArr {
			if v == 1 {
				return true
			}
		}
	}
	return false
}

func (s *shopeeService) ExpandURL(ctx context.Context, shortURL string) (string, error) {
	// Check cache
	cacheKey := "url_expand:" + shortURL
	if s.cache != nil {
		if cached, err := s.cache.Get(ctx, cacheKey); err == nil && cached != "" {
			return cached, nil
		}
	}

	// Follow redirects
	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	req, reqErr := http.NewRequestWithContext(ctx, "HEAD", shortURL, nil)
	if reqErr != nil {
		observability.LogAndCapture(s.logger, sentry.LevelError, "Failed to create Shopee ExpandURL request", reqErr, map[string]string{
			"external_service": "shopee",
			"endpoint":         "expandURL",
		})
		return shortURL, reqErr
	}

	span := sentry.StartSpan(ctx, "http.client", sentry.WithDescription("HEAD "+shortURL))
	span.SetTag("external_service", "shopee")
	span.SetTag("endpoint", "expandURL")
	resp, err := client.Do(req)
	span.Finish()
	if err != nil {
		observability.LogAndCapture(s.logger, sentry.LevelError, "Shopee ExpandURL HTTP request failed", err, map[string]string{
			"external_service": "shopee",
			"endpoint":         "expandURL",
		})
		return shortURL, err
	}
	defer resp.Body.Close()

	finalURL := resp.Request.URL.String()

	// Cache for 24 hours (best effort)
	if s.cache != nil {
		if cacheErr := s.cache.Set(ctx, cacheKey, finalURL, 24*time.Hour); cacheErr != nil {
			observability.LogAndCapture(s.logger, sentry.LevelWarning, "Failed to cache Shopee expanded URL", cacheErr, map[string]string{
				"external_service": "shopee",
				"endpoint":         "cache_set_expandURL",
			})
		}
	}

	return finalURL, nil
}

var (
	shopeeURLRegex  = regexp.MustCompile(`shopee\.com\.br/.*?\.(\d+)\.(\d+)`)
	shopeeURLRegex2 = regexp.MustCompile(`shopee\.com\.br/product/(\d+)/(\d+)`)
	shopeeURLRegex3 = regexp.MustCompile(`shopee\.com\.br/[^/]+/(\d+)/(\d+)`)
)

func (s *shopeeService) ExtractProductIDs(productURL string) (shopID, itemID int64, err error) {
	// Try pattern 1: shopee.com.br/product-name.shopid.itemid
	matches := shopeeURLRegex.FindStringSubmatch(productURL)
	if len(matches) == 3 {
		shopID, _ = strconv.ParseInt(matches[1], 10, 64)
		itemID, _ = strconv.ParseInt(matches[2], 10, 64)
		return shopID, itemID, nil
	}

	// Try pattern 2: shopee.com.br/product/shopid/itemid
	matches = shopeeURLRegex2.FindStringSubmatch(productURL)
	if len(matches) == 3 {
		shopID, _ = strconv.ParseInt(matches[1], 10, 64)
		itemID, _ = strconv.ParseInt(matches[2], 10, 64)
		return shopID, itemID, nil
	}

	// Try pattern 3: shopee.com.br/slug/shopid/itemid (expanded short URLs)
	matches = shopeeURLRegex3.FindStringSubmatch(productURL)
	if len(matches) == 3 {
		shopID, _ = strconv.ParseInt(matches[1], 10, 64)
		itemID, _ = strconv.ParseInt(matches[2], 10, 64)
		return shopID, itemID, nil
	}

	return 0, 0, fmt.Errorf("could not extract product IDs from URL")
}

// parseDiscount parses discount string like "50%" to float64 50.0
func parseDiscount(discount string) float64 {
	discount = strings.TrimSuffix(discount, "%")
	discount = strings.TrimSpace(discount)
	val, _ := strconv.ParseFloat(discount, 64)
	return val
}

// initCategoryMapping returns category name to ID mapping.
// IMPORTANT: Uses IDs from Shopee "categories" field (100XXX), NOT "fe_categories" (110XXXXX).
// The "categories" IDs are the ones that work with the Shopee Affiliate API.
// Source of truth: antigo-projeto/backend/src/config/shopeeCategories.ts
//
//nolint:misspell // Portuguese category names are not English misspellings
func initCategoryMapping() map[string][]int {
	return map[string][]int{
		// ── Main categories ──
		"Ofertas gerais": {}, // No category filter (searches everything)

		"Homens": {
			100011, // Roupas Masculinas
			100053, // Bermudas
			100533, // Bolsas & Acessórios Masculinos
			100599, // Cuidados Masculinos
			100629, // Relógios Masculinos
			100247, // Cuecas
			100244, // Camisetas (masc)
			100047, // Jeans (masc)
			100238, // Calças Cargo (masc)
			100226, // Moletons (masc)
			101310, // Jaquetas (sports/outdoor)
			101301, // Calçados para Tênis (sports)
			100259, // Chinelos (masc)
			100255, // Botas da Moda (masc)
			100067, // Oxfords (masc)
			100062, // Meias (masc)
			100028, // Bonés, Chapéus e Toucas
			100608, // Porta-cartões
			100032, // Cintos
			100151, // Óculos de sol
			101624, // Aparadores/Tosquiadeiras
			101620, // Cremes/Espumas/Géis (barbear)
			100661, // Perfumes e Fragrâncias
		},

		"Moda": {
			100017, // Roupas Femininas
			100011, // Roupas Masculinas
			100104, // Vestidos
			100360, // Shorts
			100557, // Calçados: Tênis (fem)
			100560, // Calçados: Plataformas
			100358, // Calças (fem)
			100103, // Jeans (fem)
			100352, // Camisetas (fem)
			100353, // Blusas (fem)
			100095, // Bolsas
			100382, // Lingerie: Calcinhas
			100053, // Bermudas (masc)
			100242, // Camisas (masc)
			100047, // Jeans (masc)
			100064, // Calçados: Tênis (masc)
			100259, // Calçados: Chinelos (masc)
			100009, // Acessórios de Moda
			100028, // Bonés, Chapéus e Toucas
		},

		"Eletrônicos": {
			100644, // Computadores e Acessórios
			101940, // Acessórios e Periféricos
			100634, // Jogos e Consoles
			100013, // Celulares e Dispositivos
			100287, // Powerbanks e Baterias
			100635, // Cameras e drones
			100486, // Powerbanks
			100075, // Acessórios
			100277, // Ventiladores Móveis e USB
			100074, // Aparelhos Vestíveis
			100270, // Smartwatches
			100290, // Capas
			100490, // Capas para celulares
			100285, // Luzes Móveis e USB
			100535, // Audio
			100578, // Fones de Ouvido, Headphones e Headsets
		},

		"Eletrodomésticos": {
			100010, // Eletrodomésticos (base)
			100198, // Air Fryers
			100193, // Espremedores, Liquidificadores
			100194, // Cafeteiras e Acessórios
			100200, // Micro-ondas
			100202, // Torradeiras
			100195, // Batedeiras e Mixers
			100209, // Geladeiras
			100190, // Filtros de Água
			100466, // Ventiladores
			100464, // Ar-Condicionado
			100465, // Ar-condicionado portátil
			100177, // Aspiradores de Pó
			100463, // Lava e Seca
			100175, // Ferros de Passar
			100185, // TVs
		},

		"Casa e Decoração": {
			100717, // Artigos de Cozinha
			100710, // Roupas de Cama
			100711, // Decoração
			100709, // Banheiros
			100721, // Organizadores
			100713, // Móveis
			101220, // Armazenamento de Alimentos
			101233, // Raladores, Descascadores
			101146, // Cobertores, Edredons
			101160, // Carpetes e Tapetes
			101155, // Cortinas e Persianas
			101161, // Vasos e Vasilhas
			101156, // Molduras / Decoração de Parede
			101133, // Suportes / Caixas de Sabonete
			101259, // Organizadores de Guarda-Roupas
			101255, // Sapateiras
			101174, // Prateleiras e Racks
			101169, // Escrivaninhas e Mesas
			101170, // Guarda-Roupas
			101173, // Armários e Gabinetes
			101168, // Camas / cabeceiras / colchões
			101172, // Sofás
			101171, // Bancos / Cadeiras
		},

		"Beleza": {
			100630, // Beleza
			100661, // Perfumes e Fragrâncias
			100662, // Maquiagem
			100882, // Olhos
			101639, // Máscara de Cílios
			100883, // Lábios
			100664, // Cuidados com a pele
			100898, // Máscaras Faciais
			100663, // Utensílios de Beleza
			100885, // Acessórios de Maquiagem
			101650, // Bolsas de Maquiagem
			100886, // Utensílios de Cuidados Faciais
			101661, // Massagem Facial
			100659, // Cuidados com o Cabelo
			100871, // Tratamento Capilar
			100889, // Utensílios Capilares
			101664, // Escovas e Pentes
			100658, // Cuidados com Mãos, Pés e Unhas
			100866, // Cuidados com os Pés
			100893, // Hidratante Facial
			100901, // Protetor Solar Facial
			100896, // Soro e Essência Facial
			100665, // Pacotes e Conjuntos de Beleza
			101628, // Base
			101631, // Corretivo
			101630, // Pó
			100869, // Shampoo
			100873, // Estilo do Cabelo
			100870, // Tinta de Cabelo
			102008, // Desodorantes Corporais
			102007, // Creme Corporal
			102003, // Sabonete
			102010, // Depilação
			102178, // Gel Para Unhas
			102034, // Ferramentas Manicure
		},

		"Esportes": {
			100637, // Esportes e Atividades ao Ar Livre
			100725, // Equipamentos Esportivos
			101295, // Equipamento Fitness
			101893, // Máquinas de Ginástica
			101311, // Camisetas (fitness)
			100245, // Regatas (masc)
			101890, // Musculação
			101847, // Bolas (futebol)
			101871, // Luvas/Bandagens (lutas)
			101869, // Sacos de pancada (lutas)
			101329, // Acessórios esportivos
		},

		"Livros": {
			100643, // Livros e Revistas
			100777, // Livros
			101547, // Política, Direito e Ciências Sociais
			101566, // Carreiras, Autoajuda
		},

		"Brinquedos": {
			100739, // Brinquedos e Jogos
			101400, // Jogos de Tabuleiro e Cartas
			100639, // Hobbies e Coleções
			100737, // Itens Colecionáveis
			101385, // Figuras de Ação
			101407, // Brinquedos de Controle Remoto
		},

		"Infantil": {
			100633, // Moda Infantil
			100689, // Acessórios Infantis
			100693, // Calçados de Menina
			100690, // Roupas de Meninos
		},

		"Maternidade": {
			100632, // Mãe e Bebê
			100674, // Essenciais para Viagens com Bebês
		},

		"Alimentos": {
			100789, // Sementes
			100654, // Padaria
			100646, // Lanches
			100788, // Chips
		},

		"Automotivo": {
			102187, // Peças e Acessórios para Veículos
			102272, // Limpeza Veicular
			102250, // Som Automotivo
			102264, // Pneus e Rodas
			102552, // Pneus para Automóveis
		},

		"Pets": {
			100631, // Animais Domésticos
			100668, // Acessórios para Animais
			100920, // Móveis para Animais
			100916, // Tigelas e Alimentadores
		},

		// ── Subcategories: Moda Feminina ──
		"Moda Feminina - Vestidos":               {100104},
		"Moda Feminina - Shorts":                 {100360},
		"Moda Feminina - Calçados (Tênis)":       {100557},
		"Moda Feminina - Calçados (Plataformas)": {100560},
		"Moda Feminina - Calças":                 {100358},
		"Moda Feminina - Jeans":                  {100103},
		"Moda Feminina - Camisetas":              {100352},
		"Moda Feminina - Blusas":                 {100353},
		"Moda Feminina - Bolsas":                 {100095},
		"Moda Feminina - Lingerie (Calcinhas)":   {100382},

		// ── Subcategories: Moda Masculina ──
		"Moda Masculina - Bermudas":            {100053},
		"Moda Masculina - Camisas":             {100242},
		"Moda Masculina - Jeans":               {100047},
		"Moda Masculina - Calçados (Tênis)":    {100064},
		"Moda Masculina - Calçados (Chinelos)": {100259},

		// ── Subcategories: Homens ──
		"Homens - Roupas (Camisetas)":                   {100244},
		"Homens - Roupas (Bermudas)":                    {100053},
		"Homens - Roupas (Jeans)":                       {100047},
		"Homens - Roupas (Calças Cargo)":                {100238},
		"Homens - Roupas (Moletons)":                    {100226},
		"Homens - Roupas (Jaquetas)":                    {101310},
		"Homens - Calçados (Tênis)":                     {101301},
		"Homens - Calçados (Chinelos)":                  {100259},
		"Homens - Calçados (Botas)":                     {100255},
		"Homens - Calçados (Sociais/Oxford)":            {100067},
		"Homens - Íntimas (Cuecas)":                     {100247},
		"Homens - Íntimas (Meias)":                      {100062},
		"Homens - Acessórios (Bonés)":                   {100028},
		"Homens - Acessórios (Relógios)":                {100574},
		"Homens - Acessórios (Carteiras/Porta-cartões)": {100608},
		"Homens - Acessórios (Cintos)":                  {100032},
		"Homens - Acessórios (Óculos de sol)":           {100151},
		"Homens - Barbearia (Aparadores)":               {101624},
		"Homens - Barbearia (Cremes/Géis)":              {101620},
		"Homens - Perfumes":                             {100661},

		// ── Subcategories: Eletrônicos ──
		"Eletrônicos - Celulares (Smartphones)":      {100073},
		"Eletrônicos - Celulares (Capas)":            {100490},
		"Eletrônicos - Celulares (Carregadores)":     {100482},
		"Eletrônicos - Celulares (Powerbank)":        {100486},
		"Eletrônicos - Computadores (Notebooks)":     {101942},
		"Eletrônicos - Computadores (SSD)":           {101962},
		"Eletrônicos - Computadores (RAM)":           {101955},
		"Eletrônicos - Computadores (Teclados)":      {101999},
		"Eletrônicos - Computadores (Monitores)":     {101933},
		"Eletrônicos - Áudio (Fones/Headsets)":       {100578},
		"Eletrônicos - Áudio (Caixas de Som)":        {100625},
		"Eletrônicos - Games (Consoles)":             {101081},
		"Eletrônicos - Games (Acessórios/Controles)": {100696},
		"Eletrônicos - Wearables (Smartwatch)":       {100270},
		"Eletrônicos - Câmeras (Webcams)":            {101988},
		"Eletrônicos - Drones":                       {100705},

		// ── Subcategories: Eletrodomésticos ──
		"Eletrodomésticos - Cozinha (Airfryer)":                   {100198},
		"Eletrodomésticos - Cozinha (Liquidificador/Espremedor)":  {100193},
		"Eletrodomésticos - Cozinha (Cafeteira)":                  {100194},
		"Eletrodomésticos - Cozinha (Micro-ondas)":                {100200},
		"Eletrodomésticos - Cozinha (Torradeira/Sanduicheira)":    {100202},
		"Eletrodomésticos - Cozinha (Batedeira/Mixer)":            {100195},
		"Eletrodomésticos - Cozinha (Geladeira)":                  {100209},
		"Eletrodomésticos - Cozinha (Filtro/Água)":                {100190},
		"Eletrodomésticos - Climatização (Ventilador)":            {100466},
		"Eletrodomésticos - Climatização (Ar-condicionado)":       {100464},
		"Eletrodomésticos - Climatização (Portátil/Climatizador)": {100465},
		"Eletrodomésticos - Limpeza (Aspirador/Robô)":             {100177},
		"Eletrodomésticos - Lavanderia (Lava e Seca)":             {100463},
		"Eletrodomésticos - Lavanderia (Ferro/Vaporizador)":       {100175},
		"Eletrodomésticos - TVs (TV)":                             {100185},

		// ── Subcategories: Casa e Decoração ──
		"Casa - Cozinha (Armazenamento)":      {101220},
		"Casa - Cozinha (Cortadores)":         {101233},
		"Casa - Quarto (Cama/Edredom)":        {101146},
		"Casa - Decoração (Tapetes)":          {101160},
		"Casa - Decoração (Cortinas)":         {101155},
		"Casa - Decoração (Vasos)":            {101161},
		"Casa - Decoração (Parede/Molduras)":  {101156},
		"Casa - Banheiro (Suportes/Sabonete)": {101133},
		"Casa - Organização (Guarda-roupa)":   {101259},
		"Casa - Organização (Sapateiras)":     {101255},
		"Casa - Móveis (Prateleiras/Racks)":   {101174},
		"Casa - Móveis (Mesas/Escrivaninhas)": {101169},
		"Casa - Móveis (Guarda-roupas)":       {101170},
		"Casa - Móveis (Armários/Gabinetes)":  {101173},
		"Casa - Móveis (Camas/Cabeceiras)":    {101168},
		"Casa - Móveis (Sofás)":               {101172},
		"Casa - Móveis (Cadeiras/Banquetas)":  {101171},

		// ── Subcategories: Beleza ──
		"Beleza - Perfumes e Fragrâncias":           {100661},
		"Beleza - Skincare (Hidratante Facial)":     {100893},
		"Beleza - Skincare (Protetor Solar Facial)": {100901},
		"Beleza - Skincare (Soro/Essência)":         {100896},
		"Beleza - Skincare (Máscaras)":              {100898},
		"Beleza - Skincare (Massagem Facial)":       {101661},
		"Beleza - Maquiagem (Base)":                 {101628},
		"Beleza - Maquiagem (Corretivo)":            {101631},
		"Beleza - Maquiagem (Pó)":                   {101630},
		"Beleza - Maquiagem (Kits)":                 {100665},
		"Beleza - Cabelo (Shampoo)":                 {100869},
		"Beleza - Cabelo (Tratamento)":              {100871},
		"Beleza - Cabelo (Finalizadores)":           {100873},
		"Beleza - Cabelo (Tintura)":                 {100870},
		"Beleza - Corpo (Desodorantes/Body Splash)": {102008},
		"Beleza - Corpo (Hidratante Corporal)":      {102007},
		"Beleza - Corpo (Sabonetes)":                {102003},
		"Beleza - Corpo (Depilação)":                {102010},
		"Beleza - Unhas (Gel)":                      {102178},
		"Beleza - Unhas (Ferramentas)":              {102034},

		// ── Subcategories: Esportes ──
		"Esportes - Fitness (Camisetas)":             {101311},
		"Esportes - Fitness (Regatas)":               {100245},
		"Esportes - Fitness (Musculação/Acessórios)": {101890},
		"Esportes - Fitness (Máquinas)":              {101893},
		"Esportes - Outdoor/Ciclismo (Acessórios)":   {101329},
		"Esportes - Futebol (Bolas)":                 {101847},
		"Esportes - Lutas (Luvas/Bandagens)":         {101871},
		"Esportes - Lutas (Sacos de Pancada)":        {101869},
	}
}
