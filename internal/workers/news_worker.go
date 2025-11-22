package workers

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/alexanderselivanov/trader/internal/adapters/ai"
	"github.com/alexanderselivanov/trader/internal/adapters/news"
	"github.com/alexanderselivanov/trader/internal/sentiment"
	"github.com/alexanderselivanov/trader/pkg/logger"
	"github.com/alexanderselivanov/trader/pkg/models"
)

// NewsWorker continuously fetches and caches news in background
type NewsWorker struct {
	aggregator      *news.Aggregator
	cache           *news.Cache
	impactScorer    *sentiment.ImpactScorer
	newsEvaluator   ai.NewsEvaluatorInterface
	useAIEvaluation bool
	interval        time.Duration
	keywords        []string
}

// NewNewsWorker creates new news worker
func NewNewsWorker(
	aggregator *news.Aggregator,
	cache *news.Cache,
	newsEvaluator ai.NewsEvaluatorInterface,
	interval time.Duration,
	keywords []string,
) *NewsWorker {
	return &NewsWorker{
		aggregator:      aggregator,
		cache:           cache,
		impactScorer:    sentiment.NewImpactScorer(),
		newsEvaluator:   newsEvaluator,
		useAIEvaluation: newsEvaluator != nil,
		interval:        interval,
		keywords:        keywords,
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

	// Score impact for each news item
	for i := range summary.RecentNews {
		if w.useAIEvaluation {
			// Use AI to evaluate news (more accurate but costs API calls)
			if err := w.newsEvaluator.EvaluateNews(ctx, &summary.RecentNews[i]); err != nil {
				logger.Warn("AI news evaluation failed, using keyword-based",
					zap.String("title", summary.RecentNews[i].Title),
					zap.Error(err),
				)
				// Fallback to keyword scoring
				w.impactScorer.ScoreNewsItem(&summary.RecentNews[i])
			}
		} else {
			// Use keyword-based scoring (faster, free)
			w.impactScorer.ScoreNewsItem(&summary.RecentNews[i])
		}
	}
	
	// Cache news items with impact scores
	if err := w.cache.Save(ctx, summary.RecentNews); err != nil {
		logger.Error("failed to cache news", zap.Error(err))
		return
	}
	
	// Log high impact news
	highImpact := 0
	for _, item := range summary.RecentNews {
		if item.Impact >= 7 {
			highImpact++
			logger.Info("high impact news detected",
				zap.String("title", item.Title),
				zap.Int("impact", item.Impact),
				zap.String("urgency", item.Urgency),
				zap.Float64("sentiment", item.Sentiment),
			)
		}
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
