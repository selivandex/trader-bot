package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/alexanderselivanov/trader/internal/adapters/ai"
	"github.com/alexanderselivanov/trader/internal/adapters/config"
	"github.com/alexanderselivanov/trader/internal/adapters/exchange"
	"github.com/alexanderselivanov/trader/internal/backtest"
	"github.com/alexanderselivanov/trader/pkg/logger"
)

func main() {
	// Parse flags
	var (
		symbol      = flag.String("symbol", "BTC/USDT", "Trading symbol")
		fromDate    = flag.String("from", "2024-01-01", "Start date (YYYY-MM-DD)")
		toDate      = flag.String("to", "2024-03-01", "End date (YYYY-MM-DD)")
		balance     = flag.Float64("balance", 1000, "Initial balance")
		timeframe   = flag.String("timeframe", "1h", "Candle timeframe")
		exchange    = flag.String("exchange", "binance", "Exchange (binance/bybit)")
	)
	
	flag.Parse()
	
	// Initialize logger
	if err := logger.Init("info", ""); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	
	// Parse dates
	startDate, err := time.Parse("2006-01-02", *fromDate)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid start date: %v\n", err)
		os.Exit(1)
	}
	
	endDate, err := time.Parse("2006-01-02", *toDate)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid end date: %v\n", err)
		os.Exit(1)
	}
	
	// Load config
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}
	
	// Create exchange adapter
	var ex exchange.Exchange
	
	switch *exchange {
	case "binance":
		ex, err = exchange.NewBinanceAdapter(&cfg.Exchanges.Binance)
	case "bybit":
		ex, err = exchange.NewBybitAdapter(&cfg.Exchanges.Bybit)
	default:
		// Use mock for testing
		ex = exchange.NewMockExchange(*exchange, *balance)
	}
	
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create exchange: %v\n", err)
		os.Exit(1)
	}
	defer ex.Close()
	
	// Initialize AI providers
	var aiProviders []ai.Provider
	
	if cfg.AI.DeepSeek.Enabled {
		aiProviders = append(aiProviders, ai.NewDeepSeekProvider(&cfg.AI.DeepSeek))
	}
	
	if cfg.AI.Claude.Enabled {
		aiProviders = append(aiProviders, ai.NewClaudeProvider(&cfg.AI.Claude))
	}
	
	if len(aiProviders) == 0 {
		fmt.Fprintf(os.Stderr, "No AI providers configured\n")
		os.Exit(1)
	}
	
	aiEnsemble := ai.NewEnsemble(aiProviders, cfg.AI.EnsembleMinConsensus)
	
	// Create backtest config
	backtestCfg := &backtest.Config{
		Symbol:          *symbol,
		StartDate:       startDate,
		EndDate:         endDate,
		InitialBalance:  *balance,
		Timeframe:       *timeframe,
		MaxLeverage:     3,
		StopLossPercent: 2.0,
	}
	
	// Run backtest
	fmt.Printf("\n🔬 Running backtest for %s...\n", *symbol)
	fmt.Printf("Period: %s to %s\n", *fromDate, *toDate)
	fmt.Printf("Initial Balance: $%.2f\n\n", *balance)
	
	engine := backtest.NewEngine(ex, aiEnsemble, backtestCfg)
	
	ctx := context.Background()
	result, err := engine.Run(ctx, backtestCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Backtest failed: %v\n", err)
		os.Exit(1)
	}
	
	// Print results
	result.Print()
	
	// Show recommendation
	fmt.Println("\nRECOMMENDATION:")
	if result.ROI > 10 && result.WinRate > 50 && result.SharpeRatio > 1.0 {
		fmt.Println("✅ GOOD - Strategy shows promise")
	} else if result.ROI < 0 || result.WinRate < 40 {
		fmt.Println("❌ POOR - Strategy needs improvement")
	} else {
		fmt.Println("⚠️  MEDIOCRE - More testing needed")
	}
	
	fmt.Println("\n💡 TIP: Run backtest for at least 3 months before live trading")
}

