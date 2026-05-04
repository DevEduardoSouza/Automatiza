package urlvalidation

import (
	"errors"
	"testing"

	apperrors "automatiza-go/internal/errors"
)

// ---------------------------------------------------------------------------
// ValidateMercadoLivreURL
// ---------------------------------------------------------------------------

func TestValidateMercadoLivreURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		url     string
		wantErr bool
		errCode error
	}{
		// ---- happy paths ----
		{
			name: "valid URL with /p/ path",
			url:  "https://www.mercadolivre.com.br/p/MLB12345678",
		},
		{
			name: "valid URL with mlb code in path",
			url:  "https://produto.mercadolivre.com.br/MLB-12345678-produto-nome",
		},
		{
			name: "valid URL with mlb lowercase",
			url:  "https://www.mercadolivre.com.br/produto-mlb12345678",
		},
		{
			name: "valid URL mercadolibre (Spanish) with MLA",
			url:  "https://www.mercadolibre.com.ar/p/MLA12345678",
		},
		{
			name: "valid URL with query params",
			url:  "https://www.mercadolivre.com.br/p/MLB12345678?variation=123",
		},
		{
			name: "valid URL with hash fragment",
			url:  "https://www.mercadolivre.com.br/p/MLB12345678#reviews",
		},
		{
			name: "valid URL with subdomain",
			url:  "https://produto.mercadolivre.com.br/p/MLB999",
		},
		{
			name: "valid URL with query, fragment and /p/",
			url:  "https://www.mercadolivre.com.br/p/MLB1?a=1&b=2#top",
		},
		{
			name: "valid URL with leading/trailing whitespace",
			url:  "  https://www.mercadolivre.com.br/p/MLB1  ",
		},
		{
			name: "valid URL mercadolibre with mlb in query",
			url:  "https://www.mercadolibre.com.mx/item?id=mlb123",
		},

		// ---- error paths ----
		{
			name:    "empty URL",
			url:     "",
			wantErr: true,
			errCode: apperrors.ErrInvalidURL,
		},
		{
			name:    "whitespace only",
			url:     "   ",
			wantErr: true,
			errCode: apperrors.ErrInvalidURL,
		},
		{
			name:    "tab and newline whitespace",
			url:     "\t\n",
			wantErr: true,
			errCode: apperrors.ErrInvalidURL,
		},
		{
			name:    "wrong domain - amazon",
			url:     "https://www.amazon.com.br/dp/B12345",
			wantErr: true,
			errCode: apperrors.ErrInvalidURL,
		},
		{
			name:    "wrong domain - google",
			url:     "https://www.google.com/search?q=mlb",
			wantErr: true,
			errCode: apperrors.ErrInvalidURL,
		},
		{
			name:    "no product indicator",
			url:     "https://www.mercadolivre.com.br/categorias",
			wantErr: true,
			errCode: apperrors.ErrInvalidURL,
		},
		{
			name:    "mercadolivre domain but no /p/ or mlb",
			url:     "https://www.mercadolivre.com.br/ofertas-do-dia",
			wantErr: true,
			errCode: apperrors.ErrInvalidURL,
		},
		{
			name:    "url.Parse error - control character",
			url:     "http://[::1",
			wantErr: true,
			errCode: apperrors.ErrInvalidURLFormat,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateMercadoLivreURL(tt.url)

			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateMercadoLivreURL(%q) error = %v, wantErr %v", tt.url, err, tt.wantErr)
			}

			if tt.wantErr && tt.errCode != nil {
				assertWrappedError(t, err, tt.errCode)
				assertFieldError(t, err, "origin_url")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// NormalizeURL
// ---------------------------------------------------------------------------

func TestNormalizeURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		url     string
		want    string
		wantErr bool
		errCode error
	}{
		{
			name: "removes hash fragment",
			url:  "https://www.mercadolivre.com.br/produto#reviews",
			want: "https://www.mercadolivre.com.br/produto",
		},
		{
			name: "URL without hash is unchanged",
			url:  "https://www.mercadolivre.com.br/produto",
			want: "https://www.mercadolivre.com.br/produto",
		},
		{
			name: "removes hash but preserves query",
			url:  "https://www.mercadolivre.com.br/produto?v=1#section",
			want: "https://www.mercadolivre.com.br/produto?v=1",
		},
		{
			name: "trims whitespace",
			url:  "  https://www.mercadolivre.com.br/produto  ",
			want: "https://www.mercadolivre.com.br/produto",
		},
		{
			name: "preserves multiple query params",
			url:  "https://example.com/path?a=1&b=2#frag",
			want: "https://example.com/path?a=1&b=2",
		},
		{
			name: "preserves path with trailing slash",
			url:  "https://example.com/path/",
			want: "https://example.com/path/",
		},
		{
			name:    "empty URL returns error",
			url:     "",
			want:    "",
			wantErr: true,
			errCode: apperrors.ErrInvalidURLFormat,
		},
		{
			name:    "whitespace only returns error",
			url:     "   ",
			want:    "",
			wantErr: true,
			errCode: apperrors.ErrInvalidURLFormat,
		},
		{
			name:    "url.Parse error - invalid host",
			url:     "http://[::1",
			want:    "",
			wantErr: true,
			errCode: apperrors.ErrInvalidURLFormat,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := NormalizeURL(tt.url)

			if (err != nil) != tt.wantErr {
				t.Fatalf("NormalizeURL(%q) error = %v, wantErr %v", tt.url, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("NormalizeURL(%q) = %q, want %q", tt.url, got, tt.want)
			}
			if tt.wantErr && tt.errCode != nil {
				assertWrappedError(t, err, tt.errCode)
				assertFieldError(t, err, "origin_url")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// DeduplicateURLs
// ---------------------------------------------------------------------------

func TestDeduplicateURLs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		urls     []string
		wantLen  int
		wantURLs []string // optional: exact expected output
	}{
		{
			name: "no duplicates preserves all",
			urls: []string{
				"https://www.mercadolivre.com.br/p/MLB1",
				"https://www.mercadolivre.com.br/p/MLB2",
				"https://www.mercadolivre.com.br/p/MLB3",
			},
			wantLen: 3,
		},
		{
			name: "exact duplicates removed",
			urls: []string{
				"https://www.mercadolivre.com.br/p/MLB1",
				"https://www.mercadolivre.com.br/p/MLB1",
				"https://www.mercadolivre.com.br/p/MLB2",
			},
			wantLen: 2,
			wantURLs: []string{
				"https://www.mercadolivre.com.br/p/MLB1",
				"https://www.mercadolivre.com.br/p/MLB2",
			},
		},
		{
			name: "hash fragment duplicates removed (keeps first)",
			urls: []string{
				"https://www.mercadolivre.com.br/p/MLB1",
				"https://www.mercadolivre.com.br/p/MLB1#reviews",
				"https://www.mercadolivre.com.br/p/MLB2",
			},
			wantLen: 2,
			wantURLs: []string{
				"https://www.mercadolivre.com.br/p/MLB1",
				"https://www.mercadolivre.com.br/p/MLB2",
			},
		},
		{
			name:    "empty list returns empty",
			urls:    []string{},
			wantLen: 0,
		},
		{
			name:    "nil slice returns empty",
			urls:    nil,
			wantLen: 0,
		},
		{
			name: "parseable non-ML URLs are kept",
			urls: []string{
				"https://www.mercadolivre.com.br/p/MLB1",
				"https://other-site.com/page",
				"https://www.mercadolivre.com.br/p/MLB2",
			},
			wantLen: 3,
		},
		{
			name: "empty URLs are skipped (normalization fails)",
			urls: []string{
				"https://www.mercadolivre.com.br/p/MLB1",
				"",
				"https://www.mercadolivre.com.br/p/MLB2",
			},
			wantLen: 2,
		},
		{
			name: "multiple identical URLs collapsed to one",
			urls: []string{
				"https://example.com/a",
				"https://example.com/a",
				"https://example.com/a",
				"https://example.com/a",
			},
			wantLen: 1,
			wantURLs: []string{
				"https://example.com/a",
			},
		},
		{
			name: "unparseable URLs skipped",
			urls: []string{
				"https://example.com/a",
				"http://[::1",
				"https://example.com/b",
			},
			wantLen: 2,
		},
		{
			name: "preserves original URL not normalized",
			urls: []string{
				"https://example.com/page#section",
			},
			wantLen: 1,
			wantURLs: []string{
				"https://example.com/page#section",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := DeduplicateURLs(tt.urls)

			if len(got) != tt.wantLen {
				t.Fatalf("DeduplicateURLs() returned %d URLs, want %d (got: %v)", len(got), tt.wantLen, got)
			}
			if tt.wantURLs != nil {
				for i, want := range tt.wantURLs {
					if got[i] != want {
						t.Errorf("DeduplicateURLs()[%d] = %q, want %q", i, got[i], want)
					}
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ValidateAndNormalizeURL
// ---------------------------------------------------------------------------

func TestValidateAndNormalizeURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		url     string
		want    string
		wantErr bool
		errCode error
	}{
		{
			name: "valid URL normalized (fragment removed)",
			url:  "https://www.mercadolivre.com.br/p/MLB12345678#reviews",
			want: "https://www.mercadolivre.com.br/p/MLB12345678",
		},
		{
			name: "valid URL without fragment unchanged",
			url:  "https://www.mercadolivre.com.br/p/MLB12345678",
			want: "https://www.mercadolivre.com.br/p/MLB12345678",
		},
		{
			name: "valid URL with query preserved",
			url:  "https://www.mercadolivre.com.br/p/MLB1?v=2#top",
			want: "https://www.mercadolivre.com.br/p/MLB1?v=2",
		},
		{
			name: "meli.la short link is accepted",
			url:  "https://meli.la/28fhqrh",
			want: "https://meli.la/28fhqrh",
		},
		{
			name:    "invalid domain returns validation error",
			url:     "https://www.amazon.com.br/dp/B123",
			want:    "",
			wantErr: true,
			errCode: apperrors.ErrInvalidURL,
		},
		{
			name:    "empty URL returns validation error",
			url:     "",
			want:    "",
			wantErr: true,
			errCode: apperrors.ErrInvalidURL,
		},
		{
			name:    "unsupported domain returns validation error",
			url:     "https://www.magazineluiza.com.br/produto/12345",
			want:    "",
			wantErr: true,
			errCode: apperrors.ErrInvalidURL,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ValidateAndNormalizeURL(tt.url)

			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateAndNormalizeURL(%q) error = %v, wantErr %v", tt.url, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("ValidateAndNormalizeURL(%q) = %q, want %q", tt.url, got, tt.want)
			}
			if tt.wantErr && tt.errCode != nil {
				assertWrappedError(t, err, tt.errCode)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// FilterValidURLs
// ---------------------------------------------------------------------------

func TestFilterValidURLs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		urls     []string
		wantLen  int
		wantURLs []string
	}{
		{
			name: "keeps only valid ML URLs",
			urls: []string{
				"https://www.mercadolivre.com.br/p/MLB1",
				"https://www.amazon.com.br/dp/B123",
				"https://www.mercadolivre.com.br/p/MLB2",
				"https://www.google.com",
			},
			wantLen: 2,
			wantURLs: []string{
				"https://www.mercadolivre.com.br/p/MLB1",
				"https://www.mercadolivre.com.br/p/MLB2",
			},
		},
		{
			name: "all valid URLs kept",
			urls: []string{
				"https://www.mercadolivre.com.br/p/MLB1",
				"https://produto.mercadolivre.com.br/MLB-999-item",
			},
			wantLen: 2,
		},
		{
			name: "all invalid URLs returns empty",
			urls: []string{
				"https://www.amazon.com.br/dp/B123",
				"https://www.google.com",
				"invalid-url",
			},
			wantLen: 0,
		},
		{
			name:    "empty input returns empty",
			urls:    []string{},
			wantLen: 0,
		},
		{
			name:    "nil input returns empty",
			urls:    nil,
			wantLen: 0,
		},
		{
			name: "empty strings filtered out",
			urls: []string{
				"",
				"https://www.mercadolivre.com.br/p/MLB1",
				"   ",
			},
			wantLen: 1,
			wantURLs: []string{
				"https://www.mercadolivre.com.br/p/MLB1",
			},
		},
		{
			name: "mercadolibre Spanish URLs accepted",
			urls: []string{
				"https://www.mercadolibre.com.ar/p/MLA1",
				"https://www.mercadolivre.com.br/p/MLB1",
			},
			wantLen: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := FilterValidURLs(tt.urls)

			if len(got) != tt.wantLen {
				t.Fatalf("FilterValidURLs() returned %d URLs, want %d (got: %v)", len(got), tt.wantLen, got)
			}
			if tt.wantURLs != nil {
				for i, want := range tt.wantURLs {
					if got[i] != want {
						t.Errorf("FilterValidURLs()[%d] = %q, want %q", i, got[i], want)
					}
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ValidateURLs
// ---------------------------------------------------------------------------

func TestValidateURLs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		urls          []string
		wantValidLen  int
		wantInvalidLen int
	}{
		{
			name: "mix of valid and invalid",
			urls: []string{
				"https://www.mercadolivre.com.br/p/MLB1",
				"https://www.amazon.com.br/dp/B123",
				"https://produto.mercadolivre.com.br/MLB-999-item",
				"",
			},
			wantValidLen:   2,
			wantInvalidLen: 2,
		},
		{
			name: "all valid",
			urls: []string{
				"https://www.mercadolivre.com.br/p/MLB1",
				"https://www.mercadolivre.com.br/p/MLB2",
			},
			wantValidLen:   2,
			wantInvalidLen: 0,
		},
		{
			name: "all invalid",
			urls: []string{
				"https://www.amazon.com.br/dp/B1",
				"",
				"invalid",
			},
			wantValidLen:   0,
			wantInvalidLen: 3,
		},
		{
			name:           "empty input",
			urls:           []string{},
			wantValidLen:   0,
			wantInvalidLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			results := ValidateURLs(tt.urls)

			if len(results) != len(tt.urls) {
				t.Fatalf("ValidateURLs() returned %d results, want %d", len(results), len(tt.urls))
			}

			validCount := 0
			invalidCount := 0
			for _, err := range results {
				if err == nil {
					validCount++
				} else {
					invalidCount++
				}
			}

			if validCount != tt.wantValidLen {
				t.Errorf("valid count = %d, want %d", validCount, tt.wantValidLen)
			}
			if invalidCount != tt.wantInvalidLen {
				t.Errorf("invalid count = %d, want %d", invalidCount, tt.wantInvalidLen)
			}
		})
	}
}

func TestValidateURLs_ErrorMapping(t *testing.T) {
	t.Parallel()

	urls := []string{
		"https://www.mercadolivre.com.br/p/MLB1",
		"https://www.amazon.com.br/dp/B123",
	}

	results := ValidateURLs(urls)

	// Valid URL has nil error
	if results[urls[0]] != nil {
		t.Errorf("expected nil error for valid URL, got %v", results[urls[0]])
	}

	// Invalid URL has wrapped ErrInvalidURL
	if results[urls[1]] == nil {
		t.Fatal("expected error for invalid URL, got nil")
	}
	assertWrappedError(t, results[urls[1]], apperrors.ErrInvalidURL)
}

// ---------------------------------------------------------------------------
// ValidateAndCategorizeURLs
// ---------------------------------------------------------------------------

func TestValidateAndCategorizeURLs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		urls           []string
		wantValid      int
		wantInvalid    int
		wantValidURLs  []string
		wantInvalidKeys []string
	}{
		{
			name: "categorizes valid and invalid",
			urls: []string{
				"https://www.mercadolivre.com.br/p/MLB1",
				"https://www.amazon.com.br/dp/B123",
				"https://www.mercadolivre.com.br/p/MLB2",
				"invalid",
			},
			wantValid:   2,
			wantInvalid: 2,
			wantValidURLs: []string{
				"https://www.mercadolivre.com.br/p/MLB1",
				"https://www.mercadolivre.com.br/p/MLB2",
			},
			wantInvalidKeys: []string{
				"https://www.amazon.com.br/dp/B123",
				"invalid",
			},
		},
		{
			name: "all valid",
			urls: []string{
				"https://www.mercadolivre.com.br/p/MLB1",
				"https://www.mercadolibre.com.ar/p/MLA1",
			},
			wantValid:   2,
			wantInvalid: 0,
		},
		{
			name: "all invalid",
			urls: []string{
				"https://www.google.com",
				"",
			},
			wantValid:   0,
			wantInvalid: 2,
		},
		{
			name:        "empty input",
			urls:        []string{},
			wantValid:   0,
			wantInvalid: 0,
		},
		{
			name:        "nil input",
			urls:        nil,
			wantValid:   0,
			wantInvalid: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := ValidateAndCategorizeURLs(tt.urls)

			if result == nil {
				t.Fatal("ValidateAndCategorizeURLs() returned nil")
			}
			if len(result.ValidURLs) != tt.wantValid {
				t.Errorf("ValidURLs count = %d, want %d", len(result.ValidURLs), tt.wantValid)
			}
			if len(result.InvalidURLs) != tt.wantInvalid {
				t.Errorf("InvalidURLs count = %d, want %d", len(result.InvalidURLs), tt.wantInvalid)
			}

			// Verify exact valid URLs when specified
			if tt.wantValidURLs != nil {
				for i, want := range tt.wantValidURLs {
					if i >= len(result.ValidURLs) {
						t.Errorf("missing valid URL at index %d: %q", i, want)
						continue
					}
					if result.ValidURLs[i] != want {
						t.Errorf("ValidURLs[%d] = %q, want %q", i, result.ValidURLs[i], want)
					}
				}
			}

			// Verify invalid URL keys when specified
			if tt.wantInvalidKeys != nil {
				for _, key := range tt.wantInvalidKeys {
					if _, ok := result.InvalidURLs[key]; !ok {
						t.Errorf("expected %q in InvalidURLs map", key)
					}
				}
			}
		})
	}
}

func TestValidateAndCategorizeURLs_ErrorMessages(t *testing.T) {
	t.Parallel()

	result := ValidateAndCategorizeURLs([]string{
		"https://www.amazon.com.br/dp/B123",
		"",
	})

	// Invalid URLs have non-empty error messages
	for url, msg := range result.InvalidURLs {
		if msg == "" {
			t.Errorf("InvalidURLs[%q] has empty error message", url)
		}
	}
}

// ---------------------------------------------------------------------------
// ValidationResult struct
// ---------------------------------------------------------------------------

func TestValidationResult_InitializedSlicesAndMaps(t *testing.T) {
	t.Parallel()

	result := ValidateAndCategorizeURLs([]string{})

	// Ensure slices/maps are initialized (not nil)
	if result.ValidURLs == nil {
		t.Error("ValidURLs should be initialized, got nil")
	}
	if result.InvalidURLs == nil {
		t.Error("InvalidURLs should be initialized, got nil")
	}
}

// ---------------------------------------------------------------------------
// Error type assertions (MLLinksError)
// ---------------------------------------------------------------------------

func TestValidateMercadoLivreURL_ErrorType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		url       string
		wantCode  string
		wantField string
	}{
		{
			name:      "empty URL error details",
			url:       "",
			wantCode:  apperrors.ErrInvalidURL.Error(),
			wantField: "origin_url",
		},
		{
			name:      "wrong domain error details",
			url:       "https://www.amazon.com.br/dp/B1",
			wantCode:  apperrors.ErrInvalidURL.Error(),
			wantField: "origin_url",
		},
		{
			name:      "no product indicator error details",
			url:       "https://www.mercadolivre.com.br/categorias",
			wantCode:  apperrors.ErrInvalidURL.Error(),
			wantField: "origin_url",
		},
		{
			name:      "parse error details",
			url:       "http://[::1",
			wantCode:  apperrors.ErrInvalidURLFormat.Error(),
			wantField: "origin_url",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateMercadoLivreURL(tt.url)
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			var mlErr *apperrors.MLLinksError
			if !errors.As(err, &mlErr) {
				t.Fatalf("expected *MLLinksError, got %T", err)
			}

			if mlErr.Code != tt.wantCode {
				t.Errorf("Code = %q, want %q", mlErr.Code, tt.wantCode)
			}
			if mlErr.Field != tt.wantField {
				t.Errorf("Field = %q, want %q", mlErr.Field, tt.wantField)
			}
			if mlErr.Message == "" {
				t.Error("Message should not be empty")
			}
		})
	}
}

func TestNormalizeURL_ErrorType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		url       string
		wantCode  string
		wantField string
	}{
		{
			name:      "empty URL error details",
			url:       "",
			wantCode:  apperrors.ErrInvalidURLFormat.Error(),
			wantField: "origin_url",
		},
		{
			name:      "parse error details",
			url:       "http://[::1",
			wantCode:  apperrors.ErrInvalidURLFormat.Error(),
			wantField: "origin_url",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := NormalizeURL(tt.url)
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			var mlErr *apperrors.MLLinksError
			if !errors.As(err, &mlErr) {
				t.Fatalf("expected *MLLinksError, got %T", err)
			}

			if mlErr.Code != tt.wantCode {
				t.Errorf("Code = %q, want %q", mlErr.Code, tt.wantCode)
			}
			if mlErr.Field != tt.wantField {
				t.Errorf("Field = %q, want %q", mlErr.Field, tt.wantField)
			}
			if mlErr.Message == "" {
				t.Error("Message should not be empty")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func assertWrappedError(t *testing.T, err error, target error) {
	t.Helper()
	if !errors.Is(err, target) {
		t.Errorf("expected error to wrap %v, got %v", target, err)
	}
}

func assertFieldError(t *testing.T, err error, wantField string) {
	t.Helper()
	var mlErr *apperrors.MLLinksError
	if !errors.As(err, &mlErr) {
		t.Errorf("expected *MLLinksError, got %T", err)
		return
	}
	if mlErr.Field != wantField {
		t.Errorf("Field = %q, want %q", mlErr.Field, wantField)
	}
}
