package crypto

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// validKey generates a fresh AES-256 key for test use.
func validKey(t *testing.T) string {
	t.Helper()
	k, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	return k
}

// shortKey returns a base64-encoded 16-byte key (wrong length).
func shortKey() string {
	b := make([]byte, 16)
	return base64.StdEncoding.EncodeToString(b)
}

// containsAny reports whether s contains at least one rune from chars.
func containsAny(s, chars string) bool {
	for _, c := range chars {
		if strings.ContainsRune(s, c) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// vault.go — GenerateKey
// ---------------------------------------------------------------------------

func TestGenerateKey_ReturnsBase64(t *testing.T) {
	t.Parallel()
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	decoded, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		t.Fatalf("key is not valid base64: %v", err)
	}
	if len(decoded) != 32 {
		t.Errorf("decoded key length = %d; want 32", len(decoded))
	}
}

func TestGenerateKey_Uniqueness(t *testing.T) {
	t.Parallel()
	const iterations = 50
	seen := make(map[string]struct{}, iterations)
	for i := 0; i < iterations; i++ {
		k, err := GenerateKey()
		if err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
		if _, dup := seen[k]; dup {
			t.Fatalf("duplicate key generated at iteration %d", i)
		}
		seen[k] = struct{}{}
	}
}

// ---------------------------------------------------------------------------
// vault.go — EncryptJSONV1
// ---------------------------------------------------------------------------

type testPayload struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

func TestEncryptJSONV1_Struct(t *testing.T) {
	t.Parallel()
	key := validKey(t)
	blob, err := EncryptJSONV1(testPayload{Name: "foo", Value: 42}, key)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if blob.V != 1 {
		t.Errorf("V = %d; want 1", blob.V)
	}
	if blob.Alg != "aes-256-gcm" {
		t.Errorf("Alg = %q; want %q", blob.Alg, "aes-256-gcm")
	}
	for _, field := range []struct {
		name, val string
	}{
		{"IV", blob.IV},
		{"CT", blob.CT},
		{"Tag", blob.Tag},
	} {
		if field.val == "" {
			t.Errorf("%s is empty", field.name)
		}
		if _, err := base64.StdEncoding.DecodeString(field.val); err != nil {
			t.Errorf("%s is not valid base64: %v", field.name, err)
		}
	}
}

func TestEncryptJSONV1_String(t *testing.T) {
	t.Parallel()
	key := validKey(t)
	blob, err := EncryptJSONV1("hello world", key)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if blob.V != 1 {
		t.Errorf("V = %d; want 1", blob.V)
	}
}

func TestEncryptJSONV1_InvalidKeyNotBase64(t *testing.T) {
	t.Parallel()
	_, err := EncryptJSONV1("data", "not-valid-base64!!!")
	if err == nil {
		t.Fatal("expected error for non-base64 key")
	}
}

func TestEncryptJSONV1_InvalidKeyWrongLength(t *testing.T) {
	t.Parallel()
	_, err := EncryptJSONV1("data", shortKey())
	if err == nil {
		t.Fatal("expected error for 16-byte key")
	}
}

func TestEncryptJSONV1_NonMarshallable(t *testing.T) {
	t.Parallel()
	key := validKey(t)
	ch := make(chan int)
	_, err := EncryptJSONV1(ch, key)
	if err == nil {
		t.Fatal("expected error for non-marshallable value")
	}
}

// ---------------------------------------------------------------------------
// vault.go — DecryptJSONV1
// ---------------------------------------------------------------------------

func TestDecryptJSONV1_RoundTripStruct(t *testing.T) {
	t.Parallel()
	key := validKey(t)
	original := testPayload{Name: "bar", Value: 99}

	blob, err := EncryptJSONV1(original, key)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	var got testPayload
	if err := DecryptJSONV1(blob, key, &got); err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if got != original {
		t.Errorf("got %+v; want %+v", got, original)
	}
}

func TestDecryptJSONV1_RoundTripString(t *testing.T) {
	t.Parallel()
	key := validKey(t)
	original := "sensitive data"

	blob, err := EncryptJSONV1(original, key)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	var got string
	if err := DecryptJSONV1(blob, key, &got); err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if got != original {
		t.Errorf("got %q; want %q", got, original)
	}
}

func TestDecryptJSONV1_WrongVersion(t *testing.T) {
	t.Parallel()
	key := validKey(t)
	blob, _ := EncryptJSONV1("x", key)
	blob.V = 2

	var out string
	if err := DecryptJSONV1(blob, key, &out); err == nil {
		t.Fatal("expected error for version 2")
	}
}

func TestDecryptJSONV1_WrongAlgorithm(t *testing.T) {
	t.Parallel()
	key := validKey(t)
	blob, _ := EncryptJSONV1("x", key)
	blob.Alg = "chacha20"

	var out string
	if err := DecryptJSONV1(blob, key, &out); err == nil {
		t.Fatal("expected error for unsupported algorithm")
	}
}

func TestDecryptJSONV1_InvalidKey(t *testing.T) {
	t.Parallel()
	key := validKey(t)
	blob, _ := EncryptJSONV1("x", key)

	var out string
	if err := DecryptJSONV1(blob, "not-base64!!!", &out); err == nil {
		t.Fatal("expected error for invalid key")
	}
}

func TestDecryptJSONV1_TamperedCiphertext(t *testing.T) {
	t.Parallel()
	key := validKey(t)
	blob, _ := EncryptJSONV1("secret", key)

	ct, _ := base64.StdEncoding.DecodeString(blob.CT)
	if len(ct) > 0 {
		ct[0] ^= 0xFF // flip bits
	}
	blob.CT = base64.StdEncoding.EncodeToString(ct)

	var out string
	if err := DecryptJSONV1(blob, key, &out); err == nil {
		t.Fatal("expected GCM auth failure for tampered ciphertext")
	}
}

func TestDecryptJSONV1_TamperedTag(t *testing.T) {
	t.Parallel()
	key := validKey(t)
	blob, _ := EncryptJSONV1("secret", key)

	tag, _ := base64.StdEncoding.DecodeString(blob.Tag)
	if len(tag) > 0 {
		tag[0] ^= 0xFF
	}
	blob.Tag = base64.StdEncoding.EncodeToString(tag)

	var out string
	if err := DecryptJSONV1(blob, key, &out); err == nil {
		t.Fatal("expected GCM auth failure for tampered tag")
	}
}

func TestDecryptJSONV1_InvalidIVBase64(t *testing.T) {
	t.Parallel()
	key := validKey(t)
	blob, _ := EncryptJSONV1("x", key)
	blob.IV = "%%%invalid-base64%%%"

	var out string
	if err := DecryptJSONV1(blob, key, &out); err == nil {
		t.Fatal("expected error for invalid IV base64")
	}
}

func TestDecryptJSONV1_InvalidCTBase64(t *testing.T) {
	t.Parallel()
	key := validKey(t)
	blob, _ := EncryptJSONV1("x", key)
	blob.CT = "%%%invalid-base64%%%"

	var out string
	if err := DecryptJSONV1(blob, key, &out); err == nil {
		t.Fatal("expected error for invalid CT base64")
	}
}

func TestDecryptJSONV1_InvalidTagBase64(t *testing.T) {
	t.Parallel()
	key := validKey(t)
	blob, _ := EncryptJSONV1("x", key)
	blob.Tag = "%%%invalid-base64%%%"

	var out string
	if err := DecryptJSONV1(blob, key, &out); err == nil {
		t.Fatal("expected error for invalid Tag base64")
	}
}

// ---------------------------------------------------------------------------
// vault.go — EncryptString / DecryptString
// ---------------------------------------------------------------------------

func TestEncryptDecryptString_RoundTrip(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		value string
	}{
		{"simple", "hello"},
		{"empty", ""},
		{"unicode", "olá mundo 🌎"},
		{"long", strings.Repeat("a", 10_000)},
		{"json_like", `{"key":"value"}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			key := validKey(t)

			encrypted, err := EncryptString(tc.value, key)
			if err != nil {
				t.Fatalf("EncryptString: %v", err)
			}

			got, err := DecryptString(encrypted, key)
			if err != nil {
				t.Fatalf("DecryptString: %v", err)
			}
			if got != tc.value {
				t.Errorf("got %q; want %q", got, tc.value)
			}
		})
	}
}

func TestEncryptString_InvalidKey(t *testing.T) {
	t.Parallel()
	_, err := EncryptString("data", "bad-key!!!")
	if err == nil {
		t.Fatal("expected error for invalid key")
	}
}

func TestDecryptString_InvalidJSON(t *testing.T) {
	t.Parallel()
	key := validKey(t)
	_, err := DecryptString("not json at all", key)
	if err == nil {
		t.Fatal("expected error for non-JSON input")
	}
}

func TestDecryptString_InvalidEncryptedFormat(t *testing.T) {
	t.Parallel()
	key := validKey(t)
	// Valid JSON but not a valid EncryptedBlobV1 (wrong version, missing fields, etc.)
	badBlob := EncryptedBlobV1{V: 99, Alg: "unknown", IV: "", CT: "", Tag: ""}
	data, _ := json.Marshal(badBlob)
	_, err := DecryptString(string(data), key)
	if err == nil {
		t.Fatal("expected error for invalid encrypted format")
	}
}

func TestDecryptString_WrongKey(t *testing.T) {
	t.Parallel()
	key1 := validKey(t)
	key2 := validKey(t)

	encrypted, err := EncryptString("secret", key1)
	if err != nil {
		t.Fatalf("EncryptString: %v", err)
	}

	_, err = DecryptString(encrypted, key2)
	if err == nil {
		t.Fatal("expected error when decrypting with wrong key")
	}
}

// ---------------------------------------------------------------------------
// vault.go — Cross-key test
// ---------------------------------------------------------------------------

func TestEncryptDecryptJSONV1_CrossKey(t *testing.T) {
	t.Parallel()
	key1 := validKey(t)
	key2 := validKey(t)

	blob, err := EncryptJSONV1("top secret", key1)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	var out string
	if err := DecryptJSONV1(blob, key2, &out); err == nil {
		t.Fatal("expected error when decrypting with a different key")
	}
}

// ---------------------------------------------------------------------------
// vault.go — Multiple encryptions produce different ciphertexts (IV varies)
// ---------------------------------------------------------------------------

func TestEncryptJSONV1_DifferentCiphertexts(t *testing.T) {
	t.Parallel()
	key := validKey(t)

	const iterations = 20
	ivs := make(map[string]struct{}, iterations)
	cts := make(map[string]struct{}, iterations)

	for i := 0; i < iterations; i++ {
		blob, err := EncryptJSONV1("same plaintext", key)
		if err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
		ivs[blob.IV] = struct{}{}
		cts[blob.CT] = struct{}{}
	}

	if len(ivs) < iterations {
		t.Errorf("expected %d unique IVs; got %d", iterations, len(ivs))
	}
	// Ciphertexts should also differ because IVs differ under GCM.
	if len(cts) < iterations {
		t.Errorf("expected %d unique ciphertexts; got %d", iterations, len(cts))
	}
}

// ---------------------------------------------------------------------------
// password.go — GenerateSecurePassword
// ---------------------------------------------------------------------------

func TestGenerateSecurePassword_DefaultLength(t *testing.T) {
	t.Parallel()

	cases := []int{0, 1, 2, 3, -1, -100}
	for _, length := range cases {
		t.Run("length_lt_4", func(t *testing.T) {
			t.Parallel()
			pw, err := GenerateSecurePassword(length)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(pw) != DefaultPasswordLength {
				t.Errorf("len = %d; want %d (default) for input length %d", len(pw), DefaultPasswordLength, length)
			}
		})
	}
}

func TestGenerateSecurePassword_SpecifiedLengths(t *testing.T) {
	t.Parallel()

	lengths := []int{4, 8, 12, 20, 50, 100}
	for _, l := range lengths {
		l := l
		t.Run("", func(t *testing.T) {
			t.Parallel()
			pw, err := GenerateSecurePassword(l)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(pw) != l {
				t.Errorf("len = %d; want %d", len(pw), l)
			}
		})
	}
}

func TestGenerateSecurePassword_ContainsAllCharSets(t *testing.T) {
	t.Parallel()

	// Run multiple iterations to reduce false negatives from randomness.
	const iterations = 50
	for i := 0; i < iterations; i++ {
		pw, err := GenerateSecurePassword(12)
		if err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
		if !containsAny(pw, lowercaseChars) {
			t.Errorf("iteration %d: password %q missing lowercase", i, pw)
		}
		if !containsAny(pw, uppercaseChars) {
			t.Errorf("iteration %d: password %q missing uppercase", i, pw)
		}
		if !containsAny(pw, digitChars) {
			t.Errorf("iteration %d: password %q missing digit", i, pw)
		}
		if !containsAny(pw, symbolChars) {
			t.Errorf("iteration %d: password %q missing symbol", i, pw)
		}
	}
}

func TestGenerateSecurePassword_Uniqueness(t *testing.T) {
	t.Parallel()

	const iterations = 100
	seen := make(map[string]struct{}, iterations)
	for i := 0; i < iterations; i++ {
		pw, err := GenerateSecurePassword(20)
		if err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
		if _, dup := seen[pw]; dup {
			t.Fatalf("duplicate password at iteration %d: %q", i, pw)
		}
		seen[pw] = struct{}{}
	}
}

func TestGenerateSecurePassword_MinLength4(t *testing.T) {
	t.Parallel()

	const iterations = 30
	for i := 0; i < iterations; i++ {
		pw, err := GenerateSecurePassword(4)
		if err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
		if len(pw) != 4 {
			t.Errorf("len = %d; want 4", len(pw))
		}
		if !containsAny(pw, lowercaseChars) {
			t.Errorf("iteration %d: password %q missing lowercase", i, pw)
		}
		if !containsAny(pw, uppercaseChars) {
			t.Errorf("iteration %d: password %q missing uppercase", i, pw)
		}
		if !containsAny(pw, digitChars) {
			t.Errorf("iteration %d: password %q missing digit", i, pw)
		}
		if !containsAny(pw, symbolChars) {
			t.Errorf("iteration %d: password %q missing symbol", i, pw)
		}
	}
}

func TestGenerateSecurePassword_Length100(t *testing.T) {
	t.Parallel()
	pw, err := GenerateSecurePassword(100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pw) != 100 {
		t.Errorf("len = %d; want 100", len(pw))
	}
}

func TestGenerateSecurePassword_OnlyAllowedChars(t *testing.T) {
	t.Parallel()

	const iterations = 50
	for i := 0; i < iterations; i++ {
		pw, err := GenerateSecurePassword(30)
		if err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
		for j, c := range pw {
			if !strings.ContainsRune(allChars, c) {
				t.Errorf("iteration %d pos %d: unexpected char %q", i, j, string(c))
			}
		}
	}
}

// ---------------------------------------------------------------------------
// password.go — GenerateTemporaryPassword
// ---------------------------------------------------------------------------

func TestGenerateTemporaryPassword_Length(t *testing.T) {
	t.Parallel()
	pw, err := GenerateTemporaryPassword()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pw) != DefaultPasswordLength {
		t.Errorf("len = %d; want %d", len(pw), DefaultPasswordLength)
	}
}

func TestGenerateTemporaryPassword_ContainsAllCharSets(t *testing.T) {
	t.Parallel()

	const iterations = 30
	for i := 0; i < iterations; i++ {
		pw, err := GenerateTemporaryPassword()
		if err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
		if !containsAny(pw, lowercaseChars) {
			t.Errorf("iteration %d: %q missing lowercase", i, pw)
		}
		if !containsAny(pw, uppercaseChars) {
			t.Errorf("iteration %d: %q missing uppercase", i, pw)
		}
		if !containsAny(pw, digitChars) {
			t.Errorf("iteration %d: %q missing digit", i, pw)
		}
		if !containsAny(pw, symbolChars) {
			t.Errorf("iteration %d: %q missing symbol", i, pw)
		}
	}
}

// ---------------------------------------------------------------------------
// password.go — secureRandomInt (unexported)
// ---------------------------------------------------------------------------

func TestSecureRandomInt_Range(t *testing.T) {
	t.Parallel()

	maxValues := []int{1, 2, 5, 10, 100, 1000}
	for _, max := range maxValues {
		max := max
		t.Run("", func(t *testing.T) {
			t.Parallel()
			for i := 0; i < 200; i++ {
				n, err := secureRandomInt(max)
				if err != nil {
					t.Fatalf("max=%d iter=%d: %v", max, i, err)
				}
				if n < 0 || n >= max {
					t.Fatalf("max=%d: got %d; want [0, %d)", max, n, max)
				}
			}
		})
	}
}

func TestSecureRandomInt_Max1AlwaysZero(t *testing.T) {
	t.Parallel()
	for i := 0; i < 100; i++ {
		n, err := secureRandomInt(1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if n != 0 {
			t.Fatalf("got %d; want 0 when max=1", n)
		}
	}
}

func TestSecureRandomInt_Distribution(t *testing.T) {
	t.Parallel()
	// With max=10 and 500 iterations, we expect all values to appear.
	// This is probabilistic but extremely unlikely to fail.
	const max = 10
	const iterations = 500
	counts := make(map[int]int, max)
	for i := 0; i < iterations; i++ {
		n, err := secureRandomInt(max)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		counts[n]++
	}
	for v := 0; v < max; v++ {
		if counts[v] == 0 {
			t.Errorf("value %d never produced in %d iterations", v, iterations)
		}
	}
}

// ---------------------------------------------------------------------------
// password.go — secureRandomChar (unexported)
// ---------------------------------------------------------------------------

func TestSecureRandomChar_InCharset(t *testing.T) {
	t.Parallel()

	charsets := []string{
		lowercaseChars,
		uppercaseChars,
		digitChars,
		symbolChars,
		allChars,
		"abc",
		"X",
	}

	for _, cs := range charsets {
		cs := cs
		t.Run("", func(t *testing.T) {
			t.Parallel()
			for i := 0; i < 100; i++ {
				c, err := secureRandomChar(cs)
				if err != nil {
					t.Fatalf("charset=%q iter=%d: %v", cs, i, err)
				}
				if !strings.ContainsRune(cs, rune(c)) {
					t.Fatalf("char %q not in charset %q", string(c), cs)
				}
			}
		})
	}
}

func TestSecureRandomChar_SingleCharCharset(t *testing.T) {
	t.Parallel()
	for i := 0; i < 50; i++ {
		c, err := secureRandomChar("Z")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if c != 'Z' {
			t.Fatalf("got %q; want 'Z'", string(c))
		}
	}
}

func TestSecureRandomChar_AllCharsAppear(t *testing.T) {
	t.Parallel()
	const charset = "ABCD"
	seen := make(map[byte]bool, len(charset))
	for i := 0; i < 200; i++ {
		c, err := secureRandomChar(charset)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		seen[c] = true
	}
	for i := 0; i < len(charset); i++ {
		if !seen[charset[i]] {
			t.Errorf("char %q never produced in 200 iterations", string(charset[i]))
		}
	}
}

// ---------------------------------------------------------------------------
// Table-driven: EncryptJSONV1 error cases
// ---------------------------------------------------------------------------

func TestEncryptJSONV1_ErrorCases(t *testing.T) {
	t.Parallel()

	goodKey := validKey(t)

	cases := []struct {
		name  string
		value interface{}
		key   string
	}{
		{"not_base64_key", "data", "!!!not-base64!!!"},
		{"short_key_16b", "data", shortKey()},
		{"empty_key", "data", ""},
		{"channel_value", make(chan int), goodKey},
		{"func_value", func() {}, goodKey},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := EncryptJSONV1(tc.value, tc.key)
			if err == nil {
				t.Fatalf("expected error for case %q", tc.name)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Table-driven: DecryptJSONV1 error cases
// ---------------------------------------------------------------------------

func TestDecryptJSONV1_ErrorCases(t *testing.T) {
	t.Parallel()

	key := validKey(t)
	blob, err := EncryptJSONV1("test", key)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	// Make copies so mutations do not interfere across subtests.
	copyBlob := func() EncryptedBlobV1 { return *blob }

	cases := []struct {
		name   string
		mutate func(*EncryptedBlobV1)
		key    string
	}{
		{"wrong_version", func(b *EncryptedBlobV1) { b.V = 0 }, key},
		{"wrong_alg", func(b *EncryptedBlobV1) { b.Alg = "none" }, key},
		{"bad_key", func(_ *EncryptedBlobV1) {}, "not-base64!!!"},
		{"short_key", func(_ *EncryptedBlobV1) {}, shortKey()},
		{"bad_iv_b64", func(b *EncryptedBlobV1) { b.IV = "%%%" }, key},
		{"bad_ct_b64", func(b *EncryptedBlobV1) { b.CT = "%%%" }, key},
		{"bad_tag_b64", func(b *EncryptedBlobV1) { b.Tag = "%%%" }, key},
		{"tampered_ct", func(b *EncryptedBlobV1) {
			ct, _ := base64.StdEncoding.DecodeString(b.CT)
			if len(ct) > 0 {
				ct[0] ^= 0xFF
			}
			b.CT = base64.StdEncoding.EncodeToString(ct)
		}, key},
		{"tampered_tag", func(b *EncryptedBlobV1) {
			tag, _ := base64.StdEncoding.DecodeString(b.Tag)
			if len(tag) > 0 {
				tag[0] ^= 0xFF
			}
			b.Tag = base64.StdEncoding.EncodeToString(tag)
		}, key},
		{"tampered_iv", func(b *EncryptedBlobV1) {
			iv, _ := base64.StdEncoding.DecodeString(b.IV)
			if len(iv) > 0 {
				iv[0] ^= 0xFF
			}
			b.IV = base64.StdEncoding.EncodeToString(iv)
		}, key},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b := copyBlob()
			tc.mutate(&b)
			var out string
			if err := DecryptJSONV1(&b, tc.key, &out); err == nil {
				t.Fatalf("expected error for case %q", tc.name)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// DecryptJSONV1 — json.Unmarshal failure (vault.go line 110)
// ---------------------------------------------------------------------------

func TestDecryptJSONV1_UnmarshalTypeMismatch(t *testing.T) {
	t.Parallel()
	key := validKey(t)

	// Encrypt a string, then try to decrypt into an int — json.Unmarshal
	// cannot decode a JSON string into *int, so line 110 returns an error.
	blob, err := EncryptJSONV1("this is a string", key)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	var dest int
	if err := DecryptJSONV1(blob, key, &dest); err == nil {
		t.Fatal("expected json.Unmarshal error when decrypting string into *int")
	}
}

func TestDecryptJSONV1_UnmarshalIntoStruct(t *testing.T) {
	t.Parallel()
	key := validKey(t)

	// Encrypt a plain number, try to decrypt into a struct that expects object fields.
	blob, err := EncryptJSONV1(42, key)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	var dest testPayload
	if err := DecryptJSONV1(blob, key, &dest); err == nil {
		t.Fatal("expected json.Unmarshal error when decrypting number into struct")
	}
}

// ---------------------------------------------------------------------------
// DecryptJSONV1 — empty/zero-length base64 fields
// ---------------------------------------------------------------------------

func TestDecryptJSONV1_EmptyCTAndTag(t *testing.T) {
	t.Parallel()
	key := validKey(t)
	blob, err := EncryptJSONV1("x", key)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	blob.CT = ""  // empty ciphertext
	blob.Tag = "" // empty tag → gcm.Open fails (auth mismatch)

	var out string
	if err := DecryptJSONV1(blob, key, &out); err == nil {
		t.Fatal("expected error for empty ciphertext and tag")
	}
}

func TestDecryptJSONV1_EmptyTagOnly(t *testing.T) {
	t.Parallel()
	key := validKey(t)
	blob, err := EncryptJSONV1("x", key)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	blob.Tag = "" // empty tag → GCM authentication fails

	var out string
	if err := DecryptJSONV1(blob, key, &out); err == nil {
		t.Fatal("expected error for empty tag")
	}
}

// ---------------------------------------------------------------------------
// DecryptJSONV1 — version/algorithm boundary values
// ---------------------------------------------------------------------------

func TestDecryptJSONV1_VersionZero(t *testing.T) {
	t.Parallel()
	key := validKey(t)
	blob := &EncryptedBlobV1{V: 0, Alg: "aes-256-gcm", IV: "AAAA", CT: "AAAA", Tag: "AAAA"}
	var out string
	err := DecryptJSONV1(blob, key, &out)
	if err == nil {
		t.Fatal("expected error for version 0")
	}
	if !strings.Contains(err.Error(), "unsupported encryption format") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestDecryptJSONV1_VersionNegative(t *testing.T) {
	t.Parallel()
	key := validKey(t)
	blob := &EncryptedBlobV1{V: -1, Alg: "aes-256-gcm", IV: "AAAA", CT: "AAAA", Tag: "AAAA"}
	var out string
	err := DecryptJSONV1(blob, key, &out)
	if err == nil {
		t.Fatal("expected error for negative version")
	}
}

func TestDecryptJSONV1_EmptyAlgorithm(t *testing.T) {
	t.Parallel()
	key := validKey(t)
	blob := &EncryptedBlobV1{V: 1, Alg: "", IV: "AAAA", CT: "AAAA", Tag: "AAAA"}
	var out string
	err := DecryptJSONV1(blob, key, &out)
	if err == nil {
		t.Fatal("expected error for empty algorithm")
	}
	if !strings.Contains(err.Error(), "unsupported encryption format") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// ---------------------------------------------------------------------------
// DecryptJSONV1 — key edge cases
// ---------------------------------------------------------------------------

func TestDecryptJSONV1_EmptyKey(t *testing.T) {
	t.Parallel()
	key := validKey(t)
	blob, err := EncryptJSONV1("data", key)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	var out string
	if err := DecryptJSONV1(blob, "", &out); err == nil {
		t.Fatal("expected error for empty key")
	}
}

func TestDecryptJSONV1_KeyTooLong(t *testing.T) {
	t.Parallel()
	key := validKey(t)
	blob, err := EncryptJSONV1("data", key)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	longKey := base64.StdEncoding.EncodeToString(make([]byte, 64))
	var out string
	if err := DecryptJSONV1(blob, longKey, &out); err == nil {
		t.Fatal("expected error for 64-byte key")
	}
}

// ---------------------------------------------------------------------------
// EncryptJSONV1 — key edge cases
// ---------------------------------------------------------------------------

func TestEncryptJSONV1_EmptyKey(t *testing.T) {
	t.Parallel()
	_, err := EncryptJSONV1("data", "")
	if err == nil {
		t.Fatal("expected error for empty key")
	}
}

func TestEncryptJSONV1_KeyTooLong(t *testing.T) {
	t.Parallel()
	longKey := base64.StdEncoding.EncodeToString(make([]byte, 64))
	_, err := EncryptJSONV1("data", longKey)
	if err == nil {
		t.Fatal("expected error for 64-byte key")
	}
}

func TestEncryptJSONV1_FuncValue(t *testing.T) {
	t.Parallel()
	key := validKey(t)
	_, err := EncryptJSONV1(func() {}, key)
	if err == nil {
		t.Fatal("expected error for func value (not JSON-marshalable)")
	}
}

func TestEncryptJSONV1_MapWithChanValue(t *testing.T) {
	t.Parallel()
	key := validKey(t)
	m := map[string]interface{}{"ch": make(chan int)}
	_, err := EncryptJSONV1(m, key)
	if err == nil {
		t.Fatal("expected error for map containing channel")
	}
}

// ---------------------------------------------------------------------------
// EncryptJSONV1 — various value types (ensure roundtrip correctness)
// ---------------------------------------------------------------------------

func TestEncryptDecryptJSONV1_Nil(t *testing.T) {
	t.Parallel()
	key := validKey(t)

	blob, err := EncryptJSONV1(nil, key)
	if err != nil {
		t.Fatalf("encrypt nil: %v", err)
	}

	var got interface{}
	if err := DecryptJSONV1(blob, key, &got); err != nil {
		t.Fatalf("decrypt nil: %v", err)
	}
	if got != nil {
		t.Errorf("got %v; want nil", got)
	}
}

func TestEncryptDecryptJSONV1_Bool(t *testing.T) {
	t.Parallel()
	key := validKey(t)

	blob, err := EncryptJSONV1(true, key)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	var got bool
	if err := DecryptJSONV1(blob, key, &got); err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if !got {
		t.Error("got false; want true")
	}
}

func TestEncryptDecryptJSONV1_Slice(t *testing.T) {
	t.Parallel()
	key := validKey(t)

	original := []string{"a", "b", "c"}
	blob, err := EncryptJSONV1(original, key)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	var got []string
	if err := DecryptJSONV1(blob, key, &got); err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if len(got) != len(original) {
		t.Fatalf("len = %d; want %d", len(got), len(original))
	}
	for i, v := range original {
		if got[i] != v {
			t.Errorf("index %d: got %q; want %q", i, got[i], v)
		}
	}
}

func TestEncryptDecryptJSONV1_NestedMap(t *testing.T) {
	t.Parallel()
	key := validKey(t)

	original := map[string]interface{}{
		"name": "test",
		"nested": map[string]interface{}{
			"key": float64(123),
		},
	}

	blob, err := EncryptJSONV1(original, key)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	var got map[string]interface{}
	if err := DecryptJSONV1(blob, key, &got); err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if got["name"] != "test" {
		t.Errorf("name = %v; want %q", got["name"], "test")
	}
}

func TestEncryptDecryptJSONV1_EmptyString(t *testing.T) {
	t.Parallel()
	key := validKey(t)

	blob, err := EncryptJSONV1("", key)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	var got string
	if err := DecryptJSONV1(blob, key, &got); err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if got != "" {
		t.Errorf("got %q; want empty string", got)
	}
}

func TestEncryptDecryptJSONV1_LargPayload(t *testing.T) {
	t.Parallel()
	key := validKey(t)

	original := strings.Repeat("X", 100_000)
	blob, err := EncryptJSONV1(original, key)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	var got string
	if err := DecryptJSONV1(blob, key, &got); err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if got != original {
		t.Errorf("decrypted length = %d; want %d", len(got), len(original))
	}
}

// ---------------------------------------------------------------------------
// EncryptString — output is valid JSON containing EncryptedBlobV1
// ---------------------------------------------------------------------------

func TestEncryptString_OutputIsValidJSON(t *testing.T) {
	t.Parallel()
	key := validKey(t)

	encrypted, err := EncryptString("hello", key)
	if err != nil {
		t.Fatalf("EncryptString: %v", err)
	}

	var blob EncryptedBlobV1
	if err := json.Unmarshal([]byte(encrypted), &blob); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if blob.V != 1 {
		t.Errorf("V = %d; want 1", blob.V)
	}
	if blob.Alg != "aes-256-gcm" {
		t.Errorf("Alg = %q; want %q", blob.Alg, "aes-256-gcm")
	}
}

// ---------------------------------------------------------------------------
// DecryptString — additional edge cases
// ---------------------------------------------------------------------------

func TestDecryptString_EmptyInput(t *testing.T) {
	t.Parallel()
	key := validKey(t)
	_, err := DecryptString("", key)
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestDecryptString_ValidJSONWrongKey(t *testing.T) {
	t.Parallel()
	key1 := validKey(t)
	key2 := validKey(t)

	encrypted, err := EncryptString("secret", key1)
	if err != nil {
		t.Fatalf("EncryptString: %v", err)
	}

	_, err = DecryptString(encrypted, key2)
	if err == nil {
		t.Fatal("expected error when decrypting with wrong key")
	}
}

func TestDecryptString_JSONArrayInput(t *testing.T) {
	t.Parallel()
	key := validKey(t)
	_, err := DecryptString("[1,2,3]", key)
	if err == nil {
		t.Fatal("expected error for JSON array input (not a valid blob)")
	}
}

func TestDecryptString_JSONObjectMissingFields(t *testing.T) {
	t.Parallel()
	key := validKey(t)
	_, err := DecryptString(`{"v":1}`, key)
	if err == nil {
		t.Fatal("expected error for JSON object with missing fields")
	}
}

func TestDecryptString_InvalidBase64Key(t *testing.T) {
	t.Parallel()
	key := validKey(t)
	encrypted, err := EncryptString("data", key)
	if err != nil {
		t.Fatalf("EncryptString: %v", err)
	}

	_, err = DecryptString(encrypted, "not-valid-base64!!!")
	if err == nil {
		t.Fatal("expected error for invalid base64 key in DecryptString")
	}
}

// ---------------------------------------------------------------------------
// GenerateKey — structural properties
// ---------------------------------------------------------------------------

func TestGenerateKey_MultipleCallsUnique(t *testing.T) {
	t.Parallel()
	const n = 100
	seen := make(map[string]struct{}, n)
	for i := 0; i < n; i++ {
		k, err := GenerateKey()
		if err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
		if _, dup := seen[k]; dup {
			t.Fatalf("duplicate key at iteration %d", i)
		}
		seen[k] = struct{}{}
	}
}

func TestGenerateKey_UsableForEncryptDecrypt(t *testing.T) {
	t.Parallel()
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	blob, err := EncryptJSONV1("test roundtrip", key)
	if err != nil {
		t.Fatalf("EncryptJSONV1: %v", err)
	}

	var got string
	if err := DecryptJSONV1(blob, key, &got); err != nil {
		t.Fatalf("DecryptJSONV1: %v", err)
	}
	if got != "test roundtrip" {
		t.Errorf("got %q; want %q", got, "test roundtrip")
	}
}

// ---------------------------------------------------------------------------
// GenerateSecurePassword — boundary and structural tests
// ---------------------------------------------------------------------------

func TestGenerateSecurePassword_ExactlyLength4(t *testing.T) {
	t.Parallel()
	// length=4 is the minimum that does NOT get promoted to DefaultPasswordLength.
	for i := 0; i < 30; i++ {
		pw, err := GenerateSecurePassword(4)
		if err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
		if len(pw) != 4 {
			t.Errorf("len = %d; want 4", len(pw))
		}
	}
}

func TestGenerateSecurePassword_Length5(t *testing.T) {
	t.Parallel()
	// Just above the boundary; the 5th position is a random allChars char.
	for i := 0; i < 30; i++ {
		pw, err := GenerateSecurePassword(5)
		if err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
		if len(pw) != 5 {
			t.Errorf("len = %d; want 5", len(pw))
		}
		if !containsAny(pw, lowercaseChars) {
			t.Errorf("iteration %d: password %q missing lowercase", i, pw)
		}
		if !containsAny(pw, uppercaseChars) {
			t.Errorf("iteration %d: password %q missing uppercase", i, pw)
		}
		if !containsAny(pw, digitChars) {
			t.Errorf("iteration %d: password %q missing digit", i, pw)
		}
		if !containsAny(pw, symbolChars) {
			t.Errorf("iteration %d: password %q missing symbol", i, pw)
		}
	}
}

func TestGenerateSecurePassword_NegativeLength(t *testing.T) {
	t.Parallel()
	pw, err := GenerateSecurePassword(-50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pw) != DefaultPasswordLength {
		t.Errorf("len = %d; want %d for negative input", len(pw), DefaultPasswordLength)
	}
}

// ---------------------------------------------------------------------------
// GenerateTemporaryPassword — structural checks
// ---------------------------------------------------------------------------

func TestGenerateTemporaryPassword_Uniqueness(t *testing.T) {
	t.Parallel()
	const n = 50
	seen := make(map[string]struct{}, n)
	for i := 0; i < n; i++ {
		pw, err := GenerateTemporaryPassword()
		if err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
		if _, dup := seen[pw]; dup {
			t.Fatalf("duplicate temporary password at iteration %d", i)
		}
		seen[pw] = struct{}{}
	}
}

func TestGenerateTemporaryPassword_OnlyAllowedChars(t *testing.T) {
	t.Parallel()
	for i := 0; i < 30; i++ {
		pw, err := GenerateTemporaryPassword()
		if err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
		for j, c := range pw {
			if !strings.ContainsRune(allChars, c) {
				t.Errorf("iteration %d pos %d: unexpected char %q", i, j, string(c))
			}
		}
	}
}

// ---------------------------------------------------------------------------
// EncryptString / DecryptString — special characters roundtrip
// ---------------------------------------------------------------------------

func TestEncryptDecryptString_SpecialChars(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		value string
	}{
		{"backslash", `back\slash`},
		{"quotes", `"quoted"`},
		{"newlines", "line1\nline2\nline3"},
		{"tabs", "col1\tcol2\tcol3"},
		{"null_bytes", "before\x00after"},
		{"high_unicode", "\U0001F600\U0001F4A9"},
		{"json_braces", `{"nested":{"deep":"value"}}`},
		{"html_tags", `<script>alert("xss")</script>`},
		{"sql_injection", `'; DROP TABLE users; --`},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			key := validKey(t)

			encrypted, err := EncryptString(tc.value, key)
			if err != nil {
				t.Fatalf("EncryptString: %v", err)
			}

			got, err := DecryptString(encrypted, key)
			if err != nil {
				t.Fatalf("DecryptString: %v", err)
			}
			if got != tc.value {
				t.Errorf("got %q; want %q", got, tc.value)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// secureRandomInt — edge cases
// ---------------------------------------------------------------------------

func TestSecureRandomInt_LargeMax(t *testing.T) {
	t.Parallel()
	const max = 1_000_000
	for i := 0; i < 100; i++ {
		n, err := secureRandomInt(max)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if n < 0 || n >= max {
			t.Fatalf("got %d; want [0, %d)", n, max)
		}
	}
}

func TestSecureRandomInt_Max2ProducesBothValues(t *testing.T) {
	t.Parallel()
	seen := map[int]bool{}
	for i := 0; i < 100; i++ {
		n, err := secureRandomInt(2)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		seen[n] = true
	}
	if !seen[0] || !seen[1] {
		t.Errorf("with max=2, expected both 0 and 1; seen: %v", seen)
	}
}
