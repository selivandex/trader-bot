# ClickHouse Setup Guide

## Quick Start

### 1. Install ClickHouse

**macOS:**
```bash
brew install clickhouse
brew services start clickhouse
```

**Docker:**
```bash
docker run -d \
  --name clickhouse \
  -p 9000:9000 \
  -p 8123:8123 \
  --ulimit nofile=262144:262144 \
  clickhouse/clickhouse-server
```

### 2. Create Database

```bash
clickhouse-client --query "CREATE DATABASE IF NOT EXISTS trader"
```

### 3. Apply Migrations

```bash
clickhouse-client --database trader < migrations/clickhouse/001_initial_schema.sql
```

### 4. Enable in Config

```bash
# In your .env or env file
CH_ENABLED=true
CH_HOST=localhost
CH_PORT=9000
CH_DATABASE=trader
CH_USER=default
CH_PASSWORD=
```

### 5. Start Bot

```bash
make run
```

---

## Architecture

### PostgreSQL (Operational Data)
- ✅ Agent configs, states, memory
- ✅ Active trades, orders, positions  
- ✅ **Vector search for agent memory** (pgvector)
- ✅ Users, exchanges

### ClickHouse (Time-Series Data)
- ✅ OHLCV candles (all timeframes)
- ✅ Trade history (archived)
- ✅ News archive
- ✅ On-chain data
- ✅ Agent performance metrics

---

## Data Flow

```
Bybit WebSocket → BatchWriter → ClickHouse
     (real-time)    (buffers)     (storage)
```

**Buffering:**
- Accumulates records in memory
- Flushes every 10 seconds OR 1000 records
- Prevents slow single-insert performance

**Reading:**
```go
// Agents read candles from ClickHouse
candles, _ := marketRepo.GetCandles(ctx, "BTC/USDT", "5m", 100)
// Returns in milliseconds (very fast!)
```

---

## Verification

### Check Connection

```bash
clickhouse-client --query "SELECT version()"
```

### Check Data

```sql
-- Count candles
SELECT count(*) FROM market_ohlcv;

-- Latest candles
SELECT symbol, timeframe, timestamp, close 
FROM market_ohlcv 
ORDER BY timestamp DESC 
LIMIT 10;

-- Storage size
SELECT 
    table,
    formatReadableSize(sum(bytes_on_disk)) as size
FROM system.parts
WHERE active AND database = 'trader'
GROUP BY table;
```

---

## Performance

**Expected query times:**

- Get 100 candles: **< 10ms**
- Get 10K candles: **< 50ms**  
- Aggregate 1M trades: **< 100ms**
- Complex analytics (7 days): **< 500ms**

**Batch insert:**

- 1000 candles: **< 50ms**
- 10K candles: **< 200ms**

---

## Troubleshooting

### Connection Failed

```bash
# Check if ClickHouse is running
brew services list | grep clickhouse

# Check port
lsof -i :9000
```

### Slow Queries

```sql
-- Check if partitions are working
SELECT partition, rows, bytes_on_disk
FROM system.parts
WHERE table = 'market_ohlcv' AND active
ORDER BY partition DESC;

-- Check if indexes exist
SHOW CREATE TABLE market_ohlcv;
```

### Data Not Appearing

```bash
# Check buffer table
clickhouse-client --query "SELECT count(*) FROM market_ohlcv_buffer"

# Force flush buffer
clickhouse-client --query "OPTIMIZE TABLE market_ohlcv_buffer"
```

---

## Optional: PostgreSQL Fallback

If ClickHouse is not available, bot falls back to PostgreSQL:

```bash
# Disable ClickHouse
CH_ENABLED=false
```

**Note:** PostgreSQL will be slower for large datasets but works fine for testing.

---

## Next Steps

1. Monitor logs for "ClickHouse connection established"
2. Check data is flowing: `SELECT count(*) FROM market_ohlcv`
3. Verify agents can read candles
4. Set up Grafana dashboards (optional)

