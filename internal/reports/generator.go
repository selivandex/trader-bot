package reports

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/selivandex/trader-bot/pkg/models"
	"github.com/selivandex/trader-bot/pkg/templates"
)

// Generator generates trading reports for agents
type Generator struct {
	repo            Repository
	templateManager *templates.Manager
}

// NewGenerator creates report generator
func NewGenerator(repo Repository, templateManager *templates.Manager) *Generator {
	return &Generator{
		repo:            repo,
		templateManager: templateManager,
	}
}

// GenerateDailyReport generates comprehensive daily report for agent
func (g *Generator) GenerateDailyReport(ctx context.Context, agentID, symbol string, date time.Time) (*DailyReport, error) {
	// Use end of day as reference point
	endTime := time.Date(date.Year(), date.Month(), date.Day(), 23, 59, 59, 0, date.Location())
	startTime := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())

	// Gather all data
	agent, err := g.repo.GetAgent(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent: %w", err)
	}

	state, err := g.repo.GetAgentState(ctx, agentID, symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent state: %w", err)
	}

	// Get decisions made during this day
	decisions, err := g.repo.GetDecisionsInPeriod(ctx, agentID, symbol, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("failed to get decisions: %w", err)
	}

	// Calculate metrics
	metrics := g.calculateDayMetrics(decisions, state)

	// Get top insights/lessons from the day
	insights, err := g.repo.GetInsightsInPeriod(ctx, agentID, startTime, endTime)
	if err != nil {
		insights = []string{} // Continue without insights
	}

	report := &DailyReport{
		AgentID:     agentID,
		AgentName:   agent.Name,
		Symbol:      symbol,
		Date:        date,
		Period:      Period{Start: startTime, End: endTime},
		Metrics:     metrics,
		Decisions:   decisions,
		Insights:    insights,
		State:       state,
		GeneratedAt: time.Now(),
	}

	return report, nil
}

// GenerateWeeklyReport generates weekly summary
func (g *Generator) GenerateWeeklyReport(ctx context.Context, agentID, symbol string, weekStart time.Time) (*WeeklyReport, error) {
	weekEnd := weekStart.AddDate(0, 0, 7)

	// Get daily reports for each day
	dailyReports := []*DailyReport{}
	for d := weekStart; d.Before(weekEnd); d = d.AddDate(0, 0, 1) {
		report, err := g.GenerateDailyReport(ctx, agentID, symbol, d)
		if err != nil {
			continue // Skip failed days
		}
		dailyReports = append(dailyReports, report)
	}

	// Aggregate metrics
	weekMetrics := g.aggregateWeekMetrics(dailyReports)

	weeklyReport := &WeeklyReport{
		AgentID:      agentID,
		Symbol:       symbol,
		WeekStart:    weekStart,
		WeekEnd:      weekEnd,
		DailyReports: dailyReports,
		WeekMetrics:  weekMetrics,
		GeneratedAt:  time.Now(),
	}

	return weeklyReport, nil
}

// GenerateCustomReport generates report for custom time period
func (g *Generator) GenerateCustomReport(ctx context.Context, agentID, symbol string, start, end time.Time) (*CustomReport, error) {
	decisions, err := g.repo.GetDecisionsInPeriod(ctx, agentID, symbol, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to get decisions: %w", err)
	}

	state, err := g.repo.GetAgentState(ctx, agentID, symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get state: %w", err)
	}

	metrics := g.calculatePeriodMetrics(decisions, state, end.Sub(start))

	return &CustomReport{
		AgentID:     agentID,
		Symbol:      symbol,
		Period:      Period{Start: start, End: end},
		Metrics:     metrics,
		Decisions:   decisions,
		GeneratedAt: time.Now(),
	}, nil
}

// RenderDailyReport renders daily report using template
func (g *Generator) RenderDailyReport(ctx context.Context, report *DailyReport) (string, error) {
	if g.templateManager == nil {
		return "", fmt.Errorf("template manager not available - cannot render report")
	}

	output, err := g.templateManager.ExecuteTemplate("daily_report.tmpl", report)
	if err != nil {
		return "", fmt.Errorf("failed to render daily report template: %w", err)
	}

	return output, nil
}

// RenderWeeklyReport renders weekly report using template
func (g *Generator) RenderWeeklyReport(ctx context.Context, report *WeeklyReport) (string, error) {
	if g.templateManager == nil {
		return "", fmt.Errorf("template manager not available - cannot render report")
	}

	output, err := g.templateManager.ExecuteTemplate("weekly_report.tmpl", report)
	if err != nil {
		return "", fmt.Errorf("failed to render weekly report template: %w", err)
	}

	return output, nil
}

// RenderCustomReport renders custom period report using template
func (g *Generator) RenderCustomReport(ctx context.Context, report *CustomReport) (string, error) {
	if g.templateManager == nil {
		return "", fmt.Errorf("template manager not available - cannot render report")
	}

	output, err := g.templateManager.ExecuteTemplate("custom_report.tmpl", report)
	if err != nil {
		return "", fmt.Errorf("failed to render custom report template: %w", err)
	}

	return output, nil
}

// calculateDayMetrics calculates metrics for one day
func (g *Generator) calculateDayMetrics(decisions []models.AgentDecision, state *models.AgentState) *DayMetrics {
	metrics := &DayMetrics{}

	var wins, losses int
	var totalPnL float64

	for _, decision := range decisions {
		metrics.TotalDecisions++

		if decision.Executed {
			metrics.ExecutedTrades++

			// Parse outcome if available
			if decision.Outcome != "" {
				// Extract PnL from outcome JSON
				// Outcome format: {"pnl": 125.50, "pnl_percent": 3.2}
				var outcome map[string]interface{}
				if err := json.Unmarshal([]byte(decision.Outcome), &outcome); err == nil {
					if pnl, ok := outcome["pnl"].(float64); ok {
						totalPnL += pnl
						metrics.TotalPnL += pnl

						if pnl > 0 {
							wins++
							if pnl > metrics.BestTrade {
								metrics.BestTrade = pnl
							}
						} else if pnl < 0 {
							losses++
							if pnl < metrics.WorstTrade {
								metrics.WorstTrade = pnl
							}
						}
					}
				}
			}
		} else {
			metrics.HoldCount++
		}

		// Count by action
		switch decision.Action {
		case models.ActionOpenLong:
			metrics.LongCount++
		case models.ActionOpenShort:
			metrics.ShortCount++
		case models.ActionClose:
			metrics.CloseCount++
		}

		// Track confidence
		if decision.Confidence >= 80 {
			metrics.HighConfidenceCount++
		}
	}

	// Calculate win rate
	metrics.WinningTrades = wins
	metrics.LosingTrades = losses
	if wins+losses > 0 {
		metrics.WinRate = float64(wins) / float64(wins+losses)
	}

	// Get balance/equity from state
	metrics.StartBalance, _ = state.InitialBalance.Float64()
	metrics.EndBalance, _ = state.Balance.Float64()
	metrics.StartEquity, _ = state.InitialBalance.Float64()
	metrics.EndEquity, _ = state.Equity.Float64()

	// Calculate daily return
	if metrics.StartBalance > 0 {
		metrics.DailyReturn = (metrics.EndBalance - metrics.StartBalance) / metrics.StartBalance
	}

	return metrics
}

// aggregateWeekMetrics aggregates daily metrics into weekly
func (g *Generator) aggregateWeekMetrics(dailyReports []*DailyReport) *WeekMetrics {
	weekMetrics := &WeekMetrics{}

	var totalWins, totalLosses int
	var bestDayPnL, worstDayPnL float64

	for _, daily := range dailyReports {
		weekMetrics.TotalDecisions += daily.Metrics.TotalDecisions
		weekMetrics.ExecutedTrades += daily.Metrics.ExecutedTrades
		weekMetrics.TotalPnL += daily.Metrics.TotalPnL
		weekMetrics.LongCount += daily.Metrics.LongCount
		weekMetrics.ShortCount += daily.Metrics.ShortCount

		totalWins += daily.Metrics.WinningTrades
		totalLosses += daily.Metrics.LosingTrades

		// Track best/worst day
		dayPnL := daily.Metrics.TotalPnL
		if dayPnL > bestDayPnL {
			bestDayPnL = dayPnL
		}
		if dayPnL < worstDayPnL {
			worstDayPnL = dayPnL
		}
	}

	if len(dailyReports) > 0 {
		weekMetrics.StartBalance = dailyReports[0].Metrics.StartBalance
		weekMetrics.EndBalance = dailyReports[len(dailyReports)-1].Metrics.EndBalance
	}

	// Calculate win rate
	weekMetrics.WinningTrades = totalWins
	weekMetrics.LosingTrades = totalLosses
	if totalWins+totalLosses > 0 {
		weekMetrics.WinRate = float64(totalWins) / float64(totalWins+totalLosses)
	}

	// Calculate weekly return
	if weekMetrics.StartBalance > 0 {
		weekMetrics.WeeklyReturn = (weekMetrics.EndBalance - weekMetrics.StartBalance) / weekMetrics.StartBalance
	}

	weekMetrics.BestDay = bestDayPnL
	weekMetrics.WorstDay = worstDayPnL
	weekMetrics.TradingDays = len(dailyReports)

	// TODO: Calculate Sharpe ratio
	weekMetrics.SharpeRatio = 0

	return weekMetrics
}

// calculatePeriodMetrics calculates metrics for custom period
func (g *Generator) calculatePeriodMetrics(decisions []models.AgentDecision, state *models.AgentState, duration time.Duration) *PeriodMetrics {
	metrics := &PeriodMetrics{Duration: duration}

	for _, decision := range decisions {
		metrics.TotalDecisions++
		if decision.Executed {
			metrics.ExecutedTrades++
		}
	}

	return metrics
}
