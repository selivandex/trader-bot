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
	"github.com/alexanderselivanov/trader/internal/adapters/telegram"
	"github.com/alexanderselivanov/trader/internal/bot"
	"github.com/alexanderselivanov/trader/internal/sentiment"
	"github.com/alexanderselivanov/trader/internal/workers"
	"github.com/alexanderselivanov/trader/pkg/logger"
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
	db, err := database.New(&cfg.Database)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	// Run database migrations
	migrationsPath := "./migrations"
	if err := database.RunMigrations(db.Conn(), migrationsPath); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	// Initialize AI providers
	var aiProviders []ai.Provider

	if cfg.AI.DeepSeek.Enabled {
		deepseek := ai.NewDeepSeekProvider(&cfg.AI.DeepSeek)
		aiProviders = append(aiProviders, deepseek)
	}

	if cfg.AI.Claude.Enabled {
		claude := ai.NewClaudeProvider(&cfg.AI.Claude)
		aiProviders = append(aiProviders, claude)
	}

	if cfg.AI.OpenAI.Enabled {
		openai := ai.NewOpenAIProvider(&cfg.AI.OpenAI)
		aiProviders = append(aiProviders, openai)
	}

	if len(aiProviders) == 0 {
		return fmt.Errorf("no AI providers configured")
	}

	// Create AI ensemble
	aiEnsemble := ai.NewEnsemble(aiProviders, cfg.AI.EnsembleMinConsensus)

	logger.Info("AI ensemble initialized",
		zap.Strings("providers", cfg.AI.GetEnabledAIProviders()),
		zap.Bool("ensemble_enabled", cfg.AI.EnsembleEnabled),
	)

	// Initialize sentiment analyzer
	sentimentAnalyzer := sentiment.NewAnalyzer()

	// Initialize news cache
	newsCache := news.NewCache(db)

	// Initialize news aggregator
	var newsAggregator *news.Aggregator
	var newsWorker *workers.NewsWorker

	if cfg.News.Enabled {
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

		if len(newsProviders) > 0 {
			newsAggregator = news.NewAggregator(newsProviders, cfg.News.Keywords, newsCache)

			// Create news worker (runs in background every 10 minutes)
			newsWorker = workers.NewNewsWorker(newsAggregator, newsCache, 10*time.Minute, cfg.News.Keywords)

			logger.Info("news system initialized",
				zap.Int("providers", len(newsProviders)),
				zap.Strings("keywords", cfg.News.Keywords),
			)

			// Start news worker in background
			go func() {
				if err := newsWorker.Start(ctx); err != nil && err != context.Canceled {
					logger.Error("news worker error", zap.Error(err))
				}
			}()
		}
	}

	// Initialize Multi-Pair Bot Manager
	botManager := bot.NewMultiPairManager(db, cfg, aiEnsemble, newsAggregator)

	// Initialize Multi-User Telegram bot
	telegramBot, err := telegram.NewMultiUserBot(&cfg.Telegram, botManager)
	if err != nil {
		return fmt.Errorf("failed to initialize telegram bot: %w", err)
	}
	defer telegramBot.Close()

	// Start Telegram bot in background
	go func() {
		if err := telegramBot.Start(ctx); err != nil && err != context.Canceled {
			logger.Error("telegram bot error", zap.Error(err))
		}
	}()

	// Send startup notification
	telegramBot.SendMessage(
		"🤖 *Multi-User Multi-Pair Trading Bot Started!*\n\n" +
			"✅ Multiple users supported\n" +
			"✅ Multiple trading pairs per user\n" +
			"✅ AI-powered decisions\n" +
			"✅ News sentiment analysis\n\n" +
			"New users: /start\n" +
			"Help: /help",
	)

	logger.Info("multi-user bot ready, starting bot manager...")

	// Start Bot Manager (manages all user bots)
	if err := botManager.Start(ctx); err != nil {
		if err == context.Canceled {
			logger.Info("bot manager stopped gracefully")
			return nil
		}
		return fmt.Errorf("bot manager error: %w", err)
	}

	return nil
}
