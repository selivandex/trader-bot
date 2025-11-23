package models

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

// AgentPersonality defines agent's trading style and behavior
type AgentPersonality string

const (
	PersonalityConservative AgentPersonality = "conservative" // Low risk, technical-focused
	PersonalityAggressive   AgentPersonality = "aggressive"   // High risk, news-driven
	PersonalityBalanced     AgentPersonality = "balanced"     // Mix of all signals
	PersonalityScalper      AgentPersonality = "scalper"      // Short-term, high frequency
	PersonalitySwing        AgentPersonality = "swing"        // Medium-term trends
	PersonalityNewsTrader   AgentPersonality = "news_trader"  // Reacts to major news events
	PersonalityWhaleHunter  AgentPersonality = "whale_hunter" // Follows on-chain movements
	PersonalityContrarian   AgentPersonality = "contrarian"   // Opposite to crowd sentiment
)

// AgentSpecialization defines how agent weighs different signal types
type AgentSpecialization struct {
	TechnicalWeight float64 `json:"technical_weight"` // 0.0 - 1.0
	NewsWeight      float64 `json:"news_weight"`
	OnChainWeight   float64 `json:"onchain_weight"`
	SentimentWeight float64 `json:"sentiment_weight"`
	// Sum should equal 1.0
}

// Validate checks if weights sum to 1.0
func (s *AgentSpecialization) Validate() error {
	sum := s.TechnicalWeight + s.NewsWeight + s.OnChainWeight + s.SentimentWeight
	if sum < 0.99 || sum > 1.01 { // Allow small floating point errors
		return fmt.Errorf("weights must sum to 1.0, got %.2f", sum)
	}
	return nil
}

// AgentConfig defines agent configuration and parameters
type AgentConfig struct {
	UpdatedAt           time.Time           `json:"updated_at" db:"updated_at"`
	CreatedAt           time.Time           `json:"created_at" db:"created_at"`
	ValidationConfig    *ValidationConfig   `json:"validation_config" db:"validation_config"`
	Personality         AgentPersonality    `json:"personality" db:"personality"`
	ID                  string              `json:"id" db:"id"`
	MinWhaleTransaction decimal.Decimal     `json:"min_whale_transaction" db:"min_whale_transaction"`
	Name                string              `json:"name" db:"name"`
	UserID              string              `json:"user_id" db:"user_id"`
	Strategy            StrategyParameters  `json:"strategy" db:"strategy"`
	Specialization      AgentSpecialization `json:"specialization" db:"specialization"`
	DecisionInterval    time.Duration       `json:"decision_interval" db:"decision_interval"`
	MinNewsImpact       float64             `json:"min_news_impact" db:"min_news_impact"`
	LearningRate        float64             `json:"learning_rate" db:"learning_rate"`
	InvertSentiment     bool                `json:"invert_sentiment" db:"invert_sentiment"`
	IsActive            bool                `json:"is_active" db:"is_active"`
}

// ValidationConfig configures validator council for agent
type ValidationConfig struct {
	MinConfidenceForValidation int     `json:"min_confidence_for_validation"`
	ConsensusThreshold         float64 `json:"consensus_threshold"`
	Enabled                    bool    `json:"enabled"`
	RequireUnanimous           bool    `json:"require_unanimous"`
	ValidateOnlyHighRisk       bool    `json:"validate_only_high_risk"`
}

// AgentState represents runtime state of an agent
type AgentState struct {
	UpdatedAt          time.Time       `json:"updated_at" db:"updated_at"`
	PnL                decimal.Decimal `json:"pnl" db:"pnl"`
	Symbol             string          `json:"symbol" db:"symbol"`
	Balance            decimal.Decimal `json:"balance" db:"balance"`
	InitialBalance     decimal.Decimal `json:"initial_balance" db:"initial_balance"`
	Equity             decimal.Decimal `json:"equity" db:"equity"`
	ID                 string          `json:"id" db:"id"`
	AgentID            string          `json:"agent_id" db:"agent_id"`
	TotalTrades        int             `json:"total_trades" db:"total_trades"`
	WinningTrades      int             `json:"winning_trades" db:"winning_trades"`
	LosingTrades       int             `json:"losing_trades" db:"losing_trades"`
	WinRate            float64         `json:"win_rate" db:"win_rate"`
	IsTrading          bool            `json:"is_trading" db:"is_trading"`
	LastKnownPosition  *Position       `json:"last_known_position,omitempty" db:"-"`  // Not persisted, runtime only
	PositionJustClosed bool            `json:"position_just_closed,omitempty" db:"-"` // Not persisted, runtime only
}

// AgentDecision represents a decision made by an agent
type AgentDecision struct {
	CreatedAt          time.Time       `json:"created_at" db:"created_at"`
	ClosedAt           *time.Time      `json:"closed_at" db:"closed_at"`
	OrderID            string          `json:"order_id" db:"order_id"`
	ValidatorConsensus string          `json:"validator_consensus" db:"validator_consensus"`
	AgentID            string          `json:"agent_id" db:"agent_id"`
	Reason             string          `json:"reason" db:"reason"`
	Symbol             string          `json:"symbol" db:"symbol"`
	Outcome            string          `json:"outcome" db:"outcome"`
	CoTTrace           string          `json:"cot_trace" db:"cot_trace"`
	Action             AIAction        `json:"action" db:"action"`
	TakeProfitOrderID  string          `json:"take_profit_order_id" db:"take_profit_order_id"`
	MarketData         string          `json:"market_data" db:"market_data"`
	StopLossOrderID    string          `json:"stop_loss_order_id" db:"stop_loss_order_id"`
	ExecutionPrice     decimal.Decimal `json:"execution_price" db:"execution_price"`
	ExecutionSize      decimal.Decimal `json:"execution_size" db:"execution_size"`
	ID                 string          `json:"id" db:"id"`
	FinalScore         float64         `json:"final_score" db:"final_score"`
	SentimentScore     float64         `json:"sentiment_score" db:"sentiment_score"`
	OnChainScore       float64         `json:"onchain_score" db:"onchain_score"`
	NewsScore          float64         `json:"news_score" db:"news_score"`
	TechnicalScore     float64         `json:"technical_score" db:"technical_score"`
	Confidence         int             `json:"confidence" db:"confidence"`
	Executed           bool            `json:"executed" db:"executed"`
}

// AgentMemory stores learning data for adaptation
type AgentMemory struct {
	LastAdaptedAt         time.Time `json:"last_adapted_at" db:"last_adapted_at"`
	UpdatedAt             time.Time `json:"updated_at" db:"updated_at"`
	ID                    string    `json:"id" db:"id"`
	AgentID               string    `json:"agent_id" db:"agent_id"`
	BestMarketConditions  string    `json:"best_market_conditions" db:"best_market_conditions"`
	WorstMarketConditions string    `json:"worst_market_conditions" db:"worst_market_conditions"`
	TechnicalSuccessRate  float64   `json:"technical_success_rate" db:"technical_success_rate"`
	NewsSuccessRate       float64   `json:"news_success_rate" db:"news_success_rate"`
	OnChainSuccessRate    float64   `json:"onchain_success_rate" db:"onchain_success_rate"`
	SentimentSuccessRate  float64   `json:"sentiment_success_rate" db:"sentiment_success_rate"`
	TotalDecisions        int       `json:"total_decisions" db:"total_decisions"`
	AdaptationCount       int       `json:"adaptation_count" db:"adaptation_count"`
}

// AgentTournament represents a competition between agents
type AgentTournament struct {
	StartedAt     time.Time       `json:"started_at" db:"started_at"`
	CreatedAt     time.Time       `json:"created_at" db:"created_at"`
	EndedAt       *time.Time      `json:"ended_at" db:"ended_at"`
	WinnerAgentID *string         `json:"winner_agent_id" db:"winner_agent_id"`
	ID            string          `json:"id" db:"id"`
	UserID        string          `json:"user_id" db:"user_id"`
	Name          string          `json:"name" db:"name"`
	StartBalance  decimal.Decimal `json:"start_balance" db:"start_balance"`
	Results       string          `json:"results" db:"results"`
	Symbols       []string        `json:"symbols" db:"symbols"`
	Duration      time.Duration   `json:"duration" db:"duration"`
	IsActive      bool            `json:"is_active" db:"is_active"`
}

// AgentScore represents tournament performance metrics
type AgentScore struct {
	AgentID      string          `json:"agent_id"`
	AgentName    string          `json:"agent_name"`
	TotalReturn  decimal.Decimal `json:"total_return"`
	AvgTradePnL  decimal.Decimal `json:"avg_trade_pnl"`
	ReturnPct    float64         `json:"return_pct"`
	WinRate      float64         `json:"win_rate"`
	MaxDrawdown  float64         `json:"max_drawdown"`
	SharpeRatio  float64         `json:"sharpe_ratio"`
	TradeCount   int             `json:"trade_count"`
	ProfitFactor float64         `json:"profit_factor"`
}

// WeightedDecisionInput holds all signals with their scores
type WeightedDecisionInput struct {
	TechnicalSignal SignalScore
	NewsSignal      SignalScore
	OnChainSignal   SignalScore
	SentimentSignal SignalScore
	Specialization  AgentSpecialization
}

// SignalScore represents score for a specific signal type
type SignalScore struct {
	Direction  string
	Reason     string
	Score      float64
	Confidence float64
}

// AgentMetric represents performance snapshot for ClickHouse
type AgentMetric struct {
	AgentID     string          `json:"agent_id"`
	AgentName   string          `json:"agent_name"`
	Personality string          `json:"personality"`
	Timestamp   time.Time       `json:"timestamp"`
	Symbol      string          `json:"symbol"`
	Balance     decimal.Decimal `json:"balance"`
	Equity      decimal.Decimal `json:"equity"`
	PnL         decimal.Decimal `json:"pnl"`
	PnLPercent  float64         `json:"pnl_percent"`

	// Decision metrics
	DecisionsTotal int `json:"decisions_total"`
	DecisionsHold  int `json:"decisions_hold"`
	DecisionsOpen  int `json:"decisions_open"`
	DecisionsClose int `json:"decisions_close"`

	// Trading performance
	TotalTrades   int     `json:"total_trades"`
	WinningTrades int     `json:"winning_trades"`
	LosingTrades  int     `json:"losing_trades"`
	WinRate       float64 `json:"win_rate"`

	// Cost tracking
	AICostUSD        float64 `json:"ai_cost_usd"`
	ValidatorCostUSD float64 `json:"validator_cost_usd"`

	// Risk metrics
	SharpeRatio     float64 `json:"sharpe_ratio"`
	MaxDrawdown     float64 `json:"max_drawdown"`
	CurrentDrawdown float64 `json:"current_drawdown"`
}

// CalculateFinalScore computes weighted final score for agent decision
func (w *WeightedDecisionInput) CalculateFinalScore() float64 {
	finalScore := 0.0
	finalScore += w.TechnicalSignal.Score * w.Specialization.TechnicalWeight
	finalScore += w.NewsSignal.Score * w.Specialization.NewsWeight
	finalScore += w.OnChainSignal.Score * w.Specialization.OnChainWeight
	finalScore += w.SentimentSignal.Score * w.Specialization.SentimentWeight
	return finalScore
}

// GetDominantSignal returns which signal type has the highest weighted contribution
func (w *WeightedDecisionInput) GetDominantSignal() string {
	scores := map[string]float64{
		"technical": w.TechnicalSignal.Score * w.Specialization.TechnicalWeight,
		"news":      w.NewsSignal.Score * w.Specialization.NewsWeight,
		"onchain":   w.OnChainSignal.Score * w.Specialization.OnChainWeight,
		"sentiment": w.SentimentSignal.Score * w.Specialization.SentimentWeight,
	}

	maxScore := 0.0
	dominant := "technical"
	for signal, score := range scores {
		if score > maxScore {
			maxScore = score
			dominant = signal
		}
	}
	return dominant
}
