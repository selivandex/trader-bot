-- Rollback checkpoint support

DROP INDEX IF EXISTS idx_agent_reasoning_interrupted;

ALTER TABLE agent_reasoning_sessions DROP COLUMN IF EXISTS checkpoint_history;
ALTER TABLE agent_reasoning_sessions DROP COLUMN IF EXISTS checkpoint_state;
ALTER TABLE agent_reasoning_sessions DROP COLUMN IF EXISTS is_interrupted;

