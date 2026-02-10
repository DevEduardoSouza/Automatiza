package profile

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"automatiza-go/internal/adapters/email"
	"automatiza-go/internal/auth"
	"automatiza-go/internal/repository"

	"go.uber.org/zap"
)

const (
	// TokenLength is the number of random bytes for token generation (256 bits)
	TokenLength = 32
	// TokenExpiry is how long a reset token is valid
	TokenExpiry = 15 * time.Minute
	// MaxResetAttempts per IP/email within the rate limit window
	MaxResetAttempts = 3
	// RateLimitWindow is the time window for rate limiting
	RateLimitWindow = 15 * time.Minute
	// MinPasswordLength is the minimum password length
	MinPasswordLength = 8
)

// PasswordService handles password management operations
type PasswordService struct {
	profileRepo   repository.ProfileRepository
	tokenRepo     repository.PasswordResetTokenRepository
	rateLimitRepo repository.RateLimitRepository
	emailProvider email.EmailProvider
	hmacKey       []byte
	frontendURL   string
	logger        *zap.Logger
}

// PasswordServiceConfig holds configuration for PasswordService
type PasswordServiceConfig struct {
	HMACKey     []byte // 32-byte key for HMAC-SHA256
	FrontendURL string // e.g., "https://app.automatiza.com"
}

// NewPasswordService creates a new password service
func NewPasswordService(
	profileRepo repository.ProfileRepository,
	tokenRepo repository.PasswordResetTokenRepository,
	rateLimitRepo repository.RateLimitRepository,
	emailProvider email.EmailProvider,
	config PasswordServiceConfig,
	logger *zap.Logger,
) *PasswordService {
	return &PasswordService{
		profileRepo:   profileRepo,
		tokenRepo:     tokenRepo,
		rateLimitRepo: rateLimitRepo,
		emailProvider: emailProvider,
		hmacKey:       config.HMACKey,
		frontendURL:   config.FrontendURL,
		logger:        logger,
	}
}

// RequestPasswordReset initiates the password reset flow
// Returns nil even if email doesn't exist (silent fail for security)
func (s *PasswordService) RequestPasswordReset(ctx context.Context, input RequestResetInput) error {
	// 1. Check rate limits (CLAUDE.md: rate limit per IP/user)
	if err := s.checkRateLimit(ctx, input.Email, input.IPAddress); err != nil {
		return err
	}

	// 2. Look up user by email (silent fail if not found)
	profile, err := s.profileRepo.GetProfileByEmail(ctx, input.Email)
	if err != nil {
		// SECURITY: Don't log raw email (PII) - use masked version
		s.logger.Debug("error looking up profile by email",
			zap.String("email_masked", maskEmail(input.Email)),
			zap.Error(err))
		return nil // Silent fail
	}
	if profile == nil {
		// SECURITY: Don't log raw email (PII) - use masked version
		s.logger.Debug("profile not found for email, silent fail",
			zap.String("email_masked", maskEmail(input.Email)))
		return nil // Silent fail - don't reveal if email exists
	}

	// 3. Invalidate any existing tokens for this user
	if err := s.tokenRepo.InvalidateUserTokens(ctx, profile.ID); err != nil {
		s.logger.Warn("failed to invalidate existing tokens",
			zap.String("userID", profile.ID),
			zap.Error(err))
		// Continue anyway
	}

	// 4. Generate token with high entropy (CLAUDE.md: CSPRNG)
	tokenBytes := make([]byte, TokenLength)
	if _, err := rand.Read(tokenBytes); err != nil {
		return fmt.Errorf("failed to generate token: %w", err)
	}
	token := base64.URLEncoding.EncodeToString(tokenBytes)

	// 5. Hash the token for storage (never store raw token)
	tokenHash := s.hashToken(token)

	// 6. Store the token
	expiresAt := time.Now().Add(TokenExpiry)
	if err := s.tokenRepo.Create(ctx, &repository.PasswordResetToken{
		UserID:    profile.ID,
		TokenHash: tokenHash,
		ExpiresAt: expiresAt,
		IPAddress: input.IPAddress,
		UserAgent: input.UserAgent,
	}); err != nil {
		return fmt.Errorf("failed to store token: %w", err)
	}

	// 7. Send reset email
	resetURL := fmt.Sprintf("%s/reset-password?token=%s", s.frontendURL, token)
	if err := s.emailProvider.SendPasswordResetEmail(ctx, email.PasswordResetEmailInput{
		To:        profile.Email,
		UserName:  profile.Email, // Use email as name if no name field
		ResetURL:  resetURL,
		ExpiresIn: "15 minutos",
		IPAddress: input.IPAddress,
		UserAgent: input.UserAgent,
	}); err != nil {
		// SECURITY: Don't log raw email (PII) - use userID for correlation
		s.logger.Error("failed to send reset email",
			zap.String("userID", profile.ID),
			zap.Error(err))
		// Don't return error - token was created, user might retry
	}

	// 8. Increment rate limit counters (best effort - don't fail if counter update fails)
	_ = s.rateLimitRepo.Increment(ctx, input.Email, "email")
	_ = s.rateLimitRepo.Increment(ctx, input.IPAddress, "ip")

	s.logger.Info("password reset requested",
		zap.String("userID", profile.ID),
		zap.String("ip", input.IPAddress))

	return nil
}

// ValidateResetToken checks if a token is valid
func (s *PasswordService) ValidateResetToken(ctx context.Context, token string) (*ValidateTokenResult, error) {
	tokenHash := s.hashToken(token)

	tokenRecord, err := s.tokenRepo.GetByHash(ctx, tokenHash)
	if err != nil {
		s.logger.Debug("token not found",
			zap.Error(err))
		return &ValidateTokenResult{Valid: false, Reason: "invalid_token"}, nil
	}

	if tokenRecord.UsedAt != nil {
		return &ValidateTokenResult{Valid: false, Reason: "token_used"}, nil
	}

	if time.Now().After(tokenRecord.ExpiresAt) {
		return &ValidateTokenResult{Valid: false, Reason: "token_expired"}, nil
	}

	// Mask email for display (show first 2 chars + *** + domain)
	maskedEmail := maskEmail(tokenRecord.Email)

	return &ValidateTokenResult{
		Valid: true,
		Email: maskedEmail,
	}, nil
}

// ResetPassword sets a new password using a reset token
func (s *PasswordService) ResetPassword(ctx context.Context, input ResetPasswordInput) error {
	// 1. Validate token
	result, err := s.ValidateResetToken(ctx, input.Token)
	if err != nil {
		return err
	}
	if !result.Valid {
		switch result.Reason {
		case "token_expired":
			return ErrTokenExpired
		case "token_used":
			return ErrTokenUsed
		default:
			return ErrInvalidToken
		}
	}

	// 2. Validate password strength
	if err := s.validatePasswordStrength(input.NewPassword); err != nil {
		return err
	}

	// 3. Get the token record to find user ID
	tokenHash := s.hashToken(input.Token)
	tokenRecord, err := s.tokenRepo.GetByHash(ctx, tokenHash)
	if err != nil {
		return ErrInvalidToken
	}

	// 4. Hash the new password using Argon2id (via auth package)
	hashedPassword, err := auth.HashPassword(input.NewPassword)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// 5. Update password in database
	if err := s.profileRepo.UpdatePassword(ctx, tokenRecord.UserID, hashedPassword); err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	// 6. Mark token as used
	if err := s.tokenRepo.MarkUsed(ctx, tokenHash); err != nil {
		s.logger.Warn("failed to mark token as used",
			zap.String("userID", tokenRecord.UserID),
			zap.Error(err))
	}

	// 7. Send confirmation email
	profile, _ := s.profileRepo.GetProfile(ctx, tokenRecord.UserID)
	if profile != nil {
		if err := s.emailProvider.SendPasswordChangedEmail(ctx, email.PasswordChangedEmailInput{
			To:        profile.Email,
			UserName:  profile.Email,
			IPAddress: input.IPAddress,
			Timestamp: time.Now().Format("02/01/2006 15:04:05"),
		}); err != nil {
			// SECURITY: Don't log raw email (PII) - use userID for correlation
			s.logger.Warn("failed to send password changed notification",
				zap.String("userID", profile.ID),
				zap.Error(err))
		}
	}

	s.logger.Info("password reset completed",
		zap.String("userID", tokenRecord.UserID),
		zap.String("ip", input.IPAddress))

	return nil
}

// ChangePassword changes password for a logged-in user
func (s *PasswordService) ChangePassword(ctx context.Context, userID string, input ChangePasswordInput) error {
	// 1. Get user profile
	profile, err := s.profileRepo.GetProfile(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get profile: %w", err)
	}
	if profile == nil {
		return ErrProfileNotFound
	}

	// 2. Verify current password if user has one set
	if profile.PasswordHash != nil && *profile.PasswordHash != "" {
		// Use auth.VerifyPassword which supports both Argon2id and bcrypt (legacy)
		valid, err := auth.VerifyPassword(input.CurrentPassword, *profile.PasswordHash)
		if err != nil || !valid {
			return ErrInvalidPassword
		}
	}

	// 3. Validate new password
	if err := s.validatePasswordStrength(input.NewPassword); err != nil {
		return err
	}

	// 4. Ensure new password is different from current
	if profile.PasswordHash != nil && *profile.PasswordHash != "" {
		sameAsOld, _ := auth.VerifyPassword(input.NewPassword, *profile.PasswordHash)
		if sameAsOld {
			return ErrPasswordSameAsOld
		}
	}

	// 5. Hash and store new password using Argon2id (via auth package)
	hashedPassword, err := auth.HashPassword(input.NewPassword)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	if err := s.profileRepo.UpdatePassword(ctx, userID, hashedPassword); err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	// 6. Send confirmation email
	if err := s.emailProvider.SendPasswordChangedEmail(ctx, email.PasswordChangedEmailInput{
		To:        profile.Email,
		UserName:  profile.Email,
		IPAddress: input.IPAddress,
		Timestamp: time.Now().Format("02/01/2006 15:04:05"),
	}); err != nil {
		// SECURITY: Don't log raw email (PII) - use userID for correlation
		s.logger.Warn("failed to send password changed notification",
			zap.String("userID", userID),
			zap.Error(err))
	}

	s.logger.Info("password changed",
		zap.String("userID", userID),
		zap.String("ip", input.IPAddress))

	return nil
}

// hashToken creates an HMAC-SHA256 hash of the token
// This ensures consistent hashing and prevents timing attacks
func (s *PasswordService) hashToken(token string) []byte {
	h := hmac.New(sha256.New, s.hmacKey)
	h.Write([]byte(token))
	return h.Sum(nil)
}

// validatePasswordStrength checks password requirements
func (s *PasswordService) validatePasswordStrength(password string) error {
	if len(password) < MinPasswordLength {
		return ErrPasswordTooWeak
	}
	// Additional rules can be added here (uppercase, lowercase, numbers, etc.)
	return nil
}

// checkRateLimit verifies rate limits haven't been exceeded
func (s *PasswordService) checkRateLimit(ctx context.Context, email, ip string) error {
	emailCount, err := s.rateLimitRepo.GetAttempts(ctx, email, "email", RateLimitWindow)
	if err != nil {
		s.logger.Warn("failed to check email rate limit", zap.Error(err))
	}

	ipCount, err := s.rateLimitRepo.GetAttempts(ctx, ip, "ip", RateLimitWindow)
	if err != nil {
		s.logger.Warn("failed to check IP rate limit", zap.Error(err))
	}

	// Allow 3 per email, 6 per IP (to account for shared IPs)
	if emailCount >= MaxResetAttempts || ipCount >= MaxResetAttempts*2 {
		// SECURITY: Don't log raw email (PII) - use masked version
		s.logger.Warn("rate limit exceeded",
			zap.String("email_masked", maskEmail(email)),
			zap.String("ip", ip),
			zap.Int("emailCount", emailCount),
			zap.Int("ipCount", ipCount))
		return ErrRateLimitExceeded
	}

	return nil
}

// VerifyPassword verifies a password against a hash (for login).
// Delegates to auth.VerifyPassword which supports both Argon2id and bcrypt (legacy).
// Deprecated: Use auth.VerifyPassword directly for new code.
func VerifyPassword(hash, password string) bool {
	valid, _ := auth.VerifyPassword(password, hash)
	return valid
}

// HashPassword creates an Argon2id hash of a password.
// Delegates to auth.HashPassword for consistent hashing across the application.
// Deprecated: Use auth.HashPassword directly for new code.
func HashPassword(password string) (string, error) {
	return auth.HashPassword(password)
}

// CompareTokensConstantTime compares two tokens in constant time
// to prevent timing attacks (CLAUDE.md: crypto/subtle.ConstantTimeCompare)
func CompareTokensConstantTime(a, b []byte) bool {
	return subtle.ConstantTimeCompare(a, b) == 1
}

// maskEmail masks an email for display (e.g., "jo***@example.com")
func maskEmail(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return "***"
	}

	local := parts[0]
	domain := parts[1]

	if len(local) <= 2 {
		return local[:1] + "***@" + domain
	}

	return local[:2] + "***@" + domain
}
