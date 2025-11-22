package models

import (
	"time"

	"github.com/shopspring/decimal"
)

// WhaleTransaction represents large blockchain movement
type WhaleTransaction struct {
	ID              int64           `json:"id" db:"id"`
	TxHash          string          `json:"tx_hash" db:"tx_hash"`
	Blockchain      string          `json:"blockchain" db:"blockchain"`
	Symbol          string          `json:"symbol" db:"symbol"`
	Amount          decimal.Decimal `json:"amount" db:"amount"`
	AmountUSD       decimal.Decimal `json:"amount_usd" db:"amount_usd"`
	FromAddress     string          `json:"from_address" db:"from_address"`
	ToAddress       string          `json:"to_address" db:"to_address"`
	FromOwner       string          `json:"from_owner" db:"from_owner"` // "binance", "unknown"
	ToOwner         string          `json:"to_owner" db:"to_owner"`
	TransactionType string          `json:"transaction_type" db:"transaction_type"`
	Timestamp       time.Time       `json:"timestamp" db:"timestamp"`
	ImpactScore     int             `json:"impact_score" db:"impact_score"`
	CreatedAt       time.Time       `json:"created_at" db:"created_at"`
}

// ExchangeFlow represents exchange inflow/outflow
type ExchangeFlow struct {
	ID        int64           `json:"id" db:"id"`
	Exchange  string          `json:"exchange" db:"exchange"`
	Symbol    string          `json:"symbol" db:"symbol"`
	Timestamp time.Time       `json:"timestamp" db:"timestamp"`
	Inflow    decimal.Decimal `json:"inflow" db:"inflow"`
	Outflow   decimal.Decimal `json:"outflow" db:"outflow"`
	NetFlow   decimal.Decimal `json:"net_flow" db:"net_flow"`
	CreatedAt time.Time       `json:"created_at" db:"created_at"`
}

// OnChainMetrics represents aggregated on-chain data
type OnChainMetrics struct {
	ID                    int64           `json:"id" db:"id"`
	Symbol                string          `json:"symbol" db:"symbol"`
	Timestamp             time.Time       `json:"timestamp" db:"timestamp"`
	ActiveAddresses       int             `json:"active_addresses" db:"active_addresses"`
	TransactionCount      int             `json:"transaction_count" db:"transaction_count"`
	AverageTxValue        decimal.Decimal `json:"average_tx_value" db:"average_tx_value"`
	LargeTxCount          int             `json:"large_tx_count" db:"large_tx_count"`
	ExchangeReserve       decimal.Decimal `json:"exchange_reserve" db:"exchange_reserve"`
	ExchangeReserveChange decimal.Decimal `json:"exchange_reserve_change" db:"exchange_reserve_change"`
	CreatedAt             time.Time       `json:"created_at" db:"created_at"`
}

// OnChainSummary aggregates on-chain signals for AI
type OnChainSummary struct {
	Symbol              string             `json:"symbol"`
	WhaleActivity       string             `json:"whale_activity"`       // high, medium, low
	ExchangeFlowDirection string           `json:"exchange_flow_direction"` // inflow, outflow, balanced
	NetExchangeFlow     decimal.Decimal    `json:"net_exchange_flow"`
	RecentWhaleMovements []WhaleTransaction `json:"recent_whale_movements,omitempty"`
	HighImpactAlerts    []OnChainAlert     `json:"high_impact_alerts,omitempty"`
	Metrics             *OnChainMetrics    `json:"metrics,omitempty"`
	UpdatedAt           time.Time          `json:"updated_at"`
}

// OnChainAlert represents significant on-chain event
type OnChainAlert struct {
	AlertType   string          `json:"alert_type"` // WHALE_MOVEMENT, EXCHANGE_INFLOW, etc
	Symbol      string          `json:"symbol"`
	Description string          `json:"description"`
	Value       decimal.Decimal `json:"value"` // USD value
	ImpactScore int             `json:"impact_score"`
	Timestamp   time.Time       `json:"timestamp"`
}

// GetWhaleActivityLevel returns activity level based on count and size
func (ocs *OnChainSummary) GetWhaleActivityLevel() string {
	if len(ocs.RecentWhaleMovements) == 0 {
		return "low"
	}
	
	// Count high impact movements
	highImpact := 0
	for _, whale := range ocs.RecentWhaleMovements {
		if whale.ImpactScore >= 7 {
			highImpact++
		}
	}
	
	if highImpact >= 3 {
		return "high"
	} else if highImpact >= 1 || len(ocs.RecentWhaleMovements) >= 5 {
		return "medium"
	}
	
	return "low"
}

