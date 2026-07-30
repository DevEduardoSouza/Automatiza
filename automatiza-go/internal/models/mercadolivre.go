package models

// MLProduct represents a Mercado Livre product
type MLProduct struct {
	ID             string          `json:"id"`
	Title          string          `json:"title"`
	Price          float64         `json:"price"`
	OriginalPrice  *float64        `json:"originalPrice,omitempty"`
	Discount       float64         `json:"discount,omitempty"`
	Thumbnail      string          `json:"thumbnail"`
	Permalink      string          `json:"permalink"`
	CategoryID     string          `json:"categoryId,omitempty"`
	Condition      string          `json:"condition"` // new, used
	SellerID       int64           `json:"sellerId"`
	SellerNickname string          `json:"sellerNickname,omitempty"`
	AvailableQty   int             `json:"availableQty"`
	SoldQty        int             `json:"soldQty"`
	Shipping       *MLShipping     `json:"shipping,omitempty"`
	Installments   *MLInstallments `json:"installments,omitempty"`
}

// MLShipping represents shipping info for an ML product
type MLShipping struct {
	FreeShipping bool   `json:"freeShipping"`
	Mode         string `json:"mode,omitempty"`
	LogisticType string `json:"logisticType,omitempty"`
}

// MLInstallments represents installment info for an ML product
type MLInstallments struct {
	Quantity int     `json:"quantity"`
	Amount   float64 `json:"amount"`
	Rate     float64 `json:"rate"`
}

// MLProductPreview represents a preview of an ML product for affiliate linking
// Matches the Node.js MlProductPreview structure for parity
type MLProductPreview struct {
	ID              string   `json:"id"`
	Title           string   `json:"title"`
	Price           float64  `json:"price"`
	Currency        string   `json:"currency,omitempty"`
	OriginalPrice   *float64 `json:"originalPrice,omitempty"`
	Discount        float64  `json:"discount,omitempty"`
	DiscountPercent *float64 `json:"discountPercent,omitempty"`
	Image           string   `json:"image"`
	Thumbnail       string   `json:"thumbnail,omitempty"`
	Pictures        []string `json:"pictures,omitempty"`
	Permalink       string   `json:"permalink"`
	ItemID          *string  `json:"itemId,omitempty"`
	ProductID       *string  `json:"productId,omitempty"`
	Condition       *string  `json:"condition,omitempty"`
	SoldCount       *int     `json:"soldCount,omitempty"`
	SoldQuantity    *int     `json:"soldQuantity,omitempty"`
	Rating          *float64 `json:"rating,omitempty"`
	RatingAverage   *float64 `json:"ratingAverage,omitempty"`
	ReviewCount     *int     `json:"reviewCount,omitempty"`
	ReviewsTotal    *int     `json:"reviewsTotal,omitempty"`
	Installments    *string  `json:"installments,omitempty"`
}

// HasProductData reports whether the preview carries the minimum needed to
// render an offer card (title AND price). A preview scraped from an ML
// coupon/campaign page (meli.la/sec link that is not a real product) fails
// here: it comes back empty or with a bogus title/price. Used to route those
// messages to the coupon-notice relay instead of the offer pipeline.
func (p *MLProductPreview) HasProductData() bool {
	return p != nil && p.Title != "" && p.Price > 0
}

// MLAffiliateLink represents a generated ML affiliate link
type MLAffiliateLink struct {
	OriginalURL   string `json:"originalUrl"`
	AffiliateLink string `json:"affiliateLink"`
	ProductID     string `json:"productId,omitempty"`
}

// MLCredentials represents encrypted ML credentials
type MLCredentials struct {
	Cookie string `json:"cookie"`
	CSRF   string `json:"csrf"`
	Tag    string `json:"tag,omitempty"`
}

// MLCredentialsDB represents ML credentials stored in the database
type MLCredentialsDB struct {
	ID            string `json:"id" db:"id"`
	UserID        string `json:"user_id" db:"user_id"`
	EncryptedData string `json:"encrypted_data" db:"encrypted_data"`
	CreatedAt     string `json:"created_at" db:"created_at"`
	UpdatedAt     string `json:"updated_at" db:"updated_at"`

	// Saúde da sessão (migração 084). Em claro ao lado do blob cifrado para o
	// cron listar quem precisa de checagem sem decifrar nada, e para o painel
	// exibir o estado. SessionStatus: unknown|live|expired|blocked.
	SessionStatus  string `json:"session_status" db:"session_status"`
	LastVerifiedAt string `json:"last_verified_at" db:"last_verified_at"`
	DiscoveredTag  string `json:"discovered_tag" db:"discovered_tag"`
}

// MLCreateLinkRequest represents a request to create an ML affiliate link
type MLCreateLinkRequest struct {
	ProductURL string `json:"productUrl" validate:"required"`
}

// MLPreviewRequest represents a request to preview an ML product
type MLPreviewRequest struct {
	ProductURL string `json:"productUrl" validate:"required"`
}

// MLConnectRequest represents a request to connect ML credentials
type MLConnectRequest struct {
	Cookie string `json:"cookie" validate:"required"`
	CSRF   string `json:"csrf"`
	Tag    string `json:"tag" validate:"required"`
}

// MLLinkListItem represents an item in a user's ML link list
type MLLinkListItem struct {
	ID            string  `json:"id" db:"id"`
	UserID        string  `json:"user_id" db:"user_id"`
	AutomationID  *string `json:"automation_id,omitempty" db:"automation_id"`
	ProductURL    string  `json:"product_url" db:"product_url"`
	AffiliateLink *string `json:"affiliate_link,omitempty" db:"affiliate_link"`
	Status        string  `json:"status" db:"status"` // pending, processed, failed
	ProcessedAt   *string `json:"processed_at,omitempty" db:"processed_at"`
	Error         *string `json:"error,omitempty" db:"error"`
	CreatedAt     string  `json:"created_at" db:"created_at"`
}

// MLLinkListItemStatus constants
const (
	MLLinkListItemStatusPending   = "pending"
	MLLinkListItemStatusProcessed = "processed"
	MLLinkListItemStatusFailed    = "failed"
)

// MLDispatchVars represents variables for ML dispatch templates
type MLDispatchVars struct {
	Title         string `json:"title"`
	Price         string `json:"price"`
	OriginalPrice string `json:"originalPrice,omitempty"`
	Discount      string `json:"discount,omitempty"`
	AffiliateLink string `json:"affiliateLink"`
	Image         string `json:"image,omitempty"`
	Seller        string `json:"seller,omitempty"`
}

// CanonicalKeyFromMLProduct generates a canonical key for deduplication
func CanonicalKeyFromMLProduct(product *MLProduct) string {
	return "mercadolivre:" + product.ID
}

// MlLinkList represents a Mercado Livre link list
type MlLinkList struct {
	ID          string  `json:"id" db:"id"`
	UserID      string  `json:"user_id" db:"user_id"`
	Name        string  `json:"name" db:"name"`
	Description *string `json:"description,omitempty" db:"description"`
	IsActive    bool    `json:"is_active" db:"is_active"`
	CreatedAt   string  `json:"created_at" db:"created_at"`
	UpdatedAt   string  `json:"updated_at" db:"updated_at"`
}

// MlLinkListItem represents an item in a Mercado Livre link list
type MlLinkListItem struct {
	ID                     string   `json:"id" db:"id"`
	ListID                 string   `json:"list_id" db:"list_id"`
	OriginURL              string   `json:"origin_url" db:"origin_url"`
	OrderIndex             int      `json:"order_index" db:"order_index"`
	IsActive               bool     `json:"is_active" db:"is_active"`
	ConsumedAt             *string  `json:"consumed_at,omitempty" db:"consumed_at"`
	ConsumedByAutomationID *string  `json:"consumed_by_automation_id,omitempty" db:"consumed_by_automation_id"`
	ConsumedByGroupID      *string  `json:"consumed_by_group_id,omitempty" db:"consumed_by_group_id"`
	ProductTitle           *string  `json:"product_title,omitempty" db:"product_title"`
	ProductPrice           *float64 `json:"product_price,omitempty" db:"product_price"`
	ProductImageURL        *string  `json:"product_image_url,omitempty" db:"product_image_url"`
	Metadata               []byte   `json:"metadata,omitempty" db:"metadata"`
	Info                   *string  `json:"info,omitempty" db:"info"`
	CreatedAt              string   `json:"created_at" db:"created_at"`
	UpdatedAt              string   `json:"updated_at" db:"updated_at"`
}
