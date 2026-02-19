// Package whatsapp provides the WhatsApp adapter abstraction layer.
// This file implements HMAC-SHA512 validation for securing WAHA webhooks.
//
// Security considerations:
// - Uses constant-time comparison (hmac.Equal) to prevent timing attacks
// - Never logs secret keys or actual signatures
// - Returns generic error messages to clients to avoid information leakage
package whatsapp

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"fmt"
)

// HMACValidationError represents an error during HMAC validation.
// The error message is intentionally generic to avoid leaking information.
type HMACValidationError struct {
	// Reason is the internal reason (for logging), not exposed to clients.
	Reason string
}

// Error returns a generic error message suitable for client responses.
// This prevents information leakage about the validation failure reason.
func (e *HMACValidationError) Error() string {
	return "webhook signature validation failed"
}

// Common validation errors with internal reasons for logging.
var (
	// ErrHMACSignatureMismatch indicates the computed signature does not match.
	ErrHMACSignatureMismatch = &HMACValidationError{Reason: "signature mismatch"}

	// ErrHMACSignatureInvalid indicates the signature format is invalid.
	ErrHMACSignatureInvalid = &HMACValidationError{Reason: "invalid signature format"}

	// ErrHMACValidationDisabled indicates validation was skipped (empty key).
	ErrHMACValidationDisabled = errors.New("hmac validation disabled: empty secret key")
)

// HMACValidator validates webhook signatures using HMAC-SHA512.
// It implements constant-time comparison to prevent timing attacks.
type HMACValidator struct {
	secretKey []byte
	enabled   bool
}

// NewHMACValidator creates a new HMAC validator.
// If secretKey is empty, validation is disabled and Validate() will skip checks.
// The caller should log a warning if validation is disabled in production.
func NewHMACValidator(secretKey string) *HMACValidator {
	return &HMACValidator{
		secretKey: []byte(secretKey),
		enabled:   secretKey != "",
	}
}

// Validate checks if the provided signature matches the HMAC-SHA512 of the body.
// Returns nil if the signature is valid or if validation is disabled.
// Returns an error if the signature is invalid or malformed.
//
// The signature parameter should be a hex-encoded string of the HMAC-SHA512 hash.
// WAHA sends this in the X-Webhook-Hmac header.
//
// Security: Uses hmac.Equal for constant-time comparison to prevent timing attacks.
func (v *HMACValidator) Validate(body []byte, signature string) error {
	// If validation is disabled, skip the check but return a specific error
	// so the caller can log appropriately
	if !v.enabled {
		return nil
	}

	// Compute the expected signature
	expectedSignature := v.ComputeSignature(body)

	// Decode the provided signature from hex
	providedSigBytes, err := hex.DecodeString(signature)
	if err != nil {
		// Invalid hex format - return generic error
		return ErrHMACSignatureInvalid
	}

	// Decode our expected signature for comparison
	expectedSigBytes, err := hex.DecodeString(expectedSignature)
	if err != nil {
		// This should never happen since we control the computation
		return ErrHMACSignatureInvalid
	}

	// Use constant-time comparison to prevent timing attacks
	if !hmac.Equal(providedSigBytes, expectedSigBytes) {
		return ErrHMACSignatureMismatch
	}

	return nil
}

// ComputeSignature computes the HMAC-SHA512 signature for the given body.
// Returns the signature as a lowercase hex-encoded string.
// This is useful for testing and debugging.
func (v *HMACValidator) ComputeSignature(body []byte) string {
	mac := hmac.New(sha512.New, v.secretKey)
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// IsEnabled returns whether HMAC validation is enabled.
// Validation is enabled when the secret key is non-empty.
func (v *HMACValidator) IsEnabled() bool {
	return v.enabled
}

// ValidationResult contains the result of an HMAC validation attempt.
// This is useful for structured logging without exposing sensitive data.
type ValidationResult struct {
	// Valid indicates if the signature was valid.
	Valid bool

	// Enabled indicates if validation was enabled.
	Enabled bool

	// Reason provides context for the result (for logging).
	// Never contains the actual signature or secret key.
	Reason string
}

// ValidateWithResult validates the signature and returns a detailed result.
// This is useful when you need structured logging of validation attempts.
//
// Example usage:
//
//	result := validator.ValidateWithResult(body, sig)
//	logger.Info("webhook validation",
//	    zap.Bool("valid", result.Valid),
//	    zap.Bool("enabled", result.Enabled),
//	    zap.String("reason", result.Reason),
//	)
func (v *HMACValidator) ValidateWithResult(body []byte, signature string) ValidationResult {
	if !v.enabled {
		return ValidationResult{
			Valid:   true, // Considered valid when disabled (no check performed)
			Enabled: false,
			Reason:  "validation disabled",
		}
	}

	err := v.Validate(body, signature)
	if err == nil {
		return ValidationResult{
			Valid:   true,
			Enabled: true,
			Reason:  "signature verified",
		}
	}

	// Extract internal reason for logging
	var hmacErr *HMACValidationError
	if errors.As(err, &hmacErr) {
		return ValidationResult{
			Valid:   false,
			Enabled: true,
			Reason:  hmacErr.Reason,
		}
	}

	return ValidationResult{
		Valid:   false,
		Enabled: true,
		Reason:  "unknown error",
	}
}

// MustValidate is a convenience function that panics on validation failure.
// This should only be used in tests.
func (v *HMACValidator) MustValidate(body []byte, signature string) {
	if err := v.Validate(body, signature); err != nil {
		panic(fmt.Sprintf("hmac validation failed: %v", err))
	}
}
