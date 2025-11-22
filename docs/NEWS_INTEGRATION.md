<!-- @format -->

# News Integration Guide

The trading bot runs a **background worker** that continuously fetches, analyzes, and caches news from multiple sources.

## Architecture

```
Background News Worker (every 10 minutes)
    ↓
[Twitter, Reddit, CoinDesk] → Fetch news
    ↓
Sentiment Analysis
    ↓
Cache to PostgreSQL (news_items table)
    ↓
Trading Engine reads from cache (instant access)
```

## Supported News Sources

### 1. Reddit (r/CryptoCurrency, r/Bitcoin, r/ethereum)

Community sentiment from Reddit crypto communities.

**Setup:**

```bash
REDDIT_ENABLED=true
```

**Features:**

- No API key required (public JSON API)
- Tracks hot posts from crypto subreddits
- Engagement-based relevance (upvotes + comments)
- Real community sentiment

**Why Reddit:**

- Free, no API key
- High quality discussions
- Early sentiment indicators
- Community-driven insights

### 2. CoinDesk

Professional crypto news from leading outlet.

**Setup:**

```bash
COINDESK_ENABLED=true
```

**Features:**

- No API key required
- High reliability source
- Professional journalism
- Breaking news coverage

### 3. Twitter (X) - Optional

Real-time crypto sentiment from Twitter.

**Setup:**

1. Get Twitter API v2 Bearer Token from https://developer.twitter.com
2. Set environment variable:

```bash
TWITTER_API_KEY=your_bearer_token
TWITTER_ENABLED=true
```

**Features:**

- Real-time updates
- Influential accounts
- Engagement-based filtering
- Viral sentiment detection

**Note:** Twitter requires paid API access now, so it's optional.

## Configuration

Add to `.env`:

```bash
# News Aggregation
NEWS_ENABLED=true
NEWS_KEYWORDS=bitcoin,btc,crypto,cryptocurrency,ethereum,eth

# News Sources (free, no API keys)
REDDIT_ENABLED=true
COINDESK_ENABLED=true

# Twitter (optional, requires API key)
TWITTER_API_KEY=your_bearer_token
TWITTER_ENABLED=false
```

## Sentiment Analysis

The bot uses keyword-based sentiment analysis with crypto-specific vocabulary:

**Positive keywords:** bullish, rally, surge, moon, breakout, ATH, ETF approval, etc.
**Negative keywords:** bearish, crash, dump, hack, exploit, liquidation, FUD, etc.

Sentiment score ranges from -1.0 (very bearish) to +1.0 (very bullish).

## How It Works

### Background Worker (every 10 minutes)

1. **News Collection**:

   - Fetches from Reddit, CoinDesk, Twitter (if enabled)
   - Filters by keywords (BTC, crypto, ethereum, etc.)
   - Analyzes sentiment using keyword-based analyzer
   - Calculates relevance score (engagement-based)

2. **Caching**:

   - Saves to PostgreSQL `news_items` table
   - Upserts on conflict (updates sentiment if exists)
   - Keeps last 7 days
   - Daily cleanup of old items

3. **Aggregation**:
   - Calculates rolling averages (1h, 6h, 24h)
   - Detects sentiment momentum (improving/declining)
   - Counts positive/negative/neutral news
   - Identifies overall trend (bullish/bearish/neutral)

### Trading Engine Integration

When making trading decision (every 30 minutes):

1. **Read from Cache** (instant, no API calls):

   ```go
   summary := newsAggregator.GetCachedSummary(ctx, 6*time.Hour)
   ```

2. **Include in AI Prompt**:

   - Overall sentiment (bullish/bearish/neutral)
   - Average sentiment score
   - Recent headlines (top 3-5)
   - News count breakdown

3. **AI considers**:
   - Technical indicators (RSI, MACD, etc.)
   - News sentiment
   - Sentiment momentum
   - Recent events

### Benefits

✅ **Fast**: No waiting for API calls during trading decisions
✅ **Reliable**: Cache works even if news APIs are down
✅ **Efficient**: One fetch serves all users/pairs
✅ **Fresh**: Updates every 10 minutes
✅ **Historical**: Can analyze sentiment trends over time

## Example AI Prompt Section

```
=== NEWS & SENTIMENT ===

Overall Sentiment: BULLISH
Average Score: 0.42
Total News: 15 (📈 9 | 📉 3 | ➡️ 3)

Recent Headlines:
1. 📈 [twitter] Bitcoin breaks through $44k resistance (0.65)
2. 📈 [forklog] Major exchange announces BTC ETF support (0.54)
3. ➡️ [twitter] On-chain metrics show accumulation phase (0.12)
```

## Adding New News Sources

To add a new news source:

1. Create provider in `internal/adapters/news/`:

```go
type MyNewsProvider struct {
    enabled   bool
    sentiment SentimentAnalyzer
}

func (m *MyNewsProvider) FetchLatestNews(ctx context.Context, keywords []string, limit int) ([]models.NewsItem, error) {
    // Implement fetching logic
}
```

2. Add configuration in `config.go`
3. Initialize in `cmd/bot/main.go`

## Best Practices

1. **Rate Limiting**: Be mindful of API rate limits (especially Twitter)
2. **Keywords**: Use specific keywords relevant to your trading symbols
3. **Time Window**: Default 6-hour window balances recency vs. data volume
4. **Fallback**: Bot works without news if all providers fail
5. **Testing**: Test with paper trading first to see how news affects decisions

## Troubleshooting

**No news fetched:**

- Check API keys are correct
- Verify internet connection
- Check provider is enabled in config

**Poor sentiment accuracy:**

- Add more specific keywords
- Adjust sentiment word lists in `sentiment/analyzer.go`

**API rate limits:**

- Reduce news fetch frequency
- Use fewer providers
- Cache results longer

## Future Enhancements

- [ ] CoinDesk integration
- [ ] CryptoCompare integration
- [ ] Reddit r/cryptocurrency sentiment
- [ ] AI-powered sentiment (via LLM)
- [ ] Named Entity Recognition for better filtering
- [ ] Sentiment trend analysis over time
- [ ] Breaking news alerts
