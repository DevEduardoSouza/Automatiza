// Package auth provides authentication functionality including password hashing,
// session management, token generation, and CSRF protection.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/bcrypt"
)

// Argon2Config holds the parameters for Argon2id password hashing.
// These parameters are chosen to balance security with performance.
type Argon2Config struct {
	Memory      uint32 // Memory in KB (64 * 1024 = 64 MB)
	Iterations  uint32 // Number of iterations (time cost)
	Parallelism uint8  // Parallelism factor
	SaltLength  uint32 // Salt length in bytes
	KeyLength   uint32 // Derived key length in bytes
}

// DefaultArgon2Config provides secure default parameters for Argon2id.
// - 64 MB memory: Provides good resistance against GPU attacks
// - 3 iterations: Balances security with latency
// - 2 parallelism: Uses 2 threads
// - 16 byte salt: 128 bits of randomness
// - 32 byte key: 256-bit hash output
var DefaultArgon2Config = &Argon2Config{
	Memory:      64 * 1024, // 64 MB
	Iterations:  3,
	Parallelism: 2,
	SaltLength:  16,
	KeyLength:   32,
}

var (
	// ErrInvalidHash indicates the hash format is invalid.
	ErrInvalidHash = errors.New("invalid password hash format")
	// ErrIncompatibleVersion indicates the Argon2 version is incompatible.
	ErrIncompatibleVersion = errors.New("incompatible argon2 version")
	// ErrPasswordTooShort indicates the password is too short.
	ErrPasswordTooShort = errors.New("password must be at least 8 characters")
	// ErrPasswordTooLong indicates the password is too long.
	ErrPasswordTooLong = errors.New("password must be at most 128 characters")
	// ErrPasswordNoLowercase indicates the password has no lowercase letter.
	ErrPasswordNoLowercase = errors.New("password must contain at least one lowercase letter")
	// ErrPasswordNoUppercase indicates the password has no uppercase letter.
	ErrPasswordNoUppercase = errors.New("password must contain at least one uppercase letter")
	// ErrPasswordNoDigit indicates the password has no digit.
	ErrPasswordNoDigit = errors.New("password must contain at least one digit")
	// ErrUnsupportedHashFormat indicates an unsupported hash format.
	ErrUnsupportedHashFormat = errors.New("unsupported hash format")
)

// ValidatePasswordComplexity checks that the password meets complexity requirements:
// - At least 8 characters
// - At most 128 characters
// - Contains at least one lowercase letter
// - Contains at least one uppercase letter
// - Contains at least one digit
func ValidatePasswordComplexity(password string) error {
	if len(password) < 8 {
		return ErrPasswordTooShort
	}
	if len(password) > 128 {
		return ErrPasswordTooLong
	}

	var hasLower, hasUpper, hasDigit bool
	for _, r := range password {
		switch {
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsDigit(r):
			hasDigit = true
		}
	}

	if !hasLower {
		return ErrPasswordNoLowercase
	}
	if !hasUpper {
		return ErrPasswordNoUppercase
	}
	if !hasDigit {
		return ErrPasswordNoDigit
	}

	return nil
}

// HashPassword creates an Argon2id hash of the password using the default configuration.
// The hash is encoded in a format that includes all parameters for verification.
// Format: $argon2id$v=19$m=65536,t=3,p=2$<salt>$<hash>
func HashPassword(password string) (string, error) {
	return HashPasswordWithConfig(password, DefaultArgon2Config)
}

// HashPasswordWithConfig creates an Argon2id hash using the provided configuration.
func HashPasswordWithConfig(password string, cfg *Argon2Config) (string, error) {
	// Generate a cryptographically secure salt
	salt := make([]byte, cfg.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("failed to generate salt: %w", err)
	}

	// Hash the password using Argon2id
	hash := argon2.IDKey(
		[]byte(password),
		salt,
		cfg.Iterations,
		cfg.Memory,
		cfg.Parallelism,
		cfg.KeyLength,
	)

	// Encode the hash in PHC string format
	// $argon2id$v=19$m=65536,t=3,p=2$<base64 salt>$<base64 hash>
	encodedHash := fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		cfg.Memory,
		cfg.Iterations,
		cfg.Parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	)

	return encodedHash, nil
}

// VerifyPasswordResult contains the result of password verification.
type VerifyPasswordResult struct {
	// Valid indicates if the password matches the hash.
	Valid bool
	// NeedsMigration indicates if the hash should be migrated to Argon2id.
	// This is true for bcrypt hashes from legacy Supabase.
	NeedsMigration bool
}

// VerifyPassword checks if the provided password matches the hash.
// Returns true if the password is correct, false otherwise.
// Supports both Argon2id ($argon2id$) and bcrypt ($2a$, $2b$) hash formats.
// For bcrypt hashes, NeedsMigration will be true to indicate the hash
// should be upgraded to Argon2id on next opportunity.
// Uses constant-time comparison to prevent timing attacks.
func VerifyPassword(password, encodedHash string) (bool, error) {
	result, err := VerifyPasswordWithMigration(password, encodedHash)
	if err != nil {
		return false, err
	}
	return result.Valid, nil
}

// VerifyPasswordWithMigration checks if the provided password matches the hash
// and returns whether the hash needs migration to Argon2id.
// This is the preferred function for login flows where hash migration is desired.
func VerifyPasswordWithMigration(password, encodedHash string) (*VerifyPasswordResult, error) {
	// Check for bcrypt hash (legacy Supabase format)
	if strings.HasPrefix(encodedHash, "$2a$") || strings.HasPrefix(encodedHash, "$2b$") {
		err := bcrypt.CompareHashAndPassword([]byte(encodedHash), []byte(password))
		if err != nil {
			if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
				return &VerifyPasswordResult{Valid: false, NeedsMigration: false}, nil
			}
			return nil, fmt.Errorf("bcrypt verification failed: %w", err)
		}
		return &VerifyPasswordResult{Valid: true, NeedsMigration: true}, nil
	}

	// Check for Argon2id hash
	if strings.HasPrefix(encodedHash, "$argon2id$") {
		valid, err := verifyArgon2id(password, encodedHash)
		if err != nil {
			return nil, err
		}
		return &VerifyPasswordResult{Valid: valid, NeedsMigration: false}, nil
	}

	return nil, ErrUnsupportedHashFormat
}

// verifyArgon2id verifies an Argon2id password hash.
func verifyArgon2id(password, encodedHash string) (bool, error) {
	// Parse the hash parameters
	cfg, salt, hash, err := decodeHash(encodedHash)
	if err != nil {
		return false, err
	}

	// Compute the hash of the provided password
	computedHash := argon2.IDKey(
		[]byte(password),
		salt,
		cfg.Iterations,
		cfg.Memory,
		cfg.Parallelism,
		cfg.KeyLength,
	)

	// Use constant-time comparison to prevent timing attacks
	if subtle.ConstantTimeCompare(hash, computedHash) == 1 {
		return true, nil
	}

	return false, nil
}

// decodeHash parses the encoded hash string and extracts the configuration,
// salt, and hash values.
func decodeHash(encodedHash string) (*Argon2Config, []byte, []byte, error) {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 {
		return nil, nil, nil, ErrInvalidHash
	}

	// Verify algorithm
	if parts[1] != "argon2id" {
		return nil, nil, nil, ErrInvalidHash
	}

	// Parse version
	var version int
	_, err := fmt.Sscanf(parts[2], "v=%d", &version)
	if err != nil {
		return nil, nil, nil, ErrInvalidHash
	}
	if version != argon2.Version {
		return nil, nil, nil, ErrIncompatibleVersion
	}

	// Parse parameters
	cfg := &Argon2Config{}
	_, err = fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &cfg.Memory, &cfg.Iterations, &cfg.Parallelism)
	if err != nil {
		return nil, nil, nil, ErrInvalidHash
	}

	// Decode salt
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return nil, nil, nil, ErrInvalidHash
	}

	// Decode hash
	hash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return nil, nil, nil, ErrInvalidHash
	}

	cfg.SaltLength = uint32(len(salt))
	cfg.KeyLength = uint32(len(hash))

	return cfg, salt, hash, nil
}

// NeedsRehash checks if the hash should be updated because parameters changed.
// This allows for upgrading hash parameters over time while maintaining
// compatibility with existing hashes.
func NeedsRehash(encodedHash string) bool {
	cfg, _, _, err := decodeHash(encodedHash)
	if err != nil {
		return true
	}

	return cfg.Memory != DefaultArgon2Config.Memory ||
		cfg.Iterations != DefaultArgon2Config.Iterations ||
		cfg.Parallelism != DefaultArgon2Config.Parallelism
}
