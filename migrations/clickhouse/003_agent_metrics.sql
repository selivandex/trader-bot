-- Agent Performance Metrics (real-time monitoring and analytics)
CREATE TABLE
  IF NOT EXISTS agent_performance_metrics (
    agent_id String,
    agent_name String,
    personality LowCardinality (String), -- 8 personalities only
    symbol LowCardinality (String), -- Limited trading pairs
    timestamp DateTime DEFAULT now (),
    -- Decision metrics
    decisions_total UInt32,
    decisions_hold UInt32,
    decisions_open UInt32,
    decisions_close UInt32,
    -- Validator Council metrics
    validator_calls UInt32,
    validator_approvals UInt32,
    validator_rejections UInt32,
    validator_approval_rate Float32,
    -- Chain-of-Thought metrics
    cot_iterations_avg Float32,
    cot_tools_used_avg Float32,
    cot_thinking_time_seconds Float32,
    -- Tool usage breakdown
    tools_search_news UInt32,
    tools_get_candles UInt32,
    tools_calculate_indicators UInt32,
    tools_whale_movements UInt32,
    tools_personal_memory UInt32,
    -- Cost tracking
    ai_cost_usd Float32,
    validator_cost_usd Float32,
    total_cost_usd Float32,
    -- Trading performance
    balance Float64,
    equity Float64,
    pnl Float64,
    pnl_percent Float32,
    total_trades UInt32,
    winning_trades UInt32,
    losing_trades UInt32,
    win_rate Float32,
    -- Signal performance
    technical_score_avg Float32,
    news_score_avg Float32,
    onchain_score_avg Float32,
    sentiment_score_avg Float32,
    -- Risk metrics
    max_drawdown Float32,
    sharpe_ratio Float32,
    avg_leverage Float32,
    -- System metrics
    uptime_seconds UInt32,
    errors_count UInt32,
    last_error String
  ) ENGINE = MergeTree ()
PARTITION BY
  toYYYYMM (timestamp)
ORDER BY
  (agent_id, symbol, timestamp) TTL timestamp + INTERVAL 90 DAY;

-- Keep metrics for 90 days
-- Agent Decision Events (every decision logged)
CREATE TABLE
  IF NOT EXISTS agent_decision_events (
    agent_id String,
    decision_id String,
    timestamp DateTime DEFAULT now (),
    symbol LowCardinality (String),
    action LowCardinality (String), -- HOLD, OPEN_LONG, OPEN_SHORT, CLOSE
    confidence UInt8,
    -- Scores
    technical_score Float32,
    news_score Float32,
    onchain_score Float32,
    sentiment_score Float32,
    final_score Float32,
    -- Execution details
    executed Bool,
    order_id String,
    stop_loss_order_id String,
    take_profit_order_id String,
    execution_price Float64,
    execution_size Float64,
    -- Validator decision
    validated Bool,
    validator_approved Bool,
    validator_approval_rate Float32,
    -- CoT trace
    cot_iterations UInt16,
    cot_tools_used UInt16,
    cot_thinking_time_ms UInt32,
    -- Cost
    decision_cost_usd Float32,
    -- Result (filled when position closes)
    closed_at Nullable (DateTime),
    pnl Nullable (Float64),
    pnl_percent Nullable (Float32),
    duration_seconds Nullable (UInt32),
    exit_reason LowCardinality (String) -- 'take_profit', 'stop_loss', 'manual_close'
  ) ENGINE = MergeTree ()
PARTITION BY
  toYYYYMM (timestamp)
ORDER BY
  (agent_id, timestamp) TTL timestamp + INTERVAL 180 DAY;

-- Keep decision history for 6 months
-- Validator Council Events
CREATE TABLE
  IF NOT EXISTS validator_council_events (
    agent_id String,
    decision_id String,
    timestamp DateTime DEFAULT now (),
    validator_role LowCardinality (String), -- 'technical_expert', 'risk_manager', 'market_psychologist'
    provider_name LowCardinality (String), -- 'claude', 'deepseek', 'openai'
    verdict LowCardinality (String), -- 'approve', 'reject', 'abstain'
    confidence UInt8,
    reasoning String,
    processing_time_ms UInt32
  ) ENGINE = MergeTree ()
PARTITION BY
  toYYYYMM (timestamp)
ORDER BY
  (agent_id, timestamp) TTL timestamp + INTERVAL 90 DAY;

-- Tool Usage Events (track which tools agents use)
CREATE TABLE
  IF NOT EXISTS agent_tool_usage (
    agent_id String,
    timestamp DateTime DEFAULT now (),
    tool_name LowCardinality (String), -- Limited set of tools
    parameters String, -- JSON string
    result_summary String,
    execution_time_ms UInt32,
    success Bool,
    error String
  ) ENGINE = MergeTree ()
PARTITION BY
  toYYYYMM (timestamp)
ORDER BY
  (agent_id, tool_name, timestamp) TTL timestamp + INTERVAL 30 DAY;

-- Materialized view for real-time agent monitoring (5-minute aggregates)
CREATE MATERIALIZED VIEW IF NOT EXISTS agent_metrics_5min ENGINE = SummingMergeTree ()
PARTITION BY
  toYYYYMM (timestamp)
ORDER BY
  (agent_id, symbol, timestamp) POPULATE AS
SELECT
  agent_id,
  symbol,
  toStartOfFiveMinutes (timestamp) as timestamp,
  count() as decisions_total,
  countIf (action = 'HOLD') as decisions_hold,
  countIf (
    action = 'OPEN_LONG'
    OR action = 'OPEN_SHORT'
  ) as decisions_open,
  countIf (action = 'CLOSE') as decisions_close,
  countIf (executed = 1) as executed_count,
  avg(confidence) as avg_confidence,
  sum(decision_cost_usd) as total_cost_usd
FROM
  agent_decision_events
GROUP BY
  agent_id,
  symbol,
  timestamp;

-- Materialized view for hourly tool usage stats
CREATE MATERIALIZED VIEW IF NOT EXISTS tool_usage_hourly ENGINE = SummingMergeTree ()
PARTITION BY
  toYYYYMM (timestamp)
ORDER BY
  (agent_id, tool_name, timestamp) POPULATE AS
SELECT
  agent_id,
  tool_name,
  toStartOfHour (timestamp) as timestamp,
  count() as usage_count,
  avg(execution_time_ms) as avg_execution_time,
  countIf (success = 1) as success_count,
  countIf (success = 0) as error_count
FROM
  agent_tool_usage
GROUP BY
  agent_id,
  tool_name,
  timestamp;
