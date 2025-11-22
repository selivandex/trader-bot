package news

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/alexanderselivanov/trader/pkg/logger"
	"github.com/alexanderselivanov/trader/pkg/models"
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

