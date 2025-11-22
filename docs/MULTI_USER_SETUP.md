<!-- @format -->

# Multi-User Trading Bot Setup

The bot now supports multiple users, each with their own independent trading configuration and bot instance.

## Architecture

```
Telegram Bot (Single Instance)
    ↓
Bot Manager
    ↓
├─ User 1 Bot → Binance → BTC/USDT → $1000
├─ User 2 Bot → Bybit → ETH/USDT → $500
├─ User 3 Bot → Binance → BTC/USDT → $2000
└─ ...
```

Each user has:

- **Separate exchange connection** (Binance or Bybit)
- **Independent trading pair** (BTC/USDT, ETH/USDT, etc.)
- **Own balance and equity tracking**
- **Individual risk management**
- **Isolated trading history**

## Quick Start for Users

### 1. Register

```
/start
```

This creates your user account in the system.

### 2. Connect Exchange

```
/connect binance YOUR_API_KEY YOUR_SECRET true
```

- **Exchange**: `binance` or `bybit`
- **API Key**: Your exchange API key
- **Secret**: Your exchange API secret
- **Testnet**: `true` for testnet, `false` for live trading

⚠️ **Important**:

- Start with testnet=true to test safely
- Never share your API keys
- Use API keys with trading permissions only

### 3. Add Trading Pairs

You can trade **multiple pairs simultaneously**!

Add first pair:
```
/addpair BTC/USDT 1000
```

Add more pairs:
```
/addpair ETH/USDT 500
/addpair SOL/USDT 300
```

View all pairs:
```
/listpairs
```

### 4. Start Trading

Start all pairs:
```
/start_trading
```

Or start specific pair:
```
/start_trading BTC/USDT
```

Each pair runs as independent bot with isolated balance!

### 5. Monitor & Control

Check status:

```
/status
```

View statistics:

```
/mystats
```

Check current position:

```
/position
```

Stop trading:

```
/stop_trading
```

## User Commands

### Setup Commands

- `/start` - Register in the system
- `/connect <exchange> <api_key> <secret> [testnet]` - Connect exchange
- `/addpair <symbol> <balance> [exchange]` - Add trading pair
- `/listpairs` - Show all your pairs
- `/removepair <symbol>` - Remove trading pair

### Trading Commands

- `/start_trading [symbol]` - Start trading (all pairs or specific)
- `/stop_trading [symbol]` - Stop trading (all pairs or specific)

### Info Commands

- `/status [symbol]` - Check status (all pairs or specific)
- `/config [symbol]` - View configuration
- `/mystats` - View your trading statistics
- `/position [symbol]` - See current position
- `/help` - Show help message

## Example Setup Flow - Single Pair

```
User: /start
Bot: 👋 Welcome! You've been registered.

User: /connect binance abc123key xyz789secret true
Bot: ✅ Exchange Connected

User: /addpair BTC/USDT 1000
Bot: ✅ Trading Pair Added
     Symbol: BTC/USDT
     Exchange: binance
     Balance: $1000.00

User: /start_trading
Bot: 🚀 Trading Started: BTC/USDT

User: /status
Bot: 📊 Your Trading Pairs (1)
     1. BTC/USDT 🟢 Running
        Balance: $1000.00 → Equity: $1050.00
        PnL: $50.00 (5.00%)
```

## Example Setup Flow - Multiple Pairs

```
User: /connect binance abc123key xyz789secret true
Bot: ✅ Exchange Connected

User: /addpair BTC/USDT 1000
Bot: ✅ Trading Pair Added: BTC/USDT

User: /addpair ETH/USDT 500
Bot: ✅ Trading Pair Added: ETH/USDT

User: /addpair SOL/USDT 300
Bot: ✅ Trading Pair Added: SOL/USDT

User: /listpairs
Bot: 📊 Your Trading Pairs (3)
     1. BTC/USDT 🔴 Stopped
        Balance: $1000.00
     2. ETH/USDT 🔴 Stopped
        Balance: $500.00
     3. SOL/USDT 🔴 Stopped
        Balance: $300.00

User: /start_trading
Bot: 🚀 Trading Started
     3 pairs now trading.

User: /status
Bot: 📊 Your Trading Pairs (3)
     1. BTC/USDT 🟢 Running
        Equity: $1050.00 | PnL: $50.00 (5.00%)
     2. ETH/USDT 🟢 Running
        Equity: $480.00 | PnL: -$20.00 (-4.00%)
     3. SOL/USDT 🟢 Running
        Equity: $315.00 | PnL: $15.00 (5.00%)

User: /stop_trading ETH/USDT
Bot: ⏸️ Trading Stopped: ETH/USDT

User: /removepair SOL/USDT
Bot: ⚠️ Cannot remove SOL/USDT while trading is active.
     Use /stop_trading SOL/USDT first.
```

## For Administrators

### Database Tables

- **users** - Registered users
- **user_configs** - Per-user exchange credentials and settings
- **user_states** - Current balance, equity, PnL for each user
- **trades** - All trades with user_id
- **positions** - Open positions per user
- **ai_decisions** - AI decisions per user

### View All Users

```sql
SELECT * FROM user_overview;
```

### Monitor Active Trading

```sql
SELECT
    u.username,
    uc.exchange,
    uc.symbol,
    us.equity,
    us.daily_pnl,
    uc.is_trading
FROM users u
JOIN user_configs uc ON u.id = uc.user_id
JOIN user_states us ON u.id = us.user_id
WHERE uc.is_trading = true;
```

### Performance by User

```sql
SELECT
    u.username,
    COUNT(t.id) as total_trades,
    SUM(t.pnl) as total_pnl,
    AVG(t.pnl) as avg_pnl
FROM users u
JOIN trades t ON u.id = t.user_id
GROUP BY u.username
ORDER BY total_pnl DESC;
```

## Security Considerations

1. **API Keys**: Stored in database (consider encryption for production)
2. **Isolation**: Each user's bot runs independently
3. **Rate Limits**: Consider exchange API rate limits when many users trade
4. **Resource Management**: Monitor server resources with many active bots

## Scaling

The bot manager can handle multiple users on a single server:

- **Small Scale**: 1-10 users on 2GB RAM server
- **Medium Scale**: 10-50 users on 4GB RAM server
- **Large Scale**: 50+ users - consider horizontal scaling

Each user bot runs in its own goroutine with isolated context.

## Troubleshooting

**Bot won't start:**

- Check API keys are correct
- Verify exchange is supported (binance/bybit)
- Ensure testnet setting matches your API keys

**No trades executing:**

- Check `/status` - bot must be running
- Verify balance is set correctly
- Check circuit breaker hasn't triggered

**Can't connect:**

- Ensure database is running
- Check Telegram bot token is valid
- Verify network connectivity to exchanges

## Migration from Single-User

If you were running the old single-user version:

1. Run migration: `psql trader < migrations/001_init.sql`
2. Register yourself: `/start` in Telegram
3. Setup your config: `/connect ...`
4. Start trading: `/start_trading`

Old bot_state table is replaced with user_states per user.

## Support

For issues, check:

- Logs: `logs/bot.log`
- Database: Check user_overview view
- Telegram: Commands respond with error messages

Contact administrator if bot is not responding.
