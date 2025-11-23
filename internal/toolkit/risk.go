package toolkit

import (
	"context"
	"fmt"
	"math"

	"go.uber.org/zap"

	"github.com/selivandex/trader-bot/pkg/logger"
	"github.com/selivandex/trader-bot/pkg/models"
)

// ============ Risk Calculation Tools Implementation ============

// CalculatePositionRisk calculates risk metrics for proposed position
func (t *LocalToolkit) CalculatePositionRisk(ctx context.Context, symbol string, side models.PositionSide, size, leverage float64, stopLoss float64) (*PositionRiskMetrics, error) {
	logger.Debug("toolkit: calculate_position_risk",
		zap.String("agent_id", t.agentID),
		zap.String("symbol", symbol),
		zap.String("side", string(side)),
		zap.Float64("size", size),
		zap.Float64("leverage", leverage),
		zap.Float64("stop_loss", stopLoss),
	)

	// Get current price
	currentPrice, err := t.GetLatestPrice(ctx, symbol, "1m")
	if err != nil {
		return nil, fmt.Errorf("failed to get current price: %w", err)
	}

	// Calculate position value
	positionValue := size * currentPrice * leverage

	// Calculate max loss if stop loss hits
	var maxLoss float64
	if side == models.PositionLong {
		maxLoss = (currentPrice - stopLoss) * size * leverage
	} else {
		maxLoss = (stopLoss - currentPrice) * size * leverage
	}

	// Calculate liquidation price (simplified)
	maintenanceMarginRate := 0.004 // 0.4%
	liquidationPercent := (1.0 / leverage) - maintenanceMarginRate
	var liquidationPrice float64
	if side == models.PositionLong {
		liquidationPrice = currentPrice * (1 - liquidationPercent)
	} else {
		liquidationPrice = currentPrice * (1 + liquidationPercent)
	}

	// Get agent state for risk percent calculation
	state, err := t.agentRepo.GetAgentState(ctx, t.agentID, symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent state: %w", err)
	}

	balance := state.Balance.InexactFloat64()
	riskPercent := (maxLoss / balance) * 100

	// Calculate risk/reward ratio
	var takeProfit float64
	if side == models.PositionLong {
		takeProfit = currentPrice * 1.05 // Assume 5% target
	} else {
		takeProfit = currentPrice * 0.95
	}
	potentialProfit := math.Abs(takeProfit-currentPrice) * size * leverage
	riskRewardRatio := potentialProfit / maxLoss

	// Estimate probability from historical data
	memory, _ := t.agentRepo.GetAgentStatisticalMemory(ctx, t.agentID)
	var probabilityProfit float64 = 0.5 // Default
	if memory != nil {
		// Use average success rate as probability
		probabilityProfit = (memory.TechnicalSuccessRate + memory.NewsSuccessRate + 
			memory.OnChainSuccessRate + memory.SentimentSuccessRate) / 4.0
	}

	// Calculate risk score (1-10)
	riskScore := calculateRiskScore(riskPercent, leverage, riskRewardRatio)

	return &PositionRiskMetrics{
		MaxLoss:           maxLoss,
		LiquidationPrice:  liquidationPrice,
		RiskPercent:       riskPercent,
		RiskRewardRatio:   riskRewardRatio,
		RequiredMargin:    positionValue / leverage,
		ProbabilityProfit: probabilityProfit,
		RiskScore:         riskScore,
	}, nil
}

// SimulateWorstCase simulates worst case scenario
func (t *LocalToolkit) SimulateWorstCase(ctx context.Context, symbol string, size, leverage float64) (*WorstCaseScenario, error) {
	logger.Debug("toolkit: simulate_worst_case",
		zap.String("agent_id", t.agentID),
		zap.String("symbol", symbol),
		zap.Float64("size", size),
		zap.Float64("leverage", leverage),
	)

	// Get current volatility
	volatility, err := t.CalculateVolatility(ctx, symbol, "1h", 14)
	if err != nil {
		volatility = 500 // Default conservative estimate
	}

	// Get current price
	currentPrice, err := t.GetLatestPrice(ctx, symbol, "1m")
	if err != nil {
		return nil, fmt.Errorf("failed to get current price: %w", err)
	}

	// Get agent balance
	state, err := t.agentRepo.GetAgentState(ctx, t.agentID, symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent state: %w", err)
	}
	balance := state.Balance.InexactFloat64()

	// Calculate worst case: price moves against position by 3x volatility (3 sigma event)
	maxAdverseMove := volatility * 3
	maxLossPercent := (maxAdverseMove / currentPrice) * 100 * leverage
	maxLossUSD := balance * (maxLossPercent / 100)

	// Liquidation risk assessment
	liquidationDistance := 100 / leverage // % move to liquidation
	liquidationRisk := "low"
	if liquidationDistance < maxAdverseMove/currentPrice*100 {
		liquidationRisk = "high"
	} else if liquidationDistance < maxAdverseMove/currentPrice*200 {
		liquidationRisk = "medium"
	}

	// Time to liquidation estimate (hours at current volatility)
	hourlyVolatility := volatility / math.Sqrt(24) // Convert daily to hourly
	timeToLiquidation := liquidationDistance / (hourlyVolatility / currentPrice * 100)

	// Recovery calculation: how many winning trades needed
	avgWin := balance * 0.03 // Assume 3% avg win
	tradesNeeded := maxLossUSD / avgWin
	recoveryStr := fmt.Sprintf("%.0f winning trades", math.Ceil(tradesNeeded))

	return &WorstCaseScenario{
		MaxLossUSD:        maxLossUSD,
		MaxLossPercent:    maxLossPercent,
		LiquidationRisk:   liquidationRisk,
		TimeToLiquidation: timeToLiquidation,
		Recovery:          recoveryStr,
	}, nil
}

// CheckDrawdownRisk checks if position would exceed max drawdown
func (t *LocalToolkit) CheckDrawdownRisk(ctx context.Context, agentID, symbol string, proposedLoss float64) (bool, error) {
	logger.Debug("toolkit: check_drawdown_risk",
		zap.String("agent_id", agentID),
		zap.String("symbol", symbol),
		zap.Float64("proposed_loss", proposedLoss),
	)

	// Get peak equity
	peakEquity, err := t.agentRepo.GetPeakEquity(ctx, agentID, symbol)
	if err != nil {
		return false, fmt.Errorf("failed to get peak equity: %w", err)
	}

	// Get current equity
	state, err := t.agentRepo.GetAgentState(ctx, agentID, symbol)
	if err != nil {
		return false, fmt.Errorf("failed to get agent state: %w", err)
	}
	currentEquity := state.Equity.InexactFloat64()

	// Calculate potential equity after loss
	potentialEquity := currentEquity - proposedLoss

	// Calculate drawdown
	drawdown := (peakEquity - potentialEquity) / peakEquity * 100

	// Typical max drawdown: 20%
	maxDrawdown := 20.0

	return drawdown >= maxDrawdown, nil
}

// CalculateOptimalSize calculates optimal position size based on Kelly criterion
func (t *LocalToolkit) CalculateOptimalSize(ctx context.Context, agentID, symbol string, winRate, avgWin, avgLoss float64) (float64, error) {
	logger.Debug("toolkit: calculate_optimal_size",
		zap.String("agent_id", agentID),
		zap.String("symbol", symbol),
		zap.Float64("win_rate", winRate),
	)

	// Kelly criterion: f* = (p * b - q) / b
	// where p = win rate, q = loss rate, b = avg win / avg loss
	p := winRate
	q := 1 - winRate
	b := avgWin / avgLoss

	kellyPercent := (p*b - q) / b

	// Use fractional Kelly (safer)
	fractionalKelly := kellyPercent * 0.5 // Half Kelly

	// Cap at reasonable max (25% of balance)
	if fractionalKelly > 0.25 {
		fractionalKelly = 0.25
	}

	// Ensure non-negative
	if fractionalKelly < 0 {
		fractionalKelly = 0.05 // Minimum position
	}

	return fractionalKelly, nil
}

// calculateRiskScore calculates risk score from 1-10
func calculateRiskScore(riskPercent, leverage, rrRatio float64) int {
	score := 5.0 // Baseline

	// Higher risk % = higher score
	if riskPercent > 5 {
		score += 2
	} else if riskPercent > 3 {
		score += 1
	} else if riskPercent < 1 {
		score -= 1
	}

	// Higher leverage = higher score
	if leverage >= 5 {
		score += 2
	} else if leverage >= 3 {
		score += 1
	}

	// Lower R:R = higher score
	if rrRatio < 1.5 {
		score += 2
	} else if rrRatio > 3 {
		score -= 1
	}

	// Clamp to 1-10
	if score < 1 {
		score = 1
	}
	if score > 10 {
		score = 10
	}

	return int(score)
}

