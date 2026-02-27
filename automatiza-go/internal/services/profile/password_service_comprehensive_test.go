package profile

import (
	"errors"
	"testing"
	"time"

	"automatiza-go/internal/adapters/email"
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"

	"go.uber.org/zap"
)

// ============================================================
// Tests: RequestPasswordReset - Error Paths
// ============================================================

func TestRequestPasswordReset_ProfileRepoError_SilentFail(t *testing.T) {
	t.Parallel()
	service, profileRepo, _, _, _ := setupEnhancedPasswordService(t)

	profileRepo.getProfileByEmailErr = errors.New("db connection lost")

	// Should NOT return error (silent fail for security)
	err := service.RequestPasswordReset(ctx(), RequestResetInput{
		Email:     "test@example.com",
		IPAddress: "127.0.0.1",
		UserAgent: "TestAgent",
	})
	if err != nil {
		t.Fatalf("expected silent fail, got error: %v", err)
	}
}

func TestRequestPasswordReset_TokenCreateError(t *testing.T) {
	t.Parallel()
	service, profileRepo, tokenRepo, _, _ := setupEnhancedPasswordService(t)

	profileRepo.AddProfile(&models.Profile{ID: "user-123", Email: "test@example.com"})
	tokenRepo.createErr = errors.New("token storage failed")

	err := service.RequestPasswordReset(ctx(), RequestResetInput{
		Email:     "test@example.com",
		IPAddress: "127.0.0.1",
		UserAgent: "TestAgent",
	})
	if err == nil {
		t.Fatal("expected error from token create failure")
	}
}

func TestRequestPasswordReset_EmailSendError_StillSucceeds(t *testing.T) {
	t.Parallel()
	service, profileRepo, tokenRepo, _, emailAdapter := setupEnhancedPasswordService(t)

	profileRepo.AddProfile(&models.Profile{ID: "user-123", Email: "test@example.com"})
	emailAdapter.SetFailure(errors.New("SMTP failure"))

	err := service.RequestPasswordReset(ctx(), RequestResetInput{
		Email:     "test@example.com",
		IPAddress: "127.0.0.1",
		UserAgent: "TestAgent",
	})

	// Should NOT return error - token was created, email failure is non-fatal
	if err != nil {
		t.Fatalf("expected success despite email failure, got: %v", err)
	}

	// Token should still be created
	if len(tokenRepo.tokens) != 1 {
		t.Errorf("expected 1 token created, got %d", len(tokenRepo.tokens))
	}
}

func TestRequestPasswordReset_InvalidateTokensError_StillSucceeds(t *testing.T) {
	t.Parallel()
	service, profileRepo, tokenRepo, _, _ := setupEnhancedPasswordService(t)

	profileRepo.AddProfile(&models.Profile{ID: "user-123", Email: "test@example.com"})
	tokenRepo.invalidateTokensErr = errors.New("invalidate failed")

	err := service.RequestPasswordReset(ctx(), RequestResetInput{
		Email:     "test@example.com",
		IPAddress: "127.0.0.1",
		UserAgent: "TestAgent",
	})

	// Should NOT fail - invalidation failure is non-fatal
	if err != nil {
		t.Fatalf("expected success despite invalidation failure, got: %v", err)
	}
}

func TestRequestPasswordReset_IPRateLimitExceeded(t *testing.T) {
	t.Parallel()
	service, profileRepo, _, rateLimitRepo, _ := setupEnhancedPasswordService(t)

	profileRepo.AddProfile(&models.Profile{ID: "user-123", Email: "test@example.com"})
	// IP rate limit is 2x MaxResetAttempts
	rateLimitRepo.SetAttempts("127.0.0.1", "ip", MaxResetAttempts*2)

	err := service.RequestPasswordReset(ctx(), RequestResetInput{
		Email:     "test@example.com",
		IPAddress: "127.0.0.1",
		UserAgent: "TestAgent",
	})

	if !errors.Is(err, ErrRateLimitExceeded) {
		t.Errorf("expected ErrRateLimitExceeded for IP, got: %v", err)
	}
}

func TestRequestPasswordReset_RateLimitRepoError_StillProceeds(t *testing.T) {
	t.Parallel()
	service, profileRepo, _, rateLimitRepo, _ := setupEnhancedPasswordService(t)

	profileRepo.AddProfile(&models.Profile{ID: "user-123", Email: "test@example.com"})
	rateLimitRepo.getAttemptsErr = errors.New("redis down")

	// Should proceed despite rate limit check failure (logs warning, returns 0 attempts)
	err := service.RequestPasswordReset(ctx(), RequestResetInput{
		Email:     "test@example.com",
		IPAddress: "127.0.0.1",
		UserAgent: "TestAgent",
	})

	if err != nil {
		t.Fatalf("expected success despite rate limit check error, got: %v", err)
	}
}

func TestRequestPasswordReset_IncrementError_StillSucceeds(t *testing.T) {
	t.Parallel()
	service, profileRepo, _, rateLimitRepo, _ := setupEnhancedPasswordService(t)

	profileRepo.AddProfile(&models.Profile{ID: "user-123", Email: "test@example.com"})
	rateLimitRepo.incrementErr = errors.New("increment failed")

	err := service.RequestPasswordReset(ctx(), RequestResetInput{
		Email:     "test@example.com",
		IPAddress: "127.0.0.1",
		UserAgent: "TestAgent",
	})

	// Increment failure is best-effort, should not fail the request
	if err != nil {
		t.Fatalf("expected success despite increment error, got: %v", err)
	}
}

// ============================================================
// Tests: ValidateResetToken - Additional Paths
// ============================================================

func TestValidateResetToken_ValidToken(t *testing.T) {
	t.Parallel()
	service, _, tokenRepo, _, _ := setupEnhancedPasswordService(t)

	token := "valid-test-token"
	tokenHash := service.hashToken(token)
	tokenRepo.tokens[tokenHash] = &repository.PasswordResetToken{
		UserID:    "user-123",
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(1 * time.Hour),
		Email:     "test@example.com",
	}

	result, err := service.ValidateResetToken(ctx(), token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Valid {
		t.Error("expected token to be valid")
	}
	if result.Email != "te***@example.com" {
		t.Errorf("expected masked email 'te***@example.com', got %s", result.Email)
	}
	if result.Reason != "" {
		t.Errorf("expected empty reason for valid token, got %s", result.Reason)
	}
}

func TestValidateResetToken_TokenNotInRepo(t *testing.T) {
	t.Parallel()
	service, _, _, _, _ := setupEnhancedPasswordService(t)

	result, err := service.ValidateResetToken(ctx(), "non-existent-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Valid {
		t.Error("expected invalid result for non-existent token")
	}
	if result.Reason != "invalid_token" {
		t.Errorf("expected reason 'invalid_token', got %s", result.Reason)
	}
}

func TestValidateResetToken_JustExpired(t *testing.T) {
	t.Parallel()
	service, _, tokenRepo, _, _ := setupEnhancedPasswordService(t)

	token := "just-expired-token"
	tokenHash := service.hashToken(token)
	tokenRepo.tokens[tokenHash] = &repository.PasswordResetToken{
		UserID:    "user-123",
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(-1 * time.Second), // Just expired
		Email:     "test@example.com",
	}

	result, err := service.ValidateResetToken(ctx(), token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Valid {
		t.Error("expected invalid for just-expired token")
	}
	if result.Reason != "token_expired" {
		t.Errorf("expected reason 'token_expired', got %s", result.Reason)
	}
}

func TestValidateResetToken_ShortLocalEmail(t *testing.T) {
	t.Parallel()
	service, _, tokenRepo, _, _ := setupEnhancedPasswordService(t)

	token := "test-token-short-email"
	tokenHash := service.hashToken(token)
	tokenRepo.tokens[tokenHash] = &repository.PasswordResetToken{
		UserID:    "user-123",
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(1 * time.Hour),
		Email:     "a@test.com",
	}

	result, err := service.ValidateResetToken(ctx(), token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Valid {
		t.Error("expected valid token")
	}
	if result.Email != "a***@test.com" {
		t.Errorf("expected masked email 'a***@test.com', got %s", result.Email)
	}
}

// ============================================================
// Tests: ResetPassword - Error Paths
// ============================================================

func TestResetPassword_ExpiredToken(t *testing.T) {
	t.Parallel()
	service, _, tokenRepo, _, _ := setupEnhancedPasswordService(t)

	token := "expired-reset-token"
	tokenHash := service.hashToken(token)
	tokenRepo.tokens[tokenHash] = &repository.PasswordResetToken{
		UserID:    "user-123",
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(-1 * time.Hour),
		Email:     "test@example.com",
	}

	err := service.ResetPassword(ctx(), ResetPasswordInput{
		Token:       token,
		NewPassword: "newPassword123",
		IPAddress:   "127.0.0.1",
	})

	if !errors.Is(err, ErrTokenExpired) {
		t.Errorf("expected ErrTokenExpired, got: %v", err)
	}
}

func TestResetPassword_UsedToken(t *testing.T) {
	t.Parallel()
	service, _, tokenRepo, _, _ := setupEnhancedPasswordService(t)

	token := "used-reset-token"
	tokenHash := service.hashToken(token)

	usedAt := time.Now().Add(-30 * time.Minute)
	tokenRepo.tokens[tokenHash] = &repository.PasswordResetToken{
		UserID:    "user-123",
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(1 * time.Hour),
		UsedAt:    &usedAt,
		Email:     "test@example.com",
	}

	err := service.ResetPassword(ctx(), ResetPasswordInput{
		Token:       token,
		NewPassword: "newPassword123",
		IPAddress:   "127.0.0.1",
	})

	if !errors.Is(err, ErrTokenUsed) {
		t.Errorf("expected ErrTokenUsed, got: %v", err)
	}
}

func TestResetPassword_InvalidToken(t *testing.T) {
	t.Parallel()
	service, _, _, _, _ := setupEnhancedPasswordService(t)

	err := service.ResetPassword(ctx(), ResetPasswordInput{
		Token:       "completely-invalid-token",
		NewPassword: "newPassword123",
		IPAddress:   "127.0.0.1",
	})

	if !errors.Is(err, ErrInvalidToken) {
		t.Errorf("expected ErrInvalidToken, got: %v", err)
	}
}

func TestResetPassword_UpdatePasswordError(t *testing.T) {
	t.Parallel()
	service, profileRepo, tokenRepo, _, _ := setupEnhancedPasswordService(t)

	profileRepo.AddProfile(&models.Profile{ID: "user-123", Email: "test@example.com"})

	token := "valid-token-for-update-err"
	tokenHash := service.hashToken(token)
	tokenRepo.tokens[tokenHash] = &repository.PasswordResetToken{
		UserID:    "user-123",
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(1 * time.Hour),
		Email:     "test@example.com",
	}

	profileRepo.updatePasswordErr = errors.New("password update failed")

	err := service.ResetPassword(ctx(), ResetPasswordInput{
		Token:       token,
		NewPassword: "newPassword123",
		IPAddress:   "127.0.0.1",
	})

	if err == nil {
		t.Fatal("expected error from password update failure")
	}
	if !errors.Is(err, profileRepo.updatePasswordErr) {
		t.Errorf("expected wrapped password update error, got: %v", err)
	}
}

func TestResetPassword_MarkUsedError_StillSucceeds(t *testing.T) {
	t.Parallel()
	service, profileRepo, tokenRepo, _, _ := setupEnhancedPasswordService(t)

	profileRepo.AddProfile(&models.Profile{ID: "user-123", Email: "test@example.com"})

	token := "valid-token-mark-used-err"
	tokenHash := service.hashToken(token)
	tokenRepo.tokens[tokenHash] = &repository.PasswordResetToken{
		UserID:    "user-123",
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(1 * time.Hour),
		Email:     "test@example.com",
	}

	tokenRepo.markUsedErr = errors.New("mark used failed")

	err := service.ResetPassword(ctx(), ResetPasswordInput{
		Token:       token,
		NewPassword: "newPassword123",
		IPAddress:   "127.0.0.1",
	})

	// Mark used failure is non-fatal
	if err != nil {
		t.Fatalf("expected success despite mark used failure, got: %v", err)
	}
}

func TestResetPassword_GetProfileErrorAfterReset_StillSucceeds(t *testing.T) {
	t.Parallel()

	profileRepo := newEnhancedMockProfileRepo()
	tokenRepo := newEnhancedMockTokenRepo()
	rateLimitRepo := newEnhancedMockRateLimitRepo()
	emailAdapter := email.NewFakeEmailAdapter()

	hmacKey := make([]byte, 32)
	copy(hmacKey, "test-key-for-get-profile-error!!")
	logger := zap.NewNop()

	svc := NewPasswordService(profileRepo, tokenRepo, rateLimitRepo, emailAdapter, PasswordServiceConfig{HMACKey: hmacKey, FrontendURL: "https://test.com"}, logger)

	// Don't add profile by ID - GetProfile after reset will return nil
	// But add by email for the token record
	token := "valid-token-no-profile"
	tokenHash := svc.hashToken(token)
	tokenRepo.tokens[tokenHash] = &repository.PasswordResetToken{
		UserID:    "user-no-profile",
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(1 * time.Hour),
		Email:     "test@example.com",
	}

	err := svc.ResetPassword(ctx(), ResetPasswordInput{
		Token:       token,
		NewPassword: "newPassword123",
		IPAddress:   "127.0.0.1",
	})

	// GetProfile failure after password update is non-fatal
	if err != nil {
		t.Fatalf("expected success despite GetProfile failure, got: %v", err)
	}
}

func TestResetPassword_EmailSendError_StillSucceeds(t *testing.T) {
	t.Parallel()
	service, profileRepo, tokenRepo, _, emailAdapter := setupEnhancedPasswordService(t)

	profileRepo.AddProfile(&models.Profile{ID: "user-123", Email: "test@example.com"})

	token := "valid-token-email-err"
	tokenHash := service.hashToken(token)
	tokenRepo.tokens[tokenHash] = &repository.PasswordResetToken{
		UserID:    "user-123",
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(1 * time.Hour),
		Email:     "test@example.com",
	}

	emailAdapter.SetFailure(errors.New("SMTP failure"))

	err := service.ResetPassword(ctx(), ResetPasswordInput{
		Token:       token,
		NewPassword: "newPassword123",
		IPAddress:   "127.0.0.1",
	})

	// Email failure is non-fatal
	if err != nil {
		t.Fatalf("expected success despite email failure, got: %v", err)
	}
}

func TestResetPassword_PasswordExactlyMinLength(t *testing.T) {
	t.Parallel()
	service, profileRepo, tokenRepo, _, _ := setupEnhancedPasswordService(t)

	profileRepo.AddProfile(&models.Profile{ID: "user-123", Email: "test@example.com"})

	token := "valid-token-min-len"
	tokenHash := service.hashToken(token)
	tokenRepo.tokens[tokenHash] = &repository.PasswordResetToken{
		UserID:    "user-123",
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(1 * time.Hour),
		Email:     "test@example.com",
	}

	// Exactly 8 chars
	err := service.ResetPassword(ctx(), ResetPasswordInput{
		Token:       token,
		NewPassword: "12345678",
		IPAddress:   "127.0.0.1",
	})

	if err != nil {
		t.Fatalf("expected success for password with exactly minimum length, got: %v", err)
	}
}

func TestResetPassword_PasswordOneCharBelowMinLength(t *testing.T) {
	t.Parallel()
	service, _, tokenRepo, _, _ := setupEnhancedPasswordService(t)

	token := "valid-token-below-min"
	tokenHash := service.hashToken(token)
	tokenRepo.tokens[tokenHash] = &repository.PasswordResetToken{
		UserID:    "user-123",
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(1 * time.Hour),
		Email:     "test@example.com",
	}

	// 7 chars
	err := service.ResetPassword(ctx(), ResetPasswordInput{
		Token:       token,
		NewPassword: "1234567",
		IPAddress:   "127.0.0.1",
	})

	if !errors.Is(err, ErrPasswordTooWeak) {
		t.Errorf("expected ErrPasswordTooWeak, got: %v", err)
	}
}

// ============================================================
// Tests: ChangePassword - Error Paths
// ============================================================

func TestChangePassword_NoExistingPassword(t *testing.T) {
	t.Parallel()
	service, profileRepo, _, _, _ := setupEnhancedPasswordService(t)

	// Profile without password hash (e.g., OAuth user setting password for first time)
	profileRepo.AddProfile(&models.Profile{
		ID:    "user-oauth",
		Email: "oauth@example.com",
	})

	err := service.ChangePassword(ctx(), "user-oauth", ChangePasswordInput{
		CurrentPassword: "", // No current password
		NewPassword:     "newPassword123",
		IPAddress:       "127.0.0.1",
	})

	if err != nil {
		t.Fatalf("expected success for user with no existing password, got: %v", err)
	}
}

func TestChangePassword_EmptyPasswordHash(t *testing.T) {
	t.Parallel()
	service, profileRepo, _, _, _ := setupEnhancedPasswordService(t)

	emptyHash := ""
	profileRepo.AddProfile(&models.Profile{
		ID:           "user-empty",
		Email:        "empty@example.com",
		PasswordHash: &emptyHash,
	})

	err := service.ChangePassword(ctx(), "user-empty", ChangePasswordInput{
		CurrentPassword: "",
		NewPassword:     "newPassword123",
		IPAddress:       "127.0.0.1",
	})

	if err != nil {
		t.Fatalf("expected success for user with empty password hash, got: %v", err)
	}
}

func TestChangePassword_WeakNewPassword(t *testing.T) {
	t.Parallel()
	service, profileRepo, _, _, _ := setupEnhancedPasswordService(t)

	existingHash, _ := HashPassword("oldPassword123")
	profileRepo.AddProfile(&models.Profile{
		ID:           "user-123",
		Email:        "test@example.com",
		PasswordHash: &existingHash,
	})

	err := service.ChangePassword(ctx(), "user-123", ChangePasswordInput{
		CurrentPassword: "oldPassword123",
		NewPassword:     "short",
		IPAddress:       "127.0.0.1",
	})

	if !errors.Is(err, ErrPasswordTooWeak) {
		t.Errorf("expected ErrPasswordTooWeak, got: %v", err)
	}
}

func TestChangePassword_ProfileRepoError(t *testing.T) {
	t.Parallel()
	service, profileRepo, _, _, _ := setupEnhancedPasswordService(t)

	profileRepo.getProfileErr = errors.New("db connection lost")

	err := service.ChangePassword(ctx(), "user-123", ChangePasswordInput{
		CurrentPassword: "old",
		NewPassword:     "newPassword123",
		IPAddress:       "127.0.0.1",
	})

	if err == nil {
		t.Fatal("expected error from profile repo failure")
	}
}

func TestChangePassword_UpdatePasswordRepoError(t *testing.T) {
	t.Parallel()
	service, profileRepo, _, _, _ := setupEnhancedPasswordService(t)

	existingHash, _ := HashPassword("oldPassword123")
	profileRepo.AddProfile(&models.Profile{
		ID:           "user-123",
		Email:        "test@example.com",
		PasswordHash: &existingHash,
	})
	profileRepo.updatePasswordErr = errors.New("password update failed")

	err := service.ChangePassword(ctx(), "user-123", ChangePasswordInput{
		CurrentPassword: "oldPassword123",
		NewPassword:     "differentPassword456",
		IPAddress:       "127.0.0.1",
	})

	if err == nil {
		t.Fatal("expected error from password update failure")
	}
	if !errors.Is(err, profileRepo.updatePasswordErr) {
		t.Errorf("expected wrapped update error, got: %v", err)
	}
}

func TestChangePassword_EmailSendError_StillSucceeds(t *testing.T) {
	t.Parallel()
	service, profileRepo, _, _, emailAdapter := setupEnhancedPasswordService(t)

	existingHash, _ := HashPassword("oldPassword123")
	profileRepo.AddProfile(&models.Profile{
		ID:           "user-123",
		Email:        "test@example.com",
		PasswordHash: &existingHash,
	})

	emailAdapter.SetFailure(errors.New("SMTP failure"))

	err := service.ChangePassword(ctx(), "user-123", ChangePasswordInput{
		CurrentPassword: "oldPassword123",
		NewPassword:     "newPassword456",
		IPAddress:       "127.0.0.1",
	})

	// Email failure is non-fatal
	if err != nil {
		t.Fatalf("expected success despite email failure, got: %v", err)
	}
}

func TestChangePassword_ConfirmationEmailSent(t *testing.T) {
	t.Parallel()
	service, profileRepo, _, _, emailAdapter := setupEnhancedPasswordService(t)

	existingHash, _ := HashPassword("oldPassword123")
	profileRepo.AddProfile(&models.Profile{
		ID:           "user-123",
		Email:        "test@example.com",
		PasswordHash: &existingHash,
	})

	err := service.ChangePassword(ctx(), "user-123", ChangePasswordInput{
		CurrentPassword: "oldPassword123",
		NewPassword:     "newPassword456",
		IPAddress:       "127.0.0.1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	emails := emailAdapter.GetSentEmails()
	if len(emails) != 1 {
		t.Fatalf("expected 1 email, got %d", len(emails))
	}
	if emails[0].Type != "password_changed" {
		t.Errorf("expected type 'password_changed', got %s", emails[0].Type)
	}
	if emails[0].To != "test@example.com" {
		t.Errorf("expected to 'test@example.com', got %s", emails[0].To)
	}
}

// ============================================================
// Tests: hashToken
// ============================================================

func TestHashToken_Deterministic(t *testing.T) {
	t.Parallel()
	service, _, _, _, _ := setupEnhancedPasswordService(t)

	hash1 := service.hashToken("same-token")
	hash2 := service.hashToken("same-token")

	if hash1 != hash2 {
		t.Error("same token should produce same hash")
	}
}

func TestHashToken_DifferentTokens_DifferentHashes(t *testing.T) {
	t.Parallel()
	service, _, _, _, _ := setupEnhancedPasswordService(t)

	hash1 := service.hashToken("token-a")
	hash2 := service.hashToken("token-b")

	if hash1 == hash2 {
		t.Error("different tokens should produce different hashes")
	}
}

func TestHashToken_EmptyToken(t *testing.T) {
	t.Parallel()
	service, _, _, _, _ := setupEnhancedPasswordService(t)

	hash := service.hashToken("")
	if len(hash) == 0 {
		t.Error("hash of empty token should not be empty")
	}

	// SHA-256 HMAC produces 32 bytes = 64 hex chars
	if len(hash) != 64 {
		t.Errorf("expected 64-char hex hash, got %d chars", len(hash))
	}
}

// ============================================================
// Tests: validatePasswordStrength
// ============================================================

func TestValidatePasswordStrength_BoundaryCases(t *testing.T) {
	t.Parallel()
	service, _, _, _, _ := setupEnhancedPasswordService(t)

	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{"empty", "", true},
		{"one_char", "a", true},
		{"seven_chars", "1234567", true},
		{"eight_chars", "12345678", false},
		{"nine_chars", "123456789", false},
		{"very_long", "abcdefghijklmnopqrstuvwxyz1234567890", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := service.validatePasswordStrength(tc.password)
			if tc.wantErr && err == nil {
				t.Error("expected error for weak password")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// ============================================================
// Tests: checkRateLimit
// ============================================================

func TestCheckRateLimit_EmailExceeded(t *testing.T) {
	t.Parallel()
	service, _, _, rateLimitRepo, _ := setupEnhancedPasswordService(t)

	rateLimitRepo.SetAttempts("test@example.com", "email", MaxResetAttempts)

	err := service.checkRateLimit(ctx(), "test@example.com", "127.0.0.1")
	if !errors.Is(err, ErrRateLimitExceeded) {
		t.Errorf("expected ErrRateLimitExceeded, got: %v", err)
	}
}

func TestCheckRateLimit_IPExceeded(t *testing.T) {
	t.Parallel()
	service, _, _, rateLimitRepo, _ := setupEnhancedPasswordService(t)

	rateLimitRepo.SetAttempts("127.0.0.1", "ip", MaxResetAttempts*2)

	err := service.checkRateLimit(ctx(), "test@example.com", "127.0.0.1")
	if !errors.Is(err, ErrRateLimitExceeded) {
		t.Errorf("expected ErrRateLimitExceeded, got: %v", err)
	}
}

func TestCheckRateLimit_BelowThreshold(t *testing.T) {
	t.Parallel()
	service, _, _, rateLimitRepo, _ := setupEnhancedPasswordService(t)

	rateLimitRepo.SetAttempts("test@example.com", "email", MaxResetAttempts-1)
	rateLimitRepo.SetAttempts("127.0.0.1", "ip", MaxResetAttempts*2-1)

	err := service.checkRateLimit(ctx(), "test@example.com", "127.0.0.1")
	if err != nil {
		t.Fatalf("expected no error below threshold, got: %v", err)
	}
}

func TestCheckRateLimit_RepoErrors_StillProceed(t *testing.T) {
	t.Parallel()
	service, _, _, rateLimitRepo, _ := setupEnhancedPasswordService(t)

	rateLimitRepo.getAttemptsErr = errors.New("redis error")

	// With repo error, GetAttempts returns 0, so rate limit should not be exceeded
	err := service.checkRateLimit(ctx(), "test@example.com", "127.0.0.1")
	if err != nil {
		t.Fatalf("expected no error when rate limit repo fails, got: %v", err)
	}
}

// ============================================================
// Tests: maskEmail - Additional Cases
// ============================================================

func TestMaskEmail_AdditionalCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"long_local", "johnathan@example.com", "jo***@example.com"},
		{"three_char_local", "abc@test.com", "ab***@test.com"},
		{"two_char_local", "ab@test.com", "a***@test.com"},
		{"single_char_local", "a@test.com", "a***@test.com"},
		{"no_at_sign", "invalid-email", "***"},
		{"empty_string", "", "***"},
		{"multiple_at_signs", "a@b@c.com", "***"},
		{"subdomain", "john@sub.example.com", "jo***@sub.example.com"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := maskEmail(tc.input)
			if result != tc.expected {
				t.Errorf("maskEmail(%q) = %q, expected %q", tc.input, result, tc.expected)
			}
		})
	}
}

// ============================================================
// Tests: VerifyPassword / HashPassword
// ============================================================

func TestHashPassword_Roundtrip(t *testing.T) {
	t.Parallel()

	passwords := []string{
		"simple123",
		"Complex!@#$%^&*()_+",
		"12345678",
		"a very long password with spaces and special chars !@#",
	}

	for _, pwd := range passwords {
		t.Run(pwd, func(t *testing.T) {
			t.Parallel()
			hash, err := HashPassword(pwd)
			if err != nil {
				t.Fatalf("HashPassword failed: %v", err)
			}
			if hash == "" {
				t.Fatal("expected non-empty hash")
			}
			if !VerifyPassword(hash, pwd) {
				t.Error("password should verify against its own hash")
			}
		})
	}
}

func TestHashPassword_DifferentHashesForSamePassword(t *testing.T) {
	t.Parallel()

	hash1, err := HashPassword("testPassword123")
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	hash2, err := HashPassword("testPassword123")
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	// Argon2id uses random salt, so hashes should differ
	if hash1 == hash2 {
		t.Error("two hashes of same password should differ (different salts)")
	}
}

func TestVerifyPassword_WrongPassword(t *testing.T) {
	t.Parallel()

	hash, err := HashPassword("correctPassword")
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	if VerifyPassword(hash, "wrongPassword") {
		t.Error("wrong password should not verify")
	}
}

func TestVerifyPassword_EmptyHash(t *testing.T) {
	t.Parallel()

	result := VerifyPassword("", "anyPassword")
	if result {
		t.Error("empty hash should not verify any password")
	}
}

// ============================================================
// Tests: CompareTokensConstantTime - Additional Cases
// ============================================================

func TestCompareTokensConstantTime_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		a      []byte
		b      []byte
		expect bool
	}{
		{"both_empty", []byte{}, []byte{}, true},
		{"one_empty", []byte("abc"), []byte{}, false},
		{"different_lengths", []byte("abc"), []byte("abcd"), false},
		{"same_single_byte", []byte{0x01}, []byte{0x01}, true},
		{"different_single_byte", []byte{0x01}, []byte{0x02}, false},
		{"nil_nil", nil, nil, true},
		{"nil_empty", nil, []byte{}, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := CompareTokensConstantTime(tc.a, tc.b)
			if result != tc.expect {
				t.Errorf("CompareTokensConstantTime(%v, %v) = %v, expected %v", tc.a, tc.b, result, tc.expect)
			}
		})
	}
}

// ============================================================
// Tests: ResetPassword - GetByHash second call failure (line 219)
// ============================================================

func TestResetPassword_SecondGetByHashFails(t *testing.T) {
	t.Parallel()
	service, profileRepo, tokenRepo, _, _ := setupEnhancedPasswordService(t)

	profileRepo.AddProfile(&models.Profile{ID: "user-123", Email: "test@example.com"})

	token := "valid-token-second-hash-fail"
	tokenHash := service.hashToken(token)
	tokenRepo.tokens[tokenHash] = &repository.PasswordResetToken{
		UserID:    "user-123",
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(1 * time.Hour),
		Email:     "test@example.com",
	}

	// First GetByHash (in ValidateResetToken) succeeds, second (in ResetPassword) fails
	tokenRepo.getByHashFailAfterCall = 2
	tokenRepo.getByHashFailAfterErr = errors.New("token lookup race condition")

	err := service.ResetPassword(ctx(), ResetPasswordInput{
		Token:       token,
		NewPassword: "newPassword123",
		IPAddress:   "127.0.0.1",
	})

	if !errors.Is(err, ErrInvalidToken) {
		t.Errorf("expected ErrInvalidToken when second GetByHash fails, got: %v", err)
	}
}

// ============================================================
// Tests: ResetPassword - GetProfile failure after password update (line 243)
// ============================================================

func TestResetPassword_GetProfileFailsAfterPasswordUpdate(t *testing.T) {
	t.Parallel()
	service, profileRepo, tokenRepo, _, _ := setupEnhancedPasswordService(t)

	// Add profile for token lookup but set getProfileErr so GetProfile fails
	profileRepo.AddProfile(&models.Profile{ID: "user-123", Email: "test@example.com"})

	token := "valid-token-getprofile-fail"
	tokenHash := service.hashToken(token)
	tokenRepo.tokens[tokenHash] = &repository.PasswordResetToken{
		UserID:    "user-123",
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(1 * time.Hour),
		Email:     "test@example.com",
	}

	// GetProfile will fail - this only affects the confirmation email part (line 242-249)
	profileRepo.getProfileErr = errors.New("profile lookup failed after update")

	err := service.ResetPassword(ctx(), ResetPasswordInput{
		Token:       token,
		NewPassword: "newPassword123",
		IPAddress:   "127.0.0.1",
	})

	// GetProfile failure after password update is non-fatal (observability.LogAndCapture)
	if err != nil {
		t.Fatalf("expected success despite GetProfile failure, got: %v", err)
	}
}

// ============================================================
// Tests: ChangePassword - HashPassword failure (line 306)
// This tests the code path where auth.HashPassword fails
// In practice, this requires the crypto/rand to fail which is hard to trigger
// We verify the code path up to that point is correct
// ============================================================

func TestChangePassword_PasswordHashVerification(t *testing.T) {
	t.Parallel()
	service, profileRepo, _, _, _ := setupEnhancedPasswordService(t)

	existingHash, _ := HashPassword("oldPassword123")
	profileRepo.AddProfile(&models.Profile{
		ID:           "user-123",
		Email:        "test@example.com",
		PasswordHash: &existingHash,
	})

	err := service.ChangePassword(ctx(), "user-123", ChangePasswordInput{
		CurrentPassword: "oldPassword123",
		NewPassword:     "newSecurePassword789",
		IPAddress:       "127.0.0.1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the new password was actually hashed and stored
	profile := profileRepo.profiles["user-123"]
	if profile.PasswordHash == nil {
		t.Fatal("expected password hash to be set")
	}
	if !VerifyPassword(*profile.PasswordHash, "newSecurePassword789") {
		t.Error("new password should verify against stored hash")
	}
}

// ============================================================
// Tests: NewPasswordService constructor
// ============================================================

func TestNewPasswordService_SetsAllFields(t *testing.T) {
	t.Parallel()
	service, _, _, _, _ := setupEnhancedPasswordService(t)

	if service == nil {
		t.Fatal("expected non-nil service")
	}
	if service.profileRepo == nil {
		t.Error("expected non-nil profileRepo")
	}
	if service.tokenRepo == nil {
		t.Error("expected non-nil tokenRepo")
	}
	if service.rateLimitRepo == nil {
		t.Error("expected non-nil rateLimitRepo")
	}
	if service.emailProvider == nil {
		t.Error("expected non-nil emailProvider")
	}
	if len(service.hmacKey) == 0 {
		t.Error("expected non-empty HMAC key")
	}
	if service.frontendURL != "https://app.test.com" {
		t.Errorf("expected frontendURL 'https://app.test.com', got %s", service.frontendURL)
	}
	if service.logger == nil {
		t.Error("expected non-nil logger")
	}
}

// ============================================================
// Tests: NewProfileService constructor
// ============================================================

func TestNewProfileService_SetsAllFields(t *testing.T) {
	t.Parallel()
	service, _, _, _, _ := setupEnhancedProfileService(t)

	if service == nil {
		t.Fatal("expected non-nil service")
	}
	if service.profileRepo == nil {
		t.Error("expected non-nil profileRepo")
	}
	if service.automationRepo == nil {
		t.Error("expected non-nil automationRepo")
	}
	if service.instanceRepo == nil {
		t.Error("expected non-nil instanceRepo")
	}
	if service.userRoleRepo == nil {
		t.Error("expected non-nil userRoleRepo")
	}
	if service.logger == nil {
		t.Error("expected non-nil logger")
	}
}
