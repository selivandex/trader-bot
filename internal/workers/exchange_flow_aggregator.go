package workers

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/selivandex/trader-bot/pkg/logger"
)

// ExchangeFlowAggregator calculates exchange flows from whale transactions
type ExchangeFlowAggregator struct {
	repo     *Repository
	interval time.Duration
}

// NewExchangeFlowAggregator creates new exchange flow aggregator
func NewExchangeFlowAggregator(repo *Repository, interval time.Duration) *ExchangeFlowAggregator {
	return &ExchangeFlowAggregator{
		repo:     repo,
		interval: interval,
	}
}

// Name returns worker name
func (efa *ExchangeFlowAggregator) Name() string {
	return "exchange_flow_aggregator"
}

// Run executes one iteration - aggregates exchange flows
// Called periodically by pkg/worker.PeriodicWorker
func (efa *ExchangeFlowAggregator) Run(ctx context.Context) error {
	efa.aggregate(ctx)
	return nil
}

// aggregate calculates exchange flows from whale transactions
func (efa *ExchangeFlowAggregator) aggregate(ctx context.Context) {
	logger.Debug("aggregating exchange flows...")

	// Aggregate flows for last hour
	timestamp := time.Now().Truncate(time.Hour)

	if err := efa.repo.AggregateExchangeFlows(ctx, timestamp); err != nil {
		logger.Error("failed to aggregate exchange flows", zap.Error(err))
		return
	}

	logger.Debug("exchange flows aggregated",
		zap.Time("timestamp", timestamp),
	)
}
