package models

// Campaign represents a marketing campaign
type Campaign struct {
	ID          string   `json:"id" db:"id"`
	UserID      string   `json:"user_id" db:"user_id"`
	Name        string   `json:"name" db:"name"`
	Description *string  `json:"description,omitempty" db:"description"`
	GroupIDs    []string `json:"group_ids" db:"group_ids"`
	Message     string   `json:"message" db:"message"`
	ImageURL    *string  `json:"image_url,omitempty" db:"image_url"`
	Status      string   `json:"status" db:"status"` // draft, scheduled, running, completed, canceled
	ScheduledAt *string  `json:"scheduled_at,omitempty" db:"scheduled_at"`
	StartedAt   *string  `json:"started_at,omitempty" db:"started_at"`
	CompletedAt *string  `json:"completed_at,omitempty" db:"completed_at"`
	TotalSent   int      `json:"total_sent" db:"total_sent"`
	TotalFailed int      `json:"total_failed" db:"total_failed"`
	CreatedAt   string   `json:"created_at" db:"created_at"`
	UpdatedAt   string   `json:"updated_at" db:"updated_at"`
}

// CampaignStatus constants
const (
	CampaignStatusDraft     = "draft"
	CampaignStatusScheduled = "scheduled"
	CampaignStatusRunning   = "running"
	CampaignStatusCompleted = "completed"
	CampaignStatusCancelled = "canceled"
)

// CreateCampaignRequest represents a request to create a campaign
type CreateCampaignRequest struct {
	Name        string   `json:"name" validate:"required"`
	Description *string  `json:"description,omitempty"`
	GroupIDs    []string `json:"groupIds" validate:"required,min=1"`
	Message     string   `json:"message" validate:"required"`
	ImageURL    *string  `json:"imageUrl,omitempty"`
	ScheduledAt *string  `json:"scheduledAt,omitempty"`
}

// CreateCampaignResponse represents the response from creating a campaign
type CreateCampaignResponse struct {
	CampaignID    string `json:"campaignId"`
	Status        string `json:"status"`
	TotalGroups   int    `json:"totalGroups"`
	ScheduledAt   string `json:"scheduledAt,omitempty"`
	EstimatedTime string `json:"estimatedTime,omitempty"`
}

// GenerateOfferImageRequest represents a request to generate an offer image
type GenerateOfferImageRequest struct {
	ProductName string   `json:"productName" validate:"required"`
	Price       float64  `json:"price" validate:"required"`
	Discount    *float64 `json:"discount,omitempty"`
	ImageURL    *string  `json:"imageUrl,omitempty"`
	Style       *string  `json:"style,omitempty"` // modern, classic, vibrant
}

// GenerateOfferImageResponse represents the response from generating an offer image
type GenerateOfferImageResponse struct {
	ImageURL    string `json:"imageUrl"`
	ImageBase64 string `json:"imageBase64,omitempty"`
}

// GenerateOfferPostRequest represents a request to generate a complete offer post
type GenerateOfferPostRequest struct {
	ProductName   string   `json:"productName" validate:"required"`
	Price         float64  `json:"price" validate:"required"`
	OriginalPrice *float64 `json:"originalPrice,omitempty"`
	AffiliateLink string   `json:"affiliateLink" validate:"required"`
	Features      []string `json:"features,omitempty"`
	CouponCode    *string  `json:"couponCode,omitempty"`
}

// GenerateOfferPostResponse represents the response from generating an offer post
type GenerateOfferPostResponse struct {
	Text     string  `json:"text"`
	ImageURL *string `json:"imageUrl,omitempty"`
}
