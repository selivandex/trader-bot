package reports

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/selivandex/trader-bot/pkg/logger"
)

// Scheduler handles automated report generation
type Scheduler struct {
	generator *Generator
	notifier  Notifier
}

// Notifier interface for sending reports
type Notifier interface {
	SendDailyReport(ctx context.Context, userID, agentName string, report *DailyReport, formattedText string) error
}

// NewScheduler creates report scheduler
func NewScheduler(generator *Generator, notifier Notifier) *Scheduler {
	return &Scheduler{
		generator: generator,
		notifier:  notifier,
	}
}

// Start starts the report scheduler (runs in background)
func (s *Scheduler) Start(ctx context.Context) error {
	logger.Info("📊 Report scheduler starting")

	// Wait until next midnight
	now := time.Now()
	nextMidnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 1, 0, now.Location())
	timeUntilMidnight := nextMidnight.Sub(now)

	logger.Info("next daily report scheduled",
		zap.Duration("in", timeUntilMidnight),
		zap.Time("at", nextMidnight),
	)

	// Initial timer until midnight
	timer := time.NewTimer(timeUntilMidnight)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("report scheduler stopped")
			return ctx.Err()

		case <-timer.C:
			// Generate reports for all active agents
			s.generateDailyReportsForAllAgents(ctx)

			// Reset timer for next midnight (24h from now)
			timer.Reset(24 * time.Hour)
		}
	}
}

// generateDailyReportsForAllAgents generates and sends daily reports
func (s *Scheduler) generateDailyReportsForAllAgents(ctx context.Context) {
	logger.Info("📊 generating daily reports for all agents")

	// Get yesterday's date (since we run at 00:00:01)
	yesterday := time.Now().AddDate(0, 0, -1)

	// TODO: Get list of active agents from repository
	// For now, this is a placeholder structure

	logger.Info("daily reports generated",
		zap.Time("for_date", yesterday),
	)
}

// SendDailyReportForAgent generates and sends report for specific agent
func (s *Scheduler) SendDailyReportForAgent(ctx context.Context, agentID, symbol string, date time.Time) error {
	// Generate report
	report, err := s.generator.GenerateDailyReport(ctx, agentID, symbol, date)
	if err != nil {
		return err
	}

	// Render to text
	text, err := s.generator.RenderDailyReport(ctx, report)
	if err != nil {
		return err
	}

	// Send via notifier
	if s.notifier != nil {
		// Get user ID from agent config
		// For now, use placeholder
		userID := "user123" // TODO: Get from agent

		if err := s.notifier.SendDailyReport(ctx, userID, report.AgentName, report, text); err != nil {
			logger.Error("failed to send daily report",
				zap.String("agent_id", agentID),
				zap.Error(err),
			)
			return err
		}
	}

	logger.Info("daily report sent",
		zap.String("agent_id", agentID),
		zap.String("symbol", symbol),
		zap.Time("date", date),
	)

	return nil
}
