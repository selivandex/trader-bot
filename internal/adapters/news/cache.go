package news

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/selivandex/trader-bot/pkg/embeddings"
	"github.com/selivandex/trader-bot/pkg/logger"
	"github.com/selivandex/trader-bot/pkg/models"
)

// Cache handles news caching in database with semantic search capabilities
type Cache struct {
	repo            *Repository
	embeddingClient *embeddings.Client // Unified embedding client
}

// NewCache creates new news cache
func NewCache(repo *Repository, embeddingClient *embeddings.Client) *Cache {
	return &Cache{
		repo:            repo,
		embeddingClient: embeddingClient,
	}
}

// Save saves news items to database (upsert)
func (c *Cache) Save(ctx context.Context, news []models.NewsItem) error {
	if len(news) == 0 {
		return nil
	}

	saved, err := c.repo.SaveNewsItems(ctx, news)
	if err != nil {
		return err
	}

	logger.Debug("saved news to cache",
		zap.Int("total", len(news)),
		zap.Int("saved", saved),
	)

	return nil
}

// GetRecent gets recent news from cache
func (c *Cache) GetRecent(ctx context.Context, since time.Duration) ([]models.NewsItem, error) {
	return c.repo.GetRecentNews(ctx, since, 100)
}

// CleanupOld removes old news (7+ days old)
func (c *Cache) CleanupOld(ctx context.Context) error {
	deleted, err := c.repo.CleanupOldNews(ctx, 7*24*time.Hour)
	if err != nil {
		return err
	}

	if deleted > 0 {
		logger.Info("cleaned up old news", zap.Int64("deleted", deleted))
	}

	return nil
}

// GetSentimentSummary gets aggregated sentiment from cache
func (c *Cache) GetSentimentSummary(ctx context.Context, since time.Duration) (*models.NewsSummary, error) {
	return c.repo.GetSentimentSummary(ctx, since)
}

// SearchNews searches cached news by keywords or text query
func (c *Cache) SearchNews(ctx context.Context, query string, since time.Duration, limit int) ([]models.NewsItem, error) {
	// Get all recent news and filter by query
	allNews, err := c.repo.GetRecentNews(ctx, since, 1000)
	if err != nil {
		return nil, err
	}

	// Simple text matching (case-insensitive)
	// TODO: Use full-text search or embeddings for better results
	matched := []models.NewsItem{}
	queryLower := toLower(query)

	for _, item := range allNews {
		titleLower := toLower(item.Title)
		contentLower := toLower(item.Content)

		if contains(titleLower, queryLower) || contains(contentLower, queryLower) {
			matched = append(matched, item)
			if len(matched) >= limit {
				break
			}
		}
	}

	return matched, nil
}

// Helper functions
func toLower(s string) string {
	// Simple ASCII lowercase conversion
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c = c + ('a' - 'A')
		}
		result[i] = c
	}
	return string(result)
}

func contains(s, substr string) bool {
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ============ SEMANTIC SEARCH & CLUSTERING ============

// SearchNewsSemantics performs semantic search using embeddings
// Finds news by meaning, not just keywords
// Falls back to text search if embeddings unavailable
func (c *Cache) SearchNewsSemantics(
	ctx context.Context,
	semanticQuery string,
	since time.Duration,
	limit int,
) ([]models.NewsItem, error) {
	// Try semantic search first if embedding client available
	if c.embeddingClient != nil {
		// Generate embedding for query
		queryEmbedding, err := c.embeddingClient.Generate(ctx, semanticQuery)
		if err != nil {
			logger.Warn("⚠️ semantic search failed, falling back to text search",
				zap.Error(err),
				zap.String("query", semanticQuery),
			)
			// Fallback to text search
			return c.SearchNews(ctx, semanticQuery, since, limit)
		}

		// Vector similarity search
		news, err := c.repo.SearchNewsByVector(ctx, queryEmbedding, since, limit)
		if err != nil {
			logger.Warn("vector search failed, falling back to text search",
				zap.Error(err),
				zap.String("query", semanticQuery),
			)
			// Fallback to text search
			return c.SearchNews(ctx, semanticQuery, since, limit)
		}

		logger.Debug("semantic news search completed",
			zap.String("query", semanticQuery),
			zap.Int("found", len(news)),
		)

		return news, nil
	}

	// No embedding client - fall back to text search
	logger.Debug("embedding client not configured, using text search fallback",
		zap.String("query", semanticQuery),
	)
	return c.SearchNews(ctx, semanticQuery, since, limit)
}

// GetRepo returns underlying repository for direct access to low-level methods
func (c *Cache) GetRepo() *Repository {
	return c.repo
}

// GenerateEmbeddingsForNews generates embeddings for news items in batch
// Called by news worker after AI evaluation
// Uses batch API for 10x speed improvement
func (c *Cache) GenerateEmbeddingsForNews(ctx context.Context, news []*models.NewsItem) error {
	if len(news) == 0 {
		return nil
	}

	if c.embeddingClient == nil {
		return fmt.Errorf("embedding client not configured - cannot generate embeddings (set OPENAI_API_KEY)")
	}

	// Collect texts that need embeddings
	var textsToEmbed []string
	var newsNeedingEmbedding []*models.NewsItem

	for _, item := range news {
		// Skip if already has embedding
		if len(item.Embedding) > 0 {
			continue
		}

		// Combine title + content for embedding
		text := item.Title
		if item.Content != "" {
			text = text + ". " + item.Content
		}

		// Truncate if too long (OpenAI limit ~8k tokens)
		if len(text) > 30000 {
			text = text[:30000]
		}

		textsToEmbed = append(textsToEmbed, text)
		newsNeedingEmbedding = append(newsNeedingEmbedding, item)
	}

	if len(textsToEmbed) == 0 {
		return nil // All items already have embeddings
	}

	logger.Debug("generating embeddings in batch",
		zap.Int("batch_size", len(textsToEmbed)),
	)

	// Use unified batch generation
	embeddings, err := c.embeddingClient.GenerateBatch(ctx, textsToEmbed)
	if err != nil {
		return fmt.Errorf("batch embedding generation failed: %w", err)
	}

	// Assign embeddings back to news items
	for i, embedding := range embeddings {
		newsNeedingEmbedding[i].Embedding = embedding
		newsNeedingEmbedding[i].EmbeddingModel = "ada-002"
	}

	logger.Info("embeddings generated successfully",
		zap.Int("count", len(embeddings)),
	)

	return nil
}

// ClusterSimilarNews groups similar news items (deduplication)
// Same event from multiple sources → one cluster
func (c *Cache) ClusterSimilarNews(ctx context.Context, news []*models.NewsItem, similarityThreshold float64) error {
	if len(news) == 0 {
		return nil
	}

	clustered := 0

	for _, item := range news {
		// Skip if already clustered or no embedding
		if item.ClusterID != nil || len(item.Embedding) == 0 {
			continue
		}

		// Find similar news in last 24h
		similar, err := c.repo.FindSimilarNews(ctx, item.Embedding, similarityThreshold, 24*time.Hour, 10)
		if err != nil {
			logger.Warn("failed to find similar news",
				zap.String("news_id", item.ID),
				zap.Error(err),
			)
			continue
		}

		if len(similar) > 0 {
			// Check if any similar news already has cluster
			var clusterID string
			for _, sim := range similar {
				if sim.ClusterID != nil && sim.ID != item.ID {
					clusterID = *sim.ClusterID
					break
				}
			}

			// Create new cluster if none exists
			if clusterID == "" {
				clusterID = uuid.New().String()
			}

			// Assign cluster to this news
			// Primary = highest impact source
			isPrimary := true
			for _, sim := range similar {
				if sim.Impact > item.Impact {
					isPrimary = false
					break
				}
			}

			err = c.repo.UpdateNewsCluster(ctx, item.ID, clusterID, isPrimary)
			if err != nil {
				logger.Warn("failed to update cluster",
					zap.String("news_id", item.ID),
					zap.Error(err),
				)
				continue
			}

			item.ClusterID = &clusterID
			item.IsClusterPrimary = isPrimary
			clustered++

			logger.Debug("clustered news",
				zap.String("news_id", item.ID),
				zap.String("cluster_id", clusterID),
				zap.Int("similar_count", len(similar)),
				zap.Bool("is_primary", isPrimary),
			)
		}
	}

	if clustered > 0 {
		logger.Info("news clustering completed",
			zap.Int("total", len(news)),
			zap.Int("clustered", clustered),
		)
	}

	return nil
}

// GetClusterNews retrieves all news in same cluster (related articles)
func (c *Cache) GetClusterNews(ctx context.Context, clusterID string) ([]models.NewsItem, error) {
	return c.repo.GetClusterNews(ctx, clusterID)
}

// ============ EMBEDDING COVERAGE MONITORING ============

// GetEmbeddingCoverage calculates percentage of news with embeddings in time window
// Used for health checks and monitoring
func (c *Cache) GetEmbeddingCoverage(ctx context.Context, since time.Duration) (float64, error) {
	news, err := c.repo.GetRecentNews(ctx, since, 1000)
	if err != nil {
		return 0, fmt.Errorf("failed to get recent news: %w", err)
	}

	if len(news) == 0 {
		return 1.0, nil // No news = 100% coverage
	}

	embeddedCount := 0
	for _, item := range news {
		if len(item.Embedding) > 0 {
			embeddedCount++
		}
	}

	coverage := float64(embeddedCount) / float64(len(news))

	logger.Debug("embedding coverage calculated",
		zap.Int("total", len(news)),
		zap.Int("with_embeddings", embeddedCount),
		zap.Float64("coverage", coverage),
	)

	return coverage, nil
}
