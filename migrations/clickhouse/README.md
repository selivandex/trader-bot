# ClickHouse Migrations

## Setup

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

---

## Connection Strings

### Native Protocol (9000)
```
clickhouse://localhost:9000/trader
```

### HTTP Interface (8123)
```
http://localhost:8123/?database=trader
```

---

## Go Driver

```bash
go get github.com/ClickHouse/clickhouse-go/v2
```

**Example:**
```go
import (
    "database/sql"
    _ "github.com/ClickHouse/clickhouse-go/v2"
)

db, err := sql.Open("clickhouse", "clickhouse://localhost:9000/trader")
```

---

## Testing Connection

```bash
clickhouse-client --query "SELECT version()"
```

---

## Useful Commands

### Show Tables
```sql
SHOW TABLES FROM trader;
```

### Table Info
```sql
DESCRIBE TABLE market_ohlcv;
```

### Check Data
```sql
SELECT count(*) FROM market_ohlcv;
```

### Partition Info
```sql
SELECT 
    partition,
    name,
    rows,
    bytes_on_disk
FROM system.parts
WHERE table = 'market_ohlcv' AND active
ORDER BY partition DESC;
```

### Drop All Tables (Be Careful!)
```sql
DROP DATABASE trader;
CREATE DATABASE trader;
```

---

## Performance Tips

1. **Use PARTITION BY** - ClickHouse automatically prunes partitions
2. **ORDER BY matters** - First columns should be used in WHERE clauses
3. **Batch inserts** - Insert 1000-10000 rows at once
4. **Use LowCardinality** - For columns with < 10K unique values
5. **Materialized views** - Pre-aggregate common queries

---

## Backup

```bash
# Backup database
clickhouse-client --query "BACKUP DATABASE trader TO Disk('backups', 'trader_backup.zip')"

# Restore
clickhouse-client --query "RESTORE DATABASE trader FROM Disk('backups', 'trader_backup.zip')"
```

