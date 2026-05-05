// Package e2e provides end-to-end tests for the Automatiza Go backend.
// These tests verify the HTTP/gRPC layer with real database operations
// and mocked external services (WAHA, email).
package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"automatiza-go/internal/adapters/email"
	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/config"
	"automatiza-go/internal/handlers"
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"
	"automatiza-go/internal/services/redirect"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/zap"
)

// testEnv holds the test environment configuration and resources.
type testEnv struct {
	container        testcontainers.Container
	pool             *pgxpool.Pool
	router           *gin.Engine
	deps             *handlers.Dependencies
	logger           *zap.Logger
	emailAdapter     *email.FakeEmailAdapter
	wahaAdapter      *whatsapp.FakeAdapter
	redirectLinkRepo *mockRedirectLinkRepo
}

// newTestEnv creates a new test environment with mock repositories.
// For full E2E tests with PostgreSQL, use newTestEnvWithDB.
func newTestEnv(t *testing.T) *testEnv {
	t.Helper()

	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	return createTestEnvWithMocks(t)
}

// newTestEnvWithDB creates a test environment with PostgreSQL testcontainer.
// This is slower but provides real database integration testing.
func newTestEnvWithDB(t *testing.T) *testEnv {
	t.Helper()

	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	ctx := context.Background()

	// Create PostgreSQL container
	pgContainer, err := postgres.Run(ctx,
		"postgres:15-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	require.NoError(t, err, "failed to start postgres container")

	// Get connection string
	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err, "failed to get connection string")

	// Create connection pool
	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err, "failed to create pool")

	// Ping to verify connection
	require.NoError(t, pool.Ping(ctx), "failed to ping database")

	// Create schema
	err = createSchema(ctx, pool)
	require.NoError(t, err, "failed to create schema")

	env := createTestEnvWithMocks(t)
	env.container = pgContainer
	env.pool = pool

	return env
}

// createTestEnvWithMocks creates a test environment using mock repositories.
func createTestEnvWithMocks(t *testing.T) *testEnv {
	t.Helper()

	// Create logger
	logger := zap.NewNop()

	// Create fake adapters
	emailAdapter := email.NewFakeEmailAdapter()
	wahaAdapter := whatsapp.NewFakeAdapter()

	// Create mock repositories
	profileRepo := newMockProfileRepo()
	instanceRepo := newMockInstanceRepo()
	groupRepo := newMockGroupRepo()
	automationRepo := newMockAutomationRepo()
	userRoleRepo := newMockUserRoleRepo()
	redirectLinkRepo := newMockRedirectLinkRepo()

	// Create redirect service
	redirectService := redirect.NewRedirectService(
		redirectLinkRepo,
		groupRepo,
		instanceRepo,
		wahaAdapter,
		logger,
	)

	// Create config
	cfg := &config.Config{
		NodeEnv:    "test",
		APIVersion: "v1",
		Port:       "3000",
	}

	// Create dependencies
	// Create mock system event repo
	systemEventRepo := newMockSystemEventRepo()

	deps := &handlers.Dependencies{
		Config:           cfg,
		Logger:           logger,
		WhatsAppProvider: wahaAdapter,
		RedirectService:  redirectService,
		ProfileRepo:      profileRepo,
		InstanceRepo:     instanceRepo,
		GroupRepo:        groupRepo,
		AutomationRepo:   automationRepo,
		UserRoleRepo:     userRoleRepo,
		SystemEventRepo:  systemEventRepo,
	}

	// Setup Gin router in test mode
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(gin.Recovery()) // Add recovery middleware to handle panics
	handlers.SetupRoutes(router, deps)

	return &testEnv{
		container:        nil,
		pool:             nil,
		router:           router,
		deps:             deps,
		logger:           logger,
		emailAdapter:     emailAdapter,
		wahaAdapter:      wahaAdapter,
		redirectLinkRepo: redirectLinkRepo,
	}
}

// cleanup releases test environment resources.
func (e *testEnv) cleanup(t *testing.T) {
	t.Helper()
	ctx := context.Background()

	if e.pool != nil {
		e.pool.Close()
	}
	if e.container != nil {
		require.NoError(t, e.container.Terminate(ctx), "failed to terminate container")
	}
}

// createSchema creates the database schema for tests.
func createSchema(ctx context.Context, pool *pgxpool.Pool) error {
	schema := `
		-- Profiles table
		CREATE TABLE IF NOT EXISTS profiles (
			id VARCHAR(36) PRIMARY KEY,
			email VARCHAR(255) NOT NULL UNIQUE,
			plan VARCHAR(20) NOT NULL DEFAULT 'free',
			phone_number VARCHAR(20),
			password_hash TEXT,
			billing_status VARCHAR(20) DEFAULT 'active',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		-- User roles table
		CREATE TABLE IF NOT EXISTS user_roles (
			id VARCHAR(36) PRIMARY KEY DEFAULT gen_random_uuid()::text,
			user_id VARCHAR(36) NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
			role VARCHAR(20) NOT NULL DEFAULT 'user',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE(user_id, role)
		);

		-- Instances table
		CREATE TABLE IF NOT EXISTS instances (
			id SERIAL PRIMARY KEY,
			user_id VARCHAR(36) NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
			instance_id VARCHAR(128) NOT NULL,
			friendly_name VARCHAR(255),
			status VARCHAR(20) NOT NULL DEFAULT 'disconnected',
			qr_code TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE(user_id, instance_id)
		);

		-- Groups table
		CREATE TABLE IF NOT EXISTS groups (
			id VARCHAR(36) PRIMARY KEY DEFAULT gen_random_uuid()::text,
			name VARCHAR(255) NOT NULL,
			wa_group_id VARCHAR(128) NOT NULL,
			instance_id VARCHAR(10) NOT NULL,
			admin_only BOOLEAN NOT NULL DEFAULT false,
			description TEXT,
			picture_url TEXT,
			user_id VARCHAR(36) NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
			is_admin BOOLEAN NOT NULL DEFAULT false,
			is_favorite BOOLEAN NOT NULL DEFAULT false,
			status VARCHAR(20) NOT NULL DEFAULT 'open',
			invite_code VARCHAR(128),
			members_count INTEGER NOT NULL DEFAULT 0,
			member_limit INTEGER NOT NULL DEFAULT 1024,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE(user_id, wa_group_id)
		);

		-- Automations table
		CREATE TABLE IF NOT EXISTS automations (
			id VARCHAR(36) PRIMARY KEY DEFAULT gen_random_uuid()::text,
			user_id VARCHAR(36) NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
			name VARCHAR(255) NOT NULL,
			group_ids TEXT[] DEFAULT '{}',
			shopee_link TEXT,
			custom_message TEXT,
			include_image BOOLEAN NOT NULL DEFAULT false,
			schedule_time VARCHAR(20),
			schedule_days TEXT[] DEFAULT '{}',
			status VARCHAR(20) NOT NULL DEFAULT 'inactive',
			type VARCHAR(20) DEFAULT 'search',
			monitor_group_ids TEXT[] DEFAULT '{}',
			delay_seconds INTEGER,
			dedupe_hours INTEGER,
			dispatch_text_id VARCHAR(36),
			mode VARCHAR(20),
			stores TEXT[] DEFAULT '{}',
			categories TEXT[] DEFAULT '{}',
			keyword TEXT,
			min_price DECIMAL(10,2),
			max_price DECIMAL(10,2),
			min_discount DECIMAL(5,2),
			send_groups TEXT[] DEFAULT '{}',
			start_time VARCHAR(10),
			end_time VARCHAR(10),
			interval_minutes INTEGER,
			last_run_at TIMESTAMPTZ,
			next_run_at TIMESTAMPTZ,
			monitoring_text_rules JSONB,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		-- Redirect links table
		CREATE TABLE IF NOT EXISTS redirect_links (
			id VARCHAR(36) PRIMARY KEY DEFAULT gen_random_uuid()::text,
			user_id VARCHAR(36) NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
			slug VARCHAR(50) NOT NULL UNIQUE,
			name VARCHAR(255) NOT NULL,
			strategy VARCHAR(20) NOT NULL DEFAULT 'round_robin',
			target_groups JSONB NOT NULL DEFAULT '[]',
			click_count INTEGER NOT NULL DEFAULT 0,
			group_clicks JSONB NOT NULL DEFAULT '{}',
			is_active BOOLEAN NOT NULL DEFAULT true,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		-- Password reset tokens table
		CREATE TABLE IF NOT EXISTS password_reset_tokens (
			id VARCHAR(36) PRIMARY KEY DEFAULT gen_random_uuid()::text,
			user_id VARCHAR(36) NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
			token_hash BYTEA NOT NULL,
			expires_at TIMESTAMPTZ NOT NULL,
			used_at TIMESTAMPTZ,
			ip_address VARCHAR(45),
			user_agent TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		-- Rate limit attempts table
		CREATE TABLE IF NOT EXISTS rate_limit_attempts (
			id SERIAL PRIMARY KEY,
			identifier VARCHAR(255) NOT NULL,
			identifier_type VARCHAR(50) NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE INDEX IF NOT EXISTS idx_rate_limit_identifier ON rate_limit_attempts(identifier, identifier_type);
	`

	_, err := pool.Exec(ctx, schema)
	return err
}

// httpClient provides helper methods for making HTTP requests to the test server.
type httpClient struct {
	t      *testing.T
	router *gin.Engine
	token  string
}

// newHTTPClient creates a new HTTP client for testing.
func newHTTPClient(t *testing.T, router *gin.Engine) *httpClient {
	return &httpClient{
		t:      t,
		router: router,
	}
}

// setAuthToken sets the authentication token for requests.
func (c *httpClient) setAuthToken(token string) {
	c.token = token
}

// doRequest performs an HTTP request and returns the response.
func (c *httpClient) doRequest(method, path string, body interface{}) *httptest.ResponseRecorder {
	c.t.Helper()

	var bodyReader *bytes.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		require.NoError(c.t, err, "failed to marshal request body")
		bodyReader = bytes.NewReader(jsonBody)
	} else {
		bodyReader = bytes.NewReader(nil)
	}

	req, err := http.NewRequest(method, path, bodyReader)
	require.NoError(c.t, err, "failed to create request")

	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	w := httptest.NewRecorder()
	c.router.ServeHTTP(w, req)

	return w
}

// get performs a GET request.
func (c *httpClient) get(path string) *httptest.ResponseRecorder {
	return c.doRequest(http.MethodGet, path, nil)
}

// post performs a POST request.
func (c *httpClient) post(path string, body interface{}) *httptest.ResponseRecorder {
	return c.doRequest(http.MethodPost, path, body)
}

// put performs a PUT request.
func (c *httpClient) put(path string, body interface{}) *httptest.ResponseRecorder {
	return c.doRequest(http.MethodPut, path, body)
}

// delete performs a DELETE request.
func (c *httpClient) delete(path string) *httptest.ResponseRecorder {
	return c.doRequest(http.MethodDelete, path, nil)
}

// parseResponse parses the response body into the given struct.
func parseResponse[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()

	var result T
	err := json.Unmarshal(w.Body.Bytes(), &result)
	require.NoError(t, err, "failed to parse response: %s", w.Body.String())
	return result
}

// apiResponse represents the standard API response wrapper.
type apiResponse[T any] struct {
	Success bool   `json:"success"`
	Data    T      `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
}

// =============================================================================
// Mock Repositories for E2E Tests
// =============================================================================

// mockProfileRepo is a mock implementation of ProfileRepository.
type mockProfileRepo struct {
	profiles       map[string]*models.Profile
	profileByEmail map[string]*models.Profile
}

func newMockProfileRepo() *mockProfileRepo {
	return &mockProfileRepo{
		profiles:       make(map[string]*models.Profile),
		profileByEmail: make(map[string]*models.Profile),
	}
}

func (m *mockProfileRepo) GetProfile(ctx context.Context, userID string) (*models.Profile, error) {
	profile, ok := m.profiles[userID]
	if !ok {
		return nil, fmt.Errorf("profile not found: %s", userID)
	}
	return profile, nil
}

func (m *mockProfileRepo) GetProfileByEmail(ctx context.Context, email string) (*models.Profile, error) {
	profile, ok := m.profileByEmail[email]
	if !ok {
		return nil, nil
	}
	return profile, nil
}

func (m *mockProfileRepo) GetProfileByEmailWithBilling(ctx context.Context, email string) (*models.Profile, *models.ProfileBillingFields, error) {
	profile, ok := m.profileByEmail[email]
	if !ok {
		return nil, nil, nil
	}
	return profile, nil, nil
}

func (m *mockProfileRepo) EnsureUserProfile(ctx context.Context, userID, email string) (*models.Profile, error) {
	if profile, ok := m.profiles[userID]; ok {
		return profile, nil
	}
	profile := &models.Profile{ID: userID, Email: email, Plan: models.PlanFree}
	m.profiles[userID] = profile
	m.profileByEmail[email] = profile
	return profile, nil
}

func (m *mockProfileRepo) CreateProfile(ctx context.Context, profile *models.Profile) (*models.Profile, error) {
	m.profiles[profile.ID] = profile
	m.profileByEmail[profile.Email] = profile
	return profile, nil
}

func (m *mockProfileRepo) UpdateProfile(ctx context.Context, userID string, updates map[string]interface{}) error {
	return nil
}

func (m *mockProfileRepo) UpdatePassword(ctx context.Context, userID, passwordHash string) error {
	if profile, ok := m.profiles[userID]; ok {
		profile.PasswordHash = &passwordHash
	}
	return nil
}

func (m *mockProfileRepo) UpdateBillingStatus(ctx context.Context, userID string, billing *models.ProfileBillingFields) error {
	return nil
}

func (m *mockProfileRepo) PauseUserAutomations(ctx context.Context, userID string) error {
	return nil
}

func (m *mockProfileRepo) GetMaxAutomations(ctx context.Context, userID string) (int, error) {
	return 5, nil // Default max
}

func (m *mockProfileRepo) CountUserAutomations(ctx context.Context, userID string) (int, error) {
	return 0, nil // Default count
}

func (m *mockProfileRepo) ListProfiles(ctx context.Context, cursor string, limit int) ([]*models.Profile, string, error) {
	return nil, "", nil
}

func (m *mockProfileRepo) SearchProfiles(ctx context.Context, query, cursor string, limit int) ([]*models.Profile, string, error) {
	return nil, "", nil
}

func (m *mockProfileRepo) CreateUserCredentials(ctx context.Context, userID, passwordHash string) error {
	return nil
}

func (m *mockProfileRepo) GetProfileStats(ctx context.Context) (*repository.ProfileStats, error) {
	return &repository.ProfileStats{}, nil
}

func (m *mockProfileRepo) AddProfile(profile *models.Profile) {
	m.profiles[profile.ID] = profile
	m.profileByEmail[profile.Email] = profile
}

// mockInstanceRepo is a mock implementation of InstanceRepository.
type mockInstanceRepo struct {
	instances map[int]*models.Instance
	byUserID  map[string][]models.Instance
}

func newMockInstanceRepo() *mockInstanceRepo {
	return &mockInstanceRepo{
		instances: make(map[int]*models.Instance),
		byUserID:  make(map[string][]models.Instance),
	}
}

func (m *mockInstanceRepo) GetUserInstances(ctx context.Context, userID string) ([]models.Instance, error) {
	return m.byUserID[userID], nil
}

func (m *mockInstanceRepo) GetInstanceByID(ctx context.Context, instanceID int) (*models.Instance, error) {
	instance, ok := m.instances[instanceID]
	if !ok {
		return nil, nil
	}
	return instance, nil
}

func (m *mockInstanceRepo) GetInstanceByName(ctx context.Context, instanceName string) (*models.Instance, error) {
	for _, inst := range m.instances {
		if inst.InstanceID == instanceName {
			return inst, nil
		}
	}
	return nil, nil
}

func (m *mockInstanceRepo) GetInstanceByUserAndName(ctx context.Context, userID, instanceName string) (*models.Instance, error) {
	for _, inst := range m.byUserID[userID] {
		if inst.InstanceID == instanceName {
			return &inst, nil
		}
	}
	return nil, nil
}

func (m *mockInstanceRepo) GetAllConnectedInstances(ctx context.Context) ([]models.Instance, error) {
	var result []models.Instance
	for _, inst := range m.instances {
		if inst.IsConnected() {
			result = append(result, *inst)
		}
	}
	return result, nil
}

func (m *mockInstanceRepo) UpsertInstance(ctx context.Context, userID string, data *models.Instance) (*models.Instance, error) {
	m.instances[data.ID] = data
	m.byUserID[userID] = append(m.byUserID[userID], *data)
	return data, nil
}

func (m *mockInstanceRepo) UpdateInstanceStatus(ctx context.Context, instanceID int, status string) error {
	if inst, ok := m.instances[instanceID]; ok {
		inst.Status = status
	}
	return nil
}

func (m *mockInstanceRepo) DeleteInstance(ctx context.Context, instanceID int) error {
	delete(m.instances, instanceID)
	return nil
}

func (m *mockInstanceRepo) SetDefaultInstance(ctx context.Context, userID string, instanceID int) error {
	return nil
}

func (m *mockInstanceRepo) CountUserInstances(ctx context.Context, userID string) (int, error) {
	return len(m.byUserID[userID]), nil
}

func (m *mockInstanceRepo) UpdateInstanceLabel(ctx context.Context, userID string, instanceID int, friendlyName string) error {
	return nil
}

func (m *mockInstanceRepo) UpdateInstancePhoneNumber(ctx context.Context, instanceID int, phoneNumber string) error {
	return nil
}

func (m *mockInstanceRepo) AddInstance(instance *models.Instance) {
	m.instances[instance.ID] = instance
	m.byUserID[instance.UserID] = append(m.byUserID[instance.UserID], *instance)
}

// mockGroupRepo is a mock implementation of GroupRepository.
type mockGroupRepo struct {
	groups      map[string]*models.Group
	byUserID    map[string][]models.Group
	byWAGroupID map[string]*models.Group
}

func newMockGroupRepo() *mockGroupRepo {
	return &mockGroupRepo{
		groups:      make(map[string]*models.Group),
		byUserID:    make(map[string][]models.Group),
		byWAGroupID: make(map[string]*models.Group),
	}
}

func (m *mockGroupRepo) GetUserGroups(ctx context.Context, userID string, adminOnly bool) ([]models.Group, error) {
	groups := m.byUserID[userID]
	if adminOnly {
		var result []models.Group
		for _, g := range groups {
			if g.IsAdmin {
				result = append(result, g)
			}
		}
		return result, nil
	}
	return groups, nil
}

func (m *mockGroupRepo) GetGroupByID(ctx context.Context, groupID string) (*models.Group, error) {
	group, ok := m.groups[groupID]
	if !ok {
		return nil, nil
	}
	return group, nil
}

func (m *mockGroupRepo) GetGroupByWAGroupID(ctx context.Context, userID, waGroupID string) (*models.Group, error) {
	key := userID + ":" + waGroupID
	group, ok := m.byWAGroupID[key]
	if !ok {
		return nil, nil
	}
	return group, nil
}

func (m *mockGroupRepo) GetGroupsByIDs(ctx context.Context, userID string, groupIDs []string) ([]models.Group, error) {
	var result []models.Group
	for _, id := range groupIDs {
		if group, ok := m.groups[id]; ok && group.UserID == userID {
			result = append(result, *group)
		}
	}
	return result, nil
}

func (m *mockGroupRepo) GetFavoriteGroups(ctx context.Context, userID string) ([]models.Group, error) {
	var result []models.Group
	for _, g := range m.byUserID[userID] {
		if g.IsFavorite {
			result = append(result, g)
		}
	}
	return result, nil
}

func (m *mockGroupRepo) CreateGroup(ctx context.Context, userID string, group *models.Group) (*models.Group, error) {
	group.UserID = userID
	m.groups[group.ID] = group
	m.byUserID[userID] = append(m.byUserID[userID], *group)
	key := userID + ":" + group.WAGroupID
	m.byWAGroupID[key] = group
	return group, nil
}

func (m *mockGroupRepo) SyncGroups(ctx context.Context, userID string, instanceID int, groups []models.Group, options repository.SyncOptions) error {
	return nil
}

func (m *mockGroupRepo) SyncGroupsWithStats(ctx context.Context, userID string, instanceID int, groups []models.Group, options repository.SyncOptions) (*repository.SyncResult, error) {
	return &repository.SyncResult{Created: len(groups), Updated: 0, Deleted: 0}, nil
}

func (m *mockGroupRepo) IsUserAdminInGroup(ctx context.Context, userID, waGroupID string) (bool, error) {
	key := userID + ":" + waGroupID
	if group, ok := m.byWAGroupID[key]; ok {
		return group.IsAdmin, nil
	}
	return false, nil
}

func (m *mockGroupRepo) UpdateGroup(ctx context.Context, groupID string, updates map[string]interface{}) error {
	return nil
}

func (m *mockGroupRepo) ToggleFavorite(ctx context.Context, groupID, userID string, favorite bool) error {
	return nil
}

func (m *mockGroupRepo) DeleteGroup(ctx context.Context, groupID, userID string) error {
	delete(m.groups, groupID)
	return nil
}

func (m *mockGroupRepo) DeleteGroupsByInstance(ctx context.Context, userID string, instanceID int) error {
	return nil
}

func (m *mockGroupRepo) BulkUpdateGroups(ctx context.Context, userID string, groupIDs []string, updates map[string]interface{}) (int64, error) {
	return 0, nil
}

func (m *mockGroupRepo) GetUserGroupsPaginated(ctx context.Context, userID string, page, pageSize int, adminOnly bool, search string) (*repository.GroupPaginatedResult, error) {
	return &repository.GroupPaginatedResult{}, nil
}

func (m *mockGroupRepo) CountAll(ctx context.Context) (int64, error) {
	return 0, nil
}

func (m *mockGroupRepo) CountGroupsByInstance(ctx context.Context, userID string, instanceID int) (int, error) {
	return 0, nil
}

func (m *mockGroupRepo) GetInstancesForGroups(ctx context.Context, userID string, waGroupIDs []string) (map[string]*models.Instance, error) {
	return map[string]*models.Instance{}, nil
}

func (m *mockGroupRepo) AddGroup(group *models.Group) {
	m.groups[group.ID] = group
	m.byUserID[group.UserID] = append(m.byUserID[group.UserID], *group)
	key := group.UserID + ":" + group.WAGroupID
	m.byWAGroupID[key] = group
}

// mockAutomationRepo is a mock implementation of AutomationRepository.
type mockAutomationRepo struct {
	automations map[string]*models.Automation
	byUserID    map[string][]models.Automation
}

func newMockAutomationRepo() *mockAutomationRepo {
	return &mockAutomationRepo{
		automations: make(map[string]*models.Automation),
		byUserID:    make(map[string][]models.Automation),
	}
}

func (m *mockAutomationRepo) GetUserAutomations(ctx context.Context, userID string) ([]models.Automation, error) {
	return m.byUserID[userID], nil
}

func (m *mockAutomationRepo) GetActiveAutomations(ctx context.Context) ([]models.Automation, error) {
	var result []models.Automation
	for _, auto := range m.automations {
		if auto.IsActive() {
			result = append(result, *auto)
		}
	}
	return result, nil
}

func (m *mockAutomationRepo) GetAutomationByID(ctx context.Context, automationID string) (*models.Automation, error) {
	auto, ok := m.automations[automationID]
	if !ok {
		return nil, nil
	}
	return auto, nil
}

func (m *mockAutomationRepo) GetAutomationByIDAndUser(ctx context.Context, automationID, userID string) (*models.Automation, error) {
	auto, ok := m.automations[automationID]
	if !ok || auto.UserID != userID {
		return nil, nil
	}
	return auto, nil
}

func (m *mockAutomationRepo) CreateAutomation(ctx context.Context, userID string, data *models.Automation) (*models.Automation, error) {
	data.ID = fmt.Sprintf("auto-%d", len(m.automations)+1)
	data.UserID = userID
	data.CreatedAt = time.Now().Format(time.RFC3339)
	data.UpdatedAt = data.CreatedAt
	m.automations[data.ID] = data
	m.byUserID[userID] = append(m.byUserID[userID], *data)
	return data, nil
}

func (m *mockAutomationRepo) UpdateAutomation(ctx context.Context, automationID string, updates map[string]interface{}) error {
	auto, ok := m.automations[automationID]
	if !ok {
		return fmt.Errorf("automation not found")
	}

	if name, ok := updates["name"].(string); ok {
		auto.Name = name
	}
	if status, ok := updates["status"].(string); ok {
		auto.Status = status
	}
	auto.UpdatedAt = time.Now().Format(time.RFC3339)
	return nil
}

func (m *mockAutomationRepo) DeleteAutomation(ctx context.Context, automationID, userID string) error {
	delete(m.automations, automationID)

	// Remove from byUserID slice
	automations := m.byUserID[userID]
	for i, a := range automations {
		if a.ID == automationID {
			m.byUserID[userID] = append(automations[:i], automations[i+1:]...)
			break
		}
	}
	return nil
}

func (m *mockAutomationRepo) GetMonitoringAutomationsForGroup(ctx context.Context, waGroupID string) ([]models.Automation, error) {
	return nil, nil
}

func (m *mockAutomationRepo) UpdateLastRunAt(ctx context.Context, automationID string) error {
	return nil
}

func (m *mockAutomationRepo) UpdateNextRunAt(ctx context.Context, automationID string, nextRun time.Time) error {
	return nil
}

func (m *mockAutomationRepo) BulkUpdateStatusByUserID(ctx context.Context, userID, targetStatus string, sourceStatus *string) (int, error) {
	return 0, nil
}

func (m *mockAutomationRepo) BulkUpdateNextRunAts(ctx context.Context, automationIDs []string, nextRunAts []time.Time) error {
	return nil
}

func (m *mockAutomationRepo) GetOfferGenerateAutomationsForGroup(ctx context.Context, waGroupID string) ([]models.Automation, error) {
	return nil, nil
}

func (m *mockAutomationRepo) AddAutomation(auto *models.Automation) {
	m.automations[auto.ID] = auto
	m.byUserID[auto.UserID] = append(m.byUserID[auto.UserID], *auto)
}

// mockUserRoleRepo is a mock implementation of UserRoleRepository.
type mockUserRoleRepo struct {
	roles  map[string]*models.UserRole
	admins map[string]bool
}

func newMockUserRoleRepo() *mockUserRoleRepo {
	return &mockUserRoleRepo{
		roles:  make(map[string]*models.UserRole),
		admins: make(map[string]bool),
	}
}

func (m *mockUserRoleRepo) GetUserRole(ctx context.Context, userID string) (*models.UserRole, error) {
	role, ok := m.roles[userID]
	if !ok {
		return nil, nil
	}
	return role, nil
}

func (m *mockUserRoleRepo) SetUserRole(ctx context.Context, userID, role string) error {
	m.roles[userID] = &models.UserRole{UserID: userID, Role: role}
	if role == "admin" {
		m.admins[userID] = true
	}
	return nil
}

func (m *mockUserRoleRepo) RemoveUserRole(ctx context.Context, userID, role string) error {
	delete(m.roles, userID)
	delete(m.admins, userID)
	return nil
}

func (m *mockUserRoleRepo) IsAdmin(ctx context.Context, userID string) (bool, error) {
	return m.admins[userID], nil
}

func (m *mockUserRoleRepo) GetAllUserRoles(ctx context.Context, userID string) ([]string, error) {
	if role, ok := m.roles[userID]; ok {
		return []string{role.Role}, nil
	}
	return []string{}, nil
}

func (m *mockUserRoleRepo) GetAdminUsers(ctx context.Context) ([]models.UserRole, error) {
	var result []models.UserRole
	for userID := range m.admins {
		if role, ok := m.roles[userID]; ok {
			result = append(result, *role)
		}
	}
	return result, nil
}

func (m *mockUserRoleRepo) SetAdmin(userID string, isAdmin bool) {
	if isAdmin {
		m.roles[userID] = &models.UserRole{UserID: userID, Role: "admin"}
		m.admins[userID] = true
	} else {
		delete(m.admins, userID)
		m.roles[userID] = &models.UserRole{UserID: userID, Role: "user"}
	}
}

// mockRedirectLinkRepo is a mock implementation of RedirectLinkRepository.
type mockRedirectLinkRepo struct {
	links    map[string]*models.RedirectLink
	bySlug   map[string]*models.RedirectLink
	byUserID map[string][]models.RedirectLink
}

func newMockRedirectLinkRepo() *mockRedirectLinkRepo {
	return &mockRedirectLinkRepo{
		links:    make(map[string]*models.RedirectLink),
		bySlug:   make(map[string]*models.RedirectLink),
		byUserID: make(map[string][]models.RedirectLink),
	}
}

func (m *mockRedirectLinkRepo) GetByID(ctx context.Context, linkID string) (*models.RedirectLink, error) {
	link, ok := m.links[linkID]
	if !ok {
		return nil, nil
	}
	return link, nil
}

func (m *mockRedirectLinkRepo) GetBySlug(ctx context.Context, slug string) (*models.RedirectLink, error) {
	link, ok := m.bySlug[slug]
	if !ok {
		return nil, nil
	}
	return link, nil
}

func (m *mockRedirectLinkRepo) GetByUserID(ctx context.Context, userID string) ([]models.RedirectLink, error) {
	return m.byUserID[userID], nil
}

func (m *mockRedirectLinkRepo) CheckSlugExists(ctx context.Context, slug string, excludeID string) (bool, error) {
	link, exists := m.bySlug[slug]
	if !exists {
		return false, nil
	}
	if excludeID != "" && link.ID == excludeID {
		return false, nil
	}
	return true, nil
}

func (m *mockRedirectLinkRepo) Create(ctx context.Context, link *models.RedirectLink) (*models.RedirectLink, error) {
	link.ID = fmt.Sprintf("link-%d", len(m.links)+1)
	link.CreatedAt = time.Now()
	link.UpdatedAt = time.Now()
	m.links[link.ID] = link
	m.bySlug[link.Slug] = link
	m.byUserID[link.UserID] = append(m.byUserID[link.UserID], *link)
	return link, nil
}

func (m *mockRedirectLinkRepo) Update(ctx context.Context, linkID string, updates map[string]interface{}) error {
	link, ok := m.links[linkID]
	if !ok {
		return fmt.Errorf("link not found")
	}
	link.UpdatedAt = time.Now()
	return nil
}

func (m *mockRedirectLinkRepo) Delete(ctx context.Context, linkID, userID string) error {
	link, ok := m.links[linkID]
	if !ok || link.UserID != userID {
		return fmt.Errorf("link not found")
	}
	delete(m.links, linkID)
	delete(m.bySlug, link.Slug)
	return nil
}

func (m *mockRedirectLinkRepo) IncrementClicks(ctx context.Context, linkID, groupID string) error {
	link, ok := m.links[linkID]
	if !ok {
		return nil
	}
	link.ClickCount++
	if link.GroupClicks == nil {
		link.GroupClicks = make(map[string]int)
	}
	link.GroupClicks[groupID]++
	return nil
}

func (m *mockRedirectLinkRepo) AddLink(link *models.RedirectLink) {
	m.links[link.ID] = link
	m.bySlug[link.Slug] = link
	m.byUserID[link.UserID] = append(m.byUserID[link.UserID], *link)
}

// mockSystemEventRepo is a mock implementation of SystemEventRepository.
type mockSystemEventRepo struct {
	events []models.SystemEvent
}

func newMockSystemEventRepo() *mockSystemEventRepo {
	return &mockSystemEventRepo{
		events: make([]models.SystemEvent, 0),
	}
}

func (m *mockSystemEventRepo) Create(ctx context.Context, eventType string, payload map[string]interface{}) error {
	m.events = append(m.events, models.SystemEvent{
		EventType: eventType,
		Payload:   payload,
	})
	return nil
}

func (m *mockSystemEventRepo) GetRecent(ctx context.Context, limit int) ([]models.SystemEvent, error) {
	if limit <= 0 || limit > len(m.events) {
		return m.events, nil
	}
	return m.events[:limit], nil
}
