-- Rollback initial schema

-- Drop views first (they depend on tables)
DROP VIEW IF EXISTS ai_provider_stats_by_user;
DROP VIEW IF EXISTS open_positions_by_user;
DROP VIEW IF EXISTS recent_trades_by_user;
DROP VIEW IF EXISTS user_overview;

-- Drop triggers
DROP TRIGGER IF EXISTS trigger_update_users_timestamp ON users;
DROP TRIGGER IF EXISTS trigger_update_user_configs_timestamp ON user_configs;
DROP TRIGGER IF EXISTS trigger_update_user_states_timestamp ON user_states;

-- Drop functions
DROP FUNCTION IF EXISTS calculate_daily_metrics(INTEGER, DATE);
DROP FUNCTION IF EXISTS update_user_timestamp();

-- Drop tables (in reverse order of creation to respect foreign keys)
DROP TABLE IF EXISTS user_sessions;
DROP TABLE IF EXISTS performance_metrics;
DROP TABLE IF EXISTS risk_events;
DROP TABLE IF EXISTS positions;
DROP TABLE IF EXISTS ai_decisions;
DROP TABLE IF EXISTS trades;
DROP TABLE IF EXISTS user_states;
DROP TABLE IF EXISTS user_configs;
DROP TABLE IF EXISTS users;

