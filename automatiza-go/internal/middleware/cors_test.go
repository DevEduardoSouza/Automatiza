package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// ---------------------------------------------------------------------------
// DefaultCORSConfig
// ---------------------------------------------------------------------------

func TestDefaultCORSConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		corsOrigin      string
		isDev           bool
		expectedOrigins []string
	}{
		{
			name:            "with empty origin",
			corsOrigin:      "",
			isDev:           false,
			expectedOrigins: []string{},
		},
		{
			name:            "with single origin",
			corsOrigin:      "https://example.com",
			isDev:           false,
			expectedOrigins: []string{"https://example.com"},
		},
		{
			name:            "with comma-separated origins",
			corsOrigin:      "https://a.com,https://b.com,https://c.com",
			isDev:           false,
			expectedOrigins: []string{"https://a.com", "https://b.com", "https://c.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := DefaultCORSConfig(tt.corsOrigin, tt.isDev)

			if len(cfg.AllowedOrigins) != len(tt.expectedOrigins) {
				t.Fatalf("expected %d origins, got %d", len(tt.expectedOrigins), len(cfg.AllowedOrigins))
			}
			for i, expected := range tt.expectedOrigins {
				if cfg.AllowedOrigins[i] != expected {
					t.Errorf("origin[%d]: expected %q, got %q", i, expected, cfg.AllowedOrigins[i])
				}
			}
			if !cfg.AllowCredentials {
				t.Error("expected AllowCredentials=true by default")
			}
			if cfg.MaxAge != 86400 {
				t.Errorf("expected MaxAge=86400, got %d", cfg.MaxAge)
			}
			if len(cfg.AllowedMethods) == 0 {
				t.Error("expected AllowedMethods to be populated")
			}
			if len(cfg.AllowedHeaders) == 0 {
				t.Error("expected AllowedHeaders to be populated")
			}
			if len(cfg.ExposedHeaders) == 0 {
				t.Error("expected ExposedHeaders to be populated")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// CORSMiddleware (Gin)
// ---------------------------------------------------------------------------

func TestCORSMiddleware(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		cfg                *CORSConfig
		origin             string
		method             string
		expectAllowOrigin  string
		expectCredentials  bool
		expectMaxAge       bool
		expectStatus       int
		expectNextCalled   bool
	}{
		{
			name: "no origin header - next handler called, no CORS headers",
			cfg: &CORSConfig{
				AllowedOrigins:   []string{"https://example.com"},
				AllowedMethods:   []string{"GET"},
				AllowCredentials: true,
				MaxAge:           3600,
			},
			origin:            "",
			method:            http.MethodGet,
			expectAllowOrigin: "",
			expectCredentials: false,
			expectMaxAge:      false,
			expectStatus:      http.StatusOK,
			expectNextCalled:  true,
		},
		{
			name: "dev mode - http://localhost allowed",
			cfg: &CORSConfig{
				AllowedOrigins:   []string{},
				AllowedMethods:   []string{"GET"},
				AllowedHeaders:   []string{"Content-Type"},
				ExposedHeaders:   []string{"Content-Length"},
				AllowCredentials: true,
				MaxAge:           3600,
				IsDevelopment:    true,
			},
			origin:            "http://localhost:3003",
			method:            http.MethodGet,
			expectAllowOrigin: "http://localhost:3003",
			expectCredentials: true,
			expectMaxAge:      true,
			expectStatus:      http.StatusOK,
			expectNextCalled:  true,
		},
		{
			name: "dev mode - https://localhost allowed",
			cfg: &CORSConfig{
				AllowedOrigins: []string{},
				AllowedMethods: []string{"GET"},
				AllowedHeaders: []string{},
				ExposedHeaders: []string{},
				AllowCredentials: true,
				MaxAge:           100,
				IsDevelopment:    true,
			},
			origin:            "https://localhost:3000",
			method:            http.MethodGet,
			expectAllowOrigin: "https://localhost:3000",
			expectCredentials: true,
			expectMaxAge:      true,
			expectStatus:      http.StatusOK,
			expectNextCalled:  true,
		},
		{
			name: "dev mode - http://127.0.0.1 allowed",
			cfg: &CORSConfig{
				AllowedOrigins: []string{},
				AllowedMethods: []string{"GET"},
				AllowedHeaders: []string{},
				ExposedHeaders: []string{},
				AllowCredentials: true,
				MaxAge:           100,
				IsDevelopment:    true,
			},
			origin:            "http://127.0.0.1:8080",
			method:            http.MethodGet,
			expectAllowOrigin: "http://127.0.0.1:8080",
			expectCredentials: true,
			expectMaxAge:      true,
			expectStatus:      http.StatusOK,
			expectNextCalled:  true,
		},
		{
			name: "exact origin match - CORS headers set",
			cfg: &CORSConfig{
				AllowedOrigins:   []string{"https://myapp.com"},
				AllowedMethods:   []string{"GET", "POST"},
				AllowedHeaders:   []string{"Authorization"},
				ExposedHeaders:   []string{"Content-Length"},
				AllowCredentials: true,
				MaxAge:           7200,
			},
			origin:            "https://myapp.com",
			method:            http.MethodGet,
			expectAllowOrigin: "https://myapp.com",
			expectCredentials: true,
			expectMaxAge:      true,
			expectStatus:      http.StatusOK,
			expectNextCalled:  true,
		},
		{
			name: "wildcard subdomain - matches sub.example.com",
			cfg: &CORSConfig{
				AllowedOrigins:   []string{"*.example.com"},
				AllowedMethods:   []string{"GET"},
				AllowedHeaders:   []string{},
				ExposedHeaders:   []string{},
				AllowCredentials: true,
				MaxAge:           100,
			},
			origin:            "https://sub.example.com",
			method:            http.MethodGet,
			expectAllowOrigin: "https://sub.example.com",
			expectCredentials: true,
			expectMaxAge:      true,
			expectStatus:      http.StatusOK,
			expectNextCalled:  true,
		},
		{
			name: "disallowed origin - no CORS headers",
			cfg: &CORSConfig{
				AllowedOrigins:   []string{"https://allowed.com"},
				AllowedMethods:   []string{"GET"},
				AllowCredentials: true,
				MaxAge:           3600,
			},
			origin:            "https://evil.com",
			method:            http.MethodGet,
			expectAllowOrigin: "",
			expectCredentials: false,
			expectMaxAge:      false,
			expectStatus:      http.StatusOK,
			expectNextCalled:  true,
		},
		{
			name: "OPTIONS preflight - 204 No Content",
			cfg: &CORSConfig{
				AllowedOrigins:   []string{"https://myapp.com"},
				AllowedMethods:   []string{"GET", "POST"},
				AllowedHeaders:   []string{"Authorization"},
				ExposedHeaders:   []string{},
				AllowCredentials: true,
				MaxAge:           3600,
			},
			origin:            "https://myapp.com",
			method:            http.MethodOptions,
			expectAllowOrigin: "https://myapp.com",
			expectCredentials: true,
			expectMaxAge:      true,
			expectStatus:      http.StatusNoContent,
			expectNextCalled:  false,
		},
		{
			name: "AllowCredentials false - no credentials header",
			cfg: &CORSConfig{
				AllowedOrigins:   []string{"https://myapp.com"},
				AllowedMethods:   []string{"GET"},
				AllowedHeaders:   []string{},
				ExposedHeaders:   []string{},
				AllowCredentials: false,
				MaxAge:           3600,
			},
			origin:            "https://myapp.com",
			method:            http.MethodGet,
			expectAllowOrigin: "https://myapp.com",
			expectCredentials: false,
			expectMaxAge:      true,
			expectStatus:      http.StatusOK,
			expectNextCalled:  true,
		},
		{
			name: "MaxAge 0 - no max-age header",
			cfg: &CORSConfig{
				AllowedOrigins:   []string{"https://myapp.com"},
				AllowedMethods:   []string{"GET"},
				AllowedHeaders:   []string{},
				ExposedHeaders:   []string{},
				AllowCredentials: true,
				MaxAge:           0,
			},
			origin:            "https://myapp.com",
			method:            http.MethodGet,
			expectAllowOrigin: "https://myapp.com",
			expectCredentials: true,
			expectMaxAge:      false,
			expectStatus:      http.StatusOK,
			expectNextCalled:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			w := httptest.NewRecorder()
			_, engine := gin.CreateTestContext(w)

			nextCalled := false
			engine.Use(CORSMiddleware(tt.cfg))
			engine.GET("/test", func(c *gin.Context) {
				nextCalled = true
				c.Status(http.StatusOK)
			})
			// Register OPTIONS route as well since Gin needs it
			engine.OPTIONS("/test", func(c *gin.Context) {
				nextCalled = true
				c.Status(http.StatusOK)
			})

			req := httptest.NewRequest(tt.method, "/test", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}

			engine.ServeHTTP(w, req)

			if tt.expectStatus != w.Code {
				t.Errorf("expected status %d, got %d", tt.expectStatus, w.Code)
			}

			allowOrigin := w.Header().Get("Access-Control-Allow-Origin")
			if allowOrigin != tt.expectAllowOrigin {
				t.Errorf("expected Access-Control-Allow-Origin=%q, got %q", tt.expectAllowOrigin, allowOrigin)
			}

			credHeader := w.Header().Get("Access-Control-Allow-Credentials")
			if tt.expectCredentials {
				if credHeader != "true" {
					t.Errorf("expected Access-Control-Allow-Credentials=true, got %q", credHeader)
				}
			} else {
				if credHeader != "" {
					t.Errorf("expected no Access-Control-Allow-Credentials header, got %q", credHeader)
				}
			}

			maxAgeHeader := w.Header().Get("Access-Control-Max-Age")
			if tt.expectMaxAge {
				if maxAgeHeader == "" {
					t.Error("expected Access-Control-Max-Age header to be set")
				}
			} else {
				if maxAgeHeader != "" {
					t.Errorf("expected no Access-Control-Max-Age header, got %q", maxAgeHeader)
				}
			}

			// For OPTIONS with disallowed origin, next handler is still registered but
			// middleware aborts. For non-OPTIONS with disallowed origin, next is called.
			if tt.method == http.MethodOptions && tt.expectAllowOrigin != "" {
				// Preflight with allowed origin: middleware aborts, next NOT called
				if nextCalled {
					t.Error("expected next handler NOT to be called on OPTIONS preflight")
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// HTTPCORSMiddleware (standard http.Handler)
// ---------------------------------------------------------------------------

func TestHTTPCORSMiddleware(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		cfg                *CORSConfig
		origin             string
		method             string
		expectAllowOrigin  string
		expectCredentials  bool
		expectMaxAge       bool
		expectStatus       int
		expectNextCalled   bool
	}{
		{
			name: "no origin header - next called, no CORS headers",
			cfg: &CORSConfig{
				AllowedOrigins:   []string{"https://example.com"},
				AllowedMethods:   []string{"GET"},
				AllowCredentials: true,
				MaxAge:           3600,
			},
			origin:            "",
			method:            http.MethodGet,
			expectAllowOrigin: "",
			expectCredentials: false,
			expectMaxAge:      false,
			expectStatus:      http.StatusOK,
			expectNextCalled:  true,
		},
		{
			name: "dev mode - localhost allowed",
			cfg: &CORSConfig{
				AllowedOrigins:   []string{},
				AllowedMethods:   []string{"GET"},
				AllowedHeaders:   []string{"Content-Type"},
				ExposedHeaders:   []string{"Content-Length"},
				AllowCredentials: true,
				MaxAge:           3600,
				IsDevelopment:    true,
			},
			origin:            "http://localhost:3003",
			method:            http.MethodGet,
			expectAllowOrigin: "http://localhost:3003",
			expectCredentials: true,
			expectMaxAge:      true,
			expectStatus:      http.StatusOK,
			expectNextCalled:  true,
		},
		{
			name: "exact origin match",
			cfg: &CORSConfig{
				AllowedOrigins:   []string{"https://myapp.com"},
				AllowedMethods:   []string{"GET", "POST"},
				AllowedHeaders:   []string{"Authorization"},
				ExposedHeaders:   []string{"Content-Length"},
				AllowCredentials: true,
				MaxAge:           7200,
			},
			origin:            "https://myapp.com",
			method:            http.MethodGet,
			expectAllowOrigin: "https://myapp.com",
			expectCredentials: true,
			expectMaxAge:      true,
			expectStatus:      http.StatusOK,
			expectNextCalled:  true,
		},
		{
			name: "wildcard subdomain match",
			cfg: &CORSConfig{
				AllowedOrigins:   []string{"*.example.com"},
				AllowedMethods:   []string{"GET"},
				AllowedHeaders:   []string{},
				ExposedHeaders:   []string{},
				AllowCredentials: true,
				MaxAge:           100,
			},
			origin:            "https://app.example.com",
			method:            http.MethodGet,
			expectAllowOrigin: "https://app.example.com",
			expectCredentials: true,
			expectMaxAge:      true,
			expectStatus:      http.StatusOK,
			expectNextCalled:  true,
		},
		{
			name: "disallowed origin",
			cfg: &CORSConfig{
				AllowedOrigins:   []string{"https://allowed.com"},
				AllowedMethods:   []string{"GET"},
				AllowCredentials: true,
				MaxAge:           3600,
			},
			origin:            "https://evil.com",
			method:            http.MethodGet,
			expectAllowOrigin: "",
			expectCredentials: false,
			expectMaxAge:      false,
			expectStatus:      http.StatusOK,
			expectNextCalled:  true,
		},
		{
			name: "OPTIONS preflight - 204",
			cfg: &CORSConfig{
				AllowedOrigins:   []string{"https://myapp.com"},
				AllowedMethods:   []string{"GET", "POST"},
				AllowedHeaders:   []string{"Authorization"},
				ExposedHeaders:   []string{},
				AllowCredentials: true,
				MaxAge:           3600,
			},
			origin:            "https://myapp.com",
			method:            http.MethodOptions,
			expectAllowOrigin: "https://myapp.com",
			expectCredentials: true,
			expectMaxAge:      true,
			expectStatus:      http.StatusNoContent,
			expectNextCalled:  false,
		},
		{
			name: "AllowCredentials false",
			cfg: &CORSConfig{
				AllowedOrigins:   []string{"https://myapp.com"},
				AllowedMethods:   []string{"GET"},
				AllowedHeaders:   []string{},
				ExposedHeaders:   []string{},
				AllowCredentials: false,
				MaxAge:           3600,
			},
			origin:            "https://myapp.com",
			method:            http.MethodGet,
			expectAllowOrigin: "https://myapp.com",
			expectCredentials: false,
			expectMaxAge:      true,
			expectStatus:      http.StatusOK,
			expectNextCalled:  true,
		},
		{
			name: "MaxAge 0",
			cfg: &CORSConfig{
				AllowedOrigins:   []string{"https://myapp.com"},
				AllowedMethods:   []string{"GET"},
				AllowedHeaders:   []string{},
				ExposedHeaders:   []string{},
				AllowCredentials: true,
				MaxAge:           0,
			},
			origin:            "https://myapp.com",
			method:            http.MethodGet,
			expectAllowOrigin: "https://myapp.com",
			expectCredentials: true,
			expectMaxAge:      false,
			expectStatus:      http.StatusOK,
			expectNextCalled:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			nextCalled := false
			inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				w.WriteHeader(http.StatusOK)
			})

			handler := HTTPCORSMiddleware(tt.cfg)(inner)

			req := httptest.NewRequest(tt.method, "/test", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != tt.expectStatus {
				t.Errorf("expected status %d, got %d", tt.expectStatus, w.Code)
			}

			allowOrigin := w.Header().Get("Access-Control-Allow-Origin")
			if allowOrigin != tt.expectAllowOrigin {
				t.Errorf("expected Access-Control-Allow-Origin=%q, got %q", tt.expectAllowOrigin, allowOrigin)
			}

			credHeader := w.Header().Get("Access-Control-Allow-Credentials")
			if tt.expectCredentials {
				if credHeader != "true" {
					t.Errorf("expected Access-Control-Allow-Credentials=true, got %q", credHeader)
				}
			} else {
				if credHeader != "" {
					t.Errorf("expected no Access-Control-Allow-Credentials, got %q", credHeader)
				}
			}

			maxAgeHeader := w.Header().Get("Access-Control-Max-Age")
			if tt.expectMaxAge {
				if maxAgeHeader == "" {
					t.Error("expected Access-Control-Max-Age header to be set")
				}
			} else {
				if maxAgeHeader != "" {
					t.Errorf("expected no Access-Control-Max-Age, got %q", maxAgeHeader)
				}
			}

			if tt.expectNextCalled != nextCalled {
				t.Errorf("expected nextCalled=%v, got %v", tt.expectNextCalled, nextCalled)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// itoa
// ---------------------------------------------------------------------------

func TestItoa(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    int
		expected string
	}{
		{name: "zero", input: 0, expected: "0"},
		{name: "positive single digit", input: 7, expected: "7"},
		{name: "positive multi digit", input: 86400, expected: "86400"},
		{name: "negative", input: -42, expected: "-42"},
		{name: "large number", input: 2147483647, expected: "2147483647"},
		{name: "negative large", input: -999999, expected: "-999999"},
		{name: "one", input: 1, expected: "1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := itoa(tt.input)
			if got != tt.expected {
				t.Errorf("itoa(%d) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// CORSMiddleware - additional: AllowedMethods and AllowedHeaders in response
// ---------------------------------------------------------------------------

func TestCORSMiddleware_MethodsAndHeadersInResponse(t *testing.T) {
	t.Parallel()

	cfg := &CORSConfig{
		AllowedOrigins:   []string{"https://myapp.com"},
		AllowedMethods:   []string{"GET", "POST", "DELETE"},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		ExposedHeaders:   []string{"X-Custom-Header"},
		AllowCredentials: true,
		MaxAge:           3600,
	}

	w := httptest.NewRecorder()
	_, engine := gin.CreateTestContext(w)

	engine.Use(CORSMiddleware(cfg))
	engine.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://myapp.com")
	engine.ServeHTTP(w, req)

	methods := w.Header().Get("Access-Control-Allow-Methods")
	if !strings.Contains(methods, "GET") || !strings.Contains(methods, "POST") || !strings.Contains(methods, "DELETE") {
		t.Errorf("expected methods to contain GET, POST, DELETE, got %q", methods)
	}

	headers := w.Header().Get("Access-Control-Allow-Headers")
	if !strings.Contains(headers, "Authorization") || !strings.Contains(headers, "Content-Type") {
		t.Errorf("expected headers to contain Authorization, Content-Type, got %q", headers)
	}

	exposed := w.Header().Get("Access-Control-Expose-Headers")
	if !strings.Contains(exposed, "X-Custom-Header") {
		t.Errorf("expected exposed headers to contain X-Custom-Header, got %q", exposed)
	}
}
