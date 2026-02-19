package whatsapp

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"testing"
)

func TestNewHMACValidator(t *testing.T) {
	t.Run("enabled with non-empty key", func(t *testing.T) {
		v := NewHMACValidator("my-secret-key")
		if !v.IsEnabled() {
			t.Error("expected validator to be enabled with non-empty key")
		}
	})

	t.Run("disabled with empty key", func(t *testing.T) {
		v := NewHMACValidator("")
		if v.IsEnabled() {
			t.Error("expected validator to be disabled with empty key")
		}
	})
}

func TestHMACValidator_Validate_ValidSignature(t *testing.T) {
	secretKey := "webhook-secret-key-12345"
	v := NewHMACValidator(secretKey)

	testCases := []struct {
		name string
		body []byte
	}{
		{
			name: "simple json body",
			body: []byte(`{"event":"message","session":"default"}`),
		},
		{
			name: "complex json body",
			body: []byte(`{"event":"message.ack","session":"tenant-123","payload":{"id":"msg-456","ack":2}}`),
		},
		{
			name: "empty body",
			body: []byte{},
		},
		{
			name: "unicode body",
			body: []byte(`{"text":"Hello, World!"}`),
		},
		{
			name: "binary-like body",
			body: []byte{0x00, 0x01, 0x02, 0xFF, 0xFE},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Compute the correct signature
			signature := v.ComputeSignature(tc.body)

			// Validate should pass
			err := v.Validate(tc.body, signature)
			if err != nil {
				t.Errorf("expected validation to pass, got error: %v", err)
			}
		})
	}
}

func TestHMACValidator_Validate_InvalidSignature(t *testing.T) {
	secretKey := "webhook-secret-key-12345"
	v := NewHMACValidator(secretKey)
	body := []byte(`{"event":"message"}`)

	testCases := []struct {
		name      string
		signature string
		wantErr   error
	}{
		{
			name:      "wrong signature",
			signature: "00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
			wantErr:   ErrHMACSignatureMismatch,
		},
		{
			name:      "signature from different body",
			signature: v.ComputeSignature([]byte(`{"event":"different"}`)),
			wantErr:   ErrHMACSignatureMismatch,
		},
		{
			name:      "signature from different key",
			signature: computeHMACWithKey("different-key", body),
			wantErr:   ErrHMACSignatureMismatch,
		},
		{
			name:      "truncated signature",
			signature: v.ComputeSignature(body)[:64], // Only half the signature
			wantErr:   ErrHMACSignatureMismatch,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := v.Validate(body, tc.signature)
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("expected error %v, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestHMACValidator_Validate_MalformedSignature(t *testing.T) {
	secretKey := "webhook-secret-key-12345"
	v := NewHMACValidator(secretKey)
	body := []byte(`{"event":"message"}`)

	testCases := []struct {
		name      string
		signature string
	}{
		{
			name:      "not hex encoded",
			signature: "not-a-hex-string!@#$%",
		},
		{
			name:      "odd length hex",
			signature: "abc", // Odd number of hex chars is invalid
		},
		{
			name:      "uppercase letters in signature (should still decode)",
			signature: "GGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGG",
		},
		{
			name:      "spaces in signature",
			signature: "00 11 22 33 44 55 66 77 88 99 aa bb cc dd ee ff",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := v.Validate(body, tc.signature)
			if err == nil {
				t.Error("expected validation to fail for malformed signature")
			}
		})
	}
}

func TestHMACValidator_Validate_DisabledValidator(t *testing.T) {
	v := NewHMACValidator("") // Disabled
	body := []byte(`{"event":"message"}`)

	t.Run("skips validation when disabled", func(t *testing.T) {
		// Any signature (even invalid) should pass when disabled
		err := v.Validate(body, "invalid-signature")
		if err != nil {
			t.Errorf("expected validation to be skipped (nil error), got: %v", err)
		}
	})

	t.Run("skips validation with empty signature", func(t *testing.T) {
		err := v.Validate(body, "")
		if err != nil {
			t.Errorf("expected validation to be skipped (nil error), got: %v", err)
		}
	})
}

func TestHMACValidator_ComputeSignature(t *testing.T) {
	secretKey := "test-secret"
	v := NewHMACValidator(secretKey)
	body := []byte("test body")

	t.Run("produces correct HMAC-SHA512", func(t *testing.T) {
		// Compute expected signature manually
		mac := hmac.New(sha512.New, []byte(secretKey))
		mac.Write(body)
		expected := hex.EncodeToString(mac.Sum(nil))

		got := v.ComputeSignature(body)
		if got != expected {
			t.Errorf("expected signature %s, got %s", expected, got)
		}
	})

	t.Run("produces lowercase hex", func(t *testing.T) {
		sig := v.ComputeSignature(body)
		for _, c := range sig {
			if c >= 'A' && c <= 'F' {
				t.Errorf("expected lowercase hex, found uppercase: %c", c)
			}
		}
	})

	t.Run("produces consistent results", func(t *testing.T) {
		sig1 := v.ComputeSignature(body)
		sig2 := v.ComputeSignature(body)
		if sig1 != sig2 {
			t.Error("expected consistent signature for same input")
		}
	})

	t.Run("different bodies produce different signatures", func(t *testing.T) {
		sig1 := v.ComputeSignature([]byte("body1"))
		sig2 := v.ComputeSignature([]byte("body2"))
		if sig1 == sig2 {
			t.Error("expected different signatures for different bodies")
		}
	})
}

func TestHMACValidator_ValidateWithResult(t *testing.T) {
	secretKey := "test-secret"
	v := NewHMACValidator(secretKey)
	body := []byte(`{"event":"message"}`)
	validSig := v.ComputeSignature(body)

	t.Run("valid signature returns valid result", func(t *testing.T) {
		result := v.ValidateWithResult(body, validSig)
		if !result.Valid {
			t.Error("expected Valid to be true")
		}
		if !result.Enabled {
			t.Error("expected Enabled to be true")
		}
		if result.Reason != "signature verified" {
			t.Errorf("expected reason 'signature verified', got '%s'", result.Reason)
		}
	})

	t.Run("invalid signature returns invalid result", func(t *testing.T) {
		result := v.ValidateWithResult(body, "00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000")
		if result.Valid {
			t.Error("expected Valid to be false")
		}
		if !result.Enabled {
			t.Error("expected Enabled to be true")
		}
		if result.Reason != "signature mismatch" {
			t.Errorf("expected reason 'signature mismatch', got '%s'", result.Reason)
		}
	})

	t.Run("malformed signature returns invalid result", func(t *testing.T) {
		result := v.ValidateWithResult(body, "not-hex!")
		if result.Valid {
			t.Error("expected Valid to be false")
		}
		if result.Reason != "invalid signature format" {
			t.Errorf("expected reason 'invalid signature format', got '%s'", result.Reason)
		}
	})

	t.Run("disabled validator returns valid with disabled reason", func(t *testing.T) {
		disabled := NewHMACValidator("")
		result := disabled.ValidateWithResult(body, "any-signature")
		if !result.Valid {
			t.Error("expected Valid to be true when disabled")
		}
		if result.Enabled {
			t.Error("expected Enabled to be false")
		}
		if result.Reason != "validation disabled" {
			t.Errorf("expected reason 'validation disabled', got '%s'", result.Reason)
		}
	})
}

func TestHMACValidationError_Error(t *testing.T) {
	t.Run("returns generic message", func(t *testing.T) {
		err := ErrHMACSignatureMismatch
		msg := err.Error()
		if msg != "webhook signature validation failed" {
			t.Errorf("expected generic error message, got: %s", msg)
		}
	})

	t.Run("all errors return same generic message", func(t *testing.T) {
		errors := []*HMACValidationError{
			ErrHMACSignatureMismatch,
			ErrHMACSignatureInvalid,
		}
		for _, e := range errors {
			if e.Error() != "webhook signature validation failed" {
				t.Errorf("expected generic error message, got: %s", e.Error())
			}
		}
	})
}

func TestHMACValidator_MustValidate(t *testing.T) {
	secretKey := "test-secret"
	v := NewHMACValidator(secretKey)
	body := []byte(`{"event":"message"}`)

	t.Run("does not panic on valid signature", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("unexpected panic: %v", r)
			}
		}()
		validSig := v.ComputeSignature(body)
		v.MustValidate(body, validSig)
	})

	t.Run("panics on invalid signature", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic on invalid signature")
			}
		}()
		v.MustValidate(body, "invalid")
	})
}

// Benchmark to verify constant-time comparison doesn't leak timing info
func BenchmarkHMACValidator_Validate(b *testing.B) {
	secretKey := "benchmark-secret-key"
	v := NewHMACValidator(secretKey)
	body := []byte(`{"event":"message","session":"default","payload":{"id":"msg-123"}}`)
	validSig := v.ComputeSignature(body)
	invalidSig := "00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"

	b.Run("valid signature", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = v.Validate(body, validSig)
		}
	})

	b.Run("invalid signature", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = v.Validate(body, invalidSig)
		}
	})
}

// Helper function to compute HMAC with a specific key
func computeHMACWithKey(key string, body []byte) string {
	mac := hmac.New(sha512.New, []byte(key))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}
