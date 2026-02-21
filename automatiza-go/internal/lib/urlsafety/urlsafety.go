package urlsafety

import (
	"automatiza-go/internal/lib/cache"
	"automatiza-go/internal/observability"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.uber.org/zap"
)

// UrlSafetyDecision represents the decision for a message
type UrlSafetyDecision struct {
	Action string // "allow", "redact", "block"
	Reason string
	URLs   []URLInfo
}

// URLInfo represents information about a URL
type URLInfo struct {
	Original  string
	Final     string
	FinalHost string
	IsBlocked bool
	Reason    string
}

// Service provides URL safety checking
type Service struct {
	mode                 string // "off", "redact", "block"
	blockedHosts         map[string]bool
	allowedHosts         map[string]bool
	enforceAllowlist     bool
	trustedRedirectHosts map[string]bool
	maxRedirects         int
	redirectTimeout      time.Duration
	cache                *cache.Cache
	logger               *zap.Logger
	httpClient           *http.Client
}

// Config for URL safety service
type Config struct {
	Mode             string
	BlockedHosts     []string
	AllowedHosts     []string
	EnforceAllowlist bool
}

// NewService creates a new URL safety service
func NewService(cfg Config, cache *cache.Cache, logger *zap.Logger) *Service {
	blockedHosts := make(map[string]bool)
	for _, h := range cfg.BlockedHosts {
		blockedHosts[strings.ToLower(strings.TrimSpace(h))] = true
	}

	allowedHosts := make(map[string]bool)
	for _, h := range cfg.AllowedHosts {
		allowedHosts[strings.ToLower(strings.TrimSpace(h))] = true
	}

	// Default trusted shorteners
	trustedRedirectHosts := map[string]bool{
		"s.shopee.com.br":  true,
		"shp.ee":           true,
		"bit.ly":           true,
		"tinyurl.com":      true,
		"t.co":             true,
		"amzn.to":          true,
		"mercadolivre.com": true,
	}

	return &Service{
		mode:                 cfg.Mode,
		blockedHosts:         blockedHosts,
		allowedHosts:         allowedHosts,
		enforceAllowlist:     cfg.EnforceAllowlist,
		trustedRedirectHosts: trustedRedirectHosts,
		maxRedirects:         5,
		redirectTimeout:      10 * time.Second,
		cache:                cache,
		logger:               logger,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
			Transport: otelhttp.NewTransport(http.DefaultTransport),
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return http.ErrUseLastResponse
				}
				return nil
			},
		},
	}
}

// Evaluate evaluates a message for URL safety
func (s *Service) Evaluate(ctx context.Context, message string) (*UrlSafetyDecision, error) {
	if s.mode == "off" {
		return &UrlSafetyDecision{Action: "allow"}, nil
	}

	urls := s.extractHTTPURLs(message)
	if len(urls) == 0 {
		return &UrlSafetyDecision{Action: "allow"}, nil
	}

	decision := &UrlSafetyDecision{
		Action: "allow",
		URLs:   make([]URLInfo, 0, len(urls)),
	}

	for _, urlStr := range urls {
		info := s.evaluateURL(ctx, urlStr)
		decision.URLs = append(decision.URLs, info)

		if info.IsBlocked {
			decision.Action = s.mode // "redact" or "block"
			decision.Reason = info.Reason
		}
	}

	return decision, nil
}

func (s *Service) evaluateURL(ctx context.Context, originalURL string) URLInfo {
	info := URLInfo{Original: originalURL}

	// Parse URL
	parsed, err := url.Parse(originalURL)
	if err != nil {
		info.IsBlocked = true
		info.Reason = "URL inválida"
		return info
	}

	host := strings.ToLower(parsed.Host)

	// Check if trusted shortener - resolve redirect
	if s.isTrustedRedirectHost(host) {
		finalURL, finalHost, err := s.resolveRedirect(ctx, originalURL)
		if err == nil {
			info.Final = finalURL
			info.FinalHost = finalHost
			host = finalHost
		} else {
			// BG10: Fail-open with Sentry Warning — URL allowed through despite check failure
			observability.LogAndCapture(s.logger, sentry.LevelWarning,
				"URL safety fail-open: redirect resolution failed, allowing original URL",
				fmt.Errorf("resolve redirect for %s: %w", originalURL, err),
				map[string]string{
					"module":   "url-safety",
					"flow":     "fail-open",
					"original": originalURL,
				})
			// Continue with original host
			info.Final = originalURL
			info.FinalHost = host
		}
	} else {
		info.Final = originalURL
		info.FinalHost = host
	}

	// Check blocked hosts
	if s.isBlockedHost(host) {
		info.IsBlocked = true
		info.Reason = "Host bloqueado: " + host
		return info
	}

	// Check allowlist enforcement
	if s.enforceAllowlist && !s.isAllowedHost(host) {
		info.IsBlocked = true
		info.Reason = "Host não permitido: " + host
		return info
	}

	return info
}

func (s *Service) isTrustedRedirectHost(host string) bool {
	host = strings.ToLower(host)
	return s.trustedRedirectHosts[host]
}

func (s *Service) isBlockedHost(host string) bool {
	host = strings.ToLower(host)

	// Direct match
	if s.blockedHosts[host] {
		return true
	}

	// Check parent domains
	parts := strings.Split(host, ".")
	for i := 1; i < len(parts); i++ {
		parentDomain := strings.Join(parts[i:], ".")
		if s.blockedHosts[parentDomain] {
			return true
		}
	}

	return false
}

func (s *Service) isAllowedHost(host string) bool {
	host = strings.ToLower(host)

	// Direct match
	if s.allowedHosts[host] {
		return true
	}

	// Check parent domains
	parts := strings.Split(host, ".")
	for i := 1; i < len(parts); i++ {
		parentDomain := strings.Join(parts[i:], ".")
		if s.allowedHosts[parentDomain] {
			return true
		}
	}

	return false
}

func (s *Service) resolveRedirect(ctx context.Context, originalURL string) (string, string, error) {
	// Check cache first
	cacheKey := "urlsafety:redirect:" + originalURL
	var cached struct {
		FinalURL  string `json:"finalUrl"`
		FinalHost string `json:"finalHost"`
	}

	if s.cache != nil {
		if err := s.cache.GetJSON(ctx, cacheKey, &cached); err == nil {
			return cached.FinalURL, cached.FinalHost, nil
		}
	}

	// Make HEAD request to follow redirects
	req, err := http.NewRequestWithContext(ctx, "HEAD", originalURL, nil)
	if err != nil {
		return "", "", err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	finalURL := resp.Request.URL.String()
	finalHost := strings.ToLower(resp.Request.URL.Host)

	// Cache the result for 1 hour
	if s.cache != nil {
		_ = s.cache.SetJSON(ctx, cacheKey, map[string]string{
			"finalUrl":  finalURL,
			"finalHost": finalHost,
		}, time.Hour)
	}

	return finalURL, finalHost, nil
}

var urlRegex = regexp.MustCompile(`https?://[^\s\n\r*_~\x60<>"\[\]()]+`)

func (s *Service) extractHTTPURLs(text string) []string {
	matches := urlRegex.FindAllString(text, -1)

	// Clean and deduplicate
	seen := make(map[string]bool)
	result := make([]string, 0, len(matches))

	for _, match := range matches {
		// Clean trailing punctuation
		match = strings.TrimRight(match, ".,;:!?")
		if !seen[match] {
			seen[match] = true
			result = append(result, match)
		}
	}

	return result
}

// RedactBlockedURLs replaces blocked URLs with a placeholder
func (s *Service) RedactBlockedURLs(message string, decision *UrlSafetyDecision) string {
	result := message

	for _, urlInfo := range decision.URLs {
		if urlInfo.IsBlocked {
			result = strings.ReplaceAll(result, urlInfo.Original, "[link removido]")
		}
	}

	return result
}
