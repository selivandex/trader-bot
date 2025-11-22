package price

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
)

// Repository handles price cache database operations
type Repository struct {
	db *sqlx.DB
}

// NewRepository creates new price repository
func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{db: db}
}

// SavePrice saves price to cache
func (r *Repository) SavePrice(ctx context.Context, symbol string, priceUSD float64, source string) error {
	query := `
		INSERT INTO price_cache (symbol, price_usd, source, updated_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (symbol, source)
		DO UPDATE SET
			price_usd = EXCLUDED.price_usd,
			updated_at = NOW()
	`

	_, err := r.db.ExecContext(ctx, query, symbol, priceUSD, source)
	return err
}

// GetPrice gets cached price for symbol
func (r *Repository) GetPrice(ctx context.Context, symbol string) (float64, error) {
	query := `
		SELECT price_usd
		FROM price_cache
		WHERE symbol = $1
		  AND updated_at > NOW() - INTERVAL '5 minutes'
		ORDER BY updated_at DESC
		LIMIT 1
	`

	var price decimal.Decimal
	err := r.db.GetContext(ctx, &price, query, symbol)
	if err != nil {
		return 0, fmt.Errorf("price not found in cache: %w", err)
	}

	priceFloat, _ := price.Float64()
	return priceFloat, nil
}

// GetPrices gets multiple prices
func (r *Repository) GetPrices(ctx context.Context, symbols []string) (map[string]float64, error) {
	query := `
		SELECT DISTINCT ON (symbol) symbol, price_usd
		FROM price_cache
		WHERE symbol = ANY($1)
		  AND updated_at > NOW() - INTERVAL '5 minutes'
		ORDER BY symbol, updated_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, symbols)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	prices := make(map[string]float64)
	for rows.Next() {
		var symbol string
		var price decimal.Decimal

		if err := rows.Scan(&symbol, &price); err != nil {
			continue
		}

		priceFloat, _ := price.Float64()
		prices[symbol] = priceFloat
	}

	return prices, nil
}

// GetAllRecentPrices gets all prices updated in last 5 min
func (r *Repository) GetAllRecentPrices(ctx context.Context) (map[string]float64, error) {
	query := `
		SELECT DISTINCT ON (symbol) symbol, price_usd
		FROM price_cache
		WHERE updated_at > NOW() - INTERVAL '5 minutes'
		ORDER BY symbol, updated_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	prices := make(map[string]float64)
	for rows.Next() {
		var symbol string
		var price decimal.Decimal

		if err := rows.Scan(&symbol, &price); err != nil {
			continue
		}

		priceFloat, _ := price.Float64()
		prices[symbol] = priceFloat
	}

	return prices, nil
}
