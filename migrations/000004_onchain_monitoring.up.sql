-- On-chain monitoring for agents

CREATE TABLE IF NOT EXISTS whale_transactions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tx_hash VARCHAR(100) UNIQUE NOT NULL,
    blockchain VARCHAR(50) NOT NULL,
    symbol VARCHAR(20) NOT NULL,
    amount DECIMAL(30, 8) NOT NULL,
    amount_usd DECIMAL(20, 2) NOT NULL,
    from_address VARCHAR(100),
    to_address VARCHAR(100),
    from_owner VARCHAR(100),  -- binance, unknown, etc
    to_owner VARCHAR(100),
    transaction_type VARCHAR(50),  -- exchange_inflow, exchange_outflow, whale_movement
    timestamp TIMESTAMP NOT NULL,
    impact_score INTEGER DEFAULT 5,  -- 1-10
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_whale_tx_symbol ON whale_transactions(symbol);
CREATE INDEX idx_whale_tx_timestamp ON whale_transactions(timestamp DESC);
CREATE INDEX idx_whale_tx_impact ON whale_transactions(impact_score DESC);
CREATE INDEX idx_whale_tx_type ON whale_transactions(transaction_type);

CREATE TABLE IF NOT EXISTS exchange_flows (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    exchange VARCHAR(50) NOT NULL,
    symbol VARCHAR(20) NOT NULL,
    timestamp TIMESTAMP NOT NULL,
    inflow DECIMAL(30, 8) NOT NULL DEFAULT 0,
    outflow DECIMAL(30, 8) NOT NULL DEFAULT 0,
    net_flow DECIMAL(30, 8) NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_exchange_flows_symbol ON exchange_flows(symbol);
CREATE INDEX idx_exchange_flows_timestamp ON exchange_flows(timestamp DESC);
CREATE INDEX idx_exchange_flows_exchange ON exchange_flows(exchange);

COMMENT ON TABLE whale_transactions IS 'Large blockchain transactions for agent on-chain analysis';
COMMENT ON TABLE exchange_flows IS 'Aggregated exchange inflow/outflow for agents';

