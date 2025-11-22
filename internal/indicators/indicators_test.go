package indicators

import (
	"testing"
	"time"

	"github.com/alexanderselivanov/trader/pkg/models"
)

func TestCalculator_Calculate(t *testing.T) {
	calc := NewCalculator()

	// Generate sample candles (trending up)
	candles := generateTestCandles(50, 40000, 0.01)

	indicators, err := calc.Calculate(candles)
	if err != nil {
		t.Fatalf("Failed to calculate indicators: %v", err)
	}

	// Check RSI exists
	if indicators.RSI == nil {
		t.Error("RSI should be calculated")
	}

	rsi14, ok := indicators.RSI["14"]
	if !ok {
		t.Error("RSI 14 should exist")
	}

	rsiValue, _ := rsi14.Float64()
	if rsiValue < 0 || rsiValue > 100 {
		t.Errorf("RSI should be between 0-100, got %.2f", rsiValue)
	}

	// Check MACD
	if indicators.MACD == nil {
		t.Error("MACD should be calculated")
	}

	// Check Bollinger Bands
	if indicators.BollingerBands == nil {
		t.Error("Bollinger Bands should be calculated")
	}

	bb := indicators.BollingerBands
	bbUpper, _ := bb.Upper.Float64()
	bbMiddle, _ := bb.Middle.Float64()
	if bbUpper <= bbMiddle {
		t.Error("Upper band should be above middle")
	}

	bbLower, _ := bb.Lower.Float64()
	if bbMiddle <= bbLower {
		t.Error("Middle band should be above lower")
	}

	// Check Volume
	if indicators.Volume == nil {
		t.Error("Volume indicators should be calculated")
	}
}

func TestCalculator_InsufficientData(t *testing.T) {
	calc := NewCalculator()

	// Only 10 candles - not enough
	candles := generateTestCandles(10, 40000, 0.01)

	_, err := calc.Calculate(candles)
	if err == nil {
		t.Error("Should error with insufficient data")
	}
}

func TestCalculator_CalculateRSI(t *testing.T) {
	calc := NewCalculator()

	candles := generateTestCandles(30, 40000, 0.01)

	rsi, err := calc.CalculateRSI(candles, 14)
	if err != nil {
		t.Fatalf("Failed to calculate RSI: %v", err)
	}

	if rsi < 0 || rsi > 100 {
		t.Errorf("RSI should be between 0-100, got %.2f", rsi)
	}
}

func TestCalculator_DetectTrend(t *testing.T) {
	calc := NewCalculator()

	t.Run("uptrend", func(t *testing.T) {
		// Strong uptrend
		candles := generateTestCandles(60, 40000, 0.02)

		trend, err := calc.DetectTrend(candles)
		if err != nil {
			t.Fatalf("Failed to detect trend: %v", err)
		}

		if trend != "uptrend" {
			t.Errorf("Expected uptrend, got %s", trend)
		}
	})

	t.Run("downtrend", func(t *testing.T) {
		// Strong downtrend
		candles := generateTestCandles(60, 40000, -0.02)

		trend, err := calc.DetectTrend(candles)
		if err != nil {
			t.Fatalf("Failed to detect trend: %v", err)
		}

		if trend != "downtrend" {
			t.Errorf("Expected downtrend, got %s", trend)
		}
	})
}

// Helper function to generate test candles
func generateTestCandles(count int, startPrice, trend float64) []models.Candle {
	candles := make([]models.Candle, count)
	price := startPrice

	for i := 0; i < count; i++ {
		open := price
		close := price * (1 + trend)
		high := max(open, close) * 1.002
		low := min(open, close) * 0.998

		candles[i] = models.Candle{
			Timestamp: time.Now().Add(-time.Duration(count-i) * time.Hour),
			Open:      models.NewDecimal(open),
			High:      models.NewDecimal(high),
			Low:       models.NewDecimal(low),
			Close:     models.NewDecimal(close),
			Volume:    models.NewDecimal(100 + float64(i)*2),
		}

		price = close
	}

	return candles
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
