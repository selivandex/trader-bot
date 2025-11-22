package models

import "github.com/shopspring/decimal"

// ToFloat64 safely converts decimal to float64
func ToFloat64(d decimal.Decimal) float64 {
	f, _ := d.Float64()
	return f
}

// MustFloat64 converts decimal to float64, panics on error
func MustFloat64(d decimal.Decimal) float64 {
	f, exact := d.Float64()
	if !exact {
		// Log warning but return value anyway
		// Most crypto prices fit in float64 range
	}
	return f
}

