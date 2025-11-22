package models

import (
	"time"

	"github.com/shopspring/decimal"
)

// User represents registered bot user
type User struct {
	ID         int64     `json:"id" db:"id"`
	TelegramID int64     `json:"telegram_id" db:"telegram_id"`
	Username   string    `json:"username" db:"username"`
	FirstName  string    `json:"first_name" db:"first_name"`
	IsActive   bool      `json:"is_active" db:"is_active"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
}

// UserConfig represents user trading configuration
type UserConfig struct {
	ID                 int64           `json:"id" db:"id"`
	UserID             int64           `json:"user_id" db:"user_id"`
	Exchange           string          `json:"exchange" db:"exchange"`
	APIKey             string          `json:"-" db:"api_key"`    // Hidden in JSON
	APISecret          string          `json:"-" db:"api_secret"` // Hidden in JSON
	Testnet            bool            `json:"testnet" db:"testnet"`
	Symbol             string          `json:"symbol" db:"symbol"`
	InitialBalance     decimal.Decimal `json:"initial_balance" db:"initial_balance"`
	MaxPositionPercent decimal.Decimal `json:"max_position_percent" db:"max_position_percent"`
	MaxLeverage        int             `json:"max_leverage" db:"max_leverage"`
	StopLossPercent    decimal.Decimal `json:"stop_loss_percent" db:"stop_loss_percent"`
	TakeProfitPercent  decimal.Decimal `json:"take_profit_percent" db:"take_profit_percent"`
	IsTrading          bool            `json:"is_trading" db:"is_trading"`
	CreatedAt          time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at" db:"updated_at"`
}

// UserState represents current user bot state
type UserState struct {
	ID         int64           `json:"id" db:"id"`
	UserID     int64           `json:"user_id" db:"user_id"`
	Mode       string          `json:"mode" db:"mode"`
	Status     string          `json:"status" db:"status"`
	Balance    decimal.Decimal `json:"balance" db:"balance"`
	Equity     decimal.Decimal `json:"equity" db:"equity"`
	DailyPnL   decimal.Decimal `json:"daily_pnl" db:"daily_pnl"`
	PeakEquity decimal.Decimal `json:"peak_equity" db:"peak_equity"`
	UpdatedAt  time.Time       `json:"updated_at" db:"updated_at"`
}

// UserSession represents trading session
type UserSession struct {
	ID          int64           `json:"id" db:"id"`
	UserID      int64           `json:"user_id" db:"user_id"`
	StartedAt   time.Time       `json:"started_at" db:"started_at"`
	StoppedAt   *time.Time      `json:"stopped_at,omitempty" db:"stopped_at"`
	TradesCount int             `json:"trades_count" db:"trades_count"`
	TotalPnL    decimal.Decimal `json:"total_pnl" db:"total_pnl"`
	IsActive    bool            `json:"is_active" db:"is_active"`
}

// UserOverview aggregates user information
type UserOverview struct {
	ID            int64           `json:"id" db:"id"`
	TelegramID    int64           `json:"telegram_id" db:"telegram_id"`
	Username      string          `json:"username" db:"username"`
	Exchange      string          `json:"exchange" db:"exchange"`
	Symbol        string          `json:"symbol" db:"symbol"`
	Balance       decimal.Decimal `json:"balance" db:"balance"`
	Equity        decimal.Decimal `json:"equity" db:"equity"`
	DailyPnL      decimal.Decimal `json:"daily_pnl" db:"daily_pnl"`
	Status        string          `json:"status" db:"status"`
	IsTrading     bool            `json:"is_trading" db:"is_trading"`
	TotalTrades   int             `json:"total_trades" db:"total_trades"`
	WinningTrades int             `json:"winning_trades" db:"winning_trades"`
	TotalPnL      decimal.Decimal `json:"total_pnl" db:"total_pnl"`
	CreatedAt     time.Time       `json:"created_at" db:"created_at"`
}
