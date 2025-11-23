package toolkit

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/selivandex/trader-bot/internal/reports"
	"github.com/selivandex/trader-bot/pkg/logger"
	"github.com/selivandex/trader-bot/pkg/models"
)

// reportRepoAdapter adapts AgentRepository to reports.Repository interface
type reportRepoAdapter struct {
	agentRepo AgentRepository
}

func (a *reportRepoAdapter) GetAgent(ctx context.Context, agentID string) (*models.AgentConfig, error) {
	return a.agentRepo.GetAgent(ctx, agentID)
}

func (a *reportRepoAdapter) GetAgentState(ctx context.Context, agentID, symbol string) (*models.AgentState, error) {
	return a.agentRepo.GetAgentState(ctx, agentID, symbol)
}

func (a *reportRepoAdapter) GetDecisionsInPeriod(ctx context.Context, agentID, symbol string, start, end time.Time) ([]models.AgentDecision, error) {
	return a.agentRepo.GetDecisionsInPeriod(ctx, agentID, symbol, start, end)
}

func (a *reportRepoAdapter) GetInsightsInPeriod(ctx context.Context, agentID string, start, end time.Time) ([]string, error) {
	return a.agentRepo.GetInsightsInPeriod(ctx, agentID, start, end)
}

// ============ Reporting Tools Implementation ============

// GenerateDailyReport generates daily performance report
func (t *LocalToolkit) GenerateDailyReport(ctx context.Context, date time.Time) (string, error) {
	logger.Debug("toolkit: generate_daily_report",
		zap.String("agent_id", t.agentID),
		zap.Time("date", date),
	)

	// Create report generator with adapter
	adapter := &reportRepoAdapter{agentRepo: t.agentRepo}
	generator := reports.NewGenerator(adapter, nil) // templateManager in agents package

	// Get agent config to find symbol
	agent, err := t.agentRepo.GetAgent(ctx, t.agentID)
	if err != nil {
		return "", fmt.Errorf("failed to get agent: %w", err)
	}

	// For now, use first trading pair
	// TODO: Support multiple symbols
	symbol := "BTC/USDT" // Default, should get from agent state

	// Generate report
	report, err := generator.GenerateDailyReport(ctx, t.agentID, symbol, date)
	if err != nil {
		return "", fmt.Errorf("failed to generate report: %w", err)
	}

	// Render to text
	text, err := generator.RenderDailyReport(ctx, report)
	if err != nil {
		return "", fmt.Errorf("failed to render report: %w", err)
	}

	logger.Info("daily report generated",
		zap.String("agent_id", t.agentID),
		zap.String("agent_name", agent.Name),
		zap.Time("date", date),
	)

	return text, nil
}

// GenerateWeeklyReport generates weekly summary
func (t *LocalToolkit) GenerateWeeklyReport(ctx context.Context, weekStart time.Time) (string, error) {
	logger.Debug("toolkit: generate_weekly_report",
		zap.String("agent_id", t.agentID),
		zap.Time("week_start", weekStart),
	)

	adapter := &reportRepoAdapter{agentRepo: t.agentRepo}
	generator := reports.NewGenerator(adapter, nil)

	symbol := "BTC/USDT" // TODO: Get from agent

	report, err := generator.GenerateWeeklyReport(ctx, t.agentID, symbol, weekStart)
	if err != nil {
		return "", fmt.Errorf("failed to generate weekly report: %w", err)
	}

	text, err := generator.RenderWeeklyReport(ctx, report)
	if err != nil {
		return "", fmt.Errorf("failed to render report: %w", err)
	}

	return text, nil
}

// SendDailyReportToOwner generates and sends daily report via telegram
func (t *LocalToolkit) SendDailyReportToOwner(ctx context.Context) error {
	logger.Info("📊 toolkit: send_daily_report_to_owner",
		zap.String("agent_id", t.agentID),
	)

	// Generate yesterday's report (since running at 00:00:01)
	yesterday := time.Now().AddDate(0, 0, -1)

	reportText, err := t.GenerateDailyReport(ctx, yesterday)
	if err != nil {
		return fmt.Errorf("failed to generate report: %w", err)
	}

	// Send as urgent alert (will go to owner via telegram)
	if err := t.SendUrgentAlert(ctx, reportText, "LOW"); err != nil {
		return fmt.Errorf("failed to send report: %w", err)
	}

	logger.Info("✅ daily report sent to owner",
		zap.String("agent_id", t.agentID),
		zap.Time("for_date", yesterday),
	)

	return nil
}
