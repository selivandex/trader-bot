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

// Start starts the periodic recovery check
func (w *AgentRecoveryWorker) Start(ctx context.Context) error {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	logger.Info("🔄 agent recovery worker started",
		zap.Duration("interval", w.interval),
	)

	// Run once immediately on startup (after initial delay)
	time.Sleep(30 * time.Second) // Wait for system to stabilize
	w.checkAndRecoverAgents(ctx)

	for {
		select {
		case <-ctx.Done():
			logger.Info("agent recovery worker stopped")
			return ctx.Err()

		case <-ticker.C:
			w.checkAndRecoverAgents(ctx)
		}
	}
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
