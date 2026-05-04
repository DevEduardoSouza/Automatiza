// Package urlvalidation provides URL validation and normalization utilities.
package urlvalidation

import (
	"fmt"
	"net/url"
	"strings"

	apperrors "automatiza-go/internal/errors"
)

// ValidateMercadoLivreURL validates that a URL is a valid Mercado Livre product URL.
// Valid URLs must:
// - Be from mercadolivre.com.br domain (or related subdomains)
// - Contain /p/ (product page) or mlb (product ID prefix)
//
// Example valid URLs:
// - https://www.mercadolivre.com.br/p/MLB12345678
// - https://produto.mercadolivre.com.br/MLB-12345678
// - https://mercadolivre.com.br/algum-produto-p/MLB12345678
func ValidateMercadoLivreURL(rawURL string) error {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return apperrors.NewMLLinksFieldError(
			apperrors.ErrInvalidURL.Error(),
			"URL cannot be empty",
			"origin_url",
			apperrors.ErrInvalidURL,
		)
	}

	u, err := url.Parse(trimmed)
	if err != nil {
		return apperrors.NewMLLinksFieldError(
			apperrors.ErrInvalidURLFormat.Error(),
			fmt.Sprintf("invalid URL format: %v", err),
			"origin_url",
			apperrors.ErrInvalidURLFormat,
		)
	}

	host := strings.ToLower(u.Hostname())
	if !strings.Contains(host, "mercadolivre.") && !strings.Contains(host, "mercadolibre.") {
		return apperrors.NewMLLinksFieldError(
			apperrors.ErrInvalidURL.Error(),
			"URL must be from mercadolivre.com.br domain",
			"origin_url",
			apperrors.ErrInvalidURL,
		)
	}

	path := strings.ToLower(u.Path)
	fullURL := strings.ToLower(rawURL)

	// Check for product indicators: /p/ path segment or MLB product ID
	hasProductPath := strings.Contains(path, "/p/")
	hasMLBCode := strings.Contains(fullURL, "mlb")

	if !hasProductPath && !hasMLBCode {
		return apperrors.NewMLLinksFieldError(
			apperrors.ErrInvalidURL.Error(),
			"URL must be a product page (contain /p/ or mlb)",
			"origin_url",
			apperrors.ErrInvalidURL,
		)
	}

	return nil
}

// ValidateProductURL validates that a URL is a valid product URL from a supported platform.
// Supported platforms: Mercado Livre, Shopee
func ValidateProductURL(rawURL string) error {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return apperrors.NewMLLinksFieldError(
			apperrors.ErrInvalidURL.Error(),
			"URL cannot be empty",
			"origin_url",
			apperrors.ErrInvalidURL,
		)
	}

	u, err := url.Parse(trimmed)
	if err != nil {
		return apperrors.NewMLLinksFieldError(
			apperrors.ErrInvalidURLFormat.Error(),
			fmt.Sprintf("invalid URL format: %v", err),
			"origin_url",
			apperrors.ErrInvalidURLFormat,
		)
	}

	host := strings.ToLower(u.Hostname())

	// Check Mercado Livre (full domain or official short link "meli.la")
	isMLShort := host == "meli.la"
	isML := isMLShort || strings.Contains(host, "mercadolivre.") || strings.Contains(host, "mercadolibre.")
	// Check Shopee
	isShopee := strings.Contains(host, "shopee.com") || strings.Contains(host, "shp.ee")

	if !isML && !isShopee {
		return apperrors.NewMLLinksFieldError(
			apperrors.ErrInvalidURL.Error(),
			"URL must be from Mercado Livre or Shopee",
			"origin_url",
			apperrors.ErrInvalidURL,
		)
	}

	// Short links (meli.la) and Shopee URLs skip the product-path check —
	// the destination is only known after following the redirect.
	if isML && !isMLShort {
		path := strings.ToLower(u.Path)
		fullURL := strings.ToLower(rawURL)
		hasProductPath := strings.Contains(path, "/p/")
		hasMLBCode := strings.Contains(fullURL, "mlb")
		if !hasProductPath && !hasMLBCode {
			return apperrors.NewMLLinksFieldError(
				apperrors.ErrInvalidURL.Error(),
				"URL must be a product page (contain /p/ or mlb)",
				"origin_url",
				apperrors.ErrInvalidURL,
			)
		}
	}

	return nil
}

// NormalizeURL removes the hash fragment from a URL and trims whitespace.
// This ensures URLs can be properly compared for deduplication.
//
// Example:
// - Input:  "https://mercadolivre.com.br/produto#reviews"
// - Output: "https://mercadolivre.com.br/produto"
func NormalizeURL(rawURL string) (string, error) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return "", apperrors.NewMLLinksFieldError(
			apperrors.ErrInvalidURLFormat.Error(),
			"URL cannot be empty",
			"origin_url",
			apperrors.ErrInvalidURLFormat,
		)
	}

	u, err := url.Parse(trimmed)
	if err != nil {
		return "", apperrors.NewMLLinksFieldError(
			apperrors.ErrInvalidURLFormat.Error(),
			fmt.Sprintf("invalid URL format: %v", err),
			"origin_url",
			apperrors.ErrInvalidURLFormat,
		)
	}

	// Remove fragment (hash)
	u.Fragment = ""

	return u.String(), nil
}

// DeduplicateURLs removes duplicate URLs from a slice.
// URLs are normalized before comparison to ensure proper deduplication.
// Returns unique URLs in original order (first occurrence kept).
//
// Example:
// - Input:  ["url1", "url2", "url1#hash", "url2"]
// - Output: ["url1", "url2"]
func DeduplicateURLs(urls []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(urls))

	for _, rawURL := range urls {
		normalized, err := NormalizeURL(rawURL)
		if err != nil {
			// Skip invalid URLs during deduplication
			continue
		}

		if !seen[normalized] {
			seen[normalized] = true
			result = append(result, rawURL) // Keep original URL (not normalized)
		}
	}

	return result
}

// ValidateAndNormalizeURL validates and normalizes a URL in a single operation.
// Returns the normalized URL if valid, or an error if invalid.
func ValidateAndNormalizeURL(rawURL string) (string, error) {
	if err := ValidateProductURL(rawURL); err != nil {
		return "", err
	}
	return NormalizeURL(rawURL)
}

// FilterValidURLs filters a slice of URLs, returning only valid product URLs.
// Invalid URLs are skipped silently.
func FilterValidURLs(urls []string) []string {
	result := make([]string, 0, len(urls))

	for _, rawURL := range urls {
		if err := ValidateProductURL(rawURL); err == nil {
			result = append(result, rawURL)
		}
	}

	return result
}

// ValidateURLs validates multiple URLs and returns detailed results.
// Returns a map of URL to error (nil if valid) for each input URL.
func ValidateURLs(urls []string) map[string]error {
	results := make(map[string]error, len(urls))

	for _, rawURL := range urls {
		results[rawURL] = ValidateProductURL(rawURL)
	}

	return results
}

// ValidationResult contains the results of bulk URL validation.
type ValidationResult struct {
	ValidURLs   []string          // URLs that passed validation
	InvalidURLs map[string]string // URL -> error message mapping
}

// ValidateAndCategorizeURLs validates URLs and categorizes them into valid and invalid.
func ValidateAndCategorizeURLs(urls []string) *ValidationResult {
	result := &ValidationResult{
		ValidURLs:   make([]string, 0, len(urls)),
		InvalidURLs: make(map[string]string),
	}

	for _, rawURL := range urls {
		if err := ValidateProductURL(rawURL); err != nil {
			result.InvalidURLs[rawURL] = err.Error()
		} else {
			result.ValidURLs = append(result.ValidURLs, rawURL)
		}
	}

	return result
}
