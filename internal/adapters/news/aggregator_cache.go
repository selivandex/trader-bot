package news

import (
	"context"
	"time"

	"github.com/alexanderselivanov/trader/pkg/models"
)

// GetCachedSummary gets news summary from cache
func (a *Aggregator) GetCachedSummary(ctx context.Context, since time.Duration) (*models.NewsSummary, error) {
	if !a.useCache {
		// Fallback to fetching if no cache
		return a.FetchAllNews(ctx, since)
	}
	
	return a.cache.GetSentimentSummary(ctx, since)
}

// GetCachedNews gets recent news items from cache
func (a *Aggregator) GetCachedNews(ctx context.Context, since time.Duration, limit int) ([]models.NewsItem, error) {
	if !a.useCache {
		return nil, nil
	}
	
	allNews, err := a.cache.GetRecent(ctx, since)
	if err != nil {
		return nil, err
	}
	
	if limit > 0 && len(allNews) > limit {
		return allNews[:limit], nil
	}
	
	return allNews, nil
}

