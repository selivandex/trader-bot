package users

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/selivandex/trader-bot/pkg/models"
)

// AdminRepository provides admin-level access to system data
type AdminRepository struct {
	db *sqlx.DB
}

// NewAdminRepository creates new admin repository
func NewAdminRepository(db *sqlx.DB) *AdminRepository {
	return &AdminRepository{db: db}
}

// SystemStats represents overall system statistics
type SystemStats struct {
	TopPerformingAgent   *AgentPerformance
	WorstPerformingAgent *AgentPerformance
	TotalUsers           int
	ActiveUsers          int
	TotalAgents          int
	ActiveAgents         int
	TotalTrades          int
	ProfitableTrades     int
	TotalVolume          float64
	TotalProfit          float64
}

// AgentPerformance represents agent performance metrics
type AgentPerformance struct {
	LastTradeAt time.Time
	AgentID     string
	AgentName   string
	UserID      string
	Username    string
	Personality models.AgentPersonality
	TotalTrades int
	WinRate     float64
	TotalProfit float64
	AvgProfit   float64
	MaxDrawdown float64
	Sharpe      float64
}

// UserStats represents user statistics
type UserStats struct {
	RegisteredAt time.Time
	UserID       string
	Username     string
	TelegramID   int64
	TotalAgents  int
	ActiveAgents int
	TotalTrades  int
	TotalProfit  float64
	IsBanned     bool
}

// NewsStats represents news aggregation statistics
type NewsStats struct {
	TopKeywords       []KeywordCount
	TotalNews         int
	Last24h           int
	PositiveSentiment int
	NegativeSentiment int
	NeutralSentiment  int
}

// KeywordCount represents keyword frequency
type KeywordCount struct {
	Keyword string
	Count   int
}

// TradeStats represents trading statistics
type TradeStats struct {
	ByExchange   map[string]int
	BySymbol     map[string]int
	TotalTrades  int
	Last24h      int
	Last7d       int
	Last30d      int
	AvgProfit    float64
	MedianProfit float64
	TotalVolume  float64
}

// GetSystemStats returns overall system statistics
func (r *AdminRepository) GetSystemStats(ctx context.Context) (*SystemStats, error) {
	stats := &SystemStats{}

	// Total users
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&stats.TotalUsers)
	if err != nil {
		return nil, fmt.Errorf("failed to count users: %w", err)
	}

	// Active users (users with at least one agent)
	err = r.db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT user_id) 
		FROM agent_configs
	`).Scan(&stats.ActiveUsers)
	if err != nil {
		return nil, fmt.Errorf("failed to count active users: %w", err)
	}

	// Total agents
	err = r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM agent_configs").Scan(&stats.TotalAgents)
	if err != nil {
		return nil, fmt.Errorf("failed to count agents: %w", err)
	}

	// Active agents (currently running)
	err = r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) 
		FROM agent_configs 
		WHERE is_active = true
	`).Scan(&stats.ActiveAgents)
	if err != nil {
		return nil, fmt.Errorf("failed to count active agents: %w", err)
	}

	// Trading statistics
	err = r.db.QueryRowContext(ctx, `
		SELECT 
			COALESCE(COUNT(*), 0),
			COALESCE(SUM(CASE WHEN profit > 0 THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(volume), 0),
			COALESCE(SUM(profit), 0)
		FROM agent_trades
	`).Scan(&stats.TotalTrades, &stats.ProfitableTrades, &stats.TotalVolume, &stats.TotalProfit)
	if err != nil {
		return nil, fmt.Errorf("failed to get trade stats: %w", err)
	}

	// Top performing agent
	stats.TopPerformingAgent, _ = r.getTopAgent(ctx, true)
	stats.WorstPerformingAgent, _ = r.getTopAgent(ctx, false)

	return stats, nil
}

// getTopAgent returns top or worst performing agent
func (r *AdminRepository) getTopAgent(ctx context.Context, best bool) (*AgentPerformance, error) {
	order := "DESC"
	if !best {
		order = "ASC"
	}

	query := fmt.Sprintf(`
		SELECT 
			ac.id,
			ac.name,
			ac.user_id,
			u.username,
			ac.personality,
			COUNT(at.id) as total_trades,
			COALESCE(SUM(CASE WHEN at.profit > 0 THEN 1 ELSE 0 END)::float / NULLIF(COUNT(at.id), 0), 0) as win_rate,
			COALESCE(SUM(at.profit), 0) as total_profit,
			COALESCE(AVG(at.profit), 0) as avg_profit,
			MAX(at.executed_at) as last_trade_at
		FROM agent_configs ac
		JOIN users u ON u.id = ac.user_id
		LEFT JOIN agent_trades at ON at.agent_id = ac.id
		GROUP BY ac.id, u.username
		HAVING COUNT(at.id) > 0
		ORDER BY total_profit %s
		LIMIT 1
	`, order)

	perf := &AgentPerformance{}
	err := r.db.QueryRowContext(ctx, query).Scan(
		&perf.AgentID,
		&perf.AgentName,
		&perf.UserID,
		&perf.Username,
		&perf.Personality,
		&perf.TotalTrades,
		&perf.WinRate,
		&perf.TotalProfit,
		&perf.AvgProfit,
		&perf.LastTradeAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return perf, nil
}

// GetAllUsers returns all users with statistics
func (r *AdminRepository) GetAllUsers(ctx context.Context) ([]UserStats, error) {
	query := `
		SELECT 
			u.id,
			u.username,
			u.telegram_id,
			u.created_at,
			COALESCE(COUNT(DISTINCT ac.id), 0) as total_agents,
			COALESCE(SUM(CASE WHEN ac.is_active THEN 1 ELSE 0 END), 0) as active_agents,
			COALESCE(COUNT(DISTINCT at.id), 0) as total_trades,
			COALESCE(SUM(at.profit), 0) as total_profit,
			u.is_banned
		FROM users u
		LEFT JOIN agent_configs ac ON ac.user_id = u.id
		LEFT JOIN agent_trades at ON at.agent_id = ac.id
		GROUP BY u.id
		ORDER BY u.created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query users: %w", err)
	}
	defer rows.Close()

	var users []UserStats
	for rows.Next() {
		var u UserStats
		err := rows.Scan(
			&u.UserID,
			&u.Username,
			&u.TelegramID,
			&u.RegisteredAt,
			&u.TotalAgents,
			&u.ActiveAgents,
			&u.TotalTrades,
			&u.TotalProfit,
			&u.IsBanned,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, u)
	}

	return users, nil
}

// GetAllAgents returns all agents with performance metrics
func (r *AdminRepository) GetAllAgents(ctx context.Context) ([]AgentPerformance, error) {
	query := `
		SELECT 
			ac.id,
			ac.name,
			ac.user_id,
			u.username,
			ac.personality,
			COUNT(at.id) as total_trades,
			COALESCE(SUM(CASE WHEN at.profit > 0 THEN 1 ELSE 0 END)::float / NULLIF(COUNT(at.id), 0), 0) as win_rate,
			COALESCE(SUM(at.profit), 0) as total_profit,
			COALESCE(AVG(at.profit), 0) as avg_profit,
			COALESCE(MAX(at.executed_at), ac.created_at) as last_trade_at
		FROM agent_configs ac
		JOIN users u ON u.id = ac.user_id
		LEFT JOIN agent_trades at ON at.agent_id = ac.id
		GROUP BY ac.id, u.username
		ORDER BY total_profit DESC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query agents: %w", err)
	}
	defer rows.Close()

	var agents []AgentPerformance
	for rows.Next() {
		var a AgentPerformance
		err := rows.Scan(
			&a.AgentID,
			&a.AgentName,
			&a.UserID,
			&a.Username,
			&a.Personality,
			&a.TotalTrades,
			&a.WinRate,
			&a.TotalProfit,
			&a.AvgProfit,
			&a.LastTradeAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan agent: %w", err)
		}
		agents = append(agents, a)
	}

	return agents, nil
}

// GetNewsStats returns news aggregation statistics
func (r *AdminRepository) GetNewsStats(ctx context.Context) (*NewsStats, error) {
	stats := &NewsStats{}

	// Total news
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM news_cache").Scan(&stats.TotalNews)
	if err != nil {
		return nil, fmt.Errorf("failed to count news: %w", err)
	}

	// Last 24h
	err = r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) 
		FROM news_cache 
		WHERE published_at > NOW() - INTERVAL '24 hours'
	`).Scan(&stats.Last24h)
	if err != nil {
		return nil, fmt.Errorf("failed to count recent news: %w", err)
	}

	// Sentiment distribution
	err = r.db.QueryRowContext(ctx, `
		SELECT 
			COALESCE(SUM(CASE WHEN sentiment > 0.2 THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN sentiment < -0.2 THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN sentiment BETWEEN -0.2 AND 0.2 THEN 1 ELSE 0 END), 0)
		FROM sentiment_scores
		WHERE created_at > NOW() - INTERVAL '24 hours'
	`).Scan(&stats.PositiveSentiment, &stats.NegativeSentiment, &stats.NeutralSentiment)
	if err != nil {
		return nil, fmt.Errorf("failed to get sentiment: %w", err)
	}

	return stats, nil
}

// GetTradeStats returns trading statistics
func (r *AdminRepository) GetTradeStats(ctx context.Context) (*TradeStats, error) {
	stats := &TradeStats{
		ByExchange: make(map[string]int),
		BySymbol:   make(map[string]int),
	}

	// Overall statistics
	err := r.db.QueryRowContext(ctx, `
		SELECT 
			COUNT(*),
			COALESCE(AVG(profit), 0),
			COALESCE(SUM(volume), 0)
		FROM agent_trades
	`).Scan(&stats.TotalTrades, &stats.AvgProfit, &stats.TotalVolume)
	if err != nil {
		return nil, fmt.Errorf("failed to get trade stats: %w", err)
	}

	// Recent trades
	err = r.db.QueryRowContext(ctx, `
		SELECT 
			COALESCE(SUM(CASE WHEN executed_at > NOW() - INTERVAL '24 hours' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN executed_at > NOW() - INTERVAL '7 days' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN executed_at > NOW() - INTERVAL '30 days' THEN 1 ELSE 0 END), 0)
		FROM agent_trades
	`).Scan(&stats.Last24h, &stats.Last7d, &stats.Last30d)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent trades: %w", err)
	}

	// By exchange
	rows, err := r.db.QueryContext(ctx, `
		SELECT exchange, COUNT(*) 
		FROM agent_trades 
		GROUP BY exchange
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to get trades by exchange: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var exchange string
		var count int
		if err := rows.Scan(&exchange, &count); err != nil {
			return nil, err
		}
		stats.ByExchange[exchange] = count
	}

	// By symbol
	rows2, err := r.db.QueryContext(ctx, `
		SELECT symbol, COUNT(*) 
		FROM agent_trades 
		GROUP BY symbol
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to get trades by symbol: %w", err)
	}
	defer rows2.Close()

	for rows2.Next() {
		var symbol string
		var count int
		if err := rows2.Scan(&symbol, &count); err != nil {
			return nil, err
		}
		stats.BySymbol[symbol] = count
	}

	return stats, nil
}

// BanUser bans a user (prevents agent operations)
func (r *AdminRepository) BanUser(ctx context.Context, userID string, reason string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE users 
		SET is_banned = true, ban_reason = $1, banned_at = NOW() 
		WHERE id = $2
	`, reason, userID)
	if err != nil {
		return fmt.Errorf("failed to ban user: %w", err)
	}

	// Stop all user's agents
	_, err = r.db.ExecContext(ctx, `
		UPDATE agent_configs 
		SET is_active = false 
		WHERE user_id = $1
	`, userID)
	if err != nil {
		return fmt.Errorf("failed to stop user agents: %w", err)
	}

	return nil
}

// UnbanUser unbans a user
func (r *AdminRepository) UnbanUser(ctx context.Context, userID string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE users 
		SET is_banned = false, ban_reason = NULL, banned_at = NULL 
		WHERE id = $1
	`, userID)
	return err
}

// DeactivateUser deactivates user account (soft disable, not a ban)
func (r *AdminRepository) DeactivateUser(ctx context.Context, userID string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE users 
		SET is_active = false 
		WHERE id = $1
	`, userID)
	if err != nil {
		return fmt.Errorf("failed to deactivate user: %w", err)
	}

	// Stop all user's agents
	_, err = r.db.ExecContext(ctx, `
		UPDATE agent_configs 
		SET is_active = false 
		WHERE user_id = $1
	`, userID)
	if err != nil {
		return fmt.Errorf("failed to stop user agents: %w", err)
	}

	return nil
}

// ActivateUser activates user account
func (r *AdminRepository) ActivateUser(ctx context.Context, userID string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE users 
		SET is_active = true 
		WHERE id = $1
	`, userID)
	return err
}

// StopAgentByAdmin stops any agent (admin override)
func (r *AdminRepository) StopAgentByAdmin(ctx context.Context, agentID string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE agent_configs 
		SET is_active = false 
		WHERE id = $1
	`, agentID)
	return err
}

// GetUserByID returns user by ID (admin view with all details)
func (r *AdminRepository) GetUserByID(ctx context.Context, userID string) (*UserStats, error) {
	query := `
		SELECT 
			u.id,
			u.username,
			u.telegram_id,
			u.created_at,
			COALESCE(COUNT(DISTINCT ac.id), 0) as total_agents,
			COALESCE(SUM(CASE WHEN ac.is_active THEN 1 ELSE 0 END), 0) as active_agents,
			COALESCE(COUNT(DISTINCT at.id), 0) as total_trades,
			COALESCE(SUM(at.profit), 0) as total_profit,
			u.is_banned
		FROM users u
		LEFT JOIN agent_configs ac ON ac.user_id = u.id
		LEFT JOIN agent_trades at ON at.agent_id = ac.id
		WHERE u.id = $1
		GROUP BY u.id
	`

	var u UserStats
	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&u.UserID,
		&u.Username,
		&u.TelegramID,
		&u.RegisteredAt,
		&u.TotalAgents,
		&u.ActiveAgents,
		&u.TotalTrades,
		&u.TotalProfit,
		&u.IsBanned,
	)
	if err != nil {
		return nil, err
	}

	return &u, nil
}
