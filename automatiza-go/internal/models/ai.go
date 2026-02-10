package models

// AIAnalyzeEngagementRequest represents a request to analyze engagement
type AIAnalyzeEngagementRequest struct {
	GroupData []GroupEngagementData `json:"groupData" validate:"required"`
}

// GroupEngagementData represents engagement data for a group
type GroupEngagementData struct {
	GroupID        string  `json:"groupId"`
	GroupName      string  `json:"groupName"`
	MembersCount   int     `json:"membersCount"`
	MessagesCount  int     `json:"messagesCount"`
	ActiveMembers  int     `json:"activeMembers"`
	EngagementRate float64 `json:"engagementRate"`
}

// AIAnalyzeEngagementResponse represents the response from engagement analysis
type AIAnalyzeEngagementResponse struct {
	Summary         string   `json:"summary"`
	Recommendations []string `json:"recommendations"`
	Insights        []string `json:"insights"`
}

// AIAnalyzeTrendsRequest represents a request to analyze trends
type AIAnalyzeTrendsRequest struct {
	ProductData []ProductTrendData `json:"productData" validate:"required"`
}

// ProductTrendData represents trend data for a product
type ProductTrendData struct {
	Name     string  `json:"name"`
	Category string  `json:"category"`
	Price    float64 `json:"price"`
	Sales    int     `json:"sales"`
	Discount float64 `json:"discount"`
}

// AIAnalyzeTrendsResponse represents the response from trend analysis
type AIAnalyzeTrendsResponse struct {
	TopCategories []string `json:"topCategories"`
	TrendingItems []string `json:"trendingItems"`
	Insights      []string `json:"insights"`
}

// AIEnhanceMessageRequest represents a request to enhance a message
type AIEnhanceMessageRequest struct {
	Message   string `json:"message" validate:"required"`
	Tone      string `json:"tone,omitempty"` // professional, casual, urgent
	MaxLength int    `json:"maxLength,omitempty"`
}

// AIEnhanceMessageResponse represents the response from message enhancement
type AIEnhanceMessageResponse struct {
	EnhancedMessage string   `json:"enhancedMessage"`
	Suggestions     []string `json:"suggestions,omitempty"`
}

// AISuggestOfferTextRequest represents a request to suggest offer text
type AISuggestOfferTextRequest struct {
	ProductName    string   `json:"productName" validate:"required"`
	Price          float64  `json:"price"`
	Discount       float64  `json:"discount,omitempty"`
	Features       []string `json:"features,omitempty"`
	TargetAudience string   `json:"targetAudience,omitempty"`
}

// AISuggestOfferTextResponse represents the response from offer text suggestion
type AISuggestOfferTextResponse struct {
	Suggestions []OfferTextSuggestion `json:"suggestions"`
}

// OfferTextSuggestion represents a single offer text suggestion
type OfferTextSuggestion struct {
	Text string `json:"text"`
	Tone string `json:"tone"`
}

// CampaignStrategyRequest represents a request to generate campaign strategy
type CampaignStrategyRequest struct {
	Objective      string `json:"objective" validate:"required"`
	TargetAudience string `json:"targetAudience"`
	Category       string `json:"category"`
	Tone           string `json:"tone"`
	Budget         string `json:"budget"`
}

// CampaignStrategy represents an AI-generated campaign strategy
type CampaignStrategy struct {
	Title            string            `json:"title"`
	Name             string            `json:"name"`
	Description      string            `json:"description"`
	TargetAudience   string            `json:"targetAudience,omitempty"`
	MessageTemplates []string          `json:"messageTemplates,omitempty"`
	Steps            []CampaignStep    `json:"steps,omitempty"`
	Schedule         *CampaignSchedule `json:"schedule,omitempty"`
	KPIs             []string          `json:"kpis,omitempty"`
}

// EngagementAnalysis represents the result of engagement analysis
type EngagementAnalysis struct {
	Score           int      `json:"score"`
	Sentiment       string   `json:"sentiment"`
	TopTopics       []string `json:"topTopics,omitempty"`
	PeakHours       []string `json:"peakHours,omitempty"`
	Recommendations []string `json:"recommendations,omitempty"`
}

// TrendsAnalysis represents the result of trends analysis
type TrendsAnalysis struct {
	Category            string      `json:"category"`
	TrendingProducts    []string    `json:"trendingProducts,omitempty"`
	PriceRange          *PriceRange `json:"priceRange,omitempty"`
	Seasonality         string      `json:"seasonality,omitempty"`
	Insights            []string    `json:"insights,omitempty"`
	RecommendedKeywords []string    `json:"recommendedKeywords,omitempty"`
}

// PriceRange represents a price range
type PriceRange struct {
	Min float64 `json:"min"`
	Max float64 `json:"max"`
}

// CampaignStep represents a step in a campaign strategy
type CampaignStep struct {
	Order       int    `json:"order"`
	Action      string `json:"action"`
	Description string `json:"description"`
	Timing      string `json:"timing,omitempty"`
}

// CampaignSchedule represents the schedule for a campaign
type CampaignSchedule struct {
	BestDays  []string `json:"bestDays"`
	BestTimes []string `json:"bestTimes"`
	Frequency string   `json:"frequency"`
}
