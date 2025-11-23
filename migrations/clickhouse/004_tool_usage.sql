-- Tool usage metrics for agents
-- Tracks which tools agents use, query quality, and effectiveness
CREATE TABLE
  IF NOT EXISTS agent_tool_usage (
    agent_id UUID NOT NULL,
    agent_name String NOT NULL,
    personality LowCardinality (String) NOT NULL, -- conservative, aggressive, etc
    tool_name LowCardinality (String) NOT NULL, -- SearchNewsSemantics, GetHighImpactNews, etc
    query String, -- The actual query/parameters
    results_count UInt16, -- Number of results returned
    avg_similarity Float32, -- Average similarity score of results
    was_useful Boolean, -- Did agent act on results?
    execution_time_ms UInt32, -- Tool execution time
    timestamp DateTime DEFAULT now (),
    date Date DEFAULT toDate (timestamp) -- For partitioning
  ) ENGINE = MergeTree ()
PARTITION BY
  toYYYYMM (date)
ORDER BY
  (agent_id, tool_name, timestamp) TTL timestamp + INTERVAL 90 DAY;

-- Keep metrics for 90 days
-- Indexes for fast queries
CREATE INDEX IF NOT EXISTS idx_tool_usage_tool ON agent_tool_usage (tool_name) TYPE bloom_filter GRANULARITY 1;

CREATE INDEX IF NOT EXISTS idx_tool_usage_personality ON agent_tool_usage (personality) TYPE bloom_filter GRANULARITY 1;

-- Comments
COMMENT ON TABLE agent_tool_usage IS 'Tracks agent tool usage patterns for optimization and analysis';

COMMENT ON COLUMN agent_tool_usage.avg_similarity IS 'Average similarity score (0-1) for semantic search tools';

COMMENT ON COLUMN agent_tool_usage.was_useful IS 'True if agent made decision based on tool results';
