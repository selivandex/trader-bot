<!-- @format -->

# News Data Flow - Complete Picture

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                     MAIN APPLICATION                         │
│                                                              │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐     │
│  │ News Worker  │  │ Bot Manager  │  │ Telegram Bot │     │
│  │ (Background) │  │  (Multiple   │  │  (Commands)  │     │
│  │              │  │  User Bots)  │  │              │     │
│  └──────────────┘  └──────────────┘  └──────────────┘     │
│         │                  │                  │             │
└─────────┼──────────────────┼──────────────────┼─────────────┘
          │                  │                  │
          ▼                  ▼                  ▼
    ┌──────────┐      ┌──────────┐      ┌──────────┐
    │ News DB  │      │ Users DB │      │ Telegram │
    │ (Cache)  │      │ (State)  │      │   API    │
    └──────────┘      └──────────┘      └──────────┘
```

## 1. Background News Worker

**Запускается при старте приложения:**

```go
// cmd/bot/main.go

newsWorker := workers.NewNewsWorker(
    newsAggregator,
    newsCache,
    10*time.Minute,  // Fetch every 10 minutes
    keywords,
)

// Start in goroutine
go func() {
    newsWorker.Start(ctx)
}()
```

**Цикл работы:**

```
Start
  ↓
Fetch immediately (первый раз)
  ↓
┌─────────────────────────┐
│ Every 10 minutes:       │
│                         │
│ 1. Reddit.FetchNews()   │ ←┐
│ 2. CoinDesk.FetchNews() │  │
│ 3. Twitter.FetchNews()  │  │
│         ↓               │  │
│ 4. Analyze Sentiment    │  │
│         ↓               │  │
│ 5. Cache.Save(news)     │  │
│         ↓               │  │
│ 6. Log statistics       │  │
└─────────────────────────┘  │
         ↓                    │
    Sleep 10 min ─────────────┘

Every 24 hours:
  Cache.CleanupOld() - removes news >7 days
```

## 2. Database Storage

**news_items table:**

```sql
CREATE TABLE news_items (
    id SERIAL,
    external_id VARCHAR UNIQUE,  -- "reddit_abc123", "coindesk_xyz789"
    source VARCHAR,              -- reddit, coindesk, twitter
    title TEXT,                  -- "Bitcoin breaks $50k"
    content TEXT,                -- Full article text
    url TEXT,                    -- Link to original
    author VARCHAR,              -- "u/cryptotrader" or author name
    published_at TIMESTAMP,      -- When news was published
    sentiment DECIMAL,           -- -1.0 to 1.0
    relevance DECIMAL,           -- 0.0 to 1.0 (engagement score)
    keywords TEXT[],             -- ['bitcoin', 'btc']
    created_at TIMESTAMP         -- When cached
);
```

**Example data:**

```sql
INSERT INTO news_items VALUES (
    1,
    'reddit_1a2b3c',
    'reddit',
    'Bitcoin ETF approval imminent according to insider',
    'Major sources suggest...',
    'https://reddit.com/r/CryptoCurrency/comments/1a2b3c',
    'u/cryptoinsider',
    '2024-11-22 10:30:00',
    0.75,  -- Very positive sentiment
    0.9,   -- High relevance (1000+ upvotes)
    ARRAY['bitcoin', 'btc', 'etf'],
    '2024-11-22 10:35:00'
);
```

## 3. Trading Engine Uses Cache

**Когда нужно принять решение (каждые 30 минут):**

```go
// internal/strategy/engine.go

func (e *Engine) collectMarketData(ctx context.Context) {
    // ... fetch ticker, candles, orderbook ...

    // Get news from cache (NO API calls!)
    newsSummary, err := e.newsAggregator.GetCachedSummary(ctx, 6*time.Hour)

    marketData := &models.MarketData{
        Symbol:      "BTC/USDT",
        Ticker:      ticker,
        Indicators:  indicators,
        NewsSummary: newsSummary,  // ← Новости из кэша
    }

    return marketData
}
```

## 4. News in AI Prompt

**Новости попадают в промпт так:**

```go
// internal/adapters/ai/prompts.go

func buildUserPrompt(prompt *TradingPrompt) string {
    // ... price, indicators, orderbook ...

    if prompt.MarketData.NewsSummary != nil {
        news := prompt.MarketData.NewsSummary

        sb.WriteString("=== NEWS & SENTIMENT ===\n\n")
        sb.WriteString(fmt.Sprintf("Overall Sentiment: *%s*\n", news.OverallSentiment))
        sb.WriteString(fmt.Sprintf("Average Score: %.2f\n", news.AverageSentiment))
        sb.WriteString(fmt.Sprintf("Total News: %d (📈 %d | 📉 %d | ➡️ %d)\n\n",
            news.TotalItems,
            news.PositiveCount,
            news.NegativeCount,
            news.NeutralCount))

        // Recent headlines (top 3)
        if len(news.RecentNews) > 0 {
            sb.WriteString("Recent Headlines:\n")
            for i, item := range news.RecentNews {
                if i >= 3 {
                    break
                }

                emoji := getSentimentEmoji(item.Sentiment)
                sb.WriteString(fmt.Sprintf("%d. %s [%s] %s (%.2f)\n",
                    i+1, emoji, item.Source, item.Title, item.Sentiment))
            }
        }
    }

    return sb.String()
}
```

## 5. Example AI Prompt with News

```
=== MARKET DATA ===

Symbol: BTC/USDT
Current Price: $43,250.00
24h Change: +2.30%
24h High: $43,500.00
24h Low: $42,100.00

=== TECHNICAL INDICATORS ===

RSI:
  14: 68.50 (Neutral)

MACD:
  MACD: 125.30
  Signal: 115.20
  Histogram: 10.10 (Bullish)

Bollinger Bands:
  Upper: $44,200.00
  Middle: $43,000.00
  Lower: $41,800.00

=== NEWS & SENTIMENT ===

Overall Sentiment: BULLISH
Average Score: 0.42
Total News: 15 (📈 9 | 📉 3 | ➡️ 3)

Recent Headlines:
1. 📈 [reddit] Major Bitcoin ETF approval signals from SEC (0.68)
2. 📈 [coindesk] Institutional investors increase BTC holdings (0.55)
3. ➡️ [reddit] On-chain metrics show accumulation phase (0.15)

=== CURRENT POSITION ===

No open position

=== ACCOUNT INFO ===

Balance: $1000.00
Equity: $1000.00
Daily PnL: $0.00 (0.00%)

=== YOUR DECISION ===

Based on the above data, provide your trading decision in JSON format.
```

## 6. AI Response Example

```json
{
  "action": "OPEN_LONG",
  "reason": "Strong bullish sentiment (+0.42) combined with MACD bullish crossover and RSI not overbought. ETF approval news is highly positive. Good risk/reward setup.",
  "size": 0.025,
  "stop_loss": 42100.0,
  "take_profit": 45000.0,
  "confidence": 85
}
```

## 7. News Impact on Decisions

**Сценарии:**

### Positive News + Bullish Indicators → HIGH confidence

```
News: 0.6 (bullish)
RSI: 55 (neutral)
MACD: bullish
→ AI decision: OPEN_LONG (confidence: 85%)
```

### Negative News + Bullish Indicators → HOLD

```
News: -0.5 (bearish, e.g., "SEC lawsuit")
RSI: 55
MACD: bullish
→ AI decision: HOLD (confidence: 50%, below threshold)
```

### No News + Strong Indicators → Moderate confidence

```
News: 0.1 (neutral)
RSI: 70 (overbought)
MACD: bullish
→ AI decision: HOLD (wait for better entry)
```

## 8. Cache Performance

**Without Cache (old approach):**

```
Trading Decision (every 30 min)
  ↓
Fetch Twitter API (2-3 sec)
Fetch Reddit API (1-2 sec)
Fetch CoinDesk (1-2 sec)
  ↓
Total: 4-7 seconds latency
```

**With Cache (new approach):**

```
Trading Decision (every 30 min)
  ↓
Read from PostgreSQL cache (<50ms)
  ↓
Total: <50ms latency

Meanwhile in background:
  News Worker (every 10 min)
    ↓
  Fetch all sources (4-7 sec)
    ↓
  Update cache
```

**Benefits:**

- ✅ 100x faster trading decisions
- ✅ No API timeout during critical moments
- ✅ All users share same cache (efficient)
- ✅ Works even if news APIs are down temporarily

## 9. Monitoring News

### SQL Queries

Recent news:

```sql
SELECT * FROM recent_news LIMIT 10;
```

Sentiment over time:

```sql
SELECT
    DATE_TRUNC('hour', published_at) as hour,
    AVG(sentiment) as avg_sentiment,
    COUNT(*) as news_count
FROM news_items
WHERE published_at > NOW() - INTERVAL '24 hours'
GROUP BY hour
ORDER BY hour DESC;
```

Source breakdown:

```sql
SELECT
    source,
    COUNT(*) as count,
    AVG(sentiment) as avg_sentiment
FROM news_items
WHERE published_at > NOW() - INTERVAL '6 hours'
GROUP BY source;
```

### Logs

News worker logs every fetch:

```
2024-11-22 10:35:00 INFO news cached successfully
  total_items=15 sentiment=bullish score=0.42 duration=3.2s
```

Sentiment breakdown:

```
2024-11-22 10:35:00 DEBUG sentiment breakdown
  positive=9 negative=3 neutral=3
```

## 10. Telegram Command

**Future enhancement - добавить команду для проверки новостей:**

```
/news
Bot: 📰 Latest News Sentiment

     Overall: BULLISH (0.42)
     Last 6 hours: 15 items
     📈 Positive: 9
     📉 Negative: 3
     ➡️ Neutral: 3

     Top Headlines:
     1. 📈 Bitcoin ETF approval signals...
     2. 📈 Institutional BTC holdings up...
     3. ➡️ On-chain accumulation phase...

     Last updated: 2 minutes ago
```

## Summary

### Data Flow Timeline

```
T=0min:   Bot starts → News Worker starts → Immediate fetch
T=1min:   News cached to DB
T=10min:  News Worker fetches again
T=20min:  News Worker fetches again
T=30min:  Trading Engine reads cache → Makes decision
T=40min:  News Worker fetches again
T=60min:  Trading Engine reads cache → Makes decision
...
```

### Why This Works Better

| Aspect      | Old (inline fetch) | New (background worker) |
| ----------- | ------------------ | ----------------------- |
| Latency     | 4-7 seconds        | <50ms                   |
| Efficiency  | Every user fetches | One fetch for all       |
| Reliability | Blocks on timeout  | Async, doesn't block    |
| Freshness   | Only when needed   | Always fresh            |
| Cost        | More API calls     | Optimized calls         |

### Key Points

1. **NewsWorker** runs независимо в фоне
2. **Каждые 10 минут** обновляет кэш
3. **Trading Engines** читают из кэша (быстро!)
4. **Один воркер** обслуживает всех пользователей
5. **Sentiment** уже рассчитан и готов к использованию

Всё работает автоматически без дополнительной настройки! 🚀
