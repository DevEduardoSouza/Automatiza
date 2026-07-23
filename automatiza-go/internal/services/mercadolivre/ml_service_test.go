package mercadolivre

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"automatiza-go/internal/lib/cache"
	"automatiza-go/internal/lib/commerceurl"
	"automatiza-go/internal/lib/crypto"
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"

	"go.uber.org/zap/zaptest"
)

// ---------------------------------------------------------------------------
// Mock implementations
// ---------------------------------------------------------------------------

type mockMLCredsRepo struct {
	GetFn  func(ctx context.Context, userID string) (*models.MLCredentialsDB, error)
	SaveFn func(ctx context.Context, userID, encryptedData string) error
}

func (m *mockMLCredsRepo) Get(ctx context.Context, userID string) (*models.MLCredentialsDB, error) {
	if m.GetFn != nil {
		return m.GetFn(ctx, userID)
	}
	return nil, fmt.Errorf("GetFn not set")
}

func (m *mockMLCredsRepo) Save(ctx context.Context, userID, encryptedData string) error {
	if m.SaveFn != nil {
		return m.SaveFn(ctx, userID, encryptedData)
	}
	return fmt.Errorf("SaveFn not set")
}

type mockMLLinkRepo struct {
	GetByOriginalURLFn func(ctx context.Context, userID, originalURL string) (*models.MLLinkEntry, error)
	CreateFn           func(ctx context.Context, link *models.MLLinkEntry) error
	GetRecentByUserFn  func(ctx context.Context, userID string, limit int) ([]models.MLLinkEntry, error)
}

func (m *mockMLLinkRepo) GetByOriginalURL(ctx context.Context, userID, originalURL string) (*models.MLLinkEntry, error) {
	if m.GetByOriginalURLFn != nil {
		return m.GetByOriginalURLFn(ctx, userID, originalURL)
	}
	return nil, nil
}

func (m *mockMLLinkRepo) Create(ctx context.Context, link *models.MLLinkEntry) error {
	if m.CreateFn != nil {
		return m.CreateFn(ctx, link)
	}
	return nil
}

func (m *mockMLLinkRepo) GetRecentByUser(ctx context.Context, userID string, limit int) ([]models.MLLinkEntry, error) {
	if m.GetRecentByUserFn != nil {
		return m.GetRecentByUserFn(ctx, userID, limit)
	}
	return nil, nil
}

// Verify mock implementations satisfy their interfaces.
var (
	_ repository.MLCredentialsRepository = (*mockMLCredsRepo)(nil)
	_ repository.MLLinkRepository        = (*mockMLLinkRepo)(nil)
)

// testEncryptionKey is a valid 32-byte AES key, base64-encoded.
var testEncryptionKey = base64.StdEncoding.EncodeToString([]byte("01234567890123456789012345678901"))

// ---------------------------------------------------------------------------
// NewMLService
// ---------------------------------------------------------------------------

func TestNewMLService(t *testing.T) {
	logger := zaptest.NewLogger(t)
	svc := NewMLService(nil, &mockMLCredsRepo{}, &mockMLLinkRepo{}, testEncryptionKey, logger)
	if svc == nil {
		t.Fatal("NewMLService returned nil")
	}
}

// ---------------------------------------------------------------------------
// ExtractProductID
// ---------------------------------------------------------------------------

func TestExtractProductID(t *testing.T) {
	logger := zaptest.NewLogger(t)
	svc := NewMLService(nil, &mockMLCredsRepo{}, &mockMLLinkRepo{}, testEncryptionKey, logger)

	tests := []struct {
		name    string
		url     string
		want    string
		wantErr bool
	}{
		{
			name: "full URL with /p/ pattern",
			url:  "https://www.mercadolivre.com.br/something/p/MLB123456",
			want: "MLB123456",
		},
		{
			name: "URL with hyphenated product ID",
			url:  "https://www.mercadolivre.com.br/something/p/MLB-123456",
			want: "MLB123456",
		},
		{
			name: "MLB pattern in URL query",
			url:  "https://example.com/product?id=MLB-789",
			want: "MLB789",
		},
		{
			name: "short /p/ pattern",
			url:  "https://www.mercadolivre.com.br/item/p/MLB987654",
			want: "MLB987654",
		},
		{
			name:    "invalid URL no product ID",
			url:     "https://www.example.com/no-product-here",
			wantErr: true,
		},
		{
			name:    "empty URL",
			url:     "",
			wantErr: true,
		},
		{
			name: "lowercase mlb in URL (uppercased internally)",
			url:  "https://www.mercadolivre.com.br/something/p/mlb555666",
			want: "MLB555666",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := svc.ExtractProductID(tt.url)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("ExtractProductID(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// normalizeURL and parseURL
// ---------------------------------------------------------------------------

func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "removes fragment",
			url:  "https://example.com/page#section",
			want: "https://example.com/page",
		},
		{
			name: "trims whitespace",
			url:  "  https://example.com/page  ",
			want: "https://example.com/page",
		},
		{
			name: "no fragment passes through",
			url:  "https://example.com/page?q=1",
			want: "https://example.com/page?q=1",
		},
		{
			name: "empty string trimmed",
			url:  "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeURL(tt.url)
			if got != tt.want {
				t.Errorf("normalizeURL(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestParseURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{name: "valid URL", url: "https://example.com", wantErr: false},
		{name: "empty URL", url: "", wantErr: false}, // url.Parse doesn't error on empty
		{name: "URL with spaces trimmed", url: "  https://example.com  ", wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseURL(tt.url)
			if tt.wantErr && err == nil {
				t.Error("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// IsConnected
// ---------------------------------------------------------------------------

func TestIsConnected(t *testing.T) {
	logger := zaptest.NewLogger(t)

	t.Run("no credentials returns false", func(t *testing.T) {
		svc := NewMLService(nil, &mockMLCredsRepo{
			GetFn: func(_ context.Context, _ string) (*models.MLCredentialsDB, error) {
				return nil, fmt.Errorf("not found")
			},
		}, &mockMLLinkRepo{}, testEncryptionKey, logger)

		connected, err := svc.IsConnected(context.Background(), "user1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if connected {
			t.Error("expected not connected")
		}
	})

	t.Run("empty cookie and csrf returns false", func(t *testing.T) {
		// Encrypt credentials with empty cookie and csrf
		creds := &models.MLCredentials{Cookie: "", CSRF: ""}
		blob, err := crypto.EncryptJSONV1(creds, testEncryptionKey)
		if err != nil {
			t.Fatalf("failed to encrypt: %v", err)
		}
		encData, _ := json.Marshal(blob)

		svc := NewMLService(nil, &mockMLCredsRepo{
			GetFn: func(_ context.Context, _ string) (*models.MLCredentialsDB, error) {
				return &models.MLCredentialsDB{EncryptedData: string(encData)}, nil
			},
		}, &mockMLLinkRepo{}, testEncryptionKey, logger)

		connected, err := svc.IsConnected(context.Background(), "user1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if connected {
			t.Error("expected not connected with empty cookie/csrf")
		}
	})

	t.Run("valid credentials returns true", func(t *testing.T) {
		creds := &models.MLCredentials{Cookie: "test-cookie", CSRF: "test-csrf"}
		blob, err := crypto.EncryptJSONV1(creds, testEncryptionKey)
		if err != nil {
			t.Fatalf("failed to encrypt: %v", err)
		}
		encData, _ := json.Marshal(blob)

		svc := NewMLService(nil, &mockMLCredsRepo{
			GetFn: func(_ context.Context, _ string) (*models.MLCredentialsDB, error) {
				return &models.MLCredentialsDB{EncryptedData: string(encData)}, nil
			},
		}, &mockMLLinkRepo{}, testEncryptionKey, logger)

		connected, err := svc.IsConnected(context.Background(), "user1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !connected {
			t.Error("expected connected")
		}
	})
}

// ---------------------------------------------------------------------------
// InvalidatePreviewCache
// ---------------------------------------------------------------------------

func TestInvalidatePreviewCache(t *testing.T) {
	logger := zaptest.NewLogger(t)

	t.Run("nil cache returns nil", func(t *testing.T) {
		svc := NewMLService(nil, &mockMLCredsRepo{}, &mockMLLinkRepo{}, testEncryptionKey, logger)
		err := svc.InvalidatePreviewCache(context.Background(), "https://www.mercadolivre.com.br/product/p/MLB123")
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
	})

	t.Run("invalid URL returns error with non-nil cache", func(t *testing.T) {
		// Use a disabled (non-nil) cache so it doesn't short-circuit at the nil check
		disabledCache := cache.NewCache(nil, "test", true)
		svc := NewMLService(disabledCache, &mockMLCredsRepo{}, &mockMLLinkRepo{}, testEncryptionKey, logger)
		err := svc.InvalidatePreviewCache(context.Background(), "https://example.com/no-product")
		if err == nil {
			t.Error("expected error for invalid ML URL")
		}
	})
}

// ---------------------------------------------------------------------------
// LoadCredentials
// ---------------------------------------------------------------------------

func TestLoadCredentials(t *testing.T) {
	logger := zaptest.NewLogger(t)

	t.Run("repo error", func(t *testing.T) {
		svc := NewMLService(nil, &mockMLCredsRepo{
			GetFn: func(_ context.Context, _ string) (*models.MLCredentialsDB, error) {
				return nil, fmt.Errorf("db error")
			},
		}, &mockMLLinkRepo{}, testEncryptionKey, logger)

		_, err := svc.LoadCredentials(context.Background(), "user1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("nil result from repo", func(t *testing.T) {
		svc := NewMLService(nil, &mockMLCredsRepo{
			GetFn: func(_ context.Context, _ string) (*models.MLCredentialsDB, error) {
				return nil, nil
			},
		}, &mockMLLinkRepo{}, testEncryptionKey, logger)

		_, err := svc.LoadCredentials(context.Background(), "user1")
		if err == nil {
			t.Fatal("expected error for nil credentials")
		}
	})

	t.Run("invalid encrypted data (unmarshal error)", func(t *testing.T) {
		svc := NewMLService(nil, &mockMLCredsRepo{
			GetFn: func(_ context.Context, _ string) (*models.MLCredentialsDB, error) {
				return &models.MLCredentialsDB{EncryptedData: "not-json"}, nil
			},
		}, &mockMLLinkRepo{}, testEncryptionKey, logger)

		_, err := svc.LoadCredentials(context.Background(), "user1")
		if err == nil {
			t.Fatal("expected error for invalid JSON")
		}
	})

	t.Run("decrypt error (wrong key)", func(t *testing.T) {
		// Encrypt with one key, decrypt with another
		creds := &models.MLCredentials{Cookie: "c", CSRF: "t"}
		otherKey := base64.StdEncoding.EncodeToString([]byte("99999999999999999999999999999999"))
		blob, err := crypto.EncryptJSONV1(creds, otherKey)
		if err != nil {
			t.Fatalf("failed to encrypt: %v", err)
		}
		encData, _ := json.Marshal(blob)

		svc := NewMLService(nil, &mockMLCredsRepo{
			GetFn: func(_ context.Context, _ string) (*models.MLCredentialsDB, error) {
				return &models.MLCredentialsDB{EncryptedData: string(encData)}, nil
			},
		}, &mockMLLinkRepo{}, testEncryptionKey, logger)

		_, err = svc.LoadCredentials(context.Background(), "user1")
		if err == nil {
			t.Fatal("expected decrypt error")
		}
	})

	t.Run("success", func(t *testing.T) {
		creds := &models.MLCredentials{Cookie: "test-cookie", CSRF: "test-csrf"}
		blob, err := crypto.EncryptJSONV1(creds, testEncryptionKey)
		if err != nil {
			t.Fatalf("failed to encrypt: %v", err)
		}
		encData, _ := json.Marshal(blob)

		svc := NewMLService(nil, &mockMLCredsRepo{
			GetFn: func(_ context.Context, _ string) (*models.MLCredentialsDB, error) {
				return &models.MLCredentialsDB{EncryptedData: string(encData)}, nil
			},
		}, &mockMLLinkRepo{}, testEncryptionKey, logger)

		loaded, err := svc.LoadCredentials(context.Background(), "user1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if loaded.Cookie != "test-cookie" {
			t.Errorf("Cookie = %q, want 'test-cookie'", loaded.Cookie)
		}
		if loaded.CSRF != "test-csrf" {
			t.Errorf("CSRF = %q, want 'test-csrf'", loaded.CSRF)
		}
	})
}

// ---------------------------------------------------------------------------
// SaveCredentials
// ---------------------------------------------------------------------------

func TestSaveCredentials(t *testing.T) {
	logger := zaptest.NewLogger(t)

	t.Run("encrypt error with invalid key", func(t *testing.T) {
		svc := NewMLService(nil, &mockMLCredsRepo{}, &mockMLLinkRepo{}, "invalid-key", logger)
		err := svc.SaveCredentials(context.Background(), "user1", "cookie", "csrf", "tag1")
		if err == nil {
			t.Fatal("expected error for invalid encryption key")
		}
	})

	t.Run("repo save error", func(t *testing.T) {
		svc := NewMLService(nil, &mockMLCredsRepo{
			SaveFn: func(_ context.Context, _, _ string) error {
				return fmt.Errorf("db save error")
			},
		}, &mockMLLinkRepo{}, testEncryptionKey, logger)

		err := svc.SaveCredentials(context.Background(), "user1", "cookie", "csrf", "tag1")
		if err == nil {
			t.Fatal("expected error from repo save")
		}
	})

	t.Run("success", func(t *testing.T) {
		var savedUserID, savedData string
		svc := NewMLService(nil, &mockMLCredsRepo{
			SaveFn: func(_ context.Context, userID, encData string) error {
				savedUserID = userID
				savedData = encData
				return nil
			},
		}, &mockMLLinkRepo{}, testEncryptionKey, logger)

		err := svc.SaveCredentials(context.Background(), "user1", "my-cookie", "my-csrf", "my-tag")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if savedUserID != "user1" {
			t.Errorf("userID = %q, want 'user1'", savedUserID)
		}
		if savedData == "" {
			t.Error("expected non-empty encrypted data")
		}

		// Verify we can decrypt the saved data
		var blob crypto.EncryptedBlobV1
		if err := json.Unmarshal([]byte(savedData), &blob); err != nil {
			t.Fatalf("failed to unmarshal blob: %v", err)
		}
		var creds models.MLCredentials
		if err := crypto.DecryptJSONV1(&blob, testEncryptionKey, &creds); err != nil {
			t.Fatalf("failed to decrypt: %v", err)
		}
		if creds.Cookie != "my-cookie" {
			t.Errorf("decrypted Cookie = %q, want 'my-cookie'", creds.Cookie)
		}
		if creds.CSRF != "my-csrf" {
			t.Errorf("decrypted CSRF = %q, want 'my-csrf'", creds.CSRF)
		}
	})
}

// ---------------------------------------------------------------------------
// GetProductPreview
// ---------------------------------------------------------------------------

func TestGetProductPreview(t *testing.T) {
	logger := zaptest.NewLogger(t)

	t.Run("invalid URL returns error", func(t *testing.T) {
		svc := NewMLService(nil, &mockMLCredsRepo{
			GetFn: func(_ context.Context, _ string) (*models.MLCredentialsDB, error) {
				return nil, fmt.Errorf("not found")
			},
		}, &mockMLLinkRepo{}, testEncryptionKey, logger)

		_, err := svc.GetProductPreview(context.Background(), "https://example.com/no-product", "user1")
		if err == nil {
			t.Error("expected error for invalid ML URL")
		}
	})

	t.Run("non-200 returns empty preview", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		svc := newServiceWithHTTPClient(t, nil, server.URL, &mockMLCredsRepo{
			GetFn: func(_ context.Context, _ string) (*models.MLCredentialsDB, error) {
				return nil, fmt.Errorf("not found")
			},
		}, &mockMLLinkRepo{})

		// Contrato NOVO (melhoria 9, §9): non-200 devolve erro TIPADO, não
		// mais "preview vazio com erro nil". Aquele colapso era a origem do
		// falso aviso de cupom — 404/429/HTML mudado viravam
		// HasProductData=false, que o offer_generate lia como campanha.
		preview, err := svc.GetProductPreview(context.Background(), "http://www.mercadolivre.com.br/p/MLB123456", "user1")
		if err == nil {
			t.Fatalf("esperava erro tipado para non-200, veio preview=%+v", preview)
		}
		var remote *RemoteError
		if !errors.As(err, &remote) {
			t.Fatalf("erro = %v, queria *RemoteError", err)
		}
		if remote.StatusCode != http.StatusNotFound {
			t.Errorf("StatusCode = %d, queria 404", remote.StatusCode)
		}
		if remote.Class != models.ErrorClassPermanentConte || remote.Reason != models.ReasonNotFound {
			t.Errorf("(%s,%s), queria (permanent_content,not_found)", remote.Class, remote.Reason)
		}
	})

	t.Run("success parses HTML", func(t *testing.T) {
		htmlPage := `<html><head>
			<meta property="og:title" content="Test Product">
			<meta property="og:image" content="https://img.com/test.jpg">
			<meta property="product:price:amount" content="99.90">
			<meta property="product:price:currency" content="BRL">
		</head><body>
			<span>30% OFF</span>
			<span>em 3x R$ 33,30</span>
		</body></html>`

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(htmlPage))
		}))
		defer server.Close()

		svc := newServiceWithHTTPClient(t, nil, server.URL, &mockMLCredsRepo{
			GetFn: func(_ context.Context, _ string) (*models.MLCredentialsDB, error) {
				return nil, fmt.Errorf("not found")
			},
		}, &mockMLLinkRepo{})

		preview, err := svc.GetProductPreview(context.Background(), "http://www.mercadolivre.com.br/product/p/MLB999888", "user1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if preview.Title != "Test Product" {
			t.Errorf("Title = %q, want 'Test Product'", preview.Title)
		}
		if preview.Image != "https://img.com/test.jpg" {
			t.Errorf("Image = %q, want 'https://img.com/test.jpg'", preview.Image)
		}
		if math.Abs(preview.Price-99.90) > 0.01 {
			t.Errorf("Price = %f, want 99.90", preview.Price)
		}
		if preview.Currency != "BRL" {
			t.Errorf("Currency = %q, want 'BRL'", preview.Currency)
		}
		if preview.DiscountPercent == nil || math.Abs(*preview.DiscountPercent-30.0) > 0.01 {
			t.Errorf("DiscountPercent = %v, want 30.0", preview.DiscountPercent)
		}
		if preview.Installments == nil {
			t.Error("expected non-nil Installments")
		}
	})
}

// ---------------------------------------------------------------------------
// CreateAffiliateLink
// ---------------------------------------------------------------------------

func TestCreateAffiliateLink(t *testing.T) {
	logger := zaptest.NewLogger(t)

	t.Run("invalid URL returns error", func(t *testing.T) {
		svc := NewMLService(nil, &mockMLCredsRepo{}, &mockMLLinkRepo{}, testEncryptionKey, logger)
		_, err := svc.CreateAffiliateLink(context.Background(), "https://example.com/no-mlb", "user1")
		if err == nil {
			t.Error("expected error for invalid ML URL")
		}
	})

	t.Run("short link de afiliado de terceiro NUNCA volta como-está", func(t *testing.T) {
		// Contrato NOVO (melhoria 9, §4.3 e §5.4.2): devolver o short recebido
		// como se fosse o link do nosso usuário repassava a comissão de um
		// terceiro assinada como nossa. Agora o short é resolvido
		// anonimamente até o produto; sem resolver, sai erro tipado e o
		// chamador retém o parágrafo — jamais emite o original.
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusGatewayTimeout)
		}))
		defer srv.Close()

		for _, u := range []string{"https://meli.la/2dKTwgt", "https://www.mercadolivre.com.br/sec/2dKTwgt"} {
			svc := newServiceWithHTTPClient(t, nil, srv.URL, &mockMLCredsRepo{
				GetFn: func(_ context.Context, _ string) (*models.MLCredentialsDB, error) {
					return nil, fmt.Errorf("creds must NOT be needed for affiliate short links")
				},
			}, &mockMLLinkRepo{})

			result, err := svc.CreateAffiliateLink(context.Background(), u, "user1")
			if err == nil {
				t.Fatalf("%s: esperava erro, veio %+v", u, result)
			}
			if result != nil {
				t.Fatalf("%s: resultado deve ser nil em falha, veio %+v", u, result)
			}
			if strings.Contains(err.Error(), u) {
				t.Fatalf("%s: erro vazou a URL original: %v", u, err)
			}
		}
	})

	t.Run("existing link in DB returns cached", func(t *testing.T) {
		svc := NewMLService(nil, &mockMLCredsRepo{
			GetFn: func(_ context.Context, _ string) (*models.MLCredentialsDB, error) {
				return nil, fmt.Errorf("not needed")
			},
		}, &mockMLLinkRepo{
			GetByOriginalURLFn: func(_ context.Context, _, _ string) (*models.MLLinkEntry, error) {
				return &models.MLLinkEntry{
					OriginalURL: "https://www.mercadolivre.com.br/product/p/MLB111222",
					ShortURL:    "https://meli.co/short",
					ProductID:   "MLB111222",
				}, nil
			},
		}, testEncryptionKey, logger)

		result, err := svc.CreateAffiliateLink(context.Background(), "https://www.mercadolivre.com.br/product/p/MLB111222", "user1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.AffiliateLink != "https://meli.co/short" {
			t.Errorf("AffiliateLink = %q, want 'https://meli.co/short'", result.AffiliateLink)
		}
	})

	t.Run("no credentials returns error", func(t *testing.T) {
		svc := NewMLService(nil, &mockMLCredsRepo{
			GetFn: func(_ context.Context, _ string) (*models.MLCredentialsDB, error) {
				return nil, fmt.Errorf("no credentials")
			},
		}, &mockMLLinkRepo{}, testEncryptionKey, logger)

		_, err := svc.CreateAffiliateLink(context.Background(), "https://www.mercadolivre.com.br/product/p/MLB333444", "user1")
		if err == nil {
			t.Error("expected error for missing credentials")
		}
	})

	t.Run("API success with short_url", func(t *testing.T) {
		apiResponse := map[string]interface{}{
			"status": 200,
			"urls": []map[string]interface{}{
				{
					"original_url": "https://www.mercadolivre.com.br/product/p/MLB555666",
					"long_url":     "https://meli.co/long-url",
					"short_url":    "https://meli.co/short-url",
				},
			},
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(apiResponse)
		}))
		defer server.Close()

		// Encrypt valid credentials
		creds := &models.MLCredentials{Cookie: "test-cookie", CSRF: "test-csrf"}
		blob, err := crypto.EncryptJSONV1(creds, testEncryptionKey)
		if err != nil {
			t.Fatalf("failed to encrypt: %v", err)
		}
		encData, _ := json.Marshal(blob)

		svc := newServiceWithHTTPClient(t, nil, server.URL, &mockMLCredsRepo{
			GetFn: func(_ context.Context, _ string) (*models.MLCredentialsDB, error) {
				return &models.MLCredentialsDB{EncryptedData: string(encData)}, nil
			},
		}, &mockMLLinkRepo{})

		result, err := svc.CreateAffiliateLink(context.Background(), "https://www.mercadolivre.com.br/product/p/MLB555666", "user1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result == nil {
			t.Fatal("expected non-nil result")
		}
		if result.AffiliateLink != "https://meli.co/short-url" {
			t.Errorf("AffiliateLink = %q, want 'https://meli.co/short-url'", result.AffiliateLink)
		}
		if result.ProductID != "MLB555666" {
			t.Errorf("ProductID = %q, want 'MLB555666'", result.ProductID)
		}
	})

	t.Run("API success fallback to long_url", func(t *testing.T) {
		apiResponse := map[string]interface{}{
			"status": 200,
			"urls": []map[string]interface{}{
				{
					"original_url": "https://www.mercadolivre.com.br/product/p/MLB777888",
					"long_url":     "https://meli.co/long-only",
					"short_url":    "",
				},
			},
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(apiResponse)
		}))
		defer server.Close()

		creds := &models.MLCredentials{Cookie: "test-cookie", CSRF: "test-csrf"}
		blob, err := crypto.EncryptJSONV1(creds, testEncryptionKey)
		if err != nil {
			t.Fatalf("failed to encrypt: %v", err)
		}
		encData, _ := json.Marshal(blob)

		svc := newServiceWithHTTPClient(t, nil, server.URL, &mockMLCredsRepo{
			GetFn: func(_ context.Context, _ string) (*models.MLCredentialsDB, error) {
				return &models.MLCredentialsDB{EncryptedData: string(encData)}, nil
			},
		}, &mockMLLinkRepo{})

		result, err := svc.CreateAffiliateLink(context.Background(), "https://www.mercadolivre.com.br/product/p/MLB777888", "user1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.AffiliateLink != "https://meli.co/long-only" {
			t.Errorf("AffiliateLink = %q, want 'https://meli.co/long-only'", result.AffiliateLink)
		}
	})

	t.Run("API returns non-200 status", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"internal"}`))
		}))
		defer server.Close()

		creds := &models.MLCredentials{Cookie: "test-cookie", CSRF: "test-csrf"}
		blob, err := crypto.EncryptJSONV1(creds, testEncryptionKey)
		if err != nil {
			t.Fatalf("failed to encrypt: %v", err)
		}
		encData, _ := json.Marshal(blob)

		svc := newServiceWithHTTPClient(t, nil, server.URL, &mockMLCredsRepo{
			GetFn: func(_ context.Context, _ string) (*models.MLCredentialsDB, error) {
				return &models.MLCredentialsDB{EncryptedData: string(encData)}, nil
			},
		}, &mockMLLinkRepo{})

		_, err = svc.CreateAffiliateLink(context.Background(), "https://www.mercadolivre.com.br/product/p/MLB000111", "user1")
		if err == nil {
			t.Error("expected error for non-200 response")
		}
	})

	t.Run("API returns empty urls", func(t *testing.T) {
		apiResponse := map[string]interface{}{
			"status": 200,
			"urls":   []map[string]interface{}{},
		}
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(apiResponse)
		}))
		defer server.Close()

		creds := &models.MLCredentials{Cookie: "test-cookie", CSRF: "test-csrf"}
		blob, err := crypto.EncryptJSONV1(creds, testEncryptionKey)
		if err != nil {
			t.Fatalf("failed to encrypt: %v", err)
		}
		encData, _ := json.Marshal(blob)

		svc := newServiceWithHTTPClient(t, nil, server.URL, &mockMLCredsRepo{
			GetFn: func(_ context.Context, _ string) (*models.MLCredentialsDB, error) {
				return &models.MLCredentialsDB{EncryptedData: string(encData)}, nil
			},
		}, &mockMLLinkRepo{})

		_, err = svc.CreateAffiliateLink(context.Background(), "https://www.mercadolivre.com.br/product/p/MLB222333", "user1")
		if err == nil {
			t.Error("expected error for empty urls")
		}
	})
}

// ---------------------------------------------------------------------------
// Helper: create mlService with custom HTTP client (for httptest)
// ---------------------------------------------------------------------------

// redirectAllTransport redirects ALL outgoing HTTP requests to the test server URL.
// This is needed because createShortLink and RefreshCSRFToken use hardcoded URLs.
type redirectAllTransport struct {
	targetURL string
}

func (t *redirectAllTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	newReq := req.Clone(req.Context())
	parsed, _ := parseURL(t.targetURL + req.URL.Path)
	newReq.URL = parsed
	return http.DefaultTransport.RoundTrip(newReq)
}

func newServiceWithHTTPClient(
	t *testing.T,
	_ *http.Client, // ignored, we build our own with redirectAllTransport
	serverURL string,
	credsRepo repository.MLCredentialsRepository,
	linkRepo repository.MLLinkRepository,
) Service {
	t.Helper()
	logger := zaptest.NewLogger(t)
	client := &http.Client{
		Transport: &redirectAllTransport{targetURL: serverURL},
	}
	// O cliente endurecido resolve host REAL do ML; o dialer injetado manda a
	// conexão para o servidor de teste. Assim o teste exercita a validação de
	// host de verdade em vez de contorná-la com uma URL de loopback.
	addr := strings.TrimPrefix(strings.TrimPrefix(serverURL, "http://"), "https://")
	svc := &mlService{
		httpClient: client,
		safe: newSafeMLClients(commerceurl.ClientOptions{
			DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, network, addr)
			},
		}),
		cache:         nil,
		mlCredsRepo:   credsRepo,
		mlLinkRepo:    linkRepo,
		encryptionKey: testEncryptionKey,
		logger:        logger,
	}
	return svc
}
