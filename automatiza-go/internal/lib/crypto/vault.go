package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// EncryptedBlobV1 represents an encrypted value with metadata
type EncryptedBlobV1 struct {
	V   int    `json:"v"`   // Version
	Alg string `json:"alg"` // Algorithm
	IV  string `json:"iv"`  // Initialization vector
	CT  string `json:"ct"`  // Ciphertext
	Tag string `json:"tag"` // Authentication tag
}

// EncryptJSONV1 encrypts a value using AES-256-GCM
func EncryptJSONV1(value interface{}, keyBase64 string) (*EncryptedBlobV1, error) {
	key, err := base64.StdEncoding.DecodeString(keyBase64)
	if err != nil || len(key) != 32 {
		return nil, fmt.Errorf("invalid encryption key: must be 32 bytes base64 encoded")
	}

	plaintext, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal value: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate random IV (12 bytes for GCM)
	iv := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(iv); err != nil {
		return nil, fmt.Errorf("failed to generate IV: %w", err)
	}

	// Encrypt
	ciphertext := gcm.Seal(nil, iv, plaintext, nil)

	// Split ciphertext and tag (last 16 bytes)
	tagStart := len(ciphertext) - gcm.Overhead()
	ct := ciphertext[:tagStart]
	tag := ciphertext[tagStart:]

	return &EncryptedBlobV1{
		V:   1,
		Alg: "aes-256-gcm",
		IV:  base64.StdEncoding.EncodeToString(iv),
		CT:  base64.StdEncoding.EncodeToString(ct),
		Tag: base64.StdEncoding.EncodeToString(tag),
	}, nil
}

// DecryptJSONV1 decrypts an encrypted blob
func DecryptJSONV1(blob *EncryptedBlobV1, keyBase64 string, dest interface{}) error {
	if blob.V != 1 || blob.Alg != "aes-256-gcm" {
		return fmt.Errorf("unsupported encryption format: v=%d alg=%s", blob.V, blob.Alg)
	}

	key, err := base64.StdEncoding.DecodeString(keyBase64)
	if err != nil || len(key) != 32 {
		return fmt.Errorf("invalid decryption key")
	}

	iv, err := base64.StdEncoding.DecodeString(blob.IV)
	if err != nil {
		return fmt.Errorf("invalid IV: %w", err)
	}

	ct, err := base64.StdEncoding.DecodeString(blob.CT)
	if err != nil {
		return fmt.Errorf("invalid ciphertext: %w", err)
	}

	tag, err := base64.StdEncoding.DecodeString(blob.Tag)
	if err != nil {
		return fmt.Errorf("invalid tag: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("failed to create GCM: %w", err)
	}

	// Combine ciphertext + tag
	ciphertext := append(ct, tag...)

	plaintext, err := gcm.Open(nil, iv, ciphertext, nil)
	if err != nil {
		return fmt.Errorf("decryption failed: %w", err)
	}

	return json.Unmarshal(plaintext, dest)
}

// EncryptString encrypts a string using AES-256-GCM and returns a JSON string
func EncryptString(value, keyBase64 string) (string, error) {
	blob, err := EncryptJSONV1(value, keyBase64)
	if err != nil {
		return "", err
	}

	jsonBytes, err := json.Marshal(blob)
	if err != nil {
		return "", err
	}

	return string(jsonBytes), nil
}

// DecryptString decrypts a JSON-encoded encrypted blob
func DecryptString(encrypted, keyBase64 string) (string, error) {
	var blob EncryptedBlobV1
	if err := json.Unmarshal([]byte(encrypted), &blob); err != nil {
		return "", fmt.Errorf("invalid encrypted format: %w", err)
	}

	var result string
	if err := DecryptJSONV1(&blob, keyBase64, &result); err != nil {
		return "", err
	}

	return result, nil
}

// GenerateKey generates a new random 32-byte key for AES-256
func GenerateKey() (string, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(key), nil
}
