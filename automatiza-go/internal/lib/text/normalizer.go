package text

import (
	"regexp"
	"strings"
)

var (
	// Zero-width characters
	invisibleCharsRegex = regexp.MustCompile(`[\x{200B}-\x{200F}\x{202A}-\x{202E}\x{2060}\x{FEFF}]`)

	// Excessive blank lines
	excessiveLinesRegex = regexp.MustCompile(`\n{3,}`)

	// Multiple spaces
	multipleSpacesRegex = regexp.MustCompile(`[ \t]{2,}`)
)

// NormalizeOutgoingWhatsAppText normalizes text for WhatsApp sending
func NormalizeOutgoingWhatsAppText(input string) string {
	// 1. Remove invisible characters
	result := invisibleCharsRegex.ReplaceAllString(input, "")

	// 2. Fix carriage returns
	result = strings.ReplaceAll(result, "\r\n", "\n")
	result = strings.ReplaceAll(result, "\r", "\n")

	// 3. Reduce excessive blank lines to max 2
	result = excessiveLinesRegex.ReplaceAllString(result, "\n\n")

	// 4. Trim whitespace from each line
	lines := strings.Split(result, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(line)
	}
	result = strings.Join(lines, "\n")

	// 5. Final trim
	result = strings.TrimSpace(result)

	return result
}

// IsEffectivelyEmptyWhatsAppText checks if text is effectively empty after normalization
func IsEffectivelyEmptyWhatsAppText(input string) bool {
	normalized := NormalizeOutgoingWhatsAppText(input)
	return normalized == ""
}

// CleanWhatsAppNumber cleans a phone number for WhatsApp
func CleanWhatsAppNumber(phone string) string {
	// Remove all non-numeric characters
	var result strings.Builder
	for _, r := range phone {
		if r >= '0' && r <= '9' {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// FormatWhatsAppJID formats a phone number as a WhatsApp JID
func FormatWhatsAppJID(phone string) string {
	cleaned := CleanWhatsAppNumber(phone)
	if cleaned == "" {
		return ""
	}
	return cleaned + "@s.whatsapp.net"
}

// FormatWhatsAppGroupJID formats a group ID as a WhatsApp group JID
func FormatWhatsAppGroupJID(groupID string) string {
	if strings.HasSuffix(groupID, "@g.us") {
		return groupID
	}
	return groupID + "@g.us"
}

// IsGroupJID checks if a JID is a group JID
func IsGroupJID(jid string) bool {
	return strings.HasSuffix(jid, "@g.us")
}

// IsUserJID checks if a JID is a user JID
func IsUserJID(jid string) bool {
	return strings.HasSuffix(jid, "@s.whatsapp.net")
}

// ExtractPhoneFromJID extracts the phone number from a JID
func ExtractPhoneFromJID(jid string) string {
	if strings.Contains(jid, "@") {
		return strings.Split(jid, "@")[0]
	}
	return jid
}

// TruncateText truncates text to a maximum length, adding ellipsis if needed
func TruncateText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	if maxLen <= 3 {
		return text[:maxLen]
	}
	return text[:maxLen-3] + "..."
}

// RemoveEmojis removes emoji characters from text
func RemoveEmojis(text string) string {
	// Basic emoji ranges
	emojiRegex := regexp.MustCompile(`[\x{1F600}-\x{1F64F}]|[\x{1F300}-\x{1F5FF}]|[\x{1F680}-\x{1F6FF}]|[\x{1F1E0}-\x{1F1FF}]|[\x{2702}-\x{27B0}]|[\x{24C2}-\x{1F251}]`)
	return emojiRegex.ReplaceAllString(text, "")
}

// CollapseWhitespace collapses multiple whitespace characters into single spaces
func CollapseWhitespace(text string) string {
	result := multipleSpacesRegex.ReplaceAllString(text, " ")
	result = strings.ReplaceAll(result, "\n ", "\n")
	result = strings.ReplaceAll(result, " \n", "\n")
	return strings.TrimSpace(result)
}
