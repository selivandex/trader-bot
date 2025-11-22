package portfolio

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/alexanderselivanov/trader/internal/adapters/config"
	"github.com/alexanderselivanov/trader/internal/adapters/database"
	"github.com/alexanderselivanov/trader/internal/adapters/exchange"
	"github.com/alexanderselivanov/trader/pkg/logger"
	"github.com/alexanderselivanov/trader/pkg/models"
)

// UserTracker tracks portfolio for specific user
type UserTracker struct {
	*Tracker // Embed base tracker
	userID   int64
}

// NewUserTracker creates new user-specific portfolio tracker
func NewUserTracker(db *database.DB, ex exchange.Exchange, userID int64, cfg *config.TradingConfig) *UserTracker {
	return &UserTracker{
		Tracker: NewTracker(db, ex, cfg),
		userID:  userID,
	}
}

// Initialize loads user state from database
func (ut *UserTracker) Initialize(ctx context.Context) error {
	ut.mu.Lock()
	defer ut.mu.Unlock()
	
	// Load user state from database
	row := ut.db.Conn().QueryRowContext(ctx, `
		SELECT balance, equity, daily_pnl, peak_equity, updated_at
		FROM user_states
		WHERE user_id = $1
	`, ut.userID)
	
	var updatedAt time.Time
	err := row.Scan(&ut.currentBalance, &ut.equity, &ut.dailyPnL, &ut.peakEquity, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			// Initialize new user state
			return ut.initializeState(ctx)
		}
		return fmt.Errorf("failed to load user state: %w", err)
	}
	
	ut.lastDailyReset = updatedAt
	
	// Load trade statistics for this user
	if err := ut.loadTradeStats(ctx); err != nil {
		logger.Warn("failed to load trade stats", zap.Int64("user_id", ut.userID), zap.Error(err))
	}
	
	logger.Info("user portfolio tracker initialized",
		zap.Int64("user_id", ut.userID),
		zap.Float64("balance", ut.currentBalance),
		zap.Float64("equity", ut.equity),
	)
	
	return nil
}

// initializeState initializes user state in database
func (ut *UserTracker) initializeState(ctx context.Context) error {
	_, err := ut.db.Conn().ExecContext(ctx, `
		INSERT INTO user_states (user_id, mode, status, balance, equity, daily_pnl, peak_equity, updated_at)
		VALUES ($1, 'paper', 'running', $2, $2, 0, $2, $3)
		ON CONFLICT (user_id) DO UPDATE SET
			balance = $2,
			equity = $2,
			daily_pnl = 0,
			peak_equity = $2,
			updated_at = $3
	`, ut.userID, ut.initialBalance, time.Now())
	
	return err
}

// loadTradeStats loads trade statistics for this user
func (ut *UserTracker) loadTradeStats(ctx context.Context) error {
	row := ut.db.Conn().QueryRowContext(ctx, `
		SELECT 
			COUNT(*),
			COUNT(*) FILTER (WHERE pnl > 0),
			COUNT(*) FILTER (WHERE pnl < 0),
			COALESCE(SUM(pnl), 0)
		FROM trades
		WHERE user_id = $1
	`, ut.userID)
	
	return row.Scan(&ut.totalTrades, &ut.winningTrades, &ut.losingTrades, &ut.totalPnL)
}

// RecordTrade records a completed trade for this user
func (ut *UserTracker) RecordTrade(ctx context.Context, trade *models.Trade) error {
	ut.mu.Lock()
	defer ut.mu.Unlock()
	
	pnl := trade.PnL.Float64()
	
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
	_, err := ut.db.Conn().ExecContext(ctx, `
		INSERT INTO trades (user_id, exchange, symbol, side, type, amount, price, fee, pnl, ai_decision, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`,
		ut.userID,
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
func (ut *UserTracker) saveState(ctx context.Context) error {
	_, err := ut.db.Conn().ExecContext(ctx, `
		UPDATE user_states
		SET balance = $2, equity = $3, daily_pnl = $4, peak_equity = $5, updated_at = $6
		WHERE user_id = $1
	`, ut.userID, ut.currentBalance, ut.equity, ut.dailyPnL, ut.peakEquity, time.Now())
	
	return err
}

// resetDaily resets daily counters for this user
func (ut *UserTracker) resetDaily(ctx context.Context) error {
	logger.Info("resetting daily counters",
		zap.Int64("user_id", ut.userID),
		zap.Float64("previous_daily_pnl", ut.dailyPnL),
	)
	
	ut.dailyPnL = 0
	ut.lastDailyReset = time.Now()
	
	return ut.saveState(ctx)
}

