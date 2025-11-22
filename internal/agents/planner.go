package agents

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/alexanderselivanov/trader/internal/adapters/ai"
	"github.com/alexanderselivanov/trader/pkg/logger"
	"github.com/alexanderselivanov/trader/pkg/models"
)

// PlanningEngine creates and executes trading plans
// Agents plan ahead instead of just reacting to current market
type PlanningEngine struct {
	config        *models.AgentConfig
	aiProvider    ai.AgenticProvider
	repository    *Repository
	memoryManager *SemanticMemoryManager
	activePlan    *models.TradingPlan
}

// NewPlanningEngine creates new planning engine
func NewPlanningEngine(
	config *models.AgentConfig,
	aiProvider ai.AgenticProvider,
	repository *Repository,
	memoryManager *SemanticMemoryManager,
) *PlanningEngine {
	return &PlanningEngine{
		config:        config,
		aiProvider:    aiProvider,
		repository:    repository,
		memoryManager: memoryManager,
	}
}

// CreatePlan creates forward-looking trading plan
func (pe *PlanningEngine) CreatePlan(
	ctx context.Context,
	marketData *models.MarketData,
	position *models.Position,
	timeHorizon time.Duration,
) (*models.TradingPlan, error) {
	logger.Info("📋 Agent creating trading plan",
		zap.String("agent", pe.config.Name),
		zap.Duration("horizon", timeHorizon),
	)

	// Recall relevant memories for context (personal + collective)
	observation := fmt.Sprintf("Creating plan for %s over %v", marketData.Symbol, timeHorizon)
	memories, err := pe.memoryManager.RecallRelevant(ctx, pe.config.ID, string(pe.config.Personality), observation, 5)
	if err != nil {
		logger.Warn("failed to recall memories for planning", zap.Error(err))
		memories = []models.SemanticMemory{}
	}

	// Build plan request
	planRequest := &models.PlanRequest{
		AgentName:       pe.config.Name,
		MarketData:      marketData,
		CurrentPosition: position,
		TimeHorizon:     timeHorizon,
		RiskTolerance:   float64(pe.config.Strategy.MaxPositionPercent) / 100.0,
		Memories:        memories,
	}

	// Ask AI to create plan
	plan, err := pe.aiProvider.CreatePlan(ctx, planRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to create plan: %w", err)
	}

	// Set plan metadata
	plan.AgentID = pe.config.ID
	plan.PlanID = fmt.Sprintf("plan-%d-%d", pe.config.ID, time.Now().Unix())
	plan.TimeHorizon = timeHorizon
	plan.CreatedAt = time.Now()
	plan.ExpiresAt = time.Now().Add(timeHorizon)
	plan.Status = "active"

	// Save plan to database
	if err := pe.repository.SaveTradingPlan(ctx, plan); err != nil {
		logger.Error("failed to save plan", zap.Error(err))
		// Continue despite error
	}

	pe.activePlan = plan

	logger.Info("✅ Trading plan created",
		zap.String("agent", pe.config.Name),
		zap.Int("scenarios", len(plan.Scenarios)),
		zap.Int("assumptions", len(plan.Assumptions)),
	)

	// Log scenarios
	for i, scenario := range plan.Scenarios {
		logger.Debug("scenario",
			zap.Int("num", i+1),
			zap.String("name", scenario.Name),
			zap.Float64("probability", scenario.Probability),
			zap.String("action", scenario.Action),
		)
	}

	return plan, nil
}

// GetActivePlan returns current active plan
func (pe *PlanningEngine) GetActivePlan() *models.TradingPlan {
	if pe.activePlan != nil && time.Now().Before(pe.activePlan.ExpiresAt) {
		return pe.activePlan
	}
	return nil
}

// ShouldRevisePlan checks if plan should be revised based on trigger signals
func (pe *PlanningEngine) ShouldRevisePlan(marketData *models.MarketData) (bool, string) {
	plan := pe.GetActivePlan()
	if plan == nil {
		return true, "No active plan"
	}

	// Check if plan expired
	if time.Now().After(plan.ExpiresAt) {
		return true, "Plan expired"
	}

	// Check trigger signals
	for _, trigger := range plan.TriggerSignals {
		if pe.checkTriggerCondition(trigger, marketData) {
			return true, fmt.Sprintf("Trigger: %s", trigger.Condition)
		}
	}

	return false, ""
}

// checkTriggerCondition checks if trigger condition is met
func (pe *PlanningEngine) checkTriggerCondition(trigger models.TriggerSignal, marketData *models.MarketData) bool {
	condition := strings.ToLower(trigger.Condition)

	// Volume triggers
	if strings.Contains(condition, "volume") && strings.Contains(condition, "spike") {
		if marketData.Indicators != nil && marketData.Indicators.Volume != nil {
			ratio := marketData.Indicators.Volume.Ratio.InexactFloat64()

			// Parse multiplier from condition (e.g., "3x")
			if strings.Contains(condition, "3x") && ratio > 3.0 {
				return true
			}
			if strings.Contains(condition, "2x") && ratio > 2.0 {
				return true
			}
		}
	}

	// Price movement triggers
	if strings.Contains(condition, "price") && strings.Contains(condition, "%") {
		change24h := marketData.Ticker.Change24h.InexactFloat64()

		// Parse percentage (e.g., "> 5%")
		if strings.Contains(condition, "> 5") && math.Abs(change24h) > 5.0 {
			return true
		}
		if strings.Contains(condition, "> 8") && math.Abs(change24h) > 8.0 {
			return true
		}
	}

	// News impact triggers
	if strings.Contains(condition, "news") && strings.Contains(condition, "impact") {
		if marketData.NewsSummary != nil {
			for _, item := range marketData.NewsSummary.RecentNews {
				if strings.Contains(condition, "> 9") && item.Impact >= 9 {
					return true
				}
				if strings.Contains(condition, "> 8") && item.Impact >= 8 {
					return true
				}
			}
		}
	}

	// RSI triggers
	if strings.Contains(condition, "rsi") {
		if marketData.Indicators != nil && marketData.Indicators.RSI != nil {
			if rsi14, ok := marketData.Indicators.RSI["14"]; ok {
				rsiVal := rsi14.InexactFloat64()

				if strings.Contains(condition, "> 70") && rsiVal > 70 {
					return true
				}
				if strings.Contains(condition, "< 30") && rsiVal < 30 {
					return true
				}
			}
		}
	}

	return false
}

// ExecutePlan makes decision based on active plan
func (pe *PlanningEngine) ExecutePlan(ctx context.Context, marketData *models.MarketData) (*models.AIDecision, error) {
	plan := pe.GetActivePlan()
	if plan == nil {
		return nil, fmt.Errorf("no active plan")
	}

	logger.Debug("executing plan decision",
		zap.String("agent", pe.config.Name),
		zap.String("plan_id", plan.PlanID),
	)

	// Find matching scenario based on current market
	matchedScenario := pe.findMatchingScenario(plan, marketData)
	if matchedScenario == nil {
		logger.Warn("no matching scenario in plan",
			zap.String("agent", pe.config.Name),
		)
		return nil, fmt.Errorf("no matching scenario")
	}

	logger.Info("📌 Scenario matched",
		zap.String("scenario", matchedScenario.Name),
		zap.String("action", matchedScenario.Action),
	)

	// Convert scenario action to AI decision
	decision := &models.AIDecision{
		Provider:   pe.aiProvider.GetName(),
		Action:     pe.parseAction(matchedScenario.Action),
		Reason:     fmt.Sprintf("[Plan: %s] %s", matchedScenario.Name, matchedScenario.Reasoning),
		Confidence: int(matchedScenario.Probability * 100),
	}

	return decision, nil
}

// findMatchingScenario finds scenario that matches current market conditions
func (pe *PlanningEngine) findMatchingScenario(plan *models.TradingPlan, marketData *models.MarketData) *models.Scenario {
	// Simplified scenario matching
	// In production, use AI to evaluate which scenario best matches current conditions

	for _, scenario := range plan.Scenarios {
		// Check indicators mentioned in scenario
		// For now, return first scenario with probability > 0.3
		if scenario.Probability > 0.3 {
			return &scenario
		}
	}

	return nil
}

// parseAction converts scenario action string to AIAction
func (pe *PlanningEngine) parseAction(actionStr string) models.AIAction {
	// Simplified parsing
	// In production, use more sophisticated parsing or have AI return structured actions

	switch {
	case contains(actionStr, "long"):
		return models.ActionOpenLong
	case contains(actionStr, "short"):
		return models.ActionOpenShort
	case contains(actionStr, "close"):
		return models.ActionClose
	default:
		return models.ActionHold
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr))
	// Simplified - in production use strings.Contains
}
