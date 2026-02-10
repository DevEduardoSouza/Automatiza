package email

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"

	"go.uber.org/zap"
)

// maskEmail masks an email for logs (e.g., "jo***@example.com")
// SECURITY: Never log raw email addresses (PII)
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

// SMTPConfig holds SMTP server configuration
type SMTPConfig struct {
	Host        string        // smtp.gmail.com
	Port        int           // 587 for STARTTLS, 465 for SSL
	Username    string        // email@gmail.com
	Password    string        // App Password (not regular password!)
	FromName    string        // "Automatiza"
	FromEmail   string        // noreply@automatiza.com
	TLSRequired bool          // Should be true for Gmail
	Timeout     time.Duration // Connection timeout (default 30s)
}

// smtpAdapter implements EmailProvider using SMTP
type smtpAdapter struct {
	config    SMTPConfig
	templates *Templates
	logger    *zap.Logger
}

// NewSMTPAdapter creates a new SMTP email adapter
func NewSMTPAdapter(config SMTPConfig, logger *zap.Logger) (EmailProvider, error) {
	templates, err := NewTemplates()
	if err != nil {
		return nil, fmt.Errorf("failed to parse templates: %w", err)
	}

	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	return &smtpAdapter{
		config:    config,
		templates: templates,
		logger:    logger,
	}, nil
}

// SendPasswordResetEmail sends a password reset email
func (a *smtpAdapter) SendPasswordResetEmail(ctx context.Context, input PasswordResetEmailInput) error {
	body, err := a.templates.RenderPasswordReset(input)
	if err != nil {
		return fmt.Errorf("failed to render template: %w", err)
	}

	subject := "Redefinir Senha - Automatiza"
	return a.send(ctx, input.To, subject, body)
}

// SendPasswordChangedEmail sends a notification that password was changed
func (a *smtpAdapter) SendPasswordChangedEmail(ctx context.Context, input PasswordChangedEmailInput) error {
	body, err := a.templates.RenderPasswordChanged(input)
	if err != nil {
		return fmt.Errorf("failed to render template: %w", err)
	}

	subject := "Senha Alterada - Automatiza"
	return a.send(ctx, input.To, subject, body)
}

// SendWelcomeEmail sends a welcome email with temporary password
func (a *smtpAdapter) SendWelcomeEmail(ctx context.Context, input WelcomeEmailInput) error {
	body, err := a.templates.RenderWelcome(input)
	if err != nil {
		return fmt.Errorf("failed to render template: %w", err)
	}

	subject := "Bem-vindo ao Automatiza!"
	return a.send(ctx, input.To, subject, body)
}

// send sends an email via SMTP with TLS
func (a *smtpAdapter) send(ctx context.Context, to, subject, htmlBody string) error {
	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, a.config.Timeout)
	defer cancel()

	// Build message with proper MIME headers
	msg := a.buildMessage(to, subject, htmlBody)

	// SMTP address
	addr := fmt.Sprintf("%s:%d", a.config.Host, a.config.Port)

	// TLS config - CLAUDE.md requires TLS 1.2 minimum
	tlsConfig := &tls.Config{
		ServerName: a.config.Host,
		MinVersion: tls.VersionTLS12,
	}

	// Use a channel to handle connection with context
	errChan := make(chan error, 1)

	go func() {
		var err error
		if a.config.Port == 465 {
			// Direct TLS (SSL) connection for port 465
			err = a.sendWithSSL(addr, tlsConfig, to, msg)
		} else {
			// STARTTLS for port 587
			err = a.sendWithSTARTTLS(addr, tlsConfig, to, msg)
		}
		errChan <- err
	}()

	select {
	case <-ctx.Done():
		return fmt.Errorf("email send timeout: %w", ctx.Err())
	case err := <-errChan:
		if err != nil {
			// SECURITY: Don't log raw email (PII) - use masked version
			a.logger.Error("failed to send email",
				zap.String("to_masked", maskEmail(to)),
				zap.String("subject", subject),
				zap.Error(err))
			return err
		}
	}

	// SECURITY: Don't log raw email (PII) - use masked version
	a.logger.Info("email sent successfully",
		zap.String("to_masked", maskEmail(to)),
		zap.String("subject", subject))

	return nil
}

// sendWithSTARTTLS sends email using STARTTLS (port 587)
func (a *smtpAdapter) sendWithSTARTTLS(addr string, tlsConfig *tls.Config, to string, msg []byte) error {
	// Connect to SMTP server
	conn, err := net.DialTimeout("tcp", addr, a.config.Timeout)
	if err != nil {
		return fmt.Errorf("dial failed: %w", err)
	}

	client, err := smtp.NewClient(conn, a.config.Host)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("smtp client failed: %w", err)
	}
	defer client.Close()

	// Say hello
	if err := client.Hello("localhost"); err != nil {
		return fmt.Errorf("HELO failed: %w", err)
	}

	// Start TLS
	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("STARTTLS failed: %w", err)
		}
	} else if a.config.TLSRequired {
		return fmt.Errorf("STARTTLS not supported but required")
	}

	// Authenticate
	auth := smtp.PlainAuth("", a.config.Username, a.config.Password, a.config.Host)
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("auth failed: %w", err)
	}

	// Set sender and recipient
	if err := client.Mail(a.config.FromEmail); err != nil {
		return fmt.Errorf("MAIL FROM failed: %w", err)
	}

	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("RCPT TO failed: %w", err)
	}

	// Send message body
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("DATA failed: %w", err)
	}

	_, err = w.Write(msg)
	if err != nil {
		return fmt.Errorf("write failed: %w", err)
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("close failed: %w", err)
	}

	return client.Quit()
}

// sendWithSSL sends email using direct SSL (port 465)
func (a *smtpAdapter) sendWithSSL(addr string, tlsConfig *tls.Config, to string, msg []byte) error {
	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("TLS dial failed: %w", err)
	}
	defer func() { _ = conn.Close() }()

	client, err := smtp.NewClient(conn, a.config.Host)
	if err != nil {
		return fmt.Errorf("smtp client failed: %w", err)
	}
	defer client.Close()

	// Authenticate
	auth := smtp.PlainAuth("", a.config.Username, a.config.Password, a.config.Host)
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("auth failed: %w", err)
	}

	// Set sender and recipient
	if err := client.Mail(a.config.FromEmail); err != nil {
		return fmt.Errorf("MAIL FROM failed: %w", err)
	}

	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("RCPT TO failed: %w", err)
	}

	// Send message body
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("DATA failed: %w", err)
	}

	_, err = w.Write(msg)
	if err != nil {
		return fmt.Errorf("write failed: %w", err)
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("close failed: %w", err)
	}

	return client.Quit()
}

// buildMessage constructs the email message with headers
func (a *smtpAdapter) buildMessage(to, subject, htmlBody string) []byte {
	headers := make(map[string]string)
	headers["From"] = fmt.Sprintf("%s <%s>", a.config.FromName, a.config.FromEmail)
	headers["To"] = to
	headers["Subject"] = subject
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = "text/html; charset=UTF-8"

	msg := ""
	for k, v := range headers {
		msg += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	msg += "\r\n" + htmlBody

	return []byte(msg)
}
