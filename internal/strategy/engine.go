package strategy

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/alexanderselivanov/trader/internal/adapters/ai"
	"github.com/alexanderselivanov/trader/internal/adapters/config"
	"github.com/alexanderselivanov/trader/internal/adapters/exchange"
	"github.com/alexanderselivanov/trader/internal/adapters/news"
	"github.com/alexanderselivanov/trader/internal/adapters/telegram"
	"github.com/alexanderselivanov/trader/internal/indicators"
	"github.com/alexanderselivanov/trader/internal/portfolio"
	"github.com/alexanderselivanov/trader/internal/risk"
	"github.com/alexanderselivanov/trader/pkg/logger"
	"github.com/alexanderselivanov/trader/pkg/models"
)

// Engine represents trading strategy engine
type Engine struct {
	cfg              *config.Config
	exchange         exchange.Exchange
	aiEnsemble       *ai.Ensemble
	newsAggregator   *news.Aggregator
	indicatorCalc    *indicators.Calculator
	riskManager      *RiskManager
	portfolio        *portfolio.Tracker
	telegram         *telegram.Bot
	symbol           string
	decisionInterval time.Duration
	isRunning        bool
}

// RiskManager aggregates risk components
type RiskManager struct {
	CircuitBreaker *risk.CircuitBreaker
	PositionSizer  *risk.PositionSizer
	Validator      *risk.Validator
}

// NewEngine creates new trading engine
func NewEngine(
	cfg *config.Config,
	ex exchange.Exchange,
	aiEnsemble *ai.Ensemble,
	newsAggregator *news.Aggregator,
	riskManager *RiskManager,
	portfolioTracker *portfolio.Tracker,
	telegramBot *telegram.Bot,
) *Engine {
	return &Engine{
		cfg:              cfg,
		exchange:         ex,
		aiEnsemble:       aiEnsemble,
		newsAggregator:   newsAggregator,
		indicatorCalc:    indicators.NewCalculator(),
		riskManager:      riskManager,
		portfolio:        portfolioTracker,
		telegram:         telegramBot,
		symbol:           cfg.Trading.Symbol,
		decisionInterval: cfg.AI.DecisionInterval,
		isRunning:        false,
	}
}

// Start starts the trading engine
func (e *Engine) Start(ctx context.Context) error {
	e.isRunning = true
	
	logger.Info("trading engine started",
		zap.String("symbol", e.symbol),
		zap.Duration("interval", e.decisionInterval),
	)
	
	ticker := time.NewTicker(e.decisionInterval)
	defer ticker.Stop()
	
	// Run immediately on start
	if err := e.executeTradingCycle(ctx); err != nil {
		logger.Error("trading cycle failed", zap.Error(err))
		if e.telegram != nil {
			e.telegram.AlertError(fmt.Sprintf("Trading cycle failed: %v", err))
		}
	}
	
	// Then run on interval
	for {
		select {
		case <-ctx.Done():
			e.isRunning = false
			return ctx.Err()
		case <-ticker.C:
			if !e.isRunning {
				continue
			}
			
			if err := e.executeTradingCycle(ctx); err != nil {
				logger.Error("trading cycle failed", zap.Error(err))
				if e.telegram != nil {
					e.telegram.AlertError(fmt.Sprintf("Trading cycle failed: %v", err))
				}
			}
		}
	}
}

// executeTradingCycle runs one complete trading cycle
func (e *Engine) executeTradingCycle(ctx context.Context) error {
	logger.Info("executing trading cycle", zap.String("symbol", e.symbol))
	
	// Step 1: Check circuit breaker
	if e.riskManager.CircuitBreaker.IsOpen() {
		logger.Warn("circuit breaker is open, skipping trading cycle")
		return nil
	}
	
	// Step 2: Collect market data
	marketData, err := e.collectMarketData(ctx)
	if err != nil {
		return fmt.Errorf("failed to collect market data: %w", err)
	}
	
	// Step 3: Validate market conditions
	if err := e.riskManager.Validator.ValidateMarketConditions(marketData); err != nil {
		logger.Warn("market conditions not suitable for trading", zap.Error(err))
		return nil
	}
	
	// Step 4: Get current position and balance
	position, _ := e.exchange.FetchPosition(ctx, e.symbol)
	balance := e.portfolio.GetBalance()
	equity := e.portfolio.GetEquity()
	dailyPnL := e.portfolio.GetDailyPnL()
	
	// Step 5: Check drawdown
	peakEquity := e.portfolio.GetPeakEquity()
	if err := e.riskManager.Validator.CheckDrawdown(equity, peakEquity, e.cfg.Risk.MaxDrawdownPercent); err != nil {
		logger.Error("max drawdown exceeded", zap.Error(err))
		if e.telegram != nil {
			e.telegram.AlertCircuitBreaker(err.Error(), int(e.cfg.Risk.CircuitBreakerCooldown.Minutes()))
		}
		return e.riskManager.CircuitBreaker.RecordTrade(-100, balance)
	}
	
	// Step 6: Build trading prompt
	prompt := e.buildTradingPrompt(marketData, position, balance, equity, dailyPnL)
	
	// Step 7: Get AI decision (ensemble)
	ensembleDecision, err := e.aiEnsemble.Analyze(ctx, prompt)
	if err != nil {
		return fmt.Errorf("AI analysis failed: %w", err)
	}
	
	// Step 8: Validate ensemble consensus
	if err := e.riskManager.Validator.ValidateEnsembleDecision(ensembleDecision); err != nil {
		logger.Info("ensemble decision rejected", zap.Error(err))
		return nil
	}
	
	decision := ensembleDecision.Consensus
	
	logger.Info("AI decision received",
		zap.String("provider", decision.Provider),
		zap.String("action", string(decision.Action)),
		zap.Int("confidence", decision.Confidence),
		zap.String("reason", decision.Reason),
	)
	
	// Notify about AI decision
	if e.telegram != nil {
		e.telegram.AlertAIDecision(decision.Provider, string(decision.Action), decision.Reason, decision.Confidence)
	}
	
	// Step 9: Validate AI decision
	if err := e.riskManager.Validator.ValidateDecision(decision, marketData); err != nil {
		logger.Warn("AI decision validation failed", zap.Error(err))
		return nil
	}
	
	// Step 10: Sanity check
	currentPrice := marketData.Ticker.Last.Float64()
	if err := e.riskManager.Validator.SanityCheck(decision, currentPrice); err != nil {
		logger.Warn("sanity check failed", zap.Error(err))
		return nil
	}
	
	// Step 11: Execute decision
	if err := e.executeDecision(ctx, decision, position, marketData); err != nil {
		logger.Error("failed to execute decision", zap.Error(err))
		if e.telegram != nil {
			e.telegram.AlertError(fmt.Sprintf("Execution failed: %v", err))
		}
		return err
	}
	
	// Step 12: Update portfolio from exchange
	if err := e.portfolio.UpdateFromExchange(ctx); err != nil {
		logger.Error("failed to update portfolio", zap.Error(err))
	}
	
	// Step 13: Check profit withdrawal
	if shouldWithdraw, amount := e.portfolio.CheckProfitWithdrawal(); shouldWithdraw {
		profitPercent := (amount / e.cfg.Trading.InitialBalance) * 100
		logger.Info("profit target reached",
			zap.Float64("amount", amount),
			zap.Float64("percent", profitPercent),
		)
		if e.telegram != nil {
			e.telegram.AlertProfitTarget(amount, profitPercent)
		}
	}
	
	return nil
}

// collectMarketData collects all necessary market data
func (e *Engine) collectMarketData(ctx context.Context) (*models.MarketData, error) {
	logger.Debug("collecting market data")
	
	// Fetch ticker
	ticker, err := e.exchange.FetchTicker(ctx, e.symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch ticker: %w", err)
	}
	
	// Fetch candles for multiple timeframes
	timeframes := []string{"5m", "15m", "1h", "4h"}
	candlesMap := make(map[string][]models.Candle)
	
	for _, tf := range timeframes {
		candles, err := e.exchange.FetchOHLCV(ctx, e.symbol, tf, 100)
		if err != nil {
			logger.Warn("failed to fetch candles", zap.String("timeframe", tf), zap.Error(err))
			continue
		}
		candlesMap[tf] = candles
	}
	
	// Calculate indicators (using 1h candles)
	var indicators *models.TechnicalIndicators
	if candles, ok := candlesMap["1h"]; ok && len(candles) >= 26 {
		indicators, err = e.indicatorCalc.Calculate(candles)
		if err != nil {
			logger.Warn("failed to calculate indicators", zap.Error(err))
		}
	}
	
	// Fetch order book
	orderBook, err := e.exchange.FetchOrderBook(ctx, e.symbol, 20)
	if err != nil {
		logger.Warn("failed to fetch order book", zap.Error(err))
	}
	
	// Fetch funding rate
	fundingRate, err := e.exchange.FetchFundingRate(ctx, e.symbol)
	if err != nil {
		logger.Warn("failed to fetch funding rate", zap.Error(err))
		fundingRate = 0
	}
	
	// Fetch open interest
	openInterest, err := e.exchange.FetchOpenInterest(ctx, e.symbol)
	if err != nil {
		logger.Warn("failed to fetch open interest", zap.Error(err))
		openInterest = 0
	}
	
	// Get news from cache (fetched by background worker)
	var newsSummary *models.NewsSummary
	if e.newsAggregator != nil {
		// News are already cached by NewsWorker, just read from cache
		newsSummary, err = e.newsAggregator.GetCachedSummary(ctx, 6*time.Hour)
		if err != nil {
			logger.Warn("failed to get cached news", zap.Error(err))
		} else if newsSummary != nil && newsSummary.TotalItems > 0 {
			logger.Info("using cached news",
				zap.String("sentiment", newsSummary.OverallSentiment),
				zap.Float64("score", newsSummary.AverageSentiment),
				zap.Int("items", newsSummary.TotalItems),
			)
		}
	}
	
	marketData := &models.MarketData{
		Symbol:       e.symbol,
		Ticker:       ticker,
		Candles:      candlesMap,
		OrderBook:    orderBook,
		FundingRate:  models.NewDecimal(fundingRate),
		OpenInterest: models.NewDecimal(openInterest),
		Indicators:   indicators,
		NewsSummary:  newsSummary,
		Timestamp:    time.Now(),
	}
	
	return marketData, nil
}

// buildTradingPrompt builds prompt for AI analysis
func (e *Engine) buildTradingPrompt(
	marketData *models.MarketData,
	position *models.Position,
	balance, equity, dailyPnL float64,
) *models.TradingPrompt {
	return &models.TradingPrompt{
		MarketData:      marketData,
		CurrentPosition: position,
		Balance:         models.NewDecimal(balance),
		Equity:          models.NewDecimal(equity),
		DailyPnL:        models.NewDecimal(dailyPnL),
		RecentTrades:    []models.Trade{}, // TODO: load from DB
	}
}

// executeDecision executes AI trading decision
func (e *Engine) executeDecision(
	ctx context.Context,
	decision *models.AIDecision,
	currentPosition *models.Position,
	marketData *models.MarketData,
) error {
	switch decision.Action {
	case models.ActionHold:
		logger.Info("AI decision: HOLD - no action taken")
		return nil
		
	case models.ActionClose:
		return e.closePosition(ctx, currentPosition)
		
	case models.ActionOpenLong:
		return e.openPosition(ctx, models.PositionLong, decision, marketData)
		
	case models.ActionOpenShort:
		return e.openPosition(ctx, models.PositionShort, decision, marketData)
		
	default:
		return fmt.Errorf("unsupported action: %s", decision.Action)
	}
}

// openPosition opens new position
func (e *Engine) openPosition(
	ctx context.Context,
	side models.PositionSide,
	decision *models.AIDecision,
	marketData *models.MarketData,
) error {
	currentPrice := marketData.Ticker.Last.Float64()
	balance := e.portfolio.GetBalance()
	
	// Calculate position size
	positionSize, err := e.riskManager.PositionSizer.CalculatePositionSize(balance, currentPrice, side)
	if err != nil {
		return fmt.Errorf("failed to calculate position size: %w", err)
	}
	
	// Validate position size
	if err := e.riskManager.PositionSizer.ValidatePositionSize(positionSize, balance); err != nil {
		return fmt.Errorf("invalid position size: %w", err)
	}
	
	// Set leverage
	if err := e.exchange.SetLeverage(ctx, e.symbol, positionSize.Leverage); err != nil {
		logger.Warn("failed to set leverage", zap.Error(err))
	}
	
	// Create market order
	var orderSide models.OrderSide
	if side == models.PositionLong {
		orderSide = models.SideBuy
	} else {
		orderSide = models.SideSell
	}
	
	order, err := e.exchange.CreateOrder(ctx, e.symbol, models.TypeMarket, orderSide, positionSize.Size, 0)
	if err != nil {
		return fmt.Errorf("failed to create order: %w", err)
	}
	
	logger.Info("position opened",
		zap.String("side", string(side)),
		zap.Float64("size", positionSize.Size),
		zap.Float64("price", order.Price.Float64()),
		zap.String("order_id", order.ID),
	)
	
	// Send telegram alert
	if e.telegram != nil {
		e.telegram.AlertTradeOpened(
			e.symbol,
			string(side),
			positionSize.Size,
			order.Price.Float64(),
			positionSize.StopLoss,
			positionSize.TakeProfit,
		)
	}
	
	return nil
}

// closePosition closes existing position
func (e *Engine) closePosition(ctx context.Context, position *models.Position) error {
	if position == nil {
		logger.Info("no position to close")
		return nil
	}
	
	// Create opposite order to close
	var orderSide models.OrderSide
	if position.Side == models.PositionLong {
		orderSide = models.SideSell
	} else {
		orderSide = models.SideBuy
	}
	
	order, err := e.exchange.CreateOrder(
		ctx,
		e.symbol,
		models.TypeMarket,
		orderSide,
		position.Size.Float64(),
		0,
	)
	if err != nil {
		return fmt.Errorf("failed to close position: %w", err)
	}
	
	pnl := position.UnrealizedPnL.Float64()
	pnlPercent := (pnl / position.Margin.Float64()) * 100
	
	logger.Info("position closed",
		zap.String("side", string(position.Side)),
		zap.Float64("exit_price", order.Price.Float64()),
		zap.Float64("pnl", pnl),
		zap.Float64("pnl_percent", pnlPercent),
	)
	
	// Record trade
	trade := &models.Trade{
		Exchange: e.exchange.GetName(),
		Symbol:   e.symbol,
		Side:     orderSide,
		Type:     models.TypeMarket,
		Amount:   position.Size,
		Price:    order.Price,
		Fee:      order.Fee,
		PnL:      position.UnrealizedPnL,
	}
	
	if err := e.portfolio.RecordTrade(ctx, trade); err != nil {
		logger.Error("failed to record trade", zap.Error(err))
	}
	
	// Update circuit breaker
	if err := e.riskManager.CircuitBreaker.RecordTrade(pnl, e.portfolio.GetBalance()); err != nil {
		logger.Error("circuit breaker triggered", zap.Error(err))
		if e.telegram != nil {
			e.telegram.AlertCircuitBreaker(err.Error(), int(e.cfg.Risk.CircuitBreakerCooldown.Minutes()))
		}
	}
	
	// Send telegram alert
	if e.telegram != nil {
		e.telegram.AlertTradeClosed(
			e.symbol,
			string(position.Side),
			order.Price.Float64(),
			pnl,
			pnlPercent,
		)
	}
	
	return nil
}

// Stop stops the trading engine
func (e *Engine) Stop() {
	e.isRunning = false
	logger.Info("trading engine stopped")
}

// IsRunning returns whether engine is running
func (e *Engine) IsRunning() bool {
	return e.isRunning
}

