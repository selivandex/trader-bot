package agents

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/sashabaranov/go-openai"
	"go.uber.org/zap"

	"github.com/selivandex/trader-bot/internal/adapters/ai"
	"github.com/selivandex/trader-bot/internal/adapters/exchange"
	"github.com/selivandex/trader-bot/internal/adapters/market"
	"github.com/selivandex/trader-bot/internal/adapters/news"
	redisAdapter "github.com/selivandex/trader-bot/internal/adapters/redis"
	"github.com/selivandex/trader-bot/internal/indicators"
	"github.com/selivandex/trader-bot/pkg/logger"
	"github.com/selivandex/trader-bot/pkg/models"
	"github.com/selivandex/trader-bot/pkg/templates"
)

// Notifier interface for sending notifications
type Notifier interface {
	SendTradeAlert(ctx context.Context, userID, agentName, action, symbol string, size, price, pnl float64) error
	SendAgentStarted(ctx context.Context, userID, agentName, symbol string, budget float64) error
	SendAgentStopped(ctx context.Context, userID, agentName, symbol string, finalPnL float64) error
	SendCircuitBreakerAlert(ctx context.Context, userID, agentName, reason string) error
	SendErrorAlert(ctx context.Context, userID, agentName, errorMsg string) error
}

// AgenticManager manages autonomous AI agents with full thinking capabilities
// This is the upgraded manager that uses Chain-of-Thought, Memory, Reflection, and Planning
type AgenticManager struct {
	mu              sync.RWMutex
	db              *sqlx.DB
	redisClient     *redisAdapter.Client
	lockFactory     redisAdapter.LockFactory // Factory for creating distributed locks
	repository      *Repository
	marketRepo      *market.Repository
	newsAggregator  *news.Aggregator
	newsCache       *news.Cache                   // Direct access to news cache for toolkit
	templateManager *templates.Manager            // Global templates for validators
	notifier        Notifier                      // Telegram notifier (can be nil)
	aiProviders     map[string]ai.AgenticProvider // Only agentic providers
	embeddingClient *openai.Client                // OpenAI client for semantic memory embeddings
	runningAgents   map[string]*AgenticRunner     // agentID -> runner
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup // For graceful shutdown
	shutdownOnce    sync.Once      // Ensure shutdown runs once
}

// AgenticRunner represents a fully autonomous agent
type AgenticRunner struct {
	Config           *models.AgentConfig
	State            *models.AgentState
	CoTEngine        *ChainOfThoughtEngine  // Chain-of-Thought reasoning
	ReflectionEngine *ReflectionEngine      // Post-trade learning
	PlanningEngine   *PlanningEngine        // Forward planning
	MemoryManager    *SemanticMemoryManager // Episodic memory
	ValidatorCouncil *ValidatorCouncil      // Multi-AI validator consensus
	Exchange         exchange.Exchange
	Lock             redisAdapter.AgentLock // Distributed lock for K8s (interface)
	Notifier         Notifier               // Telegram notifier (can be nil)
	CancelFunc       context.CancelFunc
	IsRunning        bool
	LastDecisionAt   time.Time
	LastReflectionAt time.Time
	LastPlanningAt   time.Time
}

// NewAgenticManager creates new agentic agent manager
func NewAgenticManager(
	db *sqlx.DB,
	redisClient *redisAdapter.Client,
	lockFactory redisAdapter.LockFactory,
	marketRepo *market.Repository,
	newsAggregator *news.Aggregator,
	newsCache *news.Cache,
	templateManager *templates.Manager,
	aiProviders []ai.Provider,
	notifier Notifier, // Can be nil if telegram disabled
	embeddingClient *openai.Client, // For semantic memory embeddings
) *AgenticManager {
	ctx, cancel := context.WithCancel(context.Background())

	// Filter only agentic providers
	agenticProviders := make(map[string]ai.AgenticProvider)
	for _, provider := range aiProviders {
		if agenticProvider := ai.GetAgenticProvider(provider); agenticProvider != nil {
			agenticProviders[provider.GetName()] = agenticProvider
		}
	}

	if len(agenticProviders) == 0 {
		logger.Warn("⚠️ No agentic AI providers available - agents will have limited autonomous capabilities")
	}

	return &AgenticManager{
		db:              db,
		redisClient:     redisClient,
		lockFactory:     lockFactory,
		repository:      NewRepository(db),
		marketRepo:      marketRepo,
		newsAggregator:  newsAggregator,
		newsCache:       newsCache,
		templateManager: templateManager,
		notifier:        notifier,
		aiProviders:     agenticProviders,
		embeddingClient: embeddingClient,
		runningAgents:   make(map[string]*AgenticRunner),
		ctx:             ctx,
		cancel:          cancel,
	}
}

// StartAgenticAgent starts a fully autonomous agent
func (am *AgenticManager) StartAgenticAgent(
	ctx context.Context,
	agentID string,
	symbol string,
	initialBalance float64,
	exchangeAdapter exchange.Exchange,
) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	// Check if already running in this pod
	if runner, exists := am.runningAgents[agentID]; exists && runner.IsRunning {
		return fmt.Errorf("agent already running in this pod")
	}

	// Try to acquire distributed lock (for multi-pod deployments)
	lock := am.lockFactory.CreateAgentLock(agentID)
	acquired, err := lock.TryAcquire(ctx)
	if err != nil {
		return fmt.Errorf("failed to acquire agent lock: %w", err)
	}

	if !acquired {
		return fmt.Errorf("agent is already running in another pod")
	}

	// Load agent config
	config, err := am.repository.GetAgent(ctx, agentID)
	if err != nil {
		// Release lock if config load fails
		lock.Release(ctx)
		return fmt.Errorf("failed to load agent config: %w", err)
	}

	if !config.IsActive {
		lock.Release(ctx)
		return fmt.Errorf("agent is not active")
	}

	// Select AI provider
	aiProvider := am.selectAgenticProvider(config)
	if aiProvider == nil {
		return fmt.Errorf("no agentic AI providers available")
	}

	logger.Info("🧠 Assigned agentic AI provider",
		zap.String("agent_id", agentID),
		zap.String("provider", aiProvider.GetName()),
	)

	// Initialize or load agent state
	state, err := am.repository.GetAgentState(ctx, agentID, symbol)
	if err != nil {
		state = &models.AgentState{
			AgentID:        agentID,
			Symbol:         symbol,
			Balance:        models.NewDecimal(initialBalance),
			InitialBalance: models.NewDecimal(initialBalance),
			Equity:         models.NewDecimal(initialBalance),
			PnL:            models.NewDecimal(0),
			IsTrading:      true,
		}

		if err := am.repository.CreateAgentState(ctx, state); err != nil {
			return fmt.Errorf("failed to create agent state: %w", err)
		}
	}

	// Create all agent components
	memoryManager := NewSemanticMemoryManager(am.repository, aiProvider, am.embeddingClient)
	cotEngine := NewChainOfThoughtEngine(config, aiProvider, memoryManager)
	reflectionEngine := NewReflectionEngine(config, aiProvider, am.repository, memoryManager)
	planningEngine := NewPlanningEngine(config, aiProvider, am.repository, memoryManager)

	// Create validator council with all available AI providers for consensus
	validatorCouncil := NewValidatorCouncil(config, am.aiProviders, nil, am.templateManager)

	// Create agent context
	agentCtx, agentCancel := context.WithCancel(am.ctx)

	runner := &AgenticRunner{
		Config:           config,
		State:            state,
		CoTEngine:        cotEngine,
		ReflectionEngine: reflectionEngine,
		PlanningEngine:   planningEngine,
		MemoryManager:    memoryManager,
		ValidatorCouncil: validatorCouncil,
		Exchange:         exchangeAdapter,
		Lock:             lock, // Store lock in runner
		Notifier:         am.notifier,
		CancelFunc:       agentCancel,
		IsRunning:        true,
		LastDecisionAt:   time.Now(),
		LastReflectionAt: time.Now(),
		LastPlanningAt:   time.Now(),
	}

	// Initialize toolkit for agent (NEW)
	am.initializeToolkit(runner)

	am.runningAgents[agentID] = runner

	// Start agent goroutine with WaitGroup
	am.wg.Add(1)
	go func() {
		defer am.wg.Done()
		am.runAgenticAgent(agentCtx, runner)
	}()

	logger.Info("🤖 Autonomous agent started",
		zap.String("agent_id", agentID),
		zap.String("name", config.Name),
		zap.String("symbol", symbol),
		zap.Float64("initial_balance", initialBalance),
	)

	// Send notification
	if am.notifier != nil {
		if err := am.notifier.SendAgentStarted(ctx, config.UserID, config.Name, symbol, initialBalance); err != nil {
			logger.Warn("failed to send agent started notification", zap.Error(err))
		}
	}

	return nil
}

// runAgenticAgent is the main autonomous agent loop
func (am *AgenticManager) runAgenticAgent(ctx context.Context, runner *AgenticRunner) {
	ticker := time.NewTicker(runner.Config.DecisionInterval)
	defer ticker.Stop()

	logger.Info("🧠 Autonomous agent loop started",
		zap.String("agent_id", runner.Config.ID),
		zap.String("name", runner.Config.Name),
	)

	// Create initial plan
	if err := am.createInitialPlan(ctx, runner); err != nil {
		logger.Error("failed to create initial plan", zap.Error(err))
	}

	// Main agent loop
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

			// Execute one autonomous thinking cycle
			if err := am.executeAgenticCycle(ctx, runner); err != nil {
				logger.Error("agentic cycle failed",
					zap.String("agent_id", runner.Config.ID),
					zap.Error(err),
				)
			}

			// Periodic self-reflection (every 24h or after 20 trades)
			if time.Since(runner.LastReflectionAt) > 24*time.Hour {
				if err := runner.ReflectionEngine.ReflectPeriodically(ctx, runner.State.Symbol); err != nil {
					logger.Error("periodic reflection failed", zap.Error(err))
				} else {
					runner.LastReflectionAt = time.Now()
				}
			}

			// Memory consolidation (forget unimportant memories weekly)
			if time.Now().Weekday() == time.Sunday && time.Now().Hour() == 0 {
				runner.MemoryManager.Forget(ctx, runner.Config.ID, 0.3)
			}
		}
	}
}

// executeAgenticCycle executes one autonomous thinking cycle
func (am *AgenticManager) executeAgenticCycle(ctx context.Context, runner *AgenticRunner) error {
	logger.Debug("🤔 Agent thinking cycle",
		zap.String("agent_id", runner.Config.ID),
	)

	// Step 1: Collect market data
	marketData, err := am.collectMarketData(ctx, runner)
	if err != nil {
		return fmt.Errorf("failed to collect market data: %w", err)
	}

	// Step 2: Check if should revise plan
	shouldRevise, reason := runner.PlanningEngine.ShouldRevisePlan(marketData)
	if shouldRevise {
		logger.Info("📋 Revising plan",
			zap.String("agent", runner.Config.Name),
			zap.String("reason", reason),
		)

		// Create new 24h plan
		_, err := runner.PlanningEngine.CreatePlan(ctx, marketData, nil, 24*time.Hour)
		if err != nil {
			logger.Warn("failed to create plan", zap.Error(err))
		}
		runner.LastPlanningAt = time.Now()
	}

	// Step 3: Get current position
	position, _ := runner.Exchange.FetchPosition(ctx, runner.State.Symbol)

	// Step 4: Execute Chain-of-Thought reasoning
	decision, reasoningTrace, err := runner.CoTEngine.Think(ctx, marketData, position)
	if err != nil {
		return fmt.Errorf("thinking failed: %w", err)
	}

	runner.LastDecisionAt = time.Now()

	// Step 5: Save decision with reasoning trace
	if err := am.repository.SaveDecision(ctx, decision); err != nil {
		logger.Error("failed to save decision", zap.Error(err))
	}

	logger.Info("💭 Autonomous decision made",
		zap.String("agent", runner.Config.Name),
		zap.String("action", string(decision.Action)),
		zap.Int("confidence", decision.Confidence),
		zap.Int("reasoning_steps", len(reasoningTrace.ChainOfThought.Steps)),
	)

	// Step 6: Validate decision through validator council (if not HOLD)
	if decision.Action != models.ActionHold && runner.ValidatorCouncil != nil {
		if runner.ValidatorCouncil.ShouldValidate(decision) {
			logger.Info("🏛️ Submitting decision to validator council",
				zap.String("agent", runner.Config.Name),
				zap.String("action", string(decision.Action)),
			)

			position, _ := runner.Exchange.FetchPosition(ctx, runner.State.Symbol)
			consensusResult, err := runner.ValidatorCouncil.ValidateDecision(ctx, decision, marketData, position)
			if err != nil {
				logger.Error("validator council failed, skipping execution",
					zap.String("agent", runner.Config.Name),
					zap.Error(err),
				)
				return err
			}

			// Log validator votes
			logger.Info("🏛️ Validator council verdict",
				zap.String("agent", runner.Config.Name),
				zap.String("verdict", string(consensusResult.FinalVerdict)),
				zap.Float64("approval_rate", consensusResult.ApprovalRate),
				zap.Bool("execution_allowed", consensusResult.ExecutionAllowed),
			)

			// Add consensus summary to decision reason
			decision.Reason += "\n\n" + consensusResult.ConsensusSummary

			// Only execute if council approves
			if !consensusResult.ExecutionAllowed {
				logger.Warn("⛔ Decision REJECTED by validator council, not executing",
					zap.String("agent", runner.Config.Name),
					zap.String("action", string(decision.Action)),
					zap.Float64("approval_rate", consensusResult.ApprovalRate),
				)
				// Save decision as rejected
				decision.Executed = false
				decision.Outcome = `{"rejected_by_council": true, "reason": "Failed validator consensus"}`
				return nil
			}

			logger.Info("✅ Decision APPROVED by validator council, executing",
				zap.String("agent", runner.Config.Name),
				zap.String("action", string(decision.Action)),
			)
		}
	}

	// Step 7: Execute decision if approved (or if validation not required)
	if decision.Action != models.ActionHold {
		position, _ := runner.Exchange.FetchPosition(ctx, runner.State.Symbol)

		if err := am.executeAgenticDecision(ctx, runner, decision, position); err != nil {
			logger.Error("failed to execute decision",
				zap.String("agent", runner.Config.Name),
				zap.Error(err),
			)
			return err
		}
	}

	// Step 8: Update agent state
	am.updateAgentState(ctx, runner)

	return nil
}

// createInitialPlan creates 24h plan when agent starts
func (am *AgenticManager) createInitialPlan(ctx context.Context, runner *AgenticRunner) error {
	marketData, err := am.collectMarketData(ctx, runner)
	if err != nil {
		return err
	}

	_, err = runner.PlanningEngine.CreatePlan(ctx, marketData, nil, 24*time.Hour)
	if err != nil {
		return fmt.Errorf("failed to create initial plan: %w", err)
	}

	runner.LastPlanningAt = time.Now()
	return nil
}

// collectMarketData collects market data for agent (from cache)
func (am *AgenticManager) collectMarketData(ctx context.Context, runner *AgenticRunner) (*models.MarketData, error) {
	symbol := runner.State.Symbol

	// Get latest ticker from exchange (real-time price critical for execution)
	ticker, err := runner.Exchange.FetchTicker(ctx, symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch ticker: %w", err)
	}

	// Get candles for multiple timeframes from database cache
	timeframes := []string{"5m", "15m", "1h", "4h"}
	candlesMap := make(map[string][]models.Candle)

	for _, tf := range timeframes {
		candles, err := am.marketRepo.GetCandles(ctx, symbol, tf, 100)
		if err != nil {
			// Fallback to exchange if cache empty
			logger.Warn("candles cache miss, fetching from exchange",
				zap.String("symbol", symbol),
				zap.String("timeframe", tf),
			)
			candles, err = runner.Exchange.FetchOHLCV(ctx, symbol, tf, 100)
			if err != nil {
				logger.Warn("failed to fetch candles", zap.Error(err))
				continue // Skip this timeframe
			}
		}
		candlesMap[tf] = candles
	}

	// Calculate technical indicators for multiple timeframes
	calc := indicators.NewCalculator()
	var technicalIndicators *models.TechnicalIndicators

	// Use 1h as primary timeframe for indicators structure
	if candles1h, ok := candlesMap["1h"]; ok && len(candles1h) >= 26 {
		technicalIndicators, err = calc.Calculate(candles1h)
		if err != nil {
			logger.Warn("failed to calculate 1h indicators", zap.Error(err))
		}

		// Calculate RSI for other timeframes and add to map
		if technicalIndicators != nil && technicalIndicators.RSI != nil {
			for tf, candles := range candlesMap {
				if tf != "1h" && len(candles) >= 14 {
					// Calculate RSI for other timeframes
					rsiValue, err := calc.CalculateRSI(candles, 14)
					if err == nil {
						technicalIndicators.RSI[tf] = models.NewDecimal(rsiValue)
					}
				}
			}
		}
	}

	// Get cached news
	var newsSummary *models.NewsSummary
	if am.newsAggregator != nil {
		newsSummary, err = am.newsAggregator.GetCachedSummary(ctx, 6*time.Hour)
		if err != nil {
			logger.Warn("failed to get cached news", zap.Error(err))
		}
	}

	// Get on-chain data from cache
	onChainData := am.getOnChainSummary(ctx, symbol)

	marketData := &models.MarketData{
		Symbol:      symbol,
		Ticker:      ticker,
		Candles:     candlesMap,
		Indicators:  technicalIndicators,
		NewsSummary: newsSummary,
		OnChainData: onChainData,
		Timestamp:   time.Now(),
	}

	return marketData, nil
}

// updateAgentState updates agent state in database
func (am *AgenticManager) updateAgentState(ctx context.Context, runner *AgenticRunner) error {
	// Update balance/equity from current state
	// AgentState is already updated in executeDecision when trades complete
	return am.repository.CreateAgentState(ctx, runner.State)
}

// StopAgenticAgent stops autonomous agent
func (am *AgenticManager) StopAgenticAgent(ctx context.Context, agentID string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	runner, exists := am.runningAgents[agentID]
	if !exists {
		return fmt.Errorf("agent not running")
	}

	runner.CancelFunc()
	runner.IsRunning = false

	// Update state
	runner.State.IsTrading = false
	if err := am.repository.CreateAgentState(ctx, runner.State); err != nil {
		logger.Error("failed to update agent state", zap.Error(err))
	}

	delete(am.runningAgents, agentID)

	logger.Info("🛑 Autonomous agent stopped",
		zap.String("agent_id", agentID),
		zap.String("name", runner.Config.Name),
	)

	// Send notification
	if am.notifier != nil {
		finalPnL, _ := runner.State.PnL.Float64()
		symbol := runner.State.Symbol
		if err := am.notifier.SendAgentStopped(ctx, runner.Config.UserID, runner.Config.Name, symbol, finalPnL); err != nil {
			logger.Warn("failed to send agent stopped notification", zap.Error(err))
		}
	}

	return nil
}

// GetAgenticRunner returns specific agentic agent runner
func (am *AgenticManager) GetAgenticRunner(agentID string) (*AgenticRunner, bool) {
	am.mu.RLock()
	defer am.mu.RUnlock()

	runner, exists := am.runningAgents[agentID]
	return runner, exists
}

// GetRunningAgents returns all running agentic agents
func (am *AgenticManager) GetRunningAgents() []*AgenticRunner {
	am.mu.RLock()
	defer am.mu.RUnlock()

	runners := make([]*AgenticRunner, 0, len(am.runningAgents))
	for _, runner := range am.runningAgents {
		if runner.IsRunning {
			runners = append(runners, runner)
		}
	}

	return runners
}

// RestoreRunningAgents restores agents that were running before pod restart
// This is called on startup to recover agents with distributed locking
func (am *AgenticManager) RestoreRunningAgents(ctx context.Context, exchangeFactory func(string, string, string, bool) (exchange.Exchange, error)) error {
	logger.Info("🔄 restoring running agents from database...")

	// Get list of agents that should be running
	agentsToRestore, err := am.repository.GetAgentsToRestore(ctx)
	if err != nil {
		return fmt.Errorf("failed to get agents to restore: %w", err)
	}

	if len(agentsToRestore) == 0 {
		logger.Info("no agents to restore")
		return nil
	}

	logger.Info("found agents to restore",
		zap.Int("count", len(agentsToRestore)),
	)

	restoredCount := 0
	skippedCount := 0
	failedCount := 0

	for _, agentInfo := range agentsToRestore {
		// Try to acquire lock first (avoid starting if another pod already has it)
		lock := am.lockFactory.CreateAgentLock(agentInfo.AgentID)
		acquired, err := lock.TryAcquire(ctx)
		if err != nil {
			logger.Warn("failed to check agent lock",
				zap.String("agent_id", agentInfo.AgentID),
				zap.Error(err),
			)
			failedCount++
			continue
		}

		if !acquired {
			logger.Info("agent already running in another pod (lock held), skipping",
				zap.String("agent_id", agentInfo.AgentID),
				zap.String("symbol", agentInfo.Symbol),
			)
			skippedCount++
			continue
		}

		// Release lock immediately - StartAgenticAgent will acquire it again
		lock.Release(ctx)

		// Create exchange adapter
		exchangeAdapter, err := exchangeFactory(agentInfo.Exchange, agentInfo.APIKey, agentInfo.APISecret, agentInfo.Testnet)
		if err != nil {
			logger.Error("failed to create exchange adapter for agent recovery",
				zap.String("agent_id", agentInfo.AgentID),
				zap.String("exchange", agentInfo.Exchange),
				zap.Error(err),
			)
			failedCount++
			continue
		}

		// Start agent with current balance
		err = am.StartAgenticAgent(ctx, agentInfo.AgentID, agentInfo.Symbol, agentInfo.Balance, exchangeAdapter)
		if err != nil {
			logger.Error("failed to restore agent",
				zap.String("agent_id", agentInfo.AgentID),
				zap.String("symbol", agentInfo.Symbol),
				zap.Error(err),
			)
			failedCount++
			continue
		}

		logger.Info("✅ agent restored successfully",
			zap.String("agent_id", agentInfo.AgentID),
			zap.String("symbol", agentInfo.Symbol),
			zap.String("exchange", agentInfo.Exchange),
		)
		restoredCount++
	}

	logger.Info("🎯 agent restoration complete",
		zap.Int("total", len(agentsToRestore)),
		zap.Int("restored", restoredCount),
		zap.Int("skipped", skippedCount),
		zap.Int("failed", failedCount),
	)

	return nil
}

// Shutdown gracefully shuts down all agentic agents
func (am *AgenticManager) Shutdown() error {
	var shutdownErr error

	am.shutdownOnce.Do(func() {
		logger.Info("🛑 shutting down agentic agent manager...")

		// Cancel all agent contexts
		am.mu.Lock()
		for agentID, runner := range am.runningAgents {
			logger.Info("stopping autonomous agent",
				zap.String("agent_id", agentID),
				zap.String("name", runner.Config.Name),
			)

			runner.CancelFunc()
			runner.IsRunning = false
		}
		am.cancel()
		am.mu.Unlock()

		// Wait for all agents to finish with timeout (K8s gives 30s terminationGracePeriodSeconds)
		done := make(chan struct{})
		go func() {
			am.wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			logger.Info("✅ all agents stopped gracefully")
		case <-time.After(25 * time.Second):
			logger.Warn("⚠️ shutdown timeout, some agents may not have stopped cleanly")
		}

		// Save final states and release locks
		am.mu.Lock()
		defer am.mu.Unlock()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		for agentID, runner := range am.runningAgents {
			// Save final state
			runner.State.IsTrading = false
			if err := am.repository.CreateAgentState(ctx, runner.State); err != nil {
				logger.Error("failed to save final agent state",
					zap.String("agent_id", agentID),
					zap.Error(err),
				)
			}

			// Release distributed lock
			if runner.Lock != nil {
				if err := runner.Lock.Release(ctx); err != nil {
					logger.Error("failed to release agent lock",
						zap.String("agent_id", agentID),
						zap.Error(err),
					)
				}
			}
		}

		logger.Info("✅ agentic agent manager shut down complete")
	})

	return shutdownErr
}

// executeAgenticDecision executes trading decision
func (am *AgenticManager) executeAgenticDecision(
	ctx context.Context,
	runner *AgenticRunner,
	decision *models.AgentDecision,
	position *models.Position,
) error {
	logger.Info("🤖 Executing autonomous agent decision",
		zap.String("agent_id", runner.Config.ID),
		zap.String("agent_name", runner.Config.Name),
		zap.String("action", string(decision.Action)),
	)

	switch decision.Action {
	case models.ActionHold:
		return nil

	case models.ActionClose:
		if position == nil || position.Side == models.PositionNone {
			logger.Warn("no position to close")
			return nil
		}
		return am.closePosition(ctx, runner, position, decision)

	case models.ActionOpenLong:
		return am.openPosition(ctx, runner, models.PositionLong, decision)

	case models.ActionOpenShort:
		return am.openPosition(ctx, runner, models.PositionShort, decision)

	default:
		return fmt.Errorf("unsupported action: %s", decision.Action)
	}
}

// openPosition opens new position
func (am *AgenticManager) openPosition(
	ctx context.Context,
	runner *AgenticRunner,
	side models.PositionSide,
	decision *models.AgentDecision,
) error {
	balance, _ := runner.State.Balance.Float64()
	maxPositionPercent := runner.Config.Strategy.MaxPositionPercent
	leverage := runner.Config.Strategy.MaxLeverage

	ticker, err := runner.Exchange.FetchTicker(ctx, runner.State.Symbol)
	if err != nil {
		return fmt.Errorf("failed to fetch ticker: %w", err)
	}
	currentPrice, _ := ticker.Last.Float64()

	positionValue := balance * maxPositionPercent / 100
	size := positionValue / currentPrice

	if err := runner.Exchange.SetLeverage(ctx, runner.State.Symbol, leverage); err != nil {
		logger.Warn("failed to set leverage", zap.Error(err))
	}

	var orderSide models.OrderSide
	if side == models.PositionLong {
		orderSide = models.SideBuy
	} else {
		orderSide = models.SideSell
	}

	order, err := runner.Exchange.CreateOrder(ctx, runner.State.Symbol, models.TypeMarket, orderSide, size, 0)
	if err != nil {
		return fmt.Errorf("failed to create order: %w", err)
	}

	// Calculate SL/TP based on agent's strategy
	stopLossPercent := runner.Config.Strategy.StopLossPercent / 100
	takeProfitPercent := runner.Config.Strategy.TakeProfitPercent / 100

	var stopLoss, takeProfit float64
	var oppositeSide models.OrderSide

	if side == models.PositionLong {
		stopLoss = currentPrice * (1 - stopLossPercent)
		takeProfit = currentPrice * (1 + takeProfitPercent)
		oppositeSide = models.SideSell
	} else {
		stopLoss = currentPrice * (1 + stopLossPercent)
		takeProfit = currentPrice * (1 - takeProfitPercent)
		oppositeSide = models.SideBuy
	}

	// Set Stop Loss order (exit on loss)
	_, err = runner.Exchange.CreateOrder(
		ctx,
		runner.State.Symbol,
		models.TypeStopMarket, // Stop-market order
		oppositeSide,
		size,
		stopLoss,
	)
	if err != nil {
		logger.Warn("failed to create stop-loss order", zap.Error(err))
		// Continue despite error - position is open
	}

	// Set Take Profit order (exit on profit)
	_, err = runner.Exchange.CreateOrder(
		ctx,
		runner.State.Symbol,
		models.TypeTakeProfitMarket, // Take-profit market order
		oppositeSide,
		size,
		takeProfit,
	)
	if err != nil {
		logger.Warn("failed to create take-profit order", zap.Error(err))
		// Continue despite error - position is open
	}

	logger.Info("✅ Position opened by agent",
		zap.String("agent", runner.Config.Name),
		zap.String("personality", string(runner.Config.Personality)),
		zap.String("side", string(side)),
		zap.Float64("size", size),
		zap.Float64("entry_price", currentPrice),
		zap.Float64("stop_loss", stopLoss),
		zap.Float64("take_profit", takeProfit),
		zap.Int("leverage", leverage),
		zap.Float64("sl_percent", runner.Config.Strategy.StopLossPercent),
		zap.Float64("tp_percent", runner.Config.Strategy.TakeProfitPercent),
	)

	decision.Executed = true
	decision.ExecutionPrice = order.Price
	decision.ExecutionSize = models.NewDecimal(size)

	return nil
}

// closePosition closes position and triggers reflection
func (am *AgenticManager) closePosition(
	ctx context.Context,
	runner *AgenticRunner,
	position *models.Position,
	decision *models.AgentDecision,
) error {
	var orderSide models.OrderSide
	if position.Side == models.PositionLong {
		orderSide = models.SideSell
	} else {
		orderSide = models.SideBuy
	}

	size, _ := position.Size.Float64()
	order, err := runner.Exchange.CreateOrder(ctx, runner.State.Symbol, models.TypeMarket, orderSide, size, 0)
	if err != nil {
		return fmt.Errorf("failed to close position: %w", err)
	}

	pnl, _ := position.UnrealizedPnL.Float64()
	exitPrice, _ := order.Price.Float64()

	logger.Info("✅ Position closed by agent",
		zap.String("agent", runner.Config.Name),
		zap.Float64("exit_price", exitPrice),
		zap.Float64("pnl", pnl),
	)

	runner.State.Balance = runner.State.Balance.Add(models.NewDecimal(pnl))
	runner.State.PnL = runner.State.PnL.Add(models.NewDecimal(pnl))

	decision.Executed = true
	decision.ExecutionPrice = order.Price

	// Trigger reflection
	tradeExp := &models.TradeExperience{
		Symbol:        runner.State.Symbol,
		Side:          string(position.Side),
		EntryPrice:    position.EntryPrice,
		ExitPrice:     order.Price,
		Size:          position.Size,
		PnL:           models.NewDecimal(pnl),
		PnLPercent:    (pnl / position.Margin.InexactFloat64()) * 100,
		Duration:      time.Since(position.Timestamp),
		EntryReason:   decision.Reason,
		ExitReason:    "Agent closed position",
		WasSuccessful: pnl > 0,
	}

	go func() {
		if err := runner.ReflectionEngine.Reflect(context.Background(), tradeExp); err != nil {
			logger.Error("reflection failed", zap.Error(err))
		}
	}()

	return nil
}

// selectAgenticProvider selects best agentic provider for agent
func (am *AgenticManager) selectAgenticProvider(config *models.AgentConfig) ai.AgenticProvider {
	// Strategy: Assign based on personality
	// Conservative -> Claude (thoughtful, analytical)
	// Aggressive -> DeepSeek (fast, decisive)
	// News traders -> GPT (context understanding)

	preferredProvider := ""
	switch config.Personality {
	case models.PersonalityConservative, models.PersonalitySwing:
		preferredProvider = "Claude"
	case models.PersonalityAggressive, models.PersonalityScalper:
		preferredProvider = "DeepSeek"
	case models.PersonalityNewsTrader:
		preferredProvider = "GPT"
	}

	// Try preferred provider first
	if preferredProvider != "" {
		if provider, ok := am.aiProviders[preferredProvider]; ok {
			return provider
		}
	}

	// Fallback to any available agentic provider
	for _, provider := range am.aiProviders {
		return provider
	}

	return nil
}

// CreateAgentFromPersonality creates new agent from preset personality
func (am *AgenticManager) CreateAgentFromPersonality(
	ctx context.Context,
	userID string,
	personality models.AgentPersonality,
	name string,
) (*models.AgentConfig, error) {
	presetFunc, ok := PresetAgentConfigs[personality]
	if !ok {
		return nil, fmt.Errorf("unknown personality: %s", personality)
	}

	config := presetFunc(userID, name)

	// Save to database
	savedConfig, err := am.repository.CreateAgent(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create agent: %w", err)
	}

	logger.Info("🎭 Autonomous agent created",
		zap.String("agent_id", savedConfig.ID),
		zap.String("name", savedConfig.Name),
		zap.String("personality", string(savedConfig.Personality)),
	)

	return savedConfig, nil
}

// getOnChainSummary builds on-chain summary from cached whale data
func (am *AgenticManager) getOnChainSummary(ctx context.Context, symbol string) *models.OnChainSummary {
	// Get recent whale transactions from cache (last 24h, impact >= 6)
	whaleTransactions, err := am.repository.GetRecentWhaleTransactions(ctx, symbol, 24, 6)
	if err != nil {
		logger.Warn("failed to get whale transactions", zap.Error(err))
		return nil
	}

	if len(whaleTransactions) == 0 {
		return nil // No on-chain data
	}

	// Get exchange flows
	flows, err := am.repository.GetExchangeFlows(ctx, symbol, 24)
	if err != nil {
		logger.Warn("failed to get exchange flows", zap.Error(err))
		flows = []models.ExchangeFlow{}
	}

	// Calculate net flow
	netFlow := models.NewDecimal(0)
	for _, flow := range flows {
		netFlow = netFlow.Add(flow.NetFlow)
	}

	// Determine flow direction
	flowDirection := "balanced"
	netFlowFloat := netFlow.InexactFloat64()
	if netFlowFloat < -1_000_000 {
		flowDirection = "outflow" // Accumulation (bullish)
	} else if netFlowFloat > 1_000_000 {
		flowDirection = "inflow" // Distribution (bearish)
	}

	// Determine whale activity level
	whaleActivity := "low"
	highImpactCount := 0
	for _, tx := range whaleTransactions {
		if tx.ImpactScore >= 8 {
			highImpactCount++
		}
	}

	if highImpactCount >= 3 {
		whaleActivity = "high"
	} else if highImpactCount >= 1 || len(whaleTransactions) >= 5 {
		whaleActivity = "medium"
	}

	summary := &models.OnChainSummary{
		Symbol:                symbol,
		WhaleActivity:         whaleActivity,
		ExchangeFlowDirection: flowDirection,
		NetExchangeFlow:       netFlow,
		RecentWhaleMovements:  whaleTransactions,
		UpdatedAt:             time.Now(),
	}

	logger.Debug("🐋 on-chain summary built",
		zap.String("symbol", symbol),
		zap.String("whale_activity", whaleActivity),
		zap.String("flow_direction", flowDirection),
		zap.Int("whale_count", len(whaleTransactions)),
		zap.Float64("net_flow_usd", netFlowFloat),
	)

	return summary
}
