package kiwify

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestValidateKiwifySignature(t *testing.T) {
	secret := "test-secret-key"
	body := []byte(`{"event":"order_approved","customer":{"email":"test@example.com"}}`)

	// Compute valid signature
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	validSignature := hex.EncodeToString(mac.Sum(nil))

	tests := []struct {
		name      string
		body      []byte
		signature string
		secret    string
		want      bool
	}{
		{
			name:      "valid signature",
			body:      body,
			signature: validSignature,
			secret:    secret,
			want:      true,
		},
		{
			name:      "invalid signature",
			body:      body,
			signature: "invalid-signature",
			secret:    secret,
			want:      false,
		},
		{
			name:      "wrong secret",
			body:      body,
			signature: validSignature,
			secret:    "wrong-secret",
			want:      false,
		},
		{
			name:      "empty secret",
			body:      body,
			signature: validSignature,
			secret:    "",
			want:      false,
		},
		{
			name:      "empty signature",
			body:      body,
			signature: "",
			secret:    secret,
			want:      false,
		},
		{
			name:      "modified body",
			body:      []byte(`{"event":"order_approved","customer":{"email":"modified@example.com"}}`),
			signature: validSignature,
			secret:    secret,
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidateKiwifySignature(tt.body, tt.signature, tt.secret)
			if got != tt.want {
				t.Errorf("ValidateKiwifySignature() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestKiwifySignatureValidator_Validate(t *testing.T) {
	secret := "test-secret-key"
	body := []byte(`{"event":"order_approved"}`)

	// Compute valid signature
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	validSignature := hex.EncodeToString(mac.Sum(nil))

	tests := []struct {
		name      string
		cfg       *SignatureValidatorConfig
		body      []byte
		signature string
		wantErr   bool
	}{
		{
			name:      "valid signature with strict mode",
			cfg:       &SignatureValidatorConfig{SecretKey: secret, Strict: true},
			body:      body,
			signature: validSignature,
			wantErr:   false,
		},
		{
			name:      "invalid signature with strict mode",
			cfg:       &SignatureValidatorConfig{SecretKey: secret, Strict: true},
			body:      body,
			signature: "invalid",
			wantErr:   true,
		},
		{
			name:      "invalid signature without strict mode",
			cfg:       &SignatureValidatorConfig{SecretKey: secret, Strict: false},
			body:      body,
			signature: "invalid",
			wantErr:   false, // Not strict, so no error
		},
		{
			name:      "empty signature with strict mode",
			cfg:       &SignatureValidatorConfig{SecretKey: secret, Strict: true},
			body:      body,
			signature: "",
			wantErr:   true,
		},
		{
			name:      "empty signature without strict mode",
			cfg:       &SignatureValidatorConfig{SecretKey: secret, Strict: false},
			body:      body,
			signature: "",
			wantErr:   false,
		},
		{
			name:      "validation disabled (empty secret)",
			cfg:       &SignatureValidatorConfig{SecretKey: "", Strict: true},
			body:      body,
			signature: "anything",
			wantErr:   false, // Disabled, so no error
		},
		{
			name:      "nil config",
			cfg:       nil,
			body:      body,
			signature: validSignature,
			wantErr:   false, // Disabled, so no error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewKiwifySignatureValidator(tt.cfg)
			err := v.Validate(tt.body, tt.signature)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestKiwifySignatureValidator_ValidateWithResult(t *testing.T) {
	secret := "test-secret-key"
	body := []byte(`{"event":"order_approved"}`)

	// Compute valid signature
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	validSignature := hex.EncodeToString(mac.Sum(nil))

	tests := []struct {
		name       string
		cfg        *SignatureValidatorConfig
		body       []byte
		signature  string
		wantValid  bool
		wantReason string
	}{
		{
			name:       "valid signature",
			cfg:        &SignatureValidatorConfig{SecretKey: secret, Strict: true},
			body:       body,
			signature:  validSignature,
			wantValid:  true,
			wantReason: "signature verified",
		},
		{
			name:       "validation disabled",
			cfg:        &SignatureValidatorConfig{SecretKey: ""},
			body:       body,
			signature:  "anything",
			wantValid:  true,
			wantReason: "validation disabled",
		},
		{
			name:       "no signature provided (non-strict)",
			cfg:        &SignatureValidatorConfig{SecretKey: secret, Strict: false},
			body:       body,
			signature:  "",
			wantValid:  true,
			wantReason: "no signature provided",
		},
		{
			name:       "no signature provided (strict)",
			cfg:        &SignatureValidatorConfig{SecretKey: secret, Strict: true},
			body:       body,
			signature:  "",
			wantValid:  false,
			wantReason: "no signature provided",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewKiwifySignatureValidator(tt.cfg)
			result := v.ValidateWithResult(tt.body, tt.signature)
			if result.Valid != tt.wantValid {
				t.Errorf("ValidateWithResult().Valid = %v, want %v", result.Valid, tt.wantValid)
			}
			if result.Reason != tt.wantReason {
				t.Errorf("ValidateWithResult().Reason = %v, want %v", result.Reason, tt.wantReason)
			}
		})
	}
}

func TestKiwifySignatureValidator_ComputeSignature(t *testing.T) {
	secret := "test-secret-key"
	body := []byte(`{"event":"order_approved"}`)

	v := NewKiwifySignatureValidator(&SignatureValidatorConfig{SecretKey: secret})
	sig := v.ComputeSignature(body)

	// Verify format is lowercase hex
	if len(sig) != 64 { // SHA256 = 32 bytes = 64 hex chars
		t.Errorf("ComputeSignature() returned wrong length: %d, want 64", len(sig))
	}

	// Verify it's valid hex
	_, err := hex.DecodeString(sig)
	if err != nil {
		t.Errorf("ComputeSignature() returned invalid hex: %v", err)
	}

	// Verify validation works with computed signature
	err = v.Validate(body, sig)
	if err != nil {
		t.Errorf("Validate() failed with computed signature: %v", err)
	}
}

func TestKiwifySignatureValidator_IsEnabled(t *testing.T) {
	tests := []struct {
		name   string
		secret string
		want   bool
	}{
		{"enabled with secret", "secret", true},
		{"disabled with empty secret", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewKiwifySignatureValidator(&SignatureValidatorConfig{SecretKey: tt.secret})
			if got := v.IsEnabled(); got != tt.want {
				t.Errorf("IsEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestKiwifySignatureValidator_IsStrict(t *testing.T) {
	tests := []struct {
		name   string
		strict bool
	}{
		{"strict mode on", true},
		{"strict mode off", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewKiwifySignatureValidator(&SignatureValidatorConfig{SecretKey: "secret", Strict: tt.strict})
			if got := v.IsStrict(); got != tt.strict {
				t.Errorf("IsStrict() = %v, want %v", got, tt.strict)
			}
		})
	}
}
