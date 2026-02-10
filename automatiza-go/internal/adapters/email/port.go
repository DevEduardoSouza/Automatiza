package email

import "context"

// EmailProvider defines the contract for sending emails.
// Domain layer does NOT know about SMTP/Gmail details.
// This follows the Adapter pattern from CLAUDE.md §6.2.
type EmailProvider interface {
	// SendPasswordResetEmail sends a password reset link to the user
	SendPasswordResetEmail(ctx context.Context, input PasswordResetEmailInput) error

	// SendPasswordChangedEmail notifies user that their password was changed
	SendPasswordChangedEmail(ctx context.Context, input PasswordChangedEmailInput) error

	// SendWelcomeEmail sends welcome email with temporary password
	SendWelcomeEmail(ctx context.Context, input WelcomeEmailInput) error
}
