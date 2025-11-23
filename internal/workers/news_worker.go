package workers

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/selivandex/trader-bot/internal/adapters/ai"
	"github.com/selivandex/trader-bot/internal/adapters/news"
	"github.com/selivandex/trader-bot/pkg/logger"
	"github.com/selivandex/trader-bot/pkg/models"
)

// NewsWorker continuously fetches and caches news in background
// Only uses AI evaluation, no keyword-based scoring
type NewsWorker struct {
	newsEvaluator   ai.NewsEvaluatorInterface
	aggregator      *news.Aggregator
	cache           *news.Cache
	keywords        []string
	interval        time.Duration
	useAIEvaluation bool
}

// NewNewsWorker creates new news worker
// Requires AI evaluator - no keyword-based scoring
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
		newsEvaluator:   newsEvaluator,
		useAIEvaluation: newsEvaluator != nil,
		interval:        interval,
		keywords:        keywords,
	}
}

// Name returns worker name
func (w *NewsWorker) Name() string {
	return "news_fetcher"
}

// Run executes one iteration - fetches and caches news
// Called periodically by pkg/worker.PeriodicWorker
// Note: cleanup runs only when called from main every 24h
func (w *NewsWorker) Run(ctx context.Context) error {
	w.fetchAndCache(ctx)
	return nil
}

// RunWithCleanup runs fetch and cleanup together (for 24h periodic call)
func (w *NewsWorker) RunWithCleanup(ctx context.Context) error {
	w.fetchAndCache(ctx)
	w.cleanup(ctx)
	return nil
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

	// Evaluate news with AI (batch processing)
	if !w.useAIEvaluation {
		logger.Warn("AI evaluation disabled, news will be saved without impact scores")
	}

	// Batch evaluate all news items (more efficient than one-by-one)
	if w.useAIEvaluation && w.newsEvaluator != nil {
		// Convert to pointer slice for batch evaluation
		newsPointers := make([]*models.NewsItem, len(summary.RecentNews))
		for i := range summary.RecentNews {
			newsPointers[i] = &summary.RecentNews[i]
		}

		if err := w.newsEvaluator.EvaluateNewsBatch(ctx, newsPointers); err != nil {
			logger.Error("AI batch news evaluation failed",
				zap.Int("count", len(newsPointers)),
				zap.Error(err),
			)
			// Don't return - save news even without AI scores
		} else {
			logger.Info("news batch evaluated successfully",
				zap.Int("count", len(newsPointers)),
			)
		}
	}

	// All news items are now evaluated (or have default scores)
	evaluatedNews := summary.RecentNews

	// Generate embeddings for semantic search
	newsPointers := make([]*models.NewsItem, len(evaluatedNews))
	for i := range evaluatedNews {
		newsPointers[i] = &evaluatedNews[i]
	}

	if err := w.cache.GenerateEmbeddingsForNews(ctx, newsPointers); err != nil {
		logger.Error("failed to generate embeddings", zap.Error(err))
		// Don't return - save news even without embeddings
	} else {
		logger.Debug("embeddings generated successfully",
			zap.Int("count", len(newsPointers)),
		)
	}

	// Cluster similar news (deduplication) - 0.85 similarity threshold
	if err := w.cache.ClusterSimilarNews(ctx, newsPointers, 0.85); err != nil {
		logger.Warn("failed to cluster news", zap.Error(err))
		// Non-critical, continue
	}

	// Cache only successfully evaluated news items
	if err := w.cache.Save(ctx, evaluatedNews); err != nil {
		logger.Error("failed to cache news", zap.Error(err))
		return
	}

	// Log high impact news
	highImpact := 0
	for _, item := range evaluatedNews {
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
		zap.Int("total_fetched", summary.TotalItems),
		zap.Int("cached", len(evaluatedNews)),
		zap.Int("high_impact", highImpact),
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
