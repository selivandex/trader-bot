package agents

import (
	"context"
	"fmt"

	"github.com/selivandex/trader-bot/internal/toolkit"
	"github.com/selivandex/trader-bot/pkg/logger"
	"github.com/selivandex/trader-bot/pkg/models"
	"github.com/selivandex/trader-bot/pkg/templates"
	"go.uber.org/zap"
)

// AlertConditionChecker determines when agent should alert owner
type AlertConditionChecker struct {
	config          *models.AgentConfig
	toolkit         toolkit.AgentToolkit
	templateManager *templates.Manager
}

// NewAlertConditionChecker creates checker
func NewAlertConditionChecker(config *models.AgentConfig, tk toolkit.AgentToolkit, tm *templates.Manager) *AlertConditionChecker {
	return &AlertConditionChecker{
		config:          config,
		toolkit:         tk,
		templateManager: tm,
	}
}

// ShouldAlertOwner checks if current situation warrants owner notification
func (acc *AlertConditionChecker) ShouldAlertOwner(
	ctx context.Context,
	state *ThinkingState,
	confidence float64,
) (bool, string, string) {

	// Priority levels: CRITICAL, HIGH, MEDIUM, LOW

	// 1. CRITICAL: Liquidation Risk
	if state.CurrentPosition != nil && state.CurrentPosition.Side != models.PositionNone {
		currentPrice := state.MarketData.Ticker.Last.InexactFloat64()
		entryPrice := state.CurrentPosition.EntryPrice.InexactFloat64()
		leverage := float64(state.CurrentPosition.Leverage)

		// Calculate distance to liquidation
		liquidationDistance := 100 / leverage // e.g., 3x leverage = 33% to liquidation

		var priceMove float64
		if state.CurrentPosition.Side == models.PositionLong {
			priceMove = (entryPrice - currentPrice) / entryPrice * 100
		} else {
			priceMove = (currentPrice - entryPrice) / entryPrice * 100
		}

		// Alert if within 50% of liquidation distance
		if priceMove > liquidationDistance*0.5 {
			data := map[string]interface{}{
				"Side":                state.CurrentPosition.Side,
				"Leverage":            state.CurrentPosition.Leverage,
				"EntryPrice":          entryPrice,
				"CurrentPrice":        currentPrice,
				"PriceMove":           priceMove,
				"LiquidationDistance": liquidationDistance,
				"DistancePercent":     (priceMove / liquidationDistance) * 100,
			}

			message := acc.renderAlertTemplate("liquidation_risk.tmpl", data)
			return true, message, "CRITICAL"
		}
	}

	// 2. CRITICAL: Major Drawdown
	if acc.toolkit != nil {
		exceedsDrawdown, _ := acc.toolkit.CheckDrawdownRisk(ctx, acc.config.ID, state.MarketData.Symbol, 0)
		if exceedsDrawdown {
			message := "📉 MAJOR DRAWDOWN: Approaching or exceeded max drawdown limit (20%)"
			return true, message, "CRITICAL"
		}
	}

	// 3. HIGH: Mega Whale Activity
	if acc.toolkit != nil {
		whaleAlert, err := acc.toolkit.CheckWhaleAlert(ctx, state.MarketData.Symbol)
		if err == nil && whaleAlert.HasAlert && whaleAlert.Severity == "CRITICAL" {
			return true, whaleAlert.Message, "HIGH"
		}
	}

	// 4. HIGH: Breaking News Impact 10
	if state.MarketData.NewsSummary != nil {
		for _, news := range state.MarketData.NewsSummary.RecentNews {
			if news.Impact >= 10 {
				message := fmt.Sprintf(
					"🚨 CRITICAL NEWS: [%d/10] %s\nSentiment: %.2f, Source: %s",
					news.Impact, news.Title, news.Sentiment, news.Source,
				)
				return true, message, "HIGH"
			}
		}
	}

	// 5. HIGH: Conflicting Signals with Open Position
	if state.CurrentPosition != nil && state.CurrentPosition.Side != models.PositionNone {
		if len(state.Evaluations) > 0 {
			// Check if top evaluation contradicts current position
			topEval := state.Evaluations[0]

			conflicting := false
			if state.CurrentPosition.Side == models.PositionLong && topEval.OptionID == "close" {
				conflicting = true
			} else if state.CurrentPosition.Side == models.PositionShort && topEval.OptionID == "close" {
				conflicting = true
			}

			if conflicting {
				message := fmt.Sprintf(
					"⚠️ CONFLICTING SIGNALS: In %s position, but top evaluation suggests closing\nCurrent PnL: $%.2f",
					state.CurrentPosition.Side, state.CurrentPosition.UnrealizedPnL.InexactFloat64(),
				)
				return true, message, "HIGH"
			}
		}
	}

	// 6. MEDIUM: Very Low Confidence on Important Decision
	if len(state.Options) > 0 && confidence < 0.4 {
		bestOption := "unknown"
		if len(state.Evaluations) > 0 {
			bestOption = state.Evaluations[0].OptionID
		}

		message := fmt.Sprintf(
			"❓ LOW CONFIDENCE: Only %.0f%% confident in best option (%s)\nMay need human judgment",
			confidence*100, bestOption,
		)
		return true, message, "MEDIUM"
	}

	// 7. MEDIUM: Discovered Important Pattern Insight
	if len(state.Insights) > 0 {
		for _, insight := range state.Insights {
			// Check if insight mentions high success rate
			if containsSubstring(insight, "success rate") && containsSubstring(insight, "90") {
				message := fmt.Sprintf("💡 VALUABLE INSIGHT: %s", insight)
				return true, message, "MEDIUM"
			}
		}
	}

	// 8. MEDIUM: Circuit Breaker Triggered
	// Check if too many losses in a row
	if acc.toolkit != nil {
		streak, isWinning, err := acc.toolkit.GetCurrentStreak(ctx, state.MarketData.Symbol)
		if err == nil && !isWinning && streak >= 5 {
			message := fmt.Sprintf(
				"🛑 LOSING STREAK: %d losses in a row\nConsider pausing or reviewing strategy",
				streak,
			)
			return true, message, "MEDIUM"
		}
	}

	// 9. LOW: Interesting Whale Pattern (not urgent)
	if acc.toolkit != nil {
		pattern, strength, err := acc.toolkit.DetectWhalePattern(ctx, state.MarketData.Symbol, 24)
		if err == nil && (strength > 80 || strength < 20) {
			message := fmt.Sprintf(
				"🐋 Whale Pattern: %s (strength: %.0f/100)",
				pattern, strength,
			)
			return true, message, "LOW"
		}
	}

	// No alert needed
	return false, "", ""
}

// ShouldAlertOnToolResult checks if tool result warrants immediate alert
func (acc *AlertConditionChecker) ShouldAlertOnToolResult(toolName string, result interface{}) (bool, string, string) {
	switch toolName {
	case "CalculatePositionRisk":
		if risk, ok := result.(*toolkit.PositionRiskMetrics); ok {
			if risk.RiskScore >= 9 {
				message := fmt.Sprintf(
					"⚠️ EXTREME RISK: Position risk score %d/10\nMax loss: $%.2f, Liquidation: $%.2f",
					risk.RiskScore, risk.MaxLoss, risk.LiquidationPrice,
				)
				return true, message, "HIGH"
			}
		}

	case "SimulateWorstCase":
		if worst, ok := result.(*toolkit.WorstCaseScenario); ok {
			if worst.LiquidationRisk == "high" {
				message := fmt.Sprintf(
					"🚨 WORST CASE: High liquidation risk\n%s\nTime to liquidation: %.1fh",
					worst.Recovery, worst.TimeToLiquidation,
				)
				return true, message, "HIGH"
			}
		}

	case "CheckWhaleAlert":
		if alert, ok := result.(*toolkit.WhaleAlert); ok {
			if alert.HasAlert && (alert.Severity == "CRITICAL" || alert.Severity == "HIGH") {
				return true, alert.Message, alert.Severity
			}
		}

	case "GetHighImpactNews":
		if news, ok := result.([]models.NewsItem); ok {
			for _, item := range news {
				if item.Impact >= 10 {
					message := fmt.Sprintf(
						"🚨 BREAKING: [%d/10] %s\nSentiment: %.2f (%s)",
						item.Impact, item.Title, item.Sentiment, item.Source,
					)
					return true, message, "CRITICAL"
				}
			}
		}
	}

	return false, "", ""
}

// FormatAlertMessage formats alert with agent context
func (acc *AlertConditionChecker) FormatAlertMessage(baseMessage, priority string, iteration int) string {
	return fmt.Sprintf(
		"🤖 *%s* [Iteration %d]\n🚨 Priority: %s\n\n%s",
		acc.config.Name,
		iteration,
		priority,
		baseMessage,
	)
}

// renderAlertTemplate renders alert template with data
func (acc *AlertConditionChecker) renderAlertTemplate(templateName string, data interface{}) string {
	if acc.templateManager == nil {
		logger.Warn("template manager not available, using fallback alert format")
		return fmt.Sprintf("%v", data)
	}

	output, err := acc.templateManager.ExecuteTemplate(templateName, data)
	if err != nil {
		logger.Error("failed to render alert template",
			zap.String("template", templateName),
			zap.Error(err),
		)
		return fmt.Sprintf("Alert: %v", data)
	}

	return output
}
