package price

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const coingeckoAPIURL = "https://api.coingecko.com/api/v3"

// CoinGeckoProvider implements PriceProvider using CoinGecko API (free, no API key needed)
type CoinGeckoProvider struct {
	client *http.Client
	cache  map[string]cachedPrice
}

type cachedPrice struct {
	timestamp time.Time
	price     float64
}

// NewCoinGeckoProvider creates new CoinGecko price provider
func NewCoinGeckoProvider() *CoinGeckoProvider {
	return &CoinGeckoProvider{
		client: &http.Client{Timeout: 10 * time.Second},
		cache:  make(map[string]cachedPrice),
	}
}

func (cg *CoinGeckoProvider) GetName() string {
	return "CoinGecko"
}

// GetPrice returns current price in USD (from API, cached in-memory)
func (cg *CoinGeckoProvider) GetPrice(ctx context.Context, symbol string) (float64, error) {
	// Check in-memory cache first
	if cached, ok := cg.cache[symbol]; ok {
		if time.Since(cached.timestamp) < 5*time.Minute {
			return cached.price, nil
		}
	}

	// Fetch from API
	coinID := mapSymbolToCoinGeckoID(symbol)

	url := fmt.Sprintf("%s/simple/price?ids=%s&vs_currencies=usd", coingeckoAPIURL, coinID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, http.NoBody)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := cg.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]struct {
		USD float64 `json:"usd"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("failed to decode response: %w", err)
	}

	priceData, ok := result[coinID]
	if !ok {
		return 0, fmt.Errorf("price not found for %s", symbol)
	}
	// Cache in memory
	cg.cache[symbol] = cachedPrice{
		price:     priceData.USD,
		timestamp: time.Now(),
	}

	return priceData.USD, nil
}

// GetPrices returns multiple prices at once
func (cg *CoinGeckoProvider) GetPrices(ctx context.Context, symbols []string) (map[string]float64, error) {
	prices := make(map[string]float64)

	// Convert to CoinGecko IDs
	coinIDs := []string{}
	symbolToID := make(map[string]string)

	for _, symbol := range symbols {
		coinID := mapSymbolToCoinGeckoID(symbol)
		coinIDs = append(coinIDs, coinID)
		symbolToID[coinID] = symbol
	}

	// Batch request
	url := fmt.Sprintf("%s/simple/price?ids=%s&vs_currencies=usd",
		coingeckoAPIURL, strings.Join(coinIDs, ","))

	req, err := http.NewRequestWithContext(ctx, "GET", url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := cg.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]struct {
		USD float64 `json:"usd"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Map back to original symbols
	for coinID, priceData := range result {
		if symbol, ok := symbolToID[coinID]; ok {
			prices[symbol] = priceData.USD

			// Cache in memory
			cg.cache[symbol] = cachedPrice{
				price:     priceData.USD,
				timestamp: time.Now(),
			}
		}
	}

	return prices, nil
}

// mapSymbolToCoinGeckoID maps trading symbols to CoinGecko IDs
func mapSymbolToCoinGeckoID(symbol string) string {
	symbolMap := map[string]string{
		"BTC":  "bitcoin",
		"ETH":  "ethereum",
		"USDT": "tether",
		"USDC": "usd-coin",
		"BNB":  "binancecoin",
		"SOL":  "solana",
		"XRP":  "ripple",
		"ADA":  "cardano",
		"DOGE": "dogecoin",
	}

	if id, ok := symbolMap[symbol]; ok {
		return id
	}

	// Default: lowercase
	return strings.ToLower(symbol)
}
