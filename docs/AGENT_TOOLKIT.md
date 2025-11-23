<!-- @format -->

# Agent Toolkit System

## Overview

The **Agent Toolkit** provides autonomous AI agents with safe, read-only access to cached market data, news, on-chain information, and their own memories. All tools query **local database caches** populated by background workers, ensuring:

- ✅ **No rate limits** - never calls exchange APIs directly
- ✅ **Low latency** - all data from Postgres/ClickHouse
- ✅ **Complete traceability** - every tool call is logged
- ✅ **Safety** - read-only, can't create orders or modify state

## Architecture

```
┌──────────────────────────────────────────────────┐
│         BACKGROUND WORKERS LAYER                 │
│  (Populate caches every N minutes)               │
├──────────────────────────────────────────────────┤
│  CandlesWorker (5min)     → ohlcv_candles        │
│  NewsWorker (10min)       → news_items           │
│  OnChainWorker (15min)    → whale_transactions   │
│  SentimentAggregator      → sentiment_cache      │
└────────────────┬─────────────────────────────────┘
                 │
                 ↓ (populate)
┌────────────────────────────────────────────────────┐
│        LOCAL DATABASE CACHE                        │
│   Postgres: news, whales, agent memories           │
│   ClickHouse: OHLCV candles (future)               │
└────────────────┬───────────────────────────────────┘
                 │
                 ↓ (read only)
┌────────────────────────────────────────────────────┐
│         AGENT TOOLKIT                              │
│  LocalToolkit implements AgentToolkit interface    │
└────────────────┬───────────────────────────────────┘
                 │
                 ↓ (available during thinking)
┌────────────────────────────────────────────────────┐
│      CHAIN-OF-THOUGHT ENGINE                       │
│   Agent can call tools while reasoning             │
└────────────────────────────────────────────────────┘
```

## Available Tools

### 📊 Market Data Tools

#### `GetCandles(symbol, timeframe, limit)`
Retrieves OHLCV candles from cache.

```go
// Agent wants to see 1-minute candles for scalping decision
candles1m, err := toolkit.GetCandles(ctx, "BTC/USDT", "1m", 50)

// Check 4-hour timeframe for trend
candles4h, err := toolkit.GetCandles(ctx, "BTC/USDT", "4h", 100)
```

**Available timeframes:** `1m`, `5m`, `15m`, `1h`, `4h`, `1d`

#### `GetCandleCount(symbol, timeframe)`
Returns total cached candles for symbol/timeframe.

```go
count, err := toolkit.GetCandleCount(ctx, "BTC/USDT", "5m")
// → 1000 (roughly 3.5 days of 5m candles)
```

#### `GetLatestPrice(symbol, timeframe)`
Gets most recent close price from candles.

```go
price, err := toolkit.GetLatestPrice(ctx, "BTC/USDT", "1m")
// → 43250.50
```

### 📰 News Tools

#### `SearchNews(query, since, limit)`
Full-text search in cached news articles.

```go
// Agent "News Ninja" searches for ETF-related news
news, err := toolkit.SearchNews(ctx, "ETF approval", 6*time.Hour, 10)

// Search for regulatory FUD
news, err := toolkit.SearchNews(ctx, "SEC lawsuit", 24*time.Hour, 20)
```

#### `GetHighImpactNews(minImpact, since)`
Filters news by AI-evaluated impact score (0-10).

```go
// Get breaking news (impact 9+)
breaking, err := toolkit.GetHighImpactNews(ctx, 9, 1*time.Hour)

// Get high-impact news (impact 7+)
highImpact, err := toolkit.GetHighImpactNews(ctx, 7, 6*time.Hour)
```

#### `GetNewsBySentiment(minSentiment, maxSentiment, since)`
Filters news by sentiment range.

```go
// Get very positive news (sentiment > 0.7)
bullish, err := toolkit.GetNewsBySentiment(ctx, 0.7, 1.0, 12*time.Hour)

// Get very negative news (sentiment < -0.7)
bearish, err := toolkit.GetNewsBySentiment(ctx, -1.0, -0.7, 12*time.Hour)

// Get neutral news
neutral, err := toolkit.GetNewsBySentiment(ctx, -0.2, 0.2, 24*time.Hour)
```

### 🐋 On-Chain Tools

#### `GetRecentWhaleMovements(symbol, minAmountUSD, hours)`
Gets whale transactions above threshold.

```go
// Agent "Whale Watcher" checks for major movements
whales, err := toolkit.GetRecentWhaleMovements(ctx, "BTC/USDT", 5_000_000, 24)
// → 3 transactions of $5M+ in last 24h

// Check for any whale activity
whales, err := toolkit.GetRecentWhaleMovements(ctx, "ETH/USDT", 1_000_000, 12)
```

#### `GetNetExchangeFlow(symbol, hours)`
Calculates net inflow/outflow over time period.

```go
// Check 24h net flow
netFlow, err := toolkit.GetNetExchangeFlow(ctx, "BTC/USDT", 24)
// → -15000000.0 ($15M outflow = bullish)
// → +8000000.0 ($8M inflow = bearish)
```

#### `GetLargestWhaleTransaction(symbol, hours)`
Finds biggest transaction in time window.

```go
largest, err := toolkit.GetLargestWhaleTransaction(ctx, "BTC/USDT", 24)
// → Transaction{AmountUSD: 12.5M, Type: "exchange_outflow", ...}
```

### 🧠 Memory Tools

#### `SearchPersonalMemories(query, topK)`
Queries agent's own semantic memory.

```go
// Agent recalls similar past situations
memories, err := toolkit.SearchPersonalMemories(ctx, "ETF news dump", 5)
// → 5 most similar memories with cosine similarity

// Search for successful trades
memories, err := toolkit.SearchPersonalMemories(ctx, "profitable long entry", 3)
```

#### `SearchCollectiveMemories(personality, query, topK)`
Queries collective wisdom of same personality agents.

```go
// Conservative agent learns from other conservative agents
memories, err := toolkit.SearchCollectiveMemories(ctx, "conservative", "bear market strategy", 5)

// News Ninja learns from other News Ninjas
memories, err := toolkit.SearchCollectiveMemories(ctx, "news_ninja", "breaking news reaction", 3)
```

#### `GetRecentMemories(limit)`
Gets agent's most recent memories.

```go
// Review last 10 experiences
recent, err := toolkit.GetRecentMemories(ctx, 10)
```

### 📈 Performance Tools

#### `GetWinRateBySignal(symbol)`
Calculates performance breakdown by signal type.

```go
stats, err := toolkit.GetWinRateBySignal(ctx, "BTC/USDT")
// → Technical: 65% win rate
// → News: 42% win rate  
// → OnChain: 61% win rate
// → Sentiment: 48% win rate
```

#### `GetCurrentStreak(symbol)`
Returns current winning/losing streak.

```go
count, isWinning, err := toolkit.GetCurrentStreak(ctx, "BTC/USDT")
// → 5, true (5-trade winning streak)
// → 3, false (3-trade losing streak)
```

## Usage Examples

### Example 1: News Ninja Checks Breaking News

```go
// In Chain-of-Thought reasoning
func (cot *ChainOfThoughtEngine) Think(ctx, marketData, position) {
    // Agent can use toolkit during thinking
    
    // "Let me check if there's any breaking news"
    breaking, _ := cot.toolkit.GetHighImpactNews(ctx, 9, 1*time.Hour)
    
    if len(breaking) > 0 {
        logger.Info("🚨 Breaking news detected",
            zap.Int("count", len(breaking)),
            zap.String("title", breaking[0].Title),
        )
        
        // Include in decision context
        situation.HighImpactNews = breaking
    }
    
    // Continue with option generation...
}
```

### Example 2: Whale Watcher Tracks Smart Money

```go
// Agent checks whale movements before decision
whales, _ := toolkit.GetRecentWhaleMovements(ctx, "BTC/USDT", 10_000_000, 24)

if len(whales) >= 3 {
    // Multiple large outflows = bullish (coins moving off exchanges)
    outflows := 0
    for _, tx := range whales {
        if tx.TransactionType == "exchange_outflow" {
            outflows++
        }
    }
    
    if outflows >= 2 {
        logger.Info("🐋 Multiple whale outflows detected - bullish signal")
    }
}
```

### Example 3: Agent Learns from Past Mistakes

```go
// Agent recalls similar situations
memories, _ := toolkit.SearchPersonalMemories(ctx, observation, 5)

for _, mem := range memories {
    if mem.Outcome == "loss" {
        // "I lost money in this situation before, be careful"
        logger.Warn("⚠️ Similar situation led to loss",
            zap.String("context", mem.Context),
            zap.String("lesson", mem.Lesson),
        )
    }
}
```

### Example 4: Checking Additional Timeframes

```go
// Agent checks if higher timeframe confirms decision
candles1h, _ := toolkit.GetCandles(ctx, "BTC/USDT", "1h", 20)
candles4h, _ := toolkit.GetCandles(ctx, "BTC/USDT", "4h", 10)

// Calculate trend alignment
trend1h := calculateTrend(candles1h)  // "bullish"
trend4h := calculateTrend(candles4h)  // "bullish"

if trend1h == trend4h {
    logger.Info("✅ Multi-timeframe confirmation")
    // Higher confidence in decision
}
```

## Tool Usage Tracing

Every tool call is automatically traced:

```json
{
  "session_id": "agent-abc123-1700000000",
  "agent_id": "abc123",
  "tool_calls": [
    {
      "tool_name": "get_high_impact_news",
      "parameters": {"min_impact": 9, "hours": 1},
      "result": [{"title": "SEC approves Bitcoin ETF", "impact": 10}],
      "success": true,
      "latency": "5ms"
    },
    {
      "tool_name": "get_whale_movements",
      "parameters": {"symbol": "BTC/USDT", "min_amount": 5000000, "hours": 24},
      "result": [{"amount_usd": 12500000, "type": "exchange_outflow"}],
      "success": true,
      "latency": "8ms"
    }
  ],
  "total_time": "13ms"
}
```

This trace is stored in `agent_reasoning_sessions` table for full explainability.

## Implementation Details

### LocalToolkit Structure

```go
type LocalToolkit struct {
    agentID       string
    marketRepo    *market.Repository       // OHLCV cache
    newsCache     *news.Cache              // News cache
    agentRepo     *agents.Repository       // Agent data
    memoryManager *SemanticMemoryManager   // Memory system
}
```

### Tool Safety Guarantees

1. **Read-Only** - Tools cannot modify any state
2. **Local Cache** - Never calls exchange APIs
3. **Rate Limit Free** - Query as much as needed
4. **Logged** - Every call recorded for audit
5. **Error Tolerant** - Failures don't crash agent

### Performance

- **Candles**: 1-5ms (Postgres index scan)
- **News search**: 5-15ms (full-text search)
- **Whale data**: 2-8ms (indexed query)
- **Memory search**: 10-30ms (vector similarity)

**Total overhead**: ~20-50ms per tool-heavy decision cycle

## Future Enhancements

- [ ] **Pattern Recognition Tools** - `FindSimilarCandles()`, `DetectChartPatterns()`
- [ ] **Cross-Agent Communication** - `ConsultOtherAgent()`, `RequestValidation()`
- [ ] **Advanced Analytics** - `CalculateCorrelation()`, `PredictVolatility()`
- [ ] **ClickHouse Integration** - Ultra-fast OLAP queries for historical analysis
- [ ] **Function Calling Integration** - Let Claude/GPT choose tools dynamically
- [ ] **Tool Composition** - Agents can chain tools automatically

## Comparison: With vs Without Toolkit

### Without Toolkit (Current)

```go
// Agent gets pre-packaged market data
func Think(marketData *MarketData) {
    // Fixed set of timeframes (5m, 15m, 1h, 4h)
    // Fixed news window (6h)
    // No access to whale data
    // Can't check additional info
}
```

### With Toolkit (New)

```go
// Agent can query what it needs
func Think(marketData *MarketData, toolkit AgentToolkit) {
    // "I need 1-minute candles for this scalp"
    candles1m := toolkit.GetCandles("BTC/USDT", "1m", 50)
    
    // "Any breaking news in last hour?"
    breaking := toolkit.GetHighImpactNews(9, 1*time.Hour)
    
    // "Have I seen this pattern before?"
    similar := toolkit.SearchPersonalMemories("sudden dump", 5)
    
    // Agent is more autonomous and adaptive
}
```

## Best Practices

### ✅ DO

- Query tools **during Chain-of-Thought** reasoning
- Use tools to **validate hypotheses**
- Check **multiple timeframes** for confirmation
- Learn from **past memories** before deciding
- Log tool usage for **explainability**

### ❌ DON'T

- Don't query same data repeatedly (cache in session)
- Don't make 100+ tool calls per decision (too slow)
- Don't use tools for data already in `MarketData`
- Don't ignore tool errors (handle gracefully)
- Don't trust tools blindly (validate results)

## Enabling Toolkit

Toolkit is automatically initialized for all agentic agents created via `AgenticManager`:

```go
// In main.go or cmd/bot/main.go
agentManager := agents.NewAgenticManager(
    db,
    redisClient,
    lockFactory,
    marketRepo,    // For candles tools
    newsAggregator,
    newsCache,     // For news tools
    aiProviders,
    notifier,
)

// Toolkit is automatically set up when agent starts
agentManager.StartAgenticAgent(ctx, agentID, symbol, balance, exchange)
```

No additional configuration needed - toolkit "just works"!

---

**The toolkit transforms agents from passive data consumers to active information seekers, making them more autonomous and intelligent.**

