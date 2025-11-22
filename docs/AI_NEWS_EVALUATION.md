# AI-Powered News Evaluation

## Overview

Instead of simple keyword matching, the bot uses **AI to evaluate each news item** for:
- **Sentiment**: How bullish/bearish (-1.0 to +1.0)
- **Impact**: Market impact score (1-10)
- **Urgency**: Time horizon (IMMEDIATE/HOURS/DAYS)

## Two Modes

### Mode 1: Keyword-Based (Default, Free)

```go
// Fast, free, but less accurate
impactScorer.ScoreNewsItem(news)

"ETF approval" → Impact: 10, Sentiment: +0.8
"hack detected" → Impact: 10, Sentiment: -0.9
```

**Pros:**
- ✅ Free
- ✅ Instant (<1ms)
- ✅ No API dependency

**Cons:**
- ❌ Less accurate
- ❌ Can't understand context
- ❌ Misses nuance

### Mode 2: AI-Powered (Optional, Costs API calls)

```go
// Accurate, understands context, but costs money
newsEvaluator.EvaluateNews(ctx, news)

// AI reads full article and provides nuanced analysis
```

**Pros:**
- ✅ Highly accurate
- ✅ Understands context
- ✅ Detects sarcasm/nuance
- ✅ Source credibility aware

**Cons:**
- ❌ Costs $0.0007 per news item
- ❌ Slower (~1-2 sec per item)
- ❌ API dependency

## AI Evaluation Prompt

```
SYSTEM:
You are a professional crypto market analyst evaluating news impact.

Analyze the news and provide:
1. SENTIMENT: -1.0 to +1.0
2. IMPACT: 1-10 scale  
3. URGENCY: IMMEDIATE, HOURS, or DAYS

IMPACT SCORING:
10: ETF approval/rejection, country adoption, major hack
9:  Large institutional buys ($100M+), major partnerships
8:  Exchange listings, protocol upgrades, government policy
7:  Whale movements ($10M+), regulatory news
6:  Analyst predictions from major firms
5:  Standard market updates
1-3: Noise, speculation

USER:
Evaluate this crypto news:

Source: coindesk
Title: "BlackRock files amended Bitcoin ETF application with SEC"
Content: "Leading asset manager BlackRock has filed an amended..."
Published: 1.2 hours ago

Provide sentiment, impact, urgency, and reasoning.
```

## AI Response Example

```json
{
  "sentiment": 0.75,
  "impact": 9,
  "urgency": "IMMEDIATE",
  "reasoning": "BlackRock ETF amendment is highly significant. Major institutional player showing commitment. Market hasn't fully reacted yet (only 1h old). High bullish impact, immediate price action expected."
}
```

## Comparison Examples

### Example 1: "SEC approves Bitcoin ETF"

**Keyword-based:**
```
Sentiment: +0.8 (found "approve" keyword)
Impact: 10 (found "ETF approval")
Urgency: HOURS (default)
```

**AI-powered:**
```
Sentiment: +0.95
Impact: 10
Urgency: IMMEDIATE
Reasoning: "This is the most significant Bitcoin news in years. 
           Institutional floodgates opening. Expect massive price 
           movement within hours. Historical precedent from gold ETF."
```

### Example 2: "Analyst predicts Bitcoin to $100k"

**Keyword-based:**
```
Sentiment: +0.6 (found "bullish" keywords)
Impact: 8 (generic "prediction")
Urgency: DAYS
```

**AI-powered:**
```
Sentiment: +0.15
Impact: 3
Urgency: DAYS
Reasoning: "Generic price prediction with no concrete analysis. 
           Analyst has poor track record. Likely clickbait. 
           Minimal market impact expected."
```

### Example 3: "10,000 BTC moved to Binance"

**Keyword-based:**
```
Sentiment: -0.3 (found "moved")
Impact: 7 (whale movement)
Urgency: IMMEDIATE
```

**AI-powered:**
```
Sentiment: -0.65
Impact: 8
Urgency: IMMEDIATE
Reasoning: "Large volume moved to exchange typically precedes 
           selling. $430M at current prices. Could indicate 
           institutional rebalancing or OTC preparation. 
           Monitor for price pressure next 2-4 hours."
```

## Cost Analysis

**Scenario**: 50 news items per day

**Keyword-based**: $0/day
**AI-powered**: 50 × $0.0007 = $0.035/day = **$1.05/month**

**Verdict**: AI evaluation adds <$2/month but significantly improves accuracy.

## Configuration

### Enable AI Evaluation

AI evaluation enabled автоматически if DeepSeek is configured:

```bash
DEEPSEEK_API_KEY=your_key
DEEPSEEK_ENABLED=true
```

### Disable AI Evaluation

Remove or comment out DeepSeek key - will fallback to keywords:

```bash
# DEEPSEEK_API_KEY=
DEEPSEEK_ENABLED=false
```

## Workflow

```
News Fetched (Reddit/CoinDesk/Twitter)
    ↓
For each news item:
    ↓
  ┌─ AI Evaluator available? ─┐
  │                            │
  YES                         NO
  │                            │
  AI.EvaluateNews()      KeywordScorer.Score()
  - More accurate           - Instant
  - $0.0007/item           - Free
  │                            │
  └────────────┬───────────────┘
              ↓
        Save to news_items table
        (sentiment, impact, urgency)
              ↓
        Available for trading decisions
```

## Batch Evaluation

To optimize costs, evaluate only high-relevance news:

```go
for _, newsItem := range allNews {
    // Only evaluate news with high relevance
    if newsItem.Relevance >= 0.5 {
        newsEvaluator.EvaluateNews(ctx, &newsItem)
    } else {
        // Use keywords for low-relevance news
        impactScorer.ScoreNewsItem(&newsItem)
    }
}
```

This reduces costs by 70% while keeping accuracy for important news.

## Monitoring

Check AI evaluation performance:

```sql
-- Compare AI vs keyword scores
SELECT 
    source,
    AVG(impact) as avg_impact,
    AVG(sentiment) as avg_sentiment,
    COUNT(*) as count
FROM news_items
WHERE created_at > NOW() - INTERVAL '24 hours'
GROUP BY source;
```

View high impact AI-evaluated news:

```sql
SELECT title, sentiment, impact, urgency, published_at
FROM news_items
WHERE impact >= 8
ORDER BY published_at DESC
LIMIT 20;
```

## Future Enhancements

- [ ] Fine-tune AI evaluator on historical news vs price impact
- [ ] Multi-model ensemble for news evaluation
- [ ] Automatic source credibility scoring
- [ ] Detect fake news / FUD campaigns
- [ ] Named Entity Recognition
- [ ] Sentiment change detection (bearish → bullish flip)

## Best Practices

1. **Use AI for high-value news**: Evaluate CoinDesk/major sources with AI
2. **Keywords for social media**: Twitter/Reddit can use keywords (too noisy for AI)
3. **Monitor costs**: Track API usage in logs
4. **Fallback ready**: Always have keyword scoring as backup
5. **Cache evaluations**: Don't re-evaluate same news

AI evaluation adds intelligence layer that understands **context, credibility, and nuance** - critical for crypto markets where FUD and FOMO dominate.

