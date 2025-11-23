package toolkit

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/selivandex/trader-bot/internal/indicators"
	"github.com/selivandex/trader-bot/pkg/logger"
	"github.com/selivandex/trader-bot/pkg/models"
)

// ============ Indicator Calculation Tools Implementation ============

// CalculateIndicators computes full set of indicators for any timeframe
func (t *LocalToolkit) CalculateIndicators(ctx context.Context, symbol, timeframe string) (*models.TechnicalIndicators, error) {
	logger.Debug("toolkit: calculate_indicators",
		zap.String("agent_id", t.agentID),
		zap.String("symbol", symbol),
		zap.String("timeframe", timeframe),
	)

	candles, err := t.GetCandles(ctx, symbol, timeframe, 100)
	if err != nil {
		return nil, fmt.Errorf("failed to get candles: %w", err)
	}

	if len(candles) < 26 {
		return nil, fmt.Errorf("insufficient candles for indicators: need 26, got %d", len(candles))
	}

	calc := indicators.NewCalculator()
	indicators, err := calc.Calculate(candles)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate indicators: %w", err)
	}

	return indicators, nil
}

// CalculateRSI computes RSI for any timeframe and period
func (t *LocalToolkit) CalculateRSI(ctx context.Context, symbol, timeframe string, period int) (float64, error) {
	logger.Debug("toolkit: calculate_rsi",
		zap.String("agent_id", t.agentID),
		zap.String("symbol", symbol),
		zap.String("timeframe", timeframe),
		zap.Int("period", period),
	)

	candles, err := t.GetCandles(ctx, symbol, timeframe, period*3) // Need extra candles for warmup
	if err != nil {
		return 0, fmt.Errorf("failed to get candles: %w", err)
	}

	calc := indicators.NewCalculator()
	rsi, err := calc.CalculateRSI(candles, period)
	if err != nil {
		return 0, fmt.Errorf("failed to calculate RSI: %w", err)
	}

	return rsi, nil
}

// CalculateEMA computes Exponential Moving Average
func (t *LocalToolkit) CalculateEMA(ctx context.Context, symbol, timeframe string, period int) (float64, error) {
	logger.Debug("toolkit: calculate_ema",
		zap.String("agent_id", t.agentID),
		zap.String("symbol", symbol),
		zap.String("timeframe", timeframe),
		zap.Int("period", period),
	)

	candles, err := t.GetCandles(ctx, symbol, timeframe, period*2)
	if err != nil {
		return 0, fmt.Errorf("failed to get candles: %w", err)
	}

	calc := indicators.NewCalculator()
	ema, err := calc.CalculateEMA(candles, period)
	if err != nil {
		return 0, fmt.Errorf("failed to calculate EMA: %w", err)
	}

	return ema, nil
}

// CalculateSMA computes Simple Moving Average
func (t *LocalToolkit) CalculateSMA(ctx context.Context, symbol, timeframe string, period int) (float64, error) {
	logger.Debug("toolkit: calculate_sma",
		zap.String("agent_id", t.agentID),
		zap.String("symbol", symbol),
		zap.String("timeframe", timeframe),
		zap.Int("period", period),
	)

	candles, err := t.GetCandles(ctx, symbol, timeframe, period*2)
	if err != nil {
		return 0, fmt.Errorf("failed to get candles: %w", err)
	}

	calc := indicators.NewCalculator()
	sma, err := calc.CalculateSMA(candles, period)
	if err != nil {
		return 0, fmt.Errorf("failed to calculate SMA: %w", err)
	}

	return sma, nil
}

// DetectTrend analyzes trend using moving averages
func (t *LocalToolkit) DetectTrend(ctx context.Context, symbol, timeframe string) (string, error) {
	logger.Debug("toolkit: detect_trend",
		zap.String("agent_id", t.agentID),
		zap.String("symbol", symbol),
		zap.String("timeframe", timeframe),
	)

	candles, err := t.GetCandles(ctx, symbol, timeframe, 100)
	if err != nil {
		return "", fmt.Errorf("failed to get candles: %w", err)
	}

	calc := indicators.NewCalculator()
	trend, err := calc.DetectTrend(candles)
	if err != nil {
		return "", fmt.Errorf("failed to detect trend: %w", err)
	}

	return trend, nil
}

// CalculateVolatility computes ATR (Average True Range)
func (t *LocalToolkit) CalculateVolatility(ctx context.Context, symbol, timeframe string, period int) (float64, error) {
	logger.Debug("toolkit: calculate_volatility",
		zap.String("agent_id", t.agentID),
		zap.String("symbol", symbol),
		zap.String("timeframe", timeframe),
		zap.Int("period", period),
	)

	candles, err := t.GetCandles(ctx, symbol, timeframe, period*2)
	if err != nil {
		return 0, fmt.Errorf("failed to get candles: %w", err)
	}

	calc := indicators.NewCalculator()
	volatility, err := calc.CalculateVolatility(candles, period)
	if err != nil {
		return 0, fmt.Errorf("failed to calculate volatility: %w", err)
	}

	return volatility, nil
}

// FindSupportLevels identifies key support levels from price history
func (t *LocalToolkit) FindSupportLevels(ctx context.Context, symbol, timeframe string, lookback int) ([]float64, error) {
	logger.Debug("toolkit: find_support_levels",
		zap.String("agent_id", t.agentID),
		zap.String("symbol", symbol),
		zap.String("timeframe", timeframe),
		zap.Int("lookback", lookback),
	)

	candles, err := t.GetCandles(ctx, symbol, timeframe, lookback)
	if err != nil {
		return nil, fmt.Errorf("failed to get candles: %w", err)
	}

	// Find local minima (support levels)
	supports := []float64{}
	for i := 2; i < len(candles)-2; i++ {
		current := candles[i].Low.InexactFloat64()
		prev1 := candles[i-1].Low.InexactFloat64()
		prev2 := candles[i-2].Low.InexactFloat64()
		next1 := candles[i+1].Low.InexactFloat64()
		next2 := candles[i+2].Low.InexactFloat64()

		// Local minimum
		if current < prev1 && current < prev2 && current < next1 && current < next2 {
			supports = append(supports, current)
		}
	}

	// Return top 5 most recent support levels
	if len(supports) > 5 {
		supports = supports[len(supports)-5:]
	}

	return supports, nil
}

// FindResistanceLevels identifies key resistance levels
func (t *LocalToolkit) FindResistanceLevels(ctx context.Context, symbol, timeframe string, lookback int) ([]float64, error) {
	logger.Debug("toolkit: find_resistance_levels",
		zap.String("agent_id", t.agentID),
		zap.String("symbol", symbol),
		zap.String("timeframe", timeframe),
		zap.Int("lookback", lookback),
	)

	candles, err := t.GetCandles(ctx, symbol, timeframe, lookback)
	if err != nil {
		return nil, fmt.Errorf("failed to get candles: %w", err)
	}

	// Find local maxima (resistance levels)
	resistances := []float64{}
	for i := 2; i < len(candles)-2; i++ {
		current := candles[i].High.InexactFloat64()
		prev1 := candles[i-1].High.InexactFloat64()
		prev2 := candles[i-2].High.InexactFloat64()
		next1 := candles[i+1].High.InexactFloat64()
		next2 := candles[i+2].High.InexactFloat64()

		// Local maximum
		if current > prev1 && current > prev2 && current > next1 && current > next2 {
			resistances = append(resistances, current)
		}
	}

	// Return top 5 most recent resistance levels
	if len(resistances) > 5 {
		resistances = resistances[len(resistances)-5:]
	}

	return resistances, nil
}

// IsNearSupport checks if price is near support level
func (t *LocalToolkit) IsNearSupport(ctx context.Context, symbol, timeframe string, currentPrice, threshold float64) (bool, error) {
	supports, err := t.FindSupportLevels(ctx, symbol, timeframe, 100)
	if err != nil {
		return false, err
	}

	for _, support := range supports {
		diff := absFloat(currentPrice-support) / currentPrice
		if diff < threshold {
			return true, nil
		}
	}

	return false, nil
}

// IsNearResistance checks if price is near resistance level
func (t *LocalToolkit) IsNearResistance(ctx context.Context, symbol, timeframe string, currentPrice, threshold float64) (bool, error) {
	resistances, err := t.FindResistanceLevels(ctx, symbol, timeframe, 100)
	if err != nil {
		return false, err
	}

	for _, resistance := range resistances {
		diff := absFloat(currentPrice-resistance) / currentPrice
		if diff < threshold {
			return true, nil
		}
	}

	return false, nil
}

// absFloat returns absolute value
func absFloat(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
