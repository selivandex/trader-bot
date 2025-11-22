package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	_ "github.com/lib/pq"
	"github.com/selivandex/trader-bot/internal/adapters/ai"
	"github.com/selivandex/trader-bot/internal/adapters/config"
	"github.com/selivandex/trader-bot/internal/adapters/database"
	"github.com/selivandex/trader-bot/internal/adapters/exchange"
	"github.com/selivandex/trader-bot/internal/adapters/market"
	"github.com/selivandex/trader-bot/internal/adapters/news"
	"github.com/selivandex/trader-bot/internal/adapters/onchain"
	"github.com/selivandex/trader-bot/internal/adapters/price"
	redisAdapter "github.com/selivandex/trader-bot/internal/adapters/redis"
	"github.com/selivandex/trader-bot/internal/adapters/telegram"
	"github.com/selivandex/trader-bot/internal/agents"
	"github.com/selivandex/trader-bot/internal/health"
	"github.com/selivandex/trader-bot/internal/sentiment"
	"github.com/selivandex/trader-bot/internal/users"
	"github.com/selivandex/trader-bot/internal/workers"
	"github.com/selivandex/trader-bot/pkg/logger"
	"github.com/selivandex/trader-bot/pkg/models"
)

func main() {
	// Setup signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\nReceived interrupt signal, shutting down...")
		cancel()
	}()

	// Run application
	if err := run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	// Load configuration and initialize logger
	cfg, err := initConfig()
	if err != nil {
		return err
	}
	defer logger.Sync()

	logger.Info("AI Trading Bot starting (Multi-User Mode)...",
		zap.String("mode", cfg.Mode.Mode),
	)

	// Initialize core infrastructure
	db, redisClient, err := initInfrastructure(cfg)
	if err != nil {
		return err
	}
	defer db.Close()
	defer redisClient.Close()

	// Initialize AI providers and market systems
	aiProviders, err := initAIProviders(cfg)
	if err != nil {
		return err
	}

	marketRepo := market.NewRepository(db.DB())

	newsAggregator, err := initNewsSystem(ctx, cfg, db, aiProviders)
	if err != nil {
		return err
	}

	// Start background workers
	startBackgroundWorkers(ctx, cfg, db, marketRepo)

	// Initialize repositories
	repos := initRepositories(db)

	// Initialize Telegram system (templates and notifier)
	templateManager, notifier := initTelegramSystem(cfg, repos.userRepo)

	// Initialize and start agent system
	agenticManager := initAgenticSystem(ctx, cfg, db, redisClient, marketRepo, newsAggregator, aiProviders, notifier)

	// Start health server
	healthServer := startHealthServer(cfg, db, redisClient, agenticManager, len(aiProviders))

	// Start Telegram bot for management
	startTelegramBot(ctx, cfg, agenticManager, repos.userRepo, repos.agentRepo, repos.adminRepo, templateManager)

	// Wait for shutdown signal
	<-ctx.Done()

	// Perform graceful shutdown
	return performGracefulShutdown(healthServer, agenticManager, db, redisClient)
}

// repositorySet holds all initialized repositories
type repositorySet struct {
	userRepo  *users.AgentsRepository
	agentRepo *agents.Repository
	adminRepo *users.AdminRepository
}

// initConfig loads configuration and initializes logger
func initConfig() (*config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	if err := logger.Init(cfg.Logging.Level, cfg.Logging.File); err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}

	return cfg, nil
}

// initInfrastructure initializes database and Redis connections
func initInfrastructure(cfg *config.Config) (*database.DB, *redisAdapter.Client, error) {
	db, err := initDatabase(cfg)
	if err != nil {
		return nil, nil, err
	}

	redisClient, err := initRedis(cfg)
	if err != nil {
		db.Close()
		return nil, nil, err
	}

	return db, redisClient, nil
}

// initRepositories initializes all repository instances
func initRepositories(db *database.DB) *repositorySet {
	return &repositorySet{
		userRepo:  users.NewAgentsRepository(db),
		agentRepo: agents.NewRepository(db.DB()),
		adminRepo: users.NewAdminRepository(db.DB()),
	}
}

// initTelegramSystem initializes Telegram templates and notifier
func initTelegramSystem(cfg *config.Config, userRepo *users.AgentsRepository) (*telegram.TemplateManager, agents.Notifier) {
	if cfg.Telegram.BotToken == "" {
		return nil, nil
	}

	templateManager, err := telegram.NewTemplateManager("./templates/telegram")
	if err != nil {
		logger.Warn("failed to load telegram templates", zap.Error(err))
		return nil, nil
	}

	notifier, err := telegram.NewNotifier(cfg.Telegram.BotToken, userRepo, &cfg.Telegram, templateManager)
	if err != nil {
		logger.Warn("failed to initialize telegram notifier", zap.Error(err))
		return templateManager, nil
	}

	logger.Info("📱 Telegram notifier initialized")
	return templateManager, notifier
}

// initAgenticSystem initializes agent manager and recovers running agents
func initAgenticSystem(
	ctx context.Context,
	cfg *config.Config,
	db *database.DB,
	redisClient *redisAdapter.Client,
	marketRepo *market.Repository,
	newsAggregator *news.Aggregator,
	aiProviders []ai.Provider,
	notifier agents.Notifier,
) *agents.AgenticManager {
	lockFactory := redisClient.GetLockFactory()
	agenticManager := agents.NewAgenticManager(
		db.DB(),
		redisClient,
		lockFactory,
		marketRepo,
		newsAggregator,
		aiProviders,
		notifier,
	)

	exchangeFactory := createExchangeFactory(cfg)

	// Restore running agents from database (pod recovery)
	if err := agenticManager.RestoreRunningAgents(ctx, exchangeFactory); err != nil {
		logger.Error("failed to restore agents", zap.Error(err))
		// Continue anyway - new agents can still be created
	}

	// Start periodic agent recovery worker (safety net)
	startAgentRecoveryWorker(ctx, agenticManager, exchangeFactory)

	return agenticManager
}

// initDatabase initializes database connection with sqlx
func initDatabase(cfg *config.Config) (*database.DB, error) {
	db, err := database.New(&cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Run migrations
	migrationsPath := "./migrations"
	if err := database.RunMigrations(db.Conn(), migrationsPath); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	logger.Info("database connection established (sqlx)",
		zap.String("host", cfg.Database.Host),
		zap.String("database", cfg.Database.Name),
	)

	return db, nil
}

// initRedis initializes Redis client with Redlock support
func initRedis(cfg *config.Config) (*redisAdapter.Client, error) {
	redisClient, err := redisAdapter.New(&cfg.Redis)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	// Test connection
	if err := redisClient.Health(); err != nil {
		redisClient.Close()
		return nil, fmt.Errorf("redis health check failed: %w", err)
	}

	logger.Info("redis connection established (redlock)",
		zap.String("host", cfg.Redis.Host),
		zap.Int("port", cfg.Redis.Port),
	)

	return redisClient, nil
}

// initAIProviders initializes AI providers
func initAIProviders(cfg *config.Config) ([]ai.Provider, error) {
	// Create default strategy params for AI providers
	strategyParams := &models.StrategyParameters{
		MaxPositionPercent:     cfg.Trading.MaxPositionPercent,
		MaxLeverage:            cfg.Trading.MaxLeverage,
		StopLossPercent:        cfg.Trading.StopLossPercent,
		TakeProfitPercent:      cfg.Trading.TakeProfitPercent,
		MinConfidenceThreshold: 70,
	}

	logger.Info("strategy parameters loaded",
		zap.Float64("max_position_percent", strategyParams.MaxPositionPercent),
		zap.Int("max_leverage", strategyParams.MaxLeverage),
	)

	var aiProviders []ai.Provider

	if cfg.AI.DeepSeek.Enabled {
		deepseek := ai.NewDeepSeekProvider(&cfg.AI.DeepSeek, strategyParams)
		aiProviders = append(aiProviders, deepseek)
	}

	if cfg.AI.Claude.Enabled {
		claude := ai.NewClaudeProvider(&cfg.AI.Claude, strategyParams)
		aiProviders = append(aiProviders, claude)
	}

	if cfg.AI.OpenAI.Enabled {
		openai := ai.NewOpenAIProvider(&cfg.AI.OpenAI, strategyParams)
		aiProviders = append(aiProviders, openai)
	}

	if len(aiProviders) == 0 {
		return nil, fmt.Errorf("no AI providers configured")
	}

	logger.Info("AI providers initialized for agents",
		zap.Strings("providers", cfg.AI.GetEnabledAIProviders()),
		zap.Int("count", len(aiProviders)),
	)

	return aiProviders, nil
}

// initNewsSystem initializes news aggregation and analysis
func initNewsSystem(ctx context.Context, cfg *config.Config, db *database.DB, aiProviders []ai.Provider) (*news.Aggregator, error) {
	if !cfg.News.Enabled {
		logger.Info("news system disabled")
		return nil, nil
	}

	sentimentAnalyzer := sentiment.NewAnalyzer()
	newsRepo := news.NewRepository(db.DB())
	newsCache := news.NewCache(newsRepo)

	var newsProviders []news.Provider

	if cfg.News.TwitterEnabled {
		twitter := news.NewTwitterProvider(cfg.News.TwitterAPIKey, true, sentimentAnalyzer)
		newsProviders = append(newsProviders, twitter)
	}

	if cfg.News.RedditEnabled {
		reddit := news.NewRedditProvider(true, []string{"CryptoCurrency", "Bitcoin", "ethereum"}, sentimentAnalyzer)
		newsProviders = append(newsProviders, reddit)
	}

	if cfg.News.CoinDeskEnabled {
		coindesk := news.NewCoinDeskProvider(true, sentimentAnalyzer)
		newsProviders = append(newsProviders, coindesk)
	}

	if len(newsProviders) == 0 {
		logger.Warn("no news providers configured")
		return nil, nil
	}

	newsAggregator := news.NewAggregator(newsProviders, cfg.News.Keywords, newsCache)

	// Create news evaluator if enabled
	var newsEvaluator ai.NewsEvaluatorInterface
	if cfg.News.EvaluatorEnabled {
		newsEvaluator = createNewsEvaluator(cfg, aiProviders)
	}

	// Start news worker
	newsWorker := workers.NewNewsWorker(newsAggregator, newsCache, newsEvaluator, 10*time.Minute, cfg.News.Keywords)
	go func() {
		if err := newsWorker.Start(ctx); err != nil && err != context.Canceled {
			logger.Error("news worker error", zap.Error(err))
		}
	}()

	logger.Info("news system initialized",
		zap.Int("providers", len(newsProviders)),
		zap.Strings("keywords", cfg.News.Keywords),
	)

	return newsAggregator, nil
}

// createNewsEvaluator creates AI news evaluator
func createNewsEvaluator(cfg *config.Config, aiProviders []ai.Provider) ai.NewsEvaluatorInterface {
	strategyParams := &models.StrategyParameters{
		MaxPositionPercent:     cfg.Trading.MaxPositionPercent,
		MaxLeverage:            cfg.Trading.MaxLeverage,
		StopLossPercent:        cfg.Trading.StopLossPercent,
		TakeProfitPercent:      cfg.Trading.TakeProfitPercent,
		MinConfidenceThreshold: 70,
	}

	if cfg.News.EvaluatorEnsemble {
		// Use ensemble
		providers := []ai.Provider{}
		for _, p := range aiProviders {
			if p.IsEnabled() {
				providers = append(providers, p)
			}
		}

		if len(providers) > 0 {
			ensemble := ai.NewNewsEvaluatorEnsemble(providers)
			logger.Info("AI news evaluator ensemble enabled",
				zap.Int("count", len(providers)),
			)
			return ensemble
		}
	} else {
		// Use single provider
		var newsProvider ai.Provider
		switch cfg.News.EvaluatorProvider {
		case "deepseek":
			if cfg.AI.DeepSeek.Enabled && cfg.AI.DeepSeek.APIKey != "" {
				newsProvider = ai.NewDeepSeekProvider(&cfg.AI.DeepSeek, strategyParams)
			}
		case "openai":
			if cfg.AI.OpenAI.Enabled && cfg.AI.OpenAI.APIKey != "" {
				newsProvider = ai.NewOpenAIProvider(&cfg.AI.OpenAI, strategyParams)
			}
		case "claude":
			if cfg.AI.Claude.Enabled && cfg.AI.Claude.APIKey != "" {
				newsProvider = ai.NewClaudeProvider(&cfg.AI.Claude, strategyParams)
			}
		}

		if newsProvider != nil {
			logger.Info("AI news evaluator enabled", zap.String("provider", newsProvider.GetName()))
			return ai.NewNewsEvaluator(newsProvider)
		}
	}

	logger.Warn("news evaluator not configured")
	return nil
}

// startBackgroundWorkers starts all background workers
func startBackgroundWorkers(ctx context.Context, cfg *config.Config, db *database.DB, marketRepo *market.Repository) {
	workersRepo := workers.NewRepository(db.DB())
	newsRepo := news.NewRepository(db.DB())

	// Candles worker - saves OHLCV for backtesting
	// Uses MockExchange for now (replace with real exchange when available)
	mockExchange := &exchange.MockExchange{}
	symbols := []string{"BTC/USDT", "ETH/USDT"}
	timeframes := []string{"1m", "5m", "15m", "1h", "4h"}
	candlesWorker := workers.NewCandlesWorker(mockExchange, marketRepo, 1*time.Minute, symbols, timeframes)

	go func() {
		if err := candlesWorker.Start(ctx); err != nil && err != context.Canceled {
			logger.Error("candles worker error", zap.Error(err))
		}
	}()

	logger.Info("candles worker started (historical data collection)")

	// Daily metrics worker
	dailyMetrics := workers.NewDailyMetricsWorker(workersRepo)
	go func() {
		if err := dailyMetrics.Start(ctx); err != nil && err != context.Canceled {
			logger.Error("daily metrics worker error", zap.Error(err))
		}
	}()

	// Sentiment aggregator
	sentimentAggregator := workers.NewSentimentAggregator(workersRepo, newsRepo, 5*time.Minute)
	go func() {
		if err := sentimentAggregator.Start(ctx); err != nil && err != context.Canceled {
			logger.Error("sentiment aggregator error", zap.Error(err))
		}
	}()

	// Exchange flow aggregator
	exchangeFlowAgg := workers.NewExchangeFlowAggregator(workersRepo, 1*time.Hour)
	go func() {
		if err := exchangeFlowAgg.Start(ctx); err != nil && err != context.Canceled {
			logger.Error("exchange flow aggregator error", zap.Error(err))
		}
	}()

	// On-chain monitoring worker with multiple providers
	if cfg.OnChain.Enabled {
		var onchainProviders []onchain.OnChainProvider

		// Whale Alert (платный, все блокчейны)
		if cfg.OnChain.WhaleAlert.Enabled {
			whaleAlert := onchain.NewWhaleAlertProvider(cfg.OnChain.WhaleAlert.APIKey, true)
			onchainProviders = append(onchainProviders, whaleAlert)
			logger.Info("WhaleAlert enabled", zap.String("cost", "$0.005/req"))
		}

		// Blockchain.com (бесплатный, BTC only)
		if cfg.OnChain.BlockchainCom.Enabled {
			// Price provider for BTC→USD conversion
			priceProvider := price.NewCoinGeckoProvider()
			blockchainCom := onchain.NewBlockchainComAdapter(true, priceProvider)
			onchainProviders = append(onchainProviders, blockchainCom)
			logger.Info("Blockchain.com enabled", zap.String("cost", "free"), zap.String("price_api", "CoinGecko+DB"))
		}

		// Etherscan (бесплатный, USDT/ETH)
		if cfg.OnChain.Etherscan.Enabled {
			etherscan := onchain.NewEtherscanAdapter(cfg.OnChain.Etherscan.APIKey, true)
			onchainProviders = append(onchainProviders, etherscan)
			logger.Info("Etherscan enabled", zap.String("cost", "free"))
		}

		if len(onchainProviders) > 0 {
			// Use first provider for worker (can be extended to use aggregator)
			primaryProvider := onchainProviders[0]
			onchainWorker := workers.NewOnChainWorker(workersRepo, primaryProvider, 15*time.Minute, cfg.OnChain.MinValueUSD)

			go func() {
				if err := onchainWorker.Start(ctx); err != nil && err != context.Canceled {
					logger.Error("on-chain worker error", zap.Error(err))
				}
			}()

			logger.Info("on-chain monitoring started",
				zap.Int("providers", len(onchainProviders)),
				zap.Int("min_value_usd", cfg.OnChain.MinValueUSD),
			)
		}
	}

	logger.Info("background workers started")
}

// startHealthServer initializes and starts health check server for K8s probes
func startHealthServer(cfg *config.Config, db *database.DB, redisClient *redisAdapter.Client, agenticManager *agents.AgenticManager, aiProvidersCount int) *health.Server {
	healthServer := health.NewServer(cfg.Health.Port, db, redisClient, agenticManager)

	go func() {
		if err := healthServer.Start(); err != nil && err != http.ErrServerClosed {
			logger.Error("health server error", zap.Error(err))
		}
	}()

	logger.Info("🤖 Autonomous AI Agent System Ready!",
		zap.Int("ai_providers", aiProvidersCount),
		zap.String("health_port", cfg.Health.Port),
	)

	// Mark service as ready after initialization
	healthServer.SetReady(true)

	return healthServer
}

// startTelegramBot initializes and starts Telegram bot for agent management
func startTelegramBot(ctx context.Context, cfg *config.Config, agenticManager *agents.AgenticManager, userRepo *users.AgentsRepository, agentRepo *agents.Repository, adminRepo *users.AdminRepository, templateManager *telegram.TemplateManager) {
	if cfg.Telegram.BotToken == "" {
		logger.Info("telegram bot disabled (no token provided)")
		return
	}

	agentBot, err := telegram.NewAgentBot(cfg, agenticManager, userRepo, agentRepo, adminRepo, templateManager)
	if err != nil {
		logger.Error("failed to create telegram bot", zap.Error(err))
		return
	}

	go func() {
		if err := agentBot.Start(ctx); err != nil && err != context.Canceled {
			logger.Error("telegram bot error", zap.Error(err))
		}
	}()

	logger.Info("📱 Telegram bot started for agent control",
		zap.Bool("admin_enabled", cfg.Telegram.AdminID != 0),
	)
}

// performGracefulShutdown handles graceful shutdown of all components
func performGracefulShutdown(healthServer *health.Server, agenticManager *agents.AgenticManager, db *database.DB, redisClient *redisAdapter.Client) error {
	logger.Info("🛑 Shutdown signal received, starting graceful shutdown...")

	// Mark service as not ready (stop accepting new traffic)
	healthServer.SetReady(false)

	// Create shutdown context with timeout (K8s gives 30s terminationGracePeriodSeconds)
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer shutdownCancel()

	// Shutdown agent manager first (stops all agents gracefully)
	logger.Info("stopping agent manager...")
	if err := agenticManager.Shutdown(); err != nil {
		logger.Error("agent manager shutdown error", zap.Error(err))
	}

	// Close database connection
	logger.Info("closing database connection...")
	if err := db.Close(); err != nil {
		logger.Error("database close error", zap.Error(err))
	}

	// Close redis connection
	logger.Info("closing redis connection...")
	if err := redisClient.Close(); err != nil {
		logger.Error("redis close error", zap.Error(err))
	}

	// Stop health server
	logger.Info("stopping health server...")
	if err := healthServer.Stop(shutdownCtx); err != nil {
		logger.Error("health server stop error", zap.Error(err))
	}

	// Sync logger
	logger.Sync()

	// Check if shutdown completed in time
	select {
	case <-shutdownCtx.Done():
		logger.Warn("⚠️ shutdown timeout exceeded")
		return fmt.Errorf("graceful shutdown timeout")
	default:
		logger.Info("✅ shutdown completed successfully")
	}

	return nil
}

// createExchangeFactory creates a factory function for exchange adapters (used for agent recovery)
func createExchangeFactory(cfg *config.Config) func(string, string, string, bool) (exchange.Exchange, error) {
	return func(exchangeName, apiKey, apiSecret string, testnet bool) (exchange.Exchange, error) {
		switch exchangeName {
		case "binance":
			return exchange.NewBinanceAdapter(apiKey, apiSecret, testnet, &cfg.Exchanges.Binance)
		case "bybit":
			return exchange.NewBybitAdapter(apiKey, apiSecret, testnet, &cfg.Exchanges.Bybit)
		default:
			return nil, fmt.Errorf("unsupported exchange: %s", exchangeName)
		}
	}
}

// startAgentRecoveryWorker starts periodic agent recovery worker
// This worker periodically checks for agents that should be running but aren't (safety net)
func startAgentRecoveryWorker(ctx context.Context, agenticManager *agents.AgenticManager, exchangeFactory func(string, string, string, bool) (exchange.Exchange, error)) {
	recoveryInterval := 5 * time.Minute
	recoveryWorker := workers.NewAgentRecoveryWorker(agenticManager, exchangeFactory, recoveryInterval)

	go func() {
		if err := recoveryWorker.Start(ctx); err != nil && err != context.Canceled {
			logger.Error("agent recovery worker error", zap.Error(err))
		}
	}()

	logger.Info("🔄 periodic agent recovery worker started",
		zap.Duration("interval", recoveryInterval),
	)
}
