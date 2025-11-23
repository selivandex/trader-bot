package indicators

import (
	"fmt"

	"github.com/shopspring/decimal"

	"github.com/cinar/indicator"
	"github.com/selivandex/trader-bot/pkg/models"
)

// Calculator calculates technical indicators from candle data
type Calculator struct{}

// NewCalculator creates new indicator calculator
func NewCalculator() *Calculator {
	return &Calculator{}
}

// Calculate calculates all technical indicators from candles
func (c *Calculator) Calculate(candles []models.Candle) (*models.TechnicalIndicators, error) {
	if len(candles) < 26 {
		return nil, fmt.Errorf("insufficient candles for indicators (need at least 26, got %d)", len(candles))
	}

	// Extract price and volume data
	closes := make([]float64, len(candles))
	highs := make([]float64, len(candles))
	lows := make([]float64, len(candles))
	volumes := make([]float64, len(candles))

	for i, candle := range candles {
		closes[i], _ = candle.Close.Float64()
		highs[i], _ = candle.High.Float64()
		lows[i], _ = candle.Low.Float64()
		volumes[i], _ = candle.Volume.Float64()
	}

	// Calculate RSI (period 14)
	_, rsi14 := indicator.Rsi(closes)
	if len(rsi14) < 14 {
		return nil, fmt.Errorf("insufficient RSI data")
	}
	rsi14 = rsi14[13:] // Skip first 13 values (warmup period)

	// Calculate MACD
	macdLine, signalLine := indicator.Macd(closes)
	histogram := make([]float64, len(macdLine))
	for i := range macdLine {
		histogram[i] = macdLine[i] - signalLine[i]
	}

	// Calculate Bollinger Bands
	bbMiddle, bbUpper, bbLower := indicator.BollingerBands(closes)

	// Calculate volume average
	volumeAvg := calculateAverage(volumes)
	currentVolume := volumes[len(volumes)-1]
	volumeRatio := currentVolume / volumeAvg

	indicators := &models.TechnicalIndicators{
		RSI: map[string]decimal.Decimal{
			"14": models.NewDecimal(rsi14[len(rsi14)-1]),
		},
		MACD: &models.MACDIndicator{
			MACD:      models.NewDecimal(macdLine[len(macdLine)-1]),
			Signal:    models.NewDecimal(signalLine[len(signalLine)-1]),
			Histogram: models.NewDecimal(histogram[len(histogram)-1]),
		},
		BollingerBands: &models.BollingerBandsIndicator{
			Upper:  models.NewDecimal(bbUpper[len(bbUpper)-1]),
			Middle: models.NewDecimal(bbMiddle[len(bbMiddle)-1]),
			Lower:  models.NewDecimal(bbLower[len(bbLower)-1]),
		},
		Volume: &models.VolumeIndicator{
			Current: models.NewDecimal(currentVolume),
			Average: models.NewDecimal(volumeAvg),
			Ratio:   models.NewDecimal(volumeRatio),
		},
	}

	return indicators, nil
}

// CalculateMultipleTimeframes calculates indicators for multiple timeframes
func (c *Calculator) CalculateMultipleTimeframes(candlesMap map[string][]models.Candle) (map[string]*models.TechnicalIndicators, error) {
	result := make(map[string]*models.TechnicalIndicators)

	for timeframe, candles := range candlesMap {
		indicators, err := c.Calculate(candles)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate indicators for %s: %w", timeframe, err)
		}
		result[timeframe] = indicators
	}

	return result, nil
}

// CalculateRSI calculates RSI for specific period
func (c *Calculator) CalculateRSI(candles []models.Candle, period int) (float64, error) {
	if len(candles) < period+1 {
		return 0, fmt.Errorf("insufficient candles for RSI calculation")
	}

	closes := make([]float64, len(candles))
	for i, candle := range candles {
		closes[i], _ = candle.Close.Float64()
	}

	_, rsi := indicator.Rsi(closes)
	if len(rsi) == 0 {
		return 0, fmt.Errorf("RSI returned no data")
	}
	return rsi[len(rsi)-1], nil
}

// CalculateEMA calculates Exponential Moving Average
func (c *Calculator) CalculateEMA(candles []models.Candle, period int) (float64, error) {
	if len(candles) < period {
		return 0, fmt.Errorf("insufficient candles for EMA calculation")
	}

	closes := make([]float64, len(candles))
	for i, candle := range candles {
		closes[i], _ = candle.Close.Float64()
	}

	ema := indicator.Ema(period, closes)
	if len(ema) == 0 {
		return 0, fmt.Errorf("EMA calculation failed")
	}
	return ema[len(ema)-1], nil
}

// CalculateSMA calculates Simple Moving Average
func (c *Calculator) CalculateSMA(candles []models.Candle, period int) (float64, error) {
	if len(candles) < period {
		return 0, fmt.Errorf("insufficient candles for SMA calculation")
	}

	closes := make([]float64, len(candles))
	for i, candle := range candles {
		closes[i], _ = candle.Close.Float64()
	}

	sma := indicator.Sma(period, closes)
	if len(sma) == 0 {
		return 0, fmt.Errorf("SMA calculation failed")
	}
	return sma[len(sma)-1], nil
}

// DetectTrend detects market trend based on moving averages
func (c *Calculator) DetectTrend(candles []models.Candle) (string, error) {
	if len(candles) < 50 {
		return "unknown", fmt.Errorf("insufficient data for trend detection")
	}

	ema20, err := c.CalculateEMA(candles, 20)
	if err != nil {
		return "unknown", err
	}

	ema50, err := c.CalculateEMA(candles, 50)
	if err != nil {
		return "unknown", err
	}

	currentPrice, _ := candles[len(candles)-1].Close.Float64()

	if currentPrice > ema20 && ema20 > ema50 {
		return "uptrend", nil
	} else if currentPrice < ema20 && ema20 < ema50 {
		return "downtrend", nil
	}

	return "sideways", nil
}

// CalculateVolatility calculates price volatility (ATR - Average True Range)
func (c *Calculator) CalculateVolatility(candles []models.Candle, period int) (float64, error) {
	if len(candles) < period+1 {
		return 0, fmt.Errorf("insufficient candles for volatility calculation")
	}

	highs := make([]float64, len(candles))
	lows := make([]float64, len(candles))
	closes := make([]float64, len(candles))

	for i, candle := range candles {
		highs[i], _ = candle.High.Float64()
		lows[i], _ = candle.Low.Float64()
		closes[i], _ = candle.Close.Float64()
	}

	_, atr := indicator.Atr(period, highs, lows, closes)
	if len(atr) == 0 {
		return 0, fmt.Errorf("ATR returned no data")
	}
	return atr[len(atr)-1], nil
}

// IsSupportLevel checks if price is near support level
func (c *Calculator) IsSupportLevel(candles []models.Candle, currentPrice float64, threshold float64) bool {
	if len(candles) < 20 {
		return false
	}

	// Find recent lows
	for i := len(candles) - 20; i < len(candles); i++ {
		low, _ := candles[i].Low.Float64()
		diff := abs(currentPrice-low) / currentPrice

		if diff < threshold {
			return true
		}
	}

	return false
}

// IsResistanceLevel checks if price is near resistance level
func (c *Calculator) IsResistanceLevel(candles []models.Candle, currentPrice float64, threshold float64) bool {
	if len(candles) < 20 {
		return false
	}

	// Find recent highs
	for i := len(candles) - 20; i < len(candles); i++ {
		high, _ := candles[i].High.Float64()
		diff := abs(currentPrice-high) / currentPrice

		if diff < threshold {
			return true
		}
	}

	return false
}

// Helper functions
func calculateAverage(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	sum := 0.0
	for _, v := range values {
		sum += v
	}

	return sum / float64(len(values))
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
