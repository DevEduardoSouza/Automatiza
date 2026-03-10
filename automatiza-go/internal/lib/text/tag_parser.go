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
}

var discountTagRegex = regexp.MustCompile(`desc\[([^\]]+)\]`)

// ParseDiscountTag extracts a discount tag from the message body.
// Supported formats:
//   - desc[25%|10] → 25% off, max R$10
//   - desc[25%]    → 25% off
//   - desc[10]     → R$10 fixed off
//
// Returns nil if no tag is found.
func ParseDiscountTag(body string) *DiscountTag {
	match := discountTagRegex.FindStringSubmatch(body)
	if match == nil {
		return nil
	}

	inner := strings.TrimSpace(match[1])
	tag := &DiscountTag{}

	parts := strings.SplitN(inner, "|", 2)

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
