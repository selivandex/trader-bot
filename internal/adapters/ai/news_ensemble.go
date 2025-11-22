package ai

import (
	"context"
	"sort"

	"go.uber.org/zap"

	"github.com/selivandex/trader-bot/pkg/logger"
	"github.com/selivandex/trader-bot/pkg/models"
)

// NewsEvaluatorEnsemble evaluates news using multiple AI providers for consensus
type NewsEvaluatorEnsemble struct {
	providers []Provider
	enabled   bool
}

// NewNewsEvaluatorEnsemble creates new news evaluator ensemble
func NewNewsEvaluatorEnsemble(providers []Provider) *NewsEvaluatorEnsemble {
	// Count enabled providers
	enabledCount := 0
	for _, p := range providers {
		if p.IsEnabled() {
			enabledCount++
		}
	}

	return &NewsEvaluatorEnsemble{
		providers: providers,
		enabled:   enabledCount > 0,
	}
}

// EvaluateNews evaluates news using all enabled providers and calculates consensus
func (nee *NewsEvaluatorEnsemble) EvaluateNews(ctx context.Context, newsItem *models.NewsItem) error {
	if !nee.enabled {
		return nil
	}

	// Store original values to restore on error
	originalSentiment := newsItem.Sentiment
	originalImpact := newsItem.Impact
	_ = originalImpact // May be used for logging

	// Create channels for results
	type result struct {
		provider  string
		sentiment float64
		impact    int
		urgency   string
		err       error
	}

	enabledCount := 0
	for _, p := range nee.providers {
		if p.IsEnabled() {
			enabledCount++
		}
	}

	results := make(chan result, enabledCount)

	// Query all providers in parallel
	for _, provider := range nee.providers {
		if !provider.IsEnabled() {
			continue
		}

		go func(p Provider) {
			// Create copy of news item for this provider
			itemCopy := *newsItem

			err := p.EvaluateNews(ctx, &itemCopy)

			results <- result{
				provider:  p.GetName(),
				sentiment: itemCopy.Sentiment,
				impact:    itemCopy.Impact,
				urgency:   itemCopy.Urgency,
				err:       err,
			}
		}(provider)
	}

	// Collect results
	sentiments := make([]float64, 0, enabledCount)
	impacts := make([]int, 0, enabledCount)
	urgencies := make([]string, 0, enabledCount)
	providerNames := make([]string, 0, enabledCount)

	successCount := 0
	for i := 0; i < enabledCount; i++ {
		res := <-results
		if res.err != nil {
			logger.Warn("news evaluation failed for provider",
				zap.String("provider", res.provider),
				zap.Error(res.err),
			)
			continue
		}

		sentiments = append(sentiments, res.sentiment)
		impacts = append(impacts, res.impact)
		urgencies = append(urgencies, res.urgency)
		providerNames = append(providerNames, res.provider)
		successCount++
	}

	// If all providers failed, return error
	if successCount == 0 {
		logger.Warn("all news evaluators failed, keeping original scores",
			zap.String("title", newsItem.Title),
		)
		return nil
	}

	// Calculate consensus
	consensusSentiment := calculateAverageSentiment(sentiments)
	consensusImpact := calculateMedianImpact(impacts)
	consensusUrgency := calculateModeUrgency(urgencies)

	// Update news item with consensus
	newsItem.Sentiment = consensusSentiment
	newsItem.Impact = consensusImpact
	newsItem.Urgency = consensusUrgency

	logger.Info("news evaluated by ensemble",
		zap.String("title", newsItem.Title),
		zap.Strings("providers", providerNames),
		zap.Int("responses", successCount),
		zap.Float64("sentiment", consensusSentiment),
		zap.Int("impact", consensusImpact),
		zap.String("urgency", consensusUrgency),
		zap.Float64("original_sentiment", originalSentiment),
		zap.Int("original_impact", originalImpact),
	)

	return nil
}

// GetProviderNames returns names of all enabled providers
func (nee *NewsEvaluatorEnsemble) GetProviderNames() []string {
	names := make([]string, 0)
	for _, p := range nee.providers {
		if p.IsEnabled() {
			names = append(names, p.GetName())
		}
	}
	return names
}

// IsEnabled returns whether ensemble is enabled
func (nee *NewsEvaluatorEnsemble) IsEnabled() bool {
	return nee.enabled
}

// === CONSENSUS CALCULATION HELPERS ===

// calculateAverageSentiment calculates weighted average sentiment
func calculateAverageSentiment(sentiments []float64) float64 {
	if len(sentiments) == 0 {
		return 0.0
	}

	sum := 0.0
	for _, s := range sentiments {
		sum += s
	}

	return sum / float64(len(sentiments))
}

// calculateMedianImpact calculates median impact score (more robust than average)
func calculateMedianImpact(impacts []int) int {
	if len(impacts) == 0 {
		return 5
	}

	// Sort impacts
	sorted := make([]int, len(impacts))
	copy(sorted, impacts)
	sort.Ints(sorted)

	// Return median
	mid := len(sorted) / 2
	if len(sorted)%2 == 0 {
		// Even number: average of two middle values
		return (sorted[mid-1] + sorted[mid]) / 2
	}
	// Odd number: middle value
	return sorted[mid]
}

// calculateModeUrgency calculates most common urgency (mode)
func calculateModeUrgency(urgencies []string) string {
	if len(urgencies) == 0 {
		return "HOURS"
	}

	// Count occurrences
	counts := make(map[string]int)
	for _, u := range urgencies {
		counts[u]++
	}

	// Find most common
	maxCount := 0
	mode := "HOURS"
	for urgency, count := range counts {
		if count > maxCount {
			maxCount = count
			mode = urgency
		}
	}

	return mode
}

