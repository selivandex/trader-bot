package test

import (
	"context"
	"testing"

	"github.com/alexanderselivanov/trader/internal/adapters/ai"
	"github.com/alexanderselivanov/trader/internal/adapters/exchange"
	"github.com/alexanderselivanov/trader/internal/indicators"
	"github.com/alexanderselivanov/trader/internal/risk"
	"github.com/alexanderselivanov/trader/pkg/models"
)

// TestTradingFlow tests complete trading flow with mock components
func TestTradingFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	// Setup mock exchange
	ex := exchange.NewMockExchange("test", 1000)

	// Setup mock AI
	mockAI := &MockAIProvider{
		enabled: true,
		decision: &models.AIDecision{
			Provider:   "mock",
			Action:     models.ActionOpenLong,
			Reason:     "test",
			Size:       models.NewDecimal(0.01),
			StopLoss:   models.NewDecimal(42000),
			TakeProfit: models.NewDecimal(45000),
			Confidence: 80,
		},
	}

	aiEnsemble := ai.NewEnsemble([]ai.Provider{mockAI}, 1)

	// Setup risk manager
	riskManager := &risk.Validator{}

	// Test flow
	t.Run("fetch market data", func(t *testing.T) {
		ticker, err := ex.FetchTicker(ctx, "BTC/USDT")
		if err != nil {
			t.Fatalf("Failed to fetch ticker: %v", err)
		}

		price, _ := ticker.Last.Float64()
		if price <= 0 {
			t.Error("Ticker price should be positive")
		}
	})

	t.Run("calculate indicators", func(t *testing.T) {
		candles, err := ex.FetchOHLCV(ctx, "BTC/USDT", "1h", 100)
		if err != nil {
			t.Fatalf("Failed to fetch candles: %v", err)
		}

		calc := indicators.NewCalculator()
		ind, err := calc.Calculate(candles)
		if err != nil {
			t.Fatalf("Failed to calculate indicators: %v", err)
		}

		if ind.RSI == nil {
			t.Error("RSI should be calculated")
		}
	})

	t.Run("AI decision", func(t *testing.T) {
		prompt := &models.TradingPrompt{
			MarketData: &models.MarketData{
				Symbol: "BTC/USDT",
				Ticker: &models.Ticker{
					Symbol: "BTC/USDT",
					Last:   models.NewDecimal(43000),
				},
			},
			Balance: models.NewDecimal(1000),
			Equity:  models.NewDecimal(1000),
		}

		decision, err := aiEnsemble.Analyze(ctx, prompt)
		if err != nil {
			t.Fatalf("AI analysis failed: %v", err)
		}

		if !decision.Agreement {
			t.Error("Expected consensus")
		}

		if decision.Consensus.Action != models.ActionOpenLong {
			t.Errorf("Expected OPEN_LONG, got %s", decision.Consensus.Action)
		}
	})

	t.Run("validate decision", func(t *testing.T) {
		decision := &models.AIDecision{
			Action:     models.ActionOpenLong,
			Size:       models.NewDecimal(0.01),
			StopLoss:   models.NewDecimal(42000),
			TakeProfit: models.NewDecimal(45000),
			Confidence: 80,
		}

		marketData := &models.MarketData{
			Ticker: &models.Ticker{
				Last: models.NewDecimal(43000),
			},
		}

		err := riskManager.ValidateDecision(decision, marketData)
		if err != nil {
			t.Errorf("Valid decision rejected: %v", err)
		}
	})

	t.Run("execute order", func(t *testing.T) {
		order, err := ex.CreateOrder(ctx, "BTC/USDT", models.TypeMarket, models.SideBuy, 0.01, 0)
		if err != nil {
			t.Fatalf("Failed to create order: %v", err)
		}

		filled, _ := order.Filled.Float64()
		if filled != 0.01 {
			t.Errorf("Expected filled 0.01, got %.4f", filled)
		}

		// Check position created
		positions, err := ex.FetchOpenPositions(ctx)
		if err != nil {
			t.Fatalf("Failed to fetch positions: %v", err)
		}

		if len(positions) != 1 {
			t.Errorf("Expected 1 position, got %d", len(positions))
		}
	})
}

// MockAIProvider for testing
type MockAIProvider struct {
	enabled  bool
	decision *models.AIDecision
}

func (m *MockAIProvider) Analyze(ctx context.Context, prompt *models.TradingPrompt) (*models.AIDecision, error) {
	return m.decision, nil
}

func (m *MockAIProvider) EvaluateNews(ctx context.Context, newsItem *models.NewsItem) error {
	// Mock implementation - just set some default values
	newsItem.Sentiment = 0.0 // neutral sentiment
	newsItem.Relevance = 0.5
	newsItem.Impact = 5 // medium impact
	return nil
}

func (m *MockAIProvider) GetName() string {
	return "mock"
}

func (m *MockAIProvider) GetCost() float64 {
	return 0
}

func (m *MockAIProvider) IsEnabled() bool {
	return m.enabled
}
