package toolkit

import (
	"context"
	"fmt"

	"github.com/selivandex/trader-bot/internal/adapters/correlation"
	"github.com/selivandex/trader-bot/pkg/models"
)

// CorrelationTool provides correlation analysis for trading agents
type CorrelationTool struct {
	corrRepo *correlation.Repository
}

// NewCorrelationTool creates a new correlation toolkit
func NewCorrelationTool(corrRepo *correlation.Repository) *CorrelationTool {
	return &CorrelationTool{
		corrRepo: corrRepo,
	}
}

// GetBTCCorrelation returns correlation coefficient for symbol vs BTC
func (t *CorrelationTool) GetBTCCorrelation(ctx context.Context, symbol string, period string) (*models.CorrelationResult, error) {
	if symbol == "BTC/USDT" {
		return &models.CorrelationResult{
			Symbol:         symbol,
			BTCCorrelation: 1.0, // BTC perfectly correlated with itself
			Period:         period,
		}, nil
	}

	corr, err := t.corrRepo.GetLatestCorrelation(ctx, symbol, "BTC/USDT", period)
	if err != nil {
		return nil, fmt.Errorf("failed to get BTC correlation: %w", err)
	}

	return &models.CorrelationResult{
		Symbol:         symbol,
		BTCCorrelation: corr.Correlation,
		Period:         period,
		UpdatedAt:      corr.CalculatedAt,
	}, nil
}

// GetGlobalMarketRegime returns current global market regime (risk-on/risk-off)
func (t *CorrelationTool) GetGlobalMarketRegime(ctx context.Context) (*models.MarketRegimeResult, error) {
	regime, err := t.corrRepo.GetLatestMarketRegime(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get market regime: %w", err)
	}

	return &models.MarketRegimeResult{
		Regime:          regime.Regime,
		BTCDominance:    regime.BTCDominance,
		AvgCorrelation:  regime.AvgCorrelation,
		VolatilityLevel: regime.VolatilityLevel,
		Confidence:      regime.Confidence,
		DetectedAt:      regime.DetectedAt,
	}, nil
}

// GetCorrelationStrength returns human-readable correlation strength
func (t *CorrelationTool) GetCorrelationStrength(correlation float64) string {
	absCorr := correlation
	if absCorr < 0 {
		absCorr = -absCorr
	}

	switch {
	case absCorr >= 0.9:
		return "very_strong"
	case absCorr >= 0.7:
		return "strong"
	case absCorr >= 0.5:
		return "moderate"
	case absCorr >= 0.3:
		return "weak"
	default:
		return "very_weak"
	}
}

// FormatForAgent formats correlation data for agent consumption
func (t *CorrelationTool) FormatForAgent(ctx context.Context, symbol string) (string, error) {
	// Get 1d correlation (most relevant for trading decisions)
	corr, err := t.GetBTCCorrelation(ctx, symbol, "1d")
	if err != nil {
		return "", err
	}

	// Get market regime
	regime, err := t.GetGlobalMarketRegime(ctx)
	if err != nil {
		// Don't fail if regime not available
		regime = &models.MarketRegimeResult{
			Regime:     "unknown",
			Confidence: 0.0,
		}
	}

	strength := t.GetCorrelationStrength(corr.BTCCorrelation)

	output := fmt.Sprintf(`📊 Correlation Analysis for %s:
- BTC Correlation: %.3f (%s)
- Market Regime: %s (confidence: %.2f)
- BTC Dominance: %.2f%%
- Avg Market Correlation: %.3f
- Volatility Level: %s

Interpretation:
%s`,
		symbol,
		corr.BTCCorrelation,
		strength,
		regime.Regime,
		regime.Confidence,
		regime.BTCDominance,
		regime.AvgCorrelation,
		regime.VolatilityLevel,
		t.getInterpretation(corr.BTCCorrelation, regime.Regime),
	)

	return output, nil
}

// getInterpretation provides trading insight based on correlation and regime
func (t *CorrelationTool) getInterpretation(correlation float64, regime string) string {
	if correlation > 0.7 {
		if regime == "risk_on" {
			return "Strong positive correlation + risk-on market = follow BTC direction closely"
		}
		return "Strong correlation to BTC - expect similar price action"
	}

	if correlation < -0.5 {
		return "Negative correlation to BTC - inverse price action expected"
	}

	if correlation > -0.3 && correlation < 0.3 {
		return "Weak correlation - asset moving independently from BTC"
	}

	return "Moderate correlation - consider BTC direction but not deterministic"
}

// Name returns the tool name for agent registry
func (t *CorrelationTool) Name() string {
	return "correlation_analysis"
}

// Description returns tool description for AI models
func (t *CorrelationTool) Description() string {
	return `Analyzes correlation between trading symbol and Bitcoin (BTC), 
provides market regime detection (risk-on/risk-off), and BTC dominance data. 
Helps understand if asset follows BTC or moves independently.`
}

// Execute runs the tool with given parameters (for agent toolkit interface)
func (t *CorrelationTool) Execute(ctx context.Context, params map[string]interface{}) (string, error) {
	symbol, ok := params["symbol"].(string)
	if !ok {
		return "", fmt.Errorf("missing or invalid 'symbol' parameter")
	}

	return t.FormatForAgent(ctx, symbol)
}
