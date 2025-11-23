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

// AdaptiveCoTEngine implements true recursive Chain-of-Thought reasoning
// Agent DECIDES what to do next at each step (not fixed pipeline)
type AdaptiveCoTEngine struct {
	config         *models.AgentConfig
	aiProvider     ai.AgenticProvider
	memoryManager  *SemanticMemoryManager
	signalAnalyzer *SignalAnalyzer
	toolkit        toolkit.AgentToolkit
	toolRegistry   *toolkit.ToolRegistry // Dynamic tool dispatch
}

// Close gracefully shuts down CoT engine and flushes metrics
func (cot *AdaptiveCoTEngine) Close() error {
	if cot.toolRegistry != nil {
		return cot.toolRegistry.Close()
	}
	return nil
}

// ThinkingState represents agent's current understanding during reasoning
type ThinkingState struct {
	Observation      string
	MarketData       *models.MarketData
	CurrentPosition  *models.Position
	RecalledMemories []models.SemanticMemory
	ToolResults      map[string]interface{} // Tool name -> result
	Questions        []QuestionAnswer
	Options          []models.TradingOption
	Evaluations      []models.OptionEvaluation
	Insights         []string
	Concerns         []string
	StartTime        time.Time
	IterationCount   int
}

// QuestionAnswer represents self-questioning
type QuestionAnswer struct {
	Question string
	Answer   string
	Insight  string
}

// ThoughtStep represents one iteration of thinking
type ThoughtStep struct {
	Iteration      int                    `json:"iteration"`
	Timestamp      time.Time              `json:"timestamp"`
	Action         string                 `json:"action"` // "use_tool", "ask_question", "generate_options", "evaluate_option", "decide", "reconsider"
	Reasoning      string                 `json:"reasoning"`
	Confidence     float64                `json:"confidence"`
	ToolName       string                 `json:"tool_name,omitempty"`
	ToolParams     map[string]interface{} `json:"tool_params,omitempty"`
	ToolResult     interface{}            `json:"tool_result,omitempty"`
	Question       string                 `json:"question,omitempty"`
	Answer         string                 `json:"answer,omitempty"`
	Decision       *models.AIDecision     `json:"decision,omitempty"`
	ReconsiderWhat string                 `json:"reconsider_what,omitempty"`
}

// NewAdaptiveCoTEngine creates new adaptive CoT engine
func NewAdaptiveCoTEngine(
	config *models.AgentConfig,
	aiProvider ai.AgenticProvider,
	memoryManager *SemanticMemoryManager,
	tk toolkit.AgentToolkit,
) *AdaptiveCoTEngine {
	return &AdaptiveCoTEngine{
		config:         config,
		aiProvider:     aiProvider,
		memoryManager:  memoryManager,
		signalAnalyzer: NewSignalAnalyzer(config),
		toolkit:        tk, // Can be nil initially, set later
	}
}

// SetToolkit sets toolkit after initialization and creates registry
func (cot *AdaptiveCoTEngine) SetToolkit(tk toolkit.AgentToolkit) {
	cot.toolkit = tk
	cot.toolRegistry = toolkit.NewToolRegistry(tk)
	
	// Set metrics logger from toolkit if available (for ClickHouse batching)
	if localToolkit, ok := tk.(*toolkit.LocalToolkit); ok {
		if metricsLogger := localToolkit.GetMetricsLogger(); metricsLogger != nil {
			cot.toolRegistry.SetMetricsLogger(metricsLogger)
			logger.Debug("metrics logger connected to tool registry",
				zap.String("agent", cot.config.Name),
			)
		}
	}

	logger.Info("🔧 Tool registry initialized for agent",
		zap.String("agent", cot.config.Name),
		zap.Int("tools_available", cot.toolRegistry.GetToolCount()),
	)
}

// ThinkAdaptively executes true recursive Chain-of-Thought
// Agent decides what to do next at each iteration
// Supports checkpoint/resume for graceful shutdown
func (cot *AdaptiveCoTEngine) ThinkAdaptively(
	ctx context.Context,
	marketData *models.MarketData,
	position *models.Position,
) (*models.AgentDecision, *ai.ReasoningTrace, error) {
	sessionID := fmt.Sprintf("adaptive-cot-%s-%d", cot.config.ID, time.Now().Unix())

	logger.Info("🧠 Agent starting ADAPTIVE Chain-of-Thought",
		zap.String("agent", cot.config.Name),
		zap.String("session", sessionID),
	)

	// Check for interrupted session (resume after restart)
	checkpoint, err := cot.memoryManager.repository.GetInterruptedSession(ctx, cot.config.ID)
	var state *ThinkingState
	var history []ThoughtStep

	if err == nil && checkpoint != nil {
		// Resume from checkpoint!
		logger.Info("🔄 Resuming interrupted Chain-of-Thought",
			zap.String("agent", cot.config.Name),
			zap.String("checkpoint_session", checkpoint.SessionID),
			zap.Duration("age", time.Since(checkpoint.StartedAt)),
		)

		// Deserialize state and history
		state, history, err = cot.restoreCheckpoint(checkpoint)
		if err != nil {
			logger.Warn("failed to restore checkpoint, starting fresh", zap.Error(err))
			state = nil // Will reinitialize below
		} else {
			// Update sessionID to the checkpoint one
			sessionID = checkpoint.SessionID
			// Update market data (it might have changed)
			state.MarketData = marketData
			state.CurrentPosition = position
		}
	}

	// Initialize fresh state if no checkpoint or restore failed
	if state == nil {
		state = &ThinkingState{
			Observation:     cot.observeMarket(marketData, position),
			MarketData:      marketData,
			CurrentPosition: position,
			ToolResults:     make(map[string]interface{}),
			Questions:       []QuestionAnswer{},
			StartTime:       time.Now(),
		}
		history = []ThoughtStep{}
	}

	maxIterations := 20 // Safety limit

	// Iterative thinking loop
	startIteration := len(history) // Resume from where we left off
	for iteration := startIteration; iteration < maxIterations; iteration++ {
		// Check if context canceled (graceful shutdown)
		select {
		case <-ctx.Done():
			logger.Warn("⚠️ Chain-of-Thought interrupted by shutdown - SAVING CHECKPOINT",
				zap.String("agent", cot.config.Name),
				zap.Int("iterations_completed", len(history)),
				zap.Error(ctx.Err()),
			)

			// Save checkpoint for resume after restart
			saveCtx, saveCancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer saveCancel()

			if err := cot.memoryManager.repository.SaveThinkingCheckpoint(
				saveCtx, sessionID, cot.config.ID, state, history,
			); err != nil {
				logger.Error("failed to save thinking checkpoint", zap.Error(err))
			} else {
				logger.Info("✅ Thinking checkpoint saved - will resume after restart",
					zap.String("session_id", sessionID),
					zap.Int("steps_saved", len(history)),
				)
			}

			// Return interrupted error (do NOT finalize decision)
			return nil, nil, fmt.Errorf("thinking interrupted and checkpointed: %w", ctx.Err())
		default:
			// Continue thinking
		}

		state.IterationCount = iteration + 1

		logger.Debug("🤔 thinking iteration",
			zap.String("agent", cot.config.Name),
			zap.Int("iteration", iteration+1),
		)

		// Ask AI: "What should I do next?" (using templates)
		nextStep, err := cot.decideNextStep(ctx, state, history)
		if err != nil {
			// Check if error is due to context cancellation
			if ctx.Err() != nil {
				logger.Warn("AI call interrupted by shutdown", zap.Error(ctx.Err()))
				decision, trace := cot.finalizeDecision(state, history)
				return decision, trace, fmt.Errorf("thinking interrupted: %w", ctx.Err())
			}
			logger.Error("failed to decide next step", zap.Error(err))
			break
		}

		logger.Info("💭 agent thought",
			zap.String("agent", cot.config.Name),
			zap.Int("iteration", iteration+1),
			zap.String("action", nextStep.Action),
			zap.String("reasoning", truncate(nextStep.Reasoning, 100)),
		)

		// Execute the decided action
		shouldContinue, err := cot.executeAction(ctx, nextStep, state)
		if err != nil {
			logger.Warn("action execution failed",
				zap.String("action", nextStep.Action),
				zap.Error(err),
			)
		}

		history = append(history, *nextStep)

		// Check if agent decided to stop thinking
		if !shouldContinue {
			logger.Info("✅ Agent reached decision",
				zap.String("agent", cot.config.Name),
				zap.Int("iterations", iteration+1),
			)
			break
		}

		// Safety: if thinking too long, force decision
		if time.Since(state.StartTime) > 2*time.Minute {
			logger.Warn("thinking timeout, forcing decision",
				zap.String("agent", cot.config.Name),
			)
			break
		}
	}

	// Convert to final decision
	decision, trace := cot.finalizeDecision(state, history)

	// Delete checkpoint (thinking completed successfully)
	deleteCtx, deleteCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer deleteCancel()

	if err := cot.memoryManager.repository.DeleteCheckpoint(deleteCtx, sessionID); err != nil {
		logger.Warn("failed to delete checkpoint", zap.Error(err))
	}

	logger.Info("🎯 Adaptive CoT complete",
		zap.String("agent", cot.config.Name),
		zap.Int("iterations", len(history)),
		zap.Int("tools_used", len(state.ToolResults)),
		zap.Int("questions_asked", len(state.Questions)),
		zap.String("action", string(decision.Action)),
	)

	return decision, trace, nil
}

// decideNextStep asks AI to decide what to do next
func (cot *AdaptiveCoTEngine) decideNextStep(
	ctx context.Context,
	state *ThinkingState,
	history []ThoughtStep,
) (*ThoughtStep, error) {

	// Build adaptive prompt from template
	systemPrompt, userPrompt := cot.buildAdaptivePrompt(state, history)

	// Ask AI what to do next
	responseText, err := cot.callAIForNextStep(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("AI call failed: %w", err)
	}

	// Parse thought step
	var thought ThoughtStep
	if err := json.Unmarshal([]byte(responseText), &thought); err != nil {
		logger.Warn("failed to parse thought JSON, using fallback",
			zap.Error(err),
			zap.String("response", truncate(responseText, 200)),
		)
		// Fallback: proceed to decision
		thought = ThoughtStep{
			Action:     "decide",
			Reasoning:  "Proceeding to decision (parsing failed)",
			Confidence: 0.5,
		}
	}

	thought.Iteration = state.IterationCount
	thought.Timestamp = time.Now()

	return &thought, nil
}

// executeAction executes the agent's decided action
// Returns: shouldContinue bool (false = stop thinking, make decision)
func (cot *AdaptiveCoTEngine) executeAction(
	ctx context.Context,
	step *ThoughtStep,
	state *ThinkingState,
) (bool, error) {

	switch step.Action {
	case "use_tool":
		// Agent decided to use a tool
		result, err := cot.executeToolCall(ctx, step.ToolName, step.ToolParams)
		if err != nil {
			logger.Warn("tool execution failed",
				zap.String("tool", step.ToolName),
				zap.Error(err),
			)
			return true, err // Continue thinking despite error
		}

		step.ToolResult = result
		state.ToolResults[step.ToolName] = result

		logger.Debug("🔧 tool executed",
			zap.String("tool", step.ToolName),
		)

		return true, nil // Continue thinking

	case "ask_question":
		// Agent asked itself a question
		answer := cot.answerOwnQuestion(step.Question, state)
		step.Answer = answer

		state.Questions = append(state.Questions, QuestionAnswer{
			Question: step.Question,
			Answer:   answer,
		})

		logger.Debug("❓ self-question",
			zap.String("question", step.Question),
			zap.String("answer", truncate(answer, 100)),
		)

		return true, nil // Continue thinking

	case "recall_memory":
		// Agent wants to recall more memories
		memories, err := cot.memoryManager.RecallRelevant(ctx, cot.config.ID, string(cot.config.Personality), step.Question, 5)
		if err == nil {
			state.RecalledMemories = append(state.RecalledMemories, memories...)
		}

		return true, nil

	case "generate_options":
		// Agent ready to generate options
		situation := &models.TradingSituation{
			MarketData:      state.MarketData,
			CurrentPosition: state.CurrentPosition,
			Memories:        state.RecalledMemories,
		}

		options, err := cot.aiProvider.GenerateOptions(ctx, situation)
		if err != nil {
			return true, fmt.Errorf("failed to generate options: %w", err)
		}

		state.Options = options

		logger.Debug("💡 options generated",
			zap.Int("count", len(options)),
		)

		return true, nil // Continue to evaluation

	case "evaluate_option":
		// Agent wants to evaluate specific option
		// Find option by ID or index
		// For now, evaluate all options
		for _, option := range state.Options {
			eval, err := cot.aiProvider.EvaluateOption(ctx, &option, state.RecalledMemories)
			if err == nil {
				state.Evaluations = append(state.Evaluations, *eval)
			}
		}

		return true, nil

	case "decide":
		// Agent confident enough to decide
		if len(state.Evaluations) == 0 && len(state.Options) > 0 {
			// Need to evaluate first
			logger.Debug("need to evaluate options before deciding")
			return true, nil
		}

		return false, nil // Stop thinking, finalize decision

	case "alert_owner":
		// Agent found something urgent
		if cot.toolkit != nil {
			priority := "MEDIUM"
			if step.Confidence < 0.3 {
				priority = "HIGH" // Low confidence = uncertainty = urgent
			}
			cot.toolkit.SendUrgentAlert(ctx, step.Reasoning, priority)
		}

		return true, nil // Continue thinking

	case "log_insight":
		// Agent wants to log an insight
		state.Insights = append(state.Insights, step.Reasoning)
		if cot.toolkit != nil {
			cot.toolkit.LogThought(ctx, step.Reasoning, step.Confidence)
		}

		return true, nil

	case "reconsider":
		// Agent wants to rethink something
		logger.Info("🔄 agent reconsidering",
			zap.String("what", step.ReconsiderWhat),
		)
		// Clear relevant state
		if step.ReconsiderWhat == "options" {
			state.Options = []models.TradingOption{}
			state.Evaluations = []models.OptionEvaluation{}
		}

		return true, nil

	default:
		logger.Warn("unknown action",
			zap.String("action", step.Action),
		)
		return true, nil
	}
}

// executeToolCall dynamically executes tool based on agent's decision using Registry
func (cot *AdaptiveCoTEngine) executeToolCall(
	ctx context.Context,
	toolName string,
	params map[string]interface{},
) (interface{}, error) {

	if cot.toolRegistry == nil {
		return nil, fmt.Errorf("tool registry not initialized")
	}

	// Use registry for type-safe dynamic dispatch
	result, err := cot.toolRegistry.Execute(ctx, toolName, params)
	if err != nil {
		return nil, fmt.Errorf("tool execution failed: %w", err)
	}

	return result, nil
}

// answerOwnQuestion helps agent answer its own question using template
func (cot *AdaptiveCoTEngine) answerOwnQuestion(
	question string,
	state *ThinkingState,
) string {
	// Extract data from tool results
	var volatility float64
	var hasVolatility bool
	var supports []float64
	var hasSupports bool

	if containsSubstring(question, "volatility") {
		if vol, ok := state.ToolResults["CalculateVolatility"].(float64); ok {
			volatility = vol
			hasVolatility = true
		}
	} else if containsSubstring(question, "support") {
		if supp, ok := state.ToolResults["FindSupportLevels"].([]float64); ok {
			supports = supp
			hasSupports = true
		}
	}

	return ai.AnswerOwnQuestion(question, volatility, hasVolatility, supports, hasSupports)
}

// buildAdaptivePrompt builds prompt asking agent what to do next using templates
func (cot *AdaptiveCoTEngine) buildAdaptivePrompt(state *ThinkingState, history []ThoughtStep) (string, string) {
	// Convert history to template-friendly format
	historyForTemplate := make([]ai.AdaptiveThoughtStep, len(history))
	for i, h := range history {
		historyForTemplate[i] = ai.AdaptiveThoughtStep{
			Iteration:  h.Iteration,
			Action:     h.Action,
			Reasoning:  h.Reasoning,
			Confidence: h.Confidence,
		}
	}

	// Convert questions to template-friendly format
	questionsForTemplate := make([]ai.QuestionAnswer, len(state.Questions))
	for i, q := range state.Questions {
		questionsForTemplate[i] = ai.QuestionAnswer{
			Question: q.Question,
			Answer:   q.Answer,
		}
	}

	// Prepare data for template
	thinkingData := &ai.AdaptiveThinkingData{
		AgentName:        cot.config.Name,
		Observation:      state.Observation,
		MarketData:       state.MarketData,      // For template to format news/on-chain
		CurrentPosition:  state.CurrentPosition, // For template context
		RecalledMemories: state.RecalledMemories,
		ToolResults:      state.ToolResults,
		Questions:        questionsForTemplate,
		Options:          state.Options,
		Evaluations:      state.Evaluations,
		History:          historyForTemplate,
		Iteration:        state.IterationCount,
	}

	// Build prompts from template
	systemPrompt, userPrompt := ai.BuildAdaptiveThinkPrompt(thinkingData)

	return systemPrompt, userPrompt
}

// callAIForNextStep calls AI to get next thinking step
func (cot *AdaptiveCoTEngine) callAIForNextStep(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	// Call AI provider's adaptive thinking method
	// AI providers (Claude, DeepSeek, OpenAI) return clean JSON already
	responseText, err := cot.aiProvider.AdaptiveThink(ctx, systemPrompt, userPrompt)
	if err != nil {
		return "", fmt.Errorf("AI adaptive thinking failed: %w", err)
	}

	return responseText, nil
}

// finalizeDecision converts thinking state to final decision
func (cot *AdaptiveCoTEngine) finalizeDecision(state *ThinkingState, history []ThoughtStep) (*models.AgentDecision, *ai.ReasoningTrace) {
	// If agent reached "decide" action, use evaluations
	var decision *models.AIDecision

	if len(state.Evaluations) > 0 {
		// Make decision from evaluations
		decision, _ = cot.aiProvider.MakeFinalDecision(context.Background(), state.Evaluations)
	} else {
		// No evaluations - default to HOLD
		decision = &models.AIDecision{
			Action:     models.ActionHold,
			Reason:     "Insufficient information gathered during thinking",
			Confidence: 50,
		}
	}

	// Build reasoning trace
	trace := &ai.ReasoningTrace{
		Observation:      state.Observation,
		RecalledMemories: state.RecalledMemories,
		GeneratedOptions: state.Options,
		Evaluations:      state.Evaluations,
		FinalReasoning:   decision.Reason,
		Decision:         decision,
		ChainOfThought:   cot.buildChainFromHistory(history),
	}

	// Convert to AgentDecision
	signals := cot.signalAnalyzer.AnalyzeSignals(state.MarketData)

	agentDecision := &models.AgentDecision{
		AgentID:        cot.config.ID,
		Symbol:         state.MarketData.Symbol,
		Action:         decision.Action,
		Confidence:     decision.Confidence,
		Reason:         cot.formatAdaptiveReasoning(decision, history, state),
		TechnicalScore: signals.Technical.Score,
		NewsScore:      signals.News.Score,
		OnChainScore:   signals.OnChain.Score,
		SentimentScore: signals.Sentiment.Score,
		FinalScore:     float64(decision.Confidence),
		Executed:       false,
	}

	return agentDecision, trace
}

// buildChainFromHistory converts history to ChainOfThought
func (cot *AdaptiveCoTEngine) buildChainFromHistory(history []ThoughtStep) *ai.ChainOfThought {
	steps := []ai.ThoughtStep{}

	for _, h := range history {
		steps = append(steps, ai.ThoughtStep{
			StepNumber: h.Iteration,
			Type:       h.Action,
			Content:    h.Reasoning,
			Confidence: h.Confidence,
		})
	}

	return &ai.ChainOfThought{Steps: steps}
}

// formatAdaptiveReasoning formats reasoning using template
func (cot *AdaptiveCoTEngine) formatAdaptiveReasoning(
	decision *models.AIDecision,
	history []ThoughtStep,
	state *ThinkingState,
) string {
	// Convert history to template-friendly format
	historyForTemplate := make([]ai.AdaptiveThoughtStep, len(history))
	for i, h := range history {
		historyForTemplate[i] = ai.AdaptiveThoughtStep{
			Iteration:  h.Iteration,
			Action:     h.Action,
			Reasoning:  truncate(h.Reasoning, 100),
			Confidence: h.Confidence,
		}
	}

	return ai.FormatAdaptiveReasoning(
		cot.config.Name,
		historyForTemplate,
		len(state.ToolResults),
		len(state.Questions),
		decision,
	)
}

// observeMarket creates basic market observation using template
// Detailed formatting (news, on-chain) is done in adaptive_think.tmpl
func (cot *AdaptiveCoTEngine) observeMarket(marketData *models.MarketData, position *models.Position) string {
	price := marketData.Ticker.Last.InexactFloat64()
	change24h := marketData.Ticker.Change24h.InexactFloat64()

	hasPosition := position != nil && position.Side != models.PositionNone
	var positionSide string
	var positionPnL float64

	if hasPosition {
		positionSide = string(position.Side)
		positionPnL = position.UnrealizedPnL.InexactFloat64()
	}

	return ai.ObserveMarket(
		marketData.Symbol,
		price,
		change24h,
		hasPosition,
		positionSide,
		positionPnL,
	)
}

// restoreCheckpoint deserializes checkpoint data for resuming
func (cot *AdaptiveCoTEngine) restoreCheckpoint(
	checkpoint *ReasoningCheckpoint,
) (*ThinkingState, []ThoughtStep, error) {
	var state ThinkingState
	if err := json.Unmarshal(checkpoint.CheckpointState, &state); err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal state: %w", err)
	}

	var history []ThoughtStep
	if err := json.Unmarshal(checkpoint.CheckpointHistory, &history); err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal history: %w", err)
	}

	// Reinitialize maps (JSON unmarshal might not preserve them)
	if state.ToolResults == nil {
		state.ToolResults = make(map[string]interface{})
	}
	if state.Questions == nil {
		state.Questions = []QuestionAnswer{}
	}
	if state.Insights == nil {
		state.Insights = []string{}
	}
	if state.Concerns == nil {
		state.Concerns = []string{}
	}

	logger.Info("✅ Checkpoint restored",
		zap.String("session", checkpoint.SessionID),
		zap.Int("iterations", len(history)),
		zap.Int("tools_used", len(state.ToolResults)),
		zap.Int("questions", len(state.Questions)),
	)

	return &state, history, nil
}
