package models

// MLLinkEntry represents a Mercado Livre link entry
type MLLinkEntry struct {
	ID          string `json:"id" db:"id"`
	UserID      string `json:"user_id" db:"user_id"`
	OriginalURL string `json:"original_url" db:"original_url"`
	ShortURL    string `json:"short_url" db:"short_url"`
	ProductID   string `json:"product_id,omitempty" db:"product_id"`
	CreatedAt   string `json:"created_at" db:"created_at"`
}

// UserServerMapping maps users to Waha servers for load balancing
type UserServerMapping struct {
	ID        string `json:"id" db:"id"`
	UserID    string `json:"user_id" db:"user_id"`
	ServerID  string `json:"server_id" db:"server_id"`
	CreatedAt string `json:"created_at" db:"created_at"`
	UpdatedAt string `json:"updated_at" db:"updated_at"`
}

// WahaServer represents a Waha Plus server configuration
type WahaServer struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	URL         string `json:"url"`
	APIKey      string `json:"api_key"`
	MaxCapacity int    `json:"max_capacity"`
	IsActive    bool   `json:"is_active"`
	Priority    int    `json:"priority"`
}

// ServerInfo represents server connection info
type ServerInfo struct {
	ID     string `json:"id"`
	URL    string `json:"url"`
	APIKey string `json:"api_key"`
}

// EmailTemplate represents an email template
type EmailTemplate struct {
	ID          string  `json:"id" db:"id"`
	Name        string  `json:"name" db:"name"`
	Subject     string  `json:"subject" db:"subject"`
	Body        string  `json:"body" db:"body"`
	IsActive    bool    `json:"is_active" db:"is_active"`
	CreatedAt   string  `json:"created_at" db:"created_at"`
	UpdatedAt   string  `json:"updated_at" db:"updated_at"`
	Description *string `json:"description,omitempty" db:"description"`
}
