package auth

import (
	"net/http"
)

const (
	// CSRFHeaderName is the header name where the CSRF token should be sent.
	CSRFHeaderName = "X-CSRF-Token"

	// CSRFHeaderNameAlt is an alternative header name for CSRF token.
	CSRFHeaderNameAlt = "X-Csrf-Token"
)

// CSRFManager handles CSRF token generation and validation.
type CSRFManager struct {
	cookieConfig *CookieConfig
}

// NewCSRFManager creates a new CSRF manager with the given cookie configuration.
func NewCSRFManager(cookieConfig *CookieConfig) *CSRFManager {
	return &CSRFManager{
		cookieConfig: cookieConfig,
	}
}

// GenerateToken generates a new CSRF token.
func (m *CSRFManager) GenerateToken() (string, error) {
	return GenerateCSRFToken()
}

// SetToken sets the CSRF token in the response cookie.
func (m *CSRFManager) SetToken(w http.ResponseWriter, token string) {
	SetCSRFCookie(w, token, m.cookieConfig)
}

// ClearToken removes the CSRF token cookie.
func (m *CSRFManager) ClearToken(w http.ResponseWriter) {
	ClearCSRFCookie(w, m.cookieConfig)
}

// ValidateRequest validates the CSRF token in the request.
// The token in the header must match the token in the cookie.
// Uses constant-time comparison to prevent timing attacks.
func (m *CSRFManager) ValidateRequest(r *http.Request) bool {
	// Get token from cookie
	cookieToken := GetCSRFCookie(r)
	if cookieToken == "" {
		return false
	}

	// Get token from header (try both header names)
	headerToken := r.Header.Get(CSRFHeaderName)
	if headerToken == "" {
		headerToken = r.Header.Get(CSRFHeaderNameAlt)
	}
	if headerToken == "" {
		return false
	}

	// Constant-time comparison
	return ConstantTimeCompare(cookieToken, headerToken)
}

// GetTokenFromRequest extracts the CSRF token from the request header.
func GetTokenFromRequest(r *http.Request) string {
	token := r.Header.Get(CSRFHeaderName)
	if token == "" {
		token = r.Header.Get(CSRFHeaderNameAlt)
	}
	return token
}

// RequiresCSRFValidation returns true if the request method requires CSRF validation.
// Safe methods (GET, HEAD, OPTIONS, TRACE) do not require CSRF validation.
func RequiresCSRFValidation(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return false
	default:
		return true
	}
}

// ExemptPaths contains paths that are exempt from CSRF validation.
// These are typically login/register endpoints that don't have a CSRF token yet.
var ExemptPaths = map[string]bool{
	"/connect/auth.v1.AuthService/Login":        true,
	"/connect/auth.v1.AuthService/Register":     true,
	"/connect/auth.v1.AuthService/GetCsrfToken": true,
	// Password reset doesn't require CSRF as it uses a one-time token
	"/connect/auth.v1.AuthService/ForgotPassword": true,
	"/connect/auth.v1.AuthService/ResetPassword":  true,
	// Email verification uses a one-time token
	"/connect/auth.v1.AuthService/VerifyEmail": true,
}

// IsExemptPath returns true if the path is exempt from CSRF validation.
func IsExemptPath(path string) bool {
	return ExemptPaths[path]
}
