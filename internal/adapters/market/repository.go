package market

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"

	"github.com/alexanderselivanov/trader/pkg/models"
)

// Repository handles market data storage
type Repository struct {
	db *sqlx.DB
}

// NewRepository creates new market repository
func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{db: db}
}

// SaveCandles saves OHLCV candles to database
func (r *Repository) SaveCandles(ctx context.Context, symbol, timeframe string, candles []models.Candle) error {
	if len(candles) == 0 {
		return nil
	}

	// Batch insert with ON CONFLICT
	query := `
		INSERT INTO ohlcv_candles (symbol, timeframe, timestamp, open, high, low, close, volume)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (symbol, timeframe, timestamp) DO UPDATE
		SET open = EXCLUDED.open,
		    high = EXCLUDED.high,
		    low = EXCLUDED.low,
		    close = EXCLUDED.close,
		    volume = EXCLUDED.volume
	`

	saved := 0
	for _, candle := range candles {
		_, err := r.db.ExecContext(
			ctx, query,
			symbol,
			timeframe,
			candle.Timestamp,
			candle.Open,
			candle.High,
			candle.Low,
			candle.Close,
			candle.Volume,
		)
		if err != nil {
			// Continue on error
			continue
		}
		saved++
	}

	return nil
}

// GetCandles retrieves candles from database
func (r *Repository) GetCandles(ctx context.Context, symbol, timeframe string, limit int) ([]models.Candle, error) {
	query := `
		SELECT timestamp, open, high, low, close, volume
		FROM ohlcv_candles
		WHERE symbol = $1 AND timeframe = $2
		ORDER BY timestamp DESC
		LIMIT $3
	`

	rows, err := r.db.QueryContext(ctx, query, symbol, timeframe, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query candles: %w", err)
	}
	defer rows.Close()

	candles := []models.Candle{}
	for rows.Next() {
		var candle models.Candle

		err := rows.Scan(
			&candle.Timestamp,
			&candle.Open,
			&candle.High,
			&candle.Low,
			&candle.Close,
			&candle.Volume,
		)
		if err != nil {
			continue
		}

		candles = append(candles, candle)
	}

	// Reverse to chronological order
	for i, j := 0, len(candles)-1; i < j; i, j = i+1, j-1 {
		candles[i], candles[j] = candles[j], candles[i]
	}

	return candles, nil
}

// GetCandleCount returns number of stored candles for symbol/timeframe
func (r *Repository) GetCandleCount(ctx context.Context, symbol, timeframe string) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM ohlcv_candles
		WHERE symbol = $1 AND timeframe = $2
	`

	var count int
	err := r.db.GetContext(ctx, &count, query, symbol, timeframe)
	return count, err
}
