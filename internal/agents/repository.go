package agents

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"

	"github.com/selivandex/trader-bot/pkg/models"
)

// Repository handles database operations for agents
type Repository struct {
	db *sqlx.DB
}

// NewRepository creates new agent repository
func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{db: db}
}

// CreateAgent creates a new agent configuration
func (r *Repository) CreateAgent(ctx context.Context, config *models.AgentConfig) (*models.AgentConfig, error) {
	// Validate specialization
	if err := config.Specialization.Validate(); err != nil {
		return nil, fmt.Errorf("invalid specialization: %w", err)
	}

	// Marshal JSON fields
	specializationJSON, err := json.Marshal(config.Specialization)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal specialization: %w", err)
	}

	strategyJSON, err := json.Marshal(config.Strategy)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal strategy: %w", err)
	}

	query := `
		INSERT INTO agent_configs (
			user_id, name, personality, specialization, strategy,
			decision_interval, min_news_impact, min_whale_transaction,
			invert_sentiment, learning_rate, is_active
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, created_at, updated_at
	`

	err = r.db.QueryRowContext(
		ctx, query,
		config.UserID,
		config.Name,
		config.Personality,
		specializationJSON,
		strategyJSON,
		config.DecisionInterval,
		config.MinNewsImpact,
		config.MinWhaleTransaction,
		config.InvertSentiment,
		config.LearningRate,
		config.IsActive,
	).Scan(&config.ID, &config.CreatedAt, &config.UpdatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to create agent: %w", err)
	}

	// Initialize agent memory
	if err := r.initializeAgentMemory(ctx, config.ID); err != nil {
		return nil, fmt.Errorf("failed to initialize agent memory: %w", err)
	}

	return config, nil
}

// GetAgent retrieves agent configuration by ID
func (r *Repository) GetAgent(ctx context.Context, agentID string) (*models.AgentConfig, error) {
	query := `
		SELECT id, user_id, name, personality, specialization, strategy,
		       decision_interval, min_news_impact, min_whale_transaction,
		       invert_sentiment, learning_rate, is_active, created_at, updated_at
		FROM agent_configs
		WHERE id = $1
	`

	var config models.AgentConfig
	var specializationJSON, strategyJSON []byte

	err := r.db.QueryRowContext(ctx, query, agentID).Scan(
		&config.ID,
		&config.UserID,
		&config.Name,
		&config.Personality,
		&specializationJSON,
		&strategyJSON,
		&config.DecisionInterval,
		&config.MinNewsImpact,
		&config.MinWhaleTransaction,
		&config.InvertSentiment,
		&config.LearningRate,
		&config.IsActive,
		&config.CreatedAt,
		&config.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("agent not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get agent: %w", err)
	}

	// Unmarshal JSON fields
	if err := json.Unmarshal(specializationJSON, &config.Specialization); err != nil {
		return nil, fmt.Errorf("failed to unmarshal specialization: %w", err)
	}

	if err := json.Unmarshal(strategyJSON, &config.Strategy); err != nil {
		return nil, fmt.Errorf("failed to unmarshal strategy: %w", err)
	}

	return &config, nil
}

// GetUserAgents retrieves all agents for a user
func (r *Repository) GetUserAgents(ctx context.Context, userID string) ([]*models.AgentConfig, error) {
	query := `
		SELECT id, user_id, name, personality, specialization, strategy,
		       decision_interval, min_news_impact, min_whale_transaction,
		       invert_sentiment, learning_rate, is_active, created_at, updated_at
		FROM agent_configs
		WHERE user_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query agents: %w", err)
	}
	defer rows.Close()

	var agents []*models.AgentConfig
	for rows.Next() {
		var config models.AgentConfig
		var specializationJSON, strategyJSON []byte

		err := rows.Scan(
			&config.ID,
			&config.UserID,
			&config.Name,
			&config.Personality,
			&specializationJSON,
			&strategyJSON,
			&config.DecisionInterval,
			&config.MinNewsImpact,
			&config.MinWhaleTransaction,
			&config.InvertSentiment,
			&config.LearningRate,
			&config.IsActive,
			&config.CreatedAt,
			&config.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan agent: %w", err)
		}

		if err := json.Unmarshal(specializationJSON, &config.Specialization); err != nil {
			return nil, fmt.Errorf("failed to unmarshal specialization: %w", err)
		}

		if err := json.Unmarshal(strategyJSON, &config.Strategy); err != nil {
			return nil, fmt.Errorf("failed to unmarshal strategy: %w", err)
		}

		agents = append(agents, &config)
	}

	return agents, nil
}

// UpdateAgentStatus updates agent's active status
func (r *Repository) UpdateAgentStatus(ctx context.Context, agentID string, isActive bool) error {
	query := `
		UPDATE agent_configs
		SET is_active = $1, updated_at = NOW()
		WHERE id = $2
	`

	result, err := r.db.ExecContext(ctx, query, isActive, agentID)
	if err != nil {
		return fmt.Errorf("failed to update agent status: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("agent not found")
	}

	return nil
}

// CreateAgentState creates or updates agent trading state
func (r *Repository) CreateAgentState(ctx context.Context, state *models.AgentState) error {
	query := `
		INSERT INTO agent_states (
			agent_id, symbol, balance, initial_balance, equity, pnl,
			total_trades, winning_trades, losing_trades, win_rate, is_trading
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (agent_id, symbol)
		DO UPDATE SET
			balance = EXCLUDED.balance,
			equity = EXCLUDED.equity,
			pnl = EXCLUDED.pnl,
			total_trades = EXCLUDED.total_trades,
			winning_trades = EXCLUDED.winning_trades,
			losing_trades = EXCLUDED.losing_trades,
			win_rate = EXCLUDED.win_rate,
			is_trading = EXCLUDED.is_trading,
			updated_at = NOW()
		RETURNING id, updated_at
	`

	err := r.db.QueryRowContext(
		ctx, query,
		state.AgentID,
		state.Symbol,
		state.Balance,
		state.InitialBalance,
		state.Equity,
		state.PnL,
		state.TotalTrades,
		state.WinningTrades,
		state.LosingTrades,
		state.WinRate,
		state.IsTrading,
	).Scan(&state.ID, &state.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create/update agent state: %w", err)
	}

	return nil
}

// GetAgentState retrieves agent trading state
func (r *Repository) GetAgentState(ctx context.Context, agentID string, symbol string) (*models.AgentState, error) {
	query := `
		SELECT id, agent_id, symbol, balance, initial_balance, equity, pnl,
		       total_trades, winning_trades, losing_trades, win_rate, is_trading, updated_at
		FROM agent_states
		WHERE agent_id = $1 AND symbol = $2
	`

	var state models.AgentState
	err := r.db.GetContext(ctx, &state, query, agentID, symbol)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("agent state not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get agent state: %w", err)
	}

	return &state, nil
}

// SaveDecision saves agent decision to database
func (r *Repository) SaveDecision(ctx context.Context, decision *models.AgentDecision) error {
	// MarketData already serialized as JSON string in decision
	marketDataJSON := decision.MarketData
	if marketDataJSON == "" {
		marketDataJSON = "{}"
	}

	query := `
		INSERT INTO agent_decisions (
			agent_id, symbol, action, confidence, reason,
			technical_score, news_score, onchain_score, sentiment_score, final_score,
			market_data, executed, execution_price, execution_size,
			order_id, stop_loss_order_id, take_profit_order_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
		RETURNING id, created_at
		ON CONFLICT (id) DO UPDATE SET
			executed = EXCLUDED.executed,
			execution_price = EXCLUDED.execution_price,
			execution_size = EXCLUDED.execution_size,
			order_id = EXCLUDED.order_id,
			stop_loss_order_id = EXCLUDED.stop_loss_order_id,
			take_profit_order_id = EXCLUDED.take_profit_order_id
	`

	// Handle NULL for order IDs
	var orderID, slOrderID, tpOrderID interface{}
	if decision.OrderID != "" {
		orderID = decision.OrderID
	}
	if decision.StopLossOrderID != "" {
		slOrderID = decision.StopLossOrderID
	}
	if decision.TakeProfitOrderID != "" {
		tpOrderID = decision.TakeProfitOrderID
	}

	err := r.db.QueryRowContext(
		ctx, query,
		decision.AgentID,
		decision.Symbol,
		decision.Action,
		decision.Confidence,
		decision.Reason,
		decision.TechnicalScore,
		decision.NewsScore,
		decision.OnChainScore,
		decision.SentimentScore,
		decision.FinalScore,
		marketDataJSON,
		decision.Executed,
		decision.ExecutionPrice,
		decision.ExecutionSize,
		orderID,
		slOrderID,
		tpOrderID,
	).Scan(&decision.ID, &decision.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to save decision: %w", err)
	}

	return nil
}

// CreateTournament creates a new agent tournament
func (r *Repository) CreateTournament(ctx context.Context, tournament *models.AgentTournament) error {
	query := `
		INSERT INTO agent_tournaments (
			user_id, name, symbols, start_balance, duration, is_active
		) VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, started_at, created_at
	`

	err := r.db.QueryRowContext(
		ctx, query,
		tournament.UserID,
		tournament.Name,
		pq.Array(tournament.Symbols),
		tournament.StartBalance,
		tournament.Duration,
		tournament.IsActive,
	).Scan(&tournament.ID, &tournament.StartedAt, &tournament.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create tournament: %w", err)
	}

	return nil
}

// EndTournament ends a tournament and records results
func (r *Repository) EndTournament(ctx context.Context, tournamentID string, results []models.AgentScore) error {
	// Find winner (highest return %)
	var winnerID string
	maxReturn := 0.0

	for _, score := range results {
		if score.ReturnPct > maxReturn {
			maxReturn = score.ReturnPct
			winnerID = score.AgentID
		}
	}

	resultsJSON, err := json.Marshal(results)
	if err != nil {
		return fmt.Errorf("failed to marshal results: %w", err)
	}

	query := `
		UPDATE agent_tournaments
		SET is_active = false,
		    ended_at = NOW(),
		    winner_agent_id = $1,
		    results = $2
		WHERE id = $3
	`

	_, err = r.db.ExecContext(ctx, query, winnerID, resultsJSON, tournamentID)
	if err != nil {
		return fmt.Errorf("failed to end tournament: %w", err)
	}

	return nil
}

// GetActiveTournaments retrieves active tournaments for user
func (r *Repository) GetActiveTournaments(ctx context.Context, userID string) ([]*models.AgentTournament, error) {
	query := `
		SELECT id, user_id, name, symbols, start_balance, duration,
		       started_at, ended_at, is_active, winner_agent_id, results, created_at
		FROM agent_tournaments
		WHERE user_id = $1 AND is_active = true
		ORDER BY started_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query tournaments: %w", err)
	}
	defer rows.Close()

	var tournaments []*models.AgentTournament
	for rows.Next() {
		var t models.AgentTournament
		var resultsJSON []byte

		err := rows.Scan(
			&t.ID,
			&t.UserID,
			&t.Name,
			pq.Array(&t.Symbols),
			&t.StartBalance,
			&t.Duration,
			&t.StartedAt,
			&t.EndedAt,
			&t.IsActive,
			&t.WinnerAgentID,
			&resultsJSON,
			&t.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan tournament: %w", err)
		}

		if len(resultsJSON) > 0 {
			t.Results = string(resultsJSON)
		}

		tournaments = append(tournaments, &t)
	}

	return tournaments, nil
}

// initializeAgentMemory creates initial memory record for agent
func (r *Repository) initializeAgentMemory(ctx context.Context, agentID string) error {
	query := `
		INSERT INTO agent_memory (
			agent_id, technical_success_rate, news_success_rate,
			onchain_success_rate, sentiment_success_rate,
			total_decisions, adaptation_count
		) VALUES ($1, 0.5, 0.5, 0.5, 0.5, 0, 0)
	`

	_, err := r.db.ExecContext(ctx, query, agentID)
	if err != nil {
		return fmt.Errorf("failed to initialize agent memory: %w", err)
	}

	return nil
}

// DeleteAgent deletes agent and all related data
func (r *Repository) DeleteAgent(ctx context.Context, agentID string) error {
	// Start transaction
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Delete in correct order (respecting foreign keys)
	tables := []string{
		"agent_decisions",
		"agent_states",
		"agent_memory",
		"agent_configs",
	}

	for _, table := range tables {
		query := fmt.Sprintf("DELETE FROM %s WHERE agent_id = $1", table)
		if table == "agent_configs" {
			query = fmt.Sprintf("DELETE FROM %s WHERE id = $1", table)
		}

		_, err := tx.ExecContext(ctx, query, agentID)
		if err != nil {
			return fmt.Errorf("failed to delete from %s: %w", table, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// ========== Semantic Memory Methods ==========

// StoreSemanticMemory saves new semantic memory for agent
func (r *Repository) StoreSemanticMemory(ctx context.Context, memory *models.SemanticMemory) error {
	query := `
		INSERT INTO agent_semantic_memories (
			agent_id, context, action, outcome, lesson, 
			embedding, importance, access_count
		) VALUES ($1, $2, $3, $4, $5, $6, $7, 0)
		RETURNING id, created_at
	`

	err := r.db.QueryRowContext(
		ctx, query,
		memory.AgentID,
		memory.Context,
		memory.Action,
		memory.Outcome,
		memory.Lesson,
		pq.Array(memory.Embedding),
		memory.Importance,
	).Scan(&memory.ID, &memory.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to store semantic memory: %w", err)
	}

	return nil
}

// GetSemanticMemories retrieves semantic memories for agent (ordered by importance)
func (r *Repository) GetSemanticMemories(ctx context.Context, agentID string, limit int) ([]models.SemanticMemory, error) {
	query := `
		SELECT id, agent_id, context, action, outcome, lesson,
		       embedding, importance, access_count, last_accessed, created_at
		FROM agent_semantic_memories
		WHERE agent_id = $1
		ORDER BY importance DESC, created_at DESC
		LIMIT $2
	`

	rows, err := r.db.QueryContext(ctx, query, agentID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query memories: %w", err)
	}
	defer rows.Close()

	memories := []models.SemanticMemory{}

	for rows.Next() {
		var mem models.SemanticMemory
		var embeddingFloats pq.Float32Array

		err := rows.Scan(
			&mem.ID,
			&mem.AgentID,
			&mem.Context,
			&mem.Action,
			&mem.Outcome,
			&mem.Lesson,
			&embeddingFloats,
			&mem.Importance,
			&mem.AccessCount,
			&mem.LastAccessed,
			&mem.CreatedAt,
		)
		if err != nil {
			continue
		}

		// Convert embedding
		embedding := make([]float32, len(embeddingFloats))
		copy(embedding, embeddingFloats)
		mem.Embedding = embedding

		memories = append(memories, mem)
	}

	return memories, nil
}

// SearchSemanticMemoriesByVector performs vector similarity search in PostgreSQL
func (r *Repository) SearchSemanticMemoriesByVector(ctx context.Context, agentID string, queryEmbedding []float32, limit int) ([]models.SemanticMemory, error) {
	// Use pgvector's cosine distance operator <=>
	// Lower distance = more similar
	// Threshold: distance < 0.3 means similarity > 70%
	const maxDistance = 0.3 // Only return results with 70%+ similarity

	query := `
		SELECT 
			id, agent_id, context, action, outcome, lesson,
			embedding, importance, access_count, last_accessed, created_at,
			embedding <=> $1::vector AS distance
		FROM agent_semantic_memories
		WHERE agent_id = $2
		  AND (embedding <=> $1::vector) < $3  -- Distance < 0.3 = similarity > 70%
		ORDER BY distance ASC
		LIMIT $4
	`

	rows, err := r.db.QueryContext(ctx, query, pq.Array(queryEmbedding), agentID, maxDistance, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to vector search memories: %w", err)
	}
	defer rows.Close()

	memories := []models.SemanticMemory{}

	for rows.Next() {
		var mem models.SemanticMemory
		var embeddingFloats pq.Float32Array
		var distance float64

		err := rows.Scan(
			&mem.ID,
			&mem.AgentID,
			&mem.Context,
			&mem.Action,
			&mem.Outcome,
			&mem.Lesson,
			&embeddingFloats,
			&mem.Importance,
			&mem.AccessCount,
			&mem.LastAccessed,
			&mem.CreatedAt,
			&distance,
		)
		if err != nil {
			continue
		}

		// Convert embedding
		embedding := make([]float32, len(embeddingFloats))
		copy(embedding, embeddingFloats)
		mem.Embedding = embedding

		memories = append(memories, mem)
	}

	return memories, nil
}

// SearchCollectiveMemoriesByVector performs vector similarity search for collective memories
func (r *Repository) SearchCollectiveMemoriesByVector(ctx context.Context, personality string, queryEmbedding []float32, limit int) ([]models.CollectiveMemory, error) {
	// Threshold: distance < 0.3 means similarity > 70%
	const maxDistance = 0.3

	query := `
		SELECT 
			id, personality, context, action, lesson,
			embedding, importance, confirmation_count, success_rate,
			last_confirmed_at, created_at,
			embedding <=> $1::vector AS distance
		FROM collective_agent_memories
		WHERE personality = $2
		  AND (embedding <=> $1::vector) < $3  -- Distance < 0.3 = similarity > 70%
		ORDER BY distance ASC
		LIMIT $4
	`

	rows, err := r.db.QueryContext(ctx, query, pq.Array(queryEmbedding), personality, maxDistance, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to vector search collective memories: %w", err)
	}
	defer rows.Close()

	memories := []models.CollectiveMemory{}

	for rows.Next() {
		var mem models.CollectiveMemory
		var embeddingFloats pq.Float32Array
		var distance float64

		err := rows.Scan(
			&mem.ID,
			&mem.Personality,
			&mem.Context,
			&mem.Action,
			&mem.Lesson,
			&embeddingFloats,
			&mem.Importance,
			&mem.ConfirmationCount,
			&mem.SuccessRate,
			&mem.LastConfirmedAt,
			&mem.CreatedAt,
			&distance,
		)
		if err != nil {
			continue
		}

		// Convert embedding
		embedding := make([]float32, len(embeddingFloats))
		copy(embedding, embeddingFloats)
		mem.Embedding = embedding

		memories = append(memories, mem)
	}

	return memories, nil
}

// UpdateMemoryAccess updates memory access count and timestamp
func (r *Repository) UpdateMemoryAccess(ctx context.Context, memoryID string) error {
	query := `
		UPDATE agent_semantic_memories
		SET access_count = access_count + 1,
		    last_accessed = NOW()
		WHERE id = $1
	`

	_, err := r.db.ExecContext(ctx, query, memoryID)
	return err
}

// DeleteOldMemories removes less important memories
func (r *Repository) DeleteOldMemories(ctx context.Context, agentID string, importanceThreshold float64) (int64, error) {
	query := `
		DELETE FROM agent_semantic_memories
		WHERE agent_id = $1
		  AND importance < $2
		  AND access_count < 2
		  AND created_at < NOW() - INTERVAL '30 days'
	`

	result, err := r.db.ExecContext(ctx, query, agentID, importanceThreshold)
	if err != nil {
		return 0, fmt.Errorf("failed to delete old memories: %w", err)
	}

	deleted, _ := result.RowsAffected()
	return deleted, nil
}

// SaveReasoningSession saves complete reasoning trace
func (r *Repository) SaveReasoningSession(ctx context.Context, session *models.ReasoningSession) error {
	// Marshal JSONs
	recalledJSON, _ := json.Marshal(session.Thoughts) // Use thoughts as recalled memories for simplicity
	optionsJSON, _ := json.Marshal([]string{})        // Will be filled by actual options
	evaluationsJSON, _ := json.Marshal([]string{})
	decisionJSON, _ := json.Marshal(session.Decision)
	cotJSON, _ := json.Marshal(session.Thoughts)

	query := `
		INSERT INTO agent_reasoning_sessions (
			session_id, agent_id, observation, recalled_memories,
			generated_options, evaluations, final_reasoning, decision,
			chain_of_thought, executed, started_at, completed_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`

	_, err := r.db.ExecContext(
		ctx, query,
		session.SessionID,
		session.AgentID,
		"", // observation - will be filled from thoughts
		recalledJSON,
		optionsJSON,
		evaluationsJSON,
		"", // final reasoning
		decisionJSON,
		cotJSON,
		session.Executed,
		session.StartedAt,
		session.CompletedAt,
	)

	return err
}

// SaveReflection saves agent's reflection
func (r *Repository) SaveReflection(ctx context.Context, agentID string, reflection *models.Reflection) error {
	adjustmentsJSON, _ := json.Marshal(reflection.SuggestedAdjustments)

	query := `
		INSERT INTO agent_reflections (
			agent_id, analysis, what_worked, what_didnt_work,
			key_lessons, suggested_adjustments, confidence_in_analysis
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at
	`

	var id int64
	var createdAt time.Time

	err := r.db.QueryRowContext(
		ctx, query,
		agentID,
		reflection.Analysis,
		pq.Array(reflection.WhatWorked),
		pq.Array(reflection.WhatDidntWork),
		pq.Array(reflection.KeyLessons),
		adjustmentsJSON,
		reflection.ConfidenceInAnalysis,
	).Scan(&id, &createdAt)

	return err
}

// SaveTradingPlan saves agent's trading plan
func (r *Repository) SaveTradingPlan(ctx context.Context, plan *models.TradingPlan) error {
	scenariosJSON, _ := json.Marshal(plan.Scenarios)
	limitsJSON, _ := json.Marshal(plan.RiskLimits)
	triggersJSON, _ := json.Marshal(plan.TriggerSignals)

	query := `
		INSERT INTO agent_trading_plans (
			plan_id, agent_id, time_horizon, assumptions,
			scenarios, risk_limits, trigger_signals, status, expires_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at
	`

	var id int64
	var createdAt time.Time

	err := r.db.QueryRowContext(
		ctx, query,
		plan.PlanID,
		plan.AgentID,
		plan.TimeHorizon,
		pq.Array(plan.Assumptions),
		scenariosJSON,
		limitsJSON,
		triggersJSON,
		plan.Status,
		plan.ExpiresAt,
	).Scan(&id, &createdAt)

	return err
}

// ========== Statistical Memory Methods (for old MemoryManager) ==========

// GetAgentStatisticalMemory retrieves agent's statistical learning data
func (r *Repository) GetAgentStatisticalMemory(ctx context.Context, agentID string) (*models.AgentMemory, error) {
	query := `
		SELECT id, agent_id, technical_success_rate, news_success_rate,
		       onchain_success_rate, sentiment_success_rate,
		       best_market_conditions, worst_market_conditions,
		       total_decisions, adaptation_count, last_adapted_at, updated_at
		FROM agent_memory
		WHERE agent_id = $1
	`

	var memory models.AgentMemory
	err := r.db.GetContext(ctx, &memory, query, agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent memory: %w", err)
	}

	return &memory, nil
}

// UpdateDecisionOutcome updates decision with trade outcome
func (r *Repository) UpdateDecisionOutcome(ctx context.Context, decisionID string, outcomeJSON []byte) error {
	query := `
		UPDATE agent_decisions
		SET outcome = $1,
		    updated_at = NOW()
		WHERE id = $2
	`

	_, err := r.db.ExecContext(ctx, query, outcomeJSON, decisionID)
	return err
}

// UpdateSignalSuccessRates updates success rates for different signal types
func (r *Repository) UpdateSignalSuccessRates(ctx context.Context, agentID string) error {
	query := `
		WITH signal_outcomes AS (
			SELECT 
				technical_score,
				news_score,
				onchain_score,
				sentiment_score,
				(outcome->>'pnl')::float AS pnl,
				CASE WHEN (outcome->>'pnl')::float > 0 THEN 1 ELSE 0 END AS is_winning
			FROM agent_decisions
			WHERE agent_id = $1
			  AND executed = true
			  AND outcome IS NOT NULL
			  AND created_at > NOW() - INTERVAL '30 days'
		)
		UPDATE agent_memory
		SET 
			technical_success_rate = (
				SELECT COALESCE(AVG(CASE WHEN technical_score > 60 THEN is_winning::float END), 0.5)
				FROM signal_outcomes
			),
			news_success_rate = (
				SELECT COALESCE(AVG(CASE WHEN news_score > 60 THEN is_winning::float END), 0.5)
				FROM signal_outcomes
			),
			onchain_success_rate = (
				SELECT COALESCE(AVG(CASE WHEN onchain_score > 60 THEN is_winning::float END), 0.5)
				FROM signal_outcomes
			),
			sentiment_success_rate = (
				SELECT COALESCE(AVG(CASE WHEN sentiment_score > 60 THEN is_winning::float END), 0.5)
				FROM signal_outcomes
			),
			total_decisions = (SELECT COUNT(*) FROM signal_outcomes),
			updated_at = NOW()
		WHERE agent_id = $1
	`

	_, err := r.db.ExecContext(ctx, query, agentID)
	return err
}

// UpdateAgentSpecialization updates agent's signal weights
func (r *Repository) UpdateAgentSpecialization(ctx context.Context, agentID string, specialization models.AgentSpecialization) error {
	specializationJSON, err := json.Marshal(specialization)
	if err != nil {
		return fmt.Errorf("failed to marshal specialization: %w", err)
	}

	query := `
		UPDATE agent_configs
		SET specialization = $1,
		    updated_at = NOW()
		WHERE id = $2
	`

	_, err = r.db.ExecContext(ctx, query, specializationJSON, agentID)
	return err
}

// IncrementAdaptationCount increments agent memory adaptation count
func (r *Repository) IncrementAdaptationCount(ctx context.Context, agentID string) error {
	query := `
		UPDATE agent_memory
		SET adaptation_count = adaptation_count + 1,
		    last_adapted_at = NOW(),
		    updated_at = NOW()
		WHERE agent_id = $1
	`

	_, err := r.db.ExecContext(ctx, query, agentID)
	return err
}

// ========== Collective Memory Methods ==========

// GetCollectiveMemories retrieves collective memories for personality
func (r *Repository) GetCollectiveMemories(ctx context.Context, personality string, limit int) ([]models.CollectiveMemory, error) {
	query := `
		SELECT id, personality, context, action, lesson, embedding,
		       importance, confirmation_count, success_rate, last_confirmed_at, created_at
		FROM collective_agent_memories
		WHERE personality = $1
		  AND confirmation_count >= 2
		  AND success_rate > 0.5
		ORDER BY success_rate DESC, confirmation_count DESC, importance DESC
		LIMIT $2
	`

	rows, err := r.db.QueryContext(ctx, query, personality, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query collective memories: %w", err)
	}
	defer rows.Close()

	memories := []models.CollectiveMemory{}
	for rows.Next() {
		var mem models.CollectiveMemory
		var embeddingFloats pq.Float32Array

		err := rows.Scan(
			&mem.ID,
			&mem.Personality,
			&mem.Context,
			&mem.Action,
			&mem.Lesson,
			&embeddingFloats,
			&mem.Importance,
			&mem.ConfirmationCount,
			&mem.SuccessRate,
			&mem.LastConfirmedAt,
			&mem.CreatedAt,
		)
		if err != nil {
			continue
		}

		embedding := make([]float32, len(embeddingFloats))
		copy(embedding, embeddingFloats)
		mem.Embedding = embedding

		memories = append(memories, mem)
	}

	return memories, nil
}

// ContributeToCollective adds or confirms collective memory
func (r *Repository) ContributeToCollective(
	ctx context.Context,
	agentID string,
	personality string,
	memory *models.MemorySummary,
	embedding []float32,
	wasSuccessful bool,
) error {
	// Check if similar collective memory already exists
	// If yes, confirm it. If no, create new one.

	query := `
		INSERT INTO collective_agent_memories (
			personality, context, action, lesson, embedding, importance,
			confirmation_count, success_rate
		) VALUES ($1, $2, $3, $4, $5, $6, 1, $7)
		ON CONFLICT DO NOTHING
		RETURNING id
	`

	successRate := 0.5
	if wasSuccessful {
		successRate = 1.0
	}

	var collectiveID string
	err := r.db.QueryRowContext(
		ctx, query,
		personality,
		memory.Context,
		memory.Action,
		memory.Lesson,
		pq.Array(embedding),
		memory.Importance,
		successRate,
	).Scan(&collectiveID)

	// If memory already exists, update it
	if err != nil {
		// Find similar existing memory and update confirmation
		return r.updateCollectiveConfirmation(ctx, agentID, personality, memory.Lesson, wasSuccessful)
	}

	// Record confirmation for new memory
	confirmQuery := `
		INSERT INTO memory_confirmations (
			collective_memory_id, agent_id, was_successful, trade_count, pnl_sum
		) VALUES ($1, $2, $3, 1, 0)
	`

	_, err = r.db.ExecContext(ctx, confirmQuery, collectiveID, agentID, wasSuccessful)

	return err
}

// updateCollectiveConfirmation updates existing collective memory
func (r *Repository) updateCollectiveConfirmation(
	ctx context.Context,
	agentID string,
	personality string,
	lesson string,
	wasSuccessful bool,
) error {
	// Find similar collective memory
	findQuery := `
		SELECT id, confirmation_count, success_rate
		FROM collective_agent_memories
		WHERE personality = $1
		  AND lesson ILIKE $2
		LIMIT 1
	`

	var collectiveID string
	var confirmCount int
	var successRate float64

	lessonPrefix := lesson
	if len(lesson) > 50 {
		lessonPrefix = lesson[:50]
	}

	err := r.db.QueryRowContext(ctx, findQuery, personality, "%"+lessonPrefix+"%").
		Scan(&collectiveID, &confirmCount, &successRate)

	if err != nil {
		return nil // Not found, that's ok
	}

	// Update confirmation count and success rate
	newConfirmCount := confirmCount + 1
	successFloat := 0.0
	if wasSuccessful {
		successFloat = 1.0
	}
	newSuccessRate := (successRate*float64(confirmCount) + successFloat) / float64(newConfirmCount)

	updateQuery := `
		UPDATE collective_agent_memories
		SET confirmation_count = $1,
		    success_rate = $2,
		    last_confirmed_at = NOW()
		WHERE id = $3
	`

	_, err = r.db.ExecContext(ctx, updateQuery, newConfirmCount, newSuccessRate, collectiveID)
	if err != nil {
		return err
	}

	// Add confirmation record
	confirmQuery := `
		INSERT INTO memory_confirmations (
			collective_memory_id, agent_id, was_successful, trade_count, pnl_sum
		) VALUES ($1, $2, $3, 1, 0)
		ON CONFLICT (collective_memory_id, agent_id) DO UPDATE
		SET was_successful = EXCLUDED.was_successful,
		    trade_count = memory_confirmations.trade_count + 1,
		    confirmed_at = NOW()
	`

	_, err = r.db.ExecContext(ctx, confirmQuery, collectiveID, agentID, wasSuccessful)

	return err
}

// ========== Performance Metrics Methods ==========

// GetTradeReturns gets trade returns for Sharpe ratio calculation
func (r *Repository) GetTradeReturns(ctx context.Context, agentID, symbol string, limit int) ([]float64, error) {
	query := `
		SELECT (outcome->>'pnl_percent')::float as pnl_percent
		FROM agent_decisions
		WHERE agent_id = $1
		  AND symbol = $2
		  AND executed = true
		  AND outcome IS NOT NULL
		ORDER BY created_at DESC
		LIMIT $3
	`

	rows, err := r.db.QueryContext(ctx, query, agentID, symbol, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	returns := []float64{}
	for rows.Next() {
		var pnlPercent float64
		if err := rows.Scan(&pnlPercent); err != nil {
			continue
		}
		returns = append(returns, pnlPercent)
	}

	return returns, nil
}

// GetPeakEquity gets peak equity for agent
func (r *Repository) GetPeakEquity(ctx context.Context, agentID, symbol string) (float64, error) {
	query := `
		SELECT COALESCE(MAX((outcome->>'equity')::float), 0) as peak_equity
		FROM agent_decisions
		WHERE agent_id = $1 AND symbol = $2 AND executed = true
	`

	var peakEquity float64
	err := r.db.GetContext(ctx, &peakEquity, query, agentID, symbol)
	return peakEquity, err
}

// GetProfitLoss gets gross profit and loss for agent
func (r *Repository) GetProfitLoss(ctx context.Context, agentID, symbol string) (grossProfit, grossLoss float64, err error) {
	query := `
		SELECT 
			COALESCE(SUM((outcome->>'pnl')::float) FILTER (WHERE (outcome->>'pnl')::float > 0), 0) as gross_profit,
			COALESCE(ABS(SUM((outcome->>'pnl')::float) FILTER (WHERE (outcome->>'pnl')::float < 0)), 0) as gross_loss
		FROM agent_decisions
		WHERE agent_id = $1 AND symbol = $2 AND executed = true AND outcome IS NOT NULL
	`

	err = r.db.QueryRowContext(ctx, query, agentID, symbol).Scan(&grossProfit, &grossLoss)
	return grossProfit, grossLoss, err
}

// GetDailyPnL calculates PnL for today
func (r *Repository) GetDailyPnL(ctx context.Context, agentID, symbol string) (float64, error) {
	query := `
		SELECT COALESCE(SUM((outcome->>'pnl')::float), 0) as daily_pnl
		FROM agent_decisions
		WHERE agent_id = $1
		  AND symbol = $2
		  AND executed = true
		  AND outcome IS NOT NULL
		  AND created_at >= CURRENT_DATE
	`

	var dailyPnL float64
	err := r.db.GetContext(ctx, &dailyPnL, query, agentID, symbol)
	return dailyPnL, err
}

// ========== On-Chain Data Methods ==========

// GetRecentWhaleTransactions gets recent whale movements for symbol
func (r *Repository) GetRecentWhaleTransactions(ctx context.Context, symbol string, hours int, minImpact int) ([]models.WhaleTransaction, error) {
	query := `
		SELECT id, tx_hash, blockchain, symbol, amount, amount_usd,
		       from_address, to_address, from_owner, to_owner,
		       transaction_type, timestamp, impact_score, created_at
		FROM whale_transactions
		WHERE symbol = $1
		  AND timestamp > NOW() - INTERVAL '%d hours'
		  AND impact_score >= $2
		ORDER BY timestamp DESC
		LIMIT 50
	`

	var transactions []models.WhaleTransaction
	err := r.db.SelectContext(ctx, &transactions, fmt.Sprintf(query, hours), symbol, minImpact)
	if err != nil {
		return nil, fmt.Errorf("failed to get whale transactions: %w", err)
	}

	return transactions, nil
}

// GetExchangeFlows gets aggregated exchange flows for symbol
func (r *Repository) GetExchangeFlows(ctx context.Context, symbol string, hours int) ([]models.ExchangeFlow, error) {
	query := `
		SELECT id, exchange, symbol, timestamp, inflow, outflow, net_flow, created_at
		FROM exchange_flows
		WHERE symbol = $1
		  AND timestamp > NOW() - INTERVAL '%d hours'
		ORDER BY timestamp DESC
		LIMIT 24
	`

	var flows []models.ExchangeFlow
	err := r.db.SelectContext(ctx, &flows, fmt.Sprintf(query, hours), symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get exchange flows: %w", err)
	}

	return flows, nil
}

// GetAgentPerformanceMetrics calculates agent performance metrics
func (r *Repository) GetAgentPerformanceMetrics(ctx context.Context, agentID string, symbol string) (*AgentPerformanceMetrics, error) {
	query := `
		WITH trade_stats AS (
			SELECT 
				COUNT(*) as total_trades,
				COUNT(*) FILTER (WHERE (outcome->>'pnl')::float > 0) as winning_trades,
				COUNT(*) FILTER (WHERE (outcome->>'pnl')::float < 0) as losing_trades,
				SUM((outcome->>'pnl')::float) as total_pnl,
				AVG((outcome->>'pnl')::float) as avg_pnl,
				MAX((outcome->>'pnl')::float) as max_win,
				MIN((outcome->>'pnl')::float) as max_loss,
				STDDEV((outcome->>'pnl')::float) as pnl_stddev
			FROM agent_decisions
			WHERE agent_id = $1
			  AND symbol = $2
			  AND executed = true
			  AND outcome IS NOT NULL
		)
		SELECT 
			total_trades,
			winning_trades,
			losing_trades,
			CASE WHEN total_trades > 0 THEN winning_trades::float / total_trades ELSE 0 END as win_rate,
			total_pnl,
			avg_pnl,
			max_win,
			max_loss,
			CASE WHEN pnl_stddev > 0 THEN avg_pnl / pnl_stddev ELSE 0 END as sharpe_ratio
		FROM trade_stats
	`

	var metrics AgentPerformanceMetrics
	err := r.db.GetContext(ctx, &metrics, query, agentID, symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get performance metrics: %w", err)
	}

	return &metrics, nil
}

// AgentPerformanceMetrics holds performance statistics
type AgentPerformanceMetrics struct {
	TotalTrades   int     `db:"total_trades"`
	WinningTrades int     `db:"winning_trades"`
	LosingTrades  int     `db:"losing_trades"`
	WinRate       float64 `db:"win_rate"`
	TotalPnL      float64 `db:"total_pnl"`
	AvgPnL        float64 `db:"avg_pnl"`
	MaxWin        float64 `db:"max_win"`
	MaxLoss       float64 `db:"max_loss"`
	SharpeRatio   float64 `db:"sharpe_ratio"`
}

// ========== Agent Recovery Methods ==========

// AgentToRestore holds information about agent that needs to be restored after pod restart
type AgentToRestore struct {
	AgentID        string  `db:"agent_id"`
	UserID         string  `db:"user_id"`
	Symbol         string  `db:"symbol"`
	Balance        float64 `db:"balance"`
	InitialBalance float64 `db:"initial_balance"`
	Exchange       string  `db:"exchange"`
	APIKey         string  `db:"api_key"`
	APISecret      string  `db:"api_secret"`
	Testnet        bool    `db:"testnet"`
}

// GetAgentsToRestore retrieves all agents that should be running (for pod recovery)
func (r *Repository) GetAgentsToRestore(ctx context.Context) ([]AgentToRestore, error) {
	query := `
		SELECT 
			ac.id as agent_id,
			ac.user_id,
			ast.symbol,
			ast.balance,
			ast.initial_balance,
			ue.exchange,
			ue.api_key,
			ue.api_secret,
			ue.testnet
		FROM agent_configs ac
		INNER JOIN agent_states ast ON ac.id = ast.agent_id
		INNER JOIN user_trading_pairs utp ON ast.symbol = utp.symbol AND utp.user_id = ac.user_id
		INNER JOIN user_exchanges ue ON utp.exchange_id = ue.id
		WHERE ac.is_active = true
		  AND ast.is_trading = true
		ORDER BY ast.updated_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query agents to restore: %w", err)
	}
	defer rows.Close()

	var agents []AgentToRestore
	for rows.Next() {
		var agent AgentToRestore
		err := rows.Scan(
			&agent.AgentID,
			&agent.UserID,
			&agent.Symbol,
			&agent.Balance,
			&agent.InitialBalance,
			&agent.Exchange,
			&agent.APIKey,
			&agent.APISecret,
			&agent.Testnet,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan agent: %w", err)
		}

		agents = append(agents, agent)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return agents, nil
}

// ========== Chain-of-Thought Checkpoint Methods ==========

// SaveThinkingCheckpoint saves intermediate CoT state during graceful shutdown
func (r *Repository) SaveThinkingCheckpoint(
	ctx context.Context,
	sessionID string,
	agentID string,
	state interface{}, // ThinkingState
	history interface{}, // []ThoughtStep
) error {
	stateJSON, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	historyJSON, err := json.Marshal(history)
	if err != nil {
		return fmt.Errorf("failed to marshal history: %w", err)
	}

	query := `
		INSERT INTO agent_reasoning_sessions (
			session_id, agent_id, observation, recalled_memories,
			generated_options, evaluations, final_reasoning, decision,
			is_interrupted, checkpoint_state, checkpoint_history,
			started_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (session_id) DO UPDATE SET
			is_interrupted = true,
			checkpoint_state = $10,
			checkpoint_history = $11
	`

	_, err = r.db.ExecContext(
		ctx, query,
		sessionID,
		agentID,
		"[INTERRUPTED]", // observation
		"[]",            // recalled_memories
		"[]",            // generated_options
		"[]",            // evaluations
		"",              // final_reasoning
		"{}",            // decision
		true,            // is_interrupted
		stateJSON,
		historyJSON,
		time.Now(),
	)

	if err != nil {
		return fmt.Errorf("failed to save checkpoint: %w", err)
	}

	return nil
}

// ReasoningCheckpoint holds checkpoint data for resuming
type ReasoningCheckpoint struct {
	SessionID         string    `db:"session_id"`
	AgentID           string    `db:"agent_id"`
	CheckpointState   []byte    `db:"checkpoint_state"`
	CheckpointHistory []byte    `db:"checkpoint_history"`
	StartedAt         time.Time `db:"started_at"`
}

// GetInterruptedSession retrieves interrupted reasoning session for agent
func (r *Repository) GetInterruptedSession(ctx context.Context, agentID string) (*ReasoningCheckpoint, error) {
	query := `
		SELECT session_id, agent_id, checkpoint_state, checkpoint_history, started_at
		FROM agent_reasoning_sessions
		WHERE agent_id = $1
		  AND is_interrupted = true
		  AND completed_at IS NULL
		ORDER BY started_at DESC
		LIMIT 1
	`

	var checkpoint ReasoningCheckpoint
	err := r.db.QueryRowContext(ctx, query, agentID).Scan(
		&checkpoint.SessionID,
		&checkpoint.AgentID,
		&checkpoint.CheckpointState,
		&checkpoint.CheckpointHistory,
		&checkpoint.StartedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil // No interrupted session
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get interrupted session: %w", err)
	}

	return &checkpoint, nil
}

// CompleteReasoningSession marks session as completed (removes checkpoint)
func (r *Repository) CompleteReasoningSession(
	ctx context.Context,
	sessionID string,
	decision interface{},
	finalReasoning string,
) error {
	decisionJSON, err := json.Marshal(decision)
	if err != nil {
		return fmt.Errorf("failed to marshal decision: %w", err)
	}

	query := `
		UPDATE agent_reasoning_sessions
		SET is_interrupted = false,
		    checkpoint_state = NULL,
		    checkpoint_history = NULL,
		    decision = $2,
		    final_reasoning = $3,
		    completed_at = NOW(),
		    duration_ms = EXTRACT(EPOCH FROM (NOW() - started_at)) * 1000,
		    executed = true
		WHERE session_id = $1
	`

	_, err = r.db.ExecContext(ctx, query, sessionID, decisionJSON, finalReasoning)
	if err != nil {
		return fmt.Errorf("failed to complete session: %w", err)
	}

	return nil
}

// DeleteCheckpoint removes checkpoint after successful completion
func (r *Repository) DeleteCheckpoint(ctx context.Context, sessionID string) error {
	query := `
		UPDATE agent_reasoning_sessions
		SET is_interrupted = false,
		    checkpoint_state = NULL,
		    checkpoint_history = NULL
		WHERE session_id = $1
	`

	_, err := r.db.ExecContext(ctx, query, sessionID)
	return err
}
