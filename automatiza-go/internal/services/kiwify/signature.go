// Package kiwify provides the Kiwify webhook processing service.
// This file implements HMAC-SHA256 signature validation for Kiwify webhooks.
//
// Security considerations:
// - Uses constant-time comparison (crypto/subtle) to prevent timing attacks
// - Never logs secret keys or actual signatures
// - Returns generic error messages to clients to avoid information leakage
package kiwify

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
)

// SignatureValidationError represents an error during signature validation.
// The error message is intentionally generic to avoid leaking information.
type SignatureValidationError struct {
	// Reason is the internal reason (for logging), not exposed to clients.
	Reason string
}

// Error returns a generic error message suitable for client responses.
// This prevents information leakage about the validation failure reason.
func (e *SignatureValidationError) Error() string {
	return "webhook signature validation failed"
}

// Common validation errors with internal reasons for logging.
var (
	// ErrSignatureMismatch indicates the computed signature does not match.
	ErrSignatureMismatch = &SignatureValidationError{Reason: "signature mismatch"}

	// ErrSignatureInvalid indicates the signature format is invalid.
	ErrSignatureInvalid = &SignatureValidationError{Reason: "invalid signature format"}

	// ErrSignatureValidationDisabled indicates validation was skipped (empty secret).
	ErrSignatureValidationDisabled = errors.New("signature validation disabled: empty secret key")
)

// KiwifySignatureValidator validates Kiwify webhook signatures using HMAC-SHA256.
// It implements constant-time comparison to prevent timing attacks.
type KiwifySignatureValidator struct {
	secretKey []byte
	enabled   bool
	strict    bool
}

// SignatureValidatorConfig holds configuration for the signature validator.
type SignatureValidatorConfig struct {
	// SecretKey is the HMAC secret key from Kiwify.
	SecretKey string

	// Strict mode rejects requests with invalid signatures.
	// When false, invalid signatures are logged but the request is processed.
	Strict bool
}

// NewKiwifySignatureValidator creates a new Kiwify signature validator.
// If secretKey is empty, validation is disabled and Validate() will skip checks.
// The caller should log a warning if validation is disabled in production.
func NewKiwifySignatureValidator(cfg *SignatureValidatorConfig) *KiwifySignatureValidator {
	if cfg == nil {
		cfg = &SignatureValidatorConfig{}
	}

	return &KiwifySignatureValidator{
		secretKey: []byte(cfg.SecretKey),
		enabled:   cfg.SecretKey != "",
		strict:    cfg.Strict,
	}
}

// ValidateKiwifySignature validates a Kiwify webhook signature.
// This is a standalone function for simple use cases.
//
// body: The raw request body bytes
// signature: The signature from the request header
// secret: The HMAC secret key
//
// Returns true if the signature is valid.
func ValidateKiwifySignature(body []byte, signature, secret string) bool {
	if secret == "" || signature == "" {
		return false
	}

	// Compute expected signature
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	// Use constant-time comparison to prevent timing attacks
	return subtle.ConstantTimeCompare([]byte(signature), []byte(expectedSig)) == 1
}

// Validate checks if the provided signature matches the HMAC-SHA256 of the body.
// Returns nil if the signature is valid or if validation is disabled.
// Returns an error if the signature is invalid or malformed.
//
// The signature parameter should be a hex-encoded string of the HMAC-SHA256 hash.
// Kiwify sends this in the "signature" header.
//
// Security: Uses constant-time comparison to prevent timing attacks.
func (v *KiwifySignatureValidator) Validate(body []byte, signature string) error {
	// If validation is disabled, skip the check
	if !v.enabled {
		return nil
	}

	// If no signature provided and strict mode is off, allow
	if signature == "" {
		if !v.strict {
			return nil
		}
		return ErrSignatureInvalid
	}

	// Compute the expected signature
	expectedSignature := v.ComputeSignature(body)

	// Decode the provided signature from hex
	providedSigBytes, err := hex.DecodeString(signature)
	if err != nil {
		// Invalid hex format - return generic error
		if v.strict {
			return ErrSignatureInvalid
		}
		return nil
	}

	// Decode our expected signature for comparison
	expectedSigBytes, err := hex.DecodeString(expectedSignature)
	if err != nil {
		// This should never happen since we control the computation
		return ErrSignatureInvalid
	}

	// Use constant-time comparison to prevent timing attacks
	if !hmac.Equal(providedSigBytes, expectedSigBytes) {
		if v.strict {
			return ErrSignatureMismatch
		}
		return nil
	}

	return nil
}

// ComputeSignature computes the HMAC-SHA256 signature for the given body.
// Returns the signature as a lowercase hex-encoded string.
// This is useful for testing and debugging.
func (v *KiwifySignatureValidator) ComputeSignature(body []byte) string {
	mac := hmac.New(sha256.New, v.secretKey)
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// IsEnabled returns whether signature validation is enabled.
// Validation is enabled when the secret key is non-empty.
func (v *KiwifySignatureValidator) IsEnabled() bool {
	return v.enabled
}

// IsStrict returns whether strict mode is enabled.
// In strict mode, invalid signatures result in rejected requests.
func (v *KiwifySignatureValidator) IsStrict() bool {
	return v.strict
}

// ValidationResult contains the result of a signature validation attempt.
// This is useful for structured logging without exposing sensitive data.
type ValidationResult struct {
	// Valid indicates if the signature was valid.
	Valid bool

	// Enabled indicates if validation was enabled.
	Enabled bool

	// Strict indicates if strict mode is enabled.
	Strict bool

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
//	logger.Info("kiwify webhook validation",
//	    zap.Bool("valid", result.Valid),
//	    zap.Bool("enabled", result.Enabled),
//	    zap.Bool("strict", result.Strict),
//	    zap.String("reason", result.Reason),
//	)
func (v *KiwifySignatureValidator) ValidateWithResult(body []byte, signature string) ValidationResult {
	if !v.enabled {
		return ValidationResult{
			Valid:   true, // Considered valid when disabled (no check performed)
			Enabled: false,
			Strict:  v.strict,
			Reason:  "validation disabled",
		}
	}

	if signature == "" {
		return ValidationResult{
			Valid:   !v.strict,
			Enabled: true,
			Strict:  v.strict,
			Reason:  "no signature provided",
		}
	}

	err := v.Validate(body, signature)
	if err == nil {
		return ValidationResult{
			Valid:   true,
			Enabled: true,
			Strict:  v.strict,
			Reason:  "signature verified",
		}
	}

	// Extract internal reason for logging
	var sigErr *SignatureValidationError
	if errors.As(err, &sigErr) {
		return ValidationResult{
			Valid:   false,
			Enabled: true,
			Strict:  v.strict,
			Reason:  sigErr.Reason,
		}
	}

	return ValidationResult{
		Valid:   false,
		Enabled: true,
		Strict:  v.strict,
		Reason:  "unknown error",
	}
}

// MustValidate is a convenience function that panics on validation failure.
// This should only be used in tests.
func (v *KiwifySignatureValidator) MustValidate(body []byte, signature string) {
	if err := v.Validate(body, signature); err != nil {
		panic(fmt.Sprintf("kiwify signature validation failed: %v", err))
	}
}
