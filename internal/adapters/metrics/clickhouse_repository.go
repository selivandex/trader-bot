package metrics

import (
	"context"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"

	"github.com/selivandex/trader-bot/pkg/logger"
)

// ClickHouseRepository implements Repository for ClickHouse
type ClickHouseRepository struct {
	db *sqlx.DB
}

// NewClickHouseRepository creates new ClickHouse repository
func NewClickHouseRepository(db *sqlx.DB) *ClickHouseRepository {
	return &ClickHouseRepository{db: db}
}

// InsertBatch inserts batch of metrics into ClickHouse table
func (r *ClickHouseRepository) InsertBatch(ctx context.Context, tableName string, values [][]interface{}) error {
	if len(values) == 0 {
		return nil
	}

	// Get column count from first row
	columnCount := len(values[0])
	if columnCount == 0 {
		return fmt.Errorf("values have no columns")
	}

	// Build INSERT query with placeholders
	placeholders := make([]string, len(values))
	args := make([]interface{}, 0, len(values)*columnCount)

	for i, row := range values {
		if len(row) != columnCount {
			return fmt.Errorf("row %d has wrong column count: expected %d, got %d", i, columnCount, len(row))
		}

		// Create placeholder like (?, ?, ?)
		valuePlaceholders := make([]string, columnCount)
		for j := range row {
			valuePlaceholders[j] = "?"
		}
		placeholders[i] = "(" + strings.Join(valuePlaceholders, ", ") + ")"

		// Append values
		args = append(args, row...)
	}

	// Execute batch insert
	query := fmt.Sprintf("INSERT INTO %s VALUES %s", tableName, strings.Join(placeholders, ", "))

	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("ClickHouse insert failed: %w", err)
	}

	logger.Debug("ClickHouse batch insert successful",
		zap.String("table", tableName),
		zap.Int("rows", len(values)),
	)

	return nil
}

// Close closes ClickHouse repository
func (r *ClickHouseRepository) Close() error {
	// DB is managed externally, don't close it
	return nil
}
