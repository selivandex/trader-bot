package market

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"

	"github.com/selivandex/trader-bot/pkg/models"
)

// Repository handles market data storage (reads from ClickHouse)
type Repository struct {
	ch *sqlx.DB // ClickHouse connection
}

// NewRepository creates new market repository
func NewRepository(ch *sqlx.DB) *Repository {
	return &Repository{ch: ch}
}

// GetCandles retrieves candles from ClickHouse
func (r *Repository) GetCandles(ctx context.Context, symbol, timeframe string, limit int) ([]models.Candle, error) {
	query := `
		SELECT timestamp, symbol, timeframe, open, high, low, close, volume, quote_volume, trades
		FROM market_ohlcv
		WHERE symbol = ? AND timeframe = ?
		ORDER BY timestamp DESC
		LIMIT ?
	`

	rows, err := r.ch.QueryxContext(ctx, query, symbol, timeframe, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query candles from ClickHouse: %w", err)
	}
	defer rows.Close()

	candles := []models.Candle{}
	for rows.Next() {
		var candle models.Candle
		var open, high, low, close, volume, quoteVol float64
		var trades int

		err := rows.Scan(
			&candle.Timestamp,
			&candle.Symbol,
			&candle.Timeframe,
			&open,
			&high,
			&low,
			&close,
			&volume,
			&quoteVol,
			&trades,
		)
		if err != nil {
			continue
		}

		candle.Open = models.NewDecimal(open)
		candle.High = models.NewDecimal(high)
		candle.Low = models.NewDecimal(low)
		candle.Close = models.NewDecimal(close)
		candle.Volume = models.NewDecimal(volume)
		candle.QuoteVolume = models.NewDecimal(quoteVol)
		candle.Trades = trades

		candles = append(candles, candle)
	}

	// Reverse to chronological order (oldest first)
	for i, j := 0, len(candles)-1; i < j; i, j = i+1, j-1 {
		candles[i], candles[j] = candles[j], candles[i]
	}

	return candles, nil
}

// GetCandleCount returns number of stored candles for symbol/timeframe
func (r *Repository) GetCandleCount(ctx context.Context, symbol, timeframe string) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM market_ohlcv
		WHERE symbol = ? AND timeframe = ?
	`

	var count int
	err := r.ch.GetContext(ctx, &count, query, symbol, timeframe)
	return count, err
}

// GetLatestCandle returns the most recent candle
func (r *Repository) GetLatestCandle(ctx context.Context, symbol, timeframe string) (*models.Candle, error) {
	query := `
		SELECT timestamp, symbol, timeframe, open, high, low, close, volume, quote_volume, trades
		FROM market_ohlcv
		WHERE symbol = ? AND timeframe = ?
		ORDER BY timestamp DESC
		LIMIT 1
	`

	var candle models.Candle
	var open, high, low, close, volume, quoteVol float64
	var trades int

	err := r.ch.QueryRowxContext(ctx, query, symbol, timeframe).Scan(
		&candle.Timestamp,
		&candle.Symbol,
		&candle.Timeframe,
		&open,
		&high,
		&low,
		&close,
		&volume,
		&quoteVol,
		&trades,
	)
	if err != nil {
		return nil, err
	}

	candle.Open = models.NewDecimal(open)
	candle.High = models.NewDecimal(high)
	candle.Low = models.NewDecimal(low)
	candle.Close = models.NewDecimal(close)
	candle.Volume = models.NewDecimal(volume)
	candle.QuoteVolume = models.NewDecimal(quoteVol)
	candle.Trades = trades

	return &candle, nil
}
