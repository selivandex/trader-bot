-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Initial database schema for AI Agent Trading System
-- API keys encrypted in application layer (AES-256-GCM)

-- Users table (Telegram users)
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    telegram_id BIGINT UNIQUE NOT NULL,
    username VARCHAR(100),
    first_name VARCHAR(100),
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_users_telegram_id ON users(telegram_id);
CREATE INDEX idx_users_is_active ON users(is_active);

-- User exchange connections (one per exchange)
-- API keys encrypted using AES-256-GCM in application layer
CREATE TABLE IF NOT EXISTS user_exchanges (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    exchange VARCHAR(50) NOT NULL, -- binance, bybit
    api_key_encrypted TEXT NOT NULL,    -- Base64 encrypted API key
    api_secret_encrypted TEXT NOT NULL, -- Base64 encrypted API secret
    testnet BOOLEAN NOT NULL DEFAULT true,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, exchange)
);

CREATE INDEX idx_user_exchanges_user_id ON user_exchanges(user_id);
CREATE INDEX idx_user_exchanges_exchange ON user_exchanges(exchange);

-- User trading pairs (which tickers user wants to trade)
CREATE TABLE IF NOT EXISTS user_trading_pairs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    exchange_id UUID NOT NULL REFERENCES user_exchanges(id) ON DELETE CASCADE,
    symbol VARCHAR(20) NOT NULL,
    budget DECIMAL(20, 8) NOT NULL, -- Budget allocated for this symbol
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, symbol)
);

CREATE INDEX idx_user_trading_pairs_user_id ON user_trading_pairs(user_id);
CREATE INDEX idx_user_trading_pairs_symbol ON user_trading_pairs(symbol);
CREATE INDEX idx_user_trading_pairs_is_active ON user_trading_pairs(is_active);

-- Triggers for updated_at
CREATE OR REPLACE FUNCTION update_timestamp()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_update_users_timestamp
BEFORE UPDATE ON users
FOR EACH ROW
EXECUTE FUNCTION update_timestamp();

CREATE TRIGGER trigger_update_user_exchanges_timestamp
BEFORE UPDATE ON user_exchanges
FOR EACH ROW
EXECUTE FUNCTION update_timestamp();

CREATE TRIGGER trigger_update_trading_pairs_timestamp
BEFORE UPDATE ON user_trading_pairs
FOR EACH ROW
EXECUTE FUNCTION update_timestamp();

-- Comments
COMMENT ON TABLE users IS 'Telegram users who can create and manage AI agents';
COMMENT ON TABLE user_exchanges IS 'User exchange API credentials (one per exchange)';
COMMENT ON TABLE user_trading_pairs IS 'Trading pairs user wants to trade with budget allocation';
