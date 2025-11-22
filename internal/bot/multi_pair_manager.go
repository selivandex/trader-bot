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
	"github.com/alexanderselivanov/trader/pkg/models"
)

// MultiPairManager manages multiple trading pairs per user
type MultiPairManager struct {
	mu             sync.RWMutex
	db             *database.DB
	userRepo       *users.Repository
	globalConfig   *config.Config
	aiEnsemble     *ai.Ensemble
	newsAggregator *news.Aggregator
	userBots       map[int64]map[string]*UserBot // userID -> symbol -> bot
	ctx            context.Context
	cancel         context.CancelFunc
}

// NewMultiPairManager creates new multi-pair bot manager
func NewMultiPairManager(
	db *database.DB,
	globalConfig *config.Config,
	aiEnsemble *ai.Ensemble,
	newsAggregator *news.Aggregator,
) *MultiPairManager {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &MultiPairManager{
		db:             db,
		userRepo:       users.NewRepository(db),
		globalConfig:   globalConfig,
		aiEnsemble:     aiEnsemble,
		newsAggregator: newsAggregator,
		userBots:       make(map[int64]map[string]*UserBot),
		ctx:            ctx,
		cancel:         cancel,
	}
}

// Start starts the multi-pair bot manager
func (m *MultiPairManager) Start(ctx context.Context) error {
	logger.Info("multi-pair bot manager starting...")
	
	// Load all trading pairs
	pairs, err := m.userRepo.GetAllTradingPairs(ctx)
	if err != nil {
		return fmt.Errorf("failed to load trading pairs: %w", err)
	}
	
	// Start bots for each pair
	for _, pair := range pairs {
		if err := m.StartUserPairBot(ctx, pair.UserID, pair.Symbol); err != nil {
			logger.Error("failed to start pair bot",
				zap.Int64("user_id", pair.UserID),
				zap.String("symbol", pair.Symbol),
				zap.Error(err),
			)
		}
	}
	
	logger.Info("multi-pair bot manager started",
		zap.Int("active_pairs", m.getTotalActiveBots()),
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

// StartUserPairBot starts bot for specific user and symbol
func (m *MultiPairManager) StartUserPairBot(ctx context.Context, userID int64, symbol string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Initialize user map if needed
	if _, exists := m.userBots[userID]; !exists {
		m.userBots[userID] = make(map[string]*UserBot)
	}
	
	// Check if already running
	if bot, exists := m.userBots[userID][symbol]; exists && bot.IsRunning {
		return fmt.Errorf("bot already running for user %d, symbol %s", userID, symbol)
	}
	
	// Load configuration
	userConfig, err := m.userRepo.GetConfigBySymbol(ctx, userID, symbol)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	
	if userConfig == nil {
		return fmt.Errorf("config not found for symbol %s", symbol)
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
		return fmt.Errorf("failed to create exchange: %w", err)
	}
	
	// Create trading config
	tradingConfig := &config.TradingConfig{
		Symbol:                    userConfig.Symbol,
		InitialBalance:            models.ToFloat64(userConfig.InitialBalance),
		MaxPositionPercent:        models.ToFloat64(userConfig.MaxPositionPercent),
		MaxLeverage:               userConfig.MaxLeverage,
		StopLossPercent:           models.ToFloat64(userConfig.StopLossPercent),
		TakeProfitPercent:         models.ToFloat64(userConfig.TakeProfitPercent),
		ProfitWithdrawalThreshold: 1.1,
	}
	
	// Create risk manager
	riskManager := &strategy.RiskManager{
		CircuitBreaker: risk.NewCircuitBreaker(&m.globalConfig.Risk),
		PositionSizer:  risk.NewPositionSizer(tradingConfig),
		Validator:      risk.NewValidator(),
	}
	
	// Create portfolio tracker
	portfolioTracker := portfolio.NewUserTracker(m.db, ex, userID, tradingConfig)
	if err := portfolioTracker.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize portfolio: %w", err)
	}
	
	// Create engine config
	engineConfig := &config.Config{
		Trading: *tradingConfig,
		AI:      m.globalConfig.AI,
		Risk:    m.globalConfig.Risk,
	}
	
	// Create trading engine
	engine := strategy.NewUserEngine(
		userID,
		engineConfig,
		ex,
		m.aiEnsemble,
		m.newsAggregator,
		riskManager,
		portfolioTracker,
		nil,
	)
	
	// Create bot context
	botCtx, botCancel := context.WithCancel(m.ctx)
	
	userBot := &UserBot{
		UserID:      userID,
		Engine:      engine,
		Portfolio:   portfolioTracker.Tracker,
		Exchange:    ex,
		RiskManager: riskManager,
		CancelFunc:  botCancel,
		IsRunning:   true,
	}
	
	m.userBots[userID][symbol] = userBot
	
	// Start engine
	go func() {
		logger.Info("starting pair bot",
			zap.Int64("user_id", userID),
			zap.String("symbol", symbol),
			zap.String("exchange", userConfig.Exchange),
		)
		
		if err := engine.Start(botCtx); err != nil && err != context.Canceled {
			logger.Error("pair bot error",
				zap.Int64("user_id", userID),
				zap.String("symbol", symbol),
				zap.Error(err),
			)
		}
	}()
	
	logger.Info("pair bot started",
		zap.Int64("user_id", userID),
		zap.String("symbol", symbol),
	)
	
	return nil
}

// StopUserPairBot stops bot for specific user and symbol
func (m *MultiPairManager) StopUserPairBot(ctx context.Context, userID int64, symbol string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	userBots, exists := m.userBots[userID]
	if !exists {
		return fmt.Errorf("no bots found for user %d", userID)
	}
	
	bot, exists := userBots[symbol]
	if !exists {
		return fmt.Errorf("bot not found for symbol %s", symbol)
	}
	
	if !bot.IsRunning {
		return fmt.Errorf("bot not running")
	}
	
	// Stop bot
	bot.CancelFunc()
	bot.IsRunning = false
	bot.Engine.Stop()
	bot.Exchange.Close()
	
	delete(userBots, symbol)
	
	// Update status
	if err := m.userRepo.SetPairTradingStatus(ctx, userID, symbol, false); err != nil {
		logger.Error("failed to update trading status", zap.Error(err))
	}
	
	logger.Info("pair bot stopped",
		zap.Int64("user_id", userID),
		zap.String("symbol", symbol),
	)
	
	return nil
}

// StopAllUserBots stops all bots for user
func (m *MultiPairManager) StopAllUserBots(ctx context.Context, userID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	userBots, exists := m.userBots[userID]
	if !exists {
		return fmt.Errorf("no bots found for user %d", userID)
	}
	
	for symbol, bot := range userBots {
		if bot.IsRunning {
			bot.CancelFunc()
			bot.IsRunning = false
			bot.Engine.Stop()
			bot.Exchange.Close()
			
			if err := m.userRepo.SetPairTradingStatus(ctx, userID, symbol, false); err != nil {
				logger.Error("failed to update trading status", zap.Error(err))
			}
			
			logger.Info("stopped pair bot",
				zap.Int64("user_id", userID),
				zap.String("symbol", symbol),
			)
		}
	}
	
	delete(m.userBots, userID)
	
	return nil
}

// GetUserPairBot returns specific pair bot
func (m *MultiPairManager) GetUserPairBot(userID int64, symbol string) (*UserBot, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if userBots, ok := m.userBots[userID]; ok {
		bot, exists := userBots[symbol]
		return bot, exists
	}
	
	return nil, false
}

// GetUserBotCount returns number of active pairs for user
func (m *MultiPairManager) GetUserBotCount(userID int64) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if userBots, ok := m.userBots[userID]; ok {
		count := 0
		for _, bot := range userBots {
			if bot.IsRunning {
				count++
			}
		}
		return count
	}
	
	return 0
}

// GetActiveBotCount returns total active bots across all users
func (m *MultiPairManager) GetActiveBotCount() int {
	return m.getTotalActiveBots()
}

// getTotalActiveBots counts total active bots
func (m *MultiPairManager) getTotalActiveBots() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	count := 0
	for _, userBots := range m.userBots {
		for _, bot := range userBots {
			if bot.IsRunning {
				count++
			}
		}
	}
	return count
}

// healthCheck monitors bot health
func (m *MultiPairManager) healthCheck(ctx context.Context) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	for userID, userBots := range m.userBots {
		for symbol, bot := range userBots {
			if !bot.IsRunning {
				continue
			}
			
			// Check if still supposed to be trading
			config, err := m.userRepo.GetConfigBySymbol(ctx, userID, symbol)
			if err != nil {
				logger.Error("health check failed",
					zap.Int64("user_id", userID),
					zap.String("symbol", symbol),
					zap.Error(err),
				)
				continue
			}
			
			if config != nil && !config.IsTrading {
				logger.Info("stopping bot (trading disabled)",
					zap.Int64("user_id", userID),
					zap.String("symbol", symbol),
				)
				go m.StopUserPairBot(ctx, userID, symbol)
			}
		}
	}
}

// shutdown gracefully shuts down all bots
func (m *MultiPairManager) shutdown() error {
	logger.Info("shutting down multi-pair bot manager...")
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	for userID, userBots := range m.userBots {
		for symbol, bot := range userBots {
			logger.Info("stopping pair bot",
				zap.Int64("user_id", userID),
				zap.String("symbol", symbol),
			)
			
			bot.CancelFunc()
			bot.Engine.Stop()
			bot.Exchange.Close()
		}
	}
	
	logger.Info("multi-pair bot manager shut down complete")
	return nil
}

// GetUserRepository returns user repository
func (m *MultiPairManager) GetUserRepository() *users.Repository {
	return m.userRepo
}

// StartUserBot starts single bot (legacy compatibility for first pair)
func (m *MultiPairManager) StartUserBot(ctx context.Context, userID int64) error {
	// Get first configured pair for user
	configs, err := m.userRepo.GetAllConfigs(ctx, userID)
	if err != nil {
		return err
	}
	
	if len(configs) == 0 {
		return fmt.Errorf("no trading pairs configured")
	}
	
	// Start first pair
	return m.StartUserPairBot(ctx, userID, configs[0].Symbol)
}

// StopUserBot stops single bot (legacy compatibility)
func (m *MultiPairManager) StopUserBot(ctx context.Context, userID int64) error {
	return m.StopAllUserBots(ctx, userID)
}

// GetUserBot returns first user bot (legacy compatibility)
func (m *MultiPairManager) GetUserBot(userID int64) (interface{}, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if userBots, ok := m.userBots[userID]; ok {
		for _, bot := range userBots {
			return bot, true
		}
	}
	
	return nil, false
}

