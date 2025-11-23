-- Unified metrics tables for buffered collection

-- Embedding deduplication metrics (cache hits/misses)
CREATE TABLE IF NOT EXISTS embedding_deduplication_metrics (
    timestamp DateTime DEFAULT now(),
    text_hash String,
    text_length UInt32,
    model String,
    cache_hit UInt8,  -- 0 = miss, 1 = hit
    cost_saved_usd Float32  -- $0.0001 per hit
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (timestamp, text_hash)
TTL timestamp + INTERVAL 90 DAY;

COMMENT ON TABLE embedding_deduplication_metrics 'Tracks embedding deduplication efficiency and cost savings';

-- Agent decision metrics (extended from existing agent_metrics)
CREATE TABLE IF NOT EXISTS agent_decision_metrics (
    timestamp DateTime DEFAULT now(),
    agent_name String,
    agent_personality String,
    symbol String,
    action String,
    confidence UInt8,
    thinking_iterations UInt16,
    tools_used UInt8,
    decision_time_ms UInt32,
    memories_recalled UInt8
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (timestamp, agent_name, symbol)
TTL timestamp + INTERVAL 90 DAY;

COMMENT ON TABLE agent_decision_metrics 'Tracks agent decision quality and thinking process metrics';

