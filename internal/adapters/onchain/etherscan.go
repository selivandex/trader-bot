package onchain

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"go.uber.org/zap"

	"github.com/alexanderselivanov/trader/pkg/logger"
	"github.com/alexanderselivanov/trader/pkg/models"
)

const (
	etherscanAPIURL      = "https://api.etherscan.io/api"
	usdtContractAddress  = "0xdac17f958d2ee523a2206206994597c13d831ec7"
	usdcContractAddress  = "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48"
)

// EtherscanAdapter implements OnChainProvider for Etherscan API (USDT/USDC/ETH, free)
type EtherscanAdapter struct {
	apiKey  string
	enabled bool
	client  *http.Client
}

// NewEtherscanAdapter creates new Etherscan adapter
func NewEtherscanAdapter(apiKey string, enabled bool) *EtherscanAdapter {
	return &EtherscanAdapter{
		apiKey:  apiKey,
		enabled: enabled && apiKey != "",
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

func (es *EtherscanAdapter) GetName() string {
	return "Etherscan"
}

func (es *EtherscanAdapter) IsEnabled() bool {
	return es.enabled
}

func (es *EtherscanAdapter) SupportedChains() []string {
	return []string{"ethereum"}
}

func (es *EtherscanAdapter) GetCost() float64 {
	return 0.0 // Free with API key (5 req/sec limit)
}

// FetchRecentTransactions fetches large USDT/USDC/ETH transactions
func (es *EtherscanAdapter) FetchRecentTransactions(ctx context.Context, minValueUSD int) ([]models.WhaleTransaction, error) {
	if !es.enabled {
		return nil, nil
	}
	
	// Fetch USDT transfers (most important for crypto markets)
	usdtTxs, err := es.fetchTokenTransfers(ctx, usdtContractAddress, "USDT", minValueUSD)
	if err != nil {
		logger.Warn("failed to fetch USDT transactions", zap.Error(err))
		return nil, err
	}
	
	// TODO: Add USDC, ETH if needed
	// usdcTxs, _ := es.fetchTokenTransfers(ctx, usdcContractAddress, "USDC", minValueUSD)
	// ethTxs, _ := es.fetchETHTransactions(ctx, minValueUSD)
	
	return usdtTxs, nil
}

// fetchTokenTransfers fetches ERC-20 token transfers
func (es *EtherscanAdapter) fetchTokenTransfers(ctx context.Context, contractAddress, symbol string, minValueUSD int) ([]models.WhaleTransaction, error) {
	// Get recent token transfers
	// Note: Etherscan free API is limited - can only get transfers for specific address
	// For whale monitoring, would need premium API or different approach
	
	url := fmt.Sprintf("%s?module=account&action=tokentx&contractaddress=%s&apikey=%s&sort=desc",
		etherscanAPIURL, contractAddress, es.apiKey)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	resp, err := es.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}
	
	var result struct {
		Status  string `json:"status"`
		Message string `json:"message"`
		Result  []struct {
			Hash        string `json:"hash"`
			From        string `json:"from"`
			To          string `json:"to"`
			Value       string `json:"value"`
			TokenSymbol string `json:"tokenSymbol"`
			TimeStamp   string `json:"timeStamp"`
		} `json:"result"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	if result.Status != "1" {
		return nil, fmt.Errorf("API returned error: %s", result.Message)
	}
	
	transactions := []models.WhaleTransaction{}
	
	for _, tx := range result.Result {
		// Parse value (in smallest unit: USDT has 6 decimals)
		value, _ := strconv.ParseFloat(tx.Value, 64)
		usdtAmount := value / 1_000_000 // Convert from smallest unit
		
		// Filter by minimum
		if usdtAmount < float64(minValueUSD) {
			continue
		}
		
		// Classify addresses
		fromOwner := classifyETHAddress(tx.From)
		toOwner := classifyETHAddress(tx.To)
		
		transactionType := "whale_movement"
		if fromOwner != "unknown" && toOwner == "unknown" {
			transactionType = "exchange_outflow"
		} else if fromOwner == "unknown" && toOwner != "unknown" {
			transactionType = "exchange_inflow"
		}
		
		timestamp, _ := strconv.ParseInt(tx.TimeStamp, 10, 64)
		
		transactions = append(transactions, models.WhaleTransaction{
			TxHash:          tx.Hash,
			Blockchain:      "ethereum",
			Symbol:          symbol,
			Amount:          models.NewDecimal(usdtAmount),
			AmountUSD:       models.NewDecimal(usdtAmount), // USDT = 1:1 USD
			FromAddress:     tx.From,
			ToAddress:       tx.To,
			FromOwner:       fromOwner,
			ToOwner:         toOwner,
			TransactionType: transactionType,
			Timestamp:       time.Unix(timestamp, 0),
			ImpactScore:     calculateImpactScore(usdtAmount),
		})
	}
	
	logger.Debug("etherscan transactions fetched",
		zap.String("symbol", symbol),
		zap.Int("count", len(transactions)),
	)
	
	return transactions, nil
}

// classifyETHAddress classifies known Ethereum addresses
func classifyETHAddress(addr string) string {
	// Known exchange hot/cold wallets
	// In production, maintain database of known addresses
	knownAddresses := map[string]string{
		"0x28c6c06298d514db089934071355e5743bf21d60": "binance_hot",
		"0x21a31ee1afc51d94c2efccaa2092ad1028285549": "binance_cold",
		// Add more...
	}
	
	if owner, ok := knownAddresses[addr]; ok {
		return owner
	}
	
	return "unknown"
}

