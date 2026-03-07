package models

// CTA represents a call-to-action text in the database
type CTA struct {
	ID        string `json:"id" db:"id"`
	UserID    string `json:"user_id" db:"user_id"`
	Message   string `json:"message" db:"message"`
	Platform  string `json:"platform" db:"platform"`
	CTAType   string `json:"cta_type" db:"cta_type"`
	IsActive  bool   `json:"is_active" db:"is_active"`
	CreatedAt string `json:"created_at" db:"created_at"`
	UpdatedAt string `json:"updated_at" db:"updated_at"`
}

// CTAPlatform constants
const (
	CTAPlatformAll          = "all"
	CTAPlatformShopee       = "shopee"
	CTAPlatformMercadoLivre = "ml"
)

// CTAType constants
const (
	CTATypeAll      = "all"
	CTATypeNormal   = "normal"
	CTATypeDiscount = "discount"
)
