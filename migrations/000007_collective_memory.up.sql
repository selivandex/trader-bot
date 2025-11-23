-- Collective Memory System (shared across agents of same personality)
-- Note: vector extension already created in migration 000006
CREATE TABLE
    IF NOT EXISTS collective_agent_memories (
        id UUID PRIMARY KEY DEFAULT uuid_generate_v4 (),
        personality VARCHAR(50) NOT NULL, -- conservative, aggressive, etc
        context TEXT NOT NULL, -- Market situation
        action TEXT NOT NULL, -- What agents did
        lesson TEXT NOT NULL, -- Key takeaway
        embedding vector (1536) NOT NULL, -- OpenAI ada-002 embedding for semantic search
        importance DECIMAL(5, 4) NOT NULL, -- Average importance
        confirmation_count INTEGER NOT NULL DEFAULT 1, -- How many agents confirmed this
        success_rate DECIMAL(5, 4) NOT NULL DEFAULT 0.5, -- Win rate for this pattern
        last_confirmed_at TIMESTAMP NOT NULL DEFAULT NOW (),
        created_at TIMESTAMP NOT NULL DEFAULT NOW (),
        CONSTRAINT check_collective_importance CHECK (
            importance >= 0
            AND importance <= 1
        ),
        CONSTRAINT check_collective_success_rate CHECK (
            success_rate >= 0
            AND success_rate <= 1
        )
    );

-- Standard indexes
CREATE INDEX idx_collective_memories_personality ON collective_agent_memories (personality);

CREATE INDEX idx_collective_memories_importance ON collective_agent_memories (importance DESC);

CREATE INDEX idx_collective_memories_success_rate ON collective_agent_memories (success_rate DESC);

CREATE INDEX idx_collective_memories_confirmations ON collective_agent_memories (confirmation_count DESC);

-- 🚀 Vector index for fast semantic similarity search
-- Lists = 50 because collective memory typically has fewer records than personal
CREATE INDEX idx_collective_memories_embedding ON collective_agent_memories USING ivfflat (embedding vector_cosine_ops)
WITH
    (lists = 50);

-- Memory Confirmations (tracks which agents confirmed which lessons)
CREATE TABLE
    IF NOT EXISTS memory_confirmations (
        id UUID PRIMARY KEY DEFAULT uuid_generate_v4 (),
        collective_memory_id UUID NOT NULL REFERENCES collective_agent_memories (id) ON DELETE CASCADE,
        agent_id UUID NOT NULL REFERENCES agent_configs (id) ON DELETE CASCADE,
        was_successful BOOLEAN NOT NULL, -- Did this pattern work for this agent?
        trade_count INTEGER NOT NULL DEFAULT 1,
        pnl_sum DECIMAL(20, 8) NOT NULL DEFAULT 0,
        confirmed_at TIMESTAMP NOT NULL DEFAULT NOW (),
        UNIQUE (collective_memory_id, agent_id)
    );

CREATE INDEX idx_memory_confirmations_collective ON memory_confirmations (collective_memory_id);

CREATE INDEX idx_memory_confirmations_agent ON memory_confirmations (agent_id);

-- View: Best collective memories by personality
CREATE
OR REPLACE VIEW best_collective_memories AS
SELECT
    cm.id,
    cm.personality,
    cm.lesson,
    cm.confirmation_count,
    cm.success_rate,
    cm.importance,
    cm.created_at,
    CASE
        WHEN cm.confirmation_count >= 10
        AND cm.success_rate > 0.65 THEN 'proven'
        WHEN cm.confirmation_count >= 5
        AND cm.success_rate > 0.55 THEN 'validated'
        WHEN cm.confirmation_count >= 2
        AND cm.success_rate > 0.50 THEN 'emerging'
        ELSE 'unproven'
    END as reliability_tier
FROM
    collective_agent_memories cm
WHERE
    cm.confirmation_count >= 2
ORDER BY
    cm.personality,
    cm.success_rate DESC,
    cm.confirmation_count DESC;

COMMENT ON TABLE collective_agent_memories IS 'Shared wisdom across all agents of same personality';

COMMENT ON TABLE memory_confirmations IS 'Tracks which agents validated which collective lessons';

COMMENT ON VIEW best_collective_memories IS 'Ranked collective memories by reliability';
