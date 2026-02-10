package models

// ShopeeProduct represents a product from Shopee API (extended version)
type ShopeeProduct struct {
	ItemID         int64   `json:"item_id"`
	ShopID         int64   `json:"shop_id"`
	Name           string  `json:"name"`
	Price          float64 `json:"price"`
	OriginalPrice  float64 `json:"original_price"`
	Discount       float64 `json:"discount"`
	ImageURL       string  `json:"image_url"`
	ProductURL     string  `json:"product_url"`
	AffiliateLink  string  `json:"affiliate_link,omitempty"`
	Rating         float64 `json:"rating"`
	Sold           int     `json:"sold"`
	Stock          int     `json:"stock"`
	CategoryID     int     `json:"category_id"`
	ShopName       string  `json:"shop_name"`
	Commission     float64 `json:"commission,omitempty"`
	CommissionRate float64 `json:"commission_rate,omitempty"`
	Currency       string  `json:"currency,omitempty"`
	ShopLocation   string  `json:"shop_location,omitempty"`
	IsFreeShipping bool    `json:"is_free_shipping,omitempty"`
}

// ShopeeSearchResponse represents the search response from Shopee API
type ShopeeSearchResponse struct {
	Products   []ShopeeProduct `json:"products"`
	Total      int             `json:"total"`
	Page       int             `json:"page"`
	TotalPages int             `json:"total_pages"`
}

// ShopeeAffiliateAPIResponse represents response from Shopee Affiliate API
type ShopeeAffiliateAPIResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message,omitempty"`
	Data    struct {
		Items []struct {
			ItemID         int64   `json:"item_id"`
			ShopID         int64   `json:"shop_id"`
			ItemName       string  `json:"item_name"`
			Price          int64   `json:"price"`           // In cents
			PriceMin       int64   `json:"price_min"`       // Min variant price
			PriceMax       int64   `json:"price_max"`       // Max variant price
			Discount       string  `json:"discount"`        // e.g., "50%"
			Commission     float64 `json:"commission"`      // Commission amount
			CommissionRate float64 `json:"commission_rate"` // Commission percentage
			ImageURL       string  `json:"image_url"`
			ShopName       string  `json:"shop_name"`
			SalesVolume    int     `json:"sales_volume"`
			RatingStar     float64 `json:"rating_star"`
			ShortLink      string  `json:"short_link"` // Affiliate link
			Stock          int     `json:"stock"`
			CatID          int     `json:"cat_id"`
		} `json:"items"`
		Total      int  `json:"total"`
		HasMore    bool `json:"has_more"`
		NextOffset int  `json:"next_offset"`
	} `json:"data"`
}

// ShopeeProductDetailsResponse represents product details from Shopee
type ShopeeProductDetailsResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message,omitempty"`
	Data    struct {
		ItemID         int64    `json:"item_id"`
		ShopID         int64    `json:"shop_id"`
		Name           string   `json:"name"`
		Description    string   `json:"description"`
		Price          int64    `json:"price"`
		OriginalPrice  int64    `json:"original_price"`
		Stock          int      `json:"stock"`
		Sold           int      `json:"sold"`
		Images         []string `json:"images"`
		Rating         float64  `json:"rating"`
		RatingCount    int      `json:"rating_count"`
		CategoryID     int      `json:"category_id"`
		ShopName       string   `json:"shop_name"`
		ShopRating     float64  `json:"shop_rating"`
		ShopLocation   string   `json:"shop_location"`
		ShortLink      string   `json:"short_link"`
		CommissionRate float64  `json:"commission_rate"`
	} `json:"data"`
}

// ShopeeCategoryMapping maps custom categories to Shopee category IDs
type ShopeeCategoryMapping struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	CategoryIDs []int  `json:"category_ids"`
}

// ShopeeAPISearchParams represents search parameters for Shopee Affiliate API
type ShopeeAPISearchParams struct {
	Keyword     string   `json:"keyword,omitempty"`
	CategoryIDs []int    `json:"category_ids,omitempty"`
	MinPrice    *float64 `json:"min_price,omitempty"`
	MaxPrice    *float64 `json:"max_price,omitempty"`
	MinDiscount *float64 `json:"min_discount,omitempty"`
	SortBy      string   `json:"sort_by,omitempty"` // relevance, price_asc, price_desc, discount, commission
	Page        int      `json:"page,omitempty"`
	Limit       int      `json:"limit,omitempty"`
	ShopIDs     []string `json:"shop_ids,omitempty"`
	FilterMode  string   `json:"filter_mode,omitempty"` // light, heavy
	Stores      []string `json:"stores,omitempty"`
}
