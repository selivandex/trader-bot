package portfolio

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"

	"github.com/selivandex/trader-bot/pkg/models"
)

// Repository handles database operations for portfolio tracking
type Repository struct {
	db *sqlx.DB
}

// NewRepository creates new portfolio repository
func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{db: db}
}

// ========== Bot State Operations ==========

// BotState represents global bot state
type BotState struct {
	ID        int64     `db:"id"`
	Mode      string    `db:"mode"`
	Status    string    `db:"status"`
	Balance   float64   `db:"balance"`
	Equity    float64   `db:"equity"`
	DailyPnL  float64   `db:"daily_pnl"`
	UpdatedAt time.Time `db:"updated_at"`
}

// LoadBotState loads global bot state
func (r *Repository) LoadBotState(ctx context.Context) (*BotState, error) {
	query := `
		SELECT id, mode, status, balance, equity, daily_pnl, updated_at
		FROM bot_state
		WHERE id = 1
	`

	var state BotState
	err := r.db.GetContext(ctx, &state, query)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("bot state not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load bot state: %w", err)
	}

	return &state, nil
}

// InitializeBotState creates or resets bot state
func (r *Repository) InitializeBotState(ctx context.Context, initialBalance float64) error {
	query := `
		INSERT INTO bot_state (id, mode, status, balance, equity, daily_pnl, updated_at)
		VALUES (1, 'paper', 'stopped', $1, $1, 0, $2)
		ON CONFLICT (id) DO UPDATE SET
			balance = $1,
			equity = $1,
			daily_pnl = 0,
			updated_at = $2
	`

	_, err := r.db.ExecContext(ctx, query, initialBalance, time.Now())
	return err
}

// SaveBotState updates bot state
func (r *Repository) SaveBotState(ctx context.Context, balance, equity, dailyPnL float64) error {
	query := `
		UPDATE bot_state
		SET balance = $1, equity = $2, daily_pnl = $3, updated_at = $4
		WHERE id = 1
	`

	_, err := r.db.ExecContext(ctx, query, balance, equity, dailyPnL, time.Now())
	return err
}

// ========== User State Operations ==========

// UserState represents user-specific portfolio state
type UserState struct {
	ID         int64     `db:"id"`
	UserID     int64     `db:"user_id"`
	Mode       string    `db:"mode"`
	Status     string    `db:"status"`
	Balance    float64   `db:"balance"`
	Equity     float64   `db:"equity"`
	DailyPnL   float64   `db:"daily_pnl"`
	PeakEquity float64   `db:"peak_equity"`
	UpdatedAt  time.Time `db:"updated_at"`
}

// LoadUserState loads user-specific state
func (r *Repository) LoadUserState(ctx context.Context, userID int64) (*UserState, error) {
	query := `
		SELECT id, user_id, mode, status, balance, equity, daily_pnl, peak_equity, updated_at
		FROM user_states
		WHERE user_id = $1
	`

	var state UserState
	err := r.db.GetContext(ctx, &state, query, userID)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user state not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load user state: %w", err)
	}

	return &state, nil
}

// InitializeUserState creates or resets user state
func (r *Repository) InitializeUserState(ctx context.Context, userID int64, initialBalance float64) error {
	query := `
		INSERT INTO user_states (user_id, mode, status, balance, equity, daily_pnl, peak_equity, updated_at)
		VALUES ($1, 'paper', 'running', $2, $2, 0, $2, $3)
		ON CONFLICT (user_id) DO UPDATE SET
			balance = $2,
			equity = $2,
			daily_pnl = 0,
			peak_equity = $2,
			updated_at = $3
	`

	_, err := r.db.ExecContext(ctx, query, userID, initialBalance, time.Now())
	return err
}

// SaveUserState updates user state
func (r *Repository) SaveUserState(ctx context.Context, userID int64, balance, equity, dailyPnL, peakEquity float64) error {
	query := `
		UPDATE user_states
		SET balance = $2, equity = $3, daily_pnl = $4, peak_equity = $5, updated_at = $6
		WHERE user_id = $1
	`

	_, err := r.db.ExecContext(ctx, query, userID, balance, equity, dailyPnL, peakEquity, time.Now())
	return err
}

// ========== Trade Statistics Operations ==========

// TradeStats holds trade statistics
type TradeStats struct {
	TotalTrades   int     `db:"total_trades"`
	WinningTrades int     `db:"winning_trades"`
	LosingTrades  int     `db:"losing_trades"`
	TotalPnL      float64 `db:"total_pnl"`
}

// LoadTradeStats loads global trade statistics
func (r *Repository) LoadTradeStats(ctx context.Context) (*TradeStats, error) {
	query := `
		SELECT 
			COUNT(*) as total_trades,
			COUNT(*) FILTER (WHERE pnl > 0) as winning_trades,
			COUNT(*) FILTER (WHERE pnl < 0) as losing_trades,
			COALESCE(SUM(pnl), 0) as total_pnl
		FROM trades
	`

	var stats TradeStats
	err := r.db.GetContext(ctx, &stats, query)
	if err != nil {
		return nil, fmt.Errorf("failed to load trade stats: %w", err)
	}

	return &stats, nil
}

// LoadUserTradeStats loads user-specific trade statistics
func (r *Repository) LoadUserTradeStats(ctx context.Context, userID int64) (*TradeStats, error) {
	query := `
		SELECT 
			COUNT(*) as total_trades,
			COUNT(*) FILTER (WHERE pnl > 0) as winning_trades,
			COUNT(*) FILTER (WHERE pnl < 0) as losing_trades,
			COALESCE(SUM(pnl), 0) as total_pnl
		FROM trades
		WHERE user_id = $1
	`

	var stats TradeStats
	err := r.db.GetContext(ctx, &stats, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to load user trade stats: %w", err)
	}

	return &stats, nil
}

// ========== Trade Recording Operations ==========

// RecordTrade saves trade to database
func (r *Repository) RecordTrade(ctx context.Context, trade *models.Trade) error {
	query := `
		INSERT INTO trades (exchange, symbol, side, type, amount, price, fee, pnl, ai_decision, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id
	`

	pnl, _ := trade.PnL.Float64()

	var id int64
	err := r.db.QueryRowContext(
		ctx, query,
		trade.Exchange,
		trade.Symbol,
		string(trade.Side),
		string(trade.Type),
		models.ToFloat64(trade.Amount),
		models.ToFloat64(trade.Price),
		models.ToFloat64(trade.Fee),
		pnl,
		trade.AIDecision,
		time.Now(),
	).Scan(&id)

	if err != nil {
		return fmt.Errorf("failed to record trade: %w", err)
	}

	return nil
}

// RecordUserTrade saves user-specific trade to database
func (r *Repository) RecordUserTrade(ctx context.Context, userID int64, trade *models.Trade) error {
	query := `
		INSERT INTO trades (user_id, exchange, symbol, side, type, amount, price, fee, pnl, ai_decision, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id
	`

	pnl, _ := trade.PnL.Float64()

	var id int64
	err := r.db.QueryRowContext(
		ctx, query,
		userID,
		trade.Exchange,
		trade.Symbol,
		string(trade.Side),
		string(trade.Type),
		models.ToFloat64(trade.Amount),
		models.ToFloat64(trade.Price),
		models.ToFloat64(trade.Fee),
		pnl,
		trade.AIDecision,
		time.Now(),
	).Scan(&id)

	if err != nil {
		return fmt.Errorf("failed to record user trade: %w", err)
	}

	return nil
}

// ========== Performance Metrics Operations ==========

// GetRecentTrades retrieves recent trades for analysis
func (r *Repository) GetRecentTrades(ctx context.Context, limit int) ([]models.Trade, error) {
	query := `
		SELECT id, exchange, symbol, side, type, amount, price, fee, pnl, ai_decision, created_at
		FROM trades
		ORDER BY created_at DESC
		LIMIT $1
	`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query recent trades: %w", err)
	}
	defer rows.Close()

	var trades []models.Trade
	for rows.Next() {
		var trade models.Trade
		var pnl float64

		err := rows.Scan(
			&trade.ID,
			&trade.Exchange,
			&trade.Symbol,
			&trade.Side,
			&trade.Type,
			&trade.Amount,
			&trade.Price,
			&trade.Fee,
			&pnl,
			&trade.AIDecision,
			&trade.CreatedAt,
		)
		if err != nil {
			continue
		}

		// Convert float64 to decimal.Decimal
		trade.PnL = decimal.NewFromFloat(pnl)
		trades = append(trades, trade)
	}

	return trades, nil
}

// GetUserRecentTrades retrieves recent trades for specific user
func (r *Repository) GetUserRecentTrades(ctx context.Context, userID int64, limit int) ([]models.Trade, error) {
	query := `
		SELECT id, exchange, symbol, side, type, amount, price, fee, pnl, ai_decision, created_at
		FROM trades
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := r.db.QueryContext(ctx, query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query user recent trades: %w", err)
	}
	defer rows.Close()

	var trades []models.Trade
	for rows.Next() {
		var trade models.Trade
		var pnl float64

		err := rows.Scan(
			&trade.ID,
			&trade.Exchange,
			&trade.Symbol,
			&trade.Side,
			&trade.Type,
			&trade.Amount,
			&trade.Price,
			&trade.Fee,
			&pnl,
			&trade.AIDecision,
			&trade.CreatedAt,
		)
		if err != nil {
			continue
		}

		// Convert float64 to decimal.Decimal
		trade.PnL = decimal.NewFromFloat(pnl)
		trades = append(trades, trade)
	}

	return trades, nil
}

// ========== Daily Metrics Operations ==========

// GetDailyPnL calculates daily PnL from trades
func (r *Repository) GetDailyPnL(ctx context.Context, date time.Time) (float64, error) {
	query := `
		SELECT COALESCE(SUM(pnl), 0) as daily_pnl
		FROM trades
		WHERE DATE(created_at) = DATE($1)
	`

	var dailyPnL float64
	err := r.db.QueryRowContext(ctx, query, date).Scan(&dailyPnL)
	if err != nil {
		return 0, fmt.Errorf("failed to get daily pnl: %w", err)
	}

	return dailyPnL, nil
}

// GetUserDailyPnL calculates daily PnL for specific user
func (r *Repository) GetUserDailyPnL(ctx context.Context, userID int64, date time.Time) (float64, error) {
	query := `
		SELECT COALESCE(SUM(pnl), 0) as daily_pnl
		FROM trades
		WHERE user_id = $1 AND DATE(created_at) = DATE($2)
	`

	var dailyPnL float64
	err := r.db.QueryRowContext(ctx, query, userID, date).Scan(&dailyPnL)
	if err != nil {
		return 0, fmt.Errorf("failed to get user daily pnl: %w", err)
	}

	return dailyPnL, nil
}
