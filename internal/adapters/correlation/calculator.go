package correlation

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/selivandex/trader-bot/internal/adapters/market"
	"github.com/selivandex/trader-bot/pkg/logger"
	"github.com/selivandex/trader-bot/pkg/models"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Calculator computes correlation coefficients between assets
type Calculator struct {
	marketRepo *market.Repository
	corrRepo   *Repository
}

// NewCalculator creates a new correlation calculator
func NewCalculator(marketRepo *market.Repository, corrRepo *Repository) *Calculator {
	return &Calculator{
		marketRepo: marketRepo,
		corrRepo:   corrRepo,
	}
}

// CalculatePearsonCorrelation computes Pearson correlation coefficient
// between two price series (returns)
func (c *Calculator) CalculatePearsonCorrelation(returns1, returns2 []float64) (float64, error) {
	if len(returns1) != len(returns2) || len(returns1) == 0 {
		return 0, fmt.Errorf("invalid return series lengths")
	}

	n := float64(len(returns1))

	// Calculate means
	var sum1, sum2 float64
	for i := range returns1 {
		sum1 += returns1[i]
		sum2 += returns2[i]
	}
	mean1 := sum1 / n
	mean2 := sum2 / n

	// Calculate correlation coefficient
	var numerator, var1, var2 float64
	for i := range returns1 {
		diff1 := returns1[i] - mean1
		diff2 := returns2[i] - mean2
		numerator += diff1 * diff2
		var1 += diff1 * diff1
		var2 += diff2 * diff2
	}

	if var1 == 0 || var2 == 0 {
		return 0, nil // No variance = no correlation
	}

	correlation := numerator / math.Sqrt(var1*var2)
	return correlation, nil
}

// CalculateAssetCorrelation calculates correlation between base and quote symbols
func (c *Calculator) CalculateAssetCorrelation(ctx context.Context, baseSymbol, quoteSymbol, period string) (*models.AssetCorrelation, error) {
	// Determine lookback period
	var lookback time.Duration
	switch period {
	case "1h":
		lookback = 24 * time.Hour // 24 hours of data
	case "4h":
		lookback = 7 * 24 * time.Hour // 7 days
	case "1d":
		lookback = 30 * 24 * time.Hour // 30 days
	default:
		return nil, fmt.Errorf("unsupported period: %s", period)
	}

	// Fetch price data for both symbols
	// Note: This assumes you have GetCandles method in market.Repository
	// If using ClickHouse, fetch from there; otherwise from PostgreSQL
	basePrices, err := c.getPriceReturns(ctx, baseSymbol, period, lookback)
	if err != nil {
		return nil, fmt.Errorf("failed to get base prices: %w", err)
	}

	quotePrices, err := c.getPriceReturns(ctx, quoteSymbol, period, lookback)
	if err != nil {
		return nil, fmt.Errorf("failed to get quote prices: %w", err)
	}

	// Align series (in case of missing data)
	minLen := len(basePrices)
	if len(quotePrices) < minLen {
		minLen = len(quotePrices)
	}
	basePrices = basePrices[:minLen]
	quotePrices = quotePrices[:minLen]

	// Calculate correlation
	correlation, err := c.CalculatePearsonCorrelation(basePrices, quotePrices)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate correlation: %w", err)
	}

	// Create correlation record
	corr := &models.AssetCorrelation{
		ID:           uuid.New(),
		BaseSymbol:   baseSymbol,
		QuoteSymbol:  quoteSymbol,
		Period:       period,
		Correlation:  correlation,
		SampleSize:   minLen,
		CalculatedAt: time.Now(),
		CreatedAt:    time.Now(),
	}

	// Save to database
	if err := c.corrRepo.SaveCorrelation(ctx, corr); err != nil {
		logger.Warn("Failed to save correlation", zap.Error(err))
		// Don't return error, we still have the calculation
	}

	return corr, nil
}

// getPriceReturns fetches price data and calculates returns
func (c *Calculator) getPriceReturns(ctx context.Context, symbol, period string, lookback time.Duration) ([]float64, error) {
	// This is a placeholder - implement based on your market data source
	// Option 1: Fetch from ClickHouse if available
	// Option 2: Fetch from PostgreSQL ohlcv_candles
	// Option 3: Fetch real-time via exchange adapter

	// For now, return empty to avoid errors
	// TODO: Implement actual data fetching
	logger.Warn("getPriceReturns not fully implemented - requires market data integration",
		zap.String("symbol", symbol),
		zap.String("period", period),
		zap.Duration("lookback", lookback),
	)

	return []float64{}, fmt.Errorf("market data integration needed")
}

// DetectMarketRegime analyzes overall market conditions
func (c *Calculator) DetectMarketRegime(ctx context.Context, topSymbols []string) (*models.MarketRegime, error) {
	// Calculate correlations with BTC for top symbols
	var correlations []float64
	for _, symbol := range topSymbols {
		if symbol == "BTC/USDT" {
			continue
		}

		corr, err := c.CalculateAssetCorrelation(ctx, symbol, "BTC/USDT", "1d")
		if err != nil {
			logger.Warn("Failed to calculate correlation",
				zap.String("symbol", symbol),
				zap.Error(err),
			)
			continue
		}
		correlations = append(correlations, corr.Correlation)
	}

	if len(correlations) == 0 {
		return nil, fmt.Errorf("no correlations calculated")
	}

	// Calculate average correlation
	var sum float64
	for _, c := range correlations {
		sum += c
	}
	avgCorr := sum / float64(len(correlations))

	// Determine market regime
	var regime string
	var confidence float64

	if avgCorr > 0.7 {
		regime = "risk_on"
		confidence = avgCorr
	} else if avgCorr < 0.3 {
		regime = "risk_off"
		confidence = 1.0 - avgCorr
	} else {
		regime = "neutral"
		confidence = 0.5
	}

	// TODO: Fetch actual BTC dominance from CoinGecko or similar
	btcDominance := 50.0 // Placeholder

	// TODO: Calculate actual volatility
	volatilityLevel := "medium" // Placeholder

	marketRegime := &models.MarketRegime{
		ID:              uuid.New(),
		Regime:          regime,
		BTCDominance:    btcDominance,
		AvgCorrelation:  avgCorr,
		VolatilityLevel: volatilityLevel,
		Confidence:      confidence,
		DetectedAt:      time.Now(),
		CreatedAt:       time.Now(),
	}

	// Save to database
	if err := c.corrRepo.SaveMarketRegime(ctx, marketRegime); err != nil {
		logger.Warn("Failed to save market regime", zap.Error(err))
	}

	return marketRegime, nil
}

// GetBTCDominance fetches BTC market cap dominance (placeholder)
func (c *Calculator) GetBTCDominance(ctx context.Context) (float64, error) {
	// TODO: Integrate with CoinGecko API
	// GET https://api.coingecko.com/api/v3/global
	// Parse response.data.market_cap_percentage.btc

	return 50.0, fmt.Errorf("BTC dominance API integration needed")
}
