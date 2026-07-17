package kiwify

import (
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"
	"automatiza-go/internal/services/email"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
)

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

// mockProfileRepo implements repository.ProfileRepository for testing.
type mockProfileRepo struct {
	// Return controls
	profile    *models.Profile
	billing    *models.ProfileBillingFields
	getErr     error
	createErr  error
	createCred error
	updateErr  error
	billingErr error
	pauseErr   error
	resumeErr  error
	pauseCount int
	resumeN    int

	// Call captures
	createdProfile *models.Profile
	updateID       string
	updates        map[string]interface{}
	billingID      string
	billingFields  *models.ProfileBillingFields
	pausedUserID   string
	resumedUserID  string
	credsUserID    string
	credsHash      string
	calls          []string
}

func (m *mockProfileRepo) GetProfileByEmailWithBilling(_ context.Context, _ string) (*models.Profile, *models.ProfileBillingFields, error) {
	m.calls = append(m.calls, "GetProfileByEmailWithBilling")
	return m.profile, m.billing, m.getErr
}

func (m *mockProfileRepo) CreateProfile(_ context.Context, p *models.Profile) (*models.Profile, error) {
	m.calls = append(m.calls, "CreateProfile")
	m.createdProfile = p
	if m.createErr != nil {
		return nil, m.createErr
	}
	return p, nil
}

func (m *mockProfileRepo) CreateUserCredentials(_ context.Context, userID, hash string) error {
	m.calls = append(m.calls, "CreateUserCredentials")
	m.credsUserID = userID
	m.credsHash = hash
	return m.createCred
}

func (m *mockProfileRepo) UpdateProfile(_ context.Context, id string, u map[string]interface{}) error {
	m.calls = append(m.calls, "UpdateProfile")
	m.updateID = id
	m.updates = u
	return m.updateErr
}

func (m *mockProfileRepo) UpdateBillingStatus(_ context.Context, id string, b *models.ProfileBillingFields) error {
	m.calls = append(m.calls, "UpdateBillingStatus")
	m.billingID = id
	m.billingFields = b
	return m.billingErr
}

func (m *mockProfileRepo) PauseUserAutomations(_ context.Context, userID string) (int, error) {
	m.calls = append(m.calls, "PauseUserAutomations")
	m.pausedUserID = userID
	return m.pauseCount, m.pauseErr
}

func (m *mockProfileRepo) ResumeUserAutomations(_ context.Context, userID string) (int, error) {
	m.calls = append(m.calls, "ResumeUserAutomations")
	m.resumedUserID = userID
	return m.resumeN, m.resumeErr
}

// Unused methods — satisfy the interface.
func (m *mockProfileRepo) EnsureUserProfile(context.Context, string, string) (*models.Profile, error) {
	panic("not implemented")
}
func (m *mockProfileRepo) GetProfile(context.Context, string) (*models.Profile, error) {
	panic("not implemented")
}
func (m *mockProfileRepo) GetProfileByEmail(context.Context, string) (*models.Profile, error) {
	panic("not implemented")
}
func (m *mockProfileRepo) UpdatePassword(context.Context, string, string) error {
	panic("not implemented")
}
func (m *mockProfileRepo) ListProfiles(context.Context, string, int) ([]*models.Profile, string, error) {
	panic("not implemented")
}
func (m *mockProfileRepo) SearchProfiles(context.Context, string, string, int) ([]*models.Profile, string, error) {
	panic("not implemented")
}
func (m *mockProfileRepo) GetMaxAutomations(context.Context, string) (int, error) {
	panic("not implemented")
}
func (m *mockProfileRepo) CountUserAutomations(context.Context, string) (int, error) {
	panic("not implemented")
}
func (m *mockProfileRepo) GetProfileStats(context.Context) (*repository.ProfileStats, error) {
	panic("not implemented")
}

// mockKiwifyEventRepo implements repository.KiwifyEventRepository for testing.
type mockKiwifyEventRepo struct {
	claimed     bool   // what Claim returns as `claimed`
	priorStatus string // status returned when not claimed
	claimErr    error

	claims   []string
	finished []string
	released []string
	audits   []map[string]interface{}
}

func (m *mockKiwifyEventRepo) Claim(_ context.Context, key, _, _ string) (bool, string, error) {
	m.claims = append(m.claims, key)
	return m.claimed, m.priorStatus, m.claimErr
}

func (m *mockKiwifyEventRepo) Finish(_ context.Context, key, _, _ string) error {
	m.finished = append(m.finished, key)
	return nil
}

func (m *mockKiwifyEventRepo) Release(_ context.Context, key string) error {
	m.released = append(m.released, key)
	return nil
}

func (m *mockKiwifyEventRepo) RecordBillingAudit(_ context.Context, payload map[string]interface{}) error {
	m.audits = append(m.audits, payload)
	return nil
}

// mockPlanRepo implements repository.PlanRepository for testing.
type mockPlanRepo struct {
	plan   *models.Plan
	getErr error

	searchedID string
}

func (m *mockPlanRepo) GetByKiwifyProductID(_ context.Context, productID string) (*models.Plan, error) {
	m.searchedID = productID
	return m.plan, m.getErr
}

func (m *mockPlanRepo) GetBySlug(context.Context, string) (*models.Plan, error) {
	panic("not implemented")
}
func (m *mockPlanRepo) GetByID(context.Context, string) (*models.Plan, error) {
	panic("not implemented")
}
func (m *mockPlanRepo) GetActivePlans(context.Context) ([]models.Plan, error) {
	panic("not implemented")
}
func (m *mockPlanRepo) GetAllPlans(context.Context) ([]models.Plan, error) {
	panic("not implemented")
}
func (m *mockPlanRepo) Create(context.Context, *models.Plan) (*models.Plan, error) {
	panic("not implemented")
}
func (m *mockPlanRepo) Update(context.Context, string, map[string]interface{}) error {
	panic("not implemented")
}
func (m *mockPlanRepo) Delete(context.Context, string) error { panic("not implemented") }

// mockSettingsRepo implements repository.SettingsRepository for testing.
type mockSettingsRepo struct {
	settings models.SettingsMap
	getErr   error
}

func (m *mockSettingsRepo) Get(_ context.Context, _ ...string) (models.SettingsMap, error) {
	return m.settings, m.getErr
}

func (m *mockSettingsRepo) GetAll(context.Context) (models.SettingsMap, error) {
	panic("not implemented")
}
func (m *mockSettingsRepo) Update(context.Context, string, interface{}) error {
	panic("not implemented")
}
func (m *mockSettingsRepo) GetDispatchText(context.Context, string) (*models.DispatchText, error) {
	panic("not implemented")
}
func (m *mockSettingsRepo) GetDispatchTextsByUser(context.Context, string) ([]models.DispatchText, error) {
	panic("not implemented")
}
func (m *mockSettingsRepo) GetActiveDispatchTextByID(context.Context, string, string) (*models.DispatchText, error) {
	panic("not implemented")
}
func (m *mockSettingsRepo) GetActiveDispatchTextsByUser(context.Context, string) ([]models.DispatchText, error) {
	panic("not implemented")
}
func (m *mockSettingsRepo) CreateDispatchText(context.Context, string, *models.DispatchText) (*models.DispatchText, error) {
	panic("not implemented")
}
func (m *mockSettingsRepo) UpdateDispatchText(context.Context, string, map[string]interface{}) error {
	panic("not implemented")
}
func (m *mockSettingsRepo) DeleteDispatchText(context.Context, string, string) error {
	panic("not implemented")
}

// mockUserRoleRepo implements repository.UserRoleRepository for testing.
type mockUserRoleRepo struct {
	setRoleErr error

	lastUserID string
	lastRole   string
}

func (m *mockUserRoleRepo) SetUserRole(_ context.Context, userID, role string) error {
	m.lastUserID = userID
	m.lastRole = role
	return m.setRoleErr
}

func (m *mockUserRoleRepo) GetUserRole(context.Context, string) (*models.UserRole, error) {
	panic("not implemented")
}
func (m *mockUserRoleRepo) RemoveUserRole(context.Context, string, string) error {
	panic("not implemented")
}
func (m *mockUserRoleRepo) IsAdmin(context.Context, string) (bool, error) {
	panic("not implemented")
}
func (m *mockUserRoleRepo) GetAllUserRoles(context.Context, string) ([]string, error) {
	panic("not implemented")
}
func (m *mockUserRoleRepo) GetAdminUsers(context.Context) ([]models.UserRole, error) {
	panic("not implemented")
}

// mockEmailService implements email.Service for testing.
type mockEmailService struct {
	sendErr error

	lastTo       string
	lastName     string
	lastPassword string
	sentCalled   bool
}

func (m *mockEmailService) SendWelcomeEmail(_ context.Context, to, name, password string) error {
	m.sentCalled = true
	m.lastTo = to
	m.lastName = name
	m.lastPassword = password
	return m.sendErr
}

func (m *mockEmailService) SendPasswordResetEmail(context.Context, string, string, string) error {
	panic("not implemented")
}
func (m *mockEmailService) SendPlanUpgradeEmail(context.Context, string, string, string) error {
	panic("not implemented")
}
func (m *mockEmailService) SendSuspensionWarningEmail(context.Context, string, string, int) error {
	panic("not implemented")
}
func (m *mockEmailService) SendCustomEmail(context.Context, string, string, string) error {
	panic("not implemented")
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func newTestService(
	pr *mockProfileRepo,
	plr *mockPlanRepo,
	sr *mockSettingsRepo,
	ur repository.UserRoleRepository,
	es email.Service,
	cfg *Config,
) *kiwifyService {
	logger := zap.NewNop()
	// eventRepo is nil here: the service guards nil (skips idempotency/audit), so
	// the existing behavioral tests are unaffected. Idempotency/audit are covered
	// by dedicated tests that inject a mockKiwifyEventRepo.
	svc := NewKiwifyService(pr, plr, sr, ur, nil, es, cfg, logger)
	return svc.(*kiwifyService)
}

func makePayload(customerEmail, productName, event string) *models.KiwifyWebhookPayload {
	return &models.KiwifyWebhookPayload{
		Customer: &models.KiwifyCustomer{
			Email: customerEmail,
		},
		Product: &models.KiwifyProduct{
			ProductName: productName,
		},
		WebhookEventType: event,
	}
}

func makeProfile(id, em, plan string) *models.Profile {
	return &models.Profile{
		ID:    id,
		Email: em,
		Plan:  plan,
	}
}

func timePtr(t time.Time) *time.Time { return &t }

// ---------------------------------------------------------------------------
// TestNewKiwifyService
// ---------------------------------------------------------------------------

func TestNewKiwifyService(t *testing.T) {
	t.Run("nil config uses defaults", func(t *testing.T) {
		svc := NewKiwifyService(
			&mockProfileRepo{}, &mockPlanRepo{}, &mockSettingsRepo{},
			nil, nil, nil, nil, zap.NewNop(),
		)
		if svc == nil {
			t.Fatal("expected non-nil service")
		}
	})

	t.Run("config with token", func(t *testing.T) {
		cfg := &Config{WebhookToken: "tok123"}
		svc := NewKiwifyService(
			&mockProfileRepo{}, &mockPlanRepo{}, &mockSettingsRepo{},
			nil, nil, nil, cfg, zap.NewNop(),
		).(*kiwifyService)
		if svc.config.WebhookToken != "tok123" {
			t.Errorf("expected token tok123, got %s", svc.config.WebhookToken)
		}
	})
}

// ---------------------------------------------------------------------------
// TestStrPtr / TestStrPtrIfNotEmpty
// ---------------------------------------------------------------------------

func TestStrPtr(t *testing.T) {
	t.Run("non-empty", func(t *testing.T) {
		p := strPtr("hello")
		if p == nil || *p != "hello" {
			t.Errorf("expected *string == hello, got %v", p)
		}
	})
	t.Run("empty", func(t *testing.T) {
		p := strPtr("")
		if p == nil || *p != "" {
			t.Errorf("expected *string == empty, got %v", p)
		}
	})
}

func TestStrPtrIfNotEmpty(t *testing.T) {
	t.Run("non-empty", func(t *testing.T) {
		p := strPtrIfNotEmpty("hello")
		if p == nil || *p != "hello" {
			t.Errorf("expected *string == hello, got %v", p)
		}
	})
	t.Run("empty returns nil", func(t *testing.T) {
		p := strPtrIfNotEmpty("")
		if p != nil {
			t.Errorf("expected nil, got %v", *p)
		}
	})
}

// ---------------------------------------------------------------------------
// TestParseFlexibleDate
// ---------------------------------------------------------------------------

func TestParseFlexibleDate(t *testing.T) {
	svc := newTestService(&mockProfileRepo{}, &mockPlanRepo{}, &mockSettingsRepo{}, nil, nil, nil)

	tests := []struct {
		name    string
		input   interface{}
		wantNil bool
		check   func(t *testing.T, result *time.Time)
	}{
		{name: "nil", input: nil, wantNil: true},
		{
			name:  "float64 unix seconds",
			input: float64(1700000000),
			check: func(t *testing.T, r *time.Time) {
				if r.Unix() != 1700000000 {
					t.Errorf("expected unix 1700000000, got %d", r.Unix())
				}
			},
		},
		{
			name:  "float64 unix ms",
			input: float64(1700000000000),
			check: func(t *testing.T, r *time.Time) {
				if r.UnixMilli() != 1700000000000 {
					t.Errorf("expected ms 1700000000000, got %d", r.UnixMilli())
				}
			},
		},
		{
			name:  "int64 unix seconds",
			input: int64(1700000000),
			check: func(t *testing.T, r *time.Time) {
				if r.Unix() != 1700000000 {
					t.Errorf("expected unix 1700000000, got %d", r.Unix())
				}
			},
		},
		{
			name:  "int64 unix ms",
			input: int64(1700000000000),
			check: func(t *testing.T, r *time.Time) {
				if r.UnixMilli() != 1700000000000 {
					t.Errorf("expected ms 1700000000000, got %d", r.UnixMilli())
				}
			},
		},
		{name: "empty string", input: "", wantNil: true},
		{name: "whitespace string", input: "   ", wantNil: true},
		{
			name:  "string numeric seconds",
			input: "1700000000",
			check: func(t *testing.T, r *time.Time) {
				if r.Unix() != 1700000000 {
					t.Errorf("expected unix 1700000000, got %d", r.Unix())
				}
			},
		},
		{
			name:  "string numeric ms",
			input: "1700000000000",
			check: func(t *testing.T, r *time.Time) {
				if r.UnixMilli() != 1700000000000 {
					t.Errorf("expected ms 1700000000000, got %d", r.UnixMilli())
				}
			},
		},
		{name: "string numeric overflow", input: "99999999999999999999999999", wantNil: true},
		{
			name:  "string RFC3339",
			input: "2024-01-15T10:30:00Z",
			check: func(t *testing.T, r *time.Time) {
				if r.Year() != 2024 || r.Month() != 1 || r.Day() != 15 {
					t.Errorf("unexpected date: %v", r)
				}
			},
		},
		{
			name:  "string RFC3339Nano",
			input: "2024-01-15T10:30:00.123456789Z",
			check: func(t *testing.T, r *time.Time) {
				if r.Year() != 2024 {
					t.Errorf("unexpected year: %d", r.Year())
				}
			},
		},
		{
			name:  "string ISO without Z",
			input: "2024-01-15T10:30:00",
			check: func(t *testing.T, r *time.Time) {
				if r.Hour() != 10 || r.Minute() != 30 {
					t.Errorf("unexpected time: %v", r)
				}
			},
		},
		{
			name:  "string date only",
			input: "2024-01-15",
			check: func(t *testing.T, r *time.Time) {
				if r.Year() != 2024 || r.Month() != 1 || r.Day() != 15 {
					t.Errorf("unexpected date: %v", r)
				}
			},
		},
		{
			name:  "string datetime with space",
			input: "2024-01-15 10:30:00",
			check: func(t *testing.T, r *time.Time) {
				if r.Hour() != 10 {
					t.Errorf("unexpected hour: %d", r.Hour())
				}
			},
		},
		{name: "string unparseable", input: "not-a-date", wantNil: true},
		{name: "bool (unknown type)", input: true, wantNil: true},
		{
			name:  "float64 zero",
			input: float64(0),
			check: func(t *testing.T, r *time.Time) {
				if r.UnixMilli() != 0 {
					t.Errorf("expected epoch, got %v", r)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := svc.parseFlexibleDate(tt.input)
			if tt.wantNil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
				return
			}
			if result == nil {
				t.Fatal("expected non-nil result")
			}
			if tt.check != nil {
				tt.check(t, result)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestExtractNextPaymentAt
// ---------------------------------------------------------------------------

func TestExtractNextPaymentAt(t *testing.T) {
	svc := newTestService(&mockProfileRepo{}, &mockPlanRepo{}, &mockSettingsRepo{}, nil, nil, nil)

	t.Run("first candidate wins", func(t *testing.T) {
		data := &models.KiwifyOrderData{
			NextPaymentAt: "2024-06-01",
		}
		r := svc.extractNextPaymentAt(data)
		if r == nil || r.Day() != 1 {
			t.Errorf("expected day 1, got %v", r)
		}
	})

	t.Run("fallback to later candidate", func(t *testing.T) {
		data := &models.KiwifyOrderData{
			NextBillingDate: "2024-07-15",
		}
		r := svc.extractNextPaymentAt(data)
		if r == nil || r.Day() != 15 {
			t.Errorf("expected day 15, got %v", r)
		}
	})

	t.Run("subscription field used", func(t *testing.T) {
		data := &models.KiwifyOrderData{
			Subscription: &models.KiwifySubscription{
				NextPaymentAt: "2024-08-20",
			},
		}
		r := svc.extractNextPaymentAt(data)
		if r == nil || r.Day() != 20 {
			t.Errorf("expected day 20, got %v", r)
		}
	})

	t.Run("subscription future charges used", func(t *testing.T) {
		data := &models.KiwifyOrderData{
			Subscription: &models.KiwifySubscription{
				Charges: &models.KiwifyCharges{
					Future: []models.KiwifyFutureCharge{
						{ChargeDate: "2024-09-10"},
					},
				},
			},
		}
		r := svc.extractNextPaymentAt(data)
		if r == nil || r.Day() != 10 {
			t.Errorf("expected day 10, got %v", r)
		}
	})

	t.Run("all nil", func(t *testing.T) {
		data := &models.KiwifyOrderData{}
		r := svc.extractNextPaymentAt(data)
		if r != nil {
			t.Errorf("expected nil, got %v", r)
		}
	})

	t.Run("subscription nil uses direct fields only", func(t *testing.T) {
		data := &models.KiwifyOrderData{
			NextChargeDate: "2024-12-25",
		}
		r := svc.extractNextPaymentAt(data)
		if r == nil || r.Day() != 25 {
			t.Errorf("expected day 25, got %v", r)
		}
	})
}

// ---------------------------------------------------------------------------
// TestExtractAccessUntil
// ---------------------------------------------------------------------------

func TestExtractAccessUntil(t *testing.T) {
	svc := newTestService(&mockProfileRepo{}, &mockPlanRepo{}, &mockSettingsRepo{}, nil, nil, nil)

	t.Run("AccessUntil set", func(t *testing.T) {
		data := &models.KiwifyOrderData{AccessUntil: "2024-12-31"}
		r := svc.extractAccessUntil(data)
		if r == nil || r.Day() != 31 {
			t.Errorf("expected day 31, got %v", r)
		}
	})

	t.Run("AccessUntilAt set", func(t *testing.T) {
		data := &models.KiwifyOrderData{AccessUntilAt: "2025-01-15"}
		r := svc.extractAccessUntil(data)
		if r == nil || r.Day() != 15 {
			t.Errorf("expected day 15, got %v", r)
		}
	})

	t.Run("subscription customer access", func(t *testing.T) {
		data := &models.KiwifyOrderData{
			Subscription: &models.KiwifySubscription{
				CustomerAccess: &models.KiwifyCustomerAccess{
					AccessUntil: "2025-03-01",
				},
			},
		}
		r := svc.extractAccessUntil(data)
		if r == nil || r.Month() != 3 {
			t.Errorf("expected March, got %v", r)
		}
	})

	t.Run("all nil", func(t *testing.T) {
		data := &models.KiwifyOrderData{}
		r := svc.extractAccessUntil(data)
		if r != nil {
			t.Errorf("expected nil, got %v", r)
		}
	})
}

// ---------------------------------------------------------------------------
// TestExtractEventAt
// ---------------------------------------------------------------------------

func TestExtractEventAt(t *testing.T) {
	svc := newTestService(&mockProfileRepo{}, &mockPlanRepo{}, &mockSettingsRepo{}, nil, nil, nil)

	t.Run("RefundedAt", func(t *testing.T) {
		data := &models.KiwifyOrderData{RefundedAt: "2024-05-01"}
		r := svc.extractEventAt(data)
		if r == nil || r.Day() != 1 {
			t.Errorf("expected day 1, got %v", r)
		}
	})

	t.Run("ApprovedDate", func(t *testing.T) {
		data := &models.KiwifyOrderData{ApprovedDate: "2024-06-15"}
		r := svc.extractEventAt(data)
		if r == nil || r.Day() != 15 {
			t.Errorf("expected day 15, got %v", r)
		}
	})

	t.Run("UpdatedAt and CreatedAt", func(t *testing.T) {
		data := &models.KiwifyOrderData{UpdatedAt: "2024-07-20"}
		r := svc.extractEventAt(data)
		if r == nil || r.Day() != 20 {
			t.Errorf("expected day 20, got %v", r)
		}
	})

	t.Run("subscription StartDate", func(t *testing.T) {
		data := &models.KiwifyOrderData{
			Subscription: &models.KiwifySubscription{
				StartDate: "2024-08-01",
			},
		}
		r := svc.extractEventAt(data)
		if r == nil || r.Day() != 1 {
			t.Errorf("expected day 1, got %v", r)
		}
	})

	t.Run("all nil", func(t *testing.T) {
		data := &models.KiwifyOrderData{}
		r := svc.extractEventAt(data)
		if r != nil {
			t.Errorf("expected nil, got %v", r)
		}
	})
}

// ---------------------------------------------------------------------------
// TestMapProductNameToPlan
// ---------------------------------------------------------------------------

func TestMapProductNameToPlan(t *testing.T) {
	svc := newTestService(&mockProfileRepo{}, &mockPlanRepo{}, &mockSettingsRepo{}, nil, nil, nil)

	tests := []struct {
		name string
		want string
	}{
		{"Plano Starter", "starter"},
		{"Basic Plan", "starter"},
		{"Quick Start", "starter"},
		{"Pro Access", "pro"},
		{"Professional Plan", "pro"},
		{"Master Plan", "master"},
		{"Premium Access", "master"},
		{"Elite Access", "master"},
		{"Curso XYZ", "starter"},
		{"STARTER UPPERCASE", "starter"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := svc.mapProductNameToPlan(tt.name)
			if got != tt.want {
				t.Errorf("mapProductNameToPlan(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestMapProductToPlan
// ---------------------------------------------------------------------------

func TestMapProductToPlan(t *testing.T) {
	ctx := context.Background()

	t.Run("DB lookup success with direct ID", func(t *testing.T) {
		plr := &mockPlanRepo{plan: &models.Plan{Slug: "pro"}}
		svc := newTestService(&mockProfileRepo{}, plr, &mockSettingsRepo{}, nil, nil, nil)

		got, err := svc.mapProductToPlan(ctx, "Starter", "PROD123")
		if err != nil {
			t.Fatal(err)
		}
		if got != "pro" {
			t.Errorf("expected pro, got %s", got)
		}
		if plr.searchedID != "PROD123" {
			t.Errorf("expected search ID PROD123, got %s", plr.searchedID)
		}
	})

	t.Run("DB lookup success with Kiwify URL (regex extracts ID)", func(t *testing.T) {
		plr := &mockPlanRepo{plan: &models.Plan{Slug: "master"}}
		svc := newTestService(&mockProfileRepo{}, plr, &mockSettingsRepo{}, nil, nil, nil)

		got, err := svc.mapProductToPlan(ctx, "Test", "https://pay.kiwify.com.br/ABC123")
		if err != nil {
			t.Fatal(err)
		}
		if got != "master" {
			t.Errorf("expected master, got %s", got)
		}
		if plr.searchedID != "ABC123" {
			t.Errorf("expected search ID ABC123, got %s", plr.searchedID)
		}
	})

	t.Run("DB lookup fails returns error", func(t *testing.T) {
		plr := &mockPlanRepo{getErr: errors.New("db error")}
		svc := newTestService(&mockProfileRepo{}, plr, &mockSettingsRepo{}, nil, nil, nil)

		_, err := svc.mapProductToPlan(ctx, "Pro Plan", "PROD123")
		if err == nil {
			t.Error("expected error from repo failure")
		}
	})

	t.Run("DB returns nil plan, fallback to name", func(t *testing.T) {
		plr := &mockPlanRepo{plan: nil}
		svc := newTestService(&mockProfileRepo{}, plr, &mockSettingsRepo{}, nil, nil, nil)

		got, err := svc.mapProductToPlan(ctx, "Master Plan", "PROD123")
		if err != nil {
			t.Fatal(err)
		}
		if got != "master" {
			t.Errorf("expected master, got %s", got)
		}
	})

	t.Run("empty productID uses name directly", func(t *testing.T) {
		plr := &mockPlanRepo{}
		svc := newTestService(&mockProfileRepo{}, plr, &mockSettingsRepo{}, nil, nil, nil)

		got, err := svc.mapProductToPlan(ctx, "Pro Plan", "")
		if err != nil {
			t.Fatal(err)
		}
		if got != "pro" {
			t.Errorf("expected pro, got %s", got)
		}
	})

	t.Run("non-Kiwify URL passes URL as-is", func(t *testing.T) {
		plr := &mockPlanRepo{plan: nil}
		svc := newTestService(&mockProfileRepo{}, plr, &mockSettingsRepo{}, nil, nil, nil)

		_, _ = svc.mapProductToPlan(ctx, "Starter", "https://other.com/XYZ")
		if plr.searchedID != "https://other.com/XYZ" {
			t.Errorf("expected full URL, got %s", plr.searchedID)
		}
	})
}

// ---------------------------------------------------------------------------
// TestProcessWebhook_Validation
// ---------------------------------------------------------------------------

func TestProcessWebhook_Validation(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(&mockProfileRepo{}, &mockPlanRepo{}, &mockSettingsRepo{}, nil, nil, nil)

	t.Run("email empty", func(t *testing.T) {
		p := makePayload("", "Product", "order_approved")
		r, err := svc.ProcessWebhook(ctx, p, "")
		if err != nil {
			t.Fatal(err)
		}
		if r.Success || !strings.Contains(r.Message, "email") {
			t.Errorf("expected email required, got %+v", r)
		}
	})

	t.Run("product name empty", func(t *testing.T) {
		p := makePayload("a@b.com", "", "order_approved")
		r, err := svc.ProcessWebhook(ctx, p, "")
		if err != nil {
			t.Fatal(err)
		}
		if r.Success || !strings.Contains(r.Message, "product_name") {
			t.Errorf("expected product_name required, got %+v", r)
		}
	})

	t.Run("event empty", func(t *testing.T) {
		p := makePayload("a@b.com", "Product", "")
		r, err := svc.ProcessWebhook(ctx, p, "")
		if err != nil {
			t.Fatal(err)
		}
		if r.Success || !strings.Contains(r.Message, "webhook_event_type") {
			t.Errorf("expected event required, got %+v", r)
		}
	})
}

// ---------------------------------------------------------------------------
// TestProcessWebhook_TokenHandling
// ---------------------------------------------------------------------------

func TestProcessWebhook_TokenHandling(t *testing.T) {
	ctx := context.Background()

	t.Run("config token set, signature match", func(t *testing.T) {
		pr := &mockProfileRepo{profile: makeProfile("u1", "a@b.com", "starter")}
		svc := newTestService(pr, &mockPlanRepo{}, &mockSettingsRepo{}, nil, nil, &Config{WebhookToken: "tok"})
		r, err := svc.ProcessWebhook(ctx, makePayload("a@b.com", "Starter", "order_approved"), "tok")
		if err != nil {
			t.Fatal(err)
		}
		if !r.Success {
			t.Errorf("expected success, got %+v", r)
		}
	})

	t.Run("config token set, signature mismatch (warn)", func(t *testing.T) {
		pr := &mockProfileRepo{profile: makeProfile("u1", "a@b.com", "starter")}
		svc := newTestService(pr, &mockPlanRepo{}, &mockSettingsRepo{}, nil, nil, &Config{WebhookToken: "tok"})
		r, err := svc.ProcessWebhook(ctx, makePayload("a@b.com", "Starter", "order_approved"), "wrong")
		if err != nil {
			t.Fatal(err)
		}
		// Should still process (warn only)
		if !r.Success {
			t.Errorf("expected success despite mismatch, got %+v", r)
		}
	})

	t.Run("no config token, loads from settings", func(t *testing.T) {
		pr := &mockProfileRepo{profile: makeProfile("u1", "a@b.com", "starter")}
		sr := &mockSettingsRepo{settings: models.SettingsMap{"kiwify_webhook_token": "fromdb"}}
		svc := newTestService(pr, &mockPlanRepo{}, sr, nil, nil, nil)
		r, err := svc.ProcessWebhook(ctx, makePayload("a@b.com", "Starter", "order_approved"), "fromdb")
		if err != nil {
			t.Fatal(err)
		}
		if !r.Success {
			t.Errorf("expected success, got %+v", r)
		}
	})

	t.Run("no token configured (warn)", func(t *testing.T) {
		pr := &mockProfileRepo{profile: makeProfile("u1", "a@b.com", "starter")}
		sr := &mockSettingsRepo{getErr: errors.New("no settings")}
		svc := newTestService(pr, &mockPlanRepo{}, sr, nil, nil, nil)
		r, err := svc.ProcessWebhook(ctx, makePayload("a@b.com", "Starter", "order_approved"), "")
		if err != nil {
			t.Fatal(err)
		}
		if !r.Success {
			t.Errorf("expected success, got %+v", r)
		}
	})
}

// ---------------------------------------------------------------------------
// TestProcessWebhook_EventClassification
// ---------------------------------------------------------------------------

func TestProcessWebhook_EventClassification(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		event       string
		wantContain string
	}{
		// Purchase events
		{"order_approved", "plan updated"},
		{"approved", "plan updated"},
		{"active", "plan updated"},
		{"aprovada", "plan updated"},
		{"renovada", "plan updated"},
		// Cancel events
		{"subscription_canceled", "Renewal canceled"},
		{"cancelada", "Renewal canceled"},
		// Refund events
		{"order_refunded", "suspended"},
		{"refunded", "suspended"},
		{"reembolso", "suspended"},
		// Late events
		{"subscription_late", "suspended"},
		{"late", "suspended"},
		{"overdue", "suspended"},
		{"atrasada", "suspended"},
		{"expirada", "suspended"},
	}

	for _, tt := range tests {
		t.Run(tt.event, func(t *testing.T) {
			pr := &mockProfileRepo{profile: makeProfile("u1", "a@b.com", "starter")}
			svc := newTestService(pr, &mockPlanRepo{}, &mockSettingsRepo{}, nil, nil, nil)
			p := makePayload("a@b.com", "Starter", tt.event)
			r, err := svc.ProcessWebhook(ctx, p, "")
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(strings.ToLower(r.Message), strings.ToLower(tt.wantContain)) {
				t.Errorf("event %q: expected message containing %q, got %q", tt.event, tt.wantContain, r.Message)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestProcessWebhook_OutOfOrder
// ---------------------------------------------------------------------------

func TestProcessWebhook_OutOfOrder(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()

	t.Run("incoming before existing is ignored", func(t *testing.T) {
		pr := &mockProfileRepo{
			profile: makeProfile("u1", "a@b.com", "starter"),
			billing: &models.ProfileBillingFields{
				BillingLastEventAt: timePtr(now),
			},
		}
		svc := newTestService(pr, &mockPlanRepo{}, &mockSettingsRepo{}, nil, nil, nil)
		p := makePayload("a@b.com", "Starter", "order_approved")
		p.ApprovedDate = now.Add(-1 * time.Hour).Format(time.RFC3339)
		r, err := svc.ProcessWebhook(ctx, p, "")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(r.Message, "Old event ignored") {
			t.Errorf("expected out-of-order message, got %q", r.Message)
		}
	})

	t.Run("incoming after existing is processed", func(t *testing.T) {
		pr := &mockProfileRepo{
			profile: makeProfile("u1", "a@b.com", "starter"),
			billing: &models.ProfileBillingFields{
				BillingLastEventAt: timePtr(now.Add(-1 * time.Hour)),
			},
		}
		svc := newTestService(pr, &mockPlanRepo{}, &mockSettingsRepo{}, nil, nil, nil)
		p := makePayload("a@b.com", "Starter", "order_approved")
		p.ApprovedDate = now.Format(time.RFC3339)
		r, err := svc.ProcessWebhook(ctx, p, "")
		if err != nil {
			t.Fatal(err)
		}
		if !r.Success || strings.Contains(r.Message, "Old event") {
			t.Errorf("expected successful processing, got %+v", r)
		}
	})

	t.Run("no BillingLastEventAt is processed", func(t *testing.T) {
		pr := &mockProfileRepo{
			profile: makeProfile("u1", "a@b.com", "starter"),
			billing: &models.ProfileBillingFields{},
		}
		svc := newTestService(pr, &mockPlanRepo{}, &mockSettingsRepo{}, nil, nil, nil)
		p := makePayload("a@b.com", "Starter", "order_approved")
		p.ApprovedDate = now.Format(time.RFC3339)
		r, err := svc.ProcessWebhook(ctx, p, "")
		if err != nil {
			t.Fatal(err)
		}
		if !r.Success {
			t.Errorf("expected success, got %+v", r)
		}
	})

	t.Run("no EventAt is processed", func(t *testing.T) {
		pr := &mockProfileRepo{
			profile: makeProfile("u1", "a@b.com", "starter"),
			billing: &models.ProfileBillingFields{
				BillingLastEventAt: timePtr(now),
			},
		}
		svc := newTestService(pr, &mockPlanRepo{}, &mockSettingsRepo{}, nil, nil, nil)
		p := makePayload("a@b.com", "Starter", "order_approved")
		// No date fields → EventAt will be nil → should process
		r, err := svc.ProcessWebhook(ctx, p, "")
		if err != nil {
			t.Fatal(err)
		}
		if !r.Success {
			t.Errorf("expected success, got %+v", r)
		}
	})
}

// ---------------------------------------------------------------------------
// TestProcessWebhook_GetProfileError
// ---------------------------------------------------------------------------

func TestProcessWebhook_GetProfileError(t *testing.T) {
	ctx := context.Background()
	pr := &mockProfileRepo{getErr: errors.New("db down")}
	svc := newTestService(pr, &mockPlanRepo{}, &mockSettingsRepo{}, nil, nil, nil)

	_, err := svc.ProcessWebhook(ctx, makePayload("a@b.com", "Starter", "order_approved"), "")
	if err == nil || !strings.Contains(err.Error(), "failed to get profile") {
		t.Errorf("expected profile error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// TestProcessWebhook_MapProductToPlanError
// ---------------------------------------------------------------------------

func TestProcessWebhook_MapProductToPlanError(t *testing.T) {
	ctx := context.Background()
	plr := &mockPlanRepo{getErr: errors.New("plan db error")}
	pr := &mockProfileRepo{profile: makeProfile("u1", "a@b.com", "starter")}
	svc := newTestService(pr, plr, &mockSettingsRepo{}, nil, nil, nil)

	p := makePayload("a@b.com", "Starter", "order_approved")
	p.CheckoutLink = "PROD123" // forces productID to be non-empty → triggers repo lookup

	r, err := svc.ProcessWebhook(ctx, p, "")
	if err != nil {
		t.Fatal(err)
	}
	// mapProductToPlan error is logged, plan falls back to default "starter"
	if !r.Success {
		t.Errorf("expected success with fallback plan, got %+v", r)
	}
}

// ---------------------------------------------------------------------------
// TestProcessWebhook_ExistingUser_UnknownEvent
// ---------------------------------------------------------------------------

func TestProcessWebhook_ExistingUser_UnknownEvent(t *testing.T) {
	ctx := context.Background()
	pr := &mockProfileRepo{profile: makeProfile("u1", "a@b.com", "starter")}
	svc := newTestService(pr, &mockPlanRepo{}, &mockSettingsRepo{}, nil, nil, nil)

	r, err := svc.ProcessWebhook(ctx, makePayload("a@b.com", "Starter", "unknown_event"), "")
	if err != nil {
		t.Fatal(err)
	}
	if !r.Success || !strings.Contains(r.Message, "no action taken") {
		t.Errorf("expected no action, got %+v", r)
	}
	if r.UserID != "u1" {
		t.Errorf("expected UserID u1, got %s", r.UserID)
	}
}

// ---------------------------------------------------------------------------
// TestProcessWebhook_NoUser_NotPurchase
// ---------------------------------------------------------------------------

func TestProcessWebhook_NoUser_NotPurchase(t *testing.T) {
	ctx := context.Background()

	events := []string{"order_refunded", "subscription_canceled", "subscription_late"}
	for _, ev := range events {
		t.Run(ev, func(t *testing.T) {
			pr := &mockProfileRepo{} // no profile
			svc := newTestService(pr, &mockPlanRepo{}, &mockSettingsRepo{}, nil, nil, nil)
			r, err := svc.ProcessWebhook(ctx, makePayload("a@b.com", "Starter", ev), "")
			if err != nil {
				t.Fatal(err)
			}
			if r.Success {
				t.Errorf("expected failure for non-purchase event without user, got %+v", r)
			}
			if !strings.Contains(r.Message, "User not found") {
				t.Errorf("expected User not found, got %q", r.Message)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestProcessWebhook_RefundViaOrderStatus
// ---------------------------------------------------------------------------

func TestProcessWebhook_RefundViaOrderStatus(t *testing.T) {
	ctx := context.Background()

	t.Run("OrderStatus refunded triggers handleRefundOrLate", func(t *testing.T) {
		pr := &mockProfileRepo{profile: makeProfile("u1", "a@b.com", "starter")}
		svc := newTestService(pr, &mockPlanRepo{}, &mockSettingsRepo{}, nil, nil, nil)
		p := makePayload("a@b.com", "Starter", "some_event")
		p.OrderStatus = "refunded"
		r, err := svc.ProcessWebhook(ctx, p, "")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(strings.ToLower(r.Message), "suspended") {
			t.Errorf("expected suspended, got %q", r.Message)
		}
	})

	t.Run("OrderStatus chargeback triggers handleRefundOrLate", func(t *testing.T) {
		pr := &mockProfileRepo{profile: makeProfile("u1", "a@b.com", "starter")}
		svc := newTestService(pr, &mockPlanRepo{}, &mockSettingsRepo{}, nil, nil, nil)
		p := makePayload("a@b.com", "Starter", "some_event")
		p.OrderStatus = "chargeback"
		r, err := svc.ProcessWebhook(ctx, p, "")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(strings.ToLower(r.Message), "suspended") {
			t.Errorf("expected suspended, got %q", r.Message)
		}
	})
}

// ---------------------------------------------------------------------------
// TestHandlePurchaseApproved
// ---------------------------------------------------------------------------

func TestHandlePurchaseApproved(t *testing.T) {
	ctx := context.Background()

	t.Run("happy path without suspension", func(t *testing.T) {
		pr := &mockProfileRepo{}
		svc := newTestService(pr, &mockPlanRepo{}, &mockSettingsRepo{}, nil, nil, nil)

		profile := makeProfile("u1", "a@b.com", "starter")
		data := &models.KiwifyProcessedData{
			Email:         "a@b.com",
			Event:         "order_approved",
			NextPaymentAt: timePtr(time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)),
			AccessUntil:   timePtr(time.Date(2025, 4, 1, 0, 0, 0, 0, time.UTC)),
			EventAt:       timePtr(time.Now().UTC()),
		}

		r, err := svc.handlePurchaseApproved(ctx, profile, nil, data, "pro")
		if err != nil {
			t.Fatal(err)
		}
		if !r.Success || r.Plan != "pro" {
			t.Errorf("expected success with pro plan, got %+v", r)
		}
		if pr.updateID != "u1" {
			t.Errorf("expected updateID u1, got %s", pr.updateID)
		}
		if pr.billingFields == nil {
			t.Fatal("expected billing fields to be set")
		}
		if pr.billingFields.BillingNextPaymentAt == nil {
			t.Error("expected NextPaymentAt to be set in billing")
		}
		if pr.billingFields.PlanAccessUntil == nil {
			t.Error("expected AccessUntil to be set in billing")
		}
	})

	t.Run("UpdateProfile fails", func(t *testing.T) {
		pr := &mockProfileRepo{updateErr: errors.New("db error")}
		svc := newTestService(pr, &mockPlanRepo{}, &mockSettingsRepo{}, nil, nil, nil)

		_, err := svc.handlePurchaseApproved(ctx, makeProfile("u1", "a@b.com", "starter"), nil, &models.KiwifyProcessedData{Event: "order_approved"}, "pro")
		if err == nil {
			t.Error("expected error from UpdateProfile")
		}
	})

	t.Run("suspended payment kind auto-unsuspends", func(t *testing.T) {
		pr := &mockProfileRepo{}
		svc := newTestService(pr, &mockPlanRepo{}, &mockSettingsRepo{}, nil, nil, nil)

		billing := &models.ProfileBillingFields{
			BillingStatus:         strPtr("suspended"),
			BillingSuspensionKind: strPtr("payment"),
		}
		r, err := svc.handlePurchaseApproved(ctx, makeProfile("u1", "a@b.com", "starter"), billing, &models.KiwifyProcessedData{Event: "order_approved"}, "pro")
		if err != nil {
			t.Fatal(err)
		}
		if !r.Success || !strings.Contains(r.Message, "plan updated") {
			t.Errorf("expected plan updated, got %+v", r)
		}
		if pr.billingFields.BillingStatus != nil {
			t.Error("expected billing status cleared (nil)")
		}
	})

	t.Run("suspended manual kind stays blocked", func(t *testing.T) {
		pr := &mockProfileRepo{}
		svc := newTestService(pr, &mockPlanRepo{}, &mockSettingsRepo{}, nil, nil, nil)

		billing := &models.ProfileBillingFields{
			BillingStatus:         strPtr("suspended"),
			BillingSuspensionKind: strPtr("manual"),
		}
		r, err := svc.handlePurchaseApproved(ctx, makeProfile("u1", "a@b.com", "starter"), billing, &models.KiwifyProcessedData{Event: "order_approved"}, "pro")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(r.Message, "manual block") {
			t.Errorf("expected manual block message, got %q", r.Message)
		}
	})

	t.Run("UpdateBillingStatus fails (non-fatal)", func(t *testing.T) {
		pr := &mockProfileRepo{billingErr: errors.New("billing err")}
		svc := newTestService(pr, &mockPlanRepo{}, &mockSettingsRepo{}, nil, nil, nil)

		r, err := svc.handlePurchaseApproved(ctx, makeProfile("u1", "a@b.com", "starter"), nil, &models.KiwifyProcessedData{Event: "order_approved"}, "pro")
		if err != nil {
			t.Fatal(err)
		}
		if !r.Success {
			t.Errorf("expected success despite billing error, got %+v", r)
		}
	})

	t.Run("billing nil auto-unsuspend is true", func(t *testing.T) {
		pr := &mockProfileRepo{}
		svc := newTestService(pr, &mockPlanRepo{}, &mockSettingsRepo{}, nil, nil, nil)

		r, err := svc.handlePurchaseApproved(ctx, makeProfile("u1", "a@b.com", "starter"), nil, &models.KiwifyProcessedData{Event: "order_approved"}, "pro")
		if err != nil {
			t.Fatal(err)
		}
		if !r.Success || !strings.Contains(r.Message, "plan updated") {
			t.Errorf("expected plan updated, got %+v", r)
		}
	})
}

// ---------------------------------------------------------------------------
// TestHandleRefundOrLate
// ---------------------------------------------------------------------------

func TestHandleRefundOrLate(t *testing.T) {
	ctx := context.Background()

	t.Run("refund event happy path", func(t *testing.T) {
		pr := &mockProfileRepo{}
		svc := newTestService(pr, &mockPlanRepo{}, &mockSettingsRepo{}, nil, nil, nil)

		data := &models.KiwifyProcessedData{Email: "a@b.com", Event: "order_refunded"}
		r, err := svc.handleRefundOrLate(ctx, makeProfile("u1", "a@b.com", "pro"), data)
		if err != nil {
			t.Fatal(err)
		}
		if !r.Success || !strings.Contains(r.Message, "suspended") {
			t.Errorf("expected suspended, got %+v", r)
		}
		if pr.pausedUserID != "u1" {
			t.Errorf("expected pause for u1, got %s", pr.pausedUserID)
		}
		if pr.billingFields.BillingStatus == nil || *pr.billingFields.BillingStatus != "suspended" {
			t.Error("expected billing status suspended")
		}
	})

	t.Run("refund via IsRefundStatus sets reason", func(t *testing.T) {
		pr := &mockProfileRepo{}
		svc := newTestService(pr, &mockPlanRepo{}, &mockSettingsRepo{}, nil, nil, nil)

		data := &models.KiwifyProcessedData{Email: "a@b.com", Event: "some_event", IsRefundStatus: true, OrderStatus: "chargeback"}
		r, err := svc.handleRefundOrLate(ctx, makeProfile("u1", "a@b.com", "pro"), data)
		if err != nil {
			t.Fatal(err)
		}
		if !r.Success {
			t.Errorf("expected success, got %+v", r)
		}
		if pr.billingFields.BillingSuspensionReason == nil || !strings.Contains(*pr.billingFields.BillingSuspensionReason, "refund_requested:chargeback") {
			t.Errorf("expected refund_requested reason, got %v", pr.billingFields.BillingSuspensionReason)
		}
	})

	t.Run("late event", func(t *testing.T) {
		pr := &mockProfileRepo{}
		svc := newTestService(pr, &mockPlanRepo{}, &mockSettingsRepo{}, nil, nil, nil)

		data := &models.KiwifyProcessedData{Email: "a@b.com", Event: "subscription_late"}
		r, err := svc.handleRefundOrLate(ctx, makeProfile("u1", "a@b.com", "pro"), data)
		if err != nil {
			t.Fatal(err)
		}
		if !r.Success || !strings.Contains(r.Message, "suspended") {
			t.Errorf("expected suspended, got %+v", r)
		}
	})

	t.Run("UpdateBillingStatus fails", func(t *testing.T) {
		pr := &mockProfileRepo{billingErr: errors.New("db error")}
		svc := newTestService(pr, &mockPlanRepo{}, &mockSettingsRepo{}, nil, nil, nil)

		data := &models.KiwifyProcessedData{Email: "a@b.com", Event: "order_refunded"}
		_, err := svc.handleRefundOrLate(ctx, makeProfile("u1", "a@b.com", "pro"), data)
		if err == nil {
			t.Error("expected error from UpdateBillingStatus")
		}
	})

	t.Run("PauseUserAutomations fails (non-fatal)", func(t *testing.T) {
		pr := &mockProfileRepo{pauseErr: errors.New("pause error")}
		svc := newTestService(pr, &mockPlanRepo{}, &mockSettingsRepo{}, nil, nil, nil)

		data := &models.KiwifyProcessedData{Email: "a@b.com", Event: "order_refunded"}
		r, err := svc.handleRefundOrLate(ctx, makeProfile("u1", "a@b.com", "pro"), data)
		if err != nil {
			t.Fatal(err)
		}
		if !r.Success {
			t.Errorf("expected success despite pause error, got %+v", r)
		}
	})
}

// ---------------------------------------------------------------------------
// TestHandleSubscriptionCanceled
// ---------------------------------------------------------------------------

func TestHandleSubscriptionCanceled(t *testing.T) {
	ctx := context.Background()

	t.Run("happy path", func(t *testing.T) {
		pr := &mockProfileRepo{}
		svc := newTestService(pr, &mockPlanRepo{}, &mockSettingsRepo{}, nil, nil, nil)

		data := &models.KiwifyProcessedData{Email: "a@b.com", Event: "subscription_canceled"}
		r, err := svc.handleSubscriptionCanceled(ctx, makeProfile("u1", "a@b.com", "pro"), data)
		if err != nil {
			t.Fatal(err)
		}
		if !r.Success || !strings.Contains(r.Message, "Renewal canceled") {
			t.Errorf("expected renewal canceled, got %+v", r)
		}
		if pr.updates["billing_next_payment_at"] != nil {
			t.Error("expected billing_next_payment_at to be nil")
		}
	})

	t.Run("UpdateProfile fails (non-fatal)", func(t *testing.T) {
		pr := &mockProfileRepo{updateErr: errors.New("db error")}
		svc := newTestService(pr, &mockPlanRepo{}, &mockSettingsRepo{}, nil, nil, nil)

		data := &models.KiwifyProcessedData{Email: "a@b.com", Event: "subscription_canceled"}
		r, err := svc.handleSubscriptionCanceled(ctx, makeProfile("u1", "a@b.com", "pro"), data)
		if err != nil {
			t.Fatal(err)
		}
		if !r.Success {
			t.Errorf("expected success despite update error, got %+v", r)
		}
	})
}

// ---------------------------------------------------------------------------
// TestHandleNewUserPurchase
// ---------------------------------------------------------------------------

func TestHandleNewUserPurchase(t *testing.T) {
	ctx := context.Background()

	t.Run("happy path with FullName", func(t *testing.T) {
		pr := &mockProfileRepo{}
		ur := &mockUserRoleRepo{}
		es := &mockEmailService{}
		svc := newTestService(pr, &mockPlanRepo{}, &mockSettingsRepo{}, ur, es, nil)

		data := &models.KiwifyProcessedData{
			Email:       "new@test.com",
			Event:       "order_approved",
			FullName:    "João Silva",
			PhoneNumber: "+5511999999999",
		}
		r, err := svc.handleNewUserPurchase(ctx, data, "pro")
		if err != nil {
			t.Fatal(err)
		}
		if !r.Success {
			t.Errorf("expected success, got %+v", r)
		}
		if r.Plan != "pro" {
			t.Errorf("expected pro plan, got %s", r.Plan)
		}
		if pr.createdProfile == nil {
			t.Fatal("expected profile to be created")
		}
		if pr.createdProfile.Email != "new@test.com" {
			t.Errorf("expected email new@test.com, got %s", pr.createdProfile.Email)
		}
		if ur.lastRole != "user" {
			t.Errorf("expected role user, got %s", ur.lastRole)
		}
		if !es.sentCalled || es.lastName != "João Silva" {
			t.Errorf("expected welcome email to João Silva, got called=%v name=%s", es.sentCalled, es.lastName)
		}
	})

	t.Run("no FullName uses email prefix", func(t *testing.T) {
		pr := &mockProfileRepo{}
		es := &mockEmailService{}
		svc := newTestService(pr, &mockPlanRepo{}, &mockSettingsRepo{}, &mockUserRoleRepo{}, es, nil)

		data := &models.KiwifyProcessedData{
			Email: "john@test.com",
			Event: "order_approved",
		}
		_, err := svc.handleNewUserPurchase(ctx, data, "starter")
		if err != nil {
			t.Fatal(err)
		}
		if es.lastName != "john" {
			t.Errorf("expected name john, got %s", es.lastName)
		}
	})

	t.Run("CreateProfile fails", func(t *testing.T) {
		pr := &mockProfileRepo{createErr: errors.New("create error")}
		svc := newTestService(pr, &mockPlanRepo{}, &mockSettingsRepo{}, nil, nil, nil)

		data := &models.KiwifyProcessedData{Email: "a@b.com", Event: "order_approved"}
		_, err := svc.handleNewUserPurchase(ctx, data, "pro")
		if err == nil || !strings.Contains(err.Error(), "failed to create profile") {
			t.Errorf("expected create profile error, got %v", err)
		}
	})

	t.Run("CreateUserCredentials fails (non-fatal)", func(t *testing.T) {
		pr := &mockProfileRepo{createCred: errors.New("creds error")}
		svc := newTestService(pr, &mockPlanRepo{}, &mockSettingsRepo{}, &mockUserRoleRepo{}, &mockEmailService{}, nil)

		data := &models.KiwifyProcessedData{Email: "a@b.com", Event: "order_approved"}
		r, err := svc.handleNewUserPurchase(ctx, data, "pro")
		if err != nil {
			t.Fatal(err)
		}
		if !r.Success {
			t.Errorf("expected success despite creds error, got %+v", r)
		}
	})

	t.Run("SetUserRole fails (non-fatal)", func(t *testing.T) {
		pr := &mockProfileRepo{}
		ur := &mockUserRoleRepo{setRoleErr: errors.New("role error")}
		svc := newTestService(pr, &mockPlanRepo{}, &mockSettingsRepo{}, ur, &mockEmailService{}, nil)

		data := &models.KiwifyProcessedData{Email: "a@b.com", Event: "order_approved"}
		r, err := svc.handleNewUserPurchase(ctx, data, "pro")
		if err != nil {
			t.Fatal(err)
		}
		if !r.Success {
			t.Errorf("expected success despite role error, got %+v", r)
		}
	})

	t.Run("SendWelcomeEmail fails (non-fatal)", func(t *testing.T) {
		pr := &mockProfileRepo{}
		es := &mockEmailService{sendErr: errors.New("smtp error")}
		svc := newTestService(pr, &mockPlanRepo{}, &mockSettingsRepo{}, &mockUserRoleRepo{}, es, nil)

		data := &models.KiwifyProcessedData{Email: "a@b.com", Event: "order_approved"}
		r, err := svc.handleNewUserPurchase(ctx, data, "pro")
		if err != nil {
			t.Fatal(err)
		}
		if !r.Success {
			t.Errorf("expected success despite email error, got %+v", r)
		}
	})

	t.Run("userRoleRepo nil skips SetUserRole", func(t *testing.T) {
		pr := &mockProfileRepo{}
		svc := newTestService(pr, &mockPlanRepo{}, &mockSettingsRepo{}, nil, &mockEmailService{}, nil)

		data := &models.KiwifyProcessedData{Email: "a@b.com", Event: "order_approved"}
		r, err := svc.handleNewUserPurchase(ctx, data, "pro")
		if err != nil {
			t.Fatal(err)
		}
		if !r.Success {
			t.Errorf("expected success with nil userRoleRepo, got %+v", r)
		}
	})

	t.Run("emailService nil skips SendWelcomeEmail", func(t *testing.T) {
		pr := &mockProfileRepo{}
		svc := newTestService(pr, &mockPlanRepo{}, &mockSettingsRepo{}, &mockUserRoleRepo{}, nil, nil)

		data := &models.KiwifyProcessedData{Email: "a@b.com", Event: "order_approved"}
		r, err := svc.handleNewUserPurchase(ctx, data, "pro")
		if err != nil {
			t.Fatal(err)
		}
		if !r.Success {
			t.Errorf("expected success with nil emailService, got %+v", r)
		}
	})

	t.Run("generatePasswordFn fails", func(t *testing.T) {
		pr := &mockProfileRepo{}
		svc := newTestService(pr, &mockPlanRepo{}, &mockSettingsRepo{}, nil, nil, nil)
		svc.generatePasswordFn = func() (string, error) { return "", errors.New("entropy fail") }

		data := &models.KiwifyProcessedData{Email: "a@b.com", Event: "order_approved"}
		_, err := svc.handleNewUserPurchase(ctx, data, "pro")
		if err == nil || !strings.Contains(err.Error(), "failed to generate password") {
			t.Errorf("expected password generation error, got %v", err)
		}
	})

	t.Run("hashPasswordFn fails", func(t *testing.T) {
		pr := &mockProfileRepo{}
		svc := newTestService(pr, &mockPlanRepo{}, &mockSettingsRepo{}, nil, nil, nil)
		svc.hashPasswordFn = func(string) (string, error) { return "", errors.New("hash fail") }

		data := &models.KiwifyProcessedData{Email: "a@b.com", Event: "order_approved"}
		_, err := svc.handleNewUserPurchase(ctx, data, "pro")
		if err == nil || !strings.Contains(err.Error(), "failed to hash password") {
			t.Errorf("expected hash error, got %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// TestNormalizePayload
// ---------------------------------------------------------------------------

func TestNormalizePayload(t *testing.T) {
	svc := newTestService(&mockProfileRepo{}, &mockPlanRepo{}, &mockSettingsRepo{}, nil, nil, nil)

	t.Run("wrapped format (Order present)", func(t *testing.T) {
		p := &models.KiwifyWebhookPayload{
			Order: &models.KiwifyOrderData{
				Customer:         &models.KiwifyCustomer{Email: "wrap@test.com", FullName: "Wrap User"},
				Product:          &models.KiwifyProduct{ProductName: "Pro Plan", ProductID: "PID1"},
				WebhookEventType: "order_approved",
				CheckoutLink:     "CL1",
			},
		}
		d := svc.normalizePayload(p)
		if d.Email != "wrap@test.com" {
			t.Errorf("expected wrap@test.com, got %s", d.Email)
		}
		if d.ProductName != "Pro Plan" {
			t.Errorf("expected Pro Plan, got %s", d.ProductName)
		}
		if d.ProductID != "CL1" {
			t.Errorf("expected CL1 (CheckoutLink), got %s", d.ProductID)
		}
		if d.Event != "order_approved" {
			t.Errorf("expected order_approved, got %s", d.Event)
		}
	})

	t.Run("direct format (Order nil)", func(t *testing.T) {
		p := &models.KiwifyWebhookPayload{
			Customer:         &models.KiwifyCustomer{Email: "direct@test.com"},
			Product:          &models.KiwifyProduct{ProductName: "Starter"},
			WebhookEventType: "order_approved",
		}
		d := svc.normalizePayload(p)
		if d.Email != "direct@test.com" {
			t.Errorf("expected direct@test.com, got %s", d.Email)
		}
	})

	t.Run("Customer nil", func(t *testing.T) {
		p := &models.KiwifyWebhookPayload{
			Product:          &models.KiwifyProduct{ProductName: "X"},
			WebhookEventType: "order_approved",
		}
		d := svc.normalizePayload(p)
		if d.Email != "" {
			t.Errorf("expected empty email, got %s", d.Email)
		}
	})

	t.Run("Product nil", func(t *testing.T) {
		p := &models.KiwifyWebhookPayload{
			Customer:         &models.KiwifyCustomer{Email: "a@b.com"},
			WebhookEventType: "order_approved",
		}
		d := svc.normalizePayload(p)
		if d.ProductName != "" {
			t.Errorf("expected empty product name, got %s", d.ProductName)
		}
	})

	t.Run("FullName empty, FirstName present", func(t *testing.T) {
		p := &models.KiwifyWebhookPayload{
			Customer:         &models.KiwifyCustomer{Email: "a@b.com", FirstName: "João"},
			Product:          &models.KiwifyProduct{ProductName: "X"},
			WebhookEventType: "ev",
		}
		d := svc.normalizePayload(p)
		if d.FullName != "João" {
			t.Errorf("expected João, got %s", d.FullName)
		}
	})

	t.Run("CheckoutLink as ProductID vs Product.ProductID", func(t *testing.T) {
		p := &models.KiwifyWebhookPayload{
			Customer:         &models.KiwifyCustomer{Email: "a@b.com"},
			Product:          &models.KiwifyProduct{ProductName: "X", ProductID: "FALLBACK"},
			WebhookEventType: "ev",
			CheckoutLink:     "CHECKOUT_LINK",
		}
		d := svc.normalizePayload(p)
		if d.ProductID != "CHECKOUT_LINK" {
			t.Errorf("expected CHECKOUT_LINK, got %s", d.ProductID)
		}

		// Without CheckoutLink → uses Product.ProductID
		p2 := &models.KiwifyWebhookPayload{
			Customer:         &models.KiwifyCustomer{Email: "a@b.com"},
			Product:          &models.KiwifyProduct{ProductName: "X", ProductID: "FALLBACK"},
			WebhookEventType: "ev",
		}
		d2 := svc.normalizePayload(p2)
		if d2.ProductID != "FALLBACK" {
			t.Errorf("expected FALLBACK, got %s", d2.ProductID)
		}
	})

	t.Run("OrderStatus with refund/chargeback/dispute sets IsRefundStatus", func(t *testing.T) {
		for _, status := range []string{"refunded", "chargeback", "dispute_opened"} {
			p := &models.KiwifyWebhookPayload{
				Customer:         &models.KiwifyCustomer{Email: "a@b.com"},
				Product:          &models.KiwifyProduct{ProductName: "X"},
				WebhookEventType: "ev",
				OrderStatus:      status,
			}
			d := svc.normalizePayload(p)
			if !d.IsRefundStatus {
				t.Errorf("expected IsRefundStatus=true for status %q", status)
			}
		}
	})

	t.Run("OrderStatus normal", func(t *testing.T) {
		p := &models.KiwifyWebhookPayload{
			Customer:         &models.KiwifyCustomer{Email: "a@b.com"},
			Product:          &models.KiwifyProduct{ProductName: "X"},
			WebhookEventType: "ev",
			OrderStatus:      "paid",
		}
		d := svc.normalizePayload(p)
		if d.IsRefundStatus {
			t.Error("expected IsRefundStatus=false for normal status")
		}
	})
}

// ---------------------------------------------------------------------------
// E2E tests via ProcessWebhook (full flow)
// ---------------------------------------------------------------------------

func TestProcessWebhook_E2E_NewUser_Wrapped(t *testing.T) {
	ctx := context.Background()
	pr := &mockProfileRepo{}
	plr := &mockPlanRepo{plan: &models.Plan{Slug: "pro"}}
	ur := &mockUserRoleRepo{}
	es := &mockEmailService{}
	svc := newTestService(pr, plr, &mockSettingsRepo{}, ur, es, nil)

	p := &models.KiwifyWebhookPayload{
		Order: &models.KiwifyOrderData{
			Customer:         &models.KiwifyCustomer{Email: "new@user.com", FullName: "New User", Mobile: "+5511"},
			Product:          &models.KiwifyProduct{ProductName: "Pro Plan"},
			WebhookEventType: "order_approved",
			CheckoutLink:     "CHECKOUT123",
		},
	}

	r, err := svc.ProcessWebhook(ctx, p, "")
	if err != nil {
		t.Fatal(err)
	}
	if !r.Success {
		t.Errorf("expected success, got %+v", r)
	}
	if r.Plan != "pro" {
		t.Errorf("expected pro, got %s", r.Plan)
	}
	if !es.sentCalled {
		t.Error("expected welcome email to be sent")
	}
	if ur.lastRole != "user" {
		t.Errorf("expected user role, got %s", ur.lastRole)
	}
}

func TestProcessWebhook_E2E_ExistingUser_Upgrade(t *testing.T) {
	ctx := context.Background()
	pr := &mockProfileRepo{profile: makeProfile("u1", "exist@test.com", "starter")}
	plr := &mockPlanRepo{plan: &models.Plan{Slug: "master"}}
	svc := newTestService(pr, plr, &mockSettingsRepo{}, nil, nil, nil)

	p := &models.KiwifyWebhookPayload{
		Customer:         &models.KiwifyCustomer{Email: "exist@test.com"},
		Product:          &models.KiwifyProduct{ProductName: "Master Plan"},
		WebhookEventType: "order_approved",
	}

	r, err := svc.ProcessWebhook(ctx, p, "")
	if err != nil {
		t.Fatal(err)
	}
	if !r.Success {
		t.Errorf("expected success, got %+v", r)
	}
	if pr.updates["plan"] != "master" {
		t.Errorf("expected plan master, got %v", pr.updates["plan"])
	}
}

func TestProcessWebhook_E2E_ExistingUser_Refund(t *testing.T) {
	ctx := context.Background()
	pr := &mockProfileRepo{profile: makeProfile("u1", "ref@test.com", "pro")}
	svc := newTestService(pr, &mockPlanRepo{}, &mockSettingsRepo{}, nil, nil, nil)

	p := makePayload("ref@test.com", "Pro Plan", "order_refunded")
	r, err := svc.ProcessWebhook(ctx, p, "")
	if err != nil {
		t.Fatal(err)
	}
	if !r.Success {
		t.Errorf("expected success, got %+v", r)
	}
	if !strings.Contains(r.Message, "suspended") {
		t.Errorf("expected suspended, got %q", r.Message)
	}
	if pr.pausedUserID != "u1" {
		t.Errorf("expected automations paused for u1, got %s", pr.pausedUserID)
	}
}

func TestProcessWebhook_E2E_ExistingUser_Cancel(t *testing.T) {
	ctx := context.Background()
	pr := &mockProfileRepo{profile: makeProfile("u1", "can@test.com", "pro")}
	svc := newTestService(pr, &mockPlanRepo{}, &mockSettingsRepo{}, nil, nil, nil)

	p := makePayload("can@test.com", "Pro Plan", "subscription_canceled")
	r, err := svc.ProcessWebhook(ctx, p, "")
	if err != nil {
		t.Fatal(err)
	}
	if !r.Success {
		t.Errorf("expected success, got %+v", r)
	}
	if !strings.Contains(r.Message, "Renewal canceled") {
		t.Errorf("expected renewal canceled, got %q", r.Message)
	}
}

func TestProcessWebhook_E2E_OutOfOrder_Rejected(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	pr := &mockProfileRepo{
		profile: makeProfile("u1", "oo@test.com", "pro"),
		billing: &models.ProfileBillingFields{
			BillingLastEventAt: timePtr(now),
		},
	}
	svc := newTestService(pr, &mockPlanRepo{}, &mockSettingsRepo{}, nil, nil, nil)

	p := makePayload("oo@test.com", "Pro Plan", "order_approved")
	p.ApprovedDate = now.Add(-2 * time.Hour).Format(time.RFC3339)

	r, err := svc.ProcessWebhook(ctx, p, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.Message, "Old event ignored") {
		t.Errorf("expected out-of-order rejection, got %q", r.Message)
	}
}

func TestProcessWebhook_E2E_Portuguese_Keywords(t *testing.T) {
	ctx := context.Background()

	events := map[string]string{
		"aprovada":  "plan updated",
		"renovada":  "plan updated",
		"cancelada": "Renewal canceled",
		"reembolso": "suspended",
		"atrasada":  "suspended",
		"expirada":  "suspended",
	}

	for ev, wantContain := range events {
		t.Run(ev, func(t *testing.T) {
			pr := &mockProfileRepo{profile: makeProfile("u1", "pt@test.com", "starter")}
			svc := newTestService(pr, &mockPlanRepo{}, &mockSettingsRepo{}, nil, nil, nil)
			p := makePayload("pt@test.com", "Starter", ev)
			r, err := svc.ProcessWebhook(ctx, p, "")
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(strings.ToLower(r.Message), strings.ToLower(wantContain)) {
				t.Errorf("event %q: expected %q in message, got %q", ev, wantContain, r.Message)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Billing suspension/reactivation cycle (root-cause regression tests)
// ---------------------------------------------------------------------------

func callsContain(calls []string, name string) bool {
	for _, c := range calls {
		if c == name {
			return true
		}
	}
	return false
}

// The bug: after a payment-suspension, an approved/renewal webhook cleared the
// suspension but never resumed the automations. This asserts the symmetric
// ResumeUserAutomations now runs.
func TestPurchaseApproved_ResumesAfterPaymentSuspension(t *testing.T) {
	ctx := context.Background()
	for _, ev := range []string{"order_approved", "compra aprovada", "assinatura renovada"} {
		t.Run(ev, func(t *testing.T) {
			pr := &mockProfileRepo{
				profile: makeProfile("u1", "a@test.com", "starter"),
				billing: &models.ProfileBillingFields{
					BillingStatus:         strPtr("suspended"),
					BillingSuspensionKind: strPtr(models.BillingSuspensionKindPayment),
				},
				resumeN: 3,
			}
			svc := newTestService(pr, &mockPlanRepo{}, &mockSettingsRepo{}, nil, nil, nil)
			r, err := svc.ProcessWebhook(ctx, makePayload("a@test.com", "Starter", ev), "")
			if err != nil {
				t.Fatal(err)
			}
			if !r.Success {
				t.Fatalf("expected success, got %+v", r)
			}
			if !callsContain(pr.calls, "ResumeUserAutomations") {
				t.Errorf("event %q: expected ResumeUserAutomations to be called; calls=%v", ev, pr.calls)
			}
			if pr.resumedUserID != "u1" {
				t.Errorf("expected resume for u1, got %q", pr.resumedUserID)
			}
			// Suspension must be cleared.
			if pr.billingFields == nil || pr.billingFields.BillingStatus != nil {
				t.Errorf("expected suspension cleared (BillingStatus nil), got %+v", pr.billingFields)
			}
		})
	}
}

// A manual admin block must NOT be auto-lifted by a payment webhook, and its
// automations must stay paused.
func TestPurchaseApproved_ManualBlock_DoesNotResume(t *testing.T) {
	ctx := context.Background()
	pr := &mockProfileRepo{
		profile: makeProfile("u2", "b@test.com", "starter"),
		billing: &models.ProfileBillingFields{
			BillingStatus:         strPtr("suspended"),
			BillingSuspensionKind: strPtr(models.BillingSuspensionKindManual),
		},
	}
	svc := newTestService(pr, &mockPlanRepo{}, &mockSettingsRepo{}, nil, nil, nil)
	r, err := svc.ProcessWebhook(ctx, makePayload("b@test.com", "Starter", "order_approved"), "")
	if err != nil {
		t.Fatal(err)
	}
	if !r.Success || !strings.Contains(r.Message, "manual block") {
		t.Fatalf("expected manual-block message, got %+v", r)
	}
	if callsContain(pr.calls, "ResumeUserAutomations") {
		t.Errorf("manual block must NOT resume automations; calls=%v", pr.calls)
	}
}

// Full cycle: refund pauses (marking paused_by_billing via the repo), then an
// approved payment resumes exactly those. Both halves must be present.
func TestBillingCycle_RefundPauses_ApprovedResumes(t *testing.T) {
	ctx := context.Background()

	// 1) Refund → suspend + pause.
	prRefund := &mockProfileRepo{profile: makeProfile("u3", "c@test.com", "starter"), pauseCount: 2}
	svc := newTestService(prRefund, &mockPlanRepo{}, &mockSettingsRepo{}, nil, nil, nil)
	if _, err := svc.ProcessWebhook(ctx, makePayload("c@test.com", "Starter", "order_refunded"), ""); err != nil {
		t.Fatal(err)
	}
	if !callsContain(prRefund.calls, "PauseUserAutomations") {
		t.Fatalf("refund must pause automations; calls=%v", prRefund.calls)
	}

	// 2) Payment approved after a payment-suspension → resume.
	prApprove := &mockProfileRepo{
		profile: makeProfile("u3", "c@test.com", "starter"),
		billing: &models.ProfileBillingFields{
			BillingStatus:         strPtr("suspended"),
			BillingSuspensionKind: strPtr(models.BillingSuspensionKindPayment),
		},
		resumeN: 2,
	}
	svc2 := newTestService(prApprove, &mockPlanRepo{}, &mockSettingsRepo{}, nil, nil, nil)
	if _, err := svc2.ProcessWebhook(ctx, makePayload("c@test.com", "Starter", "order_approved"), ""); err != nil {
		t.Fatal(err)
	}
	if !callsContain(prApprove.calls, "ResumeUserAutomations") {
		t.Fatalf("approval must resume automations; calls=%v", prApprove.calls)
	}
}

// ---------------------------------------------------------------------------
// Idempotency / duplicate protection + audit
// ---------------------------------------------------------------------------

func TestProcessWebhook_DuplicateIgnored(t *testing.T) {
	ctx := context.Background()
	pr := &mockProfileRepo{profile: makeProfile("u4", "d@test.com", "starter")}
	ev := &mockKiwifyEventRepo{claimed: false, priorStatus: "done"} // duplicate
	svc := NewKiwifyService(pr, &mockPlanRepo{}, &mockSettingsRepo{}, nil, ev, nil, nil, zap.NewNop())

	r, err := svc.ProcessWebhook(ctx, makePayload("d@test.com", "Starter", "order_approved"), "")
	if err != nil {
		t.Fatal(err)
	}
	if !r.Success || !strings.Contains(r.Message, "Duplicate") {
		t.Fatalf("expected duplicate-ignored result, got %+v", r)
	}
	// No side effects: a deduped event must not touch the profile at all.
	if len(pr.calls) != 0 {
		t.Errorf("duplicate must cause no profile mutations, got calls=%v", pr.calls)
	}
}

func TestProcessWebhook_FirstDelivery_ClaimsAndFinishes(t *testing.T) {
	ctx := context.Background()
	pr := &mockProfileRepo{
		profile: makeProfile("u5", "e@test.com", "starter"),
		billing: &models.ProfileBillingFields{
			BillingStatus:         strPtr("suspended"),
			BillingSuspensionKind: strPtr(models.BillingSuspensionKindPayment),
		},
		resumeN: 1,
	}
	ev := &mockKiwifyEventRepo{claimed: true}
	svc := NewKiwifyService(pr, &mockPlanRepo{}, &mockSettingsRepo{}, nil, ev, nil, nil, zap.NewNop())

	if _, err := svc.ProcessWebhook(ctx, makePayload("e@test.com", "Starter", "order_approved"), ""); err != nil {
		t.Fatal(err)
	}
	if len(ev.claims) != 1 {
		t.Errorf("expected exactly 1 claim, got %d", len(ev.claims))
	}
	if len(ev.finished) != 1 {
		t.Errorf("expected the claim to be finished on success, got %d", len(ev.finished))
	}
	if len(ev.released) != 0 {
		t.Errorf("expected no release on success, got %d", len(ev.released))
	}
	// Audit recorded for the transition (suspend→active).
	if len(ev.audits) == 0 {
		t.Errorf("expected a billing audit record")
	}
}

// A processing failure must release the idempotency claim so Kiwify's retry can
// reprocess (otherwise the event would be permanently deduped after a failure).
func TestProcessWebhook_FailureReleasesClaim(t *testing.T) {
	ctx := context.Background()
	pr := &mockProfileRepo{getErr: errors.New("db down")}
	ev := &mockKiwifyEventRepo{claimed: true}
	svc := NewKiwifyService(pr, &mockPlanRepo{}, &mockSettingsRepo{}, nil, ev, nil, nil, zap.NewNop())

	_, err := svc.ProcessWebhook(ctx, makePayload("f@test.com", "Starter", "order_approved"), "")
	if err == nil {
		t.Fatal("expected error when profile lookup fails")
	}
	if len(ev.released) != 1 {
		t.Errorf("expected claim released on failure, got released=%d finished=%d", len(ev.released), len(ev.finished))
	}
	if len(ev.finished) != 0 {
		t.Errorf("expected no finish on failure, got %d", len(ev.finished))
	}
}
