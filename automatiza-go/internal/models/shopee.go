package models

import "strconv"

// ShopeeOffer represents a Shopee product offer
type ShopeeOffer struct {
	ItemID          int64   `json:"itemId"`
	ShopID          int64   `json:"shopId"`
	Name            string  `json:"name"`
	Image           string  `json:"image"`
	ImageURL        string  `json:"imageUrl,omitempty"` // Alias for Image
	Price           float64 `json:"price"`
	PriceMin        float64 `json:"priceMin"`
	PriceMax        float64 `json:"priceMax"`
	OriginalPrice   float64 `json:"originalPrice"`
	Discount        float64 `json:"discount"`
	CommissionRate  float64 `json:"commissionRate"`
	Sales           int     `json:"sales"`
	Stock           int     `json:"stock"`
	ShopName        string  `json:"shopName"`
	ShopRating      float64 `json:"shopRating"`
	ProductURL      string  `json:"productUrl"`
	AffiliateLink   string  `json:"affiliateLink,omitempty"`
	Category        string  `json:"category,omitempty"`
	CategoryID      int64   `json:"categoryId,omitempty"`
	IsMall          bool    `json:"isMall"`
	IsPreferredPlus bool    `json:"isPreferredPlus"`
	// ProductCategoryIDs são os IDs OFICIAIS de categoria da Shopee
	// (ProductOfferV2.productCatIds — schema.graphql:438 declara
	// `productCatIds: [Int!]`, lista nullable de Int não nulos). É o
	// único sinal de categoria que a API entrega; nada aqui é inferido
	// de título, descrição ou nome de loja.
	//
	// nil/vazio = produto sem categoria conhecida → só casa regra de
	// cupom genérica. GraphQL Int é 32 bits, então int64 é alargamento
	// sem perda (mesma razão do BIGINT[] na coluna).
	ProductCategoryIDs []int64 `json:"productCatIds,omitempty"`
}

// ShopeeSearchParams represents search parameters for Shopee
type ShopeeSearchParams struct {
	Keyword        *string  `json:"keyword,omitempty"`
	CategoryID     *int64   `json:"categoryId,omitempty"`
	Categories     []string `json:"categories,omitempty"` // Multiple category names/IDs
	MinPrice       *float64 `json:"minPrice,omitempty"`
	MaxPrice       *float64 `json:"maxPrice,omitempty"`
	MinDiscount    *float64 `json:"minDiscount,omitempty"`
	SortBy         *string  `json:"sortBy,omitempty"`         // sales, price, discount
	FilterType     *string  `json:"filterType,omitempty"`     // light, heavy
	UseCategoryIDs *bool    `json:"useCategoryIds,omitempty"` // Use category IDs for filtering
	Limit          int      `json:"limit,omitempty"`
	Offset         int      `json:"offset,omitempty"`
}

// ShopeeSearchResult represents the result of a Shopee search
type ShopeeSearchResult struct {
	Offers     []ShopeeOffer `json:"offers"`
	Total      int           `json:"total"`
	HasMore    bool          `json:"hasMore"`
	NextOffset int           `json:"nextOffset,omitempty"`
}

// ShopeeSearchOffersRequest represents a request to search Shopee offers
type ShopeeSearchOffersRequest struct {
	Keyword     *string  `json:"keyword,omitempty"`
	CategoryID  *int64   `json:"categoryId,omitempty"`
	MinPrice    *float64 `json:"minPrice,omitempty"`
	MaxPrice    *float64 `json:"maxPrice,omitempty"`
	MinDiscount *float64 `json:"minDiscount,omitempty"`
	Limit       int      `json:"limit,omitempty"`
}

// ShopeeGenerateLinkRequest represents a request to generate an affiliate link
type ShopeeGenerateLinkRequest struct {
	ProductURL string `json:"productUrl" validate:"required"`
}

// ShopeeAffiliateLink represents a generated affiliate link
type ShopeeAffiliateLink struct {
	OriginalURL   string `json:"originalUrl"`
	AffiliateLink string `json:"affiliateLink"`
	AffiliateURL  string `json:"affiliateUrl,omitempty"` // Alias for AffiliateLink
	ShortLink     string `json:"shortLink,omitempty"`
	ImageURL      string `json:"imageUrl,omitempty"`
	ItemID        int64  `json:"itemId,omitempty"`
	ShopID        int64  `json:"shopId,omitempty"`
}

// ShopeeCategory represents a Shopee category
type ShopeeCategory struct {
	// ID é o PRIMEIRO ID do grupo, preservado para o caller de
	// diagnóstico que já existia (automation_service.go) — não é um
	// identificador único do grupo.
	ID   int64  `json:"id"`
	Name string `json:"name"`
	// IDs é a lista COMPLETA de IDs oficiais que o grupo cobre. O mapa
	// de categorias tem IDs que aparecem em mais de um grupo, então a
	// união de IDs selecionados não reconstrói os grupos de origem.
	IDs      []int64 `json:"ids,omitempty"`
	ParentID *int64  `json:"parentId,omitempty"`
	Level    int     `json:"level"`
}

// AffiliateCredentials represents affiliate store credentials (Shopee, ML, Amazon, etc.)
type AffiliateCredentials struct {
	ID             string            `json:"id" db:"id"`
	UserID         string            `json:"user_id" db:"user_id"`
	Store          string            `json:"store" db:"store"`
	AppID          string            `json:"app_id" db:"app_id"`                       // Legacy: use Credentials map instead
	AppSecret      string            `json:"app_secret" db:"app_secret"`               // Legacy: use Credentials map instead
	Credentials    map[string]string `json:"credentials" db:"credentials"`             // Store-specific credentials as JSONB
	AutoGenerate   bool              `json:"auto_generate" db:"auto_generate"`         // Auto-generate affiliate links
	IsActive       bool              `json:"is_active" db:"is_active"`                 // Whether credentials are active
	AccessToken    *string           `json:"access_token,omitempty" db:"access_token"` // OAuth access token (for OAuth stores)
	RefreshToken   *string           `json:"refresh_token,omitempty" db:"refresh_token"`
	TokenExpiresAt *string           `json:"token_expires_at,omitempty" db:"token_expires_at"`
	CreatedAt      string            `json:"created_at" db:"created_at"`
	UpdatedAt      string            `json:"updated_at" db:"updated_at"`
}

// GetCredential retrieves a specific credential value from the Credentials map
// Falls back to legacy AppID/AppSecret fields if the map is empty
func (c *AffiliateCredentials) GetCredential(key string) string {
	if c.Credentials != nil {
		if val, ok := c.Credentials[key]; ok {
			return val
		}
	}
	// Fallback to legacy fields
	switch key {
	case "app_id":
		return c.AppID
	case "app_secret":
		return c.AppSecret
	}
	return ""
}

// SetCredential sets a credential value in the Credentials map
func (c *AffiliateCredentials) SetCredential(key, value string) {
	if c.Credentials == nil {
		c.Credentials = make(map[string]string)
	}
	c.Credentials[key] = value
}

// HasValidCredentials checks if the credentials have the required fields for the store.
// Accepts both camelCase keys (from frontend) and snake_case keys (legacy).
func (c *AffiliateCredentials) HasValidCredentials() bool {
	switch c.Store {
	case "shopee":
		appID := c.GetCredential("appId")
		if appID == "" {
			appID = c.GetCredential("app_id")
		}
		secret := c.GetCredential("password")
		if secret == "" {
			secret = c.GetCredential("app_secret")
		}
		return appID != "" && secret != ""
	case "mercado_livre", "ml":
		return c.GetCredential("encrypted_token") != "" ||
			c.GetCredential("access_token") != "" ||
			c.GetCredential("tag") != ""
	case "amazon":
		tag := c.GetCredential("tag")
		if tag == "" {
			tag = c.GetCredential("partner_tag")
		}
		accessKey := c.GetCredential("accessKey")
		if accessKey == "" {
			accessKey = c.GetCredential("access_key")
		}
		secretKey := c.GetCredential("secretKey")
		if secretKey == "" {
			secretKey = c.GetCredential("secret_key")
		}
		return tag != "" && accessKey != "" && secretKey != ""
	case "awin":
		awinID := c.GetCredential("awinId")
		if awinID == "" {
			awinID = c.GetCredential("publisher_id")
		}
		token := c.GetCredential("oauth2Token")
		if token == "" {
			token = c.GetCredential("api_key")
		}
		return awinID != "" && token != ""
	case "magalu":
		return c.GetCredential("partnerStore") != ""
	case "shein":
		return c.GetCredential("affiliateLink") != ""
	case "aliexpress":
		return c.GetCredential("trackingId") != "" && c.GetCredential("appKey") != ""
	default:
		return false
	}
}

// HasRealDiscount returns true if the offer has a meaningful discount
// (final price is strictly less than the original price).
func (o *ShopeeOffer) HasRealDiscount() bool {
	return o.Price > 0 && o.OriginalPrice > 0 && o.Price < o.OriginalPrice
}

// CanonicalKeyFromShopeeOffer generates a canonical key for deduplication
func CanonicalKeyFromShopeeOffer(offer *ShopeeOffer) string {
	return FormatCanonicalKey("shopee", offer.ShopID, offer.ItemID)
}

// FormatCanonicalKey formats a canonical key for deduplication
func FormatCanonicalKey(store string, shopID, itemID int64) string {
	return store + ":" + strconv.FormatInt(shopID, 10) + ":" + strconv.FormatInt(itemID, 10)
}
