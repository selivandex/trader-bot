package ai

import (
	"context"

	"github.com/selivandex/trader-bot/pkg/models"
)

// NewsEvaluatorInterface defines interface for news evaluation
type NewsEvaluatorInterface interface {
	EvaluateNews(ctx context.Context, newsItem *models.NewsItem) error
	EvaluateNewsBatch(ctx context.Context, newsItems []*models.NewsItem) error
}

// NewsEvaluator wraps a single AI provider for news evaluation
type NewsEvaluator struct {
	provider Provider
}

// NewNewsEvaluator creates new AI news evaluator with given provider
func NewNewsEvaluator(provider Provider) *NewsEvaluator {
	return &NewsEvaluator{
		provider: provider,
	}
}

// EvaluateNews evaluates news item using the configured AI provider
func (ne *NewsEvaluator) EvaluateNews(ctx context.Context, newsItem *models.NewsItem) error {
	if ne.provider == nil || !ne.provider.IsEnabled() {
		return nil // Skip evaluation if provider not configured
	}

	return ne.provider.EvaluateNews(ctx, newsItem)
}

// EvaluateNewsBatch evaluates multiple news items in single API call
func (ne *NewsEvaluator) EvaluateNewsBatch(ctx context.Context, newsItems []*models.NewsItem) error {
	if ne.provider == nil || !ne.provider.IsEnabled() {
		return nil
	}

	return ne.provider.EvaluateNewsBatch(ctx, newsItems)
}

// GetProviderName returns name of the AI provider being used
func (ne *NewsEvaluator) GetProviderName() string {
	if ne.provider == nil {
		return "none"
	}
	return ne.provider.GetName()
}
