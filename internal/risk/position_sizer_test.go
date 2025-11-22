package risk

import (
	"testing"

	"github.com/selivandex/trader-bot/pkg/models"
)

func TestPositionSizer_CalculatePositionSize(t *testing.T) {
	params := &models.StrategyParameters{
		MaxPositionPercent:     30.0,
		MaxLeverage:            3,
		StopLossPercent:        2.0,
		TakeProfitPercent:      5.0,
		MinConfidenceThreshold: 70,
	}

	ps := NewPositionSizer(params)

	t.Run("long position", func(t *testing.T) {
		balance := 1000.0
		price := 40000.0

		size, err := ps.CalculatePositionSize(balance, price, models.PositionLong)
		if err != nil {
			t.Fatalf("Failed to calculate position size: %v", err)
		}

		// 30% of $1000 = $300 margin
		// With 3x leverage = $900 position value
		// At $40k/BTC = 0.0225 BTC
		expectedSize := 0.0225

		if abs(size.Size-expectedSize) > 0.001 {
			t.Errorf("Expected size ~%.4f, got %.4f", expectedSize, size.Size)
		}

		if size.RequiredMargin != 300 {
			t.Errorf("Expected margin $300, got $%.2f", size.RequiredMargin)
		}

		// Stop loss should be 2% below entry
		expectedSL := price * 0.98
		if abs(size.StopLoss-expectedSL) > 1.0 {
			t.Errorf("Expected stop loss ~%.2f, got %.2f", expectedSL, size.StopLoss)
		}
	})

	t.Run("short position", func(t *testing.T) {
		balance := 1000.0
		price := 40000.0

		size, err := ps.CalculatePositionSize(balance, price, models.PositionShort)
		if err != nil {
			t.Fatalf("Failed to calculate position size: %v", err)
		}

		// Stop loss should be 2% above entry for short
		expectedSL := price * 1.02
		if abs(size.StopLoss-expectedSL) > 1.0 {
			t.Errorf("Expected stop loss ~%.2f, got %.2f", expectedSL, size.StopLoss)
		}
	})
}

func TestPositionSizer_ValidatePositionSize(t *testing.T) {
	params := &models.StrategyParameters{
		MaxPositionPercent:     30.0,
		MaxLeverage:            3,
		StopLossPercent:        2.0,
		TakeProfitPercent:      5.0,
		MinConfidenceThreshold: 70,
	}

	ps := NewPositionSizer(params)

	t.Run("valid position", func(t *testing.T) {
		size := &PositionSize{
			Size:           0.025,
			Value:          1000,
			RequiredMargin: 300,
			Leverage:       3,
		}

		err := ps.ValidatePositionSize(size, 1000)
		if err != nil {
			t.Errorf("Valid position rejected: %v", err)
		}
	})

	t.Run("insufficient balance", func(t *testing.T) {
		size := &PositionSize{
			Size:           0.025,
			Value:          1000,
			RequiredMargin: 600,
			Leverage:       3,
		}

		err := ps.ValidatePositionSize(size, 500)
		if err == nil {
			t.Error("Should reject position with insufficient balance")
		}
	})

	t.Run("leverage too high", func(t *testing.T) {
		size := &PositionSize{
			Size:           0.025,
			Value:          1000,
			RequiredMargin: 100,
			Leverage:       10,
		}

		err := ps.ValidatePositionSize(size, 1000)
		if err == nil {
			t.Error("Should reject position with excessive leverage")
		}
	})
}
