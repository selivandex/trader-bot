package backtest

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/alexanderselivanov/trader/internal/adapters/ai"
	"github.com/alexanderselivanov/trader/internal/adapters/exchange"
	"github.com/alexanderselivanov/trader/internal/indicators"
	"github.com/alexanderselivanov/trader/internal/risk"
	"github.com/alexanderselivanov/trader/pkg/logger"
	"github.com/alexanderselivanov/trader/pkg/models"
)

// Engine represents backtesting engine
type Engine struct {
	exchange      exchange.Exchange
	aiEnsemble    *ai.Ensemble
	indicatorCalc *indicators.Calculator
	validator     *risk.Validator
	symbol        string
	initialBalance float64
	
	// Backtest state
	balance       float64
	equity        float64
	peakEquity    float64
	position      *models.Position
	trades        []BacktestTrade
	decisions     []models.AIDecision
}

// BacktestTrade represents trade in backtest
type BacktestTrade struct {
	Timestamp  time.Time
	Symbol     string
	Side       models.OrderSide
	Size       float64
	EntryPrice float64
	ExitPrice  float64
	PnL        float64
	PnLPercent float64
	Duration   time.Duration
	Reason     string
}

// Config represents backtest configuration
type Config struct {
	Symbol         string
	StartDate      time.Time
	EndDate        time.Time
	InitialBalance float64
	Timeframe      string
	MaxLeverage    int
	StopLossPercent float64
}

// NewEngine creates new backtest engine
func NewEngine(
	ex exchange.Exchange,
	aiEnsemble *ai.Ensemble,
	cfg *Config,
) *Engine {
	return &Engine{
		exchange:       ex,
		aiEnsemble:     aiEnsemble,
		indicatorCalc:  indicators.NewCalculator(),
		validator:      risk.NewValidator(),
		symbol:         cfg.Symbol,
		initialBalance: cfg.InitialBalance,
		balance:        cfg.InitialBalance,
		equity:         cfg.InitialBalance,
		peakEquity:     cfg.InitialBalance,
		trades:         make([]BacktestTrade, 0),
		decisions:      make([]models.AIDecision, 0),
	}
}

// Run runs backtest simulation
func (e *Engine) Run(ctx context.Context, cfg *Config) (*BacktestResult, error) {
	logger.Info("starting backtest",
		zap.String("symbol", cfg.Symbol),
		zap.Time("start", cfg.StartDate),
		zap.Time("end", cfg.EndDate),
	)
	
	// Fetch historical data
	candles, err := e.fetchHistoricalData(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch historical data: %w", err)
	}
	
	logger.Info("historical data loaded",
		zap.Int("candles", len(candles)),
	)
	
	// Simulate trading for each candle
	for i := 100; i < len(candles); i++ {
		// Use previous 100 candles for indicators
		historicalCandles := candles[i-100 : i]
		currentCandle := candles[i]
		
		// Calculate indicators
		indicators, err := e.indicatorCalc.Calculate(historicalCandles)
		if err != nil {
			logger.Warn("failed to calculate indicators", zap.Error(err))
			continue
		}
		
		// Build market data
		marketData := &models.MarketData{
			Symbol: cfg.Symbol,
			Ticker: &models.Ticker{
				Symbol:    cfg.Symbol,
				Last:      currentCandle.Close,
				Timestamp: currentCandle.Timestamp,
			},
			Candles: map[string][]models.Candle{
				cfg.Timeframe: historicalCandles,
			},
			Indicators: indicators,
			Timestamp:  currentCandle.Timestamp,
		}
		
		// Build trading prompt
		prompt := &models.TradingPrompt{
			MarketData:      marketData,
			CurrentPosition: e.position,
			Balance:         models.NewDecimal(e.balance),
			Equity:          models.NewDecimal(e.equity),
			DailyPnL:        models.NewDecimal(0),
		}
		
		// Get AI decision
		ensembleDecision, err := e.aiEnsemble.Analyze(ctx, prompt)
		if err != nil {
			logger.Warn("AI analysis failed", zap.Error(err))
			continue
		}
		
		if !ensembleDecision.Agreement {
			continue // No consensus
		}
		
		decision := ensembleDecision.Consensus
		e.decisions = append(e.decisions, *decision)
		
		// Simulate trade execution
		currentPrice := currentCandle.Close.Float64()
		e.simulateDecision(decision, currentPrice, currentCandle.Timestamp)
		
		// Update position PnL if open
		if e.position != nil {
			e.updatePositionPnL(currentPrice)
		}
	}
	
	// Close any open position at end
	if e.position != nil {
		finalPrice := candles[len(candles)-1].Close.Float64()
		e.closePosition(finalPrice, candles[len(candles)-1].Timestamp, "backtest_end")
	}
	
	// Calculate metrics
	result := e.calculateMetrics(cfg)
	
	logger.Info("backtest completed",
		zap.Float64("final_equity", result.FinalEquity),
		zap.Float64("total_pnl", result.TotalPnL),
		zap.Float64("roi", result.ROI),
		zap.Int("total_trades", result.TotalTrades),
	)
	
	return result, nil
}

// fetchHistoricalData fetches historical candles
func (e *Engine) fetchHistoricalData(ctx context.Context, cfg *Config) ([]models.Candle, error) {
	// Fetch in chunks to avoid API limits
	allCandles := make([]models.Candle, 0)
	
	currentStart := cfg.StartDate
	chunkSize := 1000
	
	for currentStart.Before(cfg.EndDate) {
		candles, err := e.exchange.FetchOHLCV(ctx, cfg.Symbol, cfg.Timeframe, chunkSize)
		if err != nil {
			return nil, err
		}
		
		if len(candles) == 0 {
			break
		}
		
		// Filter candles within date range
		for _, candle := range candles {
			if candle.Timestamp.After(currentStart) && candle.Timestamp.Before(cfg.EndDate) {
				allCandles = append(allCandles, candle)
			}
		}
		
		// Move to next chunk
		if len(candles) > 0 {
			currentStart = candles[len(candles)-1].Timestamp
		} else {
			break
		}
		
		// Rate limiting
		time.Sleep(100 * time.Millisecond)
	}
	
	return allCandles, nil
}

// simulateDecision simulates executing AI decision
func (e *Engine) simulateDecision(decision *models.AIDecision, price float64, timestamp time.Time) {
	switch decision.Action {
	case models.ActionOpenLong:
		e.openPosition(models.PositionLong, price, timestamp, decision.Reason)
	case models.ActionOpenShort:
		e.openPosition(models.PositionShort, price, timestamp, decision.Reason)
	case models.ActionClose:
		if e.position != nil {
			e.closePosition(price, timestamp, decision.Reason)
		}
	}
}

// openPosition opens new position in backtest
func (e *Engine) openPosition(side models.PositionSide, price float64, timestamp time.Time, reason string) {
	if e.position != nil {
		return // Already have position
	}
	
	// Calculate position size (30% of balance with 3x leverage)
	positionValue := e.balance * 0.3 * 3
	size := positionValue / price
	margin := e.balance * 0.3
	
	e.position = &models.Position{
		Symbol:        e.symbol,
		Side:          side,
		Size:          models.NewDecimal(size),
		EntryPrice:    models.NewDecimal(price),
		CurrentPrice:  models.NewDecimal(price),
		Leverage:      3,
		UnrealizedPnL: models.NewDecimal(0),
		Margin:        models.NewDecimal(margin),
		Timestamp:     timestamp,
	}
	
	// Reserve margin
	e.balance -= margin
	
	logger.Debug("position opened",
		zap.String("side", string(side)),
		zap.Float64("price", price),
		zap.Float64("size", size),
	)
}

// closePosition closes position in backtest
func (e *Engine) closePosition(price float64, timestamp time.Time, reason string) {
	if e.position == nil {
		return
	}
	
	// Calculate PnL
	entryPrice := e.position.EntryPrice.Float64()
	size := e.position.Size.Float64()
	
	var pnl float64
	if e.position.Side == models.PositionLong {
		pnl = (price - entryPrice) * size
	} else {
		pnl = (entryPrice - price) * size
	}
	
	// Apply fee (0.04% on entry + exit)
	fee := size * price * 0.0008
	pnl -= fee
	
	// Return margin and add PnL
	e.balance += e.position.Margin.Float64() + pnl
	e.equity = e.balance
	
	if e.equity > e.peakEquity {
		e.peakEquity = e.equity
	}
	
	duration := timestamp.Sub(e.position.Timestamp)
	pnlPercent := (pnl / e.position.Margin.Float64()) * 100
	
	// Record trade
	var side models.OrderSide
	if e.position.Side == models.PositionLong {
		side = models.SideSell
	} else {
		side = models.SideBuy
	}
	
	trade := BacktestTrade{
		Timestamp:  timestamp,
		Symbol:     e.symbol,
		Side:       side,
		Size:       size,
		EntryPrice: entryPrice,
		ExitPrice:  price,
		PnL:        pnl,
		PnLPercent: pnlPercent,
		Duration:   duration,
		Reason:     reason,
	}
	
	e.trades = append(e.trades, trade)
	e.position = nil
	
	logger.Debug("position closed",
		zap.Float64("exit_price", price),
		zap.Float64("pnl", pnl),
		zap.Float64("pnl_percent", pnlPercent),
	)
}

// updatePositionPnL updates unrealized PnL for open position
func (e *Engine) updatePositionPnL(currentPrice float64) {
	if e.position == nil {
		return
	}
	
	entryPrice := e.position.EntryPrice.Float64()
	size := e.position.Size.Float64()
	
	var unrealizedPnL float64
	if e.position.Side == models.PositionLong {
		unrealizedPnL = (currentPrice - entryPrice) * size
	} else {
		unrealizedPnL = (entryPrice - currentPrice) * size
	}
	
	e.position.UnrealizedPnL = models.NewDecimal(unrealizedPnL)
	e.position.CurrentPrice = models.NewDecimal(currentPrice)
	e.equity = e.balance + unrealizedPnL
}

// calculateMetrics calculates backtest performance metrics
func (e *Engine) calculateMetrics(cfg *Config) *BacktestResult {
	totalTrades := len(e.trades)
	winningTrades := 0
	losingTrades := 0
	totalPnL := 0.0
	wins := make([]float64, 0)
	losses := make([]float64, 0)
	
	for _, trade := range e.trades {
		totalPnL += trade.PnL
		
		if trade.PnL > 0 {
			winningTrades++
			wins = append(wins, trade.PnL)
		} else if trade.PnL < 0 {
			losingTrades++
			losses = append(losses, trade.PnL)
		}
	}
	
	winRate := 0.0
	if totalTrades > 0 {
		winRate = float64(winningTrades) / float64(totalTrades) * 100
	}
	
	avgWin := 0.0
	if len(wins) > 0 {
		for _, w := range wins {
			avgWin += w
		}
		avgWin /= float64(len(wins))
	}
	
	avgLoss := 0.0
	if len(losses) > 0 {
		for _, l := range losses {
			avgLoss += l
		}
		avgLoss /= float64(len(losses))
	}
	
	profitFactor := 0.0
	if avgLoss != 0 {
		profitFactor = avgWin / abs(avgLoss)
	}
	
	roi := (e.equity - cfg.InitialBalance) / cfg.InitialBalance * 100
	maxDrawdown := (e.peakEquity - e.equity) / e.peakEquity * 100
	
	// Calculate Sharpe Ratio (simplified)
	sharpe := e.calculateSharpeRatio()
	
	duration := cfg.EndDate.Sub(cfg.StartDate)
	
	return &BacktestResult{
		Symbol:         cfg.Symbol,
		StartDate:      cfg.StartDate,
		EndDate:        cfg.EndDate,
		Duration:       duration,
		InitialBalance: cfg.InitialBalance,
		FinalEquity:    e.equity,
		TotalPnL:       totalPnL,
		ROI:            roi,
		TotalTrades:    totalTrades,
		WinningTrades:  winningTrades,
		LosingTrades:   losingTrades,
		WinRate:        winRate,
		AverageWin:     avgWin,
		AverageLoss:    avgLoss,
		ProfitFactor:   profitFactor,
		MaxDrawdown:    maxDrawdown,
		SharpeRatio:    sharpe,
		Trades:         e.trades,
		AIDecisions:    len(e.decisions),
	}
}

// calculateSharpeRatio calculates Sharpe ratio
func (e *Engine) calculateSharpeRatio() float64 {
	if len(e.trades) < 2 {
		return 0
	}
	
	// Calculate returns
	returns := make([]float64, len(e.trades))
	for i, trade := range e.trades {
		returns[i] = trade.PnLPercent
	}
	
	// Calculate average return
	avgReturn := 0.0
	for _, r := range returns {
		avgReturn += r
	}
	avgReturn /= float64(len(returns))
	
	// Calculate standard deviation
	variance := 0.0
	for _, r := range returns {
		variance += (r - avgReturn) * (r - avgReturn)
	}
	variance /= float64(len(returns))
	stdDev := sqrt(variance)
	
	if stdDev == 0 {
		return 0
	}
	
	// Sharpe ratio (assuming 0 risk-free rate)
	return avgReturn / stdDev
}

// BacktestResult represents backtest results
type BacktestResult struct {
	Symbol         string          `json:"symbol"`
	StartDate      time.Time       `json:"start_date"`
	EndDate        time.Time       `json:"end_date"`
	Duration       time.Duration   `json:"duration"`
	InitialBalance float64         `json:"initial_balance"`
	FinalEquity    float64         `json:"final_equity"`
	TotalPnL       float64         `json:"total_pnl"`
	ROI            float64         `json:"roi_percent"`
	TotalTrades    int             `json:"total_trades"`
	WinningTrades  int             `json:"winning_trades"`
	LosingTrades   int             `json:"losing_trades"`
	WinRate        float64         `json:"win_rate_percent"`
	AverageWin     float64         `json:"average_win"`
	AverageLoss    float64         `json:"average_loss"`
	ProfitFactor   float64         `json:"profit_factor"`
	MaxDrawdown    float64         `json:"max_drawdown_percent"`
	SharpeRatio    float64         `json:"sharpe_ratio"`
	Trades         []BacktestTrade `json:"trades"`
	AIDecisions    int             `json:"ai_decisions"`
}

// Print prints backtest results
func (r *BacktestResult) Print() {
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("BACKTEST RESULTS")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("\nSymbol: %s\n", r.Symbol)
	fmt.Printf("Period: %s to %s (%.0f days)\n",
		r.StartDate.Format("2006-01-02"),
		r.EndDate.Format("2006-01-02"),
		r.Duration.Hours()/24,
	)
	fmt.Println("\nPERFORMANCE:")
	fmt.Printf("  Initial Balance: $%.2f\n", r.InitialBalance)
	fmt.Printf("  Final Equity:    $%.2f\n", r.FinalEquity)
	fmt.Printf("  Total PnL:       $%.2f (%.2f%%)\n", r.TotalPnL, r.ROI)
	fmt.Printf("  Max Drawdown:    %.2f%%\n", r.MaxDrawdown)
	
	fmt.Println("\nTRADING STATS:")
	fmt.Printf("  Total Trades:    %d\n", r.TotalTrades)
	fmt.Printf("  Winning Trades:  %d (%.1f%%)\n", r.WinningTrades, r.WinRate)
	fmt.Printf("  Losing Trades:   %d\n", r.LosingTrades)
	fmt.Printf("  Average Win:     $%.2f\n", r.AverageWin)
	fmt.Printf("  Average Loss:    $%.2f\n", r.AverageLoss)
	fmt.Printf("  Profit Factor:   %.2f\n", r.ProfitFactor)
	fmt.Printf("  Sharpe Ratio:    %.2f\n", r.SharpeRatio)
	
	fmt.Println("\nAI ANALYSIS:")
	fmt.Printf("  Total Decisions: %d\n", r.AIDecisions)
	fmt.Printf("  Execution Rate:  %.1f%%\n", float64(r.TotalTrades)/float64(r.AIDecisions)*100)
	
	fmt.Println(strings.Repeat("=", 60))
}

// Helper functions
import (
	"math"
	"strings"
)

func abs(x float64) float64 {
	return math.Abs(x)
}

func sqrt(x float64) float64 {
	return math.Sqrt(x)
}

