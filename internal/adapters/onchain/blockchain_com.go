package onchain

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/selivandex/trader-bot/internal/adapters/price"
	"github.com/selivandex/trader-bot/pkg/logger"
	"github.com/selivandex/trader-bot/pkg/models"
)

const blockchainAPIURL = "https://blockchain.info"

// BlockchainComAdapter implements OnChainProvider for Blockchain.com API (BTC only, free)
type BlockchainComAdapter struct {
	enabled       bool
	client        *http.Client
	priceProvider price.PriceProvider
}

// NewBlockchainComAdapter creates new Blockchain.com adapter
func NewBlockchainComAdapter(enabled bool, priceProvider price.PriceProvider) *BlockchainComAdapter {
	if priceProvider == nil {
		priceProvider = price.NewCoinGeckoProvider()
	}

	return &BlockchainComAdapter{
		enabled:       enabled,
		client:        &http.Client{Timeout: 10 * time.Second},
		priceProvider: priceProvider,
	}
}

func (bc *BlockchainComAdapter) GetName() string {
	return "Blockchain.com"
}

func (bc *BlockchainComAdapter) IsEnabled() bool {
	return bc.enabled
}

func (bc *BlockchainComAdapter) SupportedChains() []string {
	return []string{"bitcoin"}
}

func (bc *BlockchainComAdapter) GetCost() float64 {
	return 0.0 // Free API
}

// FetchRecentTransactions fetches recent BTC transactions
func (bc *BlockchainComAdapter) FetchRecentTransactions(ctx context.Context, minValueUSD int) ([]models.WhaleTransaction, error) {
	if !bc.enabled {
		return nil, nil
	}

	// Fetch unconfirmed transactions (mempool)
	url := fmt.Sprintf("%s/unconfirmed-transactions?format=json", blockchainAPIURL)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := bc.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Txs []struct {
			Hash   string `json:"hash"`
			Time   int64  `json:"time"`
			Size   int    `json:"size"`
			Inputs []struct {
				PrevOut struct {
					Addr  string `json:"addr"`
					Value int64  `json:"value"` // Satoshis
				} `json:"prev_out"`
			} `json:"inputs"`
			Out []struct {
				Addr  string `json:"addr"`
				Value int64  `json:"value"` // Satoshis
			} `json:"out"`
		} `json:"txs"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Get current BTC price
	btcPriceUSD, err := bc.priceProvider.GetPrice(ctx, "BTC")
	if err != nil {
		return nil, fmt.Errorf("failed to get BTC price: %w", err)
	}

	const satoshisToBTC = 0.00000001
	transactions := []models.WhaleTransaction{}

	for _, tx := range result.Txs {
		// Calculate total value
		totalSatoshis := int64(0)
		for _, out := range tx.Out {
			totalSatoshis += out.Value
		}

		btcAmount := float64(totalSatoshis) * satoshisToBTC
		usdAmount := btcAmount * btcPriceUSD

		// Filter by minimum value
		if usdAmount < float64(minValueUSD) {
			continue
		}

		// Classify transaction type (simplified)
		fromAddr := "unknown"
		toAddr := "unknown"
		if len(tx.Inputs) > 0 {
			fromAddr = tx.Inputs[0].PrevOut.Addr
		}
		if len(tx.Out) > 0 {
			toAddr = tx.Out[0].Addr
		}

		// Determine transaction type
		transactionType := "whale_movement"
		fromOwner := classifyBTCAddress(fromAddr)
		toOwner := classifyBTCAddress(toAddr)

		if fromOwner != "unknown" && toOwner == "unknown" {
			transactionType = "exchange_outflow"
		} else if fromOwner == "unknown" && toOwner != "unknown" {
			transactionType = "exchange_inflow"
		}

		// Calculate impact score (1-10)
		impactScore := calculateImpactScore(usdAmount)

		transactions = append(transactions, models.WhaleTransaction{
			TxHash:          tx.Hash,
			Blockchain:      "bitcoin",
			Symbol:          "BTC",
			Amount:          models.NewDecimal(btcAmount),
			AmountUSD:       models.NewDecimal(usdAmount),
			FromAddress:     fromAddr,
			ToAddress:       toAddr,
			FromOwner:       fromOwner,
			ToOwner:         toOwner,
			TransactionType: transactionType,
			Timestamp:       time.Unix(tx.Time, 0),
			ImpactScore:     impactScore,
		})
	}

	logger.Debug("blockchain.com transactions fetched",
		zap.Int("total", len(transactions)),
		zap.Int("filtered", len(result.Txs)),
	)

	return transactions, nil
}

// classifyBTCAddress classifies known exchange addresses
func classifyBTCAddress(addr string) string {
	// Known exchange cold wallet patterns (simplified)
	// In production, maintain database of known addresses
	knownExchanges := map[string]string{
		"bc1q": "unknown", // Binance often uses bc1q
		"1Nd":  "unknown", // Example pattern
		"3":    "unknown", // P2SH addresses
	}

	if len(addr) == 0 {
		return "unknown"
	}

	// Check prefixes
	for prefix, owner := range knownExchanges {
		if len(addr) >= len(prefix) && addr[:len(prefix)] == prefix {
			return owner
		}
	}

	return "unknown"
}

// calculateImpactScore calculates impact score 1-10 based on USD value
func calculateImpactScore(usdValue float64) int {
	switch {
	case usdValue >= 50_000_000: // $50M+
		return 10
	case usdValue >= 20_000_000: // $20M+
		return 9
	case usdValue >= 10_000_000: // $10M+
		return 8
	case usdValue >= 5_000_000: // $5M+
		return 7
	case usdValue >= 2_000_000: // $2M+
		return 6
	case usdValue >= 1_000_000: // $1M+
		return 5
	default:
		return 4
	}
}
