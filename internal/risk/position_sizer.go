package risk

import (
	"fmt"
	"math"

	"github.com/alexanderselivanov/trader/internal/adapters/config"
	"github.com/alexanderselivanov/trader/pkg/models"
)

// PositionSizer calculates optimal position sizes
type PositionSizer struct {
	maxPositionPercent float64
	maxLeverage        int
	stopLossPercent    float64
}

// NewPositionSizer creates new position sizer
func NewPositionSizer(cfg *config.TradingConfig) *PositionSizer {
	return &PositionSizer{
		maxPositionPercent: cfg.MaxPositionPercent,
		maxLeverage:        cfg.MaxLeverage,
		stopLossPercent:    cfg.StopLossPercent,
	}
}

// CalculatePositionSize calculates position size based on available balance and risk parameters
func (ps *PositionSizer) CalculatePositionSize(balance, price float64, side models.PositionSide) (*PositionSize, error) {
	if balance <= 0 {
		return nil, fmt.Errorf("invalid balance: %.2f", balance)
	}

	if price <= 0 {
		return nil, fmt.Errorf("invalid price: %.2f", price)
	}

	// Calculate maximum position value (% of balance)
	maxPositionValue := balance * (ps.maxPositionPercent / 100.0)

	// Calculate position size in base currency (BTC, ETH, etc)
	// With leverage, we can control more with less margin
	maxPositionValueWithLeverage := maxPositionValue * float64(ps.maxLeverage)
	positionSize := maxPositionValueWithLeverage / price

	// Calculate required margin
	requiredMargin := maxPositionValue

	// Calculate stop loss and take profit prices
	var stopLoss, takeProfit float64

	if side == models.PositionLong {
		stopLoss = price * (1 - ps.stopLossPercent/100.0)
		takeProfit = price * (1 + ps.stopLossPercent*2.5/100.0) // 2.5x risk:reward
	} else {
		stopLoss = price * (1 + ps.stopLossPercent/100.0)
		takeProfit = price * (1 - ps.stopLossPercent*2.5/100.0)
	}

	// Calculate potential loss if stop loss hits
	potentialLoss := math.Abs(price-stopLoss) * positionSize

	// Calculate liquidation price (approximate)
	liquidationPrice := ps.calculateLiquidationPrice(price, side, ps.maxLeverage)

	return &PositionSize{
		Size:             positionSize,
		Value:            positionSize * price,
		RequiredMargin:   requiredMargin,
		Leverage:         ps.maxLeverage,
		StopLoss:         stopLoss,
		TakeProfit:       takeProfit,
		PotentialLoss:    potentialLoss,
		RiskRewardRatio:  2.5,
		LiquidationPrice: liquidationPrice,
	}, nil
}

// calculateLiquidationPrice calculates approximate liquidation price
func (ps *PositionSizer) calculateLiquidationPrice(entryPrice float64, side models.PositionSide, leverage int) float64 {
	// Simplified liquidation calculation (Binance-style)
	// Liquidation occurs when loss equals margin (100% / leverage)
	maintenanceMarginRate := 0.004 // 0.4% for low leverage

	liquidationPercent := (1.0 / float64(leverage)) - maintenanceMarginRate

	if side == models.PositionLong {
		return entryPrice * (1 - liquidationPercent)
	}

	return entryPrice * (1 + liquidationPercent)
}

// AdjustForExistingPosition adjusts position size if there's already an open position
func (ps *PositionSizer) AdjustForExistingPosition(newSize *PositionSize, existingPosition *models.Position, balance float64) (*PositionSize, error) {
	if existingPosition == nil {
		return newSize, nil
	}

	// Calculate total position value after adding
	existingValue := existingPosition.Size.Float64() * existingPosition.CurrentPrice.Float64()
	totalValue := existingValue + newSize.Value

	maxAllowedValue := balance * (ps.maxPositionPercent / 100.0) * float64(ps.maxLeverage)

	if totalValue > maxAllowedValue {
		// Scale down new position
		availableValue := maxAllowedValue - existingValue
		if availableValue <= 0 {
			return nil, fmt.Errorf("cannot add to position: max position size reached")
		}

		scaleFactor := availableValue / newSize.Value
		newSize.Size *= scaleFactor
		newSize.Value *= scaleFactor
		newSize.RequiredMargin *= scaleFactor
	}

	return newSize, nil
}

// ValidatePositionSize checks if position size is valid
func (ps *PositionSizer) ValidatePositionSize(size *PositionSize, balance float64) error {
	if size.RequiredMargin > balance {
		return fmt.Errorf("insufficient balance: required %.2f, available %.2f", size.RequiredMargin, balance)
	}

	if size.Size <= 0 {
		return fmt.Errorf("invalid position size: %.8f", size.Size)
	}

	if size.Leverage > ps.maxLeverage {
		return fmt.Errorf("leverage too high: %d (max %d)", size.Leverage, ps.maxLeverage)
	}

	// Check minimum notional value (most exchanges have minimum order sizes)
	minNotional := 10.0 // $10 minimum
	if size.Value < minNotional {
		return fmt.Errorf("position value too small: %.2f (min %.2f)", size.Value, minNotional)
	}

	return nil
}

// PositionSize represents calculated position parameters
type PositionSize struct {
	Size             float64 `json:"size"`              // Amount in base currency
	Value            float64 `json:"value"`             // Total position value in quote currency
	RequiredMargin   float64 `json:"required_margin"`   // Required margin
	Leverage         int     `json:"leverage"`          // Leverage used
	StopLoss         float64 `json:"stop_loss"`         // Stop loss price
	TakeProfit       float64 `json:"take_profit"`       // Take profit price
	PotentialLoss    float64 `json:"potential_loss"`    // Loss if stop loss hits
	RiskRewardRatio  float64 `json:"risk_reward_ratio"` // Risk/reward ratio
	LiquidationPrice float64 `json:"liquidation_price"` // Approximate liquidation price
}
