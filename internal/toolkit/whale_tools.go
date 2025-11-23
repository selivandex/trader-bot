package toolkit

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/selivandex/trader-bot/pkg/logger"
	"github.com/selivandex/trader-bot/pkg/models"
)

// ============ Advanced Whale Analysis Tools ============

// GetWhaleAlertsSummary gets comprehensive whale activity summary
func (t *LocalToolkit) GetWhaleAlertsSummary(ctx context.Context, symbol string, hours int) (*WhaleAlertsSummary, error) {
	logger.Debug("toolkit: get_whale_alerts_summary",
		zap.String("agent_id", t.agentID),
		zap.String("symbol", symbol),
		zap.Int("hours", hours),
	)

	whales, err := t.GetRecentWhaleMovements(ctx, symbol, 1_000_000, hours)
	if err != nil {
		return nil, fmt.Errorf("failed to get whale movements: %w", err)
	}

	summary := &WhaleAlertsSummary{
		TotalTransactions: len(whales),
		TimeWindow:        time.Duration(hours) * time.Hour,
	}

	var totalInflow, totalOutflow float64
	var largestInflow, largestOutflow float64

	for _, whale := range whales {
		amountUSD := whale.AmountUSD.InexactFloat64()

		switch whale.TransactionType {
		case "exchange_inflow":
			totalInflow += amountUSD
			summary.InflowCount++
			if amountUSD > largestInflow {
				largestInflow = amountUSD
			}

		case "exchange_outflow":
			totalOutflow += amountUSD
			summary.OutflowCount++
			if amountUSD > largestOutflow {
				largestOutflow = amountUSD
			}

		case "transfer":
			summary.TransferCount++
		}

		// Track very large transactions (>$10M)
		if amountUSD >= 10_000_000 {
			summary.MegaWhaleCount++
		}
	}

	summary.TotalInflowUSD = totalInflow
	summary.TotalOutflowUSD = totalOutflow
	summary.NetFlowUSD = totalOutflow - totalInflow // Outflow = bullish
	summary.LargestInflowUSD = largestInflow
	summary.LargestOutflowUSD = largestOutflow

	// Calculate sentiment
	if summary.NetFlowUSD > 10_000_000 {
		summary.Sentiment = "bullish" // Accumulation
	} else if summary.NetFlowUSD < -10_000_000 {
		summary.Sentiment = "bearish" // Distribution
	} else {
		summary.Sentiment = "neutral"
	}

	// Alert level
	if summary.MegaWhaleCount > 0 || abs(summary.NetFlowUSD) > 50_000_000 {
		summary.AlertLevel = "HIGH"
	} else if summary.TotalTransactions > 10 {
		summary.AlertLevel = "MEDIUM"
	} else {
		summary.AlertLevel = "LOW"
	}

	return summary, nil
}

// DetectWhalePattern detects if whales are accumulating or distributing
func (t *LocalToolkit) DetectWhalePattern(ctx context.Context, symbol string, hours int) (string, float64, error) {
	logger.Debug("toolkit: detect_whale_pattern",
		zap.String("agent_id", t.agentID),
		zap.String("symbol", symbol),
		zap.Int("hours", hours),
	)

	summary, err := t.GetWhaleAlertsSummary(ctx, symbol, hours)
	if err != nil {
		return "", 0, err
	}

	// Calculate accumulation strength (0-100)
	strength := 50.0 // Neutral baseline

	if summary.NetFlowUSD > 0 {
		// Outflow = accumulation (coins leaving exchanges)
		strength += minFloat(summary.NetFlowUSD/1_000_000, 50) // Max +50
	} else {
		// Inflow = distribution (coins entering exchanges)
		strength -= minFloat(abs(summary.NetFlowUSD)/1_000_000, 50) // Max -50
	}

	// Adjust for mega whale activity
	if summary.MegaWhaleCount > 0 {
		if summary.OutflowCount > summary.InflowCount {
			strength += 10 // Strong accumulation signal
		} else {
			strength -= 10 // Strong distribution signal
		}
	}

	// Clamp 0-100
	if strength < 0 {
		strength = 0
	}
	if strength > 100 {
		strength = 100
	}

	pattern := "neutral"
	if strength > 70 {
		pattern = "strong_accumulation"
	} else if strength > 55 {
		pattern = "weak_accumulation"
	} else if strength < 30 {
		pattern = "strong_distribution"
	} else if strength < 45 {
		pattern = "weak_distribution"
	}

	return pattern, strength, nil
}

// GetWhalesByExchange groups whale transactions by exchange
func (t *LocalToolkit) GetWhalesByExchange(ctx context.Context, symbol string, hours int) (map[string][]models.WhaleTransaction, error) {
	logger.Debug("toolkit: get_whales_by_exchange",
		zap.String("agent_id", t.agentID),
		zap.String("symbol", symbol),
		zap.Int("hours", hours),
	)

	whales, err := t.GetRecentWhaleMovements(ctx, symbol, 1_000_000, hours)
	if err != nil {
		return nil, err
	}

	byExchange := make(map[string][]models.WhaleTransaction)

	for _, whale := range whales {
		exchange := whale.ExchangeName
		if exchange == "" {
			exchange = "unknown"
		}

		byExchange[exchange] = append(byExchange[exchange], whale)
	}

	return byExchange, nil
}

// CheckWhaleAlert checks if there are urgent whale alerts
func (t *LocalToolkit) CheckWhaleAlert(ctx context.Context, symbol string) (*WhaleAlert, error) {
	logger.Debug("toolkit: check_whale_alert",
		zap.String("agent_id", t.agentID),
		zap.String("symbol", symbol),
	)

	// Check last hour for mega whale activity
	whales, err := t.GetRecentWhaleMovements(ctx, symbol, 10_000_000, 1)
	if err != nil {
		return nil, err
	}

	if len(whales) == 0 {
		return &WhaleAlert{
			HasAlert: false,
			Severity: "NONE",
			Message:  "No significant whale activity",
		}, nil
	}

	// Found mega whale transactions
	largest := &whales[0]
	for i := range whales {
		if whales[i].AmountUSD.GreaterThan(largest.AmountUSD) {
			largest = &whales[i]
		}
	}

	severity := "LOW"
	if largest.AmountUSD.InexactFloat64() > 50_000_000 {
		severity = "CRITICAL"
	} else if largest.AmountUSD.InexactFloat64() > 25_000_000 {
		severity = "HIGH"
	} else if largest.AmountUSD.InexactFloat64() > 10_000_000 {
		severity = "MEDIUM"
	}

	direction := "distribution"
	if largest.TransactionType == "exchange_outflow" {
		direction = "accumulation"
	}

	message := fmt.Sprintf(
		"🐋 WHALE ALERT: $%.1fM %s detected (%s → %s) just %s ago",
		largest.AmountUSD.InexactFloat64()/1_000_000,
		direction,
		truncateAddress(largest.FromAddress),
		truncateAddress(largest.ToAddress),
		formatDuration(time.Since(largest.Timestamp)),
	)

	return &WhaleAlert{
		HasAlert:   true,
		Severity:   severity,
		Message:    message,
		LargestTx:  largest,
		TotalCount: len(whales),
		Pattern:    direction,
	}, nil
}

// WhaleAlertsSummary contains comprehensive whale activity analysis
type WhaleAlertsSummary struct {
	TotalTransactions int
	InflowCount       int
	OutflowCount      int
	TransferCount     int
	MegaWhaleCount    int // Transactions > $10M
	TotalInflowUSD    float64
	TotalOutflowUSD   float64
	NetFlowUSD        float64 // Positive = outflow (accumulation)
	LargestInflowUSD  float64
	LargestOutflowUSD float64
	Sentiment         string // "bullish", "bearish", "neutral"
	AlertLevel        string // "HIGH", "MEDIUM", "LOW"
	TimeWindow        time.Duration
}

// WhaleAlert represents urgent whale activity notification
type WhaleAlert struct {
	HasAlert   bool
	Severity   string // "CRITICAL", "HIGH", "MEDIUM", "LOW", "NONE"
	Message    string
	LargestTx  *models.WhaleTransaction
	TotalCount int
	Pattern    string // "accumulation", "distribution"
}
