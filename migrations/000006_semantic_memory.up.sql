-- Enable pgvector extension for semantic similarity search
CREATE EXTENSION IF NOT EXISTS vector;

-- Semantic Memory for Agents (Episodic Memory System)
CREATE TABLE IF NOT EXISTS agent_semantic_memories (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    agent_id UUID NOT NULL REFERENCES agent_configs(id) ON DELETE CASCADE,
    context TEXT NOT NULL,           -- "BTC dropped 5% on ETF rejection news"
    action TEXT NOT NULL,             -- "Went short at $42k with 3x leverage"
    outcome TEXT NOT NULL,            -- "Profit +3.2%, good call"
    lesson TEXT NOT NULL,             -- "News-driven drops are good short opportunities"
    embedding vector(1536) NOT NULL,  -- OpenAI ada-002 embedding (1536 dimensions)
    importance DECIMAL(5, 4) NOT NULL DEFAULT 0.5, -- 0.0 - 1.0
    access_count INTEGER NOT NULL DEFAULT 0,
    last_accessed TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    
    CONSTRAINT check_importance CHECK (importance >= 0 AND importance <= 1)
);

-- Standard indexes
CREATE INDEX idx_agent_semantic_memories_agent_id ON agent_semantic_memories(agent_id);
CREATE INDEX idx_agent_semantic_memories_importance ON agent_semantic_memories(importance DESC);
CREATE INDEX idx_agent_semantic_memories_created_at ON agent_semantic_memories(created_at DESC);
CREATE INDEX idx_agent_semantic_memories_access_count ON agent_semantic_memories(access_count DESC);

-- 🚀 Vector index for fast semantic similarity search (cosine distance)
-- IVFFlat index: splits vector space into ~100 clusters for O(sqrt(n)) search
-- vector_cosine_ops: optimized for cosine similarity (1 - cosine_distance)
CREATE INDEX idx_agent_semantic_memories_embedding ON agent_semantic_memories 
USING ivfflat (embedding vector_cosine_ops) 
WITH (lists = 100);

-- Reasoning Sessions (tracks agent's thinking process)
CREATE TABLE IF NOT EXISTS agent_reasoning_sessions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id VARCHAR(100) NOT NULL UNIQUE,
    agent_id UUID NOT NULL REFERENCES agent_configs(id) ON DELETE CASCADE,
    observation TEXT NOT NULL,
    recalled_memories JSONB,          -- Array of memory IDs and content
    generated_options JSONB NOT NULL,  -- Array of trading options
    evaluations JSONB NOT NULL,        -- Array of option evaluations
    final_reasoning TEXT NOT NULL,
    decision JSONB NOT NULL,           -- Final AI decision
    chain_of_thought JSONB,            -- Step-by-step thinking
    executed BOOLEAN NOT NULL DEFAULT false,
    started_at TIMESTAMP NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMP,
    duration_ms INTEGER
);

CREATE INDEX idx_agent_reasoning_sessions_agent_id ON agent_reasoning_sessions(agent_id);
CREATE INDEX idx_agent_reasoning_sessions_session_id ON agent_reasoning_sessions(session_id);
CREATE INDEX idx_agent_reasoning_sessions_started_at ON agent_reasoning_sessions(started_at DESC);
CREATE INDEX idx_agent_reasoning_sessions_executed ON agent_reasoning_sessions(executed);

-- Reflections (agent's self-analysis after trades)
CREATE TABLE IF NOT EXISTS agent_reflections (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    agent_id UUID NOT NULL REFERENCES agent_configs(id) ON DELETE CASCADE,
    trade_id UUID,  -- Reference to trade if exists
    analysis TEXT NOT NULL,
    what_worked TEXT[],
    what_didnt_work TEXT[],
    key_lessons TEXT[],
    suggested_adjustments JSONB,      -- Map of adjustments
    confidence_in_analysis DECIMAL(5, 4),
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_agent_reflections_agent_id ON agent_reflections(agent_id);
CREATE INDEX idx_agent_reflections_created_at ON agent_reflections(created_at DESC);

-- Trading Plans (agent's forward-looking strategies)
CREATE TABLE IF NOT EXISTS agent_trading_plans (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    plan_id VARCHAR(100) NOT NULL UNIQUE,
    agent_id UUID NOT NULL REFERENCES agent_configs(id) ON DELETE CASCADE,
    time_horizon BIGINT NOT NULL,     -- Duration in nanoseconds
    assumptions TEXT[],
    scenarios JSONB NOT NULL,          -- Array of scenarios
    risk_limits JSONB NOT NULL,
    trigger_signals JSONB,             -- When to revise plan
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMP NOT NULL,
    
    CONSTRAINT check_plan_status CHECK (status IN ('active', 'executed', 'cancelled', 'expired'))
);

CREATE INDEX idx_agent_trading_plans_agent_id ON agent_trading_plans(agent_id);
CREATE INDEX idx_agent_trading_plans_status ON agent_trading_plans(status);
CREATE INDEX idx_agent_trading_plans_expires_at ON agent_trading_plans(expires_at);

-- Views for analytics

-- Agent Memory Summary
CREATE OR REPLACE VIEW agent_memory_summary AS
SELECT 
    ac.id as agent_id,
    ac.name as agent_name,
    COUNT(DISTINCT asm.id) as total_memories,
    AVG(asm.importance) as avg_importance,
    SUM(asm.access_count) as total_accesses,
    MAX(asm.last_accessed) as last_memory_accessed,
    COUNT(DISTINCT ars.id) as total_reasoning_sessions,
    COUNT(DISTINCT ar.id) as total_reflections,
    COUNT(DISTINCT atp.id) as active_plans
FROM agent_configs ac
LEFT JOIN agent_semantic_memories asm ON ac.id = asm.agent_id
LEFT JOIN agent_reasoning_sessions ars ON ac.id = ars.agent_id
LEFT JOIN agent_reflections ar ON ac.id = ar.agent_id
LEFT JOIN agent_trading_plans atp ON ac.id = atp.agent_id AND atp.status = 'active'
GROUP BY ac.id, ac.name;

-- Agent Learning Progress
CREATE OR REPLACE VIEW agent_learning_progress AS
SELECT 
    ac.id as agent_id,
    ac.name as agent_name,
    ac.personality,
    COUNT(DISTINCT asm.id) FILTER (WHERE asm.created_at > NOW() - INTERVAL '7 days') as memories_last_7d,
    COUNT(DISTINCT ar.id) FILTER (WHERE ar.created_at > NOW() - INTERVAL '7 days') as reflections_last_7d,
    AVG(asm.importance) FILTER (WHERE asm.created_at > NOW() - INTERVAL '30 days') as recent_memory_importance,
    am.adaptation_count,
    am.total_decisions
FROM agent_configs ac
LEFT JOIN agent_semantic_memories asm ON ac.id = asm.agent_id
LEFT JOIN agent_reflections ar ON ac.id = ar.agent_id
LEFT JOIN agent_memory am ON ac.id = am.agent_id
GROUP BY ac.id, ac.name, ac.personality, am.adaptation_count, am.total_decisions;

COMMENT ON TABLE agent_semantic_memories IS 'Episodic memory system for autonomous AI agents - stores past experiences';
COMMENT ON TABLE agent_reasoning_sessions IS 'Complete Chain-of-Thought reasoning traces for agent decisions';
COMMENT ON TABLE agent_reflections IS 'Agent self-analysis and learning from past trades';
COMMENT ON TABLE agent_trading_plans IS 'Forward-looking trading plans with multiple scenarios';

