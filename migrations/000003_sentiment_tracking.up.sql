-- Sentiment tracking for agents

CREATE TABLE IF NOT EXISTS sentiment_snapshots (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    symbol VARCHAR(20) NOT NULL,
    positive_count INTEGER NOT NULL DEFAULT 0,
    negative_count INTEGER NOT NULL DEFAULT 0,
    neutral_count INTEGER NOT NULL DEFAULT 0,
    total_count INTEGER NOT NULL DEFAULT 0,
    average_sentiment DECIMAL(5, 4) NOT NULL DEFAULT 0,  -- -1.0 to 1.0
    weighted_sentiment DECIMAL(5, 4) NOT NULL DEFAULT 0,
    sentiment_trend VARCHAR(20),  -- improving, declining, stable
    snapshot_at TIMESTAMP NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_sentiment_snapshots_symbol ON sentiment_snapshots(symbol);
CREATE INDEX idx_sentiment_snapshots_snapshot_at ON sentiment_snapshots(snapshot_at DESC);

COMMENT ON TABLE sentiment_snapshots IS 'Aggregated sentiment scores over time for agents';

