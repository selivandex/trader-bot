package workers

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/alexanderselivanov/trader/internal/adapters/database"
	"github.com/alexanderselivanov/trader/pkg/logger"
)

// SentimentAggregator calculates rolling sentiment metrics
type SentimentAggregator struct {
	db       *database.DB
	interval time.Duration
}

// SentimentMetrics represents aggregated sentiment over time
type SentimentMetrics struct {
	CurrentSentiment  float64   `json:"current_sentiment"`
	SentimentTrend    string    `json:"sentiment_trend"`    // improving, declining, stable
	SentimentMomentum float64   `json:"sentiment_momentum"` // rate of change
	LastHourAvg       float64   `json:"last_hour_avg"`
	Last6HoursAvg     float64   `json:"last_6hours_avg"`
	Last24HoursAvg    float64   `json:"last_24hours_avg"`
	TrendDirection    string    `json:"trend_direction"` // bullish, bearish, neutral
	UpdatedAt         time.Time `json:"updated_at"`
}

// NewSentimentAggregator creates new sentiment aggregator
func NewSentimentAggregator(db *database.DB, interval time.Duration) *SentimentAggregator {
	return &SentimentAggregator{
		db:       db,
		interval: interval,
	}
}

// Start starts the sentiment aggregator
func (sa *SentimentAggregator) Start(ctx context.Context) error {
	logger.Info("sentiment aggregator starting",
		zap.Duration("interval", sa.interval),
	)

	ticker := time.NewTicker(sa.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("sentiment aggregator stopped")
			return ctx.Err()

		case <-ticker.C:
			sa.calculateMetrics(ctx)
		}
	}
}

// calculateMetrics calculates rolling sentiment metrics
func (sa *SentimentAggregator) calculateMetrics(ctx context.Context) {
	logger.Debug("calculating sentiment metrics...")

	// Get sentiment averages for different time windows
	row := sa.db.Conn().QueryRowContext(ctx, `
		SELECT 
			COALESCE(AVG(sentiment) FILTER (WHERE published_at > NOW() - INTERVAL '1 hour'), 0) as last_hour,
			COALESCE(AVG(sentiment) FILTER (WHERE published_at > NOW() - INTERVAL '6 hours'), 0) as last_6hours,
			COALESCE(AVG(sentiment) FILTER (WHERE published_at > NOW() - INTERVAL '24 hours'), 0) as last_24hours
		FROM news_items
		WHERE published_at > NOW() - INTERVAL '24 hours'
	`)

	var lastHour, last6Hours, last24Hours float64
	if err := row.Scan(&lastHour, &last6Hours, &last24Hours); err != nil {
		logger.Error("failed to calculate sentiment metrics", zap.Error(err))
		return
	}

	// Calculate momentum (sentiment is improving or declining)
	momentum := lastHour - last6Hours

	// Determine trend
	var trend string
	if momentum > 0.1 {
		trend = "improving"
	} else if momentum < -0.1 {
		trend = "declining"
	} else {
		trend = "stable"
	}

	// Determine direction
	var direction string
	if last6Hours > 0.2 {
		direction = "bullish"
	} else if last6Hours < -0.2 {
		direction = "bearish"
	} else {
		direction = "neutral"
	}

	logger.Info("sentiment metrics calculated",
		zap.Float64("1h_avg", lastHour),
		zap.Float64("6h_avg", last6Hours),
		zap.Float64("24h_avg", last24Hours),
		zap.Float64("momentum", momentum),
		zap.String("trend", trend),
		zap.String("direction", direction),
	)
}

// GetMetrics returns current sentiment metrics
func (sa *SentimentAggregator) GetMetrics(ctx context.Context) (*SentimentMetrics, error) {
	row := sa.db.Conn().QueryRowContext(ctx, `
		SELECT 
			COALESCE(AVG(sentiment) FILTER (WHERE published_at > NOW() - INTERVAL '1 hour'), 0),
			COALESCE(AVG(sentiment) FILTER (WHERE published_at > NOW() - INTERVAL '6 hours'), 0),
			COALESCE(AVG(sentiment) FILTER (WHERE published_at > NOW() - INTERVAL '24 hours'), 0)
		FROM news_items
	`)

	var lastHour, last6Hours, last24Hours float64
	if err := row.Scan(&lastHour, &last6Hours, &last24Hours); err != nil {
		return nil, err
	}

	momentum := lastHour - last6Hours

	var trend string
	if momentum > 0.1 {
		trend = "improving"
	} else if momentum < -0.1 {
		trend = "declining"
	} else {
		trend = "stable"
	}

	var direction string
	if last6Hours > 0.2 {
		direction = "bullish"
	} else if last6Hours < -0.2 {
		direction = "bearish"
	} else {
		direction = "neutral"
	}

	return &SentimentMetrics{
		CurrentSentiment:  lastHour,
		SentimentTrend:    trend,
		SentimentMomentum: momentum,
		LastHourAvg:       lastHour,
		Last6HoursAvg:     last6Hours,
		Last24HoursAvg:    last24Hours,
		TrendDirection:    direction,
		UpdatedAt:         time.Now(),
	}, nil
}
