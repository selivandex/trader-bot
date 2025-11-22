package bot

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/alexanderselivanov/trader/internal/adapters/ai"
	"github.com/alexanderselivanov/trader/internal/adapters/config"
	"github.com/alexanderselivanov/trader/internal/adapters/database"
	"github.com/alexanderselivanov/trader/internal/adapters/exchange"
	"github.com/alexanderselivanov/trader/internal/adapters/news"
	"github.com/alexanderselivanov/trader/internal/portfolio"
	"github.com/alexanderselivanov/trader/internal/risk"
	"github.com/alexanderselivanov/trader/internal/strategy"
	"github.com/alexanderselivanov/trader/internal/users"
	"github.com/alexanderselivanov/trader/pkg/logger"
)

// Manager manages multiple user bots
type Manager struct {
	mu             sync.RWMutex
	db             *database.DB
	userRepo       *users.Repository
	globalConfig   *config.Config
	aiEnsemble     *ai.Ensemble
	newsAggregator *news.Aggregator
	userBots       map[int64]*UserBot // userID -> bot instance
	ctx            context.Context
	cancel         context.CancelFunc
}

// UserBot represents single user's trading bot instance
type UserBot struct {
	UserID      int64
	Engine      *strategy.Engine
	Portfolio   *portfolio.Tracker
	Exchange    exchange.Exchange
	RiskManager *strategy.RiskManager
	CancelFunc  context.CancelFunc
	IsRunning   bool
}

// NewManager creates new bot manager
func NewManager(
	db *database.DB,
	globalConfig *config.Config,
	aiEnsemble *ai.Ensemble,
	newsAggregator *news.Aggregator,
) *Manager {
	ctx, cancel := context.WithCancel(context.Background())

	return &Manager{
		db:             db,
		userRepo:       users.NewRepository(db),
		globalConfig:   globalConfig,
		aiEnsemble:     aiEnsemble,
		newsAggregator: newsAggregator,
		userBots:       make(map[int64]*UserBot),
		ctx:            ctx,
		cancel:         cancel,
	}
}

// Start starts the bot manager
func (m *Manager) Start(ctx context.Context) error {
	logger.Info("bot manager starting...")

	// Load all active users with trading enabled
	activeUsers, err := m.userRepo.GetAllActiveUsers(ctx)
	if err != nil {
		return fmt.Errorf("failed to load active users: %w", err)
	}

	// Start bots for users with trading enabled
	for _, user := range activeUsers {
		config, err := m.userRepo.GetConfig(ctx, user.ID)
		if err != nil {
			logger.Error("failed to load user config", zap.Int64("user_id", user.ID), zap.Error(err))
			continue
		}

		if config != nil && config.IsTrading {
			if err := m.StartUserBot(ctx, user.ID); err != nil {
				logger.Error("failed to start user bot",
					zap.Int64("user_id", user.ID),
					zap.Error(err),
				)
			}
		}
	}

	logger.Info("bot manager started",
		zap.Int("active_bots", len(m.userBots)),
	)

	// Monitor loop
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return m.shutdown()
		case <-ticker.C:
			m.healthCheck(ctx)
		}
	}
}

// StartUserBot starts trading bot for specific user
func (m *Manager) StartUserBot(ctx context.Context, userID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if already running
	if bot, exists := m.userBots[userID]; exists && bot.IsRunning {
		return fmt.Errorf("bot already running for user %d", userID)
	}

	// Load user configuration
	userConfig, err := m.userRepo.GetConfig(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to load user config: %w", err)
	}

	if userConfig == nil {
		return fmt.Errorf("user config not found")
	}

	// Create exchange adapter
	var ex exchange.Exchange
	exchangeConfig := &config.ExchangeConfig{
		APIKey:  userConfig.APIKey,
		Secret:  userConfig.APISecret,
		Testnet: userConfig.Testnet,
	}

	switch userConfig.Exchange {
	case "binance":
		ex, err = exchange.NewBinanceAdapter(exchangeConfig)
	case "bybit":
		ex, err = exchange.NewBybitAdapter(exchangeConfig)
	default:
		return fmt.Errorf("unsupported exchange: %s", userConfig.Exchange)
	}

	if err != nil {
		return fmt.Errorf("failed to create exchange adapter: %w", err)
	}

	// Create trading config from user config
	tradingConfig := &config.TradingConfig{
		Symbol:                    userConfig.Symbol,
		InitialBalance:            userConfig.InitialBalance.Float64(),
		MaxPositionPercent:        userConfig.MaxPositionPercent.Float64(),
		MaxLeverage:               userConfig.MaxLeverage,
		StopLossPercent:           userConfig.StopLossPercent.Float64(),
		TakeProfitPercent:         userConfig.TakeProfitPercent.Float64(),
		ProfitWithdrawalThreshold: 1.1, // 10% profit target
	}

	// Create risk manager
	riskManager := &strategy.RiskManager{
		CircuitBreaker: risk.NewCircuitBreaker(&m.globalConfig.Risk),
		PositionSizer:  risk.NewPositionSizer(tradingConfig),
		Validator:      risk.NewValidator(),
	}

	// Create portfolio tracker (user-specific)
	portfolioTracker := portfolio.NewUserTracker(m.db, ex, userID, tradingConfig)
	if err := portfolioTracker.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize portfolio: %w", err)
	}

	// Create user-specific config
	userEngineConfig := &config.Config{
		Trading: *tradingConfig,
		AI:      m.globalConfig.AI,
		Risk:    m.globalConfig.Risk,
	}

	// Create trading engine
	engine := strategy.NewUserEngine(
		userID,
		userEngineConfig,
		ex,
		m.aiEnsemble,
		m.newsAggregator,
		riskManager,
		portfolioTracker,
		nil, // telegram will be set separately
	)

	// Create user bot context
	botCtx, botCancel := context.WithCancel(m.ctx)

	userBot := &UserBot{
		UserID:      userID,
		Engine:      engine,
		Portfolio:   portfolioTracker,
		Exchange:    ex,
		RiskManager: riskManager,
		CancelFunc:  botCancel,
		IsRunning:   true,
	}

	m.userBots[userID] = userBot

	// Start engine in goroutine
	go func() {
		logger.Info("starting user bot",
			zap.Int64("user_id", userID),
			zap.String("exchange", userConfig.Exchange),
			zap.String("symbol", userConfig.Symbol),
		)

		if err := engine.Start(botCtx); err != nil && err != context.Canceled {
			logger.Error("user bot error",
				zap.Int64("user_id", userID),
				zap.Error(err),
			)
		}
	}()

	// Update trading status
	if err := m.userRepo.SetTradingStatus(ctx, userID, true); err != nil {
		logger.Error("failed to update trading status", zap.Error(err))
	}

	logger.Info("user bot started",
		zap.Int64("user_id", userID),
		zap.String("exchange", userConfig.Exchange),
	)

	return nil
}

// StopUserBot stops trading bot for specific user
func (m *Manager) StopUserBot(ctx context.Context, userID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	bot, exists := m.userBots[userID]
	if !exists {
		return fmt.Errorf("bot not found for user %d", userID)
	}

	if !bot.IsRunning {
		return fmt.Errorf("bot not running for user %d", userID)
	}

	// Stop the bot
	bot.CancelFunc()
	bot.IsRunning = false
	bot.Engine.Stop()

	// Close exchange connection
	if err := bot.Exchange.Close(); err != nil {
		logger.Warn("failed to close exchange", zap.Error(err))
	}

	// Update trading status
	if err := m.userRepo.SetTradingStatus(ctx, userID, false); err != nil {
		logger.Error("failed to update trading status", zap.Error(err))
	}

	logger.Info("user bot stopped", zap.Int64("user_id", userID))

	return nil
}

// GetUserBot returns user bot instance
func (m *Manager) GetUserBot(userID int64) (*UserBot, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	bot, exists := m.userBots[userID]
	return bot, exists
}

// GetActiveBotCount returns number of active bots
func (m *Manager) GetActiveBotCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, bot := range m.userBots {
		if bot.IsRunning {
			count++
		}
	}
	return count
}

// healthCheck monitors bot health
func (m *Manager) healthCheck(ctx context.Context) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for userID, bot := range m.userBots {
		if !bot.IsRunning {
			continue
		}

		// Check if bot is still supposed to be trading
		config, err := m.userRepo.GetConfig(ctx, userID)
		if err != nil {
			logger.Error("health check: failed to load config",
				zap.Int64("user_id", userID),
				zap.Error(err),
			)
			continue
		}

		// Stop bot if trading is disabled
		if config != nil && !config.IsTrading {
			logger.Info("health check: stopping bot (trading disabled)",
				zap.Int64("user_id", userID),
			)
			go m.StopUserBot(ctx, userID)
		}
	}
}

// shutdown gracefully shuts down all bots
func (m *Manager) shutdown() error {
	logger.Info("shutting down bot manager...")

	m.mu.Lock()
	defer m.mu.Unlock()

	for userID, bot := range m.userBots {
		logger.Info("stopping user bot", zap.Int64("user_id", userID))

		bot.CancelFunc()
		bot.Engine.Stop()

		if err := bot.Exchange.Close(); err != nil {
			logger.Warn("failed to close exchange",
				zap.Int64("user_id", userID),
				zap.Error(err),
			)
		}
	}

	logger.Info("bot manager shut down complete")
	return nil
}

// GetUserRepository returns user repository
func (m *Manager) GetUserRepository() *users.Repository {
	return m.userRepo
}
