// Package crypto provides cryptographic utilities for the application.
package crypto

import (
	"crypto/rand"
	"math/big"
)

const (
	// DefaultPasswordLength is the default length for generated passwords.
	DefaultPasswordLength = 12

	// Character sets for password generation
	lowercaseChars = "abcdefghijklmnopqrstuvwxyz"
	uppercaseChars = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	digitChars     = "0123456789"
	symbolChars    = "!@#$%&*"
	allChars       = lowercaseChars + uppercaseChars + digitChars + symbolChars
)

// GenerateSecurePassword generates a cryptographically secure random password
// with the specified length. The password is guaranteed to contain at least
// one lowercase letter, one uppercase letter, one digit, and one symbol.
//
// If length is less than 4, it defaults to 12 characters.
func GenerateSecurePassword(length int) (string, error) {
	if length < 4 {
		length = DefaultPasswordLength
	}

	password := make([]byte, length)

	// Ensure at least one character from each required set
	requiredSets := []string{lowercaseChars, uppercaseChars, digitChars, symbolChars}
	positions := make([]int, len(requiredSets))

	// Generate unique random positions for required characters
	usedPositions := make(map[int]bool)
	for i := 0; i < len(requiredSets); i++ {
		for {
			pos, err := secureRandomInt(length)
			if err != nil {
				return "", err
			}
			if !usedPositions[pos] {
				usedPositions[pos] = true
				positions[i] = pos
				break
			}
		}
	}

	// Place required characters at their positions
	for i, charset := range requiredSets {
		char, err := secureRandomChar(charset)
		if err != nil {
			return "", err
		}
		password[positions[i]] = char
	}

	// Fill remaining positions with random characters from all sets
	for i := 0; i < length; i++ {
		if usedPositions[i] {
			continue
		}
		char, err := secureRandomChar(allChars)
		if err != nil {
			return "", err
		}
		password[i] = char
	}

	return string(password), nil
}

// GenerateTemporaryPassword generates a 12-character temporary password
// suitable for sending to users via email.
func GenerateTemporaryPassword() (string, error) {
	return GenerateSecurePassword(DefaultPasswordLength)
}

// secureRandomInt returns a cryptographically secure random integer in [0, max).
func secureRandomInt(max int) (int, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return 0, err
	}
	return int(n.Int64()), nil
}

// secureRandomChar returns a cryptographically secure random character
// from the given character set.
func secureRandomChar(charset string) (byte, error) {
	idx, err := secureRandomInt(len(charset))
	if err != nil {
		return 0, err
	}
	return charset[idx], nil
}
