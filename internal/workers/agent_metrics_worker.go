package workers

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/selivandex/trader-bot/internal/adapters/clickhouse"
	"github.com/selivandex/trader-bot/internal/agents"
	"github.com/selivandex/trader-bot/pkg/logger"
	"github.com/selivandex/trader-bot/pkg/models"
)

// AgentMetricsWorker collects agent performance metrics and saves to ClickHouse
type AgentMetricsWorker struct {
	agentManager   *agents.AgenticManager
	agentRepo      *agents.Repository
	clickhouseRepo *clickhouse.Repository
	batchWriter    *clickhouse.BatchWriter // Batch writer for efficient inserts
	interval       time.Duration
}

// NewAgentMetricsWorker creates new metrics worker
func NewAgentMetricsWorker(
	agentManager *agents.AgenticManager,
	agentRepo *agents.Repository,
	clickhouseRepo *clickhouse.Repository,
	interval time.Duration,
) *AgentMetricsWorker {
	w := &AgentMetricsWorker{
		agentManager:   agentManager,
		agentRepo:      agentRepo,
		clickhouseRepo: clickhouseRepo,
		interval:       interval,
	}

	// Initialize batch writer for metrics (flush every 30s or 50 records)
	if clickhouseRepo != nil {
		w.batchWriter = clickhouse.NewBatchWriter(
			clickhouseRepo,
			50,             // maxBatch: 50 metrics
			30*time.Second, // maxWait: flush every 30s
			flushAgentMetrics,
		)
	}

	return w
}

// flushAgentMetrics batch flush function for metrics
func flushAgentMetrics(ctx context.Context, repo *clickhouse.Repository, records []interface{}) error {
	metrics := make([]models.AgentMetric, len(records))
	for i, rec := range records {
		metrics[i] = rec.(models.AgentMetric)
	}

	return repo.SaveAgentMetrics(ctx, metrics)
}

// Name returns worker name
func (w *AgentMetricsWorker) Name() string {
	return "agent_metrics"
}

// Run executes one iteration - collects metrics from all running agents
// Called periodically by pkg/worker.PeriodicWorker
func (w *AgentMetricsWorker) Run(ctx context.Context) error {
	w.collectMetrics(ctx)
	return nil
}

// collectMetrics collects metrics from all running agents
func (w *AgentMetricsWorker) collectMetrics(ctx context.Context) {
	logger.Debug("collecting agent metrics...")

	startTime := time.Now()

	// Get all running agents
	runners := w.agentManager.GetRunningAgents()
	if len(runners) == 0 {
		logger.Debug("no running agents to collect metrics from")
		return
	}

	metricsCollected := 0

	for _, runner := range runners {
		// Collect metrics for this agent
		metric, err := w.collectAgentMetrics(ctx, runner)
		if err != nil {
			logger.Warn("failed to collect agent metrics",
				zap.String("agent_id", runner.Config.ID),
				zap.Error(err),
			)
			continue
		}

		// Add to batch writer (will auto-flush when full or on timer)
		if w.batchWriter != nil {
			w.batchWriter.Add(*metric)
			metricsCollected++
		}
	}

	duration := time.Since(startTime)

	logger.Info("agent metrics collected",
		zap.Int("agents", len(runners)),
		zap.Int("metrics_saved", metricsCollected),
		zap.Duration("duration", duration),
	)
}

// collectAgentMetrics collects metrics for single agent
func (w *AgentMetricsWorker) collectAgentMetrics(ctx context.Context, runner *agents.AgenticRunner) (*models.AgentMetric, error) {
	// For now, use state data only
	// TODO: Add GetRecentDecisions method to repository for detailed stats
	decisions := []models.AgentDecision{}

	// Calculate decision breakdown
	var holdCount, openCount, closeCount int
	var aiCost, validatorCost float64

	for _, d := range decisions {
		switch d.Action {
		case models.ActionHold:
			holdCount++
		case models.ActionOpenLong, models.ActionOpenShort:
			openCount++
		case models.ActionClose:
			closeCount++
		}

		// Estimate cost (simplified)
		aiCost += 0.002 // ~$0.002 per CoT decision
		if d.ValidatorConsensus != "" {
			validatorCost += 0.015 // ~$0.015 per validation
		}
	}

	balance, _ := runner.State.Balance.Float64()
	equity, _ := runner.State.Equity.Float64()
	pnl, _ := runner.State.PnL.Float64()
	initialBalance, _ := runner.State.InitialBalance.Float64()

	pnlPercent := float64(0)
	if initialBalance > 0 {
		pnlPercent = (pnl / initialBalance) * 100
	}

	metric := &models.AgentMetric{
		AgentID:       runner.Config.ID,
		AgentName:     runner.Config.Name,
		Personality:   string(runner.Config.Personality),
		Timestamp:     time.Now(),
		Symbol:        runner.State.Symbol,
		Balance:       models.NewDecimal(balance),
		Equity:        models.NewDecimal(equity),
		PnL:           models.NewDecimal(pnl),
		PnLPercent:    pnlPercent,
		TotalTrades:   runner.State.TotalTrades,
		WinningTrades: runner.State.WinningTrades,
		LosingTrades:  runner.State.LosingTrades,
		WinRate:       runner.State.WinRate,

		// Decision stats (last 5 min)
		DecisionsTotal: len(decisions),
		DecisionsHold:  holdCount,
		DecisionsOpen:  openCount,
		DecisionsClose: closeCount,

		// Costs
		AICostUSD:        aiCost,
		ValidatorCostUSD: validatorCost,
	}

	return metric, nil
}

// Stop stops the metrics worker and flushes remaining metrics
func (w *AgentMetricsWorker) Stop() error {
	if w.batchWriter != nil {
		return w.batchWriter.Close()
	}
	return nil
}
