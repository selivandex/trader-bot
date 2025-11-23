package models

import "time"

// NewsItem represents single news item
type NewsItem struct {
	ID          string    `json:"id"`
	Source      string    `json:"source"` // twitter, reddit, coindesk, etc
	Title       string    `json:"title"`
	Content     string    `json:"content"`
	URL         string    `json:"url"`
	Author      string    `json:"author"`
	PublishedAt time.Time `json:"published_at"`
	ProcessedAt time.Time `json:"processed_at"`
	Sentiment   float64   `json:"sentiment"` // -1.0 to 1.0
	Relevance   float64   `json:"relevance"` // 0.0 to 1.0
	Impact      int       `json:"impact"`    // 1-10 (market impact score)
	Urgency     string    `json:"urgency"`   // IMMEDIATE, HOURS, DAYS
	Keywords    []string  `json:"keywords"`
	Symbols     []string  `json:"symbols"` // Mentioned symbols: ["BTC", "ETH"]
}

// NewsSummary aggregates news sentiment
type NewsSummary struct {
	TotalItems       int        `json:"total_items"`
	PositiveCount    int        `json:"positive_count"`
	NegativeCount    int        `json:"negative_count"`
	NeutralCount     int        `json:"neutral_count"`
	AverageSentiment float64    `json:"average_sentiment"`
	OverallSentiment string     `json:"overall_sentiment"` // bullish, bearish, neutral
	RecentNews       []NewsItem `json:"recent_news"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// GetOverallSentiment calculates overall market sentiment
func (ns *NewsSummary) GetOverallSentiment() string {
	if ns.AverageSentiment > 0.2 {
		return "bullish"
	} else if ns.AverageSentiment < -0.2 {
		return "bearish"
	}
	return "neutral"
}
