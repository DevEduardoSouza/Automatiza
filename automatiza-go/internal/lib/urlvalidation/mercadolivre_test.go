package urlvalidation

import (
	"testing"

	apperrors "automatiza-go/internal/errors"
)

func TestValidateMercadoLivreURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
		errCode string
	}{
		{
			name:    "valid URL with /p/ path",
			url:     "https://www.mercadolivre.com.br/p/MLB12345678",
			wantErr: false,
		},
		{
			name:    "valid URL with mlb code",
			url:     "https://produto.mercadolivre.com.br/MLB-12345678-produto-nome",
			wantErr: false,
		},
		{
			name:    "valid URL with mlb lowercase",
			url:     "https://www.mercadolivre.com.br/produto-mlb12345678",
			wantErr: false,
		},
		{
			name:    "valid URL mercadolibre (Spanish) with MLA",
			url:     "https://www.mercadolibre.com.ar/p/MLA12345678",
			wantErr: false, // mercadolibre domain is valid, MLA is similar to MLB
		},
		{
			name:    "invalid URL - wrong domain",
			url:     "https://www.amazon.com.br/dp/B12345",
			wantErr: true,
			errCode: apperrors.ErrInvalidURL.Error(),
		},
		{
			name:    "invalid URL - no product indicator",
			url:     "https://www.mercadolivre.com.br/categorias",
			wantErr: true,
			errCode: apperrors.ErrInvalidURL.Error(),
		},
		{
			name:    "invalid URL - empty",
			url:     "",
			wantErr: true,
			errCode: apperrors.ErrInvalidURL.Error(),
		},
		{
			name:    "invalid URL - whitespace only",
			url:     "   ",
			wantErr: true,
			errCode: apperrors.ErrInvalidURL.Error(),
		},
		{
			name:    "valid URL with query params",
			url:     "https://www.mercadolivre.com.br/p/MLB12345678?variation=123",
			wantErr: false,
		},
		{
			name:    "valid URL with hash fragment",
			url:     "https://www.mercadolivre.com.br/p/MLB12345678#reviews",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMercadoLivreURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateMercadoLivreURL() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		want    string
		wantErr bool
	}{
		{
			name:    "URL with hash fragment",
			url:     "https://www.mercadolivre.com.br/produto#reviews",
			want:    "https://www.mercadolivre.com.br/produto",
			wantErr: false,
		},
		{
			name:    "URL without hash",
			url:     "https://www.mercadolivre.com.br/produto",
			want:    "https://www.mercadolivre.com.br/produto",
			wantErr: false,
		},
		{
			name:    "URL with query and hash",
			url:     "https://www.mercadolivre.com.br/produto?v=1#section",
			want:    "https://www.mercadolivre.com.br/produto?v=1",
			wantErr: false,
		},
		{
			name:    "URL with whitespace",
			url:     "  https://www.mercadolivre.com.br/produto  ",
			want:    "https://www.mercadolivre.com.br/produto",
			wantErr: false,
		},
		{
			name:    "empty URL",
			url:     "",
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("NormalizeURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("NormalizeURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDeduplicateURLs(t *testing.T) {
	tests := []struct {
		name string
		urls []string
		want int // expected count of unique URLs
	}{
		{
			name: "no duplicates",
			urls: []string{
				"https://www.mercadolivre.com.br/p/MLB1",
				"https://www.mercadolivre.com.br/p/MLB2",
				"https://www.mercadolivre.com.br/p/MLB3",
			},
			want: 3,
		},
		{
			name: "with exact duplicates",
			urls: []string{
				"https://www.mercadolivre.com.br/p/MLB1",
				"https://www.mercadolivre.com.br/p/MLB1",
				"https://www.mercadolivre.com.br/p/MLB2",
			},
			want: 2,
		},
		{
			name: "with hash fragment duplicates",
			urls: []string{
				"https://www.mercadolivre.com.br/p/MLB1",
				"https://www.mercadolivre.com.br/p/MLB1#reviews",
				"https://www.mercadolivre.com.br/p/MLB2",
			},
			want: 2,
		},
		{
			name: "empty list",
			urls: []string{},
			want: 0,
		},
		{
			name: "with parseable non-ML URLs (kept)",
			urls: []string{
				"https://www.mercadolivre.com.br/p/MLB1",
				"https://other-site.com/page",
				"https://www.mercadolivre.com.br/p/MLB2",
			},
			want: 3, // All parseable URLs are kept, only unparseable ones are skipped
		},
		{
			name: "with empty URL (skipped)",
			urls: []string{
				"https://www.mercadolivre.com.br/p/MLB1",
				"",
				"https://www.mercadolivre.com.br/p/MLB2",
			},
			want: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DeduplicateURLs(tt.urls)
			if len(got) != tt.want {
				t.Errorf("DeduplicateURLs() returned %d URLs, want %d", len(got), tt.want)
			}
		})
	}
}

func TestValidateAndNormalizeURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{
			name:    "valid URL",
			url:     "https://www.mercadolivre.com.br/p/MLB12345678#reviews",
			wantErr: false,
		},
		{
			name:    "invalid domain",
			url:     "https://www.amazon.com.br/dp/B123",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateAndNormalizeURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateAndNormalizeURL() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateAndCategorizeURLs(t *testing.T) {
	urls := []string{
		"https://www.mercadolivre.com.br/p/MLB1",
		"https://www.amazon.com.br/dp/B123",
		"https://www.mercadolivre.com.br/p/MLB2",
		"invalid",
	}

	result := ValidateAndCategorizeURLs(urls)

	if len(result.ValidURLs) != 2 {
		t.Errorf("Expected 2 valid URLs, got %d", len(result.ValidURLs))
	}

	if len(result.InvalidURLs) != 2 {
		t.Errorf("Expected 2 invalid URLs, got %d", len(result.InvalidURLs))
	}
}
