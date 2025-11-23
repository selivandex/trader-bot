package models

import (
	"time"

	"github.com/shopspring/decimal"
)

// User represents registered bot user
type User struct {
	CreatedAt  time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at" db:"updated_at"`
	BannedAt   *time.Time `json:"banned_at,omitempty" db:"banned_at"`
	ID         string     `json:"id" db:"id"`
	Username   string     `json:"username" db:"username"`
	FirstName  string     `json:"first_name" db:"first_name"`
	BanReason  string     `json:"ban_reason,omitempty" db:"ban_reason"`
	TelegramID int64      `json:"telegram_id" db:"telegram_id"`
	IsActive   bool       `json:"is_active" db:"is_active"`
	IsBanned   bool       `json:"is_banned" db:"is_banned"`
}

// UserExchange represents user's exchange connection
type UserExchange struct {
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
	ID        string    `json:"id" db:"id"`
	UserID    string    `json:"user_id" db:"user_id"`
	Exchange  string    `json:"exchange" db:"exchange"`
	APIKey    string    `json:"-" db:"api_key"`
	APISecret string    `json:"-" db:"api_secret"`
	Testnet   bool      `json:"testnet" db:"testnet"`
	IsActive  bool      `json:"is_active" db:"is_active"`
}

// UserTradingPair represents a symbol user wants to trade
type UserTradingPair struct {
	CreatedAt  time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at" db:"updated_at"`
	ID         string          `json:"id" db:"id"`
	UserID     string          `json:"user_id" db:"user_id"`
	ExchangeID string          `json:"exchange_id" db:"exchange_id"`
	Symbol     string          `json:"symbol" db:"symbol"`
	Budget     decimal.Decimal `json:"budget" db:"budget"`
	IsActive   bool            `json:"is_active" db:"is_active"`
}

// AgentSymbolAssignment represents which agent trades which symbol
type AgentSymbolAssignment struct {
	CreatedAt     time.Time       `json:"created_at" db:"created_at"`
	ID            string          `json:"id" db:"id"`
	UserID        string          `json:"user_id" db:"user_id"`
	AgentID       string          `json:"agent_id" db:"agent_id"`
	TradingPairID string          `json:"trading_pair_id" db:"trading_pair_id"`
	Budget        decimal.Decimal `json:"budget" db:"budget"`
	IsActive      bool            `json:"is_active" db:"is_active"`
}
