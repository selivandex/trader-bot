package models

import (
	"time"

	"github.com/shopspring/decimal"
)

// ========== Reflection Models ==========

// ReflectionPrompt contains data for agent to reflect on past trade
type ReflectionPrompt struct {
	AgentName     string           `json:"agent_name"`
	Trade         *TradeExperience `json:"trade"`
	MarketContext *MarketData      `json:"market_context"`
	PriorBeliefs  string           `json:"prior_beliefs"` // What agent thought before trade
}

// TradeExperience represents complete trade experience for reflection
type TradeExperience struct {
	SignalsUsed   map[string]float64 `json:"signals_used"`
	Symbol        string             `json:"symbol"`
	Side          string             `json:"side"`
	EntryPrice    decimal.Decimal    `json:"entry_price"`
	ExitPrice     decimal.Decimal    `json:"exit_price"`
	Size          decimal.Decimal    `json:"size"`
	PnL           decimal.Decimal    `json:"pnl"`
	EntryReason   string             `json:"entry_reason"`
	ExitReason    string             `json:"exit_reason"`
	PnLPercent    float64            `json:"pnl_percent"`
	Duration      time.Duration      `json:"duration"`
	WasSuccessful bool               `json:"was_successful"`
}

// Reflection contains agent's self-analysis after trade
type Reflection struct {
	SuggestedAdjustments map[string]float64 `json:"suggested_adjustments"`
	MemoryToStore        *MemorySummary     `json:"memory_to_store"`
	Analysis             string             `json:"analysis"`
	WhatWorked           []string           `json:"what_worked"`
	WhatDidntWork        []string           `json:"what_didnt_work"`
	KeyLessons           []string           `json:"key_lessons"`
	ConfidenceInAnalysis float64            `json:"confidence_in_analysis"`
}

// ========== Memory Models ==========

// SemanticMemory represents agent's semantic memory (episodic knowledge)
type SemanticMemory struct {
	LastAccessed time.Time `json:"last_accessed" db:"last_accessed"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	ID           string    `json:"id" db:"id"`
	AgentID      string    `json:"agent_id" db:"agent_id"`
	Context      string    `json:"context" db:"context"`
	Action       string    `json:"action" db:"action"`
	Outcome      string    `json:"outcome" db:"outcome"`
	Lesson       string    `json:"lesson" db:"lesson"`
	Embedding    []float32 `json:"embedding" db:"embedding"`
	Importance   float64   `json:"importance" db:"importance"`
	AccessCount  int       `json:"access_count" db:"access_count"`
}

// MemorySummary is what agent decides to remember
type MemorySummary struct {
	Context    string  `json:"context"`
	Action     string  `json:"action"`
	Outcome    string  `json:"outcome"`
	Lesson     string  `json:"lesson"`
	Importance float64 `json:"importance"` // How important to remember
}

// ========== Planning Models ==========

// PlanRequest contains information for creating trading plan
type PlanRequest struct {
	MarketData      *MarketData      `json:"market_data"`
	CurrentPosition *Position        `json:"current_position"`
	AgentName       string           `json:"agent_name"`
	Memories        []SemanticMemory `json:"memories"`
	TimeHorizon     time.Duration    `json:"time_horizon"`
	RiskTolerance   float64          `json:"risk_tolerance"`
}

// TradingPlan represents agent's multi-step plan
type TradingPlan struct {
	CreatedAt      time.Time       `json:"created_at"`
	ExpiresAt      time.Time       `json:"expires_at"`
	PlanID         string          `json:"plan_id"`
	AgentID        string          `json:"agent_id"`
	Status         string          `json:"status"`
	Assumptions    []string        `json:"assumptions"`
	Scenarios      []Scenario      `json:"scenarios"`
	TriggerSignals []TriggerSignal `json:"trigger_signals"`
	RiskLimits     RiskLimits      `json:"risk_limits"`
	TimeHorizon    time.Duration   `json:"time_horizon"`
}

// Scenario represents one possible market scenario and response
type Scenario struct {
	Name        string   `json:"name"`
	Action      string   `json:"action"`
	Reasoning   string   `json:"reasoning"`
	Indicators  []string `json:"indicators"`
	Probability float64  `json:"probability"`
}

// RiskLimits defines plan's risk boundaries
type RiskLimits struct {
	StopTradingIf   string  `json:"stop_trading_if"`
	MaxDrawdown     float64 `json:"max_drawdown"`
	MaxDailyLoss    float64 `json:"max_daily_loss"`
	MaxPositionSize float64 `json:"max_position_size"`
}

// TriggerSignal defines when plan should be revised
type TriggerSignal struct {
	Condition string `json:"condition"` // "Volume spikes 3x"
	Action    string `json:"action"`    // "Reassess plan immediately"
}

// ========== Option Generation & Evaluation ==========

// TradingSituation describes current market situation
type TradingSituation struct {
	MarketData      *MarketData      `json:"market_data"`
	CurrentPosition *Position        `json:"current_position"`
	CurrentPlan     *TradingPlan     `json:"current_plan,omitempty"`
	Balance         decimal.Decimal  `json:"balance"`
	RecentTrades    []Trade          `json:"recent_trades"`
	Memories        []SemanticMemory `json:"memories"`
}

// TradingOption represents one possible action agent could take
type TradingOption struct {
	OptionID        string           `json:"option_id"`
	Action          AIAction         `json:"action"`
	Description     string           `json:"description"`
	Reasoning       string           `json:"reasoning"`
	ExpectedOutcome string           `json:"expected_outcome"`
	EstimatedRisk   string           `json:"estimated_risk"`
	Timeframe       string           `json:"timeframe"`
	Parameters      OptionParameters `json:"parameters"`
}

// OptionParameters contains trade parameters for option
type OptionParameters struct {
	Size       decimal.Decimal `json:"size"`
	EntryPrice decimal.Decimal `json:"entry_price,omitempty"`
	StopLoss   decimal.Decimal `json:"stop_loss,omitempty"`
	TakeProfit decimal.Decimal `json:"take_profit,omitempty"`
	Leverage   int             `json:"leverage"`
}

// OptionEvaluation contains agent's analysis of one option
type OptionEvaluation struct {
	OptionID        string   `json:"option_id"`
	ExpectedOutcome string   `json:"expected_outcome"`
	Reasoning       string   `json:"reasoning"`
	Pros            []string `json:"pros"`
	Cons            []string `json:"cons"`
	Risks           []string `json:"risks"`
	Opportunities   []string `json:"opportunities"`
	Score           float64  `json:"score"`
	Confidence      float64  `json:"confidence"`
	ConfidenceScore int      `json:"confidence_score"`
}

// ========== Self-Analysis Models ==========

// PerformanceData contains agent's recent performance stats
type PerformanceData struct {
	SignalPerformance map[string]SignalPerformance `json:"signal_performance"`
	AgentName         string                       `json:"agent_name"`
	AgentID           string                       `json:"agent_id"`
	AvgPnL            decimal.Decimal              `json:"avg_pnl"`
	TotalPnL          decimal.Decimal              `json:"total_pnl"`
	MaxWin            decimal.Decimal              `json:"max_win"`
	MaxLoss           decimal.Decimal              `json:"max_loss"`
	RecentTrades      []TradeExperience            `json:"recent_trades"`
	CurrentWeights    AgentSpecialization          `json:"current_weights"`
	TotalTrades       int                          `json:"total_trades"`
	CurrentDrawdown   float64                      `json:"current_drawdown"`
	WinRate           float64                      `json:"win_rate"`
	TimeWindow        time.Duration                `json:"time_window"`
}

// SignalPerformance tracks performance of specific signal type
type SignalPerformance struct {
	SignalType    string  `json:"signal_type"` // "technical", "news", "onchain"
	TradesUsed    int     `json:"trades_used"` // How many trades used this signal
	WinRate       float64 `json:"win_rate"`
	AvgPnL        float64 `json:"avg_pnl"`
	CurrentWeight float64 `json:"current_weight"`
}

// SelfAnalysis contains agent's self-evaluation and adaptation suggestions
type SelfAnalysis struct {
	SuggestedChanges      SuggestedStrategyChanges `json:"suggested_changes"`
	PerformanceAssessment string                   `json:"performance_assessment"`
	ReasoningTrace        string                   `json:"reasoning_trace"`
	StrengthsIdentified   []string                 `json:"strengths_identified"`
	WeaknessesIdentified  []string                 `json:"weaknesses_identified"`
	RootCauses            []string                 `json:"root_causes"`
	Confidence            float64                  `json:"confidence"`
}

// SuggestedStrategyChanges contains agent's self-modification suggestions
type SuggestedStrategyChanges struct {
	NewWeights           *AgentSpecialization `json:"new_weights,omitempty"`
	ParameterAdjustments map[string]float64   `json:"parameter_adjustments"`
	Reasoning            string               `json:"reasoning"`
	BehavioralChanges    []string             `json:"behavioral_changes"`
	SignalsToEmphasize   []string             `json:"signals_to_emphasize"`
	SignalsToDeemphasize []string             `json:"signals_to_deemphasize"`
}

// ========== Reasoning Trace ==========

// AgentThought represents one step in agent's reasoning process
type AgentThought struct {
	Timestamp   time.Time `json:"timestamp"`
	ThoughtType string    `json:"thought_type"` // "observation", "memory_recall", "option_generation", "evaluation", "decision"
	Content     string    `json:"content"`
	Data        string    `json:"data,omitempty"` // JSON of additional data
}

// ReasoningSession captures complete agent reasoning for one decision
type ReasoningSession struct {
	StartedAt   time.Time      `json:"started_at"`
	CompletedAt time.Time      `json:"completed_at"`
	Decision    *AIDecision    `json:"decision"`
	SessionID   string         `json:"session_id"`
	AgentID     string         `json:"agent_id"`
	Thoughts    []AgentThought `json:"thoughts"`
	Executed    bool           `json:"executed"`
}

// ========== Collective Memory Models ==========

// CollectiveMemory represents shared wisdom across agents of same personality
type CollectiveMemory struct {
	LastConfirmedAt   time.Time `json:"last_confirmed_at" db:"last_confirmed_at"`
	CreatedAt         time.Time `json:"created_at" db:"created_at"`
	ID                string    `json:"id" db:"id"`
	Personality       string    `json:"personality" db:"personality"`
	Context           string    `json:"context" db:"context"`
	Action            string    `json:"action" db:"action"`
	Lesson            string    `json:"lesson" db:"lesson"`
	Embedding         []float32 `json:"embedding" db:"embedding"`
	Importance        float64   `json:"importance" db:"importance"`
	ConfirmationCount int       `json:"confirmation_count" db:"confirmation_count"`
	SuccessRate       float64   `json:"success_rate" db:"success_rate"`
}

// MemoryConfirmation tracks agent's validation of collective memory
type MemoryConfirmation struct {
	ConfirmedAt        time.Time       `json:"confirmed_at" db:"confirmed_at"`
	ID                 string          `json:"id" db:"id"`
	CollectiveMemoryID string          `json:"collective_memory_id" db:"collective_memory_id"`
	AgentID            string          `json:"agent_id" db:"agent_id"`
	PnLSum             decimal.Decimal `json:"pnl_sum" db:"pnl_sum"`
	TradeCount         int             `json:"trade_count" db:"trade_count"`
	WasSuccessful      bool            `json:"was_successful" db:"was_successful"`
}

// ========== Validator Council Models ==========

// ValidationRequest contains all data for validator to review agent's decision
type ValidationRequest struct {
	AgentDecision     *AgentDecision       `json:"agent_decision"`
	AgentProfile      *AgentConfig         `json:"agent_profile"`
	MarketData        *MarketData          `json:"market_data"`
	CurrentPosition   *Position            `json:"current_position,omitempty"`
	RecentPerformance *PerformanceSnapshot `json:"recent_performance,omitempty"`
	ValidatorRole     string               `json:"validator_role"`
	SystemPrompt      string               `json:"system_prompt"`
	UserPrompt        string               `json:"user_prompt"`
}

// ValidationResponse is validator's verdict on the decision
type ValidationResponse struct {
	Verdict            string   `json:"verdict"`
	Reasoning          string   `json:"reasoning"`
	RecommendedChanges string   `json:"recommended_changes,omitempty"`
	CriticalConcerns   string   `json:"critical_concerns,omitempty"`
	KeyRisks           []string `json:"key_risks"`
	KeyOpportunities   []string `json:"key_opportunities,omitempty"`
	Confidence         int      `json:"confidence"`
}

// PerformanceSnapshot contains recent agent performance for validator context
type PerformanceSnapshot struct {
	Last24hPnL        decimal.Decimal `json:"last_24h_pnl"`
	Last7DaysPnL      decimal.Decimal `json:"last_7days_pnl"`
	RecentWinRate     float64         `json:"recent_win_rate"` // Last 10 trades
	CurrentDrawdown   float64         `json:"current_drawdown"`
	ConsecutiveLosses int             `json:"consecutive_losses"`
}
