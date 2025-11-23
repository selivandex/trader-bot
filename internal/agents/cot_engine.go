package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/selivandex/trader-bot/internal/adapters/ai"
	"github.com/selivandex/trader-bot/internal/toolkit"
	"github.com/selivandex/trader-bot/pkg/logger"
	"github.com/selivandex/trader-bot/pkg/models"
)

// ChainOfThoughtEngine implements multi-step reasoning for autonomous agents
// This is what makes agents think like real AI - not just one-shot decisions
type ChainOfThoughtEngine struct {
	config         *models.AgentConfig
	aiProvider     ai.AgenticProvider // Must support agentic methods
	memoryManager  *SemanticMemoryManager
	signalAnalyzer *SignalAnalyzer
	toolkit        toolkit.AgentToolkit // Tools for querying cached data
}

// NewChainOfThoughtEngine creates new CoT engine
func NewChainOfThoughtEngine(
	config *models.AgentConfig,
	aiProvider ai.AgenticProvider,
	memoryManager *SemanticMemoryManager,
) *ChainOfThoughtEngine {
	return &ChainOfThoughtEngine{
		config:         config,
		aiProvider:     aiProvider,
		memoryManager:  memoryManager,
		signalAnalyzer: NewSignalAnalyzer(config),
		toolkit:        nil, // Will be set later via SetToolkit
	}
}

// SetToolkit sets agent's toolkit after initialization
func (cot *ChainOfThoughtEngine) SetToolkit(tk toolkit.AgentToolkit) {
	cot.toolkit = tk
}

// Think executes complete Chain-of-Thought reasoning process
// This is the core of agentic behavior - agent thinks through decision step by step
func (cot *ChainOfThoughtEngine) Think(
	ctx context.Context,
	marketData *models.MarketData,
	position *models.Position,
) (*models.AgentDecision, *ai.ReasoningTrace, error) {
	sessionID := fmt.Sprintf("agent-%s-%d", cot.config.ID, time.Now().Unix())

	// Get agent's personality system prompt
	systemPrompt := GetAgentSystemPrompt(cot.config.Personality, cot.config.Name)

	logger.Info("🧠 Agent starting Chain-of-Thought reasoning",
		zap.String("agent", cot.config.Name),
		zap.String("personality", string(cot.config.Personality)),
		zap.String("session", sessionID),
	)

	// Log system prompt (first time only, for debugging)
	logger.Debug("agent system prompt",
		zap.String("agent", cot.config.Name),
		zap.String("prompt_preview", systemPrompt[:min(len(systemPrompt), 200)]+"..."),
	)

	trace := &ai.ReasoningTrace{}

	// STEP 1: Observe current situation
	logger.Debug("Step 1: Observing market situation")
	observation := cot.observeMarket(marketData, position)
	trace.Observation = observation

	// STEP 2: Recall relevant memories (personal + collective)
	logger.Debug("Step 2: Recalling similar past experiences")
	memories, err := cot.memoryManager.RecallRelevant(ctx, cot.config.ID, string(cot.config.Personality), observation, 5)
	if err != nil {
		logger.Warn("failed to recall memories", zap.Error(err))
		memories = []models.SemanticMemory{} // Continue without memories
	}
	trace.RecalledMemories = memories

	logger.Info("📚 Recalled memories",
		zap.String("agent", cot.config.Name),
		zap.Int("count", len(memories)),
	)

	// STEP 3: Generate possible options
	logger.Debug("Step 3: Generating trading options")
	situation := &models.TradingSituation{
		MarketData:      marketData,
		CurrentPosition: position,
		Memories:        memories,
	}

	options, err := cot.aiProvider.GenerateOptions(ctx, situation)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate options: %w", err)
	}
	trace.GeneratedOptions = options

	logger.Info("💡 Generated options",
		zap.String("agent", cot.config.Name),
		zap.Int("count", len(options)),
	)

	// STEP 4: Evaluate each option
	logger.Debug("Step 4: Evaluating each option")
	evaluations := make([]models.OptionEvaluation, 0, len(options))

	for _, option := range options {
		eval, err := cot.aiProvider.EvaluateOption(ctx, &option, memories)
		if err != nil {
			logger.Warn("failed to evaluate option",
				zap.String("option", option.Description),
				zap.Error(err),
			)
			continue
		}
		evaluations = append(evaluations, *eval)

		logger.Debug("evaluated option",
			zap.String("option", option.Description),
			zap.Float64("score", eval.Score),
		)
	}
	trace.Evaluations = evaluations

	if len(evaluations) == 0 {
		return nil, nil, fmt.Errorf("no valid evaluations")
	}

	// STEP 5: Make final decision based on evaluations
	logger.Debug("Step 5: Making final decision")
	decision, err := cot.aiProvider.MakeFinalDecision(ctx, evaluations)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to make decision: %w", err)
	}

	trace.Decision = decision
	trace.FinalReasoning = decision.Reason

	// Enhance decision with signal scores (for compatibility)
	signals := cot.signalAnalyzer.AnalyzeSignals(marketData)

	// Create chain of thought summary
	cot.fillChainOfThought(trace)

	logger.Info("✅ Decision made through Chain-of-Thought",
		zap.String("agent", cot.config.Name),
		zap.String("action", string(decision.Action)),
		zap.Int("confidence", decision.Confidence),
		zap.Int("steps", len(trace.ChainOfThought.Steps)),
	)

	// Serialize market data for storage
	marketDataJSON, err := json.Marshal(marketData)
	if err != nil {
		logger.Warn("failed to marshal market data", zap.Error(err))
		marketDataJSON = []byte("{}")
	}

	// Convert to AgentDecision format
	agentDecision := &models.AgentDecision{
		AgentID:        cot.config.ID,
		Symbol:         marketData.Symbol,
		Action:         decision.Action,
		Confidence:     decision.Confidence,
		Reason:         cot.formatReasonWithTrace(decision, trace),
		TechnicalScore: signals.Technical.Score,
		NewsScore:      signals.News.Score,
		OnChainScore:   signals.OnChain.Score,
		SentimentScore: signals.Sentiment.Score,
		FinalScore:     float64(decision.Confidence),
		MarketData:     string(marketDataJSON),
		Executed:       false,
	}

	return agentDecision, trace, nil
}

// observeMarket creates high-level observation of current market
func (cot *ChainOfThoughtEngine) observeMarket(marketData *models.MarketData, position *models.Position) string {
	price := marketData.Ticker.Last.InexactFloat64()
	change24h := marketData.Ticker.Change24h.InexactFloat64()

	observation := fmt.Sprintf(
		"Market: %s at $%.2f (%.2f%% 24h change). ",
		marketData.Symbol, price, change24h,
	)

	if position != nil && position.Side != models.PositionNone {
		pnl := position.UnrealizedPnL.InexactFloat64()
		observation += fmt.Sprintf("Current position: %s with $%.2f PnL. ", position.Side, pnl)
	} else {
		observation += "No open position. "
	}

	// Add news context if available
	if marketData.NewsSummary != nil && marketData.NewsSummary.TotalItems > 0 {
		observation += fmt.Sprintf(
			"News sentiment: %s (%.2f from %d articles). ",
			marketData.NewsSummary.OverallSentiment,
			marketData.NewsSummary.AverageSentiment,
			marketData.NewsSummary.TotalItems,
		)
	}

	// Add on-chain context if available
	if marketData.OnChainData != nil {
		flowUSD := marketData.OnChainData.NetExchangeFlow.InexactFloat64()
		if flowUSD != 0 {
			direction := "inflow"
			if flowUSD < 0 {
				direction = "outflow"
			}
			observation += fmt.Sprintf(
				"Exchange %s: $%.1fM. ",
				direction,
				abs(flowUSD)/1_000_000,
			)
		}
	}

	return observation
}

// fillChainOfThought creates summary of reasoning steps
func (cot *ChainOfThoughtEngine) fillChainOfThought(trace *ai.ReasoningTrace) {
	steps := []ai.ThoughtStep{}

	// Step 1: Observation
	steps = append(steps, ai.ThoughtStep{
		StepNumber: 1,
		Type:       "observation",
		Content:    trace.Observation,
		Confidence: 1.0,
	})

	// Step 2: Memory recall
	if len(trace.RecalledMemories) > 0 {
		memoryContent := fmt.Sprintf("Recalled %d relevant past experiences", len(trace.RecalledMemories))
		steps = append(steps, ai.ThoughtStep{
			StepNumber: 2,
			Type:       "memory_recall",
			Content:    memoryContent,
			Confidence: 0.8,
		})
	}

	// Step 3: Option generation
	optionContent := fmt.Sprintf("Generated %d possible trading options", len(trace.GeneratedOptions))
	steps = append(steps, ai.ThoughtStep{
		StepNumber: 3,
		Type:       "option_generation",
		Content:    optionContent,
		Confidence: 0.9,
	})

	// Step 4: Evaluation
	evalContent := fmt.Sprintf("Evaluated %d options", len(trace.Evaluations))
	steps = append(steps, ai.ThoughtStep{
		StepNumber: 4,
		Type:       "evaluation",
		Content:    evalContent,
		Confidence: 0.85,
	})

	// Step 5: Final decision
	steps = append(steps, ai.ThoughtStep{
		StepNumber: 5,
		Type:       "decision",
		Content:    trace.FinalReasoning,
		Confidence: float64(trace.Decision.Confidence) / 100.0,
	})

	trace.ChainOfThought = &ai.ChainOfThought{Steps: steps}
}

// formatReasonWithTrace formats decision reason with full reasoning trace
func (cot *ChainOfThoughtEngine) formatReasonWithTrace(decision *models.AIDecision, trace *ai.ReasoningTrace) string {
	reason := fmt.Sprintf("🧠 [Chain-of-Thought Reasoning by %s]\n\n", cot.config.Name)

	reason += fmt.Sprintf("📊 Observation: %s\n\n", trace.Observation)

	if len(trace.RecalledMemories) > 0 {
		reason += fmt.Sprintf("📚 Recalled %d relevant memories:\n", len(trace.RecalledMemories))
		for i, mem := range trace.RecalledMemories {
			if i < 2 { // Show top 2
				reason += fmt.Sprintf("  - %s → %s\n", mem.Context, mem.Lesson)
			}
		}
		reason += "\n"
	}

	reason += fmt.Sprintf("💡 Generated %d options and evaluated:\n", len(trace.GeneratedOptions))
	for i, eval := range trace.Evaluations {
		if i < 3 { // Show top 3
			optName := "Unknown"
			for _, opt := range trace.GeneratedOptions {
				if opt.OptionID == eval.OptionID {
					optName = opt.Description
					break
				}
			}
			reason += fmt.Sprintf("  %d. %s (Score: %.1f/100)\n", i+1, optName, eval.Score)
		}
	}
	reason += "\n"

	reason += fmt.Sprintf("✅ Final Decision: %s\n", decision.Action)
	reason += fmt.Sprintf("Reasoning: %s\n", decision.Reason)
	reason += fmt.Sprintf("Confidence: %d%%", decision.Confidence)

	return reason
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
