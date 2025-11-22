package price

import "context"

// PriceProvider provides current cryptocurrency prices
type PriceProvider interface {
	// GetPrice returns current price in USD
	GetPrice(ctx context.Context, symbol string) (float64, error)

	// GetPrices returns multiple prices at once
	GetPrices(ctx context.Context, symbols []string) (map[string]float64, error)

	// GetName returns provider name
	GetName() string
}
