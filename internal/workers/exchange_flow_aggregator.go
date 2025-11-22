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

// Start starts the aggregator
func (efa *ExchangeFlowAggregator) Start(ctx context.Context) error {
	logger.Info("exchange flow aggregator starting",
		zap.Duration("interval", efa.interval),
	)
	
	ticker := time.NewTicker(efa.interval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			logger.Info("exchange flow aggregator stopped")
			return ctx.Err()
			
		case <-ticker.C:
			efa.aggregate(ctx)
		}
	}
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

