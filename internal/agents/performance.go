package agents

import (
	"context"
	"math"
)

// PerformanceCalculator calculates advanced performance metrics
type PerformanceCalculator struct {
	repository *Repository
}

// NewPerformanceCalculator creates new performance calculator
func NewPerformanceCalculator(repository *Repository) *PerformanceCalculator {
	return &PerformanceCalculator{repository: repository}
}

// CalculateSharpeRatio calculates Sharpe ratio for agent
// Sharpe = (average return - risk free rate) / standard deviation of returns
func (pc *PerformanceCalculator) CalculateSharpeRatio(ctx context.Context, agentID, symbol string) (float64, error) {
	// Get performance metrics via repository
	metrics, err := pc.repository.GetAgentPerformanceMetrics(ctx, agentID, symbol)
	if err != nil {
		return 0, err
	}

	// Sharpe ratio already calculated in metrics
	if metrics.SharpeRatio > 0 {
		return metrics.SharpeRatio, nil
	}

	// Calculate from individual returns if needed
	returns, err := pc.repository.GetTradeReturns(ctx, agentID, symbol, 100)
	if err != nil {
		return 0, err
	}

	if len(returns) < 2 {
		return 0, nil // Not enough data
	}

	// Calculate average return
	sum := 0.0
	for _, r := range returns {
		sum += r
	}
	avgReturn := sum / float64(len(returns))

	// Calculate standard deviation
	variance := 0.0
	for _, r := range returns {
		diff := r - avgReturn
		variance += diff * diff
	}
	variance /= float64(len(returns))
	stdDev := math.Sqrt(variance)

	if stdDev == 0 {
		return 0, nil
	}

	// Risk-free rate = 0 for crypto (no safe baseline)
	riskFreeRate := 0.0

	// Sharpe ratio
	sharpe := (avgReturn - riskFreeRate) / stdDev

	// Annualize (assume 365 days trading)
	annualizedSharpe := sharpe * math.Sqrt(365)

	return annualizedSharpe, nil
}

// CalculateMaxDrawdown calculates maximum drawdown
func (pc *PerformanceCalculator) CalculateMaxDrawdown(ctx context.Context, agentID, symbol string) (float64, error) {
	state, err := pc.repository.GetAgentState(ctx, agentID, symbol)
	if err != nil {
		return 0, err
	}

	currentBalance, _ := state.Balance.Float64()
	peakEquity, err := pc.repository.GetPeakEquity(ctx, agentID, symbol)
	if err != nil {
		return 0, err
	}
	if peakEquity == 0 {
		return 0, nil
	}

	drawdown := ((peakEquity - currentBalance) / peakEquity) * 100

	if drawdown < 0 {
		return 0, nil // No drawdown
	}

	return drawdown, nil
}

// CalculateProfitFactor calculates profit factor (gross profit / gross loss)
func (pc *PerformanceCalculator) CalculateProfitFactor(ctx context.Context, agentID, symbol string) (float64, error) {
	grossProfit, grossLoss, err := pc.repository.GetProfitLoss(ctx, agentID, symbol)
	if err != nil {
		return 0, err
	}

	if grossLoss == 0 {
		if grossProfit > 0 {
			return grossProfit, nil // Only wins
		}
		return 0, nil
	}

	return grossProfit / grossLoss, nil
}
