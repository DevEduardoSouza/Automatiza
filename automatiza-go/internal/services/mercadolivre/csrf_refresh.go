package mercadolivre

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

const (
	MLLinkbuilderURL = "https://www.mercadolivre.com.br/afiliados/linkbuilder"
)

// CSRFRefresher handles automatic CSRF token refresh for Mercado Livre
type CSRFRefresher struct {
	httpClient *http.Client
}

// NewCSRFRefresher creates a new CSRF refresher
func NewCSRFRefresher(client *http.Client) *CSRFRefresher {
	return &CSRFRefresher{httpClient: client}
}

// csrfPatterns contains regex patterns to extract CSRF token from HTML
// These patterns match the Node.js implementation exactly for parity
var csrfMetaPatterns = []*regexp.Regexp{
	// Meta tag patterns
	regexp.MustCompile(`<meta[^>]+name=["']csrf-token["'][^>]+content=["']([^"']+)["']`),
	regexp.MustCompile(`<meta[^>]+name=["']x-csrf-token["'][^>]+content=["']([^"']+)["']`),
	regexp.MustCompile(`<meta[^>]+property=["']csrf-token["'][^>]+content=["']([^"']+)["']`),
}

var csrfJSONPatterns = []*regexp.Regexp{
	// JSON patterns in JavaScript/data
	regexp.MustCompile(`"x-csrf-token"\s*:\s*"([^"]+)"`),
	regexp.MustCompile(`"csrfToken"\s*:\s*"([^"]+)"`),
	regexp.MustCompile(`"csrf_token"\s*:\s*"([^"]+)"`),
}

// ExtractCSRFFromHTML extracts CSRF token from HTML content
// Returns empty string if not found
func ExtractCSRFFromHTML(html string) string {
	if html == "" {
		return ""
	}

	// Try meta tag patterns first (most reliable)
	for _, pattern := range csrfMetaPatterns {
		if matches := pattern.FindStringSubmatch(html); len(matches) > 1 {
			token := strings.TrimSpace(matches[1])
			if token != "" {
				return token
			}
		}
	}

	// Try JSON patterns
	for _, pattern := range csrfJSONPatterns {
		if matches := pattern.FindStringSubmatch(html); len(matches) > 1 {
			token := strings.TrimSpace(matches[1])
			if token != "" {
				return token
			}
		}
	}

	return ""
}

// RefreshCSRFToken fetches a fresh CSRF token from ML linkbuilder page
func (r *CSRFRefresher) RefreshCSRFToken(ctx context.Context, cookie string) (string, error) {
	if cookie == "" {
		return "", fmt.Errorf("cookie is required for CSRF refresh")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", MLLinkbuilderURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Cookie", cookie)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120 Safari/537.36")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch linkbuilder page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("linkbuilder returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	token := ExtractCSRFFromHTML(string(body))
	if token == "" {
		return "", fmt.Errorf("could not extract CSRF token from page")
	}

	return token, nil
}

// IsCSRFError checks if the HTTP status indicates a CSRF/auth error
func IsCSRFError(statusCode int) bool {
	return statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden
}
