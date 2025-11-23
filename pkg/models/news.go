package models

import "time"

// NewsItem represents single news item
type NewsItem struct {
	PublishedAt      time.Time `json:"published_at" db:"published_at"`
	ProcessedAt      time.Time `json:"processed_at" db:"created_at"`
	ClusterID        *string   `json:"cluster_id,omitempty" db:"cluster_id"`
	EmbeddingModel   string    `json:"-" db:"embedding_model"`
	Urgency          string    `json:"urgency" db:"urgency"`
	Author           string    `json:"author" db:"author"`
	Content          string    `json:"content" db:"content"`
	Title            string    `json:"title" db:"title"`
	Source           string    `json:"source" db:"source"`
	URL              string    `json:"url" db:"url"`
	ID               string    `json:"id" db:"id"`
	RelatedNewsIDs   []string  `json:"related_news_ids,omitempty" db:"related_news_ids"`
	Keywords         []string  `json:"keywords" db:"keywords"`
	Symbols          []string  `json:"symbols"`
	Embedding        []float32 `json:"-" db:"embedding"`
	Impact           int       `json:"impact" db:"impact"`
	Relevance        float64   `json:"relevance" db:"relevance"`
	Sentiment        float64   `json:"sentiment" db:"sentiment"`
	SimilarityScore  float64   `json:"similarity_score,omitempty" db:"-"`
	IsClusterPrimary bool      `json:"is_cluster_primary" db:"is_cluster_primary"`
}

// NewsSummary aggregates news sentiment
type NewsSummary struct {
	UpdatedAt        time.Time  `json:"updated_at"`
	OverallSentiment string     `json:"overall_sentiment"`
	RecentNews       []NewsItem `json:"recent_news"`
	TotalItems       int        `json:"total_items"`
	PositiveCount    int        `json:"positive_count"`
	NegativeCount    int        `json:"negative_count"`
	NeutralCount     int        `json:"neutral_count"`
	AverageSentiment float64    `json:"average_sentiment"`
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
