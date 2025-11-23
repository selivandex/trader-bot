package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"

	"github.com/selivandex/trader-bot/pkg/logger"
	"github.com/selivandex/trader-bot/pkg/models"
)

// MemoryManager handles agent learning and adaptation (statistical)
type MemoryManager struct {
	repository *Repository
}

// NewMemoryManager creates new memory manager
func NewMemoryManager(repository *Repository) *MemoryManager {
	return &MemoryManager{repository: repository}
}

// RecordDecisionOutcome records the outcome of an agent's decision
func (m *MemoryManager) RecordDecisionOutcome(ctx context.Context, decisionID string, outcome *DecisionOutcome) error {
	outcomeJSON, err := json.Marshal(outcome)
	if err != nil {
		return fmt.Errorf("failed to marshal outcome: %w", err)
	}

	err = m.repository.UpdateDecisionOutcome(ctx, decisionID, outcomeJSON)
	if err != nil {
		return fmt.Errorf("failed to update decision outcome: %w", err)
	}

	logger.Debug("recorded decision outcome",
		zap.String("decision_id", decisionID),
		zap.Float64("pnl", outcome.PnL.InexactFloat64()),
	)

	return nil
}

// GetAgentMemory retrieves agent's memory/learning data
func (m *MemoryManager) GetAgentMemory(ctx context.Context, agentID string) (*models.AgentMemory, error) {
	return m.repository.GetAgentStatisticalMemory(ctx, agentID)
}

// UpdateSuccessRates updates success rates for different signal types
func (m *MemoryManager) UpdateSuccessRates(ctx context.Context, agentID string) error {
	err := m.repository.UpdateSignalSuccessRates(ctx, agentID)
	if err != nil {
		return fmt.Errorf("failed to update success rates: %w", err)
	}

	logger.Debug("updated agent success rates", zap.String("agent_id", agentID))
	return nil
}

// AdaptStrategy adapts agent's specialization based on performance
func (m *MemoryManager) AdaptStrategy(ctx context.Context, agentID string, config *models.AgentConfig) error {
	// Get current memory
	memory, err := m.GetAgentMemory(ctx, agentID)
	if err != nil {
		return fmt.Errorf("failed to get memory: %w", err)
	}

	// Update success rates first
	if err := m.UpdateSuccessRates(ctx, agentID); err != nil {
		return fmt.Errorf("failed to update success rates: %w", err)
	}

	// Don't adapt if not enough data
	if memory.TotalDecisions < 20 {
		logger.Debug("not enough decisions for adaptation",
			zap.String("agent_id", agentID),
			zap.Int("decisions", memory.TotalDecisions),
		)
		return nil
	}

	// Calculate new weights based on success rates
	oldWeights := config.Specialization
	newWeights := m.calculateAdaptedWeights(oldWeights, memory, config.LearningRate)

	// Validate new weights
	if err := newWeights.Validate(); err != nil {
		return fmt.Errorf("invalid adapted weights: %w", err)
	}

	// Update agent config via repository
	err = m.repository.UpdateAgentSpecialization(ctx, agentID, newWeights)
	if err != nil {
		return fmt.Errorf("failed to update agent config: %w", err)
	}

	// Update adaptation count
	err = m.repository.IncrementAdaptationCount(ctx, agentID)
	if err != nil {
		return fmt.Errorf("failed to update adaptation count: %w", err)
	}

	logger.Info("adapted agent strategy",
		zap.String("agent_id", agentID),
		zap.Float64("old_technical_weight", oldWeights.TechnicalWeight),
		zap.Float64("new_technical_weight", newWeights.TechnicalWeight),
		zap.Float64("old_news_weight", oldWeights.NewsWeight),
		zap.Float64("new_news_weight", newWeights.NewsWeight),
	)

	return nil
}

// calculateAdaptedWeights calculates new weights based on success rates
func (m *MemoryManager) calculateAdaptedWeights(
	current models.AgentSpecialization,
	memory *models.AgentMemory,
	learningRate float64,
) models.AgentSpecialization {
	// Calculate performance scores for each signal type
	techPerf := memory.TechnicalSuccessRate
	newsPerf := memory.NewsSuccessRate
	onChainPerf := memory.OnChainSuccessRate
	sentimentPerf := memory.SentimentSuccessRate

	// Adjust weights based on performance
	// Better performing signals get more weight
	techAdjustment := (techPerf - 0.5) * learningRate
	newsAdjustment := (newsPerf - 0.5) * learningRate
	onChainAdjustment := (onChainPerf - 0.5) * learningRate
	sentimentAdjustment := (sentimentPerf - 0.5) * learningRate

	// Apply adjustments
	newTech := current.TechnicalWeight + techAdjustment
	newNews := current.NewsWeight + newsAdjustment
	newOnChain := current.OnChainWeight + onChainAdjustment
	newSentiment := current.SentimentWeight + sentimentAdjustment

	// Ensure no weight goes below 5% or above 80%
	newTech = clamp(newTech, 0.05, 0.80)
	newNews = clamp(newNews, 0.05, 0.80)
	newOnChain = clamp(newOnChain, 0.05, 0.80)
	newSentiment = clamp(newSentiment, 0.05, 0.80)

	// Normalize to sum to 1.0
	total := newTech + newNews + newOnChain + newSentiment

	return models.AgentSpecialization{
		TechnicalWeight: newTech / total,
		NewsWeight:      newNews / total,
		OnChainWeight:   newOnChain / total,
		SentimentWeight: newSentiment / total,
	}
}

// ShouldAdapt determines if agent should adapt its strategy
func (m *MemoryManager) ShouldAdapt(ctx context.Context, agentID string) (bool, error) {
	memory, err := m.GetAgentMemory(ctx, agentID)
	if err != nil {
		return false, fmt.Errorf("failed to get memory: %w", err)
	}

	// Adapt every 50 decisions
	if memory.TotalDecisions%50 == 0 && memory.TotalDecisions > 0 {
		return true, nil
	}

	// Or if it's been more than 7 days since last adaptation
	if time.Since(memory.LastAdaptedAt) > 7*24*time.Hour {
		return true, nil
	}

	return false, nil
}

// GetPerformanceMetrics calculates agent performance metrics
func (m *MemoryManager) GetPerformanceMetrics(ctx context.Context, agentID string, symbol string) (*AgentPerformanceMetrics, error) {
	return m.repository.GetAgentPerformanceMetrics(ctx, agentID, symbol)
}

// DecisionOutcome represents the outcome of a trading decision
type DecisionOutcome struct {
	PnL           decimal.Decimal `json:"pnl"`
	ExitPrice     decimal.Decimal `json:"exit_price"`
	ExitReason    string          `json:"exit_reason"`
	PnLPercent    float64         `json:"pnl_percent"`
	Duration      time.Duration   `json:"duration"`
	MaxDrawdown   float64         `json:"max_drawdown"`
	WasSuccessful bool            `json:"was_successful"`
}

// Note: AgentPerformanceMetrics is now defined in repository.go

// clamp ensures value is within min and max bounds
func clamp(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
