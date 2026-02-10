package auth

import (
	"net/http"
	"time"
)

// Cookie names - use __Host- prefix in production for maximum security.
// __Host- prefix requires:
// - Secure attribute must be set
// - Path must be "/"
// - Domain attribute must not be set
// - Cookie is only sent to the exact host that set it
//
// In development (HTTP), we use simple names without the prefix.
var (
	// SessionCookieName is the cookie name for the session ID.
	SessionCookieName = "session"

	// RefreshCookieName is the cookie name for the refresh token.
	RefreshCookieName = "refresh"

	// CSRFCookieName is the cookie name for the CSRF token.
	// This is a non-HttpOnly cookie so JavaScript can read it for the header.
	CSRFCookieName = "csrf"
)

// InitCookieNames sets the cookie names based on environment.
// In production (HTTPS), uses __Host- prefix for maximum security.
// In development (HTTP), uses simple names that work without HTTPS.
func InitCookieNames(isProduction bool) {
	if isProduction {
		SessionCookieName = "__Host-session"
		RefreshCookieName = "__Host-refresh"
		CSRFCookieName = "__Host-csrf"
	} else {
		SessionCookieName = "session"
		RefreshCookieName = "refresh"
		CSRFCookieName = "csrf"
	}
}

// CookieConfig holds the configuration for authentication cookies.
type CookieConfig struct {
	// Secure sets the Secure attribute. Must be true in production.
	Secure bool

	// SameSite sets the SameSite attribute for CSRF protection.
	SameSite http.SameSite

	// SessionMaxAge is the duration for session cookies.
	SessionMaxAge time.Duration

	// RefreshMaxAge is the duration for refresh token cookies.
	RefreshMaxAge time.Duration
}

// DefaultCookieConfig returns secure defaults for production.
func DefaultCookieConfig(isProduction bool) *CookieConfig {
	// Initialize cookie names based on environment
	InitCookieNames(isProduction)

	// In development, use Lax mode for cross-origin requests (different ports)
	// In production, use Strict mode for maximum CSRF protection
	sameSite := http.SameSiteStrictMode
	if !isProduction {
		sameSite = http.SameSiteLaxMode
	}

	return &CookieConfig{
		Secure:        isProduction,
		SameSite:      sameSite,
		SessionMaxAge: 24 * time.Hour,      // 24 hours
		RefreshMaxAge: 30 * 24 * time.Hour, // 30 days
	}
}

// SetSessionCookie sets the session cookie with secure attributes.
func SetSessionCookie(w http.ResponseWriter, sessionID string, cfg *CookieConfig) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    sessionID,
		Path:     "/",
		MaxAge:   int(cfg.SessionMaxAge.Seconds()),
		HttpOnly: true,
		Secure:   cfg.Secure,
		SameSite: cfg.SameSite,
	})
}

// SetRefreshCookie sets the refresh token cookie with secure attributes.
func SetRefreshCookie(w http.ResponseWriter, refreshToken string, cfg *CookieConfig) {
	http.SetCookie(w, &http.Cookie{
		Name:     RefreshCookieName,
		Value:    refreshToken,
		Path:     "/",
		MaxAge:   int(cfg.RefreshMaxAge.Seconds()),
		HttpOnly: true,
		Secure:   cfg.Secure,
		SameSite: cfg.SameSite,
	})
}

// SetCSRFCookie sets the CSRF token cookie.
// Note: This cookie is NOT HttpOnly because JavaScript needs to read it
// to include in request headers. The security comes from:
// 1. SameSite=Strict prevents CSRF attacks
// 2. The token must match between cookie and header
// 3. __Host- prefix ensures origin binding
func SetCSRFCookie(w http.ResponseWriter, csrfToken string, cfg *CookieConfig) {
	http.SetCookie(w, &http.Cookie{
		Name:     CSRFCookieName,
		Value:    csrfToken,
		Path:     "/",
		MaxAge:   int(cfg.RefreshMaxAge.Seconds()), // Same as refresh token
		HttpOnly: false,                            // JavaScript needs access
		Secure:   cfg.Secure,
		SameSite: cfg.SameSite,
	})
}

// ClearSessionCookie removes the session cookie.
func ClearSessionCookie(w http.ResponseWriter, cfg *CookieConfig) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   cfg.Secure,
		SameSite: cfg.SameSite,
	})
}

// ClearRefreshCookie removes the refresh token cookie.
func ClearRefreshCookie(w http.ResponseWriter, cfg *CookieConfig) {
	http.SetCookie(w, &http.Cookie{
		Name:     RefreshCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   cfg.Secure,
		SameSite: cfg.SameSite,
	})
}

// ClearCSRFCookie removes the CSRF token cookie.
func ClearCSRFCookie(w http.ResponseWriter, cfg *CookieConfig) {
	http.SetCookie(w, &http.Cookie{
		Name:     CSRFCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: false,
		Secure:   cfg.Secure,
		SameSite: cfg.SameSite,
	})
}

// ClearAllAuthCookies removes all authentication cookies.
func ClearAllAuthCookies(w http.ResponseWriter, cfg *CookieConfig) {
	ClearSessionCookie(w, cfg)
	ClearRefreshCookie(w, cfg)
	ClearCSRFCookie(w, cfg)
}

// GetSessionCookie retrieves the session ID from the request cookies.
// Returns empty string if not found.
func GetSessionCookie(r *http.Request) string {
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

// GetRefreshCookie retrieves the refresh token from the request cookies.
// Returns empty string if not found.
func GetRefreshCookie(r *http.Request) string {
	cookie, err := r.Cookie(RefreshCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

// GetCSRFCookie retrieves the CSRF token from the request cookies.
// Returns empty string if not found.
func GetCSRFCookie(r *http.Request) string {
	cookie, err := r.Cookie(CSRFCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}
