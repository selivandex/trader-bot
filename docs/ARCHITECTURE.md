<!-- @format -->

# Architecture Documentation

## System Overview

The AI Trading Bot is a multi-user, multi-pair automated cryptocurrency trading system that uses AI models for decision-making, integrates news sentiment analysis, and implements comprehensive risk management.

## Core Components

### 1. Bot Manager (internal/bot/)

**MultiPairManager** orchestrates all user bot instances:

```go
type MultiPairManager struct {
    userBots map[int64]map[string]*UserBot
    // userID -> symbol -> bot instance
}
```

**Responsibilities:**

- Start/stop per-user per-pair bots
- Health monitoring
- Graceful shutdown
- Load configurations from database

### 2. Trading Engine (internal/strategy/)

Each pair runs its own **Engine** instance:

```go
type Engine struct {
    exchange       Exchange
    aiEnsemble     *ai.Ensemble
    newsAggregator *news.Aggregator
    riskManager    *RiskManager
    portfolio      *Tracker
}
```

**Main Loop (every 30 minutes):**

1. Collect market data (OHLCV, orderbook, funding rate)
2. Calculate indicators (RSI, MACD, BB)
3. Fetch news & sentiment
4. Query AI providers
5. Validate consensus
6. Execute if approved
7. Monitor & alert

### 3. Exchange Layer (internal/adapters/exchange/)

**CCXT Integration:**

```go
type Exchange interface {
    FetchTicker(symbol string) (*Ticker, error)
    FetchOHLCV(symbol, timeframe string, limit int) ([]Candle, error)
    CreateOrder(...) (*Order, error)
    SetLeverage(symbol string, leverage int) error
    // ... more methods
}
```

**Implementations:**

- BinanceAdapter (ccxt.NewBinance)
- BybitAdapter (ccxt.NewBybit)
- MockExchange (for testing)

### 4. AI Layer (internal/adapters/ai/)

**Ensemble Pattern:**

```go
type Ensemble struct {
    providers []Provider // [DeepSeek, Claude, GPT]
    minConsensus int     // 2 of 3
}
```

**Flow:**

```
Prompt → [DeepSeek, Claude, GPT] (parallel)
         ↓         ↓        ↓
      Decision1 Decision2 Decision3
         ↓         ↓        ↓
         Consensus Algorithm
                ↓
         Final Decision (if agreement)
```

### 5. News & Sentiment (internal/adapters/news/, internal/sentiment/)

**Aggregator Pattern:**

```go
type Aggregator struct {
    providers []Provider // [Twitter, Forklog, CoinDesk]
}
```

**Sentiment Analysis:**

- Keyword-based scoring
- Crypto-specific vocabulary
- -1.0 (bearish) to +1.0 (bullish)
- Integrated into AI prompts

### 6. Risk Management (internal/risk/)

**Three Components:**

```go
type RiskManager struct {
    CircuitBreaker *CircuitBreaker
    PositionSizer  *PositionSizer
    Validator      *Validator
}
```

**Circuit Breaker States:**

```
Running → 3 consecutive losses → Open (4h cooldown)
Running → -5% daily loss → Open (4h cooldown)
Open → cooldown expires → Running
Open → /resume command → Running
```

### 7. Database Schema

```sql
users (id, telegram_id, username)
    ↓
user_configs (user_id, symbol, exchange, api_key, balance)
    ↓
user_states (user_id, symbol, balance, equity, pnl)
    ↓
trades (user_id, symbol, side, amount, price, pnl)
positions (user_id, symbol, size, entry_price, pnl)
ai_decisions (user_id, provider, response, executed)
```

**Key Design:**

- UNIQUE(user_id, symbol) - multiple pairs per user
- Foreign keys with CASCADE
- Indexes on user_id, symbol
- Views for aggregated data
- Triggers for timestamps

## Data Flow

### Registration Flow

```
User → /start → Telegram Bot
                    ↓
              CreateUser(telegram_id)
                    ↓
              INSERT INTO users
                    ↓
              Welcome message
```

### Trading Setup Flow

```
User → /connect binance KEY SECRET
            ↓
       SaveConfig(user_id, exchange, keys)
            ↓
       INSERT INTO user_configs
            ↓
       INSERT INTO user_states (initial)
            ↓
       /addpair BTC/USDT 1000
            ↓
       INSERT INTO user_configs (user_id, 'BTC/USDT', 1000)
       INSERT INTO user_states (user_id, 'BTC/USDT', 1000)
```

### Trading Cycle Flow

```
Timer (30min)
    ↓
[Market Data Collection]
├─ Exchange.FetchTicker()
├─ Exchange.FetchOHLCV(5m, 15m, 1h, 4h)
├─ Exchange.FetchOrderBook()
├─ Exchange.FetchFundingRate()
└─ NewsAggregator.FetchAllNews(6h)
    ↓
[Indicator Calculation]
├─ Calculator.Calculate(candles)
├─ RSI, MACD, BollingerBands
└─ Trend, Volume, Volatility
    ↓
[AI Decision]
├─ BuildPrompt(marketData, position, news)
├─ Ensemble.Analyze() → [DeepSeek, Claude] parallel
├─ Wait for all responses
└─ Calculate consensus
    ↓
[Risk Validation]
├─ CircuitBreaker.IsOpen() ?
├─ Validator.ValidateMarketConditions()
├─ Validator.CheckDrawdown()
└─ Validator.ValidateDecision()
    ↓
[Execution]
├─ PositionSizer.CalculatePositionSize()
├─ Exchange.SetLeverage()
├─ Exchange.CreateOrder()
└─ Portfolio.RecordTrade()
    ↓
[Monitoring]
├─ Portfolio.UpdateFromExchange()
├─ CheckProfitWithdrawal()
└─ Telegram.AlertTradeOpened()
```

## Concurrency Model

```
Main Thread
    ↓
Bot Manager (monitor loop)
    ↓
├─ User 1 Bot → Goroutine 1
│   ├─ BTC/USDT Engine → Goroutine 1.1
│   └─ ETH/USDT Engine → Goroutine 1.2
│
├─ User 2 Bot → Goroutine 2
│   └─ BTC/USDT Engine → Goroutine 2.1
│
└─ Telegram Bot → Goroutine 3
    └─ Command handlers → Goroutines 3.x
```

**Synchronization:**

- Each engine has independent context
- Portfolio tracker uses mutex for state updates
- Circuit breaker uses RWMutex
- Database handles concurrency via transactions

## Error Handling Strategy

```
Error occurs
    ↓
Log error (zap)
    ↓
Telegram alert (if critical)
    ↓
Circuit breaker check
    ↓
Continue or stop based on severity
```

**Levels:**

- **Warning**: Log only (failed to fetch news)
- **Error**: Log + continue (AI provider timeout)
- **Critical**: Log + alert + circuit breaker (consecutive losses)
- **Fatal**: Log + shutdown (database connection lost)

## Configuration Precedence

```
1. Environment variables (.env)
2. Default values (in envconfig tags)
3. Per-user database overrides
```

Example:

- Global: AI_DECISION_INTERVAL=30m
- Per-user: initial_balance can differ

## Security Model

**Data Isolation:**

- Per-user database rows
- Foreign key constraints
- Query filters by user_id

**API Key Storage:**

- Stored in user_configs table
- Hidden in JSON responses
- TODO: Add encryption at rest

**Telegram:**

- Only registered users can control bots
- Commands require user lookup
- Isolated per telegram_id

## Scaling Considerations

**Current Capacity:**

- Single server: 10-50 users comfortably
- PostgreSQL connection pool: 25 connections
- Each bot: ~10MB RAM

**Bottlenecks:**

- AI API rate limits
- Exchange API rate limits
- PostgreSQL connections

**Horizontal Scaling:**

- Partition users across servers
- Shared PostgreSQL
- Separate Telegram bot instances

## Monitoring & Observability

**Logs:**

- Structured logging (zap)
- Log levels: debug, info, warn, error
- File + console output

**Database Views:**

- user_overview - all users at a glance
- recent_trades_by_user - trade history
- ai_provider_stats_by_user - AI accuracy

**Metrics (potential):**

- Trades per minute
- AI decision latency
- Circuit breaker events
- Active users/pairs

## Deployment

**Development:**

```bash
go run cmd/bot/main.go
```

**Docker:**

```bash
docker-compose up -d
```

**Production:**

- Use proper .env with real credentials
- Set MODE=paper initially
- Monitor for 1 month
- Switch to MODE=live only after proven results
- Set up log rotation
- Regular database backups

## Failure Modes & Recovery

**AI Provider Down:**

- Other providers continue
- Consensus may fail (no trade)
- Alert via Telegram

**Exchange API Down:**

- Retry with exponential backoff
- Log error
- Skip trading cycle
- Alert user

**Database Connection Lost:**

- Fatal error
- Graceful shutdown
- Alert admin
- Manual restart required

**Bot Crash:**

- Docker restarts container
- State recovered from database
- Positions remain on exchange
- Telegram notification on restart

## Testing Strategy

**Unit Tests:**

- Mock all external dependencies
- Test business logic
- Fast execution

**Integration Tests:**

- Real PostgreSQL (transactions)
- Mock exchange & AI
- Test component interactions

**Paper Trading:**

- Real-time data
- Simulated execution
- 1 month minimum
- Track metrics

## Performance Characteristics

**Latency:**

- Market data fetch: 100-500ms
- Indicator calculation: <10ms
- AI decision: 2-5 seconds
- Order execution: 200-1000ms
- **Total cycle: 3-7 seconds**

**Throughput:**

- One decision per 30 minutes per pair
- 48 decisions/day per pair
- With 10 pairs: 480 decisions/day

**Resource Usage:**

- CPU: Low (mostly waiting on I/O)
- Memory: ~50MB per user bot
- Database: ~100MB per user per month
- Network: ~10MB/day per pair

This architecture is designed for reliability, scalability, and ease of maintenance while prioritizing risk management and user safety.
