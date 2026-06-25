package text

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

// DiscountTag represents a parsed discount tag from a message body.
type DiscountTag struct {
	Percent    float64 // Percentage discount (e.g., 25 means 25%)
	FixedValue float64 // Fixed value discount in BRL
	MaxValue   float64 // Maximum discount value in BRL (only with percent)
	CouponCode string  // Optional coupon code (e.g., desc[10:EXEMPLO10])
}

var discountTagRegex = regexp.MustCompile(`desc\[([^\]]+)\]`)

// infoTagRegex captures the contents of an info[...] tag. The body may
// contain spaces and URLs (e.g. "info[APLIQUE O CUPOM https://...]"); the
// only forbidden character is `]` since it closes the tag.
var infoTagRegex = regexp.MustCompile(`info\[([^\]]*)\]`)

// ctaTagRegex captures the contents of a cta[...] tag. Same shape as the
// info tag — free-form text (may contain spaces/emoji/URLs), only `]` is
// forbidden since it closes the tag.
var ctaTagRegex = regexp.MustCompile(`cta\[([^\]]*)\]`)

// ParseInfoTag extracts the contents of an info[...] tag from the message
// body and returns the trimmed text. Returns "" when no tag is present or
// when the tag is empty.
//
// The info tag is used by /offer-generate as a free-form line that gets
// injected into the dispatch template via the {info} placeholder — typical
// use is appending a coupon code or extra observation that varies per send
// without editing the template.
func ParseInfoTag(body string) string {
	match := infoTagRegex.FindStringSubmatch(body)
	if match == nil {
		return ""
	}
	return strings.TrimSpace(match[1])
}

// StripInfoTag removes every info[...] occurrence from the body. Callers
// run this before link extraction so URLs embedded inside an info tag (the
// canonical example is a coupon link) are not mistaken for product links.
func StripInfoTag(body string) string {
	return strings.TrimSpace(infoTagRegex.ReplaceAllString(body, ""))
}

// ParseCTATag extracts the contents of a cta[...] tag from the message body
// and returns the trimmed text. Returns "" when no tag is present or when the
// tag is empty.
//
// The cta tag is used by /offer-generate as a per-send override: when present
// in the trigger message it replaces the automatically picked CTA for that
// single dispatch, feeding the {cta} placeholder directly without touching the
// configured CTA pool.
func ParseCTATag(body string) string {
	match := ctaTagRegex.FindStringSubmatch(body)
	if match == nil {
		return ""
	}
	return strings.TrimSpace(match[1])
}

// StripCTATag removes every cta[...] occurrence from the body. Callers run
// this before link extraction so URLs embedded inside a cta tag are not
// mistaken for product links.
func StripCTATag(body string) string {
	return strings.TrimSpace(ctaTagRegex.ReplaceAllString(body, ""))
}

// ParseDiscountTag extracts a discount tag from the message body.
// Supported formats:
//   - desc[25%|10]        → 25% off, max R$10
//   - desc[25%]           → 25% off
//   - desc[10]            → R$10 fixed off
//   - desc[10:CUPOM10]    → R$10 fixed off + coupon code CUPOM10
//   - desc[25%|10:CUPOM]  → 25% off, max R$10 + coupon code CUPOM
//   - desc[25%:CUPOM]     → 25% off + coupon code CUPOM
//
// Returns nil if no tag is found.
func ParseDiscountTag(body string) *DiscountTag {
	match := discountTagRegex.FindStringSubmatch(body)
	if match == nil {
		return nil
	}

	inner := strings.TrimSpace(match[1])
	tag := &DiscountTag{}

	// Extract coupon code after ":"
	discountPart := inner
	if idx := strings.LastIndex(inner, ":"); idx >= 0 {
		code := strings.TrimSpace(inner[idx+1:])
		if code != "" {
			tag.CouponCode = code
		}
		discountPart = strings.TrimSpace(inner[:idx])
	}

	parts := strings.SplitN(discountPart, "|", 2)

	first := strings.TrimSpace(parts[0])
	if strings.HasSuffix(first, "%") {
		val, err := strconv.ParseFloat(strings.TrimSuffix(first, "%"), 64)
		if err == nil && val > 0 {
			tag.Percent = val
		}
	} else {
		val, err := strconv.ParseFloat(first, 64)
		if err == nil && val > 0 {
			tag.FixedValue = val
		}
	}

	if len(parts) == 2 {
		maxVal, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if err == nil && maxVal > 0 {
			tag.MaxValue = maxVal
		}
	}

	// Return nil if nothing was parsed
	if tag.Percent == 0 && tag.FixedValue == 0 {
		return nil
	}

	return tag
}

// StripDiscountTag removes the discount tag from the message body.
func StripDiscountTag(body string) string {
	return strings.TrimSpace(discountTagRegex.ReplaceAllString(body, ""))
}

// ApplyDiscountTag calculates the discounted price based on the tag.
// Returns the original price if tag is nil.
func ApplyDiscountTag(price float64, tag *DiscountTag) float64 {
	if tag == nil || price <= 0 {
		return price
	}

	if tag.FixedValue > 0 {
		result := price - tag.FixedValue
		if result < 0 {
			return 0
		}
		return math.Round(result*100) / 100
	}

	if tag.Percent > 0 {
		discount := price * (tag.Percent / 100)
		if tag.MaxValue > 0 && discount > tag.MaxValue {
			discount = tag.MaxValue
		}
		result := price - discount
		if result < 0 {
			return 0
		}
		return math.Round(result*100) / 100
	}

	return price
}
