<!-- @format -->

# Autonomous AI Trading Agents

## Overview

This system implements **truly autonomous AI trading agents** that think, learn, and adapt like human traders. Unlike simple bots that execute predefined rules, these agents use:

- **Chain-of-Thought reasoning** (multi-step thinking)
- **Episodic memory** (remembering past experiences)
- **Self-reflection** (learning from mistakes)
- **Forward planning** (anticipating scenarios)
- **Self-adaptation** (modifying their own strategy)

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    AGENTIC AI AGENT                      │
├─────────────────────────────────────────────────────────┤
│                                                          │
│  ┌──────────────────────────────────────────┐          │
│  │  Chain-of-Thought Engine                 │          │
│  │  ────────────────────────                 │          │
│  │  1. Observe market situation              │          │
│  │  2. Recall similar memories               │          │
│  │  3. Generate 3-5 trading options          │          │
│  │  4. Evaluate each option (pros/cons)      │          │
│  │  5. Choose best option                    │          │
│  └──────────────────────────────────────────┘          │
│                      ↓                                   │
│  ┌──────────────────────────────────────────┐          │
│  │  Execution + Monitoring                   │          │
│  └──────────────────────────────────────────┘          │
│                      ↓                                   │
│  ┌──────────────────────────────────────────┐          │
│  │  Reflection Engine                        │          │
│  │  ────────────────────                     │          │
│  │  - What worked? What didn't?              │          │
│  │  - Extract key lessons                    │          │
│  │  - Suggest strategy adjustments           │          │
│  │  - Store as semantic memory               │          │
│  └──────────────────────────────────────────┘          │
│                      ↓                                   │
│  ┌──────────────────────────────────────────┐          │
│  │  Self-Adaptation                          │          │
│  │  ────────────────────                     │          │
│  │  Agent modifies its own weights           │          │
│  │  based on performance data                │          │
│  └──────────────────────────────────────────┘          │
│                                                          │
└─────────────────────────────────────────────────────────┘
```

## Components

### 1. Chain-of-Thought Engine (`cot_engine.go`)

Implements multi-step reasoning process:

```go
func (cot *ChainOfThoughtEngine) Think(marketData, position) {
    // Step 1: Observe current situation
    observation := "BTC at $43,250, up 2.3%, high volume..."

    // Step 2: Recall similar past experiences
    memories := cot.RecallRelevant(observation, topK=5)
    // → "When BTC rallied on ETF news, continuation was likely..."

    // Step 3: Generate multiple options
    options := AI.GenerateOptions(observation, memories)
    // → ["Go long 3x", "Wait for confirmation", "Short on weakness"]

    // Step 4: Evaluate each option
    evaluations := []
    for option in options:
        eval := AI.EvaluateOption(option, memories)
        // → Pros: [...], Cons: [...], Score: 75/100
        evaluations.append(eval)

    // Step 5: Make final decision
    decision := AI.ChooseBest(evaluations)
    // → "Going long because..."

    return decision
}
```

**Output**: Complete reasoning trace saved to `agent_reasoning_sessions` table.

### 2. Semantic Memory (`semantic_memory.go`)

Agents build episodic memory like humans:

```go
type SemanticMemory struct {
    Context    string    // "BTC dumped 5% on ETF rejection"
    Action     string    // "Went short at $42k"
    Outcome    string    // "Profit +3.2%, good call"
    Lesson     string    // "ETF news creates short opportunities"
    Embedding  []float32 // 128-dim vector for similarity search
    Importance float64   // 0.0 - 1.0
}
```

**Memory Retrieval**: Uses cosine similarity to find relevant past experiences.

**Example**:

```
Current: "BTC drops on regulatory news"
Recalls: "ETF rejection short was profitable" (similarity: 0.87)
Agent: "Based on past, this is a short opportunity"
```

### 3. Reflection Engine (`reflection.go`)

After each trade, agent reflects:

```
Trade: Long BTC at $43k → $44.5k
Result: +3.5% profit
Duration: 6 hours

Agent reflects:
✅ What worked:
   - Technical signals were accurate (RSI + MACD aligned)
   - Entry timing was good (bought the dip)

❌ What didn't work:
   - Exited too early (could have held for more)
   - Ignored on-chain outflow signal

💡 Key Lessons:
   - "When technical + on-chain align, hold longer"
   - "Don't exit on first resistance, wait for confirmation"

🔧 Suggested Adjustments:
   - Increase on-chain weight: +5%
   - Increase take-profit target: 5% → 6%
```

**Stored**: Reflection saved to `agent_reflections` table.  
**Applied**: Agent automatically adjusts its weights based on reflection.

### 4. Planning Engine (`planner.go`)

Agents create forward-looking plans:

```json
{
  "time_horizon": "24h",
  "assumptions": [
    "BTC will stay above $40k support",
    "No major regulatory news expected"
  ],
  "scenarios": [
    {
      "name": "Bullish breakout above $45k",
      "probability": 0.3,
      "indicators": ["Price breaks $45k with volume", "RSI > 65"],
      "action": "Go long 3x, target $48k, stop $44k",
      "reasoning": "Breakout continuation likely"
    },
    {
      "name": "Consolidation $42k-$45k",
      "probability": 0.5,
      "indicators": ["RSI 40-60", "Low volume"],
      "action": "Wait for breakout, preserve capital",
      "reasoning": "Range-bound = low R:R"
    },
    {
      "name": "Bearish breakdown < $42k",
      "probability": 0.2,
      "indicators": ["Breaks $42k", "RSI < 40"],
      "action": "Short 2x, target $40k",
      "reasoning": "Support broken = cascade"
    }
  ],
  "trigger_signals": [
    { "condition": "Volume > 3x average", "action": "Reassess plan" },
    { "condition": "News impact > 9", "action": "Revise scenarios" }
  ]
}
```

Plan is continuously checked and revised when triggers fire.

## Agent Personalities

### 8 Distinct Personalities

Each personality has unique:

- Signal weights (what they trust)
- Risk tolerance
- Decision frequency
- Learning rate
- System prompt (their "character")

#### 🛡️ **Technical Tom** (Conservative)

```
Weights: Technical 70%, News 10%, On-Chain 15%, Sentiment 5%
Risk: 20% position, 2x leverage max
Interval: 1 hour
Character: "I trust math over hype. Only trade 80%+ confidence."
```

#### ⚔️ **Aggressive Alpha** (High Risk/Reward)

```
Weights: Balanced 30/30/25/15
Risk: 50% position, 5x leverage max
Interval: 10 minutes
Character: "Big risks = big rewards. 60% confidence is enough."
```

#### ⚖️ **Balanced Bob** (Well-Rounded)

```
Weights: Technical 40%, News 25%, On-Chain 20%, Sentiment 15%
Risk: 30% position, 3x leverage
Interval: 30 minutes
Character: "No signal is superior. Use all information equally."
```

#### ⚡ **Scalper Sam** (High Frequency)

```
Weights: Technical 60%, Sentiment 25%, News 5%, On-Chain 10%
Risk: 25% position, 4x leverage
Interval: 5 minutes
Character: "Small profits compound. Speed is everything."
```

#### 🌊 **Swing Steve** (Medium-Term)

```
Weights: Technical 45%, News 25%, On-Chain 20%, Sentiment 10%
Risk: 35% position, 3x leverage
Interval: 2 hours
Character: "Trends are friends. Ride them for days."
```

#### 📰 **News Ninja** (News-Driven)

```
Weights: News 60%, Technical 20%, On-Chain 10%, Sentiment 10%
Risk: 30% position, 3x leverage
Interval: 15 minutes
Character: "News moves markets. React FAST to high-impact events."
```

#### 🐋 **Whale Watcher** (On-Chain Specialist)

```
Weights: On-Chain 60%, Technical 15%, News 15%, Sentiment 10%
Risk: 35% position, 4x leverage
Interval: 20 minutes
Character: "Follow smart money. Whales know something we don't."
```

#### 🔄 **Contrarian Carl** (Anti-Crowd)

```
Weights: Technical 40%, Sentiment 30% (INVERTED), On-Chain 20%, News 10%
Risk: 25% position, 3x leverage
Interval: 30 minutes
Character: "When crowd is bullish, be cautious. Fade the hype."
```

## How It Works

### Complete Agent Lifecycle

```
1. INITIALIZATION
   ├─ Create agent with personality
   ├─ Initialize semantic memory (empty)
   ├─ Assign AI provider (Claude, DeepSeek, or GPT)
   └─ Create 24h trading plan

2. DECISION CYCLE (every N minutes based on personality)
   ├─ Check if plan needs revision
   ├─ Observe current market (price, volume, news, on-chain)
   ├─ Recall relevant memories
   │   "This reminds me of..." (semantic search)
   ├─ Generate 3-5 trading options
   │   "I could go long, wait, or short..."
   ├─ Evaluate each option critically
   │   "Long: Pros[...] Cons[...] Risks[...] Score:75/100"
   ├─ Choose best option
   │   "Going long because highest score + acceptable risk"
   └─ Execute if confidence > threshold

3. POST-TRADE REFLECTION
   ├─ Trade closes (profit or loss)
   ├─ Agent reflects: "What happened? Why?"
   ├─ Extract key lesson
   ├─ Store as semantic memory
   └─ Suggest strategy adjustments

4. SELF-ADAPTATION (every 50 trades or 7 days)
   ├─ Analyze performance by signal type
   ├─ Identify strengths/weaknesses
   ├─ AI suggests new weights
   │   "Technical: 70% → 65% (doing well)"
   │   "News: 10% → 15% (missing opportunities)"
   └─ Apply changes automatically

5. MEMORY CONSOLIDATION (weekly)
   └─ Forget low-importance memories (< 0.3)
```

## Database Schema

```sql
-- Agent Configuration
agent_configs (
    id, user_id, name, personality,
    specialization JSONB,  -- Signal weights
    strategy JSONB,        -- Risk params
    learning_rate,
    is_active
)

-- Trading State
agent_states (
    agent_id, symbol,
    balance, equity, pnl,
    total_trades, win_rate
)

-- Decision History (with reasoning)
agent_decisions (
    agent_id, symbol, action, confidence,
    technical_score, news_score, onchain_score, sentiment_score,
    final_score, reason, executed, outcome
)

-- Episodic Memory
agent_semantic_memories (
    agent_id,
    context TEXT,      -- "BTC dumped on SEC news"
    action TEXT,       -- "Went short"
    outcome TEXT,      -- "Profit +2%"
    lesson TEXT,       -- "SEC FUD = short opportunity"
    embedding FLOAT[], -- 128-dim vector
    importance FLOAT,
    access_count
)

-- Reasoning Traces
agent_reasoning_sessions (
    session_id, agent_id,
    observation, recalled_memories,
    generated_options, evaluations,
    final_reasoning, decision,
    chain_of_thought JSONB
)

-- Reflections
agent_reflections (
    agent_id,
    analysis, what_worked, what_didnt_work,
    key_lessons, suggested_adjustments
)

-- Trading Plans
agent_trading_plans (
    plan_id, agent_id,
    time_horizon, assumptions,
    scenarios JSONB,  -- Multiple what-if scenarios
    risk_limits, trigger_signals
)

-- Statistical Memory (old system)
agent_memory (
    agent_id,
    technical_success_rate,
    news_success_rate,
    onchain_success_rate,
    sentiment_success_rate
)
```

## Usage Examples

### Creating an Agent

```go
// Create conservative agent
agentManager := agents.NewAgenticManager(db, newsAggregator, aiProviders)

config, err := agentManager.CreateAgentFromPersonality(
    ctx,
    userID,
    models.PersonalityConservative,
    "My Technical Tom",
)

// Start trading
err = agentManager.StartAgenticAgent(
    ctx,
    config.ID,
    "BTC/USDT",
    1000.0,        // $1000 initial balance
    exchangeAdapter,
)
```

### Agent Decision Process (Logs)

```
🧠 Agent starting Chain-of-Thought reasoning
   agent="Technical Tom" personality="conservative"

📊 Observation: BTC at $43,250 (+2.3%), high volume, no position

📚 Recalled 3 relevant memories:
   1. "BTC rally on volume continuation likely" (similarity: 0.89)
   2. "Wait for RSI confirmation before entry" (similarity: 0.76)
   3. "High funding rate = crowded longs = caution" (similarity: 0.68)

💡 Generated 4 options:
   opt1: Aggressive long 4x leverage
   opt2: Conservative long 2x leverage
   opt3: Wait for better entry
   opt4: Short on overbought

⚖️ Evaluations:
   opt1: Score 62/100 - "Too risky given funding rate"
   opt2: Score 81/100 - "Good R:R, aligns with personality"
   opt3: Score 58/100 - "Missing opportunity"
   opt4: Score 35/100 - "Against momentum"

✅ Final Decision: OPEN_LONG
   Reasoning: "Option 2 scores highest (81/100). Conservative 2x leverage
   aligns with my personality. Technical + volume confirm, funding rate
   acceptable. Past memories support this setup."
   Confidence: 81%

🤔 Post-trade reflection (6h later):
   Result: +2.8% profit
   ✅ What worked: Technical analysis was accurate
   ✅ Entry timing was good
   ❌ Could have used 3x leverage for more profit
   💡 Lesson: "When all signals align, can be slightly more aggressive"
   🔧 Adjustment: max_leverage: 2 → 2.5

💾 Memory stored:
   Context: "BTC uptrend with volume confirmation"
   Lesson: "Conservative entries on volume spikes work well"
   Importance: 0.75
```

### Agent Self-Analysis (Every 7 Days)

```
🎯 Self-analysis after 50 trades:

Performance:
- Win Rate: 58% (29 wins, 21 losses)
- Total PnL: +$147 (+14.7%)
- Sharpe Ratio: 1.8

Signal Performance:
- Technical: 65% win rate ✅ (currently 70% weight)
- News: 42% win rate ❌ (currently 10% weight)
- On-Chain: 61% win rate ✅ (currently 15% weight)
- Sentiment: 48% win rate ➡️ (currently 5% weight)

AI Analysis:
"Technical analysis is my strength - keep high weight. News signals
failing possibly because I'm too conservative and miss fast moves.
On-chain working well - could increase weight slightly."

Suggested Changes:
- Technical: 70% → 68% (minor decrease)
- News: 10% → 8% (decrease, not working)
- On-Chain: 15% → 19% (increase, working well)
- Sentiment: 5% → 5% (keep)

✅ Changes applied automatically
```

## AI Provider Integration

All 3 providers support agentic methods:

```go
type AgenticProvider interface {
    // Core trading
    Analyze(prompt) -> AIDecision

    // Agentic capabilities
    Reflect(trade) -> Reflection
    GenerateOptions(situation) -> []TradingOption
    EvaluateOption(option, memories) -> OptionEvaluation
    MakeFinalDecision(evaluations) -> AIDecision
    CreatePlan(request) -> TradingPlan
    SelfAnalyze(performance) -> SelfAnalysis
    SummarizeMemory(experience) -> MemorySummary
}
```

**Implemented for:**

- ✅ Claude (best for reflection & analysis)
- ✅ DeepSeek (fast & cost-effective)
- ✅ GPT (good at context understanding)

## Agent Managers

### Simple Manager (`manager.go`)

Basic agents with statistical learning:

- One-shot AI decisions
- Automatic weight adjustment every 50 trades
- Statistical memory (win rates by signal type)

### Agentic Manager (`agentic_manager.go`)

Autonomous agents with full thinking:

- Chain-of-Thought reasoning
- Semantic memory recall
- Post-trade reflection
- Forward planning
- Self-modification

## Tournament System

Pit agents against each other:

```go
tournament := agents.NewTournament(
    db, repo, agentManager,
    userID, "BTC Battle",
    []string{"BTC/USDT"},
    1000.0,        // $1000 start balance each
    24 * time.Hour, // 24h duration
)

tournament.AddParticipant(ctx, technicalTomID, exchange)
tournament.AddParticipant(ctx, aggressiveAlphaID, exchange)
tournament.AddParticipant(ctx, whaleWatcherID, exchange)

tournament.Start(ctx)

// After 24h:
leaderboard := tournament.GetLeaderboard()
// → 1. Whale Watcher: +15.2%
//   2. Technical Tom: +8.7%
//   3. Aggressive Alpha: -3.1%
```

## Cost Estimation

**Per agent per day**:

- Chain-of-Thought: ~5-10 API calls
- Reflection: ~1-2 calls per trade
- Planning: ~1-2 calls per 24h
- Self-Analysis: ~1 call per week

**Example** (30min interval, 2 trades/day):

```
DeepSeek: ~48 decisions + 4 reflections = ~$0.15/day
Claude: ~48 decisions + 4 reflections = ~$0.80/day
GPT: ~48 decisions + 4 reflections = ~$1.20/day
```

**Recommendation**: Use DeepSeek for scalpers, Claude for swing traders.

## Performance Characteristics

**Latency**:

- Simple Decision: 2-3 seconds (1 API call)
- Chain-of-Thought: 15-25 seconds (5-7 API calls)
- Reflection: 3-5 seconds (1 API call)
- Planning: 5-8 seconds (1 API call)

**Memory**:

- Agent process: ~15MB RAM
- Semantic memories: ~100KB per 100 memories
- Reasoning traces: ~500KB per 100 decisions

**Database**:

- ~1MB per agent per month (decisions + memories + reflections)

## Advantages Over Simple Bots

| Feature         | Simple Bot       | Agentic AI Agent     |
| --------------- | ---------------- | -------------------- |
| Decision Making | One-shot AI call | Multi-step reasoning |
| Memory          | Statistics only  | Semantic memories    |
| Learning        | Formula-based    | Self-reflective      |
| Adaptation      | Every 50 trades  | After each trade     |
| Planning        | None             | 24h forward planning |
| Explainability  | "AI said so"     | Full reasoning trace |

## Limitations

1. **Cost**: 5-10x more API calls than simple agents
2. **Latency**: 15-25s vs 2-3s for decisions
3. **Complexity**: More moving parts = more can break
4. **Embeddings**: Simple bag-of-words (production needs proper embeddings)
5. **JSON Parsing**: AI responses must be valid JSON

## Best Practices

### When to Use Agentic Agents

✅ **Good for:**

- Medium-term trading (swing, position)
- Learning from experience over time
- Complex market conditions
- When explainability matters
- Experimenting with strategies

❌ **Not ideal for:**

- Ultra-high frequency (< 5min intervals)
- Very simple strategies
- When cost is critical
- When latency must be minimal

### Agent Selection Guide

**Market Conditions** → **Best Agent**:

- Trending market → Swing Steve
- Range-bound → Technical Tom
- News-driven → News Ninja
- Whale movements → Whale Watcher
- Extremely volatile → Balanced Bob
- Contrarian plays → Contrarian Carl

### Optimization Tips

1. **Start with simple agents**, graduate to agentic when you understand them
2. **Use tournaments** to find best personality for your markets
3. **Monitor memory growth** - consolidate old memories monthly
4. **Review reflections** - agents learn, but verify their lessons
5. **Set proper thresholds** - min_confidence should match personality

## Future Enhancements

- [ ] Use OpenAI embeddings API for better memory retrieval
- [ ] Multi-agent collaboration (agents consulting each other)
- [ ] Meta-learning (agent of agents that manages other agents)
- [ ] Risk-adjusted Sharpe optimization
- [ ] Automatic personality evolution
- [ ] Voice/personality consistency across all prompts
- [ ] Long-term memory consolidation strategies

## Files Overview

```
internal/agents/
├── agentic_manager.go    14KB   Orchestrates autonomous agents
├── cot_engine.go         9.2KB  Chain-of-Thought reasoning
├── decision_engine.go    12KB   AI-powered decisions
├── manager.go            14KB   Simple agent manager
├── memory.go             6.3KB  Statistical learning
├── personalities.go      17KB   8 personalities + system prompts
├── planner.go            6.6KB  Forward planning
├── reflection.go         8.8KB  Post-trade reflection
├── repository.go         23KB   All database operations
├── semantic_memory.go    5.4KB  Episodic memory system
└── tournament.go         9.2KB  Agent competitions

internal/adapters/ai/
├── agentic_interface.go  3.7KB  AgenticProvider interface
├── agentic_prompts.go    27KB   High-quality shared prompts
├── claude.go             +8KB   Claude agentic methods
├── deepseek.go           +6KB   DeepSeek agentic methods
└── openai.go             +6KB   OpenAI agentic methods

pkg/models/
├── agent.go              8.9KB  Agent, State, Decision models
└── agentic.go            11KB   Reflection, Memory, Plan models

migrations/
├── 000005_agents.up.sql         Agent tables
└── 000006_semantic_memory.up.sql  Memory tables
```

**Total**: ~160KB of code implementing autonomous AI agents.

---

**This is experimental advanced AI**. Start with simple agents, observe their behavior, then enable agentic features once comfortable.
