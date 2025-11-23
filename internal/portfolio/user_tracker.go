package portfolio

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/selivandex/trader-bot/internal/adapters/config"
	"github.com/selivandex/trader-bot/internal/adapters/exchange"
	"github.com/selivandex/trader-bot/pkg/logger"
	"github.com/selivandex/trader-bot/pkg/models"
)

// UserTracker tracks portfolio for specific user
type UserTracker struct {
	*Tracker // Embed base tracker
	userID   int64
}

// NewUserTracker creates new user-specific portfolio tracker
func NewUserTracker(repo *Repository, ex exchange.Exchange, userID int64, cfg *config.TradingConfig) *UserTracker {
	return &UserTracker{
		Tracker: NewTracker(repo, ex, cfg),
		userID:  userID,
	}
}

// Initialize loads user state from database
func (ut *UserTracker) Initialize(ctx context.Context) error {
	ut.mu.Lock()
	defer ut.mu.Unlock()

	// Load user state from database
	state, err := ut.repo.LoadUserState(ctx, ut.userID)
	if err != nil {
		// Initialize new user state
		if err := ut.repo.InitializeUserState(ctx, ut.userID, ut.initialBalance); err != nil {
			return fmt.Errorf("failed to initialize user state: %w", err)
		}
		// Reload state
		state, err = ut.repo.LoadUserState(ctx, ut.userID)
		if err != nil {
			return fmt.Errorf("failed to load user state after init: %w", err)
		}
	}

	ut.currentBalance = state.Balance
	ut.equity = state.Equity
	ut.dailyPnL = state.DailyPnL
	ut.peakEquity = state.PeakEquity
	ut.lastDailyReset = state.UpdatedAt

	// Load trade statistics for this user
	stats, err := ut.repo.LoadUserTradeStats(ctx, ut.userID)
	if err != nil {
		logger.Warn("failed to load trade stats", zap.Int64("user_id", ut.userID), zap.Error(err))
	} else {
		ut.totalTrades = stats.TotalTrades
		ut.winningTrades = stats.WinningTrades
		ut.losingTrades = stats.LosingTrades
		ut.totalPnL = stats.TotalPnL
	}

	logger.Info("user portfolio tracker initialized",
		zap.Int64("user_id", ut.userID),
		zap.Float64("balance", ut.currentBalance),
		zap.Float64("equity", ut.equity),
	)

	return nil
}

// RecordTrade records a completed trade for this user
func (ut *UserTracker) RecordTrade(ctx context.Context, trade *models.Trade) error {
	ut.mu.Lock()
	defer ut.mu.Unlock()

	pnl, _ := trade.PnL.Float64()

	// Update statistics
	ut.totalTrades++
	ut.dailyPnL += pnl
	ut.totalPnL += pnl

	if pnl > 0 {
		ut.winningTrades++
	} else if pnl < 0 {
		ut.losingTrades++
	}

	// Save trade to database with user_id
	if err := ut.repo.RecordUserTrade(ctx, ut.userID, trade); err != nil {
		return err
	}

	logger.Info("trade recorded",
		zap.Int64("user_id", ut.userID),
		zap.String("symbol", trade.Symbol),
		zap.String("side", string(trade.Side)),
		zap.Float64("pnl", pnl),
	)

	// Check for daily reset
	if !isSameDay(ut.lastDailyReset, time.Now()) {
		if err := ut.resetDaily(ctx); err != nil {
			logger.Error("failed to reset daily stats", zap.Error(err))
		}
	}

	return nil
}

// saveState saves current state to database for this user
// resetDaily resets daily counters for this user
func (ut *UserTracker) resetDaily(ctx context.Context) error {
	logger.Info("resetting daily counters",
		zap.Int64("user_id", ut.userID),
		zap.Float64("previous_daily_pnl", ut.dailyPnL),
	)

	ut.dailyPnL = 0
	ut.lastDailyReset = time.Now()

	return ut.repo.SaveUserState(ctx, ut.userID, ut.currentBalance, ut.equity, ut.dailyPnL, ut.peakEquity)
}
