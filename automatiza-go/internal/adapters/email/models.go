package email

// PasswordResetEmailInput contains data for password reset email
type PasswordResetEmailInput struct {
	To        string
	UserName  string
	ResetURL  string
	ExpiresIn string // e.g., "15 minutos"
	IPAddress string
	UserAgent string
}

// PasswordChangedEmailInput contains data for password changed notification
type PasswordChangedEmailInput struct {
	To        string
	UserName  string
	IPAddress string
	Timestamp string
}

// WelcomeEmailInput contains data for welcome email
type WelcomeEmailInput struct {
	To       string
	UserName string
	Password string // Temporary password
	LoginURL string // URL to login page
}
