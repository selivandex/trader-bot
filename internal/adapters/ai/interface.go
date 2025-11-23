package ai

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/selivandex/trader-bot/pkg/logger"
	"github.com/selivandex/trader-bot/pkg/models"
)

// Provider represents AI provider interface
type Provider interface {
	// Analyze analyzes trading prompt and returns decision
	Analyze(ctx context.Context, prompt *models.TradingPrompt) (*models.AIDecision, error)

	// EvaluateNews evaluates news item and updates its sentiment/impact scores (DEPRECATED: use EvaluateNewsBatch)
	EvaluateNews(ctx context.Context, newsItem *models.NewsItem) error

	// EvaluateNewsBatch evaluates multiple news items in single API call (more efficient)
	EvaluateNewsBatch(ctx context.Context, newsItems []*models.NewsItem) error

	// GetName returns provider name
	GetName() string

	// GetCost returns approximate cost per request in USD
	GetCost() float64

	// IsEnabled returns whether provider is enabled
	IsEnabled() bool
}

// Ensemble manages multiple AI providers and creates consensus decisions
type Ensemble struct {
	providers    []Provider
	minConsensus int
	enabledCount int
}

// NewEnsemble creates new AI ensemble
func NewEnsemble(providers []Provider, minConsensus int) *Ensemble {
	enabledCount := 0
	for _, p := range providers {
		if p.IsEnabled() {
			enabledCount++
		}
	}

	return &Ensemble{
		providers:    providers,
		minConsensus: minConsensus,
		enabledCount: enabledCount,
	}
}

// Analyze queries all enabled providers and returns consensus decision
func (e *Ensemble) Analyze(ctx context.Context, prompt *models.TradingPrompt) (*models.EnsembleDecision, error) {
	// Query all providers in parallel
	type result struct {
		decision *models.AIDecision
		err      error
	}

	results := make(chan result, e.enabledCount)

	for _, provider := range e.providers {
		if !provider.IsEnabled() {
			continue
		}

		go func(p Provider) {
			decision, err := p.Analyze(ctx, prompt)
			results <- result{decision: decision, err: err}
		}(provider)
	}

	// Collect results
	decisions := make([]*models.AIDecision, 0, e.enabledCount)
	for i := 0; i < e.enabledCount; i++ {
		res := <-results
		if res.err != nil {
			// Log error but continue with other providers
			logger.Warn("AI provider failed", zap.Error(res.err))
			continue
		}
		decisions = append(decisions, res.decision)
	}

	if len(decisions) == 0 {
		return nil, fmt.Errorf("all AI providers failed")
	}

	// Calculate consensus
	consensus := e.calculateConsensus(decisions)

	return consensus, nil
}

// calculateConsensus determines consensus from multiple AI decisions
func (e *Ensemble) calculateConsensus(decisions []*models.AIDecision) *models.EnsembleDecision {
	if len(decisions) == 0 {
		return &models.EnsembleDecision{
			Decisions: decisions,
			Agreement: false,
		}
	}

	if len(decisions) == 1 {
		// Single decision, use it
		return &models.EnsembleDecision{
			Decisions:  decisions,
			Consensus:  decisions[0],
			Agreement:  true,
			Confidence: decisions[0].Confidence,
		}
	}

	// Count actions
	actionCounts := make(map[models.AIAction]int)
	actionDecisions := make(map[models.AIAction]*models.AIDecision)
	totalConfidence := 0

	for _, decision := range decisions {
		actionCounts[decision.Action]++
		actionDecisions[decision.Action] = decision
		totalConfidence += decision.Confidence
	}

	// Find most common action
	var mostCommonAction models.AIAction
	maxCount := 0

	for action, count := range actionCounts {
		if count > maxCount {
			maxCount = count
			mostCommonAction = action
		}
	}

	// Check if consensus reached
	agreement := maxCount >= e.minConsensus

	avgConfidence := totalConfidence / len(decisions)

	return &models.EnsembleDecision{
		Decisions:  decisions,
		Consensus:  actionDecisions[mostCommonAction],
		Agreement:  agreement,
		Confidence: avgConfidence,
	}
}

// GetEnabledProviders returns list of enabled providers
func (e *Ensemble) GetEnabledProviders() []Provider {
	enabled := make([]Provider, 0, e.enabledCount)
	for _, p := range e.providers {
		if p.IsEnabled() {
			enabled = append(enabled, p)
		}
	}
	return enabled
}

// GetTotalCost returns total cost of querying all enabled providers
func (e *Ensemble) GetTotalCost() float64 {
	cost := 0.0
	for _, p := range e.providers {
		if p.IsEnabled() {
			cost += p.GetCost()
		}
	}
	return cost
}
