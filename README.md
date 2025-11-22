# AI Trading Bot

An automated cryptocurrency trading bot for futures markets using AI models (DeepSeek, Claude, GPT) for decision-making. Built with Go and CCXT.

## Features

- **Multi-User Support**: Each user gets their own independent trading bot
- **Multi-Exchange Support**: Binance and Bybit futures trading
- **AI-Powered Decisions**: Ensemble approach using multiple AI providers (DeepSeek, Claude, GPT)
- **News & Sentiment Analysis**: Integrates Twitter, Forklog, and crypto news sources
- **Risk Management**: Circuit breaker, position sizing, stop-loss/take-profit per user
- **Telegram Control**: Full bot management through Telegram commands
- **Paper Trading**: Simulate trading before going live
- **PostgreSQL Storage**: Track all trades, decisions, and performance metrics per user

## Architecture

```
trader/
├── cmd/
│   └── bot/              # Main trading bot
├── internal/
│   ├── exchange/         # CCXT exchange adapters
│   ├── ai/               # AI provider clients
│   ├── strategy/         # Trading logic
│   ├── indicators/       # Technical indicators (RSI, MACD, BB)
│   ├── risk/             # Risk management
│   ├── portfolio/        # Balance tracking
│   └── telegram/         # Telegram bot
├── pkg/
│   ├── models/           # Data structures
│   └── logger/           # Logging utilities
└── configs/              # Configuration files
```

## Prerequisites

- Go 1.21+
- PostgreSQL 14+
- Exchange API keys (Binance/Bybit)
- AI API keys (DeepSeek/Claude/OpenAI)
- Telegram bot token

## Installation

1. Clone the repository:
```bash
git clone https://github.com/selivandex/trader-bot.git
cd trader
```

2. Install dependencies:
```bash
go mod download
```

3. Setup database and environment:
```bash
# Automated setup (creates DB, runs migrations, creates .env)
make setup

# Or manual setup:
make db-create      # Create database
make migrate-up     # Run migrations
cp env.example .env # Copy config template
```

> **Note**: Database migrations run automatically when the bot starts. See [docs/MIGRATIONS.md](docs/MIGRATIONS.md) for details.

**Custom PostgreSQL settings:**
```bash
# If your PostgreSQL uses different credentials:
DB_USER="postgres" DB_PASSWORD="mypass" make db-setup
```

4. Configure environment:
```bash
cp .env.example .env
# Edit .env with your API keys
```

5. Build:
```bash
go build -o bin/bot cmd/bot/main.go
```

## Usage

### Paper Trading (Recommended Start)

```bash
./bin/bot
```

The bot starts in paper trading mode by default (see `configs/config.yaml`).

### Telegram Commands

- `/status` - Current bot state
- `/balance` - Balance and PnL
- `/position` - Current position
- `/stop` - Stop trading
- `/resume` - Resume trading
- `/stats` - Performance statistics

### Live Trading

**WARNING**: Only after successful paper trading!

Edit `configs/config.yaml`:
```yaml
mode: live
exchanges:
  binance:
    testnet: false
```

## Risk Management

The bot implements multiple safety mechanisms:

- **Circuit Breaker**: Stops trading after 5 consecutive losses or -5% daily loss
- **Position Limits**: Maximum 30% of balance per trade
- **Leverage Cap**: Maximum 3x leverage
- **Stop Loss**: Automatic 2% stop loss on every position
- **Profit Withdrawal**: Auto-withdraws profits above 10% gain

## AI Decision Flow

1. **Collect market data** every 30 minutes:
   - Price, volume, order book
   - Technical indicators (RSI, MACD, Bollinger Bands)
   - Funding rate and open interest
   - **News and sentiment** from multiple sources
2. **Query multiple AI providers** in parallel (DeepSeek, Claude, GPT)
3. **Require consensus** (2 out of 2 agreement) for trade execution
4. **Validate decision** through risk checks
5. **Execute order** with stop-loss and take-profit
6. **Monitor and alert** via Telegram

## News Integration

A **background worker** continuously fetches and caches news from multiple sources:

- **Reddit**: r/CryptoCurrency, r/Bitcoin, r/ethereum (free, no API key)
- **CoinDesk**: Professional crypto journalism (free)
- **Twitter**: Real-time sentiment (optional, requires API key)

**How it works:**
1. News worker runs in background (every 10 minutes)
2. Fetches from all sources → analyzes sentiment → caches to PostgreSQL
3. Trading engines read from cache (instant, <50ms)
4. News sentiment included in AI decision-making

See [News Integration Guide](docs/NEWS_INTEGRATION.md) and [News Flow](docs/NEWS_FLOW.md) for details.

## Cost Estimation

For $1000 initial balance:
- Trading fees: ~$720/month (10 trades/day @ 0.04%)
- AI API costs: ~$86/month (DeepSeek + Claude @ 30min intervals)
- **Total**: ~$806/month

**Recommendation**: Start with larger balance ($3000+) or reduce trading frequency.

## Development

### Running Tests

Unit tests only:
```bash
make test
```

All tests with PostgreSQL:
```bash
make test-db
```

Coverage report:
```bash
make test-coverage
open coverage.html
```

### Running Bot

Build:
```bash
make build
```

Run with verbose logging:
```bash
LOG_LEVEL=debug ./bin/bot
```

Paper trading:
```bash
make paper
```

## Monitoring

All trades and AI decisions are logged to PostgreSQL. Use provided SQL queries to analyze performance:

```sql
-- Win rate
SELECT 
  COUNT(*) FILTER (WHERE pnl > 0) * 100.0 / COUNT(*) as win_rate
FROM trades;

-- AI provider accuracy
SELECT 
  provider,
  COUNT(*) FILTER (WHERE outcome->>'profitable' = 'true') * 100.0 / COUNT(*) as accuracy
FROM ai_decisions
WHERE executed = true;
```

## Disclaimer

**THIS SOFTWARE IS FOR EDUCATIONAL PURPOSES ONLY.**

- Trading cryptocurrencies involves substantial risk of loss
- Past performance does not guarantee future results
- AI models can make incorrect predictions
- Use at your own risk
- Never invest more than you can afford to lose
- Start with paper trading and small amounts

## License

MIT License - see LICENSE file for details

## Support

For issues and questions, please open a GitHub issue.

**Never share your API keys or .env file!**
