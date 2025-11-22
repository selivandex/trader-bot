package portfolio

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/alexanderselivanov/trader/internal/adapters/config"
	"github.com/alexanderselivanov/trader/internal/adapters/database"
	"github.com/alexanderselivanov/trader/internal/adapters/exchange"
	"github.com/alexanderselivanov/trader/pkg/logger"
	"github.com/alexanderselivanov/trader/pkg/models"
)

// Tracker tracks portfolio balance, equity, and performance
type Tracker struct {
	mu                         sync.RWMutex
	db                         *database.DB
	exchange                   exchange.Exchange
	initialBalance             float64
	currentBalance             float64
	equity                     float64
	peakEquity                 float64
	dailyPnL                   float64
	totalPnL                   float64
	totalTrades                int
	winningTrades              int
	losingTrades               int
	profitWithdrawalThreshold  float64
	lastDailyReset             time.Time
}

// NewTracker creates new portfolio tracker
func NewTracker(db *database.DB, ex exchange.Exchange, cfg *config.TradingConfig) *Tracker {
	return &Tracker{
		db:                        db,
		exchange:                  ex,
		initialBalance:            cfg.InitialBalance,
		currentBalance:            cfg.InitialBalance,
		equity:                    cfg.InitialBalance,
		peakEquity:                cfg.InitialBalance,
		profitWithdrawalThreshold: cfg.ProfitWithdrawalThreshold,
		lastDailyReset:            time.Now(),
	}
}

// Initialize loads state from database
func (t *Tracker) Initialize(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	// Load bot state from database
	row := t.db.Conn().QueryRowContext(ctx, `
		SELECT balance, equity, daily_pnl, updated_at
		FROM bot_state
		WHERE id = 1
	`)
	
	var updatedAt time.Time
	err := row.Scan(&t.currentBalance, &t.equity, &t.dailyPnL, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			// Initialize new state
			return t.initializeState(ctx)
		}
		return fmt.Errorf("failed to load bot state: %w", err)
	}
	
	t.peakEquity = t.equity
	t.lastDailyReset = updatedAt
	
	// Load trade statistics
	if err := t.loadTradeStats(ctx); err != nil {
		logger.Warn("failed to load trade stats", zap.Error(err))
	}
	
	logger.Info("portfolio tracker initialized",
		zap.Float64("balance", t.currentBalance),
		zap.Float64("equity", t.equity),
		zap.Float64("daily_pnl", t.dailyPnL),
	)
	
	return nil
}

// initializeState initializes bot state in database
func (t *Tracker) initializeState(ctx context.Context) error {
	_, err := t.db.Conn().ExecContext(ctx, `
		INSERT INTO bot_state (id, mode, status, balance, equity, daily_pnl, updated_at)
		VALUES (1, 'paper', 'stopped', $1, $1, 0, $2)
		ON CONFLICT (id) DO UPDATE SET
			balance = $1,
			equity = $1,
			daily_pnl = 0,
			updated_at = $2
	`, t.initialBalance, time.Now())
	
	return err
}

// loadTradeStats loads trade statistics from database
func (t *Tracker) loadTradeStats(ctx context.Context) error {
	row := t.db.Conn().QueryRowContext(ctx, `
		SELECT 
			COUNT(*),
			COUNT(*) FILTER (WHERE pnl > 0),
			COUNT(*) FILTER (WHERE pnl < 0),
			COALESCE(SUM(pnl), 0)
		FROM trades
	`)
	
	return row.Scan(&t.totalTrades, &t.winningTrades, &t.losingTrades, &t.totalPnL)
}

// UpdateFromExchange updates balance and equity from exchange
func (t *Tracker) UpdateFromExchange(ctx context.Context) error {
	// Fetch current balance
	balance, err := t.exchange.FetchBalance(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch balance: %w", err)
	}
	
	// Fetch open positions
	positions, err := t.exchange.FetchOpenPositions(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch positions: %w", err)
	}
	
	t.mu.Lock()
	defer t.mu.Unlock()
	
	// Update balance (free + used)
	t.currentBalance = balance.Total.Float64()
	
	// Calculate equity (balance + unrealized PnL)
	equity := t.currentBalance
	for _, pos := range positions {
		equity += pos.UnrealizedPnL.Float64()
	}
	
	t.equity = equity
	
	// Update peak equity
	if equity > t.peakEquity {
		t.peakEquity = equity
	}
	
	// Save to database
	if err := t.saveState(ctx); err != nil {
		logger.Error("failed to save portfolio state", zap.Error(err))
	}
	
	return nil
}

// RecordTrade records a completed trade
func (t *Tracker) RecordTrade(ctx context.Context, trade *models.Trade) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	pnl := trade.PnL.Float64()
	
	// Update statistics
	t.totalTrades++
	t.dailyPnL += pnl
	t.totalPnL += pnl
	
	if pnl > 0 {
		t.winningTrades++
	} else if pnl < 0 {
		t.losingTrades++
	}
	
	// Save trade to database
	_, err := t.db.Conn().ExecContext(ctx, `
		INSERT INTO trades (exchange, symbol, side, type, amount, price, fee, pnl, ai_decision, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`,
		trade.Exchange,
		trade.Symbol,
		string(trade.Side),
		string(trade.Type),
		trade.Amount.Float64(),
		trade.Price.Float64(),
		trade.Fee.Float64(),
		pnl,
		trade.AIDecision,
		time.Now(),
	)
	
	if err != nil {
		return fmt.Errorf("failed to record trade: %w", err)
	}
	
	logger.Info("trade recorded",
		zap.String("symbol", trade.Symbol),
		zap.String("side", string(trade.Side)),
		zap.Float64("pnl", pnl),
		zap.Float64("daily_pnl", t.dailyPnL),
	)
	
	// Check for daily reset
	if !isSameDay(t.lastDailyReset, time.Now()) {
		if err := t.resetDaily(ctx); err != nil {
			logger.Error("failed to reset daily stats", zap.Error(err))
		}
	}
	
	return nil
}

// CheckProfitWithdrawal checks if profit threshold is reached and returns withdrawal amount
func (t *Tracker) CheckProfitWithdrawal() (bool, float64) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	
	targetEquity := t.initialBalance * t.profitWithdrawalThreshold
	
	if t.equity >= targetEquity {
		withdrawAmount := t.equity - t.initialBalance
		return true, withdrawAmount
	}
	
	return false, 0
}

// GetBalance returns current balance
func (t *Tracker) GetBalance() float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.currentBalance
}

// GetEquity returns current equity
func (t *Tracker) GetEquity() float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.equity
}

// GetDailyPnL returns daily PnL
func (t *Tracker) GetDailyPnL() float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.dailyPnL
}

// GetPeakEquity returns peak equity
func (t *Tracker) GetPeakEquity() float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.peakEquity
}

// GetDrawdown returns current drawdown percentage
func (t *Tracker) GetDrawdown() float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	
	if t.peakEquity == 0 {
		return 0
	}
	
	return (t.peakEquity - t.equity) / t.peakEquity * 100
}

// GetStats returns portfolio statistics
func (t *Tracker) GetStats() *PortfolioStats {
	t.mu.RLock()
	defer t.mu.RUnlock()
	
	winRate := 0.0
	if t.totalTrades > 0 {
		winRate = float64(t.winningTrades) / float64(t.totalTrades) * 100
	}
	
	roi := 0.0
	if t.initialBalance > 0 {
		roi = (t.equity - t.initialBalance) / t.initialBalance * 100
	}
	
	return &PortfolioStats{
		InitialBalance: t.initialBalance,
		CurrentBalance: t.currentBalance,
		Equity:         t.equity,
		PeakEquity:     t.peakEquity,
		DailyPnL:       t.dailyPnL,
		TotalPnL:       t.totalPnL,
		ROI:            roi,
		Drawdown:       t.GetDrawdown(),
		TotalTrades:    t.totalTrades,
		WinningTrades:  t.winningTrades,
		LosingTrades:   t.losingTrades,
		WinRate:        winRate,
	}
}

// resetDaily resets daily counters
func (t *Tracker) resetDaily(ctx context.Context) error {
	logger.Info("resetting daily counters",
		zap.Float64("previous_daily_pnl", t.dailyPnL),
	)
	
	t.dailyPnL = 0
	t.lastDailyReset = time.Now()
	
	return t.saveState(ctx)
}

// saveState saves current state to database
func (t *Tracker) saveState(ctx context.Context) error {
	_, err := t.db.Conn().ExecContext(ctx, `
		UPDATE bot_state
		SET balance = $1, equity = $2, daily_pnl = $3, updated_at = $4
		WHERE id = 1
	`, t.currentBalance, t.equity, t.dailyPnL, time.Now())
	
	return err
}

// PortfolioStats represents portfolio statistics
type PortfolioStats struct {
	InitialBalance float64 `json:"initial_balance"`
	CurrentBalance float64 `json:"current_balance"`
	Equity         float64 `json:"equity"`
	PeakEquity     float64 `json:"peak_equity"`
	DailyPnL       float64 `json:"daily_pnl"`
	TotalPnL       float64 `json:"total_pnl"`
	ROI            float64 `json:"roi_percent"`
	Drawdown       float64 `json:"drawdown_percent"`
	TotalTrades    int     `json:"total_trades"`
	WinningTrades  int     `json:"winning_trades"`
	LosingTrades   int     `json:"losing_trades"`
	WinRate        float64 `json:"win_rate_percent"`
}

// Helper functions
func isSameDay(t1, t2 time.Time) bool {
	y1, m1, d1 := t1.Date()
	y2, m2, d2 := t2.Date()
	return y1 == y2 && m1 == m2 && d1 == d2
}

