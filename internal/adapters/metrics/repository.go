package metrics

import (
	"context"

	"github.com/selivandex/trader-bot/pkg/metrics"
)

// Repository interface for metrics storage operations
type Repository interface {
	// InsertBatch inserts batch of metrics into specific table
	InsertBatch(ctx context.Context, tableName string, values [][]interface{}) error
	// Close closes repository connection
	Close() error
}

// Writer implements metrics.Writer using Repository pattern
type Writer struct {
	repo Repository
}

// NewWriter creates new metrics writer with repository
func NewWriter(repo Repository) *Writer {
	return &Writer{repo: repo}
}

// Write writes batch of metrics to storage via repository
func (w *Writer) Write(ctx context.Context, tableName string, metricsSlice []metrics.Metric) error {
	if len(metricsSlice) == 0 {
		return nil
	}

	// Convert metrics to values
	values := make([][]interface{}, len(metricsSlice))
	for i, metric := range metricsSlice {
		values[i] = metric.Values()
	}

	// Use repository to insert
	return w.repo.InsertBatch(ctx, tableName, values)
}

// Close closes writer
func (w *Writer) Close() error {
	if w.repo != nil {
		return w.repo.Close()
	}
	return nil
}
