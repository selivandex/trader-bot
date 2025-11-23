-- Asset Correlations Tracking
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

-- Market Regime Detection
CREATE TABLE IF NOT EXISTS market_regimes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    regime VARCHAR(20) NOT NULL, -- "risk_on", "risk_off", "neutral"
    btc_dominance DECIMAL(5,2) NOT NULL,
    avg_correlation DECIMAL(5,4) NOT NULL,
    volatility_level VARCHAR(10) NOT NULL, -- "low", "medium", "high"
    confidence DECIMAL(5,4) NOT NULL,
    detected_at TIMESTAMP NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_regimes_time ON market_regimes(detected_at DESC);

COMMENT ON TABLE asset_correlations IS 'Tracks correlation coefficients between trading pairs over time';
COMMENT ON TABLE market_regimes IS 'Detects overall market regime (risk-on/risk-off) for agent decision making';

