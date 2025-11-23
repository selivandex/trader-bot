package workers

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/selivandex/trader-bot/internal/adapters/clickhouse"
	"github.com/selivandex/trader-bot/internal/adapters/exchange"
	"github.com/selivandex/trader-bot/pkg/logger"
)

// CandlesWorker periodically fetches and stores OHLCV candles to ClickHouse
type CandlesWorker struct {
	exchange     exchange.Exchange
	candleWriter *clickhouse.CandleBatchWriter
	symbols      []string
	timeframes   []string
	interval     time.Duration
}

// NewCandlesWorker creates new candles worker
func NewCandlesWorker(
	exchange exchange.Exchange,
	candleWriter *clickhouse.CandleBatchWriter,
	interval time.Duration,
	symbols []string,
	timeframes []string,
) *CandlesWorker {
	return &CandlesWorker{
		exchange:     exchange,
		candleWriter: candleWriter,
		interval:     interval,
		symbols:      symbols,
		timeframes:   timeframes,
	}
}

// Name returns worker name
func (cw *CandlesWorker) Name() string {
	return "candles_poller"
}

// Run executes one iteration - fetches candles and stores to batch writer
// Called periodically by pkg/worker.PeriodicWorker
func (cw *CandlesWorker) Run(ctx context.Context) error {
	cw.fetchAndStore(ctx)
	return nil
}

// fetchAndStore fetches candles and adds to batch writer
func (cw *CandlesWorker) fetchAndStore(ctx context.Context) {
	logger.Debug("fetching candles from exchange...")

	startTime := time.Now()
	totalFetched := 0

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

			// Add all candles to batch buffer (will auto-flush)
			for _, candle := range candles {
				candle.Symbol = symbol
				candle.Timeframe = timeframe
				cw.candleWriter.AddCandle(candle)
			}

			totalFetched += len(candles)
		}
	}

	latency := time.Since(startTime)

	logger.Info("candles fetched and buffered",
		zap.Int("total", totalFetched),
		zap.Duration("latency", latency),
	)
}
