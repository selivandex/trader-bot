-- Agent Configurations
CREATE TABLE IF NOT EXISTS agent_configs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    personality VARCHAR(50) NOT NULL,
    specialization JSONB NOT NULL,
    strategy JSONB NOT NULL,
    decision_interval BIGINT NOT NULL, -- nanoseconds
    min_news_impact DECIMAL(10, 2) NOT NULL DEFAULT 7.0,
    min_whale_transaction DECIMAL(20, 2) NOT NULL DEFAULT 10000000,
    invert_sentiment BOOLEAN NOT NULL DEFAULT false,
    learning_rate DECIMAL(5, 4) NOT NULL DEFAULT 0.10,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    
    CONSTRAINT check_learning_rate CHECK (learning_rate >= 0 AND learning_rate <= 1),
    CONSTRAINT check_min_news_impact CHECK (min_news_impact >= 0 AND min_news_impact <= 10)
);

CREATE INDEX idx_agent_configs_user_id ON agent_configs(user_id);
CREATE INDEX idx_agent_configs_personality ON agent_configs(personality);
CREATE INDEX idx_agent_configs_is_active ON agent_configs(is_active);
CREATE UNIQUE INDEX idx_agent_configs_user_name ON agent_configs(user_id, name);

-- Agent Trading States
CREATE TABLE IF NOT EXISTS agent_states (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    agent_id UUID NOT NULL REFERENCES agent_configs(id) ON DELETE CASCADE,
    symbol VARCHAR(20) NOT NULL,
    balance DECIMAL(20, 8) NOT NULL,
    initial_balance DECIMAL(20, 8) NOT NULL,
    equity DECIMAL(20, 8) NOT NULL,
    pnl DECIMAL(20, 8) NOT NULL DEFAULT 0,
    total_trades INTEGER NOT NULL DEFAULT 0,
    winning_trades INTEGER NOT NULL DEFAULT 0,
    losing_trades INTEGER NOT NULL DEFAULT 0,
    win_rate DECIMAL(5, 4) NOT NULL DEFAULT 0,
    is_trading BOOLEAN NOT NULL DEFAULT false,
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    
    CONSTRAINT check_balance CHECK (balance >= 0),
    CONSTRAINT check_win_rate CHECK (win_rate >= 0 AND win_rate <= 1),
    UNIQUE(agent_id, symbol)
);

CREATE INDEX idx_agent_states_agent_id ON agent_states(agent_id);
CREATE INDEX idx_agent_states_symbol ON agent_states(symbol);
CREATE INDEX idx_agent_states_is_trading ON agent_states(is_trading);

-- Agent Decisions
CREATE TABLE IF NOT EXISTS agent_decisions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    agent_id UUID NOT NULL REFERENCES agent_configs(id) ON DELETE CASCADE,
    symbol VARCHAR(20) NOT NULL,
    action VARCHAR(20) NOT NULL,
    confidence INTEGER NOT NULL,
    reason TEXT NOT NULL,
    technical_score DECIMAL(5, 2) NOT NULL,
    news_score DECIMAL(5, 2) NOT NULL,
    onchain_score DECIMAL(5, 2) NOT NULL,
    sentiment_score DECIMAL(5, 2) NOT NULL,
    final_score DECIMAL(5, 2) NOT NULL,
    market_data JSONB,
    executed BOOLEAN NOT NULL DEFAULT false,
    execution_price DECIMAL(20, 8),
    execution_size DECIMAL(20, 8),
    outcome JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    
    CONSTRAINT check_confidence CHECK (confidence >= 0 AND confidence <= 100),
    CONSTRAINT check_scores CHECK (
        technical_score >= 0 AND technical_score <= 100 AND
        news_score >= 0 AND news_score <= 100 AND
        onchain_score >= 0 AND onchain_score <= 100 AND
        sentiment_score >= 0 AND sentiment_score <= 100 AND
        final_score >= 0 AND final_score <= 100
    )
);

CREATE INDEX idx_agent_decisions_agent_id ON agent_decisions(agent_id);
CREATE INDEX idx_agent_decisions_symbol ON agent_decisions(symbol);
CREATE INDEX idx_agent_decisions_action ON agent_decisions(action);
CREATE INDEX idx_agent_decisions_executed ON agent_decisions(executed);
CREATE INDEX idx_agent_decisions_created_at ON agent_decisions(created_at DESC);

-- Agent Memory (Learning & Adaptation)
CREATE TABLE IF NOT EXISTS agent_memory (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    agent_id UUID NOT NULL UNIQUE REFERENCES agent_configs(id) ON DELETE CASCADE,
    technical_success_rate DECIMAL(5, 4) NOT NULL DEFAULT 0.5,
    news_success_rate DECIMAL(5, 4) NOT NULL DEFAULT 0.5,
    onchain_success_rate DECIMAL(5, 4) NOT NULL DEFAULT 0.5,
    sentiment_success_rate DECIMAL(5, 4) NOT NULL DEFAULT 0.5,
    best_market_conditions JSONB,
    worst_market_conditions JSONB,
    total_decisions INTEGER NOT NULL DEFAULT 0,
    adaptation_count INTEGER NOT NULL DEFAULT 0,
    last_adapted_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    
    CONSTRAINT check_success_rates CHECK (
        technical_success_rate >= 0 AND technical_success_rate <= 1 AND
        news_success_rate >= 0 AND news_success_rate <= 1 AND
        onchain_success_rate >= 0 AND onchain_success_rate <= 1 AND
        sentiment_success_rate >= 0 AND sentiment_success_rate <= 1
    )
);

CREATE INDEX idx_agent_memory_agent_id ON agent_memory(agent_id);

-- Agent Tournaments
CREATE TABLE IF NOT EXISTS agent_tournaments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(200) NOT NULL,
    symbols TEXT[] NOT NULL,
    start_balance DECIMAL(20, 8) NOT NULL,
    duration BIGINT NOT NULL, -- nanoseconds
    started_at TIMESTAMP NOT NULL DEFAULT NOW(),
    ended_at TIMESTAMP,
    is_active BOOLEAN NOT NULL DEFAULT true,
    winner_agent_id UUID REFERENCES agent_configs(id) ON DELETE SET NULL,
    results JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    
    CONSTRAINT check_start_balance CHECK (start_balance > 0)
);

CREATE INDEX idx_agent_tournaments_user_id ON agent_tournaments(user_id);
CREATE INDEX idx_agent_tournaments_is_active ON agent_tournaments(is_active);
CREATE INDEX idx_agent_tournaments_started_at ON agent_tournaments(started_at DESC);

-- Views for analytics

-- Agent Performance Summary
CREATE OR REPLACE VIEW agent_performance_summary AS
SELECT 
    ac.id as agent_id,
    ac.user_id,
    ac.name as agent_name,
    ac.personality,
    ac.is_active,
    ast.symbol,
    ast.balance,
    ast.initial_balance,
    ast.pnl,
    (ast.balance - ast.initial_balance) / ast.initial_balance * 100 as return_pct,
    ast.total_trades,
    ast.winning_trades,
    ast.losing_trades,
    ast.win_rate,
    am.technical_success_rate,
    am.news_success_rate,
    am.onchain_success_rate,
    am.sentiment_success_rate,
    am.adaptation_count,
    am.last_adapted_at
FROM agent_configs ac
LEFT JOIN agent_states ast ON ac.id = ast.agent_id
LEFT JOIN agent_memory am ON ac.id = am.agent_id;

-- Agent Decision Summary
CREATE OR REPLACE VIEW agent_decision_summary AS
SELECT 
    ac.id as agent_id,
    ac.name as agent_name,
    ac.personality,
    ad.symbol,
    COUNT(*) as total_decisions,
    COUNT(*) FILTER (WHERE ad.executed) as executed_decisions,
    COUNT(*) FILTER (WHERE ad.action = 'OPEN_LONG') as long_signals,
    COUNT(*) FILTER (WHERE ad.action = 'OPEN_SHORT') as short_signals,
    COUNT(*) FILTER (WHERE ad.action = 'HOLD') as hold_signals,
    COUNT(*) FILTER (WHERE ad.action = 'CLOSE') as close_signals,
    AVG(ad.confidence) as avg_confidence,
    AVG(ad.final_score) as avg_final_score,
    AVG(ad.technical_score) as avg_technical_score,
    AVG(ad.news_score) as avg_news_score,
    AVG(ad.onchain_score) as avg_onchain_score,
    AVG(ad.sentiment_score) as avg_sentiment_score
FROM agent_configs ac
LEFT JOIN agent_decisions ad ON ac.id = ad.agent_id
GROUP BY ac.id, ac.name, ac.personality, ad.symbol;

-- Tournament Leaderboard
CREATE OR REPLACE VIEW tournament_leaderboard AS
SELECT 
    at.id as tournament_id,
    at.name as tournament_name,
    jsonb_array_elements(at.results::jsonb) as agent_results
FROM agent_tournaments at
WHERE at.is_active = false AND at.results IS NOT NULL;

-- Agent-Symbol assignments (which agents trade which symbols)
-- Moved here from 000001 because it depends on agent_configs
CREATE TABLE IF NOT EXISTS agent_symbol_assignments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    agent_id UUID NOT NULL REFERENCES agent_configs(id) ON DELETE CASCADE,
    trading_pair_id UUID NOT NULL REFERENCES user_trading_pairs(id) ON DELETE CASCADE,
    budget DECIMAL(20, 8) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(agent_id, trading_pair_id)
);

CREATE INDEX idx_agent_assignments_user_id ON agent_symbol_assignments(user_id);
CREATE INDEX idx_agent_assignments_agent_id ON agent_symbol_assignments(agent_id);
CREATE INDEX idx_agent_assignments_active ON agent_symbol_assignments(is_active);

COMMENT ON TABLE agent_configs IS 'AI agent configurations with personality and strategy parameters';
COMMENT ON TABLE agent_states IS 'Current trading state for each agent-symbol pair';
COMMENT ON TABLE agent_decisions IS 'Historical decisions made by agents with weighted scores';
COMMENT ON TABLE agent_memory IS 'Learning data for agent adaptation';
COMMENT ON TABLE agent_tournaments IS 'Agent competitions with multiple participants';
COMMENT ON TABLE agent_symbol_assignments IS 'Which agents are trading which symbols for which users';

