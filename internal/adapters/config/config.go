package config

import (
	"fmt"
	"time"

	"github.com/kelseyhightower/envconfig"
)

// Config represents application configuration
type Config struct {
	Mode TradingConfig `envconfig:"MODE"`

	Exchanges ExchangesConfig `envconfig:"EXCHANGES"`
	Trading   TradingConfig   `envconfig:"TRADING"`
	AI        AIConfig        `envconfig:"AI"`
	News      NewsConfig      `envconfig:"NEWS"`
	Risk      RiskConfig      `envconfig:"RISK"`
	Telegram  TelegramConfig  `envconfig:"TELEGRAM"`
	Database  DatabaseConfig  `envconfig:"DATABASE"`
	Logging   LoggingConfig   `envconfig:"LOGGING"`
}

// TradingModeConfig represents trading mode
type TradingModeConfig struct {
	Mode string `envconfig:"MODE" default:"paper"` // paper or live
}

// ExchangesConfig represents exchange configurations
type ExchangesConfig struct {
	Binance ExchangeConfig `envconfig:"BINANCE"`
	Bybit   ExchangeConfig `envconfig:"BYBIT"`
}

// ExchangeConfig represents single exchange configuration
type ExchangeConfig struct {
	APIKey  string `envconfig:"API_KEY" required:"false"`
	Secret  string `envconfig:"SECRET" required:"false"`
	Testnet bool   `envconfig:"TESTNET" default:"true"`
}

// TradingConfig represents trading parameters
type TradingConfig struct {
	Symbol                    string  `envconfig:"TRADING_SYMBOL" default:"BTC/USDT"`
	InitialBalance            float64 `envconfig:"TRADING_INITIAL_BALANCE" default:"1000.0"`
	MaxPositionPercent        float64 `envconfig:"TRADING_MAX_POSITION_PERCENT" default:"30.0"`
	MaxLeverage               int     `envconfig:"TRADING_MAX_LEVERAGE" default:"3"`
	StopLossPercent           float64 `envconfig:"TRADING_STOP_LOSS_PERCENT" default:"2.0"`
	TakeProfitPercent         float64 `envconfig:"TRADING_TAKE_PROFIT_PERCENT" default:"5.0"`
	ProfitWithdrawalThreshold float64 `envconfig:"TRADING_PROFIT_WITHDRAWAL_THRESHOLD" default:"1.1"`
}

// AIConfig represents AI provider configurations
type AIConfig struct {
	DeepSeek             AIProviderConfig `envconfig:"DEEPSEEK"`
	Claude               AIProviderConfig `envconfig:"CLAUDE"`
	OpenAI               AIProviderConfig `envconfig:"OPENAI"`
	EnsembleEnabled      bool             `envconfig:"AI_ENSEMBLE_ENABLED" default:"true"`
	EnsembleMinConsensus int              `envconfig:"AI_ENSEMBLE_MIN_CONSENSUS" default:"2"`
	DecisionInterval     time.Duration    `envconfig:"AI_DECISION_INTERVAL" default:"30m"`
}

// AIProviderConfig represents single AI provider configuration
type AIProviderConfig struct {
	APIKey  string  `envconfig:"API_KEY" required:"false"`
	Enabled bool    `envconfig:"ENABLED" default:"false"`
	Weight  float64 `envconfig:"WEIGHT" default:"1.0"`
}

// NewsConfig represents news aggregation configuration
type NewsConfig struct {
	Enabled         bool     `envconfig:"NEWS_ENABLED" default:"true"`
	TwitterAPIKey   string   `envconfig:"TWITTER_API_KEY" required:"false"`
	TwitterEnabled  bool     `envconfig:"TWITTER_ENABLED" default:"false"`
	RedditEnabled   bool     `envconfig:"REDDIT_ENABLED" default:"true"`
	CoinDeskEnabled bool     `envconfig:"COINDESK_ENABLED" default:"true"`
	Keywords        []string `envconfig:"NEWS_KEYWORDS" default:"bitcoin,btc,crypto,cryptocurrency,ethereum,eth"`
}

// RiskConfig represents risk management parameters
type RiskConfig struct {
	MaxConsecutiveLosses   int           `envconfig:"RISK_MAX_CONSECUTIVE_LOSSES" default:"5"`
	MaxDailyLossPercent    float64       `envconfig:"RISK_MAX_DAILY_LOSS_PERCENT" default:"5.0"`
	MaxDrawdownPercent     float64       `envconfig:"RISK_MAX_DRAWDOWN_PERCENT" default:"15.0"`
	CircuitBreakerCooldown time.Duration `envconfig:"RISK_CIRCUIT_BREAKER_COOLDOWN" default:"4h"`
}

// TelegramConfig represents Telegram bot configuration
type TelegramConfig struct {
	BotToken      string `envconfig:"TELEGRAM_BOT_TOKEN" required:"true"`
	ChatID        int64  `envconfig:"TELEGRAM_CHAT_ID" required:"true"`
	AlertOnTrades bool   `envconfig:"TELEGRAM_ALERT_ON_TRADES" default:"true"`
	AlertOnErrors bool   `envconfig:"TELEGRAM_ALERT_ON_ERRORS" default:"true"`
}

// DatabaseConfig represents database connection parameters
type DatabaseConfig struct {
	Host     string `envconfig:"DB_HOST" default:"localhost"`
	Port     int    `envconfig:"DB_PORT" default:"5432"`
	Name     string `envconfig:"DB_NAME" default:"trader"`
	User     string `envconfig:"DB_USER" required:"true"`
	Password string `envconfig:"DB_PASSWORD" required:"true"`
	SSLMode  string `envconfig:"DB_SSLMODE" default:"disable"`
}

// LoggingConfig represents logging configuration
type LoggingConfig struct {
	Level string `envconfig:"LOG_LEVEL" default:"info"`
	File  string `envconfig:"LOG_FILE" default:"logs/bot.log"`
}

// Load reads configuration from environment variables
func Load() (*Config, error) {
	var cfg Config

	// Process environment variables
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, fmt.Errorf("failed to process config: %w", err)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

// Validate checks if configuration is valid
func (c *Config) Validate() error {
	// Check at least one exchange is configured
	if c.Exchanges.Binance.APIKey == "" && c.Exchanges.Bybit.APIKey == "" {
		return fmt.Errorf("at least one exchange must be configured")
	}

	// Check at least one AI provider is enabled and configured
	aiConfigured := false
	if c.AI.DeepSeek.Enabled && c.AI.DeepSeek.APIKey != "" {
		aiConfigured = true
	}
	if c.AI.Claude.Enabled && c.AI.Claude.APIKey != "" {
		aiConfigured = true
	}
	if c.AI.OpenAI.Enabled && c.AI.OpenAI.APIKey != "" {
		aiConfigured = true
	}
	if !aiConfigured {
		return fmt.Errorf("at least one AI provider must be enabled and configured")
	}

	// Validate trading parameters
	if c.Trading.MaxPositionPercent <= 0 || c.Trading.MaxPositionPercent > 100 {
		return fmt.Errorf("max_position_percent must be between 0 and 100")
	}
	if c.Trading.MaxLeverage < 1 || c.Trading.MaxLeverage > 125 {
		return fmt.Errorf("max_leverage must be between 1 and 125")
	}
	if c.Trading.StopLossPercent <= 0 {
		return fmt.Errorf("stop_loss_percent must be positive")
	}
	if c.Trading.InitialBalance <= 0 {
		return fmt.Errorf("initial_balance must be positive")
	}

	// Validate risk parameters
	if c.Risk.MaxConsecutiveLosses < 1 {
		return fmt.Errorf("max_consecutive_losses must be at least 1")
	}
	if c.Risk.MaxDailyLossPercent <= 0 {
		return fmt.Errorf("max_daily_loss_percent must be positive")
	}

	// Validate Telegram config
	if c.Telegram.BotToken == "" {
		return fmt.Errorf("telegram bot token is required")
	}
	if c.Telegram.ChatID == 0 {
		return fmt.Errorf("telegram chat_id is required")
	}

	return nil
}

// GetDSN returns PostgreSQL connection string
func (c *DatabaseConfig) GetDSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Name, c.SSLMode,
	)
}

// IsPaperTrading returns true if bot is in paper trading mode
func (c *Config) IsPaperTrading() bool {
	return c.Mode.Mode == "paper"
}

// GetEnabledAIProviders returns list of enabled AI provider names
func (c *AIConfig) GetEnabledAIProviders() []string {
	var providers []string
	if c.DeepSeek.Enabled && c.DeepSeek.APIKey != "" {
		providers = append(providers, "deepseek")
	}
	if c.Claude.Enabled && c.Claude.APIKey != "" {
		providers = append(providers, "claude")
	}
	if c.OpenAI.Enabled && c.OpenAI.APIKey != "" {
		providers = append(providers, "openai")
	}
	return providers
}
