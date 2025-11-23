-- Add checkpoint support for interrupted Chain-of-Thought reasoning sessions
-- This allows agents to resume thinking after graceful shutdown/redeploy

ALTER TABLE agent_reasoning_sessions 
ADD COLUMN IF NOT EXISTS is_interrupted BOOLEAN NOT NULL DEFAULT false;

ALTER TABLE agent_reasoning_sessions 
ADD COLUMN IF NOT EXISTS checkpoint_state JSONB;

ALTER TABLE agent_reasoning_sessions 
ADD COLUMN IF NOT EXISTS checkpoint_history JSONB;

-- Index for finding interrupted sessions
CREATE INDEX IF NOT EXISTS idx_agent_reasoning_interrupted 
ON agent_reasoning_sessions(agent_id, is_interrupted) 
WHERE is_interrupted = true AND completed_at IS NULL;

COMMENT ON COLUMN agent_reasoning_sessions.is_interrupted IS 'True if thinking was interrupted by shutdown';
COMMENT ON COLUMN agent_reasoning_sessions.checkpoint_state IS 'ThinkingState snapshot for resume';
COMMENT ON COLUMN agent_reasoning_sessions.checkpoint_history IS 'ThoughtStep[] history for resume';

