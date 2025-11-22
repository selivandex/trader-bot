-- OHLCV Candles storage for backtesting and analysis

CREATE TABLE IF NOT EXISTS ohlcv_candles (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    symbol VARCHAR(20) NOT NULL,
    timeframe VARCHAR(10) NOT NULL,  -- 1m, 5m, 15m, 1h, 4h, 1d
    timestamp TIMESTAMP NOT NULL,
    open DECIMAL(20, 8) NOT NULL,
    high DECIMAL(20, 8) NOT NULL,
    low DECIMAL(20, 8) NOT NULL,
    close DECIMAL(20, 8) NOT NULL,
    volume DECIMAL(30, 8) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(symbol, timeframe, timestamp)
);

CREATE INDEX idx_ohlcv_symbol_timeframe ON ohlcv_candles(symbol, timeframe);
CREATE INDEX idx_ohlcv_timestamp ON ohlcv_candles(timestamp DESC);
CREATE INDEX idx_ohlcv_symbol_timeframe_timestamp ON ohlcv_candles(symbol, timeframe, timestamp DESC);

COMMENT ON TABLE ohlcv_candles IS 'Historical OHLCV candles for backtesting and analysis';

