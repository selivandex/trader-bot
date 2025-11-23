package models

import "time"

// AggregatedSentiment represents sentiment over time period
type AggregatedSentiment struct {
	Timestamp        time.Time  `json:"timestamp" db:"timestamp"`
	HighImpactNews   []NewsItem `json:"high_impact_news,omitempty"`
	ID               int64      `json:"id" db:"id"`
	BullishScore     float64    `json:"bullish_score" db:"bullish_score"`
	BearishScore     float64    `json:"bearish_score" db:"bearish_score"`
	NetSentiment     float64    `json:"net_sentiment" db:"net_sentiment"`
	NewsCount        int        `json:"news_count" db:"news_count"`
	HighImpactCount  int        `json:"high_impact_count" db:"high_impact_count"`
	AverageSentiment float64    `json:"average_sentiment" db:"average_sentiment"`
}

// SentimentTrend represents sentiment trend over time
type SentimentTrend struct {
	Direction  string    `json:"direction"`
	Datapoints []float64 `json:"datapoints"`
	Current    float64   `json:"current"`
	Previous   float64   `json:"previous"`
	Momentum   float64   `json:"momentum"`
}

// GetTrend calculates sentiment trend from historical data
func GetSentimentTrend(sentiments []AggregatedSentiment) *SentimentTrend {
	if len(sentiments) < 2 {
		return &SentimentTrend{
			Direction: "stable",
			Momentum:  0,
		}
	}

	current := sentiments[0].NetSentiment
	previous := sentiments[1].NetSentiment
	momentum := current - previous

	direction := "stable"
	if momentum > 5 {
		direction = "improving"
	} else if momentum < -5 {
		direction = "declining"
	}

	// Extract datapoints
	datapoints := make([]float64, len(sentiments))
	for i, s := range sentiments {
		datapoints[i] = s.NetSentiment
	}

	return &SentimentTrend{
		Current:    current,
		Previous:   previous,
		Direction:  direction,
		Momentum:   momentum,
		Datapoints: datapoints,
	}
}
