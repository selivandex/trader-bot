package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/alexanderselivanov/trader/internal/adapters/ai"
	"github.com/alexanderselivanov/trader/internal/adapters/config"
	"github.com/alexanderselivanov/trader/internal/adapters/database"
	"github.com/alexanderselivanov/trader/internal/adapters/news"
	"github.com/alexanderselivanov/trader/internal/adapters/onchain"
	"github.com/alexanderselivanov/trader/internal/adapters/telegram"
	"github.com/alexanderselivanov/trader/internal/agents"
	"github.com/alexanderselivanov/trader/internal/sentiment"
	"github.com/alexanderselivanov/trader/internal/users"
	"github.com/alexanderselivanov/trader/internal/workers"
	"github.com/alexanderselivanov/trader/pkg/logger"
	"github.com/alexanderselivanov/trader/pkg/models"
	_ "github.com/lib/pq"
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
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize logger
	if err := logger.Init(cfg.Logging.Level, cfg.Logging.File); err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}
	defer logger.Sync()

	logger.Info("AI Trading Bot starting (Multi-User Mode)...",
		zap.String("mode", cfg.Mode.Mode),
	)

	// Initialize database
	db, err := initDatabase(cfg)
	if err != nil {
		return err
	}
	defer db.Close()

	// Initialize AI providers
	aiProviders, err := initAIProviders(cfg)
	if err != nil {
		return err
	}

	// Initialize news system
	newsAggregator, err := initNewsSystem(ctx, cfg, db, aiProviders)
	if err != nil {
		return err
	}

	// Start background workers
	startBackgroundWorkers(ctx, cfg, db)

	// Initialize AGENTIC AI Manager (autonomous agents only)
	agenticManager := agents.NewAgenticManager(db.DB(), newsAggregator, aiProviders)
	defer agenticManager.Shutdown()

	// Initialize User Repository
	userRepo := users.NewAgentsRepository(db)

	// Initialize Agent Repository
	agentRepo := agents.NewRepository(db.DB())

	logger.Info("🤖 Autonomous AI Agent System Ready!",
		zap.Int("ai_providers", len(aiProviders)),
	)

	// Initialize Telegram Bot for agent management
	if cfg.Telegram.BotToken != "" {
		agentBot, err := telegram.NewAgentBot(&cfg.Telegram, agenticManager, userRepo, agentRepo)
		if err != nil {
			logger.Error("failed to create telegram bot", zap.Error(err))
		} else {
			go func() {
				if err := agentBot.Start(ctx); err != nil && err != context.Canceled {
					logger.Error("telegram bot error", zap.Error(err))
				}
			}()

			logger.Info("📱 Telegram bot started for agent control")
		}
	}

	// Keep service running
	<-ctx.Done()
	logger.Info("shutting down gracefully...")

	return nil
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
func startBackgroundWorkers(ctx context.Context, cfg *config.Config, db *database.DB) {
	workersRepo := workers.NewRepository(db.DB())
	newsRepo := news.NewRepository(db.DB())

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

	// On-chain monitoring worker
	if cfg.OnChain.Enabled {
		whaleAlert := onchain.NewWhaleAlertProvider(cfg.OnChain.WhaleAlertKey, true)
		if whaleAlert.IsEnabled() {
			onchainWorker := workers.NewOnChainWorker(workersRepo, whaleAlert, 15*time.Minute, cfg.OnChain.MinValueUSD)
			go func() {
				if err := onchainWorker.Start(ctx); err != nil && err != context.Canceled {
					logger.Error("on-chain worker error", zap.Error(err))
				}
			}()

			logger.Info("on-chain monitoring started", zap.Int("min_value_usd", cfg.OnChain.MinValueUSD))
		}
	}

	logger.Info("background workers started")
}

// TODO: Create Telegram bot for agent management
// Commands needed:
// /start - Register user
// /connect <exchange> <api_key> <api_secret> - Connect exchange
// /add_ticker <symbol> <budget> - Add trading pair
// /create_agent <personality> <name> - Create agent
// /assign_agent <agent_id> <symbol> <budget> - Assign agent to symbol
// /start_agent <agent_id> - Start agent trading
// /stop_agent <agent_id> - Stop agent
// /agents - List all agents
// /stats - View performance
