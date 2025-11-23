# Hybrid Architecture: PostgreSQL + ClickHouse

## 🎯 Data Distribution Strategy

### PostgreSQL (Hot Data + Vector Search)

**Purpose:** Operational data, frequent updates, vector similarity search

**Tables:**

1. **Agents (OLTP - frequent updates)**
   - `agent_configs` - Agent configuration
   - `agent_states` - Current state (balance, PnL, etc)
   - `agent_memory` - Statistical memory
   - `agent_semantic_memories` - **VECTOR SEARCH** ⚡
   - `collective_agent_memories` - **VECTOR SEARCH** ⚡
   - `agent_reasoning_sessions` - Last 30 days
   - `agent_reflections` - Last 30 days
   - `agent_trading_plans` - Active plans

2. **Trading (OLTP - transactions)**
   - `trades` - Active trades only
   - `orders` - Active orders
   - `positions` - Current positions

3. **Users**
   - `users`
   - `user_exchanges`

**Size:** 1-10 GB

**Retention:** 
- Trades: Move to ClickHouse after close
- Reasoning/Reflections: Keep 30 days, then archive

---

### ClickHouse (Cold Data + Analytics)

**Purpose:** Historical data, analytics, time-series

**Tables:**

1. **Market Data (Time-Series)**
   - `market_ohlcv` - All candles, all timeframes
   - `market_tickers` - Ticker snapshots
   - `market_orderbook` - Order book snapshots (optional)

2. **Trading History (Immutable)**
   - `trades_history` - All closed trades
   - `orders_history` - All filled/cancelled orders

3. **News & Sentiment (Time-Series)**
   - `news_articles` - All news
   - `sentiment_data` - Sentiment history

4. **On-Chain (Time-Series)**
   - `onchain_transactions` - Whale transactions
   - `exchange_flows` - Exchange flow data

5. **Agent Metrics (Time-Series)**
   - `agent_performance` - Performance snapshots (hourly)
   - `agent_decisions_log` - All decisions (for analysis)

**Size:** 100 GB - 1 TB+

**Retention:** Forever (or with TTL policy)

---

## 📊 ClickHouse Schema

```sql
-- Market OHLCV (primary data source)
CREATE TABLE market_ohlcv (
    timestamp DateTime64(3),
    symbol LowCardinality(String),
    timeframe LowCardinality(String),
    open Float64,
    high Float64,
    low Float64,
    close Float64,
    volume Float64,
    quote_volume Float64,
    trades UInt32,
    date Date MATERIALIZED toDate(timestamp)
) ENGINE = MergeTree()
PARTITION BY (symbol, toYYYYMM(timestamp))
ORDER BY (symbol, timeframe, timestamp)
SETTINGS index_granularity = 8192;

-- Trades History
CREATE TABLE trades_history (
    id UUID,
    agent_id UUID,
    user_id UUID,
    symbol LowCardinality(String),
    side Enum8('long' = 1, 'short' = 2),
    entry_price Float64,
    exit_price Float64,
    size Float64,
    leverage UInt8,
    pnl Float64,
    pnl_percent Float64,
    fee Float64,
    opened_at DateTime64(3),
    closed_at DateTime64(3),
    duration UInt32,
    entry_reason String,
    exit_reason String,
    date Date MATERIALIZED toDate(closed_at)
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(closed_at)
ORDER BY (agent_id, closed_at);

-- News Articles
CREATE TABLE news_articles (
    id UUID,
    source LowCardinality(String),
    title String,
    content String,
    url String,
    author String,
    sentiment Float32,
    impact UInt8,
    symbols Array(String),
    published_at DateTime64(3),
    processed_at DateTime64(3),
    date Date MATERIALIZED toDate(published_at)
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(published_at)
ORDER BY (published_at, source);

-- On-Chain Transactions
CREATE TABLE onchain_transactions (
    id UUID,
    transaction_hash String,
    transaction_type LowCardinality(String),
    symbol LowCardinality(String),
    from_address String,
    to_address String,
    amount Float64,
    amount_usd Float64,
    impact_score UInt8,
    blockchain LowCardinality(String),
    detected_at DateTime64(3),
    date Date MATERIALIZED toDate(detected_at)
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(detected_at)
ORDER BY (symbol, detected_at);

-- Agent Performance Metrics
CREATE TABLE agent_performance (
    agent_id UUID,
    timestamp DateTime64(3),
    balance Float64,
    equity Float64,
    pnl Float64,
    pnl_percent Float64,
    total_trades UInt32,
    winning_trades UInt32,
    losing_trades UInt32,
    win_rate Float32,
    sharpe_ratio Float32,
    max_drawdown Float32,
    date Date MATERIALIZED toDate(timestamp)
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (agent_id, timestamp);

-- Agent Decisions Log (for analysis)
CREATE TABLE agent_decisions_log (
    id UUID,
    agent_id UUID,
    symbol LowCardinality(String),
    action LowCardinality(String),
    confidence UInt8,
    technical_score Float32,
    news_score Float32,
    onchain_score Float32,
    sentiment_score Float32,
    final_score Float32,
    executed Bool,
    created_at DateTime64(3),
    date Date MATERIALIZED toDate(created_at)
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(created_at)
ORDER BY (agent_id, created_at);
```

---

## 🔄 Data Flow

### 1. Market Data Flow

```
Exchange API
    ↓
Worker (candles_worker.go)
    ↓
ClickHouse (direct insert)
    ↑
Agents read from ClickHouse
```

**No PostgreSQL cache needed** - ClickHouse is fast enough for reads!

### 2. Trading Flow

```
Agent Decision
    ↓
PostgreSQL (trades table) - active trade
    ↓
Trade Closes
    ↓
Move to ClickHouse (trades_history)
    ↓
Delete from PostgreSQL
```

### 3. Memory Flow

```
Trade Experience
    ↓
AI Summarization
    ↓
Generate Embedding (OpenAI)
    ↓
PostgreSQL (agent_semantic_memories) ← VECTOR SEARCH
```

**Memory stays in PostgreSQL ONLY!**

---

## 🚀 Implementation

### Config

```go
// internal/adapters/config/config.go

type Config struct {
    PostgreSQL struct {
        DSN string
    }
    
    ClickHouse struct {
        DSN string  // clickhouse://localhost:9000/trader
    }
    
    OpenAI struct {
        APIKey string  // For embeddings
    }
    
    // ... other config
}
```

### Database Adapters

```go
// internal/adapters/database/db.go

type DatabaseManager struct {
    PG *sqlx.DB          // PostgreSQL
    CH *sql.DB           // ClickHouse
}

func NewDatabaseManager(cfg *config.Config) (*DatabaseManager, error) {
    // PostgreSQL
    pg, err := sqlx.Connect("postgres", cfg.PostgreSQL.DSN)
    if err != nil {
        return nil, err
    }
    
    // ClickHouse
    ch, err := sql.Open("clickhouse", cfg.ClickHouse.DSN)
    if err != nil {
        return nil, err
    }
    
    return &DatabaseManager{
        PG: pg,
        CH: ch,
    }, nil
}
```

### Market Repository

```go
// internal/adapters/market/repository.go

type Repository struct {
    ch *sql.DB  // ClickHouse
}

// GetCandles reads from ClickHouse
func (r *Repository) GetCandles(
    ctx context.Context,
    symbol string,
    timeframe string,
    limit int,
) ([]models.Candle, error) {
    query := `
        SELECT timestamp, symbol, timeframe, open, high, low, close, volume
        FROM market_ohlcv
        WHERE symbol = ? AND timeframe = ?
        ORDER BY timestamp DESC
        LIMIT ?
    `
    
    rows, err := r.ch.QueryContext(ctx, query, symbol, timeframe, limit)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    var candles []models.Candle
    for rows.Next() {
        var c models.Candle
        err := rows.Scan(&c.Timestamp, &c.Symbol, &c.Timeframe, 
            &c.Open, &c.High, &c.Low, &c.Close, &c.Volume)
        if err != nil {
            return nil, err
        }
        candles = append(candles, c)
    }
    
    return candles, nil
}
```

### Candles Worker

```go
// internal/workers/candles_worker.go

type CandlesWorker struct {
    ch       *sql.DB
    exchange exchange.Exchange
}

func (w *CandlesWorker) FetchAndStore(ctx context.Context, symbol, timeframe string) error {
    // 1. Fetch from exchange
    candles, err := w.exchange.FetchOHLCV(ctx, symbol, timeframe, 100)
    if err != nil {
        return err
    }
    
    // 2. Direct insert to ClickHouse (no PostgreSQL!)
    batch, err := w.ch.Begin()
    if err != nil {
        return err
    }
    
    stmt, err := batch.Prepare(`
        INSERT INTO market_ohlcv 
        (timestamp, symbol, timeframe, open, high, low, close, volume, quote_volume, trades)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `)
    
    for _, candle := range candles {
        _, err = stmt.Exec(
            candle.Timestamp,
            symbol,
            timeframe,
            candle.Open.InexactFloat64(),
            candle.High.InexactFloat64(),
            candle.Low.InexactFloat64(),
            candle.Close.InexactFloat64(),
            candle.Volume.InexactFloat64(),
            candle.QuoteVolume.InexactFloat64(),
            candle.Trades,
        )
        if err != nil {
            batch.Rollback()
            return err
        }
    }
    
    return batch.Commit()
}
```

### Trade Archiver

```go
// internal/workers/trade_archiver.go

type TradeArchiver struct {
    pg *sqlx.DB
    ch *sql.DB
}

// Archive moves closed trades from PostgreSQL to ClickHouse
func (ta *TradeArchiver) ArchiveClosedTrades(ctx context.Context) error {
    // 1. Get closed trades from PostgreSQL
    var trades []models.Trade
    err := ta.pg.SelectContext(ctx, &trades, `
        SELECT * FROM trades 
        WHERE status = 'closed' 
        AND archived_at IS NULL
        LIMIT 1000
    `)
    
    if len(trades) == 0 {
        return nil
    }
    
    // 2. Insert to ClickHouse
    batch, _ := ta.ch.Begin()
    stmt, _ := batch.Prepare(`
        INSERT INTO trades_history 
        (id, agent_id, user_id, symbol, side, entry_price, exit_price, 
         size, leverage, pnl, pnl_percent, fee, opened_at, closed_at, 
         duration, entry_reason, exit_reason)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `)
    
    tradeIDs := make([]string, 0, len(trades))
    for _, trade := range trades {
        stmt.Exec(
            trade.ID,
            trade.AgentID,
            trade.UserID,
            trade.Symbol,
            trade.Side,
            trade.EntryPrice.InexactFloat64(),
            trade.ExitPrice.InexactFloat64(),
            trade.Size.InexactFloat64(),
            trade.Leverage,
            trade.PnL.InexactFloat64(),
            trade.PnLPercent,
            trade.Fee.InexactFloat64(),
            trade.OpenedAt,
            trade.ClosedAt,
            trade.Duration,
            trade.EntryReason,
            trade.ExitReason,
        )
        tradeIDs = append(tradeIDs, trade.ID)
    }
    
    batch.Commit()
    
    // 3. Mark as archived in PostgreSQL (or delete)
    _, err = ta.pg.ExecContext(ctx, `
        DELETE FROM trades WHERE id = ANY($1)
    `, tradeIDs)
    
    logger.Info("archived trades to ClickHouse",
        zap.Int("count", len(trades)),
    )
    
    return err
}

// Run periodically
func (ta *TradeArchiver) Run(ctx context.Context) {
    ticker := time.NewTicker(5 * time.Minute)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            if err := ta.ArchiveClosedTrades(ctx); err != nil {
                logger.Error("failed to archive trades", zap.Error(err))
            }
        case <-ctx.Done():
            return
        }
    }
}
```

---

## 📈 Query Examples

### Fast Candle Queries

```go
// Get last 100 candles for agent decision
candles, _ := marketRepo.GetCandles(ctx, "BTC/USDT", "5m", 100)

// ClickHouse returns in milliseconds (very fast!)
```

### Agent Performance Analytics

```sql
-- Win rate by agent over time
SELECT 
    agent_id,
    toStartOfHour(timestamp) as hour,
    avg(win_rate) as avg_win_rate
FROM agent_performance
WHERE timestamp > now() - INTERVAL 7 DAY
GROUP BY agent_id, hour
ORDER BY hour;

-- Best performing agents
SELECT 
    agent_id,
    sum(pnl) as total_pnl,
    count(*) as total_trades,
    avg(pnl_percent) as avg_return
FROM trades_history
WHERE closed_at > now() - INTERVAL 30 DAY
GROUP BY agent_id
ORDER BY total_pnl DESC;
```

### Market Analytics

```sql
-- Price volatility by hour
SELECT 
    toStartOfHour(timestamp) as hour,
    symbol,
    max(high) - min(low) as range,
    avg(volume) as avg_volume
FROM market_ohlcv
WHERE timeframe = '1h'
  AND timestamp > now() - INTERVAL 7 DAY
GROUP BY hour, symbol
ORDER BY hour;
```

---

## 🎯 Migration Path

### Phase 1: Setup ClickHouse

1. Create ClickHouse database
2. Run ClickHouse migrations
3. Update config with ClickHouse DSN

### Phase 2: Migrate Candles

1. Update `candles_worker.go` to write to ClickHouse
2. Backfill historical data from PostgreSQL → ClickHouse
3. Drop `ohlcv_candles` from PostgreSQL

### Phase 3: Migrate Trade History

1. Create `trade_archiver` worker
2. Start archiving closed trades
3. Keep active trades in PostgreSQL

### Phase 4: Add Analytics

1. Create performance tracking
2. Build dashboards on ClickHouse data
3. Add materialized views for common queries

---

## ⚡ Performance Expectations

### ClickHouse Queries:

- Get 1000 candles: **< 10ms**
- Aggregate 1M trades: **< 100ms**
- Complex analytics (7 days): **< 500ms**

### PostgreSQL Queries:

- Vector similarity search: **< 50ms** (with IVFFlat index)
- Get agent state: **< 5ms**
- Update agent: **< 10ms**

---

## 💰 Cost Considerations

### Storage:

- PostgreSQL: 10 GB (hot data) - $1-2/month
- ClickHouse: 500 GB (historical) - $10-20/month (on cloud)

### Compute:

- PostgreSQL: Small instance (2 CPU, 4GB RAM)
- ClickHouse: Medium instance (4 CPU, 8GB RAM)

**Total: ~$50-100/month** for production-ready setup

---

## ✅ Summary

| Data Type | Database | Reason |
|-----------|----------|---------|
| **Agent Memory** | PostgreSQL | Vector search required |
| **Agent Config/State** | PostgreSQL | Frequent updates |
| **Active Trades** | PostgreSQL | Transactions critical |
| **OHLCV Candles** | ClickHouse | Time-series, huge volume |
| **Trade History** | ClickHouse | Analytics, immutable |
| **News Archive** | ClickHouse | Time-series, search |
| **Metrics** | ClickHouse | Time-series analytics |

**Key Principle:** 
- Hot, mutable data + vector search → PostgreSQL
- Cold, immutable data + analytics → ClickHouse

