package email

import (
	"automatiza-go/internal/models"
	"automatiza-go/internal/observability"
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"html/template"
	"strconv"
	"time"

	"github.com/getsentry/sentry-go"
	"go.uber.org/zap"
	"gopkg.in/gomail.v2"
)

// Service defines the email service interface
type Service interface {
	SendWelcomeEmail(ctx context.Context, to, name, password string) error
	SendPasswordResetEmail(ctx context.Context, to, name, resetLink string) error
	SendPlanUpgradeEmail(ctx context.Context, to, name, plan string) error
	SendSuspensionWarningEmail(ctx context.Context, to, name string, daysUntilSuspension int) error
	SendCustomEmail(ctx context.Context, to, subject, htmlBody string) error
}

// SettingsProvider interface for reading SMTP settings from database
type SettingsProvider interface {
	Get(ctx context.Context, keys ...string) (models.SettingsMap, error)
}

// Config holds email service configuration
type Config struct {
	Host      string
	Port      int
	User      string
	Password  string
	FromEmail string
	FromName  string
	UseTLS    bool
}

type emailService struct {
	config    *Config
	dialer    *gomail.Dialer
	templates map[string]*template.Template
	logger    *zap.Logger
}

// dynamicEmailService reads SMTP config from database on each send
type dynamicEmailService struct {
	settingsProvider SettingsProvider
	fallbackConfig   *Config
	templates        map[string]*template.Template
	logger           *zap.Logger
}

// NewEmailService creates a new email service with SMTP configuration
func NewEmailService(cfg *Config, logger *zap.Logger) Service {
	dialer := gomail.NewDialer(cfg.Host, cfg.Port, cfg.User, cfg.Password)

	// Configure TLS
	dialer.TLSConfig = &tls.Config{
		ServerName:         cfg.Host,
		InsecureSkipVerify: false,
	}

	// Parse templates
	templates := parseTemplates()

	return &emailService{
		config:    cfg,
		dialer:    dialer,
		templates: templates,
		logger:    logger,
	}
}

// NewDynamicEmailService creates an email service that reads config from database
// Falls back to provided config if database settings are not available
func NewDynamicEmailService(settingsProvider SettingsProvider, fallbackConfig *Config, logger *zap.Logger) Service {
	return &dynamicEmailService{
		settingsProvider: settingsProvider,
		fallbackConfig:   fallbackConfig,
		templates:        parseTemplates(),
		logger:           logger,
	}
}

// getConfig reads SMTP configuration from database, falling back to env config
func (s *dynamicEmailService) getConfig(ctx context.Context) *Config {
	if s.settingsProvider == nil {
		return s.fallbackConfig
	}

	settings, err := s.settingsProvider.Get(ctx,
		"smtp_host",
		"smtp_port",
		"smtp_username",
		"smtp_password",
		"smtp_from_email",
		"smtp_from_name",
	)
	if err != nil {
		s.logger.Warn("Failed to get SMTP settings from database, using fallback",
			zap.Error(err))
		return s.fallbackConfig
	}

	// Helper to safely get string from settings
	getString := func(key string) string {
		if v, ok := settings[key]; ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
		return ""
	}

	// Check if essential settings exist in DB
	host := getString("smtp_host")
	username := getString("smtp_username")
	password := getString("smtp_password")

	if host == "" || username == "" || password == "" {
		// DB settings incomplete, use fallback
		if s.fallbackConfig != nil {
			return s.fallbackConfig
		}
		return nil
	}

	// Parse port (default to 587 if not set)
	port := 587
	if portStr := getString("smtp_port"); portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil && p > 0 {
			port = p
		}
	}

	fromEmail := getString("smtp_from_email")
	if fromEmail == "" {
		fromEmail = username // Fallback to username
	}

	fromName := getString("smtp_from_name")
	if fromName == "" {
		fromName = "Automatiza"
	}

	return &Config{
		Host:      host,
		Port:      port,
		User:      username,
		Password:  password,
		FromEmail: fromEmail,
		FromName:  fromName,
	}
}

func (s *dynamicEmailService) SendWelcomeEmail(ctx context.Context, to, name, password string) error {
	data := map[string]interface{}{
		"Name":     name,
		"Password": password,
		"Year":     time.Now().Year(),
		"LoginURL": "https://astecwms.com.br/auth",
	}

	body, err := s.renderTemplate("welcome", data)
	if err != nil {
		return fmt.Errorf("failed to render welcome template: %w", err)
	}

	return s.send(ctx, to, "Bem-vindo ao Automatiza!", body)
}

func (s *dynamicEmailService) SendPasswordResetEmail(ctx context.Context, to, name, resetLink string) error {
	data := map[string]interface{}{
		"Name":      name,
		"ResetLink": resetLink,
		"Year":      time.Now().Year(),
	}

	body, err := s.renderTemplate("password_reset", data)
	if err != nil {
		return fmt.Errorf("failed to render password reset template: %w", err)
	}

	return s.send(ctx, to, "Redefinir Senha - Automatiza", body)
}

func (s *dynamicEmailService) SendPlanUpgradeEmail(ctx context.Context, to, name, plan string) error {
	planName := getPlanDisplayName(plan)

	data := map[string]interface{}{
		"Name":     name,
		"Plan":     planName,
		"PlanCode": plan,
		"Year":     time.Now().Year(),
	}

	body, err := s.renderTemplate("plan_upgrade", data)
	if err != nil {
		return fmt.Errorf("failed to render plan upgrade template: %w", err)
	}

	return s.send(ctx, to, fmt.Sprintf("Parabens! Plano %s Ativado - Automatiza", planName), body)
}

func (s *dynamicEmailService) SendSuspensionWarningEmail(ctx context.Context, to, name string, daysUntilSuspension int) error {
	data := map[string]interface{}{
		"Name": name,
		"Days": daysUntilSuspension,
		"Year": time.Now().Year(),
	}

	body, err := s.renderTemplate("suspension_warning", data)
	if err != nil {
		return fmt.Errorf("failed to render suspension warning template: %w", err)
	}

	return s.send(ctx, to, "Aviso: Sua conta sera suspensa - Automatiza", body)
}

func (s *dynamicEmailService) SendCustomEmail(ctx context.Context, to, subject, htmlBody string) error {
	return s.send(ctx, to, subject, htmlBody)
}

func (s *dynamicEmailService) send(ctx context.Context, to, subject, htmlBody string) error {
	cfg := s.getConfig(ctx)
	if cfg == nil {
		cfgErr := fmt.Errorf("SMTP not configured")
		observability.LogAndCapture(s.logger, sentry.LevelError, "Email SMTP not configured", cfgErr, map[string]string{
			"external_service": "email",
		})
		return cfgErr
	}

	dialer := gomail.NewDialer(cfg.Host, cfg.Port, cfg.User, cfg.Password)
	dialer.TLSConfig = &tls.Config{
		ServerName:         cfg.Host,
		InsecureSkipVerify: false,
	}

	m := gomail.NewMessage()
	m.SetHeader("From", fmt.Sprintf("%s <%s>", cfg.FromName, cfg.FromEmail))
	m.SetHeader("To", to)
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", htmlBody)

	if err := dialer.DialAndSend(m); err != nil {
		observability.LogAndCapture(s.logger, sentry.LevelError, "Failed to send email via SMTP", err, map[string]string{
			"external_service": "email",
			"email_subject":    subject,
		})
		return fmt.Errorf("failed to send email: %w", err)
	}

	s.logger.Info("Email sent successfully",
		zap.String("to", to),
		zap.String("subject", subject))

	return nil
}

func (s *dynamicEmailService) renderTemplate(name string, data map[string]interface{}) (string, error) {
	tmpl, ok := s.templates[name]
	if !ok {
		return "", fmt.Errorf("template not found: %s", name)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func (s *emailService) SendWelcomeEmail(ctx context.Context, to, name, password string) error {
	data := map[string]interface{}{
		"Name":     name,
		"Password": password,
		"Year":     time.Now().Year(),
		"LoginURL": "https://astecwms.com.br/auth",
	}

	body, err := s.renderTemplate("welcome", data)
	if err != nil {
		return fmt.Errorf("failed to render welcome template: %w", err)
	}

	return s.send(to, "Bem-vindo ao Automatiza!", body)
}

func (s *emailService) SendPasswordResetEmail(ctx context.Context, to, name, resetLink string) error {
	data := map[string]interface{}{
		"Name":      name,
		"ResetLink": resetLink,
		"Year":      time.Now().Year(),
	}

	body, err := s.renderTemplate("password_reset", data)
	if err != nil {
		return fmt.Errorf("failed to render password reset template: %w", err)
	}

	return s.send(to, "Redefinir Senha - Automatiza", body)
}

func (s *emailService) SendPlanUpgradeEmail(ctx context.Context, to, name, plan string) error {
	planName := getPlanDisplayName(plan)

	data := map[string]interface{}{
		"Name":     name,
		"Plan":     planName,
		"PlanCode": plan,
		"Year":     time.Now().Year(),
	}

	body, err := s.renderTemplate("plan_upgrade", data)
	if err != nil {
		return fmt.Errorf("failed to render plan upgrade template: %w", err)
	}

	return s.send(to, fmt.Sprintf("Parabens! Plano %s Ativado - Automatiza", planName), body)
}

func (s *emailService) SendSuspensionWarningEmail(ctx context.Context, to, name string, daysUntilSuspension int) error {
	data := map[string]interface{}{
		"Name": name,
		"Days": daysUntilSuspension,
		"Year": time.Now().Year(),
	}

	body, err := s.renderTemplate("suspension_warning", data)
	if err != nil {
		return fmt.Errorf("failed to render suspension warning template: %w", err)
	}

	return s.send(to, "Aviso: Sua conta sera suspensa - Automatiza", body)
}

func (s *emailService) SendCustomEmail(ctx context.Context, to, subject, htmlBody string) error {
	return s.send(to, subject, htmlBody)
}

func (s *emailService) send(to, subject, htmlBody string) error {
	m := gomail.NewMessage()
	m.SetHeader("From", fmt.Sprintf("%s <%s>", s.config.FromName, s.config.FromEmail))
	m.SetHeader("To", to)
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", htmlBody)

	if err := s.dialer.DialAndSend(m); err != nil {
		observability.LogAndCapture(s.logger, sentry.LevelError, "Failed to send email via SMTP", err, map[string]string{
			"external_service": "email",
			"email_subject":    subject,
		})
		return fmt.Errorf("failed to send email: %w", err)
	}

	s.logger.Info("Email sent successfully",
		zap.String("to", to),
		zap.String("subject", subject))

	return nil
}

func (s *emailService) renderTemplate(name string, data map[string]interface{}) (string, error) {
	tmpl, ok := s.templates[name]
	if !ok {
		return "", fmt.Errorf("template not found: %s", name)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func getPlanDisplayName(plan string) string {
	switch plan {
	case "pro":
		return "Pro"
	case "elite":
		return "Elite"
	default:
		return "Free"
	}
}

func parseTemplates() map[string]*template.Template {
	templates := make(map[string]*template.Template)

	templates["welcome"] = template.Must(template.New("welcome").Parse(welcomeTemplate))
	templates["password_reset"] = template.Must(template.New("password_reset").Parse(passwordResetTemplate))
	templates["plan_upgrade"] = template.Must(template.New("plan_upgrade").Parse(planUpgradeTemplate))
	templates["suspension_warning"] = template.Must(template.New("suspension_warning").Parse(suspensionWarningTemplate))

	return templates
}
