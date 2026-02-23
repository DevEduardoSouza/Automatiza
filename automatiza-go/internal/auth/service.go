package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"automatiza-go/internal/config"
	"automatiza-go/internal/observability"
	"automatiza-go/internal/services/email"

	"github.com/getsentry/sentry-go"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

var (
	// ErrInvalidCredentials indicates invalid email or password.
	ErrInvalidCredentials = errors.New("invalid email or password")

	// ErrAccountLocked indicates the account is locked due to too many failed attempts.
	ErrAccountLocked = errors.New("account locked due to too many failed attempts")

	// ErrEmailNotVerified indicates the email has not been verified.
	ErrEmailNotVerified = errors.New("email not verified")

	// ErrUserNotFound indicates the user was not found.
	ErrUserNotFound = errors.New("user not found")

	// ErrEmailAlreadyExists indicates the email is already registered.
	ErrEmailAlreadyExists = errors.New("email already exists")

	// ErrInvalidToken indicates the token is invalid or expired.
	ErrInvalidToken = errors.New("invalid or expired token")

	// ErrRateLimited indicates too many requests.
	ErrRateLimited = errors.New("too many requests")

	// ErrUnauthorized indicates the user is not authenticated.
	ErrUnauthorized = errors.New("unauthorized")
)

// AuthService provides authentication operations.
type AuthService struct {
	db            *config.DBClient
	sessionStore  SessionStore
	rateLimiter   *RateLimiter
	cookieConfig  *CookieConfig
	csrfManager   *CSRFManager
	emailService  email.Service
	publicBaseURL string
	logger        *zap.Logger

	// getUserOverride is a test hook: when non-nil, GetCurrentUser uses this
	// function instead of the DB-backed getUser method. Set via testing helpers only.
	getUserOverride getUserOverrideFunc
}

// AuthServiceConfig holds configuration for AuthService.
type AuthServiceConfig struct {
	DB            *config.DBClient
	Redis         *redis.Client
	CookieConfig  *CookieConfig
	EmailService  email.Service
	PublicBaseURL string
	Logger        *zap.Logger
	// RateLimitConfig provides custom rate limiting settings.
	// If nil, DefaultLoginRateLimit is used.
	RateLimitConfig *RateLimitConfig
}

// NewAuthService creates a new authentication service.
func NewAuthService(cfg *AuthServiceConfig) *AuthService {
	sessionDB := &DBSessionAdapter{
		client: cfg.DB,
	}

	sessionStore := NewRedisSessionStore(cfg.Redis, sessionDB, cfg.CookieConfig, cfg.Logger)

	// Use custom rate limit config if provided, otherwise use defaults
	rateLimitCfg := cfg.RateLimitConfig
	if rateLimitCfg == nil {
		rateLimitCfg = DefaultLoginRateLimit
	}
	rateLimiter := NewRateLimiter(cfg.Redis, "auth:ratelimit", rateLimitCfg)
	csrfManager := NewCSRFManager(cfg.CookieConfig)

	return &AuthService{
		db:            cfg.DB,
		sessionStore:  sessionStore,
		rateLimiter:   rateLimiter,
		cookieConfig:  cfg.CookieConfig,
		csrfManager:   csrfManager,
		emailService:  cfg.EmailService,
		publicBaseURL: cfg.PublicBaseURL,
		logger:        cfg.Logger,
	}
}

// User represents a user in the system.
type User struct {
	ID            string
	Email         string
	DisplayName   string
	Role          string
	EmailVerified bool
	CreatedAt     time.Time
}

// LoginResult contains the result of a successful login.
type LoginResult struct {
	User         *User
	Session      *Session
	RefreshToken string
	CSRFToken    string
}

// Login authenticates a user with email and password.
func (s *AuthService) Login(ctx context.Context, email, password, ipAddress, userAgent, deviceFingerprint string) (*LoginResult, error) {
	// Check rate limit
	rateLimitKey := LoginRateLimitKey(ipAddress, email)
	result, err := s.rateLimiter.Check(ctx, rateLimitKey)
	if err != nil {
		s.logger.Error("Failed to check rate limit", zap.Error(err))
	} else if !result.Allowed {
		// SECURITY AUDIT: Log rate limit block
		s.logger.Warn("Login blocked by rate limiter",
			zap.String("event", "rate_limit_block"),
			zap.String("ip", ipAddress),
			zap.Duration("retryAfter", result.RetryAfter),
		)
		return nil, ErrRateLimited
	}

	// Get user credentials
	creds, err := s.getCredentials(ctx, email)
	if err != nil {
		// Increment rate limit on failure
		if _, rlErr := s.rateLimiter.Increment(ctx, rateLimitKey); rlErr != nil {
			observability.LogAndCapture(s.logger, sentry.LevelError, "rate limiter increment failed on login credential error", rlErr, map[string]string{
				"module": "auth", "operation": "login_rate_limit_increment", "flow": "credential_failure",
			})
		}
		return nil, ErrInvalidCredentials
	}

	// Check if account is locked
	if creds.LockedUntil != nil && time.Now().Before(*creds.LockedUntil) {
		// SECURITY AUDIT: Log attempt to login to locked account
		s.logger.Warn("Login attempt to locked account",
			zap.String("event", "locked_account_login_attempt"),
			zap.String("userID", creds.UserID),
			zap.String("ip", ipAddress),
			zap.Time("lockedUntil", *creds.LockedUntil),
		)
		return nil, ErrAccountLocked
	}

	// Verify password (supports both Argon2id and legacy bcrypt from Supabase)
	pwResult, err := VerifyPasswordWithMigration(password, creds.PasswordHash)
	if err != nil || !pwResult.Valid {
		// Increment failed attempts
		if faErr := s.incrementFailedAttempts(ctx, creds.UserID); faErr != nil {
			observability.LogAndCapture(s.logger, sentry.LevelError, "failed to increment failed login attempts", faErr, map[string]string{
				"module": "auth", "operation": "increment_failed_attempts", "user_id": creds.UserID,
			})
		}
		if _, rlErr := s.rateLimiter.Increment(ctx, rateLimitKey); rlErr != nil {
			observability.LogAndCapture(s.logger, sentry.LevelError, "rate limiter increment failed on password failure", rlErr, map[string]string{
				"module": "auth", "operation": "login_rate_limit_increment", "flow": "password_failure",
			})
		}
		return nil, ErrInvalidCredentials
	}

	// Migrate bcrypt hash to Argon2id if needed (transparent upgrade)
	if pwResult.NeedsMigration {
		userID := creds.UserID
		observability.SafeGo("auth-password-migration", func() {
			bgCtx := context.Background()
			newHash, hashErr := HashPassword(password)
			if hashErr != nil {
				observability.LogAndCapture(s.logger, sentry.LevelWarning, "failed to hash password for migration", hashErr, map[string]string{
					"module": "auth", "operation": "password_migration", "user_id": userID,
				})
				return
			}
			if upErr := s.updatePassword(bgCtx, userID, newHash); upErr != nil {
				observability.LogAndCapture(s.logger, sentry.LevelWarning, "failed to migrate password hash to argon2id", upErr, map[string]string{
					"module": "auth", "operation": "password_migration", "user_id": userID,
				})
			} else {
				s.logger.Info("Successfully migrated password hash from bcrypt to Argon2id",
					zap.String("userID", userID))
			}
		})
	}

	// Get user profile
	user, err := s.getUser(ctx, creds.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Reset failed attempts on successful login
	if rfErr := s.resetFailedAttempts(ctx, creds.UserID); rfErr != nil {
		observability.LogAndCapture(s.logger, sentry.LevelWarning, "failed to reset failed login attempts", rfErr, map[string]string{
			"module": "auth", "operation": "reset_failed_attempts", "user_id": creds.UserID,
		})
	}
	if rlErr := s.rateLimiter.Reset(ctx, rateLimitKey); rlErr != nil {
		observability.LogAndCapture(s.logger, sentry.LevelWarning, "rate limiter reset failed after successful login", rlErr, map[string]string{
			"module": "auth", "operation": "login_rate_limit_reset",
		})
	}

	// Create session
	session, refreshToken, csrfToken, err := s.sessionStore.Create(ctx, user.ID, SessionMetadata{
		DeviceFingerprint: deviceFingerprint,
		IPAddress:         ipAddress,
		UserAgent:         userAgent,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	return &LoginResult{
		User:         user,
		Session:      session,
		RefreshToken: refreshToken,
		CSRFToken:    csrfToken,
	}, nil
}

// RegisterResult contains the result of a successful registration.
type RegisterResult struct {
	User                  *User
	EmailVerificationSent bool
}

// Register creates a new user account.
func (s *AuthService) Register(ctx context.Context, email, password, displayName string) (*RegisterResult, error) {
	// Validate password complexity
	if err := ValidatePasswordComplexity(password); err != nil {
		return nil, err
	}

	// SECURITY: Always hash password first to ensure constant-time response.
	// This prevents timing attacks that could enumerate valid emails.
	// The hash operation takes significant time, so we do it before the email check.
	passwordHash, err := HashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Check if email already exists (after hashing to prevent timing leak)
	existing, existErr := s.getUserByEmail(ctx, email)
	if existErr != nil && !errors.Is(existErr, ErrUserNotFound) {
		// Propagate unexpected DB errors; ErrUserNotFound means email is available
		return nil, fmt.Errorf("failed to check existing email: %w", existErr)
	}
	if existing != nil {
		return nil, ErrEmailAlreadyExists
	}

	// Create profile
	profile, err := s.createProfile(ctx, email, displayName)
	if err != nil {
		return nil, fmt.Errorf("failed to create profile: %w", err)
	}

	// Create credentials
	if err := s.createCredentials(ctx, profile.ID, passwordHash); err != nil {
		return nil, fmt.Errorf("failed to create credentials: %w", err)
	}

	// Generate and send verification email (async)
	profileID := profile.ID
	observability.SafeGo("auth-verification-email", func() {
		bgCtx := context.Background()
		if sendErr := s.sendVerificationEmail(bgCtx, profileID, email); sendErr != nil {
			observability.LogAndCapture(s.logger, sentry.LevelError, "failed to send verification email", sendErr, map[string]string{
				"module": "auth", "operation": "send_verification_email", "user_id": profileID,
			})
		}
	})

	return &RegisterResult{
		User: &User{
			ID:            profile.ID,
			Email:         email,
			DisplayName:   displayName,
			Role:          "user",
			EmailVerified: false,
			CreatedAt:     time.Now(),
		},
		EmailVerificationSent: true,
	}, nil
}

// Logout invalidates the current session.
func (s *AuthService) Logout(ctx context.Context, sessionID string, logoutAllDevices bool) (int, error) {
	session, err := s.sessionStore.Get(ctx, sessionID)
	if err != nil {
		return 0, err
	}

	if logoutAllDevices {
		return s.sessionStore.RevokeAllForUser(ctx, session.UserID)
	}

	if err := s.sessionStore.Revoke(ctx, sessionID); err != nil {
		return 0, err
	}
	return 1, nil
}

// RefreshSession refreshes an existing session.
func (s *AuthService) RefreshSession(ctx context.Context, sessionID, refreshToken string) (*LoginResult, error) {
	session, newRefreshToken, newCSRFToken, err := s.sessionStore.Refresh(ctx, sessionID, refreshToken)
	if err != nil {
		return nil, err
	}

	user, err := s.getUser(ctx, session.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &LoginResult{
		User:         user,
		Session:      session,
		RefreshToken: newRefreshToken,
		CSRFToken:    newCSRFToken,
	}, nil
}

// GetCurrentUser returns the current user and session info.
func (s *AuthService) GetCurrentUser(ctx context.Context, sessionID string) (*User, *Session, error) {
	session, err := s.sessionStore.Get(ctx, sessionID)
	if err != nil {
		return nil, nil, err
	}

	var user *User
	if s.getUserOverride != nil {
		user, err = s.getUserOverride(ctx, session.UserID)
	} else {
		user, err = s.getUser(ctx, session.UserID)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Update activity (fire-and-forget, never blocks request)
	if actErr := s.sessionStore.UpdateActivity(ctx, sessionID); actErr != nil {
		observability.LogAndCapture(s.logger, sentry.LevelWarning, "failed to update session activity", actErr, map[string]string{
			"module": "auth", "operation": "update_activity", "session_id": sessionID,
		})
	}

	return user, session, nil
}

// ListSessions returns all active sessions for a user.
func (s *AuthService) ListSessions(ctx context.Context, sessionID string) ([]*Session, string, error) {
	session, err := s.sessionStore.Get(ctx, sessionID)
	if err != nil {
		return nil, "", err
	}

	sessions, err := s.sessionStore.GetByUserID(ctx, session.UserID)
	if err != nil {
		return nil, "", err
	}

	return sessions, sessionID, nil
}

// RevokeSession revokes a specific session.
func (s *AuthService) RevokeSession(ctx context.Context, currentSessionID, targetSessionID string) error {
	// Verify current session exists and get user ID
	currentSession, err := s.sessionStore.Get(ctx, currentSessionID)
	if err != nil {
		return err
	}

	// Verify target session belongs to same user
	targetSession, err := s.sessionStore.Get(ctx, targetSessionID)
	if err != nil {
		return err
	}

	if currentSession.UserID != targetSession.UserID {
		return ErrUnauthorized
	}

	return s.sessionStore.Revoke(ctx, targetSessionID)
}

// ForgotPassword initiates password reset flow.
// SECURITY: This function is designed to prevent email enumeration via timing attacks.
// It always performs the same operations regardless of whether the email exists.
func (s *AuthService) ForgotPassword(ctx context.Context, email string) error {
	// SECURITY: Always generate a token first to ensure constant-time response.
	// This prevents attackers from determining if an email exists based on response time.
	token, err := GenerateResetToken()
	if err != nil {
		return fmt.Errorf("failed to generate token: %w", err)
	}

	// Compute the token hash (constant time operation)
	tokenHash := HashToken(token)

	// Look up user (this is the variable-time operation we need to hide)
	user, err := s.getUserByEmail(ctx, email)

	// Even if user doesn't exist, we still "process" the request
	// to maintain constant timing. We just don't store or send anything.
	if err != nil || user == nil {
		// SECURITY: Log for monitoring but don't reveal to caller
		s.logger.Debug("Password reset requested for unknown email",
			zap.String("email_prefix", email[:min(3, len(email))]+"..."))
		// Return nil to not reveal if email exists
		return nil
	}

	// Store token hash (only if user exists)
	if err := s.storePasswordResetToken(ctx, user.ID, tokenHash); err != nil {
		return fmt.Errorf("failed to store token: %w", err)
	}

	// Send email (async, only if user exists)
	resetUserEmail := user.Email
	resetUserID := user.ID
	observability.SafeGo("auth-reset-email", func() {
		bgCtx := context.Background()
		if sendErr := s.sendPasswordResetEmail(bgCtx, resetUserEmail, token); sendErr != nil {
			observability.LogAndCapture(s.logger, sentry.LevelError, "failed to send password reset email", sendErr, map[string]string{
				"module": "auth", "operation": "send_reset_email", "user_id": resetUserID,
			})
		}
	})

	return nil
}

// ResetPassword completes password reset with token.
func (s *AuthService) ResetPassword(ctx context.Context, token, newPassword string) error {
	// Validate password complexity
	if err := ValidatePasswordComplexity(newPassword); err != nil {
		return err
	}

	// Find and validate token
	userID, err := s.validatePasswordResetToken(ctx, HashToken(token))
	if err != nil {
		return ErrInvalidToken
	}

	// Hash new password
	passwordHash, err := HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Update password
	if err := s.updatePassword(ctx, userID, passwordHash); err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	// Mark token as used
	if markErr := s.markPasswordResetTokenUsed(ctx, HashToken(token)); markErr != nil {
		observability.LogAndCapture(s.logger, sentry.LevelWarning, "failed to mark password reset token as used", markErr, map[string]string{
			"module": "auth", "operation": "mark_reset_token_used", "security": "token_reuse_risk",
		})
	}

	// Revoke all sessions for security
	if _, revokeErr := s.sessionStore.RevokeAllForUser(ctx, userID); revokeErr != nil {
		observability.LogAndCapture(s.logger, sentry.LevelError, "CRITICAL: failed to revoke sessions after password reset", revokeErr, map[string]string{
			"module": "auth", "operation": "revoke_sessions_after_reset", "user_id": userID, "security": "session_persistence_risk",
		})
	}

	return nil
}

// ChangePassword changes password for authenticated user.
func (s *AuthService) ChangePassword(ctx context.Context, sessionID, currentPassword, newPassword string) error {
	// Validate password complexity
	if err := ValidatePasswordComplexity(newPassword); err != nil {
		return err
	}

	// Get session
	session, err := s.sessionStore.Get(ctx, sessionID)
	if err != nil {
		return err
	}

	// Verify current password
	creds, err := s.getCredentialsByUserID(ctx, session.UserID)
	if err != nil {
		return ErrInvalidCredentials
	}

	valid, err := VerifyPassword(currentPassword, creds.PasswordHash)
	if err != nil || !valid {
		return ErrInvalidCredentials
	}

	// Hash new password
	passwordHash, err := HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Update password
	if err := s.updatePassword(ctx, session.UserID, passwordHash); err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	// Revoke other sessions (keep current)
	if _, revokeErr := s.sessionStore.RevokeAllExcept(ctx, session.UserID, sessionID); revokeErr != nil {
		observability.LogAndCapture(s.logger, sentry.LevelError, "CRITICAL: failed to revoke other sessions after password change", revokeErr, map[string]string{
			"module": "auth", "operation": "revoke_sessions_after_change", "user_id": session.UserID, "security": "session_persistence_risk",
		})
	}

	return nil
}

// VerifyEmail verifies email address with token.
func (s *AuthService) VerifyEmail(ctx context.Context, token string) (*User, error) {
	// Find and validate token
	userID, err := s.validateEmailVerificationToken(ctx, HashToken(token))
	if err != nil {
		return nil, ErrInvalidToken
	}

	// Mark email as verified
	if err := s.markEmailVerified(ctx, userID); err != nil {
		return nil, fmt.Errorf("failed to verify email: %w", err)
	}

	// Mark token as used
	if markErr := s.markEmailVerificationTokenUsed(ctx, HashToken(token)); markErr != nil {
		observability.LogAndCapture(s.logger, sentry.LevelWarning, "failed to mark email verification token as used", markErr, map[string]string{
			"module": "auth", "operation": "mark_verification_token_used", "security": "token_reuse_risk",
		})
	}

	// Get updated user
	return s.getUser(ctx, userID)
}

// ResendVerificationEmail resends verification email.
func (s *AuthService) ResendVerificationEmail(ctx context.Context, sessionID string) error {
	session, err := s.sessionStore.Get(ctx, sessionID)
	if err != nil {
		return err
	}

	user, err := s.getUser(ctx, session.UserID)
	if err != nil {
		return err
	}

	if user.EmailVerified {
		return nil // Already verified
	}

	return s.sendVerificationEmail(ctx, user.ID, user.Email)
}

// GetCSRFToken returns a new CSRF token for the session.
func (s *AuthService) GetCSRFToken(ctx context.Context, sessionID string) (string, error) {
	return s.sessionStore.GetCSRFToken(ctx, sessionID)
}

// ValidateCSRF validates the CSRF token for a session.
func (s *AuthService) ValidateCSRF(ctx context.Context, sessionID, csrfToken string) (bool, error) {
	return s.sessionStore.ValidateCSRF(ctx, sessionID, csrfToken)
}

// SetAuthCookies sets all authentication cookies on the response.
func (s *AuthService) SetAuthCookies(w http.ResponseWriter, sessionID, refreshToken, csrfToken string) {
	SetSessionCookie(w, sessionID, s.cookieConfig)
	SetRefreshCookie(w, refreshToken, s.cookieConfig)
	SetCSRFCookie(w, csrfToken, s.cookieConfig)
}

// ClearAuthCookies clears all authentication cookies.
func (s *AuthService) ClearAuthCookies(w http.ResponseWriter) {
	ClearAllAuthCookies(w, s.cookieConfig)
}

// Credentials represents user credentials.
type Credentials struct {
	ID              string
	UserID          string
	PasswordHash    string
	FailedAttempts  int
	LockedUntil     *time.Time
	PasswordChanged time.Time
}

// Profile represents a user profile.
type Profile struct {
	ID            string
	Email         string
	DisplayName   string
	EmailVerified bool
	CreatedAt     time.Time
}

// Database helper methods (using PostgreSQL)

func (s *AuthService) getCredentials(ctx context.Context, email string) (*Credentials, error) {
	type credRow struct {
		ID              string  `json:"id"`
		UserID          string  `json:"user_id"`
		PasswordHash    string  `json:"password_hash"`
		FailedAttempts  int     `json:"failed_attempts"`
		LockedUntil     *string `json:"locked_until"`
		PasswordChanged string  `json:"password_changed_at"`
	}

	type profileRow struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	}

	// First get user by email
	var profiles []profileRow
	err := s.db.From("profiles").
		Select("id, email").
		Eq("email", email).
		Limit(1).
		Execute(ctx, &profiles)
	if err != nil || len(profiles) == 0 {
		return nil, ErrUserNotFound
	}

	// Get credentials
	var creds []credRow
	err = s.db.From("user_credentials").
		Select("*").
		Eq("user_id", profiles[0].ID).
		Limit(1).
		Execute(ctx, &creds)
	if err != nil || len(creds) == 0 {
		return nil, ErrUserNotFound
	}

	result := &Credentials{
		ID:             creds[0].ID,
		UserID:         creds[0].UserID,
		PasswordHash:   creds[0].PasswordHash,
		FailedAttempts: creds[0].FailedAttempts,
	}

	if creds[0].LockedUntil != nil {
		t, parseErr := time.Parse(time.RFC3339, *creds[0].LockedUntil)
		if parseErr != nil {
			// CRITICAL: Invalid LockedUntil means brute-force protection could be bypassed.
			// Treat as locked (fail-closed) and report.
			observability.LogAndCapture(s.logger, sentry.LevelError, "CRITICAL: invalid LockedUntil timestamp format, treating as locked", parseErr, map[string]string{
				"module": "auth", "operation": "parse_locked_until", "security": "brute_force_bypass_risk",
			})
			lockedFallback := time.Now().Add(30 * time.Minute)
			result.LockedUntil = &lockedFallback
		} else {
			result.LockedUntil = &t
		}
	}

	return result, nil
}

func (s *AuthService) getCredentialsByUserID(ctx context.Context, userID string) (*Credentials, error) {
	type credRow struct {
		ID             string  `json:"id"`
		UserID         string  `json:"user_id"`
		PasswordHash   string  `json:"password_hash"`
		FailedAttempts int     `json:"failed_attempts"`
		LockedUntil    *string `json:"locked_until"`
	}

	var creds []credRow
	err := s.db.From("user_credentials").
		Select("*").
		Eq("user_id", userID).
		Limit(1).
		Execute(ctx, &creds)
	if err != nil || len(creds) == 0 {
		return nil, ErrUserNotFound
	}

	result := &Credentials{
		ID:             creds[0].ID,
		UserID:         creds[0].UserID,
		PasswordHash:   creds[0].PasswordHash,
		FailedAttempts: creds[0].FailedAttempts,
	}

	if creds[0].LockedUntil != nil {
		t, parseErr := time.Parse(time.RFC3339, *creds[0].LockedUntil)
		if parseErr != nil {
			observability.LogAndCapture(s.logger, sentry.LevelError, "CRITICAL: invalid LockedUntil timestamp format, treating as locked", parseErr, map[string]string{
				"module": "auth", "operation": "parse_locked_until", "security": "brute_force_bypass_risk",
			})
			lockedFallback := time.Now().Add(30 * time.Minute)
			result.LockedUntil = &lockedFallback
		} else {
			result.LockedUntil = &t
		}
	}

	return result, nil
}

func (s *AuthService) getUser(ctx context.Context, userID string) (*User, error) {
	type profileRow struct {
		ID            string  `json:"id"`
		Email         string  `json:"email"`
		DisplayName   *string `json:"display_name"`
		EmailVerified bool    `json:"email_verified"`
		CreatedAt     string  `json:"created_at"`
	}

	type roleRow struct {
		Role string `json:"role"`
	}

	var profiles []profileRow
	err := s.db.From("profiles").
		Select("id, email, display_name, email_verified, created_at").
		Eq("id", userID).
		Limit(1).
		Execute(ctx, &profiles)
	if err != nil || len(profiles) == 0 {
		return nil, ErrUserNotFound
	}

	// Get role
	var roles []roleRow
	if rolesErr := s.db.From("user_roles").
		Select("role").
		Eq("user_id", userID).
		Limit(1).
		Execute(ctx, &roles); rolesErr != nil {
		observability.LogAndCapture(s.logger, sentry.LevelWarning, "failed to query user roles, defaulting to 'user'", rolesErr, map[string]string{
			"module": "auth", "operation": "get_user_roles", "user_id": userID,
		})
	}

	role := "user"
	if len(roles) > 0 {
		role = roles[0].Role
	}

	createdAt, parseErr := time.Parse(time.RFC3339, profiles[0].CreatedAt)
	if parseErr != nil {
		observability.LogAndCapture(s.logger, sentry.LevelWarning, "failed to parse user createdAt timestamp", parseErr, map[string]string{
			"module": "auth", "operation": "parse_created_at", "user_id": userID,
		})
		createdAt = time.Now()
	}

	user := &User{
		ID:            profiles[0].ID,
		Email:         profiles[0].Email,
		Role:          role,
		EmailVerified: profiles[0].EmailVerified,
		CreatedAt:     createdAt,
	}

	if profiles[0].DisplayName != nil {
		user.DisplayName = *profiles[0].DisplayName
	}

	return user, nil
}

func (s *AuthService) getUserByEmail(ctx context.Context, email string) (*User, error) {
	type profileRow struct {
		ID string `json:"id"`
	}

	var profiles []profileRow
	err := s.db.From("profiles").
		Select("id").
		Eq("email", email).
		Limit(1).
		Execute(ctx, &profiles)
	if err != nil || len(profiles) == 0 {
		return nil, ErrUserNotFound
	}

	return s.getUser(ctx, profiles[0].ID)
}

func (s *AuthService) createProfile(ctx context.Context, email, displayName string) (*Profile, error) {
	type profileInput struct {
		Email       string  `json:"email"`
		DisplayName *string `json:"display_name,omitempty"`
		Plan        string  `json:"plan"`
		CreatedAt   string  `json:"created_at"`
		UpdatedAt   string  `json:"updated_at"`
	}

	type profileOutput struct {
		ID string `json:"id"`
	}

	now := time.Now().UTC().Format(time.RFC3339)
	input := profileInput{
		Email:     email,
		Plan:      "free",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if displayName != "" {
		input.DisplayName = &displayName
	}

	var result []profileOutput
	err := s.db.Insert("profiles", input).Execute(ctx, &result)
	if err != nil {
		return nil, err
	}

	if len(result) == 0 {
		return nil, errors.New("failed to create profile")
	}

	return &Profile{
		ID:          result[0].ID,
		Email:       email,
		DisplayName: displayName,
		CreatedAt:   time.Now(),
	}, nil
}

func (s *AuthService) createCredentials(ctx context.Context, userID, passwordHash string) error {
	type credInput struct {
		UserID            string `json:"user_id"`
		PasswordHash      string `json:"password_hash"`
		PasswordChangedAt string `json:"password_changed_at"`
		CreatedAt         string `json:"created_at"`
	}

	now := time.Now().UTC().Format(time.RFC3339)
	input := credInput{
		UserID:            userID,
		PasswordHash:      passwordHash,
		PasswordChangedAt: now,
		CreatedAt:         now,
	}

	return s.db.Insert("user_credentials", input).Execute(ctx, nil)
}

func (s *AuthService) incrementFailedAttempts(ctx context.Context, userID string) error {
	// Get current count
	creds, err := s.getCredentialsByUserID(ctx, userID)
	if err != nil {
		return err
	}

	newCount := creds.FailedAttempts + 1
	updates := map[string]any{
		"failed_attempts": newCount,
	}

	// Lock account after 5 failures
	if newCount >= 5 {
		lockUntil := time.Now().Add(30 * time.Minute).UTC()
		updates["locked_until"] = lockUntil.Format(time.RFC3339)

		// SECURITY AUDIT: Log account lockout event for monitoring
		// Note: userID is logged (not email) to allow correlation without PII
		s.logger.Warn("Account locked due to excessive failed login attempts",
			zap.String("event", "account_lockout"),
			zap.String("userID", userID),
			zap.Int("failedAttempts", newCount),
			zap.Time("lockedUntil", lockUntil),
			zap.Duration("lockDuration", 30*time.Minute),
		)
	} else if newCount >= 3 {
		// Log warning at 3 failures as early indicator
		s.logger.Info("Multiple failed login attempts detected",
			zap.String("event", "login_failures_warning"),
			zap.String("userID", userID),
			zap.Int("failedAttempts", newCount),
			zap.Int("lockoutThreshold", 5),
		)
	}

	return s.db.Update("user_credentials", updates).
		Eq("user_id", userID).
		Execute(ctx, nil)
}

func (s *AuthService) resetFailedAttempts(ctx context.Context, userID string) error {
	updates := map[string]any{
		"failed_attempts": 0,
		"locked_until":    nil,
	}

	return s.db.Update("user_credentials", updates).
		Eq("user_id", userID).
		Execute(ctx, nil)
}

func (s *AuthService) updatePassword(ctx context.Context, userID, passwordHash string) error {
	updates := map[string]any{
		"password_hash":       passwordHash,
		"password_changed_at": time.Now().UTC().Format(time.RFC3339),
		"failed_attempts":     0,
		"locked_until":        nil,
	}

	return s.db.Update("user_credentials", updates).
		Eq("user_id", userID).
		Execute(ctx, nil)
}

func (s *AuthService) markEmailVerified(ctx context.Context, userID string) error {
	updates := map[string]any{
		"email_verified": true,
		"updated_at":     time.Now().UTC().Format(time.RFC3339),
	}

	return s.db.Update("profiles", updates).
		Eq("id", userID).
		Execute(ctx, nil)
}

func (s *AuthService) storePasswordResetToken(ctx context.Context, userID, tokenHash string) error {
	type tokenInput struct {
		UserID    string `json:"user_id"`
		TokenHash string `json:"token_hash"`
		ExpiresAt string `json:"expires_at"`
		CreatedAt string `json:"created_at"`
	}

	now := time.Now().UTC()
	input := tokenInput{
		UserID:    userID,
		TokenHash: tokenHash,
		ExpiresAt: now.Add(1 * time.Hour).Format(time.RFC3339),
		CreatedAt: now.Format(time.RFC3339),
	}

	return s.db.Insert("password_reset_tokens", input).Execute(ctx, nil)
}

func (s *AuthService) validatePasswordResetToken(ctx context.Context, tokenHash string) (string, error) {
	type tokenRow struct {
		UserID    string  `json:"user_id"`
		ExpiresAt string  `json:"expires_at"`
		UsedAt    *string `json:"used_at"`
	}

	var tokens []tokenRow
	err := s.db.From("password_reset_tokens").
		Select("user_id, expires_at, used_at").
		Eq("token_hash", tokenHash).
		Limit(1).
		Execute(ctx, &tokens)
	if err != nil || len(tokens) == 0 {
		return "", ErrInvalidToken
	}

	token := tokens[0]
	if token.UsedAt != nil {
		return "", ErrInvalidToken
	}

	expiresAt, parseErr := time.Parse(time.RFC3339, token.ExpiresAt)
	if parseErr != nil {
		observability.LogAndCapture(s.logger, sentry.LevelError, "invalid password reset token ExpiresAt format, rejecting token", parseErr, map[string]string{
			"module": "auth", "operation": "parse_reset_token_expiry", "security": "token_expiry_bypass_risk",
		})
		return "", ErrInvalidToken
	}
	if time.Now().After(expiresAt) {
		return "", ErrInvalidToken
	}

	return token.UserID, nil
}

func (s *AuthService) markPasswordResetTokenUsed(ctx context.Context, tokenHash string) error {
	updates := map[string]any{
		"used_at": time.Now().UTC().Format(time.RFC3339),
	}

	return s.db.Update("password_reset_tokens", updates).
		Eq("token_hash", tokenHash).
		Execute(ctx, nil)
}

func (s *AuthService) storeEmailVerificationToken(ctx context.Context, userID, tokenHash string) error {
	type tokenInput struct {
		UserID    string `json:"user_id"`
		TokenHash string `json:"token_hash"`
		ExpiresAt string `json:"expires_at"`
		CreatedAt string `json:"created_at"`
	}

	now := time.Now().UTC()
	input := tokenInput{
		UserID:    userID,
		TokenHash: tokenHash,
		ExpiresAt: now.Add(24 * time.Hour).Format(time.RFC3339),
		CreatedAt: now.Format(time.RFC3339),
	}

	return s.db.Insert("email_verification_tokens", input).Execute(ctx, nil)
}

func (s *AuthService) validateEmailVerificationToken(ctx context.Context, tokenHash string) (string, error) {
	type tokenRow struct {
		UserID    string  `json:"user_id"`
		ExpiresAt string  `json:"expires_at"`
		UsedAt    *string `json:"used_at"`
	}

	var tokens []tokenRow
	err := s.db.From("email_verification_tokens").
		Select("user_id, expires_at, used_at").
		Eq("token_hash", tokenHash).
		Limit(1).
		Execute(ctx, &tokens)
	if err != nil || len(tokens) == 0 {
		return "", ErrInvalidToken
	}

	token := tokens[0]
	if token.UsedAt != nil {
		return "", ErrInvalidToken
	}

	expiresAt, parseErr := time.Parse(time.RFC3339, token.ExpiresAt)
	if parseErr != nil {
		observability.LogAndCapture(s.logger, sentry.LevelError, "invalid email verification token ExpiresAt format, rejecting token", parseErr, map[string]string{
			"module": "auth", "operation": "parse_verification_token_expiry", "security": "token_expiry_bypass_risk",
		})
		return "", ErrInvalidToken
	}
	if time.Now().After(expiresAt) {
		return "", ErrInvalidToken
	}

	return token.UserID, nil
}

func (s *AuthService) markEmailVerificationTokenUsed(ctx context.Context, tokenHash string) error {
	updates := map[string]any{
		"used_at": time.Now().UTC().Format(time.RFC3339),
	}

	return s.db.Update("email_verification_tokens", updates).
		Eq("token_hash", tokenHash).
		Execute(ctx, nil)
}

func (s *AuthService) sendVerificationEmail(ctx context.Context, userID, userEmail string) error {
	// Generate token
	token, err := GenerateVerificationToken()
	if err != nil {
		return err
	}

	// Store token hash
	if err := s.storeEmailVerificationToken(ctx, userID, HashToken(token)); err != nil {
		return err
	}

	// Build verification link
	verificationLink := fmt.Sprintf("%s/verify-email?token=%s", s.publicBaseURL, token)

	// Send email if service is available
	if s.emailService != nil {
		// Get user name for personalization (non-critical, continue without name on failure)
		user, userErr := s.getUser(ctx, userID)
		if userErr != nil {
			observability.LogAndCapture(s.logger, sentry.LevelWarning, "failed to get user for email personalization", userErr, map[string]string{
				"module": "auth", "operation": "get_user_for_verification_email", "user_id": userID,
			})
		}
		name := ""
		if user != nil {
			name = user.DisplayName
		}

		subject := "Verifique seu email - Automatiza"
		body := buildVerificationEmailHTML(name, verificationLink)

		if err := s.emailService.SendCustomEmail(ctx, userEmail, subject, body); err != nil {
			s.logger.Error("Failed to send verification email",
				zap.String("email", userEmail),
				zap.Error(err))
			return fmt.Errorf("failed to send verification email: %w", err)
		}

		s.logger.Info("Verification email sent successfully",
			zap.String("email", userEmail))
	} else {
		s.logger.Warn("Email service not configured, verification email not sent",
			zap.String("email", userEmail),
			zap.String("token", token[:8]+"..."))
	}

	return nil
}

func (s *AuthService) sendPasswordResetEmail(ctx context.Context, userEmail, token string) error {
	// Build reset link
	resetLink := fmt.Sprintf("%s/reset-password?token=%s", s.publicBaseURL, token)

	// Send email if service is available
	if s.emailService != nil {
		// Get user name for personalization (non-critical, continue without name on failure)
		user, userErr := s.getUserByEmail(ctx, userEmail)
		if userErr != nil && !errors.Is(userErr, ErrUserNotFound) {
			observability.LogAndCapture(s.logger, sentry.LevelWarning, "failed to get user for reset email personalization", userErr, map[string]string{
				"module": "auth", "operation": "get_user_for_reset_email",
			})
		}
		name := ""
		if user != nil {
			name = user.DisplayName
		}

		if err := s.emailService.SendPasswordResetEmail(ctx, userEmail, name, resetLink); err != nil {
			s.logger.Error("Failed to send password reset email",
				zap.String("email", userEmail),
				zap.Error(err))
			return fmt.Errorf("failed to send password reset email: %w", err)
		}

		s.logger.Info("Password reset email sent successfully",
			zap.String("email", userEmail))
	} else {
		s.logger.Warn("Email service not configured, password reset email not sent",
			zap.String("email", userEmail),
			zap.String("token", token[:8]+"..."))
	}

	return nil
}

// SendMagicLink generates and sends a magic link to the user's email.
func (s *AuthService) SendMagicLink(ctx context.Context, email string) error {
	// Check if user exists
	user, err := s.getUserByEmail(ctx, email)
	if err != nil || user == nil {
		return ErrUserNotFound
	}

	// Check rate limit
	rateLimitKey := fmt.Sprintf("magic_link:%s", email)
	result, err := s.rateLimiter.Check(ctx, rateLimitKey)
	if err != nil {
		s.logger.Error("Failed to check rate limit", zap.Error(err))
	} else if !result.Allowed {
		return ErrRateLimited
	}

	// Generate magic link token
	token, err := GenerateMagicLinkToken()
	if err != nil {
		return fmt.Errorf("failed to generate magic link token: %w", err)
	}

	// Store token hash
	if err := s.storeMagicLinkToken(ctx, email, HashToken(token)); err != nil {
		return fmt.Errorf("failed to store magic link token: %w", err)
	}

	// Increment rate limit
	if _, rlErr := s.rateLimiter.Increment(ctx, rateLimitKey); rlErr != nil {
		observability.LogAndCapture(s.logger, sentry.LevelError, "CRITICAL: magic link rate limiter increment failed", rlErr, map[string]string{
			"module": "auth", "operation": "magic_link_rate_limit_increment", "security": "rate_limit_bypass_risk",
		})
	}

	// Send email (async)
	magicEmail := email
	observability.SafeGo("auth-magic-link-email", func() {
		bgCtx := context.Background()
		if sendErr := s.sendMagicLinkEmail(bgCtx, magicEmail, token); sendErr != nil {
			observability.LogAndCapture(s.logger, sentry.LevelError, "failed to send magic link email", sendErr, map[string]string{
				"module": "auth", "operation": "send_magic_link_email",
			})
		}
	})

	return nil
}

// MagicLinkResult contains the result of a successful magic link verification.
type MagicLinkResult struct {
	User         *User
	Session      *Session
	RefreshToken string
	CSRFToken    string
}

// VerifyMagicLink validates a magic link token and creates a session.
func (s *AuthService) VerifyMagicLink(ctx context.Context, token, ipAddress, userAgent, deviceFingerprint string) (*MagicLinkResult, error) {
	// Find and validate token
	email, err := s.validateMagicLinkToken(ctx, HashToken(token))
	if err != nil {
		return nil, ErrInvalidToken
	}

	// Get user
	user, err := s.getUserByEmail(ctx, email)
	if err != nil {
		return nil, ErrUserNotFound
	}

	// Mark token as used
	if markErr := s.markMagicLinkTokenUsed(ctx, HashToken(token)); markErr != nil {
		observability.LogAndCapture(s.logger, sentry.LevelWarning, "failed to mark magic link token as used", markErr, map[string]string{
			"module": "auth", "operation": "mark_magic_link_token_used", "security": "token_reuse_risk",
		})
	}

	// Create session
	session, refreshToken, csrfToken, err := s.sessionStore.Create(ctx, user.ID, SessionMetadata{
		DeviceFingerprint: deviceFingerprint,
		IPAddress:         ipAddress,
		UserAgent:         userAgent,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	return &MagicLinkResult{
		User:         user,
		Session:      session,
		RefreshToken: refreshToken,
		CSRFToken:    csrfToken,
	}, nil
}

func (s *AuthService) storeMagicLinkToken(ctx context.Context, email, tokenHash string) error {
	type tokenInput struct {
		Email     string `json:"email"`
		TokenHash string `json:"token_hash"`
		ExpiresAt string `json:"expires_at"`
		CreatedAt string `json:"created_at"`
	}

	now := time.Now().UTC()
	input := tokenInput{
		Email:     email,
		TokenHash: tokenHash,
		ExpiresAt: now.Add(15 * time.Minute).Format(time.RFC3339), // 15-minute expiry
		CreatedAt: now.Format(time.RFC3339),
	}

	return s.db.Insert("magic_link_tokens", input).Execute(ctx, nil)
}

func (s *AuthService) validateMagicLinkToken(ctx context.Context, tokenHash string) (string, error) {
	type tokenRow struct {
		Email     string  `json:"email"`
		ExpiresAt string  `json:"expires_at"`
		UsedAt    *string `json:"used_at"`
	}

	var tokens []tokenRow
	err := s.db.From("magic_link_tokens").
		Select("email, expires_at, used_at").
		Eq("token_hash", tokenHash).
		Limit(1).
		Execute(ctx, &tokens)
	if err != nil || len(tokens) == 0 {
		return "", ErrInvalidToken
	}

	token := tokens[0]
	if token.UsedAt != nil {
		return "", ErrInvalidToken
	}

	expiresAt, parseErr := time.Parse(time.RFC3339, token.ExpiresAt)
	if parseErr != nil {
		observability.LogAndCapture(s.logger, sentry.LevelError, "invalid magic link token ExpiresAt format, rejecting token", parseErr, map[string]string{
			"module": "auth", "operation": "parse_magic_link_token_expiry", "security": "token_expiry_bypass_risk",
		})
		return "", ErrInvalidToken
	}
	if time.Now().After(expiresAt) {
		return "", ErrInvalidToken
	}

	return token.Email, nil
}

func (s *AuthService) markMagicLinkTokenUsed(ctx context.Context, tokenHash string) error {
	updates := map[string]any{
		"used_at": time.Now().UTC().Format(time.RFC3339),
	}

	return s.db.Update("magic_link_tokens", updates).
		Eq("token_hash", tokenHash).
		Execute(ctx, nil)
}

func (s *AuthService) sendMagicLinkEmail(ctx context.Context, email, token string) error {
	// Build magic link URL (matches frontend route /auth/magic)
	magicLink := fmt.Sprintf("%s/auth/magic?token=%s", s.publicBaseURL, token)

	// Send email if service is available
	if s.emailService != nil {
		// Get user name for personalization (non-critical, continue without name on failure)
		user, userErr := s.getUserByEmail(ctx, email)
		if userErr != nil && !errors.Is(userErr, ErrUserNotFound) {
			observability.LogAndCapture(s.logger, sentry.LevelWarning, "failed to get user for magic link email personalization", userErr, map[string]string{
				"module": "auth", "operation": "get_user_for_magic_link_email",
			})
		}
		name := ""
		if user != nil {
			name = user.DisplayName
		}

		subject := "Seu link de acesso - Automatiza"
		body := buildMagicLinkEmailHTML(name, magicLink)

		if err := s.emailService.SendCustomEmail(ctx, email, subject, body); err != nil {
			s.logger.Error("Failed to send magic link email",
				zap.String("email", email),
				zap.Error(err))
			return fmt.Errorf("failed to send magic link email: %w", err)
		}

		s.logger.Info("Magic link email sent successfully",
			zap.String("email", email))
	} else {
		s.logger.Warn("Email service not configured, magic link email not sent",
			zap.String("email", email),
			zap.String("token", token[:8]+"..."))
	}

	return nil
}

// buildMagicLinkEmailHTML builds the HTML body for magic link emails
func buildMagicLinkEmailHTML(name, magicLink string) string {
	greeting := "Olá"
	if name != "" {
		greeting = "Olá, " + name
	}

	return `<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
  <title>Seu link de acesso</title>
</head>
<body style="font-family: Arial, sans-serif; line-height: 1.6; color: #333; max-width: 600px; margin: 0 auto; padding: 20px;">
  <h1 style="color: #2563eb;">` + greeting + `!</h1>
  <p>Você solicitou um link de acesso ao Automatiza. Clique no botão abaixo para entrar na sua conta:</p>
  <p style="text-align: center; margin: 30px 0;">
    <a href="` + magicLink + `" style="background-color: #2563eb; color: white; padding: 12px 24px; text-decoration: none; border-radius: 6px; display: inline-block;">Acessar minha conta</a>
  </p>
  <p>Ou copie e cole o link abaixo no seu navegador:</p>
  <p style="word-break: break-all; color: #666;">` + magicLink + `</p>
  <p style="color: #666; font-size: 12px; margin-top: 30px;">Este link expira em 15 minutos. Se você não solicitou este acesso, ignore este email.</p>
</body>
</html>`
}

// DBSessionAdapter adapts DBClient to SessionDB interface.
type DBSessionAdapter struct {
	client *config.DBClient
}

func (a *DBSessionAdapter) CreateSession(ctx context.Context, session *Session) error {
	input := map[string]any{
		"id":                 session.ID,
		"user_id":            session.UserID,
		"refresh_token_hash": session.RefreshTokenHash,
		"csrf_token_hash":    session.CSRFTokenHash,
		"expires_at":         session.ExpiresAt.Format(time.RFC3339),
		"created_at":         session.CreatedAt.Format(time.RFC3339),
		"last_activity_at":   session.LastActivityAt.Format(time.RFC3339),
	}

	if session.DeviceFingerprint != "" {
		input["device_fingerprint"] = session.DeviceFingerprint
	}
	if session.IPAddress != "" {
		input["ip_address"] = session.IPAddress
	}
	if session.UserAgent != "" {
		input["user_agent"] = session.UserAgent
	}

	return a.client.Insert("sessions", input).Execute(ctx, nil)
}

func (a *DBSessionAdapter) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	type sessionRow struct {
		ID                string  `json:"id"`
		UserID            string  `json:"user_id"`
		RefreshTokenHash  string  `json:"refresh_token_hash"`
		CSRFTokenHash     string  `json:"csrf_token_hash"`
		DeviceFingerprint *string `json:"device_fingerprint"`
		IPAddress         *string `json:"ip_address"`
		UserAgent         *string `json:"user_agent"`
		ExpiresAt         string  `json:"expires_at"`
		CreatedAt         string  `json:"created_at"`
		LastActivityAt    string  `json:"last_activity_at"`
		RevokedAt         *string `json:"revoked_at"`
	}

	var rows []sessionRow
	err := a.client.From("sessions").
		Select("*").
		Eq("id", sessionID).
		Limit(1).
		Execute(ctx, &rows)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}

	row := rows[0]
	expiresAt, err := time.Parse(time.RFC3339, row.ExpiresAt)
	if err != nil {
		observability.CaptureMessage(sentry.LevelWarning, "invalid session ExpiresAt timestamp", map[string]string{
			"module": "auth", "operation": "parse_session_expires_at", "session_id": row.ID,
		})
		expiresAt = time.Now()
	}
	createdAt, err := time.Parse(time.RFC3339, row.CreatedAt)
	if err != nil {
		createdAt = time.Now()
	}
	lastActivityAt, err := time.Parse(time.RFC3339, row.LastActivityAt)
	if err != nil {
		lastActivityAt = time.Now()
	}

	session := &Session{
		ID:               row.ID,
		UserID:           row.UserID,
		RefreshTokenHash: row.RefreshTokenHash,
		CSRFTokenHash:    row.CSRFTokenHash,
		ExpiresAt:        expiresAt,
		CreatedAt:        createdAt,
		LastActivityAt:   lastActivityAt,
	}

	if row.DeviceFingerprint != nil {
		session.DeviceFingerprint = *row.DeviceFingerprint
	}
	if row.IPAddress != nil {
		session.IPAddress = *row.IPAddress
	}
	if row.UserAgent != nil {
		session.UserAgent = *row.UserAgent
	}
	if row.RevokedAt != nil {
		revokedAt, parseErr := time.Parse(time.RFC3339, *row.RevokedAt)
		if parseErr == nil {
			session.RevokedAt = &revokedAt
		}
	}

	return session, nil
}

func (a *DBSessionAdapter) GetSessionsByUserID(ctx context.Context, userID string) ([]*Session, error) {
	type sessionRow struct {
		ID                string  `json:"id"`
		UserID            string  `json:"user_id"`
		RefreshTokenHash  string  `json:"refresh_token_hash"`
		CSRFTokenHash     string  `json:"csrf_token_hash"`
		DeviceFingerprint *string `json:"device_fingerprint"`
		IPAddress         *string `json:"ip_address"`
		UserAgent         *string `json:"user_agent"`
		ExpiresAt         string  `json:"expires_at"`
		CreatedAt         string  `json:"created_at"`
		LastActivityAt    string  `json:"last_activity_at"`
		RevokedAt         *string `json:"revoked_at"`
	}

	var rows []sessionRow
	err := a.client.From("sessions").
		Select("*").
		Eq("user_id", userID).
		Order("created_at", false).
		Execute(ctx, &rows)
	if err != nil {
		return nil, err
	}

	sessions := make([]*Session, 0, len(rows))
	for _, row := range rows {
		expiresAt, parseErr := time.Parse(time.RFC3339, row.ExpiresAt)
		if parseErr != nil {
			expiresAt = time.Now()
		}
		createdAt, parseErr := time.Parse(time.RFC3339, row.CreatedAt)
		if parseErr != nil {
			createdAt = time.Now()
		}
		lastActivityAt, parseErr := time.Parse(time.RFC3339, row.LastActivityAt)
		if parseErr != nil {
			lastActivityAt = time.Now()
		}

		session := &Session{
			ID:               row.ID,
			UserID:           row.UserID,
			RefreshTokenHash: row.RefreshTokenHash,
			CSRFTokenHash:    row.CSRFTokenHash,
			ExpiresAt:        expiresAt,
			CreatedAt:        createdAt,
			LastActivityAt:   lastActivityAt,
		}

		if row.DeviceFingerprint != nil {
			session.DeviceFingerprint = *row.DeviceFingerprint
		}
		if row.IPAddress != nil {
			session.IPAddress = *row.IPAddress
		}
		if row.UserAgent != nil {
			session.UserAgent = *row.UserAgent
		}
		if row.RevokedAt != nil {
			revokedAt, rvParseErr := time.Parse(time.RFC3339, *row.RevokedAt)
			if rvParseErr == nil {
				session.RevokedAt = &revokedAt
			}
		}

		sessions = append(sessions, session)
	}

	return sessions, nil
}

func (a *DBSessionAdapter) UpdateSession(ctx context.Context, session *Session) error {
	updates := map[string]any{
		"refresh_token_hash": session.RefreshTokenHash,
		"csrf_token_hash":    session.CSRFTokenHash,
		"expires_at":         session.ExpiresAt.Format(time.RFC3339),
		"last_activity_at":   session.LastActivityAt.Format(time.RFC3339),
	}

	return a.client.Update("sessions", updates).
		Eq("id", session.ID).
		Execute(ctx, nil)
}

func (a *DBSessionAdapter) RevokeSession(ctx context.Context, sessionID string, revokedAt time.Time) error {
	updates := map[string]any{
		"revoked_at": revokedAt.Format(time.RFC3339),
	}

	return a.client.Update("sessions", updates).
		Eq("id", sessionID).
		Execute(ctx, nil)
}

func (a *DBSessionAdapter) RevokeSessionsByUserID(ctx context.Context, userID string, revokedAt time.Time) (int, error) {
	updates := map[string]any{
		"revoked_at": revokedAt.Format(time.RFC3339),
	}

	// Note: Current implementation doesn't return affected row count
	err := a.client.Update("sessions", updates).
		Eq("user_id", userID).
		Execute(ctx, nil)

	return -1, err
}

func (a *DBSessionAdapter) RevokeSessionsExcept(ctx context.Context, userID, exceptSessionID string, revokedAt time.Time) (int, error) {
	updates := map[string]any{
		"revoked_at": revokedAt.Format(time.RFC3339),
	}

	err := a.client.Update("sessions", updates).
		Eq("user_id", userID).
		Execute(ctx, nil)

	// Also update the except session back to null (workaround)
	nullUpdates := map[string]any{
		"revoked_at": nil,
	}
	if workaroundErr := a.client.Update("sessions", nullUpdates).
		Eq("id", exceptSessionID).
		Execute(ctx, nil); workaroundErr != nil {
		observability.CaptureWithTags(workaroundErr, map[string]string{
			"module": "auth", "operation": "restore_excepted_session", "session_id": exceptSessionID,
		})
	}

	return -1, err
}

func (a *DBSessionAdapter) DeleteExpiredSessions(ctx context.Context) error {
	cutoff := time.Now().Add(-24 * time.Hour).Format(time.RFC3339)

	return a.client.Delete("sessions").
		Eq("expires_at", cutoff). // This won't work with Lt, using RPC would be better
		Execute(ctx)
}

// buildVerificationEmailHTML builds the HTML body for verification emails
func buildVerificationEmailHTML(name, verificationLink string) string {
	greeting := "Olá"
	if name != "" {
		greeting = "Olá, " + name
	}

	return `<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
  <title>Verificação de Email</title>
</head>
<body style="font-family: Arial, sans-serif; line-height: 1.6; color: #333; max-width: 600px; margin: 0 auto; padding: 20px;">
  <h1 style="color: #2563eb;">` + greeting + `!</h1>
  <p>Obrigado por se cadastrar no Automatiza. Para completar seu cadastro, por favor verifique seu email clicando no botão abaixo:</p>
  <p style="text-align: center; margin: 30px 0;">
    <a href="` + verificationLink + `" style="background-color: #2563eb; color: white; padding: 12px 24px; text-decoration: none; border-radius: 6px; display: inline-block;">Verificar Email</a>
  </p>
  <p>Ou copie e cole o link abaixo no seu navegador:</p>
  <p style="word-break: break-all; color: #666;">` + verificationLink + `</p>
  <p style="color: #666; font-size: 12px; margin-top: 30px;">Se você não criou uma conta no Automatiza, ignore este email.</p>
</body>
</html>`
}
