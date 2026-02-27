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
// Enhanced Mock Profile Repository
// ============================================================

type enhancedMockProfileRepo struct {
	profiles       map[string]*models.Profile
	profileByEmail map[string]*models.Profile
	shouldFail     bool
	failError      error

	// Per-method error overrides
	getProfileErr      error
	getProfileByEmailErr error
	updateProfileErr   error
	updatePasswordErr  error
	listProfilesErr    error
	searchProfilesErr  error

	// List/Search data
	listProfilesData   []*models.Profile
	listNextCursor     string
	searchProfilesData []*models.Profile
	searchNextCursor   string

	// Track calls
	updateProfileCalls []updateProfileCall
}

type updateProfileCall struct {
	UserID  string
	Updates map[string]interface{}
}

func newEnhancedMockProfileRepo() *enhancedMockProfileRepo {
	return &enhancedMockProfileRepo{
		profiles:           make(map[string]*models.Profile),
		profileByEmail:     make(map[string]*models.Profile),
		updateProfileCalls: make([]updateProfileCall, 0),
	}
}

func (m *enhancedMockProfileRepo) GetProfile(ctx context.Context, userID string) (*models.Profile, error) {
	if m.getProfileErr != nil {
		return nil, m.getProfileErr
	}
	if m.shouldFail {
		return nil, m.failError
	}
	profile, ok := m.profiles[userID]
	if !ok {
		return nil, nil
	}
	return profile, nil
}

func (m *enhancedMockProfileRepo) GetProfileByEmail(ctx context.Context, emailAddr string) (*models.Profile, error) {
	if m.getProfileByEmailErr != nil {
		return nil, m.getProfileByEmailErr
	}
	if m.shouldFail {
		return nil, m.failError
	}
	profile, ok := m.profileByEmail[emailAddr]
	if !ok {
		return nil, nil
	}
	return profile, nil
}

func (m *enhancedMockProfileRepo) UpdatePassword(ctx context.Context, userID, passwordHash string) error {
	if m.updatePasswordErr != nil {
		return m.updatePasswordErr
	}
	if m.shouldFail {
		return m.failError
	}
	if profile, ok := m.profiles[userID]; ok {
		profile.PasswordHash = &passwordHash
	}
	return nil
}

func (m *enhancedMockProfileRepo) UpdateProfile(ctx context.Context, userID string, updates map[string]interface{}) error {
	m.updateProfileCalls = append(m.updateProfileCalls, updateProfileCall{UserID: userID, Updates: updates})
	if m.updateProfileErr != nil {
		return m.updateProfileErr
	}
	if m.shouldFail {
		return m.failError
	}
	// Apply updates to in-memory profile
	if profile, ok := m.profiles[userID]; ok {
		for key, val := range updates {
			switch key {
			case "phone_number":
				if v, ok := val.(string); ok {
					profile.PhoneNumber = &v
				}
			case "billing_status":
				if v, ok := val.(string); ok {
					profile.BillingStatus = &v
				}
			case "plan":
				if v, ok := val.(string); ok {
					profile.Plan = v
				}
			case "max_automations":
				if v, ok := val.(int32); ok {
					intV := int(v)
					profile.MaxAutomations = &intV
				}
			}
		}
	}
	return nil
}

func (m *enhancedMockProfileRepo) ListProfiles(ctx context.Context, cursor string, limit int) ([]*models.Profile, string, error) {
	if m.listProfilesErr != nil {
		return nil, "", m.listProfilesErr
	}
	if m.shouldFail {
		return nil, "", m.failError
	}
	if m.listProfilesData != nil {
		return m.listProfilesData, m.listNextCursor, nil
	}
	return nil, "", nil
}

func (m *enhancedMockProfileRepo) SearchProfiles(ctx context.Context, query, cursor string, limit int) ([]*models.Profile, string, error) {
	if m.searchProfilesErr != nil {
		return nil, "", m.searchProfilesErr
	}
	if m.shouldFail {
		return nil, "", m.failError
	}
	if m.searchProfilesData != nil {
		return m.searchProfilesData, m.searchNextCursor, nil
	}
	return nil, "", nil
}

func (m *enhancedMockProfileRepo) EnsureUserProfile(ctx context.Context, userID, emailAddr string) (*models.Profile, error) {
	return nil, nil
}

func (m *enhancedMockProfileRepo) GetProfileByEmailWithBilling(ctx context.Context, emailAddr string) (*models.Profile, *models.ProfileBillingFields, error) {
	if m.shouldFail {
		return nil, nil, m.failError
	}
	profile, ok := m.profileByEmail[emailAddr]
	if !ok {
		return nil, nil, nil
	}
	return profile, nil, nil
}

func (m *enhancedMockProfileRepo) CreateProfile(ctx context.Context, profile *models.Profile) (*models.Profile, error) {
	if m.shouldFail {
		return nil, m.failError
	}
	m.profiles[profile.ID] = profile
	m.profileByEmail[profile.Email] = profile
	return profile, nil
}

func (m *enhancedMockProfileRepo) CreateUserCredentials(ctx context.Context, userID, passwordHash string) error {
	if m.shouldFail {
		return m.failError
	}
	if profile, ok := m.profiles[userID]; ok {
		profile.PasswordHash = &passwordHash
	}
	return nil
}

func (m *enhancedMockProfileRepo) UpdateBillingStatus(ctx context.Context, userID string, billing *models.ProfileBillingFields) error {
	return nil
}

func (m *enhancedMockProfileRepo) PauseUserAutomations(ctx context.Context, userID string) error {
	return nil
}

func (m *enhancedMockProfileRepo) GetMaxAutomations(ctx context.Context, userID string) (int, error) {
	return 5, nil
}

func (m *enhancedMockProfileRepo) CountUserAutomations(ctx context.Context, userID string) (int, error) {
	return 0, nil
}

func (m *enhancedMockProfileRepo) GetProfileStats(ctx context.Context) (*repository.ProfileStats, error) {
	if m.shouldFail {
		return nil, m.failError
	}
	return &repository.ProfileStats{}, nil
}

func (m *enhancedMockProfileRepo) AddProfile(profile *models.Profile) {
	m.profiles[profile.ID] = profile
	if profile.Email != "" {
		m.profileByEmail[profile.Email] = profile
	}
}

// ============================================================
// Enhanced Mock Automation Repository
// ============================================================

type enhancedMockAutomationRepo struct {
	automations map[string][]models.Automation
	shouldFail  bool
	failError   error
}

func newEnhancedMockAutomationRepo() *enhancedMockAutomationRepo {
	return &enhancedMockAutomationRepo{
		automations: make(map[string][]models.Automation),
	}
}

func (m *enhancedMockAutomationRepo) GetUserAutomations(ctx context.Context, userID string) ([]models.Automation, error) {
	if m.shouldFail {
		return nil, m.failError
	}
	return m.automations[userID], nil
}

func (m *enhancedMockAutomationRepo) SetAutomations(userID string, automations []models.Automation) {
	m.automations[userID] = automations
}

func (m *enhancedMockAutomationRepo) GetActiveAutomations(ctx context.Context) ([]models.Automation, error) {
	return nil, nil
}

func (m *enhancedMockAutomationRepo) GetAutomationByID(ctx context.Context, automationID string) (*models.Automation, error) {
	return nil, nil
}

func (m *enhancedMockAutomationRepo) GetAutomationByIDAndUser(ctx context.Context, automationID, userID string) (*models.Automation, error) {
	return nil, nil
}

func (m *enhancedMockAutomationRepo) CreateAutomation(ctx context.Context, userID string, data *models.Automation) (*models.Automation, error) {
	return nil, nil
}

func (m *enhancedMockAutomationRepo) UpdateAutomation(ctx context.Context, automationID string, updates map[string]interface{}) error {
	return nil
}

func (m *enhancedMockAutomationRepo) DeleteAutomation(ctx context.Context, automationID, userID string) error {
	return nil
}

func (m *enhancedMockAutomationRepo) GetMonitoringAutomationsForGroup(ctx context.Context, waGroupID string) ([]models.Automation, error) {
	return nil, nil
}

func (m *enhancedMockAutomationRepo) UpdateLastRunAt(ctx context.Context, automationID string) error {
	return nil
}

func (m *enhancedMockAutomationRepo) UpdateNextRunAt(ctx context.Context, automationID string, nextRun time.Time) error {
	return nil
}

func (m *enhancedMockAutomationRepo) BulkUpdateStatusByUserID(ctx context.Context, userID, targetStatus string, sourceStatus *string) (int, error) {
	return 0, nil
}

func (m *enhancedMockAutomationRepo) BulkUpdateNextRunAts(ctx context.Context, automationIDs []string, nextRunAts []time.Time) error {
	return nil
}

// ============================================================
// Enhanced Mock Instance Repository
// ============================================================

type enhancedMockInstanceRepo struct {
	instances  map[string][]models.Instance
	shouldFail bool
	failError  error
}

func newEnhancedMockInstanceRepo() *enhancedMockInstanceRepo {
	return &enhancedMockInstanceRepo{
		instances: make(map[string][]models.Instance),
	}
}

func (m *enhancedMockInstanceRepo) GetUserInstances(ctx context.Context, userID string) ([]models.Instance, error) {
	if m.shouldFail {
		return nil, m.failError
	}
	return m.instances[userID], nil
}

func (m *enhancedMockInstanceRepo) SetInstances(userID string, instances []models.Instance) {
	m.instances[userID] = instances
}

func (m *enhancedMockInstanceRepo) GetInstanceByID(ctx context.Context, instanceID int) (*models.Instance, error) {
	return nil, nil
}

func (m *enhancedMockInstanceRepo) GetInstanceByName(ctx context.Context, instanceName string) (*models.Instance, error) {
	return nil, nil
}

func (m *enhancedMockInstanceRepo) GetInstanceByUserAndName(ctx context.Context, userID, instanceName string) (*models.Instance, error) {
	return nil, nil
}

func (m *enhancedMockInstanceRepo) GetAllConnectedInstances(ctx context.Context) ([]models.Instance, error) {
	return nil, nil
}

func (m *enhancedMockInstanceRepo) UpsertInstance(ctx context.Context, userID string, data *models.Instance) (*models.Instance, error) {
	return nil, nil
}

func (m *enhancedMockInstanceRepo) UpdateInstanceStatus(ctx context.Context, instanceID int, status string) error {
	return nil
}

func (m *enhancedMockInstanceRepo) DeleteInstance(ctx context.Context, instanceID int) error {
	return nil
}

// ============================================================
// Enhanced Mock User Role Repository
// ============================================================

type enhancedMockUserRoleRepo struct {
	admins     map[string]bool
	shouldFail bool
	failError  error
}

func newEnhancedMockUserRoleRepo() *enhancedMockUserRoleRepo {
	return &enhancedMockUserRoleRepo{
		admins: make(map[string]bool),
	}
}

func (m *enhancedMockUserRoleRepo) IsAdmin(ctx context.Context, userID string) (bool, error) {
	if m.shouldFail {
		return false, m.failError
	}
	return m.admins[userID], nil
}

func (m *enhancedMockUserRoleRepo) SetAdmin(userID string, isAdmin bool) {
	m.admins[userID] = isAdmin
}

func (m *enhancedMockUserRoleRepo) GetUserRole(ctx context.Context, userID string) (*models.UserRole, error) {
	return nil, nil
}

func (m *enhancedMockUserRoleRepo) SetUserRole(ctx context.Context, userID, role string) error {
	return nil
}

func (m *enhancedMockUserRoleRepo) RemoveUserRole(ctx context.Context, userID, role string) error {
	return nil
}

func (m *enhancedMockUserRoleRepo) GetAllUserRoles(ctx context.Context, userID string) ([]string, error) {
	return nil, nil
}

func (m *enhancedMockUserRoleRepo) GetAdminUsers(ctx context.Context) ([]models.UserRole, error) {
	return nil, nil
}

// ============================================================
// Enhanced Mock Token Repository
// ============================================================

type enhancedMockTokenRepo struct {
	tokens     map[string]*repository.PasswordResetToken
	shouldFail bool
	failError  error

	// Per-method error overrides
	createErr             error
	getByHashErr          error
	markUsedErr           error
	invalidateTokensErr   error

	// Call counting for GetByHash (to fail on Nth call)
	getByHashCallCount     int
	getByHashFailAfterCall int // 0 = disabled; if set, fail starting from this call number
	getByHashFailAfterErr  error
}

func newEnhancedMockTokenRepo() *enhancedMockTokenRepo {
	return &enhancedMockTokenRepo{
		tokens: make(map[string]*repository.PasswordResetToken),
	}
}

func (m *enhancedMockTokenRepo) Create(ctx context.Context, token *repository.PasswordResetToken) error {
	if m.createErr != nil {
		return m.createErr
	}
	if m.shouldFail {
		return m.failError
	}
	m.tokens[token.TokenHash] = token
	return nil
}

func (m *enhancedMockTokenRepo) GetByHash(ctx context.Context, tokenHash string) (*repository.PasswordResetToken, error) {
	m.getByHashCallCount++
	if m.getByHashFailAfterCall > 0 && m.getByHashCallCount >= m.getByHashFailAfterCall {
		return nil, m.getByHashFailAfterErr
	}
	if m.getByHashErr != nil {
		return nil, m.getByHashErr
	}
	if m.shouldFail {
		return nil, m.failError
	}
	token, ok := m.tokens[tokenHash]
	if !ok {
		return nil, errors.New("token not found")
	}
	return token, nil
}

func (m *enhancedMockTokenRepo) MarkUsed(ctx context.Context, tokenHash string) error {
	if m.markUsedErr != nil {
		return m.markUsedErr
	}
	if m.shouldFail {
		return m.failError
	}
	if token, ok := m.tokens[tokenHash]; ok {
		now := time.Now()
		token.UsedAt = &now
	}
	return nil
}

func (m *enhancedMockTokenRepo) InvalidateUserTokens(ctx context.Context, userID string) error {
	if m.invalidateTokensErr != nil {
		return m.invalidateTokensErr
	}
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

func (m *enhancedMockTokenRepo) CleanupExpired(ctx context.Context) (int64, error) {
	return 0, nil
}

// ============================================================
// Enhanced Mock Rate Limit Repository
// ============================================================

type enhancedMockRateLimitRepo struct {
	attempts   map[string]int
	shouldFail bool
	failError  error

	// Per-method error overrides
	getAttemptsErr error
	incrementErr   error
}

func newEnhancedMockRateLimitRepo() *enhancedMockRateLimitRepo {
	return &enhancedMockRateLimitRepo{
		attempts: make(map[string]int),
	}
}

func (m *enhancedMockRateLimitRepo) GetAttempts(ctx context.Context, identifier, identifierType string, window time.Duration) (int, error) {
	if m.getAttemptsErr != nil {
		return 0, m.getAttemptsErr
	}
	if m.shouldFail {
		return 0, m.failError
	}
	key := identifier + ":" + identifierType
	return m.attempts[key], nil
}

func (m *enhancedMockRateLimitRepo) Increment(ctx context.Context, identifier, identifierType string) error {
	if m.incrementErr != nil {
		return m.incrementErr
	}
	if m.shouldFail {
		return m.failError
	}
	key := identifier + ":" + identifierType
	m.attempts[key]++
	return nil
}

func (m *enhancedMockRateLimitRepo) Cleanup(ctx context.Context) (int64, error) {
	return 0, nil
}

func (m *enhancedMockRateLimitRepo) Reset(ctx context.Context, identifier, identifierType string) error {
	key := identifier + ":" + identifierType
	m.attempts[key] = 0
	return nil
}

func (m *enhancedMockRateLimitRepo) SetAttempts(identifier, identifierType string, count int) {
	key := identifier + ":" + identifierType
	m.attempts[key] = count
}

// ============================================================
// Setup Functions
// ============================================================

func setupEnhancedProfileService(t *testing.T) (*ProfileService, *enhancedMockProfileRepo, *enhancedMockAutomationRepo, *enhancedMockInstanceRepo, *enhancedMockUserRoleRepo) {
	t.Helper()

	profileRepo := newEnhancedMockProfileRepo()
	automationRepo := newEnhancedMockAutomationRepo()
	instanceRepo := newEnhancedMockInstanceRepo()
	userRoleRepo := newEnhancedMockUserRoleRepo()
	logger := zap.NewNop()

	service := NewProfileService(profileRepo, automationRepo, instanceRepo, userRoleRepo, logger)
	return service, profileRepo, automationRepo, instanceRepo, userRoleRepo
}

func setupEnhancedPasswordService(t *testing.T) (*PasswordService, *enhancedMockProfileRepo, *enhancedMockTokenRepo, *enhancedMockRateLimitRepo, *email.FakeEmailAdapter) {
	t.Helper()

	profileRepo := newEnhancedMockProfileRepo()
	tokenRepo := newEnhancedMockTokenRepo()
	rateLimitRepo := newEnhancedMockRateLimitRepo()
	emailAdapter := email.NewFakeEmailAdapter()

	hmacKey := make([]byte, 32)
	if _, err := rand.Read(hmacKey); err != nil {
		t.Fatalf("failed to generate HMAC key: %v", err)
	}

	logger := zap.NewNop()

	service := NewPasswordService(
		profileRepo, tokenRepo, rateLimitRepo, emailAdapter,
		PasswordServiceConfig{HMACKey: hmacKey, FrontendURL: "https://app.test.com"},
		logger,
	)

	return service, profileRepo, tokenRepo, rateLimitRepo, emailAdapter
}
