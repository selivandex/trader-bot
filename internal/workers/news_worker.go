package workers

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/alexanderselivanov/trader/internal/adapters/news"
	"github.com/alexanderselivanov/trader/pkg/logger"
	"github.com/alexanderselivanov/trader/pkg/models"
)

// NewsWorker continuously fetches and caches news in background
type NewsWorker struct {
	aggregator *news.Aggregator
	cache      *news.Cache
	interval   time.Duration
	keywords   []string
}

// NewNewsWorker creates new news worker
func NewNewsWorker(
	aggregator *news.Aggregator,
	cache *news.Cache,
	interval time.Duration,
	keywords []string,
) *NewsWorker {
	return &NewsWorker{
		aggregator: aggregator,
		cache:      cache,
		interval:   interval,
		keywords:   keywords,
	}
}

// Start starts the news worker
func (w *NewsWorker) Start(ctx context.Context) error {
	logger.Info("news worker starting",
		zap.Duration("interval", w.interval),
		zap.Strings("keywords", w.keywords),
	)

	// Run immediately on start
	w.fetchAndCache(ctx)

	// Then run periodically
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	// Cleanup ticker (once per day)
	cleanupTicker := time.NewTicker(24 * time.Hour)
	defer cleanupTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("news worker stopped")
			return ctx.Err()

		case <-ticker.C:
			w.fetchAndCache(ctx)

		case <-cleanupTicker.C:
			w.cleanup(ctx)
		}
	}
}

// fetchAndCache fetches news from providers and caches to database
func (w *NewsWorker) fetchAndCache(ctx context.Context) {
	logger.Debug("fetching news from providers...")

	startTime := time.Now()

	// Fetch news from all providers (last 6 hours)
	summary, err := w.aggregator.FetchAllNews(ctx, 6*time.Hour)
	if err != nil {
		logger.Error("failed to fetch news", zap.Error(err))
		return
	}

	if summary.TotalItems == 0 {
		logger.Debug("no new news items")
		return
	}

	// Cache news items
	if err := w.cache.Save(ctx, summary.RecentNews); err != nil {
		logger.Error("failed to cache news", zap.Error(err))
		return
	}

	duration := time.Since(startTime)

	logger.Info("news cached successfully",
		zap.Int("total_items", summary.TotalItems),
		zap.String("sentiment", summary.OverallSentiment),
		zap.Float64("score", summary.AverageSentiment),
		zap.Duration("duration", duration),
	)

	// Log sentiment breakdown
	logger.Debug("sentiment breakdown",
		zap.Int("positive", summary.PositiveCount),
		zap.Int("negative", summary.NegativeCount),
		zap.Int("neutral", summary.NeutralCount),
	)
}

// cleanup removes old news from cache
func (w *NewsWorker) cleanup(ctx context.Context) {
	logger.Info("cleaning up old news...")

	if err := w.cache.CleanupOld(ctx); err != nil {
		logger.Error("failed to cleanup news", zap.Error(err))
		return
	}

	logger.Info("old news cleanup completed")
}

// GetCachedSummary retrieves recent news summary from cache
func (w *NewsWorker) GetCachedSummary(ctx context.Context, since time.Duration) (*models.NewsSummary, error) {
	return w.cache.GetSentimentSummary(ctx, since)
}
