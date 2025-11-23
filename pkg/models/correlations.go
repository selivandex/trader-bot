package models

import (
	"time"

	"github.com/google/uuid"
)

// AssetCorrelation represents correlation between two assets over a period
type AssetCorrelation struct {
	ID           uuid.UUID `db:"id"`
	BaseSymbol   string    `db:"base_symbol"`
	QuoteSymbol  string    `db:"quote_symbol"`
	Period       string    `db:"period"` // "1h", "4h", "1d"
	Correlation  float64   `db:"correlation"`
	SampleSize   int       `db:"sample_size"`
	CalculatedAt time.Time `db:"calculated_at"`
	CreatedAt    time.Time `db:"created_at"`
}

// MarketRegime represents overall market conditions
type MarketRegime struct {
	ID               uuid.UUID `db:"id"`
	Regime           string    `db:"regime"` // "risk_on", "risk_off", "neutral"
	BTCDominance     float64   `db:"btc_dominance"`
	AvgCorrelation   float64   `db:"avg_correlation"`
	VolatilityLevel  string    `db:"volatility_level"` // "low", "medium", "high"
	Confidence       float64   `db:"confidence"`
	DetectedAt       time.Time `db:"detected_at"`
	CreatedAt        time.Time `db:"created_at"`
}

// CorrelationResult is returned by toolkit methods
type CorrelationResult struct {
	Symbol         string
	BTCCorrelation float64
	Period         string
	UpdatedAt      time.Time
}

// MarketRegimeResult is returned by toolkit methods
type MarketRegimeResult struct {
	Regime           string
	BTCDominance     float64
	AvgCorrelation   float64
	VolatilityLevel  string
	Confidence       float64
	DetectedAt       time.Time
}

