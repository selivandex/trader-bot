package toolkit

import (
	"context"
	"encoding/json"
	"time"

	"go.uber.org/zap"

	"github.com/selivandex/trader-bot/pkg/logger"
	"github.com/selivandex/trader-bot/pkg/metrics"
)

// BatchedMetricsLogger wraps universal metrics buffer for tool usage
// Now uses metrics.BufferedMetrics under the hood for consistency
type BatchedMetricsLogger struct {
	buffer      metrics.Buffer
	agentName   string
	personality string
}

// NewBatchedMetricsLogger creates new batched metrics logger using universal buffer
func NewBatchedMetricsLogger(buffer metrics.Buffer, agentName, personality string) *BatchedMetricsLogger {
	return &BatchedMetricsLogger{
		buffer:      buffer,
		agentName:   agentName,
		personality: personality,
	}
}

// LogToolUsage adds metric to universal buffer (non-blocking)
func (l *BatchedMetricsLogger) LogToolUsage(
	ctx context.Context,
	toolName string,
	params interface{},
	resultCount int,
	avgSimilarity float32,
	useful bool,
	executionTimeMs int,
) {
	if l.buffer == nil {
		return // Metrics disabled
	}

	// Serialize params to JSON
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		paramsJSON = []byte("{}")
	}

	metric := &metrics.ToolUsageMetric{
		Timestamp:        time.Now(),
		AgentName:        l.agentName,
		AgentPersonality: l.personality,
		ToolName:         toolName,
		Params:           string(paramsJSON),
		ResultCount:      resultCount,
		AvgSimilarity:    avgSimilarity,
		Useful:           useful,
		ExecutionTimeMs:  executionTimeMs,
	}

	if err := l.buffer.Add(metric); err != nil {
		logger.Warn("failed to add tool metric to buffer", zap.Error(err))
	}
}

// Close gracefully shuts down metrics logger
// Buffer is closed externally in main.go for proper graceful shutdown coordination
func (l *BatchedMetricsLogger) Close() error {
	return nil
}

// GetBufferSize returns current buffer size (for monitoring)
func (l *BatchedMetricsLogger) GetBufferSize() int {
	if l.buffer == nil {
		return 0
	}
	return l.buffer.Size()
}
