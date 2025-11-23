-- Drop views
DROP VIEW IF EXISTS agent_learning_progress;
DROP VIEW IF EXISTS agent_memory_summary;

-- Drop tables in reverse order
DROP TABLE IF EXISTS agent_trading_plans;
DROP TABLE IF EXISTS agent_reflections;
DROP TABLE IF EXISTS agent_reasoning_sessions;
DROP TABLE IF EXISTS agent_semantic_memories;

-- Note: We don't drop the vector extension as it might be used by other tables
-- If you need to drop it: DROP EXTENSION IF EXISTS vector CASCADE;

