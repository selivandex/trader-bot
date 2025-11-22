package ai

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/alexanderselivanov/trader/pkg/models"
)

// buildSystemPrompt creates system prompt for AI
func buildSystemPrompt() string {
	return `You are a professional cryptocurrency futures trader with expertise in technical analysis and risk management.

Your task is to analyze market data and provide trading decisions in strict JSON format.

TRADING RULES:
- Maximum position size: 30% of balance
- Maximum leverage: 3x
- Always set stop-loss at 2% from entry price
- Take profit target: 4-6% from entry price
- Consider funding rates when determining position direction
- Never trade during extreme volatility or news events
- Minimum confidence threshold: 70% to execute trade

DECISION ACTIONS:
- HOLD: No action, wait for better setup
- CLOSE: Close existing position
- OPEN_LONG: Open long position (buy)
- OPEN_SHORT: Open short position (sell)
- SCALE_IN: Add to existing position
- SCALE_OUT: Partially close position

OUTPUT FORMAT (must be valid JSON):
{
  "action": "HOLD|CLOSE|OPEN_LONG|OPEN_SHORT|SCALE_IN|SCALE_OUT",
  "reason": "Brief explanation of your reasoning",
  "size": 0.0,
  "stop_loss": 0.0,
  "take_profit": 0.0,
  "confidence": 0-100
}

IMPORTANT:
- Respond ONLY with valid JSON, no additional text
- Be conservative: when in doubt, choose HOLD
- Always provide stop_loss and take_profit for OPEN actions
- Consider multiple timeframes for confirmation
- Pay attention to volume and funding rates`
}

// buildUserPrompt creates user prompt from trading data
func buildUserPrompt(prompt *models.TradingPrompt) string {
	var sb strings.Builder
	
	// Current market state
	sb.WriteString("=== MARKET DATA ===\n\n")
	
	if prompt.MarketData != nil && prompt.MarketData.Ticker != nil {
		ticker := prompt.MarketData.Ticker
		sb.WriteString(fmt.Sprintf("Symbol: %s\n", ticker.Symbol))
		sb.WriteString(fmt.Sprintf("Current Price: $%.2f\n", ticker.Last.Float64()))
		sb.WriteString(fmt.Sprintf("24h Change: %.2f%%\n", ticker.Change24h.Float64()))
		sb.WriteString(fmt.Sprintf("24h High: $%.2f\n", ticker.High24h.Float64()))
		sb.WriteString(fmt.Sprintf("24h Low: $%.2f\n", ticker.Low24h.Float64()))
		sb.WriteString(fmt.Sprintf("24h Volume: $%.2f\n", ticker.Volume24h.Float64()))
		sb.WriteString(fmt.Sprintf("Bid: $%.2f | Ask: $%.2f\n\n", ticker.Bid.Float64(), ticker.Ask.Float64()))
	}
	
	// Technical indicators
	if prompt.MarketData != nil && prompt.MarketData.Indicators != nil {
		sb.WriteString("=== TECHNICAL INDICATORS ===\n\n")
		ind := prompt.MarketData.Indicators
		
		if ind.RSI != nil {
			sb.WriteString("RSI:\n")
			for tf, val := range ind.RSI {
				sb.WriteString(fmt.Sprintf("  %s: %.2f", tf, val.Float64()))
				if val.Float64() > 70 {
					sb.WriteString(" (Overbought)\n")
				} else if val.Float64() < 30 {
					sb.WriteString(" (Oversold)\n")
				} else {
					sb.WriteString(" (Neutral)\n")
				}
			}
		}
		
		if ind.MACD != nil {
			sb.WriteString(fmt.Sprintf("\nMACD:\n"))
			sb.WriteString(fmt.Sprintf("  MACD: %.2f\n", ind.MACD.MACD.Float64()))
			sb.WriteString(fmt.Sprintf("  Signal: %.2f\n", ind.MACD.Signal.Float64()))
			sb.WriteString(fmt.Sprintf("  Histogram: %.2f", ind.MACD.Histogram.Float64()))
			if ind.MACD.Histogram.Float64() > 0 {
				sb.WriteString(" (Bullish)\n")
			} else {
				sb.WriteString(" (Bearish)\n")
			}
		}
		
		if ind.BollingerBands != nil {
			sb.WriteString(fmt.Sprintf("\nBollinger Bands:\n"))
			sb.WriteString(fmt.Sprintf("  Upper: $%.2f\n", ind.BollingerBands.Upper.Float64()))
			sb.WriteString(fmt.Sprintf("  Middle: $%.2f\n", ind.BollingerBands.Middle.Float64()))
			sb.WriteString(fmt.Sprintf("  Lower: $%.2f\n", ind.BollingerBands.Lower.Float64()))
		}
		
		if ind.Volume != nil {
			sb.WriteString(fmt.Sprintf("\nVolume Analysis:\n"))
			sb.WriteString(fmt.Sprintf("  Current: %.2f\n", ind.Volume.Current.Float64()))
			sb.WriteString(fmt.Sprintf("  Average: %.2f\n", ind.Volume.Average.Float64()))
			sb.WriteString(fmt.Sprintf("  Ratio: %.2fx", ind.Volume.Ratio.Float64()))
			if ind.Volume.Ratio.Float64() > 1.5 {
				sb.WriteString(" (High volume)\n")
			} else if ind.Volume.Ratio.Float64() < 0.5 {
				sb.WriteString(" (Low volume)\n")
			} else {
				sb.WriteString(" (Normal volume)\n")
			}
		}
		
		sb.WriteString("\n")
	}
	
	// Funding rate and open interest
	if prompt.MarketData != nil {
		sb.WriteString("=== FUTURES METRICS ===\n\n")
		sb.WriteString(fmt.Sprintf("Funding Rate: %.4f%%", prompt.MarketData.FundingRate.Float64()*100))
		if prompt.MarketData.FundingRate.Float64() > 0.01 {
			sb.WriteString(" (Longs pay shorts - bearish sentiment)\n")
		} else if prompt.MarketData.FundingRate.Float64() < -0.01 {
			sb.WriteString(" (Shorts pay longs - bullish sentiment)\n")
		} else {
			sb.WriteString(" (Neutral)\n")
		}
		sb.WriteString(fmt.Sprintf("Open Interest: $%.2f\n\n", prompt.MarketData.OpenInterest.Float64()))
	}
	
	// News and sentiment
	if prompt.MarketData != nil && prompt.MarketData.NewsSummary != nil {
		news := prompt.MarketData.NewsSummary
		sb.WriteString("=== NEWS & SENTIMENT ===\n\n")
		sb.WriteString(fmt.Sprintf("Overall Sentiment: *%s*\n", strings.ToUpper(news.OverallSentiment)))
		sb.WriteString(fmt.Sprintf("Average Score: %.2f\n", news.AverageSentiment))
		sb.WriteString(fmt.Sprintf("Total News: %d (📈 %d | 📉 %d | ➡️ %d)\n\n",
			news.TotalItems, news.PositiveCount, news.NegativeCount, news.NeutralCount))
		
		if len(news.RecentNews) > 0 {
			sb.WriteString("Recent Headlines:\n")
			for i, item := range news.RecentNews {
				if i >= 3 {
					break
				}
				
				emoji := "➡️"
				if item.Sentiment > 0.2 {
					emoji = "📈"
				} else if item.Sentiment < -0.2 {
					emoji = "📉"
				}
				
				sb.WriteString(fmt.Sprintf("%d. %s [%s] %s (%.2f)\n",
					i+1, emoji, item.Source, truncateTitle(item.Title, 80), item.Sentiment))
			}
			sb.WriteString("\n")
		}
	}
	
	// Order book imbalance
	if prompt.MarketData != nil && prompt.MarketData.OrderBook != nil {
		ob := prompt.MarketData.OrderBook
		if len(ob.Bids) > 0 && len(ob.Asks) > 0 {
			bidVolume := 0.0
			askVolume := 0.0
			
			for _, bid := range ob.Bids[:min(10, len(ob.Bids))] {
				bidVolume += bid.Amount.Float64()
			}
			for _, ask := range ob.Asks[:min(10, len(ob.Asks))] {
				askVolume += ask.Amount.Float64()
			}
			
			total := bidVolume + askVolume
			bidPercent := (bidVolume / total) * 100
			
			sb.WriteString("=== ORDER BOOK ===\n\n")
			sb.WriteString(fmt.Sprintf("Bid/Ask Imbalance: %.1f%% bids / %.1f%% asks", bidPercent, 100-bidPercent))
			if bidPercent > 60 {
				sb.WriteString(" (Bullish)\n\n")
			} else if bidPercent < 40 {
				sb.WriteString(" (Bearish)\n\n")
			} else {
				sb.WriteString(" (Balanced)\n\n")
			}
		}
	}
	
	// Current position
	if prompt.CurrentPosition != nil {
		sb.WriteString("=== CURRENT POSITION ===\n\n")
		pos := prompt.CurrentPosition
		sb.WriteString(fmt.Sprintf("Side: %s\n", string(pos.Side)))
		sb.WriteString(fmt.Sprintf("Size: %.4f %s\n", pos.Size.Float64(), pos.Symbol[:3]))
		sb.WriteString(fmt.Sprintf("Entry Price: $%.2f\n", pos.EntryPrice.Float64()))
		sb.WriteString(fmt.Sprintf("Current Price: $%.2f\n", pos.CurrentPrice.Float64()))
		sb.WriteString(fmt.Sprintf("Leverage: %dx\n", pos.Leverage))
		sb.WriteString(fmt.Sprintf("Unrealized PnL: $%.2f", pos.UnrealizedPnL.Float64()))
		
		pnlPercent := (pos.UnrealizedPnL.Float64() / pos.Margin.Float64()) * 100
		sb.WriteString(fmt.Sprintf(" (%.2f%%)\n", pnlPercent))
		sb.WriteString(fmt.Sprintf("Margin: $%.2f\n\n", pos.Margin.Float64()))
	} else {
		sb.WriteString("=== CURRENT POSITION ===\n\n")
		sb.WriteString("No open position\n\n")
	}
	
	// Account info
	sb.WriteString("=== ACCOUNT INFO ===\n\n")
	sb.WriteString(fmt.Sprintf("Balance: $%.2f\n", prompt.Balance.Float64()))
	sb.WriteString(fmt.Sprintf("Equity: $%.2f\n", prompt.Equity.Float64()))
	sb.WriteString(fmt.Sprintf("Daily PnL: $%.2f", prompt.DailyPnL.Float64()))
	
	dailyPnLPercent := (prompt.DailyPnL.Float64() / prompt.Balance.Float64()) * 100
	sb.WriteString(fmt.Sprintf(" (%.2f%%)\n\n", dailyPnLPercent))
	
	// Recent trades performance
	if len(prompt.RecentTrades) > 0 {
		sb.WriteString("=== RECENT TRADES (Last 5) ===\n\n")
		wins := 0
		losses := 0
		
		for i, trade := range prompt.RecentTrades {
			if i >= 5 {
				break
			}
			
			outcome := "Win"
			if trade.PnL.Float64() < 0 {
				outcome = "Loss"
				losses++
			} else if trade.PnL.Float64() > 0 {
				wins++
			}
			
			sb.WriteString(fmt.Sprintf("%d. %s %s @ $%.2f - PnL: $%.2f (%s)\n",
				i+1,
				string(trade.Side),
				trade.Symbol,
				trade.Price.Float64(),
				trade.PnL.Float64(),
				outcome,
			))
		}
		
		if wins+losses > 0 {
			winRate := float64(wins) / float64(wins+losses) * 100
			sb.WriteString(fmt.Sprintf("\nRecent Win Rate: %.1f%% (%d wins, %d losses)\n\n", winRate, wins, losses))
		}
	}
	
	// Final instruction
	sb.WriteString("=== YOUR DECISION ===\n\n")
	sb.WriteString("Based on the above market data, technical indicators, and current position, ")
	sb.WriteString("provide your trading decision in JSON format.\n\n")
	sb.WriteString("Remember:\n")
	sb.WriteString("- Be conservative\n")
	sb.WriteString("- Always set stop-loss and take-profit\n")
	sb.WriteString("- Consider risk/reward ratio\n")
	sb.WriteString("- Respond ONLY with valid JSON\n")
	
	return sb.String()
}

// parseAIResponse parses AI response and extracts decision
func parseAIResponse(content, provider string) (*models.AIDecision, error) {
	// Try to extract JSON from response
	// AI might return markdown code blocks or extra text
	jsonStr := extractJSON(content)
	
	var response struct {
		Action     string  `json:"action"`
		Reason     string  `json:"reason"`
		Size       float64 `json:"size"`
		StopLoss   float64 `json:"stop_loss"`
		TakeProfit float64 `json:"take_profit"`
		Confidence int     `json:"confidence"`
	}
	
	if err := json.Unmarshal([]byte(jsonStr), &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w (content: %s)", err, jsonStr)
	}
	
	// Validate action
	action := models.AIAction(strings.ToUpper(response.Action))
	validActions := map[models.AIAction]bool{
		models.ActionHold:      true,
		models.ActionClose:     true,
		models.ActionOpenLong:  true,
		models.ActionOpenShort: true,
		models.ActionScaleIn:   true,
		models.ActionScaleOut:  true,
	}
	
	if !validActions[action] {
		return nil, fmt.Errorf("invalid action: %s", response.Action)
	}
	
	// Validate confidence
	if response.Confidence < 0 || response.Confidence > 100 {
		return nil, fmt.Errorf("invalid confidence: %d", response.Confidence)
	}
	
	decision := &models.AIDecision{
		Provider:   provider,
		Prompt:     "", // Will be set by caller
		Response:   jsonStr,
		Action:     action,
		Reason:     response.Reason,
		Size:       models.NewDecimal(response.Size),
		StopLoss:   models.NewDecimal(response.StopLoss),
		TakeProfit: models.NewDecimal(response.TakeProfit),
		Confidence: response.Confidence,
		Executed:   false,
		CreatedAt:  time.Now(),
	}
	
	return decision, nil
}

// extractJSON extracts JSON from text that might contain markdown or extra content
func extractJSON(text string) string {
	// Remove markdown code blocks
	re := regexp.MustCompile("```(?:json)?\\s*([\\s\\S]*?)```")
	matches := re.FindStringSubmatch(text)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	
	// Try to find JSON object
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	
	if start >= 0 && end > start {
		return strings.TrimSpace(text[start : end+1])
	}
	
	return strings.TrimSpace(text)
}

// Helper functions
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func truncateTitle(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

