package email

import (
	"context"
	"sync"
)

// SentEmail represents an email that was "sent" by the fake adapter
type SentEmail struct {
	To      string
	Subject string
	Type    string // "password_reset", "password_changed", "welcome"
	Input   interface{}
}

// FakeEmailAdapter is a mock email adapter for testing
type FakeEmailAdapter struct {
	mu         sync.Mutex
	SentEmails []SentEmail
	ShouldFail bool
	FailError  error
}

// NewFakeEmailAdapter creates a new fake email adapter for testing
func NewFakeEmailAdapter() *FakeEmailAdapter {
	return &FakeEmailAdapter{
		SentEmails: make([]SentEmail, 0),
	}
}

// SendPasswordResetEmail records the email for testing
func (f *FakeEmailAdapter) SendPasswordResetEmail(ctx context.Context, input PasswordResetEmailInput) error {
	if f.ShouldFail {
		return f.FailError
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	f.SentEmails = append(f.SentEmails, SentEmail{
		To:      input.To,
		Subject: "Redefinir Senha - Automatiza",
		Type:    "password_reset",
		Input:   input,
	})

	return nil
}

// SendPasswordChangedEmail records the email for testing
func (f *FakeEmailAdapter) SendPasswordChangedEmail(ctx context.Context, input PasswordChangedEmailInput) error {
	if f.ShouldFail {
		return f.FailError
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	f.SentEmails = append(f.SentEmails, SentEmail{
		To:      input.To,
		Subject: "Senha Alterada - Automatiza",
		Type:    "password_changed",
		Input:   input,
	})

	return nil
}

// SendWelcomeEmail records the email for testing
func (f *FakeEmailAdapter) SendWelcomeEmail(ctx context.Context, input WelcomeEmailInput) error {
	if f.ShouldFail {
		return f.FailError
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	f.SentEmails = append(f.SentEmails, SentEmail{
		To:      input.To,
		Subject: "Bem-vindo ao Automatiza!",
		Type:    "welcome",
		Input:   input,
	})

	return nil
}

// GetSentEmails returns all sent emails (thread-safe)
func (f *FakeEmailAdapter) GetSentEmails() []SentEmail {
	f.mu.Lock()
	defer f.mu.Unlock()

	result := make([]SentEmail, len(f.SentEmails))
	copy(result, f.SentEmails)
	return result
}

// GetLastEmail returns the last sent email
func (f *FakeEmailAdapter) GetLastEmail() *SentEmail {
	f.mu.Lock()
	defer f.mu.Unlock()

	if len(f.SentEmails) == 0 {
		return nil
	}
	return &f.SentEmails[len(f.SentEmails)-1]
}

// Reset clears all sent emails
func (f *FakeEmailAdapter) Reset() {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.SentEmails = make([]SentEmail, 0)
	f.ShouldFail = false
	f.FailError = nil
}

// SetFailure configures the adapter to fail
func (f *FakeEmailAdapter) SetFailure(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.ShouldFail = true
	f.FailError = err
}

// Ensure FakeEmailAdapter implements EmailProvider
var _ EmailProvider = (*FakeEmailAdapter)(nil)
