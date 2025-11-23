package clickhouse

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/selivandex/trader-bot/pkg/logger"
	"github.com/selivandex/trader-bot/pkg/models"
)

// BatchWriter buffers records and writes them via repository in batches
type BatchWriter struct {
	repo        *Repository
	buffer      []interface{}
	bufferMu    sync.Mutex
	maxBatch    int
	maxWait     time.Duration
	flushTicker *time.Ticker
	flushFunc   func(context.Context, *Repository, []interface{}) error
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
}

// NewBatchWriter creates new batch writer
func NewBatchWriter(
	repo *Repository,
	maxBatch int,
	maxWait time.Duration,
	flushFunc func(context.Context, *Repository, []interface{}) error,
) *BatchWriter {
	ctx, cancel := context.WithCancel(context.Background())

	bw := &BatchWriter{
		repo:      repo,
		buffer:    make([]interface{}, 0, maxBatch),
		maxBatch:  maxBatch,
		maxWait:   maxWait,
		flushFunc: flushFunc,
		ctx:       ctx,
		cancel:    cancel,
	}

	bw.flushTicker = time.NewTicker(maxWait)

	bw.wg.Add(1)
	go bw.autoFlush()

	return bw
}

// Add adds record to buffer
func (bw *BatchWriter) Add(record interface{}) {
	bw.bufferMu.Lock()
	bw.buffer = append(bw.buffer, record)
	shouldFlush := len(bw.buffer) >= bw.maxBatch
	bw.bufferMu.Unlock()

	if shouldFlush {
		bw.flush()
	}
}

// autoFlush flushes buffer periodically
func (bw *BatchWriter) autoFlush() {
	defer bw.wg.Done()

	for {
		select {
		case <-bw.flushTicker.C:
			bw.flush()
		case <-bw.ctx.Done():
			// Final flush before exit
			bw.flush()
			return
		}
	}
}

// flush writes buffered records to ClickHouse via repository
func (bw *BatchWriter) flush() {
	bw.bufferMu.Lock()
	if len(bw.buffer) == 0 {
		bw.bufferMu.Unlock()
		return
	}

	// Copy buffer
	toWrite := make([]interface{}, len(bw.buffer))
	copy(toWrite, bw.buffer)
	bw.buffer = bw.buffer[:0]
	bw.bufferMu.Unlock()

	// Write via repository
	ctx, cancel := context.WithTimeout(bw.ctx, 30*time.Second)
	defer cancel()

	if err := bw.flushFunc(ctx, bw.repo, toWrite); err != nil {
		logger.Error("failed to flush batch to ClickHouse",
			zap.Int("records", len(toWrite)),
			zap.Error(err),
		)
		return
	}

	logger.Debug("flushed batch to ClickHouse",
		zap.Int("records", len(toWrite)),
	)
}

// Close stops the writer and flushes remaining data
func (bw *BatchWriter) Close() error {
	bw.flushTicker.Stop()
	bw.cancel()
	bw.wg.Wait()
	return nil
}

// CandleBatchWriter specialized writer for candles
type CandleBatchWriter struct {
	*BatchWriter
}

// NewCandleBatchWriter creates batch writer for OHLCV candles
func NewCandleBatchWriter(repo *Repository, maxBatch int, maxWait time.Duration) *CandleBatchWriter {
	flushFunc := func(ctx context.Context, r *Repository, records []interface{}) error {
		// Group candles by symbol+timeframe
		grouped := make(map[string][]models.Candle)

		for _, record := range records {
			candle := record.(models.Candle)
			key := candle.Symbol + "|" + candle.Timeframe
			grouped[key] = append(grouped[key], candle)
		}

		// Save each group
		for _, candles := range grouped {
			if len(candles) > 0 {
				symbol := candles[0].Symbol
				timeframe := candles[0].Timeframe
				if err := r.SaveCandles(ctx, symbol, timeframe, candles); err != nil {
					return err
				}
			}
		}

		return nil
	}

	bw := NewBatchWriter(repo, maxBatch, maxWait, flushFunc)

	return &CandleBatchWriter{BatchWriter: bw}
}

// AddCandle adds candle to buffer
func (cbw *CandleBatchWriter) AddCandle(candle models.Candle) {
	cbw.Add(candle)
}

// TradeBatchWriter specialized writer for trades history
type TradeBatchWriter struct {
	*BatchWriter
}

// NewTradeBatchWriter creates batch writer for trades
func NewTradeBatchWriter(repo *Repository, maxBatch int, maxWait time.Duration) *TradeBatchWriter {
	flushFunc := func(ctx context.Context, r *Repository, records []interface{}) error {
		trades := make([]models.Trade, len(records))
		for i, record := range records {
			trades[i] = record.(models.Trade)
		}
		return r.SaveTrades(ctx, trades)
	}

	bw := NewBatchWriter(repo, maxBatch, maxWait, flushFunc)

	return &TradeBatchWriter{BatchWriter: bw}
}

// AddTrade adds trade to buffer
func (tbw *TradeBatchWriter) AddTrade(trade models.Trade) {
	tbw.Add(trade)
}

// NewsBatchWriter specialized writer for news
type NewsBatchWriter struct {
	*BatchWriter
}

// NewNewsBatchWriter creates batch writer for news
func NewNewsBatchWriter(repo *Repository, maxBatch int, maxWait time.Duration) *NewsBatchWriter {
	flushFunc := func(ctx context.Context, r *Repository, records []interface{}) error {
		articles := make([]models.NewsItem, len(records))
		for i, record := range records {
			articles[i] = record.(models.NewsItem)
		}
		return r.SaveNews(ctx, articles)
	}

	bw := NewBatchWriter(repo, maxBatch, maxWait, flushFunc)

	return &NewsBatchWriter{BatchWriter: bw}
}

// AddNews adds news article to buffer
func (nbw *NewsBatchWriter) AddNews(article models.NewsItem) {
	nbw.Add(article)
}
