-- Initial database schema for multi-user trading bot

-- Users table
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    telegram_id BIGINT UNIQUE NOT NULL,
    username VARCHAR(100),
    first_name VARCHAR(100),
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- User configurations (per-user trading settings)
-- Now supports multiple trading pairs per user
CREATE TABLE IF NOT EXISTS user_configs (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    exchange VARCHAR(50) NOT NULL, -- binance, bybit
    api_key TEXT NOT NULL,
    api_secret TEXT NOT NULL,
    testnet BOOLEAN NOT NULL DEFAULT true,
    symbol VARCHAR(20) NOT NULL DEFAULT 'BTC/USDT',
    initial_balance DECIMAL(20, 8) NOT NULL DEFAULT 1000,
    max_position_percent DECIMAL(5, 2) NOT NULL DEFAULT 30.0,
    max_leverage INTEGER NOT NULL DEFAULT 3,
    stop_loss_percent DECIMAL(5, 2) NOT NULL DEFAULT 2.0,
    take_profit_percent DECIMAL(5, 2) NOT NULL DEFAULT 5.0,
    is_trading BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, symbol)
);

-- User state (per-user per-symbol bot state)
-- Each trading pair has its own state
CREATE TABLE IF NOT EXISTS user_states (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    symbol VARCHAR(20) NOT NULL,
    mode VARCHAR(20) NOT NULL DEFAULT 'paper',
    status VARCHAR(20) NOT NULL DEFAULT 'stopped',
    balance DECIMAL(20, 8) NOT NULL DEFAULT 0,
    equity DECIMAL(20, 8) NOT NULL DEFAULT 0,
    daily_pnl DECIMAL(20, 8) NOT NULL DEFAULT 0,
    peak_equity DECIMAL(20, 8) NOT NULL DEFAULT 0,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, symbol)
);

-- Trades table
CREATE TABLE IF NOT EXISTS trades (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    config_id INTEGER REFERENCES user_configs(id) ON DELETE SET NULL,
    exchange VARCHAR(50) NOT NULL,
    symbol VARCHAR(20) NOT NULL,
    side VARCHAR(10) NOT NULL,
    type VARCHAR(10) NOT NULL,
    amount DECIMAL(20, 8) NOT NULL,
    price DECIMAL(20, 8) NOT NULL,
    fee DECIMAL(20, 8) NOT NULL DEFAULT 0,
    pnl DECIMAL(20, 8),
    ai_decision JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for trades
CREATE INDEX IF NOT EXISTS idx_trades_user_id ON trades(user_id);
CREATE INDEX IF NOT EXISTS idx_trades_user_symbol ON trades(user_id, symbol);
CREATE INDEX IF NOT EXISTS idx_trades_config_id ON trades(config_id);
CREATE INDEX IF NOT EXISTS idx_trades_symbol ON trades(symbol);
CREATE INDEX IF NOT EXISTS idx_trades_created_at ON trades(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_trades_exchange ON trades(exchange);

-- AI decisions table
CREATE TABLE IF NOT EXISTS ai_decisions (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider VARCHAR(50) NOT NULL,
    prompt TEXT NOT NULL,
    response JSONB NOT NULL,
    confidence INTEGER NOT NULL,
    executed BOOLEAN NOT NULL DEFAULT false,
    outcome JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for AI decisions
CREATE INDEX IF NOT EXISTS idx_ai_decisions_user_id ON ai_decisions(user_id);
CREATE INDEX IF NOT EXISTS idx_ai_decisions_provider ON ai_decisions(provider);
CREATE INDEX IF NOT EXISTS idx_ai_decisions_executed ON ai_decisions(executed);
CREATE INDEX IF NOT EXISTS idx_ai_decisions_created_at ON ai_decisions(created_at DESC);

-- Positions table (for tracking open positions)
CREATE TABLE IF NOT EXISTS positions (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    exchange VARCHAR(50) NOT NULL,
    symbol VARCHAR(20) NOT NULL,
    side VARCHAR(10) NOT NULL,
    size DECIMAL(20, 8) NOT NULL,
    entry_price DECIMAL(20, 8) NOT NULL,
    current_price DECIMAL(20, 8) NOT NULL,
    leverage INTEGER NOT NULL DEFAULT 1,
    unrealized_pnl DECIMAL(20, 8) NOT NULL DEFAULT 0,
    liquidation_price DECIMAL(20, 8),
    margin DECIMAL(20, 8) NOT NULL,
    stop_loss DECIMAL(20, 8),
    take_profit DECIMAL(20, 8),
    opened_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    closed_at TIMESTAMP,
    is_open BOOLEAN NOT NULL DEFAULT true
);

-- Indexes for positions
CREATE INDEX IF NOT EXISTS idx_positions_user_open ON positions(user_id, is_open) WHERE is_open = true;
CREATE INDEX IF NOT EXISTS idx_positions_symbol ON positions(symbol);

-- Risk events table (for tracking circuit breaker events, losses, etc)
CREATE TABLE IF NOT EXISTS risk_events (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    event_type VARCHAR(50) NOT NULL,
    description TEXT,
    data JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for risk events
CREATE INDEX IF NOT EXISTS idx_risk_events_user_id ON risk_events(user_id);
CREATE INDEX IF NOT EXISTS idx_risk_events_type ON risk_events(event_type);
CREATE INDEX IF NOT EXISTS idx_risk_events_created_at ON risk_events(created_at DESC);

-- Performance metrics table (daily snapshots per user)
CREATE TABLE IF NOT EXISTS performance_metrics (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    date DATE NOT NULL,
    starting_balance DECIMAL(20, 8) NOT NULL,
    ending_balance DECIMAL(20, 8) NOT NULL,
    daily_pnl DECIMAL(20, 8) NOT NULL,
    daily_pnl_percent DECIMAL(10, 4) NOT NULL,
    total_trades INTEGER NOT NULL DEFAULT 0,
    winning_trades INTEGER NOT NULL DEFAULT 0,
    losing_trades INTEGER NOT NULL DEFAULT 0,
    win_rate DECIMAL(10, 4),
    total_fees DECIMAL(20, 8) NOT NULL DEFAULT 0,
    max_drawdown DECIMAL(10, 4),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, date)
);

-- Index for performance metrics
CREATE INDEX IF NOT EXISTS idx_performance_user_date ON performance_metrics(user_id, date DESC);

-- User sessions (for tracking bot activity)
CREATE TABLE IF NOT EXISTS user_sessions (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    started_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    stopped_at TIMESTAMP,
    trades_count INTEGER DEFAULT 0,
    total_pnl DECIMAL(20, 8) DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT true
);

CREATE INDEX IF NOT EXISTS idx_user_sessions_user_active ON user_sessions(user_id, is_active);

-- Functions and Triggers

-- Function to update user timestamps
CREATE OR REPLACE FUNCTION update_user_timestamp()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Triggers for updated_at
CREATE TRIGGER trigger_update_users_timestamp
BEFORE UPDATE ON users
FOR EACH ROW
EXECUTE FUNCTION update_user_timestamp();

CREATE TRIGGER trigger_update_user_configs_timestamp
BEFORE UPDATE ON user_configs
FOR EACH ROW
EXECUTE FUNCTION update_user_timestamp();

CREATE TRIGGER trigger_update_user_states_timestamp
BEFORE UPDATE ON user_states
FOR EACH ROW
EXECUTE FUNCTION update_user_timestamp();

-- Function to calculate daily metrics for specific user
CREATE OR REPLACE FUNCTION calculate_daily_metrics(target_user_id INTEGER, target_date DATE)
RETURNS void AS $$
DECLARE
    starting_bal DECIMAL(20, 8);
    ending_bal DECIMAL(20, 8);
    total_pnl DECIMAL(20, 8);
    total_count INTEGER;
    win_count INTEGER;
    loss_count INTEGER;
    total_fee DECIMAL(20, 8);
BEGIN
    -- Get metrics for the day and user
    SELECT 
        COALESCE(SUM(pnl), 0),
        COUNT(*),
        COUNT(*) FILTER (WHERE pnl > 0),
        COUNT(*) FILTER (WHERE pnl < 0),
        COALESCE(SUM(fee), 0)
    INTO total_pnl, total_count, win_count, loss_count, total_fee
    FROM trades
    WHERE user_id = target_user_id AND DATE(created_at) = target_date;

    -- Get starting balance (ending balance from previous day)
    SELECT COALESCE(ending_balance, 0) INTO starting_bal
    FROM performance_metrics
    WHERE user_id = target_user_id AND date < target_date
    ORDER BY date DESC
    LIMIT 1;

    -- If no previous day, use initial balance from config
    IF starting_bal = 0 THEN
        SELECT initial_balance INTO starting_bal 
        FROM user_configs 
        WHERE user_id = target_user_id;
    END IF;

    ending_bal := starting_bal + total_pnl;

    -- Insert or update metrics
    INSERT INTO performance_metrics (
        user_id, date, starting_balance, ending_balance, daily_pnl, daily_pnl_percent,
        total_trades, winning_trades, losing_trades, win_rate, total_fees
    ) VALUES (
        target_user_id, target_date, starting_bal, ending_bal, total_pnl,
        CASE WHEN starting_bal > 0 THEN (total_pnl / starting_bal) * 100 ELSE 0 END,
        total_count, win_count, loss_count,
        CASE WHEN total_count > 0 THEN (win_count::DECIMAL / total_count) * 100 ELSE 0 END,
        total_fee
    )
    ON CONFLICT (user_id, date) DO UPDATE SET
        ending_balance = EXCLUDED.ending_balance,
        daily_pnl = EXCLUDED.daily_pnl,
        daily_pnl_percent = EXCLUDED.daily_pnl_percent,
        total_trades = EXCLUDED.total_trades,
        winning_trades = EXCLUDED.winning_trades,
        losing_trades = EXCLUDED.losing_trades,
        win_rate = EXCLUDED.win_rate,
        total_fees = EXCLUDED.total_fees;
END;
$$ LANGUAGE plpgsql;

-- Useful Views

-- User overview (combines user, config, and state)
-- Shows all trading pairs for each user
CREATE OR REPLACE VIEW user_overview AS
SELECT 
    u.id,
    u.telegram_id,
    u.username,
    u.first_name,
    uc.id as config_id,
    uc.exchange,
    uc.symbol,
    us.balance,
    us.equity,
    us.daily_pnl,
    us.status,
    uc.is_trading,
    COUNT(DISTINCT t.id) as total_trades,
    COUNT(DISTINCT t.id) FILTER (WHERE t.pnl > 0) as winning_trades,
    COALESCE(SUM(t.pnl), 0) as total_pnl,
    u.created_at
FROM users u
LEFT JOIN user_configs uc ON u.id = uc.user_id
LEFT JOIN user_states us ON u.id = us.user_id AND uc.symbol = us.symbol
LEFT JOIN trades t ON u.id = t.user_id AND uc.symbol = t.symbol
WHERE u.is_active = true
GROUP BY u.id, u.telegram_id, u.username, u.first_name, uc.id, uc.exchange, uc.symbol, 
         us.balance, us.equity, us.daily_pnl, us.status, uc.is_trading, u.created_at;

-- Recent trades per user
CREATE OR REPLACE VIEW recent_trades_by_user AS
SELECT 
    t.id, 
    t.user_id,
    u.username,
    t.exchange, 
    t.symbol, 
    t.side, 
    t.type, 
    t.amount, 
    t.price, 
    t.fee, 
    t.pnl,
    CASE 
        WHEN t.pnl > 0 THEN 'win'
        WHEN t.pnl < 0 THEN 'loss'
        ELSE 'neutral'
    END as outcome,
    t.created_at
FROM trades t
JOIN users u ON t.user_id = u.id
ORDER BY t.created_at DESC
LIMIT 100;

-- Open positions per user
CREATE OR REPLACE VIEW open_positions_by_user AS
SELECT 
    p.id,
    p.user_id,
    u.username,
    p.exchange, 
    p.symbol, 
    p.side, 
    p.size, 
    p.entry_price, 
    p.current_price,
    p.leverage, 
    p.unrealized_pnl, 
    p.liquidation_price, 
    p.margin,
    p.stop_loss, 
    p.take_profit, 
    p.opened_at
FROM positions p
JOIN users u ON p.user_id = u.id
WHERE p.is_open = true;

-- AI provider stats per user
CREATE OR REPLACE VIEW ai_provider_stats_by_user AS
SELECT 
    ad.user_id,
    u.username,
    ad.provider,
    COUNT(*) as total_decisions,
    COUNT(*) FILTER (WHERE ad.executed = true) as executed_decisions,
    COUNT(*) FILTER (WHERE ad.executed = true AND ad.outcome->>'profitable' = 'true') as profitable_decisions,
    CASE 
        WHEN COUNT(*) FILTER (WHERE ad.executed = true) > 0 
        THEN (COUNT(*) FILTER (WHERE ad.executed = true AND ad.outcome->>'profitable' = 'true')::DECIMAL / 
              COUNT(*) FILTER (WHERE ad.executed = true)) * 100
        ELSE 0 
    END as accuracy_percent,
    AVG(ad.confidence) as avg_confidence
FROM ai_decisions ad
JOIN users u ON ad.user_id = u.id
GROUP BY ad.user_id, u.username, ad.provider;

-- Comments
COMMENT ON TABLE users IS 'Registered bot users';
COMMENT ON TABLE user_configs IS 'Per-user trading configuration and exchange credentials';
COMMENT ON TABLE user_states IS 'Current state for each user bot instance';
COMMENT ON TABLE trades IS 'All executed trades history per user';
COMMENT ON TABLE ai_decisions IS 'AI model decisions and their outcomes per user';
COMMENT ON TABLE positions IS 'Open and closed positions tracking per user';
COMMENT ON TABLE risk_events IS 'Risk management events log per user';
COMMENT ON TABLE performance_metrics IS 'Daily performance metrics per user';
COMMENT ON TABLE user_sessions IS 'Trading session history per user';
