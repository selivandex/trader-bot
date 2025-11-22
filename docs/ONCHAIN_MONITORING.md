# On-Chain Monitoring

The bot monitors blockchain activity to detect whale movements and exchange flows - critical leading indicators for price action.

## What We Track

### 1. Whale Transactions (Large Movements)

**Definition**: Transactions ≥ $1M USD

**Types:**
- **exchange_inflow**: BTC/ETH moving TO exchange → Potential selling (bearish)
- **exchange_outflow**: BTC/ETH moving FROM exchange → Accumulation (bullish)
- **whale_movement**: Unknown wallet to unknown wallet → Monitoring

**Impact Scoring:**
```
$100M+  = 10/10 (market-moving)
$50M+   = 9/10  (highly significant)
$10M+   = 8/10  (very significant)
$5M+    = 7/10  (significant)
$1M+    = 6/10  (notable)
```

### 2. Exchange Flows

**Net Flow Calculation:**
```
Net Flow = Exchange Inflow - Exchange Outflow (last 6 hours)

> +50 BTC  = INFLOW  (bearish - likely selling)
< -50 BTC  = OUTFLOW (bullish - accumulation)
-50 to +50 = BALANCED (neutral)
```

### 3. High Impact Alerts

Automatic alerts for:
- Whale movements ≥ $10M
- Exchange inflows > 100 BTC in 1 hour
- Unusual activity patterns

## Data Sources

### Whale Alert API

**Setup:**
```bash
WHALE_ALERT_API_KEY=your_api_key
ONCHAIN_MIN_VALUE_USD=1000000
```

Get API key: https://whale-alert.io

**Features:**
- Real-time whale tracking
- Exchange labeling (Binance, Coinbase, etc.)
- Multi-blockchain support (BTC, ETH, USDT, etc.)

**Pricing:**
- Free tier: 10 requests/hour
- Starter: $30/month (unlimited)

## Architecture

### Background Worker (every 15 minutes)

```
OnChainWorker
    ↓
Fetch from Whale Alert API
    ↓
Calculate Impact Scores
    ↓
Save to whale_transactions table
    ↓
Log high impact events
```

### Database Tables

**whale_transactions:**
```sql
- tx_hash (unique)
- symbol, amount, amount_usd
- from_owner → to_owner
- transaction_type
- impact_score (1-10)
- timestamp
```

**exchange_flows:**
```sql
- exchange, symbol
- inflow, outflow, net_flow
- timestamp (hourly aggregation)
```

**onchain_metrics:**
```sql
- active_addresses
- transaction_count
- exchange_reserve
- large_tx_count
```

## Usage in Trading Decisions

### In AI Prompt

```
=== ON-CHAIN SIGNALS ===

Whale Activity: HIGH
Exchange Flow: INFLOW (120.5 BTC net)
⚠️ Large inflow to exchanges = potential selling pressure

Recent Whale Movements:
⚠️ exchange_inflow: $15.2M unknown → Binance (12 min ago)
📉 exchange_inflow: $8.5M unknown → Coinbase (45 min ago)
📈 exchange_outflow: $5.1M Kraken → unknown (2h ago)
```

### Impact on Decisions

**Scenario 1: Bullish + Exchange Outflow**
```
Technical: RSI 60, MACD bullish
News: Neutral (0.1)
On-Chain: 200 BTC outflow (last 6h) ← STRONG BULLISH
→ AI Decision: OPEN_LONG (confidence 85%)
```

**Scenario 2: Bullish but Large Exchange Inflow**
```
Technical: RSI 65, MACD bullish
News: Bullish (0.4)
On-Chain: 500 BTC inflow + $50M whale to Binance ← WARNING
→ AI Decision: HOLD (confidence 45%, whale might dump)
```

**Scenario 3: Whale Accumulation Signal**
```
Technical: RSI 45, slight downtrend
News: Neutral
On-Chain: 1000 BTC outflow, multiple whales accumulating ← VERY BULLISH
→ AI Decision: OPEN_LONG (confidence 80%, whales know something)
```

## Monitoring

### SQL Queries

Recent whale activity:
```sql
SELECT * FROM recent_whale_activity WHERE symbol = 'BTC';
```

Exchange flow summary:
```sql
SELECT * FROM exchange_flow_summary WHERE symbol = 'BTC';
```

High impact alerts:
```sql
SELECT * FROM onchain_alerts WHERE symbol = 'BTC' LIMIT 10;
```

Whale transactions (last 24h):
```sql
SELECT 
    transaction_type,
    from_owner,
    to_owner,
    amount_usd,
    impact_score,
    timestamp
FROM whale_transactions
WHERE symbol = 'BTC' 
  AND timestamp > NOW() - INTERVAL '24 hours'
ORDER BY impact_score DESC, timestamp DESC;
```

### Logs

High impact transactions are logged:
```
2024-11-22 14:30:15 WARN HIGH IMPACT whale transaction
  type=exchange_inflow symbol=BTC amount_usd=15200000
  from=unknown to=binance impact=8
```

## Telegram Alerts

Future enhancement - команда для on-chain мониторинга:

```
/onchain
Bot: 🔍 On-Chain Activity (BTC)

     Whale Activity: HIGH ⚠️
     Exchange Flow: INFLOW (120 BTC) 📉
     
     Recent High Impact:
     1. $15M → Binance (12 min ago)
     2. $8M → Coinbase (45 min ago)
     3. $5M Kraken → Unknown (2h ago)
     
     ⚠️ Warning: Large exchange inflow detected
     Consider this in trading decisions.
```

## Configuration

### Environment Variables

```bash
# Enable on-chain monitoring
ONCHAIN_ENABLED=true

# Whale Alert API key
WHALE_ALERT_API_KEY=your_whale_alert_api_key

# Minimum transaction size to track (in USD)
ONCHAIN_MIN_VALUE_USD=1000000  # $1M minimum
```

### Adjust Sensitivity

Higher threshold (less noise):
```bash
ONCHAIN_MIN_VALUE_USD=5000000  # Only $5M+ transactions
```

Lower threshold (more data):
```bash
ONCHAIN_MIN_VALUE_USD=500000   # $500k+ transactions
```

## Best Practices

1. **Combine with Technical Analysis**: On-chain is a leading indicator, not standalone signal
2. **Context Matters**: 100 BTC inflow is normal for Binance, unusual for smaller exchange
3. **Timing**: Whale movements can take hours to impact price
4. **False Positives**: Not all inflows = selling (could be arbitrage, market making)
5. **Weight**: Use 20-30% weight for on-chain in final decision

## Cost Considerations

- **Whale Alert Free**: 10 requests/hour (1 request every 6 minutes)
- **Whale Alert Paid**: $30/month (unlimited, recommended)
- **Worker runs every 15 minutes** = 4 requests/hour = within free tier

## Future Enhancements

- [ ] Direct blockchain node monitoring (no API needed)
- [ ] Exchange reserve tracking (Glassnode)
- [ ] MVRV ratio (market value / realized value)
- [ ] SOPR (Spent Output Profit Ratio)
- [ ] Exchange netflow MA (moving average)
- [ ] Miner flows tracking
- [ ] Stablecoin flows (USDT, USDC movements)

## Example Impact

Real scenarios where on-chain saved from losses:

**Case 1: Tesla BTC Sale (2022)**
- On-chain detected: 10,000+ BTC Teslalet → exchange
- 2 hours before announcement
- Bot avoided long position → saved from -10% dump

**Case 2: MicroStrategy Buy (2023)**  
- Multiple large OTC purchases detected
- Exchange outflows accelerating
- Bot opened long → caught +15% rally

On-chain data gives you **information advantage** that price action doesn't show yet.

