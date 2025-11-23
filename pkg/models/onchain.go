package models

import (
	"time"

	"github.com/shopspring/decimal"
)

// WhaleTransaction represents large blockchain movement
type WhaleTransaction struct {
	Timestamp       time.Time       `json:"timestamp" db:"timestamp"`
	CreatedAt       time.Time       `json:"created_at" db:"created_at"`
	DetectedAt      time.Time       `json:"detected_at" db:"detected_at"`
	ToAddress       string          `json:"to_address" db:"to_address"`
	ToOwner         string          `json:"to_owner" db:"to_owner"`
	Amount          decimal.Decimal `json:"amount" db:"amount"`
	AmountUSD       decimal.Decimal `json:"amount_usd" db:"amount_usd"`
	FromAddress     string          `json:"from_address" db:"from_address"`
	ID              string          `json:"id" db:"id"`
	FromOwner       string          `json:"from_owner" db:"from_owner"`
	Symbol          string          `json:"symbol" db:"symbol"`
	ExchangeName    string          `json:"exchange_name" db:"exchange_name"`
	TransactionType string          `json:"transaction_type" db:"transaction_type"`
	Blockchain      string          `json:"blockchain" db:"blockchain"`
	TransactionHash string          `json:"transaction_hash" db:"transaction_hash"`
	TxHash          string          `json:"tx_hash" db:"tx_hash"`
	ImpactScore     int             `json:"impact_score" db:"impact_score"`
}

// ExchangeFlow represents exchange inflow/outflow
type ExchangeFlow struct {
	Timestamp time.Time       `json:"timestamp" db:"timestamp"`
	CreatedAt time.Time       `json:"created_at" db:"created_at"`
	Exchange  string          `json:"exchange" db:"exchange"`
	Symbol    string          `json:"symbol" db:"symbol"`
	Inflow    decimal.Decimal `json:"inflow" db:"inflow"`
	Outflow   decimal.Decimal `json:"outflow" db:"outflow"`
	NetFlow   decimal.Decimal `json:"net_flow" db:"net_flow"`
	ID        int64           `json:"id" db:"id"`
}

// OnChainMetrics represents aggregated on-chain data
type OnChainMetrics struct {
	Timestamp             time.Time       `json:"timestamp" db:"timestamp"`
	CreatedAt             time.Time       `json:"created_at" db:"created_at"`
	Symbol                string          `json:"symbol" db:"symbol"`
	AverageTxValue        decimal.Decimal `json:"average_tx_value" db:"average_tx_value"`
	ExchangeReserve       decimal.Decimal `json:"exchange_reserve" db:"exchange_reserve"`
	ExchangeReserveChange decimal.Decimal `json:"exchange_reserve_change" db:"exchange_reserve_change"`
	ID                    int64           `json:"id" db:"id"`
	ActiveAddresses       int             `json:"active_addresses" db:"active_addresses"`
	TransactionCount      int             `json:"transaction_count" db:"transaction_count"`
	LargeTxCount          int             `json:"large_tx_count" db:"large_tx_count"`
}

// OnChainSummary aggregates on-chain signals for AI
type OnChainSummary struct {
	UpdatedAt             time.Time          `json:"updated_at"`
	Metrics               *OnChainMetrics    `json:"metrics,omitempty"`
	Symbol                string             `json:"symbol"`
	WhaleActivity         string             `json:"whale_activity"`
	ExchangeFlowDirection string             `json:"exchange_flow_direction"`
	NetExchangeFlow       decimal.Decimal    `json:"net_exchange_flow"`
	RecentWhaleMovements  []WhaleTransaction `json:"recent_whale_movements,omitempty"`
	HighImpactAlerts      []OnChainAlert     `json:"high_impact_alerts,omitempty"`
}

// OnChainAlert represents significant on-chain event
type OnChainAlert struct {
	Timestamp   time.Time       `json:"timestamp"`
	AlertType   string          `json:"alert_type"`
	Symbol      string          `json:"symbol"`
	Description string          `json:"description"`
	Value       decimal.Decimal `json:"value"`
	ImpactScore int             `json:"impact_score"`
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
