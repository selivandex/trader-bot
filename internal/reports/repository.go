package reports

import (
	"context"
	"time"

	"github.com/selivandex/trader-bot/pkg/models"
)

// Repository interface for accessing data (avoids tight coupling)
type Repository interface {
	// Agent data
	GetAgent(ctx context.Context, agentID string) (*models.AgentConfig, error)
	GetAgentState(ctx context.Context, agentID, symbol string) (*models.AgentState, error)

	// Decisions in time period
	GetDecisionsInPeriod(ctx context.Context, agentID, symbol string, start, end time.Time) ([]models.AgentDecision, error)

	// Insights/reflections
	GetInsightsInPeriod(ctx context.Context, agentID string, start, end time.Time) ([]string, error)
}
