<!-- @format -->

# Agent System Improvement Plan

**Date:** November 23, 2025  
**Status:** Action Required  
**Priority:** High

---

## 📋 Executive Summary

The agentic trading system is **architecturally strong (8/10)** with well-designed autonomous AI agents, but has **critical issues** that must be addressed before production deployment. This document outlines problems, recommendations, and an actionable checklist.

---

## 🔴 Critical Problems (Must Fix Before Production)

### 1. CRITICAL: Broken Semantic Embeddings

**File:** `internal/agents/semantic_memory.go:190-217`

**Problem:**

```go
// This is a PLACEHOLDER - not real embeddings!
func (smm *SemanticMemoryManager) generateSimpleEmbedding(text string) []float32 {
    embedding := make([]float32, 128)
    for i, char := range text {
        idx := (int(char) + i) % 128
        embedding[idx] += 1.0
    }
    // Bag-of-words approach - DOES NOT WORK for semantic search
}
```

**Impact:**

- Memory recall returns irrelevant memories
- Agents cannot learn from past experiences effectively
- Collective memory sharing is broken

**Solution Options:**

**Option A: OpenAI Embeddings API (Recommended)**

```go
import "github.com/sashabaranov/go-openai"

func (smm *SemanticMemoryManager) generateEmbedding(text string) ([]float32, error) {
    resp, err := smm.openaiClient.CreateEmbeddings(context.Background(),
        openai.EmbeddingRequest{
            Model: openai.AdaEmbeddingV2,
            Input: []string{text},
        })
    if err != nil {
        return nil, err
    }
    return resp.Data[0].Embedding, nil
}
```

**Option B: Local Model (ONNX Runtime)**

- Use `all-MiniLM-L6-v2` or `all-mpnet-base-v2`
- No API costs, but requires model deployment
- Library: `github.com/knights-analytics/hugot`

**Cost Estimate (OpenAI):**

- ~1000 tokens per memory = $0.0001 per embedding
- 100 memories/day = $0.01/day = $3.65/year per agent
- Acceptable for production

**Timeline:** 2-3 days  
**Priority:** P0 - CRITICAL

---

### 2. CRITICAL: No Circuit Breaker for Agents

**File:** `internal/agents/manager.go:287-404`

**Problem:**
Agents can continue trading indefinitely even with large losses. No automatic shutdown on:

- Large drawdown (-20%+)
- Consecutive losses (5+ in a row)
- Daily loss limits

**Impact:**

- Single agent can blow up entire account
- No protection against cascade failures
- Risk management is inadequate

**Solution:**

Add circuit breaker in `executeAgenticCycle()`:

```go
func (am *AgenticManager) executeAgenticCycle(ctx context.Context, runner *AgenticRunner) error {
    // Circuit breaker checks
    if err := am.checkCircuitBreaker(runner); err != nil {
        logger.Warn("Circuit breaker triggered",
            zap.String("agent", runner.Config.Name),
            zap.Error(err),
        )

        // Stop agent
        am.StopAgenticAgent(ctx, runner.Config.ID)

        // Send alert
        if am.notifier != nil {
            am.notifier.SendCircuitBreakerAlert(ctx,
                runner.Config.UserID,
                runner.Config.Name,
                err.Error(),
            )
        }
        return err
    }

    // ... existing code
}

func (am *AgenticManager) checkCircuitBreaker(runner *AgenticRunner) error {
    // Get recent trades
    recentTrades, err := am.repository.GetRecentTrades(ctx, runner.Config.ID, 10)
    if err != nil {
        return nil // Don't block on DB errors
    }

    // Check 1: Max drawdown (from initial balance)
    currentPnL := runner.State.PnL.InexactFloat64()
    initialBalance := runner.State.InitialBalance.InexactFloat64()
    drawdownPct := (currentPnL / initialBalance) * 100

    maxDrawdown := -20.0 // 20% max drawdown
    if drawdownPct < maxDrawdown {
        return fmt.Errorf("max drawdown exceeded: %.2f%% (limit: %.2f%%)",
            drawdownPct, maxDrawdown)
    }

    // Check 2: Consecutive losses
    consecutiveLosses := 0
    for _, trade := range recentTrades {
        if trade.PnL.LessThan(decimal.Zero) {
            consecutiveLosses++
        } else {
            break
        }
    }

    maxConsecutiveLosses := 5
    if consecutiveLosses >= maxConsecutiveLosses {
        return fmt.Errorf("%d consecutive losses (limit: %d)",
            consecutiveLosses, maxConsecutiveLosses)
    }

    // Check 3: Daily loss limit
    dailyPnL, err := am.repository.GetDailyPnL(ctx, runner.Config.ID)
    if err == nil {
        maxDailyLoss := initialBalance * 0.05 // 5% of initial balance
        dailyLoss := dailyPnL.InexactFloat64()

        if dailyLoss < -maxDailyLoss {
            return fmt.Errorf("daily loss limit exceeded: $%.2f (limit: $%.2f)",
                dailyLoss, maxDailyLoss)
        }
    }

    return nil
}
```

Add to repository:

```go
func (r *Repository) GetRecentTrades(ctx context.Context, agentID string, limit int) ([]Trade, error)
func (r *Repository) GetDailyPnL(ctx context.Context, agentID string) (decimal.Decimal, error)
```

**Timeline:** 1-2 days  
**Priority:** P0 - CRITICAL

---

### 3. HIGH: AI Provider Failure Handling

**File:** `internal/agents/cot_engine.go:92-96`

**Problem:**

```go
options, err := cot.aiProvider.GenerateOptions(ctx, situation)
if err != nil {
    return nil, nil, fmt.Errorf("failed to generate options: %w", err)
    // ❌ Agent stops thinking completely if AI fails
}
```

If any step in Chain-of-Thought fails, agent makes NO decision at all.

**Impact:**

- Agent becomes non-functional on AI provider outage
- Missed trading opportunities
- No graceful degradation

**Solution:**

Add fallback to signal-based decision:

```go
// In cot_engine.go
func (cot *ChainOfThoughtEngine) Think(
    ctx context.Context,
    marketData *models.MarketData,
    position *models.Position,
) (*models.AgentDecision, *ai.ReasoningTrace, error) {
    // Try full CoT first
    decision, trace, err := cot.thinkWithCoT(ctx, marketData, position)
    if err != nil {
        logger.Warn("CoT failed, using fallback decision",
            zap.String("agent", cot.config.Name),
            zap.Error(err),
        )
        return cot.fallbackDecision(ctx, marketData, position)
    }
    return decision, trace, nil
}

func (cot *ChainOfThoughtEngine) fallbackDecision(
    ctx context.Context,
    marketData *models.MarketData,
    position *models.Position,
) (*models.AgentDecision, *ai.ReasoningTrace, error) {
    // Use signal analyzer only (no AI)
    signals := cot.signalAnalyzer.AnalyzeSignals(marketData)

    // Calculate weighted score
    score := 0.0
    score += signals.Technical.Score * cot.config.Specialization.TechnicalWeight
    score += signals.News.Score * cot.config.Specialization.NewsWeight
    score += signals.OnChain.Score * cot.config.Specialization.OnChainWeight
    score += signals.Sentiment.Score * cot.config.Specialization.SentimentWeight

    // Determine action
    var action models.AIAction
    confidence := int(score)

    if score > 65 && position == nil {
        action = models.ActionOpenLong
    } else if score < 35 && position == nil {
        action = models.ActionOpenShort
    } else if position != nil && (score > 70 || score < 30) {
        action = models.ActionClose
    } else {
        action = models.ActionHold
    }

    decision := &models.AgentDecision{
        AgentID:        cot.config.ID,
        Symbol:         marketData.Symbol,
        Action:         action,
        Confidence:     confidence,
        Reason:         "[FALLBACK MODE] AI unavailable, using signal-based decision",
        TechnicalScore: signals.Technical.Score,
        NewsScore:      signals.News.Score,
        OnChainScore:   signals.OnChain.Score,
        SentimentScore: signals.Sentiment.Score,
        FinalScore:     score,
    }

    return decision, &ai.ReasoningTrace{}, nil
}
```

**Timeline:** 1 day  
**Priority:** P1 - HIGH

---

### 4. HIGH: Validator Template Fallback Quality

**File:** `internal/agents/validator_council.go:379-413`

**Problem:**

```go
if vc.templateManager == nil {
    // Falls back to primitive prompts
    return vc.buildFallbackPrompt(role, decision, marketData)
}

// Fallback is too simple and doesn't provide proper guidance
```

**Impact:**

- If templates fail to load, validation quality drops significantly
- Risk of approving bad trades

**Solution:**

Improve fallback prompts by hardcoding comprehensive ones:

```go
func (vc *ValidatorCouncil) buildFallbackPrompt(
    role ValidatorRole,
    decision *models.AgentDecision,
    marketData *models.MarketData,
) (systemPrompt string, userPrompt string) {

    // Get comprehensive role-specific prompt from embedded string
    switch role {
    case RoleRiskManager:
        systemPrompt = getRiskManagerPrompt()
    case RoleTechnicalExpert:
        systemPrompt = getTechnicalExpertPrompt()
    case RoleMarketPsychologist:
        systemPrompt = getMarketPsychologistPrompt()
    }

    // Build detailed user prompt
    userPrompt = fmt.Sprintf(`
Validate this trading decision:

**Agent Profile:**
- Name: %s
- Personality: %s
- Typical Position Size: %.0f%%
- Leverage: %dx

**Proposed Decision:**
- Action: %s
- Symbol: %s
- Agent Confidence: %d%%
- Reasoning: %s

**Market Context:**
- Current Price: $%.2f
- 24h Change: %.2f%%
- Volume 24h: $%.0f

**Technical Indicators:**
%s

**Recent News:**
%s

**Your Task:**
Analyze this decision from your role's perspective and respond in JSON:
{
  "verdict": "APPROVE|REJECT|ABSTAIN",
  "confidence": 0-100,
  "reasoning": "Your detailed analysis",
  "key_risks": ["risk1", "risk2"],
  "key_opportunities": ["opp1"],
  "recommended_changes": "If rejecting, what should change",
  "critical_concerns": "Any red flags"
}
`,
        vc.agentConfig.Name,
        vc.agentConfig.Personality,
        vc.agentConfig.Strategy.MaxPositionPercent,
        vc.agentConfig.Strategy.MaxLeverage,
        decision.Action,
        decision.Symbol,
        decision.Confidence,
        decision.Reason,
        marketData.Ticker.Last.InexactFloat64(),
        marketData.Ticker.Change24h.InexactFloat64(),
        marketData.Ticker.Volume24h.InexactFloat64(),
        formatIndicators(marketData.Indicators),
        formatNews(marketData.NewsSummary),
    )

    return systemPrompt, userPrompt
}

// Embed comprehensive prompts as constants
const riskManagerPrompt = `You are a Risk Manager validator in a trading council.

Your PRIMARY CONCERN is downside protection and capital preservation.

Critical Evaluation Criteria:
1. Risk/Reward Ratio: Must be >2:1 for approval
2. Stop-Loss Placement: Is it adequate for market volatility?
3. Position Sizing: Is it appropriate given market conditions?
4. Hidden Risks: What could the agent have missed?
5. Worst-Case Scenario: What's the maximum loss?

Approval Standards:
- Risk/reward must be clearly favorable
- Stop-loss must be well-placed (not too tight, not too loose)
- Market conditions must be stable enough for the trade
- No major upcoming catalysts that could invalidate thesis
- Agent's reasoning must account for primary risks

Rejection Triggers:
- R:R ratio < 2:1
- Inadequate stop-loss
- Trading during high volatility without justification
- Ignoring obvious risks
- Overleveraged position

Be strict but fair. It's better to reject a good trade than approve a bad one.`
```

Create separate constants for each role with 200-300 words of guidance.

**Timeline:** 0.5 days  
**Priority:** P1 - HIGH

---

## 🟡 Medium Priority Issues

### 5. MEDIUM: Learning Rate Safety

**File:** `internal/agents/personalities.go:95`

**Problem:**

```go
// Aggressive agent adapts very fast
LearningRate: 0.15,  // 15% - can adapt to noise
```

Agents can overfit to recent random outcomes, especially aggressive/scalper personalities.

**Solution:**

Add safeguards in `reflection.go`:

```go
func (re *ReflectionEngine) applyAdjustments(ctx context.Context, adjustments map[string]float64) error {
    // Get trade count to ensure enough data
    metrics, err := re.repository.GetAgentPerformanceMetrics(ctx, re.config.ID, "")
    if err != nil {
        return err
    }

    // Require minimum trades before adapting
    minTradesForAdaptation := 20
    if metrics.TotalTrades < minTradesForAdaptation {
        logger.Info("Not enough trades for adaptation",
            zap.String("agent", re.config.Name),
            zap.Int("trades", metrics.TotalTrades),
            zap.Int("required", minTradesForAdaptation),
        )
        return nil
    }

    // Limit maximum change per adaptation
    maxChangePerStep := 0.10
    for key, change := range adjustments {
        if math.Abs(change) > maxChangePerStep {
            logger.Warn("Capping large adjustment",
                zap.String("agent", re.config.Name),
                zap.String("parameter", key),
                zap.Float64("requested", change),
                zap.Float64("capped", maxChangePerStep),
            )
            if change > 0 {
                adjustments[key] = maxChangePerStep
            } else {
                adjustments[key] = -maxChangePerStep
            }
        }
    }

    // ... existing code
}
```

**Timeline:** 0.5 days  
**Priority:** P2 - MEDIUM

---

### 6. MEDIUM: Memory Growth Control

**Problem:**
Agents accumulate memories indefinitely. While weekly cleanup exists, there's no hard limit.

**Solution:**

Add max memory limit in `semantic_memory.go`:

```go
func (smm *SemanticMemoryManager) Store(ctx context.Context, agentID string, personality string, experience *models.TradeExperience) error {
    // Check memory count
    count, err := smm.repository.CountMemories(ctx, agentID)
    if err == nil && count >= 1000 {
        // Cleanup oldest low-importance memories
        logger.Info("Memory limit reached, cleaning up",
            zap.String("agent_id", agentID),
            zap.Int("count", count),
        )
        smm.Forget(ctx, agentID, 0.4) // Remove importance < 0.4
    }

    // ... existing store logic
}
```

**Timeline:** 0.5 days  
**Priority:** P2 - MEDIUM

---

### 7. MEDIUM: Plan Scenario Matching Logic

**File:** `internal/agents/planner.go:246-259`

**Problem:**

```go
func (pe *PlanningEngine) findMatchingScenario(plan *models.TradingPlan, marketData *models.MarketData) *models.Scenario {
    // Simplified - just returns first scenario with prob > 0.3
    for _, scenario := range plan.Scenarios {
        if scenario.Probability > 0.3 {
            return &scenario  // ❌ Too simplistic
        }
    }
    return nil
}
```

**Solution:**

Use AI to match scenarios:

```go
func (pe *PlanningEngine) findMatchingScenario(plan *models.TradingPlan, marketData *models.MarketData) *models.Scenario {
    // Build prompt for AI to evaluate which scenario matches
    prompt := fmt.Sprintf(`Given current market conditions and trading plan scenarios, which scenario best matches?

Current Market:
- Price: $%.2f
- 24h Change: %.2f%%
- RSI: %.1f
- Volume Ratio: %.1fx

Scenarios:
%s

Return JSON with matching scenario number (1-based) and confidence (0-100).`,
        marketData.Ticker.Last.InexactFloat64(),
        marketData.Ticker.Change24h.InexactFloat64(),
        getRSI(marketData),
        getVolumeRatio(marketData),
        formatScenariosForPrompt(plan.Scenarios),
    )

    // Use AI to match (with fallback)
    match, err := pe.aiProvider.MatchScenario(ctx, prompt, plan.Scenarios)
    if err != nil {
        // Fallback to probability
        return findHighestProbabilityScenario(plan.Scenarios)
    }

    return match
}
```

**Timeline:** 1 day  
**Priority:** P2 - MEDIUM

---

## 💡 Enhancement Recommendations

### 8. Backtesting Framework

**Value:** Critical for validating agent strategies before live trading.

**Implementation:**

```go
// internal/agents/backtest.go
type Backtester struct {
    manager    *AgenticManager
    dataSource HistoricalDataSource
}

func (bt *Backtester) RunBacktest(
    ctx context.Context,
    agentConfig *models.AgentConfig,
    symbol string,
    startDate, endDate time.Time,
) (*BacktestResult, error) {
    // Create agent in simulation mode
    agent := bt.createSimulatedAgent(agentConfig)

    // Replay historical data
    timePoints := bt.dataSource.GetTimePoints(symbol, startDate, endDate)

    for _, t := range timePoints {
        marketData := bt.dataSource.GetMarketDataAt(symbol, t)

        // Run agent decision cycle
        decision, _, err := agent.CoTEngine.Think(ctx, marketData, nil)
        if err != nil {
            continue
        }

        // Simulate execution
        bt.simulateExecution(agent, decision, marketData)
    }

    return bt.calculateResults(agent), nil
}
```

**Files to create:**

- `internal/agents/backtest.go`
- `internal/agents/backtest_result.go`
- `internal/adapters/market/historical_data.go`

**Timeline:** 3-5 days  
**Priority:** P2 - ENHANCEMENT

---

### 9. Agent Tournament System

**Value:** Compare agent strategies, find best performers.

**Implementation:**

Models already exist (`models.AgentTournament`), just need to activate:

```go
// internal/agents/tournament.go
func (am *AgenticManager) CreateTournament(
    ctx context.Context,
    userID string,
    agentIDs []string,
    symbols []string,
    duration time.Duration,
) (*models.AgentTournament, error) {
    // Start all agents on same balance
    // Run for duration
    // Compare final PnL, win rates, Sharpe ratios
}
```

**Timeline:** 2-3 days  
**Priority:** P3 - ENHANCEMENT

---

### 10. Cross-Personality Learning

**Current:** Collective memory only within same personality.

**Enhancement:**

```go
// Conservative agent learns from News Trader's successful news-driven trade
func (smm *SemanticMemoryManager) GetCrossPersonalityLessons(
    ctx context.Context,
    personality string,
    signalType string,
) ([]models.CollectiveMemory, error) {
    // Get successful memories from other personalities
    // Filter by signal type
    // Return top lessons
}
```

**Value:** Agents learn from other strategies.

**Timeline:** 2 days  
**Priority:** P3 - ENHANCEMENT

---

### 11. Real-time Monitoring Dashboard

**Implementation:**

```go
// internal/agents/stream.go
type AgentMonitor struct {
    subscribers map[string]chan *AgentEvent
}

type AgentEvent struct {
    Type      string  // "thinking", "decision", "execution", "reflection"
    AgentID   string
    Timestamp time.Time
    Data      interface{}
}

func (am *AgenticManager) StreamAgentEvents(agentID string) <-chan *AgentEvent {
    // Return channel with real-time events
}
```

**WebSocket endpoint:**

```go
// cmd/bot/main.go
http.HandleFunc("/ws/agents/:id/stream", handleAgentStream)
```

**Timeline:** 3-4 days  
**Priority:** P3 - ENHANCEMENT

---

### 12. Decision Explanation API

**Value:** Users can understand why agent made specific decision.

```go
func (am *AgenticManager) ExplainDecision(
    ctx context.Context,
    decisionID string,
) (*DecisionExplanation, error) {
    decision := am.repository.GetDecision(ctx, decisionID)

    // Reconstruct reasoning trace
    trace := parseReasoningFromDecision(decision)

    return &DecisionExplanation{
        WhatAgentSaw: trace.Observation,
        MemoriesRecalled: trace.RecalledMemories,
        OptionsConsidered: trace.GeneratedOptions,
        EvaluationProcess: trace.Evaluations,
        FinalReasoning: trace.FinalReasoning,
        ValidatorVotes: getValidatorVotes(decisionID),
    }, nil
}
```

**Timeline:** 1-2 days  
**Priority:** P3 - ENHANCEMENT

---

## 🔧 Technical Debt

### 13. Add Comprehensive Tests

**Current coverage:** Unknown (need to check)

**Required tests:**

1. **Unit tests:**

   - `cot_engine_test.go` - Test thinking process with mock AI
   - `semantic_memory_test.go` - Test memory recall
   - `reflection_test.go` - Test learning
   - `validator_council_test.go` - Test consensus

2. **Integration tests:**

   - `agent_lifecycle_test.go` - Full agent lifecycle
   - `multi_agent_test.go` - Multiple agents interacting
   - `recovery_test.go` - Agent recovery after crash

3. **Benchmark tests:**
   - Memory recall performance
   - CoT thinking speed
   - Validator council parallelization

**Timeline:** 5-7 days  
**Priority:** P2 - TECHNICAL DEBT

---

### 14. Add Metrics and Observability

**File:** `internal/agents/metrics.go` (create)

```go
package agents

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    agentDecisions = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "agent_decisions_total",
            Help: "Total decisions made by agents",
        },
        []string{"agent_id", "action", "personality"},
    )

    cotThinkingDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "agent_cot_thinking_duration_seconds",
            Help: "Chain-of-thought thinking duration",
        },
        []string{"agent_id"},
    )

    validatorConsensusRate = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "agent_validator_consensus_rate",
            Help: "Validator approval rate",
        },
        []string{"agent_id"},
    )

    memoryRecallCount = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "agent_memory_recalls_total",
            Help: "Number of memory recalls",
        },
        []string{"agent_id", "memory_type"},
    )
)
```

**Grafana Dashboard:** Create JSON for agent monitoring.

**Timeline:** 2 days  
**Priority:** P2 - TECHNICAL DEBT

---

### 15. Add Structured Logging

**Current:** Logs are good but could be more structured.

**Enhancement:**

```go
// Add trace IDs for request tracking
logger.Info("agent decision cycle started",
    zap.String("agent_id", agentID),
    zap.String("trace_id", generateTraceID()),
    zap.String("session_id", sessionID),
    zap.Time("timestamp", time.Now()),
)

// Add performance metrics
logger.Info("decision made",
    zap.Duration("thinking_duration", thinkingTime),
    zap.Duration("validation_duration", validationTime),
    zap.Int("memories_recalled", len(memories)),
    zap.Int("options_generated", len(options)),
)
```

**Timeline:** 1 day  
**Priority:** P3 - TECHNICAL DEBT

---

## 📊 Performance Optimizations

### 16. Cache AI Provider Responses

Identical market situations should reuse cached responses:

```go
// internal/adapters/ai/cache.go
type AIResponseCache struct {
    redis *redisAdapter.Client
    ttl   time.Duration
}

func (c *AIResponseCache) GetOrGenerate(
    ctx context.Context,
    cacheKey string,
    generator func() (*models.AIDecision, error),
) (*models.AIDecision, error) {
    // Check cache first
    cached, err := c.redis.Get(ctx, cacheKey)
    if err == nil {
        return parseCachedDecision(cached), nil
    }

    // Generate and cache
    result, err := generator()
    if err != nil {
        return nil, err
    }

    c.redis.Set(ctx, cacheKey, serializeDecision(result), c.ttl)
    return result, nil
}
```

**Estimated savings:** 30-50% reduction in AI API calls.

**Timeline:** 1-2 days  
**Priority:** P3 - OPTIMIZATION

---

### 17. Parallel Memory Recall

**Current:** Queries personal and collective memories sequentially.

**Optimization:**

```go
func (smm *SemanticMemoryManager) RecallRelevant(...) ([]models.SemanticMemory, error) {
    var wg sync.WaitGroup
    var personalMemories []models.SemanticMemory
    var collectiveMemories []models.CollectiveMemory
    var personalErr, collectiveErr error

    // Query in parallel
    wg.Add(2)

    go func() {
        defer wg.Done()
        personalMemories, personalErr = smm.repository.GetSemanticMemories(ctx, agentID, 100)
    }()

    go func() {
        defer wg.Done()
        collectiveMemories, collectiveErr = smm.repository.GetCollectiveMemories(ctx, personality, 50)
    }()

    wg.Wait()

    // ... existing merge logic
}
```

**Expected speedup:** 2x faster memory recall.

**Timeline:** 0.5 days  
**Priority:** P3 - OPTIMIZATION

---

## ✅ MASTER CHECKLIST

### Phase 1: Critical Fixes (Before Production)

- [ ] **P0-1: Implement Real Semantic Embeddings**

  - [ ] Add OpenAI embeddings client
  - [ ] Update `generateSimpleEmbedding` → `generateEmbedding`
  - [ ] Test memory recall with real embeddings
  - [ ] Verify collective memory sharing works
  - [ ] Update migration to use correct embedding dimensions
  - **Estimated:** 2-3 days
  - **Blocker:** YES

- [ ] **P0-2: Add Circuit Breaker System**

  - [ ] Implement `checkCircuitBreaker()` method
  - [ ] Add max drawdown check (-20%)
  - [ ] Add consecutive loss check (5+ losses)
  - [ ] Add daily loss limit check
  - [ ] Add repository methods for trade history
  - [ ] Test circuit breaker triggers
  - [ ] Add Telegram alerts for circuit breaker
  - **Estimated:** 1-2 days
  - **Blocker:** YES

- [ ] **P1-3: Add AI Provider Fallback**

  - [ ] Implement `fallbackDecision()` method
  - [ ] Test with AI provider disabled
  - [ ] Verify signal-based decisions work
  - [ ] Add metrics for fallback usage
  - **Estimated:** 1 day
  - **Blocker:** NO

- [ ] **P1-4: Improve Validator Fallback Prompts**
  - [ ] Write comprehensive role prompts (200-300 words each)
  - [ ] Embed as constants
  - [ ] Test with templates disabled
  - [ ] Compare validation quality
  - **Estimated:** 0.5 days
  - **Blocker:** NO

**Phase 1 Total:** 4.5-6.5 days

---

### Phase 2: Risk Management & Stability

- [ ] **P2-5: Add Learning Rate Safeguards**

  - [ ] Require minimum 20 trades before adaptation
  - [ ] Cap max weight change to ±0.10 per adaptation
  - [ ] Add logging for capped adjustments
  - [ ] Test with aggressive/scalper agents
  - **Estimated:** 0.5 days

- [ ] **P2-6: Add Memory Growth Control**

  - [ ] Implement memory count check
  - [ ] Auto-cleanup at 1000 memories
  - [ ] Test with high-frequency agents
  - **Estimated:** 0.5 days

- [ ] **P2-7: Improve Plan Scenario Matching**
  - [ ] Implement AI-based scenario matching
  - [ ] Add fallback to probability-based
  - [ ] Test with various market conditions
  - **Estimated:** 1 day

**Phase 2 Total:** 2 days

---

### Phase 3: Testing & Observability

- [ ] **P2-8: Add Comprehensive Test Suite**

  - [ ] Unit tests for CoT engine
  - [ ] Unit tests for semantic memory
  - [ ] Unit tests for reflection
  - [ ] Unit tests for validator council
  - [ ] Integration test for full agent lifecycle
  - [ ] Integration test for multi-agent scenarios
  - [ ] Benchmark tests for performance
  - [ ] Achieve >80% code coverage
  - **Estimated:** 5-7 days

- [ ] **P2-9: Add Metrics & Monitoring**

  - [ ] Create `metrics.go` with Prometheus metrics
  - [ ] Add metrics to all key operations
  - [ ] Create Grafana dashboard JSON
  - [ ] Set up alerting rules
  - [ ] Document metrics in README
  - **Estimated:** 2 days

- [ ] **P3-10: Improve Structured Logging**
  - [ ] Add trace IDs to all operations
  - [ ] Add performance timing logs
  - [ ] Add context to error logs
  - [ ] Test log aggregation (e.g., Loki)
  - **Estimated:** 1 day

**Phase 3 Total:** 8-10 days

---

### Phase 4: Enhancements (Post-MVP)

- [ ] **P2-11: Build Backtesting Framework**

  - [ ] Create `backtest.go` module
  - [ ] Implement historical data replay
  - [ ] Add simulation execution
  - [ ] Generate backtest reports
  - [ ] Test with multiple agent personalities
  - **Estimated:** 3-5 days

- [ ] **P3-12: Activate Tournament System**

  - [ ] Implement tournament creation
  - [ ] Add concurrent agent execution
  - [ ] Calculate winner rankings
  - [ ] Generate tournament reports
  - [ ] Add Telegram notifications
  - **Estimated:** 2-3 days

- [ ] **P3-13: Add Cross-Personality Learning**

  - [ ] Implement cross-personality memory query
  - [ ] Filter by signal type
  - [ ] Weight by success rate
  - [ ] Test knowledge transfer
  - **Estimated:** 2 days

- [ ] **P3-14: Build Real-time Monitoring Dashboard**

  - [ ] Create WebSocket stream endpoint
  - [ ] Implement event broadcaster
  - [ ] Build frontend dashboard (optional)
  - [ ] Add authentication
  - **Estimated:** 3-4 days

- [ ] **P3-15: Add Decision Explanation API**
  - [ ] Implement explanation reconstruction
  - [ ] Create API endpoint
  - [ ] Add to Telegram bot
  - [ ] Document API
  - **Estimated:** 1-2 days

**Phase 4 Total:** 11-16 days

---

### Phase 5: Performance & Polish

- [ ] **P3-16: Add AI Response Caching**

  - [ ] Implement cache layer
  - [ ] Generate cache keys
  - [ ] Set TTL policies
  - [ ] Measure cache hit rate
  - **Estimated:** 1-2 days

- [ ] **P3-17: Optimize Memory Recall**

  - [ ] Parallelize personal/collective queries
  - [ ] Add database indexes
  - [ ] Benchmark improvements
  - **Estimated:** 0.5 days

- [ ] **P3-18: Security Audit**

  - [ ] Review authentication/authorization
  - [ ] Check for SQL injection risks
  - [ ] Validate input sanitization
  - [ ] Test rate limiting
  - **Estimated:** 2 days

- [ ] **P3-19: Documentation Update**
  - [ ] Update README with new features
  - [ ] Document circuit breaker thresholds
  - [ ] Add troubleshooting guide
  - [ ] Create deployment guide
  - **Estimated:** 1 day

**Phase 5 Total:** 4.5-5.5 days

---

## 📈 TIMELINE SUMMARY

| Phase                            | Duration       | Criticality      |
| -------------------------------- | -------------- | ---------------- |
| Phase 1: Critical Fixes          | 4.5-6.5 days   | **MUST DO**      |
| Phase 2: Risk Management         | 2 days         | **MUST DO**      |
| Phase 3: Testing & Observability | 8-10 days      | **SHOULD DO**    |
| Phase 4: Enhancements            | 11-16 days     | **NICE TO HAVE** |
| Phase 5: Performance & Polish    | 4.5-5.5 days   | **NICE TO HAVE** |
| **TOTAL**                        | **30-40 days** |                  |

**Minimum Viable Product (MVP):**

- Phase 1 + Phase 2 = **6.5-8.5 days**
- Gets system to production-ready state

**Full Production Release:**

- Phase 1-3 = **15-18.5 days**
- Includes testing and monitoring

**Complete Feature Set:**

- All phases = **30-40 days**
- Fully polished with all enhancements

---

## 🎯 QUICK START (Week 1 Plan)

### Monday-Tuesday: Embeddings

```bash
# Day 1-2: Fix semantic embeddings
- Add OpenAI client to config
- Implement real embedding generation
- Test memory recall functionality
- Verify improvement in agent decisions
```

### Wednesday: Circuit Breaker

```bash
# Day 3: Add safety mechanisms
- Implement circuit breaker logic
- Add repository methods
- Test trigger conditions
- Add Telegram alerts
```

### Thursday: Fallback & Validation

```bash
# Day 4: Resilience improvements
- Add AI provider fallback
- Improve validator prompts
- Test degraded mode
```

### Friday: Testing & Deploy

```bash
# Day 5: Validation
- Write key unit tests
- Run integration tests
- Deploy to staging
- Monitor for issues
```

---

## 🚨 PRE-PRODUCTION CHECKLIST

Before deploying to production, verify:

### Functional Requirements

- [ ] Agents can start and stop cleanly
- [ ] Agents recover after pod restart
- [ ] Decisions are saved to database
- [ ] Memory recall returns relevant results
- [ ] Validator council reaches consensus
- [ ] Circuit breaker triggers on losses
- [ ] Telegram notifications work

### Performance Requirements

- [ ] Agent decision cycle < 30 seconds
- [ ] Memory recall < 2 seconds
- [ ] Validator council < 15 seconds (parallel)
- [ ] No memory leaks (test 24h run)
- [ ] Redis lock released on crash

### Security Requirements

- [ ] User isolation (agents can't access other users)
- [ ] API keys encrypted in database
- [ ] Input validation on all endpoints
- [ ] Rate limiting on expensive operations

### Monitoring Requirements

- [ ] Prometheus metrics exported
- [ ] Grafana dashboard configured
- [ ] Alerts set up for critical issues
- [ ] Logs aggregated and searchable

### Documentation Requirements

- [ ] README updated with setup instructions
- [ ] API documented (if exposed)
- [ ] Configuration options documented
- [ ] Troubleshooting guide created

---

## 📞 SUPPORT & QUESTIONS

If you need help implementing any of these items:

1. Check existing tests for patterns
2. Review similar implementations in codebase
3. Consult AI provider documentation
4. Ask team for architecture review

**Priority order for questions:**

1. Embedding implementation (most critical)
2. Circuit breaker thresholds (risk management)
3. AI fallback behavior (reliability)
4. Testing strategies (quality assurance)

---

**Last Updated:** November 23, 2025  
**Next Review:** After Phase 1 completion  
**Owner:** Development Team
