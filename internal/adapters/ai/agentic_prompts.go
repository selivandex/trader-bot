package ai

import (
	"fmt"

	"github.com/selivandex/trader-bot/pkg/models"
)

// AgenticPrompts provides generic prompts for all agentic AI methods
// This eliminates code duplication across different AI providers

// BuildReflectionPrompt creates system and user prompts for trade reflection
func BuildReflectionPrompt(reflectionPrompt *models.ReflectionPrompt) (systemPrompt string, userPrompt string) {
	systemPrompt = fmt.Sprintf(`You are an autonomous trading agent performing POST-TRADE REFLECTION.

%s

Your task is to deeply analyze this completed trade to extract actionable insights and improve future performance.

Think like a professional trader reviewing their trade journal:
- Be brutally honest about mistakes
- Identify what signals were most predictive
- Determine if your entry/exit timing was optimal
- Assess whether your risk management was appropriate
- Extract transferable lessons for similar future situations

Return ONLY valid JSON, no additional text.`, reflectionPrompt.AgentName)

	userPrompt = fmt.Sprintf(`=== TRADE COMPLETED ===

Symbol: %s
Direction: %s
Entry Price: $%.2f
Exit Price: $%.2f
Result: %s PnL $%.2f (%.2f%%)
Holding Period: %v

=== MY ORIGINAL REASONING ===
%s

=== REFLECTION TASKS ===

1. OUTCOME ANALYSIS:
   - Was the trade profitable? Why or why not?
   - Did price move as expected?
   - How accurate was my initial analysis?

2. SIGNAL ANALYSIS:
   - Which signals correctly predicted the outcome?
   - Which signals gave false indications?
   - Were there signals I ignored that I should have heeded?

3. EXECUTION QUALITY:
   - Was my entry timing good? Too early/late?
   - Was my exit optimal or could I have done better?
   - Did I follow my plan or deviate emotionally?

4. RISK MANAGEMENT:
   - Was position size appropriate?
   - Did stop loss/take profit levels make sense?
   - Could I have managed risk better?

5. LESSON EXTRACTION:
   - What is the KEY transferable lesson?
   - In what similar situations does this lesson apply?
   - How should I adjust my strategy weights?

Provide reflection in JSON:
{
  "analysis": "2-3 sentence overall analysis of what happened and why",
  "what_worked": ["specific insight about what was correct", "another strength"],
  "what_didnt_work": ["specific mistake or misjudgment", "another weakness"],
  "key_lessons": ["actionable lesson 1", "actionable lesson 2", "actionable lesson 3"],
  "suggested_adjustments": {
    "technical_weight": -0.05,
    "news_weight": 0.05,
    "stop_loss": 0.5
  },
  "memory_to_store": {
    "context": "Concise market situation (1 sentence)",
    "action": "What I did (1 sentence)",
    "outcome": "What happened (1 sentence)",
    "lesson": "Key transferable insight (actionable, specific)",
    "importance": 0.85
  },
  "confidence_in_analysis": 0.80
}`,
		reflectionPrompt.Trade.Symbol,
		reflectionPrompt.Trade.Side,
		reflectionPrompt.Trade.EntryPrice.InexactFloat64(),
		reflectionPrompt.Trade.ExitPrice.InexactFloat64(),
		func() string {
			if reflectionPrompt.Trade.WasSuccessful {
				return "✅ WIN"
			}
			return "❌ LOSS"
		}(),
		reflectionPrompt.Trade.PnL.InexactFloat64(),
		reflectionPrompt.Trade.PnLPercent,
		reflectionPrompt.Trade.Duration,
		reflectionPrompt.PriorBeliefs,
	)

	return systemPrompt, userPrompt
}

// BuildGenerateOptionsPrompt creates prompts for option generation
func BuildGenerateOptionsPrompt(situation *models.TradingSituation) (systemPrompt string, userPrompt string) {
	systemPrompt = `You are an autonomous trading agent in OPTION GENERATION phase.

Your task is to brainstorm 3-5 distinct trading strategies for the current market situation.

Think divergently:
- Generate options with different risk profiles (conservative to aggressive)
- Consider different time horizons (scalp, swing, position)
- Explore contrarian vs trend-following approaches
- Include a "do nothing" option if appropriate

Each option should be specific, actionable, and include clear risk parameters.

Return ONLY valid JSON array, no additional text.`

	memoriesContext := ""
	if len(situation.Memories) > 0 {
		memoriesContext = "\n\n=== RELEVANT PAST EXPERIENCES ===\n"
		for i, mem := range situation.Memories {
			if i < 3 {
				memoriesContext += fmt.Sprintf("Memory %d:\n", i+1)
				memoriesContext += fmt.Sprintf("  Context: %s\n", mem.Context)
				memoriesContext += fmt.Sprintf("  Lesson: %s\n", mem.Lesson)
				memoriesContext += fmt.Sprintf("  Importance: %.1f/1.0\n\n", mem.Importance)
			}
		}
	}

	positionContext := "No current position."
	if situation.CurrentPosition != nil && situation.CurrentPosition.Side != models.PositionNone {
		positionContext = fmt.Sprintf("Current Position: %s, Size: %.4f, Entry: $%.2f, PnL: $%.2f",
			situation.CurrentPosition.Side,
			situation.CurrentPosition.Size.InexactFloat64(),
			situation.CurrentPosition.EntryPrice.InexactFloat64(),
			situation.CurrentPosition.UnrealizedPnL.InexactFloat64(),
		)
	}

	userPrompt = fmt.Sprintf(`=== CURRENT MARKET SITUATION ===

Symbol: %s
Current Price: $%.2f
24h Change: %.2f%% 
Bid: $%.2f | Ask: $%.2f
24h High: $%.2f | Low: $%.2f
Volume: %.2f

%s
%s

=== YOUR TASK ===

Generate 3-5 distinct trading options. For EACH option specify:
- Clear action (OPEN_LONG, OPEN_SHORT, CLOSE, HOLD, SCALE_IN, SCALE_OUT)
- Specific parameters (size, leverage, stops, targets)
- Time horizon estimate
- Key assumptions
- Risk/reward ratio

Example options to consider:
1. Aggressive long: High leverage, tight stops, momentum play
2. Conservative long: Low leverage, wide stops, value play
3. Short: Fade the move, mean reversion
4. Hold/Wait: Conditions not optimal, preserve capital
5. Scale in/out: Adjust existing position

JSON format:
[
  {
    "option_id": "opt1_aggressive_long",
    "action": "OPEN_LONG",
    "description": "Aggressive momentum long targeting recent highs",
    "parameters": {
      "size": 0.5,
      "leverage": 4,
      "entry_price": %.2f,
      "stop_loss": %.2f,
      "take_profit": %.2f
    },
    "reasoning": "Strong 24h momentum (+%.2f%%), high volume confirms strength, RSI not overbought yet. Risk:Reward = 1:3"
  }
]

Generate 3-5 diverse options now.`,
		situation.MarketData.Symbol,
		situation.MarketData.Ticker.Last.InexactFloat64(),
		situation.MarketData.Ticker.Change24h.InexactFloat64(),
		situation.MarketData.Ticker.Bid.InexactFloat64(),
		situation.MarketData.Ticker.Ask.InexactFloat64(),
		situation.MarketData.Ticker.High24h.InexactFloat64(),
		situation.MarketData.Ticker.Low24h.InexactFloat64(),
		situation.MarketData.Ticker.Volume24h.InexactFloat64(),
		positionContext,
		memoriesContext,
		situation.MarketData.Ticker.Last.InexactFloat64(),
		situation.MarketData.Ticker.Last.InexactFloat64()*0.98, // 2% stop
		situation.MarketData.Ticker.Last.InexactFloat64()*1.05, // 5% target
		situation.MarketData.Ticker.Change24h.InexactFloat64(),
	)

	return systemPrompt, userPrompt
}

// BuildEvaluateOptionPrompt creates prompts for option evaluation
func BuildEvaluateOptionPrompt(option *models.TradingOption, memories []models.SemanticMemory) (systemPrompt string, userPrompt string) {
	systemPrompt = `You are an autonomous trading agent in CRITICAL EVALUATION phase.

Your task is to rigorously evaluate a proposed trading option.

Be like a risk manager AND an opportunity seeker:
- Identify every possible risk (market, execution, timing)
- Identify every potential opportunity
- Challenge the assumptions
- Consider what could go wrong
- Estimate probability-weighted outcomes
- Compare risk vs reward quantitatively

Be HONEST and CRITICAL. It's better to reject a mediocre option than accept it.

Return ONLY valid JSON, no additional text.`

	memoriesContext := ""
	if len(memories) > 0 {
		memoriesContext = "\n\n=== LESSONS FROM SIMILAR PAST SITUATIONS ===\n"
		for i, mem := range memories {
			if i < 3 {
				memoriesContext += fmt.Sprintf("\nMemory %d (Importance: %.1f/1.0):\n", i+1, mem.Importance)
				memoriesContext += fmt.Sprintf("  When: %s\n", mem.Context)
				memoriesContext += fmt.Sprintf("  I did: %s\n", mem.Action)
				memoriesContext += fmt.Sprintf("  Result: %s\n", mem.Outcome)
				memoriesContext += fmt.Sprintf("  Lesson: %s\n", mem.Lesson)
			}
		}
	}

	stopLoss := option.Parameters.StopLoss.InexactFloat64()
	takeProfit := option.Parameters.TakeProfit.InexactFloat64()
	entryPrice := option.Parameters.EntryPrice.InexactFloat64()

	riskPercent := 0.0
	rewardPercent := 0.0
	riskRewardRatio := "N/A"

	if entryPrice > 0 && stopLoss > 0 {
		riskPercent = abs(entryPrice-stopLoss) / entryPrice * 100
		if takeProfit > 0 {
			rewardPercent = abs(takeProfit-entryPrice) / entryPrice * 100
			if riskPercent > 0 {
				riskRewardRatio = fmt.Sprintf("1:%.2f", rewardPercent/riskPercent)
			}
		}
	}

	userPrompt = fmt.Sprintf(`=== OPTION TO EVALUATE ===

Option ID: %s
Action: %s
Description: %s

Parameters:
- Position Size: %.4f
- Leverage: %dx
- Entry: $%.2f
- Stop Loss: $%.2f (%.2f%% risk)
- Take Profit: $%.2f (%.2f%% reward)
- Risk:Reward Ratio: %s

Original Reasoning:
%s
%s

=== EVALUATION CRITERIA ===

1. PROBABILITY ANALYSIS:
   - What's the probability this trade works? (be specific: 60%%, 70%%?)
   - What assumptions must hold true?
   - What could invalidate this trade?

2. RISK ASSESSMENT:
   - What's the worst-case scenario?
   - What's the realistic downside?
   - Are there hidden risks (liquidity, slippage, gap)?
   - Is stop loss placement optimal?

3. REWARD ASSESSMENT:
   - What's the best-case scenario?
   - What's the realistic upside?
   - Is target achievable in reasonable timeframe?
   - Are there obstacles to reaching target?

4. TIMING ANALYSIS:
   - Is NOW the right time for this trade?
   - Should we wait for better entry?
   - Is there urgency or can we be patient?

5. COMPARISON WITH MEMORIES:
   - Have we been in similar situations before?
   - What do past experiences teach us?
   - Should we trust this setup or be cautious?

Provide evaluation in JSON:
{
  "option_id": "%s",
  "score": 75.5,
  "pros": ["specific advantage 1", "specific advantage 2", "specific advantage 3"],
  "cons": ["specific disadvantage 1", "specific disadvantage 2"],
  "risks": ["specific risk 1", "specific risk 2", "specific risk 3"],
  "opportunities": ["specific opportunity 1", "specific opportunity 2"],
  "expected_outcome": "Most likely: +2-3%% over 4-6h | Best case: +5-7%% | Worst case: -2%% (stop hit)",
  "confidence": 0.75,
  "reasoning": "Detailed 2-3 sentence analysis weighing all factors and referencing past lessons if applicable"
}

Score 0-100 based on:
- 90-100: Excellent setup, high conviction
- 75-89: Good setup, favorable odds
- 60-74: Acceptable setup, neutral to slightly positive
- 40-59: Questionable setup, more cons than pros
- 0-39: Poor setup, avoid`,
		option.OptionID,
		option.Action,
		option.Description,
		option.Parameters.Size.InexactFloat64(),
		option.Parameters.Leverage,
		entryPrice,
		stopLoss,
		riskPercent,
		takeProfit,
		rewardPercent,
		riskRewardRatio,
		option.Reasoning,
		memoriesContext,
		option.OptionID,
	)

	return systemPrompt, userPrompt
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// BuildFinalDecisionPrompt creates prompts for final decision
func BuildFinalDecisionPrompt(evaluations []models.OptionEvaluation) (systemPrompt string, userPrompt string) {
	systemPrompt = `You are an autonomous trading agent in FINAL DECISION phase.

You have generated multiple options and evaluated each thoroughly.
Now choose the single best action to take.

Decision criteria:
- Highest expected value (probability × reward - probability × risk)
- Alignment with your personality and risk tolerance
- Consideration of opportunity cost
- Overall market conditions

If no option scores above 60/100, choose HOLD - patience is a valid strategy.

Return ONLY valid JSON, no additional text.`

	// Format evaluations nicely
	evalsText := "=== EVALUATED OPTIONS ===\n\n"
	for i, eval := range evaluations {
		evalsText += fmt.Sprintf("Option %d: %s\n", i+1, eval.OptionID)
		evalsText += fmt.Sprintf("Score: %.1f/100\n", eval.Score)
		evalsText += fmt.Sprintf("Confidence: %.0f%%\n", eval.Confidence*100)
		evalsText += fmt.Sprintf("Expected Outcome: %s\n", eval.ExpectedOutcome)

		evalsText += "Pros:\n"
		for _, pro := range eval.Pros {
			evalsText += fmt.Sprintf("  ✓ %s\n", pro)
		}

		evalsText += "Cons:\n"
		for _, con := range eval.Cons {
			evalsText += fmt.Sprintf("  ✗ %s\n", con)
		}

		evalsText += "Risks:\n"
		for _, risk := range eval.Risks {
			evalsText += fmt.Sprintf("  ⚠️ %s\n", risk)
		}

		evalsText += fmt.Sprintf("Reasoning: %s\n\n", eval.Reasoning)
	}

	userPrompt = fmt.Sprintf(`%s

=== YOUR DECISION TASK ===

Review ALL evaluations above and choose the best action.

Consider:
1. Which option has highest score?
2. Which option has best risk:reward ratio?
3. Which option aligns with your personality?
4. Are you comfortable with the risks?
5. Is there a clear winner or are options similar?
6. Should you wait for better setup?

Decision rules:
- If best option scores 80-100: High conviction, execute
- If best option scores 60-79: Moderate conviction, execute with caution
- If best option scores < 60: Low conviction, HOLD
- If options are close in score: Choose the safer one

Provide final decision in JSON:
{
  "action": "OPEN_LONG" | "OPEN_SHORT" | "CLOSE" | "HOLD" | "SCALE_IN" | "SCALE_OUT",
  "reason": "I chose [action] because [2-3 sentence reasoning referencing evaluation scores and your conviction level]",
  "confidence": 80
}

Confidence calibration:
- 90-100: Extremely high conviction, all factors aligned
- 75-89: High conviction, most factors favorable
- 60-74: Moderate conviction, acceptable setup
- 40-59: Low conviction, marginal setup
- 0-39: Very low conviction, avoid trade`, evalsText)

	return systemPrompt, userPrompt
}

// BuildCreatePlanPrompt creates prompts for trading plan
func BuildCreatePlanPrompt(planRequest *models.PlanRequest) (systemPrompt string, userPrompt string) {
	systemPrompt = fmt.Sprintf(`You are an autonomous trading agent creating a FORWARD-LOOKING PLAN for the next %v.

%s

Think like a chess player:
- Anticipate different market scenarios
- Plan your response to each scenario BEFORE it happens
- Set clear trigger conditions
- Define risk limits that protect capital
- Assign probabilities to scenarios
- Create contingency plans

The plan should be:
- Specific and actionable
- Probabilistic (assign realistic probabilities)
- Adaptive (define when to revise)
- Risk-aware (set hard limits)

Return ONLY valid JSON, no additional text.`, planRequest.TimeHorizon, planRequest.AgentName)

	memoriesContext := ""
	if len(planRequest.Memories) > 0 {
		memoriesContext = "\n\n=== LESSONS FROM PAST ===\n"
		for i, mem := range planRequest.Memories {
			if i < 5 {
				memoriesContext += fmt.Sprintf("- %s\n", mem.Lesson)
			}
		}
	}

	positionContext := "No current position."
	if planRequest.CurrentPosition != nil && planRequest.CurrentPosition.Side != models.PositionNone {
		positionContext = fmt.Sprintf("Current Position: %s at $%.2f, PnL: $%.2f",
			planRequest.CurrentPosition.Side,
			planRequest.CurrentPosition.EntryPrice.InexactFloat64(),
			planRequest.CurrentPosition.UnrealizedPnL.InexactFloat64(),
		)
	}

	userPrompt = fmt.Sprintf(`=== CURRENT MARKET STATE ===

Symbol: %s
Price: $%.2f
24h Change: %.2f%%
Risk Tolerance: %.1f%% of capital per trade

%s
%s

=== CREATE TRADING PLAN ===

Your plan should cover 3-5 distinct market scenarios:
1. BULLISH scenario (what if price goes up?)
2. BEARISH scenario (what if price goes down?)
3. SIDEWAYS scenario (what if price consolidates?)
4. EXTREME scenarios if relevant (flash crash, massive pump)

For EACH scenario specify:
- Descriptive name
- Probability estimate (must sum to ~1.0)
- Technical/fundamental indicators that signal this scenario
- Your planned action
- Clear reasoning

Also define:
- Key assumptions your plan relies on
- Hard risk limits (max drawdown, daily loss, position size)
- Trigger signals that would force plan revision

Example:
{
  "assumptions": [
    "BTC will remain above $40k support",
    "No major regulatory news expected",
    "Correlation with traditional markets continues"
  ],
  "scenarios": [
    {
      "name": "Bullish breakout above $45k",
      "probability": 0.25,
      "indicators": ["Price breaks $45k with volume", "RSI crosses 65", "MACD histogram expanding"],
      "action": "Go long 3x leverage, target $48k, stop $44k",
      "reasoning": "Breakout continuation likely given recent consolidation. Volume confirms institutional buying."
    },
    {
      "name": "Consolidation $42k-$45k",
      "probability": 0.50,
      "indicators": ["RSI 40-60", "Low volume", "Tight Bollinger Bands"],
      "action": "Wait for breakout, preserve capital, maybe scalp small moves",
      "reasoning": "Range-bound market offers low risk:reward. Better to wait for clarity."
    },
    {
      "name": "Bearish breakdown below $42k",
      "probability": 0.20,
      "indicators": ["Price breaks $42k support", "RSI < 40", "Volume spike on dump"],
      "action": "Go short 2x leverage, target $40k, stop $42.5k",
      "reasoning": "Support broken means likely cascade. Manage risk tightly."
    },
    {
      "name": "Extreme volatility event",
      "probability": 0.05,
      "indicators": ["News impact > 9", "Volume > 3x average", "Price moves > 8%% in 1h"],
      "action": "Close all positions immediately, reassess",
      "reasoning": "Black swan protection. Capital preservation over opportunity."
    }
  ],
  "risk_limits": {
    "max_drawdown": 5.0,
    "max_daily_loss": 100.0,
    "max_position_size": %.1f,
    "stop_trading_if": "3 consecutive losses or circuit breaker triggered"
  },
  "trigger_signals": [
    {
      "condition": "Volume spikes > 3x average",
      "action": "Reassess all scenarios immediately"
    },
    {
      "condition": "High-impact news (score 9-10)",
      "action": "Revise scenarios based on news catalyst"
    },
    {
      "condition": "Price moves > 5%% in 2 hours",
      "action": "Check if scenarios still valid"
    }
  ]
}

Create comprehensive plan now.`,
		planRequest.MarketData.Symbol,
		planRequest.MarketData.Ticker.Last.InexactFloat64(),
		planRequest.MarketData.Ticker.Change24h.InexactFloat64(),
		planRequest.RiskTolerance*100,
		positionContext,
		memoriesContext,
		planRequest.RiskTolerance*100, // max_position_size
	)

	return systemPrompt, userPrompt
}

// BuildSelfAnalysisPrompt creates prompts for self-analysis
func BuildSelfAnalysisPrompt(performance *models.PerformanceData) (systemPrompt string, userPrompt string) {
	systemPrompt = fmt.Sprintf(`You are %s performing SELF-ANALYSIS of your trading performance.

This is metacognition - thinking about your own thinking.

Be like a coach reviewing game tape:
- Brutally honest about mistakes
- Recognize genuine strengths
- Dig deep for root causes
- Suggest specific, measurable improvements
- Back recommendations with data

The goal is ADAPTATION - becoming a better trader based on evidence.

Return ONLY valid JSON, no additional text.`, performance.AgentName)

	signalPerf := "=== SIGNAL-BY-SIGNAL BREAKDOWN ===\n\n"
	for signalType, perf := range performance.SignalPerformance {
		signalPerf += fmt.Sprintf("%s Signal:\n", capitalize(signalType))
		signalPerf += fmt.Sprintf("  Current Weight: %.0f%%\n", perf.CurrentWeight*100)
		signalPerf += fmt.Sprintf("  Win Rate: %.1f%%\n", perf.WinRate*100)
		signalPerf += fmt.Sprintf("  Performance: %s\n\n", func() string {
			if perf.WinRate > 0.6 {
				return "✅ STRONG"
			} else if perf.WinRate > 0.5 {
				return "➡️ NEUTRAL"
			}
			return "❌ WEAK"
		}())
	}

	recentTradesContext := ""
	if len(performance.RecentTrades) > 0 {
		recentTradesContext = "\n=== RECENT TRADES SAMPLE ===\n"
		for i, trade := range performance.RecentTrades {
			if i < 5 {
				result := "LOSS"
				if trade.WasSuccessful {
					result = "WIN"
				}
				recentTradesContext += fmt.Sprintf("%d. %s: %.2f%% (%s) - %s\n",
					i+1, trade.Side, trade.PnLPercent, result, trade.EntryReason)
			}
		}
	}

	userPrompt = fmt.Sprintf(`=== PERFORMANCE REVIEW PERIOD: %v ===

=== OVERALL STATISTICS ===

Total Trades: %d
Win Rate: %.1f%% (%d wins, %d losses)
Total PnL: $%.2f
Average PnL per trade: $%.2f
Best Trade: +$%.2f
Worst Trade: -$%.2f
Profit Factor: %.2f

%s
%s

=== CURRENT STRATEGY CONFIGURATION ===

Signal Weights:
- Technical Analysis: %.0f%%
- News/Fundamentals: %.0f%%
- On-Chain Data: %.0f%%
- Market Sentiment: %.0f%%

=== SELF-ANALYSIS FRAMEWORK ===

Perform deep analysis following this framework:

1. PERFORMANCE DIAGNOSIS:
   - Is my overall performance acceptable? (50%%+ win rate = good)
   - Am I profitable or losing money?
   - Are wins larger than losses on average?
   - Am I consistent or erratic?

2. SIGNAL EFFECTIVENESS ANALYSIS:
   For EACH signal type:
   - Is this signal generating wins or losses?
   - Am I weighting it correctly?
   - Should I increase/decrease reliance on it?
   
3. ROOT CAUSE IDENTIFICATION:
   - WHY am I making mistakes?
   - Is it poor timing? Wrong signals? Emotion? Bad risk management?
   - Are there patterns in my losses?
   - What market conditions do I struggle with?

4. STRENGTHS TO LEVERAGE:
   - What am I doing RIGHT?
   - Which signals am I reading well?
   - What should I do MORE of?

5. CONCRETE IMPROVEMENTS:
   - Suggest NEW signal weights (must sum to 1.0)
   - Suggest parameter changes (stop loss, take profit, position size)
   - Suggest behavioral changes
   - Explain WHY each change will help

Provide self-analysis in JSON:
{
  "performance_assessment": "2-3 sentence honest assessment. Am I profitable? Consistent? What's the overall verdict?",
  "strengths_identified": [
    "Specific strength with evidence (e.g., 'Technical analysis working well - 65%% win rate on technical-heavy trades')",
    "Another strength with data"
  ],
  "weaknesses_identified": [
    "Specific weakness with evidence (e.g., 'News signals failing - only 40%% win rate, possibly reacting too fast')",
    "Another weakness with data"
  ],
  "root_causes": [
    "Root cause of weakness 1 (e.g., 'Overweighting news despite poor performance - cognitive bias?')",
    "Root cause of weakness 2"
  ],
  "suggested_changes": {
    "new_weights": {
      "technical_weight": 0.50,
      "news_weight": 0.20,
      "onchain_weight": 0.20,
      "sentiment_weight": 0.10
    },
    "parameter_adjustments": {
      "stop_loss": 2.5,
      "take_profit": 6.0
    },
    "behavioral_changes": [
      "Specific behavior change 1",
      "Specific behavior change 2"
    ],
    "signals_to_emphasize": ["technical"],
    "signals_to_deemphasize": ["news"],
    "reasoning": "Detailed explanation: Technical signals show 65%% win rate vs news at 40%%. Math says increase technical weight by 10%%, decrease news by 10%%. This should improve overall win rate by est. 5%%."
  },
  "confidence": 0.85
}

Confidence in analysis:
- 0.9-1.0: Very strong evidence, clear patterns
- 0.7-0.9: Good sample size, reliable conclusions
- 0.5-0.7: Limited data, tentative conclusions
- < 0.5: Insufficient data for reliable analysis`,
		performance.TimeWindow,
		performance.TotalTrades,
		performance.WinRate*100,
		performance.TotalTrades-int(float64(performance.TotalTrades)*(1-performance.WinRate)),
		int(float64(performance.TotalTrades)*(1-performance.WinRate)),
		performance.TotalPnL.InexactFloat64(),
		performance.AvgPnL.InexactFloat64(),
		performance.MaxWin.InexactFloat64(),
		performance.MaxLoss.InexactFloat64(),
		func() float64 {
			if performance.MaxLoss.InexactFloat64() != 0 {
				return abs(performance.MaxWin.InexactFloat64() / performance.MaxLoss.InexactFloat64())
			}
			return 0
		}(),
		signalPerf,
		recentTradesContext,
		performance.CurrentWeights.TechnicalWeight*100,
		performance.CurrentWeights.NewsWeight*100,
		performance.CurrentWeights.OnChainWeight*100,
		performance.CurrentWeights.SentimentWeight*100,
	)

	return systemPrompt, userPrompt
}

func capitalize(s string) string {
	if len(s) == 0 {
		return s
	}
	return string(s[0]-32) + s[1:]
}

// BuildSummarizeMemoryPrompt creates prompts for memory summarization
func BuildSummarizeMemoryPrompt(experience *models.TradeExperience) (systemPrompt string, userPrompt string) {
	systemPrompt = `You are creating an EPISODIC MEMORY from a trading experience.

Think like a human forming long-term memories:
- What's the CORE insight worth remembering?
- In what FUTURE situations will this memory be relevant?
- How can I phrase this for easy recall?

Memory should be:
- Concise (1 sentence each)
- Specific (not generic advice)
- Actionable (tells me what to DO in similar situations)
- Searchable (easy to recall when relevant)

Return ONLY valid JSON, no additional text.`

	signalsUsed := ""
	if len(experience.SignalsUsed) > 0 {
		signalsUsed = "\n\nSignals I relied on:\n"
		for signal, score := range experience.SignalsUsed {
			signalsUsed += fmt.Sprintf("- %s: %.0f/100\n", signal, score)
		}
	}

	userPrompt = fmt.Sprintf(`=== TRADE EXPERIENCE TO REMEMBER ===

Trade: %s %s on %s
Entry: $%.2f → Exit: $%.2f
Result: %s
PnL: $%.2f (%.2f%%)
Duration: %v
%s

=== CONTEXT ===

Why I entered:
%s

Why I exited:
%s

=== MEMORY EXTRACTION TASK ===

Extract the ONE most important lesson from this experience.

Ask yourself:
1. What was the KEY factor that made this trade succeed/fail?
2. What should I remember for next time I'm in similar situation?
3. Is this lesson broadly applicable or situation-specific?
4. How confident am I in this lesson?

Examples of GOOD memories:
✓ "When BTC breaks major resistance with 2x volume, continuation is likely - don't exit too early"
✓ "News-driven selloffs in bull markets are often buying opportunities within 24h"
✓ "Whale exchange outflows > $50M reliably predict 2-5%% rallies within 48h"

Examples of BAD memories (too generic):
✗ "Technical analysis works"
✗ "Be patient"
✗ "Manage risk"

Create memory summary in JSON:
{
  "context": "Specific market situation (1 sentence, searchable). Example: 'BTC dumped 5%% on SEC lawsuit news'",
  "action": "What I did (1 sentence, specific). Example: 'Went short at $42k with 3x leverage'",
  "outcome": "What happened (1 sentence, specific). Example: 'Price recovered within 12h, stopped out for -2%%'",
  "lesson": "KEY ACTIONABLE TAKEAWAY (1-2 sentences). Example: 'Legal FUD in bull markets creates false breakdown signals. Wait 24h for confirmation before shorting news-driven dips.'",
  "importance": 0.75
}

Importance rating guide:
- 0.9-1.0: CRITICAL insight (rare, game-changing)
  Example: Discovered major pattern that changes whole strategy
- 0.7-0.9: IMPORTANT lesson (valuable, will use often)
  Example: Reliable signal combination or common mistake to avoid
- 0.5-0.7: USEFUL reference (helpful, situational)
  Example: Works in specific market conditions
- 0.3-0.5: MINOR note (low impact)
  Example: Small optimization or edge case
- 0.0-0.3: FORGETTABLE (not worth storing)
  Example: Random outcome, no clear lesson

Rate this memory's importance realistically.`,
		experience.Side,
		experience.Symbol,
		experience.Symbol,
		experience.EntryPrice.InexactFloat64(),
		experience.ExitPrice.InexactFloat64(),
		func() string {
			if experience.WasSuccessful {
				return "✅ WIN"
			}
			return "❌ LOSS"
		}(),
		experience.PnL.InexactFloat64(),
		experience.PnLPercent,
		experience.Duration,
		signalsUsed,
		experience.EntryReason,
		experience.ExitReason,
	)

	return systemPrompt, userPrompt
}
