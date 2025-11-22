package news

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/lib/pq"
	"go.uber.org/zap"

	"github.com/alexanderselivanov/trader/internal/adapters/database"
	"github.com/alexanderselivanov/trader/pkg/logger"
	"github.com/alexanderselivanov/trader/pkg/models"
)

// Cache handles news caching in database
type Cache struct {
	db *database.DB
}

// NewCache creates new news cache
func NewCache(db *database.DB) *Cache {
	return &Cache{db: db}
}

// Save saves news items to database (upsert)
func (c *Cache) Save(ctx context.Context, news []models.NewsItem) error {
	if len(news) == 0 {
		return nil
	}
	
	tx, err := c.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()
	
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO news_items (
			external_id, source, title, content, url, author,
			published_at, sentiment, relevance, keywords, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (external_id) DO UPDATE SET
			sentiment = EXCLUDED.sentiment,
			relevance = EXCLUDED.relevance
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()
	
	saved := 0
	for _, item := range news {
		_, err := stmt.ExecContext(ctx,
			item.ID,
			item.Source,
			item.Title,
			item.Content,
			item.URL,
			item.Author,
			item.PublishedAt,
			item.Sentiment,
			item.Relevance,
			pq.Array(item.Keywords),
			time.Now(),
		)
		
		if err != nil {
			logger.Warn("failed to save news item",
				zap.String("id", item.ID),
				zap.Error(err),
			)
			continue
		}
		saved++
	}
	
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}
	
	logger.Debug("saved news to cache",
		zap.Int("total", len(news)),
		zap.Int("saved", saved),
	)
	
	return nil
}

// GetRecent gets recent news from cache
func (c *Cache) GetRecent(ctx context.Context, since time.Duration) ([]models.NewsItem, error) {
	cutoff := time.Now().Add(-since)
	
	rows, err := c.db.Conn().QueryContext(ctx, `
		SELECT 
			external_id, source, title, content, url, author,
			published_at, sentiment, relevance, keywords
		FROM news_items
		WHERE published_at > $1
		ORDER BY published_at DESC, relevance DESC
		LIMIT 100
	`, cutoff)
	
	if err != nil {
		return nil, fmt.Errorf("failed to query news: %w", err)
	}
	defer rows.Close()
	
	news := make([]models.NewsItem, 0)
	for rows.Next() {
		var item models.NewsItem
		var keywords pq.StringArray
		
		err := rows.Scan(
			&item.ID,
			&item.Source,
			&item.Title,
			&item.Content,
			&item.URL,
			&item.Author,
			&item.PublishedAt,
			&item.Sentiment,
			&item.Relevance,
			&keywords,
		)
		
		if err != nil {
			logger.Warn("failed to scan news row", zap.Error(err))
			continue
		}
		
		item.Keywords = keywords
		news = append(news, item)
	}
	
	return news, nil
}

// CleanupOld removes old news (7+ days old)
func (c *Cache) CleanupOld(ctx context.Context) error {
	result, err := c.db.Conn().ExecContext(ctx, `
		DELETE FROM news_items
		WHERE published_at < NOW() - INTERVAL '7 days'
	`)
	
	if err != nil {
		return err
	}
	
	rows, _ := result.RowsAffected()
	if rows > 0 {
		logger.Info("cleaned up old news", zap.Int64("deleted", rows))
	}
	
	return nil
}

// GetSentimentSummary gets aggregated sentiment from cache
func (c *Cache) GetSentimentSummary(ctx context.Context, since time.Duration) (*models.NewsSummary, error) {
	cutoff := time.Now().Add(-since)
	
	row := c.db.Conn().QueryRowContext(ctx, `
		SELECT 
			COUNT(*) as total,
			COUNT(*) FILTER (WHERE sentiment > 0.2) as positive,
			COUNT(*) FILTER (WHERE sentiment < -0.2) as negative,
			COUNT(*) FILTER (WHERE sentiment BETWEEN -0.2 AND 0.2) as neutral,
			COALESCE(AVG(sentiment), 0) as avg_sentiment
		FROM news_items
		WHERE published_at > $1
	`, cutoff)
	
	var total, positive, negative, neutral int
	var avgSentiment float64
	
	if err := row.Scan(&total, &positive, &negative, &neutral, &avgSentiment); err != nil {
		return nil, err
	}
	
	// Get top 5 recent items
	recentNews, err := c.GetRecent(ctx, since)
	if err != nil {
		return nil, err
	}
	
	if len(recentNews) > 5 {
		recentNews = recentNews[:5]
	}
	
	summary := &models.NewsSummary{
		TotalItems:       total,
		PositiveCount:    positive,
		NegativeCount:    negative,
		NeutralCount:     neutral,
		AverageSentiment: avgSentiment,
		RecentNews:       recentNews,
		UpdatedAt:        time.Now(),
	}
	
	summary.OverallSentiment = summary.GetOverallSentiment()
	
	return summary, nil
}

