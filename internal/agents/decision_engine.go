package agents

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/selivandex/trader-bot/internal/adapters/ai"
	"github.com/selivandex/trader-bot/pkg/logger"
	"github.com/selivandex/trader-bot/pkg/models"
)

// DecisionEngine makes trading decisions using AI providers with weighted signals
type DecisionEngine struct {
	config         *models.AgentConfig
	aiProvider     ai.Provider // Agent uses its own AI provider
	signalAnalyzer *SignalAnalyzer
}

// NewDecisionEngine creates new decision engine for agent
func NewDecisionEngine(config *models.AgentConfig, aiProvider ai.Provider) *DecisionEngine {
	return &DecisionEngine{
		config:         config,
		aiProvider:     aiProvider,
		signalAnalyzer: NewSignalAnalyzer(config),
	}
}

// Analyze analyzes market data and returns agent's AI-powered decision with weighted signals
func (e *DecisionEngine) Analyze(ctx context.Context, marketData *models.MarketData, position *models.Position, balance, equity, dailyPnL float64) (*models.AgentDecision, error) {
	logger.Debug("agent analyzing market data with AI",
		zap.String("agent", e.config.Name),
		zap.String("symbol", marketData.Symbol),
		zap.String("ai_provider", e.aiProvider.GetName()),
	)

	// Step 1: Analyze individual signals (without AI)
	signalScores := e.signalAnalyzer.AnalyzeSignals(marketData)

	// Step 2: Build specialized prompt based on agent's personality and weights
	prompt := e.buildAgentPrompt(marketData, position, signalScores, balance, equity, dailyPnL)

	// Step 3: Query AI provider for final decision
	aiDecision, err := e.aiProvider.Analyze(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("AI analysis failed: %w", err)
	}

	// Step 4: Validate AI decision meets agent's confidence threshold
	if aiDecision.Confidence < e.config.Strategy.MinConfidenceThreshold {
		logger.Debug("AI confidence below threshold, changing to HOLD",
			zap.String("agent", e.config.Name),
			zap.Int("confidence", aiDecision.Confidence),
			zap.Int("threshold", e.config.Strategy.MinConfidenceThreshold),
		)
		aiDecision.Action = models.ActionHold
	}

	// Step 5: Create agent decision with signal breakdown
	agentDecision := &models.AgentDecision{
		AgentID:        e.config.ID,
		Symbol:         marketData.Symbol,
		Action:         aiDecision.Action,
		Confidence:     aiDecision.Confidence,
		Reason:         e.formatDecisionReason(aiDecision, signalScores),
		TechnicalScore: signalScores.Technical.Score,
		NewsScore:      signalScores.News.Score,
		OnChainScore:   signalScores.OnChain.Score,
		SentimentScore: signalScores.Sentiment.Score,
		FinalScore:     float64(aiDecision.Confidence),
		Executed:       false,
	}

	logger.Info("agent AI decision made",
		zap.String("agent", e.config.Name),
		zap.String("ai_provider", e.aiProvider.GetName()),
		zap.String("action", string(aiDecision.Action)),
		zap.Int("confidence", aiDecision.Confidence),
		zap.Float64("tech_score", signalScores.Technical.Score),
		zap.Float64("news_score", signalScores.News.Score),
	)

	return agentDecision, nil
}

// buildAgentPrompt builds AI prompt customized for agent's personality and signal weights
func (e *DecisionEngine) buildAgentPrompt(
	marketData *models.MarketData,
	position *models.Position,
	signals *SignalScores,
	balance, equity, dailyPnL float64,
) *models.TradingPrompt {
	// Get agent's personality system prompt
	systemPrompt := GetAgentSystemPrompt(e.config.Personality, e.config.Name)

	// Create prompt with actual balance from agent state
	prompt := &models.TradingPrompt{
		MarketData:      marketData,
		CurrentPosition: position,
		Balance:         models.NewDecimal(balance),
		Equity:          models.NewDecimal(equity),
		DailyPnL:        models.NewDecimal(dailyPnL),
	}

	// Add agent personality to prompt
	// The AI provider should prepend this system prompt to understand its role
	// Note: This is a simplified version. In production, we'd pass systemPrompt
	// separately to the AI provider's Analyze method

	// For now, we rely on the formatDecisionReason to include personality context
	// The real magic happens when AI providers use the system prompt
	_ = systemPrompt // Will be used when we implement full agentic AI provider

	return prompt
}

// formatDecisionReason formats the AI decision reason with signal breakdown
func (e *DecisionEngine) formatDecisionReason(aiDecision *models.AIDecision, signals *SignalScores) string {
	// Prepare signal data for template
	signalsMap := map[string]ai.SignalWithWeight{
		"technical": {
			Score:     signals.Technical.Score,
			Weight:    e.config.Specialization.TechnicalWeight,
			Direction: signals.Technical.Direction,
		},
		"news": {
			Score:     signals.News.Score,
			Weight:    e.config.Specialization.NewsWeight,
			Direction: signals.News.Direction,
		},
		"onchain": {
			Score:     signals.OnChain.Score,
			Weight:    e.config.Specialization.OnChainWeight,
			Direction: signals.OnChain.Direction,
		},
		"sentiment": {
			Score:     signals.Sentiment.Score,
			Weight:    e.config.Specialization.SentimentWeight,
			Direction: signals.Sentiment.Direction,
		},
	}

	return ai.FormatDecisionReason(
		string(e.config.Personality),
		e.aiProvider.GetName(),
		aiDecision.Reason,
		signalsMap,
	)
}

// SignalAnalyzer analyzes market signals without AI
type SignalAnalyzer struct {
	config *models.AgentConfig
}

// SignalScores holds all analyzed signal scores
type SignalScores struct {
	Technical models.SignalScore
	News      models.SignalScore
	OnChain   models.SignalScore
	Sentiment models.SignalScore
}

// NewSignalAnalyzer creates new signal analyzer
func NewSignalAnalyzer(config *models.AgentConfig) *SignalAnalyzer {
	return &SignalAnalyzer{config: config}
}

// AnalyzeSignals analyzes all market signals
func (sa *SignalAnalyzer) AnalyzeSignals(marketData *models.MarketData) *SignalScores {
	technical := sa.analyzeTechnicalSignals(marketData)
	news := sa.analyzeNewsSignals(marketData)
	onChain := sa.analyzeOnChainSignals(marketData)
	sentiment := sa.analyzeSentimentSignals(marketData)

	// Apply contrarian inversion if configured
	if sa.config.InvertSentiment {
		sentiment = invertSignal(sentiment)
	}

	return &SignalScores{
		Technical: technical,
		News:      news,
		OnChain:   onChain,
		Sentiment: sentiment,
	}
}

// analyzeTechnicalSignals analyzes technical indicators
func (sa *SignalAnalyzer) analyzeTechnicalSignals(marketData *models.MarketData) models.SignalScore {
	if marketData.Indicators == nil {
		return models.SignalScore{Score: 50, Confidence: 0.0, Direction: "neutral", Reason: "No indicators"}
	}

	indicators := marketData.Indicators
	score := 50.0
	signals := []string{}

	// RSI analysis
	if rsi14, ok := indicators.RSI["14"]; ok {
		rsiVal := rsi14.InexactFloat64()
		if rsiVal < 30 {
			score += 15
			signals = append(signals, "RSI oversold")
		} else if rsiVal > 70 {
			score -= 15
			signals = append(signals, "RSI overbought")
		}
	}

	// MACD analysis
	if indicators.MACD != nil {
		histogram := indicators.MACD.Histogram.InexactFloat64()
		if histogram > 0 {
			score += 10
			signals = append(signals, "MACD bullish")
		} else {
			score -= 10
			signals = append(signals, "MACD bearish")
		}
	}

	// Bollinger Bands
	if indicators.BollingerBands != nil {
		currentPrice := marketData.Ticker.Last
		lower := indicators.BollingerBands.Lower
		upper := indicators.BollingerBands.Upper

		if currentPrice.LessThan(lower) {
			score += 15
			signals = append(signals, "Price below BB lower")
		} else if currentPrice.GreaterThan(upper) {
			score -= 15
			signals = append(signals, "Price above BB upper")
		}
	}

	// Volume confirmation
	if indicators.Volume != nil {
		ratio := indicators.Volume.Ratio.InexactFloat64()
		if ratio > 1.5 {
			if score > 50 {
				score += 10
			} else {
				score -= 10
			}
			signals = append(signals, fmt.Sprintf("High volume (%.1fx)", ratio))
		}
	}

	score = clampScore(score)
	direction := getDirection(score)

	return models.SignalScore{
		Score:      score,
		Confidence: 0.8,
		Direction:  direction,
		Reason:     fmt.Sprintf("Technical: %v", signals),
	}
}

// analyzeNewsSignals analyzes news sentiment
func (sa *SignalAnalyzer) analyzeNewsSignals(marketData *models.MarketData) models.SignalScore {
	if marketData.NewsSummary == nil || marketData.NewsSummary.TotalItems == 0 {
		return models.SignalScore{Score: 50, Confidence: 0.0, Direction: "neutral", Reason: "No news"}
	}

	news := marketData.NewsSummary
	score := (news.AverageSentiment + 1) * 50 // Convert -1/+1 to 0-100

	// Analyze recent news for high impact items
	highImpactCount := 0
	for _, item := range news.RecentNews {
		if float64(item.Impact) >= sa.config.MinNewsImpact {
			highImpactCount++
			score += item.Sentiment * 10
		}
	}

	score = clampScore(score)
	direction := getDirection(score)
	confidence := minFloat(1.0, float64(news.TotalItems)/50.0+float64(highImpactCount)/5.0)

	return models.SignalScore{
		Score:      score,
		Confidence: confidence,
		Direction:  direction,
		Reason:     fmt.Sprintf("%s (%.2f), %d items", news.OverallSentiment, news.AverageSentiment, news.TotalItems),
	}
}

// analyzeOnChainSignals analyzes on-chain data
func (sa *SignalAnalyzer) analyzeOnChainSignals(marketData *models.MarketData) models.SignalScore {
	if marketData.OnChainData == nil {
		return models.SignalScore{Score: 50, Confidence: 0.0, Direction: "neutral", Reason: "No on-chain data"}
	}

	onchain := marketData.OnChainData
	score := 50.0
	signals := []string{}

	// Exchange flow
	if onchain.NetExchangeFlow.Abs().GreaterThan(models.NewDecimal(1_000_000)) {
		flowUSD := onchain.NetExchangeFlow.InexactFloat64()
		if flowUSD < 0 {
			score += 20
			signals = append(signals, fmt.Sprintf("Outflow: $%.1fM", -flowUSD/1_000_000))
		} else {
			score -= 20
			signals = append(signals, fmt.Sprintf("Inflow: $%.1fM", flowUSD/1_000_000))
		}
	}

	// Whale transactions
	minThreshold := sa.config.MinWhaleTransaction.InexactFloat64()
	whaleCount := 0
	for _, tx := range onchain.RecentWhaleMovements {
		if tx.AmountUSD.InexactFloat64() >= minThreshold {
			whaleCount++
			switch tx.TransactionType {
			case "exchange_outflow":
				score += 5
			case "exchange_inflow":
				score -= 5
			}
		}
	}

	if whaleCount > 0 {
		signals = append(signals, fmt.Sprintf("%d whale txs", whaleCount))
	}

	score = clampScore(score)
	direction := getDirection(score)
	confidence := minFloat(1.0, float64(len(onchain.RecentWhaleMovements))/10.0)

	return models.SignalScore{
		Score:      score,
		Confidence: confidence,
		Direction:  direction,
		Reason:     fmt.Sprintf("On-chain: %v", signals),
	}
}

// analyzeSentimentSignals analyzes market sentiment
func (sa *SignalAnalyzer) analyzeSentimentSignals(marketData *models.MarketData) models.SignalScore {
	score := 50.0
	signals := []string{}

	// Funding rate
	fundingRate := marketData.FundingRate.InexactFloat64()
	if fundingRate > 0.01 {
		score -= 15
		signals = append(signals, "High funding (crowded longs)")
	} else if fundingRate < -0.01 {
		score += 15
		signals = append(signals, "Negative funding (crowded shorts)")
	}

	// Price momentum
	change24h := marketData.Ticker.Change24h.InexactFloat64()
	if change24h > 5 {
		score += 10
		signals = append(signals, "Strong rally")
	} else if change24h < -5 {
		score -= 10
		signals = append(signals, "Strong decline")
	}

	score = clampScore(score)
	direction := getDirection(score)

	return models.SignalScore{
		Score:      score,
		Confidence: 0.6,
		Direction:  direction,
		Reason:     fmt.Sprintf("Sentiment: %v", signals),
	}
}

// Helper functions

func invertSignal(signal models.SignalScore) models.SignalScore {
	inverted := signal
	inverted.Score = 100 - signal.Score

	switch signal.Direction {
	case "bullish":
		inverted.Direction = "bearish"
	case "bearish":
		inverted.Direction = "bullish"
	}

	inverted.Reason = "INVERTED: " + signal.Reason
	return inverted
}

func clampScore(score float64) float64 {
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return score
}

func getDirection(score float64) string {
	if score > 60 {
		return "bullish"
	} else if score < 40 {
		return "bearish"
	}
	return "neutral"
}
