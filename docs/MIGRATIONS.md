# Database Migrations

This project uses [`golang-migrate`](https://github.com/golang-migrate/migrate) for database schema management.

## Overview

- **Library**: `github.com/golang-migrate/migrate/v4`
- **Auto-migration**: Migrations run automatically when the bot starts
- **Manual control**: Use Makefile commands for manual migration management
- **Migration files**: Located in `./migrations/` directory

## Migration Files Format

Migration files follow the naming convention: `{version}_{description}.{up|down}.sql`

Example:
```
000001_init.up.sql                     # Initial schema
000001_init.down.sql                   # Rollback initial schema
000002_news_cache.up.sql               # News caching system
000002_news_cache.down.sql             # Remove news cache
000003_sentiment_tracking.up.sql       # Sentiment tracking
000003_sentiment_tracking.down.sql     # Remove sentiment tracking
000004_onchain_monitoring.up.sql       # On-chain monitoring
000004_onchain_monitoring.down.sql     # Remove on-chain monitoring
```

## Current Migrations

### 000001 - Initial Schema
Core trading bot tables:
- `users` - User accounts
- `user_configs` - Trading configurations per user
- `user_states` - Current bot state per user
- `trades` - Trade history
- `positions` - Open/closed positions
- `ai_decisions` - AI decision tracking
- `risk_events` - Risk management events
- `performance_metrics` - Daily performance snapshots

### 000002 - News Cache
News aggregation system:
- `news_items` - Cached news with sentiment analysis
- `recent_news` view - Last 24h relevant news
- `cleanup_old_news()` function - Auto-cleanup (7 days retention)

### 000003 - Sentiment Tracking
Advanced sentiment analysis:
- `sentiment_snapshots` - Historical sentiment (5-min intervals)
- `sentiment_trends` view - Hourly trends
- `high_impact_news` view - Market-moving events (impact ≥ 7)
- `get_current_sentiment()` function - Real-time aggregated sentiment

### 000004 - On-Chain Monitoring
Blockchain activity tracking:
- `whale_transactions` - Large transfers (>$100k)
- `exchange_flows` - Exchange inflow/outflow tracking
- `onchain_metrics` - Network activity metrics
- Views: `recent_whale_activity`, `exchange_flow_summary`, `onchain_alerts`

## Automatic Migrations

Migrations run automatically when the bot starts (see `cmd/bot/main.go`):

```go
// Run database migrations
migrationsPath := "./migrations"
if err := database.RunMigrations(db.Conn(), migrationsPath); err != nil {
    return fmt.Errorf("failed to run migrations: %w", err)
}
```

This ensures your database schema is always up to date.

## Database Setup

### Quick Start (Automated)

Create database and run migrations in one command:

```bash
# Full setup (creates DB + runs migrations)
make db-setup

# Or use the full development setup
make setup  # Creates DB, runs migrations, sets up .env
```

### Manual Database Creation

If you prefer manual control:

```bash
# Create database
make db-create

# Create test database
make db-create-test

# Run migrations
make migrate-up
```

### Custom PostgreSQL Configuration

Override default database settings:

```bash
# Custom database name
DB_NAME="my_trader" make db-create

# Custom user and host
DB_USER="postgres" DB_HOST="192.168.1.100" make db-setup

# With password
DB_USER="trader" DB_PASSWORD="secret123" DB_NAME="trader_prod" make db-setup
```

### Reset Database

**⚠️ Warning: This deletes all data!**

```bash
# Drop and recreate database with fresh migrations
make db-reset
```

## Manual Migration Management

### Install CLI Tool (Optional)

Install the official `golang-migrate` CLI for advanced usage:

```bash
make install-migrate
# or
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
```

### Basic Commands

```bash
# Run all pending migrations
make migrate-up
# or
make migrate

# Rollback the last migration
make migrate-down

# Check current migration version
make migrate-version

# Create new migration files
make migrate-create
# Example: creates 000002_add_feature.up.sql and 000002_add_feature.down.sql

# Force specific version (use with caution!)
make migrate-force
```

### Custom Database URL

Override the default database URL:

```bash
# Default: postgres://localhost:5432/trader?sslmode=disable
DB_URL="postgres://user:pass@host:5432/dbname?sslmode=disable" make migrate-up
```

### Using CLI Directly

If you installed the CLI tool:

```bash
# Run migrations
migrate -path=./migrations -database "postgres://localhost:5432/trader?sslmode=disable" up

# Rollback one migration
migrate -path=./migrations -database "postgres://localhost:5432/trader?sslmode=disable" down 1

# Check version
migrate -path=./migrations -database "postgres://localhost:5432/trader?sslmode=disable" version

# Force version (if dirty state)
migrate -path=./migrations -database "postgres://localhost:5432/trader?sslmode=disable" force 1
```

## Creating New Migrations

### Using Makefile

```bash
make migrate-create
# Enter name when prompted: add_user_settings
```

This creates:
- `migrations/000002_add_user_settings.up.sql`
- `migrations/000002_add_user_settings.down.sql`

### Using CLI

```bash
migrate create -ext sql -dir ./migrations -seq add_user_settings
```

### Migration Best Practices

**UP Migration** (`*.up.sql`):
```sql
-- Add new feature
CREATE TABLE user_settings (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    theme VARCHAR(20) DEFAULT 'dark',
    notifications_enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_user_settings_user_id ON user_settings(user_id);
```

**DOWN Migration** (`*.down.sql`):
```sql
-- Rollback feature
DROP TABLE IF EXISTS user_settings;
```

### Guidelines

1. **Always create both UP and DOWN migrations**
2. **Test migrations in development first**
3. **Keep migrations small and focused**
4. **Never modify existing migrations after deployment**
5. **Use transactions when possible** (Postgres supports DDL transactions)
6. **Add indexes separately** if table has data
7. **Backup production data** before complex migrations

## Troubleshooting

### Dirty Migration State

If a migration fails halfway, the database enters a "dirty" state:

```bash
# Check current version
make migrate-version
# Output: 2/d (dirty)

# Fix by forcing to the last working version
make migrate-force
# Enter: 1

# Then retry
make migrate-up
```

### Migration Failed

```bash
# Rollback to previous version
make migrate-down

# Fix the migration file
vim migrations/000002_feature.up.sql

# Try again
make migrate-up
```

### Manual Fix Required

If automatic fixes don't work:

```sql
-- Connect to database
psql -U trader -d trader

-- Check migration table
SELECT * FROM schema_migrations;

-- Manual cleanup if needed
DELETE FROM schema_migrations WHERE version = 2 AND dirty = true;
```

## Testing Migrations

### Integration Tests

The test database helper (`test/testdb/helper.go`) automatically runs migrations for tests:

```go
func TestSomething(t *testing.T) {
    db := testdb.Setup(t) // Migrations run automatically
    defer db.Teardown(t)
    
    // Your test code
}
```

### Manual Testing

```bash
# Start test database
docker-compose -f docker-compose.test.yml up -d

# Run migrations on test DB
DB_URL="postgres://trader:trader@localhost:5433/trader_test?sslmode=disable" make migrate-up

# Run tests
make test-db

# Cleanup
docker-compose -f docker-compose.test.yml down -v
```

## Production Deployment

### Option 1: Automatic (Recommended)

Migrations run automatically when the bot starts. Just deploy and start:

```bash
./bin/bot
```

### Option 2: Manual Before Deploy

Run migrations manually before deploying new code:

```bash
# 1. Backup database
pg_dump -U trader trader > backup_$(date +%Y%m%d_%H%M%S).sql

# 2. Run migrations
DB_URL="postgres://production-host:5432/trader?sslmode=disable" make migrate-up

# 3. Deploy new code
./deploy.sh
```

## Migration Functions

The `internal/adapters/database/migrate.go` provides:

- `RunMigrations(db, path)` - Run all pending migrations
- `RollbackMigration(db, path)` - Rollback last migration
- `GetMigrationVersion(db, path)` - Get current version and dirty state

## References

- [golang-migrate Documentation](https://github.com/golang-migrate/migrate)
- [Best Practices](https://github.com/golang-migrate/migrate/blob/master/MIGRATIONS.md)
- [Database URLs](https://github.com/golang-migrate/migrate#database-urls)

