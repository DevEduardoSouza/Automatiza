package middleware

import (
	"automatiza-go/internal/auth"
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// AuthenticatedUser represents the authenticated user in the context
type AuthenticatedUser struct {
	ID    string `json:"id"`
	Email string `json:"email,omitempty"`
	Role  string `json:"role,omitempty"`
}

// ContextKey type for context keys
type ContextKey string

const (
	// UserContextKey is the key for user in context
	UserContextKey ContextKey = "user"
)

// SessionAuthMiddleware validates sessions using AuthService (SECURE - recommended).
// This middleware extracts session ID from cookie or Authorization header and validates
// it against the sessions table with proper security checks.
func SessionAuthMiddleware(authService *auth.AuthService, userRoleRepo repository.UserRoleRepository, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. Extract session ID from cookie (preferred) or Bearer token (fallback)
		sessionID := extractSessionID(c)
		if sessionID == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, models.ApiResponse[any]{
				Success: false,
				Error:   "Sessao nao fornecida",
			})
			return
		}

		// 2. Validate session via AuthService (checks expiry, revocation, etc.)
		user, session, err := authService.GetCurrentUser(c.Request.Context(), sessionID)
		if err != nil {
			logger.Debug("Session validation failed",
				zap.Error(err),
				zap.String("sessionID", sessionID[:min(8, len(sessionID))]+"..."), // Log only prefix
			)
			c.AbortWithStatusJSON(http.StatusUnauthorized, models.ApiResponse[any]{
				Success: false,
				Error:   "Sessao invalida ou expirada",
			})
			return
		}

		// 3. Check if user is suspended (via billing_status in session/user data)
		// Note: This is also checked in suspension_interceptor for ConnectRPC

		// 4. Get user role from PostgreSQL
		role := user.Role
		if role == "" {
			role = "user"
		}
		if userRoleRepo != nil {
			userRole, err := userRoleRepo.GetUserRole(c.Request.Context(), user.ID)
			if err == nil && userRole != nil {
				role = userRole.Role
			}
		}

		// 5. Set user and session in context
		authUser := &AuthenticatedUser{
			ID:    user.ID,
			Email: user.Email,
			Role:  role,
		}
		c.Set(string(UserContextKey), authUser)
		c.Set("session", session)

		c.Next()
	}
}

// extractSessionID extracts session ID from cookie or Authorization header.
func extractSessionID(c *gin.Context) string {
	// Try cookie first (preferred for browser clients)
	if cookie, err := c.Cookie(auth.SessionCookieName); err == nil && cookie != "" {
		return cookie
	}

	// Fallback to Authorization header (for API clients)
	authHeader := c.GetHeader("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer ")
	}

	return ""
}

// AuthMiddlewareWithRepos validates JWT tokens using repositories (PostgreSQL).
// DEPRECATED: Use SessionAuthMiddleware instead for proper session validation.
// This middleware is kept for backward compatibility but should be migrated.
func AuthMiddlewareWithRepos(profileRepo repository.ProfileRepository, userRoleRepo repository.UserRoleRepository, logger *zap.Logger) gin.HandlerFunc {
	logger.Warn("AuthMiddlewareWithRepos is deprecated - migrate to SessionAuthMiddleware for proper session validation")
	return func(c *gin.Context) {
		// 1. Extract Bearer token (user ID for now - to be replaced with session validation)
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, models.ApiResponse[any]{
				Success: false,
				Error:   "Token nao fornecido",
			})
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, models.ApiResponse[any]{
				Success: false,
				Error:   "Token invalido",
			})
			return
		}

		// DEPRECATED: Treating token as user ID is insecure
		// In production, this should validate against the sessions table
		userID := token

		// 2. Get profile from PostgreSQL
		profile, err := profileRepo.GetProfile(c.Request.Context(), userID)
		if err != nil {
			logger.Debug("Auth validation failed - profile not found", zap.Error(err), zap.String("userID", userID))
			c.AbortWithStatusJSON(http.StatusUnauthorized, models.ApiResponse[any]{
				Success: false,
				Error:   "Token invalido ou expirado",
			})
			return
		}

		// 3. Check billing_status (suspension check)
		if profile.BillingStatus != nil && *profile.BillingStatus == "suspended" {
			c.AbortWithStatusJSON(http.StatusForbidden, models.ApiResponse[any]{
				Success: false,
				Error:   "Conta suspensa. Entre em contato com o suporte.",
			})
			return
		}

		// 4. Get user role from PostgreSQL
		role := "user"
		userRole, err := userRoleRepo.GetUserRole(c.Request.Context(), userID)
		if err == nil && userRole != nil {
			role = userRole.Role
		}

		// 5. Set user in context
		authUser := &AuthenticatedUser{
			ID:    profile.ID,
			Email: profile.Email,
			Role:  role,
		}
		c.Set(string(UserContextKey), authUser)

		c.Next()
	}
}

// AdminMiddleware checks if user is an admin
func AdminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := c.Get(string(UserContextKey))
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, models.ApiResponse[any]{
				Success: false,
				Error:   "Nao autenticado",
			})
			return
		}

		authUser, ok := user.(*AuthenticatedUser)
		if !ok {
			c.AbortWithStatusJSON(http.StatusInternalServerError, models.ApiResponse[any]{
				Success: false,
				Error:   "Erro interno de autenticacao",
			})
			return
		}

		if authUser.Role != "admin" {
			c.AbortWithStatusJSON(http.StatusForbidden, models.ApiResponse[any]{
				Success: false,
				Error:   "Acesso negado. Apenas administradores.",
			})
			return
		}

		c.Next()
	}
}

// OptionalSessionAuthMiddleware validates session if present (SECURE - recommended).
func OptionalSessionAuthMiddleware(authService *auth.AuthService, userRoleRepo repository.UserRoleRepository, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionID := extractSessionID(c)
		if sessionID == "" {
			c.Next()
			return
		}

		user, session, err := authService.GetCurrentUser(c.Request.Context(), sessionID)
		if err != nil {
			logger.Debug("Optional session validation failed", zap.Error(err))
			c.Next()
			return
		}

		role := user.Role
		if role == "" {
			role = "user"
		}
		if userRoleRepo != nil {
			userRole, err := userRoleRepo.GetUserRole(c.Request.Context(), user.ID)
			if err == nil && userRole != nil {
				role = userRole.Role
			}
		}

		authUser := &AuthenticatedUser{
			ID:    user.ID,
			Email: user.Email,
			Role:  role,
		}
		c.Set(string(UserContextKey), authUser)
		c.Set("session", session)

		c.Next()
	}
}

// OptionalAuthMiddlewareWithRepos validates JWT if present using repositories (PostgreSQL).
// DEPRECATED: Use OptionalSessionAuthMiddleware instead.
func OptionalAuthMiddlewareWithRepos(profileRepo repository.ProfileRepository, userRoleRepo repository.UserRoleRepository, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.Next()
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == "" {
			c.Next()
			return
		}

		// DEPRECATED: Treating token as user ID is insecure
		userID := token

		profile, err := profileRepo.GetProfile(c.Request.Context(), userID)
		if err != nil {
			logger.Debug("Optional auth validation failed", zap.Error(err))
			c.Next()
			return
		}

		role := "user"
		userRole, err := userRoleRepo.GetUserRole(c.Request.Context(), userID)
		if err == nil && userRole != nil {
			role = userRole.Role
		}

		authUser := &AuthenticatedUser{
			ID:    profile.ID,
			Email: profile.Email,
			Role:  role,
		}
		c.Set(string(UserContextKey), authUser)

		c.Next()
	}
}

// GetAuthenticatedUser retrieves the authenticated user from context
func GetAuthenticatedUser(c *gin.Context) *AuthenticatedUser {
	user, exists := c.Get(string(UserContextKey))
	if !exists {
		return nil
	}

	authUser, ok := user.(*AuthenticatedUser)
	if !ok {
		return nil
	}

	return authUser
}

// GetAuthenticatedUserFromContext retrieves user from standard context
func GetAuthenticatedUserFromContext(ctx context.Context) *AuthenticatedUser {
	user := ctx.Value(UserContextKey)
	if user == nil {
		return nil
	}

	authUser, ok := user.(*AuthenticatedUser)
	if !ok {
		return nil
	}

	return authUser
}

// MustGetAuthenticatedUser retrieves the authenticated user or panics
func MustGetAuthenticatedUser(c *gin.Context) *AuthenticatedUser {
	user := GetAuthenticatedUser(c)
	if user == nil {
		panic("user not found in context - ensure AuthMiddleware is applied")
	}
	return user
}

// SSESessionAuthMiddleware validates sessions from query parameter for SSE connections.
// This is needed for SSE (EventSource) which doesn't support custom headers.
// Usage: GET /api/v1/sse/events?session=<session_id>
// SECURITY NOTE: Tokens in query params are logged in server access logs and browser history.
// Consider using short-lived tokens or WebSocket subprotocol for better security.
func SSESessionAuthMiddleware(authService *auth.AuthService, userRoleRepo repository.UserRoleRepository, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. Extract session ID from query parameter
		sessionID := c.Query("session")
		if sessionID == "" {
			// Fallback to "token" for backward compatibility
			sessionID = c.Query("token")
		}
		if sessionID == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, models.ApiResponse[any]{
				Success: false,
				Error:   "Sessao nao fornecida",
			})
			return
		}

		// 2. Validate session via AuthService
		user, session, err := authService.GetCurrentUser(c.Request.Context(), sessionID)
		if err != nil {
			logger.Debug("SSE session validation failed",
				zap.Error(err),
				zap.String("sessionID", sessionID[:min(8, len(sessionID))]+"..."),
			)
			c.AbortWithStatusJSON(http.StatusUnauthorized, models.ApiResponse[any]{
				Success: false,
				Error:   "Sessao invalida ou expirada",
			})
			return
		}

		// 3. Get user role
		role := user.Role
		if role == "" {
			role = "user"
		}
		if userRoleRepo != nil {
			userRole, err := userRoleRepo.GetUserRole(c.Request.Context(), user.ID)
			if err == nil && userRole != nil {
				role = userRole.Role
			}
		}

		// 4. Set user in context
		authUser := &AuthenticatedUser{
			ID:    user.ID,
			Email: user.Email,
			Role:  role,
		}
		c.Set(string(UserContextKey), authUser)
		c.Set("session", session)
		c.Set("userID", user.ID)

		c.Next()
	}
}

// QueryTokenAuthMiddleware validates tokens from query parameter instead of header.
// DEPRECATED: Use SSESessionAuthMiddleware instead for proper session validation.
// This is needed for SSE (EventSource) which doesn't support custom headers.
// Usage: GET /api/v1/sse/events?token=<user_id>
func QueryTokenAuthMiddleware(profileRepo repository.ProfileRepository, userRoleRepo repository.UserRoleRepository, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. Extract token from query parameter
		token := c.Query("token")
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, models.ApiResponse[any]{
				Success: false,
				Error:   "Token nao fornecido",
			})
			return
		}

		// DEPRECATED: Treating token as user ID is insecure
		userID := token

		// 2. Get profile from PostgreSQL
		profile, err := profileRepo.GetProfile(c.Request.Context(), userID)
		if err != nil {
			logger.Debug("SSE Auth validation failed - profile not found", zap.Error(err), zap.String("userID", userID))
			c.AbortWithStatusJSON(http.StatusUnauthorized, models.ApiResponse[any]{
				Success: false,
				Error:   "Token invalido ou expirado",
			})
			return
		}

		// 3. Check billing_status (suspension check)
		if profile.BillingStatus != nil && *profile.BillingStatus == "suspended" {
			c.AbortWithStatusJSON(http.StatusForbidden, models.ApiResponse[any]{
				Success: false,
				Error:   "Conta suspensa. Entre em contato com o suporte.",
			})
			return
		}

		// 4. Get user role from PostgreSQL
		role := "user"
		userRole, err := userRoleRepo.GetUserRole(c.Request.Context(), userID)
		if err == nil && userRole != nil {
			role = userRole.Role
		}

		// 5. Set user in context
		authUser := &AuthenticatedUser{
			ID:    profile.ID,
			Email: profile.Email,
			Role:  role,
		}
		c.Set(string(UserContextKey), authUser)

		// Also set userID directly for SSE handler convenience
		c.Set("userID", profile.ID)

		c.Next()
	}
}
