package agents

import (
	"context"

	"go.uber.org/zap"

	"github.com/selivandex/trader-bot/internal/toolkit"
	"github.com/selivandex/trader-bot/pkg/logger"
	"github.com/selivandex/trader-bot/pkg/models"
)

// initializeToolkit creates toolkit for agent runner
func (am *AgenticManager) initializeToolkit(runner *AgenticRunner) {
	logger.Info("initializing agent toolkit",
		zap.String("agent_id", runner.Config.ID),
		zap.String("agent_name", runner.Config.Name),
	)

	// Create repository adapter to avoid circular dependency
	repoAdapter := &repositoryAdapter{repo: am.repository}

	agentToolkit := toolkit.NewLocalToolkit(
		runner.Config.ID,
		am.marketRepo,
		am.newsCache,
		repoAdapter,
		runner.MemoryManager,
		am.notifier,
	)

	// Set toolkit in components that need it
	runner.CoTEngine.SetToolkit(agentToolkit)

	logger.Debug("agent toolkit initialized",
		zap.String("agent_id", runner.Config.ID),
	)
}

// repositoryAdapter adapts Repository to toolkit.AgentRepository interface
type repositoryAdapter struct {
	repo *Repository
}

func (r *repositoryAdapter) GetAgent(ctx context.Context, agentID string) (*models.AgentConfig, error) {
	return r.repo.GetAgent(ctx, agentID)
}

func (r *repositoryAdapter) GetAgentState(ctx context.Context, agentID string, symbol string) (*models.AgentState, error) {
	return r.repo.GetAgentState(ctx, agentID, symbol)
}

func (r *repositoryAdapter) GetSemanticMemories(ctx context.Context, agentID string, limit int) ([]models.SemanticMemory, error) {
	return r.repo.GetSemanticMemories(ctx, agentID, limit)
}

func (r *repositoryAdapter) GetCollectiveMemories(ctx context.Context, personality string, limit int) ([]models.CollectiveMemory, error) {
	return r.repo.GetCollectiveMemories(ctx, personality, limit)
}

func (r *repositoryAdapter) GetAgentStatisticalMemory(ctx context.Context, agentID string) (*models.AgentMemory, error) {
	return r.repo.GetAgentStatisticalMemory(ctx, agentID)
}

func (r *repositoryAdapter) GetRecentWhaleTransactions(ctx context.Context, symbol string, hours int, minImpact int) ([]models.WhaleTransaction, error) {
	return r.repo.GetRecentWhaleTransactions(ctx, symbol, hours, minImpact)
}

func (r *repositoryAdapter) GetExchangeFlows(ctx context.Context, symbol string, hours int) ([]models.ExchangeFlow, error) {
	return r.repo.GetExchangeFlows(ctx, symbol, hours)
}

func (r *repositoryAdapter) GetPeakEquity(ctx context.Context, agentID, symbol string) (float64, error) {
	return r.repo.GetPeakEquity(ctx, agentID, symbol)
}

func (r *repositoryAdapter) GetAgentPerformanceMetrics(ctx context.Context, agentID string, symbol string) (*toolkit.AgentPerformanceMetrics, error) {
	metrics, err := r.repo.GetAgentPerformanceMetrics(ctx, agentID, symbol)
	if err != nil {
		return nil, err
	}

	// Convert agents.AgentPerformanceMetrics to toolkit.AgentPerformanceMetrics
	return &toolkit.AgentPerformanceMetrics{
		TotalTrades:   metrics.TotalTrades,
		WinningTrades: metrics.WinningTrades,
		LosingTrades:  metrics.LosingTrades,
		WinRate:       metrics.WinRate,
		TotalPnL:      metrics.TotalPnL,
		AvgPnL:        metrics.AvgPnL,
		MaxWin:        metrics.MaxWin,
		MaxLoss:       metrics.MaxLoss,
		SharpeRatio:   metrics.SharpeRatio,
	}, nil
}

// ensureDependencies ensures newsCache and marketRepo are available
func (am *AgenticManager) ensureDependencies() error {
	if am.newsCache == nil && am.newsAggregator != nil {
		// Extract cache from aggregator if available
		// For now, we'll need to add a method to aggregator to expose cache
		logger.Warn("newsCache not available, toolkit news features will be limited")
	}

	if am.marketRepo == nil {
		logger.Warn("marketRepo not available, toolkit market features will be limited")
	}

	return nil
}

// Fields to add to AgenticManager
// These should be added to the AgenticManager struct in manager.go:
//
// type AgenticManager struct {
//     ...existing fields...
//     marketRepo    *market.Repository  // NEW
//     newsCache     *news.Cache         // NEW
// }
//
// Constructor should accept these:
// func NewAgenticManager(..., marketRepo *market.Repository, newsCache *news.Cache) *AgenticManager
