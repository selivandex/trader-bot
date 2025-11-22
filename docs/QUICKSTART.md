# Quick Start Guide

## Prerequisites

- Go 1.21+
- PostgreSQL 15+
- Docker & Docker Compose (optional)
- Telegram Bot Token
- Exchange API keys (Binance or Bybit testnet)
- AI API keys (DeepSeek or Claude)

## 1. Database Setup

### Option A: Docker (Recommended)

```bash
docker-compose up -d postgres
```

### Option B: Local PostgreSQL

```bash
createdb trader
psql trader < migrations/001_init.sql
```

## 2. Configuration

Copy environment template:

```bash
cp env.example .env
```

Edit `.env` with your credentials:

```bash
# Minimum required
TELEGRAM_BOT_TOKEN=your_bot_token
TELEGRAM_CHAT_ID=your_chat_id

DEEPSEEK_API_KEY=your_deepseek_key
DEEPSEEK_ENABLED=true

CLAUDE_API_KEY=your_claude_key
CLAUDE_ENABLED=true

DB_USER=trader
DB_PASSWORD=trader
```

## 3. Install Dependencies

```bash
go mod download
go mod tidy
```

## 4. Build

```bash
make build
```

## 5. Run

### Using Docker (Recommended)

```bash
docker-compose up -d
docker-compose logs -f bot
```

### Running Locally

```bash
./bin/bot
```

Or directly:

```bash
go run cmd/bot/main.go
```

## 6. Setup via Telegram

Open your Telegram bot and:

### Register
```
/start
```

### Connect Exchange (Use testnet first!)
```
/connect binance YOUR_API_KEY YOUR_SECRET true
```

### Add Trading Pair
```
/addpair BTC/USDT 1000
```

### Start Trading
```
/start_trading
```

### Monitor
```
/status
/listpairs
```

## Testing First!

⚠️ **IMPORTANT**: Test before real money!

### 1. Paper Trading

Set `MODE=paper` in `.env` and run bot for at least 1 month.

### 2. Monitor Results

```sql
psql trader -c "SELECT * FROM user_overview;"
```

Check:
- Win rate > 50%
- ROI > 10%
- Max drawdown < 15%

## Telegram Commands Reference

### Setup
- `/start` - Register
- `/connect <exchange> <key> <secret>` - Connect exchange
- `/addpair <symbol> <balance>` - Add pair (e.g., /addpair ETH/USDT 500)
- `/listpairs` - Show all pairs
- `/removepair <symbol>` - Remove pair

### Trading
- `/start_trading` - Start all pairs
- `/start_trading BTC/USDT` - Start specific pair
- `/stop_trading` - Stop all
- `/stop_trading BTC/USDT` - Stop specific pair

### Monitoring
- `/status` - Status of all pairs
- `/status BTC/USDT` - Status of specific pair
- `/mystats` - Performance statistics
- `/config` - View all configurations

## Common Issues

**Bot won't start:**
```bash
# Check logs
docker-compose logs bot

# Check database
docker-compose logs postgres
```

**No trades executing:**
- Verify `/status` shows "Running"
- Check AI API keys are valid
- Ensure balance is set correctly
- Check circuit breaker: look for consecutive losses

**Database connection failed:**
```bash
# Recreate database
docker-compose down -v
docker-compose up -d
```

## Monitoring

### View Logs
```bash
tail -f logs/bot.log
```

### Database Queries

Active users:
```sql
SELECT * FROM user_overview WHERE is_trading = true;
```

Recent trades:
```sql
SELECT * FROM recent_trades_by_user LIMIT 20;
```

Performance:
```sql
SELECT username, symbol, balance, equity, daily_pnl
FROM user_overview
ORDER BY equity DESC;
```

## Stopping

```bash
# Docker
docker-compose down

# Or just Ctrl+C if running locally
```

## Next Steps

1. ✅ Test with paper trading (1 month minimum)
2. ✅ Analyze results (`/mystats`)
3. ✅ Adjust configuration if needed
4. ⚠️ Switch to live trading only after consistent profits
5. 💰 Start with small balance ($100-$200)

## Security

- ✅ Never commit `.env` file
- ✅ Use API keys with trading permissions only
- ✅ Start with testnet
- ✅ Set up withdrawal limits on exchange
- ✅ Monitor daily via Telegram alerts

## Support

- Check logs: `logs/bot.log`
- Database issues: See `test/README.md`
- Bot issues: See `docs/MULTI_USER_SETUP.md`

