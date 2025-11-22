package exchange

import (
	"context"
	"fmt"
	"strings"
	"time"

	ccxt "github.com/ccxt/ccxt/go/v4"
	"go.uber.org/zap"

	"github.com/selivandex/trader-bot/internal/adapters/config"
	"github.com/selivandex/trader-bot/pkg/logger"
	"github.com/selivandex/trader-bot/pkg/models"
)

// BinanceAdapter wraps CCXT Binance exchange
type BinanceAdapter struct {
	exchange *ccxt.Binance
	config   *config.ExchangeConfig
}

// NewBinanceAdapter creates new Binance adapter
// API keys come from UserExchange (per-user), config is for global settings (URLs)
func NewBinanceAdapter(apiKey, apiSecret string, testnet bool, cfg *config.ExchangeConfig) (*BinanceAdapter, error) {
	options := map[string]interface{}{
		"apiKey": apiKey,
		"secret": apiSecret,
	}

	// Set API URLs from config (supports custom URLs per exchange)
	urls := cfg.GetAPIURLs(
		testnet,
		BinanceTestnetPublic,
		BinanceTestnetPrivate,
		BinanceMainnetPublic,
		BinanceMainnetPrivate,
	)
	options["urls"] = urls

	// Set default options for futures trading
	options["defaultType"] = "future" // Use futures by default
	options["options"] = map[string]interface{}{
		"defaultType":                        "future",
		"adjustForTimeDifference":            true,
		"recvWindow":                         10000,
		"timeDifference":                     0,
		"warnOnFetchOpenOrdersWithoutSymbol": false,
	}

	exchange := ccxt.NewBinance(options)

	// Load markets
	_, err := exchange.LoadMarkets()
	if err != nil {
		return nil, fmt.Errorf("failed to load Binance markets: %w", err)
	}

	logger.Info("Binance adapter initialized",
		zap.Bool("testnet", testnet),
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
		Last:      models.NewDecimal(safeFloat(ticker.Last)),
		Bid:       models.NewDecimal(safeFloat(ticker.Bid)),
		Ask:       models.NewDecimal(safeFloat(ticker.Ask)),
		High24h:   models.NewDecimal(safeFloat(ticker.High)),
		Low24h:    models.NewDecimal(safeFloat(ticker.Low)),
		Volume24h: models.NewDecimal(safeFloat(ticker.BaseVolume)),
		Change24h: models.NewDecimal(safeFloat(ticker.Percentage)),
		Timestamp: time.UnixMilli(safeInt64(ticker.Timestamp)),
	}, nil
}

func (b *BinanceAdapter) FetchOHLCV(ctx context.Context, symbol, timeframe string, limit int) ([]models.Candle, error) {
	ohlcv, err := b.exchange.FetchOHLCV(symbol,
		ccxt.WithFetchOHLCVTimeframe(timeframe),
		ccxt.WithFetchOHLCVLimit(int64(limit)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch OHLCV: %w", err)
	}

	candles := make([]models.Candle, len(ohlcv))
	for i, bar := range ohlcv {
		candles[i] = models.Candle{
			Timestamp: time.UnixMilli(bar.Timestamp),
			Open:      models.NewDecimal(bar.Open),
			High:      models.NewDecimal(bar.High),
			Low:       models.NewDecimal(bar.Low),
			Close:     models.NewDecimal(bar.Close),
			Volume:    models.NewDecimal(bar.Volume),
		}
	}

	return candles, nil
}

func (b *BinanceAdapter) FetchOrderBook(ctx context.Context, symbol string, depth int) (*models.OrderBook, error) {
	orderBook, err := b.exchange.FetchOrderBook(symbol,
		ccxt.WithFetchOrderBookLimit(int64(depth)),
	)
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
		Timestamp: time.UnixMilli(safeInt64(orderBook.Timestamp)),
	}, nil
}

func (b *BinanceAdapter) FetchFundingRate(ctx context.Context, symbol string) (float64, error) {
	// Try specialized method first
	fundingRate, err := b.exchange.FetchFundingRate(symbol)
	if err == nil && fundingRate.FundingRate != nil {
		rate := safeFloat(fundingRate.FundingRate)
		logger.Debug("Fetched funding rate",
			zap.String("symbol", symbol),
			zap.Float64("rate", rate),
		)
		return rate, nil
	}

	// Fallback to ticker parsing
	ticker, errTicker := b.exchange.FetchTicker(symbol)
	if errTicker != nil {
		return 0, fmt.Errorf("failed to fetch funding rate: %w", errTicker)
	}

	// CCXT v4 includes funding rate in ticker info - try multiple field names
	if ticker.Info != nil {
		// Try float64 lastFundingRate (Binance uses this)
		if rate, ok := ticker.Info["lastFundingRate"].(float64); ok {
			return rate, nil
		}
		// Try float64 fundingRate
		if rate, ok := ticker.Info["fundingRate"].(float64); ok {
			return rate, nil
		}
		// Try string lastFundingRate and convert
		if rateStr, ok := ticker.Info["lastFundingRate"].(string); ok {
			var rate float64
			if _, err := fmt.Sscanf(rateStr, "%f", &rate); err == nil {
				return rate, nil
			}
		}
	}

	logger.Warn("Funding rate not found, returning 0",
		zap.String("symbol", symbol),
	)
	return 0, nil
}

func (b *BinanceAdapter) FetchOpenInterest(ctx context.Context, symbol string) (float64, error) {
	// Get open interest from ticker
	ticker, err := b.exchange.FetchTicker(symbol)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch ticker for open interest: %w", err)
	}

	// CCXT v4 includes open interest in ticker info - try multiple field names
	if ticker.Info != nil {
		// Try float64 openInterest
		if oi, ok := ticker.Info["openInterest"].(float64); ok {
			return oi, nil
		}
		// Try float64 open_interest
		if oi, ok := ticker.Info["open_interest"].(float64); ok {
			return oi, nil
		}
		// Try string openInterest and convert
		if oiStr, ok := ticker.Info["openInterest"].(string); ok {
			var oi float64
			if _, err := fmt.Sscanf(oiStr, "%f", &oi); err == nil {
				return oi, nil
			}
		}
	}

	logger.Warn("Open interest not found, returning 0",
		zap.String("symbol", symbol),
	)
	return 0, nil
}

func (b *BinanceAdapter) FetchBalance(ctx context.Context) (*models.Balance, error) {
	balances, err := b.exchange.FetchBalance()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch balance: %w", err)
	}

	// CCXT v4 Balances - extract USDT balance
	usdtTotal := safeFloat(balances.Total["USDT"])
	usdtFree := safeFloat(balances.Free["USDT"])
	usdtUsed := safeFloat(balances.Used["USDT"])

	currencies := map[string]models.CurrencyBalance{
		"USDT": {
			Currency: "USDT",
			Total:    models.NewDecimal(usdtTotal),
			Free:     models.NewDecimal(usdtFree),
			Used:     models.NewDecimal(usdtUsed),
		},
	}

	return &models.Balance{
		Total:      models.NewDecimal(usdtTotal),
		Free:       models.NewDecimal(usdtFree),
		Used:       models.NewDecimal(usdtUsed),
		Currencies: currencies,
	}, nil
}

func (b *BinanceAdapter) FetchPosition(ctx context.Context, symbol string) (*models.Position, error) {
	positions, err := b.exchange.FetchPositions(ccxt.WithFetchPositionsSymbols([]string{symbol}))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch positions: %w", err)
	}

	for _, pos := range positions {
		posSymbol := safeStringPtr(pos.Symbol)
		if posSymbol != symbol {
			continue
		}

		contracts := safeFloat(pos.Contracts)
		if contracts == 0 {
			// No position
			return &models.Position{
				Symbol: symbol,
				Side:   models.PositionNone,
			}, nil
		}

		side := models.PositionLong
		posSide := safeStringPtr(pos.Side)
		if posSide == "short" || contracts < 0 {
			side = models.PositionShort
		}

		return &models.Position{
			Symbol:           symbol,
			Side:             side,
			Size:             models.NewDecimal(absFloat(contracts)),
			EntryPrice:       models.NewDecimal(safeFloat(pos.EntryPrice)),
			CurrentPrice:     models.NewDecimal(safeFloat(pos.MarkPrice)),
			Leverage:         int(safeFloat(pos.Leverage)),
			UnrealizedPnL:    models.NewDecimal(safeFloat(pos.UnrealizedPnl)),
			LiquidationPrice: models.NewDecimal(safeFloat(pos.LiquidationPrice)),
			Margin:           models.NewDecimal(safeFloat(pos.Collateral)),
			Timestamp:        time.Now(),
		}, nil
	}

	// No position found
	return &models.Position{
		Symbol: symbol,
		Side:   models.PositionNone,
	}, nil
}

func (b *BinanceAdapter) CreateOrder(ctx context.Context, symbol string, orderType models.OrderType, side models.OrderSide, amount, price float64) (*models.Order, error) {
	sideStr := "buy"
	if side == models.SideSell {
		sideStr = "sell"
	}

	var order ccxt.Order
	var err error

	logger.Info("Creating order",
		zap.String("symbol", symbol),
		zap.String("type", string(orderType)),
		zap.String("side", sideStr),
		zap.Float64("amount", amount),
		zap.Float64("price", price),
	)

	switch orderType {
	case models.TypeMarket:
		order, err = b.exchange.CreateMarketOrder(symbol, sideStr, amount)

	case models.TypeLimit:
		order, err = b.exchange.CreateLimitOrder(symbol, sideStr, amount, price)

	case models.TypeStopMarket:
		// Stop-loss market orders need stopPrice in params
		params := map[string]interface{}{
			"stopPrice": price,
			"type":      "STOP_MARKET",
		}
		order, err = b.exchange.CreateOrder(symbol, "market", sideStr, amount,
			ccxt.WithCreateOrderParams(params))

	case models.TypeStopLimit:
		// Stop-loss limit orders need stopPrice and limit price
		params := map[string]interface{}{
			"stopPrice": price,
			"type":      "STOP",
		}
		order, err = b.exchange.CreateOrder(symbol, "limit", sideStr, amount,
			ccxt.WithCreateOrderPrice(price),
			ccxt.WithCreateOrderParams(params))

	case models.TypeTakeProfitMarket:
		// Take-profit market orders need stopPrice in params
		params := map[string]interface{}{
			"stopPrice": price,
			"type":      "TAKE_PROFIT_MARKET",
		}
		order, err = b.exchange.CreateOrder(symbol, "market", sideStr, amount,
			ccxt.WithCreateOrderParams(params))

	case models.TypeTakeProfitLimit:
		// Take-profit limit orders need stopPrice and limit price
		params := map[string]interface{}{
			"stopPrice": price,
			"type":      "TAKE_PROFIT",
		}
		order, err = b.exchange.CreateOrder(symbol, "limit", sideStr, amount,
			ccxt.WithCreateOrderPrice(price),
			ccxt.WithCreateOrderParams(params))

	default:
		return nil, fmt.Errorf("unsupported order type: %s", orderType)
	}

	if err != nil {
		logger.Error("Failed to create order",
			zap.Error(err),
			zap.String("symbol", symbol),
			zap.String("type", string(orderType)),
		)
		return nil, fmt.Errorf("failed to create order: %w", err)
	}

	logger.Info("Order created successfully",
		zap.String("order_id", safeStringPtr(order.Id)),
		zap.String("status", safeStringPtr(order.Status)),
	)

	return &models.Order{
		ID:        safeStringPtr(order.Id),
		Symbol:    safeStringPtr(order.Symbol),
		Type:      orderType,
		Side:      side,
		Price:     models.NewDecimal(safeFloat(order.Price)),
		Amount:    models.NewDecimal(safeFloat(order.Amount)),
		Filled:    models.NewDecimal(safeFloat(order.Filled)),
		Remaining: models.NewDecimal(safeFloat(order.Remaining)),
		Status:    safeStringPtr(order.Status),
		Timestamp: time.UnixMilli(safeInt64(order.Timestamp)),
	}, nil
}

func (b *BinanceAdapter) SetLeverage(ctx context.Context, symbol string, leverage int) error {
	logger.Info("Setting leverage",
		zap.String("symbol", symbol),
		zap.Int("leverage", leverage),
	)

	_, err := b.exchange.SetLeverage(int64(leverage),
		ccxt.WithSetLeverageSymbol(symbol))
	if err != nil {
		logger.Error("Failed to set leverage",
			zap.Error(err),
			zap.String("symbol", symbol),
			zap.Int("leverage", leverage),
		)
		return fmt.Errorf("failed to set leverage: %w", err)
	}

	logger.Info("Leverage set successfully",
		zap.String("symbol", symbol),
		zap.Int("leverage", leverage),
	)
	return nil
}

func (b *BinanceAdapter) CancelOrder(ctx context.Context, orderID, symbol string) error {
	_, err := b.exchange.CancelOrder(orderID,
		ccxt.WithCancelOrderSymbol(symbol))
	if err != nil {
		return fmt.Errorf("failed to cancel order: %w", err)
	}
	return nil
}

func (b *BinanceAdapter) FetchOpenOrders(ctx context.Context, symbol string) ([]models.Order, error) {
	orders, err := b.exchange.FetchOpenOrders(
		ccxt.WithFetchOpenOrdersSymbol(symbol))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch open orders: %w", err)
	}

	result := make([]models.Order, len(orders))
	for i, order := range orders {
		orderSide := models.SideBuy
		if safeStringPtr(order.Side) == "sell" {
			orderSide = models.SideSell
		}

		result[i] = models.Order{
			ID:        safeStringPtr(order.Id),
			Symbol:    safeStringPtr(order.Symbol),
			Side:      orderSide,
			Price:     models.NewDecimal(safeFloat(order.Price)),
			Amount:    models.NewDecimal(safeFloat(order.Amount)),
			Filled:    models.NewDecimal(safeFloat(order.Filled)),
			Remaining: models.NewDecimal(safeFloat(order.Remaining)),
			Status:    safeStringPtr(order.Status),
			Timestamp: time.UnixMilli(safeInt64(order.Timestamp)),
		}
	}

	return result, nil
}

func (b *BinanceAdapter) FetchOrder(ctx context.Context, orderID, symbol string) (*models.Order, error) {
	order, err := b.exchange.FetchOrder(orderID,
		ccxt.WithFetchOrderSymbol(symbol))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch order: %w", err)
	}

	orderSide := models.SideBuy
	if safeStringPtr(order.Side) == "sell" {
		orderSide = models.SideSell
	}

	return &models.Order{
		ID:        safeStringPtr(order.Id),
		Symbol:    safeStringPtr(order.Symbol),
		Side:      orderSide,
		Price:     models.NewDecimal(safeFloat(order.Price)),
		Amount:    models.NewDecimal(safeFloat(order.Amount)),
		Filled:    models.NewDecimal(safeFloat(order.Filled)),
		Remaining: models.NewDecimal(safeFloat(order.Remaining)),
		Status:    safeStringPtr(order.Status),
		Timestamp: time.UnixMilli(safeInt64(order.Timestamp)),
	}, nil
}

func (b *BinanceAdapter) FetchOpenPositions(ctx context.Context) ([]models.Position, error) {
	positions, err := b.exchange.FetchPositions()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch open positions: %w", err)
	}

	result := []models.Position{}
	for _, pos := range positions {
		contracts := safeFloat(pos.Contracts)
		if contracts == 0 {
			continue // Skip empty positions
		}

		side := models.PositionLong
		if safeStringPtr(pos.Side) == "short" || contracts < 0 {
			side = models.PositionShort
		}

		result = append(result, models.Position{
			Symbol:           safeStringPtr(pos.Symbol),
			Side:             side,
			Size:             models.NewDecimal(absFloat(contracts)),
			EntryPrice:       models.NewDecimal(safeFloat(pos.EntryPrice)),
			CurrentPrice:     models.NewDecimal(safeFloat(pos.MarkPrice)),
			Leverage:         int(safeFloat(pos.Leverage)),
			UnrealizedPnL:    models.NewDecimal(safeFloat(pos.UnrealizedPnl)),
			LiquidationPrice: models.NewDecimal(safeFloat(pos.LiquidationPrice)),
			Margin:           models.NewDecimal(safeFloat(pos.Collateral)),
			Timestamp:        time.Now(),
		})
	}

	return result, nil
}

func (b *BinanceAdapter) SetMarginMode(ctx context.Context, symbol string, marginMode string) error {
	// Validate margin mode - Binance accepts "isolated" or "cross" (lowercase)
	marginMode = strings.ToLower(marginMode)
	if marginMode != "isolated" && marginMode != "cross" {
		return fmt.Errorf("invalid margin mode: %s (must be 'isolated' or 'cross')", marginMode)
	}

	logger.Info("Setting margin mode",
		zap.String("symbol", symbol),
		zap.String("mode", marginMode),
	)

	_, err := b.exchange.SetMarginMode(marginMode,
		ccxt.WithSetMarginModeSymbol(symbol))
	if err != nil {
		logger.Error("Failed to set margin mode",
			zap.Error(err),
			zap.String("symbol", symbol),
			zap.String("mode", marginMode),
		)
		return fmt.Errorf("failed to set margin mode: %w", err)
	}

	logger.Info("Margin mode set successfully",
		zap.String("symbol", symbol),
		zap.String("mode", marginMode),
	)
	return nil
}

func (b *BinanceAdapter) Close() error {
	// CCXT doesn't require explicit connection closing
	return nil
}
