package exchange

import (
	"context"
	"fmt"
	"time"

	ccxt "github.com/ccxt/ccxt/go/v4"
	"go.uber.org/zap"

	"github.com/alexanderselivanov/trader/internal/adapters/config"
	"github.com/alexanderselivanov/trader/pkg/logger"
	"github.com/alexanderselivanov/trader/pkg/models"
)

// BinanceAdapter wraps CCXT Binance exchange
type BinanceAdapter struct {
	exchange *ccxt.Binance
	config   *config.ExchangeConfig
}

// NewBinanceAdapter creates new Binance adapter
func NewBinanceAdapter(cfg *config.ExchangeConfig) (*BinanceAdapter, error) {
	options := map[string]interface{}{
		"apiKey": cfg.APIKey,
		"secret": cfg.Secret,
	}

	if cfg.Testnet {
		options["testnet"] = true
	}

	exchange := ccxt.NewBinance(options)

	// Set default options
	exchange.SetOption("defaultType", "future") // Use futures by default
	exchange.SetOption("adjustForTimeDifference", true)

	// Load markets
	if err := exchange.LoadMarkets(); err != nil {
		return nil, fmt.Errorf("failed to load Binance markets: %w", err)
	}

	logger.Info("Binance adapter initialized",
		zap.Bool("testnet", cfg.Testnet),
		zap.Int("markets_count", len(exchange.Markets)),
	)

	return &BinanceAdapter{
		exchange: exchange,
		config:   cfg,
	}, nil
}

func (b *BinanceAdapter) GetName() string {
	return "binance"
}

func (b *BinanceAdapter) FetchTicker(ctx context.Context, symbol string) (*models.Ticker, error) {
	ticker, err := b.exchange.FetchTicker(symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch ticker: %w", err)
	}

	return &models.Ticker{
		Symbol:    symbol,
		Last:      models.NewDecimal(*ticker.Last),
		Bid:       models.NewDecimal(*ticker.Bid),
		Ask:       models.NewDecimal(*ticker.Ask),
		High24h:   models.NewDecimal(*ticker.High),
		Low24h:    models.NewDecimal(*ticker.Low),
		Volume24h: models.NewDecimal(*ticker.BaseVolume),
		Change24h: models.NewDecimal(*ticker.Percentage),
		Timestamp: time.UnixMilli(*ticker.Timestamp),
	}, nil
}

func (b *BinanceAdapter) FetchOHLCV(ctx context.Context, symbol, timeframe string, limit int) ([]models.Candle, error) {
	ohlcv, err := b.exchange.FetchOHLCV(
		symbol,
		ccxt.WithFetchOHLCVTimeframe(timeframe),
		ccxt.WithFetchOHLCVLimit(limit),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch OHLCV: %w", err)
	}

	candles := make([]models.Candle, len(ohlcv))
	for i, bar := range ohlcv {
		candles[i] = models.Candle{
			Timestamp: time.UnixMilli(int64(bar[0])),
			Open:      models.NewDecimal(bar[1]),
			High:      models.NewDecimal(bar[2]),
			Low:       models.NewDecimal(bar[3]),
			Close:     models.NewDecimal(bar[4]),
			Volume:    models.NewDecimal(bar[5]),
		}
	}

	return candles, nil
}

func (b *BinanceAdapter) FetchOrderBook(ctx context.Context, symbol string, depth int) (*models.OrderBook, error) {
	orderBook, err := b.exchange.FetchOrderBook(symbol, ccxt.WithFetchOrderBookLimit(depth))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch order book: %w", err)
	}

	bids := make([]models.OrderBookItem, len(orderBook.Bids))
	for i, bid := range orderBook.Bids {
		bids[i] = models.OrderBookItem{
			Price:  models.NewDecimal(bid[0]),
			Amount: models.NewDecimal(bid[1]),
		}
	}

	asks := make([]models.OrderBookItem, len(orderBook.Asks))
	for i, ask := range orderBook.Asks {
		asks[i] = models.OrderBookItem{
			Price:  models.NewDecimal(ask[0]),
			Amount: models.NewDecimal(ask[1]),
		}
	}

	return &models.OrderBook{
		Symbol:    symbol,
		Bids:      bids,
		Asks:      asks,
		Timestamp: time.UnixMilli(*orderBook.Timestamp),
	}, nil
}

func (b *BinanceAdapter) FetchFundingRate(ctx context.Context, symbol string) (float64, error) {
	// Binance API call for funding rate
	result, err := b.exchange.PublicGetPremiumIndex(map[string]interface{}{
		"symbol": b.convertSymbolToBinance(symbol),
	})
	if err != nil {
		return 0, fmt.Errorf("failed to fetch funding rate: %w", err)
	}

	// Extract funding rate from result
	if rate, ok := result["lastFundingRate"].(float64); ok {
		return rate, nil
	}

	return 0, fmt.Errorf("funding rate not found in response")
}

func (b *BinanceAdapter) FetchOpenInterest(ctx context.Context, symbol string) (float64, error) {
	result, err := b.exchange.PublicGetOpenInterest(map[string]interface{}{
		"symbol": b.convertSymbolToBinance(symbol),
	})
	if err != nil {
		return 0, fmt.Errorf("failed to fetch open interest: %w", err)
	}

	if oi, ok := result["openInterest"].(float64); ok {
		return oi, nil
	}

	return 0, fmt.Errorf("open interest not found in response")
}

func (b *BinanceAdapter) FetchBalance(ctx context.Context) (*models.Balance, error) {
	balance, err := b.exchange.FetchBalance()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch balance: %w", err)
	}

	currencies := make(map[string]models.CurrencyBalance)
	for currency, bal := range balance {
		if balMap, ok := bal.(map[string]interface{}); ok {
			currencies[currency] = models.CurrencyBalance{
				Currency: currency,
				Total:    models.NewDecimal(getFloat(balMap, "total")),
				Free:     models.NewDecimal(getFloat(balMap, "free")),
				Used:     models.NewDecimal(getFloat(balMap, "used")),
			}
		}
	}

	totalBalance := getFloat(balance, "total")
	freeBalance := getFloat(balance, "free")
	usedBalance := getFloat(balance, "used")

	return &models.Balance{
		Total:      models.NewDecimal(totalBalance),
		Free:       models.NewDecimal(freeBalance),
		Used:       models.NewDecimal(usedBalance),
		Currencies: currencies,
	}, nil
}

func (b *BinanceAdapter) FetchOpenPositions(ctx context.Context) ([]models.Position, error) {
	positions, err := b.exchange.FetchPositions()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch positions: %w", err)
	}

	result := make([]models.Position, 0)
	for _, pos := range positions {
		// Only include open positions
		contracts := getFloat(pos, "contracts")
		if contracts == 0 {
			continue
		}

		side := models.PositionLong
		if contracts < 0 {
			side = models.PositionShort
		}

		result = append(result, models.Position{
			Symbol:           getString(pos, "symbol"),
			Side:             side,
			Size:             models.NewDecimal(abs(contracts)),
			EntryPrice:       models.NewDecimal(getFloat(pos, "entryPrice")),
			CurrentPrice:     models.NewDecimal(getFloat(pos, "markPrice")),
			Leverage:         int(getFloat(pos, "leverage")),
			UnrealizedPnL:    models.NewDecimal(getFloat(pos, "unrealizedPnl")),
			LiquidationPrice: models.NewDecimal(getFloat(pos, "liquidationPrice")),
			Margin:           models.NewDecimal(getFloat(pos, "collateral")),
			Timestamp:        time.UnixMilli(int64(getFloat(pos, "timestamp"))),
		})
	}

	return result, nil
}

func (b *BinanceAdapter) FetchPosition(ctx context.Context, symbol string) (*models.Position, error) {
	positions, err := b.FetchOpenPositions(ctx)
	if err != nil {
		return nil, err
	}

	for _, pos := range positions {
		if pos.Symbol == symbol {
			return &pos, nil
		}
	}

	return nil, fmt.Errorf("position not found for symbol: %s", symbol)
}

func (b *BinanceAdapter) CreateOrder(ctx context.Context, symbol string, orderType models.OrderType, side models.OrderSide, amount, price float64) (*models.Order, error) {
	var sideStr string
	if side == models.SideBuy {
		sideStr = "buy"
	} else {
		sideStr = "sell"
	}

	var order *ccxt.Order
	var err error

	if orderType == models.TypeMarket {
		order, err = b.exchange.CreateOrder(
			symbol,
			"market",
			sideStr,
			amount,
		)
	} else {
		order, err = b.exchange.CreateOrder(
			symbol,
			"limit",
			sideStr,
			amount,
			ccxt.WithCreateOrderPrice(price),
		)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create order: %w", err)
	}

	return &models.Order{
		ID:          *order.Id,
		Symbol:      *order.Symbol,
		Type:        orderType,
		Side:        side,
		Price:       models.NewDecimal(*order.Price),
		Amount:      models.NewDecimal(*order.Amount),
		Filled:      models.NewDecimal(*order.Filled),
		Remaining:   models.NewDecimal(*order.Remaining),
		Status:      *order.Status,
		Fee:         models.NewDecimal(getFloat(order.Fee, "cost")),
		FeeCurrency: getString(order.Fee, "currency"),
		Timestamp:   time.UnixMilli(*order.Timestamp),
	}, nil
}

func (b *BinanceAdapter) CancelOrder(ctx context.Context, orderID, symbol string) error {
	_, err := b.exchange.CancelOrder(orderID, symbol)
	if err != nil {
		return fmt.Errorf("failed to cancel order: %w", err)
	}
	return nil
}

func (b *BinanceAdapter) FetchOrder(ctx context.Context, orderID, symbol string) (*models.Order, error) {
	order, err := b.exchange.FetchOrder(orderID, symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch order: %w", err)
	}

	var orderType models.OrderType
	if *order.Type == "market" {
		orderType = models.TypeMarket
	} else {
		orderType = models.TypeLimit
	}

	var side models.OrderSide
	if *order.Side == "buy" {
		side = models.SideBuy
	} else {
		side = models.SideSell
	}

	return &models.Order{
		ID:          *order.Id,
		Symbol:      *order.Symbol,
		Type:        orderType,
		Side:        side,
		Price:       models.NewDecimal(*order.Price),
		Amount:      models.NewDecimal(*order.Amount),
		Filled:      models.NewDecimal(*order.Filled),
		Remaining:   models.NewDecimal(*order.Remaining),
		Status:      *order.Status,
		Fee:         models.NewDecimal(getFloat(order.Fee, "cost")),
		FeeCurrency: getString(order.Fee, "currency"),
		Timestamp:   time.UnixMilli(*order.Timestamp),
	}, nil
}

func (b *BinanceAdapter) FetchOpenOrders(ctx context.Context, symbol string) ([]models.Order, error) {
	orders, err := b.exchange.FetchOpenOrders(symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch open orders: %w", err)
	}

	result := make([]models.Order, len(orders))
	for i, order := range orders {
		var orderType models.OrderType
		if *order.Type == "market" {
			orderType = models.TypeMarket
		} else {
			orderType = models.TypeLimit
		}

		var side models.OrderSide
		if *order.Side == "buy" {
			side = models.SideBuy
		} else {
			side = models.SideSell
		}

		result[i] = models.Order{
			ID:          *order.Id,
			Symbol:      *order.Symbol,
			Type:        orderType,
			Side:        side,
			Price:       models.NewDecimal(*order.Price),
			Amount:      models.NewDecimal(*order.Amount),
			Filled:      models.NewDecimal(*order.Filled),
			Remaining:   models.NewDecimal(*order.Remaining),
			Status:      *order.Status,
			Fee:         models.NewDecimal(getFloat(order.Fee, "cost")),
			FeeCurrency: getString(order.Fee, "currency")),
			Timestamp:   time.UnixMilli(*order.Timestamp),
		}
	}

	return result, nil
}

func (b *BinanceAdapter) SetLeverage(ctx context.Context, symbol string, leverage int) error {
	_, err := b.exchange.SetLeverage(leverage, symbol)
	if err != nil {
		return fmt.Errorf("failed to set leverage: %w", err)
	}
	return nil
}

func (b *BinanceAdapter) SetMarginMode(ctx context.Context, symbol string, marginMode string) error {
	_, err := b.exchange.SetMarginMode(marginMode, symbol)
	if err != nil {
		return fmt.Errorf("failed to set margin mode: %w", err)
	}
	return nil
}

func (b *BinanceAdapter) Close() error {
	// CCXT doesn't require explicit connection closing
	return nil
}

// Helper functions
func (b *BinanceAdapter) convertSymbolToBinance(symbol string) string {
	// Convert BTC/USDT to BTCUSDT
	return symbol[:3] + symbol[4:]
}

func getFloat(m interface{}, key string) float64 {
	if m == nil {
		return 0
	}
	if mmap, ok := m.(map[string]interface{}); ok {
		if val, ok := mmap[key]; ok {
			if fval, ok := val.(float64); ok {
				return fval
			}
		}
	}
	return 0
}

func getString(m interface{}, key string) string {
	if m == nil {
		return ""
	}
	if mmap, ok := m.(map[string]interface{}); ok {
		if val, ok := mmap[key]; ok {
			if sval, ok := val.(string); ok {
				return sval
			}
		}
	}
	return ""
}

