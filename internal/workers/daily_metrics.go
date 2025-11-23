package workers

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/selivandex/trader-bot/pkg/logger"
)

// DailyMetricsWorker calculates daily performance metrics
type DailyMetricsWorker struct {
	repo *Repository
}

// NewDailyMetricsWorker creates new daily metrics worker
func NewDailyMetricsWorker(repo *Repository) *DailyMetricsWorker {
	return &DailyMetricsWorker{repo: repo}
}

// Name returns worker name
func (dmw *DailyMetricsWorker) Name() string {
	return "daily_metrics"
}

// Run executes one iteration - calculates metrics for yesterday
// Called periodically (24h) by pkg/worker.PeriodicWorker
func (dmw *DailyMetricsWorker) Run(ctx context.Context) error {
	yesterday := time.Now().AddDate(0, 0, -1)
	dmw.calculateForAllUsers(ctx, yesterday)
	return nil
}

// calculateForAllUsers calculates metrics for all users
func (dmw *DailyMetricsWorker) calculateForAllUsers(ctx context.Context, date time.Time) {
	logger.Info("calculating daily metrics for all users",
		zap.Time("date", date),
	)

	// Get all active users
	userIDs, err := dmw.repo.GetActiveUserIDs(ctx)
	if err != nil {
		logger.Error("failed to get users", zap.Error(err))
		return
	}

	calculated := 0
	for _, userID := range userIDs {
		if err := dmw.calculateForUser(ctx, userID, date); err != nil {
			logger.Warn("failed to calculate metrics for user",
				zap.Int64("user_id", userID),
				zap.Error(err),
			)
			continue
		}
		calculated++
	}

	logger.Info("daily metrics calculated",
		zap.Int("users", calculated),
		zap.Time("date", date),
	)
}

// calculateForUser calculates metrics for specific user
func (dmw *DailyMetricsWorker) calculateForUser(ctx context.Context, userID int64, date time.Time) error {
	return dmw.repo.CalculateDailyMetrics(ctx, userID, date)
}
