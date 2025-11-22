package exchange

import (
	"context"

	"github.com/selivandex/trader-bot/pkg/models"
)

// Exchange represents unified exchange interface
type Exchange interface {
	// GetName returns exchange name
	GetName() string

	// Market Data
	FetchTicker(ctx context.Context, symbol string) (*models.Ticker, error)
	FetchOHLCV(ctx context.Context, symbol, timeframe string, limit int) ([]models.Candle, error)
	FetchOrderBook(ctx context.Context, symbol string, depth int) (*models.OrderBook, error)
	FetchFundingRate(ctx context.Context, symbol string) (float64, error)
	FetchOpenInterest(ctx context.Context, symbol string) (float64, error)

	// Account
	FetchBalance(ctx context.Context) (*models.Balance, error)
	FetchOpenPositions(ctx context.Context) ([]models.Position, error)
	FetchPosition(ctx context.Context, symbol string) (*models.Position, error)

	// Trading
	CreateOrder(ctx context.Context, symbol string, orderType models.OrderType, side models.OrderSide, amount, price float64) (*models.Order, error)
	CancelOrder(ctx context.Context, orderID, symbol string) error
	FetchOrder(ctx context.Context, orderID, symbol string) (*models.Order, error)
	FetchOpenOrders(ctx context.Context, symbol string) ([]models.Order, error)

	// Futures specific
	SetLeverage(ctx context.Context, symbol string, leverage int) error
	SetMarginMode(ctx context.Context, symbol string, marginMode string) error

	// Close connection
	Close() error
}

// Factory creates exchange instances
type Factory struct {
	exchanges map[string]Exchange
}

// NewFactory creates new exchange factory
func NewFactory() *Factory {
	return &Factory{
		exchanges: make(map[string]Exchange),
	}
}

// Register registers exchange instance
func (f *Factory) Register(name string, exchange Exchange) {
	f.exchanges[name] = exchange
}

// Get returns exchange by name
func (f *Factory) Get(name string) (Exchange, bool) {
	ex, ok := f.exchanges[name]
	return ex, ok
}

// GetAll returns all registered exchanges
func (f *Factory) GetAll() map[string]Exchange {
	return f.exchanges
}

// Close closes all exchanges
func (f *Factory) Close() error {
	for _, ex := range f.exchanges {
		if err := ex.Close(); err != nil {
			return err
		}
	}
	return nil
}
