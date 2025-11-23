<!-- @format -->

# Advanced Trading Signals - Implementation Roadmap

## Overview

This document outlines advanced trading signals and data sources to enhance AI agent decision-making capabilities. Each section includes technical specifications, API integrations, database schema, and implementation priority.

**Target:** Improve agent win rate from baseline to 60%+ by adding institutional-grade signals.

---

## 1. Asset Correlations & Market Regime Detection

### Why It Matters

- BTC movements predict 70%+ of altcoin behavior
- Traditional market risk-on/risk-off affects crypto
- Correlation breakdown signals regime change

### Implementation

#### New Toolkit Methods

```go
// internal/toolkit/correlations.go

type CorrelationTool struct {
    marketRepo *market.Repository
}

// GetBTCCorrelation returns correlation coefficient [-1, 1] for asset vs BTC
func (t *CorrelationTool) GetBTCCorrelation(symbol string, period string) (float64, error)

// GetMarketRegime detects risk-on vs risk-off based on correlations
func (t *CorrelationTool) GetMarketRegime() (*MarketRegime, error)

// GetBTCDominance returns BTC market cap dominance %
func (t *CorrelationTool) GetBTCDominance() (float64, error)

type MarketRegime struct {
    Regime            string  // "risk_on", "risk_off", "neutral"
    BTCDominance      float64
    AvgCorrelation    float64 // avg correlation across top 20 coins
    VolatilityLevel   string  // "low", "medium", "high"
    Confidence        float64
}
```

#### Database Schema

```sql
-- migrations/000011_correlations.up.sql
CREATE TABLE IF NOT EXISTS asset_correlations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    base_symbol VARCHAR(20) NOT NULL,
    quote_symbol VARCHAR(20) NOT NULL,
    period VARCHAR(10) NOT NULL, -- "1h", "4h", "1d"
    correlation DECIMAL(5,4) NOT NULL, -- -1.0000 to 1.0000
    sample_size INT NOT NULL,
    calculated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_correlations_lookup ON asset_correlations(base_symbol, quote_symbol, period);
CREATE INDEX idx_correlations_time ON asset_correlations(calculated_at DESC);

CREATE TABLE IF NOT EXISTS market_regimes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    regime VARCHAR(20) NOT NULL, -- "risk_on", "risk_off", "neutral"
    btc_dominance DECIMAL(5,2) NOT NULL,
    avg_correlation DECIMAL(5,4) NOT NULL,
    volatility_level VARCHAR(10) NOT NULL,
    confidence DECIMAL(5,4) NOT NULL,
    detected_at TIMESTAMP NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```

#### Worker Implementation

```go
// internal/workers/correlation_worker.go

type CorrelationWorker struct {
    marketRepo *market.Repository
    corrRepo   *correlation.Repository
}

func (w *CorrelationWorker) Start(ctx context.Context) {
    ticker := time.NewTicker(1 * time.Hour)

    for {
        select {
        case <-ticker.C:
            w.calculateCorrelations(ctx)
            w.detectMarketRegime(ctx)
        case <-ctx.Done():
            return
        }
    }
}
```

#### Data Sources

- **Internal:** Use existing OHLCV candles from ClickHouse
- **External (optional):** CoinGecko API for BTC dominance
- **Calculation:** Pearson correlation on returns over rolling window

**Priority:** 🟢 Tier 1 (High Impact, Easy)  
**Effort:** 2 days  
**Dependencies:** Existing market data

---

## 2. Liquidation Levels & Open Interest

### Why It Matters

- Liquidation cascades create predictable price movements
- High OI clusters = potential volatility zones
- Enables "hunt the longs/shorts" strategies

### Implementation

#### API Integration

**Primary Source:** Coinglass API (free tier: 100 req/day)

```go
// internal/adapters/derivatives/coinglass.go

type CoinglassClient struct {
    apiKey string
    client *http.Client
}

type LiquidationHeatmap struct {
    Symbol           string
    LongLevels       []LiquidationLevel
    ShortLevels      []LiquidationLevel
    CurrentPrice     float64
    TotalLongExposure float64
    TotalShortExposure float64
}

type LiquidationLevel struct {
    Price     float64
    Volume    float64 // USD value
    Leverage  float64
    Exchange  string
}

func (c *CoinglassClient) GetLiquidationHeatmap(symbol string) (*LiquidationHeatmap, error)
func (c *CoinglassClient) GetOpenInterest(symbol string) (*OpenInterest, error)

type OpenInterest struct {
    Symbol         string
    TotalOI        float64 // USD value
    Change24h      float64 // percentage
    FundingRate    float64
    LongShortRatio float64
}
```

#### Database Schema

```sql
-- migrations/000012_liquidations.up.sql
CREATE TABLE IF NOT EXISTS liquidation_levels (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    symbol VARCHAR(20) NOT NULL,
    side VARCHAR(10) NOT NULL, -- "long" or "short"
    price DECIMAL(20,8) NOT NULL,
    volume_usd DECIMAL(20,2) NOT NULL,
    leverage DECIMAL(5,2),
    exchange VARCHAR(50),
    snapshot_time TIMESTAMP NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_liquidations_symbol ON liquidation_levels(symbol, snapshot_time DESC);
CREATE INDEX idx_liquidations_price ON liquidation_levels(symbol, price);

CREATE TABLE IF NOT EXISTS open_interest_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    symbol VARCHAR(20) NOT NULL,
    total_oi_usd DECIMAL(20,2) NOT NULL,
    change_24h DECIMAL(10,4),
    funding_rate DECIMAL(10,6),
    long_short_ratio DECIMAL(10,4),
    recorded_at TIMESTAMP NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_oi_history ON open_interest_history(symbol, recorded_at DESC);
```

#### Toolkit Integration

```go
// internal/toolkit/liquidations.go

type LiquidationTool struct {
    derivRepo *derivatives.Repository
}

// GetNearestLiquidationCluster finds closest major liquidation zone
func (t *LiquidationTool) GetNearestLiquidationCluster(symbol string, side string) (*LiquidationCluster, error)

// GetLiquidationRisk calculates risk score based on proximity to clusters
func (t *LiquidationTool) GetLiquidationRisk(symbol string, currentPrice float64) (float64, error)

// GetOpenInterestTrend analyzes OI momentum
func (t *LiquidationTool) GetOpenInterestTrend(symbol string, period string) (*OITrend, error)

type LiquidationCluster struct {
    Side          string  // "long" or "short"
    CenterPrice   float64
    TotalVolume   float64
    Distance      float64 // % from current price
    Significance  float64 // 0-1 score
}

type OITrend struct {
    Symbol         string
    CurrentOI      float64
    Change1h       float64
    Change24h      float64
    Trend          string // "rising", "falling", "stable"
    LongShortRatio float64
    Signal         string // "long_squeeze_risk", "short_squeeze_risk", "neutral"
}
```

#### Worker Implementation

```go
// internal/workers/liquidation_worker.go

type LiquidationWorker struct {
    coinglassClient *derivatives.CoinglassClient
    derivRepo       *derivatives.Repository
    symbols         []string
}

func (w *LiquidationWorker) Start(ctx context.Context) {
    ticker := time.NewTicker(15 * time.Minute) // API rate limits

    for {
        select {
        case <-ticker.C:
            w.fetchLiquidationData(ctx)
        case <-ctx.Done():
            return
        }
    }
}
```

**Priority:** 🟢 Tier 1 (High Impact, Medium effort)  
**Effort:** 3-4 days  
**Cost:** Free tier sufficient for 10-20 symbols  
**API Docs:** https://www.coinglass.com/api

---

## 3. Market Microstructure & Liquidity Metrics

### Why It Matters

- Wide spreads = poor execution, especially for scalpers
- Order book imbalance predicts short-term moves
- Trade flow toxicity detects informed trading

### Implementation

#### Toolkit Extension

```go
// internal/toolkit/microstructure.go

type MicrostructureTool struct {
    exchangeClient *exchange.Client
}

type LiquiditySnapshot struct {
    Symbol            string
    BidAskSpread      float64 // in basis points
    BidAskSpreadPct   float64 // percentage
    OrderBookRatio    float64 // bid_volume / ask_volume
    MarketDepth       *MarketDepth
    TradeFlow         *TradeFlowAnalysis
    LiquidityScore    float64 // 0-100 composite score
}

type MarketDepth struct {
    BidVolume1Pct  float64 // volume within 1% of mid
    AskVolume1Pct  float64
    BidVolume5Pct  float64
    AskVolume5Pct  float64
    Imbalance      float64 // (bid - ask) / (bid + ask)
}

type TradeFlowAnalysis struct {
    BuyerInitiated   float64 // volume last 5 min
    SellerInitiated  float64
    FlowDelta        float64 // buy - sell
    ToxicityScore    float64 // 0-1, higher = more informed flow
    LargeTrades      int     // trades > $100k
}

func (t *MicrostructureTool) GetLiquiditySnapshot(symbol string) (*LiquiditySnapshot, error)
func (t *MicrostructureTool) GetExpectedSlippage(symbol string, sizeUSD float64) (float64, error)
```

#### Database Schema

```sql
-- migrations/000013_microstructure.up.sql
CREATE TABLE IF NOT EXISTS liquidity_snapshots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    symbol VARCHAR(20) NOT NULL,
    bid_ask_spread_bps DECIMAL(10,4) NOT NULL,
    orderbook_ratio DECIMAL(10,4) NOT NULL,
    bid_volume_1pct DECIMAL(20,8),
    ask_volume_1pct DECIMAL(20,8),
    flow_delta DECIMAL(20,8),
    toxicity_score DECIMAL(5,4),
    liquidity_score DECIMAL(5,2),
    snapshot_at TIMESTAMP NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_liquidity_symbol_time ON liquidity_snapshots(symbol, snapshot_at DESC);
```

#### Real-time Calculation

Use existing WebSocket connection from `internal/workers/candles_worker.go`:

- Parse order book updates
- Track trades with taker side
- Calculate metrics on rolling 5-minute window

**Priority:** 🟡 Tier 2 (Medium Impact, Medium effort)  
**Effort:** 2-3 days  
**Dependencies:** Existing exchange WebSocket

---

## 4. Temporal Patterns & Seasonality

### Why It Matters

- Asian/European/US sessions have different volatility
- "Weekend dump" is real (lower liquidity)
- Post-FOMC days have 2x normal volatility

### Implementation

#### Toolkit Methods

```go
// internal/toolkit/temporal.go

type TemporalTool struct {
    statsRepo *statistics.Repository
}

type TemporalContext struct {
    Symbol           string
    CurrentSession   string    // "asian", "european", "us", "weekend"
    IsWeekend        bool
    HourUTC          int
    DayOfWeek        string
    DayOfMonth       int
    ExpectedVolatility float64 // historical avg for this time
    VolatilityBias   string    // "high", "medium", "low"
    NextMacroEvent   *MacroEvent
}

type MacroEvent struct {
    Name      string    // "FOMC", "CPI", "NFP"
    DateTime  time.Time
    HoursAway int
    Importance string   // "high", "medium", "low"
}

func (t *TemporalTool) GetTemporalContext(symbol string) (*TemporalContext, error)
func (t *TemporalTool) GetSessionVolatility(symbol string, session string) (float64, error)
func (t *TemporalTool) GetNextMacroEvents(daysAhead int) ([]*MacroEvent, error)
```

#### Database Schema

```sql
-- migrations/000014_temporal_stats.up.sql
CREATE TABLE IF NOT EXISTS temporal_statistics (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    symbol VARCHAR(20) NOT NULL,
    hour_utc INT NOT NULL, -- 0-23
    day_of_week INT NOT NULL, -- 0=Sunday, 6=Saturday
    avg_volatility DECIMAL(10,6),
    avg_volume DECIMAL(20,8),
    avg_return DECIMAL(10,6),
    sample_size INT NOT NULL,
    last_updated TIMESTAMP NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(symbol, hour_utc, day_of_week)
);

CREATE TABLE IF NOT EXISTS macro_calendar (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_name VARCHAR(100) NOT NULL,
    event_time TIMESTAMP NOT NULL,
    importance VARCHAR(10) NOT NULL, -- "high", "medium", "low"
    country VARCHAR(10) DEFAULT 'US',
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_macro_upcoming ON macro_calendar(event_time) WHERE event_time > NOW();
```

#### Data Sources

**Session definitions:**

- Asian: 00:00-08:00 UTC
- European: 08:00-16:00 UTC
- US: 16:00-00:00 UTC

**Macro calendar:**

- Manual import from investing.com economic calendar
- Or use Tradingeconomics API (paid)
- Update weekly

**Priority:** 🟢 Tier 1 (Low effort, Good signal)  
**Effort:** 1-2 days  
**Cost:** Free (statistical analysis)

---

## 5. Macroeconomic Indicators

### Why It Matters

- DXY inverse correlation with BTC (r = -0.7)
- VIX spikes = crypto selloffs
- Fed rate decisions = regime changes

### Implementation

#### API Integration

**Yahoo Finance (Free):**

```go
// internal/adapters/macro/yahoo.go

type YahooFinanceClient struct {
    client *http.Client
}

type MacroSnapshot struct {
    DXY       *Quote // US Dollar Index
    US10Y     *Quote // 10-year Treasury yield
    VIX       *Quote // Volatility Index
    SPX       *Quote // S&P 500
    GLD       *Quote // Gold
    UpdatedAt time.Time
}

type Quote struct {
    Symbol        string
    Price         float64
    Change24h     float64
    PercentChange float64
}

func (c *YahooFinanceClient) GetMacroSnapshot() (*MacroSnapshot, error)
func (c *YahooFinanceClient) GetQuote(symbol string) (*Quote, error)
```

#### Database Schema

```sql
-- migrations/000015_macro_indicators.up.sql
CREATE TABLE IF NOT EXISTS macro_indicators (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    indicator VARCHAR(20) NOT NULL, -- "DXY", "VIX", "US10Y", etc
    value DECIMAL(20,8) NOT NULL,
    change_24h DECIMAL(10,4),
    change_pct DECIMAL(10,4),
    recorded_at TIMESTAMP NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_macro_indicator_time ON macro_indicators(indicator, recorded_at DESC);
```

#### Toolkit Integration

```go
// internal/toolkit/macro.go

type MacroTool struct {
    macroRepo *macro.Repository
}

type MacroContext struct {
    DXYTrend       string  // "rising", "falling", "neutral"
    VIXLevel       string  // "low" (<20), "medium" (20-30), "high" (>30)
    RiskAppetite   string  // "risk_on", "risk_off"
    DXYCorrelation float64 // recent correlation with BTC
    Confidence     float64
}

func (t *MacroTool) GetMacroContext() (*MacroContext, error)
func (t *MacroTool) GetDXYTrend(period string) (string, error)
func (t *MacroTool) GetNextFOMC() (*time.Time, error)
```

#### Worker

```go
// internal/workers/macro_worker.go

type MacroWorker struct {
    yahooClient *macro.YahooFinanceClient
    macroRepo   *macro.Repository
}

func (w *MacroWorker) Start(ctx context.Context) {
    ticker := time.NewTicker(5 * time.Minute) // Yahoo updates frequently

    for {
        select {
        case <-ticker.C:
            w.fetchMacroData(ctx)
        case <-ctx.Done():
            return
        }
    }
}
```

**Priority:** 🟡 Tier 2 (Good signal, Easy)  
**Effort:** 2 days  
**Cost:** Free (Yahoo Finance)

---

## 6. Exchange Flows & Reserves

### Why It Matters

- Large inflows to exchanges = selling pressure
- Outflows to cold storage = bullish accumulation
- Exchange reserves at multi-year lows = supply shock potential

### Implementation

#### API Integration

**Option A: CryptoQuant (Paid)**

- Best data quality
- $99/month for basic plan
- Real-time exchange flows

**Option B: Glassnode (Paid)**

- $29/month for starter
- More on-chain metrics

**Option C: Extend existing on-chain monitors**

- Track known exchange wallets
- Use Etherscan/Blockchain.com APIs (free)

```go
// internal/adapters/flows/cryptoquant.go

type CryptoQuantClient struct {
    apiKey string
    client *http.Client
}

type ExchangeFlows struct {
    Symbol           string
    NetFlow24h       float64 // positive = inflow, negative = outflow
    InFlow24h        float64
    OutFlow24h       float64
    ExchangeReserves float64
    ReserveChange7d  float64
    Trend            string // "accumulation", "distribution", "neutral"
}

func (c *CryptoQuantClient) GetExchangeFlows(symbol string) (*ExchangeFlows, error)
func (c *CryptoQuantClient) GetExchangeReserves(symbol string, exchanges []string) (float64, error)
```

#### Database Schema

```sql
-- migrations/000016_exchange_flows.up.sql
CREATE TABLE IF NOT EXISTS exchange_flows (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    symbol VARCHAR(20) NOT NULL,
    exchange VARCHAR(50),
    flow_type VARCHAR(10) NOT NULL, -- "inflow" or "outflow"
    amount DECIMAL(20,8) NOT NULL,
    amount_usd DECIMAL(20,2),
    tx_hash VARCHAR(100),
    detected_at TIMESTAMP NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_flows_symbol_time ON exchange_flows(symbol, detected_at DESC);

CREATE TABLE IF NOT EXISTS exchange_reserves (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    symbol VARCHAR(20) NOT NULL,
    exchange VARCHAR(50) NOT NULL,
    reserve_amount DECIMAL(20,8) NOT NULL,
    reserve_usd DECIMAL(20,2),
    change_24h DECIMAL(10,4),
    change_7d DECIMAL(10,4),
    snapshot_at TIMESTAMP NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_reserves_symbol_time ON exchange_reserves(symbol, exchange, snapshot_at DESC);
```

#### Toolkit Integration

```go
// internal/toolkit/flows.go

type FlowTool struct {
    flowsRepo *flows.Repository
}

type FlowAnalysis struct {
    Symbol          string
    NetFlow24h      float64
    NetFlow7d       float64
    Trend           string // "accumulation", "distribution", "neutral"
    Signal          string // "bullish", "bearish", "neutral"
    Confidence      float64
    LargeMovements  []LargeMovement
}

type LargeMovement struct {
    Direction   string // "inflow" or "outflow"
    AmountUSD   float64
    Exchange    string
    Age         time.Duration
}

func (t *FlowTool) GetFlowAnalysis(symbol string) (*FlowAnalysis, error)
func (t *FlowTool) GetExchangeReserveTrend(symbol string) (string, error)
```

**Priority:** 🟡 Tier 2 (Medium Impact, Medium-High cost)  
**Effort:** 3 days  
**Cost:** $29-99/month depending on provider  
**Alternative:** Extend free on-chain monitors (lower quality)

---

## 7. Options Data & Derivatives Metrics

### Why It Matters

- Put/Call ratio shows institutional sentiment
- Max Pain price attracts price due to dealer hedging
- High IV = expected volatility (trade smaller or wait)
- Gamma walls create support/resistance

### Implementation

#### API Integration

**Deribit API (Free for market data)**

```go
// internal/adapters/derivatives/deribit.go

type DeribitClient struct {
    client *http.Client
}

type OptionsMarketData struct {
    Symbol          string
    ExpiryDate      time.Time
    PutCallRatio    float64
    MaxPainPrice    float64
    TotalOI         float64
    IVIndex         float64 // Implied Volatility Index
    GammaLevels     []GammaLevel
}

type GammaLevel struct {
    Strike      float64
    GammaExposure float64 // positive or negative
    Type        string // "support" or "resistance"
}

func (c *DeribitClient) GetOptionsData(symbol string, expiry time.Time) (*OptionsMarketData, error)
func (c *DeribitClient) GetIVIndex(symbol string) (float64, error)
func (c *DeribitClient) CalculateMaxPain(symbol string, expiry time.Time) (float64, error)
```

#### Database Schema

```sql
-- migrations/000017_options_data.up.sql
CREATE TABLE IF NOT EXISTS options_metrics (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    symbol VARCHAR(20) NOT NULL,
    expiry_date DATE NOT NULL,
    put_call_ratio DECIMAL(10,4) NOT NULL,
    max_pain_price DECIMAL(20,8),
    total_oi DECIMAL(20,2),
    iv_index DECIMAL(10,4),
    recorded_at TIMESTAMP NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_options_symbol_expiry ON options_metrics(symbol, expiry_date, recorded_at DESC);

CREATE TABLE IF NOT EXISTS gamma_levels (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    symbol VARCHAR(20) NOT NULL,
    expiry_date DATE NOT NULL,
    strike_price DECIMAL(20,8) NOT NULL,
    gamma_exposure DECIMAL(20,2) NOT NULL,
    level_type VARCHAR(20), -- "support" or "resistance"
    snapshot_at TIMESTAMP NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_gamma_symbol ON gamma_levels(symbol, snapshot_at DESC);
```

#### Toolkit Integration

```go
// internal/toolkit/options.go

type OptionsTool struct {
    derivRepo *derivatives.Repository
}

type OptionsSignal struct {
    Symbol           string
    PutCallRatio     float64
    Sentiment        string // "bullish", "bearish", "neutral"
    MaxPainPrice     float64
    DistanceToMaxPain float64 // percentage
    IVLevel          string // "low", "medium", "high", "extreme"
    NearestGamma     *GammaLevel
    Signal           string
    Confidence       float64
}

func (t *OptionsTool) GetOptionsSignal(symbol string) (*OptionsSignal, error)
func (t *OptionsTool) GetNextExpiry(symbol string) (time.Time, error)
func (t *OptionsTool) GetGammaWalls(symbol string, currentPrice float64) ([]GammaLevel, error)
```

#### Worker

```go
// internal/workers/options_worker.go

type OptionsWorker struct {
    deribitClient *derivatives.DeribitClient
    derivRepo     *derivatives.Repository
    symbols       []string // BTC, ETH mainly
}

func (w *OptionsWorker) Start(ctx context.Context) {
    ticker := time.NewTicker(10 * time.Minute)

    for {
        select {
        case <-ticker.C:
            w.fetchOptionsData(ctx)
        case <-ctx.Done():
            return
        }
    }
}
```

**Priority:** 🔴 Tier 3 (High Impact, Medium effort, Complex)  
**Effort:** 4-5 days  
**Cost:** Free (Deribit market data)  
**Note:** Focus on BTC/ETH only initially

---

## 8. Enhanced Social Metrics

### Why It Matters

- Social volume spikes predict price moves (1-24h lag)
- Influencer posts drive retail FOMO
- GitHub activity = developer sentiment (for altcoins)

### Implementation

#### API Integration

**Option A: LunarCrush (Paid)**

```go
// internal/adapters/social/lunarcrush.go

type LunarCrushClient struct {
    apiKey string
}

type SocialMetrics struct {
    Symbol             string
    SocialVolume       int64   // mentions last 24h
    SocialVolume Change float64 // vs 7d avg
    SocialScore        float64 // 0-100 composite
    GalaxyScore        float64 // 0-100 proprietary
    Sentiment          float64 // -1 to 1
    InfluencerPosts    int     // posts by top 10 influencers
    RedditActivity     int
    TwitterActivity    int
}

func (c *LunarCrushClient) GetSocialMetrics(symbol string) (*SocialMetrics, error)
```

**Option B: Extend existing news system**

- Already have Twitter/Reddit scrapers
- Add volume tracking
- Add influencer whitelist

#### Toolkit Integration

```go
// Extend internal/toolkit/news.go

func (t *NewsTool) GetSocialVolume(symbol string, period string) (int64, error)
func (t *NewsTool) GetSocialVolumeChange(symbol string) (float64, error)
func (t *NewsTool) GetInfluencerActivity(symbol string) (int, error)
```

**Priority:** 🔴 Tier 3 (Medium Impact, Expensive)  
**Effort:** 2 days (if paid API), 5 days (if DIY)  
**Cost:** $99-299/month (LunarCrush) or $0 (extend existing)

---

## 9. Staking Metrics & Unlock Events

### Why It Matters

- High staking ratio = reduced circulating supply
- Unlock events create selling pressure
- Validator changes signal confidence

### Implementation

#### Manual Database

```sql
-- migrations/000018_staking_metrics.up.sql
CREATE TABLE IF NOT EXISTS staking_metrics (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    symbol VARCHAR(20) NOT NULL,
    staking_ratio DECIMAL(5,2), -- percentage of supply staked
    total_staked DECIMAL(20,8),
    validator_count INT,
    avg_stake_duration_days INT,
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(symbol)
);

CREATE TABLE IF NOT EXISTS token_unlocks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    symbol VARCHAR(20) NOT NULL,
    project_name VARCHAR(100),
    unlock_date DATE NOT NULL,
    unlock_amount DECIMAL(20,8),
    unlock_value_usd DECIMAL(20,2),
    unlock_pct_supply DECIMAL(5,2),
    recipient VARCHAR(100), -- "team", "investors", "community"
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_unlocks_upcoming ON token_unlocks(unlock_date) WHERE unlock_date > NOW();
```

#### Data Sources

- Manual research (TokenUnlocks.app, project docs)
- Staking data from blockchain explorers
- Update monthly

#### Toolkit

```go
// internal/toolkit/tokenomics.go

type TokenomicsTool struct {
    tokenRepo *tokenomics.Repository
}

type TokenomicsContext struct {
    Symbol            string
    StakingRatio      float64
    UpcomingUnlocks   []TokenUnlock
    NextUnlockDays    int
    NextUnlockPct     float64
    Risk              string // "high", "medium", "low"
}

type TokenUnlock struct {
    Date           time.Time
    AmountUSD      float64
    PercentSupply  float64
    Recipient      string
}

func (t *TokenomicsTool) GetTokenomicsContext(symbol string) (*TokenomicsContext, error)
func (t *TokenomicsTool) GetNextUnlock(symbol string) (*TokenUnlock, error)
```

**Priority:** 🔴 Tier 3 (Medium Impact, Manual work)  
**Effort:** 1 day (code) + ongoing manual updates  
**Cost:** Free (manual research)

---

## 10. Sector Rotation & Peer Analysis

### Why It Matters

- Sector momentum > individual coin in many cases
- Underperforming vs sector = potential catch-up trade
- Sector rotation signals (DeFi → Gaming → AI)

### Implementation

#### Database Schema

```sql
-- migrations/000019_sectors.up.sql
CREATE TABLE IF NOT EXISTS coin_sectors (
    symbol VARCHAR(20) PRIMARY KEY,
    sector VARCHAR(50) NOT NULL, -- "DeFi", "Layer1", "Layer2", "Gaming", "AI", etc
    market_cap_rank INT,
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_sectors ON coin_sectors(sector);

CREATE TABLE IF NOT EXISTS sector_performance (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sector VARCHAR(50) NOT NULL,
    performance_1h DECIMAL(10,4),
    performance_24h DECIMAL(10,4),
    performance_7d DECIMAL(10,4),
    market_cap_usd DECIMAL(20,2),
    volume_24h DECIMAL(20,2),
    calculated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_sector_perf ON sector_performance(sector, calculated_at DESC);
```

#### Toolkit Enhancement

```go
// Extend internal/toolkit/peers.go

type SectorAnalysis struct {
    Symbol             string
    Sector             string
    SectorPerformance  float64 // 24h %
    CoinPerformance    float64 // 24h %
    RelativeStrength   float64 // coin vs sector
    SectorRank         int     // rank within sector
    SectorTrend        string  // "leading", "lagging", "inline"
    RotationSignal     string  // "rotate_in", "rotate_out", "hold"
}

func (t *PeerTool) GetSectorAnalysis(symbol string) (*SectorAnalysis, error)
func (t *PeerTool) GetTopSectors(limit int) ([]SectorPerformance, error)
func (t *PeerTool) GetRotationSignal(symbol string) (string, error)
```

**Priority:** 🟡 Tier 2 (Good signal, Easy to extend)  
**Effort:** 1-2 days  
**Dependencies:** Existing peer comparison tools

---

## Implementation Phases

### Phase 1: Quick Wins (Week 1)

Focus on high-impact, low-effort improvements:

1. ✅ **Correlations** (2 days)

   - Worker to calculate BTC correlation
   - Market regime detection
   - Agent toolkit integration

2. ✅ **Temporal Patterns** (2 days)

   - Session detection
   - Historical volatility by hour
   - Simple macro calendar

3. ✅ **Sector Analysis** (1 day)
   - Extend existing peer tools
   - Sector classification table
   - Rotation signals

**Outcome:** Agents get 3 new strong signals with minimal cost

---

### Phase 2: External Data (Week 2-3)

Add paid/complex integrations:

1. 🔥 **Liquidation Levels** (3 days)

   - Coinglass API integration
   - Database schema
   - Worker + toolkit
   - **Priority 1** - strongest signal

2. 📊 **Macro Indicators** (2 days)

   - Yahoo Finance API
   - DXY, VIX tracking
   - Worker + toolkit

3. 💧 **Microstructure** (3 days)
   - Enhance WebSocket handler
   - Order book metrics
   - Trade flow analysis

**Outcome:** Institutional-grade signals added

---

### Phase 3: Advanced Derivatives (Week 4)

Complex but high value:

1. 📈 **Options Data** (4 days)

   - Deribit API
   - Max Pain calculation
   - Gamma level detection
   - BTC/ETH only

2. 🌊 **Exchange Flows** (3 days)
   - CryptoQuant OR extend on-chain
   - Reserve tracking
   - Flow analysis

**Outcome:** "Smart money" signals

---

### Phase 4: Optional Enhancements (Week 5+)

Lower priority refinements:

1. 📱 **Social Volume** (paid API or extend existing)
2. 🔓 **Tokenomics** (manual database maintenance)
3. 📊 **GitHub Activity** (for altcoin agents)

---

## Agent Integration Strategy

### Update Agent Personality Configs

Add new signal weights to `internal/agents/personalities.go`:

```go
type PersonalityConfig struct {
    // ... existing fields ...

    // New signal weights
    LiquidationRiskWeight  float64 // 0-1
    MarcoContextWeight     float64
    LiquidityScoreWeight   float64
    OptionsSignalWeight    float64
    FlowAnalysisWeight     float64
    CorrelationWeight      float64
    TemporalBiasWeight     float64
    SectorMomentumWeight   float64
}
```

**Example adjustments:**

- **Scalper Sam:** High liquidity + microstructure weights
- **Whale Watcher:** High flow + liquidation weights
- **Swing Steve:** High macro + options weights
- **Contrarian Carl:** Inverts liquidation + flow signals

### Chain-of-Thought Prompts

Update `templates/agentic/cot_*.tmpl` to include new context:

```
Available Market Intelligence:
- BTC Correlation: {{.Correlation}}
- Market Regime: {{.MarketRegime}}
- Liquidation Risk: {{.LiquidationRisk}}
- Liquidity Score: {{.LiquidityScore}}
- Options Signal: {{.OptionsSignal}}
- Flow Trend: {{.FlowTrend}}
- Macro Context: {{.MacroContext}}
- Temporal Bias: {{.TemporalBias}}
```

---

## Testing Strategy

### Unit Tests

- Mock all external APIs
- Test calculation logic independently
- Repository pattern makes this easy

### Integration Tests

- Paper trading with new signals
- A/B test: old agents vs enhanced agents
- Track win rate improvement

### Validation Metrics

- **Baseline:** Current agent win rate (~50-55%)
- **Target:** 60%+ win rate after Phase 2
- **Stretch Goal:** 65%+ after Phase 3

### Performance Monitoring

- Add tool usage metrics to ClickHouse
- Track which signals most correlated with winning trades
- Use semantic memory to learn signal importance

---

## Cost Summary

| Component       | API/Service   | Monthly Cost     | Priority |
| --------------- | ------------- | ---------------- | -------- |
| Correlations    | Internal      | $0               | Tier 1   |
| Temporal        | Internal      | $0               | Tier 1   |
| Sector Analysis | Internal      | $0               | Tier 2   |
| Liquidations    | Coinglass     | $0 (free tier)   | Tier 1   |
| Macro           | Yahoo Finance | $0               | Tier 2   |
| Microstructure  | Internal      | $0               | Tier 2   |
| Options         | Deribit       | $0 (market data) | Tier 3   |
| Exchange Flows  | CryptoQuant   | $99              | Tier 2   |
| Social Volume   | LunarCrush    | $99              | Tier 3   |
| Tokenomics      | Manual        | $0               | Tier 3   |

**Total Cost (Phase 1-2):** $0-99/month  
**Total Cost (All):** $99-198/month

---

## Success Metrics

### Quantitative

- Agent win rate: 50% → 60%+
- Sharpe ratio improvement: 20%+
- Max drawdown reduction: 15%+
- Average trade quality score: +0.15

### Qualitative

- Agents avoid obvious liquidation cascades
- Better timing around macro events
- Improved risk-off detection
- More nuanced sector rotation

---

## Migration Path

1. **Week 1:** Deploy Phase 1 to paper trading
2. **Week 2:** Monitor for 7 days, collect metrics
3. **Week 3:** Deploy Phase 2 to paper trading
4. **Week 4:** A/B test: 50% agents with new signals
5. **Week 5:** Full rollout if win rate > 58%
6. **Week 6+:** Phase 3 for premium users

---

## Documentation Updates Needed

- [ ] `docs/SIGNALS_GUIDE.md` - User-facing signal explanations
- [ ] `docs/API_INTEGRATIONS.md` - API setup instructions
- [ ] Update `docs/AGENTS.md` - New personality weights
- [ ] Update `docs/QUICK_START.md` - API key setup

---

## Questions to Answer Before Starting

1. **Budget:** Approved for paid APIs? ($99-198/month)
2. **Scope:** All phases or just Phase 1?
3. **Testing Duration:** How long for paper trading validation?
4. **Priority Assets:** BTC/ETH only or include top 10 alts?
5. **Performance Target:** 60% win rate acceptable?

---

## Next Steps

**Tomorrow's Implementation Plan:**

1. Read this doc thoroughly
2. Pick a phase (recommend: Phase 1 first)
3. Create feature branch: `feature/advanced-signals-phase1`
4. Start with correlations (easiest, high impact)
5. Test each component independently
6. Integration test with one agent
7. Deploy to paper trading
8. Monitor for 2-3 days
9. Iterate

**Daily Commit Strategy:**

- Commit after each component (adapter, repo, worker, toolkit)
- Don't wait for entire phase to be done
- Keep PRs small and focused

Good luck! 🚀
