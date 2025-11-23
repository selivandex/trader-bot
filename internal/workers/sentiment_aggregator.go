package workers

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/selivandex/trader-bot/internal/adapters/news"
	"github.com/selivandex/trader-bot/pkg/logger"
	"github.com/selivandex/trader-bot/pkg/models"
)

// SentimentAggregator calculates rolling sentiment metrics with impact weighting
type SentimentAggregator struct {
	repo     *Repository
	newsRepo *news.Repository
	interval time.Duration
	cache    *SentimentCache
}

// SentimentCache caches current sentiment in memory
type SentimentCache struct {
	current   *models.AggregatedSentiment
	trend     *models.SentimentTrend
	updatedAt time.Time
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
func NewSentimentAggregator(repo *Repository, newsRepo *news.Repository, interval time.Duration) *SentimentAggregator {
	return &SentimentAggregator{
		repo:     repo,
		newsRepo: newsRepo,
		interval: interval,
		cache:    &SentimentCache{},
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

// calculateMetrics calculates weighted sentiment with impact scores
func (sa *SentimentAggregator) calculateMetrics(ctx context.Context) {
	logger.Debug("calculating weighted sentiment metrics...")

	// Get weighted sentiment (impact-weighted)
	weighted, err := sa.newsRepo.GetWeightedSentiment(ctx, 6*time.Hour)
	if err != nil {
		logger.Error("failed to calculate sentiment", zap.Error(err))
		return
	}

	// Create aggregated sentiment
	aggregated := &models.AggregatedSentiment{
		Timestamp:        time.Now(),
		BullishScore:     weighted.BullishScore,
		BearishScore:     weighted.BearishScore,
		NetSentiment:     weighted.NetSentiment,
		NewsCount:        weighted.NewsCount,
		HighImpactCount:  weighted.HighImpactCount,
		AverageSentiment: weighted.AverageSentiment,
	}

	// Get high impact news
	highImpactNews, err := sa.newsRepo.GetHighImpactNews(ctx, 7, 24*time.Hour, 10)
	if err == nil {
		aggregated.HighImpactNews = highImpactNews
	}

	// Save snapshot to database
	if err := sa.repo.SaveSentimentSnapshot(ctx, aggregated); err != nil {
		logger.Error("failed to save sentiment snapshot", zap.Error(err))
	}

	// Update cache
	sa.cache.current = aggregated
	sa.cache.updatedAt = time.Now()

	// Calculate trend
	trend := sa.calculateTrend(ctx)
	sa.cache.trend = trend

	logger.Info("sentiment metrics calculated",
		zap.Float64("bullish_score", weighted.BullishScore),
		zap.Float64("bearish_score", weighted.BearishScore),
		zap.Float64("net_sentiment", weighted.NetSentiment),
		zap.Int("news_count", weighted.NewsCount),
		zap.Int("high_impact", weighted.HighImpactCount),
		zap.String("trend", trend.Direction),
	)
}

// calculateTrend calculates sentiment trend from recent snapshots
func (sa *SentimentAggregator) calculateTrend(ctx context.Context) *models.SentimentTrend {
	datapoints, err := sa.repo.GetRecentSentimentSnapshots(ctx, 12)
	if err != nil || len(datapoints) < 2 {
		return &models.SentimentTrend{Direction: "stable"}
	}

	current := datapoints[0]
	previous := datapoints[1]
	momentum := current - previous

	direction := "stable"
	if momentum > 5 {
		direction = "improving"
	} else if momentum < -5 {
		direction = "declining"
	}

	return &models.SentimentTrend{
		Current:    current,
		Previous:   previous,
		Direction:  direction,
		Momentum:   momentum,
		Datapoints: datapoints,
	}
}

// GetMetrics returns current sentiment metrics
func (sa *SentimentAggregator) GetMetrics(ctx context.Context) (*SentimentMetrics, error) {
	ts, err := sa.newsRepo.GetSentimentTimeSeries(ctx)
	if err != nil {
		return nil, err
	}

	momentum := ts.LastHour - ts.Last6Hours

	var trend string
	if momentum > 0.1 {
		trend = "improving"
	} else if momentum < -0.1 {
		trend = "declining"
	} else {
		trend = "stable"
	}

	var direction string
	if ts.Last6Hours > 0.2 {
		direction = "bullish"
	} else if ts.Last6Hours < -0.2 {
		direction = "bearish"
	} else {
		direction = "neutral"
	}

	return &SentimentMetrics{
		CurrentSentiment:  ts.LastHour,
		SentimentTrend:    trend,
		SentimentMomentum: momentum,
		LastHourAvg:       ts.LastHour,
		Last6HoursAvg:     ts.Last6Hours,
		Last24HoursAvg:    ts.Last24Hours,
		TrendDirection:    direction,
		UpdatedAt:         time.Now(),
	}, nil
}
