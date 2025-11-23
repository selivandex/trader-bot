package workers

import (
	"context"

	"go.uber.org/zap"

	"github.com/selivandex/trader-bot/internal/adapters/clickhouse"
	"github.com/selivandex/trader-bot/internal/adapters/exchange"
	"github.com/selivandex/trader-bot/pkg/logger"
)

// RealtimeMarketWorker listens to Bybit WebSocket and writes data to ClickHouse
type RealtimeMarketWorker struct {
	ws           *exchange.BybitWebSocket
	candleWriter *clickhouse.CandleBatchWriter
	testnet      bool
}

// NewRealtimeMarketWorker creates new real-time market worker
func NewRealtimeMarketWorker(
	candleWriter *clickhouse.CandleBatchWriter,
	symbols []string,
	timeframes []string,
	testnet bool,
) *RealtimeMarketWorker {
	ws := exchange.NewBybitWebSocket(symbols, timeframes, testnet)

	return &RealtimeMarketWorker{
		ws:           ws,
		candleWriter: candleWriter,
		testnet:      testnet,
	}
}

// Run starts listening to WebSocket and writing to ClickHouse
func (w *RealtimeMarketWorker) Run(ctx context.Context) error {
	logger.Info("starting real-time market worker (Bybit WebSocket)",
		zap.Bool("testnet", w.testnet),
	)

	// Connect to WebSocket
	if err := w.ws.Connect(); err != nil {
		return err
	}
	defer w.ws.Close()

	// Listen to candles and errors
	for {
		select {
		case <-ctx.Done():
			logger.Info("real-time market worker stopping")
			return nil

		case candle := <-w.ws.Candles():
			// Add to batch buffer (auto-flushes when full)
			w.candleWriter.AddCandle(candle)

		case err := <-w.ws.Errors():
			logger.Error("WebSocket error",
				zap.Error(err),
			)
			// WebSocket will auto-reconnect
		}
	}
}
