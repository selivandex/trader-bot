package metrics

import "context"

// Metric is a generic interface for any metric record
type Metric interface {
	// TableName returns ClickHouse table name for this metric
	TableName() string
	// Values returns metric values in the same order as columns
	Values() []interface{}
}

// Writer writes metrics to storage (ClickHouse, Postgres, etc.)
type Writer interface {
	// Write writes batch of metrics to storage
	Write(ctx context.Context, tableName string, metrics []Metric) error
	// Close closes writer and flushes any remaining data
	Close() error
}

// Buffer manages batching and auto-flushing of metrics
type Buffer interface {
	// Add adds metric to buffer (thread-safe)
	Add(metric Metric) error
	// Flush flushes buffer to writer
	Flush(ctx context.Context) error
	// Size returns current buffer size
	Size() int
	// Close flushes and closes buffer
	Close(ctx context.Context) error
}
