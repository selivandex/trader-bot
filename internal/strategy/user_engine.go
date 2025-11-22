package strategy

import (
	"github.com/alexanderselivanov/trader/internal/adapters/ai"
	"github.com/alexanderselivanov/trader/internal/adapters/config"
	"github.com/alexanderselivanov/trader/internal/adapters/exchange"
	"github.com/alexanderselivanov/trader/internal/adapters/news"
	"github.com/alexanderselivanov/trader/internal/indicators"
	"github.com/alexanderselivanov/trader/internal/portfolio"
)

// NewUserEngine creates trading engine for specific user
func NewUserEngine(
	userID int64,
	cfg *config.Config,
	ex exchange.Exchange,
	aiEnsemble *ai.Ensemble,
	newsAggregator *news.Aggregator,
	riskManager *RiskManager,
	portfolioTracker *portfolio.UserTracker,
	telegramBot interface{}, // Will be set later
) *Engine {
	engine := &Engine{
		cfg:              cfg,
		exchange:         ex,
		aiEnsemble:       aiEnsemble,
		newsAggregator:   newsAggregator,
		indicatorCalc:    indicators.NewCalculator(),
		riskManager:      riskManager,
		portfolio:        portfolioTracker.Tracker, // Use embedded tracker
		telegram:         nil, // Set separately
		symbol:           cfg.Trading.Symbol,
		decisionInterval: cfg.AI.DecisionInterval,
		isRunning:        false,
	}
	
	return engine
}

