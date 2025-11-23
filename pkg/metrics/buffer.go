package metrics

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/selivandex/trader-bot/pkg/logger"
)

// BufferedMetrics manages batched metrics with auto-flush
type BufferedMetrics struct {
	writer      Writer
	buffer      map[string][]Metric
	flushTicker *time.Ticker
	stopCh      chan struct{}
	wg          sync.WaitGroup
	batchSize   int
	bufferMu    sync.RWMutex
}

// BufferConfig configures metrics buffer
type BufferConfig struct {
	Writer        Writer
	BatchSize     int           // Flush when buffer reaches this size
	FlushInterval time.Duration // Auto-flush interval
	MaxBufferSize int           // Max buffer size before blocking (0 = unlimited)
}

// NewBufferedMetrics creates new buffered metrics manager
func NewBufferedMetrics(cfg BufferConfig) *BufferedMetrics {
	if cfg.BatchSize == 0 {
		cfg.BatchSize = 100 // Default batch size
	}
	if cfg.FlushInterval == 0 {
		cfg.FlushInterval = 10 * time.Second // Default flush interval
	}

	bm := &BufferedMetrics{
		writer:      cfg.Writer,
		buffer:      make(map[string][]Metric),
		batchSize:   cfg.BatchSize,
		flushTicker: time.NewTicker(cfg.FlushInterval),
		stopCh:      make(chan struct{}),
	}

	// Start auto-flush goroutine
	bm.wg.Add(1)
	go bm.autoFlush()

	logger.Info("metrics buffer initialized",
		zap.Int("batch_size", cfg.BatchSize),
		zap.Duration("flush_interval", cfg.FlushInterval),
	)

	return bm
}

// Add adds metric to buffer (thread-safe)
func (bm *BufferedMetrics) Add(metric Metric) error {
	if metric == nil {
		return fmt.Errorf("metric is nil")
	}

	tableName := metric.TableName()
	if tableName == "" {
		return fmt.Errorf("metric table name is empty")
	}

	bm.bufferMu.Lock()
	defer bm.bufferMu.Unlock()

	bm.buffer[tableName] = append(bm.buffer[tableName], metric)

	// Auto-flush if batch size reached
	if len(bm.buffer[tableName]) >= bm.batchSize {
		logger.Debug("batch size reached, flushing",
			zap.String("table", tableName),
			zap.Int("size", len(bm.buffer[tableName])),
		)
		// Flush in background to avoid blocking
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := bm.Flush(ctx); err != nil {
				logger.Error("auto-flush failed", zap.Error(err))
			}
		}()
	}

	return nil
}

// Flush flushes all buffered metrics to writer
func (bm *BufferedMetrics) Flush(ctx context.Context) error {
	bm.bufferMu.Lock()

	// Copy buffer and clear it
	toFlush := make(map[string][]Metric)
	for table, metrics := range bm.buffer {
		if len(metrics) > 0 {
			toFlush[table] = metrics
			bm.buffer[table] = nil
		}
	}
	bm.bufferMu.Unlock()

	if len(toFlush) == 0 {
		return nil
	}

	// Flush each table
	var errors []error
	for tableName, metrics := range toFlush {
		if err := bm.writer.Write(ctx, tableName, metrics); err != nil {
			logger.Error("failed to flush metrics",
				zap.String("table", tableName),
				zap.Int("count", len(metrics)),
				zap.Error(err),
			)
			errors = append(errors, err)
		} else {
			logger.Debug("metrics flushed successfully",
				zap.String("table", tableName),
				zap.Int("count", len(metrics)),
			)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("flush failed for %d tables", len(errors))
	}

	return nil
}

// Size returns current buffer size across all tables
func (bm *BufferedMetrics) Size() int {
	bm.bufferMu.RLock()
	defer bm.bufferMu.RUnlock()

	total := 0
	for _, metrics := range bm.buffer {
		total += len(metrics)
	}
	return total
}

// Close gracefully shuts down buffer and flushes remaining metrics
func (bm *BufferedMetrics) Close(ctx context.Context) error {
	logger.Info("closing metrics buffer...")

	// Stop auto-flush goroutine
	close(bm.stopCh)
	bm.flushTicker.Stop()

	// Wait for auto-flush to stop
	bm.wg.Wait()

	// Final flush
	if err := bm.Flush(ctx); err != nil {
		logger.Error("final flush failed", zap.Error(err))
		return err
	}

	// Close writer
	if err := bm.writer.Close(); err != nil {
		logger.Error("writer close failed", zap.Error(err))
		return err
	}

	logger.Info("✅ metrics buffer closed successfully")
	return nil
}

// autoFlush periodically flushes buffer
func (bm *BufferedMetrics) autoFlush() {
	defer bm.wg.Done()

	for {
		select {
		case <-bm.flushTicker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := bm.Flush(ctx); err != nil {
				logger.Warn("periodic flush failed", zap.Error(err))
			}
			cancel()

		case <-bm.stopCh:
			return
		}
	}
}
