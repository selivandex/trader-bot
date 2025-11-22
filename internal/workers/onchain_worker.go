package workers

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/selivandex/trader-bot/internal/adapters/onchain"
	"github.com/selivandex/trader-bot/pkg/logger"
	"github.com/selivandex/trader-bot/pkg/models"
)

// OnChainWorker monitors blockchain activity
type OnChainWorker struct {
	repo        *Repository
	provider    onchain.OnChainProvider // Interface instead of concrete type
	interval    time.Duration
	minValueUSD int
}

// NewOnChainWorker creates new on-chain worker
func NewOnChainWorker(
	repo *Repository,
	provider onchain.OnChainProvider,
	interval time.Duration,
	minValueUSD int,
) *OnChainWorker {
	return &OnChainWorker{
		repo:        repo,
		provider:    provider,
		interval:    interval,
		minValueUSD: minValueUSD,
	}
}

// Start starts the on-chain monitoring worker
func (ow *OnChainWorker) Start(ctx context.Context) error {
	logger.Info("on-chain worker starting",
		zap.Duration("interval", ow.interval),
		zap.Int("min_value_usd", ow.minValueUSD),
	)

	// Run immediately
	ow.fetchAndCache(ctx)

	ticker := time.NewTicker(ow.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("on-chain worker stopped")
			return ctx.Err()

		case <-ticker.C:
			ow.fetchAndCache(ctx)
		}
	}
}

// fetchAndCache fetches whale transactions and caches them
func (ow *OnChainWorker) fetchAndCache(ctx context.Context) {
	if !ow.provider.IsEnabled() {
		return
	}

	logger.Debug("fetching whale transactions from provider",
		zap.String("provider", ow.provider.GetName()),
	)

	startTime := time.Now()

	// Fetch transactions
	transactions, err := ow.provider.FetchRecentTransactions(ctx, ow.minValueUSD)
	if err != nil {
		logger.Error("failed to fetch whale transactions", zap.Error(err))
		return
	}

	if len(transactions) == 0 {
		logger.Debug("no whale transactions")
		return
	}

	// Save to database
	saved := 0
	highImpact := 0

	for _, tx := range transactions {
		if err := ow.saveTransaction(ctx, &tx); err != nil {
			logger.Warn("failed to save transaction",
				zap.String("hash", tx.TxHash),
				zap.Error(err),
			)
			continue
		}
		saved++

		if tx.ImpactScore >= 7 {
			highImpact++

			// Log high impact transactions
			logger.Warn("HIGH IMPACT whale transaction",
				zap.String("type", tx.TransactionType),
				zap.String("symbol", tx.Symbol),
				zap.Float64("amount_usd", models.ToFloat64(tx.AmountUSD)),
				zap.String("from", tx.FromOwner),
				zap.String("to", tx.ToOwner),
				zap.Int("impact", tx.ImpactScore),
			)
		}
	}

	duration := time.Since(startTime)

	logger.Info("whale transactions cached",
		zap.Int("total", len(transactions)),
		zap.Int("saved", saved),
		zap.Int("high_impact", highImpact),
		zap.Duration("duration", duration),
	)
}

// saveTransaction saves whale transaction to database
func (ow *OnChainWorker) saveTransaction(ctx context.Context, tx *models.WhaleTransaction) error {
	return ow.repo.SaveWhaleTransaction(ctx, tx)
}

// GetRecentSummary gets on-chain summary for symbol
func (ow *OnChainWorker) GetRecentSummary(ctx context.Context, symbol string) (*models.OnChainSummary, error) {
	// Get recent whale movements
	whaleMovements, err := ow.repo.GetRecentWhaleTransactions(ctx, symbol, 6*time.Hour, 10)
	if err != nil {
		return nil, err
	}

	// Get exchange flow direction
	netFlow, err := ow.repo.GetExchangeNetFlow(ctx, symbol, 6*time.Hour)
	if err != nil {
		netFlow = 0
	}

	flowDirection := "balanced"
	if netFlow > 10 {
		flowDirection = "inflow" // Bearish
	} else if netFlow < -10 {
		flowDirection = "outflow" // Bullish
	}

	summary := &models.OnChainSummary{
		Symbol:                symbol,
		ExchangeFlowDirection: flowDirection,
		NetExchangeFlow:       models.NewDecimal(netFlow),
		RecentWhaleMovements:  whaleMovements,
		UpdatedAt:             time.Now(),
	}

	summary.WhaleActivity = summary.GetWhaleActivityLevel()

	return summary, nil
}
