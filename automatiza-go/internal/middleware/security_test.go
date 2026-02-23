package middleware

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// ---------------------------------------------------------------------------
// DefaultSecurityHeadersConfig
// ---------------------------------------------------------------------------

func TestDefaultSecurityHeadersConfig(t *testing.T) {
	t.Parallel()

	cfg := DefaultSecurityHeadersConfig()

	tests := []struct {
		name  string
		check func(t *testing.T)
	}{
		{name: "EnableHSTS is true", check: func(t *testing.T) {
			t.Helper()
			if !cfg.EnableHSTS {
				t.Error("expected EnableHSTS=true")
			}
		}},
		{name: "HSTSMaxAge is 1 year", check: func(t *testing.T) {
			t.Helper()
			if cfg.HSTSMaxAge != 31536000 {
				t.Errorf("expected HSTSMaxAge=31536000, got %d", cfg.HSTSMaxAge)
			}
		}},
		{name: "HSTSIncludeSubDomains is true", check: func(t *testing.T) {
			t.Helper()
			if !cfg.HSTSIncludeSubDomains {
				t.Error("expected HSTSIncludeSubDomains=true")
			}
		}},
		{name: "EnableFrameOptions is true", check: func(t *testing.T) {
			t.Helper()
			if !cfg.EnableFrameOptions {
				t.Error("expected EnableFrameOptions=true")
			}
		}},
		{name: "FrameOptionsValue is DENY", check: func(t *testing.T) {
			t.Helper()
			if cfg.FrameOptionsValue != "DENY" {
				t.Errorf("expected FrameOptionsValue=DENY, got %q", cfg.FrameOptionsValue)
			}
		}},
		{name: "EnableXSSProtection is true", check: func(t *testing.T) {
			t.Helper()
			if !cfg.EnableXSSProtection {
				t.Error("expected EnableXSSProtection=true")
			}
		}},
		{name: "EnableContentTypeNoSniff is true", check: func(t *testing.T) {
			t.Helper()
			if !cfg.EnableContentTypeNoSniff {
				t.Error("expected EnableContentTypeNoSniff=true")
			}
		}},
		{name: "EnableReferrerPolicy is true", check: func(t *testing.T) {
			t.Helper()
			if !cfg.EnableReferrerPolicy {
				t.Error("expected EnableReferrerPolicy=true")
			}
		}},
		{name: "ReferrerPolicyValue is correct", check: func(t *testing.T) {
			t.Helper()
			if cfg.ReferrerPolicyValue != "strict-origin-when-cross-origin" {
				t.Errorf("expected ReferrerPolicyValue=strict-origin-when-cross-origin, got %q", cfg.ReferrerPolicyValue)
			}
		}},
		{name: "EnablePermissionsPolicy is true", check: func(t *testing.T) {
			t.Helper()
			if !cfg.EnablePermissionsPolicy {
				t.Error("expected EnablePermissionsPolicy=true")
			}
		}},
		{name: "PermissionsPolicyValue is set", check: func(t *testing.T) {
			t.Helper()
			if cfg.PermissionsPolicyValue == "" {
				t.Error("expected PermissionsPolicyValue to be non-empty")
			}
		}},
		{name: "EnableCSP is true", check: func(t *testing.T) {
			t.Helper()
			if !cfg.EnableCSP {
				t.Error("expected EnableCSP=true")
			}
		}},
		{name: "CSPDirectives is nil", check: func(t *testing.T) {
			t.Helper()
			if cfg.CSPDirectives != nil {
				t.Error("expected CSPDirectives=nil")
			}
		}},
		{name: "CSPReportOnly is false", check: func(t *testing.T) {
			t.Helper()
			if cfg.CSPReportOnly {
				t.Error("expected CSPReportOnly=false")
			}
		}},
		{name: "CSPReportURI is empty", check: func(t *testing.T) {
			t.Helper()
			if cfg.CSPReportURI != "" {
				t.Errorf("expected CSPReportURI=\"\", got %q", cfg.CSPReportURI)
			}
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.check(t)
		})
	}
}

// ---------------------------------------------------------------------------
// DefaultCSPDirectives
// ---------------------------------------------------------------------------

func TestDefaultCSPDirectives(t *testing.T) {
	t.Parallel()

	directives := DefaultCSPDirectives()

	expectedKeys := []string{
		"default-src", "script-src", "style-src", "img-src",
		"font-src", "connect-src", "frame-ancestors", "base-uri",
		"form-action", "object-src", "upgrade-insecure-requests",
	}

	for _, key := range expectedKeys {
		t.Run("has key "+key, func(t *testing.T) {
			t.Parallel()
			if _, ok := directives[key]; !ok {
				t.Errorf("expected key %q in DefaultCSPDirectives", key)
			}
		})
	}

	t.Run("script-src contains __CSP_NONCE__ placeholder", func(t *testing.T) {
		t.Parallel()
		scriptSrc, ok := directives["script-src"]
		if !ok {
			t.Fatal("missing script-src directive")
		}
		if !strings.Contains(scriptSrc, "__CSP_NONCE__") {
			t.Errorf("expected script-src to contain __CSP_NONCE__, got %q", scriptSrc)
		}
	})
}

// ---------------------------------------------------------------------------
// generateNonce
// ---------------------------------------------------------------------------

func TestGenerateNonce(t *testing.T) {
	t.Parallel()

	t.Run("returns base64 string", func(t *testing.T) {
		t.Parallel()
		nonce, err := generateNonce()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Verify it's valid base64
		decoded, err := base64.StdEncoding.DecodeString(nonce)
		if err != nil {
			t.Fatalf("expected valid base64, got decode error: %v", err)
		}
		if len(decoded) != 16 {
			t.Errorf("expected 16 decoded bytes, got %d", len(decoded))
		}
	})

	t.Run("24 chars long (16 bytes base64)", func(t *testing.T) {
		t.Parallel()
		nonce, err := generateNonce()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// 16 bytes -> base64 = ceil(16/3)*4 = 24 chars (with padding)
		if len(nonce) != 24 {
			t.Errorf("expected nonce length 24, got %d (%q)", len(nonce), nonce)
		}
	})

	t.Run("different each call", func(t *testing.T) {
		t.Parallel()
		nonce1, err1 := generateNonce()
		nonce2, err2 := generateNonce()
		if err1 != nil || err2 != nil {
			t.Fatalf("unexpected errors: %v, %v", err1, err2)
		}
		if nonce1 == nonce2 {
			t.Error("expected different nonces on each call")
		}
	})
}

// ---------------------------------------------------------------------------
// buildCSPHeader
// ---------------------------------------------------------------------------

func TestBuildCSPHeader(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		directives map[string]string
		nonce      string
		reportURI  string
		check      func(t *testing.T, header string)
	}{
		{
			name:       "nil directives uses defaults",
			directives: nil,
			nonce:      "test-nonce-abc",
			reportURI:  "",
			check: func(t *testing.T, header string) {
				t.Helper()
				// Should contain script-src with the nonce
				if !strings.Contains(header, "nonce-test-nonce-abc") {
					t.Errorf("expected header to contain nonce, got %q", header)
				}
				// Should contain default-src
				if !strings.Contains(header, "default-src") {
					t.Errorf("expected header to contain default-src, got %q", header)
				}
			},
		},
		{
			name: "replaces __CSP_NONCE__ with actual nonce",
			directives: map[string]string{
				"script-src": "'nonce-__CSP_NONCE__' 'strict-dynamic'",
			},
			nonce:     "my-real-nonce",
			reportURI: "",
			check: func(t *testing.T, header string) {
				t.Helper()
				if strings.Contains(header, "__CSP_NONCE__") {
					t.Error("expected __CSP_NONCE__ to be replaced")
				}
				if !strings.Contains(header, "nonce-my-real-nonce") {
					t.Errorf("expected replaced nonce in header, got %q", header)
				}
			},
		},
		{
			name: "empty value directive",
			directives: map[string]string{
				"upgrade-insecure-requests": "",
			},
			nonce:     "nonce123",
			reportURI: "",
			check: func(t *testing.T, header string) {
				t.Helper()
				// Directive with empty value should appear alone (no trailing space)
				if !strings.Contains(header, "upgrade-insecure-requests") {
					t.Errorf("expected upgrade-insecure-requests in header, got %q", header)
				}
			},
		},
		{
			name: "report-uri appended",
			directives: map[string]string{
				"default-src": "'self'",
			},
			nonce:     "nonce123",
			reportURI: "https://report.example.com/csp",
			check: func(t *testing.T, header string) {
				t.Helper()
				if !strings.Contains(header, "report-uri https://report.example.com/csp") {
					t.Errorf("expected report-uri in header, got %q", header)
				}
			},
		},
		{
			name: "no report-uri when empty",
			directives: map[string]string{
				"default-src": "'self'",
			},
			nonce:     "nonce123",
			reportURI: "",
			check: func(t *testing.T, header string) {
				t.Helper()
				if strings.Contains(header, "report-uri") {
					t.Errorf("did not expect report-uri in header, got %q", header)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			header := buildCSPHeader(tt.directives, tt.nonce, tt.reportURI)
			tt.check(t, header)
		})
	}
}

// ---------------------------------------------------------------------------
// GetCSPNonce
// ---------------------------------------------------------------------------

func TestGetCSPNonce(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		ctx      context.Context
		expected string
	}{
		{
			name:     "returns nonce from context",
			ctx:      context.WithValue(context.Background(), CSPNonceKey, "abc123nonce"),
			expected: "abc123nonce",
		},
		{
			name:     "returns empty from empty context",
			ctx:      context.Background(),
			expected: "",
		},
		{
			name:     "returns empty when wrong type in context",
			ctx:      context.WithValue(context.Background(), CSPNonceKey, 12345),
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := GetCSPNonce(tt.ctx)
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// GetCSPNonceFromGin
// ---------------------------------------------------------------------------

func TestGetCSPNonceFromGin(t *testing.T) {
	t.Parallel()

	t.Run("returns nonce from Gin context", func(t *testing.T) {
		t.Parallel()

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		ctx := context.WithValue(context.Background(), CSPNonceKey, "gin-nonce-xyz")
		c.Request = httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)

		got := GetCSPNonceFromGin(c)
		if got != "gin-nonce-xyz" {
			t.Errorf("expected %q, got %q", "gin-nonce-xyz", got)
		}
	})

	t.Run("returns empty when no nonce in Gin context", func(t *testing.T) {
		t.Parallel()

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

		got := GetCSPNonceFromGin(c)
		if got != "" {
			t.Errorf("expected empty string, got %q", got)
		}
	})
}

// ---------------------------------------------------------------------------
// SecurityHeadersMiddleware (Gin)
// ---------------------------------------------------------------------------

func TestSecurityHeadersMiddleware(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		cfg   *SecurityHeadersConfig
		check func(t *testing.T, w *httptest.ResponseRecorder)
	}{
		{
			name: "nil config uses defaults",
			cfg:  nil,
			check: func(t *testing.T, w *httptest.ResponseRecorder) {
				t.Helper()
				// Should have all default headers
				if w.Header().Get("X-Content-Type-Options") != "nosniff" {
					t.Error("expected X-Content-Type-Options=nosniff with nil config")
				}
				if w.Header().Get("X-Frame-Options") != "DENY" {
					t.Error("expected X-Frame-Options=DENY with nil config")
				}
			},
		},
		{
			name: "all headers enabled - verify all present",
			cfg:  DefaultSecurityHeadersConfig(),
			check: func(t *testing.T, w *httptest.ResponseRecorder) {
				t.Helper()
				headers := map[string]string{
					"X-Content-Type-Options": "nosniff",
					"X-Frame-Options":        "DENY",
					"X-XSS-Protection":       "1; mode=block",
					"Referrer-Policy":        "strict-origin-when-cross-origin",
				}
				for h, expected := range headers {
					got := w.Header().Get(h)
					if got != expected {
						t.Errorf("expected %s=%q, got %q", h, expected, got)
					}
				}

				hsts := w.Header().Get("Strict-Transport-Security")
				if !strings.Contains(hsts, "max-age=31536000") {
					t.Errorf("expected HSTS with max-age=31536000, got %q", hsts)
				}
				if !strings.Contains(hsts, "includeSubDomains") {
					t.Errorf("expected HSTS with includeSubDomains, got %q", hsts)
				}

				pp := w.Header().Get("Permissions-Policy")
				if pp == "" {
					t.Error("expected Permissions-Policy to be set")
				}

				csp := w.Header().Get("Content-Security-Policy")
				if csp == "" {
					t.Error("expected Content-Security-Policy to be set")
				}
			},
		},
		{
			name: "individual headers disabled - HSTS off",
			cfg: &SecurityHeadersConfig{
				EnableHSTS:               false,
				EnableFrameOptions:       true,
				FrameOptionsValue:        "SAMEORIGIN",
				EnableXSSProtection:      false,
				EnableContentTypeNoSniff: true,
				EnableReferrerPolicy:     false,
				EnablePermissionsPolicy:  false,
				EnableCSP:                false,
			},
			check: func(t *testing.T, w *httptest.ResponseRecorder) {
				t.Helper()
				if w.Header().Get("Strict-Transport-Security") != "" {
					t.Error("expected no HSTS header")
				}
				if w.Header().Get("X-XSS-Protection") != "" {
					t.Error("expected no X-XSS-Protection header")
				}
				if w.Header().Get("Referrer-Policy") != "" {
					t.Error("expected no Referrer-Policy header")
				}
				if w.Header().Get("Permissions-Policy") != "" {
					t.Error("expected no Permissions-Policy header")
				}
				if w.Header().Get("Content-Security-Policy") != "" {
					t.Error("expected no Content-Security-Policy header")
				}
				if w.Header().Get("X-Frame-Options") != "SAMEORIGIN" {
					t.Errorf("expected X-Frame-Options=SAMEORIGIN, got %q", w.Header().Get("X-Frame-Options"))
				}
				if w.Header().Get("X-Content-Type-Options") != "nosniff" {
					t.Error("expected X-Content-Type-Options=nosniff")
				}
			},
		},
		{
			name: "CSP with nonce - Content-Security-Policy header contains nonce",
			cfg: &SecurityHeadersConfig{
				EnableCSP: true,
				CSPDirectives: map[string]string{
					"script-src": "'nonce-__CSP_NONCE__' 'strict-dynamic'",
				},
			},
			check: func(t *testing.T, w *httptest.ResponseRecorder) {
				t.Helper()
				csp := w.Header().Get("Content-Security-Policy")
				if csp == "" {
					t.Fatal("expected Content-Security-Policy header to be set")
				}
				if strings.Contains(csp, "__CSP_NONCE__") {
					t.Error("expected __CSP_NONCE__ placeholder to be replaced")
				}
				if !strings.Contains(csp, "nonce-") {
					t.Errorf("expected CSP to contain nonce-, got %q", csp)
				}
			},
		},
		{
			name: "CSP report-only",
			cfg: &SecurityHeadersConfig{
				EnableCSP:     true,
				CSPReportOnly: true,
				CSPDirectives: map[string]string{
					"default-src": "'self'",
				},
			},
			check: func(t *testing.T, w *httptest.ResponseRecorder) {
				t.Helper()
				if w.Header().Get("Content-Security-Policy") != "" {
					t.Error("expected no Content-Security-Policy header in report-only mode")
				}
				reportOnly := w.Header().Get("Content-Security-Policy-Report-Only")
				if reportOnly == "" {
					t.Error("expected Content-Security-Policy-Report-Only header")
				}
			},
		},
		{
			name: "HSTS with includeSubDomains",
			cfg: &SecurityHeadersConfig{
				EnableHSTS:            true,
				HSTSMaxAge:            600,
				HSTSIncludeSubDomains: true,
			},
			check: func(t *testing.T, w *httptest.ResponseRecorder) {
				t.Helper()
				hsts := w.Header().Get("Strict-Transport-Security")
				if hsts != "max-age=600; includeSubDomains" {
					t.Errorf("expected HSTS=%q, got %q", "max-age=600; includeSubDomains", hsts)
				}
			},
		},
		{
			name: "HSTS without includeSubDomains",
			cfg: &SecurityHeadersConfig{
				EnableHSTS:            true,
				HSTSMaxAge:            600,
				HSTSIncludeSubDomains: false,
			},
			check: func(t *testing.T, w *httptest.ResponseRecorder) {
				t.Helper()
				hsts := w.Header().Get("Strict-Transport-Security")
				if hsts != "max-age=600" {
					t.Errorf("expected HSTS=%q, got %q", "max-age=600", hsts)
				}
			},
		},
		{
			name: "Permissions-Policy empty value - header not set",
			cfg: &SecurityHeadersConfig{
				EnablePermissionsPolicy: true,
				PermissionsPolicyValue:  "",
			},
			check: func(t *testing.T, w *httptest.ResponseRecorder) {
				t.Helper()
				if w.Header().Get("Permissions-Policy") != "" {
					t.Error("expected no Permissions-Policy header when value is empty")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			w := httptest.NewRecorder()
			_, engine := gin.CreateTestContext(w)

			engine.Use(SecurityHeadersMiddleware(tt.cfg))
			engine.GET("/test", func(c *gin.Context) {
				c.Status(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			engine.ServeHTTP(w, req)

			tt.check(t, w)
		})
	}
}

// ---------------------------------------------------------------------------
// HTTPSecurityHeadersMiddleware (standard http.Handler)
// ---------------------------------------------------------------------------

func TestHTTPSecurityHeadersMiddleware(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		cfg   *SecurityHeadersConfig
		check func(t *testing.T, w *httptest.ResponseRecorder)
	}{
		{
			name: "nil config uses defaults",
			cfg:  nil,
			check: func(t *testing.T, w *httptest.ResponseRecorder) {
				t.Helper()
				if w.Header().Get("X-Content-Type-Options") != "nosniff" {
					t.Error("expected X-Content-Type-Options=nosniff")
				}
				if w.Header().Get("X-Frame-Options") != "DENY" {
					t.Error("expected X-Frame-Options=DENY")
				}
			},
		},
		{
			name: "all default headers present",
			cfg:  DefaultSecurityHeadersConfig(),
			check: func(t *testing.T, w *httptest.ResponseRecorder) {
				t.Helper()
				if w.Header().Get("X-Content-Type-Options") != "nosniff" {
					t.Error("expected X-Content-Type-Options=nosniff")
				}
				if w.Header().Get("X-Frame-Options") != "DENY" {
					t.Error("expected X-Frame-Options=DENY")
				}
				if w.Header().Get("X-XSS-Protection") != "1; mode=block" {
					t.Error("expected X-XSS-Protection=1; mode=block")
				}
				hsts := w.Header().Get("Strict-Transport-Security")
				if !strings.Contains(hsts, "max-age=31536000") {
					t.Errorf("expected HSTS with max-age, got %q", hsts)
				}
				if w.Header().Get("Referrer-Policy") != "strict-origin-when-cross-origin" {
					t.Error("expected correct Referrer-Policy")
				}
				if w.Header().Get("Permissions-Policy") == "" {
					t.Error("expected Permissions-Policy to be set")
				}
				csp := w.Header().Get("Content-Security-Policy")
				if csp == "" {
					t.Error("expected Content-Security-Policy to be set")
				}
			},
		},
		{
			name: "CSP report-only via HTTP handler",
			cfg: &SecurityHeadersConfig{
				EnableCSP:     true,
				CSPReportOnly: true,
				CSPDirectives: map[string]string{
					"default-src": "'self'",
				},
			},
			check: func(t *testing.T, w *httptest.ResponseRecorder) {
				t.Helper()
				if w.Header().Get("Content-Security-Policy") != "" {
					t.Error("expected no Content-Security-Policy in report-only mode")
				}
				if w.Header().Get("Content-Security-Policy-Report-Only") == "" {
					t.Error("expected Content-Security-Policy-Report-Only header")
				}
			},
		},
		{
			name: "headers disabled",
			cfg: &SecurityHeadersConfig{
				EnableHSTS:               false,
				EnableFrameOptions:       false,
				EnableXSSProtection:      false,
				EnableContentTypeNoSniff: false,
				EnableReferrerPolicy:     false,
				EnablePermissionsPolicy:  false,
				EnableCSP:                false,
			},
			check: func(t *testing.T, w *httptest.ResponseRecorder) {
				t.Helper()
				secHeaders := []string{
					"Strict-Transport-Security",
					"X-Frame-Options",
					"X-XSS-Protection",
					"X-Content-Type-Options",
					"Referrer-Policy",
					"Permissions-Policy",
					"Content-Security-Policy",
				}
				for _, h := range secHeaders {
					if w.Header().Get(h) != "" {
						t.Errorf("expected no %s header, got %q", h, w.Header().Get(h))
					}
				}
			},
		},
		{
			name: "HSTS with includeSubDomains via HTTP handler",
			cfg: &SecurityHeadersConfig{
				EnableHSTS:            true,
				HSTSMaxAge:            1000,
				HSTSIncludeSubDomains: true,
			},
			check: func(t *testing.T, w *httptest.ResponseRecorder) {
				t.Helper()
				hsts := w.Header().Get("Strict-Transport-Security")
				if hsts != "max-age=1000; includeSubDomains" {
					t.Errorf("expected HSTS=%q, got %q", "max-age=1000; includeSubDomains", hsts)
				}
			},
		},
		{
			name: "HSTS without includeSubDomains via HTTP handler",
			cfg: &SecurityHeadersConfig{
				EnableHSTS:            true,
				HSTSMaxAge:            500,
				HSTSIncludeSubDomains: false,
			},
			check: func(t *testing.T, w *httptest.ResponseRecorder) {
				t.Helper()
				hsts := w.Header().Get("Strict-Transport-Security")
				if hsts != "max-age=500" {
					t.Errorf("expected HSTS=%q, got %q", "max-age=500", hsts)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			handler := HTTPSecurityHeadersMiddleware(tt.cfg)(inner)
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			tt.check(t, w)
		})
	}
}

// ---------------------------------------------------------------------------
// SecurityHeadersMiddleware - nonce is stored in context
// ---------------------------------------------------------------------------

func TestSecurityHeadersMiddleware_NonceInContext(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	_, engine := gin.CreateTestContext(w)

	var capturedNonce string
	cfg := DefaultSecurityHeadersConfig()
	engine.Use(SecurityHeadersMiddleware(cfg))
	engine.GET("/test", func(c *gin.Context) {
		capturedNonce = GetCSPNonceFromGin(c)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	engine.ServeHTTP(w, req)

	if capturedNonce == "" {
		t.Error("expected CSP nonce to be stored in Gin context")
	}

	csp := w.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "nonce-"+capturedNonce) {
		t.Errorf("expected CSP header to contain nonce %q, got %q", capturedNonce, csp)
	}
}
