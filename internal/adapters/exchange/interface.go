package exchange

import (
	"context"
	"time"

	"github.com/alexanderselivanov/trader/pkg/models"
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

// MockExchange implements Exchange interface for testing and paper trading
type MockExchange struct {
	name            string
	balance         *models.Balance
	positions       map[string]*models.Position
	orders          map[string]*models.Order
	lastPrice       float64
	priceGenerator  func() float64
}

// NewMockExchange creates new mock exchange
func NewMockExchange(name string, initialBalance float64) *MockExchange {
	return &MockExchange{
		name: name,
		balance: &models.Balance{
			Total: models.NewDecimal(initialBalance),
			Free:  models.NewDecimal(initialBalance),
			Used:  models.NewDecimal(0),
			Currencies: map[string]models.CurrencyBalance{
				"USDT": {
					Currency: "USDT",
					Total:    models.NewDecimal(initialBalance),
					Free:     models.NewDecimal(initialBalance),
					Used:     models.NewDecimal(0),
				},
			},
		},
		positions: make(map[string]*models.Position),
		orders:    make(map[string]*models.Order),
		lastPrice: 43000.0, // Default BTC price
		priceGenerator: func() float64 {
			// Simple random price movement
			return 43000.0 * (1.0 + (rand.Float64()-0.5)*0.02) // ±1% movement
		},
	}
}

func (m *MockExchange) GetName() string {
	return m.name
}

func (m *MockExchange) FetchTicker(ctx context.Context, symbol string) (*models.Ticker, error) {
	price := m.priceGenerator()
	m.lastPrice = price
	
	return &models.Ticker{
		Symbol:    symbol,
		Last:      models.NewDecimal(price),
		Bid:       models.NewDecimal(price * 0.9999),
		Ask:       models.NewDecimal(price * 1.0001),
		High24h:   models.NewDecimal(price * 1.05),
		Low24h:    models.NewDecimal(price * 0.95),
		Volume24h: models.NewDecimal(1000000),
		Change24h: models.NewDecimal(2.5),
		Timestamp: time.Now(),
	}, nil
}

func (m *MockExchange) FetchOHLCV(ctx context.Context, symbol, timeframe string, limit int) ([]models.Candle, error) {
	candles := make([]models.Candle, limit)
	basePrice := m.lastPrice
	
	for i := 0; i < limit; i++ {
		// Generate realistic candle
		open := basePrice * (1.0 + (rand.Float64()-0.5)*0.02)
		close := open * (1.0 + (rand.Float64()-0.5)*0.03)
		high := max(open, close) * (1.0 + rand.Float64()*0.01)
		low := min(open, close) * (1.0 - rand.Float64()*0.01)
		
		candles[i] = models.Candle{
			Timestamp: time.Now().Add(-time.Duration(limit-i) * parseDuration(timeframe)),
			Open:      models.NewDecimal(open),
			High:      models.NewDecimal(high),
			Low:       models.NewDecimal(low),
			Close:     models.NewDecimal(close),
			Volume:    models.NewDecimal(100.0 + rand.Float64()*50),
		}
		
		basePrice = close
	}
	
	return candles, nil
}

func (m *MockExchange) FetchOrderBook(ctx context.Context, symbol string, depth int) (*models.OrderBook, error) {
	price := m.lastPrice
	
	bids := make([]models.OrderBookItem, depth)
	asks := make([]models.OrderBookItem, depth)
	
	for i := 0; i < depth; i++ {
		bidPrice := price * (1.0 - float64(i+1)*0.0001)
		askPrice := price * (1.0 + float64(i+1)*0.0001)
		
		bids[i] = models.OrderBookItem{
			Price:  models.NewDecimal(bidPrice),
			Amount: models.NewDecimal(1.0 + rand.Float64()*5),
		}
		asks[i] = models.OrderBookItem{
			Price:  models.NewDecimal(askPrice),
			Amount: models.NewDecimal(1.0 + rand.Float64()*5),
		}
	}
	
	return &models.OrderBook{
		Symbol:    symbol,
		Bids:      bids,
		Asks:      asks,
		Timestamp: time.Now(),
	}, nil
}

func (m *MockExchange) FetchFundingRate(ctx context.Context, symbol string) (float64, error) {
	// Mock funding rate: 0.01% typical for perpetuals
	return 0.0001, nil
}

func (m *MockExchange) FetchOpenInterest(ctx context.Context, symbol string) (float64, error) {
	// Mock open interest
	return 1000000000.0, nil
}

func (m *MockExchange) FetchBalance(ctx context.Context) (*models.Balance, error) {
	return m.balance, nil
}

func (m *MockExchange) FetchOpenPositions(ctx context.Context) ([]models.Position, error) {
	positions := make([]models.Position, 0, len(m.positions))
	for _, pos := range m.positions {
		positions = append(positions, *pos)
	}
	return positions, nil
}

func (m *MockExchange) FetchPosition(ctx context.Context, symbol string) (*models.Position, error) {
	if pos, ok := m.positions[symbol]; ok {
		return pos, nil
	}
	return nil, fmt.Errorf("no position found for %s", symbol)
}

func (m *MockExchange) CreateOrder(ctx context.Context, symbol string, orderType models.OrderType, side models.OrderSide, amount, price float64) (*models.Order, error) {
	orderID := fmt.Sprintf("mock_%d", time.Now().UnixNano())
	
	// Simulate order execution
	execPrice := m.lastPrice
	if price > 0 && orderType == models.TypeLimit {
		execPrice = price
	}
	
	order := &models.Order{
		ID:          orderID,
		Symbol:      symbol,
		Type:        orderType,
		Side:        side,
		Price:       models.NewDecimal(execPrice),
		Amount:      models.NewDecimal(amount),
		Filled:      models.NewDecimal(amount),
		Remaining:   models.NewDecimal(0),
		Status:      "closed",
		Fee:         models.NewDecimal(amount * execPrice * 0.0004), // 0.04% fee
		FeeCurrency: "USDT",
		Timestamp:   time.Now(),
	}
	
	m.orders[orderID] = order
	
	// Update position
	m.updatePosition(symbol, side, amount, execPrice)
	
	return order, nil
}

func (m *MockExchange) updatePosition(symbol string, side models.OrderSide, amount, price float64) {
	pos, exists := m.positions[symbol]
	
	if !exists {
		// New position
		positionSide := models.PositionLong
		if side == models.SideSell {
			positionSide = models.PositionShort
			amount = -amount
		}
		
		m.positions[symbol] = &models.Position{
			Symbol:        symbol,
			Side:          positionSide,
			Size:          models.NewDecimal(amount),
			EntryPrice:    models.NewDecimal(price),
			CurrentPrice:  models.NewDecimal(price),
			Leverage:      3,
			UnrealizedPnL: models.NewDecimal(0),
			Margin:        models.NewDecimal(amount * price / 3),
			Timestamp:     time.Now(),
		}
		return
	}
	
	// Close or modify existing position
	currentSize := pos.Size.Float64()
	if side == models.SideSell {
		amount = -amount
	}
	
	newSize := currentSize + amount
	
	if abs(newSize) < 0.0001 {
		// Position closed
		delete(m.positions, symbol)
	} else {
		pos.Size = models.NewDecimal(newSize)
		pos.CurrentPrice = models.NewDecimal(price)
		// Recalculate unrealized PnL
		pnl := (price - pos.EntryPrice.Float64()) * newSize
		pos.UnrealizedPnL = models.NewDecimal(pnl)
	}
}

func (m *MockExchange) CancelOrder(ctx context.Context, orderID, symbol string) error {
	if _, ok := m.orders[orderID]; ok {
		delete(m.orders, orderID)
		return nil
	}
	return fmt.Errorf("order not found: %s", orderID)
}

func (m *MockExchange) FetchOrder(ctx context.Context, orderID, symbol string) (*models.Order, error) {
	if order, ok := m.orders[orderID]; ok {
		return order, nil
	}
	return nil, fmt.Errorf("order not found: %s", orderID)
}

func (m *MockExchange) FetchOpenOrders(ctx context.Context, symbol string) ([]models.Order, error) {
	orders := make([]models.Order, 0)
	for _, order := range m.orders {
		if order.Symbol == symbol && order.Status == "open" {
			orders = append(orders, *order)
		}
	}
	return orders, nil
}

func (m *MockExchange) SetLeverage(ctx context.Context, symbol string, leverage int) error {
	if pos, ok := m.positions[symbol]; ok {
		pos.Leverage = leverage
	}
	return nil
}

func (m *MockExchange) SetMarginMode(ctx context.Context, symbol string, marginMode string) error {
	// Mock implementation
	return nil
}

func (m *MockExchange) Close() error {
	return nil
}

// Helper functions
import (
	"fmt"
	"math/rand"
	"strings"
	"time"
)

func parseDuration(timeframe string) time.Duration {
	// Parse timeframe like "1m", "5m", "1h", "1d"
	switch {
	case strings.HasSuffix(timeframe, "m"):
		mins := parseInt(strings.TrimSuffix(timeframe, "m"))
		return time.Duration(mins) * time.Minute
	case strings.HasSuffix(timeframe, "h"):
		hours := parseInt(strings.TrimSuffix(timeframe, "h"))
		return time.Duration(hours) * time.Hour
	case strings.HasSuffix(timeframe, "d"):
		days := parseInt(strings.TrimSuffix(timeframe, "d"))
		return time.Duration(days) * 24 * time.Hour
	default:
		return 5 * time.Minute
	}
}

func parseInt(s string) int {
	var n int
	fmt.Sscanf(s, "%d", &n)
	if n == 0 {
		return 1
	}
	return n
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

