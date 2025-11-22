package agents

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"

	"github.com/alexanderselivanov/trader/internal/adapters/ai"
	"github.com/alexanderselivanov/trader/internal/adapters/exchange"
	"github.com/alexanderselivanov/trader/internal/adapters/news"
	"github.com/alexanderselivanov/trader/internal/portfolio"
	"github.com/alexanderselivanov/trader/internal/risk"
	"github.com/alexanderselivanov/trader/pkg/logger"
	"github.com/alexanderselivanov/trader/pkg/models"
)

// Manager orchestrates multiple AI agents
type Manager struct {
	mu             sync.RWMutex
	db             *sqlx.DB
	repository     *Repository
	memoryManager  *MemoryManager
	newsAggregator *news.Aggregator
	aiProviders    map[string]ai.Provider // AI providers available for agents
	runningAgents  map[string]*AgentRunner // agentID -> runner
	ctx            context.Context
	cancel         context.CancelFunc
}

// AgentRunner represents a running agent instance
type AgentRunner struct {
	Config         *models.AgentConfig
	State          *models.AgentState
	DecisionEngine *DecisionEngine
	Exchange       exchange.Exchange
	Portfolio      *portfolio.Tracker
	RiskManager    *risk.Validator
	CancelFunc     context.CancelFunc
	IsRunning      bool
	LastDecisionAt time.Time
}

// NewManager creates new agent manager
func NewManager(db *sqlx.DB, newsAggregator *news.Aggregator, aiProviders []ai.Provider) *Manager {
	ctx, cancel := context.WithCancel(context.Background())

	// Map AI providers by name for easy lookup
	providerMap := make(map[string]ai.Provider)
	for _, provider := range aiProviders {
		if provider.IsEnabled() {
			providerMap[provider.GetName()] = provider
		}
	}

	repo := NewRepository(db)

	return &Manager{
		db:             db,
		repository:     repo,
		memoryManager:  NewMemoryManager(repo),
		newsAggregator: newsAggregator,
		aiProviders:    providerMap,
		runningAgents:  make(map[string]*AgentRunner),
		ctx:            ctx,
		cancel:         cancel,
	}
}

// CreateAgent creates a new agent from preset personality
func (m *Manager) CreateAgent(ctx context.Context, userID string, personality models.AgentPersonality, name string) (*models.AgentConfig, error) {
	presetFunc, ok := PresetAgentConfigs[personality]
	if !ok {
		return nil, fmt.Errorf("unknown personality: %s", personality)
	}

	config := presetFunc(userID, name)

	// Save to database
	savedConfig, err := m.repository.CreateAgent(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create agent: %w", err)
	}

	logger.Info("agent created",
		zap.String("agent_id", savedConfig.ID),
		zap.String("name", savedConfig.Name),
		zap.String("personality", string(savedConfig.Personality)),
	)

	return savedConfig, nil
}

// StartAgent starts an agent for a specific trading pair
func (m *Manager) StartAgent(ctx context.Context, agentID string, symbol string, initialBalance float64, exchangeAdapter exchange.Exchange) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if already running
	if runner, exists := m.runningAgents[agentID]; exists && runner.IsRunning {
		return fmt.Errorf("agent already running")
	}

	// Load agent config
	config, err := m.repository.GetAgent(ctx, agentID)
	if err != nil {
		return fmt.Errorf("failed to load agent config: %w", err)
	}

	if !config.IsActive {
		return fmt.Errorf("agent is not active")
	}

	// Initialize or load agent state
	state, err := m.repository.GetAgentState(ctx, agentID, symbol)
	if err != nil {
		// Create new state
		state = &models.AgentState{
			AgentID:        agentID,
			Symbol:         symbol,
			Balance:        models.NewDecimal(initialBalance),
			InitialBalance: models.NewDecimal(initialBalance),
			Equity:         models.NewDecimal(initialBalance),
			PnL:            models.NewDecimal(0),
			IsTrading:      true,
		}

		if err := m.repository.CreateAgentState(ctx, state); err != nil {
			return fmt.Errorf("failed to create agent state: %w", err)
		}
	}

	// Select AI provider for this agent (round-robin or based on personality)
	aiProvider := m.selectAIProviderForAgent(config)
	if aiProvider == nil {
		return fmt.Errorf("no AI providers available")
	}

	logger.Info("assigned AI provider to agent",
		zap.String("agent_id", agentID),
		zap.String("provider", aiProvider.GetName()),
	)

	// Create decision engine with AI provider
	decisionEngine := NewDecisionEngine(config, aiProvider)

	// Create portfolio tracker (using database.DB wrapper for compatibility)
	// TODO: Refactor to use sqlx throughout
	// For now, we'll skip portfolio tracker initialization for agents
	// as they need a different setup than regular bots

	// Create risk validator
	riskValidator := risk.NewValidator()

	// Create agent context
	agentCtx, agentCancel := context.WithCancel(m.ctx)

	runner := &AgentRunner{
		Config:         config,
		State:          state,
		DecisionEngine: decisionEngine,
		Exchange:       exchangeAdapter,
		Portfolio:      nil, // TODO: Create simplified portfolio tracker for agents
		RiskManager:    riskValidator,
		CancelFunc:     agentCancel,
		IsRunning:      true,
		LastDecisionAt: time.Now(),
	}

	m.runningAgents[agentID] = runner

	// Start agent goroutine
	go m.runAgent(agentCtx, runner)

	logger.Info("agent started",
		zap.String("agent_id", agentID),
		zap.String("name", config.Name),
		zap.String("symbol", symbol),
		zap.Float64("initial_balance", initialBalance),
	)

	return nil
}

// runAgent is the main loop for an agent
func (m *Manager) runAgent(ctx context.Context, runner *AgentRunner) {
	ticker := time.NewTicker(runner.Config.DecisionInterval)
	defer ticker.Stop()

	logger.Info("agent loop started",
		zap.String("agent_id", runner.Config.ID),
		zap.String("name", runner.Config.Name),
	)

	// Run immediately on start
	if err := m.executeTradingCycle(ctx, runner); err != nil {
		logger.Error("trading cycle failed",
			zap.String("agent_id", runner.Config.ID),
			zap.Error(err),
		)
	}

	// Then run on interval
	for {
		select {
		case <-ctx.Done():
			logger.Info("agent stopped",
				zap.String("agent_id", runner.Config.ID),
			)
			return

		case <-ticker.C:
			if !runner.IsRunning {
				continue
			}

			if err := m.executeTradingCycle(ctx, runner); err != nil {
				logger.Error("trading cycle failed",
					zap.String("agent_id", runner.Config.ID),
					zap.Error(err),
				)
			}

			// Check if should adapt strategy
			shouldAdapt, err := m.memoryManager.ShouldAdapt(ctx, runner.Config.ID)
			if err != nil {
				logger.Error("failed to check adaptation",
					zap.String("agent_id", runner.Config.ID),
					zap.Error(err),
				)
			} else if shouldAdapt {
				if err := m.memoryManager.AdaptStrategy(ctx, runner.Config.ID, runner.Config); err != nil {
					logger.Error("failed to adapt strategy",
						zap.String("agent_id", runner.Config.ID),
						zap.Error(err),
					)
				} else {
					logger.Info("agent adapted strategy",
						zap.String("agent_id", runner.Config.ID),
					)
					// Reload config and recreate decision engine
					newConfig, err := m.repository.GetAgent(ctx, runner.Config.ID)
					if err == nil {
						runner.Config = newConfig
						aiProvider := m.selectAIProviderForAgent(newConfig)
						if aiProvider != nil {
							runner.DecisionEngine = NewDecisionEngine(newConfig, aiProvider)
						}
					}
				}
			}
		}
	}
}

// executeTradingCycle executes one trading cycle for an agent
func (m *Manager) executeTradingCycle(ctx context.Context, runner *AgentRunner) error {
	logger.Debug("executing agent trading cycle",
		zap.String("agent_id", runner.Config.ID),
		zap.String("symbol", runner.State.Symbol),
	)

	// Collect market data
	marketData, err := m.collectMarketData(ctx, runner)
	if err != nil {
		return fmt.Errorf("failed to collect market data: %w", err)
	}

	// Get current position
	position, _ := runner.Exchange.FetchPosition(ctx, runner.State.Symbol)

	// Make decision
	decision, err := runner.DecisionEngine.Analyze(ctx, marketData, position)
	if err != nil {
		return fmt.Errorf("failed to analyze market: %w", err)
	}

	runner.LastDecisionAt = time.Now()

	// Save decision to database
	if err := m.repository.SaveDecision(ctx, decision); err != nil {
		logger.Error("failed to save decision", zap.Error(err))
	}

	logger.Info("agent decision",
		zap.String("agent_id", runner.Config.ID),
		zap.String("agent_name", runner.Config.Name),
		zap.String("action", string(decision.Action)),
		zap.Int("confidence", decision.Confidence),
		zap.Float64("final_score", decision.FinalScore),
	)

	// Execute if not HOLD
	if decision.Action != models.ActionHold {
		if err := m.executeDecision(ctx, runner, decision, position); err != nil {
			logger.Error("failed to execute decision",
				zap.String("agent_id", runner.Config.ID),
				zap.Error(err),
			)
			return err
		}
	}

	// Update agent state
	if err := m.updateAgentState(ctx, runner); err != nil {
		logger.Error("failed to update agent state", zap.Error(err))
	}

	return nil
}

// collectMarketData collects market data for decision making
func (m *Manager) collectMarketData(ctx context.Context, runner *AgentRunner) (*models.MarketData, error) {
	symbol := runner.State.Symbol

	// Fetch ticker
	ticker, err := runner.Exchange.FetchTicker(ctx, symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch ticker: %w", err)
	}

	// Fetch candles
	candles, err := runner.Exchange.FetchOHLCV(ctx, symbol, "1h", 100)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch candles: %w", err)
	}

	candlesMap := map[string][]models.Candle{
		"1h": candles,
	}

	// Fetch order book
	orderBook, err := runner.Exchange.FetchOrderBook(ctx, symbol, 20)
	if err != nil {
		logger.Warn("failed to fetch order book", zap.Error(err))
	}

	// Fetch funding rate
	fundingRate, err := runner.Exchange.FetchFundingRate(ctx, symbol)
	if err != nil {
		logger.Warn("failed to fetch funding rate", zap.Error(err))
		fundingRate = 0
	}

	// Get cached news
	var newsSummary *models.NewsSummary
	if m.newsAggregator != nil {
		newsSummary, err = m.newsAggregator.GetCachedSummary(ctx, 6*time.Hour)
		if err != nil {
			logger.Warn("failed to get cached news", zap.Error(err))
		}
	}

	marketData := &models.MarketData{
		Symbol:       symbol,
		Ticker:       ticker,
		Candles:      candlesMap,
		OrderBook:    orderBook,
		FundingRate:  models.NewDecimal(fundingRate),
		OpenInterest: models.NewDecimal(0), // TODO: implement
		NewsSummary:  newsSummary,
		Timestamp:    time.Now(),
	}

	return marketData, nil
}

// executeDecision executes trading decision
func (m *Manager) executeDecision(ctx context.Context, runner *AgentRunner, decision *models.AgentDecision, position *models.Position) error {
	// TODO: Implement actual trade execution
	// For now, just log
	logger.Info("would execute trade",
		zap.String("agent_id", runner.Config.ID),
		zap.String("action", string(decision.Action)),
		zap.Int("confidence", decision.Confidence),
	)

	decision.Executed = true
	return nil
}

// updateAgentState updates agent's trading state
func (m *Manager) updateAgentState(ctx context.Context, runner *AgentRunner) error {
	// Update from portfolio tracker if available
	if runner.Portfolio != nil {
		balance := runner.Portfolio.GetBalance()
		equity := runner.Portfolio.GetEquity()
		pnl := equity - runner.State.InitialBalance.InexactFloat64()

		runner.State.Balance = models.NewDecimal(balance)
		runner.State.Equity = models.NewDecimal(equity)
		runner.State.PnL = models.NewDecimal(pnl)
	}

	// Save to database
	return m.repository.CreateAgentState(ctx, runner.State)
}

// selectAIProviderForAgent selects appropriate AI provider for agent
func (m *Manager) selectAIProviderForAgent(config *models.AgentConfig) ai.Provider {
	// Strategy: Assign AI providers based on agent personality
	// Conservative agents -> Claude (careful analysis)
	// Aggressive agents -> DeepSeek (fast, cost-effective)
	// News traders -> GPT (good at understanding context)
	// Others -> round-robin

	preferredProvider := ""
	switch config.Personality {
	case models.PersonalityConservative:
		preferredProvider = "Claude"
	case models.PersonalityAggressive, models.PersonalityScalper:
		preferredProvider = "DeepSeek"
	case models.PersonalityNewsTrader:
		preferredProvider = "GPT"
	}

	// Try preferred provider first
	if preferredProvider != "" {
		if provider, ok := m.aiProviders[preferredProvider]; ok {
			return provider
		}
	}

	// Fallback to any available provider
	for _, provider := range m.aiProviders {
		return provider
	}

	return nil
}

// StopAgent stops a running agent
func (m *Manager) StopAgent(ctx context.Context, agentID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	runner, exists := m.runningAgents[agentID]
	if !exists {
		return fmt.Errorf("agent not running")
	}

	runner.CancelFunc()
	runner.IsRunning = false

	// Update state
	runner.State.IsTrading = false
	if err := m.repository.CreateAgentState(ctx, runner.State); err != nil {
		logger.Error("failed to update agent state", zap.Error(err))
	}

	delete(m.runningAgents, agentID)

	logger.Info("agent stopped",
		zap.String("agent_id", agentID),
		zap.String("name", runner.Config.Name),
	)

	return nil
}

// GetRunningAgents returns list of running agents
func (m *Manager) GetRunningAgents() []*AgentRunner {
	m.mu.RLock()
	defer m.mu.RUnlock()

	runners := make([]*AgentRunner, 0, len(m.runningAgents))
	for _, runner := range m.runningAgents {
		if runner.IsRunning {
			runners = append(runners, runner)
		}
	}

	return runners
}

// GetAgentRunner returns specific agent runner
func (m *Manager) GetAgentRunner(agentID string) (*AgentRunner, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	runner, exists := m.runningAgents[agentID]
	return runner, exists
}

// Shutdown gracefully shuts down all agents
func (m *Manager) Shutdown() error {
	logger.Info("shutting down agent manager...")

	m.mu.Lock()
	defer m.mu.Unlock()

	for agentID, runner := range m.runningAgents {
		logger.Info("stopping agent",
			zap.String("agent_id", agentID),
			zap.String("name", runner.Config.Name),
		)

		runner.CancelFunc()
		runner.IsRunning = false

		// Update state
		runner.State.IsTrading = false
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := m.repository.CreateAgentState(ctx, runner.State); err != nil {
			logger.Error("failed to update agent state", zap.Error(err))
		}
		cancel()
	}

	m.cancel()

	logger.Info("agent manager shut down complete")
	return nil
}
