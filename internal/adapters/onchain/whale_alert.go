package onchain

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/selivandex/trader-bot/pkg/logger"
	"github.com/selivandex/trader-bot/pkg/models"
)

const whaleAlertAPIURL = "https://api.whale-alert.io/v1/transactions"

// WhaleAlertAdapter implements OnChainProvider for Whale Alert API
type WhaleAlertAdapter struct {
	client  *http.Client
	apiKey  string
	enabled bool
}

// Legacy alias for backward compatibility
type WhaleAlertProvider = WhaleAlertAdapter

// NewWhaleAlertProvider creates new Whale Alert adapter
func NewWhaleAlertProvider(apiKey string, enabled bool) *WhaleAlertAdapter {
	return &WhaleAlertAdapter{
		apiKey:  apiKey,
		enabled: enabled && apiKey != "",
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

// GetName returns provider name
func (wa *WhaleAlertAdapter) GetName() string {
	return "WhaleAlert"
}

// IsEnabled returns whether provider is enabled
func (wa *WhaleAlertAdapter) IsEnabled() bool {
	return wa.enabled
}

// SupportedChains returns supported blockchains
func (wa *WhaleAlertAdapter) SupportedChains() []string {
	return []string{"bitcoin", "ethereum", "tron", "ripple", "binance-smart-chain"}
}

// GetCost returns cost per request
func (wa *WhaleAlertAdapter) GetCost() float64 {
	// Whale Alert: ~$50-500/month for various plans
	// Assume basic plan $50/month, ~10k requests = $0.005/req
	return 0.005
}

// FetchRecentTransactions fetches recent large transactions
func (wa *WhaleAlertAdapter) FetchRecentTransactions(ctx context.Context, minValueUSD int) ([]models.WhaleTransaction, error) {
	if !wa.enabled {
		return nil, nil
	}

	// Whale Alert API: get transactions from last hour
	start := time.Now().Add(-1 * time.Hour).Unix()

	url := fmt.Sprintf("%s?api_key=%s&start=%d&min_value=%d",
		whaleAlertAPIURL, wa.apiKey, start, minValueUSD)

	req, err := http.NewRequestWithContext(ctx, "GET", url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := wa.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Result       string `json:"result"`
		Transactions []struct {
			From struct {
				Address string `json:"address"`
				Owner   string `json:"owner"`
			} `json:"from"`
			To struct {
				Address string `json:"address"`
				Owner   string `json:"owner"`
			} `json:"to"`
			Blockchain      string  `json:"blockchain"`
			Symbol          string  `json:"symbol"`
			Hash            string  `json:"hash"`
			TransactionType string  `json:"transaction_type"`
			Amount          float64 `json:"amount"`
			AmountUSD       float64 `json:"amount_usd"`
			Timestamp       int64   `json:"timestamp"`
		} `json:"transactions"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	transactions := make([]models.WhaleTransaction, 0)
	for _, tx := range result.Transactions {
		// Calculate impact score based on size and type
		impact := wa.calculateImpact(tx.AmountUSD, tx.TransactionType)

		transactions = append(transactions, models.WhaleTransaction{
			TxHash:          tx.Hash,
			Blockchain:      tx.Blockchain,
			Symbol:          tx.Symbol,
			Amount:          models.NewDecimal(tx.Amount),
			AmountUSD:       models.NewDecimal(tx.AmountUSD),
			FromAddress:     tx.From.Address,
			ToAddress:       tx.To.Address,
			FromOwner:       wa.normalizeOwner(tx.From.Owner),
			ToOwner:         wa.normalizeOwner(tx.To.Owner),
			TransactionType: tx.TransactionType,
			Timestamp:       time.Unix(tx.Timestamp, 0),
			ImpactScore:     impact,
		})
	}

	logger.Debug("fetched whale transactions",
		zap.Int("count", len(transactions)),
	)

	return transactions, nil
}

// calculateImpact calculates impact score based on transaction
func (wa *WhaleAlertProvider) calculateImpact(amountUSD float64, txType string) int {
	baseImpact := 5

	// Size impact
	if amountUSD >= 100_000_000 { // $100M+
		baseImpact = 10
	} else if amountUSD >= 50_000_000 { // $50M+
		baseImpact = 9
	} else if amountUSD >= 10_000_000 { // $10M+
		baseImpact = 8
	} else if amountUSD >= 5_000_000 { // $5M+
		baseImpact = 7
	} else if amountUSD >= 1_000_000 { // $1M+
		baseImpact = 6
	}

	// Type modifiers
	switch txType {
	case "exchange_inflow":
		// Inflow to exchange = potential selling pressure
		baseImpact += 1
	case "exchange_outflow":
		// Outflow from exchange = accumulation (bullish)
		baseImpact += 1
	}

	if baseImpact > 10 {
		baseImpact = 10
	}

	return baseImpact
}

// normalizeOwner normalizes owner name
func (wa *WhaleAlertProvider) normalizeOwner(owner string) string {
	if owner == "" {
		return "unknown wallet"
	}
	return owner
}

// GetSentiment returns sentiment based on transaction type
func GetWhaleTransactionSentiment(tx models.WhaleTransaction) float64 {
	switch tx.TransactionType {
	case "exchange_inflow":
		// Coins moving TO exchange = potential selling = bearish
		return -0.3 - (float64(tx.ImpactScore) * 0.05)

	case "exchange_outflow":
		// Coins moving FROM exchange = accumulation = bullish
		return 0.3 + (float64(tx.ImpactScore) * 0.05)

	case "whale_movement":
		// Unknown wallet to unknown wallet = neutral/slightly bearish
		return -0.1

	default:
		return 0
	}
}
