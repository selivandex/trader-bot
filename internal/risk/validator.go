package risk

import (
	"fmt"

	"github.com/alexanderselivanov/trader/pkg/models"
)

// Validator validates trading decisions against risk rules
type Validator struct {
	minConfidence int
	maxSlippage   float64
}

// NewValidator creates new decision validator
func NewValidator() *Validator {
	return &Validator{
		minConfidence: 70,  // Minimum 70% confidence to trade
		maxSlippage:   0.5, // Maximum 0.5% slippage
	}
}

// ValidateDecision validates AI decision before execution
func (v *Validator) ValidateDecision(decision *models.AIDecision, marketData *models.MarketData) error {
	// Check confidence level
	if decision.Confidence < v.minConfidence {
		return fmt.Errorf("confidence too low: %d%% (min %d%%)", decision.Confidence, v.minConfidence)
	}

	// Validate action-specific parameters
	switch decision.Action {
	case models.ActionOpenLong, models.ActionOpenShort:
		if err := v.validateOpenAction(decision, marketData); err != nil {
			return err
		}
	case models.ActionClose:
		// Close action always valid
	case models.ActionHold:
		// Hold action always valid
	case models.ActionScaleIn, models.ActionScaleOut:
		if decision.Size.Float64() <= 0 {
			return fmt.Errorf("invalid size for scale action: %.8f", decision.Size.Float64())
		}
	default:
		return fmt.Errorf("unknown action: %s", decision.Action)
	}

	return nil
}

// validateOpenAction validates parameters for opening new position
func (v *Validator) validateOpenAction(decision *models.AIDecision, marketData *models.MarketData) error {
	if decision.Size.Float64() <= 0 {
		return fmt.Errorf("invalid position size: %.8f", decision.Size.Float64())
	}

	if decision.StopLoss.Float64() <= 0 {
		return fmt.Errorf("stop loss not set")
	}

	if decision.TakeProfit.Float64() <= 0 {
		return fmt.Errorf("take profit not set")
	}

	currentPrice := marketData.Ticker.Last.Float64()

	// Validate stop loss and take profit placement
	if decision.Action == models.ActionOpenLong {
		if decision.StopLoss.Float64() >= currentPrice {
			return fmt.Errorf("invalid stop loss for long: %.2f (current price: %.2f)", decision.StopLoss.Float64(), currentPrice)
		}
		if decision.TakeProfit.Float64() <= currentPrice {
			return fmt.Errorf("invalid take profit for long: %.2f (current price: %.2f)", decision.TakeProfit.Float64(), currentPrice)
		}
	} else if decision.Action == models.ActionOpenShort {
		if decision.StopLoss.Float64() <= currentPrice {
			return fmt.Errorf("invalid stop loss for short: %.2f (current price: %.2f)", decision.StopLoss.Float64(), currentPrice)
		}
		if decision.TakeProfit.Float64() >= currentPrice {
			return fmt.Errorf("invalid take profit for short: %.2f (current price: %.2f)", decision.TakeProfit.Float64(), currentPrice)
		}
	}

	// Check if stop loss and take profit are too close (unrealistic)
	slPercent := abs(currentPrice-decision.StopLoss.Float64()) / currentPrice * 100
	tpPercent := abs(currentPrice-decision.TakeProfit.Float64()) / currentPrice * 100

	if slPercent < 0.5 {
		return fmt.Errorf("stop loss too close: %.2f%%", slPercent)
	}

	if tpPercent < 1.0 {
		return fmt.Errorf("take profit too close: %.2f%%", tpPercent)
	}

	// Check risk/reward ratio
	riskReward := tpPercent / slPercent
	if riskReward < 1.5 {
		return fmt.Errorf("poor risk/reward ratio: %.2f (min 1.5)", riskReward)
	}

	return nil
}

// ValidateMarketConditions checks if market conditions are suitable for trading
func (v *Validator) ValidateMarketConditions(marketData *models.MarketData) error {
	// Check bid-ask spread (high spread = low liquidity)
	ticker := marketData.Ticker
	spread := (ticker.Ask.Float64() - ticker.Bid.Float64()) / ticker.Last.Float64() * 100

	if spread > v.maxSlippage {
		return fmt.Errorf("spread too wide: %.3f%% (max %.3f%%)", spread, v.maxSlippage)
	}

	// Check if there's orderbook data
	if marketData.OrderBook == nil || len(marketData.OrderBook.Bids) == 0 || len(marketData.OrderBook.Asks) == 0 {
		return fmt.Errorf("insufficient order book data")
	}

	// Check extreme volatility (using Bollinger Bands width)
	if marketData.Indicators != nil && marketData.Indicators.BollingerBands != nil {
		bb := marketData.Indicators.BollingerBands
		bbWidth := (bb.Upper.Float64() - bb.Lower.Float64()) / bb.Middle.Float64() * 100

		if bbWidth > 10.0 {
			return fmt.Errorf("extreme volatility detected: BB width %.2f%%", bbWidth)
		}
	}

	return nil
}

// ValidateEnsembleDecision validates consensus from multiple AI providers
func (v *Validator) ValidateEnsembleDecision(ensemble *models.EnsembleDecision) error {
	if !ensemble.Agreement {
		return fmt.Errorf("no consensus among AI providers")
	}

	if ensemble.Consensus == nil {
		return fmt.Errorf("consensus decision is nil")
	}

	if ensemble.Confidence < v.minConfidence {
		return fmt.Errorf("ensemble confidence too low: %d%%", ensemble.Confidence)
	}

	return nil
}

// CheckDrawdown checks if current drawdown exceeds maximum allowed
func (v *Validator) CheckDrawdown(currentEquity, peakEquity, maxDrawdownPercent float64) error {
	if peakEquity <= 0 {
		return nil
	}

	drawdown := (peakEquity - currentEquity) / peakEquity * 100

	if drawdown >= maxDrawdownPercent {
		return fmt.Errorf("max drawdown exceeded: %.2f%% (max %.2f%%)", drawdown, maxDrawdownPercent)
	}

	return nil
}

// SanityCheck performs basic sanity checks on decision
func (v *Validator) SanityCheck(decision *models.AIDecision, currentPrice float64) error {
	// Check if prices are in reasonable range
	if decision.StopLoss.Float64() > 0 {
		slDiff := abs(currentPrice-decision.StopLoss.Float64()) / currentPrice * 100
		if slDiff > 50 {
			return fmt.Errorf("stop loss too far from current price: %.2f%%", slDiff)
		}
	}

	if decision.TakeProfit.Float64() > 0 {
		tpDiff := abs(currentPrice-decision.TakeProfit.Float64()) / currentPrice * 100
		if tpDiff > 50 {
			return fmt.Errorf("take profit too far from current price: %.2f%%", tpDiff)
		}
	}

	// Check if size is reasonable
	if decision.Size.Float64() > 100 {
		return fmt.Errorf("position size seems unrealistic: %.2f", decision.Size.Float64())
	}

	return nil
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
