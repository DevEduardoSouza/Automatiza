// Package middleware provides HTTP middleware components for the application.
// This file implements security headers middleware (equivalent to Helmet.js).
package middleware

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// contextKey is a type for context keys to avoid collisions.
type contextKey string

// CSPNonceKey is the context key for the CSP nonce.
const CSPNonceKey contextKey = "csp-nonce"

// generateNonce creates a cryptographically secure nonce for CSP.
func generateNonce() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

// SecurityHeadersConfig holds configuration for security headers.
type SecurityHeadersConfig struct {
	// EnableHSTS enables HTTP Strict Transport Security header.
	EnableHSTS bool

	// HSTSMaxAge is the max-age value for HSTS in seconds (default: 31536000 = 1 year).
	HSTSMaxAge int

	// HSTSIncludeSubDomains includes subdomains in HSTS.
	HSTSIncludeSubDomains bool

	// EnableFrameOptions enables X-Frame-Options header.
	EnableFrameOptions bool

	// FrameOptionsValue is the value for X-Frame-Options (default: DENY).
	FrameOptionsValue string

	// EnableXSSProtection enables X-XSS-Protection header.
	EnableXSSProtection bool

	// EnableContentTypeNoSniff enables X-Content-Type-Options: nosniff.
	EnableContentTypeNoSniff bool

	// EnableReferrerPolicy enables Referrer-Policy header.
	EnableReferrerPolicy bool

	// ReferrerPolicyValue is the value for Referrer-Policy (default: strict-origin-when-cross-origin).
	ReferrerPolicyValue string

	// EnablePermissionsPolicy enables Permissions-Policy header.
	EnablePermissionsPolicy bool

	// PermissionsPolicyValue is the value for Permissions-Policy.
	PermissionsPolicyValue string

	// EnableCSP enables Content-Security-Policy header.
	EnableCSP bool

	// CSPDirectives holds custom CSP directives.
	// If empty, a secure default is used with nonce-based script-src.
	CSPDirectives map[string]string

	// CSPReportOnly uses Content-Security-Policy-Report-Only instead of CSP.
	// Useful for testing CSP without blocking resources.
	CSPReportOnly bool

	// CSPReportURI is the URI to report CSP violations to.
	CSPReportURI string
}

// DefaultSecurityHeadersConfig returns secure default configuration.
// This provides parity with Helmet.js defaults plus CSP.
func DefaultSecurityHeadersConfig() *SecurityHeadersConfig {
	return &SecurityHeadersConfig{
		EnableHSTS:               true,
		HSTSMaxAge:               31536000, // 1 year
		HSTSIncludeSubDomains:    true,
		EnableFrameOptions:       true,
		FrameOptionsValue:        "DENY",
		EnableXSSProtection:      true,
		EnableContentTypeNoSniff: true,
		EnableReferrerPolicy:     true,
		ReferrerPolicyValue:      "strict-origin-when-cross-origin",
		EnablePermissionsPolicy:  true,
		PermissionsPolicyValue:   "geolocation=(), microphone=(), camera=()",
		EnableCSP:                true,
		CSPDirectives:            nil, // Uses secure defaults
		CSPReportOnly:            false,
		CSPReportURI:             "",
	}
}

// DefaultCSPDirectives returns secure default CSP directives.
// The nonce placeholder __CSP_NONCE__ is replaced at runtime.
func DefaultCSPDirectives() map[string]string {
	return map[string]string{
		"default-src":     "'self'",
		"script-src":      "'self' 'nonce-__CSP_NONCE__' 'strict-dynamic'",
		"style-src":       "'self' 'unsafe-inline'", // Tailwind requires inline styles
		"img-src":         "'self' data: https:",
		"font-src":        "'self' data:",
		"connect-src":     "'self'",
		"frame-ancestors": "'none'",
		"base-uri":        "'self'",
		"form-action":     "'self'",
		"object-src":      "'none'",
		"upgrade-insecure-requests": "",
	}
}

// buildCSPHeader constructs the CSP header value with the given nonce.
func buildCSPHeader(directives map[string]string, nonce string, reportURI string) string {
	if directives == nil {
		directives = DefaultCSPDirectives()
	}

	var parts []string
	for directive, value := range directives {
		// Replace nonce placeholder
		if strings.Contains(value, "__CSP_NONCE__") {
			value = strings.ReplaceAll(value, "__CSP_NONCE__", nonce)
		}

		if value == "" {
			parts = append(parts, directive)
		} else {
			parts = append(parts, directive+" "+value)
		}
	}

	if reportURI != "" {
		parts = append(parts, "report-uri "+reportURI)
	}

	return strings.Join(parts, "; ")
}

// GetCSPNonce retrieves the CSP nonce from the request context.
// Returns empty string if no nonce is set.
func GetCSPNonce(ctx context.Context) string {
	if nonce, ok := ctx.Value(CSPNonceKey).(string); ok {
		return nonce
	}
	return ""
}

// GetCSPNonceFromGin retrieves the CSP nonce from a Gin context.
func GetCSPNonceFromGin(c *gin.Context) string {
	return GetCSPNonce(c.Request.Context())
}

// SecurityHeadersMiddleware creates a Gin middleware that adds security headers.
// This provides similar protection to Helmet.js in Node.js applications.
func SecurityHeadersMiddleware(cfg *SecurityHeadersConfig) gin.HandlerFunc {
	if cfg == nil {
		cfg = DefaultSecurityHeadersConfig()
	}

	return func(c *gin.Context) {
		// Generate nonce for CSP if enabled
		var nonce string
		if cfg.EnableCSP {
			var err error
			nonce, err = generateNonce()
			if err == nil {
				// Store nonce in context for use by templates/handlers
				ctx := context.WithValue(c.Request.Context(), CSPNonceKey, nonce)
				c.Request = c.Request.WithContext(ctx)
			}
		}

		// X-Content-Type-Options: Prevents MIME type sniffing
		if cfg.EnableContentTypeNoSniff {
			c.Header("X-Content-Type-Options", "nosniff")
		}

		// X-Frame-Options: Prevents clickjacking
		if cfg.EnableFrameOptions {
			c.Header("X-Frame-Options", cfg.FrameOptionsValue)
		}

		// X-XSS-Protection: Legacy XSS protection (for older browsers)
		if cfg.EnableXSSProtection {
			c.Header("X-XSS-Protection", "1; mode=block")
		}

		// Strict-Transport-Security (HSTS): Forces HTTPS
		if cfg.EnableHSTS {
			hstsValue := "max-age=" + itoa(cfg.HSTSMaxAge)
			if cfg.HSTSIncludeSubDomains {
				hstsValue += "; includeSubDomains"
			}
			c.Header("Strict-Transport-Security", hstsValue)
		}

		// Referrer-Policy: Controls referrer information
		if cfg.EnableReferrerPolicy {
			c.Header("Referrer-Policy", cfg.ReferrerPolicyValue)
		}

		// Permissions-Policy: Controls browser features
		if cfg.EnablePermissionsPolicy && cfg.PermissionsPolicyValue != "" {
			c.Header("Permissions-Policy", cfg.PermissionsPolicyValue)
		}

		// Content-Security-Policy: XSS and injection protection
		if cfg.EnableCSP && nonce != "" {
			cspValue := buildCSPHeader(cfg.CSPDirectives, nonce, cfg.CSPReportURI)
			if cfg.CSPReportOnly {
				c.Header("Content-Security-Policy-Report-Only", cspValue)
			} else {
				c.Header("Content-Security-Policy", cspValue)
			}
		}

		c.Next()
	}
}

// HTTPSecurityHeadersMiddleware creates a standard HTTP middleware for security headers.
// Use this for non-Gin handlers (e.g., ConnectRPC).
func HTTPSecurityHeadersMiddleware(cfg *SecurityHeadersConfig) func(http.Handler) http.Handler {
	if cfg == nil {
		cfg = DefaultSecurityHeadersConfig()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Generate nonce for CSP if enabled
			var nonce string
			if cfg.EnableCSP {
				var err error
				nonce, err = generateNonce()
				if err == nil {
					// Store nonce in context for use by handlers
					ctx := context.WithValue(r.Context(), CSPNonceKey, nonce)
					r = r.WithContext(ctx)
				}
			}

			// X-Content-Type-Options: Prevents MIME type sniffing
			if cfg.EnableContentTypeNoSniff {
				w.Header().Set("X-Content-Type-Options", "nosniff")
			}

			// X-Frame-Options: Prevents clickjacking
			if cfg.EnableFrameOptions {
				w.Header().Set("X-Frame-Options", cfg.FrameOptionsValue)
			}

			// X-XSS-Protection: Legacy XSS protection (for older browsers)
			if cfg.EnableXSSProtection {
				w.Header().Set("X-XSS-Protection", "1; mode=block")
			}

			// Strict-Transport-Security (HSTS): Forces HTTPS
			if cfg.EnableHSTS {
				hstsValue := "max-age=" + itoa(cfg.HSTSMaxAge)
				if cfg.HSTSIncludeSubDomains {
					hstsValue += "; includeSubDomains"
				}
				w.Header().Set("Strict-Transport-Security", hstsValue)
			}

			// Referrer-Policy: Controls referrer information
			if cfg.EnableReferrerPolicy {
				w.Header().Set("Referrer-Policy", cfg.ReferrerPolicyValue)
			}

			// Permissions-Policy: Controls browser features
			if cfg.EnablePermissionsPolicy && cfg.PermissionsPolicyValue != "" {
				w.Header().Set("Permissions-Policy", cfg.PermissionsPolicyValue)
			}

			// Content-Security-Policy: XSS and injection protection
			if cfg.EnableCSP && nonce != "" {
				cspValue := buildCSPHeader(cfg.CSPDirectives, nonce, cfg.CSPReportURI)
				if cfg.CSPReportOnly {
					w.Header().Set("Content-Security-Policy-Report-Only", cspValue)
				} else {
					w.Header().Set("Content-Security-Policy", cspValue)
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}
