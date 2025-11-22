package ai

import (
	"fmt"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/alexanderselivanov/trader/pkg/models"
)

func TestBuildUserPrompt(t *testing.T) {
	// Create realistic market data
	prompt := &models.TradingPrompt{
		MarketData: &models.MarketData{
			Symbol: "BTC/USDT",
			Ticker: &models.Ticker{
				Symbol:    "BTC/USDT",
				Last:      models.NewDecimal(43250.50),
				Bid:       models.NewDecimal(43248.00),
				Ask:       models.NewDecimal(43252.00),
				High24h:   models.NewDecimal(44100.00),
				Low24h:    models.NewDecimal(42800.00),
				Volume24h: models.NewDecimal(1250000000),
				Change24h: models.NewDecimal(2.3),
			},
			Candles: map[string][]models.Candle{
				"5m": {
					{Timestamp: time.Now().Add(-10 * time.Minute), Close: models.NewDecimal(43100)},
					{Timestamp: time.Now().Add(-5 * time.Minute), Close: models.NewDecimal(43250)},
				},
				"15m": {
					{Timestamp: time.Now().Add(-30 * time.Minute), Close: models.NewDecimal(43050)},
					{Timestamp: time.Now().Add(-15 * time.Minute), Close: models.NewDecimal(43245)},
				},
				"1h": {
					{Timestamp: time.Now().Add(-2 * time.Hour), Close: models.NewDecimal(42900)},
					{Timestamp: time.Now().Add(-1 * time.Hour), Close: models.NewDecimal(43100)},
				},
				"4h": {
					{Timestamp: time.Now().Add(-8 * time.Hour), Close: models.NewDecimal(42200)},
					{Timestamp: time.Now().Add(-4 * time.Hour), Close: models.NewDecimal(42900)},
				},
			},
			Indicators: &models.TechnicalIndicators{
				RSI: map[string]decimal.Decimal{
					"5m":  models.NewDecimal(62.5),
					"15m": models.NewDecimal(58.3),
					"1h":  models.NewDecimal(55.2),
					"4h":  models.NewDecimal(68.7),
				},
				MACD: &models.MACDIndicator{
					MACD:      models.NewDecimal(125.30),
					Signal:    models.NewDecimal(115.20),
					Histogram: models.NewDecimal(10.10),
				},
				BollingerBands: &models.BollingerBandsIndicator{
					Upper:  models.NewDecimal(44200.00),
					Middle: models.NewDecimal(43000.00),
					Lower:  models.NewDecimal(41800.00),
				},
				Volume: &models.VolumeIndicator{
					Current: models.NewDecimal(1250000),
					Average: models.NewDecimal(980000),
					Ratio:   models.NewDecimal(1.28),
				},
			},
			NewsSummary: &models.NewsSummary{
				TotalItems:       47,
				PositiveCount:    28,
				NegativeCount:    12,
				NeutralCount:     7,
				AverageSentiment: 0.35,
				OverallSentiment: "bullish",
				RecentNews: []models.NewsItem{
					{
						Title:       "BlackRock BTC ETF sees record inflows",
						Sentiment:   0.68,
						Impact:      9,
						Urgency:     "IMMEDIATE",
						PublishedAt: time.Now().Add(-1 * time.Hour),
					},
					{
						Title:       "Multiple sources confirm institutional buying",
						Sentiment:   0.52,
						Impact:      7,
						Urgency:     "HOURS",
						PublishedAt: time.Now().Add(-2 * time.Hour),
					},
				},
			},
			OnChainData: &models.OnChainSummary{
				Symbol:                "BTC/USDT",
				WhaleActivity:         "HIGH",
				ExchangeFlowDirection: "outflow",
				NetExchangeFlow:       models.NewDecimal(-185_500_000),
				RecentWhaleMovements: []models.WhaleTransaction{
					{
						TxHash:          "abc123",
						Symbol:          "BTC",
						AmountUSD:       models.NewDecimal(12_500_000),
						TransactionType: "exchange_outflow",
						FromOwner:       "binance",
						ToOwner:         "unknown",
						ImpactScore:     8,
						Timestamp:       time.Now().Add(-25 * time.Minute),
					},
					{
						TxHash:          "xyz789",
						Symbol:          "BTC",
						AmountUSD:       models.NewDecimal(8_200_000),
						TransactionType: "exchange_outflow",
						FromOwner:       "coinbase",
						ToOwner:         "unknown",
						ImpactScore:     7,
						Timestamp:       time.Now().Add(-1 * time.Hour),
					},
				},
			},
			FundingRate: models.NewDecimal(0.0085),
		},
		CurrentPosition: nil, // No position
		Balance:         models.NewDecimal(1000),
		Equity:          models.NewDecimal(1000),
		DailyPnL:        models.NewDecimal(0),
	}

	// Build prompt
	userPrompt := buildUserPrompt(prompt)

	// Print the full prompt
	fmt.Println("=== ПОЛНЫЙ AI ПРОМПТ ===")
	fmt.Println(userPrompt)
	fmt.Println("=== КОНЕЦ ПРОМПТА ===")

	// Verify key elements are present
	if len(userPrompt) < 500 {
		t.Errorf("Prompt too short: %d chars", len(userPrompt))
	}

	// Check for multi-timeframe
	if !containsAll(userPrompt, []string{"5m", "15m", "1h", "4h"}) {
		t.Error("Missing timeframe data")
	}

	// Check for indicators
	if !containsAll(userPrompt, []string{"RSI", "MACD", "Bollinger Bands"}) {
		t.Error("Missing indicators")
	}

	// Check for news
	if !containsAll(userPrompt, []string{"NEWS", "BlackRock", "bullish"}) {
		t.Error("Missing news data")
	}

	// Check for on-chain
	if !containsAll(userPrompt, []string{"ON-CHAIN", "OUTFLOW", "exchange_outflow"}) {
		t.Error("Missing on-chain data")
	}
}

func containsAll(text string, substrings []string) bool {
	for _, s := range substrings {
		if !contains(text, s) {
			return false
		}
	}
	return true
}

func contains(text, substring string) bool {
	return len(text) >= len(substring) && findSubstring(text, substring)
}

func findSubstring(text, substring string) bool {
	for i := 0; i <= len(text)-len(substring); i++ {
		match := true
		for j := 0; j < len(substring); j++ {
			if text[i+j] != substring[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func TestBuildSystemPrompt(t *testing.T) {
	// Create strategy params
	params := &models.StrategyParameters{
		MaxPositionPercent:     30.0,
		MaxLeverage:            3,
		StopLossPercent:        2.0,
		TakeProfitPercent:      5.0,
		MinConfidenceThreshold: 70,
	}

	systemPrompt := buildSystemPrompt(params)

	fmt.Println("=== SYSTEM PROMPT (роль агента) ===")
	fmt.Println(systemPrompt)
	fmt.Println("=== КОНЕЦ SYSTEM PROMPT ===")

	// Verify it contains strategy params
	if !containsAll(systemPrompt, []string{"30%", "3x", "2.0%", "5.0%", "70%"}) {
		t.Error("Missing strategy parameters")
	}

	// Verify it contains instructions
	if !containsAll(systemPrompt, []string{"JSON", "HOLD", "OPEN_LONG", "stop-loss"}) {
		t.Error("Missing trading instructions")
	}
}
