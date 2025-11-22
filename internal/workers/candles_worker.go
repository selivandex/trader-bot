package workers

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/alexanderselivanov/trader/internal/adapters/exchange"
	"github.com/alexanderselivanov/trader/internal/adapters/market"
	"github.com/alexanderselivanov/trader/pkg/logger"
)

// CandlesWorker periodically fetches and stores OHLCV candles
type CandlesWorker struct {
	exchange      exchange.Exchange
	marketRepo    *market.Repository
	interval      time.Duration
	symbols       []string
	timeframes    []string
}

// NewCandlesWorker creates new candles worker
func NewCandlesWorker(
	exchange exchange.Exchange,
	marketRepo *market.Repository,
	interval time.Duration,
	symbols []string,
	timeframes []string,
) *CandlesWorker {
	return &CandlesWorker{
		exchange:   exchange,
		marketRepo: marketRepo,
		interval:   interval,
		symbols:    symbols,
		timeframes: timeframes,
	}
}

// Start starts the candles worker
func (cw *CandlesWorker) Start(ctx context.Context) error {
	logger.Info("candles worker starting",
		zap.Duration("interval", cw.interval),
		zap.Strings("symbols", cw.symbols),
		zap.Strings("timeframes", cw.timeframes),
	)
	
	// Fetch immediately
	cw.fetchAndStore(ctx)
	
	ticker := time.NewTicker(cw.interval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			logger.Info("candles worker stopped")
			return ctx.Err()
			
		case <-ticker.C:
			cw.fetchAndStore(ctx)
		}
	}
}

// fetchAndStore fetches candles and stores to database
func (cw *CandlesWorker) fetchAndStore(ctx context.Context) {
	logger.Debug("fetching candles from exchange...")
	
	startTime := time.Now()
	totalSaved := 0
	
	for _, symbol := range cw.symbols {
		for _, timeframe := range cw.timeframes {
			// Fetch 100 candles
			candles, err := cw.exchange.FetchOHLCV(ctx, symbol, timeframe, 100)
			if err != nil {
				logger.Warn("failed to fetch candles",
					zap.String("symbol", symbol),
					zap.String("timeframe", timeframe),
					zap.Error(err),
				)
				continue
			}
			
			// Save to database
			if err := cw.marketRepo.SaveCandles(ctx, symbol, timeframe, candles); err != nil {
				logger.Error("failed to save candles",
					zap.String("symbol", symbol),
					zap.String("timeframe", timeframe),
					zap.Error(err),
				)
				continue
			}
			
			totalSaved += len(candles)
		}
	}
	
	latency := time.Since(startTime)
	
	logger.Info("candles saved",
		zap.Int("total", totalSaved),
		zap.Duration("latency", latency),
	)
}

