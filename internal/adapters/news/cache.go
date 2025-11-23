package news

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/selivandex/trader-bot/pkg/logger"
	"github.com/selivandex/trader-bot/pkg/models"
)

// Cache handles news caching in database
type Cache struct {
	repo *Repository
}

// NewCache creates new news cache
func NewCache(repo *Repository) *Cache {
	return &Cache{repo: repo}
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
