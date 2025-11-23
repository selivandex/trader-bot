package agents

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/selivandex/trader-bot/internal/adapters/ai"
	"github.com/selivandex/trader-bot/pkg/logger"
	"github.com/selivandex/trader-bot/pkg/models"
)

// ReflectionEngine handles post-trade reflection and learning
// After each trade, agent analyzes what happened and learns from it
type ReflectionEngine struct {
	config        *models.AgentConfig
	aiProvider    ai.AgenticProvider
	repository    *Repository
	memoryManager *SemanticMemoryManager
}

// NewReflectionEngine creates new reflection engine
func NewReflectionEngine(
	config *models.AgentConfig,
	aiProvider ai.AgenticProvider,
	repository *Repository,
	memoryManager *SemanticMemoryManager,
) *ReflectionEngine {
	return &ReflectionEngine{
		config:        config,
		aiProvider:    aiProvider,
		repository:    repository,
		memoryManager: memoryManager,
	}
}

// Reflect performs post-trade reflection
// Agent analyzes trade outcome and learns lessons
func (re *ReflectionEngine) Reflect(ctx context.Context, trade *models.TradeExperience) error {
	logger.Info("🤔 Agent reflecting on trade",
		zap.String("agent", re.config.Name),
		zap.String("personality", string(re.config.Personality)),
		zap.String("symbol", trade.Symbol),
		zap.Float64("pnl_pct", trade.PnLPercent),
	)

	// Get agent's personality context for reflection
	systemPrompt := GetAgentSystemPrompt(re.config.Personality, re.config.Name)

	// Build reflection prompt with personality context
	reflectionPrompt := &models.ReflectionPrompt{
		AgentName:    re.config.Name + " (" + string(re.config.Personality) + ")",
		Trade:        trade,
		PriorBeliefs: trade.EntryReason + "\n\nMy personality: " + systemPrompt,
	}

	// Ask AI to reflect
	reflection, err := re.aiProvider.Reflect(ctx, reflectionPrompt)
	if err != nil {
		logger.Error("failed to generate reflection",
			zap.String("agent", re.config.Name),
			zap.Error(err),
		)
		return fmt.Errorf("reflection failed: %w", err)
	}

	logger.Info("📝 Reflection complete",
		zap.String("agent", re.config.Name),
		zap.Int("lessons_learned", len(reflection.KeyLessons)),
		zap.Float64("confidence", reflection.ConfidenceInAnalysis),
	)

	// Log key insights
	if len(reflection.WhatWorked) > 0 {
		logger.Info("✅ What worked",
			zap.String("agent", re.config.Name),
			zap.Strings("insights", reflection.WhatWorked),
		)
	}

	if len(reflection.WhatDidntWork) > 0 {
		logger.Info("❌ What didn't work",
			zap.String("agent", re.config.Name),
			zap.Strings("insights", reflection.WhatDidntWork),
		)
	}

	// Store reflection in database
	if err := re.repository.SaveReflection(ctx, re.config.ID, reflection); err != nil {
		logger.Error("failed to save reflection", zap.Error(err))
		// Continue despite error
	}

	// Store memory if AI suggested it
	if reflection.MemoryToStore != nil {
		if err := re.storeMemoryFromReflection(ctx, trade, reflection.MemoryToStore); err != nil {
			logger.Error("failed to store memory from reflection", zap.Error(err))
		}
	}

	// Apply suggested adjustments if confidence is high
	if reflection.ConfidenceInAnalysis > 0.7 && len(reflection.SuggestedAdjustments) > 0 {
		if err := re.applyAdjustments(ctx, reflection.SuggestedAdjustments); err != nil {
			logger.Error("failed to apply adjustments", zap.Error(err))
		}
	}

	return nil
}

// storeMemoryFromReflection stores the lesson as semantic memory
func (re *ReflectionEngine) storeMemoryFromReflection(
	ctx context.Context,
	trade *models.TradeExperience,
	memorySummary *models.MemorySummary,
) error {
	// Store with personality for collective memory contribution
	return re.memoryManager.Store(ctx, re.config.ID, string(re.config.Personality), trade)
}

// applyAdjustments applies AI-suggested strategy adjustments
func (re *ReflectionEngine) applyAdjustments(ctx context.Context, adjustments map[string]float64) error {
	logger.Info("🔧 Applying AI-suggested adjustments",
		zap.String("agent", re.config.Name),
		zap.Any("adjustments", adjustments),
	)

	// Get current config
	currentConfig, err := re.repository.GetAgent(ctx, re.config.ID)
	if err != nil {
		return fmt.Errorf("failed to get current config: %w", err)
	}

	newSpecialization := currentConfig.Specialization
	modified := false

	// Apply weight adjustments
	if techAdj, ok := adjustments["technical_weight"]; ok {
		newSpecialization.TechnicalWeight += techAdj
		modified = true
	}
	if newsAdj, ok := adjustments["news_weight"]; ok {
		newSpecialization.NewsWeight += newsAdj
		modified = true
	}
	if onchainAdj, ok := adjustments["onchain_weight"]; ok {
		newSpecialization.OnChainWeight += onchainAdj
		modified = true
	}
	if sentimentAdj, ok := adjustments["sentiment_weight"]; ok {
		newSpecialization.SentimentWeight += sentimentAdj
		modified = true
	}

	if !modified {
		return nil
	}

	// Ensure weights stay in bounds
	newSpecialization.TechnicalWeight = clampWeight(newSpecialization.TechnicalWeight)
	newSpecialization.NewsWeight = clampWeight(newSpecialization.NewsWeight)
	newSpecialization.OnChainWeight = clampWeight(newSpecialization.OnChainWeight)
	newSpecialization.SentimentWeight = clampWeight(newSpecialization.SentimentWeight)

	// Normalize to sum to 1.0
	total := newSpecialization.TechnicalWeight +
		newSpecialization.NewsWeight +
		newSpecialization.OnChainWeight +
		newSpecialization.SentimentWeight

	if total > 0 {
		newSpecialization.TechnicalWeight /= total
		newSpecialization.NewsWeight /= total
		newSpecialization.OnChainWeight /= total
		newSpecialization.SentimentWeight /= total
	}

	// Save to database
	if err := re.repository.UpdateAgentSpecialization(ctx, re.config.ID, newSpecialization); err != nil {
		return fmt.Errorf("failed to update specialization: %w", err)
	}

	logger.Info("✅ Adjustments applied",
		zap.String("agent", re.config.Name),
		zap.Float64("new_technical_weight", newSpecialization.TechnicalWeight),
		zap.Float64("new_news_weight", newSpecialization.NewsWeight),
	)

	return nil
}

// ReflectPeriodically runs periodic self-analysis on overall performance
func (re *ReflectionEngine) ReflectPeriodically(ctx context.Context, symbol string) error {
	logger.Info("🧠 Agent performing self-analysis",
		zap.String("agent", re.config.Name),
	)

	// Get performance data
	metrics, err := re.repository.GetAgentPerformanceMetrics(ctx, re.config.ID, symbol)
	if err != nil {
		return fmt.Errorf("failed to get performance metrics: %w", err)
	}

	// Not enough data to analyze
	if metrics.TotalTrades < 10 {
		logger.Debug("not enough trades for self-analysis",
			zap.Int("trades", metrics.TotalTrades),
		)
		return nil
	}

	// Get statistical memory
	memory, err := re.repository.GetAgentStatisticalMemory(ctx, re.config.ID)
	if err != nil {
		return fmt.Errorf("failed to get memory: %w", err)
	}

	// Build performance data
	performanceData := &models.PerformanceData{
		AgentID:        re.config.ID,
		AgentName:      re.config.Name,
		TimeWindow:     30 * 24 * time.Hour, // Last 30 days
		TotalTrades:    metrics.TotalTrades,
		WinRate:        metrics.WinRate,
		TotalPnL:       models.NewDecimal(metrics.TotalPnL),
		CurrentWeights: re.config.Specialization,
		SignalPerformance: map[string]models.SignalPerformance{
			"technical": {
				SignalType:    "technical",
				WinRate:       memory.TechnicalSuccessRate,
				CurrentWeight: re.config.Specialization.TechnicalWeight,
			},
			"news": {
				SignalType:    "news",
				WinRate:       memory.NewsSuccessRate,
				CurrentWeight: re.config.Specialization.NewsWeight,
			},
			"onchain": {
				SignalType:    "onchain",
				WinRate:       memory.OnChainSuccessRate,
				CurrentWeight: re.config.Specialization.OnChainWeight,
			},
			"sentiment": {
				SignalType:    "sentiment",
				WinRate:       memory.SentimentSuccessRate,
				CurrentWeight: re.config.Specialization.SentimentWeight,
			},
		},
	}

	// Ask AI to analyze own performance
	selfAnalysis, err := re.aiProvider.SelfAnalyze(ctx, performanceData)
	if err != nil {
		return fmt.Errorf("self-analysis failed: %w", err)
	}

	logger.Info("🎯 Self-analysis complete",
		zap.String("agent", re.config.Name),
		zap.Int("strengths", len(selfAnalysis.StrengthsIdentified)),
		zap.Int("weaknesses", len(selfAnalysis.WeaknessesIdentified)),
	)

	// Apply suggested changes if confidence is high
	if selfAnalysis.Confidence > 0.75 && selfAnalysis.SuggestedChanges.NewWeights != nil {
		logger.Info("🔄 Agent self-modifying strategy",
			zap.String("agent", re.config.Name),
		)

		if err := re.repository.UpdateAgentSpecialization(ctx, re.config.ID, *selfAnalysis.SuggestedChanges.NewWeights); err != nil {
			return fmt.Errorf("failed to apply self-modifications: %w", err)
		}

		// Increment adaptation count
		if err := re.repository.IncrementAdaptationCount(ctx, re.config.ID); err != nil {
			logger.Error("failed to increment adaptation count", zap.Error(err))
		}
	}

	return nil
}

func clampWeight(w float64) float64 {
	if w < 0.05 {
		return 0.05
	}
	if w > 0.80 {
		return 0.80
	}
	return w
}
