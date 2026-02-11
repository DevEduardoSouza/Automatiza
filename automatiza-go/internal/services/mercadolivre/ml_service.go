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
	"strings"
	"time"

	"automatiza-go/internal/lib/cache"
	"automatiza-go/internal/lib/crypto"
	"automatiza-go/internal/models"
	"automatiza-go/internal/observability"
	"automatiza-go/internal/repository"

	"github.com/getsentry/sentry-go"
	"go.uber.org/zap"
)

// Service defines the Mercado Livre service interface
type Service interface {
	CreateAffiliateLink(ctx context.Context, originURL, userID string) (*models.MLAffiliateLink, error)
	GetProductPreview(ctx context.Context, originURL, userID string) (*models.MLProductPreview, error)
	LoadCredentials(ctx context.Context, userID string) (*models.MLCredentials, error)
	SaveCredentials(ctx context.Context, userID, cookie, csrf string) error
	IsConnected(ctx context.Context, userID string) (bool, error)
	ExtractProductID(productURL string) (string, error)
	InvalidatePreviewCache(ctx context.Context, originURL string) error
}

type mlService struct {
	httpClient    *http.Client
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
			Timeout: 30 * time.Second,
		},
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
			"endpoint":        "getByOriginalURL",
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
				"endpoint":        "db_save_link",
			})
		}
	}

	// Cache for 6 hours (matching Node.js for parity)
	if s.cache != nil {
		if cacheErr := s.cache.SetJSON(ctx, cacheKey, result, linkCacheTTL); cacheErr != nil {
			observability.LogAndCapture(s.logger, sentry.LevelWarning, "Failed to cache ML affiliate link", cacheErr, map[string]string{
				"external_service": "mercadolivre",
				"endpoint":        "cache_set_link",
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
				if saveErr := s.SaveCredentials(context.Background(), userID, creds.Cookie, csrfCopy); saveErr != nil {
					observability.LogAndCapture(s.logger, sentry.LevelWarning, "Failed to save initial CSRF credentials", saveErr, map[string]string{
						"external_service": "mercadolivre",
						"endpoint":        "saveCredentials",
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
			"tag":  "", // Tag is embedded in the cookie session
		}

		jsonPayload, marshalErr := json.Marshal(payload)
		if marshalErr != nil {
			observability.LogAndCapture(s.logger, sentry.LevelError, "Failed to marshal ML createLink payload", marshalErr, map[string]string{
				"external_service": "mercadolivre",
				"endpoint":        "createLink",
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
				"endpoint":        "createLink",
			})
			return nil, nil, fmt.Errorf("failed to create short link: %w", err)
		}
		defer resp.Body.Close()

		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB limit
		if readErr != nil {
			observability.LogAndCapture(s.logger, sentry.LevelError, "Failed to read ML createLink response body", readErr, map[string]string{
				"external_service": "mercadolivre",
				"endpoint":        "createLink",
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
			if saveErr := s.SaveCredentials(context.Background(), userID, creds.Cookie, csrfRetry); saveErr != nil {
				observability.LogAndCapture(s.logger, sentry.LevelWarning, "Failed to save refreshed CSRF credentials", saveErr, map[string]string{
					"external_service": "mercadolivre",
					"endpoint":        "saveCredentials",
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
		httpErr := fmt.Errorf("ML API error %d: session may have expired", resp.StatusCode)
		observability.LogAndCapture(s.logger, sentry.LevelError, "ML createLink returned non-200 status", httpErr, map[string]string{
			"external_service": "mercadolivre",
			"endpoint":        "createLink",
			"http_status":     fmt.Sprintf("%d", resp.StatusCode),
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
			"endpoint":        "createLink",
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

	// Fetch product page
	req, err := http.NewRequestWithContext(ctx, "GET", cleanURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120 Safari/537.36")
	if creds != nil {
		req.Header.Set("Cookie", creds.Cookie)
	}

	span := sentry.StartSpan(ctx, "http.client", sentry.WithDescription("GET "+cleanURL))
	span.SetTag("external_service", "mercadolivre")
	span.SetTag("endpoint", "productPreview")
	resp, err := s.httpClient.Do(req)
	span.Finish()
	if err != nil {
		observability.LogAndCapture(s.logger, sentry.LevelError, "Failed to fetch ML product preview", err, map[string]string{
			"external_service": "mercadolivre",
			"endpoint":        "productPreview",
		})
		return nil, fmt.Errorf("failed to fetch product: %w", err)
	}
	defer resp.Body.Close()

	// If non-200 or not HTML, return empty preview with short cache
	if resp.StatusCode != http.StatusOK {
		emptyPreview := &models.MLProductPreview{
			ID:        productID,
			Permalink: cleanURL,
			Currency:  "BRL",
		}
		if s.cache != nil {
			if cacheErr := s.cache.SetJSON(ctx, cacheKey, emptyPreview, previewCacheTTLEmpty); cacheErr != nil {
				observability.LogAndCapture(s.logger, sentry.LevelWarning, "Failed to cache empty ML preview", cacheErr, map[string]string{
					"external_service": "mercadolivre",
					"endpoint":        "cache_set_preview",
				})
			}
		}
		return emptyPreview, nil
	}

	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 5<<20)) // 5MB limit for HTML pages
	if readErr != nil {
		observability.LogAndCapture(s.logger, sentry.LevelError, "Failed to read ML product preview response body", readErr, map[string]string{
			"external_service": "mercadolivre",
			"endpoint":        "productPreview",
		})
		return nil, fmt.Errorf("failed to read product page: %w", readErr)
	}
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
				"endpoint":        "cache_set_preview",
			})
		}
	}

	return preview, nil
}

// normalizeURL normalizes a URL for consistent caching
func normalizeURL(rawURL string) string {
	u, err := parseURL(rawURL)
	if err != nil {
		return strings.TrimSpace(rawURL)
	}
	// Remove hash fragment (never reaches server, keep cache stable)
	u.Fragment = ""
	return u.String()
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

	// Calculate discount from original price
	if preview.OriginalPrice != nil && *preview.OriginalPrice > preview.Price && preview.Price > 0 {
		discount := ((*preview.OriginalPrice - preview.Price) / *preview.OriginalPrice) * 100
		preview.Discount = discount
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
			"endpoint":        "loadCredentials",
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
			"endpoint":        "loadCredentials",
		})
		return nil, fmt.Errorf("failed to parse encrypted credentials: %w", err)
	}

	if err := crypto.DecryptJSONV1(&blob, s.encryptionKey, &creds); err != nil {
		observability.LogAndCapture(s.logger, sentry.LevelError, "Failed to decrypt ML credentials", err, map[string]string{
			"external_service": "mercadolivre",
			"endpoint":        "loadCredentials",
		})
		return nil, fmt.Errorf("failed to decrypt credentials: %w", err)
	}

	return &creds, nil
}

func (s *mlService) SaveCredentials(ctx context.Context, userID, cookie, csrf string) error {
	creds := &models.MLCredentials{
		Cookie: cookie,
		CSRF:   csrf,
	}

	// Encrypt credentials
	blob, err := crypto.EncryptJSONV1(creds, s.encryptionKey)
	if err != nil {
		observability.LogAndCapture(s.logger, sentry.LevelError, "Failed to encrypt ML credentials", err, map[string]string{
			"external_service": "mercadolivre",
			"endpoint":        "saveCredentials",
		})
		return fmt.Errorf("failed to encrypt credentials: %w", err)
	}

	encryptedData, err := json.Marshal(blob)
	if err != nil {
		observability.LogAndCapture(s.logger, sentry.LevelError, "Failed to marshal ML encrypted blob", err, map[string]string{
			"external_service": "mercadolivre",
			"endpoint":        "saveCredentials",
		})
		return fmt.Errorf("failed to marshal encrypted blob: %w", err)
	}

	if saveErr := s.mlCredsRepo.Save(ctx, userID, string(encryptedData)); saveErr != nil {
		observability.LogAndCapture(s.logger, sentry.LevelError, "Failed to save ML credentials to repository", saveErr, map[string]string{
			"external_service": "mercadolivre",
			"endpoint":        "saveCredentials",
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

var (
	mlProductIDRegex  = regexp.MustCompile(`MLB-?(\d+)`)
	mlProductIDRegex2 = regexp.MustCompile(`/p/(MLB\d+)`)
	mlProductIDRegex3 = regexp.MustCompile(`mercadolivre\.com\.br/[^/]+/p/(MLB\d+)`)
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
