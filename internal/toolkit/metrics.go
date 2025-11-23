package toolkit

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"

	"github.com/selivandex/trader-bot/pkg/logger"
)

// ToolUsageMetric represents single tool usage event
type ToolUsageMetric struct {
	AgentID         string
	AgentName       string
	Personality     string
	ToolName        string
	Query           string
	ResultsCount    int
	AvgSimilarity   float32
	WasUseful       bool
	ExecutionTimeMs int
	Timestamp       time.Time
}

// BatchedMetricsLogger buffers tool usage metrics and writes to ClickHouse in batches
// Optimized for high-throughput ClickHouse inserts
type BatchedMetricsLogger struct {
	chDB          *sqlx.DB
	agentName     string
	personality   string
	buffer        []ToolUsageMetric
	bufferMu      sync.Mutex
	batchSize     int
	flushInterval time.Duration
	stopChan      chan struct{}
	wg            sync.WaitGroup
}

// NewBatchedMetricsLogger creates new batched metrics logger
func NewBatchedMetricsLogger(chDB *sqlx.DB, agentName, personality string) *BatchedMetricsLogger {
	logger := &BatchedMetricsLogger{
		chDB:          chDB,
		agentName:     agentName,
		personality:   personality,
		buffer:        make([]ToolUsageMetric, 0, 100),
		batchSize:     100,              // Flush every 100 records
		flushInterval: 10 * time.Second, // Or every 10 seconds
		stopChan:      make(chan struct{}),
	}

	// Start background flusher
	logger.wg.Add(1)
	go logger.flushLoop()

	return logger
}

// LogToolUsage adds metric to buffer (non-blocking)
func (l *BatchedMetricsLogger) LogToolUsage(
	ctx context.Context,
	toolName string,
	params interface{},
	resultCount int,
	avgSimilarity float32,
	useful bool,
	executionTimeMs int,
) {
	// Serialize params to JSON
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		paramsJSON = []byte("{}")
	}

	metric := ToolUsageMetric{
		AgentID:         "", // Will be set if available
		AgentName:       l.agentName,
		Personality:     l.personality,
		ToolName:        toolName,
		Query:           string(paramsJSON),
		ResultsCount:    resultCount,
		AvgSimilarity:   avgSimilarity,
		WasUseful:       useful,
		ExecutionTimeMs: executionTimeMs,
		Timestamp:       time.Now(),
	}

	l.bufferMu.Lock()
	l.buffer = append(l.buffer, metric)
	shouldFlush := len(l.buffer) >= l.batchSize
	l.bufferMu.Unlock()

	// Trigger immediate flush if buffer full
	if shouldFlush {
		go l.flush()
	}
}

// flushLoop periodically flushes buffer to ClickHouse
func (l *BatchedMetricsLogger) flushLoop() {
	defer l.wg.Done()

	ticker := time.NewTicker(l.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			l.flush()
		case <-l.stopChan:
			// Final flush on shutdown
			l.flush()
			return
		}
	}
}

// flush writes buffered metrics to ClickHouse
func (l *BatchedMetricsLogger) flush() {
	l.bufferMu.Lock()
	if len(l.buffer) == 0 {
		l.bufferMu.Unlock()
		return
	}

	// Swap buffers to minimize lock time
	toFlush := l.buffer
	l.buffer = make([]ToolUsageMetric, 0, l.batchSize)
	l.bufferMu.Unlock()

	// Write batch to ClickHouse
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := l.writeBatch(ctx, toFlush); err != nil {
		logger.Error("failed to write metrics batch to ClickHouse",
			zap.Int("batch_size", len(toFlush)),
			zap.Error(err),
		)
		// TODO: Consider retry logic or dead letter queue
		return
	}

	logger.Debug("metrics batch written to ClickHouse",
		zap.Int("batch_size", len(toFlush)),
		zap.String("agent", l.agentName),
	)
}

// writeBatch performs single batch insert to ClickHouse
func (l *BatchedMetricsLogger) writeBatch(ctx context.Context, metrics []ToolUsageMetric) error {
	if l.chDB == nil {
		return nil // ClickHouse not available, silently skip
	}

	// ClickHouse batch insert - single multi-row INSERT
	tx, err := l.chDB.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO agent_tool_usage (
			agent_id, agent_name, personality, tool_name, 
			query, results_count, avg_similarity, was_useful, 
			execution_time_ms, timestamp, date
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	// Execute batch insert
	for _, m := range metrics {
		_, err := stmt.ExecContext(ctx,
			m.AgentID,
			m.AgentName,
			m.Personality,
			m.ToolName,
			m.Query,
			m.ResultsCount,
			m.AvgSimilarity,
			m.WasUseful,
			m.ExecutionTimeMs,
			m.Timestamp,
			m.Timestamp.Format("2006-01-02"), // date column
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// Close gracefully shuts down metrics logger and flushes remaining buffer
func (l *BatchedMetricsLogger) Close() error {
	close(l.stopChan)
	l.wg.Wait()

	logger.Info("metrics logger closed",
		zap.String("agent", l.agentName),
	)

	return nil
}

// GetBufferSize returns current buffer size (for monitoring)
func (l *BatchedMetricsLogger) GetBufferSize() int {
	l.bufferMu.Lock()
	defer l.bufferMu.Unlock()
	return len(l.buffer)
}
