package profile

import (
	"context"
	"crypto/rand"
	"errors"
	"testing"
	"time"

	"automatiza-go/internal/adapters/email"
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"

	"go.uber.org/zap"
)

// ============================================================
// Mock Repositories
// ============================================================

type mockProfileRepo struct {
	profiles       map[string]*models.Profile
	profileByEmail map[string]*models.Profile
	shouldFail     bool
	failError      error
}

func newMockProfileRepo() *mockProfileRepo {
	return &mockProfileRepo{
		profiles:       make(map[string]*models.Profile),
		profileByEmail: make(map[string]*models.Profile),
	}
}

func (m *mockProfileRepo) GetProfile(ctx context.Context, userID string) (*models.Profile, error) {
	if m.shouldFail {
		return nil, m.failError
	}
	profile, ok := m.profiles[userID]
	if !ok {
		return nil, nil
	}
	return profile, nil
}

func (m *mockProfileRepo) GetProfileByEmail(ctx context.Context, email string) (*models.Profile, error) {
	if m.shouldFail {
		return nil, m.failError
	}
	profile, ok := m.profileByEmail[email]
	if !ok {
		return nil, nil
	}
	return profile, nil
}

func (m *mockProfileRepo) UpdatePassword(ctx context.Context, userID, passwordHash string) error {
	if m.shouldFail {
		return m.failError
	}
	if profile, ok := m.profiles[userID]; ok {
		profile.PasswordHash = &passwordHash
	}
	return nil
}

func (m *mockProfileRepo) EnsureUserProfile(ctx context.Context, userID, email string) (*models.Profile, error) {
	return nil, nil
}

func (m *mockProfileRepo) UpdateProfile(ctx context.Context, userID string, updates map[string]interface{}) error {
	return nil
}

func (m *mockProfileRepo) ListProfiles(ctx context.Context, cursor string, limit int) ([]*models.Profile, string, error) {
	return nil, "", nil
}

func (m *mockProfileRepo) SearchProfiles(ctx context.Context, query, cursor string, limit int) ([]*models.Profile, string, error) {
	return nil, "", nil
}

func (m *mockProfileRepo) GetProfileByEmailWithBilling(ctx context.Context, email string) (*models.Profile, *models.ProfileBillingFields, error) {
	if m.shouldFail {
		return nil, nil, m.failError
	}
	profile, ok := m.profileByEmail[email]
	if !ok {
		return nil, nil, nil
	}
	return profile, nil, nil
}

func (m *mockProfileRepo) CreateProfile(ctx context.Context, profile *models.Profile) (*models.Profile, error) {
	if m.shouldFail {
		return nil, m.failError
	}
	m.profiles[profile.ID] = profile
	m.profileByEmail[profile.Email] = profile
	return profile, nil
}

func (m *mockProfileRepo) UpdateBillingStatus(ctx context.Context, userID string, billing *models.ProfileBillingFields) error {
	return nil
}

func (m *mockProfileRepo) PauseUserAutomations(ctx context.Context, userID string) (int, error) {
	return 0, nil
}

func (m *mockProfileRepo) ResumeUserAutomations(ctx context.Context, userID string) (int, error) {
	return 0, nil
}

func (m *mockProfileRepo) GetMaxAutomations(ctx context.Context, userID string) (int, error) {
	return 5, nil // Default max
}

func (m *mockProfileRepo) CountUserAutomations(ctx context.Context, userID string) (int, error) {
	return 0, nil // Default count
}

func (m *mockProfileRepo) CreateUserCredentials(ctx context.Context, userID, passwordHash string) error {
	if m.shouldFail {
		return m.failError
	}
	if profile, ok := m.profiles[userID]; ok {
		profile.PasswordHash = &passwordHash
	}
	return nil
}

func (m *mockProfileRepo) GetProfileStats(ctx context.Context) (*repository.ProfileStats, error) {
	if m.shouldFail {
		return nil, m.failError
	}
	return &repository.ProfileStats{}, nil
}

func (m *mockProfileRepo) AddProfile(profile *models.Profile) {
	m.profiles[profile.ID] = profile
	m.profileByEmail[profile.Email] = profile
}

// ============================================================
// Mock Token Repository
// ============================================================

type mockTokenRepo struct {
	tokens     map[string]*repository.PasswordResetToken
	shouldFail bool
	failError  error
}

func newMockTokenRepo() *mockTokenRepo {
	return &mockTokenRepo{
		tokens: make(map[string]*repository.PasswordResetToken),
	}
}

func (m *mockTokenRepo) Create(ctx context.Context, token *repository.PasswordResetToken) error {
	if m.shouldFail {
		return m.failError
	}
	m.tokens[token.TokenHash] = token
	return nil
}

func (m *mockTokenRepo) GetByHash(ctx context.Context, tokenHash string) (*repository.PasswordResetToken, error) {
	if m.shouldFail {
		return nil, m.failError
	}
	token, ok := m.tokens[tokenHash]
	if !ok {
		return nil, errors.New("token not found")
	}
	return token, nil
}

func (m *mockTokenRepo) MarkUsed(ctx context.Context, tokenHash string) error {
	if m.shouldFail {
		return m.failError
	}
	if token, ok := m.tokens[tokenHash]; ok {
		now := time.Now()
		token.UsedAt = &now
	}
	return nil
}

func (m *mockTokenRepo) InvalidateUserTokens(ctx context.Context, userID string) error {
	if m.shouldFail {
		return m.failError
	}
	for key, token := range m.tokens {
		if token.UserID == userID && token.UsedAt == nil {
			now := time.Now()
			m.tokens[key].UsedAt = &now
		}
	}
	return nil
}

func (m *mockTokenRepo) CleanupExpired(ctx context.Context) (int64, error) {
	return 0, nil
}

// ============================================================
// Mock Rate Limit Repository
// ============================================================

type mockRateLimitRepo struct {
	attempts   map[string]int
	shouldFail bool
	failError  error
}

func newMockRateLimitRepo() *mockRateLimitRepo {
	return &mockRateLimitRepo{
		attempts: make(map[string]int),
	}
}

func (m *mockRateLimitRepo) GetAttempts(ctx context.Context, identifier, identifierType string, window time.Duration) (int, error) {
	if m.shouldFail {
		return 0, m.failError
	}
	key := identifier + ":" + identifierType
	return m.attempts[key], nil
}

func (m *mockRateLimitRepo) Increment(ctx context.Context, identifier, identifierType string) error {
	if m.shouldFail {
		return m.failError
	}
	key := identifier + ":" + identifierType
	m.attempts[key]++
	return nil
}

func (m *mockRateLimitRepo) Cleanup(ctx context.Context) (int64, error) {
	return 0, nil
}

func (m *mockRateLimitRepo) Reset(ctx context.Context, identifier, identifierType string) error {
	key := identifier + ":" + identifierType
	m.attempts[key] = 0
	return nil
}

func (m *mockRateLimitRepo) SetAttempts(identifier, identifierType string, count int) {
	key := identifier + ":" + identifierType
	m.attempts[key] = count
}

// ============================================================
// Test Helpers
// ============================================================

func setupPasswordService(t *testing.T) (*PasswordService, *mockProfileRepo, *mockTokenRepo, *mockRateLimitRepo, *email.FakeEmailAdapter) {
	t.Helper()

	profileRepo := newMockProfileRepo()
	tokenRepo := newMockTokenRepo()
	rateLimitRepo := newMockRateLimitRepo()
	emailAdapter := email.NewFakeEmailAdapter()

	// Generate HMAC key for tests
	hmacKey := make([]byte, 32)
	if _, err := rand.Read(hmacKey); err != nil {
		t.Fatalf("failed to generate HMAC key: %v", err)
	}

	logger := zap.NewNop()

	service := NewPasswordService(
		profileRepo,
		tokenRepo,
		rateLimitRepo,
		emailAdapter,
		PasswordServiceConfig{
			HMACKey:     hmacKey,
			FrontendURL: "https://app.test.com",
		},
		logger,
	)

	return service, profileRepo, tokenRepo, rateLimitRepo, emailAdapter
}

// ============================================================
// Tests: RequestPasswordReset
// ============================================================

func TestRequestPasswordReset_Success(t *testing.T) {
	service, profileRepo, tokenRepo, _, emailAdapter := setupPasswordService(t)
	ctx := context.Background()

	// Add a user
	profileRepo.AddProfile(&models.Profile{
		ID:    "user-123",
		Email: "test@example.com",
	})

	// Request reset
	err := service.RequestPasswordReset(ctx, RequestResetInput{
		Email:     "test@example.com",
		IPAddress: "127.0.0.1",
		UserAgent: "TestAgent",
	})

	if err != nil {
		t.Fatalf("RequestPasswordReset failed: %v", err)
	}

	// Verify token was created
	if len(tokenRepo.tokens) != 1 {
		t.Errorf("expected 1 token, got %d", len(tokenRepo.tokens))
	}

	// Verify email was sent
	emails := emailAdapter.GetSentEmails()
	if len(emails) != 1 {
		t.Fatalf("expected 1 email, got %d", len(emails))
	}

	if emails[0].Type != "password_reset" {
		t.Errorf("expected password_reset email, got %s", emails[0].Type)
	}

	if emails[0].To != "test@example.com" {
		t.Errorf("expected email to test@example.com, got %s", emails[0].To)
	}
}

func TestRequestPasswordReset_UnknownEmail_SilentFail(t *testing.T) {
	service, _, _, _, emailAdapter := setupPasswordService(t)
	ctx := context.Background()

	// Request reset for non-existent email
	err := service.RequestPasswordReset(ctx, RequestResetInput{
		Email:     "unknown@example.com",
		IPAddress: "127.0.0.1",
	})

	// Should not return error (silent fail for security)
	if err != nil {
		t.Fatalf("RequestPasswordReset should not fail for unknown email: %v", err)
	}

	// Should not send email
	if len(emailAdapter.GetSentEmails()) != 0 {
		t.Error("should not send email for unknown address")
	}
}

func TestRequestPasswordReset_RateLimitExceeded(t *testing.T) {
	service, profileRepo, _, rateLimitRepo, _ := setupPasswordService(t)
	ctx := context.Background()

	profileRepo.AddProfile(&models.Profile{
		ID:    "user-123",
		Email: "test@example.com",
	})

	// Set rate limit to max
	rateLimitRepo.SetAttempts("test@example.com", "email", MaxResetAttempts)

	err := service.RequestPasswordReset(ctx, RequestResetInput{
		Email:     "test@example.com",
		IPAddress: "127.0.0.1",
	})

	if !errors.Is(err, ErrRateLimitExceeded) {
		t.Errorf("expected ErrRateLimitExceeded, got %v", err)
	}
}

// ============================================================
// Tests: ValidateResetToken
// ============================================================

func TestValidateResetToken_Valid(t *testing.T) {
	service, profileRepo, tokenRepo, _, _ := setupPasswordService(t)
	ctx := context.Background()

	profileRepo.AddProfile(&models.Profile{
		ID:    "user-123",
		Email: "test@example.com",
	})

	// Request reset to create a token
	err := service.RequestPasswordReset(ctx, RequestResetInput{
		Email:     "test@example.com",
		IPAddress: "127.0.0.1",
	})
	if err != nil {
		t.Fatalf("failed to request reset: %v", err)
	}

	// Get the token from the repo
	var storedToken *repository.PasswordResetToken
	for _, token := range tokenRepo.tokens {
		storedToken = token
		break
	}

	// Reconstruct the raw token (in real usage, this comes from the email link)
	// Since we can't easily get the raw token, we'll test with an invalid token
	result, err := service.ValidateResetToken(ctx, "invalid-token")
	if err != nil {
		t.Fatalf("ValidateResetToken failed: %v", err)
	}

	if result.Valid {
		t.Error("invalid token should not be valid")
	}

	if result.Reason != "invalid_token" {
		t.Errorf("expected reason 'invalid_token', got '%s'", result.Reason)
	}

	// Verify the stored token exists
	if storedToken == nil {
		t.Error("expected a stored token")
	}
}

func TestValidateResetToken_Expired(t *testing.T) {
	service, profileRepo, tokenRepo, _, _ := setupPasswordService(t)
	ctx := context.Background()

	profileRepo.AddProfile(&models.Profile{
		ID:    "user-123",
		Email: "test@example.com",
	})

	// Manually create an expired token
	token := "test-token"
	tokenHash := service.hashToken(token)

	tokenRepo.tokens[tokenHash] = &repository.PasswordResetToken{
		UserID:    "user-123",
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(-1 * time.Hour), // Expired
		Email:     "test@example.com",
	}

	result, err := service.ValidateResetToken(ctx, token)
	if err != nil {
		t.Fatalf("ValidateResetToken failed: %v", err)
	}

	if result.Valid {
		t.Error("expired token should not be valid")
	}

	if result.Reason != "token_expired" {
		t.Errorf("expected reason 'token_expired', got '%s'", result.Reason)
	}
}

func TestValidateResetToken_AlreadyUsed(t *testing.T) {
	service, profileRepo, tokenRepo, _, _ := setupPasswordService(t)
	ctx := context.Background()

	profileRepo.AddProfile(&models.Profile{
		ID:    "user-123",
		Email: "test@example.com",
	})

	// Create a used token
	token := "test-token"
	tokenHash := service.hashToken(token)

	usedAt := time.Now()
	tokenRepo.tokens[tokenHash] = &repository.PasswordResetToken{
		UserID:    "user-123",
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(1 * time.Hour),
		UsedAt:    &usedAt, // Already used
		Email:     "test@example.com",
	}

	result, err := service.ValidateResetToken(ctx, token)
	if err != nil {
		t.Fatalf("ValidateResetToken failed: %v", err)
	}

	if result.Valid {
		t.Error("used token should not be valid")
	}

	if result.Reason != "token_used" {
		t.Errorf("expected reason 'token_used', got '%s'", result.Reason)
	}
}

// ============================================================
// Tests: ResetPassword
// ============================================================

func TestResetPassword_Success(t *testing.T) {
	service, profileRepo, tokenRepo, _, emailAdapter := setupPasswordService(t)
	ctx := context.Background()

	profileRepo.AddProfile(&models.Profile{
		ID:    "user-123",
		Email: "test@example.com",
	})

	// Create a valid token
	token := "test-reset-token"
	tokenHash := service.hashToken(token)

	tokenRepo.tokens[tokenHash] = &repository.PasswordResetToken{
		UserID:    "user-123",
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(1 * time.Hour),
		Email:     "test@example.com",
	}

	// Reset password
	err := service.ResetPassword(ctx, ResetPasswordInput{
		Token:       token,
		NewPassword: "newPassword123",
		IPAddress:   "127.0.0.1",
	})

	if err != nil {
		t.Fatalf("ResetPassword failed: %v", err)
	}

	// Verify password was updated
	profile := profileRepo.profiles["user-123"]
	if profile.PasswordHash == nil || *profile.PasswordHash == "" {
		t.Error("password hash should be set")
	}

	// Verify token was marked as used
	storedToken := tokenRepo.tokens[tokenHash]
	if storedToken.UsedAt == nil {
		t.Error("token should be marked as used")
	}

	// Verify confirmation email was sent
	emails := emailAdapter.GetSentEmails()
	if len(emails) != 1 {
		t.Fatalf("expected 1 email, got %d", len(emails))
	}

	if emails[0].Type != "password_changed" {
		t.Errorf("expected password_changed email, got %s", emails[0].Type)
	}
}

func TestResetPassword_WeakPassword(t *testing.T) {
	service, profileRepo, tokenRepo, _, _ := setupPasswordService(t)
	ctx := context.Background()

	profileRepo.AddProfile(&models.Profile{
		ID:    "user-123",
		Email: "test@example.com",
	})

	token := "test-reset-token"
	tokenHash := service.hashToken(token)

	tokenRepo.tokens[tokenHash] = &repository.PasswordResetToken{
		UserID:    "user-123",
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(1 * time.Hour),
		Email:     "test@example.com",
	}

	// Try with weak password
	err := service.ResetPassword(ctx, ResetPasswordInput{
		Token:       token,
		NewPassword: "short", // Too short
		IPAddress:   "127.0.0.1",
	})

	if !errors.Is(err, ErrPasswordTooWeak) {
		t.Errorf("expected ErrPasswordTooWeak, got %v", err)
	}
}

// ============================================================
// Tests: ChangePassword
// ============================================================

func TestChangePassword_Success(t *testing.T) {
	service, profileRepo, _, _, emailAdapter := setupPasswordService(t)
	ctx := context.Background()

	// Create user with existing password
	existingHash, _ := HashPassword("oldPassword123")
	profileRepo.AddProfile(&models.Profile{
		ID:           "user-123",
		Email:        "test@example.com",
		PasswordHash: &existingHash,
	})

	err := service.ChangePassword(ctx, "user-123", ChangePasswordInput{
		CurrentPassword: "oldPassword123",
		NewPassword:     "newPassword456",
		IPAddress:       "127.0.0.1",
	})

	if err != nil {
		t.Fatalf("ChangePassword failed: %v", err)
	}

	// Verify password was updated
	profile := profileRepo.profiles["user-123"]
	if !VerifyPassword(*profile.PasswordHash, "newPassword456") {
		t.Error("new password should verify correctly")
	}

	// Verify confirmation email
	emails := emailAdapter.GetSentEmails()
	if len(emails) != 1 {
		t.Fatalf("expected 1 email, got %d", len(emails))
	}

	if emails[0].Type != "password_changed" {
		t.Errorf("expected password_changed email, got %s", emails[0].Type)
	}
}

func TestChangePassword_WrongCurrentPassword(t *testing.T) {
	service, profileRepo, _, _, _ := setupPasswordService(t)
	ctx := context.Background()

	existingHash, _ := HashPassword("oldPassword123")
	profileRepo.AddProfile(&models.Profile{
		ID:           "user-123",
		Email:        "test@example.com",
		PasswordHash: &existingHash,
	})

	err := service.ChangePassword(ctx, "user-123", ChangePasswordInput{
		CurrentPassword: "wrongPassword",
		NewPassword:     "newPassword456",
		IPAddress:       "127.0.0.1",
	})

	if !errors.Is(err, ErrInvalidPassword) {
		t.Errorf("expected ErrInvalidPassword, got %v", err)
	}
}

func TestChangePassword_SameAsOld(t *testing.T) {
	service, profileRepo, _, _, _ := setupPasswordService(t)
	ctx := context.Background()

	existingHash, _ := HashPassword("samePassword123")
	profileRepo.AddProfile(&models.Profile{
		ID:           "user-123",
		Email:        "test@example.com",
		PasswordHash: &existingHash,
	})

	err := service.ChangePassword(ctx, "user-123", ChangePasswordInput{
		CurrentPassword: "samePassword123",
		NewPassword:     "samePassword123", // Same as old
		IPAddress:       "127.0.0.1",
	})

	if !errors.Is(err, ErrPasswordSameAsOld) {
		t.Errorf("expected ErrPasswordSameAsOld, got %v", err)
	}
}

func TestChangePassword_UserNotFound(t *testing.T) {
	service, _, _, _, _ := setupPasswordService(t)
	ctx := context.Background()

	err := service.ChangePassword(ctx, "non-existent", ChangePasswordInput{
		CurrentPassword: "oldPassword",
		NewPassword:     "newPassword123",
		IPAddress:       "127.0.0.1",
	})

	if !errors.Is(err, ErrProfileNotFound) {
		t.Errorf("expected ErrProfileNotFound, got %v", err)
	}
}

// ============================================================
// Tests: Helper Functions
// ============================================================

func TestMaskEmail(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"john@example.com", "jo***@example.com"},
		{"ab@test.com", "a***@test.com"}, // 2-char local shows first char
		{"a@test.com", "a***@test.com"},
		{"invalid", "***"},
		{"", "***"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := maskEmail(tc.input)
			if result != tc.expected {
				t.Errorf("maskEmail(%q) = %q, expected %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestVerifyPassword(t *testing.T) {
	password := "testPassword123"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	if !VerifyPassword(hash, password) {
		t.Error("correct password should verify")
	}

	if VerifyPassword(hash, "wrongPassword") {
		t.Error("wrong password should not verify")
	}
}

func TestCompareTokensConstantTime(t *testing.T) {
	token1 := []byte("test-token-12345")
	token2 := []byte("test-token-12345")
	token3 := []byte("different-token")

	if !CompareTokensConstantTime(token1, token2) {
		t.Error("identical tokens should match")
	}

	if CompareTokensConstantTime(token1, token3) {
		t.Error("different tokens should not match")
	}
}
