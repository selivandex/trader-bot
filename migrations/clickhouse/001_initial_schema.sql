-- ClickHouse Initial Schema
-- Time-series and analytical data storage
-- 1. Market OHLCV Data
CREATE TABLE
  IF NOT EXISTS market_ohlcv (
    timestamp DateTime64 (3),
    symbol LowCardinality (String),
    timeframe LowCardinality (String), -- '1m', '5m', '15m', '1h', '4h', '1d'
    open Float64,
    high Float64,
    low Float64,
    close Float64,
    volume Float64,
    quote_volume Float64,
    trades UInt32,
    date Date MATERIALIZED toDate (timestamp)
  ) ENGINE = MergeTree ()
PARTITION BY
  (symbol, toYYYYMM (timestamp))
ORDER BY
  (symbol, timeframe, timestamp) SETTINGS index_granularity = 8192;

-- 2. Trades History (archived closed trades)
CREATE TABLE
  IF NOT EXISTS trades_history (
    id UUID,
    agent_id UUID,
    user_id UUID,
    symbol LowCardinality (String),
    side Enum8 ('long' = 1, 'short' = 2),
    entry_price Float64,
    exit_price Float64,
    size Float64,
    leverage UInt8,
    pnl Float64,
    pnl_percent Float64,
    fee Float64,
    realized_pnl Float64,
    opened_at DateTime64 (3),
    closed_at DateTime64 (3),
    duration UInt32, -- seconds
    entry_reason String,
    exit_reason String,
    date Date MATERIALIZED toDate (closed_at)
  ) ENGINE = MergeTree ()
PARTITION BY
  toYYYYMM (closed_at)
ORDER BY
  (agent_id, closed_at, symbol);

-- 3. News Articles Archive
CREATE TABLE
  IF NOT EXISTS news_articles (
    id UUID,
    source LowCardinality (String), -- 'coindesk', 'reddit', 'twitter'
    title String,
    content String,
    url String,
    author String,
    sentiment Float32, -- -1.0 to 1.0
    impact UInt8, -- 0-10
    symbols Array (String), -- ['BTC', 'ETH']
    published_at DateTime64 (3),
    processed_at DateTime64 (3),
    date Date MATERIALIZED toDate (published_at)
  ) ENGINE = MergeTree ()
PARTITION BY
  toYYYYMM (published_at)
ORDER BY
  (published_at, source, id);

-- 4. Sentiment Data History
CREATE TABLE
  IF NOT EXISTS sentiment_data (
    id UUID,
    symbol LowCardinality (String),
    sentiment_type LowCardinality (String), -- 'news', 'social', 'funding'
    score Float32, -- -1.0 to 1.0
    volume UInt32, -- number of mentions/posts
    confidence Float32, -- 0.0 to 1.0
    source LowCardinality (String),
    timestamp DateTime64 (3),
    date Date MATERIALIZED toDate (timestamp)
  ) ENGINE = MergeTree ()
PARTITION BY
  toYYYYMM (timestamp)
ORDER BY
  (symbol, sentiment_type, timestamp);

-- 5. On-Chain Transactions
CREATE TABLE
  IF NOT EXISTS onchain_transactions (
    id UUID,
    transaction_hash String,
    transaction_type LowCardinality (String), -- 'exchange_inflow', 'exchange_outflow', 'large_transfer'
    symbol LowCardinality (String),
    from_address String,
    to_address String,
    amount Float64,
    amount_usd Float64,
    impact_score UInt8, -- 0-10
    blockchain LowCardinality (String), -- 'bitcoin', 'ethereum'
    exchange_name LowCardinality (String), -- 'binance', 'coinbase', null
    detected_at DateTime64 (3),
    date Date MATERIALIZED toDate (detected_at)
  ) ENGINE = MergeTree ()
PARTITION BY
  toYYYYMM (detected_at)
ORDER BY
  (symbol, detected_at, impact_score DESC);

-- 6. Exchange Flow Aggregations
CREATE TABLE
  IF NOT EXISTS exchange_flows (
    symbol LowCardinality (String),
    exchange_name LowCardinality (String),
    hour DateTime, -- hour bucket
    inflow Float64,
    outflow Float64,
    net_flow Float64,
    transaction_count UInt32,
    date Date MATERIALIZED toDate (hour)
  ) ENGINE = SummingMergeTree ()
PARTITION BY
  toYYYYMM (hour)
ORDER BY
  (symbol, exchange_name, hour) PRIMARY KEY (symbol, exchange_name, hour);

-- 7. Agent Performance Metrics (time-series snapshots)
CREATE TABLE
  IF NOT EXISTS agent_performance (
    agent_id UUID,
    timestamp DateTime64 (3),
    symbol LowCardinality (String),
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
    current_drawdown Float32,
    date Date MATERIALIZED toDate (timestamp)
  ) ENGINE = MergeTree ()
PARTITION BY
  toYYYYMM (timestamp)
ORDER BY
  (agent_id, timestamp);

-- 8. Agent Decisions Log (for analysis and backtesting)
CREATE TABLE
  IF NOT EXISTS agent_decisions_log (
    id UUID,
    agent_id UUID,
    symbol LowCardinality (String),
    action LowCardinality (String), -- 'open_long', 'open_short', 'close', 'hold'
    confidence UInt8, -- 0-100
    technical_score Float32,
    news_score Float32,
    onchain_score Float32,
    sentiment_score Float32,
    final_score Float32,
    reason String,
    executed Bool,
    execution_price Nullable (Float64),
    execution_size Nullable (Float64),
    outcome String, -- JSON with results
    created_at DateTime64 (3),
    date Date MATERIALIZED toDate (created_at)
  ) ENGINE = MergeTree ()
PARTITION BY
  toYYYYMM (created_at)
ORDER BY
  (agent_id, created_at);

-- 9. Order History (all orders)
CREATE TABLE
  IF NOT EXISTS orders_history (
    id UUID,
    agent_id UUID,
    user_id UUID,
    symbol LowCardinality (String),
    order_type LowCardinality (String), -- 'market', 'limit', 'stop_market'
    side LowCardinality (String), -- 'buy', 'sell'
    price Float64,
    amount Float64,
    filled Float64,
    status LowCardinality (String), -- 'open', 'filled', 'cancelled', 'rejected'
    created_at DateTime64 (3),
    updated_at DateTime64 (3),
    date Date MATERIALIZED toDate (created_at)
  ) ENGINE = ReplacingMergeTree (updated_at)
PARTITION BY
  toYYYYMM (created_at)
ORDER BY
  (id, created_at);

-- Materialized Views for Common Queries
-- Hourly candles aggregation (if we store 1m and want 1h)
CREATE MATERIALIZED VIEW IF NOT EXISTS market_ohlcv_1h ENGINE = MergeTree ()
PARTITION BY
  toYYYYMM (hour)
ORDER BY
  (symbol, hour) AS
SELECT
  symbol,
  toStartOfHour (timestamp) as hour,
  '1h' as timeframe,
  argMin (open, timestamp) as open,
  max(high) as high,
  min(low) as low,
  argMax (close, timestamp) as close,
  sum(volume) as volume,
  sum(quote_volume) as quote_volume,
  sum(trades) as trades
FROM
  market_ohlcv
WHERE
  timeframe = '1m'
GROUP BY
  symbol,
  hour;

-- Daily agent performance summary
CREATE MATERIALIZED VIEW IF NOT EXISTS agent_performance_daily ENGINE = SummingMergeTree ()
PARTITION BY
  toYYYYMM (date)
ORDER BY
  (agent_id, date) AS
SELECT
  agent_id,
  toDate (timestamp) as date,
  max(balance) as end_balance,
  max(equity) as end_equity,
  max(pnl) as end_pnl,
  max(total_trades) as total_trades,
  max(win_rate) as end_win_rate
FROM
  agent_performance
GROUP BY
  agent_id,
  date;

-- Comments
COMMENT ON TABLE market_ohlcv IS 'OHLCV candles for all symbols and timeframes';

COMMENT ON TABLE trades_history IS 'Historical closed trades from all agents';

COMMENT ON TABLE news_articles IS 'Archived news articles with sentiment analysis';

COMMENT ON TABLE onchain_transactions IS 'On-chain whale transactions and exchange flows';

COMMENT ON TABLE agent_performance IS 'Time-series snapshots of agent performance metrics';

COMMENT ON TABLE agent_decisions_log IS 'All agent decisions for backtesting and analysis';

-- ==========================================
-- Buffer Tables (for high-frequency inserts)
-- ==========================================
-- Buffer for OHLCV candles (WebSocket real-time data)
CREATE TABLE
  IF NOT EXISTS market_ohlcv_buffer AS market_ohlcv ENGINE = Buffer (
    trader, -- database
    market_ohlcv, -- destination table
    16, -- number of shards
    10, -- min time to flush (seconds)
    100, -- max time to flush (seconds)
    10000, -- min rows to flush
    1000000, -- max rows to flush
    10000000, -- min bytes to flush (10MB)
    100000000 -- max bytes to flush (100MB)
  );

-- Usage:
-- INSERT INTO market_ohlcv_buffer VALUES (...)  -- Fast, buffered
-- SELECT * FROM market_ohlcv WHERE ...          -- Query main table
COMMENT ON TABLE market_ohlcv_buffer IS 'Buffer table for high-frequency candle inserts from WebSocket';
