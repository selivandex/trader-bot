# Quick Start Guide

Get your trading bot up and running in 5 minutes! ⚡

## Prerequisites

- Go 1.21+ installed
- PostgreSQL running locally (or remote)
- Exchange API keys (Binance/Bybit)
- AI API keys (DeepSeek/Claude/OpenAI)
- Telegram bot token

## Step 1: Clone & Install

```bash
git clone https://github.com/alexanderselivanov/trader.git
cd trader
go mod download
```

## Step 2: One-Command Setup

```bash
# Creates database, runs migrations, sets up .env
make setup
```

**Alternative**: If PostgreSQL needs custom credentials:

```bash
DB_USER="postgres" DB_PASSWORD="mypass" make setup
```

## Step 3: Configure API Keys

Edit `.env` file with your credentials:

```bash
vim .env
```

Required settings:
```env
# Database
DATABASE_HOST=localhost
DATABASE_PORT=5432
DATABASE_USER=your_username
DATABASE_NAME=trader
DATABASE_PASSWORD=your_password

# Telegram
TELEGRAM_BOT_TOKEN=your_bot_token
TELEGRAM_ADMIN_ID=your_telegram_id

# Exchange (choose one)
BINANCE_API_KEY=your_api_key
BINANCE_API_SECRET=your_api_secret
BINANCE_TESTNET=true

# AI Provider (at least one required)
DEEPSEEK_API_KEY=your_api_key
DEEPSEEK_ENABLED=true
```

## Step 4: Build & Run

```bash
# Build
make build

# Run in paper trading mode (safe!)
make paper

# Or run normally
make run
```

## Step 5: Telegram Commands

Open your Telegram bot and start trading:

```
/start              - Register as new user
/help               - Show all commands
/addpair BTC/USDT   - Add trading pair
/starttrading       - Start bot
/balance            - Check balance
/status             - Check status
```

## Common Use Cases

### First Time Setup
```bash
# Full automated setup
make setup
make build
make paper
```

### Reset Everything
```bash
# Drop database and start fresh
make db-reset
```

### Custom Database
```bash
# Use different database name
DB_NAME="trader_prod" make db-setup

# Remote PostgreSQL
DB_HOST="192.168.1.100" DB_USER="trader" DB_PASSWORD="secret" make db-setup
```

## Troubleshooting

### Database Connection Failed

```bash
# Check PostgreSQL is running
psql -l

# Recreate database
make db-reset
```

### Migrations Failed

```bash
# Check migration status
make migrate-version

# Manual migration
make migrate-up

# Force fix if dirty
make migrate-force
# Enter: 1 (or last working version)
```

### No PostgreSQL Installed

**macOS:**
```bash
brew install postgresql@15
brew services start postgresql@15
```

**Ubuntu/Debian:**
```bash
sudo apt install postgresql postgresql-contrib
sudo systemctl start postgresql
```

**Docker (quick alternative):**
```bash
docker run -d \
  --name trader-postgres \
  -e POSTGRES_USER=trader \
  -e POSTGRES_PASSWORD=trader \
  -e POSTGRES_DB=trader \
  -p 5432:5432 \
  postgres:15-alpine

# Then use:
DB_USER=trader DB_PASSWORD=trader make setup
```

## Architecture Quick Overview

```
┌─────────────────┐
│   Telegram Bot  │ ← You control everything here
└────────┬────────┘
         │
    ┌────▼────┐
    │   Bot   │ ← Main trading engine
    │ Manager │
    └────┬────┘
         │
    ┌────▼────────────────┐
    │  Trading Strategy   │
    │  + AI Decisions     │ ← Multiple AI providers
    │  + Risk Management  │
    └────┬────────────────┘
         │
    ┌────▼──────┐   ┌──────────┐
    │ Exchange  │   │ Database │
    │  (CCXT)   │   │   (PG)   │
    └───────────┘   └──────────┘
```

## Next Steps

1. **Read documentation**: 
   - [Migrations Guide](MIGRATIONS.md)
   - [Multi-User Setup](MULTI_USER_SETUP.md)
   - [News Integration](NEWS_INTEGRATION.md)

2. **Run tests**:
   ```bash
   make test          # Unit tests
   make test-db       # Integration tests
   ```

3. **Start with paper trading**:
   ```bash
   make paper
   ```

4. **Go live** (when ready):
   ```bash
   # Update .env: BINANCE_TESTNET=false
   make run
   ```

## Help Commands

```bash
# Show all available commands
make help

# Or just:
make
```

## Support

- Check logs: `tail -f logs/bot.log`
- Database issues: See [MIGRATIONS.md](MIGRATIONS.md)
- Configuration: See [README.md](../README.md)

## Security Notes

⚠️ **Important**:
- Start with testnet/paper trading
- Never commit `.env` file
- Use read-only API keys for testing
- Start with small amounts

Happy Trading! 🚀



