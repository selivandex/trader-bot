package toolkit

import (
	"context"

	"github.com/selivandex/trader-bot/pkg/models"
)

// AgentRepository interface for accessing agent data (avoids circular dependency)
type AgentRepository interface {
	GetAgent(ctx context.Context, agentID string) (*models.AgentConfig, error)
	GetAgentState(ctx context.Context, agentID string, symbol string) (*models.AgentState, error)
	GetSemanticMemories(ctx context.Context, agentID string, limit int) ([]models.SemanticMemory, error)
	GetCollectiveMemories(ctx context.Context, personality string, limit int) ([]models.CollectiveMemory, error)
	GetAgentStatisticalMemory(ctx context.Context, agentID string) (*models.AgentMemory, error)
	GetRecentWhaleTransactions(ctx context.Context, symbol string, hours int, minImpact int) ([]models.WhaleTransaction, error)
	GetExchangeFlows(ctx context.Context, symbol string, hours int) ([]models.ExchangeFlow, error)
	GetPeakEquity(ctx context.Context, agentID, symbol string) (float64, error)
	GetAgentPerformanceMetrics(ctx context.Context, agentID string, symbol string) (*AgentPerformanceMetrics, error)
}

// AgentPerformanceMetrics holds performance statistics
// Redefined here to avoid circular dependency with agents package
type AgentPerformanceMetrics struct {
	TotalTrades   int     `db:"total_trades"`
	WinningTrades int     `db:"winning_trades"`
	LosingTrades  int     `db:"losing_trades"`
	WinRate       float64 `db:"win_rate"`
	TotalPnL      float64 `db:"total_pnl"`
	AvgPnL        float64 `db:"avg_pnl"`
	MaxWin        float64 `db:"max_win"`
	MaxLoss       float64 `db:"max_loss"`
	SharpeRatio   float64 `db:"sharpe_ratio"`
}

// SemanticMemoryManager interface for memory operations
type SemanticMemoryManager interface {
	RecallRelevant(ctx context.Context, agentID string, personality string, query string, topK int) ([]models.SemanticMemory, error)
	FindMemoriesRelatedToNews(ctx context.Context, agentID string, personality string, newsEmbedding []float32, topK int) ([]models.SemanticMemory, error)
}

// Notifier interface for sending alerts
type Notifier interface {
	SendErrorAlert(ctx context.Context, userID, agentName, errorMsg string) error
}
