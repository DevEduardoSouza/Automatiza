package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"sync"

	"github.com/google/uuid"
)

// tokenHMACKey holds the server-side secret key for HMAC operations.
// This MUST be set via InitTokenHMAC before using HashToken functions.
var (
	tokenHMACKey   []byte
	tokenHMACKeyMu sync.RWMutex
)

// InitTokenHMAC initializes the HMAC key for token hashing.
// The key should be at least 32 bytes (256 bits) for security.
// This MUST be called at application startup with a secret from config.
func InitTokenHMAC(key []byte) error {
	if len(key) < 32 {
		return fmt.Errorf("HMAC key must be at least 32 bytes, got %d", len(key))
	}
	tokenHMACKeyMu.Lock()
	defer tokenHMACKeyMu.Unlock()
	tokenHMACKey = make([]byte, len(key))
	copy(tokenHMACKey, key)
	return nil
}

// getHMACKey returns the current HMAC key.
func getHMACKey() []byte {
	tokenHMACKeyMu.RLock()
	defer tokenHMACKeyMu.RUnlock()
	return tokenHMACKey
}

const (
	// SessionIDBytes is the number of random bytes for session IDs (256 bits).
	SessionIDBytes = 32

	// RefreshTokenBytes is the number of random bytes for refresh tokens (256 bits).
	RefreshTokenBytes = 32

	// CSRFTokenBytes is the number of random bytes for CSRF tokens (256 bits).
	CSRFTokenBytes = 32

	// VerificationTokenBytes is the number of random bytes for email verification tokens.
	VerificationTokenBytes = 32

	// ResetTokenBytes is the number of random bytes for password reset tokens.
	ResetTokenBytes = 32

	// MagicLinkTokenBytes is the number of random bytes for magic link tokens.
	MagicLinkTokenBytes = 32
)

// GenerateSessionID generates a cryptographically secure random session ID.
// Returns a UUID v4 string compatible with the PostgreSQL UUID column type.
func GenerateSessionID() (string, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return "", fmt.Errorf("failed to generate session UUID: %w", err)
	}
	return id.String(), nil
}

// GenerateRefreshToken generates a cryptographically secure random refresh token.
// Returns a hex-encoded string of 64 characters (256 bits).
func GenerateRefreshToken() (string, error) {
	return generateSecureToken(RefreshTokenBytes)
}

// GenerateCSRFToken generates a cryptographically secure random CSRF token.
// Returns a hex-encoded string of 64 characters (256 bits).
func GenerateCSRFToken() (string, error) {
	return generateSecureToken(CSRFTokenBytes)
}

// GenerateVerificationToken generates a cryptographically secure random token
// for email verification.
func GenerateVerificationToken() (string, error) {
	return generateSecureToken(VerificationTokenBytes)
}

// GenerateResetToken generates a cryptographically secure random token
// for password reset.
func GenerateResetToken() (string, error) {
	return generateSecureToken(ResetTokenBytes)
}

// GenerateMagicLinkToken generates a cryptographically secure random token
// for passwordless authentication via magic link.
func GenerateMagicLinkToken() (string, error) {
	return generateSecureToken(MagicLinkTokenBytes)
}

// generateSecureToken generates a cryptographically secure random token
// of the specified byte length, returned as a hex-encoded string.
func generateSecureToken(byteLength int) (string, error) {
	bytes := make([]byte, byteLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate secure token: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

// HashToken creates an HMAC-SHA256 hash of the token for secure storage.
// Uses a server-side secret key to prevent rainbow table attacks.
// The plain token should never be stored; only the hash is persisted.
// IMPORTANT: InitTokenHMAC must be called before using this function.
func HashToken(token string) string {
	key := getHMACKey()
	if len(key) == 0 {
		// Fallback to plain SHA-256 if HMAC key not initialized (for backward compatibility)
		// This should only happen during testing or if InitTokenHMAC was not called
		hash := sha256.Sum256([]byte(token))
		return hex.EncodeToString(hash[:])
	}

	h := hmac.New(sha256.New, key)
	h.Write([]byte(token))
	return hex.EncodeToString(h.Sum(nil))
}

// HashTokenLegacy creates a plain SHA-256 hash of the token.
// DEPRECATED: Use HashToken with HMAC for new code.
// This is only for verifying old tokens that were hashed without HMAC.
func HashTokenLegacy(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// ConstantTimeCompare performs a constant-time comparison of two strings.
// This prevents timing attacks when comparing sensitive values like tokens.
// Returns true if the strings are equal.
func ConstantTimeCompare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// ConstantTimeCompareHash compares a plain token with a stored hash.
// This hashes the plain token and performs constant-time comparison with the stored hash.
// Returns true if the token matches the hash.
func ConstantTimeCompareHash(plainToken, storedHash string) bool {
	computedHash := HashToken(plainToken)
	return ConstantTimeCompare(computedHash, storedHash)
}

// ValidateTokenFormat checks if a token has the expected format (hex-encoded, proper length).
func ValidateTokenFormat(token string, expectedBytes int) bool {
	if len(token) != expectedBytes*2 {
		return false
	}
	_, err := hex.DecodeString(token)
	return err == nil
}
