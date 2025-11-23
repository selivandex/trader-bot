package workers

import (
	"context"
	"time"

	"github.com/selivandex/trader-bot/pkg/logger"
	"go.uber.org/zap"

	"github.com/selivandex/trader-bot/internal/adapters/correlation"
)

// CorrelationWorker periodically calculates asset correlations and detects market regime
type CorrelationWorker struct {
	calculator *correlation.Calculator
	symbols    []string // Symbols to track correlations for
	interval   time.Duration
}

// NewCorrelationWorker creates a new correlation worker
func NewCorrelationWorker(calculator *correlation.Calculator, symbols []string) *CorrelationWorker {
	if len(symbols) == 0 {
		// Default top symbols for correlation tracking
		symbols = []string{
			"BTC/USDT",
			"ETH/USDT",
			"BNB/USDT",
			"SOL/USDT",
			"XRP/USDT",
			"ADA/USDT",
			"AVAX/USDT",
			"DOT/USDT",
			"MATIC/USDT",
			"LINK/USDT",
		}
	}

	return &CorrelationWorker{
		calculator: calculator,
		symbols:    symbols,
		interval:   1 * time.Hour, // Calculate every hour
	}
}

// Start begins the correlation calculation worker
func (w *CorrelationWorker) Start(ctx context.Context) error {
	logger.Info("Starting correlation worker",
		zap.Duration("interval", w.interval),
		zap.Int("symbols", len(w.symbols)),
	)

	// Run immediately on startup
	w.calculateCorrelations(ctx)
	w.detectMarketRegime(ctx)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			w.calculateCorrelations(ctx)
			w.detectMarketRegime(ctx)

		case <-ctx.Done():
			logger.Info("Correlation worker stopped")
			return ctx.Err()
		}
	}
}

// calculateCorrelations computes correlations for all tracked symbols against BTC
func (w *CorrelationWorker) calculateCorrelations(ctx context.Context) {
	logger.Info("Calculating asset correlations", zap.Int("count", len(w.symbols)))

	periods := []string{"1h", "4h", "1d"}

	for _, symbol := range w.symbols {
		if symbol == "BTC/USDT" {
			continue // Skip BTC vs BTC
		}

		for _, period := range periods {
			corr, err := w.calculator.CalculateAssetCorrelation(ctx, symbol, "BTC/USDT", period)
			if err != nil {
				logger.Warn("Failed to calculate correlation",
					zap.String("symbol", symbol),
					zap.String("period", period),
					zap.Error(err),
				)
				continue
			}

			logger.Info("Correlation calculated",
				zap.String("base", symbol),
				zap.String("quote", "BTC/USDT"),
				zap.String("period", period),
				zap.Float64("correlation", corr.Correlation),
				zap.Int("samples", corr.SampleSize),
			)
		}
	}
}

// detectMarketRegime analyzes overall market conditions
func (w *CorrelationWorker) detectMarketRegime(ctx context.Context) {
	logger.Info("Detecting market regime")

	regime, err := w.calculator.DetectMarketRegime(ctx, w.symbols)
	if err != nil {
		logger.Error("Failed to detect market regime", zap.Error(err))
		return
	}

	logger.Info("Market regime detected",
		zap.String("regime", regime.Regime),
		zap.Float64("btc_dominance", regime.BTCDominance),
		zap.Float64("avg_correlation", regime.AvgCorrelation),
		zap.String("volatility", regime.VolatilityLevel),
		zap.Float64("confidence", regime.Confidence),
	)
}

// Stop gracefully stops the worker (called by context cancellation)
func (w *CorrelationWorker) Stop() {
	logger.Info("Correlation worker stopping")
}
