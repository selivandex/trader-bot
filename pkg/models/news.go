package models

import "time"

// NewsItem represents single news item
type NewsItem struct {
	ID               string    `json:"id" db:"id"`
	Source           string    `json:"source" db:"source"` // twitter, reddit, coindesk, etc
	Title            string    `json:"title" db:"title"`
	Content          string    `json:"content" db:"content"`
	URL              string    `json:"url" db:"url"`
	Author           string    `json:"author" db:"author"`
	PublishedAt      time.Time `json:"published_at" db:"published_at"`
	ProcessedAt      time.Time `json:"processed_at" db:"created_at"`
	Sentiment        float64   `json:"sentiment" db:"sentiment"` // -1.0 to 1.0
	Relevance        float64   `json:"relevance" db:"relevance"` // 0.0 to 1.0
	Impact           int       `json:"impact" db:"impact"`       // 1-10 (market impact score)
	Urgency          string    `json:"urgency" db:"urgency"`     // IMMEDIATE, HOURS, DAYS
	Keywords         []string  `json:"keywords" db:"keywords"`
	Symbols          []string  `json:"symbols"`                                          // Mentioned symbols: ["BTC", "ETH"]
	Embedding        []float32 `json:"-" db:"embedding"`                                 // Semantic embedding (1536d) - not exposed in API
	EmbeddingModel   string    `json:"-" db:"embedding_model"`                           // Model used: "ada-002" or "fallback"
	RelatedNewsIDs   []string  `json:"related_news_ids,omitempty" db:"related_news_ids"` // Similar news IDs
	ClusterID        *string   `json:"cluster_id,omitempty" db:"cluster_id"`             // Event cluster ID
	IsClusterPrimary bool      `json:"is_cluster_primary" db:"is_cluster_primary"`       // Primary in cluster
	SimilarityScore  float64   `json:"similarity_score,omitempty" db:"-"`                // Populated during search
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
