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
	Symbol        string             `json:"symbol"`
	Side          string             `json:"side"` // "long" or "short"
	EntryPrice    decimal.Decimal    `json:"entry_price"`
	ExitPrice     decimal.Decimal    `json:"exit_price"`
	Size          decimal.Decimal    `json:"size"`
	PnL           decimal.Decimal    `json:"pnl"`
	PnLPercent    float64            `json:"pnl_percent"`
	Duration      time.Duration      `json:"duration"`
	EntryReason   string             `json:"entry_reason"`
	ExitReason    string             `json:"exit_reason"`
	SignalsUsed   map[string]float64 `json:"signals_used"` // "technical": 85, "news": 60
	WasSuccessful bool               `json:"was_successful"`
}

// Reflection contains agent's self-analysis after trade
type Reflection struct {
	Analysis             string             `json:"analysis"`
	WhatWorked           []string           `json:"what_worked"`
	WhatDidntWork        []string           `json:"what_didnt_work"`
	KeyLessons           []string           `json:"key_lessons"`
	SuggestedAdjustments map[string]float64 `json:"suggested_adjustments"` // "news_weight": -0.05
	MemoryToStore        *MemorySummary     `json:"memory_to_store"`
	ConfidenceInAnalysis float64            `json:"confidence_in_analysis"`
}

// ========== Memory Models ==========

// SemanticMemory represents agent's semantic memory (episodic knowledge)
type SemanticMemory struct {
	ID           string    `json:"id" db:"id"`
	AgentID      string    `json:"agent_id" db:"agent_id"`
	Context      string    `json:"context" db:"context"`           // "BTC dropped 5% on ETF rejection"
	Action       string    `json:"action" db:"action"`             // "Went short at $42k"
	Outcome      string    `json:"outcome" db:"outcome"`           // "Profit +3.2%, good call"
	Lesson       string    `json:"lesson" db:"lesson"`             // "News-driven drops = short opportunities"
	Embedding    []float32 `json:"embedding" db:"embedding"`       // Vector embedding for similarity search
	Importance   float64   `json:"importance" db:"importance"`     // 0.0 - 1.0
	AccessCount  int       `json:"access_count" db:"access_count"` // How often recalled
	LastAccessed time.Time `json:"last_accessed" db:"last_accessed"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
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
	AgentName       string           `json:"agent_name"`
	MarketData      *MarketData      `json:"market_data"`
	CurrentPosition *Position        `json:"current_position"`
	TimeHorizon     time.Duration    `json:"time_horizon"` // "24h", "3d", etc
	RiskTolerance   float64          `json:"risk_tolerance"`
	Memories        []SemanticMemory `json:"memories"` // Relevant past experiences
}

// TradingPlan represents agent's multi-step plan
type TradingPlan struct {
	PlanID         string          `json:"plan_id"`
	AgentID        string          `json:"agent_id"`
	TimeHorizon    time.Duration   `json:"time_horizon"`
	Assumptions    []string        `json:"assumptions"` // "BTC will range between $42k-$45k"
	Scenarios      []Scenario      `json:"scenarios"`   // Different market scenarios
	RiskLimits     RiskLimits      `json:"risk_limits"`
	TriggerSignals []TriggerSignal `json:"trigger_signals"` // When to revise plan
	CreatedAt      time.Time       `json:"created_at"`
	ExpiresAt      time.Time       `json:"expires_at"`
	Status         string          `json:"status"` // "active", "executed", "cancelled"
}

// Scenario represents one possible market scenario and response
type Scenario struct {
	Name        string   `json:"name"`        // "BTC breaks above $45k"
	Probability float64  `json:"probability"` // 0.3 = 30% chance
	Indicators  []string `json:"indicators"`  // What signals this scenario
	Action      string   `json:"action"`      // "Go long with 3x leverage"
	Reasoning   string   `json:"reasoning"`
}

// RiskLimits defines plan's risk boundaries
type RiskLimits struct {
	MaxDrawdown     float64 `json:"max_drawdown"`      // 5% max drawdown
	MaxDailyLoss    float64 `json:"max_daily_loss"`    // $100 max loss per day
	MaxPositionSize float64 `json:"max_position_size"` // 30% of balance
	StopTradingIf   string  `json:"stop_trading_if"`   // "3 consecutive losses"
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
	RecentTrades    []Trade          `json:"recent_trades"`
	Balance         decimal.Decimal  `json:"balance"`
	Memories        []SemanticMemory `json:"memories"` // Relevant memories
	CurrentPlan     *TradingPlan     `json:"current_plan,omitempty"`
}

// TradingOption represents one possible action agent could take
type TradingOption struct {
	OptionID    string           `json:"option_id"`
	Action      AIAction         `json:"action"`      // OPEN_LONG, OPEN_SHORT, HOLD, CLOSE
	Description string           `json:"description"` // "Go long with 2x leverage targeting $45k"
	Parameters  OptionParameters `json:"parameters"`
	Reasoning   string           `json:"reasoning"` // Why this option exists
}

// OptionParameters contains trade parameters for option
type OptionParameters struct {
	Size       decimal.Decimal `json:"size"`
	Leverage   int             `json:"leverage"`
	EntryPrice decimal.Decimal `json:"entry_price,omitempty"`
	StopLoss   decimal.Decimal `json:"stop_loss,omitempty"`
	TakeProfit decimal.Decimal `json:"take_profit,omitempty"`
}

// OptionEvaluation contains agent's analysis of one option
type OptionEvaluation struct {
	OptionID        string   `json:"option_id"`
	Score           float64  `json:"score"` // 0-100
	Pros            []string `json:"pros"`
	Cons            []string `json:"cons"`
	Risks           []string `json:"risks"`
	Opportunities   []string `json:"opportunities"`
	ExpectedOutcome string   `json:"expected_outcome"` // "Likely +2-3% over 6h"
	Confidence      float64  `json:"confidence"`       // 0.0 - 1.0
	Reasoning       string   `json:"reasoning"`        // Detailed analysis
}

// ========== Self-Analysis Models ==========

// PerformanceData contains agent's recent performance stats
type PerformanceData struct {
	AgentID           string                       `json:"agent_id"`
	AgentName         string                       `json:"agent_name"`
	TimeWindow        time.Duration                `json:"time_window"` // Last 7 days, 30 days, etc
	TotalTrades       int                          `json:"total_trades"`
	WinRate           float64                      `json:"win_rate"`
	AvgPnL            decimal.Decimal              `json:"avg_pnl"`
	TotalPnL          decimal.Decimal              `json:"total_pnl"`
	MaxWin            decimal.Decimal              `json:"max_win"`
	MaxLoss           decimal.Decimal              `json:"max_loss"`
	CurrentDrawdown   float64                      `json:"current_drawdown"`
	SignalPerformance map[string]SignalPerformance `json:"signal_performance"`
	RecentTrades      []TradeExperience            `json:"recent_trades"`
	CurrentWeights    AgentSpecialization          `json:"current_weights"`
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
	PerformanceAssessment string                   `json:"performance_assessment"`
	StrengthsIdentified   []string                 `json:"strengths_identified"`
	WeaknessesIdentified  []string                 `json:"weaknesses_identified"`
	RootCauses            []string                 `json:"root_causes"` // Why weaknesses exist
	SuggestedChanges      SuggestedStrategyChanges `json:"suggested_changes"`
	Confidence            float64                  `json:"confidence"` // How confident in this analysis
	ReasoningTrace        string                   `json:"reasoning_trace"`
}

// SuggestedStrategyChanges contains agent's self-modification suggestions
type SuggestedStrategyChanges struct {
	NewWeights           *AgentSpecialization `json:"new_weights,omitempty"`
	ParameterAdjustments map[string]float64   `json:"parameter_adjustments"` // "stop_loss": 2.5
	BehavioralChanges    []string             `json:"behavioral_changes"`    // "Be more patient with entries"
	SignalsToEmphasize   []string             `json:"signals_to_emphasize"`
	SignalsToDeemphasize []string             `json:"signals_to_deemphasize"`
	Reasoning            string               `json:"reasoning"`
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
	SessionID   string         `json:"session_id"`
	AgentID     string         `json:"agent_id"`
	StartedAt   time.Time      `json:"started_at"`
	CompletedAt time.Time      `json:"completed_at"`
	Thoughts    []AgentThought `json:"thoughts"`
	Decision    *AIDecision    `json:"decision"`
	Executed    bool           `json:"executed"`
}

// ========== Collective Memory Models ==========

// CollectiveMemory represents shared wisdom across agents of same personality
type CollectiveMemory struct {
	ID                string    `json:"id" db:"id"`
	Personality       string    `json:"personality" db:"personality"`
	Context           string    `json:"context" db:"context"`
	Action            string    `json:"action" db:"action"`
	Lesson            string    `json:"lesson" db:"lesson"`
	Embedding         []float32 `json:"embedding" db:"embedding"`
	Importance        float64   `json:"importance" db:"importance"`
	ConfirmationCount int       `json:"confirmation_count" db:"confirmation_count"`
	SuccessRate       float64   `json:"success_rate" db:"success_rate"`
	LastConfirmedAt   time.Time `json:"last_confirmed_at" db:"last_confirmed_at"`
	CreatedAt         time.Time `json:"created_at" db:"created_at"`
}

// MemoryConfirmation tracks agent's validation of collective memory
type MemoryConfirmation struct {
	ID                  string          `json:"id" db:"id"`
	CollectiveMemoryID  string          `json:"collective_memory_id" db:"collective_memory_id"`
	AgentID             string          `json:"agent_id" db:"agent_id"`
	WasSuccessful       bool            `json:"was_successful" db:"was_successful"`
	TradeCount          int             `json:"trade_count" db:"trade_count"`
	PnLSum              decimal.Decimal `json:"pnl_sum" db:"pnl_sum"`
	ConfirmedAt         time.Time       `json:"confirmed_at" db:"confirmed_at"`
}
