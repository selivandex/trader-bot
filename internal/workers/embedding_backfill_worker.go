package workers

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/selivandex/trader-bot/internal/adapters/news"
	"github.com/selivandex/trader-bot/pkg/logger"
	"github.com/selivandex/trader-bot/pkg/models"
)

// EmbeddingBackfillWorker fills in missing embeddings for news items
// Runs every hour to catch any items that failed during initial processing
type EmbeddingBackfillWorker struct {
	cache     *news.Cache
	repo      *news.Repository
	interval  time.Duration
	batchSize int
}

// NewEmbeddingBackfillWorker creates new backfill worker
func NewEmbeddingBackfillWorker(
	cache *news.Cache,
	repo *news.Repository,
	interval time.Duration,
) *EmbeddingBackfillWorker {
	return &EmbeddingBackfillWorker{
		cache:     cache,
		repo:      repo,
		interval:  interval,
		batchSize: 50, // Process 50 items at a time
	}
}

// Name returns worker name
func (w *EmbeddingBackfillWorker) Name() string {
	return "embedding_backfill"
}

// Run executes one iteration - backfills missing embeddings
// Called periodically by pkg/worker.PeriodicWorker
func (w *EmbeddingBackfillWorker) Run(ctx context.Context) error {
	w.backfillMissingEmbeddings(ctx)
	return nil
}

// backfillMissingEmbeddings finds news without embeddings and generates them
func (w *EmbeddingBackfillWorker) backfillMissingEmbeddings(ctx context.Context) {
	logger.Debug("checking for news without embeddings...")

	startTime := time.Now()

	// Get news without embeddings from last 7 days
	news, err := w.repo.GetNewsWithoutEmbeddings(ctx, 7*24*time.Hour, w.batchSize)
	if err != nil {
		logger.Error("failed to get news without embeddings", zap.Error(err))
		return
	}

	if len(news) == 0 {
		logger.Debug("no news items need embedding backfill")
		return
	}

	logger.Info("found news items without embeddings",
		zap.Int("count", len(news)),
	)

	// Convert to pointers for batch generation
	newsPointers := make([]*models.NewsItem, len(news))
	for i := range news {
		newsPointers[i] = &news[i]
	}

	// Generate embeddings in batch
	if err := w.cache.GenerateEmbeddingsForNews(ctx, newsPointers); err != nil {
		logger.Error("backfill embedding generation failed",
			zap.Int("batch_size", len(newsPointers)),
			zap.Error(err),
		)
		return
	}

	// Cluster similar news
	if err := w.cache.ClusterSimilarNews(ctx, newsPointers, 0.85); err != nil {
		logger.Warn("backfill clustering failed", zap.Error(err))
		// Non-critical, continue
	}

	// Save updated news items with embeddings
	updatedNews := make([]models.NewsItem, len(newsPointers))
	for i, ptr := range newsPointers {
		updatedNews[i] = *ptr
	}

	if err := w.cache.Save(ctx, updatedNews); err != nil {
		logger.Error("failed to save backfilled embeddings", zap.Error(err))
		return
	}

	duration := time.Since(startTime)

	logger.Info("✅ embedding backfill completed successfully",
		zap.Int("items_processed", len(news)),
		zap.Duration("duration", duration),
	)

	// Log statistics
	stats, err := w.repo.GetEmbeddingStats(ctx, 24*time.Hour)
	if err == nil {
		logger.Info("embedding coverage statistics",
			zap.Int("total_news_24h", stats.TotalNews),
			zap.Int("with_embeddings", stats.WithEmbeddings),
			zap.Int("without_embeddings", stats.WithoutEmbeddings),
			zap.Float64("coverage_percent", stats.CoveragePercent),
		)
	}
}
