package models

import (
	"fmt"
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
	TypeMarket           OrderType = "market"
	TypeLimit            OrderType = "limit"
	TypeStopMarket       OrderType = "stop_market"        // Stop-loss market order
	TypeStopLimit        OrderType = "stop_limit"         // Stop-loss limit order
	TypeTakeProfitMarket OrderType = "take_profit_market" // Take-profit market order
	TypeTakeProfitLimit  OrderType = "take_profit_limit"  // Take-profit limit order
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
	Timestamp time.Time       `json:"timestamp"`
	Symbol    string          `json:"symbol"`
	Last      decimal.Decimal `json:"last"`
	Bid       decimal.Decimal `json:"bid"`
	Ask       decimal.Decimal `json:"ask"`
	High24h   decimal.Decimal `json:"high_24h"`
	Low24h    decimal.Decimal `json:"low_24h"`
	Volume24h decimal.Decimal `json:"volume_24h"`
	Change24h decimal.Decimal `json:"change_24h"`
}

// Candle represents OHLCV candlestick data
type Candle struct {
	Symbol      string          `json:"symbol"`
	Timeframe   string          `json:"timeframe"`
	Timestamp   time.Time       `json:"timestamp"`
	Open        decimal.Decimal `json:"open"`
	High        decimal.Decimal `json:"high"`
	Low         decimal.Decimal `json:"low"`
	Close       decimal.Decimal `json:"close"`
	Volume      decimal.Decimal `json:"volume"`
	QuoteVolume decimal.Decimal `json:"quote_volume"`
	Trades      int             `json:"trades"`
}

// OrderBook represents exchange order book
type OrderBook struct {
	Timestamp time.Time       `json:"timestamp"`
	Symbol    string          `json:"symbol"`
	Bids      []OrderBookItem `json:"bids"`
	Asks      []OrderBookItem `json:"asks"`
}

// OrderBookItem represents single order book level
type OrderBookItem struct {
	Price  decimal.Decimal `json:"price"`
	Amount decimal.Decimal `json:"amount"`
}

// Balance represents account balance
type Balance struct {
	Currencies map[string]CurrencyBalance `json:"currencies"`
	Total      decimal.Decimal            `json:"total"`
	Free       decimal.Decimal            `json:"free"`
	Used       decimal.Decimal            `json:"used"`
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
	Timestamp   time.Time       `json:"timestamp"`
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
}

// Position represents open futures position
type Position struct {
	Timestamp        time.Time       `json:"timestamp"`
	Symbol           string          `json:"symbol"`
	Side             PositionSide    `json:"side"`
	Size             decimal.Decimal `json:"size"`
	EntryPrice       decimal.Decimal `json:"entry_price"`
	CurrentPrice     decimal.Decimal `json:"current_price"`
	UnrealizedPnL    decimal.Decimal `json:"unrealized_pnl"`
	LiquidationPrice decimal.Decimal `json:"liquidation_price"`
	Margin           decimal.Decimal `json:"margin"`
	Leverage         int             `json:"leverage"`
}

// Trade represents executed trade
type Trade struct {
	CreatedAt   time.Time       `json:"created_at" db:"created_at"`
	ClosedAt    time.Time       `json:"closed_at" db:"closed_at"`
	OpenedAt    time.Time       `json:"opened_at" db:"opened_at"`
	Side        OrderSide       `json:"side" db:"side"`
	Price       decimal.Decimal `json:"price" db:"price"`
	AIDecision  string          `json:"ai_decision" db:"ai_decision"`
	Type        OrderType       `json:"type" db:"type"`
	EntryPrice  decimal.Decimal `json:"entry_price" db:"entry_price"`
	ExitPrice   decimal.Decimal `json:"exit_price" db:"exit_price"`
	Size        decimal.Decimal `json:"size" db:"size"`
	ID          string          `json:"id" db:"id"`
	Amount      decimal.Decimal `json:"amount" db:"amount"`
	Symbol      string          `json:"symbol" db:"symbol"`
	Fee         decimal.Decimal `json:"fee" db:"fee"`
	PnL         decimal.Decimal `json:"pnl" db:"pnl"`
	Exchange    string          `json:"exchange" db:"exchange"`
	RealizedPnL decimal.Decimal `json:"realized_pnl" db:"realized_pnl"`
	UserID      string          `json:"user_id" db:"user_id"`
	AgentID     string          `json:"agent_id" db:"agent_id"`
	EntryReason string          `json:"entry_reason" db:"entry_reason"`
	ExitReason  string          `json:"exit_reason" db:"exit_reason"`
	Leverage    int             `json:"leverage" db:"leverage"`
	PnLPercent  float64         `json:"pnl_percent" db:"pnl_percent"`
}

// AIDecision represents AI model decision
type AIDecision struct {
	CreatedAt  time.Time       `json:"created_at" db:"created_at"`
	Size       decimal.Decimal `json:"size"`
	Prompt     string          `json:"prompt" db:"prompt"`
	Response   string          `json:"response" db:"response"`
	Action     AIAction        `json:"action"`
	Reason     string          `json:"reason"`
	StopLoss   decimal.Decimal `json:"stop_loss"`
	TakeProfit decimal.Decimal `json:"take_profit"`
	Outcome    string          `json:"outcome" db:"outcome"`
	Provider   string          `json:"provider" db:"provider"`
	ID         int64           `json:"id" db:"id"`
	Confidence int             `json:"confidence" db:"confidence"`
	Executed   bool            `json:"executed" db:"executed"`
}

// BotState represents persisted bot state
type BotState struct {
	UpdatedAt time.Time       `json:"updated_at" db:"updated_at"`
	Mode      TradingMode     `json:"mode" db:"mode"`
	Status    BotStatus       `json:"status" db:"status"`
	Balance   decimal.Decimal `json:"balance" db:"balance"`
	Equity    decimal.Decimal `json:"equity" db:"equity"`
	DailyPnL  decimal.Decimal `json:"daily_pnl" db:"daily_pnl"`
	ID        int             `json:"id" db:"id"`
}

// MarketData aggregates all market information
type MarketData struct {
	Timestamp    time.Time            `json:"timestamp"`
	Ticker       *Ticker              `json:"ticker"`
	Candles      map[string][]Candle  `json:"candles"`
	OrderBook    *OrderBook           `json:"order_book"`
	Indicators   *TechnicalIndicators `json:"indicators"`
	NewsSummary  *NewsSummary         `json:"news_summary,omitempty"`
	OnChainData  *OnChainSummary      `json:"onchain_data,omitempty"`
	Symbol       string               `json:"symbol"`
	FundingRate  decimal.Decimal      `json:"funding_rate"`
	OpenInterest decimal.Decimal      `json:"open_interest"`
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
	Consensus  *AIDecision   `json:"consensus"`
	Decisions  []*AIDecision `json:"decisions"`
	Confidence int           `json:"confidence"`
	Agreement  bool          `json:"agreement"`
}

// StrategyParameters represents trading strategy parameters
type StrategyParameters struct {
	// Position sizing
	MaxPositionPercent float64 `json:"max_position_percent"` // Maximum position size as % of balance
	MaxLeverage        int     `json:"max_leverage"`         // Maximum leverage to use

	// Risk management
	StopLossPercent   float64 `json:"stop_loss_percent"`   // Stop loss distance from entry (%)
	TakeProfitPercent float64 `json:"take_profit_percent"` // Take profit target from entry (%)

	// Trading rules
	MinConfidenceThreshold int `json:"min_confidence_threshold"` // Minimum AI confidence to execute (0-100)
}

// Validate checks if strategy parameters are valid
func (p *StrategyParameters) Validate() error {
	if p.MaxPositionPercent <= 0 || p.MaxPositionPercent > 100 {
		return fmt.Errorf("invalid strategy parameter MaxPositionPercent: %v", p.MaxPositionPercent)
	}
	if p.MaxLeverage < 1 || p.MaxLeverage > 125 {
		return fmt.Errorf("invalid strategy parameter MaxLeverage: %v", p.MaxLeverage)
	}
	if p.StopLossPercent <= 0 {
		return fmt.Errorf("invalid strategy parameter StopLossPercent: %v", p.StopLossPercent)
	}
	if p.TakeProfitPercent <= 0 {
		return fmt.Errorf("invalid strategy parameter TakeProfitPercent: %v", p.TakeProfitPercent)
	}
	if p.MinConfidenceThreshold < 0 || p.MinConfidenceThreshold > 100 {
		return fmt.Errorf("invalid strategy parameter MinConfidenceThreshold: %v", p.MinConfidenceThreshold)
	}
	return nil
}
