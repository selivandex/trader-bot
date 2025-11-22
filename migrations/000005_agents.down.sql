-- Drop views
DROP VIEW IF EXISTS tournament_leaderboard;
DROP VIEW IF EXISTS agent_decision_summary;
DROP VIEW IF EXISTS agent_performance_summary;

-- Drop tables in reverse order (respecting foreign keys)
DROP TABLE IF EXISTS agent_tournaments;
DROP TABLE IF EXISTS agent_memory;
DROP TABLE IF EXISTS agent_decisions;
DROP TABLE IF EXISTS agent_states;
DROP TABLE IF EXISTS agent_configs;

