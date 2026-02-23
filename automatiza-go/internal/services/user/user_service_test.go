package user

import (
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap/zaptest"
)

// --- Mock ProfileRepository ---

type mockProfileRepo struct {
	EnsureUserProfileFn func(ctx context.Context, userID, email string) (*models.Profile, error)
	GetProfileFn        func(ctx context.Context, userID string) (*models.Profile, error)
	UpdateProfileFn     func(ctx context.Context, userID string, updates map[string]interface{}) error
}

func (m *mockProfileRepo) EnsureUserProfile(ctx context.Context, userID, email string) (*models.Profile, error) {
	if m.EnsureUserProfileFn != nil {
		return m.EnsureUserProfileFn(ctx, userID, email)
	}
	return nil, nil
}

func (m *mockProfileRepo) GetProfile(ctx context.Context, userID string) (*models.Profile, error) {
	if m.GetProfileFn != nil {
		return m.GetProfileFn(ctx, userID)
	}
	return nil, nil
}

func (m *mockProfileRepo) GetProfileByEmail(_ context.Context, _ string) (*models.Profile, error) {
	return nil, nil
}

func (m *mockProfileRepo) GetProfileByEmailWithBilling(_ context.Context, _ string) (*models.Profile, *models.ProfileBillingFields, error) {
	return nil, nil, nil
}

func (m *mockProfileRepo) CreateProfile(_ context.Context, _ *models.Profile) (*models.Profile, error) {
	return nil, nil
}

func (m *mockProfileRepo) CreateUserCredentials(_ context.Context, _, _ string) error {
	return nil
}

func (m *mockProfileRepo) UpdateProfile(ctx context.Context, userID string, updates map[string]interface{}) error {
	if m.UpdateProfileFn != nil {
		return m.UpdateProfileFn(ctx, userID, updates)
	}
	return nil
}

func (m *mockProfileRepo) UpdatePassword(_ context.Context, _, _ string) error {
	return nil
}

func (m *mockProfileRepo) UpdateBillingStatus(_ context.Context, _ string, _ *models.ProfileBillingFields) error {
	return nil
}

func (m *mockProfileRepo) PauseUserAutomations(_ context.Context, _ string) error {
	return nil
}

func (m *mockProfileRepo) ListProfiles(_ context.Context, _ string, _ int) ([]*models.Profile, string, error) {
	return nil, "", nil
}

func (m *mockProfileRepo) SearchProfiles(_ context.Context, _, _ string, _ int) ([]*models.Profile, string, error) {
	return nil, "", nil
}

func (m *mockProfileRepo) GetMaxAutomations(_ context.Context, _ string) (int, error) {
	return 0, nil
}

func (m *mockProfileRepo) CountUserAutomations(_ context.Context, _ string) (int, error) {
	return 0, nil
}

func (m *mockProfileRepo) GetProfileStats(_ context.Context) (*repository.ProfileStats, error) {
	return nil, nil
}

// Compile-time check
var _ repository.ProfileRepository = (*mockProfileRepo)(nil)

// --- Mock Email Service ---

type mockEmailService struct{}

func (m *mockEmailService) SendWelcomeEmail(_ context.Context, _, _, _ string) error {
	return nil
}

func (m *mockEmailService) SendPasswordResetEmail(_ context.Context, _, _, _ string) error {
	return nil
}

func (m *mockEmailService) SendPlanUpgradeEmail(_ context.Context, _, _, _ string) error {
	return nil
}

func (m *mockEmailService) SendSuspensionWarningEmail(_ context.Context, _, _ string, _ int) error {
	return nil
}

func (m *mockEmailService) SendCustomEmail(_ context.Context, _, _, _ string) error {
	return nil
}

// --- Tests ---

func TestNewUserService(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	svc := NewUserService(&mockProfileRepo{}, &mockEmailService{}, logger)
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
}

func TestEnsureUserProfile(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		repoFn func(ctx context.Context, userID, email string) (*models.Profile, error)
		wantErr bool
	}{
		{
			name: "success",
			repoFn: func(_ context.Context, userID, email string) (*models.Profile, error) {
				return &models.Profile{ID: userID, Email: email}, nil
			},
			wantErr: false,
		},
		{
			name: "repo error",
			repoFn: func(_ context.Context, _, _ string) (*models.Profile, error) {
				return nil, errors.New("db error")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			logger := zaptest.NewLogger(t)
			repo := &mockProfileRepo{EnsureUserProfileFn: tt.repoFn}
			svc := NewUserService(repo, &mockEmailService{}, logger)

			profile, err := svc.EnsureUserProfile(context.Background(), "user-1", "a@b.com")
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if profile == nil {
				t.Fatal("expected non-nil profile")
			}
			if profile.ID != "user-1" {
				t.Errorf("expected ID user-1, got %s", profile.ID)
			}
		})
	}
}

func TestGetProfile(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		repoFn func(ctx context.Context, userID string) (*models.Profile, error)
		wantErr bool
		wantNil bool
	}{
		{
			name: "success",
			repoFn: func(_ context.Context, userID string) (*models.Profile, error) {
				return &models.Profile{ID: userID, Email: "test@test.com"}, nil
			},
			wantErr: false,
		},
		{
			name: "repo error",
			repoFn: func(_ context.Context, _ string) (*models.Profile, error) {
				return nil, errors.New("not found")
			},
			wantErr: true,
		},
		{
			name: "nil profile no error",
			repoFn: func(_ context.Context, _ string) (*models.Profile, error) {
				return nil, nil
			},
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			logger := zaptest.NewLogger(t)
			repo := &mockProfileRepo{GetProfileFn: tt.repoFn}
			svc := NewUserService(repo, &mockEmailService{}, logger)

			profile, err := svc.GetProfile(context.Background(), "user-1")
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantNil && profile != nil {
				t.Fatal("expected nil profile")
			}
		})
	}
}

func TestUpdateProfile(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		repoFn func(ctx context.Context, userID string, updates map[string]interface{}) error
		wantErr bool
	}{
		{
			name: "success with updated_at set",
			repoFn: func(_ context.Context, userID string, updates map[string]interface{}) error {
				if userID != "user-1" {
					return errors.New("wrong userID")
				}
				if _, ok := updates["updated_at"]; !ok {
					return errors.New("updated_at not set")
				}
				// Verify updated_at is a valid RFC3339 timestamp
				ts, ok := updates["updated_at"].(string)
				if !ok {
					return errors.New("updated_at is not a string")
				}
				if _, err := time.Parse(time.RFC3339, ts); err != nil {
					return errors.New("updated_at is not RFC3339")
				}
				return nil
			},
			wantErr: false,
		},
		{
			name: "repo error",
			repoFn: func(_ context.Context, _ string, _ map[string]interface{}) error {
				return errors.New("update failed")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			logger := zaptest.NewLogger(t)
			repo := &mockProfileRepo{UpdateProfileFn: tt.repoFn}
			svc := NewUserService(repo, &mockEmailService{}, logger)

			updates := map[string]interface{}{"phone_number": "123"}
			err := svc.UpdateProfile(context.Background(), "user-1", updates)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestUpdateProfile_SetsUpdatedAt(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)

	var capturedUpdates map[string]interface{}
	repo := &mockProfileRepo{
		UpdateProfileFn: func(_ context.Context, _ string, updates map[string]interface{}) error {
			capturedUpdates = updates
			return nil
		},
	}
	svc := NewUserService(repo, &mockEmailService{}, logger)

	before := time.Now().UTC().Add(-1 * time.Second)
	updates := map[string]interface{}{"plan": "pro"}
	if err := svc.UpdateProfile(context.Background(), "u1", updates); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rawTS, ok := capturedUpdates["updated_at"]
	if !ok {
		t.Fatal("updated_at not present in updates")
	}
	ts, err := time.Parse(time.RFC3339, rawTS.(string))
	if err != nil {
		t.Fatalf("updated_at is not RFC3339: %v", err)
	}
	if ts.Before(before) {
		t.Error("updated_at is too early")
	}
}

func TestCreateUserFromWebhook(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		data     *WebhookUserData
		plan     *string
		wantErr  bool
		errMsg   string
	}{
		{
			name:    "empty email returns error",
			data:    &WebhookUserData{Email: ""},
			plan:    nil,
			wantErr: true,
			errMsg:  "email is required",
		},
		{
			name: "nil plan defaults to free (success returns nil,nil)",
			data: &WebhookUserData{Email: "user@test.com", Name: "Test"},
			plan: nil,
		},
		{
			name: "empty plan string defaults to free",
			data: &WebhookUserData{Email: "user@test.com"},
			plan: strPtr(""),
		},
		{
			name: "non-empty plan used",
			data: &WebhookUserData{Email: "user@test.com"},
			plan: strPtr("pro"),
		},
		{
			name: "success returns nil profile and nil error",
			data: &WebhookUserData{Email: "ok@test.com"},
			plan: strPtr("elite"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			logger := zaptest.NewLogger(t)
			svc := NewUserService(&mockProfileRepo{}, &mockEmailService{}, logger)

			profile, err := svc.CreateUserFromWebhook(context.Background(), tt.data, tt.plan)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errMsg != "" && err.Error() != tt.errMsg {
					t.Errorf("expected error %q, got %q", tt.errMsg, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if profile != nil {
				t.Fatal("expected nil profile on success")
			}
		})
	}
}

func TestUpdateUserPlan(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		plan    string
		repoFn func(ctx context.Context, userID string, updates map[string]interface{}) error
		wantErr bool
		errMsg  string
	}{
		{
			name:    "invalid plan",
			plan:    "premium",
			wantErr: true,
			errMsg:  "invalid plan: premium",
		},
		{
			name: "valid plan free",
			plan: "free",
			repoFn: func(_ context.Context, _ string, updates map[string]interface{}) error {
				if updates["plan"] != "free" {
					return errors.New("wrong plan")
				}
				return nil
			},
		},
		{
			name: "valid plan pro",
			plan: "pro",
			repoFn: func(_ context.Context, _ string, updates map[string]interface{}) error {
				if updates["plan"] != "pro" {
					return errors.New("wrong plan")
				}
				return nil
			},
		},
		{
			name: "valid plan elite",
			plan: "elite",
			repoFn: func(_ context.Context, _ string, updates map[string]interface{}) error {
				if updates["plan"] != "elite" {
					return errors.New("wrong plan")
				}
				return nil
			},
		},
		{
			name: "repo error wrapping",
			plan: "pro",
			repoFn: func(_ context.Context, _ string, _ map[string]interface{}) error {
				return errors.New("db failure")
			},
			wantErr: true,
			errMsg:  "failed to update plan: db failure",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			logger := zaptest.NewLogger(t)
			repo := &mockProfileRepo{UpdateProfileFn: tt.repoFn}
			svc := NewUserService(repo, &mockEmailService{}, logger)

			err := svc.UpdateUserPlan(context.Background(), "user-1", tt.plan)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errMsg != "" && err.Error() != tt.errMsg {
					t.Errorf("expected error %q, got %q", tt.errMsg, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestResetUserPassword(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		repoFn       func(ctx context.Context, userID string) (*models.Profile, error)
		wantSuccess  bool
		wantPassword bool
	}{
		{
			name: "repo error returns not-found result",
			repoFn: func(_ context.Context, _ string) (*models.Profile, error) {
				return nil, errors.New("db error")
			},
			wantSuccess:  false,
			wantPassword: false,
		},
		{
			name: "nil profile returns not-found result",
			repoFn: func(_ context.Context, _ string) (*models.Profile, error) {
				return nil, nil
			},
			wantSuccess:  false,
			wantPassword: false,
		},
		{
			name: "success returns password",
			repoFn: func(_ context.Context, _ string) (*models.Profile, error) {
				return &models.Profile{ID: "user-1", Email: "test@test.com"}, nil
			},
			wantSuccess:  true,
			wantPassword: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			logger := zaptest.NewLogger(t)
			repo := &mockProfileRepo{GetProfileFn: tt.repoFn}
			svc := NewUserService(repo, &mockEmailService{}, logger)

			result, err := svc.ResetUserPassword(context.Background(), "user-1", "admin-1", "admin@test.com")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result == nil {
				t.Fatal("expected non-nil result")
			}
			if result.Success != tt.wantSuccess {
				t.Errorf("expected success=%v, got %v", tt.wantSuccess, result.Success)
			}
			if tt.wantPassword {
				if result.TemporaryPassword == "" {
					t.Error("expected non-empty temporary password")
				}
				if len(result.TemporaryPassword) != 12 {
					t.Errorf("expected 12-char password, got %d", len(result.TemporaryPassword))
				}
			} else {
				if result.TemporaryPassword != "" {
					t.Error("expected empty temporary password for failure")
				}
			}
			if !tt.wantSuccess && result.Message == "" {
				t.Error("expected non-empty message for failure")
			}
		})
	}
}

func TestSuspendUser(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		repoFn func(ctx context.Context, userID string, updates map[string]interface{}) error
		wantErr bool
	}{
		{
			name: "success",
			repoFn: func(_ context.Context, userID string, updates map[string]interface{}) error {
				if updates["billing_status"] != "suspended" {
					return errors.New("expected billing_status=suspended")
				}
				if updates["suspend_reason"] != "payment failed" {
					return errors.New("expected suspend_reason")
				}
				if _, ok := updates["updated_at"]; !ok {
					return errors.New("expected updated_at")
				}
				return nil
			},
		},
		{
			name: "repo error",
			repoFn: func(_ context.Context, _ string, _ map[string]interface{}) error {
				return errors.New("db error")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			logger := zaptest.NewLogger(t)
			repo := &mockProfileRepo{UpdateProfileFn: tt.repoFn}
			svc := NewUserService(repo, &mockEmailService{}, logger)

			err := svc.SuspendUser(context.Background(), "user-1", "payment failed")
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), "failed to suspend user") {
					t.Errorf("expected wrapped error, got %q", err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestReactivateUser(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		repoFn func(ctx context.Context, userID string, updates map[string]interface{}) error
		wantErr bool
	}{
		{
			name: "success",
			repoFn: func(_ context.Context, userID string, updates map[string]interface{}) error {
				if updates["billing_status"] != "active" {
					return errors.New("expected billing_status=active")
				}
				if updates["suspend_reason"] != nil {
					return errors.New("expected suspend_reason=nil")
				}
				if _, ok := updates["updated_at"]; !ok {
					return errors.New("expected updated_at")
				}
				return nil
			},
		},
		{
			name: "repo error",
			repoFn: func(_ context.Context, _ string, _ map[string]interface{}) error {
				return errors.New("db error")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			logger := zaptest.NewLogger(t)
			repo := &mockProfileRepo{UpdateProfileFn: tt.repoFn}
			svc := NewUserService(repo, &mockEmailService{}, logger)

			err := svc.ReactivateUser(context.Background(), "user-1")
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), "failed to reactivate user") {
					t.Errorf("expected wrapped error, got %q", err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestGenerateTemporaryPassword(t *testing.T) {
	t.Parallel()
	const validCharset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$"

	// Run multiple times to verify randomness and consistency
	for i := 0; i < 50; i++ {
		pw := generateTemporaryPassword()
		if len(pw) != 12 {
			t.Fatalf("expected 12 chars, got %d: %q", len(pw), pw)
		}
		for _, c := range pw {
			if !strings.ContainsRune(validCharset, c) {
				t.Fatalf("invalid character %q in password %q", string(c), pw)
			}
		}
	}

	// Verify uniqueness (extremely unlikely to collide with 12 random chars)
	pw1 := generateTemporaryPassword()
	pw2 := generateTemporaryPassword()
	if pw1 == pw2 {
		t.Error("two consecutive passwords should differ (probabilistic check)")
	}
}

// --- Helpers ---

func strPtr(s string) *string {
	return &s
}
