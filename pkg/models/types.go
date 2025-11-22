package models

import (
	"time"

	"github.com/shopspring/decimal"
)

// NewDecimal creates decimal from float64
func NewDecimal(value float64) decimal.Decimal {
	return decimal.NewFromFloat(value)
}

// TradingMode represents the bot's operating mode
type TradingMode string

const (
	ModePaper TradingMode = "paper"
	ModeLive  TradingMode = "live"
)

// BotStatus represents current bot state
type BotStatus string

const (
	StatusRunning     BotStatus = "running"
	StatusStopped     BotStatus = "stopped"
	StatusCircuitOpen BotStatus = "circuit_open"
)

// OrderSide represents buy or sell
type OrderSide string

const (
	SideBuy  OrderSide = "buy"
	SideSell OrderSide = "sell"
)

// OrderType represents order type
type OrderType string

const (
	TypeMarket OrderType = "market"
	TypeLimit  OrderType = "limit"
)

// PositionSide represents long or short position
type PositionSide string

const (
	PositionLong  PositionSide = "long"
	PositionShort PositionSide = "short"
	PositionNone  PositionSide = "none"
)

// AIAction represents AI decision action
type AIAction string

const (
	ActionHold      AIAction = "HOLD"
	ActionClose     AIAction = "CLOSE"
	ActionOpenLong  AIAction = "OPEN_LONG"
	ActionOpenShort AIAction = "OPEN_SHORT"
	ActionScaleIn   AIAction = "SCALE_IN"
	ActionScaleOut  AIAction = "SCALE_OUT"
)

// Ticker represents market ticker data
type Ticker struct {
	Symbol    string          `json:"symbol"`
	Last      decimal.Decimal `json:"last"`
	Bid       decimal.Decimal `json:"bid"`
	Ask       decimal.Decimal `json:"ask"`
	High24h   decimal.Decimal `json:"high_24h"`
	Low24h    decimal.Decimal `json:"low_24h"`
	Volume24h decimal.Decimal `json:"volume_24h"`
	Change24h decimal.Decimal `json:"change_24h"`
	Timestamp time.Time       `json:"timestamp"`
}

// Candle represents OHLCV candlestick data
type Candle struct {
	Timestamp time.Time       `json:"timestamp"`
	Open      decimal.Decimal `json:"open"`
	High      decimal.Decimal `json:"high"`
	Low       decimal.Decimal `json:"low"`
	Close     decimal.Decimal `json:"close"`
	Volume    decimal.Decimal `json:"volume"`
}

// OrderBook represents exchange order book
type OrderBook struct {
	Symbol    string          `json:"symbol"`
	Bids      []OrderBookItem `json:"bids"`
	Asks      []OrderBookItem `json:"asks"`
	Timestamp time.Time       `json:"timestamp"`
}

// OrderBookItem represents single order book level
type OrderBookItem struct {
	Price  decimal.Decimal `json:"price"`
	Amount decimal.Decimal `json:"amount"`
}

// Balance represents account balance
type Balance struct {
	Total      decimal.Decimal            `json:"total"`
	Free       decimal.Decimal            `json:"free"`
	Used       decimal.Decimal            `json:"used"`
	Currencies map[string]CurrencyBalance `json:"currencies"`
}

// CurrencyBalance represents balance for specific currency
type CurrencyBalance struct {
	Currency string          `json:"currency"`
	Total    decimal.Decimal `json:"total"`
	Free     decimal.Decimal `json:"free"`
	Used     decimal.Decimal `json:"used"`
}

// Order represents trading order
type Order struct {
	ID          string          `json:"id"`
	Symbol      string          `json:"symbol"`
	Type        OrderType       `json:"type"`
	Side        OrderSide       `json:"side"`
	Price       decimal.Decimal `json:"price"`
	Amount      decimal.Decimal `json:"amount"`
	Filled      decimal.Decimal `json:"filled"`
	Remaining   decimal.Decimal `json:"remaining"`
	Status      string          `json:"status"`
	Fee         decimal.Decimal `json:"fee"`
	FeeCurrency string          `json:"fee_currency"`
	Timestamp   time.Time       `json:"timestamp"`
}

// Position represents open futures position
type Position struct {
	Symbol           string          `json:"symbol"`
	Side             PositionSide    `json:"side"`
	Size             decimal.Decimal `json:"size"`
	EntryPrice       decimal.Decimal `json:"entry_price"`
	CurrentPrice     decimal.Decimal `json:"current_price"`
	Leverage         int             `json:"leverage"`
	UnrealizedPnL    decimal.Decimal `json:"unrealized_pnl"`
	LiquidationPrice decimal.Decimal `json:"liquidation_price"`
	Margin           decimal.Decimal `json:"margin"`
	Timestamp        time.Time       `json:"timestamp"`
}

// Trade represents executed trade
type Trade struct {
	ID         int64           `json:"id" db:"id"`
	Exchange   string          `json:"exchange" db:"exchange"`
	Symbol     string          `json:"symbol" db:"symbol"`
	Side       OrderSide       `json:"side" db:"side"`
	Type       OrderType       `json:"type" db:"type"`
	Amount     decimal.Decimal `json:"amount" db:"amount"`
	Price      decimal.Decimal `json:"price" db:"price"`
	Fee        decimal.Decimal `json:"fee" db:"fee"`
	PnL        decimal.Decimal `json:"pnl" db:"pnl"`
	AIDecision string          `json:"ai_decision" db:"ai_decision"` // JSONB
	CreatedAt  time.Time       `json:"created_at" db:"created_at"`
}

// AIDecision represents AI model decision
type AIDecision struct {
	ID         int64           `json:"id" db:"id"`
	Provider   string          `json:"provider" db:"provider"`
	Prompt     string          `json:"prompt" db:"prompt"`
	Response   string          `json:"response" db:"response"` // JSONB
	Action     AIAction        `json:"action"`
	Reason     string          `json:"reason"`
	Size       decimal.Decimal `json:"size"`
	StopLoss   decimal.Decimal `json:"stop_loss"`
	TakeProfit decimal.Decimal `json:"take_profit"`
	Confidence int             `json:"confidence" db:"confidence"`
	Executed   bool            `json:"executed" db:"executed"`
	Outcome    string          `json:"outcome" db:"outcome"` // JSONB
	CreatedAt  time.Time       `json:"created_at" db:"created_at"`
}

// BotState represents persisted bot state
type BotState struct {
	ID        int             `json:"id" db:"id"`
	Mode      TradingMode     `json:"mode" db:"mode"`
	Status    BotStatus       `json:"status" db:"status"`
	Balance   decimal.Decimal `json:"balance" db:"balance"`
	Equity    decimal.Decimal `json:"equity" db:"equity"`
	DailyPnL  decimal.Decimal `json:"daily_pnl" db:"daily_pnl"`
	UpdatedAt time.Time       `json:"updated_at" db:"updated_at"`
}

// MarketData aggregates all market information
type MarketData struct {
	Symbol       string               `json:"symbol"`
	Ticker       *Ticker              `json:"ticker"`
	Candles      map[string][]Candle  `json:"candles"` // timeframe -> candles
	OrderBook    *OrderBook           `json:"order_book"`
	FundingRate  decimal.Decimal      `json:"funding_rate"`
	OpenInterest decimal.Decimal      `json:"open_interest"`
	Indicators   *TechnicalIndicators `json:"indicators"`
	Timestamp    time.Time            `json:"timestamp"`
}

// TechnicalIndicators represents calculated technical indicators
type TechnicalIndicators struct {
	RSI            map[string]decimal.Decimal `json:"rsi"` // timeframe -> value
	MACD           *MACDIndicator             `json:"macd"`
	BollingerBands *BollingerBandsIndicator   `json:"bollinger_bands"`
	Volume         *VolumeIndicator           `json:"volume"`
}

// MACDIndicator represents MACD indicator values
type MACDIndicator struct {
	MACD      decimal.Decimal `json:"macd"`
	Signal    decimal.Decimal `json:"signal"`
	Histogram decimal.Decimal `json:"histogram"`
}

// BollingerBandsIndicator represents Bollinger Bands values
type BollingerBandsIndicator struct {
	Upper  decimal.Decimal `json:"upper"`
	Middle decimal.Decimal `json:"middle"`
	Lower  decimal.Decimal `json:"lower"`
}

// VolumeIndicator represents volume analysis
type VolumeIndicator struct {
	Current decimal.Decimal `json:"current"`
	Average decimal.Decimal `json:"average"`
	Ratio   decimal.Decimal `json:"ratio"` // current/average
}

// TradingPrompt represents data sent to AI for analysis
type TradingPrompt struct {
	MarketData      *MarketData     `json:"market_data"`
	CurrentPosition *Position       `json:"current_position,omitempty"`
	Balance         decimal.Decimal `json:"balance"`
	Equity          decimal.Decimal `json:"equity"`
	DailyPnL        decimal.Decimal `json:"daily_pnl"`
	RecentTrades    []Trade         `json:"recent_trades,omitempty"`
}

// EnsembleDecision represents consensus from multiple AI providers
type EnsembleDecision struct {
	Decisions  []*AIDecision `json:"decisions"`
	Consensus  *AIDecision   `json:"consensus"`
	Agreement  bool          `json:"agreement"`
	Confidence int           `json:"confidence"`
}
