package text

import (
	"automatiza-go/internal/models"
	"regexp"
	"strings"
)

var (
	// Phone number regex (Brazilian format)
	phoneRegex = regexp.MustCompile(`\+?55\s?\d{2}\s?\d{4,5}[\s-]?\d{4}|\d{10,11}`)

	// WhatsApp mention regex
	mentionRegex = regexp.MustCompile(`@[\d]+`)
)

// ProcessarMensagem processes a message according to monitoring text rules
func ProcessarMensagem(text string, rules *models.MonitoringTextRules) string {
	if rules == nil {
		return text
	}

	result := text

	// 1. Apply substitutions
	for _, sub := range rules.Substituicoes {
		result = strings.ReplaceAll(result, sub.De, sub.Para)
	}

	// 2. Remove terms
	for _, termo := range rules.RemoverTermos {
		// Try as regex first
		if re, err := regexp.Compile(termo); err == nil {
			result = re.ReplaceAllString(result, "")
		} else {
			result = strings.ReplaceAll(result, termo, "")
		}
	}

	// 3. Clean phones
	if rules.LimparTelefones {
		result = phoneRegex.ReplaceAllString(result, "")
	}

	// 4. Clean mentions
	if rules.LimparArrobas {
		result = mentionRegex.ReplaceAllString(result, "")
	}

	// 5. Normalize whitespace
	result = strings.TrimSpace(result)
	result = regexp.MustCompile(`[ \t]+`).ReplaceAllString(result, " ")
	result = regexp.MustCompile(`\n[ \t]+`).ReplaceAllString(result, "\n")
	result = regexp.MustCompile(`[ \t]+\n`).ReplaceAllString(result, "\n")

	// 6. Add prefix/suffix
	if rules.Prefixo != "" {
		result = rules.Prefixo + "\n" + result
	}
	if rules.Sufixo != "" {
		result = result + "\n" + rules.Sufixo
	}

	return strings.TrimSpace(result)
}

// ExtractLinks extracts HTTP/HTTPS links from text.
//
// The regex accepts parentheses and WhatsApp formatting characters
// (`*`, `_`, `~`, backtick) inside URLs because real-world product URLs
// contain them heavily: Shopee slugs with model codes like "(LES11)",
// Wikipedia-style slugs with underscores, query params like "display_model_id",
// and so on. These characters are stripped from the trailing edge of the
// captured URL instead — handling both WhatsApp formatting wrappers (e.g.
// "_https://foo.com_" for italic) and markdown-wrap parens (e.g.
// "(https://foo/bar)").
func ExtractLinks(text string) []string {
	linkRegex := regexp.MustCompile(`https?://[^\s\n\r<>"\[\]]+`)
	matches := linkRegex.FindAllString(text, -1)

	// Deduplicate
	seen := make(map[string]bool)
	unique := make([]string, 0, len(matches))
	for _, link := range matches {
		// Strip trailing punctuation and WhatsApp formatting delimiters
		// (`*`, `_`, `~`, backtick) so the captured URL does not leak the
		// wrapper characters when a user writes something like
		// "_https://example.com_" to italicize the link.
		link = strings.TrimRight(link, ".,;:!?*_~\x60")
		link = trimUnbalancedTrailingParens(link)
		if !seen[link] {
			seen[link] = true
			unique = append(unique, link)
		}
	}

	return unique
}

// trimUnbalancedTrailingParens strips trailing ")" characters that have no
// matching "(" earlier in the string. This allows URLs with balanced parens
// (e.g. "https://en.wikipedia.org/wiki/Foo_(bar)") to pass through unchanged
// while removing the markdown-wrap ")" from patterns like "(https://foo/bar)".
func trimUnbalancedTrailingParens(s string) string {
	for strings.HasSuffix(s, ")") {
		open := strings.Count(s, "(")
		closeP := strings.Count(s, ")")
		if closeP <= open {
			break
		}
		s = s[:len(s)-1]
	}
	return s
}

// ExtractShopeeLinks extracts Shopee links from text
func ExtractShopeeLinks(text string) []string {
	links := ExtractLinks(text)
	shopeeLinks := make([]string, 0)

	for _, link := range links {
		if IsShopeeLink(link) {
			shopeeLinks = append(shopeeLinks, link)
		}
	}

	return shopeeLinks
}

// ExtractMLLinks extracts Mercado Livre links from text
func ExtractMLLinks(text string) []string {
	links := ExtractLinks(text)
	mlLinks := make([]string, 0)

	for _, link := range links {
		if IsMLLink(link) {
			mlLinks = append(mlLinks, link)
		}
	}

	return mlLinks
}

// IsShopeeLink checks if a URL is a Shopee link
func IsShopeeLink(url string) bool {
	lowerURL := strings.ToLower(url)
	return strings.Contains(lowerURL, "shopee.com.br") ||
		strings.Contains(lowerURL, "s.shopee.com.br") ||
		strings.Contains(lowerURL, "shp.ee")
}

// IsMLLink checks if a URL is a Mercado Livre link, including the
// official short-link domain meli.la.
func IsMLLink(url string) bool {
	lowerURL := strings.ToLower(url)
	return strings.Contains(lowerURL, "mercadolivre.com.br") ||
		strings.Contains(lowerURL, "mercadolibre.com") ||
		strings.Contains(lowerURL, "ml.com.br") ||
		strings.Contains(lowerURL, "meli.la")
}

// ReplaceLinksInText replaces links in text with new links
func ReplaceLinksInText(text string, oldToNew map[string]string) string {
	result := text
	for old, new := range oldToNew {
		result = strings.ReplaceAll(result, old, new)
	}
	return result
}

// ContentFingerprint generates a fingerprint for content deduplication
func ContentFingerprint(text string, links []string) string {
	// Normalize text
	normalized := strings.ToLower(text)
	normalized = regexp.MustCompile(`\s+`).ReplaceAllString(normalized, " ")
	normalized = strings.TrimSpace(normalized)

	// Include sorted links
	if len(links) > 0 {
		normalized = normalized + "|" + strings.Join(links, ",")
	}

	return normalized
}
