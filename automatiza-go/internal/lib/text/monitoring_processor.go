package text

import (
	"automatiza-go/internal/models"
	"regexp"
	"strings"
	"unicode"
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

	// 1. Apply substitutions.
	//    Exact match first (preserves historical behavior). If the exact string
	//    is not present, fall back to a forgiving match that ignores WhatsApp
	//    formatting (*_~), collapsed whitespace/newlines and case — so a client
	//    can paste the footer exactly as they SEE it and it still matches, even
	//    though the raw message carries invisible bold markers and line breaks.
	for _, sub := range rules.Substituicoes {
		if strings.Contains(result, sub.De) {
			result = strings.ReplaceAll(result, sub.De, sub.Para)
		} else {
			result = replaceTolerant(result, sub.De, sub.Para)
		}
	}

	// 2. Remove terms.
	//    Regex/exact first (historical behavior); if that removed nothing, retry
	//    with the same forgiving match used for substitutions. This is what lets
	//    a non-technical user delete a channel footer by pasting the visible text
	//    without knowing regex or reproducing the WhatsApp bold markers.
	for _, termo := range rules.RemoverTermos {
		if termo == "" {
			continue
		}
		before := result
		if re, err := regexp.Compile(termo); err == nil {
			result = re.ReplaceAllString(result, "")
		} else {
			result = strings.ReplaceAll(result, termo, "")
		}
		if result == before {
			result = replaceTolerant(result, termo, "")
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

// replaceTolerant replaces every occurrence of `from` inside `text` with `to`,
// matching in a formatting-insensitive way: both sides are normalized (WhatsApp
// bold/italic delimiters *_~` stripped, runs of whitespace collapsed to a single
// space, lower-cased) before comparing. The replacement is written back onto the
// ORIGINAL text (via a rune→byte offset map) so untouched parts of the message
// keep their original formatting, emojis and spacing.
//
// This is what makes footer removal usable for non-technical users: they paste
// the footer as it appears on screen ("💖 Nos siga no instagram @canal ... link")
// and it matches even though the raw inbound message contains "*Nos siga*",
// double spaces and hard line breaks the user never sees. Passing to == "" turns
// it into a removal.
func replaceTolerant(text, from, to string) string {
	fromRunes, _ := normalizeForMatch(from)
	fromRunes = trimSpaceRunes(fromRunes)
	if len(fromRunes) == 0 {
		return text
	}

	textRunes, offsets := normalizeForMatch(text)
	if len(fromRunes) > len(textRunes) {
		return text
	}

	var b strings.Builder
	lastOrigEnd := 0
	i := 0
	for i+len(fromRunes) <= len(textRunes) {
		if runesEqualAt(textRunes, fromRunes, i) {
			origStart := offsets[i]
			origEnd := offsets[i+len(fromRunes)]
			// Absorb WhatsApp formatting delimiters (* _ ~ `) left orphaned right
			// up against the match boundary — e.g. a footer where only part is
			// bold ("*Nos siga* @canal ...") would otherwise leak a stray "*".
			for origStart > lastOrigEnd && isFormatDelim(text[origStart-1]) {
				origStart--
			}
			for origEnd < len(text) && isFormatDelim(text[origEnd]) {
				origEnd++
			}
			b.WriteString(text[lastOrigEnd:origStart])
			b.WriteString(to)
			lastOrigEnd = origEnd
			i += len(fromRunes)
		} else {
			i++
		}
	}
	b.WriteString(text[lastOrigEnd:])
	return b.String()
}

// isFormatDelim reports whether b is a WhatsApp inline-formatting delimiter.
func isFormatDelim(b byte) bool {
	return b == '*' || b == '_' || b == '~' || b == '`'
}

// normalizeForMatch returns the normalized rune slice of s together with a
// parallel offset slice. offsets[k] is the byte index in the ORIGINAL string
// where the k-th normalized rune began; a final sentinel offsets[len] == len(s)
// lets callers compute the end byte offset of the last matched rune.
//
// Normalization: drop WhatsApp formatting delimiters (* _ ~ `), collapse any run
// of whitespace (spaces, tabs, newlines) into a single ASCII space, and
// lower-case everything.
func normalizeForMatch(s string) ([]rune, []int) {
	runes := make([]rune, 0, len(s))
	offsets := make([]int, 0, len(s)+1)
	lastWasSpace := false
	for i, r := range s {
		switch r {
		case '*', '_', '~', '`':
			continue
		}
		if unicode.IsSpace(r) {
			if lastWasSpace {
				continue
			}
			runes = append(runes, ' ')
			offsets = append(offsets, i)
			lastWasSpace = true
			continue
		}
		runes = append(runes, unicode.ToLower(r))
		offsets = append(offsets, i)
		lastWasSpace = false
	}
	offsets = append(offsets, len(s))
	return runes, offsets
}

// trimSpaceRunes strips leading/trailing single-space runes left by
// normalizeForMatch so a pasted term with surrounding whitespace still matches.
func trimSpaceRunes(r []rune) []rune {
	start := 0
	for start < len(r) && r[start] == ' ' {
		start++
	}
	end := len(r)
	for end > start && r[end-1] == ' ' {
		end--
	}
	return r[start:end]
}

// runesEqualAt reports whether needle occurs in hay starting at index i.
func runesEqualAt(hay, needle []rune, i int) bool {
	for j := range needle {
		if hay[i+j] != needle[j] {
			return false
		}
	}
	return true
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
		// URLs de produto são ASCII puro; emoji colado no link (ex.:
		// "https://s.shopee.com.br/qem5OVT0i‼️") entrava na captura, a
		// expansão falhava e o repasse era retido. Corta no primeiro rune
		// não-ASCII antes dos trims.
		link = CutURLAtNonASCII(link)
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

// CutURLAtNonASCII trunca uma URL capturada no primeiro rune não-ASCII.
// URLs válidas (RFC 3986) são ASCII — qualquer coisa fora disso colada no
// link (emoji, ‼️, variation selector) é texto vizinho que o regex de
// captura engoliu, não parte da URL.
func CutURLAtNonASCII(s string) string {
	for i, r := range s {
		if r > unicode.MaxASCII {
			return s[:i]
		}
	}
	return s
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
