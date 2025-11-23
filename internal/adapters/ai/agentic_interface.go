package ai

import (
	"context"
	"time"

	"github.com/selivandex/trader-bot/pkg/models"
)

// AgenticProvider extends Provider with autonomous agent capabilities
// These methods enable agents to think, plan, reflect, and learn like autonomous AI
type AgenticProvider interface {
	Provider
	
	// Reflect analyzes past trade outcome and generates insights
	// Agent learns from experience: "What worked? What didn't? What to remember?"
	Reflect(ctx context.Context, reflection *models.ReflectionPrompt) (*models.Reflection, error)
	
	// GenerateOptions creates multiple possible trading strategies for current situation
	// Agent explores possibilities: "What could I do here? What are my options?"
	GenerateOptions(ctx context.Context, situation *models.TradingSituation) ([]models.TradingOption, error)
	
	// EvaluateOption analyzes pros/cons of a specific trading option
	// Agent thinks critically: "Is this a good idea? What could go wrong?"
	EvaluateOption(ctx context.Context, option *models.TradingOption, memories []models.SemanticMemory) (*models.OptionEvaluation, error)
	
	// MakeFinalDecision chooses best option after evaluation
	// Agent decides with full reasoning: "Based on all evaluations, I'll do X because Y"
	MakeFinalDecision(ctx context.Context, evaluations []models.OptionEvaluation) (*models.AIDecision, error)
	
	// CreatePlan develops multi-step trading plan for time horizon
	// Agent plans ahead: "Over next 24h, if X happens I'll do Y, if Z happens I'll do W"
	CreatePlan(ctx context.Context, planRequest *models.PlanRequest) (*models.TradingPlan, error)
	
	// SelfAnalyze evaluates own performance and suggests strategy changes
	// Agent adapts: "My news signals aren't working, I should rely more on technical analysis"
	SelfAnalyze(ctx context.Context, performance *models.PerformanceData) (*models.SelfAnalysis, error)
	
	// FindSimilarMemories retrieves relevant past experiences for current situation
	// Agent remembers: "This reminds me of when BTC dropped on ETF rejection..."
	FindSimilarMemories(ctx context.Context, currentSituation string, memories []models.SemanticMemory, topK int) ([]models.SemanticMemory, error)
	
	// SummarizeMemory creates concise summary of what to remember from experience
	// Agent stores wisdom: "When news sentiment suddenly shifts negative, wait for confirmation"
	SummarizeMemory(ctx context.Context, experience *models.TradeExperience) (*models.MemorySummary, error)
	
	// ValidateDecision validates trading decision from validator perspective
	// Validator critically evaluates: "Should this trade be executed? What are the risks?"
	ValidateDecision(ctx context.Context, validationRequest *models.ValidationRequest) (*models.ValidationResponse, error)
	
	// AdaptiveThink performs one iteration of recursive chain-of-thought reasoning
	// Agent decides next action: "What should I do next? Use tool? Ask question? Decide?"
	// Returns JSON with action and reasoning for next step
	AdaptiveThink(ctx context.Context, systemPrompt, userPrompt string) (string, error)
}

// ChainOfThought represents step-by-step reasoning process
type ChainOfThought struct {
	Steps []ThoughtStep `json:"steps"`
}

// ThoughtStep represents one step in agent's reasoning
type ThoughtStep struct {
	StepNumber  int     `json:"step_number"`
	Type        string  `json:"type"` // "observation", "reasoning", "conclusion"
	Content     string  `json:"content"`
	Confidence  float64 `json:"confidence"`
}

// ReasoningTrace captures agent's complete decision-making process
type ReasoningTrace struct {
	Observation      string                    `json:"observation"`
	RecalledMemories []models.SemanticMemory   `json:"recalled_memories"`
	GeneratedOptions []models.TradingOption    `json:"generated_options"`
	Evaluations      []models.OptionEvaluation `json:"evaluations"`
	FinalReasoning   string                    `json:"final_reasoning"`
	Decision         *models.AIDecision        `json:"decision"`
	ChainOfThought   *ChainOfThought           `json:"chain_of_thought"`
	ToolUsage        *ToolUsageTrace           `json:"tool_usage,omitempty"` // NEW: Track tool calls if tools were used
}

// ToolUsageTrace captures all tool invocations during reasoning
type ToolUsageTrace struct {
	SessionID string        `json:"session_id"`
	AgentID   string        `json:"agent_id"`
	ToolCalls []ToolCall    `json:"tool_calls"`
	TotalTime time.Duration `json:"total_time"`
	StartTime time.Time     `json:"start_time"`
	EndTime   time.Time     `json:"end_time"`
}

// ToolCall represents one tool invocation
type ToolCall struct {
	ToolName   string                 `json:"tool_name"`
	Parameters map[string]interface{} `json:"parameters"`
	Result     interface{}            `json:"result,omitempty"`
	Error      string                 `json:"error,omitempty"`
	StartTime  time.Time              `json:"start_time"`
	Latency    time.Duration          `json:"latency"`
	Success    bool                   `json:"success"`
}

// SupportsAgenticBehavior checks if provider supports agentic methods
func SupportsAgenticBehavior(provider Provider) bool {
	_, ok := provider.(AgenticProvider)
	return ok
}

// GetAgenticProvider returns agentic provider if supported, nil otherwise
func GetAgenticProvider(provider Provider) AgenticProvider {
	if agenticProvider, ok := provider.(AgenticProvider); ok {
		return agenticProvider
	}
	return nil
}

