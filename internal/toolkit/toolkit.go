package toolkit

import (
	"context"
	"time"

	"github.com/selivandex/trader-bot/pkg/models"
)

// AgentToolkit provides safe read-only tools for agents to query cached data
// All tools read from local database cache, NEVER from exchange API directly
// This ensures low latency, no rate limits, and complete traceability
type AgentToolkit interface {
	// ============ Market Data Tools (from ohlcv_candles cache) ============

	// GetCandles retrieves OHLCV candles for any timeframe from cache
	GetCandles(ctx context.Context, symbol, timeframe string, limit int) ([]models.Candle, error)

	// GetCandleCount returns total number of cached candles for symbol/timeframe
	GetCandleCount(ctx context.Context, symbol, timeframe string) (int, error)

	// GetLatestPrice gets most recent price from candles cache
	GetLatestPrice(ctx context.Context, symbol, timeframe string) (float64, error)

	// ============ Indicator Calculation Tools ============

	// CalculateIndicators computes full set of indicators for any timeframe
	CalculateIndicators(ctx context.Context, symbol, timeframe string) (*models.TechnicalIndicators, error)

	// CalculateRSI computes RSI for any timeframe and period
	CalculateRSI(ctx context.Context, symbol, timeframe string, period int) (float64, error)

	// CalculateEMA computes Exponential Moving Average
	CalculateEMA(ctx context.Context, symbol, timeframe string, period int) (float64, error)

	// CalculateSMA computes Simple Moving Average
	CalculateSMA(ctx context.Context, symbol, timeframe string, period int) (float64, error)

	// DetectTrend analyzes trend using moving averages
	DetectTrend(ctx context.Context, symbol, timeframe string) (string, error) // "uptrend", "downtrend", "sideways"

	// CalculateVolatility computes ATR (Average True Range)
	CalculateVolatility(ctx context.Context, symbol, timeframe string, period int) (float64, error)

	// FindSupportLevels identifies key support levels from price history
	FindSupportLevels(ctx context.Context, symbol, timeframe string, lookback int) ([]float64, error)

	// FindResistanceLevels identifies key resistance levels
	FindResistanceLevels(ctx context.Context, symbol, timeframe string, lookback int) ([]float64, error)

	// IsNearSupport checks if price is near support level
	IsNearSupport(ctx context.Context, symbol, timeframe string, currentPrice, threshold float64) (bool, error)

	// IsNearResistance checks if price is near resistance level
	IsNearResistance(ctx context.Context, symbol, timeframe string, currentPrice, threshold float64) (bool, error)

	// ============ News Tools (from news_items cache) ============

	// SearchNews performs full-text search in cached news articles
	SearchNews(ctx context.Context, query string, since time.Duration, limit int) ([]models.NewsItem, error)

	// GetHighImpactNews filters news by AI-evaluated impact score (7+)
	GetHighImpactNews(ctx context.Context, minImpact int, since time.Duration) ([]models.NewsItem, error)

	// GetNewsBySentiment filters news by sentiment (positive > 0.5, negative < -0.5)
	GetNewsBySentiment(ctx context.Context, minSentiment, maxSentiment float64, since time.Duration) ([]models.NewsItem, error)

	// GetLatestNews gets most recent news articles
	GetLatestNews(ctx context.Context, limit int) ([]models.NewsItem, error)

	// GetNewsBySource filters news by source (CoinDesk, CoinTelegraph, etc)
	GetNewsBySource(ctx context.Context, source string, since time.Duration, limit int) ([]models.NewsItem, error)

	// CountNewsBySentiment counts positive/negative/neutral news
	CountNewsBySentiment(ctx context.Context, since time.Duration) (positive int, negative int, neutral int, err error)

	// SearchNewsSemantics performs semantic search by meaning (not just keywords)
	SearchNewsSemantics(ctx context.Context, semanticQuery string, since time.Duration, limit int) ([]models.NewsItem, error)

	// GetRelatedNews gets all news in same cluster (same event from different sources)
	GetRelatedNews(ctx context.Context, clusterID string) ([]models.NewsItem, error)

	// GetNewsWithMemoryContext combines news search with related personal memories
	// Returns formatted text with news + agent's past experiences + lessons learned
	GetNewsWithMemoryContext(ctx context.Context, newsQuery string, since time.Duration, newsLimit int) (string, error)

	// FindNewsRelatedToCurrentSituation finds news semantically related to agent's current reasoning
	FindNewsRelatedToCurrentSituation(ctx context.Context, situationDescription string, since time.Duration, limit int) ([]models.NewsItem, error)

	// ============ On-Chain Tools (from whale_transactions, exchange_flows cache) ============

	// GetRecentWhaleMovements gets whale transactions above threshold
	GetRecentWhaleMovements(ctx context.Context, symbol string, minAmountUSD float64, hours int) ([]models.WhaleTransaction, error)

	// GetNetExchangeFlow calculates net inflow/outflow over time period
	GetNetExchangeFlow(ctx context.Context, symbol string, hours int) (float64, error)

	// GetLargestWhaleTransaction finds biggest transaction in time window
	GetLargestWhaleTransaction(ctx context.Context, symbol string, hours int) (*models.WhaleTransaction, error)

	// GetWhaleAlertsSummary gets comprehensive whale activity analysis
	GetWhaleAlertsSummary(ctx context.Context, symbol string, hours int) (*WhaleAlertsSummary, error)

	// DetectWhalePattern detects accumulation/distribution pattern
	DetectWhalePattern(ctx context.Context, symbol string, hours int) (pattern string, strength float64, err error)

	// GetWhalesByExchange groups whale transactions by exchange
	GetWhalesByExchange(ctx context.Context, symbol string, hours int) (map[string][]models.WhaleTransaction, error)

	// CheckWhaleAlert checks for urgent mega-whale activity (>$10M)
	CheckWhaleAlert(ctx context.Context, symbol string) (*WhaleAlert, error)

	// ============ Memory Tools (agent's personal and collective memory) ============

	// SearchPersonalMemories queries agent's own semantic memory
	SearchPersonalMemories(ctx context.Context, query string, topK int) ([]models.SemanticMemory, error)

	// SearchCollectiveMemories queries collective wisdom of same personality agents
	SearchCollectiveMemories(ctx context.Context, personality, query string, topK int) ([]models.CollectiveMemory, error)

	// GetRecentMemories gets agent's most recent memories
	GetRecentMemories(ctx context.Context, limit int) ([]models.SemanticMemory, error)

	// ============ Performance Tools (agent's trading statistics) ============

	// GetRecentTrades gets agent's recent completed trades
	GetRecentTrades(ctx context.Context, symbol string, limit int) ([]TradeRecord, error)

	// GetWinRateBySignal calculates win rate breakdown by signal type
	GetWinRateBySignal(ctx context.Context, symbol string) (*SignalPerformanceStats, error)

	// GetCurrentStreak gets current winning/losing streak
	GetCurrentStreak(ctx context.Context, symbol string) (int, bool, error) // count, isWinning, error

	// ============ Risk Calculation Tools ============

	// CalculatePositionRisk calculates risk metrics for proposed position
	CalculatePositionRisk(ctx context.Context, symbol string, side models.PositionSide, size, leverage float64, stopLoss float64) (*PositionRiskMetrics, error)

	// SimulateWorstCase simulates worst case scenario
	SimulateWorstCase(ctx context.Context, symbol string, size, leverage float64) (*WorstCaseScenario, error)

	// CheckDrawdownRisk checks if position would exceed max drawdown
	CheckDrawdownRisk(ctx context.Context, agentID, symbol string, proposedLoss float64) (bool, error)

	// CalculateOptimalSize calculates optimal position size based on Kelly criterion
	CalculateOptimalSize(ctx context.Context, agentID, symbol string, winRate, avgWin, avgLoss float64) (float64, error)

	// ============ Pattern Recognition Tools ============

	// FindSimilarPatterns finds similar historical patterns
	FindSimilarPatterns(ctx context.Context, symbol, timeframe string, currentCandles []models.Candle, lookback int) ([]SimilarPattern, error)

	// GetPatternOutcome gets historical success rate of pattern
	GetPatternOutcome(ctx context.Context, patternHash string) (*PatternStats, error)

	// ============ Multi-Agent Tools ============

	// CompareWithPeers compares performance with same personality agents
	CompareWithPeers(ctx context.Context, personality, symbol string) (*PeerComparison, error)

	// GetTopPerformers gets top N performing agents
	GetTopPerformers(ctx context.Context, personality string, limit int) ([]AgentPerformance, error)

	// LearnFromBestAgent gets strategy from best performing peer
	LearnFromBestAgent(ctx context.Context, personality, symbol string) (*BestPractice, error)

	// ============ Correlation & Market Analysis Tools ============

	// GetCorrelation calculates correlation between two assets
	GetCorrelation(ctx context.Context, symbol1, symbol2 string, hours int) (float64, error)

	// CheckTimeframeAlignment checks if trends align across multiple timeframes
	CheckTimeframeAlignment(ctx context.Context, symbol string, timeframes []string) (map[string]string, error)

	// GetMarketRegime detects current market regime
	GetMarketRegime(ctx context.Context, symbol, timeframe string) (string, error) // "trending", "ranging", "volatile"

	// GetVolatilityTrend checks if volatility expanding or contracting
	GetVolatilityTrend(ctx context.Context, symbol string, hours int) (string, error) // "expanding", "contracting", "stable"

	// AnalyzeLiquidity calculates liquidity score
	AnalyzeLiquidity(ctx context.Context, symbol string) (float64, error) // 0-100

	// ============ Backtesting Tools ============

	// BacktestStrategy simulates strategy on historical data
	BacktestStrategy(ctx context.Context, symbol string, lookbackHours int) (*BacktestResult, error)

	// ============ Communication Tools ============

	// SendUrgentAlert sends urgent message to agent owner
	SendUrgentAlert(ctx context.Context, message string, priority string) error

	// LogThought logs agent's internal reasoning for transparency
	LogThought(ctx context.Context, thought string, confidence float64) error

	// RequestHumanInput asks owner for input (future feature)
	RequestHumanInput(ctx context.Context, question string, options []string) (string, error)

	// ============ Reporting Tools ============

	// GenerateDailyReport generates daily performance report
	GenerateDailyReport(ctx context.Context, date time.Time) (string, error)

	// GenerateWeeklyReport generates weekly summary
	GenerateWeeklyReport(ctx context.Context, weekStart time.Time) (string, error)

	// SendDailyReportToOwner generates and sends daily report
	SendDailyReportToOwner(ctx context.Context) error
}

// PositionRiskMetrics contains risk calculations for proposed position
type PositionRiskMetrics struct {
	MaxLoss           float64 // Maximum loss if stop loss hits
	LiquidationPrice  float64 // Price at which position is liquidated
	RiskPercent       float64 // Risk as % of balance
	RiskRewardRatio   float64 // Reward/risk ratio
	RequiredMargin    float64 // Margin required
	ProbabilityProfit float64 // Estimated probability (from historical data)
	RiskScore         int     // 1-10, higher = riskier
}

// WorstCaseScenario contains worst case analysis
type WorstCaseScenario struct {
	MaxLossUSD        float64 // Maximum possible loss in USD
	MaxLossPercent    float64 // Maximum loss as % of balance
	LiquidationRisk   string  // "low", "medium", "high"
	TimeToLiquidation float64 // Estimated hours to liquidation at current volatility
	Recovery          string  // How many winning trades needed to recover
}

// SimilarPattern represents similar historical pattern
type SimilarPattern struct {
	StartTime      time.Time
	Similarity     float64 // 0.0-1.0, cosine similarity
	Outcome        string  // "up", "down", "sideways"
	OutcomePercent float64 // +5.2%, -3.1%, etc
	Duration       time.Duration
	PatternHash    string
}

// PatternStats contains statistics for pattern type
type PatternStats struct {
	PatternHash      string
	TotalOccurrences int
	BullishCount     int
	BearishCount     int
	SuccessRate      float64
	AvgOutcome       float64
	BestOutcome      float64
	WorstOutcome     float64
}

// PeerComparison compares agent with peers
type PeerComparison struct {
	MyPerformance AgentPerformance
	PeersAvg      AgentPerformance
	TopPeer       AgentPerformance
	MyRank        int
	TotalPeers    int
	StrengthAreas []string
	WeaknessAreas []string
}

// AgentPerformance represents agent performance metrics
type AgentPerformance struct {
	AgentID        string
	AgentName      string
	WinRate        float64
	TotalPnL       float64
	SharpeRatio    float64
	MaxDrawdown    float64
	TotalTrades    int
	Specialization models.AgentSpecialization
}

// BestPractice contains lessons from best performing agent
type BestPractice struct {
	TopAgentID         string
	TopAgentPnL        float64
	KeyDifferences     map[string]float64 // "onchain_weight": +0.15
	RecommendedActions []string
	ConfidenceScore    float64
}

// TradeRecord represents a completed trade with outcome
type TradeRecord struct {
	Symbol    string
	Side      string
	EntryTime time.Time
	ExitTime  time.Time
	PnL       float64
	PnLPct    float64
	Reason    string
}

// SignalPerformanceStats shows agent's performance by signal type
type SignalPerformanceStats struct {
	Technical SignalStats
	News      SignalStats
	OnChain   SignalStats
	Sentiment SignalStats
}

// SignalStats holds statistics for one signal type
type SignalStats struct {
	TotalTrades   int
	WinningTrades int
	LosingTrades  int
	WinRate       float64
	AvgPnL        float64
	BestTrade     float64
	WorstTrade    float64
}

// ToolCall represents one tool invocation with result
type ToolCall struct {
	ToolName   string                 `json:"tool_name"`
	Parameters map[string]interface{} `json:"parameters"`
	Result     interface{}            `json:"result,omitempty"`
	Error      string                 `json:"error,omitempty"`
	StartTime  time.Time              `json:"start_time"`
	Latency    time.Duration          `json:"latency"`
	Success    bool                   `json:"success"`
}

// ToolUsageTrace captures all tool calls during agent's thinking
type ToolUsageTrace struct {
	SessionID string        `json:"session_id"`
	AgentID   string        `json:"agent_id"`
	ToolCalls []ToolCall    `json:"tool_calls"`
	TotalTime time.Duration `json:"total_time"`
	StartTime time.Time     `json:"start_time"`
	EndTime   time.Time     `json:"end_time"`
}

// NewToolUsageTrace creates new trace
func NewToolUsageTrace(sessionID, agentID string) *ToolUsageTrace {
	return &ToolUsageTrace{
		SessionID: sessionID,
		AgentID:   agentID,
		ToolCalls: []ToolCall{},
		StartTime: time.Now(),
	}
}

// AddToolCall adds tool call to trace
func (t *ToolUsageTrace) AddToolCall(call ToolCall) {
	t.ToolCalls = append(t.ToolCalls, call)
}

// Finish completes trace
func (t *ToolUsageTrace) Finish() {
	t.EndTime = time.Now()
	t.TotalTime = t.EndTime.Sub(t.StartTime)
}
