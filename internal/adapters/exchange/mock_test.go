package exchange

import (
	"context"
	"testing"

	"github.com/alexanderselivanov/trader/pkg/models"
)

func TestMockExchange_FetchTicker(t *testing.T) {
	ex := NewMockExchange("test", 1000)
	ctx := context.Background()
	
	ticker, err := ex.FetchTicker(ctx, "BTC/USDT")
	if err != nil {
		t.Fatalf("Failed to fetch ticker: %v", err)
	}
	
	if ticker.Symbol != "BTC/USDT" {
		t.Errorf("Expected symbol BTC/USDT, got %s", ticker.Symbol)
	}
	
	if ticker.Last.Float64() <= 0 {
		t.Error("Price should be positive")
	}
	
	if ticker.Bid.Float64() >= ticker.Ask.Float64() {
		t.Error("Bid should be less than Ask")
	}
}

func TestMockExchange_CreateOrder(t *testing.T) {
	ex := NewMockExchange("test", 1000)
	ctx := context.Background()
	
	// Create buy order
	order, err := ex.CreateOrder(ctx, "BTC/USDT", models.TypeMarket, models.SideBuy, 0.01, 0)
	if err != nil {
		t.Fatalf("Failed to create order: %v", err)
	}
	
	if order.ID == "" {
		t.Error("Order ID should not be empty")
	}
	
	if order.Symbol != "BTC/USDT" {
		t.Errorf("Expected symbol BTC/USDT, got %s", order.Symbol)
	}
	
	if order.Side != models.SideBuy {
		t.Errorf("Expected buy side, got %s", order.Side)
	}
	
	if order.Amount.Float64() != 0.01 {
		t.Errorf("Expected amount 0.01, got %.4f", order.Amount.Float64())
	}
	
	// Check position was created
	positions, err := ex.FetchOpenPositions(ctx)
	if err != nil {
		t.Fatalf("Failed to fetch positions: %v", err)
	}
	
	if len(positions) != 1 {
		t.Errorf("Expected 1 position, got %d", len(positions))
	}
	
	if positions[0].Side != models.PositionLong {
		t.Errorf("Expected long position, got %s", positions[0].Side)
	}
}

func TestMockExchange_PositionManagement(t *testing.T) {
	ex := NewMockExchange("test", 1000)
	ctx := context.Background()
	
	// Open long position
	ex.CreateOrder(ctx, "BTC/USDT", models.TypeMarket, models.SideBuy, 0.01, 0)
	
	positions, _ := ex.FetchOpenPositions(ctx)
	if len(positions) != 1 {
		t.Fatalf("Expected 1 position")
	}
	
	// Close position
	ex.CreateOrder(ctx, "BTC/USDT", models.TypeMarket, models.SideSell, 0.01, 0)
	
	positions, _ = ex.FetchOpenPositions(ctx)
	if len(positions) != 0 {
		t.Errorf("Expected position to be closed, got %d positions", len(positions))
	}
}

func TestMockExchange_FetchOHLCV(t *testing.T) {
	ex := NewMockExchange("test", 1000)
	ctx := context.Background()
	
	candles, err := ex.FetchOHLCV(ctx, "BTC/USDT", "1h", 100)
	if err != nil {
		t.Fatalf("Failed to fetch OHLCV: %v", err)
	}
	
	if len(candles) != 100 {
		t.Errorf("Expected 100 candles, got %d", len(candles))
	}
	
	for i, candle := range candles {
		if candle.High.Float64() < candle.Low.Float64() {
			t.Errorf("Candle %d: High should be >= Low", i)
		}
		
		if candle.Open.Float64() <= 0 || candle.Close.Float64() <= 0 {
			t.Errorf("Candle %d: Prices should be positive", i)
		}
	}
}

