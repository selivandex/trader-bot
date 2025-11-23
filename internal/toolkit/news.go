package toolkit

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/selivandex/trader-bot/pkg/logger"
	"github.com/selivandex/trader-bot/pkg/models"
)

// ============ Additional News Tools Implementation ============

// GetLatestNews gets most recent news articles
func (t *LocalToolkit) GetLatestNews(ctx context.Context, limit int) ([]models.NewsItem, error) {
	logger.Debug("toolkit: get_latest_news",
		zap.String("agent_id", t.agentID),
		zap.Int("limit", limit),
	)

	// Get recent news from cache (last 24 hours by default)
	news, err := t.newsCache.GetRecent(ctx, 24*time.Hour)
	if err != nil {
		return nil, err
	}

	// Return top N most recent
	if len(news) > limit {
		news = news[:limit]
	}

	return news, nil
}

// GetNewsBySource filters news by source
func (t *LocalToolkit) GetNewsBySource(ctx context.Context, source string, since time.Duration, limit int) ([]models.NewsItem, error) {
	logger.Debug("toolkit: get_news_by_source",
		zap.String("agent_id", t.agentID),
		zap.String("source", source),
		zap.Duration("since", since),
		zap.Int("limit", limit),
	)

	// Get all recent news
	allNews, err := t.newsCache.GetRecent(ctx, since)
	if err != nil {
		return nil, err
	}

	// Filter by source
	filtered := []models.NewsItem{}
	for _, item := range allNews {
		if item.Source == source {
			filtered = append(filtered, item)
			if len(filtered) >= limit {
				break
			}
		}
	}

	return filtered, nil
}

// CountNewsBySentiment counts positive/negative/neutral news
func (t *LocalToolkit) CountNewsBySentiment(ctx context.Context, since time.Duration) (int, int, int, error) {
	logger.Debug("toolkit: count_news_by_sentiment",
		zap.String("agent_id", t.agentID),
		zap.Duration("since", since),
	)

	news, err := t.newsCache.GetRecent(ctx, since)
	if err != nil {
		return 0, 0, 0, err
	}

	var positive, negative, neutral int

	for _, item := range news {
		if item.Sentiment > 0.2 {
			positive++
		} else if item.Sentiment < -0.2 {
			negative++
		} else {
			neutral++
		}
	}

	return positive, negative, neutral, nil
}

// ============ SEMANTIC SEARCH & INTELLIGENCE ============

// SearchNewsSemantics searches news by semantic meaning, not just keywords
// Example: "regulatory problems" finds "SEC lawsuit", "government scrutiny", etc.
func (t *LocalToolkit) SearchNewsSemantics(ctx context.Context, semanticQuery string, since time.Duration, limit int) ([]models.NewsItem, error) {
	startTime := time.Now()

	logger.Debug("toolkit: search_news_semantics",
		zap.String("agent_id", t.agentID),
		zap.String("query", semanticQuery),
		zap.Duration("since", since),
		zap.Int("limit", limit),
	)

	news, err := t.newsCache.SearchNewsSemantics(ctx, semanticQuery, since, limit)
	if err != nil {
		return nil, err
	}

	executionTime := int(time.Since(startTime).Milliseconds())
	avgSimilarity := calculateAvgSimilarity(news)

	logger.Info("semantic search completed",
		zap.String("agent_id", t.agentID),
		zap.String("query", semanticQuery),
		zap.Int("found", len(news)),
		zap.Float32("avg_similarity", avgSimilarity),
		zap.Int("ms", executionTime),
	)

	// Log metrics to ClickHouse (non-blocking)
	if t.metricsLogger != nil {
		t.metricsLogger.LogToolUsage(
			ctx,
			"SearchNewsSemantics",
			semanticQuery,
			len(news),
			avgSimilarity,
			len(news) > 0, // Consider useful if found results
			executionTime,
		)
	}

	return news, nil
}

// GetRelatedNews finds news related to a specific news item (same event/topic)
func (t *LocalToolkit) GetRelatedNews(ctx context.Context, clusterID string) ([]models.NewsItem, error) {
	logger.Debug("toolkit: get_related_news",
		zap.String("agent_id", t.agentID),
		zap.String("cluster_id", clusterID),
	)

	news, err := t.newsCache.GetClusterNews(ctx, clusterID)
	if err != nil {
		return nil, err
	}

	logger.Debug("related news retrieved",
		zap.String("agent_id", t.agentID),
		zap.Int("count", len(news)),
	)

	return news, nil
}

// FindNewsForMemory finds current news semantically related to a past memory
// Use case: Agent recalls "Last time SEC sued exchange..." and wants current context
func (t *LocalToolkit) FindNewsForMemory(ctx context.Context, memoryID string, hours time.Duration, limit int) ([]models.NewsItem, error) {
	logger.Debug("toolkit: find_news_for_memory",
		zap.String("agent_id", t.agentID),
		zap.String("memory_id", memoryID),
		zap.Duration("hours", hours),
		zap.Int("limit", limit),
	)

	// Get the memory
	memories, err := t.memoryManager.GetAllMemories(ctx, t.agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get memories: %w", err)
	}

	// Find the specific memory
	var targetMemory *models.SemanticMemory
	for i := range memories {
		if memories[i].ID == memoryID {
			targetMemory = &memories[i]
			break
		}
	}

	if targetMemory == nil {
		return nil, fmt.Errorf("memory not found: %s", memoryID)
	}

	if len(targetMemory.Embedding) == 0 {
		return nil, fmt.Errorf("memory has no embedding")
	}

	// Use memory's embedding to find semantically related current news
	news, err := t.newsCache.GetRepo().SearchNewsByVector(ctx, targetMemory.Embedding, hours, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search news for memory: %w", err)
	}

	logger.Info("found news related to memory",
		zap.String("agent_id", t.agentID),
		zap.String("memory_lesson", targetMemory.Lesson),
		zap.Int("news_count", len(news)),
	)

	return news, nil
}

// calculateAvgSimilarity calculates average similarity score from news results
func calculateAvgSimilarity(news []models.NewsItem) float32 {
	if len(news) == 0 {
		return 0
	}

	var sum float64
	count := 0

	for _, item := range news {
		if item.SimilarityScore > 0 {
			sum += item.SimilarityScore
			count++
		}
	}

	if count == 0 {
		return 0
	}

	return float32(sum / float64(count))
}
