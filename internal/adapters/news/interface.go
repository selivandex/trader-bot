package news

import (
	"context"
	"time"

	"github.com/selivandex/trader-bot/pkg/models"
)

// Provider represents news source provider interface
type Provider interface {
	// GetName returns provider name
	GetName() string

	// FetchLatestNews fetches latest news items
	FetchLatestNews(ctx context.Context, keywords []string, limit int) ([]models.NewsItem, error)

	// IsEnabled returns whether provider is enabled
	IsEnabled() bool
}

// Aggregator aggregates news from multiple sources
type Aggregator struct {
	cache     *Cache
	providers []Provider
	keywords  []string
	useCache  bool
}

// NewAggregator creates new news aggregator
func NewAggregator(providers []Provider, keywords []string, cache *Cache) *Aggregator {
	return &Aggregator{
		providers: providers,
		keywords:  keywords,
		cache:     cache,
		useCache:  cache != nil,
	}
}

// FetchAllNews fetches news from all enabled providers
func (a *Aggregator) FetchAllNews(ctx context.Context, since time.Duration) (*models.NewsSummary, error) {
	allNews := make([]models.NewsItem, 0)

	// Query all providers in parallel
	type result struct {
		err  error
		news []models.NewsItem
	}

	results := make(chan result, len(a.providers))
	enabledCount := 0

	for _, provider := range a.providers {
		if !provider.IsEnabled() {
			continue
		}
		enabledCount++

		go func(p Provider) {
			news, err := p.FetchLatestNews(ctx, a.keywords, 20)
			results <- result{news: news, err: err}
		}(provider)
	}

	// Collect results
	for i := 0; i < enabledCount; i++ {
		res := <-results
		if res.err != nil {
			// Log error but continue with other providers
			continue
		}
		allNews = append(allNews, res.news...)
	}

	// Filter by time
	cutoff := time.Now().Add(-since)
	filtered := make([]models.NewsItem, 0)
	for _, item := range allNews {
		if item.PublishedAt.After(cutoff) {
			filtered = append(filtered, item)
		}
	}

	// Calculate summary
	summary := a.calculateSummary(filtered)

	return summary, nil
}

// calculateSummary calculates news summary with sentiment
func (a *Aggregator) calculateSummary(news []models.NewsItem) *models.NewsSummary {
	if len(news) == 0 {
		return &models.NewsSummary{
			OverallSentiment: "neutral",
			UpdatedAt:        time.Now(),
		}
	}

	var totalSentiment float64
	positiveCount := 0
	negativeCount := 0
	neutralCount := 0

	for _, item := range news {
		totalSentiment += item.Sentiment

		if item.Sentiment > 0.2 {
			positiveCount++
		} else if item.Sentiment < -0.2 {
			negativeCount++
		} else {
			neutralCount++
		}
	}

	avgSentiment := totalSentiment / float64(len(news))

	// Take top 5 most relevant recent news
	recentNews := news
	if len(recentNews) > 5 {
		recentNews = recentNews[:5]
	}

	summary := &models.NewsSummary{
		TotalItems:       len(news),
		PositiveCount:    positiveCount,
		NegativeCount:    negativeCount,
		NeutralCount:     neutralCount,
		AverageSentiment: avgSentiment,
		RecentNews:       recentNews,
		UpdatedAt:        time.Now(),
	}

	summary.OverallSentiment = summary.GetOverallSentiment()

	return summary
}
