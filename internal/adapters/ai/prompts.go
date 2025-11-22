package ai

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/selivandex/trader-bot/pkg/models"
)

// buildSystemPrompt creates system prompt for AI with strategy parameters
func buildSystemPrompt(params *models.StrategyParameters) string {
	return fmt.Sprintf(`You are a professional cryptocurrency futures trader with expertise in technical analysis and risk management.

Your task is to analyze market data and provide trading decisions in strict JSON format.

TRADING RULES:
- Maximum position size: %.0f%% of balance
- Maximum leverage: %dx
- Always set stop-loss at %.1f%% from entry price
- Take profit target: %.1f%% from entry price
- Consider funding rates when determining position direction
- Never trade during extreme volatility or news events
- Minimum confidence threshold: %d%% to execute trade

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
- Pay attention to volume and funding rates`,
		params.MaxPositionPercent,
		params.MaxLeverage,
		params.StopLossPercent,
		params.TakeProfitPercent,
		params.MinConfidenceThreshold,
	)
}

// buildUserPrompt creates user prompt from trading data
func buildUserPrompt(prompt *models.TradingPrompt) string {
	var sb strings.Builder

	// Current market state
	sb.WriteString("=== MARKET DATA ===\n\n")

	if prompt.MarketData != nil && prompt.MarketData.Ticker != nil {
		ticker := prompt.MarketData.Ticker
		sb.WriteString(fmt.Sprintf("Symbol: %s\n", ticker.Symbol))
		sb.WriteString(fmt.Sprintf("Current Price: $%.2f\n", models.ToFloat64(ticker.Last)))
		sb.WriteString(fmt.Sprintf("24h Change: %.2f%%\n", models.ToFloat64(ticker.Change24h)))
		sb.WriteString(fmt.Sprintf("24h High: $%.2f\n", models.ToFloat64(ticker.High24h)))
		sb.WriteString(fmt.Sprintf("24h Low: $%.2f\n", models.ToFloat64(ticker.Low24h)))
		sb.WriteString(fmt.Sprintf("24h Volume: $%.2f\n", models.ToFloat64(ticker.Volume24h)))
		sb.WriteString(fmt.Sprintf("Bid: $%.2f | Ask: $%.2f\n\n", models.ToFloat64(ticker.Bid), models.ToFloat64(ticker.Ask)))
	}

	// Multi-timeframe candles overview
	if prompt.MarketData != nil && len(prompt.MarketData.Candles) > 0 {
		sb.WriteString("=== MULTI-TIMEFRAME ANALYSIS ===\n\n")

		for _, tf := range []string{"5m", "15m", "1h", "4h"} {
			if candles, ok := prompt.MarketData.Candles[tf]; ok && len(candles) > 0 {
				latest := candles[len(candles)-1]
				prev := candles[len(candles)-2]

				change := ((models.ToFloat64(latest.Close) - models.ToFloat64(prev.Close)) / models.ToFloat64(prev.Close)) * 100

				trend := "↗️ Bullish"
				if change < -0.5 {
					trend = "↘️ Bearish"
				} else if change > -0.5 && change < 0.5 {
					trend = "→ Sideways"
				}

				sb.WriteString(fmt.Sprintf("%s timeframe: Close $%.2f (%+.2f%%) - %s\n",
					tf, models.ToFloat64(latest.Close), change, trend))
			}
		}
		sb.WriteString("\n")
	}

	// Technical indicators
	if prompt.MarketData != nil && prompt.MarketData.Indicators != nil {
		sb.WriteString("=== TECHNICAL INDICATORS ===\n\n")
		ind := prompt.MarketData.Indicators

		if len(ind.RSI) > 0 {
			sb.WriteString("RSI (Multi-Timeframe):\n")
			// Sort timeframes for consistent output
			for _, tf := range []string{"5m", "15m", "1h", "4h"} {
				if val, ok := ind.RSI[tf]; ok {
					rsiVal := models.ToFloat64(val)
					sb.WriteString(fmt.Sprintf("  %s: %.2f", tf, rsiVal))
					if rsiVal > 70 {
						sb.WriteString(" (⚠️ Overbought)\n")
					} else if rsiVal < 30 {
						sb.WriteString(" (✅ Oversold)\n")
					} else if rsiVal >= 45 && rsiVal <= 55 {
						sb.WriteString(" (➡️ Neutral zone)\n")
					} else {
						sb.WriteString("\n")
					}
				}
			}
			sb.WriteString("\n")
		}

		if ind.MACD != nil {
			macd := models.ToFloat64(ind.MACD.MACD)
			signal := models.ToFloat64(ind.MACD.Signal)
			histogram := models.ToFloat64(ind.MACD.Histogram)

			sb.WriteString("\nMACD:\n")
			sb.WriteString(fmt.Sprintf("  MACD: %.2f\n", macd))
			sb.WriteString(fmt.Sprintf("  Signal: %.2f\n", signal))
			sb.WriteString(fmt.Sprintf("  Histogram: %.2f", histogram))
			if histogram > 0 {
				sb.WriteString(" (Bullish)\n")
			} else {
				sb.WriteString(" (Bearish)\n")
			}
		}

		if ind.BollingerBands != nil {
			sb.WriteString("\nBollinger Bands:\n")
			sb.WriteString(fmt.Sprintf("  Upper: $%.2f\n", models.ToFloat64(ind.BollingerBands.Upper)))
			sb.WriteString(fmt.Sprintf("  Middle: $%.2f\n", models.ToFloat64(ind.BollingerBands.Middle)))
			sb.WriteString(fmt.Sprintf("  Lower: $%.2f\n", models.ToFloat64(ind.BollingerBands.Lower)))
		}

		if ind.Volume != nil {
			current := models.ToFloat64(ind.Volume.Current)
			average := models.ToFloat64(ind.Volume.Average)
			ratio := models.ToFloat64(ind.Volume.Ratio)

			sb.WriteString("\nVolume Analysis:\n")
			sb.WriteString(fmt.Sprintf("  Current: %.2f\n", current))
			sb.WriteString(fmt.Sprintf("  Average: %.2f\n", average))
			sb.WriteString(fmt.Sprintf("  Ratio: %.2fx", ratio))
			if ratio > 1.5 {
				sb.WriteString(" (High volume)\n")
			} else if ratio < 0.5 {
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
		fundingRate := models.ToFloat64(prompt.MarketData.FundingRate)
		openInterest := models.ToFloat64(prompt.MarketData.OpenInterest)

		sb.WriteString(fmt.Sprintf("Funding Rate: %.4f%%", fundingRate*100))
		if fundingRate > 0.01 {
			sb.WriteString(" (Longs pay shorts - bearish sentiment)\n")
		} else if fundingRate < -0.01 {
			sb.WriteString(" (Shorts pay longs - bullish sentiment)\n")
		} else {
			sb.WriteString(" (Neutral)\n")
		}
		sb.WriteString(fmt.Sprintf("Open Interest: $%.2f\n\n", openInterest))
	}

	// On-chain data
	if prompt.MarketData != nil && prompt.MarketData.OnChainData != nil {
		onchain := prompt.MarketData.OnChainData
		sb.WriteString("=== ON-CHAIN SIGNALS ===\n\n")

		sb.WriteString(fmt.Sprintf("Whale Activity: *%s*\n", strings.ToUpper(onchain.WhaleActivity)))

		netFlow, _ := onchain.NetExchangeFlow.Float64()
		sb.WriteString(fmt.Sprintf("Exchange Flow: %s (%.2f BTC net)\n",
			strings.ToUpper(onchain.ExchangeFlowDirection), netFlow))

		switch onchain.ExchangeFlowDirection {
		case "inflow":
			sb.WriteString("⚠️ Large inflow to exchanges = potential selling pressure\n")
		case "outflow":
			sb.WriteString("📈 Outflow from exchanges = accumulation (bullish)\n")
		}

		// High impact whale movements
		if len(onchain.RecentWhaleMovements) > 0 {
			sb.WriteString("\nRecent Whale Movements:\n")

			count := 0
			for _, whale := range onchain.RecentWhaleMovements {
				if whale.ImpactScore >= 7 && count < 3 {
					amountUSD, _ := whale.AmountUSD.Float64()
					emoji := "⚠️"
					switch whale.TransactionType {
					case "exchange_outflow":
						emoji = "📈"
					case "exchange_inflow":
						emoji = "📉"
					}

					ageMinutes := time.Since(whale.Timestamp).Minutes()
					sb.WriteString(fmt.Sprintf("%s %s: $%.1fM %s → %s (%.0f min ago)\n",
						emoji, whale.TransactionType,
						amountUSD/1_000_000,
						whale.FromOwner, whale.ToOwner,
						ageMinutes))
					count++
				}
			}
		}

		sb.WriteString("\n")
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
				bidVolume += models.ToFloat64(bid.Amount)
			}
			for _, ask := range ob.Asks[:min(10, len(ob.Asks))] {
				askVolume += models.ToFloat64(ask.Amount)
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
		sb.WriteString(fmt.Sprintf("Size: %.4f %s\n", models.ToFloat64(pos.Size), pos.Symbol[:3]))
		sb.WriteString(fmt.Sprintf("Entry Price: $%.2f\n", models.ToFloat64(pos.EntryPrice)))
		sb.WriteString(fmt.Sprintf("Current Price: $%.2f\n", models.ToFloat64(pos.CurrentPrice)))
		sb.WriteString(fmt.Sprintf("Leverage: %dx\n", pos.Leverage))
		sb.WriteString(fmt.Sprintf("Unrealized PnL: $%.2f", models.ToFloat64(pos.UnrealizedPnL)))

		pnlPercent := (models.ToFloat64(pos.UnrealizedPnL) / models.ToFloat64(pos.Margin)) * 100
		sb.WriteString(fmt.Sprintf(" (%.2f%%)\n", pnlPercent))
		sb.WriteString(fmt.Sprintf("Margin: $%.2f\n\n", models.ToFloat64(pos.Margin)))
	} else {
		sb.WriteString("=== CURRENT POSITION ===\n\n")
		sb.WriteString("No open position\n\n")
	}

	// Account info
	sb.WriteString("=== ACCOUNT INFO ===\n\n")
	balance := models.ToFloat64(prompt.Balance)
	equity := models.ToFloat64(prompt.Equity)
	dailyPnL := models.ToFloat64(prompt.DailyPnL)

	sb.WriteString(fmt.Sprintf("Balance: $%.2f\n", balance))
	sb.WriteString(fmt.Sprintf("Equity: $%.2f\n", equity))
	sb.WriteString(fmt.Sprintf("Daily PnL: $%.2f", dailyPnL))

	dailyPnLPercent := (dailyPnL / balance) * 100
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

			pnl := models.ToFloat64(trade.PnL)
			outcome := "Win"
			if pnl < 0 {
				outcome = "Loss"
				losses++
			} else if pnl > 0 {
				wins++
			}

			sb.WriteString(fmt.Sprintf("%d. %s %s @ $%.2f - PnL: $%.2f (%s)\n",
				i+1,
				string(trade.Side),
				trade.Symbol,
				models.ToFloat64(trade.Price),
				pnl,
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

// === NEWS EVALUATION PROMPTS ===

// buildNewsEvaluationSystemPrompt creates system prompt for news evaluation
func buildNewsEvaluationSystemPrompt() string {
	return `You are a professional crypto market analyst evaluating news impact.

Analyze the news and provide:
1. SENTIMENT: -1.0 to +1.0 (-1.0=very bearish, +1.0=very bullish, 0=neutral)
2. IMPACT: 1-10 scale (10=market-moving event, 1=noise)
3. URGENCY: IMMEDIATE, HOURS, or DAYS

IMPACT SCORING:
10: ETF approval/rejection, country adoption, major exchange hack, regulatory breakthrough
9:  Large institutional buys (>$100M), major partnerships, significant regulation
8:  Exchange listings, protocol upgrades, government statements
7:  Whale movements (>$10M), notable partnerships, regulatory news
6:  Medium institutional activity, analyst predictions from major firms
5:  Standard market updates, general news
3-4: Minor news, speculation
1-2: Noise, opinion pieces

URGENCY:
IMMEDIATE: Breaking news, hacks, approvals - affects price within minutes/hours
HOURS: Scheduled events, anticipated news - affects within 4-24 hours  
DAYS: Gradual developments, long-term trends - affects over days/weeks

IMPORTANT:
- Old news (>24h) = lower impact (already priced in)
- Rumors without sources = low confidence, low impact
- Consider source credibility (CoinDesk > random blog)
- Distinguish Bitcoin-specific vs general crypto news

Respond ONLY with valid JSON:
{
  "sentiment": 0.0,
  "impact": 5,
  "urgency": "HOURS",
  "reasoning": "brief explanation"
}`
}

// buildNewsEvaluationUserPrompt creates user prompt for news evaluation
func buildNewsEvaluationUserPrompt(newsItem *models.NewsItem) string {
	ageHours := time.Since(newsItem.PublishedAt).Hours()

	return fmt.Sprintf(`Evaluate this crypto news:

Source: %s
Title: %s
Content: %s
Published: %.1f hours ago

Provide sentiment, impact (1-10), urgency, and reasoning.`,
		newsItem.Source,
		newsItem.Title,
		truncateContent(newsItem.Content, 500),
		ageHours,
	)
}

// parseNewsEvaluation parses AI evaluation response
func parseNewsEvaluation(content string) (*NewsEvaluation, error) {
	jsonStr := extractJSON(content)

	var eval struct {
		Sentiment float64 `json:"sentiment"`
		Impact    int     `json:"impact"`
		Urgency   string  `json:"urgency"`
		Reasoning string  `json:"reasoning"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &eval); err != nil {
		return nil, fmt.Errorf("failed to unmarshal: %w", err)
	}

	// Validate
	if eval.Sentiment < -1.0 || eval.Sentiment > 1.0 {
		eval.Sentiment = 0
	}

	if eval.Impact < 1 || eval.Impact > 10 {
		eval.Impact = 5
	}

	if eval.Urgency != "IMMEDIATE" && eval.Urgency != "HOURS" && eval.Urgency != "DAYS" {
		eval.Urgency = "HOURS"
	}

	return &NewsEvaluation{
		Sentiment: eval.Sentiment,
		Impact:    eval.Impact,
		Urgency:   eval.Urgency,
		Reasoning: eval.Reasoning,
	}, nil
}

// NewsEvaluation represents AI evaluation of news
type NewsEvaluation struct {
	Sentiment float64 `json:"sentiment"`
	Impact    int     `json:"impact"`
	Urgency   string  `json:"urgency"`
	Reasoning string  `json:"reasoning"`
}

// truncateContent truncates content to maxLen characters
func truncateContent(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
