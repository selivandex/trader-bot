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
	ID                  string              `json:"id" db:"id"`
	UserID              string              `json:"user_id" db:"user_id"`
	Name                string              `json:"name" db:"name"`
	Personality         AgentPersonality    `json:"personality" db:"personality"`
	Specialization      AgentSpecialization `json:"specialization" db:"specialization"`       // JSONB
	Strategy            StrategyParameters  `json:"strategy" db:"strategy"`                   // JSONB
	ValidationConfig    *ValidationConfig   `json:"validation_config" db:"validation_config"` // JSONB
	DecisionInterval    time.Duration       `json:"decision_interval" db:"decision_interval"`
	MinNewsImpact       float64             `json:"min_news_impact" db:"min_news_impact"`
	MinWhaleTransaction decimal.Decimal     `json:"min_whale_transaction" db:"min_whale_transaction"`
	InvertSentiment     bool                `json:"invert_sentiment" db:"invert_sentiment"`
	LearningRate        float64             `json:"learning_rate" db:"learning_rate"` // 0.0 - 1.0
	IsActive            bool                `json:"is_active" db:"is_active"`
	CreatedAt           time.Time           `json:"created_at" db:"created_at"`
	UpdatedAt           time.Time           `json:"updated_at" db:"updated_at"`
}

// ValidationConfig configures validator council for agent
type ValidationConfig struct {
	Enabled                    bool    `json:"enabled"`
	MinConfidenceForValidation int     `json:"min_confidence_for_validation"` // 60 = validate if confidence >= 60%
	ConsensusThreshold         float64 `json:"consensus_threshold"`           // 0.66 = 2/3 must approve
	RequireUnanimous           bool    `json:"require_unanimous"`             // If true, ALL validators must approve
	ValidateOnlyHighRisk       bool    `json:"validate_only_high_risk"`       // Only validate high leverage/size trades
}

// AgentState represents runtime state of an agent
type AgentState struct {
	ID             string          `json:"id" db:"id"`
	AgentID        string          `json:"agent_id" db:"agent_id"`
	Symbol         string          `json:"symbol" db:"symbol"`
	Balance        decimal.Decimal `json:"balance" db:"balance"`
	InitialBalance decimal.Decimal `json:"initial_balance" db:"initial_balance"`
	Equity         decimal.Decimal `json:"equity" db:"equity"`
	PnL            decimal.Decimal `json:"pnl" db:"pnl"`
	TotalTrades    int             `json:"total_trades" db:"total_trades"`
	WinningTrades  int             `json:"winning_trades" db:"winning_trades"`
	LosingTrades   int             `json:"losing_trades" db:"losing_trades"`
	WinRate        float64         `json:"win_rate" db:"win_rate"`
	IsTrading      bool            `json:"is_trading" db:"is_trading"`
	UpdatedAt      time.Time       `json:"updated_at" db:"updated_at"`
}

// AgentDecision represents a decision made by an agent
type AgentDecision struct {
	ID             string          `json:"id" db:"id"`
	AgentID        string          `json:"agent_id" db:"agent_id"`
	Symbol         string          `json:"symbol" db:"symbol"`
	Action         AIAction        `json:"action" db:"action"`
	Confidence     int             `json:"confidence" db:"confidence"`
	Reason         string          `json:"reason" db:"reason"`
	TechnicalScore float64         `json:"technical_score" db:"technical_score"`
	NewsScore      float64         `json:"news_score" db:"news_score"`
	OnChainScore   float64         `json:"onchain_score" db:"onchain_score"`
	SentimentScore float64         `json:"sentiment_score" db:"sentiment_score"`
	FinalScore     float64         `json:"final_score" db:"final_score"`
	MarketData     string          `json:"market_data" db:"market_data"` // JSONB
	Executed       bool            `json:"executed" db:"executed"`
	ExecutionPrice decimal.Decimal `json:"execution_price" db:"execution_price"`
	ExecutionSize  decimal.Decimal `json:"execution_size" db:"execution_size"`
	Outcome        string          `json:"outcome" db:"outcome"` // JSONB with PnL, duration, etc
	CreatedAt      time.Time       `json:"created_at" db:"created_at"`
}

// AgentMemory stores learning data for adaptation
type AgentMemory struct {
	ID                    string    `json:"id" db:"id"`
	AgentID               string    `json:"agent_id" db:"agent_id"`
	TechnicalSuccessRate  float64   `json:"technical_success_rate" db:"technical_success_rate"`
	NewsSuccessRate       float64   `json:"news_success_rate" db:"news_success_rate"`
	OnChainSuccessRate    float64   `json:"onchain_success_rate" db:"onchain_success_rate"`
	SentimentSuccessRate  float64   `json:"sentiment_success_rate" db:"sentiment_success_rate"`
	BestMarketConditions  string    `json:"best_market_conditions" db:"best_market_conditions"`   // JSONB
	WorstMarketConditions string    `json:"worst_market_conditions" db:"worst_market_conditions"` // JSONB
	TotalDecisions        int       `json:"total_decisions" db:"total_decisions"`
	AdaptationCount       int       `json:"adaptation_count" db:"adaptation_count"`
	LastAdaptedAt         time.Time `json:"last_adapted_at" db:"last_adapted_at"`
	UpdatedAt             time.Time `json:"updated_at" db:"updated_at"`
}

// AgentTournament represents a competition between agents
type AgentTournament struct {
	ID            string          `json:"id" db:"id"`
	UserID        string          `json:"user_id" db:"user_id"`
	Name          string          `json:"name" db:"name"`
	Symbols       []string        `json:"symbols" db:"symbols"` // Array
	StartBalance  decimal.Decimal `json:"start_balance" db:"start_balance"`
	Duration      time.Duration   `json:"duration" db:"duration"`
	StartedAt     time.Time       `json:"started_at" db:"started_at"`
	EndedAt       *time.Time      `json:"ended_at" db:"ended_at"`
	IsActive      bool            `json:"is_active" db:"is_active"`
	WinnerAgentID *string         `json:"winner_agent_id" db:"winner_agent_id"`
	Results       string          `json:"results" db:"results"` // JSONB
	CreatedAt     time.Time       `json:"created_at" db:"created_at"`
}

// AgentScore represents tournament performance metrics
type AgentScore struct {
	AgentID      string          `json:"agent_id"`
	AgentName    string          `json:"agent_name"`
	TotalReturn  decimal.Decimal `json:"total_return"`
	ReturnPct    float64         `json:"return_pct"`
	WinRate      float64         `json:"win_rate"`
	MaxDrawdown  float64         `json:"max_drawdown"`
	SharpeRatio  float64         `json:"sharpe_ratio"`
	TradeCount   int             `json:"trade_count"`
	ProfitFactor float64         `json:"profit_factor"`
	AvgTradePnL  decimal.Decimal `json:"avg_trade_pnl"`
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
	Score      float64 // 0-100
	Confidence float64 // 0-1
	Direction  string  // "bullish", "bearish", "neutral"
	Reason     string
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
