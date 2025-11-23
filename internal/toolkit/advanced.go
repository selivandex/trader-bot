package toolkit

import (
	"context"
	"fmt"
	"math"
	"time"

	"go.uber.org/zap"

	"github.com/selivandex/trader-bot/internal/indicators"
	"github.com/selivandex/trader-bot/pkg/logger"
	"github.com/selivandex/trader-bot/pkg/models"
)

// ============ Advanced Tools Implementation ============

// GetCorrelation calculates correlation between two assets
func (t *LocalToolkit) GetCorrelation(ctx context.Context, symbol1, symbol2 string, hours int) (float64, error) {
	logger.Debug("toolkit: get_correlation",
		zap.String("agent_id", t.agentID),
		zap.String("symbol1", symbol1),
		zap.String("symbol2", symbol2),
		zap.Int("hours", hours),
	)

	// Get candles for both symbols
	limit := hours / 1 // 1h candles
	candles1, err := t.GetCandles(ctx, symbol1, "1h", limit)
	if err != nil {
		return 0, fmt.Errorf("failed to get candles for %s: %w", symbol1, err)
	}

	candles2, err := t.GetCandles(ctx, symbol2, "1h", limit)
	if err != nil {
		return 0, fmt.Errorf("failed to get candles for %s: %w", symbol2, err)
	}

	// Calculate returns for both
	returns1 := calculateReturns(candles1)
	returns2 := calculateReturns(candles2)

	// Calculate Pearson correlation
	correlation := pearsonCorrelation(returns1, returns2)

	return correlation, nil
}

// CheckTimeframeAlignment checks if trends align across timeframes
func (t *LocalToolkit) CheckTimeframeAlignment(ctx context.Context, symbol string, timeframes []string) (map[string]string, error) {
	logger.Debug("toolkit: check_timeframe_alignment",
		zap.String("agent_id", t.agentID),
		zap.String("symbol", symbol),
		zap.Strings("timeframes", timeframes),
	)

	alignment := make(map[string]string)

	for _, tf := range timeframes {
		trend, err := t.DetectTrend(ctx, symbol, tf)
		if err != nil {
			alignment[tf] = "unknown"
			continue
		}
		alignment[tf] = trend
	}

	return alignment, nil
}

// GetMarketRegime detects current market regime
func (t *LocalToolkit) GetMarketRegime(ctx context.Context, symbol, timeframe string) (string, error) {
	logger.Debug("toolkit: get_market_regime",
		zap.String("agent_id", t.agentID),
		zap.String("symbol", symbol),
		zap.String("timeframe", timeframe),
	)

	// Get candles
	candles, err := t.GetCandles(ctx, symbol, timeframe, 100)
	if err != nil {
		return "", fmt.Errorf("failed to get candles: %w", err)
	}

	// Calculate volatility
	volatility, err := t.CalculateVolatility(ctx, symbol, timeframe, 14)
	if err != nil {
		return "unknown", err
	}

	// Detect trend
	trend, err := t.DetectTrend(ctx, symbol, timeframe)
	if err != nil {
		return "unknown", err
	}

	// Calculate price range
	var high, low float64
	for i, candle := range candles {
		price := candle.Close.InexactFloat64()
		if i == 0 || price > high {
			high = price
		}
		if i == 0 || price < low {
			low = price
		}
	}
	priceRange := (high - low) / low * 100

	// Determine regime
	if volatility > 800 && priceRange > 15 {
		return "volatile", nil
	} else if trend != "sideways" && priceRange > 10 {
		return "trending", nil
	} else if priceRange < 5 {
		return "ranging", nil
	}

	return "mixed", nil
}

// GetVolatilityTrend checks if volatility is expanding or contracting
func (t *LocalToolkit) GetVolatilityTrend(ctx context.Context, symbol string, hours int) (string, error) {
	logger.Debug("toolkit: get_volatility_trend",
		zap.String("agent_id", t.agentID),
		zap.String("symbol", symbol),
		zap.Int("hours", hours),
	)

	// Get recent and older volatility
	recentVol, err := t.CalculateVolatility(ctx, symbol, "1h", 14)
	if err != nil {
		return "unknown", err
	}

	// Calculate volatility from earlier period
	candles, err := t.GetCandles(ctx, symbol, "1h", hours+14)
	if err != nil {
		return "unknown", err
	}

	if len(candles) < 50 {
		return "unknown", fmt.Errorf("insufficient data")
	}

	olderCandles := candles[:len(candles)-14]
	calc := indicators.NewCalculator()
	olderVol, err := calc.CalculateVolatility(olderCandles, 14)
	if err != nil {
		return "unknown", err
	}

	// Compare
	change := (recentVol - olderVol) / olderVol * 100

	if change > 20 {
		return "expanding", nil
	} else if change < -20 {
		return "contracting", nil
	}

	return "stable", nil
}

// AnalyzeLiquidity analyzes market liquidity
func (t *LocalToolkit) AnalyzeLiquidity(ctx context.Context, symbol string) (float64, error) {
	logger.Debug("toolkit: analyze_liquidity",
		zap.String("agent_id", t.agentID),
		zap.String("symbol", symbol),
	)

	// Get recent volume
	candles, err := t.GetCandles(ctx, symbol, "1h", 24)
	if err != nil {
		return 0, fmt.Errorf("failed to get candles: %w", err)
	}

	// Calculate average volume
	totalVolume := 0.0
	for _, candle := range candles {
		totalVolume += candle.Volume.InexactFloat64()
	}
	avgVolume := totalVolume / float64(len(candles))

	// Liquidity score: higher volume = higher liquidity
	// Normalize to 0-100 scale
	// $100M+ avg volume = 100/100
	// $10M avg volume = 50/100
	// $1M avg volume = 20/100
	liquidityScore := math.Log10(avgVolume/1_000_000) * 25

	if liquidityScore < 0 {
		liquidityScore = 0
	}
	if liquidityScore > 100 {
		liquidityScore = 100
	}

	return liquidityScore, nil
}

// BacktestStrategy simulates strategy on historical data (simplified)
func (t *LocalToolkit) BacktestStrategy(ctx context.Context, symbol string, lookbackHours int) (*BacktestResult, error) {
	logger.Debug("toolkit: backtest_strategy",
		zap.String("agent_id", t.agentID),
		zap.String("symbol", symbol),
		zap.Int("lookback_hours", lookbackHours),
	)

	// Get agent's current weights
	agent, err := t.agentRepo.GetAgent(ctx, t.agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent config: %w", err)
	}

	// Get historical performance
	metrics, err := t.agentRepo.GetAgentPerformanceMetrics(ctx, t.agentID, symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get metrics: %w", err)
	}

	// Simple backtest result based on actual performance
	return &BacktestResult{
		Symbol:         symbol,
		Period:         time.Duration(lookbackHours) * time.Hour,
		TotalTrades:    metrics.TotalTrades,
		WinningTrades:  metrics.WinningTrades,
		LosingTrades:   metrics.LosingTrades,
		WinRate:        metrics.WinRate,
		TotalReturn:    metrics.TotalPnL,
		SharpeRatio:    metrics.SharpeRatio,
		MaxDrawdown:    0, // TODO
		BestTrade:      metrics.MaxWin,
		WorstTrade:     metrics.MaxLoss,
		StrategyWeights: agent.Specialization,
	}, nil
}

// BacktestResult contains backtest results
type BacktestResult struct {
	Symbol          string
	Period          time.Duration
	TotalTrades     int
	WinningTrades   int
	LosingTrades    int
	WinRate         float64
	TotalReturn     float64
	SharpeRatio     float64
	MaxDrawdown     float64
	BestTrade       float64
	WorstTrade      float64
	StrategyWeights models.AgentSpecialization
}

// Helper functions

func calculateReturns(candles []models.Candle) []float64 {
	if len(candles) < 2 {
		return []float64{}
	}

	returns := make([]float64, len(candles)-1)
	for i := 1; i < len(candles); i++ {
		prev := candles[i-1].Close.InexactFloat64()
		curr := candles[i].Close.InexactFloat64()
		returns[i-1] = (curr - prev) / prev
	}

	return returns
}

func pearsonCorrelation(x, y []float64) float64 {
	if len(x) != len(y) || len(x) == 0 {
		return 0
	}

	n := float64(len(x))

	// Calculate means
	var sumX, sumY float64
	for i := 0; i < len(x); i++ {
		sumX += x[i]
		sumY += y[i]
	}
	meanX := sumX / n
	meanY := sumY / n

	// Calculate correlation
	var numerator, denomX, denomY float64
	for i := 0; i < len(x); i++ {
		diffX := x[i] - meanX
		diffY := y[i] - meanY
		numerator += diffX * diffY
		denomX += diffX * diffX
		denomY += diffY * diffY
	}

	if denomX == 0 || denomY == 0 {
		return 0
	}

	return numerator / (math.Sqrt(denomX) * math.Sqrt(denomY))
}

