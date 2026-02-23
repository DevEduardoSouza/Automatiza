package models

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
