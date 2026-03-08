package mercadolivre

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
)

// HTML entity decoder
var htmlEntities = map[string]string{
	"&amp;":  "&",
	"&quot;": "\"",
	"&#39;":  "'",
	"&lt;":   "<",
	"&gt;":   ">",
}

func decodeHTMLEntities(raw string) string {
	result := raw
	for entity, char := range htmlEntities {
		result = strings.ReplaceAll(result, entity, char)
	}
	return result
}

// extractMeta extracts meta tag content by property or name
func extractMeta(html, key string) string {
	if html == "" || key == "" {
		return ""
	}

	// Escape special regex characters in key
	escapedKey := regexp.QuoteMeta(key)

	// Try property first, then name
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`<meta[^>]+(?:property|name)=["']` + escapedKey + `["'][^>]*content=["']([^"']*?)["'][^>]*>`),
		regexp.MustCompile(`<meta[^>]+content=["']([^"']*?)["'][^>]*(?:property|name)=["']` + escapedKey + `["'][^>]*>`),
	}

	for _, pattern := range patterns {
		if matches := pattern.FindStringSubmatch(html); len(matches) > 1 {
			return decodeHTMLEntities(strings.TrimSpace(matches[1]))
		}
	}

	return ""
}

// extractTitle extracts product title from HTML
func extractTitle(html string) string {
	// Try og:title first
	if title := extractMeta(html, "og:title"); title != "" {
		return title
	}

	// Try twitter:title
	if title := extractMeta(html, "twitter:title"); title != "" {
		return title
	}

	// Fallback to <title> tag
	titleRegex := regexp.MustCompile(`<title>([^<]+)</title>`)
	if matches := titleRegex.FindStringSubmatch(html); len(matches) > 1 {
		title := decodeHTMLEntities(strings.TrimSpace(matches[1]))
		// Clean up common suffixes
		title = strings.Split(title, " | ")[0]
		title = strings.Split(title, " - Mercado Livre")[0]
		return title
	}

	return ""
}

// mlImageSuffixRegex matches the size suffix in mlstatic.com image URLs.
// Pattern: "-O.webp", "-O.jpg", "-I.webp", "-S.jpg", etc.
var mlImageSuffixRegex = regexp.MustCompile(`-([OISVW])\.(webp|jpg|jpeg|png)$`)

// upscaleMLImageURL replaces the ML image size suffix with "-F" (full ~800px).
// ML CDN uses: -O (original/small ~300px), -I (icon), -S (small),
// -F (full ~800px), -V (very large ~1200px).
func upscaleMLImageURL(url string) string {
	if !strings.Contains(url, "mlstatic.com") {
		return url
	}
	return mlImageSuffixRegex.ReplaceAllString(url, "-F.$2")
}

// extractImage extracts product image from HTML
func extractImage(html string) string {
	// Try og:image first
	if img := extractMeta(html, "og:image"); img != "" {
		return upscaleMLImageURL(img)
	}

	// Try twitter:image
	if img := extractMeta(html, "twitter:image"); img != "" {
		return upscaleMLImageURL(img)
	}

	return ""
}

// parseMoneyAmount parses Brazilian money format (1.234,56 or 1234.56)
func parseMoneyAmount(value string) float64 {
	if value == "" {
		return 0
	}

	// Remove R$, spaces
	cleaned := strings.TrimSpace(value)
	cleaned = strings.ReplaceAll(cleaned, "R$", "")
	cleaned = strings.ReplaceAll(cleaned, " ", "")

	// Check if Brazilian format (comma as decimal separator)
	if strings.Contains(cleaned, ",") {
		// Remove thousand separators (dots)
		cleaned = strings.ReplaceAll(cleaned, ".", "")
		// Convert comma to dot
		cleaned = strings.ReplaceAll(cleaned, ",", ".")
	}

	// Remove any remaining non-numeric chars except dot
	var result strings.Builder
	for _, c := range cleaned {
		if (c >= '0' && c <= '9') || c == '.' {
			result.WriteRune(c)
		}
	}

	price, _ := strconv.ParseFloat(result.String(), 64)
	return price
}

// extractPrice extracts product price and currency from HTML
func extractPrice(html string) (price float64, currency string) {
	currency = "BRL" // Default for Brazil

	// Try meta tags first
	priceAmount := extractMeta(html, "product:price:amount")
	if priceAmount == "" {
		priceAmount = extractMeta(html, "og:price:amount")
	}

	priceCurrency := extractMeta(html, "product:price:currency")
	if priceCurrency == "" {
		priceCurrency = extractMeta(html, "og:price:currency")
	}
	if priceCurrency != "" {
		currency = strings.ToUpper(priceCurrency)
	}

	if priceAmount != "" {
		price = parseMoneyAmount(priceAmount)
		if price > 0 {
			return price, currency
		}
	}

	// Try JSON-LD
	ldObjects := extractJSONLDObjects(html)
	for _, obj := range ldObjects {
		if offers, ok := obj["offers"]; ok {
			var offerList []map[string]interface{}
			switch v := offers.(type) {
			case map[string]interface{}:
				offerList = []map[string]interface{}{v}
			case []interface{}:
				for _, item := range v {
					if m, ok := item.(map[string]interface{}); ok {
						offerList = append(offerList, m)
					}
				}
			}

			for _, offer := range offerList {
				if p, ok := offer["price"]; ok {
					switch pv := p.(type) {
					case float64:
						price = pv
					case string:
						price = parseMoneyAmount(pv)
					}
				}
				if p, ok := offer["lowPrice"]; ok && price == 0 {
					switch pv := p.(type) {
					case float64:
						price = pv
					case string:
						price = parseMoneyAmount(pv)
					}
				}
				if c, ok := offer["priceCurrency"].(string); ok && c != "" {
					currency = strings.ToUpper(c)
				}
				if price > 0 {
					return price, currency
				}
			}
		}
	}

	// Fallback: text regex for R$ prices
	priceRegex := regexp.MustCompile(`R\$\s*([\d\.]+,\d{2})`)
	matches := priceRegex.FindAllStringSubmatch(html, -1)
	if len(matches) > 0 {
		// Pick the smallest price (usually current price, not original)
		var minPrice float64
		for _, m := range matches {
			if len(m) > 1 {
				p := parseMoneyAmount(m[1])
				if p > 0 && (minPrice == 0 || p < minPrice) {
					minPrice = p
				}
			}
		}
		if minPrice > 0 {
			return minPrice, currency
		}
	}

	return 0, currency
}

// extractJSONLDObjects extracts JSON-LD objects from HTML
func extractJSONLDObjects(html string) []map[string]interface{} {
	var result []map[string]interface{}

	ldRegex := regexp.MustCompile(`<script[^>]+type=["']application/ld\+json["'][^>]*>([\s\S]*?)</script>`)
	matches := ldRegex.FindAllStringSubmatch(html, -1)

	for _, m := range matches {
		if len(m) > 1 {
			jsonStr := strings.TrimSpace(m[1])
			if jsonStr == "" {
				continue
			}

			// Try parsing as object
			var obj map[string]interface{}
			if err := json.Unmarshal([]byte(jsonStr), &obj); err == nil {
				result = append(result, obj)
				continue
			}

			// Try parsing as array
			var arr []map[string]interface{}
			if err := json.Unmarshal([]byte(jsonStr), &arr); err == nil {
				result = append(result, arr...)
			}
		}
	}

	return result
}

// extractNumberByKey extracts a number from JSON-like text by key
func extractNumberByKey(html, key string) *float64 {
	escapedKey := regexp.QuoteMeta(key)
	pattern := regexp.MustCompile(`"` + escapedKey + `"\s*:\s*([0-9]+(?:\.[0-9]+)?)`)
	if matches := pattern.FindStringSubmatch(html); len(matches) > 1 {
		if n, err := strconv.ParseFloat(matches[1], 64); err == nil {
			return &n
		}
	}
	return nil
}

// extractCondition extracts product condition from HTML
func extractCondition(html string) *string {
	// Try JSON-LD first
	ldObjects := extractJSONLDObjects(html)
	for _, obj := range ldObjects {
		if cond, ok := obj["itemCondition"].(string); ok && cond != "" {
			var result string
			condLower := strings.ToLower(cond)
			if strings.Contains(condLower, "newcondition") {
				result = "Novo"
			} else if strings.Contains(condLower, "usedcondition") {
				result = "Usado"
			} else {
				result = cond
			}
			return &result
		}
	}

	// Try meta tags
	if cond := extractMeta(html, "og:condition"); cond != "" {
		return &cond
	}
	if cond := extractMeta(html, "product:condition"); cond != "" {
		return &cond
	}

	// Try JSON key
	if n := extractNumberByKey(html, "condition"); n != nil {
		result := strconv.FormatFloat(*n, 'f', 0, 64)
		return &result
	}

	return nil
}

// extractSoldQuantity extracts sold quantity from HTML
func extractSoldQuantity(html string) *int {
	// Try JSON keys
	if n := extractNumberByKey(html, "sold_quantity"); n != nil {
		result := int(*n)
		return &result
	}
	if n := extractNumberByKey(html, "soldQuantity"); n != nil {
		result := int(*n)
		return &result
	}

	// Try text pattern: "+50mil vendidos"
	soldRegex := regexp.MustCompile(`\+?\s*([\d.,]+)\s*mil\s+vendidos`)
	if matches := soldRegex.FindStringSubmatch(html); len(matches) > 1 {
		numStr := strings.ReplaceAll(matches[1], ".", "")
		numStr = strings.ReplaceAll(numStr, ",", ".")
		if n, err := strconv.ParseFloat(numStr, 64); err == nil {
			result := int(n * 1000)
			return &result
		}
	}

	// Try simple number pattern
	soldRegex2 := regexp.MustCompile(`(\d+)\s+vendidos`)
	if matches := soldRegex2.FindStringSubmatch(html); len(matches) > 1 {
		if n, err := strconv.Atoi(matches[1]); err == nil {
			return &n
		}
	}

	return nil
}

// extractRating extracts rating average and review count from HTML
func extractRating(html string) (ratingAverage *float64, reviewCount *int) {
	// Try JSON-LD first
	ldObjects := extractJSONLDObjects(html)
	for _, obj := range ldObjects {
		if ar, ok := obj["aggregateRating"].(map[string]interface{}); ok {
			if rv, ok := ar["ratingValue"]; ok {
				switch v := rv.(type) {
				case float64:
					ratingAverage = &v
				case string:
					if n, err := strconv.ParseFloat(v, 64); err == nil {
						ratingAverage = &n
					}
				}
			}
			if rc, ok := ar["reviewCount"]; ok {
				switch v := rc.(type) {
				case float64:
					count := int(v)
					reviewCount = &count
				case string:
					if n, err := strconv.Atoi(v); err == nil {
						reviewCount = &n
					}
				}
			}
			if ratingAverage != nil || reviewCount != nil {
				return
			}
		}
	}

	// Try JSON keys
	if n := extractNumberByKey(html, "rating_average"); n != nil {
		ratingAverage = n
	}
	if n := extractNumberByKey(html, "average_rating"); n != nil && ratingAverage == nil {
		ratingAverage = n
	}
	if n := extractNumberByKey(html, "total_reviews"); n != nil {
		count := int(*n)
		reviewCount = &count
	}
	if n := extractNumberByKey(html, "reviews_total"); n != nil && reviewCount == nil {
		count := int(*n)
		reviewCount = &count
	}
	if n := extractNumberByKey(html, "total_ratings"); n != nil && reviewCount == nil {
		count := int(*n)
		reviewCount = &count
	}

	return
}

// extractOriginalPrice extracts original price (before discount) from HTML
func extractOriginalPrice(html string) *float64 {
	if n := extractNumberByKey(html, "original_price"); n != nil {
		return n
	}
	if n := extractNumberByKey(html, "originalPrice"); n != nil {
		return n
	}
	return nil
}

// extractDiscountPercent extracts discount percentage from HTML
func extractDiscountPercent(html string) *float64 {
	// Try "X% OFF" pattern
	discountRegex := regexp.MustCompile(`(\d{1,2})%\s*OFF`)
	if matches := discountRegex.FindStringSubmatch(html); len(matches) > 1 {
		if n, err := strconv.ParseFloat(matches[1], 64); err == nil {
			return &n
		}
	}
	return nil
}

// Installments represents installment payment info
type Installments struct {
	Quantity int     `json:"quantity"`
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
	Text     string  `json:"text"`
}

// extractInstallments extracts installment info from HTML
func extractInstallments(html string) *Installments {
	// Pattern: "em 3x R$ 33,33"
	installRegex := regexp.MustCompile(`\bem\s*(\d{1,2})x\s*R\$\s*([\d\.,]+)`)
	if matches := installRegex.FindStringSubmatch(html); len(matches) > 2 {
		qty, _ := strconv.Atoi(matches[1])
		amount := parseMoneyAmount(matches[2])
		if qty > 0 && amount > 0 {
			return &Installments{
				Quantity: qty,
				Amount:   amount,
				Currency: "BRL",
				Text:     strings.TrimSpace(matches[0]),
			}
		}
	}

	// Try JSON pattern
	installJSONRegex := regexp.MustCompile(`"installments":\{"quantity":(\d+),"amount":(\d+(?:\.\d+)?)`)
	if matches := installJSONRegex.FindStringSubmatch(html); len(matches) > 2 {
		qty, _ := strconv.Atoi(matches[1])
		amount, _ := strconv.ParseFloat(matches[2], 64)
		if qty > 0 && amount > 0 {
			return &Installments{
				Quantity: qty,
				Amount:   amount,
				Currency: "BRL",
				Text:     "",
			}
		}
	}

	return nil
}
