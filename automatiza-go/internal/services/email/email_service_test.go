package email

import (
	"automatiza-go/internal/models"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap/zaptest"
)

// --- Mock Settings Provider ---

type mockSettingsProvider struct {
	GetFn func(ctx context.Context, keys ...string) (models.SettingsMap, error)
}

func (m *mockSettingsProvider) Get(ctx context.Context, keys ...string) (models.SettingsMap, error) {
	if m.GetFn != nil {
		return m.GetFn(ctx, keys...)
	}
	return models.SettingsMap{}, nil
}

var _ SettingsProvider = (*mockSettingsProvider)(nil)

// --- Tests ---

func TestNewEmailService(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	cfg := &Config{
		Host:      "smtp.test.com",
		Port:      587,
		User:      "user@test.com",
		Password:  "pass",
		FromEmail: "from@test.com",
		FromName:  "Test",
	}
	svc := NewEmailService(cfg, logger)
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
}

func TestNewDynamicEmailService(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	svc := NewDynamicEmailService(nil, nil, logger)
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
}

func TestGetPlanDisplayName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		plan string
		want string
	}{
		{"pro", "Pro"},
		{"elite", "Elite"},
		{"free", "Free"},
		{"", "Free"},
		{"unknown", "Free"},
		{"PRO", "Free"}, // case-sensitive
	}

	for _, tt := range tests {
		t.Run(tt.plan, func(t *testing.T) {
			t.Parallel()
			got := getPlanDisplayName(tt.plan)
			if got != tt.want {
				t.Errorf("getPlanDisplayName(%q) = %q, want %q", tt.plan, got, tt.want)
			}
		})
	}
}

func TestParseTemplates(t *testing.T) {
	t.Parallel()
	templates := parseTemplates()

	expected := []string{"welcome", "password_reset", "plan_upgrade", "suspension_warning"}
	for _, name := range expected {
		tmpl, ok := templates[name]
		if !ok {
			t.Errorf("template %q not found", name)
			continue
		}
		if tmpl == nil {
			t.Errorf("template %q is nil", name)
		}
	}

	if len(templates) != len(expected) {
		t.Errorf("expected %d templates, got %d", len(expected), len(templates))
	}
}

func TestRenderTemplate_Welcome(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	svc := NewDynamicEmailService(nil, nil, logger).(*dynamicEmailService)

	data := map[string]interface{}{
		"Name":     "John",
		"Password": "abc123",
		"Year":     time.Now().Year(),
		"LoginURL": "https://example.com/login",
	}

	body, err := svc.renderTemplate("welcome", data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(body, "John") {
		t.Error("expected body to contain Name")
	}
	if !strings.Contains(body, "abc123") {
		t.Error("expected body to contain Password")
	}
	if !strings.Contains(body, "https://example.com/login") {
		t.Error("expected body to contain LoginURL")
	}
}

func TestRenderTemplate_PasswordReset(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	svc := NewDynamicEmailService(nil, nil, logger).(*dynamicEmailService)

	data := map[string]interface{}{
		"Name":      "Maria",
		"ResetLink": "https://example.com/reset/token123",
		"Year":      2025,
	}

	body, err := svc.renderTemplate("password_reset", data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(body, "Maria") {
		t.Error("expected body to contain Name")
	}
	if !strings.Contains(body, "https://example.com/reset/token123") {
		t.Error("expected body to contain ResetLink")
	}
}

func TestRenderTemplate_PlanUpgrade(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	svc := NewDynamicEmailService(nil, nil, logger).(*dynamicEmailService)

	data := map[string]interface{}{
		"Name":     "Carlos",
		"Plan":     "Pro",
		"PlanCode": "pro",
		"Year":     2025,
	}

	body, err := svc.renderTemplate("plan_upgrade", data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(body, "Carlos") {
		t.Error("expected body to contain Name")
	}
	if !strings.Contains(body, "Pro") {
		t.Error("expected body to contain Plan")
	}
}

func TestRenderTemplate_SuspensionWarning(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	svc := NewDynamicEmailService(nil, nil, logger).(*dynamicEmailService)

	data := map[string]interface{}{
		"Name": "Ana",
		"Days": 7,
		"Year": 2025,
	}

	body, err := svc.renderTemplate("suspension_warning", data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(body, "Ana") {
		t.Error("expected body to contain Name")
	}
	if !strings.Contains(body, "7") {
		t.Error("expected body to contain Days")
	}
}

func TestRenderTemplate_NotFound(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	svc := NewDynamicEmailService(nil, nil, logger).(*dynamicEmailService)

	_, err := svc.renderTemplate("nonexistent", nil)
	if err == nil {
		t.Fatal("expected error for nonexistent template")
	}
	if !strings.Contains(err.Error(), "template not found") {
		t.Errorf("expected 'template not found' error, got %q", err.Error())
	}
}

func TestGetConfig_NilSettingsProvider(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	fallback := &Config{Host: "fallback.smtp.com", Port: 465}
	svc := NewDynamicEmailService(nil, fallback, logger).(*dynamicEmailService)

	cfg := svc.getConfig(context.Background())
	if cfg != fallback {
		t.Error("expected fallbackConfig when settingsProvider is nil")
	}
}

func TestGetConfig_SettingsProviderError(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	fallback := &Config{Host: "fallback.smtp.com", Port: 465}
	provider := &mockSettingsProvider{
		GetFn: func(_ context.Context, _ ...string) (models.SettingsMap, error) {
			return nil, errors.New("db error")
		},
	}
	svc := NewDynamicEmailService(provider, fallback, logger).(*dynamicEmailService)

	cfg := svc.getConfig(context.Background())
	if cfg != fallback {
		t.Error("expected fallbackConfig on settings provider error")
	}
}

func TestGetConfig_IncompleteDBSettings_WithFallback(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		settings models.SettingsMap
	}{
		{
			name: "empty host",
			settings: models.SettingsMap{
				"smtp_host":     "",
				"smtp_username": "user",
				"smtp_password": "pass",
			},
		},
		{
			name: "empty username",
			settings: models.SettingsMap{
				"smtp_host":     "host",
				"smtp_username": "",
				"smtp_password": "pass",
			},
		},
		{
			name: "empty password",
			settings: models.SettingsMap{
				"smtp_host":     "host",
				"smtp_username": "user",
				"smtp_password": "",
			},
		},
		{
			name: "missing keys entirely",
			settings: models.SettingsMap{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			logger := zaptest.NewLogger(t)
			fallback := &Config{Host: "fallback.smtp.com"}
			provider := &mockSettingsProvider{
				GetFn: func(_ context.Context, _ ...string) (models.SettingsMap, error) {
					return tt.settings, nil
				},
			}
			svc := NewDynamicEmailService(provider, fallback, logger).(*dynamicEmailService)

			cfg := svc.getConfig(context.Background())
			if cfg != fallback {
				t.Error("expected fallbackConfig for incomplete settings")
			}
		})
	}
}

func TestGetConfig_IncompleteDBSettings_NilFallback(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	provider := &mockSettingsProvider{
		GetFn: func(_ context.Context, _ ...string) (models.SettingsMap, error) {
			return models.SettingsMap{
				"smtp_host":     "",
				"smtp_username": "user",
				"smtp_password": "pass",
			}, nil
		},
	}
	svc := NewDynamicEmailService(provider, nil, logger).(*dynamicEmailService)

	cfg := svc.getConfig(context.Background())
	if cfg != nil {
		t.Error("expected nil config when fallback is nil and DB incomplete")
	}
}

func TestGetConfig_CompleteDBSettings(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	provider := &mockSettingsProvider{
		GetFn: func(_ context.Context, _ ...string) (models.SettingsMap, error) {
			return models.SettingsMap{
				"smtp_host":       "mail.example.com",
				"smtp_port":       "465",
				"smtp_username":   "user@example.com",
				"smtp_password":   "secret123",
				"smtp_from_email": "noreply@example.com",
				"smtp_from_name":  "MyApp",
			}, nil
		},
	}
	svc := NewDynamicEmailService(provider, nil, logger).(*dynamicEmailService)

	cfg := svc.getConfig(context.Background())
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.Host != "mail.example.com" {
		t.Errorf("expected Host=mail.example.com, got %s", cfg.Host)
	}
	if cfg.Port != 465 {
		t.Errorf("expected Port=465, got %d", cfg.Port)
	}
	if cfg.User != "user@example.com" {
		t.Errorf("expected User=user@example.com, got %s", cfg.User)
	}
	if cfg.Password != "secret123" {
		t.Errorf("expected Password=secret123, got %s", cfg.Password)
	}
	if cfg.FromEmail != "noreply@example.com" {
		t.Errorf("expected FromEmail=noreply@example.com, got %s", cfg.FromEmail)
	}
	if cfg.FromName != "MyApp" {
		t.Errorf("expected FromName=MyApp, got %s", cfg.FromName)
	}
}

func TestGetConfig_DefaultPort587(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	provider := &mockSettingsProvider{
		GetFn: func(_ context.Context, _ ...string) (models.SettingsMap, error) {
			return models.SettingsMap{
				"smtp_host":     "mail.example.com",
				"smtp_username": "user@example.com",
				"smtp_password": "pass",
			}, nil
		},
	}
	svc := NewDynamicEmailService(provider, nil, logger).(*dynamicEmailService)

	cfg := svc.getConfig(context.Background())
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.Port != 587 {
		t.Errorf("expected default Port=587, got %d", cfg.Port)
	}
}

func TestGetConfig_CustomPortParsing(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	provider := &mockSettingsProvider{
		GetFn: func(_ context.Context, _ ...string) (models.SettingsMap, error) {
			return models.SettingsMap{
				"smtp_host":     "mail.example.com",
				"smtp_port":     "2525",
				"smtp_username": "user@example.com",
				"smtp_password": "pass",
			}, nil
		},
	}
	svc := NewDynamicEmailService(provider, nil, logger).(*dynamicEmailService)

	cfg := svc.getConfig(context.Background())
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.Port != 2525 {
		t.Errorf("expected Port=2525, got %d", cfg.Port)
	}
}

func TestGetConfig_InvalidPortFallsBack(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		port string
	}{
		{"non-numeric", "abc"},
		{"negative", "-1"},
		{"zero", "0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			logger := zaptest.NewLogger(t)
			provider := &mockSettingsProvider{
				GetFn: func(_ context.Context, _ ...string) (models.SettingsMap, error) {
					return models.SettingsMap{
						"smtp_host":     "mail.example.com",
						"smtp_port":     tt.port,
						"smtp_username": "user@example.com",
						"smtp_password": "pass",
					}, nil
				},
			}
			svc := NewDynamicEmailService(provider, nil, logger).(*dynamicEmailService)

			cfg := svc.getConfig(context.Background())
			if cfg == nil {
				t.Fatal("expected non-nil config")
			}
			if cfg.Port != 587 {
				t.Errorf("expected default Port=587 for invalid port %q, got %d", tt.port, cfg.Port)
			}
		})
	}
}

func TestGetConfig_FromEmailDefaultsToUsername(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	provider := &mockSettingsProvider{
		GetFn: func(_ context.Context, _ ...string) (models.SettingsMap, error) {
			return models.SettingsMap{
				"smtp_host":     "mail.example.com",
				"smtp_username": "user@example.com",
				"smtp_password": "pass",
				// smtp_from_email not set
			}, nil
		},
	}
	svc := NewDynamicEmailService(provider, nil, logger).(*dynamicEmailService)

	cfg := svc.getConfig(context.Background())
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.FromEmail != "user@example.com" {
		t.Errorf("expected FromEmail to default to username, got %s", cfg.FromEmail)
	}
}

func TestGetConfig_FromNameDefaultsToAutomatiza(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	provider := &mockSettingsProvider{
		GetFn: func(_ context.Context, _ ...string) (models.SettingsMap, error) {
			return models.SettingsMap{
				"smtp_host":     "mail.example.com",
				"smtp_username": "user@example.com",
				"smtp_password": "pass",
				// smtp_from_name not set
			}, nil
		},
	}
	svc := NewDynamicEmailService(provider, nil, logger).(*dynamicEmailService)

	cfg := svc.getConfig(context.Background())
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.FromName != "Automatiza" {
		t.Errorf("expected FromName=Automatiza, got %s", cfg.FromName)
	}
}

func TestSendCustomEmail_NilConfig(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	// Provider returns incomplete settings and no fallback -> nil config
	provider := &mockSettingsProvider{
		GetFn: func(_ context.Context, _ ...string) (models.SettingsMap, error) {
			return models.SettingsMap{}, nil
		},
	}
	svc := NewDynamicEmailService(provider, nil, logger)

	err := svc.SendCustomEmail(context.Background(), "to@test.com", "Subject", "<p>Body</p>")
	if err == nil {
		t.Fatal("expected error when SMTP not configured")
	}
	if !strings.Contains(err.Error(), "SMTP not configured") {
		t.Errorf("expected 'SMTP not configured' error, got %q", err.Error())
	}
}

func TestSendWelcomeEmail_RendersTemplate(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	// This will fail at the SMTP dial stage but we want to confirm template rendering works.
	// Use a nil settings provider with nil fallback to trigger "SMTP not configured" error early.
	provider := &mockSettingsProvider{
		GetFn: func(_ context.Context, _ ...string) (models.SettingsMap, error) {
			return models.SettingsMap{}, nil
		},
	}
	svc := NewDynamicEmailService(provider, nil, logger)

	err := svc.SendWelcomeEmail(context.Background(), "to@test.com", "Test User", "tempPass123")
	// Should fail with "SMTP not configured" since config is nil
	if err == nil {
		t.Fatal("expected error (no SMTP configured)")
	}
	if !strings.Contains(err.Error(), "SMTP not configured") {
		t.Errorf("expected SMTP not configured error, got %q", err.Error())
	}
}

func TestSendPasswordResetEmail_RendersTemplate(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	provider := &mockSettingsProvider{
		GetFn: func(_ context.Context, _ ...string) (models.SettingsMap, error) {
			return models.SettingsMap{}, nil
		},
	}
	svc := NewDynamicEmailService(provider, nil, logger)

	err := svc.SendPasswordResetEmail(context.Background(), "to@test.com", "User", "https://reset.link")
	if err == nil {
		t.Fatal("expected error (no SMTP configured)")
	}
	if !strings.Contains(err.Error(), "SMTP not configured") {
		t.Errorf("expected SMTP not configured error, got %q", err.Error())
	}
}

func TestSendPlanUpgradeEmail_RendersTemplate(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	provider := &mockSettingsProvider{
		GetFn: func(_ context.Context, _ ...string) (models.SettingsMap, error) {
			return models.SettingsMap{}, nil
		},
	}
	svc := NewDynamicEmailService(provider, nil, logger)

	err := svc.SendPlanUpgradeEmail(context.Background(), "to@test.com", "User", "pro")
	if err == nil {
		t.Fatal("expected error (no SMTP configured)")
	}
	if !strings.Contains(err.Error(), "SMTP not configured") {
		t.Errorf("expected SMTP not configured error, got %q", err.Error())
	}
}

func TestSendSuspensionWarningEmail_RendersTemplate(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	provider := &mockSettingsProvider{
		GetFn: func(_ context.Context, _ ...string) (models.SettingsMap, error) {
			return models.SettingsMap{}, nil
		},
	}
	svc := NewDynamicEmailService(provider, nil, logger)

	err := svc.SendSuspensionWarningEmail(context.Background(), "to@test.com", "User", 7)
	if err == nil {
		t.Fatal("expected error (no SMTP configured)")
	}
	if !strings.Contains(err.Error(), "SMTP not configured") {
		t.Errorf("expected SMTP not configured error, got %q", err.Error())
	}
}

func TestRenderTemplate_WelcomeWithTemplateData(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	svc := NewDynamicEmailService(nil, nil, logger).(*dynamicEmailService)

	year := time.Now().Year()
	data := map[string]interface{}{
		"Name":     "Bruno",
		"Password": "mySecurePass!",
		"Year":     year,
		"LoginURL": "https://app.example.com/auth",
	}

	body, err := svc.renderTemplate("welcome", data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	checks := []struct {
		label    string
		contains string
	}{
		{"Name", "Bruno"},
		{"Password", "mySecurePass!"},
		{"LoginURL", "https://app.example.com/auth"},
		{"HTML structure", "<!DOCTYPE html>"},
		{"Footer year", "Automatiza"},
	}

	for _, check := range checks {
		if !strings.Contains(body, check.contains) {
			t.Errorf("welcome template: expected body to contain %s (%q)", check.label, check.contains)
		}
	}
}

func TestRenderTemplate_PlanUpgrade_ProFeatures(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	svc := NewDynamicEmailService(nil, nil, logger).(*dynamicEmailService)

	data := map[string]interface{}{
		"Name":     "User",
		"Plan":     "Pro",
		"PlanCode": "pro",
		"Year":     2025,
	}

	body, err := svc.renderTemplate("plan_upgrade", data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(body, "Pro") {
		t.Error("expected Pro plan name in body")
	}
}

func TestRenderTemplate_PlanUpgrade_EliteFeatures(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	svc := NewDynamicEmailService(nil, nil, logger).(*dynamicEmailService)

	data := map[string]interface{}{
		"Name":     "User",
		"Plan":     "Elite",
		"PlanCode": "elite",
		"Year":     2025,
	}

	body, err := svc.renderTemplate("plan_upgrade", data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(body, "Elite") {
		t.Error("expected Elite plan name in body")
	}
}

func TestGetConfig_NonStringSettingsValues(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	provider := &mockSettingsProvider{
		GetFn: func(_ context.Context, _ ...string) (models.SettingsMap, error) {
			return models.SettingsMap{
				"smtp_host":     123,    // non-string
				"smtp_username": true,   // non-string
				"smtp_password": []int{}, // non-string
			}, nil
		},
	}
	fallback := &Config{Host: "fallback.com"}
	svc := NewDynamicEmailService(provider, fallback, logger).(*dynamicEmailService)

	cfg := svc.getConfig(context.Background())
	if cfg != fallback {
		t.Error("expected fallback config when settings contain non-string values")
	}
}

func TestGetConfig_EmptyFromEmailAndName_WithCompleteEssentials(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	provider := &mockSettingsProvider{
		GetFn: func(_ context.Context, _ ...string) (models.SettingsMap, error) {
			return models.SettingsMap{
				"smtp_host":       "mail.example.com",
				"smtp_username":   "smtp-user@mail.com",
				"smtp_password":   "pass123",
				"smtp_from_email": "",
				"smtp_from_name":  "",
			}, nil
		},
	}
	svc := NewDynamicEmailService(provider, nil, logger).(*dynamicEmailService)

	cfg := svc.getConfig(context.Background())
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.FromEmail != "smtp-user@mail.com" {
		t.Errorf("expected FromEmail to default to username 'smtp-user@mail.com', got %q", cfg.FromEmail)
	}
	if cfg.FromName != "Automatiza" {
		t.Errorf("expected FromName to default to 'Automatiza', got %q", cfg.FromName)
	}
}

func TestEmailService_RenderTemplate_NotFound(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	cfg := &Config{
		Host:      "smtp.test.com",
		Port:      587,
		User:      "user@test.com",
		Password:  "pass",
		FromEmail: "from@test.com",
		FromName:  "Test",
	}
	svc := NewEmailService(cfg, logger).(*emailService)

	_, err := svc.renderTemplate("nonexistent", nil)
	if err == nil {
		t.Fatal("expected error for nonexistent template")
	}
	if !strings.Contains(err.Error(), "template not found") {
		t.Errorf("expected 'template not found', got %q", err.Error())
	}
}

func TestEmailService_RenderTemplate_Success(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	cfg := &Config{
		Host:      "smtp.test.com",
		Port:      587,
		User:      "user@test.com",
		Password:  "pass",
		FromEmail: "from@test.com",
		FromName:  "Test",
	}
	svc := NewEmailService(cfg, logger).(*emailService)

	data := map[string]interface{}{
		"Name":     "TestUser",
		"Password": "pass123",
		"Year":     2025,
		"LoginURL": "https://example.com",
	}

	body, err := svc.renderTemplate("welcome", data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(body, "TestUser") {
		t.Error("expected rendered body to contain Name")
	}
}

func TestDynamicEmailService_TemplatesAreInitialized(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	svc := NewDynamicEmailService(nil, nil, logger).(*dynamicEmailService)

	if svc.templates == nil {
		t.Fatal("expected templates to be initialized")
	}
	if len(svc.templates) != 4 {
		t.Errorf("expected 4 templates, got %d", len(svc.templates))
	}
}

func TestGetConfig_SettingsProviderNil_NilFallback(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)
	svc := NewDynamicEmailService(nil, nil, logger).(*dynamicEmailService)

	cfg := svc.getConfig(context.Background())
	if cfg != nil {
		t.Error("expected nil config when both provider and fallback are nil")
	}
}
