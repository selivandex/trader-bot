package workers

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/selivandex/trader-bot/internal/adapters/exchange"
	"github.com/selivandex/trader-bot/internal/agents"
	"github.com/selivandex/trader-bot/pkg/logger"
)

// AgentRecoveryWorker periodically checks for agents that should be running but aren't
// This is a safety net in addition to startup recovery
type AgentRecoveryWorker struct {
	agenticManager  *agents.AgenticManager
	exchangeFactory func(string, string, string, bool) (exchange.Exchange, error)
	interval        time.Duration
}

// NewAgentRecoveryWorker creates new agent recovery worker
func NewAgentRecoveryWorker(
	agenticManager *agents.AgenticManager,
	exchangeFactory func(string, string, string, bool) (exchange.Exchange, error),
	interval time.Duration,
) *AgentRecoveryWorker {
	if interval < 1*time.Minute {
		interval = 5 * time.Minute // Minimum 1 minute
	}

	return &AgentRecoveryWorker{
		agenticManager:  agenticManager,
		exchangeFactory: exchangeFactory,
		interval:        interval,
	}
}

// Name returns worker name
func (w *AgentRecoveryWorker) Name() string {
	return "agent_recovery"
}

// Run executes one iteration - checks and recovers agents
// Called periodically by pkg/worker.PeriodicWorker
func (w *AgentRecoveryWorker) Run(ctx context.Context) error {
	w.checkAndRecoverAgents(ctx)
	return nil
}

// checkAndRecoverAgents checks if any agents should be running but aren't
func (w *AgentRecoveryWorker) checkAndRecoverAgents(ctx context.Context) {
	logger.Debug("🔍 checking for agents to recover...")

	err := w.agenticManager.RestoreRunningAgents(ctx, w.exchangeFactory)
	if err != nil {
		logger.Error("failed to recover agents in periodic check",
			zap.Error(err),
		)
		return
	}

	logger.Debug("✅ agent recovery check complete")
}
